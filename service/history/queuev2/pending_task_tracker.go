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

//go:generate mockgen -package $GOPACKAGE -destination pending_task_tracker_mock.go github.com/uber/cadence/service/history/queuev2 PendingTaskTracker
package queuev2

import (
	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/task"
)

type (
	// PendingTaskTracker tracks the pending tasks in a virtual slice.
	PendingTaskTracker interface {
		// AddTask adds a task to the pending task tracker.
		AddTask(task.Task)
		// PruneAckedTasks prunes the acked tasks from the pending task tracker.
		PruneAckedTasks()
		// GetMinimumTaskKey returns the minimum task key in the pending task tracker, if there are no pending tasks, it returns MaximumHistoryTaskKey.
		GetMinimumTaskKey() (persistence.HistoryTaskKey, bool)
		// GetTasks returns all the tasks in the pending task tracker, the result should be read-only.
		GetTasks() map[persistence.HistoryTaskKey]task.Task
		// GetPendingTaskCount returns the number of pending tasks in the pending task tracker.
		GetPendingTaskCount() int
		// GetPerDomainPendingTaskCount returns the number of pending tasks per domain.
		GetPerDomainPendingTaskCount() map[string]int
		// Clear clears the pending task tracker.
		Clear()
	}

	pendingTaskTrackerImpl struct {
		taskMap            map[persistence.HistoryTaskKey]task.Task
		taskCountPerDomain map[string]int // domainID -> task count
		minTaskKey         persistence.HistoryTaskKey
	}
)

func NewPendingTaskTracker() PendingTaskTracker {
	return &pendingTaskTrackerImpl{
		taskMap:            make(map[persistence.HistoryTaskKey]task.Task),
		taskCountPerDomain: make(map[string]int),
		minTaskKey:         persistence.MaximumHistoryTaskKey,
	}
}

func (t *pendingTaskTrackerImpl) AddTask(task task.Task) {
	if len(t.taskMap) == 0 {
		t.minTaskKey = task.GetTaskKey()
	} else if t.minTaskKey.Compare(task.GetTaskKey()) > 0 {
		t.minTaskKey = task.GetTaskKey()
	}

	t.taskMap[task.GetTaskKey()] = task
	t.taskCountPerDomain[task.GetDomainID()]++
}

func (t *pendingTaskTrackerImpl) GetMinimumTaskKey() (persistence.HistoryTaskKey, bool) {
	if len(t.taskMap) == 0 {
		return persistence.MaximumHistoryTaskKey, false
	}
	return t.minTaskKey, true
}

func (t *pendingTaskTrackerImpl) GetTasks() map[persistence.HistoryTaskKey]task.Task {
	return t.taskMap
}

func (t *pendingTaskTrackerImpl) PruneAckedTasks() {
	minTaskKey := persistence.MaximumHistoryTaskKey
	for key, task := range t.taskMap {
		if task.State() == ctask.TaskStateAcked {
			delete(t.taskMap, key)
			t.taskCountPerDomain[task.GetDomainID()]--
			continue
		}

		if key.Compare(minTaskKey) < 0 {
			minTaskKey = key
		}
	}
	t.minTaskKey = minTaskKey
}

func (t *pendingTaskTrackerImpl) GetPendingTaskCount() int {
	return len(t.taskMap)
}

func (t *pendingTaskTrackerImpl) GetPerDomainPendingTaskCount() map[string]int {
	return t.taskCountPerDomain
}

func (t *pendingTaskTrackerImpl) Clear() {
	for _, task := range t.taskMap {
		task.Cancel()
	}
	t.taskMap = make(map[persistence.HistoryTaskKey]task.Task)
	t.taskCountPerDomain = make(map[string]int)
	t.minTaskKey = persistence.MaximumHistoryTaskKey
}
