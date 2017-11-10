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

package history

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber-go/tally"
	"github.com/uber/cadence/.gen/go/history"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
)

type (
	engineSuite struct {
		suite.Suite
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
		mockHistoryEngine  *historyEngineImpl
		mockMatchingClient *mocks.MatchingClient
		mockHistoryClient  *mocks.HistoryClient
		mockMetadataMgr    *mocks.MetadataManager
		mockVisibilityMgr  *mocks.VisibilityManager
		mockExecutionMgr   *mocks.ExecutionManager
		mockHistoryMgr     *mocks.HistoryManager
		mockShardManager   *mocks.ShardManager
		shardClosedCh      chan int
		eventSerializer    historyEventSerializer
		config             *Config
		logger             bark.Logger
	}
)

func TestEngineSuite(t *testing.T) {
	s := new(engineSuite)
	suite.Run(t, s)
}

func (s *engineSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}

	s.logger = bark.NewLoggerFromLogrus(log.New())
	s.config = NewConfig(1)
}

func (s *engineSuite) TearDownSuite() {

}

func (s *engineSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())

	shardID := 0
	s.mockMatchingClient = &mocks.MatchingClient{}
	s.mockHistoryClient = &mocks.HistoryClient{}
	s.mockMetadataMgr = &mocks.MetadataManager{}
	s.mockVisibilityMgr = &mocks.VisibilityManager{}
	s.mockExecutionMgr = &mocks.ExecutionManager{}
	s.mockHistoryMgr = &mocks.HistoryManager{}
	s.mockShardManager = &mocks.ShardManager{}
	s.shardClosedCh = make(chan int, 100)
	s.eventSerializer = newJSONHistoryEventSerializer()

	mockShard := &shardContextImpl{
		shardInfo:                 &persistence.ShardInfo{ShardID: shardID, RangeID: 1, TransferAckLevel: 0},
		transferSequenceNumber:    1,
		executionManager:          s.mockExecutionMgr,
		historyMgr:                s.mockHistoryMgr,
		shardManager:              s.mockShardManager,
		maxTransferSequenceNumber: 100000,
		closeCh:                   s.shardClosedCh,
		config:                    s.config,
		logger:                    s.logger,
		metricsClient:             metrics.NewClient(tally.NoopScope, metrics.History),
	}

	historyCache := newHistoryCache(mockShard, s.logger)
	domainCache := cache.NewDomainCache(s.mockMetadataMgr, s.logger)
	h := &historyEngineImpl{
		shard:              mockShard,
		executionManager:   s.mockExecutionMgr,
		historyMgr:         s.mockHistoryMgr,
		historyCache:       historyCache,
		domainCache:        domainCache,
		logger:             s.logger,
		metricsClient:      metrics.NewClient(tally.NoopScope, metrics.History),
		tokenSerializer:    common.NewJSONTaskTokenSerializer(),
		hSerializerFactory: persistence.NewHistorySerializerFactory(),
	}
	h.txProcessor = newTransferQueueProcessor(mockShard, h, s.mockVisibilityMgr, s.mockMatchingClient, s.mockHistoryClient)
	h.timerProcessor = newTimerQueueProcessor(mockShard, h, s.mockExecutionMgr, s.logger)
	s.mockHistoryEngine = h
}

func (s *engineSuite) TearDownTest() {
	s.mockMatchingClient.AssertExpectations(s.T())
	s.mockExecutionMgr.AssertExpectations(s.T())
	s.mockHistoryMgr.AssertExpectations(s.T())
	s.mockShardManager.AssertExpectations(s.T())
	s.mockVisibilityMgr.AssertExpectations(s.T())
}

func (s *engineSuite) TestGetWorkflowExecutionNextEventIDSync() {
	ctx := context.Background()
	domainID := "domainId"
	execution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("test-get-workflow-execution-event-id"),
		RunId:      common.StringPtr(uuid.NewUUID().String()),
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, execution, "wType", tasklist, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tasklist, identity)
	ms := createMutableState(msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gweResponse, nil).Once()

	// test get the next event ID instantly
	response, err := s.mockHistoryEngine.GetWorkflowExecutionNextEventID(ctx, &history.GetWorkflowExecutionNextEventIDRequest{
		DomainUUID: common.StringPtr(domainID),
		Execution:  &execution,
	})
	s.Nil(err)
	s.Equal(int64(4), *response.EventId)
}

func (s *engineSuite) TestGetWorkflowExecutionNextEventIDLongPoll() {
	ctx := context.Background()
	domainID := "domainId"
	execution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("test-get-workflow-execution-event-id"),
		RunId:      common.StringPtr(uuid.NewUUID().String()),
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, execution, "wType", tasklist, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tasklist, identity)
	ms := createMutableState(msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gweResponse, nil).Once()

	// test long poll on next event ID change
	asycWorkflowUpdate := func(delay time.Duration) {
		taskToken, _ := json.Marshal(&common.TaskToken{
			WorkflowID: *execution.WorkflowId,
			RunID:      *execution.RunId,
			ScheduleID: 2,
		})
		s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

		timer := time.NewTimer(delay)

		<-timer.C
		s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
			DomainUUID: common.StringPtr(domainID),
			CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
				TaskToken: taskToken,
				Identity:  &identity,
			},
		})
		// right now the next event ID is 5
	}

	// return immediately, since the expected next event ID appears
	response, err := s.mockHistoryEngine.GetWorkflowExecutionNextEventID(ctx, &history.GetWorkflowExecutionNextEventIDRequest{
		DomainUUID:          common.StringPtr(domainID),
		Execution:           &execution,
		ExpectedNextEventID: common.Int64Ptr(4),
	})
	s.Nil(err)
	s.Equal(int64(4), *response.EventId)

	// long poll, new event happen before long poll timeout
	go asycWorkflowUpdate(time.Second * 10)
	response, err = s.mockHistoryEngine.GetWorkflowExecutionNextEventID(ctx, &history.GetWorkflowExecutionNextEventIDRequest{
		DomainUUID:          common.StringPtr(domainID),
		Execution:           &execution,
		ExpectedNextEventID: common.Int64Ptr(5),
	})
	s.Nil(err)
	s.Equal(int64(5), *response.EventId)
}

