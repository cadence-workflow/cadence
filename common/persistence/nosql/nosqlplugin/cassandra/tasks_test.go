// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

func TestSelectTaskList(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		filter      *nosqlplugin.TaskListFilter
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.TaskListRow
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "success",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					rangeID := args[0].(*int64)
					*rangeID = 25
					tlDB := args[1].(*map[string]interface{})
					*tlDB = make(map[string]interface{})
					(*tlDB)["ack_level"] = int64(1000)
					(*tlDB)["kind"] = 2
					(*tlDB)["last_updated"] = now
					(*tlDB)["adaptive_partition_config"] = map[string]interface{}{
						"version":              int64(0),
						"num_read_partitions":  int(1),
						"num_write_partitions": int(1),
						"read_partitions": map[int]map[string]any{
							0: {
								"isolation_groups": []string{"foo"},
							},
						},
						"write_partitions": map[int]map[string]any{
							0: {
								"isolation_groups": []string{"bar"},
							},
						},
					}
					return nil
				}).Times(1)
			},
			wantRow: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: now,
				AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
					Version: 0,
					ReadPartitions: map[int]*persistence.TaskListPartition{
						0: {
							IsolationGroups: []string{"foo"},
						},
					},
					WritePartitions: map[int]*persistence.TaskListPartition{
						0: {
							IsolationGroups: []string{"bar"},
						},
					},
				},
			},
			wantQueries: []string{
				`SELECT range_id, task_list FROM tasks WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 1 and task_id = -12345`,
			},
		},
		{
			name: "success - partition numbers only",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					rangeID := args[0].(*int64)
					*rangeID = 25
					tlDB := args[1].(*map[string]interface{})
					*tlDB = make(map[string]interface{})
					(*tlDB)["ack_level"] = int64(1000)
					(*tlDB)["kind"] = 2
					(*tlDB)["last_updated"] = now
					(*tlDB)["adaptive_partition_config"] = map[string]interface{}{
						"version":              int64(0),
						"num_read_partitions":  int(1),
						"num_write_partitions": int(1),
					}
					return nil
				}).Times(1)
			},
			wantRow: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: now,
				AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
					Version: 0,
					ReadPartitions: map[int]*persistence.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*persistence.TaskListPartition{
						0: {},
					},
				},
			},
			wantQueries: []string{
				`SELECT range_id, task_list FROM tasks WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 1 and task_id = -12345`,
			},
		},
		{
			name: "scan failure",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotRow, err := db.SelectTaskList(context.Background(), tc.filter)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectTaskList() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantRow, gotRow); diff != "" {
				t.Fatalf("Row mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsertTaskList(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-01T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		row         *nosqlplugin.TaskListRow
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "successfully applied - nil partition_config",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO tasks (domain_id, task_list_name, task_list_type, type, task_id, range_id, task_list, created_time ) ` +
					`VALUES (domain1, tasklist1, 1, 1, -12345, 1, ` +
					`{domain_id: domain1, name: tasklist1, type: 1, ack_level: 0, kind: 2, last_updated: 2024-04-01T22:08:41Z, adaptive_partition_config: map[] }` +
					`, 2025-01-06T15:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name: "successfully applied - non-nil partition_config",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
				AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*persistence.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*persistence.TaskListPartition{
						0: {},
					},
				},
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO tasks (domain_id, task_list_name, task_list_type, type, task_id, range_id, task_list, created_time ) ` +
					`VALUES (domain1, tasklist1, 1, 1, -12345, 1, ` +
					`{domain_id: domain1, name: tasklist1, type: 1, ack_level: 0, kind: 2, last_updated: 2024-04-01T22:08:41Z, ` +
					`adaptive_partition_config: map[num_read_partitions:1 num_write_partitions:1 read_partitions:map[0:map[isolation_groups:[]]] version:1 write_partitions:map[0:map[isolation_groups:[]]]] }` +
					`, 2025-01-06T15:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name: "not applied",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					prev["range_id"] = int64(26)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscan failed",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return false, errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))
			db.timeSrc = clock.NewMockedTimeSourceAt(FixedTime)

			err := db.InsertTaskList(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("InsertTaskList() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateTaskList(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-01T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		row         *nosqlplugin.TaskListRow
		prevRangeID int64
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:        "successfully applied",
			prevRangeID: 25,
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`UPDATE tasks SET range_id = 25, task_list = {domain_id: domain1, name: tasklist1, type: 1, ack_level: 1000, kind: 2, last_updated: 2024-04-01T22:08:41Z, adaptive_partition_config: map[] } , last_updated_time = 2025-01-06T15:00:00Z WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 1 and task_id = -12345 IF range_id = 25`,
			},
		},
		{
			name: "not applied",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					prev["range_id"] = int64(26)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscan failed",
			row: &nosqlplugin.TaskListRow{
				DomainID:        "domain1",
				TaskListName:    "tasklist1",
				TaskListType:    1,
				TaskListKind:    2,
				AckLevel:        1000,
				RangeID:         25,
				LastUpdatedTime: ts,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return false, errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))
			db.timeSrc = clock.NewMockedTimeSourceAt(FixedTime)

			err := db.UpdateTaskList(context.Background(), tc.row, tc.prevRangeID)

			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateTaskList() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateTaskListWithTTL(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-01T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name                      string
		row                       *nosqlplugin.TaskListRow
		ttlSeconds                int64
		prevRangeID               int64
		mapExecuteBatchCASApplied bool
		mapExecuteBatchCASErr     error
		mapExecuteBatchCASPrev    map[string]any
		wantQueries               []string
		wantErr                   bool
	}{
		{
			name:        "successfully applied",
			ttlSeconds:  180,
			prevRangeID: 25,
			row: &nosqlplugin.TaskListRow{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
				TaskListKind: 2,
				AckLevel:     1000,
				RangeID:      25,
			},
			mapExecuteBatchCASApplied: true,
			wantQueries: []string{
				` INSERT INTO tasks (domain_id, task_list_name, task_list_type, type, task_id, created_time ) VALUES (domain1, tasklist1, 1, 1, -12345, 2024-04-01T22:08:41Z) USING TTL 180`,
				`UPDATE tasks USING TTL 180 SET range_id = 25, task_list = {domain_id: domain1, name: tasklist1, type: 1, ack_level: 1000, kind: 2, last_updated: 2024-04-01T22:08:41Z, adaptive_partition_config: map[] } , last_updated_time = 2024-04-01T22:08:41Z WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 1 and task_id = -12345 IF range_id = 25`,
			},
		},
		{
			name: "not applied",
			row: &nosqlplugin.TaskListRow{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
				TaskListKind: 2,
				AckLevel:     1000,
				RangeID:      25,
			},
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"range_id": int64(26),
			},
			wantErr: true,
		},
		{
			name: "mapscan failed",
			row: &nosqlplugin.TaskListRow{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
				TaskListKind: 2,
				AckLevel:     1000,
				RangeID:      25,
			},
			mapExecuteBatchCASErr: errors.New("cas failure"),
			wantErr:               true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{
				mapExecuteBatchCASApplied: tc.mapExecuteBatchCASApplied,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
				mapExecuteBatchCASPrev:    tc.mapExecuteBatchCASPrev,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			timeSrc := clock.NewMockedTimeSourceAt(ts)
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client), dbWithTimeSource(timeSrc))
			err := db.UpdateTaskListWithTTL(context.Background(), tc.ttlSeconds, tc.row, tc.prevRangeID)

			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateTaskListWithTTL() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.batches[0].queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListTaskList(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := gocql.NewMockClient(ctrl)
	cfg := &config.NoSQL{}
	logger := testlogger.New(t)
	dc := &persistence.DynamicConfiguration{}
	session := &fakeSession{}
	db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

	_, err := db.ListTaskList(context.Background(), 10, nil)
	var want *types.InternalServiceError
	if !errors.As(err, &want) {
		t.Fatalf("expected InternalServiceError, got %v", err)
	}
	if want.Message != "unsupported operation" {
		t.Fatalf("Unexpected message: %v", want.Message)
	}
}

func TestDeleteTaskList(t *testing.T) {
	tests := []struct {
		name         string
		filter       *nosqlplugin.TaskListFilter
		prevRangeID  int64
		queryMockFn  func(query *gocql.MockQuery)
		clientMockFn func(client *gocql.MockClient)
		wantQueries  []string
		wantErr      bool
	}{
		{
			name:        "successfully applied",
			prevRangeID: 35,
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`DELETE FROM tasks WHERE domain_id = domain1 AND task_list_name = tasklist1 AND task_list_type = 1 AND type = 1 AND task_id = -12345 IF range_id = 35`,
			},
		},
		{
			name: "not applied",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					prev["range_id"] = int64(26)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscan failed with all consistency, retry with default consistency",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				firstConsCall := query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				firstMapScanCall := query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return false, errors.New("some random error")
				}).Times(1)

				// expect retry with default consistency level
				query.EXPECT().Consistency(cassandraDefaultConsLevel).Return(query).Times(1).After(firstConsCall)
				query.EXPECT().MapScanCAS(gomock.Any()).Return(true, nil).Times(1).After(firstMapScanCall)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true).Times(1)
			},
			wantQueries: []string{
				`DELETE FROM tasks WHERE domain_id = domain1 AND task_list_name = tasklist1 AND task_list_type = 1 AND type = 1 AND task_id = -12345 IF range_id = 0`,
			},
		},
		{
			name: "mapscan failed with all consistency, retry with default consistency also failed",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				firstConsCall := query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				firstMapScanCall := query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return false, errors.New("failed to delete")
				}).Times(1)

				// expect retry with default consistency level
				query.EXPECT().Consistency(cassandraDefaultConsLevel).Return(query).Times(1).After(firstConsCall)
				query.EXPECT().MapScanCAS(gomock.Any()).Return(true, errors.New("failed to deleta again")).Times(1).After(firstMapScanCall)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(true).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscan failed with non-consistency error",
			filter: &nosqlplugin.TaskListFilter{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Consistency(cassandraAllConslevel).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(prev map[string]interface{}) (bool, error) {
					return false, errors.New("failed to delete")
				}).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsCassandraConsistencyError(gomock.Any()).Return(false).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			if tc.clientMockFn != nil {
				tc.clientMockFn(client)
			}
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.DeleteTaskList(context.Background(), tc.filter, tc.prevRangeID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteTaskList() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsertTasks(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-01T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name                      string
		tasksToInsert             []*nosqlplugin.TaskRowForInsert
		tasklistCond              *nosqlplugin.TaskListRow
		mapExecuteBatchCASApplied bool
		mapExecuteBatchCASErr     error
		mapExecuteBatchCASPrev    map[string]any
		wantQueries               []string
		wantErr                   bool
	}{
		{
			name: "successfully applied",
			tasksToInsert: []*nosqlplugin.TaskRowForInsert{
				{
					TTLSeconds: 0, // default create task query will be used for this ttl
					TaskRow: nosqlplugin.TaskRow{
						TaskID:      3,
						WorkflowID:  "wid1",
						RunID:       "rid1",
						ScheduledID: 42,
						CreatedTime: ts,
					},
				},
				{
					TTLSeconds: int(maxCassandraTTL) + 1, // ttl query will be used for this ttl
					TaskRow: nosqlplugin.TaskRow{
						TaskID:      4,
						WorkflowID:  "wid1",
						RunID:       "rid1",
						ScheduledID: 43,
						CreatedTime: ts.Add(time.Second),
					},
				},
			},
			tasklistCond: &nosqlplugin.TaskListRow{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
				RangeID:      25,
			},
			mapExecuteBatchCASApplied: true,
			wantQueries: []string{
				`INSERT INTO tasks (domain_id, task_list_name, task_list_type, type, task_id, task, created_time) VALUES(domain1, tasklist1, 1, 0, 3, {domain_id: domain1, workflow_id: wid1, run_id: rid1, schedule_id: 42,created_time: 2024-04-01T22:08:41Z, partition_config: map[] }, 2024-04-01T22:08:41Z)`,
				`INSERT INTO tasks (domain_id, task_list_name, task_list_type, type, task_id, task, created_time) VALUES(domain1, tasklist1, 1, 0, 4, {domain_id: domain1, workflow_id: wid1, run_id: rid1, schedule_id: 43,created_time: 2024-04-01T22:08:42Z, partition_config: map[] }, 2024-04-01T22:08:41Z) USING TTL 157680000`,
				`UPDATE tasks SET range_id = 25, last_updated_time = 2024-04-01T22:08:41Z WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 1 and task_id = -12345 IF range_id = 25`,
			},
		},
		{
			name: "batch cas failed",
			tasksToInsert: []*nosqlplugin.TaskRowForInsert{
				{
					TTLSeconds: 0,
					TaskRow: nosqlplugin.TaskRow{
						TaskID:      3,
						WorkflowID:  "wid1",
						RunID:       "rid1",
						ScheduledID: 42,
					},
				},
			},
			tasklistCond: &nosqlplugin.TaskListRow{
				DomainID:     "domain1",
				TaskListName: "tasklist1",
				TaskListType: 1,
				RangeID:      25,
			},
			mapExecuteBatchCASErr: errors.New("some random error"),
			wantErr:               true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			session := &fakeSession{
				mapExecuteBatchCASApplied: tc.mapExecuteBatchCASApplied,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
				mapExecuteBatchCASPrev:    tc.mapExecuteBatchCASPrev,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			timeSrc := clock.NewMockedTimeSourceAt(ts)
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client), dbWithTimeSource(timeSrc))

			err := db.InsertTasks(context.Background(), tc.tasksToInsert, tc.tasklistCond)

			if (err != nil) != tc.wantErr {
				t.Errorf("InsertTasks() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.batches[0].queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetTasksCount(t *testing.T) {
	tests := []struct {
		name          string
		filter        *nosqlplugin.TasksFilter
		queryMockFn   func(query *gocql.MockQuery)
		wantQueries   []string
		wantQueueSize int64
		wantErr       bool
	}{
		{
			name: "success",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(result map[string]interface{}) error {
					result["count"] = int64(50)
					return nil
				}).Times(1)
			},
			wantQueueSize: 50,
			wantQueries: []string{
				`SELECT count(1) as count FROM tasks WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 0 and task_id > 1 `,
			},
		},
		{
			name: "scan failure",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(result map[string]interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			queueSize, err := db.GetTasksCount(context.Background(), tc.filter)

			if (err != nil) != tc.wantErr {
				t.Errorf("GetTasksCount() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if queueSize != tc.wantQueueSize {
				t.Fatalf("Got queue size: %d, want: %v", queueSize, tc.wantQueueSize)
			}
		})
	}
}

func TestSelectTasks(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-01T22:08:41Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		filter      *nosqlplugin.TasksFilter
		iter        *fakeIter
		wantQueries []string
		wantTasks   []*nosqlplugin.TaskRow
		wantErr     bool
	}{
		{
			name: "nil iter",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
			},
			wantErr: true,
		},
		{
			name: "iter close failed",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
				BatchSize: 3,
			},
			iter:    &fakeIter{closeErr: errors.New("some random error")},
			wantErr: true,
		},
		{
			name: "success",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 0,
				MaxTaskID: 100,
				BatchSize: 3,
			},
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"task_id": int64(1),
						"task": map[string]interface{}{
							"domain_id":        &fakeUUID{uuid: "domain1"},
							"wofklow_id":       "wid1",
							"schedule_id":      int64(42),
							"created_time":     ts,
							"run_id":           &fakeUUID{uuid: "runid1"},
							"partition_config": map[string]string{},
						},
					},
					{
						"task_id": int64(2),
						"task": map[string]interface{}{
							"domain_id":        &fakeUUID{uuid: "domain1"},
							"worklow_id":       "wid1",
							"schedule_id":      int64(45),
							"created_time":     ts,
							"run_id":           &fakeUUID{uuid: "runid1"},
							"partition_config": map[string]string{},
						},
					},
					{
						"missing_task_id": int64(1), // missing task_id so this row will be skipped
					},
					{
						"task_id": int64(3),
						"task": map[string]interface{}{
							"domain_id":        &fakeUUID{uuid: "domain1"},
							"worklow_id":       "wid1",
							"schedule_id":      int64(48),
							"created_time":     ts,
							"run_id":           &fakeUUID{uuid: "runid1"},
							"partition_config": map[string]string{},
						},
					},
					{
						"task_id": int64(4), // this will be skipped because filter.BatchSize is reached
						"task": map[string]interface{}{
							"domain_id":        &fakeUUID{uuid: "domain1"},
							"worklow_id":       "wid1",
							"schedule_id":      int64(51),
							"created_time":     ts,
							"run_id":           &fakeUUID{uuid: "runid1"},
							"partition_config": map[string]string{},
						},
					},
				},
			},
			wantTasks: []*nosqlplugin.TaskRow{
				{
					DomainID:        "domain1",
					TaskID:          1,
					RunID:           "runid1",
					ScheduledID:     42,
					CreatedTime:     ts,
					PartitionConfig: map[string]string{},
				},
				{
					DomainID:        "domain1",
					TaskID:          2,
					RunID:           "runid1",
					ScheduledID:     45,
					CreatedTime:     ts,
					PartitionConfig: map[string]string{},
				},
				{
					DomainID:        "domain1",
					TaskID:          3,
					RunID:           "runid1",
					ScheduledID:     48,
					CreatedTime:     ts,
					PartitionConfig: map[string]string{},
				},
			},
			wantQueries: []string{
				`SELECT task_id, task, TTL(task) AS ttl FROM tasks WHERE domain_id = domain1 and task_list_name = tasklist1 and task_list_type = 1 and type = 0 and task_id > 0 and task_id <= 100`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(gomock.Any()).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(cfg, session, logger, nil, dbWithClient(client))

			gotRows, err := db.SelectTasks(context.Background(), tc.filter)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectTasks() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantTasks, gotRows); diff != "" {
				t.Fatalf("Task rows mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Fatal("iterator not closed")
			}
		})
	}
}

