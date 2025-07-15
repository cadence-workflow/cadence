package etcd

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/service/sharddistributor/leader/store"
)

// TestShardStoreGetStateEmpty tests GetState with no data
func TestShardStoreGetStateEmpty(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-empty"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	shardStore, err := election.ShardStore(ctx)
	require.NoError(t, err)

	heartbeats, assignments, rev, err := shardStore.GetState(ctx)
	require.NoError(t, err)
	assert.Empty(t, heartbeats)
	assert.Empty(t, assignments)
	assert.NotEmpty(t, rev)
}

// TestShardStoreRoundTrip tests complete write and read cycle
func TestShardStoreRoundTrip(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-roundtrip"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	// Create test data manually to ensure we know the exact paths
	executorID := "executor-1"
	now := time.Now().Unix()

	etcdShardStore := storage.(*shardStore)

	// Set heartbeat and state
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "heartbeat"), strconv.FormatInt(now, 10))
	require.NoError(t, err)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "state"), "ACTIVE")
	require.NoError(t, err)

	// Set reported shards
	reportedShards := map[string]store.ShardState{
		"shard-1": {Status: "running", LastUpdated: now},
	}
	reportedJSON, _ := json.Marshal(reportedShards)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "reported_shards"), string(reportedJSON))
	require.NoError(t, err)

	// Test GetState can read the data
	heartbeats, assignments, _, err := storage.GetState(ctx)
	require.NoError(t, err)

	// Verify heartbeat data
	require.Len(t, heartbeats, 1)
	heartbeat := heartbeats[executorID]
	assert.Equal(t, executorID, heartbeat.ExecutorID)
	assert.Equal(t, now, heartbeat.LastHeartbeat)
	assert.Equal(t, store.ExecutorStateActive, heartbeat.State)

	// Verify assignment data (should have reported shards)
	require.Len(t, assignments, 1)
	assignment := assignments[executorID]
	assert.Equal(t, executorID, assignment.ExecutorID)
	assert.Equal(t, reportedShards, assignment.ReportedShards)

	// Test AssignShards
	assignedShards := map[string]store.ShardAssignment{
		"shard-1": {ShardID: "shard-1", AssignedAt: now, Priority: 1},
		"shard-2": {ShardID: "shard-2", AssignedAt: now, Priority: 2},
	}

	newState := map[string]store.AssignedState{
		executorID: {
			ExecutorID:     executorID,
			AssignedShards: assignedShards,
			ReportedShards: reportedShards,
		},
	}

	err = storage.AssignShards(ctx, newState)
	require.NoError(t, err)

	// Read back and verify the assignment was written
	_, assignments, _, err = storage.GetState(ctx)
	require.NoError(t, err)

	assignment = assignments[executorID]
	assert.Len(t, assignment.AssignedShards, 2)
	assert.Equal(t, assignedShards, assignment.AssignedShards)
}

// TestShardStoreMultipleExecutors tests handling multiple executors
func TestShardStoreMultipleExecutors(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-multiple"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	etcdShardStore := storage.(*shardStore)

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	// Create data for 3 executors
	executors := []string{"executor-1", "executor-2", "executor-3"}
	states := []store.ExecutorState{store.ExecutorStateActive, store.ExecutorStateDraining, store.ExecutorStateStopped}

	for i, executorID := range executors {
		timestamp := time.Now().Unix() + int64(i)

		_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "heartbeat"), strconv.FormatInt(timestamp, 10))
		require.NoError(t, err)
		_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "state"), string(states[i]))
		require.NoError(t, err)

		// Set empty shard maps
		emptyShards, _ := json.Marshal(map[string]store.ShardState{})
		_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "reported_shards"), string(emptyShards))
		require.NoError(t, err)

		emptyAssignments, _ := json.Marshal(map[string]store.ShardAssignment{})
		_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "assigned_shards"), string(emptyAssignments))
		require.NoError(t, err)
	}

	// Test GetState
	heartbeats, assignments, _, err := storage.GetState(ctx)
	require.NoError(t, err)

	assert.Len(t, heartbeats, 3)
	assert.Len(t, assignments, 3)

	for i, executorID := range executors {
		heartbeat := heartbeats[executorID]
		assert.Equal(t, states[i], heartbeat.State)

		assignment := assignments[executorID]
		assert.Equal(t, executorID, assignment.ExecutorID)
	}
}

