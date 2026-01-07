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
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
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
	"github.com/uber/cadence/service/history/shard"
	test "github.com/uber/cadence/service/history/testing"
)

type (
	timerActiveTaskExecutorSuite struct {
		suite.Suite
		*require.Assertions

		controller         *gomock.Controller
		mockShard          *shard.TestContext
		mockEngine         *engine.MockEngine
		mockDomainCache    *cache.MockDomainCache
		mockMatchingClient *matching.MockClient

		mockExecutionMgr *mocks.ExecutionManager
		mockHistoryV2Mgr *mocks.HistoryV2Manager

		executionCache execution.Cache
		logger         log.Logger
		domainID       string
		domain         string
		domainEntry    *cache.DomainCacheEntry
		version        int64

		timeSource              clock.MockedTimeSource
		timerActiveTaskExecutor *timerActiveTaskExecutor
	}
)

func TestTimerActiveTaskExecutorSuite(t *testing.T) {
	s := new(timerActiveTaskExecutorSuite)
	suite.Run(t, s)
}

func (s *timerActiveTaskExecutorSuite) SetupSuite() {

}

func (s *timerActiveTaskExecutorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.domainID = constants.TestDomainID
	s.domain = constants.TestDomainName
	s.domainEntry = constants.TestGlobalDomainEntry
	s.version = s.domainEntry.GetFailoverVersion()

	s.timeSource = clock.NewMockedTimeSource()

	s.controller = gomock.NewController(s.T())

	config := config.NewForTest()
	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
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

	s.mockDomainCache = s.mockShard.Resource.DomainCache
	s.mockMatchingClient = s.mockShard.Resource.MatchingClient
	s.mockExecutionMgr = s.mockShard.Resource.ExecutionMgr
	s.mockHistoryV2Mgr = s.mockShard.Resource.HistoryMgr
	// ack manager will use the domain information
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(constants.TestDomainName, nil).AnyTimes()

	testActiveClusterInfo := &types.ActiveClusterInfo{
		ActiveClusterName: constants.TestGlobalDomainEntry.GetReplicationConfig().ActiveClusterName,
		FailoverVersion:   constants.TestGlobalDomainEntry.GetFailoverVersion(),
	}
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(testActiveClusterInfo, nil).AnyTimes()

	s.logger = s.mockShard.GetLogger()
	s.executionCache = execution.NewCache(s.mockShard)
	s.timerActiveTaskExecutor = NewTimerActiveTaskExecutor(
		s.mockShard,
		nil,
		s.executionCache,
		s.logger,
		s.mockShard.GetMetricsClient(),
		config,
	).(*timerActiveTaskExecutor)
}

func (s *timerActiveTaskExecutorSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *timerActiveTaskExecutorSuite) TestProcessUserTimerTimeout_Fire() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerID := "timer"
	timerTimeout := 2 * time.Second
	event, _ := test.AddTimerStartedEvent(mutableState, decisionCompletionID, timerID, int64(timerTimeout.Seconds()))

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextUserTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	_, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetUserTimerInfo(timerID)
	s.False(ok)
}

