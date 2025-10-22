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

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	numShards = 10
)

func TestGetActiveClusterInfoByClusterAttribute(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()

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
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   20,
							},
						},
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

			mgr, err := NewManager(
				domainIDToDomainFn,
				metricsCl,
				logger,
				nil,
				numShards,
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

	tests := []struct {
		name                           string
		runID                          string
		activeClusterCfg               *types.ActiveClusters
		domainIDToNameErr              error
		mockExecutionManagerFn         func(em *persistence.MockExecutionManager)
		mockExecutionManagerProviderFn func(emp *MockExecutionManagerProvider)
		cachedPolicy                   *types.ActiveClusterSelectionPolicy
		expectedResult                 *types.ActiveClusterInfo
		expectedError                  string
	}{
		{
			name:  "domain ID to name function returns error",
			runID: "test-run-id",
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
			name:  "execution manager provider returns error",
			runID: "test-run-id",
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
			name:  "execution manager GetActiveClusterSelectionPolicy returns error",
			runID: "test-run-id",
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
			name:  "policy not found (EntityNotExistsError) - uses empty policy with nil cluster attribute",
			runID: "test-run-id",
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
			name:  "policy not found (EntityNotExistsError) - empty policy with cluster attribute not found",
			runID: "test-run-id",
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
			name:  "policy found but cluster attribute not found in domain config",
			runID: "test-run-id",
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
			name:  "successful lookup - policy found and cluster attribute exists",
			runID: "test-run-id",
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
			name:  "successful lookup - policy from cache",
			runID: "test-run-id",
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
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				// No expectations needed since policy comes from cache, but we need the provider to be non-nil
			},
			expectedResult: &types.ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   250,
			},
		},
		{
			name:  "successful lookup - nil cluster attribute in policy uses domain-level info",
			runID: "test-run-id",
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
			name:  "successful lookup - multiple scopes with different attributes",
			runID: "test-run-id",
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
		{
			name:  "successful lookup - empty runID triggers GetCurrentExecution",
			runID: "",
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
				// Expect GetCurrentExecution to be called first to get the runID
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "current-run-id",
				}, nil)
				// Then expect GetActiveClusterSelectionPolicy with the returned runID
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "current-run-id").
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
			name:  "empty runID - GetCurrentExecution returns error",
			runID: "",
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
				// GetCurrentExecution returns an error
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(nil, errors.New("workflow not found"))
			},
			expectedError: "workflow not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntryWithAttributeScopes(tc.activeClusterCfg), tc.domainIDToNameErr
			}

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
				metricsCl,
				logger,
				emProvider,
				numShards,
			)
			assert.NoError(t, err)

			if tc.cachedPolicy != nil {
				key := fmt.Sprintf("%s:%s:%s", "test-domain-id", wfID, tc.runID)
				mgr.(*managerImpl).workflowPolicyCache.Put(key, tc.cachedPolicy)
			}

			result, err := mgr.GetActiveClusterInfoByWorkflow(context.Background(), "test-domain-id", wfID, tc.runID)
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

