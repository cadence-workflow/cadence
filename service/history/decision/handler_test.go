// Copyright (c) 2019 Uber Technologies, Inc.
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

package decision

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/query"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/workflow"
)

const (
	testInvalidDomainUUID = "some-invalid-UUID"
	testShardID           = 1234
)

type (
	DecisionHandlerSuite struct {
		*require.Assertions
		suite.Suite

		controller       *gomock.Controller
		mockMutableState *execution.MockMutableState

		decisionHandler *handlerImpl
		queryRegistry   query.Registry
	}
)

func TestDecisionHandlerSuite(t *testing.T) {
	suite.Run(t, new(DecisionHandlerSuite))
}

func (s *DecisionHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())
	s.controller = gomock.NewController(s.T())
	s.decisionHandler = &handlerImpl{
		versionChecker: client.NewVersionChecker(),
		metricsClient:  metrics.NewClient(tally.NoopScope, metrics.History),
		config:         config.NewForTest(),
		logger:         testlogger.New(s.T()),
		timeSource:     clock.NewRealTimeSource(),
	}
	s.queryRegistry = s.constructQueryRegistry(10)
	s.mockMutableState = execution.NewMockMutableState(s.controller)
	workflowInfo := &persistence.WorkflowExecutionInfo{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(workflowInfo).AnyTimes()
}

func (s *DecisionHandlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *DecisionHandlerSuite) TestNewHandler() {
	shardContext := shard.NewMockContext(s.controller)
	tokenSerializer := common.NewMockTaskTokenSerializer(s.controller)
	shardContext.EXPECT().GetConfig().Times(1).Return(&config.Config{})
	shardContext.EXPECT().GetLogger().Times(2).Return(testlogger.New(s.T()))
	shardContext.EXPECT().GetTimeSource().Times(1)
	shardContext.EXPECT().GetDomainCache().Times(2)
	shardContext.EXPECT().GetMetricsClient().Times(2)
	shardContext.EXPECT().GetThrottledLogger().Times(1).Return(testlogger.New(s.T()))
	shardContext.EXPECT().GetActiveClusterManager().Times(1).Return(activecluster.NewMockManager(s.controller))
	h := NewHandler(shardContext, execution.NewMockCache(s.controller), tokenSerializer)
	s.NotNil(h)
	s.Equal("handlerImpl", reflect.ValueOf(h).Elem().Type().Name())
}

func TestHandleDecisionTaskScheduled(t *testing.T) {
	tests := []struct {
		name            string
		domainID        string
		mutablestate    *persistence.WorkflowMutableState
		isfirstDecision bool
		expectCalls     func(ctrl *gomock.Controller, shardContext *shard.MockContext)
		expectErr       bool
	}{
		{
			name:     "failure to retrieve domain From ID",
			domainID: testInvalidDomainUUID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
			expectErr: true,
		},
		{
			name:     "success",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
			expectCalls: func(ctrl *gomock.Controller, shardContext *shard.MockContext) {
				shardContext.EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: false,
		},
		{
			name:     "completed workflow",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					// WorkflowStateCompleted = 2 from persistence WorkflowExecutionInfo.IsRunning()
					State: 2,
				},
			},
			expectCalls: func(ctrl *gomock.Controller, shardContext *shard.MockContext) {
				shardContext.EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: true,
		},
		{
			name:     "get start event failure",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					// execution has no event yet
					DecisionScheduleID: -23,
					LastProcessedEvent: -23,
				},
			},
			expectCalls: func(ctrl *gomock.Controller, shardContext *shard.MockContext) {
				eventsCache := events.NewMockCache(ctrl)
				shardContext.EXPECT().GetEventsCache().Times(1).Return(eventsCache)
				eventsCache.EXPECT().
					GetEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, &persistence.TimeoutError{Msg: "failed to get start event: request timeout"})
				shardContext.EXPECT().GetShardID().Return(testShardID).Times(1)
			},
			expectErr: true,
		},
		{
			name:     "first decision task scheduled failure",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionScheduleID: -23,
					LastProcessedEvent: -23,
				},
				BufferedEvents: append([]*types.HistoryEvent{}, &types.HistoryEvent{}),
			},
			expectCalls: func(ctrl *gomock.Controller, shardContext *shard.MockContext) {
				eventsCache := events.NewMockCache(ctrl)
				shardContext.EXPECT().GetEventsCache().Times(1).Return(eventsCache)
				eventsCache.EXPECT().
					GetEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&types.HistoryEvent{}, nil)
				shardContext.EXPECT().GetShardID().Return(testShardID).Times(1)
				shardContext.EXPECT().GenerateTaskIDs(gomock.Any()).Times(1).Return([]int64{}, errors.New("some random error to avoid going too deep in call stack unrelated to this unit"))
			},
			expectErr:       true,
			isfirstDecision: true,
		},
		{
			name:     "first decision task scheduled success",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionScheduleID: -23,
					LastProcessedEvent: -23,
				},
			},
			expectCalls: func(ctrl *gomock.Controller, shardContext *shard.MockContext) {
				eventsCache := events.NewMockCache(ctrl)
				shardContext.EXPECT().GetEventsCache().Times(1).Return(eventsCache)
				eventsCache.EXPECT().
					GetEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&types.HistoryEvent{}, nil)
				shardContext.EXPECT().GetShardID().Return(testShardID).Times(1)
				shardContext.EXPECT().GenerateTaskIDs(gomock.Any()).Times(1).Return([]int64{}, errors.New("some random error to avoid going too deep in call stack unrelated to this unit"))
			},
			expectErr:       true,
			isfirstDecision: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			request := &types.ScheduleDecisionTaskRequest{
				DomainUUID: test.domainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				},
				IsFirstDecision: test.isfirstDecision,
			}
			decisionHandler := &handlerImpl{
				config:          config.NewForTest(),
				shard:           shard.NewMockContext(ctrl),
				timeSource:      clock.NewRealTimeSource(),
				metricsClient:   metrics.NewClient(tally.NoopScope, metrics.History),
				logger:          testlogger.New(t),
				versionChecker:  client.NewVersionChecker(),
				tokenSerializer: common.NewMockTaskTokenSerializer(ctrl),
				domainCache:     cache.NewMockDomainCache(ctrl),
			}
			expectCommonCalls(decisionHandler, test.domainID)
			expectGetWorkflowExecution(decisionHandler, test.domainID, test.mutablestate)
			expectDefaultDomainCache(decisionHandler, test.domainID)
			if test.expectCalls != nil {
				test.expectCalls(ctrl, decisionHandler.shard.(*shard.MockContext))
			}
			decisionHandler.executionCache = execution.NewCache(decisionHandler.shard)
			err := decisionHandler.HandleDecisionTaskScheduled(context.Background(), request)
			assert.Equal(t, test.expectErr, err != nil)
		})
	}
}

