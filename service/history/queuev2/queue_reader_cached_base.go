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
	"maps"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

//go:generate mockgen -package $GOPACKAGE -destination queue_reader_cached_mock.go github.com/uber/cadence/service/history/queuev2 CachedQueueReader

// CachedQueueReader extends QueueReader with cache injection, read-level control, and a background
// lifecycle. Both the scheduled (timer) and immediate (transfer) cached readers implement it; each
// supplies its own Start/Stop. The timer reader runs a prefetch loop in a goroutine, while the
// transfer reader only flips lifecycle state and logs, since it has no background work.
type CachedQueueReader interface {
	QueueReader
	// Inject adds tasks that have just been persisted into the in-memory cache.
	// Tasks outside the current cached window are silently dropped or buffered.
	Inject(tasks []persistence.Task)
	// Clear wipes all cached state.
	// Call when the cache may be stale (e.g. after a persistence error).
	Clear()
	// UpdateReadLevel advances the eviction lower bound to readLevel,
	// dropping tasks the processor has already passed.
	UpdateReadLevel(readLevel persistence.HistoryTaskKey)
	// Start begins the reader's lifecycle. Idempotent.
	Start()
	// Stop ends the reader's lifecycle. Idempotent.
	Stop()
}

// cachedQueueReaderOptions is the configuration shared by both cached queue readers, held by the
// base. Each concrete reader populates it from its own dynamic-config keys (timer vs transfer), so
// the property names in the config file stay independent even though the code is shared.
type cachedQueueReaderOptions struct {
	// Mode controls cache behavior: "enabled" uses cache, anything else (including "disabled") disables.
	Mode dynamicproperties.StringPropertyFnWithShardIDFilter
	// MaxSize is the maximum number of tasks the cache may hold at once.
	// Insertions that would exceed this limit trigger eviction first.
	MaxSize dynamicproperties.IntPropertyFn
	// ShadowSampleInterval controls how often, at most, a GetTask call in "enabled" mode
	// is diverted through the shadow comparison path for continuous regression detection.
	// <= 0 disables sampling.
	ShadowSampleInterval dynamicproperties.DurationPropertyFn
}

// cachedQueueReaderBase holds the state and behaviour shared by the scheduled (timer) and
// immediate (transfer) cached queue readers: the cached window, coverage checks, range-ID
// fencing, the read path (including shadow-mode comparison), and eviction bound bookkeeping.
//
// Both concrete readers embed *cachedQueueReaderBase and supply only their specific behaviour
// (population via Inject, cap-guard direction, and lifecycle). Field/method promotion keeps
// callers agnostic of which concrete reader they hold.
type cachedQueueReaderBase struct {
	base    QueueReader
	shard   shard.Context
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

	// exclusiveUpperBound is the exclusive end of the cached window. Tasks with
	// key < exclusiveUpperBound are covered by the cache if they exist in the DB.
	// Invariant: inclusiveLowerBound <= exclusiveUpperBound.
	// Always update via updateExclusiveUpperBound.
	exclusiveUpperBound persistence.HistoryTaskKey

	// lastRangeID is the shard rangeID observed when the cache was last valid.
	// A change means the shard was re-acquired and the cache may be stale.
	// Protected by mu.
	lastRangeID int64

	// lastShadowSampleUnixNano is the unix-nano timestamp of the last periodic shadow
	// sample check performed while in "enabled" mode. When concurrent GetTask calls race
	// on the same due window, only one of them wins the update, so at most one sample is
	// taken per window. Zero value means no sample has been taken yet, so the first
	// eligible call fires immediately.
	lastShadowSampleUnixNano atomic.Int64

	// onUpperBoundUpdatedLockedFn is called (under mu) after exclusiveUpperBound changes. The
	// scheduled/timer reader wires this to notifyPrefetch so the prefetch loop recomputes its
	// timer; the immediate/transfer reader leaves it nil (no background loop to notify).
	onUpperBoundUpdatedLockedFn func()

	// onClearLockedFn is called (under mu) at the end of clearLocked, after the shared window
	// state has been reset. The scheduled/timer reader wires this to reset its prefetch-only
	// state (in-flight target and pending inject buffer); the immediate/transfer reader leaves
	// it nil (no prefetch state to reset).
	onClearLockedFn func()

	// status holds the daemon lifecycle state (DaemonStatusInitialized / Started / Stopped).
	// It lives on the base so both concrete readers share the same lifecycle bookkeeping; each
	// supplies its own Start/Stop, which flip it with an atomic compare-and-swap on this field.
	status int32
}

