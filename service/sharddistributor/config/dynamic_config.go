package config

import (
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

// DynamicConfig represents dynamic configuration for shard distributor service
type DynamicConfig struct {
	LoadBalancingMode dynamicproperties.StringPropertyFnWithNamespaceFilters
	MigrationMode     dynamicproperties.StringPropertyFnWithNamespaceFilters
}

// NewDynamicConfig returns a new instance of DynamicConfig
func NewDynamicConfig(dc *dynamicconfig.Collection) *DynamicConfig {
	return &DynamicConfig{
		LoadBalancingMode: dc.GetStringPropertyFilteredByNamespace(dynamicproperties.ShardDistributorLoadBalancingMode),
		MigrationMode:     dc.GetStringPropertyFilteredByNamespace(dynamicproperties.ShardDistributorMigrationMode),
	}
}

// GetMigrationMode gets the migration mode for a given namespace
// If the mode is invalid or not set, it defaults to MigrationModeONBOARDED
func (c *DynamicConfig) GetMigrationMode(namespace string) types.MigrationMode {
	mode, ok := MigrationMode[c.MigrationMode(namespace)]
	if !ok {
		return MigrationMode[MigrationModeONBOARDED]
	}
	return mode
}

// GetLoadBalancingMode gets the load balancing mode for a given namespace
// If the mode is invalid, it returns types.LoadBalancingModeINVALID
func (c *DynamicConfig) GetLoadBalancingMode(namespace string) types.LoadBalancingMode {
	mode, err := types.LoadBalancingModeString(c.LoadBalancingMode(namespace))
	if err != nil {
		return types.LoadBalancingModeINVALID
	}

	return mode
}
