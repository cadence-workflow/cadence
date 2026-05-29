package etcdkeys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildNamespacePrefix(t *testing.T) {
	got := BuildNamespacePrefix("/cadence", "test-ns")
	assert.Equal(t, "/cadence/test-ns", got)
}

func TestBuildExecutorsPrefix(t *testing.T) {
	got := BuildExecutorsPrefix("/cadence", "test-ns")
	assert.Equal(t, "/cadence/test-ns/executors/", got)
}

func TestBuildExecutorKey(t *testing.T) {
	got := BuildExecutorKey("/cadence", "test-ns", "exec-1", "heartbeat")
	assert.Equal(t, "/cadence/test-ns/executors/exec-1/heartbeat", got)
}

func TestParseExecutorKey(t *testing.T) {
	// Valid key
	executorID, keyType, err := ParseExecutorKey("/cadence", "test-ns", "/cadence/test-ns/executors/exec-1/heartbeat")
	assert.NoError(t, err)
	assert.Equal(t, "exec-1", executorID)
	assert.Equal(t, ExecutorHeartbeatKey, keyType)

	// Prefix missing
	_, _, err = ParseExecutorKey("/cadence", "test-ns", "/wrong/prefix")
	assert.ErrorContains(t, err, "key '/wrong/prefix' does not have expected prefix '/cadence/test-ns/executors/'")

	// Unexpected key format
	_, _, err = ParseExecutorKey("/cadence", "test-ns", "/cadence/test-ns/executors/exec-1/heartbeat/extra")
	assert.ErrorContains(t, err, "unexpected key format: /cadence/test-ns/executors/exec-1/heartbeat/extra")
}

func TestBuildMetadataKey(t *testing.T) {
	got := BuildMetadataKey("/cadence", "test-ns", "exec-1", "my-metadata-key")
	assert.Equal(t, "/cadence/test-ns/executors/exec-1/metadata/my-metadata-key", got)
}

func TestParseExecutorKey_MetadataKey(t *testing.T) {
	// Test that ParseExecutorKey correctly identifies metadata keys
	// and that we can extract the metadata key name from the full key
	metadataKey := BuildMetadataKey("/cadence", "test-ns", "exec-1", "hostname")

	executorID, keyType, err := ParseExecutorKey("/cadence", "test-ns", metadataKey)
	assert.NoError(t, err)
	assert.Equal(t, "exec-1", executorID)
	assert.Equal(t, ExecutorMetadataKey, keyType)
}

func TestParseExecutorKey_InvalidKeyType(t *testing.T) {
	key := BuildExecutorIDPrefix("/cadence", "test-ns", "exec-1") + "invalid_type"
	_, _, err := ParseExecutorKey("/cadence", "test-ns", key)
	assert.ErrorContains(t, err, "invalid executor key type: invalid_type")
}
