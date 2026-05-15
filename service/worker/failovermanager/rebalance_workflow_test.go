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
	s.workflowEnv.RegisterActivityWithOptions(GetDomainsForRebalanceActivity, activity.RegisterOptions{Name: getRebalanceDomainsActivityName})
	s.workflowEnv.RegisterActivityWithOptions(RebalanceDomainsActivity, activity.RegisterOptions{Name: rebalanceDomainsActivityName})
	s.activityEnv.RegisterActivityWithOptions(GetDomainsForRebalanceActivity, activity.RegisterOptions{Name: getRebalanceDomainsActivityName})
	s.activityEnv.RegisterActivityWithOptions(RebalanceDomainsActivity, activity.RegisterOptions{Name: rebalanceDomainsActivityName})
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

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, (*GetDomainsForRebalanceActivityParams)(nil))
	s.NoError(err)
	var domainData []*DomainRebalanceData
	err = actFuture.Get(&domainData)
	s.NoError(err)
	s.Equal(1, len(domainData))
}

func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_Error() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("test"))

	_, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, (*GetDomainsForRebalanceActivityParams)(nil))
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
	rebalanceResult := &RebalanceResult{
		SuccessDomains: []string{"d1", "d2", "d3"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, domainData).Return(rebalanceResult, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(3, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_Batching() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 0,
		BatchFailoverSize:              2,
	}
	domainData := []*DomainRebalanceData{
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	batch1Result := &RebalanceResult{SuccessDomains: []string{"d1", "d2"}}
	batch2Result := &RebalanceResult{SuccessDomains: []string{"d3"}}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, domainData[0:2]).Return(batch1Result, nil).Times(1)
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, domainData[2:3]).Return(batch2Result, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(3, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_PartialFailure_NoWorkflowError() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	domainData := []*DomainRebalanceData{
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	rebalanceResult := &RebalanceResult{
		SuccessDomains: []string{"d1", "d2"},
		FailedDomains:  []string{"d3"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, domainData).Return(rebalanceResult, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(2, len(result.SuccessDomains))
	s.Equal(1, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_RebalanceActivityError_NoWorkflowError() {
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
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("activity error"))
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

func (s *rebalanceWorkflowTestSuite) TestWorkflow_AAAndNonAA_Success() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
		ClusterAttributePreferences:   []RebalanceClusterAttribute{{Scope: "region", Name: "phx", PreferredCluster: "c1"}},
	}
	domainData := []*DomainRebalanceData{
		{
			DomainName: "aa1",
			ClusterAttributeUpdates: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"phx": {ActiveClusterName: "c1"},
					}},
				},
			},
		},
		{DomainName: "non1", PreferredCluster: "c2"},
	}
	rebalanceResult := &RebalanceResult{SuccessDomains: []string{"aa1", "non1"}}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(rebalanceDomainsActivityName, mock.Anything, domainData).Return(rebalanceResult, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.NoError(err)
	s.Equal(2, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_ConflictingClusterAttributePreferences_Fails() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
		ClusterAttributePreferences: []RebalanceClusterAttribute{
			{Scope: "region", Name: "phx", PreferredCluster: "c1"},
			{Scope: "region", Name: "phx", PreferredCluster: "c2"}, // conflicts with above
		},
	}
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	err := s.workflowEnv.GetWorkflowResult(&result)
	s.Error(err)
	s.Contains(err.Error(), "conflicting preferred cluster")
}

func TestRebalanceClusterAttributesToMap(t *testing.T) {
	tests := []struct {
		name      string
		input     []RebalanceClusterAttribute
		wantMap   ClusterAttributeRebalanceMap
		wantError bool
	}{
		{
			name:    "empty input returns nil map",
			input:   nil,
			wantMap: nil,
		},
		{
			name:  "single entry",
			input: []RebalanceClusterAttribute{{Scope: "region", Name: "phx", PreferredCluster: "c1"}},
			wantMap: ClusterAttributeRebalanceMap{
				"region": ClusterAttributeToClusterMap{"phx": "c1"},
			},
		},
		{
			name: "multiple distinct entries",
			input: []RebalanceClusterAttribute{
				{Scope: "region", Name: "phx", PreferredCluster: "c1"},
				{Scope: "region", Name: "dca", PreferredCluster: "c2"},
				{Scope: "dc", Name: "east", PreferredCluster: "c3"},
			},
			wantMap: ClusterAttributeRebalanceMap{
				"region": ClusterAttributeToClusterMap{"phx": "c1", "dca": "c2"},
				"dc":     ClusterAttributeToClusterMap{"east": "c3"},
			},
		},
		{
			name: "identical duplicate is silently deduplicated",
			input: []RebalanceClusterAttribute{
				{Scope: "region", Name: "phx", PreferredCluster: "c1"},
				{Scope: "region", Name: "phx", PreferredCluster: "c1"},
			},
			wantMap: ClusterAttributeRebalanceMap{
				"region": ClusterAttributeToClusterMap{"phx": "c1"},
			},
		},
		{
			name: "conflicting preferred cluster returns error",
			input: []RebalanceClusterAttribute{
				{Scope: "region", Name: "phx", PreferredCluster: "c1"},
				{Scope: "region", Name: "phx", PreferredCluster: "c2"},
			},
			wantError: true,
		},
		{
			name: "conflict in different scope is independent — no error",
			input: []RebalanceClusterAttribute{
				{Scope: "region", Name: "phx", PreferredCluster: "c1"},
				{Scope: "dc", Name: "phx", PreferredCluster: "c2"}, // same name but different scope
			},
			wantMap: ClusterAttributeRebalanceMap{
				"region": ClusterAttributeToClusterMap{"phx": "c1"},
				"dc":     ClusterAttributeToClusterMap{"phx": "c2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rebalanceClusterAttributesToMap(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.wantMap) {
				t.Fatalf("map length mismatch: got %d, want %d", len(got), len(tt.wantMap))
			}
			for scope, wantAttrs := range tt.wantMap {
				gotAttrs, ok := got[clusterAttributeScope(scope)]
				if !ok {
					t.Fatalf("scope %q missing from result", scope)
				}
				for name, wantCluster := range wantAttrs {
					gotCluster, ok := gotAttrs[clusterAttributeName(name)]
					if !ok {
						t.Fatalf("name %q missing from scope %q", name, scope)
					}
					if gotCluster != wantCluster {
						t.Fatalf("scope=%q name=%q: got %q, want %q", scope, name, gotCluster, wantCluster)
					}
				}
			}
		})
	}
}

func (s *rebalanceWorkflowTestSuite) TestShouldAllowRebalance() {
	testCases := []struct {
		name   string
		domain *types.DescribeDomainResponse
		expect bool
	}{
		{
			name: "allow rebalance",
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
			name: "not allow rebalance because domain failover is not managed by cadence",
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
			name: "not allow rebalance because domain is not global domain",
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
			name: "not allow rebalance because domain status is not registered",
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
		},
		{
			name: "not allow rebalance because domain has no preferred cluster",
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
		},
		{
			name: "not allow rebalance because preferred cluster is the same as active cluster",
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
		},
		{
			name: "not allow rebalance because preferred cluster is not in cluster list",
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
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			s.Equal(tc.expect, shouldAllowRebalance(tc.domain))
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

var aaClusters = []*types.ClusterReplicationConfiguration{
	{ClusterName: "c1"},
	{ClusterName: "c2"},
}

func makeManagedAADomain(name string, activeClusters *types.ActiveClusters) *types.DescribeDomainResponse {
	return &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name: name,
			Data: map[string]string{
				constants.DomainDataKeyForManagedFailover: "true",
			},
			Status: types.DomainStatusRegistered.Ptr(),
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			Clusters:       aaClusters,
			ActiveClusters: activeClusters,
		},
		IsGlobalDomain: true,
	}
}

func TestRebalanceEntryForScopedDomain(t *testing.T) {
	scopeMap := ClusterAttributeRebalanceMap{
		"region": ClusterAttributeToClusterMap{"phx": "c1", "dca": "c2"},
	}
	activeClusters := &types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{
			"region": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					"phx": {ActiveClusterName: "c2"}, // wrong — prefer c1
					"dca": {ActiveClusterName: "c2"}, // correct
				},
			},
		},
	}

	tests := []struct {
		name       string
		domain     *types.DescribeDomainResponse
		scopeMap   ClusterAttributeRebalanceMap
		wantNil    bool
		wantScopes map[string]map[string]string // scopeType -> attrName -> expectedCluster
	}{
		{
			name:     "one mismatched attribute → entry with update",
			domain:   makeManagedAADomain("d1", activeClusters),
			scopeMap: scopeMap,
			wantNil:  false,
			wantScopes: map[string]map[string]string{
				"region": {"phx": "c1"},
			},
		},
		{
			name: "all attributes match → nil",
			domain: makeManagedAADomain("d2", &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"phx": {ActiveClusterName: "c1"},
							"dca": {ActiveClusterName: "c2"},
						},
					},
				},
			}),
			scopeMap: scopeMap,
			wantNil:  true,
		},
		{
			name:     "no AA scopes on domain → nil",
			domain:   makeManagedAADomain("d3", nil),
			scopeMap: scopeMap,
			wantNil:  true,
		},
		{
			name: "not managed by Cadence → nil",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name:   "d4",
					Data:   map[string]string{constants.DomainDataKeyForManagedFailover: "false"},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					Clusters:       aaClusters,
					ActiveClusters: activeClusters,
				},
				IsGlobalDomain: true,
			},
			scopeMap: scopeMap,
			wantNil:  true,
		},
		{
			name:     "scope type not in scopeMap → nil",
			domain:   makeManagedAADomain("d5", activeClusters),
			scopeMap: ClusterAttributeRebalanceMap{"datacenter": ClusterAttributeToClusterMap{"phx": "c1"}},
			wantNil:  true,
		},
		{
			name: "preferred cluster not in domain's cluster list → skipped, nil",
			domain: makeManagedAADomain("d6", &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"phx": {ActiveClusterName: "c2"},
						},
					},
				},
			}),
			scopeMap: ClusterAttributeRebalanceMap{"region": ClusterAttributeToClusterMap{"phx": "c3"}}, // c3 not in aaClusters
			wantNil:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := rebalanceEntryForScopedDomain(tc.domain, tc.scopeMap)
			if tc.wantNil {
				if entry != nil {
					t.Fatalf("expected nil, got %+v", entry)
				}
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			updates := entry.ClusterAttributeUpdates
			for scopeType, wantAttrs := range tc.wantScopes {
				scope, ok := updates.AttributeScopes[scopeType]
				if !ok {
					t.Fatalf("missing scope %q in updates", scopeType)
				}
				for attrName, wantCluster := range wantAttrs {
					info, ok := scope.ClusterAttributes[attrName]
					if !ok {
						t.Fatalf("missing attribute %q in scope %q", attrName, scopeType)
					}
					if info.ActiveClusterName != wantCluster {
						t.Fatalf("scope %q attr %q: want cluster %q, got %q", scopeType, attrName, wantCluster, info.ActiveClusterName)
					}
				}
			}
		})
	}
}

