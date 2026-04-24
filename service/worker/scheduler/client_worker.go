// Copyright (c) 2026 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package scheduler

import (
	"context"
	"sync"
	"time"

	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	cadenceworker "go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/service"
)

const (
	defaultRefreshInterval = 1 * time.Minute
	defaultShutdownTimeout = 5 * time.Second

	// membershipSubscriberName is the unique name used to subscribe to membership
	// change notifications for the worker service ring.
	membershipSubscriberName = "scheduler-worker-manager"

	// workerRedundancyFactor is the number of hosts that concurrently run a
	// worker for each domain. Using 2 means every domain has a primary and one
	// backup, so a single host failure causes no scheduling gap.
	workerRedundancyFactor = 2
)

// BootstrapParams contains the parameters needed to create a scheduler worker manager.
type BootstrapParams struct {
	ServiceClient      workflowserviceclient.Interface
	FrontendClient     frontend.Client
	MetricsClient      metrics.Client
	Logger             log.Logger
	DomainCache        cache.DomainCache
	MembershipResolver membership.Resolver
	HostInfo           membership.HostInfo
}

// workerHandle is the subset of cadenceworker.Worker used by the manager,
// extracted to allow unit testing without starting real pollers.
type workerHandle interface {
	Stop()
}

// workerFactory creates a worker for a given domain. Returns a workerHandle
// and an error. The default factory creates a real Cadence SDK worker.
type workerFactory func(domainName string) (workerHandle, error)

// WorkerManager manages per-domain scheduler workers. It periodically scans
// the domain cache and uses the membership hashring to determine which domains
// this host should cover. For each such domain it starts a Cadence SDK worker
// polling the scheduler task list. Each domain is covered by
// workerRedundancyFactor hosts simultaneously so that a single host failure
// does not cause a scheduling gap.
type WorkerManager struct {
	enabledFn          dynamicproperties.BoolPropertyFnWithDomainFilter
	serviceClient      workflowserviceclient.Interface
	frontendClient     frontend.Client
	metricsClient      metrics.Client
	logger             log.Logger
	domainCache        cache.DomainCache
	membershipResolver membership.Resolver
	hostInfo           membership.HostInfo
	timeSrc            clock.TimeSource
	refreshInterval    time.Duration
	shutdownTimeout    time.Duration
	ctx                context.Context
	cancelFn           context.CancelFunc
	wg                 sync.WaitGroup
	activeWorkers      map[string]workerHandle // domain name -> worker
	createWorker       workerFactory
	membershipChangeCh chan *membership.ChangedEvent
}

// NewWorkerManager creates a new per-domain scheduler worker manager.
func NewWorkerManager(params *BootstrapParams, enabledFn dynamicproperties.BoolPropertyFnWithDomainFilter) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())
	wm := &WorkerManager{
		enabledFn:          enabledFn,
		serviceClient:      params.ServiceClient,
		frontendClient:     params.FrontendClient,
		metricsClient:      params.MetricsClient,
		logger:             params.Logger.WithTags(tag.ComponentScheduler),
		domainCache:        params.DomainCache,
		membershipResolver: params.MembershipResolver,
		hostInfo:           params.HostInfo,
		timeSrc:            clock.NewRealTimeSource(),
		refreshInterval:    defaultRefreshInterval,
		shutdownTimeout:    defaultShutdownTimeout,
		ctx:                ctx,
		cancelFn:           cancel,
		activeWorkers:      make(map[string]workerHandle),
		membershipChangeCh: make(chan *membership.ChangedEvent, 10),
	}
	wm.createWorker = wm.defaultCreateWorker
	return wm
}

// Start begins the background loop that manages per-domain workers.
func (m *WorkerManager) Start() {
	m.logger.Info("scheduler worker manager starting")
	if err := m.membershipResolver.Subscribe(service.Worker, membershipSubscriberName, m.membershipChangeCh); err != nil {
		m.logger.Warn("failed to subscribe to membership changes, will rely on periodic refresh only", tag.Error(err))
	}
	m.wg.Add(1)
	go m.run()
}

// Stop signals the background loop to stop and waits for it to finish.
// It then stops all active workers.
func (m *WorkerManager) Stop() {
	m.logger.Info("scheduler worker manager stopping")
	if err := m.membershipResolver.Unsubscribe(service.Worker, membershipSubscriberName); err != nil {
		m.logger.Warn("failed to unsubscribe from membership changes", tag.Error(err))
	}
	m.cancelFn()
	if !common.AwaitWaitGroup(&m.wg, m.shutdownTimeout) {
		m.logger.Warn("scheduler worker manager timed out on shutdown")
	}
	m.stopAllWorkers()
	m.logger.Info("scheduler worker manager stopped")
}

