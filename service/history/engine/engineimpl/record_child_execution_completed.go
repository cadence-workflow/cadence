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

	domainEntry, err := e.getActiveDomainByID(completionRequest.DomainUUID)
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
