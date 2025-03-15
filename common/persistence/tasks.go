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
	"fmt"
	"time"
)

// Task is the generic interface for workflow tasks
type Task interface {
	GetTaskType() int
	GetDomainID() string
	GetWorkflowID() string
	GetRunID() string
	GetVersion() int64
	SetVersion(version int64)
	GetTaskID() int64
	SetTaskID(id int64)
	GetVisibilityTimestamp() time.Time
	SetVisibilityTimestamp(timestamp time.Time)
}

type (
	HistoryTaskKey struct {
		ScheduledTime time.Time
		TaskID        int64
	}

	WorkflowIdentifier struct {
		DomainID   string
		WorkflowID string
		RunID      string
	}
	// TaskData is common attributes for all tasks.
	TaskData struct {
		Version             int64
		TaskID              int64
		VisibilityTimestamp time.Time
	}

	// ActivityTask identifies a transfer task for activity
	ActivityTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID string
		TaskList       string
		ScheduleID     int64
	}

	// DecisionTask identifies a transfer task for decision
	DecisionTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID string
		TaskList       string
		ScheduleID     int64
	}

	// RecordWorkflowStartedTask identifites a transfer task for writing visibility open execution record
	RecordWorkflowStartedTask struct {
		WorkflowIdentifier
		TaskData
	}

	// ResetWorkflowTask identifites a transfer task to reset workflow
	ResetWorkflowTask struct {
		WorkflowIdentifier
		TaskData
	}

	// CloseExecutionTask identifies a transfer task for deletion of execution
	CloseExecutionTask struct {
		WorkflowIdentifier
		TaskData
	}

	// DeleteHistoryEventTask identifies a timer task for deletion of history events of completed execution.
	DeleteHistoryEventTask struct {
		WorkflowIdentifier
		TaskData
	}

	// DecisionTimeoutTask identifies a timeout task.
	DecisionTimeoutTask struct {
		WorkflowIdentifier
		TaskData
		EventID         int64
		ScheduleAttempt int64
		TimeoutType     int
	}

	// WorkflowTimeoutTask identifies a timeout task.
	WorkflowTimeoutTask struct {
		WorkflowIdentifier
		TaskData
	}

	// CancelExecutionTask identifies a transfer task for cancel of execution
	CancelExecutionTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID          string
		TargetWorkflowID        string
		TargetRunID             string
		TargetChildWorkflowOnly bool
		InitiatedID             int64
	}

	// SignalExecutionTask identifies a transfer task for signal execution
	SignalExecutionTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID          string
		TargetWorkflowID        string
		TargetRunID             string
		TargetChildWorkflowOnly bool
		InitiatedID             int64
	}

	// UpsertWorkflowSearchAttributesTask identifies a transfer task for upsert search attributes
	UpsertWorkflowSearchAttributesTask struct {
		WorkflowIdentifier
		TaskData
	}

	// StartChildExecutionTask identifies a transfer task for starting child execution
	StartChildExecutionTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		InitiatedID      int64
	}

	// RecordWorkflowClosedTask identifies a transfer task for writing visibility close execution record
	RecordWorkflowClosedTask struct {
		WorkflowIdentifier
		TaskData
	}

	// RecordChildExecutionCompletedTask identifies a task for recording the competion of a child workflow
	RecordChildExecutionCompletedTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		TargetRunID      string
	}

	// CrossClusterStartChildExecutionTask is the cross-cluster version of StartChildExecutionTask
	CrossClusterStartChildExecutionTask struct {
		StartChildExecutionTask

		TargetCluster string
	}

	// CrossClusterCancelExecutionTask is the cross-cluster version of CancelExecutionTask
	CrossClusterCancelExecutionTask struct {
		CancelExecutionTask

		TargetCluster string
	}

	// CrossClusterSignalExecutionTask is the cross-cluster version of SignalExecutionTask
	CrossClusterSignalExecutionTask struct {
		SignalExecutionTask

		TargetCluster string
	}

	// CrossClusterRecordChildExecutionCompletedTask is the cross-cluster version of RecordChildExecutionCompletedTask
	CrossClusterRecordChildExecutionCompletedTask struct {
		RecordChildExecutionCompletedTask

		TargetCluster string
	}

	// ActivityTimeoutTask identifies a timeout task.
	ActivityTimeoutTask struct {
		WorkflowIdentifier
		TaskData
		TimeoutType int
		EventID     int64
		Attempt     int64
	}

	// UserTimerTask identifies a timeout task.
	UserTimerTask struct {
		WorkflowIdentifier
		TaskData
		EventID int64
	}

	// ActivityRetryTimerTask to schedule a retry task for activity
	ActivityRetryTimerTask struct {
		WorkflowIdentifier
		TaskData
		EventID int64
		Attempt int64
	}

	// WorkflowBackoffTimerTask to schedule first decision task for retried workflow
	WorkflowBackoffTimerTask struct {
		WorkflowIdentifier
		TaskData
		TimeoutType int // 0 for retry, 1 for cron.
	}

	// HistoryReplicationTask is the replication task created for shipping history replication events to other clusters
	HistoryReplicationTask struct {
		WorkflowIdentifier
		TaskData
		FirstEventID      int64
		NextEventID       int64
		BranchToken       []byte
		NewRunBranchToken []byte
	}

	// SyncActivityTask is the replication task created for shipping activity info to other clusters
	SyncActivityTask struct {
		WorkflowIdentifier
		TaskData
		ScheduledID int64
	}

	// FailoverMarkerTask is the marker for graceful failover
	FailoverMarkerTask struct {
		TaskData
		DomainID string
	}
)

