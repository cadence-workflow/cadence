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
	"sort"

	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common/types"
)

func FromAdminAddSearchAttributeRequest(t *types.AddSearchAttributeRequest) *adminv1.AddSearchAttributeRequest {
	if t == nil {
		return nil
	}
	return &adminv1.AddSearchAttributeRequest{
		SearchAttribute: FromIndexedValueTypeMap(t.SearchAttribute),
		SecurityToken:   t.SecurityToken,
	}
}

func ToAdminAddSearchAttributeRequest(t *adminv1.AddSearchAttributeRequest) *types.AddSearchAttributeRequest {
	if t == nil {
		return nil
	}
	return &types.AddSearchAttributeRequest{
		SearchAttribute: ToIndexedValueTypeMap(t.SearchAttribute),
		SecurityToken:   t.SecurityToken,
	}
}

func FromAdminCloseShardRequest(t *types.CloseShardRequest) *adminv1.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &adminv1.CloseShardRequest{
		ShardId: t.ShardID,
	}
}

func ToAdminCloseShardRequest(t *adminv1.CloseShardRequest) *types.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &types.CloseShardRequest{
		ShardID: t.ShardId,
	}
}

func FromAdminDescribeClusterResponse(t *types.DescribeClusterResponse) *adminv1.DescribeClusterResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeClusterResponse{
		SupportedClientVersions: FromSupportedClientVersions(t.SupportedClientVersions),
		MembershipInfo:          FromMembershipInfo(t.MembershipInfo),
		PersistenceInfo:         FromPersistenceInfoMap(t.PersistenceInfo),
	}
}

func ToAdminDescribeClusterResponse(t *adminv1.DescribeClusterResponse) *types.DescribeClusterResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeClusterResponse{
		SupportedClientVersions: ToSupportedClientVersions(t.SupportedClientVersions),
		MembershipInfo:          ToMembershipInfo(t.MembershipInfo),
		PersistenceInfo:         ToPersistenceInfoMap(t.PersistenceInfo),
	}
}

func FromAdminDescribeShardDistributionRequest(t *types.DescribeShardDistributionRequest) *adminv1.DescribeShardDistributionRequest {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeShardDistributionRequest{
		PageSize: t.PageSize,
		PageId:   t.PageID,
	}
}

func FromAdminDescribeHistoryHostRequest(t *types.DescribeHistoryHostRequest) *adminv1.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	if t.HostAddress != nil {
		return &adminv1.DescribeHistoryHostRequest{
			DescribeBy: &adminv1.DescribeHistoryHostRequest_HostAddress{HostAddress: *t.HostAddress},
		}
	}
	if t.ShardIDForHost != nil {
		return &adminv1.DescribeHistoryHostRequest{
			DescribeBy: &adminv1.DescribeHistoryHostRequest_ShardId{ShardId: *t.ShardIDForHost},
		}
	}
	if t.ExecutionForHost != nil {
		return &adminv1.DescribeHistoryHostRequest{
			DescribeBy: &adminv1.DescribeHistoryHostRequest_WorkflowExecution{WorkflowExecution: FromWorkflowExecution(t.ExecutionForHost)},
		}
	}
	panic("neither oneof field is set for DescribeHistoryHostRequest")
}

func ToAdminDescribeShardDistributionRequest(t *adminv1.DescribeShardDistributionRequest) *types.DescribeShardDistributionRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeShardDistributionRequest{
		PageSize: t.PageSize,
		PageID:   t.PageId,
	}
}

func ToAdminDescribeHistoryHostRequest(t *adminv1.DescribeHistoryHostRequest) *types.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	switch describeBy := t.DescribeBy.(type) {
	case *adminv1.DescribeHistoryHostRequest_HostAddress:
		return &types.DescribeHistoryHostRequest{HostAddress: &describeBy.HostAddress}
	case *adminv1.DescribeHistoryHostRequest_ShardId:
		return &types.DescribeHistoryHostRequest{ShardIDForHost: &describeBy.ShardId}
	case *adminv1.DescribeHistoryHostRequest_WorkflowExecution:
		return &types.DescribeHistoryHostRequest{ExecutionForHost: ToWorkflowExecution(describeBy.WorkflowExecution)}
	}
	panic("neither oneof field is set for DescribeHistoryHostRequest")
}

func FromAdminDescribeShardDistributionResponse(t *types.DescribeShardDistributionResponse) *adminv1.DescribeShardDistributionResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeShardDistributionResponse{
		NumberOfShards: t.NumberOfShards,
		Shards:         t.Shards,
	}
}

func FromAdminDescribeHistoryHostResponse(t *types.DescribeHistoryHostResponse) *adminv1.DescribeHistoryHostResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeHistoryHostResponse{
		NumberOfShards:        t.NumberOfShards,
		ShardIds:              t.ShardIDs,
		DomainCache:           FromDomainCacheInfo(t.DomainCache),
		ShardControllerStatus: t.ShardControllerStatus,
		Address:               t.Address,
	}
}

func ToAdminDescribeShardDistributionResponse(t *adminv1.DescribeShardDistributionResponse) *types.DescribeShardDistributionResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeShardDistributionResponse{
		NumberOfShards: t.NumberOfShards,
		Shards:         t.Shards,
	}
}

func ToAdminDescribeHistoryHostResponse(t *adminv1.DescribeHistoryHostResponse) *types.DescribeHistoryHostResponse {
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

func FromAdminDescribeQueueRequest(t *types.DescribeQueueRequest) *adminv1.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeQueueRequest{
		ShardId:     t.ShardID,
		ClusterName: t.ClusterName,
		TaskType:    FromTaskType(t.Type),
	}
}

