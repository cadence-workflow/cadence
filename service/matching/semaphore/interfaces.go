package semaphore

import "context"

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go -self_package github.com/uber/cadence/service/matching/semaphore

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

	// SemaphoreRegistry tracks the managers this host is serving, keyed by identifier. It only
	// tracks them: starting and stopping belongs to whoever creates them.
	SemaphoreRegistry interface {
		// Register adds mgr under its own identifier, replacing whatever was held for that
		// bucket.
		Register(mgr Manager)
		// Unregister drops mgr and reports whether it was the manager actually held.
		Unregister(mgr Manager) bool
		// ManagerByIdentifier returns the manager held for id, if there is one. It may still
		// be starting: Acquire is what waits for that.
		ManagerByIdentifier(id Identifier) (Manager, bool)
		// AllManagers returns a snapshot of what is registered.
		AllManagers() []Manager
	}
)
