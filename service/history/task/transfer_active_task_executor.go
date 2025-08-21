// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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
	"errors"
	"fmt"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/simulation"
	"github.com/uber/cadence/service/history/workflowcache"
	"github.com/uber/cadence/service/worker/archiver"
	"github.com/uber/cadence/service/worker/parentclosepolicy"
)

const (
	resetWorkflowTimeout = 30 * time.Second
)

var (
	// ErrMissingRequestCancelInfo indicates missing request cancel info
	ErrMissingRequestCancelInfo = &types.InternalServiceError{Message: "unable to get request cancel info"}
	// ErrMissingSignalInfo indicates missing signal external
	ErrMissingSignalInfo = &types.InternalServiceError{Message: "unable to get signal info"}
)

var (
	errUnknownTransferTask = errors.New("unknown transfer task")
	errWorkflowBusy        = errors.New("unable to get workflow execution lock within specified timeout")
	errWorkflowRateLimited = errors.New("workflow is being rate limited for making too many requests")
)

type (
	transferActiveTaskExecutor struct {
		*transferTaskExecutorBase

		historyClient           history.Client
		parentClosePolicyClient parentclosepolicy.Client
		workflowResetter        reset.WorkflowResetter
		wfIDCache               workflowcache.WFCache
	}

	generatorF = func(taskGenerator execution.MutableStateTaskGenerator) error
)

// NewTransferActiveTaskExecutor creates a new task executor for active transfer task
func NewTransferActiveTaskExecutor(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	workflowResetter reset.WorkflowResetter,
	logger log.Logger,
	config *config.Config,
	wfIDCache workflowcache.WFCache,
) Executor {

	return &transferActiveTaskExecutor{
		transferTaskExecutorBase: newTransferTaskExecutorBase(
			shard,
			archiverClient,
			executionCache,
			logger,
			config,
		),
		historyClient: shard.GetService().GetHistoryClient(),
		parentClosePolicyClient: parentclosepolicy.NewClient(
			shard.GetMetricsClient(),
			shard.GetLogger(),
			shard.GetService().GetSDKClient(),
			config.NumParentClosePolicySystemWorkflows(),
		),
		workflowResetter: workflowResetter,
		wfIDCache:        wfIDCache,
	}
}

func (t *transferActiveTaskExecutor) Execute(task Task) (ExecuteResponse, error) {
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
	scope := getOrCreateDomainTaggedScope(t.shard, GetTransferTaskMetricsScope(task.GetTaskType(), true), task.GetDomainID(), t.logger)
	executeResponse := ExecuteResponse{
		Scope:        scope,
		IsActiveTask: true,
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
		return executeResponse, t.processRecordWorkflowClosed(ctx, transferTask)
	case *persistence.RecordChildExecutionCompletedTask:
		return executeResponse, t.processRecordChildExecutionCompleted(ctx, transferTask)
	case *persistence.CancelExecutionTask:
		return executeResponse, t.processCancelExecution(ctx, transferTask)
	case *persistence.SignalExecutionTask:
		return executeResponse, t.processSignalExecution(ctx, transferTask)
	case *persistence.StartChildExecutionTask:
		return executeResponse, t.processStartChildExecution(ctx, transferTask)
	case *persistence.RecordWorkflowStartedTask:
		return executeResponse, t.processRecordWorkflowStarted(ctx, transferTask)
	case *persistence.ResetWorkflowTask:
		return executeResponse, t.processResetWorkflow(ctx, transferTask)
	case *persistence.UpsertWorkflowSearchAttributesTask:
		return executeResponse, t.processUpsertWorkflowSearchAttributes(ctx, transferTask)
	default:
		return executeResponse, errUnknownTransferTask
	}
}

// Empty func for now
func (t *transferActiveTaskExecutor) Stop() {}