func ToAdminDescribeQueueRequest(t *adminv1.DescribeQueueRequest) *types.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueRequest{
		ShardID:     t.ShardId,
		ClusterName: t.ClusterName,
		Type:        ToTaskType(t.TaskType),
	}
}

func FromAdminDescribeQueueResponse(t *types.DescribeQueueResponse) *adminv1.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

func ToAdminDescribeQueueResponse(t *adminv1.DescribeQueueResponse) *types.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

func FromAdminDescribeWorkflowExecutionRequest(t *types.AdminDescribeWorkflowExecutionRequest) *adminv1.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func ToAdminDescribeWorkflowExecutionRequest(t *adminv1.DescribeWorkflowExecutionRequest) *types.AdminDescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.AdminDescribeWorkflowExecutionRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromAdminDescribeWorkflowExecutionResponse(t *types.AdminDescribeWorkflowExecutionResponse) *adminv1.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DescribeWorkflowExecutionResponse{
		ShardId:                stringToInt32(t.ShardID),
		HistoryAddr:            t.HistoryAddr,
		MutableStateInCache:    t.MutableStateInCache,
		MutableStateInDatabase: t.MutableStateInDatabase,
	}
}

func ToAdminDescribeWorkflowExecutionResponse(t *adminv1.DescribeWorkflowExecutionResponse) *types.AdminDescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.AdminDescribeWorkflowExecutionResponse{
		ShardID:                int32ToString(t.ShardId),
		HistoryAddr:            t.HistoryAddr,
		MutableStateInCache:    t.MutableStateInCache,
		MutableStateInDatabase: t.MutableStateInDatabase,
	}
}

func FromAdminGetDLQReplicationMessagesRequest(t *types.GetDLQReplicationMessagesRequest) *adminv1.GetDLQReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetDLQReplicationMessagesRequest{
		TaskInfos: FromReplicationTaskInfoArray(t.TaskInfos),
	}
}

func ToAdminGetDLQReplicationMessagesRequest(t *adminv1.GetDLQReplicationMessagesRequest) *types.GetDLQReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.GetDLQReplicationMessagesRequest{
		TaskInfos: ToReplicationTaskInfoArray(t.TaskInfos),
	}
}

func FromAdminGetDLQReplicationMessagesResponse(t *types.GetDLQReplicationMessagesResponse) *adminv1.GetDLQReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetDLQReplicationMessagesResponse{
		ReplicationTasks: FromReplicationTaskArray(t.ReplicationTasks),
	}
}

func ToAdminGetDLQReplicationMessagesResponse(t *adminv1.GetDLQReplicationMessagesResponse) *types.GetDLQReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.GetDLQReplicationMessagesResponse{
		ReplicationTasks: ToReplicationTaskArray(t.ReplicationTasks),
	}
}

func FromAdminGetDomainReplicationMessagesRequest(t *types.GetDomainReplicationMessagesRequest) *adminv1.GetDomainReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetDomainReplicationMessagesRequest{
		LastRetrievedMessageId: fromInt64Value(t.LastRetrievedMessageID),
		LastProcessedMessageId: fromInt64Value(t.LastProcessedMessageID),
		ClusterName:            t.ClusterName,
	}
}

func ToAdminGetDomainReplicationMessagesRequest(t *adminv1.GetDomainReplicationMessagesRequest) *types.GetDomainReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.GetDomainReplicationMessagesRequest{
		LastRetrievedMessageID: toInt64Value(t.LastRetrievedMessageId),
		LastProcessedMessageID: toInt64Value(t.LastProcessedMessageId),
		ClusterName:            t.ClusterName,
	}
}

func FromAdminGetDomainReplicationMessagesResponse(t *types.GetDomainReplicationMessagesResponse) *adminv1.GetDomainReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetDomainReplicationMessagesResponse{
		Messages: FromReplicationMessages(t.Messages),
	}
}

func ToAdminGetDomainReplicationMessagesResponse(t *adminv1.GetDomainReplicationMessagesResponse) *types.GetDomainReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.GetDomainReplicationMessagesResponse{
		Messages: ToReplicationMessages(t.Messages),
	}
}

func FromAdminGetReplicationMessagesRequest(t *types.GetReplicationMessagesRequest) *adminv1.GetReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetReplicationMessagesRequest{
		Tokens:      FromReplicationTokenArray(t.Tokens),
		ClusterName: t.ClusterName,
	}
}

func ToAdminGetReplicationMessagesRequest(t *adminv1.GetReplicationMessagesRequest) *types.GetReplicationMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.GetReplicationMessagesRequest{
		Tokens:      ToReplicationTokenArray(t.Tokens),
		ClusterName: t.ClusterName,
	}
}

func FromAdminGetReplicationMessagesResponse(t *types.GetReplicationMessagesResponse) *adminv1.GetReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetReplicationMessagesResponse{
		ShardMessages: FromReplicationMessagesMap(t.MessagesByShard),
	}
}

func ToAdminGetReplicationMessagesResponse(t *adminv1.GetReplicationMessagesResponse) *types.GetReplicationMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.GetReplicationMessagesResponse{
		MessagesByShard: ToReplicationMessagesMap(t.ShardMessages),
	}
}

func FromAdminGetWorkflowExecutionRawHistoryV2Request(t *types.GetWorkflowExecutionRawHistoryV2Request) *adminv1.GetWorkflowExecutionRawHistoryV2Request {
	if t == nil {
		return nil
	}
	return &adminv1.GetWorkflowExecutionRawHistoryV2Request{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
		StartEvent:        FromEventIDVersionPair(t.StartEventID, t.StartEventVersion),
		EndEvent:          FromEventIDVersionPair(t.EndEventID, t.EndEventVersion),
		PageSize:          t.MaximumPageSize,
		NextPageToken:     t.NextPageToken,
	}
}

