// Copyright (c) 2020 Uber Technologies, Inc.
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

package task

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	hclient "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
	"github.com/uber/cadence/service/history/workflowcache"
	warchiver "github.com/uber/cadence/service/worker/archiver"
	"github.com/uber/cadence/service/worker/parentclosepolicy"
)

type (
	transferActiveTaskExecutorSuite struct {
		suite.Suite
		*require.Assertions

		controller         *gomock.Controller
		mockShard          *shard.TestContext
		mockEngine         *engine.MockEngine
		mockDomainCache    *cache.MockDomainCache
		mockWFCache        *workflowcache.MockWFCache
		mockHistoryClient  *hclient.MockClient
		mockMatchingClient *matching.MockClient

		mockVisibilityMgr           *mocks.VisibilityManager
		mockExecutionMgr            *mocks.ExecutionManager
		mockHistoryV2Mgr            *mocks.HistoryV2Manager
		mockArchivalClient          *warchiver.ClientMock
		mockArchivalMetadata        *archiver.MockArchivalMetadata
		mockArchiverProvider        *provider.MockArchiverProvider
		mockParentClosePolicyClient *parentclosepolicy.ClientMock

		logger                     log.Logger
		domainID                   string
		domainName                 string
		domainEntry                *cache.DomainCacheEntry
		version                    int64
		timeSource                 clock.MockedTimeSource
		transferActiveTaskExecutor *transferActiveTaskExecutor
	}
)

func TestTransferActiveTaskExecutorSuite(t *testing.T) {
	s := new(transferActiveTaskExecutorSuite)
	suite.Run(t, s)
}

func (s *transferActiveTaskExecutorSuite) SetupSuite() {

}

func (s *transferActiveTaskExecutorSuite) TearDownSuite() {

}

func (s *transferActiveTaskExecutorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.domainID = constants.TestDomainID
	s.domainName = constants.TestDomainName
	s.domainEntry = constants.TestGlobalDomainEntry
	s.version = s.domainEntry.GetFailoverVersion()
	s.timeSource = clock.NewMockedTimeSource()

	s.controller = gomock.NewController(s.T())

	config := config.NewForTest()
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          0,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config,
	)
	s.mockShard.SetEventsCache(events.NewCache(
		s.mockShard.GetShardID(),
		s.mockShard.GetHistoryManager(),
		s.mockShard.GetConfig(),
		s.mockShard.GetLogger(),
		s.mockShard.GetMetricsClient(),
		s.mockShard.GetDomainCache(),
	))
	s.mockShard.Resource.TimeSource = s.timeSource

	s.mockEngine = engine.NewMockEngine(s.controller)
	s.mockEngine.EXPECT().NotifyNewHistoryEvent(gomock.Any()).AnyTimes()
	s.mockEngine.EXPECT().NotifyNewTransferTasks(gomock.Any()).AnyTimes()
	s.mockEngine.EXPECT().NotifyNewTimerTasks(gomock.Any()).AnyTimes()
	s.mockEngine.EXPECT().NotifyNewReplicationTasks(gomock.Any()).AnyTimes()
	s.mockShard.SetEngine(s.mockEngine)

	s.mockParentClosePolicyClient = &parentclosepolicy.ClientMock{}
	s.mockArchivalClient = &warchiver.ClientMock{}
	s.mockMatchingClient = s.mockShard.Resource.MatchingClient
	s.mockHistoryClient = s.mockShard.Resource.HistoryClient
	s.mockExecutionMgr = s.mockShard.Resource.ExecutionMgr
	s.mockHistoryV2Mgr = s.mockShard.Resource.HistoryMgr
	s.mockVisibilityMgr = s.mockShard.Resource.VisibilityMgr
	s.mockArchivalMetadata = s.mockShard.Resource.ArchivalMetadata
	s.mockArchiverProvider = s.mockShard.Resource.ArchiverProvider
	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockWFCache = workflowcache.NewMockWFCache(s.controller)

	s.mockDomainCache.EXPECT().GetDomain(constants.TestRateLimitedDomainName).Return(constants.TestRateLimitedDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(s.domainName).Return(s.domainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestRateLimitedDomainID).Return(constants.TestRateLimitedDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(s.domainID).Return(s.domainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return(s.domainID, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(s.domainID).Return(s.domainName, nil).AnyTimes()

	s.mockDomainCache.EXPECT().GetDomainByID(constants.TestRateLimitedDomainID).Return(constants.TestRateLimitedDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(constants.TestRateLimitedDomainID).Return(constants.TestRateLimitedDomainName, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomain(constants.TestRateLimitedDomainName).Return(constants.TestRateLimitedDomainEntry, nil).AnyTimes()

	s.logger = s.mockShard.GetLogger()
	s.transferActiveTaskExecutor = NewTransferActiveTaskExecutor(
		s.mockShard,
		s.mockArchivalClient,
		execution.NewCache(s.mockShard),
		nil,
		s.logger,
		config,
		s.mockWFCache,
	).(*transferActiveTaskExecutor)
	s.transferActiveTaskExecutor.parentClosePolicyClient = s.mockParentClosePolicyClient
}

func (s *transferActiveTaskExecutorSuite) TearDownTest() {
	s.transferActiveTaskExecutor.Stop()
	s.controller.Finish()
	s.mockShard.Finish(s.T())
	s.mockArchivalClient.AssertExpectations(s.T())
	s.mockParentClosePolicyClient.AssertExpectations(s.T())
}

func (s *transferActiveTaskExecutorSuite) TestProcessActivityTask_Success() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	event, ai := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity-1",
		"some random activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte{}, 1, 1, 1, 1,
	)
	mutableState.FlushBufferedEvents()

	transferTask := s.newTransferTaskFromInfo(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID: constants.TestDomainID,
		TaskList:       mutableState.GetExecutionInfo().TaskList,
		ScheduleID:     event.ID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockMatchingClient.EXPECT().AddActivityTask(gomock.Any(), createAddActivityTaskRequest(transferTask, ai, mutableState.GetExecutionInfo().PartitionConfig)).Return(&types.AddActivityTaskResponse{}, nil).Times(1)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessActivityTask_EphemeralTaskListKind() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	event, ai, _, _, _, _ := mutableState.AddActivityTaskScheduledEvent(nil, decisionCompletionID, &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    "activity-1",
		ActivityType:                  &types.ActivityType{Name: "some random activity type"},
		TaskList:                      &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList, Kind: types.TaskListKindEphemeral.Ptr()},
		Input:                         []byte{},
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(1),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(1),
		HeartbeatTimeoutSeconds:       common.Int32Ptr(1),
	}, false,
	)
	mutableState.FlushBufferedEvents()

	transferTask := s.newTransferTaskFromInfo(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID: constants.TestDomainID,
		TaskList:       "ignored",
		ScheduleID:     event.ID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockMatchingClient.EXPECT().AddActivityTask(gomock.Any(), createAddActivityTaskRequest(transferTask, ai, mutableState.GetExecutionInfo().PartitionConfig)).Return(&types.AddActivityTaskResponse{}, nil).Times(1)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessActivityTask_Ratelimits() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, constants.TestDomainID)
	s.NoError(err)

	event, ai := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity-1",
		"some random activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte{}, 1, 1, 1, 1,
	)
	mutableState.FlushBufferedEvents()

	transferTaskInRatelimitedDomain := s.newTransferTaskFromInfo(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   constants.TestRateLimitedDomainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID: constants.TestRateLimitedDomainID,
		TaskList:       mutableState.GetExecutionInfo().TaskList,
		ScheduleID:     event.ID,
	})

	transferTask := s.newTransferTaskFromInfo(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID: constants.TestDomainID,
		TaskList:       mutableState.GetExecutionInfo().TaskList,
		ScheduleID:     event.ID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	// expected calls to matching if task processing is allowed
	s.mockMatchingClient.EXPECT().AddActivityTask(gomock.Any(), createAddActivityTaskRequest(transferTaskInRatelimitedDomain, ai, mutableState.GetExecutionInfo().PartitionConfig)).Return(&types.AddActivityTaskResponse{}, nil).Times(1)
	s.mockMatchingClient.EXPECT().AddActivityTask(gomock.Any(), createAddActivityTaskRequest(transferTask, ai, mutableState.GetExecutionInfo().PartitionConfig)).Return(&types.AddActivityTaskResponse{}, nil).Times(1)

	// RPS still below allowed value so the task can be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestRateLimitedDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTaskInRatelimitedDomain)
	s.Nil(err)

	// RPS more than allowed limit so the task cannot be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestRateLimitedDomainID, constants.TestWorkflowID).Return(false).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTaskInRatelimitedDomain)
	s.Error(err)
	s.Equal("workflow is being rate limited for making too many requests", err.Error())

	// RPS still below allowed value so the task can be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)

	// RPS more than allowed limit so the task cannot be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(false).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Error(err)
	s.Equal("workflow is being rate limited for making too many requests", err.Error())
}

func (s *transferActiveTaskExecutorSuite) TestProcessActivityTask_Duplication() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	event, ai := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity-1",
		"some random activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte{}, 1, 1, 1, 1,
	)

	event = test.AddActivityTaskStartedEvent(mutableState, event.ID, "")
	ai.StartedID = event.ID
	event = test.AddActivityTaskCompletedEvent(mutableState, ai.ScheduleID, ai.StartedID, nil, "")
	mutableState.FlushBufferedEvents()

	transferTask := s.newTransferTaskFromInfo(&persistence.ActivityTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID: constants.TestDomainID,
		TaskList:       mutableState.GetExecutionInfo().TaskList,
		ScheduleID:     event.ID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_FirstDecision() {

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_NonFirstDecision() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// make another round of decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_Ratelimits() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// make another round of decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	rateLimitedTransferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   constants.TestRateLimitedDomainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	// expected calls to matching if task processing is allowed
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(rateLimitedTransferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	// RPS still below allowed value so the task can be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestRateLimitedDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(rateLimitedTransferTask)
	s.Nil(err)

	// RPS more than allowed limit so the task cannot be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestRateLimitedDomainID, constants.TestWorkflowID).Return(false).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(rateLimitedTransferTask)
	s.Error(err)
	s.Equal("workflow is being rate limited for making too many requests", err.Error())

	// RPS still below allowed value so the task can be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)

	// RPS more than allowed limit so the task cannot be executed
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(false).Times(1)
	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Error(err)
	s.Equal("workflow is being rate limited for making too many requests", err.Error())
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_Sticky_NonFirstDecision() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// set the sticky tasklist attr
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.StickyTaskList = "some random sticky task list"
	executionInfo.StickyScheduleToStartTimeout = int32(233)

	// make another round of decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   executionInfo.StickyTaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_DecisionNotSticky_MutableStateSticky() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// set the sticky tasklist attr
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.StickyTaskList = "some random sticky task list"
	executionInfo.StickyScheduleToStartTimeout = int32(233)

	// make another round of decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_Duplication() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(4096),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: mutableState.GetPreviousStartedEventID() - 1,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, mutableState.GetNextEventID()-1, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_StickyWorkerUnavailable() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// set the sticky tasklist attr
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.StickyTaskList = "some random sticky task list"
	executionInfo.StickyScheduleToStartTimeout = int32(233)

	// make another round of decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   executionInfo.StickyTaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)

	addDecisionTaskRequest := createAddDecisionTaskRequest(transferTask, mutableState)

	// Create a deep copy of the expected modified request
	modifiedRequest := *addDecisionTaskRequest
	modifiedRequest.TaskList = &types.TaskList{
		Name: mutableState.GetExecutionInfo().TaskList,
	}

	gomock.InOrder(
		s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), addDecisionTaskRequest).Return(nil, &types.StickyWorkerUnavailableError{}).Times(1),
		s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), gomock.Eq(&modifiedRequest)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1),
	)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.NoError(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessDecisionTask_EphemeralTaskListKind() {
	workflowExecution, mutableState, err := test.StartWorkflowWithTaskList(&types.TaskList{Name: "ephemmy", Kind: types.TaskListKindEphemeral.Ptr()}, s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)

	transferTask := s.newTransferTaskFromInfo(&persistence.DecisionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TaskList:   mutableState.GetExecutionInfo().TaskList,
		ScheduleID: di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockWFCache.EXPECT().AllowInternal(constants.TestDomainID, constants.TestWorkflowID).Return(true).Times(1)
	s.mockMatchingClient.EXPECT().AddDecisionTask(gomock.Any(), createAddDecisionTaskRequest(transferTask, mutableState)).Return(&types.AddDecisionTaskResponse{}, nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_HasParent_Success() {
	s.testProcessCloseExecutionWithParent(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
		) {
			s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, mock.Anything).Return(nil).Once()
			s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())
		},
		false,
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_HasParent_Failure() {
	s.testProcessCloseExecutionWithParent(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
		) {
			s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, mock.Anything).Return(nil).Once()
			s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())
		},
		true,
	)
}

