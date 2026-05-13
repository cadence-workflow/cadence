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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

// counterScope is a minimal metrics.Scope that tracks IncCounter, RecordTimer, and RecordHistogramValue calls for testing.
type counterScope struct {
	metrics.Scope
	mu         sync.Mutex
	counters   map[metrics.MetricIdx]int
	timers     map[metrics.MetricIdx][]time.Duration
	histograms map[metrics.MetricIdx][]float64
}

func newCounterScope() *counterScope {
	return &counterScope{
		Scope:      metrics.NoopScope,
		counters:   make(map[metrics.MetricIdx]int),
		timers:     make(map[metrics.MetricIdx][]time.Duration),
		histograms: make(map[metrics.MetricIdx][]float64),
	}
}

func (s *counterScope) IncCounter(id metrics.MetricIdx) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[id]++
}

func (s *counterScope) RecordTimer(id metrics.MetricIdx, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timers[id] = append(s.timers[id], d)
}

func (s *counterScope) RecordHistogramValue(id metrics.MetricIdx, v float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.histograms[id] = append(s.histograms[id], v)
}

func (s *counterScope) count(id metrics.MetricIdx) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counters[id]
}

func (s *counterScope) lastTimer(id metrics.MetricIdx) (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vals := s.timers[id]
	if len(vals) == 0 {
		return 0, false
	}
	return vals[len(vals)-1], true
}

func (s *counterScope) lastHistogram(id metrics.MetricIdx) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vals := s.histograms[id]
	if len(vals) == 0 {
		return 0, false
	}
	return vals[len(vals)-1], true
}

func defaultTestOptions() *cachedQueueReaderOptions {
	return &cachedQueueReaderOptions{
		Mode:                      dynamicproperties.GetStringPropertyFn("enabled"),
		MaxSize:                   dynamicproperties.GetIntPropertyFn(1000),
		MaxLookAheadWindow:        dynamicproperties.GetDurationPropertyFn(5 * time.Minute),
		PrefetchTriggerWindow:     dynamicproperties.GetDurationPropertyFn(1 * time.Minute),
		PrefetchPageSize:          dynamicproperties.GetIntPropertyFn(100),
		TimeEvictionWindow:        dynamicproperties.GetDurationPropertyFn(30 * time.Second),
		MinPrefetchInterval:       dynamicproperties.GetDurationPropertyFn(0), // no minimum in tests
		PrefetchJitterCoefficient: dynamicproperties.GetFloatPropertyFn(0),    // no jitter in tests
	}
}

func newTestCachedReader(t *testing.T, opts *cachedQueueReaderOptions, ts clock.TimeSource) (*cachedQueueReader, *MockQueueReader) {
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	if opts == nil {
		opts = defaultTestOptions()
	}
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	return r, base
}

func TestCachedQueueReader_UpdateInclusiveLowerBound(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		lowerBound     persistence.HistoryTaskKey
		upperBound     persistence.HistoryTaskKey
		newKey         persistence.HistoryTaskKey
		wantLowerBound persistence.HistoryTaskKey
		wantQueueLen   int
		seedTasks      []persistence.Task
	}{
		{
			name:           "advances when newKey is farther forward",
			lowerBound:     persistence.MinimumHistoryTaskKey,
			upperBound:     persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0),
			newKey:         persistence.NewHistoryTaskKey(now, 0),
			wantLowerBound: persistence.NewHistoryTaskKey(now, 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(-1*time.Minute), 1),
				newTestTask(now.Add(1*time.Minute), 2),
			},
			wantQueueLen: 1,
		},
		{
			name:           "no-op when newKey is not farther forward",
			lowerBound:     persistence.NewHistoryTaskKey(now, 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0),
			newKey:         persistence.NewHistoryTaskKey(now.Add(-5*time.Minute), 0),
			wantLowerBound: persistence.NewHistoryTaskKey(now, 0),
			seedTasks:      []persistence.Task{newTestTask(now.Add(1*time.Minute), 1)},
			wantQueueLen:   1,
		},
		{
			name:           "caps at exclusiveUpperBound when newKey exceeds it",
			lowerBound:     persistence.MinimumHistoryTaskKey,
			upperBound:     persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			newKey:         persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			wantLowerBound: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			wantQueueLen:   0,
		},
		{
			name:           "no cap when upper bound is MinimumHistoryTaskKey sentinel",
			lowerBound:     persistence.MinimumHistoryTaskKey,
			upperBound:     persistence.MinimumHistoryTaskKey,
			newKey:         persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			wantLowerBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			wantQueueLen:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			r, _ := newTestCachedReader(t, nil, ts)
			r.inclusiveLowerBound = tc.lowerBound
			r.exclusiveUpperBound = tc.upperBound
			if tc.seedTasks != nil {
				r.queue.PutTasks(tc.seedTasks)
			}

			r.mu.Lock()
			r.advanceInclusiveLowerBound(tc.newKey)
			r.mu.Unlock()

			assert.Equal(t, tc.wantLowerBound, r.inclusiveLowerBound)
			assert.Equal(t, tc.wantQueueLen, r.queue.Len())
		})
	}
}

func TestCachedQueueReader_UpdateReadLevel(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)

	tests := []struct {
		name               string
		evictionSafeWindow time.Duration
		initialLowerBound  persistence.HistoryTaskKey
		readLevel          persistence.HistoryTaskKey
		seedTasks          []persistence.Task
		wantLowerBound     persistence.HistoryTaskKey
		wantQueueLen       int
	}{
		{
			name:               "advances to readLevel when it exceeds time eviction",
			evictionSafeWindow: 30 * time.Second,
			initialLowerBound:  persistence.MinimumHistoryTaskKey,
			readLevel:          persistence.NewHistoryTaskKey(now, 0), // now > now-30s
			seedTasks: []persistence.Task{
				newTestTask(now.Add(-1*time.Minute), 1),
				newTestTask(now.Add(-10*time.Second), 2),
				newTestTask(now.Add(1*time.Minute), 3),
			},
			wantLowerBound: persistence.NewHistoryTaskKey(now, 0),
			wantQueueLen:   1, // only task at now+1m survives
		},
		{
			name:               "no-op when all inputs are below current lower bound",
			evictionSafeWindow: 30 * time.Second,
			initialLowerBound:  persistence.NewHistoryTaskKey(now, 0),
			readLevel:          persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(1*time.Minute), 1),
			},
			// readLevel (now-1m) is not greater than initialLowerBound (now), so no advance occurs.
			wantLowerBound: persistence.NewHistoryTaskKey(now, 0),
			wantQueueLen:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultTestOptions()
			opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(tc.evictionSafeWindow)

			r, _ := newTestCachedReader(t, opts, ts)
			r.inclusiveLowerBound = tc.initialLowerBound

			// Seed the queue with tasks.
			r.queue.PutTasks(tc.seedTasks)

			r.UpdateReadLevel(tc.readLevel)

			assert.Equal(t, tc.wantLowerBound, r.inclusiveLowerBound, "inclusiveLowerBound mismatch")
			assert.Equal(t, tc.wantQueueLen, r.queue.Len(), "queue length mismatch")
		})
	}
}

// TestCachedQueueReader_LeftEvict_Invariants verifies that UpdateReadLevel never
// sets inclusiveLowerBound above exclusiveUpperBound and correctly handles the
// MaximumHistoryTaskKey sentinel.
func TestCachedQueueReader_UpdateReadLevel_Invariants(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)

	tests := []struct {
		name               string
		evictionSafeWindow time.Duration
		initialLowerBound  persistence.HistoryTaskKey
		upperBound         persistence.HistoryTaskKey // exclusiveUpperBound to pre-set
		readLevel          persistence.HistoryTaskKey
		wantLowerBound     persistence.HistoryTaskKey
	}{
		{
			name:               "readLevel exceeding upper bound is capped at upper bound",
			evictionSafeWindow: 30 * time.Second,
			initialLowerBound:  persistence.MinimumHistoryTaskKey,
			upperBound:         persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			readLevel:          persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			// readLevel > upper, so newLowerBound must be capped at upper.
			wantLowerBound: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
		},
		{
			name:               "readLevel above upper bound is capped then advances from current lower",
			evictionSafeWindow: 30 * time.Second,
			initialLowerBound:  persistence.NewHistoryTaskKey(now.Add(30*time.Second), 0),
			upperBound:         persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			readLevel:          persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			// readLevel (now+2m) exceeds upper (now+1m), so it is capped to now+1m.
			// now+1m > initialLowerBound (now+30s), so the lower bound advances.
			wantLowerBound: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := defaultTestOptions()
			opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(tc.evictionSafeWindow)

			r, _ := newTestCachedReader(t, opts, ts)
			r.inclusiveLowerBound = tc.initialLowerBound
			r.exclusiveUpperBound = tc.upperBound

			r.UpdateReadLevel(tc.readLevel)

			assert.Equal(t, tc.wantLowerBound, r.inclusiveLowerBound, "inclusiveLowerBound mismatch")
			// Invariant: lower must never exceed upper (when upper is initialised).
			if tc.upperBound.Compare(persistence.MinimumHistoryTaskKey) != 0 {
				assert.True(t,
					r.inclusiveLowerBound.Compare(r.exclusiveUpperBound) <= 0,
					"invariant violated: inclusiveLowerBound (%v) > exclusiveUpperBound (%v)",
					r.inclusiveLowerBound, r.exclusiveUpperBound,
				)
			}
		})
	}
}

