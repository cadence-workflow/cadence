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
	"fmt"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	persistenceutils "github.com/uber/cadence/common/persistence/persistence-utils"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

const (
	DefaultPageSize = 100
)

func (e *historyEngineImpl) ResetWorkflowExecution(
	ctx context.Context,
	resetRequest *types.HistoryResetWorkflowExecutionRequest,
) (response *types.ResetWorkflowExecutionResponse, retError error) {

	request := resetRequest.ResetRequest
	domainID := resetRequest.GetDomainUUID()
	workflowID := request.WorkflowExecution.GetWorkflowID()
	baseRunID := request.WorkflowExecution.GetRunID()

	baseContext, baseReleaseFn, err := e.executionCache.GetOrCreateWorkflowExecution(
		ctx,
		domainID,
		types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      baseRunID,
		},
	)
	if err != nil {
		return nil, err
	}
	defer func() { baseReleaseFn(retError) }()

	baseMutableState, err := baseContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return nil, err
	}
	if ok := baseMutableState.HasProcessedOrPendingDecision(); !ok {
		return nil, &types.BadRequestError{
			Message: "Cannot reset workflow without a decision task schedule.",
		}
	}
	if request.GetDecisionFinishEventID() <= constants.FirstEventID ||
		request.GetDecisionFinishEventID() > baseMutableState.GetNextEventID() {
		return nil, &types.BadRequestError{
			Message: "Decision finish ID must be > 1 && <= workflow next event ID.",
		}
	}
	domainName, err := e.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return nil, err
	}
	// also load the current run of the workflow, it can be different from the base runID
	resp, err := e.executionManager.GetCurrentExecution(ctx, &persistence.GetCurrentExecutionRequest{
		DomainID:   domainID,
		WorkflowID: request.WorkflowExecution.GetWorkflowID(),
		DomainName: domainName,
	})
	if err != nil {
		return nil, err
	}

	currentRunID := resp.RunID
	var currentContext execution.Context
	var currentMutableState execution.MutableState
	var currentReleaseFn execution.ReleaseFunc
	if currentRunID == baseRunID {
		currentContext = baseContext
		currentMutableState = baseMutableState
	} else {
		currentContext, currentReleaseFn, err = e.executionCache.GetOrCreateWorkflowExecution(
			ctx,
			domainID,
			types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      currentRunID,
			},
		)
		if err != nil {
			return nil, err
		}
		defer func() { currentReleaseFn(retError) }()

		currentMutableState, err = currentContext.LoadWorkflowExecution(ctx)
		if err != nil {
			return nil, err
		}
	}

	// dedup by requestID
	if currentMutableState.GetExecutionInfo().CreateRequestID == request.GetRequestID() {
		e.logger.Info("Duplicated reset request",
			tag.WorkflowID(workflowID),
			tag.WorkflowRunID(currentRunID),
			tag.WorkflowDomainID(domainID))
		return &types.ResetWorkflowExecutionResponse{
			RunID: currentRunID,
		}, nil
	}

	resetRunID := uuid.New()
	baseRebuildLastEventID := request.GetDecisionFinishEventID() - 1
	baseVersionHistories := baseMutableState.GetVersionHistories()
	baseCurrentBranchToken, err := baseMutableState.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	baseRebuildLastEventVersion := baseMutableState.GetCurrentVersion()
	baseNextEventID := baseMutableState.GetNextEventID()

	if baseVersionHistories != nil {
		baseCurrentVersionHistory, err := baseVersionHistories.GetCurrentVersionHistory()
		if err != nil {
			return nil, err
		}
		baseRebuildLastEventVersion, err = baseCurrentVersionHistory.GetEventVersion(baseRebuildLastEventID)
		if err != nil {
			return nil, err
		}
		baseCurrentBranchToken = baseCurrentVersionHistory.GetBranchToken()
	}

	// for reset workflow execution requests, the caller provides the decision finish event ID.
	// must validate the event ID to ensure it is a valid reset point
	err = e.validateResetPointForResetWorkflowExecutionRequest(ctx, request, baseCurrentBranchToken, domainID)
	if err != nil {
		return nil, err
	}

	if err := e.workflowResetter.ResetWorkflow(
		ctx,
		domainID,
		workflowID,
		baseRunID,
		baseCurrentBranchToken,
		baseRebuildLastEventID,
		baseRebuildLastEventVersion,
		baseNextEventID,
		resetRunID,
		request.GetRequestID(),
		execution.NewWorkflow(
			ctx,
			e.shard.GetClusterMetadata(),
			e.shard.GetActiveClusterManager(),
			currentContext,
			currentMutableState,
			currentReleaseFn,
			e.logger,
		),
		request.GetReason(),
		nil,
		request.GetSkipSignalReapply(),
	); err != nil {
		if t, ok := persistence.AsDuplicateRequestError(err); ok {
			if t.RequestType == persistence.WorkflowRequestTypeReset {
				return &types.ResetWorkflowExecutionResponse{
					RunID: t.RunID,
				}, nil
			}
			e.logger.Error("A bug is detected for idempotency improvement", tag.Dynamic("request-type", t.RequestType))
			return nil, t
		}
		return nil, err
	}
	return &types.ResetWorkflowExecutionResponse{
		RunID: resetRunID,
	}, nil
}