func (s *engineSuite) TestGetWorkflowExecutionNextEventIDLongPollTimeout() {
	ctx := context.Background()
	domainID := "domainId"
	execution := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("test-get-workflow-execution-event-id"),
		RunId:      common.StringPtr(uuid.NewUUID().String()),
	}
	tasklist := "testTaskList"
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, execution, "wType", tasklist, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tasklist, identity)
	ms := createMutableState(msBuilder)
	gweResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	// right now the next event ID is 4
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gweResponse, nil).Once()

	// long poll, no event happen after long poll timeout
	response, err := s.mockHistoryEngine.GetWorkflowExecutionNextEventID(ctx, &history.GetWorkflowExecutionNextEventIDRequest{
		DomainUUID:          common.StringPtr(domainID),
		Execution:           &execution,
		ExpectedNextEventID: common.Int64Ptr(5),
	})
	s.Nil(err)
	s.Equal(int64(4), *response.EventId)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedInvalidToken() {
	domainID := "domainId"
	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        invalidToken,
			Decisions:        nil,
			ExecutionContext: nil,
			Identity:         &identity,
		},
	})

	s.NotNil(err)
	s.IsType(&workflow.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfNoExecution() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil, &workflow.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfGetExecutionFailed() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil, errors.New("FAILED")).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedUpdateExecutionFailed() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfTaskCompleted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	startedEvent := addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)
	addDecisionTaskCompletedEvent(msBuilder, *scheduleEvent.EventId, *startedEvent.EventId, nil, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedIfTaskNotStarted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	addDecisionTaskScheduledEvent(msBuilder)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedConflictOnUpdate() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	identity := "testIdentity"
	context := []byte("context")
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

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId,
		activity1ID, activity1Type, tl, activity1Input, 100, 10, 5)
	activity2ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId,
		activity2ID, activity2Type, tl, activity2Input, 100, 10, 5)
	activity1StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	activity2StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity2ScheduledEvent.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity1ScheduledEvent.EventId,
		*activity1StartedEvent.EventId, activity1Result, identity)
	decisionScheduledEvent2, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent2 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent2.EventId, tl, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: *decisionScheduledEvent2.EventId,
	})

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeScheduleActivityTask),
		ScheduleActivityTaskDecisionAttributes: &workflow.ScheduleActivityTaskDecisionAttributes{
			ActivityId:   common.StringPtr(activity3ID),
			ActivityType: &workflow.ActivityType{Name: common.StringPtr(activity3Type)},
			TaskList:     &workflow.TaskList{Name: &tl},
			Input:        activity3Input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	addActivityTaskCompletedEvent(msBuilder, *activity2ScheduledEvent.EventId,
		*activity2StartedEvent.EventId, activity2Result, identity)

	ms2 := createMutableState(msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(
		&persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	s.Equal(int64(16), ms2.ExecutionInfo.NextEventID)
	s.Equal(*decisionStartedEvent2.EventId, ms2.ExecutionInfo.LastProcessedEvent)
	s.Equal(context, ms2.ExecutionInfo.ExecutionContext)

	executionBuilder := s.getBuilder(domainID, we)
	activity3Attributes := s.getActivityScheduledEvent(executionBuilder, 13).ActivityTaskScheduledEventAttributes
	s.Equal(activity3ID, *activity3Attributes.ActivityId)
	s.Equal(activity3Type, *activity3Attributes.ActivityType.Name)
	s.Equal(int64(12), *activity3Attributes.DecisionTaskCompletedEventId)
	s.Equal(tl, *activity3Attributes.TaskList.Name)
	s.Equal(activity3Input, activity3Attributes.Input)
	s.Equal(int32(100), *activity3Attributes.ScheduleToCloseTimeoutSeconds)
	s.Equal(int32(10), *activity3Attributes.ScheduleToStartTimeoutSeconds)
	s.Equal(int32(50), *activity3Attributes.StartToCloseTimeoutSeconds)
	s.Equal(int32(5), *activity3Attributes.HeartbeatTimeoutSeconds)

	di, ok := executionBuilder.GetPendingDecision(15)
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedMaxAttemptsExceeded() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	context := []byte("context")
	input := []byte("input")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeScheduleActivityTask),
		ScheduleActivityTaskDecisionAttributes: &workflow.ScheduleActivityTaskDecisionAttributes{
			ActivityId:   common.StringPtr("activity1"),
			ActivityType: &workflow.ActivityType{Name: common.StringPtr("activity_type1")},
			TaskList:     &workflow.TaskList{Name: &tl},
			Input:        input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	for i := 0; i < conditionalRetryCount; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(
			&persistence.ConditionFailedError{}).Once()
	}

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.NotNil(err)
	s.Equal(ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedCompleteWorkflowFailed() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	identity := "testIdentity"
	context := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")
	activity2ID := "activity2"
	activity2Type := "activity_type2"
	activity2Input := []byte("input2")
	activity2Result := []byte("activity2_result")
	workflowResult := []byte("workflow result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId,
		activity1ID, activity1Type, tl, activity1Input, 100, 10, 5)
	activity2ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId,
		activity2ID, activity2Type, tl, activity2Input, 100, 10, 5)
	activity1StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	activity2StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity2ScheduledEvent.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity1ScheduledEvent.EventId,
		*activity1StartedEvent.EventId, activity1Result, identity)
	decisionScheduledEvent2, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent2.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity2ScheduledEvent.EventId,
		*activity2StartedEvent.EventId, activity2Result, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: *decisionScheduledEvent2.EventId,
	})

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeCompleteWorkflowExecution),
		CompleteWorkflowExecutionDecisionAttributes: &workflow.CompleteWorkflowExecutionDecisionAttributes{
			Result: workflowResult,
		},
	}}

	for i := 0; i < 2; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(15), executionBuilder.executionInfo.NextEventID)
	s.Equal(*decisionStartedEvent1.EventId, executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(context, executionBuilder.executionInfo.ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.True(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedFailWorkflowFailed() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	identity := "testIdentity"
	context := []byte("context")
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

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 5)
	activity2ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 5)
	activity1StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	activity2StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity2ScheduledEvent.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity1ScheduledEvent.EventId,
		*activity1StartedEvent.EventId, activity1Result, identity)
	decisionScheduledEvent2, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent2.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity2ScheduledEvent.EventId,
		*activity2StartedEvent.EventId, activity2Result, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: *decisionScheduledEvent2.EventId,
	})

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeFailWorkflowExecution),
		FailWorkflowExecutionDecisionAttributes: &workflow.FailWorkflowExecutionDecisionAttributes{
			Reason:  &reason,
			Details: details,
		},
	}}

	for i := 0; i < 2; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(15), executionBuilder.executionInfo.NextEventID)
	s.Equal(*decisionStartedEvent1.EventId, executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(context, executionBuilder.executionInfo.ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.True(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedBadDecisionAttributes() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	identity := "testIdentity"
	context := []byte("context")
	activity1ID := "activity1"
	activity1Type := "activity_type1"
	activity1Input := []byte("input1")
	activity1Result := []byte("activity1_result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 5)
	activity1StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activity1ScheduledEvent.EventId,
		*activity1StartedEvent.EventId, activity1Result, identity)
	decisionScheduledEvent2, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent2.EventId, tl, identity)

	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: *decisionScheduledEvent2.EventId,
	})

	// Decision with nil attributes
	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeCompleteWorkflowExecution),
	}}

	for i := 0; i < 2; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	}

	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedSingleActivityScheduledDecision() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"
	context := []byte("context")
	input := []byte("input")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeScheduleActivityTask),
		ScheduleActivityTaskDecisionAttributes: &workflow.ScheduleActivityTaskDecisionAttributes{
			ActivityId:   common.StringPtr("activity1"),
			ActivityType: &workflow.ActivityType{Name: common.StringPtr("activity_type1")},
			TaskList:     &workflow.TaskList{Name: &tl},
			Input:        input,
			ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
			StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
			HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
		},
	}}

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(6), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(context, executionBuilder.executionInfo.ExecutionContext)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())

	activity1Attributes := s.getActivityScheduledEvent(executionBuilder, int64(5)).ActivityTaskScheduledEventAttributes
	s.Equal("activity1", *activity1Attributes.ActivityId)
	s.Equal("activity_type1", *activity1Attributes.ActivityType.Name)
	s.Equal(int64(4), *activity1Attributes.DecisionTaskCompletedEventId)
	s.Equal(tl, *activity1Attributes.TaskList.Name)
	s.Equal(input, activity1Attributes.Input)
	s.Equal(int32(100), *activity1Attributes.ScheduleToCloseTimeoutSeconds)
	s.Equal(int32(10), *activity1Attributes.ScheduleToStartTimeoutSeconds)
	s.Equal(int32(50), *activity1Attributes.StartToCloseTimeoutSeconds)
	s.Equal(int32(5), *activity1Attributes.HeartbeatTimeoutSeconds)
}

