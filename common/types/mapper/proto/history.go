// Copyright (c) 2021 Uber Technologies Inc.
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

package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	historyv1 "github.com/uber/cadence/.gen/proto/history/v1"
	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func FromHistoryCloseShardRequest(t *types.CloseShardRequest) *historyv1.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &historyv1.CloseShardRequest{
		ShardId: t.ShardID,
	}
}

func ToHistoryCloseShardRequest(t *historyv1.CloseShardRequest) *types.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &types.CloseShardRequest{
		ShardID: t.ShardId,
	}
}

func FromHistoryDescribeHistoryHostRequest(t *types.DescribeHistoryHostRequest) *historyv1.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeHistoryHostRequest{}
}

func ToHistoryDescribeHistoryHostRequest(t *historyv1.DescribeHistoryHostRequest) *types.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeHistoryHostRequest{}
}

func FromHistoryDescribeHistoryHostResponse(t *types.DescribeHistoryHostResponse) *historyv1.DescribeHistoryHostResponse {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeHistoryHostResponse{
		NumberOfShards:        t.NumberOfShards,
		ShardIds:              t.ShardIDs,
		DomainCache:           FromDomainCacheInfo(t.DomainCache),
		ShardControllerStatus: t.ShardControllerStatus,
		Address:               t.Address,
	}
}

func ToHistoryDescribeHistoryHostResponse(t *historyv1.DescribeHistoryHostResponse) *types.DescribeHistoryHostResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeHistoryHostResponse{
		NumberOfShards:        t.NumberOfShards,
		ShardIDs:              t.ShardIds,
		DomainCache:           ToDomainCacheInfo(t.DomainCache),
		ShardControllerStatus: t.ShardControllerStatus,
		Address:               t.Address,
	}
}

func FromHistoryDescribeMutableStateRequest(t *types.DescribeMutableStateRequest) *historyv1.DescribeMutableStateRequest {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeMutableStateRequest{
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func ToHistoryDescribeMutableStateRequest(t *historyv1.DescribeMutableStateRequest) *types.DescribeMutableStateRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeMutableStateRequest{
		DomainUUID: t.DomainId,
		Execution:  ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromHistoryDescribeMutableStateResponse(t *types.DescribeMutableStateResponse) *historyv1.DescribeMutableStateResponse {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeMutableStateResponse{
		MutableStateInCache:    t.MutableStateInCache,
		MutableStateInDatabase: t.MutableStateInDatabase,
	}
}

func ToHistoryDescribeMutableStateResponse(t *historyv1.DescribeMutableStateResponse) *types.DescribeMutableStateResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeMutableStateResponse{
		MutableStateInCache:    t.MutableStateInCache,
		MutableStateInDatabase: t.MutableStateInDatabase,
	}
}

func FromHistoryDescribeQueueRequest(t *types.DescribeQueueRequest) *historyv1.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeQueueRequest{
		ShardId:     t.ShardID,
		ClusterName: t.ClusterName,
		TaskType:    FromTaskType(t.Type),
	}
}

func ToHistoryDescribeQueueRequest(t *historyv1.DescribeQueueRequest) *types.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueRequest{
		ShardID:     t.ShardId,
		ClusterName: t.ClusterName,
		Type:        ToTaskType(t.TaskType),
	}
}

func FromHistoryDescribeQueueResponse(t *types.DescribeQueueResponse) *historyv1.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

func ToHistoryDescribeQueueResponse(t *historyv1.DescribeQueueResponse) *types.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

func FromHistoryDescribeWorkflowExecutionRequest(t *types.HistoryDescribeWorkflowExecutionRequest) *historyv1.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeWorkflowExecutionRequest{
		Request:  FromDescribeWorkflowExecutionRequest(t.Request),
		DomainId: t.DomainUUID,
	}
}

func ToHistoryDescribeWorkflowExecutionRequest(t *historyv1.DescribeWorkflowExecutionRequest) *types.HistoryDescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryDescribeWorkflowExecutionRequest{
		Request:    ToDescribeWorkflowExecutionRequest(t.Request),
		DomainUUID: t.DomainId,
	}
}

func FromHistoryDescribeWorkflowExecutionResponse(t *types.DescribeWorkflowExecutionResponse) *historyv1.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &historyv1.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: FromWorkflowExecutionConfiguration(t.ExecutionConfiguration),
		WorkflowExecutionInfo:  FromWorkflowExecutionInfo(t.WorkflowExecutionInfo),
		PendingActivities:      FromPendingActivityInfoArray(t.PendingActivities),
		PendingChildren:        FromPendingChildExecutionInfoArray(t.PendingChildren),
		PendingDecision:        FromPendingDecisionInfo(t.PendingDecision),
	}
}

func ToHistoryDescribeWorkflowExecutionResponse(t *historyv1.DescribeWorkflowExecutionResponse) *types.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: ToWorkflowExecutionConfiguration(t.ExecutionConfiguration),
		WorkflowExecutionInfo:  ToWorkflowExecutionInfo(t.WorkflowExecutionInfo),
		PendingActivities:      ToPendingActivityInfoArray(t.PendingActivities),
		PendingChildren:        ToPendingChildExecutionInfoArray(t.PendingChildren),
		PendingDecision:        ToPendingDecisionInfo(t.PendingDecision),
	}
}

func FromHistoryGetDLQReplicationMessagesRequest(t *types.GetDLQReplicationMessagesRequest) *historyv1.GetDLQReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.GetDLQReplicationMessagesRequest{
		TaskInfos: FromReplicationTaskInfoArray(t.TaskInfos),
	}
}

func ToHistoryGetDLQReplicationMessagesRequest(t *historyv1.GetDLQReplicationMessagesRequest) *types.GetDLQReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.GetDLQReplicationMessagesRequest{
		TaskInfos: ToReplicationTaskInfoArray(t.TaskInfos),
	}
}

func FromHistoryGetDLQReplicationMessagesResponse(t *types.GetDLQReplicationMessagesResponse) *historyv1.GetDLQReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &historyv1.GetDLQReplicationMessagesResponse{
		ReplicationTasks: FromReplicationTaskArray(t.ReplicationTasks),
	}
}

func ToHistoryGetDLQReplicationMessagesResponse(t *historyv1.GetDLQReplicationMessagesResponse) *types.GetDLQReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.GetDLQReplicationMessagesResponse{
		ReplicationTasks: ToReplicationTaskArray(t.ReplicationTasks),
	}
}

func FromHistoryGetFailoverInfoRequest(t *types.GetFailoverInfoRequest) *historyv1.GetFailoverInfoRequest {
	if t == nil {
		return nil
	}
	return &historyv1.GetFailoverInfoRequest{
		DomainId: t.GetDomainID(),
	}
}

func ToHistoryGetFailoverInfoRequest(t *historyv1.GetFailoverInfoRequest) *types.GetFailoverInfoRequest {
	if t == nil {
		return nil
	}
	return &types.GetFailoverInfoRequest{
		DomainID: t.GetDomainId(),
	}
}

