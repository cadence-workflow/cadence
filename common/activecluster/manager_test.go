// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package activecluster

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	numShards = 10
)

func TestLookupNewWorkflow(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"cluster0": {
					InitialFailoverVersion: 1,
					Region:                 "us-west",
				},
				"cluster1": {
					InitialFailoverVersion: 3,
					Region:                 "us-east",
				},
			},
			FailoverVersionIncrement: 100,
			CurrentClusterName:       "cluster0",
		},
		func(d string) bool { return false },
		metricsCl,
		logger,
	)

	tests := []struct {
		name             string
		policy           *types.ActiveClusterSelectionPolicy
		activeClusterCfg *types.ActiveClusters
		expectedResult   *LookupResult
		expectedError    string
	}{
		{
			name:             "not active-active domain, returns failover version of the domain",
			activeClusterCfg: nil, // not active-active domain
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 201,
			},
		},
		{
			name:   "active-active domain, policy is nil. returns failover version of the active cluster in current region",
			policy: nil,
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   20,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   22,
					},
				},
			},
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 20, // failover version of cluster0 in RegionToClusterMap
			},
		},
		{
			name: "active-active domain, policy is region sticky but region is missing in domain's active cluster config",
			policy: &types.ActiveClusterSelectionPolicy{
				ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
				StickyRegion:                   "us-west",
			},
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					// missing "us-west" here
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   22,
					},
				},
			},
			expectedError: "could not find region us-west in the domain test-domain-id's active cluster config",
		},
		{
			name: "active-active domain, policy is region sticky. returns failover version of the active cluster in sticky region",
			policy: &types.ActiveClusterSelectionPolicy{
				ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
				StickyRegion:                   "us-west",
			},
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   20,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   22,
					},
				},
			},
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 20, // failover version of cluster0 in RegionToClusterMap
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntry(tc.activeClusterCfg, false), nil
			}

			timeSrc := clock.NewMockedTimeSource()
			mgr, err := NewManager(
				domainIDToDomainFn,
				clusterMetadata,
				metricsCl,
				logger,
				nil,
				numShards,
				WithTimeSource(timeSrc),
			)
			assert.NoError(t, err)

			result, err := mgr.LookupNewWorkflow(context.Background(), "test-domain-id", tc.policy)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if diff := cmp.Diff(tc.expectedResult, result); diff != "" {
				t.Fatalf("expected result mismatch: %v", diff)
			}
		})
	}
}

