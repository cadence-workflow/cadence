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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHostLimiter_NilReceiver(t *testing.T) {
	var limiter *HostLimiter
	require.NoError(t, limiter.Acquire(context.Background()))
	limiter.Release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, limiter.Acquire(ctx), context.Canceled)
}

func TestHostLimiter_NonPositiveLimitIsUnbounded(t *testing.T) {
	limiter := NewHostLimiter(0)
	require.NoError(t, limiter.Acquire(context.Background()))
	limiter.Release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, limiter.Acquire(ctx), context.Canceled)
}

func TestHostLimiter_BlocksUntilRelease(t *testing.T) {
	limiter := NewHostLimiter(1)
	require.NoError(t, limiter.Acquire(context.Background()))

	acquired := make(chan error, 1)
	go func() {
		acquired <- limiter.Acquire(context.Background())
	}()

	select {
	case err := <-acquired:
		require.NoError(t, err)
		t.Fatal("second acquire succeeded before release")
	case <-time.After(100 * time.Millisecond):
	}

	limiter.Release()
	select {
	case err := <-acquired:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("second acquire did not succeed after release")
	}
	limiter.Release()
}

func TestHostLimiter_AcquireReturnsContextErrorWhileWaiting(t *testing.T) {
	limiter := NewHostLimiter(1)
	require.NoError(t, limiter.Acquire(context.Background()))
	defer limiter.Release()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- limiter.Acquire(ctx)
	}()

	select {
	case err := <-result:
		t.Fatalf("acquire returned before cancellation: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("acquire did not return after cancellation")
	}
}
