package shardcache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/testhelper"
)

// setupExecutorWithShards creates an executor in etcd with assigned shards and metadata
func setupExecutorWithShards(t *testing.T, testCluster *testhelper.StoreTestCluster, namespace, executorID string, shards []string, metadata map[string]string) {
	// Create assigned state
	assignedState := &etcdtypes.AssignedState{
		AssignedShards: make(map[string]*types.ShardAssignment),
		LastUpdated:    etcdtypes.Time(time.Now().UTC()),
	}
	for _, shardID := range shards {
		assignedState.AssignedShards[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
	}
	assignedStateJSON, err := json.Marshal(assignedState)
	require.NoError(t, err)

	executorAssignedStateKey := etcdkeys.BuildExecutorKey(testCluster.EtcdPrefix, namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
	testCluster.Client.Put(context.Background(), executorAssignedStateKey, string(assignedStateJSON))

	// Add metadata
	for key, value := range metadata {
		metadataKey := etcdkeys.BuildMetadataKey(testCluster.EtcdPrefix, namespace, executorID, key)
		testCluster.Client.Put(context.Background(), metadataKey, value)
	}
}

// verifyShardOwner checks that a shard has the expected owner and metadata
func verifyShardOwner(t *testing.T, cache *namespaceShardToExecutor, shardID, expectedExecutorID string, expectedMetadata map[string]string) {
	owner, err := cache.GetShardOwner(context.Background(), shardID)
	require.NoError(t, err)
	require.NotNil(t, owner)
	assert.Equal(t, expectedExecutorID, owner.ExecutorID)
	for key, expectedValue := range expectedMetadata {
		assert.Equal(t, expectedValue, owner.Metadata[key])
	}

	executor, err := cache.GetExecutor(context.Background(), expectedExecutorID)
	require.NoError(t, err)
	require.NotNil(t, executor)
	assert.Equal(t, expectedExecutorID, executor.ExecutorID)
	for key, expectedValue := range expectedMetadata {
		assert.Equal(t, expectedValue, executor.Metadata[key])
	}
}

func TestNamespaceShardToExecutor_Lifecycle(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)
	logger := testlogger.New(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	namespace := fmt.Sprintf("test-ns-%s", t.Name())

	// Setup: Create executor-1 with shard-1
	setupExecutorWithShards(t, testCluster, namespace, "executor-1", []string{"shard-1"}, map[string]string{
		"hostname": "executor-1-host",
		"version":  "v1.0.0",
	})

	// Start the cache
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, namespace, testCluster.Client, stopCh, logger)
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})
	time.Sleep(50 * time.Millisecond)

	// Verify executor-1 owns shard-1 with correct metadata
	verifyShardOwner(t, namespaceShardToExecutor, "shard-1", "executor-1", map[string]string{
		"hostname": "executor-1-host",
		"version":  "v1.0.0",
	})

	// Check the cache is populated
	namespaceShardToExecutor.RLock()
	_, ok := namespaceShardToExecutor.executorRevision["executor-1"]
	assert.True(t, ok)
	assert.Equal(t, "executor-1", namespaceShardToExecutor.shardToExecutor["shard-1"].ExecutorID)
	namespaceShardToExecutor.RUnlock()

	// Add executor-2 with shard-2 to trigger watch update
	setupExecutorWithShards(t, testCluster, namespace, "executor-2", []string{"shard-2"}, map[string]string{
		"hostname": "executor-2-host",
		"region":   "us-west",
	})
	time.Sleep(100 * time.Millisecond)

	// Check that executor-2 and shard-2 is in the cache
	namespaceShardToExecutor.RLock()
	_, ok = namespaceShardToExecutor.executorRevision["executor-2"]
	assert.True(t, ok)
	assert.Equal(t, "executor-2", namespaceShardToExecutor.shardToExecutor["shard-2"].ExecutorID)
	namespaceShardToExecutor.RUnlock()

	// Verify executor-2 owns shard-2 with correct metadata
	verifyShardOwner(t, namespaceShardToExecutor, "shard-2", "executor-2", map[string]string{
		"hostname": "executor-2-host",
		"region":   "us-west",
	})
}
