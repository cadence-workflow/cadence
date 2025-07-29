package store

import (
	"context"
	"fmt"

	"go.uber.org/fx"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=store_mock.go Store
//go:generate gowrap gen -g -p . -i Store -t ./wrappers/templates/metered.tmpl -o ./wrappers/metered/store_generated.go -v handler=Wrapped

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

// Store is a composite interface that combines all storage capabilities.
type Store interface {
	GetState(ctx context.Context, namespace string) (map[string]HeartbeatState, map[string]AssignedState, int64, error)
	AssignShards(ctx context.Context, namespace string, newState map[string]AssignedState, guard GuardFunc) error
	Subscribe(ctx context.Context, namespace string) (<-chan int64, error)
	DeleteExecutors(ctx context.Context, namespace string, executorIDs []string, guard GuardFunc) error

	GetHeartbeat(ctx context.Context, namespace string, executorID string) (*HeartbeatState, *AssignedState, error)
	RecordHeartbeat(ctx context.Context, namespace, executorID string, state HeartbeatState) error
}

// Impl could be used to build an implementation in the registry.
// We use registry based approach to avoid introduction of global etcd dependency.
type Impl fx.Option

var (
	storeRegistry = make(map[string]Impl)
)

// Register registers store implementation in the registry.
func Register(name string, factory Impl) {
	storeRegistry[name] = factory
}

// Module returns registered a leader store fx.Option from the configuration.
// This can introduce extra dependency requirements to the fx application.
func Module(name string) fx.Option {
	factory, ok := storeRegistry[name]
	if !ok {
		panic(fmt.Sprintf("no store registered with name %s", name))
	}
	return factory
}
