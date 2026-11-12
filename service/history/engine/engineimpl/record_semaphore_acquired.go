package engineimpl

import (
	"context"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

// RecordSemaphoreAcquired records the token Matching granted to a queued acquire.
// Matching's planned handling is in brackets.
//
// Success [Matching keeps the token]:
//  1. Hold waiting for a token: records the grant and schedules a decision.
//  2. Hold already has this token: writes nothing.
//
// Refused [Matching frees the token]:
//  1. Domain not active in this cluster: DomainNotActiveError.
//  2. Run closed: WorkflowExecutionAlreadyCompletedError.
//  3. Hold not found, after one reload if the cached state may be behind: EntityNotExistsError.
//  4. Hold for a different semaphore: EntityNotExistsError.
//  5. Hold already has a different token: EventAlreadyStartedError.
//
// Any other error, for example throttling [Matching retries].
func (e *historyEngineImpl) RecordSemaphoreAcquired(
	ctx context.Context,
	request *types.RecordSemaphoreAcquiredRequest,
) (*types.RecordSemaphoreAcquiredResponse, error) {

	domainEntry, err := e.getActiveDomainByWorkflow(ctx, request.DomainUUID, request.WorkflowExecution.GetWorkflowID(), request.WorkflowExecution.GetRunID())
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	workflowExecution := types.WorkflowExecution{
		WorkflowID: request.WorkflowExecution.GetWorkflowID(),
		RunID:      request.WorkflowExecution.GetRunID(),
	}

	reloaded := false
	err = workflow.UpdateWithActionFunc(
		ctx,
		e.logger,
		e.executionCache,
		domainID,
		workflowExecution,
		e.timeSource.Now(),
		func(_ execution.Context, mutableState execution.MutableState) (*workflow.UpdateAction, error) {
			if !mutableState.IsWorkflowExecutionRunning() {
				return nil, workflow.ErrAlreadyCompleted
			}

			initiatedID := request.GetInitiatedID()
			tokenID := request.GetTokenID()
			executionInfo := mutableState.GetExecutionInfo()
			semaphoreInfo, holdFound := mutableState.GetSemaphoreInfo(initiatedID)

			// Reload at most once per request.
			if !holdFound && !reloaded {
				// An initiated id at or past the next event id is an event this state hasn't seen, so
				// the cached state may be behind the database.
				if initiatedID >= mutableState.GetNextEventID() {
					// Reload rather than refuse a grant Matching would then free. If the reloaded state
					// still lacks the hold, the acquire isn't on the current branch, and it's refused below.
					e.metricsClient.IncCounter(metrics.HistoryRecordSemaphoreAcquiredScope, metrics.StaleMutableStateCounter)
					e.logger.Error("Encounter stale mutable state in RecordSemaphoreAcquired",
						tag.WorkflowDomainID(executionInfo.DomainID),
						tag.WorkflowID(executionInfo.WorkflowID),
						tag.WorkflowRunID(executionInfo.RunID),
						tag.WorkflowInitiatedID(initiatedID),
						tag.WorkflowNextEventID(mutableState.GetNextEventID()),
					)
					reloaded = true
					return nil, workflow.ErrStaleState
				}
			}

			if !holdFound {
				return nil, &types.EntityNotExistsError{Message: "Pending semaphore hold not found."}
			}
			if semaphoreInfo.SemaphoreName != request.GetSemaphoreName() {
				// After a failover, the same event id on another branch can be a hold on another semaphore.
				return nil, &types.EntityNotExistsError{Message: "Pending semaphore hold is for a different semaphore."}
			}
			if semaphoreInfo.HasToken() {
				if semaphoreInfo.TokenID == int(tokenID) {
					// The hold already has this token: a repeated grant, or after a failover and failback, this
					// region's grant has the same id as the token the other region granted. Either way there is
					// nothing to record, and Matching keeps the token.
					return &workflow.UpdateAction{Noop: true}, nil
				}
				// The hold has a token from another region. Tokens are per region but the owner id is not,
				// so after a failover and failback the hold can keep the token the other region granted,
				// while this region's still-queued waiter for the same owner is granted a second one.
				// Within one region Matching gives an owner at most one token.
				//
				// Refuse, so Matching frees the new token. Recording it would replace the hold's token,
				// which only a re-acquire does, after clearing it first. The run heals through the normal
				// failover path: the next decision's ownership check finds the other region's token
				// invalid here and re-acquires, and reconciliation frees that token in
				// the other region.
				e.logger.Info("Semaphore hold already has a different token than the one granted",
					tag.WorkflowDomainID(executionInfo.DomainID),
					tag.WorkflowID(executionInfo.WorkflowID),
					tag.WorkflowRunID(executionInfo.RunID),
					tag.WorkflowInitiatedID(initiatedID),
					tag.SemaphoreName(semaphoreInfo.SemaphoreName),
					tag.SemaphoreTokenID(semaphoreInfo.TokenID),
					tag.SemaphoreGrantedTokenID(int(tokenID)),
				)
				return nil, &types.EventAlreadyStartedError{Message: "Semaphore hold already has a different token."}
			}

			if _, err := mutableState.AddSemaphoreAcquiredEvent(initiatedID, tokenID); err != nil {
				return nil, err
			}
			return workflow.UpdateWithNewDecision, nil
		})
	if err != nil {
		return nil, err
	}
	return &types.RecordSemaphoreAcquiredResponse{}, nil
}
