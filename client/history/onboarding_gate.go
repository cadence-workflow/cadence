package history

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/shardmanager"
)

// NewOnboardingGate decides which history shards the shard manager manages.
// A percentage at zero offboards every host.
func NewOnboardingGate(
	metricsClient metrics.Client,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	numShards int,
) shardmanager.PercentageOnboarded {
	return shardmanager.NewPercentageOnboarded(metricsClient, metrics.ShardManagerHistoryOnboardingScope, percentageOnboarded, numShards)
}
