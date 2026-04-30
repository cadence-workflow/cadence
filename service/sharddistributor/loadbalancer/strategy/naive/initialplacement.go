package naive

import (
	"cmp"
	"maps"
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// InitialPlacement returns planned placements for a batch of unassigned shards.
func InitialPlacement(state *store.NamespaceState, shardIDs []string) ([]plan.Placement, error) {
	counts := assignmentCounts(state)
	placements := make([]plan.Placement, 0, len(shardIDs))
	for _, shardID := range shardIDs {
		executorID, err := chooseExecutor(counts)
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

func assignmentCounts(state *store.NamespaceState) map[string]int {
	counts := make(map[string]int, len(state.Executors))
	for executorID, executorState := range state.Executors {
		if executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		counts[executorID] = len(state.ShardAssignments[executorID].AssignedShards)
	}
	return counts
}

func chooseExecutor(counts map[string]int) (string, error) {
	if len(counts) == 0 {
		return "", plan.ErrNoActiveExecutors
	}
	chosen := slices.MinFunc(slices.Collect(maps.Keys(counts)), func(a, b string) int {
		return cmp.Or(
			cmp.Compare(counts[a], counts[b]),
			cmp.Compare(a, b),
		)
	})
	counts[chosen]++
	return chosen, nil
}
