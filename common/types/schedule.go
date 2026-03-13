// Copyright (c) 2024 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// --- Enums ---

// ScheduleOverlapPolicy defines behavior when a scheduled run overlaps with a previous run.
type ScheduleOverlapPolicy int32

const (
	ScheduleOverlapPolicyInvalid           ScheduleOverlapPolicy = iota
	ScheduleOverlapPolicySkipNew                                 // Skip if previous still running
	ScheduleOverlapPolicyBuffer                                  // Buffer and execute sequentially
	ScheduleOverlapPolicyConcurrent                              // Allow concurrent execution
	ScheduleOverlapPolicyCancelPrevious                          // Cancel previous, start new
	ScheduleOverlapPolicyTerminatePrevious                       // Terminate previous, start new
)

func (e ScheduleOverlapPolicy) Ptr() *ScheduleOverlapPolicy { return &e }

func (e ScheduleOverlapPolicy) String() string {
	switch e {
	case ScheduleOverlapPolicyInvalid:
		return "INVALID"
	case ScheduleOverlapPolicySkipNew:
		return "SKIP_NEW"
	case ScheduleOverlapPolicyBuffer:
		return "BUFFER"
	case ScheduleOverlapPolicyConcurrent:
		return "CONCURRENT"
	case ScheduleOverlapPolicyCancelPrevious:
		return "CANCEL_PREVIOUS"
	case ScheduleOverlapPolicyTerminatePrevious:
		return "TERMINATE_PREVIOUS"
	}
	return fmt.Sprintf("ScheduleOverlapPolicy(%d)", int32(e))
}

func (e *ScheduleOverlapPolicy) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "INVALID":
		*e = ScheduleOverlapPolicyInvalid
	case "SKIP_NEW":
		*e = ScheduleOverlapPolicySkipNew
	case "BUFFER":
		*e = ScheduleOverlapPolicyBuffer
	case "CONCURRENT":
		*e = ScheduleOverlapPolicyConcurrent
	case "CANCEL_PREVIOUS":
		*e = ScheduleOverlapPolicyCancelPrevious
	case "TERMINATE_PREVIOUS":
		*e = ScheduleOverlapPolicyTerminatePrevious
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ScheduleOverlapPolicy", err)
		}
		*e = ScheduleOverlapPolicy(val)
	}
	return nil
}

func (e ScheduleOverlapPolicy) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

// ScheduleCatchUpPolicy defines how missed runs are handled on unpause or system recovery.
type ScheduleCatchUpPolicy int32

const (
	ScheduleCatchUpPolicyInvalid ScheduleCatchUpPolicy = iota
	ScheduleCatchUpPolicySkip                          // Skip all missed runs
	ScheduleCatchUpPolicyOne                           // Run only the most recent missed time
	ScheduleCatchUpPolicyAll                           // Run for each missed (up to window)
)

func (e ScheduleCatchUpPolicy) Ptr() *ScheduleCatchUpPolicy { return &e }

func (e ScheduleCatchUpPolicy) String() string {
	switch e {
	case ScheduleCatchUpPolicyInvalid:
		return "INVALID"
	case ScheduleCatchUpPolicySkip:
		return "SKIP"
	case ScheduleCatchUpPolicyOne:
		return "ONE"
	case ScheduleCatchUpPolicyAll:
		return "ALL"
	}
	return fmt.Sprintf("ScheduleCatchUpPolicy(%d)", int32(e))
}

func (e *ScheduleCatchUpPolicy) UnmarshalText(value []byte) error {
	switch s := strings.ToUpper(string(value)); s {
	case "INVALID":
		*e = ScheduleCatchUpPolicyInvalid
	case "SKIP":
		*e = ScheduleCatchUpPolicySkip
	case "ONE":
		*e = ScheduleCatchUpPolicyOne
	case "ALL":
		*e = ScheduleCatchUpPolicyAll
	default:
		val, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unknown enum value %q for %q: %v", s, "ScheduleCatchUpPolicy", err)
		}
		*e = ScheduleCatchUpPolicy(val)
	}
	return nil
}

func (e ScheduleCatchUpPolicy) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

// --- Core Types ---

// ScheduleSpec defines when a schedule should trigger.
type ScheduleSpec struct {
	CronExpression string        `json:"cronExpression,omitempty"`
	StartTime      time.Time     `json:"startTime,omitempty"`
	EndTime        time.Time     `json:"endTime,omitempty"`
	Jitter         time.Duration `json:"jitter,omitempty"`
}

