package history

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/shardmanager"
)

// OnboardingGate decides which history shards the shard manager manages.
type OnboardingGate struct {
	percentageOnboarded shardmanager.PercentageOnboarded
	numShards           int
}

func NewOnboardingGate(
	metricsClient metrics.Client,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	numShards int,
) *OnboardingGate {
	return &OnboardingGate{
		percentageOnboarded: shardmanager.NewPercentageOnboarded(metricsClient, metrics.ShardManagerHistoryOnboardingScope, percentageOnboarded),
		numShards:           numShards,
	}
}

// Enabled reports whether any history shard is onboarded.
// A percentage at zero offboards every host.
func (g *OnboardingGate) Enabled() bool {
	return g.percentageOnboarded.Enabled()
}

func (g *OnboardingGate) ShardOnboarded(shardID int) bool {
	return shardmanager.ShardOnboarded(shardID, g.numShards, g.percentageOnboarded.Current())
}
