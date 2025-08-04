// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package activecluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const (
	// notifyChangeCallbacksInterval is the interval at which external entity change callbacks are notified to subscribers.
	// This is to avoid sending too many notifications and overwhelming the subscribers (i.e. per-shard history engines).
	notifyChangeCallbacksInterval = 5 * time.Second
)

type DomainIDToDomainFn func(id string) (*cache.DomainCacheEntry, error)

const (
	LookupNewWorkflowOpName       = "LookupNewWorkflow"
	LookupWorkflowOpName          = "LookupWorkflow"
	DomainIDToDomainFnErrorReason = "domain_id_to_name_fn_error"
)

type managerImpl struct {
	domainIDToDomainFn          DomainIDToDomainFn
	clusterMetadata             cluster.Metadata
	metricsCl                   metrics.Client
	logger                      log.Logger
	ctx                         context.Context
	cancel                      context.CancelFunc
	wg                          sync.WaitGroup
	externalEntityProviders     map[string]ExternalEntityProvider
	executionManagerProvider    ExecutionManagerProvider
	timeSrc                     clock.TimeSource
	numShards                   int
	shouldNotifyChangeCallbacks int32
	changeCallbacksLock         sync.Mutex
	changeCallbacks             map[int]func(ChangeType)
}

type ManagerOption func(*managerImpl)

func WithTimeSource(timeSource clock.TimeSource) ManagerOption {
	return func(m *managerImpl) {
		if timeSource != nil {
			m.timeSrc = timeSource
		}
	}
}

func NewManager(
	domainIDToDomainFn DomainIDToDomainFn,
	clusterMetadata cluster.Metadata,
	metricsCl metrics.Client,
	logger log.Logger,
	externalEntityProviders []ExternalEntityProvider,
	executionManagerProvider ExecutionManagerProvider,
	numShards int,
	opts ...ManagerOption,
) (Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &managerImpl{
		domainIDToDomainFn:       domainIDToDomainFn,
		clusterMetadata:          clusterMetadata,
		metricsCl:                metricsCl,
		logger:                   logger.WithTags(tag.ComponentActiveClusterManager),
		ctx:                      ctx,
		cancel:                   cancel,
		changeCallbacks:          make(map[int]func(ChangeType)),
		externalEntityProviders:  make(map[string]ExternalEntityProvider),
		timeSrc:                  clock.NewRealTimeSource(),
		executionManagerProvider: executionManagerProvider,
		numShards:                numShards,
	}

	for _, opt := range opts {
		opt(m)
	}

	for _, provider := range externalEntityProviders {
		if _, ok := m.externalEntityProviders[provider.SupportedType()]; ok {
			return nil, fmt.Errorf("external entity provider for type %s already registered", provider.SupportedType())
		}
		m.externalEntityProviders[provider.SupportedType()] = provider
	}

	return m, nil
}

func (m *managerImpl) Start() {
	for _, provider := range m.externalEntityProviders {
		m.wg.Add(1)
		go m.listenForExternalEntityChanges(provider)
	}

	m.wg.Add(1)
	go m.notifyChangeCallbacksPeriodically()
	m.logger.Info("Active cluster managerImpl started")
}

func (m *managerImpl) Stop() {
	m.logger.Info("Stopping active cluster managerImpl")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("Active cluster managerImpl stopped")
}

func (m *managerImpl) listenForExternalEntityChanges(provider ExternalEntityProvider) {
	defer m.wg.Done()
	logger := m.logger.WithTags(tag.Dynamic("entity-type", provider.SupportedType()))
	logger.Info("Listening for external entity changes")

	for {
		select {
		case <-m.ctx.Done():
			logger.Info("Stopping listener for external entity changes")
			return
		case changeType := <-provider.ChangeEvents():
			logger.Info("Received external entity change event", tag.Dynamic("change-type", changeType))
			atomic.StoreInt32(&m.shouldNotifyChangeCallbacks, 1)
		}
	}
}

