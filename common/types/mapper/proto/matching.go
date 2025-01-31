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
	matchingv1 "github.com/uber/cadence/.gen/proto/matching/v1"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func FromMatchingAddActivityTaskRequest(t *types.AddActivityTaskRequest) *matchingv1.AddActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.AddActivityTaskRequest{
		DomainId:                 t.DomainUUID,
		WorkflowExecution:        FromWorkflowExecution(t.Execution),
		SourceDomainId:           t.SourceDomainUUID,
		TaskList:                 FromTaskList(t.TaskList),
		ScheduleId:               t.ScheduleID,
		ScheduleToStartTimeout:   secondsToDuration(t.ScheduleToStartTimeoutSeconds),
		Source:                   FromTaskSource(t.Source),
		ForwardedFrom:            t.ForwardedFrom,
		ActivityTaskDispatchInfo: FromActivityTaskDispatchInfo(t.ActivityTaskDispatchInfo),
		PartitionConfig:          t.PartitionConfig,
	}
}

func ToMatchingAddActivityTaskRequest(t *matchingv1.AddActivityTaskRequest) *types.AddActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.AddActivityTaskRequest{
		DomainUUID:                    t.DomainId,
		Execution:                     ToWorkflowExecution(t.WorkflowExecution),
		SourceDomainUUID:              t.SourceDomainId,
		TaskList:                      ToTaskList(t.TaskList),
		ScheduleID:                    t.ScheduleId,
		ScheduleToStartTimeoutSeconds: durationToSeconds(t.ScheduleToStartTimeout),
		Source:                        ToTaskSource(t.Source),
		ForwardedFrom:                 t.ForwardedFrom,
		ActivityTaskDispatchInfo:      ToActivityTaskDispatchInfo(t.ActivityTaskDispatchInfo),
		PartitionConfig:               t.PartitionConfig,
	}
}

func FromMatchingAddDecisionTaskRequest(t *types.AddDecisionTaskRequest) *matchingv1.AddDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.AddDecisionTaskRequest{
		DomainId:               t.DomainUUID,
		WorkflowExecution:      FromWorkflowExecution(t.Execution),
		TaskList:               FromTaskList(t.TaskList),
		ScheduleId:             t.ScheduleID,
		ScheduleToStartTimeout: secondsToDuration(t.ScheduleToStartTimeoutSeconds),
		Source:                 FromTaskSource(t.Source),
		ForwardedFrom:          t.ForwardedFrom,
		PartitionConfig:        t.PartitionConfig,
	}
}

func ToMatchingAddDecisionTaskRequest(t *matchingv1.AddDecisionTaskRequest) *types.AddDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.AddDecisionTaskRequest{
		DomainUUID:                    t.DomainId,
		Execution:                     ToWorkflowExecution(t.WorkflowExecution),
		TaskList:                      ToTaskList(t.TaskList),
		ScheduleID:                    t.ScheduleId,
		ScheduleToStartTimeoutSeconds: durationToSeconds(t.ScheduleToStartTimeout),
		Source:                        ToTaskSource(t.Source),
		ForwardedFrom:                 t.ForwardedFrom,
		PartitionConfig:               t.PartitionConfig,
	}
}

