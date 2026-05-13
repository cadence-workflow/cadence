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

//go:generate mockgen -package $GOPACKAGE -destination queue_reader_cached_mock.go github.com/uber/cadence/service/history/queuev2 CachedQueueReader

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

// cachedQueueReaderOptions is the dynamic configuration for the cached queue reader.
type cachedQueueReaderOptions struct {
	Mode                  dynamicproperties.StringPropertyFn
	MaxSize               dynamicproperties.IntPropertyFn
	MaxLookAheadWindow    dynamicproperties.DurationPropertyFn
	PrefetchTriggerWindow dynamicproperties.DurationPropertyFn
	PrefetchPageSize      dynamicproperties.IntPropertyFn
	TimeEvictionWindow    dynamicproperties.DurationPropertyFn
	// MinPrefetchInterval is the minimum time between consecutive prefetch attempts.
	// It prevents the prefetch loop from hammering the database when the cache resets
	// or gap detection fires repeatedly.
	MinPrefetchInterval dynamicproperties.DurationPropertyFn
	// PrefetchJitterCoefficient is passed to backoff.JitDuration when computing
	// the next prefetch delay. Must be in [0, 1]. Zero disables jitter.
	PrefetchJitterCoefficient dynamicproperties.FloatPropertyFn
}

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

type cachedQueueReader struct {
	status  int32 // DaemonStatusInitialized / Started / Stopped — access only via atomic
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

	// prefetchTargetUpper is the exclusive upper bound the current in-flight prefetch
	// is aiming to reach. Set to exclusiveMaxKey under mu.Lock immediately before the
	// DB call; cleared to MinimumHistoryTaskKey after the DB call completes. Zero
	// value (MinimumHistoryTaskKey) means no prefetch is in-flight.
	prefetchTargetUpper persistence.HistoryTaskKey

	// pendingInjectBuffer accumulates tasks arriving via Inject while a prefetch
	// DB call is in-flight and whose key falls in [exclusiveUpperBound, prefetchTargetUpper).
	// Drained into the in-memory queue after the prefetch completes or fails.
	// Guarded by mu.
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
	return newCachedQueueReaderWithOptions(base, queue, &cachedQueueReaderOptions{
		Mode:                      config.TimerProcessorCachedQueueReaderMode,
		MaxSize:                   config.TimerProcessorCacheMaxSize,
		MaxLookAheadWindow:        config.TimerProcessorCacheMaxLookAheadWindow,
		PrefetchTriggerWindow:     config.TimerProcessorCachePrefetchTriggerWindow,
		PrefetchPageSize:          config.TimerTaskBatchSize,
		TimeEvictionWindow:        config.TimerProcessorCacheTimeEvictionWindow,
		MinPrefetchInterval:       config.TimerProcessorCacheMinPrefetchInterval,
		PrefetchJitterCoefficient: config.TimerProcessorCachePrefetchJitterCoefficient,
	}, shard.GetTimeSource(), shard.GetLogger(), metricsScope)
}

func newCachedQueueReaderWithOptions(
	base QueueReader,
	queue InMemQueue,
	options *cachedQueueReaderOptions,
	clockSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
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
	// Anchor the lower bound now so the cache doesn't serve tasks from the
	// beginning of time before the first ack-level update arrives.
	q.UpdateReadLevel(persistence.MinimumHistoryTaskKey)
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
			if !timer.Stop() {
				select {
				case <-timer.Chan():
				default:
				}
			}
			timer.Reset(q.nextPrefetchDelay())
		case <-timer.Chan():
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
	var delay time.Duration
	upper := q.exclusiveUpperBound
	if !upper.Equal(persistence.MinimumHistoryTaskKey) {
		triggerTime := upper.GetScheduledTime().Add(-q.options.PrefetchTriggerWindow())
		if d := triggerTime.Sub(q.clock.Now()); d > 0 {
			delay = d
		}
	}
	if min := q.options.MinPrefetchInterval(); delay < min {
		delay = min
	}
	return backoff.JitDuration(delay, q.options.PrefetchJitterCoefficient())
}

// isEnabled returns true if the cache is fully enabled
func (q *cachedQueueReader) isEnabled() bool { return q.options.Mode() == "enabled" }

// isShadow returns true if the cache is in shadow mode (results compared against DB but not used for processing)
func (q *cachedQueueReader) isShadow() bool { return q.options.Mode() == "shadow" }