// TestCachedQueueReader_BoundaryInvariants verifies that the lower/upper boundary
// invariant (lower <= upper) is maintained across combinations of putTasks and
// UpdateReadLevel calls. The invariant only applies when exclusiveUpperBound has
// been initialized (i.e., is not the MinimumHistoryTaskKey sentinel). In normal
// operation, exclusiveUpperBound is set explicitly by the prefetch loop after
// putTasks — direct putTasks calls only update it on size-cap trim. Tests that
// require a meaningful upper bound set it explicitly via initialUpperBound.
func TestCachedQueueReader_BoundaryInvariants(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	type step struct {
		// one of: putTasks, updateReadLevel
		op        string
		tasks     []persistence.Task
		readLevel persistence.HistoryTaskKey
	}

	// checkInvariant only enforces lower <= upper when upper is initialized.
	// MinimumHistoryTaskKey is the "not yet prefetched" sentinel and is exempt.
	checkInvariant := func(t *testing.T, r *cachedQueueReader, label string) {
		t.Helper()
		if r.exclusiveUpperBound.Equal(persistence.MinimumHistoryTaskKey) {
			return // upper not yet initialized; invariant does not apply
		}
		assert.True(t,
			r.inclusiveLowerBound.Compare(r.exclusiveUpperBound) <= 0,
			"%s: invariant violated: inclusiveLowerBound (%v) > exclusiveUpperBound (%v)",
			label, r.inclusiveLowerBound, r.exclusiveUpperBound,
		)
	}

	tests := []struct {
		name              string
		initialUpperBound persistence.HistoryTaskKey // set before steps; MinimumHistoryTaskKey means "use default"
		steps             []step
	}{
		{
			// putTasks doesn't change upper unless trim occurs; upper stays at MinimumHistoryTaskKey.
			// UpdateReadLevel after that should not break anything.
			name:              "putTasks (no trim) then updateReadLevel advances lower freely",
			initialUpperBound: persistence.MinimumHistoryTaskKey,
			steps: []step{
				{op: "putTasks", tasks: []persistence.Task{newTestTask(now.Add(1*time.Minute), 1)}},
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(30*time.Second), 0)},
			},
		},
		{
			// With upper bound set, UpdateReadLevel is capped at upper.
			name:              "updateReadLevel beyond upper bound is capped",
			initialUpperBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			steps: []step{
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0)},
			},
		},
		{
			// Lower advances within the upper cap.
			name:              "multiple updateReadLevel calls stay within upper bound",
			initialUpperBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			steps: []step{
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)},
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(3*time.Minute), 0)},
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(7*time.Minute), 0)},
			},
		},
		{
			// putTasks that causes trim shrinks upper; lower must remain <= new upper.
			name:              "putTasks with trim updates upper and lower stays within",
			initialUpperBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			steps: []step{
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)},
				// putTasks with maxSize=2 will trim the 3rd task and update upper to task2.key.Next()
				{op: "putTasks", tasks: []persistence.Task{
					newTestTask(now.Add(2*time.Minute), 1),
					newTestTask(now.Add(3*time.Minute), 2),
					newTestTask(now.Add(4*time.Minute), 3), // will be trimmed (maxSize=2)
				}},
			},
		},
		{
			// MaximumHistoryTaskKey is normalized to MinimumHistoryTaskKey, then
			// advanceLowerBound is called with min — a no-op since min is never greater
			// than the current lower bound. The invariant (lower <= upper) still holds.
			name:              "MaximumHistoryTaskKey readLevel with initialized upper bound",
			initialUpperBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			steps: []step{
				{op: "updateReadLevel", readLevel: persistence.MaximumHistoryTaskKey},
			},
		},
		{
			// Interleaved: lower advances, putTasks trims to a new upper, lower stays valid.
			name:              "interleaved updateReadLevel and putTasks with trim",
			initialUpperBound: persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0),
			steps: []step{
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)},
				// trim: maxSize=2, so task3 is dropped and upper shrinks
				{op: "putTasks", tasks: []persistence.Task{
					newTestTask(now.Add(3*time.Minute), 1),
					newTestTask(now.Add(5*time.Minute), 2),
					newTestTask(now.Add(7*time.Minute), 3),
				}},
				{op: "updateReadLevel", readLevel: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			opts := defaultTestOptions()
			opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(30 * time.Second)
			// Use a small maxSize so putTasks trim is easily triggered.
			opts.MaxSize = dynamicproperties.GetIntPropertyFn(2)

			r, _ := newTestCachedReader(t, opts, ts)

			// Pre-initialize upper bound if the test requires it.
			if !tc.initialUpperBound.Equal(persistence.MinimumHistoryTaskKey) {
				r.exclusiveUpperBound = tc.initialUpperBound
			}
			checkInvariant(t, r, "initial state")

			for i, s := range tc.steps {
				var label string
				switch s.op {
				case "putTasks":
					r.mu.Lock()
					r.putTasks(s.tasks)
					r.mu.Unlock()
					label = "after putTasks"
				case "updateReadLevel":
					r.UpdateReadLevel(s.readLevel)
					label = "after updateReadLevel"
				}
				checkInvariant(t, r, "step "+string(rune('0'+i))+" "+label)
			}
		})
	}
}

func TestCachedQueueReader_Inject(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		mode         string
		lowerBound   persistence.HistoryTaskKey
		upperBound   persistence.HistoryTaskKey
		tasks        []persistence.Task
		wantQueueLen int
	}{
		{
			name:         "disabled mode is a no-op",
			mode:         "disabled",
			lowerBound:   persistence.MinimumHistoryTaskKey,
			upperBound:   persistence.NewHistoryTaskKey(startTime.Add(1*time.Hour), 0),
			tasks:        []persistence.Task{newTestTask(startTime, 1)},
			wantQueueLen: 0,
		},
		{
			name:         "enabled mode inserts tasks",
			mode:         "enabled",
			lowerBound:   persistence.MinimumHistoryTaskKey,
			upperBound:   persistence.NewHistoryTaskKey(startTime.Add(1*time.Hour), 0),
			tasks:        []persistence.Task{newTestTask(startTime, 1), newTestTask(startTime.Add(time.Second), 2)},
			wantQueueLen: 2,
		},
		{
			name:         "shadow mode inserts tasks",
			mode:         "shadow",
			lowerBound:   persistence.MinimumHistoryTaskKey,
			upperBound:   persistence.NewHistoryTaskKey(startTime.Add(1*time.Hour), 0),
			tasks:        []persistence.Task{newTestTask(startTime, 1)},
			wantQueueLen: 1,
		},
		{
			// Tasks outside the cached window [lower, upper) are silently skipped;
			// the prefetch loop will load them when the window advances to cover them.
			name:       "tasks outside covered range are filtered out",
			mode:       "enabled",
			lowerBound: persistence.NewHistoryTaskKey(startTime.Add(-1*time.Minute), 0),
			upperBound: persistence.NewHistoryTaskKey(startTime.Add(5*time.Minute), 0),
			tasks: []persistence.Task{
				newTestTask(startTime.Add(-2*time.Minute), 1), // before lower bound — filtered
				newTestTask(startTime.Add(1*time.Minute), 2),  // within range — kept
				newTestTask(startTime.Add(10*time.Minute), 3), // after upper bound — filtered
			},
			wantQueueLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(startTime)
			opts := defaultTestOptions()
			opts.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)

			r, _ := newTestCachedReader(t, opts, ts)
			r.inclusiveLowerBound = tc.lowerBound
			r.exclusiveUpperBound = tc.upperBound

			r.Inject(tc.tasks)

			assert.Equal(t, tc.wantQueueLen, r.queue.Len(), "queue length mismatch")
		})
	}
}

// TestCachedQueueReader_Inject_SentinelTaskID verifies that tasks with
// taskID=0 are rejected by Inject and never enter the cache or the pending
// inject buffer. taskID=0 is reserved for range-boundary sentinel keys and
// must not corrupt queue ordering.
func TestCachedQueueReader_Inject_SentinelTaskID(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r, _ := newTestCachedReader(t, opts, ts)

	// Covered path: window is wide, sentinel would pass isTaskCovered.
	// putTasks filters it today, but Inject should reject it before putTasks.
	r.inclusiveLowerBound = persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)

	r.Inject([]persistence.Task{newTestTask(now.Add(1*time.Minute), 0)})
	assert.Equal(t, 0, r.queue.Len(), "sentinel task (taskID=0) must not enter the cache")

	// Pending-buffer path: narrow the covered window so the sentinel falls in
	// the pending prefetch range instead of the covered range.
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now, 0)
	r.prefetchTargetUpper = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)

	r.Inject([]persistence.Task{newTestTask(now.Add(1*time.Minute), 0)})
	assert.Empty(t, r.pendingInjectBuffer, "sentinel task must not enter the pending buffer")
}

func TestCachedQueueReader_Clear(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	r, _ := newTestCachedReader(t, opts, ts)

	// Pre-populate all state that Clear should wipe.
	r.mu.Lock()
	r.inclusiveLowerBound = persistence.NewHistoryTaskKey(now.Add(-time.Minute), 0)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)
	r.prefetchTargetUpper = persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0)
	r.pendingInjectBuffer = []persistence.Task{newTestTask(now.Add(time.Minute), 1)}
	r.queue.PutTasks([]persistence.Task{newTestTask(now.Add(time.Minute), 1)})
	r.mu.Unlock()

	savedLower := persistence.NewHistoryTaskKey(now.Add(-time.Minute), 0)

	r.Clear()

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.Equal(t, 0, r.queue.Len(), "queue must be empty after Clear")
	assert.Empty(t, r.pendingInjectBuffer, "pending buffer must be empty after Clear")
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.prefetchTargetUpper, "prefetchTargetUpper must be reset")
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.exclusiveUpperBound, "exclusiveUpperBound must be reset")
	assert.Equal(t, savedLower, r.inclusiveLowerBound, "inclusiveLowerBound must not change")
	assert.Equal(t, 1, len(r.prefetchCh), "prefetch must be notified")
}

func TestCachedQueueReader_NewFromShard(t *testing.T) {
	// Exercises newCachedQueueReader (the production constructor that reads from shard config).
	// Calls each option closure to verify they're wired correctly.
	ctrl := gomock.NewController(t)
	mockShard := shard.NewTestContext(
		t, ctrl,
		&persistence.ShardInfo{ShardID: 10, RangeID: 1, TransferAckLevel: 0},
		config.NewForTest(),
	)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()

	r := newCachedQueueReader(base, queue, mockShard, metrics.NoopScope)
	require.NotNil(t, r)

	// Call each option closure to cover the closure bodies.
	assert.NotEmpty(t, r.options.Mode())
	assert.Greater(t, r.options.MaxSize(), 0)
	assert.Greater(t, r.options.MaxLookAheadWindow(), time.Duration(0))
	assert.Greater(t, r.options.PrefetchTriggerWindow(), time.Duration(0))
	assert.Greater(t, r.options.PrefetchPageSize(), 0)
	assert.Greater(t, r.options.TimeEvictionWindow(), time.Duration(0))
	assert.GreaterOrEqual(t, r.options.MinPrefetchInterval(), time.Duration(0))
}

func TestCachedQueueReader_StartStop(t *testing.T) {
	t.Parallel()
	t.Run("Start sets startTime and creates context", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
		ts := clock.NewMockedTimeSourceAt(now)
		r, _ := newTestCachedReader(t, nil, ts)

		r.Start()

		require.NotNil(t, r.ctx)
		require.NotNil(t, r.cancel)

		// Context should not be done yet.
		select {
		case <-r.ctx.Done():
			t.Fatal("context should not be done after Start")
		default:
		}

		r.Stop()
	})

	t.Run("Stop cancels context", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
		ts := clock.NewMockedTimeSourceAt(now)
		r, _ := newTestCachedReader(t, nil, ts)

		r.Start()
		ctx := r.ctx
		r.Stop()

		select {
		case <-ctx.Done():
			// expected
		default:
			t.Fatal("context should be done after Stop")
		}
	})
}

func TestCachedQueueReader_GetTaskDelegatesToBase(t *testing.T) {
	t.Parallel()
	ts := clock.NewMockedTimeSource()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("disabled")
	r, base := newTestCachedReader(t, opts, ts)

	expectedResp := &GetTaskResponse{
		Tasks: []persistence.Task{newTestTask(time.Now(), 1)},
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(expectedResp, nil)

	resp, err := r.GetTask(context.Background(), &GetTaskRequest{})
	require.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
}

func TestCachedQueueReader_LookAHead_Disabled(t *testing.T) {
	t.Parallel()
	ts := clock.NewMockedTimeSource()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("disabled")

	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	expectedResp := &LookAHeadResponse{
		Task: newTestTask(time.Now(), 1),
	}
	base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(expectedResp, nil)

	resp, err := r.LookAHead(context.Background(), &LookAHeadRequest{})
	require.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
}

// TestCachedQueueReader_LookAHead_Shadow verifies that shadow mode always delegates
// to the base reader, not the cache.
func TestCachedQueueReader_LookAHead_Shadow(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)

	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("shadow")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	// Set upper bound and seed tasks to prove the cache is NOT consulted.
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)
	queue.PutTasks([]persistence.Task{newTestTask(now.Add(30*time.Second), 42)})

	baseResp := &LookAHeadResponse{Task: newTestTask(now.Add(1*time.Minute), 99)}
	base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(baseResp, nil)

	resp, err := r.LookAHead(context.Background(), &LookAHeadRequest{
		InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
	})
	require.NoError(t, err)
	assert.Equal(t, baseResp, resp, "shadow mode must return base result, not cache result")
}

