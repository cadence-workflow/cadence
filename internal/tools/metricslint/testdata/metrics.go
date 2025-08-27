package structured

import (
	"sync"
	"testing"
	"time"
)

const disallowedConst = "dynamic-name"

type Emitter struct{}
type Tags map[string]string

func (e Emitter) Count(name string, value int, meta Tags)                                      {}
func (e Emitter) Histogram(name string, buckets []time.Duration, dur time.Duration, meta Tags) {}
func (e Emitter) Gauge(name string, value float64, meta Tags)                                  {}
func (e Emitter) IntHistogram(name string, buckets []time.Duration, num int, meta Tags)        {}

type UnrelatedThing struct{ metricName string }

var _ = UnrelatedThing{metricName: disallowedConst}

func (n UnrelatedThing) Gauge(name string)                       {}
func (n UnrelatedThing) Count(name string, value int, meta Tags) {}

// test helper funcs can violate the rules, ignore them.
// they must have *testing.T as the first arg
func TestHelper(t *testing.T, name string) {
	var emitter Emitter
	emitter.Count(name, 5, nil)
}

func someMetricsCalls() {
	var emitter Emitter
	tags := Tags{"key": "value"}

	// valid calls on the emitter
	emitter.Count("test-count", 5, tags)                           // want `success: test-count`
	emitter.Histogram("test-histogram_ns", nil, time.Second, tags) // want `success: test-histogram_ns`

	// duplicates are not blocked at this stage, it's run separately because of how the analysis framework works
	emitter.Count("duplicate-metric", 1, tags) // want `success: duplicate-metric`
	emitter.Count("duplicate-metric", 2, tags) // want `success: duplicate-metric`

	// wrong suffix
	emitter.Histogram("test-histogram", nil, time.Second, tags) // want `metric name "test-histogram" is not valid for method Histogram, it must have one of the following suffixes: "_ns"`

	// string vars are not good
	dynamicName := "dynamic-name"
	emitter.Count(dynamicName, 1, tags) // want `metric names must be in-line strings, not consts or vars: dynamicName`

	// named consts are also not good
	// (this is fairly easy to allow though, if strongly desired)
	emitter.Count(disallowedConst, 1, tags) // want `metric names must be in-line strings, not consts or vars: disallowedConst`

	var unrelated UnrelatedThing
	name := "asdf"
	unrelated.Gauge(name)           // ignored, not on Emitter
	unrelated.Count(name, 42, tags) // ignored, not on Emitter

	// contents of closures must be checked too (ensures we recurse into calls)
	var once sync.Once
	once.Do(func() {
		emitter.Gauge(disallowedConst, 9001, tags)         // want `metric names must be in-line strings, not consts or vars: disallowedConst`
		emitter.IntHistogram("hist_counts", nil, 42, tags) // want `success: hist_counts`
	})
}
