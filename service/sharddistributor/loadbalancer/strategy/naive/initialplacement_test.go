package naive

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestInitialPlacement(t *testing.T) {
	t.Run("picks fewest shards and increments after each pick", func(t *testing.T) {
		state := &store.NamespaceState{
			Executors: map[string]store.HeartbeatState{
				"a": {Status: types.ExecutorStatusACTIVE},
				"b": {Status: types.ExecutorStatusACTIVE},
				"c": {Status: types.ExecutorStatusDRAINING},
			},
			ShardAssignments: map[string]store.AssignedState{
				"a": {AssignedShards: map[string]*types.ShardAssignment{"s1": {}, "s2": {}}},
				"b": {AssignedShards: map[string]*types.ShardAssignment{"s3": {}}},
				"c": {AssignedShards: map[string]*types.ShardAssignment{"s4": {}}},
			},
		}

		placements, err := InitialPlacement(state, []string{"new-1", "new-2", "new-3"})
		require.NoError(t, err)

		// b has fewer shards, so the first new shard goes there.
		// Draining executor c must never receive planned placements.
		assert.Equal(t, []plan.Placement{
			{ShardID: "new-1", ExecutorID: "b"},
			{ShardID: "new-2", ExecutorID: "a"},
			{ShardID: "new-3", ExecutorID: "b"},
		}, placements)
	})

	t.Run("empty active executors returns error", func(t *testing.T) {
		_, err := InitialPlacement(&store.NamespaceState{
			Executors: map[string]store.HeartbeatState{"a": {Status: types.ExecutorStatusDRAINING}},
		}, []string{"new-1"})
		assert.True(t, errors.Is(err, plan.ErrNoActiveExecutors))
	})
}
