// Copyright (c) 2017-2020 Uber Technologies Inc.

// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/isolationgroup/defaultisolationgroupstate"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/tasklist"
)

type (
	matchingEngineSuite struct {
		suite.Suite
		controller             *gomock.Controller
		mockHistoryClient      *history.MockClient
		mockDomainCache        *cache.MockDomainCache
		mockMembershipResolver *membership.MockResolver
		mockIsolationStore     *dynamicconfig.MockClient

		matchingEngine       *matchingEngineImpl
		taskManager          *tasklist.TestTaskManager
		isolationState       isolationgroup.State
		mockExecutionManager *mocks.ExecutionManager
		mockTimeSource       clock.MockedTimeSource
		logger               log.Logger
		handlerContext       *handlerContext
		sync.Mutex
	}
)

const (
	_minBurst              = 10000
	matchingTestDomainName = "matching-test"
	matchingTestTaskList   = "matching-test-tasklist"

	returnEmptyTaskTimeBudget       = time.Second
	_defaultTaskDispatchRPS         = 100000.0
	defaultTaskBufferIsolationGroup = ""
)

var errRemoteSyncMatchFailed = &types.RemoteSyncMatchedError{Message: "remote sync match failed"}

func TestMatchingEngineSuite(t *testing.T) {
	s := new(matchingEngineSuite)
	suite.Run(t, s)
}

func (s *matchingEngineSuite) SetupSuite() {
	// http.Handle("/test/tasks", http.HandlerFunc(s.TasksHandler))
}

// Renders content of taskManager and matchingEngine when called at http://localhost:6060/test/tasks
// Uncomment HTTP server initialization in SetupSuite method to enable.
func (s *matchingEngineSuite) TasksHandler(w http.ResponseWriter, r *http.Request) {
	s.Lock()
	defer s.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "%v\n", s.taskManager)
	fmt.Fprintf(w, "%v\n", s.matchingEngine)
}

func (s *matchingEngineSuite) TearDownSuite() {
}

func (s *matchingEngineSuite) SetupTest() {
	s.Lock()
	defer s.Unlock()
	s.logger = testlogger.New(s.Suite.T()).WithTags(tag.Dynamic("test-name", s.T().Name()))
	tlKindNormal := types.TaskListKindNormal
	s.mockExecutionManager = &mocks.ExecutionManager{}
	s.controller = gomock.NewController(s.T())
	s.mockHistoryClient = history.NewMockClient(s.controller)
	s.mockTimeSource = clock.NewMockedTimeSourceAt(time.Now())
	s.taskManager = tasklist.NewTestTaskManager(s.T(), s.logger, s.mockTimeSource)
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.CreateDomainCacheEntry(matchingTestDomainName), nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(matchingTestDomainName, nil).AnyTimes()
	s.mockMembershipResolver = membership.NewMockResolver(s.controller)
	s.mockMembershipResolver.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(membership.HostInfo{}, nil).AnyTimes()
	s.mockMembershipResolver.EXPECT().WhoAmI().Return(membership.HostInfo{}, nil).AnyTimes()
	s.mockMembershipResolver.EXPECT().Subscribe(service.Matching, "matching-engine", gomock.Any()).AnyTimes()
	s.mockIsolationStore = dynamicconfig.NewMockClient(s.controller)
	dcClient := dynamicconfig.NewInMemoryClient()
	dcClient.UpdateValue(dynamicconfig.EnableTasklistIsolation, true)
	dc := dynamicconfig.NewCollection(dcClient, s.logger)
	s.isolationState, _ = defaultisolationgroupstate.NewDefaultIsolationGroupStateWatcherWithConfigStoreClient(s.logger,
		dc,
		s.mockDomainCache,
		s.mockIsolationStore,
		metrics.NewNoopMetricsClient(),
		getIsolationGroupsHelper)
	s.handlerContext = newHandlerContext(
		context.Background(),
		matchingTestDomainName,
		&types.TaskList{Name: matchingTestTaskList, Kind: &tlKindNormal},
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		metrics.MatchingTaskListMgrScope,
		testlogger.New(s.Suite.T()),
	)

	s.matchingEngine = s.newMatchingEngine(defaultTestConfig(), s.taskManager)
	s.matchingEngine.Start()
}

func (s *matchingEngineSuite) TearDownTest() {
	s.mockExecutionManager.AssertExpectations(s.T())
	s.matchingEngine.Stop()
	s.controller.Finish()
}

func (s *matchingEngineSuite) newMatchingEngine(
	config *config.Config, taskMgr persistence.TaskManager,
) *matchingEngineImpl {
	return NewEngine(
		taskMgr,
		cluster.GetTestClusterMetadata(true),
		s.mockHistoryClient,
		nil,
		config,
		s.logger,
		metrics.NewClient(tally.NoopScope, metrics.Matching),
		s.mockDomainCache,
		s.mockMembershipResolver,
		s.isolationState,
		s.mockTimeSource,
	).(*matchingEngineImpl)
}

