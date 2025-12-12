package process

import (
	"math"
	"slices"
	"time"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type loadBalanceInputs struct {
	now                  time.Time
	moveBudgetProportion float64
	hysteresisUpperBand  float64
	hysteresisLowerBand  float64
	perShardCooldown     time.Duration
	structuralChange     bool
}

type executorLoadSnapshot struct {
	loads          map[string]float64
	totalLoad      float64
	latestMoveTime time.Time
}

func computeExecutorLoads(currentAssignments map[string][]string, namespaceState *store.NamespaceState) executorLoadSnapshot {
	loads := make(map[string]float64, len(currentAssignments))
	total := 0.0
	latestMove := time.Time{}

	for executorID, shards := range currentAssignments {
		for _, shardID := range shards {
			stats, ok := namespaceState.ShardStats[shardID]
			load := 0.0
			if ok {
				load = stats.SmoothedLoad
				if !stats.LastMoveTime.IsZero() && stats.LastMoveTime.After(latestMove) {
					latestMove = stats.LastMoveTime
				}
			}
			loads[executorID] += load
			total += load
		}
	}

	return executorLoadSnapshot{loads: loads, totalLoad: total, latestMoveTime: latestMove}
}

func shouldSkipLoadBalanceDueToGlobalCooldown(inputs loadBalanceInputs, latestMoveTime time.Time) bool {
	return !inputs.structuralChange &&
		inputs.perShardCooldown > 0 &&
		!latestMoveTime.IsZero() &&
		inputs.now.Sub(latestMoveTime) < inputs.perShardCooldown
}

// classifySourcesAndDestinations returns the source and destination executor sets for rebalancing.
func classifySourcesAndDestinations(
	executorLoads map[string]float64,
	namespaceState *store.NamespaceState,
	meanLoad float64,
	upperBand float64,
	lowerBand float64,
) (map[string]struct{}, map[string]struct{}) {
	sources := make(map[string]struct{})
	destinations := make(map[string]struct{})

	for executorID, load := range executorLoads {
		executor := namespaceState.Executors[executorID]
		if executor.Status == types.ExecutorStatusDRAINING || load > meanLoad*upperBand {
			sources[executorID] = struct{}{}
		} else if executor.Status == types.ExecutorStatusACTIVE && load < meanLoad*lowerBand {
			destinations[executorID] = struct{}{}
		}
	}

	return sources, destinations
}

// sourcesSortedByDescendingLoad orders sources by descending load so we prefer to
// move shards away from the hottest executors first. Exact ordering among equal loads is not important.
func sourcesSortedByDescendingLoad(sourceExecutors map[string]struct{}, executorLoads map[string]float64) []string {
	sources := make([]string, 0, len(sourceExecutors))
	for executorID := range sourceExecutors {
		sources = append(sources, executorID)
	}

	slices.SortFunc(sources, func(a, b string) int {
		la, lb := executorLoads[a], executorLoads[b]
		switch {
		case la > lb:
			return -1
		case la < lb:
			return 1
		default:
			return 0
		}
	})

	return sources
}

func computeMoveBudget(totalShards int, proportion float64) int {
	if totalShards <= 0 || proportion <= 0 {
		return 0
	}
	return int(math.Ceil(proportion * float64(totalShards)))
}

func (p *namespaceProcessor) findBestDestination(destinationExecutors map[string]struct{}, executorLoads map[string]float64) string {
	minLoad := math.MaxFloat64
	minExecutor := ""
	for executor := range destinationExecutors {
		load := executorLoads[executor]
		if load < minLoad {
			minLoad = load
			minExecutor = executor
		}
	}
	return minExecutor
}
