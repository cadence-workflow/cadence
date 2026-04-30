package loadbalancer

import (
	"fmt"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/greedy"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/naive"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// PlanInitialPlacement returns shardID -> executorID assignments for a batch of unassigned shards.
func PlanInitialPlacement(
	cfg *config.Config,
	namespace string,
	state *store.NamespaceState,
	shardIDs []string,
) (map[string]string, error) {
	mode := cfg.GetLoadBalancingMode(namespace)
	switch mode {
	case types.LoadBalancingModeNAIVE:
		return naive.PlanInitialPlacement(state, shardIDs)
	case types.LoadBalancingModeGREEDY:
		return greedy.PlanInitialPlacement(state, shardIDs)
	default:
		return nil, fmt.Errorf("unsupported load balancing mode: %s", mode)
	}
}
