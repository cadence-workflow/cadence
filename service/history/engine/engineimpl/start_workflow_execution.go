// Copyright (c) 2017-2021 Uber Technologies, Inc.
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

package engineimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

var errClusterAttributeNotFound = &types.BadRequestError{Message: "Cannot start workflow with a cluster attribute that is not found in the domain's metadata."}

// for startWorkflowHelper be reused by signalWithStart
type signalWithStartArg struct {
	signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest
	prevMutableState       execution.MutableState
}

// StartWorkflowExecution starts a workflow execution
func (e *historyEngineImpl) StartWorkflowExecution(
	ctx context.Context,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
) (resp *types.StartWorkflowExecutionResponse, retError error) {
	domainEntry, err := e.shard.GetDomainCache().GetDomainByID(startRequest.DomainUUID)
	if err != nil {
		return nil, err
	}

	return e.startWorkflowHelper(
		ctx,
		startRequest,
		domainEntry,
		metrics.HistoryStartWorkflowExecutionScope,
		nil)
}

func (e *historyEngineImpl) startWorkflowHelper(
	ctx context.Context,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	domainEntry *cache.DomainCacheEntry,
	metricsScope metrics.ScopeIdx,
	signalWithStartArg *signalWithStartArg,
) (resp *types.StartWorkflowExecutionResponse, retError error) {
	if domainEntry.GetInfo().Status != persistence.DomainStatusRegistered {
		return nil, errDomainDeprecated
	}
	request := startRequest.StartRequest

	err := e.validateStartWorkflowExecutionRequest(request, metricsScope)
	if err != nil {
		return nil, err
	}
	e.overrideTaskStartToCloseTimeoutSeconds(domainEntry, request, metricsScope)

	workflowID := request.GetWorkflowID()
	domainID := domainEntry.GetInfo().ID
	domain := domainEntry.GetInfo().Name

	// grab the current context as a lock, nothing more
	// use a smaller context timeout to get the lock
	childCtx, childCancel := e.newChildContext(ctx)
	defer childCancel()

	_, currentRelease, err := e.executionCache.GetOrCreateCurrentWorkflowExecution(
		childCtx,
		domainID,
		workflowID,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return nil, workflow.ErrConcurrentStartRequest
		}
		return nil, err
	}
	defer func() { currentRelease(retError) }()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	curMutableState, err := e.createMutableState(ctx, domainEntry, workflowExecution.GetRunID(), startRequest)
	if err != nil {
		return nil, err
	}

	// preprocess for signalWithStart
	var prevMutableState execution.MutableState
	var signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest
	isSignalWithStart := signalWithStartArg != nil
	if isSignalWithStart {
		prevMutableState = signalWithStartArg.prevMutableState
		signalWithStartRequest = signalWithStartArg.signalWithStartRequest
	}
	if prevMutableState != nil {
		prevLastWriteVersion, err := prevMutableState.GetLastWriteVersion()
		if err != nil {
			return nil, err
		}
		if prevLastWriteVersion > curMutableState.GetCurrentVersion() {
			policy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(ctx, domainID, workflowID, prevMutableState.GetExecutionInfo().RunID)
			if err != nil {
				return nil, err
			}
			if policy.Equals(request.ActiveClusterSelectionPolicy) {
				return nil, e.newDomainNotActiveError(
					domainEntry,
					prevLastWriteVersion,
				)
			}
		}
		err = e.applyWorkflowIDReusePolicyForSigWithStart(
			prevMutableState.GetExecutionInfo(),
			workflowExecution,
			request.GetWorkflowIDReusePolicy(),
		)
		if err != nil {
			return nil, err
		}
	} else if e.shard.GetConfig().EnableRecordWorkflowExecutionUninitialized(domainEntry.GetInfo().Name) && e.visibilityMgr != nil {
		uninitializedRequest := &persistence.RecordWorkflowExecutionUninitializedRequest{
			DomainUUID: domainID,
			Domain:     domain,
			Execution: types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      workflowExecution.RunID,
			},
			WorkflowTypeName: request.WorkflowType.Name,
			UpdateTimestamp:  e.shard.GetTimeSource().Now().UnixNano(),
			ShardID:          int64(e.shard.GetShardID()),
		}

		if err := e.visibilityMgr.RecordWorkflowExecutionUninitialized(ctx, uninitializedRequest); err != nil {
			e.logger.Error("Failed to record uninitialized workflow execution", tag.Error(err))
		}
	}

	err = e.addStartEventsAndTasks(
		curMutableState,
		workflowExecution,
		startRequest,
		signalWithStartRequest,
	)
	if err != nil {
		if e.shard.GetConfig().EnableRecordWorkflowExecutionUninitialized(domainEntry.GetInfo().Name) && e.visibilityMgr != nil {
			// delete the uninitialized workflow execution record since it failed to start the workflow
			// uninitialized record is used to find wfs that didn't make a progress or stuck during the start process
			if errVisibility := e.visibilityMgr.DeleteWorkflowExecution(ctx, &persistence.VisibilityDeleteWorkflowExecutionRequest{
				DomainID:   domainID,
				Domain:     domain,
				RunID:      workflowExecution.RunID,
				WorkflowID: workflowID,
			}); errVisibility != nil {
				e.logger.Error("Failed to delete uninitialized workflow execution record", tag.Error(errVisibility))
			}
		}

		return nil, err
	}
	wfContext := execution.NewContext(domainID, workflowExecution, e.shard, e.executionManager, e.logger)

	newWorkflow, newWorkflowEventsSeq, err := curMutableState.CloseTransactionAsSnapshot(
		e.timeSource.Now(),
		execution.TransactionPolicyActive,
	)
	if err != nil {
		return nil, err
	}
	historyBlob, err := wfContext.PersistStartWorkflowBatchEvents(ctx, newWorkflowEventsSeq[0])
	if err != nil {
		return nil, err
	}

	// create as brand new
	createMode := persistence.CreateWorkflowModeBrandNew
	prevRunID := ""
	prevLastWriteVersion := int64(0)
	// overwrite in case of signalWithStart
	if prevMutableState != nil {
		createMode = persistence.CreateWorkflowModeWorkflowIDReuse
		info := prevMutableState.GetExecutionInfo()
		// For corrupted workflows use ContinueAsNew mode.
		// WorkflowIDReuse mode require workflows to be in completed state, which is not necessarily true for corrupted workflows.
		if info.State == persistence.WorkflowStateCorrupted {
			createMode = persistence.CreateWorkflowModeContinueAsNew
		}
		prevRunID = info.RunID
		prevLastWriteVersion, err = prevMutableState.GetLastWriteVersion()
		if err != nil {
			return nil, err
		}
	}
	err = wfContext.CreateWorkflowExecution(
		ctx,
		newWorkflow,
		historyBlob,
		createMode,
		prevRunID,
		prevLastWriteVersion,
		persistence.CreateWorkflowRequestModeNew,
	)

	// The history branch (tree and nodes) was created above (PersistStartWorkflowBatchEvents)
	// but the workflow execution returned an error, and it was probably not created.
	// This may leave orphaned history_tree and history_node records that are never referenced by any workflow.
	// If the error is of type WorkflowExecutionAlreadyStartedError or DuplicateRequestError, the workflow was not and
	// didn't need to be created.
	// Remove history in these cases.
	// All other cases, log them for future leak investigations.
	if err != nil {
		var workflowExecutionAlreadyStartedError *persistence.WorkflowExecutionAlreadyStartedError
		isAlreadyStarted := errors.As(err, &workflowExecutionAlreadyStartedError)
		_, isDuplicateRequest := persistence.AsDuplicateRequestError(err)

		if isAlreadyStarted || isDuplicateRequest {
			if e.shard.GetConfig().EnableCleanupOrphanedHistoryBranchOnWorkflowCreation() {
				if cleanupErr := e.shard.GetHistoryManager().DeleteHistoryBranch(ctx, &persistence.DeleteHistoryBranchRequest{
					BranchToken: historyBlob.BranchToken,
					ShardID:     common.IntPtr(e.shard.GetShardID()),
					DomainName:  domain,
				}); cleanupErr != nil {
					e.logger.Error("Failed to cleanup history branch after CreateWorkflowExecution failure",
						tag.WorkflowDomainID(domainID),
						tag.WorkflowID(workflowID),
						tag.WorkflowRunID(workflowExecution.RunID),
						tag.Error(cleanupErr))
				}
			} else {
				e.logger.Warn("Orphaned history branch created due to WorkflowExecutionAlreadyStartedError or DuplicateRequestError",
					tag.WorkflowDomainID(domainID),
					tag.WorkflowID(workflowID),
					tag.WorkflowRunID(workflowExecution.RunID),
					tag.Error(err))
			}
		} else {
			e.logger.Error("Possible orphaned history branch due to unexpected error from CreateWorkflowExecution",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(workflowID),
				tag.WorkflowRunID(workflowExecution.RunID),
				tag.Error(err))
		}
	}
	if t, ok := persistence.AsDuplicateRequestError(err); ok {
		if t.RequestType == persistence.WorkflowRequestTypeStart || (isSignalWithStart && t.RequestType == persistence.WorkflowRequestTypeSignal) {
			return &types.StartWorkflowExecutionResponse{
				RunID: t.RunID,
			}, nil
		}
		e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
		return nil, t
	}
	// handle already started error
	if t, ok := err.(*persistence.WorkflowExecutionAlreadyStartedError); ok {
		if t.StartRequestID == request.GetRequestID() {
			return &types.StartWorkflowExecutionResponse{
				RunID: t.RunID,
			}, nil
		}

		if isSignalWithStart {
			return nil, err
		}

		if curMutableState.GetCurrentVersion() < t.LastWriteVersion {
			policy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(ctx, domainID, workflowID, t.RunID)
			if err != nil {
				return nil, err
			}
			if policy.Equals(request.ActiveClusterSelectionPolicy) {
				return nil, e.newDomainNotActiveError(
					domainEntry,
					t.LastWriteVersion,
				)
			}
		}

		prevRunID = t.RunID
		if shouldTerminateAndStart(startRequest, t.State) {
			runningWFCtx, err := workflow.LoadOnce(ctx, e.executionCache, domainID, workflowID, prevRunID)
			if err != nil {
				return nil, err
			}
			defer func() { runningWFCtx.GetReleaseFn()(retError) }()

			resp, err = e.terminateAndStartWorkflow(
				ctx,
				runningWFCtx,
				workflowExecution,
				domainEntry,
				domainID,
				startRequest,
				nil,
			)
			switch err.(type) {
			// By the time we try to terminate the workflow, it was already terminated
			// So continue as if we didn't need to terminate it in the first place
			case *types.WorkflowExecutionAlreadyCompletedError:
				e.shard.GetLogger().Warn("Workflow completed while trying to terminate, will continue starting workflow", tag.Error(err))
			default:
				return resp, err
			}
		}
		if err = e.applyWorkflowIDReusePolicyHelper(
			t.StartRequestID,
			prevRunID,
			t.State,
			t.CloseStatus,
			workflowExecution,
			startRequest.StartRequest.GetWorkflowIDReusePolicy(),
		); err != nil {
			return nil, err
		}
		// create as ID reuse
		createMode = persistence.CreateWorkflowModeWorkflowIDReuse
		err = wfContext.CreateWorkflowExecution(
			ctx,
			newWorkflow,
			historyBlob,
			createMode,
			prevRunID,
			t.LastWriteVersion,
			persistence.CreateWorkflowRequestModeNew,
		)
		if t, ok := persistence.AsDuplicateRequestError(err); ok {
			if t.RequestType == persistence.WorkflowRequestTypeStart || (isSignalWithStart && t.RequestType == persistence.WorkflowRequestTypeSignal) {
				return &types.StartWorkflowExecutionResponse{
					RunID: t.RunID,
				}, nil
			}
			e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
			return nil, t
		}
	}
	if err != nil {
		return nil, err
	}

	return &types.StartWorkflowExecutionResponse{
		RunID: workflowExecution.RunID,
	}, nil
}