func (v *ScheduleSpec) GetCronExpression() (o string) {
	if v != nil {
		return v.CronExpression
	}
	return
}

func (v *ScheduleSpec) GetStartTime() (o time.Time) {
	if v != nil {
		return v.StartTime
	}
	return
}

func (v *ScheduleSpec) GetEndTime() (o time.Time) {
	if v != nil {
		return v.EndTime
	}
	return
}

func (v *ScheduleSpec) GetJitter() (o time.Duration) {
	if v != nil {
		return v.Jitter
	}
	return
}

// StartWorkflowAction defines a workflow to start when the schedule triggers.
type StartWorkflowAction struct {
	WorkflowType                        *WorkflowType     `json:"workflowType,omitempty"`
	TaskList                            *TaskList         `json:"taskList,omitempty"`
	Input                               []byte            `json:"-"` // Potential PII
	WorkflowIDPrefix                    string            `json:"workflowIdPrefix,omitempty"`
	ExecutionStartToCloseTimeoutSeconds *int32            `json:"executionStartToCloseTimeoutSeconds,omitempty"`
	TaskStartToCloseTimeoutSeconds      *int32            `json:"taskStartToCloseTimeoutSeconds,omitempty"`
	RetryPolicy                         *RetryPolicy      `json:"retryPolicy,omitempty"`
	Memo                                *Memo             `json:"-"` // Filtering PII
	SearchAttributes                    *SearchAttributes `json:"-"` // Filtering PII
}

func (v *StartWorkflowAction) GetWorkflowType() *WorkflowType {
	if v != nil {
		return v.WorkflowType
	}
	return nil
}

func (v *StartWorkflowAction) GetTaskList() *TaskList {
	if v != nil {
		return v.TaskList
	}
	return nil
}

func (v *StartWorkflowAction) GetWorkflowIDPrefix() (o string) {
	if v != nil {
		return v.WorkflowIDPrefix
	}
	return
}

func (v *StartWorkflowAction) GetExecutionStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.ExecutionStartToCloseTimeoutSeconds != nil {
		return *v.ExecutionStartToCloseTimeoutSeconds
	}
	return
}

func (v *StartWorkflowAction) GetTaskStartToCloseTimeoutSeconds() (o int32) {
	if v != nil && v.TaskStartToCloseTimeoutSeconds != nil {
		return *v.TaskStartToCloseTimeoutSeconds
	}
	return
}

// ScheduleAction defines the action to take when the schedule triggers.
// Exactly one action field must be set.
type ScheduleAction struct {
	StartWorkflow *StartWorkflowAction `json:"startWorkflow,omitempty"`
}

func (v *ScheduleAction) GetStartWorkflow() *StartWorkflowAction {
	if v != nil {
		return v.StartWorkflow
	}
	return nil
}

// SchedulePolicies configures schedule behavior.
type SchedulePolicies struct {
	OverlapPolicy    ScheduleOverlapPolicy `json:"overlapPolicy,omitempty"`
	CatchUpPolicy    ScheduleCatchUpPolicy `json:"catchUpPolicy,omitempty"`
	CatchUpWindow    time.Duration         `json:"catchUpWindow,omitempty"`
	PauseOnFailure   bool                  `json:"pauseOnFailure,omitempty"`
	BufferLimit      int32                 `json:"bufferLimit,omitempty"`
	ConcurrencyLimit int32                 `json:"concurrencyLimit,omitempty"`
}

func (v *SchedulePolicies) GetOverlapPolicy() (o ScheduleOverlapPolicy) {
	if v != nil {
		return v.OverlapPolicy
	}
	return
}

func (v *SchedulePolicies) GetCatchUpPolicy() (o ScheduleCatchUpPolicy) {
	if v != nil {
		return v.CatchUpPolicy
	}
	return
}

func (v *SchedulePolicies) GetCatchUpWindow() (o time.Duration) {
	if v != nil {
		return v.CatchUpWindow
	}
	return
}

func (v *SchedulePolicies) GetPauseOnFailure() (o bool) {
	if v != nil {
		return v.PauseOnFailure
	}
	return
}

func (v *SchedulePolicies) GetBufferLimit() (o int32) {
	if v != nil {
		return v.BufferLimit
	}
	return
}

