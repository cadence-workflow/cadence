package host

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func TestTaskListSuite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/task_list_test_cluster.yaml")
	if err != nil {
		panic(err)
	}
	testCluster := NewPersistenceTestCluster(t, clusterConfig)
	clusterConfig.FrontendDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableTasklistIsolation:            false,
		dynamicproperties.MatchingNumTasklistWritePartitions: 1,
		dynamicproperties.MatchingNumTasklistReadPartitions:  1,
	}
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableTasklistIsolation:            false,
		dynamicproperties.MatchingNumTasklistWritePartitions: 1,
		dynamicproperties.MatchingNumTasklistReadPartitions:  1,
	}
	clusterConfig.MatchingDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.EnableTasklistIsolation:              false,
		dynamicproperties.MatchingNumTasklistWritePartitions:   1,
		dynamicproperties.MatchingNumTasklistReadPartitions:    1,
		dynamicproperties.MatchingEnableReturnAllTaskListKinds: true,
	}

	s := new(TaskListIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *TaskListIntegrationSuite) SetupSuite() {
	s.setupSuite()
}

func (s *TaskListIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

func (s *TaskListIntegrationSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
	s.TaskListName = s.T().Name()
}

func (s *TaskListIntegrationSuite) TestDescribeTaskList_Status() {
	expectedTl := &types.TaskList{Name: s.TaskListName, Kind: types.TaskListKindNormal.Ptr()}

	initialStatus := &types.TaskListStatus{
		BacklogCountHint: 0,
		ReadLevel:        0,
		AckLevel:         0,
		RatePerSecond:    100000,
		TaskIDBlock: &types.TaskIDBlock{
			StartID: 1,
			EndID:   100000,
		},
		// Isolation is disabled
		// IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{},
		NewTasksPerSecond: 0,
		Empty:             true,
	}
	withOneTask := &types.TaskListStatus{
		BacklogCountHint: 1,
		ReadLevel:        1,
		AckLevel:         0,
		RatePerSecond:    100000,
		TaskIDBlock: &types.TaskIDBlock{
			StartID: 1,
			EndID:   100000,
		},
		// Isolation is disabled and NewTasksPerSecond is volatile
		// IsolationGroupMetrics: map[string]*types.IsolationGroupMetrics{},
		// NewTasksPerSecond:     0,
		Empty: false,
	}
	completedStatus := &types.TaskListStatus{
		BacklogCountHint: 0,
		ReadLevel:        1,
		AckLevel:         1,
		RatePerSecond:    100000,
		TaskIDBlock: &types.TaskIDBlock{
			StartID: 1,
			EndID:   100000,
		},
		// Isolation is disabled and NewTasksPerSecond is volatile
		// IsolationGroupMetrics: nil,
		// NewTasksPerSecond:     0,
		Empty: true,
	}

	// 0. Check before any tasks
	response := s.describeTaskList(types.TaskListTypeDecision)
	response.TaskListStatus.IsolationGroupMetrics = nil
	s.Equal(expectedTl, response.TaskList)
	s.Equal(initialStatus, response.TaskListStatus)

	// 1. After a task has been added but not yet completed
	runID := s.startWorkflow(types.TaskListKindNormal).RunID
	response = s.describeUntil(func(response *types.DescribeTaskListResponse) bool {
		return response.TaskListStatus.BacklogCountHint == 1
	})
	response.TaskListStatus.IsolationGroupMetrics = nil
	response.TaskListStatus.NewTasksPerSecond = 0
	s.Equal(withOneTask, response.TaskListStatus)

	// 2. After the task has been completed
	poller := s.createPoller("result", types.TaskListKindNormal)
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()
	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")

	response = s.describeTaskList(types.TaskListTypeDecision)
	response.TaskListStatus.IsolationGroupMetrics = nil
	response.TaskListStatus.NewTasksPerSecond = 0
	s.Equal(completedStatus, response.TaskListStatus)
}

// Run a Workflow containing only decision tasks on an ephemeral TaskList
func (s *TaskListIntegrationSuite) TestEphemeralTaskList() {
	runID := s.startWorkflow(types.TaskListKindEphemeral).RunID

	response := s.describeWorkflow(runID)
	s.Equal(types.TaskListKindEphemeral, response.WorkflowExecutionInfo.TaskList.GetKind())

	firstEvent := s.getStartedEvent(runID)
	s.Equal(types.TaskListKindEphemeral, firstEvent.TaskList.GetKind())

	taskList := s.waitUntilTaskListExists(types.TaskListTypeDecision)
	s.Equal(types.TaskListKindEphemeral, *taskList.TaskList.Kind)

	poller := s.createPoller("result", types.TaskListKindEphemeral)
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()
	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")
}

// Run a workflow on a normal TaskList, with an activity on an ephemeral TaskList
func (s *TaskListIntegrationSuite) TestEphemeralTaskList_EphemeralActivity() {
	activity := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID: "the-one",
		ActivityType: &types.ActivityType{
			Name: "activity-type",
		},
		Domain:                        s.DomainName,
		TaskList:                      &types.TaskList{Name: s.TaskListName, Kind: types.TaskListKindEphemeral.Ptr()},
		Input:                         []byte("input"),
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(10),
	}

	// Normal workflow with an ephemeral activity
	runID := s.startWorkflow(types.TaskListKindNormal).RunID

	taskList := s.waitUntilTaskListExists(types.TaskListTypeDecision)
	s.Equal(types.TaskListKindNormal, taskList.TaskList.GetKind())

	poller := s.createDecisionPollerForSingleActivityWorkflow(types.TaskListKindNormal, activity)
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()

	taskList = s.waitUntilTaskListExists(types.TaskListTypeActivity)
	s.Equal(types.TaskListKindEphemeral, taskList.TaskList.GetKind())

	activityPoller := s.createEchoActivityPoller(types.TaskListKindEphemeral)
	cancelActivityPoller := activityPoller.PollAndProcessActivities()
	defer cancelActivityPoller()

	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")

	scheduled := s.getActivityScheduledEvent(runID)
	s.Equal(types.TaskListKindEphemeral, scheduled.TaskList.GetKind())
}