func TestHandleDecisionTaskFailed(t *testing.T) {
	taskToken := []byte("test-token")
	tests := []struct {
		name         string
		domainID     string
		mutablestate *persistence.WorkflowMutableState
		expectCalls  func(ctrl *gomock.Controller, h *handlerImpl)
		expectErr    bool
	}{
		{
			name:      " fail to retrieve domain From ID",
			domainID:  testInvalidDomainUUID,
			expectErr: true,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
		},
		{
			name:     "failure to deserialize token",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(taskToken).Return(nil, errors.New("unable to deserialize task token"))
			},
			expectErr: true,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
		},
		{
			name:     "success",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				token := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				h.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(taskToken).Return(token, nil)
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
				h.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(gomock.Any()).Return([]int64{0}, nil)
				h.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}).Return(&persistence.AppendHistoryNodesResponse{}, nil)
				h.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil)
				h.shard.(*shard.MockContext).EXPECT().GetShardID().Return(testShardID)
				engine := engine.NewMockEngine(ctrl)
				h.shard.(*shard.MockContext).EXPECT().GetEngine().Times(3).Return(engine)
				engine.EXPECT().NotifyNewHistoryEvent(gomock.Any())
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())
			},
			expectErr: false,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
		},
		{
			name:     "completed workflow",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					// WorkflowStateCompleted = 2 from persistence WorkflowExecutionInfo.IsRunning()
					State: 2,
				},
			},
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				token := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				h.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(taskToken).Return(token, nil)
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: true,
		},
		{
			name:     "decision task not found",
			domainID: constants.TestDomainID,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionScheduleID: 0,
				},
			},
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				token := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 1,
				}
				h.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(taskToken).Return(token, nil)
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			request := &types.HistoryRespondDecisionTaskFailedRequest{
				DomainUUID: test.domainID,
				FailedRequest: &types.RespondDecisionTaskFailedRequest{
					TaskToken: taskToken,
					Cause:     nil,
					Details:   nil,
				},
			}
			shardContext := shard.NewMockContext(ctrl)
			decisionHandler := &handlerImpl{
				config:          config.NewForTest(),
				shard:           shardContext,
				timeSource:      clock.NewRealTimeSource(),
				metricsClient:   metrics.NewClient(tally.NoopScope, metrics.History),
				logger:          testlogger.New(t),
				versionChecker:  client.NewVersionChecker(),
				tokenSerializer: common.NewMockTaskTokenSerializer(ctrl),
				domainCache:     cache.NewMockDomainCache(ctrl),
			}
			expectCommonCalls(decisionHandler, test.domainID)
			expectGetWorkflowExecution(decisionHandler, test.domainID, test.mutablestate)
			expectDefaultDomainCache(decisionHandler, test.domainID)
			decisionHandler.executionCache = execution.NewCache(shardContext)
			if test.expectCalls != nil {
				test.expectCalls(ctrl, decisionHandler)
			}

			err := decisionHandler.HandleDecisionTaskFailed(context.Background(), request)
			assert.Equal(t, test.expectErr, err != nil)
		})
	}
}

