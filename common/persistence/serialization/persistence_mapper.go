// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package serialization

import (
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func ToInternalWorkflowExecutionInfo(info *WorkflowExecutionInfo) *persistence.InternalWorkflowExecutionInfo {
	result := &persistence.InternalWorkflowExecutionInfo{
		CompletionEventBatchID:             constants.EmptyEventID,
		TaskList:                           info.GetTaskList(),
		TaskListKind:                       info.GetTaskListKind(),
		WorkflowTypeName:                   info.GetWorkflowTypeName(),
		WorkflowTimeout:                    info.GetWorkflowTimeout(),
		DecisionStartToCloseTimeout:        info.GetDecisionTaskTimeout(),
		ExecutionContext:                   info.GetExecutionContext(),
		State:                              int(info.GetState()),
		CloseStatus:                        int(info.GetCloseStatus()),
		LastFirstEventID:                   info.GetLastFirstEventID(),
		LastEventTaskID:                    info.GetLastEventTaskID(),
		LastProcessedEvent:                 info.GetLastProcessedEvent(),
		StartTimestamp:                     info.GetStartTimestamp(),
		LastUpdatedTimestamp:               info.GetLastUpdatedTimestamp(),
		CreateRequestID:                    info.GetCreateRequestID(),
		SignalCount:                        int32(info.GetSignalCount()),
		DecisionVersion:                    info.GetDecisionVersion(),
		DecisionScheduleID:                 info.GetDecisionScheduleID(),
		DecisionStartedID:                  info.GetDecisionStartedID(),
		DecisionRequestID:                  info.GetDecisionRequestID(),
		DecisionTimeout:                    info.GetDecisionTimeout(),
		DecisionAttempt:                    info.GetDecisionAttempt(),
		DecisionStartedTimestamp:           info.GetDecisionStartedTimestamp(),
		DecisionScheduledTimestamp:         info.GetDecisionScheduledTimestamp(),
		DecisionOriginalScheduledTimestamp: info.GetDecisionOriginalScheduledTimestamp(),
		StickyTaskList:                     info.GetStickyTaskList(),
		StickyScheduleToStartTimeout:       info.GetStickyScheduleToStartTimeout(),
		ClientLibraryVersion:               info.GetClientLibraryVersion(),
		ClientFeatureVersion:               info.GetClientFeatureVersion(),
		ClientImpl:                         info.GetClientImpl(),
		Attempt:                            int32(info.GetRetryAttempt()),
		HasRetryPolicy:                     info.GetHasRetryPolicy(),
		InitialInterval:                    info.GetRetryInitialInterval(),
		BackoffCoefficient:                 info.GetRetryBackoffCoefficient(),
		MaximumInterval:                    info.GetRetryMaximumInterval(),
		ExpirationTime:                     info.GetRetryExpirationTimestamp(),
		MaximumAttempts:                    info.GetRetryMaximumAttempts(),
		NonRetriableErrors:                 info.GetRetryNonRetryableErrors(),
		BranchToken:                        info.GetEventBranchToken(),
		CronSchedule:                       info.GetCronSchedule(),
		ExpirationInterval:                 info.GetRetryExpiration(),
		Memo:                               info.GetMemo(),
		SearchAttributes:                   info.GetSearchAttributes(),
		HistorySize:                        info.GetHistorySize(),
		FirstExecutionRunID:                info.FirstExecutionRunID.String(),
		PartitionConfig:                    info.PartitionConfig,
		IsCron:                             info.IsCron,
		CronOverlapPolicy:                  types.CronOverlapPolicy(info.GetCronOverlapPolicy()),
	}
	if info.ParentDomainID != nil {
		result.ParentDomainID = info.ParentDomainID.String()
		result.ParentWorkflowID = info.GetParentWorkflowID()
		result.ParentRunID = info.ParentRunID.String()
		result.InitiatedID = info.GetInitiatedID()
	}

	if info.GetCancelRequested() {
		result.CancelRequested = true
		result.CancelRequestID = info.GetCancelRequestID()
	}

	if info.CompletionEventBatchID != nil {
		result.CompletionEventBatchID = info.GetCompletionEventBatchID()
	}

	if info.CompletionEvent != nil {
		result.CompletionEvent = persistence.NewDataBlob(info.CompletionEvent,
			constants.EncodingType(info.GetCompletionEventEncoding()))
	}

	if info.AutoResetPoints != nil {
		result.AutoResetPoints = persistence.NewDataBlob(info.AutoResetPoints,
			constants.EncodingType(info.GetAutoResetPointsEncoding()))
	}

	if info.ActiveClusterSelectionPolicy != nil {
		result.ActiveClusterSelectionPolicy = persistence.NewDataBlob(info.ActiveClusterSelectionPolicy,
			constants.EncodingType(info.GetActiveClusterSelectionPolicyEncoding()))
	}

	return result
}

func FromInternalWorkflowExecutionInfo(executionInfo *persistence.InternalWorkflowExecutionInfo) *WorkflowExecutionInfo {
	info := &WorkflowExecutionInfo{
		TaskList:                             executionInfo.TaskList,
		TaskListKind:                         executionInfo.TaskListKind,
		WorkflowTypeName:                     executionInfo.WorkflowTypeName,
		WorkflowTimeout:                      executionInfo.WorkflowTimeout,
		DecisionTaskTimeout:                  executionInfo.DecisionStartToCloseTimeout,
		ExecutionContext:                     executionInfo.ExecutionContext,
		State:                                int32(executionInfo.State),
		CloseStatus:                          int32(executionInfo.CloseStatus),
		LastFirstEventID:                     executionInfo.LastFirstEventID,
		LastEventTaskID:                      executionInfo.LastEventTaskID,
		LastProcessedEvent:                   executionInfo.LastProcessedEvent,
		StartTimestamp:                       executionInfo.StartTimestamp,
		LastUpdatedTimestamp:                 executionInfo.LastUpdatedTimestamp,
		CreateRequestID:                      executionInfo.CreateRequestID,
		DecisionVersion:                      executionInfo.DecisionVersion,
		DecisionScheduleID:                   executionInfo.DecisionScheduleID,
		DecisionStartedID:                    executionInfo.DecisionStartedID,
		DecisionRequestID:                    executionInfo.DecisionRequestID,
		DecisionTimeout:                      executionInfo.DecisionTimeout,
		DecisionAttempt:                      executionInfo.DecisionAttempt,
		DecisionStartedTimestamp:             executionInfo.DecisionStartedTimestamp,
		DecisionScheduledTimestamp:           executionInfo.DecisionScheduledTimestamp,
		DecisionOriginalScheduledTimestamp:   executionInfo.DecisionOriginalScheduledTimestamp,
		StickyTaskList:                       executionInfo.StickyTaskList,
		StickyScheduleToStartTimeout:         executionInfo.StickyScheduleToStartTimeout,
		ClientLibraryVersion:                 executionInfo.ClientLibraryVersion,
		ClientFeatureVersion:                 executionInfo.ClientFeatureVersion,
		ClientImpl:                           executionInfo.ClientImpl,
		SignalCount:                          int64(executionInfo.SignalCount),
		HistorySize:                          executionInfo.HistorySize,
		CronSchedule:                         executionInfo.CronSchedule,
		CompletionEventBatchID:               &executionInfo.CompletionEventBatchID,
		HasRetryPolicy:                       executionInfo.HasRetryPolicy,
		RetryAttempt:                         int64(executionInfo.Attempt),
		RetryInitialInterval:                 executionInfo.InitialInterval,
		RetryBackoffCoefficient:              executionInfo.BackoffCoefficient,
		RetryMaximumInterval:                 executionInfo.MaximumInterval,
		RetryMaximumAttempts:                 executionInfo.MaximumAttempts,
		RetryExpiration:                      executionInfo.ExpirationInterval,
		RetryExpirationTimestamp:             executionInfo.ExpirationTime,
		RetryNonRetryableErrors:              executionInfo.NonRetriableErrors,
		EventStoreVersion:                    persistence.EventStoreVersion,
		EventBranchToken:                     executionInfo.BranchToken,
		AutoResetPoints:                      executionInfo.AutoResetPoints.GetData(),
		AutoResetPointsEncoding:              string(executionInfo.AutoResetPoints.GetEncoding()),
		SearchAttributes:                     executionInfo.SearchAttributes,
		Memo:                                 executionInfo.Memo,
		CompletionEventEncoding:              string(constants.EncodingTypeEmpty),
		VersionHistoriesEncoding:             string(constants.EncodingTypeEmpty),
		InitiatedID:                          constants.EmptyEventID,
		FirstExecutionRunID:                  MustParseUUID(executionInfo.FirstExecutionRunID),
		PartitionConfig:                      executionInfo.PartitionConfig,
		IsCron:                               executionInfo.IsCron,
		CronOverlapPolicy:                    executionInfo.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:         executionInfo.ActiveClusterSelectionPolicy.GetData(),
		ActiveClusterSelectionPolicyEncoding: string(executionInfo.ActiveClusterSelectionPolicy.GetEncoding()),
	}

	if executionInfo.CompletionEvent != nil {
		info.CompletionEvent = executionInfo.CompletionEvent.Data
		info.CompletionEventEncoding = string(executionInfo.CompletionEvent.Encoding)
	}

	if executionInfo.ParentDomainID != "" {
		info.ParentDomainID = MustParseUUID(executionInfo.ParentDomainID)
		info.ParentWorkflowID = executionInfo.ParentWorkflowID
		info.ParentRunID = MustParseUUID(executionInfo.ParentRunID)
		info.InitiatedID = executionInfo.InitiatedID
	}

	if executionInfo.CancelRequested {
		info.CancelRequested = true
		info.CancelRequestID = executionInfo.CancelRequestID
	}

	if executionInfo.ActiveClusterSelectionPolicy != nil {
		info.ActiveClusterSelectionPolicy = executionInfo.ActiveClusterSelectionPolicy.Data
		info.ActiveClusterSelectionPolicyEncoding = string(executionInfo.ActiveClusterSelectionPolicy.Encoding)
	}

	return info
}
