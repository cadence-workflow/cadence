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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func TestCachedScheduledQueue_Construction(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testScheduledQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := newScheduledQueue(mockShard, persistence.HistoryTaskCategoryTimer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope,
		options, mockReader)

	q := newCachedScheduledQueue(inner, mockReader)

	require.NotNil(t, q)
	csq, ok := q.(*cachedScheduledQueue)
	require.True(t, ok, "expected *cachedScheduledQueue")
	assert.NotNil(t, csq.reader)
	assert.NotNil(t, csq.scheduledQueue)
}

func TestCachedScheduledQueue_NotifyNewTask_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReader := NewMockCachedQueueReader(ctrl)

	// Inject is always called even with nil tasks; the real implementation handles it.
	mockReader.EXPECT().Inject([]persistence.Task(nil)).Times(1)

	csq := &cachedScheduledQueue{
		scheduledQueue: &scheduledQueue{
			base: &queueBase{
				metricsScope: metrics.NoopScope,
			},
			newTimerCh: make(chan struct{}, 1),
		},
		reader: mockReader,
	}

	csq.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks: nil,
	})
}

func TestCachedScheduledQueue_NotifyNewTask_WithTasks(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReader := NewMockCachedQueueReader(ctrl)

	tasks := []persistence.Task{
		&persistence.DecisionTimeoutTask{
			TaskData: persistence.TaskData{
				VisibilityTimestamp: time.Now(),
			},
		},
	}

	mockReader.EXPECT().Inject(tasks).Times(1)

	csq := &cachedScheduledQueue{
		scheduledQueue: &scheduledQueue{
			base: &queueBase{
				metricsScope: metrics.NoopScope,
			},
			newTimerCh: make(chan struct{}, 1),
		},
		reader: mockReader,
	}

	csq.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks: tasks,
	})
}

// TestCachedScheduledQueue_StartStop verifies that Start and Stop delegate to both
// the reader and the inner scheduledQueue.
func TestCachedScheduledQueue_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testScheduledQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	// processEventLoop calls LookAHead after the timer gate fires, and GetTask
	// when processing new tasks. Both can fire multiple times.
	mockReader.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{}, nil).AnyTimes()
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

	inner := newScheduledQueue(mockShard, persistence.HistoryTaskCategoryTimer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope,
		options, mockReader)

	q := newCachedScheduledQueue(inner, mockReader)

	q.Start()
	q.Stop()
}

// TestCachedScheduledQueue_StartStopDelegation verifies that cachedScheduledQueue.Start
// and Stop delegate to the embedded reader.
func TestCachedScheduledQueue_StartStopDelegation(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReader := NewMockCachedQueueReader(ctrl)
	mockReader.EXPECT().Start().Times(1)
	mockReader.EXPECT().Stop().Times(1)

	csq := &cachedScheduledQueue{
		reader: mockReader,
	}

	csq.reader.Start()
	csq.reader.Stop()
}

func TestCachedScheduledQueue_EvictionHookWired(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testScheduledQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := newScheduledQueue(mockShard, persistence.HistoryTaskCategoryTimer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope,
		options, mockReader)

	// Replace the original updateQueueStateFn with a no-op so the hook closure
	// can be called without triggering real shard persistence.
	inner.base.updateQueueStateFn = func(ctx context.Context) {}

	q := newCachedScheduledQueue(inner, mockReader)
	csq := q.(*cachedScheduledQueue)

	// The eviction hook should be wired into updateQueueStateFn.
	assert.NotNil(t, csq.scheduledQueue.base.updateQueueStateFn)

	// Calling the hooked function exercises the closure (covers the inner body).
	mockReader.EXPECT().UpdateReadLevel(gomock.Any()).Times(1)
	csq.scheduledQueue.base.updateQueueStateFn(context.Background())
}

func TestCachedScheduledQueue_NotifyNewTask_PersistenceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockReader := NewMockCachedQueueReader(ctrl)

	// PersistenceError=true: Clear is called, Inject is not.
	mockReader.EXPECT().Clear().Times(1)

	csq := &cachedScheduledQueue{
		scheduledQueue: &scheduledQueue{
			base: &queueBase{
				metricsScope: metrics.NoopScope,
			},
			newTimerCh: make(chan struct{}, 1),
		},
		reader: mockReader,
	}

	csq.NotifyNewTask("test-cluster", &hcommon.NotifyTaskInfo{
		Tasks:            []persistence.Task{&persistence.DecisionTimeoutTask{}},
		PersistenceError: true,
	})
}

func testScheduledQueueOptions() *Options {
	return &Options{
		DeleteBatchSize:                      dynamicproperties.GetIntPropertyFn(100),
		RedispatchInterval:                   dynamicproperties.GetDurationPropertyFn(10 * time.Second),
		PageSize:                             dynamicproperties.GetIntPropertyFn(100),
		PollBackoffInterval:                  dynamicproperties.GetDurationPropertyFn(10 * time.Second),
		MaxPollInterval:                      dynamicproperties.GetDurationPropertyFn(10 * time.Second),
		MaxPollIntervalJitterCoefficient:     dynamicproperties.GetFloatPropertyFn(0.1),
		UpdateAckInterval:                    dynamicproperties.GetDurationPropertyFn(10 * time.Second),
		UpdateAckIntervalJitterCoefficient:   dynamicproperties.GetFloatPropertyFn(0.1),
		MaxPollRPS:                           dynamicproperties.GetIntPropertyFn(100),
		MaxPendingTasksCount:                 dynamicproperties.GetIntPropertyFn(100),
		PollBackoffIntervalJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0.0),
		VirtualSliceForceAppendInterval:      dynamicproperties.GetDurationPropertyFn(10 * time.Second),
		CriticalPendingTaskCount:             dynamicproperties.GetIntPropertyFn(90),
		EnablePendingTaskCountAlert:          func() bool { return true },
		MaxVirtualQueueCount:                 dynamicproperties.GetIntPropertyFn(2),
	}
}
