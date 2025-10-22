package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func TestReconstructFailoverEvents(t *testing.T) {
	entries := []*persistence.DomainAuditLogEntry{
		{
			EventID:       "event-1",
			CreatedTime:   time.Now().Add(-1 * time.Hour),
			OperationType: persistence.DomainOperationTypeFailover,
			StateAfter:    []byte(`[{"op":"replace","path":"/ReplicationConfig/ActiveClusterName","value":"cluster2"}]`),
		},
	}

	events, err := ReconstructFailoverEvents(entries)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "event-1", *events[0].ID)
}

func TestReconstructClusterFailovers_DefaultCluster(t *testing.T) {
	// Test default cluster failover (active-passive)
	diffBytes := []byte(`[{"op":"replace","path":"/ReplicationConfig/ActiveClusterName","value":"cluster2"}]`)

	failovers, err := ReconstructClusterFailovers(diffBytes)
	require.NoError(t, err)
	assert.Len(t, failovers, 1)

	failover := failovers[0]
	assert.Equal(t, "cluster2", failover.ToCluster.ActiveClusterName)
	assert.True(t, *failover.IsDefaultCluster)
	assert.Nil(t, failover.ClusterAttribute)
}

func TestReconstructClusterFailovers_ClusterAttributes(t *testing.T) {
	// Test cluster attribute failover (active-active)
	// Path format: /ReplicationConfig/ActiveClusters/region/ClusterAttributes/us-east-1/ActiveClusterName
	diffBytes := []byte(`[
		{
			"op":"replace",
			"path":"/ReplicationConfig/ActiveClusters/region/ClusterAttributes/us-east-1/ActiveClusterName",
			"value":"cluster2"
		},
		{
			"op":"replace",
			"path":"/ReplicationConfig/ActiveClusters/city/ClusterAttributes/seattle/ActiveClusterName",
			"value":"cluster3"
		}
	]`)

	failovers, err := ReconstructClusterFailovers(diffBytes)
	require.NoError(t, err)
	assert.Len(t, failovers, 2)

	// First failover: region/us-east-1
	assert.Equal(t, "cluster2", failovers[0].ToCluster.ActiveClusterName)
	assert.False(t, *failovers[0].IsDefaultCluster)
	assert.NotNil(t, failovers[0].ClusterAttribute)
	assert.Equal(t, "region", failovers[0].ClusterAttribute.Scope)
	assert.Equal(t, "us-east-1", failovers[0].ClusterAttribute.Name)

	// Second failover: city/seattle
	assert.Equal(t, "cluster3", failovers[1].ToCluster.ActiveClusterName)
	assert.False(t, *failovers[1].IsDefaultCluster)
	assert.NotNil(t, failovers[1].ClusterAttribute)
	assert.Equal(t, "city", failovers[1].ClusterAttribute.Scope)
	assert.Equal(t, "seattle", failovers[1].ClusterAttribute.Name)
}

func TestReconstructClusterFailovers_MixedFailover(t *testing.T) {
	// Test both default and cluster attribute failovers in same event
	diffBytes := []byte(`[
		{
			"op":"replace",
			"path":"/ReplicationConfig/ActiveClusterName",
			"value":"cluster2"
		},
		{
			"op":"replace",
			"path":"/ReplicationConfig/ActiveClusters/region/ClusterAttributes/us-east-1/ActiveClusterName",
			"value":"cluster3"
		}
	]`)

	failovers, err := ReconstructClusterFailovers(diffBytes)
	require.NoError(t, err)
	assert.Len(t, failovers, 2)

	// Find default cluster failover
	var defaultFailover *types.ClusterFailover
	var attrFailover *types.ClusterFailover
	for _, f := range failovers {
		if f.IsDefaultCluster != nil && *f.IsDefaultCluster {
			defaultFailover = f
		} else {
			attrFailover = f
		}
	}

	require.NotNil(t, defaultFailover)
	require.NotNil(t, attrFailover)

	assert.Equal(t, "cluster2", defaultFailover.ToCluster.ActiveClusterName)
	assert.Equal(t, "cluster3", attrFailover.ToCluster.ActiveClusterName)
	assert.Equal(t, "region", attrFailover.ClusterAttribute.Scope)
}

func TestReconstructClusterFailovers_InvalidJSON(t *testing.T) {
	// Test invalid JSON input
	diffBytes := []byte(`invalid json`)

	failovers, err := ReconstructClusterFailovers(diffBytes)
	require.Error(t, err)
	assert.Nil(t, failovers)
}

func TestReconstructClusterFailovers_EmptyInput(t *testing.T) {
	// Test empty patch
	diffBytes := []byte(`[]`)

	failovers, err := ReconstructClusterFailovers(diffBytes)
	require.NoError(t, err)
	assert.Len(t, failovers, 0)
}

func TestReconstructFailoverEvents_FiltersNonFailovers(t *testing.T) {
	// Test that non-failover operations are filtered out
	entries := []*persistence.DomainAuditLogEntry{
		{
			EventID:       "event-1",
			CreatedTime:   time.Now().Add(-2 * time.Hour),
			OperationType: persistence.DomainOperationTypeCreate,
			StateAfter:    []byte(`[{"op":"add","path":"/Name","value":"test-domain"}]`),
		},
		{
			EventID:       "event-2",
			CreatedTime:   time.Now().Add(-1 * time.Hour),
			OperationType: persistence.DomainOperationTypeFailover,
			StateAfter:    []byte(`[{"op":"replace","path":"/ReplicationConfig/ActiveClusterName","value":"cluster2"}]`),
		},
		{
			EventID:       "event-3",
			CreatedTime:   time.Now(),
			OperationType: persistence.DomainOperationTypeUpdate,
			StateAfter:    []byte(`[{"op":"replace","path":"/Description","value":"updated"}]`),
		},
	}

	events, err := ReconstructFailoverEvents(entries)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "event-2", *events[0].ID)
	assert.Equal(t, types.FailoverTypeForce.Ptr(), events[0].FailoverType)
}

func TestReconstructFailoverEvents_EmptyInput(t *testing.T) {
	// Test empty input
	events, err := ReconstructFailoverEvents(nil)
	require.NoError(t, err)
	assert.Len(t, events, 0)

	events, err = ReconstructFailoverEvents([]*persistence.DomainAuditLogEntry{})
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestGetFailoverEventDetails_ValidInput(t *testing.T) {
	entry := &persistence.DomainAuditLogEntry{
		EventID:       "event-1",
		CreatedTime:   time.Now(),
		OperationType: persistence.DomainOperationTypeFailover,
		StateAfter: []byte(`[
			{
				"op":"replace",
				"path":"/ReplicationConfig/ActiveClusterName",
				"value":"cluster2"
			}
		]`),
	}

	response, err := GetFailoverEventDetails(entry)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Len(t, response.ClusterFailovers, 1)
	assert.Equal(t, "cluster2", response.ClusterFailovers[0].ToCluster.ActiveClusterName)
}

func TestGetFailoverEventDetails_InvalidJSON(t *testing.T) {
	entry := &persistence.DomainAuditLogEntry{
		EventID:       "event-1",
		CreatedTime:   time.Now(),
		OperationType: persistence.DomainOperationTypeFailover,
		StateAfter:    []byte(`invalid json`),
	}

	response, err := GetFailoverEventDetails(entry)
	require.Error(t, err)
	assert.Nil(t, response)
}
