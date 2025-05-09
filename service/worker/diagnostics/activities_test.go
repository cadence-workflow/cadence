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

package diagnostics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/analytics"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/failure"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/retry"
)

const (
	workflowTimeoutSecond = int32(110)
	testTimeStamp         = int64(2547596872371000000)
	timeUnit              = time.Second
)

func Test__identifyIssues(t *testing.T) {
	dwtest := testDiagnosticWorkflow(t)
	actMetadata := failure.FailureIssuesMetadata{
		Identity:            "localhost",
		ActivityType:        "test-activity",
		ActivityScheduledID: 2,
		ActivityStartedID:   3,
	}
	actMetadataInBytes, err := json.Marshal(actMetadata)
	require.NoError(t, err)
	retryMetadata := retry.RetryMetadata{
		EventID: 2,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds: 1,
			MaximumAttempts:          1,
		},
	}
	retryMetadataInBytes, err := json.Marshal(retryMetadata)
	require.NoError(t, err)
	expectedResult := []invariant.InvariantCheckResult{
		{
			IssueID:       1,
			InvariantType: failure.ActivityFailed.String(),
			Reason:        failure.GenericError.String(),
			Metadata:      actMetadataInBytes,
		},
		{
			IssueID:       1,
			InvariantType: retry.ActivityRetryIssue.String(),
			Reason:        retry.RetryPolicyValidationMaxAttempts.String(),
			Metadata:      retryMetadataInBytes,
		},
	}
	result, err := dwtest.identifyIssues(context.Background(), identifyIssuesParams{Execution: &types.WorkflowExecution{
		WorkflowID: "123",
		RunID:      "abc",
	}})
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func Test__rootCauseIssues(t *testing.T) {
	dwtest := testDiagnosticWorkflow(t)
	actMetadata := failure.FailureIssuesMetadata{
		Identity:            "localhost",
		ActivityScheduledID: 1,
		ActivityStartedID:   2,
	}
	actMetadataInBytes, err := json.Marshal(actMetadata)
	require.NoError(t, err)
	issues := []invariant.InvariantCheckResult{
		{
			IssueID:       1,
			InvariantType: failure.ActivityFailed.String(),
			Reason:        failure.CustomError.String(),
			Metadata:      actMetadataInBytes,
		},
	}
	expectedRootCause := []invariant.InvariantRootCauseResult{
		{
			IssueID:   1,
			RootCause: invariant.RootCauseTypeServiceSideCustomError,
			Metadata:  actMetadataInBytes,
		},
	}
	result, err := dwtest.rootCauseIssues(context.Background(), rootCauseIssuesParams{Domain: "test-domain", Issues: issues})
	require.NoError(t, err)
	require.Equal(t, expectedRootCause, result)
}

func Test__emit(t *testing.T) {
	ctrl := gomock.NewController(t)
	dwtest := testDiagnosticWorkflow(t)
	mockClient := messaging.NewMockClient(ctrl)
	mockProducer := messaging.NewMockProducer(ctrl)
	mockProducer.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil)
	mockClient.EXPECT().NewProducer(WfDiagnosticsAppName).Return(mockProducer, nil)
	err := dwtest.emit(context.Background(), analytics.WfDiagnosticsUsageData{}, mockClient)
	require.NoError(t, err)
}

func testDiagnosticWorkflow(t *testing.T) *dw {
	ctrl := gomock.NewController(t)
	mockClientBean := client.NewMockBean(ctrl)
	mockFrontendClient := frontend.NewMockClient(ctrl)
	mockClientBean.EXPECT().GetFrontendClient().Return(mockFrontendClient).AnyTimes()
	mockFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(testWorkflowExecutionHistoryResponse(), nil).AnyTimes()
	return &dw{
		clientBean: mockClientBean,
		invariants: []invariant.Invariant{failure.NewInvariant(), retry.NewInvariant()},
	}
}

func testWorkflowExecutionHistoryResponse() *types.GetWorkflowExecutionHistoryResponse {
	return &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					Timestamp: common.Int64Ptr(testTimeStamp),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(workflowTimeoutSecond),
					},
				},
				{
					ID: 2,
					ActivityTaskScheduledEventAttributes: &types.ActivityTaskScheduledEventAttributes{
						ActivityID:   "101",
						ActivityType: &types.ActivityType{Name: "test-activity"},
						RetryPolicy: &types.RetryPolicy{
							InitialIntervalInSeconds: 1,
							MaximumAttempts:          1,
						},
					},
				},
				{
					ID: 3,
					ActivityTaskStartedEventAttributes: &types.ActivityTaskStartedEventAttributes{
						Identity: "localhost",
						Attempt:  0,
					},
				},
				{
					ID:        4,
					Timestamp: common.Int64Ptr(testTimeStamp),
					ActivityTaskFailedEventAttributes: &types.ActivityTaskFailedEventAttributes{
						Reason:           common.StringPtr("cadenceInternal:Generic"),
						Details:          []byte("test-activity-failure"),
						Identity:         "localhost",
						ScheduledEventID: 2,
						StartedEventID:   3,
					},
				},
				{
					ID:                                       5,
					Timestamp:                                common.Int64Ptr(testTimeStamp + int64(workflowTimeoutSecond)*timeUnit.Nanoseconds()),
					WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{TimeoutType: types.TimeoutTypeStartToClose.Ptr()},
				},
			},
		},
	}
}
