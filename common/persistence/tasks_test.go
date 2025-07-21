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

package persistence

import (
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
)

func TestTaskCommonMethods(t *testing.T) {
	timeNow := time.Now()
	tasks := []Task{
		&ActivityTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowStartedTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ResetWorkflowTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CloseExecutionTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DeleteHistoryEventTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTimeoutTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityTimeoutTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UserTimerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityRetryTimerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowBackoffTimerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowTimeoutTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CancelExecutionTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SignalExecutionTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordChildExecutionCompletedTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UpsertWorkflowSearchAttributesTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&StartChildExecutionTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowClosedTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&HistoryReplicationTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SyncActivityTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&FailoverMarkerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
	}

	for _, task := range tasks {
		switch ty := task.(type) {
		case *ActivityTask:
			assert.Equal(t, TransferTaskTypeActivityTask, ty.GetTaskType())
		case *DecisionTask:
			assert.Equal(t, TransferTaskTypeDecisionTask, ty.GetTaskType())
		case *RecordWorkflowStartedTask:
			assert.Equal(t, TransferTaskTypeRecordWorkflowStarted, ty.GetTaskType())
		case *ResetWorkflowTask:
			assert.Equal(t, TransferTaskTypeResetWorkflow, ty.GetTaskType())
		case *CloseExecutionTask:
			assert.Equal(t, TransferTaskTypeCloseExecution, ty.GetTaskType())
		case *DeleteHistoryEventTask:
			assert.Equal(t, TaskTypeDeleteHistoryEvent, ty.GetTaskType())
		case *DecisionTimeoutTask:
			assert.Equal(t, TaskTypeDecisionTimeout, ty.GetTaskType())
		case *ActivityTimeoutTask:
			assert.Equal(t, TaskTypeActivityTimeout, ty.GetTaskType())
		case *UserTimerTask:
			assert.Equal(t, TaskTypeUserTimer, ty.GetTaskType())
		case *ActivityRetryTimerTask:
			assert.Equal(t, TaskTypeActivityRetryTimer, ty.GetTaskType())
		case *WorkflowBackoffTimerTask:
			assert.Equal(t, TaskTypeWorkflowBackoffTimer, ty.GetTaskType())
		case *WorkflowTimeoutTask:
			assert.Equal(t, TaskTypeWorkflowTimeout, ty.GetTaskType())
		case *CancelExecutionTask:
			assert.Equal(t, TransferTaskTypeCancelExecution, ty.GetTaskType())
		case *SignalExecutionTask:
			assert.Equal(t, TransferTaskTypeSignalExecution, ty.GetTaskType())
		case *RecordChildExecutionCompletedTask:
			assert.Equal(t, TransferTaskTypeRecordChildExecutionCompleted, ty.GetTaskType())
		case *UpsertWorkflowSearchAttributesTask:
			assert.Equal(t, TransferTaskTypeUpsertWorkflowSearchAttributes, ty.GetTaskType())
		case *StartChildExecutionTask:
			assert.Equal(t, TransferTaskTypeStartChildExecution, ty.GetTaskType())
		case *RecordWorkflowClosedTask:
			assert.Equal(t, TransferTaskTypeRecordWorkflowClosed, ty.GetTaskType())
		case *HistoryReplicationTask:
			assert.Equal(t, ReplicationTaskTypeHistory, ty.GetTaskType())
		case *SyncActivityTask:
			assert.Equal(t, ReplicationTaskTypeSyncActivity, ty.GetTaskType())
		case *FailoverMarkerTask:
			assert.Equal(t, ReplicationTaskTypeFailoverMarker, ty.GetTaskType())
		default:
			t.Fatalf("Unhandled task type: %T", t)
		}

		// Test version methods
		assert.Equal(t, int64(1), task.GetVersion())
		task.SetVersion(2)
		assert.Equal(t, int64(2), task.GetVersion())

		// Test TaskID methods
		assert.Equal(t, int64(1), task.GetTaskID())
		task.SetTaskID(2)
		assert.Equal(t, int64(2), task.GetTaskID())

		// Test VisibilityTimestamp methods
		assert.Equal(t, timeNow, task.GetVisibilityTimestamp())
		newTime := timeNow.Add(time.Second)
		task.SetVisibilityTimestamp(newTime)
		assert.Equal(t, newTime, task.GetVisibilityTimestamp())

		if task.GetTaskCategory().Type() == HistoryTaskCategoryTypeImmediate {
			assert.Equal(t, NewImmediateTaskKey(task.GetTaskID()), task.GetTaskKey())
		} else {
			assert.Equal(t, NewHistoryTaskKey(task.GetVisibilityTimestamp(), task.GetTaskID()), task.GetTaskKey())
		}
	}
}

