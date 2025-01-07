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

package locks

import (
	"context"
	"sync"
)

type (
	// Mutex accepts a context in its Lock method.
	// It blocks the goroutine until either the lock is acquired or the context
	// is closed.
	Mutex interface {
		Lock(context.Context) error
		Unlock()
	}
)

type impl struct {
	ch chan struct{}
}

func NewMutex() Mutex {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return &impl{
		ch: ch,
	}
}

func (m *impl) Lock(ctx context.Context) error {
	select {
	case <-m.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *impl) Unlock() {
	select {
	case m.ch <- struct{}{}:
	default:
		// reaching this branch means the mutex was not locked.
		// this will intentionally crash the process, and print stack traces.
		//
		// it's not really possible to do this by hand (`fatal` is private),
		// and other common options like `log.Fatal` don't print stacks / don't
		// print all stacks / have loads of minor inconsistencies.
		//
		// regardless, what we want is to mimic mutexes when wrongly unlocked,
		// so just use the mutex implementation to guarantee it's the same.
		var m sync.Mutex
		m.Unlock()
	}
}