// TODO(active-active): Review this method for active-active domains
func (e *historyEngineImpl) SignalWithStartWorkflowExecution(
	ctx context.Context,
	signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest,
) (retResp *types.StartWorkflowExecutionResponse, retError error) {
	domainEntry, err := e.shard.GetDomainCache().GetDomainByID(signalWithStartRequest.DomainUUID)
	if err != nil {
		return nil, err
	}
	if domainEntry.GetInfo().Status != persistence.DomainStatusRegistered {
		return nil, errDomainDeprecated
	}
	domainID := domainEntry.GetInfo().ID

	sRequest := signalWithStartRequest.SignalWithStartRequest
	workflowExecution := types.WorkflowExecution{
		WorkflowID: sRequest.WorkflowID,
	}

	var prevMutableState execution.MutableState
	attempt := 0

	wfContext, release, err0 := e.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, workflowExecution)

	if err0 == nil {
		defer func() { release(retError) }()
	Just_Signal_Loop:
		for ; attempt < workflow.ConditionalRetryCount; attempt++ {
			// workflow not exist, will create workflow then signal
			mutableState, err1 := wfContext.LoadWorkflowExecution(ctx)
			if err1 != nil {
				if _, ok := err1.(*types.EntityNotExistsError); ok {
					break
				}
				return nil, err1
			}

			if mutableState.IsSignalRequested(sRequest.GetRequestID()) {
				return &types.StartWorkflowExecutionResponse{RunID: wfContext.GetExecution().RunID}, nil
			}

			// workflow exist but not running, will restart workflow then signal
			if !mutableState.IsWorkflowExecutionRunning() {
				prevMutableState = mutableState
				break
			}

			// workflow exists but history is corrupted, will restart workflow then signal
			if corrupted, err := e.checkForHistoryCorruptions(ctx, mutableState); err != nil {
				return nil, err
			} else if corrupted {
				prevMutableState = mutableState
				break
			}

			// workflow is running, if policy is TerminateIfRunning, terminate current run then signalWithStart
			if sRequest.GetWorkflowIDReusePolicy() == types.WorkflowIDReusePolicyTerminateIfRunning {
				workflowExecution.RunID = uuid.New()
				runningWFCtx := workflow.NewContext(wfContext, release, mutableState)
				resp, errTerm := e.terminateAndStartWorkflow(
					ctx,
					runningWFCtx,
					workflowExecution,
					domainEntry,
					domainID,
					nil,
					signalWithStartRequest,
				)
				// By the time we try to terminate the workflow, it was already terminated
				// So continue as if we didn't need to terminate it in the first place
				if _, ok := errTerm.(*types.WorkflowExecutionAlreadyCompletedError); !ok {
					return resp, errTerm
				}
			}

			executionInfo := mutableState.GetExecutionInfo()
			maxAllowedSignals := e.config.MaximumSignalsPerExecution(domainEntry.GetInfo().Name)
			if maxAllowedSignals > 0 && int(executionInfo.SignalCount) >= maxAllowedSignals {
				e.logger.Info("Execution limit reached for maximum signals", tag.WorkflowSignalCount(executionInfo.SignalCount),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowDomainID(domainID))
				return nil, workflow.ErrSignalsLimitExceeded
			}

			requestID := sRequest.GetRequestID()
			if requestID != "" {
				mutableState.AddSignalRequested(requestID)
			}

			if _, err := mutableState.AddWorkflowExecutionSignaled(
				sRequest.GetSignalName(),
				sRequest.GetSignalInput(),
				sRequest.GetIdentity(),
				sRequest.GetRequestID(),
			); err != nil {
				return nil, &types.InternalServiceError{Message: "Unable to signal workflow execution."}
			}

			// Create a transfer task to schedule a decision task
			if !mutableState.HasPendingDecision() {
				_, err := mutableState.AddDecisionTaskScheduledEvent(false)
				if err != nil {
					return nil, &types.InternalServiceError{Message: "Failed to add decision scheduled event."}
				}
			}

			// We apply the update to execution using optimistic concurrency.  If it fails due to a conflict then reload
			// the history and try the operation again.
			e.logger.Debugf("SignalWithStartWorkflowExecution calling UpdateWorkflowExecutionAsActive for wfID %s",
				workflowExecution.GetWorkflowID(),
			)
			if err := wfContext.UpdateWorkflowExecutionAsActive(ctx, e.shard.GetTimeSource().Now()); err != nil {
				if t, ok := persistence.AsDuplicateRequestError(err); ok {
					if t.RequestType == persistence.WorkflowRequestTypeSignal {
						return &types.StartWorkflowExecutionResponse{RunID: t.RunID}, nil
					}
					e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
					return nil, t
				}
				if execution.IsConflictError(err) {
					continue Just_Signal_Loop
				}
				return nil, err
			}
			return &types.StartWorkflowExecutionResponse{RunID: wfContext.GetExecution().RunID}, nil
		} // end for Just_Signal_Loop
		if attempt == workflow.ConditionalRetryCount {
			return nil, workflow.ErrMaxAttemptsExceeded
		}
	} else {
		if _, ok := err0.(*types.EntityNotExistsError); !ok {
			return nil, err0
		}
		// workflow not exist, will create workflow then signal
	}

	// Start workflow and signal
	startRequest, err := getStartRequest(domainID, sRequest, signalWithStartRequest.PartitionConfig)
	if err != nil {
		return nil, err
	}

	sigWithStartArg := &signalWithStartArg{
		signalWithStartRequest: signalWithStartRequest,
		prevMutableState:       prevMutableState,
	}
	return e.startWorkflowHelper(
		ctx,
		startRequest,
		domainEntry,
		metrics.HistorySignalWithStartWorkflowExecutionScope,
		sigWithStartArg,
	)
}

