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
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/simulation"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	transferStandbyTaskExecutor struct {
		*transferTaskExecutorBase

		clusterName            string
		historyResender        ndc.HistoryResender
		getRemoteClusterNameFn func(context.Context, persistence.Task) (string, error)
	}
)

// NewTransferStandbyTaskExecutor creates a new task executor for standby transfer task
func NewTransferStandbyTaskExecutor(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	historyResender ndc.HistoryResender,
	logger log.Logger,
	clusterName string,
	config *config.Config,
) Executor {
	return &transferStandbyTaskExecutor{
		transferTaskExecutorBase: newTransferTaskExecutorBase(
			shard,
			archiverClient,
			executionCache,
			logger,
			config,
		),
		clusterName:     clusterName,
		historyResender: historyResender,
		getRemoteClusterNameFn: func(ctx context.Context, taskInfo persistence.Task) (string, error) {
			if shard.GetConfig().EnableTransferQueueV2(shard.GetShardID()) {
				return getRemoteClusterName(ctx, shard.GetClusterMetadata().GetCurrentClusterName(), shard.GetDomainCache(), shard.GetActiveClusterManager(), taskInfo)
			}
			return clusterName, nil
		},
	}
}

func (t *transferStandbyTaskExecutor) Execute(task Task) (ExecuteResponse, error) {
	simulation.LogEvents(simulation.E{
		EventName:  simulation.EventNameExecuteHistoryTask,
		Host:       t.shard.GetConfig().HostName,
		ShardID:    t.shard.GetShardID(),
		DomainID:   task.GetDomainID(),
		WorkflowID: task.GetWorkflowID(),
		RunID:      task.GetRunID(),
		Payload: map[string]any{
			"task_category": persistence.HistoryTaskCategoryTransfer.Name(),
			"task_type":     task.GetTaskType(),
			"task_key":      task.GetTaskKey(),
		},
	})
	scope := getOrCreateDomainTaggedScope(t.shard, GetTransferTaskMetricsScope(task.GetTaskType(), false), task.GetDomainID(), t.logger)
	executeResponse := ExecuteResponse{
		Scope:        scope,
		IsActiveTask: false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskDefaultTimeout)
	defer cancel()

	switch transferTask := task.GetInfo().(type) {
	case *persistence.ActivityTask:
		return executeResponse, t.processActivityTask(ctx, transferTask)
	case *persistence.DecisionTask:
		return executeResponse, t.processDecisionTask(ctx, transferTask)
	case *persistence.CloseExecutionTask:
		return executeResponse, t.processCloseExecution(ctx, transferTask)
	case *persistence.RecordWorkflowClosedTask:
		return executeResponse, t.processCloseExecution(ctx, &persistence.CloseExecutionTask{
			WorkflowIdentifier: transferTask.WorkflowIdentifier,
			TaskData:           transferTask.TaskData,
		})
	case *persistence.RecordChildExecutionCompletedTask:
		// no action needed for standby
		// check the comment in t.processCloseExecution()
		return executeResponse, nil
	case *persistence.CancelExecutionTask:
		return executeResponse, t.processCancelExecution(ctx, transferTask)
	case *persistence.SignalExecutionTask:
		return executeResponse, t.processSignalExecution(ctx, transferTask)
	case *persistence.StartChildExecutionTask:
		return executeResponse, t.processStartChildExecution(ctx, transferTask)
	case *persistence.RecordWorkflowStartedTask:
		return executeResponse, t.processRecordWorkflowStarted(ctx, transferTask)
	case *persistence.ResetWorkflowTask:
		// no reset needed for standby
		// TODO: add error logs
		return executeResponse, nil
	case *persistence.UpsertWorkflowSearchAttributesTask:
		return executeResponse, t.processUpsertWorkflowSearchAttributes(ctx, transferTask)
	default:
		return executeResponse, errUnknownTransferTask
	}
}

// Empty func for now
func (t *transferStandbyTaskExecutor) Stop() {}

func (t *transferStandbyTaskExecutor) processActivityTask(
	ctx context.Context,
	transferTask *persistence.ActivityTask,
) error {

	processTaskIfClosed := false
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		activityInfo, ok := mutableState.GetActivityInfo(transferTask.ScheduleID)
		if !ok {
			return nil, nil
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, activityInfo.Version, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		taskList := types.TaskList{
			Name: activityInfo.TaskList,
			Kind: activityInfo.TaskListKind.Ptr(),
		}
		if taskList.Name == "" {
			taskList.Name = transferTask.TaskList
		}

		if activityInfo.StartedID == constants.EmptyEventID {
			return newPushActivityToMatchingInfo(
				activityInfo.ScheduleToStartTimeout,
				taskList,
				mutableState.GetExecutionInfo().PartitionConfig,
			), nil
		}

		return nil, nil
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		transferTask.ScheduleID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			transferTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.pushActivity,
			t.pushActivity,
		),
	)
}

