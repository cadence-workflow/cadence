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

	executorID := "executor-TestRecordHeartbeat"
	req := store.HeartbeatState{
		LastHeartbeat: nowTS,
		Status:        types.ExecutorStatusACTIVE,
		ReportedShards: map[string]*types.ShardStatusReport{
			"shard-TestRecordHeartbeat": {Status: types.ShardStatusREADY},
		},
	}

	err := tc.store.RecordHeartbeat(ctx, tc.namespace, executorID, req)
	require.NoError(t, err)

	// Verify directly in etcd
	heartbeatKey := tc.store.buildExecutorKey(tc.namespace, executorID, executorHeartbeatKey)
	stateKey := tc.store.buildExecutorKey(tc.namespace, executorID, executorStatusKey)
	reportedShardsKey := tc.store.buildExecutorKey(tc.namespace, executorID, executorReportedShardsKey)

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

	executorID := "executor-get"
	req := store.HeartbeatState{
		Status:        types.ExecutorStatusDRAINING,
		LastHeartbeat: nowTS,
	}

	// 1. Record a heartbeat
	err := tc.store.RecordHeartbeat(ctx, tc.namespace, executorID, req)
	require.NoError(t, err)

	// Assign shards to one executor
	assignState := map[string]store.AssignedState{
		executorID: {
			AssignedShards: map[string]*types.ShardAssignment{
				"shard-1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	require.NoError(t, tc.store.AssignShards(ctx, tc.namespace, &store.NamespaceState{
		ShardAssignments: assignState,
	}, store.NopGuard()))

	// 2. Get the heartbeat back
	hb, assignedFromDB, err := tc.store.GetHeartbeat(ctx, tc.namespace, executorID)
	require.NoError(t, err)
	require.NotNil(t, hb)

	// 3. Verify the state
	assert.Equal(t, types.ExecutorStatusDRAINING, hb.Status)
	assert.Equal(t, nowTS, hb.LastHeartbeat)
	require.NotNil(t, assignedFromDB.AssignedShards)
	assert.Equal(t, assignState[executorID].AssignedShards, assignedFromDB.AssignedShards)

	// 4. Test getting a non-existent executor
	_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, "executor-non-existent")
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrExecutorNotFound)
}

// TestGetState verifies that the store can accurately retrieve the state of all executors.
func TestGetState(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID1 := "exec-TestGetState-1"
	executorID2 := "exec-TestGetState-2"
	shardID1 := "shard-1"
	shardID2 := "shard-2"

	// Setup: Record heartbeats and assign shards.
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusDRAINING}))
	require.NoError(t, tc.store.AssignShards(ctx, tc.namespace, &store.NamespaceState{
		ShardAssignments: map[string]store.AssignedState{
			executorID1: {AssignedShards: map[string]*types.ShardAssignment{shardID1: {}}},
			executorID2: {AssignedShards: map[string]*types.ShardAssignment{shardID2: {}}},
		},
	}, store.NopGuard()))

	// Action: Get the state.
	namespaceState, err := tc.store.GetState(ctx, tc.namespace)
	require.NoError(t, err)

	// Verification:
	// 1. Check Executors
	require.Len(t, namespaceState.Executors, 2, "Should retrieve two heartbeat states")
	assert.Equal(t, types.ExecutorStatusACTIVE, namespaceState.Executors[executorID1].Status)
	assert.Equal(t, types.ExecutorStatusDRAINING, namespaceState.Executors[executorID2].Status)

	// 2. Check Shard ownership and revisions
	require.Len(t, namespaceState.Shards, 2, "Should retrieve two shard states")
	assert.Equal(t, executorID1, namespaceState.Shards[shardID1].ExecutorID)
	assert.Greater(t, namespaceState.Shards[shardID1].Revision, int64(0))
	assert.Equal(t, executorID2, namespaceState.Shards[shardID2].ExecutorID)
	assert.Greater(t, namespaceState.Shards[shardID2].Revision, int64(0))

	// 3. Check ShardAssignments (from executor records)
	require.Len(t, namespaceState.ShardAssignments, 2, "Should retrieve two assignment states")
	assert.Contains(t, namespaceState.ShardAssignments[executorID1].AssignedShards, shardID1)
	assert.Contains(t, namespaceState.ShardAssignments[executorID2].AssignedShards, shardID2)
}

