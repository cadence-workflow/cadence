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
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

// DescribeWorkflowExecution returns information about the specified workflow execution.
func (e *historyEngineImpl) DescribeWorkflowExecution(
	ctx context.Context,
	request *types.HistoryDescribeWorkflowExecutionRequest,
) (retResp *types.DescribeWorkflowExecutionResponse, retError error) {
	if err := validateDescribeWorkflowExecutionRequest(request); err != nil {
		return nil, err
	}

	domainID := request.DomainUUID
	wfExecution := *request.Request.Execution

	wfContext, release, err0 := e.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, wfExecution)
	if err0 != nil {
		return nil, err0
	}
	defer func() { release(retError) }()

	mutableState, err1 := wfContext.LoadWorkflowExecution(ctx)
	if err1 != nil {
		return nil, err1
	}

	// If history is corrupted, return an error to the end user
	if corrupted, err := e.checkForHistoryCorruptions(ctx, mutableState); err != nil {
		return nil, err
	} else if corrupted {
		return nil, &types.EntityNotExistsError{Message: "Workflow execution corrupted."}
	}

	domainCache := e.shard.GetDomainCache()
	result, err := createDescribeWorkflowExecutionResponse(ctx, mutableState, domainCache)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// validateDescribeWorkflowExecutionRequest validates the input request
func validateDescribeWorkflowExecutionRequest(request *types.HistoryDescribeWorkflowExecutionRequest) error {
	return common.ValidateDomainUUID(request.DomainUUID)
}

func createDescribeWorkflowExecutionResponse(ctx context.Context, mutableState execution.MutableState, domainCache cache.DomainCache) (*types.DescribeWorkflowExecutionResponse, error) {
	executionInfo := mutableState.GetExecutionInfo()
	executionConfiguration, err := mapWorkflowExecutionConfiguration(executionInfo)
	if err != nil {
		return nil, err
	}

	result := &types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: executionConfiguration,
	}

	// TODO: we need to consider adding execution time to mutable state
	// For now execution time will be calculated based on start time and cron schedule/retry policy
	// each time DescribeWorkflowExecution is called.
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return nil, err
	}

	var completionEvent *types.HistoryEvent
	if executionInfo.State == persistence.WorkflowStateCompleted {
		completionEvent, err = mutableState.GetCompletionEvent(ctx)
		if err != nil {
			return nil, err
		}
	}

	historyLength := mutableState.GetNextEventID() - constants.FirstEventID
	workflowExecutionInfo, err := mapWorkflowExecutionInfo(executionInfo, startEvent, domainCache, historyLength, completionEvent)
	if err != nil {
		return nil, err
	}
	result.WorkflowExecutionInfo = workflowExecutionInfo

	pendingActivityInfos := mutableState.GetPendingActivityInfos()
	for _, ai := range pendingActivityInfos {
		scheduledEvent, err := mutableState.GetActivityScheduledEvent(ctx, ai.ScheduleID)
		if err != nil {
			return nil, err
		}

		p := mapPendingActivityInfo(ai, scheduledEvent)
		result.PendingActivities = append(result.PendingActivities, p)
	}

	childExecutions := mutableState.GetPendingChildExecutionInfos()
	domainEntry := mutableState.GetDomainEntry()
	for _, childExecution := range childExecutions {
		pendingChild, err := mapPendingChildExecutionInfo(childExecution, domainEntry, domainCache)
		if err != nil {
			return nil, err
		}
		result.PendingChildren = append(result.PendingChildren, pendingChild)
	}

	if di, ok := mutableState.GetPendingDecision(); ok {
		result.PendingDecision = mapPendingDecisionInfo(di)
	}

	return result, nil
}

func mapWorkflowExecutionConfiguration(executionInfo *persistence.WorkflowExecutionInfo) (*types.WorkflowExecutionConfiguration, error) {
	return &types.WorkflowExecutionConfiguration{
		TaskList:                            mapDecisionInfoToTaskList(executionInfo),
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionInfo.WorkflowTimeout),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(executionInfo.DecisionStartToCloseTimeout),
	}, nil
}

