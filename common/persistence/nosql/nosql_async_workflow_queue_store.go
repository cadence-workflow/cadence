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

package nosql

import (
	"context"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// nosqlAsyncWorkflowQueueStore is the NoSQL-backed AsyncWorkflowQueueStore.
//
// Each shard partition has a single writer (the history shard owner), so message ids are
// a monotonic sequence and enqueue is a plain insert with no conditional-write contention. The next id
// is max(last stored id, current ack level) + 1 so ids are never reused after a range delete.
type nosqlAsyncWorkflowQueueStore struct {
	nosqlStore
}

func newNoSQLAsyncWorkflowQueueStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.AsyncWorkflowQueueStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlAsyncWorkflowQueueStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) Enqueue(
	ctx context.Context,
	request *persistence.EnqueueAsyncWorkflowMessageRequest,
) (*persistence.EnqueueAsyncWorkflowMessageResponse, error) {
	if err := q.ensureQueueMetadata(ctx, request.ShardID, request.CurrentTimeStamp); err != nil {
		return nil, err
	}
	lastMessageID, err := q.getLastMessageID(ctx, request.ShardID, false)
	if err != nil {
		return nil, err
	}
	ackLevel, err := q.getAckLevel(ctx, request.ShardID)
	if err != nil {
		return nil, err
	}
	nextID := getNextAsyncWorkflowID(lastMessageID, ackLevel)
	if err := q.db.InsertIntoAsyncWorkflowQueue(ctx, q.toMessageRow(request, nextID)); err != nil {
		return nil, convertCommonErrors(q.db, "EnqueueAsyncWorkflowMessage", err)
	}
	return &persistence.EnqueueAsyncWorkflowMessageResponse{MessageID: nextID}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) EnqueueToDLQ(
	ctx context.Context,
	request *persistence.EnqueueAsyncWorkflowMessageRequest,
) (*persistence.EnqueueAsyncWorkflowMessageResponse, error) {
	lastMessageID, err := q.getLastMessageID(ctx, request.ShardID, true)
	if err != nil {
		return nil, err
	}
	nextID := lastMessageID + 1
	if err := q.db.InsertIntoAsyncWorkflowDLQ(ctx, q.toMessageRow(request, nextID)); err != nil {
		return nil, convertCommonErrors(q.db, "EnqueueAsyncWorkflowMessageToDLQ", err)
	}
	return &persistence.EnqueueAsyncWorkflowMessageResponse{MessageID: nextID}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) ReadMessages(
	ctx context.Context,
	request *persistence.ReadAsyncWorkflowMessagesRequest,
) (*persistence.ReadAsyncWorkflowMessagesResponse, error) {
	rows, err := q.db.SelectAsyncWorkflowMessagesFrom(ctx, request.ShardID, request.LastMessageID, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadAsyncWorkflowMessages", err)
	}
	return &persistence.ReadAsyncWorkflowMessagesResponse{Messages: q.toMessageList(rows)}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) ReadMessagesFromDLQ(
	ctx context.Context,
	request *persistence.ReadAsyncWorkflowMessagesRequest,
) (*persistence.ReadAsyncWorkflowMessagesResponse, error) {
	rows, err := q.db.SelectAsyncWorkflowDLQMessagesFrom(ctx, request.ShardID, request.LastMessageID, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadAsyncWorkflowMessagesFromDLQ", err)
	}
	return &persistence.ReadAsyncWorkflowMessagesResponse{Messages: q.toMessageList(rows)}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) UpdateAckLevel(
	ctx context.Context,
	request *persistence.UpdateAsyncWorkflowAckLevelRequest,
) error {
	if err := q.ensureQueueMetadata(ctx, request.ShardID, request.CurrentTimeStamp); err != nil {
		return err
	}
	metadata, err := q.db.SelectAsyncWorkflowQueueMetadata(ctx, request.ShardID)
	if err != nil {
		return convertCommonErrors(q.db, "UpdateAsyncWorkflowAckLevel", err)
	}
	// Never move the ack level backwards.
	if request.AckLevel <= metadata.AckLevel {
		return nil
	}
	err = q.db.UpdateAsyncWorkflowQueueMetadataCas(ctx, nosqlplugin.AsyncWorkflowQueueMetadataRow{
		ShardID:          request.ShardID,
		AckLevel:         request.AckLevel,
		Version:          metadata.Version + 1,
		CurrentTimeStamp: request.CurrentTimeStamp,
	})
	if err != nil {
		return convertCommonErrors(q.db, "UpdateAsyncWorkflowAckLevel", err)
	}
	return nil
}

func (q *nosqlAsyncWorkflowQueueStore) GetAckLevel(
	ctx context.Context,
	request *persistence.GetAsyncWorkflowAckLevelRequest,
) (*persistence.GetAsyncWorkflowAckLevelResponse, error) {
	ackLevel, err := q.getAckLevel(ctx, request.ShardID)
	if err != nil {
		return nil, err
	}
	return &persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: ackLevel}, nil
}