func ToAdminGetWorkflowExecutionRawHistoryV2Request(t *adminv1.GetWorkflowExecutionRawHistoryV2Request) *types.GetWorkflowExecutionRawHistoryV2Request {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain:            t.Domain,
		Execution:         ToWorkflowExecution(t.WorkflowExecution),
		StartEventID:      ToEventID(t.StartEvent),
		StartEventVersion: ToEventVersion(t.StartEvent),
		EndEventID:        ToEventID(t.EndEvent),
		EndEventVersion:   ToEventVersion(t.EndEvent),
		MaximumPageSize:   t.PageSize,
		NextPageToken:     t.NextPageToken,
	}
}

func FromAdminGetWorkflowExecutionRawHistoryV2Response(t *types.GetWorkflowExecutionRawHistoryV2Response) *adminv1.GetWorkflowExecutionRawHistoryV2Response {
	if t == nil {
		return nil
	}
	return &adminv1.GetWorkflowExecutionRawHistoryV2Response{
		NextPageToken:  t.NextPageToken,
		HistoryBatches: FromDataBlobArray(t.HistoryBatches),
		VersionHistory: FromVersionHistory(t.VersionHistory),
	}
}

func ToAdminGetWorkflowExecutionRawHistoryV2Response(t *adminv1.GetWorkflowExecutionRawHistoryV2Response) *types.GetWorkflowExecutionRawHistoryV2Response {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionRawHistoryV2Response{
		NextPageToken:  t.NextPageToken,
		HistoryBatches: ToDataBlobArray(t.HistoryBatches),
		VersionHistory: ToVersionHistory(t.VersionHistory),
	}
}

func FromAdminCountDLQMessagesRequest(t *types.CountDLQMessagesRequest) *adminv1.CountDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.CountDLQMessagesRequest{
		ForceFetch: t.ForceFetch,
	}
}

func ToAdminCountDLQMessagesRequest(t *adminv1.CountDLQMessagesRequest) *types.CountDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &types.CountDLQMessagesRequest{
		ForceFetch: t.ForceFetch,
	}
}

func FromAdminCountDLQMessagesResponse(t *types.CountDLQMessagesResponse) *adminv1.CountDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.CountDLQMessagesResponse{
		History: FromHistoryDLQCountEntryMap(t.History),
		Domain:  t.Domain,
	}
}

func ToAdminCountDLQMessagesResponse(t *adminv1.CountDLQMessagesResponse) *types.CountDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.CountDLQMessagesResponse{
		History: ToHistoryDLQCountEntryMap(t.History),
		Domain:  t.Domain,
	}
}

func FromAdminMergeDLQMessagesRequest(t *types.MergeDLQMessagesRequest) *adminv1.MergeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.MergeDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
		PageSize:              t.MaximumPageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func ToAdminMergeDLQMessagesRequest(t *adminv1.MergeDLQMessagesRequest) *types.MergeDLQMessagesRequest {
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

func FromAdminMergeDLQMessagesResponse(t *types.MergeDLQMessagesResponse) *adminv1.MergeDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.MergeDLQMessagesResponse{
		NextPageToken: t.NextPageToken,
	}
}

func ToAdminMergeDLQMessagesResponse(t *adminv1.MergeDLQMessagesResponse) *types.MergeDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &types.MergeDLQMessagesResponse{
		NextPageToken: t.NextPageToken,
	}
}

func FromAdminPurgeDLQMessagesRequest(t *types.PurgeDLQMessagesRequest) *adminv1.PurgeDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.PurgeDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
	}
}

