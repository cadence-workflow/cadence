// Copyright (c) 2020 Uber Technologies, Inc.
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

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/testdata"
)

func TestInsertShard(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-02T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		row         *nosqlplugin.ShardRow
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "successfully applied",
			row:  testdata.NewShardRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO executions (` +
					`shard_id, type, domain_id, workflow_id, run_id, ` +
					`visibility_ts, task_id, ` +
					`shard, data, data_encoding, ` +
					`range_id` +
					`) ` +
					`VALUES(` +
					`15, 0, 10000000-1000-f000-f000-000000000000, 20000000-1000-f000-f000-000000000000, 30000000-1000-f000-f000-000000000000, ` +
					`946684800000, -11, ` +
					`{shard_id: 15, owner: owner, range_id: 1000, stolen_since_renew: 0, updated_at: 1712080800000, replication_ack_level: 2000, transfer_ack_level: 3000, timer_ack_level: 2024-04-02T17:00:00Z, cluster_transfer_ack_level: map[cluster2:4000], cluster_timer_ack_level: map[cluster2:2024-04-02 16:00:00 +0000 UTC], transfer_processing_queue_states: [116 114 97 110 115 102 101 114 113 117 101 117 101], transfer_processing_queue_states_encoding: thriftrw, timer_processing_queue_states: [116 105 109 101 114 113 117 101 117 101], timer_processing_queue_states_encoding: thriftrw, domain_notification_version: 3, cluster_replication_level: map[cluster2:5000], replication_dlq_ack_level: map[cluster2:10], pending_failover_markers: [102 97 105 108 111 118 101 114 109 97 114 107 101 114 115], pending_failover_markers_encoding: thriftrw }, ` +
					`[115 104 97 114 100 100 97 116 97], thriftrw, ` +
					`1000` +
					`) IF NOT EXISTS`,
			},
		},
		{
			name: "not applied",
			row:  testdata.NewShardRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["range_id"] = int64(1001)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscancas failed",
			row:  testdata.NewShardRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, errors.New("mapscancas failed")
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

			err := db.InsertShard(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("InsertShard() error = %v, wantErr %v", err, tc.wantErr)
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

func TestSelectShard(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-02T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name          string
		shardID       int
		cluster       string
		queryMockFn   func(query *gocql.MockQuery)
		wantQueries   []string
		wantRangeID   int64
		wantShardInfo *nosqlplugin.ShardRow
		wantErr       bool
	}{
		{
			name:    "success with shard map fully populated",
			shardID: 15,
			cluster: "cluster1",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["range_id"] = int64(1000)
					m["shard"] = testdata.NewShardMap(ts)
					m["data"] = []byte("sharddata")
					m["data_encoding"] = "thriftrw"
					return nil
				}).Times(1)
			},
			wantRangeID: int64(1000),
			wantShardInfo: &nosqlplugin.ShardRow{
				InternalShardInfo: &persistence.InternalShardInfo{
					ShardID:                       15,
					Owner:                         "owner",
					RangeID:                       1000,
					UpdatedAt:                     ts,
					ReplicationAckLevel:           2000,
					ReplicationDLQAckLevel:        map[string]int64{"cluster2": 5},
					TransferAckLevel:              3000,
					TimerAckLevel:                 ts.Add(-1 * time.Hour),
					ClusterTransferAckLevel:       map[string]int64{"cluster1": 3000},
					ClusterTimerAckLevel:          map[string]time.Time{"cluster1": ts.Add(-1 * time.Hour)},
					TransferProcessingQueueStates: &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("transferqueue")},
					TimerProcessingQueueStates:    &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("timerqueue")},
					ClusterReplicationLevel:       map[string]int64{"cluster2": 1500},
					DomainNotificationVersion:     3,
					PendingFailoverMarkers:        &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("failovermarkers")},
				},
				Data:         []byte("sharddata"),
				DataEncoding: "thriftrw",
			},
			wantQueries: []string{
				`SELECT shard, data, data_encoding, range_id FROM executions WHERE ` +
					`shard_id = 15 and type = 0 and ` +
					`domain_id = 10000000-1000-f000-f000-000000000000 and ` +
					`workflow_id = 20000000-1000-f000-f000-000000000000 and ` +
					`run_id = 30000000-1000-f000-f000-000000000000 and ` +
					`visibility_ts = 946684800000 and ` +
					`task_id = -11`,
			},
		},
		{
			name:    "success with shard map missing some fields",
			shardID: 15,
			cluster: "cluster1",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["range_id"] = int64(1000)
					sm := testdata.NewShardMap(ts)
					// delete some fields and validate they are initialized properly
					delete(sm, "cluster_transfer_ack_level")
					delete(sm, "cluster_timer_ack_level")
					delete(sm, "cluster_replication_level")
					delete(sm, "replication_dlq_ack_level")
					m["shard"] = sm
					return nil
				}).Times(1)
			},
			wantRangeID: int64(1000),
			wantShardInfo: &nosqlplugin.ShardRow{
				InternalShardInfo: &persistence.InternalShardInfo{
					ShardID:                       15,
					Owner:                         "owner",
					RangeID:                       1000,
					UpdatedAt:                     ts,
					ReplicationAckLevel:           2000,
					ReplicationDLQAckLevel:        map[string]int64{}, // this was reset to empty map
					TransferAckLevel:              3000,
					TimerAckLevel:                 ts.Add(-1 * time.Hour),
					ClusterTransferAckLevel:       map[string]int64{"cluster1": 3000},
					ClusterTimerAckLevel:          map[string]time.Time{"cluster1": ts.Add(-1 * time.Hour)},
					TransferProcessingQueueStates: &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("transferqueue")},
					TimerProcessingQueueStates:    &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("timerqueue")},
					ClusterReplicationLevel:       map[string]int64{}, // this was reset to empty map
					DomainNotificationVersion:     3,
					PendingFailoverMarkers:        &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("failovermarkers")},
				},
			},
			wantQueries: []string{
				`SELECT shard, data, data_encoding, range_id FROM executions WHERE ` +
					`shard_id = 15 and type = 0 and ` +
					`domain_id = 10000000-1000-f000-f000-000000000000 and ` +
					`workflow_id = 20000000-1000-f000-f000-000000000000 and ` +
					`run_id = 30000000-1000-f000-f000-000000000000 and ` +
					`visibility_ts = 946684800000 and ` +
					`task_id = -11`,
			},
		},
		{
			name: "mapscan failed",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					return errors.New("mapscan failed")
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

			gotRangeID, gotShardInfo, err := db.SelectShard(context.Background(), tc.shardID, tc.cluster)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectShard() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if gotRangeID != tc.wantRangeID {
				t.Errorf("Got RangeID = %v, want %v", gotRangeID, tc.wantRangeID)
			}

			if diff := cmp.Diff(tc.wantShardInfo, gotShardInfo); diff != "" {
				t.Fatalf("ShardInfo mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateRangeID(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		rangeID     int64
		prevRangeID int64
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:        "successfully applied",
			shardID:     15,
			rangeID:     1000,
			prevRangeID: 999,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`UPDATE executions SET range_id = 1000 WHERE ` +
					`shard_id = 15 and ` +
					`type = 0 and ` +
					`domain_id = 10000000-1000-f000-f000-000000000000 and ` +
					`workflow_id = 20000000-1000-f000-f000-000000000000 and ` +
					`run_id = 30000000-1000-f000-f000-000000000000 and ` +
					`visibility_ts = 946684800000 and ` +
					`task_id = -11 ` +
					`IF range_id = 999`,
			},
		},
		{
			name:        "not applied",
			shardID:     15,
			rangeID:     1000,
			prevRangeID: 999,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["range_id"] = int64(1001)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name:        "mapscancas failed",
			shardID:     15,
			rangeID:     1000,
			prevRangeID: 999,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, errors.New("mapscancas failed")
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

			err := db.UpdateRangeID(context.Background(), tc.shardID, tc.rangeID, tc.prevRangeID)

			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateRangeID() error = %v, wantErr %v", err, tc.wantErr)
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

func TestUpdateShard(t *testing.T) {
	ts, err := time.Parse(time.RFC3339, "2024-04-02T18:00:00Z")
	if err != nil {
		t.Fatalf("Failed to parse time: %v", err)
	}

	tests := []struct {
		name        string
		row         *nosqlplugin.ShardRow
		prevRangeID int64
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:        "successfully applied",
			row:         testdata.NewShardRow(ts),
			prevRangeID: 988,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`UPDATE executions SET shard = {` +
					`shard_id: 15, ` +
					`owner: owner, ` +
					`range_id: 1000, ` +
					`stolen_since_renew: 0, ` +
					`updated_at: 1712080800000, ` +
					`replication_ack_level: 2000, ` +
					`transfer_ack_level: 3000, ` +
					`timer_ack_level: 2024-04-02T17:00:00Z, ` +
					`cluster_transfer_ack_level: map[cluster2:4000], ` +
					`cluster_timer_ack_level: map[cluster2:2024-04-02 16:00:00 +0000 UTC], ` +
					`transfer_processing_queue_states: [116 114 97 110 115 102 101 114 113 117 101 117 101], ` +
					`transfer_processing_queue_states_encoding: thriftrw, ` +
					`timer_processing_queue_states: [116 105 109 101 114 113 117 101 117 101], ` +
					`timer_processing_queue_states_encoding: thriftrw, ` +
					`domain_notification_version: 3, ` +
					`cluster_replication_level: map[cluster2:5000], ` +
					`replication_dlq_ack_level: map[cluster2:10], ` +
					`pending_failover_markers: [102 97 105 108 111 118 101 114 109 97 114 107 101 114 115], ` +
					`pending_failover_markers_encoding: thriftrw ` +
					`}, ` +
					`data = [115 104 97 114 100 100 97 116 97], ` +
					`data_encoding = thriftrw, ` +
					`range_id = 1000 ` +
					`WHERE ` +
					`shard_id = 15 and ` +
					`type = 0 and ` +
					`domain_id = 10000000-1000-f000-f000-000000000000 and ` +
					`workflow_id = 20000000-1000-f000-f000-000000000000 and ` +
					`run_id = 30000000-1000-f000-f000-000000000000 and ` +
					`visibility_ts = 946684800000 and ` +
					`task_id = -11 ` +
					`IF range_id = 988`,
			},
		},
		{
			name: "not applied",
			row:  testdata.NewShardRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					m["range_id"] = int64(1001)
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscancas failed",
			row:  testdata.NewShardRow(ts),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, errors.New("mapscancas failed")
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

			err := db.UpdateShard(context.Background(), tc.row, tc.prevRangeID)

			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateShard() error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			t.Log(session.queries[0])
			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
