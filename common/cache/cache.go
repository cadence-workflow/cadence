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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interface_mock.go -self_package github.com/uber/cadence/common/cache

package cache

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
)

// A Cache is a generalized interface to a cache.  See cache.LRU for a specific
// implementation (bounded cache with LRU eviction)
type Cache interface {
	// Get retrieves an element based on a key, returning nil if the element
	// does not exist
	Get(key interface{}) interface{}

	// Put adds an element to the cache, returning the previous element
	Put(key interface{}, value interface{}) interface{}

	// PutIfNotExist puts a value associated with a given key if it does not exist
	PutIfNotExist(key interface{}, value interface{}) (interface{}, error)

	// Delete deletes an element in the cache
	Delete(key interface{})

	// Release decrements the ref count of a pinned element. If the ref count
	// drops to 0, the element can be evicted from the cache.
	Release(key interface{})

	// Iterator returns the iterator of the cache
	Iterator() Iterator

	// Size returns the number of entries currently stored in the Cache
	Size() int
}

// Options control the behavior of the cache
type Options struct {
	// TTL controls the time-to-live for a given cache entry.  Cache entries that
	// are older than the TTL will not be returned.
	TTL time.Duration

	// InitialCapacity controls the initial capacity of the cache
	InitialCapacity int

	// Pin prevents in-use objects from getting evicted.
	Pin bool

	// RemovedFunc is an optional function called when an element
	// is scheduled for deletion
	RemovedFunc RemovedFunc

	// MaxCount controls the max capacity of the cache
	// It is required option if MaxSize is not provided
	MaxCount int

	// MaxSize is an optional flag, but it has to be used along with a value that implements Sizeable() interface
	// to control the max size in bytes of the cache
	// It is required option if MaxCount is not provided
	MaxSize dynamicproperties.IntPropertyFn

	// ActivelyEvict will evict items that has expired TTL at every operation in the cache
	// This can be expensive if a lot of items expire at the same time
	// Should be used when it's important for memory that the expired items are evicted as soon as possible
	// If not set expired items will be evicted when one of these happens
	// - when the cache is full
	// - when the item is accessed
	ActivelyEvict bool

	// TimeSource is used to get the current time
	// It is optional and defaults to clock.NewRealTimeSource()
	TimeSource clock.TimeSource

	// IsSizeBased is an optional flag to indicate if the cache is size based
	// It's default is false, but if set to true, the cache will evict items based on item size instead of count
	// But the item HAS to be able to cast as a Sizeable interface otherwise the cache will fail
	IsSizeBased dynamicproperties.BoolPropertyFn

	// MetricsScope is used to emit metrics for internals of the cache
	MetricsScope metrics.Scope

	// Logger is used to emit logs for internals of the cache
	Logger log.Logger

	// Deprecated: GetCacheItemSizeFunc is a function called upon adding the item to update the cache size.
	// It returns 0 by default, assuming the cache is just count based
	// It is required option if MaxCount is not provided
	GetCacheItemSizeFunc GetCacheItemSizeFunc
}

// SimpleOptions provides options that can be used to configure SimpleCache
type SimpleOptions struct {
	// InitialCapacity controls the initial capacity of the cache
	InitialCapacity int

	// RemovedFunc is an optional function called when an element
	// is scheduled for deletion
	RemovedFunc RemovedFunc
}

// RemovedFunc is a type for notifying applications when an item is
// scheduled for removal from the Cache. If f is a function with the
// appropriate signature and i is the interface{} scheduled for
// deletion, Cache calls go f(i)
type RemovedFunc func(interface{})

// Iterator represents the interface for cache iterators
type Iterator interface {
	// Close closes the iterator
	// and releases any allocated resources
	Close()
	// HasNext return true if there is more items to be returned
	HasNext() bool
	// Next return the next item
	Next() Entry
}

// Entry represents a key-value entry within the map
type Entry interface {
	// Key represents the key
	Key() interface{}
	// Value represents the value
	Value() interface{}
	// CreateTime represents the time when the entry is created
	CreateTime() time.Time
}

// GetCacheItemSizeFunc returns the cache item size in bytes
type GetCacheItemSizeFunc func(interface{}) uint64

// DomainMetricsScopeCache represents a interface for mapping domainID and scopeIdx to metricsScope
type DomainMetricsScopeCache interface {
	// Get retrieves metrics scope for a domainID and scopeIdx
	Get(domainID string, scopeIdx metrics.ScopeIdx) (metrics.Scope, bool)
	// Put adds metrics scope for a domainID and scopeIdx
	Put(domainID string, scopeIdx metrics.ScopeIdx, metricsScope metrics.Scope)

	common.Daemon
}

// Sizeable is a interface that implements ByteSize() function
type Sizeable interface {
	// ByteSize returns an approximate size of the object in bytes
	ByteSize() uint64
}
