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

package cassandra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

// testSemaphoreDomainID and testSemaphoreName are shared with semaphore_tokens_test.go.
const testSemaphoreBucket = 3

func newTestCDB(t *testing.T, session gocql.Session) *CDB {
	ctrl := gomock.NewController(t)
	client := gocql.NewMockClient(ctrl)
	cfg := &config.NoSQL{}
	logger := testlogger.New(t)
	dc := persistence.NewDefaultDynamicConfiguration()
	return NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))
}

func TestSelectSemaphoreTaskControlRow(t *testing.T) {
	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.SemaphoreTaskControlRow
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any()).
					DoAndReturn(func(args ...interface{}) error {
						*args[0].(*int64) = 7  // range_id
						*args[1].(*int64) = 42 // ack_level
						return nil
					}).Times(1)
			},
			wantRow: &nosqlplugin.SemaphoreTaskControlRow{
				DomainID:      testSemaphoreDomainID,
				SemaphoreName: testSemaphoreName,
				Bucket:        testSemaphoreBucket,
				RangeID:       7,
				AckLevel:      42,
			},
			wantQueries: []string{
				`SELECT range_id, ack_level ` +
					`FROM semaphore_tasks ` +
					`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 ` +
					`AND bucket = 3 AND type = 1 AND task_id = -12345`,
			},
		},
		{
			name: "not found",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any(), gomock.Any()).
					Return(errors.New("not found")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			row, err := db.SelectSemaphoreTaskControlRow(context.Background(), &nosqlplugin.SemaphoreTaskControlFilter{
				DomainID:      testSemaphoreDomainID,
				SemaphoreName: testSemaphoreName,
				Bucket:        testSemaphoreBucket,
			})

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRow, row)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestInsertSemaphoreTaskControlRow(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	row := &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:      testSemaphoreDomainID,
		SemaphoreName: testSemaphoreName,
		Bucket:        testSemaphoreBucket,
		RangeID:       1,
		AckLevel:      0,
		CreatedTime:   now,
	}

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "applied",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).Return(true, nil).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO semaphore_tasks (` +
					`domain_id, semaphore_name, bucket, type, task_id, range_id, ack_level, created_time` +
					`) VALUES (10000000-1000-f000-f000-000000000000, sem-1, 3, 1, -12345, 1, 0, 2025-06-01T12:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name: "not applied - already exists",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["range_id"] = int64(5)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscancas failed",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).Return(false, errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			err := db.InsertSemaphoreTaskControlRow(context.Background(), row)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestUpdateSemaphoreTaskControlRow(t *testing.T) {
	row := &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:      testSemaphoreDomainID,
		SemaphoreName: testSemaphoreName,
		Bucket:        testSemaphoreBucket,
		RangeID:       8,
		AckLevel:      50,
	}

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
		wantFence   bool
		wantRangeID int64
		wantDetails string
	}{
		{
			name: "applied",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).Return(true, nil).Times(1)
			},
			wantQueries: []string{
				`UPDATE semaphore_tasks SET range_id = 8, ack_level = 50 ` +
					`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 ` +
					`AND bucket = 3 AND type = 1 AND task_id = -12345 ` +
					`IF range_id = 7`,
			},
		},
		{
			name: "fence conflict returns TaskOperationConditionFailure",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["range_id"] = int64(9)
					return false, nil
				}).Times(1)
			},
			wantErr:     true,
			wantFence:   true,
			wantRangeID: 9,
		},
		{
			// Cassandra returns only the conflicted columns, so range_id is not guaranteed to be
			// among them. The tasks table's shared helper asserts on it and would panic here;
			// ours reports rangeID 0 and still surfaces a usable fence error. Two columns also
			// pin the Details ordering, which is sorted because map iteration order is random.
			name: "fence conflict without range_id reports 0 instead of panicking",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["task_id"] = int64(-12345)
					m["ack_level"] = int64(50)
					return false, nil
				}).Times(1)
			},
			wantErr:     true,
			wantFence:   true,
			wantRangeID: 0,
			wantDetails: "ack_level=50,task_id=-12345",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			err := db.UpdateSemaphoreTaskControlRow(context.Background(), row, 7)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantFence {
					var fenceErr *nosqlplugin.TaskOperationConditionFailure
					require.True(t, errors.As(err, &fenceErr), "expected *TaskOperationConditionFailure, got %T", err)
					assert.Equal(t, tc.wantRangeID, fenceErr.RangeID)
					if tc.wantDetails != "" {
						assert.Equal(t, tc.wantDetails, fenceErr.Details)
					}
				}
				return
			}
			assert.NoError(t, err)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestInsertSemaphoreTasks(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Hour)
	tasks := []*nosqlplugin.SemaphoreTaskRow{
		{
			DomainID:        testSemaphoreDomainID,
			SemaphoreName:   testSemaphoreName,
			Bucket:          testSemaphoreBucket,
			TaskID:          100,
			WorkflowID:      "wf-1",
			RunID:           "20000000-2000-f000-f000-000000000000",
			HoldID:          11,
			AcquireDeadline: &deadline,
			CreatedTime:     now,
		},
		{
			DomainID:      testSemaphoreDomainID,
			SemaphoreName: testSemaphoreName,
			Bucket:        testSemaphoreBucket,
			TaskID:        101,
			WorkflowID:    "wf-2",
			RunID:         "30000000-3000-f000-f000-000000000000",
			HoldID:        12,
			CreatedTime:   now,
		},
	}
	control := &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:      testSemaphoreDomainID,
		SemaphoreName: testSemaphoreName,
		Bucket:        testSemaphoreBucket,
		RangeID:       5,
	}

	tests := []struct {
		name        string
		tasks       []*nosqlplugin.SemaphoreTaskRow
		session     *fakeSession
		wantErr     bool
		wantFence   bool
		wantNoBatch bool
		wantBatchN  int
		wantLastCQL string
	}{
		{
			name:       "applied - task creates + fence re-assert in one batch",
			tasks:      tasks,
			session:    &fakeSession{mapExecuteBatchCASApplied: true},
			wantBatchN: 3, // two task inserts + one fence re-assert
			wantLastCQL: `UPDATE semaphore_tasks SET range_id = 5 ` +
				`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 ` +
				`AND bucket = 3 AND type = 1 AND task_id = -12345 ` +
				`IF range_id = 5`,
		},
		{
			name:  "fence conflict",
			tasks: tasks,
			session: &fakeSession{
				mapExecuteBatchCASApplied: false,
				mapExecuteBatchCASPrev:    map[string]any{"range_id": int64(6)},
			},
			wantErr:   true,
			wantFence: true,
		},
		{
			name:    "batch error",
			tasks:   tasks,
			session: &fakeSession{mapExecuteBatchCASErr: errors.New("db error")},
			wantErr: true,
		},
		{
			// No rows to write, so no batch at all: a bare fence re-assert would spend a
			// Paxos round asserting ownership no caller asked about.
			name:        "empty slice - no batch issued",
			tasks:       []*nosqlplugin.SemaphoreTaskRow{},
			session:     &fakeSession{mapExecuteBatchCASApplied: true},
			wantNoBatch: true,
		},
		{
			name:        "nil slice - no batch issued",
			tasks:       nil,
			session:     &fakeSession{mapExecuteBatchCASApplied: true},
			wantNoBatch: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestCDB(t, tc.session)

			err := db.InsertSemaphoreTasks(context.Background(), tc.tasks, control)

			if tc.wantErr {
				require.Error(t, err)
				if tc.wantFence {
					var fenceErr *nosqlplugin.TaskOperationConditionFailure
					require.True(t, errors.As(err, &fenceErr), "expected *TaskOperationConditionFailure, got %T", err)
					assert.Equal(t, int64(6), fenceErr.RangeID)
				}
				return
			}
			assert.NoError(t, err)
			if tc.wantNoBatch {
				assert.Empty(t, tc.session.batches, "no tasks to write, so no batch should be issued")
				return
			}
			require.Len(t, tc.session.batches, 1)
			batch := tc.session.batches[0]
			assert.Len(t, batch.queries, tc.wantBatchN)
			if tc.wantLastCQL != "" {
				assert.Equal(t, tc.wantLastCQL, batch.queries[len(batch.queries)-1])
			}
			// first task insert renders the UDT literal with its fields
			assert.Contains(t, batch.queries[0], "wf-1")
			assert.Contains(t, batch.queries[0], "hold_id: 11")
		})
	}
}

