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

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestConstructStartWorkflowRequest(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String(FlagTaskList, "test-task-list", "tasklist")
	set.String(FlagWorkflowType, "test-workflow-type", "workflow-type")
	set.Int("execution_timeout", 100, "execution_timeout")
	set.Int("decision_timeout", 50, "decision_timeout")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.Int("workflow_id_reuse_policy", 1, "workflow_id_reuse_policy")
	set.String("input", "{}", "input")
	set.String("cron_schedule", "* * * * *", "cron_schedule")
	set.Int("retry_attempts", 5, "retry_attempts")
	set.Int("retry_expiration", 600, "retry_expiration")
	set.Int("retry_interval", 10, "retry_interval")
	set.Float64("retry_backoff", 2.0, "retry_backoff")
	set.Int("retry_max_interval", 100, "retry_max_interval")
	set.Int(DelayStartSeconds, 5, DelayStartSeconds)
	set.Int(JitterStartSeconds, 2, JitterStartSeconds)
	set.String("first_run_at_time", "2024-07-24T12:00:00Z", "first-run-at-time")
	set.Int("cron_overlap_policy", 0, "cron_overlap_policy")

	c := cli.NewContext(nil, set, nil)
	// inject context with span
	tracer := mocktracer.New()
	span, ctx := opentracing.StartSpanFromContextWithTracer(context.Background(), tracer, "test-span")
	span.SetBaggageItem("tracer-test-key", "tracer-test-value")
	defer span.Finish()
	c.Context = ctx

	assert.NoError(t, c.Set(FlagDomain, "test-domain"))
	assert.NoError(t, c.Set(FlagTaskList, "test-task-list"))
	assert.NoError(t, c.Set(FlagWorkflowType, "test-workflow-type"))
	assert.NoError(t, c.Set("execution_timeout", "100"))
	assert.NoError(t, c.Set("decision_timeout", "50"))
	assert.NoError(t, c.Set("workflow_id", "test-workflow-id"))
	assert.NoError(t, c.Set("workflow_id_reuse_policy", "1"))
	assert.NoError(t, c.Set("input", "{}"))
	assert.NoError(t, c.Set("cron_schedule", "* * * * *"))
	assert.NoError(t, c.Set("retry_attempts", "5"))
	assert.NoError(t, c.Set("retry_expiration", "600"))
	assert.NoError(t, c.Set("retry_interval", "10"))
	assert.NoError(t, c.Set("retry_backoff", "2.0"))
	assert.NoError(t, c.Set("retry_max_interval", "100"))
	assert.NoError(t, c.Set(DelayStartSeconds, "5"))
	assert.NoError(t, c.Set(JitterStartSeconds, "2"))
	assert.NoError(t, c.Set("first_run_at_time", "2024-07-24T12:00:00Z"))
	assert.NoError(t, c.Set("cron_overlap_policy", "0"))
	request, err := constructStartWorkflowRequest(c)
	assert.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, "test-domain", request.Domain)
	assert.Equal(t, "test-task-list", request.TaskList.Name)
	assert.Equal(t, "test-workflow-type", request.WorkflowType.Name)
	assert.Equal(t, int32(100), *request.ExecutionStartToCloseTimeoutSeconds)
	assert.Equal(t, int32(50), *request.TaskStartToCloseTimeoutSeconds)
	assert.Equal(t, "test-workflow-id", request.WorkflowID)
	assert.NotNil(t, request.WorkflowIDReusePolicy)
	assert.Equal(t, int32(5), *request.DelayStartSeconds)
	assert.Equal(t, int32(2), *request.JitterStartSeconds)
	assert.Contains(t, request.Header.Fields, "mockpfx-baggage-tracer-test-key")
	assert.Equal(t, []byte("tracer-test-value"), request.Header.Fields["mockpfx-baggage-tracer-test-key"])

	firstRunAt, err := time.Parse(time.RFC3339, "2024-07-24T12:00:00Z")
	assert.NoError(t, err)
	assert.Equal(t, firstRunAt.UnixNano(), *request.FirstRunAtTimeStamp)
	assert.Equal(t, types.CronOverlapPolicySkipped, *request.CronOverlapPolicy)
}

func Test_PrintAutoResetPoints(t *testing.T) {
	tests := []struct {
		name string
		resp *types.DescribeWorkflowExecutionResponse
	}{
		{
			name: "empty reset points",
			resp: &types.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
			},
		},
		{
			name: "normal case",
			resp: &types.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
					AutoResetPoints: &types.ResetPoints{
						Points: []*types.ResetPointInfo{
							{
								BinaryChecksum: "test-binary-checksum",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := printAutoResetPoints(tt.resp)
			assert.NoError(t, err)
		})
	}
}

func Test_DescribeWorkflow(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
	}, nil).Times(1)
	c := getMockContext(t, nil, app)
	err := DescribeWorkflow(c)
	assert.NoError(t, err)
}

func Test_DescribeWorkflowWithID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
	}, nil).Times(1)
	c := getMockContext(t, nil, app)
	err := DescribeWorkflowWithID(c)
	assert.NoError(t, err)
}

func Test_DescribeWorkflowWithID_Error(t *testing.T) {
	set := flag.NewFlagSet("test", 0)
	err := DescribeWorkflowWithID(cli.NewContext(nil, set, nil))
	assert.Error(t, err)

	// WF helper describe failed
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			SearchAttributes: &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					"CustomKeywordField": []byte("test-value"),
				},
			},
		},
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(nil, errors.New("test-error")).Times(1)

	c := getMockContext(t, nil, app)
	err = DescribeWorkflowWithID(c)
	assert.Error(t, err)
}

func getMockContext(t *testing.T, set *flag.FlagSet, app *cli.App) *cli.Context {
	if set == nil {
		set = flag.NewFlagSet("test", 0)
		set.String(FlagDomain, "test-domain", "domain")
		set.String("workflow_id", "test-workflow-id", "workflow_id")
		set.String("run_id", "test-run-id", "run_id")
		set.Bool("print_reset_points", true, "print_reset_points")
		set.Parse([]string{"test-workflow-id", "test-run-id"})
	}

	c := cli.NewContext(app, set, nil)

	return c
}

func Test_ListAllWorkflow(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.CountWorkflowExecutionsResponse{
		Count: int64(1),
	}, nil).AnyTimes()
	serverFrontendClient.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListWorkflowExecutionsResponse{}, nil).AnyTimes()
	serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).AnyTimes()
	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.String("run_id", "test-run-id", "run_id")
	set.String("status", "open", "status")
	c := getMockContext(t, set, app)
	err := ListAllWorkflow(c)
	assert.NoError(t, err)
}

