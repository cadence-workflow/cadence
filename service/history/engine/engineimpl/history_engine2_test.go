// Copyright (c) 2017 Uber Technologies, Inc.
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
	"reflect"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/decision"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
	"github.com/uber/cadence/service/history/workflow"
)

type (
	engine2Suite struct {
		suite.Suite
		*require.Assertions

		controller         *gomock.Controller
		mockShard          *shard.TestContext
		mockTxProcessor    *queue.MockProcessor
		mockTimerProcessor *queue.MockProcessor
		mockEventsCache    *events.MockCache
		mockDomainCache    *cache.MockDomainCache

		historyEngine        *historyEngineImpl
		mockExecutionMgr     *mocks.ExecutionManager
		mockHistoryV2Mgr     *mocks.HistoryV2Manager
		mockShardManager     *mocks.ShardManager
		mockActiveClusterMgr *activecluster.MockManager

		config *config.Config
		logger log.Logger
	}
)

func TestEngine2Suite(t *testing.T) {
	s := new(engine2Suite)
	suite.Run(t, s)
}

func (s *engine2Suite) SetupSuite() {
	s.config = config.NewForTest()
}

func (s *engine2Suite) TearDownSuite() {
}

func (s *engine2Suite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())

	s.mockTxProcessor = queue.NewMockProcessor(s.controller)
	s.mockTimerProcessor = queue.NewMockProcessor(s.controller)
	s.mockTxProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).AnyTimes()
	s.mockTimerProcessor.EXPECT().NotifyNewTask(gomock.Any(), gomock.Any()).AnyTimes()

	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&p.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		s.config,
	)

	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockExecutionMgr = s.mockShard.Resource.ExecutionMgr
	s.mockHistoryV2Mgr = s.mockShard.Resource.HistoryMgr
	s.mockShardManager = s.mockShard.Resource.ShardMgr
	s.mockEventsCache = s.mockShard.MockEventsCache
	s.mockActiveClusterMgr = s.mockShard.Resource.ActiveClusterMgr
	testDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&p.DomainInfo{ID: constants.TestDomainID}, &p.DomainConfig{}, "",
	)
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(testDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(constants.TestDomainID, nil).AnyTimes()
	s.mockEventsCache.EXPECT().PutEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	s.logger = s.mockShard.GetLogger()

	executionCache := execution.NewCache(s.mockShard)
	h := &historyEngineImpl{
		currentClusterName:   s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		shard:                s.mockShard,
		clusterMetadata:      s.mockShard.Resource.ClusterMetadata,
		executionManager:     s.mockExecutionMgr,
		historyV2Mgr:         s.mockHistoryV2Mgr,
		executionCache:       executionCache,
		logger:               s.logger,
		throttledLogger:      s.logger,
		metricsClient:        metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}),
		tokenSerializer:      common.NewJSONTaskTokenSerializer(),
		config:               s.config,
		timeSource:           s.mockShard.GetTimeSource(),
		historyEventNotifier: events.NewNotifier(clock.NewRealTimeSource(), metrics.NewClient(tally.NoopScope, metrics.History, metrics.HistogramMigration{}), func(string) int { return 0 }),
		queueProcessors: map[p.HistoryTaskCategory]queue.Processor{
			p.HistoryTaskCategoryTransfer: s.mockTxProcessor,
			p.HistoryTaskCategoryTimer:    s.mockTimerProcessor,
		},
		activeClusterManager: s.mockActiveClusterMgr,
	}
	s.mockShard.SetEngine(h)
	h.decisionHandler = decision.NewHandler(s.mockShard, h.executionCache, h.tokenSerializer)

	s.historyEngine = h
}

func (s *engine2Suite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *engine2Suite) TestRecordDecisionTaskStartedSuccessStickyExpired() {
	domainID := constants.TestDomainID
	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	stickyTl := "stickyTaskList"
	identity := "testIdentity"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	executionInfo := msBuilder.GetExecutionInfo()
	executionInfo.StickyTaskList = stickyTl

	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)

	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	request := types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &we,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: stickyTl,
			},
			Identity: identity,
		},
	}

	expectedResponse := types.RecordDecisionTaskStartedResponse{}
	expectedResponse.WorkflowType = msBuilder.GetWorkflowType()
	executionInfo = msBuilder.GetExecutionInfo()
	if executionInfo.LastProcessedEvent != commonconstants.EmptyEventID {
		expectedResponse.PreviousStartedEventID = common.Int64Ptr(executionInfo.LastProcessedEvent)
	}
	expectedResponse.ScheduledEventID = di.ScheduleID
	expectedResponse.StartedEventID = di.ScheduleID + 1
	expectedResponse.StickyExecutionEnabled = false
	expectedResponse.NextEventID = msBuilder.GetNextEventID() + 1
	expectedResponse.Attempt = di.Attempt
	expectedResponse.WorkflowExecutionTaskList = &types.TaskList{
		Name: executionInfo.TaskList,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	expectedResponse.BranchToken, _ = msBuilder.GetCurrentBranchToken()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &request)
	s.Nil(err)
	s.NotNil(response)
	expectedResponse.StartedTimestamp = response.StartedTimestamp
	expectedResponse.ScheduledTimestamp = common.Int64Ptr(0)
	response.ScheduledTimestamp = common.Int64Ptr(0)
	expectedResponse.Queries = make(map[string]*types.WorkflowQuery)
	s.Equal(&expectedResponse, response)
}

