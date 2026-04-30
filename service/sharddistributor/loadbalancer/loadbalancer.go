package loadbalancer

import (
	"fmt"
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/greedy"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/naive"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// InitialPlacement returns shardID -> executorID assignments for a batch of unassigned shards.
func InitialPlacement(
	cfg *config.Config,
	namespace string,
	state *store.NamespaceState,
	shardIDs []string,
) (map[string]string, error) {
	mode := cfg.GetLoadBalancingMode(namespace)
	switch mode {
	case types.LoadBalancingModeNAIVE:
		return naive.InitialPlacement(state, shardIDs)
	case types.LoadBalancingModeGREEDY:
		return greedy.InitialPlacement(state, shardIDs)
	default:
		return nil, fmt.Errorf("unsupported load balancing mode: %s", mode)
	}
}

// Rebalance updates currentAssignments with planned shard moves and returns whether the plan changed.
func Rebalance(
	cfg *config.Config,
	namespace string,
	state *store.NamespaceState,
	currentAssignments map[string][]string,
	now time.Time,
	logger log.Logger,
	metricsScope metrics.Scope,
) (bool, error) {
	mode := cfg.GetLoadBalancingMode(namespace)
	switch mode {
	case types.LoadBalancingModeNAIVE:
		return naive.Rebalance(cfg.LoadBalancingNaive, namespace, state, currentAssignments, logger, metricsScope)
	case types.LoadBalancingModeGREEDY:
		return greedy.Rebalance(cfg.LoadBalancingGreedy, namespace, state, currentAssignments, now, metricsScope)
	default:
		return false, fmt.Errorf("unsupported load balancing mode: %s", mode)
	}
}
