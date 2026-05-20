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

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, &GetDomainsForRebalanceActivityParams{})
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
	// All 3 domains fit in one batch (size=10), so one FailoverActivity call with all DomainPreferences
	failoverActivityParams := &FailoverActivityParams{
		DomainPreferences: []DomainFailoverPreference{
			{DomainName: "d1", TargetCluster: "c1"},
			{DomainName: "d2", TargetCluster: "c1"},
			{DomainName: "d3", TargetCluster: "c2"},
		},
	}
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2", "d3"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams).Return(failoverActivityResult, nil).Times(1)
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
		{DomainName: "d1", PreferredCluster: "c1"},
		{DomainName: "d2", PreferredCluster: "c1"},
		{DomainName: "d3", PreferredCluster: "c2"},
	}
	// One batch; activity succeeds overall but reports d3 as failed
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
		FailedDomains:  []string{"d3"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, mock.Anything).Return(failoverActivityResult, nil).Times(1)
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
			s.Equal(tc.expect, shouldAllowRebalance(tc.domain, nil))
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

// clusterPrefs is a convenience set used by multiple rebalance TDD tests.
// scope "cluster": cluster0→cluster0, cluster1→cluster1, cluster2→cluster2
var testClusterPrefs = []ClusterAttributePreference{
	{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster0"},
	{Scope: "cluster", Name: "cluster1", PreferredCluster: "cluster1"},
	{Scope: "cluster", Name: "cluster2", PreferredCluster: "cluster2"},
}

func makeClusters(names ...string) []*types.ClusterReplicationConfiguration {
	var out []*types.ClusterReplicationConfiguration
	for _, n := range names {
		out = append(out, &types.ClusterReplicationConfiguration{ClusterName: n})
	}
	return out
}

func makeAADomain(name, activeName, preferredCluster string, managed bool, attrMap map[string]string) *types.DescribeDomainResponse {
	data := map[string]string{}
	if managed {
		data[constants.DomainDataKeyForManagedFailover] = "true"
	}
	if preferredCluster != "" {
		data[constants.DomainDataKeyForPreferredCluster] = preferredCluster
	}
	clusterAttrs := make(map[string]types.ActiveClusterInfo)
	for attr, clusterName := range attrMap {
		clusterAttrs[attr] = types.ActiveClusterInfo{ActiveClusterName: clusterName}
	}
	return &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name:   name,
			Data:   data,
			Status: types.DomainStatusRegistered.Ptr(),
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: activeName,
			Clusters:          makeClusters("cluster0", "cluster1", "cluster2"),
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: clusterAttrs},
				},
			},
		},
		IsGlobalDomain: true,
	}
}

func makeGlobalDomain(name, activeName, preferredCluster string, managed bool) *types.DescribeDomainResponse {
	data := map[string]string{}
	if managed {
		data[constants.DomainDataKeyForManagedFailover] = "true"
	}
	if preferredCluster != "" {
		data[constants.DomainDataKeyForPreferredCluster] = preferredCluster
	}
	return &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name:   name,
			Data:   data,
			Status: types.DomainStatusRegistered.Ptr(),
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: activeName,
			Clusters:          makeClusters("cluster0", "cluster1", "cluster2"),
		},
		IsGlobalDomain: true,
	}
}

