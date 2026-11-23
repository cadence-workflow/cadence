package shardmanager

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

func TestShardOnboarded(t *testing.T) {
	tests := []struct {
		name                string
		shardIndex          int
		numShards           int
		percentageOnboarded int
		want                bool
	}{
		{name: "1% onboards shard 163", shardIndex: 163, numShards: 16384, percentageOnboarded: 1, want: true},
		{name: "1% stops before shard 164", shardIndex: 164, numShards: 16384, percentageOnboarded: 1, want: false},
		{name: "100% onboards the last shard", shardIndex: 16383, numShards: 16384, percentageOnboarded: 100, want: true},
		{name: "huge negative percentage onboards nothing", shardIndex: 9, numShards: 1500, percentageOnboarded: math.MinInt / 3, want: false},
		{name: "negative shard index", shardIndex: -1, numShards: 16384, percentageOnboarded: 100, want: false},
		{name: "no shards", shardIndex: 0, numShards: 0, percentageOnboarded: 100, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShardOnboarded(tt.shardIndex, tt.numShards, tt.percentageOnboarded))
		})
	}
}

func TestShardOnboarded_RaisingThePercentageOnlyAddsShards(t *testing.T) {
	for _, numShards := range []int{1, 4, 1000, 16384} {
		for percentage := 0; percentage < 100; percentage++ {
			for shardIndex := 0; shardIndex < numShards; shardIndex++ {
				if ShardOnboarded(shardIndex, numShards, percentage) && !ShardOnboarded(shardIndex, numShards, percentage+1) {
					t.Fatalf("shard %d of %d is onboarded at %d%% but not at %d%%", shardIndex, numShards, percentage, percentage+1)
				}
			}
		}
	}
}

func TestPercentageOnboarded_Enabled(t *testing.T) {
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
			percentageOnboarded := NewPercentageOnboarded(
				metrics.NewClient(testScope, metrics.Matching, metrics.MigrationConfig{}),
				metrics.ShardManagerOnboardingScope,
				dynamicproperties.GetIntPropertyFn(tt.percentageOnboarded),
			)

			assert.Equal(t, tt.want, percentageOnboarded.Enabled())

			gauges := testScope.Snapshot().Gauges()
			require.Len(t, gauges, 1, "reading the percentage should report it as a single gauge")
			for _, gauge := range gauges {
				assert.Equal(t, "percentage_onboarded_to_shard_manager", gauge.Name())
				assert.Equal(t, float64(tt.percentageOnboarded), gauge.Value())
				assert.Equal(t, "ShardManagerOnboarding", gauge.Tags()[metrics.OperationTagName])
			}
		})
	}
}

func TestPercentageOnboarded_ReadsDynamicConfigOnEveryCall(t *testing.T) {
	percentage := 0
	percentageOnboarded := NewPercentageOnboarded(
		metrics.NewNoopMetricsClient(),
		metrics.ShardManagerOnboardingScope,
		func(...dynamicproperties.FilterOption) int { return percentage },
	)

	assert.Equal(t, 0, percentageOnboarded.Current())
	assert.False(t, percentageOnboarded.Enabled())

	percentage = 30

	assert.Equal(t, 30, percentageOnboarded.Current())
	assert.True(t, percentageOnboarded.Enabled())
}