func TestHandleDecisionTaskStarted(t *testing.T) {
	testTaskListName := "some-tasklist-name"
	testWorkflowTypeName := "some-workflow-type-name"
	tests := []struct {
		name               string
		domainID           string
		mutablestate       *persistence.WorkflowMutableState
		expectCalls        func(ctrl *gomock.Controller, h *handlerImpl)
		expectErr          error
		assertResponseBody func(t *testing.T, response *types.RecordDecisionTaskStartedResponse)
	}{
		{
			name:      "fail to retrieve domain From ID",
			domainID:  testInvalidDomainUUID,
			expectErr: &types.BadRequestError{Message: "Invalid domain UUID."},
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
		},
		{
			name:     "failure - decision task already started",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: &types.EventAlreadyStartedError{Message: "Decision task already started."},
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{},
			},
		},
		{
			name:     "failure - workflow completed",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: workflow.ErrNotExists,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					State: 2, //2 == WorkflowStateCompleted
				},
			},
		},
		{
			name:     "failure - decision task already completed",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: &types.EntityNotExistsError{Message: "Decision task not found."},
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionScheduleID: 1,
					NextEventID:        2,
				},
			},
		},
		{
			name:     "failure - cached mutable state is stale",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				// handler will attempt reloading mutable state at most 5 times
				// this test will fail all retries
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(5).Return(events.NewMockCache(ctrl))
			},
			expectErr: workflow.ErrMaxAttemptsExceeded,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionScheduleID: 1,
				},
			},
		},
		{
			name:     "success",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			expectErr: nil,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					NextEventID:       3,
					DecisionRequestID: "test-request-id",
					DecisionAttempt:   1,
				},
			},
			assertResponseBody: func(t *testing.T, resp *types.RecordDecisionTaskStartedResponse) {
				// expect test.mutablestate.ExecutionInfo.DecisionAttempt
				assert.Equal(t, int64(1), resp.DecisionInfo.ScheduledEvent.DecisionTaskScheduledEventAttributes.Attempt)
			},
		},
		{
			name:     "success - decision startedID is empty",
			domainID: constants.TestDomainID,
			expectCalls: func(ctrl *gomock.Controller, h *handlerImpl) {
				h.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
				h.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(gomock.Any()).Times(1).Return([]int64{0}, nil)
				h.shard.(*shard.MockContext).EXPECT().
					AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID}).
					Return(&persistence.AppendHistoryNodesResponse{}, nil)
				h.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil)
				h.shard.(*shard.MockContext).EXPECT().GetShardID().Return(testShardID)
				engine := engine.NewMockEngine(ctrl)
				h.shard.(*shard.MockContext).EXPECT().GetEngine().Times(3).Return(engine)
				engine.EXPECT().NotifyNewHistoryEvent(gomock.Any())
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())
			},
			expectErr: nil,
			mutablestate: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionStartedID: -23,
					NextEventID:       2,
					WorkflowTypeName:  testWorkflowTypeName,
					TaskList:          testTaskListName,
				},
			},
			assertResponseBody: func(t *testing.T, resp *types.RecordDecisionTaskStartedResponse) {
				assert.Equal(t, testWorkflowTypeName, resp.WorkflowType.Name)
				assert.Equal(t, testTaskListName, resp.WorkflowExecutionTaskList.Name)
				assert.Equal(t, int64(0), resp.ScheduledEventID)
				assert.Equal(t, int64(3), resp.NextEventID)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			request := &types.RecordDecisionTaskStartedRequest{
				DomainUUID: test.domainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				},
				RequestID: "test-request-id",
				PollRequest: &types.PollForDecisionTaskRequest{
					Domain:   test.domainID,
					Identity: "test-identity",
				},
			}
			shardContext := shard.NewMockContext(ctrl)
			decisionHandler := &handlerImpl{
				config:         config.NewForTest(),
				shard:          shardContext,
				timeSource:     clock.NewRealTimeSource(),
				metricsClient:  metrics.NewClient(tally.NoopScope, metrics.History),
				logger:         testlogger.New(t),
				versionChecker: client.NewVersionChecker(),
				domainCache:    cache.NewMockDomainCache(ctrl),
			}
			expectCommonCalls(decisionHandler, test.domainID)
			expectGetWorkflowExecution(decisionHandler, test.domainID, test.mutablestate)
			expectDefaultDomainCache(decisionHandler, test.domainID)
			decisionHandler.executionCache = execution.NewCache(shardContext)
			if test.expectCalls != nil {
				test.expectCalls(ctrl, decisionHandler)
			}

			resp, err := decisionHandler.HandleDecisionTaskStarted(context.Background(), request)
			assert.Equal(t, test.expectErr, err)
			if err == nil {
				assert.NotNil(t, resp)
				assert.Equal(t, test.mutablestate.ExecutionInfo.DecisionScheduleID, resp.ScheduledEventID)
				assert.Equal(t, test.mutablestate.ExecutionInfo.DecisionStartedID, resp.StartedEventID)
				assert.Equal(t, test.mutablestate.ExecutionInfo.NextEventID, resp.NextEventID)
				assert.Equal(t, test.mutablestate.ExecutionInfo.TaskList, resp.WorkflowExecutionTaskList.Name)
				test.assertResponseBody(t, resp)
			}
		})
	}
}

