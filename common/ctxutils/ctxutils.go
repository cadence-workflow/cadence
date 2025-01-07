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

package ctxutils

import (
	"context"
	"sync"
)

// WithPropagatedContextCancel returns a copy of parent which is cancelled whenever
// it is cancelled itself (with the returned cancel func) or cancelCtx is cancelled
// you need to call cancel func to avoid potential goroutine leak
func WithPropagatedContextCancel(parent context.Context, cancelCtx context.Context) (context.Context, context.CancelFunc) {
	done := cancelCtx.Done()
	if done == nil {
		return parent, func() {}
	}

	childWithCancel, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		select {
		case <-done:
			cancel()
		case <-childWithCancel.Done():
		}
	}()

	return childWithCancel, func() {
		cancel()
		wg.Wait()
	}
}