func (s *engineSuite) TestRespondDecisionTaskCompletedCompleteWorkflowSuccess() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	context := []byte("context")
	workflowResult := []byte("success")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeCompleteWorkflowExecution),
		CompleteWorkflowExecutionDecisionAttributes: &workflow.CompleteWorkflowExecutionDecisionAttributes{
			Result: workflowResult,
		},
	}}

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()
	s.mockMetadataMgr.On("GetDomain", mock.Anything).Return(
		&persistence.GetDomainResponse{Config: &persistence.DomainConfig{Retention: 1}}, nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(6), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(context, executionBuilder.executionInfo.ExecutionContext)
	s.Equal(persistence.WorkflowStateCompleted, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRespondDecisionTaskCompletedFailWorkflowSuccess() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	context := []byte("context")
	details := []byte("fail workflow details")
	reason := "fail workflow reason"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	scheduleEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *scheduleEvent.EventId, tl, identity)

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeFailWorkflowExecution),
		FailWorkflowExecutionDecisionAttributes: &workflow.FailWorkflowExecutionDecisionAttributes{
			Reason:  &reason,
			Details: details,
		},
	}}

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()
	s.mockMetadataMgr.On("GetDomain", mock.Anything).Return(
		&persistence.GetDomainResponse{Config: &persistence.DomainConfig{Retention: 1}}, nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: context,
			Identity:         &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(6), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(context, executionBuilder.executionInfo.ExecutionContext)
	s.Equal(persistence.WorkflowStateCompleted, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRespondActivityTaskCompletedInvalidToken() {
	domainID := "domainId"
	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: invalidToken,
			Result:    nil,
			Identity:  &identity,
		},
	})

	s.NotNil(err)
	s.IsType(&workflow.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfNoExecution() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil, &workflow.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfGetExecutionFailed() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil, errors.New("FAILED")).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskCompletedUpdateExecutionFailed() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  &identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfTaskCompleted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	activityStartedEvent := addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)
	addActivityTaskCompletedEvent(msBuilder, *activityScheduledEvent.EventId, *activityStartedEvent.EventId,
		activityResult, identity)
	addDecisionTaskScheduledEvent(msBuilder)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedIfTaskNotStarted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedConflictOnUpdate() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
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

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 5)
	activity2ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	addActivityTaskStartedEvent(msBuilder, *activity2ScheduledEvent.EventId, tl, identity)

	ms1 := createMutableState(msBuilder)
	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: ms1}

	ms2 := createMutableState(msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(&persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activity1Result,
			Identity:  &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(11), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	s.True(executionBuilder.HasPendingDecisionTask())
	di, ok := executionBuilder.GetPendingDecision(int64(10))
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
	s.Equal(int64(10), di.ScheduleID)
	s.Equal(emptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskCompletedMaxAttemptsExceeded() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	for i := 0; i < conditionalRetryCount; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(&persistence.ConditionFailedError{}).Once()
	}

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  &identity,
		},
	})
	s.Equal(ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondActivityTaskCompletedSuccess() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	activityResult := []byte("activity result")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCompleted(&history.RespondActivityTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondActivityTaskCompletedRequest{
			TaskToken: taskToken,
			Result:    activityResult,
			Identity:  &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(9), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	s.True(executionBuilder.HasPendingDecisionTask())
	di, ok := executionBuilder.GetPendingDecision(int64(8))
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(emptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskFailedInvalidToken() {
	domainID := "domainId"
	invalidToken, _ := json.Marshal("bad token")
	identity := "testIdentity"

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: invalidToken,
			Identity:  &identity,
		},
	})

	s.NotNil(err)
	s.IsType(&workflow.BadRequestError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfNoExecution() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil,
		&workflow.EntityNotExistsError{}).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfGetExecutionFailed() {
	domainID := "domainId"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 2,
	})
	identity := "testIdentity"

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(nil,
		errors.New("FAILED")).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskFailedUpdateExecutionFailed() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(errors.New("FAILED")).Once()
	s.mockShardManager.On("UpdateShard", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.EqualError(err, "FAILED")
}

func (s *engineSuite) TestRespondActivityTaskFailedIfTaskCompleted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	failReason := "fail reason"
	details := []byte("fail details")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	activityStartedEvent := addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)
	addActivityTaskFailedEvent(msBuilder, *activityScheduledEvent.EventId, *activityStartedEvent.EventId,
		failReason, details, identity)
	addDecisionTaskScheduledEvent(msBuilder)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   details,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedIfTaskNotStarted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedConflictOnUpdate() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
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

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 25, 200, identity)
	decisionScheduledEvent1, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent1 := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent1.EventId, tl, identity)
	decisionCompletedEvent1 := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent1.EventId,
		*decisionStartedEvent1.EventId, nil, identity)
	activity1ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity1ID,
		activity1Type, tl, activity1Input, 100, 10, 5)
	activity2ScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent1.EventId, activity2ID,
		activity2Type, tl, activity2Input, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activity1ScheduledEvent.EventId, tl, identity)
	activity2StartedEvent := addActivityTaskStartedEvent(msBuilder, *activity2ScheduledEvent.EventId, tl, identity)

	ms1 := createMutableState(msBuilder)
	gwmsResponse1 := &persistence.GetWorkflowExecutionResponse{State: ms1}

	addActivityTaskCompletedEvent(msBuilder, *activity2ScheduledEvent.EventId,
		*activity2StartedEvent.EventId, activity2Result, identity)
	addDecisionTaskScheduledEvent(msBuilder)

	ms2 := createMutableState(msBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse1, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(&persistence.ConditionFailedError{}).Once()

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   details,
			Identity:  &identity,
		},
	})
	s.Nil(err, s.printHistory(msBuilder))
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(12), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	s.True(executionBuilder.HasPendingDecisionTask())
	di, ok := executionBuilder.GetPendingDecision(int64(10))
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
	s.Equal(int64(10), di.ScheduleID)
	s.Equal(emptyEventID, di.StartedID)
}

