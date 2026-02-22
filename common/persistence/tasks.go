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
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

// Task is the generic interface for workflow tasks
type Task interface {
	GetTaskCategory() HistoryTaskCategory
	GetTaskKey() HistoryTaskKey
	GetTaskType() int
	GetDomainID() string
	GetWorkflowID() string
	GetRunID() string
	GetTaskList() string
	GetOriginalTaskList() string
	GetVersion() int64
	SetVersion(version int64)
	GetTaskID() int64
	SetTaskID(id int64)
	GetVisibilityTimestamp() time.Time
	SetVisibilityTimestamp(timestamp time.Time)
	ByteSize() uint64
	ToTransferTaskInfo() (*TransferTaskInfo, error)
	ToTimerTaskInfo() (*TimerTaskInfo, error)
	ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error)
}

var (
	MaximumHistoryTaskKey = HistoryTaskKey{
		scheduledTime: time.Unix(0, math.MaxInt64),
		taskID:        math.MaxInt64,
	}
)

type (
	HistoryTaskKey struct {
		scheduledTime time.Time
		taskID        int64
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
		TargetDomainID   string
		TaskList         string
		ScheduleID       int64
		OriginalTaskList string
	}

	// RecordWorkflowStartedTask identifites a transfer task for writing visibility open execution record
	RecordWorkflowStartedTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// ResetWorkflowTask identifites a transfer task to reset workflow
	ResetWorkflowTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// CloseExecutionTask identifies a transfer task for deletion of execution
	CloseExecutionTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// DeleteHistoryEventTask identifies a timer task for deletion of history events of completed execution.
	DeleteHistoryEventTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// DecisionTimeoutTask identifies a timeout task.
	DecisionTimeoutTask struct {
		WorkflowIdentifier
		TaskData
		EventID         int64
		ScheduleAttempt int64
		TimeoutType     int
		TaskList        string
	}

	// WorkflowTimeoutTask identifies a timeout task.
	WorkflowTimeoutTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
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
		TaskList                string
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
		TaskList                string
	}

	// UpsertWorkflowSearchAttributesTask identifies a transfer task for upsert search attributes
	UpsertWorkflowSearchAttributesTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// StartChildExecutionTask identifies a transfer task for starting child execution
	StartChildExecutionTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		InitiatedID      int64
		TaskList         string
	}

	// RecordWorkflowClosedTask identifies a transfer task for writing visibility close execution record
	RecordWorkflowClosedTask struct {
		WorkflowIdentifier
		TaskData
		TaskList string
	}

	// RecordChildExecutionCompletedTask identifies a task for recording the competion of a child workflow
	RecordChildExecutionCompletedTask struct {
		WorkflowIdentifier
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		TargetRunID      string
		TaskList         string
	}

	// ActivityTimeoutTask identifies a timeout task.
	ActivityTimeoutTask struct {
		WorkflowIdentifier
		TaskData
		TimeoutType int
		EventID     int64
		Attempt     int64
		TaskList    string
	}

	// UserTimerTask identifies a timeout task.
	UserTimerTask struct {
		WorkflowIdentifier
		TaskData
		EventID  int64
		TaskList string
	}

	// ActivityRetryTimerTask to schedule a retry task for activity
	ActivityRetryTimerTask struct {
		WorkflowIdentifier
		TaskData
		EventID  int64
		Attempt  int64
		TaskList string
	}

	// WorkflowBackoffTimerTask to schedule first decision task for retried workflow
	WorkflowBackoffTimerTask struct {
		WorkflowIdentifier
		TaskData
		TimeoutType int // 0 for retry, 1 for cron.
		TaskList    string
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

	immediateTaskKeyScheduleTime = time.Unix(0, 0).UTC()
)

func IsTaskCorrupted(task Task) bool {
	switch task.(type) {
	case *FailoverMarkerTask:
		return task.GetDomainID() == ""
	default:
		return task.GetDomainID() == "" || task.GetWorkflowID() == "" || task.GetRunID() == ""
	}
}

func NewImmediateTaskKey(taskID int64) HistoryTaskKey {
	return HistoryTaskKey{
		scheduledTime: immediateTaskKeyScheduleTime,
		taskID:        taskID,
	}
}

func NewHistoryTaskKey(scheduledTime time.Time, taskID int64) HistoryTaskKey {
	return HistoryTaskKey{
		scheduledTime: scheduledTime,
		taskID:        taskID,
	}
}

func (a HistoryTaskKey) GetTaskID() int64 {
	return a.taskID
}

