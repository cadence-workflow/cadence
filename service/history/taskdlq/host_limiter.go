// The MIT License (MIT)

// Copyright (c) 2026 Uber Technologies Inc.

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

package taskdlq

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// HostLimiter bounds how many shards' DLQ processing may run concurrently on a
// single history host. A nil *HostLimiter, or one built with a non-positive
// limit, imposes no bound.
type HostLimiter struct {
	sem *semaphore.Weighted
}

// NewHostLimiter returns a limiter allowing up to limit concurrent holders.
// A non-positive limit returns an unbounded limiter.
func NewHostLimiter(limit int) *HostLimiter {
	limiter := &HostLimiter{}
	if limit > 0 {
		limiter.sem = semaphore.NewWeighted(int64(limit))
	}
	return limiter
}

// Acquire blocks until a slot is free or ctx is done.
func (l *HostLimiter) Acquire(ctx context.Context) error {
	if l == nil || l.sem == nil {
		return ctx.Err()
	}
	return l.sem.Acquire(ctx, 1)
}

// Release frees a slot previously acquired with Acquire.
func (l *HostLimiter) Release() {
	if l == nil || l.sem == nil {
		return
	}
	l.sem.Release(1)
}
