package etcd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx/fxtest"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/types"
	shardDistributorCfg "github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/testflags"
)

// TestRecordHeartbeat verifies that an executor's heartbeat is correctly stored.
func TestRecordHeartbeat(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nowTS := time.Now().Unix()

	namespace := "test-heartbeat-ns"
	executorID := "executor-TestRecordHeartbeat"
	req := store.HeartbeatState{
		LastHeartbeat: nowTS,
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			"shard-TestRecordHeartbeat": {Status: types.ShardStatusREADY},
		},
	}

	err := tc.store.RecordHeartbeat(ctx, namespace, executorID, req)
	require.NoError(t, err)

	// Verify directly in etcd
	heartbeatKey := tc.store.buildExecutorKey(namespace, executorID, executorHeartbeatKey)
	stateKey := tc.store.buildExecutorKey(namespace, executorID, executorStatusKey)
	reportedShardsKey := tc.store.buildExecutorKey(namespace, executorID, executorReportedShardsKey)

	resp, err := tc.client.Get(ctx, heartbeatKey)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Count, "Heartbeat key should exist")
	assert.Equal(t, strconv.FormatInt(nowTS, 10), string(resp.Kvs[0].Value))

	resp, err = tc.client.Get(ctx, stateKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "State key should exist")
	assert.Equal(t, stringStatus(types.ExecutorStatusACTIVE), string(resp.Kvs[0].Value))

	resp, err = tc.client.Get(ctx, reportedShardsKey)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.Count, "Reported shards key should exist")

	var reportedShards map[string]*types.ShardStatusReport
	err = json.Unmarshal(resp.Kvs[0].Value, &reportedShards)
	require.NoError(t, err)
	require.Len(t, reportedShards, 1)
	assert.Equal(t, types.ShardStatusREADY, reportedShards["shard-TestRecordHeartbeat"].Status)
}