func (a HistoryTaskKey) GetScheduledTime() time.Time {
	return a.scheduledTime
}

func (a HistoryTaskKey) Compare(b HistoryTaskKey) int {
	if a.scheduledTime.Before(b.scheduledTime) {
		return -1
	} else if a.scheduledTime.After(b.scheduledTime) {
		return 1
	}
	if a.taskID < b.taskID {
		return -1
	} else if a.taskID > b.taskID {
		return 1
	}
	return 0
}

func (a HistoryTaskKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"scheduledTime": a.scheduledTime.UTC(),
		"taskID":        a.taskID,
	})
}

func (a HistoryTaskKey) Next() HistoryTaskKey {
	if a.taskID == math.MaxInt64 {
		return HistoryTaskKey{
			scheduledTime: a.scheduledTime.Add(time.Nanosecond),
			taskID:        0,
		}
	}
	return HistoryTaskKey{
		scheduledTime: a.scheduledTime,
		taskID:        a.taskID + 1,
	}
}

func MinHistoryTaskKey(a, b HistoryTaskKey) HistoryTaskKey {
	if a.Compare(b) < 0 {
		return a
	}
	return b
}

func MaxHistoryTaskKey(a, b HistoryTaskKey) HistoryTaskKey {
	if a.Compare(b) > 0 {
		return a
	}
	return b
}

func (a *WorkflowIdentifier) GetDomainID() string {
	return a.DomainID
}

func (a *WorkflowIdentifier) GetWorkflowID() string {
	return a.WorkflowID
}

func (a *WorkflowIdentifier) GetRunID() string {
	return a.RunID
}

