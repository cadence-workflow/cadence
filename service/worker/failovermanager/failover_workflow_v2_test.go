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

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

func TestValidateFailoverV2Params_WhenParamsAreInvalidItErrorsAndOtherwiseAppliesDefaults(t *testing.T) {
	assert.Error(t, validateFailoverV2Params(nil))
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{TargetCluster: "t"}))                                // no source
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{SourceClusters: []string{"s"}}))                     // no target
	assert.Error(t, validateFailoverV2Params(&FailoverV2Params{SourceClusters: []string{"x"}, TargetCluster: "x"})) // same

	p := &FailoverV2Params{SourceClusters: []string{"cluster0"}, TargetCluster: "cluster1"}
	require.NoError(t, validateFailoverV2Params(p))
	assert.Equal(t, defaultBatchSizeV2, p.BatchSize)
	assert.Equal(t, defaultWaitBetweenBatchSecondsV2, p.WaitBetweenBatchSeconds)
}

// TestFailoverPreferencesForDomain tests the logic for creating a diff between the current and desired cluster state
// for a failover.
func TestFailoverPreferencesForDomain(t *testing.T) {
	const src, tgt = "cluster0", "cluster1"
	tests := []struct {
		name         string
		domain       *types.DescribeDomainResponse
		filter       []types.ClusterAttribute
		wantOK       bool
		wantPrefs    DomainFailoverPreferences
		wantSnapshot DomainSnapshot
	}{
		{
			name:         "when domain-level active is on source it moves to target",
			domain:       createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: src, isManaged: true, isGlobal: true}),
			wantOK:       true,
			wantPrefs:    DomainFailoverPreferences{DomainName: "d", TargetCluster: tgt},
			wantSnapshot: DomainSnapshot{DomainName: "d", PreviousActiveCluster: src},
		},
		{
			name:   "when domain-level active is on a third cluster it is untouched",
			domain: createDomainResponse(createDomainResponseParams{name: "d", activeClusterName: "cluster2", isManaged: true, isGlobal: true}),
			wantOK: false,
		},
		{
			name: "when an active-active attribute is on source and in the filter it moves to target",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "d",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster0": {ActiveClusterName: src},
							"cluster1": {ActiveClusterName: "cluster1"},
						}},
					}},
				}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}},
			wantOK: true,
			wantPrefs: DomainFailoverPreferences{DomainName: "d", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: tgt},
			}},
			wantSnapshot: DomainSnapshot{DomainName: "d", PreviousClusterAttributes: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: src},
			}},
		},
		{
			name: "when an active-active attribute is on source but the filter is empty it is untouched",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "empty-filter",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster0": {ActiveClusterName: src},
						}},
					}},
				}),
			wantOK: false,
		},
		{
			name: "when an active-active attribute is on source but not in the filter it is untouched",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "unfiltered-attr",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster0": {ActiveClusterName: src},
						}},
					}},
				}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster1"}},
			wantOK: false,
		},
		{
			name: "when several attributes are on source only the filtered one moves",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "selective",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster0": {ActiveClusterName: src},
							"cluster2": {ActiveClusterName: src},
						}},
					}},
				}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}},
			wantOK: true,
			wantPrefs: DomainFailoverPreferences{DomainName: "selective", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: tgt},
			}},
			wantSnapshot: DomainSnapshot{DomainName: "selective", PreviousClusterAttributes: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: src},
			}},
		},
		{
			name: "when no active-active attribute is on source it is untouched",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "untouched-cluster-attributes",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster1": {ActiveClusterName: "cluster1"},
							"cluster2": {ActiveClusterName: "cluster2"},
						}},
					}},
				}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster1"}},
			wantOK: false,
		},
		{
			name: "when a filtered attribute is not on source it is untouched",
			domain: createDomainResponse(
				createDomainResponseParams{
					name:              "filtered-but-off-source",
					activeClusterName: "cluster1",
					isManaged:         true,
					isGlobal:          true,
					data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
					activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
						"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
							"cluster0": {ActiveClusterName: "cluster2"}, // in the filter, but active on cluster2, not source
						}},
					}},
				}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}},
			wantOK: false,
		},
		{
			name: "when both domain-level and a filtered attribute are on source both move to target",
			domain: createDomainResponse(createDomainResponseParams{name: "both-different",
				activeClusterName: src,
				isManaged:         true,
				isGlobal:          true,
				data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"cluster0": {ActiveClusterName: src},
						"cluster1": {ActiveClusterName: "cluster1"},
					}},
				}}}),
			filter: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}},
			wantOK: true,
			wantPrefs: DomainFailoverPreferences{DomainName: "both-different", TargetCluster: tgt, ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: tgt},
			}},
			wantSnapshot: DomainSnapshot{DomainName: "both-different", PreviousActiveCluster: src, PreviousClusterAttributes: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: src},
			}},
		},
		{
			name: "when only domain-level is on source the empty filter leaves attributes untouched",
			domain: createDomainResponse(createDomainResponseParams{name: "domain-only",
				activeClusterName: src,
				isManaged:         true,
				isGlobal:          true,
				data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"cluster0": {ActiveClusterName: src},
					}},
				}}}),
			wantOK:       true,
			wantPrefs:    DomainFailoverPreferences{DomainName: "domain-only", TargetCluster: tgt},
			wantSnapshot: DomainSnapshot{DomainName: "domain-only", PreviousActiveCluster: src},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs, snapshot, ok := failoverPreferencesForDomain(tt.domain, []string{src}, tgt, tt.filter)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			assert.Equal(t, tt.wantPrefs, prefs)
			assert.Equal(t, tt.wantSnapshot, snapshot)
		})
	}
}

