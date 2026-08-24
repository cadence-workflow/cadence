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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

// scheduledCacheQueueReaderOptions holds the tuning knobs used only by the scheduled (timer)
// cached reader. The immediate (transfer) reader uses none of them.
type scheduledCacheQueueReaderOptions struct {
	// MaxLookAheadWindow is how far into the future from now the cache prefetches.
	// Tasks with scheduled time beyond now+MaxLookAheadWindow are not fetched.
	MaxLookAheadWindow dynamicproperties.DurationPropertyFn
	// PrefetchTriggerWindow defines how close to the upper-bound a task must be
	// before the next prefetch is scheduled. A prefetch fires when the nearest
	// upcoming task is within PrefetchTriggerWindow of the current upper bound.
	PrefetchTriggerWindow dynamicproperties.DurationPropertyFn
	// PrefetchPageSize caps the number of tasks fetched per DB round-trip.
	PrefetchPageSize dynamicproperties.IntPropertyFn
	// TimeEvictionWindow is the lookback horizon: tasks older than
	// now-TimeEvictionWindow are evicted to reclaim cache capacity.
	TimeEvictionWindow dynamicproperties.DurationPropertyFn
	// MinPrefetchInterval is the minimum time between consecutive prefetch attempts.
	// It prevents the prefetch loop from hammering the database when the cache resets
	// or gap detection fires repeatedly.
	MinPrefetchInterval dynamicproperties.DurationPropertyFn
	// PrefetchJitterCoefficient is passed to backoff.JitDuration when computing
	// the next prefetch delay. Must be in [0, 1]. Zero disables jitter.
	PrefetchJitterCoefficient dynamicproperties.FloatPropertyFn
}

// cachedScheduledQueueReader is the scheduled (timer) cached queue reader. It embeds the shared
// cachedQueueReaderBase and adds the prefetch cycle that keeps a look-ahead window of future
// timer tasks warm in the cache.
type cachedScheduledQueueReader struct {
	*cachedQueueReaderBase

	// scheduledOpts holds the look-ahead / time-eviction / prefetch tuning knobs, specific to the
	// scheduled/timer reader.
	scheduledOpts *scheduledCacheQueueReaderOptions

	// prefetchTargetUpper is the new exclusive upper key the current in-flight prefetch is aiming
	// to reach. Prefetch-only state, so it lives on the scheduled reader rather than the shared
	// base. Reset to MinimumHistoryTaskKey when no prefetch is in flight and on clearLocked.
	prefetchTargetUpper persistence.HistoryTaskKey

	// pendingInjectBuffer holds tasks that arrive via Inject while a prefetch is in-flight with
	// keys in [exclusiveUpperBound, prefetchTargetUpper). Prefetch-only state, so it lives on the
	// scheduled reader rather than the shared base. Drained into the cache once the prefetch
	// completes and reset on clearLocked.
	pendingInjectBuffer []persistence.Task

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// prefetchCh signals the prefetchLoop to recompute its timer. Buffered(1) so
	// senders never block; duplicate signals are dropped, the loop reads current
	// state on each wake.
	prefetchCh chan struct{}
}

func newCachedScheduledQueueReader(
	base QueueReader,
	queue InMemQueue,
	shard shard.Context,
	metricsScope metrics.Scope,
) *cachedScheduledQueueReader {
	config := shard.GetConfig()
	return newCachedScheduledQueueReaderWithOptions(
		base,
		queue,
		shard,
		shard.GetTimeSource(),
		shard.GetLogger().WithTags(tag.ComponentCachedQueueReader),
		metricsScope,
		&cachedQueueReaderOptions{
			Mode:                 config.TimerProcessorCachedQueueReaderMode,
			MaxSize:              config.TimerProcessorCacheMaxSize,
			ShadowSampleInterval: config.TimerProcessorCachedQueueReaderShadowSampleInterval,
		},
		&scheduledCacheQueueReaderOptions{
			MaxLookAheadWindow:        config.TimerProcessorMaxPollInterval,
			PrefetchTriggerWindow:     config.TimerProcessorCachePrefetchTriggerWindow,
			PrefetchPageSize:          config.TimerTaskBatchSize,
			TimeEvictionWindow:        config.TimerProcessorCacheTimeEvictionWindow,
			MinPrefetchInterval:       config.TimerProcessorCacheMinPrefetchInterval,
			PrefetchJitterCoefficient: config.TimerProcessorMaxPollIntervalJitterCoefficient,
		},
	)
}