// TestAssignShards_WithRevisions tests the optimistic locking logic of AssignShards.
func TestAssignShards_WithRevisions(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID1 := "exec-rev-1"
	executorID2 := "exec-rev-2"
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

	t.Run("Success", func(t *testing.T) {
		// 1. Get initial state (empty)
		initialState, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)

		// 2. Define a new state: assign shard1 to exec1
		newState := &store.NamespaceState{
			Shards: initialState.Shards, // Pass the read revisions
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {AssignedShards: map[string]*types.ShardAssignment{"shard-1": {}}},
			},
		}

		// 3. Assign - should succeed
		err = tc.store.AssignShards(ctx, tc.namespace, newState, store.NopGuard())
		require.NoError(t, err)

		// 4. Verify the assignment
		owner, err := tc.store.GetShardOwner(ctx, tc.namespace, "shard-1")
		require.NoError(t, err)
		assert.Equal(t, executorID1, owner)
	})

	t.Run("ConflictOnNewShard", func(t *testing.T) {
		// 1. Get initial state
		initialState, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)

		// 2. Process A defines its desired state: assign shard-new to exec1
		processAState := &store.NamespaceState{
			Shards: initialState.Shards,
			ShardAssignments: map[string]store.AssignedState{
				executorID1: {AssignedShards: map[string]*types.ShardAssignment{"shard-new": {}}},
			},
		}

		// 3. Process B defines its desired state: assign shard-new to exec2
		processBState := &store.NamespaceState{
			Shards: initialState.Shards,
			ShardAssignments: map[string]store.AssignedState{
				executorID2: {AssignedShards: map[string]*types.ShardAssignment{"shard-new": {}}},
			},
		}

		// 4. Process A succeeds
		err = tc.store.AssignShards(ctx, tc.namespace, processAState, store.NopGuard())
		require.NoError(t, err)

		// 5. Process B tries to commit, but its revision check for shard-new (rev=0) will fail.
		err = tc.store.AssignShards(ctx, tc.namespace, processBState, store.NopGuard())
		require.Error(t, err)
		assert.ErrorIs(t, err, store.ErrVersionConflict)
	})

	t.Run("ConflictOnExistingShard", func(t *testing.T) {
		shardID := "shard-to-move"
		// 1. Setup: Assign the shard to executor1
		setupState, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)
		setupState.ShardAssignments = map[string]store.AssignedState{
			executorID1: {AssignedShards: map[string]*types.ShardAssignment{shardID: {}}},
		}
		require.NoError(t, tc.store.AssignShards(ctx, tc.namespace, setupState, store.NopGuard()))

		// 2. Process A reads the state, intending to move the shard to executor2
		stateForProcA, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)
		stateForProcA.ShardAssignments = map[string]store.AssignedState{
			executorID2: {AssignedShards: map[string]*types.ShardAssignment{shardID: {}}},
		}

		// 3. In the meantime, another process makes a different change (e.g., re-assigns to same executor, which changes revision)
		intermediateState, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)
		intermediateState.ShardAssignments = map[string]store.AssignedState{
			executorID1: {AssignedShards: map[string]*types.ShardAssignment{shardID: {}}},
		}
		require.NoError(t, tc.store.AssignShards(ctx, tc.namespace, intermediateState, store.NopGuard()))

		// 4. Process A tries to commit its change. It will fail because its stored revision for the shard is now stale.
		err = tc.store.AssignShards(ctx, tc.namespace, stateForProcA, store.NopGuard())
		require.Error(t, err)
		assert.ErrorIs(t, err, store.ErrVersionConflict)
	})

	t.Run("NoChanges", func(t *testing.T) {
		// Get the current state
		state, err := tc.store.GetState(ctx, tc.namespace)
		require.NoError(t, err)

		// Call AssignShards with the same assignments
		err = tc.store.AssignShards(ctx, tc.namespace, state, store.NopGuard())
		require.NoError(t, err, "Assigning with no changes should succeed")
	})
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
	err = tc.store.AssignShards(ctx, tc.namespace, &store.NamespaceState{ShardAssignments: assignState}, validGuard)
	require.NoError(t, err, "Assigning shards with a valid leader guard should succeed")

	// 4. First node resigns, second node becomes leader
	require.NoError(t, election1.Resign(ctx))
	require.NoError(t, election2.Campaign(ctx, "host-2"))

	// 5. Use the now-invalid guard from the first leader - should fail
	err = tc.store.AssignShards(ctx, tc.namespace, &store.NamespaceState{ShardAssignments: assignState}, validGuard)
	require.Error(t, err, "Assigning shards with a stale leader guard should fail")

	// 6. Use the NopGuard to delete an executor - should succeed
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	err = tc.store.DeleteExecutors(ctx, tc.namespace, []string{executorID}, store.NopGuard())
	require.NoError(t, err, "Deleting an executor without a guard should succeed")

	// Verify deletion
	newState, err := tc.store.GetState(ctx, namespace)
	require.NoError(t, err)
	_, ok := newState.ShardAssignments[executorID]
	require.False(t, ok, "Executor should have been deleted")
}

