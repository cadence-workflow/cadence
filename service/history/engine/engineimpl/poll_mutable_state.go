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
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// GetMutableState retrieves the mutable state of the workflow execution
func (e *historyEngineImpl) GetMutableState(ctx context.Context, request *types.GetMutableStateRequest) (*types.GetMutableStateResponse, error) {
	return e.getMutableStateOrLongPoll(ctx, request)
}

// PollMutableState retrieves the mutable state of the workflow execution with long polling
func (e *historyEngineImpl) PollMutableState(ctx context.Context, request *types.PollMutableStateRequest) (*types.PollMutableStateResponse, error) {
	response, err := e.getMutableStateOrLongPoll(ctx, &types.GetMutableStateRequest{
		DomainUUID:          request.DomainUUID,
		Execution:           request.Execution,
		ExpectedNextEventID: request.ExpectedNextEventID,
		CurrentBranchToken:  request.CurrentBranchToken,
		VersionHistoryItem:  request.GetVersionHistoryItem(),
	})

	if err != nil {
		return nil, e.updateEntityNotExistsErrorOnPassiveCluster(ctx, err, request.GetDomainUUID(), request.Execution)
	}

	return &types.PollMutableStateResponse{
		Execution:                            response.Execution,
		WorkflowType:                         response.WorkflowType,
		NextEventID:                          response.NextEventID,
		PreviousStartedEventID:               response.PreviousStartedEventID,
		LastFirstEventID:                     response.LastFirstEventID,
		TaskList:                             response.TaskList,
		StickyTaskList:                       response.StickyTaskList,
		ClientLibraryVersion:                 response.ClientLibraryVersion,
		ClientFeatureVersion:                 response.ClientFeatureVersion,
		ClientImpl:                           response.ClientImpl,
		StickyTaskListScheduleToStartTimeout: response.StickyTaskListScheduleToStartTimeout,
		CurrentBranchToken:                   response.CurrentBranchToken,
		VersionHistories:                     response.VersionHistories,
		WorkflowState:                        response.WorkflowState,
		WorkflowCloseState:                   response.WorkflowCloseState,
	}, nil
}

func (e *historyEngineImpl) getMutableState(
	ctx context.Context,
	domainID string,
	execution types.WorkflowExecution,
) (retResp *types.GetMutableStateResponse, retError error) {

	wfContext, release, retError := e.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, execution)
	if retError != nil {
		return
	}
	defer func() { release(retError) }()

	mutableState, retError := wfContext.LoadWorkflowExecution(ctx)
	if retError != nil {
		return
	}

	currentBranchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}

	executionInfo := mutableState.GetExecutionInfo()
	execution.RunID = wfContext.GetExecution().RunID
	workflowState, workflowCloseState := mutableState.GetWorkflowStateCloseStatus()
	retResp = &types.GetMutableStateResponse{
		Execution:                            &execution,
		WorkflowType:                         &types.WorkflowType{Name: executionInfo.WorkflowTypeName},
		LastFirstEventID:                     mutableState.GetLastFirstEventID(),
		NextEventID:                          mutableState.GetNextEventID(),
		PreviousStartedEventID:               common.Int64Ptr(mutableState.GetPreviousStartedEventID()),
		TaskList:                             &types.TaskList{Name: executionInfo.TaskList},
		StickyTaskList:                       &types.TaskList{Name: executionInfo.StickyTaskList, Kind: types.TaskListKindSticky.Ptr()},
		ClientLibraryVersion:                 executionInfo.ClientLibraryVersion,
		ClientFeatureVersion:                 executionInfo.ClientFeatureVersion,
		ClientImpl:                           executionInfo.ClientImpl,
		IsWorkflowRunning:                    mutableState.IsWorkflowExecutionRunning(),
		StickyTaskListScheduleToStartTimeout: common.Int32Ptr(executionInfo.StickyScheduleToStartTimeout),
		CurrentBranchToken:                   currentBranchToken,
		WorkflowState:                        common.Int32Ptr(int32(workflowState)),
		WorkflowCloseState:                   common.Int32Ptr(int32(workflowCloseState)),
		IsStickyTaskListEnabled:              mutableState.IsStickyTaskListEnabled(),
		HistorySize:                          mutableState.GetHistorySize(),
	}
	versionHistories := mutableState.GetVersionHistories()
	if versionHistories != nil {
		retResp.VersionHistories = versionHistories.ToInternalType()
	}
	return
}

