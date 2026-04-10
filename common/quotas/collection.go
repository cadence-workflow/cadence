// Copyright (c) 2022 Uber Technologies, Inc.
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

//go:generate mockgen -package=$GOPACKAGE -destination=collection_mock.go github.com/uber/cadence/common/quotas ICollection
//go:generate mockgen -package=$GOPACKAGE -destination=limiterfactory_mock.go github.com/uber/cadence/common/quotas LimiterFactory

package quotas

import (
	"sync"
)

// LimiterFactory is used to create a Limiter for a given domain
type LimiterFactory[K comparable] interface {
	// GetLimiter returns a new Limiter for the given domain
	GetLimiter(key K) Limiter
}

// Collection stores a map of limiters by key
type Collection[K comparable] struct {
	mu       sync.RWMutex
	factory  LimiterFactory[K]
	limiters map[K]Limiter
}

type ICollection[K comparable] interface {
	For(key K) Limiter
}

var _ ICollection[string] = (*Collection[string])(nil)

// NewCollection create a new limiter collection.
// Given factory is called to create new individual limiter.
func NewCollection[K comparable](factory LimiterFactory[K]) *Collection[K] {
	return &Collection[K]{
		factory:  factory,
		limiters: make(map[K]Limiter),
	}
}

// For retrieves limiter by a given key.
// If limiter for such key does not exists, it creates new one with via factory.
func (c *Collection[K]) For(key K) Limiter {
	c.mu.RLock()
	limiter, ok := c.limiters[key]
	c.mu.RUnlock()

	if !ok {
		// create a new limiter
		newLimiter := c.factory.GetLimiter(key)

		// verify that it is needed and add to map
		c.mu.Lock()
		limiter, ok = c.limiters[key]
		if !ok {
			c.limiters[key] = newLimiter
			limiter = newLimiter
		}
		c.mu.Unlock()
	}

	return limiter
}