func FromHistoryGetFailoverInfoResponse(t *types.GetFailoverInfoResponse) *historyv1.GetFailoverInfoResponse {
	if t == nil {
		return nil
	}
	return &historyv1.GetFailoverInfoResponse{
		CompletedShardCount: t.GetCompletedShardCount(),
		PendingShards:       t.GetPendingShards(),
	}
}

func ToHistoryGetFailoverInfoResponse(t *historyv1.GetFailoverInfoResponse) *types.GetFailoverInfoResponse {
	if t == nil {
		return nil
	}
	return &types.GetFailoverInfoResponse{
		CompletedShardCount: t.GetCompletedShardCount(),
		PendingShards:       t.GetPendingShards(),
	}
}

func FromHistoryGetMutableStateRequest(t *types.GetMutableStateRequest) *historyv1.GetMutableStateRequest {
	if t == nil {
		return nil
	}
	return &historyv1.GetMutableStateRequest{
		DomainId:            t.DomainUUID,
		WorkflowExecution:   FromWorkflowExecution(t.Execution),
		ExpectedNextEventId: t.ExpectedNextEventID,
		CurrentBranchToken:  t.CurrentBranchToken,
		VersionHistoryItem:  FromVersionHistoryItem(t.VersionHistoryItem),
	}
}

func ToHistoryGetMutableStateRequest(t *historyv1.GetMutableStateRequest) *types.GetMutableStateRequest {
	if t == nil {
		return nil
	}
	return &types.GetMutableStateRequest{
		DomainUUID:          t.DomainId,
		Execution:           ToWorkflowExecution(t.WorkflowExecution),
		ExpectedNextEventID: t.ExpectedNextEventId,
		CurrentBranchToken:  t.CurrentBranchToken,
		VersionHistoryItem:  ToVersionHistoryItem(t.VersionHistoryItem),
	}
}

func FromHistoryGetMutableStateResponse(t *types.GetMutableStateResponse) *historyv1.GetMutableStateResponse {
	if t == nil {
		return nil
	}
	var workflowCloseState *types.WorkflowExecutionCloseStatus
	if t.WorkflowCloseState != nil {
		workflowCloseState = persistence.ToInternalWorkflowExecutionCloseStatus(int(*t.WorkflowCloseState))
	}
	return &historyv1.GetMutableStateResponse{
		WorkflowExecution:                    FromWorkflowExecution(t.Execution),
		WorkflowType:                         FromWorkflowType(t.WorkflowType),
		NextEventId:                          t.NextEventID,
		PreviousStartedEventId:               fromInt64Value(t.PreviousStartedEventID),
		LastFirstEventId:                     t.LastFirstEventID,
		TaskList:                             FromTaskList(t.TaskList),
		StickyTaskList:                       FromTaskList(t.StickyTaskList),
		ClientLibraryVersion:                 t.ClientLibraryVersion,
		ClientFeatureVersion:                 t.ClientFeatureVersion,
		ClientImpl:                           t.ClientImpl,
		StickyTaskListScheduleToStartTimeout: secondsToDuration(t.StickyTaskListScheduleToStartTimeout),
		EventStoreVersion:                    t.EventStoreVersion,
		CurrentBranchToken:                   t.CurrentBranchToken,
		WorkflowState:                        FromWorkflowState(t.WorkflowState),
		WorkflowCloseState:                   FromWorkflowExecutionCloseStatus(workflowCloseState),
		VersionHistories:                     FromVersionHistories(t.VersionHistories),
		IsStickyTaskListEnabled:              t.IsStickyTaskListEnabled,
		HistorySize:                          t.HistorySize,
		StorageLocation:                      int64(t.StorageLocation),
	}
}

func ToHistoryGetMutableStateResponse(t *historyv1.GetMutableStateResponse) *types.GetMutableStateResponse {
	if t == nil {
		return nil
	}
	return &types.GetMutableStateResponse{
		Execution:                            ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:                         ToWorkflowType(t.WorkflowType),
		NextEventID:                          t.NextEventId,
		PreviousStartedEventID:               toInt64Value(t.PreviousStartedEventId),
		LastFirstEventID:                     t.LastFirstEventId,
		TaskList:                             ToTaskList(t.TaskList),
		StickyTaskList:                       ToTaskList(t.StickyTaskList),
		ClientLibraryVersion:                 t.ClientLibraryVersion,
		ClientFeatureVersion:                 t.ClientFeatureVersion,
		ClientImpl:                           t.ClientImpl,
		StickyTaskListScheduleToStartTimeout: durationToSeconds(t.StickyTaskListScheduleToStartTimeout),
		EventStoreVersion:                    t.EventStoreVersion,
		CurrentBranchToken:                   t.CurrentBranchToken,
		WorkflowState:                        ToWorkflowState(t.WorkflowState),
		WorkflowCloseState:                   common.Int32Ptr(int32(persistence.FromInternalWorkflowExecutionCloseStatus(ToWorkflowExecutionCloseStatus(t.WorkflowCloseState)))),
		VersionHistories:                     ToVersionHistories(t.VersionHistories),
		IsStickyTaskListEnabled:              t.IsStickyTaskListEnabled,
		IsWorkflowRunning:                    t.WorkflowState == sharedv1.WorkflowState_WORKFLOW_STATE_RUNNING,
		HistorySize:                          t.HistorySize,
		StorageLocation:                      types.StorageLocation(t.StorageLocation),
	}
}

func FromHistoryGetReplicationMessagesRequest(t *types.GetReplicationMessagesRequest) *historyv1.GetReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.GetReplicationMessagesRequest{
		Tokens:      FromReplicationTokenArray(t.Tokens),
		ClusterName: t.ClusterName,
	}
}

func ToHistoryGetReplicationMessagesRequest(t *historyv1.GetReplicationMessagesRequest) *types.GetReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.GetReplicationMessagesRequest{
		Tokens:      ToReplicationTokenArray(t.Tokens),
		ClusterName: t.ClusterName,
	}
}

func FromHistoryGetReplicationMessagesResponse(t *types.GetReplicationMessagesResponse) *historyv1.GetReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &historyv1.GetReplicationMessagesResponse{
		ShardMessages: FromReplicationMessagesMap(t.MessagesByShard),
	}
}

func ToHistoryGetReplicationMessagesResponse(t *historyv1.GetReplicationMessagesResponse) *types.GetReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.GetReplicationMessagesResponse{
		MessagesByShard: ToReplicationMessagesMap(t.ShardMessages),
	}
}

func FromHistoryCountDLQMessagesRequest(t *types.CountDLQMessagesRequest) *historyv1.CountDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.CountDLQMessagesRequest{
		ForceFetch: t.ForceFetch,
	}
}

func ToHistoryCountDLQMessagesRequest(t *historyv1.CountDLQMessagesRequest) *types.CountDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.CountDLQMessagesRequest{
		ForceFetch: t.ForceFetch,
	}
}

