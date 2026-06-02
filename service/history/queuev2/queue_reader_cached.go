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

//go:generate mockgen -package $GOPACKAGE -destination queue_reader_cached_mock.go github.com/uber/cadence/service/history/queuev2 CachedQueueReader

// CachedQueueReader extends QueueReader with cache injection and lifecycle control.
type CachedQueueReader interface {
	QueueReader
	// Inject adds tasks that have just been persisted into the in-memory cache.
	// Tasks outside the current prefetch window are silently dropped or buffered.
	Inject(tasks []persistence.Task)
	// Clear wipes all cached state and triggers a fresh prefetch from the DB.
	// Call when the cache may be stale (e.g. after a persistence error).
	Clear()
	// UpdateReadLevel advances the eviction lower bound to readLevel,
	// dropping tasks the processor has already passed.
	UpdateReadLevel(readLevel persistence.HistoryTaskKey)
	// Start anchors the eviction window and launches background loops.
	Start()
	// Stop cancels background goroutines and waits for them to finish.
	Stop()
}

// cachedQueueReaderOptions is the dynamic configuration for the cached queue reader.
// populated from shard.Context
type cachedQueueReaderOptions struct {
	// Mode controls cache behavior: "enabled" uses cache, anything else (including "disabled") disables.
	Mode dynamicproperties.StringPropertyFn
	// MaxSize is the maximum number of tasks the cache may hold at once.
	// Insertions that would exceed this limit trigger time-based eviction first.
	MaxSize dynamicproperties.IntPropertyFn
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

type cachedQueueReader struct {
	status  int32 // DaemonStatusInitialized / Started / Stopped
	base    QueueReader
	queue   InMemQueue
	options *cachedQueueReaderOptions
	clock   clock.TimeSource
	logger  log.Logger
	metrics metrics.Scope

	mu sync.RWMutex

	// inclusiveLowerBound is the inclusive start of the cached window. Tasks
	// before this key have been evicted and are no longer served from cache.
	// Invariant: inclusiveLowerBound <= exclusiveUpperBound.
	inclusiveLowerBound persistence.HistoryTaskKey

	// exclusiveUpperBound is the exclusive end of the prefetched window. Tasks with
	// key < exclusiveUpperBound are covered by the cache if they exist in the DB.
	// Invariant: inclusiveLowerBound <= exclusiveUpperBound.
	// Always update via updateExclusiveUpperBound to keep the prefetch loop in sync.
	exclusiveUpperBound persistence.HistoryTaskKey

	// prefetchTargetUpper is the new exclusive upper key the current in-flight prefetch
	// is aiming to reach. Set under mu before the DB call; cleared to
	// MinimumHistoryTaskKey after the call completes
	prefetchTargetUpper persistence.HistoryTaskKey

	// pendingInjectBuffer holds tasks that arrive via Inject while a prefetch is in-flight
	// with keys in [exclusiveUpperBound, prefetchTargetUpper).
	//
	// Without this buffer, a task saved to DB during a prefetch may be missed: its key is
	// beyond the current exclusiveUpperBound, so it is not injected into the cache, and the
	// prefetch may not see it due to a race between reading from DB and the task being saved.
	// When the prefetch completes, it advances exclusiveUpperBound past the task's key,
	// leaving the task permanently dropped from the cache.
	//
	// These tasks are drained into the cache after the prefetch extends the window.
	pendingInjectBuffer []persistence.Task

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// prefetchCh signals the prefetchLoop to recompute its timer. Buffered(1) so
	// senders never block; duplicate signals are dropped, the loop reads current
	// state on each wake.
	prefetchCh chan struct{}
}

func newCachedQueueReader(
	base QueueReader,
	queue InMemQueue,
	shard shard.Context,
	metricsScope metrics.Scope,
) *cachedQueueReader {
	config := shard.GetConfig()
	return newCachedQueueReaderWithOptions(
		base,
		queue,
		shard.GetTimeSource(),
		shard.GetLogger(),
		metricsScope,
		&cachedQueueReaderOptions{
			Mode:                      config.TimerProcessorCachedQueueReaderMode,
			MaxSize:                   config.TimerProcessorCacheMaxSize,
			MaxLookAheadWindow:        config.TimerProcessorMaxPollInterval,
			PrefetchTriggerWindow:     config.TimerProcessorCachePrefetchTriggerWindow,
			PrefetchPageSize:          config.TimerTaskBatchSize,
			TimeEvictionWindow:        config.TimerProcessorCacheTimeEvictionWindow,
			MinPrefetchInterval:       config.TimerProcessorCacheMinPrefetchInterval,
			PrefetchJitterCoefficient: config.TimerProcessorMaxPollIntervalJitterCoefficient,
		},
	)
}

func newCachedQueueReaderWithOptions(
	base QueueReader,
	queue InMemQueue,
	clockSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
	options *cachedQueueReaderOptions,
) *cachedQueueReader {
	ctx, cancel := context.WithCancel(context.Background())
	return &cachedQueueReader{
		status:              common.DaemonStatusInitialized,
		base:                base,
		queue:               queue,
		options:             options,
		clock:               clockSource,
		logger:              logger,
		metrics:             metricsScope,
		inclusiveLowerBound: persistence.MinimumHistoryTaskKey,
		exclusiveUpperBound: persistence.MinimumHistoryTaskKey,
		prefetchCh:          make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// Start anchors the initial eviction window and launches the background loops.
func (q *cachedQueueReader) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	q.wg.Add(1)
	go q.prefetchLoop()
}

// Stop cancels background goroutines and waits for them to finish.
func (q *cachedQueueReader) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	q.cancel()
	q.wg.Wait()
}

// prefetchLoop fetches tasks into the look-ahead window on a timer. It fires
// shortly after Start, then re-arms based on the result or when the upper
// bound changes via notifyPrefetch.
func (q *cachedQueueReader) prefetchLoop() {
	defer q.wg.Done()

	timer := q.clock.NewTimer(time.Millisecond)
	defer timer.Stop()

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Info("prefetch loop stopping")
			return
		case <-q.prefetchCh:
			// Upper bound changed externally, recompute delay and reset timer.
			timer.Reset(q.nextPrefetchDelay())
		case <-timer.Chan():
			q.tryTimeEvictIfCacheFull()
			if err := q.prefetch(); err != nil {
				q.logger.Warn("prefetch failed, retrying shortly", tag.Error(err))
				timer.Reset(q.options.MinPrefetchInterval())
			} else {
				timer.Reset(q.nextPrefetchDelay())
			}
		}
	}
}

