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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
)

func newTestSemaphoreTaskManager(store SemaphoreTaskStore, timeSrc clock.TimeSource, t *testing.T) *semaphoreTaskManagerImpl {
	return &semaphoreTaskManagerImpl{
		persistence: store,
		logger:      testlogger.New(t),
		timeSrc:     timeSrc,
	}
}

func TestSemaphoreTaskManagerClaimSemaphoreTaskBucket(t *testing.T) {
	tests := []struct {
		name      string
		request   *ClaimSemaphoreTaskBucketRequest
		setupMock func(store *MockSemaphoreTaskStore)
		wantErr   bool
		want      *ClaimSemaphoreTaskBucketResponse
	}{
		{
			name:    "success",
			request: &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).
					Return(&ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42}, nil).Times(1)
			},
			want: &ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42},
		},
		{
			name:    "renew passes the held range id through",
			request: &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 7},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, req *ClaimSemaphoreTaskBucketRequest) (*ClaimSemaphoreTaskBucketResponse, error) {
						assert.Equal(t, int64(7), req.RangeID)
						return &ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42}, nil
					}).Times(1)
			},
			want: &ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42},
		},
		{
			name:      "negative range id",
			request:   &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: -1},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:      "missing domain id",
			request:   &ClaimSemaphoreTaskBucketRequest{SemaphoreName: "sem-1", Bucket: 3},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:      "missing semaphore name",
			request:   &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", Bucket: 3},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:      "negative bucket",
			request:   &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: -1},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:    "store error is propagated",
			request: &ClaimSemaphoreTaskBucketRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().ClaimSemaphoreTaskBucket(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("store failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
			resp, err := m.ClaimSemaphoreTaskBucket(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, resp)
		})
	}
}

func TestSemaphoreTaskManagerGetSemaphoreTaskBucketState(t *testing.T) {
	tests := []struct {
		name      string
		request   *GetSemaphoreTaskBucketStateRequest
		setupMock func(store *MockSemaphoreTaskStore)
		wantErr   bool
	}{
		{
			name:    "success",
			request: &GetSemaphoreTaskBucketStateRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().GetSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).
					Return(&GetSemaphoreTaskBucketStateResponse{RangeID: 5, AckLevel: 20}, nil).Times(1)
			},
		},
		{
			name:      "missing domain id",
			request:   &GetSemaphoreTaskBucketStateRequest{SemaphoreName: "sem-1"},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
			resp, err := m.GetSemaphoreTaskBucketState(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, &GetSemaphoreTaskBucketStateResponse{RangeID: 5, AckLevel: 20}, resp)
		})
	}
}

func TestSemaphoreTaskManagerUpdateSemaphoreTaskBucketState(t *testing.T) {
	tests := []struct {
		name      string
		request   *UpdateSemaphoreTaskBucketStateRequest
		setupMock func(store *MockSemaphoreTaskStore)
		wantErr   bool
	}{
		{
			name:    "success",
			request: &UpdateSemaphoreTaskBucketStateRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 7, AckLevel: 100},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).
					Return(&UpdateSemaphoreTaskBucketStateResponse{}, nil).Times(1)
			},
		},
		{
			// A bucket that has acked nothing yet sits at 0, so 0 is a legal cursor.
			name:    "zero ack level",
			request: &UpdateSemaphoreTaskBucketStateRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 7, AckLevel: 0},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().UpdateSemaphoreTaskBucketState(gomock.Any(), gomock.Any()).
					Return(&UpdateSemaphoreTaskBucketStateResponse{}, nil).Times(1)
			},
		},
		{
			name:      "non-positive range id",
			request:   &UpdateSemaphoreTaskBucketStateRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 0},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:      "negative ack level",
			request:   &UpdateSemaphoreTaskBucketStateRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 7, AckLevel: -1},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name:      "missing semaphore name",
			request:   &UpdateSemaphoreTaskBucketStateRequest{DomainID: "domain-1", Bucket: 3, RangeID: 7},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
			resp, err := m.UpdateSemaphoreTaskBucketState(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestSemaphoreTaskManagerCreateSemaphoreTasks(t *testing.T) {
	fixedTime := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	earlier := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		request   *CreateSemaphoreTasksRequest
		setupMock func(store *MockSemaphoreTaskStore)
		wantErr   bool
		wantTask  *SemaphoreTask
	}{
		{
			name: "missing CreatedTime is stamped from timeSrc",
			request: &CreateSemaphoreTasksRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 5,
				Tasks: []*SemaphoreTask{{TaskID: 100, WorkflowID: "wf-1", RunID: "run-1", HoldID: 11}},
			},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(&CreateSemaphoreTasksResponse{}, nil).Times(1)
			},
			wantTask: &SemaphoreTask{TaskID: 100, WorkflowID: "wf-1", RunID: "run-1", HoldID: 11, CreatedTime: fixedTime},
		},
		{
			name: "existing CreatedTime is preserved",
			request: &CreateSemaphoreTasksRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 5,
				Tasks: []*SemaphoreTask{{TaskID: 100, WorkflowID: "wf-1", CreatedTime: earlier}},
			},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(&CreateSemaphoreTasksResponse{}, nil).Times(1)
			},
			wantTask: &SemaphoreTask{TaskID: 100, WorkflowID: "wf-1", CreatedTime: earlier},
		},
		{
			name: "empty tasks rejected",
			request: &CreateSemaphoreTasksRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 5,
			},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name: "non-positive range id rejected",
			request: &CreateSemaphoreTasksRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 0,
				Tasks: []*SemaphoreTask{{TaskID: 100}},
			},
			setupMock: func(store *MockSemaphoreTaskStore) {},
			wantErr:   true,
		},
		{
			name: "store error is propagated",
			request: &CreateSemaphoreTasksRequest{
				DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, RangeID: 5,
				Tasks: []*SemaphoreTask{{TaskID: 100}},
			},
			setupMock: func(store *MockSemaphoreTaskStore) {
				store.EXPECT().CreateSemaphoreTasks(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("store failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			tc.setupMock(store)

			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSourceAt(fixedTime), t)
			resp, err := m.CreateSemaphoreTasks(context.Background(), tc.request)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			require.Len(t, tc.request.Tasks, 1)
			assert.Equal(t, tc.wantTask, tc.request.Tasks[0])
		})
	}
}

