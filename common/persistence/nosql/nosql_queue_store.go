// Copyright (c) 2019 Uber Technologies, Inc.
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
	"fmt"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	emptyMessageID = -1
)

type nosqlQueueStore struct {
	queueType persistence.QueueType
	nosqlStore
	timeSrc clock.TimeSource
}

func newNoSQLQueueStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	queueType persistence.QueueType,
	dc *persistence.DynamicConfiguration,
) (persistence.QueueStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	queue := &nosqlQueueStore{
		nosqlStore: shardedStore.GetDefaultShard(),
		queueType:  queueType,
		// TODO: move the time generation to the persistence manager layer
		timeSrc: clock.NewRealTimeSource(),
	}

	if err := queue.createQueueMetadataEntryIfNotExist(queue.timeSrc.Now()); err != nil {
		return nil, fmt.Errorf("failed to check and create queue metadata entry: %v", err)
	}

	return queue, nil
}

func (q *nosqlQueueStore) createQueueMetadataEntryIfNotExist(currentTimestamp time.Time) error {
	queueMetadata, err := q.getQueueMetadata(context.Background(), q.queueType)
	if err != nil {
		return err
	}

	if queueMetadata == nil {
		if err := q.insertInitialQueueMetadataRecord(context.Background(), q.queueType, currentTimestamp); err != nil {
			return err
		}
	}

	dlqMetadata, err := q.getQueueMetadata(context.Background(), q.getDLQTypeFromQueueType())
	if err != nil {
		return err
	}

	if dlqMetadata == nil {
		return q.insertInitialQueueMetadataRecord(context.Background(), q.getDLQTypeFromQueueType(), currentTimestamp)
	}

	return nil
}

// Warning: This is not a safe concurrent operation in its current state.
// It's only used for domain replication at the moment, but needs a conditional write guard
// for concurrent use
func (q *nosqlQueueStore) EnqueueMessage(ctx context.Context, request *persistence.InternalEnqueueMessageRequest) error {
	lastMessageID, err := q.getLastMessageID(ctx, q.queueType)
	if err != nil {
		return err
	}
	ackLevelsResp, err := q.GetAckLevels(ctx, &persistence.InternalGetAckLevelsRequest{})
	if err != nil {
		return err
	}
	_, err = q.tryEnqueue(ctx, q.queueType, getNextID(ackLevelsResp.AckLevels, lastMessageID), request.MessagePayload, request.CurrentTimeStamp)
	return err
}

func (q *nosqlQueueStore) EnqueueMessageToDLQ(ctx context.Context, request *persistence.InternalEnqueueMessageToDLQRequest) error {
	// Use negative queue type as the dlq type
	lastMessageID, err := q.getLastMessageID(ctx, q.getDLQTypeFromQueueType())
	if err != nil {
		return err
	}

	_, err = q.tryEnqueue(ctx, q.getDLQTypeFromQueueType(), lastMessageID+1, request.MessagePayload, request.CurrentTimeStamp)
	return err
}

func (q *nosqlQueueStore) tryEnqueue(ctx context.Context, queueType persistence.QueueType, messageID int64, messagePayload []byte, currentTimeStamp time.Time) (int64, error) {
	err := q.db.InsertIntoQueue(ctx, &nosqlplugin.QueueMessageRow{
		QueueType:        queueType,
		ID:               messageID,
		Payload:          messagePayload,
		CurrentTimeStamp: currentTimeStamp,
	})
	if err != nil {
		if _, ok := err.(*nosqlplugin.ConditionFailure); ok {
			return emptyMessageID, &persistence.ConditionFailedError{Msg: fmt.Sprintf("message ID %v exists in queue", messageID)}
		}

		return emptyMessageID, convertCommonErrors(q.db, fmt.Sprintf("EnqueueMessage, Type: %v", queueType), err)
	}

	return messageID, nil
}

func (q *nosqlQueueStore) getLastMessageID(
	ctx context.Context,
	queueType persistence.QueueType,
) (int64, error) {

	msgID, err := q.db.SelectLastEnqueuedMessageID(ctx, queueType)
	if err != nil {
		if q.db.IsNotFoundError(err) {
			return emptyMessageID, nil
		}

		return emptyMessageID, convertCommonErrors(q.db, fmt.Sprintf("GetLastMessageID, Type: %v", queueType), err)
	}

	return msgID, nil
}

