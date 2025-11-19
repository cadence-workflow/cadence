// Copyright (c) 2017 Uber Technologies, Inc.
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

package persistence

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
)

func TestGetDomainResponse_ToType(t *testing.T) {
	tests := map[string]struct {
		input    *GetDomainResponse
		expected *types.DescribeDomainResponse
	}{
		"nil response": {
			input:    nil,
			expected: nil,
		},
		"empty response": {
			input: &GetDomainResponse{},
			expected: &types.DescribeDomainResponse{
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         0,
					FailoverStartTimestamp:  0,
					FailoverExpireTimestamp: 0,
					CompletedShardCount:     0,
					PendingShards:           nil,
				},
			},
		},
		"full response with all fields": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:          "test-domain-id",
					Name:        "test-domain",
					Status:      0, // 0 = DomainStatusRegistered
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Config: &DomainConfig{
					Retention:                7,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "test://history",
					VisibilityArchivalStatus: types.ArchivalStatusDisabled,
					VisibilityArchivalURI:    "test://visibility",
					BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:          types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "active-cluster",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
			},
			expected: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:        "test-domain-id",
					Name:        "test-domain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Configuration: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "test://history",
					VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
					VisibilityArchivalURI:                  "test://visibility",
					BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:                        &types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "active-cluster",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         100,
					FailoverStartTimestamp:  0,
					FailoverExpireTimestamp: 0,
					CompletedShardCount:     0,
					PendingShards:           nil,
				},
			},
		},
		"with failover info": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0, // 0 = DomainStatusRegistered
				},
				FailoverVersion: 200,
				LastUpdatedTime: 1000,
				FailoverEndTime: func() *int64 { t := int64(2000); return &t }(),
			},
			expected: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				FailoverVersion: 200,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         200,
					FailoverStartTimestamp:  1000,
					FailoverExpireTimestamp: 2000,
				},
			},
		},
		"without failover end time": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0, // 0 = DomainStatusRegistered
				},
				FailoverVersion: 200,
				LastUpdatedTime: 1000,
				FailoverEndTime: nil,
			},
			expected: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				FailoverVersion: 200,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         200,
					FailoverStartTimestamp:  1000,
					FailoverExpireTimestamp: 0,
					CompletedShardCount:     0,
					PendingShards:           nil,
				},
			},
		},
		"nil Info": {
			input: &GetDomainResponse{
				Info:            nil,
				FailoverVersion: 100,
			},
			expected: &types.DescribeDomainResponse{
				DomainInfo:      nil,
				FailoverVersion: 100,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         100,
					FailoverStartTimestamp:  0,
					FailoverExpireTimestamp: 0,
					CompletedShardCount:     0,
					PendingShards:           nil,
				},
			},
		},
		"nil clusters in ReplicationConfig": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0,
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "cluster-1",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						nil, // Preserved in output
						{ClusterName: "cluster-2"},
						nil, // Preserved in output
					},
				},
			},
			expected: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "cluster-1",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "cluster-1"},
						nil,
						{ClusterName: "cluster-2"},
						nil,
					},
				},
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         0,
					FailoverStartTimestamp:  0,
					FailoverExpireTimestamp: 0,
					CompletedShardCount:     0,
					PendingShards:           nil,
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := tc.input.ToType()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFromDescribeDomainResponse(t *testing.T) {
	tests := map[string]struct {
		input    *types.DescribeDomainResponse
		expected *GetDomainResponse
	}{
		"nil response": {
			input:    nil,
			expected: nil,
		},
		"empty response": {
			input:    &types.DescribeDomainResponse{},
			expected: &GetDomainResponse{},
		},
		"full response with all fields": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:        "test-domain-id",
					Name:        "test-domain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Configuration: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "test://history",
					VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
					VisibilityArchivalURI:                  "test://visibility",
					BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:                        &types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "active-cluster",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
			},
			expected: &GetDomainResponse{
				Info: &DomainInfo{
					ID:          "test-domain-id",
					Name:        "test-domain",
					Status:      0, // 0 = DomainStatusRegistered
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Config: &DomainConfig{
					Retention:                7,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "test://history",
					VisibilityArchivalStatus: types.ArchivalStatusDisabled,
					VisibilityArchivalURI:    "test://visibility",
					BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:          types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "active-cluster",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
			},
		},
		"with failover info": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				FailoverVersion: 200,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         200,
					FailoverStartTimestamp:  1000,
					FailoverExpireTimestamp: 2000,
				},
			},
			expected: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0, // 0 = DomainStatusRegistered
				},
				FailoverVersion: 200,
				LastUpdatedTime: 1000,
				// FailoverEndTime not mapped from FailoverInfo
			},
		},
		"nil DomainInfo": {
			input: &types.DescribeDomainResponse{
				DomainInfo:      nil,
				FailoverVersion: 100,
			},
			expected: &GetDomainResponse{
				Info:            nil,
				FailoverVersion: 100,
			},
		},
		"nil Status": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: nil, // Should default to 0
				},
			},
			expected: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0, // Defaults to 0
				},
			},
		},
		"nil clusters in ReplicationConfiguration": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "cluster-1",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "cluster-1"},
						nil, // Preserved in output
						{ClusterName: "cluster-2"},
						nil, // Preserved in output
					},
				},
			},
			expected: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0,
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "cluster-1",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						nil,
						{ClusterName: "cluster-2"},
						nil,
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := FromDescribeDomainResponse(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRoundtrip_ToType_FromDescribeDomainResponse(t *testing.T) {
	tests := map[string]struct {
		input *GetDomainResponse
	}{
		"empty response": {
			input: &GetDomainResponse{},
		},
		"full response": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:          "test-domain-id",
					Name:        "test-domain",
					Status:      0, // 0 = DomainStatusRegistered
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Config: &DomainConfig{
					Retention:                7,
					EmitMetric:               true,
					HistoryArchivalStatus:    types.ArchivalStatusEnabled,
					HistoryArchivalURI:       "test://history",
					VisibilityArchivalStatus: types.ArchivalStatusDisabled,
					VisibilityArchivalURI:    "test://visibility",
					BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:          types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:      types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "active-cluster",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
			},
		},
		"with failover": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0, // 0 = DomainStatusRegistered
				},
				FailoverVersion: 200,
				LastUpdatedTime: 1000,
				FailoverEndTime: func() *int64 { t := int64(2000); return &t }(),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Convert GetDomainResponse -> DescribeDomainResponse -> GetDomainResponse
			typesResp := tc.input.ToType()
			roundtripped := FromDescribeDomainResponse(typesResp)

			// The roundtrip should preserve all the data that's in both representations
			// Note: Some fields may not survive the roundtrip if they only exist in one direction
			assert.Equal(t, tc.input.Info, roundtripped.Info)
			assert.Equal(t, tc.input.Config, roundtripped.Config)
			assert.Equal(t, tc.input.ReplicationConfig, roundtripped.ReplicationConfig)
			assert.Equal(t, tc.input.IsGlobalDomain, roundtripped.IsGlobalDomain)
			assert.Equal(t, tc.input.FailoverVersion, roundtripped.FailoverVersion)
			assert.Equal(t, tc.input.LastUpdatedTime, roundtripped.LastUpdatedTime)
			// FailoverEndTime is not preserved in roundtrip
		})
	}
}