func getStartRequest(
	domainID string,
	request *types.SignalWithStartWorkflowExecutionRequest,
	partitionConfig map[string]string,
) (*types.HistoryStartWorkflowExecutionRequest, error) {

	req := &types.StartWorkflowExecutionRequest{
		Domain:                              request.Domain,
		WorkflowID:                          request.WorkflowID,
		WorkflowType:                        request.WorkflowType,
		TaskList:                            request.TaskList,
		Input:                               request.Input,
		ExecutionStartToCloseTimeoutSeconds: request.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      request.TaskStartToCloseTimeoutSeconds,
		Identity:                            request.Identity,
		RequestID:                           request.RequestID,
		WorkflowIDReusePolicy:               request.WorkflowIDReusePolicy,
		RetryPolicy:                         request.RetryPolicy,
		CronSchedule:                        request.CronSchedule,
		CronOverlapPolicy:                   request.CronOverlapPolicy,
		Memo:                                request.Memo,
		SearchAttributes:                    request.SearchAttributes,
		Header:                              request.Header,
		DelayStartSeconds:                   request.DelayStartSeconds,
		JitterStartSeconds:                  request.JitterStartSeconds,
		FirstRunAtTimeStamp:                 request.FirstRunAtTimestamp,
		ActiveClusterSelectionPolicy:        request.ActiveClusterSelectionPolicy,
	}

	return common.CreateHistoryStartWorkflowRequest(domainID, req, time.Now(), partitionConfig)
}

