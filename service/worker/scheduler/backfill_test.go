package scheduler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
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
		name           string
		v1             bool
		queued, active int32
	}{
		{"legacy", false, maxBackfillRunsTotalCount, maxBackfillRunsTotalCount},
		{"corrected", true, 1, 502},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
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
	pending[1].RunsTotalComputed = true
	pending[1].RunsTotal = maxBackfillRunsTotalCount
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
