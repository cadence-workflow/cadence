package executorclient

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/executorclient/metricsconstants"
	"github.com/uber/cadence/service/sharddistributor/executorclient/syncgeneric"
)

type processorState int32

const (
	processorStateStarting processorState = iota
	processorStateStarted
	processorStateStopping
)

const (
	heartbeatJitterMax = 100 * time.Millisecond
)

type managedProcessor[SP ShardProcessor] struct {
	processor SP
	state     atomic.Int32
}

func (mp *managedProcessor[SP]) setState(state processorState) {
	mp.state.Store(int32(state))
}

func (mp *managedProcessor[SP]) getState() processorState {
	return processorState(mp.state.Load())
}

func newManagedProcessor[SP ShardProcessor](processor SP, state processorState) *managedProcessor[SP] {
	managed := &managedProcessor[SP]{
		processor: processor,
		state:     atomic.Int32{},
	}

	managed.setState(state)
	return managed
}

type executorImpl[SP ShardProcessor] struct {
	logger                 log.Logger
	shardDistributorClient sharddistributorexecutor.Client
	shardProcessorFactory  ShardProcessorFactory[SP]
	namespace              string
	stopC                  chan struct{}
	heartBeatInterval      time.Duration
	managedProcessors      syncgeneric.Map[string, *managedProcessor[SP]]
	executorID             string
	timeSource             clock.TimeSource
	processLoopWG          sync.WaitGroup
	assignmentMutex        sync.Mutex
	metrics                tally.Scope
	migrationMode          types.MigrationMode
}

func (e *executorImpl[SP]) Start(ctx context.Context) {
	e.logger.Info("starting shard distributor executor", tag.ShardNamespace(e.namespace))
	e.processLoopWG.Add(1)
	go func() {
		defer e.processLoopWG.Done()
		e.heartbeatloop(context.WithoutCancel(ctx))
	}()
}

func (e *executorImpl[SP]) Stop() {
	e.logger.Info("stopping shard distributor executor", tag.ShardNamespace(e.namespace))
	close(e.stopC)
	e.processLoopWG.Wait()
}

func (e *executorImpl[SP]) GetShardProcess(shardID string) (SP, error) {
	shardProcess, ok := e.managedProcessors.Load(shardID)
	if !ok {
		var zero SP
		return zero, fmt.Errorf("shard process not found for shard ID: %s", shardID)
	}
	return shardProcess.processor, nil
}

func (e *executorImpl[SP]) AssignShardsFromLocalLogic(ctx context.Context, shardAssignment map[string]*types.ShardAssignment) {
	e.assignmentMutex.Lock()
	defer e.assignmentMutex.Unlock()

	e.logger.Info("Executing external shard assignment")
	e.updateShardAssignment(ctx, shardAssignment)
}

func (e *executorImpl[SP]) heartbeatloop(ctx context.Context) {
	// Check if initial migration mode is LOCAL_PASSTHROUGH - if so, skip heartbeating entirely
	if e.migrationMode == types.MigrationModeLOCALPASSTHROUGH {
		e.logger.Info("initial migration mode is local passthrough, skipping heartbeat loop")
		return
	}

	heartBeatTimer := e.timeSource.NewTimer(getJitteredHeartbeatDuration(e.heartBeatInterval, heartbeatJitterMax))
	defer heartBeatTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("shard distributorexecutor context done, stopping")
			e.stopShardProcessors()
			return
		case <-e.stopC:
			e.logger.Info("shard distributorexecutor stopped")
			e.stopShardProcessors()
			return
		case <-heartBeatTimer.Chan():
			heartBeatTimer.Reset(getJitteredHeartbeatDuration(e.heartBeatInterval, heartbeatJitterMax))
			shardAssignment, migrationMode, err := e.heartbeat(ctx)
			if err != nil {
				e.logger.Error("failed to heartbeat", tag.Error(err))
				continue // TODO: should we stop the executor, and drop all the shards?
			}

			// Handle migration mode logic
			switch migrationMode {
			case types.MigrationModeLOCALPASSTHROUGH:
				// LOCAL_PASSTHROUGH: statically assigned, stop heartbeating
				e.logger.Info("local passthrough mode: stopping heartbeat loop")
				return

			case types.MigrationModeLOCALPASSTHROUGHSHADOW:
				// LOCAL_PASSTHROUGH_SHADOW: check response but don't apply it
				e.compareAssignments(shardAssignment)
				continue

			case types.MigrationModeDISTRIBUTEDPASSTHROUGH:
				// DISTRIBUTED_PASSTHROUGH: validate then apply the assignment
				e.compareAssignments(shardAssignment)
				// Continue with applying the assignment from heartbeat

			case types.MigrationModeONBOARDED:
				// ONBOARDED: normal flow, apply the assignment from heartbeat
				// Continue with normal assignment logic below

			default:
				e.logger.Warn("unknown migration mode, skipping assignment",
					tag.Dynamic("migration-mode", migrationMode))
				continue
			}

			if !e.assignmentMutex.TryLock() {
				e.logger.Warn("already doing shard assignment, will skip this assignment")
				e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentSkipped).Inc(1)
				continue
			}
			go func() {
				defer e.assignmentMutex.Unlock()

				startTime := e.timeSource.Now()
				defer e.metrics.
					Histogram(metricsconstants.ShardDistributorExecutorAssignLoopLatency, metricsconstants.ShardDistributorExecutorAssignLoopLatencyBuckets).
					RecordDuration(e.timeSource.Since(startTime))

				e.updateShardAssignment(ctx, shardAssignment)
			}()
		}
	}
}