func (s *matchingEngineSuite) TestPollForActivityTasksEmptyResult() {
	s.PollForTasksEmptyResultTest(context.Background(), persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestPollForDecisionTasksEmptyResult() {
	s.PollForTasksEmptyResultTest(context.Background(), persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) TestPollForActivityTasksEmptyResultWithShortContext() {
	shortContextTimeout := returnEmptyTaskTimeBudget + 10*time.Millisecond
	callContext, cancel := context.WithTimeout(context.Background(), shortContextTimeout)
	defer cancel()
	s.PollForTasksEmptyResultTest(callContext, persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestPollForDecisionTasksEmptyResultWithShortContext() {
	shortContextTimeout := returnEmptyTaskTimeBudget + 10*time.Millisecond
	callContext, cancel := context.WithTimeout(context.Background(), shortContextTimeout)
	defer cancel()
	s.PollForTasksEmptyResultTest(callContext, persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) TestOnlyUnloadMatchingInstance() {
	taskListID := tasklist.NewTestTaskListID(
		s.T(),
		uuid.New(),
		"makeToast",
		persistence.TaskListTypeActivity)
	tlKind := types.TaskListKindNormal
	tlm, err := s.matchingEngine.getTaskListManager(taskListID, &tlKind)
	s.Require().NoError(err)

	tlm2, err := tasklist.NewManager(
		s.matchingEngine.domainCache,
		s.matchingEngine.logger,
		s.matchingEngine.metricsClient,
		s.matchingEngine.taskManager,
		s.matchingEngine.clusterMetadata,
		s.matchingEngine.isolationState,
		s.matchingEngine.matchingClient,
		s.matchingEngine.removeTaskListManager,
		taskListID, // same taskListID as above
		&tlKind,
		s.matchingEngine.config,
		s.matchingEngine.timeSource,
		s.matchingEngine.timeSource.Now(),
		s.matchingEngine.historyService)
	s.Require().NoError(err)

	// try to unload a different tlm instance with the same taskListID
	s.matchingEngine.unloadTaskList(tlm2)

	got, err := s.matchingEngine.getTaskListManager(taskListID, &tlKind)
	s.Require().NoError(err)
	s.Require().Same(tlm, got,
		"Unload call with non-matching taskListManager should not cause unload")

	// this time unload the right tlm
	s.matchingEngine.unloadTaskList(tlm)

	got, err = s.matchingEngine.getTaskListManager(taskListID, &tlKind)
	s.Require().NoError(err)
	s.Require().NotSame(tlm, got,
		"Unload call with matching incarnation should have caused unload")
}

func (s *matchingEngineSuite) TestPollForDecisionTasks() {
	s.PollForDecisionTasksResultTest()
}

func (s *matchingEngineSuite) PollForDecisionTasksResultTest() {
	taskType := persistence.TaskListTypeDecision
	domainID := "domainId"
	tl := "makeToast"
	tlKind := types.TaskListKindNormal
	stickyTl := "makeStickyToast"
	stickyTlKind := types.TaskListKindSticky
	identity := "selfDrivingToaster"

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stickyTl
	stickyTaskList.Kind = &stickyTlKind

	s.matchingEngine.config.RangeSize = 2 // to test that range is not updated without tasks
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(1)
	s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)

	runID := "run1"
	workflowID := "workflow1"
	workflowType := types.WorkflowType{
		Name: "workflow",
	}
	execution := types.WorkflowExecution{RunID: runID, WorkflowID: workflowID}
	scheduleID := int64(0)

	// History service is using mock
	s.mockHistoryClient.EXPECT().RecordDecisionTaskStarted(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, taskRequest *types.RecordDecisionTaskStartedRequest, option ...yarpc.CallOption) (*types.RecordDecisionTaskStartedResponse, error) {
			s.logger.Debug("Mock Received RecordDecisionTaskStartedRequest")
			taskListKindNormal := types.TaskListKindNormal
			response := &types.RecordDecisionTaskStartedResponse{}
			response.WorkflowType = &workflowType
			response.PreviousStartedEventID = common.Int64Ptr(scheduleID)
			response.ScheduledEventID = scheduleID + 1
			response.Attempt = 0
			response.StickyExecutionEnabled = true
			response.WorkflowExecutionTaskList = &types.TaskList{
				Name: tl,
				Kind: &taskListKindNormal,
			}
			return response, nil
		}).AnyTimes()

	addRequest := &addTaskRequest{
		TaskType:                      taskType,
		DomainUUID:                    domainID,
		Execution:                     &execution,
		TaskList:                      stickyTaskList,
		ScheduleToStartTimeoutSeconds: 1,
	}
	_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
	s.Error(err)
	s.Contains(err.Error(), "sticky worker is unavailable")
	// poll the sticky tasklist, should get no result
	pollReq := &pollTaskRequest{
		TaskType:   taskType,
		DomainUUID: domainID,
		TaskList:   stickyTaskList,
		Identity:   identity,
	}
	resp, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
	s.NoError(err)
	s.NotNil(resp.AutoConfigHint)
	resp.AutoConfigHint = nil
	s.Equal(&pollTaskResponse{}, resp)
	// add task to sticky tasklist again, this time it should pass
	_, err = addTask(s.matchingEngine, s.handlerContext, addRequest)
	s.NoError(err)

	resp, err = pollTask(s.matchingEngine, s.handlerContext, pollReq)

	expectedResp := &pollTaskResponse{
		TaskToken:              resp.TaskToken,
		WorkflowExecution:      &execution,
		WorkflowType:           &workflowType,
		PreviousStartedEventID: common.Int64Ptr(scheduleID),
		Attempt:                0,
		BacklogCountHint:       1,
		StickyExecutionEnabled: true,
		WorkflowExecutionTaskList: &types.TaskList{
			Name: tl,
			Kind: &tlKind,
		},
		AutoConfigHint: &types.AutoConfigHint{
			EnableAutoConfig:   false,
			PollerWaitTimeInMs: 0,
		},
	}

	s.Nil(err)
	s.Equal(expectedResp, resp)
}

func (s *matchingEngineSuite) PollForTasksEmptyResultTest(callContext context.Context, taskType int) {
	s.matchingEngine.config.RangeSize = 2 // to test that range is not updated without tasks
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(1)
	if _, ok := callContext.Deadline(); !ok {
		s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)
	}

	domainID := "domainId"
	tl := "makeToast"
	identity := "selfDrivingToaster"

	taskList := &types.TaskList{Name: tl}
	var taskListType types.TaskListType
	tlID := tasklist.NewTestTaskListID(s.T(), domainID, tl, taskType)
	s.handlerContext.Context = callContext
	const pollCount = 10
	for i := 0; i < pollCount; i++ {
		pollReq := &pollTaskRequest{
			TaskType:   taskType,
			DomainUUID: domainID,
			TaskList:   taskList,
			Identity:   identity,
		}
		pollResp, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
		s.NotNil(pollResp.AutoConfigHint)
		pollResp.AutoConfigHint = nil // poller wait time is not a fixed value, exclude it from comparison
		s.NoError(err)
		s.Equal(&pollTaskResponse{}, pollResp)

		if taskType == persistence.TaskListTypeActivity {
			taskListType = types.TaskListTypeActivity
		} else {
			taskListType = types.TaskListTypeDecision
		}
		select {
		case <-callContext.Done():
			s.FailNow("Call context has expired.")
		default:
		}
		// check the poller information
		s.handlerContext.Context = context.Background()
		descResp, err := s.matchingEngine.DescribeTaskList(s.handlerContext, &types.MatchingDescribeTaskListRequest{
			DomainUUID: domainID,
			DescRequest: &types.DescribeTaskListRequest{
				TaskList:              taskList,
				TaskListType:          &taskListType,
				IncludeTaskListStatus: false,
			},
		})
		s.NoError(err)
		s.Equal(1, len(descResp.Pollers))
		s.Equal(identity, descResp.Pollers[0].GetIdentity())
		s.NotEmpty(descResp.Pollers[0].GetLastAccessTime())
		s.Nil(descResp.GetTaskListStatus())
	}
	s.EqualValues(1, s.taskManager.GetRangeID(tlID))
}

func (s *matchingEngineSuite) TestQueryWorkflow() {
	domainID := "domainId"
	tl := "makeToast"
	tlKind := types.TaskListKindNormal
	stickyTl := "makeStickyToast"
	stickyTlKind := types.TaskListKindSticky
	identity := "selfDrivingToaster"
	taskList := &types.TaskList{
		Name: tl,
		Kind: &tlKind,
	}
	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stickyTl
	stickyTaskList.Kind = &stickyTlKind

	s.matchingEngine.config.RangeSize = 2 // to test that range is not updated without tasks
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(1)

	runID := "run1"
	workflowID := "workflow1"
	workflowType := types.WorkflowType{
		Name: "workflow",
	}
	execution := types.WorkflowExecution{RunID: runID, WorkflowID: workflowID}

	// History service is using mock
	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), &types.GetMutableStateRequest{
		DomainUUID: domainID,
		Execution:  &execution,
	}).Return(&types.GetMutableStateResponse{
		PreviousStartedEventID: common.Int64Ptr(123),
		NextEventID:            345,
		WorkflowType:           &workflowType,
		TaskList:               taskList,
		CurrentBranchToken:     []byte(`branch token`),
		HistorySize:            999,
		ClientImpl:             "uber-go",
		ClientFeatureVersion:   "1.0.0",
		StickyTaskList:         stickyTaskList,
	}, nil)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := s.matchingEngine.PollForDecisionTask(s.handlerContext, &types.MatchingPollForDecisionTaskRequest{
			DomainUUID: domainID,
			PollRequest: &types.PollForDecisionTaskRequest{
				TaskList: taskList,
				Identity: identity,
			},
		})
		s.NoError(err)
		s.NotNil(resp.TaskToken)
		token, err := s.matchingEngine.tokenSerializer.DeserializeQueryTaskToken(resp.TaskToken)
		s.NoError(err)
		s.Equal(domainID, token.DomainID)
		s.Equal(workflowID, token.WorkflowID)
		s.Equal(runID, token.RunID)
		s.Equal(tl, token.TaskList)
		s.True(resp.StickyExecutionEnabled)
		err = s.matchingEngine.RespondQueryTaskCompleted(s.handlerContext, &types.MatchingRespondQueryTaskCompletedRequest{
			DomainUUID: domainID,
			TaskList:   taskList,
			TaskID:     token.TaskID,
			CompletedRequest: &types.RespondQueryTaskCompletedRequest{
				TaskToken:     []byte(``),
				CompletedType: types.QueryTaskCompletedTypeCompleted.Ptr(),
				QueryResult:   []byte(`result`),
				WorkerVersionInfo: &types.WorkerVersionInfo{
					Impl:           "uber-go",
					FeatureVersion: "1.5.0",
				},
			},
		})
		s.NoError(err)
	}()
	time.Sleep(10 * time.Millisecond) // wait for poller to start
	resp, err := s.matchingEngine.QueryWorkflow(s.handlerContext, &types.MatchingQueryWorkflowRequest{
		DomainUUID: domainID,
		TaskList:   taskList,
		QueryRequest: &types.QueryWorkflowRequest{
			Domain:                "domain",
			Execution:             &execution,
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	})
	wg.Wait()
	s.NoError(err)
	s.Equal(&types.QueryWorkflowResponse{
		QueryResult: []byte(`result`),
	}, resp)
}

func (s *matchingEngineSuite) TestAddActivityTasks() {
	s.AddTasksTest(persistence.TaskListTypeActivity, false)
}

func (s *matchingEngineSuite) TestAddDecisionTasks() {
	s.AddTasksTest(persistence.TaskListTypeDecision, false)
}

func (s *matchingEngineSuite) TestAddActivityTasksForwarded() {
	s.AddTasksTest(persistence.TaskListTypeActivity, true)
}

func (s *matchingEngineSuite) TestAddDecisionTasksForwarded() {
	s.AddTasksTest(persistence.TaskListTypeDecision, true)
}

func (s *matchingEngineSuite) AddTasksTest(taskType int, isForwarded bool) {
	s.matchingEngine.config.RangeSize = 300 // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(150)

	domainID := "domainId"
	tl := "makeToast"
	forwardedFrom := "/__cadence_sys/makeToast/1"

	taskList := &types.TaskList{Name: tl}

	const taskCount = 111

	runID := "run1"
	workflowID := "workflow1"
	execution := types.WorkflowExecution{RunID: runID, WorkflowID: workflowID}

	for i := int64(0); i < taskCount; i++ {
		scheduleID := i * 3
		addRequest := &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    domainID,
			Execution:                     &execution,
			ScheduleID:                    scheduleID,
			TaskList:                      taskList,
			ScheduleToStartTimeoutSeconds: 1,
		}
		if isForwarded {
			addRequest.ForwardedFrom = forwardedFrom
		}
		_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)

		switch isForwarded {
		case false:
			s.NoError(err)
		case true:
			s.Equal(errRemoteSyncMatchFailed, err)
		}
	}

	switch isForwarded {
	case false:
		s.EqualValues(taskCount, s.taskManager.GetTaskCount(tasklist.NewTestTaskListID(s.T(), domainID, tl, taskType)))
	case true:
		s.EqualValues(0, s.taskManager.GetTaskCount(tasklist.NewTestTaskListID(s.T(), domainID, tl, taskType)))
	}
}