func (t *transferActiveTaskExecutor) processActivityTask(
	ctx context.Context,
	task *persistence.ActivityTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, task.ScheduleID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	domainName := mutableState.GetDomainEntry().GetInfo().Name
	ai, ok := mutableState.GetActivityInfo(task.ScheduleID)
	if !ok {
		t.logger.Debug("Potentially duplicate ", tag.TaskID(task.TaskID), tag.WorkflowScheduleID(task.ScheduleID), tag.TaskType(persistence.TransferTaskTypeActivityTask))
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, ai.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	timeout := min(ai.ScheduleToStartTimeout, constants.MaxTaskTimeout)

	taskList := &types.TaskList{
		Name: ai.TaskList,
		Kind: ai.TaskListKind.Ptr(),
	}
	if taskList.Name == "" {
		taskList.Name = task.TaskList
	}
	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	// Rate limiting task processing requests
	if !t.allowTask(task) {
		return errWorkflowRateLimited
	}

	err = t.pushActivity(ctx, task, taskList, timeout, mutableState.GetExecutionInfo().PartitionConfig)
	if err == nil {
		scope := common.NewPerTaskListScope(domainName, taskList.Name, taskList.GetKind(), t.metricsClient, metrics.TransferActiveTaskActivityScope)
		scope.RecordTimer(metrics.ScheduleToStartHistoryQueueLatencyPerTaskList, time.Since(task.GetVisibilityTimestamp()))
	}
	return err
}

func (t *transferActiveTaskExecutor) processDecisionTask(
	ctx context.Context,
	task *persistence.DecisionTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, task.ScheduleID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	decision, found := mutableState.GetDecisionInfo(task.ScheduleID)
	if !found {
		t.logger.Debug("Potentially duplicate ", tag.TaskID(task.TaskID), tag.WorkflowScheduleID(task.ScheduleID), tag.TaskType(persistence.TransferTaskTypeDecisionTask))
		return nil
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.DomainID, decision.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	domainName := mutableState.GetDomainEntry().GetInfo().Name
	executionInfo := mutableState.GetExecutionInfo()
	workflowTimeout := executionInfo.WorkflowTimeout
	decisionTimeout := min(workflowTimeout, constants.MaxTaskTimeout)

	// NOTE: previously this section check whether mutable state has enabled
	// sticky decision, if so convert the decision to a sticky decision.
	// that logic has a bug which timer task for that sticky decision is not generated
	// the correct logic should check whether the decision task is a sticky decision
	// task or not.
	taskList := &types.TaskList{
		Name: task.TaskList,
		Kind: executionInfo.TaskListKind.Ptr(),
	}
	if mutableState.GetExecutionInfo().TaskList != task.TaskList {
		// this decision is an sticky decision
		// there shall already be an timer set
		taskList.Kind = types.TaskListKindSticky.Ptr()
		decisionTimeout = executionInfo.StickyScheduleToStartTimeout
	}
	// TODO: for normal decision, we don't know if there's a scheduleToStart
	// timeout timer task associated with the decision since it's determined
	// when creating the decision and the result is not persisted in mutable
	// state.
	// If we calculated the timeout again here, the timeout may be different,
	// or even lost the decision if there's originally no timeout timer task
	// for the decision. Using MaxTaskTimeout here for now so at least no
	// decision will be lost.

	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	// Rate limiting task processing requests
	if !t.allowTask(task) {
		return errWorkflowRateLimited
	}

	err = t.pushDecision(ctx, task, taskList, decisionTimeout, mutableState.GetExecutionInfo().PartitionConfig)
	if _, ok := err.(*types.StickyWorkerUnavailableError); ok {
		// sticky worker is unavailable, switch to non-sticky task list
		taskList = &types.TaskList{
			Name: mutableState.GetExecutionInfo().TaskList,
		}

		// Continue to use sticky schedule_to_start timeout as TTL for the matching task. Because the schedule_to_start
		// timeout timer task is already created which will timeout this task if no worker pick it up in 5s anyway.
		// There is no need to reset sticky, because if this task is picked by new worker, the new worker will reset
		// the sticky queue to a new one. However, if worker is completely down, that schedule_to_start timeout task
		// will re-create a new non-sticky task and reset sticky.
		err = t.pushDecision(ctx, task, taskList, decisionTimeout, mutableState.GetExecutionInfo().PartitionConfig)
	}
	if err == nil {
		scope := common.NewPerTaskListScope(domainName, taskList.Name, taskList.GetKind(), t.metricsClient, metrics.TransferActiveTaskDecisionScope)
		scope.RecordTimer(metrics.ScheduleToStartHistoryQueueLatencyPerTaskList, time.Since(task.GetVisibilityTimestamp()))
	}
	return err
}

func (t *transferActiveTaskExecutor) allowTask(task persistence.Task) bool {
	return t.wfIDCache.AllowInternal(task.GetDomainID(), task.GetWorkflowID())
}

func (t *transferActiveTaskExecutor) processCloseExecution(
	ctx context.Context,
	task *persistence.CloseExecutionTask,
) error {
	return t.processCloseExecutionTaskHelper(ctx, task, true, true, true)
}

func (t *transferActiveTaskExecutor) processRecordWorkflowClosed(
	ctx context.Context,
	task *persistence.RecordWorkflowClosedTask,
) error {
	return t.processCloseExecutionTaskHelper(ctx, task, true, false, false)
}

func (t *transferActiveTaskExecutor) processRecordChildExecutionCompleted(
	ctx context.Context,
	task *persistence.RecordChildExecutionCompletedTask,
) error {
	return t.processCloseExecutionTaskHelper(ctx, task, false, true, false)
}

// TODO: this helper function performs three operations:
// 1. publish workflow closed visibility record
// 2. if has parent workflow, reply to the parent workflow
// 3. if has child workflow(s), apply parent close policy
// ideally we should separate them into 3 functions, but it is complicated
// but the fact that we want to release mutable state lock as early as possible
// we should see if there's a better way to organize the code
func (t *transferActiveTaskExecutor) processCloseExecutionTaskHelper(
	ctx context.Context,
	task persistence.Task,
	recordWorkflowClosed bool,
	replyToParentWorkflowIfApplicable bool,
	applyParentClosePolicy bool,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.GetDomainID(),
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return err
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.GetDomainID(), lastWriteVersion, task.GetVersion(), task)
	if err != nil || !ok {
		return err
	}

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil {
		return err
	}

	executionInfo := mutableState.GetExecutionInfo()
	completionEvent, err := mutableState.GetCompletionEvent(ctx)
	if err != nil {
		return err
	}
	wfCloseTime := completionEvent.GetTimestamp()

	parentDomainID := executionInfo.ParentDomainID
	parentWorkflowID := executionInfo.ParentWorkflowID
	parentRunID := executionInfo.ParentRunID
	initiatedID := executionInfo.InitiatedID

	workflowTypeName := executionInfo.WorkflowTypeName
	workflowCloseTimestamp := wfCloseTime
	workflowCloseStatus := persistence.ToInternalWorkflowExecutionCloseStatus(executionInfo.CloseStatus)
	workflowHistoryLength := mutableState.GetNextEventID() - 1
	isCron := len(executionInfo.CronSchedule) > 0
	numClusters := (int16)(len(domainEntry.GetReplicationConfig().Clusters))
	updateTimestamp := t.shard.GetTimeSource().Now()

	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return err
	}
	workflowStartTimestamp := startEvent.GetTimestamp()
	workflowExecutionTimestamp := getWorkflowExecutionTimestamp(mutableState, startEvent)
	visibilityMemo := getWorkflowMemo(executionInfo.Memo)
	searchAttr := executionInfo.SearchAttributes
	headers := getWorkflowHeaders(startEvent)
	domainName := mutableState.GetDomainEntry().GetInfo().Name
	children := mutableState.GetPendingChildExecutionInfos()

	// we've gathered all necessary information from mutable state.
	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	// publish workflow closed visibility records
	if recordWorkflowClosed {
		if err := t.recordWorkflowClosed(
			ctx,
			task.GetDomainID(),
			task.GetWorkflowID(),
			task.GetRunID(),
			workflowTypeName,
			workflowStartTimestamp,
			workflowExecutionTimestamp.UnixNano(),
			workflowCloseTimestamp,
			*workflowCloseStatus,
			workflowHistoryLength,
			task.GetTaskID(),
			visibilityMemo,
			executionInfo.TaskList,
			isCron,
			numClusters,
			updateTimestamp.UnixNano(),
			searchAttr,
			headers,
		); err != nil {
			return err
		}
	}

	// Communicate the result to parent execution if this is Child Workflow execution
	// and parent domain is in the same cluster
	replyToParentWorkflow := replyToParentWorkflowIfApplicable && mutableState.HasParentExecution() && executionInfo.CloseStatus != persistence.WorkflowCloseStatusContinuedAsNew
	if replyToParentWorkflow {
		recordChildCompletionCtx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
		defer cancel()
		err := t.historyClient.RecordChildExecutionCompleted(recordChildCompletionCtx, &types.RecordChildExecutionCompletedRequest{
			DomainUUID: parentDomainID,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: parentWorkflowID,
				RunID:      parentRunID,
			},
			InitiatedID: initiatedID,
			CompletedExecution: &types.WorkflowExecution{
				WorkflowID: task.GetWorkflowID(),
				RunID:      task.GetRunID(),
			},
			CompletionEvent: completionEvent,
		})

		// Check to see if the error is non-transient, in which case reset the error and continue with processing
		switch err.(type) {
		case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError:
			err = nil
		}

		if err != nil {
			return err
		}
	}

	if applyParentClosePolicy {

		parentExecution := types.WorkflowExecution{
			WorkflowID: task.GetWorkflowID(),
			RunID:      task.GetRunID(),
		}

		err := t.processParentClosePolicy(ctx, task.GetDomainID(), domainName, &parentExecution, children)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *transferActiveTaskExecutor) processCancelExecution(
	ctx context.Context,
	task *persistence.CancelExecutionTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, task.InitiatedID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	initiatedEventID := task.InitiatedID
	requestCancelInfo, ok := mutableState.GetRequestCancelInfo(initiatedEventID)
	if !ok {
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, requestCancelInfo.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	targetDomainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.TargetDomainID)
	if err != nil {
		// TODO: handle the case where target domain does not exist
		return err
	}

	targetDomainName := targetDomainEntry.GetInfo().Name

	// handle workflow cancel itself
	if task.DomainID == task.TargetDomainID && task.WorkflowID == task.TargetWorkflowID {
		// it does not matter if the run ID is a mismatch
		err = requestCancelExternalExecutionFailed(
			ctx,
			t.logger,
			task,
			wfContext,
			targetDomainName,
			task.TargetWorkflowID,
			task.TargetRunID,
			t.shard.GetTimeSource().Now(),
		)
		return err
	}

	if err = requestCancelExternalExecutionWithRetry(
		ctx,
		t.historyClient,
		task,
		targetDomainName,
		requestCancelInfo.CancelRequestID,
	); err != nil {
		t.logger.Error("Failed to cancel external workflow execution",
			tag.WorkflowDomainID(task.DomainID),
			tag.WorkflowID(task.WorkflowID),
			tag.WorkflowRunID(task.RunID),
			tag.TargetWorkflowDomainID(task.TargetDomainID),
			tag.TargetWorkflowID(task.TargetWorkflowID),
			tag.TargetWorkflowRunID(task.TargetRunID),
			tag.Error(err))

		// Check to see if the error is non-transient, in which case add RequestCancelFailed
		// event and complete transfer task by setting the err = nil
		if common.IsServiceTransientError(err) || common.IsContextTimeoutError(err) {
			// for retryable error just return
			return err
		}
		return requestCancelExternalExecutionFailed(
			ctx,
			t.logger,
			task,
			wfContext,
			targetDomainName,
			task.TargetWorkflowID,
			task.TargetRunID,
			t.shard.GetTimeSource().Now(),
		)
	}

	t.logger.Debug("RequestCancel successfully recorded to external workflow execution",
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.TargetWorkflowDomainID(task.TargetDomainID),
		tag.TargetWorkflowID(task.TargetWorkflowID),
		tag.TargetWorkflowRunID(task.TargetRunID))

	// Record ExternalWorkflowExecutionCancelRequested in source execution
	return requestCancelExternalExecutionCompleted(
		ctx,
		t.logger,
		task,
		wfContext,
		targetDomainName,
		task.TargetWorkflowID,
		task.TargetRunID,
		t.shard.GetTimeSource().Now(),
	)
}

