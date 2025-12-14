package process

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type executorLoadSnapshot struct {
	loads          map[string]float64
	totalLoad      float64
	latestMoveTime time.Time
}

func (p *namespaceProcessor) loadBalance(
	currentAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	deletedShards map[string]store.ShardState,
	structuralChange bool,
	metricsScope metrics.Scope,
) (bool, error) {

	snapshot := computeExecutorLoads(currentAssignments, namespaceState)
	if len(snapshot.loads) == 0 {
		if metricsScope != nil {
			metricsScope.UpdateGauge(metrics.ShardDistributorLoadBalanceMovesPerCycle, 0)
		}
		return false, nil
	}

	now := p.timeSource.Now().UTC()
	perShardCooldown := p.cfg.LoadBalance.PerShardCooldown

	// Cooldown derived from persisted LastMoveTime, so it survives leader failover.
	// Only applies on load-only passes. Structural changes should not be throttled.
	if shouldSkipLoadBalanceDueToCooldown(structuralChange, perShardCooldown, now, snapshot.latestMoveTime) {
		if metricsScope != nil {
			metricsScope.AddCounter(metrics.ShardDistributorLoadBalanceGlobalCooldownSkips, 1)
			metricsScope.UpdateGauge(metrics.ShardDistributorLoadBalanceMovesPerCycle, 0)
		}
		return false, nil
	}

	meanLoad := snapshot.totalLoad / float64(len(snapshot.loads))

	moveBudgetProportion := p.cfg.LoadBalance.MoveBudgetProportion
	allShards := getShards(p.namespaceCfg, namespaceState, deletedShards)
	moveBudget := computeMoveBudget(len(allShards), moveBudgetProportion)
	shardsMoved := false
	movesPlanned := 0

	// Plan multiple moves per cycle (within budget), recomputing eligibility after each move.
	// Stop early once sources/destinations are empty, i.e. imbalance is within hysteresis bands.
	for moveBudget > 0 {
		sourceExecutors, destinationExecutors := classifySourcesAndDestinations(
			snapshot.loads,
			namespaceState,
			meanLoad,
			p.cfg.LoadBalance.HysteresisUpperBand,
			p.cfg.LoadBalance.HysteresisLowerBand,
		)

		if len(sourceExecutors) == 0 {
			break
		}

		// Escape hatch: if we have sources but no destinations under the normal lower band,
		// allow moving to the least-loaded ACTIVE executor when imbalance is severe.
		if len(destinationExecutors) == 0 {
			relaxed, ok := destinationsForSevereImbalance(
				snapshot.loads,
				meanLoad,
				p.cfg.LoadBalance.SevereImbalanceRatio,
				currentAssignments,
				namespaceState,
			)
			if !ok {
				break
			}
			destinationExecutors = relaxed
		}

		sources := sourcesSortedByDescendingLoad(sourceExecutors, snapshot.loads)

		destExecutor := p.findBestDestination(destinationExecutors, snapshot.loads)
		if destExecutor == "" {
			break
		}

		// Try sources in priority order to find a shard that is not in per-shard cooldown.
		// movedThisIteration tracks whether we actually performed a move in this recompute.
		// If no source has an eligible shard (e.g., all are cooling down), we stop early.
		movedThisIteration := false
		for _, sourceExecutor := range sources {
			if sourceExecutor == destExecutor {
				continue
			}
			shardsToMove := p.findShardsToMove(
				currentAssignments,
				namespaceState,
				sourceExecutor,
				destExecutor,
				snapshot.loads,
				now,
				perShardCooldown,
			)
			if len(shardsToMove) == 0 {
				// No eligible shard for this source+destination (cooldown, or no beneficial move), try the next source.
				continue
			}

			moved, err := p.moveShards(currentAssignments, sourceExecutor, destExecutor, shardsToMove)
			if err != nil {
				return false, err
			}
			if moved {
				movesPlanned += len(shardsToMove)
			}
			shardsMoved = shardsMoved || moved

			p.updateExecutorLoadsAfterMove(namespaceState, sourceExecutor, destExecutor, snapshot.loads, shardsToMove)
			moveBudget -= len(shardsToMove)
			movedThisIteration = moved
			break
		}

		// No eligible shard could be moved from any source.
		if !movedThisIteration {
			break
		}
	}

	if metricsScope != nil {
		metricsScope.UpdateGauge(metrics.ShardDistributorLoadBalanceMovesPerCycle, float64(movesPlanned))
	}
	return shardsMoved, nil
}

