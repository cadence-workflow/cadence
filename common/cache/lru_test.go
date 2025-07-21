// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

type keyType struct {
	dummyString string
	dummyInt    int
}

func TestLRU(t *testing.T) {
	cache := New(&Options{MaxCount: 5})

	cache.Put("A", "Foo")
	assert.Equal(t, "Foo", cache.Get("A"))
	assert.Nil(t, cache.Get("B"))
	assert.Equal(t, 1, cache.Size())

	cache.Put("B", "Bar")
	cache.Put("C", "Cid")
	cache.Put("D", "Delt")
	assert.Equal(t, 4, cache.Size())

	assert.Equal(t, "Bar", cache.Get("B"))
	assert.Equal(t, "Cid", cache.Get("C"))
	assert.Equal(t, "Delt", cache.Get("D"))

	cache.Put("A", "Foo2")
	assert.Equal(t, "Foo2", cache.Get("A"))

	cache.Put("E", "Epsi")
	assert.Equal(t, "Epsi", cache.Get("E"))
	assert.Equal(t, "Foo2", cache.Get("A"))

	// Access C, D is now LRU
	cache.Get("C")
	cache.Put("F", "Felp")
	assert.Nil(t, cache.Get("B"))
	assert.Equal(t, 5, cache.Size())

	cache.Delete("C")
	assert.Nil(t, cache.Get("C"))
}

func TestGenerics(t *testing.T) {
	key := keyType{
		dummyString: "some random key",
		dummyInt:    59,
	}
	value := "some random value"

	cache := New(&Options{MaxCount: 5})
	cache.Put(key, value)

	assert.Equal(t, value, cache.Get(key))
	assert.Equal(t, value, cache.Get(keyType{
		dummyString: "some random key",
		dummyInt:    59,
	}))
	assert.Nil(t, cache.Get(keyType{
		dummyString: "some other random key",
		dummyInt:    56,
	}))
}

func TestLRUWithTTL(t *testing.T) {
	mockTimeSource := clock.NewMockedTimeSourceAt(time.UnixMilli(0))
	cache := New(&Options{
		MaxCount:   5,
		TTL:        time.Millisecond * 100,
		TimeSource: mockTimeSource,
	}).(*lru)

	cache.Put("A", "foo")
	assert.Equal(t, "foo", cache.Get("A"))

	mockTimeSource.Advance(time.Millisecond * 300)

	assert.Nil(t, cache.Get("A"))
	assert.Equal(t, 0, cache.Size())
}

func TestLRUCacheConcurrentAccess(t *testing.T) {
	cache := New(&Options{MaxCount: 5})
	values := map[string]string{
		"A": "foo",
		"B": "bar",
		"C": "zed",
		"D": "dank",
		"E": "ezpz",
	}

	for k, v := range values {
		cache.Put(k, v)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)

		// concurrent get and put
		go func() {
			defer wg.Done()

			<-start

			for j := 0; j < 1000; j++ {
				cache.Get("A")
				cache.Put("A", "fooo")
			}
		}()

		// concurrent iteration
		go func() {
			defer wg.Done()

			<-start

			for j := 0; j < 50; j++ {
				var result []Entry
				it := cache.Iterator()
				for it.HasNext() {
					entry := it.Next()
					result = append(result, entry) //nolint:staticcheck
				}
				it.Close()
			}
		}()
	}

	close(start)
	wg.Wait()
}

func TestRemoveFunc(t *testing.T) {
	ch := make(chan bool)
	cache := New(&Options{
		MaxCount: 5,
		RemovedFunc: func(i interface{}) {
			_, ok := i.(*testing.T)
			assert.True(t, ok)
			ch <- true
		},
	})

	cache.Put("testing", t)
	cache.Delete("testing")
	assert.Nil(t, cache.Get("testing"))

	timeout := time.NewTimer(time.Millisecond * 300)
	select {
	case b := <-ch:
		assert.True(t, b)
	case <-timeout.C:
		t.Error("RemovedFunc did not send true on channel ch")
	}
}

func TestRemovedFuncWithTTL(t *testing.T) {
	ch := make(chan bool)
	mockTimeSource := clock.NewMockedTimeSourceAt(time.UnixMilli(0))
	cache := New(&Options{
		MaxCount: 5,
		TTL:      time.Millisecond * 50,
		RemovedFunc: func(i interface{}) {
			_, ok := i.(*testing.T)
			assert.True(t, ok)
			ch <- true
		},
		TimeSource: mockTimeSource,
	}).(*lru)

	cache.Put("A", t)
	assert.Equal(t, t, cache.Get("A"))

	mockTimeSource.Advance(time.Millisecond * 100)

	assert.Nil(t, cache.Get("A"))

	select {
	case b := <-ch:
		assert.True(t, b)
	case <-mockTimeSource.After(100 * time.Millisecond):
		t.Error("RemovedFunc did not send true on channel ch")
	}
}

