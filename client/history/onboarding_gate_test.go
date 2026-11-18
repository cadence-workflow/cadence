package history

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

func TestOnboardingGate_ShardOnboarded(t *testing.T) {
	tests := []struct {
		name                string
		shardID             int
		numShards           int
		percentageOnboarded int
		want                bool
	}{
		{name: "1% onboards shard 163", shardID: 163, numShards: 16384, percentageOnboarded: 1, want: true},
		{name: "1% stops before shard 164", shardID: 164, numShards: 16384, percentageOnboarded: 1, want: false},
		{name: "100% onboards the last shard", shardID: 16383, numShards: 16384, percentageOnboarded: 100, want: true},
		{name: "huge negative percentage onboards nothing", shardID: 9, numShards: 1500, percentageOnboarded: math.MinInt / 3, want: false},
		{name: "negative shard ID", shardID: -1, numShards: 16384, percentageOnboarded: 100, want: false},
		{name: "no shards", shardID: 0, numShards: 0, percentageOnboarded: 100, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := NewOnboardingGate(
				metrics.NewNoopMetricsClient(),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				tt.numShards,
			)
			assert.Equal(t, tt.want, gate.ShardOnboarded(tt.shardID))
		})
	}
}

func TestOnboardingGate_RaisingThePercentageOnlyAddsShards(t *testing.T) {
	newGate := func(percentage, numShards int) *OnboardingGate {
		return NewOnboardingGate(metrics.NewNoopMetricsClient(), dynamicproperties.GetIntPropertyFn(percentage), numShards)
	}
	for _, numShards := range []int{1, 4, 1000, 16384} {
		for percentage := 0; percentage < 100; percentage++ {
			before, after := newGate(percentage, numShards), newGate(percentage+1, numShards)
			for shardID := 0; shardID < numShards; shardID++ {
				if before.ShardOnboarded(shardID) && !after.ShardOnboarded(shardID) {
					t.Fatalf("shard %d of %d is onboarded at %d%% but not at %d%%", shardID, numShards, percentage, percentage+1)
				}
			}
		}
	}
}

func TestOnboardingGate_Enabled(t *testing.T) {
	tests := []struct {
		name                string
		percentageOnboarded int
		want                bool
	}{
		{name: "negative", percentageOnboarded: -1, want: false},
		{name: "offboarded at zero", percentageOnboarded: 0, want: false},
		{name: "partially onboarded", percentageOnboarded: 1, want: true},
		{name: "fully onboarded", percentageOnboarded: 100, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", nil)
			gate := NewOnboardingGate(
				metrics.NewClient(testScope, metrics.History, metrics.MigrationConfig{}),
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
				1000,
			)

			assert.Equal(t, tt.want, gate.Enabled())

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