func FromHistoryCountDLQMessagesResponse(t *types.HistoryCountDLQMessagesResponse) *historyv1.CountDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &historyv1.CountDLQMessagesResponse{
		Entries: FromHistoryDLQCountEntryMap(t.Entries),
	}
}

func ToHistoryCountDLQMessagesResponse(t *historyv1.CountDLQMessagesResponse) *types.HistoryCountDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.HistoryCountDLQMessagesResponse{
		Entries: ToHistoryDLQCountEntryMap(t.Entries),
	}
}

func FromHistoryMergeDLQMessagesRequest(t *types.MergeDLQMessagesRequest) *historyv1.MergeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.MergeDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
		PageSize:              t.MaximumPageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func ToHistoryMergeDLQMessagesRequest(t *historyv1.MergeDLQMessagesRequest) *types.MergeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.MergeDLQMessagesRequest{
		Type:                  ToDLQType(t.Type),
		ShardID:               t.ShardId,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageID: toInt64Value(t.InclusiveEndMessageId),
		MaximumPageSize:       t.PageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func FromHistoryMergeDLQMessagesResponse(t *types.MergeDLQMessagesResponse) *historyv1.MergeDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &historyv1.MergeDLQMessagesResponse{
		NextPageToken: t.NextPageToken,
	}
}

func ToHistoryMergeDLQMessagesResponse(t *historyv1.MergeDLQMessagesResponse) *types.MergeDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.MergeDLQMessagesResponse{
		NextPageToken: t.NextPageToken,
	}
}

func FromHistoryNotifyFailoverMarkersRequest(t *types.NotifyFailoverMarkersRequest) *historyv1.NotifyFailoverMarkersRequest {
	if t == nil {
		return nil
	}
	return &historyv1.NotifyFailoverMarkersRequest{
		FailoverMarkerTokens: FromFailoverMarkerTokenArray(t.FailoverMarkerTokens),
	}
}

func ToHistoryNotifyFailoverMarkersRequest(t *historyv1.NotifyFailoverMarkersRequest) *types.NotifyFailoverMarkersRequest {
	if t == nil {
		return nil
	}
	return &types.NotifyFailoverMarkersRequest{
		FailoverMarkerTokens: ToFailoverMarkerTokenArray(t.FailoverMarkerTokens),
	}
}

func FromHistoryPollMutableStateRequest(t *types.PollMutableStateRequest) *historyv1.PollMutableStateRequest {
	if t == nil {
		return nil
	}
	return &historyv1.PollMutableStateRequest{
		DomainId:            t.DomainUUID,
		WorkflowExecution:   FromWorkflowExecution(t.Execution),
		ExpectedNextEventId: t.ExpectedNextEventID,
		CurrentBranchToken:  t.CurrentBranchToken,
	}
}

func ToHistoryPollMutableStateRequest(t *historyv1.PollMutableStateRequest) *types.PollMutableStateRequest {
	if t == nil {
		return nil
	}
	return &types.PollMutableStateRequest{
		DomainUUID:          t.DomainId,
		Execution:           ToWorkflowExecution(t.WorkflowExecution),
		ExpectedNextEventID: t.ExpectedNextEventId,
		CurrentBranchToken:  t.CurrentBranchToken,
	}
}

func FromHistoryPollMutableStateResponse(t *types.PollMutableStateResponse) *historyv1.PollMutableStateResponse {
	if t == nil {
		return nil
	}

	var workflowCloseState *types.WorkflowExecutionCloseStatus
	if t.WorkflowCloseState != nil {
		workflowCloseState = persistence.ToInternalWorkflowExecutionCloseStatus(int(*t.WorkflowCloseState))
	}
	return &historyv1.PollMutableStateResponse{
		WorkflowExecution:                    FromWorkflowExecution(t.Execution),
		WorkflowType:                         FromWorkflowType(t.WorkflowType),
		NextEventId:                          t.NextEventID,
		PreviousStartedEventId:               fromInt64Value(t.PreviousStartedEventID),
		LastFirstEventId:                     t.LastFirstEventID,
		TaskList:                             FromTaskList(t.TaskList),
		StickyTaskList:                       FromTaskList(t.StickyTaskList),
		ClientLibraryVersion:                 t.ClientLibraryVersion,
		ClientFeatureVersion:                 t.ClientFeatureVersion,
		ClientImpl:                           t.ClientImpl,
		StickyTaskListScheduleToStartTimeout: secondsToDuration(t.StickyTaskListScheduleToStartTimeout),
		CurrentBranchToken:                   t.CurrentBranchToken,
		VersionHistories:                     FromVersionHistories(t.VersionHistories),
		WorkflowState:                        FromWorkflowState(t.WorkflowState),
		WorkflowCloseState:                   FromWorkflowExecutionCloseStatus(workflowCloseState),
	}
}

func ToHistoryPollMutableStateResponse(t *historyv1.PollMutableStateResponse) *types.PollMutableStateResponse {
	if t == nil {
		return nil
	}

	return &types.PollMutableStateResponse{
		Execution:                            ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:                         ToWorkflowType(t.WorkflowType),
		NextEventID:                          t.NextEventId,
		PreviousStartedEventID:               toInt64Value(t.PreviousStartedEventId),
		LastFirstEventID:                     t.LastFirstEventId,
		TaskList:                             ToTaskList(t.TaskList),
		StickyTaskList:                       ToTaskList(t.StickyTaskList),
		ClientLibraryVersion:                 t.ClientLibraryVersion,
		ClientFeatureVersion:                 t.ClientFeatureVersion,
		ClientImpl:                           t.ClientImpl,
		StickyTaskListScheduleToStartTimeout: durationToSeconds(t.StickyTaskListScheduleToStartTimeout),
		CurrentBranchToken:                   t.CurrentBranchToken,
		VersionHistories:                     ToVersionHistories(t.VersionHistories),
		WorkflowState:                        ToWorkflowState(t.WorkflowState),
		WorkflowCloseState:                   common.Int32Ptr(int32(persistence.FromInternalWorkflowExecutionCloseStatus(ToWorkflowExecutionCloseStatus(t.WorkflowCloseState)))),
	}
}

func FromHistoryPurgeDLQMessagesRequest(t *types.PurgeDLQMessagesRequest) *historyv1.PurgeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.PurgeDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
	}
}

func ToHistoryPurgeDLQMessagesRequest(t *historyv1.PurgeDLQMessagesRequest) *types.PurgeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.PurgeDLQMessagesRequest{
		Type:                  ToDLQType(t.Type),
		ShardID:               t.ShardId,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageID: toInt64Value(t.InclusiveEndMessageId),
	}
}

func FromHistoryQueryWorkflowRequest(t *types.HistoryQueryWorkflowRequest) *historyv1.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &historyv1.QueryWorkflowRequest{
		Request:  FromQueryWorkflowRequest(t.Request),
		DomainId: t.DomainUUID,
	}
}

func ToHistoryQueryWorkflowRequest(t *historyv1.QueryWorkflowRequest) *types.HistoryQueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryQueryWorkflowRequest{
		Request:    ToQueryWorkflowRequest(t.Request),
		DomainUUID: t.DomainId,
	}
}

