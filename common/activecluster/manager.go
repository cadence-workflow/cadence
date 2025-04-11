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
	"sync"
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type DomainIDToDomainFn func(id string) (*cache.DomainCacheEntry, error)

type manager struct {
	domainIDToDomainFn DomainIDToDomainFn
	clusterMetadata    cluster.Metadata
	metricsCl          metrics.Client
	logger             log.Logger
	ctx                context.Context
	cancel             context.CancelFunc

	// TODO: fakes to be remove
	wf1StartTime  time.Time
	wf1FailedOver int32

	changeCallbacksLock sync.RWMutex
	changeCallbacks     map[int]func(ChangeType)
}

func NewManager(
	domainIDToDomainFn DomainIDToDomainFn,
	clusterMetadata cluster.Metadata,
	metricsCl metrics.Client,
	logger log.Logger,
) Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		domainIDToDomainFn: domainIDToDomainFn,
		clusterMetadata:    clusterMetadata,
		metricsCl:          metricsCl,
		logger:             logger.WithTags(tag.ComponentActiveClusterManager),
		ctx:                ctx,
		cancel:             cancel,
		changeCallbacks:    make(map[int]func(ChangeType)),
	}
}

func (m *manager) Start() {
}

func (m *manager) Stop() {
	m.cancel()
}

func (m *manager) LookupExternalEntity(ctx context.Context, entityType, entityKey string) (*LookupResult, error) {
	// TODO: implement this
	return nil, errors.New("not implemented")
}

func (m *manager) LookupExternalEntityOfNewWorkflow(ctx context.Context, req *types.HistoryStartWorkflowExecutionRequest) (*LookupResult, error) {
	d, err := m.domainIDToDomainFn(req.DomainUUID)
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

	wfID := req.StartRequest.WorkflowID
	return m.fakeLookupWorkflow(wfID)
}

func (m *manager) LookupWorkflow(ctx context.Context, domainID, wfID, rID string) (*LookupResult, error) {
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

	return m.fakeLookupWorkflow(wfID)
}

func (m *manager) LookupFailoverVersion(failoverVersion int64, domainID string) (*LookupResult, error) {
	d, err := m.domainIDToDomainFn(domainID)
	if err != nil {
		return nil, err
	}

	if !d.GetReplicationConfig().IsActiveActive() {
		cluster, err := m.clusterMetadata.ClusterNameForFailoverVersion(failoverVersion)
		if err != nil {
			return nil, err
		}
		return &LookupResult{
			ClusterName:     cluster,
			FailoverVersion: failoverVersion,
		}, nil
	}

	// For active-active domains, the failover version might be mapped to a cluster or a region
	// First check if it maps to a cluster
	cluster, err := m.clusterMetadata.ClusterNameForFailoverVersion(failoverVersion)
	if err == nil {
		return &LookupResult{
			ClusterName:     cluster,
			FailoverVersion: failoverVersion,
			Region:          m.regionOfCluster(cluster),
		}, nil
	}

	// Check if it maps to a region.
	region, err := m.clusterMetadata.RegionForFailoverVersion(failoverVersion)
	if err != nil {
		return nil, err
	}

	// Now we know the region, find the cluster in the domain's active cluster list which belongs to the region
	enabledClusters := m.clusterMetadata.GetEnabledClusterInfo()
	for _, c := range d.GetReplicationConfig().ActiveClusters {
		cl, ok := enabledClusters[c.ClusterName]
		if !ok {
			continue
		}
		if cl.Region == region {
			return &LookupResult{
				ClusterName:     c.ClusterName,
				Region:          region,
				FailoverVersion: failoverVersion,
			}, nil
		}
	}

	return nil, errors.New("could not find cluster in the domain's active cluster list which belongs to the region")
}

func (m *manager) RegisterChangeCallback(shardID int, callback func(ChangeType)) {
	m.changeCallbacksLock.Lock()
	defer m.changeCallbacksLock.Unlock()

	m.changeCallbacks[shardID] = callback
}

func (m *manager) UnregisterChangeCallback(shardID int) {
	m.changeCallbacksLock.Lock()
	defer m.changeCallbacksLock.Unlock()

	delete(m.changeCallbacks, shardID)
}

func (m *manager) notifyChangeCallbacks(changeType ChangeType) {
	m.changeCallbacksLock.RLock()
	defer m.changeCallbacksLock.RUnlock()

	for _, callback := range m.changeCallbacks {
		callback(changeType)
	}
}

// regionOfCluster returns the region of a cluster as defined in cluster metadata. May return empty if cluster is not found or have no region.
func (m *manager) regionOfCluster(cluster string) string {
	return m.clusterMetadata.GetAllClusterInfo()[cluster].Region
}
