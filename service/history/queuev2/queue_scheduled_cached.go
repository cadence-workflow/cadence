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
	"sync"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	hcommon "github.com/uber/cadence/service/history/common"
)

type cachedScheduledQueue struct {
	*scheduledQueue
	reader              CachedQueueReader
	readLevelSyncCancel context.CancelFunc
	readLevelSyncWg     sync.WaitGroup
	readLevelInterval   dynamicproperties.DurationPropertyFn
	clock               clock.TimeSource
}

func newCachedScheduledQueue(inner *scheduledQueue, reader CachedQueueReader) Queue {
	// Wrap the queue state update so the reader evicts tasks below the current
	// ack level whenever the queue persists its progress. GetMinReadLevel returns
	// MaximumHistoryTaskKey when no virtual queues exist yet; UpdateReadLevel
	// normalises that sentinel so it is treated as a no-op advance.
	originalUpdateFn := inner.base.updateQueueStateFn
	inner.base.updateQueueStateFn = func(ctx context.Context) {
		originalUpdateFn(ctx)
		reader.UpdateReadLevel(inner.base.virtualQueueManager.GetMinReadLevel())
	}

	config := inner.base.shard.GetConfig()
	return &cachedScheduledQueue{
		scheduledQueue:    inner,
		reader:            reader,
		readLevelInterval: config.TimerProcessorCacheReadLevelSyncInterval,
		clock:             inner.base.timeSource,
	}
}

func (q *cachedScheduledQueue) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	if info.PersistenceError {
		q.reader.Clear()
	} else {
		q.reader.Inject(info.Tasks)
	}
	q.scheduledQueue.NotifyNewTask(clusterName, info)
}

func (q *cachedScheduledQueue) Start() {
	q.reader.Start()
	q.scheduledQueue.Start()

	ctx, cancel := context.WithCancel(context.Background())
	q.readLevelSyncCancel = cancel
	q.readLevelSyncWg.Add(1)
	go q.readLevelSyncLoop(ctx)
}

func (q *cachedScheduledQueue) Stop() {
	q.readLevelSyncCancel()
	q.readLevelSyncWg.Wait()

	q.scheduledQueue.Stop()
	q.reader.Stop()
}

// readLevelSyncLoop periodically syncs the virtual queue manager's current read
// level to the cache reader, evicting tasks the processor has already passed.
// This runs more frequently than updateQueueStateFn (which is gated on DB writes)
// so the cache lower bound stays close to actual processing progress.
func (q *cachedScheduledQueue) readLevelSyncLoop(ctx context.Context) {
	defer q.readLevelSyncWg.Done()

	timer := q.clock.NewTimer(q.readLevelInterval())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.Chan():
			q.reader.UpdateReadLevel(q.base.virtualQueueManager.GetMinReadLevel())
			timer.Reset(q.readLevelInterval())
		}
	}
}
