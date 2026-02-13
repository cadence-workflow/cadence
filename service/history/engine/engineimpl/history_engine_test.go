// Copyright (c) 2017-2021 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/api/encoding"
	"go.uber.org/yarpc/api/transport"

	hclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	cc "github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/decision"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/ndc"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
	"github.com/uber/cadence/service/history/workflow"
)

type (
	engineSuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		mockShard            *shard.TestContext
		mockTxProcessor      *queue.MockProcessor
		mockTimerProcessor   *queue.MockProcessor
		mockDomainCache      *cache.MockDomainCache
		mockHistoryClient    *hclient.MockClient
		mockMatchingClient   *matching.MockClient
		mockEventsReapplier  *ndc.MockEventsReapplier
		mockWorkflowResetter *reset.MockWorkflowResetter

		mockHistoryEngine *historyEngineImpl
		mockExecutionMgr  *mocks.ExecutionManager
		mockHistoryV2Mgr  *mocks.HistoryV2Manager
		mockShardManager  *mocks.ShardManager

		eventsCache events.Cache
		config      *config.Config
	}
)

func TestEngineSuite(t *testing.T) {
	s := new(engineSuite)
	suite.Run(t, s)
}

func (s *engineSuite) SetupSuite() {
	s.config = config.NewForTest()
}

func (s *engineSuite) TearDownSuite() {
}

func (s *engineSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockTxProcessor = queue.NewMockProcessor(s.controller)
	s.mockTimerProcessor = queue.NewMockProcessor(s.controller)
	s.mockEventsReapplier = ndc.NewMockEventsReapplier(s.controller)
	s.mockWorkflowResetter = reset.NewMockWorkflowResetter(s.controller)
	s.mockTxProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).AnyTimes()
	s.mockTimerProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).AnyTimes()

	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			RangeID:          1,
			TransferAckLevel: 0,
		},
		s.config,
	)
	s.eventsCache = events.NewCache(
		s.mockShard.GetShardID(),
		s.mockShard.GetHistoryManager(),
		s.config,
		s.mockShard.GetLogger(),
		s.mockShard.GetMetricsClient(),
		s.mockDomainCache,
	)
	s.mockShard.SetEventsCache(s.eventsCache)

	s.mockMatchingClient = s.mockShard.Resource.MatchingClient
	s.mockHistoryClient = s.mockShard.Resource.HistoryClient
	s.mockExecutionMgr = s.mockShard.Resource.ExecutionMgr
	s.mockHistoryV2Mgr = s.mockShard.Resource.HistoryMgr
	s.mockShardManager = s.mockShard.Resource.ShardMgr
	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(constants.TestLocalDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestActiveActiveDomainID).Return(constants.TestActiveActiveDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(constants.TestDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(constants.TestActiveActiveDomainID).Return(constants.TestDomainName, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(constants.TestDomainName).Return(constants.TestLocalDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainID(constants.TestDomainName).Return(constants.TestDomainID, nil).AnyTimes()

	historyEventNotifier := events.NewNotifier(
		clock.NewRealTimeSource(),
		s.mockShard.Resource.MetricsClient,
		func(workflowID string) int {
			return len(workflowID)
		},
	)

	h := &historyEngineImpl{
		currentClusterName:   s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		shard:                s.mockShard,
		timeSource:           s.mockShard.GetTimeSource(),
		clusterMetadata:      s.mockShard.Resource.ClusterMetadata,
		executionManager:     s.mockExecutionMgr,
		historyV2Mgr:         s.mockHistoryV2Mgr,
		executionCache:       execution.NewCache(s.mockShard),
		logger:               s.mockShard.GetLogger(),
		metricsClient:        s.mockShard.GetMetricsClient(),
		tokenSerializer:      common.NewJSONTaskTokenSerializer(),
		historyEventNotifier: historyEventNotifier,
		config:               config.NewForTest(),
		queueProcessors: map[persistence.HistoryTaskCategory]queue.Processor{
			persistence.HistoryTaskCategoryTransfer: s.mockTxProcessor,
			persistence.HistoryTaskCategoryTimer:    s.mockTimerProcessor,
		},
		clientChecker:        cc.NewVersionChecker(),
		eventsReapplier:      s.mockEventsReapplier,
		workflowResetter:     s.mockWorkflowResetter,
		activeClusterManager: s.mockShard.Resource.ActiveClusterMgr,
	}
	s.mockShard.SetEngine(h)
	h.decisionHandler = decision.NewHandler(s.mockShard, h.executionCache, h.tokenSerializer)

	h.historyEventNotifier.Start()

	s.mockHistoryEngine = h
}

func (s *engineSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
	s.mockHistoryEngine.historyEventNotifier.Stop()
}

func (s *engineSuite) TestGetMutableStateSync() {
	ctx := context.Background()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	msBuilder.SetVersionHistories(&persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("token"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 2,
						Version: 1,
					},
				},
			},
		},
	})
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// test get the next event ID instantly
	response, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  &workflowExecution,
	})
	s.Nil(err)
	s.Equal(int64(4), response.GetNextEventID())
}

func (s *engineSuite) TestGetMutableState_IntestRunID() {
	ctx := context.Background()

	execution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      "run-id-not-valid-uuid",
	}

	_, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  &execution,
	})
	s.Equal(constants.ErrRunIDNotValid, err)
}

func (s *engineSuite) TestGetMutableState_EmptyRunID() {
	ctx := context.Background()

	execution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
	}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	_, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  &execution,
	})
	s.Equal(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestGetMutableState_NoVersionHistories() {
	ctx := context.Background()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      constants.TestRunID,
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)

	// simulate having no version histories
	s.Require().NoError(msBuilder.SetVersionHistories(nil))

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// return immediately, since the expected next event ID appears
	mutableState, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID: constants.TestDomainID,
		Execution:  &workflowExecution,

		// we request for the specific history version, but there are no history versions for the execution
		VersionHistoryItem: &types.VersionHistoryItem{
			EventID: 1,
			Version: 1,
		},
	})
	s.ErrorContains(err, "version histories do not exist")
	s.Nil(mutableState)
}

func (s *engineSuite) TestGetMutableStateLongPoll() {
	ctx := context.Background()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	msBuilder.SetVersionHistories(&persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("token"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 2,
						Version: 1,
					},
				},
			},
		},
	})
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// test long poll on next event ID change
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	asycWorkflowUpdate := func(delay time.Duration) {
		taskToken, _ := json.Marshal(&common.TaskToken{
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
			ScheduleID: 2,
		})
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

		timer := time.NewTimer(delay)

		<-timer.C
		_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
			DomainUUID: constants.TestDomainID,
			CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
				TaskToken: taskToken,
				Identity:  identity,
			},
		})
		s.Nil(err)
		waitGroup.Done()
		// right now the next event ID is 5
	}

	// return immediately, since the expected next event ID appears
	response, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID:          constants.TestDomainID,
		Execution:           &workflowExecution,
		ExpectedNextEventID: 3,
		VersionHistoryItem: &types.VersionHistoryItem{
			EventID: 1,
			Version: 1,
		},
	})
	s.Nil(err)
	s.Equal(int64(4), response.NextEventID)

	// long poll, new event happen before long poll timeout
	go asycWorkflowUpdate(time.Second * 2)
	start := time.Now()
	pollResponse, err := s.mockHistoryEngine.PollMutableState(ctx, &types.PollMutableStateRequest{
		DomainUUID:          constants.TestDomainID,
		Execution:           &workflowExecution,
		ExpectedNextEventID: 4,
		VersionHistoryItem: &types.VersionHistoryItem{
			EventID: 1,
			Version: 1,
		},
	})
	s.True(time.Now().After(start.Add(time.Second * 1)))
	s.Nil(err)
	s.Equal(int64(5), pollResponse.GetNextEventID())
	waitGroup.Wait()
}

