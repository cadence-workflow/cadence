package shardcache

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdclient"
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
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, clock.NewRealTimeSource())
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
	setupExecutorWithShards(t, testCluster, "executor-2", []string{"shard-2"}, map[string]string{
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
	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, clock.NewRealTimeSource())
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})

	// Refresh the cache to get the initial state
	err = namespaceShardToExecutor.refresh(context.Background())
	require.NoError(t, err)

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
	time.Sleep(10 * time.Millisecond)

	// Add executor-2 with shard-2 to trigger new subscription update
	setupExecutorWithShards(t, testCluster, "executor-2", []string{"shard-2"}, map[string]string{
		"hostname": "executor-2-host",
		"region":   "us-west",
	})

	wg.Wait()

}

func TestNamespaceShardToExecutor_watch_watchChanErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	mockClient := etcdclient.NewMockClient(ctrl)
	stopCh := make(chan struct{})
	testPrefix := "/test-prefix"
	testNamespace := "test-namespace"

	// Mock the Watch call to return our watch channel
	watchChan := make(chan clientv3.WatchResponse)
	mockClient.EXPECT().
		Watch(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(watchChan).
		AnyTimes()

	e, err := newNamespaceShardToExecutor(testPrefix, testNamespace, mockClient, stopCh, logger, clock.NewRealTimeSource())
	require.NoError(t, err)

	// Test Case #1
	// Test received compact revision error from watch channel
	{
		go func() {
			watchChan <- clientv3.WatchResponse{
				CompactRevision: 100,
			}
		}()

		err = e.watch()
		require.Error(t, err)
		assert.ErrorContains(t, err, "etcdserver: mvcc: required revision has been compacted")
	}

	// Test Case #2
	// Test closed watch channel
	{
		close(watchChan)
		err = e.watch()
		require.Error(t, err)
		assert.ErrorContains(t, err, "watch channel closed")
	}
}

func TestNamespaceShardToExecutor_namespaceRefreshLoop_watchError(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logger := testlogger.New(t)
	mockClient := etcdclient.NewMockClient(ctrl)
	timeSource := clock.NewMockedTimeSource()
	stopCh := make(chan struct{})
	testPrefix := "/test-prefix"
	testNamespace := "test-namespace"

	// mock for first watch call that receives error
	watchChanRcvErr := make(chan clientv3.WatchResponse)
	mockClient.EXPECT().
		Watch(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(watchChanRcvErr)

	// mock for second watch call that receives closed channel
	watchChanClosed := make(chan clientv3.WatchResponse)
	mockClient.EXPECT().
		Watch(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(watchChanClosed)

	// mock for third watch call that will be used when stopCh is closed
	mockClient.EXPECT().
		Watch(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(make(chan clientv3.WatchResponse))

	e, err := newNamespaceShardToExecutor(testPrefix, testNamespace, mockClient, stopCh, logger, timeSource)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	finished := atomic.Bool{}

	go func() {
		defer wg.Done()
		e.namespaceRefreshLoop()
		finished.Store(true)
	}()

	// Test Case #1: watchChan receives error
	{
		// Sends a response containing compact revision to simulate error
		watchChanRcvErr <- clientv3.WatchResponse{
			CompactRevision: 100,
		}

		timeSource.BlockUntil(1)
		require.False(t, finished.Load(), "namespaceRefreshLoop should not exit on watch error")
	}

	// Test Case #2: watchChan is closed
	{
		timeSource.Advance(2 * namespaceRefreshLoopWatchRetryInterval)

		// Sends a response containing compact revision to simulate error
		close(watchChanClosed)

		timeSource.BlockUntil(1)
		require.False(t, finished.Load(), "namespaceRefreshLoop should not exit on watch error")
	}

	// Test Case #3: stopCh is closed
	{
		timeSource.Advance(2 * namespaceRefreshLoopWatchRetryInterval)

		close(stopCh)
		wg.Wait()
		require.True(t, finished.Load(), "namespaceRefreshLoop should exit on watch error")
	}
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
	mockTime := clock.NewMockedTimeSource()

	executorID := "executor-stats"
	shardID1 := "shard-stats-1"
	shardID2 := "shard-stats-2"

	initialStats := map[string]etcdtypes.ShardStatistics{
		shardID1: {SmoothedLoad: 10.0, LastUpdateTime: etcdtypes.Time(mockTime.Now().UTC().Add(-time.Hour)), LastMoveTime: etcdtypes.Time(mockTime.Now().UTC().Add(-2 * time.Hour))},
		shardID2: {SmoothedLoad: 20.0, LastUpdateTime: etcdtypes.Time(mockTime.Now().UTC().Add(-30 * time.Minute)), LastMoveTime: etcdtypes.Time(mockTime.Now().UTC().Add(-90 * time.Minute))},
	}
	putExecutorStatisticsInEtcd(t, testCluster, executorID, initialStats)

	namespaceShardToExecutor, err := newNamespaceShardToExecutor(testCluster.EtcdPrefix, testCluster.Namespace, testCluster.Client, stopCh, logger, mockTime)
	assert.NoError(t, err)
	namespaceShardToExecutor.Start(&sync.WaitGroup{})

	// GetExecutorStatistics triggers refresh on cache miss, so no need to wait for timers
	statsFromCache, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
	require.NoError(t, err)
	assert.Equal(t, initialStats, statsFromCache)

	updatedStats := map[string]etcdtypes.ShardStatistics{
		shardID1: {SmoothedLoad: 15.0, LastUpdateTime: etcdtypes.Time(mockTime.Now().UTC())},
		shardID2: {SmoothedLoad: 25.0, LastUpdateTime: etcdtypes.Time(mockTime.Now().UTC())},
	}
	putExecutorStatisticsInEtcd(t, testCluster, executorID, updatedStats)

	// Wait for stats to be updated via watch event
	assert.Eventually(t, func() bool {
		stats, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
		if err != nil {
			return false
		}
		return stats[shardID1].SmoothedLoad == 15.0
	}, 5*time.Second, 50*time.Millisecond, "stats should be updated via watch")

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
	assert.Eventually(t, func() bool {
		stats, err := namespaceShardToExecutor.GetExecutorStatistics(context.Background(), executorID)
		if err != nil {
			return false
		}
		return len(stats) == 0
	}, 5*time.Second, 50*time.Millisecond, "stats should be deleted via watch")

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
