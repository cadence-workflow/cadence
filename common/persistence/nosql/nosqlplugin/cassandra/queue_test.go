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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func TestInsertIntoQueue(t *testing.T) {
	tests := []struct {
		name        string
		row         *nosqlplugin.QueueMessageRow
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "successfully applied",
			row:  queueMessageRow(101),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return true, nil
				}).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO queue (queue_type, message_id, message_payload, created_time) VALUES(1, 101, [116 101 115 116 45 112 97 121 108 111 97 100 45 49 48 49], 2025-01-06T15:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name: "not applied",
			row:  queueMessageRow(101),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(m map[string]interface{}) (bool, error) {
					return false, nil
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name: "mapscancas failed",
			row:  queueMessageRow(101),
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

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.InsertIntoQueue(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestSelectLastEnqueuedMessageID(t *testing.T) {
	tests := []struct {
		name        string
		queueType   persistence.QueueType
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantMsgID   int64
		wantErr     bool
	}{
		{
			name:      "success with shard map fully populated",
			queueType: persistence.DomainReplicationQueueType,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["message_id"] = int64(101)
					return nil
				}).Times(1)
			},
			wantMsgID: int64(101),
			wantQueries: []string{
				`SELECT message_id FROM queue WHERE queue_type=1 ORDER BY message_id DESC LIMIT 1`,
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
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			gotMsgID, err := db.SelectLastEnqueuedMessageID(context.Background(), tc.queueType)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if gotMsgID != tc.wantMsgID {
				t.Errorf("Got message ID = %v, want %v", gotMsgID, tc.wantMsgID)
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectQueueMetadata(t *testing.T) {
	tests := []struct {
		name        string
		queueType   persistence.QueueType
		queryMockFn func(query *gocql.MockQuery)
		wantRow     *nosqlplugin.QueueMetadataRow
		wantQueries []string
		wantErr     bool
	}{
		{
			name:      "success",
			queueType: persistence.QueueType(2),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					ackLevels := args[0].(*map[string]int64)
					*ackLevels = make(map[string]int64)
					(*ackLevels)["cluster1"] = 1000
					(*ackLevels)["cluster2"] = 2000
					version := args[1].(*int64)
					*version = int64(25)
					return nil
				}).Times(1)
			},
			wantRow: &nosqlplugin.QueueMetadataRow{
				QueueType:        persistence.QueueType(2),
				ClusterAckLevels: map[string]int64{"cluster1": 1000, "cluster2": 2000},
				Version:          25,
			},
			wantQueries: []string{
				`SELECT cluster_ack_level, version FROM queue_metadata WHERE queue_type = 2`,
			},
		},
		{
			name:      "success with empty acklevels",
			queueType: persistence.QueueType(2),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Scan(gomock.Any()).DoAndReturn(func(args ...interface{}) error {
					version := args[1].(*int64)
					*version = int64(26)
					return nil
				}).Times(1)
			},
			wantRow: &nosqlplugin.QueueMetadataRow{
				QueueType:        persistence.QueueType(2),
				ClusterAckLevels: map[string]int64{},
				Version:          26,
			},
			wantQueries: []string{
				`SELECT cluster_ack_level, version FROM queue_metadata WHERE queue_type = 2`,
			},
		},
		{
			name:      "scan failure",
			queueType: persistence.QueueType(2),
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

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			gotRow, err := db.SelectQueueMetadata(context.Background(), tc.queueType)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestGetQueueSize(t *testing.T) {
	tests := []struct {
		name        string
		queueType   persistence.QueueType
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantCount   int64
		wantErr     bool
	}{
		{
			name:      "success with shard map fully populated",
			queueType: persistence.DomainReplicationQueueType,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["count"] = int64(12)
					return nil
				}).Times(1)
			},
			wantCount: int64(12),
			wantQueries: []string{
				`SELECT COUNT(1) AS count FROM queue WHERE queue_type=1`,
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
			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			gotCount, err := db.GetQueueSize(context.Background(), tc.queueType)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if gotCount != tc.wantCount {
				t.Errorf("Got message ID = %v, want %v", gotCount, tc.wantCount)
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectMessagesFrom(t *testing.T) {
	tests := []struct {
		name                    string
		queueType               persistence.QueueType
		exclusiveBeginMessageID int64
		maxRows                 int
		iter                    *fakeIter
		wantQueries             []string
		wantRows                []*nosqlplugin.QueueMessageRow
		wantErr                 bool
	}{
		{
			name:                    "nil iter",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 20,
			maxRows:                 10,
			iter:                    nil,
			wantErr:                 true,
		},
		{
			name:                    "iter close failed",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 20,
			maxRows:                 10,
			iter:                    &fakeIter{closeErr: errors.New("some random error")},
			wantErr:                 true,
		},
		{
			name:                    "success",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 20,
			maxRows:                 10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"message_id":      int64(21),
						"message_payload": []byte("test-payload-1"),
					},
					{
						"message_id":      int64(22),
						"message_payload": []byte("test-payload-2"),
					},
				},
			},
			wantRows: []*nosqlplugin.QueueMessageRow{
				{
					ID:      21,
					Payload: []byte("test-payload-1"),
				},
				{
					ID:      22,
					Payload: []byte("test-payload-2"),
				},
			},
			wantQueries: []string{
				`SELECT message_id, message_payload FROM queue WHERE queue_type = 1 and message_id > 20 LIMIT 10`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
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
			db := NewCassandraDBFromSession(cfg, session, logger, nil, DbWithClient(client))

			gotRows, err := db.SelectMessagesFrom(context.Background(), tc.queueType, tc.exclusiveBeginMessageID, tc.maxRows)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantRows, gotRows); diff != "" {
				t.Fatalf("rows mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Fatal("iterator not closed")
			}
		})
	}
}

func TestSelectMessagesBetween(t *testing.T) {
	tests := []struct {
		name        string
		request     nosqlplugin.SelectMessagesBetweenRequest
		iter        *fakeIter
		wantQueries []string
		wantResp    *nosqlplugin.SelectMessagesBetweenResponse
		wantErr     bool
	}{
		{
			name: "nil iter",
			request: nosqlplugin.SelectMessagesBetweenRequest{
				QueueType:               persistence.DomainReplicationQueueType,
				ExclusiveBeginMessageID: 50,
				InclusiveEndMessageID:   60,
				PageSize:                5,
				NextPageToken:           []byte("next page token"),
			},
			iter:    nil,
			wantErr: true,
		},
		{
			name: "iter close failed",
			request: nosqlplugin.SelectMessagesBetweenRequest{
				QueueType:               persistence.DomainReplicationQueueType,
				ExclusiveBeginMessageID: 50,
				InclusiveEndMessageID:   60,
				PageSize:                5,
				NextPageToken:           []byte("next page token"),
			},
			iter:    &fakeIter{closeErr: errors.New("some random error")},
			wantErr: true,
		},
		{
			name: "success",
			request: nosqlplugin.SelectMessagesBetweenRequest{
				QueueType:               persistence.DomainReplicationQueueType,
				ExclusiveBeginMessageID: 50,
				InclusiveEndMessageID:   60,
				PageSize:                5,
				NextPageToken:           []byte("next page token"),
			},
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"message_id":      int64(51),
						"message_payload": []byte("test-payload-1"),
					},
					{
						"message_id":      int64(52),
						"message_payload": []byte("test-payload-2"),
					},
					{
						"message_id":      int64(53),
						"message_payload": []byte("test-payload-3"),
					},
				},
				pageState: []byte("more pages"),
			},
			wantResp: &nosqlplugin.SelectMessagesBetweenResponse{
				Rows: []nosqlplugin.QueueMessageRow{
					{
						ID:      51,
						Payload: []byte("test-payload-1"),
					},
					{
						ID:      52,
						Payload: []byte("test-payload-2"),
					},
					{
						ID:      53,
						Payload: []byte("test-payload-3"),
					},
				},
				NextPageToken: []byte("more pages"),
			},
			wantQueries: []string{
				`SELECT message_id, message_payload FROM queue WHERE queue_type = 1 and message_id > 50 and message_id <= 60`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.request.PageSize).Return(query).Times(1)
			query.EXPECT().PageState(tc.request.NextPageToken).Return(query).Times(1)
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
			db := NewCassandraDBFromSession(cfg, session, logger, nil, DbWithClient(client))

			gotResp, err := db.SelectMessagesBetween(context.Background(), tc.request)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantQueries, session.queries); diff != "" {
				t.Fatalf("Query mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantResp, gotResp); diff != "" {
				t.Fatalf("Response mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Fatal("iterator not closed")
			}
		})
	}
}

