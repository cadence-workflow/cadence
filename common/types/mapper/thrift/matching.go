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

package thrift

import (
	"github.com/uber/cadence/.gen/go/matching"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

var (
	FromMatchingDescribeTaskListResponse       = FromDescribeTaskListResponse
	ToMatchingDescribeTaskListResponse         = ToDescribeTaskListResponse
	FromMatchingGetTaskListsByDomainRequest    = FromGetTaskListsByDomainRequest
	ToMatchingGetTaskListsByDomainRequest      = ToGetTaskListsByDomainRequest
	FromMatchingGetTaskListsByDomainResponse   = FromGetTaskListsByDomainResponse
	ToMatchingGetTaskListsByDomainResponse     = ToGetTaskListsByDomainResponse
	FromMatchingListTaskListPartitionsResponse = FromListTaskListPartitionsResponse
	ToMatchingListTaskListPartitionsResponse   = ToListTaskListPartitionsResponse
	FromMatchingQueryWorkflowResponse          = FromQueryWorkflowResponse
	ToMatchingQueryWorkflowResponse            = ToQueryWorkflowResponse
)

// FromMatchingAddActivityTaskRequest converts internal AddActivityTaskRequest type to thrift
func FromMatchingAddActivityTaskRequest(t *types.AddActivityTaskRequest) *matching.AddActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &matching.AddActivityTaskRequest{
		DomainUUID:                    &t.DomainUUID,
		Execution:                     FromWorkflowExecution(t.Execution),
		SourceDomainUUID:              &t.SourceDomainUUID,
		TaskList:                      FromTaskList(t.TaskList),
		ScheduleId:                    &t.ScheduleID,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		Source:                        FromTaskSource(t.Source),
		ForwardedFrom:                 &t.ForwardedFrom,
		ActivityTaskDispatchInfo:      FromActivityTaskDispatchInfo(t.ActivityTaskDispatchInfo),
		PartitionConfig:               t.PartitionConfig,
	}
}

// ToMatchingAddActivityTaskRequest converts thrift AddActivityTaskRequest type to internal
func ToMatchingAddActivityTaskRequest(t *matching.AddActivityTaskRequest) *types.AddActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.AddActivityTaskRequest{
		DomainUUID:                    t.GetDomainUUID(),
		Execution:                     ToWorkflowExecution(t.Execution),
		SourceDomainUUID:              t.GetSourceDomainUUID(),
		TaskList:                      ToTaskList(t.TaskList),
		ScheduleID:                    t.GetScheduleId(),
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		Source:                        ToTaskSource(t.Source),
		ForwardedFrom:                 t.GetForwardedFrom(),
		ActivityTaskDispatchInfo:      ToActivityTaskDispatchInfo(t.ActivityTaskDispatchInfo),
		PartitionConfig:               t.PartitionConfig,
	}
}

func FromActivityTaskDispatchInfo(t *types.ActivityTaskDispatchInfo) *matching.ActivityTaskDispatchInfo {
	if t == nil {
		return nil
	}
	return &matching.ActivityTaskDispatchInfo{
		ScheduledEvent:                  FromHistoryEvent(t.ScheduledEvent),
		StartedTimestamp:                t.StartedTimestamp,
		Attempt:                         t.Attempt,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    FromWorkflowType(t.WorkflowType),
		WorkflowDomain:                  &t.WorkflowDomain,
	}
}

// ToRecordActivityTaskStartedResponse converts thrift RecordActivityTaskStartedResponse type to internal
func ToActivityTaskDispatchInfo(t *matching.ActivityTaskDispatchInfo) *types.ActivityTaskDispatchInfo {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskDispatchInfo{
		ScheduledEvent:                  ToHistoryEvent(t.ScheduledEvent),
		StartedTimestamp:                t.StartedTimestamp,
		Attempt:                         t.Attempt,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  common.StringDefault(t.WorkflowDomain),
	}
}

// FromMatchingAddDecisionTaskRequest converts internal AddDecisionTaskRequest type to thrift
func FromMatchingAddDecisionTaskRequest(t *types.AddDecisionTaskRequest) *matching.AddDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &matching.AddDecisionTaskRequest{
		DomainUUID:                    &t.DomainUUID,
		Execution:                     FromWorkflowExecution(t.Execution),
		TaskList:                      FromTaskList(t.TaskList),
		ScheduleId:                    &t.ScheduleID,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		Source:                        FromTaskSource(t.Source),
		ForwardedFrom:                 &t.ForwardedFrom,
		PartitionConfig:               t.PartitionConfig,
	}
}

