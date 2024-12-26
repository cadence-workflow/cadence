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

package handler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

const (
	testDomain = "test-domain"
)

type (
	handlerSuite struct {
		suite.Suite
		*require.Assertions

		controller      *gomock.Controller
		mockResource    *resource.Test
		mockEngine      *MockEngine
		mockDomainCache *cache.MockDomainCache
		mockLimiter     *quotas.MockLimiter
		handler         *handlerImpl

		testDomain string
	}
)

func TestHandlerSuite(t *testing.T) {
	s := new(handlerSuite)
	suite.Run(t, s)
}

func (s *handlerSuite) SetupSuite() {}

func (s *handlerSuite) TearDownSuite() {}

func (s *handlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Matching)
	s.mockEngine = NewMockEngine(s.controller)
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	s.mockLimiter = quotas.NewMockLimiter(s.controller)

	// Create a handler with a mock limiter
	s.handler = &handlerImpl{
		engine:        s.mockEngine,
		metricsClient: s.mockResource.MetricsClient,
		startWG:       sync.WaitGroup{},
		userRateLimiter: quotas.NewMultiStageRateLimiter(
			s.mockLimiter,
			quotas.NewCollection(quotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int { return 10 })),
		),
		workerRateLimiter: quotas.NewMultiStageRateLimiter(
			s.mockLimiter,
			quotas.NewCollection(quotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int { return 10 })),
		),
		logger:          s.mockResource.GetLogger(),
		throttledLogger: s.mockResource.GetThrottledLogger(),
		domainCache:     s.mockDomainCache,
	}

	s.testDomain = testDomain
}

func (s *handlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockResource.Finish(s.T())
}

func (s *handlerSuite) getHandler(config *config.Config) Handler {
	return NewHandler(s.mockEngine, config, s.mockDomainCache, s.mockResource.MetricsClient, s.mockResource.GetLogger(), s.mockResource.GetThrottledLogger())
}

func (s *handlerSuite) TestNewHandler() {
	cfg := config.NewConfig(dynamicconfig.NewCollection(dynamicconfig.NewInMemoryClient(), s.mockResource.Logger), "matching-test", getIsolationGroupsHelper)
	handler := s.getHandler(cfg)
	s.NotNil(handler)
}

func (s *handlerSuite) TestStart() {
	defer goleak.VerifyNone(s.T())

	cfg := config.NewConfig(dynamicconfig.NewCollection(dynamicconfig.NewInMemoryClient(), s.mockResource.Logger), "matching-test", getIsolationGroupsHelper)
	handler := s.getHandler(cfg)

	handler.Start()
}

func (s *handlerSuite) TestStop() {
	defer goleak.VerifyNone(s.T())

	cfg := config.NewConfig(dynamicconfig.NewCollection(dynamicconfig.NewInMemoryClient(), s.mockResource.Logger), "matching-test", getIsolationGroupsHelper)
	handler := s.getHandler(cfg)

	s.mockEngine.EXPECT().Stop().Times(1)

	handler.Start()
	handler.Stop()
}

func (s *handlerSuite) TestHealth() {
	resp, err := s.handler.Health(context.Background())

	s.NoError(err)
	s.Equal(&types.HealthStatus{Ok: true, Msg: "matching good"}, resp)
}

func (s *handlerSuite) TestNewHandlerContext() {
	handlerCtx := s.handler.newHandlerContext(context.Background(), testDomain, &types.TaskList{Name: "test-task-list"}, 0)

	s.NotNil(handlerCtx)
	s.IsType(&handlerContext{}, handlerCtx)
}

