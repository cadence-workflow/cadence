package host

import (
	"sort"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/types"
)

// These tests run an arbitrary code block guarded by a semaphore through the onebox cluster: real
// frontend, History and Matching services, with Cassandra. Semaphores are turned on with
// system.enableDistributedSemaphore in testdata/dynamicconfig/integration_test.yaml.
//
// The Go client has no semaphore support yet, so semaphoreWorkflow plays the SDK's part. A
// workflow written as
//
//	sem.acquire()
//	activity()
//	sem.release()
//
// reaches the server as decisions, and semaphoreWorkflow sends the same ones:
//   - sem.acquire(): an AcquireSemaphore decision.
//   - activity(): a ScheduleActivityTask decision, once SemaphoreAcquired is in the history.
//   - sem.release(): a ReleaseSemaphore decision, once the activity completes.

const (
	semaphoreWorkflowType            = "semaphore-workflow"
	semaphoreTimeoutWorkflowType     = "semaphore-timeout-workflow"
	semaphoreAutoReleaseWorkflowType = "semaphore-auto-release-workflow"
	semaphoreActivityType            = "semaphore-activity"
)

// TestSemaphore_SLOW runs workflows that acquire a semaphore, run an activity while they hold the
// token, then release it.
//   - Setup: a semaphore with 2 tokens, and 5 workflows.
//   - 2 workflows hold different tokens while 3 wait.
//   - A workflow that waits longer than its wait timeout gets SemaphoreAcquireFailed.
//   - All 5 finish, and at most 2 ever hold a token at once.
//
// It is named _SLOW because it waits out a 1-second acquire timeout, which leaves little room in
// the default 5-second limit.
func (s *IntegrationSuite) TestSemaphore_SLOW() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("semaphores are only supported on Cassandra")
	}
	const (
		capacity     = 2
		numWorkflows = 5
	)
	ctx, cancel := createContext()
	defer cancel()

	// Step 1: create a semaphore with 2 tokens.
	semaphoreName := s.RandomizeStr("semaphore")
	_, err := s.Engine.CreateSemaphore(ctx, &types.CreateSemaphoreRequest{
		Domain:        s.DomainName,
		SemaphoreName: semaphoreName,
		Capacity:      capacity,
	})
	s.NoError(err)

	// Step 2: start the worker. The activity blocks until the test closes unblockActivities. Until
	// then, the 2 workflows that got a token can't release it, so the test can check that the
	// other 3 are still waiting.
	unblockActivities := make(chan struct{})
	activity := activityTaskHandler(func(*types.WorkflowExecution, *types.ActivityType, string, []byte, []byte) ([]byte, bool, error) {
		<-unblockActivities
		return nil, false, nil
	})
	poller := s.newPoller(semaphoreWorkflow(semaphoreName), activity)
	cancelDecisionPoller := poller.PollAndProcessDecisions()
	defer cancelDecisionPoller()
	// One activity poller per workflow, so the pollers never limit how many activities run.
	for i := 0; i < numWorkflows; i++ {
		cancelActivityPoller := poller.PollAndProcessActivities()
		defer cancelActivityPoller()
	}

	// Step 3: start 5 workflows. 2 get a token and 3 wait for one.
	var executions []*types.WorkflowExecution
	for i := 0; i < numWorkflows; i++ {
		executions = append(executions, s.startSemaphoreWorkflow(semaphoreWorkflowType, poller.TaskList))
	}
	// Acquiring happens in the background, so check the histories until exactly 2 workflows hold a
	// token and 3 are waiting. No one releases yet, because every activity is still blocked.
	var heldTokens []int32
	s.Eventually(func() bool {
		heldTokens = nil
		waiting := 0
		for _, execution := range executions {
			events := s.getHistory(s.DomainName, execution)
			if acquired := findEvent(events, types.EventTypeSemaphoreAcquired); acquired != nil {
				// Holding a token.
				heldTokens = append(heldTokens, acquired.SemaphoreAcquiredEventAttributes.GetTokenID())
			} else if findEvent(events, types.EventTypeSemaphoreAcquireInitiated) != nil {
				// Asked for a token and still waiting.
				waiting++
			}
		}
		return len(heldTokens) == capacity && waiting == numWorkflows-capacity
	}, 5*time.Second, 100*time.Millisecond, "2 workflows hold a token and 3 wait")
	s.ElementsMatch([]int32{1, 2}, heldTokens, "the 2 holders have different tokens")

	// Step 4: while the semaphore is full, a workflow that waits at most 1 second gives up.
	timedOut := s.startSemaphoreWorkflow(semaphoreTimeoutWorkflowType, poller.TaskList)
	s.waitForWorkflowCompleted(timedOut)
	failed := findEvent(s.getHistory(s.DomainName, timedOut), types.EventTypeSemaphoreAcquireFailed)
	s.NotNil(failed)
	s.Equal(types.SemaphoreAcquireFailedCauseTimedOut, failed.SemaphoreAcquireFailedEventAttributes.GetCause())

	// Step 5: let the activities finish. Each release frees a token for a waiting workflow, so all 5
	// finish.
	close(unblockActivities)
	for _, execution := range executions {
		s.waitForWorkflowCompleted(execution)
	}

	// Step 6: from the histories, at most 2 workflows ever held a token at once.
	var histories [][]*types.HistoryEvent
	for _, execution := range executions {
		histories = append(histories, s.getHistory(s.DomainName, execution))
	}
	s.Equal(capacity, maxConcurrentHolds(histories))
}

