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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
)

// newTestBudgetMgr creates a budget manager for tests.
// maxCount < 0 means unlimited; >= 0 is an enforced cap.
func newTestBudgetMgr(maxCount int64) cache.Manager {
	return cache.NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(-1), // bytes: unlimited
		dynamicproperties.GetIntPropertyFn(int(maxCount)),
		cache.AdmissionOptimistic,
		0, nil, nil, nil,
	)
}

// mutableCountFn wraps an atomic so tests can change the capacity after construction.
type mutableCountFn struct{ v atomic.Int64 }

func newMutableCountFn(initial int64) *mutableCountFn {
	f := &mutableCountFn{}
	f.v.Store(initial)
	return f
}

func (m *mutableCountFn) fn(_ ...dynamicproperties.FilterOption) int {
	return int(m.v.Load())
}

// newTestBudgetMgrMutable creates a budget manager whose count capacity can be changed at runtime.
func newTestBudgetMgrMutable(cap *mutableCountFn) cache.Manager {
	return cache.NewBudgetManager(
		"test",
		dynamicproperties.GetIntPropertyFn(-1), // bytes: unlimited
		cap.fn,
		cache.AdmissionOptimistic,
		0, nil, nil, nil,
	)
}

// testTask is a minimal persistence.Task for testing InMemQueue.
// It uses DeleteHistoryEventTask because it's a timer task that uses
// NewHistoryTaskKey(VisibilityTimestamp, TaskID) for its task key.
func newTestTask(scheduledTime time.Time, taskID int64) persistence.Task {
	return &persistence.DeleteHistoryEventTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   "test-domain",
			WorkflowID: "test-workflow",
			RunID:      "test-run",
		},
		TaskData: persistence.TaskData{
			TaskID:              taskID,
			VisibilityTimestamp: scheduledTime,
		},
	}
}

func TestInMemQueue_putTask(t *testing.T) {
	tests := []struct {
		name        string
		existing    []persistence.Task
		insert      persistence.Task
		wantLen     int
		wantTaskIDs []int64
	}{
		{
			name:        "insert into empty queue",
			existing:    nil,
			insert:      newTestTask(time.Unix(10, 0), 1),
			wantLen:     1,
			wantTaskIDs: []int64{1},
		},
		{
			name: "insert maintains sorted order - earlier task after later one",
			existing: []persistence.Task{
				newTestTask(time.Unix(20, 0), 2),
			},
			insert:      newTestTask(time.Unix(10, 0), 1),
			wantLen:     2,
			wantTaskIDs: []int64{1, 2},
		},
		{
			name: "duplicate key is skipped",
			existing: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
			},
			insert:      newTestTask(time.Unix(10, 0), 1),
			wantLen:     1,
			wantTaskIDs: []int64{1},
		},
		{
			name: "insert at beginning",
			existing: []persistence.Task{
				newTestTask(time.Unix(20, 0), 2),
				newTestTask(time.Unix(30, 0), 3),
			},
			insert:      newTestTask(time.Unix(10, 0), 1),
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
		{
			name: "insert at middle",
			existing: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(30, 0), 3),
			},
			insert:      newTestTask(time.Unix(20, 0), 2),
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
		{
			name: "insert at end",
			existing: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			insert:      newTestTask(time.Unix(30, 0), 3),
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
		{
			name: "same time different task IDs - sorted by task ID",
			existing: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(10, 0), 3),
			},
			insert:      newTestTask(time.Unix(10, 0), 2),
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestBudgetMgr(-1)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			for _, task := range tt.existing {
				q.putTask(task)
			}

			q.putTask(tt.insert)

			assert.Equal(t, tt.wantLen, q.Len())
			for i, wantID := range tt.wantTaskIDs {
				assert.Equal(t, wantID, q.tasks[i].GetTaskID(), "task at index %d", i)
			}
		})
	}
}

