package metrics

import "fmt"

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
	"task_latency":    {},
	"task_latency_ns": {},

	"task_latency_processing":    {},
	"task_latency_processing_ns": {},

	// Replication task processor histograms (PR #7685).
	// Dual-emitted as timer + histogram.
	"replication_tasks_lag":                {},
	"replication_tasks_lag_ns":             {},
	"replication_tasks_applied_latency":    {},
	"replication_tasks_applied_latency_ns": {},

	"replication_task_latency":    {},
	"replication_task_latency_ns": {},

	"replication_tasks_fetched":        {},
	"replication_tasks_fetched_counts": {},
	"replication_tasks_lag_raw":        {},
	"replication_tasks_lag_raw_counts": {},
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