func (s *transferActiveTaskExecutorSuite) testProcessCloseExecutionWithParent(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
	),
	failRecordChild bool,
) {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	executionInfo := mutableState.GetExecutionInfo()
	parentInitiatedID := int64(3222)
	parentExecution := types.WorkflowExecution{
		WorkflowID: "some random parent workflow ID",
		RunID:      uuid.New(),
	}
	executionInfo.ParentDomainID = targetDomainID
	executionInfo.ParentWorkflowID = parentExecution.WorkflowID
	executionInfo.ParentRunID = parentExecution.RunID
	executionInfo.InitiatedID = parentInitiatedID

	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)

	transferTask := s.newTransferTaskFromInfo(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	if targetDomainID == constants.TestDomainID {
		var recordChildErr error
		if failRecordChild {
			recordChildErr = &types.DomainNotActiveError{}
		}
		s.mockHistoryClient.EXPECT().RecordChildExecutionCompleted(gomock.Any(), &types.RecordChildExecutionCompletedRequest{
			DomainUUID:         targetDomainID,
			WorkflowExecution:  &parentExecution,
			InitiatedID:        parentInitiatedID,
			CompletedExecution: &workflowExecution,
			CompletionEvent:    event,
		}).Return(recordChildErr).Times(1)
	}

	setupMockFn(mutableState, workflowExecution, parentExecution)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	if failRecordChild {
		s.Equal(&types.DomainNotActiveError{}, err)
	} else {
		s.NoError(err)
	}
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_NoParent() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.Require().NoError(err)
	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)

	transferTask := s.newTransferTaskFromInfo(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, createRecordWorkflowExecutionClosedRequest(
		s.T(),
		s.domainName, startEvent, transferTask, mutableState, 2, s.mockShard.GetTimeSource().Now(), event.Timestamp,
		true),
	).Return(nil).Once()
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dynamicproperties.GetStringPropertyFn("enabled"), true, dynamicproperties.GetBoolPropertyFn(true), "disabled", "random URI"))
	s.mockArchivalClient.On("Archive", mock.Anything, mock.Anything).Return(nil, nil).Once()
	// switch on context header in viz
	s.mockShard.GetConfig().EnableContextHeaderInVisibility = func(domain string) bool {
		return true
	}
	s.mockShard.GetConfig().ValidSearchAttributes = func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
		return map[string]interface{}{
			"Header_context_key": struct{}{},
		}
	}

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_NoParent_HasFewChildren() {
	s.testProcessCloseExecutionNoParentHasFewChildren(
		map[string]string{
			"child_abandon":   s.domainName,
			"child_terminate": s.domainName,
			"child_cancel":    s.domainName,
		},
		func() {
			s.expectCancelRequest(s.domainName)
			s.expectTerminateRequest(s.domainName)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestApplyParentPolicy_SameClusterChild_TargetNotActive() {
	s.testProcessCloseExecutionNoParentHasFewChildrenWithError(
		map[string]string{
			"child_terminate": s.domainName,
			"child_cancel":    s.domainName,
		},
		func() {
			s.mockHistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).
				Return(&types.DomainNotActiveError{}).MaxTimes(1)
			s.mockHistoryClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).
				Return(&types.DomainNotActiveError{}).MaxTimes(1)
		},
		&types.DomainNotActiveError{},
	)
}

func (s *transferActiveTaskExecutorSuite) expectCancelRequest(childDomainName string) {
	childDomainID, err := s.mockDomainCache.GetDomainID(childDomainName)
	s.NoError(err)
	s.mockHistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(
			_ context.Context,
			request *types.HistoryRequestCancelWorkflowExecutionRequest,
			option ...yarpc.CallOption,
		) error {
			s.Equal(childDomainID, request.DomainUUID)
			s.Equal(childDomainName, request.CancelRequest.Domain)
			s.True(request.GetChildWorkflowOnly())
			errors := []error{nil, &types.CancellationAlreadyRequestedError{}, &types.EntityNotExistsError{}}
			return errors[rand.Intn(len(errors))]
		},
	).Times(1)
}