func (s *engineSuite) TestGetMutableStateLongPoll_CurrentBranchChanged() {
	ctx := context.Background()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	msBuilder.SetVersionHistories(&persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("token"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 2,
						Version: 1,
					},
				},
			},
		},
	})
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// test long poll on next event ID change
	asyncBranchTokenUpdate := func(delay time.Duration) {
		timer := time.NewTimer(delay)
		<-timer.C
		newExecution := &types.WorkflowExecution{
			WorkflowID: workflowExecution.WorkflowID,
			RunID:      workflowExecution.RunID,
		}
		s.mockHistoryEngine.historyEventNotifier.NotifyNewHistoryEvent(events.NewNotification(
			"constants.TestDomainID",
			newExecution,
			int64(1),
			int64(4),
			int64(1),
			persistence.WorkflowStateCreated,
			persistence.WorkflowCloseStatusNone,
			nil))
	}

	// return immediately, since the expected next event ID appears
	response0, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID:          constants.TestDomainID,
		Execution:           &workflowExecution,
		ExpectedNextEventID: 3,
	})
	s.Nil(err)
	s.Equal(int64(4), response0.GetNextEventID())

	// long poll, new event happen before long poll timeout
	go asyncBranchTokenUpdate(time.Second * 2)
	start := time.Now()
	response1, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID:          constants.TestDomainID,
		Execution:           &workflowExecution,
		ExpectedNextEventID: 10,
	})
	s.True(time.Now().After(start.Add(time.Second * 1)))
	s.Nil(err)
	s.Equal(response0.GetCurrentBranchToken(), response1.GetCurrentBranchToken())
}

func (s *engineSuite) TestGetMutableStateLongPollTimeout() {
	ctx := context.Background()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-get-workflow-execution-event-id",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)

	msBuilder.SetVersionHistories(&persistence.VersionHistories{
		CurrentVersionHistoryIndex: 0,
		Histories: []*persistence.VersionHistory{
			{
				BranchToken: []byte("token"),
				Items: []*persistence.VersionHistoryItem{
					{
						EventID: 2,
						Version: 1,
					},
				},
			},
		},
	})
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// long poll, no event happen after long poll timeout
	response, err := s.mockHistoryEngine.GetMutableState(ctx, &types.GetMutableStateRequest{
		DomainUUID:          constants.TestDomainID,
		Execution:           &workflowExecution,
		ExpectedNextEventID: 4,
	})
	s.Nil(err)
	s.Equal(int64(4), response.GetNextEventID())
}

func (s *engineSuite) TestQueryWorkflow_RejectBasedOnNotEnabled() {
	s.mockHistoryEngine.config.EnableConsistentQueryByDomain = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.Nil(resp)
	s.Equal(workflow.ErrConsistentQueryNotEnabled, err)

	s.mockHistoryEngine.config.EnableConsistentQueryByDomain = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	s.mockHistoryEngine.config.EnableConsistentQuery = dynamicproperties.GetBoolPropertyFn(false)
	resp, err = s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.Nil(resp)
	s.Equal(workflow.ErrConsistentQueryNotEnabled, err)
}

func (s *engineSuite) TestQueryWorkflow_RejectBasedOnCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_RejectBasedOnCompleted",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	event := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	di.StartedID = event.ID
	event = test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, di.StartedID, nil, "some random identity")
	test.AddCompleteWorkflowEvent(msBuilder, event.ID, nil)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:            &workflowExecution,
			Query:                &types.WorkflowQuery{},
			QueryRejectCondition: types.QueryRejectConditionNotOpen.Ptr(),
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.NoError(err)
	s.Nil(resp.GetResponse().QueryResult)
	s.NotNil(resp.GetResponse().QueryRejected)
	s.Equal(types.WorkflowExecutionCloseStatusCompleted.Ptr(), resp.GetResponse().GetQueryRejected().CloseStatus)
}

func (s *engineSuite) TestQueryWorkflow_RejectBasedOnFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_RejectBasedOnFailed",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	event := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	di.StartedID = event.ID
	event = test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, di.StartedID, nil, "some random identity")
	test.AddFailWorkflowEvent(msBuilder, event.ID, "failure reason", []byte{1, 2, 3})
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:            &workflowExecution,
			Query:                &types.WorkflowQuery{},
			QueryRejectCondition: types.QueryRejectConditionNotOpen.Ptr(),
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.NoError(err)
	s.Nil(resp.GetResponse().QueryResult)
	s.NotNil(resp.GetResponse().QueryRejected)
	s.Equal(types.WorkflowExecutionCloseStatusFailed.Ptr(), resp.GetResponse().GetQueryRejected().CloseStatus)

	request = &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:            &workflowExecution,
			Query:                &types.WorkflowQuery{},
			QueryRejectCondition: types.QueryRejectConditionNotCompletedCleanly.Ptr(),
		},
	}
	resp, err = s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.NoError(err)
	s.Nil(resp.GetResponse().QueryResult)
	s.NotNil(resp.GetResponse().QueryRejected)
	s.Equal(types.WorkflowExecutionCloseStatusFailed.Ptr(), resp.GetResponse().GetQueryRejected().CloseStatus)
}

func (s *engineSuite) TestQueryWorkflow_FirstDecisionNotCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_FirstDecisionNotCompleted",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution: &workflowExecution,
			Query:     &types.WorkflowQuery{},
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.Equal(workflow.ErrQueryWorkflowBeforeFirstDecision, err)
	s.Nil(resp)
}

func (s *engineSuite) TestQueryWorkflow_DirectlyThroughMatching() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_DirectlyThroughMatching",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()
	s.mockMatchingClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(&types.MatchingQueryWorkflowResponse{QueryResult: []byte{1, 2, 3}}, nil)
	s.mockHistoryEngine.matchingClient = s.mockMatchingClient
	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution: &workflowExecution,
			Query:     &types.WorkflowQuery{},
			// since workflow is open this filter does not reject query
			QueryRejectCondition:  types.QueryRejectConditionNotOpen.Ptr(),
			QueryConsistencyLevel: types.QueryConsistencyLevelEventual.Ptr(),
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.NoError(err)
	s.NotNil(resp.GetResponse().QueryResult)
	s.Nil(resp.GetResponse().QueryRejected)
	s.Equal([]byte{1, 2, 3}, resp.GetResponse().GetQueryResult())
}

func (s *engineSuite) TestQueryWorkflow_DecisionTaskDispatch_Timeout() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_DecisionTaskDispatch_Timeout",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()
	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution: &workflowExecution,
			Query:     &types.WorkflowQuery{},
			// since workflow is open this filter does not reject query
			QueryRejectCondition:  types.QueryRejectConditionNotOpen.Ptr(),
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		resp, err := s.mockHistoryEngine.QueryWorkflow(ctx, request)
		s.Error(err)
		s.Nil(resp)
		wg.Done()
	}()

	<-time.After(time.Second)
	builder := s.getBuilder(constants.TestDomainID, workflowExecution)
	s.NotNil(builder)
	qr := builder.GetQueryRegistry()
	s.True(qr.HasBufferedQuery())
	s.False(qr.HasCompletedQuery())
	s.False(qr.HasUnblockedQuery())
	s.False(qr.HasFailedQuery())
	wg.Wait()
	s.False(qr.HasBufferedQuery())
	s.False(qr.HasCompletedQuery())
	s.False(qr.HasUnblockedQuery())
	s.False(qr.HasFailedQuery())
}

func (s *engineSuite) TestQueryWorkflow_ConsistentQueryBufferFull() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_ConsistentQueryBufferFull",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	// buffer query so that when types.QueryWorkflow is called buffer is already full
	ctx, release, err := s.mockHistoryEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(constants.TestDomainID, workflowExecution)
	s.NoError(err)
	loadedMS, err := ctx.LoadWorkflowExecution(context.Background())
	s.NoError(err)
	qr := query.NewRegistry()
	qr.BufferQuery(&types.WorkflowQuery{})
	loadedMS.SetQueryRegistry(qr)
	release(nil)

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:             &workflowExecution,
			Query:                 &types.WorkflowQuery{},
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.Nil(resp)
	s.Equal(workflow.ErrConsistentQueryBufferExceeded, err)
}

func (s *engineSuite) TestQueryWorkflow_DecisionTaskDispatch_Complete() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_DecisionTaskDispatch_Complete",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	asyncQueryUpdate := func(delay time.Duration, answer []byte) {
		defer waitGroup.Done()
		<-time.After(delay)
		builder := s.getBuilder(constants.TestDomainID, workflowExecution)
		s.NotNil(builder)
		qr := builder.GetQueryRegistry()
		buffered := qr.GetBufferedIDs()
		for _, id := range buffered {
			resultType := types.QueryResultTypeAnswered
			completedTerminationState := &query.TerminationState{
				TerminationType: query.TerminationTypeCompleted,
				QueryResult: &types.WorkflowQueryResult{
					ResultType: &resultType,
					Answer:     answer,
				},
			}
			err := qr.SetTerminationState(id, completedTerminationState)
			s.NoError(err)
			state, err := qr.GetTerminationState(id)
			s.NoError(err)
			s.Equal(query.TerminationTypeCompleted, state.TerminationType)
		}
	}

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:             &workflowExecution,
			Query:                 &types.WorkflowQuery{},
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}
	go asyncQueryUpdate(time.Second*2, []byte{1, 2, 3})
	start := time.Now()
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.True(time.Now().After(start.Add(time.Second)))
	s.NoError(err)
	s.Equal([]byte{1, 2, 3}, resp.GetResponse().GetQueryResult())
	builder := s.getBuilder(constants.TestDomainID, workflowExecution)
	s.NotNil(builder)
	qr := builder.GetQueryRegistry()
	s.False(qr.HasBufferedQuery())
	s.False(qr.HasCompletedQuery())
	waitGroup.Wait()
}

