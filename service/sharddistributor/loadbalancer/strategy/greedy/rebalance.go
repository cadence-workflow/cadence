package greedy

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// PlanRebalance returns planned shard moves for the current assignment state.
func PlanRebalance(
	cfg config.LoadBalancingGreedyConfig,
	namespace string,
	namespaceState *store.NamespaceState,
	currentAssignments map[string][]string,
	now time.Time,
	metricsScope metrics.Scope,
) ([]plan.Move, error) {
	now = now.UTC()
	workingAssignments := cloneAssignments(currentAssignments)
	loads, totalLoad := computeExecutorLoads(workingAssignments, namespaceState)
	if len(loads) == 0 {
		return nil, nil
	}

	meanLoad := totalLoad / float64(len(loads))
	totalShards := 0
	for _, shards := range currentAssignments {
		totalShards += len(shards)
	}
	moveBudget := computeMoveBudget(totalShards, cfg.MoveBudgetProportion(namespace))
	if moveBudget <= 0 {
		return nil, nil
	}
	moves := make([]plan.Move, 0, moveBudget)
	movedShards := make(map[string]struct{})

	// Plan multiple moves per cycle (within budget), recomputing eligibility after each move.
	// Stop early once sources/destinations are empty, i.e. imbalance is within hysteresis bands.
	for moveBudget > 0 {
		move, moved, err := planAndApplyNextMove(cfg, namespace, namespaceState, workingAssignments, loads, meanLoad, movedShards, now)
		if err != nil {
			return nil, err
		}
		if !moved {
			break
		}

		moves = append(moves, move)
		if metricsScope != nil {
			metricsScope.UpdateGauge(metrics.ShardDistributorAssignLoopMovedShardLoad, namespaceState.ShardStats[move.ShardID].SmoothedLoad)
		}
		moveBudget--
	}
	if len(moves) > 0 && metricsScope != nil {
		metricsScope.AddCounter(metrics.ShardDistributorAssignLoopLoadBasedMoves, int64(len(moves)))
	}
	return moves, nil
}

func cloneAssignments(assignments map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(assignments))
	for executorID, shardIDs := range assignments {
		clonedShards := make([]string, len(shardIDs))
		copy(clonedShards, shardIDs)
		cloned[executorID] = clonedShards
	}
	return cloned
}

func computeExecutorLoads(currentAssignments map[string][]string, state *store.NamespaceState) (map[string]float64, float64) {
	loads := make(map[string]float64, len(currentAssignments))
	total := 0.0

	for executorID, shards := range currentAssignments {
		for _, shardID := range shards {
			stats, ok := state.ShardStats[shardID]
			load := 0.0
			if ok {
				load = stats.SmoothedLoad
			}
			loads[executorID] += load
			total += load
		}
	}

	return loads, total
}

func computeMoveBudget(totalShards int, proportion float64) int {
	if totalShards <= 0 || proportion <= 0 {
		return 0
	}
	return int(math.Ceil(proportion * float64(totalShards)))
}

// planAndApplyNextMove attempts to plan one beneficial move and applies it to
// the in-memory working assignments, executor loads, and moved-shard set. It
// returns moved=false when no eligible move is available and the caller should
// stop the rebalance pass.
func planAndApplyNextMove(cfg config.LoadBalancingGreedyConfig, namespace string, namespaceState *store.NamespaceState, workingAssignments map[string][]string, loads map[string]float64, meanLoad float64, movedShards map[string]struct{}, now time.Time) (plan.Move, bool, error) {
	sourceExecutors, destinationExecutors := classifySourcesAndDestinations(
		loads,
		namespaceState,
		meanLoad,
		cfg.HysteresisUpperBand(namespace),
		cfg.HysteresisLowerBand(namespace),
	)
	if len(sourceExecutors) == 0 {
		return plan.Move{}, false, nil
	}

	// If we have sources but no destinations under the normal lower band,
	// allow moving to the least-loaded ACTIVE executor when imbalance is severe.
	if len(destinationExecutors) == 0 {
		if !isSevereImbalance(loads, meanLoad, cfg.SevereImbalanceRatio(namespace)) {
			return plan.Move{}, false, nil
		}
		relaxed := make(map[string]struct{})
		for executorID := range workingAssignments {
			if namespaceState.Executors[executorID].Status == types.ExecutorStatusACTIVE {
				relaxed[executorID] = struct{}{}
			}
		}
		if len(relaxed) == 0 {
			return plan.Move{}, false, nil
		}
		destinationExecutors = relaxed
	}

	destExecutor := findBestDestination(destinationExecutors, loads)
	if destExecutor == "" {
		return plan.Move{}, false, nil
	}

	// Try sources in priority order to find a shard that is not in per-shard cooldown.
	// If no source has an eligible shard (e.g., all are cooling down), stop early.
	for _, sourceExecutor := range sourcesSortedByDescendingLoad(sourceExecutors, loads) {
		if sourceExecutor == destExecutor {
			continue
		}
		shardToMove, idx, found := findShardToMove(
			workingAssignments,
			namespaceState,
			sourceExecutor,
			destExecutor,
			loads,
			movedShards,
			now,
			cfg.PerShardCooldown(namespace),
		)
		if !found {
			// No eligible shard for this source+destination (cooldown, or no beneficial move), try the next source.
			continue
		}

		if err := moveShard(workingAssignments, sourceExecutor, destExecutor, shardToMove, idx); err != nil {
			return plan.Move{}, false, err
		}
		movedShards[shardToMove] = struct{}{}
		updateExecutorLoadsAfterMove(namespaceState, sourceExecutor, destExecutor, loads, shardToMove)
		return plan.Move{
			ShardID: shardToMove,
			From:    sourceExecutor,
			To:      destExecutor,
		}, true, nil
	}

	return plan.Move{}, false, nil
}

