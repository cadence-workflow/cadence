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
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/types"
)

var testLogger = zap.NewNop()

func mustParseCron(t *testing.T, expr string) cron.Schedule {
	t.Helper()
	s, err := cron.ParseStandard(expr)
	require.NoError(t, err)
	return s
}

func TestComputeNextRunTime(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		cron     string
		now      time.Time
		spec     types.ScheduleSpec
		wantZero bool
		wantTime time.Time
	}{
		{
			name:     "every hour - next on the hour",
			cron:     "0 * * * *",
			now:      now,
			spec:     types.ScheduleSpec{},
			wantTime: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name:     "every day at midnight",
			cron:     "0 0 * * *",
			now:      now,
			spec:     types.ScheduleSpec{},
			wantTime: time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "now is before startTime - uses startTime as base",
			cron:     "0 * * * *",
			now:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			spec:     types.ScheduleSpec{StartTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
			wantTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "next run is after endTime - returns zero",
			cron:     "0 0 * * *",
			now:      time.Date(2026, 1, 15, 23, 0, 0, 0, time.UTC),
			spec:     types.ScheduleSpec{EndTime: time.Date(2026, 1, 15, 23, 59, 0, 0, time.UTC)},
			wantZero: true,
		},
		{
			name:     "next run is before endTime - returns next",
			cron:     "0 * * * *",
			now:      now,
			spec:     types.ScheduleSpec{EndTime: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
			wantTime: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name: "startTime and endTime together - within window",
			cron: "0 * * * *",
			now:  time.Date(2026, 6, 1, 5, 30, 0, 0, time.UTC),
			spec: types.ScheduleSpec{
				StartTime: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			},
			wantTime: time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC),
		},
		{
			name:     "every minute",
			cron:     "* * * * *",
			now:      now,
			spec:     types.ScheduleSpec{},
			wantTime: time.Date(2026, 1, 15, 10, 31, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := mustParseCron(t, tt.cron)
			got := computeNextRunTime(sched, tt.now, tt.spec)
			if tt.wantZero {
				assert.True(t, got.IsZero(), "expected zero time, got %v", got)
			} else {
				assert.Equal(t, tt.wantTime, got)
			}
		})
	}
}

func TestComputeMissedFireTimes(t *testing.T) {
	tests := []struct {
		name          string
		cron          string
		lastRun       time.Time
		now           time.Time
		spec          types.ScheduleSpec
		wantTimes     []time.Time
		wantTruncated bool
	}{
		{
			name:      "no missed fires - now is before next fire",
			cron:      "0 * * * *",
			lastRun:   time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			wantTimes: nil,
		},
		{
			name:    "one missed fire",
			cron:    "0 * * * *",
			lastRun: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:     time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC),
			wantTimes: []time.Time{
				time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "multiple missed fires",
			cron:    "0 * * * *",
			lastRun: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:     time.Date(2026, 1, 15, 13, 30, 0, 0, time.UTC),
			wantTimes: []time.Time{
				time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
				time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "missed fire exactly at now is included",
			cron:    "0 * * * *",
			lastRun: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
			wantTimes: []time.Time{
				time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
			},
		},
		{
			name:    "respects endTime - no fires past end",
			cron:    "0 * * * *",
			lastRun: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:     time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
			spec:    types.ScheduleSpec{EndTime: time.Date(2026, 1, 15, 12, 30, 0, 0, time.UTC)},
			wantTimes: []time.Time{
				time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name:      "lastRun equals now - no missed fires",
			cron:      "0 * * * *",
			lastRun:   time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			now:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			wantTimes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := mustParseCron(t, tt.cron)
			got := computeMissedFireTimes(sched, tt.lastRun, tt.now, tt.spec)
			assert.Equal(t, tt.wantTimes, got.times)
			assert.Equal(t, tt.wantTruncated, got.truncated)
		})
	}
}

func TestCatchUpOrchestration(t *testing.T) {
	now := time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC)
	lastProcessed := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	cronExpr := "0 * * * *"

	tests := []struct {
		name                   string
		policy                 types.ScheduleCatchUpPolicy
		window                 time.Duration
		wantFiredCount         int
		wantSkipped            int64
		wantLastProcessedAfter time.Time
	}{
		{
			name:                   "Skip advances watermark past all missed, fires nothing",
			policy:                 types.ScheduleCatchUpPolicySkip,
			wantFiredCount:         0,
			wantSkipped:            4, // 11:00, 12:00, 13:00, 14:00
			wantLastProcessedAfter: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:                   "One fires most recent, skips rest, advances watermark",
			policy:                 types.ScheduleCatchUpPolicyOne,
			wantFiredCount:         1,
			wantSkipped:            3, // 11:00, 12:00, 13:00 skipped
			wantLastProcessedAfter: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:                   "All fires everything, skips nothing, advances watermark",
			policy:                 types.ScheduleCatchUpPolicyAll,
			wantFiredCount:         4,
			wantSkipped:            0,
			wantLastProcessedAfter: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:                   "One with window excludes old fires",
			policy:                 types.ScheduleCatchUpPolicyOne,
			window:                 90 * time.Minute,
			wantFiredCount:         1,
			wantSkipped:            3, // 11:00, 12:00 out of window + 13:00 skipped eligible
			wantLastProcessedAfter: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		},
		{
			name:                   "All with tight window fires only recent",
			policy:                 types.ScheduleCatchUpPolicyAll,
			window:                 90 * time.Minute,
			wantFiredCount:         2, // 13:00 and 14:00 within 90min of 14:00
			wantSkipped:            2, // 11:00 and 12:00 out of window
			wantLastProcessedAfter: time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := mustParseCron(t, cronExpr)
			fires := computeMissedFireTimes(sched, lastProcessed, now, types.ScheduleSpec{})
			require.False(t, fires.truncated)
			require.Equal(t, 4, len(fires.times)) // 11:00, 12:00, 13:00, 14:00

			result := applyMissedRunPolicy(tt.policy, tt.window, fires.times, now, testLogger)
			assert.Equal(t, tt.wantFiredCount, len(result.toFire), "fired count")
			assert.Equal(t, tt.wantSkipped, result.skipped, "skipped count")

			lastMissed := fires.times[len(fires.times)-1]
			assert.True(t, !lastMissed.Before(tt.wantLastProcessedAfter), "watermark should advance to at least %v", tt.wantLastProcessedAfter)
		})
	}
}

