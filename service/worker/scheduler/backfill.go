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
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/types"
)

func handleBackfill(logger *zap.Logger, scope tally.Scope, sig BackfillSignal, state *SchedulerWorkflowState) bool {
	if !sig.EndTime.After(sig.StartTime) {
		scope.Tagged(map[string]string{ReasonTag: BackfillRejectedReasonInvalidRange}).
			Counter(SchedulerBackfillRejectedCountPerDomain).Inc(1)
		logger.Warn("ignoring backfill with invalid time range",
			zap.Time("startTime", sig.StartTime),
			zap.Time("endTime", sig.EndTime),
		)
		return false
	}
	if len(state.PendingBackfills) >= maxPendingBackfills {
		scope.Tagged(map[string]string{ReasonTag: BackfillRejectedReasonQueueFull}).
			Counter(SchedulerBackfillRejectedCountPerDomain).Inc(1)
		logger.Warn("ignoring backfill: pending backfill queue is full",
			zap.String("backfillId", sig.BackfillID),
			zap.Int("queueSize", len(state.PendingBackfills)),
			zap.Int("maxPendingBackfills", maxPendingBackfills),
		)
		return false
	}
	for _, existing := range state.PendingBackfills {
		if sig.BackfillID != "" && existing.BackfillID == sig.BackfillID {
			// Drop the duplicate: BackfillID already matches a pending
			// request, so a second copy would fire the range twice. Match is
			// only checked against state.PendingBackfills, so a retry that
			// lands after the original has drained is not detected here.
			scope.Tagged(map[string]string{ReasonTag: BackfillRejectedReasonDuplicateID}).
				Counter(SchedulerBackfillRejectedCountPerDomain).Inc(1)
			logger.Info("ignoring duplicate backfill: BackfillID matches a pending request",
				zap.String("backfillId", sig.BackfillID),
			)
			return false
		}
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

// countCronFires returns the number of cron fire times in (start, end] without
// materializing them. It stops at cancellation or the limit and reports whether
// it reached the limit.
func countCronFires(ctx context.Context, sched cron.Schedule, start, end time.Time, spec types.ScheduleSpec, limit int) (int, bool) {
	count := 0
	t := start
	for count < limit && ctx.Err() == nil {
		next := computeNextRunTime(sched, t, spec)
		if next.IsZero() || next.After(end) {
			return count, false
		}
		count++
		t = next
	}
	return count, count == limit
}

// reconcileBackfillTotals updates cached totals when a continued execution picks
// a different fractional-start version. A legacy cron could rediscover a tick
// before StartTime forever, in which case its cached total is always the cap.
func reconcileBackfillTotals(ctx workflow.Context, sched cron.Schedule, spec types.ScheduleSpec, pending []BackfillRequest) error {
	corrected, fractionalStart := sched.(fractionalStartCron)
	for i := range pending {
		bf := &pending[i]
		if !bf.RunsTotalComputed || bf.RunsTotalWithFractionalStart == fractionalStart {
			continue
		}
		if !fractionalStart {
			if repeatsBeforeStart(sched, spec, bf.StartTime, bf.EndTime) {
				bf.RunsTotal = maxBackfillRunsTotalCount
			}
		} else if needsFractionalStartRecount(corrected.Schedule, spec, *bf) {
			// A calendar scan can exceed the decision timeout. Replay uses the recorded activity result.
			actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				ScheduleToStartTimeout: time.Minute,
				StartToCloseTimeout:    time.Minute,
			})
			if err := workflow.ExecuteActivity(actCtx, recountBackfillActivity, spec, *bf).Get(ctx, &bf.RunsTotal); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				// Keep the old total for a later retry while scheduling continues.
				workflow.GetLogger(ctx).Warn("could not refresh backfill total", zap.String("backfillId", bf.BackfillID), zap.Error(err))
				continue
			}
		}
		bf.RunsTotalWithFractionalStart = fractionalStart
	}
	return nil
}

func needsFractionalStartRecount(legacy cron.Schedule, spec types.ScheduleSpec, bf BackfillRequest) bool {
	if bf.RunsTotal != maxBackfillRunsTotalCount || bf.RunsCompleted >= maxBackfillRunsTotalCount {
		return false
	}
	start := bf.StartTime
	if bf.RunsCompleted > 0 {
		// The original start is lost once a request has advanced. Rule out
		// stale counts only if the spec itself could never produce the bad tick.
		start = spec.StartTime
	}
	return repeatsBeforeStart(legacy, spec, start, bf.EndTime)
}

