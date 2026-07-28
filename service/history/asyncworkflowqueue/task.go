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
	"errors"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/task"
)

// dlqWriteTimeout bounds the DLQ write performed when a task is nacked with a
// terminal error.
const dlqWriteTimeout = 5 * time.Second

type (
	// consumerTask adapts one async workflow queue message to common/task.Task
	// so it can be executed on the host-level task scheduler.
	//
	// Lifecycle contract with the shard consumer:
	//   - Ack (successful execution): the message ID is acked into the shard's
	//     ack manager, advancing the contiguous ack level.
	//   - Nack with an error (corrupt message or retries exhausted): the message
	//     is copied to the DLQ; on DLQ-write success the message ID is acked so
	//     the queue is not blocked. On DLQ-write failure the message is left
	//     unacked: the ack level stalls and the message is redelivered.
	//   - Nack with a nil error (scheduler drain on shutdown): the message is
	//     neither DLQ'd nor acked, so it is redelivered after the shard is
	//     re-acquired.
	consumerTask struct {
		prepared   *consumer.PreparedRequest
		message    *persistence.AsyncWorkflowMessage
		mgr        persistence.AsyncWorkflowQueueManager
		ackMgr     messaging.AckManager
		ctx        context.Context
		timeSource clock.TimeSource
		logger     log.Logger
		scope      metrics.Scope
		state      int32
	}
)

// newConsumerTask creates a task for one decoded queue message. ctx is the
// owning shard consumer's lifetime context: execution attempts and DLQ writes
// stop when the shard is closed.
func newConsumerTask(
	ctx context.Context,
	prepared *consumer.PreparedRequest,
	message *persistence.AsyncWorkflowMessage,
	mgr persistence.AsyncWorkflowQueueManager,
	ackMgr messaging.AckManager,
	timeSource clock.TimeSource,
	logger log.Logger,
	scope metrics.Scope,
) *consumerTask {
	return &consumerTask{
		prepared:   prepared,
		message:    message,
		mgr:        mgr,
		ackMgr:     ackMgr,
		ctx:        ctx,
		timeSource: timeSource,
		logger: logger.WithTags(
			tag.AsyncWFRequestType(prepared.RequestType),
			tag.WorkflowDomainName(prepared.Domain),
			tag.WorkflowID(prepared.WorkflowID),
			tag.Dynamic("message-id", message.MessageID),
		),
		scope: scope.Tagged(metrics.DomainTag(prepared.Domain)),
		state: int32(task.TaskStatePending),
	}
}

// Domain returns the domain of the decoded request; it keys the hierarchical
// scheduler and the per-domain rate limiter.
func (t *consumerTask) Domain() string {
	return t.prepared.Domain
}

// Execute performs a single frontend call attempt. Retries are driven by the
// parallel task processor's retry policy, gated by RetryErr.
func (t *consumerTask) Execute() error {
	runID, err := t.prepared.Invoke(t.ctx)
	if err != nil {
		return err
	}
	t.logger.Info("Processed async workflow request", tag.WorkflowRunID(runID))
	return nil
}

func (t *consumerTask) HandleErr(err error) error {
	return err
}

// RetryErr allows retries only for transient service errors while the shard is
// still owned. Corrupt messages are never retried.
func (t *consumerTask) RetryErr(err error) bool {
	if t.ctx.Err() != nil {
		return false
	}
	if errors.Is(err, consumer.ErrCorruptMessage) {
		return false
	}
	return common.IsServiceTransientError(err)
}

// Ack marks successful completion and advances the shard's ack manager.
func (t *consumerTask) Ack() {
	if !atomic.CompareAndSwapInt32(&t.state, int32(task.TaskStatePending), int32(task.TaskStateAcked)) {
		return
	}
	t.ackMgr.AckItem(t.message.MessageID)
	t.scope.IncCounter(metrics.AsyncWorkflowSuccessCount)
}

// Nack handles terminal failure. A nil error means the scheduler is draining on
// shutdown: the task is neither DLQ'd nor acked, so the message is redelivered
// later. A non-nil error means the message is poison or retries are exhausted:
// it is copied to the DLQ and, if that write succeeds, acked past.
func (t *consumerTask) Nack(err error) {
	if !atomic.CompareAndSwapInt32(&t.state, int32(task.TaskStatePending), int32(task.TaskStateCanceled)) {
		return
	}
	if err == nil {
		t.logger.Info("Async workflow task dropped on shutdown, message will be redelivered")
		return
	}
	if t.ctx.Err() != nil {
		// The shard consumer is gone; retries may have been cut short by
		// shutdown rather than exhausted. Don't DLQ — redeliver instead.
		t.logger.Info("Async workflow task abandoned after shard close, message will be redelivered", tag.Error(err))
		return
	}

	t.logger.Error("Async workflow task failed terminally, moving message to DLQ", tag.Error(err))
	t.scope.IncCounter(metrics.AsyncWorkflowFailureByFrontendCount)

	ctx, cancel := context.WithTimeout(t.ctx, dlqWriteTimeout)
	defer cancel()
	if _, dlqErr := t.mgr.EnqueueToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageRequest{
		ShardID:          t.message.ShardID,
		Payload:          t.message.Payload,
		Encoding:         t.message.Encoding,
		PartitionKey:     t.message.PartitionKey,
		CurrentTimeStamp: t.timeSource.Now(),
	}); dlqErr != nil {
		// Leaving the message unacked stalls the shard's ack level, which
		// surfaces via the consumer backlog gauge; the message is redelivered.
		t.logger.Error("Failed to move async workflow message to DLQ, message stays unacked", tag.Error(dlqErr))
		return
	}

	t.ackMgr.AckItem(t.message.MessageID)
}

func (t *consumerTask) Cancel() {
	atomic.CompareAndSwapInt32(&t.state, int32(task.TaskStatePending), int32(task.TaskStateCanceled))
}

func (t *consumerTask) State() task.State {
	return task.State(atomic.LoadInt32(&t.state))
}
