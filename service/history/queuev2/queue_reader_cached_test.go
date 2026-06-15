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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

func testOptions(overrides ...func(*cachedQueueReaderOptions)) *cachedQueueReaderOptions {
	opts := &cachedQueueReaderOptions{
		Mode:                      dynamicproperties.GetStringPropertyFn("enabled"),
		MaxSize:                   dynamicproperties.GetIntPropertyFn(100),
		MaxLookAheadWindow:        dynamicproperties.GetDurationPropertyFn(time.Hour),
		PrefetchTriggerWindow:     dynamicproperties.GetDurationPropertyFn(5 * time.Minute),
		PrefetchPageSize:          dynamicproperties.GetIntPropertyFn(10),
		TimeEvictionWindow:        dynamicproperties.GetDurationPropertyFn(time.Minute),
		MinPrefetchInterval:       dynamicproperties.GetDurationPropertyFn(100 * time.Millisecond),
		PrefetchJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0),
	}
	for _, o := range overrides {
		if o == nil {
			continue
		}
		o(opts)
	}
	return opts
}

type cachedQueueReaderMockDeps struct {
	mockBase  *MockQueueReader
	mockQueue *MockInMemQueue
	mockShard *shard.MockContext
	clock     clock.MockedTimeSource
}

func setupMocksForCachedQueueReader(
	t *testing.T,
	ctrl *gomock.Controller,
	overrides ...func(*cachedQueueReaderOptions),
) (*cachedQueueReader, *cachedQueueReaderMockDeps) {
	t.Helper()
	mockShard := shard.NewMockContext(ctrl)
	mockShard.EXPECT().GetRangeID().Return(int64(0)).AnyTimes()
	mockShard.EXPECT().GetConfig().Return(&config.Config{RangeSizeBits: 20}).AnyTimes()
	deps := &cachedQueueReaderMockDeps{
		mockBase:  NewMockQueueReader(ctrl),
		mockQueue: NewMockInMemQueue(ctrl),
		mockShard: mockShard,
		clock:     clock.NewMockedTimeSource(),
	}

	r := newCachedQueueReaderWithOptions(
		deps.mockBase,
		deps.mockQueue,
		deps.mockShard,
		deps.clock,
		testlogger.New(t),
		metrics.NoopScope,
		testOptions(overrides...),
	)

	return r, deps
}

func setBounds(r *cachedQueueReader, lower, upper persistence.HistoryTaskKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inclusiveLowerBound = lower
	r.exclusiveUpperBound = upper
}

func newTimeKey(t time.Time) persistence.HistoryTaskKey {
	return newTaskKey(t, 0)
}

func newTaskKey(t time.Time, taskID int64) persistence.HistoryTaskKey {
	return persistence.NewHistoryTaskKey(t, taskID)
}

func newTask(id int64, scheduledAt time.Time) persistence.Task {
	return &persistence.DeleteHistoryEventTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "test-domain",
			WorkflowID: "test-workflow",
			RunID:      "test-run",
		},
		TaskData: persistence.TaskData{
			TaskID:              id,
			VisibilityTimestamp: scheduledAt,
		},
	}
}

func newProgress(lower, upper persistence.HistoryTaskKey) *GetTaskProgress {
	return &GetTaskProgress{
		Range:         Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: upper},
		NextPageToken: nil,
		NextTaskKey:   lower,
	}
}

func TestCachedQueueReader_Modes(t *testing.T) {
	tests := []struct {
		mode     string
		enabled  bool
		shadow   bool
		disabled bool
	}{
		{"enabled", true, false, false},
		{"shadow", false, true, false},
		{"disabled", false, false, true},
		{"unknown", false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, _ := setupMocksForCachedQueueReader(t, ctrl, func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)
			})
			assert.Equal(t, tc.enabled, r.isEnabled(), "mode %q: isEnabled", tc.mode)
			assert.Equal(t, tc.shadow, r.isShadow(), "mode %q: isShadow", tc.mode)
			assert.Equal(t, tc.disabled, r.isDisabled(), "mode %q: isDisabled", tc.mode)
		})
	}
}

func TestCachedQueueReader_UpdateReadLevel(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		initLower  persistence.HistoryTaskKey
		initUpper  persistence.HistoryTaskKey
		readLevel  persistence.HistoryTaskKey
		setupMocks func(queue *MockInMemQueue)
		wantLower  persistence.HistoryTaskKey
	}{
		{
			name:      "advances lower bound",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: newTimeKey(now.Add(time.Hour)),
			readLevel: newTimeKey(now),
			wantLower: newTimeKey(now),
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().LTrim(newTimeKey(now))
			},
		},
		{
			name:      "does not retreat lower bound",
			initLower: newTimeKey(now.Add(time.Minute)),
			initUpper: newTimeKey(now.Add(time.Hour)),
			readLevel: newTimeKey(now),
			wantLower: newTimeKey(now.Add(time.Minute)),
		},
		{
			name:      "MaximumHistoryTaskKey not change",
			initLower: newTimeKey(now.Add(time.Minute)),
			initUpper: newTimeKey(now.Add(time.Minute)),
			readLevel: persistence.MaximumHistoryTaskKey,
			wantLower: newTimeKey(now.Add(time.Minute)),
		},
		{
			name:      "capped at upper bound",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: newTimeKey(now.Add(time.Minute)),
			readLevel: newTimeKey(now.Add(time.Hour)),
			wantLower: newTimeKey(now.Add(time.Minute)),
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().LTrim(newTimeKey(now.Add(time.Minute)))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl)
			queue := deps.mockQueue
			setBounds(r, tc.initLower, tc.initUpper)

			queue.EXPECT().Len().Return(0).AnyTimes()
			if tc.setupMocks != nil {
				tc.setupMocks(queue)
			}

			r.UpdateReadLevel(tc.readLevel)

			r.mu.RLock()
			got := r.inclusiveLowerBound
			r.mu.RUnlock()
			assert.True(t, got.Equal(tc.wantLower), "got %v want %v", got, tc.wantLower)
		})
	}
}