func Test_ConvertSearchAttributesToMapOfInterface(t *testing.T) {
	tests := []struct {
		name          string
		in            *types.SearchAttributes
		out           map[string]interface{}
		expectedError bool
		mockResponse  *types.GetSearchAttributesResponse
		mockError     error
	}{
		{
			name:          "empty search attributes",
			out:           nil,
			expectedError: false,
		},
		{
			name: "error when get search attributes",
			in: &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					"CustomKeywordField": []byte("test-value"),
				},
			},
			out:           nil,
			mockError:     errors.New("test-error"),
			expectedError: true,
		},
		{
			name: "error deserialize search attributes",
			in: &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					"CustomKeywordField": []byte("test-value"),
				},
			},
			out:           nil,
			mockError:     nil,
			expectedError: true,
		},
		{
			name: "normal case",
			in: &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					"CustomKeywordField": []byte(`"test-value"`), // Assuming the value is a serialized string
				},
			},
			out: map[string]interface{}{
				"CustomKeywordField": "test-value", // Expected deserialized value
			},
			mockResponse: &types.GetSearchAttributesResponse{
				Keys: map[string]types.IndexedValueType{
					"CustomKeywordField": types.IndexedValueTypeKeyword,
				},
			},
			mockError:     nil,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			serverFrontendClient := frontend.NewMockClient(mockCtrl)
			serverAdminClient := admin.NewMockClient(mockCtrl)
			app := NewCliApp(&clientFactoryMock{
				serverFrontendClient: serverFrontendClient,
				serverAdminClient:    serverAdminClient,
			})
			c := getMockContext(t, nil, app)
			serverFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(tt.mockResponse, tt.mockError).AnyTimes()
			out, err := convertSearchAttributesToMapOfInterface(tt.in, serverFrontendClient, c)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.out, out)
			}
		})
	}
}

func Test_GetAllWorkflowIDsByQuery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})
	// missing required flag
	set := flag.NewFlagSet("test", 0)
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	c := cli.NewContext(app, set, nil)
	_, err := getAllWorkflowIDsByQuery(c, "WorkflowType='test-workflow-type'")
	assert.Error(t, err)

	c = getMockContext(t, nil, app)
	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), &types.ListWorkflowExecutionsRequest{
		Query:    "WorkflowType='test-workflow-type'",
		Domain:   "test-domain",
		PageSize: 1000,
	}).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)

	resp, err := getAllWorkflowIDsByQuery(c, "WorkflowType='test-workflow-type'")
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), &types.ListWorkflowExecutionsRequest{
		Query:    "WorkflowType='test-workflow-type'",
		Domain:   "test-domain",
		PageSize: 1000,
	}).Return(&types.ListWorkflowExecutionsResponse{}, errors.New("test-error")).Times(1)

	_, err = getAllWorkflowIDsByQuery(c, "WorkflowType='test-workflow-type'")
	assert.Error(t, err)
}

func Test_GetWorkflowStatus(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expected      types.WorkflowExecutionCloseStatus
		expectedError bool
	}{
		{
			name:          "Valid status - completed",
			input:         "completed",
			expected:      types.WorkflowExecutionCloseStatusCompleted,
			expectedError: false,
		},
		{
			name:          "Valid alias - fail",
			input:         "fail",
			expected:      types.WorkflowExecutionCloseStatusFailed,
			expectedError: false,
		},
		{
			name:          "Valid status - timed_out",
			input:         "timed_out",
			expected:      types.WorkflowExecutionCloseStatusTimedOut,
			expectedError: false,
		},
		{
			name:          "Invalid status",
			input:         "invalid",
			expected:      -1,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := getWorkflowStatus(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, -1, int(status))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, status)
			}
		})
	}
}

func Test_ConvertDescribeWorkflowExecutionResponse(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	mockResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
		PendingActivities: []*types.PendingActivityInfo{
			{
				ActivityID: "test-activity-id",
				ActivityType: &types.ActivityType{
					Name: "test-activity-type",
				},
				HeartbeatDetails:   []byte("test-heartbeat-details"),
				LastFailureDetails: []byte("test-failure-details"),
			},
		},
		PendingDecision: &types.PendingDecisionInfo{
			State: nil,
		},
	}

	resp, err := convertDescribeWorkflowExecutionResponse(mockResp, serverFrontendClient, nil)
	assert.NoError(t, err)
	assert.Equal(t, "test-workflow-id", resp.WorkflowExecutionInfo.Execution.WorkflowID)
}

func Test_PrintRunStatus(t *testing.T) {
	// this method only prints results, no need to test the output
	tests := []struct {
		name  string
		event *types.HistoryEvent
	}{
		{
			name: "COMPLETED",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
				WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{
					Result: []byte("workflow completed successfully"),
				},
			},
		},
		{
			name: "FAILED",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionFailed.Ptr(),
				WorkflowExecutionFailedEventAttributes: &types.WorkflowExecutionFailedEventAttributes{
					Reason:  nil,
					Details: []byte("failure details"),
				},
			},
		},
		{
			name: "TIMEOUT",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionTimedOut.Ptr(),
				WorkflowExecutionTimedOutEventAttributes: &types.WorkflowExecutionTimedOutEventAttributes{
					TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
				},
			},
		},
		{
			name: "CANCELED",
			event: &types.HistoryEvent{
				EventType: types.EventTypeWorkflowExecutionCanceled.Ptr(),
				WorkflowExecutionCanceledEventAttributes: &types.WorkflowExecutionCanceledEventAttributes{
					Details: []byte("canceled details"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				printRunStatus(tt.event)
			})
		})
	}
}

func Test_ListWorkflowExecutions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})
	c := getMockContext(t, nil, app)
	listFn := listWorkflowExecutions(serverFrontendClient, 100, "test-domain", "WorkflowType='test-workflow-type'", c)
	assert.NotNil(t, listFn)
	expectedResp := &types.ListWorkflowExecutionsResponse{
		Executions: []*types.WorkflowExecutionInfo{
			{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
		},
		NextPageToken: []byte("test-next-page-token"),
	}
	serverFrontendClient.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(expectedResp, nil).Times(1)
	executions, nextPageToken, err := listFn(nil)
	assert.NoError(t, err)
	assert.NotNil(t, executions)
	assert.Equal(t, expectedResp.Executions, executions)
	assert.NotNil(t, nextPageToken)

	serverFrontendClient.EXPECT().ListWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	_, _, err = listFn(nil)
	assert.Error(t, err)
}

