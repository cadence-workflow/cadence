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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

// aaPrefsJSON provides a three cluster setup of ClusterAttributes
const aaPrefsJSON = `[{"scope":"cluster","name":"cluster0","preferredCluster":"cluster0"},` +
	`{"scope":"cluster","name":"cluster1","preferredCluster":"cluster1"},` +
	`{"scope":"cluster","name":"cluster2","preferredCluster":"cluster2"}]`

// TestRebalancePreferencesForDomain tests the different combinations of live replication config and
// domain preferences that can be used to trigger a rebalance.
func TestRebalancePreferencesForDomain(t *testing.T) {
	tests := []struct {
		name      string
		domain    *types.DescribeDomainResponse
		wantErr   bool
		wantPrefs DomainFailoverPreferences
	}{
		{
			name:    "when domain is not managed it is skipped",
			domain:  createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster1", isManaged: false, isGlobal: true}),
			wantErr: true,
		},
		{
			name:    "when preferred equals active it makes no change",
			domain:  createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0"}}),
			wantErr: true,
		},
		{
			name:      "when preferred differs from active it moves to preferred",
			domain:    createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}}),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", TargetCluster: "cluster1"},
		},
		{
			name: "when one attribute is wrong it moves only that attribute",
			domain: createDomainResponse(createDomainResponseParams{
				name:              "d",
				activeClusterName: "cluster0",
				isManaged:         true,
				isGlobal:          true,
				data:              map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"cluster0": {ActiveClusterName: "cluster1"},
						"cluster1": {ActiveClusterName: "cluster1"},
						"cluster2": {ActiveClusterName: "cluster2"},
					}},
				}},
			}),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster0"},
			}},
		},
		{
			name: "when domain-level and two attributes are wrong it moves all of them",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "d",
					activeClusterName: "cluster0",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"cluster": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"cluster0": {ActiveClusterName: "cluster1"},
									"cluster1": {ActiveClusterName: "cluster0"},
									"cluster2": {ActiveClusterName: "cluster2"},
								},
							},
						},
					},
				},
			),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", TargetCluster: "cluster1", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster0"},
				{Scope: "cluster", Name: "cluster1", PreferredCluster: "cluster1"},
			}},
		},
		{
			name: "when only domain-level is wrong it moves only domain-level",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "d",
					activeClusterName: "cluster0",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"cluster": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"cluster0": {ActiveClusterName: "cluster0"},
									"cluster1": {ActiveClusterName: "cluster1"},
									"cluster2": {ActiveClusterName: "cluster2"},
								},
							},
						},
					},
				},
			),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", TargetCluster: "cluster1"},
		},
		{
			name: "when domain is not managed it is skipped",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "d",
					activeClusterName: "cluster0",
					isManaged:         false,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"cluster": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"cluster0": {ActiveClusterName: "cluster1"},
									"cluster1": {ActiveClusterName: "cluster1"},
									"cluster2": {ActiveClusterName: "cluster2"},
								},
							},
						},
					},
				},
			),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs, err := rebalancePreferencesForDomain(tt.domain)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrefs.DomainName, prefs.DomainName)
			assert.Equal(t, tt.wantPrefs.TargetCluster, prefs.TargetCluster)
			assert.ElementsMatch(t, tt.wantPrefs.ClusterAttributeUpdates, prefs.ClusterAttributeUpdates)
		})
	}
}

func TestGetClusterAttributePreferences(t *testing.T) {
	t.Run("when preferences are absent it returns nil", func(t *testing.T) {
		prefs, err := getClusterAttributePreferences(createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true}))
		require.NoError(t, err)
		assert.Nil(t, prefs)
	})
	t.Run("when JSON is valid it returns parsed preferences", func(t *testing.T) {
		d := createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON}})
		prefs, err := getClusterAttributePreferences(d)
		require.NoError(t, err)
		assert.Len(t, prefs, 3)
	})
	t.Run("when JSON is malformed it returns an unmarshal error", func(t *testing.T) {
		d := createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForClusterAttributePreferences: "{not-json"}})
		_, err := getClusterAttributePreferences(d)
		assert.ErrorIs(t, err, errUnmarshalClusterAttributePreferencesV2)
	})
}

