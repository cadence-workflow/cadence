package etcdclient

import (
	"fmt"
	"time"

	"github.com/uber/cadence/service/sharddistributor/config"
)

// BaseConfig has common ETCD connection and store settings.
type BaseConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	DialTimeout time.Duration `yaml:"dialTimeout"`
	Prefix      string        `yaml:"prefix"`
}

// ExecutorStoreConfig extends BaseConfig with executor-specific settings.
type ExecutorStoreConfig struct {
	BaseConfig  `yaml:",inline"`
	Compression string `yaml:"compression"`
}

// LeaderStoreConfig extends BaseConfig with leader-specific settings.
type LeaderStoreConfig struct {
	BaseConfig  `yaml:",inline"`
	ElectionTTL time.Duration `yaml:"electionTTL"`
}

// NewExecutorStoreConfig parses ExecutorStoreConfig from ShardDistribution config.
func NewExecutorStoreConfig(cfg config.ShardDistribution) (ExecutorStoreConfig, error) {
	var out ExecutorStoreConfig
	if err := cfg.Store.StorageParams.Decode(&out); err != nil {
		return out, fmt.Errorf("bad config for executor store: %w", err)
	}
	return out, nil
}

// NewLeaderStoreConfig parses LeaderStoreConfig from ShardDistribution config.
func NewLeaderStoreConfig(cfg config.ShardDistribution) (LeaderStoreConfig, error) {
	var out LeaderStoreConfig
	if err := cfg.LeaderStore.StorageParams.Decode(&out); err != nil {
		return out, fmt.Errorf("bad config for leader store: %w", err)
	}
	return out, nil
}