func (s *engine2Suite) TestRecordDecisionTaskStartedSuccessStickyEnabled() {
	domainID := constants.TestDomainID
	we := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}
	tl := "testTaskList"
	stickyTl := "stickyTaskList"
	identity := "testIdentity"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	executionInfo := msBuilder.GetExecutionInfo()
	executionInfo.LastUpdatedTimestamp = time.Now()
	executionInfo.StickyTaskList = stickyTl

	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)

	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	request := types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &we,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: stickyTl,
			},
			Identity: identity,
		},
	}

	expectedResponse := types.RecordDecisionTaskStartedResponse{}
	expectedResponse.WorkflowType = msBuilder.GetWorkflowType()
	executionInfo = msBuilder.GetExecutionInfo()
	if executionInfo.LastProcessedEvent != commonconstants.EmptyEventID {
		expectedResponse.PreviousStartedEventID = common.Int64Ptr(executionInfo.LastProcessedEvent)
	}
	expectedResponse.ScheduledEventID = di.ScheduleID
	expectedResponse.StartedEventID = di.ScheduleID + 1
	expectedResponse.StickyExecutionEnabled = true
	expectedResponse.NextEventID = msBuilder.GetNextEventID() + 1
	expectedResponse.Attempt = di.Attempt
	expectedResponse.WorkflowExecutionTaskList = &types.TaskList{
		Name: executionInfo.TaskList,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	currentBranchTokken, err := msBuilder.GetCurrentBranchToken()
	s.NoError(err)
	expectedResponse.BranchToken = currentBranchTokken

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &request)
	s.Nil(err)
	s.NotNil(response)
	expectedResponse.StartedTimestamp = response.StartedTimestamp
	expectedResponse.ScheduledTimestamp = common.Int64Ptr(0)
	response.ScheduledTimestamp = common.Int64Ptr(0)
	expectedResponse.Queries = make(map[string]*types.WorkflowQuery)
	s.Equal(&expectedResponse, response)
}

func (s *engine2Suite) TestRecordDecisionTaskStartedIfNoExecution() {
	domainID := constants.TestDomainID
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	identity := "testIdentity"
	tl := "testTaskList"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(response)
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engine2Suite) TestRecordDecisionTaskStartedIfGetExecutionFailed() {
	domainID := constants.TestDomainID
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	identity := "testIdentity"
	tl := "testTaskList"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("FAILED")).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(response)
	s.NotNil(err)
	s.EqualError(err, "FAILED")
}

func (s *engine2Suite) TestRecordDecisionTaskStartedIfTaskAlreadyStarted() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(response)
	s.NotNil(err)
	s.IsType(&types.EventAlreadyStartedError{}, err)
	s.logger.Error("RecordDecisionTaskStarted failed with", tag.Error(err))
}

func (s *engine2Suite) TestRecordDecisionTaskStartedIfTaskAlreadyCompleted() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)
	test.AddDecisionTaskCompletedEvent(msBuilder, int64(2), int64(3), nil, identity)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(response)
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
	s.logger.Error("RecordDecisionTaskStarted failed with", tag.Error(err))
}

func (s *engine2Suite) TestRecordDecisionTaskStartedConflictOnUpdate() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(3)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.ConditionFailedError{}).Once()

	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &p.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(err)
	s.NotNil(response)
	s.Equal("wType", response.WorkflowType.Name)
	s.True(response.PreviousStartedEventID == nil)
	s.Equal(int64(3), response.StartedEventID)
}

func (s *engine2Suite) TestRecordDecisionTaskRetrySameRequest() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	tl := "testTaskList"
	identity := "testIdentity"
	requestID := "testRecordDecisionTaskRetrySameRequestID"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(3)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.ConditionFailedError{}).Once()

	startedEventID := test.AddDecisionTaskStartedEventWithRequestID(msBuilder, int64(2), requestID, tl, identity)
	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &p.GetWorkflowExecutionResponse{State: ms2}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         requestID,
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.Nil(err)
	s.NotNil(response)
	s.Equal("wType", response.WorkflowType.Name)
	s.True(response.PreviousStartedEventID == nil)
	s.Equal(startedEventID.ID, response.StartedEventID)
}