func (q *nosqlQueueStore) ReadMessages(
	ctx context.Context,
	request *persistence.InternalReadMessagesRequest,
) (*persistence.InternalReadMessagesResponse, error) {
	messages, err := q.db.SelectMessagesFrom(ctx, q.queueType, request.LastMessageID, request.MaxCount)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadMessages", err)
	}
	var result []*persistence.InternalQueueMessage
	for _, msg := range messages {
		result = append(result, &persistence.InternalQueueMessage{
			ID:        msg.ID,
			QueueType: q.queueType,
			Payload:   msg.Payload,
		})
	}

	return &persistence.InternalReadMessagesResponse{Messages: result}, nil
}

func (q *nosqlQueueStore) ReadMessagesFromDLQ(
	ctx context.Context,
	request *persistence.InternalReadMessagesFromDLQRequest,
) (*persistence.InternalReadMessagesFromDLQResponse, error) {
	response, err := q.db.SelectMessagesBetween(ctx, nosqlplugin.SelectMessagesBetweenRequest{
		QueueType:               q.getDLQTypeFromQueueType(),
		ExclusiveBeginMessageID: request.FirstMessageID,
		InclusiveEndMessageID:   request.LastMessageID,
		PageSize:                request.PageSize,
		NextPageToken:           request.PageToken,
	})
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadMessagesFromDLQ", err)
	}
	var result []*persistence.InternalQueueMessage
	for _, msg := range response.Rows {
		result = append(result, &persistence.InternalQueueMessage{
			ID:        msg.ID,
			QueueType: msg.QueueType,
			Payload:   msg.Payload,
		})
	}

	return &persistence.InternalReadMessagesFromDLQResponse{
		Messages:      result,
		NextPageToken: response.NextPageToken,
	}, nil
}

func (q *nosqlQueueStore) DeleteMessagesBefore(
	ctx context.Context,
	request *persistence.InternalDeleteMessagesBeforeRequest,
) error {
	if err := q.db.DeleteMessagesBefore(ctx, q.queueType, request.MessageID); err != nil {
		return convertCommonErrors(q.db, "DeleteMessagesBefore", err)
	}

	return nil
}

func (q *nosqlQueueStore) DeleteMessageFromDLQ(
	ctx context.Context,
	request *persistence.InternalDeleteMessageFromDLQRequest,
) error {
	// Use negative queue type as the dlq type
	if err := q.db.DeleteMessage(ctx, q.getDLQTypeFromQueueType(), request.MessageID); err != nil {
		return convertCommonErrors(q.db, "DeleteMessageFromDLQ", err)
	}

	return nil
}

func (q *nosqlQueueStore) RangeDeleteMessagesFromDLQ(
	ctx context.Context,
	request *persistence.InternalRangeDeleteMessagesFromDLQRequest,
) error {
	// Use negative queue type as the dlq type
	if err := q.db.DeleteMessagesInRange(ctx, q.getDLQTypeFromQueueType(), request.FirstMessageID, request.LastMessageID); err != nil {
		return convertCommonErrors(q.db, "RangeDeleteMessagesFromDLQ", err)
	}

	return nil
}

func (q *nosqlQueueStore) insertInitialQueueMetadataRecord(ctx context.Context, queueType persistence.QueueType, currentTimeStamp time.Time) error {
	version := int64(0)

	row := nosqlplugin.QueueMetadataRow{
		Version:          version,
		QueueType:        queueType,
		CurrentTimeStamp: currentTimeStamp,
	}
	if err := q.db.InsertQueueMetadata(ctx, row); err != nil {
		return convertCommonErrors(q.db, fmt.Sprintf("InsertInitialQueueMetadataRecord, Type: %v", queueType), err)
	}

	return nil
}

func (q *nosqlQueueStore) UpdateAckLevel(ctx context.Context, request *persistence.InternalUpdateAckLevelRequest) error {
	return q.updateAckLevel(ctx, request.MessageID, request.ClusterName, q.queueType, request.CurrentTimeStamp)
}

