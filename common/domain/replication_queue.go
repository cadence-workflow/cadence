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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination replication_queue_mock.go -self_package github.com/uber/common/persistence

package domain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/.gen/go/replicator"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

const (
	purgeInterval                 = 5 * time.Minute
	queueSizeQueryInterval        = 5 * time.Minute
	localDomainReplicationCluster = "domainReplication"
)

var _ ReplicationQueue = (*replicationQueueImpl)(nil)

// NewReplicationQueue creates a new ReplicationQueue instance
func NewReplicationQueue(
	queue persistence.QueueManager,
	clusterName string,
	metricsClient metrics.Client,
	logger log.Logger,
) ReplicationQueue {
	return &replicationQueueImpl{
		queue:         queue,
		clusterName:   clusterName,
		metricsClient: metricsClient,
		logger:        logger,
		encoder:       codec.NewThriftRWEncoder(),
		done:          make(chan bool),
		status:        common.DaemonStatusInitialized,
	}
}

type (
	replicationQueueImpl struct {
		queue         persistence.QueueManager
		clusterName   string
		metricsClient metrics.Client
		logger        log.Logger
		encoder       codec.BinaryEncoder
		done          chan bool
		status        int32
	}

	// ReplicationQueue is used to publish and list domain replication tasks
	ReplicationQueue interface {
		common.Daemon
		Publish(ctx context.Context, message interface{}) error
		PublishToDLQ(ctx context.Context, message interface{}) error
		GetReplicationMessages(ctx context.Context, lastMessageID int64, maxCount int) ([]*types.ReplicationTask, int64, error)
		UpdateAckLevel(ctx context.Context, lastProcessedMessageID int64, clusterName string) error
		GetAckLevels(ctx context.Context) (map[string]int64, error)
		GetMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64, pageSize int, pageToken []byte) ([]*types.ReplicationTask, []byte, error)
		UpdateDLQAckLevel(ctx context.Context, lastProcessedMessageID int64) error
		GetDLQAckLevel(ctx context.Context) (int64, error)
		RangeDeleteMessagesFromDLQ(ctx context.Context, firstMessageID int64, lastMessageID int64) error
		DeleteMessageFromDLQ(ctx context.Context, messageID int64) error
		GetDLQSize(ctx context.Context) (int64, error)
	}
)

func (q *replicationQueueImpl) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	go q.purgeProcessor()
}

func (q *replicationQueueImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	close(q.done)
}

func (q *replicationQueueImpl) Publish(
	ctx context.Context,
	message interface{},
) error {
	task, ok := message.(*types.ReplicationTask)
	if !ok {
		return errors.New("wrong message type")
	}

	bytes, err := q.encoder.Encode(thrift.FromReplicationTask(task))
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}
	return q.queue.EnqueueMessage(ctx, &persistence.EnqueueMessageRequest{
		MessagePayload: bytes,
	})
}

func (q *replicationQueueImpl) PublishToDLQ(
	ctx context.Context,
	message interface{},
) error {
	task, ok := message.(*types.ReplicationTask)
	if !ok {
		return errors.New("wrong message type")
	}

	bytes, err := q.encoder.Encode(thrift.FromReplicationTask(task))
	if err != nil {
		return fmt.Errorf("failed to encode message: %v", err)
	}

	return q.queue.EnqueueMessageToDLQ(ctx, &persistence.EnqueueMessageToDLQRequest{
		MessagePayload: bytes,
	})
}

func (q *replicationQueueImpl) GetReplicationMessages(
	ctx context.Context,
	lastMessageID int64,
	maxCount int,
) ([]*types.ReplicationTask, int64, error) {

	resp, err := q.queue.ReadMessages(ctx, &persistence.ReadMessagesRequest{
		LastMessageID: lastMessageID,
		MaxCount:      maxCount,
	})
	if err != nil {
		return nil, lastMessageID, err
	}

	var replicationTasks []*types.ReplicationTask
	for _, message := range resp.Messages {
		var replicationTask replicator.ReplicationTask
		err := q.encoder.Decode(message.Payload, &replicationTask)
		if err != nil {
			return nil, lastMessageID, fmt.Errorf("failed to decode task: %v", err)
		}

		lastMessageID = message.ID
		replicationTasks = append(replicationTasks, thrift.ToReplicationTask(&replicationTask))
	}

	q.logger.Debugf("Returning %d domain replication tasks. lastMessageID: %d", len(replicationTasks), lastMessageID)
	return replicationTasks, lastMessageID, nil
}