// ToMatchingAddDecisionTaskRequest converts thrift AddDecisionTaskRequest type to internal
func ToMatchingAddDecisionTaskRequest(t *matching.AddDecisionTaskRequest) *types.AddDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.AddDecisionTaskRequest{
		DomainUUID:                    t.GetDomainUUID(),
		Execution:                     ToWorkflowExecution(t.Execution),
		TaskList:                      ToTaskList(t.TaskList),
		ScheduleID:                    t.GetScheduleId(),
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		Source:                        ToTaskSource(t.Source),
		ForwardedFrom:                 t.GetForwardedFrom(),
		PartitionConfig:               t.PartitionConfig,
	}
}

// FromMatchingCancelOutstandingPollRequest converts internal CancelOutstandingPollRequest type to thrift
func FromMatchingCancelOutstandingPollRequest(t *types.CancelOutstandingPollRequest) *matching.CancelOutstandingPollRequest {
	if t == nil {
		return nil
	}
	return &matching.CancelOutstandingPollRequest{
		DomainUUID:   &t.DomainUUID,
		TaskListType: t.TaskListType,
		TaskList:     FromTaskList(t.TaskList),
		PollerID:     &t.PollerID,
	}
}

// ToMatchingCancelOutstandingPollRequest converts thrift CancelOutstandingPollRequest type to internal
func ToMatchingCancelOutstandingPollRequest(t *matching.CancelOutstandingPollRequest) *types.CancelOutstandingPollRequest {
	if t == nil {
		return nil
	}
	return &types.CancelOutstandingPollRequest{
		DomainUUID:   t.GetDomainUUID(),
		TaskListType: t.TaskListType,
		TaskList:     ToTaskList(t.TaskList),
		PollerID:     t.GetPollerID(),
	}
}

// FromMatchingDescribeTaskListRequest converts internal DescribeTaskListRequest type to thrift
func FromMatchingDescribeTaskListRequest(t *types.MatchingDescribeTaskListRequest) *matching.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &matching.DescribeTaskListRequest{
		DomainUUID:  &t.DomainUUID,
		DescRequest: FromDescribeTaskListRequest(t.DescRequest),
	}
}

// ToMatchingDescribeTaskListRequest converts thrift DescribeTaskListRequest type to internal
func ToMatchingDescribeTaskListRequest(t *matching.DescribeTaskListRequest) *types.MatchingDescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingDescribeTaskListRequest{
		DomainUUID:  t.GetDomainUUID(),
		DescRequest: ToDescribeTaskListRequest(t.DescRequest),
	}
}

// FromMatchingListTaskListPartitionsRequest converts internal ListTaskListPartitionsRequest type to thrift
func FromMatchingListTaskListPartitionsRequest(t *types.MatchingListTaskListPartitionsRequest) *matching.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &matching.ListTaskListPartitionsRequest{
		Domain:   &t.Domain,
		TaskList: FromTaskList(t.TaskList),
	}
}

// ToMatchingListTaskListPartitionsRequest converts thrift ListTaskListPartitionsRequest type to internal
func ToMatchingListTaskListPartitionsRequest(t *matching.ListTaskListPartitionsRequest) *types.MatchingListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingListTaskListPartitionsRequest{
		Domain:   t.GetDomain(),
		TaskList: ToTaskList(t.TaskList),
	}
}

// FromMatchingPollForActivityTaskRequest converts internal PollForActivityTaskRequest type to thrift
func FromMatchingPollForActivityTaskRequest(t *types.MatchingPollForActivityTaskRequest) *matching.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &matching.PollForActivityTaskRequest{
		DomainUUID:     &t.DomainUUID,
		PollerID:       &t.PollerID,
		PollRequest:    FromPollForActivityTaskRequest(t.PollRequest),
		ForwardedFrom:  &t.ForwardedFrom,
		IsolationGroup: &t.IsolationGroup,
	}
}

// ToMatchingPollForActivityTaskRequest converts thrift PollForActivityTaskRequest type to internal
func ToMatchingPollForActivityTaskRequest(t *matching.PollForActivityTaskRequest) *types.MatchingPollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForActivityTaskRequest{
		DomainUUID:     t.GetDomainUUID(),
		PollerID:       t.GetPollerID(),
		PollRequest:    ToPollForActivityTaskRequest(t.PollRequest),
		ForwardedFrom:  t.GetForwardedFrom(),
		IsolationGroup: t.GetIsolationGroup(),
	}
}

