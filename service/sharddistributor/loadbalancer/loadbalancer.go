package loadbalancer

import (
	"fmt"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/greedy"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type LoadBalance struct {
	cfg *config.Config
}

// Top:
// lb := LoadBalance{cfg: cfg}
//
//
// Længere nede:
// lb.Pick(state, namespace)

func (l LoadBalance) Pick(state *store.NamespaceState, namespace string) (string, error) {
	mode := l.cfg.GetLoadBalancingMode(namespace)

	switch mode {
	case types.LoadBalancingModeNAIVE:
		chosen, err := naive.InitialPlacement(state)
		if err != nil {
			return "", fmt.Errorf("failed to pick naive placement: %w", err)
		}

		return chosen, nil

	case types.LoadBalancingModeGREEDY:
		chosen, err := greedy.InitialPlacement(state)
		if err != nil {
			return "", fmt.Errorf("failed to pick initial placement: %w", err)
		}

		return chosen, nil
	default:
		return "", fmt.Errorf("unsupported load balancing mode: %s", mode)

	}

}