func Test_PrintListResults(t *testing.T) {
	executions := []*types.WorkflowExecutionInfo{
		{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id-1",
				RunID:      "test-run-id-1",
			},
			Type: &types.WorkflowType{
				Name: "test-workflow-type-1",
			},
		},
		{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id-2",
				RunID:      "test-run-id-2",
			},
			Type: &types.WorkflowType{
				Name: "test-workflow-type-2",
			},
		},
	}

	assert.NotPanics(t, func() {
		printListResults(executions, true, false)
		printListResults(executions, false, false)
		printListResults(executions, true, true)
		printListResults(nil, true, false)
	})
}

func Test_ResetWorkflow(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})

	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	// missing domain flag
	err := ResetWorkflow(c)
	assert.Error(t, err)

	set.String(FlagDomain, "test-domain", "domain")
	// missing workflowID flag
	err = ResetWorkflow(c)
	assert.Error(t, err)

	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.Parse([]string{"test-workflow-id", "test-run-id"})
	// missing reason flag
	err = ResetWorkflow(c)
	assert.Error(t, err)

	set.String("reason", "test", "reason")
	set.String("decision_offset", "-1", "decision_offset")
	// invalid event ID
	err = ResetWorkflow(c)
	assert.Error(t, err)

	set.String("reset_type", "LastDecisionCompleted", "reset_type")
	set.String("run_id", "test-run-id", "run_id")
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				},
			},
		},
	}, nil).Times(2)
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}, nil).AnyTimes()
	err = ResetWorkflow(c)
	assert.NoError(t, err)

	// reset failed
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	err = ResetWorkflow(c)
	assert.Error(t, err)

	// getResetEventIDByType failed
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	err = ResetWorkflow(c)
	assert.Error(t, err)
}

func Test_ResetWorkflow_Invalid_Decision_Offset(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.Parse([]string{"test-workflow-id", "test-run-id"})
	set.String("reason", "test", "reason")
	set.String("decision_offset", "100", "decision_offset")
	c := cli.NewContext(app, set, nil)
	err := ResetWorkflow(c)
	assert.Error(t, err)
}

func Test_ResetWorkflow_Missing_RunID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.String("reason", "test", "reason")
	set.String("decision_offset", "-1", "decision_offset")
	set.String("reset_type", "BadBinary", "reset_type")
	set.String("reset_bad_binary_checksum", "test-bad-binary-checksum", "reset_bad_binary_checksum")
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}, errors.New("test-error")).AnyTimes()
	c := cli.NewContext(app, set, nil)
	err := ResetWorkflow(c)
	assert.Error(t, err)
}

func (s *cliAppSuite) TestCompleteActivity() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf activity complete -w wid -r rid -aid 3 -result result --identity tester`,
			err:     "",
			mock: func() {
				s.serverFrontendClient.EXPECT().RespondActivityTaskCompletedByID(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestDescribeWorkflow() {
	testCases := []testcase{
		{
			"happy",
			"cadence --do test-domain wf describe -w wid",
			"",
			func() {
				resp := &types.DescribeWorkflowExecutionResponse{
					WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
						Execution: &types.WorkflowExecution{
							WorkflowID: "wid",
						},
						Type: &types.WorkflowType{
							Name: "workflow-type",
						},
						StartTime: common.Int64Ptr(time.Now().UnixNano()),
						CloseTime: common.Int64Ptr(time.Now().UnixNano()),
					},
					PendingActivities: []*types.PendingActivityInfo{
						{},
					},
					PendingDecision: &types.PendingDecisionInfo{},
				}
				s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestFailActivity() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf activity fail -w wid -r rid -aid 3 --reason somereason --detail somedetail --identity tester`,
			err:     "",
			mock: func() {
				s.serverFrontendClient.EXPECT().RespondActivityTaskFailedByID(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestListAllWorkflow() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf listall`,
			mock: func() {
				countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
				s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
				s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(listClosedWorkflowExecutionsResponse, nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestQueryWorkflow() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test`,
			err:     "",
			mock: func() {
				resp := &types.QueryWorkflowResponse{
					QueryResult: []byte("query-result"),
				}
				s.serverFrontendClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(resp, nil)
			},
		},
		{
			name:    "query with reject not_open",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_reject_condition not_open`,
			err:     "",
			mock: func() {
				resp := &types.QueryWorkflowResponse{
					QueryResult: []byte("query-result"),
				}
				s.serverFrontendClient.EXPECT().
					QueryWorkflow(gomock.Any(), &types.QueryWorkflowRequest{
						Domain: "test-domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wid",
						},
						Query: &types.WorkflowQuery{
							QueryType: "query-type-test",
						},
						QueryRejectCondition: types.QueryRejectConditionNotOpen.Ptr(),
					}).
					Return(resp, nil)
			},
		},
		{
			name:    "query with reject not_completed_cleanly",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_reject_condition not_completed_cleanly`,
			err:     "",
			mock: func() {
				resp := &types.QueryWorkflowResponse{
					QueryResult: []byte("query-result"),
				}
				s.serverFrontendClient.EXPECT().
					QueryWorkflow(gomock.Any(), &types.QueryWorkflowRequest{
						Domain: "test-domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wid",
						},
						Query: &types.WorkflowQuery{
							QueryType: "query-type-test",
						},
						QueryRejectCondition: types.QueryRejectConditionNotCompletedCleanly.Ptr(),
					}).
					Return(resp, nil)
			},
		},
		{
			name:    "query with unknown reject",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_reject_condition unknown`,
			err:     "invalid reject condition",
		},
		{
			name:    "query with eventual consistency",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_consistency_level eventual`,
			err:     "",
			mock: func() {
				resp := &types.QueryWorkflowResponse{
					QueryResult: []byte("query-result"),
				}
				s.serverFrontendClient.EXPECT().
					QueryWorkflow(gomock.Any(), &types.QueryWorkflowRequest{
						Domain: "test-domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wid",
						},
						Query: &types.WorkflowQuery{
							QueryType: "query-type-test",
						},
						QueryConsistencyLevel: types.QueryConsistencyLevelEventual.Ptr(),
					}).
					Return(resp, nil)
			},
		},
		{
			name:    "query with strong consistency",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_consistency_level strong`,
			err:     "",
			mock: func() {
				resp := &types.QueryWorkflowResponse{
					QueryResult: []byte("query-result"),
				}
				s.serverFrontendClient.EXPECT().
					QueryWorkflow(gomock.Any(), &types.QueryWorkflowRequest{
						Domain: "test-domain",
						Execution: &types.WorkflowExecution{
							WorkflowID: "wid",
						},
						Query: &types.WorkflowQuery{
							QueryType: "query-type-test",
						},
						QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
					}).
					Return(resp, nil)
			},
		},
		{
			name:    "query with invalid consistency",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test --query_consistency_level invalid`,
			err:     "invalid query consistency level",
		},
		{
			name:    "failed",
			command: `cadence --do test-domain wf query -w wid -qt query-type-test`,
			err:     "some error",
			mock: func() {
				s.serverFrontendClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
		},
		{
			name:    "missing flags",
			command: "cadence wf query",
			err:     "Required flag not found",
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestResetWorkflow() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf reset -w wid -r rid -reason test-reason --event_id 1`,
			err:     "",
			mock: func() {
				resp := &types.ResetWorkflowExecutionResponse{RunID: uuid.New()}
				s.serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestScanAllWorkflow() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf scanall`,
			mock: func() {
				s.serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListWorkflowExecutionsResponse{}, nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestSignalWithStartWorkflowExecution() {
	testCases := []testcase{
		{
			name:    "happy",
			command: `cadence --do test-domain wf signalwithstart --et 100 --workflow_type sometype --tasklist tasklist -w wid -n signal-name --signal_input []`,
			err:     "",
			mock: func() {
				s.serverFrontendClient.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.StartWorkflowExecutionResponse{}, nil)
			},
		},
		{
			name:    "failed",
			command: `cadence --do test-domain wf signalwithstart --et 100 --workflow_type sometype --tasklist tasklist -w wid -n signal-name --signal_input []`,
			err:     "some error",
			mock: func() {
				s.serverFrontendClient.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("some error"))
			},
		},
		{
			name:    "missing flags",
			command: "cadence wf signalwithstart",
			err:     "Required flag not found",
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func Test_DoReset(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})

	set := flag.NewFlagSet("test", 0)

	set.String(FlagDomain, "test-domain", "domain")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.Parse([]string{"test-workflow-id", "test-run-id"})
	set.String("reason", "test", "reason")
	set.String("decision_offset", "-1", "decision_offset")
	set.String("reset_type", "LastDecisionCompleted", "reset_type")
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				},
				{
					ID:        2,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				},
			},
		},
	}, nil).AnyTimes()
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}, errors.New("test-error")).Times(1)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
	}, nil).AnyTimes()

	c := cli.NewContext(app, set, nil)
	params := &batchResetParamsType{
		reason:    "test",
		resetType: "LastDecisionCompleted",
	}
	// describe workflow execution failed
	err := doReset(c, "test-domain", "test-workflow-id", "test-run-id", *params)
	assert.Error(t, err)

	// get reset event id failure
	err = doReset(c, "test-domain", "test-workflow-id", "test-run-id", *params)
	assert.Error(t, err)

	// reset failure
	err = doReset(c, "test-domain", "test-workflow-id", "", *params)
	assert.Error(t, err)

	// normal case
	err = doReset(c, "test-domain", "test-workflow-id", "", *params)
	assert.NoError(t, err)

	// dry run
	params.dryRun = true
	err = doReset(c, "test-domain", "test-workflow-id", "", *params)
	assert.NoError(t, err)

	// skip current open
	params.skipCurrentOpen = true
	err = doReset(c, "test-domain", "test-workflow-id", "", *params)
	assert.NoError(t, err)

	// current run id not match with input rid
	params.skipBaseNotCurrent = true
	err = doReset(c, "test-domain", "test-workflow-id", "test-not-matched-rid", *params)
	assert.NoError(t, err)

	// dry run
	params.dryRun = true
	err = doReset(c, "test-domain", "test-workflow-id", "", *params)
	assert.NoError(t, err)
}