func (s *engine2Suite) TestRecordDecisionTaskRetryDifferentRequest() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	tl := "testTaskList"
	identity := "testIdentity"
	requestID := "testRecordDecisionTaskRetrySameRequestID"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(3)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.ConditionFailedError{}).Once()

	// Add event.
	test.AddDecisionTaskStartedEventWithRequestID(msBuilder, int64(2), "some_other_req", tl, identity)
	ms2 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse2 := &p.GetWorkflowExecutionResponse{State: ms2}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse2, nil).Once()

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         requestID,
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.Nil(response)
	s.NotNil(err)
	s.IsType(&types.EventAlreadyStartedError{}, err)
	s.logger.Info("Failed with error", tag.Error(err))
}

func (s *engine2Suite) TestRecordDecisionTaskStartedMaxAttemptsExceeded() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	tl := "testTaskList"
	identity := "testIdentity"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(workflow.ConditionalRetryCount + 1)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	for i := 0; i < workflow.ConditionalRetryCount; i++ {
		ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
		gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Times(
		workflow.ConditionalRetryCount)
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil,
		&p.ConditionFailedError{}).Times(workflow.ConditionalRetryCount)

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.NotNil(err)
	s.Nil(response)
	s.Equal(workflow.ErrMaxAttemptsExceeded, err)
}

func (s *engine2Suite) TestRecordDecisionTaskSuccess() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	tl := "testTaskList"
	identity := "testIdentity"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(3)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	// load mutable state such that it already exists in memory when respond decision task is called
	// this enables us to set query registry on it
	ctx, release, err := s.historyEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(constants.TestDomainID, workflowExecution)
	s.NoError(err)
	loadedMS, err := ctx.LoadWorkflowExecution(context.Background())
	s.NoError(err)
	qr := query.NewRegistry()
	id1, _ := qr.BufferQuery(&types.WorkflowQuery{})
	id2, _ := qr.BufferQuery(&types.WorkflowQuery{})
	id3, _ := qr.BufferQuery(&types.WorkflowQuery{})
	loadedMS.SetQueryRegistry(qr)
	release(nil)

	response, err := s.historyEngine.RecordDecisionTaskStarted(context.Background(), &types.RecordDecisionTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        2,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForDecisionTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.Nil(err)
	s.NotNil(response)
	s.Equal("wType", response.WorkflowType.Name)
	s.True(response.PreviousStartedEventID == nil)
	s.Equal(int64(3), response.StartedEventID)
	expectedQueryMap := map[string]*types.WorkflowQuery{
		id1: {},
		id2: {},
		id3: {},
	}
	s.Equal(expectedQueryMap, response.Queries)
}

func (s *engine2Suite) TestRecordActivityTaskStartedIfNoExecution() {
	domainID := constants.TestDomainID
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil)

	identity := "testIdentity"
	tl := "testTaskList"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{}).Once()

	response, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	if err != nil {
		s.logger.Error("Unexpected Error", tag.Error(err))
	}
	s.Nil(response)
	s.NotNil(err)
	s.IsType(&types.EntityNotExistsError{}, err)
}

func (s *engine2Suite) TestRecordActivityTaskStartedSuccess() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, int64(2), int64(3), nil, identity)
	scheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	s.mockEventsCache.EXPECT().GetEvent(
		gomock.Any(), gomock.Any(), domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID(),
		decisionCompletedEvent.ID, scheduledEvent.ID, gomock.Any(),
	).Return(scheduledEvent, nil)
	response, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(err)
	s.NotNil(response)
	s.Equal(scheduledEvent, response.ScheduledEvent)
}

func (s *engine2Suite) TestRecordActivityTaskStartedResurrected() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID}
	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	timeSource := clock.NewMockedTimeSource()
	s.historyEngine.timeSource = timeSource

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, int64(2), int64(3), nil, identity)
	scheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, "activity1_id", "activity_type1", tl, []byte("input1"), 100, 10, 1, 5)

	// Use mutable state snapshot before start/completion of the activity (to indicate resurrected state)
	msSnapshot := execution.CreatePersistenceMutableState(s.T(), msBuilder)

	startedEvent := test.AddActivityTaskStartedEvent(msBuilder, scheduledEvent.ID, identity)
	test.AddActivityTaskCompletedEvent(msBuilder, scheduledEvent.ID, startedEvent.ID, nil, identity)

	// Use full history after the activity start/completion
	historySnapshot := msBuilder.GetHistoryBuilder().GetHistory()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&p.GetWorkflowExecutionResponse{State: msSnapshot}, nil).Once()
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(&p.ReadHistoryBranchResponse{HistoryEvents: historySnapshot.Events}, nil).Once()

	// Expect that mutable state will be updated to delete resurrected activity
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.UpdateWorkflowExecutionRequest) bool {
		return len(request.UpdateWorkflowMutation.DeleteActivityInfos) == 1
	})).Return(&p.UpdateWorkflowExecutionResponse{}, nil).Once()

	// Ensure enough time passed
	timeSource.Advance(time.Hour)

	_, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        scheduledEvent.ID,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest:       &types.PollForActivityTaskRequest{TaskList: &types.TaskList{Name: tl}, Identity: identity},
	})

	s.Equal(err, workflow.ErrActivityTaskNotFound)
}