func shouldTerminateAndStart(startRequest *types.HistoryStartWorkflowExecutionRequest, state int) bool {
	return startRequest.StartRequest.GetWorkflowIDReusePolicy() == types.WorkflowIDReusePolicyTerminateIfRunning &&
		persistence.IsWorkflowRunning(state)
}

func (e *historyEngineImpl) validateStartWorkflowExecutionRequest(request *types.StartWorkflowExecutionRequest, metricsScope metrics.ScopeIdx) error {
	if len(request.GetRequestID()) == 0 {
		return &types.BadRequestError{Message: "Missing request ID."}
	}
	if request.ExecutionStartToCloseTimeoutSeconds == nil || request.GetExecutionStartToCloseTimeoutSeconds() <= 0 {
		return &types.BadRequestError{Message: "Missing or invalid ExecutionStartToCloseTimeoutSeconds."}
	}
	if request.TaskStartToCloseTimeoutSeconds == nil || request.GetTaskStartToCloseTimeoutSeconds() <= 0 {
		return &types.BadRequestError{Message: "Missing or invalid TaskStartToCloseTimeoutSeconds."}
	}
	if request.TaskList == nil || request.TaskList.GetName() == "" {
		return &types.BadRequestError{Message: "Missing Tasklist."}
	}
	if request.WorkflowType == nil || request.WorkflowType.GetName() == "" {
		return &types.BadRequestError{Message: "Missing WorkflowType."}
	}

	if !common.IsValidIDLength(
		request.GetDomain(),
		e.metricsClient.Scope(metricsScope),
		e.config.MaxIDLengthWarnLimit(),
		e.config.DomainNameMaxLength(request.GetDomain()),
		metrics.CadenceErrDomainNameExceededWarnLimit,
		request.GetDomain(),
		e.logger,
		tag.IDTypeDomainName) {
		return &types.BadRequestError{Message: "Domain exceeds length limit."}
	}

	if !common.IsValidIDLength(
		request.GetWorkflowID(),
		e.metricsClient.Scope(metricsScope),
		e.config.MaxIDLengthWarnLimit(),
		e.config.WorkflowIDMaxLength(request.GetDomain()),
		metrics.CadenceErrWorkflowIDExceededWarnLimit,
		request.GetDomain(),
		e.logger,
		tag.IDTypeWorkflowID) {
		return &types.BadRequestError{Message: "WorkflowId exceeds length limit."}
	}
	if !common.IsValidIDLength(
		request.TaskList.GetName(),
		e.metricsClient.Scope(metricsScope),
		e.config.MaxIDLengthWarnLimit(),
		e.config.TaskListNameMaxLength(request.GetDomain()),
		metrics.CadenceErrTaskListNameExceededWarnLimit,
		request.GetDomain(),
		e.logger,
		tag.IDTypeTaskListName) {
		return &types.BadRequestError{Message: "TaskList exceeds length limit."}
	}
	if !common.IsValidIDLength(
		request.WorkflowType.GetName(),
		e.metricsClient.Scope(metricsScope),
		e.config.MaxIDLengthWarnLimit(),
		e.config.WorkflowTypeMaxLength(request.GetDomain()),
		metrics.CadenceErrWorkflowTypeExceededWarnLimit,
		request.GetDomain(),
		e.logger,
		tag.IDTypeWorkflowType) {
		return &types.BadRequestError{Message: "WorkflowType exceeds length limit."}
	}

	return common.ValidateRetryPolicy(request.RetryPolicy)
}

