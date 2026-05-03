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
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/types"
)

// signalChannels bundles the signal channels used by the scheduler workflow
type signalChannels struct {
	pause    workflow.Channel
	unpause  workflow.Channel
	update   workflow.Channel
	backfill workflow.Channel
	delete   workflow.Channel
}

// SchedulerWorkflow is a long-running workflow that manages a single schedule.
// It computes the next fire time from the cron expression, waits via a timer,
// and dispatches the configured action. Signals control pause/unpause, update,
// backfill, and deletion.
//
// The main loop follows a state-machine pattern: all inputs (signals and timer)
// uniformly mutate state, and then a single decision point inspects the resulting
// state to determine what to do next. ContinueAsNew is triggered on any
// state-changing signal (pause, unpause, update) so the new execution's input
// is always the authoritative source of truth.
func SchedulerWorkflow(ctx workflow.Context, input SchedulerWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("scheduler workflow started",
		zap.String("domain", input.Domain),
		zap.String("scheduleId", input.ScheduleID),
		zap.Bool("paused", input.State.Paused),
	)

	scope := workflow.GetMetricsScope(ctx).Tagged(map[string]string{"domain": input.Domain})

	state := &input.State

	err := workflow.SetQueryHandler(ctx, QueryTypeDescribe, func() (*ScheduleDescription, error) {
		return buildScheduleDescription(&input, state), nil
	})
	if err != nil {
		return fmt.Errorf("failed to register query handler: %w", err)
	}

	// Re-upsert search attributes on every execution (including after ContinueAsNew).
	// Values set via UpsertSearchAttributes in a prior execution are not automatically
	// carried over, so we must refresh them here to keep ListSchedules visibility
	// results in sync with the current state/spec/action. UpdateSchedule triggers
	// ContinueAsNew, so the new cron and workflow type land here on the next start.
	if err := workflow.UpsertSearchAttributes(ctx, buildScheduleSearchAttributes(&input, state)); err != nil {
		logger.Warn("failed to upsert schedule search attributes", zap.Error(err))
	}

	chs := signalChannels{
		pause:    workflow.GetSignalChannel(ctx, SignalNamePause),
		unpause:  workflow.GetSignalChannel(ctx, SignalNameUnpause),
		update:   workflow.GetSignalChannel(ctx, SignalNameUpdate),
		backfill: workflow.GetSignalChannel(ctx, SignalNameBackfill),
		delete:   workflow.GetSignalChannel(ctx, SignalNameDelete),
	}

	sched, err := cron.ParseStandard(input.Spec.CronExpression)
	if err != nil {
		logger.Error("invalid cron expression, terminating", zap.String("cron", input.Spec.CronExpression), zap.Error(err))
		return fmt.Errorf("invalid cron expression %q: %w", input.Spec.CronExpression, err)
	}

	// Drain fires buffered by the BUFFER overlap policy in the previous
	// execution before processing missed runs. Buffered fires are older than
	// anything processMissedRuns can compute, so draining them first preserves
	// FIFO order across ContinueAsNew. If the queue is large, drain in batches
	// (at most maxDrainFiresPerExecution per execution) so a single decision
	// task doesn't time out.
	if !state.Paused {
		if moreToDrain := drainBufferedFires(ctx, logger, &input, state); moreToDrain {
			return safeContinueAsNew(ctx, logger, scope, ContinueAsNewReasonBufferDrain, chs.delete, input, state)
		}
	}

	// On the first iteration (after ContinueAsNew or fresh start), check for
	// fires that were missed during the transition gap or prior pause period.
	// Subsequent iterations don't need this because the timer handles fire times.
	// If more missed fires remain beyond the per-execution cap, ContinueAsNew
	// immediately so each batch runs in its own decision task.
	if moreMissed := processMissedRuns(ctx, logger, scope, sched, &input, state); moreMissed {
		return safeContinueAsNew(ctx, logger, scope, ContinueAsNewReasonMissedRun, chs.delete, input, state)
	}

	// Process any pending backfill requests carried over from a previous execution.
	if moreBackfills := processBackfills(ctx, logger, scope, sched, &input, state); moreBackfills {
		return safeContinueAsNew(ctx, logger, scope, ContinueAsNewReasonBackfill, chs.delete, input, state)
	}

	for {
		state.Iterations++

		// Set up timer only when not paused. When paused, applyAllInputs
		// blocks on signals alone until an unpause or delete arrives.
		var timerFuture workflow.Future
		var timerCancel func()
		if !state.Paused {
			now := workflow.Now(ctx)
			nextRun := computeNextRunTime(sched, now, input.Spec)
			if nextRun.IsZero() {
				logger.Info("schedule has no more runs (past end time), completing")
				return nil
			}
			state.NextRunTime = nextRun

			dur := nextRun.Sub(now)
			if dur < 0 {
				dur = 0
			}
			var timerCtx workflow.Context
			timerCtx, timerCancel = workflow.WithCancel(ctx)
			timerFuture = workflow.NewTimer(timerCtx, dur)
		}

		previousPaused := state.Paused
		changed, timerFired := applyAllInputs(ctx, logger, scope, timerFuture, chs, state, &input)

		if timerCancel != nil {
			timerCancel()
		}

		if state.Paused != previousPaused {
			if err := workflow.UpsertSearchAttributes(ctx, map[string]interface{}{
				SearchAttrScheduleState: scheduleStateFromPaused(state.Paused),
			}); err != nil {
				logger.Warn("failed to upsert schedule state search attribute", zap.Error(err))
			}
		}
		// Note: cron and workflow type search attributes are refreshed at the top
		// of the next workflow execution after UpdateSchedule triggers ContinueAsNew,
		// so no inline upsert is needed here.

		// Deleted schedules terminate the workflow here. Any further signals
		// (pause, unpause, update, backfill) sent after this point fail with
		// EntityNotExistsError at the RPC layer because the workflow is closed;
		// the frontend normalizes that to a user-friendly "schedule not found".
		if state.Deleted {
			logger.Info("schedule deleted")
			return nil
		}

		// Drain BUFFER-overlap fires before handling the current fire so the
		// queue is processed in FIFO order. If the cap is hit, ContinueAsNew
		// so the next batch runs in a fresh decision task.
		if !state.Paused {
			if moreToDrain := drainBufferedFires(ctx, logger, &input, state); moreToDrain {
				return safeContinueAsNew(ctx, logger, scope, ContinueAsNewReasonBufferDrain, chs.delete, input, state)
			}
		}

		if timerFired && !state.Paused {
			processScheduleFire(ctx, logger, scope, &input, state, state.NextRunTime, TriggerSourceSchedule, input.Policies.OverlapPolicy)
		}

		if changed || state.Iterations >= maxIterationsBeforeContinueAsNew {
			reason := ContinueAsNewReasonSignal
			if !changed {
				reason = ContinueAsNewReasonIterationCap
			}
			return safeContinueAsNew(ctx, logger, scope, reason, chs.delete, input, state)
		}
	}
}