func TestCachedQueueReader_LookAHead_Enabled(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		mode              string
		upperBound        persistence.HistoryTaskKey
		lowerBound        persistence.HistoryTaskKey // zero value → use default MinimumHistoryTaskKey
		reqMinTaskKey     persistence.HistoryTaskKey
		seedTasks         []persistence.Task
		wantTaskID        int64 // 0 means nil task expected
		wantLookAheadTime time.Time
		setupBase         func(base *MockQueueReader) *LookAHeadResponse
	}{
		{
			name:          "covered and task found: returns task and capturedUpperBound time",
			mode:          "enabled",
			upperBound:    persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 42),
			},
			wantTaskID:        42,
			wantLookAheadTime: now.Add(5 * time.Minute),
		},
		{
			name:              "covered and no task: returns nil task with capturedUpperBound time, no DB fallback",
			mode:              "enabled",
			upperBound:        persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqMinTaskKey:     persistence.NewHistoryTaskKey(now, 0),
			seedTasks:         nil, // empty cache
			wantTaskID:        0,
			wantLookAheadTime: now.Add(5 * time.Minute),
		},
		{
			name:          "not covered: falls through to base",
			mode:          "enabled",
			upperBound:    persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			reqMinTaskKey: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0), // beyond upper bound
			seedTasks:     nil,
			setupBase: func(base *MockQueueReader) *LookAHeadResponse {
				resp := &LookAHeadResponse{Task: newTestTask(now.Add(3*time.Minute), 99)}
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
		{
			// req.InclusiveMinTaskKey == exclusiveUpperBound: the upper bound is exclusive,
			// so the cache has no tasks at or after this key. Must delegate to the DB.
			name:          "req equals upper bound: falls through to base (off-by-one)",
			mode:          "enabled",
			upperBound:    persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			reqMinTaskKey: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0), // == upper bound
			seedTasks:     nil,
			setupBase: func(base *MockQueueReader) *LookAHeadResponse {
				resp := &LookAHeadResponse{Task: newTestTask(now.Add(2*time.Minute), 55)}
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
		{
			// Shadow mode always delegates to base regardless of cache coverage.
			name:          "shadow mode always delegates to base",
			mode:          "shadow",
			upperBound:    persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 42),
			},
			setupBase: func(base *MockQueueReader) *LookAHeadResponse {
				resp := &LookAHeadResponse{Task: newTestTask(now.Add(30*time.Second), 42)}
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
		{
			// reqMinTaskKey is below inclusiveLowerBound — time-based eviction may
			// have advanced the lower bound past the caller's min key. The cache
			// must not claim coverage; it must fall through to the DB because it
			// has evicted the tasks the caller is looking for.
			name:       "req below lower bound: falls through to base",
			mode:       "enabled",
			upperBound: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			// Simulate time eviction advancing the lower bound to now; any request
			// with InclusiveMinTaskKey < now must fall through to the DB.
			lowerBound:    persistence.NewHistoryTaskKey(now, 0),
			reqMinTaskKey: persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0), // below lower bound
			seedTasks:     nil,
			setupBase: func(base *MockQueueReader) *LookAHeadResponse {
				resp := &LookAHeadResponse{Task: newTestTask(now, 77)}
				base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			ctrl := gomock.NewController(t)
			base := NewMockQueueReader(ctrl)
			queue := newInMemQueue()
			opts := defaultTestOptions()
			opts.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)

			r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
			r.exclusiveUpperBound = tc.upperBound
			if !tc.lowerBound.Equal(persistence.MinimumHistoryTaskKey) {
				r.inclusiveLowerBound = tc.lowerBound
			}

			if tc.seedTasks != nil {
				queue.PutTasks(tc.seedTasks)
			}

			var expectedResp *LookAHeadResponse
			if tc.setupBase != nil {
				expectedResp = tc.setupBase(base)
			}

			req := &LookAHeadRequest{
				InclusiveMinTaskKey: tc.reqMinTaskKey,
			}

			resp, err := r.LookAHead(context.Background(), req)
			require.NoError(t, err)

			if tc.setupBase != nil {
				assert.Equal(t, expectedResp, resp)
			} else {
				if tc.wantTaskID == 0 {
					assert.Nil(t, resp.Task, "expected nil task")
				} else {
					require.NotNil(t, resp.Task)
					assert.Equal(t, tc.wantTaskID, resp.Task.GetTaskID())
				}
				assert.Equal(t, tc.wantLookAheadTime, resp.LookAheadMaxTime)
			}
		})
	}
}

func TestCachedQueueReader_GetTask_Disabled(t *testing.T) {
	t.Parallel()
	ts := clock.NewMockedTimeSource()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("disabled")

	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	scope := newCounterScope()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)

	expectedResp := &GetTaskResponse{
		Tasks: []persistence.Task{newTestTask(time.Now(), 1)},
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(expectedResp, nil)

	resp, err := r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.MinimumHistoryTaskKey,
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(time.Now().Add(time.Hour), 0),
			},
			NextTaskKey: persistence.MinimumHistoryTaskKey,
		},
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Equal(t, 0, scope.count(metrics.CachedQueueHitsCounter))
	assert.Equal(t, 0, scope.count(metrics.CachedQueueMissesCounter))
}

func TestCachedQueueReader_GetTask_Enabled(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	// all-pass predicate
	allPass := func(ctrl *gomock.Controller) Predicate {
		p := NewMockPredicate(ctrl)
		p.EXPECT().Check(gomock.Any()).Return(true).AnyTimes()
		return p
	}

	tests := []struct {
		name           string
		lowerBound     persistence.HistoryTaskKey
		upperBound     persistence.HistoryTaskKey
		reqNextTaskKey persistence.HistoryTaskKey
		reqExclMax     persistence.HistoryTaskKey
		seedTasks      []persistence.Task
		pageSize       int
		wantTaskIDs    []int64
		wantNextKey    persistence.HistoryTaskKey
		wantHits       int
		wantMisses     int
		setupBase      func(base *MockQueueReader) *GetTaskResponse
	}{
		{
			name:           "cache hit returns tasks from cache",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
				newTestTask(now.Add(60*time.Second), 2),
			},
			pageSize:    10,
			wantTaskIDs: []int64{1, 2},
			wantNextKey: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0), // ExclusiveMaxTaskKey since no truncation
			wantHits:    1,
			wantMisses:  0,
		},
		{
			name:           "cache hit with PageSize truncation",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(10*time.Second), 1),
				newTestTask(now.Add(20*time.Second), 2),
				newTestTask(now.Add(30*time.Second), 3),
			},
			pageSize:    2,
			wantTaskIDs: []int64{1, 2},
			wantNextKey: newTestTask(now.Add(20*time.Second), 2).GetTaskKey().Next(),
			wantHits:    1,
			wantMisses:  0,
		},
		{
			name:           "cache hit with empty result is not a miss",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			seedTasks:      nil, // no tasks in cache at all
			pageSize:       10,
			wantTaskIDs:    nil,
			wantNextKey:    persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			wantHits:       1,
			wantMisses:     0,
		},
		{
			// exclusiveMax == exclusiveUpperBound is covered because !exclusiveMax.Greater(upper) is satisfied.
			name:           "cache hit when exclusiveMax equals exclusiveUpperBound",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0), // == upper bound
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
			},
			pageSize:    10,
			wantTaskIDs: []int64{1},
			wantNextKey: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			wantHits:    1,
			wantMisses:  0,
		},
		{
			name:           "cache miss lower bound violation",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0), // before lower bound
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			pageSize:       10,
			wantHits:       0,
			wantMisses:     1,
			setupBase: func(base *MockQueueReader) *GetTaskResponse {
				resp := &GetTaskResponse{Tasks: []persistence.Task{newTestTask(now.Add(30*time.Second), 99)}}
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
		{
			name:           "cache miss upper bound violation",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0), // beyond upper bound
			pageSize:       10,
			wantHits:       0,
			wantMisses:     1,
			setupBase: func(base *MockQueueReader) *GetTaskResponse {
				resp := &GetTaskResponse{Tasks: []persistence.Task{newTestTask(now.Add(30*time.Second), 99)}}
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)
				return resp
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			ctrl := gomock.NewController(t)
			base := NewMockQueueReader(ctrl)
			queue := newInMemQueue()
			scope := newCounterScope()
			opts := defaultTestOptions()
			opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

			r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)
			r.inclusiveLowerBound = tc.lowerBound
			r.exclusiveUpperBound = tc.upperBound

			if tc.seedTasks != nil {
				queue.PutTasks(tc.seedTasks)
			}

			var expectedResp *GetTaskResponse
			if tc.setupBase != nil {
				expectedResp = tc.setupBase(base)
			}

			req := &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: tc.reqNextTaskKey,
						ExclusiveMaxTaskKey: tc.reqExclMax,
					},
					NextTaskKey: tc.reqNextTaskKey,
				},
				Predicate: allPass(ctrl),
				PageSize:  tc.pageSize,
			}

			resp, err := r.GetTask(context.Background(), req)
			require.NoError(t, err)

			if tc.setupBase != nil {
				// cache miss: expect base result
				assert.Equal(t, expectedResp, resp)
			} else {
				// cache hit: verify returned tasks
				var gotIDs []int64
				for _, task := range resp.Tasks {
					gotIDs = append(gotIDs, task.GetTaskID())
				}
				assert.Equal(t, tc.wantTaskIDs, gotIDs, "task IDs mismatch")
				assert.Equal(t, tc.wantNextKey, resp.Progress.NextTaskKey, "NextTaskKey mismatch")
			}

			assert.Equal(t, tc.wantHits, scope.count(metrics.CachedQueueHitsCounter), "hits counter")
			assert.Equal(t, tc.wantMisses, scope.count(metrics.CachedQueueMissesCounter), "misses counter")
		})
	}
}

func TestCachedQueueReader_Prefetch(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		mode              string
		initialUpperBound persistence.HistoryTaskKey
		baseTasks         []persistence.Task // one page only — prefetch makes a single GetTask call
		wantQueueLen      int
		wantUpperBound    persistence.HistoryTaskKey
		wantQueueSizeEmit bool
		wantErr           bool
	}{
		{
			name:              "prefetch fetches tasks and advances upper bound",
			mode:              "enabled",
			initialUpperBound: persistence.MinimumHistoryTaskKey,
			baseTasks: []persistence.Task{
				newTestTask(now.Add(1*time.Minute), 1),
				newTestTask(now.Add(2*time.Minute), 2),
			},
			wantQueueLen:      2,
			wantUpperBound:    persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			wantQueueSizeEmit: true,
			wantErr:           false,
		},
		{
			name:              "disabled mode skips prefetch entirely",
			mode:              "disabled",
			initialUpperBound: persistence.MinimumHistoryTaskKey,
			baseTasks:         nil, // base should not be called
			wantQueueLen:      0,
			wantUpperBound:    persistence.MinimumHistoryTaskKey,
			wantQueueSizeEmit: false,
			wantErr:           false,
		},
		{
			name:              "shadow mode prefetches",
			mode:              "shadow",
			initialUpperBound: persistence.MinimumHistoryTaskKey,
			baseTasks: []persistence.Task{
				newTestTask(now.Add(1*time.Minute), 10),
			},
			wantQueueLen:      1,
			wantUpperBound:    persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			wantQueueSizeEmit: true,
			wantErr:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			ctrl := gomock.NewController(t)
			base := NewMockQueueReader(ctrl)
			queue := newInMemQueue()
			scope := newCounterScope()
			opts := defaultTestOptions()
			opts.Mode = dynamicproperties.GetStringPropertyFn(tc.mode)

			r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)
			r.exclusiveUpperBound = tc.initialUpperBound

			// Set up base mock expectation for the single prefetch page.
			if tc.baseTasks != nil {
				resp := &GetTaskResponse{
					Tasks: tc.baseTasks,
					Progress: &GetTaskProgress{
						NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
					},
				}
				base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)
			}

			err := r.prefetch()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantQueueLen, r.queue.Len(), "queue length")
			assert.Equal(t, tc.wantUpperBound, r.exclusiveUpperBound, "exclusiveUpperBound")

			if tc.wantQueueSizeEmit {
				v, ok := scope.lastHistogram(metrics.CachedQueueSizeHistogram)
				assert.True(t, ok, "expected queue size metric emission")
				assert.Equal(t, float64(tc.wantQueueLen), v, "queue size metric value")
			}
		})
	}
}

