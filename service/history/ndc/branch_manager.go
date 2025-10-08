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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination branch_manager_mock.go

package ndc

import (
	"context"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

const (
	outOfOrderDeliveryMessage = "Resend events due to out of order delivery"
)

type (
	branchManager interface {
		prepareVersionHistory(
			ctx context.Context,
			incomingVersionHistory *persistence.VersionHistory,
			incomingFirstEventID int64,
			incomingFirstEventVersion int64,
		) (bool, int, error)
	}

	branchManagerImpl struct {
		shard           shard.Context
		clusterMetadata cluster.Metadata
		historyV2Mgr    persistence.HistoryManager

		context      execution.Context
		mutableState execution.MutableState
		logger       log.Logger
	}
)

var _ branchManager = (*branchManagerImpl)(nil)

func newBranchManager(
	shard shard.Context,
	context execution.Context,
	mutableState execution.MutableState,
	logger log.Logger,
) branchManager {

	return &branchManagerImpl{
		shard:           shard,
		clusterMetadata: shard.GetService().GetClusterMetadata(),
		historyV2Mgr:    shard.GetHistoryManager(),

		context:      context,
		mutableState: mutableState,
		logger:       logger,
	}
}

func (r *branchManagerImpl) prepareVersionHistory(
	ctx context.Context,
	incomingVersionHistory *persistence.VersionHistory,
	incomingFirstEventID int64,
	incomingFirstEventVersion int64,
) (bool, int, error) {

	versionHistoryIndex, lcaVersionHistoryItem, err := r.flushBufferedEvents(ctx, incomingVersionHistory)
	if err != nil {
		return false, 0, err
	}

	localVersionHistories := r.mutableState.GetVersionHistories()
	if localVersionHistories == nil {
		return false, 0, execution.ErrMissingVersionHistories
	}
	versionHistory, err := localVersionHistories.GetVersionHistory(versionHistoryIndex)
	if err != nil {
		return false, 0, err
	}

	// if can directly append to a branch
	if versionHistory.IsLCAAppendable(lcaVersionHistoryItem) {
		doContinue, err := r.verifyEventsOrder(
			ctx,
			versionHistory,
			incomingFirstEventID,
			incomingFirstEventVersion,
		)
		if err != nil {
			return false, 0, err
		}
		return doContinue, versionHistoryIndex, nil
	}

	newVersionHistory, err := versionHistory.DuplicateUntilLCAItem(lcaVersionHistoryItem)
	if err != nil {
		return false, 0, err
	}

	// if cannot directly append to the new branch to be created
	doContinue, err := r.verifyEventsOrder(
		ctx,
		newVersionHistory,
		incomingFirstEventID,
		incomingFirstEventVersion,
	)
	if err != nil || !doContinue {
		return false, 0, err
	}

	newVersionHistoryIndex, err := r.createNewBranch(
		ctx,
		versionHistory.GetBranchToken(),
		lcaVersionHistoryItem.EventID,
		newVersionHistory,
	)
	if err != nil {
		return false, 0, err
	}

	return true, newVersionHistoryIndex, nil
}

func (r *branchManagerImpl) flushBufferedEvents(
	ctx context.Context,
	incomingVersionHistory *persistence.VersionHistory,
) (int, *persistence.VersionHistoryItem, error) {

	localVersionHistories := r.mutableState.GetVersionHistories()
	if localVersionHistories == nil {
		return 0, nil, execution.ErrMissingVersionHistories
	}
	versionHistoryIndex, lcaVersionHistoryItem, err := localVersionHistories.FindLCAVersionHistoryIndexAndItem(
		incomingVersionHistory,
	)
	if err != nil {
		return 0, nil, err
	}

	// check whether there are buffered events, if so, flush it
	// NOTE: buffered events does not show in version history or next event id
	if !r.mutableState.HasBufferedEvents() {
		return versionHistoryIndex, lcaVersionHistoryItem, nil
	}

	targetWorkflow := execution.NewWorkflow(
		ctx,
		r.clusterMetadata,
		r.context,
		r.mutableState,
		execution.NoopReleaseFn,
		r.logger,
	)
	if err := targetWorkflow.FlushBufferedEvents(); err != nil {
		return 0, nil, err
	}
	// the workflow must be updated as active, to send out replication tasks
	r.logger.Debug("flushBufferedEvents calling UpdateWorkflowExecutionAsActive", tag.WorkflowID(r.mutableState.GetExecutionInfo().WorkflowID))
	if err := targetWorkflow.GetContext().UpdateWorkflowExecutionAsActive(
		ctx,
		r.shard.GetTimeSource().Now(),
	); err != nil {
		return 0, nil, err
	}

	r.context = targetWorkflow.GetContext()
	r.mutableState = targetWorkflow.GetMutableState()

	localVersionHistories = r.mutableState.GetVersionHistories()
	return localVersionHistories.FindLCAVersionHistoryIndexAndItem(incomingVersionHistory)
}

func (r *branchManagerImpl) verifyEventsOrder(
	ctx context.Context,
	localVersionHistory *persistence.VersionHistory,
	incomingFirstEventID int64,
	incomingFirstEventVersion int64,
) (bool, error) {

	lastVersionHistoryItem, err := localVersionHistory.GetLastItem()
	if err != nil {
		return false, err
	}
	nextEventID := lastVersionHistoryItem.EventID + 1

	if incomingFirstEventID < nextEventID {
		// duplicate replication task
		return false, nil
	}
	if incomingFirstEventID > nextEventID {
		executionInfo := r.mutableState.GetExecutionInfo()
		return false, newNDCRetryTaskErrorWithHint(
			outOfOrderDeliveryMessage,
			executionInfo.DomainID,
			executionInfo.WorkflowID,
			executionInfo.RunID,
			common.Int64Ptr(lastVersionHistoryItem.EventID),
			common.Int64Ptr(lastVersionHistoryItem.Version),
			common.Int64Ptr(incomingFirstEventID),
			common.Int64Ptr(incomingFirstEventVersion))
	}
	// task.getFirstEvent().GetEventId() == nextEventID
	return true, nil
}

func (r *branchManagerImpl) createNewBranch(
	ctx context.Context,
	baseBranchToken []byte,
	baseBranchLastEventID int64,
	newVersionHistory *persistence.VersionHistory,
) (newVersionHistoryIndex int, retError error) {

	shardID := r.shard.GetShardID()
	executionInfo := r.mutableState.GetExecutionInfo()
	domainID := executionInfo.DomainID
	workflowID := executionInfo.WorkflowID
	domainName, err := r.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return 0, err
	}

	resp, err := r.historyV2Mgr.ForkHistoryBranch(ctx, &persistence.ForkHistoryBranchRequest{
		ForkBranchToken: baseBranchToken,
		ForkNodeID:      baseBranchLastEventID + 1,
		Info:            persistence.BuildHistoryGarbageCleanupInfo(domainID, workflowID, uuid.New()),
		ShardID:         common.IntPtr(shardID),
		DomainName:      domainName,
	})
	if err != nil {
		return 0, err
	}

	if err := newVersionHistory.SetBranchToken(resp.NewBranchToken); err != nil {
		return 0, err
	}
	versionHistory := r.mutableState.GetVersionHistories()
	if versionHistory == nil {
		return 0, execution.ErrMissingVersionHistories
	}
	branchChanged, newIndex, err := versionHistory.AddVersionHistory(
		newVersionHistory,
	)
	if err != nil {
		return 0, err
	}
	if branchChanged {
		return 0, &types.BadRequestError{
			Message: "nDCBranchMgr encounter branch change during conflict resolution",
		}
	}

	return newIndex, nil
}