func (s *engineSuite) TestRespondActivityTaskFailedMaxAttemptsExceeded() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	for i := 0; i < conditionalRetryCount; i++ {
		ms := createMutableState(msBuilder)
		gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

		s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
		s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
		s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(&persistence.ConditionFailedError{}).Once()
	}

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Identity:  &identity,
		},
	})
	s.Equal(ErrMaxAttemptsExceeded, err)
}

func (s *engineSuite) TestRespondActivityTaskFailedSuccess() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")
	failReason := "failed"
	failDetails := []byte("fail details.")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 5)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskFailed(&history.RespondActivityTaskFailedRequest{
		DomainUUID: common.StringPtr(domainID),
		FailedRequest: &workflow.RespondActivityTaskFailedRequest{
			TaskToken: taskToken,
			Reason:    &failReason,
			Details:   failDetails,
			Identity:  &identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(9), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	s.True(executionBuilder.HasPendingDecisionTask())
	di, ok := executionBuilder.GetPendingDecision(int64(8))
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
	s.Equal(int64(8), di.ScheduleID)
	s.Equal(emptyEventID, di.StartedID)
}

func (s *engineSuite) TestRecordActivityTaskHeartBeatSuccess_NoTimer() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 0)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	// No HeartBeat timer running.
	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	detais := []byte("details")

	_, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(&history.RecordActivityTaskHeartbeatRequest{
		DomainUUID: common.StringPtr(domainID),
		HeartbeatRequest: &workflow.RecordActivityTaskHeartbeatRequest{
			TaskToken: taskToken,
			Identity:  &identity,
			Details:   detais,
		},
	})
	s.Nil(err)
}

