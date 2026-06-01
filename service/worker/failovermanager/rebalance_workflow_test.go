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

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity)
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

	_, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity)
	s.Error(err)
}

func (s *rebalanceWorkflowTestSuite) TestWorkflow_Success() {
	params := &RebalanceParams{
		BatchFailoverWaitTimeInSeconds: 10,
		BatchFailoverSize:              10,
	}
	// Covers all three rebalance scenarios in a single batch:
	//   d1 — global domain: only domain-level preferred cluster differs
	//   d2 — active-active domain: only cluster attributes differ, domain-level is already correct
	//   d3 — active-active domain: both domain-level and cluster attributes differ
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "d1",
			PreferredCluster: "c1",
		},
		{
			DomainName: "d2",
			ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
			},
		},
		{
			DomainName:       "d3",
			PreferredCluster: "c2",
			ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c1", PreferredCluster: "cluster1"},
			},
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(domainData, nil)
	// All domains go in a single batch with per-domain DomainPreferences; TargetCluster is empty.
	failoverActivityParams := &FailoverActivityParams{
		Domains:       []string{"d1", "d2", "d3"},
		TargetCluster: "",
		DomainPreferences: map[string]*DomainFailoverPreferences{
			"d1": {PreferredCluster: "c1"},
			"d2": {ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
			}},
			"d3": {
				PreferredCluster: "c2",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c1", PreferredCluster: "cluster1"},
				},
			},
		},
	}
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2", "d3"},
	}
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
	// Mixed domain types, mirroring TestWorkflow_Success: global + active-active cluster-attribute only + active-active both.
	domainData := []*DomainRebalanceData{
		{
			DomainName:       "d1",
			PreferredCluster: "c1",
		},
		{
			DomainName: "d2",
			ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
			},
		},
		{
			DomainName:       "d3",
			PreferredCluster: "c2",
			ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c1", PreferredCluster: "cluster1"},
			},
		},
	}
	s.workflowEnv.OnActivity(getRebalanceDomainsActivityName, mock.Anything).Return(domainData, nil)
	failoverActivityParams := &FailoverActivityParams{
		Domains:       []string{"d1", "d2", "d3"},
		TargetCluster: "",
		DomainPreferences: map[string]*DomainFailoverPreferences{
			"d1": {PreferredCluster: "c1"},
			"d2": {ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
			}},
			"d3": {
				PreferredCluster: "c2",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c1", PreferredCluster: "cluster1"},
				},
			},
		},
	}
	// Partial results: global domain and AA attr-only succeed; AA both-diff fails.
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
		FailedDomains:  []string{"d3"},
	}
	s.workflowEnv.OnActivity(failoverActivityName, mock.Anything, failoverActivityParams).Return(failoverActivityResult, nil).Times(1)
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

func TestClusterAttributeUpdatesNeeded(t *testing.T) {
	activeClusters := func(scopeToAttrs map[string]map[string]string) *types.ActiveClusters {
		ac := &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{}}
		for scope, attrs := range scopeToAttrs {
			info := map[string]types.ActiveClusterInfo{}
			for name, cluster := range attrs {
				info[name] = types.ActiveClusterInfo{ActiveClusterName: cluster}
			}
			ac.AttributeScopes[scope] = types.ClusterAttributeScope{ClusterAttributes: info}
		}
		return ac
	}
	makeDomain := func(ac *types.ActiveClusters) *types.DescribeDomainResponse {
		return &types.DescribeDomainResponse{
			DomainInfo: &types.DomainInfo{},
			ReplicationConfiguration: &types.DomainReplicationConfiguration{
				ActiveClusters: ac,
			},
		}
	}

	tests := []struct {
		name   string
		domain *types.DescribeDomainResponse
		prefs  []ClusterAttributePreference
		want   []ClusterAttributePreference
	}{
		{
			name:   "when domain has no ActiveClusters it should return nil",
			domain: makeDomain(nil),
			prefs:  []ClusterAttributePreference{{Scope: "city", Name: "prague", PreferredCluster: "c1"}},
			want:   nil,
		},
		{
			name:   "when all attributes match their preferences it should return nothing",
			domain: makeDomain(activeClusters(map[string]map[string]string{"city": {"prague": "c1"}})),
			prefs:  []ClusterAttributePreference{{Scope: "city", Name: "prague", PreferredCluster: "c1"}},
			want:   nil,
		},
		{
			name:   "when an attribute does not match its preferred cluster it should return it as an update",
			domain: makeDomain(activeClusters(map[string]map[string]string{"city": {"prague": "c2"}})),
			prefs:  []ClusterAttributePreference{{Scope: "city", Name: "prague", PreferredCluster: "c1"}},
			want:   []ClusterAttributePreference{{Scope: "city", Name: "prague", PreferredCluster: "c1"}},
		},
		{
			name: "when some attributes match and others do not it should return only mismatches",
			domain: makeDomain(activeClusters(map[string]map[string]string{
				"city": {"prague": "c1", "brno": "c2"},
			})),
			prefs: []ClusterAttributePreference{
				{Scope: "city", Name: "prague", PreferredCluster: "c1"}, // matches
				{Scope: "city", Name: "brno", PreferredCluster: "c1"},   // mismatch (actual: c2)
			},
			want: []ClusterAttributePreference{{Scope: "city", Name: "brno", PreferredCluster: "c1"}},
		},
		{
			name:   "when a preference references an attribute not in the domain it should skip it",
			domain: makeDomain(activeClusters(map[string]map[string]string{"city": {}})),
			prefs:  []ClusterAttributePreference{{Scope: "city", Name: "missing", PreferredCluster: "c1"}},
			want:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clusterAttributeUpdatesNeeded(tc.domain, tc.prefs)
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("expected nil/empty, got %v", got)
				}
			} else {
				if len(got) != len(tc.want) {
					t.Fatalf("length mismatch: want %d, got %d (%v)", len(tc.want), len(got), got)
				}
				for i := range tc.want {
					if got[i] != tc.want[i] {
						t.Errorf("element %d: want %v, got %v", i, tc.want[i], got[i])
					}
				}
			}
		})
	}
}

func TestGetClusterAttributePreferences(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]string
		want    []ClusterAttributePreference
		wantErr bool
	}{
		{
			name: "when ClusterAttributePreferences key is absent it should return nil",
			data: map[string]string{},
			want: nil,
		},
		{
			name: "when ClusterAttributePreferences contains valid JSON it should decode to typed preferences",
			data: map[string]string{
				constants.DomainDataKeyForClusterAttributePreferences: `[{"scope":"cluster","name":"c0","preferredCluster":"cluster0"}]`,
			},
			want: []ClusterAttributePreference{{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"}},
		},
		{
			name: "when ClusterAttributePreferences contains malformed JSON it should return an error",
			data: map[string]string{
				constants.DomainDataKeyForClusterAttributePreferences: `not-json`,
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain := &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{Name: "d1", Data: tc.data},
			}
			got, err := getClusterAttributePreferences(domain)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: want %d, got %d", len(tc.want), len(got))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("element %d: want %v, got %v", i, tc.want[i], got[i])
				}
			}
		})
	}
}