// isDisabled returns true for the "disabled" mode and for any unrecognised
// value
func (q *cachedQueueReader) isDisabled() bool {
	if q.options.Mode() == "disabled" {
		return true
	}
	if q.isEnabled() || q.isShadow() {
		return false
	}

	// Default to disabled for unrecognized modes to
	// avoid unintended consequences of a bad config value.
	return true
}

// Clear wipes all cached state and triggers a fresh prefetch from the DB.
// Use when the cache may be stale — e.g. when a persistence error means
// tasks may or may not have been saved.
func (q *cachedQueueReader) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.logger.Info("cache cleared due to persistence uncertainty",
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	)

	q.queue.Clear()
	q.pendingInjectBuffer = nil
	q.prefetchTargetUpper = persistence.MinimumHistoryTaskKey
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

	// Snapshot capacity and upper bound together under one lock. Two separate
	// reads would let a concurrent putTasks or UpdateReadLevel slip in between,
	// giving us a stale starting position for the fetch.
	q.mu.RLock()
	availableCacheSize := q.options.MaxSize() - q.queue.Len()
	upperBound := q.exclusiveUpperBound
	q.mu.RUnlock()

	// If the cache is full, attempt lazy eviction before giving up: evict tasks
	// older than now - TimeEvictionWindow and re-snapshot capacity. Without this,
	// prefetch would skip indefinitely when the read level is stuck (no
	// UpdateReadLevel calls to advance the lower bound), and the window would
	// never grow to cover new tasks.
	if availableCacheSize <= 0 {
		q.mu.Lock()
		q.tryTimeEvict(1)
		availableCacheSize = q.options.MaxSize() - q.queue.Len()
		upperBound = q.exclusiveUpperBound
		q.mu.Unlock()
	}

	if availableCacheSize <= 0 {
		q.logger.Debug("prefetch skipped, cache full")
		return nil // not an error — the loop will reschedule via nextPrefetchDelay
	}

	now := q.clock.Now()

	// Ceiling of the look-ahead window; tasks at or after this time aren't due yet.
	exclusiveMaxKey := persistence.NewHistoryTaskKey(now.Add(q.options.MaxLookAheadWindow()), 0)

	// Start from the existing upper bound so pages don't overlap. On the first
	// run (upperBound is MinimumHistoryTaskKey, nothing fetched yet), anchor to
	// now-TimeEvictionWindow; starting from absolute minimum would pull tasks
	// that timeEvict would drop immediately.
	inclusiveMinTaskKey := upperBound
	if inclusiveMinTaskKey.Equal(persistence.MinimumHistoryTaskKey) {
		inclusiveMinTaskKey = persistence.NewHistoryTaskKey(now.Add(-q.options.TimeEvictionWindow()), 0)
	}

	// Window is already covered; skip the DB round-trip.
	if !inclusiveMinTaskKey.Less(exclusiveMaxKey) {
		q.logger.Debug("prefetch skipped, window already covered",
			tag.Dynamic("inclusiveMinTaskKey", inclusiveMinTaskKey),
			tag.Dynamic("exclusiveMaxKey", exclusiveMaxKey),
		)
		return nil
	}

	// Cap the page to available space (so the insert won't spill into RTrimBySize)
	// and to the configured page size (to bound each round-trip).
	pageSize := availableCacheSize
	if q.options.PrefetchPageSize() < pageSize {
		pageSize = q.options.PrefetchPageSize()
	}

	// Record the prefetch's target window so Inject can buffer tasks that arrive
	// while the DB call is in-flight. Cleared under the write lock after the call.
	q.mu.Lock()
	q.prefetchTargetUpper = exclusiveMaxKey
	q.mu.Unlock()

	resp, err := q.base.GetTask(q.ctx, &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: inclusiveMinTaskKey,
				ExclusiveMaxTaskKey: exclusiveMaxKey,
			},
			NextPageToken: nil,
			NextTaskKey:   inclusiveMinTaskKey,
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  pageSize,
	})

	q.mu.Lock()
	defer q.mu.Unlock()
	defer q.insertBufferedTasks()

	// Clear the prefetch target to signal Inject that no prefetch is in-flight, even on error.
	q.prefetchTargetUpper = persistence.MinimumHistoryTaskKey

	if err != nil {
		q.logger.Error("prefetch failed", tag.Error(err))
		return fmt.Errorf("base.GetTask failed: %w", err)
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

	// If a trim occurred, putTasks already updated the upper bound correctly.
	// Re-advancing would create false coverage.
	if trimmed := q.putTasks(resp.Tasks); trimmed {
		return nil
	}

	// No trim: advance to the appropriate target.
	// Partial page → we've seen the full range, advance to the ceiling.
	// Full page → DB has more, advance only to the next task key.
	target := exclusiveMaxKey
	if len(resp.Tasks) >= pageSize {
		target = resp.Progress.NextTaskKey
	}
	if q.exclusiveUpperBound.Less(target) {
		q.updateExclusiveUpperBound(target)
	}

	// On the first prefetch (upperBound was MinimumHistoryTaskKey), the fetch
	// started at inclusiveMinTaskKey = now - TimeEvictionWindow. Tasks before
	// that anchor were not fetched and will never be in the cache. Advance
	// inclusiveLowerBound to match so that isRangeCovered correctly returns
	// false for those historical ranges, forcing a DB fallback instead of
	// returning 0 tasks from a falsely "covered" range.
	if upperBound.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateInclusiveLowerBound(inclusiveMinTaskKey)
	}

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