func ToAdminPurgeDLQMessagesRequest(t *adminv1.PurgeDLQMessagesRequest) *types.PurgeDLQMessagesRequest {
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

func FromAdminReadDLQMessagesRequest(t *types.ReadDLQMessagesRequest) *adminv1.ReadDLQMessagesRequest {
	if t == nil {
		return nil
	}
	return &adminv1.ReadDLQMessagesRequest{
		Type:                  FromDLQType(t.Type),
		ShardId:               t.ShardID,
		SourceCluster:         t.SourceCluster,
		InclusiveEndMessageId: fromInt64Value(t.InclusiveEndMessageID),
		PageSize:              t.MaximumPageSize,
		NextPageToken:         t.NextPageToken,
	}
}

func ToAdminReadDLQMessagesRequest(t *adminv1.ReadDLQMessagesRequest) *types.ReadDLQMessagesRequest {
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

func FromAdminReadDLQMessagesResponse(t *types.ReadDLQMessagesResponse) *adminv1.ReadDLQMessagesResponse {
	if t == nil {
		return nil
	}
	return &adminv1.ReadDLQMessagesResponse{
		Type:                 FromDLQType(t.Type),
		ReplicationTasks:     FromReplicationTaskArray(t.ReplicationTasks),
		ReplicationTasksInfo: FromReplicationTaskInfoArray(t.ReplicationTasksInfo),
		NextPageToken:        t.NextPageToken,
	}
}

func ToAdminReadDLQMessagesResponse(t *adminv1.ReadDLQMessagesResponse) *types.ReadDLQMessagesResponse {
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

func FromAdminReapplyEventsRequest(t *types.ReapplyEventsRequest) *adminv1.ReapplyEventsRequest {
	if t == nil {
		return nil
	}
	return &adminv1.ReapplyEventsRequest{
		Domain:            t.DomainName,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Events:            FromDataBlob(t.Events),
	}
}

func ToAdminReapplyEventsRequest(t *adminv1.ReapplyEventsRequest) *types.ReapplyEventsRequest {
	if t == nil {
		return nil
	}
	return &types.ReapplyEventsRequest{
		DomainName:        t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Events:            ToDataBlob(t.Events),
	}
}

func FromAdminRefreshWorkflowTasksRequest(t *types.RefreshWorkflowTasksRequest) *adminv1.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &adminv1.RefreshWorkflowTasksRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func ToAdminRefreshWorkflowTasksRequest(t *adminv1.RefreshWorkflowTasksRequest) *types.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &types.RefreshWorkflowTasksRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromAdminRemoveTaskRequest(t *types.RemoveTaskRequest) *adminv1.RemoveTaskRequest {
	if t == nil {
		return nil
	}
	return &adminv1.RemoveTaskRequest{
		ShardId:        t.ShardID,
		TaskType:       FromTaskType(t.Type),
		TaskId:         t.TaskID,
		VisibilityTime: unixNanoToTime(t.VisibilityTimestamp),
		ClusterName:    t.ClusterName,
	}
}

func ToAdminRemoveTaskRequest(t *adminv1.RemoveTaskRequest) *types.RemoveTaskRequest {
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

func FromAdminResendReplicationTasksRequest(t *types.ResendReplicationTasksRequest) *adminv1.ResendReplicationTasksRequest {
	if t == nil {
		return nil
	}
	return &adminv1.ResendReplicationTasksRequest{
		DomainId:          t.DomainID,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		RemoteCluster:     t.RemoteCluster,
		StartEvent:        FromEventIDVersionPair(t.StartEventID, t.StartVersion),
		EndEvent:          FromEventIDVersionPair(t.EndEventID, t.EndVersion),
	}
}

func ToAdminResendReplicationTasksRequest(t *adminv1.ResendReplicationTasksRequest) *types.ResendReplicationTasksRequest {
	if t == nil {
		return nil
	}
	return &types.ResendReplicationTasksRequest{
		DomainID:      t.DomainId,
		WorkflowID:    ToWorkflowID(t.WorkflowExecution),
		RunID:         ToRunID(t.WorkflowExecution),
		RemoteCluster: t.RemoteCluster,
		StartEventID:  ToEventID(t.StartEvent),
		StartVersion:  ToEventVersion(t.StartEvent),
		EndEventID:    ToEventID(t.EndEvent),
		EndVersion:    ToEventVersion(t.EndEvent),
	}
}

func FromAdminResetQueueRequest(t *types.ResetQueueRequest) *adminv1.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &adminv1.ResetQueueRequest{
		ShardId:     t.ShardID,
		ClusterName: t.ClusterName,
		TaskType:    FromTaskType(t.Type),
	}
}

func ToAdminResetQueueRequest(t *adminv1.ResetQueueRequest) *types.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &types.ResetQueueRequest{
		ShardID:     t.ShardId,
		ClusterName: t.ClusterName,
		Type:        ToTaskType(t.TaskType),
	}
}

// FromAdminGetCrossClusterTasksRequest converts internal GetCrossClusterTasksRequest type to proto
func FromAdminGetCrossClusterTasksRequest(t *types.GetCrossClusterTasksRequest) *adminv1.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetCrossClusterTasksRequest{
		ShardIds:      t.ShardIDs,
		TargetCluster: t.TargetCluster,
	}
}

// ToAdminGetCrossClusterTasksRequest converts proto GetCrossClusterTasksRequest type to internal
func ToAdminGetCrossClusterTasksRequest(t *adminv1.GetCrossClusterTasksRequest) *types.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksRequest{
		ShardIDs:      t.ShardIds,
		TargetCluster: t.TargetCluster,
	}
}

// FromAdminGetCrossClusterTasksResponse converts internal GetCrossClusterTasksResponse type to proto
func FromAdminGetCrossClusterTasksResponse(t *types.GetCrossClusterTasksResponse) *adminv1.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetCrossClusterTasksResponse{
		TasksByShard:       FromCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: FromGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// ToAdminGetCrossClusterTasksResponse converts proto GetCrossClusterTasksResponse type to internal
func ToAdminGetCrossClusterTasksResponse(t *adminv1.GetCrossClusterTasksResponse) *types.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksResponse{
		TasksByShard:       ToCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: ToGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// FromAdminRespondCrossClusterTasksCompletedRequest converts internal RespondCrossClusterTasksCompletedRequest type to thrift
func FromAdminRespondCrossClusterTasksCompletedRequest(t *types.RespondCrossClusterTasksCompletedRequest) *adminv1.RespondCrossClusterTasksCompletedRequest {
	if t == nil {
		return nil
	}
	return &adminv1.RespondCrossClusterTasksCompletedRequest{
		ShardId:       t.ShardID,
		TargetCluster: t.TargetCluster,
		TaskResponses: FromCrossClusterTaskResponseArray(t.TaskResponses),
		FetchNewTasks: t.FetchNewTasks,
	}
}

// ToAdminRespondCrossClusterTasksCompletedRequest converts thrift RespondCrossClusterTasksCompletedRequest type to internal
func ToAdminRespondCrossClusterTasksCompletedRequest(t *adminv1.RespondCrossClusterTasksCompletedRequest) *types.RespondCrossClusterTasksCompletedRequest {
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

// FromAdminRespondCrossClusterTasksCompletedResponse converts internal RespondCrossClusterTasksCompletedResponse type to thrift
func FromAdminRespondCrossClusterTasksCompletedResponse(t *types.RespondCrossClusterTasksCompletedResponse) *adminv1.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &adminv1.RespondCrossClusterTasksCompletedResponse{
		Tasks: FromCrossClusterTaskRequestArray(t.Tasks),
	}
}

// ToAdminRespondCrossClusterTasksCompletedResponse converts thrift RespondCrossClusterTasksCompletedResponse type to internal
func ToAdminRespondCrossClusterTasksCompletedResponse(t *adminv1.RespondCrossClusterTasksCompletedResponse) *types.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.RespondCrossClusterTasksCompletedResponse{
		Tasks: ToCrossClusterTaskRequestArray(t.Tasks),
	}
}

// FromAdminGetDynamicConfigRequest converts internal GetDynamicConfigRequest type to proto
func FromAdminGetDynamicConfigRequest(t *types.GetDynamicConfigRequest) *adminv1.GetDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetDynamicConfigRequest{
		ConfigName: t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminGetDynamicConfigRequest converts proto GetDynamicConfigRequest type to internal
func ToAdminGetDynamicConfigRequest(t *adminv1.GetDynamicConfigRequest) *types.GetDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.GetDynamicConfigRequest{
		ConfigName: t.ConfigName,
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminGetDynamicConfigResponse converts internal GetDynamicConfigResponse type to proto
func FromAdminGetDynamicConfigResponse(t *types.GetDynamicConfigResponse) *adminv1.GetDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetDynamicConfigResponse{
		Value: FromDataBlob(t.Value),
	}
}

// ToAdminGetDynamicConfigResponse converts proto GetDynamicConfigResponse type to internal
func ToAdminGetDynamicConfigResponse(t *adminv1.GetDynamicConfigResponse) *types.GetDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.GetDynamicConfigResponse{
		Value: ToDataBlob(t.Value),
	}
}

// FromAdminUpdateDynamicConfigRequest converts internal UpdateDynamicConfigRequest type to proto
func FromAdminUpdateDynamicConfigRequest(t *types.UpdateDynamicConfigRequest) *adminv1.UpdateDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateDynamicConfigRequest{
		ConfigName:   t.ConfigName,
		ConfigValues: FromDynamicConfigValueArray(t.ConfigValues),
	}
}

