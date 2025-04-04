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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc/yarpctest"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/domain"
	dc "github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
)

const (
	numHistoryShards          = 10
	testDomain                = "test-domain"
	canaryDomain              = "cadence-canary"
	testDomainID              = "e4f90ec0-1313-45be-9877-8aa41f72a45a"
	testWorkflowID            = "test-workflow-id"
	testRunID                 = "2c8b555f-1f55-4955-9d1c-b980194555c9"
	testHistoryArchivalURI    = "testScheme://history/URI"
	testVisibilityArchivalURI = "testScheme://visibility/URI"
)

type (
	workflowHandlerSuite struct {
		suite.Suite
		*require.Assertions

		controller         *gomock.Controller
		mockResource       *resource.Test
		mockDomainCache    *cache.MockDomainCache
		mockHistoryClient  *history.MockClient
		mockMatchingClient *matching.MockClient
		domainHandler      domain.Handler

		mockProducer           *mocks.KafkaProducer
		mockMessagingClient    messaging.Client
		mockMetadataMgr        *mocks.MetadataManager
		mockHistoryV2Mgr       *mocks.HistoryV2Manager
		mockVisibilityMgr      *mocks.VisibilityManager
		mockArchivalMetadata   *archiver.MockArchivalMetadata
		mockArchiverProvider   *provider.MockArchiverProvider
		mockHistoryArchiver    *archiver.HistoryArchiverMock
		mockVisibilityArchiver *archiver.VisibilityArchiverMock
		mockVersionChecker     *client.MockVersionChecker
		mockTokenSerializer    *common.MockTaskTokenSerializer

		testDomain   string
		testDomainID string
	}
)

func TestWorkflowHandlerSuite(t *testing.T) {
	s := new(workflowHandlerSuite)
	suite.Run(t, s)
}

func (s *workflowHandlerSuite) SetupSuite() {
}

func (s *workflowHandlerSuite) TearDownSuite() {
}

func (s *workflowHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.testDomain = testDomain
	s.testDomainID = testDomainID

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Frontend)
	s.mockDomainCache = s.mockResource.DomainCache
	s.mockHistoryClient = s.mockResource.HistoryClient
	s.mockMatchingClient = s.mockResource.MatchingClient
	s.mockMetadataMgr = s.mockResource.MetadataMgr
	s.mockHistoryV2Mgr = s.mockResource.HistoryMgr
	s.mockVisibilityMgr = s.mockResource.VisibilityMgr
	s.mockArchivalMetadata = s.mockResource.ArchivalMetadata
	s.mockArchiverProvider = s.mockResource.ArchiverProvider
	s.mockTokenSerializer = common.NewMockTaskTokenSerializer(s.controller)

	s.mockProducer = &mocks.KafkaProducer{}
	s.mockMessagingClient = mocks.NewMockMessagingClient(s.mockProducer, nil)
	s.mockHistoryArchiver = &archiver.HistoryArchiverMock{}
	s.mockVisibilityArchiver = &archiver.VisibilityArchiverMock{}
	s.mockVersionChecker = client.NewMockVersionChecker(s.controller)

	// these tests don't mock the domain handler
	config := s.newConfig(dc.NewInMemoryClient())
	s.domainHandler = domain.NewHandler(
		config.DomainConfig,
		s.mockResource.GetLogger(),
		s.mockResource.GetDomainManager(),
		s.mockResource.GetClusterMetadata(),
		domain.NewDomainReplicator(s.mockProducer, s.mockResource.GetLogger()),
		s.mockResource.GetArchivalMetadata(),
		s.mockResource.GetArchiverProvider(),
		s.mockResource.GetTimeSource(),
	)

	mockMonitor := s.mockResource.MembershipResolver
	mockMonitor.EXPECT().MemberCount(service.Frontend).Return(5, nil).AnyTimes()
	s.mockVersionChecker.EXPECT().ClientSupported(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

func (s *workflowHandlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockResource.Finish(s.T())
	s.mockProducer.AssertExpectations(s.T())
	s.mockHistoryArchiver.AssertExpectations(s.T())
	s.mockVisibilityArchiver.AssertExpectations(s.T())
}

func (s *workflowHandlerSuite) getWorkflowHandler(config *frontendcfg.Config) *WorkflowHandler {
	return NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, s.domainHandler)
}

func (s *workflowHandlerSuite) TestDisableListVisibilityByFilter() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.DisableListVisibilityByFilter = dc.GetBoolPropertyFnFilteredByDomain(true)

	wh := s.getWorkflowHandler(config)

	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()

	// test list open by wid
	listRequest := &types.ListOpenWorkflowExecutionsRequest{
		Domain: s.testDomain,
		StartTimeFilter: &types.StartTimeFilter{
			EarliestTime: common.Int64Ptr(0),
			LatestTime:   common.Int64Ptr(time.Now().UnixNano()),
		},
		ExecutionFilter: &types.WorkflowExecutionFilter{
			WorkflowID: "wid",
		},
	}
	_, err := wh.ListOpenWorkflowExecutions(context.Background(), listRequest)
	s.Error(err)
	s.Equal(validate.ErrNoPermission, err)

	// test list open by workflow type
	listRequest.ExecutionFilter = nil
	listRequest.TypeFilter = &types.WorkflowTypeFilter{
		Name: "workflow-type",
	}
	_, err = wh.ListOpenWorkflowExecutions(context.Background(), listRequest)
	s.Error(err)
	s.Equal(validate.ErrNoPermission, err)

	// test list close by wid
	listRequest2 := &types.ListClosedWorkflowExecutionsRequest{
		Domain: s.testDomain,
		StartTimeFilter: &types.StartTimeFilter{
			EarliestTime: common.Int64Ptr(0),
			LatestTime:   common.Int64Ptr(time.Now().UnixNano()),
		},
		ExecutionFilter: &types.WorkflowExecutionFilter{
			WorkflowID: "wid",
		},
	}
	_, err = wh.ListClosedWorkflowExecutions(context.Background(), listRequest2)
	s.Error(err)
	s.Equal(validate.ErrNoPermission, err)

	// test list close by workflow type
	listRequest2.ExecutionFilter = nil
	listRequest2.TypeFilter = &types.WorkflowTypeFilter{
		Name: "workflow-type",
	}
	_, err = wh.ListClosedWorkflowExecutions(context.Background(), listRequest2)
	s.Error(err)
	s.Equal(validate.ErrNoPermission, err)

	// test list close by workflow status
	listRequest2.TypeFilter = nil
	failedStatus := types.WorkflowExecutionCloseStatusFailed
	listRequest2.StatusFilter = &failedStatus
	_, err = wh.ListClosedWorkflowExecutions(context.Background(), listRequest2)
	s.Error(err)
	s.Equal(validate.ErrNoPermission, err)
}

func (s *workflowHandlerSuite) TestPollForTask_Failed_ContextTimeoutTooShort() {
	config := s.newConfig(dc.NewInMemoryClient())
	wh := s.getWorkflowHandler(config)

	bgCtx := context.Background()
	_, err := wh.PollForDecisionTask(bgCtx, &types.PollForDecisionTaskRequest{
		Domain: s.testDomain,
	})
	s.Error(err)
	s.Equal(common.ErrContextTimeoutNotSet, err)

	_, err = wh.PollForActivityTask(bgCtx, &types.PollForActivityTaskRequest{
		Domain: s.testDomain,
	})
	s.Error(err)
	s.Equal(common.ErrContextTimeoutNotSet, err)

	shortCtx, cancel := context.WithTimeout(bgCtx, constants.MinLongPollTimeout-time.Millisecond)
	defer cancel()

	_, err = wh.PollForDecisionTask(shortCtx, &types.PollForDecisionTaskRequest{
		Domain: s.testDomain,
	})
	s.Error(err)
	s.Equal(common.ErrContextTimeoutTooShort, err)

	_, err = wh.PollForActivityTask(shortCtx, &types.PollForActivityTaskRequest{
		Domain: s.testDomain,
	})
	s.Error(err)
	s.Equal(common.ErrContextTimeoutTooShort, err)
}

func (s *workflowHandlerSuite) TestPollForDecisionTask_AutoConfigHint() {

	for _, tt := range []struct {
		name     string
		response *types.MatchingPollForDecisionTaskResponse
		expected *types.PollForDecisionTaskResponse
	}{
		{
			"success",
			&types.MatchingPollForDecisionTaskResponse{
				TaskToken: []byte("some value"),
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "rid",
				},
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
			&types.PollForDecisionTaskResponse{
				TaskToken: []byte("some value"),
				History: &types.History{
					Events: []*types.HistoryEvent{},
				},
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "rid",
				},
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
		},
		{
			"success with empty poll",
			&types.MatchingPollForDecisionTaskResponse{
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
			&types.PollForDecisionTaskResponse{
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
		},
	} {
		s.T().Run(tt.name, func(t *testing.T) {
			s.mockDomainCache.EXPECT().GetDomain(s.testDomain).Return(cache.NewLocalDomainCacheEntryForTest(
				&persistence.DomainInfo{Name: s.testDomain, ID: s.testDomainID},
				&persistence.DomainConfig{},
				"",
			), nil).AnyTimes()
			s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil).AnyTimes()
			s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(
				&persistence.ReadHistoryBranchResponse{}, nil).Maybe()
			s.mockResource.MatchingClient.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any()).Return(
				tt.response, nil).Times(1)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			config := s.newConfig(dc.NewInMemoryClient())
			wh := s.getWorkflowHandler(config)
			resp, err := wh.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
				Domain: s.testDomain,
				TaskList: &types.TaskList{
					Name: "task-list",
				},
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp)
		})
	}
}

func (s *workflowHandlerSuite) TestPollForDecisionTask_IsolationGroupDrained() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableTasklistIsolation = dc.GetBoolPropertyFnFilteredByDomain(true)
	wh := s.getWorkflowHandler(config)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	isolationGroup := "dca1"
	ctx = isolationgroup.ContextWithIsolationGroup(ctx, isolationGroup)

	s.mockDomainCache.EXPECT().GetDomain(s.testDomain).Return(cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{},
		"",
	), nil)
	s.mockResource.IsolationGroups.EXPECT().IsDrained(gomock.Any(), s.testDomain, isolationGroup).Return(true, nil).AnyTimes()
	resp, err := wh.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain: s.testDomain,
		TaskList: &types.TaskList{
			Name: "task-list",
		},
	})
	s.NoError(err)
	s.Equal(&types.PollForDecisionTaskResponse{}, resp)
}

func (s *workflowHandlerSuite) TestPollForActivityTask_IsolationGroupDrained() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableTasklistIsolation = dc.GetBoolPropertyFnFilteredByDomain(true)
	wh := s.getWorkflowHandler(config)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	isolationGroup := "dca1"
	ctx = isolationgroup.ContextWithIsolationGroup(ctx, isolationGroup)

	s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
	s.mockResource.IsolationGroups.EXPECT().IsDrained(gomock.Any(), s.testDomain, isolationGroup).Return(true, nil).AnyTimes()
	resp, err := wh.PollForActivityTask(ctx, &types.PollForActivityTaskRequest{
		Domain: s.testDomain,
		TaskList: &types.TaskList{
			Name: "task-list",
		},
	})
	s.NoError(err)
	s.Equal(&types.PollForActivityTaskResponse{}, resp)
}

