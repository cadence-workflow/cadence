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
	"reflect"
	"strings"
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
	s.workflowEnv.RegisterActivityWithOptions(RebalanceActivity, activity.RegisterOptions{Name: rebalanceActivityName})
	s.workflowEnv.RegisterActivityWithOptions(GetDomainsForRebalanceActivity, activity.RegisterOptions{Name: getRebalanceDomainsActivityName})
	s.activityEnv.RegisterActivityWithOptions(RebalanceActivity, activity.RegisterOptions{Name: rebalanceActivityName})
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
	var domainData []DomainFailoverPreferences
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

// TestGetDomainsForRebalanceActivity_AttributeOnly covers a domain whose ClusterAttributePreferences
// stored in domain data disagree with the live ActiveClusters config, with no PreferredCluster key
// set. The discovery activity should return a DomainFailoverPreferences entry that has ONLY
// ClusterAttributeUpdates populated (PreferredCluster left empty), so the downstream rebalance
// touches the attribute but leaves the domain-level ActiveClusterName alone.
func (s *rebalanceWorkflowTestSuite) TestGetDomainsForRebalanceActivity_AttributeOnly() {
	actEnv, mockResource := s.prepareTestActivityEnv()

	prefsJSON := `[{"scope":"cluster","name":"c0","preferredCluster":"cluster0"}]`
	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name:   "attr-only-domain",
					Status: types.DomainStatusRegistered.Ptr(),
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForClusterAttributePreferences: prefsJSON,
					},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					// Domain-level ActiveClusterName is "cluster0" — but no PreferredCluster
					// is set in domain data, so this should NOT trigger a domain-level update.
					ActiveClusterName: "cluster0",
					Clusters:          clusters,
					// Active-active attribute c0 is currently on "c1" — the stored preference
					// wants it on "cluster0", so this attribute needs an update.
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
								"c0": {ActiveClusterName: "c1"},
							}},
						},
					},
				},
				IsGlobalDomain: true,
			},
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	actFuture, err := actEnv.ExecuteActivity(GetDomainsForRebalanceActivity)
	s.NoError(err)
	var domainData []DomainFailoverPreferences
	s.NoError(actFuture.Get(&domainData))
	s.Require().Equal(1, len(domainData))
	s.Equal("attr-only-domain", domainData[0].DomainName)
	s.Equal("", domainData[0].PreferredCluster, "PreferredCluster should stay empty when domain data has no preferredCluster key")
	s.Equal(
		[]ClusterAttributePreference{{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"}},
		domainData[0].ClusterAttributeUpdates,
	)
}

// TestRebalanceActivity_AttributeOnly confirms that when RebalanceActivity is handed prefs that
// only carry ClusterAttributeUpdates (no PreferredCluster), the resulting UpdateDomain request
// sets ActiveClusters but leaves ActiveClusterName nil — the domain-level active cluster stays
// where it is.
func TestRebalanceActivity_AttributeOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRes := resource.NewTest(t, ctrl, metrics.Worker)

	pollerMap := map[string]*types.DescribeTaskListResponse{
		"tl": {Pollers: []*types.PollerInfo{{Identity: "worker"}}},
	}
	pollerResp := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: pollerMap,
		ActivityTaskListMap: pollerMap,
	}
	mockRes.FrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(pollerResp, nil).Times(1)
	mockRes.RemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(pollerResp, nil).Times(1)

	wantUpdate := &types.UpdateDomainRequest{
		Name: "attr-only-domain",
		// No ActiveClusterName: domain-level cluster must not change.
		ActiveClusters: &types.ActiveClusters{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
					"c0": {ActiveClusterName: "cluster0"},
				}},
			},
		},
		// No FailoverTimeoutInSeconds: rebalance never sets graceful timeout.
	}
	mockRes.FrontendClient.EXPECT().UpdateDomain(gomock.Any(), wantUpdate).Return(nil, nil).Times(1)

	ctx := context.WithValue(
		context.Background(),
		failoverManagerContextKey,
		&FailoverManager{svcClient: mockRes.GetSDKClient(), clientBean: mockRes.ClientBean},
	)
	env := testsuite.WorkflowTestSuite{}
	actEnv := env.NewTestActivityEnvironment()
	actEnv.SetWorkerOptions(worker.Options{BackgroundActivityContext: ctx})
	actEnv.RegisterActivityWithOptions(RebalanceActivity, activity.RegisterOptions{Name: rebalanceActivityName})

	params := &RebalanceActivityParams{
		DomainPreferences: []DomainFailoverPreferences{
			{
				DomainName: "attr-only-domain",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
				},
			},
		},
	}
	fut, err := actEnv.ExecuteActivity(rebalanceActivityName, params)
	if err != nil {
		t.Fatalf("activity error: %v", err)
	}
	var result FailoverActivityResult
	if err := fut.Get(&result); err != nil {
		t.Fatalf("get result error: %v", err)
	}
	if len(result.SuccessDomains) != 1 || result.SuccessDomains[0] != "attr-only-domain" {
		t.Errorf("want SuccessDomains=[attr-only-domain], got %v", result.SuccessDomains)
	}
	if len(result.FailedDomains) != 0 {
		t.Errorf("want FailedDomains=[], got %v", result.FailedDomains)
	}

	mockRes.Finish(t)
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
	domainData := []DomainFailoverPreferences{
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
	// All entries go in a single batch — the batch helper slices the prefs directly.
	rebalanceActivityParams := &RebalanceActivityParams{DomainPreferences: domainData}
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2", "d3"},
	}
	s.workflowEnv.OnActivity(rebalanceActivityName, mock.Anything, rebalanceActivityParams).Return(failoverActivityResult, nil).Times(1)
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
	domainData := []DomainFailoverPreferences{
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
	rebalanceActivityParams := &RebalanceActivityParams{DomainPreferences: domainData}
	// Partial results: global domain and AA attr-only succeed; AA both-diff fails.
	failoverActivityResult := &FailoverActivityResult{
		SuccessDomains: []string{"d1", "d2"},
		FailedDomains:  []string{"d3"},
	}
	s.workflowEnv.OnActivity(rebalanceActivityName, mock.Anything, rebalanceActivityParams).Return(failoverActivityResult, nil).Times(1)
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
	domainData := []DomainFailoverPreferences{
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
	s.workflowEnv.OnActivity(rebalanceActivityName, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("test"))
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

// getPreferencesForDomainWrapper is registered as an activity so getPreferencesForDomain runs
// under a real activity context — required because the function calls activity.GetLogger.
func getPreferencesForDomainWrapper(ctx context.Context, domain *types.DescribeDomainResponse) (DomainFailoverPreferences, error) {
	return getPreferencesForDomain(ctx, domain)
}

// TestGetPreferencesForDomain exhaustively covers the per-domain rebalance decision: domain
// eligibility (status, global, managed-by-cadence), preferred-cluster handling (empty, matches
// active, not in cluster list, valid), attribute-level handling (no JSON, malformed JSON,
// matching attr, mismatched attr, both), and the no-work ErrNoRebalanceRequired path. The
// scenarios marked "ported from TestShouldAllowRebalance" preserve the eligibility coverage
// that used to live in that deleted suite test.
func TestGetPreferencesForDomain(t *testing.T) {
	const validPrefsJSON = `[{"scope":"cluster","name":"c0","preferredCluster":"cluster0"}]`

	// activeClustersWith builds an ActiveClusters with one scope:name mapped to active.
	activeClustersWith := func(scope, name, active string) *types.ActiveClusters {
		return &types.ActiveClusters{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				scope: {ClusterAttributes: map[string]types.ActiveClusterInfo{
					name: {ActiveClusterName: active},
				}},
			},
		}
	}

	tests := []struct {
		name       string
		domain     *types.DescribeDomainResponse
		want       DomainFailoverPreferences
		wantErrSub string // substring match on err.Error(); empty means expect no error.
	}{
		// --- eligibility (ported from TestShouldAllowRebalance) ---
		{
			name: "domain not managed by cadence is ineligible",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "false",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: "not eligible",
		},
		{
			name: "non-global domain is ineligible",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           false,
			},
			wantErrSub: "not eligible",
		},
		{
			name: "deprecated domain is ineligible",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusDeprecated.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: "not eligible",
		},
		{
			name: "deleted domain is ineligible",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusDeleted.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: "not eligible",
		},

		// --- ErrNoRebalanceRequired (eligible but nothing to do) ---
		{
			name: "eligible domain with no preferred cluster and no attribute prefs needs no rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name:   "d1",
					Data:   map[string]string{constants.DomainDataKeyForManagedFailover: "true"},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},
		{
			name: "preferred cluster already matches active and no attribute prefs needs no rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c1",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},
		{
			name: "preferred cluster not in cluster list is ignored and yields no rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c3",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},
		{
			name: "attribute prefs that already match current state need no rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForClusterAttributePreferences: validPrefsJSON,
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters:    activeClustersWith("cluster", "c0", "cluster0"),
				},
				IsGlobalDomain: true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},

		// --- domain-level only ---
		{
			name: "preferred cluster differs and is valid returns domain-level prefs only",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:  "true",
						constants.DomainDataKeyForPreferredCluster: "c2",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			want: DomainFailoverPreferences{DomainName: "d1", PreferredCluster: "c2"},
		},

		// --- attribute-level only ---
		{
			name: "attribute mismatch with no preferred cluster returns attribute updates only",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForClusterAttributePreferences: validPrefsJSON,
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "cluster0",
					Clusters:          clusters,
					// Attribute c0 is currently on c1 — preference wants it on cluster0.
					ActiveClusters: activeClustersWith("cluster", "c0", "c1"),
				},
				IsGlobalDomain: true,
			},
			want: DomainFailoverPreferences{
				DomainName: "d1",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
				},
			},
		},
		{
			name: "preferred matches active but attribute mismatches returns attribute updates only",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForPreferredCluster:            "c1", // matches active, ignored
						constants.DomainDataKeyForClusterAttributePreferences: validPrefsJSON,
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters:    activeClustersWith("cluster", "c0", "c1"),
				},
				IsGlobalDomain: true,
			},
			want: DomainFailoverPreferences{
				DomainName: "d1",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
				},
			},
		},

		// --- both domain-level and attribute-level ---
		{
			name: "preferred differs and attribute mismatches returns both",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForPreferredCluster:            "c2",
						constants.DomainDataKeyForClusterAttributePreferences: validPrefsJSON,
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters:    activeClustersWith("cluster", "c0", "c1"),
				},
				IsGlobalDomain: true,
			},
			want: DomainFailoverPreferences{
				DomainName:       "d1",
				PreferredCluster: "c2",
				ClusterAttributeUpdates: []ClusterAttributePreference{
					{Scope: "cluster", Name: "c0", PreferredCluster: "cluster0"},
				},
			},
		},

		// --- malformed JSON: logged and skipped, but domain-level pref still applied ---
		{
			name: "malformed cluster attribute prefs JSON does not block domain-level rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForPreferredCluster:            "c2",
						constants.DomainDataKeyForClusterAttributePreferences: "not-json",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			want: DomainFailoverPreferences{DomainName: "d1", PreferredCluster: "c2"},
		},
		{
			name: "malformed cluster attribute prefs JSON with no other work yields no rebalance",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForClusterAttributePreferences: "not-json",
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{ActiveClusterName: "c1", Clusters: clusters},
				IsGlobalDomain:           true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},

		// --- attribute pref referencing missing attribute is skipped ---
		{
			name: "attribute pref for missing scope:name is ignored",
			domain: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					Name: "d1",
					Data: map[string]string{
						constants.DomainDataKeyForManagedFailover:             "true",
						constants.DomainDataKeyForClusterAttributePreferences: `[{"scope":"cluster","name":"unknown","preferredCluster":"cluster0"}]`,
					},
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "c1",
					Clusters:          clusters,
					ActiveClusters:    activeClustersWith("cluster", "c0", "c1"),
				},
				IsGlobalDomain: true,
			},
			wantErrSub: ErrNoRebalanceRequired.Error(),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			suite := testsuite.WorkflowTestSuite{}
			actEnv := suite.NewTestActivityEnvironment()
			actEnv.RegisterActivity(getPreferencesForDomainWrapper)

			fut, err := actEnv.ExecuteActivity(getPreferencesForDomainWrapper, tc.domain)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got DomainFailoverPreferences
			if err := fut.Get(&got); err != nil {
				t.Fatalf("get result error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("prefs mismatch\n got: %#v\nwant: %#v", got, tc.want)
			}
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
