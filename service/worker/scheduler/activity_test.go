// Copyright (c) 2026 Uber Technologies, Inc.
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

package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
)

func TestGenerateWorkflowID(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		prefix     string
		scheduleID string
		time       time.Time
		want       string
	}{
		{
			name:       "uses prefix when provided",
			prefix:     "my-workflow",
			scheduleID: "sched-123",
			time:       ts,
			want:       "my-workflow-2026-01-15T10:00:00Z",
		},
		{
			name:       "falls back to scheduleID when prefix is empty",
			prefix:     "",
			scheduleID: "sched-456",
			time:       ts,
			want:       "sched-456-2026-01-15T10:00:00Z",
		},
		{
			name:       "deterministic for same inputs",
			prefix:     "wf",
			scheduleID: "sched-789",
			time:       ts,
			want:       "wf-2026-01-15T10:00:00Z",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := generateWorkflowID(tc.prefix, tc.scheduleID, tc.time)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGenerateRequestID(t *testing.T) {
	t.Run("returns valid UUID", func(t *testing.T) {
		id := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		_, err := uuid.Parse(id)
		assert.NoError(t, err)
	})
	t.Run("deterministic for same inputs", func(t *testing.T) {
		a := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		b := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		assert.Equal(t, a, b)
	})
	t.Run("different for different scheduleID", func(t *testing.T) {
		a := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		b := generateRequestID("sched-2", 1000000000, TriggerSourceSchedule)
		assert.NotEqual(t, a, b)
	})
	t.Run("different for different time", func(t *testing.T) {
		a := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		b := generateRequestID("sched-1", 2000000000, TriggerSourceSchedule)
		assert.NotEqual(t, a, b)
	})
	t.Run("different for different trigger source", func(t *testing.T) {
		a := generateRequestID("sched-1", 1000000000, TriggerSourceSchedule)
		b := generateRequestID("sched-1", 1000000000, TriggerSourceBackfill)
		assert.NotEqual(t, a, b)
	})
}

func TestStartWorkflowActivity(t *testing.T) {
	scheduledTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	int32Ptr := func(v int32) *int32 { return &v }
	baseReq := StartWorkflowRequest{
		Domain:     "test-domain",
		ScheduleID: "sched-1",
		Action: types.StartWorkflowAction{
			WorkflowType:                        &types.WorkflowType{Name: "my-workflow"},
			TaskList:                            &types.TaskList{Name: "my-tasklist"},
			Input:                               []byte(`{"key":"value"}`),
			WorkflowIDPrefix:                    "my-prefix",
			ExecutionStartToCloseTimeoutSeconds: int32Ptr(3600),
			TaskStartToCloseTimeoutSeconds:      int32Ptr(60),
		},
		ScheduledTime: scheduledTime,
		TriggerSource: TriggerSourceSchedule,
	}

	tests := []struct {
		name       string
		req        StartWorkflowRequest
		setupMock  func(m *frontend.MockClient)
		wantResult *StartWorkflowResult
		wantErr    bool
	}{
		{
			name: "successful start",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.StartWorkflowExecutionRequest, _ ...interface{}) (*types.StartWorkflowExecutionResponse, error) {
						assert.Equal(t, "test-domain", req.Domain)
						assert.Equal(t, "my-prefix-"+formatTime(scheduledTime), req.WorkflowID)
						assert.Equal(t, "my-workflow", req.WorkflowType.Name)
						assert.Equal(t, "my-tasklist", req.TaskList.Name)
						_, uuidErr := uuid.Parse(req.RequestID)
						assert.NoError(t, uuidErr, "RequestID must be a valid UUID")
						assert.Equal(t, generateRequestID("sched-1", scheduledTime.UnixNano(), TriggerSourceSchedule), req.RequestID, "RequestID must be deterministic")
						return &types.StartWorkflowExecutionResponse{RunID: "run-abc"}, nil
					})
			},
			wantResult: &StartWorkflowResult{
				WorkflowID: "my-prefix-" + formatTime(scheduledTime),
				RunID:      "run-abc",
				Started:    true,
			},
		},
		{
			name: "already started returns skipped with RunID",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{
						Message: "already started",
						RunID:   "existing-run-id",
					})
			},
			wantResult: &StartWorkflowResult{
				WorkflowID: "my-prefix-" + formatTime(scheduledTime),
				RunID:      "existing-run-id",
				Skipped:    true,
			},
		},
		{
			name: "transient error propagated",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "empty prefix falls back to scheduleID",
			req: func() StartWorkflowRequest {
				r := baseReq
				r.Action.WorkflowIDPrefix = ""
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.StartWorkflowExecutionRequest, _ ...interface{}) (*types.StartWorkflowExecutionResponse, error) {
						assert.Equal(t, "sched-1-"+formatTime(scheduledTime), req.WorkflowID)
						return &types.StartWorkflowExecutionResponse{RunID: "run-def"}, nil
					})
			},
			wantResult: &StartWorkflowResult{
				WorkflowID: "sched-1-" + formatTime(scheduledTime),
				RunID:      "run-def",
				Started:    true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockClient := frontend.NewMockClient(ctrl)
			tc.setupMock(mockClient)

			ctx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
				FrontendClient: mockClient,
			})

			result, err := startWorkflowActivity(ctx, tc.req)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantResult, result)
		})
	}
}