func Test_DoReset_SkipCurrentCompleted(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverAdminClient := admin.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
		serverAdminClient:    serverAdminClient,
	})

	set := flag.NewFlagSet("test", 0)

	set.String(FlagDomain, "test-domain", "domain")
	set.String("workflow_id", "test-workflow-id", "workflow_id")
	set.Parse([]string{"test-workflow-id", "test-run-id"})
	set.String("reason", "test", "reason")
	set.String("decision_offset", "-1", "decision_offset")
	set.String("reset_type", "LastDecisionCompleted", "reset_type")
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				},
			},
		},
	}, nil).AnyTimes()
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			CloseStatus: types.WorkflowExecutionCloseStatusCompleted.Ptr(),
			CloseTime:   common.Int64Ptr(time.Now().UnixNano()),
		},
	}, nil).AnyTimes()

	c := cli.NewContext(app, set, nil)
	params := &batchResetParamsType{
		reason:               "test",
		resetType:            "LastDecisionCompleted",
		nonDeterministicOnly: true,
	}
	// check non determinism failed when get history
	err := doReset(c, "test-domain", "test-workflow-id", "test-run-id", *params)
	assert.Error(t, err)

	err = doReset(c, "test-domain", "test-workflow-id", "test-run-id", *params)
	assert.NoError(t, err)

	// describe workflow execution failed
	params.skipCurrentCompleted = true
	err = doReset(c, "test-domain", "test-workflow-id", "test-run-id", *params)
	assert.NoError(t, err)
}

func createTempFileWithContent(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	_, err = tmpFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}

	tmpFileName := tmpFile.Name()
	tmpFile.Close()

	t.Cleanup(func() {
		_ = os.Remove(tmpFileName)
	})

	return tmpFileName
}

func TestLoadWorkflowIDsFromFile_Success(t *testing.T) {
	content := "wid1,wid2,wid3\n\nwid4,wid5\nwid6\n"
	fileName := createTempFileWithContent(t, content)

	workflowIDs, err := loadWorkflowIDsFromFile(fileName, ",")
	assert.NoError(t, err)

	expected := map[string]bool{
		"wid1": true,
		"wid4": true,
		"wid6": true,
	}
	assert.Equal(t, expected, workflowIDs)
}

func TestLoadWorkflowIDsFromFile_Failure(t *testing.T) {
	// open failed
	_, err := loadWorkflowIDsFromFile("non exist file", ",")
	assert.Error(t, err)
}