func FromMatchingAddActivityTaskResponse(t *types.AddActivityTaskResponse) *matchingv1.AddActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.AddActivityTaskResponse{
		PartitionConfig: FromTaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToMatchingAddActivityTaskResponse(t *matchingv1.AddActivityTaskResponse) *types.AddActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &types.AddActivityTaskResponse{
		PartitionConfig: ToTaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromMatchingAddDecisionTaskResponse(t *types.AddDecisionTaskResponse) *matchingv1.AddDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.AddDecisionTaskResponse{
		PartitionConfig: FromTaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToMatchingAddDecisionTaskResponse(t *matchingv1.AddDecisionTaskResponse) *types.AddDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &types.AddDecisionTaskResponse{
		PartitionConfig: ToTaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromActivityTaskDispatchInfo(t *types.ActivityTaskDispatchInfo) *matchingv1.ActivityTaskDispatchInfo {
	if t == nil {
		return nil
	}
	return &matchingv1.ActivityTaskDispatchInfo{
		ScheduledEvent:             FromHistoryEvent(t.ScheduledEvent),
		StartedTime:                unixNanoToTime(t.StartedTimestamp),
		Attempt:                    int32(common.Int64Default(t.Attempt)),
		ScheduledTimeOfThisAttempt: unixNanoToTime(t.ScheduledTimestampOfThisAttempt),
		HeartbeatDetails:           FromPayload(t.HeartbeatDetails),
		WorkflowType:               FromWorkflowType(t.WorkflowType),
		WorkflowDomain:             t.WorkflowDomain,
	}
}

func ToActivityTaskDispatchInfo(t *matchingv1.ActivityTaskDispatchInfo) *types.ActivityTaskDispatchInfo {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskDispatchInfo{
		ScheduledEvent:                  ToHistoryEvent(t.ScheduledEvent),
		StartedTimestamp:                timeToUnixNano(t.StartedTime),
		Attempt:                         common.Int64Ptr(int64(t.Attempt)),
		ScheduledTimestampOfThisAttempt: timeToUnixNano(t.ScheduledTimeOfThisAttempt),
		HeartbeatDetails:                ToPayload(t.HeartbeatDetails),
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.WorkflowDomain,
	}
}

func FromMatchingCancelOutstandingPollRequest(t *types.CancelOutstandingPollRequest) *matchingv1.CancelOutstandingPollRequest {
	if t == nil {
		return nil
	}
	var taskListType *types.TaskListType
	if t.TaskListType != nil {
		taskListType = types.TaskListType(*t.TaskListType).Ptr()
	}
	return &matchingv1.CancelOutstandingPollRequest{
		DomainId:     t.DomainUUID,
		PollerId:     t.PollerID,
		TaskListType: FromTaskListType(taskListType),
		TaskList:     FromTaskList(t.TaskList),
	}
}

func ToMatchingCancelOutstandingPollRequest(t *matchingv1.CancelOutstandingPollRequest) *types.CancelOutstandingPollRequest {
	if t == nil {
		return nil
	}
	var taskListType *int32
	if tlt := ToTaskListType(t.TaskListType); tlt != nil {
		taskListType = common.Int32Ptr(int32(*tlt))
	}
	return &types.CancelOutstandingPollRequest{
		DomainUUID:   t.DomainId,
		PollerID:     t.PollerId,
		TaskListType: taskListType,
		TaskList:     ToTaskList(t.TaskList),
	}
}

func FromMatchingDescribeTaskListRequest(t *types.MatchingDescribeTaskListRequest) *matchingv1.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.DescribeTaskListRequest{
		Request:  FromDescribeTaskListRequest(t.DescRequest),
		DomainId: t.DomainUUID,
	}
}

func ToMatchingDescribeTaskListRequest(t *matchingv1.DescribeTaskListRequest) *types.MatchingDescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingDescribeTaskListRequest{
		DescRequest: ToDescribeTaskListRequest(t.Request),
		DomainUUID:  t.DomainId,
	}
}

func FromMatchingDescribeTaskListResponse(t *types.DescribeTaskListResponse) *matchingv1.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.DescribeTaskListResponse{
		Pollers:         FromPollerInfoArray(t.Pollers),
		TaskListStatus:  FromTaskListStatus(t.TaskListStatus),
		PartitionConfig: FromAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToMatchingDescribeTaskListResponse(t *matchingv1.DescribeTaskListResponse) *types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeTaskListResponse{
		Pollers:         ToPollerInfoArray(t.Pollers),
		TaskListStatus:  ToTaskListStatus(t.TaskListStatus),
		PartitionConfig: ToAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromMatchingListTaskListPartitionsRequest(t *types.MatchingListTaskListPartitionsRequest) *matchingv1.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.ListTaskListPartitionsRequest{
		Domain:   t.Domain,
		TaskList: FromTaskList(t.TaskList),
	}
}

func ToMatchingListTaskListPartitionsRequest(t *matchingv1.ListTaskListPartitionsRequest) *types.MatchingListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingListTaskListPartitionsRequest{
		Domain:   t.Domain,
		TaskList: ToTaskList(t.TaskList),
	}
}

func FromMatchingListTaskListPartitionsResponse(t *types.ListTaskListPartitionsResponse) *matchingv1.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: FromTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: FromTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

func ToMatchingListTaskListPartitionsResponse(t *matchingv1.ListTaskListPartitionsResponse) *types.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: ToTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: ToTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

func FromMatchingGetTaskListsByDomainRequest(t *types.GetTaskListsByDomainRequest) *matchingv1.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.GetTaskListsByDomainRequest{
		Domain: t.Domain,
	}
}

func ToMatchingGetTaskListsByDomainRequest(t *matchingv1.GetTaskListsByDomainRequest) *types.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &types.GetTaskListsByDomainRequest{
		Domain: t.Domain,
	}
}