func TestGetDomainsForFailoverV2Activity_WhenListingDomainsItReturnsOnlyManagedDomainsActiveOnSource(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			createDomainResponse(createDomainResponseParams{name: "managed-on-source", activeClusterName: "cluster0", isManaged: true, isGlobal: true}),
			createDomainResponse(createDomainResponseParams{name: "managed-on-third", activeClusterName: "cluster2", isManaged: true, isGlobal: true}),
			createDomainResponse(createDomainResponseParams{name: "unmanaged", activeClusterName: "cluster0", isManaged: false, isGlobal: true}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)
	expectPollersPresent(mockResource)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceClusters: []string{"cluster0"},
		TargetCluster:  "cluster1",
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	require.Len(t, result.Preferences, 1)
	assert.Equal(t, "managed-on-source", result.Preferences[0].DomainName)
	assert.Equal(t, "cluster1", result.Preferences[0].TargetCluster)
	require.Len(t, result.Snapshots, 1)
	assert.Equal(t, "cluster0", result.Snapshots[0].PreviousActiveCluster)
}

func TestGetDomainsForFailoverV2Activity_WhenClusterAttributesAreSetItFailsOverMatchingAttributes(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			createDomainResponse(createDomainResponseParams{
				name:              "aa-on-source",
				activeClusterName: "cluster1", // domain-level not on source, only the attribute should move
				isManaged:         true,
				isGlobal:          true,
				data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"cluster0": {ActiveClusterName: "cluster0"},
					}},
				}},
			}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)
	expectPollersPresent(mockResource)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceClusters:    []string{"cluster0"},
		TargetCluster:     "cluster1",
		ClusterAttributes: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}},
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	require.Len(t, result.Preferences, 1)
	assert.Equal(t, "aa-on-source", result.Preferences[0].DomainName)
	assert.Empty(t, result.Preferences[0].TargetCluster)
	assert.Equal(t, []ClusterAttributePreference{
		{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
	}, result.Preferences[0].ClusterAttributeUpdates)
}

func TestGetDomainsForFailoverV2Activity_WhenClusterAttributesAreEmptyItSkipsAttributeOnlyDomains(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			createDomainResponse(createDomainResponseParams{
				name:              "aa-on-source",
				activeClusterName: "cluster1", // domain-level not on source; with no filter there is nothing to move
				isManaged:         true,
				isGlobal:          true,
				data:              map[string]string{constants.DomainDataKeyForClusterAttributePreferences: aaPrefsJSON},
				activeClusters: &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
					"cluster": {ClusterAttributes: map[string]types.ActiveClusterInfo{
						"cluster0": {ActiveClusterName: "cluster0"},
					}},
				}},
			}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceClusters: []string{"cluster0"},
		TargetCluster:  "cluster1",
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	assert.Empty(t, result.Preferences)
}

