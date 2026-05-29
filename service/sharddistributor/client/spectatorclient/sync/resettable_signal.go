package sync

import (
	"context"
	"errors"
	"sync"
)

// ErrReset is returned by Wait() when Reset() is called while goroutines are waiting
var ErrReset = errors.New("signal was reset")

// resettableSignal is a synchronization primitive that allows waiting for a one-time
// signal with context support, and can be reset to wait for a new signal.
// Similar to sync.WaitGroup but for single completion events with reset capability.
type ResettableSignal struct {
	mu      sync.Mutex
	doneCh  chan struct{}
	resetCh chan struct{}
	done    bool
}

// NewResettableSignal creates a new resettable signal in waiting state
func NewResettableSignal() *ResettableSignal {
	return &ResettableSignal{
		doneCh:  make(chan struct{}),
		resetCh: make(chan struct{}),
	}
}

// Done signals that the event has completed. Safe to call multiple times (idempotent).
func (s *ResettableSignal) Done() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.done {
		s.done = true
		close(s.doneCh)
	}
}

// Wait blocks until either Done() is called, the context is cancelled, or Reset() is called.
// Returns:
//   - nil if Done() was called
//   - ctx.Err() if context was cancelled
//   - ErrReset if Reset() was called while waiting
func (s *ResettableSignal) Wait(ctx context.Context) error {
	s.mu.Lock()
	doneCh := s.doneCh
	resetCh := s.resetCh
	done := s.done
	s.mu.Unlock()

	// Fast path: already done
	if done {
		return nil
	}

	select {
	case <-doneCh:
		return nil
	case <-resetCh:
		return ErrReset
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reset resets the signal to waiting state. Any goroutines currently blocked in Wait()
// will immediately be unblocked with ErrReset.
func (s *ResettableSignal) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close reset channel to unblock any waiters (they'll get ErrReset)
	// Only close if not already done (to avoid closing a closed channel)
	if !s.done {
		close(s.resetCh)
	}

	// Create new channels and reset done flag
	s.doneCh = make(chan struct{})
	s.resetCh = make(chan struct{})
	s.done = false
}
