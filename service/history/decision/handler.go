// Copyright (c) 2017 Uber Technologies, Inc.
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

package decision

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/workflow"
)

type (
	// Handler contains decision business logic
	Handler interface {
		HandleDecisionTaskScheduled(context.Context, *types.ScheduleDecisionTaskRequest) error
		HandleDecisionTaskStarted(context.Context,
			*types.RecordDecisionTaskStartedRequest) (*types.RecordDecisionTaskStartedResponse, error)
		HandleDecisionTaskFailed(context.Context,
			*types.HistoryRespondDecisionTaskFailedRequest) error
		HandleDecisionTaskCompleted(context.Context,
			*types.HistoryRespondDecisionTaskCompletedRequest) (*types.HistoryRespondDecisionTaskCompletedResponse, error)
		// TODO also include the handle of decision timeout here
	}

	handlerImpl struct {
		config          *config.Config
		shard           shard.Context
		timeSource      clock.TimeSource
		domainCache     cache.DomainCache
		executionCache  execution.Cache
		tokenSerializer common.TaskTokenSerializer
		metricsClient   metrics.Client
		logger          log.Logger
		throttledLogger log.Logger
		attrValidator   *attrValidator
		versionChecker  client.VersionChecker
	}
)

// NewHandler creates a new Handler for handling decision business logic
func NewHandler(
	shard shard.Context,
	executionCache execution.Cache,
	tokenSerializer common.TaskTokenSerializer,
) Handler {
	config := shard.GetConfig()
	logger := shard.GetLogger().WithTags(tag.ComponentDecisionHandler)
	return &handlerImpl{
		config:          config,
		shard:           shard,
		timeSource:      shard.GetTimeSource(),
		domainCache:     shard.GetDomainCache(),
		executionCache:  executionCache,
		tokenSerializer: tokenSerializer,
		metricsClient:   shard.GetMetricsClient(),
		logger:          shard.GetLogger().WithTags(tag.ComponentDecisionHandler),
		throttledLogger: shard.GetThrottledLogger().WithTags(tag.ComponentDecisionHandler),
		attrValidator: newAttrValidator(
			shard.GetDomainCache(),
			shard.GetMetricsClient(),
			config,
			logger,
		),
		versionChecker: client.NewVersionChecker(),
	}
}

func (handler *handlerImpl) HandleDecisionTaskScheduled(
	ctx context.Context,
	req *types.ScheduleDecisionTaskRequest,
) error {

	domainEntry, err := handler.getActiveDomainByID(req.DomainUUID)
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	workflowExecution := types.WorkflowExecution{
		WorkflowID: req.WorkflowExecution.WorkflowID,
		RunID:      req.WorkflowExecution.RunID,
	}

	return workflow.UpdateWithActionFunc(
		ctx,
		handler.executionCache,
		domainID,
		workflowExecution,
		handler.timeSource.Now(),
		func(context execution.Context, mutableState execution.MutableState) (*workflow.UpdateAction, error) {
			if !mutableState.IsWorkflowExecutionRunning() {
				return nil, workflow.ErrNotExists
			}

			if mutableState.HasProcessedOrPendingDecision() {
				return &workflow.UpdateAction{
					Noop: true,
				}, nil
			}

			startEvent, err := mutableState.GetStartEvent(ctx)
			if err != nil {
				return nil, err
			}
			if err := mutableState.AddFirstDecisionTaskScheduled(
				startEvent,
			); err != nil {
				return nil, err
			}

			return &workflow.UpdateAction{}, nil
		},
	)
}