func (s *matchingEngineSuite) TestAddAndPollDecisionTasks() {
	s.AddAndPollTasks(persistence.TaskListTypeDecision, false)
}

func (s *matchingEngineSuite) TestAddAndPollActivityTasks() {
	s.AddAndPollTasks(persistence.TaskListTypeDecision, false)
}

func (s *matchingEngineSuite) TestAddAndPollDecisionTasksIsolation() {
	s.AddAndPollTasks(persistence.TaskListTypeDecision, true)
}

func (s *matchingEngineSuite) TestAddAndPollActivityTasksIsolation() {
	s.AddAndPollTasks(persistence.TaskListTypeDecision, true)
}

func (s *matchingEngineSuite) AddAndPollTasks(taskType int, enableIsolation bool) {
	s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(enableIsolation)

	isolationGroups := s.matchingEngine.config.AllIsolationGroups()

	const taskCount = 6
	const initialRangeID = 102
	// TODO: Understand why publish is low when rangeSize is 3
	const rangeSize = 30

	testParam := newTestParam(s.T(), taskType)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)

	s.setupGetDrainStatus()

	for i := int64(0); i < taskCount; i++ {
		scheduleID := i * 3
		addRequest := &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    testParam.DomainID,
			Execution:                     testParam.WorkflowExecution,
			ScheduleID:                    scheduleID,
			TaskList:                      testParam.TaskList,
			ScheduleToStartTimeoutSeconds: 1,
			PartitionConfig:               map[string]string{isolationgroup.GroupKey: isolationGroups[int(i)%len(isolationGroups)]},
		}
		_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
		s.NoError(err)
	}
	s.EqualValues(taskCount, s.taskManager.GetTaskCount(testParam.TaskListID))

	s.setupRecordTaskStartedMock(taskType, testParam, false)

	for i := int64(0); i < taskCount; {
		scheduleID := i * 3
		pollReq := &pollTaskRequest{
			TaskType:       taskType,
			DomainUUID:     testParam.DomainID,
			TaskList:       testParam.TaskList,
			Identity:       testParam.Identity,
			IsolationGroup: isolationGroups[int(i)%len(isolationGroups)],
		}
		result, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
		s.NoError(err)
		s.NotNil(result)
		if isEmptyToken(result.TaskToken) {
			s.logger.Debug("empty poll returned")
			continue
		}
		s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
		i++
	}
	s.EqualValues(0, s.taskManager.GetTaskCount(testParam.TaskListID))
	expectedRange := getExpectedRange(initialRangeID, taskCount, rangeSize)
	// Due to conflicts some ids are skipped and more real ranges are used.
	s.True(expectedRange <= s.taskManager.GetRangeID(testParam.TaskListID))
}

func (s *matchingEngineSuite) TestSyncMatchActivityTasks() {
	s.SyncMatchTasks(persistence.TaskListTypeActivity, false)
}

func (s *matchingEngineSuite) TestSyncMatchDecisionTasks() {
	s.SyncMatchTasks(persistence.TaskListTypeDecision, false)
}

func (s *matchingEngineSuite) TestSyncMatchActivityTasksIsolation() {
	s.SyncMatchTasks(persistence.TaskListTypeActivity, true)
}

func (s *matchingEngineSuite) TestSyncMatchDecisionTasksIsolation() {
	s.SyncMatchTasks(persistence.TaskListTypeDecision, true)
}

