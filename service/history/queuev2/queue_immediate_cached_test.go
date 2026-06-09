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

package queuev2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func TestCachedImmediateQueue_Construction(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testImmediateQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := newImmediateQueue(mockShard, persistence.HistoryTaskCategoryTransfer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options)

	q := newCachedImmediateQueue(inner, mockReader)

	require.NotNil(t, q)
	_, ok := q.(*cachedImmediateQueue)
	require.True(t, ok, "expected *cachedImmediateQueue")
}

func TestCachedImmediateQueue_NotifyNewTask(t *testing.T) {
	tasks := []persistence.Task{
		&persistence.ActivityTask{
			TaskData: persistence.TaskData{TaskID: 1},
		},
	}

	tests := []struct {
		name            string
		info            *hcommon.NotifyTaskInfo
		setupMockReader func(*MockCachedQueueReader)
	}{
		{
			name: "with tasks injects into reader",
			info: &hcommon.NotifyTaskInfo{Tasks: tasks},
			setupMockReader: func(r *MockCachedQueueReader) {
				r.EXPECT().Inject(tasks).Times(1)
			},
		},
		{
			name: "nil tasks no reader call",
			info: &hcommon.NotifyTaskInfo{Tasks: nil},
			setupMockReader: func(r *MockCachedQueueReader) {
				// Neither Inject nor Clear should be called for empty task list + no error
			},
		},
		{
			name: "persistence error clears reader",
			info: &hcommon.NotifyTaskInfo{
				Tasks:            []persistence.Task{&persistence.ActivityTask{}},
				PersistenceError: true,
			},
			setupMockReader: func(r *MockCachedQueueReader) {
				r.EXPECT().Clear().Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockReader := NewMockCachedQueueReader(ctrl)
			tt.setupMockReader(mockReader)

			ciq := &cachedImmediateQueue{
				immediateQueue: &immediateQueue{
					base: &queueBase{
						metricsScope: metrics.NoopScope,
					},
					notifyCh: make(chan struct{}, 1),
				},
				reader: mockReader,
			}

			ciq.NotifyNewTask("test-cluster", tt.info)
		})
	}
}

func TestCachedImmediateQueue_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testImmediateQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	mockReader.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			return &GetTaskResponse{
				Progress: &GetTaskProgress{
					Range:       req.Progress.Range,
					NextTaskKey: req.Progress.ExclusiveMaxTaskKey,
				},
			}, nil
		},
	).AnyTimes()
	mockReader.EXPECT().Start().Times(1)
	mockReader.EXPECT().Stop().Times(1)

	inner := newImmediateQueue(mockShard, persistence.HistoryTaskCategoryTransfer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options)

	q := newCachedImmediateQueue(inner, mockReader)

	q.Start()
	q.Stop()
}

func TestCachedImmediateQueue_EvictionHookWired(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testImmediateQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := newImmediateQueue(mockShard, persistence.HistoryTaskCategoryTransfer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options)

	// Replace the original updateQueueStateFn with a no-op so the hook closure
	// can be called without triggering real shard persistence.
	inner.base.updateQueueStateFn = func(ctx context.Context) {}

	q := newCachedImmediateQueue(inner, mockReader)
	ciq := q.(*cachedImmediateQueue)

	// Calling the hooked function exercises the closure (covers the inner body).
	mockReader.EXPECT().UpdateReadLevel(gomock.Any()).Times(1)
	ciq.immediateQueue.base.updateQueueStateFn(context.Background())

	assert.NotNil(t, ciq)
}

func testImmediateQueueOptions() *Options {
	return testScheduledQueueOptions()
}