func (s *timerActiveTaskExecutorSuite) TestProcessUserTimerTimeout_Noop() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerID := "timer"
	timerTimeout := 2 * time.Second
	event, _ := test.AddTimerStartedEvent(mutableState, decisionCompletionID, timerID, int64(timerTimeout.Seconds()))

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextUserTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	event = test.AddTimerFiredEvent(mutableState, timerID)
	mutableState.FlushBufferedEvents()

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, event.ID, event.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestProcessUserTimerTimeout_Resurrected() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// schedule two timers
	timerID1 := "timer1"
	timerTimeout1 := 2 * time.Second
	startEvent1, _ := test.AddTimerStartedEvent(mutableState, decisionCompletionID, timerID1, int64(timerTimeout1.Seconds()))
	timerID2 := "timer2"
	timerTimeout2 := 5 * time.Second
	startEvent2, _ := test.AddTimerStartedEvent(mutableState, decisionCompletionID, timerID2, int64(timerTimeout2.Seconds()))

	// fire timer 1
	firedEvent1 := test.AddTimerFiredEvent(mutableState, timerID1)
	mutableState.FlushBufferedEvents()
	// there should be a decision scheduled event after timer1 is fired
	// omitted here to make the test easier to read

	// create timer task for timer2
	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks() // remove existing timer task for timerID1
	modified, err := timerSequence.CreateNextUserTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, firedEvent1.ID, firedEvent1.Version)
	s.NoError(err)
	// add resurrected timer info for timer1
	persistenceMutableState.TimerInfos[timerID1] = &persistence.TimerInfo{
		Version:    startEvent1.Version,
		TimerID:    timerID1,
		ExpiryTime: time.Unix(0, startEvent1.GetTimestamp()).Add(timerTimeout1),
		StartedID:  startEvent1.ID,
		TaskStatus: execution.TimerTaskStatusNone,
	}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	nextPageToken := []byte{1, 2, 3}
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.ReadHistoryBranchRequest) bool {
		return req.MinEventID == 1 && req.NextPageToken == nil
	})).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{startEvent1, startEvent2},
		NextPageToken: nextPageToken,
	}, nil).Once()
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.ReadHistoryBranchRequest) bool {
		return req.MinEventID == 1 && bytes.Equal(req.NextPageToken, nextPageToken)
	})).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{firedEvent1},
		NextPageToken: nil,
	}, nil).Once()
	// only timer2 should be fired
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.MatchedBy(func(req *persistence.AppendHistoryNodesRequest) bool {
		numTimerFiredEvents := 0
		for _, event := range req.Events {
			if event.GetEventType() == types.EventTypeTimerFired {
				numTimerFiredEvents++
			}
		}
		return numTimerFiredEvents == 1
	})).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	// both timerInfo should be deleted
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return len(req.UpdateWorkflowMutation.DeleteTimerInfos) == 2
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timerActiveTaskExecutor.config.ResurrectionCheckMinDelay = dynamicproperties.GetDurationPropertyFnFilteredByDomain(timerTimeout2 - timerTimeout1)
	s.timeSource.Advance(timerTimeout2)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_NoRetryPolicy_Fire() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTimeout := 2 * time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
	)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	_, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetActivityInfo(scheduledEvent.ID)
	s.False(ok)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_NoRetryPolicy_Noop() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	identity := "identity"
	timerTimeout := 2 * time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
	)
	startedEvent := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent.ID, identity)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	completeEvent := test.AddActivityTaskCompletedEvent(mutableState, scheduledEvent.ID, startedEvent.ID, []byte(nil), identity)
	mutableState.FlushBufferedEvents()

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, completeEvent.ID, completeEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_RetryPolicy_Retry_StartToClose() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTimeout := 2 * time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)
	startedEvent := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent.ID, "identity")
	s.Nil(startedEvent)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	activityInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetActivityInfo(scheduledEvent.ID)
	s.True(ok)
	s.Equal(scheduledEvent.ID, activityInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, activityInfo.StartedID)
	// only a schedule to start timer will be created, apart from the retry timer
	s.Equal(int32(execution.TimerTaskStatusCreatedScheduleToStart), activityInfo.TimerTaskStatus)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_RetryPolicy_Retry_ScheduleToStart() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTimeout := 2 * time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	activityInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetActivityInfo(scheduledEvent.ID)
	s.True(ok)
	s.Equal(scheduledEvent.ID, activityInfo.ScheduleID)
	s.Equal(commonconstants.EmptyEventID, activityInfo.StartedID)
	// only a schedule to start timer will be created, apart from the retry timer
	s.Equal(int32(execution.TimerTaskStatusCreatedScheduleToStart), activityInfo.TimerTaskStatus)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_RetryPolicy_Noop() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	identity := "identity"
	timerTimeout := 2 * time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)
	startedEvent := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent.ID, identity)
	s.Nil(startedEvent)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	completeEvent := test.AddActivityTaskCompletedEvent(mutableState, scheduledEvent.ID, commonconstants.TransientEventID, []byte(nil), identity)
	mutableState.FlushBufferedEvents()

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, completeEvent.ID, completeEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	s.timeSource.Advance(2 * timerTimeout)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_Heartbeat_Noop() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	identity := "identity"
	timerTimeout := 2 * time.Second
	heartbeatTimerTimeout := time.Second
	scheduledEvent, _ := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(heartbeatTimerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)
	startedEvent := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent.ID, identity)
	s.Nil(startedEvent)

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	s.Equal(int(execution.TimerTypeHeartbeat), task.(*persistence.ActivityTimeoutTask).TimeoutType)
	task.SetVisibilityTimestamp(task.GetVisibilityTimestamp().Add(-time.Second))
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestProcessActivityTimeout_Resurrected() {

	_, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	identity := "identity"
	timerTimeout1 := 2 * time.Second
	timerTimeout2 := 5 * time.Second
	// schedule 2 activities
	scheduledEvent1, _ := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity1",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout1.Seconds()),
		int32(timerTimeout1.Seconds()),
		int32(timerTimeout1.Seconds()),
		int32(timerTimeout1.Seconds()),
	)
	scheduledEvent2, _ := test.AddActivityTaskScheduledEvent(
		mutableState,
		decisionCompletionID,
		"activity2",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout2.Seconds()),
		int32(timerTimeout2.Seconds()),
		int32(timerTimeout2.Seconds()),
		int32(timerTimeout2.Seconds()),
	)

	startedEvent1 := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent1.ID, identity)
	completeEvent1 := test.AddActivityTaskCompletedEvent(mutableState, scheduledEvent1.ID, startedEvent1.ID, []byte(nil), identity)
	mutableState.FlushBufferedEvents()
	// there should be a decision scheduled event after activity1 is completed
	// omitted here to make the test easier to read

	timerSequence := execution.NewTimerSequence(mutableState)
	mutableState.DeleteTimerTasks()
	modified, err := timerSequence.CreateNextActivityTimer()
	s.NoError(err)
	s.True(modified)
	task := mutableState.GetTimerTasks()[0]
	timerTask := s.newTimerTaskFromInfo(task)

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, completeEvent1.ID, completeEvent1.Version)
	s.NoError(err)
	// add resurrected activity info for activity1
	persistenceMutableState.ActivityInfos[scheduledEvent1.ID] = &persistence.ActivityInfo{
		Version:                  scheduledEvent1.Version,
		ScheduleID:               scheduledEvent1.ID,
		ScheduledEventBatchID:    scheduledEvent1.ID,
		ScheduledTime:            time.Unix(0, scheduledEvent1.GetTimestamp()),
		StartedID:                commonconstants.EmptyEventID,
		StartedTime:              time.Time{},
		ActivityID:               "activity1",
		DomainID:                 s.domainID,
		ScheduleToStartTimeout:   int32(timerTimeout1.Seconds()),
		ScheduleToCloseTimeout:   int32(timerTimeout1.Seconds()),
		StartToCloseTimeout:      int32(timerTimeout1.Seconds()),
		HeartbeatTimeout:         int32(timerTimeout1.Seconds()),
		CancelRequested:          false,
		CancelRequestID:          commonconstants.EmptyEventID,
		LastHeartBeatUpdatedTime: time.Time{},
		TimerTaskStatus:          execution.TimerTaskStatusNone,
		TaskList:                 mutableState.GetExecutionInfo().TaskList,
	}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	nextPageToken := []byte{1, 2, 3}
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.ReadHistoryBranchRequest) bool {
		return req.MinEventID == 1 && req.NextPageToken == nil
	})).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{scheduledEvent1, scheduledEvent2, startedEvent1, completeEvent1},
		NextPageToken: nextPageToken,
	}, nil).Once()
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.ReadHistoryBranchRequest) bool {
		return req.MinEventID == 1 && bytes.Equal(req.NextPageToken, nextPageToken)
	})).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{scheduledEvent1, scheduledEvent2, startedEvent1, completeEvent1},
		NextPageToken: nil,
	}, nil).Once()
	// only activity timer for activity2 should be fired
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.MatchedBy(func(req *persistence.AppendHistoryNodesRequest) bool {
		numActivityTimeoutEvents := 0
		for _, event := range req.Events {
			if event.GetEventType() == types.EventTypeActivityTaskTimedOut {
				numActivityTimeoutEvents++
			}
		}
		return numActivityTimeoutEvents == 1
	})).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	// both activityInfo should be deleted
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return len(req.UpdateWorkflowMutation.DeleteActivityInfos) == 2
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.timerActiveTaskExecutor.config.ResurrectionCheckMinDelay = dynamicproperties.GetDurationPropertyFnFilteredByDomain(timerTimeout2 - timerTimeout1)
	s.timeSource.Advance(timerTimeout2)
	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestDecisionScheduleToStartTimeout_NormalDecision() {

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeScheduleToStart),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return req.UpdateWorkflowMutation.ExecutionInfo.DecisionAttempt == 1 &&
			req.UpdateWorkflowMutation.ExecutionInfo.DecisionScheduleID == 4 &&
			req.UpdateWorkflowMutation.ExecutionInfo.NextEventID == 4 && // transient decision
			len(req.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer]) == 1 // another schedule to start timer
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestDecisionScheduleToStartTimeout_TransientDecision() {
	s.mockShard.GetConfig().NormalDecisionScheduleToStartMaxAttempts = dynamicproperties.GetIntPropertyFilteredByDomain(1)

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	decisionAttempt := int64(1)
	mutableState.GetExecutionInfo().DecisionAttempt = decisionAttempt // fake a transient decision
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType:     int(types.TimeoutTypeScheduleToStart),
		EventID:         di.ScheduleID,
		ScheduleAttempt: decisionAttempt,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return req.UpdateWorkflowMutation.ExecutionInfo.DecisionAttempt == 2 &&
			req.UpdateWorkflowMutation.ExecutionInfo.DecisionScheduleID == 2 &&
			req.UpdateWorkflowMutation.ExecutionInfo.NextEventID == 2 && // transient decision
			len(req.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer]) == 0 // since the max attempt is 1 at the beginning of the test
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestDecisionScheduleToStartTimeout_StickyDecision() {

	workflowExecution, mutableState, _, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.StickyTaskList = "sticky-tasklist"
	executionInfo.StickyScheduleToStartTimeout = 1
	executionInfo.LastUpdatedTimestamp = s.timeSource.Now()

	// schedule a second (sticky) decision task
	di := test.AddDecisionTaskScheduledEvent(mutableState)

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeScheduleToStart),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, di.ScheduleID, di.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		executionInfo := req.UpdateWorkflowMutation.ExecutionInfo
		return executionInfo.DecisionAttempt == 0 && // attempt is not increased when convert to normal decision
			executionInfo.DecisionScheduleID == 7 &&
			executionInfo.NextEventID == 8 && // normal decision
			executionInfo.StickyTaskList == "" && // stickyness should be cleared
			executionInfo.StickyScheduleToStartTimeout == 0 && // stickyness should be cleared
			len(req.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer]) == 1 // schedule to start timer
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestDecisionStartToCloseTimeout_Fire() {

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)
	startedEvent := test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeStartToClose),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startedEvent.ID, startedEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	decisionInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetPendingDecision()
	s.True(ok)
	s.True(decisionInfo.ScheduleID != commonconstants.EmptyEventID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(int64(1), decisionInfo.Attempt)
}

