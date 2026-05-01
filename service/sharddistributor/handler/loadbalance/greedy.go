package loadbalance

import (
	"cmp"
	"maps"
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// greedy picks the ACTIVE executor with the lowest smoothed load, with shard
// count as tie-breaker. After each pick, the chosen executor's load is bumped
// by the namespace average shard load so subsequent picks within the batch
// don't pile onto the same executor before its load reflects new shards.
type greedy struct {
	loads            map[string]greedyLoad
	averageShardLoad float64
}

type greedyLoad struct {
	shardCount   int
	smoothedLoad float64
}

func newGreedy(state *store.NamespaceState) *greedy {
	loads := make(map[string]greedyLoad, len(state.Executors))
	totalSmoothedLoad := 0.0
	totalShardCount := 0

	for executorID, executorState := range state.Executors {
		if executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		var load greedyLoad
		for shardID := range state.ShardAssignments[executorID].AssignedShards {
			load.shardCount++
			if stats, ok := state.ShardStats[shardID]; ok {
				load.smoothedLoad += stats.SmoothedLoad
			}
		}
		totalShardCount += load.shardCount
		totalSmoothedLoad += load.smoothedLoad
		loads[executorID] = load
	}
	var averageShardLoad float64
	if totalShardCount > 0 {
		averageShardLoad = totalSmoothedLoad / float64(totalShardCount)
	}
	return &greedy{loads: loads, averageShardLoad: averageShardLoad}
}

func (g *greedy) Pick() (string, error) {
	if len(g.loads) == 0 {
		return "", ErrNoActiveExecutors
	}
	chosen := slices.MinFunc(slices.Collect(maps.Keys(g.loads)), func(a, b string) int {
		la, lb := g.loads[a], g.loads[b]
		return cmp.Or(
			cmp.Compare(la.smoothedLoad, lb.smoothedLoad),
			cmp.Compare(la.shardCount, lb.shardCount),
		)
	})
	load := g.loads[chosen]
	load.shardCount++
	load.smoothedLoad += g.averageShardLoad
	g.loads[chosen] = load
	return chosen, nil
}