func TestCachedQueueReader_Inject(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))
	prefetchTarget := newTimeKey(now.Add(2 * time.Hour))
	inside := newTask(1, now.Add(30*time.Minute))
	before := newTask(2, now.Add(-time.Minute))
	atUpper := newTask(3, upper.GetScheduledTime())
	inBuffer := newTask(4, now.Add(90*time.Minute))  // in [upper, prefetchTarget)
	beyondTarget := newTask(5, now.Add(3*time.Hour)) // beyond prefetchTarget
	zeroID := newTask(0, now.Add(30*time.Minute))
	trimKey := inside.GetTaskKey().Next()

	tests := []struct {
		name               string
		tasks              []persistence.Task
		initPrefetchTarget persistence.HistoryTaskKey
		optsOverride       func(*cachedQueueReaderOptions)
		setupMocks         func(queue *MockInMemQueue)
		wantUpper          persistence.HistoryTaskKey
		wantBufferLen      int
	}{
		{
			name:  "disabled skips all",
			tasks: []persistence.Task{inside},
			optsOverride: func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn("disabled")
			},
			setupMocks: func(*MockInMemQueue) {},
			wantUpper:  upper,
		},
		{
			name:  "task inside window accepted",
			tasks: []persistence.Task{inside},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().PutTasks([]persistence.Task{inside})
				queue.EXPECT().RTrimBySize(100).Return(persistence.MinimumHistoryTaskKey, false)
			},
			wantUpper: upper,
		},
		{
			name:  "task before lower skipped",
			tasks: []persistence.Task{before},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
			},
			wantUpper: upper,
		},
		{
			name:  "task at upper bound (exclusive) skipped",
			tasks: []persistence.Task{atUpper},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
			},
			wantUpper: upper,
		},
		{
			name:  "mixed: only inside accepted",
			tasks: []persistence.Task{inside, before, atUpper},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().PutTasks([]persistence.Task{inside})
				queue.EXPECT().RTrimBySize(100).Return(persistence.MinimumHistoryTaskKey, false)
			},
			wantUpper: upper,
		},
		{
			// putTasks short-circuits on empty slice, so no queue calls expected.
			name:       "task with ID=0 skipped",
			tasks:      []persistence.Task{zeroID},
			setupMocks: func(*MockInMemQueue) {},
			wantUpper:  upper,
		},
		{
			// RTrimBySize fires and the upper bound must shrink to the trim key.
			name:  "trims when over capacity: upper bound updated",
			tasks: []persistence.Task{inside},
			optsOverride: func(o *cachedQueueReaderOptions) {
				o.MaxSize = dynamicproperties.GetIntPropertyFn(1)
			},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().PutTasks([]persistence.Task{inside})
				queue.EXPECT().RTrimBySize(1).Return(trimKey, true)
			},
			wantUpper: trimKey,
		},
		{
			name:               "task in [upper, prefetchTarget) buffered when prefetch in-flight",
			tasks:              []persistence.Task{inBuffer},
			initPrefetchTarget: prefetchTarget,
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
			},
			wantUpper:     upper,
			wantBufferLen: 1,
		},
		{
			name:               "task beyond prefetchTarget dropped even when prefetch in-flight",
			tasks:              []persistence.Task{beyondTarget},
			initPrefetchTarget: prefetchTarget,
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
			},
			wantUpper:     upper,
			wantBufferLen: 0,
		},
		{
			name:               "task with ID=0 not buffered even when prefetch in-flight",
			tasks:              []persistence.Task{zeroID},
			initPrefetchTarget: prefetchTarget,
			setupMocks:         func(*MockInMemQueue) {},
			wantUpper:          upper,
			wantBufferLen:      0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl, tc.optsOverride)
			queue := deps.mockQueue
			setBounds(r, lower, upper)
			if !tc.initPrefetchTarget.Equal(persistence.MinimumHistoryTaskKey) {
				r.mu.Lock()
				r.prefetchTargetUpper = tc.initPrefetchTarget
				r.mu.Unlock()
			}
			tc.setupMocks(queue)

			r.Inject(tc.tasks)

			r.mu.RLock()
			gotUpper := r.exclusiveUpperBound
			gotBufferLen := len(r.pendingInjectBuffer)
			r.mu.RUnlock()
			assert.True(t, gotUpper.Equal(tc.wantUpper), "upper: got %v want %v", gotUpper, tc.wantUpper)
			assert.Equal(t, tc.wantBufferLen, gotBufferLen, "buffer length")
		})
	}
}

func TestCachedQueueReader_Clear(t *testing.T) {
	now := time.Now()
	ctrl := gomock.NewController(t)
	r, deps := setupMocksForCachedQueueReader(t, ctrl)
	queue := deps.mockQueue
	setBounds(r, newTimeKey(now), newTimeKey(now.Add(time.Hour)))
	r.mu.Lock()
	r.prefetchTargetUpper = newTimeKey(now.Add(2 * time.Hour))
	r.pendingInjectBuffer = []persistence.Task{newTask(1, now.Add(90*time.Minute))}
	r.mu.Unlock()

	queue.EXPECT().Len().Return(0).AnyTimes()
	queue.EXPECT().Clear()

	r.Clear()

	r.mu.RLock()
	defer r.mu.RUnlock()

	assert.True(t, r.exclusiveUpperBound.Equal(persistence.MinimumHistoryTaskKey),
		"upper: got %v want Minimum", r.exclusiveUpperBound)
	assert.True(t, r.inclusiveLowerBound.Equal(persistence.MinimumHistoryTaskKey),
		"lower: got %v want Minimum", r.inclusiveLowerBound)
	assert.True(t, r.prefetchTargetUpper.Equal(persistence.MinimumHistoryTaskKey),
		"prefetchTargetUpper: got %v want Minimum", r.prefetchTargetUpper)
	assert.Empty(t, r.pendingInjectBuffer, "pendingInjectBuffer should be empty after Clear")
}

