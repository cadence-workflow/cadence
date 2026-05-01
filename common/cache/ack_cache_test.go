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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

// testItem implements AckCacheItem for testing
type testItem struct {
	sequenceID int64
	data       string
	size       uint64
}

func (t *testItem) GetSequenceID() int64 {
	return t.sequenceID
}

func (t *testItem) ByteSize() uint64 {
	if t.size > 0 {
		return t.size
	}
	// Default size based on data length
	return uint64(len(t.data)) + 16 // 16 bytes overhead
}

func newTestItem(sequenceID int64, data string) *testItem {
	return &testItem{
		sequenceID: sequenceID,
		data:       data,
	}
}

func newTestItemWithSize(sequenceID int64, data string, size uint64) *testItem {
	return &testItem{
		sequenceID: sequenceID,
		data:       data,
		size:       size,
	}
}

func TestBoundedAckCache_BasicOperations(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),   // max count
		dynamicproperties.GetIntPropertyFn(1000), // max size
		log.NewNoop(),
		nil,
		"",
	)

	// Test empty cache
	assert.Equal(t, uint64(0), cache.Size())
	assert.Equal(t, 0, cache.Count())
	assert.Nil(t, cache.Get(100))

	// Test adding items
	item1 := newTestItem(100, "item1")
	item2 := newTestItem(200, "item2")
	item3 := newTestItem(150, "item3") // out of order

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	assert.Equal(t, item1.ByteSize(), cache.Size())
	assert.Equal(t, 1, cache.Count())
	assert.Equal(t, item1, cache.Get(100))

	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	assert.Equal(t, item1.ByteSize()+item2.ByteSize(), cache.Size())
	assert.Equal(t, 2, cache.Count())
	assert.Equal(t, item2, cache.Get(200))

	// Test out-of-order insertion
	require.NoError(t, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, item1.ByteSize()+item2.ByteSize()+item3.ByteSize(), cache.Size())
	assert.Equal(t, 3, cache.Count())
	assert.Equal(t, item3, cache.Get(150))

	// Test duplicate insertion (should be silently ignored)
	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	assert.Equal(t, 3, cache.Count()) // No change
}

func TestBoundedAckCache_Acknowledgment(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		nil,
		"",
	)

	item1 := newTestItem(100, "item1")
	item2 := newTestItem(200, "item2")
	item3 := newTestItem(300, "item3")
	item4 := newTestItem(150, "item4")

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	require.NoError(t, cache.Put(item3, item3.ByteSize()))
	require.NoError(t, cache.Put(item4, item4.ByteSize()))

	totalSize := item1.ByteSize() + item2.ByteSize() + item3.ByteSize() + item4.ByteSize()
	assert.Equal(t, totalSize, cache.Size())
	assert.Equal(t, 4, cache.Count())

	// Ack at level 0 - should not remove anything
	freedSize, removedCount := cache.Ack(0)
	assert.Equal(t, uint64(0), freedSize)
	assert.Equal(t, 0, removedCount)
	assert.Equal(t, 4, cache.Count())
	assert.Equal(t, totalSize, cache.Size())

	// Ack at level 100 - should remove item1
	freedSize, removedCount = cache.Ack(100)
	assert.Equal(t, item1.ByteSize(), freedSize)
	assert.Equal(t, 1, removedCount)
	assert.Equal(t, 3, cache.Count())
	assert.Equal(t, totalSize-item1.ByteSize(), cache.Size())
	assert.Nil(t, cache.Get(100))
	assert.Equal(t, item4, cache.Get(150))
	assert.Equal(t, item2, cache.Get(200))
	assert.Equal(t, item3, cache.Get(300))

	// Ack at level 200 - should remove item4 and item2
	freedSize, removedCount = cache.Ack(200)
	assert.Equal(t, item4.ByteSize()+item2.ByteSize(), freedSize)
	assert.Equal(t, 2, removedCount)
	assert.Equal(t, 1, cache.Count())
	assert.Equal(t, item3.ByteSize(), cache.Size())
	assert.Nil(t, cache.Get(100))
	assert.Nil(t, cache.Get(150))
	assert.Nil(t, cache.Get(200))
	assert.Equal(t, item3, cache.Get(300))

	// Ack at level 500 - should remove everything
	freedSize, removedCount = cache.Ack(500)
	assert.Equal(t, item3.ByteSize(), freedSize)
	assert.Equal(t, 1, removedCount)
	assert.Equal(t, 0, cache.Count())
	assert.Equal(t, uint64(0), cache.Size())
	assert.Nil(t, cache.Get(300))
}