func (s *transferActiveTaskExecutorSuite) expectTerminateRequest(childDomainName string) {
	childDomainID, err := s.mockDomainCache.GetDomainID(childDomainName)
	s.NoError(err)
	s.mockHistoryClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).DoAndReturn(
		func(
			_ context.Context,
			request *types.HistoryTerminateWorkflowExecutionRequest,
			option ...yarpc.CallOption,
		) error {
			s.Equal(childDomainID, request.DomainUUID)
			s.Equal(childDomainName, request.TerminateRequest.Domain)
			errors := []error{nil, &types.EntityNotExistsError{}}
			return errors[rand.Intn(len(errors))]
		},
	).Times(1)
}

func (s *transferActiveTaskExecutorSuite) testProcessCloseExecutionNoParentHasFewChildren(
	childrenDomainNames map[string]string,
	setupMockFn func(),
) {
	s.testProcessCloseExecutionNoParentHasFewChildrenWithError(childrenDomainNames, setupMockFn, nil)
}

func (s *transferActiveTaskExecutorSuite) testProcessCloseExecutionNoParentHasFewChildrenWithError(
	childrenDomainNames map[string]string,
	setupMockFn func(),
	expectedErr error,
) {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	parentClosePolicy1 := types.ParentClosePolicyAbandon
	parentClosePolicy2 := types.ParentClosePolicyTerminate
	parentClosePolicy3 := types.ParentClosePolicyRequestCancel

	_, _, err = mutableState.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletionID, uuid.New(), &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:     childrenDomainNames["child_abandon"],
		WorkflowID: "child workflow1",
		WorkflowType: &types.WorkflowType{
			Name: "child workflow type",
		},
		TaskList:          &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList},
		Input:             []byte("random input"),
		ParentClosePolicy: &parentClosePolicy1,
	})
	s.Nil(err)
	_, _, err = mutableState.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletionID, uuid.New(), &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:     childrenDomainNames["child_terminate"],
		WorkflowID: "child workflow2",
		WorkflowType: &types.WorkflowType{
			Name: "child workflow type",
		},
		TaskList:          &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList},
		Input:             []byte("random input"),
		ParentClosePolicy: &parentClosePolicy2,
	})
	s.Nil(err)
	_, _, err = mutableState.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletionID, uuid.New(), &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:     childrenDomainNames["child_cancel"],
		WorkflowID: "child workflow3",
		WorkflowType: &types.WorkflowType{
			Name: "child workflow type",
		},
		TaskList:          &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList},
		Input:             []byte("random input"),
		ParentClosePolicy: &parentClosePolicy3,
	})
	s.Nil(err)
	s.NoError(mutableState.FlushBufferedEvents())

	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)

	transferTask := s.newTransferTaskFromInfo(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())
	setupMockFn()

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Equal(expectedErr, err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_NoParent_HasManyChildren() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	numChildWorkflows := 500
	for i := 0; i < numChildWorkflows; i++ {
		_, _, err = mutableState.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletionID, uuid.New(), &types.StartChildWorkflowExecutionDecisionAttributes{
			Domain:     s.domainName,
			WorkflowID: "child workflow" + strconv.Itoa(i),
			WorkflowType: &types.WorkflowType{
				Name: "child workflow type",
			},
			TaskList:          &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList},
			Input:             []byte("random input"),
			ParentClosePolicy: types.ParentClosePolicyTerminate.Ptr(),
		})
		s.Nil(err)
	}

	s.NoError(mutableState.FlushBufferedEvents())

	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)

	transferTask := s.newTransferTaskFromInfo(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())
	s.mockParentClosePolicyClient.On("SendParentClosePolicyRequest", mock.Anything, mock.MatchedBy(
		func(request parentclosepolicy.Request) bool {
			if len(request.Executions) != s.mockShard.GetConfig().ParentClosePolicyBatchSize(constants.TestDomainName) {
				return false
			}
			for _, executions := range request.Executions {
				if executions.DomainName != constants.TestDomainName {
					return false
				}
			}
			return true
		},
	)).Return(nil).Times(numChildWorkflows / s.mockShard.GetConfig().ParentClosePolicyBatchSize(constants.TestDomainName))
	s.mockParentClosePolicyClient.On("SendParentClosePolicyRequest", mock.Anything, mock.MatchedBy(
		func(request parentclosepolicy.Request) bool {
			if len(request.Executions) != numChildWorkflows%s.mockShard.GetConfig().ParentClosePolicyBatchSize(constants.TestDomainName) {
				return false
			}
			for _, executions := range request.Executions {
				if executions.DomainName != constants.TestDomainName {
					return false
				}
			}
			return true
		},
	)).Return(nil).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCloseExecution_NoParent_HasManyAbandonedChildren() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	for i := 0; i < 10; i++ {
		_, _, err = mutableState.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletionID, uuid.New(), &types.StartChildWorkflowExecutionDecisionAttributes{
			WorkflowID: "child workflow" + strconv.Itoa(i),
			WorkflowType: &types.WorkflowType{
				Name: "child workflow type",
			},
			TaskList:          &types.TaskList{Name: mutableState.GetExecutionInfo().TaskList},
			Input:             []byte("random input"),
			ParentClosePolicy: types.ParentClosePolicyAbandon.Ptr(),
		})
		s.Nil(err)
	}

	s.NoError(mutableState.FlushBufferedEvents())

	event := test.AddCompleteWorkflowEvent(mutableState, decisionCompletionID, nil)

	transferTask := s.newTransferTaskFromInfo(&persistence.CloseExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockVisibilityMgr.On("RecordWorkflowExecutionClosed", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCancelExecution_Success() {
	s.testProcessCancelExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			requestCancelInfo *persistence.RequestCancelInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			cancelRequest := createTestRequestCancelWorkflowExecutionRequest(constants.TestDomainName, transferTask.GetInfo().(*persistence.CancelExecutionTask), requestCancelInfo.CancelRequestID)
			s.mockHistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), cancelRequest).Return(nil).Times(1)
			s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCancelExecution_Failure() {
	s.testProcessCancelExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			requestCancelInfo *persistence.RequestCancelInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			cancelRequest := createTestRequestCancelWorkflowExecutionRequest(constants.TestDomainName, transferTask.GetInfo().(*persistence.CancelExecutionTask), requestCancelInfo.CancelRequestID)
			s.mockHistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), cancelRequest).Return(&types.EntityNotExistsError{}).Times(1)
			s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCancelExecution_Duplication() {
	s.testProcessCancelExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			requestCancelInfo *persistence.RequestCancelInfo,
		) {
			event = test.AddCancelRequestedEvent(mutableState, event.ID, constants.TestDomainID, targetExecution.GetWorkflowID(), targetExecution.GetRunID())
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) testProcessCancelExecution(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		cancelInitEvent *types.HistoryEvent,
		transferTask Task,
		requestCancelInfo *persistence.RequestCancelInfo,
	),
) {
	s.testProcessCancelExecutionWithError(targetDomainID, setupMockFn, nil)
}