// TestShouldAllowRebalance_WithPreferences covers all 7 rebalance scenarios from test_rebalance.sh.
func TestShouldAllowRebalance_WithPreferences(t *testing.T) {
	tests := []struct {
		name   string
		domain *types.DescribeDomainResponse
		prefs  []ClusterAttributePreference
		expect bool
	}{
		// test case 1: rb-global-unmanaged — not managed, no change
		{
			name:   "when global domain is not managed by Cadence it should not allow rebalance",
			domain: makeGlobalDomain("rb-global-unmanaged", "cluster1", "", false),
			prefs:  testClusterPrefs,
			expect: false,
		},
		// test case 2: rb-global-same — managed, preferred == active
		{
			name:   "when global domain preferred cluster equals active cluster it should not allow rebalance",
			domain: makeGlobalDomain("rb-global-same", "cluster0", "cluster0", true),
			prefs:  nil,
			expect: false,
		},
		// test case 3: rb-global-diff — managed, preferred != active
		{
			name:   "when global domain preferred cluster differs from active cluster it should allow rebalance",
			domain: makeGlobalDomain("rb-global-diff", "cluster0", "cluster1", true),
			prefs:  nil,
			expect: true,
		},
		// test case 7: rb-aa-unmanaged — AA domain, not managed
		{
			name: "when AA domain is not managed by Cadence it should not allow rebalance",
			domain: makeAADomain("rb-aa-unmanaged", "cluster0", "", false,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}),
			prefs:  testClusterPrefs,
			expect: false,
		},
		// test case 4: rb-aa-attr-only — preferred == active, but cluster.cluster0 is wrong
		{
			name: "when AA domain preferred equals active and one attr does not match preferences it should allow rebalance",
			domain: makeAADomain("rb-aa-attr-only", "cluster0", "cluster0", true,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}),
			prefs:  testClusterPrefs,
			expect: true,
		},
		// AA domain, preferred == active, all attrs already correct
		{
			name: "when AA domain preferred equals active and all attrs match preferences it should not allow rebalance",
			domain: makeAADomain("rb-aa-all-correct", "cluster0", "cluster0", true,
				map[string]string{"cluster0": "cluster0", "cluster1": "cluster1", "cluster2": "cluster2"}),
			prefs:  testClusterPrefs,
			expect: false,
		},
		// test case 5: rb-aa-both-diff — preferred != active, AND attrs wrong
		{
			name: "when AA domain preferred differs from active and attrs are wrong it should allow rebalance",
			domain: makeAADomain("rb-aa-both-diff", "cluster0", "cluster1", true,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster0", "cluster2": "cluster2"}),
			prefs:  testClusterPrefs,
			expect: true,
		},
		// test case 6: rb-aa-no-attr-diff — preferred != active, all attrs already correct
		{
			name: "when AA domain preferred differs from active and all attrs match preferences it should allow rebalance",
			domain: makeAADomain("rb-aa-no-attr-diff", "cluster0", "cluster1", true,
				map[string]string{"cluster0": "cluster0", "cluster1": "cluster1", "cluster2": "cluster2"}),
			prefs:  testClusterPrefs,
			expect: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAllowRebalance(tc.domain, tc.prefs)
			if got != tc.expect {
				t.Errorf("shouldAllowRebalance() = %v, want %v", got, tc.expect)
			}
		})
	}
}

// TestBuildMismatchedTargets verifies only wrong attributes are returned.
func TestBuildMismatchedTargets(t *testing.T) {
	domain := makeAADomain("d", "cluster0", "cluster0", true,
		// cluster0 attr is on cluster1 (wrong), cluster1 attr is correct, cluster2 attr is correct
		map[string]string{"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"})

	targets := buildMismatchedTargets(domain, testClusterPrefs)
	if len(targets) != 1 {
		t.Fatalf("expected 1 mismatched target, got %d", len(targets))
	}
	got := targets[0]
	if got.Attribute.Scope != "cluster" || got.Attribute.Name != "cluster0" || got.TargetCluster != "cluster0" {
		t.Errorf("unexpected target: %+v", got)
	}
}