// notifyPrefetch signals the prefetchLoop to recompute its timer. Non-blocking;
// drops the signal if one is already pending, the loop reads current state on wake.
func (q *cachedQueueReader) notifyPrefetch() {
	select {
	case q.prefetchCh <- struct{}{}:
	default:
	}
}

// nextPrefetchDelay returns how long to wait before the next prefetch. It
// computes the trigger window relative to exclusiveUpperBound, clamped to
// MinPrefetchInterval.
func (q *cachedQueueReader) nextPrefetchDelay() time.Duration {
	q.mu.RLock()
	defer q.mu.RUnlock()

	triggerTime := q.exclusiveUpperBound.GetScheduledTime().Add(-q.options.PrefetchTriggerWindow())
	delay := max(q.options.MinPrefetchInterval(), triggerTime.Sub(q.clock.Now()))

	return backoff.JitDuration(delay, q.options.PrefetchJitterCoefficient())
}

// isEnabled returns true if the cache is fully enabled
func (q *cachedQueueReader) isEnabled() bool { return q.options.Mode() == "enabled" }

// isDisabled returns true for the "disabled" mode and for any unrecognised value
func (q *cachedQueueReader) isDisabled() bool {
	switch q.options.Mode() {
	case "disabled":
		return true
	case "enabled":
		return false
	default:
		// Default to disabled for unrecognized modes to
		// avoid unintended consequences of a bad config value.
		return true
	}
}

