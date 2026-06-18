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

	"github.com/uber/cadence/common/definition"
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

	// Metric name strings emitted via tally.Scope (workflow.GetMetricsScope).
	SchedulerSignalReceivedCountPerDomain = "scheduler_signal_received_count_per_domain"
	SchedulerMissedFiredCountPerDomain    = "scheduler_missed_fired_count_per_domain"
	SchedulerMissedSkippedCountPerDomain  = "scheduler_missed_skipped_count_per_domain"
	SchedulerBackfillFiredCountPerDomain  = "scheduler_backfill_fired_count_per_domain"
	// SchedulerBackfillRejectedCountPerDomain counts backfill signals dropped by
	// the workflow after the RPC has already returned success. Tagged with the
	// rejection reason (invalid_range, queue_full).
	SchedulerBackfillRejectedCountPerDomain = "scheduler_backfill_rejected_count_per_domain"
	SchedulerContinueAsNewCountPerDomain    = "scheduler_continue_as_new_count_per_domain"
	// SchedulerBufferOverflowCountPerDomain measures fires dropped because the
	// BUFFER overlap policy queue is full. Tagged with the drop reason so
	// operators can distinguish drops driven by the user's buffer_limit
	// (reason=user_limit) from drops driven by the server-side ceiling that
	// protects ContinueAsNew payload size (reason=system_limit).
	SchedulerBufferOverflowCountPerDomain = "scheduler_buffer_overflow_count_per_domain"

	// Tag key strings for scheduler workflow metrics.
	SignalTypeTag    = "signal_type"
	CatchUpPolicyTag = "catch_up_policy"
	ReasonTag        = "reason"

	// ContinueAsNew reason tag values for scheduler_continue_as_new_count metric.
	ContinueAsNewReasonMissedRun    = "missed_run"
	ContinueAsNewReasonBackfill     = "back_fill"
	ContinueAsNewReasonBufferDrain  = "buffer_drain"
	ContinueAsNewReasonSignal       = "signal"
	ContinueAsNewReasonIterationCap = "iteration_cap"

	// Buffer overflow reason tag values for scheduler_buffer_overflow_count metric.
	// Distinguishes drops driven by the user's buffer_limit from drops driven by
	// the server-side cap that protects ContinueAsNew payload size.
	BufferOverflowReasonUserLimit   = "user_limit"
	BufferOverflowReasonSystemLimit = "system_limit"

	// Reason tag values for scheduler_backfill_rejected_count_per_domain.
	BackfillRejectedReasonInvalidRange = "invalid_range"
	BackfillRejectedReasonQueueFull    = "queue_full"
	// BackfillRejectedReasonDuplicateID is emitted when a backfill signal
	// arrives with a non-empty BackfillID that matches an already-pending
	// backfill. The signal is absorbed for idempotent-retry safety rather
	// than enqueued a second time.
	BackfillRejectedReasonDuplicateID = "duplicate_id"

	// MaxBufferedFiresSystemLimit caps the BUFFER overlap policy queue regardless
	// of buffer_limit (including buffer_limit=0 meaning unlimited). It bounds the
	// ContinueAsNew payload size: each BufferedFire is ~50 bytes JSON, so 1000
	// entries stays well within the workflow input size limit.
	MaxBufferedFiresSystemLimit = 1000

	// MaxConcurrencyLimitSystemLimit caps ConcurrencyLimit for the bounded CONCURRENT
	// overlap policy regardless of the user-configured value. It bounds the
	// RunningWorkflows slice carried in ContinueAsNew payload: each RunningWorkflowInfo
	// is ~110 bytes JSON, so 1000 entries adds ~107KB — well within the 2MB hard limit
	// and leaving headroom for the rest of the workflow state. Exceeding the 2MB limit
	// causes Cadence to fail the workflow entirely with no graceful degradation.
	MaxConcurrencyLimitSystemLimit = 1000

	// signal_type tag values for scheduler_signal_received_count metric.
	signalTypeTagPause    = "pause"
	signalTypeTagUnpause  = "unpause"
	signalTypeTagUpdate   = "update"
	signalTypeTagBackfill = "backfill"
	signalTypeTagDelete   = "delete"

	// Search attribute keys set on target workflows started by the scheduler.
	// The string values are defined in common/definition to make them part of
	// the default indexed keys for all visibility backends.
	SearchAttrScheduleID   = definition.CadenceScheduleID
	SearchAttrScheduleTime = definition.CadenceScheduleTime
	SearchAttrIsBackfill   = definition.CadenceScheduleIsBackfill
	SearchAttrBackfillID   = definition.CadenceScheduleBackfillID

	// Search attribute keys set on the scheduler workflow itself for ListSchedules.
	// CadenceScheduleState is a Keyword SA holding the current lifecycle state
	// ("active" or "paused"). Modeled as a string rather than a boolean so it can
	// be extended to additional states (e.g. "expired") without introducing new
	// search attributes. "Deleted" is not a value because a deleted schedule's
	// workflow is closed and filtered by workflow status instead.
	SearchAttrScheduleState = definition.CadenceScheduleState
	// CadenceScheduleCron holds the current cron expression so ListSchedules
	// can display it without querying each scheduler workflow. Refreshed on
	// workflow start (including after ContinueAsNew triggered by UpdateSchedule).
	SearchAttrScheduleCron = definition.CadenceScheduleCron
	// CadenceScheduleWorkflowType holds the target workflow type name that the
	// schedule starts on each fire. Same refresh semantics as the cron SA.
	SearchAttrScheduleWorkflowType = definition.CadenceScheduleWorkflowType

	ScheduleStateActive = "active"
	ScheduleStatePaused = "paused"

	maxIterationsBeforeContinueAsNew = 500
	// maxActivitiesPerExecution caps the total number of activity fires the drain
	// coroutine may dispatch before triggering ContinueAsNew. It also bounds the
	// per-execution work of catch-up and backfill processing.
	maxActivitiesPerExecution = 500
	maxPendingBackfills       = 10

	// maxBackfillRunsTotalCount caps the cron walk that populates
	// BackfillRequest.RunsTotal. When a backfill range produces more fires
	// than this, RunsTotal is set to the cap as a lower bound.
	maxBackfillRunsTotalCount = 100000

	// defaultCatchUpWindow is applied to CatchUpWindow when the field is unset
	// (zero) and the catch-up policy consults the window (ONE or ALL).
	// One year is large enough to cover most outage scenarios while still
	// bounding catch-up so a long-dormant schedule doesn't fire thousands of
	// times on restart.
	defaultCatchUpWindow = 365 * 24 * time.Hour

	localActivityScheduleToCloseTimeout = 60 * time.Second
	localActivityMaxRetries             = 3
	localActivityRetryInitialInterval   = time.Second
	localActivityRetryMaxInterval       = 10 * time.Second
)

