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

package task

import (
	"context"
	"fmt"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

type (
	mockTaskMatcher struct {
		task *MockTask
	}
)

// InitializeLoggerForTask creates a new logger with additional tags for task info
func InitializeLoggerForTask(
	shardID int,
	task persistence.Task,
	logger log.Logger,
) log.Logger {
	return logger.WithTags(
		tag.ShardID(shardID),
		tag.TaskID(task.GetTaskID()),
		tag.TaskVisibilityTimestamp(task.GetVisibilityTimestamp().UnixNano()),
		tag.FailoverVersion(task.GetVersion()),
		tag.TaskType(task.GetTaskType()),
		tag.WorkflowDomainID(task.GetDomainID()),
		tag.WorkflowID(task.GetWorkflowID()),
		tag.WorkflowRunID(task.GetRunID()),
	)
}

// GetTransferTaskMetricsScope returns the metrics scope index for transfer task
func GetTransferTaskMetricsScope(
	taskType int,
	isActive bool,
) int {
	switch taskType {
	case persistence.TransferTaskTypeActivityTask:
		if isActive {
			return metrics.TransferActiveTaskActivityScope
		}
		return metrics.TransferStandbyTaskActivityScope
	case persistence.TransferTaskTypeDecisionTask:
		if isActive {
			return metrics.TransferActiveTaskDecisionScope
		}
		return metrics.TransferStandbyTaskDecisionScope
	case persistence.TransferTaskTypeCloseExecution:
		if isActive {
			return metrics.TransferActiveTaskCloseExecutionScope
		}
		return metrics.TransferStandbyTaskCloseExecutionScope
	case persistence.TransferTaskTypeCancelExecution:
		if isActive {
			return metrics.TransferActiveTaskCancelExecutionScope
		}
		return metrics.TransferStandbyTaskCancelExecutionScope
	case persistence.TransferTaskTypeSignalExecution:
		if isActive {
			return metrics.TransferActiveTaskSignalExecutionScope
		}
		return metrics.TransferStandbyTaskSignalExecutionScope
	case persistence.TransferTaskTypeStartChildExecution:
		if isActive {
			return metrics.TransferActiveTaskStartChildExecutionScope
		}
		return metrics.TransferStandbyTaskStartChildExecutionScope
	case persistence.TransferTaskTypeRecordWorkflowStarted:
		if isActive {
			return metrics.TransferActiveTaskRecordWorkflowStartedScope
		}
		return metrics.TransferStandbyTaskRecordWorkflowStartedScope
	case persistence.TransferTaskTypeResetWorkflow:
		if isActive {
			return metrics.TransferActiveTaskResetWorkflowScope
		}
		return metrics.TransferStandbyTaskResetWorkflowScope
	case persistence.TransferTaskTypeUpsertWorkflowSearchAttributes:
		if isActive {
			return metrics.TransferActiveTaskUpsertWorkflowSearchAttributesScope
		}
		return metrics.TransferStandbyTaskUpsertWorkflowSearchAttributesScope
	case persistence.TransferTaskTypeRecordWorkflowClosed:
		if isActive {
			return metrics.TransferActiveTaskRecordWorkflowClosedScope
		}
		return metrics.TransferStandbyTaskRecordWorkflowClosedScope
	case persistence.TransferTaskTypeRecordChildExecutionCompleted:
		if isActive {
			return metrics.TransferActiveTaskRecordChildExecutionCompletedScope
		}
		return metrics.TransferStandbyTaskRecordChildExecutionCompletedScope
	case persistence.TransferTaskTypeApplyParentClosePolicy:
		if isActive {
			return metrics.TransferActiveTaskApplyParentClosePolicyScope
		}
		return metrics.TransferStandbyTaskApplyParentClosePolicyScope
	default:
		if isActive {
			return metrics.TransferActiveQueueProcessorScope
		}
		return metrics.TransferStandbyQueueProcessorScope
	}
}

// GetTimerTaskMetricScope returns the metrics scope index for timer task
func GetTimerTaskMetricScope(
	taskType int,
	isActive bool,
) int {
	switch taskType {
	case persistence.TaskTypeDecisionTimeout:
		if isActive {
			return metrics.TimerActiveTaskDecisionTimeoutScope
		}
		return metrics.TimerStandbyTaskDecisionTimeoutScope
	case persistence.TaskTypeActivityTimeout:
		if isActive {
			return metrics.TimerActiveTaskActivityTimeoutScope
		}
		return metrics.TimerStandbyTaskActivityTimeoutScope
	case persistence.TaskTypeUserTimer:
		if isActive {
			return metrics.TimerActiveTaskUserTimerScope
		}
		return metrics.TimerStandbyTaskUserTimerScope
	case persistence.TaskTypeWorkflowTimeout:
		if isActive {
			return metrics.TimerActiveTaskWorkflowTimeoutScope
		}
		return metrics.TimerStandbyTaskWorkflowTimeoutScope
	case persistence.TaskTypeDeleteHistoryEvent:
		if isActive {
			return metrics.TimerActiveTaskDeleteHistoryEventScope
		}
		return metrics.TimerStandbyTaskDeleteHistoryEventScope
	case persistence.TaskTypeActivityRetryTimer:
		if isActive {
			return metrics.TimerActiveTaskActivityRetryTimerScope
		}
		return metrics.TimerStandbyTaskActivityRetryTimerScope
	case persistence.TaskTypeWorkflowBackoffTimer:
		if isActive {
			return metrics.TimerActiveTaskWorkflowBackoffTimerScope
		}
		return metrics.TimerStandbyTaskWorkflowBackoffTimerScope
	default:
		if isActive {
			return metrics.TimerActiveQueueProcessorScope
		}
		return metrics.TimerStandbyQueueProcessorScope
	}
}

// verifyTaskVersion, will return true if failover version check is successful
func verifyTaskVersion(
	shard shard.Context,
	logger log.Logger,
	domainID string,
	version int64,
	taskVersion int64,
	task persistence.Task,
) (bool, error) {

	// the first return value is whether this task is valid for further processing
	domainEntry, err := shard.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		logger.Debug(fmt.Sprintf("Cannot find domainID: %v, err: %v.", domainID, err))
		return false, err
	}
	if !domainEntry.IsGlobalDomain() {
		logger.Debug(fmt.Sprintf("DomainID: %v is not active, task: %v version check pass", domainID, task))
		return true, nil
	} else if version != taskVersion {
		logger.Debug(fmt.Sprintf("DomainID: %v is active, task: %v version != target version: %v.", domainID, task, version))
		return false, nil
	}
	logger.Debug(fmt.Sprintf("DomainID: %v is active, task: %v version == target version: %v.", domainID, task, version))
	return true, nil
}

