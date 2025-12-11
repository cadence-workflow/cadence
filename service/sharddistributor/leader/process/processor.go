package process

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=process_mock.go Factory,Processor

// Module provides processor factory for fx app.
var Module = fx.Module(
	"leader-process",
	fx.Provide(NewProcessorFactory),
)

// Processor represents a process that runs when the instance is the leader
type Processor interface {
	Run(ctx context.Context) error
	Terminate(ctx context.Context) error
}

// Factory creates processor instances
type Factory interface {
	// CreateProcessor creates a new processor, it takes the generic store
	// and the election object which provides the transactional guard.
	CreateProcessor(cfg config.Namespace, storage store.Store, election store.Election) Processor
}

const (
	_defaultPeriod       = time.Second
	_defaultHeartbeatTTL = 10 * time.Second
	_defaultTimeout      = 1 * time.Second
	// Default cooldown between moving the same shard / applying consecutive moves.
	_defaultPerShardCooldown = time.Minute
	// Default fraction of total shards that may be moved per load-balance pass.
	_defaultMoveBudgetProportion = 0.01
	// Default hysteresis bands around mean load.
	_defaultHysteresisUpperBand = 1.15
	_defaultHysteresisLowerBand = 0.95
)

type processorFactory struct {
	logger        log.Logger
	timeSource    clock.TimeSource
	cfg           config.LeaderProcess
	metricsClient metrics.Client
}

type namespaceProcessor struct {
	namespaceCfg        config.Namespace
	logger              log.Logger
	metricsClient       metrics.Client
	timeSource          clock.TimeSource
	running             bool
	cancel              context.CancelFunc
	cfg                 config.LeaderProcess
	wg                  sync.WaitGroup
	shardStore          store.Store
	election            store.Election
	lastAppliedRevision int64
}

// NewProcessorFactory creates a new processor factory
func NewProcessorFactory(
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	cfg config.ShardDistribution,
) Factory {
	if cfg.Process.Period <= 0 {
		cfg.Process.Period = _defaultPeriod
	}
	if cfg.Process.HeartbeatTTL <= 0 {
		cfg.Process.HeartbeatTTL = _defaultHeartbeatTTL
	}
	if cfg.Process.Timeout <= 0 {
		cfg.Process.Timeout = _defaultTimeout
	}
	if cfg.Process.PerShardCooldown <= 0 {
		cfg.Process.PerShardCooldown = _defaultPerShardCooldown
	}
	if cfg.Process.MoveBudgetProportion <= 0 {
		cfg.Process.MoveBudgetProportion = _defaultMoveBudgetProportion
	}
	if cfg.Process.HysteresisUpperBand <= 0 {
		cfg.Process.HysteresisUpperBand = _defaultHysteresisUpperBand
	}
	if cfg.Process.HysteresisLowerBand <= 0 {
		cfg.Process.HysteresisLowerBand = _defaultHysteresisLowerBand
	}

	return &processorFactory{
		logger:        logger,
		timeSource:    timeSource,
		cfg:           cfg.Process,
		metricsClient: metricsClient,
	}
}

// CreateProcessor creates a new processor for the given namespace
func (f *processorFactory) CreateProcessor(cfg config.Namespace, shardStore store.Store, election store.Election) Processor {
	return &namespaceProcessor{
		namespaceCfg:  cfg,
		logger:        f.logger.WithTags(tag.ComponentLeaderProcessor, tag.ShardNamespace(cfg.Name)),
		timeSource:    f.timeSource,
		cfg:           f.cfg,
		shardStore:    shardStore,
		election:      election, // Store the election object
		metricsClient: f.metricsClient,
	}
}

// Run begins processing for this namespace
func (p *namespaceProcessor) Run(ctx context.Context) error {
	if p.running {
		return fmt.Errorf("processor is already running")
	}

	pCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true

	p.logger.Info("Starting")

	p.wg.Add(1)
	// Start the process in a goroutine
	go p.runProcess(pCtx)

	return nil
}

