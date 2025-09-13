package types

import (
	"reflect"
	"testing"
)

type byteSizer interface{ ByteSize() uint64 }

// AssertByteSizeMatchesReflect verifies v.(byteSizer).ByteSize()
// matches an independent reflective deep-size computation.
func AssertByteSizeMatchesReflect(t *testing.T, v any) {
	t.Helper()

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
	if !ok {
		t.Fatalf("%T does not implement ByteSize()", v)
	}

	// Ensure pointer fields are non-nil so new fields are exercised.
	autoPopulateNonNil(rv)

	// Compare impl vs reflect-based computation.
	root := rv
	if root.Kind() == reflect.Ptr {
		root = root.Elem()
	}
	exp := uint64(root.Type().Size()) + structPayloadSize(root)
	got := bs.ByteSize()

	if got != exp {
		t.Fatalf("ByteSize mismatch for %T: got=%d expected=%d (likely a new/changed field not reflected in ByteSize)", v, got, exp)
	}
}

// AssertReachablesImplementByteSize ensures every reachable named struct
// under root implements ByteSize (on value or pointer receiver).
// You can pass a value (&T{}) or a type via (*T)(nil).
func AssertReachablesImplementByteSize(t *testing.T, root any) {
	t.Helper()

	rt := reflect.TypeOf(root)
	if rt == nil {
		t.Fatalf("nil type")
	}
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	seen := map[reflect.Type]bool{}
	var missing []string
	checkStructHasByteSize(rt, seen, &missing)

	if len(missing) > 0 {
		t.Fatalf("reachable struct types missing ByteSize(): %v", missing)
	}
}

// ------------------- helpers -------------------

func autoPopulateNonNil(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		switch f.Kind() {
		case reflect.Ptr:
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
			autoPopulateNonNil(f)
		case reflect.Struct:
			// embedded struct (non-pointer)
			autoPopulateNonNil(f.Addr())
			// Leave slices/maps/strings at zero; both sides treat them equally.
		}
	}
}

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
		if bs, ok := v.Interface().(byteSizer); ok {
			// Trust nested ByteSize(), so the check fails
			// if implementers forget to account for new fields.
			return bs.ByteSize()
		}
		elem := v.Elem()
		if isScalar(elem.Kind()) {
			// pointer-to-scalar: count pointed value
			return uint64(elem.Type().Size())
		}
		// pointer-to-struct/complex: header + payload
		return uint64(elem.Type().Size()) + typePayloadSize(elem)

	case reflect.Struct:
		var sum uint64
		for i := 0; i < v.NumField(); i++ {
			sum += fieldPayloadSize(v.Field(i))
		}
		return sum

	case reflect.String:
		return uint64(v.Len()) // header counted in parent

	case reflect.Slice:
		return slicePayloadSize(v)

	case reflect.Array:
		var sum uint64
		for i := 0; i < v.Len(); i++ {
			sum += typePayloadSize(v.Index(i))
		}
		return sum

	case reflect.Map:
		// Approximate: sum key/value payloads (ignores map bucket overhead)
		var sum uint64
		for _, k := range v.MapKeys() {
			sum += typePayloadSize(k)
			sum += typePayloadSize(v.MapIndex(k))
		}
		return sum

	default:
		return 0 // basic scalar: no extra payload
	}
}

func fieldPayloadSize(f reflect.Value) uint64 {
	switch f.Kind() {
	case reflect.Ptr, reflect.Struct, reflect.Map:
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
