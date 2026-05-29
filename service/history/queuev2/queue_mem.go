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

//go:generate mockgen -package $GOPACKAGE -destination queue_mem_mock.go github.com/uber/cadence/service/history/queuev2 InMemQueue

// InMemQueue is a sorted in-memory task store using persistence.HistoryTaskKey
// for ordering. Not internally synchronized; callers must handle locking.
type InMemQueue interface {
	// PutTasks inserts tasks into the queue in sorted order, skipping any task
	// whose key already exists.
	PutTasks(tasks []persistence.Task)
	// LookAHead returns the first task at or after minTaskKey, or nil if none exists.
	LookAHead(minTaskKey persistence.HistoryTaskKey) persistence.Task
	// GetTasks returns up to pageSize tasks in [inclusiveMinTaskKey, exclusiveMaxTaskKey)
	// that match the predicate. nextTaskKey points to where the next page should start.
	GetTasks(inclusiveMinTaskKey, exclusiveMaxTaskKey persistence.HistoryTaskKey, predicate Predicate, pageSize int) (tasks []persistence.Task, nextTaskKey persistence.HistoryTaskKey)
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
	n := len(q.tasks)

	// Fast path: empty queue or key is strictly greater than the tail — append directly.
	if n == 0 || q.tasks[n-1].GetTaskKey().Compare(key) < 0 {
		q.tasks = append(q.tasks, task)
		return
	}

	// Slow path: binary search to find the insertion point.
	pos := q.findTaskPosition(key)
	if pos < n && q.tasks[pos].GetTaskKey().Compare(key) == 0 {
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
	pos := q.findTaskPosition(minTaskKey)

	if pos >= len(q.tasks) {
		return nil
	}
	return q.tasks[pos]
}

// findTaskPosition returns the index of the first task with key >= taskKey, or len(q.tasks) if no such task exists.
func (q *inMemQueueImpl) findTaskPosition(taskKey persistence.HistoryTaskKey) int {
	return sort.Search(len(q.tasks), func(i int) bool {
		return q.tasks[i].GetTaskKey().Compare(taskKey) >= 0
	})
}

// GetTasks returns up to pageSize tasks in [inclusiveMinTaskKey, exclusiveMaxTaskKey)
// that match the predicate. nextTaskKey points to where the next page should start.
func (q *inMemQueueImpl) GetTasks(inclusiveMinTaskKey, exclusiveMaxTaskKey persistence.HistoryTaskKey, predicate Predicate, pageSize int) ([]persistence.Task, persistence.HistoryTaskKey) {
	// Guard against misconfigured callers — treat zero or negative page size as
	// no results so we never index into an empty slice below.
	if pageSize <= 0 {
		return nil, inclusiveMinTaskKey
	}

	start := q.findTaskPosition(inclusiveMinTaskKey)

	var tasks []persistence.Task
	for i := start; i < len(q.tasks); i++ {
		if q.tasks[i].GetTaskKey().Compare(exclusiveMaxTaskKey) >= 0 {
			break
		}

		// Filter out tasks that don't match the predicate. We do this after checking the max key
		if !predicate.Check(q.tasks[i]) {
			continue
		}

		tasks = append(tasks, q.tasks[i])
		if len(tasks) > pageSize {
			// Trim the extra task so we return at most pageSize tasks
			// and set nextTaskKey to the key of the first excluded task
			return tasks[:pageSize], tasks[pageSize].GetTaskKey()
		}
	}

	return tasks, exclusiveMaxTaskKey
}

// LTrim removes all tasks before newInclusiveMinTaskKey.
// The task at newInclusiveMinTaskKey is retained.
func (q *inMemQueueImpl) LTrim(newInclusiveMinTaskKey persistence.HistoryTaskKey) {
	pos := q.findTaskPosition(newInclusiveMinTaskKey)

	if pos <= 0 {
		return
	}

	remaining := make([]persistence.Task, len(q.tasks)-pos)
	copy(remaining, q.tasks[pos:])
	q.tasks = remaining
}

// RTrimBySize truncates the queue to at most maxSize elements by dropping
// the newest tasks from the right.
// Returns the next key after the last task or MinimumHistoryTaskKey if the queue is empty, and
// true if any tasks were removed, or false if the queue was already within bounds.
func (q *inMemQueueImpl) RTrimBySize(maxSize int) (persistence.HistoryTaskKey, bool) {
	if len(q.tasks) == 0 {
		return persistence.MinimumHistoryTaskKey, false
	}

	if maxSize <= 0 {
		q.tasks = nil
		return persistence.MinimumHistoryTaskKey, true
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