func TestRoundtrip_FromDescribeDomainResponse_ToType(t *testing.T) {
	tests := map[string]struct {
		input *types.DescribeDomainResponse
	}{
		"empty response": {
			input: &types.DescribeDomainResponse{},
		},
		"full response": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:        "test-domain-id",
					Name:        "test-domain",
					Status:      types.DomainStatusRegistered.Ptr(),
					Description: "test description",
					OwnerEmail:  "test@example.com",
					Data:        map[string]string{"key": "value"},
				},
				Configuration: &types.DomainConfiguration{
					WorkflowExecutionRetentionPeriodInDays: 7,
					EmitMetric:                             true,
					HistoryArchivalStatus:                  types.ArchivalStatusEnabled.Ptr(),
					HistoryArchivalURI:                     "test://history",
					VisibilityArchivalStatus:               types.ArchivalStatusDisabled.Ptr(),
					VisibilityArchivalURI:                  "test://visibility",
					BadBinaries:                            &types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{"bad": {Reason: "test"}}},
					IsolationGroups:                        &types.IsolationGroupConfiguration{},
					AsyncWorkflowConfig:                    &types.AsyncWorkflowConfiguration{Enabled: true},
				},
				ReplicationConfiguration: &types.DomainReplicationConfiguration{
					ActiveClusterName: "active-cluster",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
					ActiveClusters: &types.ActiveClusters{
						AttributeScopes: map[string]types.ClusterAttributeScope{
							"region": {
								ClusterAttributes: map[string]types.ActiveClusterInfo{
									"us-east": {ActiveClusterName: "cluster-1", FailoverVersion: 100},
								},
							},
						},
					},
				},
				IsGlobalDomain:  true,
				FailoverVersion: 100,
			},
		},
		"with failover info": {
			input: &types.DescribeDomainResponse{
				DomainInfo: &types.DomainInfo{
					UUID:   "test-id",
					Name:   "test",
					Status: types.DomainStatusRegistered.Ptr(),
				},
				FailoverVersion: 200,
				FailoverInfo: &types.FailoverInfo{
					FailoverVersion:         200,
					FailoverStartTimestamp:  1000,
					FailoverExpireTimestamp: 2000,
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Convert DescribeDomainResponse -> GetDomainResponse -> DescribeDomainResponse
			persistenceResp := FromDescribeDomainResponse(tc.input)
			roundtripped := persistenceResp.ToType()

			// The roundtrip should preserve all the data that's in both representations
			assert.Equal(t, tc.input.DomainInfo, roundtripped.DomainInfo)
			assert.Equal(t, tc.input.Configuration, roundtripped.Configuration)
			assert.Equal(t, tc.input.ReplicationConfiguration, roundtripped.ReplicationConfiguration)
			assert.Equal(t, tc.input.IsGlobalDomain, roundtripped.IsGlobalDomain)
			assert.Equal(t, tc.input.FailoverVersion, roundtripped.FailoverVersion)
			// FailoverInfo is always created in ToType(), so check fields separately
			assert.NotNil(t, roundtripped.FailoverInfo)
			if tc.input.FailoverInfo != nil {
				assert.Equal(t, tc.input.FailoverInfo.FailoverStartTimestamp, roundtripped.FailoverInfo.FailoverStartTimestamp)
				// Other FailoverInfo fields (CompletedShardCount, PendingShards) are not present at persistence layer
			}
		})
	}
}