// TestGetDomainsForFailoverV2Activity_WhenGivenMixedDomainsItFailsOverOnlyScopedOnSource mirrors
// the failover phase of scripts/test_failover_rebalance.sh: a single ListDomains containing one
// domain per scenario, asserting that exactly the right domains and changes are collected.
func TestGetDomainsForFailoverV2Activity_WhenGivenMixedDomainsItFailsOverOnlyScopedOnSource(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	attrs := func(scope, name, cluster string) *types.ActiveClusters {
		return &types.ActiveClusters{AttributeScopes: map[string]types.ClusterAttributeScope{
			scope: {ClusterAttributes: map[string]types.ActiveClusterInfo{
				name: {ActiveClusterName: cluster},
			}},
		}}
	}

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			// INCLUDED attribute on source -> only the attribute moves (domain-level off source).
			createDomainResponse(createDomainResponseParams{
				name: "aa-included", activeClusterName: "cluster1", isManaged: true, isGlobal: true,
				activeClusters: attrs("cluster", "cluster0", "cluster0"),
			}),
			// EXCLUDED attribute on source -> skipped (not in the filter).
			createDomainResponse(createDomainResponseParams{
				name: "aa-excluded", activeClusterName: "cluster1", isManaged: true, isGlobal: true,
				activeClusters: attrs("cluster", "cluster1", "cluster0"),
			}),
			// INCLUDED attribute but not on source -> skipped.
			createDomainResponse(createDomainResponseParams{
				name: "aa-included-offsrc", activeClusterName: "cluster1", isManaged: true, isGlobal: true,
				activeClusters: attrs("cluster", "cluster2", "cluster1"),
			}),
			// Standard global domain active on source -> domain-level moves.
			createDomainResponse(createDomainResponseParams{
				name: "global-onsrc", activeClusterName: "cluster0", isManaged: true, isGlobal: true,
			}),
			// Unmanaged domain with an in-filter attribute on source -> skipped (not eligible).
			createDomainResponse(createDomainResponseParams{
				name: "aa-unmanaged", activeClusterName: "cluster0", isManaged: false, isGlobal: true,
				activeClusters: attrs("cluster", "cluster0", "cluster0"),
			}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)
	expectPollersPresent(mockResource)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceClusters:    []string{"cluster0"},
		TargetCluster:     "cluster1",
		ClusterAttributes: []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}, {Scope: "cluster", Name: "cluster2"}},
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	byName := make(map[string]DomainFailoverPreferences, len(result.Preferences))
	for _, p := range result.Preferences {
		byName[p.DomainName] = p
	}

	// Only aa-included and global-onsrc are collected.
	require.Len(t, result.Preferences, 2)
	require.Contains(t, byName, "aa-included")
	require.Contains(t, byName, "global-onsrc")
	assert.NotContains(t, byName, "aa-excluded")
	assert.NotContains(t, byName, "aa-included-offsrc")
	assert.NotContains(t, byName, "aa-unmanaged")

	// aa-included: only the in-filter, on-source attribute moves; domain-level untouched.
	assert.Empty(t, byName["aa-included"].TargetCluster)
	assert.Equal(t, []ClusterAttributePreference{
		{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
	}, byName["aa-included"].ClusterAttributeUpdates)

	// global-onsrc: domain-level moves; no attribute updates.
	assert.Equal(t, "cluster1", byName["global-onsrc"].TargetCluster)
	assert.Empty(t, byName["global-onsrc"].ClusterAttributeUpdates)
}

