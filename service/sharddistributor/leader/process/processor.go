package process

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"

	"go.uber.org/fx"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/store"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination=process_mock.go Factory,Processor

// Module provides processor factor for fx app.
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
	CreateProcessor(cfg config.Namespace, shardStore store.ShardStore) Processor
}

type processorFactory struct {
	logger     log.Logger
	timeSource clock.TimeSource
	cfg        config.LeaderProcess
}

type namespaceProcessor struct {
	namespaceCfg        config.Namespace
	logger              log.Logger
	timeSource          clock.TimeSource
	running             bool
	cancel              context.CancelFunc
	cfg                 config.LeaderProcess
	wg                  sync.WaitGroup
	shardStore          store.ShardStore
	lastAppliedRevision int64
}

// NewProcessorFactory creates a new processor factory
func NewProcessorFactory(
	logger log.Logger,
	timeSource clock.TimeSource,
	cfg config.LeaderElection,
) Factory {
	return &processorFactory{
		logger:     logger,
		timeSource: timeSource,
		cfg:        cfg.Process,
	}
}

// CreateProcessor creates a new processor for the given namespace
func (f *processorFactory) CreateProcessor(cfg config.Namespace, shardStore store.ShardStore) Processor {
	return &namespaceProcessor{
		namespaceCfg: cfg,
		logger:       f.logger.WithTags(tag.ComponentLeaderProcessor, tag.ShardNamespace(cfg.Name)),
		timeSource:   f.timeSource,
		cfg:          f.cfg,
		shardStore:   shardStore,
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

// runProcess launches and manages the independent processing loops.
func (p *namespaceProcessor) runProcess(ctx context.Context) {
	defer p.wg.Done()

	var loopWg sync.WaitGroup
	loopWg.Add(2) // We have two loops to manage.

	// Launch the rebalancing process in its own goroutine.
	go func() {
		defer loopWg.Done()
		p.runRebalancingLoop(ctx)
	}()

	// Launch the heartbeat cleanup process in its own goroutine.
	go func() {
		defer loopWg.Done()
		p.runCleanupLoop(ctx)
	}()

	// Wait for both loops to exit.
	loopWg.Wait()
}

// runRebalancingLoop handles shard assignment and redistribution.
func (p *namespaceProcessor) runRebalancingLoop(ctx context.Context) {
	ticker := p.timeSource.NewTicker(p.cfg.Period)
	defer ticker.Stop()

	// Perform an initial rebalance on startup.
	p.rebalanceShards(ctx)

	updateChan, err := p.shardStore.Subscribe(ctx)
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
			p.logger.Info("State change detected, triggering rebalance.")
			p.rebalanceShards(ctx)
		case <-ticker.Chan():
			p.logger.Info("Periodic reconciliation triggered, rebalancing.")
			p.rebalanceShards(ctx)
		}
	}
}

// runCleanupLoop periodically removes stale executors.
func (p *namespaceProcessor) runCleanupLoop(ctx context.Context) {
	ticker := p.timeSource.NewTicker(p.cfg.HeartbeatTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Cleanup loop cancelled.")
			return
		case <-ticker.Chan():
			p.logger.Info("Periodic heartbeat cleanup triggered.")
			p.cleanupStaleExecutors(ctx)
		}
	}
}

// cleanupStaleExecutors removes executors who have not reported a heartbeat recently.
func (p *namespaceProcessor) cleanupStaleExecutors(ctx context.Context) {
	// 1. Get the current heartbeat states. We don't need assignments for this operation.
	heartbeatStates, _, _, err := p.shardStore.GetState(ctx)
	if err != nil {
		p.logger.Error("Failed to get state for heartbeat cleanup", tag.Error(err))
		return
	}

	// 2. Identify expired executors.
	var expiredExecutors []string
	now := p.timeSource.Now().Unix()
	heartbeatTTL := int64(p.cfg.HeartbeatTTL.Seconds())

	for executorID, state := range heartbeatStates {
		if (now - state.LastHeartbeat) > heartbeatTTL {
			expiredExecutors = append(expiredExecutors, executorID)
		}
	}

	if len(expiredExecutors) == 0 {
		return // Nothing to do.
	}

	p.logger.Info("Removing stale executor", tag.ShardExecutors(expiredExecutors))
	// 3. Remove the stale executors from the store.
	for _, executorID := range expiredExecutors {
		if err := p.shardStore.DeleteExecutor(ctx, executorID); err != nil {
			p.logger.Error("Failed to delete stale executor", tag.Error(err), tag.ShardExecutor(executorID))
		}
	}
}