// TestGetDomainsForRebalanceActivity_WithPreferences covers all 7 scenarios.
func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_WithPreferences() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	allDomains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			// 1. rb-global-unmanaged: not managed → excluded
			makeGlobalDomain("rb-global-unmanaged", "cluster1", "", false),
			// 2. rb-global-same: managed, preferred == active → excluded
			makeGlobalDomain("rb-global-same", "cluster0", "cluster0", true),
			// 3. rb-global-diff: managed, preferred != active → included, no ClusterAttributeTargets
			makeGlobalDomain("rb-global-diff", "cluster0", "cluster1", true),
			// 4. rb-aa-attr-only: preferred == active, one attr wrong → included, 1 ClusterAttributeTarget
			makeAADomain("rb-aa-attr-only", "cluster0", "cluster0", true,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}),
			// 5. rb-aa-both-diff: preferred != active, attrs wrong → included
			makeAADomain("rb-aa-both-diff", "cluster0", "cluster1", true,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster0", "cluster2": "cluster2"}),
			// 6. rb-aa-no-attr-diff: preferred != active, all attrs correct → included, no ClusterAttributeTargets
			makeAADomain("rb-aa-no-attr-diff", "cluster0", "cluster1", true,
				map[string]string{"cluster0": "cluster0", "cluster1": "cluster1", "cluster2": "cluster2"}),
			// 7. rb-aa-unmanaged: not managed → excluded
			makeAADomain("rb-aa-unmanaged", "cluster0", "", false,
				map[string]string{"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(allDomains, nil)

	params := &GetDomainsForRebalanceActivityParams{ClusterAttributePreferences: testClusterPrefs}
	future, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity, params)
	s.NoError(err)
	var result []*DomainRebalanceData
	s.NoError(future.Get(&result))

	// Expect 4 domains: rb-global-diff, rb-aa-attr-only, rb-aa-both-diff, rb-aa-no-attr-diff
	s.Require().Len(result, 4)
	byName := make(map[string]*DomainRebalanceData)
	for _, d := range result {
		byName[d.DomainName] = d
	}

	// rb-global-diff: no AA attrs
	d := byName["rb-global-diff"]
	s.Require().NotNil(d, "rb-global-diff should be in results")
	s.Equal("cluster1", d.PreferredCluster)
	s.Nil(d.ClusterAttributeTargets)

	// rb-aa-attr-only: only cluster0 attr is wrong
	d = byName["rb-aa-attr-only"]
	s.Require().NotNil(d, "rb-aa-attr-only should be in results")
	s.Equal("cluster0", d.PreferredCluster)
	s.Require().Len(d.ClusterAttributeTargets, 1)
	s.Equal("cluster0", d.ClusterAttributeTargets[0].Attribute.Name)
	s.Equal("cluster0", d.ClusterAttributeTargets[0].TargetCluster)

	// rb-aa-both-diff: cluster0 and cluster1 attrs are wrong
	d = byName["rb-aa-both-diff"]
	s.Require().NotNil(d, "rb-aa-both-diff should be in results")
	s.Equal("cluster1", d.PreferredCluster)
	s.Require().Len(d.ClusterAttributeTargets, 2)

	// rb-aa-no-attr-diff: all attrs correct, only domain-level change
	d = byName["rb-aa-no-attr-diff"]
	s.Require().NotNil(d, "rb-aa-no-attr-diff should be in results")
	s.Equal("cluster1", d.PreferredCluster)
	s.Nil(d.ClusterAttributeTargets)
}

// TestWorkflow_RebalanceMixedDomains verifies that non-AA and AA domains
// are submitted in one FailoverActivity call with correct DomainPreferences.
func (s *rebalanceWorkflowTestSuite) TestWorkflow_RebalanceMixedDomains() {
	params := &RebalanceParams{
		BatchFailoverSize:              10,
		BatchFailoverWaitTimeInSeconds: 1,
		ClusterAttributePreferences:    testClusterPrefs,
	}
	// Simulate what GetDomainsForRebalanceActivity would return:
	// - rb-global-diff: plain global, preferred != active, no ClusterAttributeTargets
	// - rb-aa-attr-only: AA, preferred == active, one attr wrong
	domainData := []*DomainRebalanceData{
		{DomainName: "rb-global-diff", PreferredCluster: "cluster1"},
		{
			DomainName:       "rb-aa-attr-only",
			PreferredCluster: "cluster0",
			ClusterAttributeTargets: []ClusterAttributeTarget{
				{Attribute: types.ClusterAttribute{Scope: "cluster", Name: "cluster0"}, TargetCluster: "cluster0"},
			},
		},
	}
	expectedParams := &FailoverActivityParams{
		DomainPreferences: []DomainFailoverPreference{
			{DomainName: "rb-global-diff", TargetCluster: "cluster1"},
			{
				DomainName:    "rb-aa-attr-only",
				TargetCluster: "cluster0",
				ClusterAttributeTargets: []ClusterAttributeTarget{
					{Attribute: types.ClusterAttribute{Scope: "cluster", Name: "cluster0"}, TargetCluster: "cluster0"},
				},
			},
		},
	}
	activityResult := &FailoverActivityResult{
		SuccessDomains: []string{"rb-global-diff", "rb-aa-attr-only"},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything, mock.Anything).Return(domainData, nil)
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, expectedParams).Return(activityResult, nil).Times(1)
	s.workflowEnv.ExecuteWorkflow(RebalanceWorkflowTypeName, params)
	var result RebalanceResult
	s.NoError(s.workflowEnv.GetWorkflowResult(&result))
	s.Equal(2, len(result.SuccessDomains))
	s.Equal(0, len(result.FailedDomains))
}
