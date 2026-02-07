package executorstore

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/statistics"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdtypes"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/executorstore/common"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/leaderstore"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/testhelper"
)

// TestRecordHeartbeat verifies that an executor's heartbeat is correctly stored.
func TestRecordHeartbeat(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()

	executorID := "executor-TestRecordHeartbeat"
	req := store.HeartbeatState{
		LastHeartbeat: now,
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			"shard-TestRecordHeartbeat": {Status: types.ShardStatusREADY},
		},
		Metadata: map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
		},
	}

	err := executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, req)
	require.NoError(t, err)

	// Verify directly in etcd
	heartbeatKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorHeartbeatKey)
	stateKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorStatusKey)
	reportedShardsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorReportedShardsKey)
	metadataKey1 := etcdkeys.BuildMetadataKey(tc.EtcdPrefix, tc.Namespace, executorID, "key-1")
	metadataKey2 := etcdkeys.BuildMetadataKey(tc.EtcdPrefix, tc.Namespace, executorID, "key-2")

	resp, err := tc.Client.Get(ctx, heartbeatKey)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Count, "Heartbeat key should exist")
	assert.Equal(t, etcdtypes.FormatTime(now), string(resp.Kvs[0].Value))

	resp, err = tc.Client.Get(ctx, stateKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "State key should exist")
	decompressedState, err := common.Decompress(resp.Kvs[0].Value)
	require.NoError(t, err)
	assert.Equal(t, stringStatus(types.ExecutorStatusACTIVE), string(decompressedState))

	resp, err = tc.Client.Get(ctx, reportedShardsKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "Reported shards key should exist")

	decompressedReportedShards, err := common.Decompress(resp.Kvs[0].Value)
	require.NoError(t, err)
	var reportedShards map[string]*types.ShardStatusReport
	err = json.Unmarshal(decompressedReportedShards, &reportedShards)
	require.NoError(t, err)
	require.Len(t, reportedShards, 1)
	assert.Equal(t, types.ShardStatusREADY, reportedShards["shard-TestRecordHeartbeat"].Status)

	resp, err = tc.Client.Get(ctx, metadataKey1)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "Metadata key 1 should exist")
	assert.Equal(t, "value-1", string(resp.Kvs[0].Value))

	resp, err = tc.Client.Get(ctx, metadataKey2)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "Metadata key 2 should exist")
	assert.Equal(t, "value-2", string(resp.Kvs[0].Value))
}

func TestRecordHeartbeat_NoCompression(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)

	var etcdCfg struct {
		Endpoints   []string      `yaml:"endpoints"`
		DialTimeout time.Duration `yaml:"dialTimeout"`
		Prefix      string        `yaml:"prefix"`
		Compression string        `yaml:"compression"`
	}
	require.NoError(t, tc.LeaderCfg.Store.StorageParams.Decode(&etcdCfg))
	etcdCfg.Compression = "none"

	encodedCfg, err := yaml.Marshal(etcdCfg)
	require.NoError(t, err)

	var yamlNode *config.YamlNode
	require.NoError(t, yaml.Unmarshal(encodedCfg, &yamlNode))
	tc.LeaderCfg.Store.StorageParams = yamlNode
	tc.LeaderCfg.LeaderStore.StorageParams = yamlNode
	tc.Compression = "none"

	executorStore := createStore(t, tc)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "executor-no-compression"
	req := store.HeartbeatState{
		LastHeartbeat: time.Now().UTC(),
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			"shard-no-compression": {Status: types.ShardStatusREADY},
		},
	}

	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, req))

	stateKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorStatusKey)
	require.NoError(t, err)
	reportedShardsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorReportedShardsKey)
	require.NoError(t, err)

	stateResp, err := tc.Client.Get(ctx, stateKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), stateResp.Count)
	statusJSON, err := json.Marshal(req.Status)
	require.NoError(t, err)
	assert.Equal(t, string(statusJSON), string(stateResp.Kvs[0].Value))

	reportedResp, err := tc.Client.Get(ctx, reportedShardsKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), reportedResp.Count)
	reportedJSON, err := json.Marshal(req.ReportedShards)
	require.NoError(t, err)
	assert.Equal(t, string(reportedJSON), string(reportedResp.Kvs[0].Value))
}

