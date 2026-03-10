// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package task

import (
	"sync/atomic"
	"time"
)

// TTLChannel is a channel that can expire if it is not written to for a given amount of time.
type TTLChannel[V any] struct {
	c             chan V
	lastWriteTime atomic.Int64
	refCount      atomic.Int32
}

func NewTTLChannel[V any](bufferSize int) *TTLChannel[V] {
	return &TTLChannel[V]{
		c: make(chan V, bufferSize),
	}
}

func (c *TTLChannel[V]) IncRef() {
	c.refCount.Add(1)
}

func (c *TTLChannel[V]) DecRef() {
	c.refCount.Add(-1)
}

func (c *TTLChannel[V]) RefCount() int32 {
	return c.refCount.Load()
}

func (c *TTLChannel[V]) LastWriteTime() time.Time {
	return time.Unix(c.lastWriteTime.Load(), 0)
}

func (c *TTLChannel[V]) UpdateLastWriteTime(now time.Time) {
	c.lastWriteTime.Store(now.Unix())
}

func (c *TTLChannel[V]) Chan() chan V {
	return c.c
}

func (c *TTLChannel[V]) Len() int {
	return len(c.c)
}

func (c *TTLChannel[V]) Cap() int {
	return cap(c.c)
}

func (c *TTLChannel[V]) ShouldCleanup(now time.Time, ttl time.Duration) bool {
	return now.Sub(c.LastWriteTime()) > ttl && c.Len() == 0 && c.RefCount() == 0
}