func (t *transferStandbyTaskExecutor) processDecisionTask(
	ctx context.Context,
	transferTask *persistence.DecisionTask,
) error {

	processTaskIfClosed := false
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		decisionInfo, ok := mutableState.GetDecisionInfo(transferTask.ScheduleID)
		if !ok {
			return nil, nil
		}

		executionInfo := mutableState.GetExecutionInfo()
		workflowTimeout := executionInfo.WorkflowTimeout
		decisionTimeout := min(workflowTimeout, constants.MaxTaskTimeout)
		if executionInfo.TaskList != transferTask.TaskList {
			// Experimental: try to push sticky task as regular task with sticky timeout as TTL.
			// workflow might be sticky before namespace become standby
			// there shall already be a schedule_to_start timer created
			decisionTimeout = executionInfo.StickyScheduleToStartTimeout
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, decisionInfo.Version, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		if decisionInfo.StartedID == constants.EmptyEventID {
			return newPushDecisionToMatchingInfo(
				decisionTimeout,
				types.TaskList{Name: executionInfo.TaskList, Kind: executionInfo.TaskListKind.Ptr()}, // at standby, always use non-sticky tasklist
				mutableState.GetExecutionInfo().PartitionConfig,
			), nil
		}

		return nil, nil
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		transferTask.ScheduleID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			transferTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.pushDecision,
			t.pushDecision,
		),
	)
}

func (t *transferStandbyTaskExecutor) processCloseExecution(
	ctx context.Context,
	transferTask *persistence.CloseExecutionTask,
) error {

	processTaskIfClosed := true
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		if mutableState.IsWorkflowExecutionRunning() {
			// this can happen if workflow is reset.
			return nil, nil
		}

		completionEvent, err := mutableState.GetCompletionEvent(ctx)
		if err != nil {
			return nil, err
		}
		wfCloseTime := completionEvent.GetTimestamp()

		executionInfo := mutableState.GetExecutionInfo()
		workflowTypeName := executionInfo.WorkflowTypeName
		workflowCloseTimestamp := wfCloseTime
		workflowCloseStatus := persistence.ToInternalWorkflowExecutionCloseStatus(executionInfo.CloseStatus)
		workflowHistoryLength := mutableState.GetNextEventID() - 1
		startEvent, err := mutableState.GetStartEvent(ctx)
		if err != nil {
			return nil, err
		}
		workflowStartTimestamp := startEvent.GetTimestamp()
		workflowExecutionTimestamp := getWorkflowExecutionTimestamp(mutableState, startEvent)
		visibilityMemo := getWorkflowMemo(executionInfo.Memo)
		searchAttr := executionInfo.SearchAttributes
		headers := getWorkflowHeaders(startEvent)
		isCron := len(executionInfo.CronSchedule) > 0
		updateTimestamp := t.shard.GetTimeSource().Now()

		lastWriteVersion, err := mutableState.GetLastWriteVersion()
		if err != nil {
			return nil, err
		}
		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, lastWriteVersion, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		domainEntry, err := t.shard.GetDomainCache().GetDomainByID(transferTask.DomainID)
		if err != nil {
			return nil, err
		}
		numClusters := (int16)(len(domainEntry.GetReplicationConfig().Clusters))

		// DO NOT REPLY TO PARENT
		// since event replication should be done by active cluster
		return nil, t.recordWorkflowClosed(
			ctx,
			transferTask.DomainID,
			transferTask.WorkflowID,
			transferTask.RunID,
			workflowTypeName,
			workflowStartTimestamp,
			workflowExecutionTimestamp.UnixNano(),
			workflowCloseTimestamp,
			*workflowCloseStatus,
			workflowHistoryLength,
			transferTask.GetTaskID(),
			visibilityMemo,
			executionInfo.TaskList,
			isCron,
			numClusters,
			updateTimestamp.UnixNano(),
			searchAttr,
			headers,
		)
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		0,
		actionFn,
		standbyTaskPostActionNoOp,
	) // no op post action, since the entire workflow is finished
}

