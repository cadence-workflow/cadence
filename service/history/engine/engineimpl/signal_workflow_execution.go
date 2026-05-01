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

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

func (e *historyEngineImpl) SignalWorkflowExecution(
	ctx context.Context,
	signalRequest *types.HistorySignalWorkflowExecutionRequest,
) error {
	request := signalRequest.SignalRequest
	workflowExecution := types.WorkflowExecution{
		WorkflowID: request.WorkflowExecution.WorkflowID,
		RunID:      request.WorkflowExecution.RunID,
	}
	domainEntry, err := e.getActiveDomainByWorkflow(ctx, signalRequest.DomainUUID, workflowExecution.WorkflowID, workflowExecution.RunID)
	if err != nil {
		return err
	}
	if domainEntry.GetInfo().Status != persistence.DomainStatusRegistered {
		return errDomainDeprecated
	}
	domainID := domainEntry.GetInfo().ID
	parentExecution := signalRequest.ExternalWorkflowExecution
	childWorkflowOnly := signalRequest.GetChildWorkflowOnly()

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
			// first deduplicate by request id for signal decision
			// this is done before workflow running check so that already completed error
			// won't be returned for duplicated signals even if the workflow is closed.
			if requestID := request.GetRequestID(); requestID != "" {
				if mutableState.IsSignalRequested(requestID) {
					return &workflow.UpdateAction{
						Noop:           true,
						CreateDecision: false,
					}, nil
				}
			}

			if !mutableState.IsWorkflowExecutionRunning() {
				return nil, workflow.ErrAlreadyCompleted
			}

			// If history is corrupted, signal will be rejected
			if corrupted, err := e.checkForHistoryCorruptions(ctx, mutableState); err != nil {
				return nil, err
			} else if corrupted {
				return nil, &types.EntityNotExistsError{Message: "Workflow execution corrupted."}
			}

			executionInfo := mutableState.GetExecutionInfo()
			createDecisionTask := true
			if !mutableState.HasProcessedOrPendingDecision() {
				createDecisionTask = false
			}

			maxAllowedSignals := e.config.MaximumSignalsPerExecution(domainEntry.GetInfo().Name)
			if maxAllowedSignals > 0 && int(executionInfo.SignalCount) >= maxAllowedSignals {
				e.logger.Info("Execution limit reached for maximum signals", tag.WorkflowSignalCount(executionInfo.SignalCount),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowDomainID(domainID))
				return nil, workflow.ErrSignalsLimitExceeded
			}

			if childWorkflowOnly {
				parentWorkflowID := executionInfo.ParentWorkflowID
				parentRunID := executionInfo.ParentRunID
				if parentExecution.GetWorkflowID() != parentWorkflowID ||
					parentExecution.GetRunID() != parentRunID {
					return nil, workflow.ErrParentMismatch
				}
			}

			if requestID := request.GetRequestID(); requestID != "" {
				mutableState.AddSignalRequested(requestID)
			}

			if _, err := mutableState.AddWorkflowExecutionSignaled(
				request.GetSignalName(),
				request.GetInput(),
				request.GetIdentity(),
				request.GetRequestID(),
			); err != nil {
				return nil, &types.InternalServiceError{Message: "Unable to signal workflow execution."}
			}

			return &workflow.UpdateAction{
				Noop:           false,
				CreateDecision: createDecisionTask,
			}, nil
		})
}