func newCachedQueueReaderBase(
	base QueueReader,
	queue InMemQueue,
	shard shard.Context,
	clockSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
	options *cachedQueueReaderOptions,
) *cachedQueueReaderBase {
	return &cachedQueueReaderBase{
		base:                base,
		shard:               shard,
		queue:               queue,
		options:             options,
		clock:               clockSource,
		logger:              logger,
		metrics:             metricsScope,
		inclusiveLowerBound: persistence.MinimumHistoryTaskKey,
		exclusiveUpperBound: persistence.MinimumHistoryTaskKey,
		lastRangeID:         shard.GetRangeID(),
		status:              common.DaemonStatusInitialized,
	}
}

// isEnabled returns true if the cache is fully enabled
func (q *cachedQueueReaderBase) isEnabled() bool {
	return q.options.Mode(q.shard.GetShardID()) == "enabled"
}

// isShadow returns true when cache runs in shadow mode — results are compared
// against the DB but the DB result is returned to the processor.
func (q *cachedQueueReaderBase) isShadow() bool {
	return q.options.Mode(q.shard.GetShardID()) == "shadow"
}

// isCachedQueueReaderDisabled reports whether the given mode disables the cached queue reader.
func isCachedQueueReaderDisabled(mode string) bool {
	switch mode {
	case "enabled", "shadow":
		return false
	default:
		return true
	}
}

// isDisabled returns true for the "disabled" mode and for any unrecognised value
func (q *cachedQueueReaderBase) isDisabled() bool {
	return isCachedQueueReaderDisabled(q.options.Mode(q.shard.GetShardID()))
}

// IsEmpty reports whether the cache queue reader is empty
func (q *cachedQueueReaderBase) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.exclusiveUpperBound.Equal(persistence.MinimumHistoryTaskKey)
}

// clearIfNotEmpty clears cached state only when the cache has data
func (q *cachedQueueReaderBase) clearIfNotEmpty() {
	if !q.IsEmpty() {
		q.Clear()
	}
}

// Clear wipes all cached state.
func (q *cachedQueueReaderBase) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.logger.Info("cache fully cleared",
		tag.Dynamic("cacheState", q.getState()),
	)

	q.clearLocked()
}

// clearLocked wipes all cached state. Caller must hold q.mu for writing.
func (q *cachedQueueReaderBase) clearLocked() {
	q.queue.Clear()
	q.updateInclusiveLowerBound(persistence.MinimumHistoryTaskKey)
	q.updateExclusiveUpperBound(persistence.MinimumHistoryTaskKey)
	if q.onClearLockedFn != nil {
		q.onClearLockedFn()
	}
}

// isRangeIDChangedLocked reports whether the shard's current rangeID differs
// from the last observed value. Caller must hold q.mu (read or write).
func (q *cachedQueueReaderBase) isRangeIDChangedLocked() bool {
	return q.shard.GetRangeID() != q.lastRangeID
}

// isRangeIDChanged reports whether the shard's current rangeID differs
// from the last observed value.
func (q *cachedQueueReaderBase) isRangeIDChanged() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.isRangeIDChangedLocked()
}

