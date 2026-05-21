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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
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
	actEnv, mockResource := s.prepareTestActivityEnv()

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
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
						constants.DomainDataKeyForManagedFailover:  "false",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
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
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
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
						constants.DomainDataKeyForManagedFailover: "false",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			{
				DomainInfo: &types.DomainInfo{
					Name: "d5",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover: "true",
					},
					Status: types.DomainStatusRegistered.Ptr(),
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

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, &GetDomainsForRebalanceActivityParams{})
	s.NoError(err)
	var domainData []*DomainRebalanceData
	err = actFuture.Get(&domainData)
	s.NoError(err)
	s.Equal(1, len(domainData))
	s.Equal("d1", domainData[0].DomainName)
	s.Equal("c2", domainData[0].PreferredCluster)
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_ActiveActiveAttrOnly() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	// AA domain: preferred == active (no domain-level change), but cluster.c1 attribute points to c2 (wrong).
	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name: "aa1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"test": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"attr1": {ActiveClusterName: "c2"}, // wrong; preferred is c1
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	prefs := &GetDomainsForRebalanceActivityParams{
		ClusterAttributePreferences: []ClusterAttributePreference{
			{Scope: "test", Name: "attr1", PreferredCluster: "c1"},
		},
	}
	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, prefs)
	s.NoError(err)
	var domainData []*DomainRebalanceData
	err = actFuture.Get(&domainData)
	s.NoError(err)
	s.Require().Equal(1, len(domainData))
	s.Equal("aa1", domainData[0].DomainName)
	s.Equal("", domainData[0].PreferredCluster) // no domain-level change
	s.Require().Equal(1, len(domainData[0].ClusterAttributeTargets))
	s.Equal("test", domainData[0].ClusterAttributeTargets[0].Attribute.Scope)
	s.Equal("attr1", domainData[0].ClusterAttributeTargets[0].Attribute.Name)
	s.Equal("c1", domainData[0].ClusterAttributeTargets[0].TargetCluster)
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_ActiveActiveBothDiff() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	// AA domain: preferred != active (domain-level change needed), and an attribute also mismatched.
	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name: "aa2",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"test": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"attr1": {ActiveClusterName: "c1"}, // wrong; preferred is c2
								"attr2": {ActiveClusterName: "c2"}, // correct
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	prefs := &GetDomainsForRebalanceActivityParams{
		ClusterAttributePreferences: []ClusterAttributePreference{
			{Scope: "test", Name: "attr1", PreferredCluster: "c2"},
			{Scope: "test", Name: "attr2", PreferredCluster: "c2"},
		},
	}
	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, prefs)
	s.NoError(err)
	var domainData []*DomainRebalanceData
	err = actFuture.Get(&domainData)
	s.NoError(err)
	s.Require().Equal(1, len(domainData))
	s.Equal("aa2", domainData[0].DomainName)
	s.Equal("c2", domainData[0].PreferredCluster)
	// Only attr1 is mismatched; attr2 is already correct.
	s.Require().Equal(1, len(domainData[0].ClusterAttributeTargets))
	s.Equal("attr1", domainData[0].ClusterAttributeTargets[0].Attribute.Name)
	s.Equal("c2", domainData[0].ClusterAttributeTargets[0].TargetCluster)
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_Error() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("test"))

	_, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, &GetDomainsForRebalanceActivityParams{})
	s.Error(err)
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_Success() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	domainData := []*DomainRebalanceData{
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	failoverActivityParams1 := &FailoverActivityParams{
		DomainSpecs: []DomainFailoverSpec{
			{DomainName: "d1", TargetCluster: "c1"},
			{DomainName: "d2", TargetCluster: "c1"},
			{DomainName: "d3", TargetCluster: "c2"},
		},
	}
	failoverActivityResult1 := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2", "d3"},
	}
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams1).Return(failoverActivityResult1, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(3, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_WithClusterAttributePreferences() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
		ClusterAttributePreferences: []ClusterAttributePreference{
			{Scope: "cluster", Name: "c0", PreferredCluster: "c0"},
		},
	}
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "aa1",
			PreferredCluster: "",
			ClusterAttributeTargets: []ClusterAttributeTarget{
				{Attribute: types.ClusterAttribute{Scope: "cluster", Name: "c0"}, TargetCluster: "c0"},
			},
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	failoverActivityParams := &FailoverActivityParams{
		DomainSpecs: []DomainFailoverSpec{
			{
				DomainName:    "aa1",
				TargetCluster: "",
				AttributeTargets: []ClusterAttributeTarget{
					{Attribute: types.ClusterAttribute{Scope: "cluster", Name: "c0"}, TargetCluster: "c0"},
				},
			},
		},
	}
	failoverResult := &FailoverActivityResult{SuccessDomains: []string{"aa1"}}
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams).Return(failoverResult, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal([]string{"aa1"}, result.SuccessDomains)
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_HalfFailoverActivityError_NoWorkflowError() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              2,
	}
	domainData := []*DomainRebalanceData{
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	failoverActivityParams1 := &FailoverActivityParams{
		DomainSpecs: []DomainFailoverSpec{
			{DomainName: "d1", TargetCluster: "c1"},
			{DomainName: "d2", TargetCluster: "c1"},
		},
	}
	failoverActivityResult1 := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
	}
	failoverActivityParams2 := &FailoverActivityParams{
		DomainSpecs: []DomainFailoverSpec{
			{DomainName: "d3", TargetCluster: "c2"},
		},
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
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
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
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("test"))
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.Error(err)
}

