package shardcache

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/common"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/testhelper"
)

func TestNamespaceShardToExecutor_Lifecycle(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)
	logger := testlogger.New(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Setup: Create executor-1 with shard-1
	setupExecutorWithShards(t, testCluster, "executor-1", []string{"shard-1"}, map[string]string{
		"hostname": "executor-1-host",
		"version":  "v1.0.0",
	})

	// Start the cache
	mockTime := clock.NewMockedTimeSource()
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, mockTime)
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})

	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		owner, _ := namespaceShardToExecutor.GetShardOwner(context.Background(), "shard-1")
		return owner != nil && owner.ExecutorID == "executor-1"
	}, 1*time.Second, 10*time.Millisecond, "cache was not populated with shard-1 in time")

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
	setupExecutorWithShards(t, testCluster, "executor-2", []string{"shard-2"}, map[string]string{
		"hostname": "executor-2-host",
		"region":   "us-west",
	})

	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		owner, _ := namespaceShardToExecutor.GetShardOwner(context.Background(), "shard-2")
		return owner != nil && owner.ExecutorID == "executor-2"
	}, 1*time.Second, 10*time.Millisecond, "cache was not populated with shard-2 in time")

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

func TestNamespaceShardToExecutor_Subscribe(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)
	logger := testlogger.New(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Setup: Create executor-1 with shard-1
	setupExecutorWithShards(t, testCluster, "executor-1", []string{"shard-1"}, map[string]string{
		"hostname": "executor-1-host",
		"version":  "v1.0.0",
	})

	// Start the cache
	mockTime := clock.NewMockedTimeSource()
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, mockTime)
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})

	// Refresh the cache to get the initial state
	namespaceShardToExecutor.refresh(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	subCh, unSub := namespaceShardToExecutor.Subscribe(ctx)
	defer unSub()

	var wg sync.WaitGroup
	wg.Add(1)

	// start listener
	go func() {
		defer wg.Done()
		// Check that we get the initial state
		state := <-subCh
		assert.Len(t, state, 1)
		verifyExecutorInState(t, state, "executor-1", []string{"shard-1"}, map[string]string{
			"hostname": "executor-1-host",
			"version":  "v1.0.0",
		})

		// Check that we get the updated state
		state = <-subCh
		assert.Len(t, state, 2)
		verifyExecutorInState(t, state, "executor-1", []string{"shard-1"}, map[string]string{
			"hostname": "executor-1-host",
			"version":  "v1.0.0",
		})
		verifyExecutorInState(t, state, "executor-2", []string{"shard-2"}, map[string]string{
			"hostname": "executor-2-host",
			"region":   "us-west",
		})
	}()

	// Add executor-2 with shard-2 to trigger new subscription update
	setupExecutorWithShards(t, testCluster, "executor-2", []string{"shard-2"}, map[string]string{
		"hostname": "executor-2-host",
		"region":   "us-west",
	})
	// Let the watch event propagate
	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		owner, _ := namespaceShardToExecutor.GetShardOwner(context.Background(), "shard-2")
		return owner != nil && owner.ExecutorID == "executor-2"
	}, 1*time.Second, 10*time.Millisecond, "cache was not populated with shard-2 in time")
}