func (s *workflowHandlerSuite) TestPollForActivityTask() {

	for _, tt := range []struct {
		name     string
		response *types.MatchingPollForActivityTaskResponse
		expected *types.PollForActivityTaskResponse
	}{
		{
			"success",
			&types.MatchingPollForActivityTaskResponse{
				TaskToken: []byte("token"),
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "rid",
				},
				ActivityID:     "1",
				Input:          []byte(`{"key": "value"}`),
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
			&types.PollForActivityTaskResponse{
				TaskToken: []byte("token"),
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "wid",
					RunID:      "rid",
				},
				ActivityID:     "1",
				Input:          []byte(`{"key": "value"}`),
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
		},
		{
			"success with empty polls",
			&types.MatchingPollForActivityTaskResponse{
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
			&types.PollForActivityTaskResponse{
				AutoConfigHint: &types.AutoConfigHint{true, 1000},
			},
		},
	} {
		s.T().Run(tt.name, func(t *testing.T) {
			config := s.newConfig(dc.NewInMemoryClient())
			config.EnableTasklistIsolation = dc.GetBoolPropertyFnFilteredByDomain(true)
			wh := s.getWorkflowHandler(config)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			isolationGroup := "dca1"
			ctx = isolationgroup.ContextWithIsolationGroup(ctx, isolationGroup)

			s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
			s.mockResource.IsolationGroups.EXPECT().IsDrained(gomock.Any(), s.testDomain, isolationGroup).Return(false, nil).AnyTimes()
			s.mockMatchingClient.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any()).Return(tt.response, nil)
			resp, err := wh.PollForActivityTask(ctx, &types.PollForActivityTaskRequest{
				Domain: s.testDomain,
				TaskList: &types.TaskList{
					Name: "task-list",
				},
			})
			s.NoError(err)
			s.Equal(tt.expected, resp)
		})
	}
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_RequestIdNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(&types.BadRequestError{Message: "requestId \"\" is not a valid UUID"}, err)
	startWorkflowExecutionRequest.RequestID = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	_, err = wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(&types.BadRequestError{Message: "requestId \"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\" is not a valid UUID"}, err)

}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_BadDelayStartSeconds() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID:         uuid.New(),
		DelayStartSeconds: common.Int32Ptr(-1),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrInvalidDelayStartSeconds, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_StartRequestNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	_, err := wh.StartWorkflowExecution(context.Background(), nil)
	s.Error(err)
	s.Equal(validate.ErrRequestNotSet, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_DomainNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrDomainNotSet, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_WorkflowIdNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrWorkflowIDNotSet, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_WorkflowTypeNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrWorkflowTypeNotSet, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_TaskListNotSet() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrTaskListNotSet, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_InvalidExecutionStartToCloseTimeout() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(0),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrInvalidExecutionStartToCloseTimeoutSeconds, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Failed_InvalidTaskStartToCloseTimeout() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(0),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.Error(err)
	s.Equal(validate.ErrInvalidTaskStartToCloseTimeoutSeconds, err)
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_IsolationGroupDrained() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	config.EnableTasklistIsolation = dc.GetBoolPropertyFnFilteredByDomain(true)
	wh := s.getWorkflowHandler(config)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	isolationGroup := "dca1"
	ctx := isolationgroup.ContextWithIsolationGroup(context.Background(), isolationGroup)
	s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
	s.mockResource.IsolationGroups.EXPECT().IsDrained(gomock.Any(), s.testDomain, isolationGroup).Return(true, nil)
	_, err := wh.StartWorkflowExecution(ctx, startWorkflowExecutionRequest)
	s.Error(err)
	s.IsType(err, &types.BadRequestError{})
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_LogJitterTime() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.UserRPS = dc.GetIntPropertyFn(10)
	wh := s.getWorkflowHandler(config)
	jitterStart := int32(10)

	startWorkflowExecutionRequest := &types.StartWorkflowExecutionRequest{
		Domain:     canaryDomain,
		WorkflowID: "workflow-id",
		WorkflowType: &types.WorkflowType{
			Name: "workflow-type",
		},
		TaskList: &types.TaskList{
			Name: "task-list",
		},
		JitterStartSeconds:                  &jitterStart,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          2,
			MaximumIntervalInSeconds:    2,
			MaximumAttempts:             1,
			ExpirationIntervalInSeconds: 1,
		},
		RequestID: uuid.New(),
	}
	s.mockDomainCache.EXPECT().GetDomainID(canaryDomain).Return(s.testDomainID, nil).Times(2)
	s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.StartWorkflowExecutionResponse{RunID: "test-rid"}, nil)
	_, err := wh.StartWorkflowExecution(context.Background(), startWorkflowExecutionRequest)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestDiagnoseWorkflowExecution_Success() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := &types.DiagnoseWorkflowExecutionRequest{
		Domain: testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		Identity: "",
	}
	diagnosticWfDomain := "cadence-system"
	diagnosticWfID := fmt.Sprintf("%s-%s", testDomain, testRunID)
	diagnosticWfRunID := "123"
	resp := &types.DiagnoseWorkflowExecutionResponse{
		Domain: diagnosticWfDomain,
		DiagnosticWorkflowExecution: &types.WorkflowExecution{
			WorkflowID: diagnosticWfID,
			RunID:      diagnosticWfRunID,
		},
	}

	s.mockDomainCache.EXPECT().GetDomainID(diagnosticWfDomain).Return(s.testDomainID, nil).Times(2)
	s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.StartWorkflowExecutionResponse{RunID: diagnosticWfRunID}, nil)
	result, err := wh.DiagnoseWorkflowExecution(context.Background(), req)
	s.NoError(err)
	s.Equal(resp, result)
}

func (s *workflowHandlerSuite) TestDiagnoseWorkflowExecution_Failed_RequestNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	result, err := wh.DiagnoseWorkflowExecution(context.Background(), nil)
	s.Error(err)
	s.Equal(validate.ErrRequestNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestDiagnoseWorkflowExecution_Failed_DomainNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	result, err := wh.DiagnoseWorkflowExecution(context.Background(), &types.DiagnoseWorkflowExecutionRequest{
		Domain: "",
	})
	s.Error(err)
	s.Equal(validate.ErrDomainNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestDiagnoseWorkflowExecution_Failed_ExecutionNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	result, err := wh.DiagnoseWorkflowExecution(context.Background(), &types.DiagnoseWorkflowExecutionRequest{
		Domain: testDomain,
	})
	s.Error(err)
	s.Equal(validate.ErrExecutionNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeat_Success() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	taskToken := common.TaskToken{
		DomainID:   s.testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "1",
	}
	taskTokenBytes, err := wh.tokenSerializer.Serialize(&taskToken)
	s.NoError(err)
	req := &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: taskTokenBytes,
		Details:   nil,
		Identity:  "",
	}
	resp := &types.RecordActivityTaskHeartbeatResponse{CancelRequested: false}

	s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
	s.mockHistoryClient.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(),
		&types.HistoryRecordActivityTaskHeartbeatRequest{
			DomainUUID:       s.testDomainID,
			HeartbeatRequest: req,
		}).Return(resp, nil)

	result, err := wh.RecordActivityTaskHeartbeat(context.Background(), req)
	s.NoError(err)
	s.Equal(resp, result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeat_RequestNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	result, err := wh.RecordActivityTaskHeartbeat(context.Background(), nil /*request is not set*/)

	s.Error(err)
	s.Equal(validate.ErrRequestNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeat_TaskTokenNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	result, err := wh.RecordActivityTaskHeartbeat(context.Background(), &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: nil, // task token is not set
		Details:   nil,
		Identity:  "",
	})

	s.Error(err)
	s.Equal(validate.ErrTaskTokenNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeatByID_Success() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	req := &types.RecordActivityTaskHeartbeatByIDRequest{
		Domain:     s.testDomain,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "1",
	}
	resp := &types.RecordActivityTaskHeartbeatResponse{CancelRequested: false}

	s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
	s.mockHistoryClient.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), gomock.Any()).Return(resp, nil)

	result, err := wh.RecordActivityTaskHeartbeatByID(context.Background(), req)
	s.NoError(err)
	s.Equal(resp, result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeatByID_RequestNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	result, err := wh.RecordActivityTaskHeartbeatByID(context.Background(), nil /*request is not set*/)

	s.Error(err)
	s.Equal(validate.ErrRequestNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestRecordActivityTaskHeartbeatByID_DomainNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	result, err := wh.RecordActivityTaskHeartbeatByID(
		context.Background(),
		&types.RecordActivityTaskHeartbeatByIDRequest{
			Domain: "", // domain not set
		})

	s.Error(err)
	s.Equal(validate.ErrDomainNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestRespondActivityTaskCompleted_Success() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	taskToken := common.TaskToken{
		DomainID:   s.testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "1",
	}
	taskTokenBytes, err := wh.tokenSerializer.Serialize(&taskToken)
	s.NoError(err)
	req := &types.RespondActivityTaskCompletedRequest{
		TaskToken: taskTokenBytes,
	}

	s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
	s.mockHistoryClient.EXPECT().RespondActivityTaskCompleted(gomock.Any(),
		&types.HistoryRespondActivityTaskCompletedRequest{
			DomainUUID:      taskToken.DomainID,
			CompleteRequest: req,
		}).Return(nil)

	err = wh.RespondActivityTaskCompleted(context.Background(), req)
	// only checking for successful write here
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestRespondActivityTaskCompletedByID_Success() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	req := &types.RespondActivityTaskCompletedByIDRequest{
		Domain:     s.testDomain,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "1",
	}

	s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
	s.mockHistoryClient.EXPECT().RespondActivityTaskCompleted(gomock.Any(), gomock.Any()).Return(nil)

	err := wh.RespondActivityTaskCompletedByID(context.Background(), req)
	// only checking for successful write here
	s.NoError(err)
}

func buildRespondActivityTaskFailedRequest(taskToken common.TaskToken) *types.RespondActivityTaskFailedRequest {
	serializer := common.NewJSONTaskTokenSerializer()
	taskTokenBytes, err := serializer.Serialize(&taskToken)
	if err != nil {
		panic(err)
	}
	return &types.RespondActivityTaskFailedRequest{
		TaskToken: taskTokenBytes,
	}
}

func TestRespondActivityTaskFailed(t *testing.T) {
	failedRequest := buildRespondActivityTaskFailedRequest(common.TaskToken{
		DomainID:   testDomainID,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "1",
	})

	type fields struct {
		shuttingDown int32
	}

	type args struct {
		ctx           context.Context
		failedRequest *types.RespondActivityTaskFailedRequest
	}

	tests := []struct {
		name       string
		fields     fields
		setupMocks func(*resource.Test, *client.MockVersionChecker)
		args       args
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "Success",
			fields: fields{
				shuttingDown: 0,
			},
			setupMocks: func(t *resource.Test, mockVersionChecker *client.MockVersionChecker) {
				t.HistoryClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), &types.HistoryRespondActivityTaskFailedRequest{
					DomainUUID:    testDomainID,
					FailedRequest: failedRequest,
				}).Return(nil)

				t.DomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-id", nil)
			},
			args: args{
				context.Background(),
				failedRequest,
			},
			wantErr: assert.NoError,
		},
		{
			name:   "Error when shutting down",
			fields: fields{shuttingDown: 1},
			setupMocks: func(t *resource.Test, mockVersionChecker *client.MockVersionChecker) {

			},
			args: args{
				context.Background(),
				buildRespondActivityTaskFailedRequest(common.TaskToken{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
					ActivityID: "1",
				}),
			},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)

			tt.setupMocks(mockResource, mockVersionChecker)

			mockProducerManager := NewMockProducerManager(mockCtrl)

			config := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)

			wh := NewWorkflowHandler(mockResource, config, mockVersionChecker, nil)
			wh.shuttingDown = tt.fields.shuttingDown
			wh.producerManager = mockProducerManager

			tt.wantErr(t, wh.RespondActivityTaskFailed(tt.args.ctx, tt.args.failedRequest),
				fmt.Sprintf("RespondActivityTaskFailed(%v, %v)", tt.args.ctx, tt.args.failedRequest))
		})
	}

}

func (s *workflowHandlerSuite) TestRegisterDomain_Failure_MissingDomainDataKey() {
	dynamicClient := dc.NewInMemoryClient()
	err := dynamicClient.UpdateValue(dc.RequiredDomainDataKeys, map[string]interface{}{"Tier": true})
	s.NoError(err)
	cfg := s.newConfig(dynamicClient)
	wh := s.getWorkflowHandler(cfg)

	req := registerDomainRequest(
		types.ArchivalStatusEnabled.Ptr(),
		testHistoryArchivalURI,
		types.ArchivalStatusEnabled.Ptr(),
		testVisibilityArchivalURI,
	)
	err = wh.RegisterDomain(context.Background(), req)
	s.Error(err)
	s.Contains(err.Error(), "domain data error, missing required key")
}