func TestHandleDecisionTaskCompleted(t *testing.T) {
	serializedTestToken := []byte("test-token")
	testTaskListName := "some-tasklist-name"
	testWorkflowTypeName := "some-workflow-type-name"
	tests := []struct {
		name                        string
		domainID                    string
		expectedErr                 error
		expectMockCalls             func(ctrl *gomock.Controller, decisionHandler *handlerImpl)
		assertResponseBody          func(t *testing.T, resp *types.HistoryRespondDecisionTaskCompletedResponse)
		mutableState                *persistence.WorkflowMutableState
		request                     *types.HistoryRespondDecisionTaskCompletedRequest
		expectGetWorkflowExecution  bool
		expectNonDefaultDomainCache bool
	}{
		{
			name:        "failure to get domain from ID",
			domainID:    testInvalidDomainUUID,
			expectedErr: &types.BadRequestError{Message: "Invalid domain UUID."},
		},
		{
			name:        "token deserialazation failure",
			domainID:    constants.TestDomainID,
			expectedErr: workflow.ErrDeserializingToken,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(nil, errors.New("unable to deserialize task token"))
			},
		},
		{
			name:        "get or create wf execution failure",
			domainID:    constants.TestDomainID,
			expectedErr: &types.BadRequestError{Message: "Can't load workflow execution.  WorkflowId not set."},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				taskToken := &common.TaskToken{
					DomainID: constants.TestDomainID,
					// empty workflow ID to force decisionHandler.executionCache.GetOrCreateWorkflowExecution() failure
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(taskToken, nil)
			},
		},
		{
			name:                       "success",
			domainID:                   constants.TestDomainID,
			expectedErr:                nil,
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 0,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Serialize(&common.TaskToken{
					DomainID:     constants.TestDomainID,
					WorkflowID:   constants.TestWorkflowID,
					WorkflowType: testWorkflowTypeName,
					RunID:        constants.TestRunID,
					ScheduleID:   1,
					ActivityID:   "some-activity-id",
					ActivityType: "some-activity-name",
				}).Return(serializedTestToken, nil)

				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(eventsCache)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, int64(1), gomock.Any())
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(1).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(4).Return([]int64{0, 1, 2, 3}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(6).Return([]int64{0, 1, 2, 3, 4, 5}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}).Return(&persistence.AppendHistoryNodesResponse{}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(context.Background(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil)

				engine := engine.NewMockEngine(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEngine().Return(engine).Times(3)
				engine.EXPECT().NotifyNewHistoryEvent(events.NewNotification(constants.TestDomainID, &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					0, 5, 0, 1, 0, nil))
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())

				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Times(1).Return(constants.TestLocalDomainEntry, nil)
				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainID(constants.TestDomainName).Times(1).Return(constants.TestDomainID, nil)
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					WorkflowTimeout: 600,
					AutoResetPoints: &types.ResetPoints{
						Points: func() []*types.ResetPointInfo {
							if historyMaxResetPoints, ok := dynamicproperties.IntKeys[dynamicproperties.HistoryMaxAutoResetPoints]; ok {
								return make([]*types.ResetPointInfo, historyMaxResetPoints.DefaultValue)
							}
							return []*types.ResetPointInfo{}
						}(),
					},
					WorkflowTypeName: testWorkflowTypeName,
					TaskList:         testTaskListName,
				},
				Checksum:       checksum.Checksum{},
				BufferedEvents: append([]*types.HistoryEvent{}, &types.HistoryEvent{}),
				ActivityInfos:  make(map[int64]*persistence.ActivityInfo),
			},
			assertResponseBody: func(t *testing.T, resp *types.HistoryRespondDecisionTaskCompletedResponse) {
				assert.True(t, resp.StartedResponse.StickyExecutionEnabled)
				assert.Equal(t, 1, len(resp.ActivitiesToDispatchLocally))
				assert.Equal(t, testWorkflowTypeName, resp.StartedResponse.WorkflowType.Name)
				assert.Equal(t, int64(0), resp.StartedResponse.Attempt)
				assert.Equal(t, testTaskListName, resp.StartedResponse.WorkflowExecutionTaskList.Name)
			},
		},
		{
			name:                       "decision task failure",
			domainID:                   constants.TestDomainID,
			expectedErr:                &types.InternalServiceError{Message: "add-decisiontask-failed-event operation failed"},
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(2).Return(eventsCache)
				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomain(constants.TestDomainName).Times(1).Return(constants.TestLocalDomainEntry, nil)
			},
		},
		{
			name:                       "workflow completed",
			domainID:                   constants.TestDomainID,
			expectedErr:                workflow.ErrAlreadyCompleted,
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(eventsCache)
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					State: 2,
				},
			},
		},
		{
			name:                       "decision task not found",
			domainID:                   constants.TestDomainID,
			expectedErr:                &types.EntityNotExistsError{Message: "Decision task not found."},
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:        constants.TestDomainID,
					WorkflowID:      constants.TestWorkflowID,
					RunID:           constants.TestRunID,
					ScheduleAttempt: 1,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(eventsCache)
			},
		},
		{
			name:                       "decision heartbeat time out",
			domainID:                   constants.TestDomainID,
			expectedErr:                &types.EntityNotExistsError{Message: "decision heartbeat timeout"},
			expectGetWorkflowExecution: true,
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken:                  serializedTestToken,
					Decisions:                  []*types.Decision{},
					ReturnNewDecisionTask:      true,
					ForceCreateNewDecisionTask: true,
					StickyAttributes: &types.StickyExecutionAttributes{
						WorkerTaskList:                &types.TaskList{Name: testTaskListName},
						ScheduleToStartTimeoutSeconds: func(i int32) *int32 { return &i }(10),
					},
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 0,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)

				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(eventsCache)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(1).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(1).Return([]int64{0}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}).Return(&persistence.AppendHistoryNodesResponse{}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(context.Background(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil)

				engine := engine.NewMockEngine(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEngine().Return(engine).Times(3)
				engine.EXPECT().NotifyNewHistoryEvent(events.NewNotification(constants.TestDomainID, &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					0, 1, 0, 1, 0, nil))
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionOriginalScheduledTimestamp: 1,
				},
			},
		},
		{
			name:                       "update continueAsNew info failure - execution size limit exceeded",
			domainID:                   constants.TestDomainID,
			expectedErr:                execution.ErrWorkflowFinished,
			expectGetWorkflowExecution: true,
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: common.Ptr(types.DecisionTypeContinueAsNewWorkflowExecution),
						ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
							WorkflowType: &types.WorkflowType{Name: testWorkflowTypeName},
							TaskList:     &types.TaskList{Name: testTaskListName},
						},
					}},
					ReturnNewDecisionTask: true,
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 0,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(3).Return(eventsCache)
				eventsCache.EXPECT().GetEvent(context.Background(), testShardID, constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, commonconstants.FirstEventID, commonconstants.FirstEventID, nil).Return(&types.HistoryEvent{}, nil)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, gomock.Any(), int64(1), gomock.Any()).Times(2)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(1).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(2).Times(1).Return([]int64{0, 1}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(nil, &persistence.TransactionSizeLimitError{Msg: fmt.Sprintf("transaction size exceeds limit")})
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetExecutionManager().Times(1)
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DecisionOriginalScheduledTimestamp: 1,
					WorkflowTimeout:                    100,
				},
			},
		},
		{
			name:                       "update continueAsNew info failure - conflict error",
			domainID:                   constants.TestDomainID,
			expectedErr:                workflow.ErrAlreadyCompleted,
			expectGetWorkflowExecution: true,
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: common.Ptr(types.DecisionTypeCompleteWorkflowExecution),
						CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{Result: []byte{}},
					}},
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(3).Return(eventsCache)
				eventsCache.EXPECT().GetEvent(context.Background(), testShardID, constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, commonconstants.FirstEventID, commonconstants.FirstEventID, nil).
					Return(&types.HistoryEvent{
						WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
					}, nil).Times(3)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, gomock.Any(), int64(1), gomock.Any()).Times(2)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(3).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(2).Times(1).Return([]int64{0, 1}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(nil, execution.NewConflictError(new(testing.T), errors.New("some random conflict error")))
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetExecutionManager().Times(1)
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CronSchedule: "0 1 * * 1", //some random cron schedule
				},
			},
		},
		{
			name:        "update continueAsNew info failure - load execution",
			domainID:    constants.TestDomainID,
			expectedErr: errors.New("some error occurred when loading workflow execution"),
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: common.Ptr(types.DecisionTypeFailWorkflowExecution),
						FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
							Reason: func(reason string) *string { return &reason }("some reason to fail workflow execution"),
						},
					}},
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(2).Return(eventsCache)
				eventsCache.EXPECT().GetEvent(context.Background(), testShardID, constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, commonconstants.FirstEventID, commonconstants.FirstEventID, nil).
					Return(&types.HistoryEvent{
						WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
					}, nil).Times(3)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, gomock.Any(), int64(1), gomock.Any()).Times(2)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(3).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(2).Times(1).Return([]int64{0, 1}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(nil, &persistence.TransactionSizeLimitError{Msg: fmt.Sprintf("transaction size exceeds limit")})
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetExecutionManager().Times(1)
				firstGetWfExecutionCall := decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).
					Return(&persistence.GetWorkflowExecutionResponse{
						State: &persistence.WorkflowMutableState{
							ExecutionInfo: &persistence.WorkflowExecutionInfo{
								DomainID:     constants.TestDomainID,
								WorkflowID:   constants.TestWorkflowID,
								RunID:        constants.TestRunID,
								CronSchedule: "0 1 * * 1", //some random cron schedule
							},
							ExecutionStats: &persistence.ExecutionStats{},
						},
						MutableStateStats: &persistence.MutableStateStats{},
					}, nil)
				lastGetWfExecutionCall := decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).Return(nil, errors.New("some error occurred when loading workflow execution"))
				gomock.InOrder(firstGetWfExecutionCall, lastGetWfExecutionCall)
			},
		},
		{
			name:        "update continueAsNew info failure - update execution",
			domainID:    constants.TestDomainID,
			expectedErr: errors.New("some error updating workflow execution"),
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: common.Ptr(types.DecisionTypeFailWorkflowExecution),
						FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
							Reason: func(reason string) *string { return &reason }("some reason to fail workflow execution"),
						},
					}},
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(3).Return(eventsCache)
				eventsCache.EXPECT().GetEvent(context.Background(), testShardID, constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, commonconstants.FirstEventID, commonconstants.FirstEventID, nil).
					Return(&types.HistoryEvent{
						WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
					}, nil).Times(3)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, gomock.Any(), int64(1), gomock.Any()).Times(3)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(3).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(2).Times(2).Return([]int64{0, 1}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(1).Times(1).Return([]int64{0}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(nil, &persistence.TransactionSizeLimitError{Msg: fmt.Sprintf("transaction size exceeds limit")})
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(&persistence.AppendHistoryNodesResponse{}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetExecutionManager().Times(1)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).
					DoAndReturn(func(ctx interface{}, request interface{}) (*persistence.GetWorkflowExecutionResponse, error) {
						return &persistence.GetWorkflowExecutionResponse{
							State: &persistence.WorkflowMutableState{
								ExecutionInfo: &persistence.WorkflowExecutionInfo{
									DomainID:     constants.TestDomainID,
									WorkflowID:   constants.TestWorkflowID,
									RunID:        constants.TestRunID,
									CronSchedule: "0 1 * * 1", //some random cron schedule
								},
								ExecutionStats: &persistence.ExecutionStats{},
							},
							MutableStateStats: &persistence.MutableStateStats{},
						}, nil
					}).Times(2)
				decisionHandler.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(context.Background(), gomock.Any()).Return(nil, errors.New("some error updating workflow execution"))
				engine := engine.NewMockEngine(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEngine().Return(engine).Times(2)
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())

			},
		},
		{
			name:                        "bad binaries",
			domainID:                    constants.TestDomainID,
			expectedErr:                 errors.New("some error updating continue as new info"),
			expectNonDefaultDomainCache: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				eventsCache := events.NewMockCache(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(3).Return(eventsCache)
				eventsCache.EXPECT().PutEvent(constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID, int64(0), gomock.Any())
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(1).Return([]int64{0}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, gomock.Any()).Return(nil, errors.New("some error updating continue as new info"))
				domainEntry := cache.NewLocalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: constants.TestDomainID, Name: constants.TestDomainName},
					&persistence.DomainConfig{
						Retention:   1,
						BadBinaries: types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"test-binary-checksum": {Reason: "some reason"}}},
					},
					cluster.TestCurrentClusterName)
				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainByID(constants.TestDomainID).AnyTimes().Return(domainEntry, nil)

				decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).
					Return(&persistence.GetWorkflowExecutionResponse{
						State: &persistence.WorkflowMutableState{
							ExecutionInfo:  &persistence.WorkflowExecutionInfo{DomainID: constants.TestDomainID, WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID, DecisionScheduleID: 1},
							ExecutionStats: &persistence.ExecutionStats{},
						},
						MutableStateStats: &persistence.MutableStateStats{},
					}, nil).Times(1)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).
					Return(&persistence.GetWorkflowExecutionResponse{
						State: &persistence.WorkflowMutableState{
							ExecutionInfo:  &persistence.WorkflowExecutionInfo{DomainID: constants.TestDomainID, WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
							ExecutionStats: &persistence.ExecutionStats{},
						},
						MutableStateStats: &persistence.MutableStateStats{},
					}, nil).Times(1)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).
					Return(&persistence.GetWorkflowExecutionResponse{
						State: &persistence.WorkflowMutableState{
							ExecutionInfo:  &persistence.WorkflowExecutionInfo{DomainID: constants.TestDomainID, WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID, DecisionAttempt: 2},
							ExecutionStats: &persistence.ExecutionStats{},
						},
						MutableStateStats: &persistence.MutableStateStats{},
					}, nil).Times(1)
			},
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken:                  serializedTestToken,
					BinaryChecksum:             "test-binary-checksum",
					ForceCreateNewDecisionTask: true,
					ReturnNewDecisionTask:      true,
				},
			},
		},
		{
			name:        "failure to load workflow execution - shard closed",
			domainID:    constants.TestDomainID,
			expectedErr: errors.New("some random error"),
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), gomock.Any()).Times(1).Return(nil, errors.New("some random error"))
			},
		},
		{
			name:                        "failure to load workflow execution stats",
			domainID:                    constants.TestDomainID,
			expectedErr:                 errors.New("some random error"),
			expectGetWorkflowExecution:  true,
			expectNonDefaultDomainCache: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainByID(constants.TestDomainID).Times(2).Return(constants.TestLocalDomainEntry, nil)
				decisionHandler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainByID(constants.TestDomainID).Times(1).Return(nil, errors.New("some random error"))
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
		},
		{
			name:                       "task handler fail decisions",
			domainID:                   constants.TestDomainID,
			expectedErr:                &types.InternalServiceError{Message: "unable to change workflow state from 0 to 2, close status 3"},
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 0,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
			},
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: common.Ptr(types.DecisionTypeCancelWorkflowExecution),
						CancelWorkflowExecutionDecisionAttributes: &types.CancelWorkflowExecutionDecisionAttributes{Details: []byte{}},
					}},
				},
			},
		},
		{
			name:                       "Update_History_Loop max attempt exceeded",
			domainID:                   constants.TestDomainID,
			expectedErr:                workflow.ErrMaxAttemptsExceeded,
			expectGetWorkflowExecution: true,
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
					ScheduleID: 2,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(5).Return(events.NewMockCache(ctrl))
			},
		},
		{
			name:                       "success with decision heartbeat",
			domainID:                   constants.TestDomainID,
			expectedErr:                nil,
			expectGetWorkflowExecution: true,
			request: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: constants.TestDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken:                  serializedTestToken,
					Decisions:                  []*types.Decision{},
					ReturnNewDecisionTask:      true,
					ForceCreateNewDecisionTask: true,
					StickyAttributes: &types.StickyExecutionAttributes{
						WorkerTaskList: &types.TaskList{Name: testTaskListName},
					},
				},
			},
			expectMockCalls: func(ctrl *gomock.Controller, decisionHandler *handlerImpl) {
				deserializedTestToken := &common.TaskToken{
					DomainID:     constants.TestDomainID,
					WorkflowID:   constants.TestWorkflowID,
					RunID:        constants.TestRunID,
					WorkflowType: testWorkflowTypeName,
				}
				decisionHandler.tokenSerializer.(*common.MockTaskTokenSerializer).EXPECT().Deserialize(serializedTestToken).Return(deserializedTestToken, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEventsCache().Times(1).Return(events.NewMockCache(ctrl))
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetShardID().Times(1).Return(testShardID)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GenerateTaskIDs(3).Return([]int64{0, 1, 2}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), constants.TestDomainID, types.WorkflowExecution{
					WorkflowID: constants.TestWorkflowID,
					RunID:      constants.TestRunID,
				}).Return(&persistence.AppendHistoryNodesResponse{}, nil)
				decisionHandler.shard.(*shard.MockContext).EXPECT().UpdateWorkflowExecution(context.Background(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{}, nil)
				engine := engine.NewMockEngine(ctrl)
				decisionHandler.shard.(*shard.MockContext).EXPECT().GetEngine().Return(engine).Times(3)
				engine.EXPECT().NotifyNewHistoryEvent(events.NewNotification(constants.TestDomainID, &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					0, 3, 0, 1, 0, nil))
				engine.EXPECT().NotifyNewTransferTasks(gomock.Any())
				engine.EXPECT().NotifyNewTimerTasks(gomock.Any())
				engine.EXPECT().NotifyNewReplicationTasks(gomock.Any())
			},
			assertResponseBody: func(t *testing.T, resp *types.HistoryRespondDecisionTaskCompletedResponse) {
				assert.True(t, resp.StartedResponse.StickyExecutionEnabled)
				assert.Equal(t, testWorkflowTypeName, resp.StartedResponse.WorkflowType.Name)
				assert.Equal(t, int64(0), resp.StartedResponse.Attempt)
				assert.Equal(t, testTaskListName, resp.StartedResponse.WorkflowExecutionTaskList.Name)
			},
			mutableState: &persistence.WorkflowMutableState{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					WorkflowTypeName: testWorkflowTypeName,
					TaskList:         testTaskListName,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			shard := shard.NewMockContext(ctrl)
			domainCache := cache.NewMockDomainCache(ctrl)
			handlerConfig := config.NewForTest()
			handlerConfig.MaxActivityCountDispatchByDomain = func(domain string) int { return 1 } // some value > 0
			handlerConfig.EnableActivityLocalDispatchByDomain = func(domain string) bool { return true }
			handlerConfig.DecisionRetryMaxAttempts = func(domain string) int { return 1 }
			decisionHandler := &handlerImpl{
				config:          handlerConfig,
				shard:           shard,
				timeSource:      clock.NewMockedTimeSource(),
				domainCache:     domainCache,
				metricsClient:   metrics.NewClient(tally.NoopScope, metrics.History),
				logger:          testlogger.New(t),
				versionChecker:  client.NewVersionChecker(),
				tokenSerializer: common.NewMockTaskTokenSerializer(ctrl),
				attrValidator:   newAttrValidator(domainCache, metrics.NewClient(tally.NoopScope, metrics.History), config.NewForTest(), testlogger.New(t)),
			}
			expectCommonCalls(decisionHandler, test.domainID)
			if test.expectGetWorkflowExecution {
				expectGetWorkflowExecution(decisionHandler, test.domainID, test.mutableState)
			}
			if !test.expectNonDefaultDomainCache {
				expectDefaultDomainCache(decisionHandler, test.domainID)
			}
			decisionHandler.executionCache = execution.NewCache(shard)

			request := &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: test.domainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: serializedTestToken,
					Decisions: []*types.Decision{{
						DecisionType: nil,
						ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
							ActivityID:                    "some-activity-id",
							ActivityType:                  &types.ActivityType{Name: "some-activity-name"},
							Domain:                        constants.TestDomainName,
							TaskList:                      &types.TaskList{Name: testTaskListName},
							ScheduleToCloseTimeoutSeconds: func(i int32) *int32 { return &i }(200),
							ScheduleToStartTimeoutSeconds: func(i int32) *int32 { return &i }(100),
							StartToCloseTimeoutSeconds:    func(i int32) *int32 { return &i }(100),
							RequestLocalDispatch:          true,
						},
					}},
					ReturnNewDecisionTask: true,
				},
			}
			if test.expectMockCalls != nil {
				test.expectMockCalls(ctrl, decisionHandler)
			}
			if test.request != nil {
				request = test.request
			}
			resp, err := decisionHandler.HandleDecisionTaskCompleted(context.Background(), request)
			assert.Equal(t, test.expectedErr, err)
			if err != nil {
				assert.Nil(t, resp)
			} else {
				test.assertResponseBody(t, resp)
			}
		})
	}
}