// ToAdminUpdateDynamicConfigRequest converts proto UpdateDynamicConfigRequest type to internal
func ToAdminUpdateDynamicConfigRequest(t *adminv1.UpdateDynamicConfigRequest) *types.UpdateDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateDynamicConfigRequest{
		ConfigName:   t.ConfigName,
		ConfigValues: ToDynamicConfigValueArray(t.ConfigValues),
	}
}

// FromAdminRestoreDynamicConfigRequest converts internal RestoreDynamicConfigRequest type to proto
func FromAdminRestoreDynamicConfigRequest(t *types.RestoreDynamicConfigRequest) *adminv1.RestoreDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &adminv1.RestoreDynamicConfigRequest{
		ConfigName: t.ConfigName,
		Filters:    FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToAdminRestoreDynamicConfigRequest converts proto RestoreDynamicConfigRequest type to internal
func ToAdminRestoreDynamicConfigRequest(t *adminv1.RestoreDynamicConfigRequest) *types.RestoreDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.RestoreDynamicConfigRequest{
		ConfigName: t.ConfigName,
		Filters:    ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromAdminDeleteWorkflowRequest converts internal AdminDeleteWorkflowRequest type to proto
func FromAdminDeleteWorkflowRequest(t *types.AdminDeleteWorkflowRequest) *adminv1.DeleteWorkflowRequest {
	if t == nil {
		return nil
	}
	return &adminv1.DeleteWorkflowRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

// ToAdminDeleteWorkflowRequest converts proto AdminDeleteWorkflowRequest type to internal
func ToAdminDeleteWorkflowRequest(t *adminv1.DeleteWorkflowRequest) *types.AdminDeleteWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.AdminDeleteWorkflowRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

// FromAdminDeleteWorkflowResponse converts internal AdminDeleteWorkflowRequest type to proto
func FromAdminDeleteWorkflowResponse(t *types.AdminDeleteWorkflowResponse) *adminv1.DeleteWorkflowResponse {
	if t == nil {
		return nil
	}
	return &adminv1.DeleteWorkflowResponse{
		HistoryDeleted:    t.HistoryDeleted,
		ExecutionsDeleted: t.ExecutionsDeleted,
		VisibilityDeleted: t.VisibilityDeleted,
	}
}

// ToAdminDeleteWorkflowResponse converts proto AdminDeleteWorkflowResponse type to internal
func ToAdminDeleteWorkflowResponse(t *adminv1.DeleteWorkflowResponse) *types.AdminDeleteWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.AdminDeleteWorkflowResponse{
		HistoryDeleted:    t.HistoryDeleted,
		ExecutionsDeleted: t.ExecutionsDeleted,
		VisibilityDeleted: t.VisibilityDeleted,
	}
}

// FromAdminMaintainCorruptWorkflowRequest converts internal AdminMaintainWorkflowRequest type to proto
func FromAdminMaintainCorruptWorkflowRequest(t *types.AdminMaintainWorkflowRequest) *adminv1.MaintainCorruptWorkflowRequest {
	if t == nil {
		return nil
	}
	return &adminv1.MaintainCorruptWorkflowRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

// ToAdminMaintainCorruptWorkflowRequest converts proto AdminMaintainWorkflowRequest type to internal
func ToAdminMaintainCorruptWorkflowRequest(t *adminv1.MaintainCorruptWorkflowRequest) *types.AdminMaintainWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.AdminMaintainWorkflowRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

// FromAdminMaintainCorruptWorkflowResponse converts internal AdminMaintainWorkflowResponse type to proto
func FromAdminMaintainCorruptWorkflowResponse(t *types.AdminMaintainWorkflowResponse) *adminv1.MaintainCorruptWorkflowResponse {
	if t == nil {
		return nil
	}
	return &adminv1.MaintainCorruptWorkflowResponse{
		HistoryDeleted:    t.HistoryDeleted,
		ExecutionsDeleted: t.ExecutionsDeleted,
		VisibilityDeleted: t.VisibilityDeleted,
	}
}

// ToAdminMaintainCorruptWorkflowResponse converts proto AdminMaintainWorkflowResponse type to internal
func ToAdminMaintainCorruptWorkflowResponse(t *adminv1.MaintainCorruptWorkflowResponse) *types.AdminMaintainWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.AdminMaintainWorkflowResponse{
		HistoryDeleted:    t.HistoryDeleted,
		ExecutionsDeleted: t.ExecutionsDeleted,
		VisibilityDeleted: t.VisibilityDeleted,
	}
}

// FromAdminListDynamicConfigRequest converts internal ListDynamicConfigRequest type to proto
func FromAdminListDynamicConfigRequest(t *types.ListDynamicConfigRequest) *adminv1.ListDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &adminv1.ListDynamicConfigRequest{
		ConfigName: t.ConfigName,
	}
}

// ToAdminListDynamicConfigRequest converts proto ListDynamicConfigRequest type to internal
func ToAdminListDynamicConfigRequest(t *adminv1.ListDynamicConfigRequest) *types.ListDynamicConfigRequest {
	if t == nil {
		return nil
	}
	return &types.ListDynamicConfigRequest{
		ConfigName: t.ConfigName,
	}
}

// FromAdminListDynamicConfigResponse converts internal ListDynamicConfigResponse type to proto
func FromAdminListDynamicConfigResponse(t *types.ListDynamicConfigResponse) *adminv1.ListDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &adminv1.ListDynamicConfigResponse{
		Entries: FromDynamicConfigEntryArray(t.Entries),
	}
}