func (s *engineSuite) TestQueryWorkflow_DecisionTaskDispatch_Complete_ActiveActive() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestActiveActiveDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestActiveActiveDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_DecisionTaskDispatch_Complete_ActiveActive",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestActiveActiveDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, &constants.TestActiveClusterSelectionPolicy)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	asyncQueryUpdate := func(delay time.Duration, answer []byte) {
		defer waitGroup.Done()
		<-time.After(delay)
		builder := s.getBuilder(constants.TestActiveActiveDomainID, workflowExecution)
		s.NotNil(builder)
		qr := builder.GetQueryRegistry()
		buffered := qr.GetBufferedIDs()
		for _, id := range buffered {
			resultType := types.QueryResultTypeAnswered
			completedTerminationState := &query.TerminationState{
				TerminationType: query.TerminationTypeCompleted,
				QueryResult: &types.WorkflowQueryResult{
					ResultType: &resultType,
					Answer:     answer,
				},
			}
			err := qr.SetTerminationState(id, completedTerminationState)
			s.NoError(err)
			state, err := qr.GetTerminationState(id)
			s.NoError(err)
			s.Equal(query.TerminationTypeCompleted, state.TerminationType)
		}
	}

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestActiveActiveDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:             &workflowExecution,
			Query:                 &types.WorkflowQuery{},
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}
	go asyncQueryUpdate(time.Second*2, []byte{1, 2, 3})
	start := time.Now()
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.True(time.Now().After(start.Add(time.Second)))
	s.NoError(err)
	s.Equal([]byte{1, 2, 3}, resp.GetResponse().GetQueryResult())
	builder := s.getBuilder(constants.TestActiveActiveDomainID, workflowExecution)
	s.NotNil(builder)
	qr := builder.GetQueryRegistry()
	s.False(qr.HasBufferedQuery())
	s.False(qr.HasCompletedQuery())
	waitGroup.Wait()
}

func (s *engineSuite) TestQueryWorkflow_DecisionTaskDispatch_Unblocked() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "TestQueryWorkflow_DecisionTaskDispatch_Unblocked",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)
	di = test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tasklist, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gweResponse, nil).Once()
	s.mockMatchingClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(&types.MatchingQueryWorkflowResponse{QueryResult: []byte{1, 2, 3}}, nil)
	s.mockHistoryEngine.matchingClient = s.mockMatchingClient
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	asyncQueryUpdate := func(delay time.Duration, answer []byte) {
		defer waitGroup.Done()
		<-time.After(delay)
		builder := s.getBuilder(constants.TestDomainID, workflowExecution)
		s.NotNil(builder)
		qr := builder.GetQueryRegistry()
		buffered := qr.GetBufferedIDs()
		for _, id := range buffered {
			s.NoError(qr.SetTerminationState(id, &query.TerminationState{TerminationType: query.TerminationTypeUnblocked}))
			state, err := qr.GetTerminationState(id)
			s.NoError(err)
			s.Equal(query.TerminationTypeUnblocked, state.TerminationType)
		}
	}

	request := &types.HistoryQueryWorkflowRequest{
		DomainUUID: constants.TestDomainID,
		Request: &types.QueryWorkflowRequest{
			Execution:             &workflowExecution,
			Query:                 &types.WorkflowQuery{},
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}
	go asyncQueryUpdate(time.Second*2, []byte{1, 2, 3})
	start := time.Now()
	resp, err := s.mockHistoryEngine.QueryWorkflow(context.Background(), request)
	s.True(time.Now().After(start.Add(time.Second)))
	s.NoError(err)
	s.Equal([]byte{1, 2, 3}, resp.GetResponse().GetQueryResult())
	builder := s.getBuilder(constants.TestDomainID, workflowExecution)
	s.NotNil(builder)
	qr := builder.GetQueryRegistry()
	s.False(qr.HasBufferedQuery())
	s.False(qr.HasCompletedQuery())
	s.False(qr.HasUnblockedQuery())
	waitGroup.Wait()
}

func (s *engineSuite) TestRespondDecisionTaskCompletedInvalidToken() {

	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        invalidToken,
			Decisions:        nil,
			ExecutionContext: nil,
			Identity:         identity,
		},
	})

	s.NotNil(err)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfNoExecution() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfGetExecutionFailed() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("FAILED")).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedUpdateExecutionFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfTaskCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	startedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID, startedEvent.ID, nil, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfTaskNotStarted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedConflictOnUpdate() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	executionContext := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")
	activity2Result := []byte("activity2_result")
	activity3ID := "activity3"
	activity3Type := "activity_type3"
	activity3Input := []byte("input3")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di1 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di1.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di1.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID,
		activity1ID, activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity2ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID,
		activity2ID, activity2Type, tl, activity2Input, 100, 10, 1, 5)
	activity1StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	activity2StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity2ScheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity1ScheduledEvent.ID,
		activity1StartedEvent.ID, activity1Result, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent2 := test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: di2.ScheduleID,
	})

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
		ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
			ActivityID:                    activity3ID,
			ActivityType:                  &types.ActivityType{Name: activity3Type},
			TaskList:                      &types.TaskList{Name: tl},
			Input:                         activity3Input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	test.AddActivityTaskCompletedEvent(msBuilder, activity2ScheduledEvent.ID,
		activity2StartedEvent.ID, activity2Result, identity)

	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}},
		&persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	s.Equal(int64(16), ms2.ExecutionInfo.NextEventID)
	s.Equal(decisionStartedEvent2.ID, ms2.ExecutionInfo.LastProcessedEvent)
	s.Equal(executionContext, ms2.ExecutionInfo.ExecutionContext)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	activity3Attributes := s.getActivityScheduledEvent(executionBuilder, 13).ActivityTaskScheduledEventAttributes
	s.Equal(activity3ID, activity3Attributes.ActivityID)
	s.Equal(activity3Type, activity3Attributes.ActivityType.Name)
	s.Equal(int64(12), activity3Attributes.DecisionTaskCompletedEventID)
	s.Equal(tl, activity3Attributes.TaskList.Name)
	s.Equal(activity3Input, activity3Attributes.Input)
	s.Equal(int32(100), *activity3Attributes.ScheduleToCloseTimeoutSeconds)
	s.Equal(int32(10), *activity3Attributes.ScheduleToStartTimeoutSeconds)
	s.Equal(int32(50), *activity3Attributes.StartToCloseTimeoutSeconds)
	s.Equal(int32(5), *activity3Attributes.HeartbeatTimeoutSeconds)

	di, ok := executionBuilder.GetDecisionInfo(15)
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
}

