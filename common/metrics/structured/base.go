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

// Tags is simply a map of metric tags, to help differentiate from other maps.
type Tags map[string]string

func (o Tags) With(key, value string, more ...string) Tags {
	if len(more)%2 != 0 {
		// pretty easy to catch at dev time, seems reasonably unlikely to ever happen in prod.
		panic("Tags.With requires an even number of 'more' arguments")
	}

	dup := make(Tags, len(o)+1+len(more)/2)
	maps.Copy(dup, o)
	dup[key] = value
	for i := 0; i < len(more)-1; i += 2 {
		dup[more[i]] = more[i+1]
	}
	return dup
}

// Emitter is the base helper for emitting metrics, and it contains only low-level
// metrics-emitting funcs to keep it as simple as possible.
//
// Calls to metrics methods on this type are checked by internal/tools/metricslint
// to ensure that all metrics are uniquely named, to reduce our risk of collisions
// that break queries / Prometheus / etc.
//
// Tags must be passed in each time to help make it clear what tags are used,
// and you MUST NOT vary the tags per metric name - this breaks Prometheus.
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
//
// Metric names MUST have an "_ns" suffix to avoid confusion with timers,
// and to make it clear they are duration-based histograms.
func (b Emitter) Histogram(name string, buckets SubsettableHistogram, dur time.Duration, meta Tags) {
	tags := make(Tags, len(meta)+3)
	maps.Copy(tags, meta)
	writeHistogramRangeTags(buckets.tallyBuckets, buckets.scale, tags)

	if !strings.HasSuffix(name, "_ns") {
		// duration-based histograms are always in nanoseconds,
		// and the name MUST be different from timers while we migrate,
		// so this ensures we always have a unique _ns suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_ns"
	}
	b.scope.Tagged(tags).Histogram(name, buckets.tallyBuckets).RecordDuration(dur)
}

// IntHistogram records a count-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
func (b Emitter) IntHistogram(name string, buckets IntSubsettableHistogram, num int, meta Tags) {
	tags := make(Tags, len(meta)+3)
	maps.Copy(tags, meta)
	writeHistogramRangeTags(buckets.tallyBuckets, buckets.scale, tags)

	if !strings.HasSuffix(name, "_counts") {
		// int-based histograms are always in "_counts" (currently anyway),
		// and the name MUST be different from timers while we migrate.
		// so this ensures we always have a unique _counts suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_counts"
	}
	b.scope.Tagged(tags).Histogram(name, buckets.tallyBuckets).RecordDuration(time.Duration(num))
}

func writeHistogramRangeTags(buckets []time.Duration, scale int, into Tags) {
	// all subsettable histograms need to emit scale values so scale changes
	// can be correctly merged at query time.
	if _, ok := into["histogram_start"]; ok {
		into["error_rename_this_tag_histogram_start"] = into["histogram_start"]
	}
	if _, ok := into["histogram_end"]; ok {
		into["error_rename_this_tag_histogram_end"] = into["histogram_end"]
	}
	if _, ok := into["histogram_scale"]; ok {
		into["error_rename_this_tag_histogram_scale"] = into["histogram_scale"]
	}
	// record the full range and scale of the histogram so it can be recreated from any individual metric.
	into["histogram_start"] = strconv.Itoa(int(buckets[1]))            // first non-zero bucket
	into["histogram_end"] = strconv.Itoa(int(buckets[len(buckets)-1])) // note this will change if scale changes
	// include the scale, so we know how far away from the requested scale it is, when re-subsetting.
	into["histogram_scale"] = strconv.Itoa(scale)
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
func (b Emitter) Count(name string, num int, meta Tags) {
	b.scope.Tagged(meta).Counter(name).Inc(int64(num))
}

// Gauge emits a gauge with the provided data.
func (b Emitter) Gauge(name string, val float64, meta Tags) {
	b.scope.Tagged(meta).Gauge(name).Update(val)
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