func (s *handlerSuite) TestAddActivityTask() {
	request := types.AddActivityTaskRequest{
		DomainUUID:    "test-domain-id",
		TaskList:      &types.TaskList{Name: "test-task-list"},
		ForwardedFrom: "forwarded-from",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		want       *types.AddActivityTaskResponse
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().AddActivityTask(gomock.Any(), &request).Return(&types.AddActivityTaskResponse{
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  partitions(2),
						WritePartitions: partitions(2),
					},
				}, nil).Times(1)
			},
			want: &types.AddActivityTaskResponse{
				PartitionConfig: &types.TaskListPartitionConfig{
					Version:         1,
					ReadPartitions:  partitions(2),
					WritePartitions: partitions(2),
				},
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - AddActivityTask failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1) // Ensure Allow() returns true
				s.mockEngine.EXPECT().AddActivityTask(gomock.Any(), &request).Return(nil, errors.New("add-activity-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "add-activity-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.AddActivityTask(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.Equal(tc.want, resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestAddDecisionTask() {
	request := types.AddDecisionTaskRequest{
		DomainUUID:    "test-domain-id",
		TaskList:      &types.TaskList{Name: "test-task-list"},
		ForwardedFrom: "forwarded-from",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		want       *types.AddDecisionTaskResponse
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().AddDecisionTask(gomock.Any(), &request).Return(&types.AddDecisionTaskResponse{
					PartitionConfig: &types.TaskListPartitionConfig{
						Version:         1,
						ReadPartitions:  partitions(2),
						WritePartitions: partitions(2),
					},
				}, nil).Times(1)
			},
			want: &types.AddDecisionTaskResponse{
				PartitionConfig: &types.TaskListPartitionConfig{
					Version:         1,
					ReadPartitions:  partitions(2),
					WritePartitions: partitions(2),
				},
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - AddDecisionTask failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1) // Ensure Allow() returns true
				s.mockEngine.EXPECT().AddDecisionTask(gomock.Any(), &request).Return(nil, errors.New("add-decision-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "add-decision-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.AddDecisionTask(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.Equal(tc.want, resp)
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestPollForActivityTask() {
	request := types.MatchingPollForActivityTaskRequest{
		DomainUUID:    "test-domain-id",
		ForwardedFrom: "forwarded-from",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		getCtx     func() (context.Context, context.CancelFunc)
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().PollForActivityTask(gomock.Any(), &request).
					Return(&types.MatchingPollForActivityTaskResponse{TaskToken: []byte("task-token")}, nil).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now())
				return ctx, cancel
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) { return context.Background(), nil },
			err:    &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - LongPollContextTimeout not set",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) { return context.Background(), nil },
			err:    common.ErrContextTimeoutNotSet,
		},
		{
			name: "Error case - PollForActivityTask failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().PollForActivityTask(gomock.Any(), &request).Return(nil, errors.New("poll-activity-error")).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now())
				return ctx, cancel
			},
			err: &types.InternalServiceError{Message: "poll-activity-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			ctx, cancel := tc.getCtx()
			if cancel != nil {
				defer cancel()
			}

			resp, err := s.handler.PollForActivityTask(ctx, &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.MatchingPollForActivityTaskResponse{TaskToken: []byte("task-token")}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestPollForDecisionTask() {
	request := types.MatchingPollForDecisionTaskRequest{
		DomainUUID:    "test-domain-id",
		ForwardedFrom: "forwarded-from",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		getCtx     func() (context.Context, context.CancelFunc)
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().PollForDecisionTask(gomock.Any(), &request).
					Return(&types.MatchingPollForDecisionTaskResponse{TaskToken: []byte("task-token")}, nil).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now())
				return ctx, cancel
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) { return context.Background(), nil },
			err:    &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - LongPollContextTimeout not set",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) { return context.Background(), nil },
			err:    common.ErrContextTimeoutNotSet,
		},
		{
			name: "Error case - PollForDecisionTask failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().PollForDecisionTask(gomock.Any(), &request).Return(nil, errors.New("poll-decision-error")).Times(1)
			},
			getCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now())
				return ctx, cancel
			},
			err: &types.InternalServiceError{Message: "poll-decision-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			ctx, cancel := tc.getCtx()
			if cancel != nil {
				defer cancel()
			}

			resp, err := s.handler.PollForDecisionTask(ctx, &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.MatchingPollForDecisionTaskResponse{TaskToken: []byte("task-token")}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestQueryWorkflow() {
	request := types.MatchingQueryWorkflowRequest{
		DomainUUID:    "test-domain-id",
		ForwardedFrom: "forwarded-from",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().QueryWorkflow(gomock.Any(), &request).
					Return(&types.QueryWorkflowResponse{QueryResult: []byte("query-result")}, nil).Times(1)
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - QueryWorkflow failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().QueryWorkflow(gomock.Any(), &request).Return(nil, errors.New("query-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "query-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.QueryWorkflow(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.QueryWorkflowResponse{QueryResult: []byte("query-result")}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestRespondQueryTaskCompleted() {
	request := types.MatchingRespondQueryTaskCompletedRequest{
		DomainUUID: "test-domain-id",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockEngine.EXPECT().RespondQueryTaskCompleted(gomock.Any(), &request).Return(nil).Times(1)
			},
		},
		{
			name: "Error case - RespondQueryTaskCompleted failed",
			setupMocks: func() {
				s.mockEngine.EXPECT().RespondQueryTaskCompleted(gomock.Any(), &request).Return(errors.New("respond-query-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "respond-query-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)
			s.mockLimiter.EXPECT().Allow().Return(true).Times(1)

			err := s.handler.RespondQueryTaskCompleted(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestCancelOutstandingPoll() {
	request := types.CancelOutstandingPollRequest{
		DomainUUID: "test-domain-id",
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockEngine.EXPECT().CancelOutstandingPoll(gomock.Any(), &request).Return(nil).Times(1)
			},
		},
		{
			name: "Error case - CancelOutstandingPoll failed",
			setupMocks: func() {
				s.mockEngine.EXPECT().CancelOutstandingPoll(gomock.Any(), &request).Return(errors.New("cancel-poll-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "cancel-poll-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)
			s.mockLimiter.EXPECT().Allow().Return(true).Times(1)

			err := s.handler.CancelOutstandingPoll(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *handlerSuite) TestDescribeTaskList() {
	request := types.MatchingDescribeTaskListRequest{
		DomainUUID: "test-domain-id",
		DescRequest: &types.DescribeTaskListRequest{
			TaskList: &types.TaskList{Name: "test-task-list"},
		},
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().DescribeTaskList(gomock.Any(), &request).
					Return(&types.DescribeTaskListResponse{Pollers: []*types.PollerInfo{{}}}, nil).Times(1)
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - DescribeTaskList failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().DescribeTaskList(gomock.Any(), &request).
					Return(nil, errors.New("describe-tasklist-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "describe-tasklist-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.DescribeTaskList(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.DescribeTaskListResponse{Pollers: []*types.PollerInfo{{}}}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestListTaskListPartitions() {
	request := types.MatchingListTaskListPartitionsRequest{
		TaskList: &types.TaskList{Name: "test-task-list"},
		Domain:   s.testDomain,
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().ListTaskListPartitions(gomock.Any(), &request).
					Return(&types.ListTaskListPartitionsResponse{ActivityTaskListPartitions: []*types.TaskListPartitionMetadata{
						{Key: "test-key", OwnerHostName: "test-host"}}}, nil).Times(1)
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - ListTaskListPartitions failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().ListTaskListPartitions(gomock.Any(), &request).
					Return(nil, errors.New("list-tasklist-partitions-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "list-tasklist-partitions-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			resp, err := s.handler.ListTaskListPartitions(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.ListTaskListPartitionsResponse{ActivityTaskListPartitions: []*types.TaskListPartitionMetadata{
					{Key: "test-key", OwnerHostName: "test-host"}}}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestGetTaskListsByDomain() {
	request := types.GetTaskListsByDomainRequest{
		Domain: s.testDomain,
	}

	testCases := []struct {
		name       string
		setupMocks func()
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().GetTaskListsByDomain(gomock.Any(), &request).
					Return(&types.GetTaskListsByDomainResponse{
						DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{"test-decision-task-list": {}},
						ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{"test-activity-task-list": {}},
					}, nil).Times(1)
			},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - GetTaskListsByDomain failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().GetTaskListsByDomain(gomock.Any(), &request).
					Return(nil, errors.New("get-tasklists-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "get-tasklists-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			resp, err := s.handler.GetTaskListsByDomain(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(&types.GetTaskListsByDomainResponse{
					DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{"test-decision-task-list": {}},
					ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{"test-activity-task-list": {}},
				}, resp)
			}
		})
	}
}

func (s *handlerSuite) TestDomainName() {
	testCases := []struct {
		name       string
		setupMocks func()
		resp       string
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomain).Return(s.testDomain, nil).Times(1)
			},
			resp: s.testDomain,
		},
		{
			name: "Error case - GetDomainName failed",
			setupMocks: func() {
				s.mockDomainCache.EXPECT().GetDomainName(s.testDomain).Return("", errors.New("get-domain-error")).Times(1)
			},
			resp: "",
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()

			resp := s.handler.domainName(s.testDomain)

			s.Equal(tc.resp, resp)
		})

	}
}
func (s *handlerSuite) TestRefreshTaskListPartitionConfig() {
	request := types.MatchingRefreshTaskListPartitionConfigRequest{
		DomainUUID: "test-domain-id",
		TaskList:   &types.TaskList{Name: "test-task-list"},
	}

	testCases := []struct {
		name       string
		setupMocks func()
		want       *types.MatchingRefreshTaskListPartitionConfigResponse
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &request).
					Return(&types.MatchingRefreshTaskListPartitionConfigResponse{}, nil).Times(1)
			},
			want: &types.MatchingRefreshTaskListPartitionConfigResponse{},
		},
		{
			name: "Error case - RefreshTaskListPartitionConfig failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &request).
					Return(nil, errors.New("refresh-tasklist-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "refresh-tasklist-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.RefreshTaskListPartitionConfig(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(tc.want, resp)
			}
		})
	}
}

func (s *handlerSuite) TestUpdateTaskListPartitionConfig() {
	request := types.MatchingUpdateTaskListPartitionConfigRequest{
		DomainUUID: "test-domain-id",
		TaskList:   &types.TaskList{Name: "test-task-list"},
	}

	testCases := []struct {
		name       string
		setupMocks func()
		want       *types.MatchingUpdateTaskListPartitionConfigResponse
		err        error
	}{
		{
			name: "Success case",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &request).
					Return(&types.MatchingUpdateTaskListPartitionConfigResponse{}, nil).Times(1)
			},
			want: &types.MatchingUpdateTaskListPartitionConfigResponse{},
		},
		{
			name: "Error case - rate limiter not allowed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(false).Times(1)
			},
			err: &types.ServiceBusyError{Message: "Matching host rps exceeded"},
		},
		{
			name: "Error case - UpdateTaskListPartitionConfig failed",
			setupMocks: func() {
				s.mockLimiter.EXPECT().Allow().Return(true).Times(1)
				s.mockEngine.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &request).
					Return(nil, errors.New("update-tasklist-error")).Times(1)
			},
			err: &types.InternalServiceError{Message: "update-tasklist-error"},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks()
			s.mockDomainCache.EXPECT().GetDomainName(request.DomainUUID).Return(s.testDomain, nil).Times(1)

			resp, err := s.handler.UpdateTaskListPartitionConfig(context.Background(), &request)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
				s.Equal(tc.want, resp)
			}
		})
	}
}

func partitions(num int) map[int]*types.TaskListPartition {
	result := make(map[int]*types.TaskListPartition, num)
	for i := 0; i < num; i++ {
		result[i] = &types.TaskListPartition{}
	}
	return result
}