func (t *transferActiveTaskExecutor) processSignalExecution(
	ctx context.Context,
	task *persistence.SignalExecutionTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, task.InitiatedID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	initiatedEventID := task.InitiatedID
	signalInfo, ok := mutableState.GetSignalInfo(initiatedEventID)
	if !ok {
		// TODO: here we should also RemoveSignalMutableState from target workflow
		// Otherwise, target SignalRequestID still can leak if shard restart after signalExternalExecutionCompleted
		// To do that, probably need to add the SignalRequestID in transfer
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, signalInfo.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	targetDomainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.TargetDomainID)
	if err != nil {
		// TODO: handle the case where target domain does not exist
		return err
	}

	targetDomainName := targetDomainEntry.GetInfo().Name

	// handle workflow signal itself
	if task.DomainID == task.TargetDomainID && task.WorkflowID == task.TargetWorkflowID {
		// it does not matter if the run ID is a mismatch
		return signalExternalExecutionFailed(
			ctx,
			t.logger,
			task,
			wfContext,
			targetDomainName,
			task.TargetWorkflowID,
			task.TargetRunID,
			signalInfo.Control,
			t.shard.GetTimeSource().Now(),
		)
	}

	if err = signalExternalExecutionWithRetry(
		ctx,
		t.historyClient,
		task,
		targetDomainName,
		signalInfo,
	); err != nil {
		if !common.IsExpectedError(err) {
			t.logger.Error("Failed to signal external workflow execution",
				tag.WorkflowDomainID(task.DomainID),
				tag.WorkflowID(task.WorkflowID),
				tag.WorkflowRunID(task.RunID),
				tag.TargetWorkflowDomainID(task.TargetDomainID),
				tag.TargetWorkflowID(task.TargetWorkflowID),
				tag.TargetWorkflowRunID(task.TargetRunID),
				tag.Error(err))
		}

		// Check to see if the error is non-transient, in which case add SignalFailed
		// event and complete transfer task by setting the err = nil
		if common.IsServiceTransientError(err) || common.IsContextTimeoutError(err) {
			// for retryable error just return
			return err
		}
		return signalExternalExecutionFailed(
			ctx,
			t.logger,
			task,
			wfContext,
			targetDomainName,
			task.TargetWorkflowID,
			task.TargetRunID,
			signalInfo.Control,
			t.shard.GetTimeSource().Now(),
		)
	}

	t.logger.Debug("Signal successfully recorded to external workflow execution",
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.TargetWorkflowDomainID(task.TargetDomainID),
		tag.TargetWorkflowID(task.TargetWorkflowID),
		tag.TargetWorkflowRunID(task.TargetRunID))

	err = signalExternalExecutionCompleted(
		ctx,
		t.logger,
		task,
		wfContext,
		targetDomainName,
		task.TargetWorkflowID,
		task.TargetRunID,
		signalInfo.Control,
		t.shard.GetTimeSource().Now(),
	)
	if err != nil {
		return err
	}

	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(retError)

	// remove signalRequestedID from target workflow, after Signal detail is removed from source workflow
	return removeSignalMutableStateWithRetry(ctx, t.historyClient, task, signalInfo.SignalRequestID)
}