// fallbackIfRangeIDChanged clears cache if rangeID changed by more than 1
// (shard moved away and was re-acquired). A change of exactly 1 means the same
// host reacquired the shard — cache remains valid.
// Returns true if cache was cleared.
func (q *cachedQueueReaderBase) fallbackIfRangeIDChanged() bool {
	if !q.isRangeIDChanged() {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.isRangeIDChangedLocked() {
		return false
	}

	newRangeID := q.shard.GetRangeID()
	prevRangeID := q.lastRangeID
	q.lastRangeID = newRangeID

	if newRangeID == prevRangeID+1 {
		// Same host reacquired the shard. Cache still valid.
		q.logger.Info("rangeID changed on same host, cache not cleared",
			tag.Dynamic("previousRangeID", prevRangeID),
			tag.Dynamic("newRangeID", newRangeID),
		)
		return false
	}

	// rangeID changed by more than 1: possible stale cache.
	q.logger.Info("rangeID changed, clearing cache",
		tag.Dynamic("previousRangeID", prevRangeID),
		tag.Dynamic("newRangeID", newRangeID),
	)
	q.clearLocked()
	return true
}

// isRangeCovered reports whether [inclusiveMin, exclusiveMax) falls fully
// within the cached window [inclusiveLowerBound, exclusiveUpperBound).
// Caller must hold q.mu (read or write).
func (q *cachedQueueReaderBase) isRangeCovered(inclusiveMin, exclusiveMax persistence.HistoryTaskKey) bool {
	return !inclusiveMin.Less(q.inclusiveLowerBound) && !exclusiveMax.Greater(q.exclusiveUpperBound)
}

// isTaskCovered reports whether the given task key falls within the cached window.
// Caller must hold q.mu (read or write).
func (q *cachedQueueReaderBase) isTaskCovered(key persistence.HistoryTaskKey) bool {
	return !key.Less(q.inclusiveLowerBound) && key.Less(q.exclusiveUpperBound)
}

// updateExclusiveUpperBound sets the upper bound and runs the onUpperBoundUpdatedLockedFn hook if set.
// Caller must hold q.mu.
func (q *cachedQueueReaderBase) updateExclusiveUpperBound(newKey persistence.HistoryTaskKey) {
	if q.logger.DebugOn() {
		q.logger.Debug("upper bound is updated",
			tag.Dynamic("cacheState", q.getState()),
			tag.Dynamic("newUpperBound", newKey),
		)
	}

	q.exclusiveUpperBound = newKey
	q.metrics.RecordHistogramValue(metrics.CachedQueueSizeHistogram, float64(q.queue.Len()))
	if q.onUpperBoundUpdatedLockedFn != nil {
		q.onUpperBoundUpdatedLockedFn()
	}
}

// updateInclusiveLowerBound sets the lower bound
// Caller must hold q.mu.
func (q *cachedQueueReaderBase) updateInclusiveLowerBound(newKey persistence.HistoryTaskKey) {
	if q.logger.DebugOn() {
		q.logger.Debug("lower bound is updated",
			tag.Dynamic("cacheState", q.getState()),
			tag.Dynamic("newLowerBound", newKey),
		)
	}

	q.inclusiveLowerBound = newKey
	q.metrics.RecordHistogramValue(metrics.CachedQueueSizeHistogram, float64(q.queue.Len()))
}

// advanceInclusiveLowerBound advances inclusiveLowerBound to newKey if it's
// ahead, trimming evicted tasks. Caps at exclusiveUpperBound when set to
// preserve the lower <= upper invariant.
// Caller must hold q.mu (write).
func (q *cachedQueueReaderBase) advanceInclusiveLowerBound(newKey persistence.HistoryTaskKey) {
	if !newKey.Greater(q.inclusiveLowerBound) {
		return
	}

	if !newKey.Less(q.exclusiveUpperBound) {
		newKey = q.exclusiveUpperBound
	}

	q.queue.LTrim(newKey)
	q.updateInclusiveLowerBound(newKey)
}

// UpdateReadLevel advances the lower bound to the processor's ack position.
// MaximumHistoryTaskKey means "no valid read level" and skipped
func (q *cachedQueueReaderBase) UpdateReadLevel(readLevel persistence.HistoryTaskKey) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if readLevel.Equal(persistence.MaximumHistoryTaskKey) {
		return
	}

	q.advanceInclusiveLowerBound(readLevel)
}

// inject status values reported via metrics.CachedQueueInjectAttemptCounter,
// one per outcome branch of Inject.
const (
	injectStatusInjected     = "injected"
	injectStatusBuffered     = "buffered"
	injectStatusDroppedBelow = "dropped_below"
	injectStatusDroppedUpper = "dropped_upper"
)

// emitInjectStatusCount records the number of injected tasks that took the given
// outcome path, tagged by status. Zero counts are skipped to avoid emitting empty series.
func (q *cachedQueueReaderBase) emitInjectStatusCount(status string, count int64) {
	if count == 0 {
		return
	}
	q.metrics.Tagged(metrics.StatusTag(status)).AddCounter(metrics.CachedQueueInjectAttemptCounter, count)
}