func TestTransferTaskMapping(t *testing.T) {
	f := fuzz.New().NilChance(0.0)
	tasks := []Task{
		&ActivityTask{},
		&DecisionTask{},
		&RecordWorkflowStartedTask{},
		&ResetWorkflowTask{},
		&CloseExecutionTask{},
		&CancelExecutionTask{},
		&SignalExecutionTask{},
		&RecordChildExecutionCompletedTask{},
		&UpsertWorkflowSearchAttributesTask{},
		&StartChildExecutionTask{},
		&RecordWorkflowClosedTask{},
	}
	for i := 0; i < 1000; i++ {
		for _, task := range tasks {
			f.Fuzz(task)
			transfer, err := task.ToTransferTaskInfo()
			assert.NoError(t, err)
			t.Log(transfer)
			task2, err := transfer.ToTask()
			assert.NoError(t, err)
			assert.Equal(t, task, task2)
		}
	}
}

func TestTimerTaskMapping(t *testing.T) {
	f := fuzz.New().NilChance(0.0)
	tasks := []Task{
		&DecisionTimeoutTask{},
		&ActivityTimeoutTask{},
		&DeleteHistoryEventTask{},
		&WorkflowTimeoutTask{},
		&UserTimerTask{},
		&ActivityRetryTimerTask{},
		&WorkflowBackoffTimerTask{},
	}
	for i := 0; i < 1000; i++ {
		for _, task := range tasks {
			f.Fuzz(task)
			timer, err := task.ToTimerTaskInfo()
			assert.NoError(t, err)
			task2, err := timer.ToTask()
			assert.NoError(t, err)
			assert.Equal(t, task, task2)
		}
	}
}

func TestHistoryTaskKeyComparison(t *testing.T) {
	now := time.Now()
	key1 := NewHistoryTaskKey(now, 1)
	key2 := NewHistoryTaskKey(now, 2)
	key3 := NewHistoryTaskKey(now.Add(time.Second), 1)

	assert.Equal(t, 0, key1.Compare(key1))
	assert.Equal(t, -1, key1.Compare(key2))
	assert.Equal(t, 1, key2.Compare(key1))

	assert.Equal(t, -1, key2.Compare(key3))
	assert.Equal(t, 1, key3.Compare(key2))
}