// TestCachedQueueReader_PrefetchFullPage verifies that when the DB returns a
// full page (len == pageSize), the upper bound advances only to the next task
// key — not to the window ceiling. This prevents the cache from falsely
// claiming coverage for tasks that weren't fetched yet.
func TestCachedQueueReader_PrefetchFullPage(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	opts.PrefetchPageSize = dynamicproperties.GetIntPropertyFn(2) // page size = 2

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	task1 := newTestTask(now.Add(1*time.Minute), 1)
	task2 := newTestTask(now.Add(2*time.Minute), 2)
	wantNextKey := task2.GetTaskKey().Next()

	// Return exactly pageSize tasks — signals that there are likely more.
	resp := &GetTaskResponse{
		Tasks: []persistence.Task{task1, task2},
		Progress: &GetTaskProgress{
			NextTaskKey: wantNextKey,
		},
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)

	err := r.prefetch()
	assert.NoError(t, err)

	// Upper bound must stop at the next task key, not the window ceiling.
	assert.Equal(t, wantNextKey, r.exclusiveUpperBound,
		"full page: upper bound should stop at NextTaskKey, not the window ceiling")
	assert.Equal(t, 2, r.queue.Len())
}

func TestCachedQueueReader_PrefetchPartialPage(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	opts.PrefetchPageSize = dynamicproperties.GetIntPropertyFn(10) // page size = 10

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	// Return fewer tasks than pageSize — means the window is fully scanned.
	resp := &GetTaskResponse{
		Tasks: []persistence.Task{
			newTestTask(now.Add(1*time.Minute), 1),
			newTestTask(now.Add(2*time.Minute), 2),
		},
		Progress: &GetTaskProgress{
			NextTaskKey: newTestTask(now.Add(2*time.Minute), 2).GetTaskKey().Next(),
		},
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)

	err := r.prefetch()
	assert.NoError(t, err)

	// Partial page: upper bound must advance to the window ceiling.
	wantCeiling := persistence.NewHistoryTaskKey(now.Add(opts.MaxLookAheadWindow()), 0)
	assert.Equal(t, wantCeiling, r.exclusiveUpperBound,
		"partial page: upper bound should advance to the window ceiling")
}

func TestCachedQueueReader_PrefetchRTrims(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	scope := newCounterScope()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(2) // only allow 2 tasks

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)

	// Return 3 tasks but max size is 2
	resp := &GetTaskResponse{
		Tasks: []persistence.Task{
			newTestTask(now.Add(1*time.Minute), 1),
			newTestTask(now.Add(2*time.Minute), 2),
			newTestTask(now.Add(3*time.Minute), 3),
		},
		Progress: &GetTaskProgress{},
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(resp, nil)

	r.prefetch()

	assert.Equal(t, 2, r.queue.Len(), "queue should be trimmed to max size")
}

// TestCachedQueueReader_StartStop_Idempotent verifies that double-Start and
// double-Stop are both safe no-ops on the second call.
func TestCachedQueueReader_StartStop_Idempotent(t *testing.T) {
	defer goleak.VerifyNone(t)
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	r, _ := newTestCachedReader(t, nil, ts)

	r.Start()
	ts.BlockUntil(1) // both loops registered their timers
	r.Start()        // second Start — must be a no-op

	r.Stop()
	r.Stop() // second Stop — must be a no-op
}

func TestCachedQueueReader_PrefetchLoopNotifyPrefetch(t *testing.T) {
	defer goleak.VerifyNone(t)
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	// Large MinPrefetchInterval ensures the timer doesn't fire immediately
	// after being reset inside the prefetchCh branch.
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(10 * time.Minute)

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	r.Start()
	defer r.Stop()

	// Wait for both loops to register their timers.
	ts.BlockUntil(1)

	// Trigger the prefetchCh branch by calling notifyPrefetch directly.
	r.notifyPrefetch()

	// The timer is reset inside the prefetchCh case; BlockUntil(2) confirms
	// both timers are re-registered (prefetch timer re-armed, evict still waiting).
	ts.BlockUntil(1)
}

func TestCachedQueueReader_PrefetchLoopStops(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	r.Start()

	// Wait for the timer to be created in the goroutine
	ts.BlockUntil(1)

	// Stop should cancel the goroutine and return without hanging
	r.Stop()
}

func TestCachedQueueReader_PrefetchLoopFires(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	scope := newCounterScope()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)

	// Set up base to return tasks on the first prefetch call.
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
		Tasks: []persistence.Task{
			newTestTask(now.Add(1*time.Minute), 1),
		},
		Progress: &GetTaskProgress{},
	}, nil)

	r.Start()
	defer r.Stop()

	// Wait for the timer to be registered, then advance past the initial delay.
	ts.BlockUntil(1)
	ts.Advance(time.Millisecond)

	// Poll until the prefetch goroutine finishes inserting into the queue.
	// (The GetTask return and the putTasks call are not atomic, so a channel
	// closed on GetTask return would race with the queue insert.)
	assert.Eventually(t, func() bool {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.queue.Len() == 1
	}, 5*time.Second, time.Millisecond, "queue should have 1 task after prefetch fires")
}

func TestCachedQueueReader_PrefetchBaseError(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	// Base returns an error
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	err := r.prefetch()
	assert.Error(t, err)

	// Queue should remain empty, upper bound should not advance
	assert.Equal(t, 0, r.queue.Len(), "queue should be empty after error")
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.exclusiveUpperBound, "upper bound should not advance after error")
}

func TestCachedQueueReader_PrefetchNoSpace(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(2)

	r, _ := newTestCachedReader(t, opts, ts)
	// Fill cache to capacity
	r.queue.PutTasks([]persistence.Task{
		newTestTask(now.Add(1*time.Minute), 1),
		newTestTask(now.Add(2*time.Minute), 2),
	})

	result := r.prefetch()
	// Cache full is not an error — returns nil so the loop uses nextPrefetchDelay.
	assert.NoError(t, result)
}

// TestCachedQueueReader_FirstPrefetchAdvancesLowerBound verifies that after the
// first prefetch (when upperBound was MinimumHistoryTaskKey), inclusiveLowerBound
// is advanced to the prefetch anchor (now - TimeEvictionWindow). Without this,
// historical tasks from a previous shard owner pass isRangeCovered (lower =
// MinimumHistoryTaskKey), the cache returns 0, and the tasks are permanently
// skipped at shard takeover.
func TestCachedQueueReader_FirstPrefetchAdvancesLowerBound(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	evictionWindow := 10 * time.Second
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(evictionWindow)

	r, base := newTestCachedReader(t, opts, ts)
	// First prefetch: upperBound is MinimumHistoryTaskKey (nothing fetched yet).
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.exclusiveUpperBound)

	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
		Tasks: []persistence.Task{newTestTask(now.Add(1*time.Minute), 1)},
		Progress: &GetTaskProgress{
			NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
		},
	}, nil)

	err := r.prefetch()
	require.NoError(t, err)

	// Lower bound must be anchored to now - TimeEvictionWindow, not left at MinimumHistoryTaskKey.
	expectedLower := persistence.NewHistoryTaskKey(now.Add(-evictionWindow), 0)
	assert.Equal(t, expectedLower, r.inclusiveLowerBound,
		"first prefetch must set inclusiveLowerBound to the fetch anchor (now - TimeEvictionWindow) "+
			"so historical tasks from the previous shard owner correctly miss the cache")

	// Consequence: a request for tasks before the anchor is NOT covered,
	// so it falls through to DB rather than returning 0 from cache.
	r.mu.RLock()
	oldTaskKey := persistence.NewHistoryTaskKey(now.Add(-20*time.Second), 0)
	covered := r.isRangeCovered(oldTaskKey, persistence.NewHistoryTaskKey(now.Add(-15*time.Second), 0))
	r.mu.RUnlock()
	assert.False(t, covered, "tasks from before the fetch anchor must not be reported as covered")
}

// putTasks triggers RTrimBySize (because concurrent Inject calls filled the cache
// between the capacity snapshot and the write-lock), the prefetch does NOT
// re-advance exclusiveUpperBound past the trimmed point. Re-advancing would claim
// coverage for tasks that were just evicted, causing GetTask to return fewer tasks
// than exist in the DB for that range.
func TestCachedQueueReader_PrefetchTrimPreventsUpperBoundAdvance(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	opts := defaultTestOptions()
	// MaxSize=2: capacity snapshot sees 2 free slots, fetches 1 DB task.
	// During the DB round-trip, inject adds 2 tasks that fill the cache.
	// putTasks then tries to add the DB task → 3 tasks > MaxSize=2 → RTrim.
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(2)

	// inject tasks arrive concurrently (during DB fetch, below prevUpper).
	injectTask1 := newTestTask(now.Add(30*time.Second), 1)
	injectTask2 := newTestTask(now.Add(45*time.Second), 2)
	// DB task is fetched from [prevUpper, exclusiveMaxKey).
	dbTask := newTestTask(now.Add(2*time.Minute), 3)
	// prevUpper falls between inject tasks and dbTask.
	prevUpper := persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)

	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	r.exclusiveUpperBound = prevUpper

	// During the DB round-trip, two inject tasks arrive and fill the cache.
	// We bypass Inject() (which would check the window) and write directly to
	// the queue to simulate a concurrent Inject that completed just in time.
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			r.mu.Lock()
			r.queue.PutTasks([]persistence.Task{injectTask1, injectTask2})
			r.mu.Unlock()
			return &GetTaskResponse{
				Tasks:    []persistence.Task{dbTask},
				Progress: &GetTaskProgress{NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)},
			}, nil
		},
	)

	err := r.prefetch()
	assert.NoError(t, err)

	// RTrim kept the 2 oldest tasks (inject tasks) and dropped dbTask.
	// exclusiveUpperBound must be injectTask2.Next() — NOT advanced to exclusiveMaxKey.
	assert.Equal(t, 2, r.queue.Len(), "both inject tasks should be kept by RTrim")
	wantUpper := injectTask2.GetTaskKey().Next()
	assert.Equal(t, wantUpper, r.exclusiveUpperBound,
		"upper bound must not advance past the trimmed point after concurrent inject caused RTrim")
	// Sanity check: trimmed bound is below prevUpper, confirming the fix fired.
	assert.True(t, r.exclusiveUpperBound.Less(prevUpper),
		"trimmed upper bound should be less than prevUpper")
}

