package structured

import (
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uber-go/tally"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(func(s tally.Scope) Emitter {
		return Emitter{
			scope: s,
			types: &sync.Map{},
		}
	}),
)

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

	// cached type information for metadata containers
	types *sync.Map
}

// Histogram records a duration-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
//
// `meta` must be either a DynamicTags or a struct with appropriate fields, checked by a linter.
func (b Emitter) Histogram(name string, buckets SubsettableHistogram, dur time.Duration, meta any) {
	tags := b.getTags(meta)
	// all subsettable histograms need to emit scale values so scale changes
	// can be correctly merged at query time.
	tags = tags.With("histogram_scale", strconv.Itoa(buckets.scale))

	if !strings.HasSuffix(name, "_ns") {
		// duration-based histograms are always in nanoseconds,
		// and the name MUST be different from timers while we migrate,
		// so this ensures we always have a unique _ns suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_ns"
	}
	b.scope.Tagged(tags.m).Histogram(name, buckets).RecordDuration(dur)
}

// IntHistogram records a count-based histogram with the provided data.
// It adds a "histogram_scale" tag, so histograms can be accurately subset in queries or via middleware.
//
// `meta` must be either a DynamicTags or a struct with appropriate fields, checked by a linter.
func (b Emitter) IntHistogram(name string, buckets IntSubsettableHistogram, num int, meta any) {
	tags := b.getTags(meta)

	// all subsettable histograms need to emit scale values so scale changes
	// can be correctly merged at query time.
	tags = tags.With("histogram_scale", strconv.Itoa(buckets.scale))

	if !strings.HasSuffix(name, "_counts") {
		// int-based histograms are always in "_counts" (currently anyway),
		// and the name MUST be different from timers while we migrate.
		// so this ensures we always have a unique _counts suffix.
		//
		// hopefully this is never used, but it'll at least make it clear if it is.
		name = name + "_error_missing_suffix_counts"
	}
	b.scope.Tagged(tags.m).Histogram(name, buckets).RecordDuration(time.Duration(num))
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
//
// `meta` must be either a DynamicTags or a struct with appropriate fields, checked by a linter.
func (b Emitter) Count(name string, num int, meta any) {
	b.scope.Tagged(b.getTags(meta).m).Counter(name).Inc(int64(num))
}

// Gauge emits a gauge with the provided data.
//
// `meta` must be either a DynamicTags or a struct with appropriate fields, checked by a linter.
func (b Emitter) Gauge(name string, val float64, meta any) {
	b.scope.Tagged(b.getTags(meta).m).Gauge(name).Update(val)
}

// Tags is a simple read-only map wrapper, to prevent accidental mutation.
//
// The zero value is ready to use, but generally you should be asking the Emitter to create one for you.
type Tags struct{ m map[string]string }

func (t Tags) With(key, value string, pairs ...string) Tags {
	dup := make(map[string]string, len(t.m)+1+(len(pairs)/2))
	maps.Copy(dup, t.m)
	dup[key] = value
	for i := 0; i < len(pairs); i += 2 {
		dup[pairs[i]] = pairs[i+1]
	}
	return Tags{dup}
}

func (b Emitter) TagsFrom(meta any) Tags {
	return b.getTags(meta)
}

type fieldAccess struct {
	index  []int         // path from outer tags-type to the usable field
	name   string        // tag name to use, linter enforces uniqueness per nested structure
	getter reflect.Value // optional getter func, must be func(self)string
}

func (b Emitter) getTags(tags any) Tags {
	if t, ok := tags.(map[string]string); ok {
		return Tags{maps.Clone(t)}
	}
	result := map[string]string{}

	// find the type info
	v := reflect.ValueOf(tags)
	vt := v.Type()
	fields := b.fields(vt)

	// read the field access paths to get the values
	for _, field := range fields {
		ff := v.FieldByIndex(field.index)

		// look for a custom getter
		if field.getter.IsValid() {
			got := field.getter.Call([]reflect.Value{v})
			result[field.name] = got[0].Interface().(string)
			continue
		}

		// must be a primitive type, enforced by the linter and panics if wrong.

		// check for nils, dereference if ptr
		if ff.Kind() == reflect.Ptr {
			if ff.IsNil() {
				result[field.name] = ""
				continue
			}
			ff = ff.Elem()
		}

		// must be a trivial type, check it and convert if necessary
		ffi := ff.Interface()
		switch ffiv := ffi.(type) {
		case int, int32, int64, bool:
			result[field.name] = fmt.Sprintf("%v", ffiv)
		case string:
			result[field.name] = ffiv
		default:
			// prevented by the linter
			panic(fmt.Sprintf("field %q (index %v) on type %T has an unexpected type: %T", field.name, field.index, tags, ffi))
		}
	}

	return Tags{result}
}

// recursively gather types.  higher levels override lower.
func (b Emitter) fields(t reflect.Type) []fieldAccess {
	if val, ok := b.types.Load(t); ok {
		return val.([]fieldAccess)
	}

	var result []fieldAccess
	tagNames := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i) // `t.Field(i)` == `f.Index` always has just one value == `i`

		// embedded field (yes the name is bad)
		if f.Anonymous {
			contains := b.fields(f.Type)
			for _, field := range contains {
				if tagNames[field.name] {
					// duplicates are checked to catch linter bugs, this should
					// be prevented by the linter normally.
					panic(fmt.Sprintf("duplicate tag name on type %v.%v field %v", t.PkgPath(), t.Name(), f.Name))
				}
				tagNames[field.name] = true
				result = append(result, fieldAccess{
					index:  append(f.Index, field.index...),
					name:   field.name,
					getter: field.getter,
				})
			}
			continue
		}

		// check for `struct{}` embedding
		if f.Type.Name() == "" {
			// this is true of all anonymous types, but it seems fine to call this good enough, and let the linter prevent other types.
			// reserved field, ignore
			continue
		}

		// get the tag name
		tagName := f.Tag.Get("tag")
		if tagName == "" {
			// prevented by the linter
			panic(fmt.Sprintf("missing tag on type %v.%v field %v", t.PkgPath(), t.Name(), f.Name))
		}
		tagNames[tagName] = true

		// get the custom getter func on the parent struct, if requested
		var getter reflect.Value
		getterName := f.Tag.Get("getter")
		if getterName != "" {
			// must be a method on the parent type
			getterm, ok := t.MethodByName(getterName)
			if !ok || !getterm.IsExported() {
				// prevented by the linter
				panic(fmt.Sprintf("missing exported getter func %q on type %v.%v field %v", getterName, t.PkgPath(), t.Name(), f.Name))
			}
			// must be the correct type signature
			if getterm.Type.Kind() != reflect.Func ||
				getterm.Type.NumIn() != 1 || getterm.Type.In(0) != t || // instance methods have an implicit self-arg of the receiver type
				getterm.Type.NumOut() != 1 || getterm.Type.Out(0).Kind() != reflect.String {
				// prevented by the linter
				panic(fmt.Sprintf("getter %q on type %v.%v for field %v is invalid, must be a `func() string` method on the type", getterName, t.PkgPath(), t.Name(), f.Name))
			}
			getter = getterm.Func
		}

		// must be a public field or have a public getter
		if !getter.IsValid() && !f.IsExported() {
			panic(fmt.Sprintf("field %v on type %v.%v must be public or have a getter func on the parent struct", f.Name, t.PkgPath(), t.Name()))
		}

		result = append(result, fieldAccess{
			index:  f.Index,
			name:   tagName,
			getter: getter,
		})
	}
	prev, loaded := b.types.LoadOrStore(t, result)
	if loaded {
		return prev.([]fieldAccess)
	}
	return result
}

// NewTestEmitter creates an emitter for tests, optionally using the provided scope.
// If scope is nil, a no-op scope will be used.
func NewTestEmitter(t *testing.T, scope tally.Scope) Emitter {
	t.Name() // require non-nil
	if scope == nil {
		scope = tally.NoopScope
	}
	return Emitter{
		scope: scope,
		types: &sync.Map{},
	}
}
