package membership

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/metrics"
)

func TestPercentageOnboarded(t *testing.T) {
	tests := []struct {
		name  string
		value int
	}{
		{name: "zero", value: 0},
		{name: "partial", value: 42},
		{name: "full", value: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", nil)
			mc := metrics.NewClient(testScope, metrics.Matching, metrics.MigrationConfig{})
			po := NewPercentageOnboarded(mc, metrics.ShardManagerOnboardingScope, dynamicproperties.GetIntPropertyFn(tt.value))

			assert.Equal(t, tt.value, po.Value())

			gauges := findGauges(testScope.Snapshot().Gauges(), "percentage_onboarded_to_shard_manager")
			assert.Len(t, gauges, 1, "expected a single operational gauge")
			assert.Equal(t, float64(tt.value), gauges[0].Value())
			assert.Equal(t, "ShardManagerOnboarding", gauges[0].Tags()[metrics.OperationTagName])
		})
	}
}

func TestStaticPercentageOnboarded(t *testing.T) {
	assert.Equal(t, 7, StaticPercentageOnboarded(7).Value())
}

func TestPercentageOnboarded_SeparateScopes(t *testing.T) {
	testScope := tally.NewTestScope("", nil)
	mc := metrics.NewClient(testScope, metrics.Frontend, metrics.MigrationConfig{})
	matching := NewPercentageOnboarded(mc, metrics.ShardManagerOnboardingScope, dynamicproperties.GetIntPropertyFn(30))
	history := NewPercentageOnboarded(mc, metrics.ShardManagerHistoryOnboardingScope, dynamicproperties.GetIntPropertyFn(5))

	assert.Equal(t, 30, matching.Value())
	assert.Equal(t, 5, history.Value())

	gauges := findGauges(testScope.Snapshot().Gauges(), "percentage_onboarded_to_shard_manager")
	assert.Len(t, gauges, 2, "each scope must emit its own gauge")

	byOperation := make(map[string]float64, len(gauges))
	for _, g := range gauges {
		byOperation[g.Tags()[metrics.OperationTagName]] = g.Value()
	}
	assert.Equal(t, map[string]float64{
		"ShardManagerOnboarding":        30,
		"ShardManagerHistoryOnboarding": 5,
	}, byOperation)
}

func findGauges(all map[string]tally.GaugeSnapshot, suffix string) []tally.GaugeSnapshot {
	var matches []tally.GaugeSnapshot
	for _, g := range all {
		if g.Name() == suffix {
			matches = append(matches, g)
		}
	}
	return matches
}