func (s *rebalanceWorkflowTestSuite) TestRebalanceDomainsActivity_AllSuccess() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	domainData := []*DomainRebalanceData{
		{
			DomainName: "aa1",
			ClusterAttributeUpdates: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"phx": {ActiveClusterName: "c1"},
					}},
				},
			},
		},
		{DomainName: "non1", PreferredCluster: "c2"},
	}

	mockResource.FrontendClient.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(&types.UpdateDomainResponse{}, nil).Times(2)

	actFuture, err := actEnv.ExecuteActivity(RebalanceDomainsActivity, domainData)
	s.NoError(err)
	var result RebalanceResult
	err = actFuture.Get(&result)
	s.NoError(err)
	s.Equal(2, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}

func (s *rebalanceWorkflowTestSuite) TestRebalanceDomainsActivity_OneFailure() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	domainData := []*DomainRebalanceData{
		{
			DomainName: "aa1",
			ClusterAttributeUpdates: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"phx": {ActiveClusterName: "c1"},
					}},
				},
			},
		},
		{DomainName: "non1", PreferredCluster: "c2"},
	}

	mockResource.FrontendClient.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(&types.UpdateDomainResponse{}, nil).Times(1)
	mockResource.FrontendClient.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("rpc error")).Times(1)

	actFuture, err := actEnv.ExecuteActivity(RebalanceDomainsActivity, domainData)
	s.NoError(err)
	var result RebalanceResult
	err = actFuture.Get(&result)
	s.NoError(err)
	s.Equal(1, len(result.SuccessDomains))
	s.Equal(1, len(result.FailedDomains))
}