func (m *managerImpl) notifyChangeCallbacksPeriodically() {
	defer m.wg.Done()

	t := m.timeSrc.NewTicker(notifyChangeCallbacksInterval)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("Stopping notify change callbacks periodically")
			return
		case <-t.Chan():
			if atomic.CompareAndSwapInt32(&m.shouldNotifyChangeCallbacks, 1, 0) {
				m.logger.Info("Notifying change callbacks")
				m.changeCallbacksLock.Lock()
				for shardID, callback := range m.changeCallbacks {
					m.logger.Info("Notifying change callback for shard", tag.ShardID(shardID))
					callback(ChangeTypeEntityMap)
					m.logger.Info("Notified change callback for shard", tag.ShardID(shardID))
				}
				m.changeCallbacksLock.Unlock()
				m.logger.Info("Notified change callbacks")
			} else {
				m.logger.Debug("Skipping notify change callbacks because there's no change since last notification")
			}
		}
	}
}

func (m *managerImpl) LookupNewWorkflow(ctx context.Context, domainID string, policy *types.ActiveClusterSelectionPolicy) (res *LookupResult, e error) {
	defer func() {
		logFn := m.logger.Debug
		if e != nil {
			logFn = m.logger.Warn
		}
		logFn("LookupNewWorkflow",
			tag.WorkflowDomainID(domainID),
			tag.Dynamic("policy", policy),
			tag.Dynamic("result", res),
			tag.Error(e),
		)
	}()

	d, scope, err := m.getDomainAndScope(domainID, LookupNewWorkflowOpName)
	if err != nil {
		return nil, err
	}
	scope = scope.Tagged(metrics.ActiveClusterSelectionStrategyTag(policy.GetStrategy().String()))
	defer m.handleError(scope, &e, time.Now())

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return failover version of the domain entry
		return &LookupResult{
			ClusterName:     d.GetReplicationConfig().ActiveClusterName,
			FailoverVersion: d.GetFailoverVersion(),
		}, nil
	}

	// region sticky policy
	if policy.GetStrategy() == types.ActiveClusterSelectionStrategyRegionSticky {
		// use current region for nil policy, otherwise use sticky region from policy
		region := m.clusterMetadata.GetCurrentRegion()
		if policy != nil {
			region = policy.StickyRegion
		}

		cluster, ok := d.GetReplicationConfig().ActiveClusters.ActiveClustersByRegion[region]
		if !ok {
			return nil, newRegionNotFoundForDomainError(region, domainID)
		}

		return &LookupResult{
			ClusterName:     cluster.ActiveClusterName,
			FailoverVersion: cluster.FailoverVersion,
		}, nil
	}

	if policy.GetStrategy() != types.ActiveClusterSelectionStrategyExternalEntity {
		return nil, fmt.Errorf("unsupported active cluster selection strategy: %s", policy.GetStrategy())
	}

	// find cluster name & failover version of the external entity
	externalEntity, err := m.getExternalEntity(ctx, policy.ExternalEntityType, policy.ExternalEntityKey)
	if err != nil {
		return nil, err
	}
	cluster, err := m.ClusterNameForFailoverVersion(externalEntity.FailoverVersion, domainID)
	if err != nil {
		return nil, err
	}

	return &LookupResult{
		Region:          externalEntity.Region,
		ClusterName:     cluster,
		FailoverVersion: externalEntity.FailoverVersion,
	}, nil
}