// TestSubscribe verifies that the subscription channel receives notifications for significant changes.
func TestSubscribe(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "exec-sub"

	// Start subscription
	sub, err := tc.store.Subscribe(ctx, tc.namespace)
	require.NoError(t, err)

	// Manually put a heartbeat update, which is an insignificant change
	heartbeatKey := tc.store.buildExecutorKey(tc.namespace, executorID, "heartbeat")
	_, err = tc.client.Put(ctx, heartbeatKey, "timestamp")
	require.NoError(t, err)

	select {
	case <-sub:
		t.Fatal("Should not receive notification for a heartbeat-only update")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior
	}

	// Now update the reported shards, which IS a significant change
	reportedShardsKey := tc.store.buildExecutorKey(tc.namespace, executorID, "reported_shards")
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

	err := tc.store.DeleteExecutors(ctx, tc.namespace, []string{}, store.NopGuard())
	require.NoError(t, err)
}

// TestDeleteExecutors covers various scenarios for the DeleteExecutors method.
func TestDeleteExecutors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup: Create two active executors for the tests.
	executorID1 := "executor-to-delete-1"
	executorID2 := "executor-to-delete-2"
	survivingExecutorID := "executor-survivor"
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, executorID2, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
	require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, survivingExecutorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

	t.Run("DeletesExecutorAndAssignedShards", func(t *testing.T) {
		shardID := "shard-to-be-deleted"
		require.NoError(t, tc.store.AssignShard(ctx, tc.namespace, shardID, executorID1), "Setup: Assign shard")

		// Action: Delete the executor.
		err := tc.store.DeleteExecutors(ctx, tc.namespace, []string{executorID1}, store.NopGuard())
		require.NoError(t, err)

		// Verification:
		// 1. Check that the executor is gone.
		_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, executorID1)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor should be deleted")

		// 2. Check that its assigned shard is also gone.
		_, err = tc.store.GetShardOwner(ctx, tc.namespace, shardID)
		assert.ErrorIs(t, err, store.ErrShardNotFound, "Assigned shard should be deleted")
	})

	t.Run("DoesNotDeleteShardIfReassigned", func(t *testing.T) {
		shardID := "shard-reassigned"
		// Setup:
		// 1. Assign a shard to executor2.
		require.NoError(t, tc.store.AssignShard(ctx, tc.namespace, shardID, executorID2))

		// 2. Simulate a race condition: Manually reassign the shard to the 'surviving' executor
		//    by directly writing to the shard owner key. This mimics a state where the distributor
		//    reassigns the shard after DeleteExecutors has read the old state but before it commits.
		shardOwnerKey := tc.store.buildShardKey(tc.namespace, shardID, shardAssignedKey)
		_, err := tc.client.Put(ctx, shardOwnerKey, survivingExecutorID)
		require.NoError(t, err, "Setup: Manually reassign shard owner")

		// Action: Attempt to delete executor2. The transaction should proceed but should
		// fail to delete the shard key because the owner no longer matches.
		err = tc.store.DeleteExecutors(ctx, tc.namespace, []string{executorID2}, store.NopGuard())
		require.NoError(t, err)

		// Verification:
		// 1. Check that executor2 is gone.
		_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, executorID2)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor should be deleted")

		// 2. Check that the shard was NOT deleted and is still owned by the survivor.
		owner, err := tc.store.GetShardOwner(ctx, tc.namespace, shardID)
		require.NoError(t, err, "Shard should not have been deleted")
		assert.Equal(t, survivingExecutorID, owner, "Shard ownership should have been preserved")
	})

	t.Run("SucceedsForNonExistentExecutor", func(t *testing.T) {
		// Action: Delete a non-existent executor.
		err := tc.store.DeleteExecutors(ctx, tc.namespace, []string{"non-existent-executor"}, store.NopGuard())
		// Verification: Should not return an error.
		require.NoError(t, err)
	})

	t.Run("DeletesMultipleExecutorsAndShards", func(t *testing.T) {
		// Setup: Create and assign shards to multiple executors.
		execToDelete1 := "multi-delete-1"
		execToDelete2 := "multi-delete-2"
		execToKeep := "multi-keep-1"
		shardToDelete1 := "multi-shard-1"
		shardToDelete2 := "multi-shard-2"
		shardToKeep := "multi-shard-keep"

		require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, execToDelete1, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
		require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, execToDelete2, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))
		require.NoError(t, tc.store.RecordHeartbeat(ctx, tc.namespace, execToKeep, store.HeartbeatState{Status: types.ExecutorStatusACTIVE}))

		require.NoError(t, tc.store.AssignShard(ctx, tc.namespace, shardToDelete1, execToDelete1))
		require.NoError(t, tc.store.AssignShard(ctx, tc.namespace, shardToDelete2, execToDelete2))
		require.NoError(t, tc.store.AssignShard(ctx, tc.namespace, shardToKeep, execToKeep))

		// Action: Delete two of the three executors in one call.
		err := tc.store.DeleteExecutors(ctx, tc.namespace, []string{execToDelete1, execToDelete2}, store.NopGuard())
		require.NoError(t, err)

		// Verification:
		// 1. Check deleted executors and shards are gone.
		_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, execToDelete1)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor 1 should be gone")
		_, err = tc.store.GetShardOwner(ctx, tc.namespace, shardToDelete1)
		assert.ErrorIs(t, err, store.ErrShardNotFound, "Shard 1 should be gone")

		_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, execToDelete2)
		assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Executor 2 should be gone")
		_, err = tc.store.GetShardOwner(ctx, tc.namespace, shardToDelete2)
		assert.ErrorIs(t, err, store.ErrShardNotFound, "Shard 2 should be gone")

		// 2. Check that the surviving executor and its shard remain.
		_, _, err = tc.store.GetHeartbeat(ctx, tc.namespace, execToKeep)
		assert.NoError(t, err, "Surviving executor should still exist")
		owner, err := tc.store.GetShardOwner(ctx, tc.namespace, shardToKeep)
		assert.NoError(t, err, "Surviving shard should still exist")
		assert.Equal(t, execToKeep, owner, "Surviving shard should retain its owner")
	})
}

