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

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

// RecordChildExecutionCompleted records the completion of child execution into parent execution history
func (e *historyEngineImpl) RecordChildExecutionCompleted(
	ctx context.Context,
	completionRequest *types.RecordChildExecutionCompletedRequest,
) error {

	domainEntry, err := e.getActiveDomainByWorkflow(ctx, completionRequest.DomainUUID, completionRequest.WorkflowExecution.GetWorkflowID(), completionRequest.WorkflowExecution.GetRunID())
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	workflowExecution := types.WorkflowExecution{
		WorkflowID: completionRequest.WorkflowExecution.GetWorkflowID(),
		RunID:      completionRequest.WorkflowExecution.GetRunID(),
	}

	return e.updateWithActionFn(ctx, e.logger, e.executionCache, domainID, workflowExecution, true, e.timeSource.Now(),
		func(wfContext execution.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return workflow.ErrNotExists
			}

			initiatedID := completionRequest.InitiatedID
			startedID := completionRequest.StartedID
			completedExecution := completionRequest.CompletedExecution
			completionEvent := completionRequest.CompletionEvent

			// Check mutable state to make sure child execution is in pending child executions
			ci, isRunning := mutableState.GetChildExecutionInfo(initiatedID)
			if !isRunning {
				if initiatedID >= mutableState.GetNextEventID() {
					fromOtherBranch, err := e.isChildCompletionFromOtherBranch(ctx, domainEntry, mutableState, completionRequest)
					if err != nil {
						return err
					}
					if fromOtherBranch {
						e.metricsClient.IncCounter(metrics.HistoryRecordChildExecutionCompletedScope, metrics.ChildWorkflowCompletionOtherBranchCounter)
						e.logger.Info("Drop the completion of a child that was started by an initiated event that can not be on the current branch",
							tag.WorkflowDomainName(domainEntry.GetInfo().Name),
							tag.WorkflowID(workflowExecution.GetWorkflowID()),
							tag.WorkflowRunID(workflowExecution.GetRunID()),
							tag.WorkflowInitiatedID(initiatedID),
							tag.WorkflowNextEventID(mutableState.GetNextEventID()),
							tag.Dynamic("child-start-version", *completionRequest.ChildStartVersion),
						)
						return &types.EntityNotExistsError{Message: "Pending child execution can not be on the current branch."}
					}
					e.metricsClient.IncCounter(metrics.HistoryRecordChildExecutionCompletedScope, metrics.StaleMutableStateCounter)
					e.logger.Error("Encounter stale mutable state in RecordChildExecutionCompleted",
						tag.WorkflowDomainName(domainEntry.GetInfo().Name),
						tag.WorkflowID(workflowExecution.GetWorkflowID()),
						tag.WorkflowRunID(workflowExecution.GetRunID()),
						tag.WorkflowInitiatedID(initiatedID),
						tag.WorkflowStartedID(startedID),
						tag.WorkflowNextEventID(mutableState.GetNextEventID()),
					)
					return workflow.ErrStaleState
				}
				// This can happen if the child execution is already completed, so it's deleted from mutable state
				// We must return a non-retryable error to avoid infinite retries
				return &types.EntityNotExistsError{Message: "Pending child execution not found."}
			}
			if ci.StartedID == constants.EmptyEventID {
				// This means that the child execution is initiated but not started in the view of the parent workflow,
				// but it receives a notification on child workflow completion.
				// This can happen if the parent workflow data is stale (due to shard movement), some bad guy is spoiling our API, or there is a bug.
				e.metricsClient.IncCounter(metrics.HistoryRecordChildExecutionCompletedScope, metrics.StaleMutableStateCounter)
				e.logger.Error("Encounter stale mutable state in RecordChildExecutionCompleted",
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowInitiatedID(initiatedID),
					tag.WorkflowStartedID(startedID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
				)
				return workflow.ErrStaleState
			}
			if ci.StartedWorkflowID != completedExecution.GetWorkflowID() {
				return &types.EntityNotExistsError{Message: "Pending child execution workflowID mismatch."}
			}
			firstRunID := completionRequest.FirstExecutionRunID
			// a completion without the first run ID can not be checked
			if firstRunID != "" && firstRunID != ci.StartedRunID {
				// the completion is from another run with the same workflow ID, for example a run that a new run replaced
				e.metricsClient.IncCounter(metrics.HistoryRecordChildExecutionCompletedScope, metrics.ChildWorkflowCompletionOtherRunCounter)
				e.logger.Info("Drop the completion of a child run that the parent did not start",
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowInitiatedID(initiatedID),
					tag.Dynamic("child-first-run-id", firstRunID),
					tag.Dynamic("child-started-run-id", ci.StartedRunID),
				)
				return &types.EntityNotExistsError{Message: "Pending child execution run mismatch."}
			}

			switch *completionEvent.EventType {
			case types.EventTypeWorkflowExecutionCompleted:
				attributes := completionEvent.WorkflowExecutionCompletedEventAttributes
				_, err = mutableState.AddChildWorkflowExecutionCompletedEvent(initiatedID, completedExecution, attributes)
			case types.EventTypeWorkflowExecutionFailed:
				attributes := completionEvent.WorkflowExecutionFailedEventAttributes
				_, err = mutableState.AddChildWorkflowExecutionFailedEvent(initiatedID, completedExecution, attributes)
			case types.EventTypeWorkflowExecutionCanceled:
				attributes := completionEvent.WorkflowExecutionCanceledEventAttributes
				_, err = mutableState.AddChildWorkflowExecutionCanceledEvent(initiatedID, completedExecution, attributes)
			case types.EventTypeWorkflowExecutionTerminated:
				attributes := completionEvent.WorkflowExecutionTerminatedEventAttributes
				_, err = mutableState.AddChildWorkflowExecutionTerminatedEvent(initiatedID, completedExecution, attributes)
			case types.EventTypeWorkflowExecutionTimedOut:
				attributes := completionEvent.WorkflowExecutionTimedOutEventAttributes
				_, err = mutableState.AddChildWorkflowExecutionTimedOutEvent(initiatedID, completedExecution, attributes)
			}
			return err
		})
}

// isChildCompletionFromOtherBranch returns true when the completed child was started by an initiated event that can not
// be added to the current branch of the parent.
func (e *historyEngineImpl) isChildCompletionFromOtherBranch(
	ctx context.Context,
	domainEntry *cache.DomainCacheEntry,
	mutableState execution.MutableState,
	completionRequest *types.RecordChildExecutionCompletedRequest,
) (bool, error) {
	childStartVersion := completionRequest.ChildStartVersion
	if childStartVersion == nil {
		// a completion without the start version of the child can not be checked
		return false, nil
	}
	if domainEntry.GetReplicationConfig().IsActiveActive() {
		// the versions of the child can be compared only when they come from the same cluster attribute as the versions
		// of the parent
		childPolicy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(
			ctx,
			domainEntry.GetInfo().ID,
			completionRequest.CompletedExecution.GetWorkflowID(),
			completionRequest.CompletedExecution.GetRunID(),
		)
		if err != nil {
			return false, err
		}
		if !childPolicy.Equals(mutableState.GetExecutionInfo().ActiveClusterSelectionPolicy) {
			return false, nil
		}
	}
	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return false, err
	}
	// the current branch already has a version higher than the start version of the child, and the initiated event of
	// the child can not have a higher version than the child start, so the initiated event can not be added to the
	// current branch
	return lastWriteVersion > *childStartVersion, nil
}