func Test_ResetInBatch_WithQuery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	// missing domain flag
	err := ResetInBatch(c)
	assert.Error(t, err)

	set.String(FlagDomain, "test-domain", "domain")
	set.String("reset_type", "BadBinary", "reset_type")
	set.String("reset_bad_binary_checksum", "test-bad-binary-checksum", "reset_bad_binary_checksum")
	set.String(FlagParallismDeprecated, "1", "input parallism")
	set.String(FlagParallelism, "2", "parallelism")
	set.String(FlagExcludeWorkflowIDByQuery, "test-workflow-id", "exclude query")

	// missing reason
	err = ResetInBatch(c)
	assert.Error(t, err)

	set.String(FlagReason, "test", "reason")
	// missing query
	err = ResetInBatch(c)
	assert.Error(t, err)

	set.String(FlagListQuery, "WorkflowType='test-workflow-type'", "list query")
	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListWorkflowExecutionsResponse{
		Executions: []*types.WorkflowExecutionInfo{
			{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
		},
	}, nil).Times(2)
	err = ResetInBatch(c)
	assert.NoError(t, err)
}

func Test_ResetInBatch_WithFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("reset_type", "BadBinary", "reset_type")
	set.String("reset_bad_binary_checksum", "test-bad-binary-checksum", "reset_bad_binary_checksum")
	content := "wid1,wid2,wid3\n\nwid4,wid5\nwid6\n"
	fileName := createTempFileWithContent(t, content)

	set.String(FlagInputFile, fileName, "input file")
	set.String(FlagParallismDeprecated, "1", "input parallism")
	set.String(FlagParallelism, "2", "parallelism")
	set.String(FlagExcludeFile, fileName, "exclude query")
	set.String(FlagReason, "test", "reason")

	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListWorkflowExecutionsResponse{
		Executions: []*types.WorkflowExecutionInfo{
			{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
		},
	}, nil).AnyTimes()
	c := cli.NewContext(app, set, nil)
	err := ResetInBatch(c)
	assert.NoError(t, err)

	// error when both exclude query and file are provided
	set.String(FlagExcludeWorkflowIDByQuery, "test-workflow-id", "exclude query")
	err = ResetInBatch(c)
	assert.Error(t, err)
}

func Test_ResetInBatch_InvalidDescisionOffset(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String(FlagDecisionOffset, "100", "decision_offset")
	set.String(FlagResetType, "BadBinary", "reset_type")
	c := cli.NewContext(app, set, nil)
	err := ResetInBatch(c)
	assert.Error(t, err)
}

func Test_ResetInBatch_InvalidResetType(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String(FlagDecisionOffset, "-1", "decision_offset")
	set.String(FlagResetType, "test", "reset_type")
	c := cli.NewContext(app, set, nil)
	err := ResetInBatch(c)
	assert.Error(t, err)
}

func Test_ResetInBatch_InvalidInputFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("reset_type", "BadBinary", "reset_type")
	set.String("reset_bad_binary_checksum", "test-bad-binary-checksum", "reset_bad_binary_checksum")
	set.String(FlagInputFile, "non exist file", "input file")
	set.String(FlagParallismDeprecated, "1", "input parallism")
	set.String(FlagParallelism, "2", "parallelism")
	set.String(FlagReason, "test", "reason")
	c := cli.NewContext(app, set, nil)
	err := ResetInBatch(c)
	assert.Error(t, err)
}

func Test_ResetInBatch_InvalidexcludeFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})

	set := flag.NewFlagSet("test", 0)
	set.String(FlagDomain, "test-domain", "domain")
	set.String("reset_type", "BadBinary", "reset_type")
	set.String("reset_bad_binary_checksum", "test-bad-binary-checksum", "reset_bad_binary_checksum")
	set.String(FlagExcludeFile, "non exist file", "exclude query")
	set.String(FlagParallismDeprecated, "1", "input parallism")
	set.String(FlagParallelism, "2", "parallelism")
	set.String(FlagReason, "test", "reason")
	set.String(FlagListQuery, "WorkflowType='test-workflow-type'", "list query")
	c := cli.NewContext(app, set, nil)
	err := ResetInBatch(c)
	assert.Error(t, err)
}

func Test_GetFirstDecisionTaskByType(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
				},
			},
		},
	}, nil).Times(1)
	// get desision type successfully
	decisionFinishID, err := getFirstDecisionTaskByType(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient, 6)
	assert.Equal(t, int64(1), decisionFinishID)
	assert.NoError(t, err)

	// failed to get workflow history
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(nil, errors.New("test-error")).Times(1)
	decisionFinishID, err = getFirstDecisionTaskByType(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient, 6)
	assert.Equal(t, int64(0), decisionFinishID)
	assert.Error(t, err)

	// not found decision task
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	decisionFinishID, err = getFirstDecisionTaskByType(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient, 6)
	assert.Equal(t, int64(0), decisionFinishID)
	assert.Error(t, err)

	// next page token
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		NextPageToken: []byte("test-next-page-token"),
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
		NextPageToken:   []byte("test-next-page-token"),
	}).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	decisionFinishID, err = getFirstDecisionTaskByType(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient, 6)
	assert.Equal(t, int64(0), decisionFinishID)
	assert.Error(t, err)
}

func Test_GetCurrentRunID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
		},
	}).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				RunID: "test-run-id",
			},
		},
	}, nil).Times(1)

	runID, err := getCurrentRunID(context.Background(), "test-domain", "test-workflow-id", serverFrontendClient)
	assert.NoError(t, err)
	assert.Equal(t, "test-run-id", runID)
}

func Test_GetBadDecisionCompletedID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
	}).Return(&types.DescribeWorkflowExecutionResponse{}, errors.New("test-error")).Times(1)

	decision, err := getBadDecisionCompletedID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", "", serverFrontendClient)
	assert.Equal(t, int64(0), decision)
	assert.Error(t, err)

	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
	}).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			AutoResetPoints: &types.ResetPoints{
				Points: []*types.ResetPointInfo{
					{
						BinaryChecksum:           "test-bad-binary-checksum",
						FirstDecisionCompletedID: 5,
						Resettable:               true,
					},
				},
			},
		},
	}, nil).Times(2)
	decision, err = getBadDecisionCompletedID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", "", serverFrontendClient)
	assert.Equal(t, int64(0), decision)
	assert.Error(t, err)

	decision, err = getBadDecisionCompletedID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", "test-bad-binary-checksum", serverFrontendClient)
	assert.Equal(t, int64(5), decision)
	assert.NoError(t, err)
}

