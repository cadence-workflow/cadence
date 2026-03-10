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

	state := &input.State

	err := workflow.SetQueryHandler(ctx, QueryTypeDescribe, func() (*ScheduleDescription, error) {
		return buildScheduleDescription(&input, state), nil
	})
	if err != nil {
		return fmt.Errorf("failed to register query handler: %w", err)
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

		changed, timerFired := applyAllInputs(ctx, logger, timerFuture, chs, state, &input)

		if timerCancel != nil {
			timerCancel()
		}

		if state.Deleted {
			logger.Info("schedule deleted")
			return nil
		}

		if timerFired && !state.Paused {
			processScheduleFire(ctx, logger, &input, state)
		}

		if changed || state.Iterations >= maxIterationsBeforeContinueAsNew {
			return safeContinueAsNew(ctx, logger, chs.delete, input, state)
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
		if handlePause(logger, sig, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.unpause, func(c workflow.Channel, more bool) {
		var sig UnpauseSignal
		c.Receive(ctx, &sig)
		if handleUnpause(logger, sig, state) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.update, func(c workflow.Channel, more bool) {
		var sig UpdateSignal
		c.Receive(ctx, &sig)
		if handleUpdate(logger, sig, input) {
			stateChanged = true
		}
	})

	selector.AddReceive(chs.backfill, func(c workflow.Channel, more bool) {
		var sig BackfillSignal
		c.Receive(ctx, &sig)
		handleBackfill(logger, sig, state)
	})

	selector.AddReceive(chs.delete, func(c workflow.Channel, more bool) {
		c.Receive(ctx, nil)
		state.Deleted = true
	})

	selector.Select(ctx)

	if drainBufferedSignals(logger, chs, state, input) {
		stateChanged = true
	}

	return stateChanged, timerFired
}

// drainBufferedSignals processes any remaining buffered signals without blocking.
// Delete signals are checked first to prevent signal loss across ContinueAsNew boundaries.
// Returns true if a state-changing signal was found.
func drainBufferedSignals(
	logger *zap.Logger,
	chs signalChannels,
	state *SchedulerWorkflowState,
	input *SchedulerWorkflowInput,
) bool {
	if chs.delete.ReceiveAsync(nil) {
		state.Deleted = true
		return false
	}

	stateChanged := false
	for {
		var sig PauseSignal
		if !chs.pause.ReceiveAsync(&sig) {
			break
		}
		if handlePause(logger, sig, state) {
			stateChanged = true
		}
	}
	for {
		var sig UnpauseSignal
		if !chs.unpause.ReceiveAsync(&sig) {
			break
		}
		if handleUnpause(logger, sig, state) {
			stateChanged = true
		}
	}
	for {
		var sig UpdateSignal
		if !chs.update.ReceiveAsync(&sig) {
			break
		}
		if handleUpdate(logger, sig, input) {
			stateChanged = true
		}
	}
	for {
		var sig BackfillSignal
		if !chs.backfill.ReceiveAsync(&sig) {
			break
		}
		handleBackfill(logger, sig, state)
	}

	return stateChanged
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

func handleUpdate(logger *zap.Logger, sig UpdateSignal, input *SchedulerWorkflowInput) bool {
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
		}
	}
	if sig.Action != nil {
		input.Action = *sig.Action
		changed = true
	}
	if sig.Policies != nil {
		input.Policies = *sig.Policies
		changed = true
	}
	if changed {
		logger.Info("schedule updated")
	}
	return changed
}

func handleBackfill(logger *zap.Logger, sig BackfillSignal, state *SchedulerWorkflowState) {
	logger.Info("backfill signal received",
		zap.Time("startTime", sig.StartTime),
		zap.Time("endTime", sig.EndTime),
		zap.String("overlapPolicy", sig.OverlapPolicy.String()),
		zap.String("backfillId", sig.BackfillID),
	)
}

// processScheduleFire executes the configured action for a schedule fire.
// It calls the start-workflow activity, updates state counters, and logs the outcome.
// Activity failures do not terminate the schedule, they are logged and counted as missed runs.
func processScheduleFire(ctx workflow.Context, logger *zap.Logger, input *SchedulerWorkflowInput, state *SchedulerWorkflowState) {
	scheduledTime := state.NextRunTime
	state.LastRunTime = scheduledTime
	state.TotalRuns++

	logger.Info("schedule fired",
		zap.Time("scheduledTime", scheduledTime),
		zap.Int64("totalRuns", state.TotalRuns),
	)

	activityOpts := workflow.LocalActivityOptions{
		ScheduleToCloseTimeout: localActivityScheduleToCloseTimeout,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    localActivityRetryInitialInterval,
			MaximumInterval:    localActivityRetryMaxInterval,
			MaximumAttempts:    localActivityMaxRetries,
			BackoffCoefficient: 2,
		},
	}
	actCtx := workflow.WithLocalActivityOptions(ctx, activityOpts)

	if input.Action.StartWorkflow == nil {
		state.MissedRuns++
		logger.Error("schedule action has no StartWorkflow configuration")
		return
	}

	req := StartWorkflowRequest{
		Domain:        input.Domain,
		ScheduleID:    input.ScheduleID,
		Action:        *input.Action.StartWorkflow,
		ScheduledTime: scheduledTime,
	}

	var result StartWorkflowResult
	err := workflow.ExecuteLocalActivity(actCtx, startWorkflowActivity, req).Get(ctx, &result)
	if err != nil {
		state.MissedRuns++
		logger.Error("scheduled action failed",
			zap.Time("scheduledTime", scheduledTime),
			zap.Error(err),
		)
		return
	}

	if result.Skipped {
		state.SkippedRuns++
		logger.Info("scheduled action skipped (already running)",
			zap.String("workflowId", result.WorkflowID),
		)
		return
	}

	logger.Info("scheduled workflow started",
		zap.String("workflowId", result.WorkflowID),
		zap.String("runId", result.RunID),
	)
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
func safeContinueAsNew(ctx workflow.Context, logger *zap.Logger, deleteCh workflow.Channel, input SchedulerWorkflowInput, state *SchedulerWorkflowState) error {
	if deleteCh.ReceiveAsync(nil) {
		logger.Info("schedule deleted (caught before ContinueAsNew)")
		return nil
	}
	state.Iterations = 0
	input.State = *state
	return workflow.NewContinueAsNewError(ctx, WorkflowTypeName, input)
}
