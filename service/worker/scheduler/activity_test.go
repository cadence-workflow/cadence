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
	"encoding/json"
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

func TestProcessScheduleFireActivity(t *testing.T) {
	scheduledTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	int32Ptr := func(v int32) *int32 { return &v }

	baseReq := ProcessFireRequest{
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
		OverlapPolicy: types.ScheduleOverlapPolicySkipNew,
	}

	expectedWfID := "my-prefix-" + formatTime(scheduledTime)

	tests := []struct {
		name       string
		req        ProcessFireRequest
		setupMock  func(m *frontend.MockClient)
		wantResult *ProcessFireResult
		wantErr    bool
		noContext  bool
	}{
		{
			name: "successful start with no previous workflow",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *types.StartWorkflowExecutionRequest, _ ...interface{}) (*types.StartWorkflowExecutionResponse, error) {
						assert.Equal(t, "test-domain", req.Domain)
						assert.Equal(t, expectedWfID, req.WorkflowID)
						assert.Equal(t, types.WorkflowIDReusePolicyAllowDuplicate, *req.WorkflowIDReusePolicy)
						_, uuidErr := uuid.Parse(req.RequestID)
						assert.NoError(t, uuidErr)
						return &types.StartWorkflowExecutionResponse{RunID: "run-abc"}, nil
					})
			},
			wantResult: &ProcessFireResult{
				TotalDelta:      1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "run-abc"},
			},
		},
		{
			name: "SKIP_NEW skips when previous is running",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicySkipNew
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: nil},
					}, nil)
			},
			wantResult: &ProcessFireResult{
				SkippedDelta:    1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"},
			},
		},
		{
			name: "SKIP_NEW starts when previous is closed",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicySkipNew
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				status := types.WorkflowExecutionCloseStatusCompleted
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: &status},
					}, nil)
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "new-run"}, nil)
			},
			wantResult: &ProcessFireResult{
				TotalDelta:      1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "new-run"},
			},
		},
		{
			name: "TERMINATE_PREVIOUS terminates then starts",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicyTerminatePrevious
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: nil},
					}, nil)
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "new-run"}, nil)
			},
			wantResult: &ProcessFireResult{
				TotalDelta:      1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "new-run"},
			},
		},
		{
			name: "CANCEL_PREVIOUS cancels then starts",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicyCancelPrevious
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: nil},
					}, nil)
				m.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "new-run"}, nil)
			},
			wantResult: &ProcessFireResult{
				TotalDelta:      1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "new-run"},
			},
		},
		{
			name: "TERMINATE_PREVIOUS failure returns error for retry",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicyTerminatePrevious
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: nil},
					}, nil)
				m.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "CANCEL_PREVIOUS failure returns error for retry",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicyCancelPrevious
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.DescribeWorkflowExecutionResponse{
						WorkflowExecutionInfo: &types.WorkflowExecutionInfo{CloseStatus: nil},
					}, nil)
				m.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "CONCURRENT skips overlap check and starts",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicyConcurrent
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "new-run"}, nil)
			},
			wantResult: &ProcessFireResult{
				TotalDelta:      1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "new-run"},
			},
		},
		{
			name: "AlreadyStartedError returns skipped with RunID",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{
						Message: "already started",
						RunID:   "existing-run",
					})
			},
			wantResult: &ProcessFireResult{
				SkippedDelta:    1,
				StartedWorkflow: &RunningWorkflowInfo{WorkflowID: expectedWfID, RunID: "existing-run"},
			},
		},
		{
			name: "start failure returns error for retry",
			req:  baseReq,
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name: "describe failure returns error for retry",
			req: func() ProcessFireRequest {
				r := baseReq
				r.OverlapPolicy = types.ScheduleOverlapPolicySkipNew
				r.LastStartedWorkflow = &RunningWorkflowInfo{WorkflowID: "old-wf", RunID: "old-run"}
				return r
			}(),
			setupMock: func(m *frontend.MockClient) {
				m.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("connection refused"))
			},
			wantErr: true,
		},
		{
			name:      "missing context returns error",
			req:       baseReq,
			noContext: true,
			setupMock: func(m *frontend.MockClient) {},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := frontend.NewMockClient(ctrl)
			tc.setupMock(mockClient)

			var ctx context.Context
			if tc.noContext {
				ctx = context.Background()
			} else {
				ctx = context.WithValue(context.Background(), schedulerContextKey, schedulerContext{
					FrontendClient: mockClient,
				})
			}

			result, err := processScheduleFireActivity(ctx, tc.req)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantResult, result)
		})
	}
}

func TestBuildSearchAttributes(t *testing.T) {
	scheduledTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("sets scheduler-managed attributes", func(t *testing.T) {
		req := ProcessFireRequest{
			ScheduleID:    "sched-1",
			ScheduledTime: scheduledTime,
			TriggerSource: TriggerSourceSchedule,
		}
		sa := buildSearchAttributes(req)
		require.NotNil(t, sa)

		var schedID string
		require.NoError(t, json.Unmarshal(sa.IndexedFields[SearchAttrScheduleID], &schedID))
		assert.Equal(t, "sched-1", schedID)

		var isBackfill bool
		require.NoError(t, json.Unmarshal(sa.IndexedFields[SearchAttrIsBackfill], &isBackfill))
		assert.False(t, isBackfill)

		assert.Contains(t, sa.IndexedFields, SearchAttrScheduleTime)
	})

	t.Run("backfill trigger sets isBackfill true", func(t *testing.T) {
		req := ProcessFireRequest{
			ScheduleID:    "sched-1",
			ScheduledTime: scheduledTime,
			TriggerSource: TriggerSourceBackfill,
		}
		sa := buildSearchAttributes(req)

		var isBackfill bool
		require.NoError(t, json.Unmarshal(sa.IndexedFields[SearchAttrIsBackfill], &isBackfill))
		assert.True(t, isBackfill)
	})

	t.Run("preserves user search attributes", func(t *testing.T) {
		userVal, _ := json.Marshal("my-value")
		req := ProcessFireRequest{
			ScheduleID:    "sched-1",
			ScheduledTime: scheduledTime,
			TriggerSource: TriggerSourceSchedule,
			Action: types.StartWorkflowAction{
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{
						"CustomAttr": userVal,
					},
				},
			},
		}
		sa := buildSearchAttributes(req)

		assert.Equal(t, userVal, sa.IndexedFields["CustomAttr"])
		assert.Contains(t, sa.IndexedFields, SearchAttrScheduleID)
	})

	t.Run("scheduler attributes override conflicting user attributes", func(t *testing.T) {
		userVal, _ := json.Marshal("user-override")
		req := ProcessFireRequest{
			ScheduleID:    "sched-1",
			ScheduledTime: scheduledTime,
			TriggerSource: TriggerSourceSchedule,
			Action: types.StartWorkflowAction{
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{
						SearchAttrScheduleID: userVal,
					},
				},
			},
		}
		sa := buildSearchAttributes(req)

		var schedID string
		require.NoError(t, json.Unmarshal(sa.IndexedFields[SearchAttrScheduleID], &schedID))
		assert.Equal(t, "sched-1", schedID, "scheduler-managed attribute should override user value")
	})
}

func TestIsEntityNotExistsError(t *testing.T) {
	assert.True(t, isEntityNotExistsError(&types.EntityNotExistsError{Message: "not found"}))
	assert.False(t, isEntityNotExistsError(errors.New("other")))
	assert.False(t, isEntityNotExistsError(nil))
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
