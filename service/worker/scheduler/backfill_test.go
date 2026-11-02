package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/encoded"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/types"
)

func TestDescribeRecountsLegacyFractionalStartBackfills(t *testing.T) {
	minute := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	input := SchedulerWorkflowInput{
		Spec: types.ScheduleSpec{CronExpression: "* * * * *", StartTime: minute.Add(500 * time.Millisecond)},
		State: SchedulerWorkflowState{
			CreateTime: minute.Add(-time.Hour),
			Paused:     true,
			PendingBackfills: []BackfillRequest{
				{
					StartTime: minute, EndTime: minute.Add(time.Minute),
					RunsTotalComputed: true, RunsTotal: maxBackfillRunsTotalCount,
				},
				{
					StartTime: minute.Add(2 * time.Minute), EndTime: minute.Add(3 * time.Minute),
					RunsTotalComputed: true, RunsTotal: maxBackfillRunsTotalCount, RunsCompleted: 500,
				},
			},
		},
	}

	for _, tt := range []struct {
		name            string
		v1, unavailable bool
		queued, active  int32
	}{
		{"legacy", false, false, maxBackfillRunsTotalCount, maxBackfillRunsTotalCount},
		{"corrected", true, false, 1, 502},
		{"recount worker unavailable", true, true, maxBackfillRunsTotalCount, maxBackfillRunsTotalCount},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterActivity(recountBackfillActivity)
			if tt.unavailable {
				env.SetActivityTaskList("another-task-list", recountBackfillActivity)
			}
			env.SetStartTime(minute.Add(-time.Minute))
			var desc ScheduleDescription
			env.RegisterDelayedCallback(func() {
				result, err := env.QueryWorkflow(QueryTypeDescribe)
				require.NoError(t, err)
				require.NoError(t, result.Get(&desc))
				env.SignalWorkflow(SignalNameDelete, nil)
			}, time.Second)
			env.ExecuteWorkflow(func(ctx workflow.Context, input SchedulerWorkflowInput) error {
				if tt.v1 {
					workflow.GetVersion(ctx, fractionalStartTimeChangeID, workflow.DefaultVersion, 1)
				}
				return SchedulerWorkflow(ctx, input)
			}, input)
			require.NoError(t, env.GetWorkflowError())
			require.Len(t, desc.OngoingBackfills, 2)
			require.Equal(t, tt.queued, desc.OngoingBackfills[0].RunsTotal)
			require.Equal(t, int32(0), desc.OngoingBackfills[0].RunsCompleted)
			require.Equal(t, tt.active, desc.OngoingBackfills[1].RunsTotal)
			require.Equal(t, int32(500), desc.OngoingBackfills[1].RunsCompleted)
		})
	}
}

type countingBackfillCron struct {
	cron.Schedule
	calls int
}

func (c *countingBackfillCron) Next(t time.Time) time.Time {
	c.calls++
	return c.Schedule.Next(t)
}

func TestFractionalStartBackfillCountCap(t *testing.T) {
	minute := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	spec := types.ScheduleSpec{CronExpression: "@every 1s", StartTime: minute.Add(500 * time.Millisecond)}
	raw := &countingBackfillCron{Schedule: mustParseCron(t, spec.CronExpression)}
	sched := fractionalStartCron{Schedule: raw, start: spec.StartTime}
	req := BackfillRequest{StartTime: minute.Add(2 * time.Second), EndTime: minute.Add(48 * time.Hour)}
	pending := []BackfillRequest{req, req, req}
	pending[1].RunsCompleted = maxBackfillRunsTotalCount - 1
	pending[2].RunsCompleted = maxBackfillRunsTotalCount + 1
	updateBackfillTotals(sched, spec, pending)
	for _, bf := range pending {
		require.Equal(t, int32(maxBackfillRunsTotalCount), bf.RunsTotal)
	}
	require.Equal(t, maxBackfillRunsTotalCount+1, raw.calls)
	require.Equal(t, int32(maxBackfillRunsTotalCount+1), pending[2].RunsCompleted)

	// A continued execution can reuse even capped totals without walking the cron again.
	data, err := json.Marshal(pending)
	require.NoError(t, err)
	var resumed []BackfillRequest
	require.NoError(t, json.Unmarshal(data, &resumed))
	updateBackfillTotals(sched, spec, resumed)
	require.Equal(t, maxBackfillRunsTotalCount+1, raw.calls)
}