func TestGetHeartbeat(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nowTS := time.Now().Unix()

	namespace := "test-get-heartbeat-ns"
	executorID := "executor-get"
	req := store.HeartbeatState{
		Status:        types.ExecutorStatusDRAINING,
		LastHeartbeat: nowTS,
	}

	// 1. Record a heartbeat
	err := tc.store.RecordHeartbeat(ctx, namespace, executorID, req)
	require.NoError(t, err)

	// Assign shards to one executor
	assignState := map[string]store.AssignedState{
		executorID: {
			AssignedShards: map[string]*types.ShardAssignment{
				"shard-1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	require.NoError(t, tc.store.AssignShards(ctx, namespace, assignState, store.NopGuard()))

	// 2. Get the heartbeat back
	hb, assignedFromDB, err := tc.store.GetHeartbeat(ctx, namespace, executorID)
	require.NoError(t, err)
	require.NotNil(t, hb)

	// 3. Verify the state
	assert.Equal(t, types.ExecutorStatusDRAINING, hb.Status)
	assert.Equal(t, nowTS, hb.LastHeartbeat)
	require.NotNil(t, assignedFromDB.AssignedShards)
	assert.Equal(t, assignState[executorID].AssignedShards, assignedFromDB.AssignedShards)

	// 4. Test getting a non-existent executor
	_, _, err = tc.store.GetHeartbeat(ctx, namespace, "executor-non-existent")
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrExecutorNotFound)
}

// TestGetState verifies that the store can accurately retrieve the state of all executors.
func TestGetState(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-getstate-ns"
	executorID1 := "exec-TestGetState"
	// Record two heartbeats, one with reported shards
	require.NoError(t, tc.store.RecordHeartbeat(ctx, namespace, executorID1, store.HeartbeatState{
		Status: types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			"shard-10": {Status: types.ShardStatusREADY},
		},
	}))
	executorID2 := "exec-TestGetState-2"
	require.NoError(t, tc.store.RecordHeartbeat(ctx, namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusDRAINING}))

	// Assign shards to one executor
	assignState := map[string]store.AssignedState{
		executorID1: {
			AssignedShards: map[string]*types.ShardAssignment{
				"shard-1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	require.NoError(t, tc.store.AssignShards(ctx, namespace, assignState, store.NopGuard()))

	// Get the state
	heartbeats, assignments, _, err := tc.store.GetState(ctx, namespace)
	require.NoError(t, err)

	require.Len(t, heartbeats, 2, "Should retrieve two heartbeat states")
	assert.Equal(t, types.ExecutorStatusACTIVE, heartbeats[executorID1].Status)
	assert.Equal(t, types.ExecutorStatusDRAINING, heartbeats[executorID2].Status)

	require.Len(t, assignments, 2, "Should retrieve two assignment states")
	require.Len(t, assignments[executorID1].AssignedShards, 1, "Executor 1 should have one shard assigned")
	assert.Contains(t, assignments[executorID1].AssignedShards, "shard-1")
	require.Len(t, assignments[executorID2].AssignedShards, 0, "Executor 2 should have no shards assigned")

	// Verify reported shards from heartbeat were also retrieved
	require.Len(t, heartbeats[executorID1].ReportedShards, 1, "Executor 1 should have one shard reported")
	assert.Equal(t, types.ShardStatusREADY, heartbeats[executorID1].ReportedShards["shard-10"].Status)
	require.Len(t, heartbeats[executorID2].ReportedShards, 0, "Executor 2 should have no shards reported")
}

// TestGuardedOperations verifies that AssignShards and DeleteExecutors respect the leader guard.
func TestGuardedOperations(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	namespace := "test-guarded-ns"
	executorID := "exec-to-delete"

	// 1. Create two potential leaders
	// FIX: Use the correct constructor for the leader elector.
	elector, err := NewLeaderStore(StoreParams{Client: tc.client, Cfg: tc.leaderCfg, Lifecycle: tc.lifecycle})
	require.NoError(t, err)
	election1, err := elector.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election1.Cleanup(ctx)
	election2, err := elector.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election2.Cleanup(ctx)

	// 2. First node becomes leader
	require.NoError(t, election1.Campaign(ctx, "host-1"))
	validGuard := election1.Guard()

	// 3. Use the valid guard to assign shards - should succeed
	assignState := map[string]store.AssignedState{"exec-1": {}}
	err = tc.store.AssignShards(ctx, namespace, assignState, validGuard)
	require.NoError(t, err, "Assigning shards with a valid leader guard should succeed")

	// 4. First node resigns, second node becomes leader
	require.NoError(t, election1.Resign(ctx))
	require.NoError(t, election2.Campaign(ctx, "host-2"))

	// 5. Use the now-invalid guard from the first leader - should fail
	err = tc.store.AssignShards(ctx, namespace, assignState, validGuard)
	require.Error(t, err, "Assigning shards with a stale leader guard should fail")

	// 6. Use the NopGuard to delete an executor - should succeed
	require.NoError(t, tc.store.RecordHeartbeat(ctx, namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	err = tc.store.DeleteExecutors(ctx, namespace, []string{executorID}, store.NopGuard())
	require.NoError(t, err, "Deleting an executor without a guard should succeed")

	// Verify deletion
	_, assignments, _, err := tc.store.GetState(ctx, namespace)
	require.NoError(t, err)
	_, ok := assignments[executorID]
	require.False(t, ok, "Executor should have been deleted")
}

// TestSubscribe verifies that the subscription channel receives notifications for significant changes.
func TestSubscribe(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-subscribe-ns"
	executorID := "exec-sub"

	// Start subscription
	sub, err := tc.store.Subscribe(ctx, namespace)
	require.NoError(t, err)

	// Manually put a heartbeat update, which is an insignificant change
	heartbeatKey := tc.store.buildExecutorKey(namespace, executorID, "heartbeat")
	_, err = tc.client.Put(ctx, heartbeatKey, "timestamp")
	require.NoError(t, err)

	select {
	case <-sub:
		t.Fatal("Should not receive notification for a heartbeat-only update")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior
	}

	// Now update the reported shards, which IS a significant change
	reportedShardsKey := tc.store.buildExecutorKey(namespace, executorID, "reported_shards")
	_, err = tc.client.Put(ctx, reportedShardsKey, `{"shard-1":{"status":"running"}}`)
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
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	namespace := "test-delete-empty"

	err := tc.store.DeleteExecutors(ctx, namespace, []string{}, store.NopGuard())
	require.NoError(t, err)
}

func TestParseExecutorKey_Errors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	namespace := "test-parsing-ns"

	_, _, err := tc.store.parseExecutorKey(namespace, "/wrong/prefix/exec/heartbeat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have expected prefix")

	key := tc.store.buildExecutorPrefix(namespace) + "too/many/parts"
	_, _, err = tc.store.parseExecutorKey(namespace, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected key format")
}

// TestAssignAndGetShardOwnerRoundtrip verifies the successful assignment and retrieval of a shard owner.
func TestAssignAndGetShardOwnerRoundtrip(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-roundtrip-ns"
	executorID := "executor-roundtrip"
	shardID := "shard-roundtrip"

	// Setup: Create an active executor.
	err := tc.store.RecordHeartbeat(ctx, namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)

	// 1. Assign a shard to the active executor.
	err = tc.store.AssignShard(ctx, namespace, shardID, executorID)
	require.NoError(t, err, "Should successfully assign shard to an active executor")

	// 2. Get the owner and verify it's the correct executor.
	owner, err := tc.store.GetShardOwner(ctx, namespace, shardID)
	require.NoError(t, err, "Should successfully get the shard owner after assignment")
	assert.Equal(t, executorID, owner, "Owner should be the executor it was assigned to")
}

// TestAssignShardErrors tests the various error conditions when assigning a shard.
func TestAssignShardErrors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-assign-errors-ns"
	activeExecutorID := "executor-active-errors"
	drainingExecutorID := "executor-draining-errors"
	shardID1 := "shard-err-1"
	shardID2 := "shard-err-2"

	// Setup: Create an active and a draining executor, and assign one shard.
	err := tc.store.RecordHeartbeat(ctx, namespace, activeExecutorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)
	err = tc.store.RecordHeartbeat(ctx, namespace, drainingExecutorID, store.HeartbeatState{Status: types.ExecutorStatusDRAINING})
	require.NoError(t, err)
	err = tc.store.AssignShard(ctx, namespace, shardID1, activeExecutorID)
	require.NoError(t, err)

	// Case 1: Assigning an already-assigned shard.
	err = tc.store.AssignShard(ctx, namespace, shardID1, activeExecutorID)
	require.Error(t, err, "Should fail to assign an already-assigned shard")
	assert.ErrorIs(t, err, store.ErrVersionConflict, "Error should be ErrVersionConflict for duplicate assignment")

	// Case 2: Assigning to a non-existent executor.
	err = tc.store.AssignShard(ctx, namespace, shardID2, "non-existent-executor")
	require.Error(t, err, "Should fail to assign to a non-existent executor")
	assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Error should be ErrExecutorNotFound")

	// Case 3: Assigning to a non-active (draining) executor.
	err = tc.store.AssignShard(ctx, namespace, shardID2, drainingExecutorID)
	require.Error(t, err, "Should fail to assign to a draining executor")
	assert.ErrorIs(t, err, store.ErrVersionConflict, "Error should be ErrVersionConflict for non-active executor")
}

// TestGetShardOwnerErrors tests error conditions for getting a shard owner.
func TestGetShardOwnerErrors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-get-owner-errors-ns"

	// Try to get the owner of a shard that has not been assigned.
	_, err := tc.store.GetShardOwner(ctx, namespace, "non-existent-shard")
	require.Error(t, err, "Should return an error for a non-existent shard")
	assert.ErrorIs(t, err, store.ErrShardNotFound, "Error should be ErrShardNotFound")
}

// --- Test Setup ---

type storeTestCluster struct {
	store     *Store // Use concrete type to access buildExecutorKey
	leaderCfg shardDistributorCfg.LeaderElection
	client    *clientv3.Client
	lifecycle *fxtest.Lifecycle
}

func setupStoreTestCluster(t *testing.T) *storeTestCluster {
	t.Helper()
	flag.Parse()
	testflags.RequireEtcd(t)

	endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
	if endpoints == nil || len(endpoints) == 0 || endpoints[0] == "" {
		endpoints = []string{"localhost:2379"}
	}
	t.Logf("ETCD endpoints: %v", endpoints)

	etcdConfigRaw := map[string]interface{}{
		"endpoints":   endpoints,
		"dialTimeout": "5s",
		"prefix":      fmt.Sprintf("/test-shard-store/%s", t.Name()),
		"electionTTL": "5s", // Needed for leader config part
	}

	yamlCfg, err := yaml.Marshal(etcdConfigRaw)
	require.NoError(t, err)
	var yamlNode *config.YamlNode
	err = yaml.Unmarshal(yamlCfg, &yamlNode)
	require.NoError(t, err)

	leaderCfg := shardDistributorCfg.LeaderElection{
		Enabled:     true,
		Store:       shardDistributorCfg.Store{StorageParams: yamlNode},
		LeaderStore: shardDistributorCfg.Store{StorageParams: yamlNode},
	}

	lifecycle := fxtest.NewLifecycle(t)
	s, err := NewStore(StoreParams{
		Cfg:       leaderCfg,
		Lifecycle: lifecycle,
	})
	require.NoError(t, err)

	client, err := clientv3.New(clientv3.Config{Endpoints: endpoints, DialTimeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return &storeTestCluster{
		store:     s.(*Store),
		leaderCfg: leaderCfg,
		client:    client,
		lifecycle: lifecycle,
	}
}

func stringStatus(s types.ExecutorStatus) string {
	res, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(res)
}
