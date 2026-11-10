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

package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	testSemTaskDomainID = "domain-1"
	testSemTaskName     = "sem-1"
	testSemTaskBucket   = 3
)

func setUpMocksForSemaphoreTaskStore(t *testing.T) (*nosqlSemaphoreTaskStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)
	store := &nosqlSemaphoreTaskStore{
		nosqlStore: nosqlStore{db: dbMock},
	}
	return store, dbMock
}

func TestNoSQLClaimSemaphoreTaskBucket(t *testing.T) {
	ctx := context.Background()
	req := &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      testSemTaskDomainID,
		SemaphoreName: testSemTaskName,
		Bucket:        testSemTaskBucket,
	}

	tests := map[string]struct {
		// request overrides the shared req; nil means a steal (RangeID 0).
		request   *persistence.ClaimSemaphoreTaskBucketRequest
		setupMock func(*nosqlplugin.MockDB)
		expectErr bool
		expected  *persistence.ClaimSemaphoreTaskBucketResponse
	}{
		"first use - control row created": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(nil, errors.New("not found")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
				dbMock.EXPECT().InsertSemaphoreTaskControlRow(ctx, gomock.Any()).
					DoAndReturn(func(_ context.Context, row *nosqlplugin.SemaphoreTaskControlRow) error {
						assert.Equal(t, int64(initialSemaphoreRangeID), row.RangeID)
						assert.Equal(t, int64(initialSemaphoreAckLevel), row.AckLevel)
						return nil
					}).Times(1)
			},
			expected: &persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 1, AckLevel: 0},
		},
		"existing bucket - range_id bumped": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 7, AckLevel: 42}, nil).Times(1)
				dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
					DoAndReturn(func(_ context.Context, row *nosqlplugin.SemaphoreTaskControlRow, prev int64) error {
						assert.Equal(t, int64(8), row.RangeID)
						assert.Equal(t, int64(42), row.AckLevel)
						return nil
					}).Times(1)
			},
			expected: &persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42},
		},
		"renew - caller still holds the claimed range_id": {
			request: &persistence.ClaimSemaphoreTaskBucketRequest{
				DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket, RangeID: 7,
			},
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 7, AckLevel: 42}, nil).Times(1)
				dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
					Return(nil).Times(1)
			},
			expected: &persistence.ClaimSemaphoreTaskBucketResponse{RangeID: 8, AckLevel: 42},
		},
		"renew - caller lost the bucket, no write attempted": {
			request: &persistence.ClaimSemaphoreTaskBucketRequest{
				DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket, RangeID: 7,
			},
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				// Another host already claimed the bucket at 8. No UpdateSemaphoreTaskControlRow is expected:
				// the caller must be told it lost rather than take the bucket back.
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 8, AckLevel: 42}, nil).Times(1)
			},
			expectErr: true,
		},
		"renew - control row missing, bucket not recreated": {
			request: &persistence.ClaimSemaphoreTaskBucketRequest{
				DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket, RangeID: 7,
			},
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				// No InsertSemaphoreTaskControlRow is expected: the row was lost out of band, and
				// recreating it would report a successful renew of a bucket reset to ack_level 0.
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(nil, errors.New("not found")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			expectErr: true,
		},
		"insert fence conflict maps to ConditionFailedError": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(nil, errors.New("not found")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
				dbMock.EXPECT().InsertSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(&nosqlplugin.TaskOperationConditionFailure{RangeID: 1}).Times(1)
			},
			expectErr: true,
		},
		"update fence conflict maps to ConditionFailedError": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 7}, nil).Times(1)
				dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
					Return(&nosqlplugin.TaskOperationConditionFailure{RangeID: 9}).Times(1)
			},
			expectErr: true,
		},
		"select generic error propagates": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
					Return(nil, errors.New("db error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
				expectNotACommonError(dbMock)
			},
			expectErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForSemaphoreTaskStore(t)
			tc.setupMock(dbMock)

			request := tc.request
			if request == nil {
				request = req
			}
			resp, err := store.ClaimSemaphoreTaskBucket(ctx, request)

			if tc.expectErr {
				require.Error(t, err)
				assert.Nil(t, resp)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, resp)
		})
	}
}