func (s *DecisionHandlerSuite) TestCreateRecordDecisionTaskStartedResponse() {
	tests := []struct {
		name        string
		expectCalls func()
		expectedErr error
		indexes     []string
	}{
		{
			name: "success",
			expectCalls: func() {
				s.mockMutableState.EXPECT().GetWorkflowType().Return(&types.WorkflowType{})
				s.mockMutableState.EXPECT().GetNextEventID().Return(int64(1))
				s.mockMutableState.EXPECT().CreateTransientDecisionEvents(gomock.Any(), "test-identity").Return(&types.HistoryEvent{}, &types.HistoryEvent{})
				s.mockMutableState.EXPECT().GetCurrentBranchToken().Return([]byte{}, nil)
				registry := query.NewMockRegistry(s.controller)
				s.mockMutableState.EXPECT().GetQueryRegistry().Return(registry)
				registry.EXPECT().GetBufferedIDs().Return([]string{"test-id", "test-id1", "test-id2"})
				registry.EXPECT().GetQueryInput(gomock.Any()).Return(&types.WorkflowQuery{}, nil).Times(2)
				registry.EXPECT().GetQueryInput(gomock.Any()).Return(nil, &types.InternalServiceError{Message: "query does not exist"})
				s.mockMutableState.EXPECT().GetHistorySize()
			},
			expectedErr: nil,
			indexes:     []string{"test-id", "test-id1"},
		},
		{
			name: "failure",
			expectCalls: func() {
				s.mockMutableState.EXPECT().GetWorkflowType().Return(&types.WorkflowType{})
				s.mockMutableState.EXPECT().GetNextEventID().Return(int64(1))
				s.mockMutableState.EXPECT().CreateTransientDecisionEvents(gomock.Any(), "test-identity").Return(&types.HistoryEvent{}, &types.HistoryEvent{})
				s.mockMutableState.EXPECT().GetCurrentBranchToken().Return([]byte{}, &types.BadRequestError{Message: fmt.Sprintf("getting branch index: %d, available branch count: %d", 0, 0)})
			},
			expectedErr: &types.BadRequestError{Message: fmt.Sprintf("getting branch index: %d, available branch count: %d", 0, 0)},
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			decision := &execution.DecisionInfo{
				ScheduleID: 1,
				StartedID:  2,
				RequestID:  constants.TestRequestID,
				TaskList:   "test-tasklist",
				Attempt:    1,
			}
			test.expectCalls()
			resp, err := s.decisionHandler.createRecordDecisionTaskStartedResponse(constants.TestDomainID, s.mockMutableState, decision, "test-identity")
			s.Equal(test.expectedErr, err)
			if err != nil {
				s.Nil(resp)
			} else {
				s.Equal(&types.HistoryEvent{}, resp.DecisionInfo.ScheduledEvent)
				s.Equal(&types.HistoryEvent{}, resp.DecisionInfo.StartedEvent)
				s.Equal([]byte{}, resp.BranchToken)
				for _, index := range test.indexes {
					_, ok := resp.Queries[index]
					s.True(ok)
				}
			}
		})
	}
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_ClientNotSupports() {
	s.mockMutableState.EXPECT().GetQueryRegistry().Return(s.queryRegistry)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
	s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, "0.0.0", nil, false, constants.TestGlobalDomainEntry, false)
	s.assertQueryCounts(s.queryRegistry, 0, 0, 0, 10)
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_HeartbeatDecision() {
	s.mockMutableState.EXPECT().GetQueryRegistry().Return(s.queryRegistry)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
	queryResults := s.constructQueryResults(s.queryRegistry.GetBufferedIDs()[0:5], 10)
	s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, client.GoWorkerConsistentQueryVersion, queryResults, false, constants.TestGlobalDomainEntry, true)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_NewDecisionTask() {
	s.mockMutableState.EXPECT().GetQueryRegistry().Return(s.queryRegistry)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
	queryResults := s.constructQueryResults(s.queryRegistry.GetBufferedIDs()[0:5], 10)
	s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, client.GoWorkerConsistentQueryVersion, queryResults, true, constants.TestGlobalDomainEntry, false)
	s.assertQueryCounts(s.queryRegistry, 5, 5, 0, 0)
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_NoNewDecisionTask() {
	s.mockMutableState.EXPECT().GetQueryRegistry().Return(s.queryRegistry)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
	queryResults := s.constructQueryResults(s.queryRegistry.GetBufferedIDs()[0:5], 10)
	s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, client.GoWorkerConsistentQueryVersion, queryResults, false, constants.TestGlobalDomainEntry, false)
	s.assertQueryCounts(s.queryRegistry, 0, 5, 5, 0)
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_QueryTooLarge() {
	s.mockMutableState.EXPECT().GetQueryRegistry().Return(s.queryRegistry)
	s.assertQueryCounts(s.queryRegistry, 10, 0, 0, 0)
	bufferedIDs := s.queryRegistry.GetBufferedIDs()
	queryResults := s.constructQueryResults(bufferedIDs[0:5], 10)
	largeQueryResults := s.constructQueryResults(bufferedIDs[5:10], 10*1024*1024)
	for k, v := range largeQueryResults {
		queryResults[k] = v
	}
	s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, client.GoWorkerConsistentQueryVersion, queryResults, false, constants.TestGlobalDomainEntry, false)
	s.assertQueryCounts(s.queryRegistry, 0, 5, 0, 5)
}

