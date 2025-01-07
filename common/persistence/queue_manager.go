// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package persistence

import "context"

type (
	queueManager struct {
		persistence Queue
	}
)

var _ QueueManager = (*queueManager)(nil)

// NewQueueManager returns a new QueueManager
func NewQueueManager(
	persistence Queue,
) QueueManager {
	return &queueManager{
		persistence: persistence,
	}
}

func (q *queueManager) Close() {
	q.persistence.Close()
}

func (q *queueManager) EnqueueMessage(ctx context.Context, messagePayload []byte) error {
	return q.persistence.EnqueueMessage(ctx, messagePayload)
}

func (q *queueManager) ReadMessages(ctx context.Context, lastMessageID int64, maxCount int) (QueueMessageList, error) {
	resp, err := q.persistence.ReadMessages(ctx, lastMessageID, maxCount)
	if err != nil {
		return nil, err
	}
	var output []*QueueMessage
	for _, message := range resp {
		output = append(output, q.fromInternalQueueMessage(message))
	}
	return output, nil
}

func (q *queueManager) DeleteMessagesBefore(ctx context.Context, messageID int64) error {
	return q.persistence.DeleteMessagesBefore(ctx, messageID)
}

func (q *queueManager) UpdateAckLevel(ctx context.Context, messageID int64, clusterName string) error {
	return q.persistence.UpdateAckLevel(ctx, messageID, clusterName)
}

func (q *queueManager) GetAckLevels(ctx context.Context) (map[string]int64, error) {
	return q.persistence.GetAckLevels(ctx)
}

func (q *queueManager) EnqueueMessageToDLQ(ctx context.Context, messagePayload []byte) error {
	return q.persistence.EnqueueMessageToDLQ(ctx, messagePayload)
}

func (q *queueManager) ReadMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64, pageSize int, pageToken []byte) ([]*QueueMessage, []byte, error) {
	resp, data, err := q.persistence.ReadMessagesFromDLQ(ctx, firstMessageID, lastMessageID, pageSize, pageToken)
	if resp == nil {
		return nil, data, err
	}
	var output []*QueueMessage
	for _, message := range resp {
		output = append(output, q.fromInternalQueueMessage(message))
	}
	return output, data, err
}

func (q *queueManager) DeleteMessageFromDLQ(ctx context.Context, messageID int64) error {
	return q.persistence.DeleteMessageFromDLQ(ctx, messageID)
}

func (q *queueManager) RangeDeleteMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64) error {
	return q.persistence.RangeDeleteMessagesFromDLQ(ctx, firstMessageID, lastMessageID)
}

func (q *queueManager) UpdateDLQAckLevel(ctx context.Context, messageID int64, clusterName string) error {
	return q.persistence.UpdateDLQAckLevel(ctx, messageID, clusterName)
}

func (q *queueManager) GetDLQAckLevels(ctx context.Context) (map[string]int64, error) {
	return q.persistence.GetDLQAckLevels(ctx)
}

func (q *queueManager) GetDLQSize(ctx context.Context) (int64, error) {
	return q.persistence.GetDLQSize(ctx)
}

func (q *queueManager) fromInternalQueueMessage(message *InternalQueueMessage) *QueueMessage {
	return &QueueMessage{
		ID:        message.ID,
		QueueType: message.QueueType,
		Payload:   message.Payload,
	}
}
