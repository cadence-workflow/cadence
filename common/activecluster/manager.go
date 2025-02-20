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
	startTime          time.Time
}

func NewManager(
	domainIDToDomainFn DomainIDToDomainFn,
	clusterMetadata cluster.Metadata,
	metricsCl metrics.Client,
	logger log.Logger,
) Manager {
	return &manager{
		domainIDToDomainFn: domainIDToDomainFn,
		clusterMetadata:    clusterMetadata,
		metricsCl:          metricsCl,
		logger:             logger.WithTags(tag.ComponentActiveClusterManager),
		startTime:          time.Now(),
	}
}

func (m *manager) Start() {
	// TODO: implement this
}

func (m *manager) Stop() {
	// TODO: implement this
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
	return m.helperToBeRemoved(wfID)
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

	return m.helperToBeRemoved(wfID)
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

// regionOfCluster returns the region of a cluster as defined in cluster metadata. May return empty if cluster is not found or have no region.
func (m *manager) regionOfCluster(cluster string) string {
	return m.clusterMetadata.GetAllClusterInfo()[cluster].Region
}

func (m *manager) helperToBeRemoved(wfID string) (*LookupResult, error) {
	// TODO: Remove below fake implementation and implement properly
	// - lookup active region given <domain id, wf id, run id> from executions table RowType=ActiveCluster.
	// - cache this info
	// - add metrics for cache hit/miss
	// - return cluster name

	// Fake logic:
	// - wf1 is active in cluster0 for first 60 seconds, then active in cluster1.
	// 		Note: Simulation sleeps for 30s in the beginning and runs wf1 for 60s. So wf1 should start in cluster0 and complete in cluster1.
	// - other workflows are always active in cluster1
	if wfID == "wf1" && time.Since(m.startTime) < 60*time.Second {
		m.logger.Debug("Returning cluster0 for wf1")
		return &LookupResult{
			Region:          "region0",
			ClusterName:     "cluster0",
			FailoverVersion: 1,
		}, nil
	}

	if wfID == "wf1" {
		m.logger.Debug("Returning cluster1 for wf1")
	}

	return &LookupResult{
		Region:          "region1",
		ClusterName:     "cluster1",
		FailoverVersion: 2,
	}, nil
}