func (s *transferActiveTaskExecutorSuite) testProcessCancelExecutionWithError(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		cancelInitEvent *types.HistoryEvent,
		transferTask Task,
		requestCancelInfo *persistence.RequestCancelInfo,
	),
	expectedErr error,
) {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	targetExecution := types.WorkflowExecution{
		WorkflowID: "some random target workflow ID",
		RunID:      uuid.New(),
	}

	event, rci := test.AddRequestCancelInitiatedEvent(
		mutableState,
		decisionCompletionID,
		uuid.New(),
		constants.TestDomainName,
		targetExecution.GetWorkflowID(),
		targetExecution.GetRunID(),
	)

	transferTask := s.newTransferTaskFromInfo(&persistence.CancelExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID:   targetDomainID,
		TargetWorkflowID: targetExecution.GetWorkflowID(),
		TargetRunID:      targetExecution.GetRunID(),
		InitiatedID:      event.ID,
	})

	setupMockFn(mutableState, workflowExecution, targetExecution, event, transferTask, rci)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Equal(expectedErr, err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessCancelExecution_WorkflowCancellingItself() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	event, rci := test.AddRequestCancelInitiatedEvent(
		mutableState,
		decisionCompletionID,
		uuid.New(),
		constants.TestDomainName,
		workflowExecution.GetWorkflowID(),
		workflowExecution.GetRunID(),
	)

	transferTask := s.newTransferTaskFromInfo(&persistence.CancelExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID:   s.domainID,
		TargetWorkflowID: workflowExecution.GetWorkflowID(),
		TargetRunID:      workflowExecution.GetRunID(),
		InitiatedID:      event.ID,
	})

	setupMockFn := func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		event *types.HistoryEvent,
		transferTask Task,
		requestCancelInfo *persistence.RequestCancelInfo,
	) {
		persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
		s.NoError(err)
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
	}
	setupMockFn(mutableState, workflowExecution, workflowExecution, event, transferTask, rci)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.NoError(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessSignalExecution_Success() {
	s.testProcessSignalExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			signalInfo *persistence.SignalInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			signalRequest := createTestSignalWorkflowExecutionRequest(constants.TestDomainName, transferTask.GetInfo().(*persistence.SignalExecutionTask), signalInfo)
			s.mockHistoryClient.EXPECT().SignalWorkflowExecution(gomock.Any(), signalRequest).Return(nil).Times(1)
			s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

			taskInfo := transferTask.GetInfo().(*persistence.SignalExecutionTask)
			s.mockHistoryClient.EXPECT().RemoveSignalMutableState(gomock.Any(), &types.RemoveSignalMutableStateRequest{
				DomainUUID: taskInfo.TargetDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: taskInfo.TargetWorkflowID,
					RunID:      taskInfo.TargetRunID,
				},
				RequestID: signalInfo.SignalRequestID,
			}).Return(nil).Times(1)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessSignalExecution_Failure() {
	for name, c := range map[string]struct {
		Err          error
		ExpectedLogs []string
	}{
		"GenericErr": {
			Err:          assert.AnError,
			ExpectedLogs: []string{"Failed to signal external workflow execution"},
		},
		"NotExistsErr": {
			Err:          &types.EntityNotExistsError{},
			ExpectedLogs: nil,
		},
		"WorkflowExecutionAlreadyCompleted": {
			Err:          &types.WorkflowExecutionAlreadyCompletedError{},
			ExpectedLogs: nil,
		},
	} {
		s.Run(name, func() {
			// Need setup the suite manually, since we are in a subtest
			s.SetupTest()
			s.testProcessSignalExecutionWithErrorAndLogs(
				constants.TestDomainID,
				func(
					mutableState execution.MutableState,
					workflowExecution, targetExecution types.WorkflowExecution,
					event *types.HistoryEvent,
					transferTask Task,
					signalInfo *persistence.SignalInfo,
				) {
					persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
					s.NoError(err)
					s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
					signalRequest := createTestSignalWorkflowExecutionRequest(constants.TestDomainName, transferTask.GetInfo().(*persistence.SignalExecutionTask), signalInfo)
					s.mockHistoryClient.EXPECT().SignalWorkflowExecution(gomock.Any(), signalRequest).Return(c.Err).Times(1)
					s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
					s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
				},
				nil,
				c.ExpectedLogs,
			)
		})
	}

}

func (s *transferActiveTaskExecutorSuite) TestProcessSignalExecution_Duplication() {
	s.testProcessSignalExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, targetExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			signalInfo *persistence.SignalInfo,
		) {
			event = test.AddSignaledEvent(mutableState, event.ID, constants.TestDomainName, targetExecution.GetWorkflowID(), targetExecution.GetRunID(), nil)
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessSignalExecution_WorkflowSignalingItself() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	event, signalInfo := test.AddRequestSignalInitiatedEvent(
		mutableState,
		decisionCompletionID,
		uuid.New(),
		constants.TestDomainName,
		workflowExecution.GetWorkflowID(),
		workflowExecution.GetRunID(),
		"some random signal name",
		[]byte("some random signal input"),
		[]byte("some random signal control"),
	)

	transferTask := s.newTransferTaskFromInfo(&persistence.SignalExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID:   s.domainID,
		TargetWorkflowID: workflowExecution.GetWorkflowID(),
		TargetRunID:      workflowExecution.GetRunID(),
		InitiatedID:      event.ID,
	})

	// Make sure we can observe the logs
	observedZapCore, _ := observer.New(zap.InfoLevel)
	s.transferActiveTaskExecutor.logger = log.NewLogger(zap.New(observedZapCore))

	setupMockFn := func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		event *types.HistoryEvent,
		transferTask Task,
		signalInfo *persistence.SignalInfo,
	) {
		mutableState.FlushBufferedEvents()

		persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
		s.NoError(err)
		s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
	}

	setupMockFn(mutableState, workflowExecution, workflowExecution, event, transferTask, signalInfo)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.NoError(err)
}