// TestCachedQueueReader_PrefetchTrimRaisesUpperBoundAdvance verifies the
// complementary case: when RTrimBySize keeps some prefetched tasks (raising
// exclusiveUpperBound above prevUpper) but drops the newest ones, the prefetch
// must NOT re-advance to NextTaskKey or exclusiveMaxKey. Doing so would claim
// coverage for the dropped tasks, causing GetTask hits to miss them.
func TestCachedQueueReader_PrefetchTrimRaisesUpperBoundAbovePrevUpper(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	opts := defaultTestOptions()
	// MaxSize=3. prevUpper = now+1min. DB returns 3 tasks (now+2m, now+3m, now+4m).
	// During the DB round-trip, inject adds 1 inject task (now+30s, below prevUpper).
	// After putTasks: queue = inject(30s) + db(2m) + db(3m) + db(4m) = 4 tasks > MaxSize=3.
	// RTrimBySize(3) keeps 3 oldest: inject(30s), db(2m), db(3m); drops db(4m).
	// newUpper = db(3m).Next() > prevUpper (now+1min) → !Equal fires → skip re-advance.
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(3)

	injectTask := newTestTask(now.Add(30*time.Second), 0)
	dbTask1 := newTestTask(now.Add(2*time.Minute), 1)
	dbTask2 := newTestTask(now.Add(3*time.Minute), 2)
	dbTask3 := newTestTask(now.Add(4*time.Minute), 3) // will be dropped
	prevUpper := persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)

	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	r.exclusiveUpperBound = prevUpper

	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			r.mu.Lock()
			r.queue.PutTasks([]persistence.Task{injectTask})
			r.mu.Unlock()
			nextKey := persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{dbTask1, dbTask2, dbTask3},
				Progress: &GetTaskProgress{NextTaskKey: nextKey},
			}, nil
		},
	)

	err := r.prefetch()
	assert.NoError(t, err)

	// Queue should hold the 3 oldest: injectTask, dbTask1, dbTask2. dbTask3 dropped.
	assert.Equal(t, 3, r.queue.Len())
	// Upper bound must be dbTask2.Next() — NOT advanced to NextTaskKey (now+5min).
	wantUpper := dbTask2.GetTaskKey().Next()
	assert.Equal(t, wantUpper, r.exclusiveUpperBound,
		"upper bound must not advance past trimmed point even when trim raises bound above prevUpper")
	// Confirm it is above prevUpper (the scenario the original Less check missed).
	assert.True(t, r.exclusiveUpperBound.Greater(prevUpper),
		"trimmed upper is above prevUpper — the case the Less-only guard missed")
}

func TestCachedQueueReader_PrefetchGapDetected(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0)

	// Base.GetTask modifies upper bound during fetch to simulate concurrent Inject
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			// Simulate concurrent operation changing upper bound
			r.mu.Lock()
			r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(30*time.Second), 0)
			r.mu.Unlock()
			return &GetTaskResponse{
				Tasks:    []persistence.Task{newTestTask(now.Add(2*time.Minute), 1)},
				Progress: &GetTaskProgress{},
			}, nil
		},
	)

	result := r.prefetch()
	assert.Error(t, result)
	// Cache is NOT cleared: existing tasks (none seeded here) are preserved.
	assert.Equal(t, 0, r.queue.Len(), "empty cache stays empty on gap detection")
	// Upper bound stays at the concurrently-set value, not reset to MinimumHistoryTaskKey.
	assert.Equal(t, persistence.NewHistoryTaskKey(now.Add(30*time.Second), 0), r.exclusiveUpperBound,
		"gap detection preserves the upper bound set by the concurrent operation")
}

func TestCachedQueueReader_NextPrefetchDelay(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name                string
		upperBound          persistence.HistoryTaskKey
		triggerWindow       time.Duration
		minPrefetchInterval time.Duration
		wantDelay           time.Duration
	}{
		{
			name:                "MinimumHistoryTaskKey with no upper bound returns min interval",
			upperBound:          persistence.MinimumHistoryTaskKey,
			triggerWindow:       30 * time.Second,
			minPrefetchInterval: 0,
			wantDelay:           0,
		},
		{
			name:                "MinimumHistoryTaskKey clamped to MinPrefetchInterval",
			upperBound:          persistence.MinimumHistoryTaskKey,
			triggerWindow:       30 * time.Second,
			minPrefetchInterval: 5 * time.Second,
			wantDelay:           5 * time.Second,
		},
		{
			name:                "upper bound far in future returns computed delay",
			upperBound:          persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			triggerWindow:       30 * time.Second,
			minPrefetchInterval: 0,
			// triggerTime = now+5m - 30s = now+4m30s; delay = triggerTime - now = 4m30s
			wantDelay: 4*time.Minute + 30*time.Second,
		},
		{
			name:                "computed delay clamped to MinPrefetchInterval when small",
			upperBound:          persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			triggerWindow:       30 * time.Second,
			minPrefetchInterval: 10 * time.Minute,
			// computed = 4m30s < min=10m → clamped to 10m
			wantDelay: 10 * time.Minute,
		},
		{
			name:                "upper bound in past returns zero delay (clamped to min)",
			upperBound:          persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			triggerWindow:       30 * time.Second,
			minPrefetchInterval: 0,
			wantDelay:           0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := clock.NewMockedTimeSourceAt(now)
			opts := defaultTestOptions()
			opts.PrefetchTriggerWindow = dynamicproperties.GetDurationPropertyFn(tc.triggerWindow)
			opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(tc.minPrefetchInterval)

			r, _ := newTestCachedReader(t, opts, ts)
			r.exclusiveUpperBound = tc.upperBound

			assert.Equal(t, tc.wantDelay, r.nextPrefetchDelay())
		})
	}
}

// TestCachedQueueReader_NextPrefetchDelay_Jitter verifies that a non-zero
// PrefetchJitterCoefficient produces varied delays and that all delays fall
// within [(1-coeff)*base, (1+coeff)*base) as backoff.JitDuration guarantees.
func TestCachedQueueReader_NextPrefetchDelay_Jitter(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)

	const coeff = 0.5
	opts := defaultTestOptions()
	opts.PrefetchJitterCoefficient = dynamicproperties.GetFloatPropertyFn(coeff)
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(0)
	// PrefetchTriggerWindow = 1 min (defaultTestOptions).
	// Set upper bound so base delay = 2 min:
	//   triggerTime = now + 3min - 1min = now + 2min → delay = 2min.
	const baseDelay = 2 * time.Minute

	r, _ := newTestCachedReader(t, opts, ts)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(baseDelay+opts.PrefetchTriggerWindow()), 0)

	lo := time.Duration(float64(baseDelay) * (1 - coeff))
	hi := time.Duration(float64(baseDelay) * (1 + coeff))

	seen := make(map[time.Duration]struct{}, 20)
	for i := 0; i < 20; i++ {
		d := r.nextPrefetchDelay()
		assert.GreaterOrEqual(t, d, lo, "delay must be >= (1-coeff)*base")
		assert.Less(t, d, hi, "delay must be < (1+coeff)*base")
		seen[d] = struct{}{}
	}
	assert.Greater(t, len(seen), 1, "jitter should produce varied delays")
}

func TestCachedQueueReader_IsRangeCovered(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		lowerBound   persistence.HistoryTaskKey
		upperBound   persistence.HistoryTaskKey
		inclusiveMin persistence.HistoryTaskKey
		exclusiveMax persistence.HistoryTaskKey
		want         bool
	}{
		{
			name:         "fully covered",
			lowerBound:   persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:   persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			inclusiveMin: persistence.NewHistoryTaskKey(now, 0),
			exclusiveMax: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			want:         true,
		},
		{
			name:         "lower bound violation",
			lowerBound:   persistence.NewHistoryTaskKey(now, 0),
			upperBound:   persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			inclusiveMin: persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			exclusiveMax: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			want:         false,
		},
		{
			name:         "upper bound violation",
			lowerBound:   persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:   persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			inclusiveMin: persistence.NewHistoryTaskKey(now, 0),
			exclusiveMax: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			want:         false,
		},
		{
			// exclusiveMax == exclusiveUpperBound is covered: !exclusiveMax.Greater(upper) is satisfied.
			name:         "exclusiveMax equals exclusiveUpperBound is covered",
			lowerBound:   persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:   persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			inclusiveMin: persistence.NewHistoryTaskKey(now, 0),
			exclusiveMax: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0), // == upper bound
			want:         true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := newTestCachedReader(t, nil, clock.NewMockedTimeSourceAt(now))
			r.inclusiveLowerBound = tc.lowerBound
			r.exclusiveUpperBound = tc.upperBound
			assert.Equal(t, tc.want, r.isRangeCovered(tc.inclusiveMin, tc.exclusiveMax))
		})
	}
}

func TestCachedQueueReader_PutTasks(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	scope := newCounterScope()
	opts := defaultTestOptions()
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(2)

	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)

	r.putTasks([]persistence.Task{
		newTestTask(now.Add(1*time.Minute), 1),
		newTestTask(now.Add(2*time.Minute), 2),
		newTestTask(now.Add(3*time.Minute), 3),
	})

	assert.Equal(t, 2, r.queue.Len(), "queue should be trimmed to max size")
	// Upper bound should be set to last retained task's key.Next()
	assert.True(t, r.exclusiveUpperBound.Compare(persistence.MinimumHistoryTaskKey) > 0)
	// Metrics emitted
	_, ok := scope.lastHistogram(metrics.CachedQueueSizeHistogram)
	assert.True(t, ok, "expected queue size metric emission")
}

// TestCachedQueueReader_PutTasks_RTrimEmptiesCache verifies that when
// RTrimBySize empties the cache (MaxSize <= 0), putTasks resets
// exclusiveUpperBound to MinimumHistoryTaskKey. Without the reset the cache
// would claim coverage over [lower, oldUpper) while holding zero tasks,
// causing GetTask hits to silently return empty results.
func TestCachedQueueReader_PutTasks_RTrimEmptiesCache(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(0) // force RTrimBySize to empty the queue

	r, _ := newTestCachedReader(t, opts, ts)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)
	r.queue.PutTasks([]persistence.Task{newTestTask(now.Add(time.Minute), 1)})

	r.mu.Lock()
	r.putTasks([]persistence.Task{newTestTask(now.Add(2*time.Minute), 2)})
	r.mu.Unlock()

	assert.Equal(t, 0, r.queue.Len(), "queue should be empty after RTrim with MaxSize=0")
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.exclusiveUpperBound,
		"upper bound must be reset to MinimumHistoryTaskKey when RTrim empties the cache")
}