// putTasks adds tasks to the cache and enforces the size cap.
// Caller must hold q.mu.
// Returns true if RTrimBySize fired and updated exclusiveUpperBound,
// meaning the caller must not re-advance the bound.
func (q *cachedQueueReader) putTasks(tasks []persistence.Task) bool {
	// taskID=0 is reserved for range-boundary sentinel keys (e.g. exclusiveUpperBound).
	// Real tasks always receive a non-zero ID from generateTaskIDLocked.
	// Inserting sentinels would corrupt the queue ordering.
	filtered := tasks[:0]
	for _, t := range tasks {
		if t.GetTaskID() != 0 {
			filtered = append(filtered, t)
		}
	}

	maxSize := q.options.MaxSize()
	// Lazy eviction: if inserting filtered tasks would exceed MaxSize, evict
	// tasks older than TimeEvictionWindow first to make room.
	q.tryTimeEvict(len(filtered))

	q.queue.PutTasks(filtered)
	newUpper, trimmed := q.queue.RTrimBySize(maxSize)

	if !trimmed {
		return false
	}

	if !newUpper.Greater(persistence.MinimumHistoryTaskKey) {
		// RTrimBySize emptied the cache (MaxSize <= 0). Reset the upper bound to
		// avoid claiming coverage over a range for which the cache holds no tasks.
		q.updateExclusiveUpperBound(persistence.MinimumHistoryTaskKey)
	} else {
		q.updateExclusiveUpperBound(newUpper)
	}
	return true
}

// updateExclusiveUpperBound sets the upper bound and wakes the prefetchLoop.
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

// updateInclusiveLowerBound sets the lowe bound
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
	if !q.exclusiveUpperBound.Equal(persistence.MinimumHistoryTaskKey) &&
		newKey.Greater(q.exclusiveUpperBound) {
		newKey = q.exclusiveUpperBound
	}

	if !newKey.Greater(q.inclusiveLowerBound) {
		return
	}

	q.queue.LTrim(newKey)
	q.updateInclusiveLowerBound(newKey)
}

// runTimeEviction advances inclusiveLowerBound to now - TimeEvictionWindow,
// evicting tasks that are too old to be served from cache.
// tryTimeEvict evicts tasks older than TimeEvictionWindow if adding extraTasks
// would exceed MaxSize. extraTasks=1 checks whether a single slot is available;
// extraTasks=len(batch) checks before a bulk insert.
// Caller must hold q.mu (write).
func (q *cachedQueueReader) tryTimeEvict(extraTasks int) {
	maxSize := q.options.MaxSize()
	if maxSize > 0 && q.queue.Len()+extraTasks > maxSize {
		evictBefore := persistence.NewHistoryTaskKey(
			q.clock.Now().Add(-q.options.TimeEvictionWindow()), 0,
		)
		q.advanceInclusiveLowerBound(evictBefore)
	}
}

// UpdateReadLevel advances the lower bound to the processor's ack position.
// MaximumHistoryTaskKey means "no valid read level" and is treated as minimum.
func (q *cachedQueueReader) UpdateReadLevel(readLevel persistence.HistoryTaskKey) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// MaximumHistoryTaskKey means "no valid read level"; treat as minimum.
	if readLevel.Equal(persistence.MaximumHistoryTaskKey) {
		readLevel = persistence.MinimumHistoryTaskKey
	}

	q.advanceInclusiveLowerBound(readLevel)
}

