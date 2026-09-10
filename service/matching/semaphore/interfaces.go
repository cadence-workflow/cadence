package semaphore

import "context"

type (
	// Manager hands out the slots of one semaphore token bucket. Its free-set is only a cache;
	// the conditional write in persistence decides every grant. Always start or stop a manager,
	// since Acquire blocks until one of the two happens.
	Manager interface {
		// Start scans the partition and builds the free-set and the reverse index from what is
		// stored there. Call it exactly once, and discard the manager if it returns an error.
		Start(ctx context.Context) error
		// Stop gives up the bucket: later acquires get ErrNotReady.
		Stop()
		// Acquire asks for a slot on behalf of ownerID, and queues a waiter when this host
		// cannot find one.
		Acquire(ctx context.Context, ownerID string) (AcquireResult, error)
		// Identifier names the bucket this manager serves.
		Identifier() Identifier
	}
)
