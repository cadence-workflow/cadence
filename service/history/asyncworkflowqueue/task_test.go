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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/common/types"
)

const testMessageID = int64(7)

type consumerTaskTestDeps struct {
	mgr    *persistence.MockAsyncWorkflowQueueManager
	ackMgr messaging.AckManager
	ctx    context.Context
	cancel context.CancelFunc
}

func newTestConsumerTask(t *testing.T, ctrl *gomock.Controller, invoke func(ctx context.Context) (string, error)) (*consumerTask, *consumerTaskTestDeps) {
	t.Helper()

	deps := &consumerTaskTestDeps{
		mgr:    persistence.NewMockAsyncWorkflowQueueManager(ctrl),
		ackMgr: messaging.NewContinuousAckManager(testlogger.New(t)),
	}
	deps.ctx, deps.cancel = context.WithCancel(context.Background())
	t.Cleanup(deps.cancel)

	require.NoError(t, deps.ackMgr.ReadItem(testMessageID))

	ct := newConsumerTask(
		deps.ctx,
		&consumer.PreparedRequest{
			RequestType: "StartWorkflowExecutionAsyncRequest",
			Domain:      "test-domain",
			WorkflowID:  "test-workflow-id",
			Invoke:      invoke,
		},
		&persistence.AsyncWorkflowMessage{
			ShardID:      testShardID,
			MessageID:    testMessageID,
			Payload:      []byte("payload"),
			Encoding:     "thriftrw",
			PartitionKey: "test-workflow-id",
		},
		deps.mgr,
		deps.ackMgr,
		clock.NewMockedTimeSource(),
		testlogger.New(t),
		metrics.NewNoopMetricsClient().Scope(metrics.AsyncWorkflowConsumerScope),
	)
	return ct, deps
}

func TestConsumerTaskExecute(t *testing.T) {
	ctrl := gomock.NewController(t)

	t.Run("success", func(t *testing.T) {
		ct, _ := newTestConsumerTask(t, ctrl, func(ctx context.Context) (string, error) {
			return "run-1", nil
		})
		require.NoError(t, ct.Execute())
	})

	t.Run("error propagates", func(t *testing.T) {
		wantErr := errors.New("boom")
		ct, _ := newTestConsumerTask(t, ctrl, func(ctx context.Context) (string, error) {
			return "", wantErr
		})
		assert.ErrorIs(t, ct.Execute(), wantErr)
	})
}

func TestConsumerTaskRetryErr(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		cancelCtx bool
		want      bool
	}{
		{
			name: "transient service error is retryable",
			err:  &types.InternalServiceError{Message: "oh no"},
			want: true,
		},
		{
			name: "non-transient error is not retryable",
			err:  &types.BadRequestError{Message: "bad"},
			want: false,
		},
		{
			name: "corrupt message is not retryable",
			err:  fmt.Errorf("%w: bad payload", consumer.ErrCorruptMessage),
			want: false,
		},
		{
			name:      "canceled context is not retryable",
			err:       &types.InternalServiceError{Message: "oh no"},
			cancelCtx: true,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ct, deps := newTestConsumerTask(t, ctrl, nil)
			if tc.cancelCtx {
				deps.cancel()
			}
			assert.Equal(t, tc.want, ct.RetryErr(tc.err))
		})
	}
}

func TestConsumerTaskAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	ct, deps := newTestConsumerTask(t, ctrl, nil)

	ct.Ack()
	assert.Equal(t, task.TaskStateAcked, ct.State())
	assert.Equal(t, testMessageID, deps.ackMgr.GetAckLevel())

	// Ack is idempotent and a later Nack is a no-op (no DLQ expectations set).
	ct.Ack()
	ct.Nack(errors.New("too late"))
	assert.Equal(t, task.TaskStateAcked, ct.State())
}

func TestConsumerTaskNack(t *testing.T) {
	// The ack manager seeds its level to firstItemID-1 on the first ReadItem,
	// so "unacked" leaves the level just below the test message.
	initialAckLevel := testMessageID - 1

	tests := []struct {
		name         string
		nackErr      error
		cancelCtx    bool
		setupMgr     func(mgr *persistence.MockAsyncWorkflowQueueManager)
		wantAckLevel int64
	}{
		{
			name:         "nil error means shutdown drain: no DLQ, no ack",
			nackErr:      nil,
			wantAckLevel: initialAckLevel,
		},
		{
			name:         "canceled context: no DLQ, no ack",
			nackErr:      errors.New("interrupted mid-retry"),
			cancelCtx:    true,
			wantAckLevel: initialAckLevel,
		},
		{
			name:    "terminal error: DLQ then ack",
			nackErr: errors.New("poison"),
			setupMgr: func(mgr *persistence.MockAsyncWorkflowQueueManager) {
				mgr.EXPECT().EnqueueToDLQ(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageRequest) (*persistence.EnqueueAsyncWorkflowMessageResponse, error) {
						assert.Equal(t, testShardID, req.ShardID)
						assert.Equal(t, []byte("payload"), req.Payload)
						assert.Equal(t, "thriftrw", req.Encoding)
						assert.Equal(t, "test-workflow-id", req.PartitionKey)
						return &persistence.EnqueueAsyncWorkflowMessageResponse{MessageID: 1}, nil
					}).Times(1)
			},
			wantAckLevel: testMessageID,
		},
		{
			name:    "DLQ write failure: message stays unacked",
			nackErr: errors.New("poison"),
			setupMgr: func(mgr *persistence.MockAsyncWorkflowQueueManager) {
				mgr.EXPECT().EnqueueToDLQ(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("dlq unavailable")).Times(1)
			},
			wantAckLevel: initialAckLevel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ct, deps := newTestConsumerTask(t, ctrl, nil)
			if tc.cancelCtx {
				deps.cancel()
			}
			if tc.setupMgr != nil {
				tc.setupMgr(deps.mgr)
			}

			ct.Nack(tc.nackErr)
			assert.Equal(t, task.TaskStateCanceled, ct.State())
			assert.Equal(t, tc.wantAckLevel, deps.ackMgr.GetAckLevel())

			// Nack is terminal: a later Ack must not advance the ack level.
			ackLevelAfterNack := deps.ackMgr.GetAckLevel()
			ct.Ack()
			assert.Equal(t, task.TaskStateCanceled, ct.State())
			assert.Equal(t, ackLevelAfterNack, deps.ackMgr.GetAckLevel())
		})
	}
}