func Test_GetLastContinueAsNewID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1,
	}).Return(nil, errors.New("test-error")).Times(1)
	// get workflow history failed
	runID, decisionID, err := getLastContinueAsNewID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "",
					},
				},
			},
		},
	}, nil).Times(1)
	// cannot get reset base runID
	runID, decisionID, err = getLastContinueAsNewID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					ID:        10,
				},
			},
		},
	}, nil).Times(1)
	runID, decisionID, err = getLastContinueAsNewID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(10), decisionID)
	assert.NoError(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(nil, errors.New("test-error")).Times(1)
	// fail to get workflow history after getting the reset base runID
	runID, decisionID, err = getLastContinueAsNewID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{},
		},
		NextPageToken: []byte("test-next-page-token"),
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{},
		},
	}, nil).Times(1)
	// fail to get workflow history after getting the reset base runID
	runID, decisionID, err = getLastContinueAsNewID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)
}

func Test_GetEarliestDecisionID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(nil, errors.New("test-error")).Times(1)

	// get workflow execution history failed
	decisionID, err := getEarliestDecisionID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", int64(100), serverFrontendClient)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{},
		},
		NextPageToken: []byte("test-next-page-token"),
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
		NextPageToken:   []byte("test-next-page-token"),
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{},
		},
	}, nil).Times(1)
	// no DecisionFinishID
	decisionID, err = getEarliestDecisionID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", int64(100), serverFrontendClient)
	assert.Equal(t, int64(0), decisionID)
	assert.Error(t, err)

	testEarliestTime := int64(110)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		MaximumPageSize: 1000,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					ID:        10,
					Timestamp: &testEarliestTime,
				},
			},
		},
	}, nil).Times(1)
	// no DecisionFinishID
	decisionID, err = getEarliestDecisionID(context.Background(), "test-domain", "test-workflow-id", "test-run-id", int64(100), serverFrontendClient)
	assert.Equal(t, int64(10), decisionID)
	assert.NoError(t, err)
}

func Test_FailActivity_CompleteActivity_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	// call fail activity and complete activity without required flags, they should return the same error
	err := FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))
	set.String(FlagDomain, "test-domain", "domain")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
	set.String(FlagWorkflowID, "test-workflow-id", "workflow_id")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagRunID))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagRunID))
	set.String(FlagRunID, "test-run-id", "run_id")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagActivityID))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagActivityID))
	set.String(FlagActivityID, "test-activity-id", "activity_id")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagReason))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagResult))
	set.String(FlagReason, "test", "reason")
	set.String(FlagResult, "test", "result")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDetail))
	set.String(FlagDetail, "test", "detail")

	err = FailActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagIdentity))
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagIdentity))
	set.String(FlagIdentity, "test", "indentity")

	serverFrontendClient.EXPECT().RespondActivityTaskFailedByID(gomock.Any(), &types.RespondActivityTaskFailedByIDRequest{
		Domain:     "test-domain",
		WorkflowID: "test-workflow-id",
		RunID:      "test-run-id",
		ActivityID: "test-activity-id",
		Reason:     common.StringPtr("test"),
		Details:    []byte("test"),
		Identity:   "test",
	}).Return(errors.New("test-error")).Times(1)
	err = FailActivity(c)
	assert.ErrorContains(t, err, "Failing activity failed")

	serverFrontendClient.EXPECT().RespondActivityTaskCompletedByID(gomock.Any(), &types.RespondActivityTaskCompletedByIDRequest{
		Domain:     "test-domain",
		WorkflowID: "test-workflow-id",
		RunID:      "test-run-id",
		ActivityID: "test-activity-id",
		Result:     []byte("test"),
		Identity:   "test",
	}).Return(errors.New("test-error")).Times(1)
	err = CompleteActivity(c)
	assert.ErrorContains(t, err, "Completing activity failed")
}

func Test_GetResetEventIDByType_LastContinuedAsNew(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	c := getMockContext(t, nil, app)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        15,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).Times(2)
	// reset type last continued as new
	runID, decisionID, err := getResetEventIDByType(context.Background(), c, resetTypeLastContinuedAsNew, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(15), decisionID)
	assert.NoError(t, err)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ChildWorkflowExecutionFailedEventAttributes: &types.ChildWorkflowExecutionFailedEventAttributes{},
				},
			},
		},
	}, nil).Times(1)
	_, _, err = getResetEventIDByType(context.Background(), c, resetTypeLastContinuedAsNew, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.ErrorContains(t, err, "cannot get resetBaseRunID")
}

func Test_GetResetEventIDByType_FirstDecisionCompleted(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	c := getMockContext(t, nil, app)

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        15,
					EventType: types.EventTypeActivityTaskCompleted.Ptr().Ptr(),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).Times(1)

	// reset type last decision completed
	runID, decisionID, err := getResetEventIDByType(context.Background(), c, resetTypeFirstDecisionCompleted, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.ErrorContains(t, err, "no DecisionFinishID")
}

func Test_GetResetEventIDByType_BadBinary(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	set := flag.NewFlagSet("test", 0)
	set.String(FlagResetBadBinaryChecksum, "test-bad-binary-checksum", "reset_bad_binary_checksum")
	c := getMockContext(t, set, app)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), &types.DescribeWorkflowExecutionRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
	}).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}, nil).Times(1)
	runID, decisionID, err := getResetEventIDByType(context.Background(), c, resetTypeBadBinary, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.ErrorContains(t, err, "no DecisionFinishID")
}

func Test_GetResetEventIDByType_DecisionCompletedTime(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	set := flag.NewFlagSet("test", 0)
	c := getMockContext(t, set, app)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 15,
				},
			},
		},
	}, nil).Times(1)
	_, decisionID, err := getResetEventIDByType(context.Background(), c, resetTypeDecisionCompletedTime, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, int64(0), decisionID)
	assert.ErrorContains(t, err, "no DecisionFinishID")

	set.String(FlagEarliestTime, "20201025Test", "earliest_time")
	_, _, err = getResetEventIDByType(context.Background(), c, resetTypeDecisionCompletedTime, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.ErrorContains(t, err, "use UTC format")
}

func Test_GetResetEventIDByType_FirstDecisionScheduled_LastDecisionScheduled(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	c := getMockContext(t, nil, app)
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID: 15,
				},
			},
		},
	}, nil).Times(2)
	// reset type first decision scheduled
	runID, decisionID, err := getResetEventIDByType(context.Background(), c, resetTypeFirstDecisionScheduled, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.ErrorContains(t, err, "no DecisionFinishID")
	// reset type Last decision scheduled
	runID, decisionID, err = getResetEventIDByType(context.Background(), c, resetTypeLastDecisionScheduled, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(0), decisionID)
	assert.ErrorContains(t, err, "no DecisionFinishID")
	// unsupported reset type
	assert.Panics(t, func() {
		getResetEventIDByType(context.Background(), c, "test reset type", -1, "test-domain", "test-workflow-id", "test-run-id", serverFrontendClient)
	})

	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        15,
					EventType: types.EventTypeDecisionTaskScheduled.Ptr(),
				},
			},
		},
	}, nil).Times(2)
	_, decisionID, err = getResetEventIDByType(context.Background(), c, resetTypeFirstDecisionScheduled, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(16), decisionID)
	assert.NoError(t, err)

	_, decisionID, err = getResetEventIDByType(context.Background(), c, resetTypeLastDecisionScheduled, -1, "test-domain",
		"test-workflow-id", "test-run-id", serverFrontendClient)
	assert.Equal(t, "test-run-id", runID)
	assert.Equal(t, int64(16), decisionID)
	assert.NoError(t, err)
}