// assert all task types implements Task interface
var (
	_ Task = (*ActivityTask)(nil)
	_ Task = (*DecisionTask)(nil)
	_ Task = (*RecordWorkflowStartedTask)(nil)
	_ Task = (*ResetWorkflowTask)(nil)
	_ Task = (*CloseExecutionTask)(nil)
	_ Task = (*DeleteHistoryEventTask)(nil)
	_ Task = (*DecisionTimeoutTask)(nil)
	_ Task = (*WorkflowTimeoutTask)(nil)
	_ Task = (*CancelExecutionTask)(nil)
	_ Task = (*SignalExecutionTask)(nil)
	_ Task = (*RecordChildExecutionCompletedTask)(nil)
	_ Task = (*UpsertWorkflowSearchAttributesTask)(nil)
	_ Task = (*StartChildExecutionTask)(nil)
	_ Task = (*RecordWorkflowClosedTask)(nil)
	_ Task = (*CrossClusterStartChildExecutionTask)(nil)
	_ Task = (*CrossClusterCancelExecutionTask)(nil)
	_ Task = (*CrossClusterSignalExecutionTask)(nil)
	_ Task = (*CrossClusterRecordChildExecutionCompletedTask)(nil)
	_ Task = (*ActivityTimeoutTask)(nil)
	_ Task = (*UserTimerTask)(nil)
	_ Task = (*ActivityRetryTimerTask)(nil)
	_ Task = (*WorkflowBackoffTimerTask)(nil)
	_ Task = (*HistoryReplicationTask)(nil)
	_ Task = (*SyncActivityTask)(nil)
	_ Task = (*FailoverMarkerTask)(nil)
)

func (a *WorkflowIdentifier) GetDomainID() string {
	return a.DomainID
}

func (a *WorkflowIdentifier) GetWorkflowID() string {
	return a.WorkflowID
}

func (a *WorkflowIdentifier) GetRunID() string {
	return a.RunID
}

// GetVersion returns the version of the task
func (a *TaskData) GetVersion() int64 {
	return a.Version
}

// SetVersion sets the version of the task
func (a *TaskData) SetVersion(version int64) {
	a.Version = version
}

// GetTaskID returns the sequence ID of the task
func (a *TaskData) GetTaskID() int64 {
	return a.TaskID
}

// SetTaskID sets the sequence ID of the task
func (a *TaskData) SetTaskID(id int64) {
	a.TaskID = id
}

// GetVisibilityTimestamp get the visibility timestamp
func (a *TaskData) GetVisibilityTimestamp() time.Time {
	return a.VisibilityTimestamp
}

// SetVisibilityTimestamp set the visibility timestamp
func (a *TaskData) SetVisibilityTimestamp(timestamp time.Time) {
	a.VisibilityTimestamp = timestamp
}

// GetType returns the type of the activity task
func (a *ActivityTask) GetTaskType() int {
	return TransferTaskTypeActivityTask
}

// GetType returns the type of the decision task
func (d *DecisionTask) GetTaskType() int {
	return TransferTaskTypeDecisionTask
}

// GetVersion returns the version of the decision task
func (d *DecisionTask) GetVersion() int64 {
	return d.Version
}

// GetType returns the type of the record workflow started task
func (a *RecordWorkflowStartedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowStarted
}

// GetType returns the type of the ResetWorkflowTask
func (a *ResetWorkflowTask) GetTaskType() int {
	return TransferTaskTypeResetWorkflow
}

// GetType returns the type of the close execution task
func (a *CloseExecutionTask) GetTaskType() int {
	return TransferTaskTypeCloseExecution
}

