package greedy

import (
	"cmp"
	"maps"
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type executorLoad struct {
	shardCount   int
	smoothedLoad float64
}

// PlanInitialPlacement returns planned placements for a batch of unassigned shards.
func PlanInitialPlacement(state *store.NamespaceState, shardIDs []string) ([]plan.Placement, error) {
	loads, averageShardLoad := executorLoads(state)
	placements := make([]plan.Placement, 0, len(shardIDs))
	for _, shardID := range shardIDs {
		executorID, err := chooseExecutor(loads, averageShardLoad)
		if err != nil {
			return nil, err
		}
		placements = append(placements, plan.Placement{
			ShardID:    shardID,
			ExecutorID: executorID,
		})
	}
	return placements, nil
}

func executorLoads(state *store.NamespaceState) (map[string]executorLoad, float64) {
	loads := make(map[string]executorLoad, len(state.Executors))
	totalSmoothedLoad := 0.0
	totalShardCount := 0

	for executorID, executorState := range state.Executors {
		if executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		var load executorLoad
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
	return loads, averageShardLoad
}

func chooseExecutor(loads map[string]executorLoad, averageShardLoad float64) (string, error) {
	if len(loads) == 0 {
		return "", plan.ErrNoActiveExecutors
	}
	chosen := slices.MinFunc(slices.Collect(maps.Keys(loads)), func(a, b string) int {
		la, lb := loads[a], loads[b]
		return cmp.Or(
			cmp.Compare(la.smoothedLoad, lb.smoothedLoad),
			cmp.Compare(la.shardCount, lb.shardCount),
			cmp.Compare(a, b),
		)
	})
	load := loads[chosen]
	load.shardCount++
	load.smoothedLoad += averageShardLoad
	loads[chosen] = load
	return chosen, nil
}
