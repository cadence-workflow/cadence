package structured

import (
	"sync"
	"testing"
	"time"
)

// minimal reproduction of the structured package
type Metadata interface {
	NumTags() int
	PutTags(into map[string]string)
	GetTags() map[string]string
}

type Emitter struct{}

func (e Emitter) Count(name string, value int, meta Metadata)                                      {}
func (e Emitter) Histogram(name string, buckets []time.Duration, dur time.Duration, meta Metadata) {}
func (e Emitter) Gauge(name string, value float64, meta Metadata)                                  {}
func (e Emitter) IntHistogram(name string, buckets []time.Duration, num int, meta Metadata)        {}

// check a simple case, where all the methods are defined on the struct itself (== the codegen)
type TestTags struct {
	Emitter
}

// would be generated
func (t TestTags) NumTags() int                                                      { return 2 }
func (t TestTags) PutTags(into map[string]string)                                    {}
func (t TestTags) GetTags() map[string]string                                        { return nil }
func (t TestTags) Gauge(name string, value float64)                                  {}
func (t TestTags) Count(name string, value int)                                      {}
func (t TestTags) Histogram(name string, buckets []time.Duration, dur time.Duration) {}
func (t TestTags) IntHistogram(name string, buckets []time.Duration, num int)        {}

// would be done by hand
func (t TestTags) SomeOtherMethod(name string) {
	t.Count("constant_name", 1) // want `success: constant_name`
	t.Count(name, 1)            // want `metric names must be in-line strings, not consts or vars: name`
}

// check a harder case, where the methods are embedded
type AnotherTestTags struct {
	TestTags
}

func (a AnotherTestTags) SomeOtherMethod(name string) {
	a.Count("constant_name", 1) // want `success: constant_name`
	a.Count(name, 1)            // want `metric names must be in-line strings, not consts or vars: name`
}

type NoConvenienceTags struct {
	Emitter
}

func (n NoConvenienceTags) NumTags() int                   { return 2 }
func (n NoConvenienceTags) PutTags(into map[string]string) {}
func (n NoConvenienceTags) GetTags() map[string]string     { return nil }
func (n NoConvenienceTags) SomeOtherMethod(name string) {
	// using the embedded emitter via embedding
	n.Count("constant_name", 1, n) // want `success: constant_name`
	n.Count(name, 1, n)            // want `metric names must be in-line strings, not consts or vars: name`
	// using the embedded emitter explicitly
	n.Emitter.Count("constant_name", 1, n) // want `success: constant_name`
	n.Emitter.Count(name, 1, n)            // want `metric names must be in-line strings, not consts or vars: name`
}

type PtrTags struct {
	Emitter
}

func (p *PtrTags) DoThing(value int) {
	// (unintentionally) allowed because Emitter is a value type.
	// this is not great, but it's complicated to block, and may not be worth it.
	p.Emitter.Count("name", value, nil) // want `success: name`
}

type PtrEmbed struct {
	*Emitter
}

func (p *PtrEmbed) DoThing(value int) {
	p.Emitter.Count("name", value, nil) // want `pointer receivers are not allowed on metrics emission calls: p.Emitter.Count`
}

// NonTagsStruct should be ignored (doesn't end with "Tags")
type NonTagsStruct struct {
	Field string
}

func (n NonTagsStruct) Gauge(name string)            {}
func (n NonTagsStruct) Count(name string, value int) {}

// test helper funcs can violate the rules, ignore them.
// they must have *testing.T as the first arg.
func TestHelper(t *testing.T, name string) {
	var emitter Emitter
	emitter.Count(name, 5, nil)
}

func SomeMetricsCalls() {
	var emitter Emitter
	tags := TestTags{}
	anotherTags := AnotherTestTags{}
	nonTags := NonTagsStruct{}

	// valid calls on the emitter
	emitter.Count("test-count", 5, tags)                           // want `success: test-count`
	emitter.Histogram("test-histogram_ns", nil, time.Second, tags) // want `success: test-histogram_ns`

	// valid calls on a struct
	tags.Gauge("tags_counter", 3)                         // want `success: tags_counter`
	tags.Count("tags_count", 10)                          // want `success: tags_count`
	tags.Histogram("tags_histogram_ns", nil, time.Minute) // want `success: tags_histogram_ns`

	// valid calls on an embedding struct
	anotherTags.Gauge("another_counter", 9) // want `success: another_counter`
	anotherTags.Count("another_count", 3)   // want `success: another_count`

	// string vars are not good
	dynamicName := "dynamic-name"
	tags.Gauge(dynamicName, 27)         // want `metric names must be in-line strings, not consts or vars: dynamicName`
	emitter.Count(dynamicName, 1, tags) // want `metric names must be in-line strings, not consts or vars: dynamicName`

	// pre-defined consts are also not good
	// (this is fairly easy to allow though, if strongly desired)
	const sneakyConst = "dynamic-name"
	tags.Gauge(sneakyConst, 1337)       // want `metric names must be in-line strings, not consts or vars: sneakyConst`
	emitter.Count(sneakyConst, 1, tags) // want `metric names must be in-line strings, not consts or vars: sneakyConst`

	// similar-looking calls on non-tags structs are ignored
	nonTags.Gauge("should-be-ignored")
	nonTags.Count(sneakyConst, 1)

	// this call is fine, but not what it does internally
	tags.SomeOtherMethod("ignored")

	// contents of closures must be checked too (ensures we recurse into calls)
	var once sync.Once
	once.Do(func() {
		tags.Gauge(sneakyConst, 9001)                // want `metric names must be in-line strings, not consts or vars: sneakyConst`
		anotherTags.IntHistogram("hist_ns", nil, 42) // want `success: hist_ns`
	})
}