// load mutable state, if mutable state's next event ID <= task event ID, will attempt to refresh
// if still mutable state's next event ID <= task event ID, will return nil, nil
func loadMutableState(
	ctx context.Context,
	wfContext execution.Context,
	task persistence.Task,
	metricsScope metrics.Scope,
	logger log.Logger,
	eventID int64,
) (execution.MutableState, error) {
	msBuilder, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); ok {
			// this could happen if this is a duplicate processing of the task, and the execution has already completed.
			return nil, nil
		}
		return nil, err
	}
	executionInfo := msBuilder.GetExecutionInfo()

	// check to see if cache needs to be refreshed as we could potentially have stale workflow execution
	// the exception is decision consistently fail
	// there will be no event generated, thus making the decision schedule ID == next event ID
	isDecisionRetry := (task.GetTaskType() == persistence.TaskTypeDecisionTimeout || task.GetTaskType() == persistence.TransferTaskTypeDecisionTask) &&
		executionInfo.DecisionScheduleID == eventID &&
		executionInfo.DecisionAttempt > 0

	if eventID >= msBuilder.GetNextEventID() && !isDecisionRetry {
		metricsScope.IncCounter(metrics.StaleMutableStateCounter)
		wfContext.Clear()

		msBuilder, err = wfContext.LoadWorkflowExecution(ctx)
		if err != nil {
			return nil, err
		}
		// after refresh, still mutable state's next event ID <= task ID
		if eventID >= msBuilder.GetNextEventID() {
			domainName := msBuilder.GetDomainEntry().GetInfo().Name
			metricsScope.Tagged(metrics.DomainTag(domainName)).IncCounter(metrics.DataInconsistentCounter)
			logger.Error("Task Processor: task event ID >= MS NextEventID, skip.",
				tag.WorkflowDomainName(domainName),
				tag.WorkflowDomainID(task.GetDomainID()),
				tag.WorkflowID(task.GetWorkflowID()),
				tag.WorkflowRunID(task.GetRunID()),
				tag.TaskType(task.GetTaskType()),
				tag.TaskID(task.GetTaskID()),
				tag.WorkflowEventID(eventID),
				tag.WorkflowNextEventID(msBuilder.GetNextEventID()),
			)
			return nil, nil
		}
	}
	return msBuilder, nil
}