func TestApplyMissedRunPolicy(t *testing.T) {
	now := time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC)
	fires := []time.Time{
		time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC), // 3h ago
		time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), // 2h ago
		time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC), // 1h ago
	}

	tests := []struct {
		name        string
		policy      types.ScheduleCatchUpPolicy
		window      time.Duration
		wantToFire  []time.Time
		wantSkipped int64
	}{
		{
			name:        "Skip - all missed fires are skipped",
			policy:      types.ScheduleCatchUpPolicySkip,
			wantToFire:  nil,
			wantSkipped: 3,
		},
		{
			name:        "Invalid (zero value) - defaults to skip",
			policy:      types.ScheduleCatchUpPolicyInvalid,
			wantToFire:  nil,
			wantSkipped: 3,
		},
		{
			name:        "One - no window, fires most recent",
			policy:      types.ScheduleCatchUpPolicyOne,
			wantToFire:  []time.Time{fires[2]},
			wantSkipped: 2,
		},
		{
			name:        "One - window excludes two oldest, fires most recent eligible",
			policy:      types.ScheduleCatchUpPolicyOne,
			window:      90 * time.Minute,
			wantToFire:  []time.Time{fires[2]}, // only 13:00 is within 90min of 14:00
			wantSkipped: 2,                     // 2 out-of-window, 0 skipped eligible
		},
		{
			name:        "One - window excludes all, nothing fired",
			policy:      types.ScheduleCatchUpPolicyOne,
			window:      30 * time.Minute,
			wantToFire:  nil,
			wantSkipped: 3,
		},
		{
			name:        "All - no window, fires all",
			policy:      types.ScheduleCatchUpPolicyAll,
			wantToFire:  fires,
			wantSkipped: 0,
		},
		{
			name:        "All - window filters two oldest, fires most recent only",
			policy:      types.ScheduleCatchUpPolicyAll,
			window:      90 * time.Minute,
			wantToFire:  fires[2:], // only 13:00 is within 90min of 14:00
			wantSkipped: 2,
		},
		{
			name:        "All - window excludes all, nothing fired",
			policy:      types.ScheduleCatchUpPolicyAll,
			window:      30 * time.Minute,
			wantToFire:  nil,
			wantSkipped: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyMissedRunPolicy(tt.policy, tt.window, fires, now, testLogger)
			assert.Equal(t, tt.wantToFire, got.toFire)
			assert.Equal(t, tt.wantSkipped, got.skipped)
		})
	}
}