// Terminate halts processing for this namespace
func (p *namespaceProcessor) Terminate(ctx context.Context) error {
	if !p.running {
		return fmt.Errorf("processor has not been started")
	}

	p.logger.Info("Stopping")

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	p.running = false

	// Ensure that the process has stopped.
	p.wg.Wait()

	return nil
}

// runProcess launches and manages the processing loops.
func (p *namespaceProcessor) runProcess(ctx context.Context) {
	defer p.wg.Done()

	var loopWg sync.WaitGroup
	loopWg.Add(2) // We have two loops to manage.

	// Launch the assignment and executor cleanup process in its own goroutine.
	go func() {
		defer loopWg.Done()
		p.runRebalancingLoop(ctx)
	}()

	// Launch the shard stats cleanup process in its own goroutine.
	go func() {
		defer loopWg.Done()
		p.runShardStatsCleanupLoop(ctx)
	}()

	// Wait for both loops to exit.
	loopWg.Wait()
}

// runRebalancingLoop handles shard assignment and redistribution.
func (p *namespaceProcessor) runRebalancingLoop(ctx context.Context) {
	ticker := p.timeSource.NewTicker(p.cfg.Period)
	defer ticker.Stop()

	// Perform an initial rebalance on startup.
	if p.namespaceCfg.Mode == config.MigrationModeONBOARDED {
		err := p.rebalanceShards(ctx)
		if err != nil {
			p.logger.Error("initial rebalance failed", tag.Error(err))
		}
	}

	updateChan, err := p.shardStore.Subscribe(ctx, p.namespaceCfg.Name)
	if err != nil {
		p.logger.Error("Failed to subscribe to state changes, stopping rebalancing loop.", tag.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Rebalancing loop cancelled.")
			return
		case latestRevision, ok := <-updateChan:
			if !ok {
				p.logger.Info("Update channel closed, stopping rebalancing loop.")
				return
			}
			if latestRevision <= p.lastAppliedRevision {
				continue
			}
			if p.namespaceCfg.Mode != config.MigrationModeONBOARDED {
				p.logger.Info("Namespace not onboarded, rebalance not triggered", tag.ShardNamespace(p.namespaceCfg.Name))
				break
			}
			p.logger.Info("State change detected, triggering rebalance.")
			err = p.rebalanceShards(ctx)
		case <-ticker.Chan():
			if p.namespaceCfg.Mode != config.MigrationModeONBOARDED {
				p.logger.Info("Namespace not onboarded, skipped periodic reconciliation", tag.ShardNamespace(p.namespaceCfg.Name))
				break
			}
			p.logger.Info("Periodic reconciliation triggered, rebalancing.")
			err = p.rebalanceShards(ctx)
		}
		if err != nil {
			p.logger.Error("rebalance failed", tag.Error(err))
		}
	}
}

// runShardStatsCleanupLoop periodically removes stale shard statistics.
func (p *namespaceProcessor) runShardStatsCleanupLoop(ctx context.Context) {
	ticker := p.timeSource.NewTicker(p.cfg.HeartbeatTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Shard stats cleanup loop cancelled.")
			return
		case <-ticker.Chan():
			p.logger.Info("Periodic shard stats cleanup triggered.")
			namespaceState, err := p.shardStore.GetState(ctx, p.namespaceCfg.Name)
			if err != nil {
				p.logger.Error("Failed to get state for shard stats cleanup", tag.Error(err))
				continue
			}
			staleShardStats := p.identifyStaleShardStats(namespaceState)
			if len(staleShardStats) == 0 {
				// No stale shard stats to delete
				continue
			}
			if err := p.shardStore.DeleteShardStats(ctx, p.namespaceCfg.Name, staleShardStats, p.election.Guard()); err != nil {
				p.logger.Error("Failed to delete stale shard stats", tag.Error(err))
			}
		}
	}
}