func (s *engine2Suite) TestRecordActivityTaskStartedStaleState() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(workflow.ConditionalRetryCount + 1)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Times(workflow.ConditionalRetryCount)

	response, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.Error(err)
	s.Nil(response)
	s.Equal(workflow.ErrMaxAttemptsExceeded, err)
}

func (s *engine2Suite) TestRecordActivityTaskStartedActivityNotPending() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()

	response, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        3,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})

	s.Error(err)
	s.Nil(response)
	s.Equal(workflow.ErrActivityTaskNotFound, err)
}

func (s *engine2Suite) TestRecordActivityTaskStartedActivityAlreadyStarted() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(6)

	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, true)
	decisionCompletedEvent := test.AddDecisionTaskCompletedEvent(msBuilder, int64(2), int64(3), nil, identity)
	scheduledEvent, _ := test.AddActivityTaskScheduledEvent(msBuilder, decisionCompletedEvent.ID, activityID,
		activityType, tl, activityInput, 100, 10, 1, 5)

	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	s.mockEventsCache.EXPECT().GetEvent(
		gomock.Any(), gomock.Any(), domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID(),
		decisionCompletedEvent.ID, scheduledEvent.ID, gomock.Any(),
	).Return(scheduledEvent, nil).Times(1)

	// start activity
	response, err := s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(err)
	s.NotNil(response)
	s.Equal(scheduledEvent, response.ScheduledEvent)

	// another request made with the same scheduleID and same requestID
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	s.mockEventsCache.EXPECT().GetEvent(
		gomock.Any(), gomock.Any(), domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID(),
		decisionCompletedEvent.ID, scheduledEvent.ID, gomock.Any(),
	).Return(scheduledEvent, nil).Times(1)

	response, err = s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "reqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Nil(err)
	s.NotNil(response)
	s.Equal(scheduledEvent, response.ScheduledEvent)

	// another request made with the same scheduleID and different requestID
	s.mockEventsCache.EXPECT().GetEvent(
		gomock.Any(), gomock.Any(), domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID(),
		decisionCompletedEvent.ID, scheduledEvent.ID, gomock.Any(),
	).Return(scheduledEvent, nil).Times(1)

	response, err = s.historyEngine.RecordActivityTaskStarted(context.Background(), &types.RecordActivityTaskStartedRequest{
		DomainUUID:        domainID,
		WorkflowExecution: &workflowExecution,
		ScheduleID:        5,
		TaskID:            100,
		RequestID:         "otherReqId",
		PollRequest: &types.PollForActivityTaskRequest{
			TaskList: &types.TaskList{
				Name: tl,
			},
			Identity: identity,
		},
	})
	s.Error(err)
	s.Nil(response)
	s.Equal(&types.EventAlreadyStartedError{Message: "Activity task already started."}, err)
}

func (s *engine2Suite) TestRequestCancelWorkflowExecutionSuccess() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	err := s.historyEngine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			Identity: "identity",
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, workflowExecution)
	s.Equal(int64(4), executionBuilder.GetNextEventID())
}

func (s *engine2Suite) TestRequestCancelWorkflowExecutionDuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.DuplicateRequestError{RunID: "test-run-id"}).Once()

	err := s.historyEngine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			Identity: "identity",
		},
	})
	s.Nil(err)
}

func (s *engine2Suite) TestRequestCancelWorkflowExecutionAlreadyCancelled_Success() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"
	cancelRequestID := "cancelrequestid"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	msBuilder.GetExecutionInfo().State = p.WorkflowStateCompleted
	msBuilder.GetExecutionInfo().CloseStatus = p.WorkflowCloseStatusCanceled
	msBuilder.GetExecutionInfo().CancelRequested = true
	msBuilder.GetExecutionInfo().CancelRequestID = cancelRequestID
	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()

	err := s.historyEngine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			Identity:  "identity",
			RequestID: cancelRequestID,
		},
	})
	s.Nil(err)
}

func (s *engine2Suite) TestRequestCancelWorkflowExecutionAlreadyCancelled_Fail() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"
	cancelRequestID := "cancelrequestid"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	msBuilder.GetExecutionInfo().State = p.WorkflowStateCompleted
	msBuilder.GetExecutionInfo().CancelRequested = true
	msBuilder.GetExecutionInfo().CancelRequestID = cancelRequestID
	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()

	err := s.historyEngine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			Identity:  "identity",
			RequestID: cancelRequestID + "xxx",
		},
	})
	s.NotNil(err)
	s.IsType(&types.WorkflowExecutionAlreadyCompletedError{}, err)
}