func Test_SignalWorkflow_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := SignalWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = SignalWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))

	set.String(FlagWorkflowID, "test-workflow-id", "workflow_id")
	err = SignalWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagName))
}

func Test_RestartWorkflow_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := RestartWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = RestartWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_DiagnoseWorkflow_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := DiagnoseWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = DiagnoseWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))

	set.String(FlagWorkflowID, "test-workflow-id", "workflow_id")
	err = DiagnoseWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagRunID))
}

func Test_TerminateWorkflow_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := TerminateWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = TerminateWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_ShowHistory_MissingWorkflowID(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := ShowHistory(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_ShowHistoryWithID_MissingWorkflowID(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := ShowHistoryWithWID(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))

	set.Parse([]string{"test-workflow-id", "test-run-id"})
	err = ShowHistoryWithWID(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))
}

func Test_ConstructStartWorkflowRequest_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	_, err := constructStartWorkflowRequest(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	_, err = constructStartWorkflowRequest(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagTaskList))

	set.String(FlagTaskList, "test-tasklist", "tasklist")
	_, err = constructStartWorkflowRequest(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowType))

	set.String(FlagWorkflowType, "test-workflow-type", "workflow_type")
	_, err = constructStartWorkflowRequest(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s format is invalid", FlagExecutionTimeout))

	set.String(FlagExecutionTimeout, "10", "execution_timeout")
	set.Int(FlagWorkflowIDReusePolicy, 1, "workflowidreusepolicy")
	_, err = constructStartWorkflowRequest(c)
	assert.NoError(t, err)

	// invalid workflowID reuse policy
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, -10))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "value is not in supported range")

	// process Json error
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagInput, "invalid json"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "input is not valid JSON")

	// error processing first run at
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagCronSchedule, "* * * * *"),
		clitest.StringArgument(FirstRunAtTime, "10:00"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "time format invalid")

	// error processing header
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagCronSchedule, "* * * * *"),
		clitest.StringArgument(FlagHeaderFile, "invalid file"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "error when process header")

	// error processing memo
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagCronSchedule, "* * * * *"),
		clitest.StringArgument(FlagMemoFile, "invalid file"), clitest.StringArgument(FlagSearchAttributesKey, "key"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "Error processing memo")

	// error processing search attributes
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagCronSchedule, "* * * * *"),
		clitest.StringArgument(FlagSearchAttributesKey, "key"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "error processing search attributes")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.IntArgument(FlagWorkflowIDReusePolicy, 1), clitest.StringArgument(FlagCronSchedule, "* * * * *"),
		clitest.StringArgument(FlagSearchAttributesKey, "key"), clitest.StringArgument(FlagSearchAttributesVal, "val"))
	_, err = constructStartWorkflowRequest(ctx)
	assert.NoError(t, err)
}

func Test_NewTest(t *testing.T) {

}

func Test_ProcessSearchAttr(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	set.String(FlagSearchAttributesKey, "key", "search attribute key")
	c := cli.NewContext(app, set, nil)
	_, err := processSearchAttr(c)
	assert.ErrorContains(t, err, "keys and values are not equal")

	set.String(FlagSearchAttributesVal, "value", "search attribute value")
	resp, err := processSearchAttr(c)
	assert.NoError(t, err)
	expectedVal, _ := json.Marshal("value")
	expectedResp := map[string][]byte{"key": expectedVal}
	assert.Equal(t, expectedResp, resp)
}

func Test_CancelWorkflow_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := CancelWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = CancelWorkflow(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_QueryWorkflowHelper_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := queryWorkflowHelper(c, "")
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	set.String(FlagDomain, "test-domain", "domain")
	err = queryWorkflowHelper(c, "")
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))

	content := "wid1,wid2,wid3\n\nwid4,wid5\nwid6\n"
	fileName := createTempFileWithContent(t, content)
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagWorkflowID, "test-workflow-id"),
		clitest.StringArgument(FlagInputFile, fileName))
	err = QueryWorkflowUsingQueryTypes(ctx)
	assert.ErrorContains(t, err, "Error processing json")
}

func Test_ProcessJsonInputHelper(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	content := "wid1,wid2,wid3\n\nwid4,wid5\nwid6\n"
	fileName := createTempFileWithContent(t, content)

	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagInputFile, fileName))
	_, err := processJSONInputHelper(ctx, jsonTypeInput)
	assert.ErrorContains(t, err, "input is not valid JSON")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagInputFile, "non exist file"))
	_, err = processJSONInputHelper(ctx, jsonTypeInput)
	assert.ErrorContains(t, err, "error reading input file")

	resp, err := processJSONInputHelper(ctx, -1)
	assert.Equal(t, "", resp)
	assert.NoError(t, err)
}

func Test_ConstructSignalWithStartWorkflowRequest_Errors(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"))

	_, err := constructSignalWithStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagName))

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagTaskList, "test-tasklist"),
		clitest.StringArgument(FlagWorkflowType, "test-workflow-type"), clitest.StringArgument(FlagExecutionTimeout, "10"),
		clitest.StringArgument(FlagName, "test-signal-name"), clitest.StringArgument(FlagSignalInputFile, "invalid json"))
	_, err = constructSignalWithStartWorkflowRequest(ctx)
	assert.ErrorContains(t, err, "error processing json input signal")
}

func Test_ListWorkflow_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app)
	err := ListWorkflow(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).Times(1)
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagWorkflowID, "test-workflow-id"), clitest.StringArgument(FlagExcludeWorkflowIDByQuery, "test-exclude"),
		clitest.StringArgument(FlagListQuery, "test-query"))
	err = ListWorkflow(ctx)
	assert.ErrorContains(t, err, "test-error")
}

