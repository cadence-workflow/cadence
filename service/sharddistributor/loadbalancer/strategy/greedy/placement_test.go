package greedy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestPlanInitialPlacement(t *testing.T) {
	t.Run("picks lowest smoothed load and bumps by average after each pick", func(t *testing.T) {
		state := &store.NamespaceState{
			Executors: map[string]store.HeartbeatState{
				"hot":  {Status: types.ExecutorStatusACTIVE},
				"warm": {Status: types.ExecutorStatusACTIVE},
				"cold": {Status: types.ExecutorStatusACTIVE},
			},
			ShardAssignments: map[string]store.AssignedState{
				"hot":  {AssignedShards: map[string]*types.ShardAssignment{"s1": {}}},
				"warm": {AssignedShards: map[string]*types.ShardAssignment{"s2": {}, "s3": {}}},
				"cold": {AssignedShards: map[string]*types.ShardAssignment{"s4": {}, "s5": {}}},
			},
			ShardStats: map[string]store.ShardStatistics{
				"s1": {SmoothedLoad: 100.0},
				"s2": {SmoothedLoad: 1.5},
				"s3": {SmoothedLoad: 1.5},
				"s4": {SmoothedLoad: 1.0},
				"s5": {SmoothedLoad: 1.0},
			},
		}

		placements, err := PlanInitialPlacement(state, []string{"new-1", "new-2"})
		require.NoError(t, err)

		// cold has the lowest smoothed load. After bumping cold by the
		// namespace average, warm becomes the lowest.
		assert.Equal(t, []plan.Placement{
			{ShardID: "new-1", ExecutorID: "cold"},
			{ShardID: "new-2", ExecutorID: "warm"},
		}, placements)
	})

	t.Run("ties on smoothed load fall through to shard count", func(t *testing.T) {
		state := &store.NamespaceState{
			Executors: map[string]store.HeartbeatState{
				"few":  {Status: types.ExecutorStatusACTIVE},
				"many": {Status: types.ExecutorStatusACTIVE},
			},
			ShardAssignments: map[string]store.AssignedState{
				"few":  {AssignedShards: map[string]*types.ShardAssignment{"s1": {}}},
				"many": {AssignedShards: map[string]*types.ShardAssignment{"s2": {}, "s3": {}, "s4": {}}},
			},
		}

		placements, err := PlanInitialPlacement(state, []string{"new-1"})
		require.NoError(t, err)

		// All shard stats are missing, so smoothed loads tie and shard count breaks the tie.
		assert.Equal(t, []plan.Placement{{ShardID: "new-1", ExecutorID: "few"}}, placements)
	})

	t.Run("includes active executors with no assignments", func(t *testing.T) {
		state := &store.NamespaceState{
			Executors:        map[string]store.HeartbeatState{"new": {Status: types.ExecutorStatusACTIVE}},
			ShardAssignments: map[string]store.AssignedState{},
		}

		placements, err := PlanInitialPlacement(state, []string{"new-1"})
		require.NoError(t, err)
		assert.Equal(t, []plan.Placement{{ShardID: "new-1", ExecutorID: "new"}}, placements)
	})

	t.Run("empty active executors returns error", func(t *testing.T) {
		_, err := PlanInitialPlacement(&store.NamespaceState{}, []string{"new-1"})
		assert.True(t, errors.Is(err, plan.ErrNoActiveExecutors))
	})
}