func TestCachedQueueReader_InsertBufferedTasks(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))

	tIn := newTask(1, now.Add(30*time.Minute))  // within [lower, upper)
	tOut := newTask(2, now.Add(90*time.Minute)) // beyond upper

	tests := []struct {
		name       string
		initBuffer []persistence.Task
		setupMocks func(queue *MockInMemQueue)
	}{
		{
			name:       "empty buffer: no-op",
			initBuffer: nil,
			setupMocks: func(*MockInMemQueue) {},
		},
		{
			name:       "covered task drained into cache",
			initBuffer: []persistence.Task{tIn},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().PutTasks([]persistence.Task{tIn})
				queue.EXPECT().RTrimBySize(100).Return(persistence.MinimumHistoryTaskKey, false)
			},
		},
		{
			name:       "uncovered task dropped, buffer cleared",
			initBuffer: []persistence.Task{tOut},
			setupMocks: func(*MockInMemQueue) {},
		},
		{
			name:       "mixed: covered drained, uncovered dropped",
			initBuffer: []persistence.Task{tIn, tOut},
			setupMocks: func(queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().PutTasks([]persistence.Task{tIn})
				queue.EXPECT().RTrimBySize(100).Return(persistence.MinimumHistoryTaskKey, false)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl)
			queue := deps.mockQueue
			setBounds(r, lower, upper)

			r.mu.Lock()
			r.pendingInjectBuffer = append(r.pendingInjectBuffer, tc.initBuffer...)
			r.mu.Unlock()

			tc.setupMocks(queue)

			r.mu.Lock()
			r.insertBufferedTasks()
			gotBuffer := r.pendingInjectBuffer
			r.mu.Unlock()

			assert.Empty(t, gotBuffer, "pendingInjectBuffer after drain")
		})
	}
}

func TestCachedQueueReader_GetTask(t *testing.T) {
	var (
		now         = time.Now()
		beforeLower = newTimeKey(now.Add(-time.Minute))
		lower       = newTimeKey(now)
		cacheStart  = newTimeKey(now.Add(30 * time.Minute))
		upper       = newTimeKey(now.Add(time.Hour))
		rangeMax    = newTimeKey(now.Add(2 * time.Hour))

		t1        = newTask(1, now.Add(10*time.Minute))
		t2        = newTask(2, now.Add(20*time.Minute))
		cacheTask = newTask(2, now.Add(45*time.Minute))
	)

	tests := []struct {
		name        string
		mode        string
		lastRangeID int64
		lower       persistence.HistoryTaskKey
		upper       persistence.HistoryTaskKey
		req         *GetTaskRequest
		setupMocks  func(base *MockQueueReader, queue *MockInMemQueue)
		wantErr     bool
		wantResp    *GetTaskResponse
	}{
		{
			name:  "disabled delegates to base",
			mode:  "disabled",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: newProgress(lower, upper),
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: newProgress(lower, upper),
			},
		},
		{
			name:  "miss: start key before lower bound delegates to base",
			mode:  "enabled",
			lower: lower,
			upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(beforeLower, rangeMax),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: beforeLower, ExclusiveMaxTaskKey: rangeMax},
						NextTaskKey: rangeMax,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: beforeLower, ExclusiveMaxTaskKey: rangeMax},
					NextTaskKey: rangeMax,
				},
			},
		},
		{
			name:  "miss: error from base returns error",
			mode:  "enabled",
			lower: lower,
			upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(beforeLower, rangeMax),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		{
			name:  "hit: cache returns tasks",
			mode:  "enabled",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1, t2}, upper)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{
					Range: Range{
						// InclusiveMinTaskKey advances to nextTaskKey per GetTask implementation.
						InclusiveMinTaskKey: upper,
						ExclusiveMaxTaskKey: upper,
					},
					NextTaskKey: upper,
				},
			},
		},
		{
			// NextPageToken set and NextTaskKey falls inside cache window → cache hit.
			name:  "nextPageToken with nextTaskKey inside cache hits cache",
			mode:  "enabled",
			lower: cacheStart, upper: rangeMax,
			req: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range:         Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: rangeMax},
					NextPageToken: []byte("page-1"),
					NextTaskKey:   cacheStart,
				},
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(cacheStart, rangeMax, gomock.Any(), 10).
					Return([]persistence.Task{cacheTask}, rangeMax)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{cacheTask},
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: rangeMax,
						ExclusiveMaxTaskKey: rangeMax,
					},
					NextTaskKey: rangeMax,
				},
			},
		},
		{
			// NextPageToken is set but NextTaskKey is MinimumHistoryTaskKey → explicit base fallback.
			// GetTask returns early before acquiring the cache lock.
			name:  "NextPageToken: MinimumHistoryTaskKey NextTaskKey delegates to base",
			mode:  "enabled",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range:         Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: upper},
					NextPageToken: []byte("page-x"),
					NextTaskKey:   persistence.MinimumHistoryTaskKey,
				},
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				// no queue.Len(): early return before acquiring lock
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: upper},
						NextTaskKey: upper,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: upper},
					NextTaskKey: upper,
				},
			},
		},
		{
			// Cache hit but queue returns no tasks for the sub-range (empty window slice).
			name:  "cache hit: empty task list when no tasks in range",
			mode:  "enabled",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return(nil, upper) // no tasks; nextTaskKey = upper
			},
			wantResp: &GetTaskResponse{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: upper, // nextTaskKey
						ExclusiveMaxTaskKey: upper,
					},
					NextTaskKey: upper,
				},
			},
		},
		{
			// Miss: request exclusiveMax exceeds the cached window upper bound.
			name:  "miss: request range end beyond window upper bound",
			mode:  "enabled",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, rangeMax), // rangeMax > upper
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: rangeMax},
						NextTaskKey: rangeMax,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: rangeMax},
					NextTaskKey: rangeMax,
				},
			},
		},
		{
			// Miss: cache is uninitialized (both bounds at Minimum). Every real request is a miss
			// because exclusiveMax of any real range exceeds Minimum.
			name:  "miss: cache uninitialized (both bounds Minimum)",
			mode:  "enabled",
			lower: persistence.MinimumHistoryTaskKey, upper: persistence.MinimumHistoryTaskKey,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: newProgress(lower, upper),
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: newProgress(lower, upper),
			},
		},
		{
			name:        "rangeID changed: clears cache and falls back to base",
			mode:        "enabled",
			lastRangeID: 1,
			lower:       lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, rangeMax),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().Clear()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Progress: newProgress(upper, rangeMax),
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Progress: newProgress(upper, rangeMax),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl, func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)
			})
			base, queue := deps.mockBase, deps.mockQueue
			setBounds(r, tc.lower, tc.upper)
			r.lastRangeID = tc.lastRangeID
			tc.setupMocks(base, queue)

			resp, err := r.GetTask(context.Background(), tc.req)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantResp, resp)
		})
	}
}