// TestShardStoreAssignShards tests shard assignment functionality
func TestShardStoreAssignShards(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-assign"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	// Assign shards to multiple executors
	now := time.Now().Unix()
	newState := map[string]store.AssignedState{
		"executor-1": {
			ExecutorID: "executor-1",
			AssignedShards: map[string]store.ShardAssignment{
				"shard-1": {ShardID: "shard-1", AssignedAt: now, Priority: 1},
			},
			ReportedShards: make(map[string]store.ShardState),
		},
		"executor-2": {
			ExecutorID: "executor-2",
			AssignedShards: map[string]store.ShardAssignment{
				"shard-2": {ShardID: "shard-2", AssignedAt: now, Priority: 1},
			},
			ReportedShards: make(map[string]store.ShardState),
		},
	}

	err = storage.AssignShards(ctx, newState)
	require.NoError(t, err)

	// Verify assignments were written
	_, assignments, _, err := storage.GetState(ctx)
	require.NoError(t, err)

	assert.Len(t, assignments, 2)
	assert.Contains(t, assignments["executor-1"].AssignedShards, "shard-1")
	assert.Contains(t, assignments["executor-2"].AssignedShards, "shard-2")
}

// TestShardStoreLeadershipChange tests assignment failure when leadership changes
func TestShardStoreLeadershipChange(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-leadership"
	election1, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election1.Cleanup(ctx)

	err = election1.Campaign(ctx, "test-host-1")
	require.NoError(t, err)

	shardStore1, err := election1.ShardStore(ctx)
	require.NoError(t, err)

	// Create second election and change leadership
	election2, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election2.Cleanup(ctx)

	err = election1.Resign(ctx)
	require.NoError(t, err)

	err = election2.Campaign(ctx, "test-host-2")
	require.NoError(t, err)

	// Try to assign shards with old shard store (should fail)
	newState := map[string]store.AssignedState{
		"executor-1": {
			ExecutorID:     "executor-1",
			AssignedShards: map[string]store.ShardAssignment{},
			ReportedShards: make(map[string]store.ShardState),
		},
	}

	err = shardStore1.AssignShards(ctx, newState)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leadership may have changed")
}

// TestShardStoreMalformedJSON tests handling of malformed JSON
func TestShardStoreMalformedJSON(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-malformed"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	etcdShardStore := storage.(*shardStore)

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set up malformed JSON
	executorID := "executor-1"

	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "heartbeat"), "1234567890")
	require.NoError(t, err)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "state"), "ACTIVE")
	require.NoError(t, err)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "reported_shards"), "{invalid json")
	require.NoError(t, err)

	// Should return error for malformed JSON
	_, _, _, err = storage.GetState(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal reported shards")
}

// TestShardStoreInvalidHeartbeat tests handling of invalid heartbeat values
func TestShardStoreInvalidHeartbeat(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-invalid-heartbeat"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)

	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	etcdShardStore := storage.(*shardStore)

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	executorID := "executor-1"

	// Set invalid heartbeat (non-numeric)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "heartbeat"), "not-a-number")
	require.NoError(t, err)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "state"), "ACTIVE")
	require.NoError(t, err)

	// Set valid shard maps
	emptyShards, _ := json.Marshal(map[string]store.ShardState{})
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "reported_shards"), string(emptyShards))
	require.NoError(t, err)

	emptyAssignments, _ := json.Marshal(map[string]store.ShardAssignment{})
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorID, "assigned_shards"), string(emptyAssignments))
	require.NoError(t, err)

	// Should handle invalid heartbeat gracefully
	heartbeats, _, _, err := storage.GetState(ctx)
	require.NoError(t, err)

	require.Len(t, heartbeats, 1)
	heartbeat := heartbeats[executorID]
	assert.Equal(t, store.ExecutorStateActive, heartbeat.State)
	assert.Equal(t, heartbeat.LastHeartbeat, int64(0)) // Should be set to current time
}

// TestSubscribe_NotificationOnUpdate verifies that an update triggers a notification with a new revision.
func TestSubscribe_NotificationOnUpdate(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	election, err := tc.store.CreateElection(ctx, "ns-subscribe-update")
	require.NoError(t, err)
	defer election.Cleanup(ctx)
	err = election.Campaign(ctx, "host")
	require.NoError(t, err)
	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	// Get the current revision before making changes.
	_, _, initialRevision, err := storage.GetState(ctx)
	require.NoError(t, err)

	updateChan, err := storage.Subscribe(ctx)
	require.NoError(t, err)

	// Trigger an update.
	client, err := clientv3.New(clientv3.Config{Endpoints: tc.endpoints})
	require.NoError(t, err)
	defer client.Close()
	etcdStore := storage.(*shardStore)
	key := etcdStore.buildExecutorKey("executor-foo", "state")
	_, err = client.Put(ctx, key, "ACTIVE")
	require.NoError(t, err)

	// Assert: expect a new revision on the channel that is greater than the initial one.
	select {
	case receivedRevision := <-updateChan:
		assert.Greater(t, receivedRevision, initialRevision, "Received revision should be greater than the initial revision")
	case <-ctx.Done():
		assert.Fail(t, "timed out waiting for update notification")
	}
}

