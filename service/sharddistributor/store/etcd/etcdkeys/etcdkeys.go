package etcdkeys

import (
	"fmt"
	"strings"
)

const (
	ExecutorHeartbeatKey      = "heartbeat"
	ExecutorStatusKey         = "status"
	ExecutorReportedShardsKey = "reported_shards"
	ExecutorAssignedStateKey  = "assigned_state"
	ShardAssignedKey          = "assigned"
	ShardStatisticsKey        = "statistics"
	ExecutorMetadataKey       = "metadata"
)

var validKeyTypes = []string{
	ExecutorHeartbeatKey,
	ExecutorStatusKey,
	ExecutorReportedShardsKey,
	ExecutorAssignedStateKey,
	ExecutorMetadataKey,
}

func isValidKeyType(key string) bool {
	for _, validKey := range validKeyTypes {
		if key == validKey {
			return true
		}
	}
	return false
}

func BuildNamespacePrefix(prefix string, namespace string) string {
	return fmt.Sprintf("%s/%s", prefix, namespace)
}

func BuildExecutorPrefix(prefix string, namespace string) string {
	return fmt.Sprintf("%s/executors/", BuildNamespacePrefix(prefix, namespace))
}

func BuildExecutorKey(prefix string, namespace, executorID, keyType string) (string, error) {
	// We allow an empty key, to build the full prefix
	if !isValidKeyType(keyType) && keyType != "" {
		return "", fmt.Errorf("invalid key type: %s", keyType)
	}
	return fmt.Sprintf("%s%s/%s", BuildExecutorPrefix(prefix, namespace), executorID, keyType), nil
}

func ParseExecutorKey(prefix string, namespace, key string) (executorID, keyType string, err error) {
	prefix = BuildExecutorPrefix(prefix, namespace)
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
	if len(parts) > 2 && parts[1] == ExecutorMetadataKey {
		// This is a metadata key, return "metadata" as the keyType
		return parts[0], parts[1], nil
	}
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected key format: %s", key)
	}
	return parts[0], parts[1], nil
}

func BuildShardPrefix(prefix string, namespace string) string {
	return fmt.Sprintf("%s/shards/", BuildNamespacePrefix(prefix, namespace))
}

func BuildShardKey(prefix string, namespace, shardID, keyType string) (string, error) {
	if keyType != ShardStatisticsKey {
		return "", fmt.Errorf("invalid shard key type: %s", keyType)
	}
	return fmt.Sprintf("%s%s/%s", BuildShardPrefix(prefix, namespace), shardID, keyType), nil
}

func ParseShardKey(prefix string, namespace, key string) (shardID, keyType string, err error) {
	prefix = BuildShardPrefix(prefix, namespace)
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("key '%s' does not have expected prefix '%s'", key, prefix)
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected shard key format: %s", key)
	}
	return parts[0], parts[1], nil
}

func BuildMetadataKey(prefix string, namespace, executorID, metadataKey string) string {
	metadataKeyPrefix, err := BuildExecutorKey(prefix, namespace, executorID, ExecutorMetadataKey)
	if err != nil {
		// This should never happen since ExecutorMetadataKey is a valid constant
		panic(fmt.Sprintf("BuildMetadataKey: unexpected error building executor key: %v", err))
	}
	return fmt.Sprintf("%s/%s", metadataKeyPrefix, metadataKey)
}