func (s *engineSuite) TestValidateSignalRequest() {
	workflowType := "testType"
	input := []byte("input")
	startRequest := &types.StartWorkflowExecutionRequest{
		WorkflowID:                          "ID",
		WorkflowType:                        &types.WorkflowType{Name: workflowType},
		TaskList:                            &types.TaskList{Name: "taskptr"},
		Input:                               input,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(10),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            "identity",
	}

	err := s.mockHistoryEngine.validateStartWorkflowExecutionRequest(startRequest, 0)
	s.Error(err, "startRequest doesn't have request id, it should error out")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedMaxAttemptsExceeded() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	input := []byte("input")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
		ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
			ActivityID:                    "activity1",
			ActivityType:                  &types.ActivityType{Name: "activity_type1"},
			TaskList:                      &types.TaskList{Name: tl},
			Input:                         input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	for i := 0; i < workflow.ConditionalRetryCount; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}},
			&persistence.ConditionFailedError{}).Once()
	}

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.NotNil(err)
	s.Equal(workflow.ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedCompleteWorkflowFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	executionContext := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")
	activity2Result := []byte("activity2_result")
	workflowResult := []byte("workflow result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity, nil)
	di1 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di1.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di1.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID,
		activity1ID, activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity2ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID,
		activity2ID, activity2Type, tl, activity2Input, 100, 10, 1, 5)
	activity1StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	activity2StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity2ScheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity1ScheduledEvent.ID,
		activity1StartedEvent.ID, activity1Result, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity2ScheduledEvent.ID,
		activity2StartedEvent.ID, activity2Result, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: di2.ScheduleID,
	})

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
		CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
			Result: workflowResult,
		},
	}}

	for i := 0; i < 2; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(15), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(decisionStartedEvent1.ID, executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Empty(executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
	di3, ok := executionBuilder.GetDecisionInfo(executionBuilder.GetExecutionInfo().NextEventID - 1)
	s.True(ok)
	s.Equal(executionBuilder.GetExecutionInfo().NextEventID-1, di3.ScheduleID)
	s.Equal(int64(0), di3.Attempt)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedFailWorkflowFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	executionContext := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")
	activity2Result := []byte("activity2_result")
	reason := "workflow fail reason"
	details := []byte("workflow fail details")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity, nil)
	di1 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di1.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di1.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity2ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 1, 5)
	activity1StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	activity2StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity2ScheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity1ScheduledEvent.ID,
		activity1StartedEvent.ID, activity1Result, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity2ScheduledEvent.ID,
		activity2StartedEvent.ID, activity2Result, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: di2.ScheduleID,
	})

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
		FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
			Reason:  &reason,
			Details: details,
		},
	}}

	for i := 0; i < 2; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(15), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(decisionStartedEvent1.ID, executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Empty(executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
	di3, ok := executionBuilder.GetDecisionInfo(executionBuilder.GetExecutionInfo().NextEventID - 1)
	s.True(ok)
	s.Equal(executionBuilder.GetExecutionInfo().NextEventID-1, di3.ScheduleID)
	s.Equal(int64(0), di3.Attempt)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedBadDecisionAttributes() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	executionContext := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity, nil)
	di1 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di1.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di1.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity1StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activity1ScheduledEvent.ID,
		activity1StartedEvent.ID, activity1Result, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: di2.ScheduleID,
	})

	// Decision with nil attributes
	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
	}}

	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil,
	).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err)
}

// This test unit tests the activity schedule timeout validation logic of HistoryEngine's RespondDecisionTaskComplete function.
// An scheduled activity decision has 3 timeouts: ScheduleToClose, ScheduleToStart and StartToClose.
// This test verifies that when either ScheduleToClose or ScheduleToStart and StartToClose are specified,
// HistoryEngine's validateActivityScheduleAttribute will deduce the missing timeout and fill it in
// instead of returning a BadRequest error and only when all three are missing should a BadRequest be returned.
func (s *engineSuite) TestRespondDecisionTaskCompletedSingleActivityScheduledAttribute() {
	workflowTimeout := int32(100)
	testIterationVariables := []struct {
		scheduleToClose         *int32
		scheduleToStart         *int32
		startToClose            *int32
		heartbeat               *int32
		expectedScheduleToClose int32
		expectedScheduleToStart int32
		expectedStartToClose    int32
		expectDecisionFail      bool
	}{
		// No ScheduleToClose timeout, will use ScheduleToStart + StartToClose
		{nil, common.Int32Ptr(3), common.Int32Ptr(7), nil,
			3 + 7, 3, 7, false},
		// Has ScheduleToClose timeout but not ScheduleToStart or StartToClose,
		// will use ScheduleToClose for ScheduleToStart and StartToClose
		{common.Int32Ptr(7), nil, nil, nil,
			7, 7, 7, false},
		// No ScheduleToClose timeout, ScheduleToStart or StartToClose, expect error return
		{nil, nil, nil, nil,
			0, 0, 0, true},
		// Negative ScheduleToClose, expect error return
		{common.Int32Ptr(-1), nil, nil, nil,
			0, 0, 0, true},
		// Negative ScheduleToStart, expect error return
		{nil, common.Int32Ptr(-1), nil, nil,
			0, 0, 0, true},
		// Negative StartToClose, expect error return
		{nil, nil, common.Int32Ptr(-1), nil,
			0, 0, 0, true},
		// Negative HeartBeat, expect error return
		{nil, nil, nil, common.Int32Ptr(-1),
			0, 0, 0, true},
		// Use workflow timeout
		{common.Int32Ptr(workflowTimeout), nil, nil, nil,
			workflowTimeout, workflowTimeout, workflowTimeout, false},
		// Timeout larger than workflow timeout
		{common.Int32Ptr(workflowTimeout + 1), nil, nil, nil,
			workflowTimeout, workflowTimeout, workflowTimeout, false},
		{nil, common.Int32Ptr(workflowTimeout + 1), nil, nil,
			0, 0, 0, true},
		{nil, nil, common.Int32Ptr(workflowTimeout + 1), nil,
			0, 0, 0, true},
		{nil, nil, nil, common.Int32Ptr(workflowTimeout + 1),
			0, 0, 0, true},
		// No ScheduleToClose timeout, will use ScheduleToStart + StartToClose, but exceed limit
		{nil, common.Int32Ptr(workflowTimeout), common.Int32Ptr(10), nil,
			workflowTimeout, workflowTimeout, 10, false},
	}

	for _, iVar := range testIterationVariables {
		testActiveClusterInfo := &types.ActiveClusterInfo{
			ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
			FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
		}
		s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

		we := types.WorkflowExecution{
			WorkflowID: "wId",
			RunID:      constants.TestRunID,
		}
		tl := "testTaskList"
		taskToken, _ := json.Marshal(&common.TaskToken{
			WorkflowID: "wId",
			RunID:      we.GetRunID(),
			ScheduleID: 2,
		})
		identity := "testIdentity"
		executionContext := []byte("context")
		input := []byte("input")

		msBuilder := execution.NewMutableStateBuilderWithEventV2(
			s.mockHistoryEngine.shard,
			testlogger.New(s.Suite.T()),
			we.GetRunID(),
			constants.TestLocalDomainEntry,
		)
		test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), workflowTimeout, 200, identity, nil)
		di := test.AddDecisionTaskScheduledEvent(msBuilder)
		test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

		decisions := []*types.Decision{{
			DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
			ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
				ActivityID:                    "activity1",
				ActivityType:                  &types.ActivityType{Name: "activity_type1"},
				TaskList:                      &types.TaskList{Name: tl},
				Input:                         input,
				ScheduleToCloseTimeoutSeconds: iVar.scheduleToClose,
				ScheduleToStartTimeoutSeconds: iVar.scheduleToStart,
				StartToCloseTimeoutSeconds:    iVar.startToClose,
				HeartbeatTimeoutSeconds:       iVar.heartbeat,
			},
		}}

		gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
		if iVar.expectDecisionFail {
			gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
		}

		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

		_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
			DomainUUID: constants.TestDomainID,
			CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
				TaskToken:        taskToken,
				Decisions:        decisions,
				ExecutionContext: executionContext,
				Identity:         identity,
			},
		})

		s.Nil(err, s.printHistory(msBuilder))
		executionBuilder := s.getBuilder(constants.TestDomainID, we)
		if !iVar.expectDecisionFail {
			s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
			s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
			s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
			s.False(executionBuilder.HasPendingDecision())

			activity1Attributes := s.getActivityScheduledEvent(executionBuilder, int64(5)).ActivityTaskScheduledEventAttributes
			s.Equal(iVar.expectedScheduleToClose, activity1Attributes.GetScheduleToCloseTimeoutSeconds())
			s.Equal(iVar.expectedScheduleToStart, activity1Attributes.GetScheduleToStartTimeoutSeconds())
			s.Equal(iVar.expectedStartToClose, activity1Attributes.GetStartToCloseTimeoutSeconds())
		} else {
			s.Equal(int64(5), executionBuilder.GetExecutionInfo().NextEventID)
			s.Equal(commonconstants.EmptyEventID, executionBuilder.GetExecutionInfo().LastProcessedEvent)
			s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
			s.True(executionBuilder.HasPendingDecision())
		}
		s.TearDownTest()
		s.SetupTest()
	}
}

