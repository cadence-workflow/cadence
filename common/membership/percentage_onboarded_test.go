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
		name                  string
		useOperational        bool
		emergency             bool
		dynamicValue          int
		operationalValue      int
		want                  int
		wantDynamicConfigMode string
		wantOperationalMode   string
	}{
		{
			name:                  "reads from dynamic config when flag is off",
			useOperational:        false,
			emergency:             false,
			dynamicValue:          42,
			operationalValue:      99,
			want:                  42,
			wantDynamicConfigMode: "active",
			wantOperationalMode:   "shadow",
		},
		{
			name:                  "reads from operational store when flag is on",
			useOperational:        true,
			emergency:             false,
			dynamicValue:          42,
			operationalValue:      99,
			want:                  99,
			wantDynamicConfigMode: "shadow",
			wantOperationalMode:   "active",
		},
		{
			name:                  "emergency offboarding returns 0 even when reading from dynamic config",
			useOperational:        false,
			emergency:             true,
			dynamicValue:          42,
			operationalValue:      99,
			want:                  0,
			wantDynamicConfigMode: "active",
			wantOperationalMode:   "shadow",
		},
		{
			name:                  "emergency offboarding returns 0 even when reading from operational store",
			useOperational:        true,
			emergency:             true,
			dynamicValue:          42,
			operationalValue:      99,
			want:                  0,
			wantDynamicConfigMode: "shadow",
			wantOperationalMode:   "active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testScope := tally.NewTestScope("", nil)
			mc := metrics.NewClient(testScope, metrics.Matching, metrics.MigrationConfig{})
			po := NewPercentageOnboarded(
				mc,
				dynamicproperties.GetIntPropertyFn(tt.dynamicValue),
				dynamicproperties.GetIntPropertyFn(tt.operationalValue),
				dynamicproperties.GetBoolPropertyFn(tt.useOperational),
				dynamicproperties.GetBoolPropertyFn(tt.emergency),
			)

			assert.Equal(t, tt.want, po.Value())

			gauges := findGauges(testScope.Snapshot().Gauges(), "percentage_onboarded_to_shard_manager")
			if tt.emergency {
				assert.Empty(t, gauges, "no gauges emitted during emergency offboarding")
				return
			}
			assert.Len(t, gauges, 2, "expected one gauge per source")
			for _, g := range gauges {
				tagsMap := g.Tags()
				switch tagsMap["onboarding_source"] {
				case "dynamicconfig":
					assert.Equal(t, float64(tt.dynamicValue), g.Value())
					assert.Equal(t, tt.wantDynamicConfigMode, tagsMap["onboarding_active"])
				case "operational":
					assert.Equal(t, float64(tt.operationalValue), g.Value(), "operational gauge should reflect the operational source value")
					assert.Equal(t, tt.wantOperationalMode, tagsMap["onboarding_active"])
				default:
					t.Fatalf("unexpected onboarding_source tag: %q", tagsMap["onboarding_source"])
				}
			}
		})
	}
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
