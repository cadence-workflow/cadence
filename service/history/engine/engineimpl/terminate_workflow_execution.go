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

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

func (e *historyEngineImpl) TerminateWorkflowExecution(
	ctx context.Context,
	terminateRequest *types.HistoryTerminateWorkflowExecutionRequest,
) error {
	request := terminateRequest.TerminateRequest
	parentExecution := terminateRequest.ExternalWorkflowExecution
	childWorkflowOnly := terminateRequest.GetChildWorkflowOnly()
	workflowExecution := types.WorkflowExecution{
		WorkflowID: request.WorkflowExecution.WorkflowID,
	}
	// If firstExecutionRunID is set on the request always try to cancel currently running execution
	if request.GetFirstExecutionRunID() == "" {
		workflowExecution.RunID = request.WorkflowExecution.RunID
	}

	domainEntry, err := e.getActiveDomainByWorkflow(ctx, terminateRequest.DomainUUID, workflowExecution.WorkflowID, workflowExecution.RunID)
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	return workflow.UpdateCurrentWithActionFunc(
		ctx,
		e.logger,
		e.executionCache,
		e.executionManager,
		domainID,
		e.shard.GetDomainCache(),
		workflowExecution,
		e.timeSource.Now(),
		func(wfContext execution.Context, mutableState execution.MutableState) (*workflow.UpdateAction, error) {
			if !mutableState.IsWorkflowExecutionRunning() {
				return nil, workflow.ErrAlreadyCompleted
			}

			executionInfo := mutableState.GetExecutionInfo()
			if request.GetFirstExecutionRunID() != "" {
				firstRunID := executionInfo.FirstExecutionRunID
				if firstRunID == "" {
					// This is needed for backwards compatibility.  Workflow execution create with Cadence release v0.25.0 or earlier
					// does not have FirstExecutionRunID stored as part of mutable state.  If this is not set then load it from
					// workflow execution started event.
					startEvent, err := mutableState.GetStartEvent(ctx)
					if err != nil {
						return nil, err
					}
					firstRunID = startEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstExecutionRunID()
				}
				if request.GetFirstExecutionRunID() != firstRunID {
					return nil, &types.EntityNotExistsError{Message: "Workflow execution not found"}
				}
			}
			if childWorkflowOnly {
				parentWorkflowID := executionInfo.ParentWorkflowID
				parentRunID := executionInfo.ParentRunID
				if parentExecution.GetWorkflowID() != parentWorkflowID ||
					parentExecution.GetRunID() != parentRunID {
					return nil, workflow.ErrParentMismatch
				}
			}

			eventBatchFirstEventID := mutableState.GetNextEventID()
			return workflow.UpdateWithoutDecision, execution.TerminateWorkflow(
				mutableState,
				eventBatchFirstEventID,
				request.GetReason(),
				request.GetDetails(),
				request.GetIdentity(),
			)
		})
}
