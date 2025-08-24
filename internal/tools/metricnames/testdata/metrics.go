package structured

import (
	"sync"
	"time"
)

// minimal reproduction of the structured package
type Metadata interface {
	NumTags() int
	PutTags(into map[string]string)
	GetTags() map[string]string
}

type Emitter struct{}

func (e Emitter) Count(meta Metadata, name string, value int)                                      {}
func (e Emitter) Histogram(meta Metadata, name string, buckets []time.Duration, dur time.Duration) {}
func (e Emitter) Gauge(meta Metadata, name string, value float64)                                  {}
func (e Emitter) IntHistogram(meta Metadata, name string, buckets []time.Duration, num int)        {}

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
	t.Count(name, 1)            // want `metric call with non-inline string argument: name`
}

// check a harder case, where the methods are embedded
type AnotherTestTags struct {
	TestTags
}

func (a AnotherTestTags) SomeOtherMethod(name string) {
	a.Count("constant_name", 1) // want `success: constant_name`
	a.Count(name, 1)            // want `metric call with non-inline string argument: name`
}

type NoConvenienceTags struct {
	Emitter
}

func (n NoConvenienceTags) NumTags() int                   { return 2 }
func (n NoConvenienceTags) PutTags(into map[string]string) {}
func (n NoConvenienceTags) GetTags() map[string]string     { return nil }
func (n NoConvenienceTags) SomeOtherMethod(name string) {
	// using the embedded emitter via embedding
	n.Count(n, "constant_name", 1) // want `success: constant_name`
	n.Count(n, name, 1)            // want `metric call with non-inline string argument: name`
	// using the embedded emitter explicitly
	n.Emitter.Count(n, "constant_name", 1) // want `success: constant_name`
	n.Emitter.Count(n, name, 1)            // want `metric call with non-inline string argument: name`
}

// NonTagsStruct should be ignored (doesn't end with "Tags")
type NonTagsStruct struct {
	Field string
}

func (n NonTagsStruct) Gauge(name string)            {}
func (n NonTagsStruct) Count(name string, value int) {}

func TestMetricCalls() {
	var emitter Emitter
	tags := TestTags{}
	anotherTags := AnotherTestTags{}
	nonTags := NonTagsStruct{}

	// valid calls on the emitter
	emitter.Count(tags, "test-count", 5)                           // want `success: test-count`
	emitter.Histogram(tags, "test-histogram_ns", nil, time.Second) // want `success: test-histogram_ns`

	// valid calls on a struct
	tags.Gauge("tags_counter", 3)                         // want `success: tags_counter`
	tags.Count("tags_count", 10)                          // want `success: tags_count`
	tags.Histogram("tags_histogram_ns", nil, time.Minute) // want `success: tags_histogram_ns`

	// valid calls on an embedding struct
	anotherTags.Gauge("another_counter", 9) // want `success: another_counter`
	anotherTags.Count("another_count", 3)   // want `success: another_count`

	// string vars are not good
	dynamicName := "dynamic-name"
	tags.Gauge(dynamicName, 27)         // want `metric call with non-inline string argument: dynamicName`
	emitter.Count(tags, dynamicName, 1) // want `metric call with non-inline string argument: dynamicName`

	// pre-defined consts are also not good
	// (this is fairly easy to allow though, if strongly desired)
	const sneakyConst = "dynamic-name"
	tags.Gauge(sneakyConst, 1337)       // want `metric call with non-inline string argument: sneakyConst`
	emitter.Count(tags, sneakyConst, 1) // want `metric call with non-inline string argument: sneakyConst`

	// similar-looking calls on non-tags structs are ignored
	nonTags.Gauge("should-be-ignored")
	nonTags.Count(sneakyConst, 1)

	// this call is fine, but not what it does internally
	tags.SomeOtherMethod("ignored")

	// closures must be checked too
	var once sync.Once
	once.Do(func() {
		tags.Gauge(sneakyConst, 9001)                // want `metric call with non-inline string argument: sneakyConst`
		anotherTags.IntHistogram("hist_ns", nil, 42) // want `success: hist_ns`
	})
}
