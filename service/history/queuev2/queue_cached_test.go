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

func TestCachedQueue_Construction_Scheduled(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := NewScheduledQueue(mockShard, persistence.HistoryTaskCategoryTimer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options).(*scheduledQueue)

	q := newCachedQueue(inner, inner.base, mockReader)

	require.NotNil(t, q)
	_, ok := q.(*cachedQueue)
	require.True(t, ok, "expected *cachedQueue")
}

func TestCachedQueue_Construction_Immediate(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	inner := newImmediateQueue(mockShard, persistence.HistoryTaskCategoryTransfer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options)

	q := newCachedQueue(inner, inner.base, mockReader)

	require.NotNil(t, q)
	_, ok := q.(*cachedQueue)
	require.True(t, ok, "expected *cachedQueue")
}

func TestCachedQueue_NotifyNewTask(t *testing.T) {
	for _, k := range cachedQueueKinds() {
		t.Run(k.name, func(t *testing.T) {
			tests := []struct {
				name            string
				info            *hcommon.NotifyTaskInfo
				setupMockReader func(*MockCachedQueueReader)
			}{
				{
					name: "nil tasks",
					info: &hcommon.NotifyTaskInfo{Tasks: nil},
					setupMockReader: func(r *MockCachedQueueReader) {
						r.EXPECT().Inject([]persistence.Task(nil)).Times(1)
					},
				},
				{
					name: "with tasks",
					info: &hcommon.NotifyTaskInfo{Tasks: k.tasks},
					setupMockReader: func(r *MockCachedQueueReader) {
						r.EXPECT().Inject(k.tasks).Times(1)
					},
				},
				{
					name: "persistence error",
					info: &hcommon.NotifyTaskInfo{
						Tasks:            k.tasks,
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

					inner := newMinimalInnerQueue(k.category, &queueBase{metricsScope: metrics.NoopScope})
					cq := &cachedQueue{Queue: inner, reader: mockReader}

					cq.NotifyNewTask("test-cluster", tt.info)
				})
			}
		})
	}
}

func TestCachedQueue_StartStop_Scheduled(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testQueueOptions()
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

	inner := NewScheduledQueue(mockShard, persistence.HistoryTaskCategoryTimer,
		task.NewMockProcessor(ctrl), task.NewMockExecutor(ctrl),
		mockShard.GetLogger(), metrics.NoopClient, metrics.NoopScope, mockReader, options).(*scheduledQueue)

	q := newCachedQueue(inner, inner.base, mockReader)

	q.Start()
	q.Stop()
}

func TestCachedQueue_StartStop_Immediate(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctrl := gomock.NewController(t)

	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)

	options := testQueueOptions()
	mockReader := NewMockCachedQueueReader(ctrl)

	// processEventLoop calls GetTask when processing new tasks; it can fire multiple times.
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

	q := newCachedQueue(inner, inner.base, mockReader)

	q.Start()
	q.Stop()
}

func TestCachedQueue_UpdateQueueStateFn_PropagatesReadLevel(t *testing.T) {
	for _, k := range cachedQueueKinds() {
		t.Run(k.name, func(t *testing.T) {
			tests := []struct {
				name          string
				minReadLevel  persistence.HistoryTaskKey
				expectedLevel persistence.HistoryTaskKey
			}{
				{
					name:          "slices exist - use min read level",
					minReadLevel:  k.sliceReadLevel,
					expectedLevel: k.sliceReadLevel,
				},
				{
					name:          "no slices - fall back to ack level",
					minReadLevel:  persistence.MaximumHistoryTaskKey,
					expectedLevel: k.ackLevel,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					ctrl := gomock.NewController(t)
					mockReader := NewMockCachedQueueReader(ctrl)
					mockVQM := NewMockVirtualQueueManager(ctrl)

					base := &queueBase{
						metricsScope:        metrics.NoopScope,
						virtualQueueManager: mockVQM,
						exclusiveAckLevel:   k.ackLevel,
					}
					base.updateQueueStateFn = func(ctx context.Context) {}
					inner := newMinimalInnerQueue(k.category, base)

					mockVQM.EXPECT().GetMinReadLevel().Return(tt.minReadLevel)
					mockReader.EXPECT().UpdateReadLevel(tt.expectedLevel)

					// newCachedQueue wraps base.updateQueueStateFn in place; invoke it directly.
					newCachedQueue(inner, base, mockReader)
					base.updateQueueStateFn(context.Background())
				})
			}
		})
	}
}

func testQueueOptions() *Options {
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

// newMinimalInnerQueue builds a bare inner Queue of the requested kind for behaviour tests that only
// exercise the cachedQueue wrapper. base is supplied by the caller so it can invoke the wrapped
// updateQueueStateFn directly.
func newMinimalInnerQueue(kind persistence.HistoryTaskCategory, base *queueBase) Queue {
	if kind.Type() == persistence.HistoryTaskCategoryTypeScheduled {
		return &scheduledQueue{base: base, newTimerCh: make(chan struct{}, 1)}
	}
	return &immediateQueue{base: base, notifyCh: make(chan struct{}, 1)}
}

// cachedQueueKind describes one concrete queue kind the generic cachedQueue wrapper supports.
type cachedQueueKind struct {
	name           string
	category       persistence.HistoryTaskCategory
	tasks          []persistence.Task
	sliceReadLevel persistence.HistoryTaskKey
	ackLevel       persistence.HistoryTaskKey
}

func cachedQueueKinds() []cachedQueueKind {
	now := time.Now()
	return []cachedQueueKind{
		{
			name:     "scheduled",
			category: persistence.HistoryTaskCategoryTimer,
			tasks: []persistence.Task{
				&persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{VisibilityTimestamp: now},
				},
			},
			sliceReadLevel: persistence.NewHistoryTaskKey(now, 100),
			ackLevel:       persistence.NewHistoryTaskKey(now.Add(-time.Minute), 50),
		},
		{
			name:     "immediate",
			category: persistence.HistoryTaskCategoryTransfer,
			tasks: []persistence.Task{
				&persistence.ActivityTask{TaskData: persistence.TaskData{TaskID: 1}},
			},
			sliceReadLevel: persistence.NewImmediateTaskKey(100),
			ackLevel:       persistence.NewImmediateTaskKey(50),
		},
	}
}
