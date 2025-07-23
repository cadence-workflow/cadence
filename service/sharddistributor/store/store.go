package store

import (
	"context"
	"fmt"
)

// ErrExecutorNotFound is an error that is returned when queries executor is not registered in the storage.
var ErrExecutorNotFound = fmt.Errorf("executor not found")

// Txn represents a generic, backend-agnostic transaction.
// It is used as a vehicle for the GuardFunc to operate on.
type Txn interface{}

// GuardFunc is a function that applies a transactional precondition.
// It takes a generic transaction, applies a backend-specific guard,
// and returns the modified transaction.
type GuardFunc func(Txn) (Txn, error)

// NopGuard is a no-op guard that can be used when no transactional
// check is required. It simply returns the transaction as-is.
func NopGuard() GuardFunc {
	return func(txn Txn) (Txn, error) {
		return txn, nil
	}
}

// HeartbeatStore defines the interface for recording executor heartbeats.
type HeartbeatStore interface {
	GetHeartbeat(ctx context.Context, namespace string, executorID string) (*HeartbeatState, error)
	RecordHeartbeat(ctx context.Context, namespace string, state HeartbeatState) error
}

// ShardStore defines the interface for shard management. Write operations
// can be protected by an optional transactional guard.
type ShardStore interface {
	GetState(ctx context.Context, namespace string) (map[string]HeartbeatState, map[string]AssignedState, int64, error)
	AssignShards(ctx context.Context, namespace string, newState map[string]AssignedState, guard GuardFunc) error
	Subscribe(ctx context.Context, namespace string) (<-chan int64, error)
	DeleteExecutors(ctx context.Context, namespace string, executorIDs []string, guard GuardFunc) error
}

// Store is a composite interface that combines all storage capabilities.
type Store interface {
	HeartbeatStore
	ShardStore
}
