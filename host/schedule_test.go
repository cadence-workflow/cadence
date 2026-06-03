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

// TestScheduleFullCreateRoundTrip creates a schedule with every optional field set
// and asserts DescribeSchedule returns them unchanged. It exercises the full
// CreateSchedule -> DescribeSchedule mapping path that the CLI cannot reach (the
// CLI only exposes cron/workflowType/taskList/timeouts/input/overlap/catchup/
// concurrencyLimit).
func (s *IntegrationSuite) TestScheduleFullCreateRoundTrip() {
	if s.TestClusterConfig.WorkerConfig == nil || !s.TestClusterConfig.WorkerConfig.EnableScheduler {
		s.T().Skip("scheduler worker manager not enabled on this cluster")
	}

	scheduleID := s.RandomizeStr("full-create")
	start := time.Now().Add(2 * time.Minute).Round(time.Second).UTC()
	end := start.Add(time.Hour)

	createReq := &types.CreateScheduleRequest{
		Domain:     s.DomainName,
		ScheduleID: scheduleID,
		Spec: &types.ScheduleSpec{
			CronExpression: "@every 30s",
			StartTime:      start,
			EndTime:        end,
			Jitter:         15 * time.Second,
		},
		Action: &types.ScheduleAction{
			StartWorkflow: &types.StartWorkflowAction{
				WorkflowType:                        &types.WorkflowType{Name: "full-create-target"},
				TaskList:                            &types.TaskList{Name: s.RandomizeStr("full-tl")},
				Input:                               []byte(`"hello"`),
				WorkflowIDPrefix:                    "wfpfx-",
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
				RetryPolicy: &types.RetryPolicy{
					InitialIntervalInSeconds:    1,
					BackoffCoefficient:          2.0,
					MaximumIntervalInSeconds:    10,
					MaximumAttempts:             3,
					ExpirationIntervalInSeconds: 100,
				},
				Memo: &types.Memo{Fields: map[string][]byte{"actionMemo": []byte(`"m"`)}},
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{"CustomKeywordField": []byte(`"sa"`)},
				},
			},
		},
		Policies: &types.SchedulePolicies{
			OverlapPolicy:    types.ScheduleOverlapPolicyBuffer,
			CatchUpPolicy:    types.ScheduleCatchUpPolicyOne,
			CatchUpWindow:    time.Hour,
			PauseOnFailure:   true,
			BufferLimit:      common.Int32Ptr(5),
			ConcurrencyLimit: common.Int32Ptr(0),
		},
		Memo: &types.Memo{Fields: map[string][]byte{"schedMemo": []byte(`"sm"`)}},
		SearchAttributes: &types.SearchAttributes{
			IndexedFields: map[string][]byte{"CustomIntField": []byte(`7`)},
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

	// DescribeSchedule queries the scheduler workflow, which must handle its first
	// decision task before it can be queried; retry briefly to ride out that window.
	var desc *types.DescribeScheduleResponse
	s.Require().Eventually(func() bool {
		c, cc := createContext()
		defer cc()
		d, e := s.Engine.DescribeSchedule(c, &types.DescribeScheduleRequest{
			Domain:     s.DomainName,
			ScheduleID: scheduleID,
		})
		if e != nil {
			s.Logger.Info("DescribeSchedule not ready yet", tag.Error(e))
			return false
		}
		desc = d
		return true
	}, 20*time.Second, time.Second, "DescribeSchedule never became queryable")

	// Spec round-trip.
	s.Equal("@every 30s", desc.GetSpec().GetCronExpression())
	s.WithinDuration(start, desc.GetSpec().GetStartTime(), time.Second)
	s.WithinDuration(end, desc.GetSpec().GetEndTime(), time.Second)
	s.Equal(15*time.Second, desc.GetSpec().GetJitter())

	// Action round-trip.
	sw := desc.GetAction().GetStartWorkflow()
	s.Require().NotNil(sw)
	s.Equal("full-create-target", sw.GetWorkflowType().GetName())
	s.Equal("wfpfx-", sw.GetWorkflowIDPrefix())
	s.Equal([]byte(`"hello"`), sw.GetInput())
	s.Equal(int32(60), sw.GetExecutionStartToCloseTimeoutSeconds())
	s.Equal(int32(10), sw.GetTaskStartToCloseTimeoutSeconds())
	s.Require().NotNil(sw.GetRetryPolicy())

	// Retry policy
	rp := sw.GetRetryPolicy()
	s.Equal(int32(1), rp.InitialIntervalInSeconds)
	s.InDelta(2.0, rp.BackoffCoefficient, 0.001)
	s.Equal(int32(10), rp.MaximumIntervalInSeconds)
	s.Equal(int32(3), rp.MaximumAttempts)
	s.Equal(int32(100), rp.ExpirationIntervalInSeconds)
	s.Require().NotNil(sw.GetMemo())
	s.Equal([]byte(`"m"`), sw.GetMemo().Fields["actionMemo"])
	s.Require().NotNil(sw.GetSearchAttributes())
	s.Equal([]byte(`"sa"`), sw.GetSearchAttributes().IndexedFields["CustomKeywordField"])

	// Policies round-trip, including the nullable limit fields.
	pol := desc.GetPolicies()
	s.Require().NotNil(pol)
	s.Equal(types.ScheduleOverlapPolicyBuffer, pol.GetOverlapPolicy())
	s.Equal(types.ScheduleCatchUpPolicyOne, pol.GetCatchUpPolicy())
	s.Equal(time.Hour, pol.GetCatchUpWindow())
	s.True(pol.GetPauseOnFailure())
	s.Require().NotNil(pol.GetBufferLimit())
	s.Equal(int32(5), *pol.GetBufferLimit())
	s.Require().NotNil(pol.GetConcurrencyLimit())
	s.Equal(int32(0), *pol.GetConcurrencyLimit())

	// Schedule-level Memo and SearchAttributes round-trip.
	s.Require().NotNil(desc.GetMemo())
	s.Equal([]byte(`"sm"`), desc.GetMemo().Fields["schedMemo"])
	s.Require().NotNil(desc.GetSearchAttributes())
	s.Equal([]byte(`7`), desc.GetSearchAttributes().IndexedFields["CustomIntField"])

	// Schedule starts active.
	s.False(desc.GetState().GetPaused())
}