// resolveInclusiveMinTaskKey returns the effective InclusiveMinTaskKey for req, accounting for
// the NextPageToken/NextTaskKey pagination-continuation override in GetTaskProgress. The second
// return value is false only when NextPageToken is set but NextTaskKey was never populated — an
// edge case callers should handle by delegating to the base reader.
func resolveInclusiveMinTaskKey(req *GetTaskRequest) (persistence.HistoryTaskKey, bool) {
	inclusiveMinTaskKey := req.Progress.Range.InclusiveMinTaskKey
	if req.Progress.NextPageToken != nil {
		if req.Progress.NextTaskKey.Equal(persistence.MinimumHistoryTaskKey) {
			return persistence.HistoryTaskKey{}, false
		}
		inclusiveMinTaskKey = req.Progress.NextTaskKey
	}
	return inclusiveMinTaskKey, true
}

// LookAHead is the default look-ahead: it delegates straight to the underlying base reader. It
// exists on the base so cached readers without cache-aware look-ahead (the immediate/transfer
// reader) satisfy QueueReader purely by embedding, without re-implementing it. The scheduled/timer
// reader overrides this with a cache-aware version, since look-ahead is a scheduled concern.
func (q *cachedQueueReaderBase) LookAHead(ctx context.Context, req *LookAHeadRequest) (*LookAHeadResponse, error) {
	return q.base.LookAHead(ctx, req)
}