func (a *WorkflowIdentifier) ByteSize() uint64 {
	return uint64(len(a.DomainID) + len(a.WorkflowID) + len(a.RunID))
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

func (a *TaskData) ByteSize() uint64 {
	return uint64(8 + 8 + 24) // time.Time is 24 bytes
}

// GetType returns the type of the activity task
func (a *ActivityTask) GetTaskType() int {
	return TransferTaskTypeActivityTask
}

func (a *ActivityTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (a *ActivityTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *ActivityTask) GetTaskList() string {
	return a.TaskList
}

func (a *ActivityTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *ActivityTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + uint64(len(a.TargetDomainID)) + uint64(len(a.TaskList)) + 8
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

func (a *ActivityTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("activity task is not replication task")
}

// GetType returns the type of the decision task
func (d *DecisionTask) GetTaskType() int {
	return TransferTaskTypeDecisionTask
}

func (d *DecisionTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (d *DecisionTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(d.TaskID)
}

func (d *DecisionTask) GetTaskList() string {
	return d.TaskList
}

func (d *DecisionTask) GetOriginalTaskList() string {
	return d.OriginalTaskList
}

func (d *DecisionTask) ByteSize() uint64 {
	return d.WorkflowIdentifier.ByteSize() + d.TaskData.ByteSize() + uint64(len(d.TargetDomainID)) + uint64(len(d.TaskList)) + uint64(len(d.OriginalTaskList)) + 8
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
		OriginalTaskList:    d.OriginalTaskList,
	}, nil
}

func (d *DecisionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("decision task is not timer task")
}

func (d *DecisionTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("decision task is not replication task")
}

// GetType returns the type of the record workflow started task
func (a *RecordWorkflowStartedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowStarted
}

func (a *RecordWorkflowStartedTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (a *RecordWorkflowStartedTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *RecordWorkflowStartedTask) GetTaskList() string {
	return a.TaskList
}

func (a *RecordWorkflowStartedTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *RecordWorkflowStartedTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + uint64(len(a.TaskList))
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
		TaskList:            a.TaskList,
	}, nil
}

func (a *RecordWorkflowStartedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record workflow started task is not timer task")
}

func (a *RecordWorkflowStartedTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("record workflow started task is not replication task")
}

// GetType returns the type of the ResetWorkflowTask
func (a *ResetWorkflowTask) GetTaskType() int {
	return TransferTaskTypeResetWorkflow
}

func (a *ResetWorkflowTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (a *ResetWorkflowTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *ResetWorkflowTask) GetTaskList() string {
	return a.TaskList
}

func (a *ResetWorkflowTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *ResetWorkflowTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + uint64(len(a.TaskList))
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
		TaskList:            a.TaskList,
	}, nil
}

func (a *ResetWorkflowTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("reset workflow task is not timer task")
}

func (a *ResetWorkflowTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("reset workflow task is not replication task")
}

// GetType returns the type of the close execution task
func (a *CloseExecutionTask) GetTaskType() int {
	return TransferTaskTypeCloseExecution
}

func (a *CloseExecutionTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (a *CloseExecutionTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *CloseExecutionTask) GetTaskList() string {
	return a.TaskList
}

func (a *CloseExecutionTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *CloseExecutionTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + uint64(len(a.TaskList))
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
		TaskList:            a.TaskList,
	}, nil
}

func (a *CloseExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("close execution task is not timer task")
}

func (a *CloseExecutionTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("close execution task is not replication task")
}

// GetType returns the type of the delete execution task
func (a *DeleteHistoryEventTask) GetTaskType() int {
	return TaskTypeDeleteHistoryEvent
}

func (a *DeleteHistoryEventTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (a *DeleteHistoryEventTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(a.VisibilityTimestamp, a.TaskID)
}

func (a *DeleteHistoryEventTask) GetTaskList() string {
	return a.TaskList
}

func (a *DeleteHistoryEventTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *DeleteHistoryEventTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + uint64(len(a.TaskList))
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
		TaskList:            a.TaskList,
	}, nil
}

func (a *DeleteHistoryEventTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("delete history event task is not replication task")
}

// GetType returns the type of the timer task
func (d *DecisionTimeoutTask) GetTaskType() int {
	return TaskTypeDecisionTimeout
}

func (d *DecisionTimeoutTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (d *DecisionTimeoutTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(d.VisibilityTimestamp, d.TaskID)
}

func (d *DecisionTimeoutTask) GetTaskList() string {
	return d.TaskList
}

func (d *DecisionTimeoutTask) GetOriginalTaskList() string {
	return d.TaskList
}

func (d *DecisionTimeoutTask) ByteSize() uint64 {
	return d.WorkflowIdentifier.ByteSize() + d.TaskData.ByteSize() + 8 + 8 + 8 + uint64(len(d.TaskList))
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
		TaskList:            d.TaskList,
	}, nil
}

func (d *DecisionTimeoutTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("decision timeout task is not replication task")
}

// GetType returns the type of the timer task
func (a *ActivityTimeoutTask) GetTaskType() int {
	return TaskTypeActivityTimeout
}

func (a *ActivityTimeoutTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (a *ActivityTimeoutTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(a.VisibilityTimestamp, a.TaskID)
}

func (a *ActivityTimeoutTask) GetTaskList() string {
	return a.TaskList
}

func (a *ActivityTimeoutTask) GetOriginalTaskList() string {
	return a.TaskList
}

func (a *ActivityTimeoutTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + 8 + 8 + 8 + uint64(len(a.TaskList))
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
		TaskList:            a.TaskList,
	}, nil
}

func (a *ActivityTimeoutTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("activity timeout task is not replication task")
}

// GetType returns the type of the timer task
func (u *UserTimerTask) GetTaskType() int {
	return TaskTypeUserTimer
}

func (u *UserTimerTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (u *UserTimerTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(u.VisibilityTimestamp, u.TaskID)
}

func (u *UserTimerTask) GetTaskList() string {
	return u.TaskList
}

func (u *UserTimerTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *UserTimerTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + 8 + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *UserTimerTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("user timer task is not replication task")
}

// GetType returns the type of the retry timer task
func (r *ActivityRetryTimerTask) GetTaskType() int {
	return TaskTypeActivityRetryTimer
}

func (r *ActivityRetryTimerTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (r *ActivityRetryTimerTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(r.VisibilityTimestamp, r.TaskID)
}

func (r *ActivityRetryTimerTask) GetTaskList() string {
	return r.TaskList
}

func (r *ActivityRetryTimerTask) GetOriginalTaskList() string {
	return r.TaskList
}

func (r *ActivityRetryTimerTask) ByteSize() uint64 {
	return r.WorkflowIdentifier.ByteSize() + r.TaskData.ByteSize() + 8 + 8 + uint64(len(r.TaskList))
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
		TaskList:            r.TaskList,
	}, nil
}

func (r *ActivityRetryTimerTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("activity retry timer task is not replication task")
}

// GetType returns the type of the retry timer task
func (r *WorkflowBackoffTimerTask) GetTaskType() int {
	return TaskTypeWorkflowBackoffTimer
}

func (r *WorkflowBackoffTimerTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (r *WorkflowBackoffTimerTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(r.VisibilityTimestamp, r.TaskID)
}

func (r *WorkflowBackoffTimerTask) GetTaskList() string {
	return r.TaskList
}

func (r *WorkflowBackoffTimerTask) GetOriginalTaskList() string {
	return r.TaskList
}

func (r *WorkflowBackoffTimerTask) ByteSize() uint64 {
	return r.WorkflowIdentifier.ByteSize() + r.TaskData.ByteSize() + 8 + uint64(len(r.TaskList))
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
		TaskList:            r.TaskList,
	}, nil
}

func (r *WorkflowBackoffTimerTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("workflow backoff timer task is not replication task")
}

// GetType returns the type of the timeout task.
func (u *WorkflowTimeoutTask) GetTaskType() int {
	return TaskTypeWorkflowTimeout
}

func (u *WorkflowTimeoutTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTimer
}

func (u *WorkflowTimeoutTask) GetTaskKey() HistoryTaskKey {
	return NewHistoryTaskKey(u.VisibilityTimestamp, u.TaskID)
}

func (u *WorkflowTimeoutTask) GetTaskList() string {
	return u.TaskList
}

func (u *WorkflowTimeoutTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *WorkflowTimeoutTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *WorkflowTimeoutTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("workflow timeout task is not replication task")
}

// GetType returns the type of the cancel transfer task
func (u *CancelExecutionTask) GetTaskType() int {
	return TransferTaskTypeCancelExecution
}

func (u *CancelExecutionTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *CancelExecutionTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *CancelExecutionTask) GetTaskList() string {
	return u.TaskList
}

func (u *CancelExecutionTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *CancelExecutionTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TargetDomainID)) + uint64(len(u.TargetWorkflowID)) + uint64(len(u.TargetRunID)) + 8 + 1 + uint64(len(u.TaskList))
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
		TaskList:                u.TaskList,
	}, nil
}

func (u *CancelExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("cancel execution task is not timer task")
}

func (u *CancelExecutionTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("cancel execution task is not replication task")
}

// GetType returns the type of the signal transfer task
func (u *SignalExecutionTask) GetTaskType() int {
	return TransferTaskTypeSignalExecution
}

func (u *SignalExecutionTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *SignalExecutionTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *SignalExecutionTask) GetTaskList() string {
	return u.TaskList
}

func (u *SignalExecutionTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *SignalExecutionTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TargetDomainID)) + uint64(len(u.TargetWorkflowID)) + uint64(len(u.TargetRunID)) + 8 + 1 + uint64(len(u.TaskList))
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
		TaskList:                u.TaskList,
	}, nil
}