func TestLookupWorkflow(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"cluster0": {
					InitialFailoverVersion: 1,
					Region:                 "us-west",
				},
				"cluster1": {
					InitialFailoverVersion: 3,
					Region:                 "us-east",
				},
			},
			FailoverVersionIncrement: 100,
			CurrentClusterName:       "cluster0",
		},
		func(d string) bool { return false },
		metricsCl,
		logger,
	)

	tests := []struct {
		name                        string
		getClusterSelectionPolicyFn func(ctx context.Context, domainID, wfID, rID string) (*types.ActiveClusterSelectionPolicy, error)
		mockFn                      func(em *persistence.MockExecutionManager)
		activeClusterCfg            *types.ActiveClusters
		cachedPolicy                *types.ActiveClusterSelectionPolicy
		domainIDToNameErr           error
		migratedFromActivePassive   bool
		expectedResult              *LookupResult
		expectedError               string
	}{
		{
			name:             "domain is not active-active",
			activeClusterCfg: nil,
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 201,
			},
		},
		{
			name: "domain id to name fn returns error",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			domainIDToNameErr: errors.New("failed to find domain by id"),
			expectedError:     "failed to find domain by id",
		},
		{
			name: "domain is active-active, failed to fetch active cluster policy",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("failed to fetch policy"))
			},
			expectedError: "failed to fetch policy",
		},
		{
			name:                      "domain is migrated from active-passive to active-active, activeness metadata not-found. falls back to domain's active cluster name and failover version",
			migratedFromActivePassive: true,
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 201,
			},
		},
		{
			name: "domain is active-active and NOT migrated from active-passive, policy not-found. return cluster name and failover version of current region",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   101,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.EntityNotExistsError{})
			},
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 101,
				Region:          "us-west",
			},
		},
		{
			name: "domain is active-active, policy is nil means region sticky",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
			},
			expectedResult: &LookupResult{
				ClusterName:     "cluster0",
				FailoverVersion: 1,
				Region:          "us-west",
			},
		},
		{
			name: "domain is active-active, policy shows region sticky",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterSelectionPolicy{
						ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
						StickyRegion:                   "us-east",
					}, nil)
			},
			expectedResult: &LookupResult{
				Region:          "us-east",
				ClusterName:     "cluster1",
				FailoverVersion: 3,
			},
		},
		{
			name: "domain is active-active, policy returned from cache",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			cachedPolicy: &types.ActiveClusterSelectionPolicy{
				ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
				StickyRegion:                   "us-east",
			},
			expectedResult: &LookupResult{
				Region:          "us-east",
				ClusterName:     "cluster1",
				FailoverVersion: 3,
			},
		},
		{
			name: "domain is active-active, policy shows region sticky. domain is failed over to cluster1",
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					// both regions have cluster1 as active cluster
					"us-west": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
					"us-east": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   3,
					},
				},
			},
			mockFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterSelectionPolicy{
						ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
						StickyRegion:                   "us-west",
					}, nil)
			},
			expectedResult: &LookupResult{
				Region:          "us-west",
				ClusterName:     "cluster1",
				FailoverVersion: 3,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntry(tc.activeClusterCfg, tc.migratedFromActivePassive), tc.domainIDToNameErr
			}

			timeSrc := clock.NewMockedTimeSource()
			ctrl := gomock.NewController(t)

			wfID := "test-workflow-id"
			shardID := 6 // corresponds to wfID given numShards
			em := persistence.NewMockExecutionManager(ctrl)
			if tc.mockFn != nil {
				tc.mockFn(em)
			}
			emProvider := NewMockExecutionManagerProvider(ctrl)
			emProvider.EXPECT().GetExecutionManager(shardID).Return(em, nil).AnyTimes()

			mgr, err := NewManager(
				domainIDToDomainFn,
				clusterMetadata,
				metricsCl,
				logger,
				emProvider,
				numShards,
				WithTimeSource(timeSrc),
			)
			assert.NoError(t, err)

			if tc.cachedPolicy != nil {
				key := fmt.Sprintf("%s:%s:%s", "test-domain-id", wfID, "test-run-id")
				mgr.(*managerImpl).workflowPolicyCache.Put(key, tc.cachedPolicy)
			}

			result, err := mgr.LookupWorkflow(context.Background(), "test-domain-id", wfID, "test-run-id")
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectedResult != nil {
				if result == nil {
					t.Fatalf("expected result not nil, got nil")
				}
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestGetActiveClusterInfoByClusterAttribute(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"cluster0": {
					InitialFailoverVersion: 1,
					Region:                 "us-west",
				},
				"cluster1": {
					InitialFailoverVersion: 3,
					Region:                 "us-east",
				},
			},
			FailoverVersionIncrement: 100,
			CurrentClusterName:       "cluster0",
		},
		func(d string) bool { return false },
		metricsCl,
		logger,
	)

	tests := []struct {
		name              string
		clusterAttribute  *types.ClusterAttribute
		activeClusterCfg  *types.ActiveClusters
		domainIDToNameErr error
		expectedResult    *types.ActiveClusterInfo
		expectedError     string
	}{
		{
			name:             "nil cluster attribute - returns domain-level active cluster info",
			clusterAttribute: nil,
			activeClusterCfg: &types.ActiveClusters{
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   20,
					},
				},
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0",
				FailoverVersion:   201, // domain failover version
			},
		},
		{
			name: "domain ID to name function returns error",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			domainIDToNameErr: errors.New("failed to find domain by id"),
			expectedError:     "failed to find domain by id",
		},
		{
			name: "nil active clusters - returns ClusterAttributeNotFoundError",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			activeClusterCfg: nil,
			expectedError:    "could not find cluster attribute &{region us-west} in the domain test-domain-id's active cluster config",
		},
		{
			name: "nil attribute scopes - returns ClusterAttributeNotFoundError",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: nil,
			},
			expectedError: "could not find cluster attribute &{region us-west} in the domain test-domain-id's active cluster config",
		},
		{
			name: "scope not found - returns ClusterAttributeNotFoundError",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "datacenter",
				Name:  "dc1",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			expectedError: "could not find cluster attribute &{datacenter dc1} in the domain test-domain-id's active cluster config",
		},
		{
			name: "attribute name not found in scope - returns ClusterAttributeNotFoundError",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-central",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
							"us-east": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			expectedError: "could not find cluster attribute &{region us-central} in the domain test-domain-id's active cluster config",
		},
		{
			name: "successful lookup - returns active cluster info for region attribute",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "region",
				Name:  "us-west",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
							"us-east": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0",
				FailoverVersion:   100,
			},
		},
		{
			name: "successful lookup - returns active cluster info for custom scope",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "datacenter",
				Name:  "dc1",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"datacenter": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"dc1": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   150,
							},
							"dc2": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   250,
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0",
				FailoverVersion:   150,
			},
		},
		{
			name: "successful lookup - multiple scopes with different attributes",
			clusterAttribute: &types.ClusterAttribute{
				Scope: "city",
				Name:  "seattle",
			},
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
					"city": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"seattle": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   300,
							},
							"san_francisco": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   350,
							},
						},
					},
				},
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   300,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntryWithAttributeScopes(tc.activeClusterCfg), tc.domainIDToNameErr
			}

			timeSrc := clock.NewMockedTimeSource()
			mgr, err := NewManager(
				domainIDToDomainFn,
				clusterMetadata,
				metricsCl,
				logger,
				nil,
				numShards,
				WithTimeSource(timeSrc),
			)
			assert.NoError(t, err)

			result, err := mgr.GetActiveClusterInfoByClusterAttribute(context.Background(), "test-domain-id", tc.clusterAttribute)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func getDomainCacheEntry(cfg *types.ActiveClusters, migratedFromActivePassive bool) *cache.DomainCacheEntry {
	// only thing we care in domain cache entry is the active clusters config
	// for domains migrated from active-passive to active-active, we set the failover version to 201
	activeClusterName := ""
	if migratedFromActivePassive || cfg == nil {
		activeClusterName = "cluster0"
	}
	return cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{
			Name: "test-domain-id",
		},
		nil,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusters:    cfg,
			ActiveClusterName: activeClusterName,
		},
		201,
		nil,
		1,
		1,
		1,
	)
}