func TestCachedQueueReader_GetTask_Shadow(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))
	rangeMax := newTimeKey(now.Add(2 * time.Hour))

	t1 := newTask(1, now.Add(10*time.Minute))
	t2 := newTask(2, now.Add(20*time.Minute))
	t3 := newTask(3, now.Add(30*time.Minute))
	// tOtherOwner has a taskID in rangeID=1 (1<<20 at RangeSizeBits=20).
	// The default mock GetRangeID()=0, so this task will be classified as OwnerChanged.
	tOtherOwner := newTask(1<<20, now.Add(40*time.Minute))

	tests := []struct {
		name       string
		lower      persistence.HistoryTaskKey
		upper      persistence.HistoryTaskKey
		req        *GetTaskRequest
		setupMocks func(base *MockQueueReader, queue *MockInMemQueue)
		wantErr    bool
		wantResp   *GetTaskResponse
	}{
		{
			// Cache is populated and DB returns the same tasks → returns DB result.
			name:  "cache hit, tasks match: returns DB result",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1, t2}, upper)
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1, t2},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
						NextTaskKey: upper,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
					NextTaskKey: upper,
				},
			},
		},
		{
			// DB has a task not in cache → MissedFromCache mismatch; still returns DB result.
			name:  "cache hit, task missing from cache: returns DB result",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				// Cache has only t1; DB has t1 and t2.
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1}, upper)
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1, t2},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
						NextTaskKey: upper,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
					NextTaskKey: upper,
				},
			},
		},
		{
			// Cache has an extra task not in DB (inject race) → ExtraInCache only; returns DB result.
			name:  "cache hit, extra task in cache (inject race): returns DB result",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				// Cache has t1, t2, t3; DB has only t1, t2 (t3 not yet committed).
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1, t2, t3}, upper)
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1, t2},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
						NextTaskKey: upper,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
					NextTaskKey: upper,
				},
			},
		},
		{
			// NextTaskKey may differ (Cassandra cursor artifact) — must not flag as mismatch; returns DB result.
			name:  "cache hit, NextTaskKey differs: no mismatch, returns DB result",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1, t2}, upper)
				// DB returns same tasks but a different NextTaskKey (Cassandra cursor artifact).
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1, t2},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: rangeMax, ExclusiveMaxTaskKey: rangeMax},
						NextTaskKey: rangeMax,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: rangeMax, ExclusiveMaxTaskKey: rangeMax},
					NextTaskKey: rangeMax,
				},
			},
		},
		{
			// Request range not covered by cache → falls through to base without comparison.
			name:  "cache miss: delegates to base without comparison",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, rangeMax), // rangeMax > upper
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: rangeMax},
						NextTaskKey: rangeMax,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: lower, ExclusiveMaxTaskKey: rangeMax},
					NextTaskKey: rangeMax,
				},
			},
		},
		{
			// base.GetTask returns an error during shadow comparison → error propagated.
			name:  "cache hit, base error: returns error",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1}, upper)
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		{
			// DB has a task created by a different shard owner (different rangeID) → classified as
			// OwnerChanged, not a mismatch; DB result returned normally.
			name:  "task missing from cache, different rangeID: owner changed, no mismatch warning",
			lower: lower, upper: upper,
			req: &GetTaskRequest{
				Progress:  newProgress(lower, upper),
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				// Cache has only t1; DB has t1 and tOtherOwner (rangeID=1, current rangeID=0).
				queue.EXPECT().GetTasks(lower, upper, gomock.Any(), 10).
					Return([]persistence.Task{t1}, upper)
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks: []persistence.Task{t1, tOtherOwner},
					Progress: &GetTaskProgress{
						Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
						NextTaskKey: upper,
					},
				}, nil)
			},
			wantResp: &GetTaskResponse{
				Tasks: []persistence.Task{t1, tOtherOwner},
				Progress: &GetTaskProgress{
					Range:       Range{InclusiveMinTaskKey: upper, ExclusiveMaxTaskKey: upper},
					NextTaskKey: upper,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl, func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn("shadow")
			})
			setBounds(r, tc.lower, tc.upper)
			tc.setupMocks(deps.mockBase, deps.mockQueue)

			resp, err := r.GetTask(context.Background(), tc.req)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantResp, resp)
		})
	}
}