func FromHistoryQueryWorkflowResponse(t *types.HistoryQueryWorkflowResponse) *historyv1.QueryWorkflowResponse {
	if t == nil || t.Response == nil {
		return nil
	}
	return &historyv1.QueryWorkflowResponse{
		QueryResult:   FromPayload(t.Response.QueryResult),
		QueryRejected: FromQueryRejected(t.Response.QueryRejected),
	}
}

func ToHistoryQueryWorkflowResponse(t *historyv1.QueryWorkflowResponse) *types.HistoryQueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.HistoryQueryWorkflowResponse{
		Response: &types.QueryWorkflowResponse{
			QueryResult:   ToPayload(t.QueryResult),
			QueryRejected: ToQueryRejected(t.QueryRejected),
		},
	}
}

func FromHistoryReadDLQMessagesRequest(t *types.ReadDLQMessagesRequest) *historyv1.ReadDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &historyv1.ReadDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
		PageSize:              t.MaximumPageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func ToHistoryReadDLQMessagesRequest(t *historyv1.ReadDLQMessagesRequest) *types.ReadDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.ReadDLQMessagesRequest{
		Type:                  ToDLQType(t.Type),
		ShardID:               t.ShardId,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageID: toInt64Value(t.InclusiveEndMessageId),
		MaximumPageSize:       t.PageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func FromHistoryReadDLQMessagesResponse(t *types.ReadDLQMessagesResponse) *historyv1.ReadDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &historyv1.ReadDLQMessagesResponse{
		Type:                 FromDLQType(t.Type),
		ReplicationTasks:     FromReplicationTaskArray(t.ReplicationTasks),
		ReplicationTasksInfo: FromReplicationTaskInfoArray(t.ReplicationTasksInfo),
		NextPageToken:        t.NextPageToken,
	}
}

func ToHistoryReadDLQMessagesResponse(t *historyv1.ReadDLQMessagesResponse) *types.ReadDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.ReadDLQMessagesResponse{
		Type:                 ToDLQType(t.Type),
		ReplicationTasks:     ToReplicationTaskArray(t.ReplicationTasks),
		ReplicationTasksInfo: ToReplicationTaskInfoArray(t.ReplicationTasksInfo),
		NextPageToken:        t.NextPageToken,
	}
}

func FromHistoryReapplyEventsRequest(t *types.HistoryReapplyEventsRequest) *historyv1.ReapplyEventsRequest {
	if t == nil || t.Request == nil {
		return nil
	}
	return &historyv1.ReapplyEventsRequest{
		Domain:            t.Request.DomainName,
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.Request.WorkflowExecution),
		Events:            FromDataBlob(t.Request.Events),
	}
}

func ToHistoryReapplyEventsRequest(t *historyv1.ReapplyEventsRequest) *types.HistoryReapplyEventsRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryReapplyEventsRequest{
		DomainUUID: t.DomainId,
		Request: &types.ReapplyEventsRequest{
			DomainName:        t.Domain,
			WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
			Events:            ToDataBlob(t.Events),
		},
	}
}

func FromHistoryRecordActivityTaskHeartbeatRequest(t *types.HistoryRecordActivityTaskHeartbeatRequest) *historyv1.RecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RecordActivityTaskHeartbeatRequest{
		DomainId: t.DomainUUID,
		Request:  FromRecordActivityTaskHeartbeatRequest(t.HeartbeatRequest),
	}
}

func ToHistoryRecordActivityTaskHeartbeatRequest(t *historyv1.RecordActivityTaskHeartbeatRequest) *types.HistoryRecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID:       t.DomainId,
		HeartbeatRequest: ToRecordActivityTaskHeartbeatRequest(t.Request),
	}
}

func FromHistoryRecordActivityTaskHeartbeatResponse(t *types.RecordActivityTaskHeartbeatResponse) *historyv1.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.CancelRequested,
	}
}

func ToHistoryRecordActivityTaskHeartbeatResponse(t *historyv1.RecordActivityTaskHeartbeatResponse) *types.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.CancelRequested,
	}
}

func FromHistoryRecordActivityTaskStartedRequest(t *types.RecordActivityTaskStartedRequest) *historyv1.RecordActivityTaskStartedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RecordActivityTaskStartedRequest{
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		ScheduleId:        t.ScheduleID,
		TaskId:            t.TaskID,
		RequestId:         t.RequestID,
		PollRequest:       FromPollForActivityTaskRequest(t.PollRequest),
	}
}

func ToHistoryRecordActivityTaskStartedRequest(t *historyv1.RecordActivityTaskStartedRequest) *types.RecordActivityTaskStartedRequest {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskStartedRequest{
		DomainUUID:        t.DomainId,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		ScheduleID:        t.ScheduleId,
		TaskID:            t.TaskId,
		RequestID:         t.RequestId,
		PollRequest:       ToPollForActivityTaskRequest(t.PollRequest),
	}
}

func FromHistoryRecordActivityTaskStartedResponse(t *types.RecordActivityTaskStartedResponse) *historyv1.RecordActivityTaskStartedResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RecordActivityTaskStartedResponse{
		ScheduledEvent:             FromHistoryEvent(t.ScheduledEvent),
		StartedTime:                unixNanoToTime(t.StartedTimestamp),
		Attempt:                    int32(t.Attempt),
		ScheduledTimeOfThisAttempt: unixNanoToTime(t.ScheduledTimestampOfThisAttempt),
		HeartbeatDetails:           FromPayload(t.HeartbeatDetails),
		WorkflowType:               FromWorkflowType(t.WorkflowType),
		WorkflowDomain:             t.WorkflowDomain,
	}
}

func ToHistoryRecordActivityTaskStartedResponse(t *historyv1.RecordActivityTaskStartedResponse) *types.RecordActivityTaskStartedResponse {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskStartedResponse{
		ScheduledEvent:                  ToHistoryEvent(t.ScheduledEvent),
		StartedTimestamp:                timeToUnixNano(t.StartedTime),
		Attempt:                         int64(t.Attempt),
		ScheduledTimestampOfThisAttempt: timeToUnixNano(t.ScheduledTimeOfThisAttempt),
		HeartbeatDetails:                ToPayload(t.HeartbeatDetails),
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.WorkflowDomain,
	}
}

func FromHistoryRecordChildExecutionCompletedRequest(t *types.RecordChildExecutionCompletedRequest) *historyv1.RecordChildExecutionCompletedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RecordChildExecutionCompletedRequest{
		DomainId:           t.DomainUUID,
		WorkflowExecution:  FromWorkflowExecution(t.WorkflowExecution),
		InitiatedId:        t.InitiatedID,
		CompletedExecution: FromWorkflowExecution(t.CompletedExecution),
		CompletionEvent:    FromHistoryEvent(t.CompletionEvent),
		StartedId:          t.StartedID,
	}
}