func (s *DecisionHandlerSuite) TestHandleBufferedQueries_QueryRegistryFailures() {
	tests := []struct {
		name                 string
		expectMockCalls      func()
		assertCalls          func(logs *observer.ObservedLogs)
		clientFeatureVersion string
		queryResults         map[string]*types.WorkflowQueryResult
	}{
		{
			name: "no buffered queries",
			expectMockCalls: func() {
				queryRegistry := query.NewMockRegistry(s.controller)
				s.mockMutableState.EXPECT().GetQueryRegistry().Return(queryRegistry)
				queryRegistry.EXPECT().HasBufferedQuery().Return(false)
			},
			assertCalls: func(logs *observer.ObservedLogs) {},
		},
		{
			name: "set query termination state failed - client unsupported",
			expectMockCalls: func() {
				queryRegistry := query.NewMockRegistry(s.controller)
				s.mockMutableState.EXPECT().GetQueryRegistry().Return(queryRegistry)
				queryRegistry.EXPECT().HasBufferedQuery().Return(true)
				queryRegistry.EXPECT().GetBufferedIDs().Return([]string{"some-buffered-id"})
				queryRegistry.EXPECT().SetTerminationState("some-buffered-id", gomock.Any()).Return(&types.InternalServiceError{Message: "query does not exist"})
			},
			assertCalls: func(logs *observer.ObservedLogs) {
				s.Equal(1, logs.FilterMessage("failed to set query termination state to failed").Len())
			},
			clientFeatureVersion: "0.0.0",
		},
		{
			name: "set query termination state failed - query too large",
			expectMockCalls: func() {
				queryRegistry := query.NewMockRegistry(s.controller)
				s.mockMutableState.EXPECT().GetQueryRegistry().Return(queryRegistry)
				queryRegistry.EXPECT().HasBufferedQuery().Return(true)
				queryRegistry.EXPECT().GetBufferedIDs().Return([]string{"some-id"})
				queryRegistry.EXPECT().SetTerminationState("some-id", gomock.Any()).Return(&types.InternalServiceError{Message: "query already in terminal state"}).Times(2)
			},
			queryResults:         s.constructQueryResults([]string{"some-id"}, 10*1024*1024),
			clientFeatureVersion: client.GoWorkerConsistentQueryVersion,
			assertCalls: func(logs *observer.ObservedLogs) {
				s.Equal(1, logs.FilterMessage("failed to set query termination state to failed").Len())
				s.Equal(1, logs.FilterMessage("failed to set query termination state to unblocked").Len())
			},
		},
		{
			name: "set query termination state unblocked",
			expectMockCalls: func() {
				queryRegistry := query.NewMockRegistry(s.controller)
				s.mockMutableState.EXPECT().GetQueryRegistry().Return(queryRegistry)
				queryRegistry.EXPECT().HasBufferedQuery().Return(true)
				queryRegistry.EXPECT().GetBufferedIDs().Return([]string{"some-buffered-id"})
				queryRegistry.EXPECT().SetTerminationState("some-id", gomock.Any()).Return(&types.InternalServiceError{Message: "query does not exist"})
				queryRegistry.EXPECT().SetTerminationState("some-buffered-id", gomock.Any()).Return(&types.InternalServiceError{Message: "query already in terminal state"})
			},
			clientFeatureVersion: client.GoWorkerConsistentQueryVersion,
			queryResults:         map[string]*types.WorkflowQueryResult{"some-id": &types.WorkflowQueryResult{}},
			assertCalls: func(logs *observer.ObservedLogs) {
				s.Equal(1, logs.FilterMessage("failed to set query termination state to completed").Len())
				s.Equal(1, logs.FilterMessage("failed to set query termination state to unblocked").Len())
			},
		},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			core, observedLogs := observer.New(zap.ErrorLevel)
			logger := zap.New(core)
			s.decisionHandler.logger = log.NewLogger(logger, log.WithSampleFunc(func(int) bool { return true }))

			test.expectMockCalls()
			s.decisionHandler.handleBufferedQueries(s.mockMutableState, client.GoSDK, test.clientFeatureVersion, test.queryResults, false, constants.TestGlobalDomainEntry, false)
			test.assertCalls(observedLogs)
		})
	}
}

