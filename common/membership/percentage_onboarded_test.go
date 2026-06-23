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
			po := NewPercentageOnboarded(mc, dynamicproperties.GetIntPropertyFn(tt.value))

			assert.Equal(t, tt.value, po.Value())

			gauges := findGauges(testScope.Snapshot().Gauges(), "percentage_onboarded_to_shard_manager")
			assert.Len(t, gauges, 1, "expected a single operational gauge")
			assert.Equal(t, float64(tt.value), gauges[0].Value())
		})
	}
}

func TestStaticPercentageOnboarded(t *testing.T) {
	assert.Equal(t, 7, StaticPercentageOnboarded(7).Value())
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
