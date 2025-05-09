// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package timeout

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	publicservicetest "go.uber.org/cadence/.gen/go/cadence/workflowservicetest"
	"go.uber.org/cadence/.gen/go/shared"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

const (
	workflowTimeoutSecond = int32(110)
	taskTimeoutSecond     = int32(50)
	testTimeStamp         = int64(2547596872371000000)
	timeUnit              = time.Second
	testTasklist          = "test-tasklist"
	testDomain            = "test-domain"
	testTaskListBacklog   = int64(10)
)

func Test__Check(t *testing.T) {
	decisionTimeoutMetadata := DecisionTimeoutMetadata{ConfiguredTimeout: 50 * time.Second}
	decisionTimeoutMetadataInBytes, err := json.Marshal(decisionTimeoutMetadata)
	require.NoError(t, err)
	testCases := []struct {
		name           string
		testData       *types.GetWorkflowExecutionHistoryResponse
		expectedResult []invariant.InvariantCheckResult
		err            error
	}{
		{
			name:     "workflow execution timeout",
			testData: wfTimeoutHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeExecution.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      wfTimeoutDataInBytes(t),
				},
			},
			err: nil,
		},
		{
			name:     "child workflow execution timeout",
			testData: childWfTimeoutHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeChildWorkflow.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      childWfTimeoutDataInBytes(t),
				},
			},
			err: nil,
		},
		{
			name:     "activity timeout",
			testData: activityTimeoutHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "SCHEDULE_TO_START",
					Metadata:      activityScheduleToStartTimeoutDataInBytes(t),
				},
				{
					IssueID:       2,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "HEARTBEAT",
					Metadata:      activityHeartBeatTimeoutDataInBytes(t),
				},
			},
			err: nil,
		},
		{
			name:     "decision timeout",
			testData: decisionTimeoutHistory(),
			expectedResult: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeDecision.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      decisionTimeoutMetadataInBytes,
				},
				{
					IssueID:       2,
					InvariantType: TimeoutTypeDecision.String(),
					Reason:        "workflow reset - New run ID: new run ID",
					Metadata:      decisionTimeoutMetadataInBytes,
				},
			},
			err: nil,
		},
	}
	ctrl := gomock.NewController(t)
	mockClient := publicservicetest.NewMockClient(ctrl)
	for _, tc := range testCases {
		inv := NewInvariant(Params{
			Client: mockClient,
		})
		result, err := inv.Check(context.Background(), invariant.InvariantCheckInput{
			WorkflowExecutionHistory: tc.testData,
			Domain:                   testDomain,
		})
		require.Equal(t, tc.err, err)
		require.Equal(t, len(tc.expectedResult), len(result))
		for i := range result {
			require.Equal(t, tc.expectedResult[i], result[i])
		}

	}
}

func wfTimeoutHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					Timestamp: common.Int64Ptr(testTimeStamp),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
						TaskList: &types.TaskList{
							Name: testTasklist,
							Kind: nil,
						},
					},
				},
				{
					ID:                                       2,
					Timestamp:                                common.Int64Ptr(testTimeStamp + int64(workflowTimeoutSecond)*timeUnit.Nanoseconds()),
					WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{TimeoutType: types.TimeoutTypeStartToClose.Ptr()},
				},
			},
		},
	}
}

func childWfTimeoutHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 1,
					StartChildWorkflowExecutionInitiatedEventAttributes: &types.StartChildWorkflowExecutionInitiatedEventAttributes{
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
					},
				},
				{
					ID:        2,
					Timestamp: common.Int64Ptr(testTimeStamp),
					ChildWorkflowExecutionStartedEventAttributes: &types.ChildWorkflowExecutionStartedEventAttributes{
						InitiatedEventID: 1,
					},
				},
				{
					ID:        3,
					Timestamp: common.Int64Ptr(testTimeStamp + int64(workflowTimeoutSecond)*timeUnit.Nanoseconds()),
					ChildWorkflowExecutionTimedOutEventAttributes: &types.ChildWorkflowExecutionTimedOutEventAttributes{
						InitiatedEventID: 1,
						StartedEventID:   2,
						TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
						WorkflowExecution: &types.WorkflowExecution{
							WorkflowID: "123",
							RunID:      "abc",
						},
					},
				},
			},
		},
	}
}

func activityTimeoutHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					Timestamp: common.Int64Ptr(testTimeStamp),
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ScheduleToStartTimeoutSeconds: common.Int32Ptr(taskTimeoutSecond),
						TaskList: &types.TaskList{
							Name: testTasklist,
							Kind: nil,
						},
					},
				},
				{
					ID:        2,
					Timestamp: common.Int64Ptr(testTimeStamp + int64(taskTimeoutSecond)*timeUnit.Nanoseconds()),
					ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
						ScheduledEventID: 1,
						TimeoutType:      types.TimeoutTypeScheduleToStart.Ptr(),
					},
				},
				{
					ID: 3,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						HeartbeatTimeoutSeconds: common.Int32Ptr(taskTimeoutSecond),
						TaskList: &types.TaskList{
							Name: testTasklist,
							Kind: nil,
						},
					},
				},
				{
					ID:        4,
					Timestamp: common.Int64Ptr(testTimeStamp),
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						ScheduledEventID: 21,
					},
				},
				{
					ID:        5,
					Timestamp: common.Int64Ptr(testTimeStamp + int64(taskTimeoutSecond)*timeUnit.Nanoseconds()),
					ActivityTaskTimedOutEventAttributes: &types.ActivityTaskTimedOutEventAttributes{
						ScheduledEventID: 3,
						StartedEventID:   4,
						TimeoutType:      types.TimeoutTypeHeartbeat.Ptr(),
					},
				},
			},
		},
	}
}