func (s *engineSuite) TestRecordActivityTaskHeartBeatSuccess_TimerRunning() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 1)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	// HeartBeat timer running.
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	detais := []byte("details")

	_, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(&history.RecordActivityTaskHeartbeatRequest{
		DomainUUID: common.StringPtr(domainID),
		HeartbeatRequest: &workflow.RecordActivityTaskHeartbeatRequest{
			TaskToken: taskToken,
			Identity:  &identity,
			Details:   detais,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(7), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRespondActivityTaskCanceled_Scheduled() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 1)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(&history.RespondActivityTaskCanceledRequest{
		DomainUUID: common.StringPtr(domainID),
		CancelRequest: &workflow.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.NotNil(err)
	s.IsType(&workflow.EntityNotExistsError{}, err)
}

func (s *engineSuite) TestRespondActivityTaskCanceled_Started() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 5,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 1)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)
	msBuilder.AddActivityTaskCancelRequestedEvent(*decisionCompletedEvent.EventId, activityID, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondActivityTaskCanceled(&history.RespondActivityTaskCanceledRequest{
		DomainUUID: common.StringPtr(domainID),
		CancelRequest: &workflow.RespondActivityTaskCanceledRequest{
			TaskToken: taskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(10), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	s.True(executionBuilder.HasPendingDecisionTask())
	di, ok := executionBuilder.GetPendingDecision(int64(9))
	s.True(ok)
	s.Equal(int32(200), di.DecisionTimeout)
	s.Equal(int64(9), di.ScheduleID)
	s.Equal(emptyEventID, di.StartedID)
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_NotScheduled() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	activityID := "activity1_id"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeRequestCancelActivityTask),
		RequestCancelActivityTaskDecisionAttributes: &workflow.RequestCancelActivityTaskDecisionAttributes{
			ActivityId: common.StringPtr(activityID),
		},
	}}

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)
	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(7), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Scheduled() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 1)
	decisionScheduled2Event, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduled2Event.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeRequestCancelActivityTask),
		RequestCancelActivityTaskDecisionAttributes: &workflow.RequestCancelActivityTaskDecisionAttributes{
			ActivityId: common.StringPtr(activityID),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(11), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(7), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_NoHeartBeat() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 0)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)
	decisionScheduled2Event, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduled2Event.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeRequestCancelActivityTask),
		RequestCancelActivityTaskDecisionAttributes: &workflow.RequestCancelActivityTaskDecisionAttributes{
			ActivityId: common.StringPtr(activityID),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(11), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(8), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())

	// Try recording activity heartbeat
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	activityTaskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 5,
	})

	hbResponse, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(&history.RecordActivityTaskHeartbeatRequest{
		DomainUUID: common.StringPtr(domainID),
		HeartbeatRequest: &workflow.RecordActivityTaskHeartbeatRequest{
			TaskToken: activityTaskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	s.NotNil(hbResponse)
	s.True(*hbResponse.CancelRequested)

	// Try cancelling the request.
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(&history.RespondActivityTaskCanceledRequest{
		DomainUUID: common.StringPtr(domainID),
		CancelRequest: &workflow.RespondActivityTaskCanceledRequest{
			TaskToken: activityTaskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)

	executionBuilder = s.getBuilder(domainID, we)
	s.Equal(int64(13), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(8), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.True(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestRequestCancel_RespondDecisionTaskCompleted_Success() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 7,
	})
	identity := "testIdentity"
	activityID := "activity1_id"
	activityType := "activity_type1"
	activityInput := []byte("input1")

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	activityScheduledEvent, _ := addActivityTaskScheduledEvent(msBuilder, *decisionCompletedEvent.EventId, activityID,
		activityType, tl, activityInput, 100, 10, 1)
	addActivityTaskStartedEvent(msBuilder, *activityScheduledEvent.EventId, tl, identity)
	decisionScheduled2Event, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduled2Event.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeRequestCancelActivityTask),
		RequestCancelActivityTaskDecisionAttributes: &workflow.RequestCancelActivityTaskDecisionAttributes{
			ActivityId: common.StringPtr(activityID),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(11), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(8), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())

	// Try recording activity heartbeat
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	activityTaskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: "wId",
		RunID:      "rId",
		ScheduleID: 5,
	})

	hbResponse, err := s.mockHistoryEngine.RecordActivityTaskHeartbeat(&history.RecordActivityTaskHeartbeatRequest{
		DomainUUID: common.StringPtr(domainID),
		HeartbeatRequest: &workflow.RecordActivityTaskHeartbeatRequest{
			TaskToken: activityTaskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)
	s.NotNil(hbResponse)
	s.True(*hbResponse.CancelRequested)

	// Try cancelling the request.
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err = s.mockHistoryEngine.RespondActivityTaskCanceled(&history.RespondActivityTaskCanceledRequest{
		DomainUUID: common.StringPtr(domainID),
		CancelRequest: &workflow.RespondActivityTaskCanceledRequest{
			TaskToken: activityTaskToken,
			Identity:  &identity,
			Details:   []byte("details"),
		},
	})
	s.Nil(err)

	executionBuilder = s.getBuilder(domainID, we)
	s.Equal(int64(13), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(8), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.True(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestStarTimer_DuplicateTimerID() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))

	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeStartTimer),
		StartTimerDecisionAttributes: &workflow.StartTimerDecisionAttributes{
			TimerId:                   common.StringPtr(timerID),
			StartToFireTimeoutSeconds: common.Int64Ptr(1),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)

	// Try to add the same timer ID again.
	decisionScheduledEvent2, _ := addDecisionTaskScheduledEvent(executionBuilder)
	addDecisionTaskStartedEvent(executionBuilder, *decisionScheduledEvent2.EventId, tl, identity)
	taskToken2, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: *decisionScheduledEvent2.EventId,
	})

	ms2 := createMutableState(executionBuilder)
	gwmsResponse2 := &persistence.GetWorkflowExecutionResponse{State: ms2}

	decisionFailedEvent := false
	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse2, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Run(func(arguments mock.Arguments) {
		req := arguments.Get(0).(*persistence.AppendHistoryEventsRequest)
		hs := persistence.NewJSONHistorySerializer()
		h, err := hs.Deserialize(req.Events)
		if err != nil {
			panic(err)
		}
		decTaskIndex := len(h.Events) - 2
		if decTaskIndex >= 0 && *h.Events[decTaskIndex].EventType == workflow.EventTypeDecisionTaskFailed {
			decisionFailedEvent = true
		}
	}).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err = s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken2,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	s.True(decisionFailedEvent)
	executionBuilder = s.getBuilder(domainID, we)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.True(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestUserTimer_RespondDecisionTaskCompleted() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 6,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	// Verify cancel timer with a start event.
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	decisionStartedEvent := addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)
	decisionCompletedEvent := addDecisionTaskCompletedEvent(msBuilder, *decisionScheduledEvent.EventId,
		*decisionStartedEvent.EventId, nil, identity)
	addTimerStartedEvent(msBuilder, *decisionCompletedEvent.EventId, timerID, 10)
	decision2ScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decision2ScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeCancelTimer),
		CancelTimerDecisionAttributes: &workflow.CancelTimerDecisionAttributes{
			TimerId: common.StringPtr(timerID),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(10), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(7), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) TestCancelTimer_RespondDecisionTaskCompleted_NoStartTimer() {
	domainID := "domainId"
	we := workflow.WorkflowExecution{
		WorkflowId: common.StringPtr("wId"),
		RunId:      common.StringPtr("rId"),
	}
	tl := "testTaskList"
	taskToken, _ := json.Marshal(&common.TaskToken{
		WorkflowID: *we.WorkflowId,
		RunID:      *we.RunId,
		ScheduleID: 2,
	})
	identity := "testIdentity"
	timerID := "t1"

	msBuilder := newMutableStateBuilder(s.config, bark.NewLoggerFromLogrus(log.New()))
	// Verify cancel timer with a start event.
	addWorkflowExecutionStartedEvent(msBuilder, we, "wType", tl, []byte("input"), 100, 200, identity)
	decisionScheduledEvent, _ := addDecisionTaskScheduledEvent(msBuilder)
	addDecisionTaskStartedEvent(msBuilder, *decisionScheduledEvent.EventId, tl, identity)

	ms := createMutableState(msBuilder)
	gwmsResponse := &persistence.GetWorkflowExecutionResponse{State: ms}

	decisions := []*workflow.Decision{{
		DecisionType: common.DecisionTypePtr(workflow.DecisionTypeCancelTimer),
		CancelTimerDecisionAttributes: &workflow.CancelTimerDecisionAttributes{
			TimerId: common.StringPtr(timerID),
		},
	}}

	s.mockExecutionMgr.On("GetWorkflowExecution", mock.Anything).Return(gwmsResponse, nil).Once()
	s.mockHistoryMgr.On("AppendHistoryEvents", mock.Anything).Return(nil).Once()
	s.mockExecutionMgr.On("UpdateWorkflowExecution", mock.Anything).Return(nil).Once()

	err := s.mockHistoryEngine.RespondDecisionTaskCompleted(&history.RespondDecisionTaskCompletedRequest{
		DomainUUID: common.StringPtr(domainID),
		CompleteRequest: &workflow.RespondDecisionTaskCompletedRequest{
			TaskToken:        taskToken,
			Decisions:        decisions,
			ExecutionContext: []byte("context"),
			Identity:         &identity,
		},
	})
	s.Nil(err)

	executionBuilder := s.getBuilder(domainID, we)
	s.Equal(int64(6), executionBuilder.executionInfo.NextEventID)
	s.Equal(int64(3), executionBuilder.executionInfo.LastProcessedEvent)
	s.Equal(persistence.WorkflowStateRunning, executionBuilder.executionInfo.State)
	s.False(executionBuilder.HasPendingDecisionTask())
}

