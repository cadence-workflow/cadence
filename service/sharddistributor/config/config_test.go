package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestNewDynamicConfigCreatesInstanceWithProperties(t *testing.T) {
	dc := dynamicconfig.NewNopCollection()

	config := NewConfig(dc)

	assert.NotNil(t, config)
	assert.NotNil(t, config.LoadBalancingMode)
	assert.NotNil(t, config.MigrationMode)
}

func TestGetMigrationMode(t *testing.T) {
	tests := []struct {
		name         string
		configValue  string
		expectedMode types.MigrationMode
	}{
		{
			name:         "LocalPassthrough",
			configValue:  MigrationModeLOCALPASSTHROUGH,
			expectedMode: types.MigrationModeLOCALPASSTHROUGH,
		},
		{
			name:         "LocalPassthroughShadow",
			configValue:  MigrationModeLOCALPASSTHROUGHSHADOW,
			expectedMode: types.MigrationModeLOCALPASSTHROUGHSHADOW,
		},
		{
			name:         "DistributedPassthrough",
			configValue:  MigrationModeDISTRIBUTEDPASSTHROUGH,
			expectedMode: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
		},
		{
			name:         "Onboarded",
			configValue:  MigrationModeONBOARDED,
			expectedMode: types.MigrationModeONBOARDED,
		},
		{
			name:         "Empty",
			configValue:  "",
			expectedMode: types.MigrationModeINVALID,
		},
		{
			name:         "Invalid",
			configValue:  MigrationModeINVALID,
			expectedMode: types.MigrationModeINVALID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicconfig.NewInMemoryClient()
			err := client.UpdateValue(dynamicproperties.ShardDistributorMigrationMode, tt.configValue)
			require.NoError(t, err)
			dc := dynamicconfig.NewCollection(client, testlogger.New(t))
			config := NewConfig(dc)

			mode := config.GetMigrationMode("test-namespace")
			assert.Equal(t, tt.expectedMode, mode)
		})
	}
}

func TestGetLoadBalancingMode(t *testing.T) {
	tests := []struct {
		name         string
		configValue  string
		expectedMode types.LoadBalancingMode
	}{
		{
			name:         "Naive",
			configValue:  "naive",
			expectedMode: types.LoadBalancingModeNAIVE,
		},
		{
			name:         "Greedy",
			configValue:  "greedy",
			expectedMode: types.LoadBalancingModeGREEDY,
		},
		{
			name:         "Invalid",
			configValue:  "invalid",
			expectedMode: types.LoadBalancingModeINVALID,
		},
		{
			name:         "Empty",
			configValue:  "",
			expectedMode: types.LoadBalancingModeINVALID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicconfig.NewInMemoryClient()
			err := client.UpdateValue(dynamicproperties.ShardDistributorLoadBalancingMode, tt.configValue)
			require.NoError(t, err)
			dc := dynamicconfig.NewCollection(client, testlogger.New(t))
			config := NewConfig(dc)

			mode := config.GetLoadBalancingMode("test-namespace")
			assert.Equal(t, tt.expectedMode, mode)
		})
	}
}
