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

	hcommon "github.com/uber/cadence/service/history/common"
)

type cachedScheduledQueue struct {
	*scheduledQueue
	reader CachedQueueReader
}

func newCachedScheduledQueue(inner *scheduledQueue, reader CachedQueueReader) Queue {
	// Wrap the queue state update to propagate min read level across all virtual queues
	// to CachedQueueReader each time the ack level is updated
	originalUpdateFn := inner.base.updateQueueStateFn
	inner.base.updateQueueStateFn = func(ctx context.Context) {
		originalUpdateFn(ctx)
		reader.UpdateReadLevel(inner.base.virtualQueueManager.GetMinReadLevel())
	}

	return &cachedScheduledQueue{
		scheduledQueue: inner,
		reader:         reader,
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
}

func (q *cachedScheduledQueue) Stop() {
	q.scheduledQueue.Stop()
	q.reader.Stop()
}