func (s *DecisionHandlerSuite) constructQueryResults(ids []string, resultSize int) map[string]*types.WorkflowQueryResult {
	results := make(map[string]*types.WorkflowQueryResult)
	for _, id := range ids {
		results[id] = &types.WorkflowQueryResult{
			ResultType: types.QueryResultTypeAnswered.Ptr(),
			Answer:     make([]byte, resultSize),
		}
	}
	return results
}

func (s *DecisionHandlerSuite) constructQueryRegistry(numQueries int) query.Registry {
	queryRegistry := query.NewRegistry()
	for i := 0; i < numQueries; i++ {
		queryRegistry.BufferQuery(&types.WorkflowQuery{})
	}
	return queryRegistry
}

func (s *DecisionHandlerSuite) assertQueryCounts(queryRegistry query.Registry, buffered, completed, unblocked, failed int) {
	s.Len(queryRegistry.GetBufferedIDs(), buffered)
	s.Len(queryRegistry.GetCompletedIDs(), completed)
	s.Len(queryRegistry.GetUnblockedIDs(), unblocked)
	s.Len(queryRegistry.GetFailedIDs(), failed)
}

func expectCommonCalls(handler *handlerImpl, domainID string) {
	handler.shard.(*shard.MockContext).EXPECT().GetConfig().AnyTimes().Return(handler.config)
	handler.shard.(*shard.MockContext).EXPECT().GetLogger().AnyTimes().Return(handler.logger)
	handler.shard.(*shard.MockContext).EXPECT().GetTimeSource().AnyTimes().Return(handler.timeSource)
	handler.shard.(*shard.MockContext).EXPECT().GetDomainCache().AnyTimes().Return(handler.domainCache)
	handler.shard.(*shard.MockContext).EXPECT().GetClusterMetadata().AnyTimes().Return(constants.TestClusterMetadata)
	handler.shard.(*shard.MockContext).EXPECT().GetMetricsClient().AnyTimes().Return(handler.metricsClient)
	handler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainName(domainID).AnyTimes().Return(constants.TestDomainName, nil)
	handler.shard.(*shard.MockContext).EXPECT().GetExecutionManager().Times(1)
	handler.shard.(*shard.MockContext).EXPECT().GetShardID().Return(testShardID).Times(1)
}

