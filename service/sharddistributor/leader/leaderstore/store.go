package leaderstore

import (
	"context"
	"fmt"

	"go.uber.org/fx"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=leaderstore_mock.go Store,Election

// Store is an interface that provides a way to establish a session for election.
// It establishes connection and a session and provides Election to run for leader.
type Store interface {
	CreateElection(ctx context.Context, namespace string) (Election, error)
}

// Election is an interface that establishes leader campaign.
type Election interface {
	// Campaign is a blocking call that will block until either leadership is acquired and return nil or block.
	Campaign(ctx context.Context, hostname string) error
	// Resign resigns from leadership.
	Resign(ctx context.Context) error
	// Done returns a channel that notifies that the election session closed.
	Done() <-chan struct{}
	// Cleanup stops internal processes and releases keys.
	Cleanup(ctx context.Context) error
}

// StoreImpl could be used to build an implementation in the registry.
// We use registry based approach to avoid introduction of global etcd dependency.
type StoreImpl fx.Option

var (
	storeRegistry = make(map[string]StoreImpl)
)

// RegisterStore registers store implementation in the registry.
func RegisterStore(name string, factory StoreImpl) {
	storeRegistry[name] = factory
}

// StoreModule returns registered a leader store fx.Option from the configuration.
// This can introduce extra dependency requirements to the fx application.
func StoreModule(name string) fx.Option {
	factory, ok := storeRegistry[name]
	if !ok {
		panic(fmt.Sprintf("no leader store registered with name %s", name))
	}
	return factory
}
