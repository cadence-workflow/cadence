package membership

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination percentage_onboarded_mock.go -self_package github.com/uber/cadence/common/membership

// PercentageOnboarded is the shared read path for the shard-manager onboarding
// percentage.
type PercentageOnboarded interface {
	Value() int
}

func NewPercentageOnboarded(
	metricsClient metrics.Client,
	value dynamicproperties.IntPropertyFn,
) PercentageOnboarded {
	return &percentageOnboarded{
		metricsClient: metricsClient,
		value:         value,
	}
}

type percentageOnboarded struct {
	metricsClient metrics.Client
	value         dynamicproperties.IntPropertyFn
}

// StaticPercentageOnboarded always returns the given value. Intended for test
// harnesses that build a Resource without bootstrap wiring.
func StaticPercentageOnboarded(value int) PercentageOnboarded {
	return staticPercentageOnboarded(value)
}

type staticPercentageOnboarded int

func (s staticPercentageOnboarded) Value() int { return int(s) }

func (p *percentageOnboarded) Value() int {
	value := p.value()

	p.metricsClient.Scope(metrics.ShardManagerOnboardingScope).
		UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(value))

	return value
}
