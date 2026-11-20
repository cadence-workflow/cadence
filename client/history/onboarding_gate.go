package history

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

// OnboardingGate decides which history shards the shard manager manages.
// The lowest shard IDs are onboarded first,
// so raising the percentage only adds shards and lowering it only removes them
type OnboardingGate struct {
	metricsScope        metrics.Scope
	percentageOnboarded dynamicproperties.IntPropertyFn
	numShards           int
}

func NewOnboardingGate(
	metricsClient metrics.Client,
	percentageOnboarded dynamicproperties.IntPropertyFn,
	numShards int,
) *OnboardingGate {
	return &OnboardingGate{
		metricsScope:        metricsClient.Scope(metrics.ShardManagerHistoryOnboardingScope),
		percentageOnboarded: percentageOnboarded,
		numShards:           numShards,
	}
}

// Enabled reports whether any history shard is onboarded.
// A percentage at zero offboards every host.
func (g *OnboardingGate) Enabled() bool {
	return g.currentPercentageOnboarded() > 0
}

func (g *OnboardingGate) ShardOnboarded(shardID int) bool {
	return shardOnboarded(shardID, g.numShards, g.currentPercentageOnboarded())
}

func (g *OnboardingGate) currentPercentageOnboarded() int {
	percentageOnboarded := g.percentageOnboarded()
	g.metricsScope.UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(percentageOnboarded))
	return percentageOnboarded
}

func shardOnboarded(shardID, numShards, percentageOnboarded int) bool {
	if shardID < 0 || shardID >= numShards {
		return false
	}

	shardPercentile := shardID * 100 / numShards
	return shardPercentile < percentageOnboarded
}