func TestRecordHeartbeatUpdatesShardStatistics(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	setLoadBalancingMode(executorStore, config.LoadBalancingModeGREEDY)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "executor-shard-stats"
	shardID := "shard-with-load"

	baseTime := time.Now().UTC()
	initialStats := store.ShardStatistics{
		SmoothedLoad:   1.23,
		LastUpdateTime: baseTime.Add(-5 * time.Second),
		LastMoveTime:   baseTime.Add(-30 * time.Second),
	}

	executorStats := map[string]etcdtypes.ShardStatistics{
		shardID: *etcdtypes.FromShardStatistics(&initialStats),
	}
	statsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	payload, err := json.Marshal(executorStats)
	require.NoError(t, err)
	writer, err := common.NewRecordWriter(tc.Compression)
	require.NoError(t, err)
	compressedPayload, err := writer.Write(payload)
	require.NoError(t, err)
	_, err = tc.Client.Put(ctx, statsKey, string(compressedPayload))
	require.NoError(t, err)

	req := store.HeartbeatState{
		LastHeartbeat: time.Now().UTC(),
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			shardID: {
				Status:    types.ShardStatusREADY,
				ShardLoad: 45.6,
			},
		},
	}

	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, req))

	nsState, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)

	updated, ok := nsState.ShardStats[shardID]
	require.True(t, ok)
	assert.True(t, updated.LastUpdateTime.After(initialStats.LastUpdateTime))
	expectedLoad := statistics.CalculateSmoothedLoad(initialStats.SmoothedLoad, req.ReportedShards[shardID].ShardLoad, initialStats.LastUpdateTime, updated.LastUpdateTime)
	assert.InDelta(t, expectedLoad, updated.SmoothedLoad, 1e-9)
	assert.Equal(t, initialStats.LastMoveTime, updated.LastMoveTime)
}

func TestRecordHeartbeatSkipsShardStatisticsWithNilReport(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	setLoadBalancingMode(executorStore, config.LoadBalancingModeGREEDY)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "executor-missing-load"
	validShardID := "shard-with-valid-load"
	skippedShardID := "shard-missing-load"

	req := store.HeartbeatState{
		LastHeartbeat: time.Now().UTC(),
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			validShardID: {
				Status:    types.ShardStatusREADY,
				ShardLoad: 3.21,
			},
			skippedShardID: nil,
		},
	}

	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, req))

	nsState, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)

	validStats, ok := nsState.ShardStats[validShardID]
	require.True(t, ok)
	assert.InDelta(t, 3.21, validStats.SmoothedLoad, 1e-9)
	assert.False(t, validStats.LastUpdateTime.IsZero())

	assert.NotContains(t, nsState.ShardStats, skippedShardID)
}

func TestGetHeartbeat(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now().UTC()

	executorID := "executor-get"
	req := store.HeartbeatState{
		Status:        types.ExecutorStatusDRAINING,
		LastHeartbeat: now,
	}

	// 1. Record a heartbeat
	err := executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, req)
	require.NoError(t, err)

	// Assign shards to one executor
	assignState := map[string]store.AssignedState{
		executorID: {
			AssignedShards: map[string]*types.ShardAssignment{
				"shard-1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	require.NoError(t, executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{
		NewState: &store.NamespaceState{
			ShardAssignments: assignState,
		},
	}, store.NopGuard()))

	// 2. Get the heartbeat back
	hb, assignedFromDB, err := executorStore.GetHeartbeat(ctx, tc.Namespace, executorID)
	require.NoError(t, err)
	require.NotNil(t, hb)

	// 3. Verify the state
	assert.Equal(t, types.ExecutorStatusDRAINING, hb.Status)
	assert.Equal(t, now, hb.LastHeartbeat)
	require.NotNil(t, assignedFromDB.AssignedShards)
	assert.Equal(t, assignState[executorID].AssignedShards, assignedFromDB.AssignedShards)

	// 4. Test getting a non-existent executor
	_, _, err = executorStore.GetHeartbeat(ctx, tc.Namespace, "executor-non-existent")
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrExecutorNotFound)
}