func (e *historyEngineImpl) updateEntityNotExistsErrorOnPassiveCluster(ctx context.Context, err error, domainID string, execution *types.WorkflowExecution) error {
	switch err.(type) {
	case *types.EntityNotExistsError:
		activeClusterInfo, activeClusterErr := e.activeClusterManager.GetActiveClusterInfoByWorkflow(ctx, domainID, execution.GetWorkflowID(), execution.GetRunID())
		if activeClusterErr != nil {
			return err // return original error if could not access active cluster manager
		}
		if activeClusterInfo.ActiveClusterName != e.clusterMetadata.GetCurrentClusterName() {
			return &types.EntityNotExistsError{
				Message:        "Workflow execution not found in non-active cluster",
				ActiveCluster:  activeClusterInfo.ActiveClusterName,
				CurrentCluster: e.clusterMetadata.GetCurrentClusterName(),
			}
		}
	}
	return err
}

func (e *historyEngineImpl) getMutableStateWithCurrentVersionHistory(
	ctx context.Context,
	domainID string,
	execution types.WorkflowExecution,
) (*types.GetMutableStateResponse, *persistence.VersionHistory, error) {

	mutableState, err := e.getMutableState(ctx, domainID, execution)
	if err != nil {
		return nil, nil, err
	}

	// must be corrupted workflow
	if mutableState.GetVersionHistories() == nil {
		e.logger.Warn("version histories do not exist")
		return nil, nil, &types.InternalDataInconsistencyError{Message: "version histories do not exist"}
	}

	currentVersionHistory, err := persistence.NewVersionHistoriesFromInternalType(
		mutableState.GetVersionHistories(),
	).GetCurrentVersionHistory()

	if err != nil {
		return nil, nil, fmt.Errorf("unable to get version history while looking up response for mutable state: %w", err)
	}

	return mutableState, currentVersionHistory, err
}

func (e *historyEngineImpl) getMutableStateOrLongPoll(
	ctx context.Context,
	request *types.GetMutableStateRequest,
) (*types.GetMutableStateResponse, error) {

	if err := common.ValidateDomainUUID(request.DomainUUID); err != nil {
		return nil, err
	}
	domainID := request.DomainUUID
	execution := types.WorkflowExecution{
		WorkflowID: request.Execution.WorkflowID,
		RunID:      request.Execution.RunID,
	}

	mutableState, currentVersionHistory, err := e.getMutableStateWithCurrentVersionHistory(ctx, domainID, execution)
	if err != nil {
		return nil, err
	}

	if request.VersionHistoryItem == nil {
		lastVersionHistoryItem, err := currentVersionHistory.GetLastItem()
		if err != nil {
			return nil, err
		}
		request.VersionHistoryItem = lastVersionHistoryItem.ToInternalType()
	}
	// Use the latest event id + event version as the branch identifier. This pair is unique across clusters.
	// We return the full version histories. Callers need to fetch the last version history item from current branch
	// and use the last version history item in following calls.
	if !currentVersionHistory.ContainsItem(persistence.NewVersionHistoryItemFromInternalType(request.VersionHistoryItem)) {
		e.logger.Warn("current version history and requested one are different - found on first check",
			tag.Dynamic("current-version-history", currentVersionHistory),
			tag.Dynamic("requested-version-history", request.VersionHistoryItem),
		)
		return nil, &types.CurrentBranchChangedError{
			Message:            "current branch token and request branch token doesn't match",
			CurrentBranchToken: mutableState.CurrentBranchToken}
	}
	// set the run id in case query the current running workflow
	execution.RunID = mutableState.Execution.RunID

	// expectedNextEventID is 0 when caller want to get the current next event ID without blocking
	expectedNextEventID := constants.FirstEventID
	if request.ExpectedNextEventID != 0 {
		expectedNextEventID = request.GetExpectedNextEventID()
	}

	if expectedNextEventID < mutableState.GetNextEventID() || !mutableState.GetIsWorkflowRunning() {
		return mutableState, nil
	}
	// if caller decide to long poll on workflow execution
	// and the event ID we are looking for is smaller than current next event ID
	return e.longPollForEventID(ctx, expectedNextEventID, domainID, execution, request.VersionHistoryItem)
}