// applyAllInputs blocks until at least one input (signal or timer) arrives,
// processes it, then drains any remaining buffered signals.
// Signals and the timer are treated uniformly: each mutates state without
// triggering side effects (no timer cancellation, no ContinueAsNew).
// Returns (stateChanged, timerFired): stateChanged is true if a state-changing
// signal (pause, unpause, update) was received; timerFired is true if the timer
// completed successfully.
func applyAllInputs(
	ctx workflow.Context,
	logger *zap.Logger,
	scope tally.Scope,
	timerFuture workflow.Future,
	chs signalChannels,
	state *SchedulerWorkflowState,
	input *SchedulerWorkflowInput,
) (bool, bool) {
	selector := workflow.NewSelector(ctx)
	stateChanged := false

	timerFired := false
	if timerFuture != nil {
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			if f.Get(ctx, nil) != nil {
				return
			}
			timerFired = true
		})
	}

	selector.AddReceive(chs.pause, func(c workflow.Channel, more bool) {
		var sig PauseSignal
		c.Receive(ctx, &sig)
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagPause}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handlePause(logger, sig, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.unpause, func(c workflow.Channel, more bool) {
		var sig UnpauseSignal
		c.Receive(ctx, &sig)
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagUnpause}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleUnpause(logger, sig, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.update, func(c workflow.Channel, more bool) {
		var sig UpdateSignal
		c.Receive(ctx, &sig)
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagUpdate}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleUpdate(logger, sig, input, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.backfill, func(c workflow.Channel, more bool) {
		var sig BackfillSignal
		c.Receive(ctx, &sig)
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagBackfill}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleBackfill(logger, sig, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.delete, func(c workflow.Channel, more bool) {
		c.Receive(ctx, nil)
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagDelete}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		state.Deleted = true
	})

	selector.Select(ctx)

	if drainBufferedSignals(logger, scope, chs, state, input) {
		stateChanged = true
	}

	return stateChanged, timerFired
}

// drainBufferedSignals processes any remaining buffered signals without blocking.
// Delete signals are checked first to prevent signal loss across ContinueAsNew boundaries.
// Returns true if a state-changing signal was found.
func drainBufferedSignals(
	logger *zap.Logger,
	scope tally.Scope,
	chs signalChannels,
	state *SchedulerWorkflowState,
	input *SchedulerWorkflowInput,
) bool {
	if chs.delete.ReceiveAsync(nil) {
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagDelete}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		state.Deleted = true
		return false
	}

	stateChanged := false
	for {
		var sig PauseSignal
		if !chs.pause.ReceiveAsync(&sig) {
			break
		}
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagPause}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handlePause(logger, sig, state) {
			stateChanged = true
		}
	}
	for {
		var sig UnpauseSignal
		if !chs.unpause.ReceiveAsync(&sig) {
			break
		}
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagUnpause}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleUnpause(logger, sig, state) {
			stateChanged = true
		}
	}
	for {
		var sig UpdateSignal
		if !chs.update.ReceiveAsync(&sig) {
			break
		}
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagUpdate}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleUpdate(logger, sig, input, state) {
			stateChanged = true
		}
	}
	for {
		var sig BackfillSignal
		if !chs.backfill.ReceiveAsync(&sig) {
			break
		}
		scope.Tagged(map[string]string{SignalTypeTag: signalTypeTagBackfill}).Counter(SchedulerSignalReceivedCountPerDomain).Inc(1)
		if handleBackfill(logger, sig, state) {
			stateChanged = true
		}
	}

	return stateChanged
}

