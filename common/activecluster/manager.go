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

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
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

type managerImpl struct {
	domainIDToDomainFn      DomainIDToDomainFn
	clusterMetadata         cluster.Metadata
	metricsCl               metrics.Client
	logger                  log.Logger
	ctx                     context.Context
	cancel                  context.CancelFunc
	wg                      sync.WaitGroup
	externalEntityProviders map[string]ExternalEntityProvider
	timeSrc                 clock.TimeSource

	shouldNotifyChangeCallbacks int32
	changeCallbacksLock         sync.Mutex
	changeCallbacks             map[int]func(ChangeType)

	// define some internal helper functions as member variables to be mocked in tests
	getWorkflowActivenessMetadataFn func(ctx context.Context, domainID, wfID, rID string) (*WorkflowActivenessMetadata, error)
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
	opts ...ManagerOption,
) (Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &managerImpl{
		domainIDToDomainFn:      domainIDToDomainFn,
		clusterMetadata:         clusterMetadata,
		metricsCl:               metricsCl,
		logger:                  logger.WithTags(tag.ComponentActiveClusterManager),
		ctx:                     ctx,
		cancel:                  cancel,
		changeCallbacks:         make(map[int]func(ChangeType)),
		externalEntityProviders: make(map[string]ExternalEntityProvider),
		timeSrc:                 clock.NewRealTimeSource(),
	}

	for _, opt := range opts {
		opt(m)
	}

	for _, provider := range externalEntityProviders {
		if _, ok := m.externalEntityProviders[provider.SupportedSource()]; ok {
			return nil, fmt.Errorf("external entity provider for source %s already registered", provider.SupportedSource())
		}
		m.externalEntityProviders[provider.SupportedSource()] = provider
	}

	m.getWorkflowActivenessMetadataFn = m.getWorkflowActivenessMetadata
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
	logger := m.logger.WithTags(tag.Dynamic("entity-source", provider.SupportedSource()))
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

func (m *managerImpl) FailoverVersionOfNewWorkflow(ctx context.Context, req *types.HistoryStartWorkflowExecutionRequest) (int64, error) {
	if req == nil {
		return 0, errors.New("request is nil")
	}

	d, err := m.domainIDToDomainFn(req.DomainUUID)
	if err != nil {
		return 0, err
	}

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return failover version of the domain entry
		return d.GetFailoverVersion(), nil
	}

	if req.StartRequest == nil {
		return 0, errors.New("start request is nil")
	}

	entitySource, entityKey, ok := m.getExternalEntitySourceAndKeyFromHeaders(req.StartRequest.Header)

	// If external entity headers are provided, return failover version of the external entity
	if ok {
		externalEntity, err := m.getExternalEntity(ctx, entitySource, entityKey)
		if err != nil {
			return 0, err
		}

		return externalEntity.FailoverVersion, nil
	}

	// If external entity headers are not provided, consider it as region sticky workflow.
	// Return failover version of the active cluster in current region.
	region := m.clusterMetadata.GetCurrentRegion()
	cluster, ok := d.GetReplicationConfig().ActiveClusters.RegionToClusterMap[region]
	if !ok {
		return 0, newRegionNotFoundForDomainError(region, req.DomainUUID)
	}

	return cluster.FailoverVersion, nil
}

func (m *managerImpl) LookupWorkflow(ctx context.Context, domainID, wfID, rID string) (*LookupResult, error) {
	d, err := m.domainIDToDomainFn(domainID)
	if err != nil {
		return nil, err
	}

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return ActiveClusterName from domain entry
		return &LookupResult{
			ClusterName:     d.GetReplicationConfig().ActiveClusterName,
			FailoverVersion: d.GetFailoverVersion(),
		}, nil
	}

	activenessMetadata, err := m.getWorkflowActivenessMetadataFn(ctx, domainID, wfID, rID)
	if err != nil {
		var notExistsErr *types.EntityNotExistsError
		if errors.As(err, &notExistsErr) {
			// Case 1.b: domain migrated from active-passive to active-active case
			return &LookupResult{
				ClusterName:     d.GetReplicationConfig().ActiveClusterName,
				FailoverVersion: d.GetFailoverVersion(),
			}, nil
		}

		return nil, err
	}

	region := ""
	if activenessMetadata.Type == WorkflowActivenessTypeRegionSticky {
		// Case 2.a: workflow is region sticky
		region = activenessMetadata.Region
	} else if activenessMetadata.Type == WorkflowActivenessTypeExternalEntity {
		// Case 2.b: workflow has external entity
		externalEntity, err := m.getExternalEntity(ctx, activenessMetadata.EntitySource, activenessMetadata.EntityKey)
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

	cluster, ok := d.GetReplicationConfig().ActiveClusters.RegionToClusterMap[region]
	if !ok {
		return nil, newRegionNotFoundForDomainError(region, domainID)
	}

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
	cfg, ok := d.GetReplicationConfig().ActiveClusters.RegionToClusterMap[region]
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

func (m *managerImpl) getExternalEntity(ctx context.Context, entitySource, entityKey string) (*ExternalEntity, error) {
	provider, ok := m.externalEntityProviders[entitySource]
	if !ok {
		return nil, fmt.Errorf("external entity provider for source %q not found", entitySource)
	}

	return provider.GetExternalEntity(ctx, entityKey)
}

func (m *managerImpl) getExternalEntitySourceAndKeyFromHeaders(header *types.Header) (string, string, bool) {
	if header == nil || len(header.Fields) == 0 {
		return "", "", false
	}

	entityType, ok := header.Fields[constants.ActiveActiveEntityTypeHeaderKey]
	if !ok {
		return "", "", false
	}

	entityKey, ok := header.Fields[constants.ActiveActiveEntityKeyHeaderKey]
	if !ok {
		return "", "", false
	}

	return string(entityType), string(entityKey), true
}

func (m *managerImpl) getWorkflowActivenessMetadata(ctx context.Context, domainID, wfID, rID string) (*WorkflowActivenessMetadata, error) {
	// TODO(active-active): Fetch ActivenessMetadata from persistence
	return nil, errors.New("not implemented")
}
