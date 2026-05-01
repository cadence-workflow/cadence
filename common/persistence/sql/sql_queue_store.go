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

package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

type (
	sqlQueueStore struct {
		queueType persistence.QueueType
		logger    log.Logger
		sqlStore
	}
)

func newQueueStore(
	db sqlplugin.DB,
	logger log.Logger,
	queueType persistence.QueueType,
) (persistence.QueueStore, error) {
	return &sqlQueueStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
		},
		queueType: queueType,
		logger:    logger,
	}, nil
}

func (q *sqlQueueStore) EnqueueMessage(ctx context.Context, request *persistence.InternalEnqueueMessageRequest) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "EnqueueMessage", func(tx sqlplugin.Tx) error {
		lastMessageID, err := tx.GetLastEnqueuedMessageIDForUpdate(ctx, q.queueType)
		if err != nil {
			if err == sql.ErrNoRows {
				lastMessageID = -1
			} else {
				return err
			}
		}

		ackLevels, err := tx.GetAckLevels(ctx, q.queueType, true)
		if err != nil {
			return err
		}

		_, err = tx.InsertIntoQueue(ctx, newQueueRow(q.queueType, getNextID(ackLevels, lastMessageID), request.MessagePayload))
		return err
	})
}

func (q *sqlQueueStore) ReadMessages(
	ctx context.Context,
	request *persistence.InternalReadMessagesRequest,
) (*persistence.InternalReadMessagesResponse, error) {

	rows, err := q.db.GetMessagesFromQueue(ctx, q.queueType, request.LastMessageID, request.MaxCount)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadMessages", "", err)
	}

	var messages []*persistence.InternalQueueMessage
	for _, row := range rows {
		messages = append(messages, &persistence.InternalQueueMessage{ID: row.MessageID, Payload: row.MessagePayload})
	}
	return &persistence.InternalReadMessagesResponse{Messages: messages}, nil
}

func newQueueRow(
	queueType persistence.QueueType,
	messageID int64,
	payload []byte,
) *sqlplugin.QueueRow {

	return &sqlplugin.QueueRow{QueueType: queueType, MessageID: messageID, MessagePayload: payload}
}

func (q *sqlQueueStore) DeleteMessagesBefore(
	ctx context.Context,
	request *persistence.InternalDeleteMessagesBeforeRequest,
) error {

	_, err := q.db.DeleteMessagesBefore(ctx, q.queueType, request.MessageID)
	if err != nil {
		return convertCommonErrors(q.db, "DeleteMessagesBefore", "", err)
	}
	return nil
}

func (q *sqlQueueStore) UpdateAckLevel(ctx context.Context, request *persistence.InternalUpdateAckLevelRequest) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "UpdateAckLevel", func(tx sqlplugin.Tx) error {
		clusterAckLevels, err := tx.GetAckLevels(ctx, q.queueType, true)
		if err != nil {
			return err
		}

		if clusterAckLevels == nil {
			return tx.InsertAckLevel(ctx, q.queueType, request.MessageID, request.ClusterName)
		}

		// Ignore possibly delayed message
		if ackLevel, ok := clusterAckLevels[request.ClusterName]; ok && ackLevel >= request.MessageID {
			return nil
		}

		clusterAckLevels[request.ClusterName] = request.MessageID
		return tx.UpdateAckLevels(ctx, q.queueType, clusterAckLevels)
	})
}

func (q *sqlQueueStore) GetAckLevels(
	ctx context.Context,
	_ *persistence.InternalGetAckLevelsRequest,
) (*persistence.InternalGetAckLevelsResponse, error) {
	result, err := q.db.GetAckLevels(ctx, q.queueType, false)
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetAckLevels", "", err)
	}
	return &persistence.InternalGetAckLevelsResponse{AckLevels: result}, nil
}

func (q *sqlQueueStore) EnqueueMessageToDLQ(ctx context.Context, request *persistence.InternalEnqueueMessageToDLQRequest) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "EnqueueMessageToDLQ", func(tx sqlplugin.Tx) error {
		var err error
		lastMessageID, err := tx.GetLastEnqueuedMessageIDForUpdate(ctx, q.getDLQTypeFromQueueType())
		if err != nil {
			if err == sql.ErrNoRows {
				lastMessageID = -1
			} else {
				return err
			}
		}
		_, err = tx.InsertIntoQueue(ctx, newQueueRow(q.getDLQTypeFromQueueType(), lastMessageID+1, request.MessagePayload))
		return err
	})
}