// FromMatchingPollForDecisionTaskRequest converts internal PollForDecisionTaskRequest type to thrift
func FromMatchingPollForDecisionTaskRequest(t *types.MatchingPollForDecisionTaskRequest) *matching.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &matching.PollForDecisionTaskRequest{
		DomainUUID:     &t.DomainUUID,
		PollerID:       &t.PollerID,
		PollRequest:    FromPollForDecisionTaskRequest(t.PollRequest),
		ForwardedFrom:  &t.ForwardedFrom,
		IsolationGroup: &t.IsolationGroup,
	}
}

// ToMatchingPollForDecisionTaskRequest converts thrift PollForDecisionTaskRequest type to internal
func ToMatchingPollForDecisionTaskRequest(t *matching.PollForDecisionTaskRequest) *types.MatchingPollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForDecisionTaskRequest{
		DomainUUID:     t.GetDomainUUID(),
		PollerID:       t.GetPollerID(),
		PollRequest:    ToPollForDecisionTaskRequest(t.PollRequest),
		ForwardedFrom:  t.GetForwardedFrom(),
		IsolationGroup: t.GetIsolationGroup(),
	}
}

// FromMatchingPollForDecisionTaskResponse converts internal PollForDecisionTaskResponse type to thrift
func FromMatchingPollForDecisionTaskResponse(t *types.MatchingPollForDecisionTaskResponse) *matching.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &matching.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              FromWorkflowType(t.WorkflowType),
		PreviousStartedEventId:    t.PreviousStartedEventID,
		StartedEventId:            &t.StartedEventID,
		Attempt:                   &t.Attempt,
		NextEventId:               &t.NextEventID,
		BacklogCountHint:          &t.BacklogCountHint,
		StickyExecutionEnabled:    &t.StickyExecutionEnabled,
		Query:                     FromWorkflowQuery(t.Query),
		DecisionInfo:              FromTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: FromTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         &t.EventStoreVersion,
		BranchToken:               t.BranchToken,
		ScheduledTimestamp:        t.ScheduledTimestamp,
		StartedTimestamp:          t.StartedTimestamp,
		Queries:                   FromWorkflowQueryMap(t.Queries),
		TotalHistoryBytes:         &t.TotalHistoryBytes,
		AutoConfigHint:            FromAutoConfigHint(t.AutoConfigHint),
	}
}

// ToMatchingPollForDecisionTaskResponse converts thrift PollForDecisionTaskResponse type to internal
func ToMatchingPollForDecisionTaskResponse(t *matching.PollForDecisionTaskResponse) *types.MatchingPollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              ToWorkflowType(t.WorkflowType),
		PreviousStartedEventID:    t.PreviousStartedEventId,
		StartedEventID:            t.GetStartedEventId(),
		Attempt:                   t.GetAttempt(),
		NextEventID:               t.GetNextEventId(),
		BacklogCountHint:          t.GetBacklogCountHint(),
		StickyExecutionEnabled:    t.GetStickyExecutionEnabled(),
		Query:                     ToWorkflowQuery(t.Query),
		DecisionInfo:              ToTransientDecisionInfo(t.DecisionInfo),
		WorkflowExecutionTaskList: ToTaskList(t.WorkflowExecutionTaskList),
		EventStoreVersion:         t.GetEventStoreVersion(),
		BranchToken:               t.BranchToken,
		ScheduledTimestamp:        t.ScheduledTimestamp,
		StartedTimestamp:          t.StartedTimestamp,
		Queries:                   ToWorkflowQueryMap(t.Queries),
		TotalHistoryBytes:         t.GetTotalHistoryBytes(),
		AutoConfigHint:            ToAutoConfigHint(t.AutoConfigHint),
	}
}

func FromMatchingPollForActivityTaskResponse(t *types.MatchingPollForActivityTaskResponse) *shared.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &shared.PollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               FromWorkflowExecution(t.WorkflowExecution),
		ActivityId:                      &t.ActivityID,
		ActivityType:                    FromActivityType(t.ActivityType),
		Input:                           t.Input,
		ScheduledTimestamp:              t.ScheduledTimestamp,
		ScheduleToCloseTimeoutSeconds:   t.ScheduleToCloseTimeoutSeconds,
		StartedTimestamp:                t.StartedTimestamp,
		StartToCloseTimeoutSeconds:      t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:         t.HeartbeatTimeoutSeconds,
		Attempt:                         &t.Attempt,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    FromWorkflowType(t.WorkflowType),
		WorkflowDomain:                  &t.WorkflowDomain,
		Header:                          FromHeader(t.Header),
		AutoConfigHint:                  FromAutoConfigHint(t.AutoConfigHint),
	}
}

