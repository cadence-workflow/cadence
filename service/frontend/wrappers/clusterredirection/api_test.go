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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/metrics"
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

	s.mockClusterRedirectionPolicy = &MockClusterRedirectionPolicy{}

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Frontend)
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
	s.mockClusterRedirectionPolicy.AssertExpectations(s.T())
}

func (s *clusterRedirectionHandlerSuite) TestDescribeTaskList() {
	apiName := "DescribeTaskList"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.DescribeTaskListRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.DescribeTaskList(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().DescribeTaskList(gomock.Any(), req).Return(&types.DescribeTaskListResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), req, s.handler.callOptions).Return(&types.DescribeTaskListResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestDescribeWorkflowExecution() {
	apiName := "DescribeWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.DescribeWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.DescribeWorkflowExecution(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestDescribeWorkflowExecutionStrongConsistency() {
	apiName := "DescribeWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelStrong, mock.Anything).Return(nil).Times(1)

	req := &types.DescribeWorkflowExecutionRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
	}
	resp, err := s.handler.DescribeWorkflowExecution(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().DescribeWorkflowExecution(gomock.Any(), req).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(&types.DescribeWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestGetWorkflowExecutionHistory() {
	apiName := "GetWorkflowExecutionHistory"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.GetWorkflowExecutionHistory(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), req).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), req, s.handler.callOptions).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestGetWorkflowExecutionHistoryStrongConsistency() {
	apiName := "GetWorkflowExecutionHistory"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelStrong, mock.Anything).Return(nil).Times(1)

	req := &types.GetWorkflowExecutionHistoryRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
	}
	resp, err := s.handler.GetWorkflowExecutionHistory(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), req).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), req, s.handler.callOptions).Return(&types.GetWorkflowExecutionHistoryResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListArchivedWorkflowExecutions() {
	apiName := "ListArchivedWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListArchivedWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ListArchivedWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ListArchivedWorkflowExecutions(gomock.Any(), req).Return(&types.ListArchivedWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ListArchivedWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListArchivedWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListClosedWorkflowExecutions() {
	apiName := "ListClosedWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListClosedWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ListClosedWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), req).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListOpenWorkflowExecutions() {
	apiName := "ListOpenWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListOpenWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ListOpenWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), req).Return(&types.ListOpenWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListOpenWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListWorkflowExecutions() {
	apiName := "ListWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ListWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ListWorkflowExecutions(gomock.Any(), req).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ListWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestScanWorkflowExecutions() {
	apiName := "ScanWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ScanWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ScanWorkflowExecutions(gomock.Any(), req).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ScanWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestCountWorkflowExecutions() {
	apiName := "CountWorkflowExecutions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.CountWorkflowExecutionsRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.CountWorkflowExecutions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().CountWorkflowExecutions(gomock.Any(), req).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), req, s.handler.callOptions).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestPollForActivityTask() {
	apiName := "PollForActivityTask"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.PollForActivityTaskRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.PollForActivityTask(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().PollForActivityTask(gomock.Any(), req).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().PollForActivityTask(gomock.Any(), req, s.handler.callOptions).Return(&types.PollForActivityTaskResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestPollForDecisionTask() {
	apiName := "PollForDecisionTask"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.PollForDecisionTaskRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.PollForDecisionTask(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().PollForDecisionTask(gomock.Any(), req).Return(&types.PollForDecisionTaskResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().PollForDecisionTask(gomock.Any(), req, s.handler.callOptions).Return(&types.PollForDecisionTaskResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestQueryWorkflow() {
	apiName := "QueryWorkflow"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.QueryWorkflowRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.QueryWorkflow(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().QueryWorkflow(gomock.Any(), req).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().QueryWorkflow(gomock.Any(), req, s.handler.callOptions).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestQueryWorkflowWithStrongConsistency() {
	apiName := "QueryWorkflow"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelStrong, mock.Anything).Return(nil).Times(1)

	req := &types.QueryWorkflowRequest{
		Domain:                s.domainName,
		QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
	}
	resp, err := s.handler.QueryWorkflow(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().QueryWorkflow(gomock.Any(), req).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().QueryWorkflow(gomock.Any(), req, s.handler.callOptions).Return(&types.QueryWorkflowResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRecordActivityTaskHeartbeat() {
	apiName := "RecordActivityTaskHeartbeat"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: token,
	}
	resp, err := s.handler.RecordActivityTaskHeartbeat(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), req).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RecordActivityTaskHeartbeat(gomock.Any(), req, s.handler.callOptions).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRecordActivityTaskHeartbeatByID() {
	apiName := "RecordActivityTaskHeartbeatByID"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.RecordActivityTaskHeartbeatByIDRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.RecordActivityTaskHeartbeatByID(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RecordActivityTaskHeartbeatByID(gomock.Any(), req).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RecordActivityTaskHeartbeatByID(gomock.Any(), req, s.handler.callOptions).Return(&types.RecordActivityTaskHeartbeatResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRequestCancelWorkflowExecution() {
	apiName := "RequestCancelWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.RequestCancelWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	err := s.handler.RequestCancelWorkflowExecution(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestResetStickyTaskList() {
	apiName := "ResetStickyTaskList"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ResetStickyTaskListRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ResetStickyTaskList(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ResetStickyTaskList(gomock.Any(), req).Return(&types.ResetStickyTaskListResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ResetStickyTaskList(gomock.Any(), req, s.handler.callOptions).Return(&types.ResetStickyTaskListResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestResetWorkflowExecution() {
	apiName := "ResetWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ResetWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.ResetWorkflowExecution(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ResetWorkflowExecution(gomock.Any(), req).Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ResetWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(&types.ResetWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCanceled() {
	apiName := "RespondActivityTaskCanceled"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskCanceledRequest{
		TaskToken: token,
	}
	err = s.handler.RespondActivityTaskCanceled(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskCanceled(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCanceled(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCanceledByID() {
	apiName := "RespondActivityTaskCanceledByID"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.RespondActivityTaskCanceledByIDRequest{
		Domain: s.domainName,
	}
	err := s.handler.RespondActivityTaskCanceledByID(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskCanceledByID(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCanceledByID(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCompleted() {
	apiName := "RespondActivityTaskCompleted"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskCompletedRequest{
		TaskToken: token,
	}
	err = s.handler.RespondActivityTaskCompleted(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskCompleted(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCompleted(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskCompletedByID() {
	apiName := "RespondActivityTaskCompletedByID"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.RespondActivityTaskCompletedByIDRequest{
		Domain: s.domainName,
	}
	err := s.handler.RespondActivityTaskCompletedByID(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskCompletedByID(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskCompletedByID(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskFailed() {
	apiName := "RespondActivityTaskFailed"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondActivityTaskFailedRequest{
		TaskToken: token,
	}
	err = s.handler.RespondActivityTaskFailed(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskFailed(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskFailed(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondActivityTaskFailedByID() {
	apiName := "RespondActivityTaskFailedByID"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.RespondActivityTaskFailedByIDRequest{
		Domain: s.domainName,
	}
	err := s.handler.RespondActivityTaskFailedByID(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondActivityTaskFailedByID(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondActivityTaskFailedByID(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondDecisionTaskCompleted() {
	apiName := "RespondDecisionTaskCompleted"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondDecisionTaskCompletedRequest{
		TaskToken: token,
	}
	resp, err := s.handler.RespondDecisionTaskCompleted(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), req).Return(&types.RespondDecisionTaskCompletedResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondDecisionTaskCompleted(gomock.Any(), req, s.handler.callOptions).Return(&types.RespondDecisionTaskCompletedResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondDecisionTaskFailed() {
	apiName := "RespondDecisionTaskFailed"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.Serialize(&common.TaskToken{
		DomainID: s.domainID,
	})
	s.Nil(err)
	req := &types.RespondDecisionTaskFailedRequest{
		TaskToken: token,
	}
	err = s.handler.RespondDecisionTaskFailed(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondDecisionTaskFailed(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondDecisionTaskFailed(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestRespondQueryTaskCompleted() {
	apiName := "RespondQueryTaskCompleted"

	s.mockClusterRedirectionPolicy.On("WithDomainIDRedirect",
		s.domainID, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	token, err := s.handler.tokenSerializer.SerializeQueryTaskToken(&common.QueryTaskToken{
		DomainID: s.domainID,
	})
	s.NoError(err)
	req := &types.RespondQueryTaskCompletedRequest{
		TaskToken: token,
	}
	err = s.handler.RespondQueryTaskCompleted(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().RespondQueryTaskCompleted(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().RespondQueryTaskCompleted(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestSignalWithStartWorkflowExecution() {
	apiName := "SignalWithStartWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.SignalWithStartWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.SignalWithStartWorkflowExecution(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), req).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestSignalWorkflowExecution() {
	apiName := "SignalWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	err := s.handler.SignalWorkflowExecution(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().SignalWorkflowExecution(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().SignalWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestStartWorkflowExecution() {
	apiName := "StartWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.StartWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.StartWorkflowExecution(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().StartWorkflowExecution(gomock.Any(), req).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(&types.StartWorkflowExecutionResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestTerminateWorkflowExecution() {
	apiName := "TerminateWorkflowExecution"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.TerminateWorkflowExecutionRequest{
		Domain: s.domainName,
	}
	err := s.handler.TerminateWorkflowExecution(context.Background(), req)
	s.Nil(err)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().TerminateWorkflowExecution(gomock.Any(), req).Return(nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), req, s.handler.callOptions).Return(nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestListTaskListPartitions() {
	apiName := "ListTaskListPartitions"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.ListTaskListPartitionsRequest{
		Domain: s.domainName,
		TaskList: &types.TaskList{
			Name: "test_tesk_list",
			Kind: types.TaskListKind(0).Ptr(),
		},
	}
	resp, err := s.handler.ListTaskListPartitions(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().ListTaskListPartitions(gomock.Any(), req).Return(&types.ListTaskListPartitionsResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().ListTaskListPartitions(gomock.Any(), req, s.handler.callOptions).Return(&types.ListTaskListPartitionsResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}

func (s *clusterRedirectionHandlerSuite) TestGetTaskListsByDomain() {
	apiName := "GetTaskListsByDomain"

	s.mockClusterRedirectionPolicy.On("WithDomainNameRedirect",
		s.domainName, apiName, types.QueryConsistencyLevelEventual, mock.Anything).Return(nil).Times(1)

	req := &types.GetTaskListsByDomainRequest{
		Domain: s.domainName,
	}
	resp, err := s.handler.GetTaskListsByDomain(context.Background(), req)
	s.Nil(err)
	// the resp is initialized to nil, since inner function is not called
	s.Nil(resp)

	callFn := s.mockClusterRedirectionPolicy.Calls[0].Arguments[3].(func(string) error)
	s.mockFrontendHandler.EXPECT().GetTaskListsByDomain(gomock.Any(), req).Return(&types.GetTaskListsByDomainResponse{}, nil).Times(1)
	err = callFn(s.currentClusterName)
	s.Nil(err)
	s.mockRemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), req, s.handler.callOptions).Return(&types.GetTaskListsByDomainResponse{}, nil).Times(1)
	err = callFn(s.alternativeClusterName)
	s.Nil(err)
}