func TestBuildScheduleDescription(t *testing.T) {
	lastRun := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	nextRun := time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input SchedulerWorkflowInput
		state SchedulerWorkflowState
		want  *ScheduleDescription
	}{
		{
			name: "running schedule with counters",
			input: SchedulerWorkflowInput{
				ScheduleID: "sched-1",
				Domain:     "test-domain",
				Spec:       types.ScheduleSpec{CronExpression: "0 * * * *"},
				Action: types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "my-wf"},
					},
				},
				Policies: types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicySkipNew},
			},
			state: SchedulerWorkflowState{
				LastRunTime: lastRun,
				NextRunTime: nextRun,
				TotalRuns:   42,
				MissedRuns:  1,
				SkippedRuns: 3,
			},
			want: &ScheduleDescription{
				ScheduleID: "sched-1",
				Domain:     "test-domain",
				Spec:       types.ScheduleSpec{CronExpression: "0 * * * *"},
				Action: types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "my-wf"},
					},
				},
				Policies:    types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicySkipNew},
				LastRunTime: lastRun,
				NextRunTime: nextRun,
				TotalRuns:   42,
				MissedRuns:  1,
				SkippedRuns: 3,
			},
		},
		{
			name: "paused schedule",
			input: SchedulerWorkflowInput{
				ScheduleID: "sched-2",
				Domain:     "prod",
				Spec:       types.ScheduleSpec{CronExpression: "0 0 * * *"},
			},
			state: SchedulerWorkflowState{
				Paused:      true,
				PauseReason: "maintenance",
				PausedBy:    "admin@test.com",
				TotalRuns:   10,
			},
			want: &ScheduleDescription{
				ScheduleID:  "sched-2",
				Domain:      "prod",
				Spec:        types.ScheduleSpec{CronExpression: "0 0 * * *"},
				Paused:      true,
				PauseReason: "maintenance",
				PausedBy:    "admin@test.com",
				TotalRuns:   10,
			},
		},
		{
			name:  "fresh schedule with no runs",
			input: SchedulerWorkflowInput{ScheduleID: "sched-new", Domain: "dev"},
			state: SchedulerWorkflowState{},
			want:  &ScheduleDescription{ScheduleID: "sched-new", Domain: "dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildScheduleDescription(&tt.input, &tt.state)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHandlePause(t *testing.T) {
	tests := []struct {
		name         string
		initial      SchedulerWorkflowState
		sig          PauseSignal
		wantPaused   bool
		wantReason   string
		wantPausedBy string
		wantChanged  bool
	}{
		{
			name:         "pause from running",
			initial:      SchedulerWorkflowState{},
			sig:          PauseSignal{Reason: "maintenance", PausedBy: "admin@test.com"},
			wantPaused:   true,
			wantReason:   "maintenance",
			wantPausedBy: "admin@test.com",
			wantChanged:  true,
		},
		{
			name:         "pause overwrites previous pause reason",
			initial:      SchedulerWorkflowState{Paused: true, PauseReason: "old", PausedBy: "old-user"},
			sig:          PauseSignal{Reason: "new reason", PausedBy: "new-user"},
			wantPaused:   true,
			wantReason:   "new reason",
			wantPausedBy: "new-user",
			wantChanged:  true,
		},
		{
			name:         "pause with empty reason",
			initial:      SchedulerWorkflowState{},
			sig:          PauseSignal{},
			wantPaused:   true,
			wantReason:   "",
			wantPausedBy: "",
			wantChanged:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			changed := handlePause(testLogger, tt.sig, &state)
			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.wantPaused, state.Paused)
			assert.Equal(t, tt.wantReason, state.PauseReason)
			assert.Equal(t, tt.wantPausedBy, state.PausedBy)
		})
	}
}