// isToInjectTask reports whether t should be accepted into the in-memory
// queue immediately. Returns false for sentinel tasks (taskID=0) and for tasks
// outside [inclusiveLowerBound, exclusiveUpperBound).
// Caller must hold q.mu (read or write).
func (q *cachedQueueReader) isToInjectTask(t persistence.Task) bool {
	if t.GetTaskID() == 0 {
		return false
	}
	return q.isTaskCovered(t.GetTaskKey())
}

// isToBufferTask reports whether t should be placed in the pending inject
// buffer so it can be drained into the cache once the in-flight prefetch
// extends the window. Only meaningful when shouldInjectTask returned false.
// Returns false for sentinel tasks (taskID=0) and when no prefetch is in-flight.
// Caller must hold q.mu (read or write).
func (q *cachedQueueReader) isToBufferTask(t persistence.Task) bool {
	if t.GetTaskID() == 0 {
		return false
	}

	key := t.GetTaskKey()
	return !q.prefetchTargetUpper.Equal(persistence.MinimumHistoryTaskKey) &&
		!key.Less(q.inclusiveLowerBound) &&
		key.Less(q.prefetchTargetUpper)
}

// Inject adds tasks within [inclusiveLowerBound, exclusiveUpperBound) to the
// in-memory queue. Tasks outside the window are skipped; the prefetch loop
// loads them as the window advances. No-op when the cache is off.
func (q *cachedQueueReader) Inject(tasks []persistence.Task) {
	if q.isDisabled() {
		q.logger.Debug("inject skipped, cache disabled")
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	logTags := []tag.Tag{
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
	}

	var covered []persistence.Task
	for _, t := range tasks {
		key := t.GetTaskKey()
		logTags := append(logTags, tag.Dynamic("taskKey", key))

		if q.isToInjectTask(t) {
			q.logger.Debug("inject task accepted", logTags...)
			covered = append(covered, t)
			continue
		}

		if q.isToBufferTask(t) {
			q.logger.Debug("inject task buffered, pending prefetch", logTags...)
			q.pendingInjectBuffer = append(q.pendingInjectBuffer, t)
			continue
		}

		q.logger.Debug("inject task skipped", logTags...)
	}

	if len(covered) == 0 {
		q.logger.Debug("no tasks within cache window", logTags...)
		return
	}

	q.putTasks(covered)
}

// insertBufferedTasks inserts buffered tasks that now fall within the
// current cache window. Must be called under q.mu (write lock).
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
	if len(covered) > 0 {
		q.putTasks(covered)
	}
}

// GetTask serves tasks from the cache when the range is fully covered.
// Shadow mode always hits the DB and compares with the cache result to detect
// divergence. Disabled mode bypasses the cache entirely.
func (q *cachedQueueReader) GetTask(ctx context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
	if q.isDisabled() {
		q.logger.Debug("fail back to original get task, cache is disabled")
		return q.base.GetTask(ctx, req)
	}

	q.mu.RLock()
	inclusiveLowerBound := q.inclusiveLowerBound
	logTags := []tag.Tag{
		tag.Dynamic("requestedRange", req.Progress.Range),
		tag.Dynamic("inclusiveLowerBound", q.inclusiveLowerBound),
		tag.Dynamic("exclusiveUpperBound", q.exclusiveUpperBound),
		tag.Dynamic("cacheSize", q.queue.Len()),
	}

	covered := q.isRangeCovered(req.Progress.NextTaskKey, req.Progress.ExclusiveMaxTaskKey)
	if !covered {
		q.mu.RUnlock()
		q.metrics.IncCounter(metrics.CachedQueueMissesCounter)
		q.logger.Debug("cache miss", logTags...)
		return q.base.GetTask(ctx, req)
	}

	cacheResp := q.queue.GetTasks(req)
	q.mu.RUnlock()

	q.metrics.IncCounter(metrics.CachedQueueHitsCounter)
	q.logger.Debug("cache hit", logTags...)

	if q.isShadow() {
		return q.getTaskInShadow(ctx, req, cacheResp, inclusiveLowerBound, logTags)
	}

	return cacheResp, nil
}

