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
	"github.com/uber/cadence/common/service"
)

const (
	defaultRefreshInterval = 1 * time.Minute
	defaultShutdownTimeout = 5 * time.Second
)

// BootstrapParams contains the parameters needed to create a scheduler worker manager.
type BootstrapParams struct {
	ServiceClient      workflowserviceclient.Interface
	FrontendClient     frontend.Client
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
// this host owns. For each owned domain it starts a Cadence SDK worker polling
// the scheduler task list in that domain.
type WorkerManager struct {
	enabledFn          dynamicproperties.BoolPropertyFn
	serviceClient      workflowserviceclient.Interface
	frontendClient     frontend.Client
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
}

// NewWorkerManager creates a new per-domain scheduler worker manager.
func NewWorkerManager(params *BootstrapParams, enabledFn dynamicproperties.BoolPropertyFn) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())
	wm := &WorkerManager{
		enabledFn:          enabledFn,
		serviceClient:      params.ServiceClient,
		frontendClient:     params.FrontendClient,
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
	}
	wm.createWorker = wm.defaultCreateWorker
	return wm
}

// Start begins the background loop that manages per-domain workers.
func (m *WorkerManager) Start() {
	m.logger.Info("scheduler worker manager starting")
	m.wg.Add(1)
	go m.run()
}

// Stop signals the background loop to stop and waits for it to finish.
// It then stops all active workers.
func (m *WorkerManager) Stop() {
	m.logger.Info("scheduler worker manager stopping")
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

	enabled := m.enabledFn()
	if enabled {
		m.refreshWorkers()
	} else {
		m.logger.Info("scheduler worker manager is disabled, skipping initial refresh")
	}

	for {
		select {
		case <-ticker.Chan():
			previouslyEnabled := enabled
			enabled = m.enabledFn()
			if enabled != previouslyEnabled {
				m.logger.Info("scheduler worker manager enabled state changed",
					tag.Dynamic("enabled", enabled),
				)
			}

			if enabled {
				m.refreshWorkers()
			} else {
				m.stopAllWorkers()
			}
		case <-m.ctx.Done():
			m.logger.Info("scheduler worker manager background loop stopped")
			return
		}
	}
}

// refreshWorkers scans all domains and reconciles the set of active workers
// with the domains this host owns via the membership hashring.
func (m *WorkerManager) refreshWorkers() {
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

		owner, err := m.membershipResolver.Lookup(service.Worker, domainName)
		if err != nil {
			m.logger.Warn("failed to look up domain owner, skipping",
				tag.WorkflowDomainName(domainName),
				tag.Error(err),
			)
			lookupFailed[domainName] = struct{}{}
			continue
		}

		if owner.Identity() != m.hostInfo.Identity() {
			continue
		}

		ownedDomains[domainName] = struct{}{}

		if _, exists := m.activeWorkers[domainName]; exists {
			continue
		}

		m.startWorkerForDomain(domainName)
	}

	for domainName, w := range m.activeWorkers {
		if _, owned := ownedDomains[domainName]; owned {
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
		delete(m.activeWorkers, domainName)
	}

	m.logger.Debug("scheduler workers refreshed",
		tag.Dynamic("active-worker-count", len(m.activeWorkers)),
	)
}

func (m *WorkerManager) startWorkerForDomain(domainName string) {
	w, err := m.createWorker(domainName)
	if err != nil {
		m.logger.Error("failed to start scheduler worker for domain",
			tag.WorkflowDomainName(domainName),
			tag.Error(err),
		)
		return
	}

	m.activeWorkers[domainName] = w
	m.logger.Info("started scheduler worker for domain",
		tag.WorkflowDomainName(domainName),
	)
}

func (m *WorkerManager) defaultCreateWorker(domainName string) (workerHandle, error) {
	actCtx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
		FrontendClient: m.frontendClient,
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