func (s *engineSuite) getBuilder(domainID string, we workflow.WorkflowExecution) *mutableStateBuilder {
	context, release, err := s.mockHistoryEngine.historyCache.getOrCreateWorkflowExecution(domainID, we)
	if err != nil {
		return nil
	}
	defer release()

	return context.msBuilder
}

func (s *engineSuite) getActivityScheduledEvent(msBuilder *mutableStateBuilder,
	scheduleID int64) *workflow.HistoryEvent {

	ai, ok := msBuilder.GetActivityInfo(scheduleID)
	if !ok {
		return nil
	}

	event, err := s.eventSerializer.Deserialize(ai.ScheduledEvent)
	if err != nil {
		s.logger.Errorf("Error Deserializing Event: %v", err)
	}

	return event
}

func (s *engineSuite) getActivityStartedEvent(msBuilder *mutableStateBuilder,
	scheduleID int64) *workflow.HistoryEvent {

	ai, ok := msBuilder.GetActivityInfo(scheduleID)
	if !ok {
		return nil
	}

	event, err := s.eventSerializer.Deserialize(ai.StartedEvent)
	if err != nil {
		s.logger.Errorf("Error Deserializing Event: %v", err)
	}

	return event
}

func (s *engineSuite) printHistory(builder *mutableStateBuilder) string {
	history, err := builder.hBuilder.Serialize()
	if err != nil {
		s.logger.Errorf("Error serializing history: %v", err)
		return ""
	}

	//s.logger.Info(string(history))
	return history.String()
}