func TestGetActiveClusterInfoByWorkflow(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"cluster0": {
					InitialFailoverVersion: 1,
					Region:                 "us-west",
				},
				"cluster1": {
					InitialFailoverVersion: 3,
					Region:                 "us-east",
				},
			},
			FailoverVersionIncrement: 100,
			CurrentClusterName:       "cluster0",
		},
		func(d string) bool { return false },
		metricsCl,
		logger,
	)

	tests := []struct {
		name                           string
		activeClusterCfg               *types.ActiveClusters
		domainIDToNameErr              error
		mockExecutionManagerFn         func(em *persistence.MockExecutionManager)
		mockExecutionManagerProviderFn func(emp *MockExecutionManagerProvider)
		cachedPolicy                   *types.ActiveClusterSelectionPolicy
		expectedResult                 *types.ActiveClusterInfo
		expectedError                  string
	}{
		{
			name: "domain ID to name function returns error",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			domainIDToNameErr: errors.New("failed to find domain by id"),
			expectedError:     "failed to find domain by id",
		},
		{
			name: "execution manager provider returns error",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerProviderFn: func(emp *MockExecutionManagerProvider) {
				emp.EXPECT().GetExecutionManager(6).Return(nil, errors.New("failed to get execution manager")).AnyTimes()
			},
			expectedError: "failed to get execution manager",
		},
		{
			name: "execution manager GetActiveClusterSelectionPolicy returns error",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(nil, errors.New("database error"))
			},
			expectedError: "database error",
		},
		{
			name: "policy not found (EntityNotExistsError) - uses empty policy with nil cluster attribute",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(nil, &types.EntityNotExistsError{Message: "policy not found"})
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0", // domain-level active cluster
				FailoverVersion:   201,        // domain failover version
			},
		},
		{
			name: "policy not found (EntityNotExistsError) - empty policy with cluster attribute not found",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(nil, &types.EntityNotExistsError{Message: "policy not found"})
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0", // domain-level active cluster
				FailoverVersion:   201,        // domain failover version
			},
		},
		{
			name: "policy found but cluster attribute not found in domain config",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "datacenter",
							Name:  "dc1",
						},
					}, nil)
			},
			expectedError: "could not find cluster attribute &{datacenter dc1} in the domain test-domain-id's active cluster config",
		},
		{
			name: "successful lookup - policy found and cluster attribute exists",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
							"us-east": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-east",
						},
					}, nil)
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   200,
			},
		},
		{
			name: "successful lookup - policy from cache",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"datacenter": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"dc1": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   150,
							},
							"dc2": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   250,
							},
						},
					},
				},
			},
			cachedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: "datacenter",
					Name:  "dc2",
				},
			},
			// No mock needed since policy comes from cache
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   250,
			},
		},
		{
			name: "successful lookup - nil cluster attribute in policy uses domain-level info",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: nil, // nil cluster attribute
					}, nil)
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster0", // domain-level active cluster
				FailoverVersion:   201,        // domain failover version
			},
		},
		{
			name: "successful lookup - multiple scopes with different attributes",
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   100,
							},
						},
					},
					"city": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"seattle": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   300,
							},
							"san_francisco": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   350,
							},
						},
					},
				},
			},
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "city",
							Name:  "seattle",
						},
					}, nil)
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   300,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntryWithAttributeScopes(tc.activeClusterCfg), tc.domainIDToNameErr
			}

			timeSrc := clock.NewMockedTimeSource()
			ctrl := gomock.NewController(t)

			wfID := "test-workflow-id"
			shardID := 6 // corresponds to wfID given numShards

			var emProvider *MockExecutionManagerProvider
			if tc.mockExecutionManagerProviderFn != nil || tc.mockExecutionManagerFn != nil {
				emProvider = NewMockExecutionManagerProvider(ctrl)

				if tc.mockExecutionManagerProviderFn != nil {
					tc.mockExecutionManagerProviderFn(emProvider)
				} else {
					// Default case: create execution manager and set up provider
					em := persistence.NewMockExecutionManager(ctrl)
					if tc.mockExecutionManagerFn != nil {
						tc.mockExecutionManagerFn(em)
					}
					emProvider.EXPECT().GetExecutionManager(shardID).Return(em, nil).AnyTimes()
				}
			}

			mgr, err := NewManager(
				domainIDToDomainFn,
				clusterMetadata,
				metricsCl,
				logger,
				emProvider,
				numShards,
				WithTimeSource(timeSrc),
			)
			assert.NoError(t, err)

			if tc.cachedPolicy != nil {
				key := fmt.Sprintf("%s:%s:%s", "test-domain-id", wfID, "test-run-id")
				mgr.(*managerImpl).workflowPolicyCache.Put(key, tc.cachedPolicy)
			}

			result, err := mgr.GetActiveClusterInfoByWorkflow(context.Background(), "test-domain-id", wfID, "test-run-id")
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func getDomainCacheEntryWithAttributeScopes(cfg *types.ActiveClusters) *cache.DomainCacheEntry {
	// Helper function specifically for GetActiveClusterInfoByClusterAttribute tests
	// Always sets activeClusterName to "cluster0" for consistency
	return cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{
			Name: "test-domain-id",
		},
		nil,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusters:    cfg,
			ActiveClusterName: "cluster0",
		},
		201, // domain failover version
		nil,
		1,
		1,
		1,
	)
}
