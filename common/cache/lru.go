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
	"container/list"
	"errors"
	"sync"
	"time"
)

var (
	// ErrCacheFull is returned if Put fails due to cache being filled with pinned elements
	ErrCacheFull = errors.New("Cache capacity is fully occupied with pinned elements")
)

// lru is a concurrent fixed size cache that evicts elements in lru order
type (
	lru struct {
		mut      sync.Mutex
		byAccess *list.List
		byKey    map[interface{}]*list.Element
		maxSize  int
		ttl      time.Duration
		pin      bool
		rmFunc   RemovedFunc
	}

	iteratorImpl struct {
		stopCh chan struct{}
		dataCh chan Entry
	}

	entryImpl struct {
		key       interface{}
		timestamp time.Time
		value     interface{}
		refCount  int
	}
)

// Close closes the iterator
func (it *iteratorImpl) Close() {
	close(it.stopCh)
}

// Entries returns a channel of map entries
func (it *iteratorImpl) Entries() <-chan Entry {
	return it.dataCh
}

// Iterator returns an iterator to the map. This map
// does not use re-entrant locks, so access or modification
// to the map during iteration can cause a dead lock.
func (c *lru) Iterator() Iterator {

	iterator := &iteratorImpl{
		dataCh: make(chan Entry, 8),
		stopCh: make(chan struct{}),
	}

	go func(iterator *iteratorImpl) {
		c.mut.Lock()
		for _, element := range c.byKey {
			entry := element.Value.(*entryImpl)
			if c.isEntryExpired(entry) {
				// Entry has expired
				c.deleteInternal(element)
				continue
			}
			// make a copy of the entry so there will be no concurrent access to this entry
			entry = &entryImpl{
				key:       entry.key,
				value:     entry.value,
				timestamp: entry.timestamp,
			}
			select {
			case iterator.dataCh <- entry:
			case <-iterator.stopCh:
				c.mut.Unlock()
				close(iterator.dataCh)
				return
			}
		}
		c.mut.Unlock()
		close(iterator.dataCh)
	}(iterator)

	return iterator
}

func (entry *entryImpl) Key() interface{} {
	return entry.key
}

func (entry *entryImpl) Value() interface{} {
	return entry.value
}

func (entry *entryImpl) Timestamp() time.Time {
	return entry.timestamp
}

// New creates a new cache with the given options
func New(maxSize int, opts *Options) Cache {
	if opts == nil {
		opts = &Options{}
	}

	return &lru{
		byAccess: list.New(),
		byKey:    make(map[interface{}]*list.Element, opts.InitialCapacity),
		ttl:      opts.TTL,
		maxSize:  maxSize,
		pin:      opts.Pin,
		rmFunc:   opts.RemovedFunc,
	}
}

// NewLRU creates a new LRU cache of the given size, setting initial capacity
// to the max size
func NewLRU(maxSize int) Cache {
	return New(maxSize, nil)
}

// NewLRUWithInitialCapacity creates a new LRU cache with an initial capacity
// and a max size
func NewLRUWithInitialCapacity(initialCapacity, maxSize int) Cache {
	return New(maxSize, &Options{
		InitialCapacity: initialCapacity,
	})
}

// Get retrieves the value stored under the given key
func (c *lru) Get(key interface{}) interface{} {
	c.mut.Lock()
	defer c.mut.Unlock()

	element := c.byKey[key]
	if element == nil {
		return nil
	}

	entry := element.Value.(*entryImpl)

	if c.pin {
		entry.refCount++
	}

	if c.isEntryExpired(entry) {
		// Entry has expired
		c.deleteInternal(element)
		return nil
	}

	c.byAccess.MoveToFront(element)
	return entry.value
}

// Put puts a new value associated with a given key, returning the existing value (if present)
func (c *lru) Put(key interface{}, value interface{}) interface{} {
	if c.pin {
		panic("Cannot use Put API in Pin mode. Use Delete and PutIfNotExist if necessary")
	}
	val, _ := c.putInternal(key, value, true)
	return val
}

// PutIfNotExist puts a value associated with a given key if it does not exist
func (c *lru) PutIfNotExist(key interface{}, value interface{}) (interface{}, error) {
	existing, err := c.putInternal(key, value, false)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		// This is a new value
		return value, err
	}

	return existing, err
}

// Delete deletes a key, value pair associated with a key
func (c *lru) Delete(key interface{}) {
	c.mut.Lock()
	defer c.mut.Unlock()

	element := c.byKey[key]
	if element != nil {
		c.deleteInternal(element)
	}
}

// Release decrements the ref count of a pinned element.
func (c *lru) Release(key interface{}) {
	c.mut.Lock()
	defer c.mut.Unlock()

	elt := c.byKey[key]
	entry := elt.Value.(*entryImpl)
	entry.refCount--
}

// Size returns the number of entries currently in the lru, useful if cache is not full
func (c *lru) Size() int {
	c.mut.Lock()
	defer c.mut.Unlock()

	return len(c.byKey)
}

// Put puts a new value associated with a given key, returning the existing value (if present)
// allowUpdate flag is used to control overwrite behavior if the value exists
func (c *lru) putInternal(key interface{}, value interface{}, allowUpdate bool) (interface{}, error) {
	c.mut.Lock()
	defer c.mut.Unlock()

	elt := c.byKey[key]
	if elt != nil {
		entry := elt.Value.(*entryImpl)
		existing := entry.value
		if allowUpdate {
			entry.value = value
		}
		if c.ttl != 0 {
			entry.timestamp = time.Now()
		}
		c.byAccess.MoveToFront(elt)
		if c.pin {
			entry.refCount++
		}
		return existing, nil
	}

	entry := &entryImpl{
		key:   key,
		value: value,
	}

	if c.pin {
		entry.refCount++
	}

	if c.ttl != 0 {
		entry.timestamp = time.Now()
	}

	c.byKey[key] = c.byAccess.PushFront(entry)
	if len(c.byKey) == c.maxSize {
		oldest := c.byAccess.Back().Value.(*entryImpl)

		if oldest.refCount > 0 {
			// Cache is full with pinned elements
			// revert the insert and return
			c.deleteInternal(c.byAccess.Front())
			return nil, ErrCacheFull
		}

		c.deleteInternal(c.byAccess.Back())
	}

	return nil, nil
}

func (c *lru) deleteInternal(element *list.Element) {
	entry := c.byAccess.Remove(element).(*entryImpl)
	if c.rmFunc != nil {
		go c.rmFunc(entry.value)
	}
	delete(c.byKey, entry.key)
}

func (c *lru) isEntryExpired(entry *entryImpl) bool {
	return entry.refCount == 0 && !entry.timestamp.IsZero() && time.Now().After(entry.timestamp.Add(c.ttl))
}
