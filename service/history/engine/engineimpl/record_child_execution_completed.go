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

			// the current branch of the parent does not wait for a child from another branch, so drop its completion with
			// a non-retryable error
			dropFromOtherBranch := func() error {
				e.metricsClient.IncCounter(metrics.HistoryRecordChildExecutionCompletedScope, metrics.ChildWorkflowCompletionOtherBranchCounter)
				e.logger.Info("Drop the completion of a child that was started by an initiated event that is not on the current branch",
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowInitiatedID(initiatedID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
					tag.Dynamic("child-first-run-id", completionRequest.FirstExecutionRunID),
					tag.Dynamic("child-start-version", completionRequest.ChildStartVersion),
				)
				return &types.EntityNotExistsError{Message: "Pending child execution is from another branch of the parent."}
			}

			// Check mutable state to make sure child execution is in pending child executions
			ci, isRunning := mutableState.GetChildExecutionInfo(initiatedID)
			if !isRunning {
				if initiatedID >= mutableState.GetNextEventID() {
					childStartVersion := completionRequest.ChildStartVersion
					// a completion without the start version of the child can not be checked
					versionsComparable := childStartVersion != nil
					if versionsComparable && domainEntry.GetReplicationConfig().IsActiveActive() {
						// the versions of the child can be compared only when they come from the same cluster attribute as
						// the versions of the parent
						childPolicy, err := e.shard.GetActiveClusterManager().GetActiveClusterSelectionPolicyForWorkflow(
							ctx,
							domainID,
							completedExecution.GetWorkflowID(),
							completedExecution.GetRunID(),
						)
						if err != nil {
							return err
						}
						versionsComparable = childPolicy.Equals(mutableState.GetExecutionInfo().ActiveClusterSelectionPolicy)
					}
					if versionsComparable {
						lastWriteVersion, err := mutableState.GetLastWriteVersion()
						if err != nil {
							return err
						}
						// the current branch does not have an event with the initiated ID of the child, as checked above.
						// When the child start version is lower than the last write version, the initiated event can not
						// be added to the current branch, so the child is from another branch. When it is equal or
						// higher, the initiated event can still be added, for example because the parent state is stale.
						if lastWriteVersion > *childStartVersion {
							return dropFromOtherBranch()
						}
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
				// the completion is from a child run that the current branch of the parent did not start, for example a
				// closed child run from a losing branch, after the current branch started a new run with the same
				// workflow ID
				return dropFromOtherBranch()
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