// TestSemaphoreAutoRelease checks that a run's token is freed when the run closes without
// releasing it.
//   - Setup: a semaphore with 1 token.
//   - The first workflow acquires the token and completes without releasing it.
//   - The second workflow then gets the token, which it can only do if the first one's close
//     freed it.
func (s *IntegrationSuite) TestSemaphoreAutoRelease() {
	if TestFlags.PersistenceType != config.StoreTypeCassandra {
		s.T().Skip("semaphores are only supported on Cassandra")
	}
	ctx, cancel := createContext()
	defer cancel()

	// Step 1: create a semaphore with 1 token.
	semaphoreName := s.RandomizeStr("semaphore")
	_, err := s.Engine.CreateSemaphore(ctx, &types.CreateSemaphoreRequest{
		Domain:        s.DomainName,
		SemaphoreName: semaphoreName,
		Capacity:      1,
	})
	s.NoError(err)

	// Step 2: start the worker.
	poller := s.newPoller(semaphoreWorkflow(semaphoreName), nil)
	cancelDecisionPoller := poller.PollAndProcessDecisions()
	defer cancelDecisionPoller()

	// Step 3: the first workflow acquires the token and completes without releasing it.
	first := s.startSemaphoreWorkflow(semaphoreAutoReleaseWorkflowType, poller.TaskList)
	s.waitForWorkflowCompleted(first)
	firstHistory := s.getHistory(s.DomainName, first)
	s.NotNil(findEvent(firstHistory, types.EventTypeSemaphoreAcquired))
	// Auto-release frees the token with a release task, without a SemaphoreReleased event.
	s.Nil(findEvent(firstHistory, types.EventTypeSemaphoreReleased))

	// Step 4: the second workflow gets the token the first one's close freed.
	second := s.startSemaphoreWorkflow(semaphoreAutoReleaseWorkflowType, poller.TaskList)
	s.waitForWorkflowCompleted(second)
	acquired := findEvent(s.getHistory(s.DomainName, second), types.EventTypeSemaphoreAcquired)
	s.NotNil(acquired, "the second workflow gets the token the first one held")
	s.Equal(int32(1), acquired.SemaphoreAcquiredEventAttributes.GetTokenID())
}

