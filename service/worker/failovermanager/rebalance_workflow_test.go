// Copyright (c) 2017-2021 Uber Technologies Inc.
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

package failovermanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/types"
)

type rebalanceWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	activityEnv *testsuite.TestActivityEnvironment
	workflowEnv *testsuite.TestWorkflowEnvironment
}

func TestRebalanceWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(rebalanceWorkflowTestSuite))
}

func (s *rebalanceWorkflowTestSuite) SetupTest() {
	s.activityEnv = s.NewTestActivityEnvironment()
	s.workflowEnv = s.NewTestWorkflowEnvironment()
	s.workflowEnv.RegisterWorkflowWithOptions(RebalanceWorkflow, workflow.RegisterOptions{Name: RebalanceWorkflowTypeName})
	s.workflowEnv.RegisterActivityWithOptions(FailoverActivity, activity.RegisterOptions{Name: failoverActivityName})
	s.workflowEnv.RegisterActivityWithOptions(GetDomainsForRebalanceActivity, activity.RegisterOptions{Name: getRebalanceDomainsActivityName})
	s.activityEnv.RegisterActivityWithOptions(FailoverActivity, activity.RegisterOptions{Name: failoverActivityName})
	s.activityEnv.RegisterActivityWithOptions(GetDomainsForRebalanceActivity, activity.RegisterOptions{Name: getRebalanceDomainsActivityName})
}

func (s *rebalanceWorkflowTestSuite) TearDownTest() {
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_ReturnOne() {
	actEnv, mockResource, controller := s.prepareTestActivityEnv()
	defer controller.Finish()
	defer mockResource.Finish(s.T())

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						common.DomainDataKeyForManagedFailover:  "true",
						common.DomainDataKeyForPreferredCluster: "c2",
					},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			{
				DomainInfo: &types.DomainInfo{
					Name: "d2",
					Data: map[string]string{
						common.DomainDataKeyForManagedFailover:  "false",
						common.DomainDataKeyForPreferredCluster: "c2",
					},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			{
				DomainInfo: &types.DomainInfo{
					Name: "d3",
					Data: map[string]string{
						common.DomainDataKeyForManagedFailover:  "true",
						common.DomainDataKeyForPreferredCluster: "c1",
					},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			{
				DomainInfo: &types.DomainInfo{
					Name: "d4",
					Data: map[string]string{
						common.DomainDataKeyForManagedFailover: "false",
					},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity)
	s.NoError(err)
	var domainData []*DomainRebalanceData
	err = actFuture.Get(&domainData)
	s.NoError(err)
	s.Equal(1, len(domainData))
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_Error() {
	actEnv, mockResource, controller := s.prepareTestActivityEnv()
	defer controller.Finish()
	defer mockResource.Finish(s.T())

	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("test"))

	_, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity)
	s.Error(err)
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_Success() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "d1",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d2",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d3",
			PreferredCluster: "c2",
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(domainData, nil)
	failoverActivityParams1 := &FailoverActivityParams{
		Domains:       []string{"d1", "d2"},
		TargetCluster: "c1",
	}
	failoverActivityResult1 := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
	}
	failoverActivityParams2 := &FailoverActivityParams{
		Domains:       []string{"d3"},
		TargetCluster: "c2",
	}
	failoverActivityResult2 := &FailoverActivityResult{
		SuccessDomains: []string{"d3"},
	}
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams1).Return(failoverActivityResult1, nil).Times(1)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams2).Return(failoverActivityResult2, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(3, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_HalfFailoverActivityError_NoWorkflowError() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "d1",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d2",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d3",
			PreferredCluster: "c2",
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(domainData, nil)
	failoverActivityParams1 := &FailoverActivityParams{
		Domains:       []string{"d1", "d2"},
		TargetCluster: "c1",
	}
	failoverActivityResult1 := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
	}
	failoverActivityParams2 := &FailoverActivityParams{
		Domains:       []string{"d3"},
		TargetCluster: "c2",
	}
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams1).Return(failoverActivityResult1, nil).Times(1)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams2).
		Return(nil, fmt.Errorf("test")).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(2, len(result.SuccessDomains))
	s.Equal(1, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_FailoverActivityError_NoWorkflowError() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "d1",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d2",
			PreferredCluster: "c1",
		},
		{
			DomainName:       "d3",
			PreferredCluster: "c2",
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("test"))
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(0, len(result.SuccessDomains))
	s.Equal(3, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_GetRebalanceDomainsActivityError_WorkflowError() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(nil, fmt.Errorf("test"))
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.Error(err)
}

func (s *rebalanceWorkflowTestSuite) prepareTestActivityEnv() (*testsuite.TestActivityEnvironment, *resource.Test, *gomock.Controller) {
	controller := gomock.NewController(s.T())
	mockResource := resource.NewTest(controller, metrics.Worker)

	ctx := &FailoverManager{
		svcClient:  mockResource.GetSDKClient(),
		clientBean: mockResource.ClientBean,
	}
	s.activityEnv.SetTestTimeout(time.Second * 5)
	s.activityEnv.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: context.WithValue(context.Background(), failoverManagerContextKey, ctx),
	})
	return s.activityEnv, mockResource, controller
}