func TestFindMismatchesInShadow(t *testing.T) {
	now := time.Now()
	t1 := newTask(1, now.Add(10*time.Minute))
	t2 := newTask(2, now.Add(20*time.Minute))
	t3 := newTask(3, now.Add(30*time.Minute))
	// Tasks with taskID in rangeID=1 range (1<<20 .. 2<<20-1) at RangeSizeBits=20.
	// The default mock rangeID is 0, so these will be classified as OwnerChanged.
	tOtherRange1 := newTask(1<<20, now.Add(10*time.Minute))
	tOtherRange2 := newTask(2<<20, now.Add(20*time.Minute))

	info := func(t persistence.Task) shadowMismatchTaskInfo { return toShadowMismatchTaskInfo(t) }

	tests := []struct {
		name         string
		snapshotResp *GetTaskResponse
		dbResp       *GetTaskResponse
		wantResult   findMismatchesInShadowResult
	}{
		{
			name:         "no mismatches",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1, t2}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, t2}},
			wantResult:   findMismatchesInShadowResult{HasMismatches: false, CacheTaskCount: 2, DBTaskCount: 2},
		},
		{
			name:         "empty on both sides",
			snapshotResp: &GetTaskResponse{},
			dbResp:       &GetTaskResponse{},
			wantResult:   findMismatchesInShadowResult{HasMismatches: false},
		},
		{
			name:         "task missing from cache: same rangeID → real mismatch",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, t2}},
			wantResult: findMismatchesInShadowResult{
				MissedInCacheTasks: []shadowMismatchTaskInfo{info(t2)},
				HasMismatches:      true,
				CacheTaskCount:     1,
				DBTaskCount:        2,
			},
		},
		{
			name:         "task missing from cache: different rangeID → owner changed, not a mismatch",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, tOtherRange1}},
			wantResult: findMismatchesInShadowResult{
				OwnerChangedTasks:    []shadowMismatchTaskInfo{info(tOtherRange1)},
				OwnerChangedRangeIDs: []int64{1},
				HasMismatches:        false,
				CacheTaskCount:       1,
				DBTaskCount:          2,
			},
		},
		{
			name:         "mixed: same-range missing (mismatch) and different-range missing (owner change)",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, t2, tOtherRange1}},
			wantResult: findMismatchesInShadowResult{
				MissedInCacheTasks:   []shadowMismatchTaskInfo{info(t2)},
				OwnerChangedTasks:    []shadowMismatchTaskInfo{info(tOtherRange1)},
				OwnerChangedRangeIDs: []int64{1},
				HasMismatches:        true,
				CacheTaskCount:       1,
				DBTaskCount:          3,
			},
		},
		{
			name:         "all db tasks from different range: no mismatch, all owner changed",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, tOtherRange1, tOtherRange2}},
			wantResult: findMismatchesInShadowResult{
				OwnerChangedTasks:    []shadowMismatchTaskInfo{info(tOtherRange1), info(tOtherRange2)},
				OwnerChangedRangeIDs: []int64{1, 2},
				HasMismatches:        false,
				CacheTaskCount:       1,
				DBTaskCount:          3,
			},
		},
		{
			name:         "extra task in cache",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1, t2, t3}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, t2}},
			wantResult: findMismatchesInShadowResult{
				ExtraInCacheTasks: []shadowMismatchTaskInfo{info(t3)},
				HasMismatches:     true,
				CacheTaskCount:    3,
				DBTaskCount:       2,
			},
		},
		{
			name:         "missing and extra simultaneously",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1, t3}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1, t2}},
			wantResult: findMismatchesInShadowResult{
				MissedInCacheTasks: []shadowMismatchTaskInfo{info(t2)},
				ExtraInCacheTasks:  []shadowMismatchTaskInfo{info(t3)},
				HasMismatches:      true,
				CacheTaskCount:     2,
				DBTaskCount:        2,
			},
		},
		{
			// Cache task has nanosecond timestamp; DB task has the same time truncated to ms.
			// They should compare as equal after truncation.
			name: "timestamp sub-millisecond jitter: no mismatch",
			snapshotResp: &GetTaskResponse{
				Tasks: []persistence.Task{newTask(1, now.Add(10*time.Minute+500*time.Nanosecond))},
			},
			dbResp: &GetTaskResponse{
				Tasks: []persistence.Task{newTask(1, now.Add(10*time.Minute))},
			},
			wantResult: findMismatchesInShadowResult{HasMismatches: false, CacheTaskCount: 1, DBTaskCount: 1},
		},
		{
			// NextTaskKey is not compared: Cassandra may return a non-empty cursor on the last page,
			// causing DB to report lastTask.Next() while cache reports exclusiveMaxTaskKey.
			// Differing NextTaskKey must not be flagged as a mismatch.
			name: "NextTaskKey differs: no mismatch (Cassandra cursor artifact)",
			snapshotResp: &GetTaskResponse{
				Tasks:    []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{NextTaskKey: newTimeKey(now.Add(time.Hour))},
			},
			dbResp: &GetTaskResponse{
				Tasks:    []persistence.Task{t1, t2},
				Progress: &GetTaskProgress{NextTaskKey: newTimeKey(now.Add(2 * time.Hour))},
			},
			wantResult: findMismatchesInShadowResult{HasMismatches: false, CacheTaskCount: 2, DBTaskCount: 2},
		},
		{
			name: "task ID present in both but scheduled time differs → IncorrectTimeTasks",
			snapshotResp: &GetTaskResponse{
				Tasks: []persistence.Task{newTask(1, now.Add(10*time.Minute))},
			},
			dbResp: &GetTaskResponse{
				Tasks: []persistence.Task{newTask(1, now.Add(11*time.Minute))},
			},
			wantResult: findMismatchesInShadowResult{
				IncorrectTimeTasks: []shadowTimeMismatch{
					toShadowTimeMismatch(
						newTask(1, now.Add(11*time.Minute)),
						now.Add(11*time.Minute).Truncate(persistence.DBTimestampMinPrecision),
						now.Add(10*time.Minute).Truncate(persistence.DBTimestampMinPrecision),
					),
				},
				HasMismatches:  true,
				CacheTaskCount: 1,
				DBTaskCount:    1,
			},
		},
		{
			name:         "extra task in cache: different rangeID → owner changed, not a mismatch",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1, tOtherRange1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1}},
			wantResult: findMismatchesInShadowResult{
				OwnerChangedTasks:    []shadowMismatchTaskInfo{info(tOtherRange1)},
				OwnerChangedRangeIDs: []int64{1},
				HasMismatches:        false,
				CacheTaskCount:       2,
				DBTaskCount:          1,
			},
		},
		{
			name:         "extra task in cache: mixed same-range (mismatch) and different-range (owner change)",
			snapshotResp: &GetTaskResponse{Tasks: []persistence.Task{t1, t3, tOtherRange1}},
			dbResp:       &GetTaskResponse{Tasks: []persistence.Task{t1}},
			wantResult: findMismatchesInShadowResult{
				ExtraInCacheTasks:    []shadowMismatchTaskInfo{info(t3)},
				OwnerChangedTasks:    []shadowMismatchTaskInfo{info(tOtherRange1)},
				OwnerChangedRangeIDs: []int64{1},
				HasMismatches:        true,
				CacheTaskCount:       3,
				DBTaskCount:          1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, _ := setupMocksForCachedQueueReader(t, ctrl)
			got := r.findMismatchesInShadow(tc.snapshotResp, tc.dbResp)
			slices.Sort(got.OwnerChangedRangeIDs)
			require.Equal(t, tc.wantResult, got)
		})
	}
}