func TestInMemQueue_PutTasks(t *testing.T) {
	tests := []struct {
		name        string
		existing    []persistence.Task
		insert      []persistence.Task
		wantLen     int
		wantTaskIDs []int64
	}{
		{
			name:     "insert multiple tasks into empty queue",
			existing: nil,
			insert: []persistence.Task{
				newTestTask(time.Unix(30, 0), 3),
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
		{
			name: "deduplication - tasks with existing keys are skipped",
			existing: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(30, 0), 3),
			},
			insert: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1), // duplicate
				newTestTask(time.Unix(20, 0), 2), // new
				newTestTask(time.Unix(30, 0), 3), // duplicate
			},
			wantLen:     3,
			wantTaskIDs: []int64{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestBudgetMgr(-1)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			for _, task := range tt.existing {
				q.putTask(task)
			}

			q.PutTasks(tt.insert)

			assert.Equal(t, tt.wantLen, q.Len())
			for i, wantID := range tt.wantTaskIDs {
				assert.Equal(t, wantID, q.tasks[i].GetTaskID(), "task at index %d", i)
			}
		})
	}
}

func TestInMemQueue_LookAHead(t *testing.T) {
	tests := []struct {
		name       string
		tasks      []persistence.Task
		minTaskKey persistence.HistoryTaskKey
		wantTaskID int64 // 0 means no task expected (nil)
	}{
		{
			name:       "empty queue returns nil",
			tasks:      nil,
			minTaskKey: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
		},
		{
			name: "key matches existing task",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			minTaskKey: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			wantTaskID: 1,
		},
		{
			name: "key between tasks returns first task at-or-after key",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(30, 0), 3),
			},
			minTaskKey: persistence.NewHistoryTaskKey(time.Unix(20, 0), 2),
			wantTaskID: 3,
		},
		{
			name: "key after all tasks returns nil",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			minTaskKey: persistence.NewHistoryTaskKey(time.Unix(30, 0), 3),
		},
		{
			name: "key before all tasks returns first task",
			tasks: []persistence.Task{
				newTestTask(time.Unix(20, 0), 2),
				newTestTask(time.Unix(30, 0), 3),
			},
			minTaskKey: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			wantTaskID: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestBudgetMgr(-1)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			q.PutTasks(tt.tasks)

			task := q.LookAHead(tt.minTaskKey)

			if tt.wantTaskID == 0 {
				assert.Nil(t, task)
			} else {
				require.NotNil(t, task)
				assert.Equal(t, tt.wantTaskID, task.GetTaskID())
			}
		})
	}
}