func TestHandleUnpause(t *testing.T) {
	tests := []struct {
		name         string
		initial      SchedulerWorkflowState
		sig          UnpauseSignal
		wantPaused   bool
		wantReason   string
		wantPausedBy string
		wantChanged  bool
	}{
		{
			name:         "unpause from paused",
			initial:      SchedulerWorkflowState{Paused: true, PauseReason: "maintenance", PausedBy: "admin"},
			sig:          UnpauseSignal{Reason: "maintenance done"},
			wantPaused:   false,
			wantReason:   "",
			wantPausedBy: "",
			wantChanged:  true,
		},
		{
			name:         "unpause when not paused is a no-op",
			initial:      SchedulerWorkflowState{Paused: false},
			sig:          UnpauseSignal{Reason: "shouldn't matter"},
			wantPaused:   false,
			wantReason:   "",
			wantPausedBy: "",
			wantChanged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.initial
			changed := handleUnpause(testLogger, tt.sig, &state)
			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.wantPaused, state.Paused)
			assert.Equal(t, tt.wantReason, state.PauseReason)
			assert.Equal(t, tt.wantPausedBy, state.PausedBy)
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	original := SchedulerWorkflowInput{
		Domain:     "test-domain",
		ScheduleID: "sched-1",
		Spec:       types.ScheduleSpec{CronExpression: "0 * * * *"},
		Action: types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType: &types.WorkflowType{Name: "old-workflow"},
			},
		},
		Policies: types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicySkipNew},
	}

	tests := []struct {
		name        string
		sig         UpdateSignal
		wantCron    string
		wantWF      string
		wantPol     types.ScheduleOverlapPolicy
		wantChanged bool
	}{
		{
			name: "update spec only",
			sig: UpdateSignal{
				Spec: &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
			},
			wantCron:    "*/5 * * * *",
			wantWF:      "old-workflow",
			wantPol:     types.ScheduleOverlapPolicySkipNew,
			wantChanged: true,
		},
		{
			name: "update action only",
			sig: UpdateSignal{
				Action: &types.ScheduleAction{
					StartWorkflow: &types.StartWorkflowAction{
						WorkflowType: &types.WorkflowType{Name: "new-workflow"},
					},
				},
			},
			wantCron:    "0 * * * *",
			wantWF:      "new-workflow",
			wantPol:     types.ScheduleOverlapPolicySkipNew,
			wantChanged: true,
		},
		{
			name: "update policies only",
			sig: UpdateSignal{
				Policies: &types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicyConcurrent},
			},
			wantCron:    "0 * * * *",
			wantWF:      "old-workflow",
			wantPol:     types.ScheduleOverlapPolicyConcurrent,
			wantChanged: true,
		},
		{
			name:        "nil fields leave input unchanged",
			sig:         UpdateSignal{},
			wantCron:    "0 * * * *",
			wantWF:      "old-workflow",
			wantPol:     types.ScheduleOverlapPolicySkipNew,
			wantChanged: false,
		},
		{
			name: "invalid cron expression is rejected, spec unchanged",
			sig: UpdateSignal{
				Spec: &types.ScheduleSpec{CronExpression: "not-a-cron"},
			},
			wantCron:    "0 * * * *",
			wantWF:      "old-workflow",
			wantPol:     types.ScheduleOverlapPolicySkipNew,
			wantChanged: false,
		},
		{
			name: "invalid cron rejected but action and policies still applied",
			sig: UpdateSignal{
				Spec:     &types.ScheduleSpec{CronExpression: "bad"},
				Action:   &types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "new-workflow"}}},
				Policies: &types.SchedulePolicies{OverlapPolicy: types.ScheduleOverlapPolicyConcurrent},
			},
			wantCron:    "0 * * * *",
			wantWF:      "new-workflow",
			wantPol:     types.ScheduleOverlapPolicyConcurrent,
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := original
			state := &SchedulerWorkflowState{}
			changed := handleUpdate(testLogger, tt.sig, &input, state)
			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.wantCron, input.Spec.CronExpression)
			assert.Equal(t, tt.wantWF, input.Action.StartWorkflow.WorkflowType.Name)
			assert.Equal(t, tt.wantPol, input.Policies.OverlapPolicy)
		})
	}

	t.Run("spec change clears pending backfills", func(t *testing.T) {
		input := original
		state := &SchedulerWorkflowState{
			PendingBackfills: []BackfillRequest{
				{StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
				{StartTime: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)},
			},
		}
		changed := handleUpdate(testLogger, UpdateSignal{
			Spec: &types.ScheduleSpec{CronExpression: "*/5 * * * *"},
		}, &input, state)
		assert.True(t, changed)
		assert.Equal(t, "*/5 * * * *", input.Spec.CronExpression)
		assert.Empty(t, state.PendingBackfills)
	})

	t.Run("action-only update preserves pending backfills", func(t *testing.T) {
		input := original
		state := &SchedulerWorkflowState{
			PendingBackfills: []BackfillRequest{
				{StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
		}
		changed := handleUpdate(testLogger, UpdateSignal{
			Action: &types.ScheduleAction{StartWorkflow: &types.StartWorkflowAction{WorkflowType: &types.WorkflowType{Name: "new-workflow"}}},
		}, &input, state)
		assert.True(t, changed)
		assert.Len(t, state.PendingBackfills, 1)
	})

	t.Run("invalid cron does not clear pending backfills", func(t *testing.T) {
		input := original
		state := &SchedulerWorkflowState{
			PendingBackfills: []BackfillRequest{
				{StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
		}
		changed := handleUpdate(testLogger, UpdateSignal{
			Spec: &types.ScheduleSpec{CronExpression: "not-a-cron"},
		}, &input, state)
		assert.False(t, changed)
		assert.Len(t, state.PendingBackfills, 1)
	})
}

func TestHandleBackfill(t *testing.T) {
	tests := []struct {
		name           string
		sig            BackfillSignal
		initialPending int
		wantQueued     bool
		wantPendingLen int
	}{
		{
			name: "valid backfill is queued",
			sig: BackfillSignal{
				StartTime:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:       time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				OverlapPolicy: types.ScheduleOverlapPolicyConcurrent,
				BackfillID:    "bf-1",
			},
			wantQueued:     true,
			wantPendingLen: 1,
		},
		{
			name: "invalid range (end <= start) is rejected",
			sig: BackfillSignal{
				StartTime: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			wantQueued:     false,
			wantPendingLen: 0,
		},
		{
			name: "equal start and end is rejected",
			sig: BackfillSignal{
				StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			wantQueued:     false,
			wantPendingLen: 0,
		},
		{
			name: "multiple backfills accumulate",
			sig: BackfillSignal{
				StartTime:  time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
				EndTime:    time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
				BackfillID: "bf-2",
			},
			initialPending: 1,
			wantQueued:     true,
			wantPendingLen: 2,
		},
		{
			name: "overlapping backfill is queued with warning",
			sig: BackfillSignal{
				StartTime:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
				EndTime:    time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
				BackfillID: "bf-overlap",
			},
			initialPending: 1,
			wantQueued:     true,
			wantPendingLen: 2,
		},
		{
			name: "backfill rejected when queue is full",
			sig: BackfillSignal{
				StartTime:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
				EndTime:    time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
				BackfillID: "bf-over-cap",
			},
			initialPending: maxPendingBackfills,
			wantQueued:     false,
			wantPendingLen: maxPendingBackfills,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &SchedulerWorkflowState{}
			for i := 0; i < tt.initialPending; i++ {
				state.PendingBackfills = append(state.PendingBackfills, BackfillRequest{
					StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					EndTime:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				})
			}
			got := handleBackfill(testLogger, tt.sig, state)
			assert.Equal(t, tt.wantQueued, got)
			assert.Equal(t, tt.wantPendingLen, len(state.PendingBackfills))
		})
	}
}

func TestProcessBackfillsRespectsPause(t *testing.T) {
	sched := mustParseCron(t, "0 * * * *")
	input := &SchedulerWorkflowInput{
		Spec: types.ScheduleSpec{CronExpression: "0 * * * *"},
	}
	state := &SchedulerWorkflowState{
		Paused: true,
		PendingBackfills: []BackfillRequest{
			{
				StartTime:  time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
				EndTime:    time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC),
				BackfillID: "bf-paused",
			},
		},
	}
	// processBackfills should short-circuit without touching PendingBackfills
	moreWork := processBackfills(nil, testLogger, sched, input, state)
	assert.False(t, moreWork, "paused schedule should not process backfills")
	assert.Len(t, state.PendingBackfills, 1, "pending backfills should be preserved while paused")
}

func TestBackfillFireComputation(t *testing.T) {
	sched := mustParseCron(t, "0 * * * *")

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		wantFires int
	}{
		{
			name:      "3-hour window [10:00, 13:00] produces 4 fires (inclusive both ends)",
			startTime: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			endTime:   time.Date(2026, 1, 15, 13, 0, 0, 0, time.UTC),
			wantFires: 4, // 10:00, 11:00, 12:00, 13:00
		},
		{
			name:      "exact boundary [10:00, 11:00] includes both endpoints",
			startTime: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			endTime:   time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
			wantFires: 2, // 10:00, 11:00
		},
		{
			name:      "sub-hour window [10:00, 10:30] includes start fire only",
			startTime: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			endTime:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			wantFires: 1, // 10:00
		},
		{
			name:      "24-hour window [00:00, 00:00+1d] produces 25 fires",
			startTime: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			endTime:   time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC),
			wantFires: 25, // 00:00 through 00:00 next day, inclusive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fires := computeMissedFireTimes(sched, tt.startTime.Add(-time.Second), tt.endTime, types.ScheduleSpec{})
			assert.Equal(t, tt.wantFires, len(fires.times))
		})
	}
}