// ToAdminListDynamicConfigResponse converts proto ListDynamicConfigResponse type to internal
func ToAdminListDynamicConfigResponse(t *adminv1.ListDynamicConfigResponse) *types.ListDynamicConfigResponse {
	if t == nil {
		return nil
	}
	return &types.ListDynamicConfigResponse{
		Entries: ToDynamicConfigEntryArray(t.Entries),
	}
}

// FromDynamicConfigEntryArray converts internal DynamicConfigEntry array type to proto
func FromDynamicConfigEntryArray(t []*types.DynamicConfigEntry) []*adminv1.DynamicConfigEntry {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.DynamicConfigEntry, len(t))
	for i := range t {
		v[i] = FromDynamicConfigEntry(t[i])
	}
	return v
}

// ToDynamicConfigEntryArray converts proto DynamicConfigEntry array type to internal
func ToDynamicConfigEntryArray(t []*adminv1.DynamicConfigEntry) []*types.DynamicConfigEntry {
	if t == nil {
		return nil
	}
	v := make([]*types.DynamicConfigEntry, len(t))
	for i := range t {
		v[i] = ToDynamicConfigEntry(t[i])
	}
	return v
}

// FromDynamicConfigEntry converts internal DynamicConfigEntry type to proto
func FromDynamicConfigEntry(t *types.DynamicConfigEntry) *adminv1.DynamicConfigEntry {
	if t == nil {
		return nil
	}
	return &adminv1.DynamicConfigEntry{
		Name:   t.Name,
		Values: FromDynamicConfigValueArray(t.Values),
	}
}

// ToDynamicConfigEntry converts proto DynamicConfigEntry type to internal
func ToDynamicConfigEntry(t *adminv1.DynamicConfigEntry) *types.DynamicConfigEntry {
	if t == nil {
		return nil
	}
	return &types.DynamicConfigEntry{
		Name:   t.Name,
		Values: ToDynamicConfigValueArray(t.Values),
	}
}

// FromDynamicConfigValueArray converts internal DynamicConfigValue array type to proto
func FromDynamicConfigValueArray(t []*types.DynamicConfigValue) []*adminv1.DynamicConfigValue {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.DynamicConfigValue, len(t))
	for i := range t {
		v[i] = FromDynamicConfigValue(t[i])
	}
	return v
}

// ToDynamicConfigValueArray converts proto DynamicConfigValue array type to internal
func ToDynamicConfigValueArray(t []*adminv1.DynamicConfigValue) []*types.DynamicConfigValue {
	if t == nil {
		return nil
	}
	v := make([]*types.DynamicConfigValue, len(t))
	for i := range t {
		v[i] = ToDynamicConfigValue(t[i])
	}
	return v
}

// FromDynamicConfigValue converts internal DynamicConfigValue type to proto
func FromDynamicConfigValue(t *types.DynamicConfigValue) *adminv1.DynamicConfigValue {
	if t == nil {
		return nil
	}
	return &adminv1.DynamicConfigValue{
		Value:   FromDataBlob(t.Value),
		Filters: FromDynamicConfigFilterArray(t.Filters),
	}
}

// ToDynamicConfigValue converts proto DynamicConfigValue type to internal
func ToDynamicConfigValue(t *adminv1.DynamicConfigValue) *types.DynamicConfigValue {
	if t == nil {
		return nil
	}
	return &types.DynamicConfigValue{
		Value:   ToDataBlob(t.Value),
		Filters: ToDynamicConfigFilterArray(t.Filters),
	}
}

// FromDynamicConfigFilterArray converts internal DynamicConfigFilter array type to proto
func FromDynamicConfigFilterArray(t []*types.DynamicConfigFilter) []*adminv1.DynamicConfigFilter {
	if t == nil {
		return nil
	}
	v := make([]*adminv1.DynamicConfigFilter, len(t))
	for i := range t {
		v[i] = FromDynamicConfigFilter(t[i])
	}
	return v
}

