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
	"github.com/uber/cadence/common/types"
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

	MigrationConfig struct {
		MigrationMode dynamicproperties.StringPropertyFnWithNamespaceFilters
	}

	StaticConfig struct {
		// ShardDistribution is the configuration for leader election mechanism that is used by Shard distributor to handle shard distribution per namespace.
		ShardDistribution ShardDistribution `yaml:"shardDistribution"`
	}

	// ShardDistribution is a configuration for leader election running.
	ShardDistribution struct {
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
		// Period is the maximum duration between shard rebalance operations
		// Default: 30 seconds
		Period time.Duration `yaml:"period"`

		// Timeout is the maximum duration of a single shard rebalance operation
		// Default: 1 second
		Timeout time.Duration `yaml:"timeout"`

		// HeartbeatTTL is the duration after which, if no heartbeat is received from an executor,
		// the executor is considered stale and its shards are eligible for redistribution.
		// Default: 10 seconds
		HeartbeatTTL time.Duration `yaml:"heartbeatTTL"`
	}
)

const (
	NamespaceTypeFixed     = "fixed"
	NamespaceTypeEphemeral = "ephemeral"
)

const (
	MigrationModeINVALID                = "invalid"
	MigrationModeLOCALPASSTHROUGH       = "local_pass"
	MigrationModeLOCALPASSTHROUGHSHADOW = "local_pass_shadow"
	MigrationModeDISTRIBUTEDPASSTHROUGH = "distributed_pass"
	MigrationModeONBOARDED              = "onboarded"
)

// ConfigMode maps string migration mode values to types.MigrationMode
var ConfigMode = map[string]types.MigrationMode{
	MigrationModeINVALID:                types.MigrationModeINVALID,
	MigrationModeLOCALPASSTHROUGH:       types.MigrationModeLOCALPASSTHROUGH,
	MigrationModeLOCALPASSTHROUGHSHADOW: types.MigrationModeLOCALPASSTHROUGHSHADOW,
	MigrationModeDISTRIBUTEDPASSTHROUGH: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
	MigrationModeONBOARDED:              types.MigrationModeONBOARDED,
}

func (s *ShardDistribution) GetMigrationMode(namespace string) types.MigrationMode {
	for _, ns := range s.Namespaces {
		if ns.Name == namespace {
			return ConfigMode[ns.Mode]
		}
	}
	// TODO in the dynamic configuration I will setup a default value
	return ConfigMode[MigrationModeONBOARDED]
}

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, hostName string) *Config {
	return &Config{
		PersistenceMaxQPS:       dc.GetIntProperty(dynamicproperties.ShardManagerPersistenceMaxQPS),
		PersistenceGlobalMaxQPS: dc.GetIntProperty(dynamicproperties.ShardManagerPersistenceGlobalMaxQPS),
		ThrottledLogRPS:         dc.GetIntProperty(dynamicproperties.ShardManagerThrottledLogRPS),
		HostName:                hostName,
	}
}

// NewMigrationConfig returns new dynamic config with onboarding info
func NewMigrationConfig(dc *dynamicconfig.Collection) *MigrationConfig {
	return &MigrationConfig{
		MigrationMode: dc.GetStringPropertyFilteredByNamespace(dynamicproperties.MigrationMode),
	}
}

func (c *MigrationConfig) GetMigrationMode(namespace string) types.MigrationMode {
	mode, ok := ConfigMode[c.MigrationMode(namespace)]
	if !ok {
		return ConfigMode[MigrationModeONBOARDED]
	}
	return mode
}

// GetShardDistributionFromExternal converts other configs to an internal one.
func GetShardDistributionFromExternal(in config.ShardDistribution) ShardDistribution {

	namespaces := make([]Namespace, 0, len(in.Namespaces))
	for _, namespace := range in.Namespaces {
		namespaces = append(namespaces, Namespace(namespace))
	}

	return ShardDistribution{
		LeaderStore: Store(in.LeaderStore),
		Store:       Store(in.Store),
		Election:    Election(in.Election),
		Namespaces:  namespaces,
		Process:     LeaderProcess(in.Process),
	}
}
