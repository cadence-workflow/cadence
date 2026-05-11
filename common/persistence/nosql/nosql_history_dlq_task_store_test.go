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
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

func setUpMocksForHistoryDLQTaskStore(t *testing.T) (*nosqlHistoryDLQTaskStore, *nosqlplugin.MockDB) {
	t.Helper()
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)
	return &nosqlHistoryDLQTaskStore{
		nosqlStore: nosqlStore{db: dbMock, logger: testlogger.New(t)},
	}, dbMock
}

func TestNoSQLHistoryDLQTaskStore_CreateHistoryDLQTask(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2025, 6, 1, 12, 0, 1, 0, time.UTC)

	baseRequest := persistence.InternalCreateHistoryDLQTaskRequest{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskType:              3,
		TaskID:                99,
		VisibilityTimestamp:   now,
		CreatedAt:             createdAt,
		TaskBlob: &persistence.DataBlob{
			Data:     []byte("task-payload"),
			Encoding: constants.EncodingTypeThriftRW,
		},
	}

	expectedTask := &nosqlplugin.HistoryDLQTaskRow{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskType:              3,
		TaskID:                99,
		VisibilityTimestamp:   now,
		Data:                  []byte("task-payload"),
		DataEncoding:          string(constants.EncodingTypeThriftRW),
		CreatedAt:             createdAt,
	}

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		request        persistence.InternalCreateHistoryDLQTaskRequest
		expectError    bool
		errorValidator func(t *testing.T, err error)
	}{
		"when insert succeeds then no error is returned": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().
					InsertHistoryDLQTaskRow(ctx, expectedTask).
					Return(nil).
					Times(1)
			},
			request: baseRequest,
		},
		"when the database returns a timeout error then TimeoutError is returned": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().
					InsertHistoryDLQTaskRow(ctx, gomock.Any()).
					Return(errors.New("context deadline exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				dbMock.EXPECT().IsTimeoutError(gomock.Any()).Return(true)
			},
			request:     baseRequest,
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var timeoutErr *persistence.TimeoutError
				assert.ErrorAs(t, err, &timeoutErr)
			},
		},
		"when the database returns a throttling error then ServiceBusyError is returned": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().
					InsertHistoryDLQTaskRow(ctx, gomock.Any()).
					Return(errors.New("rate exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				dbMock.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				dbMock.EXPECT().IsThrottlingError(gomock.Any()).Return(true)
			},
			request:     baseRequest,
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var busyErr *types.ServiceBusyError
				assert.ErrorAs(t, err, &busyErr)
			},
		},
		"when task blob is nil then InvalidPersistenceRequestError is returned": {
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				// no DB calls expected — nil check fires before any DB interaction
			},
			request: func() persistence.InternalCreateHistoryDLQTaskRequest {
				r := baseRequest
				r.TaskBlob = nil
				return r
			}(),
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var invalidReqErr *persistence.InvalidPersistenceRequestError
				assert.ErrorAs(t, err, &invalidReqErr)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
			tc.setupMock(dbMock)

			err := store.CreateHistoryDLQTask(ctx, tc.request)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					tc.errorValidator(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNoSQLHistoryDLQTaskStore_GetName(t *testing.T) {
	store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
	dbMock.EXPECT().PluginName().Return("cassandra")
	assert.Equal(t, "cassandra", store.GetName())
}

func TestNoSQLHistoryDLQTaskStore_Close(t *testing.T) {
	// Close is a no-op on the nosqlHistoryDLQTaskStore itself; the db is not closed here.
	store, _ := setUpMocksForHistoryDLQTaskStore(t)
	store.Close() // must not panic
}

// The following tests verify the implemented read/delete/ack-level methods.

func TestNoSQLHistoryDLQTaskStore_GetHistoryDLQTasks(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	maxTS := time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC)

	minKey := persistence.NewHistoryTaskKey(now, 10)
	maxKey := persistence.NewHistoryTaskKey(maxTS, 20)

	baseRequest := persistence.HistoryDLQGetTasksRequest{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskCategory:          persistence.HistoryTaskCategoryReplication,
		InclusiveMinTaskKey:   minKey,
		ExclusiveMaxTaskKey:   maxKey,
		PageSize:              5,
	}

	dbRow := &nosqlplugin.HistoryDLQTaskRow{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskType:              3,
		VisibilityTimestamp:   now,
		TaskID:                15,
		Data:                  []byte("payload"),
		DataEncoding:          string(constants.EncodingTypeThriftRW),
		CreatedAt:             now,
	}

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		request        persistence.HistoryDLQGetTasksRequest
		expectError    bool
		errorValidator func(t *testing.T, err error)
		validate       func(t *testing.T, resp persistence.InternalGetHistoryDLQTasksResponse)
	}{
		"when db returns rows then tasks are mapped and returned with next page token": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					SelectHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskFilter{
						ShardID:                  5,
						DomainID:                 "domain-abc",
						ClusterAttributeScope:    "scope-1",
						ClusterAttributeName:     "cluster-west",
						TaskType:                 3,
						InclusiveMinVisibilityTS: now,
						InclusiveMinTaskID:       10,
						ExclusiveMaxVisibilityTS: maxTS,
						ExclusiveMaxTaskID:       20,
						PageSize:                 5,
					}).
					Return([]*nosqlplugin.HistoryDLQTaskRow{dbRow}, []byte("next-page"), nil)
			},
			request: baseRequest,
			validate: func(t *testing.T, resp persistence.InternalGetHistoryDLQTasksResponse) {
				assert.Equal(t, []byte("next-page"), resp.NextPageToken)
				assert.Len(t, resp.Tasks, 1)
				task := resp.Tasks[0]
				assert.Equal(t, "domain-abc", task.DomainID)
				assert.Equal(t, int64(15), task.TaskID)
				assert.Equal(t, []byte("payload"), task.TaskPayload.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.TaskPayload.Encoding)
			},
		},
		"when db returns no rows then empty task list and nil page token are returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					SelectHistoryDLQTaskRows(ctx, gomock.Any()).
					Return(nil, nil, nil)
			},
			request: baseRequest,
			validate: func(t *testing.T, resp persistence.InternalGetHistoryDLQTasksResponse) {
				assert.Empty(t, resp.Tasks)
				assert.Nil(t, resp.NextPageToken)
			},
		},
		"when db returns a throttling error then ServiceBusyError is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().SelectHistoryDLQTaskRows(ctx, gomock.Any()).Return(nil, nil, errors.New("rate exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				db.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				db.EXPECT().IsThrottlingError(gomock.Any()).Return(true)
			},
			request:     baseRequest,
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var busyErr *types.ServiceBusyError
				assert.ErrorAs(t, err, &busyErr)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
			tc.setupMock(dbMock)

			resp, err := store.GetHistoryDLQTasks(ctx, tc.request)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					tc.errorValidator(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, resp)
				}
			}
		})
	}
}