func (s *workflowHandlerSuite) TestRegisterDomain_Failure_InvalidArchivalURI() {
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{})
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(errors.New("invalid URI"))
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := registerDomainRequest(
		types.ArchivalStatusEnabled.Ptr(),
		testHistoryArchivalURI,
		types.ArchivalStatusEnabled.Ptr(),
		testVisibilityArchivalURI,
	)
	err := wh.RegisterDomain(context.Background(), req)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestRegisterDomain_Success_EnabledWithNoArchivalURI() {
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", testHistoryArchivalURI))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", testVisibilityArchivalURI))
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{})
	s.mockMetadataMgr.On("CreateDomain", mock.Anything, mock.Anything).Return(&persistence.CreateDomainResponse{
		ID: "test-id",
	}, nil)
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := registerDomainRequest(types.ArchivalStatusEnabled.Ptr(), "", types.ArchivalStatusEnabled.Ptr(), "")
	err := wh.RegisterDomain(context.Background(), req)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestRegisterDomain_Success_EnabledWithArchivalURI() {
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "invalidURI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "invalidURI"))
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{})
	s.mockMetadataMgr.On("CreateDomain", mock.Anything, mock.Anything).Return(&persistence.CreateDomainResponse{
		ID: "test-id",
	}, nil)
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := registerDomainRequest(
		types.ArchivalStatusEnabled.Ptr(),
		testHistoryArchivalURI,
		types.ArchivalStatusEnabled.Ptr(),
		testVisibilityArchivalURI,
	)
	err := wh.RegisterDomain(context.Background(), req)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestRegisterDomain_Success_ClusterNotConfiguredForArchival() {
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewDisabledArchvialConfig())
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{})
	s.mockMetadataMgr.On("CreateDomain", mock.Anything, mock.Anything).Return(&persistence.CreateDomainResponse{
		ID: "test-id",
	}, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := registerDomainRequest(
		types.ArchivalStatusEnabled.Ptr(),
		testVisibilityArchivalURI,
		types.ArchivalStatusEnabled.Ptr(),
		"invalidURI",
	)
	err := wh.RegisterDomain(context.Background(), req)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestRegisterDomain_Success_NotEnabled() {
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(nil, &types.EntityNotExistsError{})
	s.mockMetadataMgr.On("CreateDomain", mock.Anything, mock.Anything).Return(&persistence.CreateDomainResponse{
		ID: "test-id",
	}, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := registerDomainRequest(nil, "", nil, "")
	err := wh.RegisterDomain(context.Background(), req)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestListDomains_Success() {
	domain := persistenceGetDomainResponse(
		&domain.ArchivalState{},
		&domain.ArchivalState{},
	)
	listDomainResp := &persistence.ListDomainsResponse{
		Domains: []*persistence.GetDomainResponse{
			domain,
			domain,
		},
	}
	s.mockMetadataMgr.
		On("ListDomains", mock.Anything, mock.Anything).
		Return(listDomainResp, nil)
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	result, err := wh.ListDomains(context.Background(), &types.ListDomainsRequest{})
	s.NoError(err)

	s.Equal(2, len(result.GetDomains()))
}

func (s *workflowHandlerSuite) TestListDomains_RequestNotSet() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	result, err := wh.ListDomains(context.Background(), nil /* list request is not set */)
	s.Error(err)
	s.Equal(validate.ErrRequestNotSet, err)
	s.Nil(result)
}

func (s *workflowHandlerSuite) TestHealth_Status() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient())) // workflow handler gets initial health status as HealthStatusWarmingUp

	result, err := wh.Health(context.Background()) // Health check looks for HealthStatusOK
	s.NoError(err)
	s.False(result.Ok)
	s.Equal("WarmingUp", result.Msg)

	wh.UpdateHealthStatus(HealthStatusOK)
	result, err = wh.Health(context.Background())
	s.NoError(err)
	s.True(result.Ok)
	s.Equal("OK", result.Msg)

	wh.UpdateHealthStatus(HealthStatusShuttingDown)
	result, err = wh.Health(context.Background())
	s.NoError(err)
	s.False(result.Ok)
	s.Equal("ShuttingDown", result.Msg)

	wh.UpdateHealthStatus(HealthStatus(-1))
	result, err = wh.Health(context.Background())
	s.NoError(err)
	s.False(result.Ok)
	s.Equal("unknown", result.Msg)
}

func (s *workflowHandlerSuite) TestGetClusterInfo() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.GetClusterInfo(context.Background())
	s.NoError(err)
	s.Equal("1.7.0", resp.SupportedClientVersions.GoSdk)
	s.Equal("1.5.0", resp.SupportedClientVersions.JavaSdk)
}

func (s *workflowHandlerSuite) TestDescribeDomain_Success_ArchivalDisabled() {
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := &types.DescribeDomainRequest{
		Name: common.StringPtr(s.testDomain),
	}
	result, err := wh.DescribeDomain(context.Background(), req)

	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal("", result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal("", result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestDescribeDomain_Success_ArchivalEnabled() {
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testHistoryArchivalURI},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testVisibilityArchivalURI},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	req := &types.DescribeDomainRequest{
		Name: common.StringPtr(s.testDomain),
	}
	result, err := wh.DescribeDomain(context.Background(), req)

	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal(testHistoryArchivalURI, result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal(testVisibilityArchivalURI, result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Failure_UpdateExistingArchivalURI() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testHistoryArchivalURI},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testVisibilityArchivalURI},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		nil,
		nil,
		common.StringPtr("updated visibility URI"),
		nil,
	)
	_, err := wh.UpdateDomain(context.Background(), updateReq)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestUpdateDomain_Failure_InvalidArchivalURI() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(errors.New("invalid URI"))
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		common.StringPtr("testScheme://invalid/updated/history/URI"),
		types.ArchivalStatusEnabled.Ptr(),
		nil,
		nil,
	)

	_, err := wh.UpdateDomain(context.Background(), updateReq)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_ArchivalEnabledToArchivalDisabledWithoutSettingURI() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testHistoryArchivalURI},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testVisibilityArchivalURI},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, mock.Anything).Return(nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		nil,
		types.ArchivalStatusDisabled.Ptr(),
		nil,
		types.ArchivalStatusDisabled.Ptr(),
	)
	result, err := wh.UpdateDomain(context.Background(), updateReq)
	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal(testHistoryArchivalURI, result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal(testVisibilityArchivalURI, result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_ClusterNotConfiguredForArchival() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: "some random history URI"},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: "some random visibility URI"},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewDisabledArchvialConfig())
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(nil, types.ArchivalStatusDisabled.Ptr(), nil, nil)
	result, err := wh.UpdateDomain(context.Background(), updateReq)
	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal("some random history URI", result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal("some random visibility URI", result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_ArchivalEnabledToArchivalDisabledWithSettingBucket() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testHistoryArchivalURI},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testVisibilityArchivalURI},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, mock.Anything).Return(nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		common.StringPtr(testHistoryArchivalURI),
		types.ArchivalStatusDisabled.Ptr(),
		common.StringPtr(testVisibilityArchivalURI),
		types.ArchivalStatusDisabled.Ptr(),
	)
	result, err := wh.UpdateDomain(context.Background(), updateReq)
	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal(testHistoryArchivalURI, result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusDisabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal(testVisibilityArchivalURI, result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_ArchivalEnabledToEnabled() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testHistoryArchivalURI},
		&domain.ArchivalState{Status: types.ArchivalStatusEnabled, URI: testVisibilityArchivalURI},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		common.StringPtr(testHistoryArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.StringPtr(testVisibilityArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
	)
	result, err := wh.UpdateDomain(context.Background(), updateReq)
	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal(testHistoryArchivalURI, result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal(testVisibilityArchivalURI, result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_ArchivalNeverEnabledToEnabled() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponse(
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
	)
	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, mock.Anything).Return(nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "some random URI"))
	s.mockHistoryArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockVisibilityArchiver.On("ValidateURI", mock.Anything).Return(nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateRequest(
		common.StringPtr(testHistoryArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.StringPtr(testVisibilityArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
	)
	result, err := wh.UpdateDomain(context.Background(), updateReq)
	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetHistoryArchivalStatus())
	s.Equal(testHistoryArchivalURI, result.Configuration.GetHistoryArchivalURI())
	s.Equal(types.ArchivalStatusEnabled, result.Configuration.GetVisibilityArchivalStatus())
	s.Equal(testVisibilityArchivalURI, result.Configuration.GetVisibilityArchivalURI())
}

func (s *workflowHandlerSuite) TestUpdateDomain_Success_FailOver() {
	s.mockMetadataMgr.On("GetMetadata", mock.Anything).Return(&persistence.GetMetadataResponse{
		NotificationVersion: int64(0),
	}, nil)
	getDomainResp := persistenceGetDomainResponseForFailoverTest(
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
		&domain.ArchivalState{Status: types.ArchivalStatusDisabled, URI: ""},
	)

	// This test is simulating a domain failover from the point of view of the 'standby' cluster
	// for a domain where the cluster 'active' is being failed over to 'standby'. The test is executing
	// in the 'standby' cluster, so the above is setting the configuration to appear that way.
	s.mockResource.ClusterMetadata = cluster.TestPassiveClusterMetadata

	// Re-instantiate the domain-handler object due to it relying on it
	// pulling in the mock cluster metadata object mutated above.
	// Todo (David.Porter) consider refactoring these tests
	// to be setup without mutation and without as long dependency chains
	s.domainHandler = domain.NewHandler(
		s.newConfig(dc.NewInMemoryClient()).DomainConfig,
		s.mockResource.GetLogger(),
		s.mockResource.GetDomainManager(),
		s.mockResource.GetClusterMetadata(),
		domain.NewDomainReplicator(s.mockProducer, s.mockResource.GetLogger()),
		s.mockResource.GetArchivalMetadata(),
		s.mockResource.GetArchiverProvider(),
		s.mockResource.GetTimeSource(),
	)

	s.mockMetadataMgr.On("GetDomain", mock.Anything, mock.Anything).Return(getDomainResp, nil)
	s.mockMetadataMgr.On("UpdateDomain", mock.Anything, mock.Anything).Return(nil)
	s.mockArchivalMetadata.On("GetHistoryConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("disabled"), false, dc.GetBoolPropertyFn(false), "disabled", "some random URI"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("disabled"), false, dc.GetBoolPropertyFn(false), "disabled", "some random URI"))
	s.mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil).Once()
	s.mockResource.RemoteFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).
		Return(describeDomainResponseServer, nil).AnyTimes()

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	updateReq := updateFailoverRequest(
		common.StringPtr(testHistoryArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.StringPtr(testVisibilityArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.Int32Ptr(1),
		common.StringPtr(cluster.TestAlternativeClusterName),
	)
	result, err := wh.UpdateDomain(context.Background(), updateReq)

	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.Configuration)
	s.Equal(result.ReplicationConfiguration.ActiveClusterName, cluster.TestAlternativeClusterName)
}

func (s *workflowHandlerSuite) TestUpdateDomain_Failure_FailoverLockdown() {

	dynamicClient := dc.NewInMemoryClient()
	err := dynamicClient.UpdateValue(dc.Lockdown, true)
	s.NoError(err)
	wh := s.getWorkflowHandler(s.newConfig(dynamicClient))

	updateReq := updateFailoverRequest(
		common.StringPtr(testHistoryArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.StringPtr(testVisibilityArchivalURI),
		types.ArchivalStatusEnabled.Ptr(),
		common.Int32Ptr(1),
		common.StringPtr(cluster.TestAlternativeClusterName),
	)
	resp, err := wh.UpdateDomain(context.Background(), updateReq)
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestHistoryArchived() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	getHistoryRequest := &types.GetWorkflowExecutionHistoryRequest{}
	s.False(wh.historyArchived(context.Background(), getHistoryRequest, s.testDomain))

	getHistoryRequest = &types.GetWorkflowExecutionHistoryRequest{
		Execution: &types.WorkflowExecution{},
	}
	s.False(wh.historyArchived(context.Background(), getHistoryRequest, s.testDomain))

	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	getHistoryRequest = &types.GetWorkflowExecutionHistoryRequest{
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
	}
	s.False(wh.historyArchived(context.Background(), getHistoryRequest, s.testDomain))

	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{Message: "got archival indication error"}).Times(1)
	getHistoryRequest = &types.GetWorkflowExecutionHistoryRequest{
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
	}
	s.True(wh.historyArchived(context.Background(), getHistoryRequest, s.testDomain))

	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), gomock.Any()).Return(nil, errors.New("got non-archival indication error")).Times(1)
	getHistoryRequest = &types.GetWorkflowExecutionHistoryRequest{
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
	}
	s.False(wh.historyArchived(context.Background(), getHistoryRequest, s.testDomain))
}

func (s *workflowHandlerSuite) TestGetArchivedHistory_Failure_DomainCacheEntryError() {
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(nil, errors.New("error getting domain")).Times(1)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.getArchivedHistory(context.Background(), getHistoryRequest(nil), s.testDomainID)
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestGetArchivedHistory_Failure_ArchivalURIEmpty() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{
			HistoryArchivalStatus:    types.ArchivalStatusDisabled,
			HistoryArchivalURI:       "",
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
			VisibilityArchivalURI:    "",
		},
		"",
	)
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(domainEntry, nil).AnyTimes()

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.getArchivedHistory(context.Background(), getHistoryRequest(nil), s.testDomainID)
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestGetArchivedHistory_Failure_InvalidURI() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{
			HistoryArchivalStatus:    types.ArchivalStatusEnabled,
			HistoryArchivalURI:       "uri without scheme",
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
			VisibilityArchivalURI:    "",
		},
		"",
	)
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(domainEntry, nil).AnyTimes()

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.getArchivedHistory(context.Background(), getHistoryRequest(nil), s.testDomainID)
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestGetArchivedHistory_Success_GetFirstPage() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{
			HistoryArchivalStatus:    types.ArchivalStatusEnabled,
			HistoryArchivalURI:       testHistoryArchivalURI,
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
			VisibilityArchivalURI:    "",
		},
		"",
	)
	s.mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(domainEntry, nil).AnyTimes()

	nextPageToken := []byte{'1', '2', '3'}
	historyBatch1 := &types.History{
		Events: []*types.HistoryEvent{
			{ID: 1},
			{ID: 2},
		},
	}
	historyBatch2 := &types.History{
		Events: []*types.HistoryEvent{
			{ID: 3},
			{ID: 4},
			{ID: 5},
		},
	}
	history := &types.History{}
	history.Events = append(history.Events, historyBatch1.Events...)
	history.Events = append(history.Events, historyBatch2.Events...)
	s.mockHistoryArchiver.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(&archiver.GetHistoryResponse{
		NextPageToken:  nextPageToken,
		HistoryBatches: []*types.History{historyBatch1, historyBatch2},
	}, nil)
	s.mockArchiverProvider.On("GetHistoryArchiver", mock.Anything, mock.Anything).Return(s.mockHistoryArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.getArchivedHistory(context.Background(), getHistoryRequest(nil), s.testDomainID)
	s.NoError(err)
	s.NotNil(resp)
	s.NotNil(resp.History)
	s.Equal(history, resp.History)
	s.Equal(nextPageToken, resp.NextPageToken)
	s.True(resp.GetArchived())
}