func TestReuseUnaffectedCappedBackfills(t *testing.T) {
	boundary := time.Date(2026, 1, 1, 12, 0, 0, 500000000, time.UTC)
	spec := types.ScheduleSpec{CronExpression: "0 * * * *", StartTime: boundary}
	raw := &countingBackfillCron{Schedule: mustParseCron(t, spec.CronExpression)}
	corrected := fractionalStartCron{Schedule: raw, start: boundary}
	pending := make([]BackfillRequest, maxPendingBackfills)
	for i := range pending {
		pending[i] = BackfillRequest{
			StartTime: boundary.Add(time.Hour), EndTime: boundary.AddDate(15, 0, 0),
			RunsTotal: maxBackfillRunsTotalCount, RunsTotalComputed: true,
		}
	}
	require.NoError(t, reconcileBackfillTotals(nil, corrected, spec, pending))
	for _, bf := range pending {
		require.Equal(t, int32(maxBackfillRunsTotalCount), bf.RunsTotal)
		require.True(t, bf.RunsTotalWithFractionalStart)
	}
	require.Zero(t, raw.calls)
}

func TestAffectedCappedBackfillsCountOutsideWorkflow(t *testing.T) {
	boundary := time.Date(2026, 1, 1, 12, 0, 0, 500000000, time.UTC)
	spec := types.ScheduleSpec{CronExpression: "@every 1s", StartTime: boundary}
	raw := &countingBackfillCron{Schedule: mustParseCron(t, spec.CronExpression)}
	corrected := fractionalStartCron{Schedule: raw, start: boundary}
	pending := make([]BackfillRequest, maxPendingBackfills)
	for i := range pending {
		pending[i] = BackfillRequest{
			StartTime: boundary, EndTime: boundary.Add(24*time.Hour + time.Duration(i)*time.Second),
			RunsTotal: maxBackfillRunsTotalCount, RunsTotalComputed: true,
		}
	}

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var activities, probesAtFirstStart int
	env.RegisterActivity(recountBackfillActivity)
	env.SetOnActivityStartedListener(func(*activity.Info, context.Context, encoded.Values) {
		if activities == 0 {
			// The test environment delivers this callback after the workflow first yields.
			probesAtFirstStart = raw.calls
		}
		activities++
	})
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		if err := reconcileBackfillTotals(ctx, corrected, spec, pending); err != nil {
			return err
		}
		return reconcileBackfillTotals(ctx, corrected, spec, pending)
	})
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, len(pending), probesAtFirstStart)
	require.Equal(t, len(pending), activities)
	require.LessOrEqual(t, raw.calls, len(pending))
	for i, bf := range pending {
		require.EqualValues(t, 24*60*60+i, bf.RunsTotal)
		require.True(t, bf.RunsTotalWithFractionalStart)
	}
}

func TestRollbackRecountsBeforeCatchUpContinues(t *testing.T) {
	noon := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartTime(noon.Add(24 * time.Hour))
	env.ExecuteWorkflow(SchedulerWorkflow, SchedulerWorkflowInput{
		Spec:     types.ScheduleSpec{CronExpression: "* * * * *", StartTime: noon.Add(500 * time.Millisecond)},
		Policies: types.SchedulePolicies{CatchUpPolicy: types.ScheduleCatchUpPolicySkip},
		State: SchedulerWorkflowState{
			CreateTime: noon.Add(time.Minute),
			PendingBackfills: []BackfillRequest{{
				StartTime: noon, EndTime: noon.Add(time.Minute),
				RunsTotal: 1, RunsTotalComputed: true, RunsTotalWithFractionalStart: true,
			}},
		},
	})

	var continued *workflow.ContinueAsNewError
	require.ErrorAs(t, env.GetWorkflowError(), &continued)
	state := continued.Args()[0].(SchedulerWorkflowInput).State
	require.EqualValues(t, 1000, state.SkippedRuns)
	require.Len(t, state.PendingBackfills, 1)
	bf := state.PendingBackfills[0]
	require.EqualValues(t, maxBackfillRunsTotalCount, bf.RunsTotal)
	require.False(t, bf.RunsTotalWithFractionalStart)
}
