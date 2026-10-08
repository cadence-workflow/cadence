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