// SchedulerWorkflowInput is the input to the scheduler workflow.
// It carries the schedule definition and any prior state (for ContinueAsNew).
type SchedulerWorkflowInput struct {
	Domain           string                  `json:"domain"`
	ScheduleID       string                  `json:"scheduleId"`
	Spec             types.ScheduleSpec      `json:"spec"`
	Action           types.ScheduleAction    `json:"action"`
	Policies         types.SchedulePolicies  `json:"policies"`
	SearchAttributes *types.SearchAttributes `json:"searchAttributes,omitempty"`
	Memo             *types.Memo             `json:"memo,omitempty"`
	State            SchedulerWorkflowState  `json:"state"`
}

// SchedulerWorkflowState is the mutable runtime state that survives ContinueAsNew.
type SchedulerWorkflowState struct {
	Paused            bool              `json:"paused"`
	PauseReason       string            `json:"pauseReason,omitempty"`
	PausedBy          string            `json:"pausedBy,omitempty"`
	Deleted           bool              `json:"-"`                           // transient flag, not persisted across ContinueAsNew
	LastRunTime       time.Time         `json:"lastRunTime,omitempty"`       // most recent scheduled run time the workflow has processed
	LastProcessedTime time.Time         `json:"lastProcessedTime,omitempty"` // catch-up watermark: latest missed fire we've processed (fired or skipped)
	NextRunTime       time.Time         `json:"nextRunTime,omitempty"`
	TotalRuns         int64             `json:"totalRuns"`
	MissedRuns        int64             `json:"missedRuns"`
	SkippedRuns       int64             `json:"skippedRuns"`
	Iterations        int               `json:"iterations"`
	PendingBackfills  []BackfillRequest `json:"pendingBackfills,omitempty"`
	// BufferedFires holds fires queued for sequential execution under the BUFFER
	// overlap policy. Fires are appended when the previous target workflow is
	// still running at fire time and drained in FIFO order on subsequent
	// opportunities (timer wakeups, signal wakeups). Persisted across
	// ContinueAsNew so buffered work isn't lost on workflow recycling.
	BufferedFires []BufferedFire `json:"bufferedFires,omitempty"`
	// LastStartedWorkflow tracks the most recently started target workflow so
	// the overlap policy can check whether it is still running before starting
	// the next one. Nil when no workflow has been started yet.
	LastStartedWorkflow *RunningWorkflowInfo `json:"lastStartedWorkflow,omitempty"`
	// RunningWorkflows holds in-flight target workflows under bounded CONCURRENT
	// (ConcurrencyLimit > 0); completed entries are pruned by the activity on each fire.
	RunningWorkflows []RunningWorkflowInfo `json:"runningWorkflows,omitempty"`
	// UnpauseCatchUpPolicy is a one-shot override set by UnpauseSignal.CatchUpPolicy.
	// processMissedRunsAt prefers this over input.Policies.CatchUpPolicy for the first
	// catch-up pass after an unpause, then clears it. Invalid (zero) means no override.
	UnpauseCatchUpPolicy types.ScheduleCatchUpPolicy `json:"unpauseCatchUpPolicy,omitempty"`
	// CreateTime is the wall-clock time when the schedule was first created.
	// Set once on the first workflow execution and preserved across ContinueAsNew.
	CreateTime time.Time `json:"createTime,omitempty"`
	// LastUpdateTime is the wall-clock time of the most recent UpdateSchedule that
	// changed at least one field. Also set to CreateTime at schedule creation.
	LastUpdateTime time.Time `json:"lastUpdateTime,omitempty"`
	// PausedAt is the wall-clock time when the schedule was most recently paused.
	// Zero when the schedule is not paused (or was never paused).
	PausedAt time.Time `json:"pausedAt,omitempty"`
}

