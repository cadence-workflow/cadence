package store

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
)

func TestNamespaceState_CountExecutorsByStatus(t *testing.T) {
	tests := []struct {
		name      string
		executors map[string]HeartbeatState
		expected  map[types.ExecutorStatus]int
	}{
		{
			name:      "empty executors",
			executors: map[string]HeartbeatState{},
			expected:  map[types.ExecutorStatus]int{},
		},
		{
			name: "single active executor",
			executors: map[string]HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},
			},
			expected: map[types.ExecutorStatus]int{
				types.ExecutorStatusACTIVE: 1,
			},
		},
		{
			name: "multiple executors same status",
			executors: map[string]HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},
				"exec-2": {Status: types.ExecutorStatusACTIVE},
				"exec-3": {Status: types.ExecutorStatusACTIVE},
			},
			expected: map[types.ExecutorStatus]int{
				types.ExecutorStatusACTIVE: 3,
			},
		},
		{
			name: "all statuses",
			executors: map[string]HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusINVALID},
				"exec-2": {Status: types.ExecutorStatusACTIVE},
				"exec-3": {Status: types.ExecutorStatusDRAINING},
				"exec-4": {Status: types.ExecutorStatusDRAINED},
			},
			expected: map[types.ExecutorStatus]int{
				types.ExecutorStatusINVALID:  1,
				types.ExecutorStatusACTIVE:   1,
				types.ExecutorStatusDRAINING: 1,
				types.ExecutorStatusDRAINED:  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &NamespaceState{
				Executors: tt.executors,
			}
			result := ns.CountExecutorsByStatus()
			assert.Equal(t, tt.expected, result)
		})
	}
}