func (e *historyEngineImpl) overrideTaskStartToCloseTimeoutSeconds(
	domainEntry *cache.DomainCacheEntry,
	request *types.StartWorkflowExecutionRequest,
	metricsScope metrics.ScopeIdx,
) {
	domainName := domainEntry.GetInfo().Name
	maxDecisionStartToCloseTimeoutSeconds := int32(e.config.MaxDecisionStartToCloseSeconds(domainName))
	taskStartToCloseTimeoutSecs := request.GetTaskStartToCloseTimeoutSeconds()
	taskStartToCloseTimeoutSecs = min(taskStartToCloseTimeoutSecs, maxDecisionStartToCloseTimeoutSeconds)
	taskStartToCloseTimeoutSecs = min(taskStartToCloseTimeoutSecs, request.GetExecutionStartToCloseTimeoutSeconds())

	if taskStartToCloseTimeoutSecs != request.GetTaskStartToCloseTimeoutSeconds() {
		request.TaskStartToCloseTimeoutSeconds = &taskStartToCloseTimeoutSecs
		e.metricsClient.Scope(
			metricsScope,
			metrics.DomainTag(domainName),
		).IncCounter(metrics.DecisionStartToCloseTimeoutOverrideCount)
	}
}

// terminate running workflow then start a new run in one transaction
func (e *historyEngineImpl) terminateAndStartWorkflow(
	ctx context.Context,
	runningWFCtx workflow.Context,
	workflowExecution types.WorkflowExecution,
	domainEntry *cache.DomainCacheEntry,
	domainID string,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest,
) (*types.StartWorkflowExecutionResponse, error) {
	runningMutableState := runningWFCtx.GetMutableState()
	var err error
	if signalWithStartRequest != nil {
		startRequest, err = getStartRequest(domainID, signalWithStartRequest.SignalWithStartRequest, signalWithStartRequest.PartitionConfig)
		if err != nil {
			return nil, err
		}
	}

	activeCluster, err := e.clusterMetadata.ClusterNameForFailoverVersion(runningMutableState.GetCurrentVersion())
	if err != nil {
		return nil, err
	}
	if activeCluster != e.currentClusterName {
		if runningMutableState.GetExecutionInfo().ActiveClusterSelectionPolicy.Equals(startRequest.StartRequest.ActiveClusterSelectionPolicy) {
			return nil, e.newDomainNotActiveError(domainEntry, runningMutableState.GetCurrentVersion())
		}
		// TODO(active-active): This is a short-term fix to handle this special case, because we don't have a way to terminate the existing workflow in a different cluster and start a new workflow in the current cluster
		// atomically in one transaction. We'll review this when we have time to implement a better solution.
		return nil, &types.BadRequestError{Message: "Cannot terminate the existing workflow and start a new workflow because it is active in a different cluster with a different active cluster selection policy."}
	}
UpdateWorkflowLoop:
	for attempt := 0; attempt < workflow.ConditionalRetryCount; attempt++ {
		if !runningMutableState.IsWorkflowExecutionRunning() {
			return nil, workflow.ErrAlreadyCompleted
		}

		if err := execution.TerminateWorkflow(
			runningMutableState,
			runningMutableState.GetNextEventID(),
			TerminateIfRunningReason,
			getTerminateIfRunningDetails(workflowExecution.GetRunID()),
			execution.IdentityHistoryService,
		); err != nil {
			if err == workflow.ErrStaleState {
				// Handler detected that cached workflow mutable could potentially be stale
				// Reload workflow execution history
				runningWFCtx.GetContext().Clear()
				if attempt != workflow.ConditionalRetryCount-1 {
					_, err = runningWFCtx.ReloadMutableState(ctx)
					if err != nil {
						return nil, err
					}
				}
				continue UpdateWorkflowLoop
			}
			return nil, err
		}

		// new mutable state
		newMutableState, err := e.createMutableState(ctx, domainEntry, workflowExecution.GetRunID(), startRequest)
		if err != nil {
			return nil, err
		}

		err = e.addStartEventsAndTasks(
			newMutableState,
			workflowExecution,
			startRequest,
			signalWithStartRequest,
		)
		if err != nil {
			return nil, err
		}

		updateErr := runningWFCtx.GetContext().UpdateWorkflowExecutionWithNewAsActive(
			ctx,
			e.timeSource.Now(),
			execution.NewContext(
				domainID,
				workflowExecution,
				e.shard,
				e.shard.GetExecutionManager(),
				e.logger,
			),
			newMutableState,
		)
		if updateErr != nil {
			if execution.IsConflictError(updateErr) {
				e.metricsClient.IncCounter(metrics.HistoryStartWorkflowExecutionScope, metrics.ConcurrencyUpdateFailureCounter)
				continue UpdateWorkflowLoop
			}
			return nil, updateErr
		}
		break UpdateWorkflowLoop
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: workflowExecution.RunID,
	}, nil
}

