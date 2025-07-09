package host

import (
	"context"
	"errors"
	"flag"
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

const (
	tl = "integration-task-list-tl"
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
		dynamicproperties.EnableTasklistIsolation:            false,
		dynamicproperties.MatchingNumTasklistWritePartitions: 1,
		dynamicproperties.MatchingNumTasklistReadPartitions:  1,
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
}

func (s *TaskListIntegrationSuite) TestDescribeTaskList_Status() {
	expectedTl := &types.TaskList{Name: tl, Kind: types.TaskListKindNormal.Ptr()}

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
	response := s.describeTaskList()
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
	poller := s.createPoller("result")
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()
	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")

	response = s.describeTaskList()
	response.TaskListStatus.IsolationGroupMetrics = nil
	response.TaskListStatus.NewTasksPerSecond = 0
	s.Equal(completedStatus, response.TaskListStatus)
}

func (s *TaskListIntegrationSuite) TestEphemeralTaskList() {
	runID := s.startWorkflow(types.TaskListKindEphemeral).RunID

	response := s.describeWorkflow(runID)
	s.Equal(types.TaskListKindEphemeral, response.WorkflowExecutionInfo.TaskList.GetKind())

	firstEvent := s.getStartedEvent(runID)
	s.Equal(types.TaskListKindEphemeral, firstEvent.TaskList.GetKind())

	// Finish the workflow, although it doesn't dispatch its tasks as Ephemeral yet
	poller := s.createPoller("result")
	cancelPoller := poller.PollAndProcessDecisions()
	defer cancelPoller()
	_, err := s.getWorkflowResult(runID)
	s.NoError(err, "failed to get workflow result")
}

func (s *TaskListIntegrationSuite) createPoller(identity string) *TaskPoller {
	return &TaskPoller{
		Engine:   s.Engine,
		Domain:   s.DomainName,
		TaskList: &types.TaskList{Name: tl, Kind: types.TaskListKindNormal.Ptr()},
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
			Name: tl,
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
				Name: tl,
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

func (s *TaskListIntegrationSuite) describeTaskList() *types.DescribeTaskListResponse {
	ctx, cancel := createContext()
	defer cancel()
	result, err := s.Engine.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
		Domain:       s.DomainName,
		TaskListType: types.TaskListTypeDecision.Ptr(),
		TaskList: &types.TaskList{
			Name: tl,
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