func destinationsForSevereImbalance(
	executorLoads map[string]float64,
	meanLoad float64,
	severeImbalanceRatio float64,
	currentAssignments map[string][]string,
	namespaceState *store.NamespaceState,
) (map[string]struct{}, bool) {
	maxLoad := 0.0
	for _, load := range executorLoads {
		if load > maxLoad {
			maxLoad = load
		}
	}

	severe := meanLoad > 0 &&
		severeImbalanceRatio > 0 &&
		maxLoad/meanLoad >= severeImbalanceRatio
	if !severe {
		return nil, false
	}

	relaxed := make(map[string]struct{})
	for executorID := range currentAssignments {
		if namespaceState.Executors[executorID].Status == types.ExecutorStatusACTIVE {
			relaxed[executorID] = struct{}{}
		}
	}
	if len(relaxed) == 0 {
		return nil, false
	}

	return relaxed, true
}

// shardMoveBenefitSquaredError returns the expected reduction in sum of squared error (SSE)
// around the mean load if we move a shard of size shardLoad from sourceLoad to destLoad.
// A positive value means the move improves overall load balance.
func shardMoveBenefitSquaredError(sourceLoad, destLoad, shardLoad float64) float64 {
	// SSE delta depends only on (sourceLoad, destLoad, shardLoad) since the mean stays constant
	// when moving load between executors (total load is conserved).
	// benefit = -(deltaSSE) = 2*w*(Ls-Ld) - 2*w^2
	w := shardLoad
	return 2*w*(sourceLoad-destLoad) - 2*w*w
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

// shouldSkipLoadBalanceDueToCooldown implements a cooldown for load-only balancing:
// if any shard moved recently (latestMoveTime within the cooldown window), skip this pass to reduce churn.
func shouldSkipLoadBalanceDueToCooldown(structuralChange bool, cooldown time.Duration, now time.Time, latestMoveTime time.Time) bool {
	return !structuralChange &&
		cooldown > 0 &&
		!latestMoveTime.IsZero() &&
		now.Sub(latestMoveTime) < cooldown
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
		if load > meanLoad*upperBand {
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

func (p *namespaceProcessor) findShardsToMove(
	currentAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	source string,
	destination string,
	executorLoads map[string]float64,
	now time.Time,
	perShardCooldown time.Duration,
) []string {
	// Pick a single eligible shard to move from source -> destination.
	//
	// For load-based balancing, prefer shards with the largest positive benefit (SSE reduction)
	// and skip shards that would not improve balance.
	bestBenefit := 0.0
	bestShard := ""
	sourceLoad := executorLoads[source]
	destLoad := executorLoads[destination]
	for _, shard := range currentAssignments[source] {
		stats, ok := namespaceState.ShardStats[shard]
		if !ok {
			continue
		}
		if perShardCooldown > 0 && !stats.LastMoveTime.IsZero() && now.Sub(stats.LastMoveTime) < perShardCooldown {
			continue
		}

		load := stats.SmoothedLoad

		benefit := shardMoveBenefitSquaredError(sourceLoad, destLoad, load)
		if benefit <= 0 {
			continue
		}
		if benefit > bestBenefit {
			bestBenefit = benefit
			bestShard = shard
		}
	}

	if bestShard == "" {
		return make([]string, 0)
	}
	return []string{bestShard}
}

func (p *namespaceProcessor) moveShards(currentAssignments map[string][]string, sourceExecutor string, destExecutor string, shardsToMove []string) (bool, error) {
	movedShards := false
	for _, shard := range shardsToMove {
		i := slices.Index(currentAssignments[sourceExecutor], shard)

		if i == -1 {
			return false, fmt.Errorf("shard %s not found in source executor %s", shard, sourceExecutor)
		}

		// Remove shard from source.
		currentAssignments[sourceExecutor][i] = currentAssignments[sourceExecutor][len(currentAssignments[sourceExecutor])-1]
		currentAssignments[sourceExecutor] = currentAssignments[sourceExecutor][:len(currentAssignments[sourceExecutor])-1]

		// Add shard to destination.
		currentAssignments[destExecutor] = append(currentAssignments[destExecutor], shard)
		movedShards = true
	}
	return movedShards, nil
}

func (p *namespaceProcessor) updateExecutorLoadsAfterMove(
	namespaceState *store.NamespaceState,
	source string,
	destination string,
	executorLoads map[string]float64,
	movedShards []string,
) {
	delta := 0.0
	for _, shardID := range movedShards {
		stats, ok := namespaceState.ShardStats[shardID]
		if !ok {
			continue
		}
		delta += stats.SmoothedLoad
	}
	executorLoads[source] -= delta
	executorLoads[destination] += delta
}