func (s *matchingEngineSuite) SyncMatchTasks(taskType int, enableIsolation bool) {
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(enableIsolation)
	const taskCount = 10
	const initialRangeID = 102
	// TODO: Understand why publish is low when rangeSize is 3
	const rangeSize = 30
	var throttledTaskCount int64
	if taskType == persistence.TaskListTypeActivity {
		throttledTaskCount = 3
	}
	isolationGroups := s.matchingEngine.config.AllIsolationGroups

	// Set a short long poll expiration so we don't have to wait too long for 0 throttling cases
	s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(200 * time.Millisecond)
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)
	s.matchingEngine.config.TaskDispatchRPSTTL = time.Nanosecond
	s.matchingEngine.config.MinTaskThrottlingBurstSize = dynamicconfig.GetIntPropertyFilteredByTaskListInfo(_minBurst)
	// So we can get snapshots
	scope := tally.NewTestScope("test", nil)
	s.matchingEngine.metricsClient = metrics.NewClient(scope, metrics.Matching)

	testParam := newTestParam(s.T(), taskType)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)

	s.setupGetDrainStatus()
	s.setupRecordTaskStartedMock(taskType, testParam, false)

	pollFunc := func(maxDispatch float64, isolationGroup string) (*pollTaskResponse, error) {
		pollReq := &pollTaskRequest{
			TaskType:         taskType,
			DomainUUID:       testParam.DomainID,
			TaskList:         testParam.TaskList,
			Identity:         testParam.Identity,
			TaskListMetadata: &types.TaskListMetadata{MaxTasksPerSecond: &maxDispatch},
			IsolationGroup:   isolationGroup,
		}
		return pollTask(s.matchingEngine, s.handlerContext, pollReq)
	}

	for i := int64(0); i < taskCount; i++ {
		scheduleID := i * 3
		group := isolationGroups()[int(i)%len(isolationGroups())]
		var wg sync.WaitGroup
		var result *pollTaskResponse
		var pollErr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, pollErr = pollFunc(_defaultTaskDispatchRPS, group)
		}()
		time.Sleep(20 * time.Millisecond) // Wait for a short period of time to let the poller start so that sync match will happen
		addRequest := &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    testParam.DomainID,
			Execution:                     testParam.WorkflowExecution,
			ScheduleID:                    scheduleID,
			TaskList:                      testParam.TaskList,
			ScheduleToStartTimeoutSeconds: 1,
			PartitionConfig:               map[string]string{isolationgroup.GroupKey: group},
		}
		_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
		wg.Wait()
		s.NoError(err)
		s.NoError(pollErr)
		s.NotNil(result)
		s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
	}
	// expect more than half of the tasks get sync matches
	s.True(s.taskManager.GetCreateTaskCount(testParam.TaskListID) < taskCount/2)

	// Set the dispatch RPS to 0, to verify that poller will not get any task and task will be persisted into database
	// Revert the dispatch RPS and verify that poller will get the task
	for i := int64(0); i < throttledTaskCount; i++ {
		scheduleID := i * 3
		group := isolationGroups()[int(i)%len(isolationGroups())]
		var wg sync.WaitGroup
		var result *pollTaskResponse
		var pollErr error
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, pollErr = pollFunc(0.0, group)
		}()
		time.Sleep(20 * time.Millisecond) // Wait for a short period of time to let the poller start so that sync match will happen
		addRequest := &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    testParam.DomainID,
			Execution:                     testParam.WorkflowExecution,
			ScheduleID:                    scheduleID,
			TaskList:                      testParam.TaskList,
			ScheduleToStartTimeoutSeconds: 1,
			PartitionConfig:               map[string]string{isolationgroup.GroupKey: group},
		}
		_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
		wg.Wait()
		s.NoError(err)
		s.NoError(pollErr)
		s.NotNil(result)
		// when ratelimit is set to zero, poller is expected to return empty result
		// reset ratelimit, poll again and make sure task is returned this time
		s.True(isEmptyToken(result.TaskToken))
		result, pollErr = pollFunc(_defaultTaskDispatchRPS, group)
		s.NoError(err)
		s.NoError(pollErr)
		s.NotNil(result)
		s.False(isEmptyToken(result.TaskToken))
		s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
	}
	s.True(int(throttledTaskCount) <= s.taskManager.GetCreateTaskCount(testParam.TaskListID))
	s.EqualValues(0, s.taskManager.GetTaskCount(testParam.TaskListID))
	expectedRange := getExpectedRange(initialRangeID, int(taskCount+throttledTaskCount), rangeSize)
	// Due to conflicts some ids are skipped and more real ranges are used.
	s.True(expectedRange <= s.taskManager.GetRangeID(testParam.TaskListID))

	if throttledTaskCount > 0 {
		syncCtr := scope.Snapshot().Counters()["test.sync_throttle_count_per_tl+domain="+matchingTestDomainName+",operation=TaskListMgr,tasklist="+testParam.TaskList.Name+",tasklistType=activity"]
		s.EqualValues(throttledTaskCount, int(syncCtr.Value()))
	}

	// check the poller information
	descResp, err := s.matchingEngine.DescribeTaskList(s.handlerContext, &types.MatchingDescribeTaskListRequest{
		DomainUUID: testParam.DomainID,
		DescRequest: &types.DescribeTaskListRequest{
			TaskList:              testParam.TaskList,
			TaskListType:          testParam.TaskListType,
			IncludeTaskListStatus: true,
		},
	})
	s.NoError(err)
	s.Equal(1, len(descResp.Pollers))
	s.Equal(testParam.Identity, descResp.Pollers[0].GetIdentity())
	s.NotEmpty(descResp.Pollers[0].GetLastAccessTime())
	s.Equal(_defaultTaskDispatchRPS, descResp.Pollers[0].GetRatePerSecond())
	s.NotNil(descResp.GetTaskListStatus())
	s.True(descResp.GetTaskListStatus().GetRatePerSecond() >= (_defaultTaskDispatchRPS - 1))
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollActivities() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeActivity, 20, 100, false, false)
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollActivitiesWithZeroDispatch() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeActivity, 20, 100, true, false)
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollDecisions() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeDecision, 20, 100, false, false)
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollActivitiesIsolation() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeActivity, 20, 100, false, true)
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollActivitiesWithZeroDispatchIsolation() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeActivity, 20, 100, true, true)
}

func (s *matchingEngineSuite) TestConcurrentAddAndPollDecisionsIsolation() {
	s.ConcurrentAddAndPollTasks(persistence.TaskListTypeDecision, 20, 100, false, true)
}

func (s *matchingEngineSuite) ConcurrentAddAndPollTasks(taskType int, workerCount int, taskCount int64, throttled, enableIsolation bool) {
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(enableIsolation)
	isolationGroups := s.matchingEngine.config.AllIsolationGroups
	dispatchLimitFn := func(wc int, tc int64) float64 {
		return _defaultTaskDispatchRPS
	}
	if throttled {
		dispatchLimitFn = func(wc int, tc int64) float64 {
			if tc%50 == 0 && wc%5 == 0 { // Gets triggered atleast 20 times
				return 0
			}
			return _defaultTaskDispatchRPS
		}
	}
	scope := tally.NewTestScope("test", nil)
	s.matchingEngine.metricsClient = metrics.NewClient(scope, metrics.Matching)

	const initialRangeID = 0
	const rangeSize = 3
	var scheduleID int64 = 123

	testParam := newTestParam(s.T(), taskType)
	tlKind := types.TaskListKindNormal
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)
	s.matchingEngine.config.TaskDispatchRPSTTL = time.Nanosecond
	s.matchingEngine.config.MinTaskThrottlingBurstSize = dynamicconfig.GetIntPropertyFilteredByTaskListInfo(_minBurst)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)

	s.setupGetDrainStatus()

	var wg sync.WaitGroup
	wg.Add(2 * workerCount)

	for p := 0; p < workerCount; p++ {
		go func() {
			defer wg.Done()
			for i := int64(0); i < taskCount; i++ {
				group := isolationGroups()[int(i)%len(isolationGroups())] // let each worker to generate tasks for all isolation groups
				addRequest := &addTaskRequest{
					TaskType:                      taskType,
					DomainUUID:                    testParam.DomainID,
					Execution:                     testParam.WorkflowExecution,
					ScheduleID:                    scheduleID,
					TaskList:                      testParam.TaskList,
					ScheduleToStartTimeoutSeconds: 1,
					PartitionConfig:               map[string]string{isolationgroup.GroupKey: group},
				}
				_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
				if err != nil {
					s.logger.Info("Failure in AddActivityTask", tag.Error(err))
					i--
				}
			}
		}()
	}

	s.setupRecordTaskStartedMock(taskType, testParam, false)

	for p := 0; p < workerCount; p++ {
		go func(wNum int) {
			defer wg.Done()
			for i := int64(0); i < taskCount; {
				maxDispatch := dispatchLimitFn(wNum, i)
				group := isolationGroups()[int(wNum)%len(isolationGroups())] // let each worker only polls from one isolation group
				pollReq := &pollTaskRequest{
					TaskType:         taskType,
					DomainUUID:       testParam.DomainID,
					TaskList:         testParam.TaskList,
					Identity:         testParam.Identity,
					TaskListMetadata: &types.TaskListMetadata{MaxTasksPerSecond: &maxDispatch},
					IsolationGroup:   group,
				}
				result, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
				s.NoError(err)
				s.NotNil(result)
				if isEmptyToken(result.TaskToken) {
					s.logger.Debug("empty poll returned")
					continue
				}
				s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
				i++
			}
		}(p)
	}
	wg.Wait()
	totalTasks := int(taskCount) * workerCount
	persisted := s.taskManager.GetCreateTaskCount(testParam.TaskListID)
	s.True(persisted < totalTasks)
	expectedRange := getExpectedRange(initialRangeID, persisted, rangeSize)
	// Due to conflicts some ids are skipped and more real ranges are used.
	s.True(expectedRange <= s.taskManager.GetRangeID(testParam.TaskListID))
	mgr, err := s.matchingEngine.getTaskListManager(testParam.TaskListID, &tlKind)
	s.NoError(err)
	// stop the tasklist manager to force the acked tasks to be deleted
	mgr.Stop()
	s.EqualValues(0, s.taskManager.GetTaskCount(testParam.TaskListID))

	syncCtr := scope.Snapshot().Counters()["test.sync_throttle_count_per_tl+domain="+matchingTestDomainName+",operation=TaskListMgr,tasklist="+testParam.TaskList.Name+",tasklistType=activity"]
	bufCtr := scope.Snapshot().Counters()["test.buffer_throttle_count_per_tl+domain="+matchingTestDomainName+",operation=TaskListMgr,tasklist="+testParam.TaskList.Name+",tasklistType=activity"]
	total := int64(0)
	if syncCtr != nil {
		total += syncCtr.Value()
	}
	if bufCtr != nil {
		total += bufCtr.Value()
	}
	if throttled {
		// atleast once from 0 dispatch poll, and until TTL is hit at which time throttle limit is reset
		// hard to predict exactly how many times, since the atomic.Value load might not have updated.
		s.True(total >= 1)
	} else {
		s.EqualValues(0, total)
	}
}

