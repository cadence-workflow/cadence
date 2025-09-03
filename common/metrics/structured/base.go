package structured

import (
	"maps"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(func(s tally.Scope) Emitter {
		return Emitter{scope: s}
	}),
)

// Metadata is a shared interface for all "...Tags" structs.
//
// You are generally NOT expected to implement any of this yourself.
// Just define your struct, and let the code generator take care of it (`make metrics`).
//
// For the intended usage and implementation, see generated code.
type Metadata interface {
	NumTags() int                   // for efficient pre-allocation
	PutTags(into map[string]string) // populates the map
	GetTags() map[string]string     // returns a pre-allocated and pre-populated map
}

// DynamicTags is a very simple helper for treating an arbitrary map as a Metadata.
//
// This can be used externally (for completely manual metrics) or in metrics-emitting
// methods to simplify adding custom tags (e.g. it is returned from GetTags).
type DynamicTags map[string]string

var _ Metadata = DynamicTags{}

func (o DynamicTags) NumTags() int                   { return len(o) }
func (o DynamicTags) PutTags(into map[string]string) { maps.Copy(into, o) }
func (o DynamicTags) GetTags() map[string]string     { return maps.Clone(o) }

// Emitter is the base helper for emitting metrics, and it contains only low-level
// metrics-emitting funcs to keep it as simple as possible.
//
// It is intended to be used with the `make metrics` code generator and structs-of-tags,
// but it's intentionally possible to (ab)use it by hand because ad-hoc metrics
// should be easy and encouraged.
//
// Metadata can be constructed from any map via DynamicTags, but this API intentionally hides
// [tally.Scope.Tagged] because it's (somewhat) memory-wasteful, self-referential interfaces are
// difficult to mock, and it's very hard to figure out what tags may be present at runtime.
//
// TODO: this can / likely should be turned into an interface to allow disconnecting from tally,
// to allow providing a specific version or to drop it entirely if desired.
type Emitter struct {
	// intentionally NOT no-op by default.
	//
	// use a test emitter in tests, it should be quite easy to construct,
	// and this way it will panic if forgotten for some reason, rather than
	// causing a misleading lack-of-metrics.
	//
	// currently, because this is constructed by common/config/metrics.go,
	// this scope already contains the `cadence_service:cadence-{whatever}` tag,
	// but essentially no others (aside from platform-level stuff).
	// you can get the instance from go.uber.org/fx, as just `tally.Scope`.
	scope tally.Scope
}

// Histogram records a duration-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
func (b Emitter) Histogram(name string, buckets SubsettableHistogram, dur time.Duration, meta Metadata) {
	tags := make(DynamicTags, meta.NumTags()+1)
	meta.PutTags(tags)

	// all subsettable histograms need to emit scale values so scale changes
	// can be correctly merged at query time.
	if _, ok := tags["histogram_scale"]; ok {
		// rewrite the existing tag so it can be noticed
		tags["error_rename_this_tag_histogram_scale"] = tags["histogram_scale"]
	}
	tags["histogram_scale"] = strconv.Itoa(buckets.scale)

	if !strings.HasSuffix(name, "_ns") {
		// duration-based histograms are always in nanoseconds,
		// and the name MUST be different from timers while we migrate,
		// so this ensures we always have a unique _ns suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_ns"
	}
	b.scope.Tagged(tags).Histogram(name, buckets).RecordDuration(dur)
}

// IntHistogram records a count-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
func (b Emitter) IntHistogram(name string, buckets IntSubsettableHistogram, num int, meta Metadata) {
	tags := make(DynamicTags, meta.NumTags()+1)
	meta.PutTags(tags)

	// all subsettable histograms need to emit scale values so scale changes
	// can be correctly merged at query time.
	if _, ok := tags["histogram_scale"]; ok {
		// rewrite the existing tag so it can be noticed
		tags["error_rename_this_tag_histogram_scale"] = tags["histogram_scale"]
	}
	tags["histogram_scale"] = strconv.Itoa(buckets.scale)

	if !strings.HasSuffix(name, "_counts") {
		// int-based histograms are always in "_counts" (currently anyway),
		// and the name MUST be different from timers while we migrate.
		// so this ensures we always have a unique _counts suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_counts"
	}
	b.scope.Tagged(tags).Histogram(name, buckets).RecordDuration(time.Duration(num))
}

// TODO: make a MinMaxHistogram helper which maintains a precise, rolling
// min/max gauge, over a window larger than the metrics granularity (e.g. ~20s)
// to work around gauges' last-data-only behavior.
//
// This will likely require some additional state though, and might benefit from
// keeping that state further up the Tags-stack to keep contention and
// series-deduplication-costs low.
//
// Maybe OTEL / Prometheus will natively support this one day.  It'd be simple.

// Count records a counter with the provided data.
func (b Emitter) Count(name string, num int, meta Metadata) {
	b.scope.Tagged(meta.GetTags()).Counter(name).Inc(int64(num))
}

// Gauge emits a gauge with the provided data.
func (b Emitter) Gauge(name string, val float64, meta Metadata) {
	b.scope.Tagged(meta.GetTags()).Gauge(name).Update(val)
}

// NewTestEmitter creates an emitter for tests, optionally using the provided scope.
// If scope is nil, a no-op scope will be used.
func NewTestEmitter(t *testing.T, scope tally.Scope) Emitter {
	t.Name() // require non-nil
	if scope == nil {
		scope = tally.NoopScope
	}
	return Emitter{scope}
}