// TestGetState verifies that the store can accurately retrieve the state of all executors.
func TestGetState(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID1 := "exec-TestGetState-1"
	executorID2 := "exec-TestGetState-2"
	shardID1 := "shard-1"
	shardID2 := "shard-2"

	// Setup: Record heartbeats and assign shards.
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusDRAINING}))
	require.NoError(t, executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{
		NewState: &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {AssignedShards: map[string]*types.ShardAssignment{shardID1: {}}},
				executorID2: {AssignedShards: map[string]*types.ShardAssignment{shardID2: {}}},
			},
		},
	}, store.NopGuard()))

	// Action: Get the state.
	namespaceState, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)

	// Verification:
	// Check Executors
	require.Len(t, namespaceState.Executors, 2, "Should retrieve two heartbeat states")
	assert.Equal(t, types.ExecutorStatusACTIVE, namespaceState.Executors[executorID1].Status)
	assert.Equal(t, types.ExecutorStatusDRAINING, namespaceState.Executors[executorID2].Status)

	// Check ShardAssignments (from executor records)
	require.Len(t, namespaceState.ShardAssignments, 2, "Should retrieve two assignment states")
	assert.Contains(t, namespaceState.ShardAssignments[executorID1].AssignedShards, shardID1)
	assert.Contains(t, namespaceState.ShardAssignments[executorID2].AssignedShards, shardID2)
}

// TestAssignShards_WithRevisions tests the optimistic locking logic of AssignShards.
func TestAssignShards_WithRevisions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID1 := "exec-rev-1"
	executorID2 := "exec-rev-2"

	t.Run("Success", func(t *testing.T) {
		tc := testhelper.SetupStoreTestCluster(t)
		executorStore := createStore(t, tc)
		recordHeartbeats(ctx, t, executorStore, tc.Namespace, executorID1, executorID2)

		// Define a new state: assign shard1 to exec1
		newState := &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {AssignedShards: map[string]*types.ShardAssignment{"shard-1": {}}},
			},
		}

		// Assign - should succeed
		err := executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: newState}, store.NopGuard())
		require.NoError(t, err)

		// Verify the assignment
		state, err := executorStore.GetState(ctx, tc.Namespace)
		require.NoError(t, err)
		assert.Contains(t, state.ShardAssignments[executorID1].AssignedShards, "shard-1")
	})

	t.Run("ConflictOnNewShard", func(t *testing.T) {
		tc := testhelper.SetupStoreTestCluster(t)
		executorStore := createStore(t, tc)
		recordHeartbeats(ctx, t, executorStore, tc.Namespace, executorID1, executorID2)

		// Process A defines its desired state: assign shard-new to exec1
		processAState := &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {AssignedShards: map[string]*types.ShardAssignment{"shard-new": {}}},
				executorID2: {},
			},
		}

		// Process B defines its desired state: assign shard-new to exec2
		processBState := &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {},
				executorID2: {AssignedShards: map[string]*types.ShardAssignment{"shard-new": {}}},
			},
		}

		// Process A succeeds
		err := executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: processAState}, store.NopGuard())
		require.NoError(t, err)

		// Process B tries to commit, but its revision check for shard-new (rev=0) will fail.
		err = executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: processBState}, store.NopGuard())
		require.Error(t, err)
		assert.ErrorIs(t, err, store.ErrVersionConflict)
	})

	t.Run("ConflictOnExistingShard", func(t *testing.T) {
		tc := testhelper.SetupStoreTestCluster(t)
		executorStore := createStore(t, tc)
		recordHeartbeats(ctx, t, executorStore, tc.Namespace, executorID1, executorID2)

		shardID := "shard-to-move"
		// 1. Setup: Assign the shard to executor1
		setupState, err := executorStore.GetState(ctx, tc.Namespace)
		require.NoError(t, err)
		setupState.ShardAssignments = map[string]store.AssignedState{
			executorID1: {AssignedShards: map[string]*types.ShardAssignment{shardID: {}}},
		}
		require.NoError(t, executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: setupState}, store.NopGuard()))

		// 2. Process A reads the state, intending to move the shard to executor2
		stateForProcA, err := executorStore.GetState(ctx, tc.Namespace)
		require.NoError(t, err)
		stateForProcA.ShardAssignments = map[string]store.AssignedState{
			executorID1: {ModRevision: stateForProcA.ShardAssignments[executorID1].ModRevision},
			executorID2: {AssignedShards: map[string]*types.ShardAssignment{shardID: {}}, ModRevision: 0},
		}

		// 3. In the meantime, another process makes a different change (e.g., re-assigns to same executor, which changes revision)
		intermediateState, err := executorStore.GetState(ctx, tc.Namespace)
		require.NoError(t, err)
		intermediateState.ShardAssignments = map[string]store.AssignedState{
			executorID1: {
				AssignedShards: map[string]*types.ShardAssignment{shardID: {}},
				ModRevision:    intermediateState.ShardAssignments[executorID1].ModRevision,
			},
		}
		require.NoError(t, executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: intermediateState}, store.NopGuard()))

		// 4. Process A tries to commit its change. It will fail because its stored revision for the shard is now stale.
		err = executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: stateForProcA}, store.NopGuard())
		require.Error(t, err)
		assert.ErrorIs(t, err, store.ErrVersionConflict)
	})

	t.Run("NoChanges", func(t *testing.T) {
		tc := testhelper.SetupStoreTestCluster(t)
		executorStore := createStore(t, tc)
		recordHeartbeats(ctx, t, executorStore, tc.Namespace, executorID1, executorID2)

		// Get the current state
		state, err := executorStore.GetState(ctx, tc.Namespace)
		require.NoError(t, err)

		// Call AssignShards with the same assignments
		err = executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: state}, store.NopGuard())
		require.NoError(t, err, "Assigning with no changes should succeed")
	})
}