func timeoutWorkflow(
	mutableState execution.MutableState,
	eventBatchFirstEventID int64,
) error {

	if decision, ok := mutableState.GetInFlightDecision(); ok {
		if err := execution.FailDecision(
			mutableState,
			decision,
			types.DecisionTaskFailedCauseForceCloseDecision,
		); err != nil {
			return err
		}
	}

	_, err := mutableState.AddTimeoutWorkflowEvent(
		eventBatchFirstEventID,
	)
	return err
}

func retryWorkflow(
	ctx context.Context,
	mutableState execution.MutableState,
	eventBatchFirstEventID int64,
	parentDomainName string,
	continueAsNewAttributes *types.ContinueAsNewWorkflowExecutionDecisionAttributes,
) (execution.MutableState, error) {

	if decision, ok := mutableState.GetInFlightDecision(); ok {
		if err := execution.FailDecision(
			mutableState,
			decision,
			types.DecisionTaskFailedCauseForceCloseDecision,
		); err != nil {
			return nil, err
		}
	}

	_, newMutableState, err := mutableState.AddContinueAsNewEvent(
		ctx,
		eventBatchFirstEventID,
		constants.EmptyEventID,
		parentDomainName,
		continueAsNewAttributes,
	)
	if err != nil {
		return nil, err
	}
	return newMutableState, nil
}

func getWorkflowExecution(
	taskInfo persistence.Task,
) types.WorkflowExecution {

	return types.WorkflowExecution{
		WorkflowID: taskInfo.GetWorkflowID(),
		RunID:      taskInfo.GetRunID(),
	}
}

func shouldPushToMatching(
	ctx context.Context,
	shard shard.Context,
	taskInfo persistence.Task,
) (bool, error) {
	domainEntry, err := shard.GetDomainCache().GetDomainByID(taskInfo.GetDomainID())
	if err != nil {
		return false, err
	}

	if !domainEntry.GetReplicationConfig().IsActiveActive() {
		// Preserve the behavior of active-standby and local domains. Always push to matching
		return true, nil
	}

	// For active-active domains, only push to matching if the workflow is active in current cluster
	// We may revisit this logic in the future. Current idea is to not pollute tasklists with passive workflows of active-active domains
	// because they would cause head-of-line blocking in the tasklist. Passive task completion logic doesn't apply to active-active domains.
	lookupRes, err := shard.GetActiveClusterManager().LookupWorkflow(ctx, taskInfo.GetDomainID(), taskInfo.GetWorkflowID(), taskInfo.GetRunID())
	if err != nil {
		return false, err
	}
	if lookupRes.ClusterName != shard.GetClusterMetadata().GetCurrentClusterName() {
		return false, nil
	}

	return true, nil
}

// NewMockTaskMatcher creates a gomock matcher for mock Task
func NewMockTaskMatcher(mockTask *MockTask) gomock.Matcher {
	return &mockTaskMatcher{
		task: mockTask,
	}
}

func (m *mockTaskMatcher) Matches(x interface{}) bool {
	taskPtr, ok := x.(*MockTask)
	if !ok {
		return false
	}
	return taskPtr == m.task
}

func (m *mockTaskMatcher) String() string {
	return fmt.Sprintf("is equal to %v", m.task)
}
