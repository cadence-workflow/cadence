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
			changed := handleUpdate(testLogger, tt.sig, &input)
			assert.Equal(t, tt.wantChanged, changed)
			assert.Equal(t, tt.wantCron, input.Spec.CronExpression)
			assert.Equal(t, tt.wantWF, input.Action.StartWorkflow.WorkflowType.Name)
			assert.Equal(t, tt.wantPol, input.Policies.OverlapPolicy)
		})
	}
}