// TestGuardedOperations verifies that AssignShards and DeleteExecutors respect the leader guard.
func TestGuardedOperations(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	namespace := "test-guarded-ns"
	executorID := "exec-to-delete"

	// 1. Create two potential leaders
	// FIX: Use the correct constructor for the leader elector.
	elector, err := leaderstore.NewLeaderStore(leaderstore.StoreParams{Client: tc.Client, Cfg: tc.LeaderCfg, Lifecycle: fxtest.NewLifecycle(t)})
	require.NoError(t, err)
	election1, err := elector.CreateElection(ctx, namespace)
	defer election1.Cleanup(ctx)
	defer func() { _ = election1.Cleanup(ctx) }()
	election2, err := elector.CreateElection(ctx, namespace)
	defer election2.Cleanup(ctx)
	defer func() { _ = election2.Cleanup(ctx) }()

	// 2. First node becomes leader
	require.NoError(t, election1.Campaign(ctx, "host-1"))
	validGuard := election1.Guard()

	// 3. Use the valid guard to assign shards - should succeed
	assignState := map[string]store.AssignedState{"exec-1": {}}
	err = executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: &store.NamespaceState{ShardAssignments: assignState}}, validGuard)
	require.NoError(t, err, "Assigning shards with a valid leader guard should succeed")

	// 4. First node resigns, second node becomes leader
	require.NoError(t, election1.Resign(ctx))
	require.NoError(t, election2.Campaign(ctx, "host-2"))

	// 5. Use the now-invalid guard from the first leader - should fail
	err = executorStore.AssignShards(ctx, tc.Namespace, store.AssignShardsRequest{NewState: &store.NamespaceState{ShardAssignments: assignState}}, validGuard)
	require.Error(t, err, "Assigning shards with a stale leader guard should fail")

	// 6. Use the NopGuard to delete an executor - should succeed
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	err = executorStore.DeleteExecutors(ctx, tc.Namespace, []string{executorID}, store.NopGuard())
	require.NoError(t, err, "Deleting an executor without a guard should succeed")

	// Verify deletion
	newState, err := executorStore.GetState(ctx, namespace)
	require.NoError(t, err)
	_, ok := newState.ShardAssignments[executorID]
	require.False(t, ok, "Executor should have been deleted")
}