func newCachedScheduledQueueReaderWithOptions(
	base QueueReader,
	queue InMemQueue,
	shard shard.Context,
	clockSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
	options *cachedQueueReaderOptions,
	scheduledOpts *scheduledCacheQueueReaderOptions,
) *cachedScheduledQueueReader {
	ctx, cancel := context.WithCancel(context.Background())
	q := &cachedScheduledQueueReader{
		cachedQueueReaderBase: newCachedQueueReaderBase(
			base,
			queue,
			shard,
			clockSource,
			logger,
			metricsScope,
			options,
		),
		scheduledOpts:       scheduledOpts,
		prefetchTargetUpper: persistence.MinimumHistoryTaskKey,
		prefetchCh:          make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
	}
	// Wake the prefetch loop whenever the upper bound moves so it can recompute its timer.
	q.cachedQueueReaderBase.onUpperBoundUpdatedLockedFn = q.notifyPrefetch
	// Reset the prefetch-only state whenever the shared cache is cleared.
	q.cachedQueueReaderBase.onClearLockedFn = q.resetPrefetchState
	return q
}

// resetPrefetchState clears the in-flight prefetch target and pending inject buffer. Wired as the
// base's onClearLockedFn hook so a shared clearLocked also discards prefetch-only state.
// Caller must hold q.mu (write), which clearLocked guarantees.
func (q *cachedScheduledQueueReader) resetPrefetchState() {
	q.pendingInjectBuffer = q.pendingInjectBuffer[:0]
	q.prefetchTargetUpper = persistence.MinimumHistoryTaskKey
}

// Start anchors the initial eviction window and launches the background loops.
func (q *cachedScheduledQueueReader) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	q.logger.Info("Cached Queue Reader state changed", tag.LifeCycleStarting)
	q.wg.Add(1)
	go q.prefetchLoop()
}

// Stop cancels background goroutines and waits for them to finish.
func (q *cachedScheduledQueueReader) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	q.logger.Info("Cached Queue Reader state changed", tag.LifeCycleStopping)
	q.cancel()
	q.wg.Wait()
	q.logger.Info("Cached Queue Reader state changed", tag.LifeCycleStopped)
}

// prefetchLoop fetches tasks into the look-ahead window on a timer. It fires
// shortly after Start, then re-arms based on the result or when the upper
// bound changes via notifyPrefetch.
func (q *cachedScheduledQueueReader) prefetchLoop() {
	defer q.wg.Done()

	timer := q.clock.NewTimer(time.Millisecond)
	defer timer.Stop()

	q.logger.Info("Cached Queue Reader state changed", tag.LifeCycleStarted)

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.prefetchCh:
			// Upper bound changed externally, recompute delay and reset timer.
			timer.Reset(q.nextPrefetchDelay())
		case <-timer.Chan():
			q.tryTimeEvictIfCacheFull()
			if err := q.prefetch(); err != nil {
				q.logger.Warn("prefetch failed, retrying shortly", tag.Error(err))
				timer.Reset(q.scheduledOpts.MinPrefetchInterval())
			} else {
				timer.Reset(q.nextPrefetchDelay())
			}
		}
	}
}

// notifyPrefetch signals the prefetchLoop to recompute its timer. Non-blocking;
// drops the signal if one is already pending, the loop reads current state on wake.
func (q *cachedScheduledQueueReader) notifyPrefetch() {
	select {
	case q.prefetchCh <- struct{}{}:
	default:
	}
}

// nextPrefetchDelay returns how long to wait before the next prefetch. It
// computes the trigger window relative to exclusiveUpperBound, clamped to
// MinPrefetchInterval.
func (q *cachedScheduledQueueReader) nextPrefetchDelay() time.Duration {
	q.mu.RLock()
	defer q.mu.RUnlock()

	triggerTime := q.exclusiveUpperBound.GetScheduledTime().Add(-q.scheduledOpts.PrefetchTriggerWindow())
	delay := max(q.scheduledOpts.MinPrefetchInterval(), triggerTime.Sub(q.clock.Now()))
	jittered := backoff.JitDuration(delay, q.scheduledOpts.PrefetchJitterCoefficient())

	// Cap the jittered delay to the original delay: positive jitter must not push the
	// prefetch past triggerTime, which would cause it to fire after the upper bound and
	// produce cache misses. MinPrefetchInterval is re-applied as the floor because
	// negative jitter on a delay already clamped to MinPrefetchInterval can otherwise
	// return a value below it.
	return max(q.scheduledOpts.MinPrefetchInterval(), min(delay, jittered))
}

