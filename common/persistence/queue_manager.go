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

import (
	"context"

	"github.com/uber/cadence/common/clock"
)

type (
	queueManager struct {
		persistence QueueStore
		timeSrc     clock.TimeSource
	}
)

var _ QueueManager = (*queueManager)(nil)

// NewQueueManager returns a new QueueManager
func NewQueueManager(
	persistence QueueStore,
) QueueManager {
	return &queueManager{
		persistence: persistence,
		timeSrc:     clock.NewRealTimeSource(),
	}
}

func (q *queueManager) Close() {
	q.persistence.Close()
}

func (q *queueManager) EnqueueMessage(ctx context.Context, request *EnqueueMessageRequest) error {
	currentTimestamp := q.timeSrc.Now()
	return q.persistence.EnqueueMessage(ctx, &InternalEnqueueMessageRequest{
		MessagePayload:   request.MessagePayload,
		CurrentTimeStamp: currentTimestamp,
	})
}

func (q *queueManager) ReadMessages(ctx context.Context, request *ReadMessagesRequest) (*ReadMessagesResponse, error) {
	resp, err := q.persistence.ReadMessages(ctx, &InternalReadMessagesRequest{
		LastMessageID: request.LastMessageID,
		MaxCount:      request.MaxCount,
	})
	if err != nil {
		return nil, err
	}
	output := make(QueueMessageList, 0, len(resp.Messages))
	for _, message := range resp.Messages {
		output = append(output, q.fromInternalQueueMessage(message))
	}
	return &ReadMessagesResponse{Messages: output}, nil
}

func (q *queueManager) DeleteMessagesBefore(ctx context.Context, request *DeleteMessagesBeforeRequest) error {
	return q.persistence.DeleteMessagesBefore(ctx, &InternalDeleteMessagesBeforeRequest{
		MessageID: request.MessageID,
	})
}

func (q *queueManager) UpdateAckLevel(ctx context.Context, request *UpdateAckLevelRequest) error {
	currentTimestamp := q.timeSrc.Now()
	return q.persistence.UpdateAckLevel(ctx, &InternalUpdateAckLevelRequest{
		MessageID:        request.MessageID,
		ClusterName:      request.ClusterName,
		CurrentTimeStamp: currentTimestamp,
	})
}

func (q *queueManager) GetAckLevels(ctx context.Context, request *GetAckLevelsRequest) (*GetAckLevelsResponse, error) {
	resp, err := q.persistence.GetAckLevels(ctx, &InternalGetAckLevelsRequest{})
	if err != nil {
		return nil, err
	}
	return &GetAckLevelsResponse{AckLevels: resp.AckLevels}, nil
}

func (q *queueManager) EnqueueMessageToDLQ(ctx context.Context, request *EnqueueMessageToDLQRequest) error {
	currentTimestamp := q.timeSrc.Now()
	return q.persistence.EnqueueMessageToDLQ(ctx, &InternalEnqueueMessageToDLQRequest{
		MessagePayload:   request.MessagePayload,
		CurrentTimeStamp: currentTimestamp,
	})
}

func (q *queueManager) ReadMessagesFromDLQ(ctx context.Context, request *ReadMessagesFromDLQRequest) (*ReadMessagesFromDLQResponse, error) {
	resp, err := q.persistence.ReadMessagesFromDLQ(ctx, &InternalReadMessagesFromDLQRequest{
		FirstMessageID: request.FirstMessageID,
		LastMessageID:  request.LastMessageID,
		PageSize:       request.PageSize,
		PageToken:      request.PageToken,
	})
	if resp == nil {
		return nil, err
	}
	output := make([]*QueueMessage, 0, len(resp.Messages))
	for _, message := range resp.Messages {
		output = append(output, q.fromInternalQueueMessage(message))
	}
	return &ReadMessagesFromDLQResponse{
		Messages:      output,
		NextPageToken: resp.NextPageToken,
	}, err
}

func (q *queueManager) DeleteMessageFromDLQ(ctx context.Context, request *DeleteMessageFromDLQRequest) error {
	return q.persistence.DeleteMessageFromDLQ(ctx, &InternalDeleteMessageFromDLQRequest{
		MessageID: request.MessageID,
	})
}

func (q *queueManager) RangeDeleteMessagesFromDLQ(ctx context.Context, request *RangeDeleteMessagesFromDLQRequest) error {
	return q.persistence.RangeDeleteMessagesFromDLQ(ctx, &InternalRangeDeleteMessagesFromDLQRequest{
		FirstMessageID: request.FirstMessageID,
		LastMessageID:  request.LastMessageID,
	})
}

func (q *queueManager) UpdateDLQAckLevel(ctx context.Context, request *UpdateDLQAckLevelRequest) error {
	currentTimestamp := q.timeSrc.Now()
	return q.persistence.UpdateDLQAckLevel(ctx, &InternalUpdateDLQAckLevelRequest{
		MessageID:        request.MessageID,
		ClusterName:      request.ClusterName,
		CurrentTimeStamp: currentTimestamp,
	})
}

func (q *queueManager) GetDLQAckLevels(ctx context.Context, request *GetDLQAckLevelsRequest) (*GetDLQAckLevelsResponse, error) {
	resp, err := q.persistence.GetDLQAckLevels(ctx, &InternalGetDLQAckLevelsRequest{})
	if err != nil {
		return nil, err
	}
	return &GetDLQAckLevelsResponse{AckLevels: resp.AckLevels}, nil
}

func (q *queueManager) GetDLQSize(ctx context.Context, request *GetDLQSizeRequest) (*GetDLQSizeResponse, error) {
	resp, err := q.persistence.GetDLQSize(ctx, &InternalGetDLQSizeRequest{})
	if err != nil {
		return nil, err
	}
	return &GetDLQSizeResponse{Size: resp.Size}, nil
}

func (q *queueManager) fromInternalQueueMessage(message *InternalQueueMessage) *QueueMessage {
	return &QueueMessage{
		ID:        message.ID,
		QueueType: message.QueueType,
		Payload:   message.Payload,
	}
}
