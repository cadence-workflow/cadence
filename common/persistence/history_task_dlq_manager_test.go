// Copyright (c) 2025 Uber Technologies, Inc.
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

package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
)

func TestHistoryTaskDLQManager_CreateHistoryDLQTask(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	testTask := &ActivityTask{
		WorkflowIdentifier: WorkflowIdentifier{
			DomainID:   "test-domain",
			WorkflowID: "test-workflow",
			RunID:      "test-run",
		},
		TaskData: TaskData{
			Version:             1,
			TaskID:              42,
			VisibilityTimestamp: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		},
		TargetDomainID: "target-domain",
		TaskList:       "test-tasklist",
	}

	serializedBlob := DataBlob{
		Data:     []byte("serialized-task"),
		Encoding: constants.EncodingTypeThriftRW,
	}

	tests := []struct {
		name      string
		mockSetup func(*MockHistoryDLQTaskStore, *MockHistoryTaskSerializer)
		wantErr   string
	}{
		{
			name: "successful write",
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				ser.EXPECT().
					SerializeTask(HistoryTaskCategoryTransfer, testTask).
					Return(serializedBlob, nil)
				store.EXPECT().
					CreateHistoryDLQTask(gomock.Any(), gomock.AssignableToTypeOf(InternalCreateHistoryDLQTaskRequest{})).
					DoAndReturn(func(_ context.Context, req InternalCreateHistoryDLQTaskRequest) error {
						assert.Equal(t, 1, req.ShardID)
						assert.Equal(t, "test-domain", req.DomainID)
						assert.Equal(t, "scope", req.ClusterAttributeScope)
						assert.Equal(t, "cluster-a", req.ClusterAttributeName)
						assert.Equal(t, int64(42), req.TaskID)
						assert.Equal(t, now, req.CreatedAt)
						assert.Equal(t, serializedBlob.Data, req.TaskBlob.Data)
						return nil
					})
			},
		},
		{
			name: "serialization failure",
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				ser.EXPECT().
					SerializeTask(HistoryTaskCategoryTransfer, testTask).
					Return(DataBlob{}, errors.New("codec error"))
				// store must NOT be called
			},
			wantErr: "failed to serialize history DLQ task: codec error",
		},
		{
			name: "store error propagation",
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				ser.EXPECT().
					SerializeTask(HistoryTaskCategoryTransfer, testTask).
					Return(serializedBlob, nil)
				store.EXPECT().
					CreateHistoryDLQTask(gomock.Any(), gomock.Any()).
					Return(errors.New("cassandra unavailable"))
			},
			wantErr: "cassandra unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			mockSerializer := NewMockHistoryTaskSerializer(ctrl)
			tc.mockSetup(mockStore, mockSerializer)

			mgr := &historyTaskDLQManagerImpl{
				persistence:    mockStore,
				taskSerializer: mockSerializer,
				logger:         log.NewNoop(),
				timeSrc:        clock.NewMockedTimeSourceAt(now),
			}
			err := mgr.CreateHistoryDLQTask(context.Background(), CreateHistoryDLQTaskRequest{
				ShardID:               1,
				DomainID:              "test-domain",
				ClusterAttributeScope: "scope",
				ClusterAttributeName:  "cluster-a",
				Task:                  testTask,
			})

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHistoryTaskDLQManager_GetName(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockHistoryDLQTaskStore(ctrl)
	mockStore.EXPECT().GetName().Return("cassandra")

	mgr := NewHistoryTaskDLQManager(mockStore, NewMockHistoryTaskSerializer(ctrl), log.NewNoop())
	assert.Equal(t, "cassandra", mgr.GetName())
}

func TestHistoryTaskDLQManager_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := NewMockHistoryDLQTaskStore(ctrl)
	mockStore.EXPECT().Close()

	mgr := NewHistoryTaskDLQManager(mockStore, NewMockHistoryTaskSerializer(ctrl), log.NewNoop())
	mgr.Close()
}