func Test_ListAllWorkflow_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app)
	err := ListAllWorkflow(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	serverFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).Times(1)
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagWorkflowID, "test-workflow-id"), clitest.StringArgument(FlagExcludeWorkflowIDByQuery, "test-exclude"),
		clitest.StringArgument(FlagListQuery, "test-query"))
	err = ListAllWorkflow(ctx)
	assert.ErrorContains(t, err, "test-error")
}

func Test_CountWorkflow_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app)
	err := CountWorkflow(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"))
	serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).AnyTimes()
	err = CountWorkflow(ctx)
	assert.ErrorContains(t, err, "test-error")
}

func Test_DescribeWorkflow_Errors(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	ctx := clitest.NewCLIContext(t, app)
	err := DescribeWorkflow(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_ObserveHistory_MissingFlags(t *testing.T) {
	app := NewCliApp(&clientFactoryMock{})
	set := flag.NewFlagSet("test", 0)
	c := cli.NewContext(app, set, nil)
	err := ObserveHistory(c)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagWorkflowID))
}

func Test_ShowHistoryHelper(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
		Domain: "test-domain",
		Execution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
		WaitForNewEvent:        false,
		HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
		SkipArchival:           false,
	}).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        1,
					Version:   1,
					EventType: types.EventTypeDecisionTaskStarted.Ptr(),
				},
				{
					ID:        2,
					Version:   1,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					DecisionTaskCompletedEventAttributes: &types.DecisionTaskCompletedEventAttributes{
						ScheduledEventID: 1,
					},
				},
			},
		},
	}, nil).Times(5)
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{}).Times(1)
	// workflow not exists
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagPrintFullyDetail, "true"), clitest.StringArgument(FlagResetPointsOnly, "true"),
		clitest.StringArgument(FlagOutputFilename, "test-file"))
	defer func() {
		err := os.Remove("test-file")
		assert.NoError(t, err, "Expected no error during file cleanup")
	}()
	err := showHistoryHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, "workflow not exist")

	// error when describe workflow
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).Times(1)
	err = showHistoryHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, "cannot get information of pending activities")

	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			SearchAttributes: &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					"CustomKeywordField": []byte("test"),
				},
			},
		},
	}, nil).Times(1)
	serverFrontendClient.EXPECT().GetSearchAttributes(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).Times(1)
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagPrintFullyDetail, "false"), clitest.StringArgument(FlagResetPointsOnly, "true"),
		clitest.StringArgument(FlagPrintEventVersion, "true"))
	// error when converting search attribtues
	err = showHistoryHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, "Error in convert describe wf")

	// no error when converting search attribtues
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
		PendingActivities: []*types.PendingActivityInfo{
			{
				ActivityID: "test-activity-id",
				ActivityType: &types.ActivityType{
					Name: "test-activity-type",
				},
			},
		},
	}, nil).Times(1)
	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagPrintFullyDetail, "false"), clitest.StringArgument(FlagResetPointsOnly, "true"),
		clitest.StringArgument(FlagEventID, "1"))
	err = showHistoryHelper(ctx, "test-workflow-id", "test-run-id")
	assert.NoError(t, err)

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagPrintFullyDetail, "false"), clitest.StringArgument(FlagResetPointsOnly, "true"),
		clitest.StringArgument(FlagEventID, "test-event-id"))
	err = showHistoryHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, "EventId out of range")
}

func Test_DescribeWorkflowHelper_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app)
	err := describeWorkflowHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"))
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("test-error")).Times(1)
	err = describeWorkflowHelper(ctx, "test-workflow-id", "test-run-id")
	assert.ErrorContains(t, err, "Describe workflow execution failed")
}

func Test_ProcessResets(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
			Execution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
		},
	}, nil).AnyTimes()
	serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(&types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					ID:        15,
					EventType: types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
				{
					ID:        17,
					EventType: types.EventTypeDecisionTaskCompleted.Ptr(),
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						ContinuedExecutionRunID: "test-run-id",
					},
				},
			},
		},
	}, nil).AnyTimes()
	serverFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"))

	// Initialize channels and WaitGroup
	wes := make(chan types.WorkflowExecution, 1)
	done := make(chan bool)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	wid := "test-workflow-id"
	rid := "test-run-id"
	params := batchResetParamsType{
		resetType: resetTypeLastContinuedAsNew,
		reason:    "test-reason",
	}

	// Run processResets in a goroutine to prevent hanging
	go func() {
		defer close(wes)
		defer close(done)
		processResets(ctx, "test-domain", wes, done, wg, params)
	}()

	wes <- types.WorkflowExecution{
		WorkflowID: wid,
		RunID:      rid,
	}
	done <- true
	wg.Wait()
	assert.NoError(t, doReset(ctx, "test-domain", wid, rid, params))
}

func Test_ListWorkflows_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagEarliestTime, "InvalidTime"))
	_, err := listWorkflows(ctx)
	assert.ErrorContains(t, err, "cannot parse timeRange")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagLatestTime, "InvalidTime"))
	_, err = listWorkflows(ctx)
	assert.ErrorContains(t, err, "cannot parse timeRange")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagWorkflowStatus, "Closed"), clitest.StringArgument(FlagOpen, "true"))
	_, err = listWorkflows(ctx)
	assert.ErrorContains(t, err, "you can only filter on status for closed workflow")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagWorkflowStatus, "Test"))
	_, err = listWorkflows(ctx)
	assert.ErrorContains(t, err, "failed to parse workflow status")

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"),
		clitest.StringArgument(FlagWorkflowID, "test-workflow-id"), clitest.StringArgument(FlagWorkflowType, "test-workflow-type"))
	_, err = listWorkflows(ctx)
	assert.ErrorContains(t, err, "you can filter on workflow_id or workflow_type")
}

func Test_ListArchivedWorkflows_Errors(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	serverFrontendClient := frontend.NewMockClient(mockCtrl)
	app := NewCliApp(&clientFactoryMock{
		serverFrontendClient: serverFrontendClient,
	})
	ctx := clitest.NewCLIContext(t, app)
	_, err := listArchivedWorkflows(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagDomain))

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"))
	_, err = listArchivedWorkflows(ctx)
	assert.ErrorContains(t, err, fmt.Sprintf("%s is required", FlagListQuery))

	ctx = clitest.NewCLIContext(t, app, clitest.StringArgument(FlagDomain, "test-domain"), clitest.StringArgument(FlagListQuery, "test-query"),
		clitest.IntArgument(FlagPageSize, -1), clitest.IntArgument(FlagContextTimeout, 10))
	_, err = listArchivedWorkflows(ctx)
	assert.NoError(t, err)
}
