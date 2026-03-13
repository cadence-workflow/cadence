package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResettableSignal_DoneWait(t *testing.T) {
	signal := NewResettableSignal()

	signal.Done()

	ctx := context.Background()
	err := signal.Wait(ctx)
	assert.NoError(t, err)
}

func TestResettableSignal_WaitDone(t *testing.T) {
	signal := NewResettableSignal()

	done := make(chan error)
	go func() {
		done <- signal.Wait(context.Background())
	}()

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	signal.Done()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not complete after Done")
	}
}

func TestResettableSignal_ContextCancellation(t *testing.T) {
	signal := NewResettableSignal()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error)
	go func() {
		done <- signal.Wait(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not complete after context cancellation")
	}
}

func TestResettableSignal_ResetWhileWaiting(t *testing.T) {
	signal := NewResettableSignal()

	done := make(chan error)
	go func() {
		done <- signal.Wait(context.Background())
	}()

	time.Sleep(10 * time.Millisecond)

	signal.Reset()

	select {
	case err := <-done:
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrReset), "expected ErrReset, got %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Wait did not complete after Reset")
	}
}

func TestResettableSignal_MultipleWaitersReset(t *testing.T) {
	signal := NewResettableSignal()

	const numWaiters = 5
	results := make([]chan error, numWaiters)
	for i := 0; i < numWaiters; i++ {
		i := i // capture the loop variable, the linter insists
		results[i] = make(chan error)
		go func() {
			results[i] <- signal.Wait(context.Background())
		}()
	}

	// Give goroutines time to start waiting
	time.Sleep(10 * time.Millisecond)

	signal.Reset()

	for i, ch := range results {
		select {
		case err := <-ch:
			assert.Error(t, err, "waiter %d should get error", i)
			assert.True(t, errors.Is(err, ErrReset), "waiter %d: expected ErrReset, got %v", i, err)
		case <-time.After(1 * time.Second):
			t.Fatalf("Waiter %d did not complete after Reset", i)
		}
	}
}

func TestResettableSignal_ResetAfterDone(t *testing.T) {
	signal := NewResettableSignal()

	// Terminate and reset the signal
	signal.Done()
	signal.Reset()

	// Now signal should be waiting again
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := signal.Wait(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestResettableSignal_ResetThenDoneThenWait(t *testing.T) {
	signal := NewResettableSignal()

	// Do a full cycle
	signal.Done()
	signal.Reset()
	signal.Done()

	err := signal.Wait(context.Background())
	assert.NoError(t, err)
}

func TestResettableSignal_IdempotentDone(t *testing.T) {
	signal := NewResettableSignal()

	// Call Done multiple times
	signal.Done()
	signal.Done()
	signal.Done()

	err := signal.Wait(context.Background())
	assert.NoError(t, err)
}