// LookAHead returns the next task at or after req.InclusiveMinTaskKey. Serves
// from cache when the request falls within the cached window. Bypasses
// cache when disabled or in shadow mode. Shadow mode bypasses because in-flight
// inject notifications make cache/DB comparison unreliable for look-ahead.
//
// Look-ahead is a scheduled/timer concern: only the scheduled queue's event loop calls it, to
// learn when the next future timer fires. The immediate/transfer reader never uses cached
// look-ahead (its wrapper delegates straight to the base reader).
func (q *cachedScheduledQueueReader) LookAHead(ctx context.Context, req *LookAHeadRequest) (*LookAHeadResponse, error) {
	if q.isDisabled() || q.isShadow() {
		return q.base.LookAHead(ctx, req)
	}

	if q.fallbackIfRangeIDChanged() {
		q.logger.Info("LookAHead falling back to base reader after rangeID change")
		return q.base.LookAHead(ctx, req)
	}

	q.mu.RLock()

	logTags := []tag.Tag{
		tag.Dynamic("lookAHeadRequest", req),
		tag.Dynamic("cacheState", q.getState()),
	}

	if !q.isTaskCovered(req.InclusiveMinTaskKey) {
		q.mu.RUnlock()
		q.logger.Debug("look-ahead cache miss", logTags...)
		return q.base.LookAHead(ctx, req)
	}

	cacheTask := q.queue.LookAHead(req.InclusiveMinTaskKey)
	lookAHeadMaxTime := q.exclusiveUpperBound.GetScheduledTime()

	q.mu.RUnlock()

	return &LookAHeadResponse{
		Task:             cacheTask,
		LookAheadMaxTime: lookAHeadMaxTime,
	}, nil
}

// prefetch fetches one page of tasks into the look-ahead window. Returns nil
// on success (including no-op cases); non-nil on any failure. The caller
// (prefetchLoop) schedules the next attempt.
func (q *cachedScheduledQueueReader) prefetch() error {
	if q.isDisabled() {
		// Clear stale cache so re-enabling starts with a fresh prefetch
		// instead of serving outdated boundaries that cause cache misses.
		q.clearIfNotEmpty()
		q.logger.Debug("prefetch skipped, cache disabled")
		return nil
	}

	q.mu.RLock()
	availableCacheSize := q.options.MaxSize() - q.queue.Len()
	upperBound := q.exclusiveUpperBound
	q.mu.RUnlock()

	if availableCacheSize <= 0 {
		q.logger.Debug("prefetch skipped, cache full")
		return nil
	}

	now := q.clock.Now()

	// Started after the no-op guards so skips aren't timed.
	sw := q.metrics.StartTimerWithExponentialHistogram(metrics.CachedQueuePrefetchLatency, metrics.CachedQueuePrefetchLatencyHistogram)
	defer sw.Stop()

	// Ceiling of the look-ahead window; tasks at or after this time aren't due yet.
	exclusiveMaxTaskKey := persistence.NewHistoryTaskKey(now.Add(q.scheduledOpts.MaxLookAheadWindow()), 0)

	// Start from the existing upper bound so pages don't overlap. On the first
	// run (upperBound is MinimumHistoryTaskKey, nothing fetched yet), anchor to
	// now-TimeEvictionWindow; starting from absolute minimum would pull tasks
	// that timeEvict would drop immediately.
	inclusiveMinTaskKey := upperBound
	if inclusiveMinTaskKey.Equal(persistence.MinimumHistoryTaskKey) {
		inclusiveMinTaskKey = persistence.NewHistoryTaskKey(now.Add(-q.scheduledOpts.TimeEvictionWindow()), 0)
	}

	// Cap the page to available space (so the insert won't spill into RTrimBySize)
	// and to the configured page size (to bound each round-trip).
	pageSize := min(availableCacheSize, q.scheduledOpts.PrefetchPageSize())

	// Record the prefetch's target window so Inject can buffer tasks that arrive
	// while the DB call is in-flight. Cleared inside the write lock after the call.
	q.mu.Lock()
	q.prefetchTargetUpper = exclusiveMaxTaskKey
	q.mu.Unlock()

	resp, err := q.base.GetTask(q.ctx, &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: inclusiveMinTaskKey,
				ExclusiveMaxTaskKey: exclusiveMaxTaskKey,
			},
			NextPageToken: nil,
			NextTaskKey:   inclusiveMinTaskKey,
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  pageSize,
	})

	q.mu.Lock()
	defer q.mu.Unlock()
	// Always clear the in-flight target and drain the buffer, even on error,
	// so that buffered tasks are not permanently lost.
	defer q.insertBufferedTasks()
	q.prefetchTargetUpper = persistence.MinimumHistoryTaskKey

	if err != nil {
		q.metrics.IncCounter(metrics.CachedQueuePrefetchFailureCounter)
		q.logger.Error("prefetch failed", tag.Error(err))
		return fmt.Errorf("prefetch failed: %w", err)
	}

	if q.isDisabled() {
		// Mode flipped to disabled mid-fetch: the result is discarded rather than
		// completed, so it is neither a success nor a failure and is not counted.
		q.logger.Info("prefetch result discarded, mode switched to disabled during prefetch")
		q.pendingInjectBuffer = q.pendingInjectBuffer[:0]
		return nil
	}

	// Upper bound changed while we held the lock (e.g. a concurrent Inject
	// triggered RTrimBySize, shrinking the window). The fetched tasks start at
	// the old upperBound, which is now beyond the current window end, so they
	// cannot be inserted contiguously. Discard only the fetched data; the
	// existing cache remains valid for [inclusiveLowerBound, exclusiveUpperBound).
	// The next prefetch will fill the gap from the new exclusiveUpperBound.
	if !q.exclusiveUpperBound.Equal(upperBound) {
		q.metrics.IncCounter(metrics.CachedQueuePrefetchGapDetectedCounter)
		q.logger.Info("gap detected, discarding fetched data",
			tag.Dynamic("prevUpper", upperBound),
			tag.Dynamic("cacheState", q.getState()),
		)
		return fmt.Errorf("gap detected: upper bound changed during fetch")
	}

	// If inclusiveLowerBound is still at the  minimum, this is the first prefetch after the start or run of a Clear.
	// Advance it to the inclusiveMinTaskKey of this fetch so the cache window is correctly anchored and isRangeCovered works as intended.
	if q.inclusiveLowerBound.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateInclusiveLowerBound(inclusiveMinTaskKey)
	}

	// If a trim occurred, putTasks already updated the upper bound correctly.
	// Otherwise advance to NextTaskKey: the key to start the next page, pointing
	// to the first task beyond the current page (or ExclusiveMaxTaskKey when
	// there are no more tasks to fetch).
	if trimmed := q.putTasks(resp.Tasks); !trimmed {
		q.updateExclusiveUpperBound(resp.Progress.NextTaskKey)
	}

	q.metrics.IncCounter(metrics.CachedQueuePrefetchSuccessCounter)

	// Whether prefetch is keeping up with newly created tasks.
	windowSpan := q.exclusiveUpperBound.GetScheduledTime().Sub(q.inclusiveLowerBound.GetScheduledTime())
	q.metrics.ExponentialHistogram(metrics.CachedQueuePrefetchWindowSpanHistogram, windowSpan)

	q.logger.Debug("prefetch complete",
		tag.Dynamic("tasksFetched", len(resp.Tasks)),
		tag.Dynamic("windowSpan", windowSpan),
		tag.Dynamic("cacheState", q.getState()),
	)
	return nil
}

