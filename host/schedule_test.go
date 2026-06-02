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
	"strings"
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

// TestScheduleUnpauseCatchUp verifies UnpauseSchedule catch-up semantics end-to-end:
// when a schedule is paused across several cron boundaries and then unpaused, the
// number of *missed* fires replayed must match the catch-up policy —
//
//	Skip -> 0, One -> exactly 1 (most recent missed), All -> all missed (>1).
//
// Caught-up fires are distinguished from normal post-unpause fires by their
// scheduled time, which is encoded in the target WorkflowID
// (generateWorkflowID -> "<scheduleID>-<RFC3339 scheduledTime>"). A fire is
// "caught-up" iff its scheduled time falls within the pause window
// [pauseTime, unpauseTime).
//
// NOTE: this asserts the *intended* behavior. If catch-up is broken (it replayed
// nothing in dev-cluster CLI testing — "OBS-CatchUp"), the One/All cases fail
// here, which is the point: it surfaces the bug on current code. Timing constants
// (pause window, drain window) may need tuning on slow CI.
func (s *IntegrationSuite) TestScheduleUnpauseCatchUp() {
	if s.TestClusterConfig.WorkerConfig == nil || !s.TestClusterConfig.WorkerConfig.EnableScheduler {
		s.T().Skip("scheduler worker manager not enabled on this cluster")
	}

	cases := []struct {
		name   string
		policy types.ScheduleCatchUpPolicy
	}{
		{"Skip", types.ScheduleCatchUpPolicySkip},
		{"One", types.ScheduleCatchUpPolicyOne},
		{"All", types.ScheduleCatchUpPolicyAll},
	}

	for _, tc := range cases {
		caughtUp := s.runCatchUpCase(tc.policy)
		s.T().Logf("catch-up policy=%s caught-up fires=%d", tc.name, caughtUp)
		switch tc.policy {
		case types.ScheduleCatchUpPolicySkip:
			s.Equal(0, caughtUp, "Skip must replay no missed fires")
		case types.ScheduleCatchUpPolicyOne:
			s.Equal(1, caughtUp, "One must replay exactly the most recent missed fire")
		case types.ScheduleCatchUpPolicyAll:
			s.GreaterOrEqual(caughtUp, 2, "All must replay every missed fire (>1)")
		}
	}
}

// runCatchUpCase creates an @every-5s schedule, pauses it immediately, lets
// several boundaries pass, unpauses it, drains fired target workflows with a
// poller, and returns how many of them were *caught-up* (scheduled within the
// pause window). The schedule is deleted on return.
func (s *IntegrationSuite) runCatchUpCase(policy types.ScheduleCatchUpPolicy) int {
	scheduleID := s.RandomizeStr("catchup-schedule")
	targetWorkflowType := "catchup-target"
	targetTaskList := s.RandomizeStr("catchup-tasklist")
	identity := "schedule-catchup-test"

	createReq := &types.CreateScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
		Spec:       &types.ScheduleSpec{CronExpression: "@every 5s"},
		Action: &types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType:                        &types.WorkflowType{Name: targetWorkflowType},
				TaskList:                            &types.TaskList{Name: targetTaskList},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
			},
		},
		Policies: &types.SchedulePolicies{
			// Concurrent so caught-up fires aren't skipped by overlap against a
			// still-running previous target; large window so missed fires aren't
			// dropped for being too old.
			OverlapPolicy: types.ScheduleOverlapPolicyConcurrent,
			CatchUpPolicy: policy,
			CatchUpWindow: time.Hour,
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

	// Pause as soon as possible so subsequent @every-5s boundaries are "missed".
	pauseTime := time.Now().UTC()
	pauseCtx, pauseCancel := createContext()
	_, err = s.Engine.PauseSchedule(pauseCtx, &types.PauseScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
		Reason:     "catchup-test",
	})
	pauseCancel()
	s.Require().NoError(err, "PauseSchedule failed")

	// Let several boundaries pass while paused (~4 at @every 5s).
	time.Sleep(22 * time.Second)

	// Poller records every fired target's WorkflowID and completes it.
	var mu sync.Mutex
	firedWIDs := make(map[string]struct{})
	completeAndRecord := func(execution *types.WorkflowExecution, _ *types.WorkflowType,
		_, _ int64, _ *types.History) ([]byte, []*types.Decision, error) {
		mu.Lock()
		firedWIDs[execution.GetWorkflowID()] = struct{}{}
		mu.Unlock()
		return nil, []*types.Decision{
			{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("catchup-target-complete"),
				},
			},
		}, nil
	}
	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        &types.TaskList{Name: targetTaskList},
		Identity:        identity,
		DecisionHandler: completeAndRecord,
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
				s.Logger.Info("catchup decision-task poll returned", tag.Error(perr))
			}
		}
	}()

	// Unpause and mark the catch-up / normal boundary.
	unpauseTime := time.Now().UTC()
	unpauseCtx, unpauseCancel := createContext()
	_, err = s.Engine.UnpauseSchedule(unpauseCtx, &types.UnpauseScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
	})
	unpauseCancel()
	s.Require().NoError(err, "UnpauseSchedule failed")

	// Drain the catch-up burst (and some normal fires) before stopping the poller.
	time.Sleep(20 * time.Second)
	close(done)
	pollerWG.Wait()

	// Caught-up = fires whose scheduled time is within the pause window. The
	// scheduled time is the RFC3339 suffix of the target WorkflowID.
	caughtUp := 0
	mu.Lock()
	defer mu.Unlock()
	for wid := range firedWIDs {
		tsStr := strings.TrimPrefix(wid, scheduleID+"-")
		ts, perr := time.Parse(time.RFC3339, tsStr)
		if perr != nil {
			s.Logger.Warn("could not parse scheduled time from target WorkflowID",
				tag.WorkflowID(wid), tag.Error(perr))
			continue
		}
		ts = ts.UTC()
		if !ts.Before(pauseTime) && ts.Before(unpauseTime) {
			caughtUp++
		}
	}
	return caughtUp
}
