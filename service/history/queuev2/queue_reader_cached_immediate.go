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
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

// cachedImmediateQueueReader is the immediate (transfer) cached queue reader. It embeds the shared
// cachedQueueReaderBase and populates the cache purely from injection; there is no prefetch cycle.
// Unlike the timer reader, its cap guard drops the OLDEST tasks (LTrimBySize).
type cachedImmediateQueueReader struct {
	*cachedQueueReaderBase
}

func newCachedImmediateQueueReaderWithOptions(
	base QueueReader,
	queue InMemQueue,
	shard shard.Context,
	clockSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
	options *cachedQueueReaderOptions,
) *cachedImmediateQueueReader {
	return &cachedImmediateQueueReader{
		cachedQueueReaderBase: newCachedQueueReaderBase(
			base,
			queue,
			shard,
			clockSource,
			logger,
			metricsScope,
			options,
		),
	}
}

// Start marks the reader started. The log line is kept to simplify debugging if needed.
func (q *cachedImmediateQueueReader) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	q.logger.Info("Immediate Cached Queue Reader state changed", tag.LifeCycleStarted)
}

// Stop marks the reader stopped. The log line is kept to simplify debugging if needed.
func (q *cachedImmediateQueueReader) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	q.logger.Info("Immediate Cached Queue Reader state changed", tag.LifeCycleStopped)
}

// Inject adds tasks that have just been persisted into the in-memory cache. The first batch opens
// the window: the lower bound anchors to the earliest injected key, the upper bound is fixed to the
// maximum. No-op when the cache is off.
func (q *cachedImmediateQueueReader) Inject(tasks []persistence.Task) {
	if q.isDisabled() {
		// Clear stale cache so re-enabling starts fresh instead of serving outdated
		// boundaries that cause cache misses.
		q.clearIfNotEmpty()
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	var injected, droppedBelow int64

	var toPut []persistence.Task
	for _, t := range tasks {
		if t.GetTaskID() == 0 {
			// no tasks with taskID == 0 are expected
			continue
		}
		key := t.GetTaskKey()
		// Below the lower bound: already evicted, so reads fall through to the DB. Drop it.
		if key.Less(q.inclusiveLowerBound) {
			droppedBelow++
			q.logger.Warn("task key is below the lower bound, dropping task",
				tag.Dynamic("taskKey", key),
				tag.Dynamic("cacheState", q.getState()),
			)
			continue
		}
		injected++
		if q.logger.DebugOn() {
			q.logger.Debug("injecting task",
				tag.Dynamic("taskKey", key),
				tag.Dynamic("cacheState", q.getState()),
			)
		}
		toPut = append(toPut, t)
	}

	q.emitInjectStatusCount(injectStatusInjected, injected)
	q.emitInjectStatusCount(injectStatusDroppedBelow, droppedBelow)

	q.putTasks(toPut)
}

// putTasks inserts injected tasks, opens the window on the first fill,
// and caps size by dropping the OLDEST tasks (head eviction).
// Caller must hold q.mu.
func (q *cachedImmediateQueueReader) putTasks(tasks []persistence.Task) {
	if len(tasks) == 0 {
		return
	}

	q.queue.PutTasks(tasks)

	// On the first fill, move the lower bound up to the earliest injected key, and the upper bound to the maximum.
	if q.exclusiveUpperBound.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateInclusiveLowerBound(q.queue.LookAHead(persistence.MinimumHistoryTaskKey).GetTaskKey())
		q.updateExclusiveUpperBound(persistence.MaximumHistoryTaskKey)
	}

	newLower, trimmed := q.queue.LTrimBySize(q.options.MaxSize())
	if !trimmed {
		return
	}

	// edge-case: if the trim removed everything, the queue is now empty and the window should reset to the minimum
	if newLower.Equal(persistence.MinimumHistoryTaskKey) {
		q.updateExclusiveUpperBound(persistence.MinimumHistoryTaskKey)
	}
	q.updateInclusiveLowerBound(newLower)
}
