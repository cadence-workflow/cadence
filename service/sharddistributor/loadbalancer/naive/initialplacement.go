package naive

import (
	"cmp"
	"fmt"
	"maps"
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// PlanInitialPlacement returns shardID -> executorID assignments for a batch of unassigned shards.
func PlanInitialPlacement(state *store.NamespaceState, shardIDs []string) (map[string]string, error) {
	counts := assignmentCounts(state)
	assignments := make(map[string]string, len(shardIDs))
	for _, shardID := range shardIDs {
		executorID, err := chooseExecutor(counts)
		if err != nil {
			return nil, err
		}
		assignments[shardID] = executorID
	}
	return assignments, nil
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
		return "", fmt.Errorf("no active executors available")
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
