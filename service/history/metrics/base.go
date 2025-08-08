package metrics

import (
	"maps"
	"math"
	"os"
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
	GetTags() map[string]string  // returns a pre-allocated and pre-populated map
}

// Emitter needs to be an interface, but you get the idea.
// basically the glue code between concepts (histogram, count) and the outputter (tally, custom).
//
// we kinda have this already with the metrics client and the tally wrapper, but they allow too much behavior.
// this is intentionally restricted and very simple to replace, e.g. no sub-scoping or return values at all.
//
// for ease of migration though, this could very easily just be some funcs that accept our metrics client,
// e.g. `func Histogram(client, meta, name, dur)`
// that's already available almost everywhere.
type Emitter struct {
	scope tally.Scope
}

// TODO: embed emitter, and generate short versions that pass self.  worth it for convenience.

func (b Emitter) Histogram(meta Metadata, name string, dur time.Duration) {
	// TODO: need to do something for different buckets.
	// I'd rather avoid actually passing it as an arg, since the precise buckets are unique to tally
	// and the precision should probably be dynamically configurable,
	// but some kind of conceptual "1ms100s" expected-range-key seems probably usable.
	b.scope.Tagged(meta.GetTags()).Histogram(name, defaultBuckets).RecordDuration(dur)
}
func (b Emitter) Count(meta Metadata, name string, num int) {
	b.scope.Tagged(meta.GetTags()).Counter(name).Inc(int64(num))
}
func (b Emitter) Inc(meta Metadata, name string) {
	b.scope.Tagged(meta.GetTags()).Counter(name).Inc(1)
}

// DynamicTags is a very simple helper for treating an arbitrary map as metadata
type DynamicTags map[string]string

var _ Metadata = DynamicTags{}

func (o DynamicTags) NumTags() int                { return len(o) }
func (o DynamicTags) Tags(into map[string]string) { maps.Copy(into, o) }
func (o DynamicTags) GetTags() map[string]string  { return maps.Clone(o) }

//go:generate metricsgen

// ServiceTags is just the most-basic tags we use for everything,
// modeled as a struct for type purposes.
type ServiceTags struct {
	Emitter

	Hostname   string `tag:"host"`
	RuntimeEnv string `tag:"env"`
	//                 ^^^^^^^^ custom struct tag, used by the code generator
	//                          to reduce some of the annoying boilerplate,
	//                          so these tag-types are very easy to create.
}

// UsernameTags shows how to do a custom Tags-impl for runtime-only info that
// does not need to be copied everywhere / must be gathered every time.
//
// skip:Tags <- this text skips the Tags generation, so it can be customized
// skip:Convenience <- skips the convenience methods, because there is no embedded Emitter
type UsernameTags struct {
	// a wholly-dynamic field, no storage used at all.
	// declaring this does reserve room in the map passed to Tags though.
	Username struct{} `tag:"username"`
}

func (u UsernameTags) Tags(into map[string]string) {
	// a bad example, but an example: this retrieves the current USER env var
	// every time tags are collected to emit any metric.
	//
	// doing it this way allows it to be used as a parent without needing to
	// manually populate this field every time.
	into["username"] = os.Getenv("USER")
}