// identifyStaleExecutors returns a list of executors who have not reported a heartbeat recently.
func (p *namespaceProcessor) identifyStaleExecutors(namespaceState *store.NamespaceState) map[string]int64 {
	expiredExecutors := make(map[string]int64)
	now := p.timeSource.Now().UTC()

	for executorID, state := range namespaceState.Executors {
		if now.Sub(state.LastHeartbeat) > p.cfg.HeartbeatTTL {
			p.logger.Info("Executor has not reported a heartbeat recently", tag.ShardExecutor(executorID), tag.ShardNamespace(p.namespaceCfg.Name), tag.Value(state.LastHeartbeat))
			expiredExecutors[executorID] = namespaceState.ShardAssignments[executorID].ModRevision
		}
	}

	return expiredExecutors
}

// identifyStaleShardStats returns a list of shard statistics that are no longer relevant.
func (p *namespaceProcessor) identifyStaleShardStats(namespaceState *store.NamespaceState) []string {
	activeShards := make(map[string]struct{})
	now := p.timeSource.Now().UTC()

	// 1. build set of active executors

	// add all assigned shards from executors that are ACTIVE and not stale
	for executorID, assignedState := range namespaceState.ShardAssignments {
		executor, exists := namespaceState.Executors[executorID]
		if !exists {
			continue
		}

		isActive := executor.Status == types.ExecutorStatusACTIVE
		isNotStale := now.Sub(executor.LastHeartbeat) <= p.cfg.HeartbeatTTL
		if isActive && isNotStale {
			for shardID := range assignedState.AssignedShards {
				activeShards[shardID] = struct{}{}
			}
		}
	}

	// add all shards in ReportedShards where the status is not DONE
	for _, heartbeatState := range namespaceState.Executors {
		for shardID, shardStatusReport := range heartbeatState.ReportedShards {
			if shardStatusReport.Status != types.ShardStatusDONE {
				activeShards[shardID] = struct{}{}
			}
		}
	}

	// 2. build set of stale shard stats

	// append all shard stats that are not in the active shards set
	var staleShardStats []string
	for shardID, stats := range namespaceState.ShardStats {
		if _, ok := activeShards[shardID]; ok {
			continue
		}
		recentUpdate := !stats.LastUpdateTime.IsZero() && now.Sub(stats.LastUpdateTime) <= p.cfg.HeartbeatTTL
		recentMove := !stats.LastMoveTime.IsZero() && now.Sub(stats.LastMoveTime) <= p.cfg.HeartbeatTTL
		if recentUpdate || recentMove {
			// Preserve stats that have been updated recently to allow cooldown/load history to
			// survive executor churn. These shards are likely awaiting reassignment,
			// so we don't want to delete them.
			continue
		}
		staleShardStats = append(staleShardStats, shardID)
	}

	return staleShardStats
}

// rebalanceShards is the core logic for distributing shards among active executors.
func (p *namespaceProcessor) rebalanceShards(ctx context.Context) (err error) {
	metricsLoopScope := p.metricsClient.Scope(
		metrics.ShardDistributorAssignLoopScope,
		metrics.NamespaceTag(p.namespaceCfg.Name),
		metrics.NamespaceTypeTag(p.namespaceCfg.Type),
	)

	metricsLoopScope.AddCounter(metrics.ShardDistributorAssignLoopAttempts, 1)
	defer func() {
		if err != nil {
			metricsLoopScope.AddCounter(metrics.ShardDistributorAssignLoopFail, 1)
		} else {
			metricsLoopScope.AddCounter(metrics.ShardDistributorAssignLoopSuccess, 1)
		}
	}()

	start := p.timeSource.Now()
	defer func() {
		metricsLoopScope.RecordHistogramDuration(metrics.ShardDistributorAssignLoopShardRebalanceLatency, p.timeSource.Now().Sub(start))
	}()

	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	return p.rebalanceShardsImpl(ctx, metricsLoopScope)
}

