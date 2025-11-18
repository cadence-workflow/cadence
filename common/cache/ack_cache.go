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
	"container/heap"
	"errors"
	"math"
	"sync"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

var (
	// ErrAckCacheFull is returned when the cache is at capacity and cannot accept new items
	ErrAckCacheFull = errors.New("ack cache is full")
	// ErrAlreadyAcked is returned when trying to add an item with sequence ID that was already acknowledged
	ErrAlreadyAcked = errors.New("sequence ID already acknowledged")
)

// AckCacheItem represents an item that can be stored in an AckCache.
// Items must have a sequence ID for ordering.
type AckCacheItem interface {
	// GetSequenceID returns the sequence identifier for this item.
	// Items are ordered by sequence ID and acknowledged up to a sequence level.
	GetSequenceID() int64
}

// AckCache is a bounded cache that stores items in sequence ID order.
// Items are removed via acknowledgment - calling Ack(level) removes all items
// with sequence ID <= level. When the cache reaches capacity, new items are
// rejected rather than evicting existing items.
//
// This cache is useful for scenarios like:
// - Buffering out-of-order messages that need acknowledgment
// - Caching items that must be processed in sequence
// - Storing items with backpressure (reject rather than evict)
//
// The cache is thread-safe for concurrent readers and writers.
type AckCache[T AckCacheItem] interface {
	// Put stores an item in the cache with the specified size. Returns ErrAckCacheFull if the cache
	// is at capacity, or ErrAlreadyAcked if the item's sequence ID has already
	// been acknowledged. Silently ignores duplicate sequence IDs.
	Put(item T, size uint64) error

	// Get retrieves an item by its sequence ID. Returns the zero value of T
	// if the item is not found.
	Get(sequenceID int64) T

	// Ack acknowledges all items with sequence ID <= level, removing them
	// from the cache. Returns the total size of items that were removed and the count of items removed.
	// This is the primary mechanism for freeing cache space.
	Ack(level int64) (uint64, int)

	// Size returns the current total byte size of all items in the cache.
	Size() uint64

	// Count returns the current number of items in the cache.
	Count() int
}

// BoundedAckCache is a heap-based implementation of AckCache that maintains
// items in sequence ID order and rejects new items when capacity limits are reached.
type BoundedAckCache[T AckCacheItem] struct {
	mu sync.Mutex

	maxCount dynamicproperties.IntPropertyFn
	maxSize  dynamicproperties.IntPropertyFn

	// Heap maintains items in sequence ID order for efficient acknowledgment
	order sequenceHeap[T]
	// Map provides O(1) lookup by sequence ID
	cache map[int64]T

	// Track acknowledgment level and current usage
	lastAck  int64
	currSize uint64

	logger        log.Logger
	budgetManager Manager
	cacheID       string
}

// NewBoundedAckCache creates a new bounded ack cache with the specified capacity limits.
//
// Parameters:
//   - maxCount: maximum number of items (dynamic property)
//   - maxSize: maximum total byte size (dynamic property)
//   - logger: optional logger for diagnostics (can be nil)
//   - budgetManager: optional budget manager for host-level capacity tracking (can be nil)
//   - cacheID: cache identifier for budget manager (required if budgetManager is provided)
//
// The cache will reject new items when either limit would be exceeded.
// If a budget manager is provided, Put and Ack operations will automatically
// reserve and release capacity through the budget manager.
func NewBoundedAckCache[T AckCacheItem](
	maxCount dynamicproperties.IntPropertyFn,
	maxSize dynamicproperties.IntPropertyFn,
	logger log.Logger,
	budgetManager Manager,
	cacheID string,
) AckCache[T] {
	initialCount := maxCount()
	return &BoundedAckCache[T]{
		maxCount:      maxCount,
		maxSize:       maxSize,
		order:         make(sequenceHeap[T], 0, initialCount),
		cache:         make(map[int64]T, initialCount),
		logger:        logger,
		budgetManager: budgetManager,
		cacheID:       cacheID,
	}
}

// Put stores an item in the cache with the specified size.
func (c *BoundedAckCache[T]) Put(item T, size uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sequenceID := item.GetSequenceID()

	// Reject items that have already been acknowledged
	if c.lastAck >= sequenceID {
		return ErrAlreadyAcked
	}

	// Silently ignore duplicate sequence IDs
	if _, exists := c.cache[sequenceID]; exists {
		return nil
	}

	// Check capacity limits
	if (len(c.order) >= c.maxCount()) ||
		(c.currSize+size >= uint64(c.maxSize())) ||
		(c.currSize > math.MaxUint64-size) {
		return ErrAckCacheFull
	}

	if c.budgetManager != nil {
		return c.budgetManager.ReserveWithCallback(c.cacheID, size, 1, func() error {
			return c.putInternal(item, sequenceID, size)
		})
	}

	return c.putInternal(item, sequenceID, size)
}

// Get retrieves an item by sequence ID.
func (c *BoundedAckCache[T]) Get(sequenceID int64) T {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache[sequenceID]
}

// Ack acknowledges all items with sequence ID <= level and returns total freed size and count of items removed.
func (c *BoundedAckCache[T]) Ack(level int64) (uint64, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.budgetManager != nil {
		var freedSize uint64
		var removedCount int64
		err := c.budgetManager.ReleaseWithCallback(c.cacheID, func() (uint64, int64, error) {
			freedSize, removedCount = c.ackInternal(level)
			return freedSize, removedCount, nil
		})
		if err != nil {
			return 0, 0
		}
		return freedSize, int(removedCount)
	}

	freedSize, removedCount := c.ackInternal(level)
	return freedSize, int(removedCount)
}

// Size returns current total byte size.
func (c *BoundedAckCache[T]) Size() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.currSize
}

// Count returns current number of items.
func (c *BoundedAckCache[T]) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.order)
}

// putInternal adds an item to the cache. Caller must hold the lock.
func (c *BoundedAckCache[T]) putInternal(item T, sequenceID int64, size uint64) error {
	c.cache[sequenceID] = item
	heap.Push(&c.order, heapItem[T]{sequenceID: sequenceID, size: size})
	c.currSize += size
	return nil
}

// ackInternal removes all items with sequence ID <= level. Caller must hold the lock.
func (c *BoundedAckCache[T]) ackInternal(level int64) (uint64, int64) {
	var freedSize uint64
	var removedCount int64

	for c.order.Len() > 0 && c.order.Peek().sequenceID <= level {
		item := heap.Pop(&c.order).(heapItem[T])
		delete(c.cache, item.sequenceID)
		c.currSize -= item.size
		freedSize += item.size
		removedCount++
	}

	c.lastAck = level
	return freedSize, removedCount
}

// heapItem represents an item in the sequence heap
type heapItem[T AckCacheItem] struct {
	sequenceID int64
	size       uint64
}

// sequenceHeap implements heap.Interface for maintaining items in sequence order
type sequenceHeap[T AckCacheItem] []heapItem[T]

func (h sequenceHeap[T]) Len() int           { return len(h) }
func (h sequenceHeap[T]) Less(i, j int) bool { return h[i].sequenceID < h[j].sequenceID }
func (h sequenceHeap[T]) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *sequenceHeap[T]) Push(x interface{}) {
	*h = append(*h, x.(heapItem[T]))
}

func (h *sequenceHeap[T]) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func (h *sequenceHeap[T]) Peek() heapItem[T] {
	return (*h)[0]
}