func (m *WorkerManager) run() {
	defer m.wg.Done()

	ticker := m.timeSrc.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	m.refreshWorkers()

	for {
		select {
		case <-ticker.Chan():
			m.refreshWorkers()

		case <-m.membershipChangeCh:
			drainMembershipCh(m.membershipChangeCh)
			m.logger.Debug("membership ring changed, refreshing scheduler workers")
			m.refreshWorkers()

		case <-m.ctx.Done():
			m.logger.Info("scheduler worker manager background loop stopped")
			return
		}
	}
}

// refreshWorkers scans all domains and reconciles the set of active workers
// with the domains this host owns via the membership hashring.
func (m *WorkerManager) refreshWorkers() {
	scope := m.metricsClient.Scope(metrics.SchedulerWorkerScope)
	startTime := time.Now()
	defer func() {
		scope.ExponentialHistogram(metrics.SchedulerWorkerRefreshLatencyHistogram, time.Since(startTime))
		scope.UpdateGauge(metrics.SchedulerWorkerActiveGauge, float64(len(m.activeWorkers)))
	}()

	domains := m.domainCache.GetAllDomain()
	ownedDomains := make(map[string]struct{}, len(domains))
	lookupFailed := make(map[string]struct{})

	for _, domainEntry := range domains {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		if domainEntry.IsDeprecatedOrDeleted() {
			continue
		}

		domainName := domainEntry.GetInfo().Name

		owners, err := m.membershipResolver.LookupN(service.Worker, domainName, workerRedundancyFactor)
		if err != nil {
			m.logger.Warn("failed to look up domain owners, skipping",
				tag.WorkflowDomainName(domainName),
				tag.Error(err),
			)
			scope.IncCounter(metrics.SchedulerWorkerLookupFailuresCount)
			lookupFailed[domainName] = struct{}{}
			continue
		}

		if !containsHost(owners, m.hostInfo) {
			continue
		}

		if !m.enabledFn(domainName) {
			continue
		}

		ownedDomains[domainName] = struct{}{}

		if _, exists := m.activeWorkers[domainName]; exists {
			continue
		}

		m.startWorkerForDomain(scope, domainName)
	}

	for domainName, w := range m.activeWorkers {
		if _, owned := ownedDomains[domainName]; owned {
			scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.SchedulerWorkerDomainCoverageCount)
			continue
		}
		// Keep workers running for domains where lookup failed to avoid
		// unnecessary churn during transient membership ring issues.
		if _, failed := lookupFailed[domainName]; failed {
			continue
		}
		m.logger.Info("stopping scheduler worker for domain no longer owned",
			tag.WorkflowDomainName(domainName),
		)
		w.Stop()
		scope.IncCounter(metrics.SchedulerWorkerStoppedCount)
		delete(m.activeWorkers, domainName)
	}

	m.logger.Debug("scheduler workers refreshed",
		tag.Dynamic("active-worker-count", len(m.activeWorkers)),
	)
}

func (m *WorkerManager) startWorkerForDomain(scope metrics.Scope, domainName string) {
	w, err := m.createWorker(domainName)
	if err != nil {
		m.logger.Error("failed to start scheduler worker for domain",
			tag.WorkflowDomainName(domainName),
			tag.Error(err),
		)
		scope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.SchedulerWorkerStartErrorsCountPerDomain)
		return
	}

	m.activeWorkers[domainName] = w
	scope.IncCounter(metrics.SchedulerWorkerStartedCount)
	m.logger.Info("started scheduler worker for domain",
		tag.WorkflowDomainName(domainName),
	)
}

func (m *WorkerManager) defaultCreateWorker(domainName string) (workerHandle, error) {
	actCtx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
		FrontendClient: m.frontendClient,
		MetricsClient:  m.metricsClient,
	})

	w := cadenceworker.New(m.serviceClient, domainName, TaskListName, cadenceworker.Options{
		BackgroundActivityContext: actCtx,
	})
	w.RegisterWorkflowWithOptions(SchedulerWorkflow, workflow.RegisterOptions{Name: WorkflowTypeName})

	if err := w.Start(); err != nil {
		return nil, err
	}
	return w, nil
}

func (m *WorkerManager) stopAllWorkers() {
	for domainName, w := range m.activeWorkers {
		w.Stop()
		m.logger.Info("stopped scheduler worker for domain",
			tag.WorkflowDomainName(domainName),
		)
		delete(m.activeWorkers, domainName)
	}
}

// drainMembershipCh consumes all pending events from the channel without
// blocking, so that a single refreshWorkers call covers all queued changes.
func drainMembershipCh(ch <-chan *membership.ChangedEvent) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// containsHost reports whether hosts contains the given target host.
func containsHost(hosts []membership.HostInfo, target membership.HostInfo) bool {
	for _, h := range hosts {
		if h.Identity() == target.Identity() {
			return true
		}
	}
	return false
}