// scheduleStateFromPaused maps the workflow's boolean Paused flag to the
// keyword value stored in the CadenceScheduleState search attribute.
func scheduleStateFromPaused(paused bool) string {
	if paused {
		return ScheduleStatePaused
	}
	return ScheduleStateActive
}

// buildScheduleSearchAttributes returns the search attributes that describe a
// scheduler workflow for ListSchedules: lifecycle state, cron expression, and
// target workflow type. The state SA is always written (the boolean Paused has
// a meaningful default). Optional fields (cron, workflow type) are omitted when
// empty so visibility queries can distinguish "absent" from "empty string".
func buildScheduleSearchAttributes(input *SchedulerWorkflowInput, state *SchedulerWorkflowState) map[string]interface{} {
	sa := map[string]interface{}{
		SearchAttrScheduleState: scheduleStateFromPaused(state.Paused),
	}
	if cron := input.Spec.CronExpression; cron != "" {
		sa[SearchAttrScheduleCron] = cron
	}
	if sw := input.Action.StartWorkflow; sw != nil && sw.WorkflowType != nil && sw.WorkflowType.Name != "" {
		sa[SearchAttrScheduleWorkflowType] = sw.WorkflowType.Name
	}
	return sa
}

func handlePause(logger *zap.Logger, sig PauseSignal, state *SchedulerWorkflowState) bool {
	state.Paused = true
	state.PauseReason = sig.Reason
	state.PausedBy = sig.PausedBy
	logger.Info("schedule paused", zap.String("reason", sig.Reason), zap.String("pausedBy", sig.PausedBy))
	return true
}

func handleUnpause(logger *zap.Logger, sig UnpauseSignal, state *SchedulerWorkflowState) bool {
	if !state.Paused {
		logger.Info("ignoring unpause signal, schedule is not paused")
		return false
	}
	state.Paused = false
	state.PauseReason = ""
	state.PausedBy = ""
	logger.Info("schedule unpaused", zap.String("reason", sig.Reason), zap.String("catchUpPolicy", sig.CatchUpPolicy.String()))
	return true
}

func handleUpdate(logger *zap.Logger, sig UpdateSignal, input *SchedulerWorkflowInput, state *SchedulerWorkflowState) bool {
	if sig.Spec == nil && sig.Action == nil && sig.Policies == nil {
		logger.Info("ignoring empty update signal")
		return false
	}
	changed := false
	if sig.Spec != nil {
		if _, err := cron.ParseStandard(sig.Spec.CronExpression); err != nil {
			logger.Error("ignoring update with invalid cron expression",
				zap.String("cron", sig.Spec.CronExpression), zap.Error(err))
		} else {
			input.Spec = *sig.Spec
			changed = true
			if len(state.PendingBackfills) > 0 {
				logger.Warn("spec change cleared pending backfills",
					zap.Int("clearedCount", len(state.PendingBackfills)))
				state.PendingBackfills = nil
			}
		}
	}
	if sig.Action != nil {
		input.Action = *sig.Action
		changed = true
	}
	if sig.Policies != nil {
		previousOverlap := input.Policies.OverlapPolicy
		input.Policies = *sig.Policies
		changed = true
		// Drop buffered fires if the overlap policy is no longer BUFFER:
		// draining a queue under non-BUFFER semantics is ill-defined.
		if previousOverlap == types.ScheduleOverlapPolicyBuffer &&
			input.Policies.OverlapPolicy != types.ScheduleOverlapPolicyBuffer &&
			len(state.BufferedFires) > 0 {
			logger.Warn("overlap policy change cleared buffered fires",
				zap.String("from", previousOverlap.String()),
				zap.String("to", input.Policies.OverlapPolicy.String()),
				zap.Int("clearedCount", len(state.BufferedFires)))
			state.SkippedRuns += int64(len(state.BufferedFires))
			state.BufferedFires = nil
		}
		// Drop running-workflow tracking when leaving bounded CONCURRENT: the
		// list is meaningless under any other policy or when limit becomes 0.
		newOverlap := input.Policies.OverlapPolicy
		newLimit := input.Policies.ConcurrencyLimit
		if previousOverlap == types.ScheduleOverlapPolicyConcurrent &&
			(newOverlap != types.ScheduleOverlapPolicyConcurrent || newLimit == 0) &&
			len(state.RunningWorkflows) > 0 {
			logger.Warn("policy change cleared running workflows tracking",
				zap.String("from", previousOverlap.String()),
				zap.String("to", newOverlap.String()),
				zap.Int32("newLimit", newLimit),
				zap.Int("clearedCount", len(state.RunningWorkflows)))
			state.RunningWorkflows = nil
		}
	}
	if changed {
		logger.Info("schedule updated")
	}
	return changed
}

