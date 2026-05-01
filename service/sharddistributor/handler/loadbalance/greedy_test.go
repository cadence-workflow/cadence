package loadbalance

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestGreedy(t *testing.T) {
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
		g := newGreedy(state)

		// cold has the lowest smoothed load (2.0).
		first, err := g.Pick()
		require.NoError(t, err)
		assert.Equal(t, "cold", first)
		// After bumping cold by the namespace average, warm (2.5) becomes the lowest.
		second, err := g.Pick()
		require.NoError(t, err)
		assert.Equal(t, "warm", second)
	})

	t.Run("ties on smoothed load fall through to shard count (cold start)", func(t *testing.T) {
		// All shard stats missing — every executor's smoothedLoad is 0.
		// The pick must fall through to the count tie-breaker, picking "few".
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
		g := newGreedy(state)

		pick, err := g.Pick()
		require.NoError(t, err)
		assert.Equal(t, "few", pick)
	})

	t.Run("includes active executors with no assignments", func(t *testing.T) {
		state := &store.NamespaceState{
			Executors: map[string]store.HeartbeatState{
				"new": {Status: types.ExecutorStatusACTIVE},
			},
			ShardAssignments: map[string]store.AssignedState{},
		}
		g := newGreedy(state)

		pick, err := g.Pick()
		require.NoError(t, err)
		assert.Equal(t, "new", pick)
	})

	t.Run("empty active executors returns sentinel", func(t *testing.T) {
		g := newGreedy(&store.NamespaceState{})
		_, err := g.Pick()
		assert.True(t, errors.Is(err, ErrNoActiveExecutors))
	})
}