func TestInMemQueue_GetTasks(t *testing.T) {
	allTasks := []persistence.Task{
		newTestTask(time.Unix(10, 0), 1),
		newTestTask(time.Unix(20, 0), 2),
		newTestTask(time.Unix(30, 0), 3),
		newTestTask(time.Unix(40, 0), 4),
		newTestTask(time.Unix(50, 0), 5),
	}

	tests := []struct {
		name         string
		tasks        []persistence.Task
		inclusiveMin persistence.HistoryTaskKey
		exclusiveMax persistence.HistoryTaskKey
		predicate    Predicate
		pageSize     int
		wantTaskIDs  []int64
		wantNextKey  persistence.HistoryTaskKey
	}{
		{
			name:         "range matches multiple tasks",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(20, 0), 2),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(40, 0), 4),
			predicate:    NewUniversalPredicate(),
			pageSize:     100,
			wantTaskIDs:  []int64{2, 3},
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(40, 0), 4),
		},
		{
			name:         "predicate filters out some tasks",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
			predicate:    &taskIDFilterPredicate{allowedIDs: map[int64]struct{}{1: {}, 3: {}}},
			pageSize:     100,
			wantTaskIDs:  []int64{1, 3},
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
		},
		{
			name:         "empty range returns empty slice",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(60, 0), 6),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(70, 0), 7),
			predicate:    NewUniversalPredicate(),
			pageSize:     100,
			wantTaskIDs:  nil,
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(70, 0), 7),
		},
		{
			name:         "boundary - inclusiveMin is included, exclusiveMax is excluded",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(20, 0), 2),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(30, 0), 3),
			predicate:    NewUniversalPredicate(),
			pageSize:     100,
			wantTaskIDs:  []int64{2},
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(30, 0), 3),
		},
		{
			name:         "no tasks match predicate",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
			predicate:    NewEmptyPredicate(),
			pageSize:     100,
			wantTaskIDs:  nil,
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
		},
		{
			name:         "empty queue",
			tasks:        nil,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
			predicate:    NewUniversalPredicate(),
			pageSize:     100,
			wantTaskIDs:  nil,
			wantNextKey:  persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
		},
		{
			name:         "pageSize truncates result and returns next task key after last returned task",
			tasks:        allTasks,
			inclusiveMin: persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			exclusiveMax: persistence.NewHistoryTaskKey(time.Unix(50, 0), 5),
			predicate:    NewUniversalPredicate(),
			pageSize:     2,
			wantTaskIDs:  []int64{1, 2},
			wantNextKey:  newTestTask(time.Unix(20, 0), 2).GetTaskKey().Next(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestBudgetMgr(-1)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			q.PutTasks(tt.tasks)

			gotTasks, nextKey := q.GetTasks(tt.inclusiveMin, tt.exclusiveMax, tt.predicate, tt.pageSize)

			var gotIDs []int64
			for _, task := range gotTasks {
				gotIDs = append(gotIDs, task.GetTaskID())
			}
			assert.Equal(t, tt.wantTaskIDs, gotIDs)
			assert.Equal(t, tt.wantNextKey, nextKey)
		})
	}
}

func TestInMemQueue_LTrim(t *testing.T) {
	tests := []struct {
		name        string
		tasks       []persistence.Task
		trimKey     persistence.HistoryTaskKey
		wantLen     int
		wantTaskIDs []int64
	}{
		{
			name: "trim removes tasks before key",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
				newTestTask(time.Unix(30, 0), 3),
			},
			trimKey:     persistence.NewHistoryTaskKey(time.Unix(20, 0), 2),
			wantLen:     2,
			wantTaskIDs: []int64{2, 3},
		},
		{
			name: "task at key is retained",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			trimKey:     persistence.NewHistoryTaskKey(time.Unix(20, 0), 2),
			wantLen:     1,
			wantTaskIDs: []int64{2},
		},
		{
			name: "trim with key before first task is no-op",
			tasks: []persistence.Task{
				newTestTask(time.Unix(20, 0), 2),
				newTestTask(time.Unix(30, 0), 3),
			},
			trimKey:     persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			wantLen:     2,
			wantTaskIDs: []int64{2, 3},
		},
		{
			name: "trim with key after all tasks empties queue",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			trimKey:     persistence.NewHistoryTaskKey(time.Unix(30, 0), 3),
			wantLen:     0,
			wantTaskIDs: nil,
		},
		{
			name:        "trim on empty queue is no-op",
			tasks:       nil,
			trimKey:     persistence.NewHistoryTaskKey(time.Unix(10, 0), 1),
			wantLen:     0,
			wantTaskIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestBudgetMgr(-1)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			q.PutTasks(tt.tasks)

			q.LTrim(tt.trimKey)

			assert.Equal(t, tt.wantLen, q.Len())
			var gotIDs []int64
			for _, task := range q.tasks {
				gotIDs = append(gotIDs, task.GetTaskID())
			}
			assert.Equal(t, tt.wantTaskIDs, gotIDs)
		})
	}
}