func TestRangeDeleteTasks(t *testing.T) {
	tests := []struct {
		name            string
		filter          *nosqlplugin.TasksFilter
		queryMockFn     func(query *gocql.MockQuery)
		wantQueries     []string
		wantRowsDeleted int
		wantErr         bool
	}{
		{
			name: "success",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantRowsDeleted: persistence.UnknownNumRowsAffected,
			wantQueries: []string{
				`DELETE FROM tasks WHERE domain_id = domain1 AND task_list_name = tasklist1 AND task_list_type = 1 AND type = 0 AND task_id > 1 AND task_id <= 100 `,
			},
		},
		{
			name: "failure",
			filter: &nosqlplugin.TasksFilter{
				TaskListFilter: nosqlplugin.TaskListFilter{
					DomainID:     "domain1",
					TaskListName: "tasklist1",
					TaskListType: 1,
				},
				MinTaskID: 1,
				MaxTaskID: 100,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("some random error")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(cfg, session, logger, nil, dbWithClient(client))

			rowsDeleted, err := db.RangeDeleteTasks(context.Background(), tc.filter)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteTasks() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if rowsDeleted != tc.wantRowsDeleted {
				t.Fatalf("Got rows deleted: %d, want: %v", rowsDeleted, tc.wantRowsDeleted)
			}
		})
	}
}