func (s *engine2Suite) TestRequestCancelWorkflowExecutionFail() {
	domainID := constants.TestDomainID
	workflowExecution := types.WorkflowExecution{
		WorkflowID: "wId",
		RunID:      constants.TestRunID,
	}

	identity := "testIdentity"
	tl := "testTaskList"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(2)

	msBuilder := s.createExecutionStartedState(workflowExecution, tl, identity, false)
	msBuilder.GetExecutionInfo().State = p.WorkflowStateCompleted
	ms1 := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse1 := &p.GetWorkflowExecutionResponse{State: ms1}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse1, nil).Once()

	err := s.historyEngine.RequestCancelWorkflowExecution(context.Background(), &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: workflowExecution.WorkflowID,
				RunID:      workflowExecution.RunID,
			},
			Identity: "identity",
		},
	})
	s.NotNil(err)
	s.IsType(&types.WorkflowExecutionAlreadyCompletedError{}, err)
}

func (s *engine2Suite) createExecutionStartedState(
	we types.WorkflowExecution,
	tl, identity string,
	startDecision bool,
) execution.MutableState {
	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		s.logger,
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	if startDecision {
		test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)
	}
	_ = msBuilder.SetHistoryTree(we.GetRunID())

	return msBuilder
}

//nolint:unused
//lint:ignore U1000 for printing within tests
func (s *engine2Suite) printHistory(builder execution.MutableState) string {
	return thrift.FromHistory(builder.GetHistoryBuilder().GetHistory()).String()
}

func (s *engine2Suite) TestRespondDecisionTaskCompletedRecordMarkerDecision() {
	domainID := constants.TestDomainID
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
	markerDetails := []byte("marker details")
	markerName := "marker name"

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.historyEngine.clusterMetadata.GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(3)

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		we.GetRunID(),
		constants.TestLocalDomainEntry,
	)
	test.AddWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity, nil)
	di := test.AddDecisionTaskScheduledEvent(msBuilder)
	test.AddDecisionTaskStartedEvent(msBuilder, di.ScheduleID, tl, identity)

	decisions := []*types.Decision{{
		DecisionType: types.DecisionTypeRecordMarker.Ptr(),
		RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
			MarkerName: markerName,
			Details:    markerDetails,
		},
	}}

	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	_, err := s.historyEngine.RespondDecisionTaskCompleted(context.Background(), &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: domainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: nil,
			Identity:         identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(6), executionBuilder.GetExecutionInfo().NextEventID)
	s.Equal(int64(3), executionBuilder.GetExecutionInfo().LastProcessedEvent)
	s.Equal(p.WorkflowStateRunning, executionBuilder.GetExecutionInfo().State)
	s.False(executionBuilder.HasPendingDecision())
}

func (s *engine2Suite) TestStartWorkflowExecution_BrandNew() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(&p.CreateWorkflowExecutionResponse{}, nil).Once()

	requestID := uuid.New()
	resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	})
	s.Nil(err)
	s.NotNil(resp.RunID)
}

func (s *engine2Suite) TestStartWorkflowExecution_BrandNew_DuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(nil, &p.DuplicateRequestError{RunID: "test-run-id"}).Once()

	requestID := uuid.New()
	resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	})
	s.NoError(err)
	s.Equal("test-run-id", resp.RunID)
}

func (s *engine2Suite) TestStartWorkflowExecution_BrandNew_DuplicateRequestError_TypeMismatch() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeSignal, RunID: "test-run-id"}).Once()

	requestID := uuid.New()
	_, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	})
	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
}

func (s *engine2Suite) TestStartWorkflowExecution_StillRunning_Dedup() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	runID := "runID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	requestID := "requestID"
	lastWriteVersion := commonconstants.EmptyVersion

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   requestID,
		RunID:            runID,
		State:            p.WorkflowStateRunning,
		CloseStatus:      p.WorkflowCloseStatusNone,
		LastWriteVersion: lastWriteVersion,
	}).Once()

	resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           requestID,
		},
	})
	s.Nil(err)
	s.Equal(runID, resp.GetRunID())
}

func (s *engine2Suite) TestStartWorkflowExecution_StillRunning_NonDeDup() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	runID := "runID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	lastWriteVersion := commonconstants.EmptyVersion

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   "oldRequestID",
		RunID:            runID,
		State:            p.WorkflowStateRunning,
		CloseStatus:      p.WorkflowCloseStatusNone,
		LastWriteVersion: lastWriteVersion,
	}).Once()

	resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           "newRequestID",
		},
	})
	if _, ok := err.(*types.WorkflowExecutionAlreadyStartedError); !ok {
		s.Fail("return err is not *types.WorkflowExecutionAlreadyStartedError")
	}
	s.Nil(resp)
}