func TestInMemQueue_RTrimBySize(t *testing.T) {
	tests := []struct {
		name        string
		tasks       []persistence.Task
		maxSize     int64
		wantLen     int
		wantTaskIDs []int64
		wantKey     persistence.HistoryTaskKey
		wantTrimmed bool
	}{
		{
			name: "queue larger than maxSize - truncated",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
				newTestTask(time.Unix(30, 0), 3),
				newTestTask(time.Unix(40, 0), 4),
			},
			maxSize:     2,
			wantLen:     2,
			wantTaskIDs: []int64{1, 2},
			wantKey:     newTestTask(time.Unix(20, 0), 2).GetTaskKey().Next(),
			wantTrimmed: true,
		},
		{
			name: "queue smaller than maxSize - no-op",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			maxSize:     5,
			wantLen:     2,
			wantTaskIDs: []int64{1, 2},
			wantKey:     newTestTask(time.Unix(20, 0), 2).GetTaskKey().Next(),
			wantTrimmed: false,
		},
		{
			name: "queue exactly maxSize - no-op",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			maxSize:     2,
			wantLen:     2,
			wantTaskIDs: []int64{1, 2},
			wantKey:     newTestTask(time.Unix(20, 0), 2).GetTaskKey().Next(),
			wantTrimmed: false,
		},
		{
			name:        "empty queue returns MinimumHistoryTaskKey",
			tasks:       nil,
			maxSize:     5,
			wantLen:     0,
			wantTaskIDs: nil,
			wantKey:     persistence.MinimumHistoryTaskKey,
			wantTrimmed: false,
		},
		{
			name: "maxSize 0 empties queue",
			tasks: []persistence.Task{
				newTestTask(time.Unix(10, 0), 1),
				newTestTask(time.Unix(20, 0), 2),
			},
			maxSize:     0,
			wantLen:     0,
			wantTaskIDs: nil,
			wantKey:     persistence.MinimumHistoryTaskKey,
			wantTrimmed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert tasks with unlimited capacity so all fit.
			cap := newMutableCountFn(-1)
			mgr := newTestBudgetMgrMutable(cap)
			defer mgr.Stop()
			q := newInMemQueueWithBudget(mgr, "test")
			q.PutTasks(tt.tasks)

			// Now reduce capacity to maxSize to trigger trimming on next RTrimBySize call.
			cap.v.Store(tt.maxSize)

			gotKey, gotTrimmed := q.RTrimBySize()

			assert.Equal(t, tt.wantLen, q.Len())
			var gotIDs []int64
			for _, task := range q.tasks {
				gotIDs = append(gotIDs, task.GetTaskID())
			}
			assert.Equal(t, tt.wantTaskIDs, gotIDs)
			assert.Equal(t, 0, tt.wantKey.Compare(gotKey), "expected key %v, got %v", tt.wantKey, gotKey)
			assert.Equal(t, tt.wantTrimmed, gotTrimmed, "trimmed flag")
		})
	}
}

func TestInMemQueue_Len(t *testing.T) {
	mgr := newTestBudgetMgr(-1)
	defer mgr.Stop()
	q := newInMemQueueWithBudget(mgr, "test")
	assert.Equal(t, 0, q.Len())

	q.putTask(newTestTask(time.Unix(10, 0), 1))
	q.putTask(newTestTask(time.Unix(20, 0), 2))
	assert.Equal(t, 2, q.Len())

	q.LTrim(persistence.NewHistoryTaskKey(time.Unix(20, 0), 2))
	assert.Equal(t, 1, q.Len())
}

func TestInMemQueue_Clear(t *testing.T) {
	mgr := newTestBudgetMgr(-1)
	defer mgr.Stop()
	q := newInMemQueueWithBudget(mgr, "test")
	q.PutTasks([]persistence.Task{
		newTestTask(time.Unix(10, 0), 1),
		newTestTask(time.Unix(20, 0), 2),
	})
	assert.Equal(t, 2, q.Len())

	q.Clear()
	assert.Equal(t, 0, q.Len())
}