func TestRoundtrip_FuzzGenerated(t *testing.T) {
	fuzzer := testdatagen.New(t)

	// Run multiple iterations to test with different randomly generated data
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Test GetDomainResponse -> DescribeDomainResponse -> GetDomainResponse
			original := &GetDomainResponse{}
			fuzzer.Fuzz(original)

			typesResp := original.ToType()
			require.NotNil(t, typesResp)

			roundtripped := FromDescribeDomainResponse(typesResp)
			require.NotNil(t, roundtripped)

			// Build expected result accounting for non-symmetric fields
			expected := &GetDomainResponse{
				Info:              original.Info,
				Config:            original.Config,
				ReplicationConfig: original.ReplicationConfig,
				IsGlobalDomain:    original.IsGlobalDomain,
				FailoverVersion:   original.FailoverVersion,
				LastUpdatedTime:   original.LastUpdatedTime,
				// Fields below are not present in types.DescribeDomainResponse and don't roundtrip:
				FailoverNotificationVersion: 0,   // Not mapped
				PreviousFailoverVersion:     0,   // Not mapped
				FailoverEndTime:             nil, // Not mapped
				ConfigVersion:               0,   // Not mapped
				NotificationVersion:         0,   // Not mapped
			}

			// Apply transformations that the mapper does
			if expected.Info != nil {
				// Clamp Status to valid range (0-2) as ToType does
				if expected.Info.Status < 0 || expected.Info.Status > 2 {
					expected.Info.Status = 0
				}
			}

			assert.Equal(t, expected, roundtripped)
		})
	}
}