// rebalanceShards is the core logic for distributing shards among active executors.
func (p *namespaceProcessor) rebalanceShards(ctx context.Context) {
	// 1. Get the current state from the store.
	heartbeatStates, assignedStates, readRevision, err := p.shardStore.GetState(ctx)
	if err != nil {
		p.logger.Error("Failed to get latest state from store", tag.Error(err))
		return
	}

	// If the state we just read isn't newer than the one we last applied, stop.
	if readRevision <= p.lastAppliedRevision {
		return
	}

	// 2. Identify active executors.
	var activeExecutors []string
	for id, state := range heartbeatStates {
		if state.State == store.ExecutorStateActive {
			activeExecutors = append(activeExecutors, id)
		}
	}

	if len(activeExecutors) == 0 {
		p.logger.Warn("No active executors found. Cannot assign shards.")
		return
	}

	// ensure activeExecutor order is fixed.
	sort.Strings(activeExecutors)

	// 3. Collect all shards that need to be assigned.
	allShards := make(map[string]struct{})
	for _, shardID := range getShards(p.namespaceCfg) {
		allShards[strconv.FormatInt(shardID, 10)] = struct{}{}
	}

	// 4. Determine current assignments and find shards needing reassignment.
	shardsToReassign := make(map[string]struct{})
	currentAssignments := make(map[string][]string) // executorID -> []shardID

	for _, executorID := range activeExecutors {
		currentAssignments[executorID] = []string{}
	}

	// Check existing assignments.
	for executorID, state := range assignedStates {
		isActive := heartbeatStates[executorID].State == store.ExecutorStateActive
		for shardID := range state.AssignedShards {
			if _, ok := allShards[shardID]; ok {
				delete(allShards, shardID)
				if isActive {
					// Keep track of assignments for active executors.
					currentAssignments[executorID] = append(currentAssignments[executorID], shardID)
				} else {
					// Shard is on a dead/draining executor, needs reassignment.
					shardsToReassign[shardID] = struct{}{}
				}
			}
		}
	}

	// Add any completely unassigned shards to the pool.
	for shardID := range allShards {
		shardsToReassign[shardID] = struct{}{}
	}

	// 5. Rebalance: Distribute the shards needing reassignment.
	// This is a simple round-robin distribution. More complex strategies could be used.
	i := rand.Intn(len(activeExecutors)) // Randomize the starting executor index
	for shardID := range shardsToReassign {
		executorID := activeExecutors[i%len(activeExecutors)]
		currentAssignments[executorID] = append(currentAssignments[executorID], shardID)
		i++
	}

	// 6. Build the new state to be written to the store.
	newState := make(map[string]store.AssignedState)
	for executorID, shards := range currentAssignments {
		assignedShardsMap := make(map[string]store.ShardAssignment)
		for _, shardID := range shards {
			assignedShardsMap[shardID] = store.ShardAssignment{ShardID: shardID}
		}
		newState[executorID] = store.AssignedState{
			ExecutorID:     executorID,
			AssignedShards: assignedShardsMap,
			ReportedShards: assignedStates[executorID].ReportedShards, // Preserve reported state
		}
	}

	// 7. Commit the new state.
	p.logger.Info("Applying new shard distribution.")
	err = p.shardStore.AssignShards(ctx, newState)
	if err != nil {
		p.logger.Error("Failed to apply new shard assignments", tag.Error(err))
		// Do not update the revision, so we can retry on the next trigger.
		return
	}

	p.lastAppliedRevision = readRevision
}

func getShards(cfg config.Namespace) []int64 {
	if cfg.Type == config.NamespaceTypeFixed {
		return makeRange(0, cfg.ShardNum-1)
	}
	return nil
}

func makeRange(min, max int64) []int64 {
	a := make([]int64, max-min+1)
	for i := range a {
		a[i] = min + int64(i+1)
	}
	return a
}
