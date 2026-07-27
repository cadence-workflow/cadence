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
	"fmt"
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// InsertIntoAsyncWorkflowQueue inserts a message into async_workflow_queue.
// The shard owner is the single writer for shardID, so this is a plain insert.
func (db *CDB) InsertIntoAsyncWorkflowQueue(ctx context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
	query := db.session.Query(templateEnqueueAsyncWorkflowMessageQuery,
		row.ShardID, row.ID, row.Payload, row.Encoding, row.PartitionKey, row.CurrentTimeStamp,
	).WithContext(ctx)
	return query.Exec()
}

// SelectLastAsyncWorkflowMessageID returns the id of the last message in the shard.
func (db *CDB) SelectLastAsyncWorkflowMessageID(ctx context.Context, shardID int) (int64, error) {
	return db.selectLastAsyncWorkflowMessageID(ctx, templateGetLastAsyncWorkflowMessageIDQuery, shardID)
}

// SelectAsyncWorkflowMessagesFrom reads up to maxRows messages with message_id > exclusiveBeginMessageID.
func (db *CDB) SelectAsyncWorkflowMessagesFrom(ctx context.Context, shardID int, exclusiveBeginMessageID int64, maxRows int) ([]*nosqlplugin.AsyncWorkflowQueueMessageRow, error) {
	return db.selectAsyncWorkflowMessagesFrom(ctx, templateGetAsyncWorkflowMessagesFromQuery, shardID, exclusiveBeginMessageID, maxRows)
}

// RangeDeleteAsyncWorkflowMessages deletes all messages with message_id <= inclusiveEndMessageID.
func (db *CDB) RangeDeleteAsyncWorkflowMessages(ctx context.Context, shardID int, inclusiveEndMessageID int64) error {
	query := db.session.Query(templateRangeDeleteAsyncWorkflowMessagesQuery, shardID, inclusiveEndMessageID).WithContext(ctx)
	return db.executeWithConsistencyAll(query)
}

// InsertAsyncWorkflowQueueMetadata inserts an initial metadata row if it does not already exist.
func (db *CDB) InsertAsyncWorkflowQueueMetadata(ctx context.Context, row nosqlplugin.AsyncWorkflowQueueMetadataRow) error {
	query := db.session.Query(templateInsertAsyncWorkflowQueueMetadataQuery,
		row.ShardID, row.AckLevel, row.Version, row.CurrentTimeStamp,
	).WithContext(ctx)

	// NOTE: Must pass nils to be compatible with ScyllaDB's LWT behavior.
	// See https://docs.scylladb.com/kb/lwt-differences/
	_, err := query.ScanCAS(nil, nil, nil, nil)
	if err != nil {
		return err
	}
	// it's ok if the query is not applied, which means the record already exists.
	return nil
}