func TestNoSQLHistoryDLQTaskStore_RangeDeleteHistoryDLQTasks(t *testing.T) {
	ctx := context.Background()
	ackTS := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	maxKey := persistence.NewHistoryTaskKey(ackTS, 42)

	baseRequest := persistence.HistoryDLQDeleteTasksRequest{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskCategory:          persistence.HistoryTaskCategoryReplication,
		ExclusiveMaxTaskKey:   maxKey,
	}

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		expectError    bool
		errorValidator func(t *testing.T, err error)
	}{
		"when db delete succeeds then no error is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					RangeDeleteHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskRangeDeleteFilter{
						ShardID:                  5,
						DomainID:                 "domain-abc",
						ClusterAttributeScope:    "scope-1",
						ClusterAttributeName:     "cluster-west",
						TaskType:                 3,
						ExclusiveMaxVisibilityTS: ackTS,
						ExclusiveMaxTaskID:       42,
					}).
					Return(nil)
			},
		},
		"when db returns a throttling error then ServiceBusyError is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().RangeDeleteHistoryDLQTaskRows(ctx, gomock.Any()).Return(errors.New("rate exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				db.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				db.EXPECT().IsThrottlingError(gomock.Any()).Return(true)
			},
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var busyErr *types.ServiceBusyError
				assert.ErrorAs(t, err, &busyErr)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
			tc.setupMock(dbMock)

			err := store.RangeDeleteHistoryDLQTasks(ctx, baseRequest)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					tc.errorValidator(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNoSQLHistoryDLQTaskStore_GetHistoryDLQAckLevels(t *testing.T) {
	ctx := context.Background()
	updatedAt := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	ackTS := time.Date(2025, 6, 1, 11, 0, 0, 0, time.UTC)

	dbRow := &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               5,
		DomainID:              "domain-abc",
		ClusterAttributeScope: "scope-1",
		ClusterAttributeName:  "cluster-west",
		TaskType:              3,
		AckLevelVisibilityTS:  ackTS,
		AckLevelTaskID:        77,
		LastUpdatedAt:         updatedAt,
	}

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		request        persistence.HistoryDLQGetAckLevelsRequest
		expectError    bool
		errorValidator func(t *testing.T, err error)
		validate       func(t *testing.T, resp persistence.InternalGetHistoryDLQAckLevelsResponse)
	}{
		"when db returns rows then ack levels are mapped and returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					SelectHistoryDLQAckLevelRows(ctx, nosqlplugin.HistoryDLQAckLevelFilter{
						ShardID:               5,
						DomainID:              "domain-abc",
						ClusterAttributeScope: "scope-1",
						ClusterAttributeName:  "cluster-west",
					}).
					Return([]*nosqlplugin.HistoryDLQAckLevelRow{dbRow}, nil)
			},
			request: persistence.HistoryDLQGetAckLevelsRequest{
				ShardID:               5,
				TaskCategory:          persistence.HistoryTaskCategoryReplication,
				DomainID:              "domain-abc",
				ClusterAttributeScope: "scope-1",
				ClusterAttributeName:  "cluster-west",
			},
			validate: func(t *testing.T, resp persistence.InternalGetHistoryDLQAckLevelsResponse) {
				assert.Len(t, resp.AckLevels, 1)
				al := resp.AckLevels[0]
				assert.Equal(t, 5, al.ShardID)
				assert.Equal(t, "domain-abc", al.DomainID)
				assert.Equal(t, int64(77), al.AckLevelTaskID)
			},
		},
		"when only shard ID is provided then all ack levels for the shard are returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					SelectHistoryDLQAckLevelRows(ctx, nosqlplugin.HistoryDLQAckLevelFilter{ShardID: 5}).
					Return(nil, nil)
			},
			request: persistence.HistoryDLQGetAckLevelsRequest{
				ShardID:      5,
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
			},
			validate: func(t *testing.T, resp persistence.InternalGetHistoryDLQAckLevelsResponse) {
				assert.Empty(t, resp.AckLevels)
			},
		},
		"when db returns a throttling error then ServiceBusyError is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().SelectHistoryDLQAckLevelRows(ctx, gomock.Any()).Return(nil, errors.New("rate exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				db.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				db.EXPECT().IsThrottlingError(gomock.Any()).Return(true)
			},
			request: persistence.HistoryDLQGetAckLevelsRequest{
				ShardID:      5,
				TaskCategory: persistence.HistoryTaskCategoryTransfer,
			},
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var busyErr *types.ServiceBusyError
				assert.ErrorAs(t, err, &busyErr)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
			tc.setupMock(dbMock)

			resp, err := store.GetHistoryDLQAckLevels(ctx, tc.request)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					tc.errorValidator(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, resp)
				}
			}
		})
	}
}