// semaphoreWorkflow sends the decisions for sem.acquire(), activity() and sem.release(). Each
// decision task reacts to the events added since the previous one.
//   - Timeout workflow: waits at most 1 second for a token.
//   - Auto-release workflow: completes as soon as it gets the token.
func semaphoreWorkflow(semaphoreName string) decisionTaskHandler {
	return func(_ *types.WorkflowExecution, workflowType *types.WorkflowType, previousStartedEventID, _ int64, history *types.History) ([]byte, []*types.Decision, error) {
		complete := &types.Decision{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{},
		}
		for _, event := range history.Events[previousStartedEventID:] {
			switch event.GetEventType() {
			case types.EventTypeWorkflowExecutionStarted:
				// sem.acquire()
				var waitTimeoutSeconds *int32
				if workflowType.GetName() == semaphoreTimeoutWorkflowType {
					waitTimeoutSeconds = common.Int32Ptr(1)
				}
				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeAcquireSemaphore.Ptr(),
					AcquireSemaphoreDecisionAttributes: &types.AcquireSemaphoreDecisionAttributes{
						SemaphoreName:      semaphoreName,
						WaitTimeoutSeconds: waitTimeoutSeconds,
					},
				}}, nil
			case types.EventTypeSemaphoreAcquired:
				if workflowType.GetName() == semaphoreAutoReleaseWorkflowType {
					return nil, []*types.Decision{complete}, nil
				}
				// activity()
				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
					ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
						ActivityID:                    "1",
						ActivityType:                  &types.ActivityType{Name: semaphoreActivityType},
						ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
						ScheduleToStartTimeoutSeconds: common.Int32Ptr(100),
						StartToCloseTimeoutSeconds:    common.Int32Ptr(100),
					},
				}}, nil
			case types.EventTypeActivityTaskCompleted:
				// sem.release(), then complete.
				acquired := findEvent(history.Events, types.EventTypeSemaphoreAcquired)
				return nil, []*types.Decision{
					{
						DecisionType: types.DecisionTypeReleaseSemaphore.Ptr(),
						ReleaseSemaphoreDecisionAttributes: &types.ReleaseSemaphoreDecisionAttributes{
							InitiatedEventID: acquired.SemaphoreAcquiredEventAttributes.InitiatedEventID,
						},
					},
					complete,
				}, nil
			case types.EventTypeSemaphoreAcquireFailed:
				return nil, []*types.Decision{complete}, nil
			}
		}
		return nil, nil, nil
	}
}

func (s *IntegrationSuite) startSemaphoreWorkflow(workflowType string, taskList *types.TaskList) *types.WorkflowExecution {
	workflowID := s.RandomizeStr(workflowType)
	ctx, cancel := createContext()
	defer cancel()
	resp, err := s.Engine.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          workflowID,
		WorkflowType:                        &types.WorkflowType{Name: workflowType},
		TaskList:                            taskList,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            "semaphore-test",
	})
	s.NoError(err)
	return &types.WorkflowExecution{WorkflowID: workflowID, RunID: resp.GetRunID()}
}

// waitForWorkflowCompleted waits until the last event in execution's history is
// WorkflowExecutionCompleted.
func (s *IntegrationSuite) waitForWorkflowCompleted(execution *types.WorkflowExecution) {
	s.Eventually(func() bool {
		events := s.getHistory(s.DomainName, execution)
		return events[len(events)-1].GetEventType() == types.EventTypeWorkflowExecutionCompleted
	}, 5*time.Second, 100*time.Millisecond, "workflow %v completes", execution.WorkflowID)
}

func findEvent(events []*types.HistoryEvent, eventType types.EventType) *types.HistoryEvent {
	for _, event := range events {
		if event.GetEventType() == eventType {
			return event
		}
	}
	return nil
}

// maxConcurrentHolds returns the most workflows holding a token at once, from the times of their
// SemaphoreAcquired and SemaphoreReleased events.
func maxConcurrentHolds(histories [][]*types.HistoryEvent) int {
	type change struct {
		time  int64
		delta int
	}
	var changes []change
	for _, events := range histories {
		if acquired := findEvent(events, types.EventTypeSemaphoreAcquired); acquired != nil {
			changes = append(changes, change{time: acquired.GetTimestamp(), delta: 1})
		}
		if released := findEvent(events, types.EventTypeSemaphoreReleased); released != nil {
			changes = append(changes, change{time: released.GetTimestamp(), delta: -1})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].time < changes[j].time })
	holds, maxHolds := 0, 0
	for _, c := range changes {
		holds += c.delta
		maxHolds = max(maxHolds, holds)
	}
	return maxHolds
}