// UpdateAsyncWorkflowQueueMetadataCas conditionally updates the metadata row if current version == row.Version-1.
func (db *CDB) UpdateAsyncWorkflowQueueMetadataCas(ctx context.Context, row nosqlplugin.AsyncWorkflowQueueMetadataRow) error {
	query := db.session.Query(templateUpdateAsyncWorkflowQueueMetadataQuery,
		row.AckLevel,
		row.Version,
		row.CurrentTimeStamp,
		row.ShardID,
		row.Version-1,
	).WithContext(ctx)

	// NOTE: Must pass nils to be compatible with ScyllaDB's LWT behavior.
	applied, err := query.ScanCAS(nil, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	if !applied {
		return nosqlplugin.NewConditionFailure("async_workflow_queue_metadata")
	}
	return nil
}

// SelectAsyncWorkflowQueueMetadata reads the metadata row for the shard.
func (db *CDB) SelectAsyncWorkflowQueueMetadata(ctx context.Context, shardID int) (*nosqlplugin.AsyncWorkflowQueueMetadataRow, error) {
	query := db.session.Query(templateGetAsyncWorkflowQueueMetadataQuery, shardID).WithContext(ctx)
	var ackLevel int64
	var version int64
	if err := query.Scan(&ackLevel, &version); err != nil {
		return nil, err
	}
	return &nosqlplugin.AsyncWorkflowQueueMetadataRow{
		ShardID:  shardID,
		AckLevel: ackLevel,
		Version:  version,
	}, nil
}

// InsertIntoAsyncWorkflowDLQ inserts a poison message into async_workflow_queue_dlq.
func (db *CDB) InsertIntoAsyncWorkflowDLQ(ctx context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
	query := db.session.Query(templateEnqueueAsyncWorkflowDLQMessageQuery,
		row.ShardID, row.ID, row.Payload, row.Encoding, row.PartitionKey, row.CurrentTimeStamp,
	).WithContext(ctx)
	return query.Exec()
}

// SelectLastAsyncWorkflowDLQMessageID returns the id of the last DLQ message in the shard.
func (db *CDB) SelectLastAsyncWorkflowDLQMessageID(ctx context.Context, shardID int) (int64, error) {
	return db.selectLastAsyncWorkflowMessageID(ctx, templateGetLastAsyncWorkflowDLQMessageIDQuery, shardID)
}

// SelectAsyncWorkflowDLQMessagesFrom reads up to maxRows DLQ messages with message_id > exclusiveBeginMessageID.
func (db *CDB) SelectAsyncWorkflowDLQMessagesFrom(ctx context.Context, shardID int, exclusiveBeginMessageID int64, maxRows int) ([]*nosqlplugin.AsyncWorkflowQueueMessageRow, error) {
	return db.selectAsyncWorkflowMessagesFrom(ctx, templateGetAsyncWorkflowDLQMessagesFromQuery, shardID, exclusiveBeginMessageID, maxRows)
}

// RangeDeleteAsyncWorkflowDLQMessages deletes all DLQ messages with message_id <= inclusiveEndMessageID.
func (db *CDB) RangeDeleteAsyncWorkflowDLQMessages(ctx context.Context, shardID int, inclusiveEndMessageID int64) error {
	query := db.session.Query(templateRangeDeleteAsyncWorkflowDLQMessagesQuery, shardID, inclusiveEndMessageID).WithContext(ctx)
	return db.executeWithConsistencyAll(query)
}

func (db *CDB) selectLastAsyncWorkflowMessageID(ctx context.Context, template string, shardID int) (int64, error) {
	query := db.session.Query(template, shardID).WithContext(ctx)
	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return 0, err
	}
	return result["message_id"].(int64), nil
}

func (db *CDB) selectAsyncWorkflowMessagesFrom(ctx context.Context, template string, shardID int, exclusiveBeginMessageID int64, maxRows int) ([]*nosqlplugin.AsyncWorkflowQueueMessageRow, error) {
	query := db.session.Query(template, shardID, exclusiveBeginMessageID, maxRows).WithContext(ctx)
	iter := query.Iter()
	if iter == nil {
		return nil, fmt.Errorf("SelectAsyncWorkflowMessagesFrom operation failed. Not able to create query iterator")
	}

	var result []*nosqlplugin.AsyncWorkflowQueueMessageRow
	message := make(map[string]interface{})
	for iter.MapScan(message) {
		row := &nosqlplugin.AsyncWorkflowQueueMessageRow{
			ShardID: shardID,
			ID:      message["message_id"].(int64),
			Payload: message["message_payload"].([]byte),
		}
		if v, ok := message["encoding"].(string); ok {
			row.Encoding = v
		}
		if v, ok := message["partition_key"].(string); ok {
			row.PartitionKey = v
		}
		if v, ok := message["created_time"].(time.Time); ok {
			row.CurrentTimeStamp = v
		}
		result = append(result, row)
		message = make(map[string]interface{})
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}
	return result, nil
}
