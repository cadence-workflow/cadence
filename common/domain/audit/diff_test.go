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
	oldDomain := &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.InternalDomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.InternalDomainReplicationConfig{
			ActiveClusterName: "cluster1",
		},
	}

	newDomain := &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.InternalDomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.InternalDomainReplicationConfig{
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

func TestComputeDomainDiff_ActiveActiveFailover(t *testing.T) {
	// Create domain with cluster attributes
	// For active-active, we need to serialize the ActiveClusters map to a DataBlob
	// For this test, we'll use simple JSON encoding
	oldActiveClusters := map[string]interface{}{
		"region": map[string]interface{}{
			"us-east-1": map[string]interface{}{
				"ActiveClusterName": "cluster1",
				"FailoverVersion":   100,
			},
			"us-west-1": map[string]interface{}{
				"ActiveClusterName": "cluster2",
				"FailoverVersion":   101,
			},
		},
	}
	oldActiveClustersBytes, _ := json.Marshal(oldActiveClusters)

	oldDomain := &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		ReplicationConfig: &persistence.InternalDomainReplicationConfig{
			ActiveClusterName: "cluster1",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			ActiveClustersConfig: &persistence.DataBlob{
				Data:     oldActiveClustersBytes,
				Encoding: "json",
			},
		},
	}

	// Failover us-east-1 from cluster1 to cluster2
	newActiveClusters := map[string]interface{}{
		"region": map[string]interface{}{
			"us-east-1": map[string]interface{}{
				"ActiveClusterName": "cluster2", // Changed!
				"FailoverVersion":   102,
			},
			"us-west-1": map[string]interface{}{
				"ActiveClusterName": "cluster2",
				"FailoverVersion":   101,
			},
		},
	}
	newActiveClustersBytes, _ := json.Marshal(newActiveClusters)

	newDomain := &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		ReplicationConfig: &persistence.InternalDomainReplicationConfig{
			ActiveClusterName: "cluster1",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: "cluster1"},
				{ClusterName: "cluster2"},
			},
			ActiveClustersConfig: &persistence.DataBlob{
				Data:     newActiveClustersBytes,
				Encoding: "json",
			},
		},
	}

	diffBytes, err := ComputeDomainDiff(oldDomain, newDomain)
	require.NoError(t, err)
	assert.NotNil(t, diffBytes)

	// Verify the diff captures the cluster attribute change
	// The diff should show a change to the ActiveClustersConfig/Data blob
	diffStr := string(diffBytes)
	assert.Contains(t, diffStr, "/ReplicationConfig/ActiveClustersConfig/Data")
	assert.Contains(t, diffStr, "replace")

	// Decode the diff to verify it contains the new cluster configuration
	var patch []map[string]interface{}
	err = json.Unmarshal(diffBytes, &patch)
	require.NoError(t, err)
	assert.NotEmpty(t, patch)
}

func TestComputeDomainDiff_NoChanges(t *testing.T) {
	domain := &persistence.InternalGetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain",
		},
		Config: &persistence.InternalDomainConfig{
			Retention: 7,
		},
		ReplicationConfig: &persistence.InternalDomainReplicationConfig{
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