func TestNoSQLHistoryDLQTaskStore_UpdateHistoryDLQAckLevel(t *testing.T) {
	ctx := context.Background()
	ackTS := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 6, 1, 13, 0, 0, 0, time.UTC)

	baseRequest := persistence.InternalUpdateHistoryDLQAckLevelRequest{
		Row: persistence.InternalHistoryDLQAckLevel{
			ShardID:               5,
			DomainID:              "domain-abc",
			ClusterAttributeScope: "scope-1",
			ClusterAttributeName:  "cluster-west",
			TaskCategory:          3,
			AckLevelVisibilityTS:  ackTS,
			AckLevelTaskID:        88,
			LastUpdatedAt:         updatedAt,
		},
	}

	tests := map[string]struct {
		setupMock      func(*nosqlplugin.MockDB)
		expectError    bool
		errorValidator func(t *testing.T, err error)
	}{
		"when db upsert succeeds then no error is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().
					InsertOrUpdateHistoryDLQAckLevelRow(ctx, &nosqlplugin.HistoryDLQAckLevelRow{
						ShardID:               5,
						DomainID:              "domain-abc",
						ClusterAttributeScope: "scope-1",
						ClusterAttributeName:  "cluster-west",
						TaskType:              3,
						AckLevelVisibilityTS:  ackTS,
						AckLevelTaskID:        88,
						LastUpdatedAt:         updatedAt,
					}).
					Return(nil)
			},
		},
		"when db returns a throttling error then ServiceBusyError is returned": {
			setupMock: func(db *nosqlplugin.MockDB) {
				db.EXPECT().InsertOrUpdateHistoryDLQAckLevelRow(ctx, gomock.Any()).Return(errors.New("rate exceeded"))
				// These are mocked as the convertCommonErrors function calls an ErrorChecker interface, mocked by
				// nosqlplugin.MockDB without a real implementation.
				db.EXPECT().IsNotFoundError(gomock.Any()).Return(false)
				db.EXPECT().IsTimeoutError(gomock.Any()).Return(false)
				db.EXPECT().IsThrottlingError(gomock.Any()).Return(true)
			},
			expectError: true,
			errorValidator: func(t *testing.T, err error) {
				var busyErr *types.ServiceBusyError
				assert.ErrorAs(t, err, &busyErr)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			store, dbMock := setUpMocksForHistoryDLQTaskStore(t)
			tc.setupMock(dbMock)

			err := store.UpdateHistoryDLQAckLevel(ctx, baseRequest)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorValidator != nil {
					tc.errorValidator(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