// ToDynamicConfigFilterArray converts proto DynamicConfigFilter array type to internal
func ToDynamicConfigFilterArray(t []*adminv1.DynamicConfigFilter) []*types.DynamicConfigFilter {
	if t == nil {
		return nil
	}
	v := make([]*types.DynamicConfigFilter, len(t))
	for i := range t {
		v[i] = ToDynamicConfigFilter(t[i])
	}
	return v
}

// FromDynamicConfigFilter converts internal DynamicConfigFilter type to proto
func FromDynamicConfigFilter(t *types.DynamicConfigFilter) *adminv1.DynamicConfigFilter {
	if t == nil {
		return nil
	}
	return &adminv1.DynamicConfigFilter{
		Name:  t.Name,
		Value: FromDataBlob(t.Value),
	}
}

// ToDynamicConfigFilter converts thrift DynamicConfigFilter type to internal
func ToDynamicConfigFilter(t *adminv1.DynamicConfigFilter) *types.DynamicConfigFilter {
	if t == nil {
		return nil
	}
	return &types.DynamicConfigFilter{
		Name:  t.Name,
		Value: ToDataBlob(t.Value),
	}
}

func FromAdminGetGlobalIsolationGroupsResponse(t *types.GetGlobalIsolationGroupsResponse) *adminv1.GetGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &adminv1.GetGlobalIsolationGroupsResponse{
		IsolationGroups: FromIsolationGroupConfig(&t.IsolationGroups),
	}
}

func FromAdminUpdateGlobalIsolationGroupsRequest(t *types.UpdateGlobalIsolationGroupsRequest) *adminv1.UpdateGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: FromIsolationGroupConfig(&t.IsolationGroups),
	}
}

func FromAdminUpdateDomainIsolationGroupsRequest(t *types.UpdateDomainIsolationGroupsRequest) *adminv1.UpdateDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateDomainIsolationGroupsRequest{
		Domain:          t.Domain,
		IsolationGroups: FromIsolationGroupConfig(&t.IsolationGroups),
	}
}

func FromAdminGetGlobalIsolationGroupsRequest(t *types.GetGlobalIsolationGroupsRequest) *adminv1.GetGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetGlobalIsolationGroupsRequest{}
}

func FromAdminGetDomainIsolationGroupsRequest(t *types.GetDomainIsolationGroupsRequest) *adminv1.GetDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &adminv1.GetDomainIsolationGroupsRequest{
		Domain: t.Domain,
	}
}

func ToAdminGetGlobalIsolationGroupsRequest(t *adminv1.GetGlobalIsolationGroupsRequest) *types.GetGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &types.GetGlobalIsolationGroupsRequest{}
}

func ToAdminGetGlobalIsolationGroupsResponse(t *adminv1.GetGlobalIsolationGroupsResponse) *types.GetGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	ig := ToIsolationGroupConfig(t.IsolationGroups)
	if ig == nil {
		return &types.GetGlobalIsolationGroupsResponse{}
	}
	return &types.GetGlobalIsolationGroupsResponse{
		IsolationGroups: *ig,
	}
}

func ToAdminGetDomainIsolationGroupsResponse(t *adminv1.GetDomainIsolationGroupsResponse) *types.GetDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	ig := ToIsolationGroupConfig(t.IsolationGroups)
	if ig == nil {
		return &types.GetDomainIsolationGroupsResponse{}
	}
	return &types.GetDomainIsolationGroupsResponse{
		IsolationGroups: *ig,
	}
}

func FromAdminGetDomainIsolationGroupsResponse(t *types.GetDomainIsolationGroupsResponse) *adminv1.GetDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	cfg := FromIsolationGroupConfig(&t.IsolationGroups)
	return &adminv1.GetDomainIsolationGroupsResponse{
		IsolationGroups: cfg,
	}
}

func ToAdminGetDomainIsolationGroupsRequest(t *adminv1.GetDomainIsolationGroupsRequest) *types.GetDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	return &types.GetDomainIsolationGroupsRequest{Domain: t.Domain}
}

func FromAdminUpdateGlobalIsolationGroupsResponse(t *types.UpdateGlobalIsolationGroupsResponse) *adminv1.UpdateGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateGlobalIsolationGroupsResponse{}
}

func ToAdminUpdateGlobalIsolationGroupsRequest(t *adminv1.UpdateGlobalIsolationGroupsRequest) *types.UpdateGlobalIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	cfg := ToIsolationGroupConfig(t.IsolationGroups)
	if cfg == nil {
		return &types.UpdateGlobalIsolationGroupsRequest{}
	}
	return &types.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: *cfg,
	}
}

func ToAdminUpdateGlobalIsolationGroupsResponse(t *adminv1.UpdateGlobalIsolationGroupsResponse) *types.UpdateGlobalIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateGlobalIsolationGroupsResponse{}
}

func ToAdminUpdateDomainIsolationGroupsResponse(t *adminv1.UpdateDomainIsolationGroupsResponse) *types.UpdateDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateDomainIsolationGroupsResponse{}
}

func FromAdminUpdateDomainIsolationGroupsResponse(t *types.UpdateDomainIsolationGroupsResponse) *adminv1.UpdateDomainIsolationGroupsResponse {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateDomainIsolationGroupsResponse{}
}

func ToAdminUpdateDomainIsolationGroupsRequest(t *adminv1.UpdateDomainIsolationGroupsRequest) *types.UpdateDomainIsolationGroupsRequest {
	if t == nil {
		return nil
	}
	cfg := ToIsolationGroupConfig(t.IsolationGroups)
	if cfg == nil {
		return &types.UpdateDomainIsolationGroupsRequest{
			Domain: t.Domain,
		}
	}
	return &types.UpdateDomainIsolationGroupsRequest{
		Domain:          t.Domain,
		IsolationGroups: *cfg,
	}
}