func FromMatchingGetTaskListsByDomainResponse(t *types.GetTaskListsByDomainResponse) *matchingv1.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &matchingv1.GetTaskListsByDomainResponse{
		DecisionTaskListMap: FromMatchingDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: FromMatchingDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

func ToMatchingGetTaskListsByDomainResponse(t *matchingv1.GetTaskListsByDomainResponse) *types.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: ToMatchingDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: ToMatchingDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

func FromMatchingDescribeTaskListResponseMap(t map[string]*types.DescribeTaskListResponse) map[string]*matchingv1.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*matchingv1.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = FromMatchingDescribeTaskListResponse(value)
	}
	return taskListMap
}

func ToMatchingDescribeTaskListResponseMap(t map[string]*matchingv1.DescribeTaskListResponse) map[string]*types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*types.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = ToMatchingDescribeTaskListResponse(value)
	}
	return taskListMap
}

func FromMatchingPollForActivityTaskRequest(t *types.MatchingPollForActivityTaskRequest) *matchingv1.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.PollForActivityTaskRequest{
		Request:        FromPollForActivityTaskRequest(t.PollRequest),
		DomainId:       t.DomainUUID,
		PollerId:       t.PollerID,
		ForwardedFrom:  t.ForwardedFrom,
		IsolationGroup: t.IsolationGroup,
	}
}

func ToMatchingPollForActivityTaskRequest(t *matchingv1.PollForActivityTaskRequest) *types.MatchingPollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForActivityTaskRequest{
		PollRequest:    ToPollForActivityTaskRequest(t.Request),
		DomainUUID:     t.DomainId,
		PollerID:       t.PollerId,
		ForwardedFrom:  t.ForwardedFrom,
		IsolationGroup: t.IsolationGroup,
	}
}

func FromTaskListPartitionConfig(t *types.TaskListPartitionConfig) *matchingv1.TaskListPartitionConfig {
	if t == nil {
		return nil
	}
	return &matchingv1.TaskListPartitionConfig{
		Version:            t.Version,
		NumReadPartitions:  int32(len(t.ReadPartitions)),
		NumWritePartitions: int32(len(t.WritePartitions)),
		ReadPartitions:     FromMatchingTaskListPartitionsMap(t.ReadPartitions),
		WritePartitions:    FromMatchingTaskListPartitionsMap(t.WritePartitions),
	}
}

func ToTaskListPartitionConfig(t *matchingv1.TaskListPartitionConfig) *types.TaskListPartitionConfig {
	if t == nil {
		return nil
	}
	return &types.TaskListPartitionConfig{
		Version:         t.Version,
		ReadPartitions:  ToMatchingTaskListPartitionsMap(t.NumReadPartitions, t.ReadPartitions),
		WritePartitions: ToMatchingTaskListPartitionsMap(t.NumWritePartitions, t.WritePartitions),
	}
}

func FromMatchingTaskListPartition(t *types.TaskListPartition) *matchingv1.TaskListPartition {
	if t == nil {
		return nil
	}
	return &matchingv1.TaskListPartition{
		IsolationGroups: t.IsolationGroups,
	}
}

func ToMatchingTaskListPartition(t *matchingv1.TaskListPartition) *types.TaskListPartition {
	if t == nil {
		return nil
	}
	return &types.TaskListPartition{
		IsolationGroups: t.IsolationGroups,
	}
}

