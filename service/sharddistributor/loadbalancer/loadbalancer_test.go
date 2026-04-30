package loadbalancer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestPlanInitialPlacement(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{name: "naive", mode: config.LoadBalancingModeNAIVE},
		{name: "greedy", mode: config.LoadBalancingModeGREEDY},
		{name: "invalid", mode: config.LoadBalancingModeINVALID, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				LoadBalancingMode: func(namespace string) string {
					return tt.mode
				},
			}
			assignments, err := PlanInitialPlacement(cfg, "test-namespace", &store.NamespaceState{}, nil)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, assignments)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, assignments)
		})
	}
}

func TestPlanInitialPlacement_NoActiveExecutors(t *testing.T) {
	cfg := &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return config.LoadBalancingModeNAIVE
		},
	}

	_, err := PlanInitialPlacement(cfg, "test-namespace", &store.NamespaceState{}, []string{"shard-1"})
	assert.ErrorContains(t, err, "no active executors available")
}
