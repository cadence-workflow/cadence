package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestCompressDecompress(t *testing.T) {
	original := []byte(`{"status":"ACTIVE","shards":["shard1","shard2"]}`)

	compressed, err := Compress(original, true)
	require.NoError(t, err)
	require.NotNil(t, compressed)

	assert.NotEqual(t, original, compressed)

	decompressed, err := Decompress(compressed)
	require.NoError(t, err)
	assert.Equal(t, original, decompressed)
}

func TestCompressWithDisabledFlag(t *testing.T) {
	original := []byte(`{"status":"ACTIVE","shards":["shard1","shard2"]}`)

	result, err := Compress(original, false)
	require.NoError(t, err)
	assert.Equal(t, original, result, "Should return original data when compression is disabled")

	var status map[string]interface{}
	err = json.Unmarshal(result, &status)
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", status["status"])
}

func TestDecompress(t *testing.T) {
	t.Run("Empty data", func(t *testing.T) {
		decompressed, err := Decompress([]byte{})
		require.NoError(t, err)
		assert.Empty(t, decompressed)
	})

	t.Run("Nil data", func(t *testing.T) {
		decompressed, err := Decompress(nil)
		require.NoError(t, err)
		assert.Nil(t, decompressed)
	})

	t.Run("Uncompressed data", func(t *testing.T) {
		uncompressed := []byte(`{"status":"ACTIVE"}`)

		result, err := Decompress(uncompressed)
		require.NoError(t, err)
		assert.Equal(t, uncompressed, result, "Uncompressed data is returned as-is")

		var status map[string]string
		err = json.Unmarshal(result, &status)
		require.NoError(t, err)
		assert.Equal(t, "ACTIVE", status["status"])
	})

	t.Run("Compressed data", func(t *testing.T) {
		original := []byte(`{"status":"DRAINING"}`)
		compressed, err := Compress(original, true)
		require.NoError(t, err)

		result, err := Decompress(compressed)
		require.NoError(t, err)
		assert.Equal(t, original, result)

		var status map[string]string
		err = json.Unmarshal(result, &status)
		require.NoError(t, err)
		assert.Equal(t, "DRAINING", status["status"])
	})
}

func TestDecompressAndUnmarshal(t *testing.T) {
	type testData struct {
		Status string   `json:"status"`
		Shards []string `json:"shards"`
	}
	logger := testlogger.New(t)

	t.Run("Uncompressed data", func(t *testing.T) {
		data := []byte(`{"status":"ACTIVE","shards":["shard1","shard2"]}`)

		var result testData
		err := DecompressAndUnmarshal(data, &result, "test data", logger)
		require.NoError(t, err)
		assert.Equal(t, "ACTIVE", result.Status)
		assert.Equal(t, []string{"shard1", "shard2"}, result.Shards)
	})

	t.Run("Compressed data", func(t *testing.T) {
		original := testData{
			Status: "DRAINING",
			Shards: []string{"shard3", "shard4"},
		}
		originalJSON, _ := json.Marshal(original)
		compressed, err := Compress(originalJSON, true)
		require.NoError(t, err)

		var result testData
		err = DecompressAndUnmarshal(compressed, &result, "test data", logger)
		require.NoError(t, err)
		assert.Equal(t, original.Status, result.Status)
		assert.Equal(t, original.Shards, result.Shards)
	})

	t.Run("Invalid JSON in uncompressed data", func(t *testing.T) {
		invalidJSON := []byte(`{invalid json}`)

		var result testData
		err := DecompressAndUnmarshal(invalidJSON, &result, "test data", logger)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal test data")
	})
}

func TestCompressedActiveStatus(t *testing.T) {
	t.Run("Compression enabled", func(t *testing.T) {
		compressed := CompressedActiveStatus(true)
		require.NotEmpty(t, compressed)

		decompressed, err := Decompress([]byte(compressed))
		require.NoError(t, err)

		var status types.ExecutorStatus
		err = json.Unmarshal(decompressed, &status)
		require.NoError(t, err)
		assert.Equal(t, types.ExecutorStatusACTIVE, status)
	})

	t.Run("Compression disabled", func(t *testing.T) {
		uncompressed := CompressedActiveStatus(false)
		require.NotEmpty(t, uncompressed)

		var status types.ExecutorStatus
		err := json.Unmarshal([]byte(uncompressed), &status)
		require.NoError(t, err)
		assert.Equal(t, types.ExecutorStatusACTIVE, status)
	})
}

func TestHasFramedHeader(t *testing.T) {
	t.Run("Data with header", func(t *testing.T) {
		data := append(snappyMagic, []byte("some data")...)
		assert.True(t, hasFramedHeader(data))
	})

	t.Run("Data without header", func(t *testing.T) {
		data := []byte(`{"json":"data"}`)
		assert.False(t, hasFramedHeader(data))
	})

	t.Run("Empty data", func(t *testing.T) {
		assert.False(t, hasFramedHeader([]byte{}))
	})

	t.Run("Data shorter than header", func(t *testing.T) {
		assert.False(t, hasFramedHeader([]byte{0xff, 0x06}))
	})
}
