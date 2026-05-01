package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/testing/testdatagen"
)

type byteSizer interface{ ByteSize() uint64 }

// AssertByteSizeMatchesReflect verifies v.(byteSizer).ByteSize()
// matches an independent reflective deep-size computation.
func AssertByteSizeMatchesReflect(t *testing.T, v any) {
	t.Helper()

	gen := testdatagen.New(t)

	for i := 0; i < 100; i++ {

		gen.Fuzz(v)

		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			t.Fatalf("nil/invalid value")
		}

		// Must implement ByteSize (on value or pointer)
		bs, ok := v.(byteSizer)
		if !ok {
			// if they passed a non-pointer, check its pointer receiver too
			if rv.Kind() != reflect.Ptr {
				if pv, ok2 := reflect.New(rv.Type()).Interface().(byteSizer); ok2 {
					bs = pv
					rv = rv.Addr()
					ok = true
				}
			}
		}
		assert.True(t, ok, "%T does not implement ByteSize()", v)

		// Compare impl vs reflect-based computation.
		root := rv
		if root.Kind() == reflect.Ptr {
			root = root.Elem()
		}
		exp := uint64(root.Type().Size()) + structPayloadSize(root)
		got := bs.ByteSize()

		assert.Equal(t, exp, got, "ByteSize mismatch for %T: got=%d expected=%d (likely a new/changed field not reflected in ByteSize)", v, got, exp)
	}
}

// AssertReachablesImplementByteSize ensures every reachable named struct
// under root implements ByteSize (on value or pointer receiver).
// You can pass a value (&T{}) or a type via (*T)(nil).
func AssertReachablesImplementByteSize(t *testing.T, root any) {
	t.Helper()

	rt := reflect.TypeOf(root)

	assert.NotNil(t, rt, "nil type")

	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	seen := map[reflect.Type]bool{}
	var missing []string
	checkStructHasByteSize(rt, seen, &missing)

	assert.Equal(t, 0, len(missing), "reachable struct types missing ByteSize(): %v", missing)
}

// ------------------- helpers -------------------

func structPayloadSize(v reflect.Value) uint64 {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0
		}
		return typePayloadSize(v.Elem())
	}
	return typePayloadSize(v)
}

func typePayloadSize(v reflect.Value) uint64 {
	if !v.IsValid() {
		return 0
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return 0
		}
		// DO NOT call ByteSize() here — keep independence
		// the size calculation is done recursively using reflection to provide an independent check on the ByteSize impl
		// Therefore, the size calculated by reflection is the ground truth and the ByteSize impl must match it
		elem := v.Elem()
		// Add the pointed-to value's header size (since it's a separate allocation)
		// plus its dynamic payload.
		switch elem.Kind() {
		case reflect.Struct, reflect.String, reflect.Slice, reflect.Map, reflect.Array:
			return uint64(elem.Type().Size()) + typePayloadSize(elem)
		default:
			// scalar pointed-to: just the scalar's size
			return uint64(elem.Type().Size())
		}

	case reflect.Struct:
		var sum uint64
		for i := 0; i < v.NumField(); i++ {
			sum += fieldPayloadSize(v.Field(i))
		}
		return sum

	case reflect.String:
		return uint64(v.Len()) // header counted by parent (or pointer case above)

	case reflect.Slice:
		return slicePayloadSize(v)

	case reflect.Array:
		var sum uint64
		for i := 0; i < v.Len(); i++ {
			sum += typePayloadSize(v.Index(i))
		}
		return sum

	case reflect.Map:
		var sum uint64
		for _, k := range v.MapKeys() {
			sum += typePayloadSize(k)
			sum += typePayloadSize(v.MapIndex(k))
		}
		return sum

	default:
		return 0
	}
}

func fieldPayloadSize(f reflect.Value) uint64 {
	switch f.Kind() {
	case reflect.Ptr:
		// pointer slot already in parent header; add referent header+payload
		if f.IsNil() {
			return 0
		}
		elem := f.Elem()
		switch elem.Kind() {
		case reflect.Struct, reflect.String, reflect.Slice, reflect.Map, reflect.Array:
			return uint64(elem.Type().Size()) + typePayloadSize(elem)
		default:
			return uint64(elem.Type().Size())
		}
	case reflect.Struct, reflect.Map:
		return typePayloadSize(f)
	case reflect.String:
		return uint64(f.Len())
	case reflect.Slice:
		return slicePayloadSize(f)
	case reflect.Array:
		var sum uint64
		for i := 0; i < f.Len(); i++ {
			sum += typePayloadSize(f.Index(i))
		}
		return sum
	default:
		return 0
	}
}

func slicePayloadSize(v reflect.Value) uint64 {
	if v.IsNil() {
		return 0
	}
	n := v.Len()
	elem := v.Type().Elem()
	sum := uint64(n) * uint64(elem.Size()) // backing array
	switch elem.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map, reflect.Struct, reflect.Ptr:
		for i := 0; i < n; i++ {
			sum += typePayloadSize(v.Index(i))
		}
	}
	return sum
}

func isScalar(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128:
		return true
	default:
		return false
	}
}

func checkStructHasByteSize(t reflect.Type, seen map[reflect.Type]bool, missing *[]string) {
	switch t.Kind() {
	case reflect.Ptr:
		checkStructHasByteSize(t.Elem(), seen, missing)
		return
	case reflect.Struct:
		if t.Name() != "" {
			if seen[t] {
				return
			}
			seen[t] = true
			// value or pointer receiver ok
			if _, ok := t.MethodByName("ByteSize"); !ok {
				if _, ok := reflect.PtrTo(t).MethodByName("ByteSize"); !ok {
					*missing = append(*missing, t.String())
				}
			}
		}
		for i := 0; i < t.NumField(); i++ {
			checkStructHasByteSize(t.Field(i).Type, seen, missing)
		}
	case reflect.Slice, reflect.Array:
		checkStructHasByteSize(t.Elem(), seen, missing)
	case reflect.Map:
		checkStructHasByteSize(t.Key(), seen, missing)
		checkStructHasByteSize(t.Elem(), seen, missing)
	}
}