func ToHistoryRecordChildExecutionCompletedRequest(t *historyv1.RecordChildExecutionCompletedRequest) *types.RecordChildExecutionCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RecordChildExecutionCompletedRequest{
		DomainUUID:         t.DomainId,
		WorkflowExecution:  ToWorkflowExecution(t.WorkflowExecution),
		InitiatedID:        t.InitiatedId,
		CompletedExecution: ToWorkflowExecution(t.CompletedExecution),
		CompletionEvent:    ToHistoryEvent(t.CompletionEvent),
		StartedID:          t.StartedId,
	}
}

func FromHistoryRecordDecisionTaskStartedRequest(t *types.RecordDecisionTaskStartedRequest) *historyv1.RecordDecisionTaskStartedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RecordDecisionTaskStartedRequest{
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		ScheduleId:        t.ScheduleID,
		TaskId:            t.TaskID,
		RequestId:         t.RequestID,
		PollRequest:       FromPollForDecisionTaskRequest(t.PollRequest),
	}
}

func ToHistoryRecordDecisionTaskStartedRequest(t *historyv1.RecordDecisionTaskStartedRequest) *types.RecordDecisionTaskStartedRequest {
	if t == nil {
		return nil
	}
	return &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        t.DomainId,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		ScheduleID:        t.ScheduleId,
		TaskID:            t.TaskId,
		RequestID:         t.RequestId,
		PollRequest:       ToPollForDecisionTaskRequest(t.PollRequest),
	}
}

func FromHistoryRecordDecisionTaskStartedResponse(t *types.RecordDecisionTaskStartedResponse) *historyv1.RecordDecisionTaskStartedResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RecordDecisionTaskStartedResponse{
		WorkflowType:              FromWorkflowType(t.WorkflowType),
		PreviousStartedEventId:    fromInt64Value(t.PreviousStartedEventID),
		ScheduledEventId:          t.ScheduledEventID,
		StartedEventId:            t.StartedEventID,
		NextEventId:               t.NextEventID,
		Attempt:                   int32(t.Attempt),
		StickyExecutionEnabled:    t.StickyExecutionEnabled,
		DecisionInfo:              FromTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: FromTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         t.EventStoreVersion,
		BranchToken:               t.BranchToken,
		ScheduledTime:             unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:               unixNanoToTime(t.StartedTimestamp),
		Queries:                   FromWorkflowQueryMap(t.Queries),
		HistorySize:               t.HistorySize,
	}
}

func ToHistoryRecordDecisionTaskStartedResponse(t *historyv1.RecordDecisionTaskStartedResponse) *types.RecordDecisionTaskStartedResponse {
	if t == nil {
		return nil
	}
	return &types.RecordDecisionTaskStartedResponse{
		WorkflowType:              ToWorkflowType(t.WorkflowType),
		PreviousStartedEventID:    toInt64Value(t.PreviousStartedEventId),
		ScheduledEventID:          t.ScheduledEventId,
		StartedEventID:            t.StartedEventId,
		NextEventID:               t.NextEventId,
		Attempt:                   int64(t.Attempt),
		StickyExecutionEnabled:    t.StickyExecutionEnabled,
		DecisionInfo:              ToTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: ToTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         t.EventStoreVersion,
		BranchToken:               t.BranchToken,
		ScheduledTimestamp:        timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:          timeToUnixNano(t.StartedTime),
		Queries:                   ToWorkflowQueryMap(t.Queries),
		HistorySize:               t.HistorySize,
	}
}

func FromHistoryRefreshWorkflowTasksRequest(t *types.HistoryRefreshWorkflowTasksRequest) *historyv1.RefreshWorkflowTasksRequest {
	if t == nil || t.Request == nil {
		return nil
	}
	return &historyv1.RefreshWorkflowTasksRequest{
		DomainId:          t.DomainUIID,
		Domain:            t.Request.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Request.Execution),
	}
}

func ToHistoryRefreshWorkflowTasksRequest(t *historyv1.RefreshWorkflowTasksRequest) *types.HistoryRefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRefreshWorkflowTasksRequest{
		Request: &types.RefreshWorkflowTasksRequest{
			Domain:    t.Domain,
			Execution: ToWorkflowExecution(t.WorkflowExecution),
		},
		DomainUIID: t.DomainId,
	}
}

func FromHistoryRemoveSignalMutableStateRequest(t *types.RemoveSignalMutableStateRequest) *historyv1.RemoveSignalMutableStateRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RemoveSignalMutableStateRequest{
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		RequestId:         t.RequestID,
	}
}

func ToHistoryRemoveSignalMutableStateRequest(t *historyv1.RemoveSignalMutableStateRequest) *types.RemoveSignalMutableStateRequest {
	if t == nil {
		return nil
	}
	return &types.RemoveSignalMutableStateRequest{
		DomainUUID:        t.DomainId,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		RequestID:         t.RequestId,
	}
}

func FromHistoryRemoveTaskRequest(t *types.RemoveTaskRequest) *historyv1.RemoveTaskRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RemoveTaskRequest{
		ShardId:        t.ShardID,
		TaskType:       FromTaskType(t.Type),
		TaskId:         t.TaskID,
		VisibilityTime: unixNanoToTime(t.VisibilityTimestamp),
		ClusterName:    t.ClusterName,
	}
}

func ToHistoryRemoveTaskRequest(t *historyv1.RemoveTaskRequest) *types.RemoveTaskRequest {
	if t == nil {
		return nil
	}
	return &types.RemoveTaskRequest{
		ShardID:             t.ShardId,
		Type:                ToTaskType(t.TaskType),
		TaskID:              t.TaskId,
		VisibilityTimestamp: timeToUnixNano(t.VisibilityTime),
		ClusterName:         t.ClusterName,
	}
}

func FromHistoryReplicateEventsV2Request(t *types.ReplicateEventsV2Request) *historyv1.ReplicateEventsV2Request {
	if t == nil {
		return nil
	}
	return &historyv1.ReplicateEventsV2Request{
		DomainId:            t.DomainUUID,
		WorkflowExecution:   FromWorkflowExecution(t.WorkflowExecution),
		VersionHistoryItems: FromVersionHistoryItemArray(t.VersionHistoryItems),
		Events:              FromDataBlob(t.Events),
		NewRunEvents:        FromDataBlob(t.NewRunEvents),
	}
}

func ToHistoryReplicateEventsV2Request(t *historyv1.ReplicateEventsV2Request) *types.ReplicateEventsV2Request {
	if t == nil {
		return nil
	}
	return &types.ReplicateEventsV2Request{
		DomainUUID:          t.DomainId,
		WorkflowExecution:   ToWorkflowExecution(t.WorkflowExecution),
		VersionHistoryItems: ToVersionHistoryItemArray(t.VersionHistoryItems),
		Events:              ToDataBlob(t.Events),
		NewRunEvents:        ToDataBlob(t.NewRunEvents),
	}
}