func mapWorkflowExecutionInfo(executionInfo *persistence.WorkflowExecutionInfo, startEvent *types.HistoryEvent, domainCache cache.DomainCache, historyLength int64, completionEvent *types.HistoryEvent) (*types.WorkflowExecutionInfo, error) {
	result := &types.WorkflowExecutionInfo{
		Execution: &types.WorkflowExecution{
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskList:                     mapDecisionInfoToTaskList(executionInfo),
		Type:                         &types.WorkflowType{Name: executionInfo.WorkflowTypeName},
		StartTime:                    common.Int64Ptr(executionInfo.StartTimestamp.UnixNano()),
		HistoryLength:                historyLength,
		AutoResetPoints:              executionInfo.AutoResetPoints,
		Memo:                         &types.Memo{Fields: executionInfo.CopyMemo()},
		IsCron:                       len(executionInfo.CronSchedule) > 0,
		UpdateTime:                   common.Int64Ptr(executionInfo.LastUpdatedTimestamp.UnixNano()),
		SearchAttributes:             &types.SearchAttributes{IndexedFields: executionInfo.CopySearchAttributes()},
		PartitionConfig:              executionInfo.CopyPartitionConfig(),
		CronOverlapPolicy:            &executionInfo.CronOverlapPolicy,
		ActiveClusterSelectionPolicy: executionInfo.ActiveClusterSelectionPolicy,
	}

	backoffDuration := time.Duration(startEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstDecisionTaskBackoffSeconds()) * time.Second
	result.ExecutionTime = common.Int64Ptr(result.GetStartTime() + backoffDuration.Nanoseconds())

	if executionInfo.ParentRunID != "" {
		result.ParentExecution = &types.WorkflowExecution{
			WorkflowID: executionInfo.ParentWorkflowID,
			RunID:      executionInfo.ParentRunID,
		}
		result.ParentDomainID = common.StringPtr(executionInfo.ParentDomainID)
		result.ParentInitiatedID = common.Int64Ptr(executionInfo.InitiatedID)
		parentDomain, err := domainCache.GetDomainName(executionInfo.ParentDomainID)
		if err != nil {
			return nil, err
		}
		result.ParentDomain = common.StringPtr(parentDomain)
	}

	if executionInfo.State == persistence.WorkflowStateCompleted {
		result.CloseStatus = persistence.ToInternalWorkflowExecutionCloseStatus(executionInfo.CloseStatus)
		if completionEvent != nil {
			result.CloseTime = common.Int64Ptr(completionEvent.GetTimestamp())
		}
	}

	return result, nil
}

func mapPendingActivityInfo(ai *persistence.ActivityInfo, activityScheduledEvent *types.HistoryEvent) *types.PendingActivityInfo {
	p := &types.PendingActivityInfo{
		ActivityID: ai.ActivityID,
		ScheduleID: ai.ScheduleID,
	}

	state := types.PendingActivityStateScheduled
	if ai.CancelRequested {
		state = types.PendingActivityStateCancelRequested
	} else if ai.StartedID != constants.EmptyEventID {
		state = types.PendingActivityStateStarted
	}
	p.State = &state

	lastHeartbeatUnixNano := ai.LastHeartBeatUpdatedTime.UnixNano()
	if lastHeartbeatUnixNano > 0 {
		p.LastHeartbeatTimestamp = common.Int64Ptr(lastHeartbeatUnixNano)
		p.HeartbeatDetails = ai.Details
	}

	p.ActivityType = activityScheduledEvent.ActivityTaskScheduledEventAttributes.ActivityType
	if state == types.PendingActivityStateScheduled {
		p.ScheduledTimestamp = common.Int64Ptr(ai.ScheduledTime.UnixNano())
	} else {
		p.LastStartedTimestamp = common.Int64Ptr(ai.StartedTime.UnixNano())
	}

	if ai.HasRetryPolicy {
		p.Attempt = ai.Attempt
		if !ai.ExpirationTime.IsZero() {
			p.ExpirationTimestamp = common.Int64Ptr(ai.ExpirationTime.UnixNano())
		}
		if ai.MaximumAttempts != 0 {
			p.MaximumAttempts = ai.MaximumAttempts
		}
		if ai.LastFailureReason != "" {
			p.LastFailureReason = common.StringPtr(ai.LastFailureReason)
			p.LastFailureDetails = ai.LastFailureDetails
		}
		if ai.LastWorkerIdentity != "" {
			p.LastWorkerIdentity = ai.LastWorkerIdentity
		}
		if ai.StartedIdentity != "" {
			p.StartedWorkerIdentity = ai.StartedIdentity
		}
	}

	return p
}

func mapPendingChildExecutionInfo(childExecution *persistence.ChildExecutionInfo, domainEntry *cache.DomainCacheEntry, domainCache cache.DomainCache) (*types.PendingChildExecutionInfo, error) {
	childDomainName, err := execution.GetChildExecutionDomainName(
		childExecution,
		domainCache,
		domainEntry,
	)
	if err != nil {
		if !common.IsEntityNotExistsError(err) {
			return nil, err
		}
		// child domain already deleted, instead of failing the request,
		// return domainID instead since this field is only for information purpose
		childDomainName = childExecution.DomainID
	}
	return &types.PendingChildExecutionInfo{
		Domain:            childDomainName,
		WorkflowID:        childExecution.StartedWorkflowID,
		RunID:             childExecution.StartedRunID,
		WorkflowTypeName:  childExecution.WorkflowTypeName,
		InitiatedID:       childExecution.InitiatedID,
		ParentClosePolicy: &childExecution.ParentClosePolicy,
	}, nil
}

func mapPendingDecisionInfo(di *execution.DecisionInfo) *types.PendingDecisionInfo {
	pendingDecision := &types.PendingDecisionInfo{
		State:                      types.PendingDecisionStateScheduled.Ptr(),
		ScheduledTimestamp:         common.Int64Ptr(di.ScheduledTimestamp),
		Attempt:                    di.Attempt,
		OriginalScheduledTimestamp: common.Int64Ptr(di.OriginalScheduledTimestamp),
		ScheduleID:                 di.ScheduleID,
	}
	if di.StartedID != constants.EmptyEventID {
		pendingDecision.State = types.PendingDecisionStateStarted.Ptr()
		pendingDecision.StartedTimestamp = common.Int64Ptr(di.StartedTimestamp)
	}
	return pendingDecision
}

func mapDecisionInfoToTaskList(executionInfo *persistence.WorkflowExecutionInfo) *types.TaskList {
	return &types.TaskList{
		Name: executionInfo.TaskList,
		Kind: executionInfo.TaskListKind.Ptr(),
	}
}