// TestCachedQueueReader_UnknownMode_TreatedAsDisabled verifies that an
// TestCachedQueueReader_PutTasks_LazyEviction verifies that putTasks evicts
// tasks older than TimeEvictionWindow before inserting when the insertion
// would exceed MaxSize. This replaces the removed periodic timeEvictionLoop.
func TestCachedQueueReader_PutTasks_LazyEviction(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	evictionWindow := 30 * time.Second
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(evictionWindow)
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(3)

	r, _ := newTestCachedReader(t, opts, ts)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0)

	// Seed 2 old tasks (beyond TimeEvictionWindow) and 1 recent task.
	oldTask1 := newTestTask(now.Add(-2*time.Minute), 1)
	oldTask2 := newTestTask(now.Add(-1*time.Minute), 2)
	recentTask := newTestTask(now.Add(-10*time.Second), 3)
	r.queue.PutTasks([]persistence.Task{oldTask1, oldTask2, recentTask})
	require.Equal(t, 3, r.queue.Len(), "pre-condition: cache is full at MaxSize")

	// Inserting one more task would exceed MaxSize=3.
	// putTasks should evict tasks older than now-TimeEvictionWindow first,
	// then insert without triggering RTrim.
	newTask := newTestTask(now.Add(1*time.Minute), 4)
	trimmed := r.putTasks([]persistence.Task{newTask})
	assert.False(t, trimmed, "RTrim must not fire: eviction made room before insert")

	// Both old tasks (now-2m and now-1m) are before evictBefore=now-30s → evicted.
	// recentTask (now-10s) and newTask (now+1m) remain.
	assert.Equal(t, 2, r.queue.Len(), "old tasks evicted, recent + new task remain")

	// Lower bound advanced to now - TimeEvictionWindow.
	wantLB := persistence.NewHistoryTaskKey(now.Add(-evictionWindow), 0)
	assert.Equal(t, wantLB, r.inclusiveLowerBound,
		"lower bound must advance to now - TimeEvictionWindow after lazy eviction")
}

// unrecognised mode value causes isDisabled() to return true, so all cache
// operations fall through to the base reader rather than serving stale data.
func TestCachedQueueReader_UnknownMode_TreatedAsDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabeld") // typo — unknown mode

	base := NewMockQueueReader(ctrl)
	r, _ := newTestCachedReader(t, opts, ts)
	r.base = base

	assert.True(t, r.isDisabled(), "unknown mode must report as disabled")
	assert.False(t, r.isEnabled(), "unknown mode must not report as enabled")
	assert.False(t, r.isShadow(), "unknown mode must not report as shadow")

	// GetTask must delegate to base, not serve from cache.
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{}, nil)
	_, err := r.GetTask(context.Background(), &GetTaskRequest{Progress: &GetTaskProgress{}})
	assert.NoError(t, err)

	// LookAHead must also delegate.
	base.EXPECT().LookAHead(gomock.Any(), gomock.Any()).Return(&LookAHeadResponse{}, nil)
	_, err = r.LookAHead(context.Background(), &LookAHeadRequest{})
	assert.NoError(t, err)
}

// TestCachedQueueReader_ConcurrentUpdateReadLevel verifies that concurrent
// UpdateReadLevel calls maintain the lower <= upper invariant under the race detector.
func TestCachedQueueReader_ConcurrentUpdateReadLevel(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	opts := defaultTestOptions()
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(30 * time.Second)

	r, _ := newTestCachedReader(t, opts, ts)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(10*time.Minute), 0)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := persistence.NewHistoryTaskKey(now.Add(time.Duration(i)*time.Second), 0)
			r.UpdateReadLevel(key)
		}(i)
	}
	wg.Wait()

	r.mu.RLock()
	defer r.mu.RUnlock()
	assert.True(t,
		r.inclusiveLowerBound.Compare(r.exclusiveUpperBound) <= 0,
		"invariant violated: lower > upper after concurrent UpdateReadLevel",
	)
}

// TestCachedQueueReader_ConcurrentPrefetchAndInject exercises prefetch() and
// Inject() concurrently to catch data races under -race.
func TestCachedQueueReader_ConcurrentPrefetchAndInject(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(&GetTaskResponse{
		Tasks: nil,
		Progress: &GetTaskProgress{
			NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
		},
	}, nil).AnyTimes()

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			r.prefetch()
		}(i)
		go func(i int) {
			defer wg.Done()
			r.Inject([]persistence.Task{newTestTask(now.Add(time.Duration(i)*time.Second), int64(i))})
		}(i)
	}
	wg.Wait()
}

// TestCachedQueueReader_InjectDuringPrefetch_Buffered verifies that a task
// injected while a prefetch DB call is in-flight — and therefore rejected by
// isTaskCovered because exclusiveUpperBound has not yet advanced — is buffered
// and drained into the cache after the prefetch extends the window.
func TestCachedQueueReader_InjectDuringPrefetch_Buffered(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()

	opts := defaultTestOptions()
	opts.MaxLookAheadWindow = dynamicproperties.GetDurationPropertyFn(5 * time.Minute)
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(1 * time.Minute)
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	// Task that arrives via Inject during the DB call.
	// Its scheduledTime falls in (exclusiveUpperBound=Min, prefetchTargetUpper=now+5m).
	// The DB does NOT return it (simulating: committed to Cassandra after the DB snapshot).
	injectedTask := newTestTask(now.Add(2*time.Minute), 42)

	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			// Inject fires mid-flight, before exclusiveUpperBound is advanced.
			r.Inject([]persistence.Task{injectedTask})
			return &GetTaskResponse{
				Tasks: nil,
				Progress: &GetTaskProgress{
					NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
				},
			}, nil
		},
	)

	err := r.prefetch()
	require.NoError(t, err)

	// The task must be in cache after the buffer drain.
	assert.Equal(t, 1, r.queue.Len(),
		"task injected during prefetch DB call must be in cache after drain")

	// Confirm GetTasks finds it in the expected range.
	cacheResp := r.queue.GetTasks(&GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(3*time.Minute), 0),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.Len(t, cacheResp.Tasks, 1, "injected task must be returned by GetTasks")
	assert.Equal(t, injectedTask.GetTaskID(), cacheResp.Tasks[0].GetTaskID())
}

// TestCachedQueueReader_InjectDuringPrefetch_OutsideTarget verifies that tasks
// whose scheduledTime is beyond prefetchTargetUpper are still skipped (not buffered).
func TestCachedQueueReader_InjectDuringPrefetch_OutsideTarget(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()

	opts := defaultTestOptions()
	opts.MaxLookAheadWindow = dynamicproperties.GetDurationPropertyFn(5 * time.Minute)
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(1 * time.Minute)
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")

	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), metrics.NoopScope)

	// Task beyond the prefetch target (now+5m) — must be skipped, not buffered.
	farFutureTask := newTestTask(now.Add(10*time.Minute), 99)

	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			r.Inject([]persistence.Task{farFutureTask})
			return &GetTaskResponse{
				Tasks: nil,
				Progress: &GetTaskProgress{
					NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
				},
			}, nil
		},
	)

	err := r.prefetch()
	require.NoError(t, err)
	assert.Equal(t, 0, r.queue.Len(), "task beyond prefetchTargetUpper must not be buffered")
}

func TestFindMismatchesInShadow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	preFetchLower := persistence.NewHistoryTaskKey(now, 0)

	task := func(sec int, id int64) persistence.Task {
		return newTestTask(now.Add(time.Duration(sec)*time.Second), id)
	}
	resp := func(tasks []persistence.Task, nextSec int) *GetTaskResponse {
		return &GetTaskResponse{
			Tasks: tasks,
			Progress: &GetTaskProgress{
				NextTaskKey: persistence.NewHistoryTaskKey(now.Add(time.Duration(nextSec)*time.Second), 0),
			},
		}
	}

	tests := []struct {
		name                 string
		snapshotResp         *GetTaskResponse
		dbResp               *GetTaskResponse
		preFetchLowerBound   persistence.HistoryTaskKey
		wantMissingFromCache []int64
		wantExtraInCache     []int64
		wantNextKeyMismatch  bool
		wantHasMismatches    bool
	}{
		{
			name:               "perfect match",
			snapshotResp:       resp([]persistence.Task{task(10, 1), task(20, 2)}, 30),
			dbResp:             resp([]persistence.Task{task(10, 1), task(20, 2)}, 30),
			preFetchLowerBound: preFetchLower,
		},
		{
			name:                 "DB task missing from cache",
			snapshotResp:         resp([]persistence.Task{task(10, 1)}, 30),
			dbResp:               resp([]persistence.Task{task(10, 1), task(20, 2)}, 30),
			preFetchLowerBound:   preFetchLower,
			wantMissingFromCache: []int64{2},
			wantHasMismatches:    true,
		},
		{
			name:               "cache task missing from DB — benign but still a mismatch for observability",
			snapshotResp:       resp([]persistence.Task{task(10, 1), task(20, 2)}, 30),
			dbResp:             resp([]persistence.Task{task(10, 1)}, 30),
			preFetchLowerBound: preFetchLower,
			wantExtraInCache:   []int64{2},
			wantHasMismatches:  true, // ExtraInCache sets HasMismatches for observability
		},
		{
			name:                 "DB task missing from cache snapshot — real miss, no liveResp filter",
			snapshotResp:         resp([]persistence.Task{task(10, 1)}, 30),
			dbResp:               resp([]persistence.Task{task(10, 1), task(20, 2)}, 30),
			preFetchLowerBound:   preFetchLower,
			wantMissingFromCache: []int64{2},
			wantHasMismatches:    true,
		},
		{
			name:               "eviction — below preFetchLowerBound skipped both directions",
			snapshotResp:       resp([]persistence.Task{task(-10, 1), task(20, 2)}, 30),
			dbResp:             resp([]persistence.Task{task(20, 2)}, 30),
			preFetchLowerBound: preFetchLower,
		},
		{
			name:                "NextTaskKey mismatch — tasks match but next page differs",
			snapshotResp:        resp([]persistence.Task{task(10, 1)}, 20),
			dbResp:              resp([]persistence.Task{task(10, 1)}, 30),
			preFetchLowerBound:  preFetchLower,
			wantNextKeyMismatch: true,
			wantHasMismatches:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := findMismatchesInShadow(tc.snapshotResp, tc.dbResp, tc.preFetchLowerBound)

			if len(tc.wantMissingFromCache) == 0 {
				assert.Empty(t, result.MissingFromCache, "missingFromCache")
			} else {
				require.Len(t, result.MissingFromCache, len(tc.wantMissingFromCache))
				for i, wantID := range tc.wantMissingFromCache {
					assert.Equal(t, wantID, result.MissingFromCache[i].GetTaskID())
				}
			}

			if len(tc.wantExtraInCache) == 0 {
				assert.Empty(t, result.ExtraInCache, "extraInCache")
			} else {
				require.Len(t, result.ExtraInCache, len(tc.wantExtraInCache))
				for i, wantID := range tc.wantExtraInCache {
					assert.Equal(t, wantID, result.ExtraInCache[i].GetTaskID())
				}
			}

			assert.Equal(t, tc.wantNextKeyMismatch, result.NextKeyMismatch, "nextKeyMismatch")
			assert.Equal(t, tc.wantHasMismatches, result.HasMismatches, "hasMismatches")
		})
	}
}