func (s *timerActiveTaskExecutorSuite) TestDecisionStartToCloseTimeout_ExceedsMaxAttempts() {
	s.mockShard.GetConfig().DecisionRetryMaxAttempts = dynamicproperties.GetIntPropertyFilteredByDomain(1)
	s.mockShard.GetConfig().EnforceDecisionTaskAttempts = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)
	startedEvent := test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeStartToClose),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startedEvent.ID, startedEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return req.UpdateWorkflowMutation.ExecutionInfo.State == persistence.WorkflowStateCompleted &&
			req.UpdateWorkflowMutation.ExecutionInfo.CloseStatus == persistence.WorkflowCloseStatusTerminated
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	s.False(s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).IsWorkflowExecutionRunning())
}

func (s *timerActiveTaskExecutorSuite) TestDecisionStartToCloseTimeout_BelowMaxAttempts() {
	s.mockShard.GetConfig().DecisionRetryMaxAttempts = dynamicproperties.GetIntPropertyFilteredByDomain(5)
	s.mockShard.GetConfig().EnforceDecisionTaskAttempts = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)
	startedEvent := test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeStartToClose),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startedEvent.ID, startedEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return req.UpdateWorkflowMutation.ExecutionInfo.State == persistence.WorkflowStateRunning &&
			req.UpdateWorkflowMutation.ExecutionInfo.DecisionScheduleID != commonconstants.EmptyEventID
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	decisionInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetPendingDecision()
	s.True(ok)
	s.Equal(int64(1), decisionInfo.Attempt)
	s.True(s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).IsWorkflowExecutionRunning())
}

