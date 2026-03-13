package etcdclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/service/sharddistributor/config"
)

func TestNewExecutorStoreConfig(t *testing.T) {
	expected := ExecutorStoreConfig{
		BaseConfig: BaseConfig{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
			Prefix:      "/prefix",
		},
		Compression: "snappy",
	}

	sdConfig := createSDConfig(t, expected)
	result, err := NewExecutorStoreConfig(sdConfig)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestNewExecutorStoreConfig_InvalidConfig(t *testing.T) {
	sdConfig := config.ShardDistribution{
		Store: config.Store{StorageParams: createYamlNode(t, "")},
	}

	_, err := NewExecutorStoreConfig(sdConfig)
	require.Error(t, err)
}

func TestNewLeaderStoreConfig(t *testing.T) {
	expected := LeaderStoreConfig{
		BaseConfig: BaseConfig{
			Endpoints:   []string{"127.0.0.1:2379"},
			DialTimeout: 5 * time.Second,
			Prefix:      "/election",
		},
		ElectionTTL: 10 * time.Second,
	}

	sdConfig := createSDConfigLeader(t, expected)
	result, err := NewLeaderStoreConfig(sdConfig)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestNewLeaderStoreConfig_InvalidConfig(t *testing.T) {
	sdConfig := config.ShardDistribution{
		LeaderStore: config.Store{StorageParams: createYamlNode(t, "")},
	}

	_, err := NewLeaderStoreConfig(sdConfig)
	require.Error(t, err)
}

func createSDConfig(t *testing.T, cfg ExecutorStoreConfig) config.ShardDistribution {
	t.Helper()
	return config.ShardDistribution{
		Store: config.Store{StorageParams: createYamlNode(t, cfg)},
	}
}

func createSDConfigLeader(t *testing.T, cfg LeaderStoreConfig) config.ShardDistribution {
	t.Helper()
	return config.ShardDistribution{
		LeaderStore: config.Store{StorageParams: createYamlNode(t, cfg)},
	}
}

func createYamlNode(t *testing.T, v any) *config.YamlNode {
	t.Helper()
	encoded, err := yaml.Marshal(v)
	require.NoError(t, err)

	var node *config.YamlNode
	err = yaml.Unmarshal(encoded, &node)
	require.NoError(t, err)

	return node
}
