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

// TestScheduleSmokeTest verifies the schedule pipeline end-to-end against the
// integration test cluster: CreateSchedule, wait for the scheduler workflow to
// fire at least one target run, DeleteSchedule.
func (s *IntegrationSuite) TestScheduleSmokeTest() {
	if s.TestClusterConfig.WorkerConfig == nil || !s.TestClusterConfig.WorkerConfig.EnableScheduler {
		s.T().Skip("scheduler worker manager not enabled on this cluster")
	}

	scheduleID := s.RandomizeStr("smoke-schedule")
	targetWorkflowType := "smoke-schedule-target"
	targetTaskList := s.RandomizeStr("smoke-schedule-tasklist")
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

	// Drain target decision tasks in the background so fired workflows
	// don't stay open.
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
			if _, perr := poller.PollAndProcessDecisionTask(false, false); perr != nil {
				s.Logger.Info("decision-task poll returned", tag.Error(perr))
			}
		}
	}()
	defer func() {
		close(done)
		pollerWG.Wait()
	}()

	// Budget: ~2s manager refresh + one cron interval + StartWorkflowExecution.
	const (
		fireDeadline = 30 * time.Second
		pollInterval = 1 * time.Second
	)
	timeout := time.NewTimer(fireDeadline)
	defer timeout.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fired := false
	for !fired {
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
		select {
		case <-ticker.C:
		case <-timeout.C:
			s.Failf("schedule did not fire", "no target workflow fired within %s", fireDeadline)
			return
		}
	}
}
