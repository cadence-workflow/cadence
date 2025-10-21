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

func TestBuildExecutorKey(t *testing.T) {
	got, err := BuildExecutorKey("/cadence", "test-ns", "exec-1", "heartbeat")
	assert.NoError(t, err)
	assert.Equal(t, "/cadence/test-ns/executors/exec-1/heartbeat", got)
}

func TestBuildExecutorKeyFail(t *testing.T) {
	_, err := BuildExecutorKey("/cadence", "test-ns", "exec-1", "invalid")
	assert.ErrorContains(t, err, "invalid key type: invalid")
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