func TestGetDomainsForRebalanceV2Activity_WhenListingDomainsItReturnsOnlyManagedDomainsNeedingAMove(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			createDomainResponse(createDomainResponseParams{name: "needs-move", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}}),
			createDomainResponse(createDomainResponseParams{name: "already-balanced", activeClusterName: "cluster0", isManaged: true, isGlobal: true, data: map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0"}}),
			createDomainResponse(createDomainResponseParams{name: "unmanaged", activeClusterName: "cluster0", isManaged: false, isGlobal: true, data: map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	val, err := env.ExecuteActivity(GetDomainsForRebalanceV2Activity)
	require.NoError(t, err)
	var prefs []DomainFailoverPreferences
	require.NoError(t, val.Get(&prefs))
	require.Len(t, prefs, 1)
	assert.Equal(t, "needs-move", prefs[0].DomainName)
	assert.Equal(t, "cluster1", prefs[0].TargetCluster)
}

func TestRebalanceWorkflowV2_WhenAllActivitiesSucceedItReturnsSuccessDomains(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(RebalanceWorkflowV2, workflow.RegisterOptions{Name: RebalanceWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForRebalanceV2Activity, activity.RegisterOptions{Name: getDomainsForRebalanceV2ActivityName})

	prefs := []DomainFailoverPreferences{{DomainName: "d1", TargetCluster: "cluster1"}}
	env.OnActivity(getDomainsForRebalanceV2ActivityName, mock.Anything).Return(prefs, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}}}, nil)

	env.ExecuteWorkflow(RebalanceWorkflowV2TypeName, &RebalanceV2Params{})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result RebalanceV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, []string{"d1"}, successDomainNames(result.SuccessDomains))
}

// TestRebalanceWorkflowV2_WhenStartedWithNilParamsItDoesNotPanic verifies the workflow guards against a
// nil params payload (e.g. a manual start or cron trigger that omits input) by applying defaults instead
// of panicking on a nil-pointer dereference, which would wedge the fixed-ID workflow.
func TestRebalanceWorkflowV2_WhenStartedWithNilParamsItDoesNotPanic(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(RebalanceWorkflowV2, workflow.RegisterOptions{Name: RebalanceWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForRebalanceV2Activity, activity.RegisterOptions{Name: getDomainsForRebalanceV2ActivityName})

	prefs := []DomainFailoverPreferences{{DomainName: "d1", TargetCluster: "cluster1"}}
	env.OnActivity(getDomainsForRebalanceV2ActivityName, mock.Anything).Return(prefs, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}}}, nil)

	var params *RebalanceV2Params
	env.ExecuteWorkflow(RebalanceWorkflowV2TypeName, params)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result RebalanceV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, []string{"d1"}, successDomainNames(result.SuccessDomains))
}

// TestRebalanceWorkflowV2_QueryReportsTotalDomains verifies the query handler reports a meaningful
// TotalDomains denominator once domains have been collected.
func TestRebalanceWorkflowV2_QueryReportsTotalDomains(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(RebalanceWorkflowV2, workflow.RegisterOptions{Name: RebalanceWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForRebalanceV2Activity, activity.RegisterOptions{Name: getDomainsForRebalanceV2ActivityName})

	prefs := []DomainFailoverPreferences{
		{DomainName: "d1", TargetCluster: "cluster1"},
		{DomainName: "d2", TargetCluster: "cluster1"},
	}
	env.OnActivity(getDomainsForRebalanceV2ActivityName, mock.Anything).Return(prefs, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}, {DomainName: "d2"}}}, nil)

	env.ExecuteWorkflow(RebalanceWorkflowV2TypeName, &RebalanceV2Params{})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	val, err := env.QueryWorkflow(QueryType)
	require.NoError(t, err)
	var qr QueryResult
	require.NoError(t, val.Get(&qr))
	assert.Equal(t, len(prefs), qr.TotalDomains)
}
