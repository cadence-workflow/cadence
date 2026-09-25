package semaphore

import "context"

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go -self_package github.com/uber/cadence/service/matching/semaphore

type (
	// Manager hands out the slots of one semaphore token bucket. A manager has to be started
	// before it can serve, and stopped once it is done serving: Acquire waits on startup, or
	// until its own context deadline.
	Manager interface {
		// Start builds the free-set and the reverse index by scanning the partition. Safe to
		// call again and from several callers at once: the first loads, the rest wait for that
		// load, and a manager already running returns straight away. A failed load stops the
		// manager, so the next caller builds a fresh one. A stopped manager returns ErrNotReady.
		Start(ctx context.Context) error
		// Stop shuts the manager down: later acquires get ErrNotReady. Safe to call again, and
		// called by the manager itself once the bucket has gone IdleTTL without a request.
		Stop()
		// Acquire answers Acquired or NoSlot.
		Acquire(ctx context.Context, ownerID string) (AcquireResult, error)
		// Identifier names the bucket this manager serves.
		Identifier() Identifier
	}

	// SemaphoreRegistry tracks the managers this host is serving, keyed by identifier. It only
	// tracks them: starting and stopping belongs to whoever creates them.
	SemaphoreRegistry interface {
		// GetOrCreate returns the manager held or builds one with create for the given identifier.
		// A bucket never ends up with two managers. create runs under the registry lock, so it must not call back
		// in and must be quick.
		GetOrCreate(id Identifier, create func() (Manager, error)) (Manager, error)
		// Unregister drops mgr and reports whether it was the manager actually held.
		Unregister(mgr Manager) bool
		// ManagerByIdentifier returns the manager held for id, if there is one. It may still
		// be starting: Acquire is what waits for that.
		ManagerByIdentifier(id Identifier) (Manager, bool)
		// AllManagers returns a snapshot of what is registered.
		AllManagers() []Manager
	}
)