func (s *engineSuite) TestRespondDecisionTaskCompletedBadBinary() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	domainID := uuid.New()
	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: constants.TestDomainName},
		&persistence.DomainConfig{
			Retention: 2,
			BadBinaries: types.BadBinaries{
				Binaries: map[string]*types.BadBinaryInfo{
					"test-bad-binary": {},
				},
			},
		},
		cluster.TestCurrentClusterName,
	)

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		domainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	var decisions []*types.Decision

	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: execution.CreatePersistenceMutableState(s.T(), msBuilder)}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
	s.mockDomainCache.EXPECT().GetDomainByID(domainID).Return(domainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(domainID).Return(constants.TestDomainName, nil).AnyTimes()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: domainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
			BinaryChecksum:   "test-bad-binary",
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(5), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(commonconstants.EmptyEventID, executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Empty(executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedSingleActivityScheduledDecision() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	input := []byte("input")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
		ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
			ActivityID:                    "activity1",
			ActivityType:                  &types.ActivityType{Name: "activity_type1"},
			TaskList:                      &types.TaskList{Name: tl},
			Input:                         input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())

	activity1Attributes := s.getActivityScheduledEvent(executionBuilder, int64(5)).ActivityTaskScheduledEventAttributes
	s.Equal("activity1", activity1Attributes.ActivityID)
	s.Equal("activity_type1", activity1Attributes.ActivityType.Name)
	s.Equal(int64(4), activity1Attributes.DecisionTaskCompletedEventID)
	s.Equal(tl, activity1Attributes.TaskList.Name)
	s.Equal(input, activity1Attributes.Input)
	s.Equal(int32(100), *activity1Attributes.ScheduleToCloseTimeoutSeconds)
	s.Equal(int32(10), *activity1Attributes.ScheduleToStartTimeoutSeconds)
	s.Equal(int32(50), *activity1Attributes.StartToCloseTimeoutSeconds)
	s.Equal(int32(5), *activity1Attributes.HeartbeatTimeoutSeconds)
}

func (s *engineSuite) TestRespondDecisionTaskCompleted_DecisionHeartbeatTimeout() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	msBuilder.GetExecutionInfo().DecisionOriginalScheduledTimestamp = time.Now().Add(-time.Hour).UnixNano()

	decisions := []*types.Decision{}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			ForceCreateNewDecisionTask: true,
			TaskToken:                  taskToken,
			Decisions:                  decisions,
			ExecutionContext:           executionContext,
			Identity:                   identity,
		},
	})
	s.Error(err, "decision heartbeat timeout")
}

func (s *engineSuite) TestRespondDecisionTaskCompleted_DecisionHeartbeatNotTimeout() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	msBuilder.GetExecutionInfo().DecisionOriginalScheduledTimestamp = time.Now().Add(-time.Minute).UnixNano()

	decisions := []*types.Decision{}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			ForceCreateNewDecisionTask: true,
			TaskToken:                  taskToken,
			Decisions:                  decisions,
			ExecutionContext:           executionContext,
			Identity:                   identity,
		},
	})
	s.Nil(err)
}

func (s *engineSuite) TestRespondDecisionTaskCompleted_DecisionHeartbeatNotTimeout_ZeroOrignalScheduledTime() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	msBuilder.GetExecutionInfo().DecisionOriginalScheduledTimestamp = 0

	decisions := []*types.Decision{}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			ForceCreateNewDecisionTask: true,
			TaskToken:                  taskToken,
			Decisions:                  decisions,
			ExecutionContext:           executionContext,
			Identity:                   identity,
		},
	})
	s.Nil(err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedCompleteWorkflowSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	workflowResult := []byte("success")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
		CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
			Result: workflowResult,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateCompleted, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedFailWorkflowSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	details := []byte("fail workflow details")
	reason := "fail workflow reason"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
		FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
			Reason:  &reason,
			Details: details,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(persistence.WorkflowStateCompleted, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedSignalExternalWorkflowSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeSignalExternalWorkflowExecution.Ptr(),
		SignalExternalWorkflowExecutionDecisionAttributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
			Domain: constants.TestDomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: we.WorkflowID,
				RunID:      we.RunID,
			},
			SignalName: "signal",
			Input:      []byte("test input"),
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedStartChildWorkflowWithAbandonPolicy() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	abandon := types.ParentClosePolicyAbandon
	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
		StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
			Domain:     constants.TestDomainName,
			WorkflowID: "child-workflow-id",
			WorkflowType: &types.WorkflowType{
				Name: "child-workflow-type",
			},
			ParentClosePolicy: &abandon,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(int(1), len(executionBuilder.GetPendingChildExecutionInfos()))
	var childID int64
	for c := range executionBuilder.GetPendingChildExecutionInfos() {
		childID = c
		break
	}
	s.Equal("child-workflow-id", executionBuilder.GetPendingChildExecutionInfos()[childID].StartedWorkflowID)
	s.Equal(types.ParentClosePolicyAbandon, executionBuilder.GetPendingChildExecutionInfos()[childID].ParentClosePolicy)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedStartChildWorkflowWithTerminatePolicy() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	terminate := types.ParentClosePolicyTerminate
	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
		StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
			Domain:     constants.TestDomainName,
			WorkflowID: "child-workflow-id",
			WorkflowType: &types.WorkflowType{
				Name: "child-workflow-type",
			},
			ParentClosePolicy: &terminate,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(executionContext, executionBuilder.GetExecutionInfo().ExecutionContext)
	s.Equal(int(1), len(executionBuilder.GetPendingChildExecutionInfos()))
	var childID int64
	for c := range executionBuilder.GetPendingChildExecutionInfos() {
		childID = c
		break
	}
	s.Equal("child-workflow-id", executionBuilder.GetPendingChildExecutionInfos()[childID].StartedWorkflowID)
	s.Equal(types.ParentClosePolicyTerminate, executionBuilder.GetPendingChildExecutionInfos()[childID].ParentClosePolicy)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedSignalExternalWorkflowFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      "invalid run id",
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.GetWorkflowID(),
		RunID:      we.GetRunID(),
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeSignalExternalWorkflowExecution.Ptr(),
		SignalExternalWorkflowExecutionDecisionAttributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
			Domain: constants.TestDomainID,
			Execution: &types.WorkflowExecution{
				WorkflowID: we.WorkflowID,
				RunID:      we.RunID,
			},
			SignalName: "signal",
			Input:      []byte("test input"),
		},
	}}

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})

	s.EqualError(err, "RunID is not valid UUID.")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedSignalExternalWorkflowFailed_UnKnownDomain() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	executionContext := []byte("context")
	foreignDomain := "unknown domain"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeSignalExternalWorkflowExecution.Ptr(),
		SignalExternalWorkflowExecutionDecisionAttributes: &types.SignalExternalWorkflowExecutionDecisionAttributes{
			Domain: foreignDomain,
			Execution: &types.WorkflowExecution{
				WorkflowID: we.WorkflowID,
				RunID:      we.RunID,
			},
			SignalName: "signal",
			Input:      []byte("test input"),
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockDomainCache.EXPECT().GetDomain(foreignDomain).Return(
		nil, errors.New("get foreign domain error"),
	).Times(1)

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: executionContext,
			Identity:         identity,
		},
	})

	s.NotNil(err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedInvalidToken() {

	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: invalidToken,
			Result:    nil,
			Identity:  identity,
		},
	})

	s.NotNil(err)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfNoExecution() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfNoRunID() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfGetExecutionFailed() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("FAILED")).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfNoAIdProvided() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
	})

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "Neither ActivityID nor ScheduleID is provided")
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfNotFound() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: "aid",
	})
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.Error(err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedUpdateExecutionFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfTaskCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	activityStartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, activityScheduledEvent.ID, activityStartedEvent.ID,
		activityResult, identity)
	test.AddDecisionTaskScheduledEvent(msBuilder)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfTaskNotStarted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedConflictOnUpdate() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity2ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	test.AddActivityTaskStartedEvent(msBuilder, activity2ScheduledEvent.ID, identity)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: ms1}

	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, &persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activity1Result,
			Identity:  identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(10))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(10), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskCompletedMaxAttemptsExceeded() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	for i := 0; i < workflow.ConditionalRetryCount; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, &persistence.ConditionFailedError{}).Once()
	}

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.Equal(workflow.ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(9), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(8))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskCompletedByIdSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"

	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: activityID,
	})

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	decisionScheduledEvent := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, decisionScheduledEvent.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, decisionScheduledEvent.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: we.RunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(context.Background(), &types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(9), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(8))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskFailedInvalidToken() {

	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: invalidToken,
			Identity:  identity,
		},
	})

	s.NotNil(err)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfNoExecution() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil,
		&types.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfNoRunID() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil,
		&types.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfGetExecutionFailed() {

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil,
		errors.New("FAILED")).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskFailededIfNoAIdProvided() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
	})
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "Neither ActivityID nor ScheduleID is provided")
}

