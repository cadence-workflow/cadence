package taskdlq

import "context"

// HostLimiter bounds how many shards' DLQ processing runs may execute concurrently on
// one history host. The limit is fixed when the limiter is built; if it ever needs to
// change without a restart, replace the channel with a limiter that re-reads the
// dynamic property on each Acquire. A nil *HostLimiter imposes no bound.
type HostLimiter struct {
	sem chan struct{}
}

// NewHostLimiter returns a limiter allowing up to limit concurrent holders.
// A non-positive limit returns an unbounded limiter.
func NewHostLimiter(limit int) *HostLimiter {
	l := &HostLimiter{}
	if limit > 0 {
		l.sem = make(chan struct{}, limit)
	}
	return l
}

// Acquire blocks until a slot is free or ctx is done.
func (l *HostLimiter) Acquire(ctx context.Context) error {
	if l == nil || l.sem == nil {
		return ctx.Err()
	}
	select {
	case l.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot previously acquired with Acquire.
func (l *HostLimiter) Release() {
	if l == nil || l.sem == nil {
		return
	}
	<-l.sem
}
