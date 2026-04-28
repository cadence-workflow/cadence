package loadbalance

import (
	"errors"
	"fmt"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// ErrNoActiveExecutors is returned by Pick when no ACTIVE executors are
// available to host a shard.
var ErrNoActiveExecutors = errors.New("no active executors available")

// Balancer assigns shards to executors within a single batch.
// Each Pick returns the next executor and updates the in-batch load so
// repeated calls spread across executors.
type Balancer interface {
	Pick() (string, error)
}

// New returns a Balancer initialized from the current namespace state.
func New(mode types.LoadBalancingMode, state *store.NamespaceState) (Balancer, error) {
	switch mode {
	case types.LoadBalancingModeNAIVE:
		return newNaive(state), nil
	case types.LoadBalancingModeGREEDY:
		return newGreedy(state), nil
	default:
		return nil, fmt.Errorf("unsupported load balancing mode: %s", mode)
	}
}