func FromHistoryRequestCancelWorkflowExecutionRequest(t *types.HistoryRequestCancelWorkflowExecutionRequest) *historyv1.RequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RequestCancelWorkflowExecutionRequest{
		DomainId:              t.DomainUUID,
		CancelRequest:         FromRequestCancelWorkflowExecutionRequest(t.CancelRequest),
		ExternalExecutionInfo: FromExternalExecutionInfoFields(t.ExternalWorkflowExecution, t.ExternalInitiatedEventID),
		ChildWorkflowOnly:     t.ChildWorkflowOnly,
	}
}

func ToHistoryRequestCancelWorkflowExecutionRequest(t *historyv1.RequestCancelWorkflowExecutionRequest) *types.HistoryRequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID:                t.DomainId,
		CancelRequest:             ToRequestCancelWorkflowExecutionRequest(t.CancelRequest),
		ExternalInitiatedEventID:  ToExternalInitiatedID(t.ExternalExecutionInfo),
		ExternalWorkflowExecution: ToExternalWorkflowExecution(t.ExternalExecutionInfo),
		ChildWorkflowOnly:         t.ChildWorkflowOnly,
	}
}

func FromHistoryResetQueueRequest(t *types.ResetQueueRequest) *historyv1.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &historyv1.ResetQueueRequest{
		ShardId:     t.ShardID,
		ClusterName: t.ClusterName,
		TaskType:    FromTaskType(t.Type),
	}
}

func ToHistoryResetQueueRequest(t *historyv1.ResetQueueRequest) *types.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &types.ResetQueueRequest{
		ShardID:     t.ShardId,
		ClusterName: t.ClusterName,
		Type:        ToTaskType(t.TaskType),
	}
}

func FromHistoryResetStickyTaskListRequest(t *types.HistoryResetStickyTaskListRequest) *historyv1.ResetStickyTaskListRequest {
	if t == nil {
		return nil
	}
	return &historyv1.ResetStickyTaskListRequest{
		Request: &apiv1.ResetStickyTaskListRequest{
			Domain:            "",
			WorkflowExecution: FromWorkflowExecution(t.Execution),
		},
		DomainId: t.DomainUUID,
	}
}

func ToHistoryResetStickyTaskListRequest(t *historyv1.ResetStickyTaskListRequest) *types.HistoryResetStickyTaskListRequest {
	if t == nil || t.Request == nil {
		return nil
	}
	return &types.HistoryResetStickyTaskListRequest{
		DomainUUID: t.DomainId,
		Execution:  ToWorkflowExecution(t.Request.WorkflowExecution),
	}
}

func FromHistoryResetWorkflowExecutionRequest(t *types.HistoryResetWorkflowExecutionRequest) *historyv1.ResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.ResetWorkflowExecutionRequest{
		Request:  FromResetWorkflowExecutionRequest(t.ResetRequest),
		DomainId: t.DomainUUID,
	}
}

func ToHistoryResetWorkflowExecutionRequest(t *historyv1.ResetWorkflowExecutionRequest) *types.HistoryResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryResetWorkflowExecutionRequest{
		ResetRequest: ToResetWorkflowExecutionRequest(t.Request),
		DomainUUID:   t.DomainId,
	}
}

func FromHistoryResetWorkflowExecutionResponse(t *types.ResetWorkflowExecutionResponse) *historyv1.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &historyv1.ResetWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToHistoryResetWorkflowExecutionResponse(t *historyv1.ResetWorkflowExecutionResponse) *types.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.ResetWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromHistoryRespondActivityTaskCanceledRequest(t *types.HistoryRespondActivityTaskCanceledRequest) *historyv1.RespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondActivityTaskCanceledRequest{
		DomainId: t.DomainUUID,
		Request:  FromRespondActivityTaskCanceledRequest(t.CancelRequest),
	}
}

func ToHistoryRespondActivityTaskCanceledRequest(t *historyv1.RespondActivityTaskCanceledRequest) *types.HistoryRespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID:    t.DomainId,
		CancelRequest: ToRespondActivityTaskCanceledRequest(t.Request),
	}
}

func FromHistoryRespondActivityTaskCompletedRequest(t *types.HistoryRespondActivityTaskCompletedRequest) *historyv1.RespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondActivityTaskCompletedRequest{
		DomainId: t.DomainUUID,
		Request:  FromRespondActivityTaskCompletedRequest(t.CompleteRequest),
	}
}

func ToHistoryRespondActivityTaskCompletedRequest(t *historyv1.RespondActivityTaskCompletedRequest) *types.HistoryRespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID:      t.DomainId,
		CompleteRequest: ToRespondActivityTaskCompletedRequest(t.Request),
	}
}

func FromHistoryRespondActivityTaskFailedRequest(t *types.HistoryRespondActivityTaskFailedRequest) *historyv1.RespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondActivityTaskFailedRequest{
		DomainId: t.DomainUUID,
		Request:  FromRespondActivityTaskFailedRequest(t.FailedRequest),
	}
}

func ToHistoryRespondActivityTaskFailedRequest(t *historyv1.RespondActivityTaskFailedRequest) *types.HistoryRespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID:    t.DomainId,
		FailedRequest: ToRespondActivityTaskFailedRequest(t.Request),
	}
}

func FromHistoryRespondDecisionTaskCompletedRequest(t *types.HistoryRespondDecisionTaskCompletedRequest) *historyv1.RespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondDecisionTaskCompletedRequest{
		DomainId: t.DomainUUID,
		Request:  FromRespondDecisionTaskCompletedRequest(t.CompleteRequest),
	}
}

func ToHistoryRespondDecisionTaskCompletedRequest(t *historyv1.RespondDecisionTaskCompletedRequest) *types.HistoryRespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID:      t.DomainId,
		CompleteRequest: ToRespondDecisionTaskCompletedRequest(t.Request),
	}
}

func FromHistoryRespondDecisionTaskCompletedResponse(t *types.HistoryRespondDecisionTaskCompletedResponse) *historyv1.RespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RespondDecisionTaskCompletedResponse{
		StartedResponse:             FromHistoryRecordDecisionTaskStartedResponse(t.StartedResponse),
		ActivitiesToDispatchLocally: FromActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

func ToHistoryRespondDecisionTaskCompletedResponse(t *historyv1.RespondDecisionTaskCompletedResponse) *types.HistoryRespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondDecisionTaskCompletedResponse{
		StartedResponse:             ToHistoryRecordDecisionTaskStartedResponse(t.StartedResponse),
		ActivitiesToDispatchLocally: ToActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

func FromHistoryRespondDecisionTaskFailedRequest(t *types.HistoryRespondDecisionTaskFailedRequest) *historyv1.RespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondDecisionTaskFailedRequest{
		DomainId: t.DomainUUID,
		Request:  FromRespondDecisionTaskFailedRequest(t.FailedRequest),
	}
}

func ToHistoryRespondDecisionTaskFailedRequest(t *historyv1.RespondDecisionTaskFailedRequest) *types.HistoryRespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryRespondDecisionTaskFailedRequest{
		DomainUUID:    t.DomainId,
		FailedRequest: ToRespondDecisionTaskFailedRequest(t.Request),
	}
}