func (e *historyEngineImpl) addStartEventsAndTasks(
	mutableState execution.MutableState,
	workflowExecution types.WorkflowExecution,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest,
) error {
	// Add WF start event
	startEvent, err := mutableState.AddWorkflowExecutionStartedEvent(
		workflowExecution,
		startRequest,
	)
	if err != nil {
		return &types.InternalServiceError{
			Message: "Failed to add workflow execution started event.",
		}
	}

	if signalWithStartRequest != nil {
		// Add signal event
		sRequest := signalWithStartRequest.SignalWithStartRequest
		if sRequest.GetRequestID() != "" {
			mutableState.AddSignalRequested(sRequest.GetRequestID())
		}
		_, err := mutableState.AddWorkflowExecutionSignaled(
			sRequest.GetSignalName(),
			sRequest.GetSignalInput(),
			sRequest.GetIdentity(),
			sRequest.GetRequestID(),
		)
		if err != nil {
			return &types.InternalServiceError{Message: "Failed to add workflow execution signaled event."}
		}
	}

	// Generate first decision task event if not child WF and no first decision task backoff
	return e.generateFirstDecisionTask(
		mutableState,
		startRequest.ParentExecutionInfo,
		startEvent,
	)
}

func getTerminateIfRunningDetails(newRunID string) []byte {
	return []byte(fmt.Sprintf(TerminateIfRunningDetailsTemplate, newRunID))
}