// Run a workflow on an Ephemeral TaskList, which forces the activity to be dispatched to an Ephemeral TaskList.
func (s *TaskListIntegrationSuite) TestEphemeralTaskList_EphemeralWorkflow() {
	activity := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID: "the-one",
		ActivityType: &types.ActivityType{
			Name: "activity-type",
		},
		Domain:                        s.DomainName,
		TaskList:                      &types.TaskList{Name: s.TaskListName, Kind: types.TaskListKindNormal.Ptr()},
		Input:                         []byte("input"),
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(20),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(10),
	}

	// Normal workflow with an ephemeral activity
	runID := s.startWorkflow(types.TaskListKindEphemeral).RunID

	// Just for sanity
	response := s.describeWorkflow(runID)
	s.Equal(types.TaskListKindEphemeral, response.WorkflowExecutionInfo.TaskList.GetKind())

	taskList := s.waitUntilTaskListExists(types.TaskListTypeDecision)
	s.Equal(types.TaskListKindEphemeral, taskList.TaskList.GetKind())

	poller := s.createDecisionPollerForSingleActivityWorkflow(types.TaskListKindEphemeral, activity)
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()

	taskList = s.waitUntilTaskListExists(types.TaskListTypeActivity)
	s.Equal(types.TaskListKindEphemeral, taskList.TaskList.GetKind())

	activityPoller := s.createEchoActivityPoller(types.TaskListKindEphemeral)
	cancelActivityPoller := activityPoller.PollAndProcessActivities()
	defer cancelActivityPoller()

	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")

	scheduled := s.getActivityScheduledEvent(runID)
	s.Equal(types.TaskListKindEphemeral, scheduled.TaskList.GetKind())
}

func (s *TaskListIntegrationSuite) createEchoActivityPoller(tlKind types.TaskListKind) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   s.DomainName,
		TaskList: &types.TaskList{Name: s.TaskListName, Kind: tlKind.Ptr()},
		Identity: "activity-poller",
		ActivityHandler: func(execution *types.WorkflowExecution, activityType *types.ActivityType, ActivityID string, input []byte, takeToken []byte) ([]byte, bool, error) {
			return input, false, nil
		},
		Logger:      s.Logger,
		T:           s.T(),
		CallOptions: []yarpc.CallOption{},
	}
}

func (s *TaskListIntegrationSuite) createDecisionPollerForSingleActivityWorkflow(tlKind types.TaskListKind, activity *types.ScheduleActivityTaskDecisionAttributes) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   s.DomainName,
		TaskList: &types.TaskList{Name: s.TaskListName, Kind: tlKind.Ptr()},
		Identity: "decision-poller",
		DecisionHandler: func(execution *types.WorkflowExecution, wt *types.WorkflowType, previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
			// Ignore the DecisionTaskScheduled and DecisionTaskStarted that'll be at the end
			latestEvent := history.Events[len(history.Events)-3]
			// Started Event only
			if *latestEvent.EventType == types.EventTypeWorkflowExecutionStarted {
				return nil, []*types.Decision{{
					DecisionType:                           types.DecisionTypeScheduleActivityTask.Ptr(),
					ScheduleActivityTaskDecisionAttributes: activity,
				}}, nil
			}
			if *latestEvent.EventType == types.EventTypeActivityTaskCompleted {
				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
					CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
						Result: []byte("done"),
					},
				}}, nil
			}
			return nil, nil, fmt.Errorf("unexpected event type: %v", latestEvent.EventType)
		},
		Logger:      s.Logger,
		T:           s.T(),
		CallOptions: []yarpc.CallOption{},
	}
}