func FromHistoryScheduleDecisionTaskRequest(t *types.ScheduleDecisionTaskRequest) *historyv1.ScheduleDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &historyv1.ScheduleDecisionTaskRequest{
		DomainId:          t.DomainUUID,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		IsFirstDecision:   t.IsFirstDecision,
	}
}

func ToHistoryScheduleDecisionTaskRequest(t *historyv1.ScheduleDecisionTaskRequest) *types.ScheduleDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.ScheduleDecisionTaskRequest{
		DomainUUID:        t.DomainId,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		IsFirstDecision:   t.IsFirstDecision,
	}
}

func FromHistorySignalWithStartWorkflowExecutionRequest(t *types.HistorySignalWithStartWorkflowExecutionRequest) *historyv1.SignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.SignalWithStartWorkflowExecutionRequest{
		Request:         FromSignalWithStartWorkflowExecutionRequest(t.SignalWithStartRequest),
		DomainId:        t.DomainUUID,
		PartitionConfig: t.PartitionConfig,
	}
}

func ToHistorySignalWithStartWorkflowExecutionRequest(t *historyv1.SignalWithStartWorkflowExecutionRequest) *types.HistorySignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistorySignalWithStartWorkflowExecutionRequest{
		SignalWithStartRequest: ToSignalWithStartWorkflowExecutionRequest(t.Request),
		DomainUUID:             t.DomainId,
		PartitionConfig:        t.PartitionConfig,
	}
}

func FromHistorySignalWithStartWorkflowExecutionResponse(t *types.StartWorkflowExecutionResponse) *historyv1.SignalWithStartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &historyv1.SignalWithStartWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToHistorySignalWithStartWorkflowExecutionResponse(t *historyv1.SignalWithStartWorkflowExecutionResponse) *types.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromHistorySignalWorkflowExecutionRequest(t *types.HistorySignalWorkflowExecutionRequest) *historyv1.SignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.SignalWorkflowExecutionRequest{
		Request:                   FromSignalWorkflowExecutionRequest(t.SignalRequest),
		DomainId:                  t.DomainUUID,
		ExternalWorkflowExecution: FromWorkflowExecution(t.ExternalWorkflowExecution),
		ChildWorkflowOnly:         t.ChildWorkflowOnly,
	}
}

func ToHistorySignalWorkflowExecutionRequest(t *historyv1.SignalWorkflowExecutionRequest) *types.HistorySignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistorySignalWorkflowExecutionRequest{
		SignalRequest:             ToSignalWorkflowExecutionRequest(t.Request),
		DomainUUID:                t.DomainId,
		ExternalWorkflowExecution: ToWorkflowExecution(t.ExternalWorkflowExecution),
		ChildWorkflowOnly:         t.ChildWorkflowOnly,
	}
}

func FromHistoryStartWorkflowExecutionRequest(t *types.HistoryStartWorkflowExecutionRequest) *historyv1.StartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.StartWorkflowExecutionRequest{
		Request:                  FromStartWorkflowExecutionRequest(t.StartRequest),
		DomainId:                 t.DomainUUID,
		ParentExecutionInfo:      FromParentExecutionInfo(t.ParentExecutionInfo),
		Attempt:                  t.Attempt,
		ExpirationTime:           unixNanoToTime(t.ExpirationTimestamp),
		ContinueAsNewInitiator:   FromContinueAsNewInitiator(t.ContinueAsNewInitiator),
		ContinuedFailure:         FromFailure(t.ContinuedFailureReason, t.ContinuedFailureDetails),
		LastCompletionResult:     FromPayload(t.LastCompletionResult),
		FirstDecisionTaskBackoff: secondsToDuration(t.FirstDecisionTaskBackoffSeconds),
		PartitionConfig:          t.PartitionConfig,
	}
}

func ToHistoryStartWorkflowExecutionRequest(t *historyv1.StartWorkflowExecutionRequest) *types.HistoryStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryStartWorkflowExecutionRequest{
		StartRequest:                    ToStartWorkflowExecutionRequest(t.Request),
		DomainUUID:                      t.DomainId,
		ParentExecutionInfo:             ToParentExecutionInfo(t.ParentExecutionInfo),
		Attempt:                         t.Attempt,
		ExpirationTimestamp:             timeToUnixNano(t.ExpirationTime),
		ContinueAsNewInitiator:          ToContinueAsNewInitiator(t.ContinueAsNewInitiator),
		ContinuedFailureReason:          ToFailureReason(t.ContinuedFailure),
		ContinuedFailureDetails:         ToFailureDetails(t.ContinuedFailure),
		LastCompletionResult:            ToPayload(t.LastCompletionResult),
		FirstDecisionTaskBackoffSeconds: durationToSeconds(t.FirstDecisionTaskBackoff),
		PartitionConfig:                 t.PartitionConfig,
	}
}

func FromHistoryStartWorkflowExecutionResponse(t *types.StartWorkflowExecutionResponse) *historyv1.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &historyv1.StartWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToHistoryStartWorkflowExecutionResponse(t *historyv1.StartWorkflowExecutionResponse) *types.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromHistorySyncActivityRequest(t *types.SyncActivityRequest) *historyv1.SyncActivityRequest {
	if t == nil {
		return nil
	}
	return &historyv1.SyncActivityRequest{
		DomainId:           t.DomainID,
		WorkflowExecution:  FromWorkflowRunPair(t.WorkflowID, t.RunID),
		Version:            t.Version,
		ScheduledId:        t.ScheduledID,
		ScheduledTime:      unixNanoToTime(t.ScheduledTime),
		StartedId:          t.StartedID,
		StartedTime:        unixNanoToTime(t.StartedTime),
		LastHeartbeatTime:  unixNanoToTime(t.LastHeartbeatTime),
		Details:            FromPayload(t.Details),
		Attempt:            t.Attempt,
		LastFailure:        FromFailure(t.LastFailureReason, t.LastFailureDetails),
		LastWorkerIdentity: t.LastWorkerIdentity,
		VersionHistory:     FromVersionHistory(t.VersionHistory),
	}
}

func ToHistorySyncActivityRequest(t *historyv1.SyncActivityRequest) *types.SyncActivityRequest {
	if t == nil {
		return nil
	}
	return &types.SyncActivityRequest{
		DomainID:           t.DomainId,
		WorkflowID:         ToWorkflowID(t.WorkflowExecution),
		RunID:              ToRunID(t.WorkflowExecution),
		Version:            t.Version,
		ScheduledID:        t.ScheduledId,
		ScheduledTime:      timeToUnixNano(t.ScheduledTime),
		StartedID:          t.StartedId,
		StartedTime:        timeToUnixNano(t.StartedTime),
		LastHeartbeatTime:  timeToUnixNano(t.LastHeartbeatTime),
		Details:            ToPayload(t.Details),
		Attempt:            t.Attempt,
		LastFailureReason:  ToFailureReason(t.LastFailure),
		LastFailureDetails: ToFailureDetails(t.LastFailure),
		LastWorkerIdentity: t.LastWorkerIdentity,
		VersionHistory:     ToVersionHistory(t.VersionHistory),
	}
}