func (e *historyEngineImpl) longPollForEventID(
	ctx context.Context,
	expectedNextEventID int64,
	domainID string,
	execution types.WorkflowExecution,
	versionHistoryItem *types.VersionHistoryItem,
) (*types.GetMutableStateResponse, error) {

	wfIdentifier := definition.NewWorkflowIdentifier(domainID, execution.GetWorkflowID(), execution.GetRunID())
	subscriberID, channel, err := e.historyEventNotifier.WatchHistoryEvent(wfIdentifier)
	if err != nil {
		return nil, err
	}
	defer e.historyEventNotifier.UnwatchHistoryEvent(wfIdentifier, subscriberID) //nolint:errcheck

	// check again in case the next event ID is updated before we subscribed
	mutableState, currentVersionHistory, err := e.getMutableStateWithCurrentVersionHistory(ctx, domainID, execution)
	if err != nil {
		return nil, err
	}

	if !currentVersionHistory.ContainsItem(persistence.NewVersionHistoryItemFromInternalType(versionHistoryItem)) {
		e.logger.Warn("current version history and requested one are different - found on checking mutableState",
			tag.Dynamic("current-version-history", currentVersionHistory),
			tag.Dynamic("requested-version-history", versionHistoryItem),
		)
		return nil, &types.CurrentBranchChangedError{
			Message:            "current branch token and request branch token doesn't match",
			CurrentBranchToken: mutableState.CurrentBranchToken,
		}
	}

	if expectedNextEventID < mutableState.GetNextEventID() || !mutableState.GetIsWorkflowRunning() {
		return mutableState, nil
	}

	domainName, err := e.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return nil, err
	}

	expirationInterval := e.shard.GetConfig().LongPollExpirationInterval(domainName)
	if deadline, ok := ctx.Deadline(); ok {
		remainingTime := deadline.Sub(e.shard.GetTimeSource().Now())
		// Here we return a safeguard error, to ensure that older clients are not stuck in long poll loop until context fully expires.
		// Otherwise, it results in multiple additional requests being made that returns empty responses.
		// Newer clients will not make request with too small timeout remaining.
		if remainingTime < longPollCompletionBuffer {
			return nil, context.DeadlineExceeded
		}
		// longPollCompletionBuffer is here to leave some room to finish current request without its timeout.
		expirationInterval = min(
			expirationInterval,
			remainingTime-longPollCompletionBuffer,
		)
	}
	if expirationInterval <= 0 {
		return mutableState, nil
	}
	timer := time.NewTimer(expirationInterval)
	defer timer.Stop()
	for {
		select {
		case event := <-channel:
			mutableState.LastFirstEventID = event.LastFirstEventID
			mutableState.NextEventID = event.NextEventID
			mutableState.IsWorkflowRunning = event.WorkflowCloseState == persistence.WorkflowCloseStatusNone
			mutableState.PreviousStartedEventID = common.Int64Ptr(event.PreviousStartedEventID)
			mutableState.WorkflowState = common.Int32Ptr(int32(event.WorkflowState))
			mutableState.WorkflowCloseState = common.Int32Ptr(int32(event.WorkflowCloseState))

			currentVersionHistoryOnPoll, err := event.VersionHistories.GetCurrentVersionHistory()
			if err != nil {
				return nil, fmt.Errorf("unexpected error getting current version history while polling for mutable state: %w", err)
			}
			if !currentVersionHistoryOnPoll.ContainsItem(persistence.NewVersionHistoryItemFromInternalType(versionHistoryItem)) {
				e.logger.Warn("current version history and requested one are different",
					tag.Dynamic("current-version-history", currentVersionHistoryOnPoll),
					tag.Dynamic("requested-version-history", versionHistoryItem),
				)
				return nil, &types.CurrentBranchChangedError{
					Message:            "current and requested version histories don't match - changed while polling",
					CurrentBranchToken: mutableState.CurrentBranchToken,
				}
			}

			if expectedNextEventID < mutableState.GetNextEventID() || !mutableState.GetIsWorkflowRunning() {
				return mutableState, nil
			}
		case <-timer.C:
			return mutableState, nil
		}
	}
}