func TestSelectSemaphoreTasks(t *testing.T) {
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(time.Hour)
	filter := &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      testSemaphoreDomainID,
			SemaphoreName: testSemaphoreName,
			Bucket:        testSemaphoreBucket,
		},
		ExclusiveMinTaskID: 0,
		InclusiveMaxTaskID: 1000,
		BatchSize:          10,
	}

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		iterMockFn  func(iter *gocql.MockIter)
		nilIter     bool
		wantRows    []*nosqlplugin.SemaphoreTaskRow
		wantErr     bool
	}{
		{
			name: "two tasks, deadline and no-deadline",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(100)
					m["task"] = map[string]interface{}{
						"workflow_id": "wf-1",
						"run_id":      &fakeUUID{uuid: "20000000-2000-f000-f000-000000000000"},
						"hold_id":     int64(11),
					}
					m["acquire_deadline"] = deadline
					m["created_time"] = now
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(101)
					m["task"] = map[string]interface{}{
						"workflow_id": "wf-2",
						"run_id":      &fakeUUID{uuid: "30000000-3000-f000-f000-000000000000"},
						"hold_id":     int64(12),
					}
					m["created_time"] = now
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).Return(false).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreTaskRow{
				{
					DomainID:        testSemaphoreDomainID,
					SemaphoreName:   testSemaphoreName,
					Bucket:          testSemaphoreBucket,
					TaskID:          100,
					WorkflowID:      "wf-1",
					RunID:           "20000000-2000-f000-f000-000000000000",
					HoldID:          11,
					AcquireDeadline: &deadline,
					CreatedTime:     now,
				},
				{
					DomainID:      testSemaphoreDomainID,
					SemaphoreName: testSemaphoreName,
					Bucket:        testSemaphoreBucket,
					TaskID:        101,
					WorkflowID:    "wf-2",
					RunID:         "30000000-3000-f000-f000-000000000000",
					HoldID:        12,
					CreatedTime:   now,
				},
			},
		},
		{
			name: "row without task_id skipped",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					// no task_id key -> skipped
					m["range_id"] = int64(5)
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).Return(false).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: nil,
		},
		{
			// A null `task` column must not panic the read. The good row after it still
			// comes back, so one bad row cannot take out the whole batch.
			name: "row with null task UDT skipped, later rows still returned",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(100)
					m["task"] = nil // null UDT: the unguarded assertion used to panic here
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(101)
					m["task"] = map[string]interface{}{
						"workflow_id": "wf-2",
						"run_id":      &fakeUUID{uuid: "30000000-3000-f000-f000-000000000000"},
						"hold_id":     int64(12),
					}
					m["created_time"] = now
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).Return(false).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreTaskRow{
				{
					DomainID:      testSemaphoreDomainID,
					SemaphoreName: testSemaphoreName,
					Bucket:        testSemaphoreBucket,
					TaskID:        101,
					WorkflowID:    "wf-2",
					RunID:         "30000000-3000-f000-f000-000000000000",
					HoldID:        12,
					CreatedTime:   now,
				},
			},
		},
		{
			// A UDT present but missing a field is as unusable as no UDT at all, and must be
			// skipped the same way rather than returned with the field zeroed.
			name: "row with an incomplete task UDT skipped, later rows still returned",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(100)
					m["task"] = map[string]interface{}{
						"workflow_id": "wf-1",
						"run_id":      nil, // null field: the unguarded assertion used to panic here
						"hold_id":     int64(11),
					}
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) bool {
					m["task_id"] = int64(101)
					m["task"] = map[string]interface{}{
						"workflow_id": "wf-2",
						"run_id":      &fakeUUID{uuid: "30000000-3000-f000-f000-000000000000"},
						"hold_id":     int64(12),
					}
					m["created_time"] = now
					return true
				}).Times(1)
				iter.EXPECT().MapScan(gomock.Any()).Return(false).Times(1)
				iter.EXPECT().Close().Return(nil).Times(1)
			},
			wantRows: []*nosqlplugin.SemaphoreTaskRow{
				{
					DomainID:      testSemaphoreDomainID,
					SemaphoreName: testSemaphoreName,
					Bucket:        testSemaphoreBucket,
					TaskID:        101,
					WorkflowID:    "wf-2",
					RunID:         "30000000-3000-f000-f000-000000000000",
					HoldID:        12,
					CreatedTime:   now,
				},
			},
		},
		{
			name:    "nil iterator",
			nilIter: true,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {},
			wantErr:    true,
		},
		{
			name: "iterator close fails",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(10).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			},
			iterMockFn: func(iter *gocql.MockIter) {
				iter.EXPECT().MapScan(gomock.Any()).Return(false).Times(1)
				iter.EXPECT().Close().Return(errors.New("close failed")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			iter := gocql.NewMockIter(ctrl)

			tc.queryMockFn(query)
			if !tc.nilIter {
				query.EXPECT().Iter().Return(iter).Times(1)
			}
			tc.iterMockFn(iter)

			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			rows, err := db.SelectSemaphoreTasks(context.Background(), filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantRows, rows)
		})
	}
}