func TestHistoryTaskDLQManager_GetAckLevels(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	storeRows := []*InternalHistoryDLQAckLevel{
		{
			ShardID:               3,
			DomainID:              "domain-1",
			ClusterAttributeScope: "scope",
			ClusterAttributeName:  "cluster-a",
			TaskType:              HistoryTaskCategoryIDTransfer,
			AckLevelVisibilityTS:  now,
			AckLevelTaskID:        100,
			LastUpdatedAt:         now,
		},
	}

	tests := []struct {
		name       string
		mockSetup  func(*MockHistoryDLQTaskStore)
		wantLevels []HistoryDLQAckLevel
		wantErr    string
	}{
		{
			name: "returns converted ack levels",
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					GetHistoryDLQAckLevels(gomock.Any(), InternalGetHistoryDLQAckLevelsRequest{ShardID: 3}).
					Return(InternalGetHistoryDLQAckLevelsResponse{AckLevels: storeRows}, nil)
			},
			wantLevels: []HistoryDLQAckLevel{
				{
					ShardID:               3,
					DomainID:              "domain-1",
					ClusterAttributeScope: "scope",
					ClusterAttributeName:  "cluster-a",
					TaskType:              HistoryTaskCategoryIDTransfer,
					AckLevelVisibilityTS:  now,
					AckLevelTaskID:        100,
				},
			},
		},
		{
			name: "store error propagates",
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).
					Return(InternalGetHistoryDLQAckLevelsResponse{}, errors.New("db error"))
			},
			wantErr: "db error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			tc.mockSetup(mockStore)

			mgr := NewHistoryTaskDLQManager(mockStore, NewMockHistoryTaskSerializer(ctrl), log.NewNoop())
			got, err := mgr.GetAckLevels(context.Background(), 3)

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantLevels, got)
			}
		})
	}
}

