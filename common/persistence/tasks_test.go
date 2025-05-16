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
