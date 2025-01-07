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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestWithPropagatedContextCancel(t *testing.T) {
	defer goleak.VerifyNone(t)
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	ctx, stopper := WithPropagatedContextCancel(context.Background(), cancelCtx)
	defer stopper()

	cancelFn() // cancel should be propagated to "chained" ctx

	testTimeout := time.NewTimer(10 * time.Second)
	cancelledOriginalContext := false
	select {
	case <-ctx.Done():
		cancelledOriginalContext = true
	case <-testTimeout.C:
		t.Error("test timed out")
	}

	require.True(t, cancelledOriginalContext)
}

func TestWithPropagatedContextCancelWorksWhenContextHasNoCancellation(t *testing.T) {
	assert.NotPanics(t, func() {
		WithPropagatedContextCancel(context.Background(), context.Background())
	})

	goleak.VerifyNone(t)
}

func TestWithPropagatedContextCancelWorksWhenParentContextIsCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)

	cancelCtx1, cancelFn1 := context.WithCancel(context.Background())
	cancelCtx2, _ := context.WithCancel(context.Background())

	ctx, stopper := WithPropagatedContextCancel(cancelCtx1, cancelCtx2)
	defer stopper()

	// in this case there is no need to wait for the cancelCtx to cancel
	// since the parent context is closed before
	cancelFn1()

	testTimeout := time.NewTimer(10 * time.Second)
	select {
	case <-ctx.Done():
	case <-testTimeout.C:
		t.Error("sanity check #1 - we expect ctx to be cancelled")
	}

	select {
	case <-cancelCtx1.Done():
	case <-testTimeout.C:
		t.Error("sanity check #2 - we expect ctx to be cancelled")
	}
}

func TestWithPropagatedContextSanityCheckOriginalContextNeverAffected(t *testing.T) {
	defer goleak.VerifyNone(t)
	originalCtx, _ := context.WithCancel(context.Background())
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	ctx, stopper := WithPropagatedContextCancel(originalCtx, cancelCtx)

	cancelFn() // cancel should be propagated to "chained" ctx
	stopper()

	testTimeout := time.NewTimer(10 * time.Second)

	// cancellation of ctx is expected
	select {
	case <-ctx.Done():
		break
	case <-testTimeout.C:
		t.Error("test timed out")
	}

	// ... but cancellation of originalCtx is not
	// actually this is guaranteed by Go (contexts are immutable), but let's check anyway
	select {
	case <-originalCtx.Done():
		t.Error("originalCtx should not be cancelled")
	default:
	}
}