func (s *transferActiveTaskExecutorSuite) testProcessSignalExecution(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		signalInitEvent *types.HistoryEvent,
		transferTask Task,
		signalInfo *persistence.SignalInfo,
	),
) {
	s.testProcessSignalExecutionWithErrorAndLogs(constants.TestDomainID, setupMockFn, nil, nil)
}

func (s *transferActiveTaskExecutorSuite) testProcessSignalExecutionWithErrorAndLogs(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		signalInitEvent *types.HistoryEvent,
		transferTask Task,
		signalInfo *persistence.SignalInfo,
	),
	expectedErr error,
	expectedLogs []string,
) {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	targetExecution := types.WorkflowExecution{
		WorkflowID: "some random target workflow ID",
		RunID:      uuid.New(),
	}

	event, signalInfo := test.AddRequestSignalInitiatedEvent(
		mutableState,
		decisionCompletionID,
		uuid.New(),
		constants.TestDomainName,
		targetExecution.GetWorkflowID(),
		targetExecution.GetRunID(),
		"some random signal name",
		[]byte("some random signal input"),
		[]byte("some random signal control"),
	)

	transferTask := s.newTransferTaskFromInfo(&persistence.SignalExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID:   targetDomainID,
		TargetWorkflowID: targetExecution.GetWorkflowID(),
		TargetRunID:      targetExecution.GetRunID(),
		InitiatedID:      event.ID,
	})

	// Make sure we can observe the logs
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	s.transferActiveTaskExecutor.logger = log.NewLogger(zap.New(observedZapCore))

	setupMockFn(mutableState, workflowExecution, targetExecution, event, transferTask, signalInfo)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Equal(expectedErr, err)

	assert.Equal(s.T(), len(expectedLogs), observedLogs.Len(), "expected %v logs, but got %v logs", len(expectedLogs), observedLogs.Len())
	for _, expectedLog := range expectedLogs {
		s.True(observedLogs.FilterMessage(expectedLog).Len() > 0, "expected log missing: %v", expectedLog)
	}
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_Success() {
	s.testProcessStartChildExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			taskInfo := transferTask.GetInfo().(*persistence.StartChildExecutionTask)
			event, err = mutableState.GetChildExecutionInitiatedEvent(context.Background(), taskInfo.InitiatedID)
			s.NoError(err)
			historyReq, err := createTestChildWorkflowExecutionRequest(
				s.domainName,
				constants.TestDomainName,
				taskInfo,
				event.StartChildWorkflowExecutionInitiatedEventAttributes,
				childInfo.CreateRequestID,
				s.mockShard.GetTimeSource().Now(),
				mutableState.GetExecutionInfo().PartitionConfig,
			)
			require.NoError(s.T(), err)
			s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), historyReq).Return(&types.StartWorkflowExecutionResponse{RunID: childExecution.RunID}, nil).Times(1)
			s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
			s.mockHistoryClient.EXPECT().ScheduleDecisionTask(gomock.Any(), &types.ScheduleDecisionTaskRequest{
				DomainUUID: constants.TestDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: childExecution.WorkflowID,
					RunID:      childExecution.RunID,
				},
				IsFirstDecision: true,
			}).Return(nil).Times(1)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_Failure() {
	s.testProcessStartChildExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			taskInfo := transferTask.GetInfo().(*persistence.StartChildExecutionTask)
			event, err = mutableState.GetChildExecutionInitiatedEvent(context.Background(), taskInfo.InitiatedID)
			s.NoError(err)
			historyReq, err := createTestChildWorkflowExecutionRequest(
				s.domainName,
				constants.TestDomainName,
				taskInfo,
				event.StartChildWorkflowExecutionInitiatedEventAttributes,
				childInfo.CreateRequestID,
				s.mockShard.GetTimeSource().Now(),
				mutableState.GetExecutionInfo().PartitionConfig,
			)
			require.NoError(s.T(), err)
			s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), historyReq).Return(nil, &types.WorkflowExecutionAlreadyStartedError{}).Times(1)
			s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
		},
	)
}

// This test was originally written for the Cross-cluster use-case where the target domain is not active.
// However, it remains a valid test for the scenario where there's a race between parent and child in transfer
// tasks.
// ie: When a failover has occurred, and the parent workflow has spawned a child, this has been picked up by a
// host which has not yet updated it's domain-cache to include the new information that the domain has failed over
// and incorrectly thinks that the domain is not active.
// In this case the correct behaviour would be to return the domainNotActive error and ensure that it's retried on the
// host running the child workflow.
func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_TargetNotActive() {
	s.testProcessStartChildExecutionWithError(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			taskInfo := transferTask.GetInfo().(*persistence.StartChildExecutionTask)
			_, err = mutableState.GetChildExecutionInitiatedEvent(context.Background(), taskInfo.InitiatedID)
			s.NoError(err)
			s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.DomainNotActiveError{
				Message: "domain not active error 123",
			}).Times(1)
		},
		&types.DomainNotActiveError{
			Message: "domain not active error 123",
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_Success_Dup() {
	s.testProcessStartChildExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			startEvent := test.AddChildWorkflowExecutionStartedEvent(mutableState, event.ID, constants.TestDomainID, childExecution.WorkflowID, childExecution.RunID, childInfo.WorkflowTypeName)
			childInfo.StartedID = startEvent.ID
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startEvent.ID, startEvent.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			s.mockHistoryClient.EXPECT().ScheduleDecisionTask(gomock.Any(), &types.ScheduleDecisionTaskRequest{
				DomainUUID: constants.TestDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: childExecution.WorkflowID,
					RunID:      childExecution.RunID,
				},
				IsFirstDecision: true,
			}).Return(nil).Times(1)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_Dup_TargetNotActive() {
	s.testProcessStartChildExecutionWithError(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			startEvent := test.AddChildWorkflowExecutionStartedEvent(mutableState, event.ID, constants.TestDomainID, childExecution.WorkflowID, childExecution.RunID, childInfo.WorkflowTypeName)
			childInfo.StartedID = startEvent.ID
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startEvent.ID, startEvent.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			s.mockHistoryClient.EXPECT().ScheduleDecisionTask(gomock.Any(), gomock.Any()).Return(&types.DomainNotActiveError{}).Times(1)
		},
		&types.DomainNotActiveError{},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_Duplication() {
	s.testProcessStartChildExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			startEvent := test.AddChildWorkflowExecutionStartedEvent(mutableState, event.ID, constants.TestDomainID, childExecution.GetWorkflowID(), childExecution.GetRunID(), childInfo.WorkflowTypeName)
			childInfo.StartedID = startEvent.ID
			startEvent = test.AddChildWorkflowExecutionCompletedEvent(mutableState, childInfo.InitiatedID, &childExecution, &types.WorkflowExecutionCompletedEventAttributes{
				Result:                       []byte("some random child workflow execution result"),
				DecisionTaskCompletedEventID: transferTask.GetInfo().(*persistence.StartChildExecutionTask).InitiatedID,
			})
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startEvent.ID, startEvent.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) TestProcessStartChildExecution_StartedAbandonChild_ParentClosed() {
	s.testProcessStartChildExecution(
		constants.TestDomainID,
		func(
			mutableState execution.MutableState,
			workflowExecution, childExecution types.WorkflowExecution,
			event *types.HistoryEvent,
			transferTask Task,
			childInfo *persistence.ChildExecutionInfo,
		) {
			event = test.AddChildWorkflowExecutionStartedEvent(mutableState, event.ID, constants.TestDomainID, childExecution.WorkflowID, childExecution.RunID, childInfo.WorkflowTypeName)
			childInfo.StartedID = event.ID
			di := test.AddDecisionTaskScheduledEvent(mutableState)
			event = test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, "some random identity")
			event = test.AddDecisionTaskCompletedEvent(mutableState, di.ScheduleID, event.ID, nil, "some random identity")
			event = test.AddCompleteWorkflowEvent(mutableState, event.ID, nil)
			mutableState.FlushBufferedEvents()

			persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
			s.NoError(err)
			s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
			s.mockHistoryClient.EXPECT().ScheduleDecisionTask(gomock.Any(), &types.ScheduleDecisionTaskRequest{
				DomainUUID: constants.TestDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: childExecution.WorkflowID,
					RunID:      childExecution.RunID,
				},
				IsFirstDecision: true,
			}).Return(nil).Times(1)
		},
	)
}