func (q *nosqlAsyncWorkflowQueueStore) RangeDeleteMessages(
	ctx context.Context,
	request *persistence.RangeDeleteAsyncWorkflowMessagesRequest,
) error {
	if err := q.db.RangeDeleteAsyncWorkflowMessages(ctx, request.ShardID, request.InclusiveEndMessageID); err != nil {
		return convertCommonErrors(q.db, "RangeDeleteAsyncWorkflowMessages", err)
	}
	return nil
}

func (q *nosqlAsyncWorkflowQueueStore) RangeDeleteMessagesFromDLQ(
	ctx context.Context,
	request *persistence.RangeDeleteAsyncWorkflowMessagesRequest,
) error {
	if err := q.db.RangeDeleteAsyncWorkflowDLQMessages(ctx, request.ShardID, request.InclusiveEndMessageID); err != nil {
		return convertCommonErrors(q.db, "RangeDeleteAsyncWorkflowMessagesFromDLQ", err)
	}
	return nil
}

func (q *nosqlAsyncWorkflowQueueStore) ensureQueueMetadata(
	ctx context.Context,
	shardID int,
	currentTimestamp time.Time,
) error {
	_, err := q.db.SelectAsyncWorkflowQueueMetadata(ctx, shardID)
	if err == nil {
		return nil
	}
	if !q.db.IsNotFoundError(err) {
		return convertCommonErrors(q.db, "EnsureAsyncWorkflowQueueMetadata", err)
	}
	// Insert an initial metadata row. InsertAsyncWorkflowQueueMetadata is a no-op if it already exists
	// (another writer created it concurrently), so this is safe.
	insertErr := q.db.InsertAsyncWorkflowQueueMetadata(ctx, nosqlplugin.AsyncWorkflowQueueMetadataRow{
		ShardID:          shardID,
		AckLevel:         emptyMessageID,
		Version:          0,
		CurrentTimeStamp: currentTimestamp,
	})
	if insertErr != nil {
		return convertCommonErrors(q.db, "EnsureAsyncWorkflowQueueMetadata", insertErr)
	}
	return nil
}

func (q *nosqlAsyncWorkflowQueueStore) getAckLevel(ctx context.Context, shardID int) (int64, error) {
	metadata, err := q.db.SelectAsyncWorkflowQueueMetadata(ctx, shardID)
	if err != nil {
		if q.db.IsNotFoundError(err) {
			return emptyMessageID, nil
		}
		return emptyMessageID, convertCommonErrors(q.db, "GetAsyncWorkflowAckLevel", err)
	}
	return metadata.AckLevel, nil
}

func (q *nosqlAsyncWorkflowQueueStore) getLastMessageID(ctx context.Context, shardID int, dlq bool) (int64, error) {
	var (
		id  int64
		err error
	)
	if dlq {
		id, err = q.db.SelectLastAsyncWorkflowDLQMessageID(ctx, shardID)
	} else {
		id, err = q.db.SelectLastAsyncWorkflowMessageID(ctx, shardID)
	}
	if err != nil {
		if q.db.IsNotFoundError(err) {
			return emptyMessageID, nil
		}
		return emptyMessageID, convertCommonErrors(q.db, "GetLastAsyncWorkflowMessageID", err)
	}
	return id, nil
}

func (q *nosqlAsyncWorkflowQueueStore) toMessageRow(request *persistence.EnqueueAsyncWorkflowMessageRequest, id int64) *nosqlplugin.AsyncWorkflowQueueMessageRow {
	return &nosqlplugin.AsyncWorkflowQueueMessageRow{
		ShardID:          request.ShardID,
		ID:               id,
		Payload:          request.Payload,
		Encoding:         request.Encoding,
		PartitionKey:     request.PartitionKey,
		CurrentTimeStamp: request.CurrentTimeStamp,
	}
}

func (q *nosqlAsyncWorkflowQueueStore) toMessageList(rows []*nosqlplugin.AsyncWorkflowQueueMessageRow) persistence.AsyncWorkflowMessageList {
	messages := make(persistence.AsyncWorkflowMessageList, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, &persistence.AsyncWorkflowMessage{
			ShardID:      row.ShardID,
			MessageID:    row.ID,
			Payload:      row.Payload,
			Encoding:     row.Encoding,
			PartitionKey: row.PartitionKey,
			CreatedTime:  row.CurrentTimeStamp,
		})
	}
	return messages
}

// getNextAsyncWorkflowID returns the next monotonic id for a shard: one past the greater of the last
// stored message id and the current ack level (so ids are not reused after acked messages are deleted).
func getNextAsyncWorkflowID(lastMessageID, ackLevel int64) int64 {
	next := lastMessageID
	if ackLevel > next {
		next = ackLevel
	}
	return next + 1
}
