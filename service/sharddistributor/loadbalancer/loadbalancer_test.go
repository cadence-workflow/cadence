package loadbalancer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestInitialPlacement(t *testing.T) {
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
			assignments, err := InitialPlacement(cfg, "test-namespace", &store.NamespaceState{}, nil)
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

func TestInitialPlacement_NoActiveExecutors(t *testing.T) {
	cfg := &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return config.LoadBalancingModeNAIVE
		},
	}

	_, err := InitialPlacement(cfg, "test-namespace", &store.NamespaceState{}, []string{"shard-1"})
	assert.ErrorContains(t, err, "no active executors available")
}

func TestRebalance(t *testing.T) {
	cfg := &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return config.LoadBalancingModeINVALID
		},
	}
	changed, err := Rebalance(cfg, "test-namespace", &store.NamespaceState{}, nil, time.Time{}, nil, metrics.NoopScope)
	require.Error(t, err)
	assert.False(t, changed)
	assert.ErrorContains(t, err, "unsupported load balancing mode")
}
