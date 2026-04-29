package greedy

import (
	"cmp"
	"maps"
	"slices"

	"github.com/uber/cadence/service/sharddistributor/store"
)

func InitialPlacement(state *store.NamespaceState) (string, error) {
	if len(state.ShardAssignments) == 0 {
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