func (q *replicationQueueImpl) UpdateAckLevel(
	ctx context.Context,
	lastProcessedMessageID int64,
	clusterName string,
) error {

	if err := q.queue.UpdateAckLevel(ctx, &persistence.UpdateAckLevelRequest{
		MessageID:   lastProcessedMessageID,
		ClusterName: clusterName,
	}); err != nil {
		return fmt.Errorf("failed to update ack level: %v", err)
	}

	return nil
}

func (q *replicationQueueImpl) GetAckLevels(
	ctx context.Context,
) (map[string]int64, error) {
	resp, err := q.queue.GetAckLevels(ctx, &persistence.GetAckLevelsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.AckLevels, nil
}

func (q *replicationQueueImpl) GetMessagesFromDLQ(
	ctx context.Context,
	firstMessageID int64,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []byte, error) {

	resp, err := q.queue.ReadMessagesFromDLQ(ctx, &persistence.ReadMessagesFromDLQRequest{
		FirstMessageID: firstMessageID,
		LastMessageID:  lastMessageID,
		PageSize:       pageSize,
		PageToken:      pageToken,
	})
	if err != nil {
		return nil, nil, err
	}

	var replicationTasks []*types.ReplicationTask
	for _, message := range resp.Messages {
		var replicationTask replicator.ReplicationTask
		err := q.encoder.Decode(message.Payload, &replicationTask)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode dlq task: %v", err)
		}

		// Overwrite to local cluster message id
		replicationTask.SourceTaskId = common.Int64Ptr(int64(message.ID))
		replicationTasks = append(replicationTasks, thrift.ToReplicationTask(&replicationTask))
	}

	return replicationTasks, resp.NextPageToken, nil
}

func (q *replicationQueueImpl) UpdateDLQAckLevel(
	ctx context.Context,
	lastProcessedMessageID int64,
) error {
	return q.queue.UpdateDLQAckLevel(ctx, &persistence.UpdateDLQAckLevelRequest{
		MessageID:   lastProcessedMessageID,
		ClusterName: localDomainReplicationCluster,
	})
}

func (q *replicationQueueImpl) GetDLQAckLevel(
	ctx context.Context,
) (int64, error) {
	dlqMetadataResp, err := q.queue.GetDLQAckLevels(ctx, &persistence.GetDLQAckLevelsRequest{})
	if err != nil {
		return constants.EmptyMessageID, err
	}

	ackLevel, ok := dlqMetadataResp.AckLevels[localDomainReplicationCluster]
	if !ok {
		return constants.EmptyMessageID, nil
	}
	return ackLevel, nil
}

func (q *replicationQueueImpl) RangeDeleteMessagesFromDLQ(
	ctx context.Context,
	firstMessageID int64,
	lastMessageID int64,
) error {
	return q.queue.RangeDeleteMessagesFromDLQ(ctx, &persistence.RangeDeleteMessagesFromDLQRequest{
		FirstMessageID: firstMessageID,
		LastMessageID:  lastMessageID,
	})
}

func (q *replicationQueueImpl) DeleteMessageFromDLQ(
	ctx context.Context,
	messageID int64,
) error {

	return q.queue.DeleteMessageFromDLQ(ctx, &persistence.DeleteMessageFromDLQRequest{
		MessageID: messageID,
	})
}

func (q *replicationQueueImpl) GetDLQSize(ctx context.Context) (int64, error) {
	resp, err := q.queue.GetDLQSize(ctx, &persistence.GetDLQSizeRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Size, nil
}

func (q *replicationQueueImpl) purgeAckedMessages() error {
	ackLevelByCluster, err := q.GetAckLevels(context.Background())
	if err != nil {
		return fmt.Errorf("failed to purge messages: %v", err)
	}

	if len(ackLevelByCluster) == 0 {
		return nil
	}

	minAckLevel := int64(math.MaxInt64)
	for _, ackLevel := range ackLevelByCluster {
		if ackLevel < minAckLevel {
			minAckLevel = ackLevel
		}
	}

	if minAckLevel != int64(math.MaxInt64) {
		if err = q.queue.DeleteMessagesBefore(context.Background(), &persistence.DeleteMessagesBeforeRequest{
			MessageID: minAckLevel,
		}); err != nil {
			return fmt.Errorf("failed to purge messages: %v", err)
		}
	}
	return nil
}

func (q *replicationQueueImpl) purgeProcessor() {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.done:
			return
		case <-ticker.C:
			if err := q.purgeAckedMessages(); err != nil {
				q.logger.Warn("Failed to purge acked domain replication messages.", tag.Error(err))
			}
		}
	}
}