func (t *transferStandbyTaskExecutor) processCancelExecution(
	ctx context.Context,
	transferTask *persistence.CancelExecutionTask,
) error {

	processTaskIfClosed := false
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		requestCancelInfo, ok := mutableState.GetRequestCancelInfo(transferTask.InitiatedID)
		if !ok {
			return nil, nil
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, requestCancelInfo.Version, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		return getHistoryResendInfo(mutableState)
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		transferTask.InitiatedID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			transferTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *transferStandbyTaskExecutor) processSignalExecution(
	ctx context.Context,
	transferTask *persistence.SignalExecutionTask,
) error {

	processTaskIfClosed := false
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		signalInfo, ok := mutableState.GetSignalInfo(transferTask.InitiatedID)
		if !ok {
			return nil, nil
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, signalInfo.Version, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		return getHistoryResendInfo(mutableState)
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		transferTask.InitiatedID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			transferTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *transferStandbyTaskExecutor) processStartChildExecution(
	ctx context.Context,
	transferTask *persistence.StartChildExecutionTask,
) error {

	processTaskIfClosed := false
	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		childWorkflowInfo, ok := mutableState.GetChildExecutionInfo(transferTask.InitiatedID)
		if !ok {
			return nil, nil
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.DomainID, childWorkflowInfo.Version, transferTask.Version, transferTask)
		if err != nil || !ok {
			return nil, err
		}

		if childWorkflowInfo.StartedID != constants.EmptyEventID {
			return nil, nil
		}

		return getHistoryResendInfo(mutableState)
	}

	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		transferTask.InitiatedID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			transferTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *transferStandbyTaskExecutor) processRecordWorkflowStarted(
	ctx context.Context,
	transferTask *persistence.RecordWorkflowStartedTask,
) error {

	processTaskIfClosed := false
	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		0,
		func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {
			return nil, t.processRecordWorkflowStartedOrUpsertHelper(ctx, transferTask, mutableState, true)
		},
		standbyTaskPostActionNoOp,
	)
}

func (t *transferStandbyTaskExecutor) processUpsertWorkflowSearchAttributes(
	ctx context.Context,
	transferTask *persistence.UpsertWorkflowSearchAttributesTask,
) error {

	processTaskIfClosed := false
	return t.processTransfer(
		ctx,
		processTaskIfClosed,
		transferTask,
		0,
		func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {
			return nil, t.processRecordWorkflowStartedOrUpsertHelper(ctx, transferTask, mutableState, false)
		},
		standbyTaskPostActionNoOp,
	)
}

func (t *transferStandbyTaskExecutor) processRecordWorkflowStartedOrUpsertHelper(
	ctx context.Context,
	transferTask persistence.Task,
	mutableState execution.MutableState,
	isRecordStart bool,
) error {

	workflowStartedScope := getOrCreateDomainTaggedScope(t.shard, metrics.TransferStandbyTaskRecordWorkflowStartedScope, transferTask.GetDomainID(), t.logger)

	// verify task version for RecordWorkflowStarted.
	// upsert doesn't require verifyTask, because it is just a sync of mutableState.
	if isRecordStart {
		startVersion, err := mutableState.GetStartVersion()
		if err != nil {
			return err
		}
		ok, err := verifyTaskVersion(t.shard, t.logger, transferTask.GetDomainID(), startVersion, transferTask.GetVersion(), transferTask)
		if err != nil || !ok {
			return err
		}
	}

	executionInfo := mutableState.GetExecutionInfo()
	workflowTimeout := executionInfo.WorkflowTimeout
	wfTypeName := executionInfo.WorkflowTypeName
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return err
	}
	startTimestamp := startEvent.GetTimestamp()
	executionTimestamp := getWorkflowExecutionTimestamp(mutableState, startEvent)
	visibilityMemo := getWorkflowMemo(executionInfo.Memo)
	isCron := len(executionInfo.CronSchedule) > 0
	updateTimestamp := t.shard.GetTimeSource().Now()

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(transferTask.GetDomainID())
	if err != nil {
		return err
	}
	numClusters := (int16)(len(domainEntry.GetReplicationConfig().Clusters))

	searchAttr := copySearchAttributes(executionInfo.SearchAttributes)
	headers := getWorkflowHeaders(startEvent)

	if isRecordStart {
		workflowStartedScope.IncCounter(metrics.WorkflowStartedCount)
		return t.recordWorkflowStarted(
			ctx,
			transferTask.GetDomainID(),
			transferTask.GetWorkflowID(),
			transferTask.GetRunID(),
			wfTypeName,
			startTimestamp,
			executionTimestamp.UnixNano(),
			workflowTimeout,
			transferTask.GetTaskID(),
			executionInfo.TaskList,
			isCron,
			numClusters,
			visibilityMemo,
			updateTimestamp.UnixNano(),
			searchAttr,
			headers,
		)
	}
	return t.upsertWorkflowExecution(
		ctx,
		transferTask.GetDomainID(),
		transferTask.GetWorkflowID(),
		transferTask.GetRunID(),
		wfTypeName,
		startTimestamp,
		executionTimestamp.UnixNano(),
		workflowTimeout,
		transferTask.GetTaskID(),
		executionInfo.TaskList,
		visibilityMemo,
		isCron,
		numClusters,
		updateTimestamp.UnixNano(),
		searchAttr,
		headers,
	)

}