func TestBoundedAckCache_AlreadyAcked(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		nil,
		"",
	)

	item1 := newTestItem(100, "item1")
	item2 := newTestItem(200, "item2")
	item3 := newTestItem(50, "item3") // lower than ack level

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	require.NoError(t, cache.Put(item2, item2.ByteSize()))

	// Ack at level 150
	_, _ = cache.Ack(150)
	assert.Equal(t, 1, cache.Count())
	assert.Nil(t, cache.Get(100))
	assert.Equal(t, item2, cache.Get(200))

	// Try to add item with sequence ID lower than ack level
	assert.Equal(t, ErrAlreadyAcked, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, 1, cache.Count()) // No change

	// Try to add item1 again (already acked)
	assert.Equal(t, ErrAlreadyAcked, cache.Put(item1, item1.ByteSize()))
	assert.Equal(t, 1, cache.Count()) // No change
}

func TestBoundedAckCache_CountLimit(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(3),    // max count = 3
		dynamicproperties.GetIntPropertyFn(1000), // large size limit
		log.NewNoop(),
		nil,
		"",
	)

	item1 := newTestItem(100, "item1")
	item2 := newTestItem(200, "item2")
	item3 := newTestItem(300, "item3")
	item4 := newTestItem(400, "item4")

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	require.NoError(t, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, 3, cache.Count())

	// Fourth item should be rejected
	assert.Equal(t, ErrAckCacheFull, cache.Put(item4, item4.ByteSize()))
	assert.Equal(t, 3, cache.Count())
	assert.Nil(t, cache.Get(400))

	// After acking, should be able to add more
	_, _ = cache.Ack(200)
	assert.Equal(t, 1, cache.Count())
	require.NoError(t, cache.Put(item4, item4.ByteSize()))
	assert.Equal(t, 2, cache.Count())
	assert.Equal(t, item4, cache.Get(400))
}

func TestBoundedAckCache_SizeLimit(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),  // large count limit
		dynamicproperties.GetIntPropertyFn(100), // max size = 100
		log.NewNoop(),
		nil,
		"",
	)

	item1 := newTestItemWithSize(100, "item1", 40)
	item2 := newTestItemWithSize(200, "item2", 40)
	item3 := newTestItemWithSize(300, "item3", 30) // would exceed size limit

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	assert.Equal(t, uint64(80), cache.Size())
	assert.Equal(t, 2, cache.Count())

	// Third item should be rejected due to size
	assert.Equal(t, ErrAckCacheFull, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, uint64(80), cache.Size())
	assert.Equal(t, 2, cache.Count())
	assert.Nil(t, cache.Get(300))

	// After acking, should be able to add more
	_, _ = cache.Ack(150)
	assert.Equal(t, uint64(40), cache.Size())
	assert.Equal(t, 1, cache.Count())
	require.NoError(t, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, uint64(70), cache.Size())
	assert.Equal(t, 2, cache.Count())
}

func TestBoundedAckCache_DynamicLimits(t *testing.T) {
	maxCount := dynamicproperties.GetIntPropertyFn(2)
	maxSize := dynamicproperties.GetIntPropertyFn(100)

	cache := NewBoundedAckCache[*testItem](maxCount, maxSize, log.NewNoop(), nil, "")

	item1 := newTestItem(100, "item1")
	item2 := newTestItem(200, "item2")
	item3 := newTestItem(300, "item3")

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	assert.Equal(t, ErrAckCacheFull, cache.Put(item3, item3.ByteSize()))

	// Increase count limit dynamically
	maxCount = dynamicproperties.GetIntPropertyFn(5)
	cache.(*BoundedAckCache[*testItem]).maxCount = maxCount

	// Now the third item should be accepted
	require.NoError(t, cache.Put(item3, item3.ByteSize()))
	assert.Equal(t, 3, cache.Count())
}

func TestBoundedAckCache_ConcurrentOperations(t *testing.T) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(1000),
		dynamicproperties.GetIntPropertyFn(100000),
		log.NewNoop(),
		nil,
		"",
	)

	// This test ensures no race conditions with concurrent access
	done := make(chan bool, 3)

	// Writer goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			item := newTestItem(int64(i*10), "item")
			cache.Put(item, item.ByteSize())
		}
	}()

	// Reader goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			cache.Get(int64(i * 10))
			cache.Size()
			cache.Count()
		}
	}()

	// Acker goroutine
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 10; i++ {
			_, _ = cache.Ack(int64(i * 100))
		}
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Final ack to clean up
	_, _ = cache.Ack(1000)
	assert.Equal(t, 0, cache.Count())
	assert.Equal(t, uint64(0), cache.Size())
}

