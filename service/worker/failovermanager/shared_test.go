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
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"
	"go.uber.org/zap"

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

func TestFailuresFromBatch_WhenGivenPreferencesItMarksAllFailedWithErrorAndNilForNilInput(t *testing.T) {
	assert.Nil(t, failuresFromBatch(nil, errors.New("boom")))
	got := failuresFromBatch([]DomainFailoverPreferences{{DomainName: "a"}, {DomainName: "b"}}, errors.New("boom"))
	assert.Equal(t, []DomainFailoverFailure{
		{DomainName: "a", Error: "boom"},
		{DomainName: "b", Error: "boom"},
	}, got)
}

func TestDomainNameExtractors_ReturnNamesAndNilForNilInput(t *testing.T) {
	assert.Nil(t, successDomainNames(nil))
	assert.Nil(t, failedDomainNames(nil))
	assert.Equal(t, []string{"a", "b"}, successDomainNames([]DomainFailoverSuccess{{DomainName: "a"}, {DomainName: "b"}}))
	assert.Equal(t, []string{"a", "b"}, failedDomainNames([]DomainFailoverFailure{{DomainName: "a"}, {DomainName: "b"}}))
}

func TestClustersTouchedByPreferences(t *testing.T) {
	tests := []struct {
		name  string
		prefs DomainFailoverPreferences
		want  []string
	}{
		{
			name:  "when the preferences only contains the domain default, it should return the domain default target cluster",
			prefs: DomainFailoverPreferences{DomainName: "d", TargetCluster: "cluster1"},
			want:  []string{"cluster1"},
		},
		{
			name: "when the preferences contains multiple cluster attribute updates, it should return all distinct target clusters",
			prefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
				{Scope: "region", Name: "us-west", PreferredCluster: "cluster2"},
			}},
			want: []string{"cluster1", "cluster2"},
		},
		{
			name: "when the preferences contains both domain-level and cluster attribute updates, it should return only distinct target clusters",
			prefs: DomainFailoverPreferences{DomainName: "d", TargetCluster: "cluster1", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
				{Scope: "cluster", Name: "cluster2", PreferredCluster: "cluster2"},
			}},
			want: []string{"cluster1", "cluster2"},
		},
		{
			name:  "when no clusters are specified, it should return an empty list",
			prefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{{Scope: "cluster", Name: "cluster0", PreferredCluster: ""}}},
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clustersTouchedByPreferences(tt.prefs))
		})
	}
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
			func(ctx workflow.Context, batch []DomainFailoverPreferences) (s []DomainFailoverSuccess, f []DomainFailoverFailure) {
				batchSizes = append(batchSizes, len(batch))
				for _, p := range batch {
					s = append(s, DomainFailoverSuccess{DomainName: p.DomainName})
				}
				return s, nil
			})
		return successDomainNames(success), nil
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

// taskListMapWithPollers returns a task-list map with a single task list that has an active poller.
func taskListMapWithPollers() map[string]*types.DescribeTaskListResponse {
	return map[string]*types.DescribeTaskListResponse{
		"tl": {Pollers: []*types.PollerInfo{{Identity: "test"}}},
	}
}

// expectPollersPresent makes the destination poller check pass for any domain: both the local and the
// remote frontend report a task list with pollers, so warnIfMissingPollers logs nothing. Wired with
// AnyTimes so it is a no-op for tests whose domains are all skipped before the check runs.
func expectPollersPresent(mockResource *resource.Test) {
	resp := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: taskListMapWithPollers(),
		ActivityTaskListMap: taskListMapWithPollers(),
	}
	mockResource.FrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(resp, nil).AnyTimes()
	mockResource.RemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(resp, nil).AnyTimes()
}

// expectDestinationMissingPollers simulates a destination cluster with no pollers: the local frontend
// reports task lists with pollers but the remote (destination) frontend reports none, so the poller
// check fails and warnIfMissingPollers logs a warning — without skipping the domain.
func expectDestinationMissingPollers(mockResource *resource.Test) {
	local := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: taskListMapWithPollers(),
		ActivityTaskListMap: taskListMapWithPollers(),
	}
	remote := &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{"tl": {}},
		ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{"tl": {}},
	}
	mockResource.FrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(local, nil).AnyTimes()
	mockResource.RemoteFrontendClient.EXPECT().GetTaskListsByDomain(gomock.Any(), gomock.Any()).Return(remote, nil).AnyTimes()
}