func (s *timerActiveTaskExecutorSuite) TestDecisionStartToCloseTimeout_AttemptsNotEnforced() {
	s.mockShard.GetConfig().DecisionRetryMaxAttempts = dynamicproperties.GetIntPropertyFilteredByDomain(1)
	s.mockShard.GetConfig().EnforceDecisionTaskAttempts = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)
	startedEvent := test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeStartToClose),
		EventID:     di.ScheduleID,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startedEvent.ID, startedEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.UpdateWorkflowExecutionRequest) bool {
		return req.UpdateWorkflowMutation.ExecutionInfo.State == persistence.WorkflowStateRunning &&
			req.UpdateWorkflowMutation.ExecutionInfo.DecisionScheduleID != commonconstants.EmptyEventID
	})).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	decisionInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetPendingDecision()
	s.True(ok)
	s.Equal(int64(1), decisionInfo.Attempt)
	s.True(s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).IsWorkflowExecutionRunning())
}

func (s *timerActiveTaskExecutorSuite) TestDecisionStartToCloseTimeout_Noop() {

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	di := test.AddDecisionTaskScheduledEvent(mutableState)
	startedEvent := test.AddDecisionTaskStartedEvent(mutableState, di.ScheduleID, mutableState.GetExecutionInfo().TaskList, uuid.New())

	timerTask := s.newTimerTaskFromInfo(&persistence.DecisionTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: int(types.TimeoutTypeStartToClose),
		EventID:     di.ScheduleID - 1, // no corresponding decision for this scheduleID
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startedEvent.ID, startedEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestWorkflowBackoffTimer_Fire() {

	workflowExecution, mutableState, err := test.StartWorkflow(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTask := s.newTimerTaskFromInfo(&persistence.WorkflowBackoffTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: persistence.WorkflowBackoffTimeoutTypeRetry,
	})

	startEvent, err := mutableState.GetStartEvent(context.Background())
	s.NoError(err)
	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, startEvent.ID, startEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	decisionInfo, ok := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetPendingDecision()
	s.True(ok)
	s.True(decisionInfo.ScheduleID != commonconstants.EmptyEventID)
	s.Equal(commonconstants.EmptyEventID, decisionInfo.StartedID)
	s.Equal(int64(0), decisionInfo.Attempt)
}

func (s *timerActiveTaskExecutorSuite) TestWorkflowBackoffTimer_Noop() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTask := s.newTimerTaskFromInfo(&persistence.WorkflowBackoffTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		TimeoutType: persistence.WorkflowBackoffTimeoutTypeRetry,
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestActivityRetryTimer_Fire() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTimeout := 2 * time.Second
	scheduledEvent, activityInfo := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)
	activityInfo.Attempt = 1

	timerTask := s.newTimerTaskFromInfo(&persistence.ActivityRetryTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		EventID: activityInfo.ScheduleID,
		Attempt: int64(activityInfo.Attempt),
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockMatchingClient.EXPECT().AddActivityTask(
		gomock.Any(),
		&types.AddActivityTaskRequest{
			DomainUUID:       activityInfo.DomainID,
			SourceDomainUUID: activityInfo.DomainID,
			Execution:        &workflowExecution,
			TaskList: &types.TaskList{
				Name: activityInfo.TaskList,
			},
			ScheduleID:                    activityInfo.ScheduleID,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(activityInfo.ScheduleToStartTimeout),
			PartitionConfig:               mutableState.GetExecutionInfo().PartitionConfig,
		},
	).Return(&types.AddActivityTaskResponse{}, nil).Times(1)

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestActivityRetryTimer_Noop() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTimeout := 2 * time.Second
	scheduledEvent, activityInfo := test.AddActivityTaskScheduledEventWithRetry(
		mutableState,
		decisionCompletionID,
		"activity",
		"activity type",
		mutableState.GetExecutionInfo().TaskList,
		[]byte(nil),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		int32(timerTimeout.Seconds()),
		&types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.2,
			MaximumIntervalInSeconds:    5,
			MaximumAttempts:             5,
			NonRetriableErrorReasons:    []string{"（╯' - ')╯ ┻━┻ "},
			ExpirationIntervalInSeconds: 999,
		},
	)
	startedEvent := test.AddActivityTaskStartedEvent(mutableState, scheduledEvent.ID, "identity")
	s.Nil(startedEvent)

	timerTask := s.newTimerTaskFromInfo(&persistence.ActivityRetryTimerTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
		EventID: activityInfo.ScheduleID,
		Attempt: int64(activityInfo.Attempt),
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, scheduledEvent.ID, scheduledEvent.Version)
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)
}