func TestHistoryTaskDLQManager_GetAckLevelsForPartition(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		mockSetup func(*MockHistoryDLQTaskStore)
		wantErr   string
	}{
		{
			name: "passes all filter fields to store",
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					GetHistoryDLQAckLevels(gomock.Any(), InternalGetHistoryDLQAckLevelsRequest{
						ShardID:               5,
						DomainID:              "domain-x",
						ClusterAttributeScope: "scope",
						ClusterAttributeName:  "cluster-b",
					}).
					Return(InternalGetHistoryDLQAckLevelsResponse{
						AckLevels: []*InternalHistoryDLQAckLevel{
							{ShardID: 5, DomainID: "domain-x", TaskType: HistoryTaskCategoryIDTimer,
								AckLevelVisibilityTS: now, AckLevelTaskID: 50},
						},
					}, nil)
			},
		},
		{
			name: "store error propagates",
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					GetHistoryDLQAckLevels(gomock.Any(), gomock.Any()).
					Return(InternalGetHistoryDLQAckLevelsResponse{}, errors.New("timeout"))
			},
			wantErr: "timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			tc.mockSetup(mockStore)

			mgr := NewHistoryTaskDLQManager(mockStore, NewMockHistoryTaskSerializer(ctrl), log.NewNoop())
			_, err := mgr.GetAckLevelsForPartition(context.Background(), HistoryDLQGetAckLevelsRequest{
				ShardID:               5,
				DomainID:              "domain-x",
				ClusterAttributeScope: "scope",
				ClusterAttributeName:  "cluster-b",
			})

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHistoryTaskDLQManager_GetTasks(t *testing.T) {
	minKey := NewImmediateTaskKey(10)
	maxKey := NewImmediateTaskKey(20)
	taskBlob := &DataBlob{Data: []byte("task-bytes"), Encoding: constants.EncodingTypeThriftRW}

	storeTask := &InternalHistoryDLQTask{
		TaskType:    HistoryTaskCategoryIDTransfer,
		TaskID:      15,
		TaskPayload: taskBlob,
	}
	deserializedTask := &ActivityTask{TaskData: TaskData{TaskID: 15}}

	tests := []struct {
		name      string
		request   HistoryDLQGetTasksRequest
		mockSetup func(*MockHistoryDLQTaskStore, *MockHistoryTaskSerializer)
		wantTasks []Task
		wantErr   string
	}{
		{
			name: "converts keys and deserializes tasks",
			request: HistoryDLQGetTasksRequest{
				ShardID:             1,
				DomainID:            "dom",
				TaskType:            HistoryTaskCategoryIDTransfer,
				InclusiveMinTaskKey: minKey,
				ExclusiveMaxTaskKey: maxKey,
				PageSize:            10,
			},
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				store.EXPECT().
					GetHistoryDLQTasks(gomock.Any(), InternalGetHistoryDLQTasksRequest{
						ShardID:                  1,
						DomainID:                 "dom",
						TaskType:                 HistoryTaskCategoryIDTransfer,
						ExclusiveMinVisibilityTS: minKey.GetScheduledTime(),
						ExclusiveMinTaskID:       minKey.GetTaskID() - 1,
						InclusiveMaxVisibilityTS: maxKey.GetScheduledTime(),
						InclusiveMaxTaskID:       maxKey.GetTaskID() - 1,
						PageSize:                 10,
					}).
					Return(InternalGetHistoryDLQTasksResponse{
						Tasks:         []*InternalHistoryDLQTask{storeTask},
						NextPageToken: []byte("token"),
					}, nil)
				ser.EXPECT().
					DeserializeTask(HistoryTaskCategoryTransfer, taskBlob).
					Return(deserializedTask, nil)
			},
			wantTasks: []Task{deserializedTask},
		},
		{
			name: "unknown task type returns error",
			request: HistoryDLQGetTasksRequest{
				ShardID:  1,
				TaskType: 99,
			},
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				store.EXPECT().
					GetHistoryDLQTasks(gomock.Any(), gomock.Any()).
					Return(InternalGetHistoryDLQTasksResponse{Tasks: []*InternalHistoryDLQTask{storeTask}}, nil)
			},
			wantErr: "unknown history task type: 99",
		},
		{
			name: "deserialization error surfaces",
			request: HistoryDLQGetTasksRequest{
				ShardID:  1,
				TaskType: HistoryTaskCategoryIDTransfer,
			},
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				store.EXPECT().
					GetHistoryDLQTasks(gomock.Any(), gomock.Any()).
					Return(InternalGetHistoryDLQTasksResponse{Tasks: []*InternalHistoryDLQTask{storeTask}}, nil)
				ser.EXPECT().
					DeserializeTask(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("bad encoding"))
			},
			wantErr: "failed to deserialize history DLQ task: bad encoding",
		},
		{
			name: "store error propagates",
			request: HistoryDLQGetTasksRequest{
				ShardID:  1,
				TaskType: HistoryTaskCategoryIDTransfer,
			},
			mockSetup: func(store *MockHistoryDLQTaskStore, ser *MockHistoryTaskSerializer) {
				store.EXPECT().
					GetHistoryDLQTasks(gomock.Any(), gomock.Any()).
					Return(InternalGetHistoryDLQTasksResponse{}, errors.New("cassandra down"))
			},
			wantErr: "cassandra down",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			mockSerializer := NewMockHistoryTaskSerializer(ctrl)
			tc.mockSetup(mockStore, mockSerializer)

			mgr := NewHistoryTaskDLQManager(mockStore, mockSerializer, log.NewNoop())
			resp, err := mgr.GetTasks(context.Background(), tc.request)

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantTasks, resp.Tasks)
			}
		})
	}
}