func TestGetSemaphoreTasksCount(t *testing.T) {
	filter := &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      testSemaphoreDomainID,
			SemaphoreName: testSemaphoreName,
			Bucket:        testSemaphoreBucket,
		},
		ExclusiveMinTaskID: 42,
	}

	tests := []struct {
		name        string
		queryMockFn func(query *gocql.MockQuery)
		wantCount   int64
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "count returned",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["count"] = int64(3)
					return nil
				}).Times(1)
			},
			wantCount: 3,
			wantQueries: []string{
				`SELECT count(1) as count ` +
					`FROM semaphore_tasks ` +
					`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 ` +
					`AND bucket = 3 AND type = 0 AND task_id > 42`,
			},
		},
		{
			name: "scan error",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).Return(errors.New("db error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{query: query}
			db := newTestCDB(t, session)

			count, err := db.GetSemaphoreTasksCount(context.Background(), filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.wantCount, count)
			if tc.wantQueries != nil {
				assert.Equal(t, tc.wantQueries, session.queries)
			}
		})
	}
}

func TestRangeDeleteSemaphoreTasks(t *testing.T) {
	filter := &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      testSemaphoreDomainID,
			SemaphoreName: testSemaphoreName,
			Bucket:        testSemaphoreBucket,
		},
		ExclusiveMinTaskID: 0,
		InclusiveMaxTaskID: 100,
	}

	ctrl := gomock.NewController(t)
	query := gocql.NewMockQuery(ctrl)
	query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
	query.EXPECT().Exec().Return(nil).Times(1)
	session := &fakeSession{query: query}
	db := newTestCDB(t, session)

	rowsDeleted, err := db.RangeDeleteSemaphoreTasks(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, persistence.UnknownNumRowsAffected, rowsDeleted)
	assert.Equal(t, []string{
		`DELETE FROM semaphore_tasks ` +
			`WHERE domain_id = 10000000-1000-f000-f000-000000000000 AND semaphore_name = sem-1 ` +
			`AND bucket = 3 AND type = 0 AND task_id > 0 AND task_id <= 100`,
	}, session.queries)
}