// Clear wipes all cached state and triggers a fresh prefetch from the DB.
func (q *cachedQueueReader) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.logger.Info("cache fully cleared",
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	)

	q.queue.Clear()
	q.pendingInjectBuffer = q.pendingInjectBuffer[:0]
	q.prefetchTargetUpper = persistence.MinimumHistoryTaskKey
	q.updateInclusiveLowerBound(persistence.MinimumHistoryTaskKey)
	q.updateExclusiveUpperBound(persistence.MinimumHistoryTaskKey)
}

// prefetch fetches one page of tasks into the look-ahead window. Returns nil
// on success (including no-op cases); non-nil on any failure. The caller
// (prefetchLoop) schedules the next attempt.
func (q *cachedQueueReader) prefetch() error {
	if q.isDisabled() {
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

	// Ceiling of the look-ahead window; tasks at or after this time aren't due yet.
	exclusiveMaxTaskKey := persistence.NewHistoryTaskKey(now.Add(q.options.MaxLookAheadWindow()), 0)

	// Start from the existing upper bound so pages don't overlap. On the first
	// run (upperBound is MinimumHistoryTaskKey, nothing fetched yet), anchor to
	// now-TimeEvictionWindow; starting from absolute minimum would pull tasks
	// that timeEvict would drop immediately.
	inclusiveMinTaskKey := upperBound
	if inclusiveMinTaskKey.Equal(persistence.MinimumHistoryTaskKey) {
		inclusiveMinTaskKey = persistence.NewHistoryTaskKey(now.Add(-q.options.TimeEvictionWindow()), 0)
	}

	// Cap the page to available space (so the insert won't spill into RTrimBySize)
	// and to the configured page size (to bound each round-trip).
	pageSize := min(availableCacheSize, q.options.PrefetchPageSize())

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
		q.logger.Error("prefetch failed", tag.Error(err))
		return fmt.Errorf("prefetch failed: %w", err)
	}

	// Upper bound changed while we held the lock (e.g. a concurrent Inject
	// triggered RTrimBySize, shrinking the window). The fetched tasks start at
	// the old upperBound, which is now beyond the current window end, so they
	// cannot be inserted contiguously. Discard only the fetched data; the
	// existing cache remains valid for [inclusiveLowerBound, exclusiveUpperBound).
	// The next prefetch will fill the gap from the new exclusiveUpperBound.
	if !q.exclusiveUpperBound.Equal(upperBound) {
		q.logger.Info("gap detected, discarding fetched data",
			tag.Dynamic("prevUpper", upperBound),
			tag.Dynamic("newUpper", q.exclusiveUpperBound),
		)
		return fmt.Errorf("gap detected: upper bound changed during fetch")
	}

	// If inclusiveLowerBound is still at the  minimum, this is the first prefetch after the start or run of a Clear.
	// Advance it to the inclusiveMinTaskKey of this fetch so the cache window is correctly anchored and isRangeCovered works as intended.
	if q.inclusiveLowerBound.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateInclusiveLowerBound(inclusiveMinTaskKey)
	}

	// If a trim occurred, putTasks already updated the upper bound correctly.
	if trimmed := q.putTasks(resp.Tasks); trimmed {
		return nil
	}

	// NextTaskKey is the key to start the next page; it points to the first task beyond the current page
	// or may be equal to ExclusiveMaxTaskKey if there are no more tasks to fetch
	q.updateExclusiveUpperBound(resp.Progress.NextTaskKey)

	q.logger.Debug("prefetch complete",
		tag.Dynamic("tasksFetched", len(resp.Tasks)),
		tag.Dynamic("newUpper", q.exclusiveUpperBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	)
	return nil
}

// isRangeCovered reports whether [inclusiveMin, exclusiveMax) falls fully
// within the cached window [inclusiveLowerBound, exclusiveUpperBound).
// Caller must hold q.mu (read or write).
func (q *cachedQueueReader) isRangeCovered(inclusiveMin, exclusiveMax persistence.HistoryTaskKey) bool {
	return !inclusiveMin.Less(q.inclusiveLowerBound) && !exclusiveMax.Greater(q.exclusiveUpperBound)
}