func (s *matchingEngineSuite) TestPollActivityWithExpiredContext() {
	s.PollWithExpiredContext(persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestPollDecisionWithExpiredContext() {
	s.PollWithExpiredContext(persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) PollWithExpiredContext(taskType int) {
	identity := "nobody"
	domainID := "domainId"
	tl := "makeToast"

	taskList := &types.TaskList{Name: tl}

	// Try with cancelled context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	cancel()
	s.handlerContext.Context = ctx
	pollReq := &pollTaskRequest{
		TaskType:   taskType,
		DomainUUID: domainID,
		TaskList:   taskList,
		Identity:   identity,
	}
	_, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
	s.Equal(ctx.Err(), err)

	// Try with expired context
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.handlerContext.Context = ctx
	resp, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
	s.Nil(err)
	s.NotNil(resp.AutoConfigHint)
	resp.AutoConfigHint = nil
	s.Equal(&pollTaskResponse{}, resp)
}

func (s *matchingEngineSuite) TestMultipleEnginesActivitiesRangeStealing() {
	s.MultipleEnginesTasksRangeStealing(persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestMultipleEnginesDecisionsRangeStealing() {
	s.MultipleEnginesTasksRangeStealing(persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) MultipleEnginesTasksRangeStealing(taskType int) {
	s.T().Cleanup(func() { goleak.VerifyNone(s.T()) })
	// Add N tasks to engine1 and then N tasks to engine2. Then poll all tasks from engine2. Engine1 should be closed
	const N = 10
	const initialRangeID = 0
	const rangeSize = 5
	var scheduleID int64 = 123

	testParam := newTestParam(s.T(), taskType)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)

	engine1 := s.newMatchingEngine(defaultTestConfig(), s.taskManager)
	engine1.config.RangeSize = rangeSize
	engine1.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)
	engine1.Start()
	defer engine1.Stop()

	createAddTaskReq := func() *addTaskRequest {
		return &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    testParam.DomainID,
			Execution:                     testParam.WorkflowExecution,
			ScheduleID:                    scheduleID,
			TaskList:                      testParam.TaskList,
			ScheduleToStartTimeoutSeconds: 600,
		}
	}

	// First add tasks to engine1
	for i := 0; i < N; i++ {
		_, err := addTask(engine1, s.handlerContext, createAddTaskReq())
		s.Require().NoError(err)
	}

	engine2 := s.newMatchingEngine(defaultTestConfig(), s.taskManager)
	engine2.config.RangeSize = rangeSize
	engine2.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)
	engine2.Start()
	defer engine2.Stop()

	// Then add tasks to engine2. It should be able to steal lease and process tasks
	for i := 0; i < N; i++ {
		_, err := addTask(engine2, s.handlerContext, createAddTaskReq())
		s.Require().NoError(err)
	}

	_, err := addTask(engine1, s.handlerContext, createAddTaskReq())
	// Adding another task to engine1 should fail because it will not have the lease
	s.Require().ErrorContains(err, "task list shutting down")

	s.EqualValues(2*N, s.taskManager.GetCreateTaskCount(testParam.TaskListID))

	s.setupRecordTaskStartedMock(taskType, testParam, true)

	// Poll all tasks from engine2.
	for i := 0; i < 2*N; i++ {
		pollReq := &pollTaskRequest{
			TaskType:   taskType,
			DomainUUID: testParam.DomainID,
			TaskList:   testParam.TaskList,
			Identity:   testParam.Identity,
		}
		result, err := pollTask(engine2, s.handlerContext, pollReq)
		s.Require().NoError(err)
		s.NotNil(result)
		if isEmptyToken(result.TaskToken) {
			s.Fail("empty poll returned")
		}
		s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
	}

	s.EqualValues(0, s.taskManager.GetTaskCount(testParam.TaskListID))
	totalTasks := 2 * N
	persisted := s.taskManager.GetCreateTaskCount(testParam.TaskListID)
	// No sync matching as all messages are added first
	s.EqualValues(totalTasks, persisted)
	expectedRange := getExpectedRange(initialRangeID, persisted, rangeSize)
	// Due to conflicts some ids are skipped and more real ranges are used.
	s.True(expectedRange <= s.taskManager.GetRangeID(testParam.TaskListID))
}

func (s *matchingEngineSuite) TestAddTaskAfterStartFailure() {
	taskType := persistence.TaskListTypeActivity
	runID := "run1"
	workflowID := "workflow1"
	workflowExecution := types.WorkflowExecution{RunID: runID, WorkflowID: workflowID}

	domainID := "domainId"
	tl := "makeToast"
	tlID := tasklist.NewTestTaskListID(s.T(), domainID, tl, taskType)
	tlKind := types.TaskListKindNormal

	taskList := &types.TaskList{Name: tl}

	scheduleID := int64(0)
	addRequest := &addTaskRequest{
		TaskType:                      taskType,
		DomainUUID:                    domainID,
		Execution:                     &workflowExecution,
		ScheduleID:                    scheduleID,
		TaskList:                      taskList,
		ScheduleToStartTimeoutSeconds: 1,
	}

	_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
	s.NoError(err)
	s.EqualValues(1, s.taskManager.GetTaskCount(tlID))

	tlMgr, err := s.matchingEngine.getTaskListManager(tlID, &tlKind)
	s.NoError(err)
	ctx, err := tlMgr.GetTask(context.Background(), nil)
	s.NoError(err)

	ctx.Finish(errors.New("test error"))
	s.EqualValues(1, s.taskManager.GetTaskCount(tlID))
	ctx2, err := tlMgr.GetTask(context.Background(), nil)
	s.NoError(err)

	s.NotEqual(ctx.Event.TaskID, ctx2.Event.TaskID)
	s.Equal(ctx.Event.WorkflowID, ctx2.Event.WorkflowID)
	s.Equal(ctx.Event.RunID, ctx2.Event.RunID)
	s.Equal(ctx.Event.ScheduleID, ctx2.Event.ScheduleID)

	ctx2.Finish(nil)
	s.EqualValues(0, s.taskManager.GetTaskCount(tlID))
}

func (s *matchingEngineSuite) TestUnloadActivityTasklistOnIsolationConfigChange() {
	s.UnloadTasklistOnIsolationConfigChange(persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestUnloadDecisionTasklistOnIsolationConfigChange() {
	s.UnloadTasklistOnIsolationConfigChange(persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) UnloadTasklistOnIsolationConfigChange(taskType int) {
	s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(50 * time.Millisecond)
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(false)

	const taskCount = 1000
	const initialRangeID = 102
	// TODO: Understand why publish is low when rangeSize is 3
	const rangeSize = 30

	testParam := newTestParam(s.T(), taskType)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)

	addRequest := &addTaskRequest{
		TaskType:                      taskType,
		DomainUUID:                    testParam.DomainID,
		Execution:                     testParam.WorkflowExecution,
		ScheduleID:                    333,
		TaskList:                      testParam.TaskList,
		ScheduleToStartTimeoutSeconds: 1,
	}
	_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
	s.NoError(err)

	// enable isolation and verify that poller should not get any task
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(true)
	s.setupGetDrainStatus()
	s.setupRecordTaskStartedMock(taskType, testParam, false)

	pollReq := &pollTaskRequest{
		TaskType:   taskType,
		DomainUUID: testParam.DomainID,
		TaskList:   testParam.TaskList,
		Identity:   testParam.Identity,
	}
	result, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
	s.NoError(err)
	s.NotNil(result.AutoConfigHint)
	result.AutoConfigHint = nil
	s.Equal(&pollTaskResponse{}, result)

	result, err = pollTask(s.matchingEngine, s.handlerContext, pollReq)
	s.NoError(err)
	s.NotNil(result)
	s.assertPollTaskResponse(taskType, testParam, 333, result)

	// disable isolation again and verify add tasklist should fail
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(false)
	_, err = addTask(s.matchingEngine, s.handlerContext, addRequest)
	s.Error(err)
	s.Contains(err.Error(), "task list shutting down")
}

func (s *matchingEngineSuite) TestDrainActivityBacklogNoPollersIsolationGroup() {
	s.DrainBacklogNoPollersIsolationGroup(persistence.TaskListTypeActivity)
}

func (s *matchingEngineSuite) TestDrainDecisionBacklogNoPollersIsolationGroup() {
	s.DrainBacklogNoPollersIsolationGroup(persistence.TaskListTypeDecision)
}

func (s *matchingEngineSuite) DrainBacklogNoPollersIsolationGroup(taskType int) {
	s.matchingEngine.config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)
	s.matchingEngine.config.EnableTasklistIsolation = dynamicconfig.GetBoolPropertyFnFilteredByDomainID(true)
	s.matchingEngine.config.AsyncTaskDispatchTimeout = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)

	isolationGroups := s.matchingEngine.config.AllIsolationGroups

	const taskCount = 1000
	const initialRangeID = 102
	// TODO: Understand why publish is low when rangeSize is 3
	const rangeSize = 30
	// use a const scheduleID because we don't know the order of task polled
	const scheduleID = 11111

	testParam := newTestParam(s.T(), taskType)
	s.taskManager.SetRangeID(testParam.TaskListID, initialRangeID)
	s.matchingEngine.config.RangeSize = rangeSize // override to low number for the test
	s.matchingEngine.config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(rangeSize / 2)
	_, err := s.matchingEngine.getTaskListManager(testParam.TaskListID, testParam.TaskList.Kind)
	s.NoError(err)
	// advance the time a bit more than warmup time of new tasklist after the creation of tasklist manager, which is 1 minute
	s.mockTimeSource.Advance(time.Minute)
	s.mockTimeSource.Advance(time.Second)

	s.setupGetDrainStatus()

	for i := int64(0); i < taskCount; i++ {
		addRequest := &addTaskRequest{
			TaskType:                      taskType,
			DomainUUID:                    testParam.DomainID,
			Execution:                     testParam.WorkflowExecution,
			ScheduleID:                    scheduleID,
			TaskList:                      testParam.TaskList,
			ScheduleToStartTimeoutSeconds: 1,
			PartitionConfig:               map[string]string{isolationgroup.GroupKey: isolationGroups()[int(i)%len(isolationGroups())]},
		}
		_, err := addTask(s.matchingEngine, s.handlerContext, addRequest)
		s.NoError(err)
	}
	s.EqualValues(taskCount, s.taskManager.GetTaskCount(testParam.TaskListID))

	s.setupRecordTaskStartedMock(taskType, testParam, false)

	for i := int64(0); i < taskCount; {
		pollReq := &pollTaskRequest{
			TaskType:       taskType,
			DomainUUID:     testParam.DomainID,
			TaskList:       testParam.TaskList,
			Identity:       testParam.Identity,
			IsolationGroup: isolationGroups()[0],
		}
		result, err := pollTask(s.matchingEngine, s.handlerContext, pollReq)
		s.NoError(err)
		s.NotNil(result)
		if isEmptyToken(result.TaskToken) {
			s.logger.Debug("empty poll returned")
			continue
		}
		s.assertPollTaskResponse(taskType, testParam, scheduleID, result)
		i++
	}
	s.EqualValues(0, s.taskManager.GetTaskCount(testParam.TaskListID))
	expectedRange := getExpectedRange(initialRangeID, taskCount, rangeSize)
	// Due to conflicts some ids are skipped and more real ranges are used.
	s.True(expectedRange <= s.taskManager.GetRangeID(testParam.TaskListID))
}

func (s *matchingEngineSuite) setupRecordTaskStartedMock(taskType int, param *testParam, checkDuplicate bool) {
	startedTasks := make(map[int64]bool)
	if taskType == persistence.TaskListTypeActivity {
		s.mockHistoryClient.EXPECT().RecordActivityTaskStarted(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, taskRequest *types.RecordActivityTaskStartedRequest, option ...yarpc.CallOption) (*types.RecordActivityTaskStartedResponse, error) {
				s.logger.Debug(fmt.Sprintf("Mock Received RecordActivityTaskStartedRequest, taskID: %v", taskRequest.TaskID))
				if checkDuplicate {
					if _, ok := startedTasks[taskRequest.TaskID]; ok {
						return nil, &types.EventAlreadyStartedError{Message: "already started"}
					}
					startedTasks[taskRequest.TaskID] = true
				}
				return &types.RecordActivityTaskStartedResponse{
					ScheduledEvent: newActivityTaskScheduledEvent(taskRequest.ScheduleID, 0,
						&types.ScheduleActivityTaskDecisionAttributes{
							ActivityID:                    param.ActivityID,
							TaskList:                      param.TaskList,
							ActivityType:                  param.ActivityType,
							Input:                         param.ActivityInput,
							Header:                        param.ActivityHeader,
							ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
							ScheduleToStartTimeoutSeconds: common.Int32Ptr(50),
							StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
							HeartbeatTimeoutSeconds:       common.Int32Ptr(10),
						}),
				}, nil
			}).AnyTimes()
	} else {
		s.mockHistoryClient.EXPECT().RecordDecisionTaskStarted(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, taskRequest *types.RecordDecisionTaskStartedRequest, option ...yarpc.CallOption) (*types.RecordDecisionTaskStartedResponse, error) {
				s.logger.Debug(fmt.Sprintf("Mock Received RecordDecisionTaskStartedRequest, taskID: %v", taskRequest.TaskID))
				if checkDuplicate {
					if _, ok := startedTasks[taskRequest.TaskID]; ok {
						return nil, &types.EventAlreadyStartedError{Message: "already started"}
					}
					startedTasks[taskRequest.TaskID] = true
				}
				return &types.RecordDecisionTaskStartedResponse{
					PreviousStartedEventID: &param.StartedEventID,
					StartedEventID:         param.StartedEventID,
					ScheduledEventID:       taskRequest.ScheduleID,
					WorkflowType:           param.WorkflowType,
				}, nil
			}).AnyTimes()
	}
}