func (p *namespaceProcessor) rebalanceShardsImpl(ctx context.Context, metricsLoopScope metrics.Scope) (err error) {
	namespaceState, err := p.shardStore.GetState(ctx, p.namespaceCfg.Name)
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	p.lastAppliedRevision = namespaceState.GlobalRevision

	// Identify stale executors that need to be removed
	staleExecutors := p.identifyStaleExecutors(namespaceState)
	if len(staleExecutors) > 0 {
		p.logger.Info("Identified stale executors for removal", tag.ShardExecutors(slices.Collect(maps.Keys(staleExecutors))))
	}

	activeExecutors := p.getActiveExecutors(namespaceState, staleExecutors)
	if len(activeExecutors) == 0 {
		p.logger.Info("No active executors found. Cannot assign shards.")
		return nil
	}
	deletedShards := p.findDeletedShards(namespaceState)
	shardsToReassign, currentAssignments := p.findShardsToReassign(activeExecutors, namespaceState, deletedShards, staleExecutors)

	metricsLoopScope.UpdateGauge(metrics.ShardDistributorAssignLoopNumRebalancedShards, float64(len(shardsToReassign)))

	// If there are deleted shards or stale executors, the distribution has changed.
	assignedToEmptyExecutors := assignShardsToEmptyExecutors(currentAssignments)
	updatedAssignments := p.updateAssignments(shardsToReassign, activeExecutors, currentAssignments)
	// structuralChange means we must reconcile for liveness/namespace-structure reasons.
	// Cooldowns only gate load-only moves, while structural changes apply immediately.
	structuralChange := len(deletedShards) > 0 || len(staleExecutors) > 0 || assignedToEmptyExecutors || updatedAssignments
	loadBalanceChange, err := p.loadBalance(currentAssignments, namespaceState, deletedShards, structuralChange, metricsLoopScope)
	if err != nil {
		return fmt.Errorf("load balance: %w", err)
	}

	distributionChanged := structuralChange || loadBalanceChange
	if !distributionChanged {
		p.logger.Info("No changes to distribution detected. Skipping rebalance.")
		return nil
	}

	p.addAssignmentsToNamespaceState(namespaceState, currentAssignments)
	p.logger.Info("Applying new shard distribution.")

	// Use the leader guard for the assign and delete operation.
	err = p.shardStore.AssignShards(ctx, p.namespaceCfg.Name, store.AssignShardsRequest{
		NewState:          namespaceState,
		ExecutorsToDelete: staleExecutors,
	}, p.election.Guard())
	if err != nil {
		return fmt.Errorf("assign shards: %w", err)
	}

	totalActiveShards := 0
	for _, assignedState := range namespaceState.ShardAssignments {
		totalActiveShards += len(assignedState.AssignedShards)
	}
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorActiveShards, float64(totalActiveShards))

	return nil
}

func (p *namespaceProcessor) findDeletedShards(namespaceState *store.NamespaceState) map[string]store.ShardState {
	deletedShards := make(map[string]store.ShardState)

	for executorID, executor := range namespaceState.Executors {
		for shardID, shardState := range executor.ReportedShards {
			if shardState.Status == types.ShardStatusDONE {
				deletedShards[shardID] = store.ShardState{
					ExecutorID: executorID,
				}
			}
		}
	}
	return deletedShards
}

func (p *namespaceProcessor) findShardsToReassign(
	activeExecutors []string,
	namespaceState *store.NamespaceState,
	deletedShards map[string]store.ShardState,
	staleExecutors map[string]int64,
) ([]string, map[string][]string) {
	allShards := make(map[string]struct{})
	for _, shardID := range getShards(p.namespaceCfg, namespaceState, deletedShards) {
		allShards[shardID] = struct{}{}
	}

	shardsToReassign := make([]string, 0)
	currentAssignments := make(map[string][]string)

	for _, executorID := range activeExecutors {
		currentAssignments[executorID] = []string{}
	}

	for executorID, state := range namespaceState.ShardAssignments {
		isActive := namespaceState.Executors[executorID].Status == types.ExecutorStatusACTIVE
		_, isStale := staleExecutors[executorID]

		for shardID := range state.AssignedShards {
			if _, ok := allShards[shardID]; ok {
				delete(allShards, shardID)
				// If executor is active AND not stale, keep the assignment
				if isActive && !isStale {
					currentAssignments[executorID] = append(currentAssignments[executorID], shardID)
				} else {
					// Otherwise, reassign the shard (executor is either inactive or stale)
					shardsToReassign = append(shardsToReassign, shardID)
				}
			}
		}
	}

	for shardID := range allShards {
		shardsToReassign = append(shardsToReassign, shardID)
	}
	return shardsToReassign, currentAssignments
}

