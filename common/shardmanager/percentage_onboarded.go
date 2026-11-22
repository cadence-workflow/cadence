package shardmanager

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

// PercentageOnboarded reads the percentage onboarded to the shard manager and emits it as a gauge on
// every read.
type PercentageOnboarded struct {
	metricsScope metrics.Scope
	percentage   dynamicproperties.IntPropertyFn
}

func NewPercentageOnboarded(
	metricsClient metrics.Client,
	scopeIdx metrics.ScopeIdx,
	percentage dynamicproperties.IntPropertyFn,
) PercentageOnboarded {
	return PercentageOnboarded{
		metricsScope: metricsClient.Scope(scopeIdx),
		percentage:   percentage,
	}
}

func (p PercentageOnboarded) Current() int {
	percentage := p.percentage()
	p.metricsScope.UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(percentage))
	return percentage
}

// Enabled reports whether the percentage is above 0.
func (p PercentageOnboarded) Enabled() bool {
	return p.Current() > 0
}

// ShardOnboarded reports whether shardIndex is onboarded in a fixed namespace with numShards shards.
// The lowest shard indexes are onboarded first,
// so raising the percentage only adds shards and lowering it only removes them.
func ShardOnboarded(shardIndex, numShards, percentageOnboarded int) bool {
	if shardIndex < 0 || shardIndex >= numShards {
		return false
	}

	shardPercentile := shardIndex * 100 / numShards
	return shardPercentile < percentageOnboarded
}
