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

package clusterredirection

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/api"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

type (
	clusterRedirectionHandlerSuite struct {
		suite.Suite
		*require.Assertions

		controller               *gomock.Controller
		mockResource             *resource.Test
		mockFrontendHandler      *api.MockHandler
		mockRemoteFrontendClient *frontend.MockClient

		mockClusterRedirectionPolicy *MockClusterRedirectionPolicy

		domainName             string
		domainID               string
		domainCacheEntry       *cache.DomainCacheEntry
		currentClusterName     string
		alternativeClusterName string
		config                 *frontendcfg.Config

		handler *clusterRedirectionHandler
	}
)

func TestForwardingPolicyV2ContainsV1(t *testing.T) {
	require.NotEqual(t, selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2, selectedAPIsForwardingRedirectionPolicyAPIAllowlist)
	for k := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		_, ok := selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2[k]
		require.True(t, ok, "v2 does not contain a key that is in v1: %v", k)
	}
}

func TestClusterRedirectionHandlerSuite(t *testing.T) {
	s := new(clusterRedirectionHandlerSuite)
	suite.Run(t, s)
}

func (s *clusterRedirectionHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.domainName = "some random domain name"
	s.domainID = "some random domain ID"
	s.currentClusterName = cluster.TestCurrentClusterName
	s.alternativeClusterName = cluster.TestAlternativeClusterName

	s.controller = gomock.NewController(s.T())
	s.mockClusterRedirectionPolicy = NewMockClusterRedirectionPolicy(s.controller)
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Frontend)
	s.domainCacheEntry = cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{ID: s.domainID, Name: s.domainName}, &persistence.DomainConfig{}, true, nil, 0, nil, 0, 0, 0)
	s.mockResource.DomainCache.EXPECT().GetDomain(s.domainName).Return(s.domainCacheEntry, nil).AnyTimes()
	s.mockResource.DomainCache.EXPECT().GetDomainByID(s.domainID).Return(s.domainCacheEntry, nil).AnyTimes()

	s.mockRemoteFrontendClient = s.mockResource.RemoteFrontendClient

	s.config = frontendcfg.NewConfig(
		dynamicconfig.NewCollection(
			dynamicconfig.NewNopClient(),
			s.mockResource.GetLogger(),
		),
		0,
		false,
		"hostname",
		s.mockResource.GetLogger(),
	)
	dh := domain.NewMockHandler(s.controller)
	frontendHandler := api.NewWorkflowHandler(s.mockResource, s.config, client.NewVersionChecker(), dh)

	s.mockFrontendHandler = api.NewMockHandler(s.controller)
	s.handler = NewAPIHandler(frontendHandler, s.mockResource, s.config, config.ClusterRedirectionPolicy{}).(*clusterRedirectionHandler)
	s.handler.frontendHandler = s.mockFrontendHandler
	s.handler.redirectionPolicy = s.mockClusterRedirectionPolicy
}

func (s *clusterRedirectionHandlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockResource.Finish(s.T())
}

