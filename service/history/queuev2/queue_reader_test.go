package queuev2

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

func TestQueueReader_GetTask(t *testing.T) {
	historyTasks := []persistence.Task{
		&persistence.DecisionTimeoutTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   "test-domain",
				WorkflowID: "test-workflow",
				RunID:      "test-run",
			},
			TaskData: persistence.TaskData{
				Version:             1,
				TaskID:              1,
				VisibilityTimestamp: time.Now().UTC(),
			},
		},
		&persistence.ActivityTimeoutTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   "test-domain",
				WorkflowID: "test-workflow",
				RunID:      "test-run",
			},
			TaskData: persistence.TaskData{
				Version:             1,
				TaskID:              2,
				VisibilityTimestamp: time.Now().UTC(),
			},
		},
	}
	tests := []struct {
		name            string
		request         *GetTaskRequest
		mockSetup       func(*shard.MockContext, *persistence.MockExecutionManager, *MockPredicate)
		expectedTasks   []persistence.Task
		expectedToken   []byte
		expectedNextKey persistence.HistoryTaskKey
		expectedError   error
	}{
		{
			name: "successful task retrieval",
			request: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
						ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
				},
				PageSize: 10,
			},
			mockSetup: func(mockShard *shard.MockContext, mockExec *persistence.MockExecutionManager, mockPredicate *MockPredicate) {
				mockShard.EXPECT().GetExecutionManager().Return(mockExec)
				mockExec.EXPECT().GetHistoryTasks(gomock.Any(), &persistence.GetHistoryTasksRequest{
					TaskCategory:        persistence.HistoryTaskCategoryTimer,
					InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
					ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					PageSize:            10,
					NextPageToken:       nil,
				}).Return(&persistence.GetHistoryTasksResponse{
					Tasks:         historyTasks,
					NextPageToken: []byte("next-page"),
				}, nil)
				mockPredicate.EXPECT().Check(gomock.Any()).Return(true).AnyTimes()
			},
			expectedTasks:   historyTasks,
			expectedToken:   []byte("next-page"),
			expectedNextKey: historyTasks[1].GetTaskKey().Next(),
		},
		{
			name: "successful task retrieval with empty next page token",
			request: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
						ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
				},
				PageSize: 10,
			},
			mockSetup: func(mockShard *shard.MockContext, mockExec *persistence.MockExecutionManager, mockPredicate *MockPredicate) {
				mockShard.EXPECT().GetExecutionManager().Return(mockExec)
				mockExec.EXPECT().GetHistoryTasks(gomock.Any(), &persistence.GetHistoryTasksRequest{
					TaskCategory:        persistence.HistoryTaskCategoryTimer,
					InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
					ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					PageSize:            10,
					NextPageToken:       nil,
				}).Return(&persistence.GetHistoryTasksResponse{
					Tasks:         historyTasks,
					NextPageToken: nil,
				}, nil)
				mockPredicate.EXPECT().Check(gomock.Any()).Return(true).AnyTimes()
			},
			expectedTasks:   historyTasks,
			expectedToken:   nil,
			expectedNextKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
		},
		{
			name: "task filtering with predicate",
			request: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
						ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
				},
				PageSize: 10,
			},
			mockSetup: func(mockShard *shard.MockContext, mockExec *persistence.MockExecutionManager, mockPredicate *MockPredicate) {
				mockShard.EXPECT().GetExecutionManager().Return(mockExec)
				mockExec.EXPECT().GetHistoryTasks(gomock.Any(), &persistence.GetHistoryTasksRequest{
					TaskCategory:        persistence.HistoryTaskCategoryTimer,
					InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
					ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					PageSize:            10,
					NextPageToken:       nil,
				}).Return(&persistence.GetHistoryTasksResponse{
					Tasks:         historyTasks,
					NextPageToken: []byte("next-page"),
				}, nil)
				mockPredicate.EXPECT().Check(historyTasks[0]).Return(true)
				mockPredicate.EXPECT().Check(historyTasks[1]).Return(false)
			},
			expectedTasks:   []persistence.Task{historyTasks[0]},
			expectedToken:   []byte("next-page"),
			expectedNextKey: historyTasks[1].GetTaskKey().Next(),
		},
		{
			name: "error from execution manager",
			request: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
						ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
				},
				PageSize: 10,
			},
			mockSetup: func(mockShard *shard.MockContext, mockExec *persistence.MockExecutionManager, mockPredicate *MockPredicate) {
				mockShard.EXPECT().GetExecutionManager().Return(mockExec)
				mockExec.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			expectedError: assert.AnError,
		},
		{
			name: "empty task list",
			request: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
						ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
					},
					NextPageToken: nil,
					NextTaskKey:   persistence.NewHistoryTaskKey(time.Unix(0, 0), 1),
				},
				PageSize: 10,
			},
			mockSetup: func(mockShard *shard.MockContext, mockExec *persistence.MockExecutionManager, mockPredicate *MockPredicate) {
				mockShard.EXPECT().GetExecutionManager().Return(mockExec)
				mockExec.EXPECT().GetHistoryTasks(gomock.Any(), gomock.Any()).Return(&persistence.GetHistoryTasksResponse{
					Tasks:         []persistence.Task{},
					NextPageToken: nil,
				}, nil)
			},
			expectedTasks:   []persistence.Task{},
			expectedToken:   nil,
			expectedNextKey: persistence.NewHistoryTaskKey(time.Unix(0, math.MaxInt64), 10),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			mockShard := shard.NewMockContext(controller)
			mockExec := persistence.NewMockExecutionManager(controller)
			mockPredicate := NewMockPredicate(controller)
			tt.mockSetup(mockShard, mockExec, mockPredicate)

			reader := NewQueueReader(mockShard, persistence.HistoryTaskCategoryTimer)
			tt.request.Predicate = mockPredicate
			resp, err := reader.GetTask(context.Background(), tt.request)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTasks, resp.Tasks)
			assert.Equal(t, tt.expectedToken, resp.Progress.NextPageToken)
			assert.Equal(t, tt.expectedNextKey, resp.Progress.NextTaskKey)
		})
	}
}