// GetTask serves tasks from the cache when the starting key is covered.
// Disabled mode bypasses the cache entirely.
func (q *cachedQueueReaderBase) GetTask(ctx context.Context, req *GetTaskRequest) (*GetTaskResponse, error) {
	if q.isDisabled() {
		return q.base.GetTask(ctx, req)
	}

	if q.fallbackIfRangeIDChanged() {
		q.logger.Info("GetTask falling back to base reader after rangeID change")
		return q.base.GetTask(ctx, req)
	}

	inclusiveMinTaskKey, ok := resolveInclusiveMinTaskKey(req)
	if !ok {
		q.logger.Info("NextPageToken is set but NextTaskKey is not set, delegating to base reader", tag.Dynamic("getTaskRequest", req))
		return q.base.GetTask(ctx, req)
	}
	exclusiveMaxTaskKey := req.Progress.Range.ExclusiveMaxTaskKey

	q.mu.RLock()
	covered := q.isRangeCovered(inclusiveMinTaskKey, exclusiveMaxTaskKey)

	logTags := []tag.Tag{
		tag.Dynamic("getTaskRequest", req),
		tag.Dynamic("cacheState", q.getState()),
		tag.Dynamic("inclusiveMinTaskKey", inclusiveMinTaskKey),
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

	if q.isShadow() || q.isPeriodicShadowSample() {
		return q.getTaskInShadow(ctx, req, cacheResp, logTags)
	}

	return cacheResp, nil
}

// isPeriodicShadowSample reports whether this call should be diverted into a shadow
// comparison as part of the periodic health check for "enabled" mode. Gated on elapsed
// wall-clock time rather than a request counter, so the check cadence is independent of
// request volume. When multiple callers race on the same due window, only one of them
// wins and performs the sample; the timestamp is advanced before the comparison runs, so
// the interval is measured from attempt to attempt rather than success to success.
//
// TODO: this periodic shadow sample check is temporary scaffolding for continuous
// regression detection while "enabled" mode is being rolled out. It gives operators
// an ongoing signal that the cache still agrees with the DB after promotion out of
// "shadow" mode. Remove once CachedQueueReader is enabled by default and "shadow"
// mode rollouts (and this periodic variant of it) are no longer needed.
func (q *cachedQueueReaderBase) isPeriodicShadowSample() bool {
	interval := q.options.ShadowSampleInterval()
	if interval <= 0 {
		return false
	}

	now := q.clock.Now().UnixNano()
	last := q.lastShadowSampleUnixNano.Load()
	if now-last < interval.Nanoseconds() {
		return false
	}
	if !q.lastShadowSampleUnixNano.CompareAndSwap(last, now) {
		return false
	}
	return true
}

// getState returns a snapshot of the cached queue reader's key state variables for logging and debugging.
// Caller must hold q.mu (read or write).
func (q *cachedQueueReaderBase) getState() cachedQueueReaderState {
	return cachedQueueReaderState{
		InclusiveLowerBound: q.inclusiveLowerBound,
		ExclusiveUpperBound: q.exclusiveUpperBound,
		CacheSize:           q.queue.Len(),
	}
}

// cachedQueueReaderState is a snapshot of the cached queue reader's key state variables for logging and debugging.
type cachedQueueReaderState struct {
	InclusiveLowerBound persistence.HistoryTaskKey `json:"inclusiveLowerBound"`
	ExclusiveUpperBound persistence.HistoryTaskKey `json:"exclusiveUpperBound"`
	CacheSize           int                        `json:"cacheSize"`
}

// getTaskInShadow queries the DB for the same request, compares the result
// against the cache snapshot, and returns the DB result. Mismatches are
// logged but do not affect processing.
func (q *cachedQueueReaderBase) getTaskInShadow(
	ctx context.Context,
	req *GetTaskRequest,
	cacheResp *GetTaskResponse,
	logTags []tag.Tag,
) (*GetTaskResponse, error) {
	dbResp, err := q.base.GetTask(ctx, req)
	if err != nil {
		q.logger.Error("shadow comparison skipped, base returned error",
			append(logTags, tag.Error(err))...,
		)
		return dbResp, err
	}
	result := q.findMismatchesInShadow(cacheResp, dbResp, req)
	q.reportShadowComparison(result, logTags)
	return dbResp, nil
}

// shadowMismatchLogLimit caps the number of tasks logged
const shadowMismatchLogLimit = 100

// capShadowMismatchSlice returns the first shadowMismatchLogLimit elements of s, or s itself if shorter.
func capShadowMismatchSlice[T any](s []T) []T {
	if len(s) <= shadowMismatchLogLimit {
		return s
	}
	return s[:shadowMismatchLogLimit]
}

// shadowMismatchTaskInfo holds identifying information about a task mismatch for logging purposes.
type shadowMismatchTaskInfo struct {
	TaskKey persistence.HistoryTaskKey `json:"taskKey"`
	RunID   string                     `json:"runID"`
}

// toShadowMismatchTaskInfo extracts the identifying information from a persistence.Task for logging mismatches.
func toShadowMismatchTaskInfo(t persistence.Task) shadowMismatchTaskInfo {
	return shadowMismatchTaskInfo{
		TaskKey: t.GetTaskKey(),
		RunID:   t.GetRunID(),
	}
}

// TaskMismatches groups the tasks found in one rangeID-relative bucket of a shadow comparison.
type TaskMismatches struct {
	// RangeIDs holds the distinct rangeIDs encoded in this bucket's tasks. Left unpopulated
	// for the current-range bucket, where it would always equal CurrentRangeID.
	RangeIDs []int64 `json:"rangeIDs,omitempty"`
	// Extra holds tasks present in the cache snapshot but absent from the DB response.
	Extra []shadowMismatchTaskInfo `json:"extra,omitempty"`
	// Missed holds tasks present in the DB response but absent from the cache snapshot.
	Missed []shadowMismatchTaskInfo `json:"missed,omitempty"`
	// TaskIDBoundaryNoise holds DB-only tasks that are not real mismatches: Cassandra's timer
	// task query range-filters only on scheduledTime, never taskID, so at a shared boundary
	// timestamp the DB can return tasks below the requested floor's taskID that the (correctly,
	// more strictly filtered) cache excludes. A task's rangeID is independent of this check, so
	// it is bucketed the same way Missed/Extra are. Kept separate from Missed so it doesn't
	// affect mismatch severity or CachedQueueShadowMismatchCounter, while remaining visible.
	TaskIDBoundaryNoise []shadowMismatchTaskInfo `json:"taskIDBoundaryNoise,omitempty"`
}

func (m TaskMismatches) isEmpty() bool {
	return len(m.Extra) == 0 && len(m.Missed) == 0
}

func (m TaskMismatches) cap() TaskMismatches {
	m.RangeIDs = capShadowMismatchSlice(m.RangeIDs)
	m.Extra = capShadowMismatchSlice(m.Extra)
	m.Missed = capShadowMismatchSlice(m.Missed)
	m.TaskIDBoundaryNoise = capShadowMismatchSlice(m.TaskIDBoundaryNoise)
	return m
}

// findMismatchesInShadowResult holds the outcome of a shadow comparison, with tasks partitioned
// by their encoded rangeID relative to the shard's CurrentRangeID at comparison time.
type findMismatchesInShadowResult struct {
	// NewRange holds tasks whose rangeID is greater than CurrentRangeID — proof this host is
	// stale (a newer owner already created tasks, or a prefetch already pulled tasks from a
	// just-renewed range). Its presence means the rest of the comparison isn't trustworthy.
	NewRange TaskMismatches `json:"newRange,omitempty"`
	// CurrentRange holds tasks whose rangeID equals CurrentRangeID.
	CurrentRange TaskMismatches `json:"currentRange,omitempty"`
	// PreviousRange holds tasks whose rangeID is less than CurrentRangeID — leftovers from
	// before the current ownership period.
	PreviousRange TaskMismatches `json:"previousRange,omitempty"`
	// CurrentRangeID is the shard's rangeID at the time of comparison.
	CurrentRangeID int64 `json:"currentRangeID"`
	// CacheTaskCount is the number of tasks in the cache snapshot.
	CacheTaskCount int `json:"cacheTaskCount"`
	// DBTaskCount is the number of tasks in the DB response.
	DBTaskCount int `json:"dbTaskCount"`
}

// getTaskRangeID extracts the rangeID encoded in taskID, which is assigned at task creation time and immutable.
func (q *cachedQueueReaderBase) getTaskRangeID(taskID int64) int64 {
	return taskID >> int64(q.shard.GetConfig().RangeSizeBits)
}

// reportShadowComparison logs exactly one line describing the outcome of a shadow comparison,
// in order of decreasing severity:
//  1. NewRange non-empty: this host is stale; nothing else is evaluated.
//  2. CurrentRange.Missed non-empty: a task the queue processor may have skipped serving from cache.
//  3. CurrentRange.Extra, or anything in PreviousRange: a benign or inconclusive finding, logged
//     for visibility only.
//  4. Otherwise: cache and DB agreed.
func (q *cachedQueueReaderBase) reportShadowComparison(result findMismatchesInShadowResult, logTags []tag.Tag) {
	// Metric emission must run before capping below: capping truncates the slices for
	// logging, which would undercount the metric for large comparisons.
	q.emitShadowMismatchMetrics(result)

	// Cap the number of mismatched task keys logged to avoid excessively large logs.
	result.NewRange = result.NewRange.cap()
	result.CurrentRange = result.CurrentRange.cap()
	result.PreviousRange = result.PreviousRange.cap()

	logTags = append(logTags, tag.Dynamic("shadowComparison", result))

	switch {
	case !result.NewRange.isEmpty():
		q.logger.Info("stale shard owner, no check for mismatches", logTags...)
	case len(result.CurrentRange.Missed) > 0:
		q.logger.Warn("potential severe mismatch between db and cache states", logTags...)
	case len(result.CurrentRange.Extra) > 0 || !result.PreviousRange.isEmpty():
		q.logger.Info("potential non-critical mismatch between db and cache states", logTags...)
	default:
		q.logger.Debug("shadow comparison matched", logTags...)
	}
}

// emitShadowMismatchMetrics records CachedQueueShadowMismatchCounter for both the current-range
// and previous-range missed-task buckets, tagged by which bucket they came from so the two never
// mix into one series. Skipped entirely when NewRange is non-empty: a stale shard owner makes the
// whole comparison untrustworthy, not just the log severity.
func (q *cachedQueueReaderBase) emitShadowMismatchMetrics(result findMismatchesInShadowResult) {
	if !result.NewRange.isEmpty() {
		return
	}
	if len(result.CurrentRange.Missed) > 0 {
		q.metrics.Tagged(metrics.Range("current")).
			AddCounter(metrics.CachedQueueShadowMismatchCounter, int64(len(result.CurrentRange.Missed)))
	}
	if len(result.PreviousRange.Missed) > 0 {
		q.metrics.Tagged(metrics.Range("previous")).
			AddCounter(metrics.CachedQueueShadowMismatchCounter, int64(len(result.PreviousRange.Missed)))
	}
}

// findMismatchesInShadow compares a cache snapshot response against the DB response.
//
// Task comparison uses taskID as the primary key. NextTaskKey is intentionally
// not compared: Cassandra commonly returns a non-empty paging cursor even on
// the last page, which causes the DB reader to report lastTask.Next() while
// the cache (which knows its window is exhausted) reports exclusiveMaxTaskKey.
// Comparing these would produce false-positive mismatches under normal
// production traffic without indicating any real divergence in task data.
//
// Tasks missing from one side are bucketed by comparing their encoded rangeID (assigned
// at creation and immutable) against the shard's CurrentRangeID: greater into NewRange,
// equal into CurrentRange, less into PreviousRange. DB tasks missing from the cache land
// in the bucket's Missed field; cache tasks missing from the DB response land in Extra.
//
// A DB task missing from the cache is first checked against req's effective InclusiveMinTaskKey
// (resolved the same way GetTask resolves it, via resolveInclusiveMinTaskKey): Cassandra's timer
// task query enforces scheduledTime >= that floor's scheduledTime as a hard bound but never
// filters on taskID, so the only way a DB task's key can compare less than the floor is a shared
// boundary timestamp with a lower taskID — the cache correctly excludes it under its own
// (taskID-inclusive) filtering, and it is not a real mismatch. Such tasks land in the same
// rangeID bucket's TaskIDBoundaryNoise field instead of its Missed field: the boundary-noise
// check is orthogonal to which range the task's taskID encodes, so it can occur in any bucket.
func (q *cachedQueueReaderBase) findMismatchesInShadow(
	cacheResp *GetTaskResponse,
	dbResp *GetTaskResponse,
	req *GetTaskRequest,
) findMismatchesInShadowResult {
	inclusiveMinTaskKey, ok := resolveInclusiveMinTaskKey(req)
	if !ok {
		// Unreachable in practice: GetTask already returned early via the base reader for this
		// same req before getTaskInShadow could be reached. Fall back to the safe default of
		// never suppressing a mismatch as boundary noise.
		inclusiveMinTaskKey = persistence.MinimumHistoryTaskKey
	}

	cacheTaskKeys := make(map[int64]struct{}, len(cacheResp.Tasks))
	for _, t := range cacheResp.Tasks {
		cacheTaskKeys[t.GetTaskID()] = struct{}{}
	}
	dbTaskKeys := make(map[int64]struct{}, len(dbResp.Tasks))
	for _, t := range dbResp.Tasks {
		dbTaskKeys[t.GetTaskID()] = struct{}{}
	}

	var (
		result           findMismatchesInShadowResult
		currentRangeID   = q.shard.GetRangeID()
		newRangeIDs      = map[int64]struct{}{}
		previousRangeIDs = map[int64]struct{}{}
	)

	// bucket returns the TaskMismatches this taskRangeID belongs to, relative to currentRangeID,
	// recording the rangeID for later visibility when it isn't the current one.
	bucket := func(taskRangeID int64) *TaskMismatches {
		switch {
		case taskRangeID > currentRangeID:
			newRangeIDs[taskRangeID] = struct{}{}
			return &result.NewRange
		case taskRangeID < currentRangeID:
			previousRangeIDs[taskRangeID] = struct{}{}
			return &result.PreviousRange
		default:
			return &result.CurrentRange
		}
	}

	for _, t := range dbResp.Tasks {
		if _, ok := cacheTaskKeys[t.GetTaskID()]; ok {
			// Task is present in both DB and cache
			continue
		}

		b := bucket(q.getTaskRangeID(t.GetTaskID()))
		if t.GetTaskKey().Less(inclusiveMinTaskKey) {
			b.TaskIDBoundaryNoise = append(b.TaskIDBoundaryNoise, toShadowMismatchTaskInfo(t))
			continue
		}
		b.Missed = append(b.Missed, toShadowMismatchTaskInfo(t))
	}

	for _, t := range cacheResp.Tasks {
		if _, ok := dbTaskKeys[t.GetTaskID()]; ok {
			// Task is present in DB
			continue
		}

		b := bucket(q.getTaskRangeID(t.GetTaskID()))
		b.Extra = append(b.Extra, toShadowMismatchTaskInfo(t))
	}

	result.NewRange.RangeIDs = slices.Collect(maps.Keys(newRangeIDs))
	result.PreviousRange.RangeIDs = slices.Collect(maps.Keys(previousRangeIDs))
	result.CurrentRangeID = currentRangeID
	result.DBTaskCount = len(dbResp.Tasks)
	result.CacheTaskCount = len(cacheResp.Tasks)

	return result
}