func handleBackfill(logger *zap.Logger, sig BackfillSignal, state *SchedulerWorkflowState) bool {
	if !sig.EndTime.After(sig.StartTime) {
		logger.Warn("ignoring backfill with invalid time range",
			zap.Time("startTime", sig.StartTime),
			zap.Time("endTime", sig.EndTime),
		)
		return false
	}
	if len(state.PendingBackfills) >= maxPendingBackfills {
		logger.Warn("ignoring backfill: pending backfill queue is full",
			zap.String("backfillId", sig.BackfillID),
			zap.Int("queueSize", len(state.PendingBackfills)),
			zap.Int("maxPendingBackfills", maxPendingBackfills),
		)
		return false
	}
	for _, existing := range state.PendingBackfills {
		if sig.StartTime.Before(existing.EndTime) && sig.EndTime.After(existing.StartTime) {
			logger.Warn("backfill window overlaps with pending backfill, fires for overlapping times will be deduplicated",
				zap.String("newBackfillId", sig.BackfillID),
				zap.String("existingBackfillId", existing.BackfillID),
				zap.Time("overlapStart", maxTime(sig.StartTime, existing.StartTime)),
				zap.Time("overlapEnd", minTime(sig.EndTime, existing.EndTime)),
			)
		}
	}
	state.PendingBackfills = append(state.PendingBackfills, BackfillRequest{
		StartTime:     sig.StartTime,
		EndTime:       sig.EndTime,
		OverlapPolicy: sig.OverlapPolicy,
		BackfillID:    sig.BackfillID,
	})
	logger.Info("backfill queued",
		zap.Time("startTime", sig.StartTime),
		zap.Time("endTime", sig.EndTime),
		zap.String("overlapPolicy", sig.OverlapPolicy.String()),
		zap.String("backfillId", sig.BackfillID),
		zap.Int("pendingCount", len(state.PendingBackfills)),
	)
	return true
}

// effectiveFireOverlap returns the overlap policy applied to a single fire.
// For backfill, BackfillSignal.overlap_policy may be INVALID (0) to inherit the
// schedule's configured policy; any other value overrides for that backfill only.
func effectiveFireOverlap(trigger TriggerSource, backfillOverlap, scheduleOverlap types.ScheduleOverlapPolicy) types.ScheduleOverlapPolicy {
	if trigger == TriggerSourceBackfill && backfillOverlap != types.ScheduleOverlapPolicyInvalid {
		return backfillOverlap
	}
	return scheduleOverlap
}

// processScheduleFire executes the configured action for a single schedule fire.
// All side effects (overlap check, cancel/terminate, start) are encapsulated in
// a single activity so that the overlap logic can evolve without introducing
// nondeterminism in the workflow history.
//
// Under the BUFFER overlap policy, a fire that finds the previous target
// workflow still running is enqueued in state.BufferedFires (subject to
// BufferLimit and MaxBufferedFiresSystemLimit) and retried on the next loop
// iteration by drainBufferedFires.
//
// If BUFFER's queue is already non-empty when a new live fire arrives, we
// enqueue it directly without calling the activity. This both saves the
// describe RPC and prevents a FIFO-violation race: between the prior drain
// (which left fires queued because the previous workflow was running) and
// the live-fire activity call, the previous workflow could complete.
// tryStartFire would then start the live fire ahead of older queued fires,
// breaking FIFO.
func processScheduleFire(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, input *SchedulerWorkflowInput, state *SchedulerWorkflowState, scheduledTime time.Time, trigger TriggerSource, overlapPolicy types.ScheduleOverlapPolicy) {
	if overlapPolicy == types.ScheduleOverlapPolicyBuffer && len(state.BufferedFires) > 0 {
		// Skipping tryStartFire, so advance LastRunTime here.
		if scheduledTime.After(state.LastRunTime) {
			state.LastRunTime = scheduledTime
		}
		enqueueBufferedFire(logger, scope, input, state, scheduledTime, trigger, overlapPolicy)
		return
	}
	if tryStartFire(ctx, logger, input, state, scheduledTime, trigger, overlapPolicy) == fireOutcomeBuffered {
		enqueueBufferedFire(logger, scope, input, state, scheduledTime, trigger, overlapPolicy)
	}
}

