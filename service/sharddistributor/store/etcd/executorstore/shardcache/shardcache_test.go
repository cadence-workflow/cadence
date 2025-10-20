package shardcache

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/etcdkeys"
	"github.com/uber/cadence/service/sharddistributor/store/etcd/testhelper"
)

func TestNewShardToExecutorCache(t *testing.T) {
	logger := testlogger.New(t)

	client := &clientv3.Client{}
	cache := NewShardToExecutorCache("some-prefix", client, logger)

	assert.NotNil(t, cache)

	assert.NotNil(t, cache.namespaceToShards)
	assert.NotNil(t, cache.stopC)

	assert.Equal(t, logger, cache.logger)
	assert.Equal(t, "some-prefix", cache.prefix)
	assert.Equal(t, client, cache.client)
}

func TestShardExecutorCache(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)

	logger := testlogger.New(t)

	// Setup: Create an assigned state for the executor
	assignedState := &store.AssignedState{
		AssignedShards: map[string]*types.ShardAssignment{
			"shard-1": {Status: types.AssignmentStatusREADY},
		},
	}
	assignedStateJSON, err := json.Marshal(assignedState)
	require.NoError(t, err)

	executorAssignedStateKey, err := etcdkeys.BuildExecutorKey(testCluster.EtcdPrefix, "test-ns", "executor-1", etcdkeys.ExecutorAssignedStateKey)
	require.NoError(t, err)
	testCluster.Client.Put(context.Background(), executorAssignedStateKey, string(assignedStateJSON))

	cache := NewShardToExecutorCache(testCluster.EtcdPrefix, testCluster.Client, logger)

	cache.Start()
	defer cache.Stop()

	// This will read the namespace from the store as the cache is empty
	owner, err := cache.GetShardOwner(context.Background(), "test-ns", "shard-1")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", owner)

	// Check the cache is populated
	assert.Greater(t, cache.namespaceToShards["test-ns"].executorRevision["executor-1"], int64(0))
	assert.Equal(t, "executor-1", cache.namespaceToShards["test-ns"].shardToExecutor["shard-1"])
}
