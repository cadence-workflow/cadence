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

package testdata

import (
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

const (
	ForwardedFrom = "ForwardedFrom"
	PollerID      = "PollerID"
)

var (
	TaskListPartitionConfig = types.TaskListPartitionConfig{
		Version: 1,
		ReadPartitions: map[int]*types.TaskListPartition{
			0: {},
			1: {},
			2: {
				IsolationGroups: []string{"foo"},
			},
		},
		WritePartitions: map[int]*types.TaskListPartition{
			0: {},
			1: {
				IsolationGroups: []string{"bar"},
			},
		},
	}
	LoadBalancerHints = types.LoadBalancerHints{
		BacklogCount:  1000,
		RatePerSecond: 1.0,
	}
	MatchingAddActivityTaskRequest = types.AddActivityTaskRequest{
		DomainUUID:                    DomainID,
		Execution:                     &WorkflowExecution,
		SourceDomainUUID:              DomainID,
		TaskList:                      &TaskList,
		ScheduleID:                    EventID1,
		ScheduleToStartTimeoutSeconds: &Duration1,
		Source:                        types.TaskSourceDbBacklog.Ptr(),
		ForwardedFrom:                 ForwardedFrom,
		PartitionConfig:               PartitionConfig,
	}
	MatchingAddDecisionTaskRequest = types.AddDecisionTaskRequest{
		DomainUUID:                    DomainID,
		Execution:                     &WorkflowExecution,
		TaskList:                      &TaskList,
		ScheduleID:                    EventID1,
		ScheduleToStartTimeoutSeconds: &Duration1,
		Source:                        types.TaskSourceDbBacklog.Ptr(),
		ForwardedFrom:                 ForwardedFrom,
		PartitionConfig:               PartitionConfig,
	}
	MatchingAddActivityTaskResponse = types.AddActivityTaskResponse{
		PartitionConfig: &TaskListPartitionConfig,
	}
	MatchingAddDecisionTaskResponse = types.AddDecisionTaskResponse{
		PartitionConfig: &TaskListPartitionConfig,
	}
	MatchingCancelOutstandingPollRequest = types.CancelOutstandingPollRequest{
		DomainUUID:   DomainID,
		TaskListType: common.Int32Ptr(int32(TaskListType)),
		TaskList:     &TaskList,
		PollerID:     PollerID,
	}
	MatchingDescribeTaskListRequest = types.MatchingDescribeTaskListRequest{
		DomainUUID:  DomainID,
		DescRequest: &DescribeTaskListRequest,
	}
	MatchingDescribeTaskListResponse = types.DescribeTaskListResponse{
		Pollers:        PollerInfoArray,
		TaskListStatus: &TaskListStatus,
	}
	MatchingListTaskListPartitionsRequest = types.MatchingListTaskListPartitionsRequest{
		Domain:   DomainName,
		TaskList: &TaskList,
	}
	MatchingListTaskListPartitionsResponse = types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: TaskListPartitionMetadataArray,
		DecisionTaskListPartitions: TaskListPartitionMetadataArray,
	}
	MatchingPollForActivityTaskRequest = types.MatchingPollForActivityTaskRequest{
		DomainUUID:     DomainID,
		PollerID:       PollerID,
		PollRequest:    &PollForActivityTaskRequest,
		ForwardedFrom:  ForwardedFrom,
		IsolationGroup: IsolationGroup,
	}
	MatchingPollForActivityTaskResponse = types.MatchingPollForActivityTaskResponse{
		TaskToken:                       TaskToken,
		WorkflowExecution:               &WorkflowExecution,
		ActivityID:                      ActivityID,
		ActivityType:                    &ActivityType,
		Input:                           Payload1,
		ScheduledTimestamp:              &Timestamp1,
		ScheduleToCloseTimeoutSeconds:   &Duration1,
		StartedTimestamp:                &Timestamp2,
		StartToCloseTimeoutSeconds:      &Duration2,
		HeartbeatTimeoutSeconds:         &Duration3,
		Attempt:                         Attempt,
		ScheduledTimestampOfThisAttempt: &Timestamp3,
		HeartbeatDetails:                Payload2,
		WorkflowType:                    &WorkflowType,
		WorkflowDomain:                  DomainName,
		Header:                          &Header,
		PartitionConfig:                 &TaskListPartitionConfig,
		LoadBalancerHints:               &LoadBalancerHints,
		AutoConfigHint:                  &AutoConfigHint,
	}
	MatchingPollForDecisionTaskRequest = types.MatchingPollForDecisionTaskRequest{
		DomainUUID:     DomainID,
		PollerID:       PollerID,
		PollRequest:    &PollForDecisionTaskRequest,
		ForwardedFrom:  ForwardedFrom,
		IsolationGroup: IsolationGroup,
	}
	MatchingPollForDecisionTaskResponse = types.MatchingPollForDecisionTaskResponse{
		TaskToken:                 TaskToken,
		WorkflowExecution:         &WorkflowExecution,
		WorkflowType:              &WorkflowType,
		PreviousStartedEventID:    common.Int64Ptr(EventID1),
		StartedEventID:            EventID2,
		Attempt:                   Attempt,
		NextEventID:               EventID3,
		BacklogCountHint:          BacklogCountHint,
		StickyExecutionEnabled:    true,
		Query:                     &WorkflowQuery,
		DecisionInfo:              &TransientDecisionInfo,
		WorkflowExecutionTaskList: &TaskList,
		EventStoreVersion:         EventStoreVersion,
		BranchToken:               BranchToken,
		ScheduledTimestamp:        &Timestamp1,
		StartedTimestamp:          &Timestamp2,
		Queries:                   WorkflowQueryMap,
		PartitionConfig:           &TaskListPartitionConfig,
		LoadBalancerHints:         &LoadBalancerHints,
		AutoConfigHint:            &AutoConfigHint,
	}
	MatchingQueryWorkflowRequest = types.MatchingQueryWorkflowRequest{
		DomainUUID:    DomainID,
		TaskList:      &TaskList,
		QueryRequest:  &QueryWorkflowRequest,
		ForwardedFrom: ForwardedFrom,
	}
	MatchingQueryWorkflowResponse = types.QueryWorkflowResponse{
		QueryResult:   Payload1,
		QueryRejected: &QueryRejected,
	}
	MatchingRespondQueryTaskCompletedRequest = types.MatchingRespondQueryTaskCompletedRequest{
		DomainUUID:       DomainID,
		TaskList:         &TaskList,
		TaskID:           "TaskID",
		CompletedRequest: &RespondQueryTaskCompletedRequest,
	}
	MatchingGetTaskListsByDomainRequest = types.GetTaskListsByDomainRequest{
		Domain: DomainName,
	}

	GetTaskListsByDomainResponse = types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: DescribeTaskListResponseMap,
		ActivityTaskListMap: DescribeTaskListResponseMap,
	}

	DescribeTaskListResponseMap = map[string]*types.DescribeTaskListResponse{DomainName: &MatchingDescribeTaskListResponse}

	MatchingActivityTaskDispatchInfo = types.ActivityTaskDispatchInfo{
		ScheduledEvent:                  &HistoryEvent_WorkflowExecutionStarted,
		StartedTimestamp:                &Timestamp1,
		Attempt:                         &Attempt2,
		ScheduledTimestampOfThisAttempt: &Timestamp1,
		HeartbeatDetails:                Payload2,
		WorkflowType:                    &WorkflowType,
		WorkflowDomain:                  DomainName,
	}

	MatchingUpdateTaskListPartitionConfigRequest = types.MatchingUpdateTaskListPartitionConfigRequest{
		DomainUUID:      DomainID,
		TaskList:        &TaskList,
		TaskListType:    &TaskListType,
		PartitionConfig: &TaskListPartitionConfig,
	}

	MatchingRefreshTaskListPartitionConfigRequest = types.MatchingRefreshTaskListPartitionConfigRequest{
		DomainUUID:      DomainID,
		TaskList:        &TaskList,
		TaskListType:    &TaskListType,
		PartitionConfig: &TaskListPartitionConfig,
	}
)
