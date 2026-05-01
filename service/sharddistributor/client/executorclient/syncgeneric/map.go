package syncgeneric

import "sync"

// Map is a wrapper around sync.Map that provides a more convenient, generic,
// and type-safe API.
type Map[K comparable, V any] struct {
	contents sync.Map
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *Map[K, V]) Load(key K) (V, bool) {
	val, ok := m.contents.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Store sets the value for a key.
func (m *Map[K, V]) Store(key K, value V) {
	m.contents.Store(key, value)
}

// Delete deletes the value for a key.
func (m *Map[K, V]) Delete(key K) {
	m.contents.Delete(key)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.contents.Range(func(key, value interface{}) bool {
		return f(key.(K), value.(V))
	})
}
