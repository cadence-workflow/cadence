// Copyright (c) 2024 Uber Technologies, Inc.
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
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package domaindeprecation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/worker/batcher"
)

var (
	testDomain    = "test-domain"
	defaultParams = DomainDeprecationParams{
		DomainName:    testDomain,
		SecurityToken: "token",
	}
)

type domainDeprecationWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	workflowEnv *testsuite.TestWorkflowEnvironment
	deprecator  *domainDeprecator
}

func TestDomainDeprecationWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(domainDeprecationWorkflowTestSuite))
}

func (s *domainDeprecationWorkflowTestSuite) SetupTest() {
	s.workflowEnv = s.NewTestWorkflowEnvironment()
	controller := gomock.NewController(s.T())
	mockResource := resource.NewTest(s.T(), controller, metrics.Worker)
	publicClient := mockResource.GetSDKClient()
	s.deprecator = &domainDeprecator{
		cfg: Config{
			AdminOperationToken: dynamicproperties.GetStringPropertyFn(""),
		},
		svcClient:     publicClient,
		clientBean:    mockResource.ClientBean,
		metricsClient: metrics.NewNoopMetricsClient(),
		tally:         tally.NoopScope,
		logger:        mockResource.GetLogger(),
	}

	s.T().Cleanup(func() {
		mockResource.Finish(s.T())
	})

	s.workflowEnv.RegisterWorkflowWithOptions(s.deprecator.DomainDeprecationWorkflow, workflow.RegisterOptions{Name: DomainDeprecationWorkflowTypeName})
	s.workflowEnv.RegisterActivityWithOptions(s.deprecator.DisableArchivalActivity, activity.RegisterOptions{Name: disableArchivalActivity})
	s.workflowEnv.RegisterActivityWithOptions(s.deprecator.DeprecateDomainActivity, activity.RegisterOptions{Name: deprecateDomainActivity})
	s.workflowEnv.RegisterActivityWithOptions(s.deprecator.CheckOpenWorkflowsActivity, activity.RegisterOptions{Name: checkOpenWorkflowsActivity})
}

func (s *domainDeprecationWorkflowTestSuite) TearDownTest() {
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *domainDeprecationWorkflowTestSuite) TestWorkflow_Success() {
	s.workflowEnv.OnActivity(disableArchivalActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnActivity(deprecateDomainActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnWorkflow(batcher.BatchWorkflow, mock.Anything, mock.Anything).Return(
		batcher.HeartBeatDetails{
			SuccessCount: 10,
			ErrorCount:   0,
		}, nil).Once()
	s.workflowEnv.OnActivity(checkOpenWorkflowsActivity, mock.Anything, defaultParams).Return(false, nil)

	s.workflowEnv.ExecuteWorkflow(DomainDeprecationWorkflowTypeName, defaultParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
}

func (s *domainDeprecationWorkflowTestSuite) TestWorkflow_Disable_Archival_Error() {
	mockErr := errors.New("error")
	s.workflowEnv.OnActivity(disableArchivalActivity, mock.Anything, defaultParams).Return(mockErr)
	s.workflowEnv.ExecuteWorkflow(DomainDeprecationWorkflowTypeName, defaultParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), mockErr.Error())
}

func (s *domainDeprecationWorkflowTestSuite) TestWorkflow_Deprecate_Domain_Error() {
	mockErr := errors.New("error")
	s.workflowEnv.OnActivity(disableArchivalActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnActivity(deprecateDomainActivity, mock.Anything, defaultParams).Return(mockErr)
	s.workflowEnv.ExecuteWorkflow(DomainDeprecationWorkflowTypeName, defaultParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), mockErr.Error())
}

func (s *domainDeprecationWorkflowTestSuite) TestWorkflow_BatchTerminate_ChildWorkflowError() {
	mockErr := errors.New("batch workflow failed")
	s.workflowEnv.OnActivity(disableArchivalActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnActivity(deprecateDomainActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnWorkflow(batcher.BatchWorkflow, mock.Anything, mock.Anything).Return(
		batcher.HeartBeatDetails{}, mockErr)

	s.workflowEnv.ExecuteWorkflow(DomainDeprecationWorkflowTypeName, defaultParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), mockErr.Error())
}

func (s *domainDeprecationWorkflowTestSuite) TestWorkflow_CheckOpenWorkflows_Error() {
	mockErr := errors.New("error")
	s.workflowEnv.OnActivity(disableArchivalActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnActivity(deprecateDomainActivity, mock.Anything, defaultParams).Return(nil)
	s.workflowEnv.OnWorkflow(batcher.BatchWorkflow, mock.Anything, mock.Anything).Return(
		batcher.HeartBeatDetails{
			SuccessCount: 10,
			ErrorCount:   0,
		}, nil).Once()

	s.workflowEnv.OnActivity(checkOpenWorkflowsActivity, mock.Anything, defaultParams).Return(true, mockErr)

	s.workflowEnv.ExecuteWorkflow(DomainDeprecationWorkflowTypeName, defaultParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.ErrorContains(s.workflowEnv.GetWorkflowError(), mockErr.Error())
}