// TestInMemQueue_RTrimBySize_NilsTail verifies that RTrimBySize nils out the
// backing array elements beyond maxSize so removed Task interface values are
// eligible for GC and don't linger in the underlying array.
func TestInMemQueue_RTrimBySize_NilsTail(t *testing.T) {
	// Insert 3 tasks with unlimited capacity, then shrink capacity to 1.
	capFn := newMutableCountFn(-1)
	mgr := newTestBudgetMgrMutable(capFn)
	defer mgr.Stop()
	q := newInMemQueueWithBudget(mgr, "test")
	q.PutTasks([]persistence.Task{
		newTestTask(time.Unix(1, 0), 1),
		newTestTask(time.Unix(2, 0), 2),
		newTestTask(time.Unix(3, 0), 3),
	})
	// Grow the backing array so we can inspect it after trimming.
	cap0 := cap(q.tasks)

	// Reduce capacity to 1 so RTrimBySize trims.
	capFn.v.Store(1)
	_, trimmed := q.RTrimBySize()
	assert.True(t, trimmed)
	assert.Equal(t, 1, q.Len())

	// The backing array must be unchanged in capacity...
	assert.Equal(t, cap0, cap(q.tasks))
	// ...but slots beyond the new length must be nil so the GC can collect them.
	full := q.tasks[:cap0]
	for i := 1; i < cap0; i++ {
		assert.Nil(t, full[i], "backing array slot %d should be nil after RTrimBySize", i)
	}
}

func TestInMemQueue_PutTasks_AppendFastPath(t *testing.T) {
	mgr := newTestBudgetMgr(-1)
	defer mgr.Stop()
	q := newInMemQueueWithBudget(mgr, "test")
	// Insert tasks in ascending order — all should use fast path.
	tasks := []persistence.Task{
		newTestTask(time.Unix(10, 0), 1),
		newTestTask(time.Unix(20, 0), 2),
		newTestTask(time.Unix(30, 0), 3),
	}
	q.PutTasks(tasks)

	assert.Equal(t, 3, q.Len())
	assert.Equal(t, int64(1), q.tasks[0].GetTaskID())
	assert.Equal(t, int64(2), q.tasks[1].GetTaskID())
	assert.Equal(t, int64(3), q.tasks[2].GetTaskID())
}

