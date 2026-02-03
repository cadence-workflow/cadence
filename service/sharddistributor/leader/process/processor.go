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
)

type processorFactory struct {
	logger        log.Logger
	timeSource    clock.TimeSource
	cfg           config.LeaderProcess
	metricsClient metrics.Client
	sdConfig      *config.Config
}

type namespaceProcessor struct {
	namespaceCfg        config.Namespace
	logger              log.Logger
	metricsClient       metrics.Client
	timeSource          clock.TimeSource
	running             bool
	cancel              context.CancelFunc
	sdConfig            *config.Config
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
	sdConfig *config.Config,
) Factory {
	if cfg.Process.Period == 0 {
		cfg.Process.Period = _defaultPeriod
	}
	if cfg.Process.HeartbeatTTL == 0 {
		cfg.Process.HeartbeatTTL = _defaultHeartbeatTTL
	}
	if cfg.Process.Timeout == 0 {
		cfg.Process.Timeout = _defaultTimeout
	}

	return &processorFactory{
		logger:        logger,
		timeSource:    timeSource,
		cfg:           cfg.Process,
		metricsClient: metricsClient,
		sdConfig:      sdConfig,
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
		sdConfig:      f.sdConfig,
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

	// Perform an initial rebalance on startup.
	err := p.rebalanceShards(ctx)
	if err != nil {
		p.logger.Error("initial rebalance failed", tag.Error(err))
	}

	triggerChan, err := p.runRebalanceTriggeringLoop(ctx)
	if err != nil {
		p.logger.Error("failed to start rebalance triggering loop", tag.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Rebalancing loop cancelled.")
			return

		case trigger := <-triggerChan:
			p.logger.Info("Rebalancing triggered", tag.Dynamic("reason", trigger))
			if err := p.rebalanceShards(ctx); err != nil {
				p.logger.Error("rebalance failed", tag.Error(err))
			}
		}
	}
}

// runRebalanceTriggeringLoop monitors for state changes and periodic triggers to initiate rebalancing.
// it doesn't block Subscribe calls to avoid a growing backlog of updates.
func (p *namespaceProcessor) runRebalanceTriggeringLoop(ctx context.Context) (<-chan string, error) {
	// Buffered channel to allow one pending rebalance trigger.
	triggerChan := make(chan string, 1)

	updateChan, err := p.shardStore.Subscribe(ctx, p.namespaceCfg.Name)
	if err != nil {
		p.logger.Error("Failed to subscribe to state changes, stopping rebalancing loop.", tag.Error(err))
		return nil, err
	}

	go func() {
		defer close(triggerChan)

		ticker := p.timeSource.NewTicker(p.cfg.Period)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				p.logger.Info("Rebalance triggering loop cancelled.")
				return

			case latestRevision, ok := <-updateChan:
				if !ok {
					p.logger.Info("Update channel closed, stopping rebalance triggering loop.")
					return
				}
				if latestRevision <= p.lastAppliedRevision {
					continue
				}

				select {
				case triggerChan <- "State change detected":
				default:
					p.logger.Info("Rebalance already pending, skipping trigger from state change.")
				}

			case <-ticker.Chan():
				select {
				case triggerChan <- "Periodic reconciliation triggered":
				default:
					p.logger.Info("Rebalance already pending, skipping trigger from periodic reconciliation.")
				}
			}
		}
	}()

	return triggerChan, nil
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
			// Only perform shard stats cleanup in GREEDY load balancing mode
			// TODO: refactor this to not have this loop for non-GREEDY modes
			if p.sdConfig.GetLoadBalancingMode(p.namespaceCfg.Name) != types.LoadBalancingModeGREEDY {
				p.logger.Debug("Load balancing mode is not GREEDY, skipping shard stats cleanup.", tag.ShardNamespace(p.namespaceCfg.Name))
				continue
			}

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

	if namespaceState.GlobalRevision <= p.lastAppliedRevision {
		p.logger.Info("No changes detected. Skipping rebalance.")
		return nil
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
		return p.deleteStaleExecutorsAssignedStates(ctx, staleExecutors)
	}
	p.logger.Info("Active executors", tag.ShardExecutors(activeExecutors))

	deletedShards := p.findDeletedShards(namespaceState)
	shardsToReassign, currentAssignments := p.findShardsToReassign(activeExecutors, namespaceState, deletedShards, staleExecutors)

	metricsLoopScope.UpdateGauge(metrics.ShardDistributorAssignLoopNumRebalancedShards, float64(len(shardsToReassign)))

	// If there are deleted shards or stale executors, the distribution has changed.
	assignedToEmptyExecutors := assignShardsToEmptyExecutors(currentAssignments)
	updatedAssignments := p.updateAssignments(shardsToReassign, activeExecutors, currentAssignments)
	isRebalancedByShardLoad := p.rebalanceByShardLoad(calcShardLoad(namespaceState), currentAssignments)

	distributionChanged := len(deletedShards) > 0 || len(staleExecutors) > 0 || assignedToEmptyExecutors || updatedAssignments || isRebalancedByShardLoad
	if !distributionChanged {
		p.logger.Info("No changes to distribution detected. Skipping rebalance.")
		return nil
	}

	newState := p.getNewAssignmentsState(namespaceState, currentAssignments)

	p.emitExecutorMetric(namespaceState, metricsLoopScope)
	p.emitOldestExecutorHeartbeatLag(namespaceState, metricsLoopScope)

	if p.sdConfig.GetMigrationMode(p.namespaceCfg.Name) != types.MigrationModeONBOARDED {
		p.logger.Info("Running rebalancing in shadow mode", tag.Dynamic("old_assignments", namespaceState.ShardAssignments), tag.Dynamic("new_assignments", newState))
		p.emitActiveShardMetric(namespaceState.ShardAssignments, metricsLoopScope)

		if len(staleExecutors) > 0 {
			p.logger.Info("Cleaning up stale executors in shadow mode", tag.ShardExecutors(slices.Collect(maps.Keys(staleExecutors))))
			if err := p.shardStore.DeleteExecutors(ctx, p.namespaceCfg.Name, slices.Collect(maps.Keys(staleExecutors)), p.election.Guard()); err != nil {
				p.logger.Error("Failed to delete stale executors in shadow mode", tag.Error(err))
				// Non-blocking: stale executors in shadow mode will be cleaned up the next cycle
			}
		}
		return nil
	}

	namespaceState.ShardAssignments = newState
	p.logger.Info("Applying new shard distribution.")

	// Use the leader guard for the assign and delete operation.
	err = p.shardStore.AssignShards(ctx, p.namespaceCfg.Name, store.AssignShardsRequest{
		NewState:          namespaceState,
		ExecutorsToDelete: staleExecutors,
	}, p.election.Guard())
	if err != nil {
		return fmt.Errorf("assign shards: %w", err)
	}

	p.emitActiveShardMetric(namespaceState.ShardAssignments, metricsLoopScope)
	return nil
}