func TestFailoverActivityV2_WhenGivenPreferencesItAppliesThemAndReportsSuccessDomains(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	mockResource.FrontendClient.EXPECT().FailoverDomain(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)

	val, err := env.ExecuteActivity(FailoverActivityV2, &FailoverActivityV2Params{
		DomainPreferences: []DomainFailoverPreferences{
			{DomainName: "d1", TargetCluster: "cluster1"},
			{DomainName: "d2", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
			}},
		},
	})
	require.NoError(t, err)
	var result FailoverActivityV2Result
	require.NoError(t, val.Get(&result))
	got := successDomainNames(result.SuccessDomains)
	sort.Strings(got)
	assert.Equal(t, []string{"d1", "d2"}, got)
	assert.Empty(t, result.FailedDomains)
}

func TestFailoverActivityV2_WhenADomainFailsItIsReportedFailedWithTheError(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	mockResource.FrontendClient.EXPECT().FailoverDomain(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	val, err := env.ExecuteActivity(FailoverActivityV2, &FailoverActivityV2Params{
		DomainPreferences: []DomainFailoverPreferences{
			{DomainName: "d1", TargetCluster: "cluster1"},
		},
	})
	require.NoError(t, err)
	var result FailoverActivityV2Result
	require.NoError(t, val.Get(&result))
	assert.Empty(t, result.SuccessDomains)
	require.Len(t, result.FailedDomains, 1)
	assert.Equal(t, "d1", result.FailedDomains[0].DomainName)
	assert.Contains(t, result.FailedDomains[0].Error, "boom")
}

func TestGetFailoverTimeoutSeconds(t *testing.T) {
	tests := []struct {
		name string
		data map[string]string
		want int32
	}{
		{"missing key", nil, 0},
		{"empty value", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: ""}, 0},
		{"zero", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: "0"}, 0},
		{"negative", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: "-5"}, 0},
		{"non-numeric", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: "abc"}, 0},
		{"valid", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: "300"}, 300},
		{"valid small", map[string]string{constants.DomainDataKeyForFailoverTimeoutSeconds: "1"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain := createDomainResponse(createDomainResponseParams{
				name:              "test",
				activeClusterName: "cluster0",
				isManaged:         true,
				isGlobal:          true,
				data:              tc.data,
			})
			assert.Equal(t, tc.want, getFailoverTimeoutSeconds(domain, zap.NewNop()))
		})
	}
}

func TestFailoverActivityV2_WhenDomainHasTimeoutItPassesGracefulFailover(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	var capturedReq *types.FailoverDomainRequest
	mockResource.FrontendClient.EXPECT().FailoverDomain(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *types.FailoverDomainRequest, _ ...yarpc.CallOption) (*types.FailoverDomainResponse, error) {
			capturedReq = req
			return nil, nil
		})

	val, err := env.ExecuteActivity(FailoverActivityV2, &FailoverActivityV2Params{
		DomainPreferences: []DomainFailoverPreferences{
			{DomainName: "d1", TargetCluster: "cluster1", FailoverTimeoutSeconds: 300},
		},
	})
	require.NoError(t, err)
	var result FailoverActivityV2Result
	require.NoError(t, val.Get(&result))
	assert.Len(t, result.SuccessDomains, 1)
	require.NotNil(t, capturedReq.FailoverTimeoutInSeconds)
	assert.Equal(t, int32(300), *capturedReq.FailoverTimeoutInSeconds)
}

func TestFailoverActivityV2_WhenDomainHasNoTimeoutItUsesForceFailover(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	var capturedReq *types.FailoverDomainRequest
	mockResource.FrontendClient.EXPECT().FailoverDomain(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *types.FailoverDomainRequest, _ ...yarpc.CallOption) (*types.FailoverDomainResponse, error) {
			capturedReq = req
			return nil, nil
		})

	val, err := env.ExecuteActivity(FailoverActivityV2, &FailoverActivityV2Params{
		DomainPreferences: []DomainFailoverPreferences{
			{DomainName: "d1", TargetCluster: "cluster1"},
		},
	})
	require.NoError(t, err)
	var result FailoverActivityV2Result
	require.NoError(t, val.Get(&result))
	assert.Len(t, result.SuccessDomains, 1)
	assert.Nil(t, capturedReq.FailoverTimeoutInSeconds)
}
