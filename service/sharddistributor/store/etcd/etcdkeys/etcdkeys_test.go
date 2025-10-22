package etcdkeys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildNamespacePrefix(t *testing.T) {
	got := BuildNamespacePrefix("/cadence", "test-ns")
	assert.Equal(t, "/cadence/test-ns", got)
}

func TestBuildExecutorPrefix(t *testing.T) {
	got := BuildExecutorPrefix("/cadence", "test-ns")
	assert.Equal(t, "/cadence/test-ns/executors/", got)
}

func TestBuildShardPrefix(t *testing.T) {
	got := BuildShardPrefix("/cadence", "test-ns")
	assert.Equal(t, "/cadence/test-ns/shards/", got)
}

func TestBuildExecutorKey(t *testing.T) {
	got, err := BuildExecutorKey("/cadence", "test-ns", "exec-1", "heartbeat")
	assert.NoError(t, err)
	assert.Equal(t, "/cadence/test-ns/executors/exec-1/heartbeat", got)
}

func TestBuildExecutorKeyFail(t *testing.T) {
	_, err := BuildExecutorKey("/cadence", "test-ns", "exec-1", "invalid")
	assert.ErrorContains(t, err, "invalid key type: invalid")
}

func TestBuildShardKey(t *testing.T) {
	got, err := BuildShardKey("/cadence", "test-ns", "shard-1", ShardMetricsKey)
	assert.NoError(t, err)
	assert.Equal(t, "/cadence/test-ns/shards/shard-1/metrics", got)
}

func TestBuildShardKeyFail(t *testing.T) {
	_, err := BuildShardKey("/cadence", "test-ns", "shard-1", "not-valid")
	assert.ErrorContains(t, err, "invalid shard key type: not-valid")
}

func TestParseExecutorKey(t *testing.T) {
	// Valid key
	executorID, keyType, err := ParseExecutorKey("/cadence", "test-ns", "/cadence/test-ns/executors/exec-1/heartbeat")
	assert.NoError(t, err)
	assert.Equal(t, "exec-1", executorID)
	assert.Equal(t, "heartbeat", keyType)

	// Prefix missing
	_, _, err = ParseExecutorKey("/cadence", "test-ns", "/wrong/prefix")
	assert.ErrorContains(t, err, "key '/wrong/prefix' does not have expected prefix '/cadence/test-ns/executors/'")

	// Unexpected key format
	_, _, err = ParseExecutorKey("/cadence", "test-ns", "/cadence/test-ns/executors/exec-1/heartbeat/extra")
	assert.ErrorContains(t, err, "unexpected key format: /cadence/test-ns/executors/exec-1/heartbeat/extra")
}

func TestParseShardKey(t *testing.T) {
	// Valid key
	shardID, keyType, err := ParseShardKey("/cadence", "test-ns", "/cadence/test-ns/shards/shard-7/metrics")
	assert.NoError(t, err)
	assert.Equal(t, "shard-7", shardID)
	assert.Equal(t, ShardMetricsKey, keyType)

	// Prefix missing
	_, _, err = ParseShardKey("/cadence", "test-ns", "/cadence/other/shards/shard-7/metrics")
	assert.ErrorContains(t, err, "key '/cadence/other/shards/shard-7/metrics' does not have expected prefix '/cadence/test-ns/shards/'")

	// Unexpected format
	_, _, err = ParseShardKey("/cadence", "test-ns", "/cadence/test-ns/shards/shard-7/metrics/extra")
	assert.ErrorContains(t, err, "unexpected shard key format: /cadence/test-ns/shards/shard-7/metrics/extra")
}
