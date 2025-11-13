package syncgeneric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testKey struct {
	str string
	i   int
}

type testValue struct {
	m map[string]string
}

var (
	testKey1   = &testKey{str: "key1", i: 1}
	testKey2   = &testKey{str: "key2", i: 2}
	testValue1 = &testValue{m: map[string]string{"key1": "value1"}}
	testValue2 = &testValue{m: map[string]string{"key2": "value2"}}
)

func testMap(t *testing.T) *Map[*testKey, *testValue] {

	// Create the type-safe map
	m := &Map[*testKey, *testValue]{}

	// Insert some data into the underlying sync.Map
	m.contents.Store(testKey1, testValue1)
	m.contents.Store(testKey2, testValue2)
	loaded, ok := m.contents.Load(testKey1)
	require.True(t, ok)
	require.Equal(t, testValue1, loaded)
	loaded, ok = m.contents.Load(testKey2)
	require.True(t, ok)
	require.Equal(t, testValue2, loaded)

	return m
}

func TestStoreAndLoad(t *testing.T) {
	cases := map[string]struct {
		key   *testKey
		value *testValue
	}{
		"new key":      {key: &testKey{str: "key3", i: 1}, value: &testValue{m: map[string]string{"key3": "value3"}}},
		"existing key": {key: testKey1, value: testValue1},
		"nil key":      {key: nil, value: &testValue{m: map[string]string{"key1": "value1-updated"}}},
		"nil value":    {key: &testKey{str: "key1", i: 1}, value: nil},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := testMap(t)
			m.Store(c.key, c.value)
			loaded, ok := m.Load(c.key)
			assert.True(t, ok)
			assert.Equal(t, c.value, loaded)
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("load existing key", func(t *testing.T) {
		m := testMap(t)
		loaded, ok := m.Load(testKey1)
		assert.True(t, ok)
		assert.Equal(t, testValue1, loaded)
	})
	t.Run("load non-existing key", func(t *testing.T) {
		m := testMap(t)
		loaded, ok := m.Load(&testKey{str: "key3", i: 1})
		assert.False(t, ok)
		assert.Nil(t, loaded)
	})
	t.Run("load non-existing key with zero value", func(t *testing.T) {
		m := Map[int, string]{}
		loaded, ok := m.Load(0)
		assert.False(t, ok)
		// Should return the zero value of the type
		assert.Equal(t, "", loaded)
	})
	t.Run("load non-existing key with other zero value", func(t *testing.T) {
		type s struct {
			_ string
			_ int
			_ *int
		}
		m := Map[int, s]{}
		loaded, ok := m.Load(0)
		assert.False(t, ok)
		assert.Equal(t, s{}, loaded)
	})
}

func TestDelete(t *testing.T) {
	m := testMap(t)
	// testKey1 is in the map
	loaded, ok := m.Load(testKey1)
	assert.True(t, ok)
	assert.Equal(t, testValue1, loaded)

	// delete testKey1
	m.Delete(testKey1)

	// testKey1 is no longer in the map
	loaded, ok = m.Load(testKey1)
	assert.False(t, ok)
	assert.Nil(t, loaded)
}

func TestRange(t *testing.T) {
	m := testMap(t)

	// We have 2 keys in the map
	count := 0
	m.Range(func(key *testKey, value *testValue) bool {
		count++
		return true
	})
	assert.Equal(t, 2, count)
}