// GetType returns the type of the delete execution task
func (a *DeleteHistoryEventTask) GetTaskType() int {
	return TaskTypeDeleteHistoryEvent
}

// GetType returns the type of the timer task
func (d *DecisionTimeoutTask) GetTaskType() int {
	return TaskTypeDecisionTimeout
}

// GetType returns the type of the timer task
func (a *ActivityTimeoutTask) GetTaskType() int {
	return TaskTypeActivityTimeout
}

// GetType returns the type of the timer task
func (u *UserTimerTask) GetTaskType() int {
	return TaskTypeUserTimer
}

// GetType returns the type of the retry timer task
func (r *ActivityRetryTimerTask) GetTaskType() int {
	return TaskTypeActivityRetryTimer
}

// GetType returns the type of the retry timer task
func (r *WorkflowBackoffTimerTask) GetTaskType() int {
	return TaskTypeWorkflowBackoffTimer
}

// GetType returns the type of the timeout task.
func (u *WorkflowTimeoutTask) GetTaskType() int {
	return TaskTypeWorkflowTimeout
}

// GetType returns the type of the cancel transfer task
func (u *CancelExecutionTask) GetTaskType() int {
	return TransferTaskTypeCancelExecution
}

// GetType returns the type of the signal transfer task
func (u *SignalExecutionTask) GetTaskType() int {
	return TransferTaskTypeSignalExecution
}

// GetType returns the type of the record child execution completed task
func (u *RecordChildExecutionCompletedTask) GetTaskType() int {
	return TransferTaskTypeRecordChildExecutionCompleted
}

// GetType returns the type of the upsert search attributes transfer task
func (u *UpsertWorkflowSearchAttributesTask) GetTaskType() int {
	return TransferTaskTypeUpsertWorkflowSearchAttributes
}

// GetType returns the type of the start child transfer task
func (u *StartChildExecutionTask) GetTaskType() int {
	return TransferTaskTypeStartChildExecution
}

// GetType returns the type of the record workflow closed task
func (u *RecordWorkflowClosedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowClosed
}

// GetType returns of type of the cross-cluster start child task
func (c *CrossClusterStartChildExecutionTask) GetTaskType() int {
	return CrossClusterTaskTypeStartChildExecution
}

// GetType returns of type of the cross-cluster cancel task
func (c *CrossClusterCancelExecutionTask) GetTaskType() int {
	return CrossClusterTaskTypeCancelExecution
}

// GetType returns of type of the cross-cluster signal task
func (c *CrossClusterSignalExecutionTask) GetTaskType() int {
	return CrossClusterTaskTypeSignalExecution
}

// GetType returns of type of the cross-cluster record child workflow completion task
func (c *CrossClusterRecordChildExecutionCompletedTask) GetTaskType() int {
	return CrossClusterTaskTypeRecordChildExeuctionCompleted
}

// GetType returns the type of the history replication task
func (a *HistoryReplicationTask) GetTaskType() int {
	return ReplicationTaskTypeHistory
}

// GetType returns the type of the history replication task
func (a *SyncActivityTask) GetTaskType() int {
	return ReplicationTaskTypeSyncActivity
}

// GetType returns the type of the history replication task
func (a *FailoverMarkerTask) GetTaskType() int {
	return ReplicationTaskTypeFailoverMarker
}

func (a *FailoverMarkerTask) GetDomainID() string {
	return a.DomainID
}

func (a *FailoverMarkerTask) GetWorkflowID() string {
	return ""
}

func (a *FailoverMarkerTask) GetRunID() string {
	return ""
}