func (t *transferActiveTaskExecutor) processStartChildExecution(
	ctx context.Context,
	task *persistence.StartChildExecutionTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, task.InitiatedID)
	if err != nil {
		return err
	}
	if mutableState == nil {
		return nil
	}

	initiatedEventID := task.InitiatedID
	childInfo, ok := mutableState.GetChildExecutionInfo(initiatedEventID)
	if !ok {
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, childInfo.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	workflowRunning := mutableState.IsWorkflowExecutionRunning()
	childStarted := childInfo.StartedID != constants.EmptyEventID

	if !workflowRunning && (!childStarted || childInfo.ParentClosePolicy != types.ParentClosePolicyAbandon) {
		// three cases here:
		// case 1: workflow not running, child started, close policy is not abandon
		// case 2 & 3: workflow not running, child not started, close policy is or is not abandon
		return nil
	}

	// Get target domain name
	var targetDomainName string
	var targetDomainEntry *cache.DomainCacheEntry
	if targetDomainEntry, err = t.shard.GetDomainCache().GetDomainByID(task.TargetDomainID); err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return err
		}
		// TODO: handle the case where target domain does not exist

		// it is possible that the domain got deleted. Use domainID instead as this is only needed for the history event
		targetDomainName = task.TargetDomainID
	} else {

		targetDomainName = targetDomainEntry.GetInfo().Name
	}

	// ChildExecution already started, just create DecisionTask and complete transfer task
	// if parent already closed, since child workflow started event already written to history,
	// still schedule the decision if the parent close policy is Abandon.
	// If parent close policy cancel, a decision will be scheduled when processing that close policy.
	if childStarted {
		// NOTE: do not access anything related mutable state after this lock release
		// release the context lock since we no longer need mutable state builder and
		// the rest of logic is making RPC call, which takes time.
		release(nil)
		return createFirstDecisionTask(
			ctx,
			t.historyClient,
			task.TargetDomainID,
			&types.WorkflowExecution{
				WorkflowID: childInfo.StartedWorkflowID,
				RunID:      childInfo.StartedRunID,
			})
	}
	// remaining 2 cases:
	// workflow running, child not started, close policy is or is not abandon

	initiatedEvent, err := mutableState.GetChildExecutionInitiatedEvent(ctx, initiatedEventID)
	if err != nil {
		return err
	}

	attributes := initiatedEvent.StartChildWorkflowExecutionInitiatedEventAttributes
	childRunID, err := startWorkflowWithRetry(
		ctx,
		t.historyClient,
		t.shard.GetTimeSource(),
		t.shard.GetDomainCache(),
		task,
		targetDomainName,
		childInfo.CreateRequestID,
		attributes,
		mutableState.GetExecutionInfo().PartitionConfig,
	)
	if err != nil {

		// Check to see if the error is non-transient, in which case add StartChildWorkflowExecutionFailed
		// event and complete transfer task by setting the err = nil
		switch err.(type) {
		// TODO: we should also handle domain not exist error here
		// but we probably need to introduce a new error type for DomainNotExists,
		// for now when getting an EntityNotExists error, we can't tell if it's domain or workflow.
		case *types.WorkflowExecutionAlreadyStartedError:
			t.logger.Info("workflow has already started",
				tag.WorkflowDomainID(task.DomainID),
				tag.WorkflowID(task.WorkflowID),
				tag.WorkflowRunID(task.RunID),
				tag.TargetWorkflowDomainID(task.TargetDomainID),
				tag.TargetWorkflowID(attributes.WorkflowID),
			)
			err = recordStartChildExecutionFailed(ctx, t.logger, task, wfContext, attributes, t.shard.GetTimeSource().Now())
		default:
			t.logger.Error("Failed to start child workflow execution",
				tag.WorkflowDomainID(task.DomainID),
				tag.WorkflowID(task.WorkflowID),
				tag.WorkflowRunID(task.RunID),
				tag.TargetWorkflowDomainID(task.TargetDomainID),
				tag.TargetWorkflowID(attributes.WorkflowID),
				tag.Error(err))
		}
		return err
	}

	t.logger.Debug("Child Execution started successfully",
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.TargetWorkflowDomainID(task.TargetDomainID),
		tag.TargetWorkflowID(attributes.WorkflowID),
		tag.TargetWorkflowRunID(childRunID))

	// Child execution is successfully started, record ChildExecutionStartedEvent in parent execution
	err = recordChildExecutionStarted(ctx, t.logger, task, wfContext, attributes, childRunID, t.shard.GetTimeSource().Now())
	if err != nil {
		return err
	}

	// NOTE: do not access anything related mutable state after this lock release
	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)
	// Finally create first decision task for Child execution so it is really started
	// entity not exist error is checked and ignored in HandleErr() method in task.go
	return createFirstDecisionTask(
		ctx,
		t.historyClient,
		task.TargetDomainID,
		&types.WorkflowExecution{
			WorkflowID: task.TargetWorkflowID,
			RunID:      childRunID,
		})
}

func (t *transferActiveTaskExecutor) processRecordWorkflowStarted(
	ctx context.Context,
	task *persistence.RecordWorkflowStartedTask,
) (retError error) {

	return t.processRecordWorkflowStartedOrUpsertHelper(ctx, task, true)
}

func (t *transferActiveTaskExecutor) processUpsertWorkflowSearchAttributes(
	ctx context.Context,
	task *persistence.UpsertWorkflowSearchAttributesTask,
) (retError error) {

	return t.processRecordWorkflowStartedOrUpsertHelper(ctx, task, false)
}