func (s *engineSuite) TestRespondActivityTaskFailededIfNotFound() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: "aid",
	})
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.Error(err)
}

func (s *engineSuite) TestRespondActivityTaskFailedUpdateExecutionFailed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskFailedIfTaskCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	failReason := "fail reason"
	details := []byte("fail details")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	activityStartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	test.AddActivityTaskFailedEvent(msBuilder, activityScheduledEvent.ID, activityStartedEvent.ID,
		failReason, details, identity)
	test.AddDecisionTaskScheduledEvent(msBuilder)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   details,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfTaskNotStarted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedConflictOnUpdate() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	failReason := "fail reason"
	details := []byte("fail details.")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")
	activity2Result := []byte("activity2_result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 25, identity, nil)
	di1 := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := test.AddDecisionTaskStartedEvent(msBuilder, di1.ScheduleID, tl, identity)
	decisionCompletedEvent1 := test.AddDecisionTaskCompletedEvent(msBuilder, di1.ScheduleID,
		decisionStartedEvent1.ID, nil, identity)
	activity1ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 1, 5)
	activity2ScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent1.ID, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activity1ScheduledEvent.ID, identity)
	activity2StartedEvent := test.AddActivityTaskStartedEvent(msBuilder, activity2ScheduledEvent.ID, identity)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: ms1}

	test.AddActivityTaskCompletedEvent(msBuilder, activity2ScheduledEvent.ID,
		activity2StartedEvent.ID, activity2Result, identity)
	test.AddDecisionTaskScheduledEvent(msBuilder)

	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, &persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   details,
			Identity:  identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(12), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(10))
	s.True(ok)
	s.Equal(int32(25), di.DecisionTimeout)
	s.Equal(int64(10), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskFailedMaxAttemptsExceeded() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	for i := 0; i < workflow.ConditionalRetryCount; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, &persistence.ConditionFailedError{}).Once()
	}

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.Equal(workflow.ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	failReason := "failed"
	failDetails := []byte("fail details.")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   failDetails,
			Identity:  identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(9), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(8))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskFailedByIDSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"

	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	failReason := "failed"
	failDetails := []byte("fail details.")
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: activityID,
	})

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	decisionScheduledEvent := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, decisionScheduledEvent.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, decisionScheduledEvent.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: we.RunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(context.Background(), &types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID: constants.TestDomainID,
		FailedRequest: &types.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   failDetails,
			Identity:  identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(9), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(8))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRecordActivityTaskHeartBeatSuccess_NoTimer() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 0)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	// No HeartBeat timer running.
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	detais := []byte("details")

	_, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   detais,
		},
	})
	s.Nil(err)
}

func (s *engineSuite) TestRecordActivityTaskHeartBeatSuccess_TimerRunning() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	// HeartBeat timer running.
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	detais := []byte("details")

	_, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   detais,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRecordActivityTaskHeartBeatByIDSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: activityID,
	})

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 0)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)

	// No HeartBeat timer running.
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	detais := []byte("details")

	_, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   detais,
		},
	})
	s.Nil(err)
}

func (s *engineSuite) TestRespondActivityTaskCanceled_Scheduled() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCanceled_Started() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	_, _, err := msBuilder.AddActivityTaskCancelRequestedEvent(decisionCompletedEvent.ID, activityID, identity)
	s.Nil(err)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(10), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(9))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(9), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskCanceledByID_Started() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: activityID,
	})

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	decisionScheduledEvent := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, decisionScheduledEvent.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, decisionScheduledEvent.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	_, _, err := msBuilder.AddActivityTaskCancelRequestedEvent(decisionCompletedEvent.ID, activityID, identity)
	s.Nil(err)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: we.RunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(10), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	s.True(executionBuilder.HasPendingDecision())
	di, ok := executionBuilder.GetDecisionInfo(int64(9))
	s.True(ok)
	s.Equal(int32(100), di.DecisionTimeout)
	s.Equal(int64(9), di.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskCanceledIfNoRunID() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCanceledIfNoAIdProvided() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
	})
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.EqualError(err, "Neither ActivityID nor ScheduleID is provided")
}

func (s *engineSuite) TestRespondActivityTaskCanceledIfNotFound() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		ScheduleID: commonconstants.EmptyEventID,
		ActivityID: "aid",
	})
	identity := "testIdentity"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  identity,
		},
	})
	s.Error(err)
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_NotScheduled() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	activityID := "activity1_id"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Scheduled() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(12), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
	di2, ok := executionBuilder.GetDecisionInfo(executionBuilder.GetExecutionInfo().NextEventID - 1)
	s.True(ok)
	s.Equal(executionBuilder.GetExecutionInfo().NextEventID-1, di2.ScheduleID)
	s.Equal(int64(0), di2.Attempt)
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Started() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 0)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Completed() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	workflowResult := []byte("workflow result")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 0)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	decisions := []*types.Decision{
		{
			DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
			RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
				ActivityID: activityID,
			},
		},
		{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: workflowResult,
			},
		},
	}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateCompleted, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_NoHeartBeat() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 0)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())

	// Try recording activity heartbeat
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	activityTaskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: 5,
	})

	hbResponse, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	s.NotNil(hbResponse)
	s.True(hbResponse.CancelRequested)

	// Try cancelling the request.
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)

	executionBuilder = s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(13), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Success() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())

	// Try recording activity heartbeat
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	activityTaskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: 5,
	})

	hbResponse, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	s.NotNil(hbResponse)
	s.True(hbResponse.CancelRequested)

	// Try cancelling the request.
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)

	executionBuilder = s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(13), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_SuccessWithQueries() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	// load mutable state such that it already exists in memory when respond decision task is called
	// this enables us to set query registry on it
	ctx, release, err := s.mockHistoryEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(constants.TestDomainID, we)
	s.NoError(err)
	loadedMS, err := ctx.LoadWorkflowExecution(context.Background())
	s.NoError(err)
	qr := query.NewRegistry()
	id1, _ := qr.BufferQuery(&types.WorkflowQuery{})
	id2, _ := qr.BufferQuery(&types.WorkflowQuery{})
	id3, _ := qr.BufferQuery(&types.WorkflowQuery{})
	loadedMS.SetQueryRegistry(qr)
	release(nil)
	result1 := &types.WorkflowQueryResult{
		ResultType: types.QueryResultTypeAnswered.Ptr(),
		Answer:     []byte{1, 2, 3},
	}
	result2 := &types.WorkflowQueryResult{
		ResultType:   types.QueryResultTypeFailed.Ptr(),
		ErrorMessage: "error reason",
	}
	queryResults := map[string]*types.WorkflowQueryResult{
		id1: result1,
		id2: result2,
	}
	_, err = s.mockHistoryEngine.RespondDecisionTaskCompleted(s.constructCallContext(cc.GoWorkerConsistentQueryVersion), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
			QueryResults:     queryResults,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
	s.Len(qr.GetCompletedIDs(), 2)
	completed1, err := qr.GetTerminationState(id1)
	s.NoError(err)
	s.Equal(result1, completed1.QueryResult)
	s.Equal(query.TerminationTypeCompleted, completed1.TerminationType)
	completed2, err := qr.GetTerminationState(id2)
	s.NoError(err)
	s.Equal(result2, completed2.QueryResult)
	s.Equal(query.TerminationTypeCompleted, completed2.TerminationType)
	s.Len(qr.GetBufferedIDs(), 0)
	s.Len(qr.GetFailedIDs(), 0)
	s.Len(qr.GetUnblockedIDs(), 1)
	unblocked1, err := qr.GetTerminationState(id3)
	s.NoError(err)
	s.Nil(unblocked1.QueryResult)
	s.Equal(query.TerminationTypeUnblocked, unblocked1.TerminationType)

	// Try recording activity heartbeat
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	activityTaskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      we.GetRunID(),
		ScheduleID: 5,
	})

	hbResponse, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(context.Background(), &types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: constants.TestDomainID,
		HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	s.NotNil(hbResponse)
	s.True(hbResponse.CancelRequested)

	// Try cancelling the request.
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(context.Background(), &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: constants.TestDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: activityTaskToken,
			Identity:  identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)

	executionBuilder = s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(13), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_SuccessWithConsistentQueriesUnsupported() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	activityScheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 1)
	test.AddActivityTaskStartedEvent(msBuilder, activityScheduledEvent.ID, identity)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &types.RequestCancelActivityTaskDecisionAttributes{
			ActivityID: activityID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	// load mutable state such that it already exists in memory when respond decision task is called
	// this enables us to set query registry on it
	ctx, release, err := s.mockHistoryEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(constants.TestDomainID, we)
	s.NoError(err)
	loadedMS, err := ctx.LoadWorkflowExecution(context.Background())
	s.NoError(err)
	qr := query.NewRegistry()
	qr.BufferQuery(&types.WorkflowQuery{})
	qr.BufferQuery(&types.WorkflowQuery{})
	qr.BufferQuery(&types.WorkflowQuery{})
	loadedMS.SetQueryRegistry(qr)
	release(nil)
	_, err = s.mockHistoryEngine.RespondDecisionTaskCompleted(s.constructCallContext("0.0.0"), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(11), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(8), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
	s.Len(qr.GetBufferedIDs(), 0)
	s.Len(qr.GetCompletedIDs(), 0)
	s.Len(qr.GetUnblockedIDs(), 0)
	s.Len(qr.GetFailedIDs(), 3)
}

func (s *engineSuite) constructCallContext(featureVersion string) context.Context {
	ctx := context.Background()
	ctx, call := encoding.NewInboundCall(ctx)
	err := call.ReadFromRequest(&transport.Request{
		Headers: transport.NewHeaders().With(common.ClientImplHeaderName, cc.GoSDK).With(common.FeatureVersionHeaderName, featureVersion),
	})
	s.NoError(err)
	return ctx
}

func (s *engineSuite) TestStarTimer_DuplicateTimerID() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)

	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeStartTimer.Ptr(),
		StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
			TimerID:                   timerID,
			StartToFireTimeoutSeconds: common.Int64Ptr(1),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)

	// Try to add the same timer ID again.
	di2 := test.AddDecisionTaskScheduledEvent(executionBuilder)
	test.AddDecisionTaskStartedEvent(executionBuilder, di2.ScheduleID, tl, identity)
	taskToken2, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: di2.ScheduleID,
	})

	ms2 := execution.CreatePersistenceMutableState(s.T(), executionBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	decisionFailedEvent := false
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Run(func(arguments mock.Arguments) {
		req := arguments.Get(1).(*persistence.AppendHistoryNodesRequest)
		decTaskIndex := len(req.Events) - 1
		if decTaskIndex >= 0 && *req.Events[decTaskIndex].EventType == types.EventTypeDecisionTaskFailed {
			decisionFailedEvent = true
		}
	}).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken2,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	s.True(decisionFailedEvent)
	executionBuilder = s.getBuilder(constants.TestDomainID, we)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.True(executionBuilder.HasPendingDecision())
	di3, ok := executionBuilder.GetDecisionInfo(executionBuilder.GetExecutionInfo().NextEventID)
	s.True(ok, "DI.ScheduleID: %v, ScheduleID: %v, StartedID: %v", di2.ScheduleID,
		executionBuilder.GetExecutionInfo().DecisionScheduleID, executionBuilder.GetExecutionInfo().DecisionStartedID)
	s.Equal(executionBuilder.GetExecutionInfo().NextEventID, di3.ScheduleID)
	s.Equal(int64(1), di3.Attempt)
}