func TestCachedQueueReader_LookAHead(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))

	task1 := newTask(1, now.Add(10*time.Minute))

	tests := []struct {
		name        string
		mode        string
		lastRangeID int64
		initLower   persistence.HistoryTaskKey
		initUpper   persistence.HistoryTaskKey
		minKey      persistence.HistoryTaskKey
		setupMocks  func(base *MockQueueReader, queue *MockInMemQueue)
		wantErr     bool
		wantResp    *LookAHeadResponse
	}{
		{
			name:      "disabled falls back to DB",
			mode:      "disabled",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: persistence.MinimumHistoryTaskKey,
			minKey:    lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				// disabled returns before acquiring the lock, so no queue.Len()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{}, nil)
			},
			wantResp: &LookAHeadResponse{},
		},
		{
			name:      "disabled falls back to DB: error propagated",
			mode:      "disabled",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: persistence.MinimumHistoryTaskKey,
			minKey:    lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				// disabled returns before acquiring the lock, so no queue.Len()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantResp: nil,
			wantErr:  true,
		},
		{
			// shadow bypasses cache entirely — same early return as disabled.
			name:      "shadow: bypasses cache, delegates to DB",
			mode:      "shadow",
			initLower: lower,
			initUpper: upper,
			minKey:    lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				// returns before acquiring the lock, so no queue.Len()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{Task: task1}, nil)
			},
			wantResp: &LookAHeadResponse{Task: task1},
		},
		{
			name:      "miss: min key before lower bound falls back to DB",
			mode:      "enabled",
			initLower: newTimeKey(now.Add(time.Minute)),
			initUpper: upper,
			minKey:    lower, // lower < initLower → miss
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{}, nil)
			},
			wantResp: &LookAHeadResponse{},
		},
		{
			name:      "miss: min key at upper bound (exclusive) falls back to DB",
			mode:      "enabled",
			initLower: lower,
			initUpper: upper,
			minKey:    upper, // upper is exclusive → isTaskCovered false → miss
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{}, nil)
			},
			wantResp: &LookAHeadResponse{},
		},
		{
			name:      "hit: task found in window",
			mode:      "enabled",
			initLower: lower,
			initUpper: upper,
			minKey:    lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().LookAHead(lower).Return(task1)
			},
			wantResp: &LookAHeadResponse{
				Task:             task1,
				LookAheadMaxTime: upper.GetScheduledTime(),
			},
		},
		{
			name:      "hit: no task in window",
			mode:      "enabled",
			initLower: lower,
			initUpper: upper,
			minKey:    lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().LookAHead(lower).Return(nil)
			},
			wantResp: &LookAHeadResponse{
				LookAheadMaxTime: upper.GetScheduledTime(),
			},
		},
		{
			name:        "rangeID changed: clears cache and falls back to base",
			mode:        "enabled",
			lastRangeID: 1,
			initLower:   lower,
			initUpper:   upper,
			minKey:      lower,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				queue.EXPECT().Clear()
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{
					LookAheadMaxTime: upper.GetScheduledTime(),
				}, nil)
			},
			wantResp: &LookAHeadResponse{
				LookAheadMaxTime: upper.GetScheduledTime(),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl, func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)
			})
			base, queue := deps.mockBase, deps.mockQueue
			setBounds(r, tc.initLower, tc.initUpper)
			r.lastRangeID = tc.lastRangeID
			tc.setupMocks(base, queue)

			resp, err := r.LookAHead(context.Background(), &LookAHeadRequest{InclusiveMinTaskKey: tc.minKey})

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantResp, resp)
		})
	}
}