func (s *workflowHandlerSuite) TestGetHistory() {
	domainID := uuid.New()
	domainName := uuid.New()
	firstEventID := int64(100)
	nextEventID := int64(101)
	branchToken := []byte{1}
	we := types.WorkflowExecution{
		WorkflowID: "wid",
		RunID:      "rid",
	}
	shardID := common.WorkflowIDToHistoryShard(we.WorkflowID, numHistoryShards)
	req := &persistence.ReadHistoryBranchRequest{
		BranchToken:   branchToken,
		MinEventID:    firstEventID,
		MaxEventID:    nextEventID,
		PageSize:      0,
		NextPageToken: []byte{},
		ShardID:       common.IntPtr(shardID),
		DomainName:    domainName,
	}
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, req).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{
			{
				ID: int64(100),
			},
		},
		NextPageToken:    []byte{},
		Size:             1,
		LastFirstEventID: nextEventID,
	}, nil).Once()

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	scope := metrics.NoopScope(metrics.Frontend)
	history, token, err := wh.getHistory(context.Background(), scope, domainID, domainName, we, firstEventID, nextEventID, 0, []byte{}, nil, branchToken)
	s.NoError(err)
	s.NotNil(history)
	s.Equal([]byte{}, token)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Failure_InvalidRequest() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), &types.ListArchivedWorkflowExecutionsRequest{})
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Failure_ClusterNotConfiguredForArchival() {
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewDisabledArchvialConfig())

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), listArchivedWorkflowExecutionsTestRequest())
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Failure_DomainCacheEntryError() {
	s.mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(nil, errors.New("error getting domain"))
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), listArchivedWorkflowExecutionsTestRequest())
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Failure_DomainNotConfiguredForArchival() {
	s.mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewLocalDomainCacheEntryForTest(
		nil,
		&persistence.DomainConfig{
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
		},
		"",
	), nil)
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), listArchivedWorkflowExecutionsTestRequest())
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Failure_InvalidURI() {
	s.mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
			VisibilityArchivalURI:    "uri without scheme",
		},
		"",
	), nil)
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), listArchivedWorkflowExecutionsTestRequest())
	s.Nil(resp)
	s.Error(err)
}

func (s *workflowHandlerSuite) TestListArchivedVisibility_Success() {
	s.mockDomainCache.EXPECT().GetDomain(gomock.Any()).Return(cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomain},
		&persistence.DomainConfig{
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    testVisibilityArchivalURI,
		},
		"",
	), nil).AnyTimes()
	s.mockArchivalMetadata.On("GetVisibilityConfig").Return(archiver.NewArchivalConfig("enabled", dc.GetStringPropertyFn("enabled"), true, dc.GetBoolPropertyFn(true), "disabled", "random URI"))
	s.mockVisibilityArchiver.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(&archiver.QueryVisibilityResponse{}, nil)
	s.mockArchiverProvider.On("GetVisibilityArchiver", mock.Anything, mock.Anything).Return(s.mockVisibilityArchiver, nil)

	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	resp, err := wh.ListArchivedWorkflowExecutions(context.Background(), listArchivedWorkflowExecutionsTestRequest())
	s.NotNil(resp)
	s.NoError(err)
}

func (s *workflowHandlerSuite) TestGetSearchAttributes() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	ctx := context.Background()
	resp, err := wh.GetSearchAttributes(ctx)
	s.NoError(err)
	s.NotNil(resp)
}

func (s *workflowHandlerSuite) TestGetWorkflowExecutionHistory__Success__RawHistoryEnabledTransientDecisionEmitted() {
	var nextEventID int64 = 5
	s.getWorkflowExecutionHistory(5, &types.TransientDecisionInfo{
		StartedEvent:   &types.HistoryEvent{ID: nextEventID + 1},
		ScheduledEvent: &types.HistoryEvent{ID: nextEventID},
	}, []*types.HistoryEvent{{}, {}, {}})
}

func (s *workflowHandlerSuite) TestGetWorkflowExecutionHistory__Success__RawHistoryEnabledNoTransientDecisionEmitted() {
	var nextEventID int64 = 5
	s.getWorkflowExecutionHistory(5, &types.TransientDecisionInfo{
		StartedEvent:   &types.HistoryEvent{ID: nextEventID + 1},
		ScheduledEvent: &types.HistoryEvent{ID: nextEventID},
	}, []*types.HistoryEvent{{}, {}, {}})
}

func (s *workflowHandlerSuite) TestRestartWorkflowExecution_IsolationGroupDrained() {
	dynamicClient := dc.NewInMemoryClient()
	err := dynamicClient.UpdateValue(dc.SendRawWorkflowHistory, false)
	s.NoError(err)
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableTasklistIsolation = dc.GetBoolPropertyFnFilteredByDomain(true)
	wh := s.getWorkflowHandler(config)
	isolationGroup := "dca1"
	ctx := isolationgroup.ContextWithIsolationGroup(context.Background(), isolationGroup)
	s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
	s.mockResource.IsolationGroups.EXPECT().IsDrained(gomock.Any(), s.testDomain, isolationGroup).Return(true, nil)
	_, err = wh.RestartWorkflowExecution(ctx, &types.RestartWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
		},
		Identity: "",
	})
	s.Error(err)
	s.IsType(err, &types.BadRequestError{})
}

func (s *workflowHandlerSuite) TestRestartWorkflowExecution__Success() {
	dynamicClient := dc.NewInMemoryClient()
	err := dynamicClient.UpdateValue(dc.SendRawWorkflowHistory, false)
	s.NoError(err)
	wh := s.getWorkflowHandler(
		frontendcfg.NewConfig(
			dc.NewCollection(
				dynamicClient,
				s.mockResource.GetLogger()),
			numHistoryShards,
			false,
			"hostname",
		),
	)
	ctx := context.Background()
	s.mockHistoryClient.EXPECT().PollMutableState(gomock.Any(), gomock.Any()).Return(&types.PollMutableStateResponse{
		CurrentBranchToken: []byte(""),
		Execution: &types.WorkflowExecution{
			WorkflowID: testRunID,
		},
		LastFirstEventID: 0,
		NextEventID:      2,
		VersionHistories: &types.VersionHistories{
			CurrentVersionHistoryIndex: 0,
			Histories: []*types.VersionHistory{
				{
					BranchToken: []byte("token"),
					Items: []*types.VersionHistoryItem{
						{
							EventID: 1,
							Version: 1,
						},
					},
				},
			},
		},
	}, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()
	s.mockVersionChecker.EXPECT().SupportsRawHistoryQuery(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadHistoryBranchResponse{
		HistoryEvents: []*types.HistoryEvent{&types.HistoryEvent{
			ID: 1,
			WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
				WorkflowType: &types.WorkflowType{
					Name: "workflowtype",
				},
				TaskList: &types.TaskList{
					Name: "tasklist",
				},
			},
		}},
	}, nil).Once()
	s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.StartWorkflowExecutionResponse{
		RunID: testRunID,
	}, nil)
	resp, err := wh.RestartWorkflowExecution(ctx, &types.RestartWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
		},
		Identity: "",
	})
	s.Equal(testRunID, resp.GetRunID())
	s.NoError(err)
}

func (s *workflowHandlerSuite) getWorkflowExecutionHistory(nextEventID int64, transientDecision *types.TransientDecisionInfo, historyEvents []*types.HistoryEvent) {
	dynamicClient := dc.NewInMemoryClient()
	err := dynamicClient.UpdateValue(dc.SendRawWorkflowHistory, true)
	s.NoError(err)
	wh := s.getWorkflowHandler(
		frontendcfg.NewConfig(
			dc.NewCollection(
				dynamicClient,
				s.mockResource.GetLogger()),
			numHistoryShards,
			false,
			"hostname",
		),
	)
	ctx := context.Background()
	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()
	s.mockVersionChecker.EXPECT().SupportsRawHistoryQuery(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	blob, _ := wh.GetPayloadSerializer().SerializeBatchEvents(historyEvents, constants.EncodingTypeThriftRW)
	s.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadRawHistoryBranchResponse{
		HistoryEventBlobs: []*persistence.DataBlob{blob},
		NextPageToken:     []byte{},
	}, nil).Once()
	token, _ := json.Marshal(&getHistoryContinuationToken{
		FirstEventID:      1,
		NextEventID:       nextEventID,
		RunID:             testRunID,
		TransientDecision: transientDecision,
	})
	resp, err := wh.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.testDomain,
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		SkipArchival:  true,
		NextPageToken: token,
	})
	s.NoError(err)
	s.NotNil(resp)
	s.NotNil(resp.RawHistory)
	s.Equal(2, len(resp.RawHistory))

	events := deserializeBlobDataToHistoryEvents(wh, resp.RawHistory)
	s.NotNil(events)
	if transientDecision != nil {
		s.Equal(len(historyEvents)+2, len(events))
	} else {
		s.Equal(len(historyEvents), len(events))
	}
}

func deserializeBlobDataToHistoryEvents(wh *WorkflowHandler, dataBlobs []*types.DataBlob) []*types.HistoryEvent {
	var historyEvents []*types.HistoryEvent
	for _, batch := range dataBlobs {
		events, err := wh.GetPayloadSerializer().DeserializeBatchEvents(&persistence.DataBlob{Data: batch.Data, Encoding: constants.EncodingTypeThriftRW})
		if err != nil {
			return nil
		}
		historyEvents = append(historyEvents, events...)
	}
	return historyEvents
}

