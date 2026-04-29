package loadbalance

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestNaive(t *testing.T) {
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
		n := newNaive(state)

		// b has fewer shards, picked first; after increment a and b are tied at 2.
		first, err := n.Pick()
		require.NoError(t, err)
		assert.Equal(t, "b", first)

		// Draining executor must never be returned.
		for i := 0; i < 5; i++ {
			pick, err := n.Pick()
			require.NoError(t, err)
			assert.NotEqual(t, "c", pick)
		}
	})

	t.Run("empty active executors returns sentinel", func(t *testing.T) {
		n := newNaive(&store.NamespaceState{
			Executors: map[string]store.HeartbeatState{"a": {Status: types.ExecutorStatusDRAINING}},
		})
		_, err := n.Pick()
		assert.True(t, errors.Is(err, ErrNoActiveExecutors))
	})
}
