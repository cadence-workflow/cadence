// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package asyncworkflowqueue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

// frontendCallTimeout bounds each frontend call attempt made when executing an
// async workflow request. It matches the Kafka DefaultConsumer's per-attempt
// timeout.
const frontendCallTimeout = 3 * time.Second

type (
	// Consumer is the per-shard daemon that reads async workflow queue messages,
	// decodes them, and submits them to the host-level TaskScheduler. It owns the
	// shard's in-memory ack manager and periodically commits the contiguous ack
	// level so GC can reclaim processed messages.
	Consumer interface {
		common.Daemon
	}

	// ConsumerParams are the dependencies needed to build a Consumer.
	ConsumerParams struct {
		ShardID          int
		Manager          persistence.AsyncWorkflowQueueManager
		Scheduler        TaskScheduler
		RequestProcessor *consumer.RequestProcessor
		Enabled          dynamicproperties.BoolPropertyFnWithShardIDFilter
		PollInterval     dynamicproperties.DurationPropertyFnWithShardIDFilter
		CommitInterval   dynamicproperties.DurationPropertyFnWithShardIDFilter
		PageSize         dynamicproperties.IntPropertyFnWithShardIDFilter
		TimeSource       clock.TimeSource
		Metrics          metrics.Client
		Logger           log.Logger
	}

	consumerImpl struct {
		shardID      int
		mgr          persistence.AsyncWorkflowQueueManager
		scheduler    TaskScheduler
		reqProcessor *consumer.RequestProcessor
		enabled      dynamicproperties.BoolPropertyFnWithShardIDFilter
		pollInterval dynamicproperties.DurationPropertyFnWithShardIDFilter
		commitEvery  dynamicproperties.DurationPropertyFnWithShardIDFilter
		pageSize     dynamicproperties.IntPropertyFnWithShardIDFilter
		timeSource   clock.TimeSource
		scope        metrics.Scope
		logger       log.Logger

		status int32
		ctx    context.Context
		cancel context.CancelFunc
		wg     sync.WaitGroup

		// The following fields are owned by the single consumeLoop goroutine.
		initialized   bool
		cursor        int64
		ackMgr        messaging.AckManager
		lastCommitted int64
	}
)

var _ Consumer = (*consumerImpl)(nil)

// NewConsumer creates a Consumer from the given dependencies.
func NewConsumer(params ConsumerParams) Consumer {
	return &consumerImpl{
		shardID:      params.ShardID,
		mgr:          params.Manager,
		scheduler:    params.Scheduler,
		reqProcessor: params.RequestProcessor,
		enabled:      params.Enabled,
		pollInterval: params.PollInterval,
		commitEvery:  params.CommitInterval,
		pageSize:     params.PageSize,
		timeSource:   params.TimeSource,
		scope:        params.Metrics.Scope(metrics.AsyncWorkflowQueueConsumerScope),
		logger:       params.Logger.WithTags(tag.ShardID(params.ShardID)),
		status:       common.DaemonStatusInitialized,
		cancel:       func() {}, // no-op until Start() sets the real cancel
	}
}

// NewConsumerFromShard is a convenience constructor that derives the
// shard-scoped dependencies from the shard context and the host-level
// scheduler, and delegates to NewConsumer.
func NewConsumerFromShard(
	shardCtx shard.Context,
	scheduler TaskScheduler,
	enabled dynamicproperties.BoolPropertyFnWithShardIDFilter,
	pollInterval dynamicproperties.DurationPropertyFnWithShardIDFilter,
	commitInterval dynamicproperties.DurationPropertyFnWithShardIDFilter,
	pageSize dynamicproperties.IntPropertyFnWithShardIDFilter,
) Consumer {
	return NewConsumer(ConsumerParams{
		ShardID:   shardCtx.GetShardID(),
		Manager:   shardCtx.GetService().GetAsyncWorkflowQueueManager(),
		Scheduler: scheduler,
		RequestProcessor: consumer.NewRequestProcessor(
			shardCtx.GetService().GetFrontendClient(),
			frontendCallTimeout,
			shardCtx.GetLogger(),
		),
		Enabled:        enabled,
		PollInterval:   pollInterval,
		CommitInterval: commitInterval,
		PageSize:       pageSize,
		TimeSource:     shardCtx.GetTimeSource(),
		Metrics:        shardCtx.GetMetricsClient(),
		Logger:         shardCtx.GetLogger(),
	})
}

