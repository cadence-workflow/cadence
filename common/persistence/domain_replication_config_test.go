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

package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

// TestIsActiveActiveMethodsAreInSync ensures that the IsActiveActive() implementations
// in both persistence.DomainReplicationConfig and types.DomainReplicationConfiguration
// behave identically
func TestIsActiveActiveMethodsAreInSync(t *testing.T) {
	tests := []struct {
		name              string
		typesConfig       *types.DomainReplicationConfiguration
		persistenceConfig *DomainReplicationConfig
		want              bool
	}{
		{
			name:              "both nil should return false",
			typesConfig:       nil,
			persistenceConfig: nil,
			want:              false,
		},
		{
			name:              "both with nil ActiveClusters should return false",
			typesConfig:       &types.DomainReplicationConfiguration{ActiveClusters: nil},
			persistenceConfig: &DomainReplicationConfig{ActiveClusters: nil},
			want:              false,
		},
		{
			name:              "both with empty ActiveClusters should return false",
			typesConfig:       &types.DomainReplicationConfiguration{ActiveClusters: &types.ActiveClusters{}},
			persistenceConfig: &DomainReplicationConfig{ActiveClusters: &types.ActiveClusters{}},
			want:              false,
		},
		{
			name: "both with only AttributeScopes populated should return true",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
							},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "both with only AttributeScopes with non-empty ClusterAttributes should return true",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
							},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "both with empty AttributeScopes map should return false",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{},
				},
			},
			want: false,
		},
		{
			name: "both with AttributeScopes containing scope with empty ClusterAttributes should return false",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "both with multiple AttributeScopes, some empty should return true if any has ClusterAttributes",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
							},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "both with multiple AttributeScopes all empty should return false",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "both with multiple regions in AttributeScopes should return true",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
								"eu-west-1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
							},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
								"eu-west-1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "both with multiple AttributeScopes, each with multiple ClusterAttributes should return true",
			typesConfig: &types.DomainReplicationConfiguration{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
							},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
								"dc2": {
									ActiveClusterName: "cluster4",
									FailoverVersion:   400,
								},
							},
						},
					},
				},
			},
			persistenceConfig: &DomainReplicationConfig{
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-east-1": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   100,
								},
								"us-west-1": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   200,
								},
							},
						},
						"datacenter": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"dc1": {
									ActiveClusterName: "cluster3",
									FailoverVersion:   300,
								},
								"dc2": {
									ActiveClusterName: "cluster4",
									FailoverVersion:   400,
								},
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typesResult := tt.typesConfig.IsActiveActive()
			persistenceResult := tt.persistenceConfig.IsActiveActive()

			if tt.persistenceConfig != nil && tt.typesConfig != nil {
				assert.Equal(t, tt.persistenceConfig.ActiveClusters, tt.typesConfig.ActiveClusters)
			}

			// Assert that both implementations return the same value
			assert.Equal(t, persistenceResult, typesResult,
				"IsActiveActive() implementations differ: types=%v, persistence=%v",
				typesResult, persistenceResult)

			// Also assert that they match the expected value
			assert.Equal(t, tt.want, typesResult,
				"types.IsActiveActive() = %v, want %v", typesResult, tt.want)
			assert.Equal(t, tt.want, persistenceResult,
				"persistence.IsActiveActive() = %v, want %v", persistenceResult, tt.want)
		})
	}
}