func (handler *handlerImpl) HandleDecisionTaskStarted(
	ctx context.Context,
	req *types.RecordDecisionTaskStartedRequest,
) (*types.RecordDecisionTaskStartedResponse, error) {

	domainEntry, err := handler.getActiveDomainByID(req.DomainUUID)
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	workflowExecution := types.WorkflowExecution{
		WorkflowID: req.WorkflowExecution.WorkflowID,
		RunID:      req.WorkflowExecution.RunID,
	}

	scheduleID := req.GetScheduleID()
	requestID := req.GetRequestID()

	var resp *types.RecordDecisionTaskStartedResponse
	err = workflow.UpdateWithActionFunc(
		ctx,
		handler.executionCache,
		domainID,
		workflowExecution,
		handler.timeSource.Now(),
		func(context execution.Context, mutableState execution.MutableState) (*workflow.UpdateAction, error) {
			if !mutableState.IsWorkflowExecutionRunning() {
				return nil, workflow.ErrNotExists
			}

			decision, isRunning := mutableState.GetDecisionInfo(scheduleID)

			// First check to see if cache needs to be refreshed as we could potentially have stale workflow execution in
			// some extreme cassandra failure cases.
			if !isRunning && scheduleID >= mutableState.GetNextEventID() {
				handler.metricsClient.IncCounter(metrics.HistoryRecordDecisionTaskStartedScope, metrics.StaleMutableStateCounter)
				handler.logger.Error("Encounter stale mutable state in RecordDecisionTaskStarted",
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowScheduleID(scheduleID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
				)
				// Reload workflow execution history
				// ErrStaleState will trigger updateWorkflowExecutionWithAction function to reload the mutable state
				return nil, workflow.ErrStaleState
			}

			// Check execution state to make sure task is in the list of outstanding tasks and it is not yet started.  If
			// task is not outstanding than it is most probably a duplicate and complete the task.
			if !isRunning {
				// Looks like DecisionTask already completed as a result of another call.
				// It is OK to drop the task at this point.
				return nil, &types.EntityNotExistsError{Message: "Decision task not found."}
			}

			updateAction := &workflow.UpdateAction{}

			if decision.StartedID != common.EmptyEventID {
				// If decision is started as part of the current request scope then return a positive response
				if decision.RequestID == requestID {
					resp, err = handler.createRecordDecisionTaskStartedResponse(domainID, mutableState, decision, req.PollRequest.GetIdentity())
					if err != nil {
						return nil, err
					}
					updateAction.Noop = true
					return updateAction, nil
				}

				// Looks like DecisionTask already started as a result of another call.
				// It is OK to drop the task at this point.
				return nil, &types.EventAlreadyStartedError{Message: "Decision task already started."}
			}

			_, decision, err = mutableState.AddDecisionTaskStartedEvent(scheduleID, requestID, req.PollRequest)
			if err != nil {
				// Unable to add DecisionTaskStarted event to history
				return nil, &types.InternalServiceError{Message: "Unable to add DecisionTaskStarted event to history."}
			}

			resp, err = handler.createRecordDecisionTaskStartedResponse(domainID, mutableState, decision, req.PollRequest.GetIdentity())
			if err != nil {
				return nil, err
			}
			return updateAction, nil
		},
	)

	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (handler *handlerImpl) HandleDecisionTaskFailed(
	ctx context.Context,
	req *types.HistoryRespondDecisionTaskFailedRequest,
) (retError error) {

	domainEntry, err := handler.getActiveDomainByID(req.DomainUUID)
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	request := req.FailedRequest
	token, err := handler.tokenSerializer.Deserialize(request.TaskToken)
	if err != nil {
		return workflow.ErrDeserializingToken
	}

	workflowExecution := types.WorkflowExecution{
		WorkflowID: token.WorkflowID,
		RunID:      token.RunID,
	}

	return workflow.UpdateWithAction(ctx, handler.executionCache, domainID, workflowExecution, true, handler.timeSource.Now(),
		func(context execution.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return workflow.ErrAlreadyCompleted
			}

			scheduleID := token.ScheduleID
			decision, isRunning := mutableState.GetDecisionInfo(scheduleID)
			if !isRunning || decision.Attempt != token.ScheduleAttempt || decision.StartedID == common.EmptyEventID {
				return &types.EntityNotExistsError{Message: "Decision task not found."}
			}

			_, err := mutableState.AddDecisionTaskFailedEvent(decision.ScheduleID, decision.StartedID, request.GetCause(), request.Details,
				request.GetIdentity(), "", request.GetBinaryChecksum(), "", "", 0, "")
			return err
		})
}

func (handler *handlerImpl) HandleDecisionTaskCompleted(
	ctx context.Context,
	req *types.HistoryRespondDecisionTaskCompletedRequest,
) (resp *types.HistoryRespondDecisionTaskCompletedResponse, retError error) {
	domainEntry, err := handler.getActiveDomainByID(req.DomainUUID)
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	request := req.CompleteRequest
	token, err0 := handler.tokenSerializer.Deserialize(request.TaskToken)
	if err0 != nil {
		return nil, workflow.ErrDeserializingToken
	}

	workflowExecution := types.WorkflowExecution{
		WorkflowID: token.WorkflowID,
		RunID:      token.RunID,
	}

	domainName := domainEntry.GetInfo().Name
	logger := handler.logger.WithTags(
		tag.WorkflowDomainName(domainName),
		tag.WorkflowDomainID(domainEntry.GetInfo().ID),
		tag.WorkflowID(workflowExecution.GetWorkflowID()),
		tag.WorkflowRunID(workflowExecution.GetRunID()),
		tag.WorkflowScheduleID(token.ScheduleID),
	)
	scope := handler.metricsClient.Scope(metrics.HistoryRespondDecisionTaskCompletedScope,
		metrics.DomainTag(domainName),
		metrics.WorkflowTypeTag(token.WorkflowType))

	call := yarpc.CallFromContext(ctx)
	clientLibVersion := call.Header(common.LibraryVersionHeaderName)
	clientFeatureVersion := call.Header(common.FeatureVersionHeaderName)
	clientImpl := call.Header(common.ClientImplHeaderName)

	wfContext, release, err := handler.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, workflowExecution)
	if err != nil {
		return nil, err
	}
	defer func() { release(retError) }()

Update_History_Loop:
	for attempt := 0; attempt < workflow.ConditionalRetryCount; attempt++ {
		logger.Debug("Update_History_Loop attempt", tag.Attempt(int32(attempt)))
		msBuilder, err := wfContext.LoadWorkflowExecution(ctx)
		if err != nil {
			return nil, err
		}
		if !msBuilder.IsWorkflowExecutionRunning() {
			return nil, workflow.ErrAlreadyCompleted
		}
		executionStats, err := wfContext.LoadExecutionStats(ctx)
		if err != nil {
			return nil, err
		}

		executionInfo := msBuilder.GetExecutionInfo()
		currentDecision, isRunning := msBuilder.GetDecisionInfo(token.ScheduleID)

		// First check to see if cache needs to be refreshed as we could potentially have stale workflow execution in
		// some extreme cassandra failure cases.
		if !isRunning && token.ScheduleID >= msBuilder.GetNextEventID() {
			scope.IncCounter(metrics.StaleMutableStateCounter)
			logger.Error("Encounter stale mutable state in RespondDecisionTaskCompleted", tag.WorkflowNextEventID(msBuilder.GetNextEventID()))
			// Reload workflow execution history
			wfContext.Clear()
			continue Update_History_Loop
		}

		if !msBuilder.IsWorkflowExecutionRunning() || !isRunning || currentDecision.Attempt != token.ScheduleAttempt || currentDecision.StartedID == common.EmptyEventID {
			logger.Debugf("Decision task not found. IsWorkflowExecutionRunning: %v, isRunning: %v, currentDecision.Attempt: %v, token.ScheduleAttempt: %v, currentDecision.StartID: %v",
				msBuilder.IsWorkflowExecutionRunning(), isRunning, getDecisionInfoAttempt(currentDecision), token.ScheduleAttempt, getDecisionInfoStartedID(currentDecision))
			return nil, &types.EntityNotExistsError{Message: "Decision task not found."}
		}

		startedID := currentDecision.StartedID
		maxResetPoints := handler.config.MaxAutoResetPoints(domainEntry.GetInfo().Name)
		if msBuilder.GetExecutionInfo().AutoResetPoints != nil && maxResetPoints == len(msBuilder.GetExecutionInfo().AutoResetPoints.Points) {
			logger.Debugf("Max reset points %d is exceeded", maxResetPoints)
			scope.IncCounter(metrics.AutoResetPointsLimitExceededCounter)
		}

		decisionHeartbeating := request.GetForceCreateNewDecisionTask() && len(request.Decisions) == 0
		var decisionHeartbeatTimeout bool
		var completedEvent *types.HistoryEvent
		if decisionHeartbeating {
			timeout := handler.config.DecisionHeartbeatTimeout(domainName)
			if currentDecision.OriginalScheduledTimestamp > 0 && handler.timeSource.Now().After(time.Unix(0, currentDecision.OriginalScheduledTimestamp).Add(timeout)) {
				decisionHeartbeatTimeout = true
				scope.IncCounter(metrics.DecisionHeartbeatTimeoutCounter)
				completedEvent, err = msBuilder.AddDecisionTaskTimedOutEvent(currentDecision.ScheduleID, currentDecision.StartedID)
				if err != nil {
					return nil, &types.InternalServiceError{Message: "Failed to add decision timeout event."}
				}
				msBuilder.ClearStickyness()
			} else {
				logger.Debug("Adding DecisionTaskCompletedEvent to mutable state for heartbeat")
				completedEvent, err = msBuilder.AddDecisionTaskCompletedEvent(token.ScheduleID, startedID, request, maxResetPoints)
				if err != nil {
					return nil, &types.InternalServiceError{Message: "Unable to add DecisionTaskCompleted event to history."}
				}
			}
		} else {
			completedEvent, err = msBuilder.AddDecisionTaskCompletedEvent(token.ScheduleID, startedID, request, maxResetPoints)
			if err != nil {
				return nil, &types.InternalServiceError{Message: "Unable to add DecisionTaskCompleted event to history."}
			}
		}

		var (
			failDecision                bool
			failCause                   types.DecisionTaskFailedCause
			failMessage                 string
			activityNotStartedCancelled bool
			continueAsNewBuilder        execution.MutableState
			hasUnhandledEvents          bool
			decisionResults             []*decisionResult
		)
		hasUnhandledEvents = msBuilder.HasBufferedEvents()

		if request.StickyAttributes == nil || request.StickyAttributes.WorkerTaskList == nil {
			scope.IncCounter(metrics.CompleteDecisionWithStickyDisabledCounter)
			executionInfo.StickyTaskList = ""
			executionInfo.StickyScheduleToStartTimeout = 0
		} else {
			scope.IncCounter(metrics.CompleteDecisionWithStickyEnabledCounter)
			executionInfo.StickyTaskList = request.StickyAttributes.WorkerTaskList.GetName()
			executionInfo.StickyScheduleToStartTimeout = request.StickyAttributes.GetScheduleToStartTimeoutSeconds()
		}
		executionInfo.ClientLibraryVersion = clientLibVersion
		executionInfo.ClientFeatureVersion = clientFeatureVersion
		executionInfo.ClientImpl = clientImpl

		binChecksum := request.GetBinaryChecksum()
		if _, ok := domainEntry.GetConfig().BadBinaries.Binaries[binChecksum]; ok {
			failDecision = true
			failCause = types.DecisionTaskFailedCauseBadBinary
			failMessage = fmt.Sprintf("binary %v is already marked as bad deployment", binChecksum)
		} else {
			workflowSizeChecker := newWorkflowSizeChecker(
				domainName,
				handler.config.BlobSizeLimitWarn(domainName),
				handler.config.BlobSizeLimitError(domainName),
				handler.config.HistorySizeLimitWarn(domainName),
				handler.config.HistorySizeLimitError(domainName),
				handler.config.HistoryCountLimitWarn(domainName),
				handler.config.HistoryCountLimitError(domainName),
				completedEvent.ID,
				msBuilder,
				executionStats,
				handler.metricsClient.Scope(metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DomainTag(domainName)),
				handler.logger,
			)

			decisionTaskHandler := newDecisionTaskHandler(
				request.GetIdentity(),
				completedEvent.ID,
				domainEntry,
				msBuilder,
				handler.attrValidator,
				workflowSizeChecker,
				handler.tokenSerializer,
				handler.logger,
				handler.domainCache,
				handler.metricsClient,
				handler.config,
			)

			if decisionResults, err = decisionTaskHandler.handleDecisions(
				ctx,
				request.ExecutionContext,
				request.Decisions,
			); err != nil {
				return nil, err
			}

			// set the vars used by following logic
			// further refactor should also clean up the vars used below
			failDecision = decisionTaskHandler.failDecision
			if failDecision {
				failCause = *decisionTaskHandler.failDecisionCause
				failMessage = *decisionTaskHandler.failMessage
			}

			// failMessage is not used by decisionTaskHandler
			activityNotStartedCancelled = decisionTaskHandler.activityNotStartedCancelled
			// continueAsNewTimerTasks is not used by decisionTaskHandler
			continueAsNewBuilder = decisionTaskHandler.continueAsNewBuilder
			hasUnhandledEvents = decisionTaskHandler.hasUnhandledEventsBeforeDecisions
		}

		if failDecision {
			scope.IncCounter(metrics.FailedDecisionsCounter)
			logger.Info("Failing the decision.", tag.WorkflowDecisionFailCause(int64(failCause)))
			msBuilder, err = handler.failDecisionHelper(
				ctx, wfContext, token.ScheduleID, startedID, failCause, []byte(failMessage), request, domainEntry)
			if err != nil {
				return nil, err
			}
			hasUnhandledEvents = true
			continueAsNewBuilder = nil
		}

		createNewDecisionTask := msBuilder.IsWorkflowExecutionRunning() && (hasUnhandledEvents || request.GetForceCreateNewDecisionTask() || activityNotStartedCancelled)
		logger.Debugf("createNewDecisionTask: %v, msBuilder.IsWorkflowExecutionRunning: %v, hasUnhandledEvents: %v, request.GetForceCreateNewDecisionTask: %v, activityNotStartedCancelled: %v",
			createNewDecisionTask, msBuilder.IsWorkflowExecutionRunning(), hasUnhandledEvents, request.GetForceCreateNewDecisionTask(), activityNotStartedCancelled)
		var newDecisionTaskScheduledID int64
		if createNewDecisionTask {
			var newDecision *execution.DecisionInfo
			var err error
			if decisionHeartbeating && !decisionHeartbeatTimeout {
				newDecision, err = msBuilder.AddDecisionTaskScheduledEventAsHeartbeat(
					request.GetReturnNewDecisionTask(),
					currentDecision.OriginalScheduledTimestamp,
				)
			} else {
				newDecision, err = msBuilder.AddDecisionTaskScheduledEvent(
					request.GetReturnNewDecisionTask(),
				)
			}
			if err != nil {
				return nil, &types.InternalServiceError{Message: "Failed to add decision scheduled event."}
			}

			newDecisionTaskScheduledID = newDecision.ScheduleID
			// skip transfer task for decision if request asking to return new decision task
			if request.GetReturnNewDecisionTask() {
				logger.Debugf("Adding DecisionTaskStartedEvent to mutable state. new decision's ScheduleID: %d, TaskList: %s", newDecisionTaskScheduledID, newDecision.TaskList)
				// start the new decision task if request asked to do so
				// TODO: replace the poll request
				_, _, err := msBuilder.AddDecisionTaskStartedEvent(newDecision.ScheduleID, "request-from-RespondDecisionTaskCompleted", &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{Name: newDecision.TaskList},
					Identity: request.Identity,
				})
				if err != nil {
					return nil, err
				}
			}
		}

		// We apply the update to execution using optimistic concurrency.  If it fails due to a conflict then reload
		// the history and try the operation again.
		var updateErr error
		if continueAsNewBuilder != nil {
			continueAsNewExecutionInfo := continueAsNewBuilder.GetExecutionInfo()
			logger.Debugf("Updating execution with continue as new info. new wfid: %s, runid: %s", continueAsNewExecutionInfo.WorkflowID, continueAsNewExecutionInfo.RunID)
			updateErr = wfContext.UpdateWorkflowExecutionWithNewAsActive(
				ctx,
				handler.shard.GetTimeSource().Now(),
				execution.NewContext(
					continueAsNewExecutionInfo.DomainID,
					types.WorkflowExecution{
						WorkflowID: continueAsNewExecutionInfo.WorkflowID,
						RunID:      continueAsNewExecutionInfo.RunID,
					},
					handler.shard,
					handler.shard.GetExecutionManager(),
					handler.logger,
				),
				continueAsNewBuilder,
			)
		} else {
			updateErr = wfContext.UpdateWorkflowExecutionAsActive(ctx, handler.shard.GetTimeSource().Now())
		}

		if updateErr != nil {
			if execution.IsConflictError(updateErr) {
				scope.IncCounter(metrics.ConcurrencyUpdateFailureCounter)
				continue Update_History_Loop
			}

			// if updateErr resulted in TransactionSizeLimitError then fail workflow
			switch updateErr.(type) {
			case *persistence.TransactionSizeLimitError:
				// must reload mutable state because the first call to updateWorkflowExecutionWithContext or continueAsNewWorkflowExecution
				// clears mutable state if error is returned
				msBuilder, err = wfContext.LoadWorkflowExecution(ctx)
				if err != nil {
					return nil, err
				}

				eventBatchFirstEventID := msBuilder.GetNextEventID()
				if err := execution.TerminateWorkflow(
					msBuilder,
					eventBatchFirstEventID,
					common.FailureReasonTransactionSizeExceedsLimit,
					[]byte(updateErr.Error()),
					execution.IdentityHistoryService,
				); err != nil {
					return nil, err
				}
				if err := wfContext.UpdateWorkflowExecutionAsActive(
					ctx,
					handler.shard.GetTimeSource().Now(),
				); err != nil {
					return nil, err
				}
			}

			return nil, updateErr
		}

		handler.handleBufferedQueries(
			msBuilder,
			clientImpl,
			clientFeatureVersion,
			req.GetCompleteRequest().GetQueryResults(),
			createNewDecisionTask,
			domainEntry,
			decisionHeartbeating)

		if decisionHeartbeatTimeout {
			// at this point, update is successful, but we still return an error to client so that the worker will give up this workflow
			return nil, &types.EntityNotExistsError{
				Message: "decision heartbeat timeout",
			}
		}

		resp = &types.HistoryRespondDecisionTaskCompletedResponse{}
		if !msBuilder.IsWorkflowExecutionRunning() {
			// Workflow has been completed/terminated, so there is no need to dispatch more activity/decision tasks.
			return resp, nil
		}

		activitiesToDispatchLocally := make(map[string]*types.ActivityLocalDispatchInfo)
		for _, dr := range decisionResults {
			if dr.activityDispatchInfo != nil {
				activitiesToDispatchLocally[dr.activityDispatchInfo.ActivityID] = dr.activityDispatchInfo
			}
		}
		logger.Debugf("%d activities will be dispatched locally on the client side")
		resp.ActivitiesToDispatchLocally = activitiesToDispatchLocally

		if request.GetReturnNewDecisionTask() && createNewDecisionTask {
			decision, _ := msBuilder.GetDecisionInfo(newDecisionTaskScheduledID)
			resp.StartedResponse, err = handler.createRecordDecisionTaskStartedResponse(domainID, msBuilder, decision, request.GetIdentity())
			if err != nil {
				return nil, err
			}
			// sticky is always enabled when worker request for new decision task from RespondDecisionTaskCompleted
			resp.StartedResponse.StickyExecutionEnabled = true
		}

		return resp, nil
	}

	return nil, workflow.ErrMaxAttemptsExceeded
}

