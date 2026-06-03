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

// aaPrefsJSON mirrors the ClusterAttributePreferences stored by scripts/test_rebalance.sh: each
// attribute prefers the cluster of the same name.
const aaPrefsJSON = `[{"scope":"cluster","name":"cluster0","preferredCluster":"cluster0"},` +
	`{"scope":"cluster","name":"cluster1","preferredCluster":"cluster1"},` +
	`{"scope":"cluster","name":"cluster2","preferredCluster":"cluster2"}]`

// TestRebalancePreferencesForDomain maps 1:1 to the seven scenarios in scripts/test_rebalance.sh.
func TestRebalancePreferencesForDomain(t *testing.T) {
	tests := []struct {
		name      string
		domain    *types.DescribeDomainResponse
		wantErr   bool
		wantPrefs DomainFailoverPreferences
	}{
		{
			name:    "1. rb-global-unmanaged: not managed -> skipped",
			domain:  domainV2("d", "cluster1", false, true, nil, nil),
			wantErr: true,
		},
		{
			name:    "2. rb-global-same: preferred == active -> no change",
			domain:  domainV2("d", "cluster0", true, true, map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0"}, nil),
			wantErr: true,
		},
		{
			name:      "3. rb-global-diff: preferred != active -> move to preferred",
			domain:    domainV2("d", "cluster0", true, true, map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}, nil),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", PreferredCluster: "cluster1"},
		},
		{
			name: "4. rb-aa-attr-only: only one attribute wrong",
			domain: domainV2("d", "cluster0", true, true,
				map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				map[string]map[string]string{"cluster": {"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}}),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster0"},
			}},
		},
		{
			name: "5. rb-aa-both-diff: domain-level and two attributes wrong",
			domain: domainV2("d", "cluster0", true, true,
				map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				map[string]map[string]string{"cluster": {"cluster0": "cluster1", "cluster1": "cluster0", "cluster2": "cluster2"}}),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", PreferredCluster: "cluster1", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster0"},
				{Scope: "cluster", Name: "cluster1", PreferredCluster: "cluster1"},
			}},
		},
		{
			name: "6. rb-aa-no-attr-diff: only domain-level wrong",
			domain: domainV2("d", "cluster0", true, true,
				map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1", constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				map[string]map[string]string{"cluster": {"cluster0": "cluster0", "cluster1": "cluster1", "cluster2": "cluster2"}}),
			wantPrefs: DomainFailoverPreferences{DomainName: "d", PreferredCluster: "cluster1"},
		},
		{
			name: "7. rb-aa-unmanaged: not managed -> skipped",
			domain: domainV2("d", "cluster0", false, true, nil,
				map[string]map[string]string{"cluster": {"cluster0": "cluster1", "cluster1": "cluster1", "cluster2": "cluster2"}}),
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
			assert.Equal(t, tt.wantPrefs.PreferredCluster, prefs.PreferredCluster)
			assert.ElementsMatch(t, tt.wantPrefs.ClusterAttributeUpdates, prefs.ClusterAttributeUpdates)
		})
	}
}

func TestGetClusterAttributePreferencesV2(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		prefs, err := getClusterAttributePreferencesV2(domainV2("d", "cluster0", true, true, nil, nil))
		require.NoError(t, err)
		assert.Nil(t, prefs)
	})
	t.Run("valid", func(t *testing.T) {
		d := domainV2("d", "cluster0", true, true,
			map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON}, nil)
		prefs, err := getClusterAttributePreferencesV2(d)
		require.NoError(t, err)
		assert.Len(t, prefs, 3)
	})
	t.Run("malformed", func(t *testing.T) {
		d := domainV2("d", "cluster0", true, true,
			map[string]string{constants.DomainDataKeyForClusterAttributePreferences: "{not-json"}, nil)
		_, err := getClusterAttributePreferencesV2(d)
		assert.ErrorIs(t, err, errUnmarshalClusterAttributePreferencesV2)
	})
}

func TestGetDomainsForRebalanceV2Activity(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			domainV2("needs-move", "cluster0", true, true, map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}, nil),
			domainV2("already-balanced", "cluster0", true, true, map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster0"}, nil),
			domainV2("unmanaged", "cluster0", false, true, map[string]string{constants.DomainDataKeyForPreferredCluster: "cluster1"}, nil),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	val, err := env.ExecuteActivity(GetDomainsForRebalanceV2Activity)
	require.NoError(t, err)
	var prefs []DomainFailoverPreferences
	require.NoError(t, val.Get(&prefs))
	require.Len(t, prefs, 1)
	assert.Equal(t, "needs-move", prefs[0].DomainName)
	assert.Equal(t, "cluster1", prefs[0].PreferredCluster)
}

func TestRebalanceWorkflowV2_Success(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(RebalanceWorkflowV2, workflow.RegisterOptions{Name: RebalanceWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForRebalanceV2Activity, activity.RegisterOptions{Name: getDomainsForRebalanceV2ActivityName})

	prefs := []DomainFailoverPreferences{{DomainName: "d1", PreferredCluster: "cluster1"}}
	env.OnActivity(getDomainsForRebalanceV2ActivityName, mock.Anything).Return(prefs, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []string{"d1"}}, nil)

	env.ExecuteWorkflow(RebalanceWorkflowV2TypeName, &RebalanceV2Params{})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result RebalanceV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, []string{"d1"}, result.SuccessDomains)
}