func (s *matchingEngineSuite) setupGetDrainStatus() {
	s.mockIsolationStore.EXPECT().GetListValue(dynamicconfig.DefaultIsolationGroupConfigStoreManagerGlobalMapping, nil).Return(nil, nil).AnyTimes()
}

func (s *matchingEngineSuite) awaitCondition(cond func() bool, timeout time.Duration) bool {
	expiry := time.Now().Add(timeout)
	for !cond() {
		time.Sleep(time.Millisecond * 5)
		if time.Now().After(expiry) {
			return false
		}
	}
	return true
}

func (s *matchingEngineSuite) assertPollTaskResponse(taskType int, param *testParam, scheduleID int64, actual *pollTaskResponse) {
	if taskType == persistence.TaskListTypeActivity {
		token := &common.TaskToken{
			DomainID:     param.DomainID,
			WorkflowID:   param.WorkflowExecution.WorkflowID,
			RunID:        param.WorkflowExecution.RunID,
			ScheduleID:   scheduleID,
			ActivityID:   param.ActivityID,
			ActivityType: param.ActivityType.Name,
		}
		s.EqualValues(token, actual.TaskToken)
		s.EqualValues(param.ActivityID, actual.ActivityID)
		s.EqualValues(param.ActivityType, actual.ActivityType)
		s.EqualValues(param.ActivityInput, actual.Input)
		s.EqualValues(param.ActivityHeader, actual.Header)
		s.EqualValues(param.WorkflowExecution, actual.WorkflowExecution)
	} else {
		token := &common.TaskToken{
			DomainID:   param.DomainID,
			WorkflowID: param.WorkflowExecution.WorkflowID,
			RunID:      param.WorkflowExecution.RunID,
			ScheduleID: scheduleID,
		}
		s.EqualValues(token, actual.TaskToken)
		s.EqualValues(param.WorkflowExecution, actual.WorkflowExecution)
		s.EqualValues(param.WorkflowType, actual.WorkflowType)
		s.EqualValues(param.StartedEventID, actual.StartedEventID)
	}
}

