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

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
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

	// validate if workflow is resettable
	if err := e.isWorkflowResettable(baseMutableState, domainID); err != nil {
		return nil, &types.BadRequestError{
			Message: fmt.Sprintf("workflow is not resettable. Error: %v", err),
		}
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

/*
If a workflow was started in a different cluster, it is not resettable because replication stack is not able to handle workflow start event.
*/
func (e *historyEngineImpl) isWorkflowResettable(baseMutableState execution.MutableState, domainID string) error {
	startVersion, err := baseMutableState.GetStartVersion()
	if err != nil {
		return fmt.Errorf("fail to get failover version of workflow start event: %w", err)
	}

	startCluster, err := e.shard.GetActiveClusterManager().ClusterNameForFailoverVersion(startVersion, domainID)
	if err != nil {
		return fmt.Errorf("fail to get cluster name for failover version: %w", err)
	}

	if currentCluster := e.clusterMetadata.GetCurrentClusterName(); currentCluster != startCluster {
		return fmt.Errorf("workflow was not started in the current cluster: failover to workflow start cluster %s before reset", startCluster)
	}

	return nil
}