func decisionTimeoutHistory() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 13,
					DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
						StartToCloseTimeoutSeconds: common.Int32Ptr(taskTimeoutSecond),
					},
				},
				{
					DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
						ScheduledEventID: 13,
						StartedEventID:   14,
						Cause:            types.DecisionTaskTimedOutCauseTimeout.Ptr(),
						TimeoutType:      types.TimeoutTypeStartToClose.Ptr(),
					},
				},
				{
					ID: 23,
					DecisionTaskScheduledEventAttributes: &types.DecisionTaskScheduledEventAttributes{
						StartToCloseTimeoutSeconds: common.Int32Ptr(taskTimeoutSecond),
					},
				},
				{
					DecisionTaskTimedOutEventAttributes: &types.DecisionTaskTimedOutEventAttributes{
						ScheduledEventID: 23,
						Cause:            types.DecisionTaskTimedOutCauseReset.Ptr(),
						Reason:           "workflow reset",
						NewRunID:         "new run ID",
					},
				},
			},
		},
	}
}

func wfTimeoutDataInBytes(t *testing.T) []byte {
	data := ExecutionTimeoutMetadata{
		ExecutionTime:     110 * time.Second,
		ConfiguredTimeout: 110 * time.Second,
		LastOngoingEvent: &types.HistoryEvent{
			ID:        1,
			Timestamp: common.Int64Ptr(testTimeStamp),
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
				TaskList: &types.TaskList{
					Name: testTasklist,
					Kind: nil,
				},
			},
		},
		Tasklist: &types.TaskList{
			Name: testTasklist,
			Kind: nil,
		},
	}
	dataInBytes, err := json.Marshal(data)
	require.NoError(t, err)
	return dataInBytes
}

func activityScheduleToStartTimeoutData() ActivityTimeoutMetadata {
	return ActivityTimeoutMetadata{
		TimeoutType:       types.TimeoutTypeScheduleToStart.Ptr(),
		ConfiguredTimeout: 50 * time.Second,
		TimeElapsed:       50 * time.Second,
		RetryPolicy:       nil,
		HeartBeatTimeout:  0,
		Tasklist: &types.TaskList{
			Name: testTasklist,
			Kind: nil,
		},
	}
}

func activityStartToCloseTimeoutData() ActivityTimeoutMetadata {
	return ActivityTimeoutMetadata{
		TimeoutType:       types.TimeoutTypeStartToClose.Ptr(),
		ConfiguredTimeout: 50 * time.Second,
		TimeElapsed:       50 * time.Second,
		RetryPolicy:       nil,
		HeartBeatTimeout:  0,
		Tasklist: &types.TaskList{
			Name: testTasklist,
			Kind: nil,
		},
	}
}

func activityScheduleToStartTimeoutDataInBytes(t *testing.T) []byte {
	data := activityScheduleToStartTimeoutData()
	dataInBytes, err := json.Marshal(data)
	require.NoError(t, err)
	return dataInBytes
}

func activityStartToCloseTimeoutDataInBytes(t *testing.T) []byte {
	data := activityStartToCloseTimeoutData()
	dataInBytes, err := json.Marshal(data)
	require.NoError(t, err)
	return dataInBytes
}

func activityHeartBeatTimeoutDataInBytes(t *testing.T) []byte {
	actTimeoutData := activityStartToCloseTimeoutData()
	actTimeoutData.TimeoutType = types.TimeoutTypeHeartbeat.Ptr()
	actTimeoutData.HeartBeatTimeout = 50 * time.Second
	actHeartBeatTimeoutDataInBytes, err := json.Marshal(actTimeoutData)
	require.NoError(t, err)
	return actHeartBeatTimeoutDataInBytes
}

func activityHeartBeatTimeoutDataWithRetryPolicyInBytes(t *testing.T) []byte {
	actTimeoutData := activityStartToCloseTimeoutData()
	actTimeoutData.TimeoutType = types.TimeoutTypeHeartbeat.Ptr()
	actTimeoutData.HeartBeatTimeout = 50 * time.Second
	actTimeoutData.RetryPolicy = &types.RetryPolicy{
		MaximumAttempts: 3,
	}
	actHeartBeatTimeoutDataInBytes, err := json.Marshal(actTimeoutData)
	require.NoError(t, err)
	return actHeartBeatTimeoutDataInBytes
}