// TestSubscribe_Debouncing verifies that rapid significant changes result in a single, final notification.
func TestSubscribe_Debouncing(t *testing.T) {
	tc := setupETCDCluster(t)
	// Use the test's context for a hard deadline on the entire test.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	election, err := tc.store.CreateElection(ctx, "ns-subscribe-debounce")
	require.NoError(t, err)
	defer election.Cleanup(ctx)
	err = election.Campaign(ctx, "host")
	require.NoError(t, err)
	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	updateChan, err := storage.Subscribe(ctx)
	require.NoError(t, err)

	// Trigger multiple rapid updates to a significant key.
	client, err := clientv3.New(clientv3.Config{Endpoints: tc.endpoints})
	require.NoError(t, err)
	defer client.Close()
	etcdStore := storage.(*shardStore)

	var lastRevision int64
	for i := 0; i < 5; i++ {
		key := etcdStore.buildExecutorKey("executor-foo", "state")
		resp, err := client.Put(ctx, key, strconv.Itoa(i))
		require.NoError(t, err)
		lastRevision = resp.Header.Revision // Keep track of the last written revision.
	}

	// Use assert.Eventually to poll the channel. This is more robust than a fixed
	// timer because it will pass as soon as the condition is met.
	var receivedRevision int64
	assert.Eventually(
		t,
		func() bool {
			select {
			case receivedRevision = <-updateChan:
				// The debouncing may result in receiving an intermediate revision first.
				// We keep polling until we receive the final revision or a later one.
				return receivedRevision >= lastRevision
			default:
				// No notification yet, continue polling.
				return false
			}
		},
		5*time.Second,       // A generous deadline for the condition to be met on a slow CI.
		10*time.Millisecond, // How often to check the channel.
		"timed out waiting for debounced revision %d", lastRevision,
	)

	// After Eventually succeeds, we must have received the exact final revision.
	assert.Equal(t, lastRevision, receivedRevision)

	// Assert that the channel is now empty using a non-blocking check.
	select {
	case rev := <-updateChan:
		assert.Fail(t, "Should not have received any more notifications after settling", "Got revision %d", rev)
	default:
		// Success, channel is empty.
	}
}

// TestSubscribe_FilteringLogic verifies that notifications are correctly filtered based on key type and event.
func TestSubscribe_FilteringLogic(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	election, err := tc.store.CreateElection(ctx, "ns-subscribe-filter")
	require.NoError(t, err)
	defer election.Cleanup(ctx)
	err = election.Campaign(ctx, "host")
	require.NoError(t, err)
	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)

	updateChan, err := storage.Subscribe(ctx)
	require.NoError(t, err)

	client, err := clientv3.New(clientv3.Config{Endpoints: tc.endpoints})
	require.NoError(t, err)
	defer client.Close()
	etcdStore := storage.(*shardStore)
	executorID := "executor-filter"

	// Helper to assert no notification is received within a timeout.
	assertNoNotification := func(t *testing.T, because string) {
		t.Helper()
		select {
		case rev := <-updateChan:
			assert.Fail(t, "Should not have received a notification", "Reason: %s. Got revision %d", because, rev)
		case <-time.After(200 * time.Millisecond):
			// Success, no notification received.
		}
	}

	// Helper to assert a specific notification is received.
	assertNotification := func(t *testing.T, expectedRevision int64) {
		t.Helper()
		select {
		case receivedRevision := <-updateChan:
			assert.Equal(t, expectedRevision, receivedRevision, "Should receive the correct revision")
		case <-ctx.Done():
			assert.Fail(t, "timed out waiting for notification")
		}
	}

	t.Run("InsignificantChanges_AreIgnored", func(t *testing.T) {
		// Heartbeat update should be ignored.
		_, err = client.Put(ctx, etcdStore.buildExecutorKey(executorID, "heartbeat"), strconv.FormatInt(time.Now().Unix(), 10))
		require.NoError(t, err)
		assertNoNotification(t, "heartbeat updates are ignored")

		// Assigned shards update should be ignored.
		_, err = client.Put(ctx, etcdStore.buildExecutorKey(executorID, "assigned_shards"), "{}")
		require.NoError(t, err)
		assertNoNotification(t, "assigned_shards updates are ignored")

		// Unparseable key should be ignored.
		unparseableKey := etcdStore.buildExecutorPrefix() + "unparseable/key/format"
		_, err = client.Put(ctx, unparseableKey, "some-value")
		require.NoError(t, err)
		assertNoNotification(t, "unparseable keys are ignored")
	})

	t.Run("SignificantChanges_TriggerNotification", func(t *testing.T) {
		// State update should trigger notification.
		stateKey := etcdStore.buildExecutorKey(executorID, "state")
		resp, err := client.Put(ctx, stateKey, "DRAINING")
		require.NoError(t, err)
		assertNotification(t, resp.Header.Revision)

		// Reported shards update should trigger notification.
		rsKey := etcdStore.buildExecutorKey(executorID, "reported_shards")
		resp, err = client.Put(ctx, rsKey, `{"shard-1":{"status":"running"}}`)
		require.NoError(t, err)
		assertNotification(t, resp.Header.Revision)

		// Deletion should trigger notification.
		keyToDelete := etcdStore.buildExecutorKey(executorID, "temp_key_for_deletion")
		putResp, err := client.Put(ctx, keyToDelete, "to-be-deleted")
		require.NoError(t, err)
		assertNotification(t, putResp.Header.Revision) // Clear the channel from the PUT before the DELETE.

		delResp, err := client.Delete(ctx, keyToDelete)
		require.NoError(t, err)
		assertNotification(t, delResp.Header.Revision)
	})

	t.Run("MixedBatch_TriggersNotification", func(t *testing.T) {
		// A transaction with both an insignificant and a significant change should notify.
		insignificantOp := clientv3.OpPut(etcdStore.buildExecutorKey(executorID, "heartbeat"), "123")
		significantOp := clientv3.OpPut(etcdStore.buildExecutorKey(executorID, "state"), "STOPPED")

		txnResp, err := client.Txn(ctx).Then(insignificantOp, significantOp).Commit()
		require.NoError(t, err)
		require.True(t, txnResp.Succeeded)
		assertNotification(t, txnResp.Header.Revision)
	})
}