func (s *TaskListIntegrationSuite) createPoller(identity string, tlKind types.TaskListKind) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   s.DomainName,
		TaskList: &types.TaskList{Name: s.TaskListName, Kind: tlKind.Ptr()},
		Identity: identity,
		DecisionHandler: func(execution *types.WorkflowExecution, wt *types.WorkflowType, previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
			// Complete the workflow
			return []byte(strconv.Itoa(0)), []*types.Decision{{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte(identity),
				},
			}}, nil
		},
		Logger:      s.Logger,
		T:           s.T(),
		CallOptions: []yarpc.CallOption{},
	}
}

func (s *TaskListIntegrationSuite) startWorkflow(tlKind types.TaskListKind) *types.StartWorkflowExecutionResponse {
	identity := "test"

	request := &types.StartWorkflowExecutionRequest{
		RequestID:  uuid.New(),
		Domain:     s.DomainName,
		WorkflowID: s.T().Name(),
		WorkflowType: &types.WorkflowType{
			Name: "integration-task-list-type",
		},
		TaskList: &types.TaskList{
			Name: s.TaskListName,
			Kind: tlKind.Ptr(),
		},
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(10),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
	}

	ctx, cancel := createContext()
	defer cancel()
	result, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	return result
}

func (s *TaskListIntegrationSuite) describeWorkflow(runID string) *types.DescribeWorkflowExecutionResponse {
	request := &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: s.T().Name(),
			RunID:      runID,
		},
	}
	ctx, cancel := createContext()
	defer cancel()
	result, err := s.Engine.DescribeWorkflowExecution(ctx, request)
	s.Nil(err)

	return result
}

func (s *TaskListIntegrationSuite) describeUntil(condition func(response *types.DescribeTaskListResponse) bool) *types.DescribeTaskListResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			s.FailNow("timed out waiting for condition")
		default:
		}
		result, err := s.Engine.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
			Domain:       s.DomainName,
			TaskListType: types.TaskListTypeDecision.Ptr(),
			TaskList: &types.TaskList{
				Name: s.TaskListName,
				Kind: types.TaskListKindNormal.Ptr(),
			},
			IncludeTaskListStatus: true,
		})
		if err != nil {
			s.T().Log("failed to describe task list: %w", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if condition(result) {
			return result
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *TaskListIntegrationSuite) waitUntilTaskListExists(tlType types.TaskListType) *types.DescribeTaskListResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			s.FailNow("timed out waiting for TaskList")
		default:
		}
		result, err := s.Engine.GetTaskListsByDomain(ctx, &types.GetTaskListsByDomainRequest{
			Domain: s.DomainName,
		})
		if err != nil {
			s.T().Log("failed to describe task list: %w", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if tlResponse, ok := result.ActivityTaskListMap[s.TaskListName]; ok && tlType == types.TaskListTypeActivity {
			return tlResponse
		}
		if tlResponse, ok := result.DecisionTaskListMap[s.TaskListName]; ok && tlType == types.TaskListTypeDecision {
			return tlResponse
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *TaskListIntegrationSuite) describeTaskList(tlType types.TaskListType) *types.DescribeTaskListResponse {
	ctx, cancel := createContext()
	defer cancel()
	result, err := s.Engine.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
		Domain:       s.DomainName,
		TaskListType: tlType.Ptr(),
		TaskList: &types.TaskList{
			Name: s.TaskListName,
			Kind: types.TaskListKindNormal.Ptr(),
		},
		IncludeTaskListStatus: true,
	})
	s.NoError(err, "failed to describe task list")
	return result
}

func (s *TaskListIntegrationSuite) getStartedEvent(runID string) *types.WorkflowExecutionStartedEventAttributes {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: s.T().Name(),
			RunID:      runID,
		},
		HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
	})
	s.NoError(err, "failed to get workflow history")

	history := historyResponse.History

	firstEvent := history.Events[0]
	if *firstEvent.EventType != types.EventTypeWorkflowExecutionStarted {
		s.FailNow("expected first event to be WorkflowExecutionStarted")
	}

	return firstEvent.WorkflowExecutionStartedEventAttributes
}

func (s *TaskListIntegrationSuite) getActivityScheduledEvent(runID string) *types.ActivityTaskScheduledEventAttributes {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: s.T().Name(),
			RunID:      runID,
		},
		HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
	})
	s.NoError(err, "failed to get workflow history")

	history := historyResponse.History

	for _, event := range history.Events {
		if *event.EventType == types.EventTypeActivityTaskScheduled {
			return event.ActivityTaskScheduledEventAttributes
		}
	}
	s.FailNow("failed to find EventTypeActivityTaskScheduled")
	return nil
}

func (s *TaskListIntegrationSuite) getWorkflowResult(runID string) (string, error) {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: s.T().Name(),
			RunID:      runID,
		},
		HistoryEventFilterType: types.HistoryEventFilterTypeCloseEvent.Ptr(),
		WaitForNewEvent:        true,
	})
	if err != nil {
		return "", err
	}
	history := historyResponse.History

	lastEvent := history.Events[len(history.Events)-1]
	if *lastEvent.EventType != types.EventTypeWorkflowExecutionCompleted {
		return "", errors.New("workflow didn't complete")
	}

	return string(lastEvent.WorkflowExecutionCompletedEventAttributes.Result), nil
}
