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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// domainV2 builds a DescribeDomainResponse for the V2 tests. attrs maps scope -> name -> activeCluster.
func domainV2(name, active string, managed, global bool, data map[string]string, attrs map[string]map[string]string) *types.DescribeDomainResponse {
	d := map[string]string{}
	for k, v := range data {
		d[k] = v
	}
	if managed {
		d[constants.DomainDataKeyForManagedFailover] = "true"
	}
	repl := &types.DomainReplicationConfiguration{
		ActiveClusterName: active,
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: "cluster0"}, {ClusterName: "cluster1"}, {ClusterName: "cluster2"},
		},
	}
	if attrs != nil {
		scopes := map[string]types.ClusterAttributeScope{}
		for scope, names := range attrs {
			ca := map[string]types.ActiveClusterInfo{}
			for n, c := range names {
				ca[n] = types.ActiveClusterInfo{ActiveClusterName: c}
			}
			scopes[scope] = types.ClusterAttributeScope{ClusterAttributes: ca}
		}
		repl.ActiveClusters = &types.ActiveClusters{AttributeScopes: scopes}
	}
	return &types.DescribeDomainResponse{
		IsGlobalDomain:           global,
		DomainInfo:               &types.DomainInfo{Name: name, Data: d},
		ReplicationConfiguration: repl,
	}
}

func TestEligibleForFailoverV2(t *testing.T) {
	tests := []struct {
		name   string
		domain *types.DescribeDomainResponse
		want   bool
	}{
		{"nil", nil, false},
		{"nil domain info", &types.DescribeDomainResponse{}, false},
		{"managed global", domainV2("d", "cluster0", true, true, nil, nil), true},
		{"unmanaged", domainV2("d", "cluster0", false, true, nil, nil), false},
		{"not global", domainV2("d", "cluster0", true, false, nil, nil), false},
		{
			"deprecated",
			func() *types.DescribeDomainResponse {
				d := domainV2("d", "cluster0", true, true, nil, nil)
				dep := types.DomainStatusDeprecated
				d.DomainInfo.Status = &dep
				return d
			}(),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, eligibleForFailoverV2(tt.domain))
		})
	}
}

func TestCollectTargetClusters(t *testing.T) {
	got := collectTargetClusters(DomainFailoverPreferences{
		PreferredCluster: "cluster1",
		ClusterAttributeUpdates: []ClusterAttributePreference{
			{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
			{Scope: "cluster", Name: "cluster2", PreferredCluster: "cluster2"},
		},
	})
	sort.Strings(got)
	assert.Equal(t, []string{"cluster1", "cluster2"}, got)
}

func TestBuildActiveClustersFromUpdates(t *testing.T) {
	ac := buildActiveClustersFromUpdates([]ClusterAttributePreference{
		{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
		{Scope: "cluster", Name: "cluster1", PreferredCluster: "cluster1"},
		{Scope: "region", Name: "us-west", PreferredCluster: "cluster2"},
	})
	require.NotNil(t, ac)
	assert.Equal(t, "cluster1", ac.AttributeScopes["cluster"].ClusterAttributes["cluster0"].ActiveClusterName)
	assert.Equal(t, "cluster1", ac.AttributeScopes["cluster"].ClusterAttributes["cluster1"].ActiveClusterName)
	assert.Equal(t, "cluster2", ac.AttributeScopes["region"].ClusterAttributes["us-west"].ActiveClusterName)
}

func TestDomainNamesV2(t *testing.T) {
	assert.Nil(t, domainNamesV2(nil))
	got := domainNamesV2([]DomainFailoverPreferences{{DomainName: "a"}, {DomainName: "b"}})
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestProcessInBatchesV2(t *testing.T) {
	prefs := []DomainFailoverPreferences{
		{DomainName: "a"}, {DomainName: "b"}, {DomainName: "c"}, {DomainName: "d"}, {DomainName: "e"},
	}
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	var batchSizes []int
	wf := func(ctx workflow.Context) ([]string, error) {
		success, _ := processInBatchesV2(ctx, prefs, 2, 0, nil,
			func(ctx workflow.Context, batch []DomainFailoverPreferences) (s, f []string) {
				batchSizes = append(batchSizes, len(batch))
				return domainNamesV2(batch), nil
			})
		return success, nil
	}
	env.RegisterWorkflow(wf)
	env.ExecuteWorkflow(wf)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var success []string
	require.NoError(t, env.GetWorkflowResult(&success))
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, success)
	assert.Equal(t, []int{2, 2, 1}, batchSizes)
}

// newFailoverV2ActivityEnv wires a TestActivityEnvironment with a FailoverManager backed by mock
// frontend clients, matching the production activity context.
func newFailoverV2ActivityEnv(t *testing.T) (*testsuite.TestActivityEnvironment, *resource.Test) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	ctrl := gomock.NewController(t)
	mockResource := resource.NewTest(t, ctrl, metrics.Worker)
	mgr := &FailoverManager{
		svcClient:  mockResource.GetSDKClient(),
		clientBean: mockResource.ClientBean,
	}
	env.SetWorkerOptions(worker.Options{
		BackgroundActivityContext: context.WithValue(context.Background(), failoverManagerContextKey, mgr),
	})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})
	env.RegisterActivityWithOptions(GetDomainsForRebalanceV2Activity, activity.RegisterOptions{Name: getDomainsForRebalanceV2ActivityName})
	t.Cleanup(func() { mockResource.Finish(t) })
	return env, mockResource
}

func TestFailoverActivityV2_Apply(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	withPollers := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{},
	}
	mockResource.FrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(withPollers, nil).AnyTimes()
	mockResource.RemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(withPollers, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)

	val, err := env.ExecuteActivity(FailoverActivityV2, &FailoverActivityV2Params{
		DomainPreferences: []DomainFailoverPreferences{
			{DomainName: "d1", PreferredCluster: "cluster1"},
			{DomainName: "d2", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
			}},
		},
	})
	require.NoError(t, err)
	var result FailoverActivityV2Result
	require.NoError(t, val.Get(&result))
	sort.Strings(result.SuccessDomains)
	assert.Equal(t, []string{"d1", "d2"}, result.SuccessDomains)
	assert.Empty(t, result.FailedDomains)
}