// setupExecutorWithShards creates an executor in etcd with assigned shards and metadata
func setupExecutorWithShards(t *testing.T, testCluster *testhelper.StoreTestCluster, executorID string, shards []string, metadata map[string]string) {
	// Create assigned state
	assignedState := &etcdtypes.AssignedState{
		AssignedShards: make(map[string]*types.ShardAssignment),
	}
	for _, shardID := range shards {
		assignedState.AssignedShards[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
	}
	assignedStateJSON, err := json.Marshal(assignedState)
	require.NoError(t, err)

	var operations []clientv3.Op

	executorAssignedStateKey := etcdkeys.BuildExecutorKey(testCluster.EtcdPrefix, testCluster.Namespace, executorID, etcdkeys.ExecutorAssignedStateKey)
	operations = append(operations, clientv3.OpPut(executorAssignedStateKey, string(assignedStateJSON)))

	// Add metadata
	for key, value := range metadata {
		metadataKey := etcdkeys.BuildMetadataKey(testCluster.EtcdPrefix, testCluster.Namespace, executorID, key)
		operations = append(operations, clientv3.OpPut(metadataKey, value))
	}

	txnResp, err := testCluster.Client.Txn(context.Background()).Then(operations...).Commit()
	require.NoError(t, err)
	require.True(t, txnResp.Succeeded)
}

func verifyExecutorInState(t *testing.T, state map[*store.ShardOwner][]string, executorID string, shards []string, metadata map[string]string) {
	executorInState := false
	for executor, executorShards := range state {
		if executor.ExecutorID == executorID {
			assert.Equal(t, shards, executorShards)
			assert.Equal(t, metadata, executor.Metadata)
			executorInState = true
			break
		}
	}
	assert.True(t, executorInState)
}

func TestNamespaceShardToExecutor_ExecutorStatistics(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)
	logger := testlogger.New(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	executorID := "executor-stats"
	shardID1 := "shard-stats-1"
	shardID2 := "shard-stats-2"

	now := time.Now().UTC()

	initialStats := map[string]etcdtypes.ShardStatistics{
		shardID1: {SmoothedLoad: 10.0, LastUpdateTime: etcdtypes.Time(now.Add(-time.Hour)), LastMoveTime: etcdtypes.Time(now.Add(-2 * time.Hour))},
		shardID2: {SmoothedLoad: 20.0, LastUpdateTime: etcdtypes.Time(now.Add(-30 * time.Minute)), LastMoveTime: etcdtypes.Time(now.Add(-90 * time.Minute))},
	}
	putExecutorStatisticsInEtcd(t, testCluster, executorID, initialStats)

	mockTime := clock.NewMockedTimeSource()
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, mockTime)
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})

	// Wait for initial stats to be loaded
	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		stats, _ := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
		return len(stats) == 2
	}, 1*time.Second, 10*time.Millisecond, "initial stats not loaded")

	statsFromCache, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
	require.NoError(t, err)
	assert.Equal(t, initialStats, statsFromCache)

	updatedStats := map[string]etcdtypes.ShardStatistics{
		shardID1: {SmoothedLoad: 15.0, LastUpdateTime: etcdtypes.Time(now)},
		shardID2: {SmoothedLoad: 25.0, LastUpdateTime: etcdtypes.Time(now)},
	}
	putExecutorStatisticsInEtcd(t, testCluster, executorID, updatedStats)

	// Wait for stats to be updated via watch event
	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		stats, _ := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
		return stats[shardID1].SmoothedLoad == 15.0
	}, 1*time.Second, 10*time.Millisecond, "stats not updated")

	statsFromCacheAfterUpdate, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
	require.NoError(t, err)
	assert.Equal(t, updatedStats, statsFromCacheAfterUpdate)

	nonExistentStats, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), "non-existent-executor")
	require.NoError(t, err) // No error, just empty map
	assert.Empty(t, nonExistentStats)

	statsKey := etcdkeys.BuildExecutorKey(testCluster.EtcdPrefix, testCluster.Namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	_, err = testCluster.Client.Delete(context.Background(), statsKey)
	require.NoError(t, err)

	// Wait for stats to be deleted via watch event
	require.Eventually(t, func() bool {
		mockTime.Advance(10 * time.Millisecond)
		stats, _ := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
		return len(stats) == 0
	}, 1*time.Second, 10*time.Millisecond, "stats not deleted")

	deletedStats, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
	require.NoError(t, err)
	assert.Empty(t, deletedStats)
}

// putExecutorStatisticsInEtcd is a helper to directly put compressed executor statistics into etcd.
func putExecutorStatisticsInEtcd(t *testing.T, tc *testhelper.StoreTestCluster, executorID string, stats map[string]etcdtypes.ShardStatistics) {
	payload, err := json.Marshal(stats)
	require.NoError(t, err)

	writer, err := common.NewRecordWriter(tc.Compression)
	require.NoError(t, err)
	compressedPayload, err := writer.Write(payload)
	require.NoError(t, err)

	statsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	_, err = tc.Client.Put(context.Background(), statsKey, string(compressedPayload))
	require.NoError(t, err)
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