// BufferedFire is a schedule fire queued for sequential execution by the BUFFER
// overlap policy. ScheduledTime and TriggerSource are preserved so the deferred
// start uses the same WorkflowID and RequestID it would have used at fire time.
// OverlapPolicy is the overlap policy in effect for this fire (schedule default or
// a backfill override). Zero (INVALID) means inherit input.Policies.OverlapPolicy
// for compatibility with older persisted workflow state.
type BufferedFire struct {
	ScheduledTime time.Time                   `json:"scheduledTime"`
	TriggerSource TriggerSource               `json:"triggerSource"`
	OverlapPolicy types.ScheduleOverlapPolicy `json:"overlapPolicy,omitempty"`
	// BackfillID is set when TriggerSource is backfill so BUFFER drains stamp the same SA.
	BackfillID string `json:"backfillId,omitempty"`
}

// RunningWorkflowInfo identifies a target workflow started by the scheduler,
// used for overlap policy checks.
type RunningWorkflowInfo struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
}

// BackfillRequest is a queued backfill that persists across ContinueAsNew.
type BackfillRequest struct {
	StartTime     time.Time                   `json:"startTime"`
	EndTime       time.Time                   `json:"endTime"`
	OverlapPolicy types.ScheduleOverlapPolicy `json:"overlapPolicy"`
	BackfillID    string                      `json:"backfillId,omitempty"`
	// RunsTotal is the number of cron fires within (StartTime, EndTime] at the
	// time the backfill was first processed. Populated lazily by processBackfills
	// using the workflow's currently parsed cron. When the range exceeds
	// maxBackfillRunsTotalCount, RunsTotal is set to that cap as a lower bound;
	// consumers may observe RunsCompleted approach or exceed RunsTotal before
	// the backfill finishes draining.
	RunsTotal int32 `json:"runsTotal,omitempty"`
	// RunsCompleted is the number of fires from this backfill that
	// processBackfills has handed off to processScheduleFire — counted whether
	// the fire started, was skipped under the overlap policy, or was deferred
	// into the BUFFER queue.
	RunsCompleted int32 `json:"runsCompleted,omitempty"`
	// RunsTotalComputed records whether RunsTotal has been populated, so
	// processBackfills does not re-walk the cron on every batch and an empty
	// range (RunsTotal == 0) is not confused with "not yet computed".
	RunsTotalComputed bool `json:"runsTotalComputed,omitempty"`
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
	Spec             *types.ScheduleSpec     `json:"spec,omitempty"`
	Action           *types.ScheduleAction   `json:"action,omitempty"`
	Policies         *types.SchedulePolicies `json:"policies,omitempty"`
	SearchAttributes *types.SearchAttributes `json:"searchAttributes,omitempty"`
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
	ScheduleID       string                  `json:"scheduleId"`
	Domain           string                  `json:"domain"`
	Spec             types.ScheduleSpec      `json:"spec"`
	Action           types.ScheduleAction    `json:"action"`
	Policies         types.SchedulePolicies  `json:"policies"`
	Paused           bool                    `json:"paused"`
	PauseReason      string                  `json:"pauseReason,omitempty"`
	PausedBy         string                  `json:"pausedBy,omitempty"`
	PausedAt         time.Time               `json:"pausedAt,omitempty"`
	LastRunTime      time.Time               `json:"lastRunTime,omitempty"`
	NextRunTime      time.Time               `json:"nextRunTime,omitempty"`
	TotalRuns        int64                   `json:"totalRuns"`
	MissedRuns       int64                   `json:"missedRuns"`
	SkippedRuns      int64                   `json:"skippedRuns"`
	CreateTime       time.Time               `json:"createTime,omitempty"`
	LastUpdateTime   time.Time               `json:"lastUpdateTime,omitempty"`
	Memo             *types.Memo             `json:"memo,omitempty"`
	SearchAttributes *types.SearchAttributes `json:"searchAttributes,omitempty"`
	// OngoingBackfills mirrors SchedulerWorkflowState.PendingBackfills at the
	// time of the describe query.
	OngoingBackfills []types.BackfillInfo `json:"ongoingBackfills,omitempty"`
}