func (e *historyEngineImpl) applyWorkflowIDReusePolicyForSigWithStart(
	prevExecutionInfo *persistence.WorkflowExecutionInfo,
	execution types.WorkflowExecution,
	wfIDReusePolicy types.WorkflowIDReusePolicy,
) error {

	prevStartRequestID := prevExecutionInfo.CreateRequestID
	prevRunID := prevExecutionInfo.RunID
	prevState := prevExecutionInfo.State
	prevCloseState := prevExecutionInfo.CloseStatus

	return e.applyWorkflowIDReusePolicyHelper(
		prevStartRequestID,
		prevRunID,
		prevState,
		prevCloseState,
		execution,
		wfIDReusePolicy,
	)
}

func (e *historyEngineImpl) applyWorkflowIDReusePolicyHelper(
	prevStartRequestID,
	prevRunID string,
	prevState int,
	prevCloseState int,
	execution types.WorkflowExecution,
	wfIDReusePolicy types.WorkflowIDReusePolicy,
) error {

	// here we know some information about the prev workflow, i.e. either running right now
	// or has history check if the workflow is finished
	switch prevState {
	case persistence.WorkflowStateCreated,
		persistence.WorkflowStateRunning:
		msg := "Workflow execution is already running. WorkflowId: %v, RunId: %v."
		return getWorkflowAlreadyStartedError(msg, prevStartRequestID, execution.GetWorkflowID(), prevRunID)
	case persistence.WorkflowStateCompleted:
		// previous workflow completed, proceed
	case persistence.WorkflowStateCorrupted:
		// ignore workflow ID reuse policy for corrupted workflows, treat as they do not exist
		return nil
	default:
		// persistence.WorkflowStateZombie or unknown type
		return &types.InternalServiceError{Message: fmt.Sprintf("Failed to process workflow, workflow has invalid state: %v.", prevState)}
	}

	switch wfIDReusePolicy {
	case types.WorkflowIDReusePolicyAllowDuplicateFailedOnly:
		if _, ok := FailedWorkflowCloseState[prevCloseState]; !ok {
			msg := "Workflow execution already finished successfully. WorkflowId: %v, RunId: %v. Workflow ID reuse policy: allow duplicate workflow ID if last run failed."
			return getWorkflowAlreadyStartedError(msg, prevStartRequestID, execution.GetWorkflowID(), prevRunID)
		}
	case types.WorkflowIDReusePolicyAllowDuplicate,
		types.WorkflowIDReusePolicyTerminateIfRunning:
		// no check need here
	case types.WorkflowIDReusePolicyRejectDuplicate:
		msg := "Workflow execution already finished. WorkflowId: %v, RunId: %v. Workflow ID reuse policy: reject duplicate workflow ID."
		return getWorkflowAlreadyStartedError(msg, prevStartRequestID, execution.GetWorkflowID(), prevRunID)
	default:
		return &types.InternalServiceError{Message: "Failed to process start workflow reuse policy."}
	}

	return nil
}