func (e *historyEngineImpl) validateResetPointForResetWorkflowExecutionRequest(
	ctx context.Context,
	request *types.ResetWorkflowExecutionRequest,
	baseCurrentBranchToken []byte,
	domainID string,
) error {
	iter := collection.NewPagingIterator(e.getPaginationFn(
		ctx,
		constants.FirstEventID,
		request.GetDecisionFinishEventID()+1,
		baseCurrentBranchToken,
		domainID,
	))

	if !iter.HasNext() {
		return fmt.Errorf("workflow has corrupted or missing history")
	}

	var events []*types.HistoryEvent

	for iter.HasNext() {
		batch, err := iter.Next()
		if err != nil {
			return err
		}

		events = batch.(*types.History).Events

		// get the batch of events that contains the reset event and exit the loop
		if events[len(events)-1].ID >= request.GetDecisionFinishEventID() {
			break
		}
	}

	return checkResetEventType(events, request.GetDecisionFinishEventID())
}

func (e *historyEngineImpl) getPaginationFn(
	ctx context.Context,
	firstEventID int64,
	nextEventID int64,
	branchToken []byte,
	domainID string,
) collection.PaginationFn {

	return func(paginationToken []byte) ([]interface{}, []byte, error) {
		_, historyBatches, token, _, err := persistenceutils.PaginateHistory(
			ctx,
			e.historyV2Mgr,
			true,
			branchToken,
			firstEventID,
			nextEventID,
			paginationToken,
			DefaultPageSize,
			common.IntPtr(e.shard.GetShardID()),
			domainID,
			e.shard.GetDomainCache(),
		)
		if err != nil {
			return nil, nil, err
		}

		var paginateItems []interface{}
		for _, history := range historyBatches {
			paginateItems = append(paginateItems, history)
		}
		return paginateItems, token, nil
	}
}

// checkResetEventType checks the type of the reset event and only allows specific types to be resettable
func checkResetEventType(events []*types.HistoryEvent, resetEventID int64) error {
	for _, event := range events {
		if event.ID == resetEventID {
			switch *event.EventType {
			case types.EventTypeDecisionTaskStarted:
				return nil
			case types.EventTypeDecisionTaskTimedOut:
				return nil
			case types.EventTypeDecisionTaskFailed:
				return nil
			case types.EventTypeDecisionTaskCompleted:
				return nil
			default:
				return &types.BadRequestError{
					Message: fmt.Sprintf("reset event must be of type DecisionTaskStarted, DecisionTaskTimedOut, DecisionTaskFailed, or DecisionTaskCompleted. Attempting to reset on event type: %v", event.EventType.String()),
				}
			}
		}
	}

	return fmt.Errorf("reset event ID %v not found in the history events", resetEventID)
}
