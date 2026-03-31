// Copyright (c) 2026 Uber Technologies, Inc.
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

package scheduler

import (
	"time"

	"github.com/uber/cadence/common/types"
)

const (
	WorkflowTypeName = "cadence-scheduler"
	TaskListName     = "cadence-scheduler"

	SignalNamePause    = "scheduler-pause"
	SignalNameUnpause  = "scheduler-unpause"
	SignalNameUpdate   = "scheduler-update"
	SignalNameBackfill = "scheduler-backfill"
	SignalNameDelete   = "scheduler-delete"

	QueryTypeDescribe = "scheduler-describe"

	maxIterationsBeforeContinueAsNew = 500
	maxCatchUpFiresPerExecution      = 10
	maxBackfillFiresPerExecution     = 10
	maxPendingBackfills              = 10

	localActivityScheduleToCloseTimeout = 60 * time.Second
	localActivityMaxRetries             = 3
	localActivityRetryInitialInterval   = time.Second
	localActivityRetryMaxInterval       = 10 * time.Second
)

// SchedulerWorkflowInput is the input to the scheduler workflow.
// It carries the schedule definition and any prior state (for ContinueAsNew).
type SchedulerWorkflowInput struct {
	Domain     string                 `json:"domain"`
	ScheduleID string                 `json:"scheduleId"`
	Spec       types.ScheduleSpec     `json:"spec"`
	Action     types.ScheduleAction   `json:"action"`
	Policies   types.SchedulePolicies `json:"policies"`
	State      SchedulerWorkflowState `json:"state"`
}

// SchedulerWorkflowState is the mutable runtime state that survives ContinueAsNew.
type SchedulerWorkflowState struct {
	Paused            bool              `json:"paused"`
	PauseReason       string            `json:"pauseReason,omitempty"`
	PausedBy          string            `json:"pausedBy,omitempty"`
	Deleted           bool              `json:"-"`                           // transient flag, not persisted across ContinueAsNew
	LastRunTime       time.Time         `json:"lastRunTime,omitempty"`       // last time a workflow was actually started
	LastProcessedTime time.Time         `json:"lastProcessedTime,omitempty"` // catch-up watermark: latest missed fire we've processed (fired or skipped)
	NextRunTime       time.Time         `json:"nextRunTime,omitempty"`
	TotalRuns         int64             `json:"totalRuns"`
	MissedRuns        int64             `json:"missedRuns"`
	SkippedRuns       int64             `json:"skippedRuns"`
	Iterations        int               `json:"iterations"`
	BufferedRuns      int               `json:"bufferedRuns"`
	PendingBackfills  []BackfillRequest `json:"pendingBackfills,omitempty"`
}

// BackfillRequest is a queued backfill that persists across ContinueAsNew.
type BackfillRequest struct {
	StartTime     time.Time                   `json:"startTime"`
	EndTime       time.Time                   `json:"endTime"`
	OverlapPolicy types.ScheduleOverlapPolicy `json:"overlapPolicy"`
	BackfillID    string                      `json:"backfillId,omitempty"`
}

// PauseSignal is the payload sent with a pause signal.
type PauseSignal struct {
	Reason   string `json:"reason,omitempty"`
	PausedBy string `json:"pausedBy,omitempty"`
}

// UnpauseSignal is the payload sent with an unpause signal.
type UnpauseSignal struct {
	Reason        string                      `json:"reason,omitempty"`
	CatchUpPolicy types.ScheduleCatchUpPolicy `json:"catchUpPolicy,omitempty"`
}

// UpdateSignal is the payload sent with an update signal.
type UpdateSignal struct {
	Spec     *types.ScheduleSpec     `json:"spec,omitempty"`
	Action   *types.ScheduleAction   `json:"action,omitempty"`
	Policies *types.SchedulePolicies `json:"policies,omitempty"`
}

// BackfillSignal is the payload sent with a backfill signal.
type BackfillSignal struct {
	StartTime     time.Time                   `json:"startTime"`
	EndTime       time.Time                   `json:"endTime"`
	OverlapPolicy types.ScheduleOverlapPolicy `json:"overlapPolicy"`
	BackfillID    string                      `json:"backfillId,omitempty"`
}

// ScheduleDescription is the query result returned by the describe query handler.
// It provides a snapshot of the schedule's current configuration and runtime state.
type ScheduleDescription struct {
	ScheduleID  string                 `json:"scheduleId"`
	Domain      string                 `json:"domain"`
	Spec        types.ScheduleSpec     `json:"spec"`
	Action      types.ScheduleAction   `json:"action"`
	Policies    types.SchedulePolicies `json:"policies"`
	Paused      bool                   `json:"paused"`
	PauseReason string                 `json:"pauseReason,omitempty"`
	PausedBy    string                 `json:"pausedBy,omitempty"`
	LastRunTime time.Time              `json:"lastRunTime,omitempty"`
	NextRunTime time.Time              `json:"nextRunTime,omitempty"`
	TotalRuns   int64                  `json:"totalRuns"`
	MissedRuns  int64                  `json:"missedRuns"`
	SkippedRuns int64                  `json:"skippedRuns"`
}

// TriggerSource identifies what caused a schedule fire, used to differentiate
// RequestIDs so that e.g. a backfill for the same timestamp as a normal fire
// does not collide in server-side deduplication.
type TriggerSource string

const (
	TriggerSourceSchedule TriggerSource = "schedule"
	TriggerSourceBackfill TriggerSource = "backfill"
)

// StartWorkflowRequest is the input to the start-workflow activity.
type StartWorkflowRequest struct {
	Domain        string                    `json:"domain"`
	ScheduleID    string                    `json:"scheduleId"`
	Action        types.StartWorkflowAction `json:"action"`
	ScheduledTime time.Time                 `json:"scheduledTime"`
	TriggerSource TriggerSource             `json:"triggerSource"`
}

// StartWorkflowResult is the output of the start-workflow activity.
type StartWorkflowResult struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
	Started    bool   `json:"started"`
	Skipped    bool   `json:"skipped"`
}
