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

	if err := common.ValidateDomainUUID(request.DomainUUID); err != nil {
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

	executionInfo := mutableState.GetExecutionInfo()

	result := &types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: &types.WorkflowExecutionConfiguration{
			TaskList:                            &types.TaskList{Name: executionInfo.TaskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionInfo.WorkflowTimeout),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(executionInfo.DecisionStartToCloseTimeout),
		},
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: executionInfo.WorkflowID,
				RunID:      executionInfo.RunID,
			},
			TaskList: &types.TaskList{
				Name: executionInfo.TaskList,
				Kind: executionInfo.TaskListKind.Ptr(),
			},
			Type:             &types.WorkflowType{Name: executionInfo.WorkflowTypeName},
			StartTime:        common.Int64Ptr(executionInfo.StartTimestamp.UnixNano()),
			HistoryLength:    mutableState.GetNextEventID() - constants.FirstEventID,
			AutoResetPoints:  executionInfo.AutoResetPoints,
			Memo:             &types.Memo{Fields: executionInfo.CopyMemo()},
			IsCron:           len(executionInfo.CronSchedule) > 0,
			UpdateTime:       common.Int64Ptr(executionInfo.LastUpdatedTimestamp.UnixNano()),
			SearchAttributes: &types.SearchAttributes{IndexedFields: executionInfo.CopySearchAttributes()},
			PartitionConfig:  executionInfo.CopyPartitionConfig(),
		},
	}

	// TODO: we need to consider adding execution time to mutable state
	// For now execution time will be calculated based on start time and cron schedule/retry policy
	// each time DescribeWorkflowExecution is called.
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return nil, err
	}
	backoffDuration := time.Duration(startEvent.GetWorkflowExecutionStartedEventAttributes().GetFirstDecisionTaskBackoffSeconds()) * time.Second
	result.WorkflowExecutionInfo.ExecutionTime = common.Int64Ptr(result.WorkflowExecutionInfo.GetStartTime() + backoffDuration.Nanoseconds())

	if executionInfo.ParentRunID != "" {
		result.WorkflowExecutionInfo.ParentExecution = &types.WorkflowExecution{
			WorkflowID: executionInfo.ParentWorkflowID,
			RunID:      executionInfo.ParentRunID,
		}
		result.WorkflowExecutionInfo.ParentDomainID = common.StringPtr(executionInfo.ParentDomainID)
		result.WorkflowExecutionInfo.ParentInitiatedID = common.Int64Ptr(executionInfo.InitiatedID)
		parentDomain, err := e.shard.GetDomainCache().GetDomainName(executionInfo.ParentDomainID)
		if err != nil {
			return nil, err
		}
		result.WorkflowExecutionInfo.ParentDomain = common.StringPtr(parentDomain)
	}
	if executionInfo.State == persistence.WorkflowStateCompleted {
		// for closed workflow
		result.WorkflowExecutionInfo.CloseStatus = persistence.ToInternalWorkflowExecutionCloseStatus(executionInfo.CloseStatus)
		completionEvent, err := mutableState.GetCompletionEvent(ctx)
		if err != nil {
			return nil, err
		}
		result.WorkflowExecutionInfo.CloseTime = common.Int64Ptr(completionEvent.GetTimestamp())
	}

	if len(mutableState.GetPendingActivityInfos()) > 0 {
		for _, ai := range mutableState.GetPendingActivityInfos() {
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
			// TODO: move to mutable state instead of loading it from event
			scheduledEvent, err := mutableState.GetActivityScheduledEvent(ctx, ai.ScheduleID)
			if err != nil {
				return nil, err
			}
			p.ActivityType = scheduledEvent.ActivityTaskScheduledEventAttributes.ActivityType
			if state == types.PendingActivityStateScheduled {
				p.ScheduledTimestamp = common.Int64Ptr(ai.ScheduledTime.UnixNano())
			} else {
				p.LastStartedTimestamp = common.Int64Ptr(ai.StartedTime.UnixNano())
			}
			if ai.HasRetryPolicy {
				p.Attempt = ai.Attempt
				p.ExpirationTimestamp = common.Int64Ptr(ai.ExpirationTime.UnixNano())
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
			result.PendingActivities = append(result.PendingActivities, p)
		}
	}

	if len(mutableState.GetPendingChildExecutionInfos()) > 0 {
		for _, ch := range mutableState.GetPendingChildExecutionInfos() {
			childDomainName, err := execution.GetChildExecutionDomainName(
				ch,
				e.shard.GetDomainCache(),
				mutableState.GetDomainEntry(),
			)
			if err != nil {
				if !common.IsEntityNotExistsError(err) {
					return nil, err
				}
				// child domain already deleted, instead of failing the request,
				// return domainID instead since this field is only for information purpose
				childDomainName = ch.DomainID
			}
			p := &types.PendingChildExecutionInfo{
				Domain:            childDomainName,
				WorkflowID:        ch.StartedWorkflowID,
				RunID:             ch.StartedRunID,
				WorkflowTypeName:  ch.WorkflowTypeName,
				InitiatedID:       ch.InitiatedID,
				ParentClosePolicy: &ch.ParentClosePolicy,
			}
			result.PendingChildren = append(result.PendingChildren, p)
		}
	}

	if di, ok := mutableState.GetPendingDecision(); ok {
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
		result.PendingDecision = pendingDecision
	}

	return result, nil
}