// Both ways a claim can be refused must surface as the same error type, since callers branch on
// it to unload the bucket: the CAS losing a race, and the renew check rejecting a stale claim.
func TestNoSQLClaimSemaphoreTaskBucketConditionType(t *testing.T) {
	ctx := context.Background()
	req := &persistence.ClaimSemaphoreTaskBucketRequest{DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket}

	t.Run("CAS conflict", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
			Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 7}, nil).Times(1)
		dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
			Return(&nosqlplugin.TaskOperationConditionFailure{RangeID: 9}).Times(1)

		_, err := store.ClaimSemaphoreTaskBucket(ctx, req)
		require.Error(t, err)
		_, ok := err.(*persistence.ConditionFailedError)
		assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
	})

	t.Run("stale renew claim", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
			Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 9}, nil).Times(1)

		renew := *req
		renew.RangeID = 7
		_, err := store.ClaimSemaphoreTaskBucket(ctx, &renew)
		require.Error(t, err)
		_, ok := err.(*persistence.ConditionFailedError)
		assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
	})

	t.Run("renew against a missing control row", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
			Return(nil, errors.New("not found")).Times(1)
		dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)

		renew := *req
		renew.RangeID = 7
		_, err := store.ClaimSemaphoreTaskBucket(ctx, &renew)
		require.Error(t, err)
		_, ok := err.(*persistence.ConditionFailedError)
		assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
	})
}

func TestNoSQLGetSemaphoreTaskBucketState(t *testing.T) {
	ctx := context.Background()
	req := &persistence.GetSemaphoreTaskBucketStateRequest{DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket}

	t.Run("success", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
			Return(&nosqlplugin.SemaphoreTaskControlRow{RangeID: 5, AckLevel: 20}, nil).Times(1)

		resp, err := store.GetSemaphoreTaskBucketState(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, &persistence.GetSemaphoreTaskBucketStateResponse{RangeID: 5, AckLevel: 20}, resp)
	})

	t.Run("error propagates", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTaskControlRow(ctx, gomock.Any()).
			Return(nil, errors.New("db error")).Times(1)
		expectNotACommonError(dbMock)

		resp, err := store.GetSemaphoreTaskBucketState(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestNoSQLUpdateSemaphoreTaskBucketState(t *testing.T) {
	ctx := context.Background()
	req := &persistence.UpdateSemaphoreTaskBucketStateRequest{
		DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket,
		RangeID: 7, AckLevel: 100,
	}

	t.Run("success is fenced by RangeID", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
			DoAndReturn(func(_ context.Context, row *nosqlplugin.SemaphoreTaskControlRow, prev int64) error {
				assert.Equal(t, int64(7), row.RangeID)
				assert.Equal(t, int64(100), row.AckLevel)
				return nil
			}).Times(1)

		resp, err := store.UpdateSemaphoreTaskBucketState(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, &persistence.UpdateSemaphoreTaskBucketStateResponse{}, resp)
	})

	t.Run("fence conflict maps to ConditionFailedError", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().UpdateSemaphoreTaskControlRow(ctx, gomock.Any(), int64(7)).
			Return(&nosqlplugin.TaskOperationConditionFailure{RangeID: 8}).Times(1)

		resp, err := store.UpdateSemaphoreTaskBucketState(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)
		_, ok := err.(*persistence.ConditionFailedError)
		assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
	})
}

func TestNoSQLCreateSemaphoreTasks(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()
	deadline := now.Add(time.Hour)
	req := &persistence.CreateSemaphoreTasksRequest{
		DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket, RangeID: 5,
		Tasks: []*persistence.SemaphoreTask{
			{TaskID: 100, WorkflowID: "wf-1", RunID: "run-1", HoldID: 11, AcquireDeadline: &deadline, CreatedTime: now},
		},
	}

	t.Run("success maps tasks and fences by RangeID", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().InsertSemaphoreTasks(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, tasks []*nosqlplugin.SemaphoreTaskRow, control *nosqlplugin.SemaphoreTaskControlRow) error {
				require.Len(t, tasks, 1)
				assert.Equal(t, int64(100), tasks[0].TaskID)
				assert.Equal(t, "wf-1", tasks[0].WorkflowID)
				assert.Equal(t, int64(11), tasks[0].HoldID)
				assert.Equal(t, &deadline, tasks[0].AcquireDeadline)
				assert.Equal(t, int64(5), control.RangeID)
				return nil
			}).Times(1)

		resp, err := store.CreateSemaphoreTasks(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, &persistence.CreateSemaphoreTasksResponse{}, resp)
	})

	t.Run("fence conflict maps to ConditionFailedError", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().InsertSemaphoreTasks(ctx, gomock.Any(), gomock.Any()).
			Return(&nosqlplugin.TaskOperationConditionFailure{RangeID: 6}).Times(1)

		resp, err := store.CreateSemaphoreTasks(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)
		_, ok := err.(*persistence.ConditionFailedError)
		assert.True(t, ok, "expected *persistence.ConditionFailedError, got %T", err)
	})
}