func TestDeleteMessagesBefore(t *testing.T) {
	tests := []struct {
		name                    string
		queueType               persistence.QueueType
		exclusiveBeginMessageID int64
		queryMockFn             func(query *gocql.MockQuery)
		wantQueries             []string
		wantErr                 bool
	}{
		{
			name:                    "success",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 100,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantQueries: []string{
				`DELETE FROM queue WHERE queue_type = 1 and message_id < 100`,
			},
		},
		{
			name:                    "failure",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 100,
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
			db := NewCassandraDBFromSession(cfg, session, logger, nil, DbWithClient(client))

			err := db.DeleteMessagesBefore(context.Background(), tc.queueType, tc.exclusiveBeginMessageID)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestDeleteMessagesInRange(t *testing.T) {
	tests := []struct {
		name                    string
		queueType               persistence.QueueType
		exclusiveBeginMessageID int64
		inclusiveEndMsgID       int64
		queryMockFn             func(query *gocql.MockQuery)
		wantQueries             []string
		wantErr                 bool
	}{
		{
			name:                    "success",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 100,
			inclusiveEndMsgID:       200,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantQueries: []string{
				`DELETE FROM queue WHERE queue_type = 1 and message_id > 100 and message_id <= 200`,
			},
		},
		{
			name:                    "failure",
			queueType:               persistence.DomainReplicationQueueType,
			exclusiveBeginMessageID: 100,
			inclusiveEndMsgID:       200,
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
			db := NewCassandraDBFromSession(cfg, session, logger, nil, DbWithClient(client))

			err := db.DeleteMessagesInRange(context.Background(), tc.queueType, tc.exclusiveBeginMessageID, tc.inclusiveEndMsgID)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestDeleteMessage(t *testing.T) {
	tests := []struct {
		name        string
		queueType   persistence.QueueType
		msgID       int64
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:      "success",
			queueType: persistence.DomainReplicationQueueType,
			msgID:     36,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantQueries: []string{
				`DELETE FROM queue WHERE queue_type = 1 and message_id = 36`,
			},
		},
		{
			name:      "failure",
			queueType: persistence.DomainReplicationQueueType,
			msgID:     36,
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
			db := NewCassandraDBFromSession(cfg, session, logger, nil, DbWithClient(client))

			err := db.DeleteMessage(context.Background(), tc.queueType, tc.msgID)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestInsertQueueMetadata(t *testing.T) {
	tests := []struct {
		name        string
		queueType   persistence.QueueType
		version     int64
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name:      "success",
			queueType: persistence.QueueType(2),
			version:   25,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().ScanCAS(gomock.Any()).Return(false, nil).Times(1)
			},
			wantQueries: []string{
				`INSERT INTO queue_metadata (queue_type, cluster_ack_level, version, created_time) VALUES(2, map[], 25, 2025-01-06T15:00:00Z) IF NOT EXISTS`,
			},
		},
		{
			name:      "scan failure",
			queueType: persistence.QueueType(2),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().ScanCAS(gomock.Any()).Return(false, errors.New("some random error")).Times(1)
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

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			row := nosqlplugin.QueueMetadataRow{
				QueueType:        tc.queueType,
				Version:          tc.version,
				CurrentTimeStamp: FixedTime,
			}
			err := db.InsertQueueMetadata(context.Background(), row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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

func TestUpdateQueueMetadataCas(t *testing.T) {
	tests := []struct {
		name        string
		row         nosqlplugin.QueueMetadataRow
		queryMockFn func(query *gocql.MockQuery)
		wantQueries []string
		wantErr     bool
	}{
		{
			name: "successfully applied",
			row: nosqlplugin.QueueMetadataRow{
				QueueType:        persistence.QueueType(2),
				ClusterAckLevels: map[string]int64{"cluster1": 1000, "cluster2": 2000},
				Version:          25,
				CurrentTimeStamp: FixedTime,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().ScanCAS(gomock.Any()).Return(true, nil).Times(1)
			},
			wantQueries: []string{
				`UPDATE queue_metadata SET cluster_ack_level = map[cluster1:1000 cluster2:2000], version = 25, last_updated_time = 2025-01-06T15:00:00Z WHERE queue_type = 2 IF version = 24`,
			},
		},
		{
			name: "could not apply",
			row: nosqlplugin.QueueMetadataRow{
				QueueType:        persistence.QueueType(2),
				ClusterAckLevels: map[string]int64{"cluster1": 1000, "cluster2": 2000},
				Version:          25,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().ScanCAS(gomock.Any()).Return(false, nil).Times(1)
			},
			wantErr: true,
		},
		{
			name: "scancas failed",
			row: nosqlplugin.QueueMetadataRow{
				QueueType:        persistence.QueueType(2),
				ClusterAckLevels: map[string]int64{"cluster1": 1000, "cluster2": 2000},
				Version:          25,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().ScanCAS(gomock.Any()).Return(false, errors.New("some random error")).Times(1)
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

			db := NewCassandraDBFromSession(cfg, session, logger, dc, DbWithClient(client))

			err := db.UpdateQueueMetadataCas(context.Background(), tc.row)

			if (err != nil) != tc.wantErr {
				t.Errorf("Got error = %v, wantErr %v", err, tc.wantErr)
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
func queueMessageRow(id int64) *nosqlplugin.QueueMessageRow {
	return &nosqlplugin.QueueMessageRow{
		QueueType:        persistence.DomainReplicationQueueType,
		ID:               id,
		Payload:          []byte(fmt.Sprintf("test-payload-%d", id)),
		CurrentTimeStamp: FixedTime,
	}
}