func (t *transferActiveTaskExecutor) processRecordWorkflowStartedOrUpsertHelper(
	ctx context.Context,
	task persistence.Task,
	recordStart bool,
) (retError error) {

	workflowStartedScope := getOrCreateDomainTaggedScope(t.shard, metrics.TransferActiveTaskRecordWorkflowStartedScope, task.GetDomainID(), t.logger)

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.GetDomainID(),
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	// verify task version for RecordWorkflowStarted.
	// upsert doesn't require verifyTask, because it is just a sync of mutableState.
	if recordStart {
		startVersion, err := mutableState.GetStartVersion()
		if err != nil {
			return err
		}
		ok, err := verifyTaskVersion(t.shard, t.logger, task.GetDomainID(), startVersion, task.GetVersion(), task)
		if err != nil || !ok {
			return err
		}
	}

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil {
		return err
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
	searchAttr := copySearchAttributes(executionInfo.SearchAttributes)
	headers := getWorkflowHeaders(startEvent)
	isCron := len(executionInfo.CronSchedule) > 0
	numClusters := (int16)(len(domainEntry.GetReplicationConfig().Clusters))
	updateTimestamp := t.shard.GetTimeSource().Now()

	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	if recordStart {
		workflowStartedScope.IncCounter(metrics.WorkflowStartedCount)
		return t.recordWorkflowStarted(
			ctx,
			task.GetDomainID(),
			task.GetWorkflowID(),
			task.GetRunID(),
			wfTypeName,
			startTimestamp,
			executionTimestamp.UnixNano(),
			workflowTimeout,
			task.GetTaskID(),
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
		task.GetDomainID(),
		task.GetWorkflowID(),
		task.GetRunID(),
		wfTypeName,
		startTimestamp,
		executionTimestamp.UnixNano(),
		workflowTimeout,
		task.GetTaskID(),
		executionInfo.TaskList,
		visibilityMemo,
		isCron,
		numClusters,
		updateTimestamp.UnixNano(),
		searchAttr,
		headers,
	)
}

func (t *transferActiveTaskExecutor) processResetWorkflow(
	ctx context.Context,
	task *persistence.ResetWorkflowTask,
) (retError error) {

	currentContext, currentRelease, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { currentRelease(retError) }()

	currentMutableState, err := loadMutableState(ctx, currentContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if currentMutableState == nil {
		return nil
	}

	logger := t.logger.WithTags(
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
	)
	domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if err != nil {
		return err
	}
	if !currentMutableState.IsWorkflowExecutionRunning() {
		// it means this this might not be current anymore, we need to check
		var resp *persistence.GetCurrentExecutionResponse
		resp, err = t.shard.GetExecutionManager().GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
			DomainID:   task.DomainID,
			WorkflowID: task.WorkflowID,
			DomainName: domainName,
		})
		if err != nil {
			return err
		}
		if resp.RunID != task.RunID {
			logger.Warn("Auto-Reset is skipped, because current run is stale.")
			return nil
		}
	}
	// TODO: current reset doesn't allow childWFs, in the future we will release this restriction
	if len(currentMutableState.GetPendingChildExecutionInfos()) > 0 {
		logger.Warn("Auto-Reset is skipped, because current run has pending child executions.")
		return nil
	}

	currentStartVersion, err := currentMutableState.GetStartVersion()
	if err != nil {
		return err
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.DomainID, currentStartVersion, task.Version, task)
	if err != nil || !ok {
		return err
	}

	executionInfo := currentMutableState.GetExecutionInfo()
	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(executionInfo.DomainID)
	if err != nil {
		return err
	}
	logger = logger.WithTags(tag.WorkflowDomainName(domainEntry.GetInfo().Name))

	reason, resetPoint := execution.FindAutoResetPoint(t.shard.GetTimeSource(), &domainEntry.GetConfig().BadBinaries, executionInfo.AutoResetPoints)
	if resetPoint == nil {
		logger.Warn("Auto-Reset is skipped, because reset point is not found.")
		return nil
	}
	logger = logger.WithTags(
		tag.WorkflowResetBaseRunID(resetPoint.GetRunID()),
		tag.WorkflowBinaryChecksum(resetPoint.GetBinaryChecksum()),
		tag.WorkflowEventID(resetPoint.GetFirstDecisionCompletedID()),
	)

	var baseContext execution.Context
	var baseMutableState execution.MutableState
	var baseRelease execution.ReleaseFunc
	if resetPoint.GetRunID() == executionInfo.RunID {
		baseContext = currentContext
		baseMutableState = currentMutableState
		baseRelease = currentRelease
	} else {
		baseExecution := types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      resetPoint.GetRunID(),
		}
		baseContext, baseRelease, err = t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
			task.DomainID,
			baseExecution,
			taskGetExecutionContextTimeout,
		)
		if err != nil {
			return err
		}

		defer func() { baseRelease(retError) }()
		baseMutableState, err = loadMutableState(ctx, baseContext, task, t.metricsClient.Scope(metrics.TransferQueueProcessorScope), t.logger, 0)
		if err != nil {
			return err
		}
		if baseMutableState == nil {
			return nil
		}
	}

	// reset workflow needs to go through the history so it may take a long time.
	// as a result it's not subject to the taskDefaultTimeout. Otherwise the task
	// may got stuck if the workflow history is large.
	return t.resetWorkflow(
		task,
		domainEntry.GetInfo().Name,
		reason,
		resetPoint,
		baseContext,
		baseMutableState,
		currentContext,
		currentMutableState,
		logger,
	)
}

func recordChildExecutionStarted(
	ctx context.Context,
	logger log.Logger,
	task *persistence.StartChildExecutionTask,
	wfContext execution.Context,
	initiatedAttributes *types.StartChildWorkflowExecutionInitiatedEventAttributes,
	runID string,
	now time.Time,
) error {

	return updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.EntityNotExistsError{Message: "Workflow execution already completed."}
			}

			domain := initiatedAttributes.Domain
			initiatedEventID := task.InitiatedID
			ci, ok := mutableState.GetChildExecutionInfo(initiatedEventID)
			if !ok || ci.StartedID != constants.EmptyEventID {
				return &types.EntityNotExistsError{Message: "Pending child execution not found."}
			}

			_, err := mutableState.AddChildWorkflowExecutionStartedEvent(
				domain,
				&types.WorkflowExecution{
					WorkflowID: task.TargetWorkflowID,
					RunID:      runID,
				},
				initiatedAttributes.WorkflowType,
				initiatedEventID,
				initiatedAttributes.Header,
			)

			return err
		},
		now,
	)
}

func recordStartChildExecutionFailed(
	ctx context.Context,
	logger log.Logger,
	task *persistence.StartChildExecutionTask,
	wfContext execution.Context,
	initiatedAttributes *types.StartChildWorkflowExecutionInitiatedEventAttributes,
	now time.Time,
) error {

	return updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.EntityNotExistsError{Message: "Workflow execution already completed."}
			}

			initiatedEventID := task.InitiatedID
			ci, ok := mutableState.GetChildExecutionInfo(initiatedEventID)
			if !ok || ci.StartedID != constants.EmptyEventID {
				return &types.EntityNotExistsError{Message: "Pending child execution not found."}
			}

			_, err := mutableState.AddStartChildWorkflowExecutionFailedEvent(initiatedEventID,
				types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning, initiatedAttributes)

			return err
		},
		now,
	)
}

