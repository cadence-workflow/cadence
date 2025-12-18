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

	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

type (
	// Config represents configuration for shard manager service
	Config struct {
		LoadBalancingMode dynamicproperties.StringPropertyFnWithNamespaceFilters
		MigrationMode     dynamicproperties.StringPropertyFnWithNamespaceFilters
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
		StorageParams *YamlNode `yaml:"storageParams"`
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
		// Default: 1 second
		Period time.Duration `yaml:"period"`

		// Timeout is the maximum duration of a single shard rebalance operation
		// Default: 1 second
		Timeout time.Duration `yaml:"timeout"`

		// HeartbeatTTL is the duration after which, if no heartbeat is received from an executor,
		// the executor is considered stale and its shards are eligible for redistribution.
		// Default: 10 seconds
		HeartbeatTTL time.Duration `yaml:"heartbeatTTL"`

		// LoadBalance contains tunables for load-based rebalancing.
		LoadBalance LoadBalance `yaml:"loadBalance"`
	}

	LoadBalance struct {
		// PerShardCooldown is the minimum time between moving the same shard.
		// Default: 1 minute.
		PerShardCooldown time.Duration `yaml:"perShardCooldown"`

		// MoveBudgetProportion is the fraction of total shards that may be moved per load-balance pass.
		// Default: 0.01 (1%).
		MoveBudgetProportion float64 `yaml:"moveBudgetProportion"`

		// HysteresisUpperBand is the multiplier above mean load that qualifies an executor as a source.
		// Default: 1.15.
		HysteresisUpperBand float64 `yaml:"hysteresisUpperBand"`

		// HysteresisLowerBand is the multiplier below mean load that qualifies an executor as a destination.
		// Default: 0.95.
		HysteresisLowerBand float64 `yaml:"hysteresisLowerBand"`

		// SevereImbalanceRatio triggers a destination "escape hatch" when maxLoad/meanLoad exceeds this value
		// but no executor qualifies as a destination under the normal hysteresis lower band.
		// Default: 1.5.
		SevereImbalanceRatio float64 `yaml:"severeImbalanceRatio"`

		// DisableBenefitGating controls whether load-based balancing may accept moves that do not
		// strictly improve the objective function (currently: sum of squared error around mean load).
		// Default: false (benefit gating enabled).
		DisableBenefitGating bool `yaml:"disableBenefitGating"`
	}

	// YamlNode is a lazy-unmarshaler, because *yaml.Node only exists in gopkg.in/yaml.v3, not v2,
	// and go.uber.org/config currently uses only v2.
	YamlNode struct {
		unmarshal func(out any) error
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

// MigrationMode maps string migration mode values to types.MigrationMode
var MigrationMode = map[string]types.MigrationMode{
	MigrationModeINVALID:                types.MigrationModeINVALID,
	MigrationModeLOCALPASSTHROUGH:       types.MigrationModeLOCALPASSTHROUGH,
	MigrationModeLOCALPASSTHROUGHSHADOW: types.MigrationModeLOCALPASSTHROUGHSHADOW,
	MigrationModeDISTRIBUTEDPASSTHROUGH: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
	MigrationModeONBOARDED:              types.MigrationModeONBOARDED,
}

// NewConfig returns a new instance of Config
func NewConfig(dc *dynamicconfig.Collection) *Config {
	return &Config{
		LoadBalancingMode: dc.GetStringPropertyFilteredByNamespace(dynamicproperties.ShardDistributorLoadBalancingMode),
		MigrationMode:     dc.GetStringPropertyFilteredByNamespace(dynamicproperties.ShardDistributorMigrationMode),
	}
}

// GetMigrationMode gets the migration mode for a given namespace
// If the mode is not set, it defaults to MigrationModeINVALID
func (c *Config) GetMigrationMode(namespace string) types.MigrationMode {
	mode, ok := MigrationMode[c.MigrationMode(namespace)]
	if !ok {
		return MigrationMode[MigrationModeINVALID]
	}
	return mode
}

const (
	LoadBalancingModeINVALID = "invalid"
	LoadBalancingModeNAIVE   = "naive"
	LoadBalancingModeGREEDY  = "greedy"
)

// LoadBalancingMode maps string migration mode values to types.LoadBalancingMode
var LoadBalancingMode = map[string]types.LoadBalancingMode{
	LoadBalancingModeINVALID: types.LoadBalancingModeINVALID,
	LoadBalancingModeNAIVE:   types.LoadBalancingModeNAIVE,
	LoadBalancingModeGREEDY:  types.LoadBalancingModeGREEDY,
}

// GetLoadBalancingMode gets the load balancing mode for a given namespace
// If the mode is invalid, it returns types.LoadBalancingModeINVALID
func (c *Config) GetLoadBalancingMode(namespace string) types.LoadBalancingMode {
	mode, ok := LoadBalancingMode[c.LoadBalancingMode(namespace)]
	if !ok {
		return types.LoadBalancingModeINVALID
	}

	return mode
}

func (s *ShardDistribution) GetMigrationMode(namespace string) types.MigrationMode {
	for _, ns := range s.Namespaces {
		if ns.Name == namespace {
			return MigrationMode[ns.Mode]
		}
	}
	// TODO in the dynamic configuration I will setup a default value
	return MigrationMode[MigrationModeONBOARDED]
}

var _ yaml.Unmarshaler = (*YamlNode)(nil)

func (y *YamlNode) UnmarshalYAML(unmarshal func(interface{}) error) error {
	y.unmarshal = unmarshal
	return nil
}

func (y *YamlNode) Decode(out any) error {
	if y == nil {
		return nil
	}
	return y.unmarshal(out)
}