func TestStartWorkflowActivity_MissingContext(t *testing.T) {
	ctx := context.Background()
	_, err := startWorkflowActivity(ctx, StartWorkflowRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheduler context not found")
}

func TestDescribeWorkflowActivity(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(m *frontend.MockClient)
		wantResult *DescribeWorkflowResult
		wantErr    bool
	}{
		{
			name: "running workflow",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
							CloseStatus: nil,
						},
					}, nil)
			},
			wantResult: &DescribeWorkflowResult{IsRunning: true},
		},
		{
			name: "closed workflow",
			setupMock: func(m *frontend.MockClient) {
				status := types.WorkflowExecutionCloseStatusCompleted
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
							CloseStatus: &status,
						},
					}, nil)
			},
			wantResult: &DescribeWorkflowResult{IsRunning: false},
		},
		{
			name: "entity not found",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{Message: "not found"})
			},
			wantResult: &DescribeWorkflowResult{IsRunning: false},
		},
		{
			name: "transient error propagated",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("connection refused"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(ctrl)
			tc.setupMock(mockClient)

			ctx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
				FrontendClient: mockClient,
			})

			result, err := describeWorkflowActivity(ctx, DescribeWorkflowRequest{
				Domain:     "test-domain",
				WorkflowID: "wf-1",
				RunID:      "run-1",
			})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantResult, result)
		})
	}
}

func TestCancelWorkflowActivity(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m *frontend.MockClient)
		wantErr   bool
	}{
		{
			name: "successful cancel with cause",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.RequestCancelWorkflowExecutionRequest, _ ...interface{}) error {
						assert.Equal(t, "schedule overlap policy: CANCEL_PREVIOUS", req.Cause)
						assert.Equal(t, "wf-1", req.WorkflowExecution.WorkflowID)
						return nil
					})
			},
		},
		{
			name: "entity not found is not an error",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "not found"})
			},
		},
		{
			name: "transient error propagated",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("connection refused"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(ctrl)
			tc.setupMock(mockClient)

			ctx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
				FrontendClient: mockClient,
			})

			err := cancelWorkflowActivity(ctx, CancelWorkflowRequest{
				Domain:     "test-domain",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				Cause:      "schedule overlap policy: CANCEL_PREVIOUS",
			})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTerminateWorkflowActivity(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(m *frontend.MockClient)
		wantErr   bool
	}{
		{
			name: "successful terminate",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "entity not found is not an error",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.EntityNotExistsError{Message: "not found"})
			},
		},
		{
			name: "transient error propagated",
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("connection refused"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(ctrl)
			tc.setupMock(mockClient)

			ctx := context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
				FrontendClient: mockClient,
			})

			err := terminateWorkflowActivity(ctx, TerminateWorkflowRequest{
				Domain:     "test-domain",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				Reason:     "overlap",
			})
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestIsEntityNotExistsError(t *testing.T) {
	assert.True(t, isEntityNotExistsError(&types.EntityNotExistsError{Message: "not found"}))
	assert.False(t, isEntityNotExistsError(errors.New("other")))
	assert.False(t, isEntityNotExistsError(nil))
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