func TestSemaphoreTaskManagerGetSemaphoreTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	req := &GetSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, MaxReadLevel: 1000, BatchSize: 10}
	want := &GetSemaphoreTasksResponse{Tasks: []*SemaphoreTask{{TaskID: 100, WorkflowID: "wf-1"}}}
	store.EXPECT().GetSemaphoreTasks(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.GetSemaphoreTasks(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestSemaphoreTaskManagerGetSemaphoreTasksValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *GetSemaphoreTasksRequest
	}{
		{
			name: "missing domain id",
			req:  &GetSemaphoreTasksRequest{SemaphoreName: "sem-1", BatchSize: 10},
		},
		{
			// Zero would disable both the page limit and the store's early exit, turning the
			// read into an unbounded scan rather than reading nothing.
			name: "zero batch size",
			req:  &GetSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, MaxReadLevel: 1000},
		},
		{
			name: "negative batch size",
			req:  &GetSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, MaxReadLevel: 1000, BatchSize: -1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			// No store call is expected: gomock fails the test if the manager delegates.
			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)

			resp, err := m.GetSemaphoreTasks(context.Background(), tc.req)
			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestSemaphoreTaskManagerRangeCompleteSemaphoreTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	req := &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, AckLevel: 100}
	want := &RangeCompleteSemaphoreTasksResponse{RowsDeleted: UnknownNumRowsAffected}
	store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.RangeCompleteSemaphoreTasks(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestSemaphoreTaskManagerRangeCompleteSemaphoreTasksValidation(t *testing.T) {
	tests := []struct {
		name string
		req  *RangeCompleteSemaphoreTasksRequest
	}{
		{
			name: "missing semaphore name",
			req:  &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1"},
		},
		{
			name: "negative read level",
			req:  &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, ReadLevel: -1, AckLevel: 100},
		},
		{
			// An inverted range deletes nothing but reports success, so it has to be rejected
			// here rather than silently no-op in the store.
			name: "ack level below read level",
			req:  &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, ReadLevel: 100, AckLevel: 50},
		},
		{
			name: "negative ack level",
			req:  &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, ReadLevel: 0, AckLevel: -1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := NewMockSemaphoreTaskStore(ctrl)
			// No EXPECT: validation must reject before reaching the store.

			m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
			resp, err := m.RangeCompleteSemaphoreTasks(context.Background(), tc.req)
			assert.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestSemaphoreTaskManagerRangeCompleteSemaphoreTasksEqualLevels(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	// ReadLevel == AckLevel is an empty but legal range: nothing new has been acked since the
	// last cleanup. It must reach the store rather than be rejected as inverted.
	req := &RangeCompleteSemaphoreTasksRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, ReadLevel: 100, AckLevel: 100}
	want := &RangeCompleteSemaphoreTasksResponse{RowsDeleted: UnknownNumRowsAffected}
	store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.RangeCompleteSemaphoreTasks(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestSemaphoreTaskManagerGetSemaphoreTasksCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	req := &GetSemaphoreTasksCountRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: 3, ReadLevel: 42}
	want := &GetSemaphoreTasksCountResponse{Count: 3}
	store.EXPECT().GetSemaphoreTasksCount(gomock.Any(), req).Return(want, nil).Times(1)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.GetSemaphoreTasksCount(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, want, resp)
}

func TestSemaphoreTaskManagerGetSemaphoreTasksCountValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	resp, err := m.GetSemaphoreTasksCount(context.Background(), &GetSemaphoreTasksCountRequest{DomainID: "domain-1", SemaphoreName: "sem-1", Bucket: -1})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSemaphoreTaskManagerGetNameAndClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := NewMockSemaphoreTaskStore(ctrl)

	store.EXPECT().GetName().Return("cassandra").Times(1)
	store.EXPECT().Close().Times(1)

	m := newTestSemaphoreTaskManager(store, clock.NewMockedTimeSource(), t)
	assert.Equal(t, "cassandra", m.GetName())
	m.Close()
}
