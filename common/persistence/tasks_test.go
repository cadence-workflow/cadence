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
			assert.Equal(t, TransferTaskTypeActivityTask, ty.GetType())
		case *DecisionTask:
			assert.Equal(t, TransferTaskTypeDecisionTask, ty.GetType())
		case *RecordWorkflowStartedTask:
			assert.Equal(t, TransferTaskTypeRecordWorkflowStarted, ty.GetType())
		case *ResetWorkflowTask:
			assert.Equal(t, TransferTaskTypeResetWorkflow, ty.GetType())
		case *CloseExecutionTask:
			assert.Equal(t, TransferTaskTypeCloseExecution, ty.GetType())
		case *DeleteHistoryEventTask:
			assert.Equal(t, TaskTypeDeleteHistoryEvent, ty.GetType())
		case *DecisionTimeoutTask:
			assert.Equal(t, TaskTypeDecisionTimeout, ty.GetType())
		case *ActivityTimeoutTask:
			assert.Equal(t, TaskTypeActivityTimeout, ty.GetType())
		case *UserTimerTask:
			assert.Equal(t, TaskTypeUserTimer, ty.GetType())
		case *ActivityRetryTimerTask:
			assert.Equal(t, TaskTypeActivityRetryTimer, ty.GetType())
		case *WorkflowBackoffTimerTask:
			assert.Equal(t, TaskTypeWorkflowBackoffTimer, ty.GetType())
		case *WorkflowTimeoutTask:
			assert.Equal(t, TaskTypeWorkflowTimeout, ty.GetType())
		case *CancelExecutionTask:
			assert.Equal(t, TransferTaskTypeCancelExecution, ty.GetType())
		case *SignalExecutionTask:
			assert.Equal(t, TransferTaskTypeSignalExecution, ty.GetType())
		case *RecordChildExecutionCompletedTask:
			assert.Equal(t, TransferTaskTypeRecordChildExecutionCompleted, ty.GetType())
		case *UpsertWorkflowSearchAttributesTask:
			assert.Equal(t, TransferTaskTypeUpsertWorkflowSearchAttributes, ty.GetType())
		case *StartChildExecutionTask:
			assert.Equal(t, TransferTaskTypeStartChildExecution, ty.GetType())
		case *RecordWorkflowClosedTask:
			assert.Equal(t, TransferTaskTypeRecordWorkflowClosed, ty.GetType())
		case *HistoryReplicationTask:
			assert.Equal(t, ReplicationTaskTypeHistory, ty.GetType())
		case *SyncActivityTask:
			assert.Equal(t, ReplicationTaskTypeSyncActivity, ty.GetType())
		case *FailoverMarkerTask:
			assert.Equal(t, ReplicationTaskTypeFailoverMarker, ty.GetType())
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
	}
}