func (s *matchingEngineSuite) TestConfigDefaultHostName() {
	configEmpty := config.Config{}
	s.NotEqualValues(s.matchingEngine.config.HostName, configEmpty.HostName)
	s.EqualValues(configEmpty.HostName, "")
}

func newActivityTaskScheduledEvent(eventID int64, decisionTaskCompletedEventID int64,
	scheduleAttributes *types.ScheduleActivityTaskDecisionAttributes) *types.HistoryEvent {
	historyEvent := newHistoryEvent(eventID, types.EventTypeActivityTaskScheduled)
	attributes := &types.ActivityTaskScheduledEventAttributes{}
	attributes.ActivityID = scheduleAttributes.ActivityID
	attributes.ActivityType = scheduleAttributes.ActivityType
	attributes.TaskList = scheduleAttributes.TaskList
	attributes.Input = scheduleAttributes.Input
	attributes.Header = scheduleAttributes.Header
	attributes.ScheduleToCloseTimeoutSeconds = scheduleAttributes.ScheduleToCloseTimeoutSeconds
	attributes.ScheduleToStartTimeoutSeconds = scheduleAttributes.ScheduleToStartTimeoutSeconds
	attributes.StartToCloseTimeoutSeconds = scheduleAttributes.StartToCloseTimeoutSeconds
	attributes.HeartbeatTimeoutSeconds = scheduleAttributes.HeartbeatTimeoutSeconds
	attributes.DecisionTaskCompletedEventID = decisionTaskCompletedEventID
	historyEvent.ActivityTaskScheduledEventAttributes = attributes

	return historyEvent
}

func newHistoryEvent(eventID int64, eventType types.EventType) *types.HistoryEvent {
	ts := common.Int64Ptr(time.Now().UnixNano())
	historyEvent := &types.HistoryEvent{}
	historyEvent.ID = eventID
	historyEvent.Timestamp = ts
	historyEvent.EventType = &eventType

	return historyEvent
}

func validateTimeRange(t time.Time, expectedDuration time.Duration) bool {
	currentTime := time.Now()
	diff := time.Duration(currentTime.UnixNano() - t.UnixNano())
	if diff > expectedDuration {
		fmt.Printf("Current time: %v, Application time: %v, Difference: %v \n", currentTime, t, diff)
		return false
	}
	return true
}

func defaultTestConfig() *config.Config {
	config := config.NewConfig(dynamicconfig.NewNopCollection(), "some random hostname", getIsolationGroupsHelper)
	config.LongPollExpirationInterval = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(100 * time.Millisecond)
	config.MaxTaskDeleteBatchSize = dynamicconfig.GetIntPropertyFilteredByTaskListInfo(1)
	config.ReadRangeSize = dynamicconfig.GetIntPropertyFn(50000)
	config.GetTasksBatchSize = dynamicconfig.GetIntPropertyFilteredByTaskListInfo(10)
	config.AsyncTaskDispatchTimeout = dynamicconfig.GetDurationPropertyFnFilteredByTaskListInfo(10 * time.Millisecond)
	config.MaxTimeBetweenTaskDeletes = time.Duration(0)
	config.EnableTasklistOwnershipGuard = func(opts ...dynamicconfig.FilterOption) bool { return true }
	return config
}

func getExpectedRange(initialRangeID, taskCount, rangeSize int) int64 {
	expectedRange := int64(initialRangeID + taskCount/rangeSize)
	if taskCount%rangeSize > 0 {
		expectedRange++
	}
	return expectedRange
}

type testParam struct {
	DomainID          string
	WorkflowExecution *types.WorkflowExecution
	TaskList          *types.TaskList
	TaskListID        *tasklist.Identifier
	TaskListType      *types.TaskListType
	Identity          string
	ActivityID        string
	ActivityType      *types.ActivityType
	ActivityInput     []byte
	ActivityHeader    *types.Header
	WorkflowType      *types.WorkflowType
	StartedEventID    int64
	ScheduledEventID  int64
}

func newTestParam(t *testing.T, taskType int) *testParam {
	domainID := uuid.New()
	taskList := &types.TaskList{
		Name: strings.ReplaceAll(uuid.New(), "-", ""), // metric tags are sanitized
	}
	tlID := tasklist.NewTestTaskListID(t, domainID, taskList.Name, taskType)
	taskListType := types.TaskListType(taskType)
	return &testParam{
		DomainID: domainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: uuid.New(),
			RunID:      uuid.New(),
		},
		TaskList:     taskList,
		TaskListID:   tlID,
		TaskListType: &taskListType,
		Identity:     uuid.New(),
		ActivityID:   uuid.New(),
		ActivityType: &types.ActivityType{
			Name: uuid.New(),
		},
		ActivityInput:    []byte(uuid.New()),
		ActivityHeader:   &types.Header{Fields: map[string][]byte{"tracing": []byte("tracing data")}},
		WorkflowType:     &types.WorkflowType{Name: uuid.New()},
		ScheduledEventID: 1412,
	}
}