func TestParseExecutorKey_Errors(t *testing.T) {
	tc := setupStoreTestCluster(t)

	_, _, err := tc.store.parseExecutorKey(tc.namespace, "/wrong/prefix/exec/heartbeat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have expected prefix")

	key := tc.store.buildExecutorPrefix(tc.namespace) + "too/many/parts"
	_, _, err = tc.store.parseExecutorKey(tc.namespace, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected key format")
}

// TestAssignAndGetShardOwnerRoundtrip verifies the successful assignment and retrieval of a shard owner.
func TestAssignAndGetShardOwnerRoundtrip(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executorID := "executor-roundtrip"
	shardID := "shard-roundtrip"

	// Setup: Create an active executor.
	err := tc.store.RecordHeartbeat(ctx, tc.namespace, executorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)

	// 1. Assign a shard to the active executor.
	err = tc.store.AssignShard(ctx, tc.namespace, shardID, executorID)
	require.NoError(t, err, "Should successfully assign shard to an active executor")

	// 2. Get the owner and verify it's the correct executor.
	owner, err := tc.store.GetShardOwner(ctx, tc.namespace, shardID)
	require.NoError(t, err, "Should successfully get the shard owner after assignment")
	assert.Equal(t, executorID, owner, "Owner should be the executor it was assigned to")
}

// TestAssignShardErrors tests the various error conditions when assigning a shard.
func TestAssignShardErrors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	activeExecutorID := "executor-active-errors"
	drainingExecutorID := "executor-draining-errors"
	shardID1 := "shard-err-1"
	shardID2 := "shard-err-2"

	// Setup: Create an active and a draining executor, and assign one shard.
	err := tc.store.RecordHeartbeat(ctx, tc.namespace, activeExecutorID, store.HeartbeatState{Status: types.ExecutorStatusACTIVE})
	require.NoError(t, err)
	err = tc.store.RecordHeartbeat(ctx, tc.namespace, drainingExecutorID, store.HeartbeatState{Status: types.ExecutorStatusDRAINING})
	require.NoError(t, err)
	err = tc.store.AssignShard(ctx, tc.namespace, shardID1, activeExecutorID)
	require.NoError(t, err)

	// Case 1: Assigning an already-assigned shard.
	err = tc.store.AssignShard(ctx, tc.namespace, shardID1, activeExecutorID)
	require.Error(t, err, "Should fail to assign an already-assigned shard")
	assert.ErrorIs(t, err, store.ErrVersionConflict, "Error should be ErrVersionConflict for duplicate assignment")

	// Case 2: Assigning to a non-existent executor.
	err = tc.store.AssignShard(ctx, tc.namespace, shardID2, "non-existent-executor")
	require.Error(t, err, "Should fail to assign to a non-existent executor")
	assert.ErrorIs(t, err, store.ErrExecutorNotFound, "Error should be ErrExecutorNotFound")

	// Case 3: Assigning to a non-active (draining) executor.
	err = tc.store.AssignShard(ctx, tc.namespace, shardID2, drainingExecutorID)
	require.Error(t, err, "Should fail to assign to a draining executor")
	assert.ErrorIs(t, err, store.ErrVersionConflict, "Error should be ErrVersionConflict for non-active executor")
}