func addWorkflowExecutionStartedEvent(builder *mutableStateBuilder, workflowExecution workflow.WorkflowExecution,
	workflowType, taskList string, input []byte, executionStartToCloseTimeout, taskStartToCloseTimeout int32,
	identity string) *workflow.HistoryEvent {
	e := builder.AddWorkflowExecutionStartedEvent("domainId", workflowExecution, &workflow.StartWorkflowExecutionRequest{
		WorkflowId:   common.StringPtr(*workflowExecution.WorkflowId),
		WorkflowType: &workflow.WorkflowType{Name: common.StringPtr(workflowType)},
		TaskList:     &workflow.TaskList{Name: common.StringPtr(taskList)},
		Input:        input,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeout),
		Identity:                            common.StringPtr(identity),
	})

	return e
}

func addDecisionTaskScheduledEvent(builder *mutableStateBuilder) (*workflow.HistoryEvent, *decisionInfo) {
	return builder.AddDecisionTaskScheduledEvent()
}

func addDecisionTaskStartedEvent(builder *mutableStateBuilder, scheduleID int64, taskList,
	identity string) *workflow.HistoryEvent {
	return addDecisionTaskStartedEventWithRequestID(builder, scheduleID, uuid.New(), taskList, identity)
}

func addDecisionTaskStartedEventWithRequestID(builder *mutableStateBuilder, scheduleID int64, requestID string,
	taskList, identity string) *workflow.HistoryEvent {
	e := builder.AddDecisionTaskStartedEvent(scheduleID, requestID, &workflow.PollForDecisionTaskRequest{
		TaskList: &workflow.TaskList{Name: common.StringPtr(taskList)},
		Identity: common.StringPtr(identity),
	})

	return e
}

func addDecisionTaskCompletedEvent(builder *mutableStateBuilder, scheduleID, startedID int64, context []byte,
	identity string) *workflow.HistoryEvent {
	e := builder.AddDecisionTaskCompletedEvent(scheduleID, startedID, &workflow.RespondDecisionTaskCompletedRequest{
		ExecutionContext: context,
		Identity:         common.StringPtr(identity),
	})

	builder.FlushBufferedEvents()

	return e
}

func addActivityTaskScheduledEvent(builder *mutableStateBuilder, decisionCompletedID int64, activityID, activityType,
	taskList string, input []byte, timeout, queueTimeout, hearbeatTimeout int32) (*workflow.HistoryEvent,
	*persistence.ActivityInfo) {
	return builder.AddActivityTaskScheduledEvent(decisionCompletedID, &workflow.ScheduleActivityTaskDecisionAttributes{
		ActivityId:   common.StringPtr(activityID),
		ActivityType: &workflow.ActivityType{Name: common.StringPtr(activityType)},
		TaskList:     &workflow.TaskList{Name: common.StringPtr(taskList)},
		Input:        input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(timeout),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(queueTimeout),
		HeartbeatTimeoutSeconds:       common.Int32Ptr(hearbeatTimeout),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(1),
	})
}

func addActivityTaskStartedEvent(builder *mutableStateBuilder, scheduleID int64,
	taskList, identity string) *workflow.HistoryEvent {
	ai, _ := builder.GetActivityInfo(scheduleID)
	return builder.AddActivityTaskStartedEvent(ai, scheduleID, uuid.New(), &workflow.PollForActivityTaskRequest{
		TaskList: &workflow.TaskList{Name: common.StringPtr(taskList)},
		Identity: common.StringPtr(identity),
	})
}

func addActivityTaskCompletedEvent(builder *mutableStateBuilder, scheduleID, startedID int64, result []byte,
	identity string) *workflow.HistoryEvent {
	e := builder.AddActivityTaskCompletedEvent(scheduleID, startedID, &workflow.RespondActivityTaskCompletedRequest{
		Result:   result,
		Identity: common.StringPtr(identity),
	})

	return e
}

func addActivityTaskFailedEvent(builder *mutableStateBuilder, scheduleID, startedID int64, reason string, details []byte,
	identity string) *workflow.HistoryEvent {
	e := builder.AddActivityTaskFailedEvent(scheduleID, startedID, &workflow.RespondActivityTaskFailedRequest{
		Reason:   common.StringPtr(reason),
		Details:  details,
		Identity: common.StringPtr(identity),
	})

	return e
}

func addTimerStartedEvent(builder *mutableStateBuilder, decisionCompletedEventID int64, timerID string,
	timeOut int64) (*workflow.HistoryEvent, *persistence.TimerInfo) {
	return builder.AddTimerStartedEvent(decisionCompletedEventID,
		&workflow.StartTimerDecisionAttributes{
			TimerId:                   common.StringPtr(timerID),
			StartToFireTimeoutSeconds: common.Int64Ptr(timeOut),
		})
}

func addRequestCancelInitiatedEvent(builder *mutableStateBuilder, decisionCompletedEventID int64,
	cancelRequestID, domain, workflowID, runID string) *workflow.HistoryEvent {
	event, _ := builder.AddRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID,
		cancelRequestID, &workflow.RequestCancelExternalWorkflowExecutionDecisionAttributes{
			Domain:     common.StringPtr(domain),
			WorkflowId: common.StringPtr(workflowID),
			RunId:      common.StringPtr(runID),
		})

	return event
}