func (s *engineSuite) TestUserTimer_RespondDecisionTaskCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	// Verify cancel timer with a start event.
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddTimerStartedEvent(msBuilder, decisionCompletedEvent.ID, timerID, 10)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCancelTimer.Ptr(),
		CancelTimerDecisionAttributes: &types.CancelTimerDecisionAttributes{
			TimerID: timerID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(10), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestCancelTimer_RespondDecisionTaskCompleted_NoStartTimer() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	// Verify cancel timer with a start event.
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCancelTimer.Ptr(),
		CancelTimerDecisionAttributes: &types.CancelTimerDecisionAttributes{
			TimerID: timerID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err := s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engineSuite) TestCancelTimer_RespondDecisionTaskCompleted_TimerFired() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: we.WorkflowID,
		RunID:      we.RunID,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	// Verify cancel timer with a start event.
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 100, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, di.ScheduleID,
		decisionStartedEvent.ID, nil, identity)
	test.AddTimerStartedEvent(msBuilder, decisionCompletedEvent.ID, timerID, 10)
	di2 := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di2.ScheduleID, tl, identity)
	test.AddTimerFiredEvent(msBuilder, timerID)
	_, _, err := msBuilder.CloseTransactionAsMutation(time.Now(), execution.TransactionPolicyActive)
	s.Nil(err)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.True(len(gwmsResponse.State.BufferedEvents) > 0)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeCancelTimer.Ptr(),
		CancelTimerDecisionAttributes: &types.CancelTimerDecisionAttributes{
			TimerID: timerID,
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(input *persistence.UpdateWorkflowExecutionRequest) bool {
		// need to check whether the buffered events are cleared
		s.True(input.UpdateWorkflowMutation.ClearBufferedEvents)
		return true
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.mockHistoryEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: constants.TestDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(constants.TestDomainID, we)
	s.Equal(int64(10), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(7), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
	s.False(executionBuilder.HasBufferedEvents())
}

func (s *engineSuite) TestSignalWorkflowExecution() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	signalRequest := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            constants.TestDomainID,
			WorkflowExecution: &we,
			Identity:          identity,
			SignalName:        signalName,
			Input:             input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.DomainID = constants.TestDomainID
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.SignalWorkflowExecution(context.Background(), signalRequest)
	s.Nil(err)
}

// Test signal decision by adding request ID
func (s *engineSuite) TestSignalWorkflowExecution_DuplicateRequest_WorkflowOpen() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name 2"
	input := []byte("test input 2")
	requestID := uuid.New()
	signalRequest := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            constants.TestDomainID,
			WorkflowExecution: &we,
			Identity:          identity,
			SignalName:        signalName,
			Input:             input,
			RequestID:         requestID,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	// assume duplicate request id
	ms.SignalRequestedIDs = make(map[string]struct{})
	ms.SignalRequestedIDs[requestID] = struct{}{}
	ms.ExecutionInfo.DomainID = constants.TestDomainID
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.SignalWorkflowExecution(context.Background(), signalRequest)
	s.Nil(err)
}

func (s *engineSuite) TestSignalWorkflowExecution_DuplicateRequest_WorkflowCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name 2"
	input := []byte("test input 2")
	requestID := uuid.New()
	signalRequest := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            constants.TestDomainID,
			WorkflowExecution: &we,
			Identity:          identity,
			SignalName:        signalName,
			Input:             input,
			RequestID:         requestID,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	// assume duplicate request id
	ms.SignalRequestedIDs = make(map[string]struct{})
	ms.SignalRequestedIDs[requestID] = struct{}{}
	ms.ExecutionInfo.DomainID = constants.TestDomainID
	ms.ExecutionInfo.State = persistence.WorkflowStateCompleted
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.SignalWorkflowExecution(context.Background(), signalRequest)
	s.Nil(err)
}

func (s *engineSuite) TestSignalWorkflowExecution_WorkflowCompleted() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := &types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	signalRequest := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            constants.TestDomainID,
			WorkflowExecution: we,
			Identity:          identity,
			SignalName:        signalName,
			Input:             input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, *we, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = persistence.WorkflowStateCompleted
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.SignalWorkflowExecution(context.Background(), signalRequest)
	s.EqualError(err, "workflow execution already completed")
}

func (s *engineSuite) TestSignalWorkflowExecution_DelayStart_NoDecisionScheduled() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	we := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	signalRequest := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            constants.TestDomainID,
			WorkflowExecution: &we,
			Identity:          identity,
			SignalName:        signalName,
			Input:             input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	// Only add start event, no decision task scheduled — simulates DelayStart waiting
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.DomainID = constants.TestDomainID
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	var updateReq *persistence.UpdateWorkflowExecutionRequest

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			req, _ := args.Get(1).(*persistence.UpdateWorkflowExecutionRequest)
			updateReq = req
		}).
		Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).
		Once()

	err := s.mockHistoryEngine.SignalWorkflowExecution(context.Background(), signalRequest)
	s.Nil(err)
	s.NotNil(updateReq)
	s.NotNil(updateReq.UpdateWorkflowMutation)
	s.NotNil(updateReq.UpdateWorkflowMutation.ExecutionInfo)
	s.Equal(commonconstants.EmptyEventID, updateReq.UpdateWorkflowMutation.ExecutionInfo.DecisionScheduleID)
}