func (s *workflowHandlerSuite) TestListWorkflowExecutions() {
	config := s.newConfig(dc.NewInMemoryClient())
	wh := s.getWorkflowHandler(config)

	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()
	s.mockVisibilityMgr.On("ListWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.ListWorkflowExecutionsResponse{}, nil).Once()

	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.testDomain,
		PageSize: int32(config.ESIndexMaxResultWindow()),
	}
	ctx := context.Background()

	query := "WorkflowID = 'wid'"
	listRequest.Query = query
	_, err := wh.ListWorkflowExecutions(ctx, listRequest)
	s.NoError(err)
	s.Equal(query, listRequest.GetQuery())

	query = "InvalidKey = 'a'"
	listRequest.Query = query
	_, err = wh.ListWorkflowExecutions(ctx, listRequest)
	s.NotNil(err)

	listRequest.PageSize = int32(config.ESIndexMaxResultWindow() + 1)
	_, err = wh.ListWorkflowExecutions(ctx, listRequest)
	s.NotNil(err)
}

func (s *workflowHandlerSuite) TestScantWorkflowExecutions() {
	config := s.newConfig(dc.NewInMemoryClient())
	wh := s.getWorkflowHandler(config)

	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()
	s.mockVisibilityMgr.On("ScanWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.ListWorkflowExecutionsResponse{}, nil).Once()

	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.testDomain,
		PageSize: int32(config.ESIndexMaxResultWindow()),
	}
	ctx := context.Background()

	query := "WorkflowID = 'wid'"
	listRequest.Query = query
	_, err := wh.ScanWorkflowExecutions(ctx, listRequest)
	s.NoError(err)
	s.Equal(query, listRequest.GetQuery())

	query = "InvalidKey = 'a'"
	listRequest.Query = query
	_, err = wh.ScanWorkflowExecutions(ctx, listRequest)
	s.NotNil(err)

	listRequest.PageSize = int32(config.ESIndexMaxResultWindow() + 1)
	_, err = wh.ListWorkflowExecutions(ctx, listRequest)
	s.NotNil(err)
}

func (s *workflowHandlerSuite) TestCountWorkflowExecutions() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))

	s.mockDomainCache.EXPECT().GetDomainID(gomock.Any()).Return(s.testDomainID, nil).AnyTimes()
	s.mockVisibilityMgr.On("CountWorkflowExecutions", mock.Anything, mock.Anything).Return(&persistence.CountWorkflowExecutionsResponse{}, nil).Once()

	countRequest := &types.CountWorkflowExecutionsRequest{
		Domain: s.testDomain,
	}
	ctx := context.Background()

	query := "WorkflowID = 'wid'"
	countRequest.Query = query
	_, err := wh.CountWorkflowExecutions(ctx, countRequest)
	s.NoError(err)
	s.Equal(query, countRequest.GetQuery())

	query = "InvalidKey = 'a'"
	countRequest.Query = query
	_, err = wh.CountWorkflowExecutions(ctx, countRequest)
	s.NotNil(err)
}

func (s *workflowHandlerSuite) TestConvertIndexedKeyToThrift() {
	wh := s.getWorkflowHandler(s.newConfig(dc.NewInMemoryClient()))
	m := map[string]interface{}{
		"key1":  float64(0),
		"key2":  float64(1),
		"key3":  float64(2),
		"key4":  float64(3),
		"key5":  float64(4),
		"key6":  float64(5),
		"key1i": 0,
		"key2i": 1,
		"key3i": 2,
		"key4i": 3,
		"key5i": 4,
		"key6i": 5,
		"key1t": types.IndexedValueTypeString,
		"key2t": types.IndexedValueTypeKeyword,
		"key3t": types.IndexedValueTypeInt,
		"key4t": types.IndexedValueTypeDouble,
		"key5t": types.IndexedValueTypeBool,
		"key6t": types.IndexedValueTypeDatetime,
		"key1s": "STRING",
		"key2s": "KEYWORD",
		"key3s": "INT",
		"key4s": "DOUBLE",
		"key5s": "BOOL",
		"key6s": "DATETIME",
	}
	result := wh.convertIndexedKeyToThrift(m)
	s.Equal(types.IndexedValueTypeString, result["key1"])
	s.Equal(types.IndexedValueTypeKeyword, result["key2"])
	s.Equal(types.IndexedValueTypeInt, result["key3"])
	s.Equal(types.IndexedValueTypeDouble, result["key4"])
	s.Equal(types.IndexedValueTypeBool, result["key5"])
	s.Equal(types.IndexedValueTypeDatetime, result["key6"])
	s.Equal(types.IndexedValueTypeString, result["key1i"])
	s.Equal(types.IndexedValueTypeKeyword, result["key2i"])
	s.Equal(types.IndexedValueTypeInt, result["key3i"])
	s.Equal(types.IndexedValueTypeDouble, result["key4i"])
	s.Equal(types.IndexedValueTypeBool, result["key5i"])
	s.Equal(types.IndexedValueTypeDatetime, result["key6i"])
	s.Equal(types.IndexedValueTypeString, result["key1t"])
	s.Equal(types.IndexedValueTypeKeyword, result["key2t"])
	s.Equal(types.IndexedValueTypeInt, result["key3t"])
	s.Equal(types.IndexedValueTypeDouble, result["key4t"])
	s.Equal(types.IndexedValueTypeBool, result["key5t"])
	s.Equal(types.IndexedValueTypeDatetime, result["key6t"])
	s.Equal(types.IndexedValueTypeString, result["key1s"])
	s.Equal(types.IndexedValueTypeKeyword, result["key2s"])
	s.Equal(types.IndexedValueTypeInt, result["key3s"])
	s.Equal(types.IndexedValueTypeDouble, result["key4s"])
	s.Equal(types.IndexedValueTypeBool, result["key5s"])
	s.Equal(types.IndexedValueTypeDatetime, result["key6s"])
	s.Panics(func() {
		wh.convertIndexedKeyToThrift(map[string]interface{}{
			"invalidType": "unknown",
		})
	})
}

func (s *workflowHandlerSuite) TestVerifyHistoryIsComplete() {
	events := make([]*types.HistoryEvent, 50)
	for i := 0; i < len(events); i++ {
		events[i] = &types.HistoryEvent{ID: int64(i + 1)}
	}
	var eventsWithHoles []*types.HistoryEvent
	eventsWithHoles = append(eventsWithHoles, events[9:12]...)
	eventsWithHoles = append(eventsWithHoles, events[20:31]...)

	testCases := []struct {
		events       []*types.HistoryEvent
		firstEventID int64
		lastEventID  int64
		isFirstPage  bool
		isLastPage   bool
		pageSize     int
		isResultErr  bool
	}{
		{events[:1], 1, 1, true, true, 1000, false},
		{events[:5], 1, 5, true, true, 1000, false},
		{events[9:31], 10, 31, true, true, 1000, false},
		{events[9:29], 10, 50, true, false, 20, false},
		{events[9:30], 10, 50, true, false, 20, false},

		{events[9:29], 1, 50, false, false, 20, false},
		{events[9:29], 1, 29, false, true, 20, false},

		{eventsWithHoles, 1, 50, false, false, 22, true},
		{eventsWithHoles, 10, 50, true, false, 22, true},
		{eventsWithHoles, 1, 31, false, true, 22, true},
		{eventsWithHoles, 10, 31, true, true, 1000, true},

		{events[9:31], 9, 31, true, true, 1000, true},
		{events[9:31], 9, 50, true, false, 22, true},
		{events[9:31], 11, 31, true, true, 1000, true},
		{events[9:31], 11, 50, true, false, 22, true},

		{events[9:31], 10, 30, true, true, 1000, true},
		{events[9:31], 1, 30, false, true, 22, true},
		{events[9:31], 10, 32, true, true, 1000, true},
		{events[9:31], 1, 32, false, true, 22, true},
	}

	for i, tc := range testCases {
		err := verifyHistoryIsComplete(tc.events, tc.firstEventID, tc.lastEventID, tc.isFirstPage, tc.isLastPage, tc.pageSize)
		if tc.isResultErr {
			s.Error(err, "testcase %v failed", i)
		} else {
			s.NoError(err, "testcase %v failed", i)
		}
	}
}

func (s *workflowHandlerSuite) newConfig(dynamicClient dc.Client) *frontendcfg.Config {
	config := frontendcfg.NewConfig(
		dc.NewCollection(
			dynamicClient,
			s.mockResource.GetLogger(),
		),
		numHistoryShards,
		false,
		"hostname",
	)
	config.EmitSignalNameMetricsTag = dc.GetBoolPropertyFnFilteredByDomain(true)
	return config
}

func (s *workflowHandlerSuite) TestRespondActivityTaskFailedByID() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.RespondActivityTaskFailedByIDRequest{
		Domain:     s.testDomain,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "activityID",
		Identity:   "identity",
		Details:    make([]byte, 1000),
	}

	testInput := map[string]struct {
		request     *types.RespondActivityTaskFailedByIDRequest
		expectError bool
		mockFn      func()
	}{
		"shutting down": {
			request: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError: true,
		},
		"nil request": {
			request:     nil,
			mockFn:      func() {},
			expectError: true,
		},
		"empty domain": {
			request: &types.RespondActivityTaskFailedByIDRequest{
				Domain: "",
			},
			mockFn:      func() {},
			expectError: true,
		},
		"cannot get domain ID": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"empty domain ID": {
			request: &types.RespondActivityTaskFailedByIDRequest{
				Domain:     s.testDomain,
				WorkflowID: "",
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", nil)
			},
			expectError: true,
		},
		"empty workflow ID": {
			request: &types.RespondActivityTaskFailedByIDRequest{
				Domain:     s.testDomain,
				WorkflowID: "",
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
			},
			expectError: true,
		},
		"empty activity ID": {
			request: &types.RespondActivityTaskFailedByIDRequest{
				Domain:     s.testDomain,
				WorkflowID: testWorkflowID,
				ActivityID: "",
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
			},
			expectError: true,
		},
		"exceeds id length limit": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1)
				wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError: true,
		},
		"serialzation failure": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"return exceeds blob size limit": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(make([]byte, 100), nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1)
				s.mockHistoryClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		"history client returns error": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(make([]byte, 100), nil)
				s.mockHistoryClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.RespondActivityTaskFailedByID(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1000)
			wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1000)

		})
	}
}

func (s *workflowHandlerSuite) TestRespondActivityTaskCanceled() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validInput := &types.RespondActivityTaskCanceledRequest{
		TaskToken: []byte("token"),
		Identity:  "identity",
		Details:   make([]byte, 1000),
	}

	testInput := map[string]struct {
		input       *types.RespondActivityTaskCanceledRequest
		mockFn      func()
		expectError bool
	}{
		"shutting down": {
			input: validInput,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError: true,
		},
		"nil request": {
			input:       nil,
			mockFn:      func() {},
			expectError: true,
		},
		"empty task token": {
			input: &types.RespondActivityTaskCanceledRequest{
				TaskToken: nil,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"deserialzation failure": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"empty domain ID": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: ""}, nil)
			},
			expectError: true,
		},
		"cannot get domain name": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return("", errors.New("error getting domain name"))
			},
			expectError: true,
		},
		"exceeds id length limit": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1)
				wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError: true,
		},
		"exceeds blob size limit": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1)
				s.mockHistoryClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"history client returns error": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockHistoryClient.EXPECT().RespondActivityTaskCanceled(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"no error": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockHistoryClient.EXPECT().RespondActivityTaskCanceled(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.RespondActivityTaskCanceled(context.Background(), input.input)
			if input.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1000)
			wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1000)

		})
	}
}

