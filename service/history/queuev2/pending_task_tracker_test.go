package queuev2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/task"
)

func TestPendingTaskTracker(t *testing.T) {
	testTime := time.Unix(0, 0)
	tests := []struct {
		name          string
		setupTasks    func(ctrl *gomock.Controller) []*task.MockTask
		pruneAcked    bool
		clear         bool
		wantMinKey    persistence.HistoryTaskKey
		wantHasMinKey bool
		wantTaskCount int
	}{
		{
			name:          "empty tracker",
			setupTasks:    func(ctrl *gomock.Controller) []*task.MockTask { return []*task.MockTask{} },
			wantMinKey:    persistence.MaximumHistoryTaskKey,
			wantHasMinKey: false,
			wantTaskCount: 0,
		},
		{
			name: "single task",
			setupTasks: func(ctrl *gomock.Controller) []*task.MockTask {
				mockTask := task.NewMockTask(ctrl)
				mockTask.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 1)).AnyTimes()
				mockTask.EXPECT().State().Return(ctask.TaskStatePending).AnyTimes()
				return []*task.MockTask{mockTask}
			},
			wantMinKey:    persistence.NewHistoryTaskKey(testTime, 1),
			wantHasMinKey: true,
			wantTaskCount: 1,
		},
		{
			name: "multiple tasks",
			setupTasks: func(ctrl *gomock.Controller) []*task.MockTask {
				task1 := task.NewMockTask(ctrl)
				task1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 3)).AnyTimes()
				task1.EXPECT().State().Return(ctask.TaskStatePending).AnyTimes()

				task2 := task.NewMockTask(ctrl)
				task2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 1)).AnyTimes()
				task2.EXPECT().State().Return(ctask.TaskStatePending).AnyTimes()

				task3 := task.NewMockTask(ctrl)
				task3.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 2)).AnyTimes()
				task3.EXPECT().State().Return(ctask.TaskStatePending).AnyTimes()

				return []*task.MockTask{task1, task2, task3}
			},
			wantMinKey:    persistence.NewHistoryTaskKey(testTime, 1),
			wantHasMinKey: true,
			wantTaskCount: 3,
		},
		{
			name: "prune acked tasks",
			setupTasks: func(ctrl *gomock.Controller) []*task.MockTask {
				task1 := task.NewMockTask(ctrl)
				task1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 1)).AnyTimes()
				task1.EXPECT().State().Return(ctask.TaskStateAcked).AnyTimes()

				task2 := task.NewMockTask(ctrl)
				task2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 2)).AnyTimes()
				task2.EXPECT().State().Return(ctask.TaskStatePending).AnyTimes()

				task3 := task.NewMockTask(ctrl)
				task3.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 3)).AnyTimes()
				task3.EXPECT().State().Return(ctask.TaskStateAcked).AnyTimes()

				return []*task.MockTask{task1, task2, task3}
			},
			pruneAcked:    true,
			wantMinKey:    persistence.NewHistoryTaskKey(testTime, 2),
			wantHasMinKey: true,
			wantTaskCount: 1,
		},
		{
			name: "all tasks acked",
			setupTasks: func(ctrl *gomock.Controller) []*task.MockTask {
				task1 := task.NewMockTask(ctrl)
				task1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 1)).AnyTimes()
				task1.EXPECT().State().Return(ctask.TaskStateAcked).AnyTimes()

				task2 := task.NewMockTask(ctrl)
				task2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 2)).AnyTimes()
				task2.EXPECT().State().Return(ctask.TaskStateAcked).AnyTimes()

				return []*task.MockTask{task1, task2}
			},
			pruneAcked:    true,
			wantMinKey:    persistence.MaximumHistoryTaskKey,
			wantHasMinKey: false,
			wantTaskCount: 0,
		},
		{
			name: "clear all tasks",
			setupTasks: func(ctrl *gomock.Controller) []*task.MockTask {
				task1 := task.NewMockTask(ctrl)
				task1.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 1)).AnyTimes()
				task1.EXPECT().Cancel().AnyTimes()
				task1.EXPECT().State().Return(ctask.TaskStateCanceled).Times(1)

				task2 := task.NewMockTask(ctrl)
				task2.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 2)).AnyTimes()
				task2.EXPECT().Cancel().AnyTimes()
				task2.EXPECT().State().Return(ctask.TaskStateCanceled).Times(1)

				task3 := task.NewMockTask(ctrl)
				task3.EXPECT().GetTaskKey().Return(persistence.NewHistoryTaskKey(testTime, 3)).AnyTimes()
				task3.EXPECT().Cancel().AnyTimes()
				task3.EXPECT().State().Return(ctask.TaskStateCanceled).Times(1)

				return []*task.MockTask{task1, task2, task3}
			},
			clear:         true,
			wantMinKey:    persistence.MaximumHistoryTaskKey,
			wantHasMinKey: false,
			wantTaskCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewPendingTaskTracker()

			ctrl := gomock.NewController(t)
			inputTasks := tt.setupTasks(ctrl)
			// Setup tasks
			for _, task := range inputTasks {
				tracker.AddTask(task)
			}

			// Prune acked tasks if needed
			if tt.pruneAcked {
				tracker.PruneAckedTasks()
			}

			// Clear all tasks if needed
			if tt.clear {
				tracker.Clear()
			}

			// Test GetMinimumTaskKey
			gotMinKey, gotHasMinKey := tracker.GetMinimumTaskKey()
			assert.Equal(t, tt.wantMinKey, gotMinKey)
			assert.Equal(t, tt.wantHasMinKey, gotHasMinKey)

			// Test GetTasks
			tasks := tracker.GetTasks()
			assert.Equal(t, tt.wantTaskCount, tracker.GetPendingTaskCount())

			// Verify all tasks are in the map
			for _, task := range inputTasks {
				if tt.clear {
					// After clear, no tasks should be in the map
					_, exists := tasks[task.GetTaskKey()]
					assert.False(t, exists, "After clear, no task should be in the map")
					assert.Equal(t, ctask.TaskStateCanceled, task.State())
				} else if tt.pruneAcked && task.State() == ctask.TaskStateAcked {
					_, exists := tasks[task.GetTaskKey()]
					assert.False(t, exists, "Acked task should not be in the map")
				} else {
					_, exists := tasks[task.GetTaskKey()]
					assert.True(t, exists, "Task should be in the map")
				}
			}
		})
	}
}