func (s *engine2Suite) TestStartWorkflowExecution_NotRunning_PrevSuccess_DuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	runID := "runID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	lastWriteVersion := commonconstants.EmptyVersion
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On(
		"CreateWorkflowExecution",
		mock.Anything,
		mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
			return request.Mode == p.CreateWorkflowModeBrandNew &&
				!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
				reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
		}),
	).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   "oldRequestID",
		RunID:            runID,
		State:            p.WorkflowStateCompleted,
		CloseStatus:      p.WorkflowCloseStatusCompleted,
		LastWriteVersion: lastWriteVersion,
	}).Once()

	s.mockExecutionMgr.On(
		"CreateWorkflowExecution",
		mock.Anything,
		mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
			return request.Mode == p.CreateWorkflowModeWorkflowIDReuse &&
				request.PreviousRunID == runID &&
				request.PreviousLastWriteVersion == lastWriteVersion &&
				!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
				reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
		}),
	).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeStart, RunID: "test-run-id"}).Once()

	resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           "newRequestID",
			WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		},
		PartitionConfig: partitionConfig,
	})

	s.Nil(err)
	s.Equal("test-run-id", resp.RunID)
}

func (s *engine2Suite) TestStartWorkflowExecution_NotRunning_PrevSuccess_DuplicateRequestError_TypeMismatch() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	runID := "runID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	lastWriteVersion := commonconstants.EmptyVersion
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On(
		"CreateWorkflowExecution",
		mock.Anything,
		mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
			return request.Mode == p.CreateWorkflowModeBrandNew &&
				!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
				reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
		}),
	).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   "oldRequestID",
		RunID:            runID,
		State:            p.WorkflowStateCompleted,
		CloseStatus:      p.WorkflowCloseStatusCompleted,
		LastWriteVersion: lastWriteVersion,
	}).Once()

	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On(
		"CreateWorkflowExecution",
		mock.Anything,
		mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
			return request.Mode == p.CreateWorkflowModeWorkflowIDReuse &&
				request.PreviousRunID == runID &&
				request.PreviousLastWriteVersion == lastWriteVersion &&
				!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
				reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
		}),
	).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeSignal, RunID: "test-run-id"}).Once()

	_, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			RequestID:                           "newRequestID",
			WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		},
		PartitionConfig: partitionConfig,
	})

	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
}

func (s *engine2Suite) TestStartWorkflowExecution_NotRunning_PrevSuccess() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	runID := "runID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	lastWriteVersion := commonconstants.EmptyVersion
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}

	options := []types.WorkflowIDReusePolicy{
		types.WorkflowIDReusePolicyAllowDuplicateFailedOnly,
		types.WorkflowIDReusePolicyAllowDuplicate,
		types.WorkflowIDReusePolicyRejectDuplicate,
	}

	expectedErrs := []bool{true, false, true}

	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Times(3)
	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Times(2)
	s.mockExecutionMgr.On(
		"CreateWorkflowExecution",
		mock.Anything,
		mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
			return request.Mode == p.CreateWorkflowModeBrandNew &&
				!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
				reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
		}),
	).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   "oldRequestID",
		RunID:            runID,
		State:            p.WorkflowStateCompleted,
		CloseStatus:      p.WorkflowCloseStatusCompleted,
		LastWriteVersion: lastWriteVersion,
	}).Times(len(expectedErrs))

	for index, option := range options {
		s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
		if !expectedErrs[index] {
			s.mockExecutionMgr.On(
				"CreateWorkflowExecution",
				mock.Anything,
				mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
					return request.Mode == p.CreateWorkflowModeWorkflowIDReuse &&
						request.PreviousRunID == runID &&
						request.PreviousLastWriteVersion == lastWriteVersion &&
						!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
						reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
				}),
			).Return(&p.CreateWorkflowExecutionResponse{}, nil).Once()
		}

		resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
			DomainUUID: domainID,
			StartRequest: &types.StartWorkflowExecutionRequest{
				Domain:                              domainID,
				WorkflowID:                          workflowID,
				WorkflowType:                        &types.WorkflowType{Name: workflowType},
				TaskList:                            &types.TaskList{Name: taskList},
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
				Identity:                            identity,
				RequestID:                           "newRequestID",
				WorkflowIDReusePolicy:               &option,
			},
			PartitionConfig: partitionConfig,
		})

		if expectedErrs[index] {
			if _, ok := err.(*types.WorkflowExecutionAlreadyStartedError); !ok {
				s.Fail("return err is not *types.WorkflowExecutionAlreadyStartedError")
			}
			s.Nil(resp)
		} else {
			s.Nil(err)
			s.NotNil(resp)
		}
	}
}

