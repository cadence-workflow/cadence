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
	"sync"
	"time"

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
//
// Polling design: the target task list is drained in a background goroutine so
// the main test loop only does cheap DescribeSchedule calls. PollForDecisionTask
// long-polls up to 90s each call, so polling inline would dominate the test
// runtime and starve other parallel tests in the IntegrationSuite of task-list
// capacity.
func (s *IntegrationSuite) TestScheduleSmokeTest() {
	scheduleID := "smoke-schedule"
	targetWorkflowType := "smoke-schedule-target"
	targetTaskList := "smoke-schedule-tasklist"
	identity := "schedule-smoke-test"

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
	_, err := s.Engine.CreateSchedule(createCtx, createReq)
	createCancel()
	s.Require().NoError(err, "CreateSchedule failed")

	// Always tear the schedule down at the end so we don't leak a per-domain
	// scheduler workflow that keeps firing across subsequent tests.
	defer func() {
		delCtx, delCancel := createContext()
		defer delCancel()
		if _, derr := s.Engine.DeleteSchedule(delCtx, &types.DeleteScheduleRequest{
			Domain:     s.DomainName,
			ScheduleID: scheduleID,
		}); derr != nil {
			s.Logger.Warn("DeleteSchedule (cleanup) failed", tag.Error(derr))
		}
	}()

	// Background poller: drains decision tasks for the target task list until
	// the test signals done. Each scheduler-fired target needs its first
	// decision processed so it doesn't sit open consuming history; without this
	// the scheduler still counts a successful start (TotalRuns advances) but
	// the fixture leaks fired-but-stuck workflows.
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

	done := make(chan struct{})
	var pollerWG sync.WaitGroup
	pollerWG.Add(1)
	go func() {
		defer pollerWG.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			// PollAndProcessDecisionTask uses a 90s context internally and
			// returns whenever a task arrives or the long-poll deadline elapses.
			// We don't care about per-poll errors (e.g. empty-task-list timeout);
			// the goroutine just keeps going until done is closed.
			if _, perr := poller.PollAndProcessDecisionTask(false, false); perr != nil {
				s.Logger.Info("decision-task poll returned", tag.Error(perr))
			}
		}
	}()
	defer func() {
		close(done)
		pollerWG.Wait()
	}()

	// Wait for the schedule to fire at least once. Worst case timing budget:
	//   - up to 60s for the scheduler worker manager periodic refresh to
	//     discover the per-test domain (test domains are registered in
	//     SetupSuite *after* the cluster boots),
	//   - then `@every 5s` until the first cron fire,
	//   - then a few seconds for StartWorkflowExecution + the activity round-trip.
	// 120s gives comfortable headroom; describe is a cheap query, so polling
	// every 2s is fine.
	const (
		fireDeadline = 120 * time.Second
		pollInterval = 2 * time.Second
	)
	deadline := time.Now().Add(fireDeadline)
	fired := false
	for time.Now().Before(deadline) {
		descCtx, descCancel := createContext()
		desc, derr := s.Engine.DescribeSchedule(descCtx, &types.DescribeScheduleRequest{
			Domain:     s.DomainName,
			ScheduleID: scheduleID,
		})
		descCancel()
		if derr != nil {
			s.Logger.Warn("DescribeSchedule failed during smoke test", tag.Error(derr))
		} else if desc.GetInfo().GetTotalRuns() >= 1 {
			s.Logger.Info("schedule fired",
				tag.Counter(int(desc.GetInfo().GetTotalRuns())))
			fired = true
			break
		}
		time.Sleep(pollInterval)
	}
	s.True(fired, "schedule did not fire any target workflow within %s", fireDeadline)
}
