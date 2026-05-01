package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

func TestDomainAuditLog_ToFailoverEvents(t *testing.T) {
	now := time.Unix(1234567890, 0)

	tests := map[string]struct {
		auditLog *DomainAuditLog
		expected *types.FailoverEvent
	}{
		"simple active cluster failover": {
			auditLog: &DomainAuditLog{
				EventID:     "event-1",
				DomainID:    "domain-1",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					Info: &DomainInfo{
						ID:   "domain-1",
						Name: "test-domain",
					},
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-us-east",
						Clusters: []*ClusterReplicationConfig{
							{ClusterName: "cluster-us-east"},
							{ClusterName: "cluster-us-west"},
						},
					},
					FailoverVersion: 100,
				},
				StateAfter: &GetDomainResponse{
					Info: &DomainInfo{
						ID:   "domain-1",
						Name: "test-domain",
					},
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-us-west",
						Clusters: []*ClusterReplicationConfig{
							{ClusterName: "cluster-us-east"},
							{ClusterName: "cluster-us-west"},
						},
					},
					FailoverVersion: 101,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-1"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-east",
							FailoverVersion:   100,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-west",
							FailoverVersion:   101,
						},
					},
				},
			},
		},
		"cluster attribute failover - single region change": {
			auditLog: &DomainAuditLog{
				EventID:     "event-2",
				DomainID:    "active-active-domain",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					Info: &DomainInfo{
						ID:   "domain-2",
						Name: "active-active-domain",
					},
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us-east": {ActiveClusterName: "cluster0", FailoverVersion: 200},
										"us-west": {ActiveClusterName: "cluster1", FailoverVersion: 201},
									},
								},
							},
						},
					},
					FailoverVersion: 200,
				},
				StateAfter: &GetDomainResponse{
					Info: &DomainInfo{
						ID:   "domain-2",
						Name: "active-active-domain",
					},
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us-east": {ActiveClusterName: "cluster1", FailoverVersion: 201},
										"us-west": {ActiveClusterName: "cluster1", FailoverVersion: 201},
									},
								},
							},
						},
					},
					FailoverVersion: 201,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-2"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster0",
							FailoverVersion:   200,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster1",
							FailoverVersion:   201,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-east",
						},
					},
				},
			},
		},
		"cluster attribute failover - multiple regions change": {
			auditLog: &DomainAuditLog{
				EventID:     "event-3",
				DomainID:    "domain-3",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"region-1": {ActiveClusterName: "cluster-us-east", FailoverVersion: 300},
										"region-2": {ActiveClusterName: "cluster-us-west", FailoverVersion: 301},
										"region-3": {ActiveClusterName: "cluster-eu-central", FailoverVersion: 302},
									},
								},
							},
						},
					},
					FailoverVersion: 300,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"region-1": {ActiveClusterName: "cluster-eu-central", FailoverVersion: 302},
										"region-2": {ActiveClusterName: "cluster-eu-central", FailoverVersion: 302},
										"region-3": {ActiveClusterName: "cluster-eu-central", FailoverVersion: 302},
									},
								},
							},
						},
					},
					FailoverVersion: 400,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-3"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-east",
							FailoverVersion:   300,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-eu-central",
							FailoverVersion:   302,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "region-1",
						},
					},
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-west",
							FailoverVersion:   301,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-eu-central",
							FailoverVersion:   302,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "region-2",
						},
					},
				},
			},
		},
		"combined failover - both active cluster and attributes": {
			auditLog: &DomainAuditLog{
				EventID:     "event-4",
				DomainID:    "domain-4",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-primary",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us": {ActiveClusterName: "cluster-us", FailoverVersion: 400},
									},
								},
							},
						},
					},
					FailoverVersion: 400,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-secondary",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us": {ActiveClusterName: "cluster-us-backup", FailoverVersion: 401},
									},
								},
							},
						},
					},
					FailoverVersion: 401,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-4"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-primary",
							FailoverVersion:   400,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-secondary",
							FailoverVersion:   401,
						},
					},
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us",
							FailoverVersion:   400,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-backup",
							FailoverVersion:   401,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us",
						},
					},
				},
			},
		},
		"new cluster attribute scope added": {
			auditLog: &DomainAuditLog{
				EventID:     "event-5",
				DomainID:    "domain-5",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
					},
					FailoverVersion: 500,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us-east": {ActiveClusterName: "cluster-us-east", FailoverVersion: 501},
										"eu":      {ActiveClusterName: "cluster-eu", FailoverVersion: 502},
									},
								},
							},
						},
					},
					FailoverVersion: 502,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-5"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-eu",
							FailoverVersion:   502,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "eu",
						},
					},
					{
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-east",
							FailoverVersion:   501,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-east",
						},
					},
				},
			},
		},
		"no changes - same active cluster": {
			auditLog: &DomainAuditLog{
				EventID:     "event-6",
				DomainID:    "domain-6",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-stable",
					},
					FailoverVersion: 600,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-stable",
					},
					FailoverVersion: 600,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:               stringPtr("event-6"),
				CreatedTime:      int64Ptr(now.UnixNano()),
				FailoverType:     types.FailoverTypeForce.Ptr(),
				ClusterFailovers: nil,
			},
		},
		"nil replication config": {
			auditLog: &DomainAuditLog{
				EventID:       "event-7",
				DomainID:      "domain-7",
				CreatedTime:   now,
				StateBefore:   &GetDomainResponse{},
				StateAfter:    &GetDomainResponse{},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:               stringPtr("event-7"),
				FailoverType:     types.FailoverTypeForce.Ptr(),
				CreatedTime:      int64Ptr(now.UnixNano()),
				ClusterFailovers: nil,
			},
		},
		"multiple scopes with changes": {
			auditLog: &DomainAuditLog{
				EventID:     "event-8",
				DomainID:    "domain-8",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us": {ActiveClusterName: "cluster-us-1", FailoverVersion: 700},
									},
								},
								"datacenter": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"dc1": {ActiveClusterName: "cluster-dc1-primary", FailoverVersion: 700},
									},
								},
							},
						},
					},
					FailoverVersion: 700,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us": {ActiveClusterName: "cluster-us-2", FailoverVersion: 701},
									},
								},
								"datacenter": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"dc1": {ActiveClusterName: "cluster-dc1-backup", FailoverVersion: 701},
									},
								},
							},
						},
					},
					FailoverVersion: 701,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-8"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-dc1-primary",
							FailoverVersion:   700,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-dc1-backup",
							FailoverVersion:   701,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "datacenter",
							Name:  "dc1",
						},
					},
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-1",
							FailoverVersion:   700,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-2",
							FailoverVersion:   701,
						},
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us",
						},
					},
				},
			},
		},
		"cluster attribute scope removed": {
			auditLog: &DomainAuditLog{
				EventID:     "event-9",
				DomainID:    "domain-9",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"us-east": {ActiveClusterName: "cluster-us-east", FailoverVersion: 800},
										"us-west": {ActiveClusterName: "cluster-us-west", FailoverVersion: 801},
									},
								},
							},
						},
					},
					FailoverVersion: 800,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "cluster-default",
					},
					FailoverVersion: 801,
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-9"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeForce.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-east",
							FailoverVersion:   800,
						},
						ToCluster: nil,
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-east",
						},
					},
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "cluster-us-west",
							FailoverVersion:   801,
						},
						ToCluster: nil,
						ClusterAttribute: &types.ClusterAttribute{
							Scope: "region",
							Name:  "us-west",
						},
					},
				},
			},
		},
		"graceful failover": {
			auditLog: &DomainAuditLog{
				EventID:     "event-9",
				DomainID:    "domain-9",
				CreatedTime: now,
				StateBefore: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "region-us-east",
					},
					FailoverVersion: 800,
				},
				StateAfter: &GetDomainResponse{
					ReplicationConfig: &DomainReplicationConfig{
						ActiveClusterName: "region-us-west",
					},
					FailoverVersion: 801,
					FailoverEndTime: int64Ptr(now.Add(1 * time.Hour).UnixNano()),
				},
				OperationType: DomainAuditOperationTypeFailover,
			},
			expected: &types.FailoverEvent{
				ID:           stringPtr("event-9"),
				CreatedTime:  int64Ptr(now.UnixNano()),
				FailoverType: types.FailoverTypeGraceful.Ptr(),
				ClusterFailovers: []*types.ClusterFailover{
					{
						FromCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "region-us-east",
							FailoverVersion:   800,
						},
						ToCluster: &types.ActiveClusterInfo{
							ActiveClusterName: "region-us-west",
							FailoverVersion:   801,
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := tc.auditLog.ToFailoverEvents()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestSerializeDeserializeGetDomainResponse_Roundtrip(t *testing.T) {
	tests := map[string]struct {
		input        *GetDomainResponse
		encodingType constants.EncodingType
	}{
		"nil response": {
			input:        nil,
			encodingType: constants.EncodingTypeThriftRWSnappy,
		},
		"empty response with ThriftRW": {
			input:        &GetDomainResponse{},
			encodingType: constants.EncodingTypeThriftRW,
		},
		"empty response with ThriftRWSnappy": {
			input:        &GetDomainResponse{},
			encodingType: constants.EncodingTypeThriftRWSnappy,
		},
		"full response with all fields": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:          "test-domain-id",
					Name:        "test-domain",
					Status:      0,
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
				IsGlobalDomain:              true,
				FailoverVersion:             100,
				ConfigVersion:               5,
				FailoverNotificationVersion: 99,
				PreviousFailoverVersion:     98,
				LastUpdatedTime:             1234567890,
				NotificationVersion:         10,
			},
			encodingType: constants.EncodingTypeThriftRWSnappy,
		},
		"response with failover info": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 0,
				},
				FailoverVersion: 200,
				LastUpdatedTime: 1000,
				FailoverEndTime: func() *int64 { t := int64(2000); return &t }(),
			},
			encodingType: constants.EncodingTypeThriftRW,
		},
		"response with nil clusters in ReplicationConfig": {
			input: &GetDomainResponse{
				Info: &DomainInfo{
					ID:     "test-id",
					Name:   "test",
					Status: 1,
				},
				ReplicationConfig: &DomainReplicationConfig{
					ActiveClusterName: "cluster-1",
					Clusters: []*ClusterReplicationConfig{
						{ClusterName: "cluster-1"},
						{ClusterName: "cluster-2"},
					},
				},
			},
			encodingType: constants.EncodingTypeThriftRWSnappy,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Serialize
			blob, err := serializeGetDomainResponse(tc.input, tc.encodingType)
			assert.NoError(t, err)

			if tc.input == nil {
				assert.Nil(t, blob)
				return
			}

			assert.NotNil(t, blob)
			assert.Equal(t, tc.encodingType, blob.Encoding)
			assert.NotEmpty(t, blob.Data)

			// Deserialize
			deserialized, err := deserializeGetDomainResponse(blob)
			assert.NoError(t, err)
			assert.NotNil(t, deserialized)

			// Build expected result accounting for non-symmetric fields
			expected := &GetDomainResponse{
				Info:              tc.input.Info,
				Config:            tc.input.Config,
				ReplicationConfig: tc.input.ReplicationConfig,
				IsGlobalDomain:    tc.input.IsGlobalDomain,
				FailoverVersion:   tc.input.FailoverVersion,
				LastUpdatedTime:   tc.input.LastUpdatedTime,
				// Fields below are not present in types.DescribeDomainResponse and don't roundtrip:
				FailoverNotificationVersion: 0,
				PreviousFailoverVersion:     0,
				FailoverEndTime:             nil,
				ConfigVersion:               0,
				NotificationVersion:         0,
			}

			// Apply transformations that the mapper does
			if expected.Info != nil {
				// Clamp Status to valid range (0-2) as ToType does
				if expected.Info.Status < 0 || expected.Info.Status > 2 {
					expected.Info.Status = 0
				}
			}

			assert.Equal(t, expected, deserialized)
		})
	}
}

func TestDeserializeGetDomainResponse_EmptyBlob(t *testing.T) {
	tests := map[string]struct {
		blob     *DataBlob
		expected *GetDomainResponse
	}{
		"nil blob": {
			blob:     nil,
			expected: nil,
		},
		"empty data": {
			blob: &DataBlob{
				Data:     []byte{},
				Encoding: constants.EncodingTypeThriftRWSnappy,
			},
			expected: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := deserializeGetDomainResponse(tc.blob)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDeserializeGetDomainResponse_InvalidEncoding(t *testing.T) {
	blob := &DataBlob{
		Data:     []byte("some data"),
		Encoding: constants.EncodingType("invalid"),
	}

	result, err := deserializeGetDomainResponse(blob)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.IsType(t, &UnknownEncodingTypeError{}, err)
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
