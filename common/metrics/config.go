package metrics

import "fmt"

// MigrationConfig groups all metric migration configurations.
// Future migration types can be added here without changing NewClient's signature.
type MigrationConfig struct {
	Histogram HistogramMigration
	Gauge     GaugeMigration
	Counter   CounterMigration
}

// EmitTimer returns true if the metric should be emitted as a timer.
// A metric is suppressed from timer emission if the migration that owns it
// has been configured to move away from timer.
// Metrics not in any migration map are always emitted.
func (mc MigrationConfig) EmitTimer(name string) bool {
	return mc.Histogram.EmitTimer(name) &&
		mc.Gauge.EmitTimer(name) &&
		mc.Counter.EmitTimer(name)
}

type HistogramMigration struct {
	Default HistogramMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or histogram should be emitted.
	//
	// This is only checked for timers and histograms that are in HistogramMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (h *HistogramMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType HistogramMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := HistogramMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown histogram-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.HistogramMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*h = HistogramMigration(tmp)
	return nil
}

// HistogramMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and histograms, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var HistogramMigrationMetrics = map[string]struct{}{
	"task_attempt":                          {},
	"task_attempt_counts":                   {},
	"task_attempt_per_domain":               {},
	"task_attempt_per_domain_counts":        {},
	"task_latency":                          {},
	"task_latency_ns":                       {},
	"task_latency_per_domain":               {},
	"task_latency_per_domain_ns":            {},
	"task_latency_processing":               {},
	"task_latency_processing_ns":            {},
	"task_latency_queue":                    {},
	"task_latency_queue_ns":                 {},
	"task_latency_processing_per_domain":    {},
	"task_latency_processing_per_domain_ns": {},
	"task_latency_queue_per_domain":         {},
	"task_latency_queue_per_domain_ns":      {},

	"replication_tasks_lag":                {},
	"replication_tasks_lag_counts":         {},
	"replication_tasks_applied_latency":    {},
	"replication_tasks_applied_latency_ns": {},

	"cache_latency":     {},
	"cache_latency_ns":  {},
	"cache_size":        {},
	"cache_size_counts": {},

	"replication_task_latency":    {},
	"replication_task_latency_ns": {},

	"replication_tasks_returned":             {},
	"replication_tasks_returned_counts":      {},
	"replication_tasks_returned_diff":        {},
	"replication_tasks_returned_diff_counts": {},

	"replication_tasks_fetched":        {},
	"replication_tasks_fetched_counts": {},
	"replication_tasks_lag_raw":        {},
	"replication_tasks_lag_raw_counts": {},

	"activity_end_to_end_latency":    {},
	"activity_end_to_end_latency_ns": {},

	"schedule_to_start_history_queue_latency_per_tl":    {},
	"schedule_to_start_history_queue_latency_per_tl_ns": {},

	"processing_queue_num":        {},
	"processing_queue_num_counts": {},

	"processing_queue_max_level":        {},
	"processing_queue_max_level_counts": {},

	"persistence_latency_per_domain":    {},
	"persistence_latency_per_domain_ns": {},
	"persistence_latency":               {},
	"persistence_latency_ns":            {},
	"persistence_latency_histogram":     {},

	"persistence_latency_per_shard":    {},
	"persistence_latency_per_shard_ns": {},

	"elasticsearch_latency_per_domain":    {},
	"elasticsearch_latency_per_domain_ns": {},
	"elasticsearch_latency":               {},
	"elasticsearch_latency_ns":            {},
}

func (h HistogramMigration) EmitTimer(name string) bool {
	if _, ok := HistogramMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := h.Names[name]
	if ok {
		return emit
	}
	return h.Default.EmitTimer()
}
func (h HistogramMigration) EmitHistogram(name string) bool {
	if _, ok := HistogramMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := h.Names[name]
	if ok {
		return emit
	}
	return h.Default.EmitHistogram()
}

// HistogramMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the HistogramMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have histograms available, and will
// eventually be fully deprecated and removed.
type HistogramMigrationMode string

func (h *HistogramMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read histogram migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "histogram", "both":
		*h = HistogramMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported histogram migration mode %q, must be "timer", "histogram", or "both"`, value)
	}
	return nil
}

func (h HistogramMigrationMode) EmitTimer() bool {
	switch h {
	case "timer", "both", "": // default == not specified == both
		return true
	default:
		return false
	}
}

func (h HistogramMigrationMode) EmitHistogram() bool {
	switch h {
	case "histogram", "both": // default == not specified == both
		return true
	default:
		return false
	}
}

type GaugeMigration struct {
	Default GaugeMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or gauge should be emitted.
	//
	// This is only checked for timers and gauges that are in GaugeMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (g *GaugeMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType GaugeMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := GaugeMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown gauge-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.GaugeMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*g = GaugeMigration(tmp)
	return nil
}

// GaugeMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and gauges, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var GaugeMigrationMetrics = map[string]struct{}{}

func (g GaugeMigration) EmitTimer(name string) bool {
	if _, ok := GaugeMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := g.Names[name]
	if ok {
		return emit
	}
	return g.Default.EmitTimer()
}
func (g GaugeMigration) EmitGauge(name string) bool {
	if _, ok := GaugeMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := g.Names[name]
	if ok {
		return emit
	}
	return g.Default.EmitGauge()
}

// GaugeMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the GaugeMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have gauges available, and will
// eventually be fully deprecated and removed.
type GaugeMigrationMode string

func (g *GaugeMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read gauge migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "gauge", "both":
		*g = GaugeMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported gauge migration mode %q, must be "timer", "gauge", or "both"`, value)
	}
	return nil
}

func (g GaugeMigrationMode) EmitTimer() bool {
	switch g {
	case "timer", "both", "": // default == not specified == timer
		return true
	default:
		return false
	}
}

func (g GaugeMigrationMode) EmitGauge() bool {
	switch g {
	case "gauge", "both": // default == not specified == timer
		return true
	default:
		return false
	}
}

type CounterMigration struct {
	Default CounterMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or counter should be emitted.
	//
	// This is only checked for timers and counters that are in CounterMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (c *CounterMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType CounterMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := CounterMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown counter-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.CounterMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*c = CounterMigration(tmp)
	return nil
}

// CounterMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and counters, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var CounterMigrationMetrics = map[string]struct{}{}

func (c CounterMigration) EmitTimer(name string) bool {
	if _, ok := CounterMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := c.Names[name]
	if ok {
		return emit
	}
	return c.Default.EmitTimer()
}
func (c CounterMigration) EmitCounter(name string) bool {
	if _, ok := CounterMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := c.Names[name]
	if ok {
		return emit
	}
	return c.Default.EmitCounter()
}

// CounterMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the CounterMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have counters available, and will
// eventually be fully deprecated and removed.
type CounterMigrationMode string

func (c *CounterMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read counter migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "counter", "both":
		*c = CounterMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported counter migration mode %q, must be "timer", "counter", or "both"`, value)
	}
	return nil
}

func (c CounterMigrationMode) EmitTimer() bool {
	switch c {
	case "timer", "both", "": // default == not specified == timer
		return true
	default:
		return false
	}
}

func (c CounterMigrationMode) EmitCounter() bool {
	switch c {
	case "counter", "both": // default == not specified == timer
		return true
	default:
		return false
	}
}
