package store

import (
	"context"
	"fmt"

	"go.uber.org/fx"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=leaderstore_mock.go Elector,Election

// Elector is an interface that provides a way to establish a session for election.
// It establishes connection and a session and provides Election to run for leader.
type Elector interface {
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
	// Guard returns a transaction guard representing the current leadership term.
	// This guard can be passed to the generic store to perform leader-protected writes.
	Guard() GuardFunc
}

var (
	leaderStoreRegistry = make(map[string]Impl)
)

// RegisterLeaderStore registers store implementation in the registry.
func RegisterLeaderStore(name string, factory Impl) {
	leaderStoreRegistry[name] = factory
}

// LeaderModule returns registered a leader store fx.Option from the configuration.
// This can introduce extra dependency requirements to the fx application.
func LeaderModule(name string) fx.Option {
	factory, ok := leaderStoreRegistry[name]
	if !ok {
		panic(fmt.Sprintf("no leader store registered with name %s", name))
	}
	return factory
}