func FromTransferTaskInfo(t *TransferTaskInfo) (Task, error) {
	workflowIdentifier := WorkflowIdentifier{
		DomainID:   t.DomainID,
		WorkflowID: t.WorkflowID,
		RunID:      t.RunID,
	}
	taskData := TaskData{
		Version:             t.Version,
		TaskID:              t.TaskID,
		VisibilityTimestamp: t.VisibilityTimestamp,
	}
	switch t.TaskType {
	case TransferTaskTypeActivityTask:
		return &ActivityTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     t.TargetDomainID,
			TaskList:           t.TaskList,
			ScheduleID:         t.ScheduleID,
		}, nil
	case TransferTaskTypeDecisionTask:
		return &DecisionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     t.TargetDomainID,
			TaskList:           t.TaskList,
			ScheduleID:         t.ScheduleID,
		}, nil
	case TransferTaskTypeCloseExecution:
		return &CloseExecutionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TransferTaskTypeRecordWorkflowStarted:
		return &RecordWorkflowStartedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TransferTaskTypeResetWorkflow:
		return &ResetWorkflowTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TransferTaskTypeRecordWorkflowClosed:
		return &RecordWorkflowClosedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TransferTaskTypeRecordChildExecutionCompleted:
		return &RecordChildExecutionCompletedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     t.TargetDomainID,
			TargetWorkflowID:   t.TargetWorkflowID,
			TargetRunID:        t.TargetRunID,
		}, nil
	case TransferTaskTypeUpsertWorkflowSearchAttributes:
		return &UpsertWorkflowSearchAttributesTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TransferTaskTypeStartChildExecution:
		return &StartChildExecutionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     t.TargetDomainID,
			TargetWorkflowID:   t.TargetWorkflowID,
			InitiatedID:        t.ScheduleID,
		}, nil
	case TransferTaskTypeCancelExecution:
		return &CancelExecutionTask{
			WorkflowIdentifier:      workflowIdentifier,
			TaskData:                taskData,
			TargetDomainID:          t.TargetDomainID,
			TargetWorkflowID:        t.TargetWorkflowID,
			TargetRunID:             t.TargetRunID,
			InitiatedID:             t.ScheduleID,
			TargetChildWorkflowOnly: t.TargetChildWorkflowOnly,
		}, nil
	case TransferTaskTypeSignalExecution:
		return &SignalExecutionTask{
			WorkflowIdentifier:      workflowIdentifier,
			TaskData:                taskData,
			TargetDomainID:          t.TargetDomainID,
			TargetWorkflowID:        t.TargetWorkflowID,
			TargetRunID:             t.TargetRunID,
			InitiatedID:             t.ScheduleID,
			TargetChildWorkflowOnly: t.TargetChildWorkflowOnly,
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %d", t.TaskType)
	}
}

func ToTransferTaskInfo(task Task) (*TransferTaskInfo, error) {
	switch t := task.(type) {
	case *ActivityTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeActivityTask,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
			TargetDomainID:      t.TargetDomainID,
			TaskList:            t.TaskList,
			ScheduleID:          t.ScheduleID,
		}, nil
	case *DecisionTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeDecisionTask,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
			TargetDomainID:      t.TargetDomainID,
			TaskList:            t.TaskList,
			ScheduleID:          t.ScheduleID,
		}, nil
	case *CloseExecutionTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeCloseExecution,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *RecordWorkflowStartedTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeRecordWorkflowStarted,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *ResetWorkflowTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeResetWorkflow,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *RecordWorkflowClosedTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeRecordWorkflowClosed,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *RecordChildExecutionCompletedTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeRecordChildExecutionCompleted,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
			TargetDomainID:      t.TargetDomainID,
			TargetWorkflowID:    t.TargetWorkflowID,
			TargetRunID:         t.TargetRunID,
		}, nil
	case *UpsertWorkflowSearchAttributesTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeUpsertWorkflowSearchAttributes,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *StartChildExecutionTask:
		return &TransferTaskInfo{
			TaskType:            TransferTaskTypeStartChildExecution,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
			TargetDomainID:      t.TargetDomainID,
			TargetWorkflowID:    t.TargetWorkflowID,
			ScheduleID:          t.InitiatedID,
		}, nil
	case *CancelExecutionTask:
		return &TransferTaskInfo{
			TaskType:                TransferTaskTypeCancelExecution,
			DomainID:                t.DomainID,
			WorkflowID:              t.WorkflowID,
			RunID:                   t.RunID,
			TaskID:                  t.TaskID,
			VisibilityTimestamp:     t.VisibilityTimestamp,
			Version:                 t.Version,
			TargetDomainID:          t.TargetDomainID,
			TargetWorkflowID:        t.TargetWorkflowID,
			TargetRunID:             t.TargetRunID,
			TargetChildWorkflowOnly: t.TargetChildWorkflowOnly,
			ScheduleID:              t.InitiatedID,
		}, nil
	case *SignalExecutionTask:
		return &TransferTaskInfo{
			TaskType:                TransferTaskTypeSignalExecution,
			DomainID:                t.DomainID,
			WorkflowID:              t.WorkflowID,
			RunID:                   t.RunID,
			TaskID:                  t.TaskID,
			VisibilityTimestamp:     t.VisibilityTimestamp,
			Version:                 t.Version,
			TargetDomainID:          t.TargetDomainID,
			TargetWorkflowID:        t.TargetWorkflowID,
			TargetRunID:             t.TargetRunID,
			ScheduleID:              t.InitiatedID,
			TargetChildWorkflowOnly: t.TargetChildWorkflowOnly,
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %d", t.GetTaskType())
	}
}