func expectGetWorkflowExecution(handler *handlerImpl, domainID string, state *persistence.WorkflowMutableState) {
	workflowExecutionResponse := &persistence.GetWorkflowExecutionResponse{
		State:             state,
		MutableStateStats: &persistence.MutableStateStats{},
	}
	if state == nil {
		workflowExecutionResponse.State = &persistence.WorkflowMutableState{ExecutionInfo: &persistence.WorkflowExecutionInfo{}}
	}
	workflowExecutionResponse.State.ExecutionStats = &persistence.ExecutionStats{}
	workflowExecutionResponse.State.ExecutionInfo.DomainID = domainID
	workflowExecutionResponse.State.ExecutionInfo.WorkflowID = constants.TestWorkflowID
	workflowExecutionResponse.State.ExecutionInfo.RunID = constants.TestRunID

	handler.shard.(*shard.MockContext).EXPECT().GetWorkflowExecution(context.Background(), &persistence.GetWorkflowExecutionRequest{
		DomainID:   domainID,
		DomainName: constants.TestDomainName,
		Execution: types.WorkflowExecution{
			WorkflowID: constants.TestWorkflowID,
			RunID:      constants.TestRunID,
		},
	}).AnyTimes().Return(workflowExecutionResponse, nil)
}

func expectDefaultDomainCache(handler *handlerImpl, domainID string) {
	handler.domainCache.(*cache.MockDomainCache).EXPECT().GetDomainByID(domainID).AnyTimes().Return(constants.TestLocalDomainEntry, nil)
}
