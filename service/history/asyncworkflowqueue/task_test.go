package asyncworkflowqueue

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/common/types"
)

func testMessage() *persistence.AsyncWorkflowMessage {
	return &persistence.AsyncWorkflowMessage{
		ShardID:       1,
		SourceCluster: "cluster0",
		MessageID:     5,
		DomainName:    "test-domain",
		WorkflowID:    "test-wf",
		RequestID:     "test-request",
	}
}

// executeWithClock runs tk.Execute in a goroutine and, each time the retrier is parked waiting on the
// mocked clock (BlockUntil(1)), calls onBlocked, until Execute returns. Retry backoffs inside
// backoff.ThrottleRetry therefore make progress deterministically, without real sleeps.
func executeWithClock(ts clock.MockedTimeSource, tk ctask.Task, onBlocked func()) error {
	done := make(chan error, 1)
	go func() { done <- tk.Execute() }()
	for {
		blocked := make(chan struct{})
		go func() {
			ts.BlockUntil(1)
			close(blocked)
		}()
		select {
		case err := <-done:
			// Register a waiter so a still-pending BlockUntil helper above returns instead of leaking.
			ts.After(time.Hour)
			return err
		case <-blocked:
			onBlocked()
		}
	}
}

func TestTask(t *testing.T) {
	transientErr := &types.InternalServiceError{Message: "transient"}
	nonTransientErr := &types.BadRequestError{Message: "bad request"}
	notFoundErr := &types.EntityNotExistsError{Message: "not found"}
	dlqWriteErr := errors.New("dlq write failed")
	decodeErr := fmt.Errorf("bad payload: %w", ErrCorruptMessage)

	const (
		actionRun        = "run"         // mimic ParallelTaskProcessor: Execute -> HandleErr -> Ack or Nack(err)
		actionNackNil    = "nack-nil"    // processor drained/stopped: Nack(nil)
		actionNackCtx    = "nack-ctx"    // shard ctx cancelled, then Nack(some error)
		actionNackTwice  = "nack-twice"  // Ack, then a late Nack and Cancel must be no-ops
		actionCancelWait = "cancel-wait" // shard ctx cancelled while the retrier waits on the clock, then Nack(err)
	)

	tests := []struct {
		name   string
		action string
		// decodeResults are returned by successive Decode calls (nil = a valid request); the last repeats
		// forever. Empty means Decode always succeeds.
		decodeResults []error
		// processResults are returned by successive Process calls; the last repeats forever.
		processResults []error
		// expectProcess says whether Process must be called at all (corrupt rows must not reach it).
		expectProcess bool
		// dlqErr is returned by EnqueueAsyncWorkflowMessageToDLQ when it is called.
		dlqErr error

		wantMinProcessCalls int
		wantExactProcess    int // 0 = only the minimum is checked
		wantDLQ             bool
		wantReason          string // "" = no reason check
		wantAcked           bool   // AckMgr ack level advanced to the message id
		wantState           ctask.State
		wantDLQCounter      int64
		wantDLQFailCounter  int64
		wantDecodeFailCtr   int64
	}{
		{
			name:                "success acks without DLQ",
			action:              actionRun,
			processResults:      []error{nil},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantAcked:           true,
			wantState:           ctask.TaskStateAcked,
		},
		{
			// WorkflowExecutionAlreadyStarted is turned into nil inside Processor, so Process returns nil.
			name:                "process returns nil for already-started acks without DLQ",
			action:              actionRun,
			processResults:      []error{nil},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantAcked:           true,
			wantState:           ctask.TaskStateAcked,
		},
		{
			name:                "transient error then success on second attempt acks without DLQ",
			action:              actionRun,
			processResults:      []error{transientErr, nil},
			expectProcess:       true,
			wantMinProcessCalls: 2,
			wantExactProcess:    2,
			wantAcked:           true,
			wantState:           ctask.TaskStateAcked,
		},
		{
			name:                "transient until retry expiry writes DLQ then acks",
			action:              actionRun,
			processResults:      []error{transientErr},
			expectProcess:       true,
			wantMinProcessCalls: 2,
			wantDLQ:             true,
			wantReason:          transientErr.Error(),
			wantAcked:           true,
			wantState:           ctask.TaskStateCanceled,
			wantDLQCounter:      1,
		},
		{
			name:                "transient then non-transient writes DLQ with the final error as the reason",
			action:              actionRun,
			processResults:      []error{transientErr, nonTransientErr},
			expectProcess:       true,
			wantMinProcessCalls: 2,
			wantExactProcess:    2,
			wantDLQ:             true,
			wantReason:          nonTransientErr.Error(),
			wantAcked:           true,
			wantState:           ctask.TaskStateCanceled,
			wantDLQCounter:      1,
		},
		{
			name:                "non-transient error writes DLQ immediately then acks",
			action:              actionRun,
			processResults:      []error{nonTransientErr},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantDLQ:             true,
			wantReason:          nonTransientErr.Error(),
			wantAcked:           true,
			wantState:           ctask.TaskStateCanceled,
			wantDLQCounter:      1,
		},
		{
			name:              "corrupt message writes DLQ then acks without calling Process",
			action:            actionRun,
			decodeResults:     []error{decodeErr},
			expectProcess:     false,
			wantDLQ:           true,
			wantReason:        decodeErr.Error(),
			wantAcked:         true,
			wantState:         ctask.TaskStateCanceled,
			wantDLQCounter:    1,
			wantDecodeFailCtr: 1,
		},
		{
			name:                "transient decode error then valid request acks without DLQ",
			action:              actionRun,
			decodeResults:       []error{transientErr, nil},
			processResults:      []error{nil},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantAcked:           true,
			wantState:           ctask.TaskStateAcked,
		},
		{
			name:           "non-transient non-corrupt decode error writes DLQ without calling Process or counting a decode failure",
			action:         actionRun,
			decodeResults:  []error{notFoundErr},
			expectProcess:  false,
			wantDLQ:        true,
			wantReason:     notFoundErr.Error(),
			wantAcked:      true,
			wantState:      ctask.TaskStateCanceled,
			wantDLQCounter: 1,
		},
		{
			name:                "EntityNotExists from Process writes DLQ immediately then acks",
			action:              actionRun,
			processResults:      []error{notFoundErr},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantDLQ:             true,
			wantReason:          notFoundErr.Error(),
			wantAcked:           true,
			wantState:           ctask.TaskStateCanceled,
			wantDLQCounter:      1,
		},
		{
			name:                "shard ctx cancelled during retry wait writes no DLQ and does not ack",
			action:              actionCancelWait,
			processResults:      []error{transientErr},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantState:           ctask.TaskStateCanceled,
		},
		{
			name:                "DLQ write failure leaves the message unacked",
			action:              actionRun,
			processResults:      []error{nonTransientErr},
			expectProcess:       true,
			dlqErr:              dlqWriteErr,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantDLQ:             true,
			wantReason:          nonTransientErr.Error(),
			wantAcked:           false,
			wantState:           ctask.TaskStateCanceled,
			wantDLQFailCounter:  1,
		},
		{
			name:      "Nack(nil) writes no DLQ and does not ack",
			action:    actionNackNil,
			wantState: ctask.TaskStateCanceled,
		},
		{
			name:      "Nack with cancelled shard context writes no DLQ and does not ack",
			action:    actionNackCtx,
			wantState: ctask.TaskStateCanceled,
		},
		{
			name:                "Nack and Cancel after Ack are no-ops",
			action:              actionNackTwice,
			processResults:      []error{nil},
			expectProcess:       true,
			wantMinProcessCalls: 1,
			wantExactProcess:    1,
			wantAcked:           true,
			wantState:           ctask.TaskStateAcked,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			logger := testlogger.New(t)
			msg := testMessage()

			decoder := NewMockDecoder(ctrl)
			processor := NewMockProcessor(ctrl)
			manager := persistence.NewMockAsyncWorkflowQueueManager(ctrl)

			ackMgr := messaging.NewContinuousAckManager(logger)
			require.NoError(t, ackMgr.ReadItem(msg.MessageID)) // ack level becomes msgID-1
			require.Equal(t, msg.MessageID-1, ackMgr.GetAckLevel())

			testScope := tally.NewTestScope("", nil)
			scope := metrics.NewClient(testScope, metrics.History, metrics.MigrationConfig{}).Scope(metrics.AsyncWorkflowQueueConsumerScope)

			ts := clock.NewMockedTimeSource()

			// short, deterministic retry policy: expires after 5s of mocked time.
			policy := backoff.NewExponentialRetryPolicy(100 * time.Millisecond)
			policy.SetMaximumInterval(time.Second)
			policy.SetExpirationInterval(5 * time.Second)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Decode runs inside the retried attempt; Nack-only actions never decode.
			decoded := &Request{Message: msg, DomainID: "domain-id"}
			if tc.action != actionNackNil && tc.action != actionNackCtx {
				var decodeCalls atomic.Int32
				decoder.EXPECT().Decode(msg).DoAndReturn(
					func(_ *persistence.AsyncWorkflowMessage) (*Request, error) {
						i := int(decodeCalls.Add(1)) - 1
						if len(tc.decodeResults) == 0 {
							return decoded, nil
						}
						if i >= len(tc.decodeResults) {
							i = len(tc.decodeResults) - 1
						}
						if err := tc.decodeResults[i]; err != nil {
							return nil, err
						}
						return decoded, nil
					}).MinTimes(1)
			}

			var processCalls atomic.Int32
			if tc.expectProcess {
				processor.EXPECT().Process(gomock.Any(), decoded).DoAndReturn(
					func(_ context.Context, _ *Request) error {
						i := int(processCalls.Add(1)) - 1
						if i >= len(tc.processResults) {
							i = len(tc.processResults) - 1
						}
						return tc.processResults[i]
					}).MinTimes(1)
			}

			var gotReason string
			if tc.wantDLQ {
				manager.EXPECT().EnqueueAsyncWorkflowMessageToDLQ(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageToDLQRequest) error {
						assert.Same(t, msg, req.Message)
						gotReason = req.Reason
						return tc.dlqErr
					}).Times(1)
			}

			tk := NewTask(ctx, TaskParams{
				Message:     msg,
				Decoder:     decoder,
				Processor:   processor,
				Manager:     manager,
				AckMgr:      ackMgr,
				RetryPolicy: policy,
				TimeSource:  ts,
				Scope:       scope,
				Logger:      logger,
			})
			assert.Equal(t, ctask.TaskStatePending, tk.State())
			assert.False(t, tk.RetryErr(errors.New("anything")), "the task never asks the processor to retry; retries happen inside Execute")

			switch tc.action {
			case actionRun, actionNackTwice, actionCancelWait:
				onBlocked := func() { ts.Advance(200 * time.Millisecond) }
				if tc.action == actionCancelWait {
					onBlocked = cancel
				}
				execErr := executeWithClock(ts, tk, onBlocked)
				if execErr != nil {
					execErr = tk.HandleErr(execErr)
					require.Error(t, execErr, "HandleErr must return the error")
				}
				if execErr == nil {
					tk.Ack()
				} else {
					tk.Nack(execErr)
				}
				if tc.action == actionNackTwice {
					tk.Nack(errors.New("late nack"))
					tk.Cancel()
				}
			case actionNackNil:
				tk.Nack(nil)
			case actionNackCtx:
				cancel()
				tk.Nack(errors.New("shutting down"))
			}

			assert.Equal(t, tc.wantState, tk.State())
			if tc.wantAcked {
				assert.Equal(t, msg.MessageID, ackMgr.GetAckLevel(), "message must be acked")
			} else {
				assert.Equal(t, msg.MessageID-1, ackMgr.GetAckLevel(), "message must stay unacked")
			}
			if tc.expectProcess {
				assert.GreaterOrEqual(t, int(processCalls.Load()), tc.wantMinProcessCalls)
				if tc.wantExactProcess > 0 {
					assert.Equal(t, tc.wantExactProcess, int(processCalls.Load()))
				}
			}
			if tc.wantReason != "" {
				assert.Equal(t, tc.wantReason, gotReason)
			}

			snap := testScope.Snapshot()
			assert.Equal(t, tc.wantDLQCounter, counterTotal(snap, "async_workflow_queue_dlq"))
			assert.Equal(t, tc.wantDLQFailCounter, counterTotal(snap, "async_workflow_queue_dlq_failures"))
			assert.Equal(t, tc.wantDecodeFailCtr, counterTotal(snap, "async_workflow_queue_decode_failures"))
		})
	}
}

func TestTask_ExecuteWithCancelledContextDoesNotDecodeOrProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tk := NewTask(ctx, TaskParams{
		Message:     testMessage(),
		Decoder:     NewMockDecoder(ctrl),   // no expectations: Decode must not be called
		Processor:   NewMockProcessor(ctrl), // no expectations: Process must not be called
		Manager:     persistence.NewMockAsyncWorkflowQueueManager(ctrl),
		AckMgr:      messaging.NewContinuousAckManager(logger),
		RetryPolicy: backoff.NewExponentialRetryPolicy(time.Millisecond),
		TimeSource:  clock.NewMockedTimeSource(),
		Scope:       metrics.NewNoopMetricsClient().Scope(metrics.AsyncWorkflowQueueConsumerScope),
		Logger:      logger,
	})

	assert.ErrorIs(t, tk.Execute(), context.Canceled)
}