func TestRemovedFuncWithTTL_Pin(t *testing.T) {
	ch := make(chan bool)
	mockTimeSource := clock.NewMockedTimeSourceAt(time.UnixMilli(0))
	cache := New(&Options{
		MaxCount: 5,
		TTL:      time.Millisecond * 50,
		Pin:      true,
		RemovedFunc: func(i interface{}) {
			_, ok := i.(*testing.T)
			assert.True(t, ok)
			ch <- true
		},
		TimeSource: mockTimeSource,
	}).(*lru)

	_, err := cache.PutIfNotExist("A", t)
	assert.NoError(t, err)
	assert.Equal(t, t, cache.Get("A"))
	mockTimeSource.Advance(time.Millisecond * 100)
	assert.Equal(t, t, cache.Get("A"))
	// release 3 time since put if not exist also increase the counter
	cache.Release("A")
	cache.Release("A")
	cache.Release("A")
	assert.Nil(t, cache.Get("A"))

	select {
	case b := <-ch:
		assert.True(t, b)
	case <-mockTimeSource.After(300 * time.Millisecond):
		t.Error("RemovedFunc did not send true on channel ch")
	}
}

func TestIterator(t *testing.T) {
	expected := map[string]string{
		"A": "Alpha",
		"B": "Beta",
		"G": "Gamma",
		"D": "Delta",
	}

	cache := New(&Options{MaxCount: 5})

	for k, v := range expected {
		cache.Put(k, v)
	}

	actual := map[string]string{}

	it := cache.Iterator()
	for it.HasNext() {
		entry := it.Next()
		actual[entry.Key().(string)] = entry.Value().(string)
	}
	it.Close()
	assert.Equal(t, expected, actual)

	it = cache.Iterator()
	for i := 0; i < len(expected); i++ {
		entry := it.Next()
		actual[entry.Key().(string)] = entry.Value().(string)
	}
	it.Close()
	assert.Equal(t, expected, actual)
}

// Move the struct definition and method outside the test function
type sizeableValue struct {
	val  string
	size uint64
}

func (s sizeableValue) ByteSize() uint64 {
	return s.size
}

func TestLRU_SizeBased_SizeExceeded(t *testing.T) {
	cache := New(&Options{
		IsSizeBased: dynamicproperties.GetBoolPropertyFn(true),
		MaxSize:     dynamicproperties.GetIntPropertyFn(15),
	})

	fooValue := sizeableValue{val: "Foo", size: 5}
	cache.Put("A", fooValue)
	assert.Equal(t, fooValue, cache.Get("A"))
	assert.Nil(t, cache.Get("B"))
	assert.Equal(t, 1, cache.Size())

	barValue := sizeableValue{val: "Bar", size: 5}
	cidValue := sizeableValue{val: "Cid", size: 5}
	deltValue := sizeableValue{val: "Delt", size: 5}

	cache.Put("B", barValue)
	cache.Put("C", cidValue)
	cache.Put("D", deltValue)
	assert.Nil(t, cache.Get("A"))
	assert.Equal(t, 3, cache.Size())

	assert.Equal(t, barValue, cache.Get("B"))
	assert.Equal(t, cidValue, cache.Get("C"))
	assert.Equal(t, deltValue, cache.Get("D"))

	foo2Value := sizeableValue{val: "Foo2", size: 5}
	cache.Put("A", foo2Value)
	assert.Equal(t, foo2Value, cache.Get("A"))
	assert.Nil(t, cache.Get("B"))
	assert.Equal(t, 3, cache.Size())

	// Put large value to evict the rest in a loop
	epsiValue := sizeableValue{val: "Epsi", size: 15}
	cache.Put("E", epsiValue)
	assert.Nil(t, cache.Get("C"))
	assert.Equal(t, epsiValue, cache.Get("E"))
	assert.Nil(t, cache.Get("A"))
	assert.Equal(t, 1, cache.Size())

	// Put large value greater than maxSize but should not evict anything
	mepsiValue := sizeableValue{val: "Mepsi", size: 25}
	cache.Put("M", mepsiValue)
	assert.Nil(t, cache.Get("M"))
	assert.Equal(t, 1, cache.Size())
}