func (s *workflowHandlerSuite) TestRespondActivityTaskCanceledByID() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validInput := &types.RespondActivityTaskCanceledByIDRequest{
		Domain:     s.testDomain,
		WorkflowID: testWorkflowID,
		RunID:      testRunID,
		ActivityID: "activityID",
		Identity:   "identity",
		Details:    make([]byte, 1000),
	}

	testInput := map[string]struct {
		request     *types.RespondActivityTaskCanceledByIDRequest
		expectError bool
		mockFn      func()
	}{
		"shutting down": {
			request: validInput,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError: true,
		},
		"nil request": {
			request:     nil,
			mockFn:      func() {},
			expectError: true,
		},
		"empty domain name": {
			request: &types.RespondActivityTaskCanceledByIDRequest{
				Domain: "",
			},
			mockFn:      func() {},
			expectError: true,
		},
		"cannot get domain ID": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"empty domain ID": {
			request: &types.RespondActivityTaskCanceledByIDRequest{
				Domain: s.testDomain,
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", nil)
			},
			expectError: true,
		},
		"empty workflow ID": {
			request: &types.RespondActivityTaskCanceledByIDRequest{
				Domain:     s.testDomain,
				WorkflowID: "",
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
			},
			expectError: true,
		},
		"empty activity ID": {
			request: &types.RespondActivityTaskCanceledByIDRequest{
				Domain:     s.testDomain,
				WorkflowID: testWorkflowID,
				ActivityID: "",
			},
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
			},
			expectError: true,
		},
		"exceeds id length limit": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1)
				wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError: true,
		},
		"serialization failure": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"exceeds blob size limit": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(make([]byte, 100), nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(10)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(10)
				s.mockHistoryClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"history client returns error": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(make([]byte, 5), nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockHistoryClient.EXPECT().RespondActivityTaskCanceled(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"success": {
			request: validInput,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return(make([]byte, 5), nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockHistoryClient.EXPECT().RespondActivityTaskCanceled(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.RespondActivityTaskCanceledByID(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1000)
			wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1000)
		})
	}
}

func (s *workflowHandlerSuite) TestRespondDecisionTaskCompleted() {
	validRequest := &types.RespondDecisionTaskCompletedRequest{
		TaskToken: []byte("token"),
		Identity:  "identity",
		Decisions: make([]*types.Decision, 100),
	}
	mockResp := &types.HistoryRespondDecisionTaskCompletedResponse{
		StartedResponse: &types.RecordDecisionTaskStartedResponse{
			Attempt:          1,
			ScheduledEventID: 2,
		},
	}
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	testInput := map[string]struct {
		input           *types.RespondDecisionTaskCompletedRequest
		mockFn          func()
		expectError     bool
		expectErrorType error
	}{
		"shutting down": {
			input: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			input:           nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"nil task token": {
			input: &types.RespondDecisionTaskCompletedRequest{
				TaskToken: nil,
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrTaskTokenNotSet,
		},
		"deserialization failure": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"empty domain ID": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: ""}, nil)
			},
			expectError:     true,
			expectErrorType: validate.ErrDomainNotSet,
		},
		"cannot get domain name": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return("", errors.New("error getting domain name"))
			},
			expectError: true,
		},
		"exceeds id length limit": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1)
				wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrIdentityTooLong,
		},
		"exceeds decision size limit": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.DecisionResultCountLimit = dc.GetIntPropertyFilteredByDomain(10)
			},
			expectError: true,
		},
		"history client returns error": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				s.mockHistoryClient.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
			},
			expectError: true,
		},
		"no error": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				s.mockHistoryClient.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any()).Return(mockResp, nil)
			},
			expectError: false,
		},
		"return new decision task true": {
			input: &types.RespondDecisionTaskCompletedRequest{
				TaskToken:             []byte("token"),
				Identity:              "identity",
				Decisions:             make([]*types.Decision, 100),
				ReturnNewDecisionTask: true,
			},
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockTokenSerializer.EXPECT().Serialize(gomock.Any()).Return([]byte("new task token"), nil)
				s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.testDomain, nil).Times(2)
				s.mockHistoryClient.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), gomock.Any()).Return(mockResp, nil)
				s.mockHistoryV2Mgr.On("ReadHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadHistoryBranchResponse{
					HistoryEvents:    []*types.HistoryEvent{},
					NextPageToken:    []byte{},
					Size:             0,
					LastFirstEventID: 1,
				}, nil).Once()
			},
			expectError: false,
		},
	}
	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			_, err := wh.RespondDecisionTaskCompleted(context.Background(), input.input)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1000)
			wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1000)
			wh.config.DecisionResultCountLimit = dc.GetIntPropertyFilteredByDomain(1000)
		})
	}
}

func (s *workflowHandlerSuite) TestRespondDecisionTaskFailed() {
	validRequest := &types.RespondDecisionTaskFailedRequest{
		TaskToken: []byte("token"),
		Cause:     types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
		Identity:  "identity",
		Details:   make([]byte, 1000),
	}
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	testInput := map[string]struct {
		input           *types.RespondDecisionTaskFailedRequest
		mockFn          func()
		expectError     bool
		expectErrorType error
	}{
		"shutting down": {
			input: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			input:           nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"nil task token": {
			input: &types.RespondDecisionTaskFailedRequest{
				TaskToken: nil,
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrTaskTokenNotSet,
		},
		"deserialization failure": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"empty domain ID": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: ""}, nil)
			},
			expectError:     true,
			expectErrorType: validate.ErrDomainNotSet,
		},
		"cannot get domain name": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return("", errors.New("error getting domain name"))
			},
			expectError: true,
		},
		"exceeds id length limit": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1)
				wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrIdentityTooLong,
		},
		"exceeds blob size limit": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1)
				s.mockHistoryClient.EXPECT().RespondDecisionTaskFailed(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		"history client returns error": {
			input: validRequest,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().Deserialize(gomock.Any()).Return(&common.TaskToken{DomainID: s.testDomainID}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockHistoryClient.EXPECT().RespondDecisionTaskFailed(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.RespondDecisionTaskFailed(context.Background(), input.input)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.MaxIDLengthWarnLimit = dc.GetIntPropertyFn(1000)
			wh.config.IdentityMaxLength = dc.GetIntPropertyFilteredByDomain(1000)
		})
	}
}

func (s *workflowHandlerSuite) TestRespondQueryTaskCompleted() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validInput := &types.RespondQueryTaskCompletedRequest{
		TaskToken:   []byte("token"),
		QueryResult: []byte(`{"result": "result"}`),
	}

	testInput := map[string]struct {
		input           *types.RespondQueryTaskCompletedRequest
		mockFn          func()
		expectError     bool
		expectErrorType error
	}{
		"shutting down": {
			input: validInput,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			input:           nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"empty task token": {
			input: &types.RespondQueryTaskCompletedRequest{
				TaskToken: nil,
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrTaskTokenNotSet,
		},
		"deserialzation failure": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().DeserializeQueryTaskToken(gomock.Any()).Return(nil, errors.New("failed to deserialize token"))
			},
			expectError: true,
		},
		"empty domain ID": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().DeserializeQueryTaskToken(gomock.Any()).Return(&common.QueryTaskToken{DomainID: ""}, nil)
			},
			expectError:     true,
			expectErrorType: validate.ErrInvalidTaskToken,
		},
		"cannot get domain name": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().DeserializeQueryTaskToken(gomock.Any()).Return(&common.QueryTaskToken{
					DomainID: s.testDomainID,
					TaskList: "tasklist",
					TaskID:   "taskID"}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return("", errors.New("error getting domain name"))
			},
			expectError: true,
		},
		"exceed blob size limit and success": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().DeserializeQueryTaskToken(gomock.Any()).Return(&common.QueryTaskToken{
					DomainID: s.testDomainID,
					TaskList: "tasklist",
					TaskID:   "taskID"}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1)
				s.mockMatchingClient.EXPECT().RespondQueryTaskCompleted(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
		"matching client returns error": {
			input: validInput,
			mockFn: func() {
				s.mockTokenSerializer.EXPECT().DeserializeQueryTaskToken(gomock.Any()).Return(&common.QueryTaskToken{
					DomainID: s.testDomainID,
					TaskList: "tasklist",
					TaskID:   "taskID"}, nil)
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return(s.testDomain, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
				s.mockMatchingClient.EXPECT().RespondQueryTaskCompleted(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.RespondQueryTaskCompleted(context.Background(), input.input)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
		})
	}
}

func (s *workflowHandlerSuite) TestStartWorkflowExecution_Remaining() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.StartWorkflowExecutionRequest{
		Domain:     s.testDomain,
		WorkflowID: "wid",
		RequestID:  testRunID,
		WorkflowType: &types.WorkflowType{
			Name: "wType",
		},
		TaskList: &types.TaskList{
			Name: "tasklist",
		},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(100),
	}
	testInput := map[string]struct {
		Request         *types.StartWorkflowExecutionRequest
		MockFn          func()
		ExpectError     bool
		ExpectErrorType error
	}{
		"shutting down": {
			Request: validRequest,
			MockFn: func() {
				wh.shuttingDown = int32(1)
			},
			ExpectError:     true,
			ExpectErrorType: validate.ErrShuttingDown,
		},
		"cannot get domain ID": {
			Request: validRequest,
			MockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("domainid", nil)
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			ExpectError: true,
		},
		"history client returns error": {
			Request: validRequest,
			MockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
			},
			ExpectError: true,
		},
		"success": {
			Request: validRequest,
			MockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			ExpectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.MockFn()
			_, err := wh.StartWorkflowExecution(context.Background(), input.Request)
			if input.ExpectError {
				s.Error(err)
				if input.ExpectErrorType != nil {
					s.Equal(input.ExpectErrorType, err)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
		})

	}
}

func (s *workflowHandlerSuite) TestSignalWorkflowExecution() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.SignalWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		SignalName: "signal",
		RequestID:  testRunID,
		Input:      make([]byte, 1000),
	}

	testInput := map[string]struct {
		request         *types.SignalWorkflowExecutionRequest
		expectError     bool
		mockFn          func()
		expectErrorType error
	}{
		"shutting down": {
			request: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			request:         nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"empty domain": {
			request: &types.SignalWorkflowExecutionRequest{
				Domain: "",
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrDomainNotSet,
		},
		"empty workflow ID": {
			request: &types.SignalWorkflowExecutionRequest{
				Domain: s.testDomain,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "",
				},
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrWorkflowIDNotSet,
		},
		"domain length exceeds limit": {
			request: validRequest,
			mockFn: func() {
				wh.config.DomainNameMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrDomainTooLong,
		},
		"empty signal name": {
			request: &types.SignalWorkflowExecutionRequest{
				Domain: s.testDomain,
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				},
				SignalName: "",
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrSignalNameNotSet,
		},
		"signal name length exceeds limit": {
			request: validRequest,
			mockFn: func() {
				wh.config.SignalNameMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrSignalNameTooLong,
		},
		"requestID length exceeds limit": {
			request: validRequest,
			mockFn: func() {
				wh.config.RequestIDMaxLength = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrRequestIDTooLong,
		},
		"cannot get domain ID": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"input exceeds blob size limit": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1)
				wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1)
			},
			expectError: true,
		},
		"history client returns error": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"success": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.SignalWorkflowExecution(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
			wh.config.DomainNameMaxLength = dc.GetIntPropertyFilteredByDomain(200)
			wh.config.SignalNameMaxLength = dc.GetIntPropertyFilteredByDomain(200)
			wh.config.RequestIDMaxLength = dc.GetIntPropertyFilteredByDomain(200)
			wh.config.BlobSizeLimitWarn = dc.GetIntPropertyFilteredByDomain(1000)
			wh.config.BlobSizeLimitError = dc.GetIntPropertyFilteredByDomain(1000)
		})
	}
}

func updateRequest(
	historyArchivalURI *string,
	historyArchivalStatus *types.ArchivalStatus,
	visibilityArchivalURI *string,
	visibilityArchivalStatus *types.ArchivalStatus,
) *types.UpdateDomainRequest {
	return &types.UpdateDomainRequest{
		Name:                     "test-name",
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
	}
}

func updateFailoverRequest(
	historyArchivalURI *string,
	historyArchivalStatus *types.ArchivalStatus,
	visibilityArchivalURI *string,
	visibilityArchivalStatus *types.ArchivalStatus,
	failoverTimeoutInSeconds *int32,
	activeClusterName *string,
) *types.UpdateDomainRequest {
	return &types.UpdateDomainRequest{
		Name:                     "test-name",
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
		FailoverTimeoutInSeconds: failoverTimeoutInSeconds,
		ActiveClusterName:        activeClusterName,
	}
}

func persistenceGetDomainResponse(historyArchivalState, visibilityArchivalState *domain.ArchivalState) *persistence.GetDomainResponse {
	return &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:          "test-id",
			Name:        "test-name",
			Status:      0,
			Description: "test-description",
			OwnerEmail:  "test-owner-email",
			Data:        make(map[string]string),
		},
		Config: &persistence.DomainConfig{
			Retention:                1,
			EmitMetric:               true,
			HistoryArchivalStatus:    historyArchivalState.Status,
			HistoryArchivalURI:       historyArchivalState.URI,
			VisibilityArchivalStatus: visibilityArchivalState.Status,
			VisibilityArchivalURI:    visibilityArchivalState.URI,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{
					ClusterName: cluster.TestCurrentClusterName,
				},
			},
		},
		IsGlobalDomain:              false,
		ConfigVersion:               0,
		FailoverVersion:             0,
		FailoverNotificationVersion: 0,
		NotificationVersion:         0,
	}
}