func (m *managerImpl) LookupWorkflow(ctx context.Context, domainID, wfID, rID string) (res *LookupResult, e error) {
	d, scope, err := m.getDomainAndScope(domainID, LookupWorkflowOpName)
	if err != nil {
		return nil, err
	}
	defer m.handleError(scope, &e, time.Now())

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return ActiveClusterName from domain entry
		m.logger.Debug("LookupWorkflow: not an active-active domain. returning ActiveClusterName from domain entry",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
			tag.FailoverVersion(d.GetFailoverVersion()),
		)
		return &LookupResult{
			ClusterName:     d.GetReplicationConfig().ActiveClusterName,
			FailoverVersion: d.GetFailoverVersion(),
		}, nil
	}

	plcy, err := m.getClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		var notExistsErr *types.EntityNotExistsError
		if !errors.As(err, &notExistsErr) {
			return nil, err
		}

		// Case 1.b: domain migrated from active-passive to active-active case
		if d.GetReplicationConfig().ActiveClusterName != "" {
			m.logger.Debug("LookupWorkflow: domain migrated from active-passive to active-active case. returning ActiveClusterName from domain entry",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(wfID),
				tag.WorkflowRunID(rID),
				tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
				tag.FailoverVersion(d.GetFailoverVersion()),
			)
			return &LookupResult{
				ClusterName:     d.GetReplicationConfig().ActiveClusterName,
				FailoverVersion: d.GetFailoverVersion(),
			}, nil
		}

		// Case 1.c: workflow is retired. return current cluster and its failover version
		region := m.clusterMetadata.GetCurrentRegion()
		cluster, ok := d.GetReplicationConfig().ActiveClusters.ActiveClustersByRegion[region]
		if !ok {
			return nil, newRegionNotFoundForDomainError(region, domainID)
		}

		m.logger.Debug("LookupWorkflow: workflow is retired. returning region, cluster name and failover version",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.Region(region),
			tag.ActiveClusterName(cluster.ActiveClusterName),
			tag.FailoverVersion(cluster.FailoverVersion),
		)
		return &LookupResult{
			Region:          region,
			ClusterName:     cluster.ActiveClusterName,
			FailoverVersion: cluster.FailoverVersion,
		}, nil
	}

	if plcy.GetStrategy() == types.ActiveClusterSelectionStrategyExternalEntity {
		// Case 2.b: workflow has external entity
		externalEntity, err := m.getExternalEntity(ctx, plcy.ExternalEntityType, plcy.ExternalEntityKey)
		if err != nil {
			return nil, err
		}

		cluster, err := m.ClusterNameForFailoverVersion(externalEntity.FailoverVersion, domainID)
		if err != nil {
			return nil, err
		}

		m.logger.Debug("LookupWorkflow: workflow has external entity. returning region, cluster name and failover version",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.WorkflowExternalEntityType(plcy.ExternalEntityType),
			tag.WorkflowExternalEntityKey(plcy.ExternalEntityKey),
			tag.Region(externalEntity.Region),
			tag.ActiveClusterName(cluster),
			tag.FailoverVersion(externalEntity.FailoverVersion),
		)
		return &LookupResult{
			Region:          externalEntity.Region,
			ClusterName:     cluster,
			FailoverVersion: externalEntity.FailoverVersion,
		}, nil
	}

	if plcy.GetStrategy() != types.ActiveClusterSelectionStrategyRegionSticky {
		return nil, fmt.Errorf("unsupported active cluster selection strategy: %s", plcy.GetStrategy())
	}

	region := plcy.StickyRegion
	cluster, ok := d.GetReplicationConfig().ActiveClusters.ActiveClustersByRegion[region]
	if !ok {
		return nil, newRegionNotFoundForDomainError(region, domainID)
	}

	m.logger.Debug("LookupWorkflow: workflow is region sticky. returning region, cluster name and failover version",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(wfID),
		tag.WorkflowRunID(rID),
		tag.Region(region),
		tag.ActiveClusterName(cluster.ActiveClusterName),
		tag.FailoverVersion(cluster.FailoverVersion),
	)
	return &LookupResult{
		Region:          region,
		ClusterName:     cluster.ActiveClusterName,
		FailoverVersion: cluster.FailoverVersion,
	}, nil
}

