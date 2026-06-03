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

type createDomainResponseParams struct {
	name              string
	activeClusterName string
	isManaged         bool
	isGlobal          bool
	data              map[string]string
	activeClusters    *types.ActiveClusters
}

// createDomainResponse builds a DescribeDomainResponse for the V2 tests. attrs maps scope -> name -> activeCluster.
func createDomainResponse(params createDomainResponseParams) *types.DescribeDomainResponse {
	d := map[string]string{}
	for k, v := range params.data {
		d[k] = v
	}
	if params.isManaged {
		d[constants.DomainDataKeyForManagedFailover] = "true"
	}
	repl := &types.DomainReplicationConfiguration{
		ActiveClusterName: params.activeClusterName,
		Clusters: []*types.ClusterReplicationConfiguration{
			{ClusterName: "cluster0"}, {ClusterName: "cluster1"}, {ClusterName: "cluster2"},
		},
	}
	if params.activeClusters != nil {
		repl.ActiveClusters = params.activeClusters
	}
	return &types.DescribeDomainResponse{
		IsGlobalDomain:           params.isGlobal,
		DomainInfo:               &types.DomainInfo{Name: params.name, Data: d},
		ReplicationConfiguration: repl,
	}
}

func TestIsEligibleForFailover(t *testing.T) {
	tests := []struct {
		name   string
		domain *types.DescribeDomainResponse
		want   bool
	}{
		{"when domain is nil it is not eligible", nil, false},
		{"when DomainInfo is nil it is not eligible", &types.DescribeDomainResponse{}, false},
		{"when domain is managed and global it is eligible", createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true}), true},
		{"when domain is not managed it is not eligible", createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: false, isGlobal: true}), false},
		{"when domain is not global it is not eligible", createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: false}), false},
		{
			"when domain is deprecated it is not eligible",
			func() *types.DescribeDomainResponse {
				d := createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster0", isManaged: true, isGlobal: true})
				d.DomainInfo.Status = types.DomainStatusDeprecated.Ptr()
				return d
			}(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isEligibleForFailover(tt.domain))
		})
	}
}

func TestCollectTargetClusters_WhenGivenPreferencesItReturnsUniqueTargetClusters(t *testing.T) {
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

func TestBuildActiveClustersFromUpdates_WhenGivenUpdatesItGroupsThemByScopeAndName(t *testing.T) {
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

func TestDomainNames_WhenGivenPreferencesItReturnsNamesAndNilForNilInput(t *testing.T) {
	assert.Nil(t, domainNames(nil))
	got := domainNames([]DomainFailoverPreferences{{DomainName: "a"}, {DomainName: "b"}})
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestProcessInBatches_WhenItemsExceedBatchSizeItSplitsIntoSizedBatches(t *testing.T) {
	prefs := []DomainFailoverPreferences{
		{DomainName: "a"}, {DomainName: "b"}, {DomainName: "c"}, {DomainName: "d"}, {DomainName: "e"},
	}
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	var batchSizes []int
	wf := func(ctx workflow.Context) ([]string, error) {
		success, _ := processInBatches(ctx, prefs, 2, 0, nil,
			func(ctx workflow.Context, batch []DomainFailoverPreferences) (s, f []string) {
				batchSizes = append(batchSizes, len(batch))
				return domainNames(batch), nil
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

func TestFailoverActivityV2_WhenGivenPreferencesItAppliesThemAndReportsSuccessDomains(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	withPollers := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{},
	}
	mockResource.FrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(withPollers, nil).AnyTimes()
	mockResource.RemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(withPollers, nil).AnyTimes()
	mockResource.FrontendClient.EXPECT().FailoverDomain(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)

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