func TestCachedQueueReader_Prefetch(t *testing.T) {
	// These constants mirror testOptions defaults so test expectations stay in sync.
	const (
		maxSize     = 100
		lookAhead   = time.Hour
		evictWindow = time.Minute
	)

	// Fixed reference time; each sub-test's mock clock is advanced to this value so all
	// precomputed keys are consistent with what prefetch() observes via q.clock.Now().
	now := time.Date(2030, 6, 1, 10, 0, 0, 0, time.UTC)

	maxKey := persistence.NewHistoryTaskKey(now.Add(lookAhead), 0)
	someLower := newTimeKey(now.Add(-30 * time.Minute))
	someUpper := newTimeKey(now.Add(30 * time.Minute))
	evictBefore := persistence.NewHistoryTaskKey(now.Add(-evictWindow), 0)
	trimKey := persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0)
	// differentUpper simulates a concurrent window shrink (e.g. Inject→RTrimBySize)
	// that happens while prefetch is waiting on the DB.
	differentUpper := newTimeKey(now.Add(15 * time.Minute))

	t1 := newTask(1, now.Add(5*time.Minute))
	t2 := newTask(2, now.Add(10*time.Minute))
	t3 := newTask(3, now.Add(35*time.Minute))
	t4 := newTask(4, now.Add(40*time.Minute))

	tests := []struct {
		name         string
		optsOverride func(*cachedQueueReaderOptions)
		initLower    persistence.HistoryTaskKey
		initUpper    persistence.HistoryTaskKey
		initBuffer   []persistence.Task
		setupMocks   func(base *MockQueueReader, queue *MockInMemQueue, r *cachedQueueReader)
		wantErr      bool
		wantLower    persistence.HistoryTaskKey
		wantUpper    persistence.HistoryTaskKey
	}{
		{
			name: "disabled: no-op, bounds unchanged",
			optsOverride: func(o *cachedQueueReaderOptions) {
				o.Mode = dynamicproperties.GetStringPropertyFn("disabled")
			},
			initLower:  persistence.MinimumHistoryTaskKey,
			initUpper:  persistence.MinimumHistoryTaskKey,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {},
			wantLower:  persistence.MinimumHistoryTaskKey,
			wantUpper:  persistence.MinimumHistoryTaskKey,
		},
		{
			name:      "cache full: skips DB fetch, bounds unchanged",
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(maxSize).AnyTimes()
			},
			wantLower: someLower,
			wantUpper: someUpper,
		},
		{
			name:      "DB error: returns error, bounds unchanged",
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr:   true,
			wantLower: someLower,
			wantUpper: someUpper,
		},
		{
			name:      "clear cache, first prefetch, no tasks",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: persistence.MinimumHistoryTaskKey,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    nil,
					Progress: &GetTaskProgress{NextTaskKey: maxKey},
				}, nil)
			},
			wantLower: evictBefore,
			wantUpper: maxKey,
		},
		{
			name:      "clear cache, first prefetch, tasks returned",
			initLower: persistence.MinimumHistoryTaskKey,
			initUpper: persistence.MinimumHistoryTaskKey,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    []persistence.Task{t1, t2},
					Progress: &GetTaskProgress{NextTaskKey: maxKey},
				}, nil)
				queue.EXPECT().PutTasks([]persistence.Task{t1, t2})
				queue.EXPECT().RTrimBySize(maxSize).Return(persistence.MinimumHistoryTaskKey, false)
			},
			wantLower: evictBefore,
			wantUpper: maxKey,
		},
		{
			name:      "subsequent prefetch, no tasks",
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    nil,
					Progress: &GetTaskProgress{NextTaskKey: maxKey},
				}, nil)
			},
			wantLower: someLower,
			wantUpper: maxKey,
		},
		{
			name:      "subsequent prefetch, tasks returned, tasks inserted",
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    []persistence.Task{t3, t4},
					Progress: &GetTaskProgress{NextTaskKey: t4.GetTaskKey().Next()},
				}, nil)
				queue.EXPECT().PutTasks([]persistence.Task{t3, t4})
				queue.EXPECT().RTrimBySize(maxSize).Return(persistence.MinimumHistoryTaskKey, false)
			},
			wantLower: someLower,
			wantUpper: t4.GetTaskKey().Next(),
		},
		{
			name: "subsequent prefetch, tasks returned, tasks inserted, trimmed by size",
			optsOverride: func(o *cachedQueueReaderOptions) {
				o.MaxSize = dynamicproperties.GetIntPropertyFn(1)
			},
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    []persistence.Task{t3},
					Progress: &GetTaskProgress{NextTaskKey: maxKey},
				}, nil)
				queue.EXPECT().LTrim(gomock.Any()).Do(func(key persistence.HistoryTaskKey) {
					assert.Equal(t, evictBefore.Compare(key), 0)
				})
				queue.EXPECT().PutTasks([]persistence.Task{t3})
				queue.EXPECT().RTrimBySize(1).Return(trimKey, true)
			},
			wantLower: evictBefore,
			wantUpper: trimKey,
		},
		{
			name:      "gap detected: upper changed during fetch, returns error, bounds unchanged",
			initLower: someLower,
			initUpper: someUpper,
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, r *cachedQueueReader) {
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
						// Simulate concurrent Inject→RTrimBySize shrinking the window.
						r.mu.Lock()
						r.exclusiveUpperBound = differentUpper
						r.mu.Unlock()
						return &GetTaskResponse{
							Tasks:    nil,
							Progress: &GetTaskProgress{NextTaskKey: maxKey},
						}, nil
					},
				)
			},
			wantErr:   true,
			wantLower: someLower,
			wantUpper: differentUpper,
		},
		{
			// A task buffered in pendingInjectBuffer (arrived while prefetch was in-flight)
			// must be drained into the cache once the prefetch completes and the upper
			// bound advances to cover it.
			name:      "buffered task drained into cache after prefetch completes",
			initLower: someLower,
			initUpper: someUpper,
			initBuffer: []persistence.Task{
				// scheduled at someUpper+1min; will fall within [someUpper, maxKey) after prefetch
				newTask(99, someUpper.GetScheduledTime().Add(time.Minute)),
			},
			setupMocks: func(base *MockQueueReader, queue *MockInMemQueue, _ *cachedQueueReader) {
				tBuf := newTask(99, someUpper.GetScheduledTime().Add(time.Minute))
				queue.EXPECT().Len().Return(0).AnyTimes()
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
					Tasks:    nil,
					Progress: &GetTaskProgress{NextTaskKey: maxKey},
				}, nil)
				// insertBufferedTasks drains tBuf after upper advances to maxKey.
				queue.EXPECT().PutTasks([]persistence.Task{tBuf})
				queue.EXPECT().RTrimBySize(maxSize).Return(persistence.MinimumHistoryTaskKey, false)
			},
			wantLower: someLower,
			wantUpper: maxKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var overrides []func(*cachedQueueReaderOptions)
			if tc.optsOverride != nil {
				overrides = append(overrides, tc.optsOverride)
			}
			ctrl := gomock.NewController(t)
			r, deps := setupMocksForCachedQueueReader(t, ctrl, overrides...)
			base, queue, clk := deps.mockBase, deps.mockQueue, deps.clock
			clk.Advance(now.Sub(clk.Now())) // sync mock clock to the fixed reference time

			setBounds(r, tc.initLower, tc.initUpper)
			if len(tc.initBuffer) > 0 {
				r.mu.Lock()
				r.pendingInjectBuffer = append(r.pendingInjectBuffer, tc.initBuffer...)
				r.mu.Unlock()
			}
			tc.setupMocks(base, queue, r)

			err := r.prefetch()

			r.mu.RLock()
			gotLower := r.inclusiveLowerBound
			gotUpper := r.exclusiveUpperBound
			r.mu.RUnlock()

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.True(t, gotLower.Equal(tc.wantLower), "lower: got %v want %v", gotLower, tc.wantLower)
			assert.True(t, gotUpper.Equal(tc.wantUpper), "upper: got %v want %v", gotUpper, tc.wantUpper)
		})
	}
}

func TestCachedQueueReader_StartStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	r, deps := setupMocksForCachedQueueReader(t, ctrl)
	queue := deps.mockQueue
	queue.EXPECT().Len().Return(0).AnyTimes()

	r.Start()
	r.Stop()

	// Idempotent
	r.Start()
	r.Stop()
}