func FromIsolationGroupConfig(in *types.IsolationGroupConfiguration) *apiv1.IsolationGroupConfiguration {
	if in == nil || *in == nil {
		return nil
	}
	var out []*apiv1.IsolationGroupPartition
	for _, v := range *in {
		out = append(out, &apiv1.IsolationGroupPartition{
			Name:  v.Name,
			State: apiv1.IsolationGroupState(v.State),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i] == nil || out[j] == nil {
			return false
		}
		return out[i].Name < out[j].Name
	})
	return &apiv1.IsolationGroupConfiguration{
		IsolationGroups: out,
	}
}

func ToIsolationGroupConfig(in *apiv1.IsolationGroupConfiguration) *types.IsolationGroupConfiguration {
	if in == nil {
		return nil
	}
	out := make(types.IsolationGroupConfiguration)
	for v := range in.IsolationGroups {
		out[in.IsolationGroups[v].Name] = types.IsolationGroupPartition{
			Name:  in.IsolationGroups[v].Name,
			State: types.IsolationGroupState(in.IsolationGroups[v].State),
		}
	}
	return &out
}

func ToAdminGetDomainAsyncWorkflowConfiguratonRequest(in *adminv1.GetDomainAsyncWorkflowConfiguratonRequest) *types.GetDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &types.GetDomainAsyncWorkflowConfiguratonRequest{
		Domain: in.Domain,
	}
}

func FromAdminGetDomainAsyncWorkflowConfiguratonResponse(in *types.GetDomainAsyncWorkflowConfiguratonResponse) *adminv1.GetDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &adminv1.GetDomainAsyncWorkflowConfiguratonResponse{
		Configuration: FromDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func FromDomainAsyncWorkflowConfiguraton(in *types.AsyncWorkflowConfiguration) *apiv1.AsyncWorkflowConfiguration {
	if in == nil {
		return nil
	}

	return &apiv1.AsyncWorkflowConfiguration{
		Enabled:             in.Enabled,
		PredefinedQueueName: in.PredefinedQueueName,
		QueueType:           in.QueueType,
		QueueConfig:         FromDataBlob(in.QueueConfig),
	}
}

func ToAdminUpdateDomainAsyncWorkflowConfiguratonRequest(in *adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest) *types.UpdateDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain:        in.Domain,
		Configuration: ToDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func ToDomainAsyncWorkflowConfiguraton(in *apiv1.AsyncWorkflowConfiguration) *types.AsyncWorkflowConfiguration {
	if in == nil {
		return nil
	}

	return &types.AsyncWorkflowConfiguration{
		Enabled:             in.Enabled,
		PredefinedQueueName: in.PredefinedQueueName,
		QueueType:           in.QueueType,
		QueueConfig:         ToDataBlob(in.QueueConfig),
	}
}

func FromAdminUpdateDomainAsyncWorkflowConfiguratonResponse(in *types.UpdateDomainAsyncWorkflowConfiguratonResponse) *adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse{}
}

func FromAdminGetDomainAsyncWorkflowConfiguratonRequest(in *types.GetDomainAsyncWorkflowConfiguratonRequest) *adminv1.GetDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &adminv1.GetDomainAsyncWorkflowConfiguratonRequest{
		Domain: in.Domain,
	}
}

func ToAdminGetDomainAsyncWorkflowConfiguratonResponse(in *adminv1.GetDomainAsyncWorkflowConfiguratonResponse) *types.GetDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &types.GetDomainAsyncWorkflowConfiguratonResponse{
		Configuration: ToDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func FromAdminUpdateDomainAsyncWorkflowConfiguratonRequest(in *types.UpdateDomainAsyncWorkflowConfiguratonRequest) *adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest {
	if in == nil {
		return nil
	}
	return &adminv1.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain:        in.Domain,
		Configuration: FromDomainAsyncWorkflowConfiguraton(in.Configuration),
	}
}

func ToAdminUpdateDomainAsyncWorkflowConfiguratonResponse(in *adminv1.UpdateDomainAsyncWorkflowConfiguratonResponse) *types.UpdateDomainAsyncWorkflowConfiguratonResponse {
	if in == nil {
		return nil
	}
	return &types.UpdateDomainAsyncWorkflowConfiguratonResponse{}
}

func FromAdminUpdateTaskListPartitionConfigRequest(in *types.UpdateTaskListPartitionConfigRequest) *adminv1.UpdateTaskListPartitionConfigRequest {
	if in == nil {
		return nil
	}
	return &adminv1.UpdateTaskListPartitionConfigRequest{
		Domain:          in.Domain,
		TaskList:        FromTaskList(in.TaskList),
		TaskListType:    FromTaskListType(in.TaskListType),
		PartitionConfig: FromAPITaskListPartitionConfig(in.PartitionConfig),
	}
}

func ToAdminUpdateTaskListPartitionConfigRequest(in *adminv1.UpdateTaskListPartitionConfigRequest) *types.UpdateTaskListPartitionConfigRequest {
	if in == nil {
		return nil
	}
	return &types.UpdateTaskListPartitionConfigRequest{
		Domain:          in.Domain,
		TaskList:        ToTaskList(in.TaskList),
		TaskListType:    ToTaskListType(in.TaskListType),
		PartitionConfig: ToAPITaskListPartitionConfig(in.PartitionConfig),
	}
}

func FromAdminUpdateTaskListPartitionConfigResponse(t *types.UpdateTaskListPartitionConfigResponse) *adminv1.UpdateTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &adminv1.UpdateTaskListPartitionConfigResponse{}
}

func ToAdminUpdateTaskListPartitionConfigResponse(t *adminv1.UpdateTaskListPartitionConfigResponse) *types.UpdateTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateTaskListPartitionConfigResponse{}
}