// tryStartFire runs the scheduler activity for a single fire and applies the
// result to state, returning whether the fire was buffered. Shared by the
// live-fire and drain-buffered-fire paths; the caller decides how to handle
// a buffered outcome.
func tryStartFire(ctx workflow.Context, logger *zap.Logger, input *SchedulerWorkflowInput, state *SchedulerWorkflowState, scheduledTime time.Time, trigger TriggerSource, overlapPolicy types.ScheduleOverlapPolicy) fireOutcome {
	// LastRunTime moves forward only. Under BUFFER, an older queued fire can
	// drain after a newer fire has already been processed.
	if scheduledTime.After(state.LastRunTime) {
		state.LastRunTime = scheduledTime
	}

	logger.Info("schedule fired",
		zap.Time("scheduledTime", scheduledTime),
	)

	if input.Action.StartWorkflow == nil {
		state.MissedRuns++
		logger.Error("schedule action has no StartWorkflow configuration")
		return fireOutcomeDone
	}

	actCtx := workflow.WithLocalActivityOptions(ctx, defaultActivityOptions())

	req := ProcessFireRequest{
		Domain:              input.Domain,
		ScheduleID:          input.ScheduleID,
		Action:              *input.Action.StartWorkflow,
		ScheduledTime:       scheduledTime,
		TriggerSource:       trigger,
		OverlapPolicy:       overlapPolicy,
		LastStartedWorkflow: state.LastStartedWorkflow,
		ConcurrencyLimit:    input.Policies.ConcurrencyLimit,
		RunningWorkflows:    state.RunningWorkflows,
	}

	var result ProcessFireResult
	if err := workflow.ExecuteLocalActivity(actCtx, processScheduleFireActivity, req).Get(ctx, &result); err != nil {
		state.MissedRuns++
		logger.Error("processScheduleFireActivity failed",
			zap.Time("scheduledTime", scheduledTime),
			zap.Error(err),
		)
		return fireOutcomeDone
	}

	if result.Buffered {
		return fireOutcomeBuffered
	}

	state.TotalRuns += result.TotalDelta
	state.SkippedRuns += result.SkippedDelta
	if result.StartedWorkflow != nil {
		state.LastStartedWorkflow = result.StartedWorkflow
	}
	if result.ActiveWorkflows != nil {
		state.RunningWorkflows = result.ActiveWorkflows
	}

	if result.TotalDelta > 0 && result.StartedWorkflow != nil {
		logger.Info("scheduled workflow started",
			zap.String("workflowId", result.StartedWorkflow.WorkflowID),
			zap.String("runId", result.StartedWorkflow.RunID),
		)
	} else if result.SkippedDelta > 0 {
		logger.Info("schedule fire skipped",
			zap.Time("scheduledTime", scheduledTime),
		)
	}
	return fireOutcomeDone
}

// enqueueBufferedFire appends a fire to state.BufferedFires, enforcing both the
// user-configured buffer_limit and the MaxBufferedFiresSystemLimit ceiling.
// Drops increment SkippedRuns and emit scheduler_buffer_overflow_count_per_domain
// tagged with the binding limit (user_limit vs. system_limit).
func enqueueBufferedFire(logger *zap.Logger, scope tally.Scope, input *SchedulerWorkflowInput, state *SchedulerWorkflowState, scheduledTime time.Time, trigger TriggerSource, overlapPolicy types.ScheduleOverlapPolicy) {
	effective, reason := effectiveBufferLimit(input.Policies.BufferLimit)
	if len(state.BufferedFires) >= effective {
		state.SkippedRuns++
		scope.Tagged(map[string]string{ReasonTag: reason}).
			Counter(SchedulerBufferOverflowCountPerDomain).Inc(1)
		logger.Warn("buffer cap reached; dropping fire",
			zap.Time("scheduledTime", scheduledTime),
			zap.String("reason", reason),
			zap.Int("effectiveLimit", effective),
			zap.Int32("userBufferLimit", input.Policies.BufferLimit),
			zap.Int("systemLimit", MaxBufferedFiresSystemLimit),
			zap.Int("bufferSize", len(state.BufferedFires)),
		)
		return
	}
	state.BufferedFires = append(state.BufferedFires, BufferedFire{
		ScheduledTime: scheduledTime,
		TriggerSource: trigger,
		OverlapPolicy: overlapPolicy,
	})
	logger.Info("schedule fire buffered",
		zap.Time("scheduledTime", scheduledTime),
		zap.Int("bufferSize", len(state.BufferedFires)),
	)
}

