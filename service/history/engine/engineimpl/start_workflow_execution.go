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
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

var errClusterAttributeNotFound = &types.BadRequestError{Message: "Cannot start workflow with a cluster attribute that is not found in the domain's metadata."}

// messages of the errors of a child start that can not adopt the run with the same request ID
const (
	childRunClosedByOtherClusterMsg = "Workflow execution already finished in another cluster."
	childChainEndUnknownMsg         = "Workflow execution started by this request already finished, and the cluster that closed it can not be determined, so it can not be adopted."
	childRunNotFoundMsg             = "Workflow execution was started by this request but no longer exists, so it can not be adopted."
	childRunZombieMsg               = "Workflow execution started by this request is a zombie, so it can not be adopted."
)

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

	resp, workflowExecution, historyBlob, err := e.startWorkflowHelper(
		ctx,
		startRequest,
		domainEntry,
		metrics.HistoryStartWorkflowExecutionScope,
		nil)
	if err != nil {
		e.handleCreateWorkflowExecutionFailureCleanup(ctx,
			startRequest,
			domainEntry,
			workflowExecution,
			historyBlob,
			false,
			err,
		)
		return nil, err
	}
	return resp, nil
}

func (e *historyEngineImpl) startWorkflowHelper(
	ctx context.Context,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	domainEntry *cache.DomainCacheEntry,
	metricsScope metrics.ScopeIdx,
	sigWithStartArg *signalWithStartArg,
) (_ *types.StartWorkflowExecutionResponse, _ *types.WorkflowExecution, _ *events.PersistedBlob, retError error) {
	if domainEntry.GetInfo().Status != persistence.DomainStatusRegistered {
		return nil, nil, nil, errDomainDeprecated
	}
	request := startRequest.StartRequest

	err := e.validateStartWorkflowExecutionRequest(request, metricsScope)
	if err != nil {
		return nil, nil, nil, err
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
			return nil, nil, nil, workflow.ErrConcurrentStartRequest
		}
		return nil, nil, nil, err
	}
	defer func() { currentRelease(retError) }()

	workflowExecution := &types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      uuid.New(),
	}
	curMutableState, err := e.createMutableState(ctx, domainEntry, workflowExecution.GetRunID(), startRequest)
	if err != nil {
		return nil, nil, nil, err
	}

	// preprocess for signalWithStart
	var prevMutableState execution.MutableState
	var signalWithStartRequest *types.HistorySignalWithStartWorkflowExecutionRequest
	isSignalWithStart := sigWithStartArg != nil
	isChildStart := startRequest.ParentExecutionInfo != nil
	if isSignalWithStart {
		prevMutableState = sigWithStartArg.prevMutableState
		signalWithStartRequest = sigWithStartArg.signalWithStartRequest
	}
	if prevMutableState != nil {
		prevLastWriteVersion, err := prevMutableState.GetLastWriteVersion()
		if err != nil {
			return nil, nil, nil, err
		}
		if prevLastWriteVersion > curMutableState.GetCurrentVersion() {
			policy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(ctx, domainID, workflowID, prevMutableState.GetExecutionInfo().RunID)
			if err != nil {
				return nil, nil, nil, err
			}
			if policy.Equals(request.ActiveClusterSelectionPolicy) {
				return nil, nil, nil, e.newDomainNotActiveError(
					domainEntry,
					prevLastWriteVersion,
				)
			}
		}
		err = e.applyWorkflowIDReusePolicyForSigWithStart(
			prevMutableState.GetExecutionInfo(),
			*workflowExecution,
			request.GetWorkflowIDReusePolicy(),
		)
		if err != nil {
			return nil, nil, nil, err
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
		*workflowExecution,
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
		return nil, nil, nil, err
	}
	wfContext := execution.NewContext(domainID, *workflowExecution, e.shard, e.executionManager, e.logger)

	newWorkflow, newWorkflowEventsSeq, err := curMutableState.CloseTransactionAsSnapshot(
		e.timeSource.Now(),
		execution.TransactionPolicyActive,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	b, err := wfContext.PersistStartWorkflowBatchEvents(ctx, newWorkflowEventsSeq[0])
	if err != nil {
		return nil, workflowExecution, nil, err
	}
	historyBlob := &b

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
			return nil, workflowExecution, historyBlob, err
		}
	}
	err = wfContext.CreateWorkflowExecution(
		ctx,
		newWorkflow,
		*historyBlob,
		createMode,
		prevRunID,
		prevLastWriteVersion,
		persistence.CreateWorkflowRequestModeNew,
	)
	// only NoSQL stores with history.enableStrongIdempotency enabled return a duplicate request error
	if t, ok := persistence.AsDuplicateRequestError(err); ok {
		if t.RequestType == persistence.WorkflowRequestTypeStart || (isSignalWithStart && t.RequestType == persistence.WorkflowRequestTypeSignal) {
			if isChildStart {
				if err := e.validateDuplicateChildRunAdoption(ctx, startRequest, domainEntry, t.RunID); err != nil {
					return nil, workflowExecution, historyBlob, err
				}
			}
			return &types.StartWorkflowExecutionResponse{
				RunID: t.RunID,
			}, workflowExecution, historyBlob, nil
		}
		e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
		return nil, workflowExecution, historyBlob, t
	}
	// handle already started error
	if t, ok := err.(*persistence.WorkflowExecutionAlreadyStartedError); ok {
		if t.StartRequestID == request.GetRequestID() {
			if isChildStart {
				// t is the current run, so it is the last run of its chain
				currentRun := childRunInfo{
					runID:            t.RunID,
					state:            t.State,
					closeStatus:      t.CloseStatus,
					lastWriteVersion: t.LastWriteVersion,
				}
				if err := e.validateLastRunOfChain(startRequest, currentRun); err != nil {
					return nil, workflowExecution, historyBlob, err
				}
			}
			return &types.StartWorkflowExecutionResponse{
				RunID: t.RunID,
			}, workflowExecution, historyBlob, nil
		}

		if isSignalWithStart {
			e.logger.Warn("signal-with-start might have left an orphaned history branch",
				tag.WorkflowDomainID(domainEntry.GetInfo().ID),
				tag.WorkflowID(workflowExecution.WorkflowID),
				tag.WorkflowRunID(workflowExecution.RunID),
				tag.Dynamic("debug-startRequest", startRequest),
				tag.Dynamic("debug-historyBlob", historyBlob),
				tag.Dynamic("debug-domainEntry", domainEntry),
				tag.Dynamic("debug-workflowExecution", workflowExecution),
				tag.Error(err))
			return nil, workflowExecution, historyBlob, err
		}

		if curMutableState.GetCurrentVersion() < t.LastWriteVersion {
			policy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(ctx, domainID, workflowID, t.RunID)
			if err != nil {
				return nil, workflowExecution, historyBlob, err
			}
			if policy.Equals(request.ActiveClusterSelectionPolicy) {
				return nil, workflowExecution, historyBlob, e.newDomainNotActiveError(
					domainEntry,
					t.LastWriteVersion,
				)
			}
		}

		prevRunID = t.RunID
		if shouldTerminateAndStart(startRequest, t.State) {
			runningWFCtx, err := workflow.LoadOnce(ctx, e.executionCache, domainID, workflowID, prevRunID)
			if err != nil {
				return nil, workflowExecution, historyBlob, err
			}
			defer func() { runningWFCtx.GetReleaseFn()(retError) }()

			resp, err := e.terminateAndStartWorkflow(
				ctx,
				runningWFCtx,
				*workflowExecution,
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
				return resp, workflowExecution, historyBlob, err
			}
		}
		if err = e.applyWorkflowIDReusePolicyHelper(
			t.StartRequestID,
			prevRunID,
			t.State,
			t.CloseStatus,
			*workflowExecution,
			startRequest.StartRequest.GetWorkflowIDReusePolicy(),
		); err != nil {
			return nil, workflowExecution, historyBlob, err
		}
		// create as ID reuse
		createMode = persistence.CreateWorkflowModeWorkflowIDReuse
		err = wfContext.CreateWorkflowExecution(
			ctx,
			newWorkflow,
			*historyBlob,
			createMode,
			prevRunID,
			t.LastWriteVersion,
			persistence.CreateWorkflowRequestModeNew,
		)
		if t, ok := persistence.AsDuplicateRequestError(err); ok {
			if t.RequestType == persistence.WorkflowRequestTypeStart || (isSignalWithStart && t.RequestType == persistence.WorkflowRequestTypeSignal) {
				if isChildStart {
					if err := e.validateDuplicateChildRunAdoption(ctx, startRequest, domainEntry, t.RunID); err != nil {
						return nil, workflowExecution, historyBlob, err
					}
				}
				return &types.StartWorkflowExecutionResponse{
					RunID: t.RunID,
				}, workflowExecution, historyBlob, nil
			}
			e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
			return nil, workflowExecution, historyBlob, t
		}
	}
	if err != nil {
		return nil, workflowExecution, historyBlob, err
	}

	return &types.StartWorkflowExecutionResponse{
		RunID: workflowExecution.RunID,
	}, workflowExecution, historyBlob, nil
}