// TriggerSource identifies what caused a schedule fire, used to differentiate
// RequestIDs so that e.g. a backfill for the same timestamp as a normal fire
// does not collide in server-side deduplication.
type TriggerSource string

const (
	TriggerSourceSchedule TriggerSource = "schedule"
	TriggerSourceBackfill TriggerSource = "backfill"
)

// fireOutcome is the result of attempting to fire a single schedule run. It
// tells the workflow whether the fire was processed to completion or was
// deferred by the BUFFER overlap policy and should be re-attempted later.
type fireOutcome int

const (
	// fireOutcomeDone means the fire was processed to completion (started, skipped,
	// cancelled/terminated-then-started, already-running, or errored-and-logged).
	fireOutcomeDone fireOutcome = iota
	// fireOutcomeBuffered means the BUFFER overlap policy deferred the fire
	// because the previous target workflow is still running.
	fireOutcomeBuffered
)

// ProcessFireRequest is the input to processScheduleFireActivity. It contains
// everything the activity needs to resolve the overlap policy and start the
// target workflow. All side effects (describe, cancel, terminate, start) happen
// inside this single activity so the workflow history stays stable when the
// overlap logic evolves.
type ProcessFireRequest struct {
	Domain              string                      `json:"domain"`
	ScheduleID          string                      `json:"scheduleId"`
	Action              types.StartWorkflowAction   `json:"action"`
	ScheduledTime       time.Time                   `json:"scheduledTime"`
	TriggerSource       TriggerSource               `json:"triggerSource"`
	OverlapPolicy       types.ScheduleOverlapPolicy `json:"overlapPolicy"`
	LastStartedWorkflow *RunningWorkflowInfo        `json:"lastStartedWorkflow,omitempty"`
	// ConcurrencyLimit mirrors SchedulePolicies.ConcurrencyLimit:
	//   0 = unlimited (use server default)
	//   N = capped at N concurrent runs
	ConcurrencyLimit int32 `json:"concurrencyLimit,omitempty"`
	// RunningWorkflows is the current in-flight set from workflow state; used
	// only when OverlapPolicy==CONCURRENT and ConcurrencyLimit > 0.
	RunningWorkflows []RunningWorkflowInfo `json:"runningWorkflows,omitempty"`
	// BackfillID is non-empty only for fires driven by a schedule backfill (matches RPC BackfillID).
	BackfillID string `json:"backfillId,omitempty"`
}

// ProcessFireResult is the output of processScheduleFireActivity. The workflow
// applies these counters and tracking info to its state after the activity returns.
type ProcessFireResult struct {
	StartedWorkflow *RunningWorkflowInfo `json:"startedWorkflow,omitempty"`
	TotalDelta      int64                `json:"totalDelta"`
	SkippedDelta    int64                `json:"skippedDelta"`
	// Buffered is true when the BUFFER overlap policy deferred this fire
	// because the previous target workflow was still running. The workflow
	// appends the fire to state.BufferedFires and retries draining on the
	// next loop iteration.
	Buffered bool `json:"buffered,omitempty"`
	// ActiveWorkflows is the updated in-flight set for bounded CONCURRENT; the workflow
	// replaces state.RunningWorkflows with it after each fire. Nil for all other policies.
	ActiveWorkflows []RunningWorkflowInfo `json:"activeWorkflows,omitempty"`
}