// effectiveBufferLimit returns the queue cap actually enforced for the BUFFER
// overlap policy and the reason tag value to attribute drops at that cap.
//
//   - userLimit == 0 (unlimited): returns the system limit, reason=system_limit.
//   - 0 < userLimit <= system limit: returns userLimit, reason=user_limit.
//   - userLimit > system limit: returns the system limit, reason=system_limit
//     (the user's limit cannot be honored without risking ContinueAsNew payload bloat).
func effectiveBufferLimit(userLimit int32) (effective int, reason string) {
	if userLimit <= 0 || int(userLimit) > MaxBufferedFiresSystemLimit {
		return MaxBufferedFiresSystemLimit, BufferOverflowReasonSystemLimit
	}
	return int(userLimit), BufferOverflowReasonUserLimit
}

// effectiveConcurrencyLimit returns the concurrency cap enforced for the bounded
// CONCURRENT overlap policy. Values above the system ceiling are silently clamped
// so RunningWorkflows never grows large enough to bloat the ContinueAsNew payload
// toward Cadence's BlobSizeLimitError (default 2MB). Only called when userLimit > 0
// (i.e., isBoundedConcurrent is true).
func effectiveConcurrencyLimit(userLimit int32) int32 {
	if userLimit > MaxConcurrencyLimitSystemLimit {
		return MaxConcurrencyLimitSystemLimit
	}
	return userLimit
}

// drainBufferedFires executes queued fires in FIFO order, stopping as soon as
// one re-buffers (previous target workflow still running) or
// maxDrainFiresPerExecution fires have been processed. Returns true when more
// queued work remains and the caller should ContinueAsNew so the next batch
// runs in a fresh decision task (mirrors processMissedRuns / processBackfills).
//
// Retries are safe because the activity derives WorkflowID and RequestID from
// scheduledTime and triggerSource, so the server de-duplicates on replay.
func drainBufferedFires(ctx workflow.Context, logger *zap.Logger, input *SchedulerWorkflowInput, state *SchedulerWorkflowState) bool {
	drained := 0
	for len(state.BufferedFires) > 0 {
		if drained >= maxDrainFiresPerExecution {
			logger.Info("buffer drain cap reached, continuing after ContinueAsNew",
				zap.Int("drainedThisBatch", drained),
				zap.Int("remaining", len(state.BufferedFires)),
			)
			return true
		}
		head := state.BufferedFires[0]
		headOverlap := head.OverlapPolicy
		// OverlapPolicy is INVALID only for BufferedFires persisted before this
		// field was added; fall back to the schedule's current policy (guaranteed
		// BUFFER here: handleUpdate clears the queue on any policy change away from BUFFER).
		if headOverlap == types.ScheduleOverlapPolicyInvalid {
			headOverlap = input.Policies.OverlapPolicy
		}
		if tryStartFire(ctx, logger, input, state, head.ScheduledTime, head.TriggerSource, headOverlap) == fireOutcomeBuffered {
			return false
		}
		state.BufferedFires = state.BufferedFires[1:]
		drained++
	}
	return false
}

// defaultActivityOptions returns the standard local activity options used by
// all scheduler activities.
func defaultActivityOptions() workflow.LocalActivityOptions {
	return workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: localActivityScheduleToCloseTimeout,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    localActivityRetryInitialInterval,
			MaximumInterval:    localActivityRetryMaxInterval,
			MaximumAttempts:    localActivityMaxRetries,
			BackoffCoefficient: 2,
		},
	}
}

// computeNextRunTime determines the next fire time for the cron schedule,
// respecting the spec's StartTime and EndTime boundaries.
func computeNextRunTime(sched cron.Schedule, now time.Time, spec types.ScheduleSpec) time.Time {
	if !spec.StartTime.IsZero() && now.Before(spec.StartTime) {
		now = spec.StartTime.Add(-time.Second)
	}
	next := sched.Next(now)
	if !spec.EndTime.IsZero() && next.After(spec.EndTime) {
		return time.Time{}
	}
	return next
}

// missedFiresResult holds the output of computeMissedFireTimes.
type missedFiresResult struct {
	times     []time.Time
	truncated bool // true if the result was capped at maxCatchUpFires
}

// computeMissedFireTimes returns all cron fire times between (lastRun, now].
// It caps the result at maxCatchUpFires to prevent unbounded iteration
// for very frequent schedules that were paused for a long time.
// The truncated flag signals that more fires exist beyond the cap.
func computeMissedFireTimes(sched cron.Schedule, lastRun, now time.Time, spec types.ScheduleSpec) missedFiresResult {
	const maxCatchUpFires = 1000
	var missed []time.Time
	t := lastRun
	for len(missed) < maxCatchUpFires {
		next := computeNextRunTime(sched, t, spec)
		if next.IsZero() || next.After(now) {
			return missedFiresResult{times: missed, truncated: false}
		}
		missed = append(missed, next)
		t = next
	}
	return missedFiresResult{times: missed, truncated: true}
}

// missedRunPolicyResult is the output of applyMissedRunPolicy.
type missedRunPolicyResult struct {
	toFire  []time.Time // fire times to execute, in order
	skipped int64       // fires not executed due to catch-up policy or window
}