func (handler *handlerImpl) createRecordDecisionTaskStartedResponse(
	domainID string,
	msBuilder execution.MutableState,
	decision *execution.DecisionInfo,
	identity string,
) (*types.RecordDecisionTaskStartedResponse, error) {

	response := &types.RecordDecisionTaskStartedResponse{}
	response.WorkflowType = msBuilder.GetWorkflowType()
	executionInfo := msBuilder.GetExecutionInfo()
	if executionInfo.LastProcessedEvent != common.EmptyEventID {
		response.PreviousStartedEventID = common.Int64Ptr(executionInfo.LastProcessedEvent)
	}

	// Starting decision could result in different scheduleID if decision was transient and new new events came in
	// before it was started.
	response.ScheduledEventID = decision.ScheduleID
	response.StartedEventID = decision.StartedID
	// if we call IsStickyTaskListEnabled then it's possible that the decision is a
	// sticky decision but due to TTL check, the field becomes false
	// NOTE: it's possible that StickyTaskList is empty is even if the decision is scheduled
	// on a sticky tasklist since stickiness can be cleared at anytime by the ResetStickyTaskList API.
	// When this field is false, we will send full workflow history to client
	// (see createPollForDecisionTaskResponse in workflowHandler.go)
	// This is actually desired since if that API is called, it basically means the client side
	// cache has been cleared for the workflow and full history is needed by the client. Even if
	// client side still has the cache, client library is still able to handle the situation.
	response.StickyExecutionEnabled = msBuilder.GetExecutionInfo().StickyTaskList != ""
	response.NextEventID = msBuilder.GetNextEventID()
	response.Attempt = decision.Attempt
	response.WorkflowExecutionTaskList = &types.TaskList{
		Name: executionInfo.TaskList,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	response.ScheduledTimestamp = common.Int64Ptr(decision.ScheduledTimestamp)
	response.StartedTimestamp = common.Int64Ptr(decision.StartedTimestamp)

	if decision.Attempt > 0 {
		// This decision is retried from mutable state
		// Also return schedule and started which are not written to history yet
		scheduledEvent, startedEvent := msBuilder.CreateTransientDecisionEvents(decision, identity)
		response.DecisionInfo = &types.TransientDecisionInfo{}
		response.DecisionInfo.ScheduledEvent = scheduledEvent
		response.DecisionInfo.StartedEvent = startedEvent
	}
	currentBranchToken, err := msBuilder.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	response.BranchToken = currentBranchToken

	qr := msBuilder.GetQueryRegistry()
	buffered := qr.GetBufferedIDs()
	queries := make(map[string]*types.WorkflowQuery)
	for _, id := range buffered {
		input, err := qr.GetQueryInput(id)
		if err != nil {
			continue
		}
		queries[id] = input
	}
	response.Queries = queries
	response.HistorySize = msBuilder.GetHistorySize()
	return response, nil
}

func (handler *handlerImpl) handleBufferedQueries(
	msBuilder execution.MutableState,
	clientImpl string,
	clientFeatureVersion string,
	queryResults map[string]*types.WorkflowQueryResult,
	createNewDecisionTask bool,
	domainEntry *cache.DomainCacheEntry,
	decisionHeartbeating bool,
) {
	queryRegistry := msBuilder.GetQueryRegistry()
	if !queryRegistry.HasBufferedQuery() {
		return
	}

	domainID := domainEntry.GetInfo().ID
	domain := domainEntry.GetInfo().Name
	workflowID := msBuilder.GetExecutionInfo().WorkflowID
	runID := msBuilder.GetExecutionInfo().RunID

	scope := handler.metricsClient.Scope(
		metrics.HistoryRespondDecisionTaskCompletedScope,
		metrics.DomainTag(domainEntry.GetInfo().Name),
		metrics.DecisionTypeTag("ConsistentQuery"))

	// Consistent query requires both server and client worker support. If a consistent query was requested (meaning there are
	// buffered queries) but worker does not support consistent query then all buffered queries should be failed.
	versionErr := handler.versionChecker.SupportsConsistentQuery(clientImpl, clientFeatureVersion)
	// todo (David.Porter) remove the skip on version check for
	// clientImpl and clientFeatureVersion where they're nil
	// There's a bug, probably in matching somewhere which isn't
	// forwarding the client headers for version
	// info correctly making this call erroneously fail sometimes.
	// https://t3.uberinternal.com/browse/CDNC-8641
	// So defaulting just this flow to fail-open in the absence of headers.
	if versionErr != nil && clientImpl != "" && clientFeatureVersion != "" {
		scope.IncCounter(metrics.WorkerNotSupportsConsistentQueryCount)
		failedTerminationState := &query.TerminationState{
			TerminationType: query.TerminationTypeFailed,
			Failure:         &types.BadRequestError{Message: versionErr.Error()},
		}
		buffered := queryRegistry.GetBufferedIDs()
		handler.logger.Info(
			"failing query because worker does not support consistent query",
			tag.ClientImpl(clientImpl),
			tag.ClientFeatureVersion(clientFeatureVersion),
			tag.WorkflowDomainName(domain),
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(runID),
			tag.Error(versionErr))
		for _, id := range buffered {
			if err := queryRegistry.SetTerminationState(id, failedTerminationState); err != nil {
				handler.logger.Error(
					"failed to set query termination state to failed",
					tag.WorkflowDomainName(domain),
					tag.WorkflowID(workflowID),
					tag.WorkflowRunID(runID),
					tag.QueryID(id),
					tag.Error(err))
				scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			}
		}
		return
	}

	// if its a heartbeat decision it means local activities may still be running on the worker
	// which were started by an external event which happened before the query
	if decisionHeartbeating {
		return
	}

	sizeLimitError := handler.config.BlobSizeLimitError(domain)
	sizeLimitWarn := handler.config.BlobSizeLimitWarn(domain)

	// Complete or fail all queries we have results for
	for id, result := range queryResults {
		if err := common.CheckEventBlobSizeLimit(
			len(result.GetAnswer()),
			sizeLimitWarn,
			sizeLimitError,
			domainID,
			domain,
			workflowID,
			runID,
			scope,
			handler.logger,
			tag.BlobSizeViolationOperation("ConsistentQuery"),
		); err != nil {
			handler.logger.Info("failing query because query result size is too large",
				tag.WorkflowDomainName(domain),
				tag.WorkflowID(workflowID),
				tag.WorkflowRunID(runID),
				tag.QueryID(id),
				tag.Error(err))
			failedTerminationState := &query.TerminationState{
				TerminationType: query.TerminationTypeFailed,
				Failure:         err,
			}
			if err := queryRegistry.SetTerminationState(id, failedTerminationState); err != nil {
				handler.logger.Error(
					"failed to set query termination state to failed",
					tag.WorkflowDomainName(domain),
					tag.WorkflowID(workflowID),
					tag.WorkflowRunID(runID),
					tag.QueryID(id),
					tag.Error(err))
				scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			}
		} else {
			completedTerminationState := &query.TerminationState{
				TerminationType: query.TerminationTypeCompleted,
				QueryResult:     result,
			}
			if err := queryRegistry.SetTerminationState(id, completedTerminationState); err != nil {
				handler.logger.Error(
					"failed to set query termination state to completed",
					tag.WorkflowDomainName(domain),
					tag.WorkflowID(workflowID),
					tag.WorkflowRunID(runID),
					tag.QueryID(id),
					tag.Error(err))
				scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			}
		}
	}

	// If no decision task was created then it means no buffered events came in during this decision task's handling.
	// This means all unanswered buffered queries can be dispatched directly through matching at this point.
	if !createNewDecisionTask {
		buffered := queryRegistry.GetBufferedIDs()
		for _, id := range buffered {
			unblockTerminationState := &query.TerminationState{
				TerminationType: query.TerminationTypeUnblocked,
			}
			if err := queryRegistry.SetTerminationState(id, unblockTerminationState); err != nil {
				handler.logger.Error(
					"failed to set query termination state to unblocked",
					tag.WorkflowDomainName(domain),
					tag.WorkflowID(workflowID),
					tag.WorkflowRunID(runID),
					tag.QueryID(id),
					tag.Error(err))
				scope.IncCounter(metrics.QueryRegistryInvalidStateCount)
			}
		}
	}
}

func (handler *handlerImpl) failDecisionHelper(
	ctx context.Context,
	wfContext execution.Context,
	scheduleID int64,
	startedID int64,
	cause types.DecisionTaskFailedCause,
	details []byte,
	request *types.RespondDecisionTaskCompletedRequest,
	domainEntry *cache.DomainCacheEntry,
) (execution.MutableState, error) {

	// Clear any updates we have accumulated so far
	wfContext.Clear()

	// Reload workflow execution so we can apply the decision task failure event
	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return nil, err
	}

	if _, err = mutableState.AddDecisionTaskFailedEvent(
		scheduleID, startedID, cause, details, request.GetIdentity(), "", request.GetBinaryChecksum(), "", "", 0, "",
	); err != nil {
		return nil, err
	}

	domainName := domainEntry.GetInfo().Name
	maxAttempts := handler.config.DecisionRetryMaxAttempts(domainName)
	if maxAttempts > 0 && mutableState.GetExecutionInfo().DecisionAttempt > int64(maxAttempts) {
		message := fmt.Sprintf(
			"Decision attempt exceeds limit. Last decision failure cause and details: %v - %v",
			cause,
			details)
		executionInfo := mutableState.GetExecutionInfo()
		handler.logger.Error(message,
			tag.WorkflowDomainID(executionInfo.DomainID),
			tag.WorkflowID(executionInfo.WorkflowID),
			tag.WorkflowRunID(executionInfo.RunID))
		handler.metricsClient.IncCounter(metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DecisionRetriesExceededCounter)

		if _, err := mutableState.AddWorkflowExecutionTerminatedEvent(
			mutableState.GetNextEventID(),
			common.FailureReasonDecisionAttemptsExceedsLimit,
			[]byte(message),
			execution.IdentityHistoryService,
		); err != nil {
			return nil, err
		}
	}

	// Return new builder back to the caller for further updates
	return mutableState, nil
}

func (handler *handlerImpl) getActiveDomainByID(id string) (*cache.DomainCacheEntry, error) {
	return cache.GetActiveDomainByID(handler.shard.GetDomainCache(), handler.shard.GetClusterMetadata().GetCurrentClusterName(), id)
}

func getDecisionInfoAttempt(di *execution.DecisionInfo) int64 {
	if di == nil {
		return 0
	}
	return di.Attempt
}

func getDecisionInfoStartedID(di *execution.DecisionInfo) int64 {
	if di == nil {
		return 0
	}
	return di.StartedID
}
