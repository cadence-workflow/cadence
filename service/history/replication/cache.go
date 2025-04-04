// The MIT License (MIT)
//
// Copyright (c) 2022 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package replication

import (
	"container/heap"
	"errors"
	"sync"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

var (
	errCacheFull    = errors.New("cache is full")
	errAlreadyAcked = errors.New("already acked")
)

// Cache is an in-memory implementation of a cache for storing hydrated replication messages.
// Messages can come out of order as long as their task ID is higher than last acknowledged message.
// Out of order is expected as different source clusters will share hydrated replication messages.
//
// Cache utilizes heap to keep replication messages in order. This is needed for efficient acknowledgements in O(log N).
//
// Cache capacity can be increased dynamically. Decrease will require a restart, as new tasks will not be accepted, but memory will not be reclaimed either.
//
// Cache methods are thread safe. It is expected to have writers and readers from different go routines.
type Cache struct {
	mu sync.RWMutex

	capacity dynamicproperties.IntPropertyFn

	order int64Heap
	cache map[int64]*types.ReplicationTask

	lastAck int64
}

// NewCache create a new instance of replication cache
func NewCache(capacity dynamicproperties.IntPropertyFn) *Cache {
	initialCapacity := capacity()
	return &Cache{
		capacity: capacity,
		order:    make(int64Heap, 0, initialCapacity),
		cache:    make(map[int64]*types.ReplicationTask, initialCapacity),
	}
}

// Size returns current size of the cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.order)
}

// Put stores replication task in the cache.
// - If cache is full, it will return errCacheFull
// - If given task has ID lower than previously acknowledged task, it will errOutOfOrder
func (c *Cache) Put(task *types.ReplicationTask) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for full cache
	if len(c.order) >= c.capacity() {
		return errCacheFull
	}

	taskID := task.SourceTaskID

	// Reject task as it was already acknowledged
	if c.lastAck >= taskID {
		return errAlreadyAcked
	}

	// Do not add duplicate tasks
	if _, exists := c.cache[taskID]; exists {
		return nil
	}

	c.cache[taskID] = task
	heap.Push(&c.order, taskID)

	return nil
}

// Get will return a stored task having a given taskID.
// If task is not cache, nil is returned.
func (c *Cache) Get(taskID int64) *types.ReplicationTask {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.cache[taskID]
}

// Ack is used to acknowledge replication messages.
// Meaning they will be removed from the cache.
func (c *Cache) Ack(level int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for c.order.Len() > 0 && c.order.Peek() <= level {
		taskID := heap.Pop(&c.order).(int64)
		delete(c.cache, taskID)
	}

	c.lastAck = level
}

type int64Heap []int64

func (h int64Heap) Len() int           { return len(h) }
func (h int64Heap) Less(i, j int) bool { return h[i] < h[j] }
func (h int64Heap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *int64Heap) Push(x interface{}) {
	*h = append(*h, x.(int64))
}

func (h *int64Heap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *int64Heap) Peek() int64 {
	return (*h)[0]
}