func FromHistorySyncShardStatusRequest(t *types.SyncShardStatusRequest) *historyv1.SyncShardStatusRequest {
	if t == nil {
		return nil
	}
	return &historyv1.SyncShardStatusRequest{
		SourceCluster: t.SourceCluster,
		ShardId:       int32(t.ShardID),
		Time:          unixNanoToTime(t.Timestamp),
	}
}

func ToHistorySyncShardStatusRequest(t *historyv1.SyncShardStatusRequest) *types.SyncShardStatusRequest {
	if t == nil {
		return nil
	}
	return &types.SyncShardStatusRequest{
		SourceCluster: t.SourceCluster,
		ShardID:       int64(t.ShardId),
		Timestamp:     timeToUnixNano(t.Time),
	}
}

func FromHistoryTerminateWorkflowExecutionRequest(t *types.HistoryTerminateWorkflowExecutionRequest) *historyv1.TerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &historyv1.TerminateWorkflowExecutionRequest{
		Request:                   FromTerminateWorkflowExecutionRequest(t.TerminateRequest),
		DomainId:                  t.DomainUUID,
		ExternalWorkflowExecution: FromWorkflowExecution(t.ExternalWorkflowExecution),
		ChildWorkflowOnly:         t.ChildWorkflowOnly,
	}
}

func ToHistoryTerminateWorkflowExecutionRequest(t *historyv1.TerminateWorkflowExecutionRequest) *types.HistoryTerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.HistoryTerminateWorkflowExecutionRequest{
		TerminateRequest:          ToTerminateWorkflowExecutionRequest(t.Request),
		DomainUUID:                t.DomainId,
		ExternalWorkflowExecution: ToWorkflowExecution(t.ExternalWorkflowExecution),
		ChildWorkflowOnly:         t.ChildWorkflowOnly,
	}
}

// FromHistoryGetCrossClusterTasksRequest converts internal GetCrossClusterTasksRequest type to proto
func FromHistoryGetCrossClusterTasksRequest(t *types.GetCrossClusterTasksRequest) *historyv1.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &historyv1.GetCrossClusterTasksRequest{
		ShardIds:      t.ShardIDs,
		TargetCluster: t.TargetCluster,
	}
}

// ToHistoryGetCrossClusterTasksRequest converts proto GetCrossClusterTasksRequest type to internal
func ToHistoryGetCrossClusterTasksRequest(t *historyv1.GetCrossClusterTasksRequest) *types.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksRequest{
		ShardIDs:      t.ShardIds,
		TargetCluster: t.TargetCluster,
	}
}

// FromHistoryGetCrossClusterTasksResponse converts internal GetCrossClusterTasksResponse type to proto
func FromHistoryGetCrossClusterTasksResponse(t *types.GetCrossClusterTasksResponse) *historyv1.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &historyv1.GetCrossClusterTasksResponse{
		TasksByShard:       FromCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: FromGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// ToHistoryGetCrossClusterTasksResponse converts proto GetCrossClusterTasksResponse type to internal
func ToHistoryGetCrossClusterTasksResponse(t *historyv1.GetCrossClusterTasksResponse) *types.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksResponse{
		TasksByShard:       ToCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: ToGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// FromHistoryRespondCrossClusterTasksCompletedRequest converts internal RespondCrossClusterTasksCompletedRequest type to thrift
func FromHistoryRespondCrossClusterTasksCompletedRequest(t *types.RespondCrossClusterTasksCompletedRequest) *historyv1.RespondCrossClusterTasksCompletedRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RespondCrossClusterTasksCompletedRequest{
		ShardId:       t.ShardID,
		TargetCluster: t.TargetCluster,
		TaskResponses: FromCrossClusterTaskResponseArray(t.TaskResponses),
		FetchNewTasks: t.FetchNewTasks,
	}
}

// ToHistoryRespondCrossClusterTasksCompletedRequest converts thrift RespondCrossClusterTasksCompletedRequest type to internal
func ToHistoryRespondCrossClusterTasksCompletedRequest(t *historyv1.RespondCrossClusterTasksCompletedRequest) *types.RespondCrossClusterTasksCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondCrossClusterTasksCompletedRequest{
		ShardID:       t.ShardId,
		TargetCluster: t.TargetCluster,
		TaskResponses: ToCrossClusterTaskResponseArray(t.TaskResponses),
		FetchNewTasks: t.FetchNewTasks,
	}
}

// FromHistoryRespondCrossClusterTasksCompletedResponse converts internal RespondCrossClusterTasksCompletedResponse type to thrift
func FromHistoryRespondCrossClusterTasksCompletedResponse(t *types.RespondCrossClusterTasksCompletedResponse) *historyv1.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RespondCrossClusterTasksCompletedResponse{
		Tasks: FromCrossClusterTaskRequestArray(t.Tasks),
	}
}

// ToHistoryRespondCrossClusterTasksCompletedResponse converts thrift RespondCrossClusterTasksCompletedResponse type to internal
func ToHistoryRespondCrossClusterTasksCompletedResponse(t *historyv1.RespondCrossClusterTasksCompletedResponse) *types.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.RespondCrossClusterTasksCompletedResponse{
		Tasks: ToCrossClusterTaskRequestArray(t.Tasks),
	}
}

func FromHistoryResetStickyTaskListResponse(t *types.HistoryResetStickyTaskListResponse) *historyv1.ResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &historyv1.ResetStickyTaskListResponse{}
}

func ToHistoryResetStickyTaskListResponse(t *historyv1.ResetStickyTaskListResponse) *types.HistoryResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.HistoryResetStickyTaskListResponse{}
}

func FromHistoryRatelimitUpdateRequest(t *types.RatelimitUpdateRequest) *historyv1.RatelimitUpdateRequest {
	if t == nil {
		return nil
	}
	return &historyv1.RatelimitUpdateRequest{
		Data: FromAny(t.Any),
	}
}
func ToHistoryRatelimitUpdateRequest(t *historyv1.RatelimitUpdateRequest) *types.RatelimitUpdateRequest {
	if t == nil {
		return nil
	}
	return &types.RatelimitUpdateRequest{
		Any: ToAny(t.Data),
	}
}

func FromHistoryRatelimitUpdateResponse(t *types.RatelimitUpdateResponse) *historyv1.RatelimitUpdateResponse {
	if t == nil {
		return nil
	}
	return &historyv1.RatelimitUpdateResponse{
		Data: FromAny(t.Any),
	}
}

func ToHistoryRatelimitUpdateResponse(t *historyv1.RatelimitUpdateResponse) *types.RatelimitUpdateResponse {
	if t == nil {
		return nil
	}
	return &types.RatelimitUpdateResponse{
		Any: ToAny(t.Data),
	}
}