func TestCachedQueueReader_NextPrefetchDelay(t *testing.T) {
	ctrl := gomock.NewController(t)
	// 50% jitter: un-capped range would be [0.5*base, 1.5*base].
	// The cap (min(delay, jittered)) keeps the result in [0.5*base, base],
	// and the MinPrefetchInterval floor ensures the result is never below 1s.
	r, deps := setupMocksForCachedQueueReader(t, ctrl, func(o *cachedQueueReaderOptions) {
		o.PrefetchTriggerWindow = dynamicproperties.GetDurationPropertyFn(10 * time.Minute)
		o.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(time.Second)
		o.PrefetchJitterCoefficient = dynamicproperties.GetFloatPropertyFn(0.5)
	})
	now := deps.clock.Now()

	tests := []struct {
		name                string
		exclusiveUpperBound persistence.HistoryTaskKey
		wantMinDelay        time.Duration
		wantMaxDelay        time.Duration
	}{
		{
			name:                "no upper bound -> clamped to MinPrefetchInterval",
			exclusiveUpperBound: persistence.MinimumHistoryTaskKey,
			wantMinDelay:        time.Second,
			wantMaxDelay:        time.Second,
		},
		{
			name:                "upper in the past -> clamped to MinPrefetchInterval",
			exclusiveUpperBound: newTimeKey(now.Add(-5 * time.Minute)), // triggerTime = now-15m -> d<0 -> min
			wantMinDelay:        time.Second,
			wantMaxDelay:        time.Second,
		},
		{
			name:                "upper within trigger window -> clamped to MinPrefetchInterval",
			exclusiveUpperBound: newTimeKey(now.Add(5 * time.Minute)), // triggerTime = now-5m -> d<=0 -> min
			wantMinDelay:        time.Second,
			wantMaxDelay:        time.Second,
		},
		{
			name:                "upper exactly at trigger boundary -> clamped to MinPrefetchInterval",
			exclusiveUpperBound: newTimeKey(now.Add(10 * time.Minute)), // triggerTime = now -> d=0 -> min
			wantMinDelay:        time.Second,
			wantMaxDelay:        time.Second,
		},
		{
			// base delay = 10m; jitter range = [5m, 15m]; cap keeps it in [5m, 10m].
			// Without the cap, positive jitter could return up to 15m, causing the prefetch
			// to fire 5m after the upper bound and producing cache misses.
			name:                "upper beyond trigger window -> jitter capped at base delay",
			exclusiveUpperBound: newTimeKey(now.Add(20 * time.Minute)), // triggerTime = now+10m -> base=10m
			wantMinDelay:        5 * time.Minute,                       // 0.5 * base (negative jitter floor)
			wantMaxDelay:        10 * time.Minute,                      // base delay — the cap that prevents late prefetch
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r.exclusiveUpperBound = tc.exclusiveUpperBound
			d := r.nextPrefetchDelay()
			assert.GreaterOrEqual(t, d, tc.wantMinDelay)
			assert.LessOrEqual(t, d, tc.wantMaxDelay)
		})
	}
}

func TestCachedQueueReader_IsRangeCovered(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))

	tests := []struct {
		name         string
		inclusiveMin persistence.HistoryTaskKey
		exclusiveMax persistence.HistoryTaskKey
		wantCovered  bool
	}{
		{
			name:         "exact window",
			inclusiveMin: lower,
			exclusiveMax: upper,
			wantCovered:  true,
		},
		{
			name:         "sub-range inside window",
			inclusiveMin: newTimeKey(now.Add(10 * time.Minute)),
			exclusiveMax: newTimeKey(now.Add(30 * time.Minute)),
			wantCovered:  true,
		},
		{
			name:         "starts before lower bound",
			inclusiveMin: newTimeKey(now.Add(-time.Minute)),
			exclusiveMax: upper,
			wantCovered:  false,
		},
		{
			name:         "ends after upper bound",
			inclusiveMin: lower,
			exclusiveMax: newTimeKey(now.Add(2 * time.Hour)),
			wantCovered:  false,
		},
	}

	ctrl := gomock.NewController(t)
	r, _ := setupMocksForCachedQueueReader(t, ctrl)
	setBounds(r, lower, upper)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantCovered, r.isRangeCovered(tc.inclusiveMin, tc.exclusiveMax),
				"test %q: covered", tc.name)
		})
	}
}

func TestCachedQueueReader_IsTaskCovered(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))

	tests := []struct {
		name string
		key  persistence.HistoryTaskKey
		want bool
	}{
		{
			name: "key before lower bound",
			key:  newTimeKey(now.Add(-time.Minute)),
			want: false,
		},
		{
			name: "key at lower bound (inclusive)",
			key:  lower,
			want: true,
		},
		{
			name: "key inside window",
			key:  newTimeKey(now.Add(30 * time.Minute)),
			want: true,
		},
		{
			name: "key at upper bound (exclusive)",
			key:  upper,
			want: false,
		},
		{
			name: "key beyond upper bound",
			key:  newTimeKey(now.Add(2 * time.Hour)),
			want: false,
		},
	}

	ctrl := gomock.NewController(t)
	r, _ := setupMocksForCachedQueueReader(t, ctrl)
	setBounds(r, lower, upper)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, r.isTaskCovered(tc.key))
		})
	}
}

func TestCachedQueueReader_IsToBufferTask(t *testing.T) {
	now := time.Now()
	lower := newTimeKey(now)
	upper := newTimeKey(now.Add(time.Hour))
	prefetchTarget := newTimeKey(now.Add(2 * time.Hour))

	tests := []struct {
		name                string
		prefetchTargetUpper persistence.HistoryTaskKey
		key                 persistence.HistoryTaskKey
		want                bool
	}{
		{
			name:                "no prefetch in-flight (prefetchTargetUpper is minimum)",
			prefetchTargetUpper: persistence.MinimumHistoryTaskKey,
			key:                 newTimeKey(now.Add(90 * time.Minute)),
			want:                false,
		},
		{
			name:                "key inside covered window (below upper bound)",
			prefetchTargetUpper: prefetchTarget,
			key:                 newTimeKey(now.Add(30 * time.Minute)),
			want:                false,
		},
		{
			name:                "key at upper bound (start of buffer window, inclusive)",
			prefetchTargetUpper: prefetchTarget,
			key:                 upper,
			want:                true,
		},
		{
			name:                "key inside buffer window",
			prefetchTargetUpper: prefetchTarget,
			key:                 newTimeKey(now.Add(90 * time.Minute)),
			want:                true,
		},
		{
			name:                "key at prefetch target (exclusive)",
			prefetchTargetUpper: prefetchTarget,
			key:                 prefetchTarget,
			want:                false,
		},
		{
			name:                "key beyond prefetch target",
			prefetchTargetUpper: prefetchTarget,
			key:                 newTimeKey(now.Add(3 * time.Hour)),
			want:                false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, _ := setupMocksForCachedQueueReader(t, ctrl)
			setBounds(r, lower, upper)
			r.prefetchTargetUpper = tc.prefetchTargetUpper

			assert.Equal(t, tc.want, r.isToBufferTask(tc.key))
		})
	}
}
