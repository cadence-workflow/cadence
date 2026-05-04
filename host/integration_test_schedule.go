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

package host

import (
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

// TestScheduleSmokeTest exercises the minimum schedule-pipeline path end-to-end:
// CreateSchedule → scheduler workflow starts → a target workflow fires →
// DescribeSchedule reports a non-zero TotalRuns → DeleteSchedule.
//
// This is the smoke test for the host/onebox.go scheduler-worker-manager wiring
// added in the same change. It does not exercise overlap/catch-up/backfill
// semantics; those live in dedicated tests in a follow-up change.
func (s *IntegrationSuite) TestScheduleSmokeTest() {
	scheduleID := "smoke-schedule"
	targetWorkflowType := "smoke-schedule-target"
	targetTaskList := "smoke-schedule-tasklist"
	identity := "schedule-smoke-test"

	// `@every 5s` is the smallest interval cron parser accepts. We allow up to
	// 90s for the first fire to land — the scheduler worker manager has a 60s
	// periodic refresh, and registering this domain races that ticker.
	createReq := &types.CreateScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
		Spec: &types.ScheduleSpec{
			CronExpression: "@every 5s",
		},
		Action: &types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType:                        &types.WorkflowType{Name: targetWorkflowType},
				TaskList:                            &types.TaskList{Name: targetTaskList},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
			},
		},
	}

	createCtx, createCancel := createContext()
	defer createCancel()
	_, err := s.Engine.CreateSchedule(createCtx, createReq)
	s.Require().NoError(err, "CreateSchedule failed")

	// Each scheduler-fired target workflow needs a poller to complete its first
	// decision task; otherwise the target stays open and the scheduler still
	// counts a successful start, but the fixture leaks.
	completeOnFirstDecision := func(execution *types.WorkflowExecution, _ *types.WorkflowType,
		_, _ int64, _ *types.History) ([]byte, []*types.Decision, error) {
		s.Logger.Info("scheduler-fired target workflow polled",
			tag.WorkflowID(execution.GetWorkflowID()),
			tag.WorkflowRunID(execution.GetRunID()))
		return nil, []*types.Decision{
			{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("smoke-target-complete"),
				},
			},
		}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        &types.TaskList{Name: targetTaskList},
		Identity:        identity,
		DecisionHandler: completeOnFirstDecision,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Poll inline, alternating between draining decision tasks and re-checking
	// DescribeSchedule. We bail as soon as TotalRuns advances past zero.
	const maxIterations = 30 // ~120s with 4s per iteration in the worst case
	fired := false
	for i := 0; i < maxIterations; i++ {
		// Best-effort: a poll may return immediately if there's no task ready.
		if _, perr := poller.PollAndProcessDecisionTask(false, false); perr != nil {
			s.Logger.Info("decision-task poll returned", tag.Error(perr))
		}

		descCtx, descCancel := createContext()
		desc, derr := s.Engine.DescribeSchedule(descCtx, &types.DescribeScheduleRequest{
			Domain:     s.DomainName,
			ScheduleID: scheduleID,
		})
		descCancel()
		if derr != nil {
			s.Logger.Warn("DescribeSchedule failed during smoke test", tag.Error(derr))
			continue
		}
		s.Logger.Info("DescribeSchedule snapshot during smoke test",
			tag.Counter(int(desc.GetInfo().GetTotalRuns())))
		if desc.GetInfo().GetTotalRuns() >= 1 {
			fired = true
			break
		}
	}
	s.True(fired, "schedule did not fire any target workflow within the polling window")

	// Clean up. We don't assert on post-delete describe behavior — the
	// scheduler workflow signals delete and exits asynchronously, so a follow-up
	// describe may briefly succeed before returning EntityNotExists.
	delCtx, delCancel := createContext()
	defer delCancel()
	_, err = s.Engine.DeleteSchedule(delCtx, &types.DeleteScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
	})
	s.NoError(err, "DeleteSchedule failed")
}