func persistenceGetDomainResponseForFailoverTest(historyArchivalState, visibilityArchivalState *domain.ArchivalState) *persistence.GetDomainResponse {
	return &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:          "test-id",
			Name:        "test-name",
			Status:      0,
			Description: "test-description",
			OwnerEmail:  "test-owner-email",
			Data:        make(map[string]string),
		},
		Config: &persistence.DomainConfig{
			Retention:                1,
			EmitMetric:               true,
			HistoryArchivalStatus:    historyArchivalState.Status,
			HistoryArchivalURI:       historyArchivalState.URI,
			VisibilityArchivalStatus: visibilityArchivalState.Status,
			VisibilityArchivalURI:    visibilityArchivalState.URI,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{
					ClusterName: cluster.TestAlternativeClusterName,
				},
			},
		},
		IsGlobalDomain:              true,
		ConfigVersion:               0,
		FailoverVersion:             0,
		FailoverNotificationVersion: 0,
		NotificationVersion:         0,
	}
}

func registerDomainRequest(
	historyArchivalStatus *types.ArchivalStatus,
	historyArchivalURI string,
	visibilityArchivalStatus *types.ArchivalStatus,
	visibilityArchivalURI string,
) *types.RegisterDomainRequest {
	return &types.RegisterDomainRequest{
		Name:                                   "test-domain",
		Description:                            "test-description",
		OwnerEmail:                             "test-owner-email",
		WorkflowExecutionRetentionPeriodInDays: 10,
		EmitMetric:                             common.BoolPtr(true),
		Clusters: []*types.ClusterReplicationConfiguration{
			{
				ClusterName: cluster.TestCurrentClusterName,
			},
		},
		ActiveClusterName:        cluster.TestCurrentClusterName,
		Data:                     make(map[string]string),
		SecurityToken:            "token",
		HistoryArchivalStatus:    historyArchivalStatus,
		HistoryArchivalURI:       historyArchivalURI,
		VisibilityArchivalStatus: visibilityArchivalStatus,
		VisibilityArchivalURI:    visibilityArchivalURI,
		IsGlobalDomain:           false,
	}
}

func getHistoryRequest(nextPageToken []byte) *types.GetWorkflowExecutionHistoryRequest {
	return &types.GetWorkflowExecutionHistoryRequest{
		Execution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		NextPageToken: nextPageToken,
	}
}

func listArchivedWorkflowExecutionsTestRequest() *types.ListArchivedWorkflowExecutionsRequest {
	return &types.ListArchivedWorkflowExecutionsRequest{
		Domain:   "some random domain name",
		PageSize: 10,
		Query:    "some random query string",
	}
}

var describeDomainResponseServer = &types.DescribeDomainResponse{
	DomainInfo: &types.DomainInfo{
		Name:        "test-domain",
		Description: "a test domain",
		OwnerEmail:  "test@uber.com",
	},
	Configuration: &types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: 3,
		EmitMetric:                             true,
	},
	ReplicationConfiguration: &types.DomainReplicationConfiguration{
		ActiveClusterName: cluster.TestCurrentClusterName,
		Clusters: []*types.ClusterReplicationConfiguration{
			{
				ClusterName: cluster.TestCurrentClusterName,
			},
			{
				ClusterName: cluster.TestAlternativeClusterName,
			},
		},
	},
}

func TestStartWorkflowExecutionAsync(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func(*MockProducerManager)
		request    *types.StartWorkflowExecutionAsyncRequest
		wantErr    bool
	}{
		{
			name: "Success case",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockProducer := &mocks.KafkaProducer{}
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(mockProducer, nil)
				mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			request: &types.StartWorkflowExecutionAsyncRequest{
				StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
				},
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to get async queue producer",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(nil, errors.New("test-error"))
			},
			request: &types.StartWorkflowExecutionAsyncRequest{
				StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
				},
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to publish message",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockProducer := &mocks.KafkaProducer{}
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(mockProducer, nil)
				mockProducer.On("Publish", mock.Anything, mock.Anything).Return(errors.New("test-error"))
			},
			request: &types.StartWorkflowExecutionAsyncRequest{
				StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)
			mockProducerManager := NewMockProducerManager(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)
			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.producerManager = mockProducerManager

			tc.setupMocks(mockProducerManager)

			_, err := wh.StartWorkflowExecutionAsync(context.Background(), tc.request)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSignalWithStartWorkflowExecutionAsync(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func(*MockProducerManager)
		request    *types.SignalWithStartWorkflowExecutionAsyncRequest
		wantErr    bool
	}{
		{
			name: "Success case",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockProducer := &mocks.KafkaProducer{}
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(mockProducer, nil)
				mockProducer.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			request: &types.SignalWithStartWorkflowExecutionAsyncRequest{
				SignalWithStartWorkflowExecutionRequest: &types.SignalWithStartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
					SignalName:                          "test-signal-name",
				},
			},
			wantErr: false,
		},
		{
			name: "Error case - failed to get async queue producer",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(nil, errors.New("test-error"))
			},
			request: &types.SignalWithStartWorkflowExecutionAsyncRequest{
				SignalWithStartWorkflowExecutionRequest: &types.SignalWithStartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
					SignalName:                          "test-signal-name",
				},
			},
			wantErr: true,
		},
		{
			name: "Error case - failed to publish message",
			setupMocks: func(mockQueue *MockProducerManager) {
				mockProducer := &mocks.KafkaProducer{}
				mockQueue.EXPECT().GetProducerByDomain(gomock.Any()).Return(mockProducer, nil)
				mockProducer.On("Publish", mock.Anything, mock.Anything).Return(errors.New("test-error"))
			},
			request: &types.SignalWithStartWorkflowExecutionAsyncRequest{
				SignalWithStartWorkflowExecutionRequest: &types.SignalWithStartWorkflowExecutionRequest{
					Domain:     "test-domain",
					WorkflowID: "test-workflow-id",
					WorkflowType: &types.WorkflowType{
						Name: "test-workflow-type",
					},
					TaskList: &types.TaskList{
						Name: "test-task-list",
					},
					Input:                               []byte("test-input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "test-identity",
					RequestID:                           uuid.New(),
					SignalName:                          "test-signal-name",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)
			mockProducerManager := NewMockProducerManager(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)
			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.producerManager = mockProducerManager

			tc.setupMocks(mockProducerManager)

			_, err := wh.SignalWithStartWorkflowExecutionAsync(context.Background(), tc.request)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequestCancelWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name          string
		setupMocks    func(checkerMock *client.MockVersionChecker, mockResource *resource.Test)
		cancelRequest *types.RequestCancelWorkflowExecutionRequest
		shuttingDown  int32
		wantErr       bool
		err           error
	}{
		{
			name: "Success case",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				mockResource.HistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			cancelRequest: &types.RequestCancelWorkflowExecutionRequest{
				Domain: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
		},
		{
			name:         "Error case - is shutting down",
			setupMocks:   func(_ *client.MockVersionChecker, _ *resource.Test) {},
			shuttingDown: 1,
			err:          validate.ErrShuttingDown,
		},
		{
			name: "Error case - error request not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			err: validate.ErrRequestNotSet,
		},
		{
			name: "Error case - domain name not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			cancelRequest: &types.RequestCancelWorkflowExecutionRequest{},
			err:           validate.ErrDomainNotSet,
		},
		{
			name: "Error case - check execution error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			cancelRequest: &types.RequestCancelWorkflowExecutionRequest{
				Domain: "test-domain",
			},
			err: validate.ErrExecutionNotSet,
		},
		{
			name: "Error case - get domain ID error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("", errors.New("get-domain-id-error")).Times(1)
			},
			cancelRequest: &types.RequestCancelWorkflowExecutionRequest{
				Domain: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: errors.New("get-domain-id-error"),
		},
		{
			name: "Error case - RequestCancelWorkflowExecution error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				mockResource.HistoryClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(errors.New("request-cancel-workflow-execution-error")).Times(1)
			},
			cancelRequest: &types.RequestCancelWorkflowExecutionRequest{
				Domain: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: errors.New("request-cancel-workflow-execution-error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)
			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.shuttingDown = tc.shuttingDown

			tc.setupMocks(mockVersionChecker, mockResource)

			err := wh.RequestCancelWorkflowExecution(context.Background(), tc.cancelRequest)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestQueryWorkflow(t *testing.T) {
	testCases := []struct {
		name           string
		setupMocks     func(*client.MockVersionChecker, *resource.Test)
		queryRequest   *types.QueryWorkflowRequest
		inMemoryClient dc.Client
		isShuttingDown int32
		err            error
	}{
		{
			name: "Success case",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
				resourceMock.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				resourceMock.HistoryClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(
					&types.HistoryQueryWorkflowResponse{
						Response: &types.QueryWorkflowResponse{
							QueryResult: []byte("test-result"),
						},
					}, nil).Times(1)
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
				Query: &types.WorkflowQuery{
					QueryType: "test-query-type",
				},
			},
		},
		{
			name:           "Error case - is shutting down",
			setupMocks:     func(_ *client.MockVersionChecker, _ *resource.Test) {},
			isShuttingDown: 1,
			err:            validate.ErrShuttingDown,
		},
		{
			name: "Error case - query request not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			inMemoryClient: dc.NewInMemoryClient(),
			err:            validate.ErrRequestNotSet,
		},
		{
			name: "Error case - domain not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest:   &types.QueryWorkflowRequest{},
			err:            validate.ErrDomainNotSet,
		},
		{
			name: "Error case - check execution error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
			},
			err: validate.ErrExecutionNotSet,
		},
		{
			name: "Error case - query disallowed for domain",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
			},
			inMemoryClient: func() dc.Client {
				inMemoryClient := dc.NewInMemoryClient()
				inMemoryClient.UpdateValue(dc.DisallowQuery, true)
				return inMemoryClient
			}(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: validate.ErrQueryDisallowedForDomain,
		},
		{
			name: "Error case - query not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: validate.ErrQueryNotSet,
		},
		{
			name: "Error case - query type not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
				Query: &types.WorkflowQuery{},
			},
			err: validate.ErrQueryTypeNotSet,
		},
		{
			name: "Error case - get domain ID error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
				resourceMock.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("", errors.New("get-domain-id-error")).Times(1)
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
				Query: &types.WorkflowQuery{
					QueryType: "test-query-type",
				},
			},
			err: errors.New("get-domain-id-error"),
		},
		{
			name: "Error case - CheckEventBlobSizeLimit error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
				resourceMock.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
				Query: &types.WorkflowQuery{
					QueryType: "test-query-type",
					QueryArgs: []byte("test-query-args"),
				},
			},
			err: common.ErrBlobSizeExceedsLimit,
		},
		{
			name: "Error case - QueryWorkflow error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, resourceMock *resource.Test) {
				resourceMock.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				resourceMock.HistoryClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(nil, errors.New("query-workflow-error")).Times(1)
			},
			inMemoryClient: dc.NewInMemoryClient(),
			queryRequest: &types.QueryWorkflowRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
				Query: &types.WorkflowQuery{
					QueryType: "test-query-type",
				},
			},
			err: errors.New("query-workflow-error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					tc.inMemoryClient,
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)
			cfg.BlobSizeLimitError = func(domain string) int { return 10 }
			cfg.BlobSizeLimitWarn = func(domain string) int { return 9 }

			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.shuttingDown = tc.isShuttingDown

			tc.setupMocks(mockVersionChecker, mockResource)

			queryResult, err := wh.QueryWorkflow(context.Background(), tc.queryRequest)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, []byte("test-result"), queryResult.QueryResult)
			}
		})
	}
}