// TestSubscribe verifies that the subscription channel receives notifications for significant changes.
func TestSubscribe(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "exec-sub"

	// Start subscription
	sub, err := executorStore.Subscribe(ctx, tc.Namespace)
	require.NoError(t, err)

	// Manually put a heartbeat update, which is an insignificant change
	heartbeatKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, "heartbeat")
	_, err = tc.Client.Put(ctx, heartbeatKey, "timestamp")
	require.NoError(t, err)

	select {
	case <-sub:
		t.Fatal("Should not receive notification for a heartbeat-only update")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior
	}

	// Now update the reported shards, which IS a significant change
	reportedShardsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, "reported_shards")
	require.NoError(t, err)
	writer, err := common.NewRecordWriter(tc.Compression)
	require.NoError(t, err)
	compressedShards, err := writer.Write([]byte(`{"shard-1":{"status":"running"}}`))
	require.NoError(t, err)
	_, err = tc.Client.Put(ctx, reportedShardsKey, string(compressedShards))
	require.NoError(t, err)

	select {
	case rev, ok := <-sub:
		require.True(t, ok, "Channel should be open")
		assert.Greater(t, rev, int64(0), "Should receive a valid revision for reported shards change")
	case <-time.After(1 * time.Second):
		t.Fatal("Should have received a notification for a reported shards change")
	}
}

func TestDeleteExecutors_Empty(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := executorStore.DeleteExecutors(ctx, tc.Namespace, []string{}, store.NopGuard())
	require.NoError(t, err)
}

// TestDeleteExecutors covers various scenarios for the DeleteExecutors method.
func TestDeleteExecutors(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup: Create two active executors for the tests.
	executorID1 := "executor-to-delete-1"
	executorID2 := "executor-to-delete-2"
	survivingExecutorID := "executor-survivor"
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, survivingExecutorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

	t.Run("SucceedsForNonExistentExecutor", func(t *testing.T) {
		// Action: Delete a non-existent executor.
		err := executorStore.DeleteExecutors(ctx, tc.Namespace, []string{"non-existent-executor"}, store.NopGuard())
		// Verification: Should not return an error.
		require.NoError(t, err)
	})

	t.Run("DeletesMultipleExecutors", func(t *testing.T) {
		// Setup: Create and assign shards to multiple executors.
		execToDelete1 := "multi-delete-1"
		execToDelete2 := "multi-delete-2"
		execToKeep := "multi-keep-1"
		shardOfDeletedExecutor1 := "multi-shard-1"
		shardOfDeletedExecutor2 := "multi-shard-2"
		shardOfSurvivingExecutor := "multi-shard-keep"

		require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, execToDelete1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
		require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, execToDelete2, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
		require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, execToKeep, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

		require.NoError(t, executorStore.AssignShard(ctx, tc.Namespace, shardOfDeletedExecutor1, execToDelete1))
		require.NoError(t, executorStore.AssignShard(ctx, tc.Namespace, shardOfDeletedExecutor2, execToDelete2))
		require.NoError(t, executorStore.AssignShard(ctx, tc.Namespace, shardOfSurvivingExecutor, execToKeep))

		// Action: Delete two of the three executors in one call.
		err := executorStore.DeleteExecutors(ctx, tc.Namespace, []string{execToDelete1, execToDelete2}, store.NopGuard())
		require.NoError(t, err)

		// Verification:
		// 1. Check deleted executors are gone.
		_, _, err = executorStore.GetHeartbeat(ctx, tc.Namespace, execToDelete1)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor 1 should be gone")

		_, _, err = executorStore.GetHeartbeat(ctx, tc.Namespace, execToDelete2)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor 2 should be gone")

		// 2. Check that the surviving executor remain.
		_, _, err = executorStore.GetHeartbeat(ctx, tc.Namespace, execToKeep)
		assert.NoError(t, err, "Surviving executor should still exist")
	})
}

func TestParseExecutorKey_Errors(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)

	_, _, err := etcdkeys.ParseExecutorKey(tc.EtcdPrefix, tc.Namespace, "/wrong/prefix/exec/heartbeat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have expected prefix")

	key := etcdkeys.BuildExecutorsPrefix(tc.EtcdPrefix, tc.Namespace) + "too/many/parts"
	_, _, err = etcdkeys.ParseExecutorKey(tc.EtcdPrefix, tc.Namespace, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected key format")
}