func ToMatchingPollForActivityTaskResponse(t *shared.PollForActivityTaskResponse) *types.MatchingPollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &types.MatchingPollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               ToWorkflowExecution(t.WorkflowExecution),
		ActivityID:                      t.GetActivityId(),
		ActivityType:                    ToActivityType(t.ActivityType),
		Input:                           t.Input,
		ScheduledTimestamp:              t.ScheduledTimestamp,
		ScheduleToCloseTimeoutSeconds:   t.ScheduleToCloseTimeoutSeconds,
		StartedTimestamp:                t.StartedTimestamp,
		StartToCloseTimeoutSeconds:      t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:         t.HeartbeatTimeoutSeconds,
		Attempt:                         t.GetAttempt(),
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.GetWorkflowDomain(),
		Header:                          ToHeader(t.Header),
		AutoConfigHint:                  ToAutoConfigHint(t.AutoConfigHint),
	}
}

// FromMatchingQueryWorkflowRequest converts internal QueryWorkflowRequest type to thrift
func FromMatchingQueryWorkflowRequest(t *types.MatchingQueryWorkflowRequest) *matching.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &matching.QueryWorkflowRequest{
		DomainUUID:    &t.DomainUUID,
		TaskList:      FromTaskList(t.TaskList),
		QueryRequest:  FromQueryWorkflowRequest(t.QueryRequest),
		ForwardedFrom: &t.ForwardedFrom,
	}
}

// ToMatchingQueryWorkflowRequest converts thrift QueryWorkflowRequest type to internal
func ToMatchingQueryWorkflowRequest(t *matching.QueryWorkflowRequest) *types.MatchingQueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingQueryWorkflowRequest{
		DomainUUID:    t.GetDomainUUID(),
		TaskList:      ToTaskList(t.TaskList),
		QueryRequest:  ToQueryWorkflowRequest(t.QueryRequest),
		ForwardedFrom: t.GetForwardedFrom(),
	}
}

// FromMatchingRespondQueryTaskCompletedRequest converts internal RespondQueryTaskCompletedRequest type to thrift
func FromMatchingRespondQueryTaskCompletedRequest(t *types.MatchingRespondQueryTaskCompletedRequest) *matching.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &matching.RespondQueryTaskCompletedRequest{
		DomainUUID:       &t.DomainUUID,
		TaskList:         FromTaskList(t.TaskList),
		TaskID:           &t.TaskID,
		CompletedRequest: FromRespondQueryTaskCompletedRequest(t.CompletedRequest),
	}
}

// ToMatchingRespondQueryTaskCompletedRequest converts thrift RespondQueryTaskCompletedRequest type to internal
func ToMatchingRespondQueryTaskCompletedRequest(t *matching.RespondQueryTaskCompletedRequest) *types.MatchingRespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.MatchingRespondQueryTaskCompletedRequest{
		DomainUUID:       t.GetDomainUUID(),
		TaskList:         ToTaskList(t.TaskList),
		TaskID:           t.GetTaskID(),
		CompletedRequest: ToRespondQueryTaskCompletedRequest(t.CompletedRequest),
	}
}

// FromTaskSource converts internal TaskSource type to thrift
func FromTaskSource(t *types.TaskSource) *matching.TaskSource {
	if t == nil {
		return nil
	}
	switch *t {
	case types.TaskSourceHistory:
		v := matching.TaskSourceHistory
		return &v
	case types.TaskSourceDbBacklog:
		v := matching.TaskSourceDbBacklog
		return &v
	}
	panic("unexpected enum value")
}

// ToTaskSource converts thrift TaskSource type to internal
func ToTaskSource(t *matching.TaskSource) *types.TaskSource {
	if t == nil {
		return nil
	}
	switch *t {
	case matching.TaskSourceHistory:
		v := types.TaskSourceHistory
		return &v
	case matching.TaskSourceDbBacklog:
		v := types.TaskSourceDbBacklog
		return &v
	}
	panic("unexpected enum value")
}