// isToBufferTask reports whether the given task key should be placed in pendingInjectBuffer
// and within [exclusiveUpperBound, prefetchTargetUpper) and if a prefetch is in-flight
// Caller must hold q.mu (read or write).
func (q *cachedScheduledQueueReader) isToBufferTask(key persistence.HistoryTaskKey) bool {
	// there is no in-flight prefetch, so no tasks should be buffered.
	if q.prefetchTargetUpper.Equal(persistence.MinimumHistoryTaskKey) {
		return false
	}

	return key.GreaterOrEqual(q.exclusiveUpperBound) && key.Less(q.prefetchTargetUpper)
}

// putTasks adds tasks to the cache and enforces the size cap.
// Returns true if RTrimBySize fired and updated exclusiveUpperBound,
// meaning the caller must not re-advance the bound.
// Caller must hold q.mu.
func (q *cachedScheduledQueueReader) putTasks(tasks []persistence.Task) bool {
	if len(tasks) == 0 {
		return false
	}

	// Lazy eviction: if inserting filtered tasks would exceed MaxSize, evict
	// tasks older than TimeEvictionWindow first to make room.
	q.tryTimeEvict(len(tasks))
	q.queue.PutTasks(tasks)
	newUpper, trimmed := q.queue.RTrimBySize(q.options.MaxSize())

	if !trimmed {
		return false
	}

	// edge-case: if the trim removed everything, the queue is now empty and the window should reset to the minimum
	if newUpper.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateInclusiveLowerBound(persistence.MinimumHistoryTaskKey)
	}
	q.updateExclusiveUpperBound(newUpper)

	return true
}

