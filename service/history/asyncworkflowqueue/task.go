package asyncworkflowqueue

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/task"
)

const dlqWriteTimeout = 5 * time.Second

// TaskParams are the dependencies of a single async workflow queue message task.
type TaskParams struct {
	Message     *persistence.AsyncWorkflowMessage
	Decoder     Decoder
	Processor   Processor
	Manager     persistence.AsyncWorkflowQueueManager
	AckMgr      messaging.AckManager
	RetryPolicy backoff.RetryPolicy // common.CreateFrontendServiceRetryPolicy()
	TimeSource  clock.TimeSource
	Scope       metrics.Scope
	Logger      log.Logger
}

type asyncWorkflowTask struct {
	//nolint:containedctx // task.Task has no ctx parameter, so the shard lifecycle ctx must be stored
	ctx   context.Context
	p     TaskParams
	state atomic.Int32
}

// NewTask creates a task that decodes and processes one message, writing it to the DLQ before
// acking when it cannot be handled. ctx is the shard lifecycle context.
func NewTask(ctx context.Context, p TaskParams) task.Task {
	t := &asyncWorkflowTask{ctx: ctx, p: p}
	t.state.Store(int32(task.TaskStatePending))
	return t
}

func (t *asyncWorkflowTask) Execute() error {
	if err := t.ctx.Err(); err != nil {
		return err
	}

	// Decode is inside the retried closure because it resolves the domain ID, which can fail transiently.
	// ThrottleRetry.Do prefers the previous attempt's error over the one that ended the retry, so the
	// final attempt's error is tracked here: it is what Nack records as the DLQ reason.
	var lastErr error
	err := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(t.p.RetryPolicy),
		backoff.WithRetryableError(common.IsServiceTransientError),
		backoff.WithClock(t.p.TimeSource),
	).Do(t.ctx, func(ctx context.Context) error {
		req, err := t.p.Decoder.Decode(t.p.Message)
		if err == nil {
			err = t.p.Processor.Process(ctx, req)
		}
		lastErr = err
		return err
	})
	if err != nil && lastErr != nil {
		err = lastErr
	}
	if errors.Is(err, ErrCorruptMessage) {
		t.p.Scope.IncCounter(metrics.AsyncWorkflowQueueDecodeFailuresCounter)
	}
	return err
}

func (t *asyncWorkflowTask) HandleErr(err error) error { return err }

func (t *asyncWorkflowTask) RetryErr(error) bool { return false }

func (t *asyncWorkflowTask) Ack() {
	if !t.state.CompareAndSwap(int32(task.TaskStatePending), int32(task.TaskStateAcked)) {
		return
	}
	t.p.AckMgr.AckItem(t.p.Message.MessageID)
}

func (t *asyncWorkflowTask) Nack(err error) {
	if !t.state.CompareAndSwap(int32(task.TaskStatePending), int32(task.TaskStateCanceled)) {
		return
	}

	if err == nil {
		t.p.Logger.Info("async workflow queue task stopped before completion, leaving message unacked", t.tags()...)
		return
	}
	if t.ctx.Err() != nil {
		t.p.Logger.Info("shard stopping, leaving async workflow queue message unacked", append(t.tags(), tag.Error(err))...)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), dlqWriteTimeout)
	defer cancel()
	dlqErr := t.p.Manager.EnqueueAsyncWorkflowMessageToDLQ(ctx, &persistence.EnqueueAsyncWorkflowMessageToDLQRequest{
		Message: t.p.Message,
		Reason:  err.Error(),
	})
	if dlqErr != nil {
		t.p.Logger.Error("failed to write async workflow queue message to DLQ, leaving it unacked", append(t.tags(), tag.Error(dlqErr))...)
		t.p.Scope.IncCounter(metrics.AsyncWorkflowQueueDLQFailuresCounter)
		return
	}

	t.p.Scope.IncCounter(metrics.AsyncWorkflowQueueDLQCounter)
	t.p.Logger.Warn("async workflow queue message written to DLQ", append(t.tags(), tag.Error(err))...)
	t.p.AckMgr.AckItem(t.p.Message.MessageID)
}

func (t *asyncWorkflowTask) Cancel() {
	t.state.CompareAndSwap(int32(task.TaskStatePending), int32(task.TaskStateCanceled))
}

func (t *asyncWorkflowTask) State() task.State {
	return task.State(t.state.Load())
}

func (t *asyncWorkflowTask) tags() []tag.Tag {
	return []tag.Tag{
		tag.ShardID(t.p.Message.ShardID),
		tag.WorkflowDomainName(t.p.Message.DomainName),
		tag.WorkflowID(t.p.Message.WorkflowID),
		tag.Dynamic("message-id", t.p.Message.MessageID),
		tag.Dynamic("source-cluster", t.p.Message.SourceCluster),
	}
}