func (*namespaceProcessor) updateAssignments(shardsToReassign []string, activeExecutors []string, currentAssignments map[string][]string) (distributionChanged bool) {
	if len(shardsToReassign) == 0 {
		return false
	}

	i := rand.Intn(len(activeExecutors))
	for _, shardID := range shardsToReassign {
		executorID := activeExecutors[i%len(activeExecutors)]
		currentAssignments[executorID] = append(currentAssignments[executorID], shardID)
		i++
	}
	return true
}

func (p *namespaceProcessor) loadBalance(
	currentAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	deletedShards map[string]store.ShardState,
	structuralChange bool,
	metricsScope metrics.Scope,
) (bool, error) {

	shardsMoved := false

	allShards := getShards(p.namespaceCfg, namespaceState, deletedShards)

	moveBudgetProportion := p.cfg.MoveBudgetProportion
	hysteresisUpperBand := p.cfg.HysteresisUpperBand
	hysteresisLowerBand := p.cfg.HysteresisLowerBand

	moveBudget := int(math.Ceil(moveBudgetProportion * float64(len(allShards))))

	now := p.timeSource.Now().UTC()
	// PerShardCooldown is the minimum time between moving the same shard.
	perShardCooldown := p.cfg.PerShardCooldown

	executorLoads := make(map[string]float64, len(currentAssignments))
	totalLoad := 0.0
	latestMoveTime := time.Time{}
	// Compute executor loads from the in-memory assignment plan.
	// Track latest shard move time for the global cooldown.
	for executorID, shards := range currentAssignments {
		for _, shardID := range shards {
			stats, ok := namespaceState.ShardStats[shardID]
			load := 0.0
			if ok {
				load = stats.SmoothedLoad
				if !stats.LastMoveTime.IsZero() && stats.LastMoveTime.After(latestMoveTime) {
					latestMoveTime = stats.LastMoveTime
				}
			}
			executorLoads[executorID] += load
			totalLoad += load
		}
	}
	if len(executorLoads) == 0 {
		if metricsScope != nil {
			metricsScope.UpdateGauge(metrics.ShardDistributorLoadBalanceMovesPerCycle, 0)
		}
		return false, nil
	}

	// Global cooldown derived from persisted LastMoveTime, so it survives leader failover.
	// Only applies on load-only passes. Structural changes should not be throttled.
	if !structuralChange && perShardCooldown > 0 && !latestMoveTime.IsZero() && now.Sub(latestMoveTime) < perShardCooldown {
		if metricsScope != nil {
			metricsScope.AddCounter(metrics.ShardDistributorLoadBalanceGlobalCooldownSkips, 1)
			metricsScope.UpdateGauge(metrics.ShardDistributorLoadBalanceMovesPerCycle, 0)
		}
		return false, nil
	}

	meanLoad := totalLoad / float64(len(executorLoads))

	movesPlanned := 0
	// Plan multiple moves per cycle (within budget), recomputing eligibility after each move.
	// Stop early once sources/destinations are empty, i.e. imbalance is within hysteresis bands.
	for moveBudget > 0 {
		sourceExecutors := make(map[string]struct{})
		destinationExecutors := make(map[string]struct{})

		for executorID, load := range executorLoads {
			executor := namespaceState.Executors[executorID]
			if executor.Status == types.ExecutorStatusDRAINING || load > meanLoad*hysteresisUpperBand {
				sourceExecutors[executorID] = struct{}{}
			} else if executor.Status == types.ExecutorStatusACTIVE && load < meanLoad*hysteresisLowerBand {
				destinationExecutors[executorID] = struct{}{}
			}
		}

		// Nothing to do once we're within bands (or have no eligible destinations).
		if len(sourceExecutors) == 0 || len(destinationExecutors) == 0 {
			break
		}

		// Deterministic iteration over sources (ordered by load desc, then ID) reduces churn/oscillation.
		// Without sorting, identical state can yield different move sequences across cycles, making shards more likely to bounce near thresholds.
		// Executor count is small, so this sort is cheap compared to shard movement cost.
		sources := make([]string, 0, len(sourceExecutors))
		for executorID := range sourceExecutors {
			sources = append(sources, executorID)
		}
		slices.SortFunc(sources, func(a, b string) int {
			la, lb := executorLoads[a], executorLoads[b]
			if la == lb {
				if a < b {
					return -1
				}
				if a > b {
					return 1
				}
				return 0
			}
			if la > lb {
				return -1
			}
			return 1
		})

		destExecutor := p.findBestDestination(destinationExecutors, executorLoads)
		if destExecutor == "" {
			break
		}

		// Try sources in priority order to find a shard that is not in per-shard cooldown.
		// movedThisIteration tracks whether we actually performed a move in this recompute.
		// If no source has an eligible shard (e.g., all are cooling down), we stop early.
		movedThisIteration := false
		for _, sourceExecutor := range sources {
			shardsToMove := p.findShardsToMove(currentAssignments, namespaceState, sourceExecutor, now, perShardCooldown)
			if len(shardsToMove) == 0 {
				// All shards on this source are in cooldown; try the next source.
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

			p.updateLoad(currentAssignments, namespaceState, sourceExecutor, destExecutor, executorLoads)
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

func (p *namespaceProcessor) findBestDestination(destinationExecutors map[string]struct{}, executorLoads map[string]float64) string {
	minLoad := math.MaxFloat64
	minExecutor := ""
	for executor := range destinationExecutors {
		load := executorLoads[executor]
		// Tie-break on executor ID to keep destination selection stable when loads are equal.
		if load < minLoad || (load == minLoad && (minExecutor == "" || executor < minExecutor)) {
			minLoad = executorLoads[executor]
			minExecutor = executor
		}
	}
	return minExecutor
}

func (p *namespaceProcessor) findShardsToMove(
	currentAssignments map[string][]string,
	namespaceState *store.NamespaceState,
	source string,
	now time.Time,
	perShardCooldown time.Duration,
) []string {
	// Pick the hottest eligible shard on the source executor.
	// Recently moved shards (within cooldown) are skipped.
	largestLoad := -1.0
	largestShard := ""
	for _, shard := range currentAssignments[source] {
		stats, ok := namespaceState.ShardStats[shard]
		if ok && perShardCooldown > 0 && !stats.LastMoveTime.IsZero() && now.Sub(stats.LastMoveTime) < perShardCooldown {
			continue
		}

		load := 0.0
		if ok {
			load = stats.SmoothedLoad
		}

		if load > largestLoad || (load == largestLoad && (largestShard == "" || shard < largestShard)) {
			largestLoad = load
			largestShard = shard
		}
	}
	if largestShard == "" {
		return make([]string, 0)
	}
	return []string{largestShard}
}

func (p *namespaceProcessor) moveShards(currentAssignments map[string][]string, sourceExecutor string, destExecutor string, shardsToMove []string) (bool, error) {
	movedShards := false
	for _, shard := range shardsToMove {
		i := slices.Index(currentAssignments[sourceExecutor], shard)

		if i == -1 {
			return false, fmt.Errorf("shard %s not found in source executor %s", shard, sourceExecutor)
		}

		// Remove shard from source
		currentAssignments[sourceExecutor][i] = currentAssignments[sourceExecutor][len(currentAssignments[sourceExecutor])-1]
		currentAssignments[sourceExecutor] = currentAssignments[sourceExecutor][:len(currentAssignments[sourceExecutor])-1]

		// Add shard to destination
		currentAssignments[destExecutor] = append(currentAssignments[destExecutor], shard)
		movedShards = true
	}
	return movedShards, nil
}

func (p *namespaceProcessor) updateLoad(currentAssignments map[string][]string, namespaceState *store.NamespaceState, source, destination string, executorLoads map[string]float64) {
	executorLoads[source] = 0
	for _, shard := range currentAssignments[source] {
		executorLoads[source] += namespaceState.ShardStats[shard].SmoothedLoad
	}
	executorLoads[destination] = 0
	for _, shard := range currentAssignments[destination] {
		executorLoads[destination] += namespaceState.ShardStats[shard].SmoothedLoad
	}

}

func (p *namespaceProcessor) addAssignmentsToNamespaceState(namespaceState *store.NamespaceState, currentAssignments map[string][]string) {
	newState := make(map[string]store.AssignedState, len(currentAssignments))

	for executorID, shards := range currentAssignments {
		assignedShardsMap := make(map[string]*types.ShardAssignment)

		for _, shardID := range shards {
			assignedShardsMap[shardID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		}

		modRevision := int64(0) // Should be 0 if we have not seen it yet
		if namespaceAssignments, ok := namespaceState.ShardAssignments[executorID]; ok {
			modRevision = namespaceAssignments.ModRevision
		}

		newState[executorID] = store.AssignedState{
			AssignedShards:     assignedShardsMap,
			LastUpdated:        p.timeSource.Now().UTC(),
			ModRevision:        modRevision,
			ShardHandoverStats: p.addHandoverStatsToExecutorAssignedState(namespaceState, executorID, shards),
		}
	}

	namespaceState.ShardAssignments = newState
}

func (p *namespaceProcessor) addHandoverStatsToExecutorAssignedState(
	namespaceState *store.NamespaceState,
	executorID string, shardIDs []string,
) map[string]store.ShardHandoverStats {
	var newStats = make(map[string]store.ShardHandoverStats)

	// Prepare handover stats for each shard
	for _, shardID := range shardIDs {
		handoverStats := p.newHandoverStats(namespaceState, shardID, executorID)

		// If there is no handover (first assignment), we skip adding handover stats
		if handoverStats != nil {
			newStats[shardID] = *handoverStats
		}
	}

	return newStats
}

// newHandoverStats creates shard handover statistics if a handover occurred.
func (p *namespaceProcessor) newHandoverStats(
	namespaceState *store.NamespaceState,
	shardID string,
	newExecutorID string,
) *store.ShardHandoverStats {
	logger := p.logger.WithTags(
		tag.ShardNamespace(p.namespaceCfg.Name),
		tag.ShardKey(shardID),
		tag.ShardExecutor(newExecutorID),
	)

	// Fetch previous shard owners from cache
	prevExecutor, err := p.shardStore.GetShardOwner(context.Background(), p.namespaceCfg.Name, shardID)
	if err != nil && !errors.Is(err, store.ErrShardNotFound) {
		logger.Warn("failed to get shard owner for shard statistic", tag.Error(err))
		return nil
	}
	// previous executor is not found in cache
	// meaning this is the first assignment of the shard
	// so we skip updating handover stats
	if prevExecutor == nil {
		return nil
	}

	// No change in assignment
	// meaning no handover occurred
	// skip updating handover stats
	if prevExecutor.ExecutorID == newExecutorID {
		return nil
	}

	// previous executor heartbeat is not found in namespace state
	// meaning the executor has already been cleaned up
	// skip updating handover stats
	prevExecutorHeartbeat, ok := namespaceState.Executors[prevExecutor.ExecutorID]
	if !ok {
		logger.Info("previous executor heartbeat not found, skipping handover stats")
		return nil
	}

	handoverType := types.HandoverTypeEMERGENCY

	// Consider it a graceful handover if the previous executor was in DRAINING or DRAINED status
	// otherwise, it's an emergency handover
	if prevExecutorHeartbeat.Status == types.ExecutorStatusDRAINING || prevExecutorHeartbeat.Status == types.ExecutorStatusDRAINED {
		handoverType = types.HandoverTypeGRACEFUL
	}

	return &store.ShardHandoverStats{
		HandoverType:                      handoverType,
		PreviousExecutorLastHeartbeatTime: prevExecutorHeartbeat.LastHeartbeat,
	}
}

func (*namespaceProcessor) getActiveExecutors(namespaceState *store.NamespaceState, staleExecutors map[string]int64) []string {
	var activeExecutors []string
	for id, state := range namespaceState.Executors {
		// Executor must be ACTIVE and not stale
		if state.Status == types.ExecutorStatusACTIVE {
			if _, ok := staleExecutors[id]; !ok {
				activeExecutors = append(activeExecutors, id)
			}
		}
	}

	sort.Strings(activeExecutors)
	return activeExecutors
}

func assignShardsToEmptyExecutors(currentAssignments map[string][]string) bool {
	emptyExecutors := make([]string, 0)
	executorsWithShards := make([]string, 0)
	minShardsCurrentlyAssigned := 0

	// Ensure the iteration is deterministic.
	executors := make([]string, 0, len(currentAssignments))
	for executorID := range currentAssignments {
		executors = append(executors, executorID)
	}
	slices.Sort(executors)

	for _, executorID := range executors {
		if len(currentAssignments[executorID]) == 0 {
			emptyExecutors = append(emptyExecutors, executorID)
		} else {
			executorsWithShards = append(executorsWithShards, executorID)
			if minShardsCurrentlyAssigned == 0 || len(currentAssignments[executorID]) < minShardsCurrentlyAssigned {
				minShardsCurrentlyAssigned = len(currentAssignments[executorID])
			}
		}
	}

	// If there are no empty executors or no executors with shards, we don't need to do anything.
	if len(emptyExecutors) == 0 || len(executorsWithShards) == 0 {
		return false
	}

	// We calculate the number of shards to assign each of the empty executors. The idea is to assume all current executors have
	// the same number of shards `minShardsCurrentlyAssigned`. We use the minimum so when steeling we don't have to worry about
	// steeling more shards that the executors have.
	// We then calculate the total number of assumed shards `minShardsCurrentlyAssigned * len(executorsWithShards)` and divide it by the
	// number of current executors. This gives us the number of shards per executor, thus the number of shards to assign to each of the
	// empty executors.
	numShardsToAssignEmptyExecutors := minShardsCurrentlyAssigned * len(executorsWithShards) / len(currentAssignments)

	stealRound := 0
	for i := 0; i < numShardsToAssignEmptyExecutors; i++ {
		for _, emptyExecutor := range emptyExecutors {
			executorToSteelFrom := executorsWithShards[stealRound%len(executorsWithShards)]
			stealRound++

			stolenShard := currentAssignments[executorToSteelFrom][0]

			currentAssignments[executorToSteelFrom] = currentAssignments[executorToSteelFrom][1:]
			currentAssignments[emptyExecutor] = append(currentAssignments[emptyExecutor], stolenShard)
		}
	}

	return true
}

func getShards(cfg config.Namespace, namespaceState *store.NamespaceState, deletedShards map[string]store.ShardState) []string {
	if cfg.Type == config.NamespaceTypeFixed {
		return makeShards(cfg.ShardNum)
	} else if cfg.Type == config.NamespaceTypeEphemeral {
		shards := make([]string, 0)
		for _, state := range namespaceState.ShardAssignments {
			for shardID := range state.AssignedShards {
				if _, ok := deletedShards[shardID]; !ok {
					shards = append(shards, shardID)
				}
			}
		}

		return shards
	}
	return nil
}

func makeShards(num int64) []string {
	shards := make([]string, num)
	for i := range num {
		shards[i] = strconv.FormatInt(i, 10)
	}
	return shards
}
