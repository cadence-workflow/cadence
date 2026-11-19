package semaphore

import (
	"context"
	"time"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination interfaces_mock.go -self_package github.com/uber/cadence/service/matching/semaphore

type (
	// Manager serves one semaphore bucket. It stops itself when idle, when its load fails, or when
	// another host claims the bucket. A stopped manager returns ErrNotReady.
	Manager interface {
		// Start claims the bucket and loads its tokens. Only the first call loads.
		Start(ctx context.Context) error
		Stop()
		// Acquire grants ownerID a token, or writes it as a waiter and answers Queued.
		// A nil acquireDeadline means the waiter never expires.
		Acquire(ctx context.Context, ownerID string, acquireDeadline *time.Time) (AcquireResult, error)
		// Release frees the token ownerID holds. Releasing an owner that holds nothing succeeds.
		Release(ctx context.Context, ownerID string) error
		// GetTokenByOwner returns the token ownerID holds, or 0 if it holds none.
		GetTokenByOwner(ctx context.Context, ownerID string) (int, error)
		// Identifier names the bucket this manager serves.
		Identifier() Identifier
	}

	// SemaphoreRegistry tracks the managers this host serves. It doesn't start or stop them.
	SemaphoreRegistry interface {
		// GetOrCreate returns the manager for id, building one with create if none exists.
		// create runs under the registry lock, so it must be quick and must not call the registry.
		GetOrCreate(id Identifier, create func() (Manager, error)) (Manager, error)
		// Unregister removes mgr and reports whether it was the registered one.
		Unregister(mgr Manager) bool
		ManagerByIdentifier(id Identifier) (Manager, bool)
		AllManagers() []Manager
	}
)