// applyMissedRunPolicy is a pure function that determines which missed fires
// to execute and how many to skip, given the catch-up policy and window.
// It is separated from processMissedRuns to allow direct unit testing.
func applyMissedRunPolicy(policy types.ScheduleCatchUpPolicy, window time.Duration, missed []time.Time, now time.Time, logger *zap.Logger) missedRunPolicyResult {
	var eligible []time.Time
	for _, t := range missed {
		if window <= 0 || now.Sub(t) <= window {
			eligible = append(eligible, t)
		}
	}
	outOfWindow := int64(len(missed) - len(eligible))

	switch policy {
	case types.ScheduleCatchUpPolicyOne:
		if len(eligible) == 0 {
			return missedRunPolicyResult{skipped: int64(len(missed))}
		}
		return missedRunPolicyResult{
			toFire:  []time.Time{eligible[len(eligible)-1]},
			skipped: outOfWindow + int64(len(eligible)-1),
		}
	case types.ScheduleCatchUpPolicyAll:
		return missedRunPolicyResult{
			toFire:  eligible,
			skipped: outOfWindow,
		}
	case types.ScheduleCatchUpPolicySkip:
		return missedRunPolicyResult{skipped: int64(len(missed))}
	default:
		logger.Warn("unknown catch-up policy, defaulting to skip",
			zap.Int32("policy", int32(policy)),
		)
		return missedRunPolicyResult{skipped: int64(len(missed))}
	}
}

// processMissedRuns checks for and processes any cron fires that were missed
// while the schedule was paused or during ContinueAsNew transitions.
// The catch-up policy determines how missed fires are handled:
//   - Skip: all missed fires are counted as skipped
//   - One: only the most recent eligible fire (within CatchUpWindow) is executed
//   - All: all eligible fires within the CatchUpWindow are executed
//
// To avoid exceeding the decision task timeout, at most maxCatchUpFiresPerExecution
// fires are executed per workflow execution. Returns true if there are more missed
// fires remaining, signalling the caller to ContinueAsNew for the next batch.
func processMissedRuns(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, sched cron.Schedule, input *SchedulerWorkflowInput, state *SchedulerWorkflowState) bool {
	watermark := catchUpWatermark(state)
	if state.Paused || watermark.IsZero() {
		return false
	}
	return processMissedRunsAt(ctx, logger, scope, sched, input, state, watermark, workflow.Now(ctx))
}

// catchUpWatermark returns the high-water mark for catch-up fire computation:
// the most recent timestamp the scheduler is known to have already attempted.
// We take max(LastProcessedTime, LastRunTime) because LastProcessedTime is
// only advanced by catch-up itself, while LastRunTime is advanced on every
// fire (live or buffered). Using only LastProcessedTime would let catch-up
// recompute fire times for fires that already happened (or were buffered
// under BUFFER), which deduplicates server-side via WorkflowID/RequestID but
// still double-counts state.TotalRuns / state.SkippedRuns.
func catchUpWatermark(state *SchedulerWorkflowState) time.Time {
	watermark := state.LastProcessedTime
	if state.LastRunTime.After(watermark) {
		watermark = state.LastRunTime
	}
	return watermark
}

// processMissedRunsAt is the testable core of processMissedRuns, accepting an explicit now
// so the caller can inject a deterministic time without needing a workflow environment.
func processMissedRunsAt(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, sched cron.Schedule, input *SchedulerWorkflowInput, state *SchedulerWorkflowState, watermark, now time.Time) bool {
	fires := computeMissedFireTimes(sched, watermark, now, input.Spec)
	if len(fires.times) == 0 {
		return false
	}

	if fires.truncated {
		logger.Warn("missed fires truncated, remaining will be caught up after ContinueAsNew",
			zap.Int("count", len(fires.times)),
			zap.Time("lastProcessedTime", watermark),
			zap.Time("now", now),
		)
	}

	result := applyMissedRunPolicy(input.Policies.CatchUpPolicy, input.Policies.CatchUpWindow, fires.times, now, logger)

	fired := 0
	for _, t := range result.toFire {
		if fired >= maxCatchUpFiresPerExecution {
			break
		}
		processScheduleFire(ctx, logger, scope, input, state, t, TriggerSourceSchedule, input.Policies.OverlapPolicy)
		fired++
	}
	unfired := int64(len(result.toFire) - fired)

	if fired > 0 {
		scope.Counter(SchedulerMissedFiredCountPerDomain).Inc(int64(fired))
	}

	policyStr := input.Policies.CatchUpPolicy.String()
	if result.skipped > 0 {
		scope.Tagged(map[string]string{CatchUpPolicyTag: policyStr}).
			Counter(SchedulerMissedSkippedCountPerDomain).Inc(result.skipped)
		state.SkippedRuns += result.skipped
		logger.Info("catch-up skipped missed fires",
			zap.Int64("skipped", result.skipped),
			zap.Int("total_missed", len(fires.times)),
			zap.String("policy", policyStr),
		)
	}

	// Advance watermark past all fires we've fully processed (fired or
	// skipped) to avoid re-discovering them after ContinueAsNew.
	// If we capped fires via maxCatchUpFiresPerExecution, only advance
	// to the last one we actually fired so the rest are retried.
	if unfired > 0 {
		state.LastProcessedTime = result.toFire[fired-1]
	} else if last := fires.times[len(fires.times)-1]; last.After(state.LastProcessedTime) {
		state.LastProcessedTime = last
	}

	return unfired > 0 || fires.truncated
}

