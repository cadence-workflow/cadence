package shardcache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/uber/cadence/common/log/testlogger"
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

func TestShardExecutorCacheForwarding(t *testing.T) {
	testCluster := testhelper.SetupStoreTestCluster(t)
	logger := testlogger.New(t)

	// Setup: Create executor-1 with shard-1 and metadata
	setupExecutorWithShards(t, testCluster, "executor-1", []string{"shard-1"}, map[string]string{
		"datacenter": "dc1",
		"rack":       "rack-42",
	}, nil)

	cache := NewShardToExecutorCache(testCluster.EtcdPrefix, testCluster.Client, logger)
	cache.Start()
	defer cache.Stop()

	// This will read the namespace from the store as the cache is empty
	owner, err := cache.GetShardOwner(context.Background(), testCluster.Namespace, "shard-1")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", owner.ExecutorID)
	assert.Equal(t, "dc1", owner.Metadata["datacenter"])
	assert.Equal(t, "rack-42", owner.Metadata["rack"])

	// Check the cache is populated
	assert.Greater(t, cache.namespaceToShards[testCluster.Namespace].executorRevision["executor-1"], int64(0))
	assert.Equal(t, "executor-1", cache.namespaceToShards[testCluster.Namespace].shardToExecutor["shard-1"].ExecutorID)

	// Check the executor is also cached
	executor, err := cache.GetExecutor(context.Background(), testCluster.Namespace, "executor-1")
	assert.NoError(t, err)
	assert.Equal(t, "executor-1", executor.ExecutorID)
	assert.Equal(t, "dc1", executor.Metadata["datacenter"])
	assert.Equal(t, "rack-42", executor.Metadata["rack"])
}
