package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

func TestOnboardingGate(t *testing.T) {
	tests := []struct {
		name                string
		percentageOnboarded int
		shardID             int
		wantEnabled         bool
		wantShardOnboarded  bool
	}{
		{name: "offboarded at zero", percentageOnboarded: 0, shardID: 0, wantEnabled: false, wantShardOnboarded: false},
		{name: "1% onboards shard 163", percentageOnboarded: 1, shardID: 163, wantEnabled: true, wantShardOnboarded: true},
		{name: "1% stops before shard 164", percentageOnboarded: 1, shardID: 164, wantEnabled: true, wantShardOnboarded: false},
		{name: "100% onboards the last shard", percentageOnboarded: 100, shardID: 16383, wantEnabled: true, wantShardOnboarded: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", nil)
			gate := NewOnboardingGate(
				metrics.NewClient(testScope, metrics.History, metrics.MigrationConfig{}),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				16384,
			)

			assert.Equal(t, tt.wantEnabled, gate.Enabled())
			assert.Equal(t, tt.wantShardOnboarded, gate.ShardOnboarded(tt.shardID))

			gauges := testScope.Snapshot().Gauges()
			require.Len(t, gauges, 1)
			for _, gauge := range gauges {
				assert.Equal(t, "percentage_onboarded_to_shard_manager", gauge.Name())
				assert.Equal(t, float64(tt.percentageOnboarded), gauge.Value())
				assert.Equal(t, "ShardManagerHistoryOnboarding", gauge.Tags()[metrics.OperationTagName])
			}
		})
	}
}
