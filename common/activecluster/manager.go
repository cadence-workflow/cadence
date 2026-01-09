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
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type DomainIDToDomainFn func(id string) (*cache.DomainCacheEntry, error)

const (
	LookupNewWorkflowOpName                                 = "LookupNewWorkflow"
	LookupWorkflowOpName                                    = "LookupWorkflow"
	GetActiveClusterInfoByClusterAttributeOpName            = "GetActiveClusterInfoByClusterAttribute"
	GetActiveClusterInfoByWorkflowOpName                    = "GetActiveClusterInfoByWorkflow"
	GetActiveClusterSelectionPolicyForWorkflowOpName        = "GetActiveClusterSelectionPolicyForWorkflow"
	GetActiveClusterSelectionPolicyForCurrentWorkflowOpName = "GetActiveClusterSelectionPolicyForCurrentWorkflow"
	DomainIDToDomainFnErrorReason                           = "domain_id_to_name_fn_error"

	workflowPolicyCacheTTL      = 10 * time.Second
	workflowPolicyCacheMaxCount = 1000
)

type managerImpl struct {
	domainIDToDomainFn       DomainIDToDomainFn
	metricsCl                metrics.Client
	logger                   log.Logger
	executionManagerProvider ExecutionManagerProvider
	numShards                int
	workflowPolicyCache      cache.Cache
}

type ManagerOption func(*managerImpl)

func NewManager(
	domainIDToDomainFn DomainIDToDomainFn,
	metricsCl metrics.Client,
	logger log.Logger,
	executionManagerProvider ExecutionManagerProvider,
	numShards int,
	opts ...ManagerOption,
) (Manager, error) {
	m := &managerImpl{
		domainIDToDomainFn:       domainIDToDomainFn,
		metricsCl:                metricsCl,
		logger:                   logger.WithTags(tag.ComponentActiveClusterManager),
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

func (m *managerImpl) getClusterSelectionPolicy(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterSelectionPolicy, error) {
	shardID := common.WorkflowIDToHistoryShard(wfID, m.numShards)
	executionManager, err := m.executionManagerProvider.GetExecutionManager(shardID)
	if err != nil {
		return nil, err
	}
	if rID == "" {
		execution, err := executionManager.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
			DomainID:   domainID,
			WorkflowID: wfID,
		})
		if err != nil {
			return nil, err
		}
		rID = execution.RunID
	}
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

func (m *managerImpl) GetActiveClusterInfoByClusterAttribute(ctx context.Context, domainID string, clusterAttribute *types.ClusterAttribute) (res *types.ActiveClusterInfo, e error) {
	defer func() {
		logFn := m.logger.Debug
		if e != nil {
			logFn = m.logger.Warn
		}
		logFn("GetActiveClusterInfoByClusterAttribute",
			tag.WorkflowDomainID(domainID),
			tag.Dynamic("clusterAttribute", clusterAttribute),
			tag.Dynamic("result", res),
			tag.Error(e),
		)
	}()

	d, scope, err := m.getDomainAndScope(domainID, GetActiveClusterInfoByClusterAttributeOpName)
	if err != nil {
		return nil, err
	}
	defer m.handleError(scope, &e, time.Now())

	res, ok := d.GetActiveClusterInfoByClusterAttribute(clusterAttribute)
	if !ok {
		return nil, &ClusterAttributeNotFoundError{
			DomainID:         domainID,
			ClusterAttribute: clusterAttribute,
		}
	}
	return res, nil
}

func (m *managerImpl) GetActiveClusterInfoByWorkflow(ctx context.Context, domainID, wfID, rID string) (res *types.ActiveClusterInfo, e error) {
	d, scope, err := m.getDomainAndScope(domainID, GetActiveClusterInfoByWorkflowOpName)
	if err != nil {
		return nil, err
	}
	defer m.handleError(scope, &e, time.Now())

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return ActiveClusterName from domain entry
		m.logger.Debug("GetActiveClusterInfoByWorkflow: not an active-active domain. returning ActiveClusterName from domain entry",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
			tag.FailoverVersion(d.GetFailoverVersion()),
		)
		return &types.ActiveClusterInfo{
			ActiveClusterName: d.GetReplicationConfig().ActiveClusterName,
			FailoverVersion:   d.GetFailoverVersion(),
		}, nil
	}

	policy, err := m.getClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		var notExistsErr *types.EntityNotExistsError
		if !errors.As(err, &notExistsErr) {
			return nil, err
		}
		policy = &types.ActiveClusterSelectionPolicy{}
	}
	res, ok := d.GetActiveClusterInfoByClusterAttribute(policy.ClusterAttribute)
	if !ok {
		m.logger.Debug("GetActiveClusterInfoByWorkflow: cluster attribute not found",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
			tag.FailoverVersion(d.GetFailoverVersion()),
		)
		return nil, &ClusterAttributeNotFoundError{
			DomainID:         domainID,
			ClusterAttribute: policy.ClusterAttribute,
		}
	}
	m.logger.Debug("GetActiveClusterInfoByWorkflow: returning active cluster info",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(wfID),
		tag.WorkflowRunID(rID),
		tag.ActiveClusterName(res.ActiveClusterName),
		tag.FailoverVersion(res.FailoverVersion),
	)

	return res, nil
}

func (m *managerImpl) GetActiveClusterSelectionPolicyForWorkflow(ctx context.Context, domainID, wfID, rID string) (res *types.ActiveClusterSelectionPolicy, e error) {
	d, scope, err := m.getDomainAndScope(domainID, GetActiveClusterSelectionPolicyForWorkflowOpName)
	if err != nil {
		return nil, err
	}
	defer m.handleError(scope, &e, time.Now())

	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return ActiveClusterName from domain entry
		m.logger.Debug("GetActiveClusterSelectionPolicyForWorkflow: not an active-active domain. returning ActiveClusterName from domain entry",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
			tag.WorkflowRunID(rID),
			tag.ActiveClusterName(d.GetReplicationConfig().ActiveClusterName),
			tag.FailoverVersion(d.GetFailoverVersion()),
		)
		return nil, nil
	}

	plcy, err := m.getClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		var notExistsErr *types.EntityNotExistsError
		if !errors.As(err, &notExistsErr) {
			return nil, err
		}
		return nil, nil
	}
	return plcy, nil
}

func (m *managerImpl) GetActiveClusterSelectionPolicyForCurrentWorkflow(ctx context.Context, domainID, wfID string) (res *types.ActiveClusterSelectionPolicy, running bool, e error) {
	d, scope, err := m.getDomainAndScope(domainID, GetActiveClusterSelectionPolicyForCurrentWorkflowOpName)
	if err != nil {
		return nil, false, err
	}
	defer m.handleError(scope, &e, time.Now())
	if !d.GetReplicationConfig().IsActiveActive() {
		// Not an active-active domain. return nil
		m.logger.Debug("GetActiveClusterSelectionPolicyForCurrentWorkflow: not an active-active domain. returning nil",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(wfID),
		)
		return nil, false, nil
	}

	shardID := common.WorkflowIDToHistoryShard(wfID, m.numShards)
	executionManager, err := m.executionManagerProvider.GetExecutionManager(shardID)
	if err != nil {
		return nil, false, err
	}
	execution, err := executionManager.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: wfID,
	})
	if err != nil {
		return nil, false, err
	}
	if persistence.IsWorkflowRunning(execution.State) {
		policy, err := m.getClusterSelectionPolicy(ctx, domainID, wfID, execution.RunID)
		if err != nil {
			return nil, false, err
		}
		return policy, true, nil
	}
	return nil, false, nil
}