func TestBoundedAckCache_WithBudgetManager(t *testing.T) {
	budgetMgr := NewBudgetManager(
		"test-budget",
		dynamicproperties.GetIntPropertyFn(1000),
		dynamicproperties.GetIntPropertyFn(100),
		AdmissionOptimistic,
		0,
		nil,
		log.NewNoop(),
		nil,
	)

	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		budgetMgr,
		"cache1",
	)

	item1 := newTestItemWithSize(100, "item1", 100)
	item2 := newTestItemWithSize(200, "item2", 150)

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	assert.Equal(t, uint64(100), budgetMgr.UsedBytes())
	assert.Equal(t, int64(1), budgetMgr.UsedCount())

	require.NoError(t, cache.Put(item2, item2.ByteSize()))
	assert.Equal(t, uint64(250), budgetMgr.UsedBytes())
	assert.Equal(t, int64(2), budgetMgr.UsedCount())

	freedSize, freedCount := cache.Ack(100)
	assert.Equal(t, uint64(100), freedSize)
	assert.Equal(t, 1, freedCount)
	assert.Equal(t, uint64(150), budgetMgr.UsedBytes())
	assert.Equal(t, int64(1), budgetMgr.UsedCount())

	freedSize, freedCount = cache.Ack(200)
	assert.Equal(t, uint64(150), freedSize)
	assert.Equal(t, 1, freedCount)
	assert.Equal(t, uint64(0), budgetMgr.UsedBytes())
	assert.Equal(t, int64(0), budgetMgr.UsedCount())
}

func TestBoundedAckCache_WithBudgetManager_BudgetExceeded(t *testing.T) {
	budgetMgr := NewBudgetManager(
		"test-budget",
		dynamicproperties.GetIntPropertyFn(200),
		dynamicproperties.GetIntPropertyFn(10),
		AdmissionOptimistic,
		0,
		nil,
		log.NewNoop(),
		nil,
	)

	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		budgetMgr,
		"cache1",
	)

	item1 := newTestItemWithSize(100, "item1", 150)
	item2 := newTestItemWithSize(200, "item2", 100)

	require.NoError(t, cache.Put(item1, item1.ByteSize()))
	assert.Equal(t, uint64(150), budgetMgr.UsedBytes())

	err := cache.Put(item2, item2.ByteSize())
	assert.Equal(t, ErrBytesBudgetExceeded, err)
	assert.Equal(t, uint64(150), budgetMgr.UsedBytes())
	assert.Equal(t, int64(1), budgetMgr.UsedCount())
	assert.Equal(t, 1, cache.Count())
	assert.Nil(t, cache.Get(200))
}

func TestBoundedAckCache_WithBudgetManager_MultipleCaches(t *testing.T) {
	budgetMgr := NewBudgetManager(
		"test-budget",
		dynamicproperties.GetIntPropertyFn(500),
		dynamicproperties.GetIntPropertyFn(100),
		AdmissionOptimistic,
		0,
		nil,
		log.NewNoop(),
		nil,
	)

	cache1 := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		budgetMgr,
		"cache1",
	)

	cache2 := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10),
		dynamicproperties.GetIntPropertyFn(1000),
		log.NewNoop(),
		budgetMgr,
		"cache2",
	)

	item1 := newTestItemWithSize(100, "item1", 200)
	item2 := newTestItemWithSize(200, "item2", 200)

	require.NoError(t, cache1.Put(item1, item1.ByteSize()))
	assert.Equal(t, uint64(200), budgetMgr.UsedBytes())

	require.NoError(t, cache2.Put(item2, item2.ByteSize()))
	assert.Equal(t, uint64(400), budgetMgr.UsedBytes())

	freedSize, freedCount := cache1.Ack(100)
	assert.Equal(t, uint64(200), freedSize)
	assert.Equal(t, 1, freedCount)
	assert.Equal(t, uint64(200), budgetMgr.UsedBytes())

	freedSize, freedCount = cache2.Ack(200)
	assert.Equal(t, uint64(200), freedSize)
	assert.Equal(t, 1, freedCount)
	assert.Equal(t, uint64(0), budgetMgr.UsedBytes())
}

func BenchmarkBoundedAckCache(b *testing.B) {
	cache := NewBoundedAckCache[*testItem](
		dynamicproperties.GetIntPropertyFn(10000),
		dynamicproperties.GetIntPropertyFn(1000000),
		log.NewNoop(),
		nil,
		"",
	)

	// Pre-populate cache
	for i := 0; i < 5000; i++ {
		item := newTestItem(int64(i*100), "benchmark_item")
		cache.Put(item, item.ByteSize())
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sequenceID := int64(n * 100)
		cache.Get(sequenceID)
		item := newTestItem(int64(n*100+500000), "new_item")
		cache.Put(item, item.ByteSize())
		if n%100 == 0 {
			_, _ = cache.Ack(sequenceID)
		}
	}
}