// createFirstDecisionTask is used by StartChildExecution transfer task to create the first decision task for
// child execution.
func createFirstDecisionTask(
	ctx context.Context,
	historyClient history.Client,
	domainID string,
	execution *types.WorkflowExecution,
) error {

	scheduleDecisionCtx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()
	err := historyClient.ScheduleDecisionTask(scheduleDecisionCtx, &types.ScheduleDecisionTaskRequest{
		DomainUUID:        domainID,
		WorkflowExecution: execution,
		IsFirstDecision:   true,
	})

	if err != nil {
		switch err.(type) {
		// Maybe child workflow execution already timedout or terminated
		// Safe to discard the error and complete this transfer task
		// cross cluster task need to catch entity not exist error
		// as the target domain may failover before first decision is scheduled.
		case *types.WorkflowExecutionAlreadyCompletedError:
			return nil
		}
	}

	return err
}

func requestCancelExternalExecutionCompleted(
	ctx context.Context,
	logger log.Logger,
	task *persistence.CancelExecutionTask,
	wfContext execution.Context,
	targetDomain string,
	targetWorkflowID string,
	targetRunID string,
	now time.Time,
) error {

	err := updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.WorkflowExecutionAlreadyCompletedError{Message: "Workflow execution already completed."}
			}

			initiatedEventID := task.InitiatedID
			_, ok := mutableState.GetRequestCancelInfo(initiatedEventID)
			if !ok {
				return ErrMissingRequestCancelInfo
			}

			_, err := mutableState.AddExternalWorkflowExecutionCancelRequested(
				initiatedEventID,
				targetDomain,
				targetWorkflowID,
				targetRunID,
			)
			return err
		},
		now,
	)

	switch err.(type) {
	// this could happen if this is a duplicate processing of the task,
	// or the execution has already completed.
	case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError:
		return nil
	}
	return err
}

func signalExternalExecutionCompleted(
	ctx context.Context,
	logger log.Logger,
	task *persistence.SignalExecutionTask,
	wfContext execution.Context,
	targetDomain string,
	targetWorkflowID string,
	targetRunID string,
	control []byte,
	now time.Time,
) error {

	err := updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.WorkflowExecutionAlreadyCompletedError{Message: "Workflow execution already completed."}
			}

			initiatedEventID := task.InitiatedID
			_, ok := mutableState.GetSignalInfo(initiatedEventID)
			if !ok {
				return ErrMissingSignalInfo
			}

			_, err := mutableState.AddExternalWorkflowExecutionSignaled(
				initiatedEventID,
				targetDomain,
				targetWorkflowID,
				targetRunID,
				control,
			)
			return err
		},
		now,
	)

	switch err.(type) {
	// this could happen if this is a duplicate processing of the task,
	// or the execution has already completed.
	case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError:
		return nil
	}

	return err
}

func requestCancelExternalExecutionFailed(
	ctx context.Context,
	logger log.Logger,
	task *persistence.CancelExecutionTask,
	wfContext execution.Context,
	targetDomain string,
	targetWorkflowID string,
	targetRunID string,
	now time.Time,
) error {

	err := updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.WorkflowExecutionAlreadyCompletedError{Message: "Workflow execution already completed."}
			}

			initiatedEventID := task.InitiatedID
			_, ok := mutableState.GetRequestCancelInfo(initiatedEventID)
			if !ok {
				return ErrMissingRequestCancelInfo
			}

			_, err := mutableState.AddRequestCancelExternalWorkflowExecutionFailedEvent(
				constants.EmptyEventID,
				initiatedEventID,
				targetDomain,
				targetWorkflowID,
				targetRunID,
				types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution,
			)
			return err
		},
		now,
	)

	switch err.(type) {
	// this could happen if this is a duplicate processing of the task,
	// or the execution has already completed.
	case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError:
		return nil
	}
	return err
}

func signalExternalExecutionFailed(
	ctx context.Context,
	logger log.Logger,
	task *persistence.SignalExecutionTask,
	wfContext execution.Context,
	targetDomain string,
	targetWorkflowID string,
	targetRunID string,
	control []byte,
	now time.Time,
) error {

	err := updateWorkflowExecution(ctx, logger, wfContext, true,
		func(ctx context.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return &types.WorkflowExecutionAlreadyCompletedError{Message: "Workflow execution already completed."}
			}

			initiatedEventID := task.InitiatedID
			_, ok := mutableState.GetSignalInfo(initiatedEventID)
			if !ok {
				return ErrMissingSignalInfo
			}

			_, err := mutableState.AddSignalExternalWorkflowExecutionFailedEvent(
				constants.EmptyEventID,
				initiatedEventID,
				targetDomain,
				targetWorkflowID,
				targetRunID,
				control,
				types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution,
			)
			return err
		},
		now,
	)

	switch err.(type) {
	// this could happen if this is a duplicate processing of the task,
	// or the execution has already completed.
	case *types.EntityNotExistsError, *types.WorkflowExecutionAlreadyCompletedError:
		return nil
	}

	return err
}

func updateWorkflowExecution(
	ctx context.Context,
	logger log.Logger,
	wfContext execution.Context,
	createDecisionTask bool,
	action func(ctx context.Context, builder execution.MutableState) error,
	now time.Time,
) error {

	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return err
	}

	if err := action(ctx, mutableState); err != nil {
		return err
	}

	if createDecisionTask {
		// Create a transfer task to schedule a decision task
		err := execution.ScheduleDecision(mutableState)
		if err != nil {
			return err
		}
	}

	logger.Debugf("transferActiveTaskExecutor.updateWorkflowExecution calling UpdateWorkflowExecutionAsActive for wfID %s",
		mutableState.GetExecutionInfo().WorkflowID,
	)
	return wfContext.UpdateWorkflowExecutionAsActive(ctx, now)
}

func requestCancelExternalExecutionWithRetry(
	ctx context.Context,
	historyClient history.Client,
	task *persistence.CancelExecutionTask,
	targetDomain string,
	cancelRequestID string,
) error {

	request := &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: task.TargetDomainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			Domain: targetDomain,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: task.TargetWorkflowID,
				RunID:      task.TargetRunID,
			},
			Identity: execution.IdentityHistoryService,
			// Use the same request ID to dedupe RequestCancelWorkflowExecution calls
			RequestID: cancelRequestID,
		},
		ExternalInitiatedEventID: common.Int64Ptr(task.InitiatedID),
		ExternalWorkflowExecution: &types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
		},
		ChildWorkflowOnly: task.TargetChildWorkflowOnly,
	}

	requestCancelCtx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()
	op := func(ctx context.Context) error {
		return historyClient.RequestCancelWorkflowExecution(ctx, request)
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(taskRetryPolicy),
		backoff.WithRetryableError(common.IsServiceTransientError),
	)
	err := throttleRetry.Do(requestCancelCtx, op)
	switch err.(type) {
	case *types.CancellationAlreadyRequestedError:
		// err is CancellationAlreadyRequestedError
		// this could happen if target workflow cancellation is already requested
		// mark as success
		err = nil
	}
	return err
}

