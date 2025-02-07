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
	"time"

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
) (persistence.Queue, error) {
	return &sqlQueueStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
		},
		queueType: queueType,
		logger:    logger,
	}, nil
}

func (q *sqlQueueStore) EnqueueMessage(
	ctx context.Context,
	messagePayload []byte,
) error {
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

		_, err = tx.InsertIntoQueue(ctx, newQueueRow(q.queueType, getNextID(ackLevels, lastMessageID), messagePayload))
		return err
	})
}

func (q *sqlQueueStore) ReadMessages(
	ctx context.Context,
	lastMessageID int64,
	maxCount int,
) ([]*persistence.InternalQueueMessage, error) {

	rows, err := q.db.GetMessagesFromQueue(ctx, q.queueType, lastMessageID, maxCount)
	if err != nil {
		return nil, convertCommonErrors(q.db, "ReadMessages", "", err)
	}

	var messages []*persistence.InternalQueueMessage
	for _, row := range rows {
		messages = append(messages, &persistence.InternalQueueMessage{ID: row.MessageID, Payload: row.MessagePayload})
	}
	return messages, nil
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
	messageID int64,
) error {

	_, err := q.db.DeleteMessagesBefore(ctx, q.queueType, messageID)
	if err != nil {
		return convertCommonErrors(q.db, "DeleteMessagesBefore", "", err)
	}
	return nil
}

func (q *sqlQueueStore) UpdateAckLevel(ctx context.Context, messageID int64, clusterName string, now time.Time) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "UpdateAckLevel", func(tx sqlplugin.Tx) error {
		clusterAckLevels, err := tx.GetAckLevels(ctx, q.queueType, true)
		if err != nil {
			return err
		}

		if clusterAckLevels == nil {
			return tx.InsertAckLevel(ctx, q.queueType, messageID, clusterName)
		}

		// Ignore possibly delayed message
		if ackLevel, ok := clusterAckLevels[clusterName]; ok && ackLevel >= messageID {
			return nil
		}

		clusterAckLevels[clusterName] = messageID
		return tx.UpdateAckLevels(ctx, q.queueType, clusterAckLevels)
	})
}

func (q *sqlQueueStore) GetAckLevels(
	ctx context.Context,
) (map[string]int64, error) {
	result, err := q.db.GetAckLevels(ctx, q.queueType, false)
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetAckLevels", "", err)
	}
	return result, nil
}

func (q *sqlQueueStore) EnqueueMessageToDLQ(
	ctx context.Context,
	messagePayload []byte,
) error {
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
		_, err = tx.InsertIntoQueue(ctx, newQueueRow(q.getDLQTypeFromQueueType(), lastMessageID+1, messagePayload))
		return err
	})
}

func (q *sqlQueueStore) ReadMessagesFromDLQ(
	ctx context.Context,
	firstMessageID int64,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*persistence.InternalQueueMessage, []byte, error) {

	if len(pageToken) != 0 {
		lastReadMessageID, err := deserializePageToken(pageToken)
		if err != nil {
			return nil, nil, &types.InternalServiceError{
				Message: fmt.Sprintf("invalid next page token %v", pageToken)}
		}
		firstMessageID = lastReadMessageID
	}

	rows, err := q.db.GetMessagesBetween(ctx, q.getDLQTypeFromQueueType(), firstMessageID, lastMessageID, pageSize)
	if err != nil {
		return nil, nil, convertCommonErrors(q.db, "ReadMessagesFromDLQ", "", err)
	}

	var messages []*persistence.InternalQueueMessage
	for _, row := range rows {
		messages = append(messages, &persistence.InternalQueueMessage{ID: row.MessageID, Payload: row.MessagePayload})
	}

	var newPagingToken []byte
	if messages != nil && len(messages) >= pageSize {
		lastReadMessageID := messages[len(messages)-1].ID
		newPagingToken = serializePageToken(int64(lastReadMessageID))
	}
	return messages, newPagingToken, nil
}

func (q *sqlQueueStore) DeleteMessageFromDLQ(
	ctx context.Context,
	messageID int64,
) error {
	_, err := q.db.DeleteMessage(ctx, q.getDLQTypeFromQueueType(), messageID)
	if err != nil {
		return convertCommonErrors(q.db, "DeleteMessageFromDLQ", "", err)
	}
	return nil
}

func (q *sqlQueueStore) RangeDeleteMessagesFromDLQ(
	ctx context.Context,
	firstMessageID int64,
	lastMessageID int64,
) error {
	_, err := q.db.RangeDeleteMessages(ctx, q.getDLQTypeFromQueueType(), firstMessageID, lastMessageID)
	if err != nil {
		return convertCommonErrors(q.db, "RangeDeleteMessagesFromDLQ", "", err)
	}
	return nil
}

func (q *sqlQueueStore) UpdateDLQAckLevel(ctx context.Context, messageID int64, clusterName string, now time.Time) error {
	return q.txExecute(ctx, sqlplugin.DbDefaultShard, "UpdateDLQAckLevel", func(tx sqlplugin.Tx) error {
		clusterAckLevels, err := tx.GetAckLevels(ctx, q.getDLQTypeFromQueueType(), true)
		if err != nil {
			return err
		}

		if clusterAckLevels == nil {
			return tx.InsertAckLevel(ctx, q.getDLQTypeFromQueueType(), messageID, clusterName)
		}

		// Ignore possibly delayed message
		if ackLevel, ok := clusterAckLevels[clusterName]; ok && ackLevel >= messageID {
			return nil
		}

		clusterAckLevels[clusterName] = messageID
		return tx.UpdateAckLevels(ctx, q.getDLQTypeFromQueueType(), clusterAckLevels)
	})
}

func (q *sqlQueueStore) GetDLQAckLevels(
	ctx context.Context,
) (map[string]int64, error) {
	result, err := q.db.GetAckLevels(ctx, q.getDLQTypeFromQueueType(), false)
	if err != nil {
		return nil, convertCommonErrors(q.db, "GetDLQAckLevels", "", err)
	}
	return result, nil
}

func (q *sqlQueueStore) GetDLQSize(
	ctx context.Context,
) (int64, error) {
	result, err := q.db.GetQueueSize(ctx, q.getDLQTypeFromQueueType())
	if err != nil {
		return 0, convertCommonErrors(q.db, "GetDLQSize", "", err)
	}
	return result, nil
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