func (s *timerActiveTaskExecutorSuite) TestWorkflowTimeout_Fire() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	timerTask := s.newTimerTaskFromInfo(&persistence.WorkflowTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	running := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).IsWorkflowExecutionRunning()
	s.False(running)
}

func (s *timerActiveTaskExecutorSuite) TestWorkflowTimeout_ContinueAsNew_Retry() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	// need to override the workflow retry policy
	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.HasRetryPolicy = true
	executionInfo.ExpirationTime = s.timeSource.Now().Add(1000 * time.Second)
	executionInfo.MaximumAttempts = 10
	executionInfo.InitialInterval = 1
	executionInfo.MaximumInterval = 1
	executionInfo.BackoffCoefficient = 1

	timerTask := s.newTimerTaskFromInfo(&persistence.WorkflowTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	// one for current workflow, one for new
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Times(2)
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), s.domainID, gomock.Any()).Return(&types.ActiveClusterInfo{}, nil).AnyTimes()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	state, closeStatus := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetWorkflowStateCloseStatus()
	s.Equal(persistence.WorkflowStateCompleted, state)
	s.Equal(persistence.WorkflowCloseStatusContinuedAsNew, closeStatus)
}

func (s *timerActiveTaskExecutorSuite) TestWorkflowTimeout_ContinueAsNew_Cron() {

	workflowExecution, mutableState, decisionCompletionID, err := test.SetupWorkflowWithCompletedDecision(s.T(), s.mockShard, s.domainID)
	s.NoError(err)

	executionInfo := mutableState.GetExecutionInfo()
	executionInfo.StartTimestamp = s.timeSource.Now()
	executionInfo.CronSchedule = "* * * * *"

	timerTask := s.newTimerTaskFromInfo(&persistence.WorkflowTimeoutTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: workflowExecution.GetWorkflowID(),
			RunID:      workflowExecution.GetRunID(),
		},
		TaskData: persistence.TaskData{
			Version:             s.version,
			TaskID:              int64(100),
			VisibilityTimestamp: s.timeSource.Now(),
		},
	})

	persistenceMutableState, err := test.CreatePersistenceMutableState(s.T(), mutableState, decisionCompletionID, mutableState.GetCurrentVersion())
	s.NoError(err)
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{State: persistenceMutableState}, nil)
	// one for current workflow, one for new
	s.mockHistoryV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.Anything).Return(&persistence.AppendHistoryNodesResponse{}, nil).Times(2)
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()

	s.mockShard.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), s.domainID, gomock.Any()).Return(&types.ActiveClusterInfo{}, nil).AnyTimes()

	_, err = s.timerActiveTaskExecutor.Execute(timerTask)
	s.NoError(err)

	state, closeStatus := s.getMutableStateFromCache(s.domainID, workflowExecution.GetWorkflowID(), workflowExecution.GetRunID()).GetWorkflowStateCloseStatus()
	s.Equal(persistence.WorkflowStateCompleted, state)
	s.Equal(persistence.WorkflowCloseStatusContinuedAsNew, closeStatus)
}

func (s *timerActiveTaskExecutorSuite) getMutableStateFromCache(
	domainID string,
	workflowID string,
	runID string,
) execution.MutableState {

	return s.executionCache.Get(
		definition.NewWorkflowIdentifier(domainID, workflowID, runID),
	).(execution.Context).GetWorkflowExecution()
}

func (s *timerActiveTaskExecutorSuite) newTimerTaskFromInfo(
	task persistence.Task,
) Task {
	return NewHistoryTask(s.mockShard, task, QueueTypeActiveTimer, s.logger, nil, nil, nil, nil, nil)
}

func (s *timerActiveTaskExecutorSuite) TestActiveTaskTimeout() {
	deleteHistoryEventTask := s.newTimerTaskFromInfo(&persistence.DeleteHistoryEventTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   s.domainID,
			WorkflowID: "workflowID",
			RunID:      "runID",
		},
		TaskData: persistence.TaskData{
			Version: s.version,
			TaskID:  int64(100),
		},
	})
	s.timerActiveTaskExecutor.Execute(deleteHistoryEventTask)
}