// TestAssignAndGetShardOwnerRoundtrip verifies the successful assignment and retrieval of a shard owner.
func TestAssignAndGetShardOwnerRoundtrip(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc).(*executorStoreImpl)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := executorStore.timeSource.Now().UTC()
	executorID := "executor-roundtrip"
	shardID := "shard-roundtrip"

	// Setup: Create an active executor.
	err := executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)

	// 1. Assign a shard to the active executor.
	err = executorStore.AssignShard(ctx, tc.Namespace, shardID, executorID)
	require.NoError(t, err, "Should successfully assign shard to an active executor")

	// 2. Get the owner and verify it's the correct executor.
	state, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)
	assert.Contains(t, state.ShardAssignments[executorID].AssignedShards, shardID)
	assert.Equal(t, now, state.ShardAssignments[executorID].LastUpdated)
}

// TestAssignShardErrors tests the various error conditions when assigning a shard.
func TestAssignShardErrors(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	activeExecutorID := "executor-active-errors"
	drainingExecutorID := "executor-draining-errors"
	shardID1 := "shard-err-1"
	shardID2 := "shard-err-2"

	// Setup: Create an active and a draining executor, and assign one shard.
	err := executorStore.RecordHeartbeat(ctx, tc.Namespace, activeExecutorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)
	err = executorStore.RecordHeartbeat(ctx, tc.Namespace, drainingExecutorID, store.HeartbeatState{Status: types.ExecutorStatusDRAINING})
	require.NoError(t, err)
	err = executorStore.AssignShard(ctx, tc.Namespace, shardID1, activeExecutorID)
	require.NoError(t, err)

	// Case 1: Assigning an already-assigned shard.
	err = executorStore.AssignShard(ctx, tc.Namespace, shardID1, activeExecutorID)
	require.Error(t, err, "Should fail to assign an already-assigned shard")
	assert.ErrorAs(t, err, new(*store.ErrShardAlreadyAssigned))

	// Case 2: Assigning to a non-existent executor.
	err = executorStore.AssignShard(ctx, tc.Namespace, shardID2, "non-existent-executor")
	require.Error(t, err, "Should fail to assign to a non-existent executor")
	assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Error should be ErrExecutorNotFound")

	// Case 3: Assigning to a non-active (draining) executor.
	err = executorStore.AssignShard(ctx, tc.Namespace, shardID2, drainingExecutorID)
	require.Error(t, err, "Should fail to assign to a draining executor")
	assert.ErrorIs(t, err, store.ErrVersionConflict, "Error should be ErrVersionConflict for non-active executor")
}

// TestShardStatisticsPersistence verifies that shard statistics are preserved on assignment
// when they already exist, and that GetState exposes them.
func TestShardStatisticsPersistence(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	executorStore.(*executorStoreImpl).cfg = &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return config.LoadBalancingModeGREEDY
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "exec-stats"
	shardID := "shard-stats"

	// 1. Setup: ensure executor is ACTIVE
	require.NoError(t, executorStore.RecordHeartbeat(ctx, tc.Namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

	// 2. Pre-create shard statistics as if coming from prior history
	stats := store.ShardStatistics{SmoothedLoad: 12.5, LastUpdateTime: time.Unix(1234, 0).UTC(), LastMoveTime: time.Unix(5678, 0).UTC()}
	// The stats the executor should have after assignment
	executorStats := map[string]etcdtypes.ShardStatistics{
		shardID: *etcdtypes.FromShardStatistics(&stats),
	}
	payload, err := json.Marshal(executorStats)
	require.NoError(t, err)
	statsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	writer, err := common.NewRecordWriter(tc.Compression)
	require.NoError(t, err)
	compressedPayload, err := writer.Write(payload)
	require.NoError(t, err)
	// Write those stats to etcd under the executor (exec-stats) stats key
	_, err = tc.Client.Put(ctx, statsKey, string(compressedPayload))
	require.NoError(t, err)

	// 3. Assign the shard via AssignShard (should not clobber existing metrics)
	require.NoError(t, executorStore.AssignShard(ctx, tc.Namespace, shardID, executorID))

	// 4. Verify via GetState that metrics are preserved and exposed
	nsState, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)
	require.Contains(t, nsState.ShardStats, shardID)
	updatedStats := nsState.ShardStats[shardID]
	assert.Equal(t, stats.SmoothedLoad, updatedStats.SmoothedLoad)
	assert.Equal(t, stats.LastUpdateTime, updatedStats.LastUpdateTime)
	// This should be greater than the last move time
	assert.Greater(t, updatedStats.LastMoveTime, stats.LastMoveTime)

	// 5. Also ensure assignment recorded correctly
	require.Contains(t, nsState.ShardAssignments[executorID].AssignedShards, shardID)
}

// TestGetShardStatisticsForMissingShard verifies GetState does not report statistics for unknown shards.
func TestGetShardStatisticsForMissingShard(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// No metrics are written; GetState should not contain unknown shard
	st, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)
	assert.NotContains(t, st.ShardStats, "unknown")
}

