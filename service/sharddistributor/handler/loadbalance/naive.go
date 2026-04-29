package loadbalance

import (
	"cmp"
	"maps"
	"slices"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// naive picks the ACTIVE executor with the fewest assigned shards.
type naive struct {
	counts map[string]int
}

func newNaive(state *store.NamespaceState) *naive {
	counts := make(map[string]int, len(state.Executors))
	for executorID, executorState := range state.Executors {
		if executorState.Status != types.ExecutorStatusACTIVE {
			continue
		}
		counts[executorID] = len(state.ShardAssignments[executorID].AssignedShards)
	}
	return &naive{counts: counts}
}

func (n *naive) Pick() (string, error) {
	if len(n.counts) == 0 {
		return "", ErrNoActiveExecutors
	}
	chosen := slices.MinFunc(slices.Collect(maps.Keys(n.counts)), func(a, b string) int {
		return cmp.Compare(n.counts[a], n.counts[b])
	})
	n.counts[chosen]++
	return chosen, nil
}