// TestGetShardOwnerErrors tests error conditions for getting a shard owner.
func TestGetShardOwnerErrors(t *testing.T) {
	tc := setupStoreTestCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get the owner of a shard that has not been assigned.
	_, err := tc.store.GetShardOwner(ctx, tc.namespace, "non-existent-shard")
	require.Error(t, err, "Should return an error for a non-existent shard")
	assert.ErrorIs(t, err, store.ErrShardNotFound, "Error should be ErrShardNotFound")
}

// --- Test Setup ---

type storeTestCluster struct {
	store     *Store // Use concrete type to access buildExecutorKey
	namespace string
	leaderCfg shardDistributorCfg.LeaderElection
	client    *clientv3.Client
	lifecycle *fxtest.Lifecycle
}

func setupStoreTestCluster(t *testing.T) *storeTestCluster {
	t.Helper()
	flag.Parse()
	testflags.RequireEtcd(t)

	namespace := fmt.Sprintf("ns-%s", strings.ToLower(t.Name()))

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

	rawStore := s.(*Store)

	_, err = client.Delete(context.Background(), rawStore.buildNamespacePrefix(namespace), clientv3.WithPrefix())
	require.NoError(t, err)

	return &storeTestCluster{
		namespace: namespace,
		store:     rawStore,
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
