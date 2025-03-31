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

package quotas

import (
	"sync"
)

// Collection stores a map of limiters by key
type Collection struct {
	mu       sync.RWMutex
	factory  LimiterFactory
	limiters map[string]Limiter
}

type ICollection interface {
	For(key string) Limiter
}

var _ ICollection = (*Collection)(nil)

// NewCollection create a new limiter collection.
// Given factory is called to create new individual limiter.
func NewCollection(factory LimiterFactory) *Collection {
	return &Collection{
		factory:  factory,
		limiters: make(map[string]Limiter),
	}
}

// For retrieves limiter by a given key.
// If limiter for such key does not exists, it creates new one with via factory.
func (c *Collection) For(key string) Limiter {
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