// isTaskCovered reports whether the given task key falls within the cached window.
// Caller must hold q.mu (read or write).
func (q *cachedQueueReader) isTaskCovered(key persistence.HistoryTaskKey) bool {
	return !key.Less(q.inclusiveLowerBound) && key.Less(q.exclusiveUpperBound)
}

// isToBufferTask reports whether the given task key should be placed in pendingInjectBuffer
// and within [exclusiveUpperBound, prefetchTargetUpper) and if a prefetch is in-flight
// Caller must hold q.mu (read or write).
func (q *cachedQueueReader) isToBufferTask(key persistence.HistoryTaskKey) bool {
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
func (q *cachedQueueReader) putTasks(tasks []persistence.Task) bool {
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
func (q *cachedQueueReader) insertBufferedTasks() {
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

// updateExclusiveUpperBound sets the upper bound and trigger prefetch if needed.
// Caller must hold q.mu.
func (q *cachedQueueReader) updateExclusiveUpperBound(newKey persistence.HistoryTaskKey) {
	q.logger.Debug("upper bound is updated",
		tag.Dynamic("prevUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("newUpperBound", newKey),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	)

	q.exclusiveUpperBound = newKey
	q.metrics.RecordHistogramValue(metrics.CachedQueueSizeHistogram, float64(q.queue.Len()))
	q.notifyPrefetch()
}

// updateInclusiveLowerBound sets the lower bound
// Caller must hold q.mu.
func (q *cachedQueueReader) updateInclusiveLowerBound(newKey persistence.HistoryTaskKey) {
	q.logger.Debug("lower bound is updated",
		tag.Dynamic("prevLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("newLowerBound", newKey),
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	)

	q.inclusiveLowerBound = newKey
	q.metrics.RecordHistogramValue(metrics.CachedQueueSizeHistogram, float64(q.queue.Len()))
}

// updateInclusiveLowerBound advances inclusiveLowerBound to newKey if it's
// ahead, trimming evicted tasks. Caps at exclusiveUpperBound when set to
// preserve the lower <= upper invariant.
// Caller must hold q.mu (write).
func (q *cachedQueueReader) advanceInclusiveLowerBound(newKey persistence.HistoryTaskKey) {
	if !newKey.Greater(q.inclusiveLowerBound) {
		return
	}

	if !newKey.Less(q.exclusiveUpperBound) {
		newKey = q.exclusiveUpperBound
	}

	q.queue.LTrim(newKey)
	q.updateInclusiveLowerBound(newKey)
}

// tryTimeEvict evicts tasks older than TimeEvictionWindow if adding extraTasks
// would exceed MaxSize.
// Caller must hold q.mu (write).
func (q *cachedQueueReader) tryTimeEvict(extraTasks int) {
	if q.queue.Len()+extraTasks < q.options.MaxSize() {
		return
	}
	evictBefore := persistence.NewHistoryTaskKey(q.clock.Now().Add(-q.options.TimeEvictionWindow()), 0)
	q.advanceInclusiveLowerBound(evictBefore)
}

// tryTimeEvictIfCacheFull evicts tasks older than TimeEvictionWindow if the cache is full
func (q *cachedQueueReader) tryTimeEvictIfCacheFull() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.tryTimeEvict(1)
}

// UpdateReadLevel advances the lower bound to the processor's ack position.
// MaximumHistoryTaskKey means "no valid read level" and skipped
func (q *cachedQueueReader) UpdateReadLevel(readLevel persistence.HistoryTaskKey) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if readLevel.Equal(persistence.MaximumHistoryTaskKey) {
		return
	}

	q.advanceInclusiveLowerBound(readLevel)
}

// Inject adds tasks that have just been persisted into the in-memory cache.
// Tasks within the current cache window are inserted immediately. Tasks that
// fall in [exclusiveUpperBound, prefetchTargetUpper) while a prefetch is
// in-flight are buffered and drained once the prefetch completes. All other
// tasks are dropped. No-op when the cache is off.
func (q *cachedQueueReader) Inject(tasks []persistence.Task) {
	if q.isDisabled() {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	var covered []persistence.Task
	for _, t := range tasks {
		if t.GetTaskID() == 0 {
			// no tasks with taskID == 0 are expected
			continue
		}
		if q.isTaskCovered(t.GetTaskKey()) {
			covered = append(covered, t)
			continue
		}
		if q.isToBufferTask(t.GetTaskKey()) {
			q.pendingInjectBuffer = append(q.pendingInjectBuffer, t)
			continue
		}
	}

	q.putTasks(covered)
}

// GetTask serves tasks from the cache when the starting key is covered.
// Disabled mode bypasses the cache entirely.
func (q *cachedQueueReader) GetTask(ctx context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
	if q.isDisabled() {
		return q.base.GetTask(ctx, req)
	}

	inclusiveMinTaskKey := req.Progress.Range.InclusiveMinTaskKey
	exclusiveMaxTaskKey := req.Progress.Range.ExclusiveMaxTaskKey

	// When NextPageToken is set, we need to use NextTaskKey as the starting point
	if req.Progress.NextPageToken != nil {
		// If NextTaskKey is not set, it delegates to the base reader to handle this edge case
		if req.Progress.NextTaskKey.Equal(persistence.MinimumHistoryTaskKey) {
			q.logger.Info("NextPageToken is set but NextTaskKey is not set, delegating to base reader", tag.Dynamic("getTaskRequest", req))
			return q.base.GetTask(ctx, req)
		}
		inclusiveMinTaskKey = req.Progress.NextTaskKey
	}

	q.mu.RLock()
	covered := q.isRangeCovered(inclusiveMinTaskKey, exclusiveMaxTaskKey)

	logTags := []tag.Tag{
		tag.Dynamic("getTaskRequest", req),
		tag.Dynamic("inclusiveMinTaskKey", inclusiveMinTaskKey),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	}

	if !covered {
		q.mu.RUnlock()
		q.metrics.IncCounter(metrics.CachedQueueMissesCounter)
		q.logger.Debug("cache miss", logTags...)
		return q.base.GetTask(ctx, req)
	}

	tasks, nextTaskKey := q.queue.GetTasks(inclusiveMinTaskKey, exclusiveMaxTaskKey, req.Predicate, req.PageSize)
	q.mu.RUnlock()

	q.metrics.IncCounter(metrics.CachedQueueHitsCounter)
	q.logger.Debug("cache hit", logTags...)

	// cacheResp is constructed with Progress.Range starting at nextTaskKey and the same exclusiveMaxTaskKey as the request.
	// This ensures that if the next page is fetched from the DB, Progress.Range will start at the correct position (nextTaskKey)
	// and end at the same exclusiveMaxTaskKey. Since NextPageToken is not used when serving from the cache, Progress.Range
	// is the sole source of truth for the next page's start and end. Using the request's original InclusiveMinTaskKey instead
	// of nextTaskKey would cause the next page to start at the wrong position, leading to duplicate or skipped tasks.
	cacheResp := &GetTaskResponse{
		Tasks: tasks,
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: nextTaskKey,
				ExclusiveMaxTaskKey: req.Progress.Range.ExclusiveMaxTaskKey,
			},
			NextPageToken: nil,
			NextTaskKey:   nextTaskKey,
		},
	}

	return cacheResp, nil
}

// LookAHead returns the next task at or after req.InclusiveMinTaskKey. Serves
// from cache when the request falls within the prefetched window. Bypasses
// cache when disabled mode.
func (q *cachedQueueReader) LookAHead(ctx context.Context, req *LookAHeadRequest) (*LookAHeadResponse, error) {
	if q.isDisabled() {
		q.logger.Debug("fail back to original look-ahead, cache is disabled")
		return q.base.LookAHead(ctx, req)
	}

	q.mu.RLock()

	logTags := []tag.Tag{
		tag.Dynamic("lookAHeadRequest", req),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
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