// TestInMemQueueBudget covers budget-specific behaviors of inMemQueueImpl.
func TestInMemQueueBudget(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "PutTasks reserves count for inserted tasks",
			run: func(t *testing.T) {
				mgr := newTestBudgetMgr(10)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
				})

				assert.Equal(t, int64(3), mgr.UsedCount())
				assert.Equal(t, 3, q.Len())
			},
		},
		{
			name: "PutTasks does NOT reserve for duplicate tasks",
			run: func(t *testing.T) {
				mgr := newTestBudgetMgr(10)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				tasks := []persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
				}
				q.PutTasks(tasks)
				// Insert same 3 tasks again — duplicates should be skipped, count unchanged.
				q.PutTasks(tasks)

				assert.Equal(t, int64(3), mgr.UsedCount(), "duplicates must not increment used count")
			},
		},
		{
			name: "RTrimBySize trims when over capacity",
			run: func(t *testing.T) {
				// Insert 5 tasks with unlimited capacity, then shrink to 3 to trigger trim.
				capFn := newMutableCountFn(-1)
				mgr := newTestBudgetMgrMutable(capFn)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
					newTestTask(time.Unix(40, 0), 4),
					newTestTask(time.Unix(50, 0), 5),
				})

				capFn.v.Store(3) // reduce capacity to trigger trimming
				_, trimmed := q.RTrimBySize()

				assert.True(t, trimmed)
				assert.Equal(t, 3, q.Len())
				assert.Equal(t, int64(3), mgr.UsedCount())
			},
		},
		{
			name: "RTrimBySize is no-op when within capacity",
			run: func(t *testing.T) {
				mgr := newTestBudgetMgr(10)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
				})

				_, trimmed := q.RTrimBySize()

				assert.False(t, trimmed)
				assert.Equal(t, 3, q.Len())
				assert.Equal(t, int64(3), mgr.UsedCount())
			},
		},
		{
			name: "LTrim releases count for removed tasks",
			run: func(t *testing.T) {
				mgr := newTestBudgetMgr(10)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
					newTestTask(time.Unix(40, 0), 4),
					newTestTask(time.Unix(50, 0), 5),
				})
				// LTrim removes tasks strictly before key(40,4) — removes tasks 1,2,3.
				q.LTrim(persistence.NewHistoryTaskKey(time.Unix(40, 0), 4))

				assert.Equal(t, 2, q.Len())
				assert.Equal(t, int64(2), mgr.UsedCount())
			},
		},
		{
			name: "Clear releases all count",
			run: func(t *testing.T) {
				mgr := newTestBudgetMgr(10)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
					newTestTask(time.Unix(40, 0), 4),
				})

				q.Clear()

				assert.Equal(t, 0, q.Len())
				assert.Equal(t, int64(0), mgr.UsedCount())
			},
		},
		{
			name: "RTrimBySize trims untracked task inserted when at capacity",
			run: func(t *testing.T) {
				// Capacity=3. Insert 3 tasks (all succeed). Then insert a 4th task
				// whose ReserveCountWithCallback fails (budget at capacity with
				// AdmissionOptimistic: increments, detects overage, rolls back).
				// The 4th task is still appended to q.tasks (by design — tasks are
				// never silently skipped). RTrimBySize must detect Len()>capacity and
				// trim the untracked tail task.
				mgr := newTestBudgetMgr(3)
				defer mgr.Stop()
				q := newInMemQueueWithBudget(mgr, "test")

				q.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
				})
				// 4th insert: reserve will fail (capacity=3, used=3).
				q.putTask(newTestTask(time.Unix(40, 0), 4))
				// Queue has 4 tasks but UsedCount is still 3.
				require.Equal(t, 4, q.Len(), "pre-condition: 4th task was inserted despite reserve failure")

				_, trimmed := q.RTrimBySize()

				assert.True(t, trimmed)
				assert.Equal(t, 3, q.Len())
			},
		},
		{
			name: "Two queues sharing budget: RTrimBySize trims local queue to capacity bound",
			run: func(t *testing.T) {
				// RTrimBySize trims the local queue down to CapacityCount entries.
				// It cannot account for tasks held in sibling queues — that is the
				// caller's responsibility (trim each queue individually).
				// Here q1 holds 3 tasks, q2 holds 5 tasks, capacity=3.
				// q2.RTrimBySize() trims q2 from 5→3 (the local len bound).
				capFn := newMutableCountFn(-1)
				mgr := newTestBudgetMgrMutable(capFn)
				defer mgr.Stop()

				q1 := newInMemQueueWithBudget(mgr, "q1")
				q2 := newInMemQueueWithBudget(mgr, "q2")

				q1.PutTasks([]persistence.Task{
					newTestTask(time.Unix(10, 0), 1),
					newTestTask(time.Unix(20, 0), 2),
					newTestTask(time.Unix(30, 0), 3),
				})
				q2.PutTasks([]persistence.Task{
					newTestTask(time.Unix(40, 0), 4),
					newTestTask(time.Unix(50, 0), 5),
					newTestTask(time.Unix(60, 0), 6),
					newTestTask(time.Unix(70, 0), 7),
					newTestTask(time.Unix(80, 0), 8),
				})

				// Shrink shared capacity to 3 (total 8 > 3).
				capFn.v.Store(3)
				_, trimmed := q2.RTrimBySize()

				assert.True(t, trimmed)
				assert.Equal(t, 3, q1.Len(), "q1 must be untouched")
				assert.Equal(t, 3, q2.Len(), "q2 must be trimmed to CapacityCount=3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// taskIDFilterPredicate is a test helper that only allows specific task IDs.
type taskIDFilterPredicate struct {
	allowedIDs map[int64]struct{}
}

func (p *taskIDFilterPredicate) IsEmpty() bool {
	return len(p.allowedIDs) == 0
}

func (p *taskIDFilterPredicate) Check(task persistence.Task) bool {
	_, ok := p.allowedIDs[task.GetTaskID()]
	return ok
}

func (p *taskIDFilterPredicate) Equals(other Predicate) bool {
	return false
}