func (e *historyEngineImpl) handleCreateWorkflowExecutionFailureCleanup(
	ctx context.Context,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	domainEntry *cache.DomainCacheEntry,
	workflowExecution *types.WorkflowExecution,
	historyBlob *events.PersistedBlob,
	isSignalWithStart bool,
	err error,
) {

	if workflowExecution == nil || historyBlob == nil {
		// expected behaviour, errors caused by validation will not have a workflow execution
		return
	}

	// we cannot know if the workflow has succeeded, so the safest thing to do is
	// to do nothing. Deleting the history risks breaking a valid execution and prevrenting it from being restarted
	if persistence.IsTimeoutError(err) {
		e.logger.Warn("timeout error detected when creating execution. If the excution was not created successfully this has probably left some orphaned history branch that needs cleaning up",
			tag.WorkflowDomainID(domainEntry.GetInfo().ID),
			tag.WorkflowID(workflowExecution.WorkflowID),
			tag.WorkflowRunID(workflowExecution.RunID),
			tag.Error(err))
		e.metricsClient.IncCounter(metrics.HistoryEngineScope, metrics.WorkflowCreationFailedCleanupHaltedTimeoutCount)
		return
	}

	// this is the set of errors for which we condider safe to history branch created
	// earlier when starting the workflow. It does not include any
	// errors where the state of the workflow is not known (like timeouts, DB internal errors)
	isKnownFailureRequiringCleanup := errors.As(err, new(*types.WorkflowExecutionAlreadyStartedError)) ||
		errors.As(err, new(*types.ShardOwnershipLostError)) ||
		errors.As(err, new(*persistence.CurrentWorkflowConditionFailedError)) ||
		errors.As(err, new(*persistence.ConditionFailedError)) ||
		errors.As(err, new(*persistence.ShardAlreadyExistError)) ||
		errors.As(err, new(*persistence.WorkflowExecutionAlreadyStartedError)) ||
		errors.As(err, new(*types.ShardOwnershipLostError)) ||
		errors.As(err, new(*types.WorkflowExecutionAlreadyStartedError)) ||
		errors.As(err, new(*types.WorkflowExecutionAlreadyCompletedError)) ||
		errors.As(err, new(*persistence.ShardOwnershipLostError)) ||
		errors.As(err, new(*persistence.DuplicateRequestError))

	if !isKnownFailureRequiringCleanup {
		e.logger.Warn("unknown failure detected when creating execution. If the excution was not created successfully this has probably left some orphaned history branch that needs cleaning up",
			tag.WorkflowDomainID(domainEntry.GetInfo().ID),
			tag.WorkflowID(workflowExecution.WorkflowID),
			tag.WorkflowRunID(workflowExecution.RunID),
			tag.Error(err))
		e.metricsClient.IncCounter(metrics.HistoryEngineScope, metrics.WorkflowCreationFailedCleanupUnknownCount)
		return
	}

	if !e.shard.GetConfig().EnableCleanupOrphanedHistoryBranchOnWorkflowCreation(domainEntry.GetInfo().Name) {
		e.logger.Warn("cleanup of orphaned history branch is disabled, but possible orphaned history branch was detected",
			tag.WorkflowDomainID(domainEntry.GetInfo().ID),
			tag.WorkflowID(workflowExecution.WorkflowID),
			tag.WorkflowRunID(workflowExecution.RunID),
			tag.Dynamic("debug-isSignalWithStart", isSignalWithStart),
			tag.Error(err),
		)
		return
	}

	if startRequest == nil || domainEntry == nil || workflowExecution.WorkflowID == "" || workflowExecution.RunID == "" {
		e.logger.Error("some parameters are missing in handleCreateWorkflowExecutionFailureCleanup. This is a bug",
			tag.WorkflowDomainID(domainEntry.GetInfo().ID),
			tag.WorkflowID(workflowExecution.WorkflowID),
			tag.WorkflowRunID(workflowExecution.RunID),
			tag.Dynamic("debug-startRequest", startRequest),
			tag.Dynamic("debug-historyBlob", historyBlob),
			tag.Dynamic("debug-domainEntry", domainEntry),
			tag.Dynamic("debug-workflowExecution", workflowExecution),
			tag.Error(err))
		return
	}

	// The key here is to delete the additational branch, since it has just been created earlier in this call
	// and is not used. However, we must be careful, there may be other existing branches that we must not touch
	e.logger.Debug("Deleting orphaned history branch during cleanup after identified failure during creation",
		tag.WorkflowDomainID(domainEntry.GetInfo().ID),
		tag.WorkflowID(workflowExecution.WorkflowID),
		tag.WorkflowRunID(workflowExecution.RunID),
		tag.Dynamic("debug-isSignalWithStart", isSignalWithStart),
		tag.Error(err))

	cleanupErr := e.shard.GetHistoryManager().DeleteHistoryBranch(ctx, &persistence.DeleteHistoryBranchRequest{
		BranchToken: historyBlob.BranchToken,
		ShardID:     common.IntPtr(e.shard.GetShardID()),
		DomainName:  domainEntry.GetInfo().Name,
	})
	if cleanupErr != nil {
		e.logger.Warn("Failed to cleanup orphaned history branch",
			tag.WorkflowDomainID(domainEntry.GetInfo().ID),
			tag.WorkflowID(workflowExecution.WorkflowID),
			tag.WorkflowRunID(workflowExecution.RunID),
			tag.Error(cleanupErr))
		e.metricsClient.IncCounter(metrics.HistoryEngineScope, metrics.WorkflowCreationFailedCleanupFailureCount)
		return
	}
	e.metricsClient.IncCounter(metrics.HistoryEngineScope, metrics.WorkflowCreationFailedCleanupSuccessCount)
	if e.shard.GetConfig().EnableRecordWorkflowExecutionUninitialized(domainEntry.GetInfo().Name) && e.visibilityMgr != nil {
		// delete the uninitialized workflow execution record since it failed to start the workflow
		// uninitialized record is used to find wfs that didn't make a progress or stuck during the start process
		if errVisibility := e.visibilityMgr.DeleteWorkflowExecution(ctx, &persistence.VisibilityDeleteWorkflowExecutionRequest{
			DomainID:   domainEntry.GetInfo().ID,
			Domain:     domainEntry.GetInfo().Name,
			RunID:      workflowExecution.RunID,
			WorkflowID: workflowExecution.WorkflowID,
		}); errVisibility != nil {
			e.logger.Warn("Failed to delete uninitialized workflow execution record after failed CreateWorkflowExecution",
				tag.WorkflowDomainID(domainEntry.GetInfo().ID),
				tag.WorkflowID(workflowExecution.WorkflowID),
				tag.WorkflowRunID(workflowExecution.RunID),
				tag.Error(errVisibility))
		}
	}
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
			// Do not schedule if the workflow hasn't processed its first decision yet
			// (e.g. waiting for DelayStart or Cron timer)
			if !mutableState.HasPendingDecision() && mutableState.HasProcessedOrPendingDecision() {
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
	resp, createdWFExecution, historyBlob, err := e.startWorkflowHelper(
		ctx,
		startRequest,
		domainEntry,
		metrics.HistorySignalWithStartWorkflowExecutionScope,
		sigWithStartArg,
	)
	if err != nil {
		e.handleCreateWorkflowExecutionFailureCleanup(ctx,
			startRequest,
			domainEntry,
			createdWFExecution,
			historyBlob,
			true,
			err,
		)
		return nil, err
	}
	return resp, nil
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

// childChainEndUnknownError returns the error of a child start that can not determine which cluster closed the last
// run of the chain of the duplicate run, so the run can not be adopted.
func (e *historyEngineImpl) childChainEndUnknownError(
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	duplicateRunID string,
	currentRunID string,
) error {
	e.logger.Info("Child start can not determine which cluster closed the chain of the run with the same request ID",
		tag.WorkflowDomainID(startRequest.DomainUUID),
		tag.WorkflowID(startRequest.StartRequest.GetWorkflowID()),
		tag.WorkflowRunID(duplicateRunID),
		tag.WorkflowRequestID(startRequest.StartRequest.GetRequestID()))
	return &types.WorkflowExecutionAlreadyStartedError{
		Message:        childChainEndUnknownMsg,
		StartRequestID: startRequest.StartRequest.GetRequestID(),
		RunID:          currentRunID,
	}
}

// validateLastRunOfChain returns an error when the last run of the chain of the duplicate run, which sends the
// completion to the parent, can not be adopted.
func (e *historyEngineImpl) validateLastRunOfChain(startRequest *types.HistoryStartWorkflowExecutionRequest, run childRunInfo) error {
	switch {
	case persistence.IsWorkflowRunning(run.state):
		// the run is still open and sends the completion to the parent when it closes, so it can be adopted
		return nil
	case run.state == persistence.WorkflowStateCorrupted:
		// a corrupted run does not progress, but it is the run of this request, so it is adopted and linked to the parent
		return nil
	case run.state == persistence.WorkflowStateZombie:
		// a zombie run does not run on this cluster, so it does not send the completion and can not be adopted
		e.logger.Info("Child start found a zombie run with the same request ID",
			tag.WorkflowDomainID(startRequest.DomainUUID),
			tag.WorkflowID(startRequest.StartRequest.GetWorkflowID()),
			tag.WorkflowRunID(run.runID),
			tag.WorkflowRequestID(startRequest.StartRequest.GetRequestID()))
		return &types.WorkflowExecutionAlreadyStartedError{
			Message:        childRunZombieMsg,
			StartRequestID: startRequest.StartRequest.GetRequestID(),
			RunID:          run.runID,
		}
	case run.state == persistence.WorkflowStateCompleted:
		closingCluster, err := e.clusterMetadata.ClusterNameForFailoverVersion(run.lastWriteVersion)
		if err != nil {
			return err
		}
		if closingCluster == e.currentClusterName {
			// the close transfer task of the run retries the completion until the parent records the start
			return nil
		}
		// only the cluster that closed the run sends the completion to the parent, and after a failover it does not
		// send it, so the run can not be adopted. The start fails, because this request already started the chain of the
		// run, and the reuse policy only applies to a new request.
		e.logger.Info("Child start found a run with the same request ID closed by another cluster",
			tag.WorkflowDomainID(startRequest.DomainUUID),
			tag.WorkflowID(startRequest.StartRequest.GetWorkflowID()),
			tag.WorkflowRunID(run.runID),
			tag.WorkflowRequestID(startRequest.StartRequest.GetRequestID()),
			tag.ClusterName(closingCluster),
			tag.FailoverVersion(run.lastWriteVersion))
		return &types.WorkflowExecutionAlreadyStartedError{
			Message:        childRunClosedByOtherClusterMsg,
			StartRequestID: startRequest.StartRequest.GetRequestID(),
			RunID:          run.runID,
		}
	default:
		return &types.InternalServiceError{Message: fmt.Sprintf("Child start found a run with the same request ID in an invalid state: %v.", run.state)}
	}
}

// validateDuplicateChildRunAdoption returns an error when the parent would not receive the completion of the run of a
// duplicate child start request, so the run can not be adopted as it is.
func (e *historyEngineImpl) validateDuplicateChildRunAdoption(
	ctx context.Context,
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	domainEntry *cache.DomainCacheEntry,
	duplicateRunID string,
) error {
	domainID := domainEntry.GetInfo().ID
	workflowID := startRequest.StartRequest.GetWorkflowID()
	requestID := startRequest.StartRequest.GetRequestID()

	var notExistsErr *types.EntityNotExistsError
	duplicate, err := e.loadChildRunInfo(ctx, domainID, workflowID, duplicateRunID)
	if errors.As(err, &notExistsErr) {
		// a run that no longer exists, for example because an operator deleted it or the domain retention is 0 days,
		// can not send its completion to the parent, so it can not be adopted. The request row still blocks a new run
		// with the same request ID, so the start fails.
		e.logger.Info("Child start found a run with the same request ID that no longer exists",
			tag.WorkflowDomainID(domainID),
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(duplicateRunID),
			tag.WorkflowRequestID(requestID))
		return &types.WorkflowExecutionAlreadyStartedError{
			Message:        childRunNotFoundMsg,
			StartRequestID: requestID,
			RunID:          duplicateRunID,
		}
	}
	if err != nil {
		return err
	}
	// the duplicate run did not continue as new, so it is the last run of its chain
	if duplicate.closeStatus != persistence.WorkflowCloseStatusContinuedAsNew {
		return e.validateLastRunOfChain(startRequest, duplicate)
	}

	// a run that continued as new does not send the completion, the last run of its chain does
	current, err := e.executionManager.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: workflowID,
		DomainName: domainEntry.GetInfo().Name,
		ShardID:    common.Ptr(e.shard.GetShardID()),
	})
	if errors.As(err, &notExistsErr) {
		return e.childChainEndUnknownError(startRequest, duplicateRunID, duplicateRunID)
	}
	if err != nil {
		return err
	}
	currentRun, err := e.loadChildRunInfo(ctx, domainID, workflowID, current.RunID)
	if errors.As(err, &notExistsErr) {
		// without the current run, it is unknown whether the current run is in the chain of the duplicate run
		return e.childChainEndUnknownError(startRequest, duplicateRunID, current.RunID)
	}
	if err != nil {
		return err
	}
	switch {
	case duplicate.firstRunID != "" && currentRun.firstRunID == duplicate.firstRunID:
		// the current run is in the chain of the duplicate run, so it is the last run of the chain
		return e.validateLastRunOfChain(startRequest, currentRun)
	case !domainEntry.GetReplicationConfig().IsActiveActive() && duplicate.lastWriteVersion == currentRun.lastWriteVersion:
		// in an active-passive domain, when the current run has the same version as the duplicate run, the chain of the
		// duplicate run completed in the same cluster as the current run
		return e.validateLastRunOfChain(startRequest, duplicate)
	default:
		return e.childChainEndUnknownError(startRequest, duplicateRunID, current.RunID)
	}
}

// childRunInfo is the info of a run of a child workflow that validateDuplicateChildRunAdoption checks.
type childRunInfo struct {
	runID            string
	firstRunID       string
	state            int
	closeStatus      int
	lastWriteVersion int64
}

// loadChildRunInfo loads a run of a child workflow and releases it immediately, so that
// validateDuplicateChildRunAdoption never holds the locks of two runs at the same time.
func (e *historyEngineImpl) loadChildRunInfo(
	ctx context.Context,
	domainID string,
	workflowID string,
	runID string,
) (_ childRunInfo, retError error) {
	wfCtx, err := workflow.LoadOnce(ctx, e.executionCache, domainID, workflowID, runID)
	if err != nil {
		return childRunInfo{}, err
	}
	defer func() { wfCtx.GetReleaseFn()(retError) }()

	mutableState := wfCtx.GetMutableState()
	executionInfo := mutableState.GetExecutionInfo()
	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return childRunInfo{}, err
	}
	return childRunInfo{
		runID:            runID,
		firstRunID:       executionInfo.FirstExecutionRunID,
		state:            executionInfo.State,
		closeStatus:      executionInfo.CloseStatus,
		lastWriteVersion: lastWriteVersion,
	}, nil
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
