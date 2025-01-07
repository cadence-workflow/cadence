// Copyright (c) 2021 Uber Technologies, Inc.
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

package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/quotas/global/algorithm"
	"github.com/uber/cadence/common/quotas/global/rpc"
	"github.com/uber/cadence/common/quotas/global/shared"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/failover"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/workflowcache"
)

const (
	testWorkflowID    = "test-workflow-id"
	testWorkflowRunID = "test-workflow-run-id"
	testDomainID      = "BF80C53A-ED56-4DD9-84EB-BE9AD4E45867"
	testValidUUID     = "FCD00931-EBD4-4028-B67E-4DE624641255"
)

type (
	handlerSuite struct {
		suite.Suite
		*require.Assertions

		controller               *gomock.Controller
		mockResource             *resource.Test
		mockShardController      *shard.MockController
		mockEngine               *engine.MockEngine
		mockWFCache              *workflowcache.MockWFCache
		mockTokenSerializer      *common.MockTaskTokenSerializer
		mockHistoryEventNotifier *events.MockNotifier
		mockRatelimiter          *quotas.MockLimiter
		mockFailoverCoordinator  *failover.MockCoordinator

		handler *handlerImpl
	}
)

func TestHandlerSuite(t *testing.T) {
	s := new(handlerSuite)
	suite.Run(t, s)
}

func (s *handlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.History)
	s.mockResource.Logger = testlogger.New(s.Suite.T())
	s.mockShardController = shard.NewMockController(s.controller)
	s.mockEngine = engine.NewMockEngine(s.controller)
	s.mockWFCache = workflowcache.NewMockWFCache(s.controller)
	s.mockFailoverCoordinator = failover.NewMockCoordinator(s.controller)
	internalRequestRateLimitingEnabledConfig := func(domainName string) bool { return false }
	s.handler = NewHandler(s.mockResource, config.NewForTest(), s.mockWFCache, internalRequestRateLimitingEnabledConfig).(*handlerImpl)
	s.handler.controller = s.mockShardController
	s.mockTokenSerializer = common.NewMockTaskTokenSerializer(s.controller)
	s.mockRatelimiter = quotas.NewMockLimiter(s.controller)
	s.handler.rateLimiter = s.mockRatelimiter
	s.handler.tokenSerializer = s.mockTokenSerializer
	s.handler.startWG.Done()
}

func (s *handlerSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *handlerSuite) TestHealth() {
	hs, err := s.handler.Health(context.Background())
	s.NoError(err)
	s.Equal(&types.HealthStatus{Ok: true, Msg: "OK"}, hs)
}