func TestLRU_SizeBased_CountExceeded(t *testing.T) {
	cache := New(&Options{
		MaxCount:    5,
		IsSizeBased: dynamicproperties.GetBoolPropertyFn(true),
		MaxSize:     dynamicproperties.GetIntPropertyFn(10000),
	})

	fooValue := sizeableValue{val: "Foo", size: 5}
	cache.Put("A", fooValue)
	assert.Equal(t, fooValue, cache.Get("A"))
	assert.Nil(t, cache.Get("B"))
	assert.Equal(t, 1, cache.Size())

	barValue := sizeableValue{val: "Bar", size: 5}
	cidValue := sizeableValue{val: "Cid", size: 5}
	deltValue := sizeableValue{val: "Delt", size: 5}

	cache.Put("B", barValue)
	cache.Put("C", cidValue)
	cache.Put("D", deltValue)
	assert.Equal(t, 4, cache.Size())

	assert.Equal(t, barValue, cache.Get("B"))
	assert.Equal(t, cidValue, cache.Get("C"))
	assert.Equal(t, deltValue, cache.Get("D"))

	foo2Value := sizeableValue{val: "Foo2", size: 5}
	cache.Put("A", foo2Value)
	assert.Equal(t, foo2Value, cache.Get("A"))
	assert.Equal(t, 4, cache.Size())

	epsiValue := sizeableValue{val: "Epsi", size: 5}
	cache.Put("E", epsiValue)
	assert.Equal(t, barValue, cache.Get("B"))
	assert.Equal(t, epsiValue, cache.Get("E"))
	assert.Equal(t, foo2Value, cache.Get("A"))
	assert.Equal(t, 5, cache.Size())
}

func TestLRU_EvictWhileSwitchToSizeBased(t *testing.T) {
	sizeBased := true

	// Create a function literal that can be implicitly coerced to BoolPropertyFn
	cache := New(&Options{
		MaxCount:    2,
		IsSizeBased: func(...dynamicproperties.FilterOption) bool { return sizeBased },
		MaxSize:     dynamicproperties.GetIntPropertyFn(10000),
	})

	fooValue := sizeableValue{val: "Foo", size: 5}
	barValue := sizeableValue{val: "Bar", size: 5}
	cidValue := sizeableValue{val: "Cid", size: 5}
	deltValue := sizeableValue{val: "Delt", size: 5}
	cache.Put("A", fooValue)
	cache.Put("B", barValue)
	cache.Put("C", cidValue)
	cache.Put("D", deltValue)
	assert.Equal(t, 4, cache.Size())
	assert.Equal(t, fooValue, cache.Get("A"))
	assert.Equal(t, barValue, cache.Get("B"))
	assert.Equal(t, cidValue, cache.Get("C"))
	assert.Equal(t, deltValue, cache.Get("D"))

	// Change the sizeBased flag to false
	sizeBased = false

	echoValue := sizeableValue{val: "Echo", size: 5}
	cache.Put("E", echoValue)
	assert.Equal(t, deltValue, cache.Get("D"))
	assert.Equal(t, echoValue, cache.Get("E"))
	assert.Equal(t, 2, cache.Size())
}

func TestPanicMaxCountAndSizeNotProvided(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The LRU was initialized without panic")
		}
	}()

	New(&Options{
		TTL: time.Millisecond * 100,
		GetCacheItemSizeFunc: func(interface{}) uint64 {
			return 5
		},
	})
}

func TestPanicMaxCountAndSizeFuncNotProvided(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The LRU was initialized without panic")
		}
	}()

	New(&Options{
		TTL:     time.Millisecond * 100,
		MaxSize: dynamicproperties.GetIntPropertyFn(0),
	})
}

func TestPanicOptionsIsNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The LRU was initialized without panic")
		}
	}()

	New(nil)
}

func TestEvictItemsPastTimeToLive_ActivelyEvict(t *testing.T) {
	// Create the cache with a TTL of 75s
	mockTimeSource := clock.NewMockedTimeSourceAt(time.UnixMilli(0))
	cache, ok := New(&Options{
		MaxCount:      5,
		TTL:           time.Second * 75,
		ActivelyEvict: true,
		TimeSource:    mockTimeSource,
	}).(*lru)
	require.True(t, ok)

	_, err := cache.PutIfNotExist("A", 1)
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("B", 2)
	require.NoError(t, err)

	// Nothing is expired after 50s
	mockTimeSource.Advance(time.Second * 50)
	assert.Equal(t, 2, cache.Size())

	_, err = cache.PutIfNotExist("C", 3)
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("D", 4)
	require.NoError(t, err)

	// No time has passed, so still nothing is expired
	assert.Equal(t, 4, cache.Size())

	// Advance time to 100s, so A and B should be expired
	mockTimeSource.Advance(time.Second * 50)
	assert.Equal(t, 2, cache.Size())

	// Advance time to 150s, so C and D should be expired as well
	mockTimeSource.Advance(time.Second * 50)
	assert.Equal(t, 0, cache.Size())
}