func repeatsBeforeStart(legacy cron.Schedule, spec types.ScheduleSpec, start, end time.Time) bool {
	cursor := start.Add(-time.Second)
	if !cursor.Before(spec.StartTime) {
		return false
	}
	first := computeNextRunTime(legacy, cursor, spec)
	return !first.IsZero() && first.Before(spec.StartTime) && !first.After(end)
}

func recountBackfillActivity(ctx context.Context, spec types.ScheduleSpec, bf BackfillRequest) (int32, error) {
	sched, err := cron.ParseStandard(spec.CronExpression)
	if err != nil {
		return 0, err
	}
	total := backfillTotal(ctx, fractionalStartCron{Schedule: sched, start: spec.StartTime}, spec, bf, bf.RunsCompleted)
	return total, ctx.Err()
}

func backfillTotal(ctx context.Context, sched cron.Schedule, spec types.ScheduleSpec, bf BackfillRequest, completed int32) int32 {
	completed = min(completed, maxBackfillRunsTotalCount)
	remaining, _ := countCronFires(ctx, sched, bf.StartTime.Add(-time.Second), bf.EndTime, spec, maxBackfillRunsTotalCount-int(completed))
	return completed + int32(remaining)
}

// updateBackfillTotals initializes totals for newly queued requests.
func updateBackfillTotals(sched cron.Schedule, spec types.ScheduleSpec, pending []BackfillRequest) {
	_, fractionalStart := sched.(fractionalStartCron)
	for i := range pending {
		bf := &pending[i]
		if bf.RunsTotalComputed {
			continue
		}
		var completed int32
		if fractionalStart {
			completed = bf.RunsCompleted
		}
		bf.RunsTotal = backfillTotal(context.Background(), sched, spec, *bf, completed)
		bf.RunsTotalComputed = true
		bf.RunsTotalWithFractionalStart = fractionalStart
	}
}

// processBackfills drains pending backfill requests from state, computing
// cron fire times for each request's time range and executing them.
// Returns true when more backfill work remains (budget exhausted or scan cap
// reached), signalling the caller to ContinueAsNew.
func processBackfills(ctx workflow.Context, logger *zap.Logger, scope tally.Scope, sched cron.Schedule, input *SchedulerWorkflowInput, state *SchedulerWorkflowState, budget *int) bool {
	if len(state.PendingBackfills) == 0 {
		return false
	}

	// Count paused work too so DescribeSchedule can report its progress.
	updateBackfillTotals(sched, input.Spec, state.PendingBackfills)

	// Backfills respect the pause state: an explicit user request to replay
	// a time range should not fire workflows while the schedule is paused.
	// The pending backfills are preserved and will execute on unpause.
	if state.Paused {
		return false
	}

	fired := 0
	for len(state.PendingBackfills) > 0 {
		bf := &state.PendingBackfills[0]

		fires := computeMissedFireTimes(sched, bf.StartTime.Add(-time.Second), bf.EndTime, input.Spec)

		for _, t := range fires.times {
			if *budget <= 0 {
				bf.StartTime = t
				logger.Info("activity budget exhausted mid-backfill, continuing after ContinueAsNew",
					zap.String("backfillId", bf.BackfillID),
					zap.Int("firedThisExecution", fired),
				)
				scope.Counter(SchedulerBackfillFiredCountPerDomain).Inc(int64(fired))
				return true
			}
			overlap := effectiveFireOverlap(TriggerSourceBackfill, bf.OverlapPolicy, input.Policies.OverlapPolicy)
			processScheduleFire(ctx, logger, scope, input, state, t, TriggerSourceBackfill, overlap, bf.BackfillID)
			fired++
			// Count any fire handed off to processScheduleFire, whether it
			// started, was skipped under the overlap policy, or was queued
			// into the BUFFER. Counting only "started" would pin a
			// BUFFER-deferred backfill in OngoingBackfills indefinitely.
			bf.RunsCompleted++
			*budget--
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
			zap.Int32("runsCompleted", bf.RunsCompleted),
			zap.Int32("runsTotal", bf.RunsTotal),
		)
		state.PendingBackfills = state.PendingBackfills[1:]
	}

	if fired > 0 {
		scope.Counter(SchedulerBackfillFiredCountPerDomain).Inc(int64(fired))
	}

	return false
}