func (v *SchedulePolicies) GetConcurrencyLimit() (o int32) {
	if v != nil {
		return v.ConcurrencyLimit
	}
	return
}

// SchedulePauseInfo captures the state of a paused schedule (response-only, server-populated).
type SchedulePauseInfo struct {
	Reason   string    `json:"reason,omitempty"`
	PausedAt time.Time `json:"pausedAt,omitempty"`
	PausedBy string    `json:"pausedBy,omitempty"`
}

func (v *SchedulePauseInfo) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

func (v *SchedulePauseInfo) GetPausedAt() (o time.Time) {
	if v != nil {
		return v.PausedAt
	}
	return
}

func (v *SchedulePauseInfo) GetPausedBy() (o string) {
	if v != nil {
		return v.PausedBy
	}
	return
}

// ScheduleState represents the current runtime state of a schedule.
type ScheduleState struct {
	Paused    bool               `json:"paused,omitempty"`
	PauseInfo *SchedulePauseInfo `json:"pauseInfo,omitempty"`
}

func (v *ScheduleState) GetPaused() (o bool) {
	if v != nil {
		return v.Paused
	}
	return
}

func (v *ScheduleState) GetPauseInfo() *SchedulePauseInfo {
	if v != nil {
		return v.PauseInfo
	}
	return nil
}

// BackfillInfo tracks the progress of an ongoing backfill operation.
type BackfillInfo struct {
	BackfillID    string    `json:"backfillId,omitempty"`
	StartTime     time.Time `json:"startTime,omitempty"`
	EndTime       time.Time `json:"endTime,omitempty"`
	RunsCompleted int32     `json:"runsCompleted,omitempty"`
	RunsTotal     int32     `json:"runsTotal,omitempty"`
}

func (v *BackfillInfo) GetStartTime() (o time.Time) {
	if v != nil {
		return v.StartTime
	}
	return
}

func (v *BackfillInfo) GetEndTime() (o time.Time) {
	if v != nil {
		return v.EndTime
	}
	return
}

func (v *BackfillInfo) GetBackfillID() (o string) {
	if v != nil {
		return v.BackfillID
	}
	return
}

func (v *BackfillInfo) GetRunsCompleted() (o int32) {
	if v != nil {
		return v.RunsCompleted
	}
	return
}

func (v *BackfillInfo) GetRunsTotal() (o int32) {
	if v != nil {
		return v.RunsTotal
	}
	return
}

// ScheduleInfo provides runtime information about the schedule.
type ScheduleInfo struct {
	LastRunTime      time.Time       `json:"lastRunTime,omitempty"`
	NextRunTime      time.Time       `json:"nextRunTime,omitempty"`
	TotalRuns        int64           `json:"totalRuns,omitempty"`
	CreateTime       time.Time       `json:"createTime,omitempty"`
	LastUpdateTime   time.Time       `json:"lastUpdateTime,omitempty"`
	OngoingBackfills []*BackfillInfo `json:"ongoingBackfills,omitempty"`
}

func (v *ScheduleInfo) GetLastRunTime() (o time.Time) {
	if v != nil {
		return v.LastRunTime
	}
	return
}

func (v *ScheduleInfo) GetNextRunTime() (o time.Time) {
	if v != nil {
		return v.NextRunTime
	}
	return
}

func (v *ScheduleInfo) GetTotalRuns() (o int64) {
	if v != nil {
		return v.TotalRuns
	}
	return
}

func (v *ScheduleInfo) GetCreateTime() (o time.Time) {
	if v != nil {
		return v.CreateTime
	}
	return
}

func (v *ScheduleInfo) GetLastUpdateTime() (o time.Time) {
	if v != nil {
		return v.LastUpdateTime
	}
	return
}

func (v *ScheduleInfo) GetOngoingBackfills() (o []*BackfillInfo) {
	if v != nil {
		return v.OngoingBackfills
	}
	return
}

func (v *StartWorkflowAction) GetInput() (o []byte) {
	if v != nil {
		return v.Input
	}
	return
}

func (v *StartWorkflowAction) GetRetryPolicy() *RetryPolicy {
	if v != nil {
		return v.RetryPolicy
	}
	return nil
}

func (v *StartWorkflowAction) GetMemo() *Memo {
	if v != nil {
		return v.Memo
	}
	return nil
}

func (v *StartWorkflowAction) GetSearchAttributes() *SearchAttributes {
	if v != nil {
		return v.SearchAttributes
	}
	return nil
}
