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

	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
)

type cachedQueue struct {
	Queue
	reader CachedQueueReader
}

// newCachedQueue wraps passed queue with reader.
// queueBase must be queue's underlying *queueBase.
func newCachedQueue(queue Queue, queueBase *queueBase, reader CachedQueueReader) Queue {
	// Wrap the queue state update to propagate min read level across all virtual queues
	// to CachedQueueReader each time the ack level is updated
	originalUpdateFn := queueBase.updateQueueStateFn
	queueBase.updateQueueStateFn = func(ctx context.Context) {
		originalUpdateFn(ctx)
		// MaximumHistoryTaskKey means no slices — fall back to ack level.
		readLevel := queueBase.virtualQueueManager.GetMinReadLevel()
		if readLevel.Equal(persistence.MaximumHistoryTaskKey) {
			readLevel = queueBase.exclusiveAckLevel
		}
		reader.UpdateReadLevel(readLevel)
	}
	return &cachedQueue{
		Queue:  queue,
		reader: reader,
	}
}

func (q *cachedQueue) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	if info.PersistenceError {
		q.reader.Clear()
	} else {
		q.reader.Inject(info.Tasks)
	}
	q.Queue.NotifyNewTask(clusterName, info)
}

func (q *cachedQueue) Start() {
	q.reader.Start()
	q.Queue.Start()
}

func (q *cachedQueue) Stop() {
	q.Queue.Stop()
	q.reader.Stop()
}