// TestGetDomainsForFailoverV2Activity_WhenDestinationHasNoPollersItStillIncludesTheDomain verifies
// the V2 poller check is advisory: a destination cluster with zero pollers produces a warning but the
// domain is still collected for failover (unlike V1, which skips it).
func TestGetDomainsForFailoverV2Activity_WhenDestinationHasNoPollersItStillIncludesTheDomain(t *testing.T) {
	env, mockResource := newFailoverV2ActivityEnv(t)

	domains := &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			createDomainResponse(createDomainResponseParams{name: "managed-on-source", activeClusterName: "cluster0", isManaged: true, isGlobal: true}),
		},
	}
	mockResource.FrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(domains, nil)
	expectDestinationMissingPollers(mockResource)

	val, err := env.ExecuteActivity(GetDomainsForFailoverV2Activity, &GetDomainsForFailoverV2Params{
		SourceClusters: []string{"cluster0"},
		TargetCluster:  "cluster1",
	})
	require.NoError(t, err)
	var result GetDomainsForFailoverV2Result
	require.NoError(t, val.Get(&result))

	require.Len(t, result.Preferences, 1)
	assert.Equal(t, "managed-on-source", result.Preferences[0].DomainName)
	assert.Equal(t, "cluster1", result.Preferences[0].TargetCluster)
}

func TestFailoverWorkflowV2_WhenParamsAreInvalidItFailsTheWorkflow(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{})
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestFailoverWorkflowV2_WhenAllActivitiesSucceedItReturnsSuccessDomainsAndSnapshots(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})

	collected := &GetDomainsForFailoverV2Result{
		Preferences: []DomainFailoverPreferences{{DomainName: "d1", TargetCluster: "cluster1"}},
		Snapshots:   []DomainSnapshot{{DomainName: "d1", PreviousActiveCluster: "cluster0"}},
	}
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(collected, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}}}, nil)

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{
		SourceClusters: []string{"cluster0"}, TargetCluster: "cluster1",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result FailoverV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, []string{"d1"}, successDomainNames(result.SuccessDomains))
	assert.Equal(t, collected.Snapshots, result.Snapshots)
}

func TestFailoverWorkflowV2_WhenClusterAttributesAreSetItForwardsThemToGetDomainsActivity(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})

	clusterAttributes := []types.ClusterAttribute{{Scope: "cluster", Name: "cluster0"}}
	var gotParams GetDomainsForFailoverV2Params
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			gotParams = *args.Get(1).(*GetDomainsForFailoverV2Params)
		}).
		Return(&GetDomainsForFailoverV2Result{
			Preferences: []DomainFailoverPreferences{{DomainName: "d1", ClusterAttributeUpdates: []ClusterAttributePreference{
				{Scope: "cluster", Name: "cluster0", PreferredCluster: "cluster1"},
			}}},
		}, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}}}, nil)

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{
		SourceClusters: []string{"cluster0"}, TargetCluster: "cluster1", ClusterAttributes: clusterAttributes,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, clusterAttributes, gotParams.ClusterAttributes)
}

func TestFailoverWorkflowV2_WhenGetDomainsActivityErrorsItFailsTheWorkflow(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	env.ExecuteWorkflow(FailoverWorkflowV2TypeName, &FailoverV2Params{SourceClusters: []string{"cluster0"}, TargetCluster: "cluster1"})
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestFailoverWorkflowV2_WhenPausedItBlocksUntilResumedThenCompletes(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(FailoverWorkflowV2, workflow.RegisterOptions{Name: FailoverWorkflowV2TypeName})
	env.RegisterActivityWithOptions(FailoverActivityV2, activity.RegisterOptions{Name: failoverActivityV2Name})
	env.RegisterActivityWithOptions(GetDomainsForFailoverV2Activity, activity.RegisterOptions{Name: getDomainsForFailoverV2ActivityName})

	collected := &GetDomainsForFailoverV2Result{
		Preferences: []DomainFailoverPreferences{
			{DomainName: "d1", TargetCluster: "cluster1"},
			{DomainName: "d2", TargetCluster: "cluster1"},
		},
	}
	env.OnActivity(getDomainsForFailoverV2ActivityName, mock.Anything, mock.Anything).Return(collected, nil)
	env.OnActivity(failoverActivityV2Name, mock.Anything, mock.Anything).
		Return(&FailoverActivityV2Result{SuccessDomains: []DomainFailoverSuccess{{DomainName: "d1"}, {DomainName: "d2"}}}, nil).Once()

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
		SourceClusters: []string{"cluster0"}, TargetCluster: "cluster1",
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result FailoverV2Result
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.ElementsMatch(t, []string{"d1", "d2"}, successDomainNames(result.SuccessDomains))
}