func (t *transferStandbyTaskExecutor) processTransfer(
	ctx context.Context,
	processTaskIfClosed bool,
	transferTask persistence.Task,
	eventID int64,
	actionFn standbyActionFn,
	postActionFn standbyPostActionFn,
) (retError error) {
	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		transferTask.GetDomainID(),
		getWorkflowExecution(transferTask),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() {
		if isRedispatchErr(err) {
			release(nil)
		} else {
			release(retError)
		}
	}()

	mutableState, err := loadMutableState(ctx, wfContext, transferTask, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, eventID)
	if err != nil || mutableState == nil {
		return err
	}

	if !mutableState.IsWorkflowExecutionRunning() && !processTaskIfClosed {
		// workflow already finished, no need to process the timer
		return nil
	}

	historyResendInfo, err := actionFn(ctx, wfContext, mutableState)
	if err != nil {
		return err
	}

	release(nil)
	return postActionFn(ctx, transferTask, historyResendInfo, t.logger)
}

func (t *transferStandbyTaskExecutor) pushActivity(
	ctx context.Context,
	task persistence.Task,
	postActionInfo interface{},
	logger log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	pushActivityInfo := postActionInfo.(*pushActivityToMatchingInfo)
	timeout := min(pushActivityInfo.activityScheduleToStartTimeout, constants.MaxTaskTimeout)
	taskList := &pushActivityInfo.tasklist
	return t.transferTaskExecutorBase.pushActivity(
		ctx,
		task.(*persistence.ActivityTask),
		taskList,
		timeout,
		pushActivityInfo.partitionConfig,
	)
}

func (t *transferStandbyTaskExecutor) pushDecision(
	ctx context.Context,
	task persistence.Task,
	postActionInfo interface{},
	logger log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	pushDecisionInfo := postActionInfo.(*pushDecisionToMatchingInfo)
	timeout := min(pushDecisionInfo.decisionScheduleToStartTimeout, constants.MaxTaskTimeout)
	return t.transferTaskExecutorBase.pushDecision(
		ctx,
		task.(*persistence.DecisionTask),
		&pushDecisionInfo.tasklist,
		timeout,
		pushDecisionInfo.partitionConfig,
	)
}

func (t *transferStandbyTaskExecutor) fetchHistoryFromRemote(
	ctx context.Context,
	taskInfo persistence.Task,
	postActionInfo interface{},
	_ log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	resendInfo := postActionInfo.(*historyResendInfo)

	t.metricsClient.IncCounter(metrics.HistoryRereplicationByTransferTaskScope, metrics.CadenceClientRequests)
	stopwatch := t.metricsClient.StartTimer(metrics.HistoryRereplicationByTransferTaskScope, metrics.CadenceClientLatency)
	defer stopwatch.Stop()

	remoteClusterName, err := t.getRemoteClusterNameFn(ctx, taskInfo)
	if err != nil {
		return err
	}
	if resendInfo.lastEventID != nil && resendInfo.lastEventVersion != nil {
		// note history resender doesn't take in a context parameter, there's a separate dynamicconfig for
		// controlling the timeout for resending history.
		err = t.historyResender.SendSingleWorkflowHistory(
			remoteClusterName,
			taskInfo.GetDomainID(),
			taskInfo.GetWorkflowID(),
			taskInfo.GetRunID(),
			resendInfo.lastEventID,
			resendInfo.lastEventVersion,
			nil,
			nil,
		)
	} else {
		err = &types.InternalServiceError{
			Message: fmt.Sprintf("incomplete historyResendInfo: %v", resendInfo),
		}
	}

	if err != nil {
		t.logger.Error("Error re-replicating history from remote.",
			tag.ShardID(t.shard.GetShardID()),
			tag.WorkflowDomainID(taskInfo.GetDomainID()),
			tag.WorkflowID(taskInfo.GetWorkflowID()),
			tag.WorkflowRunID(taskInfo.GetRunID()),
			tag.SourceCluster(remoteClusterName),
			tag.Error(err),
		)
	}

	// return error so task processing logic will retry
	return &redispatchError{Reason: "fetchHistoryFromRemote"}
}

func (t *transferStandbyTaskExecutor) getCurrentTime(taskInfo persistence.Task) (time.Time, error) {
	// for standby task, we can always use timesource.Now, because they're supposed to be processed immediately after creation,
	// this is what're doing for queue v2, for queue v1, I just don't bother to change the behavior
	return t.shard.GetCurrentTime(t.clusterName), nil
}
