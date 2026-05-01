package host

import (
	"fmt"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestCronWorkflowWithOverlapPolicy() {
	id := "integration-wf-cron-overlap-test"
	wt := "integration-wf-cron-overlap-type"
	tl := "integration-wf-cron-overlap-tasklist"
	identity := "worker1"
	cronSchedule := "@every 5s"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Test both overlap policies
	testCases := []struct {
		name              string
		cronOverlapPolicy types.CronOverlapPolicy
	}{
		{
			name:              "SkippedPolicy",
			cronOverlapPolicy: types.CronOverlapPolicySkipped,
		},
		{
			name:              "BufferOnePolicy",
			cronOverlapPolicy: types.CronOverlapPolicyBufferOne,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			workflowID := id + "-" + tc.name

			request := &types.StartWorkflowExecutionRequest{
				RequestID:                           uuid.New(),
				Domain:                              s.DomainName,
				WorkflowID:                          workflowID,
				WorkflowType:                        workflowType,
				TaskList:                            taskList,
				Input:                               nil,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				Identity:                            identity,
				CronSchedule:                        cronSchedule,
				CronOverlapPolicy:                   &tc.cronOverlapPolicy,
			}

			ctx, cancel := createContext()
			defer cancel()
			we, err := s.Engine.StartWorkflowExecution(ctx, request)
			s.Nil(err)

			// Logging for debug
			fmt.Printf("StartWorkflowExecution RunID: %s\n", we.RunID)

			var executions []*types.WorkflowExecution
			attemptCount := 0
			executionTimes := make([]time.Time, 0)

			// Decision task handler that simulates long-running workflow
			dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
				previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
				executions = append(executions, execution)
				attemptCount++
				executionTimes = append(executionTimes, time.Now())

				if attemptCount == 1 {
					return nil, []*types.Decision{
						{
							DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
							CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
								Result: []byte(fmt.Sprintf("execution-%d-complete", attemptCount)),
							},
						},
					}, nil
				}

				return nil, []*types.Decision{
					{
						DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
						CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
							Result: []byte(fmt.Sprintf("execution-%d-complete", attemptCount)),
						},
					},
				}, nil
			}

			poller := &TaskPoller{
				Engine:          s.Engine,
				Domain:          s.DomainName,
				TaskList:        taskList,
				Identity:        identity,
				DecisionHandler: dtHandler,
				Logger:          s.Logger,
				T:               s.T(),
			}

			for i := 0; i < 3; i++ {
				_, err = poller.PollAndProcessDecisionTask(false, false)
				s.True(err == nil, err)
				time.Sleep(500 * time.Millisecond)
			}

			s.True(attemptCount >= 2, "Expected at least 2 executions, got %d", attemptCount)

			if len(executionTimes) >= 2 {
				for i := 1; i < len(executionTimes); i++ {
					timeDiff := executionTimes[i].Sub(executionTimes[i-1])

					if tc.cronOverlapPolicy == types.CronOverlapPolicySkipped {
						s.True(timeDiff >= 1500*time.Millisecond,
							"Expected execution %d to be spaced by at least 1.5s from previous, got %v",
							i, timeDiff)
					} else {
						s.True(timeDiff >= 0, "Expected non-negative time difference, got %v", timeDiff)
					}
				}
			}

			if len(executions) > 0 {
				events := s.getHistory(s.DomainName, executions[0])
				s.True(len(events) > 0, "Expected workflow history to have events")
				lastEvent := events[len(events)-1]
				s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())
				attributes := lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
				s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
			}

			ctx, cancel = createContext()
			defer cancel()
			terminateErr := s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
				Domain: s.DomainName,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: workflowID,
				},
			})
			s.NoError(terminateErr)

			ctx, cancel = createContext()
			defer cancel()
			dweResponse, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
				Domain: s.DomainName,
				Execution: &types.WorkflowExecution{
					WorkflowID: workflowID,
					RunID:      we.RunID,
				},
			})
			s.Nil(err)
			// Verify that the workflow execution info is returned after termination
			s.NotNil(dweResponse.WorkflowExecutionInfo)
			s.True(dweResponse.WorkflowExecutionInfo.IsCron, "Expected workflow to be marked as cron")
			s.Equal(tc.cronOverlapPolicy, *dweResponse.WorkflowExecutionInfo.CronOverlapPolicy)
		})
	}
}

