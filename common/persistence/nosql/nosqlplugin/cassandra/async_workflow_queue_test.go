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

package cassandra

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func newAsyncQueueTestDB(t *testing.T, session *fakeSession) *CDB {
	ctrl := gomock.NewController(t)
	client := gocql.NewMockClient(ctrl)
	return NewCassandraDBFromSession(&config.NoSQL{}, session, testlogger.New(t), nil, DbWithClient(client))
}

func TestInsertIntoAsyncWorkflowQueue(t *testing.T) {
	session := &fakeSession{query: &fakeQuery{}}
	db := newAsyncQueueTestDB(t, session)

	err := db.InsertIntoAsyncWorkflowQueue(context.Background(), &nosqlplugin.AsyncWorkflowQueueMessageRow{
		ShardID: 3, ID: 42, Payload: []byte("p"), Encoding: "thriftrw", PartitionKey: "wf-1", CurrentTimeStamp: FixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		`INSERT INTO async_workflow_queue (shard_id, message_id, message_payload, encoding, partition_key, created_time) VALUES(3, 42, [112], thriftrw, wf-1, 2025-01-06T15:00:00Z)`,
	}
	if diff := cmp.Diff(want, session.queries); diff != "" {
		t.Fatalf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestRangeDeleteAsyncWorkflowMessages(t *testing.T) {
	session := &fakeSession{query: &fakeQuery{}}
	db := newAsyncQueueTestDB(t, session)

	if err := db.RangeDeleteAsyncWorkflowMessages(context.Background(), 3, 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{`DELETE FROM async_workflow_queue WHERE shard_id=3 AND message_id <= 20`}
	if diff := cmp.Diff(want, session.queries); diff != "" {
		t.Fatalf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectAsyncWorkflowMessagesFrom(t *testing.T) {
	iter := &fakeIter{mapScanInputs: []map[string]interface{}{
		{"message_id": int64(1), "message_payload": []byte("a"), "encoding": "thriftrw", "partition_key": "wf-1", "created_time": FixedTime},
		{"message_id": int64(2), "message_payload": []byte("b"), "encoding": "thriftrw", "partition_key": "wf-2", "created_time": FixedTime},
	}}
	session := &fakeSession{query: &fakeQuery{iter: iter}}
	db := newAsyncQueueTestDB(t, session)

	rows, err := db.SelectAsyncWorkflowMessagesFrom(context.Background(), 3, 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []*nosqlplugin.AsyncWorkflowQueueMessageRow{
		{ShardID: 3, ID: 1, Payload: []byte("a"), Encoding: "thriftrw", PartitionKey: "wf-1", CurrentTimeStamp: FixedTime},
		{ShardID: 3, ID: 2, Payload: []byte("b"), Encoding: "thriftrw", PartitionKey: "wf-2", CurrentTimeStamp: FixedTime},
	}
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Fatalf("row mismatch (-want +got):\n%s", diff)
	}
	wantQueries := []string{`SELECT message_id, message_payload, encoding, partition_key, created_time FROM async_workflow_queue WHERE shard_id=3 AND message_id > 0 LIMIT 10`}
	if diff := cmp.Diff(wantQueries, session.queries); diff != "" {
		t.Fatalf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectAsyncWorkflowQueueMetadata(t *testing.T) {
	session := &fakeSession{query: &fakeQuery{scanValues: []interface{}{int64(12), int64(4)}}}
	db := newAsyncQueueTestDB(t, session)

	row, err := db.SelectAsyncWorkflowQueueMetadata(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &nosqlplugin.AsyncWorkflowQueueMetadataRow{ShardID: 3, AckLevel: 12, Version: 4}
	if diff := cmp.Diff(want, row); diff != "" {
		t.Fatalf("row mismatch (-want +got):\n%s", diff)
	}
	wantQueries := []string{`SELECT ack_level, version FROM async_workflow_queue_metadata WHERE shard_id=3`}
	if diff := cmp.Diff(wantQueries, session.queries); diff != "" {
		t.Fatalf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateAsyncWorkflowQueueMetadataCas(t *testing.T) {
	session := &fakeSession{query: &fakeQuery{scanCASApplied: true}}
	db := newAsyncQueueTestDB(t, session)

	err := db.UpdateAsyncWorkflowQueueMetadataCas(context.Background(), nosqlplugin.AsyncWorkflowQueueMetadataRow{
		ShardID: 3, AckLevel: 9, Version: 5, CurrentTimeStamp: FixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{`UPDATE async_workflow_queue_metadata SET ack_level = 9, version = 5, last_updated_time = 2025-01-06T15:00:00Z WHERE shard_id=3 IF version = 4`}
	if diff := cmp.Diff(want, session.queries); diff != "" {
		t.Fatalf("query mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateAsyncWorkflowQueueMetadataCas_NotApplied(t *testing.T) {
	session := &fakeSession{query: &fakeQuery{scanCASApplied: false}}
	db := newAsyncQueueTestDB(t, session)

	err := db.UpdateAsyncWorkflowQueueMetadataCas(context.Background(), nosqlplugin.AsyncWorkflowQueueMetadataRow{
		ShardID: 3, AckLevel: 9, Version: 5,
	})
	if _, ok := err.(*nosqlplugin.ConditionFailure); !ok {
		t.Fatalf("expected ConditionFailure, got %v", err)
	}
}