// Start launches the background consume loop.
func (c *consumerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&c.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.logger.Debug("Async workflow queue consumer starting")
	c.wg.Add(1)
	go c.consumeLoop()
	c.logger.Debug("Async workflow queue consumer started")
}

// Stop signals the loop to exit and waits for it. A final best-effort ack level
// commit runs before the loop returns. Idempotent.
func (c *consumerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&c.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	c.logger.Debug("Async workflow queue consumer stopping")
	c.cancel()
	c.wg.Wait()
	c.logger.Debug("Async workflow queue consumer stopped")
}

// consumeLoop is the single background goroutine: it polls for messages, and on
// a slower cadence commits the ack level. Both intervals are re-read every tick
// so dynamic config changes take effect without a restart.
func (c *consumerImpl) consumeLoop() {
	defer c.wg.Done()
	defer func() { log.CapturePanic(recover(), c.logger, nil) }()

	pollTimer := c.timeSource.NewTimer(c.pollInterval(c.shardID))
	defer pollTimer.Stop()
	commitTimer := c.timeSource.NewTimer(c.commitEvery(c.shardID))
	defer commitTimer.Stop()

	for {
		select {
		case <-c.ctx.Done():
			// Final best-effort commit so redelivery after shard movement is
			// minimized. Use a fresh context: c.ctx is already canceled.
			ctx, cancel := context.WithTimeout(context.Background(), dlqWriteTimeout)
			c.commit(ctx)
			cancel()
			return
		case <-pollTimer.Chan():
			if c.enabled(c.shardID) {
				c.poll()
			}
			pollTimer.Reset(c.pollInterval(c.shardID))
		case <-commitTimer.Chan():
			if c.initialized {
				c.commit(c.ctx)
			}
			commitTimer.Reset(c.commitEvery(c.shardID))
		}
	}
}

// poll seeds the cursor from the committed ack level on first run, then reads
// and dispatches pages of messages until a short page or shutdown.
func (c *consumerImpl) poll() {
	if !c.initialized {
		resp, err := c.mgr.GetAckLevel(c.ctx, &persistence.GetAsyncWorkflowAckLevelRequest{
			ShardID: c.shardID,
		})
		if err != nil {
			c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerPollFailuresCounter)
			c.logger.Error("failed to get async workflow queue ack level", tag.Error(err))
			return
		}
		c.cursor = resp.AckLevel
		c.lastCommitted = resp.AckLevel
		c.ackMgr = messaging.NewContinuousAckManager(c.logger)
		c.ackMgr.SetAckLevel(resp.AckLevel)
		c.initialized = true
	}

	pageSize := c.pageSize(c.shardID)
	for {
		resp, err := c.mgr.ReadMessages(c.ctx, &persistence.ReadAsyncWorkflowMessagesRequest{
			ShardID:       c.shardID,
			LastMessageID: c.cursor,
			PageSize:      pageSize,
		})
		if err != nil {
			c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerPollFailuresCounter)
			c.logger.Error("failed to read async workflow queue messages", tag.Error(err))
			return
		}

		for _, msg := range resp.Messages {
			if c.ctx.Err() != nil {
				return
			}
			if !c.dispatch(msg) {
				// The message was neither submitted nor terminally handled;
				// leave the cursor on the previous message so it is re-read.
				return
			}
			c.cursor = msg.MessageID
		}
		c.scope.UpdateGauge(metrics.AsyncWorkflowQueueConsumerBacklogGauge, float64(c.ackMgr.GetBacklogCount()))

		if len(resp.Messages) < pageSize {
			return
		}
	}
}

