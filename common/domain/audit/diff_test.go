package audit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestComputeDomainDiff_ActivePassiveFailover(t *testing.T) {
	oldDomain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.DomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster1",
		},
	}

	newDomain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.DomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster2",
		},
	}

	diffBytes, err := ComputeDomainDiff(oldDomain, newDomain)
	require.NoError(t, err)
	assert.NotNil(t, diffBytes)
	// Diff should be JSON patch format
	diffStr := string(diffBytes)
	assert.Contains(t, diffStr, "op")
	assert.Contains(t, diffStr, "cluster2")
}

func TestComputeDomainDiff_ClusterChange(t *testing.T) {
	// Test a simple cluster list change
	oldDomain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster1",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
		},
	}

	newDomain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster1",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
				{ClusterName: "cluster3"}, // Added cluster3
			},
		},
	}

	diffBytes, err := ComputeDomainDiff(oldDomain, newDomain)
	require.NoError(t, err)
	assert.NotNil(t, diffBytes)

	// Verify the diff shows the change
	var patch []map[string]interface{}
	err = json.Unmarshal(diffBytes, &patch)
	require.NoError(t, err)
	assert.NotEmpty(t, patch)
}

func TestComputeDomainDiff_NoChanges(t *testing.T) {
	domain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.DomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: "cluster1",
		},
	}

	diffBytes, err := ComputeDomainDiff(domain, domain)
	require.NoError(t, err)
	// Diff should be empty array or no operations
	var patch []interface{}
	err = json.Unmarshal(diffBytes, &patch)
	require.NoError(t, err)
	assert.Empty(t, patch, "diff should be empty when domains are identical")
}

func TestDetermineOperationType(t *testing.T) {
	tests := []struct {
		name     string
		request  *types.UpdateDomainRequest
		expected persistence.DomainOperationType
	}{
		{
			name: "failover - active cluster change",
			request: &types.UpdateDomainRequest{
				Name:              "test",
				ActiveClusterName: common.StringPtr("cluster2"),
			},
			expected: persistence.DomainOperationTypeFailover,
		},
		{
			name: "failover - active clusters change",
			request: &types.UpdateDomainRequest{
				Name: "test",
				ActiveClusters: &types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {},
					},
				},
			},
			expected: persistence.DomainOperationTypeFailover,
		},
		{
			name: "update - description change",
			request: &types.UpdateDomainRequest{
				Name:        "test",
				Description: common.StringPtr("new desc"),
			},
			expected: persistence.DomainOperationTypeUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineOperationType(tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}