func TestCachedQueueReader_GetTask_Shadow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		lowerBound      persistence.HistoryTaskKey
		upperBound      persistence.HistoryTaskKey
		reqNextTaskKey  persistence.HistoryTaskKey
		reqExclMax      persistence.HistoryTaskKey
		seedTasks       []persistence.Task
		baseTasks       []persistence.Task
		baseErr         error
		pageSize        int
		wantMismatch    int
		wantReturnedIDs []int64
		wantErr         bool
	}{
		{
			name:           "covered and matching: no mismatch",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
				newTestTask(now.Add(60*time.Second), 2),
			},
			baseTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
				newTestTask(now.Add(60*time.Second), 2),
			},
			pageSize:        10,
			wantMismatch:    0,
			wantReturnedIDs: []int64{1, 2},
		},
		{
			name:           "covered but cache missing a DB task: emits mismatch",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			seedTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
				// task 2 is missing from cache
			},
			baseTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
				newTestTask(now.Add(60*time.Second), 2),
			},
			pageSize:        10,
			wantMismatch:    1,
			wantReturnedIDs: []int64{1, 2}, // always returns base result
		},
		{
			name:           "base error skips comparison",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(-1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0),
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			baseErr:        errors.New("db error"),
			pageSize:       10,
			wantMismatch:   0,
			wantErr:        true,
		},
		{
			name:           "not covered: no comparison, returns base result",
			lowerBound:     persistence.NewHistoryTaskKey(now.Add(1*time.Minute), 0),
			upperBound:     persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			reqNextTaskKey: persistence.NewHistoryTaskKey(now, 0), // before lower bound
			reqExclMax:     persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			baseTasks: []persistence.Task{
				newTestTask(now.Add(30*time.Second), 1),
			},
			pageSize:        10,
			wantMismatch:    0,
			wantReturnedIDs: []int64{1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := clock.NewMockedTimeSourceAt(now)
			ctrl := gomock.NewController(t)
			base := NewMockQueueReader(ctrl)
			queue := newInMemQueue()
			scope := newCounterScope()
			opts := defaultTestOptions()
			opts.Mode = dynamicproperties.GetStringPropertyFn("shadow")

			r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)
			r.inclusiveLowerBound = tc.lowerBound
			r.exclusiveUpperBound = tc.upperBound

			if tc.seedTasks != nil {
				queue.PutTasks(tc.seedTasks)
			}

			pred := NewMockPredicate(ctrl)
			pred.EXPECT().Check(gomock.Any()).Return(true).AnyTimes()

			baseResp := &GetTaskResponse{Tasks: tc.baseTasks}
			base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(baseResp, tc.baseErr)

			req := &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: tc.reqNextTaskKey,
						ExclusiveMaxTaskKey: tc.reqExclMax,
					},
					NextTaskKey: tc.reqNextTaskKey,
				},
				Predicate: pred,
				PageSize:  tc.pageSize,
			}

			resp, err := r.GetTask(context.Background(), req)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var gotIDs []int64
			for _, task := range resp.Tasks {
				gotIDs = append(gotIDs, task.GetTaskID())
			}
			assert.Equal(t, tc.wantReturnedIDs, gotIDs, "returned task IDs")
			assert.Equal(t, tc.wantMismatch, scope.count(metrics.CachedQueueMismatchCounter), "mismatch counter")
		})
	}
}

// ── Lifecycle tests ──────────────────────────────────────────────────────────
//
// These tests run all goroutines (prefetchLoop, timeEvictionLoop) and drive
// multi-step scenarios through clock.MockedTimeSource.
// goleak is registered before startLifecycle so Stop runs before the leak check
// (t.Cleanup is LIFO: last registered runs first).

// TestCachedQueueReader_Shadow_EvictionBetweenReads verifies that tasks evicted
// from the cache between the initial snapshot and the second cache read are not
// counted as real mismatches. Before the fix, timeEvict advancing the lower
// bound between the two reads would produce false mismatch warnings.
func TestCachedQueueReader_Shadow_EvictionBetweenReads(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	scope := newCounterScope()
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("shadow")

	// task1 is within the look-ahead window. It will be in the initial snapshot
	// but evicted (via LTrim) before the second cache read, simulating timeEvict
	// or UpdateReadLevel advancing the lower bound.
	task1 := newTestTask(now.Add(30*time.Second), 1)
	task2 := newTestTask(now.Add(2*time.Minute), 2)

	base := NewMockQueueReader(ctrl)
	queue := newInMemQueue()
	r := newCachedQueueReaderWithOptions(base, queue, opts, ts, testlogger.New(t), scope)

	// Set up cache: both tasks present, lower bound = now, upper = now+5min.
	r.inclusiveLowerBound = persistence.NewHistoryTaskKey(now, 0)
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0)
	queue.PutTasks([]persistence.Task{task1, task2})

	// DB returns both tasks. During the DB round-trip, task1 is evicted (lower
	// bound advances past it). The second cache read sees only task2.
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			// Simulate eviction: advance lower bound past task1.
			r.mu.Lock()
			r.inclusiveLowerBound = task2.GetTaskKey()
			queue.LTrim(task2.GetTaskKey())
			r.mu.Unlock()
			return &GetTaskResponse{
				Tasks: []persistence.Task{task1, task2},
				Progress: &GetTaskProgress{
					NextTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
				},
			}, nil
		},
	)

	req := &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: r.inclusiveLowerBound,
				ExclusiveMaxTaskKey: r.exclusiveUpperBound,
			},
			NextTaskKey: r.inclusiveLowerBound,
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  100,
	}

	_, err := r.GetTask(context.Background(), req)
	require.NoError(t, err)

	// task1 was evicted — its absence from the second read must not count as a
	// mismatch. The mismatch counter should remain zero.
	assert.Equal(t, 0, scope.count(metrics.CachedQueueMismatchCounter),
		"evicted task must not be counted as a shadow mismatch")
}

// dbFromTasks returns a GetTask handler that serves tasks from a fixed sorted
// slice, respecting [NextTaskKey, ExclusiveMaxTaskKey) and PageSize. A full
// page returns NextTaskKey = last task's Next(); partial returns the ceiling.
func dbFromTasks(allTasks []persistence.Task) func(context.Context, *GetTaskRequest) (*GetTaskResponse, error) {
	return func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
		var page []persistence.Task
		for _, t := range allTasks {
			k := t.GetTaskKey()
			if !k.Less(req.Progress.NextTaskKey) && k.Less(req.Progress.ExclusiveMaxTaskKey) {
				page = append(page, t)
				if len(page) >= req.PageSize {
					break
				}
			}
		}
		var nextKey persistence.HistoryTaskKey
		if len(page) >= req.PageSize {
			nextKey = page[len(page)-1].GetTaskKey().Next()
		} else {
			nextKey = req.Progress.ExclusiveMaxTaskKey
		}
		return &GetTaskResponse{
			Tasks: page,
			Progress: &GetTaskProgress{
				Range:       req.Progress.Range,
				NextTaskKey: nextKey,
			},
		}, nil
	}
}

// startLifecycle starts the reader and wires Stop + goleak as t.Cleanup in the
// correct LIFO order so Stop always runs before the goroutine leak check.
func startLifecycle(t *testing.T, r *cachedQueueReader, ts clock.MockedTimeSource) {
	t.Helper()
	t.Cleanup(func() { goleak.VerifyNone(t) }) // registered first → runs last
	t.Cleanup(r.Stop)                          // registered second → runs first
	r.Start()
	ts.BlockUntil(1)
}

// awaitPrefetch advances the mock clock by d and blocks until the prefetch
// signals completion via done, then re-waits for the prefetch goroutine timer.
func awaitPrefetch(ts clock.MockedTimeSource, d time.Duration, done <-chan struct{}) {
	ts.Advance(d)
	<-done
	ts.BlockUntil(1)
}

// lifecycleBaseOpts returns common opts for lifecycle tests: dormant prefetch
// timers so each test can control exactly when they fire.
func lifecycleBaseOpts() *cachedQueueReaderOptions {
	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(10 * time.Minute)
	// Large trigger window keeps nextPrefetchDelay clamped to MinPrefetchInterval.
	opts.PrefetchTriggerWindow = dynamicproperties.GetDurationPropertyFn(10 * time.Minute)
	return opts
}

// TestCachedQueueReader_Lifecycle_InjectFillsCache: injecting past MaxSize trims
// the newest task and shrinks the upper bound. GetTask within the bound hits the
// cache; beyond it falls back to the base reader.
func TestCachedQueueReader_Lifecycle_InjectFillsCache(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	scope := newCounterScope()

	opts := lifecycleBaseOpts()
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(2)

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), scope)
	task1 := newTestTask(now.Add(1*time.Minute), 1)

	p1 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p1)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task1},
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)
	awaitPrefetch(ts, 1*time.Millisecond, p1)

	task2 := newTestTask(now.Add(2*time.Minute), 2)
	task3 := newTestTask(now.Add(3*time.Minute), 3) // trimmed (newest, exceeds MaxSize=2)
	r.Inject([]persistence.Task{task2, task3})

	r.mu.RLock()
	cacheLen := r.queue.Len()
	newUpper := r.exclusiveUpperBound
	r.mu.RUnlock()

	assert.Equal(t, 2, cacheLen, "cache trimmed to MaxSize=2")
	assert.Equal(t, task2.GetTaskKey().Next(), newUpper, "upper bound shrunk after trim")

	// GetTask within covered range → cache hit.
	resp, err := r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
				ExclusiveMaxTaskKey: task2.GetTaskKey().Next(),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(now, 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 2)
	assert.Equal(t, 1, scope.count(metrics.CachedQueueHitsCounter))

	// GetTask beyond trimmed bound → base reader called (miss).
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).Return(
		&GetTaskResponse{Tasks: []persistence.Task{task3}}, nil)
	resp, err = r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: task2.GetTaskKey().Next(),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			},
			NextTaskKey: task2.GetTaskKey().Next(),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 1)
	assert.Equal(t, 1, scope.count(metrics.CachedQueueMissesCounter))
}

// TestCachedQueueReader_Lifecycle_PrefetchPagination: a full DB page advances
// the upper bound only to the last task's next key. A second prefetch extends it.
func TestCachedQueueReader_Lifecycle_PrefetchPagination(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)

	opts := lifecycleBaseOpts()
	opts.PrefetchPageSize = dynamicproperties.GetIntPropertyFn(2)
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(500 * time.Millisecond)

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), metrics.NoopScope)

	task1 := newTestTask(now.Add(1*time.Minute), 1)
	task2 := newTestTask(now.Add(2*time.Minute), 2)
	task3 := newTestTask(now.Add(3*time.Minute), 3)

	// Page 1: full (2 of 3) → upper bound = task2.Next().
	p1 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p1)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task1, task2},
				Progress: &GetTaskProgress{NextTaskKey: task2.GetTaskKey().Next()},
			}, nil
		},
	)
	// Page 2: partial (1 task) → upper bound = ceiling.
	p2 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p2)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task3},
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)
	awaitPrefetch(ts, 1*time.Millisecond, p1)

	r.mu.RLock()
	upperAfterPage1 := r.exclusiveUpperBound
	r.mu.RUnlock()
	assert.Equal(t, task2.GetTaskKey().Next(), upperAfterPage1,
		"full page: upper bound stops at next task key, not the ceiling")

	resp, err := r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
				ExclusiveMaxTaskKey: task2.GetTaskKey().Next(),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(now, 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 2)

	// MinPrefetchInterval elapsed → second prefetch fires and extends to ceiling.
	awaitPrefetch(ts, 500*time.Millisecond, p2)

	r.mu.RLock()
	upperAfterPage2 := r.exclusiveUpperBound
	cacheLen := r.queue.Len()
	r.mu.RUnlock()

	wantCeiling := persistence.NewHistoryTaskKey(ts.Now().Add(opts.MaxLookAheadWindow()), 0)
	assert.Equal(t, wantCeiling, upperAfterPage2, "partial page: upper bound reaches ceiling")
	assert.Equal(t, 3, cacheLen)
}