func FromMatchingTaskListPartitionsMap(m map[int]*types.TaskListPartition) map[int32]*matchingv1.TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int32]*matchingv1.TaskListPartition, len(m))
	for id, p := range m {
		result[int32(id)] = FromMatchingTaskListPartition(p)
	}
	return result
}

func ToMatchingTaskListPartitionsMap(numPartitions int32, m map[int32]*matchingv1.TaskListPartition) map[int]*types.TaskListPartition {
	if m == nil && numPartitions == 0 {
		return nil
	}
	result := make(map[int]*types.TaskListPartition, len(m))
	if numPartitions != int32(len(m)) {
		for i := int32(0); i < numPartitions; i++ {
			result[int(i)] = &types.TaskListPartition{}
		}
	} else {
		for id, p := range m {
			result[int(id)] = ToMatchingTaskListPartition(p)
		}
	}
	return result
}

func FromMatchingPollForActivityTaskResponse(t *types.MatchingPollForActivityTaskResponse) *matchingv1.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.PollForActivityTaskResponse{
		TaskToken:                  t.TaskToken,
		WorkflowExecution:          FromWorkflowExecution(t.WorkflowExecution),
		ActivityId:                 t.ActivityID,
		ActivityType:               FromActivityType(t.ActivityType),
		Input:                      FromPayload(t.Input),
		ScheduledTime:              unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:                unixNanoToTime(t.StartedTimestamp),
		ScheduleToCloseTimeout:     secondsToDuration(t.ScheduleToCloseTimeoutSeconds),
		StartToCloseTimeout:        secondsToDuration(t.StartToCloseTimeoutSeconds),
		HeartbeatTimeout:           secondsToDuration(t.HeartbeatTimeoutSeconds),
		Attempt:                    t.Attempt,
		ScheduledTimeOfThisAttempt: unixNanoToTime(t.ScheduledTimestampOfThisAttempt),
		HeartbeatDetails:           FromPayload(t.HeartbeatDetails),
		WorkflowType:               FromWorkflowType(t.WorkflowType),
		WorkflowDomain:             t.WorkflowDomain,
		Header:                     FromHeader(t.Header),
		PartitionConfig:            FromTaskListPartitionConfig(t.PartitionConfig),
		LoadBalancerHints:          FromLoadBalancerHints(t.LoadBalancerHints),
		AutoConfigHint:             FromAutoConfigHint(t.AutoConfigHint),
	}
}

func ToMatchingPollForActivityTaskResponse(t *matchingv1.PollForActivityTaskResponse) *types.MatchingPollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               ToWorkflowExecution(t.WorkflowExecution),
		ActivityID:                      t.ActivityId,
		ActivityType:                    ToActivityType(t.ActivityType),
		Input:                           ToPayload(t.Input),
		ScheduledTimestamp:              timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:                timeToUnixNano(t.StartedTime),
		ScheduleToCloseTimeoutSeconds:   durationToSeconds(t.ScheduleToCloseTimeout),
		StartToCloseTimeoutSeconds:      durationToSeconds(t.StartToCloseTimeout),
		HeartbeatTimeoutSeconds:         durationToSeconds(t.HeartbeatTimeout),
		Attempt:                         t.Attempt,
		ScheduledTimestampOfThisAttempt: timeToUnixNano(t.ScheduledTimeOfThisAttempt),
		HeartbeatDetails:                ToPayload(t.HeartbeatDetails),
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.WorkflowDomain,
		Header:                          ToHeader(t.Header),
		PartitionConfig:                 ToTaskListPartitionConfig(t.PartitionConfig),
		LoadBalancerHints:               ToLoadBalancerHints(t.LoadBalancerHints),
		AutoConfigHint:                  ToAutoConfigHint(t.AutoConfigHint),
	}
}

func FromMatchingPollForDecisionTaskRequest(t *types.MatchingPollForDecisionTaskRequest) *matchingv1.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.PollForDecisionTaskRequest{
		Request:        FromPollForDecisionTaskRequest(t.PollRequest),
		DomainId:       t.DomainUUID,
		PollerId:       t.PollerID,
		ForwardedFrom:  t.ForwardedFrom,
		IsolationGroup: t.IsolationGroup,
	}
}