func TestLRU_PutInternal_EvictUnpinnedInMiddle(t *testing.T) {
	cache, ok := New(&Options{
		MaxCount: 5,
		Pin:      true,
	}).(*lru)
	require.True(t, ok)

	_, err := cache.PutIfNotExist("A", "Alpha")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("B", "Beta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("C", "Charlie")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("D", "Delta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("E", "Echo")
	require.NoError(t, err)

	// Verify all items are present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// release C and D twice
	cache.Release("C")
	cache.Release("C")
	cache.Release("D")
	cache.Release("D")

	// Try to add a new item - should evict unpinned items C and D
	_, err = cache.PutIfNotExist("F", "Foxtrot")
	require.NoError(t, err)

	// Verify pinned items are still present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Echo", cache.Get("E"))
	assert.Equal(t, "Foxtrot", cache.Get("F"))

	// Verify only C was evicted
	assert.Nil(t, cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))

}

func TestLRU_PutInternal_EvictUnpinnedFront(t *testing.T) {
	cache, ok := New(&Options{
		MaxCount: 5,
		Pin:      true,
	}).(*lru)
	require.True(t, ok)

	_, err := cache.PutIfNotExist("A", "Alpha")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("B", "Beta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("C", "Charlie")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("D", "Delta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("E", "Echo")
	require.NoError(t, err)

	// Verify all items are present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// move C to the front
	assert.Equal(t, "Charlie", cache.Get("C"))

	// release C twice
	cache.Release("C")
	cache.Release("C")

	// Try to add a new item - should evict unpinned item C
	_, err = cache.PutIfNotExist("F", "Foxtrot")
	require.NoError(t, err)

	// Verify pinned items are still present
	assert.Equal(t, "Foxtrot", cache.Get("F"))
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Verify only C was evicted
	assert.Nil(t, cache.Get("C"))
}

func TestLRU_PutInternal_EvictUnpinnedBack(t *testing.T) {
	cache, ok := New(&Options{
		MaxCount: 5,
		Pin:      true,
	}).(*lru)
	require.True(t, ok)

	_, err := cache.PutIfNotExist("A", "Alpha")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("B", "Beta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("C", "Charlie")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("D", "Delta")
	require.NoError(t, err)
	_, err = cache.PutIfNotExist("E", "Echo")
	require.NoError(t, err)

	// Verify all items are present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// release A twice
	cache.Release("A")
	cache.Release("A")

	// Try to add a new item - should evict unpinned item A
	_, err = cache.PutIfNotExist("F", "Foxtrot")
	require.NoError(t, err)

	// Verify pinned items are still present
	assert.Equal(t, "Foxtrot", cache.Get("F"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Verify A was evicted
	assert.Nil(t, cache.Get("A"))
}

func TestLRU_PutInternal_AllPinned(t *testing.T) {
	cache, ok := New(&Options{
		MaxCount: 5,
		Pin:      true,
	}).(*lru)
	assert.True(t, ok)

	_, err := cache.PutIfNotExist("A", "Alpha")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("B", "Beta")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("C", "Charlie")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("D", "Delta")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("E", "Echo")
	assert.NoError(t, err)

	// Verify all items are present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Try to add a new item - should not evict anything
	_, err = cache.PutIfNotExist("F", "Foxtrot")
	assert.Error(t, err)

	// Verify pinned items are still present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Verify F was never added
	assert.Nil(t, cache.Get("F"))
}

func TestLRU_PutInternal_Unpinned(t *testing.T) {
	cache, ok := New(&Options{
		MaxCount: 5,
		Pin:      false,
	}).(*lru)
	assert.True(t, ok)

	_, err := cache.PutIfNotExist("A", "Alpha")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("B", "Beta")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("C", "Charlie")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("D", "Delta")
	assert.NoError(t, err)
	_, err = cache.PutIfNotExist("E", "Echo")
	assert.NoError(t, err)

	// Verify all items are present
	assert.Equal(t, "Alpha", cache.Get("A"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Try to add a new item - should just evict A
	_, err = cache.PutIfNotExist("F", "Foxtrot")
	assert.NoError(t, err)

	assert.Equal(t, "Foxtrot", cache.Get("F"))
	assert.Equal(t, "Beta", cache.Get("B"))
	assert.Equal(t, "Charlie", cache.Get("C"))
	assert.Equal(t, "Delta", cache.Get("D"))
	assert.Equal(t, "Echo", cache.Get("E"))

	// Verify A was evicted
	assert.Nil(t, cache.Get("A"))
}