func (s *engineSuite) TestRemoveSignalMutableState() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockHistoryEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tasklist := "testTaskList"
	identity := "testIdentity"
	requestID := uuid.New()
	removeRequest := &types.RemoveSignalMutableStateRequest{
		DomainUUID:        constants.TestDomainID,
		WorkflowExecution: &workflowExecution,
		RequestID:         requestID,
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		constants.TestRunID,
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, workflowExecution, "wType", tasklist, []byte("input"), 100, 200, identity, nil)
	test.AddDecisionTaskScheduledEvent(msBuilder)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.DomainID = constants.TestDomainID
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	err := s.mockHistoryEngine.RemoveSignalMutableState(context.Background(), removeRequest)
	s.Nil(err)
}

func (s *engineSuite) TestReapplyEvents_ReturnSuccess() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-reapply",
		RunID:      constants.TestRunID,
	}
	history := []*types.HistoryEvent{
		{
			ID:        1,
			EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
			Version:   1,
		},
	}
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}
	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockEventsReapplier.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	err := s.mockHistoryEngine.ReapplyEvents(
		context.Background(),
		constants.TestDomainID,
		workflowExecution.WorkflowID,
		workflowExecution.RunID,
		history,
	)
	s.NoError(err)
}

func (s *engineSuite) TestReapplyEvents_IgnoreSameVersionEvents() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-reapply-same-version",
		RunID:      constants.TestRunID,
	}
	history := []*types.HistoryEvent{
		{
			ID:        1,
			EventType: types.EventTypeTimerStarted.Ptr(),
			Version:   commonconstants.EmptyVersion,
		},
	}
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}
	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockEventsReapplier.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := s.mockHistoryEngine.ReapplyEvents(
		context.Background(),
		constants.TestDomainID,
		workflowExecution.WorkflowID,
		workflowExecution.RunID,
		history,
	)
	s.NoError(err)
}

func (s *engineSuite) TestReapplyEvents_ResetWorkflow() {
	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestLocalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestLocalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).AnyTimes()

	workflowExecution := types.WorkflowExecution{
		WorkflowID: "test-reapply-reset-workflow",
		RunID:      constants.TestRunID,
	}
	history := []*types.HistoryEvent{
		{
			ID:        1,
			EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
			Version:   100,
		},
	}
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.mockHistoryEngine.shard,
		testlogger.New(s.Suite.T()),
		workflowExecution.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = persistence.WorkflowStateCompleted
	ms.ExecutionInfo.LastProcessedEvent = 1
	token, err := msBuilder.GetCurrentBranchToken()
	s.NoError(err)
	item := persistence.NewVersionHistoryItem(1, 1)
	versionHistory := persistence.NewVersionHistory(token, []*persistence.VersionHistoryItem{item})
	ms.VersionHistories = persistence.NewVersionHistories(versionHistory)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &persistence.GetCurrentExecutionResponse{RunID: constants.TestRunID}
	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockEventsReapplier.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	s.mockWorkflowResetter.EXPECT().ResetWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(),
	).Return(nil).Times(1)
	err = s.mockHistoryEngine.ReapplyEvents(
		context.Background(),
		constants.TestDomainID,
		workflowExecution.WorkflowID,
		workflowExecution.RunID,
		history,
	)
	s.NoError(err)
}

func (s *engineSuite) getBuilder(testDomainID string, we types.WorkflowExecution) execution.MutableState {
	context, release, err := s.mockHistoryEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(testDomainID, we)
	if err != nil {
		return nil
	}
	defer release(nil)

	return context.GetWorkflowExecution()
}

func (s *engineSuite) getActivityScheduledEvent(
	msBuilder execution.MutableState,
	scheduleID int64,
) *types.HistoryEvent {
	event, _ := msBuilder.GetActivityScheduledEvent(context.Background(), scheduleID)
	return event
}

func (s *engineSuite) printHistory(builder execution.MutableState) string {
	return thrift.FromHistory(builder.GetHistoryBuilder().GetHistory()).String()
}

func TestRecordChildExecutionCompleted(t *testing.T) {
	testCases := []struct {
		name      string
		request   *types.RecordChildExecutionCompletedRequest
		mockSetup func(*execution.MockMutableState)
		wantErr   bool
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "workflow not running",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 1,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(false)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, workflow.ErrNotExists, err)
			},
		},
		{
			name: "stale mutable state - not containing initiated event",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 10,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(true)
				ms.EXPECT().GetChildExecutionInfo(int64(10)).Return(nil, false)
				ms.EXPECT().GetNextEventID().Return(int64(10)).Times(2)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, workflow.ErrStaleState, err)
			},
		},
		{
			name: "pending child execution not found",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 10,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(true)
				ms.EXPECT().GetChildExecutionInfo(int64(10)).Return(nil, false)
				ms.EXPECT().GetNextEventID().Return(int64(11)).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.EntityNotExistsError{Message: "Pending child execution not found."}, err)
			},
		},
		{
			name: "stale mutable state - not containing started event",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 10,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(true)
				ms.EXPECT().GetChildExecutionInfo(int64(10)).Return(&persistence.ChildExecutionInfo{StartedID: commonconstants.EmptyEventID}, true)
				ms.EXPECT().GetNextEventID().Return(int64(11)).Times(1)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, workflow.ErrStaleState, err)
			},
		},
		{
			name: "pending child execution workflowID mismatch",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 10,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
				StartedID: 11,
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(true)
				ms.EXPECT().GetChildExecutionInfo(int64(10)).Return(&persistence.ChildExecutionInfo{StartedID: 11, StartedWorkflowID: "wid0"}, true)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.EntityNotExistsError{Message: "Pending child execution workflowID mismatch."}, err)
			},
		},
		{
			name: "success - child workflow completed",
			request: &types.RecordChildExecutionCompletedRequest{
				DomainUUID:  "58f7998e-9c00-4827-bbd3-6a891b3ca0ca",
				InitiatedID: 10,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "4353ddce-ca34-4c78-9785-dc0b83af4bbc",
				},
				CompletedExecution: &types.WorkflowExecution{
					WorkflowID: "wid1",
					RunID:      "312b2440-2859-4e50-a59f-d92a300a072d",
				},
				CompletionEvent: &types.HistoryEvent{
					EventType: types.EventTypeWorkflowExecutionCompleted.Ptr(),
					WorkflowExecutionCompletedEventAttributes: &types.WorkflowExecutionCompletedEventAttributes{
						Result: []byte("success"),
					},
				},
				StartedID: 11,
			},
			mockSetup: func(ms *execution.MockMutableState) {
				ms.EXPECT().IsWorkflowExecutionRunning().Return(true)
				ms.EXPECT().GetChildExecutionInfo(int64(10)).Return(&persistence.ChildExecutionInfo{StartedID: 11, StartedWorkflowID: "wid1"}, true)
				ms.EXPECT().AddChildWorkflowExecutionCompletedEvent(int64(10), gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewTestContext(t, ctrl, &persistence.ShardInfo{}, config.NewForTest())
			mockDomainCache := mockShard.Resource.DomainCache
			mockActiveClusterManager := mockShard.Resource.ActiveClusterMgr
			mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.ActiveClusterInfo{ActiveClusterName: mockShard.GetClusterMetadata().GetCurrentClusterName(), FailoverVersion: 0}, nil).Times(1)
			testDomainEntry := cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: constants.TestDomainID}, &persistence.DomainConfig{}, "",
			)
			mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(testDomainEntry, nil).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(constants.TestDomainID, nil).AnyTimes()
			ms := execution.NewMockMutableState(ctrl)
			tc.mockSetup(ms)

			historyEngine := &historyEngineImpl{
				shard:           mockShard,
				clusterMetadata: mockShard.GetClusterMetadata(),
				timeSource:      mockShard.GetTimeSource(),
				metricsClient:   metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}),
				logger:          mockShard.GetLogger(),
				updateWithActionFn: func(_ context.Context, _ log.Logger, _ execution.Cache, _ string, _ types.WorkflowExecution, _ bool, _ time.Time, actionFn func(wfContext execution.Context, mutableState execution.MutableState) error) error {
					return actionFn(nil, ms)
				},
				activeClusterManager: mockActiveClusterManager,
			}

			err := historyEngine.RecordChildExecutionCompleted(context.Background(), tc.request)
			if tc.wantErr {
				tc.assertErr(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_createShardNameFromShardID(t *testing.T) {
	shardID := 1
	shardName := createShardNameFromShardID(shardID)
	assert.Equal(t, fmt.Sprintf("history-engine-%d", shardID), shardName)
}