type addTaskRequest struct {
	TaskType                      int
	DomainUUID                    string
	Execution                     *types.WorkflowExecution
	TaskList                      *types.TaskList
	ScheduleID                    int64
	ScheduleToStartTimeoutSeconds int32
	Source                        *types.TaskSource
	ForwardedFrom                 string
	PartitionConfig               map[string]string
}

type addTaskResponse struct {
	PartitionConfig *types.TaskListPartitionConfig
}

func addTask(engine *matchingEngineImpl, hCtx *handlerContext, request *addTaskRequest) (*addTaskResponse, error) {
	if request.TaskType == persistence.TaskListTypeActivity {
		resp, err := engine.AddActivityTask(hCtx, &types.AddActivityTaskRequest{
			SourceDomainUUID:              request.DomainUUID,
			DomainUUID:                    request.DomainUUID,
			Execution:                     request.Execution,
			TaskList:                      request.TaskList,
			ScheduleID:                    request.ScheduleID,
			ScheduleToStartTimeoutSeconds: &request.ScheduleToStartTimeoutSeconds,
			Source:                        request.Source,
			ForwardedFrom:                 request.ForwardedFrom,
			PartitionConfig:               request.PartitionConfig,
		})
		if err != nil {
			return nil, err
		}
		return &addTaskResponse{
			PartitionConfig: resp.PartitionConfig,
		}, nil
	}
	resp, err := engine.AddDecisionTask(hCtx, &types.AddDecisionTaskRequest{
		DomainUUID:                    request.DomainUUID,
		Execution:                     request.Execution,
		TaskList:                      request.TaskList,
		ScheduleID:                    request.ScheduleID,
		ScheduleToStartTimeoutSeconds: &request.ScheduleToStartTimeoutSeconds,
		Source:                        request.Source,
		ForwardedFrom:                 request.ForwardedFrom,
		PartitionConfig:               request.PartitionConfig,
	})
	if err != nil {
		return nil, err
	}
	return &addTaskResponse{
		PartitionConfig: resp.PartitionConfig,
	}, nil
}

type pollTaskRequest struct {
	TaskType         int
	DomainUUID       string
	PollerID         string
	TaskList         *types.TaskList
	Identity         string
	ForwardedFrom    string
	IsolationGroup   string
	TaskListMetadata *types.TaskListMetadata
	BinaryChecksum   string
}

type pollTaskResponse struct {
	TaskToken                       *common.TaskToken
	WorkflowExecution               *types.WorkflowExecution
	ActivityID                      string
	ActivityType                    *types.ActivityType
	Input                           []byte
	ScheduledTimestamp              *int64
	ScheduleToCloseTimeoutSeconds   *int32
	StartedTimestamp                *int64
	StartToCloseTimeoutSeconds      *int32
	HeartbeatTimeoutSeconds         *int32
	ScheduledTimestampOfThisAttempt *int64
	HeartbeatDetails                []byte
	WorkflowType                    *types.WorkflowType
	WorkflowDomain                  string
	Header                          *types.Header
	PreviousStartedEventID          *int64
	StartedEventID                  int64
	Attempt                         int64
	NextEventID                     int64
	BacklogCountHint                int64
	StickyExecutionEnabled          bool
	Query                           *types.WorkflowQuery
	DecisionInfo                    *types.TransientDecisionInfo
	WorkflowExecutionTaskList       *types.TaskList
	EventStoreVersion               int32
	BranchToken                     []byte
	Queries                         map[string]*types.WorkflowQuery
	AutoConfigHint                  *types.AutoConfigHint
}

func pollTask(engine *matchingEngineImpl, hCtx *handlerContext, request *pollTaskRequest) (*pollTaskResponse, error) {
	if request.TaskType == persistence.TaskListTypeActivity {
		resp, err := engine.PollForActivityTask(hCtx, &types.MatchingPollForActivityTaskRequest{
			DomainUUID: request.DomainUUID,
			PollerID:   request.PollerID,
			PollRequest: &types.PollForActivityTaskRequest{
				TaskList:         request.TaskList,
				Identity:         request.Identity,
				TaskListMetadata: request.TaskListMetadata,
			},
			IsolationGroup: request.IsolationGroup,
			ForwardedFrom:  request.ForwardedFrom,
		})
		if err != nil {
			return nil, err
		}
		var token *common.TaskToken
		if len(resp.TaskToken) > 0 {
			token, err = engine.tokenSerializer.Deserialize(resp.TaskToken)
			if err != nil {
				return nil, err
			}
		}
		return &pollTaskResponse{
			TaskToken:                       token,
			WorkflowExecution:               resp.WorkflowExecution,
			ActivityID:                      resp.ActivityID,
			ActivityType:                    resp.ActivityType,
			Input:                           resp.Input,
			ScheduledTimestamp:              resp.ScheduledTimestamp,
			ScheduleToCloseTimeoutSeconds:   resp.ScheduleToCloseTimeoutSeconds,
			StartedTimestamp:                resp.StartedTimestamp,
			StartToCloseTimeoutSeconds:      resp.StartToCloseTimeoutSeconds,
			HeartbeatTimeoutSeconds:         resp.HeartbeatTimeoutSeconds,
			Attempt:                         int64(resp.Attempt),
			ScheduledTimestampOfThisAttempt: resp.ScheduledTimestampOfThisAttempt,
			HeartbeatDetails:                resp.HeartbeatDetails,
			WorkflowType:                    resp.WorkflowType,
			WorkflowDomain:                  resp.WorkflowDomain,
			Header:                          resp.Header,
			AutoConfigHint:                  resp.AutoConfigHint,
		}, nil
	}
	resp, err := engine.PollForDecisionTask(hCtx, &types.MatchingPollForDecisionTaskRequest{
		DomainUUID: request.DomainUUID,
		PollerID:   request.PollerID,
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList:       request.TaskList,
			Identity:       request.Identity,
			BinaryChecksum: request.BinaryChecksum,
		},
		IsolationGroup: request.IsolationGroup,
		ForwardedFrom:  request.ForwardedFrom,
	})
	if err != nil {
		return nil, err
	}
	var token *common.TaskToken
	if len(resp.TaskToken) > 0 {
		token, err = engine.tokenSerializer.Deserialize(resp.TaskToken)
		if err != nil {
			return nil, err
		}
	}
	return &pollTaskResponse{
		TaskToken:                 token,
		WorkflowExecution:         resp.WorkflowExecution,
		WorkflowType:              resp.WorkflowType,
		PreviousStartedEventID:    resp.PreviousStartedEventID,
		StartedEventID:            resp.StartedEventID,
		Attempt:                   resp.Attempt,
		NextEventID:               resp.NextEventID,
		BacklogCountHint:          resp.BacklogCountHint,
		StickyExecutionEnabled:    resp.StickyExecutionEnabled,
		Query:                     resp.Query,
		DecisionInfo:              resp.DecisionInfo,
		WorkflowExecutionTaskList: resp.WorkflowExecutionTaskList,
		EventStoreVersion:         resp.EventStoreVersion,
		BranchToken:               resp.BranchToken,
		ScheduledTimestamp:        resp.ScheduledTimestamp,
		StartedTimestamp:          resp.StartedTimestamp,
		Queries:                   resp.Queries,
		AutoConfigHint:            resp.AutoConfigHint,
	}, nil
}

func isEmptyToken(token *common.TaskToken) bool {
	return token == nil || *token == common.TaskToken{}
}

func getIsolationGroupsHelper() []string {
	return []string{"zone-a", "zone-b"}
}