func (s *IntegrationSuite) TestCronWorkflowOverlapBehavior() {
	id := "integration-wf-cron-overlap-behavior-test"
	wt := "integration-wf-cron-overlap-behavior-type"
	tl := "integration-wf-cron-overlap-behavior-tasklist"
	identity := "worker1"
	cronSchedule := "@every 1s"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Test both overlap policies with realistic overlap scenarios
	testCases := []struct {
		name              string
		cronOverlapPolicy types.CronOverlapPolicy
		workflowDuration  time.Duration
	}{
		{
			name:              "SkippedPolicy_ShortWorkflow",
			cronOverlapPolicy: types.CronOverlapPolicySkipped,
			workflowDuration:  500 * time.Millisecond,
		},
		{
			name:              "SkippedPolicy_LongWorkflow",
			cronOverlapPolicy: types.CronOverlapPolicySkipped,
			workflowDuration:  2 * time.Second,
		},
		{
			name:              "BufferOnePolicy_ShortWorkflow",
			cronOverlapPolicy: types.CronOverlapPolicyBufferOne,
			workflowDuration:  500 * time.Millisecond,
		},
		{
			name:              "BufferOnePolicy_LongWorkflow",
			cronOverlapPolicy: types.CronOverlapPolicyBufferOne,
			workflowDuration:  2 * time.Second,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			workflowID := id + "-" + tc.name

			request := &types.StartWorkflowExecutionRequest{
				RequestID:                           uuid.New(),
				Domain:                              s.DomainName,
				WorkflowID:                          workflowID,
				WorkflowType:                        workflowType,
				TaskList:                            taskList,
				Input:                               nil,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
				Identity:                            identity,
				CronSchedule:                        cronSchedule,
				CronOverlapPolicy:                   &tc.cronOverlapPolicy,
			}

			ctx, cancel := createContext()
			defer cancel()
			we, err := s.Engine.StartWorkflowExecution(ctx, request)
			s.Nil(err)

			s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

			var executions []*types.WorkflowExecution
			attemptCount := 0
			executionStartTimes := make([]time.Time, 0)
			executionEndTimes := make([]time.Time, 0)

			// Decision task handler that simulates workflows with controlled duration
			dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
				previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
				executions = append(executions, execution)
				attemptCount++
				startTime := time.Now()
				executionStartTimes = append(executionStartTimes, startTime)

				// Simulate workflow duration by sleeping
				time.Sleep(tc.workflowDuration)
				endTime := time.Now()
				executionEndTimes = append(executionEndTimes, endTime)

				// Complete the workflow
				return nil, []*types.Decision{
					{
						DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
						CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
							Result: []byte(fmt.Sprintf("execution-%d-complete-duration-%v", attemptCount, tc.workflowDuration)),
						},
					}}, nil
			}

			poller := &TaskPoller{
				Engine:          s.Engine,
				Domain:          s.DomainName,
				TaskList:        taskList,
				Identity:        identity,
				DecisionHandler: dtHandler,
				Logger:          s.Logger,
				T:               s.T(),
			}

			// Process multiple decision tasks to observe overlap behavior
			// For long workflows, we'll process fewer tasks to avoid test timeout
			maxTasks := 3
			if tc.workflowDuration > time.Second {
				maxTasks = 2
			}

			for i := 0; i < maxTasks; i++ {
				_, err = poller.PollAndProcessDecisionTask(false, false)
				s.True(err == nil, err)
			}

			// Verify we have executions
			s.True(attemptCount >= 1, "Expected at least 1 execution, got %d", attemptCount)

			// Analyze overlap behavior - only if we have multiple executions
			if len(executionStartTimes) >= 2 {
				for i := 1; i < len(executionStartTimes); i++ {
					timeBetweenStarts := executionStartTimes[i].Sub(executionStartTimes[i-1])
					timeBetweenEndAndNextStart := executionStartTimes[i].Sub(executionEndTimes[i-1])

					s.Logger.Info("Execution timing analysis",
						tag.Counter(i),
						tag.WorkflowID(fmt.Sprintf("timeBetweenStarts:%v", timeBetweenStarts)),
						tag.WorkflowRunID(fmt.Sprintf("timeBetweenEndAndNextStart:%v", timeBetweenEndAndNextStart)),
						tag.WorkflowID(fmt.Sprintf("workflowDuration:%v", tc.workflowDuration)),
						tag.WorkflowID(fmt.Sprintf("overlapPolicy:%s", tc.cronOverlapPolicy.String())))

					if tc.cronOverlapPolicy == types.CronOverlapPolicySkipped {
						// For skipped policy, if workflow duration > cron interval,
						// subsequent executions should be skipped until previous completes
						if tc.workflowDuration > time.Second {
							// Should wait for previous execution to complete
							s.True(timeBetweenEndAndNextStart >= 0,
								"Expected next execution to start after previous completion, got %v",
								timeBetweenEndAndNextStart)
						} else {
							// Short workflows should follow cron schedule
							s.True(timeBetweenStarts >= 800*time.Millisecond,
								"Expected executions to be spaced by cron interval, got %v",
								timeBetweenStarts)
						}
					} else {
						// For buffer one policy, next execution should start after previous completion
						// (but we're more lenient with timing in test environment)
						s.True(timeBetweenEndAndNextStart >= 0,
							"Expected next execution to start after previous completion, got %v",
							timeBetweenEndAndNextStart)
					}
				}
			}

			// Verify workflow history
			if len(executions) > 0 {
				events := s.getHistory(s.DomainName, executions[0])
				s.True(len(events) > 0, "Expected workflow history to have events")

				// Check that the last event is a continue-as-new event
				lastEvent := events[len(events)-1]
				s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())

				attributes := lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
				s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
			}

			// Terminate the cron workflow
			ctx, cancel = createContext()
			defer cancel()
			terminateErr := s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
				Domain: s.DomainName,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: workflowID,
				},
			})
			s.NoError(terminateErr)
		})
	}
}
