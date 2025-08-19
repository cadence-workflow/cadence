// Copyright (c) 2020 Uber Technologies, Inc.
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

package execution

import (
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func (c *contextImpl) emitLargeWorkflowShardIDStats(blobSize int64, oldHistoryCount int64, oldHistorySize int64, newHistoryCount int64) {
	if c.shard.GetConfig().EnableShardIDMetrics() {
		shardID := c.shard.GetShardID()

		blobSizeWarn := min(int64(c.shard.GetConfig().LargeShardHistoryBlobMetricThreshold()), int64(c.shard.GetConfig().BlobSizeLimitWarn(c.GetDomainName())))
		// check if blob size is larger than threshold in Dynamic config if so alert on it every time
		if blobSize > blobSizeWarn {
			c.logger.SampleInfo("Workflow writing a large blob", c.shard.GetConfig().SampleLoggingRate(), tag.WorkflowDomainName(c.GetDomainName()),
				tag.WorkflowID(c.workflowExecution.GetWorkflowID()), tag.ShardID(c.shard.GetShardID()), tag.WorkflowRunID(c.workflowExecution.GetRunID()))
			c.metricsClient.Scope(metrics.LargeExecutionBlobShardScope, metrics.ShardIDTag(shardID), metrics.DomainTag(c.GetDomainName())).IncCounter(metrics.LargeHistoryBlobCount)
		}

		historyCountWarn := min(int64(c.shard.GetConfig().LargeShardHistoryEventMetricThreshold()), int64(c.shard.GetConfig().HistoryCountLimitWarn(c.GetDomainName())))
		// check if the new history count is greater than our threshold and only count/log it once when it passes it
		// this seems to double count and I can't figure out why but should be ok to get a rough idea and identify bad actors
		if oldHistoryCount < historyCountWarn && newHistoryCount >= historyCountWarn {
			c.logger.Warn("Workflow history event count is reaching dangerous levels", tag.WorkflowDomainName(c.GetDomainName()),
				tag.WorkflowID(c.workflowExecution.GetWorkflowID()), tag.ShardID(c.shard.GetShardID()), tag.WorkflowRunID(c.workflowExecution.GetRunID()))
			c.metricsClient.Scope(metrics.LargeExecutionCountShardScope, metrics.ShardIDTag(shardID), metrics.DomainTag(c.GetDomainName())).IncCounter(metrics.LargeHistoryEventCount)
		}

		historySizeWarn := min(int64(c.shard.GetConfig().LargeShardHistorySizeMetricThreshold()), int64(c.shard.GetConfig().HistorySizeLimitWarn(c.GetDomainName())))
		// check if the new history size is greater than our threshold and only count/log it once when it passes it
		if oldHistorySize < historySizeWarn && c.stats.HistorySize >= historySizeWarn {
			c.logger.Warn("Workflow history event size is reaching dangerous levels", tag.WorkflowDomainName(c.GetDomainName()),
				tag.WorkflowID(c.workflowExecution.GetWorkflowID()), tag.ShardID(c.shard.GetShardID()), tag.WorkflowRunID(c.workflowExecution.GetRunID()))
			c.metricsClient.Scope(metrics.LargeExecutionSizeShardScope, metrics.ShardIDTag(shardID), metrics.DomainTag(c.GetDomainName())).IncCounter(metrics.LargeHistorySizeCount)
		}
	}
}

func emitWorkflowHistoryStats(
	metricsClient metrics.Client,
	domainName string,
	historySize int,
	historyCount int,
) {

	sizeScope := metricsClient.Scope(metrics.ExecutionSizeStatsScope, metrics.DomainTag(domainName))
	countScope := metricsClient.Scope(metrics.ExecutionCountStatsScope, metrics.DomainTag(domainName))

	sizeScope.RecordTimer(metrics.HistorySize, time.Duration(historySize))
	countScope.RecordTimer(metrics.HistoryCount, time.Duration(historyCount))
}

func emitWorkflowExecutionStats(
	metricsClient metrics.Client,
	domainName string,
	stats *persistence.MutableStateStats,
	executionInfoHistorySize int64,
) {

	if stats == nil {
		return
	}

	sizeScope := metricsClient.Scope(metrics.ExecutionSizeStatsScope, metrics.DomainTag(domainName))
	countScope := metricsClient.Scope(metrics.ExecutionCountStatsScope, metrics.DomainTag(domainName))

	sizeScope.RecordTimer(metrics.HistorySize, time.Duration(executionInfoHistorySize))
	sizeScope.RecordTimer(metrics.MutableStateSize, time.Duration(stats.MutableStateSize))
	sizeScope.RecordTimer(metrics.ExecutionInfoSize, time.Duration(stats.MutableStateSize))
	sizeScope.RecordTimer(metrics.ActivityInfoSize, time.Duration(stats.ActivityInfoSize))
	sizeScope.RecordTimer(metrics.TimerInfoSize, time.Duration(stats.TimerInfoSize))
	sizeScope.RecordTimer(metrics.ChildInfoSize, time.Duration(stats.ChildInfoSize))
	sizeScope.RecordTimer(metrics.SignalInfoSize, time.Duration(stats.SignalInfoSize))
	sizeScope.RecordTimer(metrics.BufferedEventsSize, time.Duration(stats.BufferedEventsSize))

	countScope.RecordTimer(metrics.ActivityInfoCount, time.Duration(stats.ActivityInfoCount))
	countScope.RecordTimer(metrics.TimerInfoCount, time.Duration(stats.TimerInfoCount))
	countScope.RecordTimer(metrics.ChildInfoCount, time.Duration(stats.ChildInfoCount))
	countScope.RecordTimer(metrics.SignalInfoCount, time.Duration(stats.SignalInfoCount))
	countScope.RecordTimer(metrics.RequestCancelInfoCount, time.Duration(stats.RequestCancelInfoCount))
	countScope.RecordTimer(metrics.BufferedEventsCount, time.Duration(stats.BufferedEventsCount))
}

func emitSessionUpdateStats(
	metricsClient metrics.Client,
	domainName string,
	stats *persistence.MutableStateUpdateSessionStats,
) {

	if stats == nil {
		return
	}

	sizeScope := metricsClient.Scope(metrics.SessionSizeStatsScope, metrics.DomainTag(domainName))
	countScope := metricsClient.Scope(metrics.SessionCountStatsScope, metrics.DomainTag(domainName))

	sizeScope.RecordTimer(metrics.MutableStateSize, time.Duration(stats.MutableStateSize))
	sizeScope.RecordTimer(metrics.ExecutionInfoSize, time.Duration(stats.ExecutionInfoSize))
	sizeScope.RecordTimer(metrics.ActivityInfoSize, time.Duration(stats.ActivityInfoSize))
	sizeScope.RecordTimer(metrics.TimerInfoSize, time.Duration(stats.TimerInfoSize))
	sizeScope.RecordTimer(metrics.ChildInfoSize, time.Duration(stats.ChildInfoSize))
	sizeScope.RecordTimer(metrics.SignalInfoSize, time.Duration(stats.SignalInfoSize))
	sizeScope.RecordTimer(metrics.BufferedEventsSize, time.Duration(stats.BufferedEventsSize))

	countScope.RecordTimer(metrics.ActivityInfoCount, time.Duration(stats.ActivityInfoCount))
	countScope.RecordTimer(metrics.TimerInfoCount, time.Duration(stats.TimerInfoCount))
	countScope.RecordTimer(metrics.ChildInfoCount, time.Duration(stats.ChildInfoCount))
	countScope.RecordTimer(metrics.SignalInfoCount, time.Duration(stats.SignalInfoCount))
	countScope.RecordTimer(metrics.RequestCancelInfoCount, time.Duration(stats.RequestCancelInfoCount))
	countScope.RecordTimer(metrics.DeleteActivityInfoCount, time.Duration(stats.DeleteActivityInfoCount))
	countScope.RecordTimer(metrics.DeleteTimerInfoCount, time.Duration(stats.DeleteTimerInfoCount))
	countScope.RecordTimer(metrics.DeleteChildInfoCount, time.Duration(stats.DeleteChildInfoCount))
	countScope.RecordTimer(metrics.DeleteSignalInfoCount, time.Duration(stats.DeleteSignalInfoCount))
	countScope.RecordTimer(metrics.DeleteRequestCancelInfoCount, time.Duration(stats.DeleteRequestCancelInfoCount))
	countScope.RecordTimer(metrics.TransferTasksCount, time.Duration(stats.TaskCountByCategory[persistence.HistoryTaskCategoryTransfer]))
	countScope.RecordTimer(metrics.TimerTasksCount, time.Duration(stats.TaskCountByCategory[persistence.HistoryTaskCategoryTimer]))
	countScope.RecordTimer(metrics.ReplicationTasksCount, time.Duration(stats.TaskCountByCategory[persistence.HistoryTaskCategoryReplication]))
	countScope.IncCounter(metrics.UpdateWorkflowExecutionCount)
}

func emitWorkflowCompletionStats(
	metricsClient metrics.Client,
	logger log.Logger,
	domainName string,
	workflowType string,
	workflowID string,
	runID string,
	taskList string,
	event *types.HistoryEvent,
) {

	if event.EventType == nil {
		return
	}

	scope := metricsClient.Scope(
		metrics.WorkflowCompletionStatsScope,
		metrics.DomainTag(domainName),
		metrics.WorkflowTypeTag(workflowType),
		metrics.TaskListTag(taskList),
	)

	switch *event.EventType {
	case types.EventTypeWorkflowExecutionCompleted:
		scope.IncCounter(metrics.WorkflowSuccessCount)
	case types.EventTypeWorkflowExecutionCanceled:
		scope.IncCounter(metrics.WorkflowCancelCount)
	case types.EventTypeWorkflowExecutionFailed:
		scope.IncCounter(metrics.WorkflowFailedCount)
	case types.EventTypeWorkflowExecutionTimedOut:
		scope.IncCounter(metrics.WorkflowTimeoutCount)
		logger.Info("workflow execution timed out",
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.WorkflowDomainName(domainName),
		)
	case types.EventTypeWorkflowExecutionTerminated:
		scope.IncCounter(metrics.WorkflowTerminateCount)
		logger.Info("workflow terminated",
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.WorkflowDomainName(domainName),
		)
	case types.EventTypeWorkflowExecutionContinuedAsNew:
		scope.IncCounter(metrics.WorkflowContinuedAsNew)
		logger.Debug("workflow continued as new",
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.WorkflowDomainName(domainName),
		)
	default:
		scope.IncCounter(metrics.WorkflowCompletedUnknownType)
		logger.Warn("Workflow completed with an unknown event type",
			tag.WorkflowEventType(event.EventType.String()),
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.WorkflowDomainName(domainName),
		)
	}
}
