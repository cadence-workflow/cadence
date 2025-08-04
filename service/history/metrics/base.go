package metrics

import (
	"maps"
	"math"
	"time"

	"github.com/uber-go/tally"
)

var defaultBuckets = append(
	tally.DurationBuckets{0},
	tally.MustMakeExponentialDurationBuckets(
		time.Millisecond,
		math.Pow(2, math.Pow(2, -3)),
		160,
	)...,
)

type Metadata interface {
	NumTags() int                // for efficient pre-allocation
	Tags(into map[string]string) // populates the map
}

type Emitter struct {
	scope tally.Scope
}

func (b Emitter) Histogram(meta Metadata, name string, dur time.Duration) {
	tags := make(map[string]string, meta.NumTags())
	meta.Tags(tags)
	b.scope.Tagged(tags).Histogram(name, defaultBuckets).RecordDuration(dur)
}
func (b Emitter) Count(meta Metadata, name string, num int) {
	tags := make(map[string]string, meta.NumTags())
	meta.Tags(tags)
	b.scope.Tagged(tags).Counter(name).Inc(int64(num))
}

// OnetimeMetric is a very simple helper for treating an arbitrary map as metadata
type OnetimeMetric map[string]string

var _ Metadata = OnetimeMetric{}

func (o OnetimeMetric) NumTags() int                { return len(o) }
func (o OnetimeMetric) Tags(into map[string]string) { maps.Copy(into, o) }

//go:generate metricsgen

// ServiceTags is just the most-basic tags we use for everything,
// modeled as a struct for type purposes.
type ServiceTags struct {
	Hostname   string `tag:"host"`
	RuntimeEnv string `tag:"env"`
	//                 ^^^^^^^^ custom struct tag, used by the code generator
	//                          to reduce some of the annoying boilerplate,
	//                          so these tag-types are very easy to create.
}