// insertBufferedTasks drains pendingInjectBuffer into the cache for tasks now
// covered by the updated exclusiveUpperBound. Must be called under q.mu (write lock).
func (q *cachedScheduledQueueReader) insertBufferedTasks() {
	if len(q.pendingInjectBuffer) == 0 {
		return
	}
	var covered []persistence.Task
	for _, t := range q.pendingInjectBuffer {
		if q.isTaskCovered(t.GetTaskKey()) {
			covered = append(covered, t)
		}
	}
	q.pendingInjectBuffer = q.pendingInjectBuffer[:0]
	q.putTasks(covered)
}

// tryTimeEvict evicts tasks older than TimeEvictionWindow if adding extraTasks
// would exceed MaxSize.
// Caller must hold q.mu (write).
func (q *cachedScheduledQueueReader) tryTimeEvict(extraTasks int) {
	if q.queue.Len()+extraTasks < q.options.MaxSize() {
		return
	}
	evictBefore := persistence.NewHistoryTaskKey(q.clock.Now().Add(-q.scheduledOpts.TimeEvictionWindow()), 0)
	q.advanceInclusiveLowerBound(evictBefore)
}

// tryTimeEvictIfCacheFull evicts tasks older than TimeEvictionWindow if the cache is full
func (q *cachedScheduledQueueReader) tryTimeEvictIfCacheFull() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tryTimeEvict(1)
}

// Inject adds tasks that have just been persisted into the in-memory cache.
// Tasks within the current cache window are inserted immediately. Tasks that
// fall in [exclusiveUpperBound, prefetchTargetUpper) while a prefetch is
// in-flight are buffered and drained once the prefetch completes. All other
// tasks are dropped. No-op when the cache is off.
func (q *cachedScheduledQueueReader) Inject(tasks []persistence.Task) {
	if q.isDisabled() {
		// Clear stale cache so re-enabling starts with a fresh prefetch
		// instead of serving outdated boundaries that cause cache misses.
		q.clearIfNotEmpty()
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	now := q.clock.Now()
	var injected, buffered, droppedBelow, droppedUpper int64

	var covered []persistence.Task
	for _, t := range tasks {
		if t.GetTaskID() == 0 {
			// no tasks with taskID == 0 are expected
			continue
		}
		if q.isTaskCovered(t.GetTaskKey()) {
			injected++
			if q.logger.DebugOn() {
				q.logger.Debug("injecting task",
					tag.Dynamic("taskKey", t.GetTaskKey()),
					tag.Dynamic("cacheState", q.getState()),
				)
			}
			covered = append(covered, t)
			continue
		}
		if q.isToBufferTask(t.GetTaskKey()) {
			buffered++
			if q.logger.DebugOn() {
				q.logger.Debug("buffering task",
					tag.Dynamic("taskKey", t.GetTaskKey()),
					tag.Dynamic("cacheState", q.getState()),
					tag.Dynamic("prefetchTargetUpper", q.prefetchTargetUpper),
				)
			}
			q.pendingInjectBuffer = append(q.pendingInjectBuffer, t)
			continue
		}

		// this case should not happen under normal operation,
		// as the processor should only be persisting tasks near the current upper bound
		// but log just in case to help debug any unexpected ordering issues
		if t.GetTaskKey().Less(q.inclusiveLowerBound) {
			droppedBelow++
			q.logger.Warn("task key is below the lower bound, dropping task",
				tag.Dynamic("taskKey", t.GetTaskKey()),
				tag.Dynamic("cacheState", q.getState()),
			)
			continue
		}

		// Task key is at or beyond the cache's exclusive upper bound (the prefetch
		// frontier) and outside the in-flight prefetch buffer window, so the cache
		// does not cover it and it is dropped. Record how far ahead of now it is
		// scheduled: the frontier normally sits near now+lookahead, so this typically
		// measures how far into the future the dropped task was scheduled, which helps
		// tune the look-ahead window.
		droppedUpper++
		q.metrics.ExponentialHistogram(
			metrics.CachedQueueDroppedFutureTimerTasksDurationHistogram,
			t.GetTaskKey().GetScheduledTime().Sub(now),
		)
		if q.logger.DebugOn() {
			q.logger.Debug("task key is beyond the upper/target prefetch bound, dropping task",
				tag.Dynamic("taskKey", t.GetTaskKey()),
				tag.Dynamic("cacheState", q.getState()),
			)
		}
	}

	q.emitInjectStatusCount(injectStatusInjected, injected)
	q.emitInjectStatusCount(injectStatusBuffered, buffered)
	q.emitInjectStatusCount(injectStatusDroppedBelow, droppedBelow)
	q.emitInjectStatusCount(injectStatusDroppedUpper, droppedUpper)

	q.putTasks(covered)
}
