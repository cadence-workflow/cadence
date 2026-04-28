package loadbalance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		mode     types.LoadBalancingMode
		wantType Balancer
		wantErr  bool
	}{
		{name: "naive", mode: types.LoadBalancingModeNAIVE, wantType: &naive{}},
		{name: "greedy", mode: types.LoadBalancingModeGREEDY, wantType: &greedy{}},
		{name: "invalid", mode: types.LoadBalancingModeINVALID, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(tt.mode, &store.NamespaceState{})
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, b)
				return
			}
			require.NoError(t, err)
			assert.IsType(t, tt.wantType, b)
		})
	}
}