// TestDeleteShardStatsDeletesAllStats verifies that shard statistics are correctly deleted.
func TestDeleteShardStatsDeletesAllStats(t *testing.T) {
	tc := testhelper.SetupStoreTestCluster(t)
	executorStore := createStore(t, tc)
	setLoadBalancingMode(executorStore, config.LoadBalancingModeGREEDY)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	totalShardStats := 135 // number of stats to add and make stale
	shardIDs := make([]string, 0, totalShardStats)
	executorID := "exec-delete-stats"

	// ensure executor exists
	ctxHeartbeat, cancelHb := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelHb()
	require.NoError(t, executorStore.RecordHeartbeat(ctxHeartbeat, tc.Namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

	// Create stale stats
	executorStats := make(map[string]etcdtypes.ShardStatistics)
	for i := 0; i < totalShardStats; i++ {
		shardID := "stale-stats-" + strconv.Itoa(i)
		shardIDs = append(shardIDs, shardID)

		stats := store.ShardStatistics{
			SmoothedLoad:   float64(i),
			LastUpdateTime: time.Unix(int64(i), 0).UTC(),
			LastMoveTime:   time.Unix(int64(i), 0).UTC(),
		}
		executorStats[shardID] = *etcdtypes.FromShardStatistics(&stats)
	}

	statsKey := etcdkeys.BuildExecutorKey(tc.EtcdPrefix, tc.Namespace, executorID, etcdkeys.ExecutorShardStatisticsKey)
	payload, err := json.Marshal(executorStats)
	require.NoError(t, err)
	writer, err := common.NewRecordWriter(tc.Compression)
	require.NoError(t, err)
	compressedPayload, err := writer.Write(payload)
	require.NoError(t, err)
	_, err = tc.Client.Put(ctx, statsKey, string(compressedPayload))
	require.NoError(t, err)

	require.NoError(t, executorStore.DeleteShardStats(ctx, tc.Namespace, shardIDs, store.NopGuard()))

	nsState, err := executorStore.GetState(ctx, tc.Namespace)
	require.NoError(t, err)
	// All stats should be deleted
	assert.Empty(t, nsState.ShardStats)
}

// --- Test Setup ---

func stringStatus(s types.ExecutorStatus) string {
	res, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}

func recordHeartbeats(ctx context.Context, t *testing.T, executorStore store.Store, namespace string, executorIDs ...string) {
	t.Helper()

	for _, executorID := range executorIDs {
		require.NoError(t, executorStore.RecordHeartbeat(ctx, namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	}
}

func setLoadBalancingMode(executorStore store.Store, mode string) {
	executorStore.(*executorStoreImpl).cfg = &config.Config{
		LoadBalancingMode: func(string) string { return mode },
	}
}

func createStore(t *testing.T, tc *testhelper.StoreTestCluster) store.Store {
	t.Helper()

	etcdConfig, err := NewETCDConfig(tc.LeaderCfg)
	require.NoError(t, err)

	store, err := NewStore(ExecutorStoreParams{
		Client:     tc.Client,
		ETCDConfig: etcdConfig,
		Lifecycle:  fxtest.NewLifecycle(t),
		Logger:     testlogger.New(t),
		TimeSource: clock.NewMockedTimeSourceAt(time.Now()),
		Config: &config.Config{
			LoadBalancingMode: func(namespace string) string { return config.LoadBalancingModeNAIVE },
		},
	})
	require.NoError(t, err)
	return store
}
