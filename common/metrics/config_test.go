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
			name:     "migration metric with default empty mode does not emit gauge",
			config:   GaugeMigration{},
			metric:   "metric_a",
			expected: false,
		},
		{
			name:     "migration metric with default timer mode does not emit gauge",
			config:   GaugeMigration{Default: "timer"},
			metric:   "metric_a",
			expected: false,
		},
		{
			name:     "migration metric with default gauge mode emits",
			config:   GaugeMigration{Default: "gauge"},
			metric:   "metric_a",
			expected: true,
		},
		{
			name:     "migration metric with default both mode emits",
			config:   GaugeMigration{Default: "both"},
			metric:   "metric_a",
			expected: true,
		},
		{
			name: "explicit true overrides default timer",
			config: GaugeMigration{
				Default: "timer",
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

func TestGaugeMigration_EmitTimer(t *testing.T) {
	orig := GaugeMigrationMetrics
	t.Cleanup(func() { GaugeMigrationMetrics = orig })

	GaugeMigrationMetrics = map[string]struct{}{
		"metric_a": {},
		"metric_b": {},
	}

	tests := []struct {
		name     string
		config   GaugeMigration
		metric   string
		expected bool
	}{
		{
			name:     "non-migration metric always emits timer",
			config:   GaugeMigration{},
			metric:   "some_other_metric",
			expected: true,
		},
		{
			name:     "migration metric with default empty mode emits timer",
			config:   GaugeMigration{},
			metric:   "metric_a",
			expected: true,
		},
		{
			name:     "migration metric with default timer mode emits timer",
			config:   GaugeMigration{Default: "timer"},
			metric:   "metric_a",
			expected: true,
		},
		{
			name:     "migration metric with default gauge mode does not emit timer",
			config:   GaugeMigration{Default: "gauge"},
			metric:   "metric_a",
			expected: false,
		},
		{
			name:     "migration metric with default both mode emits timer",
			config:   GaugeMigration{Default: "both"},
			metric:   "metric_a",
			expected: true,
		},
		{
			name: "explicit false overrides default timer",
			config: GaugeMigration{
				Default: "timer",
				Names:   map[string]bool{"metric_b": false},
			},
			metric:   "metric_b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EmitTimer(tt.metric))
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
			name:     "migration counter with default empty mode does not emit counter",
			config:   CounterMigration{},
			metric:   "counter_a",
			expected: false,
		},
		{
			name:     "migration counter with default timer mode does not emit counter",
			config:   CounterMigration{Default: "timer"},
			metric:   "counter_a",
			expected: false,
		},
		{
			name:     "migration counter with default counter mode emits",
			config:   CounterMigration{Default: "counter"},
			metric:   "counter_a",
			expected: true,
		},
		{
			name:     "migration counter with default both mode emits",
			config:   CounterMigration{Default: "both"},
			metric:   "counter_a",
			expected: true,
		},
		{
			name: "explicit true overrides default timer",
			config: CounterMigration{
				Default: "timer",
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

func TestCounterMigration_EmitTimer(t *testing.T) {
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
			name:     "non-migration counter always emits timer",
			config:   CounterMigration{},
			metric:   "some_other_counter",
			expected: true,
		},
		{
			name:     "migration counter with default empty mode emits timer",
			config:   CounterMigration{},
			metric:   "counter_a",
			expected: true,
		},
		{
			name:     "migration counter with default timer mode emits timer",
			config:   CounterMigration{Default: "timer"},
			metric:   "counter_a",
			expected: true,
		},
		{
			name:     "migration counter with default counter mode does not emit timer",
			config:   CounterMigration{Default: "counter"},
			metric:   "counter_a",
			expected: false,
		},
		{
			name:     "migration counter with default both mode emits timer",
			config:   CounterMigration{Default: "both"},
			metric:   "counter_a",
			expected: true,
		},
		{
			name: "explicit false overrides default timer",
			config: CounterMigration{
				Default: "timer",
				Names:   map[string]bool{"counter_b": false},
			},
			metric:   "counter_b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EmitTimer(tt.metric))
		})
	}
}

func TestMigrationConfig_EmitTimer(t *testing.T) {
	origH := HistogramMigrationMetrics
	origG := GaugeMigrationMetrics
	origC := CounterMigrationMetrics
	t.Cleanup(func() {
		HistogramMigrationMetrics = origH
		GaugeMigrationMetrics = origG
		CounterMigrationMetrics = origC
	})

	HistogramMigrationMetrics = map[string]struct{}{"histogram_metric": {}, "multi_metric": {}}
	GaugeMigrationMetrics = map[string]struct{}{"gauge_metric": {}, "multi_metric": {}}
	CounterMigrationMetrics = map[string]struct{}{"counter_metric": {}, "multi_metric": {}}

	tests := []struct {
		name     string
		config   MigrationConfig
		metric   string
		expected bool
	}{
		{
			name:     "metric not in any migration map always emits timer",
			config:   MigrationConfig{},
			metric:   "some_other_metric",
			expected: true,
		},
		{
			name:     "histogram metric with default empty mode emits timer",
			config:   MigrationConfig{},
			metric:   "histogram_metric",
			expected: true,
		},
		{
			name: "histogram metric with histogram mode suppresses timer",
			config: MigrationConfig{
				Histogram: HistogramMigration{Default: "histogram"},
			},
			metric:   "histogram_metric",
			expected: false,
		},
		{
			name: "histogram metric with both mode emits timer",
			config: MigrationConfig{
				Histogram: HistogramMigration{Default: "both"},
			},
			metric:   "histogram_metric",
			expected: true,
		},
		{
			name:     "gauge metric with default empty mode emits timer",
			config:   MigrationConfig{},
			metric:   "gauge_metric",
			expected: true,
		},
		{
			name: "gauge metric with gauge mode suppresses timer",
			config: MigrationConfig{
				Gauge: GaugeMigration{Default: "gauge"},
			},
			metric:   "gauge_metric",
			expected: false,
		},
		{
			name: "gauge metric with both mode emits timer",
			config: MigrationConfig{
				Gauge: GaugeMigration{Default: "both"},
			},
			metric:   "gauge_metric",
			expected: true,
		},
		{
			name:     "counter metric with default empty mode emits timer",
			config:   MigrationConfig{},
			metric:   "counter_metric",
			expected: true,
		},
		{
			name: "counter metric with counter mode suppresses timer",
			config: MigrationConfig{
				Counter: CounterMigration{Default: "counter"},
			},
			metric:   "counter_metric",
			expected: false,
		},
		{
			name: "counter metric with both mode emits timer",
			config: MigrationConfig{
				Counter: CounterMigration{Default: "both"},
			},
			metric:   "counter_metric",
			expected: true,
		},
		{
			name: "histogram and gauge suppress timer, counter allows: result false",
			config: MigrationConfig{
				Histogram: HistogramMigration{Default: "histogram"},
				Gauge:     GaugeMigration{Default: "gauge"},
			},
			metric:   "multi_metric",
			expected: false,
		},
		{
			name: "histogram and counter suppress timer, gauge allows: result false",
			config: MigrationConfig{
				Histogram: HistogramMigration{Default: "histogram"},
				Counter:   CounterMigration{Default: "counter"},
			},
			metric:   "multi_metric",
			expected: false,
		},
		{
			name: "gauge and counter suppress timer, histogram allows: result false",
			config: MigrationConfig{
				Gauge:   GaugeMigration{Default: "gauge"},
				Counter: CounterMigration{Default: "counter"},
			},
			metric:   "multi_metric",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.EmitTimer(tt.metric))
		})
	}
}

func TestGaugeMigrationMode_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected GaugeMigrationMode
	}{
		{"timer", true, "timer"},
		{"gauge", true, "gauge"},
		{"both", true, "both"},
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
		{"timer", true, "timer"},
		{"counter", true, "counter"},
		{"both", true, "both"},
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