func signalExternalExecutionWithRetry(
	ctx context.Context,
	historyClient history.Client,
	task *persistence.SignalExecutionTask,
	targetDomain string,
	signalInfo *persistence.SignalInfo,
) error {

	request := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: task.TargetDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain: targetDomain,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: task.TargetWorkflowID,
				RunID:      task.TargetRunID,
			},
			Identity:   execution.IdentityHistoryService,
			SignalName: signalInfo.SignalName,
			Input:      signalInfo.Input,
			// Use same request ID to deduplicate SignalWorkflowExecution calls
			RequestID: signalInfo.SignalRequestID,
			Control:   signalInfo.Control,
		},
		ExternalWorkflowExecution: &types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
		},
		ChildWorkflowOnly: task.TargetChildWorkflowOnly,
	}

	signalCtx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()
	op := func(ctx context.Context) error {
		return historyClient.SignalWorkflowExecution(ctx, request)
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(taskRetryPolicy),
		backoff.WithRetryableError(common.IsServiceTransientError),
	)
	return throttleRetry.Do(signalCtx, op)
}

func removeSignalMutableStateWithRetry(
	ctx context.Context,
	historyClient history.Client,
	task *persistence.SignalExecutionTask,
	signalRequestID string,
) error {
	ctx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()

	removeSignalRequest := &types.RemoveSignalMutableStateRequest{
		DomainUUID: task.TargetDomainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: task.TargetWorkflowID,
			RunID:      task.TargetRunID,
		},
		RequestID: signalRequestID,
	}

	op := func(ctx context.Context) error {
		return historyClient.RemoveSignalMutableState(ctx, removeSignalRequest)
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(taskRetryPolicy),
		backoff.WithRetryableError(common.IsServiceTransientError),
	)
	err := throttleRetry.Do(ctx, op)
	switch err.(type) {
	case *types.EntityNotExistsError:
		// it's safe to discard entity not exists error here
		// as there's nothing to remove.
		return nil
	}
	return err
}

func startWorkflowWithRetry(
	ctx context.Context,
	historyClient history.Client,
	timeSource clock.TimeSource,
	domainCache cache.DomainCache,
	task *persistence.StartChildExecutionTask,
	targetDomain string,
	requestID string,
	attributes *types.StartChildWorkflowExecutionInitiatedEventAttributes,
	partitionConfig map[string]string,
) (string, error) {

	// Get parent domain name
	domainName, err := domainCache.GetDomainName(task.DomainID)
	if err != nil {
		if _, ok := err.(*types.EntityNotExistsError); !ok {
			return "", err
		}
		// it is possible that the domain got deleted. Use domainID instead as this is only needed for the history event
		domainName = task.DomainID
	}

	frontendStartReq := &types.StartWorkflowExecutionRequest{
		Domain:                              targetDomain,
		WorkflowID:                          attributes.WorkflowID,
		WorkflowType:                        attributes.WorkflowType,
		TaskList:                            attributes.TaskList,
		Input:                               attributes.Input,
		Header:                              attributes.Header,
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      attributes.TaskStartToCloseTimeoutSeconds,
		// Use the same request ID to dedupe StartWorkflowExecution calls
		RequestID:                    requestID,
		WorkflowIDReusePolicy:        attributes.WorkflowIDReusePolicy,
		RetryPolicy:                  attributes.RetryPolicy,
		CronSchedule:                 attributes.CronSchedule,
		CronOverlapPolicy:            attributes.CronOverlapPolicy,
		Memo:                         attributes.Memo,
		SearchAttributes:             attributes.SearchAttributes,
		DelayStartSeconds:            attributes.DelayStartSeconds,
		JitterStartSeconds:           attributes.JitterStartSeconds,
		FirstRunAtTimeStamp:          attributes.FirstRunAtTimestamp,
		ActiveClusterSelectionPolicy: attributes.ActiveClusterSelectionPolicy,
	}

	historyStartReq, err := common.CreateHistoryStartWorkflowRequest(task.TargetDomainID, frontendStartReq, timeSource.Now(), partitionConfig)
	if err != nil {
		return "", err
	}
	historyStartReq.ParentExecutionInfo = &types.ParentExecutionInfo{
		DomainUUID: task.DomainID,
		Domain:     domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
		},
		InitiatedID: task.InitiatedID,
	}

	startWorkflowCtx, cancel := context.WithTimeout(ctx, taskRPCCallTimeout)
	defer cancel()
	var response *types.StartWorkflowExecutionResponse
	op := func(ctx context.Context) error {
		response, err = historyClient.StartWorkflowExecution(ctx, historyStartReq)
		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(taskRetryPolicy),
		backoff.WithRetryableError(common.IsServiceTransientError),
	)
	if err := throttleRetry.Do(startWorkflowCtx, op); err != nil {
		return "", err
	}
	return response.GetRunID(), nil
}

func (t *transferActiveTaskExecutor) resetWorkflow(
	task *persistence.ResetWorkflowTask,
	domain string,
	reason string,
	resetPoint *types.ResetPointInfo,
	baseContext execution.Context,
	baseMutableState execution.MutableState,
	currentContext execution.Context,
	currentMutableState execution.MutableState,
	logger log.Logger,
) error {

	var err error
	resetCtx, cancel := context.WithTimeout(context.Background(), resetWorkflowTimeout)
	defer cancel()

	domainID := task.DomainID
	WorkflowID := task.WorkflowID
	baseRunID := baseMutableState.GetExecutionInfo().RunID
	resetRunID := uuid.New()
	baseRebuildLastEventID := resetPoint.GetFirstDecisionCompletedID() - 1
	baseVersionHistories := baseMutableState.GetVersionHistories()
	if baseVersionHistories == nil {
		return execution.ErrMissingVersionHistories
	}
	baseCurrentVersionHistory, err := baseVersionHistories.GetCurrentVersionHistory()
	if err != nil {
		return err
	}
	baseRebuildLastEventVersion, err := baseCurrentVersionHistory.GetEventVersion(baseRebuildLastEventID)
	if err != nil {
		return err
	}
	baseCurrentBranchToken := baseCurrentVersionHistory.GetBranchToken()
	baseNextEventID := baseMutableState.GetNextEventID()

	err = t.workflowResetter.ResetWorkflow(
		resetCtx,
		domainID,
		WorkflowID,
		baseRunID,
		baseCurrentBranchToken,
		baseRebuildLastEventID,
		baseRebuildLastEventVersion,
		baseNextEventID,
		resetRunID,
		uuid.New(),
		execution.NewWorkflow(
			resetCtx,
			t.shard.GetClusterMetadata(),
			t.shard.GetActiveClusterManager(),
			currentContext,
			currentMutableState,
			execution.NoopReleaseFn, // this is fine since caller will defer on release
			logger,
		),
		reason,
		nil,
		false,
	)

	switch err.(type) {
	case nil:
		return nil

	case *types.BadRequestError:
		// This means the reset point is corrupted and not retry able.
		// There must be a bug in our system that we must fix.(for example, history is not the same in active/passive)
		t.metricsClient.IncCounter(metrics.TransferQueueProcessorScope, metrics.AutoResetPointCorruptionCounter)
		logger.Error("Auto-Reset workflow failed and not retryable. The reset point is corrupted.", tag.Error(err))
		return nil

	default:
		// log this error and retry
		logger.Error("Auto-Reset workflow failed", tag.Error(err))
		return err
	}
}

