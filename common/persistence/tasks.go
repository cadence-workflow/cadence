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
	ToTransferTaskInfo() (*TransferTaskInfo, error)
	ToTimerTaskInfo() (*TimerTaskInfo, error)
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

func (a *ActivityTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeActivityTask,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
		TargetDomainID:      a.TargetDomainID,
		TaskList:            a.TaskList,
		ScheduleID:          a.ScheduleID,
	}, nil
}

func (a *ActivityTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("activity task is not timer task")
}

// GetType returns the type of the decision task
func (d *DecisionTask) GetTaskType() int {
	return TransferTaskTypeDecisionTask
}

func (d *DecisionTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeDecisionTask,
		DomainID:            d.DomainID,
		WorkflowID:          d.WorkflowID,
		RunID:               d.RunID,
		TaskID:              d.TaskID,
		VisibilityTimestamp: d.VisibilityTimestamp,
		Version:             d.Version,
		TargetDomainID:      d.TargetDomainID,
		TaskList:            d.TaskList,
		ScheduleID:          d.ScheduleID,
	}, nil
}

func (d *DecisionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("decision task is not timer task")
}

// GetType returns the type of the record workflow started task
func (a *RecordWorkflowStartedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowStarted
}

func (a *RecordWorkflowStartedTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeRecordWorkflowStarted,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
	}, nil
}

func (a *RecordWorkflowStartedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record workflow started task is not timer task")
}

// GetType returns the type of the ResetWorkflowTask
func (a *ResetWorkflowTask) GetTaskType() int {
	return TransferTaskTypeResetWorkflow
}

func (a *ResetWorkflowTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeResetWorkflow,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
	}, nil
}

func (a *ResetWorkflowTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("reset workflow task is not timer task")
}

// GetType returns the type of the close execution task
func (a *CloseExecutionTask) GetTaskType() int {
	return TransferTaskTypeCloseExecution
}

func (a *CloseExecutionTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeCloseExecution,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
	}, nil
}

func (a *CloseExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("close execution task is not timer task")
}

// GetType returns the type of the delete execution task
func (a *DeleteHistoryEventTask) GetTaskType() int {
	return TaskTypeDeleteHistoryEvent
}

func (a *DeleteHistoryEventTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("delete history event task is not transfer task")
}

func (a *DeleteHistoryEventTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeDeleteHistoryEvent,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
	}, nil
}

// GetType returns the type of the timer task
func (d *DecisionTimeoutTask) GetTaskType() int {
	return TaskTypeDecisionTimeout
}

func (d *DecisionTimeoutTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("decision timeout task is not transfer task")
}

func (d *DecisionTimeoutTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeDecisionTimeout,
		DomainID:            d.DomainID,
		WorkflowID:          d.WorkflowID,
		RunID:               d.RunID,
		TaskID:              d.TaskID,
		VisibilityTimestamp: d.VisibilityTimestamp,
		Version:             d.Version,
		EventID:             d.EventID,
		ScheduleAttempt:     d.ScheduleAttempt,
		TimeoutType:         d.TimeoutType,
	}, nil
}

// GetType returns the type of the timer task
func (a *ActivityTimeoutTask) GetTaskType() int {
	return TaskTypeActivityTimeout
}

func (a *ActivityTimeoutTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("activity timeout task is not transfer task")
}

func (a *ActivityTimeoutTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeActivityTimeout,
		DomainID:            a.DomainID,
		WorkflowID:          a.WorkflowID,
		RunID:               a.RunID,
		TaskID:              a.TaskID,
		VisibilityTimestamp: a.VisibilityTimestamp,
		Version:             a.Version,
		EventID:             a.EventID,
		ScheduleAttempt:     a.Attempt,
		TimeoutType:         a.TimeoutType,
	}, nil
}

// GetType returns the type of the timer task
func (u *UserTimerTask) GetTaskType() int {
	return TaskTypeUserTimer
}

func (u *UserTimerTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("user timer task is not transfer task")
}

func (u *UserTimerTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeUserTimer,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
		EventID:             u.EventID,
	}, nil
}

// GetType returns the type of the retry timer task
func (r *ActivityRetryTimerTask) GetTaskType() int {
	return TaskTypeActivityRetryTimer
}

func (r *ActivityRetryTimerTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("activity retry timer task is not transfer task")
}

func (r *ActivityRetryTimerTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeActivityRetryTimer,
		DomainID:            r.DomainID,
		WorkflowID:          r.WorkflowID,
		RunID:               r.RunID,
		TaskID:              r.TaskID,
		VisibilityTimestamp: r.VisibilityTimestamp,
		Version:             r.Version,
		EventID:             r.EventID,
		ScheduleAttempt:     r.Attempt,
	}, nil
}

// GetType returns the type of the retry timer task
func (r *WorkflowBackoffTimerTask) GetTaskType() int {
	return TaskTypeWorkflowBackoffTimer
}

func (r *WorkflowBackoffTimerTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("workflow backoff timer task is not transfer task")
}

func (r *WorkflowBackoffTimerTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeWorkflowBackoffTimer,
		DomainID:            r.DomainID,
		WorkflowID:          r.WorkflowID,
		RunID:               r.RunID,
		TaskID:              r.TaskID,
		VisibilityTimestamp: r.VisibilityTimestamp,
		Version:             r.Version,
		TimeoutType:         r.TimeoutType,
	}, nil
}