func classifySourcesAndDestinations(
	executorLoads map[string]float64,
	state *store.NamespaceState,
	meanLoad float64,
	upperBand float64,
	lowerBand float64,
) (map[string]struct{}, map[string]struct{}) {
	sources := make(map[string]struct{})
	destinations := make(map[string]struct{})

	for executorID, load := range executorLoads {
		executor := state.Executors[executorID]
		// Intentionally allow DRAINING executors as sources so they can shed shards
		if load > meanLoad*upperBand {
			sources[executorID] = struct{}{}
		} else if executor.Status == types.ExecutorStatusACTIVE && load < meanLoad*lowerBand {
			destinations[executorID] = struct{}{}
		}
	}

	return sources, destinations
}

func isSevereImbalance(executorLoads map[string]float64, meanLoad, severeImbalanceRatio float64) bool {
	if meanLoad <= 0 || severeImbalanceRatio <= 0 {
		return false
	}

	maxLoad := 0.0
	for _, load := range executorLoads {
		if load > maxLoad {
			maxLoad = load
		}
	}
	return maxLoad/meanLoad >= severeImbalanceRatio
}

func findBestDestination(destinationExecutors map[string]struct{}, executorLoads map[string]float64) string {
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

func findShardToMove(
	currentAssignments map[string][]string,
	state *store.NamespaceState,
	source string,
	destination string,
	executorLoads map[string]float64,
	movedShards map[string]struct{},
	now time.Time,
	perShardCooldown time.Duration,
) (string, int, bool) {
	bestShard := ""

	sourceLoad := executorLoads[source]
	destLoad := executorLoads[destination]
	idx := -1

	bestBenefit := 0.0
	for i, shard := range currentAssignments[source] {
		if _, ok := movedShards[shard]; ok {
			continue
		}

		stats, ok := state.ShardStats[shard]
		if !ok {
			continue
		}
		if perShardCooldown > 0 && !stats.LastMoveTime.IsZero() && now.Sub(stats.LastMoveTime) < perShardCooldown {
			continue
		}

		load := stats.SmoothedLoad

		benefit := computeBenefitOfMove(sourceLoad, destLoad, load)
		if benefit <= 0 {
			continue
		}
		if benefit > bestBenefit {
			bestBenefit = benefit
			bestShard = shard
			idx = i
		}
	}

	return bestShard, idx, bestShard != ""
}

func computeBenefitOfMove(sourceLoad, destLoad, shardLoad float64) float64 {
	return 2*shardLoad*(sourceLoad-destLoad) - 2*shardLoad*shardLoad
}

func moveShard(currentAssignments map[string][]string, sourceExecutor string, destExecutor string, shardID string, idx int) error {
	// Defensive fallback in case index is stale.
	if idx < 0 || idx >= len(currentAssignments[sourceExecutor]) || currentAssignments[sourceExecutor][idx] != shardID {
		idx = slices.Index(currentAssignments[sourceExecutor], shardID)
	}
	if idx == -1 {
		return fmt.Errorf("shard %s not found in source executor %s", shardID, sourceExecutor)
	}

	currentAssignments[sourceExecutor][idx] = currentAssignments[sourceExecutor][len(currentAssignments[sourceExecutor])-1]
	currentAssignments[sourceExecutor] = currentAssignments[sourceExecutor][:len(currentAssignments[sourceExecutor])-1]
	currentAssignments[destExecutor] = append(currentAssignments[destExecutor], shardID)
	return nil
}

func updateExecutorLoadsAfterMove(
	state *store.NamespaceState,
	source string,
	destination string,
	executorLoads map[string]float64,
	shardID string,
) {
	stats, ok := state.ShardStats[shardID]
	if !ok {
		return
	}
	executorLoads[source] -= stats.SmoothedLoad
	executorLoads[destination] += stats.SmoothedLoad
}