func TestNoSQLGetSemaphoreTasks(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1234567890, 0).UTC()
	req := &persistence.GetSemaphoreTasksRequest{
		DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket,
		ReadLevel: 0, MaxReadLevel: 1000, BatchSize: 10,
	}

	t.Run("success maps rows to tasks", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTasks(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, filter *nosqlplugin.SemaphoreTasksFilter) ([]*nosqlplugin.SemaphoreTaskRow, error) {
				assert.Equal(t, int64(0), filter.ExclusiveMinTaskID)
				assert.Equal(t, int64(1000), filter.InclusiveMaxTaskID)
				assert.Equal(t, 10, filter.BatchSize)
				return []*nosqlplugin.SemaphoreTaskRow{
					{TaskID: 100, WorkflowID: "wf-1", RunID: "run-1", HoldID: 11, CreatedTime: now},
				}, nil
			}).Times(1)

		resp, err := store.GetSemaphoreTasks(ctx, req)
		assert.NoError(t, err)
		require.Len(t, resp.Tasks, 1)
		assert.Equal(t, &persistence.SemaphoreTask{TaskID: 100, WorkflowID: "wf-1", RunID: "run-1", HoldID: 11, CreatedTime: now}, resp.Tasks[0])
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTasks(ctx, gomock.Any()).Return(nil, nil).Times(1)

		resp, err := store.GetSemaphoreTasks(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp.Tasks)
		assert.Empty(t, resp.Tasks)
	})

	t.Run("error propagates", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTasks(ctx, gomock.Any()).Return(nil, errors.New("db error")).Times(1)
		expectNotACommonError(dbMock)

		resp, err := store.GetSemaphoreTasks(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("inverted range short-circuits without querying", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().SelectSemaphoreTasks(gomock.Any(), gomock.Any()).Times(0)

		resp, err := store.GetSemaphoreTasks(ctx, &persistence.GetSemaphoreTasksRequest{
			DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket,
			ReadLevel: 1000, MaxReadLevel: 999, BatchSize: 10,
		})
		assert.NoError(t, err)
		assert.Empty(t, resp.Tasks)
	})
}

func TestNoSQLRangeCompleteSemaphoreTasks(t *testing.T) {
	ctx := context.Background()
	req := &persistence.RangeCompleteSemaphoreTasksRequest{
		DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket,
		ReadLevel: 0, AckLevel: 100,
	}

	t.Run("success returns rows deleted", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().RangeDeleteSemaphoreTasks(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (int, error) {
				assert.Equal(t, int64(0), filter.ExclusiveMinTaskID)
				assert.Equal(t, int64(100), filter.InclusiveMaxTaskID)
				return persistence.UnknownNumRowsAffected, nil
			}).Times(1)

		resp, err := store.RangeCompleteSemaphoreTasks(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, persistence.UnknownNumRowsAffected, resp.RowsDeleted)
	})

	t.Run("error propagates", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().RangeDeleteSemaphoreTasks(ctx, gomock.Any()).Return(0, errors.New("db error")).Times(1)
		expectNotACommonError(dbMock)

		resp, err := store.RangeCompleteSemaphoreTasks(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestNoSQLGetSemaphoreTasksCount(t *testing.T) {
	ctx := context.Background()
	req := &persistence.GetSemaphoreTasksCountRequest{
		DomainID: testSemTaskDomainID, SemaphoreName: testSemTaskName, Bucket: testSemTaskBucket, ReadLevel: 42,
	}

	t.Run("success", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().GetSemaphoreTasksCount(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (int64, error) {
				assert.Equal(t, int64(42), filter.ExclusiveMinTaskID)
				return int64(3), nil
			}).Times(1)

		resp, err := store.GetSemaphoreTasksCount(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), resp.Count)
	})

	t.Run("error propagates", func(t *testing.T) {
		store, dbMock := setUpMocksForSemaphoreTaskStore(t)
		dbMock.EXPECT().GetSemaphoreTasksCount(ctx, gomock.Any()).Return(int64(0), errors.New("db error")).Times(1)
		expectNotACommonError(dbMock)

		resp, err := store.GetSemaphoreTasksCount(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
