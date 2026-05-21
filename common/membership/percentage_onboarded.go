package membership

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination percentage_onboarded_mock.go -self_package github.com/uber/cadence/common/membership

// PercentageOnboarded is the shared read path for the shard-manager onboarding
// percentage. Value returns 0 when emergency offboarding is on.
type PercentageOnboarded interface {
	Value() int
}

func NewPercentageOnboarded(
	metricsClient metrics.Client,
	fromDynamicConfig dynamicproperties.IntPropertyFn,
	fromOperational dynamicproperties.IntPropertyFn,
	useOperational dynamicproperties.BoolPropertyFn,
	emergencyOffboarding dynamicproperties.BoolPropertyFn,
) PercentageOnboarded {
	return &percentageOnboarded{
		metricsClient:        metricsClient,
		fromDynamicConfig:    fromDynamicConfig,
		fromOperational:      fromOperational,
		useOperational:       useOperational,
		emergencyOffboarding: emergencyOffboarding,
	}
}

type percentageOnboarded struct {
	metricsClient        metrics.Client
	fromDynamicConfig    dynamicproperties.IntPropertyFn
	fromOperational      dynamicproperties.IntPropertyFn
	useOperational       dynamicproperties.BoolPropertyFn
	emergencyOffboarding dynamicproperties.BoolPropertyFn
}

// StaticPercentageOnboarded always returns the given value. Intended for test
// harnesses that build a Resource without bootstrap wiring.
func StaticPercentageOnboarded(value int) PercentageOnboarded {
	return staticPercentageOnboarded(value)
}

type staticPercentageOnboarded int

func (s staticPercentageOnboarded) Value() int { return int(s) }

func (p *percentageOnboarded) Value() int {
	if p.emergencyOffboarding() {
		return 0
	}

	operational := p.useOperational()
	dynamicValue := p.fromDynamicConfig()
	operationalValue := p.fromOperational()

	p.metricsClient.Scope(metrics.ShardManagerOnboardingScope,
		metrics.OnboardingSourceTag("dynamicconfig"),
		metrics.OnboardingActiveTag(!operational),
	).UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(dynamicValue))
	p.metricsClient.Scope(metrics.ShardManagerOnboardingScope,
		metrics.OnboardingSourceTag("operational"),
		metrics.OnboardingActiveTag(operational),
	).UpdateGauge(metrics.PercentageOnboardedToShardManagerGauge, float64(operationalValue))

	if operational {
		return operationalValue
	}
	return dynamicValue
}