func (s *rebalanceWorkflowTestSuite) TestShouldAllowRebalance() {
	testCases := []struct {
		name    string
		domain  *types.DescribeDomainResponse
		prefMap map[string]map[string]string
		expect  bool
	}{
		{
			name: "allow rebalance - domain-level mismatch",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: true,
		},
		{
			name: "allow rebalance - active-active attribute mismatch only",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"test": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"a1": {ActiveClusterName: "c2"},
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
			prefMap: map[string]map[string]string{"test": {"a1": "c1"}},
			expect:  true,
		},
		{
			name: "not allow rebalance - not managed by cadence",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "false",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - not global domain",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: false,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - domain status is not registered",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusDeprecated.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - no preferred cluster and no attr prefs",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover: "true",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - preferred cluster same as active",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - preferred cluster not in cluster list",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c3",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
				},
				IsGlobalDomain: true,
			},
			expect: false,
		},
		{
			name: "not allow rebalance - active-active, attrs all match prefs",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"test": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"a1": {ActiveClusterName: "c1"}, // already correct
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
			prefMap: map[string]map[string]string{"test": {"a1": "c1"}},
			expect:  false,
		},
		{
			name: "not allow rebalance - active-active unmanaged",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover: "false",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"test": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"a1": {ActiveClusterName: "c2"}, // wrong but unmanaged
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
			prefMap: map[string]map[string]string{"test": {"a1": "c1"}},
			expect:  false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			s.Equal(tc.expect, shouldAllowRebalance(tc.domain, tc.prefMap))
		})
	}
}

func (s *rebalanceWorkflowTestSuite) prepareTestActivityEnv() (*testsuite.TestActivityEnvironment, *resource.Test) {
	controller := gomock.NewController(s.T())
	mockResource := resource.NewTest(s.T(), controller, metrics.Worker)

	ctx := &FailoverManager{
		svcClient:  mockResource.GetSDKClient(),
		clientBean: mockResource.ClientBean,
	}
	s.activityEnv.SetTestTimeout(time.Second * 5)
	s.activityEnv.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: context.WithValue(context.Background(), failoverManagerContextKey, ctx),
	})

	s.T().Cleanup(func() {
		mockResource.Finish(s.T())
	})

	return s.activityEnv, mockResource
}