func (s *engine2Suite) TestStartWorkflowExecution_NotRunning_PrevFail() {
	domainID := constants.TestDomainID
	workflowID := "workflowID"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	lastWriteVersion := commonconstants.EmptyVersion
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}

	options := []types.WorkflowIDReusePolicy{
		types.WorkflowIDReusePolicyAllowDuplicateFailedOnly,
		types.WorkflowIDReusePolicyAllowDuplicate,
		types.WorkflowIDReusePolicyRejectDuplicate,
	}

	expectedErrs := []bool{false, false, true}

	closeStates := []int{
		p.WorkflowCloseStatusFailed,
		p.WorkflowCloseStatusCanceled,
		p.WorkflowCloseStatusTerminated,
		p.WorkflowCloseStatusTimedOut,
	}
	runIDs := []string{"1", "2", "3", "4"}

	for i, closeState := range closeStates {

		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Times(3)
		s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Times(1)
		s.mockExecutionMgr.On(
			"CreateWorkflowExecution",
			mock.Anything,
			mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
				return request.Mode == p.CreateWorkflowModeBrandNew &&
					!request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() &&
					reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
			}),
		).Return(nil, &p.WorkflowExecutionAlreadyStartedError{
			Msg:              "random message",
			StartRequestID:   "oldRequestID",
			RunID:            runIDs[i],
			State:            p.WorkflowStateCompleted,
			CloseStatus:      closeState,
			LastWriteVersion: lastWriteVersion,
		}).Times(len(expectedErrs))

		for j, option := range options {

			s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil)
			if !expectedErrs[j] {
				s.mockExecutionMgr.On(
					"CreateWorkflowExecution",
					mock.Anything,
					mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
						return request.Mode == p.CreateWorkflowModeWorkflowIDReuse &&
							request.PreviousRunID == runIDs[i] &&
							request.PreviousLastWriteVersion == lastWriteVersion &&
							reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
					}),
				).Return(&p.CreateWorkflowExecutionResponse{}, nil).Once()
			}

			resp, err := s.historyEngine.StartWorkflowExecution(context.Background(), &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: domainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              domainID,
					WorkflowID:                          workflowID,
					WorkflowType:                        &types.WorkflowType{Name: workflowType},
					TaskList:                            &types.TaskList{Name: taskList},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
					Identity:                            identity,
					RequestID:                           "newRequestID",
					WorkflowIDReusePolicy:               &option,
				},
				PartitionConfig: partitionConfig,
			})

			if expectedErrs[j] {
				if _, ok := err.(*types.WorkflowExecutionAlreadyStartedError); !ok {
					s.Fail("return err is not *types.WorkflowExecutionAlreadyStartedError")
				}
				s.Nil(resp)
			} else {
				s.Nil(err)
				s.NotNil(resp)
			}
		}
	}
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_JustSignal() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:     domainID,
			WorkflowID: workflowID,
			Identity:   identity,
			SignalName: signalName,
			Input:      input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&p.UpdateWorkflowExecutionResponse{
		MutableStateUpdateSessionStats: &p.MutableStateUpdateSessionStats{},
	}, nil).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.Equal(runID, resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_JustSignal_DuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:     domainID,
			WorkflowID: workflowID,
			Identity:   identity,
			SignalName: signalName,
			Input:      input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeSignal, RunID: "test-run-id"}).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.Equal("test-run-id", resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_JustSignal_DuplicateRequestError_TypeMismatch() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:     domainID,
			WorkflowID: workflowID,
			Identity:   identity,
			SignalName: signalName,
			Input:      input,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeStart, RunID: "test-run-id"}).Once()

	_, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_WorkflowNotExist() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := uuid.New()
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	}

	notExistErr := &types.EntityNotExistsError{Message: "Workflow not exist"}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, notExistErr).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(&p.CreateWorkflowExecutionResponse{}, nil).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.NotNil(resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_WorkflowNotExist_DuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := uuid.New()
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	}

	notExistErr := &types.EntityNotExistsError{Message: "Workflow not exist"}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, notExistErr).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(nil, &p.DuplicateRequestError{RunID: "test-run-id"}).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.Equal("test-run-id", resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_WorkflowNotExist_DuplicateRequestError_TypeMismatch() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := uuid.New()
	partitionConfig := map[string]string{
		"zone": "phx",
	}

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
		},
		PartitionConfig: partitionConfig,
	}

	notExistErr := &types.EntityNotExistsError{Message: "Workflow not exist"}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, notExistErr).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(nil, &p.DuplicateRequestError{RequestType: p.WorkflowRequestTypeCancel, RunID: "test-run-id"}).Once()

	_, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Error(err)
	s.IsType(&p.DuplicateRequestError{}, err)
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_CreateTimeout() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := uuid.New()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)

	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
		},
	}

	notExistErr := &types.EntityNotExistsError{Message: "Workflow not exist"}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(nil, notExistErr).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.TimeoutError{}).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything, mock.Anything).Return(nil).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.True(p.IsTimeoutError(err))
	s.NotNil(resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_WorkflowNotRunning() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := uuid.New()
	partitionConfig := map[string]string{
		"zone": "phx",
	}
	policy := types.WorkflowIDReusePolicyAllowDuplicate

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
			WorkflowIDReusePolicy:               &policy,
		},
		PartitionConfig: partitionConfig,
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = p.WorkflowStateCompleted
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.MatchedBy(func(request *p.CreateWorkflowExecutionRequest) bool {
		return !request.NewWorkflowSnapshot.ExecutionInfo.StartTimestamp.IsZero() && reflect.DeepEqual(partitionConfig, request.NewWorkflowSnapshot.ExecutionInfo.PartitionConfig)
	})).Return(&p.CreateWorkflowExecutionResponse{}, nil).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.NotNil(resp.GetRunID())
	s.NotEqual(runID, resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_Start_DuplicateRequests() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := "testRequestID"
	policy := types.WorkflowIDReusePolicyAllowDuplicate

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
			WorkflowIDReusePolicy:               &policy,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = p.WorkflowStateCompleted
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}
	workflowAlreadyStartedErr := &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   requestID, // use same requestID
		RunID:            runID,
		State:            p.WorkflowStateRunning,
		CloseStatus:      p.WorkflowCloseStatusNone,
		LastWriteVersion: commonconstants.EmptyVersion,
	}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, workflowAlreadyStartedErr).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.NotNil(resp.GetRunID())
	s.Equal(runID, resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_Start_DuplicateRequestError() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := "testRequestID"
	policy := types.WorkflowIDReusePolicyAllowDuplicate

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
			WorkflowIDReusePolicy:               &policy,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = p.WorkflowStateCompleted
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &p.DuplicateRequestError{RunID: "test-run-id"}).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(err)
	s.Equal("test-run-id", resp.GetRunID())
}