func (s *handlerSuite) TestRecordActivityTaskHeartbeat() {
	testInput := map[string]struct {
		caseName      string
		input         *types.HistoryRecordActivityTaskHeartbeatRequest
		expected      *types.RecordActivityTaskHeartbeatResponse
		expectedError bool
	}{
		"valid input": {
			caseName: "valid input",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
				HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
					TaskToken: []byte("task-token"),
				},
			},
			expected:      &types.RecordActivityTaskHeartbeatResponse{CancelRequested: false},
			expectedError: false,
		},
		"empty domainID": {
			caseName: "empty domainID",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: "",
			},
			expected:      nil,
			expectedError: true,
		},
		"ratelimit exceeded": {
			caseName: "ratelimit exceeded",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
			},
			expected:      nil,
			expectedError: true,
		},
		"token deserialization error": {
			caseName: "token deserialization error",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
				HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
					TaskToken: []byte("task-token"),
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"invalid task token": {
			caseName: "invalid task token",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
				HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
					TaskToken: []byte("task-token"),
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"get engine error": {
			caseName: "get engine error",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
				HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
					TaskToken: []byte("task-token"),
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"engine error": {
			caseName: "engine error",
			input: &types.HistoryRecordActivityTaskHeartbeatRequest{
				DomainUUID: testDomainID,
				HeartbeatRequest: &types.RecordActivityTaskHeartbeatRequest{
					TaskToken: []byte("task-token"),
				},
			},
			expected:      nil,
			expectedError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			switch input.caseName {
			case "valid input":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), input.input).Return(input.expected, nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "empty domainID":
			case "ratelimit exceeded":
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			case "token deserialization error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			case "invalid task token":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			case "get engine error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			case "engine error":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), input.input).Return(nil, errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			}
			response, err := s.handler.RecordActivityTaskHeartbeat(context.Background(), input.input)
			s.Equal(input.expected, response)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRecordActivityTaskStarted() {
	testInput := map[string]struct {
		caseName      string
		input         *types.RecordActivityTaskStartedRequest
		expected      *types.RecordActivityTaskStartedResponse
		expectedError bool
	}{
		"valid input": {
			caseName: "valid input",
			input: &types.RecordActivityTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      &types.RecordActivityTaskStartedResponse{Attempt: 1},
			expectedError: false,
		},
		"empty domainID": {
			caseName: "empty domainID",
			input: &types.RecordActivityTaskStartedRequest{
				DomainUUID: "",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"ratelimit exceeded": {
			caseName: "ratelimit exceeded",
			input: &types.RecordActivityTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"get engine error": {
			caseName: "get engine error",
			input: &types.RecordActivityTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"engine error": {
			caseName: "engine error",
			input: &types.RecordActivityTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			switch input.caseName {
			case "valid input":
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordActivityTaskStarted(gomock.Any(), input.input).Return(input.expected, nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "empty domainID":
			case "ratelimit exceeded":
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			case "get engine error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			case "engine error":
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordActivityTaskStarted(gomock.Any(), input.input).Return(nil, errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			}
			response, err := s.handler.RecordActivityTaskStarted(context.Background(), input.input)
			s.Equal(input.expected, response)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRecordDecisionTaskStarted() {
	testInput := map[string]struct {
		input         *types.RecordDecisionTaskStartedRequest
		expected      *types.RecordDecisionTaskStartedResponse
		expectedError bool
	}{
		"valid input": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
				},
			},
			expected: &types.RecordDecisionTaskStartedResponse{
				WorkflowType: &types.WorkflowType{
					Name: "test-workflow-type",
				},
				Attempt: 1,
			},
			expectedError: false,
		},
		"empty domainID": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: "",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"ratelimit exceeded": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"get engine error": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"engine error": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"engine error with ShardOwnershipLost": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
				PollRequest: &types.PollForDecisionTaskRequest{
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
				},
			},
			expected:      nil,
			expectedError: true,
		},
		"empty poll request": {
			input: &types.RecordDecisionTaskStartedRequest{
				DomainUUID: testDomainID,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				},
			},
			expected:      nil,
			expectedError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			switch name {
			case "valid input":
				s.mockShardController.EXPECT().GetEngine(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordDecisionTaskStarted(gomock.Any(), input.input).Return(input.expected, nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "empty domainID":
			case "ratelimit exceeded":
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			case "get engine error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			case "engine error":
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordDecisionTaskStarted(gomock.Any(), input.input).Return(nil, errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "engine error with ShardOwnershipLost":
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().RecordDecisionTaskStarted(gomock.Any(), input.input).Return(nil, &persistence.ShardOwnershipLostError{ShardID: 123}).Times(1)
				s.mockResource.MembershipResolver.EXPECT().Lookup(service.History, string(rune(123)))
			case "empty poll request":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			}

			response, err := s.handler.RecordDecisionTaskStarted(context.Background(), input.input)
			s.Equal(input.expected, response)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRespondActivityTaskCompleted() {
	testInput := map[string]struct {
		caseName      string
		input         *types.HistoryRespondActivityTaskCompletedRequest
		expectedError bool
	}{
		"valid input": {
			caseName: "valid input",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: false,
		},
		"empty domainID": {
			caseName: "empty domainID",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: "",
			},
			expectedError: true,
		},
		"ratelimit exceeded": {
			caseName: "ratelimit exceeded",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"token deserialization error": {
			caseName: "token deserialization error",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"invalid task token": {
			caseName: "invalid task token",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"get engine error": {
			caseName: "get engine error",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"engine error": {
			caseName: "engine error",
			input: &types.HistoryRespondActivityTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondActivityTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Result:    []byte("result"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			switch input.caseName {
			case "valid input":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskCompleted(gomock.Any(), input.input).Return(nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "empty domainID":
			case "ratelimit exceeded":
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			case "token deserialization error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			case "invalid task token":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			case "get engine error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			case "engine error":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskCompleted(gomock.Any(), input.input).Return(errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			}
			err := s.handler.RespondActivityTaskCompleted(context.Background(), input.input)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRespondActivityTaskFailed() {
	testInput := map[string]struct {
		caseName      string
		input         *types.HistoryRespondActivityTaskFailedRequest
		expectedError bool
	}{
		"valid input": {
			caseName: "valid input",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: false,
		},
		"empty domainID": {
			caseName: "empty domainID",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: "",
			},
			expectedError: true,
		},
		"ratelimit exceeded": {
			caseName: "ratelimit exceeded",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"token deserialization error": {
			caseName: "token deserialization error",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"invalid task token": {
			caseName: "invalid task token",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"get engine error": {
			caseName: "get engine error",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
		"engine error": {
			caseName: "engine error",
			input: &types.HistoryRespondActivityTaskFailedRequest{
				DomainUUID: testDomainID,
				FailedRequest: &types.RespondActivityTaskFailedRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			switch input.caseName {
			case "valid input":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskFailed(gomock.Any(), input.input).Return(nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			case "empty domainID":
			case "ratelimit exceeded":
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			case "token deserialization error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			case "invalid task token":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			case "get engine error":
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			case "engine error":
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskFailed(gomock.Any(), input.input).Return(errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			}
			err := s.handler.RespondActivityTaskFailed(context.Background(), input.input)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRespondActivityTaskCanceled() {
	validInput := &types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: testDomainID,
		CancelRequest: &types.RespondActivityTaskCanceledRequest{
			TaskToken: []byte("task-token"),
			Details:   []byte("Details"),
			Identity:  "identity",
		},
	}
	testInput := map[string]struct {
		input         *types.HistoryRespondActivityTaskCanceledRequest
		expectedError bool
		mockFn        func()
	}{
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskCanceled(gomock.Any(), validInput).Return(nil).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: testDomainID,
				CancelRequest: &types.RespondActivityTaskCanceledRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"token deserialization error": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: testDomainID,
				CancelRequest: &types.RespondActivityTaskCanceledRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			},
		},
		"invalid task token": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: testDomainID,
				CancelRequest: &types.RespondActivityTaskCanceledRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			},
		},
		"get engine error": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: testDomainID,
				CancelRequest: &types.RespondActivityTaskCanceledRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input: &types.HistoryRespondActivityTaskCanceledRequest{
				DomainUUID: testDomainID,
				CancelRequest: &types.RespondActivityTaskCanceledRequest{
					TaskToken: []byte("task-token"),
					Details:   []byte("Details"),
					Identity:  "identity",
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondActivityTaskCanceled(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RespondActivityTaskCanceled(context.Background(), input.input)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRespondDecisionTaskCompleted() {
	validReq := &types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: testDomainID,
		CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
			TaskToken: []byte("task-token"),
			Decisions: []*types.Decision{
				{
					DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				},
			},
			ExecutionContext: nil,
			Identity:         "identity",
		},
	}
	validResp := &types.HistoryRespondDecisionTaskCompletedResponse{
		StartedResponse: &types.RecordDecisionTaskStartedResponse{
			WorkflowType: &types.WorkflowType{},
		},
	}
	testInput := map[string]struct {
		input         *types.HistoryRespondDecisionTaskCompletedRequest
		expectedError bool
		mockFn        func()
	}{
		"valid input": {
			input:         validReq,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), validReq).Return(validResp, nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validReq,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"token deserialization error": {
			input: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Decisions: []*types.Decision{},
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			},
		},
		"invalid task token": {
			input: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Decisions: []*types.Decision{},
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			},
		},
		"get engine error": {
			input: &types.HistoryRespondDecisionTaskCompletedRequest{
				DomainUUID: testDomainID,
				CompleteRequest: &types.RespondDecisionTaskCompletedRequest{
					TaskToken: []byte("task-token"),
					Decisions: []*types.Decision{},
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validReq,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), validReq).Return(nil, errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.RespondDecisionTaskCompleted(context.Background(), input.input)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestRespondDecisionTaskFailed() {
	validInput := &types.HistoryRespondDecisionTaskFailedRequest{
		DomainUUID: testDomainID,
		FailedRequest: &types.RespondDecisionTaskFailedRequest{
			TaskToken: []byte("task-token"),
			Cause:     types.DecisionTaskFailedCauseBadBinary.Ptr(),
			Details:   []byte("Details"),
			Identity:  "identity",
		},
	}
	specialInput := &types.HistoryRespondDecisionTaskFailedRequest{
		DomainUUID: testDomainID,
		FailedRequest: &types.RespondDecisionTaskFailedRequest{
			TaskToken: []byte("task-token"),
			Cause:     types.DecisionTaskFailedCauseUnhandledDecision.Ptr(),
			Details:   []byte("Details"),
			Identity:  "identity",
		},
	}
	testInput := map[string]struct {
		input         *types.HistoryRespondDecisionTaskFailedRequest
		expectedError bool
		mockFn        func()
	}{
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskFailed(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryRespondDecisionTaskFailedRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"token deserialization error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("some random error")).Times(1)
			},
		},
		"invalid task token": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: "",
					RunID:      "",
				}, nil).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskFailed(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"special domain": {
			input:         specialInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockResource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("name", nil).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskFailed(gomock.Any(), specialInput).Return(nil).Times(1)
			},
		},
		"special domain2": {
			input:         specialInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{
					WorkflowID: testWorkflowID,
					RunID:      testValidUUID,
				}, nil).Times(1)
				s.mockResource.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("", errors.New("error")).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RespondDecisionTaskFailed(gomock.Any(), specialInput).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RespondDecisionTaskFailed(context.Background(), input.input)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestDescribeHistoryHost() {
	request := &types.DescribeHistoryHostRequest{
		HostAddress: common.StringPtr("test"),
	}

	mockStatus := map[string]int32{
		"initialized": 0,
		"started":     1,
		"stopped":     2,
	}

	for status, value := range mockStatus {
		s.mockResource.DomainCache.EXPECT().GetCacheSize().Return(int64(2), int64(3)).Times(1)
		s.mockShardController.EXPECT().Status().Return(value).Times(1)
		s.mockShardController.EXPECT().NumShards().Return(1)
		s.mockShardController.EXPECT().ShardIDs().Return([]int32{0})
		resp, err := s.handler.DescribeHistoryHost(context.Background(), request)
		s.NoError(err)
		s.Equal(resp.DomainCache, &types.DomainCacheInfo{
			NumOfItemsInCacheByID:   2,
			NumOfItemsInCacheByName: 3,
		})
		s.Equal(resp.ShardControllerStatus, status)
	}
}

func (s *handlerSuite) TestRemoveTask() {
	now := time.Now()
	testInput := map[string]struct {
		request       *types.RemoveTaskRequest
		expectedError bool
		mockFn        func()
	}{
		"transfer task": {
			request: &types.RemoveTaskRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeTransfer)),
				TaskID:  int64(1),
			},
			expectedError: false,
			mockFn: func() {
				s.mockResource.ExecutionMgr.On("CompleteTransferTask", mock.Anything, &persistence.CompleteTransferTaskRequest{
					TaskID: int64(1),
				}).Return(nil).Once()
			},
		},
		"timer task": {
			request: &types.RemoveTaskRequest{
				ShardID:             0,
				Type:                common.Int32Ptr(int32(common.TaskTypeTimer)),
				TaskID:              int64(1),
				VisibilityTimestamp: common.Int64Ptr(int64(now.UnixNano())),
			},
			expectedError: false,
			mockFn: func() {
				s.mockResource.ExecutionMgr.On("CompleteTimerTask", mock.Anything, mock.Anything).Return(nil).Once()
			},
		},
		"replication task": {
			request: &types.RemoveTaskRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeReplication)),
				TaskID:  int64(1),
			},
			expectedError: false,
			mockFn: func() {
				s.mockResource.ExecutionMgr.On("CompleteReplicationTask", mock.Anything, &persistence.CompleteReplicationTaskRequest{
					TaskID: int64(1),
				}).Return(nil).Once()
			},
		},
		"invalid": {
			request: &types.RemoveTaskRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(100)),
			},
			expectedError: true,
			mockFn:        func() {},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RemoveTask(context.Background(), input.request)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestCloseShard() {
	request := &types.CloseShardRequest{
		ShardID: 0,
	}

	s.mockShardController.EXPECT().RemoveEngineForShard(0).Return().Times(1)
	err := s.handler.CloseShard(context.Background(), request)
	s.NoError(err)
}

func (s *handlerSuite) TestResetQueue() {
	testInput := map[string]struct {
		request       *types.ResetQueueRequest
		expectedError bool
		mockFn        func()
	}{
		"getEngine error": {
			request: &types.ResetQueueRequest{
				ShardID: 0,
			},
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(nil, errors.New("error")).Times(1)
			},
		},
		"transfer task": {
			request: &types.ResetQueueRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeTransfer)),
			},
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetTransferQueue(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"timer task": {
			request: &types.ResetQueueRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeTimer)),
			},
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetTimerQueue(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		"invalid task": {
			request: &types.ResetQueueRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(100)),
			},
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.ResetQueue(context.Background(), input.request)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestDescribeQueue() {
	testInput := map[string]struct {
		request       *types.DescribeQueueRequest
		expectedError bool
		mockFn        func()
	}{
		"getEngine error": {
			request: &types.DescribeQueueRequest{
				ShardID: 0,
			},
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(nil, errors.New("error")).Times(1)
			},
		},
		"transfer task": {
			request: &types.DescribeQueueRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeTransfer)),
			},
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeTransferQueue(gomock.Any(), gomock.Any()).Return(&types.DescribeQueueResponse{}, nil).Times(1)
			},
		},
		"timer task": {
			request: &types.DescribeQueueRequest{
				ShardID: 0,
				Type:    common.Int32Ptr(int32(common.TaskTypeTimer)),
			},
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeTimerQueue(gomock.Any(), gomock.Any()).Return(&types.DescribeQueueResponse{}, nil).Times(1)
			},
		},
		"invalid task": {
			request: &types.DescribeQueueRequest{
				Type: common.Int32Ptr(int32(100)),
			},
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(0).Return(s.mockEngine, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.DescribeQueue(context.Background(), input.request)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestDescribeMutableState() {
	validInput := &types.DescribeMutableStateRequest{
		DomainUUID: testDomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}
	testInput := map[string]struct {
		request       *types.DescribeMutableStateRequest
		expectedError bool
		mockFn        func()
	}{
		"empty domainID": {
			request: &types.DescribeMutableStateRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"getEngine error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"getMutableState error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeMutableState(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			request:       validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeMutableState(gomock.Any(), validInput).Return(&types.DescribeMutableStateResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.DescribeMutableState(context.Background(), input.request)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestGetMutableState() {
	validInput := &types.GetMutableStateRequest{
		DomainUUID: testDomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}
	testInput := map[string]struct {
		request       *types.GetMutableStateRequest
		expectedError bool
		mockFn        func()
	}{
		"empty domainID": {
			request: &types.GetMutableStateRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"getMutableState error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetMutableState(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			request:       validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetMutableState(gomock.Any(), validInput).Return(&types.GetMutableStateResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.GetMutableState(context.Background(), input.request)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestPollMutableState() {
	validInput := &types.PollMutableStateRequest{
		DomainUUID: testDomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}
	testInput := map[string]struct {
		request       *types.PollMutableStateRequest
		expectedError bool
		mockFn        func()
	}{
		"empty domainID": {
			request: &types.PollMutableStateRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"getMutableState error": {
			request:       validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().PollMutableState(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			request:       validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().PollMutableState(gomock.Any(), validInput).Return(&types.PollMutableStateResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.PollMutableState(context.Background(), input.request)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestDescribeWorkflowExecution() {
	validInput := &types.HistoryDescribeWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		Request: &types.DescribeWorkflowExecutionRequest{
			Execution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}
	testInput := map[string]struct {
		input         *types.HistoryDescribeWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeWorkflowExecution(gomock.Any(), validInput).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryDescribeWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().DescribeWorkflowExecution(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.DescribeWorkflowExecution(context.Background(), input.input)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRequestCancelWorkflowExecution() {
	validInput := &types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		CancelRequest: &types.RequestCancelWorkflowExecutionRequest{
			Domain: "domain",
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryRequestCancelWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryRequestCancelWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RequestCancelWorkflowExecution(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestSignalWorkflowExecution() {
	validInput := &types.HistorySignalWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		SignalRequest: &types.SignalWorkflowExecutionRequest{
			Domain: "domain",
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}

	testInput := map[string]struct {
		input         *types.HistorySignalWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWorkflowExecution(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistorySignalWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWorkflowExecution(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.SignalWorkflowExecution(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestSignalWithStartWorkflowExecution() {
	validInput := &types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
			WorkflowID: testWorkflowID,
		},
	}

	testInput := map[string]struct {
		input         *types.HistorySignalWithStartWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistorySignalWithStartWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"special engine error and retry failure": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(nil, &persistence.WorkflowExecutionAlreadyStartedError{}).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"special engine error and retry success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(nil, &persistence.CurrentWorkflowConditionFailedError{}).Times(1)
				s.mockEngine.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), validInput).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.SignalWithStartWorkflowExecution(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRemoveSignalMutableState() {
	validInput := &types.RemoveSignalMutableStateRequest{
		DomainUUID: testDomainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}

	testInput := map[string]struct {
		input         *types.RemoveSignalMutableStateRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RemoveSignalMutableState(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.RemoveSignalMutableStateRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RemoveSignalMutableState(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RemoveSignalMutableState(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestTerminateWorkflowExecution() {
	validInput := &types.HistoryTerminateWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		TerminateRequest: &types.TerminateWorkflowExecutionRequest{
			Domain: "domain",
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryTerminateWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().TerminateWorkflowExecution(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().TerminateWorkflowExecution(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.TerminateWorkflowExecution(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestResetWorkflowExecution() {
	validInput := &types.HistoryResetWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		ResetRequest: &types.ResetWorkflowExecutionRequest{
			Domain: "domain",
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
			Reason:                "test",
			DecisionFinishEventID: 1,
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryResetWorkflowExecutionRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"valid input": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetWorkflowExecution(gomock.Any(), validInput).Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryResetWorkflowExecutionRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetWorkflowExecution(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.ResetWorkflowExecution(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestQueryWorkflow() {
	validInput := &types.HistoryQueryWorkflowRequest{
		DomainUUID: testDomainID,
		Request: &types.QueryWorkflowRequest{
			Domain: "domain",
			Execution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryQueryWorkflowRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryQueryWorkflowRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"queryWorkflow error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().QueryWorkflow(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().QueryWorkflow(gomock.Any(), validInput).Return(&types.HistoryQueryWorkflowResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.QueryWorkflow(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestScheduleDecisionTask() {
	validInput := &types.ScheduleDecisionTaskRequest{
		DomainUUID: testDomainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}

	testInput := map[string]struct {
		input         *types.ScheduleDecisionTaskRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"empty domainID": {
			input: &types.ScheduleDecisionTaskRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"scheduleDecisionTask error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ScheduleDecisionTask(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ScheduleDecisionTask(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty execution": {
			input: &types.ScheduleDecisionTaskRequest{
				DomainUUID: testDomainID,
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.ScheduleDecisionTask(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRecordChildExecutionCompleted() {
	validInput := &types.RecordChildExecutionCompletedRequest{
		DomainUUID:  testDomainID,
		InitiatedID: 1,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}

	testInput := map[string]struct {
		input         *types.RecordChildExecutionCompletedRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"empty domainID": {
			input: &types.RecordChildExecutionCompletedRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"recordChildExecutionCompleted error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordChildExecutionCompleted(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RecordChildExecutionCompleted(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty execution": {
			input: &types.RecordChildExecutionCompletedRequest{
				DomainUUID: testDomainID,
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RecordChildExecutionCompleted(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestResetStickyTaskList() {
	validInput := &types.HistoryResetStickyTaskListRequest{
		DomainUUID: testDomainID,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryResetStickyTaskListRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"empty domainID": {
			input: &types.HistoryResetStickyTaskListRequest{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"resetStickyTaskList error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetStickyTaskList(gomock.Any(), validInput).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ResetStickyTaskList(gomock.Any(), validInput).Return(&types.HistoryResetStickyTaskListResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.ResetStickyTaskList(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})

	}
}

func (s *handlerSuite) TestReplicateEventsV2() {
	validInput := &types.ReplicateEventsV2Request{
		DomainUUID: testDomainID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testValidUUID,
		},
		VersionHistoryItems: []*types.VersionHistoryItem{
			{
				EventID: 1,
				Version: 1,
			},
		},
		Events: &types.DataBlob{
			EncodingType: types.EncodingTypeThriftRW.Ptr(),
			Data:         []byte{1, 2, 3},
		},
	}

	testInput := map[string]struct {
		input         *types.ReplicateEventsV2Request
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"empty domainID": {
			input: &types.ReplicateEventsV2Request{
				DomainUUID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"getEngine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"replicateEventsV2 error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReplicateEventsV2(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.ReplicateEventsV2(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestSyncShardStatus() {
	validInput := &types.SyncShardStatusRequest{
		SourceCluster: "test",
		ShardID:       1,
		Timestamp:     common.Int64Ptr(time.Now().UnixNano()),
	}

	testInput := map[string]struct {
		input         *types.SyncShardStatusRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"get shard engine": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.ShardID)).Return(nil, errors.New("error")).Times(1)
			},
		},
		"syncShardStatus error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SyncShardStatus(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SyncShardStatus(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
		"empty sourceCluster": {
			input: &types.SyncShardStatusRequest{
				SourceCluster: "",
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
		"missing timestamp": {
			input: &types.SyncShardStatusRequest{
				SourceCluster: "test",
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.SyncShardStatus(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestSyncActivity() {
	validInput := &types.SyncActivityRequest{
		DomainID:    testDomainID,
		WorkflowID:  testWorkflowID,
		RunID:       testValidUUID,
		Version:     1,
		ScheduledID: 1,
		Details:     []byte{1, 2, 3},
	}

	testInput := map[string]struct {
		input         *types.SyncActivityRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"ratelimit exceeded": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(false).Times(1)
			},
		},
		"empty domainID": {
			input: &types.SyncActivityRequest{
				DomainID: "",
			},
			expectedError: true,
			mockFn:        func() {},
		},
		"empty workflowID": {
			input: &types.SyncActivityRequest{
				DomainID:   testDomainID,
				WorkflowID: "",
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
		"empty runID": {
			input: &types.SyncActivityRequest{
				DomainID:   testDomainID,
				WorkflowID: testWorkflowID,
				RunID:      "",
			},
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
			},
		},
		"cannot get engine": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"syncActivity error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SyncActivity(gomock.Any(), validInput).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().SyncActivity(gomock.Any(), validInput).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.SyncActivity(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestGetReplicationMessages() {
	validInput := &types.GetReplicationMessagesRequest{
		ClusterName: "test",
		Tokens: []*types.ReplicationToken{
			{
				ShardID:                1,
				LastRetrievedMessageID: 1,
			},
			{
				ShardID:                2,
				LastRetrievedMessageID: 2,
			},
		},
	}

	testInput := map[string]struct {
		input         *types.GetReplicationMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[0].ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetReplicationMessages(gomock.Any(), validInput.ClusterName, validInput.Tokens[0].LastRetrievedMessageID).Return(&types.ReplicationMessages{}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[1].ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetReplicationMessages(gomock.Any(), validInput.ClusterName, validInput.Tokens[1].LastRetrievedMessageID).Return(&types.ReplicationMessages{}, nil).Times(1)
			},
		},
		"cannot get engine and cannot get task": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[0].ShardID)).Return(nil, errors.New("errors")).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[1].ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetReplicationMessages(gomock.Any(), validInput.ClusterName, validInput.Tokens[1].LastRetrievedMessageID).Return(nil, errors.New("errors")).Times(1)
			},
		},
		"maxSize exceeds": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.handler.config.MaxResponseSize = 0
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[0].ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetReplicationMessages(gomock.Any(), validInput.ClusterName, validInput.Tokens[0].LastRetrievedMessageID).Return(&types.ReplicationMessages{
					ReplicationTasks: []*types.ReplicationTask{
						{
							TaskType: types.ReplicationTaskTypeHistory.Ptr(),
						},
						{
							TaskType: types.ReplicationTaskTypeHistory.Ptr(),
						},
					},
				}, nil).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(int(validInput.Tokens[1].ShardID)).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetReplicationMessages(gomock.Any(), validInput.ClusterName, validInput.Tokens[1].LastRetrievedMessageID).Return(&types.ReplicationMessages{
					ReplicationTasks: []*types.ReplicationTask{
						{
							TaskType: types.ReplicationTaskTypeHistory.Ptr(),
						},
						{
							TaskType: types.ReplicationTaskTypeHistory.Ptr(),
						},
					},
				}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.GetReplicationMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
			goleak.VerifyNone(s.T())
		})
	}
}

func (s *handlerSuite) TestGetDLQReplicationMessages() {
	validInput := &types.GetDLQReplicationMessagesRequest{
		TaskInfos: []*types.ReplicationTaskInfo{
			{
				DomainID:   testDomainID,
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}
	mockResp := make([]*types.ReplicationTask, 0, 10)
	mockResp = append(mockResp, &types.ReplicationTask{
		TaskType: types.ReplicationTaskTypeHistory.Ptr(),
	})

	mockEmptyResp := make([]*types.ReplicationTask, 0)

	testInput := map[string]struct {
		input         *types.GetDLQReplicationMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any()).Return(mockResp, nil).Times(1)
			},
		},
		"cannot get engine": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(gomock.Any()).Return(nil, errors.New("error")).Times(1)
			},
		},
		"cannot get task": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
			},
		},
		"empty task response": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any()).Return(mockEmptyResp, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.GetDLQReplicationMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
			goleak.VerifyNone(s.T())
		})

	}
}

func (s *handlerSuite) TestReapplyEvents() {
	validInput := &types.HistoryReapplyEventsRequest{
		DomainUUID: testDomainID,
		Request: &types.ReapplyEventsRequest{
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
			Events: &types.DataBlob{
				EncodingType: types.EncodingTypeThriftRW.Ptr(),
				Data:         []byte{},
			},
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryReapplyEventsRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"cannot get engine": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"cannot get serialized": {
			input: &types.HistoryReapplyEventsRequest{
				DomainUUID: testDomainID,
				Request: &types.ReapplyEventsRequest{
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: testWorkflowID,
						RunID:      testValidUUID,
					},
					Events: &types.DataBlob{
						EncodingType: types.EncodingTypeThriftRW.Ptr(),
						Data:         []byte{1, 2, 3, 4},
					},
				},
			},
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
			},
		},
		"reapplyEvents error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.ReapplyEvents(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestCountDLQMessages() {
	validInput := &types.CountDLQMessagesRequest{
		ForceFetch: true,
	}

	testInput := map[string]struct {
		input         *types.CountDLQMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"cannot get engine": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().ShardIDs().Return([]int32{0}).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(gomock.Any()).Return(nil, errors.New("error")).Times(1)
			},
		},
		"countDLQMessages error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().ShardIDs().Return([]int32{0}).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any()).Return(map[string]int64{}, errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().ShardIDs().Return([]int32{0}).Times(1)
				s.mockShardController.EXPECT().GetEngineForShard(gomock.Any()).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test":  1,
					"test2": 2,
				}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			_, err := s.handler.CountDLQMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestReadDLQMessages() {
	validInput := &types.ReadDLQMessagesRequest{
		ShardID: 1,
	}

	testInput := map[string]struct {
		input         *types.ReadDLQMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"get shard engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(nil, errors.New("error")).Times(1)
			},
		},
		"readDLQMessages error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReadDLQMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				resp := &types.ReadDLQMessagesResponse{}
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().ReadDLQMessages(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			resp, err := s.handler.ReadDLQMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Nil(resp)
				s.Error(err)
			} else {
				s.NotNil(resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestPurgeDLQMessages() {
	validInput := &types.PurgeDLQMessagesRequest{
		ShardID: 1,
	}

	testInput := map[string]struct {
		input         *types.PurgeDLQMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"get shard engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(nil, errors.New("error")).Times(1)
			},
		},
		"purgeDLQMessages error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().PurgeDLQMessages(gomock.Any(), gomock.Any()).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().PurgeDLQMessages(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.PurgeDLQMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestMergeDLQMessages() {
	validInput := &types.MergeDLQMessagesRequest{
		ShardID: 1,
	}

	testInput := map[string]struct {
		input         *types.MergeDLQMessagesRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"get shard engine error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(nil, errors.New("error")).Times(1)
			},
		},
		"mergeDLQMessages error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().MergeDLQMessages(gomock.Any(), gomock.Any()).Return(nil, errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngineForShard(1).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().MergeDLQMessages(gomock.Any(), gomock.Any()).Return(&types.MergeDLQMessagesResponse{}, nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			_, err := s.handler.MergeDLQMessages(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestRefreshWorkflowTasks() {
	validInput := &types.HistoryRefreshWorkflowTasksRequest{
		DomainUIID: testDomainID,
		Request: &types.RefreshWorkflowTasksRequest{
			Execution: &types.WorkflowExecution{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
		},
	}

	testInput := map[string]struct {
		input         *types.HistoryRefreshWorkflowTasksRequest
		expectedError bool
		mockFn        func()
	}{
		"shutting down": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.handler.shuttingDown = int32(1)
			},
		},
		"cannot get engine": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(nil, errors.New("error")).Times(1)
			},
		},
		"refreshWorkflowTasks error": {
			input:         validInput,
			expectedError: true,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("error")).Times(1)
			},
		},
		"success": {
			input:         validInput,
			expectedError: false,
			mockFn: func() {
				s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
				s.mockEngine.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := s.handler.RefreshWorkflowTasks(context.Background(), input.input)
			s.handler.shuttingDown = int32(0)
			if input.expectedError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestStartWorkflowExecution() {

	request := &types.HistoryStartWorkflowExecutionRequest{
		DomainUUID: testDomainID,
		StartRequest: &types.StartWorkflowExecutionRequest{
			WorkflowID: testWorkflowID,
		},
	}

	expectedResponse := &types.StartWorkflowExecutionResponse{
		RunID: testWorkflowRunID,
	}

	s.mockShardController.EXPECT().GetEngine(testWorkflowID).Return(s.mockEngine, nil).Times(1)
	s.mockRatelimiter.EXPECT().Allow().Return(true).Times(1)
	s.mockEngine.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(expectedResponse, nil).Times(1)

	response, err := s.handler.StartWorkflowExecution(context.Background(), request)
	s.Equal(expectedResponse, response)
	s.Nil(err)
}

func (s *handlerSuite) TestEmitInfoOrDebugLog() {
	// test emitInfoOrDebugLog
	s.mockResource.Logger = testlogger.New(s.Suite.T())
	s.handler.emitInfoOrDebugLog("domain1", "test log")
}

func (s *handlerSuite) TestValidateTaskToken() {
	testInput := map[string]struct {
		taskToken     *common.TaskToken
		expectedError error
	}{
		"valid task token": {
			taskToken: &common.TaskToken{
				WorkflowID: testWorkflowID,
				RunID:      testValidUUID,
			},
			expectedError: nil,
		},
		"empty workflow id": {
			taskToken: &common.TaskToken{
				WorkflowID: "",
			},
			expectedError: constants.ErrWorkflowIDNotSet,
		},
		"invalid run id": {
			taskToken: &common.TaskToken{
				WorkflowID: testWorkflowID,
				RunID:      "invalid",
			},
			expectedError: constants.ErrRunIDNotValid,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			err := validateTaskToken(input.taskToken)
			s.Equal(input.expectedError, err)
		})
	}
}

func (s *handlerSuite) TestCorrectUseOfErrorHandling() {

	tests := map[string]struct {
		input       error
		expectation func(scope *mocks.Scope)
	}{
		"A deadline exceeded error": {
			input: context.DeadlineExceeded,
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrContextTimeoutCounter).Once()
			},
		},
		"A cancelled error": {
			input: context.Canceled,
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrContextTimeoutCounter).Once()
			},
		},
		"A shard ownership lost error": {
			input: &types.ShardOwnershipLostError{
				Message: "something is lost",
				Owner:   "owner",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrShardOwnershipLostCounter).Once()
			},
		},
		"a workflow is already started": {
			input: &types.EventAlreadyStartedError{
				Message: "workflow already running",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrEventAlreadyStartedCounter).Once()
			},
		},
		"a bad request": {
			input: &types.BadRequestError{
				Message: "bad request",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrBadRequestCounter).Once()
			},
		},
		"domain is not active": {
			input: &types.DomainNotActiveError{
				Message: "domain not active",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrDomainNotActiveCounter).Once()
			},
		},
		"workflow is already started err": {
			input: &types.WorkflowExecutionAlreadyStartedError{
				Message: "bad already started",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrExecutionAlreadyStartedCounter).Once()
			},
		},
		"does not exist": {
			input: &types.EntityNotExistsError{
				Message: "the workflow doesn't exist",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrEntityNotExistsCounter).Once()
			},
		},
		"already completed": {
			input: &types.WorkflowExecutionAlreadyCompletedError{
				Message: "the workflow is done",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrWorkflowExecutionAlreadyCompletedCounter).Once()
			},
		},
		"Cancellation already requested": {
			input: &types.CancellationAlreadyRequestedError{
				Message: "the workflow is cancelled already",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrCancellationAlreadyRequestedCounter).Once()
			},
		},
		"rate-limits": {
			input: &types.LimitExceededError{
				Message: "limits exceeded",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrLimitExceededCounter).Once()
			},
		},
		"retry tasks": {
			input: &types.RetryTaskV2Error{
				Message: "limits exceeded",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrRetryTaskCounter).Once()
			},
		},
		"service busy error": {
			input: &types.ServiceBusyError{
				Message: "limits exceeded - service is busy",
			},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrServiceBusyCounter).Once()
			},
		},
		"deadline exceeded": {
			input: yarpcerrors.DeadlineExceededErrorf("some deadline exceeded err"),
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceErrContextTimeoutCounter).Once()
				scope.Mock.On("IncCounter", metrics.CadenceFailures).Once()
			},
		},
		"internal error": {
			input: types.InternalServiceError{Message: "internal error"},
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceFailures).Once()
			},
		},
		"uncategorized error": {
			input: errors.New("some random error"),
			expectation: func(scope *mocks.Scope) {
				scope.Mock.On("IncCounter", metrics.CadenceFailures).Once()
			},
		},
	}

	for name, td := range tests {
		s.Run(name, func() {
			scope := mocks.Scope{}
			td.expectation(&scope)
			h := handlerImpl{
				Resource: resource.NewTest(s.T(), gomock.NewController(s.T()), 0),
			}
			h.error(td.input, &scope, "some-domain", "some-wf", "some-run")
			// we're doing the args assertion in the On, so using mock.Anything to avoid having to duplicate this
			// a wrong metric being emitted will fail the mock.On() expectation. This will catch missing calls
			scope.Mock.AssertCalled(s.T(), "IncCounter", mock.Anything)
		})
	}
}

func (s *handlerSuite) TestConvertError() {
	testCases := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name: "workflow already started error",
			input: &persistence.WorkflowExecutionAlreadyStartedError{
				Msg: "workflow already started",
			},
			expected: &types.InternalServiceError{
				Message: "workflow already started",
			},
		},
		{
			name: "current workflow condition failed error",
			input: &persistence.CurrentWorkflowConditionFailedError{
				Msg: "current workflow condition failed",
			},
			expected: &types.InternalServiceError{
				Message: "current workflow condition failed",
			},
		},
		{
			name: "persistence timeout error",
			input: &persistence.TimeoutError{
				Msg: "persistence timeout",
			},
			expected: &types.InternalServiceError{
				Message: "persistence timeout",
			},
		},
		{
			name: "transaction size limit error",
			input: &persistence.TransactionSizeLimitError{
				Msg: "transaction size limit",
			},
			expected: &types.BadRequestError{
				Message: "transaction size limit",
			},
		},
		{
			name: "shard ownership lost error",
			input: &persistence.ShardOwnershipLostError{
				ShardID: 1,
			},
			expected: &types.ShardOwnershipLostError{
				Owner:   "127.0.0.1:1234",
				Message: "Shard is not owned by host: test_host",
			},
		},
	}

	for _, tc := range testCases {
		s.mockResource.MembershipResolver.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(membership.NewHostInfo("127.0.0.1:1234"), nil).AnyTimes()
		err := s.handler.convertError(tc.input)
		s.Equal(tc.expected, err, tc.name)
	}
}

func TestRatelimitUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	res := resource.NewMockResource(ctrl)
	res.EXPECT().GetMetricsClient().Return(metrics.NewNoopMetricsClient()).AnyTimes()
	res.EXPECT().GetLogger().Return(testlogger.New(t)).AnyTimes()
	update, err := rpc.TestUpdateToAny(t, "testhost", time.Second, map[shared.GlobalKey]rpc.Calls{
		"test:domain-user-limit": {
			Allowed:  10,
			Rejected: 5,
		},
	})
	require.NoError(t, err)
	alg, err := algorithm.New(
		metrics.NewNoopMetricsClient(),
		testlogger.New(t),
		algorithm.Config{
			NewDataWeight:  func(opts ...dynamicconfig.FilterOption) float64 { return 0.5 },
			UpdateInterval: func(opts ...dynamicconfig.FilterOption) time.Duration { return 3 * time.Second },
			DecayAfter:     func(opts ...dynamicconfig.FilterOption) time.Duration { return 6 * time.Second },
			GcAfter:        func(opts ...dynamicconfig.FilterOption) time.Duration { return time.Minute },
		},
	)
	require.NoError(t, err)
	h := &handlerImpl{
		Resource:            res,
		ratelimitAggregator: alg,
	}

	resp, err := h.RatelimitUpdate(context.Background(), &types.RatelimitUpdateRequest{
		Any: update,
	})
	require.NoError(t, err)
	w, err := rpc.TestAnyToWeights(t, resp.Any)
	require.NoError(t, err)
	assert.Equalf(t,
		map[shared.GlobalKey]rpc.UpdateEntry{
			"test:domain-user-limit": {
				Weight: 1,
				// re 10 vs 15: used RPS only tracks accepted.
				//
				// this way we don't consider any incorrectly-rejected requests
				// when calculating our new limits.
				UsedRPS: 10,
			},
		},
		w,
		"unexpected weights returned from aggregator or serialization.  if values differ in a reasonable way, possibly aggregator behavior changed?",
	)
}
