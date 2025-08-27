package structured

import (
	"maps"
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

// Tags is an immutable map of strings, to prevent accidentally mutating shared vars.
type Tags struct{ m map[string]string }

func TagsFromMap(m map[string]string) Tags {
	return Tags{m}
}

// With makes a copy with the added key/value pair(s), overriding any conflicting keys.
func (o Tags) With(key, value string, more ...string) Tags {
	if len(more)%2 != 0 {
		// pretty easy to catch at dev time, seems reasonably unlikely to ever happen in prod.
		panic("Tags.With requires an even number of 'more' arguments")
	}

	dup := Tags{make(map[string]string, len(o.m)+1+len(more)/2)}
	maps.Copy(dup.m, o.m)
	dup.m[key] = value
	for i := 0; i < len(more)-1; i += 2 {
		dup.m[more[i]] = more[i+1]
	}
	return dup
}

// Emitter is essentially our new metrics.Client, but with a much smaller API surface
// to make it easier to reason about, lint, and fully disconnect from Tally details later.
//
// Calls to metrics methods on this type are checked by internal/tools/metricslint
// to ensure that all metrics are uniquely named, to reduce our risk of collisions
// that break queries / Prometheus / etc.
//
// Tags must be passed in each time to help make it clear what tags are used,
// and you MUST NOT vary the tags per metric name - this breaks Prometheus.
type Emitter struct {
	// intentionally NOT no-op by default.  use a test emitter in tests.
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
func (b Emitter) Histogram(name string, buckets SubsettableHistogram, dur time.Duration, tags Tags) {
	histogramTags := make(map[string]string, len(tags.m)+3)
	maps.Copy(histogramTags, tags.m)
	buckets.writeTags(name, histogramTags, b)

	if !strings.HasSuffix(name, "_ns") {
		// duration-based histograms are always in nanoseconds,
		// and the name MUST be different from timers while we migrate,
		// so this ensures we always have a unique _ns suffix.
		//
		// this suffix is also checked in the linter, to change the allowed
		// suffix(es) just make sure you update both.
		b.scope.Tagged(map[string]string{"bad_metric_name": name}).Counter("incorrect_histogram_metric_name").Inc(1)
		name = name + "_ns"
	}
	b.scope.Tagged(histogramTags).Histogram(name, buckets.tallyBuckets).RecordDuration(dur)
}

// IntHistogram records a count-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
func (b Emitter) IntHistogram(name string, buckets IntSubsettableHistogram, num int, tags Tags) {
	histogramTags := make(map[string]string, len(tags.m)+3)
	maps.Copy(histogramTags, tags.m)
	buckets.writeTags(name, histogramTags, b)

	if !strings.HasSuffix(name, "_counts") {
		// same as duration suffix.
		// this suffix is also checked in the linter, to change the allowed
		// suffix(es) just make sure you update both.
		b.scope.Tagged(map[string]string{"bad_metric_name": name}).Counter("incorrect_int_histogram_metric_name").Inc(1)
		name = name + "_counts"
	}
	b.scope.Tagged(histogramTags).Histogram(name, buckets.tallyBuckets).RecordDuration(time.Duration(num))
}

func (b Emitter) assertNoTag(key string, errorName string, in map[string]string) {
	if _, ok := in[key]; ok {
		b.scope.Tagged(map[string]string{"bad_key": key}).Counter(errorName).Inc(1)
	}
}

// TODO: make a MinMaxGauge helper which maintains a precise, rolling
// min/max gauge, over a window larger than the metrics granularity (e.g. ~20s)
// to work around gauges' last-data-only behavior.
//
// This will likely require some additional state though, and might benefit from
// keeping that state further up the Tags-stack to keep contention and
// series-deduplication-costs low.
//
// Maybe OTEL / Prometheus will natively support this one day.  It'd be simple.

// Count records a counter with the provided data.
func (b Emitter) Count(name string, num int, tags Tags) {
	b.scope.Tagged(tags.m).Counter(name).Inc(int64(num))
}

// Gauge emits a gauge with the provided data.
func (b Emitter) Gauge(name string, val float64, tags Tags) {
	b.scope.Tagged(tags.m).Gauge(name).Update(val)
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
