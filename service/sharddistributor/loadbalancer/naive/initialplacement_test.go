package naive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
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

		assignments, err := InitialPlacement(state, []string{"new-1", "new-2", "new-3"})
		require.NoError(t, err)

		// b has fewer shards, so the first new shard goes there.
		assert.Equal(t, "b", assignments["new-1"])

		// Draining executor must never receive planned assignments.
		assert.NotEqual(t, "c", assignments["new-2"])
		assert.NotEqual(t, "c", assignments["new-3"])
	})

	t.Run("empty active executors returns error", func(t *testing.T) {
		_, err := InitialPlacement(&store.NamespaceState{
			Executors: map[string]store.HeartbeatState{"a": {Status: types.ExecutorStatusDRAINING}},
		}, []string{"new-1"})
		assert.ErrorContains(t, err, "no active executors available")
	})
}
