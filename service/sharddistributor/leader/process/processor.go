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
	_defaultMoveBudgetProportion = 0.02
	// Default hysteresis bands around mean load.
	_defaultHysteresisUpperBand = 1.1
	_defaultHysteresisLowerBand = 0.9
	// Default threshold for triggering severe-imbalance escape hatch.
	_defaultSevereImbalanceRatio = 1.5
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
	if cfg.Process.LoadBalance.PerShardCooldown <= 0 {
		cfg.Process.LoadBalance.PerShardCooldown = _defaultPerShardCooldown
	}
	if cfg.Process.LoadBalance.MoveBudgetProportion <= 0 {
		cfg.Process.LoadBalance.MoveBudgetProportion = _defaultMoveBudgetProportion
	}
	if cfg.Process.LoadBalance.HysteresisUpperBand <= 0 {
		cfg.Process.LoadBalance.HysteresisUpperBand = _defaultHysteresisUpperBand
	}
	if cfg.Process.LoadBalance.HysteresisLowerBand <= 0 {
		cfg.Process.LoadBalance.HysteresisLowerBand = _defaultHysteresisLowerBand
	}
	if cfg.Process.LoadBalance.SevereImbalanceRatio <= 0 {
		cfg.Process.LoadBalance.SevereImbalanceRatio = _defaultSevereImbalanceRatio
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

	return p.executeRebalanceCycle(ctx, metricsLoopScope)
}

func (p *namespaceProcessor) executeRebalanceCycle(ctx context.Context, metricsLoopScope metrics.Scope) (err error) {
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
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorActiveExecutors, float64(len(activeExecutors)))

	deletedShards := p.findDeletedShards(namespaceState)
	shardsNeedingReassignment, currentAssignments := p.findShardsNeedingReassignment(activeExecutors, namespaceState, deletedShards, staleExecutors)

	metricsLoopScope.UpdateGauge(metrics.ShardDistributorAssignLoopNumRebalancedShards, float64(len(shardsNeedingReassignment)))

	// Reassign shards from deleted/stale executors and backfill empty ones
	didAssignToEmpty := assignShardsToEmptyExecutors(currentAssignments)
	didAssignShardsNeedingReassignment := p.assignShardsNeedingReassignment(shardsNeedingReassignment, activeExecutors, currentAssignments)

	// Load-based changes
	didLoadBalance, err := p.loadBalance(currentAssignments, namespaceState, deletedShards, metricsLoopScope)
	if err != nil {
		return fmt.Errorf("load balance: %w", err)
	}

	p.emitAssignmentLoadCV(metricsLoopScope, currentAssignments, namespaceState)
	p.emitShardMovesLastMinute(metricsLoopScope, namespaceState)

	distributionChanged := len(deletedShards) > 0 || len(staleExecutors) > 0 || didAssignToEmpty || didAssignShardsNeedingReassignment || didLoadBalance
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

func (p *namespaceProcessor) findShardsNeedingReassignment(
	activeExecutors []string,
	namespaceState *store.NamespaceState,
	deletedShards map[string]store.ShardState,
	staleExecutors map[string]int64,
) ([]string, map[string][]string) {
	allShards := make(map[string]struct{})
	for _, shardID := range getShards(p.namespaceCfg, namespaceState, deletedShards) {
		allShards[shardID] = struct{}{}
	}

	shardsNeedingReassignment := make([]string, 0)
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
					shardsNeedingReassignment = append(shardsNeedingReassignment, shardID)
				}
			}
		}
	}

	for shardID := range allShards {
		shardsNeedingReassignment = append(shardsNeedingReassignment, shardID)
	}
	return shardsNeedingReassignment, currentAssignments
}

func (*namespaceProcessor) assignShardsNeedingReassignment(shardsNeedingReassignment []string, activeExecutors []string, currentAssignments map[string][]string) bool {
	if len(shardsNeedingReassignment) == 0 {
		return false
	}

	i := rand.Intn(len(activeExecutors))
	for _, shardID := range shardsNeedingReassignment {
		executorID := activeExecutors[i%len(activeExecutors)]
		currentAssignments[executorID] = append(currentAssignments[executorID], shardID)
		i++
	}
	return true
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

func (p *namespaceProcessor) emitAssignmentLoadCV(
	metricsLoopScope metrics.Scope,
	assignments map[string][]string,
	namespaceState *store.NamespaceState,
) {
	if metricsLoopScope == nil {
		return
	}
	cv := computeExecutorLoadCV(assignments, namespaceState)
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorAssignmentLoadCV, cv)
}

func (p *namespaceProcessor) emitShardMovesLastMinute(
	metricsLoopScope metrics.Scope,
	namespaceState *store.NamespaceState,
) {
	if metricsLoopScope == nil || namespaceState == nil {
		return
	}

	if len(namespaceState.ShardStats) == 0 {
		metricsLoopScope.UpdateGauge(metrics.ShardDistributorShardMovesLastMinute, 0)
		return
	}

	window := time.Minute
	now := p.timeSource.Now().UTC()
	recentMoves := 0
	for _, stats := range namespaceState.ShardStats {
		if stats.LastMoveTime.IsZero() {
			continue
		}
		if now.Sub(stats.LastMoveTime) <= window {
			recentMoves++
		}
	}
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorShardMovesLastMinute, float64(recentMoves))
}

func computeExecutorLoadCV(assignments map[string][]string, namespaceState *store.NamespaceState) float64 {
	if len(assignments) == 0 || namespaceState == nil || len(namespaceState.Executors) == 0 {
		return 0
	}

	loads := make([]float64, 0, len(assignments))
	for executorID, shards := range assignments {
		load := 0.0
		heartbeat, ok := namespaceState.Executors[executorID]
		for _, shardID := range shards {
			if !ok || heartbeat.ReportedShards == nil {
				continue
			}
			if shardReport, exists := heartbeat.ReportedShards[shardID]; exists && shardReport != nil {
				load += shardReport.ShardLoad
			}
		}
		loads = append(loads, load)
	}

	return coefficientOfVariation(loads)
}

func coefficientOfVariation(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	total := 0.0
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))
	if mean == 0 {
		return 0
	}

	variance := 0.0
	for _, value := range values {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	if variance == 0 {
		return 0
	}

	return math.Sqrt(variance) / mean
}