func getWorkflowAlreadyStartedError(errMsg string, createRequestID string, workflowID string, runID string) error {
	return &types.WorkflowExecutionAlreadyStartedError{
		Message:        fmt.Sprintf(errMsg, workflowID, runID),
		StartRequestID: createRequestID,
		RunID:          runID,
	}
}

func (e *historyEngineImpl) newChildContext(
	parentCtx context.Context,
) (context.Context, context.CancelFunc) {

	ctxTimeout := contextLockTimeout
	if deadline, ok := parentCtx.Deadline(); ok {
		now := e.shard.GetTimeSource().Now()
		parentTimeout := deadline.Sub(now)
		if parentTimeout > 0 && parentTimeout < contextLockTimeout {
			ctxTimeout = parentTimeout
		}
	}
	return context.WithTimeout(context.Background(), ctxTimeout)
}

func (e *historyEngineImpl) createMutableState(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	runID string,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
) (execution.MutableState, error) {
	activeClusterInfo, err := e.shard.GetActiveClusterManager().GetActiveClusterInfoByClusterAttribute(ctx, domainEntry.GetInfo().ID, startRequest.StartRequest.ActiveClusterSelectionPolicy.GetClusterAttribute())
	if err != nil {
		var errNotFound *activecluster.ClusterAttributeNotFoundError
		if !errors.As(err, &errNotFound) {
			// unexpected error
			return nil, err
		}
		e.logger.Warn("Failed to get active cluster info by cluster attribute, falling back to domain-level active cluster info", tag.Error(err))
		return nil, errClusterAttributeNotFound
	}
	if activeClusterInfo.ActiveClusterName != e.currentClusterName {
		return nil, e.newDomainNotActiveError(domainEntry, activeClusterInfo.FailoverVersion)
	}

	newMutableState := execution.NewMutableStateBuilderWithVersionHistories(
		e.shard,
		e.logger,
		domainEntry,
		activeClusterInfo.FailoverVersion,
	)

	if err := newMutableState.SetHistoryTree(runID); err != nil {
		return nil, err
	}

	return newMutableState, nil
}

func (e *historyEngineImpl) generateFirstDecisionTask(
	mutableState execution.MutableState,
	parentInfo *types.ParentExecutionInfo,
	startEvent *types.HistoryEvent,
) error {

	if parentInfo == nil {
		// DecisionTask is only created when it is not a Child Workflow and no backoff is needed
		if err := mutableState.AddFirstDecisionTaskScheduled(
			startEvent,
		); err != nil {
			return err
		}
	}
	return nil
}
