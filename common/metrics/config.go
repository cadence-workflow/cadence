package metrics

import "fmt"

type HistogramMigration struct {
	Default HistogramMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or histogram should be emitted.
	//
	// This is only checked for timers and histograms, and because names are
	// required to be unique per type, you may need to specify both for full
	// control.
	Names map[string]bool `yaml:"names"`
}

func (h HistogramMigration) EmitTimer(name string) bool {
	emit, ok := h.Names[name]
	if ok {
		return emit
	}
	return h.Default.EmitTimer()
}
func (h HistogramMigration) EmitHistogram(name string) bool {
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