func (q *sqlQueueStore) ReadMessagesFromDLQ(
	ctx context.Context,
	request *persistence.InternalReadMessagesFromDLQRequest,
) (*persistence.InternalReadMessagesFromDLQResponse, error) {

	firstMessageID := request.FirstMessageID
	if len(request.PageToken) != 0 {
		lastReadMessageID, err := deserializePageToken(request.PageToken)
		if err != nil {
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf("invalid next page token %v", request.PageToken)}
		}
		firstMessageID = lastReadMessageID
	}

	rows, err := q.db.GetMessagesBetween(ctx, q.getDLQTypeFromQueueType(), firstMessageID, request.LastMessageID, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadMessagesFromDLQ", "", err)
	}

	var messages []*persistence.InternalQueueMessage
	for _, row := range rows {
		messages = append(messages, &persistence.InternalQueueMessage{ID: row.MessageID, Payload: row.MessagePayload})
	}

	var newPagingToken []byte
	if messages != nil && len(messages) >= request.PageSize {
		lastReadMessageID := messages[len(messages)-1].ID
		newPagingToken = serializePageToken(int64(lastReadMessageID))
	}
	return &persistence.InternalReadMessagesFromDLQResponse{
		Messages:      messages,
		NextPageToken: newPagingToken,
	}, nil
}

func (q *sqlQueueStore) DeleteMessageFromDLQ(
	ctx context.Context,
	request *persistence.InternalDeleteMessageFromDLQRequest,
) error {
	_, err := q.db.DeleteMessage(ctx, q.getDLQTypeFromQueueType(), request.MessageID)
	if err != nil {
		return convertCommonErrors(q.db, "DeleteMessageFromDLQ", "", err)
	}
	return nil
}

func (q *sqlQueueStore) RangeDeleteMessagesFromDLQ(
	ctx context.Context,
	request *persistence.InternalRangeDeleteMessagesFromDLQRequest,
) error {
	_, err := q.db.RangeDeleteMessages(ctx, q.getDLQTypeFromQueueType(), request.FirstMessageID, request.LastMessageID)
	if err != nil {
		return convertCommonErrors(q.db, "RangeDeleteMessagesFromDLQ", "", err)
	}
	return nil
}

func (q *sqlQueueStore) UpdateDLQAckLevel(ctx context.Context, request *persistence.InternalUpdateDLQAckLevelRequest) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "UpdateDLQAckLevel", func(tx sqlplugin.Tx) error {
		clusterAckLevels, err := tx.GetAckLevels(ctx, q.getDLQTypeFromQueueType(), true)
		if err != nil {
			return err
		}

		if clusterAckLevels == nil {
			return tx.InsertAckLevel(ctx, q.getDLQTypeFromQueueType(), request.MessageID, request.ClusterName)
		}

		// Ignore possibly delayed message
		if ackLevel, ok := clusterAckLevels[request.ClusterName]; ok && ackLevel >= request.MessageID {
			return nil
		}

		clusterAckLevels[request.ClusterName] = request.MessageID
		return tx.UpdateAckLevels(ctx, q.getDLQTypeFromQueueType(), clusterAckLevels)
	})
}

func (q *sqlQueueStore) GetDLQAckLevels(
	ctx context.Context,
	_ *persistence.InternalGetDLQAckLevelsRequest,
) (*persistence.InternalGetDLQAckLevelsResponse, error) {
	result, err := q.db.GetAckLevels(ctx, q.getDLQTypeFromQueueType(), false)
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetDLQAckLevels", "", err)
	}
	return &persistence.InternalGetDLQAckLevelsResponse{AckLevels: result}, nil
}

func (q *sqlQueueStore) GetDLQSize(
	ctx context.Context,
	_ *persistence.InternalGetDLQSizeRequest,
) (*persistence.InternalGetDLQSizeResponse, error) {
	result, err := q.db.GetQueueSize(ctx, q.getDLQTypeFromQueueType())
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetDLQSize", "", err)
	}
	return &persistence.InternalGetDLQSizeResponse{Size: result}, nil
}

func (q *sqlQueueStore) getDLQTypeFromQueueType() persistence.QueueType {
	return -q.queueType
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