func (s *engine2Suite) TestSignalWithStartWorkflowExecution_Start_WorkflowAlreadyStarted() {
	domainID := constants.TestDomainID
	workflowID := "wId"
	runID := constants.TestRunID
	workflowType := "workflowType"
	taskList := "testTaskList"
	identity := "testIdentity"
	signalName := "my signal name"
	input := []byte("test input")
	requestID := "testRequestID"
	policy := types.WorkflowIDReusePolicyAllowDuplicate

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: s.mockShard.GetClusterMetadata().GetCurrentClusterName(),
		FailoverVersion:   0,
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, gomock.Any()).Return(testActiveClusterInfo, nil).Times(1)
	sRequest := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			Domain:                              domainID,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: taskList},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
			Identity:                            identity,
			SignalName:                          signalName,
			Input:                               input,
			RequestID:                           requestID,
			WorkflowIDReusePolicy:               &policy,
		},
	}

	msBuilder := execution.NewMutableStateBuilderWithEventV2(
		s.historyEngine.shard,
		testlogger.New(s.Suite.T()),
		runID,
		constants.TestLocalDomainEntry,
	)
	ms := execution.CreatePersistenceMutableState(s.T(), msBuilder)
	ms.ExecutionInfo.State = p.WorkflowStateCompleted
	gwmsResponse := &p.GetWorkflowExecutionResponse{State: ms}
	gceResponse := &p.GetCurrentExecutionResponse{RunID: runID}
	workflowAlreadyStartedErr := &p.WorkflowExecutionAlreadyStartedError{
		Msg:              "random message",
		StartRequestID:   "new request ID",
		RunID:            runID,
		State:            p.WorkflowStateRunning,
		CloseStatus:      p.WorkflowCloseStatusNone,
		LastWriteVersion: commonconstants.EmptyVersion,
	}

	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(gceResponse, nil).Once()
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&p.AppendHistoryNodesResponse{}, nil).Once()
	s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, workflowAlreadyStartedErr).Once()

	resp, err := s.historyEngine.SignalWithStartWorkflowExecution(context.Background(), sRequest)
	s.Nil(resp)
	s.NotNil(err)
}

func (s *engine2Suite) TestTerminateWorkflowExecution_Success() {

}

func (s *engine2Suite) TestNewChildContext() {
	ctx := context.Background()
	childCtx, childCancel := s.historyEngine.newChildContext(ctx)
	defer childCancel()
	_, ok := childCtx.Deadline()
	s.True(ok)

	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	childCtx, childCancel = s.historyEngine.newChildContext(ctx)
	defer childCancel()
	deadline, ok := childCtx.Deadline()
	s.True(ok)
	s.True(time.Until(deadline) < 10*time.Minute)
}

func (s *engine2Suite) getBuilder(domainID string, we types.WorkflowExecution) execution.MutableState {
	context, release, err := s.historyEngine.executionCache.GetOrCreateWorkflowExecutionForBackground(domainID, we)
	if err != nil {
		return nil
	}
	defer release(nil)

	return context.GetWorkflowExecution()
}