// processBackfills drains pending backfill requests from state, computing
// cron fire times for each request's time range and executing them.
// Like processMissedRuns, it caps fires per execution and returns true
// if more work remains (signalling the caller to ContinueAsNew).
func processBackfills(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, sched cron.Schedule, input *SchedulerWorkflowInput, state *SchedulerWorkflowState) bool {
	// Backfills respect the pause state: an explicit user request to replay a time
	// range should not fire workflows while the schedule is paused. The pending
	// backfills are preserved in state and will execute once the schedule is unpaused.
	if state.Paused || len(state.PendingBackfills) == 0 {
		return false
	}

	fired := 0
	for len(state.PendingBackfills) > 0 {
		bf := &state.PendingBackfills[0]

		fires := computeMissedFireTimes(sched, bf.StartTime.Add(-time.Second), bf.EndTime, input.Spec)

		for _, t := range fires.times {
			if fired >= maxBackfillFiresPerExecution {
				bf.StartTime = t
				logger.Info("backfill batch cap reached, continuing after ContinueAsNew",
					zap.String("backfillId", bf.BackfillID),
					zap.Time("resumeFrom", t),
					zap.Int("firedThisBatch", fired),
				)
				scope.Counter(SchedulerBackfillFiredCountPerDomain).Inc(int64(fired))
				return true
			}
			overlap := effectiveFireOverlap(TriggerSourceBackfill, bf.OverlapPolicy, input.Policies.OverlapPolicy)
			processScheduleFire(ctx, logger, scope, input, state, t, TriggerSourceBackfill, overlap)
			fired++
		}

		if fires.truncated {
			// More fires exist beyond the 1000-fire scan cap.
			// Advance start past the last processed fire so it isn't replayed.
			if len(fires.times) > 0 {
				bf.StartTime = fires.times[len(fires.times)-1].Add(time.Second)
			}
			logger.Info("backfill range has more fires beyond scan cap, continuing after ContinueAsNew",
				zap.String("backfillId", bf.BackfillID),
				zap.Int("firedThisBatch", fired),
			)
			scope.Counter(SchedulerBackfillFiredCountPerDomain).Inc(int64(fired))
			return true
		}

		logger.Info("backfill completed",
			zap.String("backfillId", bf.BackfillID),
			zap.Int("firedTotal", fired),
		)
		state.PendingBackfills = state.PendingBackfills[1:]
	}

	if fired > 0 {
		scope.Counter(SchedulerBackfillFiredCountPerDomain).Inc(int64(fired))
	}

	return false
}

// buildScheduleDescription creates a snapshot of the current schedule
// configuration and runtime state for the describe query handler.
func buildScheduleDescription(input *SchedulerWorkflowInput, state *SchedulerWorkflowState) *ScheduleDescription {
	return &ScheduleDescription{
		ScheduleID:  input.ScheduleID,
		Domain:      input.Domain,
		Spec:        input.Spec,
		Action:      input.Action,
		Policies:    input.Policies,
		Paused:      state.Paused,
		PauseReason: state.PauseReason,
		PausedBy:    state.PausedBy,
		LastRunTime: state.LastRunTime,
		NextRunTime: state.NextRunTime,
		TotalRuns:   state.TotalRuns,
		MissedRuns:  state.MissedRuns,
		SkippedRuns: state.SkippedRuns,
	}
}

// safeContinueAsNew drains the delete channel before performing ContinueAsNew.
// Buffered signals are not carried across ContinueAsNew boundaries, so a delete
// signal that arrived alongside a state-changing signal would be lost without this check.
func safeContinueAsNew(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, cause string, deleteCh workflow.Channel, input SchedulerWorkflowInput, state *SchedulerWorkflowState) error {
	if deleteCh.ReceiveAsync(nil) {
		logger.Info("schedule deleted (caught before ContinueAsNew)")
		return nil
	}
	scope.Tagged(map[string]string{ReasonTag: cause}).Counter(SchedulerContinueAsNewCountPerDomain).Inc(1)
	state.Iterations = 0
	input.State = *state
	return workflow.NewContinueAsNewError(ctx, WorkflowTypeName, input)
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
