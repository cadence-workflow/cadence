package etcdkeys

import (
	"fmt"
	"strings"
)

// BuildNamespacePrefix constructs the etcd key prefix for a given namespace.
// result: <prefix>/<namespace>
func BuildNamespacePrefix(prefix, namespace string) string {
	return fmt.Sprintf("%s/%s", prefix, namespace)
}

// BuildExecutorsPrefix constructs the etcd key prefix for executors within a given namespace.
// result: <prefix>/<namespace>/executors/
func BuildExecutorsPrefix(prefix, namespace string) string {
	return fmt.Sprintf("%s/executors/", BuildNamespacePrefix(prefix, namespace))
}

// BuildExecutorIDPrefix constructs the etcd key prefix for a specific executor within a namespace.
// result: <prefix>/<namespace>/executors/<executorID>/
func BuildExecutorIDPrefix(prefix, namespace, executorID string) string {
	return fmt.Sprintf("%s%s/", BuildExecutorsPrefix(prefix, namespace), executorID)
}

// BuildShardsPrefix constructs the etcd key prefix for shards within a given namespace.
// result: <prefix>/<namespace>/shards/
func BuildShardsPrefix(prefix, namespace string) string {
	return fmt.Sprintf("%s/shards/", BuildNamespacePrefix(prefix, namespace))
}

// ExecutorKeyType represents the allowed executor-level key types in etcd.
// Use BuildExecutorKey to construct keys of these types.
type ExecutorKeyType string

const (
	ExecutorHeartbeatKey       ExecutorKeyType = "heartbeat"
	ExecutorStatusKey          ExecutorKeyType = "status"
	ExecutorReportedShardsKey  ExecutorKeyType = "reported_shards"
	ExecutorAssignedStateKey   ExecutorKeyType = "assigned_state"
	ExecutorMetadataKey        ExecutorKeyType = "metadata"
	ExecutorShardStatisticsKey ExecutorKeyType = "statistics"
)

// validExecutorKeyTypes defines the set of valid executor key types.
var validExecutorKeyTypes = map[ExecutorKeyType]struct{}{
	ExecutorHeartbeatKey:      {},
	ExecutorStatusKey:         {},
	ExecutorReportedShardsKey: {},
	ExecutorAssignedStateKey:  {},
	ExecutorMetadataKey:       {},
	ExecutorShardStatisticsKey: {},
}

// IsValidExecutorKeyType checks if the provided key type is valid.
func IsValidExecutorKeyType(keyType ExecutorKeyType) bool {
	_, exist := validExecutorKeyTypes[keyType]
	return exist
}

// BuildExecutorKey constructs the etcd key for a specific executor and key type.
// result: <prefix>/<namespace>/executors/<executorID>/<keyType>
func BuildExecutorKey(prefix, namespace, executorID string, keyType ExecutorKeyType) string {
	return fmt.Sprintf("%s%s", BuildExecutorIDPrefix(prefix, namespace, executorID), keyType)
}

// ParseExecutorKey parses an etcd key and extracts the executor ID and key type.
// It returns an error if the key does not conform to the expected format.
// Expected format of key: <prefix>/<namespace>/executors/<executorID>/<keyType>
func ParseExecutorKey(prefix, namespace, key string) (executorID string, keyType ExecutorKeyType, err error) {
	prefix = BuildExecutorsPrefix(prefix, namespace)
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("key '%s' does not have expected prefix '%s'", key, prefix)
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected key format: %s", key)
	}
	// For metadata keys, the format is: executorID/metadata/metadataKey
	// For other keys, the format is: executorID/keyType
	// We return executorID and the first keyType (e.g., "metadata")
	if len(parts) > 2 && ExecutorKeyType(parts[1]) == ExecutorMetadataKey {
		// This is a metadata key, return "metadata" as the keyType
		return parts[0], ExecutorMetadataKey, nil
	}
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected key format: %s", key)
	}
	if !IsValidExecutorKeyType(ExecutorKeyType(parts[1])) {
		return "", "", fmt.Errorf("invalid executor key type: %s", parts[1])
	}
	return parts[0], ExecutorKeyType(parts[1]), nil
}

// BuildMetadataKey constructs the etcd key for a specific metadata entry of an executor.
// result: <prefix>/<namespace>/executors/<executorID>/metadata/<metadataKey>
func BuildMetadataKey(prefix string, namespace, executorID, metadataKey string) string {
	return fmt.Sprintf("%s/%s", BuildExecutorKey(prefix, namespace, executorID, ExecutorMetadataKey), metadataKey)
}

// ShardKeyType represents the allowed shard-level key types in etcd.
// Use BuildShardKey to construct keys of these types.
type ShardKeyType string

const (
	ShardStatisticsKey ShardKeyType = "statistics"
)

// BuildShardKey constructs the etcd key for a specific shard and key type.
// result: <prefix>/<namespace>/shards/<shardID>/<keyType>
func BuildShardKey(prefix string, namespace, shardID string, keyType ShardKeyType) string {
	return fmt.Sprintf("%s%s/%s", BuildShardsPrefix(prefix, namespace), shardID, keyType)
}

// ParseShardKey parses an etcd key and extracts the shard ID and key type.
// It returns an error if the key does not conform to the expected format.
// Expected format of key: <prefix>/<namespace>/shards/<shardID>/<keyType>
func ParseShardKey(prefix string, namespace, key string) (shardID string, keyType ShardKeyType, err error) {
	prefix = BuildShardsPrefix(prefix, namespace)
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("key '%s' does not have expected prefix '%s'", key, prefix)
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected shard key format: %s", key)
	}
	if parts[1] != string(ShardStatisticsKey) {
		return "", "", fmt.Errorf("invalid shard key type: %s", parts[1])
	}
	return parts[0], ShardStatisticsKey, nil
}