func (s *clusterRedirectionHandlerSuite) TestDescribeTaskList() {
	apiName := "DescribeTaskList"

	ctx := context.Background()
	req := &types.DescribeTaskListRequest{
		Domain: s.domainName,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().DescribeTaskList(ctx, req).Return(&types.DescribeTaskListResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().DescribeTaskList(ctx, req, s.handler.callOptions).Return(&types.DescribeTaskListResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.DescribeTaskList(ctx, req)
	s.Nil(err)
	s.Equal(&types.DescribeTaskListResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestDescribeTaskListError() {
	apiName := "DescribeTaskList"

	ctx := context.Background()
	req := &types.DescribeTaskListRequest{
		Domain: s.domainName,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().DescribeTaskList(ctx, req).Return(nil, errors.New("some error"))
			err := callFn(s.currentClusterName)
			s.NotNil(err)
			return err
		}).
		Times(1)

	resp, err := s.handler.DescribeTaskList(ctx, req)
	s.ErrorContains(err, "some error")
	s.Nil(resp)
}

func (s *clusterRedirectionHandlerSuite) TestDescribeWorkflowExecution() {
	apiName := "DescribeWorkflowExecution"

	ctx := context.Background()
	req := &types.DescribeWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().DescribeWorkflowExecution(ctx, req).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().DescribeWorkflowExecution(ctx, req, s.handler.callOptions).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.DescribeWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.DescribeWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestDescribeWorkflowExecutionStrongConsistency() {
	apiName := "DescribeWorkflowExecution"

	ctx := context.Background()
	req := &types.DescribeWorkflowExecutionRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelStrong, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().DescribeWorkflowExecution(ctx, req).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().DescribeWorkflowExecution(ctx, req, s.handler.callOptions).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.DescribeWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.DescribeWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestGetWorkflowExecutionHistory() {
	apiName := "GetWorkflowExecutionHistory"

	ctx := context.Background()
	req := &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().GetWorkflowExecutionHistory(ctx, req).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().GetWorkflowExecutionHistory(ctx, req, s.handler.callOptions).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.GetWorkflowExecutionHistory(ctx, req)
	s.Nil(err)
	s.Equal(&types.GetWorkflowExecutionHistoryResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestGetWorkflowExecutionHistoryStrongConsistency() {
	apiName := "GetWorkflowExecutionHistory"

	ctx := context.Background()
	req := &types.GetWorkflowExecutionHistoryRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		Execution: &types.WorkflowExecution{
			WorkflowID: "some random workflow ID",
			RunID:      "some random run ID",
		},
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, req.Execution, nil, apiName, types.QueryConsistencyLevelStrong, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().GetWorkflowExecutionHistory(ctx, req).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().GetWorkflowExecutionHistory(ctx, req, s.handler.callOptions).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.GetWorkflowExecutionHistory(ctx, req)
	s.Nil(err)
	s.Equal(&types.GetWorkflowExecutionHistoryResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestListArchivedWorkflowExecutions() {
	apiName := "ListArchivedWorkflowExecutions"

	ctx := context.Background()
	req := &types.ListArchivedWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ListArchivedWorkflowExecutions(ctx, req).Return(&types.ListArchivedWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ListArchivedWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.ListArchivedWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ListArchivedWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListArchivedWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestListClosedWorkflowExecutions() {
	apiName := "ListClosedWorkflowExecutions"

	ctx := context.Background()
	req := &types.ListClosedWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ListClosedWorkflowExecutions(ctx, req).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ListClosedWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ListClosedWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListClosedWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestListOpenWorkflowExecutions() {
	apiName := "ListOpenWorkflowExecutions"

	ctx := context.Background()
	req := &types.ListOpenWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ListOpenWorkflowExecutions(ctx, req).Return(&types.ListOpenWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ListOpenWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.ListOpenWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ListOpenWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListOpenWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestListWorkflowExecutions() {
	apiName := "ListWorkflowExecutions"

	ctx := context.Background()
	req := &types.ListWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ListWorkflowExecutions(ctx, req).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ListWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ListWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestScanWorkflowExecutions() {
	apiName := "ScanWorkflowExecutions"

	ctx := context.Background()
	req := &types.ListWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ScanWorkflowExecutions(ctx, req).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ScanWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ScanWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestCountWorkflowExecutions() {
	apiName := "CountWorkflowExecutions"

	ctx := context.Background()
	req := &types.CountWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().CountWorkflowExecutions(ctx, req).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().CountWorkflowExecutions(ctx, req, s.handler.callOptions).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.CountWorkflowExecutions(ctx, req)
	s.Nil(err)
	s.Equal(&types.CountWorkflowExecutionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestPollForActivityTask() {
	apiName := "PollForActivityTask"

	ctx := context.Background()
	req := &types.PollForActivityTaskRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().PollForActivityTask(ctx, req).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().PollForActivityTask(ctx, req, s.handler.callOptions).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.PollForActivityTask(ctx, req)
	s.Nil(err)
	s.Equal(&types.PollForActivityTaskResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestPollForDecisionTask() {
	apiName := "PollForDecisionTask"

	ctx := context.Background()
	req := &types.PollForDecisionTaskRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().PollForDecisionTask(ctx, req).Return(&types.PollForDecisionTaskResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().PollForDecisionTask(ctx, req, s.handler.callOptions).Return(&types.PollForDecisionTaskResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.PollForDecisionTask(ctx, req)
	s.Nil(err)
	s.Equal(&types.PollForDecisionTaskResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestQueryWorkflow() {
	apiName := "QueryWorkflow"

	ctx := context.Background()
	req := &types.QueryWorkflowRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().QueryWorkflow(ctx, req).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().QueryWorkflow(ctx, req, s.handler.callOptions).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.QueryWorkflow(ctx, req)
	s.Nil(err)
	s.Equal(&types.QueryWorkflowResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestQueryWorkflowWithStrongConsistency() {
	apiName := "QueryWorkflow"

	ctx := context.Background()
	req := &types.QueryWorkflowRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelStrong, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().QueryWorkflow(ctx, req).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().QueryWorkflow(ctx, req, s.handler.callOptions).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.QueryWorkflow(ctx, req)
	s.Nil(err)
	s.Equal(&types.QueryWorkflowResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestRecordActivityTaskHeartbeat() {
	apiName := "RecordActivityTaskHeartbeat"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: token,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RecordActivityTaskHeartbeat(ctx, req).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RecordActivityTaskHeartbeat(ctx, req, s.handler.callOptions).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.RecordActivityTaskHeartbeat(ctx, req)
	s.Nil(err)
	s.Equal(&types.RecordActivityTaskHeartbeatResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestRecordActivityTaskHeartbeatByID() {
	apiName := "RecordActivityTaskHeartbeatByID"

	ctx := context.Background()
	req := &types.RecordActivityTaskHeartbeatByIDRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RecordActivityTaskHeartbeatByID(ctx, req).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RecordActivityTaskHeartbeatByID(ctx, req, s.handler.callOptions).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.RecordActivityTaskHeartbeatByID(ctx, req)
	s.Nil(err)
	s.Equal(&types.RecordActivityTaskHeartbeatResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestRequestCancelWorkflowExecution() {
	apiName := "RequestCancelWorkflowExecution"

	ctx := context.Background()
	req := &types.RequestCancelWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RequestCancelWorkflowExecution(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RequestCancelWorkflowExecution(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.RequestCancelWorkflowExecution(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestResetStickyTaskList() {
	apiName := "ResetStickyTaskList"

	ctx := context.Background()
	req := &types.ResetStickyTaskListRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ResetStickyTaskList(ctx, req).Return(&types.ResetStickyTaskListResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ResetStickyTaskList(ctx, req, s.handler.callOptions).Return(&types.ResetStickyTaskListResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ResetStickyTaskList(ctx, req)
	s.Nil(err)
	s.Equal(&types.ResetStickyTaskListResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestResetWorkflowExecution() {
	apiName := "ResetWorkflowExecution"

	ctx := context.Background()
	req := &types.ResetWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ResetWorkflowExecution(ctx, req).Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ResetWorkflowExecution(ctx, req, s.handler.callOptions).Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ResetWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.ResetWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCanceled() {
	apiName := "RespondActivityTaskCanceled"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskCanceledRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskCanceled(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCanceled(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err = s.handler.RespondActivityTaskCanceled(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCanceledByID() {
	apiName := "RespondActivityTaskCanceledByID"

	ctx := context.Background()
	req := &types.RespondActivityTaskCanceledByIDRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskCanceledByID(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCanceledByID(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.RespondActivityTaskCanceledByID(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCompleted() {
	apiName := "RespondActivityTaskCompleted"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskCompletedRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskCompleted(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCompleted(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err = s.handler.RespondActivityTaskCompleted(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCompletedByID() {
	apiName := "RespondActivityTaskCompletedByID"

	ctx := context.Background()
	req := &types.RespondActivityTaskCompletedByIDRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskCompletedByID(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCompletedByID(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.RespondActivityTaskCompletedByID(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskFailed() {
	apiName := "RespondActivityTaskFailed"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskFailedRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskFailed(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskFailed(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err = s.handler.RespondActivityTaskFailed(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskFailedByID() {
	apiName := "RespondActivityTaskFailedByID"

	ctx := context.Background()
	req := &types.RespondActivityTaskFailedByIDRequest{
		Domain: s.domainName,
	}
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondActivityTaskFailedByID(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskFailedByID(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.RespondActivityTaskFailedByID(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondDecisionTaskCompleted() {
	apiName := "RespondDecisionTaskCompleted"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondDecisionTaskCompletedRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondDecisionTaskCompleted(ctx, req).Return(&types.RespondDecisionTaskCompletedResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondDecisionTaskCompleted(ctx, req, s.handler.callOptions).Return(&types.RespondDecisionTaskCompletedResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.RespondDecisionTaskCompleted(ctx, req)
	s.Nil(err)
	s.Equal(&types.RespondDecisionTaskCompletedResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestRespondDecisionTaskFailed() {
	apiName := "RespondDecisionTaskFailed"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondDecisionTaskFailedRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondDecisionTaskFailed(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondDecisionTaskFailed(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err = s.handler.RespondDecisionTaskFailed(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondQueryTaskCompleted() {
	apiName := "RespondQueryTaskCompleted"

	ctx := context.Background()
	token, err := s.handler.tokenSerializer.SerializeQueryTaskToken(&common.QueryTaskToken{
		DomainID: s.domainID,
	})
	s.NoError(err)
	req := &types.RespondQueryTaskCompletedRequest{
		TaskToken: token,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().RespondQueryTaskCompleted(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().RespondQueryTaskCompleted(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err = s.handler.RespondQueryTaskCompleted(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestSignalWithStartWorkflowExecution() {
	apiName := "SignalWithStartWorkflowExecution"

	ctx := context.Background()
	req := &types.SignalWithStartWorkflowExecutionRequest{
		Domain: s.domainName,
		ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "region-a",
			},
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, &types.WorkflowExecution{}, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			req2 := &types.SignalWithStartWorkflowExecutionRequest{
				Domain: s.domainName,
			}
			s.mockFrontendHandler.EXPECT().SignalWithStartWorkflowExecution(ctx, req2).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().SignalWithStartWorkflowExecution(ctx, req2, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.SignalWithStartWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.StartWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestSignalWithStartWorkflowExecutionWithActiveClusterSelectionPolicy() {
	s.handler.config.EnableActiveClusterSelectionPolicyInStartWorkflow = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	apiName := "SignalWithStartWorkflowExecution"

	ctx := context.Background()
	req := &types.SignalWithStartWorkflowExecutionRequest{
		Domain: s.domainName,
		ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "region-a",
			},
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, &types.WorkflowExecution{}, req.ActiveClusterSelectionPolicy, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().SignalWithStartWorkflowExecution(ctx, req).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().SignalWithStartWorkflowExecution(ctx, req, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.SignalWithStartWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.StartWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestSignalWorkflowExecution() {
	apiName := "SignalWorkflowExecution"

	ctx := context.Background()
	req := &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, req.WorkflowExecution, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().SignalWorkflowExecution(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().SignalWorkflowExecution(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.SignalWorkflowExecution(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestStartWorkflowExecution() {
	apiName := "StartWorkflowExecution"

	ctx := context.Background()
	req := &types.StartWorkflowExecutionRequest{
		Domain: s.domainName,
		ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "region-b",
			},
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			req2 := &types.StartWorkflowExecutionRequest{
				Domain: s.domainName,
			}
			s.mockFrontendHandler.EXPECT().StartWorkflowExecution(ctx, req2).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().StartWorkflowExecution(ctx, req2, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.StartWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.StartWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestStartWorkflowExecutionWithActiveClusterSelectionPolicy() {
	s.handler.config.EnableActiveClusterSelectionPolicyInStartWorkflow = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	apiName := "StartWorkflowExecution"

	ctx := context.Background()
	req := &types.StartWorkflowExecutionRequest{
		Domain: s.domainName,
		ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
			ClusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "region-b",
			},
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, req.ActiveClusterSelectionPolicy, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().StartWorkflowExecution(ctx, req).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().StartWorkflowExecution(ctx, req, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.StartWorkflowExecution(ctx, req)
	s.Nil(err)
	s.Equal(&types.StartWorkflowExecutionResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestTerminateWorkflowExecution() {
	apiName := "TerminateWorkflowExecution"

	ctx := context.Background()
	req := &types.TerminateWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: "test-workflow-id",
			RunID:      "test-run-id",
		},
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, req.WorkflowExecution, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().TerminateWorkflowExecution(ctx, req).Return(nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().TerminateWorkflowExecution(ctx, req, s.handler.callOptions).Return(nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	err := s.handler.TerminateWorkflowExecution(ctx, req)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListTaskListPartitions() {
	apiName := "ListTaskListPartitions"

	req := &types.ListTaskListPartitionsRequest{
		Domain: s.domainName,
		TaskList: &types.TaskList{
			Name: "test_tesk_list",
			Kind: types.TaskListKind(0).Ptr(),
		},
	}

	ctx := context.Background()
	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().ListTaskListPartitions(ctx, req).Return(&types.ListTaskListPartitionsResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().ListTaskListPartitions(ctx, req, s.handler.callOptions).Return(&types.ListTaskListPartitionsResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.ListTaskListPartitions(ctx, req)
	s.Nil(err)
	s.Equal(&types.ListTaskListPartitionsResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestGetTaskListsByDomain() {
	apiName := "GetTaskListsByDomain"

	ctx := context.Background()
	req := &types.GetTaskListsByDomainRequest{
		Domain: s.domainName,
	}

	s.mockClusterRedirectionPolicy.EXPECT().Redirect(ctx, s.domainCacheEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, gomock.Any()).
		DoAndReturn(func(ctx context.Context, domainCacheEntry *cache.DomainCacheEntry, wfExec *types.WorkflowExecution, selPlcy *types.ActiveClusterSelectionPolicy, apiName string, consistencyLevel types.QueryConsistencyLevel, callFn func(targetDC string) error) error {
			// validate callFn logic
			s.mockFrontendHandler.EXPECT().GetTaskListsByDomain(ctx, req).Return(&types.GetTaskListsByDomainResponse{}, nil).Times(1)
			err := callFn(s.currentClusterName)
			s.Nil(err)
			s.mockRemoteFrontendClient.EXPECT().GetTaskListsByDomain(ctx, req, s.handler.callOptions).Return(&types.GetTaskListsByDomainResponse{}, nil).Times(1)
			err = callFn(s.alternativeClusterName)
			s.Nil(err)
			return nil
		}).
		Times(1)

	resp, err := s.handler.GetTaskListsByDomain(ctx, req)
	s.Nil(err)
	s.Equal(&types.GetTaskListsByDomainResponse{}, resp)
}

func (s *clusterRedirectionHandlerSuite) TestGetTaskListsByDomainError() {
	s.handler.redirectionPolicy = newSelectedOrAllAPIsForwardingPolicy(
		s.currentClusterName,
		s.config,
		true,
		selectedAPIsForwardingRedirectionPolicyAPIAllowlistV2,
		"",
		s.mockResource.GetLogger(),
		s.mockResource.GetActiveClusterManager(),
		s.mockResource.GetMetricsClient(),
	)
	ctx := context.Background()
	req := &types.GetTaskListsByDomainRequest{
		Domain: "custom domain name",
	}
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: "custom domain name"},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: s.alternativeClusterName,
		},
		1,
	)
	s.mockResource.DomainCache.EXPECT().GetDomain("custom domain name").Return(domainEntry, nil).Times(1)
	s.mockRemoteFrontendClient.EXPECT().GetTaskListsByDomain(ctx, req, s.handler.callOptions).Return(nil, &types.InternalServiceError{Message: "test error"}).Times(1)

	resp, err := s.handler.GetTaskListsByDomain(ctx, req)
	s.Error(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)
}