func (u *SignalExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("signal execution task is not timer task")
}

func (u *SignalExecutionTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("signal execution task is not replication task")
}

// GetType returns the type of the record child execution completed task
func (u *RecordChildExecutionCompletedTask) GetTaskType() int {
	return TransferTaskTypeRecordChildExecutionCompleted
}

func (u *RecordChildExecutionCompletedTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *RecordChildExecutionCompletedTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *RecordChildExecutionCompletedTask) GetTaskList() string {
	return u.TaskList
}

func (u *RecordChildExecutionCompletedTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *RecordChildExecutionCompletedTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TargetDomainID)) + uint64(len(u.TargetWorkflowID)) + uint64(len(u.TargetRunID)) + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *RecordChildExecutionCompletedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record child execution completed task is not timer task")
}

func (u *RecordChildExecutionCompletedTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("record child execution completed task is not replication task")
}

// GetType returns the type of the upsert search attributes transfer task
func (u *UpsertWorkflowSearchAttributesTask) GetTaskType() int {
	return TransferTaskTypeUpsertWorkflowSearchAttributes
}

func (u *UpsertWorkflowSearchAttributesTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *UpsertWorkflowSearchAttributesTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *UpsertWorkflowSearchAttributesTask) GetTaskList() string {
	return u.TaskList
}

func (u *UpsertWorkflowSearchAttributesTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *UpsertWorkflowSearchAttributesTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *UpsertWorkflowSearchAttributesTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("upsert workflow search attributes task is not timer task")
}

func (u *UpsertWorkflowSearchAttributesTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("upsert workflow search attributes task is not replication task")
}

// GetType returns the type of the start child transfer task
func (u *StartChildExecutionTask) GetTaskType() int {
	return TransferTaskTypeStartChildExecution
}

func (u *StartChildExecutionTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *StartChildExecutionTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *StartChildExecutionTask) GetTaskList() string {
	return u.TaskList
}

func (u *StartChildExecutionTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *StartChildExecutionTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TargetDomainID)) + uint64(len(u.TargetWorkflowID)) + 8 + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *StartChildExecutionTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("start child execution task is not timer task")
}

func (u *StartChildExecutionTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("start child execution task is not replication task")
}

