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
	"sort"

	"github.com/uber/cadence/common/persistence"
)

// InMemQueue is a sorted in-memory task store using persistence.HistoryTaskKey
// for ordering. Not internally synchronized; callers must handle locking.
type InMemQueue interface {
	// PutTasks inserts tasks into the queue in sorted order, skipping any task
	// whose key already exists.
	PutTasks(tasks []persistence.Task)
	// LookAHead returns the first task at or after minTaskKey, or nil if none exists.
	LookAHead(minTaskKey persistence.HistoryTaskKey) persistence.Task
	// GetTasks returns a page of tasks starting from the request's NextTaskKey,
	// up to PageSize tasks, bounded by the request's ExclusiveMaxTaskKey.
	GetTasks(req *GetTaskRequest) *GetTaskResponse
	// LTrim removes all tasks strictly before newInclusiveMinTaskKey.
	LTrim(newInclusiveMinTaskKey persistence.HistoryTaskKey)
	// RTrimBySize drops tasks from the tail until Len() <= maxSize.
	// Returns the key just past the last kept task and true if any tasks were
	// removed, or the zero key and false if the queue was already within bounds.
	RTrimBySize(maxSize int) (persistence.HistoryTaskKey, bool)
	// Clear removes all tasks.
	Clear()
	// Len returns the number of tasks currently in the queue.
	Len() int
}

type inMemQueueImpl struct {
	tasks []persistence.Task
}

func newInMemQueue() *inMemQueueImpl {
	return &inMemQueueImpl{}
}

// putTask inserts a task into the queue maintaining sorted order by task key.
// If a task with the same key already exists, it is skipped.
func (q *inMemQueueImpl) putTask(task persistence.Task) {
	key := task.GetTaskKey()

	// Fast path: append if task key is greater than the last element
	if len(q.tasks) > 0 {
		lastKey := q.tasks[len(q.tasks)-1].GetTaskKey()
		cmp := lastKey.Compare(key)
		if cmp == 0 {
			return // duplicate
		}
		if cmp < 0 {
			q.tasks = append(q.tasks, task)
			return
		}
	} else {
		q.tasks = append(q.tasks, task)
		return
	}

	// Slow path: binary search insert
	pos := sort.Search(len(q.tasks), func(i int) bool {
		return q.tasks[i].GetTaskKey().Compare(key) >= 0
	})

	if pos < len(q.tasks) && q.tasks[pos].GetTaskKey().Compare(key) == 0 {
		return // duplicate
	}

	q.tasks = append(q.tasks, nil)
	copy(q.tasks[pos+1:], q.tasks[pos:])
	q.tasks[pos] = task
}

// PutTasks inserts multiple tasks into the queue.
func (q *inMemQueueImpl) PutTasks(tasks []persistence.Task) {
	for _, task := range tasks {
		q.putTask(task)
	}
}

// LookAHead returns the first task at-or-after minTaskKey (>= semantics).
// Returns nil if no such task exists.
func (q *inMemQueueImpl) LookAHead(minTaskKey persistence.HistoryTaskKey) persistence.Task {
	pos := sort.Search(len(q.tasks), func(i int) bool {
		return q.tasks[i].GetTaskKey().Compare(minTaskKey) >= 0
	})

	if pos >= len(q.tasks) {
		return nil
	}
	return q.tasks[pos]
}

// GetTasks returns tasks in [req.Progress.NextTaskKey, req.Progress.ExclusiveMaxTaskKey)
// that match req.Predicate, truncated to req.PageSize. NextTaskKey in the response
// points to where the next page should start.
func (q *inMemQueueImpl) GetTasks(req *GetTaskRequest) *GetTaskResponse {
	inclusiveMinTaskKey := req.Progress.NextTaskKey
	exclusiveMaxTaskKey := req.Progress.ExclusiveMaxTaskKey

	// Guard against misconfigured callers — treat zero or negative page size as
	// no results so we never index into an empty slice below.
	if req.PageSize <= 0 {
		return &GetTaskResponse{
			Tasks: nil,
			Progress: &GetTaskProgress{
				Range:       req.Progress.Range,
				NextTaskKey: inclusiveMinTaskKey,
			},
		}
	}

	start := sort.Search(len(q.tasks), func(i int) bool {
		return q.tasks[i].GetTaskKey().Compare(inclusiveMinTaskKey) >= 0
	})

	var tasks []persistence.Task
	for i := start; i < len(q.tasks); i++ {
		if q.tasks[i].GetTaskKey().Compare(exclusiveMaxTaskKey) >= 0 {
			break
		}
		if req.Predicate.Check(q.tasks[i]) {
			tasks = append(tasks, q.tasks[i])
			if len(tasks) > req.PageSize {
				// Collected one extra to detect that a next page exists;
				// stop scanning to avoid unnecessary allocations.
				break
			}
		}
	}

	var nextKey persistence.HistoryTaskKey
	if len(tasks) > req.PageSize {
		tasks = tasks[:req.PageSize]
		nextKey = tasks[len(tasks)-1].GetTaskKey().Next()
	} else {
		nextKey = exclusiveMaxTaskKey
	}
	return &GetTaskResponse{
		Tasks: tasks,
		Progress: &GetTaskProgress{
			Range:       req.Progress.Range,
			NextTaskKey: nextKey,
		},
	}
}

// LTrim removes all tasks before newInclusiveMinTaskKey.
// The task at newInclusiveMinTaskKey is retained.
func (q *inMemQueueImpl) LTrim(newInclusiveMinTaskKey persistence.HistoryTaskKey) {
	pos := sort.Search(len(q.tasks), func(i int) bool {
		return q.tasks[i].GetTaskKey().Compare(newInclusiveMinTaskKey) >= 0
	})

	if pos > 0 {
		remaining := make([]persistence.Task, len(q.tasks)-pos)
		copy(remaining, q.tasks[pos:])
		q.tasks = remaining
	}
}

// RTrimBySize truncates the queue to at most maxSize elements by dropping
// the newest tasks from the right. Returns (newUpperBound, trimmed) where
// newUpperBound is lastTask.GetTaskKey().Next() and trimmed indicates whether
// any tasks were actually removed. Returns (MinimumHistoryTaskKey, false) if
// the queue is empty after the operation, or (MinimumHistoryTaskKey, hadTasks)
// if maxSize <= 0.
func (q *inMemQueueImpl) RTrimBySize(maxSize int) (persistence.HistoryTaskKey, bool) {
	if maxSize <= 0 {
		hadTasks := len(q.tasks) > 0
		q.tasks = nil
		return persistence.MinimumHistoryTaskKey, hadTasks
	}

	trimmed := len(q.tasks) > maxSize
	if trimmed {
		// Nil out the tail elements so the backing array releases references to
		// the removed Task interface values. Without this, interface values beyond
		// maxSize remain rooted in the array until it is reallocated.
		for i := maxSize; i < len(q.tasks); i++ {
			q.tasks[i] = nil
		}
		q.tasks = q.tasks[:maxSize]
	}

	if len(q.tasks) == 0 {
		return persistence.MinimumHistoryTaskKey, false
	}

	return q.tasks[len(q.tasks)-1].GetTaskKey().Next(), trimmed
}

// Clear removes all tasks from the queue.
func (q *inMemQueueImpl) Clear() {
	q.tasks = nil
}

// Len returns the number of tasks in the queue.
func (q *inMemQueueImpl) Len() int {
	return len(q.tasks)
}
