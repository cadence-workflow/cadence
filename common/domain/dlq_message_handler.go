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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination dlq_message_handler_mock.go

package domain

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

type (
	// DLQMessageHandler is the interface handles domain DLQ messages
	DLQMessageHandler interface {
		common.Daemon

		Count(ctx context.Context, forceFetch bool) (int64, error)
		Read(ctx context.Context, lastMessageID int64, pageSize int, pageToken []byte) ([]*types.ReplicationTask, []byte, error)
		Purge(ctx context.Context, lastMessageID int64) error
		Merge(ctx context.Context, lastMessageID int64, pageSize int, pageToken []byte) ([]byte, error)
	}

	dlqMessageHandlerImpl struct {
		replicationHandler          ReplicationTaskExecutor
		semaphoreReplicationHandler SemaphoreReplicationTaskExecutor
		replicationQueue            ReplicationQueue
		logger                      log.Logger
		metricsClient               metrics.Client
		timeSrc                     clock.TimeSource
		done                        chan struct{}
		status                      int32

		mu        sync.Mutex
		lastCount int64
	}
)

// NewDLQMessageHandler returns a DLQTaskHandler instance
func NewDLQMessageHandler(
	replicationHandler ReplicationTaskExecutor,
	semaphoreReplicationHandler SemaphoreReplicationTaskExecutor,
	replicationQueue ReplicationQueue,
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
) DLQMessageHandler {
	return &dlqMessageHandlerImpl{
		replicationHandler:          replicationHandler,
		semaphoreReplicationHandler: semaphoreReplicationHandler,
		replicationQueue:            replicationQueue,
		logger:                      logger,
		metricsClient:               metricsClient,
		timeSrc:                     timeSource,
		done:                        make(chan struct{}),
		lastCount:                   -1,
	}
}

// Start starts the DLQ handler
func (d *dlqMessageHandlerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&d.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	go d.emitDLQSizeMetricsLoop()
	d.logger.Info("Domain DLQ handler started.")
}

// Stop stops the DLQ handler
func (d *dlqMessageHandlerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&d.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	d.logger.Info("Domain DLQ handler shutting down.")
	close(d.done)
}

// Count counts domain replication DLQ messages
func (d *dlqMessageHandlerImpl) Count(ctx context.Context, forceFetch bool) (int64, error) {
	if forceFetch || d.lastCount == -1 {
		if err := d.fetchAndEmitDLQSize(ctx); err != nil {
			return 0, err
		}
	}
	return d.lastCount, nil
}

// ReadMessages reads domain replication DLQ messages
func (d *dlqMessageHandlerImpl) Read(
	ctx context.Context,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []byte, error) {

	ackLevel, err := d.replicationQueue.GetDLQAckLevel(ctx)
	if err != nil {
		return nil, nil, err
	}

	return d.replicationQueue.GetMessagesFromDLQ(
		ctx,
		ackLevel,
		lastMessageID,
		pageSize,
		pageToken,
	)
}

// PurgeMessages purges domain replication DLQ messages
func (d *dlqMessageHandlerImpl) Purge(
	ctx context.Context,
	lastMessageID int64,
) error {

	ackLevel, err := d.replicationQueue.GetDLQAckLevel(ctx)
	if err != nil {
		return err
	}

	if err := d.replicationQueue.RangeDeleteMessagesFromDLQ(
		ctx,
		ackLevel,
		lastMessageID,
	); err != nil {
		return err
	}

	if err := d.replicationQueue.UpdateDLQAckLevel(
		ctx,
		lastMessageID,
	); err != nil {
		d.logger.Error("Failed to update DLQ ack level after purging messages", tag.Error(err))
	}

	return nil
}

// MergeMessages merges domain replication DLQ messages
func (d *dlqMessageHandlerImpl) Merge(
	ctx context.Context,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]byte, error) {

	ackLevel, err := d.replicationQueue.GetDLQAckLevel(ctx)
	if err != nil {
		return nil, err
	}

	messages, token, err := d.replicationQueue.GetMessagesFromDLQ(
		ctx,
		ackLevel,
		lastMessageID,
		pageSize,
		pageToken,
	)
	if err != nil {
		return nil, err
	}

	var ackedMessageID int64
	for _, message := range messages {
		switch message.GetTaskType() {
		case types.ReplicationTaskTypeSemaphoreMetadata:
			err = d.semaphoreReplicationHandler.Execute(message.GetSemaphoreMetadataTaskAttributes())
		default:
			domainTask := message.GetDomainTaskAttributes()
			if domainTask == nil {
				return nil, &types.InternalServiceError{Message: "Encounter non domain replication task in domain replication queue."}
			}
			err = d.replicationHandler.Execute(domainTask)
		}
		if err != nil {
			return nil, err
		}
		ackedMessageID = message.SourceTaskID
	}

	if err := d.replicationQueue.RangeDeleteMessagesFromDLQ(
		ctx,
		ackLevel,
		ackedMessageID,
	); err != nil {
		d.logger.Error("failed to delete merged tasks on merging domain DLQ message", tag.Error(err))
		return nil, err
	}
	if err := d.replicationQueue.UpdateDLQAckLevel(ctx, ackedMessageID); err != nil {
		d.logger.Error("failed to update ack level on merging domain DLQ message", tag.Error(err))
	}

	return token, nil
}

func (d *dlqMessageHandlerImpl) emitDLQSizeMetricsLoop() {
	ticker := d.timeSrc.NewTicker(queueSizeQueryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.done:
			return
		case <-ticker.Chan():
			err := d.fetchAndEmitDLQSize(context.Background())
			if err != nil {
				d.logger.Warn("Failed to get DLQ size.", tag.Error(err))
			}
		}
	}
}

func (d *dlqMessageHandlerImpl) fetchAndEmitDLQSize(ctx context.Context) error {
	size, err := d.replicationQueue.GetDLQSize(ctx)
	if err != nil {
		d.metricsClient.Scope(metrics.DomainReplicationQueueScope).IncCounter(metrics.DomainReplicationQueueSizeErrorCount)
		return err
	}

	d.metricsClient.Scope(metrics.DomainReplicationQueueScope).UpdateGauge(metrics.DomainReplicationQueueSizeGauge, float64(size))

	d.mu.Lock()
	d.lastCount = size
	d.mu.Unlock()

	return nil
}
