package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestHistogramMigrationMetricsExist(t *testing.T) {
	dup := maps.Clone(HistogramMigrationMetrics)
	for _, serviceMetrics := range MetricDefs {
		for _, def := range serviceMetrics {
			delete(dup, def.metricName.String())
		}
	}
	if len(dup) != 0 {
		t.Error("HistogramMigrationMetrics contains metric names which do not exist:", dup)
	}
}

func TestGaugeMigrationMetricsExist(t *testing.T) {
	dup := maps.Clone(GaugeMigrationMetrics)
	for _, serviceMetrics := range MetricDefs {
		for _, def := range serviceMetrics {
			delete(dup, def.metricName.String())
		}
	}
	if len(dup) != 0 {
		t.Error("GaugeMigrationMetrics contains metric names which do not exist:", dup)
	}
}

func TestCounterMigrationMetricsExist(t *testing.T) {
	dup := maps.Clone(CounterMigrationMetrics)
	for _, serviceMetrics := range MetricDefs {
		for _, def := range serviceMetrics {
			delete(dup, def.metricName.String())
		}
	}
	if len(dup) != 0 {
		t.Error("CounterMigrationMetrics contains metric names which do not exist:", dup)
	}
}

func TestGaugeMigration_EmitGauge(t *testing.T) {
	orig := GaugeMigrationMetrics
	t.Cleanup(func() { GaugeMigrationMetrics = orig })

	GaugeMigrationMetrics = map[string]struct{}{
		"metric_a": {},
		"metric_b": {},
		"metric_c": {},
	}

	tests := []struct {
		name     string
		config   GaugeMigration
		metric   string
		expected bool
	}{
		{
			name:     "non-migration metric always emits",
			config:   GaugeMigration{},
			metric:   "some_other_metric",
			expected: true,
		},
		{
			name:     "migration metric with default empty mode emits gauge",
			config:   GaugeMigration{},
			metric:   "metric_a",
			expected: true,
		},
		{
			name:     "migration metric with default gauge mode emits",
			config:   GaugeMigration{Default: "gauge"},
			metric:   "metric_a",
			expected: true,
		},
		{
			name:     "migration metric with default none mode does not emit",
			config:   GaugeMigration{Default: "none"},
			metric:   "metric_a",
			expected: false,
		},
		{
			name: "explicit true overrides default none",
			config: GaugeMigration{
				Default: "none",
				Names:   map[string]bool{"metric_a": true},
			},
			metric:   "metric_a",
			expected: true,
		},
		{
			name: "explicit false overrides default gauge",
			config: GaugeMigration{
				Default: "gauge",
				Names:   map[string]bool{"metric_b": false},
			},
			metric:   "metric_b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EmitGauge(tt.metric))
		})
	}
}

func TestCounterMigration_EmitCounter(t *testing.T) {
	orig := CounterMigrationMetrics
	t.Cleanup(func() { CounterMigrationMetrics = orig })

	CounterMigrationMetrics = map[string]struct{}{
		"counter_a": {},
		"counter_b": {},
	}

	tests := []struct {
		name     string
		config   CounterMigration
		metric   string
		expected bool
	}{
		{
			name:     "non-migration counter always emits",
			config:   CounterMigration{},
			metric:   "some_other_counter",
			expected: true,
		},
		{
			name:     "migration counter with default empty mode emits",
			config:   CounterMigration{},
			metric:   "counter_a",
			expected: true,
		},
		{
			name:     "migration counter with default none mode does not emit",
			config:   CounterMigration{Default: "none"},
			metric:   "counter_a",
			expected: false,
		},
		{
			name: "explicit true overrides default none",
			config: CounterMigration{
				Default: "none",
				Names:   map[string]bool{"counter_a": true},
			},
			metric:   "counter_a",
			expected: true,
		},
		{
			name: "explicit false overrides default counter",
			config: CounterMigration{
				Default: "counter",
				Names:   map[string]bool{"counter_b": false},
			},
			metric:   "counter_b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EmitCounter(tt.metric))
		})
	}
}

func TestGaugeMigrationMode_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected GaugeMigrationMode
	}{
		{"gauge", true, "gauge"},
		{"none", true, "none"},
		{"invalid", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var mode GaugeMigrationMode
			err := mode.UnmarshalYAML(func(v any) error {
				*(v.(*string)) = tt.input
				return nil
			})
			if tt.valid {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, mode)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestCounterMigrationMode_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected CounterMigrationMode
	}{
		{"counter", true, "counter"},
		{"none", true, "none"},
		{"invalid", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var mode CounterMigrationMode
			err := mode.UnmarshalYAML(func(v any) error {
				*(v.(*string)) = tt.input
				return nil
			})
			if tt.valid {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, mode)
			} else {
				require.Error(t, err)
			}
		})
	}
}