// GetType returns the type of the timeout task.
func (u *WorkflowTimeoutTask) GetTaskType() int {
	return TaskTypeWorkflowTimeout
}

func (u *WorkflowTimeoutTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("workflow timeout task is not transfer task")
}

func (u *WorkflowTimeoutTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return &TimerTaskInfo{
		TaskType:            TaskTypeWorkflowTimeout,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
	}, nil
}

// GetType returns the type of the cancel transfer task
func (u *CancelExecutionTask) GetTaskType() int {
	return TransferTaskTypeCancelExecution
}

func (u *CancelExecutionTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:                TransferTaskTypeCancelExecution,
		DomainID:                u.DomainID,
		WorkflowID:              u.WorkflowID,
		RunID:                   u.RunID,
		TaskID:                  u.TaskID,
		VisibilityTimestamp:     u.VisibilityTimestamp,
		Version:                 u.Version,
		TargetDomainID:          u.TargetDomainID,
		TargetWorkflowID:        u.TargetWorkflowID,
		TargetRunID:             u.TargetRunID,
		TargetChildWorkflowOnly: u.TargetChildWorkflowOnly,
		ScheduleID:              u.InitiatedID,
	}, nil
}

func (u *CancelExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("cancel execution task is not timer task")
}

// GetType returns the type of the signal transfer task
func (u *SignalExecutionTask) GetTaskType() int {
	return TransferTaskTypeSignalExecution
}

func (u *SignalExecutionTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:                TransferTaskTypeSignalExecution,
		DomainID:                u.DomainID,
		WorkflowID:              u.WorkflowID,
		RunID:                   u.RunID,
		TaskID:                  u.TaskID,
		VisibilityTimestamp:     u.VisibilityTimestamp,
		Version:                 u.Version,
		TargetDomainID:          u.TargetDomainID,
		TargetWorkflowID:        u.TargetWorkflowID,
		TargetRunID:             u.TargetRunID,
		TargetChildWorkflowOnly: u.TargetChildWorkflowOnly,
		ScheduleID:              u.InitiatedID,
	}, nil
}

func (u *SignalExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("signal execution task is not timer task")
}

// GetType returns the type of the record child execution completed task
func (u *RecordChildExecutionCompletedTask) GetTaskType() int {
	return TransferTaskTypeRecordChildExecutionCompleted
}

func (u *RecordChildExecutionCompletedTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeRecordChildExecutionCompleted,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
		TargetDomainID:      u.TargetDomainID,
		TargetWorkflowID:    u.TargetWorkflowID,
		TargetRunID:         u.TargetRunID,
	}, nil
}

func (u *RecordChildExecutionCompletedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record child execution completed task is not timer task")
}

// GetType returns the type of the upsert search attributes transfer task
func (u *UpsertWorkflowSearchAttributesTask) GetTaskType() int {
	return TransferTaskTypeUpsertWorkflowSearchAttributes
}

func (u *UpsertWorkflowSearchAttributesTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeUpsertWorkflowSearchAttributes,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
	}, nil
}

func (u *UpsertWorkflowSearchAttributesTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("upsert workflow search attributes task is not timer task")
}

// GetType returns the type of the start child transfer task
func (u *StartChildExecutionTask) GetTaskType() int {
	return TransferTaskTypeStartChildExecution
}

func (u *StartChildExecutionTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeStartChildExecution,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
		TargetDomainID:      u.TargetDomainID,
		TargetWorkflowID:    u.TargetWorkflowID,
		ScheduleID:          u.InitiatedID,
	}, nil
}

func (u *StartChildExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("start child execution task is not timer task")
}

// GetType returns the type of the record workflow closed task
func (u *RecordWorkflowClosedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowClosed
}

func (u *RecordWorkflowClosedTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return &TransferTaskInfo{
		TaskType:            TransferTaskTypeRecordWorkflowClosed,
		DomainID:            u.DomainID,
		WorkflowID:          u.WorkflowID,
		RunID:               u.RunID,
		TaskID:              u.TaskID,
		VisibilityTimestamp: u.VisibilityTimestamp,
		Version:             u.Version,
	}, nil
}

func (u *RecordWorkflowClosedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record workflow closed task is not timer task")
}

// GetType returns the type of the history replication task
func (a *HistoryReplicationTask) GetTaskType() int {
	return ReplicationTaskTypeHistory
}

func (a *HistoryReplicationTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("history replication task is not transfer task")
}

func (a *HistoryReplicationTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("history replication task is not timer task")
}

// GetType returns the type of the sync activity task
func (a *SyncActivityTask) GetTaskType() int {
	return ReplicationTaskTypeSyncActivity
}

func (a *SyncActivityTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("sync activity task is not transfer task")
}

func (a *SyncActivityTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("sync activity task is not timer task")
}

// GetType returns the type of the history replication task
func (a *FailoverMarkerTask) GetTaskType() int {
	return ReplicationTaskTypeFailoverMarker
}

func (a *FailoverMarkerTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("failover marker task is not transfer task")
}

func (a *FailoverMarkerTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("failover marker task is not timer task")
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
