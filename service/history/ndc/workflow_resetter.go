// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination workflow_resetter_mock.go

package ndc

import (
	"context"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

const (
	resendOnResetWorkflowMessage = "Resend events due to reset workflow"
)

type (
	// WorkflowResetter handles workflow reset for NDC
	WorkflowResetter interface {
		ResetWorkflow(
			ctx context.Context,
			now time.Time,
			baseLastEventID int64,
			baseLastEventVersion int64,
			incomingFirstEventID int64,
			incomingFirstEventVersion int64,
		) (execution.MutableState, error)
	}

	workflowResetterImpl struct {
		shard              shard.Context
		transactionManager transactionManager
		historyV2Manager   persistence.HistoryManager
		stateRebuilder     execution.StateRebuilder

		domainID   string
		workflowID string
		baseRunID  string
		newContext execution.Context
		newRunID   string

		logger log.Logger
	}
)

var _ WorkflowResetter = (*workflowResetterImpl)(nil)

// NewWorkflowResetter creates workflow resetter
func NewWorkflowResetter(
	shard shard.Context,
	transactionManager transactionManager,
	domainID string,
	workflowID string,
	baseRunID string,
	newContext execution.Context,
	newRunID string,
	logger log.Logger,
) WorkflowResetter {

	return &workflowResetterImpl{
		shard:              shard,
		transactionManager: transactionManager,
		historyV2Manager:   shard.GetHistoryManager(),
		stateRebuilder:     execution.NewStateRebuilder(shard, logger),

		domainID:   domainID,
		workflowID: workflowID,
		baseRunID:  baseRunID,
		newContext: newContext,
		newRunID:   newRunID,
		logger:     logger,
	}
}

func (r *workflowResetterImpl) ResetWorkflow(
	ctx context.Context,
	now time.Time,
	baseLastEventID int64,
	baseLastEventVersion int64,
	incomingFirstEventID int64,
	incomingFirstEventVersion int64,
) (execution.MutableState, error) {

	baseBranchToken, err := r.getBaseBranchToken(
		ctx,
		baseLastEventID,
		baseLastEventVersion,
		incomingFirstEventID,
		incomingFirstEventVersion,
	)

	if err != nil {
		return nil, err
	}

	resetBranchToken, err := r.getResetBranchToken(ctx, baseBranchToken, baseLastEventID)
	if err != nil {
		return nil, err
	}

	requestID := uuid.New()
	rebuildMutableState, rebuiltHistorySize, err := r.stateRebuilder.Rebuild(
		ctx,
		now,
		definition.NewWorkflowIdentifier(
			r.domainID,
			r.workflowID,
			r.baseRunID,
		),
		baseBranchToken,
		baseLastEventID,
		baseLastEventVersion,
		definition.NewWorkflowIdentifier(
			r.domainID,
			r.workflowID,
			r.newRunID,
		),
		resetBranchToken,
		requestID,
	)
	if err != nil {
		return nil, err
	}

	r.newContext.Clear()
	r.newContext.SetHistorySize(rebuiltHistorySize)
	return rebuildMutableState, nil
}

func (r *workflowResetterImpl) getBaseBranchToken(
	ctx context.Context,
	baseLastEventID int64,
	baseLastEventVersion int64,
	incomingFirstEventID int64,
	incomingFirstEventVersion int64,
) (baseBranchToken []byte, retError error) {

	baseWorkflow, err := r.transactionManager.loadNDCWorkflow(
		ctx,
		r.domainID,
		r.workflowID,
		r.baseRunID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		baseWorkflow.GetReleaseFn()(retError)
	}()

	mutableState := baseWorkflow.GetMutableState()
	branchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	baseVersionHistories := mutableState.GetVersionHistories()
	if baseVersionHistories != nil {

		_, baseVersionHistory, err := baseVersionHistories.FindFirstVersionHistoryByItem(
			persistence.NewVersionHistoryItem(baseLastEventID, baseLastEventVersion),
		)
		if err != nil {
			// the base event and incoming event are from different branch
			// only re-replicate the gap on the incoming branch
			// the base branch event will eventually arrived
			return nil, newNDCRetryTaskErrorWithHint(
				resendOnResetWorkflowMessage,
				r.domainID,
				r.workflowID,
				r.newRunID,
				nil,
				nil,
				common.Int64Ptr(incomingFirstEventID),
				common.Int64Ptr(incomingFirstEventVersion),
			)
		}

		branchToken = baseVersionHistory.GetBranchToken()
	}
	return branchToken, nil
}

func (r *workflowResetterImpl) getResetBranchToken(
	ctx context.Context,
	baseBranchToken []byte,
	baseLastEventID int64,
) ([]byte, error) {

	// fork a new history branch
	shardID := r.shard.GetShardID()
	domainName, err := r.shard.GetDomainCache().GetDomainName(r.domainID)
	if err != nil {
		return nil, err
	}
	resp, err := r.historyV2Manager.ForkHistoryBranch(
		ctx,
		&persistence.ForkHistoryBranchRequest{
			ForkBranchToken: baseBranchToken,
			ForkNodeID:      baseLastEventID + 1,
			Info:            persistence.BuildHistoryGarbageCleanupInfo(r.domainID, r.workflowID, r.newRunID),
			ShardID:         common.IntPtr(shardID),
			DomainName:      domainName,
		},
	)
	if err != nil {
		return nil, err
	}

	return resp.NewBranchToken, nil
}