func childWfTimeoutDataInBytes(t *testing.T) []byte {
	data := ChildWfTimeoutMetadata{
		ExecutionTime:     110 * time.Second,
		ConfiguredTimeout: 110 * time.Second,
		Execution: &types.WorkflowExecution{
			WorkflowID: "123",
			RunID:      "abc",
		},
	}
	dataInBytes, err := json.Marshal(data)
	require.NoError(t, err)
	return dataInBytes
}

func Test__RootCause(t *testing.T) {
	actStartToCloseTimeoutData := activityStartToCloseTimeoutData()
	pollersMetadataInBytes, err := json.Marshal(PollersMetadata{TaskListName: testTasklist, TaskListBacklog: testTaskListBacklog})
	require.NoError(t, err)
	heartBeatingMetadataInBytes, err := json.Marshal(HeartbeatingMetadata{TimeElapsed: actStartToCloseTimeoutData.TimeElapsed})
	require.NoError(t, err)
	heartBeatingMetadataWithRetryPolicyInBytes, err := json.Marshal(HeartbeatingMetadata{TimeElapsed: actStartToCloseTimeoutData.TimeElapsed, RetryPolicy: &types.RetryPolicy{MaximumAttempts: 3}})
	require.NoError(t, err)
	testCases := []struct {
		name           string
		input          []invariant.InvariantCheckResult
		clientExpects  func(client *publicservicetest.MockClient)
		expectedResult []invariant.InvariantRootCauseResult
		err            error
	}{
		{
			name: "workflow execution timeout without pollers",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeExecution.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      wfTimeoutDataInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: nil,
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypeMissingPollers,
					Metadata:  pollersMetadataInBytes,
				},
			},
			err: nil,
		},
		{
			name: "workflow execution timeout with pollers",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeExecution.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      wfTimeoutDataInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: []*shared.PollerInfo{
						{
							Identity: common.StringPtr("dca24-xy"),
						},
					},
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypePollersStatus,
					Metadata:  pollersMetadataInBytes,
				},
			},
			err: nil,
		},
		{
			name: "activity timeout and heart beating not enabled",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      activityStartToCloseTimeoutDataInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: []*shared.PollerInfo{
						{
							Identity: common.StringPtr("dca24-xy"),
						},
					},
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypePollersStatus,
					Metadata:  pollersMetadataInBytes,
				},
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypeNoHeartBeatTimeoutNoRetryPolicy,
					Metadata:  heartBeatingMetadataInBytes,
				},
			},
			err: nil,
		},
		{
			name: "activity schedule to start timeout",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "SCHEDULE_TO_START",
					Metadata:      activityScheduleToStartTimeoutDataInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: []*shared.PollerInfo{
						{
							Identity: common.StringPtr("dca24-xy"),
						},
					},
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypePollersStatus,
					Metadata:  pollersMetadataInBytes,
				},
			},
			err: nil,
		},
		{
			name: "activity timeout and heart beating enabled with retry policy",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      activityHeartBeatTimeoutDataWithRetryPolicyInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: []*shared.PollerInfo{
						{
							Identity: common.StringPtr("dca24-xy"),
						},
					},
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypePollersStatus,
					Metadata:  pollersMetadataInBytes,
				},
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypeHeartBeatingEnabledMissingHeartbeat,
					Metadata:  heartBeatingMetadataWithRetryPolicyInBytes,
				},
			},
			err: nil,
		},
		{
			name: "activity timeout and heart beating enabled without retry policy",
			input: []invariant.InvariantCheckResult{
				{
					IssueID:       1,
					InvariantType: TimeoutTypeActivity.String(),
					Reason:        "START_TO_CLOSE",
					Metadata:      activityHeartBeatTimeoutDataInBytes(t),
				},
			},
			clientExpects: func(client *publicservicetest.MockClient) {
				client.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(&shared.DescribeTaskListResponse{
					Pollers: []*shared.PollerInfo{
						{
							Identity: common.StringPtr("dca24-xy"),
						},
					},
					TaskListStatus: &shared.TaskListStatus{
						BacklogCountHint: common.Int64Ptr(testTaskListBacklog),
					},
				}, nil)
			},
			expectedResult: []invariant.InvariantRootCauseResult{
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypePollersStatus,
					Metadata:  pollersMetadataInBytes,
				},
				{
					IssueID:   1,
					RootCause: invariant.RootCauseTypeHeartBeatingEnabledWithoutRetryPolicy,
					Metadata:  heartBeatingMetadataInBytes,
				},
			},
			err: nil,
		},
	}
	ctrl := gomock.NewController(t)
	mockClient := publicservicetest.NewMockClient(ctrl)
	inv := NewInvariant(Params{
		Client: mockClient,
	})
	for _, tc := range testCases {
		tc.clientExpects(mockClient)
		result, err := inv.RootCause(context.Background(), invariant.InvariantRootCauseInput{
			Domain: testDomain,
			Issues: tc.input,
		})
		require.Equal(t, tc.err, err)
		require.Equal(t, len(tc.expectedResult), len(result))
		for i := range result {
			require.Equal(t, tc.expectedResult[i], result[i])
		}

	}
}