// getTaskInShadow always queries the DB and compares the result against the
// cache snapshot. Returns the DB result; mismatches are logged but don't
// affect processing.
func (q *cachedQueueReader) getTaskInShadow(
	ctx context.Context,
	req *GetTaskRequest,
	cacheResp *GetTaskResponse,
	snapshotLowerBound persistence.HistoryTaskKey,
	logTags []tag.Tag,
) (*GetTaskResponse, error) {
	dbResp, err := q.base.GetTask(ctx, req)
	if err != nil {
		q.logger.Error("shadow comparison skipped, base returned error",
			append(logTags, tag.Error(err))...,
		)
		return dbResp, err
	}

	result := findMismatchesInShadow(cacheResp, dbResp, snapshotLowerBound)
	q.reportShadowComparison(result, cacheResp, dbResp, logTags)
	return dbResp, nil
}

// findMismatchesInShadowResult holds the outcome of a shadow comparison.
type findMismatchesInShadowResult struct {
	// MissingFromCache contains task keys present in the DB response but absent
	// from the cache snapshot (after filtering benign eviction and inject races).
	MissingFromCache []persistence.HistoryTaskKey
	// ExtraInCache contains task keys present in the cache snapshot but absent
	// from the DB response.
	ExtraInCache []persistence.HistoryTaskKey
	// NextKeyMismatch is true when the cache and DB disagree on the next-page
	// boundary key, meaning a subsequent GetTask would start at different points.
	NextKeyMismatch bool
	// HasMismatches is true when any of the above fields indicate divergence.
	HasMismatches bool
}

// reportShadowComparison logs the result of a shadow comparison and increments
// the mismatch metric when HasMismatches is true.
func (q *cachedQueueReader) reportShadowComparison(
	result findMismatchesInShadowResult,
	cacheResp *GetTaskResponse,
	dbResp *GetTaskResponse,
	logTags []tag.Tag,
) {
	if !result.HasMismatches {
		q.logger.Debug("shadow comparison matched")
		return
	}

	mismatchTags := append(logTags,
		tag.Dynamic("shadowMismatch.missingFromCache", result.MissingFromCache),
		tag.Dynamic("shadowMismatch.extraInCache", result.ExtraInCache),
		tag.Dynamic("shadowMismatch.nextKeyMismatch", result.NextKeyMismatch),
		tag.Dynamic("shadowMismatch.dbTaskCount", len(dbResp.Tasks)),
		tag.Dynamic("shadowMismatch.cacheTaskCount", len(cacheResp.Tasks)),
	)
	if cacheResp.Progress != nil {
		mismatchTags = append(mismatchTags, tag.Dynamic("shadowMismatch.cacheNextKey", cacheResp.Progress.NextTaskKey))
	}
	if dbResp.Progress != nil {
		mismatchTags = append(mismatchTags, tag.Dynamic("shadowMismatch.dbNextKey", dbResp.Progress.NextTaskKey))
	}

	// NextKeyMismatch is a meaningful divergence even when task sets match, so count it as a mismatch.
	// ExtraInCache is benign given task indepetency, but still a mismatch to be observed and counted.
	if result.NextKeyMismatch || len(result.MissingFromCache) > 0 {
		q.metrics.IncCounter(metrics.CachedQueueMismatchCounter)
		q.logger.Warn("shadow comparison mismatch", mismatchTags...)
	} else {
		q.logger.Info("shadow comparison mismatch (only extra tasks in cache)", mismatchTags...)
	}
}