func (t *transferActiveTaskExecutor) processParentClosePolicy(
	ctx context.Context,
	domainID string,
	domainName string,
	parentExecution *types.WorkflowExecution,
	childInfos map[int64]*persistence.ChildExecutionInfo,
) error {

	if len(childInfos) == 0 {
		return nil
	}

	scope := t.metricsClient.Scope(metrics.TransferActiveTaskCloseExecutionScope)

	if t.shard.GetConfig().EnableParentClosePolicyWorker() &&
		len(childInfos) >= t.shard.GetConfig().ParentClosePolicyThreshold(domainName) {

		batchSize := t.shard.GetConfig().ParentClosePolicyBatchSize(domainName)
		executions := make([]parentclosepolicy.RequestDetail, 0, min(len(childInfos), batchSize))
		count := 0
		for _, childInfo := range childInfos {
			count++
			if childInfo.ParentClosePolicy == types.ParentClosePolicyAbandon {
				continue
			}

			executions = append(executions, parentclosepolicy.RequestDetail{
				DomainID:   domainID,
				DomainName: domainName,
				WorkflowID: childInfo.StartedWorkflowID,
				RunID:      childInfo.StartedRunID,
				Policy:     childInfo.ParentClosePolicy,
			})

			if len(executions) == batchSize {
				err := t.parentClosePolicyClient.SendParentClosePolicyRequest(ctx, parentclosepolicy.Request{
					DomainName: domainName,
					Executions: executions,
				})
				if err != nil {
					return err
				}
				executions = make([]parentclosepolicy.RequestDetail, 0, min(len(childInfos)-count, batchSize))
			}
		}

		if len(executions) == 0 {
			return nil
		}

		return t.parentClosePolicyClient.SendParentClosePolicyRequest(ctx, parentclosepolicy.Request{
			DomainName: domainName,
			Executions: executions,
		})
	}

	for _, childInfo := range childInfos {
		if err := t.applyParentClosePolicy(
			ctx,
			domainID,
			domainName,
			parentExecution,
			childInfo,
		); err != nil {

			switch err.(type) {
			case *types.EntityNotExistsError,
				*types.WorkflowExecutionAlreadyCompletedError,
				*types.CancellationAlreadyRequestedError:
				// expected error, no-op
				break
			default:
				scope.IncCounter(metrics.ParentClosePolicyProcessorFailures)
				return err
			}
		}
		scope.IncCounter(metrics.ParentClosePolicyProcessorSuccess)
	}
	return nil
}

func (t *transferActiveTaskExecutor) applyParentClosePolicy(
	ctx context.Context,
	domainID string,
	domainName string,
	parentWorkflowExecution *types.WorkflowExecution,
	childInfo *persistence.ChildExecutionInfo,
) error {

	switch childInfo.ParentClosePolicy {
	case types.ParentClosePolicyAbandon:
		// noop
		return nil

	case types.ParentClosePolicyTerminate:
		return t.historyClient.TerminateWorkflowExecution(ctx, &types.HistoryTerminateWorkflowExecutionRequest{
			DomainUUID: domainID,
			TerminateRequest: &types.TerminateWorkflowExecutionRequest{
				Domain: domainName,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: childInfo.StartedWorkflowID,
				},
				Reason:   "by parent close policy",
				Identity: execution.IdentityHistoryService,
				// Include StartedRunID as FirstExecutionRunID on the request to allow child to be terminated across runs.
				// If the child does continue as new it still propagates the RunID of first execution.
				FirstExecutionRunID: childInfo.StartedRunID,
			},
			ExternalWorkflowExecution: parentWorkflowExecution,
			ChildWorkflowOnly:         true,
		})

	case types.ParentClosePolicyRequestCancel:
		return t.historyClient.RequestCancelWorkflowExecution(ctx, &types.HistoryRequestCancelWorkflowExecutionRequest{
			DomainUUID: domainID,
			CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
				Domain: domainName,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: childInfo.StartedWorkflowID,
				},
				Identity: execution.IdentityHistoryService,
				// Include StartedRunID as FirstExecutionRunID on the request to allow child to be canceled across runs.
				// If the child does continue as new it still propagates the RunID of first execution.
				FirstExecutionRunID: childInfo.StartedRunID,
			},
			ExternalWorkflowExecution: parentWorkflowExecution,
			ChildWorkflowOnly:         true,
		})

	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("unknown parent close policy: %v", childInfo.ParentClosePolicy),
		}
	}
}

func filterPendingChildExecutions(
	targetDomainIDs map[string]struct{},
	children map[int64]*persistence.ChildExecutionInfo,
	domainCache cache.DomainCache,
	parentDomainEntry *cache.DomainCacheEntry,
) (map[int64]*persistence.ChildExecutionInfo, error) {
	if len(targetDomainIDs) == 0 {
		return children, nil
	}

	filteredChildren := make(map[int64]*persistence.ChildExecutionInfo, len(children))
	for initiatedID, child := range children {
		domainID, err := execution.GetChildExecutionDomainID(child, domainCache, parentDomainEntry)
		if err != nil {
			if common.IsEntityNotExistsError(err) {
				// target domain deleted, ignore the child
				continue
			}
			return nil, err
		}
		if _, ok := targetDomainIDs[domainID]; ok {
			filteredChildren[initiatedID] = child
		}
	}

	return filteredChildren, nil
}
