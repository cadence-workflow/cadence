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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/testsuite"
	"go.uber.org/cadence/workflow"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/types"
)

func TestValidateFailoverV2Params(t *testing.T) {
	assert.Error(t, validateFailoverV2Params(nil))
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{TargetCluster: "t"}))                     // no source
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{SourceCluster: "s"}))                     // no target
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{SourceCluster: "x", TargetCluster: "x"})) // same

	p := &FailoverV2Params{SourceCluster: "cluster0", TargetCluster: "cluster1"}
	require.NoError(t, validateFailoverV2Params(p))
	assert.Equal(t, defaultBatchSizeV2, p.BatchSize)
	assert.Equal(t, defaultWaitBetweenBatchSecondsV2, p.WaitBetweenBatchSeconds)
}

// TestFailoverPreferencesForDomain covers the failover-equivalent of the rebalance scenarios in
// scripts/test_rebalance.sh, plus N-region correctness. Source is always cluster0, target cluster1.
func TestFailoverPreferencesForDomain(t *testing.T) {
	const src, tgt = "cluster0", "cluster1"
	tests := []struct {
		name         string
		domain       *types.DescribeDomainResponse
		wantOK       bool
		wantPrefs    DomainFailoverPreferences
		wantSnapshot DomainSnapshot
	}{
		{
			name:         "domain-level active on source moves to target",
			domain:       domainV2("d", src, true, true, nil, nil),
			wantOK:       true,
			wantPrefs:    DomainFailoverPreferences{DomainName: "d", PreferredCluster: tgt},
			wantSnapshot: DomainSnapshot{DomainName: "d", PreviousActiveCluster: src},
		},
		{
			name:   "domain-level active on a third cluster is untouched",
			domain: domainV2("d", "cluster2", true, true, nil, nil),
			wantOK: false,
		},
		{
			name: "active-active attribute on source moves to target",
			domain: domainV2("d", "cluster1", true, true, nil, map[string]map[string]string{
				"cluster": {"cluster0": src, "cluster1": "cluster1"},
			}),
			wantOK: true,
			wantPrefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: tgt},
			}},
			wantSnapshot: DomainSnapshot{DomainName: "d", PreviousClusterAttributes: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: src},
			}},
		},
		{
			name: "active-active attribute not on source is untouched",
			domain: domainV2("d", "cluster1", true, true, nil, map[string]map[string]string{
				"cluster": {"cluster1": "cluster1", "cluster2": "cluster2"},
			}),
			wantOK: false,
		},
		{
			name: "both domain-level and attribute on source are both moved",
			domain: domainV2("d", src, true, true, nil, map[string]map[string]string{
				"cluster": {"cluster0": src, "cluster1": "cluster1"},
			}),
			wantOK: true,
			wantPrefs: DomainFailoverPreferences{DomainName: "d", PreferredCluster: tgt, ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: tgt},
			}},
			wantSnapshot: DomainSnapshot{DomainName: "d", PreviousActiveCluster: src, PreviousClusterAttributes: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: src},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs, snapshot, ok := failoverPreferencesForDomain(tt.domain, src, tgt)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			assert.Equal(t, tt.wantPrefs, prefs)
			assert.Equal(t, tt.wantSnapshot, snapshot)
		})
	}
}

func TestGetDomainsForFailoverV2Activity(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			domainV2("managed-on-source", "cluster0", true, true, nil, nil),
			domainV2("managed-on-third", "cluster2", true, true, nil, nil),
			domainV2("unmanaged", "cluster0", false, true, nil, nil),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceCluster: "cluster0",
		TargetCluster: "cluster1",
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	require.Len(t, result.Preferences, 1)
	assert.Equal(t, "managed-on-source", result.Preferences[0].DomainName)
	assert.Equal(t, "cluster1", result.Preferences[0].PreferredCluster)
	require.Len(t, result.Snapshots, 1)
	assert.Equal(t, "cluster0", result.Snapshots[0].PreviousActiveCluster)
}

func TestFailoverWorkflowV2_InvalidParams(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{})
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestFailoverWorkflowV2_Success(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})

	collected := &GetDomainsForFailoverV2Result{
		Preferences: []DomainFailoverPreferences{{DomainName: "d1", PreferredCluster: "cluster1"}},
		Snapshots:   []DomainSnapshot{{DomainName: "d1", PreviousActiveCluster: "cluster0"}},
	}
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(collected, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []string{"d1"}}, nil)

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{
		SourceCluster: "cluster0", TargetCluster: "cluster1",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result FailoverV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, []string{"d1"}, result.SuccessDomains)
	assert.Equal(t, collected.Snapshots, result.Snapshots)
}

func TestFailoverWorkflowV2_GetDomainsError(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{SourceCluster: "cluster0", TargetCluster: "cluster1"})
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestFailoverWorkflowV2_PauseResume(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})

	collected := &GetDomainsForFailoverV2Result{
		Preferences: []DomainFailoverPreferences{
			{DomainName: "d1", PreferredCluster: "cluster1"},
			{DomainName: "d2", PreferredCluster: "cluster1"},
		},
	}
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(collected, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []string{"d1", "d2"}}, nil).Once()

	// Pause before the first batch runs; the workflow blocks at the batch boundary and reports
	// paused until resumed. This mirrors the V1 pause test pattern.
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(PauseSignal, nil)
	}, 0)
	env.RegisterDelayedCallback(func() {
		var qr QueryResult
		res, err := env.QueryWorkflow(QueryType)
		require.NoError(t, err)
		require.NoError(t, res.Get(&qr))
		assert.Equal(t, WorkflowPaused, qr.State)
	}, 100*time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ResumeSignal, nil)
	}, 200*time.Millisecond)

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{
		SourceCluster: "cluster0", TargetCluster: "cluster1",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result FailoverV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.ElementsMatch(t, []string{"d1", "d2"}, result.SuccessDomains)
}