// findMismatchesInShadow compares a cache snapshot response against the DB
// response for the same request and returns a findMismatchesInShadowResult.
//
// Task comparison filters one benign case:
//   - keys below preFetchLowerBound: evicted since snapshot was taken
//
// Compares by taskID rather than full key: injected tasks have nanosecond
// timestamps whereas Cassandra stores millisecond precision, so full-key
// comparison produces false positives for every injected task.
//
// Note: there is no second cache re-read to filter in-flight Inject races.
// By the scheduledTaskMaxReadLevel invariant, every new task has
// visibilityTimestamp > readLevel, so it cannot appear in the GetTask range
// at commit time. Any DB task absent from the cache snapshot is a real miss.
func findMismatchesInShadow(
	snapshotResp *GetTaskResponse, // cache response captured before the DB read
	dbResp *GetTaskResponse,
	preFetchLowerBound persistence.HistoryTaskKey, // lower bound at snapshot time
) findMismatchesInShadowResult {
	snapshotIDs := make(map[int64]struct{}, len(snapshotResp.Tasks))
	for _, t := range snapshotResp.Tasks {
		snapshotIDs[t.GetTaskID()] = struct{}{}
	}
	dbIDs := make(map[int64]struct{}, len(dbResp.Tasks))
	for _, t := range dbResp.Tasks {
		dbIDs[t.GetTaskID()] = struct{}{}
	}

	var result findMismatchesInShadowResult

	// DB tasks missing from cache snapshot.
	for _, t := range dbResp.Tasks {
		if _, found := snapshotIDs[t.GetTaskID()]; found {
			continue // in snapshot
		}
		if t.GetTaskKey().Less(preFetchLowerBound) {
			continue // evicted since snapshot
		}
		result.MissingFromCache = append(result.MissingFromCache, t.GetTaskKey())
	}

	// Cache snapshot tasks missing from DB — tracked separately; extra tasks are
	// benign (Inject races, tasks written but not yet committed when the DB read fires).
	for _, t := range snapshotResp.Tasks {
		if _, found := dbIDs[t.GetTaskID()]; found {
			continue // in DB result
		}
		if t.GetTaskKey().Less(preFetchLowerBound) {
			continue // evicted since snapshot
		}
		result.ExtraInCache = append(result.ExtraInCache, t.GetTaskKey())
	}

	// Compare progress: if NextTaskKey differs the caller would start the next
	// page at a different position — a meaningful divergence even when task sets
	// match. Guard against nil Progress (defensive; both sides should always set
	// it, but mocks sometimes omit it).
	if snapshotResp.Progress != nil && dbResp.Progress != nil {
		result.NextKeyMismatch = !snapshotResp.Progress.NextTaskKey.Equal(dbResp.Progress.NextTaskKey)
	}

	// HasMismatches is true when any of the above fields indicate divergence,
	// including ExtraInCache which is benign but still a mismatch to be observed.
	result.HasMismatches = len(result.MissingFromCache) > 0 || len(result.ExtraInCache) > 0 || result.NextKeyMismatch
	return result
}

// LookAHead returns the next task at or after req.InclusiveMinTaskKey. Serves
// from cache when the request falls within the prefetched window. Bypasses
// cache when disabled or in shadow mode.
func (q *cachedQueueReader) LookAHead(ctx context.Context, req *LookAHeadRequest) (*LookAHeadResponse, error) {
	if q.isDisabled() || q.isShadow() {
		q.logger.Debug("fail back to original look-ahead, cache is disabled or in shadow mode")
		return q.base.LookAHead(ctx, req)
	}

	q.mu.RLock()
	exclusiveUpperBound := q.exclusiveUpperBound
	inclusiveLowerBound := q.inclusiveLowerBound

	// MinimumHistoryTaskKey is the sentinel meaning "not yet initialized"; the
	// cache window is valid only after the first prefetch sets a real upper bound.
	// Also require the request starts at or after the lower bound — time eviction
	// can advance it past the caller's min key, in which case the cache has
	// evicted those tasks and the DB must be consulted.
	// Upper bound is exclusive: req == upper means the cache has no tasks there,
	// so use strict Less rather than !Greater to avoid a false coverage claim.
	covered := exclusiveUpperBound.Greater(persistence.MinimumHistoryTaskKey) &&
		!req.InclusiveMinTaskKey.Less(inclusiveLowerBound) &&
		req.InclusiveMinTaskKey.Less(exclusiveUpperBound)

	var cacheTask persistence.Task
	if covered {
		cacheTask = q.queue.LookAHead(req.InclusiveMinTaskKey)
	}
	q.mu.RUnlock()

	if covered {
		q.logger.Debug("look-ahead cache hit",
			tag.Dynamic("inclusiveMinTaskKey", req.InclusiveMinTaskKey),
			tag.Dynamic("exclusiveUpperBound", exclusiveUpperBound),
			tag.Dynamic("taskFound", cacheTask != nil),
		)
		return &LookAHeadResponse{
			Task:             cacheTask,
			LookAheadMaxTime: exclusiveUpperBound.GetScheduledTime(),
		}, nil
	}

	q.logger.Debug("look-ahead cache miss",
		tag.Dynamic("inclusiveMinTaskKey", req.InclusiveMinTaskKey),
		tag.Dynamic("exclusiveUpperBound", exclusiveUpperBound),
	)
	return q.base.LookAHead(ctx, req)
}