// TestCachedQueueReader_Lifecycle_GapDetectionAndRecovery: when the upper bound
// changes while a prefetch DB call is in-flight, gap detection clears the cache
// and the next prefetch rebuilds it.
func TestCachedQueueReader_Lifecycle_GapDetectionAndRecovery(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)

	opts := lifecycleBaseOpts()
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(500 * time.Millisecond)

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), metrics.NoopScope)

	task2 := newTestTask(now.Add(3*time.Minute), 2) // loaded in recovery

	p1Started := make(chan struct{})
	p1Unblock := make(chan struct{})
	p1Done := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *GetTaskRequest) (*GetTaskResponse, error) {
			close(p1Started)
			<-p1Unblock
			defer close(p1Done)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{newTestTask(now.Add(1*time.Minute), 1)},
				Progress: &GetTaskProgress{NextTaskKey: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0)},
			}, nil
		},
	)
	p2 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p2)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task2},
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)
	ts.Advance(1 * time.Millisecond) // trigger prefetch
	<-p1Started                      // prefetch is in-flight

	// Change upper bound mid-fetch to trigger gap detection.
	r.mu.Lock()
	r.exclusiveUpperBound = persistence.NewHistoryTaskKey(now.Add(30*time.Second), 0)
	r.mu.Unlock()
	close(p1Unblock)

	<-p1Done         // gap detected, fetched data discarded, error returned
	ts.BlockUntil(1) // wait for prefetch loop to re-arm at MinPrefetchInterval

	awaitPrefetch(ts, 500*time.Millisecond, p2)

	r.mu.RLock()
	cacheLen := r.queue.Len()
	r.mu.RUnlock()
	assert.Equal(t, 1, cacheLen, "cache rebuilt with recovery task after gap")
}

// TestCachedQueueReader_Lifecycle_InjectBeforePrefetch: tasks injected before
// the first prefetch (upper bound = MinimumHistoryTaskKey) are skipped and then
// loaded from DB when prefetch runs.
func TestCachedQueueReader_Lifecycle_InjectBeforePrefetch(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	scope := newCounterScope()

	opts := lifecycleBaseOpts()

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), scope)

	task1 := newTestTask(now.Add(1*time.Minute), 1)
	p1 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p1)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task1},
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)

	r.Inject([]persistence.Task{task1})
	r.mu.RLock()
	assert.Equal(t, 0, r.queue.Len(), "inject before prefetch: task not yet in cache")
	r.mu.RUnlock()

	awaitPrefetch(ts, time.Millisecond, p1)

	r.mu.RLock()
	assert.Equal(t, 1, r.queue.Len(), "task in cache after prefetch")
	r.mu.RUnlock()

	resp, err := r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(5*time.Minute), 0),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(now, 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 1)
	assert.Equal(t, 1, scope.count(metrics.CachedQueueHitsCounter))
}

// TestCachedQueueReader_Lifecycle_PrefetchBeforeInject: when prefetch runs first
// (upper bound set), a subsequent Inject lands in the cache immediately so the
// next GetTask returns a hit with no base reader call.
func TestCachedQueueReader_Lifecycle_PrefetchBeforeInject(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	scope := newCounterScope()

	opts := lifecycleBaseOpts()

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), scope)

	p1 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p1)
			return &GetTaskResponse{
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)
	awaitPrefetch(ts, 1*time.Millisecond, p1)

	r.mu.RLock()
	upperBound := r.exclusiveUpperBound
	r.mu.RUnlock()
	assert.True(t, upperBound.Greater(persistence.MinimumHistoryTaskKey), "upper bound set by prefetch")

	task1 := newTestTask(now.Add(1*time.Minute), 1)
	r.Inject([]persistence.Task{task1})
	r.mu.RLock()
	assert.Equal(t, 1, r.queue.Len(), "inject after prefetch: in cache immediately")
	r.mu.RUnlock()

	resp, err := r.GetTask(context.Background(), &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(now, 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(now.Add(2*time.Minute), 0),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(now, 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 1)
	assert.Equal(t, 1, scope.count(metrics.CachedQueueHitsCounter))
	assert.Equal(t, 0, scope.count(metrics.CachedQueueMissesCounter))
}

// TestCachedQueueReader_Lifecycle_BoundariesAndInject is the canonical happy-path
// lifecycle test: prefetch fires, boundaries are set exactly right, Inject lands
// the task in cache, and UpdateReadLevel advances the lower bound.
func TestCachedQueueReader_Lifecycle_BoundariesAndInject(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)

	maxLookAhead := 5 * time.Minute
	evictionSafeWindow := 30 * time.Second

	opts := lifecycleBaseOpts()
	opts.MaxLookAheadWindow = dynamicproperties.GetDurationPropertyFn(maxLookAhead)
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(evictionSafeWindow)

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), metrics.NoopScope)

	task1 := newTestTask(now.Add(1*time.Minute), 1)

	p1 := make(chan struct{})
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
			defer close(p1)
			return &GetTaskResponse{
				Tasks:    []persistence.Task{task1},
				Progress: &GetTaskProgress{NextTaskKey: req.Progress.ExclusiveMaxTaskKey},
			}, nil
		},
	)

	startLifecycle(t, r, ts)

	// Lower and upper bounds start unset before the first prefetch.
	r.mu.RLock()
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.inclusiveLowerBound, "lower bound must start at minimum")
	assert.Equal(t, persistence.MinimumHistoryTaskKey, r.exclusiveUpperBound, "upper bound must be unset before first prefetch")
	r.mu.RUnlock()

	// Trigger and wait for the initial prefetch. The clock advances by 1ms.
	awaitPrefetch(ts, time.Millisecond, p1)

	// After the first prefetch the clock is at now+1ms. The prefetch anchors
	// inclusiveLowerBound to (now+1ms) - TimeEvictionWindow and sets
	// exclusiveUpperBound to (now+1ms) + MaxLookAheadWindow (the ceiling).
	afterPrefetch := now.Add(time.Millisecond)
	wantLower := persistence.NewHistoryTaskKey(afterPrefetch.Add(-evictionSafeWindow), 0)
	wantCeiling := persistence.NewHistoryTaskKey(afterPrefetch.Add(maxLookAhead), 0)

	r.mu.RLock()
	assert.Equal(t, wantCeiling, r.exclusiveUpperBound, "upper bound must equal look-ahead ceiling after prefetch")
	assert.Equal(t, wantLower, r.inclusiveLowerBound, "lower bound must be anchored to now-TimeEvictionWindow on first prefetch")
	assert.Equal(t, 1, r.queue.Len(), "prefetched task must be in cache")
	r.mu.RUnlock()

	// Inject a new task within the window — it must land in cache immediately.
	task2 := newTestTask(now.Add(2*time.Minute), 2)
	r.Inject([]persistence.Task{task2})
	r.mu.RLock()
	assert.Equal(t, 2, r.queue.Len(), "injected task must be in cache")
	r.mu.RUnlock()

	// Advance the lower bound to task2's key; task1 (before task2) is evicted.
	r.UpdateReadLevel(task2.GetTaskKey())
	r.mu.RLock()
	assert.Equal(t, task2.GetTaskKey(), r.inclusiveLowerBound, "lower bound must advance to new read level")
	assert.Equal(t, 1, r.queue.Len(), "task1 evicted from cache after read-level advance")
	r.mu.RUnlock()
}

// TestCachedQueueReader_Lifecycle_LongRunning exercises the cache over a longer
// simulated period: multiple prefetch pages, injections at different scheduled
// times, GetTask requests, and time-based eviction all running concurrently.
// The DB is modelled by a closure over a sorted task slice.
func TestCachedQueueReader_Lifecycle_LongRunning(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := clock.NewMockedTimeSourceAt(now)
	ctrl := gomock.NewController(t)
	base := NewMockQueueReader(ctrl)
	scope := newCounterScope()

	opts := defaultTestOptions()
	opts.Mode = dynamicproperties.GetStringPropertyFn("enabled")
	opts.TimeEvictionWindow = dynamicproperties.GetDurationPropertyFn(3 * time.Second)
	opts.MaxLookAheadWindow = dynamicproperties.GetDurationPropertyFn(4 * time.Second)
	// Large trigger window → nextPrefetchDelay always clamped to MinPrefetchInterval.
	opts.PrefetchTriggerWindow = dynamicproperties.GetDurationPropertyFn(10 * time.Minute)
	opts.MinPrefetchInterval = dynamicproperties.GetDurationPropertyFn(1 * time.Second)
	opts.PrefetchPageSize = dynamicproperties.GetIntPropertyFn(5)
	opts.MaxSize = dynamicproperties.GetIntPropertyFn(100)

	// DB: tasks at T+0.5s, T+1s, T+1.5s, … T+9.5s (19 tasks).
	var dbTasks []persistence.Task
	for i := 1; i <= 19; i++ {
		dbTasks = append(dbTasks, newTestTask(now.Add(time.Duration(i)*500*time.Millisecond), int64(i)))
	}
	base.EXPECT().GetTask(gomock.Any(), gomock.Any()).
		DoAndReturn(dbFromTasks(dbTasks)).
		AnyTimes()

	r := newCachedQueueReaderWithOptions(base, newInMemQueue(), opts, ts, testlogger.New(t), scope)
	startLifecycle(t, r, ts)

	// Trigger initial prefetch.
	ts.Advance(1 * time.Millisecond)
	ts.BlockUntil(1)

	// Inject tasks to simulate concurrent workflow timer creation.
	r.Inject([]persistence.Task{
		newTestTask(now.Add(500*time.Millisecond), 100),
		newTestTask(now.Add(1500*time.Millisecond), 101),
	})

	// Simulate 10 rounds of 1 second each.
	for round := 1; round <= 10; round++ {
		ts.Advance(1 * time.Second)
		ts.BlockUntil(1) // both loops re-armed

		currentTime := now.Add(time.Duration(round) * time.Second)

		// Inject a task 2 seconds in the future.
		r.Inject([]persistence.Task{newTestTask(currentTime.Add(2*time.Second), int64(200+round))})

		r.mu.RLock()
		lb, ub := r.inclusiveLowerBound, r.exclusiveUpperBound
		r.mu.RUnlock()

		// Invariant: lower <= upper at all times.
		assert.True(t, lb.Compare(ub) <= 0,
			"round %d: invariant violated lower(%v) > upper(%v)", round, lb, ub)

		// Issue GetTask only if the range is covered.
		rangeMin := persistence.NewHistoryTaskKey(currentTime.Add(-500*time.Millisecond), 0)
		rangeMax := persistence.NewHistoryTaskKey(currentTime.Add(500*time.Millisecond), 0)
		if !lb.Greater(rangeMin) && !ub.Less(rangeMax) {
			resp, err := r.GetTask(context.Background(), &GetTaskRequest{
				Progress: &GetTaskProgress{
					Range: Range{
						InclusiveMinTaskKey: rangeMin,
						ExclusiveMaxTaskKey: rangeMax,
					},
					NextTaskKey: rangeMin,
				},
				Predicate: NewUniversalPredicate(),
				PageSize:  10,
			})
			require.NoError(t, err, "round %d GetTask", round)
			for _, task := range resp.Tasks {
				k := task.GetTaskKey()
				assert.True(t, !k.Less(rangeMin) && k.Less(rangeMax),
					"round %d: task key %v outside requested range", round, k)
			}
		}
	}

	assert.Greater(t, scope.count(metrics.CachedQueueHitsCounter), 0, "should have cache hits")
}