func TestHistoryTaskDLQManager_UpdateAckLevel(t *testing.T) {
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		request   HistoryDLQUpdateAckLevelRequest
		mockSetup func(*MockHistoryDLQTaskStore)
		wantErr   string
	}{
		{
			name: "passes all fields to store with LastUpdatedAt set",
			request: HistoryDLQUpdateAckLevelRequest{
				ShardID:                   2,
				DomainID:                  "dom",
				ClusterAttributeScope:     "scope",
				ClusterAttributeName:      "cluster",
				TaskType:                  HistoryTaskCategoryIDReplication,
				UpdatedInclusiveReadLevel: NewImmediateTaskKey(77),
			},
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					UpdateHistoryDLQAckLevel(gomock.Any(), gomock.AssignableToTypeOf(InternalUpdateHistoryDLQAckLevelRequest{})).
					DoAndReturn(func(_ context.Context, req InternalUpdateHistoryDLQAckLevelRequest) error {
						assert.Equal(t, 2, req.Row.ShardID)
						assert.Equal(t, "dom", req.Row.DomainID)
						assert.Equal(t, HistoryTaskCategoryIDReplication, req.Row.TaskType)
						assert.Equal(t, int64(77), req.Row.AckLevelTaskID)
						assert.Equal(t, time.Unix(0, 0).UTC(), req.Row.AckLevelVisibilityTS)
						assert.Equal(t, now, req.Row.LastUpdatedAt)
						return nil
					})
			},
		},
		{
			name:    "store error propagates",
			request: HistoryDLQUpdateAckLevelRequest{ShardID: 1},
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					UpdateHistoryDLQAckLevel(gomock.Any(), gomock.Any()).
					Return(errors.New("write error"))
			},
			wantErr: "write error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			tc.mockSetup(mockStore)

			mgr := &historyTaskDLQManagerImpl{
				persistence:    mockStore,
				taskSerializer: NewMockHistoryTaskSerializer(ctrl),
				logger:         log.NewNoop(),
				timeSrc:        clock.NewMockedTimeSourceAt(now),
			}
			err := mgr.UpdateAckLevel(context.Background(), tc.request)

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHistoryTaskDLQManager_DeleteTasks(t *testing.T) {
	maxKey := NewImmediateTaskKey(50)

	tests := []struct {
		name      string
		request   HistoryDLQDeleteTasksRequest
		mockSetup func(*MockHistoryDLQTaskStore)
		wantErr   string
	}{
		{
			name: "converts ExclusiveMaxTaskKey to inclusive store boundary",
			request: HistoryDLQDeleteTasksRequest{
				ShardID:               4,
				DomainID:              "dom",
				ClusterAttributeScope: "scope",
				ClusterAttributeName:  "cluster",
				TaskType:              HistoryTaskCategoryIDTimer,
				ExclusiveMaxTaskKey:   maxKey,
			},
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					RangeDeleteHistoryDLQTasks(gomock.Any(), InternalRangeDeleteHistoryDLQTasksRequest{
						ShardID:               4,
						DomainID:              "dom",
						ClusterAttributeScope: "scope",
						ClusterAttributeName:  "cluster",
						TaskType:              HistoryTaskCategoryIDTimer,
						AckLevelVisibilityTS:  maxKey.GetScheduledTime(),
						AckLevelTaskID:        maxKey.GetTaskID() - 1,
					}).
					Return(nil)
			},
		},
		{
			name:    "store error propagates",
			request: HistoryDLQDeleteTasksRequest{ShardID: 1, ExclusiveMaxTaskKey: maxKey},
			mockSetup: func(store *MockHistoryDLQTaskStore) {
				store.EXPECT().
					RangeDeleteHistoryDLQTasks(gomock.Any(), gomock.Any()).
					Return(errors.New("delete error"))
			},
			wantErr: "delete error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStore := NewMockHistoryDLQTaskStore(ctrl)
			tc.mockSetup(mockStore)

			mgr := NewHistoryTaskDLQManager(mockStore, NewMockHistoryTaskSerializer(ctrl), log.NewNoop())
			err := mgr.DeleteTasks(context.Background(), tc.request)

			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHistoryTaskCategoryForType(t *testing.T) {
	tests := []struct {
		taskType int
		want     HistoryTaskCategory
		wantErr  bool
	}{
		{HistoryTaskCategoryIDTransfer, HistoryTaskCategoryTransfer, false},
		{HistoryTaskCategoryIDTimer, HistoryTaskCategoryTimer, false},
		{HistoryTaskCategoryIDReplication, HistoryTaskCategoryReplication, false},
		{99, HistoryTaskCategory{}, true},
	}
	for _, tc := range tests {
		got, err := historyTaskCategoryForType(tc.taskType)
		if tc.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		}
	}
}
