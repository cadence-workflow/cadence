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

package metered

import (
	"errors"
	"time"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type base struct {
	metricClient                  metrics.Client
	logger                        log.Logger
	enableLatencyHistogramMetrics bool
	sampleLoggingRate             dynamicconfig.IntPropertyFn
	enableShardIDMetrics          dynamicconfig.BoolPropertyFn
}

func (p *base) updateErrorMetricPerDomain(scope int, err error, scopeWithDomainTag metrics.Scope) {
	switch {
	case errors.As(err, new(*types.DomainAlreadyExistsError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrDomainAlreadyExistsCounterPerDomain)
	case errors.As(err, new(*types.BadRequestError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrBadRequestCounterPerDomain)
	case errors.As(err, new(*persistence.WorkflowExecutionAlreadyStartedError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrExecutionAlreadyStartedCounterPerDomain)
	case errors.As(err, new(*persistence.ConditionFailedError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrConditionFailedCounterPerDomain)
	case errors.As(err, new(*persistence.CurrentWorkflowConditionFailedError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrCurrentWorkflowConditionFailedCounterPerDomain)
	case errors.As(err, new(*persistence.ShardAlreadyExistError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrShardExistsCounterPerDomain)
	case errors.As(err, new(*persistence.ShardOwnershipLostError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrShardOwnershipLostCounterPerDomain)
	case errors.As(err, new(*types.EntityNotExistsError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrEntityNotExistsCounterPerDomain)
	case errors.As(err, new(*persistence.DuplicateRequestError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrDuplicateRequestCounterPerDomain)
	case errors.As(err, new(*persistence.TimeoutError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrTimeoutCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.PersistenceFailuresPerDomain)
	case errors.As(err, new(*types.ServiceBusyError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrBusyCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.PersistenceFailuresPerDomain)
	case errors.As(err, new(*persistence.DBUnavailableError)):
		scopeWithDomainTag.IncCounter(metrics.PersistenceErrDBUnavailableCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.PersistenceFailuresPerDomain)
		p.logger.Error("DBUnavailable Error:", tag.Error(err), tag.MetricScope(scope))
	default:
		p.logger.Error("Operation failed with internal error.", tag.Error(err), tag.MetricScope(scope))
		scopeWithDomainTag.IncCounter(metrics.PersistenceFailuresPerDomain)
	}
}

func (p *base) updateErrorMetric(scope int, err error, metricsScope metrics.Scope) {
	switch {
	case errors.As(err, new(*types.DomainAlreadyExistsError)):
		metricsScope.IncCounter(metrics.PersistenceErrDomainAlreadyExistsCounter)
	case errors.As(err, new(*types.BadRequestError)):
		metricsScope.IncCounter(metrics.PersistenceErrBadRequestCounter)
	case errors.As(err, new(*persistence.WorkflowExecutionAlreadyStartedError)):
		metricsScope.IncCounter(metrics.PersistenceErrExecutionAlreadyStartedCounter)
	case errors.As(err, new(*persistence.ConditionFailedError)):
		metricsScope.IncCounter(metrics.PersistenceErrConditionFailedCounter)
	case errors.As(err, new(*persistence.CurrentWorkflowConditionFailedError)):
		metricsScope.IncCounter(metrics.PersistenceErrCurrentWorkflowConditionFailedCounter)
	case errors.As(err, new(*persistence.ShardAlreadyExistError)):
		metricsScope.IncCounter(metrics.PersistenceErrShardExistsCounter)
	case errors.As(err, new(*persistence.ShardOwnershipLostError)):
		metricsScope.IncCounter(metrics.PersistenceErrShardOwnershipLostCounter)
	case errors.As(err, new(*types.EntityNotExistsError)):
		metricsScope.IncCounter(metrics.PersistenceErrEntityNotExistsCounter)
	case errors.As(err, new(*persistence.DuplicateRequestError)):
		metricsScope.IncCounter(metrics.PersistenceErrDuplicateRequestCounter)
	case errors.As(err, new(*persistence.TimeoutError)):
		metricsScope.IncCounter(metrics.PersistenceErrTimeoutCounter)
		metricsScope.IncCounter(metrics.PersistenceFailures)
	case errors.As(err, new(*types.ServiceBusyError)):
		metricsScope.IncCounter(metrics.PersistenceErrBusyCounter)
		metricsScope.IncCounter(metrics.PersistenceFailures)
	case errors.As(err, new(*persistence.DBUnavailableError)):
		metricsScope.IncCounter(metrics.PersistenceErrDBUnavailableCounter)
		metricsScope.IncCounter(metrics.PersistenceFailures)
		p.logger.Error("DBUnavailable Error:", tag.Error(err), tag.MetricScope(scope))
	default:
		p.logger.Error("Operation failed with internal error.", tag.Error(err), tag.MetricScope(scope))
		metricsScope.IncCounter(metrics.PersistenceFailures)
	}
}

func (p *base) call(scope int, op func() error, tags ...metrics.Tag) error {
	metricsScope := p.metricClient.Scope(scope, tags...)
	if len(tags) > 0 {
		metricsScope.IncCounter(metrics.PersistenceRequestsPerDomain)
	} else {
		metricsScope.IncCounter(metrics.PersistenceRequests)
	}
	before := time.Now()
	err := op()
	duration := time.Since(before)
	if len(tags) > 0 {
		metricsScope.RecordTimer(metrics.PersistenceLatencyPerDomain, duration)
	} else {
		metricsScope.RecordTimer(metrics.PersistenceLatency, duration)
	}

	if p.enableLatencyHistogramMetrics {
		metricsScope.RecordHistogramDuration(metrics.PersistenceLatencyHistogram, duration)
	}
	if err != nil {
		if len(tags) > 0 {
			p.updateErrorMetricPerDomain(scope, err, metricsScope)
		} else {
			p.updateErrorMetric(scope, err, metricsScope)
		}
	}
	return err
}

func (p *base) callWithDomainAndShardScope(scope int, op func() error, domainTag metrics.Tag, shardIDTag metrics.Tag) error {
	domainMetricsScope := p.metricClient.Scope(scope, domainTag)
	shardOperationsMetricsScope := p.metricClient.Scope(scope, shardIDTag)
	shardOverallMetricsScope := p.metricClient.Scope(metrics.PersistenceShardRequestCountScope, shardIDTag)

	domainMetricsScope.IncCounter(metrics.PersistenceRequestsPerDomain)
	shardOperationsMetricsScope.IncCounter(metrics.PersistenceRequestsPerShard)
	shardOverallMetricsScope.IncCounter(metrics.PersistenceRequestsPerShard)

	before := time.Now()
	err := op()
	duration := time.Since(before)

	domainMetricsScope.RecordTimer(metrics.PersistenceLatencyPerDomain, duration)
	shardOperationsMetricsScope.RecordTimer(metrics.PersistenceLatencyPerShard, duration)
	shardOverallMetricsScope.RecordTimer(metrics.PersistenceLatencyPerShard, duration)

	if p.enableLatencyHistogramMetrics {
		domainMetricsScope.RecordHistogramDuration(metrics.PersistenceLatencyHistogram, duration)
	}
	if err != nil {
		p.updateErrorMetricPerDomain(scope, err, domainMetricsScope)
	}
	return err
}

type lengther interface {
	Len() int
}

type taggedRequest interface {
	MetricTags() []metrics.Tag
}

type extraLogRequest interface {
	GetExtraLogTags() []tag.Tag
}

func (p *base) emptyMetric(methodName string, req any, res any, err error) {
	scope, ok := emptyCountedMethods[methodName]
	if !ok {
		// Method is not counted as empty.
		return
	}

	resLen, ok := res.(lengther)
	if !ok {
		return
	}

	if err != nil {
		return
	}

	metricScope := p.metricClient.Scope(scope.scope, getCustomMetricTags(req)...)

	if resLen.Len() == 0 {
		metricScope.IncCounter(metrics.PersistenceEmptyResponseCounter)
	} else {
		metricScope.RecordHistogramValue(metrics.PersistenceResponseRowSize, float64(resLen.Len()))
	}
}

var emptyCountedMethods = map[string]struct {
	scope int
}{
	"ExecutionManager.ListCurrentExecutions": {
		scope: metrics.PersistenceListCurrentExecutionsScope,
	},
	"ExecutionManager.GetTransferTasks": {
		scope: metrics.PersistenceGetTransferTasksScope,
	},
	"ExecutionManager.GetCrossClusterTasks": {
		scope: metrics.PersistenceGetCrossClusterTasksScope,
	},
	"ExecutionManager.GetReplicationTasks": {
		scope: metrics.PersistenceGetReplicationTasksScope,
	},
	"ExecutionManager.GetReplicationTasksFromDLQ": {
		scope: metrics.PersistenceGetReplicationTasksFromDLQScope,
	},
	"ExecutionManager.GetTimerIndexTasks": {
		scope: metrics.PersistenceGetTimerIndexTasksScope,
	},
	"TaskManager.GetTasks": {
		scope: metrics.PersistenceGetTasksScope,
	},
	"DomainManager.ListDomains": {
		scope: metrics.PersistenceListDomainsScope,
	},
	"HistoryManager.ReadHistoryBranch": {
		scope: metrics.PersistenceReadHistoryBranchScope,
	},
	"HistoryManager.GetAllHistoryTreeBranches": {
		scope: metrics.PersistenceGetAllHistoryTreeBranchesScope,
	},
	"QueueManager.ReadMessages": {
		scope: metrics.PersistenceReadMessagesScope,
	},
}

type domainTaggedRequest interface {
	GetDomainName() string
}

func getDomainNameFromRequest(req any) (res string, check bool) {
	d, check := req.(domainTaggedRequest)
	if check {
		res = d.GetDomainName()
	}
	return res, check
}

func getCustomLogTags(req any) (res []tag.Tag) {
	d, check := req.(extraLogRequest)
	if check {
		res = d.GetExtraLogTags()
	}
	return res
}

func getCustomMetricTags(req any) (res []metrics.Tag) {
	d, check := req.(taggedRequest)
	if check {
		res = d.MetricTags()
	}
	return res
}