func FromTimerTaskInfo(t *TimerTaskInfo) (Task, error) {
	workflowIdentifier := WorkflowIdentifier{
		DomainID:   t.DomainID,
		WorkflowID: t.WorkflowID,
		RunID:      t.RunID,
	}
	taskData := TaskData{
		Version:             t.Version,
		TaskID:              t.TaskID,
		VisibilityTimestamp: t.VisibilityTimestamp,
	}
	switch t.TaskType {
	case TaskTypeDecisionTimeout:
		return &DecisionTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            t.EventID,
			ScheduleAttempt:    t.ScheduleAttempt,
			TimeoutType:        t.TimeoutType,
		}, nil
	case TaskTypeActivityTimeout:
		return &ActivityTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TimeoutType:        t.TimeoutType,
			EventID:            t.EventID,
			Attempt:            t.ScheduleAttempt,
		}, nil
	case TaskTypeDeleteHistoryEvent:
		return &DeleteHistoryEventTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TaskTypeWorkflowTimeout:
		return &WorkflowTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
		}, nil
	case TaskTypeUserTimer:
		return &UserTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            t.EventID,
		}, nil
	case TaskTypeActivityRetryTimer:
		return &ActivityRetryTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            t.EventID,
			Attempt:            t.ScheduleAttempt,
		}, nil
	case TaskTypeWorkflowBackoffTimer:
		return &WorkflowBackoffTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TimeoutType:        t.TimeoutType,
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %d", t.TaskType)
	}
}

func ToTimerTaskInfo(task Task) (*TimerTaskInfo, error) {
	switch t := task.(type) {
	case *DecisionTimeoutTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeDecisionTimeout,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			EventID:             t.EventID,
			ScheduleAttempt:     t.ScheduleAttempt,
			Version:             t.Version,
			TimeoutType:         t.TimeoutType,
		}, nil
	case *ActivityTimeoutTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeActivityTimeout,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			EventID:             t.EventID,
			ScheduleAttempt:     t.Attempt,
			Version:             t.Version,
			TimeoutType:         t.TimeoutType,
		}, nil
	case *DeleteHistoryEventTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeDeleteHistoryEvent,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *WorkflowTimeoutTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeWorkflowTimeout,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
		}, nil
	case *UserTimerTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeUserTimer,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			EventID:             t.EventID,
			Version:             t.Version,
		}, nil
	case *ActivityRetryTimerTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeActivityRetryTimer,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			EventID:             t.EventID,
			ScheduleAttempt:     t.Attempt,
			Version:             t.Version,
		}, nil
	case *WorkflowBackoffTimerTask:
		return &TimerTaskInfo{
			TaskType:            TaskTypeWorkflowBackoffTimer,
			DomainID:            t.DomainID,
			WorkflowID:          t.WorkflowID,
			RunID:               t.RunID,
			TaskID:              t.TaskID,
			VisibilityTimestamp: t.VisibilityTimestamp,
			Version:             t.Version,
			TimeoutType:         t.TimeoutType,
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %d", t.GetTaskType())
	}
}

func FromInternalReplicationTaskInfo(t *InternalReplicationTaskInfo) (Task, error) {
	switch t.TaskType {
	case ReplicationTaskTypeHistory:
		return &HistoryReplicationTask{
			WorkflowIdentifier: WorkflowIdentifier{
				DomainID:   t.DomainID,
				WorkflowID: t.WorkflowID,
				RunID:      t.RunID,
			},
			TaskData: TaskData{
				Version:             t.Version,
				TaskID:              t.TaskID,
				VisibilityTimestamp: t.CreationTime,
			},
			FirstEventID:      t.FirstEventID,
			NextEventID:       t.NextEventID,
			BranchToken:       t.BranchToken,
			NewRunBranchToken: t.NewRunBranchToken,
		}, nil
	case ReplicationTaskTypeSyncActivity:
		return &SyncActivityTask{
			WorkflowIdentifier: WorkflowIdentifier{
				DomainID:   t.DomainID,
				WorkflowID: t.WorkflowID,
				RunID:      t.RunID,
			},
			TaskData: TaskData{
				Version:             t.Version,
				TaskID:              t.TaskID,
				VisibilityTimestamp: t.CreationTime,
			},
			ScheduledID: t.ScheduledID,
		}, nil
	case ReplicationTaskTypeFailoverMarker:
		return &FailoverMarkerTask{
			TaskData: TaskData{
				Version: t.Version,
				TaskID:  t.TaskID,
			},
			DomainID: t.DomainID,
		}, nil
	default:
		return nil, fmt.Errorf("unknown task type: %d", t.TaskType)
	}
}
