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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/.gen/go/sqlblobs"
	asyncconsumer "github.com/uber/cadence/common/asyncworkflow/queue/consumer"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

// fakeTaskScheduler records submitted tasks or fails submissions.
type fakeTaskScheduler struct {
	submitErr error
	submitted []*consumerTask
}

func (f *fakeTaskScheduler) Start() {}
func (f *fakeTaskScheduler) Stop()  {}
func (f *fakeTaskScheduler) Submit(t *consumerTask) error {
	if f.submitErr != nil {
		return f.submitErr
	}
	f.submitted = append(f.submitted, t)
	return nil
}

func validMessagePayload(t *testing.T, workflowID string) []byte {
	t.Helper()
	encoder := codec.NewThriftRWEncoder()
	inner, err := encoder.Encode(thrift.FromStartWorkflowExecutionAsyncRequest(&types.StartWorkflowExecutionAsyncRequest{
		StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
			Domain:       "test-domain",
			WorkflowID:   workflowID,
			WorkflowType: &types.WorkflowType{Name: "test-workflow-type"},
		},
	}))
	require.NoError(t, err)

	reqType := sqlblobs.AsyncRequestTypeStartWorkflowExecutionAsyncRequest
	encoding := string(constants.EncodingTypeThriftRW)
	envelope, err := encoder.Encode(&sqlblobs.AsyncRequestMessage{
		PartitionKey: &workflowID,
		Type:         &reqType,
		Encoding:     &encoding,
		Payload:      inner,
	})
	require.NoError(t, err)
	return envelope
}

func queueMessage(id int64, payload []byte) *persistence.AsyncWorkflowMessage {
	return &persistence.AsyncWorkflowMessage{
		ShardID:      testShardID,
		MessageID:    id,
		Payload:      payload,
		Encoding:     string(constants.EncodingTypeThriftRW),
		PartitionKey: "pk",
	}
}

type consumerTestEnv struct {
	consumer  *consumerImpl
	mgr       *persistence.MockAsyncWorkflowQueueManager
	scheduler *fakeTaskScheduler
}

func newTestConsumer(t *testing.T, ctrl *gomock.Controller, opts func(p *ConsumerParams)) *consumerTestEnv {
	t.Helper()
	mgr := persistence.NewMockAsyncWorkflowQueueManager(ctrl)
	scheduler := &fakeTaskScheduler{}
	params := ConsumerParams{
		ShardID:          testShardID,
		Manager:          mgr,
		Scheduler:        scheduler,
		RequestProcessor: asyncconsumer.NewRequestProcessor(nil, time.Second, testlogger.New(t)),
		Enabled:          func(shardID int) bool { return true },
		PollInterval:     dynamicproperties.GetDurationPropertyFnFilteredByShardID(time.Second),
		CommitInterval:   dynamicproperties.GetDurationPropertyFnFilteredByShardID(5 * time.Second),
		PageSize:         dynamicproperties.GetIntPropertyFilteredByShardID(10),
		TimeSource:       clock.NewMockedTimeSource(),
		Metrics:          metrics.NewNoopMetricsClient(),
		Logger:           testlogger.New(t),
	}
	if opts != nil {
		opts(&params)
	}
	c := NewConsumer(params).(*consumerImpl)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	t.Cleanup(c.cancel)
	return &consumerTestEnv{consumer: c, mgr: mgr, scheduler: scheduler}
}

func TestConsumerPollSeedsFromAckLevelAndDispatches(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), &persistence.GetAsyncWorkflowAckLevelRequest{
		ShardID: testShardID,
	}).Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: 5}, nil).Times(1)

	env.mgr.EXPECT().ReadMessages(gomock.Any(), &persistence.ReadAsyncWorkflowMessagesRequest{
		ShardID:       testShardID,
		LastMessageID: 5,
		PageSize:      10,
	}).Return(&persistence.ReadAsyncWorkflowMessagesResponse{
		Messages: []*persistence.AsyncWorkflowMessage{
			queueMessage(6, validMessagePayload(t, "wf-6")),
			queueMessage(7, validMessagePayload(t, "wf-7")),
		},
	}, nil).Times(1)

	env.consumer.poll()

	assert.True(t, env.consumer.initialized)
	assert.Equal(t, int64(7), env.consumer.cursor)
	require.Len(t, env.scheduler.submitted, 2)
	assert.Equal(t, "test-domain", env.scheduler.submitted[0].Domain())
	assert.Equal(t, "wf-6", env.scheduler.submitted[0].prepared.WorkflowID)
	assert.Equal(t, "wf-7", env.scheduler.submitted[1].prepared.WorkflowID)

	// A subsequent poll does not re-read the ack level.
	env.mgr.EXPECT().ReadMessages(gomock.Any(), &persistence.ReadAsyncWorkflowMessagesRequest{
		ShardID:       testShardID,
		LastMessageID: 7,
		PageSize:      10,
	}).Return(&persistence.ReadAsyncWorkflowMessagesResponse{}, nil).Times(1)
	env.consumer.poll()
}

func TestConsumerPollPaginates(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, func(p *ConsumerParams) {
		p.PageSize = dynamicproperties.GetIntPropertyFilteredByShardID(2)
	})

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)

	gomock.InOrder(
		env.mgr.EXPECT().ReadMessages(gomock.Any(), &persistence.ReadAsyncWorkflowMessagesRequest{
			ShardID:       testShardID,
			LastMessageID: constants.EmptyMessageID,
			PageSize:      2,
		}).Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, validMessagePayload(t, "wf-1")),
				queueMessage(2, validMessagePayload(t, "wf-2")),
			},
		}, nil),
		env.mgr.EXPECT().ReadMessages(gomock.Any(), &persistence.ReadAsyncWorkflowMessagesRequest{
			ShardID:       testShardID,
			LastMessageID: 2,
			PageSize:      2,
		}).Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(3, validMessagePayload(t, "wf-3")),
			},
		}, nil),
	)

	env.consumer.poll()

	assert.Equal(t, int64(3), env.consumer.cursor)
	assert.Len(t, env.scheduler.submitted, 3)
}

func TestConsumerDecodeFailureGoesToDLQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)
	env.mgr.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).
		Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, []byte("garbage")),
				queueMessage(2, validMessagePayload(t, "wf-2")),
			},
		}, nil).Times(1)
	env.mgr.EXPECT().EnqueueToDLQ(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.EnqueueAsyncWorkflowMessageRequest) (*persistence.EnqueueAsyncWorkflowMessageResponse, error) {
			assert.Equal(t, []byte("garbage"), req.Payload)
			return &persistence.EnqueueAsyncWorkflowMessageResponse{MessageID: 1}, nil
		}).Times(1)

	env.consumer.poll()

	// The poison message is acked past; the valid one was submitted.
	assert.Equal(t, int64(1), env.consumer.ackMgr.GetAckLevel())
	assert.Equal(t, int64(2), env.consumer.cursor)
	require.Len(t, env.scheduler.submitted, 1)
	assert.Equal(t, "wf-2", env.scheduler.submitted[0].prepared.WorkflowID)
}

func TestConsumerDLQFailureStopsCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)
	env.mgr.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).
		Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, []byte("garbage")),
				queueMessage(2, validMessagePayload(t, "wf-2")),
			},
		}, nil).Times(1)
	env.mgr.EXPECT().EnqueueToDLQ(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("dlq unavailable")).Times(1)

	env.consumer.poll()

	// The cursor stays before the poison message so it is re-read next poll;
	// the following message was not dispatched.
	assert.Equal(t, int64(constants.EmptyMessageID), env.consumer.cursor)
	assert.Empty(t, env.scheduler.submitted)
}

func TestConsumerSubmitFailureStopsCursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)
	env.scheduler.submitErr = errors.New("scheduler closed")

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)
	env.mgr.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).
		Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, validMessagePayload(t, "wf-1")),
			},
		}, nil).Times(1)

	env.consumer.poll()

	assert.Equal(t, int64(constants.EmptyMessageID), env.consumer.cursor)
}

func TestConsumerCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)
	env.mgr.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).
		Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, validMessagePayload(t, "wf-1")),
				queueMessage(2, validMessagePayload(t, "wf-2")),
			},
		}, nil).Times(1)
	env.consumer.poll()

	// Nothing acked yet, but reading messages 1..2 from an empty queue (level
	// -1) legitimately advances the contiguous level to 0 (nothing below the
	// first read message exists).
	env.mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.UpdateAsyncWorkflowAckLevelRequest) error {
			assert.Equal(t, int64(0), req.AckLevel)
			return nil
		}).Times(1)
	env.consumer.commit(context.Background())

	// Ack both tasks and commit.
	for _, ct := range env.scheduler.submitted {
		ct.Ack()
	}
	env.mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *persistence.UpdateAsyncWorkflowAckLevelRequest) error {
			assert.Equal(t, testShardID, req.ShardID)
			assert.Equal(t, int64(2), req.AckLevel)
			return nil
		}).Times(1)
	env.consumer.commit(context.Background())

	// The level did not advance further: committing again is a no-op.
	env.consumer.commit(context.Background())
}

func TestConsumerCommitFailureRetriesNextTime(t *testing.T) {
	ctrl := gomock.NewController(t)
	env := newTestConsumer(t, ctrl, nil)

	env.mgr.EXPECT().GetAckLevel(gomock.Any(), gomock.Any()).
		Return(&persistence.GetAsyncWorkflowAckLevelResponse{AckLevel: constants.EmptyMessageID}, nil).Times(1)
	env.mgr.EXPECT().ReadMessages(gomock.Any(), gomock.Any()).
		Return(&persistence.ReadAsyncWorkflowMessagesResponse{
			Messages: []*persistence.AsyncWorkflowMessage{
				queueMessage(1, validMessagePayload(t, "wf-1")),
			},
		}, nil).Times(1)
	env.consumer.poll()
	env.scheduler.submitted[0].Ack()

	gomock.InOrder(
		env.mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(errors.New("cas conflict")),
		env.mgr.EXPECT().UpdateAckLevel(gomock.Any(), gomock.Any()).Return(nil),
	)
	env.consumer.commit(context.Background())
	env.consumer.commit(context.Background())
}

func TestConsumerStartStopDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	mgr := persistence.NewMockAsyncWorkflowQueueManager(ctrl)

	// No expectations on the manager: a disabled consumer must not touch
	// persistence, and stopping before initialization must not commit.
	c := NewConsumer(ConsumerParams{
		ShardID:          testShardID,
		Manager:          mgr,
		Scheduler:        &fakeTaskScheduler{},
		RequestProcessor: asyncconsumer.NewRequestProcessor(nil, time.Second, testlogger.New(t)),
		Enabled:          func(shardID int) bool { return false },
		PollInterval:     dynamicproperties.GetDurationPropertyFnFilteredByShardID(time.Millisecond),
		CommitInterval:   dynamicproperties.GetDurationPropertyFnFilteredByShardID(time.Millisecond),
		PageSize:         dynamicproperties.GetIntPropertyFilteredByShardID(10),
		TimeSource:       clock.NewRealTimeSource(),
		Metrics:          metrics.NewNoopMetricsClient(),
		Logger:           testlogger.New(t),
	})

	c.Start()
	c.Start() // idempotent
	time.Sleep(20 * time.Millisecond)
	c.Stop()
	c.Stop() // idempotent
}