// GetType returns the type of the record workflow closed task
func (u *RecordWorkflowClosedTask) GetTaskType() int {
	return TransferTaskTypeRecordWorkflowClosed
}

func (u *RecordWorkflowClosedTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryTransfer
}

func (u *RecordWorkflowClosedTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(u.TaskID)
}

func (u *RecordWorkflowClosedTask) GetTaskList() string {
	return u.TaskList
}

func (u *RecordWorkflowClosedTask) GetOriginalTaskList() string {
	return u.TaskList
}

func (u *RecordWorkflowClosedTask) ByteSize() uint64 {
	return u.WorkflowIdentifier.ByteSize() + u.TaskData.ByteSize() + uint64(len(u.TaskList))
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
		TaskList:            u.TaskList,
	}, nil
}

func (u *RecordWorkflowClosedTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("record workflow closed task is not timer task")
}

func (u *RecordWorkflowClosedTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, fmt.Errorf("record workflow closed task is not replication task")
}

// GetType returns the type of the history replication task
func (a *HistoryReplicationTask) GetTaskType() int {
	return ReplicationTaskTypeHistory
}

func (a *HistoryReplicationTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryReplication
}

func (a *HistoryReplicationTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *HistoryReplicationTask) GetTaskList() string {
	return ""
}

func (a *HistoryReplicationTask) GetOriginalTaskList() string {
	return ""
}

func (a *HistoryReplicationTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + 8 + 8 + uint64(len(a.BranchToken)) + uint64(len(a.NewRunBranchToken))
}

func (a *HistoryReplicationTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("history replication task is not transfer task")
}

func (a *HistoryReplicationTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("history replication task is not timer task")
}

func (a *HistoryReplicationTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return &types.ReplicationTaskInfo{
		DomainID:     a.DomainID,
		WorkflowID:   a.WorkflowID,
		RunID:        a.RunID,
		TaskType:     ReplicationTaskTypeHistory,
		TaskID:       a.TaskID,
		Version:      a.Version,
		FirstEventID: a.FirstEventID,
		NextEventID:  a.NextEventID,
		ScheduledID:  constants.EmptyEventID,
	}, nil
}

// GetType returns the type of the sync activity task
func (a *SyncActivityTask) GetTaskType() int {
	return ReplicationTaskTypeSyncActivity
}

func (a *SyncActivityTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryReplication
}

func (a *SyncActivityTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *SyncActivityTask) GetTaskList() string {
	return ""
}

func (a *SyncActivityTask) GetOriginalTaskList() string {
	return ""
}

func (a *SyncActivityTask) ByteSize() uint64 {
	return a.WorkflowIdentifier.ByteSize() + a.TaskData.ByteSize() + 8
}

func (a *SyncActivityTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return &types.ReplicationTaskInfo{
		DomainID:     a.DomainID,
		WorkflowID:   a.WorkflowID,
		RunID:        a.RunID,
		TaskType:     ReplicationTaskTypeSyncActivity,
		TaskID:       a.TaskID,
		Version:      a.Version,
		FirstEventID: constants.EmptyEventID,
		NextEventID:  constants.EmptyEventID,
		ScheduledID:  a.ScheduledID,
	}, nil
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

func (a *FailoverMarkerTask) GetTaskCategory() HistoryTaskCategory {
	return HistoryTaskCategoryReplication
}

func (a *FailoverMarkerTask) GetTaskKey() HistoryTaskKey {
	return NewImmediateTaskKey(a.TaskID)
}

func (a *FailoverMarkerTask) GetTaskList() string {
	return ""
}

func (a *FailoverMarkerTask) GetOriginalTaskList() string {
	return ""
}

func (a *FailoverMarkerTask) ByteSize() uint64 {
	return uint64(len(a.DomainID)) + a.TaskData.ByteSize()
}

func (a *FailoverMarkerTask) ToTransferTaskInfo() (*TransferTaskInfo, error) {
	return nil, fmt.Errorf("failover marker task is not transfer task")
}

func (a *FailoverMarkerTask) ToTimerTaskInfo() (*TimerTaskInfo, error) {
	return nil, fmt.Errorf("failover marker task is not timer task")
}

func (a *FailoverMarkerTask) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return &types.ReplicationTaskInfo{
		DomainID:     a.DomainID,
		TaskType:     ReplicationTaskTypeFailoverMarker,
		TaskID:       a.TaskID,
		Version:      a.Version,
		FirstEventID: constants.EmptyEventID,
		NextEventID:  constants.EmptyEventID,
		ScheduledID:  constants.EmptyEventID,
	}, nil
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