func TestDescribeWorkflowExecution(t *testing.T) {
	resp := &types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: &types.WorkflowExecutionConfiguration{
			TaskList: &types.TaskList{
				Name: "test-task-list",
			},
		},
	}

	testCases := []struct {
		name            string
		setupMocks      func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test)
		describeRequest *types.DescribeWorkflowExecutionRequest
		isShuttingDown  int32
		err             error
	}{
		{
			name: "Success case",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				mockResource.HistoryClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
			},
			describeRequest: &types.DescribeWorkflowExecutionRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
		},
		{
			name:           "Error case - is shutting down",
			setupMocks:     func(_ *client.MockVersionChecker, _ *resource.Test) {},
			isShuttingDown: 1,
			err:            validate.ErrShuttingDown,
		},
		{
			name: "Error case - describe request not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			err: validate.ErrRequestNotSet,
		},
		{
			name: "Error case - domain name not set",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			describeRequest: &types.DescribeWorkflowExecutionRequest{},
			err:             validate.ErrDomainNotSet,
		},
		{
			name: "Error case - check execution error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, _ *resource.Test) {
			},
			describeRequest: &types.DescribeWorkflowExecutionRequest{
				Domain: "test-domain",
			},
			err: validate.ErrExecutionNotSet,
		},
		{
			name: "Error case - get domain ID error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("", errors.New("get-domain-id-error")).Times(1)
			},
			describeRequest: &types.DescribeWorkflowExecutionRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: errors.New("get-domain-id-error"),
		},
		{
			name: "Error case - DescribeWorkflowExecution error",
			setupMocks: func(mockVersionChecker *client.MockVersionChecker, mockResource *resource.Test) {
				mockResource.DomainCache.EXPECT().GetDomainID(gomock.Any()).Return("test-domain-id", nil).Times(1)
				mockResource.HistoryClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("describe-workflow-execution-error")).Times(1)
			},
			describeRequest: &types.DescribeWorkflowExecutionRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				},
			},
			err: errors.New("describe-workflow-execution-error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockResource := resource.NewTest(t, mockCtrl, metrics.Frontend)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)

			cfg := frontendcfg.NewConfig(
				dc.NewCollection(
					dc.NewInMemoryClient(),
					mockResource.GetLogger(),
				),
				numHistoryShards,
				false,
				"hostname",
			)

			wh := NewWorkflowHandler(mockResource, cfg, mockVersionChecker, nil)
			wh.shuttingDown = tc.isShuttingDown

			tc.setupMocks(mockVersionChecker, mockResource)

			describeResponse, err := wh.DescribeWorkflowExecution(context.Background(), tc.describeRequest)

			if tc.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, resp, describeResponse)
			}
		})
	}
}

func (s *workflowHandlerSuite) TestSignalWithStartWorkflowExecution() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.SignalWithStartWorkflowExecutionRequest{
		Domain:                              s.testDomain,
		WorkflowID:                          testWorkflowID,
		Identity:                            "identity",
		SignalName:                          "signal",
		Input:                               nil,
		WorkflowType:                        &types.WorkflowType{Name: "wType"},
		TaskList:                            &types.TaskList{Name: "taskList"},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(10),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
	}

	testInput := map[string]struct {
		request     *types.SignalWithStartWorkflowExecutionRequest
		expectError bool
		mockFn      func()
	}{
		"shutting down": {
			request: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError: true,
		},
		"nil request": {
			request:     nil,
			mockFn:      func() {},
			expectError: true,
		},
		"empty domain": {
			request: &types.SignalWithStartWorkflowExecutionRequest{
				Domain: "",
			},
			mockFn:      func() {},
			expectError: true,
		},
		"empty workflow ID": {
			request: &types.SignalWithStartWorkflowExecutionRequest{
				Domain:     s.testDomain,
				WorkflowID: "",
			},
			mockFn:      func() {},
			expectError: true,
		},
		"empty workflow type": {
			request: &types.SignalWithStartWorkflowExecutionRequest{
				Domain:       s.testDomain,
				WorkflowID:   testWorkflowID,
				WorkflowType: nil,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"cannot get domain ID": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"history client error": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil).Times(2)
				s.mockHistoryClient.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
			},
			expectError: true,
		},
		"success": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil).Times(2)
				s.mockHistoryClient.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			_, err := wh.SignalWithStartWorkflowExecution(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
		})
	}
}

func (s *workflowHandlerSuite) TestResetWorkflowExecution() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.ResetWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		Reason: "reason",
	}

	testInput := map[string]struct {
		request         *types.ResetWorkflowExecutionRequest
		expectError     bool
		mockFn          func()
		expectErrorType error
	}{
		"shutting down": {
			request: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			request:         nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"empty domain": {
			request: &types.ResetWorkflowExecutionRequest{
				Domain: "",
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrDomainNotSet,
		},
		"empty workflow ID": {
			request: &types.ResetWorkflowExecutionRequest{
				Domain: s.testDomain,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"empty workflow execution": {
			request: &types.ResetWorkflowExecutionRequest{
				Domain:            s.testDomain,
				WorkflowExecution: nil,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"cannot get domain ID": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"history client error": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("error"))
			},
			expectError: true,
		},
		"success": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().ResetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			_, err := wh.ResetWorkflowExecution(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
		})
	}
}

func (s *workflowHandlerSuite) TestTerminateWorkflowExecution() {
	config := s.newConfig(dc.NewInMemoryClient())
	config.EnableClientVersionCheck = dc.GetBoolPropertyFn(true)
	wh := NewWorkflowHandler(s.mockResource, config, s.mockVersionChecker, nil)
	wh.tokenSerializer = s.mockTokenSerializer

	validRequest := &types.TerminateWorkflowExecutionRequest{
		Domain: s.testDomain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: testWorkflowID,
			RunID:      testRunID,
		},
		Reason:   "reason",
		Details:  nil,
		Identity: "identity",
	}

	testInput := map[string]struct {
		request         *types.TerminateWorkflowExecutionRequest
		expectError     bool
		mockFn          func()
		expectErrorType error
	}{
		"shutting down": {
			request: validRequest,
			mockFn: func() {
				wh.shuttingDown = int32(1)
			},
			expectError:     true,
			expectErrorType: validate.ErrShuttingDown,
		},
		"nil request": {
			request:         nil,
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrRequestNotSet,
		},
		"empty domain": {
			request: &types.TerminateWorkflowExecutionRequest{
				Domain: "",
			},
			mockFn:          func() {},
			expectError:     true,
			expectErrorType: validate.ErrDomainNotSet,
		},
		"empty workflow ID": {
			request: &types.TerminateWorkflowExecutionRequest{
				Domain: s.testDomain,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"empty workflow execution": {
			request: &types.TerminateWorkflowExecutionRequest{
				Domain:            s.testDomain,
				WorkflowExecution: nil,
			},
			mockFn:      func() {},
			expectError: true,
		},
		"cannot get domain ID": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return("", errors.New("error getting domain ID"))
			},
			expectError: true,
		},
		"history client error": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(errors.New("error"))
			},
			expectError: true,
		},
		"success": {
			request: validRequest,
			mockFn: func() {
				s.mockDomainCache.EXPECT().GetDomainID(s.testDomain).Return(s.testDomainID, nil)
				s.mockHistoryClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectError: false,
		},
	}

	for name, input := range testInput {
		s.Run(name, func() {
			input.mockFn()
			err := wh.TerminateWorkflowExecution(context.Background(), input.request)
			if input.expectError {
				s.Error(err)
				if input.expectErrorType != nil {
					s.ErrorIs(err, input.expectErrorType)
				}
			} else {
				s.NoError(err)
			}
			wh.shuttingDown = int32(0)
		})
	}
}

func (s *workflowHandlerSuite) TestNormalizeVersionedErrors() {
	config := s.newConfig(dc.NewInMemoryClient())
	wh := s.getWorkflowHandler(config)

	ctx := yarpctest.ContextWithCall(context.Background(), &yarpctest.Call{
		Headers: map[string]string{
			common.FeatureVersionHeaderName:     "feature-version",
			common.ClientImplHeaderName:         "impl-header",
			common.ClientFeatureFlagsHeaderName: "",
		},
	})

	s.mockVersionChecker.EXPECT().SupportsWorkflowAlreadyCompletedError("impl-header", "feature-version", shared.FeatureFlags{}).Return(nil)
	err := wh.normalizeVersionedErrors(ctx, &types.WorkflowExecutionAlreadyCompletedError{})
	s.IsType(err, &types.WorkflowExecutionAlreadyCompletedError{})

	s.mockVersionChecker.EXPECT().SupportsWorkflowAlreadyCompletedError("impl-header", "feature-version", shared.FeatureFlags{}).Return(errors.New("error"))
	err = wh.normalizeVersionedErrors(ctx, &types.WorkflowExecutionAlreadyCompletedError{})
	s.IsType(err, &types.EntityNotExistsError{})
}

func TestWorkflowDescribeEmitStatusMetrics(t *testing.T) {

	tests := map[string]struct {
		res              *types.DescribeWorkflowExecutionResponse
		err              error
		expectedCounters map[string]tally.CounterSnapshot
	}{
		"valid closed workflow": {
			res: &types.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
					CloseStatus: common.Ptr(types.WorkflowExecutionCloseStatusCompleted),
				},
			},
			expectedCounters: map[string]tally.CounterSnapshot{
				"describe_wf_status+domain=some-domain,operation=DescribeWorkflowExecutionStatus,workflow_close_status=COMPLETED": &counterSnapshotMock{
					name: "describe_wf_status",
					tags: map[string]string{
						"domain":                "some-domain",
						"workflow_close_status": "COMPLETED",
						"operation":             "DescribeWorkflowExecutionStatus",
					},
					value: 1,
				},
			},
		},
		"A workflow not found": {
			res: nil,
			err: &types.EntityNotExistsError{},
			expectedCounters: map[string]tally.CounterSnapshot{
				"describe_wf_error+domain=some-domain,operation=DescribeWorkflowExecutionStatus": &counterSnapshotMock{
					name: "describe_wf_error",
					tags: map[string]string{
						"domain":    "some-domain",
						"operation": "DescribeWorkflowExecutionStatus",
					},
					value: 1,
				},
			},
		},
		"A invalid input 1": {
			res: nil,
			err: nil,
			expectedCounters: map[string]tally.CounterSnapshot{
				"describe_wf_error+domain=some-domain,operation=DescribeWorkflowExecutionStatus": &counterSnapshotMock{
					name: "describe_wf_error",
					tags: map[string]string{
						"domain":    "some-domain",
						"operation": "DescribeWorkflowExecutionStatus",
					},
					value: 1,
				},
			},
		},
		"invalid input 2": {
			res: &types.DescribeWorkflowExecutionResponse{
				WorkflowExecutionInfo: &types.WorkflowExecutionInfo{
					// intentionally nil
					// CloseStatus: common.Ptr(types.WorkflowExecutionCloseStatusCompleted),
				},
			},
			expectedCounters: map[string]tally.CounterSnapshot{
				"describe_wf_status+domain=some-domain,operation=DescribeWorkflowExecutionStatus,workflow_close_status=unknown": &counterSnapshotMock{
					name: "describe_wf_status",
					tags: map[string]string{
						"domain":                "some-domain",
						"workflow_close_status": "unknown",
						"operation":             "DescribeWorkflowExecutionStatus",
					},
					value: 1,
				},
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			scope := tally.NewTestScope("", nil)
			mockR := resource.Test{
				MetricsScope:  scope,
				MetricsClient: metrics.NewClient(scope, 1),
			}

			wh := WorkflowHandler{
				Resource: &mockR,
			}

			wh.emitDescribeWorkflowExecutionMetrics("some-domain", td.res, td.err)
			snap := scope.Snapshot()

			for k, v := range td.expectedCounters {
				_, ok := snap.Counters()[k]
				if !ok {
					t.Errorf("the metric string expected was not found. Expected a map with this key: %q\ngot %v", k, snap.Counters())
					return
				}

				assert.Equal(t, snap.Counters()[k].Name(), v.Name())
				assert.Equal(t, snap.Counters()[k].Value(), v.Value())
				assert.Equal(t, snap.Counters()[k].Tags(), v.Tags())
			}
		})
	}
}

type counterSnapshotMock struct {
	name  string
	tags  map[string]string
	value int64
}

func (cs *counterSnapshotMock) Name() string            { return cs.name }
func (cs *counterSnapshotMock) Tags() map[string]string { return cs.tags }
func (cs *counterSnapshotMock) Value() int64            { return cs.value }
