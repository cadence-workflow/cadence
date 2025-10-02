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
	"runtime/debug"
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

type DomainIDToDomainFn func(id string) (*cache.DomainCacheEntry, error)

const (
	LookupNewWorkflowOpName       = "LookupNewWorkflow"
	LookupWorkflowOpName          = "LookupWorkflow"
	LookupClusterOpName           = "LookupCluster"
	DomainIDToDomainFnErrorReason = "domain_id_to_name_fn_error"

	workflowPolicyCacheTTL      = 10 * time.Second
	workflowPolicyCacheMaxCount = 1000
)

type managerImpl struct {
	domainIDToDomainFn       DomainIDToDomainFn
	clusterMetadata          cluster.Metadata
	metricsCl                metrics.Client
	logger                   log.Logger
	executionManagerProvider ExecutionManagerProvider
	timeSrc                  clock.TimeSource
	numShards                int
	workflowPolicyCache      cache.Cache
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
	executionManagerProvider ExecutionManagerProvider,
	numShards int,
	opts ...ManagerOption,
) (Manager, error) {
	m := &managerImpl{
		domainIDToDomainFn:       domainIDToDomainFn,
		clusterMetadata:          clusterMetadata,
		metricsCl:                metricsCl,
		logger:                   logger.WithTags(tag.ComponentActiveClusterManager),
		timeSrc:                  clock.NewRealTimeSource(),
		executionManagerProvider: executionManagerProvider,
		numShards:                numShards,
		workflowPolicyCache: cache.New(&cache.Options{
			TTL:           workflowPolicyCacheTTL,
			MaxCount:      workflowPolicyCacheMaxCount,
			ActivelyEvict: true,
			Pin:           false,
			MetricsScope:  metricsCl.Scope(metrics.ActiveClusterManagerWorkflowCacheScope),
			Logger:        logger,
		}),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m, nil
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

	return nil, fmt.Errorf("unsupported active cluster selection strategy: %s", policy.GetStrategy())
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
			if m.logger.DebugOn() {
				m.logger.Debug("LookupWorkflow: domain migrated from active-passive to active-active case. returning ActiveClusterName from domain entry",
					tag.WorkflowDomainID(domainID),
					tag.WorkflowID(wfID),
					tag.WorkflowRunID(rID),
					tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
					tag.FailoverVersion(d.GetFailoverVersion()),
					tag.Dynamic("stack", string(debug.Stack())),
				)
			}
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

		if m.logger.DebugOn() {
			m.logger.Debug("LookupWorkflow: workflow is retired. returning region, cluster name and failover version",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(wfID),
				tag.WorkflowRunID(rID),
				tag.Region(region),
				tag.ActiveClusterName(cluster.ActiveClusterName),
				tag.FailoverVersion(cluster.FailoverVersion),
				tag.Dynamic("stack", string(debug.Stack())),
			)
		}
		return &LookupResult{
			Region:          region,
			ClusterName:     cluster.ActiveClusterName,
			FailoverVersion: cluster.FailoverVersion,
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

	if m.logger.DebugOn() {
		m.logger.Debug("LookupWorkflow: workflow is region sticky. returning region, cluster name and failover version",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.Region(region),
			tag.ActiveClusterName(cluster.ActiveClusterName),
			tag.FailoverVersion(cluster.FailoverVersion),
			tag.Dynamic("stack", string(debug.Stack())),
		)
	}
	return &LookupResult{
		Region:          region,
		ClusterName:     cluster.ActiveClusterName,
		FailoverVersion: cluster.FailoverVersion,
	}, nil
}

func (m *managerImpl) CurrentRegion() string {
	return m.clusterMetadata.GetCurrentRegion()
}

func (m *managerImpl) getClusterSelectionPolicy(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterSelectionPolicy, error) {
	// Check if the policy is already in the cache. create a key from domainID, wfID, rID
	key := fmt.Sprintf("%s:%s:%s", domainID, wfID, rID)
	cacheData := m.workflowPolicyCache.Get(key)
	if cacheData != nil {
		plcy, ok := cacheData.(*types.ActiveClusterSelectionPolicy)
		if ok {
			return plcy, nil
		}

		// This should never happen. If it does, we ignore cache value and get it from DB.
		m.logger.Warn(fmt.Sprintf("Cache data for key %s is of type %T, not a *types.ActiveClusterSelectionPolicy", key, cacheData))
	}

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

	m.workflowPolicyCache.Put(key, plcy)
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