func (e *executorImpl[SP]) heartbeat(ctx context.Context) (shardAssignments map[string]*types.ShardAssignment, migrationMode types.MigrationMode, err error) {
	// Fill in the shard status reports
	shardStatusReports := make(map[string]*types.ShardStatusReport)
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if managedProcessor.getState() == processorStateStarted {
			shardStatus := managedProcessor.processor.GetShardReport()

			shardStatusReports[shardID] = &types.ShardStatusReport{
				ShardLoad: shardStatus.ShardLoad,
				Status:    shardStatus.Status,
			}
		}
		return true
	})

	e.metrics.Gauge(metricsconstants.ShardDistributorExecutorOwnedShards).Update(float64(len(shardStatusReports)))

	// Create the request
	request := &types.ExecutorHeartbeatRequest{
		Namespace:          e.namespace,
		ExecutorID:         e.executorID,
		Status:             types.ExecutorStatusACTIVE,
		ShardStatusReports: shardStatusReports,
	}

	// Send the request
	response, err := e.shardDistributorClient.Heartbeat(ctx, request)
	if err != nil {
		return nil, types.MigrationModeINVALID, fmt.Errorf("send heartbeat: %w", err)
	}

	previousMode := e.migrationMode
	currentMode := response.MigrationMode
	if previousMode != currentMode {
		e.logger.Info("migration mode transition",
			tag.Dynamic("previous", previousMode),
			tag.Dynamic("current", currentMode))
		e.migrationMode = currentMode
	}

	return response.ShardAssignments, response.MigrationMode, nil
}

func (e *executorImpl[SP]) updateShardAssignment(ctx context.Context, shardAssignments map[string]*types.ShardAssignment) {
	wg := sync.WaitGroup{}

	// Stop shard processing for shards not assigned to this executor
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if assignment, ok := shardAssignments[shardID]; !ok || assignment.Status != types.AssignmentStatusREADY {
			e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStopped).Inc(1)

			wg.Add(1)
			go func() {
				defer wg.Done()
				managedProcessor.setState(processorStateStopping)
				managedProcessor.processor.Stop()
				e.managedProcessors.Delete(shardID)
			}()
		}
		return true
	})

	// Start shard processing for shards assigned to this executor
	for shardID, assignment := range shardAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			if _, ok := e.managedProcessors.Load(shardID); !ok {
				e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStarted).Inc(1)

				wg.Add(1)
				go func() {
					defer wg.Done()
					processor, err := e.shardProcessorFactory.NewShardProcessor(shardID)
					if err != nil {
						e.logger.Error("failed to create shard processor", tag.Error(err))
						e.metrics.Counter(metricsconstants.ShardDistributorExecutorProcessorCreationFailures).Inc(1)
						return
					}
					managedProcessor := newManagedProcessor(processor, processorStateStarting)
					e.managedProcessors.Store(shardID, managedProcessor)

					processor.Start(ctx)

					managedProcessor.setState(processorStateStarted)
				}()
			}
		}
	}

	wg.Wait()
}

func (e *executorImpl[SP]) stopShardProcessors() {
	wg := sync.WaitGroup{}

	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		// If the processor is already stopping, skip it
		if managedProcessor.getState() == processorStateStopping {
			return true
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			managedProcessor.setState(processorStateStopping)
			managedProcessor.processor.Stop()
			e.managedProcessors.Delete(shardID)
		}()
		return true
	})

	wg.Wait()
}

// compareAssignments compares the local assignments with the heartbeat response assignments
// and emits convergence or divergence metrics
func (e *executorImpl[SP]) compareAssignments(heartbeatAssignments map[string]*types.ShardAssignment) {
	// Get current local assignments
	localAssignments := make(map[string]bool)
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if managedProcessor.getState() == processorStateStarted {
			localAssignments[shardID] = true
		}
		return true
	})

	// Check if all local assignments are in heartbeat assignments with READY status
	for shardID := range localAssignments {
		assignment, exists := heartbeatAssignments[shardID]
		if !exists || assignment.Status != types.AssignmentStatusREADY {
			e.logger.Warn("assignment divergence: local shard not in heartbeat or not ready",
				tag.Dynamic("shard-id", shardID))
			e.emitMetricsConvergence(false)
			return
		}
	}

	// Check if all heartbeat READY assignments are in local assignments
	for shardID, assignment := range heartbeatAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			if !localAssignments[shardID] {
				e.logger.Warn("assignment divergence: heartbeat shard not in local",
					tag.Dynamic("shard-id", shardID))
				e.emitMetricsConvergence(false)
				return
			}
		}
	}

	e.emitMetricsConvergence(true)
}

func (e *executorImpl[SP]) emitMetricsConvergence(converged bool) {
	if converged {
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentConvergence).Inc(1)
	} else {
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentDivergence).Inc(1)
	}
}

func getJitteredHeartbeatDuration(interval time.Duration, jitterMax time.Duration) time.Duration {
	jitterMaxNanos := int64(jitterMax)
	randomJitterNanos := rand.Int63n(jitterMaxNanos)
	jitter := time.Duration(randomJitterNanos)
	return interval - jitter
}
