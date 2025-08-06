// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package config

import (
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

type (
	// Config represents configuration for shard manager service
	Config struct {
		PersistenceMaxQPS       dynamicproperties.IntPropertyFn
		PersistenceGlobalMaxQPS dynamicproperties.IntPropertyFn
		ThrottledLogRPS         dynamicproperties.IntPropertyFn

		// hostname info
		HostName string
	}

	StaticConfig struct {
		// ShardDistribution is the configuration for leader election mechanism that is used by Shard distributor to handle shard distribution per namespace.
		ShardDistribution ShardDistribution `yaml:"shardDistribution"`
	}

	// ShardDistribution is a configuration for leader election running.
	ShardDistribution struct {
		Enabled     bool          `yaml:"enabled"`
		LeaderStore Store         `yaml:"leaderStore"`
		Election    Election      `yaml:"election"`
		Namespaces  []Namespace   `yaml:"namespaces"`
		Process     LeaderProcess `yaml:"process"`
		Store       Store         `yaml:"store"`
	}

	// Store is a generic container for any storage configuration that should be parsed by the implementation.
	Store struct {
		StorageParams *config.YamlNode `yaml:"storageParams"`
	}

	Namespace struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"` // The field is a string since it is shared between global config Supported values: fixed|ephemeral.
		Mode string `yaml:"mode"` // TODO: this should be an ENUM with possible modes: enabled, read_only, proxy, disabled
		// ShardNum is defined for fixed namespace.
		ShardNum int64 `yaml:"shardNum"`
	}

	Election struct {
		LeaderPeriod           time.Duration `yaml:"leaderPeriod"`           // Time to hold leadership before resigning
		MaxRandomDelay         time.Duration `yaml:"maxRandomDelay"`         // Maximum random delay before campaigning
		FailedElectionCooldown time.Duration `yaml:"failedElectionCooldown"` // wait between election attempts with unhandled errors
	}

	LeaderProcess struct {
		Period       time.Duration `yaml:"period"`
		HeartbeatTTL time.Duration `yaml:"heartbeatTTL"`
	}
)

const (
	NamespaceTypeFixed     = "fixed"
	NamespaceTypeEphemeral = "ephemeral"
)

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, hostName string) *Config {
	return &Config{
		PersistenceMaxQPS:       dc.GetIntProperty(dynamicproperties.ShardManagerPersistenceMaxQPS),
		PersistenceGlobalMaxQPS: dc.GetIntProperty(dynamicproperties.ShardManagerPersistenceGlobalMaxQPS),
		ThrottledLogRPS:         dc.GetIntProperty(dynamicproperties.ShardManagerThrottledLogRPS),
		HostName:                hostName,
	}
}

// GetShardDistributionFromExternal converts other configs to an internal one.
func GetShardDistributionFromExternal(in config.ShardDistribution) ShardDistribution {

	namespaces := make([]Namespace, 0, len(in.Namespaces))
	for _, namespace := range in.Namespaces {
		namespaces = append(namespaces, Namespace(namespace))
	}

	return ShardDistribution{
		Enabled:     in.Enabled,
		LeaderStore: Store(in.LeaderStore),
		Store:       Store(in.Store),
		Election:    Election(in.Election),
		Namespaces:  namespaces,
		Process:     LeaderProcess(in.Process),
	}
}