func TestGetActiveClusterSelectionPolicyForCurrentWorkflow(t *testing.T) {
	metricsCl := metrics.NewNoopMetricsClient()
	logger := log.NewNoop()

	tests := []struct {
		name                           string
		activeClusterCfg               *types.ActiveClusters
		isActiveActive                 bool
		domainIDToNameErr              error
		mockExecutionManagerFn         func(em *persistence.MockExecutionManager)
		mockExecutionManagerProviderFn func(emp *MockExecutionManagerProvider)
		cachedPolicy                   *types.ActiveClusterSelectionPolicy
		expectedPolicy                 *types.ActiveClusterSelectionPolicy
		expectedRunning                bool
		expectedError                  string
	}{
		{
			name:              "domain ID to name function returns error",
			isActiveActive:    true,
			domainIDToNameErr: errors.New("failed to find domain by id"),
			expectedError:     "failed to find domain by id",
			expectedRunning:   false,
		},
		{
			name:            "non-active-active domain returns nil",
			isActiveActive:  false,
			expectedPolicy:  nil,
			expectedRunning: false,
		},
		{
			name:           "execution manager provider returns error",
			isActiveActive: true,
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
			expectedError:   "failed to get execution manager",
			expectedRunning: false,
		},
		{
			name:           "GetCurrentExecution returns error",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(nil, errors.New("workflow not found"))
			},
			expectedError:   "workflow not found",
			expectedRunning: false,
		},
		{
			name:           "workflow completed - returns nil, false",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateCompleted,
				}, nil)
			},
			expectedPolicy:  nil,
			expectedRunning: false,
		},
		{
			name:           "workflow running - successfully returns policy",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateRunning,
				}, nil)
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-east",
						},
					}, nil)
			},
			expectedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: "region",
					Name:  "us-east",
				},
			},
			expectedRunning: true,
		},
		{
			name:           "workflow created state - successfully returns policy",
			isActiveActive: true,
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
			mockExecutionManagerFn: func(em *persistence.MockExecutionManager) {
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateCreated,
				}, nil)
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "datacenter",
							Name:  "dc1",
						},
					}, nil)
			},
			expectedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: "datacenter",
					Name:  "dc1",
				},
			},
			expectedRunning: true,
		},
		{
			name:           "workflow running but GetActiveClusterSelectionPolicy fails",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateRunning,
				}, nil)
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(nil, errors.New("database error"))
			},
			expectedError:   "database error",
			expectedRunning: false,
		},
		{
			name:           "workflow running - policy from cache",
			isActiveActive: true,
			activeClusterCfg: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateRunning,
				}, nil)
				// No expectation for GetActiveClusterSelectionPolicy since it comes from cache
			},
			cachedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: "city",
					Name:  "seattle",
				},
			},
			expectedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: &types.ClusterAttribute{
					Scope: "city",
					Name:  "seattle",
				},
			},
			expectedRunning: true,
		},
		{
			name:           "workflow running with EntityNotExistsError returns error",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateRunning,
				}, nil)
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(nil, &types.EntityNotExistsError{Message: "policy not found"})
			},
			expectedError:   "policy not found",
			expectedRunning: false,
		},
		{
			name:           "workflow running with nil cluster attribute in policy",
			isActiveActive: true,
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
				em.EXPECT().GetCurrentExecution(gomock.Any(), &persistence.GetCurrentExecutionRequest{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
				}).Return(&persistence.GetCurrentExecutionResponse{
					RunID: "test-run-id",
					State: persistence.WorkflowStateRunning,
				}, nil)
				em.EXPECT().GetActiveClusterSelectionPolicy(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").
					Return(&types.ActiveClusterSelectionPolicy{
						ClusterAttribute: nil,
					}, nil)
			},
			expectedPolicy: &types.ActiveClusterSelectionPolicy{
				ClusterAttribute: nil,
			},
			expectedRunning: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domainIDToDomainFn := func(id string) (*cache.DomainCacheEntry, error) {
				return getDomainCacheEntry(tc.activeClusterCfg, tc.isActiveActive), tc.domainIDToNameErr
			}

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
				metricsCl,
				logger,
				emProvider,
				numShards,
			)
			assert.NoError(t, err)

			if tc.cachedPolicy != nil {
				// Need to get the runID from the test case to build the cache key
				// For cached policy tests, we assume the runID is "test-run-id"
				key := fmt.Sprintf("%s:%s:%s", "test-domain-id", wfID, "test-run-id")
				mgr.(*managerImpl).workflowPolicyCache.Put(key, tc.cachedPolicy)
			}

			policy, running, err := mgr.GetActiveClusterSelectionPolicyForCurrentWorkflow(context.Background(), "test-domain-id", wfID)
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				assert.Nil(t, policy)
				assert.Equal(t, tc.expectedRunning, running)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPolicy, policy)
				assert.Equal(t, tc.expectedRunning, running)
			}
		})
	}
}