func ToMatchingPollForDecisionTaskRequest(t *matchingv1.PollForDecisionTaskRequest) *types.MatchingPollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForDecisionTaskRequest{
		PollRequest:    ToPollForDecisionTaskRequest(t.Request),
		DomainUUID:     t.DomainId,
		PollerID:       t.PollerId,
		ForwardedFrom:  t.ForwardedFrom,
		IsolationGroup: t.IsolationGroup,
	}
}

func FromMatchingPollForDecisionTaskResponse(t *types.MatchingPollForDecisionTaskResponse) *matchingv1.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              FromWorkflowType(t.WorkflowType),
		PreviousStartedEventId:    fromInt64Value(t.PreviousStartedEventID),
		StartedEventId:            t.StartedEventID,
		Attempt:                   int32(t.Attempt),
		NextEventId:               t.NextEventID,
		BacklogCountHint:          t.BacklogCountHint,
		StickyExecutionEnabled:    t.StickyExecutionEnabled,
		Query:                     FromWorkflowQuery(t.Query),
		DecisionInfo:              FromTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: FromTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         t.EventStoreVersion,
		BranchToken:               t.BranchToken,
		ScheduledTime:             unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:               unixNanoToTime(t.StartedTimestamp),
		Queries:                   FromWorkflowQueryMap(t.Queries),
		TotalHistoryBytes:         t.TotalHistoryBytes,
		PartitionConfig:           FromTaskListPartitionConfig(t.PartitionConfig),
		LoadBalancerHints:         FromLoadBalancerHints(t.LoadBalancerHints),
		AutoConfigHint:            FromAutoConfigHint(t.AutoConfigHint),
	}
}

func ToMatchingPollForDecisionTaskResponse(t *matchingv1.PollForDecisionTaskResponse) *types.MatchingPollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              ToWorkflowType(t.WorkflowType),
		PreviousStartedEventID:    toInt64Value(t.PreviousStartedEventId),
		StartedEventID:            t.StartedEventId,
		Attempt:                   int64(t.Attempt),
		NextEventID:               t.NextEventId,
		BacklogCountHint:          t.BacklogCountHint,
		StickyExecutionEnabled:    t.StickyExecutionEnabled,
		Query:                     ToWorkflowQuery(t.Query),
		DecisionInfo:              ToTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: ToTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         t.EventStoreVersion,
		BranchToken:               t.BranchToken,
		ScheduledTimestamp:        timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:          timeToUnixNano(t.StartedTime),
		Queries:                   ToWorkflowQueryMap(t.Queries),
		TotalHistoryBytes:         t.TotalHistoryBytes,
		PartitionConfig:           ToTaskListPartitionConfig(t.PartitionConfig),
		LoadBalancerHints:         ToLoadBalancerHints(t.LoadBalancerHints),
		AutoConfigHint:            ToAutoConfigHint(t.AutoConfigHint),
	}
}

func FromMatchingQueryWorkflowRequest(t *types.MatchingQueryWorkflowRequest) *matchingv1.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.QueryWorkflowRequest{
		Request:       FromQueryWorkflowRequest(t.QueryRequest),
		DomainId:      t.DomainUUID,
		TaskList:      FromTaskList(t.TaskList),
		ForwardedFrom: t.ForwardedFrom,
	}
}

func ToMatchingQueryWorkflowRequest(t *matchingv1.QueryWorkflowRequest) *types.MatchingQueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingQueryWorkflowRequest{
		QueryRequest:  ToQueryWorkflowRequest(t.Request),
		DomainUUID:    t.DomainId,
		TaskList:      ToTaskList(t.TaskList),
		ForwardedFrom: t.ForwardedFrom,
	}
}

func FromMatchingQueryWorkflowResponse(t *types.QueryWorkflowResponse) *matchingv1.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.QueryWorkflowResponse{
		QueryResult:   FromPayload(t.QueryResult),
		QueryRejected: FromQueryRejected(t.QueryRejected),
	}
}

func ToMatchingQueryWorkflowResponse(t *matchingv1.QueryWorkflowResponse) *types.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.QueryWorkflowResponse{
		QueryResult:   ToPayload(t.QueryResult),
		QueryRejected: ToQueryRejected(t.QueryRejected),
	}
}