func (m *managerImpl) ClusterNameForFailoverVersion(failoverVersion int64, domainID string) (string, error) {
	d, err := m.domainIDToDomainFn(domainID)
	if err != nil {
		return "", err
	}

	if !d.GetReplicationConfig().IsActiveActive() {
		cluster, err := m.clusterMetadata.ClusterNameForFailoverVersion(failoverVersion)
		if err != nil {
			return "", err
		}
		return cluster, nil
	}

	// For active-active domains, the failover version might be mapped to a cluster or a region
	// First check if it maps to a cluster
	cluster, err := m.clusterMetadata.ClusterNameForFailoverVersion(failoverVersion)
	if err == nil {
		// failover version belongs to a cluster.
		return cluster, nil
	}

	// Check if it maps to a region.
	region, err := m.clusterMetadata.RegionForFailoverVersion(failoverVersion)
	if err != nil {
		return "", err
	}

	// Now we know the region, find the cluster in the domain's active cluster list which belongs to the region
	cfg, ok := d.GetReplicationConfig().ActiveClusters.ActiveClustersByRegion[region]
	if !ok {
		return "", newRegionNotFoundForDomainError(region, domainID)
	}

	allClusters := m.clusterMetadata.GetAllClusterInfo()
	_, ok = allClusters[cfg.ActiveClusterName]
	if !ok {
		return "", newClusterNotFoundForRegionError(cfg.ActiveClusterName, region)
	}

	return cfg.ActiveClusterName, nil
}

func (m *managerImpl) RegisterChangeCallback(shardID int, callback func(ChangeType)) {
	m.changeCallbacksLock.Lock()
	defer m.changeCallbacksLock.Unlock()

	m.changeCallbacks[shardID] = callback
}

func (m *managerImpl) UnregisterChangeCallback(shardID int) {
	m.changeCallbacksLock.Lock()
	defer m.changeCallbacksLock.Unlock()

	delete(m.changeCallbacks, shardID)
}

func (m *managerImpl) SupportedExternalEntityType(entityType string) bool {
	_, ok := m.externalEntityProviders[entityType]
	return ok
}

func (m *managerImpl) CurrentRegion() string {
	return m.clusterMetadata.GetCurrentRegion()
}

func (m *managerImpl) getExternalEntity(ctx context.Context, entityType, entityKey string) (*ExternalEntity, error) {
	provider, ok := m.externalEntityProviders[entityType]
	if !ok {
		return nil, fmt.Errorf("external entity provider for type %q not found", entityType)
	}

	return provider.GetExternalEntity(ctx, entityKey)
}

func (m *managerImpl) getClusterSelectionPolicy(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterSelectionPolicy, error) {
	shardID := common.WorkflowIDToHistoryShard(wfID, m.numShards)
	executionManager, err := m.executionManagerProvider.GetExecutionManager(shardID)
	if err != nil {
		return nil, err
	}

	plcy, err := executionManager.GetActiveClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		return nil, err
	}

	if plcy == nil {
		return nil, &types.EntityNotExistsError{
			Message: "active cluster selection policy not found",
		}
	}

	return plcy, nil
}

func (m *managerImpl) getDomainAndScope(domainID, fn string) (*cache.DomainCacheEntry, metrics.Scope, error) {
	scope := m.metricsCl.Scope(metrics.ActiveClusterManager).Tagged(metrics.ActiveClusterLookupFnTag(fn))

	d, err := m.domainIDToDomainFn(domainID)
	if err != nil {
		scope = scope.Tagged(metrics.ReasonTag(DomainIDToDomainFnErrorReason))
		scope.IncCounter(metrics.ActiveClusterManagerLookupFailureCount)
		return nil, nil, err
	}

	scope = scope.Tagged(metrics.DomainTag(d.GetInfo().Name))
	scope = scope.Tagged(metrics.IsActiveActiveDomainTag(d.GetReplicationConfig().IsActiveActive()))
	scope.IncCounter(metrics.ActiveClusterManagerLookupRequestCount)

	return d, scope, nil
}

func (m *managerImpl) handleError(scope metrics.Scope, err *error, start time.Time) {
	if err != nil && *err != nil {
		scope.IncCounter(metrics.ActiveClusterManagerLookupFailureCount)
	} else {
		scope.IncCounter(metrics.ActiveClusterManagerLookupSuccessCount)
	}
	scope.RecordHistogramDuration(metrics.ActiveClusterManagerLookupLatency, time.Since(start))
}