// dispatch decodes one message and submits it to the scheduler. Undecodable
// messages go straight to the DLQ and are acked past. It returns false only if
// the message could not be handled at all and must be re-read later.
func (c *consumerImpl) dispatch(msg *persistence.AsyncWorkflowMessage) bool {
	c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerFetchedCounter)

	prepared, decodeErr := c.decode(msg)
	if decodeErr != nil {
		c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerDecodeFailuresCounter)
		c.logger.Error("failed to decode async workflow queue message, moving to DLQ",
			tag.Dynamic("message-id", msg.MessageID), tag.Error(decodeErr))
		return c.moveToDLQ(msg)
	}

	if err := c.ackMgr.ReadItem(msg.MessageID); err != nil {
		// Should not happen: messages are read in increasing message ID order.
		c.logger.Error("failed to track async workflow queue message",
			tag.Dynamic("message-id", msg.MessageID), tag.Error(err))
		return false
	}

	t := newConsumerTask(c.ctx, prepared, msg, c.mgr, c.ackMgr, c.timeSource, c.logger, c.scope)
	if err := c.scheduler.Submit(t); err != nil {
		// Scheduler is shutting down; the message stays unacked and is
		// redelivered after the shard is re-acquired.
		c.logger.Warn("failed to submit async workflow task to scheduler",
			tag.Dynamic("message-id", msg.MessageID), tag.Error(err))
		return false
	}
	c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerSubmittedCounter)
	return true
}

func (c *consumerImpl) decode(msg *persistence.AsyncWorkflowMessage) (*consumer.PreparedRequest, error) {
	env, err := c.reqProcessor.DecodeEnvelope(msg.Payload)
	if err != nil {
		return nil, err
	}
	return c.reqProcessor.Prepare(env)
}

// moveToDLQ copies an undecodable message to the DLQ and acks past it so the
// queue is not blocked. On DLQ-write failure the message is re-read next poll.
func (c *consumerImpl) moveToDLQ(msg *persistence.AsyncWorkflowMessage) bool {
	ctx, cancel := context.WithTimeout(c.ctx, dlqWriteTimeout)
	defer cancel()
	if _, err := c.mgr.EnqueueToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageRequest{
		ShardID:          msg.ShardID,
		Payload:          msg.Payload,
		Encoding:         msg.Encoding,
		PartitionKey:     msg.PartitionKey,
		CurrentTimeStamp: c.timeSource.Now(),
	}); err != nil {
		c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerDLQFailuresCounter)
		c.logger.Error("failed to move async workflow queue message to DLQ",
			tag.Dynamic("message-id", msg.MessageID), tag.Error(err))
		return false
	}
	c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerDLQCounter)

	if err := c.ackMgr.ReadItem(msg.MessageID); err != nil {
		c.logger.Error("failed to track async workflow queue message",
			tag.Dynamic("message-id", msg.MessageID), tag.Error(err))
		return false
	}
	c.ackMgr.AckItem(msg.MessageID)
	return true
}

// commit persists the ack manager's contiguous ack level if it advanced since
// the last commit. Failures (including CAS conflicts from a new shard owner)
// are logged and counted; the next tick retries.
func (c *consumerImpl) commit(ctx context.Context) {
	if !c.initialized {
		return
	}
	ackLevel := c.ackMgr.GetAckLevel()
	if ackLevel <= c.lastCommitted {
		return
	}
	if err := c.mgr.UpdateAckLevel(ctx, &persistence.UpdateAsyncWorkflowAckLevelRequest{
		ShardID:          c.shardID,
		AckLevel:         ackLevel,
		CurrentTimeStamp: c.timeSource.Now(),
	}); err != nil {
		c.scope.IncCounter(metrics.AsyncWorkflowQueueConsumerCommitFailuresCounter)
		c.logger.Error("failed to commit async workflow queue ack level",
			tag.Dynamic("ack-level", ackLevel), tag.Error(err))
		return
	}
	c.lastCommitted = ackLevel
	c.scope.UpdateGauge(metrics.AsyncWorkflowQueueConsumerBacklogGauge, float64(c.ackMgr.GetBacklogCount()))
}