func (s *transferActiveTaskExecutorSuite) testProcessStartChildExecution(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		childInitEvent *types.HistoryEvent,
		transferTask Task,
		childInfo *persistence.ChildExecutionInfo,
	),
) {
	s.testProcessStartChildExecutionWithError(targetDomainID, setupMockFn, nil)
}

func (s *transferActiveTaskExecutorSuite) testProcessStartChildExecutionWithError(
	targetDomainID string,
	setupMockFn func(
		mutableState execution.MutableState,
		workflowExecution, targetExecution types.WorkflowExecution,
		childInitEvent *types.HistoryEvent,
		transferTask Task,
		childInfo *persistence.ChildExecutionInfo,
	),
	expectedErr error,
) {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	childExecution := types.WorkflowExecution{
		WorkflowID: "some random child workflow ID",
		RunID:      uuid.New(),
	}

	event, ci := test.AddStartChildWorkflowExecutionInitiatedEvent(
		mutableState,
		decisionCompletionID,
		uuid.New(),
		constants.TestDomainName,
		childExecution.WorkflowID,
		"some random child workflow type",
		"some random child task list",
		nil,
		1,
		1,
		&types.RetryPolicy{
			ExpirationIntervalInSeconds: 100,
			MaximumAttempts:             3,
			InitialIntervalInSeconds:    1,
			MaximumIntervalInSeconds:    2,
			BackoffCoefficient:          1,
		},
	)

	transferTask := s.newTransferTaskFromInfo(&persistence.StartChildExecutionTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
		TargetDomainID:   targetDomainID,
		TargetWorkflowID: childExecution.WorkflowID,
		InitiatedID:      event.ID,
	})

	setupMockFn(mutableState, workflowExecution, childExecution, event, transferTask, ci)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Equal(expectedErr, err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessRecordWorkflowStartedTask() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.CronSchedule = "@every 5s"
	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.NoError(err)
	startEvent.WorkflowExecutionStartedEventAttributes.FirstDecisionTaskBackoffSeconds = common.Int32Ptr(5)

	transferTask := s.newTransferTaskFromInfo(&persistence.RecordWorkflowStartedTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	if s.mockShard.GetConfig().EnableRecordWorkflowExecutionUninitialized(s.domainName) {
		s.mockVisibilityMgr.On(
			"RecordWorkflowExecutionUninitialized",
			mock.Anything,
			createRecordWorkflowExecutionUninitializedRequest(transferTask, mutableState, s.mockShard.GetTimeSource().Now(), 1234),
		).Once().Return(nil)
	}
	s.mockVisibilityMgr.On(
		"RecordWorkflowExecutionStarted",
		mock.Anything,
		createRecordWorkflowExecutionStartedRequest(
			s.T(),
			s.domainName, startEvent, transferTask, mutableState, 2, s.mockShard.GetTimeSource().Now(),
			false),
	).Once().Return(nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessRecordWorkflowStartedTaskWithContextHeader() {
	// switch on context header in viz
	s.mockShard.GetConfig().EnableContextHeaderInVisibility = func(domain string) bool { return true }
	s.mockShard.GetConfig().ValidSearchAttributes = func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
		return map[string]interface{}{
			"Header_context_key": struct{}{},
			"123456":             struct{}{}, // unsanitizable key
		}
	}

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.CronSchedule = "@every 5s"
	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.NoError(err)
	startEvent.WorkflowExecutionStartedEventAttributes.FirstDecisionTaskBackoffSeconds = common.Int32Ptr(5)

	transferTask := s.newTransferTaskFromInfo(&persistence.RecordWorkflowStartedTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	if s.mockShard.GetConfig().EnableRecordWorkflowExecutionUninitialized(s.domainName) {
		s.mockVisibilityMgr.On(
			"RecordWorkflowExecutionUninitialized",
			mock.Anything,
			createRecordWorkflowExecutionUninitializedRequest(transferTask, mutableState, s.mockShard.GetTimeSource().Now(), 1234),
		).Once().Return(nil)
	}
	s.mockVisibilityMgr.On(
		"RecordWorkflowExecutionStarted",
		mock.Anything,
		createRecordWorkflowExecutionStartedRequest(
			s.T(),
			s.domainName, startEvent, transferTask, mutableState, 2, s.mockShard.GetTimeSource().Now(),
			true),
	).Once().Return(nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessUpsertWorkflowSearchAttributes() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	transferTask := s.newTransferTaskFromInfo(&persistence.UpsertWorkflowSearchAttributesTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.NoError(err)
	s.mockVisibilityMgr.On(
		"UpsertWorkflowExecution",
		mock.Anything,
		createUpsertWorkflowSearchAttributesRequest(
			s.T(),
			s.domainName, startEvent, transferTask, mutableState, 2, s.mockShard.GetTimeSource().Now(),
			false),
	).Once().Return(nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessUpsertWorkflowSearchAttributesWithContextHeader() {
	// switch on context header in viz
	s.mockShard.GetConfig().EnableContextHeaderInVisibility = func(domain string) bool { return true }
	s.mockShard.GetConfig().ValidSearchAttributes = func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
		return map[string]interface{}{
			"Header_context_key": struct{}{},
		}
	}

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	transferTask := s.newTransferTaskFromInfo(&persistence.UpsertWorkflowSearchAttributesTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.NoError(err)
	s.mockVisibilityMgr.On(
		"UpsertWorkflowExecution",
		mock.Anything,
		createUpsertWorkflowSearchAttributesRequest(
			s.T(),
			s.domainName, startEvent, transferTask, mutableState, 2, s.mockShard.GetTimeSource().Now(),
			true),
	).Once().Return(nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_ResetPointNil() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	transferTask := s.newTransferTaskFromInfo(&persistence.ResetWorkflowTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_WorkflowNotRunning() {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	transferTask := s.newTransferTaskFromInfo(&persistence.ResetWorkflowTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	mutableState.GetExecutionInfo().State = persistence.WorkflowStateCompleted

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockExecutionMgr.On("GetCurrentExecution", mock.Anything, &persistence.GetCurrentExecutionRequest{DomainID: s.domainID, WorkflowID: workflowExecution.GetWorkflowID(), DomainName: s.domainName}).
		Return(&persistence.GetCurrentExecutionResponse{RunID: "runID"}, nil)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)
	s.Nil(err)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_Success() {
	s.testProcessResetWorkflowWithError(true, nil, nil)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_DifferentRunID() {
	s.testProcessResetWorkflowWithError(false, nil, nil)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_CorruptedResetPoint() {
	s.testProcessResetWorkflowWithError(true, &types.BadRequestError{}, nil)
}

func (s *transferActiveTaskExecutorSuite) TestProcessResetWorkflow_OtherError() {
	s.testProcessResetWorkflowWithError(true, errors.New("some random error"), errors.New("some random error"))
}

func (s *transferActiveTaskExecutorSuite) testProcessResetWorkflowWithError(sameRunID bool, resetError error, returnErr error) {
	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	s.domainEntry.GetConfig().BadBinaries = types.BadBinaries{
		Binaries: map[string]*types.BadBinaryInfo{
			"test-binary-checksum": {
				Reason:          "test-reason",
				Operator:        "test-operator",
				CreatedTimeNano: common.Ptr(time.Now().UnixNano()),
			},
		},
	}

	transferTask := s.newTransferTaskFromInfo(&persistence.ResetWorkflowTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(59),
		},
	})

	firstDecisionCompletedID := int64(2)

	runID := workflowExecution.GetRunID()
	if !sameRunID {
		runID = uuid.New()
	}

	resetPoints := &types.ResetPoints{
		Points: []*types.ResetPointInfo{
			{
				BinaryChecksum:           "test-binary-checksum",
				RunID:                    runID,
				FirstDecisionCompletedID: firstDecisionCompletedID,
				Resettable:               true,
			},
		},
	}

	mutableState.GetExecutionInfo().AutoResetPoints = resetPoints

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	workflowResetter := reset.NewMockWorkflowResetter(s.controller)
	s.transferActiveTaskExecutor.workflowResetter = workflowResetter
	versionHistories := mutableState.GetVersionHistories()
	currentVersionHistory, err := versionHistories.GetCurrentVersionHistory()
	s.NoError(err)

	currentBranchToken := currentVersionHistory.GetBranchToken()
	rebuildLastEventVersion, err := currentVersionHistory.GetEventVersion(firstDecisionCompletedID)
	s.NoError(err)

	workflowResetter.EXPECT().ResetWorkflow(
		gomock.Any(),
		s.domainID,
		workflowExecution.GetWorkflowID(),
		workflowExecution.GetRunID(),
		currentBranchToken,
		firstDecisionCompletedID-1,
		rebuildLastEventVersion,
		mutableState.GetNextEventID(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		"test-reason",
		nil,
		false).Return(resetError).Times(1)

	_, err = s.transferActiveTaskExecutor.Execute(transferTask)

	if returnErr != nil {
		s.Equal(returnErr, err)
	} else {
		s.Nil(err)
	}
}

func (s *transferActiveTaskExecutorSuite) TestCopySearchAttributes() {
	var input map[string][]byte
	s.Nil(copySearchAttributes(input))

	key := "key"
	val := []byte{'1', '2', '3'}
	input = map[string][]byte{
		key: val,
	}
	result := copySearchAttributes(input)
	s.Equal(input, result)
	result[key][0] = '0'
	s.Equal(byte('1'), val[0])
}

func (s *transferActiveTaskExecutorSuite) newTransferTaskFromInfo(
	task persistence.Task,
) Task {
	return NewHistoryTask(s.mockShard, task, QueueTypeActiveTransfer, s.logger, nil, nil, nil, nil, nil)
}

func createAddActivityTaskRequest(
	transferTask Task,
	ai *persistence.ActivityInfo,
	partitionConfig map[string]string,
) *types.AddActivityTaskRequest {

	taskInfo := transferTask.GetInfo().(*persistence.ActivityTask)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	taskList := &types.TaskList{Name: ai.TaskList, Kind: ai.TaskListKind.Ptr()}

	return &types.AddActivityTaskRequest{
		DomainUUID:                    taskInfo.TargetDomainID,
		SourceDomainUUID:              taskInfo.DomainID,
		Execution:                     &workflowExecution,
		TaskList:                      taskList,
		ScheduleID:                    taskInfo.ScheduleID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(ai.ScheduleToStartTimeout),
		PartitionConfig:               partitionConfig,
	}
}

func createAddDecisionTaskRequest(
	transferTask Task,
	mutableState execution.MutableState,
) *types.AddDecisionTaskRequest {

	taskInfo := transferTask.GetInfo().(*persistence.DecisionTask)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	taskList := &types.TaskList{Name: taskInfo.TaskList, Kind: mutableState.GetExecutionInfo().TaskListKind.Ptr()}
	executionInfo := mutableState.GetExecutionInfo()
	timeout := executionInfo.WorkflowTimeout
	if mutableState.GetExecutionInfo().TaskList != taskInfo.TaskList {
		taskListStickyKind := types.TaskListKindSticky
		taskList.Kind = &taskListStickyKind
		timeout = executionInfo.StickyScheduleToStartTimeout
	}

	return &types.AddDecisionTaskRequest{
		DomainUUID:                    taskInfo.DomainID,
		Execution:                     &workflowExecution,
		TaskList:                      taskList,
		ScheduleID:                    taskInfo.ScheduleID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(timeout),
		PartitionConfig:               executionInfo.PartitionConfig,
	}
}

func createRecordWorkflowExecutionStartedRequest(
	t *testing.T,
	domainName string,
	startEvent *types.HistoryEvent,
	transferTask Task,
	mutableState execution.MutableState,
	numClusters int16,
	updateTime time.Time,
	enableContextHeaderInVisibility bool,
) *persistence.RecordWorkflowExecutionStartedRequest {
	taskInfo := transferTask.GetInfo().(*persistence.RecordWorkflowStartedTask)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	executionInfo := mutableState.GetExecutionInfo()
	backoffSeconds := startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds()
	executionTimestamp := int64(0)
	if backoffSeconds != 0 {
		executionTimestamp = startEvent.GetTimestamp() + int64(backoffSeconds)*int64(time.Second)
	}
	var searchAttributes map[string][]byte
	if enableContextHeaderInVisibility {
		contextValueJSONString, err := json.Marshal("contextValue")
		if err != nil {
			t.Fatal(err)
		}
		searchAttributes = map[string][]byte{
			"Header_context_key": contextValueJSONString,
		}
	}
	return &persistence.RecordWorkflowExecutionStartedRequest{
		Domain:             domainName,
		DomainUUID:         taskInfo.DomainID,
		Execution:          workflowExecution,
		WorkflowTypeName:   executionInfo.WorkflowTypeName,
		StartTimestamp:     startEvent.GetTimestamp(),
		ExecutionTimestamp: executionTimestamp,
		WorkflowTimeout:    int64(executionInfo.WorkflowTimeout),
		TaskID:             taskInfo.TaskID,
		TaskList:           executionInfo.TaskList,
		IsCron:             len(executionInfo.CronSchedule) > 0,
		NumClusters:        numClusters,
		UpdateTimestamp:    updateTime.UnixNano(),
		SearchAttributes:   searchAttributes,
	}
}

func createRecordWorkflowExecutionClosedRequest(
	t *testing.T,
	domainName string,
	startEvent *types.HistoryEvent,
	transferTask Task,
	mutableState execution.MutableState,
	numClusters int16,
	updateTime time.Time,
	closeTimestamp *int64,
	enableContextHeaderInVisibility bool,
) *persistence.RecordWorkflowExecutionClosedRequest {
	taskInfo := transferTask.GetInfo().(*persistence.CloseExecutionTask)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	executionInfo := mutableState.GetExecutionInfo()
	backoffSeconds := startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds()
	executionTimestamp := int64(0)
	if backoffSeconds != 0 {
		executionTimestamp = startEvent.GetTimestamp() + int64(backoffSeconds)*int64(time.Second)
	}
	var searchAttributes map[string][]byte
	if enableContextHeaderInVisibility {
		contextValueJSONString, err := json.Marshal("contextValue")
		if err != nil {
			t.Fatal(err)
		}
		searchAttributes = map[string][]byte{
			"Header_context_key": contextValueJSONString,
		}
	}
	return &persistence.RecordWorkflowExecutionClosedRequest{
		Domain:             domainName,
		DomainUUID:         taskInfo.DomainID,
		Execution:          workflowExecution,
		HistoryLength:      mutableState.GetNextEventID() - 1,
		WorkflowTypeName:   executionInfo.WorkflowTypeName,
		StartTimestamp:     startEvent.GetTimestamp(),
		ExecutionTimestamp: executionTimestamp,
		TaskID:             taskInfo.TaskID,
		TaskList:           executionInfo.TaskList,
		IsCron:             len(executionInfo.CronSchedule) > 0,
		NumClusters:        numClusters,
		UpdateTimestamp:    updateTime.UnixNano(),
		CloseTimestamp:     *closeTimestamp,
		RetentionSeconds:   int64(mutableState.GetDomainEntry().GetRetentionDays(taskInfo.GetWorkflowID()) * 24 * 3600),
		SearchAttributes:   searchAttributes,
	}
}

func createTestRequestCancelWorkflowExecutionRequest(
	targetDomainName string,
	taskInfo *persistence.CancelExecutionTask,
	requestID string,
) *types.HistoryRequestCancelWorkflowExecutionRequest {

	sourceExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	targetExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.TargetWorkflowID,
		RunID:      taskInfo.TargetRunID,
	}

	return &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: taskInfo.TargetDomainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			Domain:            targetDomainName,
			WorkflowExecution: &targetExecution,
			Identity:          execution.IdentityHistoryService,
			// Use the same request ID to dedupe RequestCancelWorkflowExecution calls
			RequestID: requestID,
		},
		ExternalInitiatedEventID:  common.Int64Ptr(taskInfo.InitiatedID),
		ExternalWorkflowExecution: &sourceExecution,
		ChildWorkflowOnly:         taskInfo.TargetChildWorkflowOnly,
	}
}

func createTestSignalWorkflowExecutionRequest(
	targetDomainName string,
	taskInfo *persistence.SignalExecutionTask,
	si *persistence.SignalInfo,
) *types.HistorySignalWorkflowExecutionRequest {

	sourceExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	targetExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.TargetWorkflowID,
		RunID:      taskInfo.TargetRunID,
	}

	return &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: taskInfo.TargetDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain:            targetDomainName,
			WorkflowExecution: &targetExecution,
			Identity:          execution.IdentityHistoryService,
			SignalName:        si.SignalName,
			Input:             si.Input,
			RequestID:         si.SignalRequestID,
			Control:           si.Control,
		},
		ExternalWorkflowExecution: &sourceExecution,
		ChildWorkflowOnly:         taskInfo.TargetChildWorkflowOnly,
	}
}

func createTestChildWorkflowExecutionRequest(
	domainName string,
	childDomainName string,
	taskInfo *persistence.StartChildExecutionTask,
	attributes *types.StartChildWorkflowExecutionInitiatedEventAttributes,
	requestID string,
	now time.Time,
	partitionConfig map[string]string,
) (*types.HistoryStartWorkflowExecutionRequest, error) {

	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	frontendStartReq := &types.StartWorkflowExecutionRequest{
		Domain:                              childDomainName,
		WorkflowID:                          attributes.WorkflowID,
		WorkflowType:                        attributes.WorkflowType,
		TaskList:                            attributes.TaskList,
		Input:                               attributes.Input,
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      attributes.TaskStartToCloseTimeoutSeconds,
		// Use the same request ID to dedupe StartWorkflowExecution calls
		RequestID:             requestID,
		WorkflowIDReusePolicy: attributes.WorkflowIDReusePolicy,
		RetryPolicy:           attributes.RetryPolicy,
	}

	parentInfo := &types.ParentExecutionInfo{
		DomainUUID:  taskInfo.DomainID,
		Domain:      domainName,
		Execution:   &workflowExecution,
		InitiatedID: taskInfo.InitiatedID,
	}

	historyStartReq, err := common.CreateHistoryStartWorkflowRequest(
		taskInfo.TargetDomainID, frontendStartReq, now, partitionConfig)
	if err != nil {
		return nil, err
	}

	historyStartReq.ParentExecutionInfo = parentInfo
	return historyStartReq, nil
}

func createUpsertWorkflowSearchAttributesRequest(
	t *testing.T,
	domainName string,
	startEvent *types.HistoryEvent,
	transferTask Task,
	mutableState execution.MutableState,
	numClusters int16,
	updateTime time.Time,
	enableContextHeaderInVisibility bool,
) *persistence.UpsertWorkflowExecutionRequest {

	taskInfo := transferTask.GetInfo().(*persistence.UpsertWorkflowSearchAttributesTask)
	workflowExecution := types.WorkflowExecution{
		WorkflowID: taskInfo.WorkflowID,
		RunID:      taskInfo.RunID,
	}
	executionInfo := mutableState.GetExecutionInfo()
	backoffSeconds := startEvent.WorkflowExecutionStartedEventAttributes.GetFirstDecisionTaskBackoffSeconds()
	executionTimestamp := int64(0)
	if backoffSeconds != 0 {
		executionTimestamp = startEvent.GetTimestamp() + int64(backoffSeconds)*int64(time.Second)
	}
	var searchAttributes map[string][]byte
	if enableContextHeaderInVisibility {
		contextValueJSONString, err := json.Marshal("contextValue")
		if err != nil {
			t.Fatal(err)
		}
		searchAttributes = map[string][]byte{
			"Header_context_key": contextValueJSONString,
		}
	}

	return &persistence.UpsertWorkflowExecutionRequest{
		Domain:             domainName,
		DomainUUID:         taskInfo.DomainID,
		Execution:          workflowExecution,
		WorkflowTypeName:   executionInfo.WorkflowTypeName,
		StartTimestamp:     startEvent.GetTimestamp(),
		ExecutionTimestamp: executionTimestamp,
		WorkflowTimeout:    int64(executionInfo.WorkflowTimeout),
		TaskID:             taskInfo.TaskID,
		TaskList:           executionInfo.TaskList,
		IsCron:             len(executionInfo.CronSchedule) > 0,
		NumClusters:        numClusters,
		UpdateTimestamp:    updateTime.UnixNano(),
		SearchAttributes:   searchAttributes,
	}
}

func createRecordWorkflowExecutionUninitializedRequest(
	transferTask Task,
	mutableState execution.MutableState,
	updateTime time.Time,
	shardID int64,
) *persistence.RecordWorkflowExecutionUninitializedRequest {
	workflowExecution := types.WorkflowExecution{
		WorkflowID: transferTask.GetWorkflowID(),
		RunID:      transferTask.GetRunID(),
	}
	executionInfo := mutableState.GetExecutionInfo()
	return &persistence.RecordWorkflowExecutionUninitializedRequest{
		DomainUUID:       transferTask.GetDomainID(),
		Execution:        workflowExecution,
		WorkflowTypeName: executionInfo.WorkflowTypeName,
		UpdateTimestamp:  updateTime.UnixNano(),
		ShardID:          shardID,
	}
}