func addStartChildWorkflowExecutionInitiatedEvent(builder *mutableStateBuilder, decisionCompletedID int64,
	createRequestID, domain, workflowID, workflowType, tasklist string, input []byte,
	executionStartToCloseTimeout, taskStartToCloseTimeout int32) (*workflow.HistoryEvent,
	*persistence.ChildExecutionInfo) {
	return builder.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletedID, createRequestID,
		&workflow.StartChildWorkflowExecutionDecisionAttributes{
			Domain:       common.StringPtr(domain),
			WorkflowId:   common.StringPtr(workflowID),
			WorkflowType: &workflow.WorkflowType{Name: common.StringPtr(workflowType)},
			TaskList:     &workflow.TaskList{Name: common.StringPtr(tasklist)},
			Input:        input,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeout),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeout),
			ChildPolicy:                         common.ChildPolicyPtr(workflow.ChildPolicyTerminate),
			Control:                             nil,
		})
}

func addCompleteWorkflowEvent(builder *mutableStateBuilder, decisionCompletedEventID int64,
	result []byte) *workflow.HistoryEvent {
	e := builder.AddCompletedWorkflowEvent(decisionCompletedEventID, &workflow.CompleteWorkflowExecutionDecisionAttributes{
		Result: result,
	})

	return e
}

func createMutableState(builder *mutableStateBuilder) *persistence.WorkflowMutableState {
	info := copyWorkflowExecutionInfo(builder.executionInfo)
	activityInfos := make(map[int64]*persistence.ActivityInfo)
	for id, info := range builder.pendingActivityInfoIDs {
		activityInfos[id] = copyActivityInfo(info)
	}
	timerInfos := make(map[string]*persistence.TimerInfo)
	for id, info := range builder.pendingTimerInfoIDs {
		timerInfos[id] = copyTimerInfo(info)
	}
	builder.FlushBufferedEvents()
	var bufferedEvents []*persistence.SerializedHistoryEventBatch
	if len(builder.bufferedEvents) > 0 {
		bufferedEvents = append(bufferedEvents, builder.bufferedEvents...)
	}
	if builder.updateBufferedEvents != nil {
		bufferedEvents = append(bufferedEvents, builder.updateBufferedEvents)
	}

	return &persistence.WorkflowMutableState{
		ExecutionInfo:  info,
		ActivitInfos:   activityInfos,
		TimerInfos:     timerInfos,
		BufferedEvents: bufferedEvents,
	}
}

func copyWorkflowExecutionInfo(sourceInfo *persistence.WorkflowExecutionInfo) *persistence.WorkflowExecutionInfo {
	return &persistence.WorkflowExecutionInfo{
		DomainID:             sourceInfo.DomainID,
		WorkflowID:           sourceInfo.WorkflowID,
		RunID:                sourceInfo.RunID,
		ParentDomainID:       sourceInfo.ParentDomainID,
		ParentWorkflowID:     sourceInfo.ParentWorkflowID,
		ParentRunID:          sourceInfo.ParentRunID,
		InitiatedID:          sourceInfo.InitiatedID,
		CompletionEvent:      sourceInfo.CompletionEvent,
		TaskList:             sourceInfo.TaskList,
		WorkflowTypeName:     sourceInfo.WorkflowTypeName,
		WorkflowTimeout:      sourceInfo.WorkflowTimeout,
		DecisionTimeoutValue: sourceInfo.DecisionTimeoutValue,
		ExecutionContext:     sourceInfo.ExecutionContext,
		State:                sourceInfo.State,
		CloseStatus:          sourceInfo.CloseStatus,
		NextEventID:          sourceInfo.NextEventID,
		LastProcessedEvent:   sourceInfo.LastProcessedEvent,
		LastUpdatedTimestamp: sourceInfo.LastUpdatedTimestamp,
		CreateRequestID:      sourceInfo.CreateRequestID,
		DecisionScheduleID:   sourceInfo.DecisionScheduleID,
		DecisionStartedID:    sourceInfo.DecisionStartedID,
		DecisionRequestID:    sourceInfo.DecisionRequestID,
		DecisionTimeout:      sourceInfo.DecisionTimeout,
	}
}

func copyActivityInfo(sourceInfo *persistence.ActivityInfo) *persistence.ActivityInfo {
	return &persistence.ActivityInfo{
		ScheduleID:             sourceInfo.ScheduleID,
		ScheduledEvent:         sourceInfo.ScheduledEvent,
		StartedID:              sourceInfo.StartedID,
		StartedEvent:           sourceInfo.StartedEvent,
		ActivityID:             sourceInfo.ActivityID,
		RequestID:              sourceInfo.RequestID,
		Details:                sourceInfo.Details,
		ScheduleToStartTimeout: sourceInfo.ScheduleToStartTimeout,
		ScheduleToCloseTimeout: sourceInfo.ScheduleToCloseTimeout,
		StartToCloseTimeout:    sourceInfo.StartToCloseTimeout,
		HeartbeatTimeout:       sourceInfo.HeartbeatTimeout,
		CancelRequested:        sourceInfo.CancelRequested,
		CancelRequestID:        sourceInfo.CancelRequestID,
	}
}

func copyTimerInfo(sourceInfo *persistence.TimerInfo) *persistence.TimerInfo {
	return &persistence.TimerInfo{
		TimerID:    sourceInfo.TimerID,
		StartedID:  sourceInfo.StartedID,
		ExpiryTime: sourceInfo.ExpiryTime,
		TaskID:     sourceInfo.TaskID,
	}
}