func FromMatchingRespondQueryTaskCompletedRequest(t *types.MatchingRespondQueryTaskCompletedRequest) *matchingv1.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.RespondQueryTaskCompletedRequest{
		Request:  FromRespondQueryTaskCompletedRequest(t.CompletedRequest),
		DomainId: t.DomainUUID,
		TaskList: FromTaskList(t.TaskList),
		TaskId:   t.TaskID,
	}
}

func ToMatchingRespondQueryTaskCompletedRequest(t *matchingv1.RespondQueryTaskCompletedRequest) *types.MatchingRespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingRespondQueryTaskCompletedRequest{
		CompletedRequest: ToRespondQueryTaskCompletedRequest(t.Request),
		DomainUUID:       t.DomainId,
		TaskList:         ToTaskList(t.TaskList),
		TaskID:           t.TaskId,
	}
}

func FromMatchingUpdateTaskListPartitionConfigRequest(t *types.MatchingUpdateTaskListPartitionConfigRequest) *matchingv1.UpdateTaskListPartitionConfigRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.UpdateTaskListPartitionConfigRequest{
		DomainId:        t.DomainUUID,
		TaskList:        FromTaskList(t.TaskList),
		TaskListType:    FromTaskListType(t.TaskListType),
		PartitionConfig: FromAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToMatchingUpdateTaskListPartitionConfigRequest(t *matchingv1.UpdateTaskListPartitionConfigRequest) *types.MatchingUpdateTaskListPartitionConfigRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingUpdateTaskListPartitionConfigRequest{
		DomainUUID:      t.DomainId,
		TaskList:        ToTaskList(t.TaskList),
		TaskListType:    ToTaskListType(t.TaskListType),
		PartitionConfig: ToAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromMatchingRefreshTaskListPartitionConfigRequest(t *types.MatchingRefreshTaskListPartitionConfigRequest) *matchingv1.RefreshTaskListPartitionConfigRequest {
	if t == nil {
		return nil
	}
	return &matchingv1.RefreshTaskListPartitionConfigRequest{
		DomainId:        t.DomainUUID,
		TaskList:        FromTaskList(t.TaskList),
		TaskListType:    FromTaskListType(t.TaskListType),
		PartitionConfig: FromAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToMatchingRefreshTaskListPartitionConfigRequest(t *matchingv1.RefreshTaskListPartitionConfigRequest) *types.MatchingRefreshTaskListPartitionConfigRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingRefreshTaskListPartitionConfigRequest{
		DomainUUID:      t.DomainId,
		TaskList:        ToTaskList(t.TaskList),
		TaskListType:    ToTaskListType(t.TaskListType),
		PartitionConfig: ToAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromMatchingUpdateTaskListPartitionConfigResponse(t *types.MatchingUpdateTaskListPartitionConfigResponse) *matchingv1.UpdateTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.UpdateTaskListPartitionConfigResponse{}
}

func ToMatchingUpdateTaskListPartitionConfigResponse(t *matchingv1.UpdateTaskListPartitionConfigResponse) *types.MatchingUpdateTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingUpdateTaskListPartitionConfigResponse{}
}

func FromMatchingRefreshTaskListPartitionConfigResponse(t *types.MatchingRefreshTaskListPartitionConfigResponse) *matchingv1.RefreshTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &matchingv1.RefreshTaskListPartitionConfigResponse{}
}

func ToMatchingRefreshTaskListPartitionConfigResponse(t *matchingv1.RefreshTaskListPartitionConfigResponse) *types.MatchingRefreshTaskListPartitionConfigResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingRefreshTaskListPartitionConfigResponse{}
}

func FromLoadBalancerHints(t *types.LoadBalancerHints) *matchingv1.LoadBalancerHints {
	if t == nil {
		return nil
	}
	return &matchingv1.LoadBalancerHints{
		BacklogCount:  t.BacklogCount,
		RatePerSecond: t.RatePerSecond,
	}
}

func ToLoadBalancerHints(t *matchingv1.LoadBalancerHints) *types.LoadBalancerHints {
	if t == nil {
		return nil
	}
	return &types.LoadBalancerHints{
		BacklogCount:  t.BacklogCount,
		RatePerSecond: t.RatePerSecond,
	}
}
