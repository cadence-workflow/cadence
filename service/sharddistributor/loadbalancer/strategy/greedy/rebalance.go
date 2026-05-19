package greedy

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type moveCandidate struct {
	shardID         string
	from            string
	to              string
	assignmentIndex int
}

// PlanRebalance returns planned shard moves for the current assignment state.
func PlanRebalance(
	cfg config.LoadBalancingGreedyConfig,
	namespace string,
	namespaceState *store.NamespaceState,
	currentAssignments map[string][]string,
	now time.Time,
	logger log.Logger,
	metricsScope metrics.Scope,
) ([]plan.Move, error) {
	now = now.UTC()
	workingAssignments := cloneAssignments(currentAssignments)
	loads, meanLoad, ok := computeExecutorLoads(workingAssignments, namespaceState)
	if !ok {
		return nil, nil
	}

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
		shardLoad := namespaceState.ShardStats[move.ShardID].SmoothedLoad
		logGreedyMove(logger, loads, move, shardLoad)
		if metricsScope != nil {
			metricsScope.UpdateGauge(metrics.ShardDistributorAssignLoopMovedShardLoad, shardLoad)
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

func computeExecutorLoads(currentAssignments map[string][]string, state *store.NamespaceState) (map[string]float64, float64, bool) {
	loads := make(map[string]float64, len(currentAssignments))
	total := 0.0

	for executorID, shards := range currentAssignments {
		for _, shardID := range shards {
			stats, ok := state.ShardStats[shardID]
			if ok {
				loads[executorID] += stats.SmoothedLoad
				total += stats.SmoothedLoad
			}
		}
	}
	if len(loads) == 0 {
		return loads, 0, false
	}

	mean := total / float64(len(loads))
	return loads, mean, true
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
func planAndApplyNextMove(
	cfg config.LoadBalancingGreedyConfig,
	namespace string,
	namespaceState *store.NamespaceState,
	workingAssignments map[string][]string,
	loads map[string]float64,
	meanLoad float64,
	movedShards map[string]struct{},
	now time.Time,
) (plan.Move, bool, error) {
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

	destinationExecutor, ok := selectDestinationExecutor(
		destinationExecutors,
		workingAssignments,
		namespaceState,
		loads,
		meanLoad,
		cfg.SevereImbalanceRatio(namespace),
	)
	if !ok {
		return plan.Move{}, false, nil
	}

	candidate, found := findNextMoveCandidate(
		sourceExecutors,
		destinationExecutor,
		workingAssignments,
		namespaceState,
		loads,
		movedShards,
		now,
		cfg.PerShardCooldown(namespace),
	)
	if !found {
		return plan.Move{}, false, nil
	}

	if err := applyMoveCandidate(workingAssignments, candidate); err != nil {
		return plan.Move{}, false, err
	}

	movedShards[candidate.shardID] = struct{}{}
	updateExecutorLoadsAfterMove(namespaceState, candidate.from, candidate.to, loads, candidate.shardID)

	return plan.Move{
		ShardID: candidate.shardID,
		From:    candidate.from,
		To:      candidate.to,
	}, true, nil
}

// selectDestinationExecutor picks the least-loaded destination executor. If
// there are no destination executors, it falls back to all ACTIVE executors only
// when the namespace is severely imbalanced.
func selectDestinationExecutor(
	destinationExecutors []string,
	workingAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	loads map[string]float64,
	meanLoad float64,
	severeImbalanceRatio float64,
) (string, bool) {
	if len(destinationExecutors) == 0 {
		if !isSevereImbalance(loads, meanLoad, severeImbalanceRatio) {
			return "", false
		}
		allActiveExecutors := make([]string, 0, len(workingAssignments))
		for executorID := range workingAssignments {
			if namespaceState.Executors[executorID].Status == types.ExecutorStatusACTIVE {
				allActiveExecutors = append(allActiveExecutors, executorID)
			}
		}
		if len(allActiveExecutors) == 0 {
			return "", false
		}
		destinationExecutors = allActiveExecutors
	}

	return findBestDestination(destinationExecutors, loads)
}

// findNextMoveCandidate searches sources by descending load and returns the
// first eligible source/shard pair for the destination.
func findNextMoveCandidate(
	sourceExecutors []string,
	destinationExecutor string,
	workingAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	loads map[string]float64,
	movedShards map[string]struct{},
	now time.Time,
	perShardCooldown time.Duration,
) (moveCandidate, bool) {
	sortByDescendingLoad(sourceExecutors, loads)
	for _, sourceExecutor := range sourceExecutors {
		if sourceExecutor == destinationExecutor {
			continue
		}
		shardID, idx, found := findBestShardForMove(
			workingAssignments,
			namespaceState,
			sourceExecutor,
			destinationExecutor,
			loads,
			movedShards,
			now,
			perShardCooldown,
		)
		if !found {
			// No eligible shard for this source+destination (cooldown, or no beneficial move), try the next source.
			continue
		}

		return moveCandidate{
			shardID:         shardID,
			from:            sourceExecutor,
			to:              destinationExecutor,
			assignmentIndex: idx,
		}, true
	}

	return moveCandidate{}, false
}

func classifySourcesAndDestinations(
	executorLoads map[string]float64,
	state *store.NamespaceState,
	meanLoad float64,
	upperBand float64,
	lowerBand float64,
) ([]string, []string) {
	sources := make([]string, 0)
	destinations := make([]string, 0)

	for executorID, load := range executorLoads {
		executor := state.Executors[executorID]
		// Intentionally allow DRAINING executors as sources so they can shed shards
		if load > meanLoad*upperBand {
			sources = append(sources, executorID)
		} else if executor.Status == types.ExecutorStatusACTIVE && load < meanLoad*lowerBand {
			destinations = append(destinations, executorID)
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

func findBestDestination(destinationExecutors []string, executorLoads map[string]float64) (string, bool) {
	minExecutor := ""
	found := false
	var minLoad float64
	for _, executor := range destinationExecutors {
		load := executorLoads[executor]
		if !found || load < minLoad {
			minLoad = load
			minExecutor = executor
			found = true
		}
	}
	return minExecutor, found
}

func sortByDescendingLoad(executors []string, executorLoads map[string]float64) {
	slices.SortFunc(executors, func(a, b string) int {
		return cmp.Compare(executorLoads[b], executorLoads[a])
	})
}

func findBestShardForMove(
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

// computeBenefitOfMove returns the reduction in squared executor load from
// moving shardLoad from source to destination. Positive values mean the move
// improves balance between the two executors.
func computeBenefitOfMove(sourceLoad, destLoad, shardLoad float64) float64 {
	squaredLoadBeforeMove := sourceLoad*sourceLoad + destLoad*destLoad
	afterSourceLoad := sourceLoad - shardLoad
	afterDestLoad := destLoad + shardLoad
	squaredLoadAfterMove := afterSourceLoad*afterSourceLoad + afterDestLoad*afterDestLoad
	return squaredLoadBeforeMove - squaredLoadAfterMove
}

// applyMoveCandidate applies a planned move to the in-memory assignment state.
func applyMoveCandidate(currentAssignments map[string][]string, candidate moveCandidate) error {
	if candidate.assignmentIndex < 0 || candidate.assignmentIndex >= len(currentAssignments[candidate.from]) {
		return fmt.Errorf("candidate assignment index out of range for shard %s on source executor %s", candidate.shardID, candidate.from)
	}
	if currentAssignments[candidate.from][candidate.assignmentIndex] != candidate.shardID {
		return fmt.Errorf("candidate assignment index mismatch for shard %s on source executor %s", candidate.shardID, candidate.from)
	}

	currentAssignments[candidate.from][candidate.assignmentIndex] = currentAssignments[candidate.from][len(currentAssignments[candidate.from])-1]
	currentAssignments[candidate.from] = currentAssignments[candidate.from][:len(currentAssignments[candidate.from])-1]
	currentAssignments[candidate.to] = append(currentAssignments[candidate.to], candidate.shardID)
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

func logGreedyMove(logger log.Logger, loads map[string]float64, move plan.Move, shardLoad float64) {
	sourceLoadBefore := loads[move.From] + shardLoad
	destinationLoadBefore := loads[move.To] - shardLoad
	logger.Info("Greedy load-based shard move",
		tag.ShardKey(move.ShardID),
		tag.ShardExecutor(move.From),
		tag.Dynamic("destination_executor", move.To),
		tag.ShardLoad(fmt.Sprintf("%f", shardLoad)),
		tag.Dynamic("source_executor_load_before", sourceLoadBefore),
		tag.Dynamic("destination_executor_load_before", destinationLoadBefore),
	)
}