func TestIsTaskCorrupted(t *testing.T) {
	timeNow := time.Now()

	tests := []struct {
		name     string
		task     Task
		expected bool
	}{
		{
			name: "Valid ActivityTask",
			task: &ActivityTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: false,
		},
		{
			name: "Valid DecisionTask",
			task: &DecisionTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: false,
		},
		{
			name: "Valid TimerTask",
			task: &UserTimerTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				EventID: 123,
			},
			expected: false,
		},
		{
			name: "Valid ReplicationTask",
			task: &HistoryReplicationTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				FirstEventID: 1,
				NextEventID:  10,
			},
			expected: false,
		},
		{
			name: "Task with empty DomainID",
			task: &ActivityTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: true,
		},
		{
			name: "Task with empty WorkflowID",
			task: &DecisionTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: true,
		},
		{
			name: "Task with empty RunID",
			task: &UserTimerTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "test-workflow",
					RunID:      "",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				EventID: 123,
			},
			expected: true,
		},
		{
			name: "Task with all empty identifiers",
			task: &ActivityTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "",
					WorkflowID: "",
					RunID:      "",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				TimeoutType: 1,
				EventID:     123,
				Attempt:     1,
			},
			expected: true,
		},
		{
			name: "Task with whitespace-only identifiers",
			task: &WorkflowTimeoutTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "   ",
					WorkflowID: "test-workflow",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: false, // Whitespace is not empty string
		},
		{
			name: "FailoverMarkerTask with empty DomainID",
			task: &FailoverMarkerTask{
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				DomainID: "",
			},
			expected: true,
		},
		{
			name: "FailoverMarkerTask with valid DomainID",
			task: &FailoverMarkerTask{
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				DomainID: "test-domain",
			},
			expected: false, // FailoverMarkerTask only has DomainID, WorkflowID and RunID are empty by design
		},
		{
			name: "Task with mixed empty and non-empty identifiers",
			task: &CancelExecutionTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "test-domain",
					WorkflowID: "",
					RunID:      "test-run",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				TargetDomainID:          "target-domain",
				TargetWorkflowID:        "target-workflow",
				TargetRunID:             "target-run",
				TargetChildWorkflowOnly: false,
				InitiatedID:             123,
			},
			expected: true,
		},
		{
			name: "Task with very long identifiers",
			task: &SignalExecutionTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "very-long-domain-id-that-exceeds-normal-length",
					WorkflowID: "very-long-workflow-id-that-exceeds-normal-length",
					RunID:      "very-long-run-id-that-exceeds-normal-length",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				TargetDomainID:          "target-domain",
				TargetWorkflowID:        "target-workflow",
				TargetRunID:             "target-run",
				TargetChildWorkflowOnly: false,
				InitiatedID:             123,
			},
			expected: false,
		},
		{
			name: "Task with special characters in identifiers",
			task: &StartChildExecutionTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "domain-with-special-chars-!@#$%^&*()",
					WorkflowID: "workflow-with-special-chars-!@#$%^&*()",
					RunID:      "run-with-special-chars-!@#$%^&*()",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
				TargetDomainID:   "target-domain",
				TargetWorkflowID: "target-workflow",
				InitiatedID:      123,
			},
			expected: false,
		},
		{
			name: "Task with unicode characters in identifiers",
			task: &RecordWorkflowStartedTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "domain-with-unicode-测试",
					WorkflowID: "workflow-with-unicode-测试",
					RunID:      "run-with-unicode-测试",
				},
				TaskData: TaskData{
					Version:             1,
					TaskID:              1,
					VisibilityTimestamp: timeNow,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTaskCorrupted(tt.task)
			assert.Equal(t, tt.expected, result, "IsTaskCorrupted() = %v, want %v", result, tt.expected)
		})
	}
}

func TestIsTaskCorruptedWithAllTaskTypes(t *testing.T) {
	timeNow := time.Now()
	validIdentifier := WorkflowIdentifier{
		DomainID:   "test-domain",
		WorkflowID: "test-workflow",
		RunID:      "test-run",
	}
	emptyIdentifier := WorkflowIdentifier{
		DomainID:   "",
		WorkflowID: "",
		RunID:      "",
	}

	// Test all task types with valid identifiers
	validTasks := []Task{
		&ActivityTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowStartedTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ResetWorkflowTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CloseExecutionTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DeleteHistoryEventTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTimeoutTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowTimeoutTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CancelExecutionTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SignalExecutionTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordChildExecutionCompletedTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UpsertWorkflowSearchAttributesTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&StartChildExecutionTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowClosedTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityTimeoutTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UserTimerTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityRetryTimerTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowBackoffTimerTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&HistoryReplicationTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SyncActivityTask{WorkflowIdentifier: validIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&FailoverMarkerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}, DomainID: "test-domain"},
	}

	// Test all task types with empty identifiers
	corruptedTasks := []Task{
		&ActivityTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowStartedTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ResetWorkflowTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CloseExecutionTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DeleteHistoryEventTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&DecisionTimeoutTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowTimeoutTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&CancelExecutionTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SignalExecutionTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordChildExecutionCompletedTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UpsertWorkflowSearchAttributesTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&StartChildExecutionTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&RecordWorkflowClosedTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityTimeoutTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&UserTimerTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&ActivityRetryTimerTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&WorkflowBackoffTimerTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&HistoryReplicationTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&SyncActivityTask{WorkflowIdentifier: emptyIdentifier, TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}},
		&FailoverMarkerTask{TaskData: TaskData{Version: 1, TaskID: 1, VisibilityTimestamp: timeNow}, DomainID: ""},
	}

	t.Run("All task types with valid identifiers should not be corrupted", func(t *testing.T) {
		for i, task := range validTasks {
			result := IsTaskCorrupted(task)
			assert.False(t, result, "Task type %T at index %d should not be corrupted", task, i)
		}
	})

	t.Run("All task types with empty identifiers should be corrupted", func(t *testing.T) {
		for i, task := range corruptedTasks {
			result := IsTaskCorrupted(task)
			assert.True(t, result, "Task type %T at index %d should be corrupted", task, i)
		}
	})
}