func (q *nosqlQueueStore) GetAckLevels(
	ctx context.Context,
	_ *persistence.InternalGetAckLevelsRequest,
) (*persistence.InternalGetAckLevelsResponse, error) {
	queueMetadata, err := q.getQueueMetadata(ctx, q.queueType)
	if err != nil {
		return nil, err
	}
	if queueMetadata == nil {
		return &persistence.InternalGetAckLevelsResponse{AckLevels: map[string]int64{}}, nil
	}

	return &persistence.InternalGetAckLevelsResponse{AckLevels: queueMetadata.ClusterAckLevels}, nil
}

func (q *nosqlQueueStore) UpdateDLQAckLevel(ctx context.Context, request *persistence.InternalUpdateDLQAckLevelRequest) error {
	return q.updateAckLevel(ctx, request.MessageID, request.ClusterName, q.getDLQTypeFromQueueType(), request.CurrentTimeStamp)
}

func (q *nosqlQueueStore) GetDLQAckLevels(
	ctx context.Context,
	_ *persistence.InternalGetDLQAckLevelsRequest,
) (*persistence.InternalGetDLQAckLevelsResponse, error) {

	// Use negative queue type as the dlq type
	queueMetadata, err := q.getQueueMetadata(ctx, q.getDLQTypeFromQueueType())
	if err != nil {
		return nil, err
	}
	if queueMetadata == nil {
		return &persistence.InternalGetDLQAckLevelsResponse{AckLevels: map[string]int64{}}, nil
	}

	return &persistence.InternalGetDLQAckLevelsResponse{AckLevels: queueMetadata.ClusterAckLevels}, nil
}

func (q *nosqlQueueStore) GetDLQSize(
	ctx context.Context,
	_ *persistence.InternalGetDLQSizeRequest,
) (*persistence.InternalGetDLQSizeResponse, error) {

	size, err := q.db.GetQueueSize(ctx, q.getDLQTypeFromQueueType())
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetDLQSize", err)
	}
	return &persistence.InternalGetDLQSizeResponse{Size: size}, nil
}

func (q *nosqlQueueStore) getQueueMetadata(
	ctx context.Context,
	queueType persistence.QueueType,
) (*nosqlplugin.QueueMetadataRow, error) {
	row, err := q.db.SelectQueueMetadata(ctx, queueType)
	if err != nil {
		if q.db.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, convertCommonErrors(q.db, "GetQueueMetadata", err)
	}

	return row, nil
}

func (q *nosqlQueueStore) updateQueueMetadata(
	ctx context.Context,
	metadata *nosqlplugin.QueueMetadataRow,
) error {
	err := q.db.UpdateQueueMetadataCas(ctx, *metadata)
	if err != nil {
		if _, ok := err.(*nosqlplugin.ConditionFailure); ok {
			return &types.InternalServiceError{
				Message: "UpdateQueueMetadata operation encounter concurrent write.",
			}
		}

		return convertCommonErrors(q.db, "UpdateQueueMetadata", err)
	}

	return nil
}

// DLQ type of is the negative of number of the non-DLQ
func (q *nosqlQueueStore) getDLQTypeFromQueueType() persistence.QueueType {
	return -q.queueType
}

func (q *nosqlQueueStore) updateAckLevel(
	ctx context.Context,
	messageID int64,
	clusterName string,
	queueType persistence.QueueType,
	currentTimestamp time.Time,
) error {

	queueMetadata, err := q.getQueueMetadata(ctx, queueType)
	if err != nil {
		return err
	}
	if queueMetadata == nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("UpdateQueueMetadata operation: queue metadata not found for queue type %v", queueType),
		}
	}

	// Ignore possibly delayed message
	if ackLevel, ok := queueMetadata.ClusterAckLevels[clusterName]; ok && ackLevel >= messageID {
		return nil
	}

	queueMetadata.ClusterAckLevels[clusterName] = messageID
	queueMetadata.Version++
	queueMetadata.CurrentTimeStamp = currentTimestamp

	// Use negative queue type as the dlq type
	err = q.updateQueueMetadata(ctx, queueMetadata)
	if err != nil {
		return err
	}
	return nil
}

// if, for whatever reason, the ack-levels get ahead of the actual messages
// then ensure the next ID follows
func getNextID(acks map[string]int64, lastMessageID int64) int64 {
	o := lastMessageID
	for _, v := range acks {
		if v > o {
			o = v
		}
	}
	return o + 1
}