func (p *namespaceProcessor) emitActiveShardMetric(shardAssignments map[string]store.AssignedState, metricsLoopScope metrics.Scope) {
	totalActiveShards := 0
	for _, assignedState := range shardAssignments {
		totalActiveShards += len(assignedState.AssignedShards)
	}
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorActiveShards, float64(totalActiveShards))
}

func (p *namespaceProcessor) emitExecutorMetric(namespaceState *store.NamespaceState, metricsLoopScope metrics.Scope) {
	for status, count := range namespaceState.CountExecutorsByStatus() {
		metricsLoopScope.Tagged(metrics.ExecutorStatusTag(status.String())).UpdateGauge(metrics.ShardDistributorTotalExecutors, float64(count))
	}
}

func (p *namespaceProcessor) emitOldestExecutorHeartbeatLag(namespaceState *store.NamespaceState, metricsLoopScope metrics.Scope) {
	if len(namespaceState.Executors) == 0 {
		return
	}

	var oldestHeartbeat time.Time
	for _, executor := range namespaceState.Executors {
		if oldestHeartbeat.IsZero() || executor.LastHeartbeat.Before(oldestHeartbeat) {
			oldestHeartbeat = executor.LastHeartbeat
		}
	}

	lag := p.timeSource.Now().Sub(oldestHeartbeat)
	metricsLoopScope.UpdateGauge(metrics.ShardDistributorOldestExecutorHeartbeatLag, float64(lag.Milliseconds()))
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

// calcShardLoad returns a map of shardID to its load based on the latest reported shard loads from executors
func calcShardLoad(namespaceState *store.NamespaceState) map[string]float64 {
	shardLoad := make(map[string]float64)
	for _, state := range namespaceState.Executors {
		for shardID, report := range state.ReportedShards {
			shardLoad[shardID] = report.ShardLoad
		}
	}
	return shardLoad
}

// rebalanceByShardLoad does a rebalance if a difference between hottest and coldest executors' loads is more than maxDeviation
// in this case the hottest shard will be moved to the coldest executor
func (p *namespaceProcessor) rebalanceByShardLoad(shardLoad map[string]float64, currentAssignments map[string][]string) (distributedChanged bool) {
	// no rebalance if there are no more than 1 executor
	if len(currentAssignments) < 2 {
		return false
	}

	var (
		hottestExecutorLoad = float64(0)
		hottestExecutorID   = ""

		hottestShardID   = ""
		hottestShardLoad = float64(0)

		coldestExecutorLoad = math.MaxFloat64
		coldestExecutorID   = ""
	)

	// finding loads of hottest, coldest executors and hottest shard
	executorLoad := make(map[string]float64)
	for executorID, shardIDs := range currentAssignments {
		for _, shardID := range shardIDs {
			executorLoad[executorID] += shardLoad[shardID]
		}

		if executorLoad[executorID] <= coldestExecutorLoad {
			coldestExecutorLoad = executorLoad[executorID]
			coldestExecutorID = executorID
		}

		if executorLoad[executorID] >= hottestExecutorLoad {
			hottestExecutorLoad = executorLoad[executorID]
			hottestExecutorID = executorID

			var maxShardLoad = float64(0)
			for _, shardID := range shardIDs {
				if shardLoad[shardID] >= maxShardLoad {
					hottestShardID = shardID
					maxShardLoad = shardLoad[shardID]
				}
			}
			hottestShardLoad = maxShardLoad
		}
	}

	// no rebalance if a deviation between coldest and hottest executors less than maxDeviation
	if hottestExecutorLoad/coldestExecutorLoad < p.sdConfig.LoadBalancingNaive.MaxDeviation(p.namespaceCfg.Name) {
		return false
	}

	// no rebalance if coldest executor becomes a hottest
	if coldestExecutorLoad+hottestShardLoad >= hottestExecutorLoad {
		return false
	}

	// remove the hottest Shard from the hottest executor
	// put it to the coldest executor
	for i, shardID := range currentAssignments[hottestExecutorID] {
		if shardID == hottestShardID {
			currentAssignments[hottestExecutorID] = append(currentAssignments[hottestExecutorID][:i], currentAssignments[hottestExecutorID][i+1:]...)
		}
	}
	currentAssignments[coldestExecutorID] = append(currentAssignments[coldestExecutorID], hottestShardID)

	return true
}

func (p *namespaceProcessor) getNewAssignmentsState(namespaceState *store.NamespaceState, currentAssignments map[string][]string) map[string]store.AssignedState {
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

	return newState
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

// deleteStaleExecutorsAssignedStates removes the assigned states of stale executors from the store.
func (p *namespaceProcessor) deleteStaleExecutorsAssignedStates(ctx context.Context, staleExecutors map[string]int64) error {
	if len(staleExecutors) == 0 {
		return nil
	}

	staleExecutorIDs := make([]string, 0, len(staleExecutors))
	for executorID := range staleExecutors {
		staleExecutorIDs = append(staleExecutorIDs, executorID)
	}

	p.logger.Info("Deleting assigned states of stale executors", tag.ShardExecutors(staleExecutorIDs))
	if err := p.shardStore.DeleteAssignedStates(ctx, p.namespaceCfg.Name, staleExecutorIDs, p.election.Guard()); err != nil {
		return fmt.Errorf("delete assigned states: %w", err)
	}

	return nil
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