// TestSubscribe_ContextCancellation verifies the subscription channel closes on context cancellation.
func TestSubscribe_ContextCancellation(t *testing.T) {
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is always cancelled.

	election, err := tc.store.CreateElection(ctx, "ns-subscribe-cancel")
	require.NoError(t, err)
	defer election.Cleanup(ctx)
	err = election.Campaign(ctx, "host")
	require.NoError(t, err)
	shardStore, err := election.ShardStore(ctx)
	require.NoError(t, err)

	updateChan, err := shardStore.Subscribe(ctx)
	require.NoError(t, err)

	// Cancel the context.
	cancel()

	// Assert: the channel should be closed.
	select {
	case _, ok := <-updateChan:
		require.False(t, ok, "expected channel to be closed after context cancellation")
	case <-time.After(2 * time.Second):
		assert.Fail(t, "timed out waiting for channel to close")
	}
}

// TestDeleteExecutor verifies that all data for a specific executor is removed.
func TestDeleteExecutor(t *testing.T) {
	// Arrange
	tc := setupETCDCluster(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// --- Become leader to get a valid shard store ---
	namespace := "test-delete-executor"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)
	err = election.Campaign(ctx, "test-host")
	require.NoError(t, err)
	storage, err := election.ShardStore(ctx)
	require.NoError(t, err)
	etcdShardStore := storage.(*shardStore)

	// --- Manually create state for two executors in etcd ---
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	executorToDelete := "executor-to-delete"
	executorToKeep := "executor-to-keep"

	// Put data for the executor that will be deleted
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorToDelete, "heartbeat"), "123")
	require.NoError(t, err)
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorToDelete, "state"), "ACTIVE")
	require.NoError(t, err)

	// Put data for the executor that should remain
	_, err = client.Put(ctx, etcdShardStore.buildExecutorKey(executorToKeep, "heartbeat"), "456")
	require.NoError(t, err)

	// Act
	err = storage.DeleteExecutor(ctx, executorToDelete)
	require.NoError(t, err)

	// Assert
	// --- Verify that the target executor's data is gone ---
	deletedPrefix := etcdShardStore.buildExecutorKey(executorToDelete, "") // Gets the prefix for the executor
	getResp, err := client.Get(ctx, deletedPrefix, clientv3.WithPrefix())
	require.NoError(t, err)
	assert.Equal(t, int64(0), getResp.Count, "No keys should exist for the deleted executor")

	// --- Verify that the other executor's data remains ---
	keptPrefix := etcdShardStore.buildExecutorKey(executorToKeep, "")
	getResp, err = client.Get(ctx, keptPrefix, clientv3.WithPrefix())
	require.NoError(t, err)
	assert.Equal(t, int64(1), getResp.Count, "Data for the other executor should still exist")

	// --- Verify GetState reflects the deletion ---
	heartbeats, _, _, err := storage.GetState(ctx)
	require.NoError(t, err)
	assert.NotContains(t, heartbeats, executorToDelete, "GetState should not return the deleted executor")
	assert.Contains(t, heartbeats, executorToKeep, "GetState should still return the executor that was kept")
}
