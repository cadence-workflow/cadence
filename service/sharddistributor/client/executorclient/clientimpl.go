package executorclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-go/tally"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/client/clientcommon"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient/metricsconstants"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient/syncgeneric"
)

var (
	// ErrLocalPassthroughMode indicates that the heartbeat loop should stop due to local passthrough mode
	ErrLocalPassthroughMode = errors.New("local passthrough mode: stopping heartbeat loop")
	// ErrAssignmentDivergenceLocalShard indicates that the local shard is not reported back from the heartbeat
	ErrAssignmentDivergenceLocalShard = errors.New("assignment divergence: local shard not in heartbeat or not ready")
	// ErrAssignmentDivergenceHeartbeatShard indicates that the shard in the heartbeat is not present in the local assignment
	ErrAssignmentDivergenceHeartbeatShard = errors.New("assignment divergence: heartbeat shard not in local")
)

type processorState int32

const (
	processorStateStarting processorState = iota
	processorStateStarted
	processorStateStopping
)

const (
	heartbeatJitterCoeff           = 0.1 // 10% jitter
	drainingHeartbeatTimeout       = 5 * time.Second
	processorAsyncOperationTimeout = 10 * time.Second //  maximum time allowed for a shard processor Start or Stop call.
)

type managedProcessor[SP ShardProcessor] struct {
	processor SP
	state     atomic.Int32
}

type syncExecutorMetadata struct {
	sync.RWMutex

	data map[string]string
}

func (m *syncExecutorMetadata) Set(metadata map[string]string) {
	m.Lock()
	defer m.Unlock()

	m.data = metadata
}

func (m *syncExecutorMetadata) Get() map[string]string {
	m.RLock()
	defer m.RUnlock()

	// Copy the map
	result := make(map[string]string, len(m.data))
	for k, v := range m.data {
		result[k] = v
	}

	return result
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
	ttlShard               time.Duration
	managedProcessors      syncgeneric.Map[string, *managedProcessor[SP]]
	processorsToLastUse    syncgeneric.Map[string, time.Time]
	executorID             string
	timeSource             clock.TimeSource
	processLoopWG          sync.WaitGroup
	assignmentMutex        sync.Mutex
	metrics                tally.Scope
	hostMetrics            tally.Scope
	migrationMode          atomic.Int32
	metadata               syncExecutorMetadata
	drainObserver          clientcommon.DrainSignalObserver
}

func (e *executorImpl[SP]) setMigrationMode(mode types.MigrationMode) {
	e.migrationMode.Store(int32(mode))
}

func (e *executorImpl[SP]) getMigrationMode() types.MigrationMode {
	return types.MigrationMode(e.migrationMode.Load())
}

func (e *executorImpl[SP]) Start(ctx context.Context) {
	e.logger.Info("starting shard distributor executor", tag.ShardNamespace(e.namespace))
	e.processLoopWG.Add(2)
	go func() {
		defer e.processLoopWG.Done()
		e.heartbeatloop(context.WithoutCancel(ctx))
	}()
	go func() {
		defer e.processLoopWG.Done()
		e.shardCleanUpLoop(context.WithoutCancel(ctx))
	}()
}

func (e *executorImpl[SP]) Stop() {
	e.logger.Info("stopping shard distributor executor", tag.ShardNamespace(e.namespace))
	close(e.stopC)
	e.processLoopWG.Wait()
}

func (e *executorImpl[SP]) GetShardProcess(ctx context.Context, shardID string) (SP, error) {
	e.processorsToLastUse.Store(shardID, e.timeSource.Now())

	shardProcess, ok := e.managedProcessors.Load(shardID)
	if !ok {
		if e.getMigrationMode() == types.MigrationModeLOCALPASSTHROUGH {
			// Fail immediately if we are in LOCAL_PASSTHROUGH mode
			var zero SP
			return zero, fmt.Errorf("%w for shard ID: %s", ErrShardProcessNotFound, shardID)
		}

		// Do a heartbeat and check again
		err := e.heartbeatAndUpdateAssignment(ctx)
		if err != nil {
			var zero SP
			return zero, fmt.Errorf("heartbeat and assign shards: %w", err)
		}

		// Check again if the shard process is found
		shardProcess, ok = e.managedProcessors.Load(shardID)
		if !ok {
			var zero SP
			return zero, fmt.Errorf("%w for shard ID: %s", ErrShardProcessNotFound, shardID)
		}
	}

	return shardProcess.processor, nil
}

func (e *executorImpl[SP]) IsOnboardedToSD() bool {
	return e.getMigrationMode() == types.MigrationModeONBOARDED
}

func (e *executorImpl[SP]) AssignShardsFromLocalLogic(ctx context.Context, shardAssignment map[string]*types.ShardAssignment) error {
	e.assignmentMutex.Lock()
	defer e.assignmentMutex.Unlock()
	if e.getMigrationMode() == types.MigrationModeONBOARDED {
		return fmt.Errorf("migration mode is onborded, no local assignemnt allowed")
	}
	e.logger.Info("Executing external shard assignment")
	e.addNewShards(ctx, shardAssignment)
	return nil
}

func (e *executorImpl[SP]) RemoveShardsFromLocalLogic(shardIDs []string) error {
	if e.getMigrationMode() == types.MigrationModeONBOARDED {
		return fmt.Errorf("migration mode is onborded, no local assignemnt allowed")
	}

	return e.removeShards(shardIDs)
}

func (e *executorImpl[SP]) removeShards(shardIDs []string) error {
	e.assignmentMutex.Lock()
	defer e.assignmentMutex.Unlock()
	e.logger.Info("Executing external shard deletion assignment")
	e.deleteShards(shardIDs)
	return nil
}

// drainChannel returns the drain signal channel, or nil if no observer is configured.
func (e *executorImpl[SP]) drainChannel() <-chan struct{} {
	if e.drainObserver != nil {
		return e.drainObserver.Drain()
	}
	return nil
}

func (e *executorImpl[SP]) heartbeatloop(ctx context.Context) {
	// Check if initial migration mode is LOCAL_PASSTHROUGH - if so, skip heartbeating entirely
	if e.getMigrationMode() == types.MigrationModeLOCALPASSTHROUGH {
		e.logger.Info("initial migration mode is local passthrough, skipping heartbeat loop")
		return
	}

	heartBeatTimer := e.timeSource.NewTimer(backoff.JitDuration(e.heartBeatInterval, heartbeatJitterCoeff))
	defer heartBeatTimer.Stop()

	drainCh := e.drainChannel()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("shard distributor executor context done, stopping")
			e.stopShardProcessors()
			e.sendDrainingHeartbeat()
			return
		case <-e.stopC:
			e.logger.Info("shard distributor executor stopped")
			e.stopShardProcessors()
			e.sendDrainingHeartbeat()
			return
		case <-drainCh:
			e.logger.Info("drain signal received, stopping shard processors")
			e.stopShardProcessors()
			e.sendDrainingHeartbeat()

			if !e.waitForUndrain(ctx) {
				return
			}

			e.logger.Info("undrain signal received, resuming heartbeat")
			drainCh = e.drainObserver.Drain()
			heartBeatTimer.Reset(backoff.JitDuration(e.heartBeatInterval, heartbeatJitterCoeff))
		case <-heartBeatTimer.Chan():
			heartBeatTimer.Reset(backoff.JitDuration(e.heartBeatInterval, heartbeatJitterCoeff))
			err := e.heartbeatAndUpdateAssignment(ctx)
			if errors.Is(err, ErrLocalPassthroughMode) {
				e.logger.Info("local passthrough mode: stopping heartbeat loop")
				return
			}
			if err != nil {
				e.logger.Error("failed to heartbeat and assign shards", tag.Error(err))
				continue
			}
		}
	}
}

// waitForUndrain blocks until the undrain signal fires or the executor is stopped.
// Returns true if undrained (caller should resume), false if stopped.
func (e *executorImpl[SP]) waitForUndrain(ctx context.Context) bool {
	if e.drainObserver == nil {
		return false
	}

	undrainCh := e.drainObserver.Undrain()

	select {
	case <-ctx.Done():
		return false
	case <-e.stopC:
		return false
	case <-undrainCh:
		return true
	}
}

func (e *executorImpl[SP]) heartbeatAndUpdateAssignment(ctx context.Context) error {
	if !e.assignmentMutex.TryLock() {
		e.logger.Error("still doing assignment, skipping heartbeat")
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorHeartbeatSkipped).Inc(1)
		return nil
	}
	defer e.assignmentMutex.Unlock()
	shardAssignment, err := e.heartbeatAndHandleMigrationMode(ctx)
	if err != nil {
		return err
	}
	if shardAssignment != nil {
		e.updateShardAssignmentMetered(ctx, shardAssignment)
	}
	return nil
}

func (e *executorImpl[SP]) heartbeatAndHandleMigrationMode(ctx context.Context) (shardAssignment map[string]*types.ShardAssignment, err error) {
	shardAssignment, migrationMode, err := e.heartbeat(ctx)
	if err != nil {
		// TODO: should we stop the executor, and drop all the shards?
		return nil, fmt.Errorf("failed to heartbeat: %w", err)
	}

	// Handle migration mode logic
	switch migrationMode {
	case types.MigrationModeLOCALPASSTHROUGH:
		// LOCAL_PASSTHROUGH: statically assigned, stop heartbeating
		return nil, ErrLocalPassthroughMode

	case types.MigrationModeLOCALPASSTHROUGHSHADOW:
		// LOCAL_PASSTHROUGH_SHADOW: check response but don't apply it
		err = e.compareAssignments(shardAssignment)
		return nil, err

	case types.MigrationModeDISTRIBUTEDPASSTHROUGH:
		// DISTRIBUTED_PASSTHROUGH: validate then apply the assignment
		err = e.compareAssignments(shardAssignment)
		if err != nil {
			return nil, err
		}
		return shardAssignment, nil
		// Continue with applying the assignment from heartbeat

	case types.MigrationModeONBOARDED:
		// ONBOARDED: normal flow, apply the assignment from heartbeat
		return shardAssignment, nil
		// Continue with normal assignment logic below

	default:
		e.logger.Warn("unknown migration mode, skipping assignment",
			tag.ShardNamespace(e.namespace), tag.Dynamic("migration-mode", migrationMode))
		return nil, nil
	}
}

func (e *executorImpl[SP]) updateShardAssignmentMetered(ctx context.Context, shardAssignment map[string]*types.ShardAssignment) {
	startTime := e.timeSource.Now()
	defer e.metrics.
		Histogram(metricsconstants.ShardDistributorExecutorAssignLoopLatency, metricsconstants.ShardDistributorExecutorAssignLoopLatencyBuckets).
		RecordDuration(e.timeSource.Since(startTime))

	e.updateShardAssignment(ctx, shardAssignment)
}

func (e *executorImpl[SP]) heartbeat(ctx context.Context) (shardAssignments map[string]*types.ShardAssignment, migrationMode types.MigrationMode, err error) {
	return e.sendHeartbeat(ctx, types.ExecutorStatusACTIVE)
}

func (e *executorImpl[SP]) sendHeartbeat(ctx context.Context, status types.ExecutorStatus) (map[string]*types.ShardAssignment, types.MigrationMode, error) {
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

	e.hostMetrics.Gauge(metricsconstants.ShardDistributorExecutorOwnedShards).Update(float64(len(shardStatusReports)))

	// Create the request
	request := &types.ExecutorHeartbeatRequest{
		Namespace:          e.namespace,
		ExecutorID:         e.executorID,
		Status:             status,
		ShardStatusReports: shardStatusReports,
		Metadata:           e.metadata.Get(),
	}

	// Send the request
	response, err := e.shardDistributorClient.Heartbeat(ctx, request)
	if err != nil {
		return nil, types.MigrationModeINVALID, fmt.Errorf("send heartbeat: %w", err)
	}

	previousMode := e.getMigrationMode()
	currentMode := response.MigrationMode
	if previousMode != currentMode {
		e.logger.Info("migration mode transition",
			tag.Dynamic("previous", previousMode),
			tag.Dynamic("current", currentMode),
			tag.ShardNamespace(e.namespace),
			tag.ShardExecutor(e.executorID))
		e.setMigrationMode(currentMode)
	}

	return response.ShardAssignments, response.MigrationMode, nil
}

func (e *executorImpl[SP]) sendDrainingHeartbeat() {
	ctx, cancel := context.WithTimeout(context.Background(), drainingHeartbeatTimeout)
	defer cancel()

	_, _, err := e.sendHeartbeat(ctx, types.ExecutorStatusDRAINING)
	if err != nil {
		e.logger.Error("failed to send draining heartbeat", tag.Error(err))
	}
}

func (e *executorImpl[SP]) updateShardAssignment(ctx context.Context, shardAssignments map[string]*types.ShardAssignment) {
	// Stop shards no longer assigned. Each call fires 2 goroutines: one for the
	// Stop() call and one per-shard timeout watcher.
	e.managedProcessors.Range(func(shardID string, managedProcessor *managedProcessor[SP]) bool {
		if assignment, ok := shardAssignments[shardID]; !ok || assignment.Status != types.AssignmentStatusREADY {
			e.stopManagerProcessor(shardID)
		}
		return true
	})

	// Start newly assigned shards. Each call fires 2 goroutines: one for the
	// Start() call and one per-shard timeout watcher.
	for shardID, assignment := range shardAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			e.addManagerProcessor(ctx, shardID)
		}
	}
}

func (e *executorImpl[SP]) addNewShards(ctx context.Context, shardAssignments map[string]*types.ShardAssignment) {
	for shardID, assignment := range shardAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			e.addManagerProcessor(ctx, shardID)
		}
	}
}

func (e *executorImpl[SP]) deleteShards(shardIDs []string) {
	for _, shardID := range shardIDs {
		e.stopManagerProcessor(shardID)
	}
}

func (e *executorImpl[SP]) stopShardProcessors() {
	// Synchronous: collect each per-shard done channel and block until all
	// Stop() calls finish.
	var doneChans []<-chan struct{}
	e.managedProcessors.Range(func(shardID string, _ *managedProcessor[SP]) bool {
		if ch := e.stopManagerProcessor(shardID); ch != nil {
			doneChans = append(doneChans, ch)
		}
		return true
	})
	for _, ch := range doneChans {
		<-ch
	}
}

// addManagerProcessor creates a new shard processor, stores it in the map
// synchronously (state: starting), then fires 2 goroutines: one to call Start()
// and one per-shard timeout watcher. No-ops if the shard is already tracked.
func (e *executorImpl[SP]) addManagerProcessor(ctx context.Context, shardID string) {
	if existing, ok := e.managedProcessors.Load(shardID); ok {
		// The processor already exists. If it is stopping (Stop goroutine still
		// running) we log and skip — the next heartbeat will re-send the shard as
		// READY once the entry is gone and addManagerProcessor will create a fresh one.
		if existing.getState() == processorStateStopping {
			e.logger.Info("shard processor add skipped: existing processor is still stopping, will retry on next heartbeat",
				tag.ShardKey(shardID))
		}
		return
	}

	e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStarted).Inc(1)
	processor, err := e.shardProcessorFactory.NewShardProcessor(shardID)
	if err != nil {
		e.logger.Error("failed to create shard processor", tag.Error(err))
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorProcessorCreationFailures).Inc(1)
		return
	}
	managedProcessor := newManagedProcessor(processor, processorStateStarting)
	e.managedProcessors.Store(shardID, managedProcessor)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := processor.Start(context.WithoutCancel(ctx)); err != nil {
			e.logger.Error("shard processor start failed", tag.ShardKey(shardID), tag.Error(err))
			// Remove the failed processor so the next heartbeat can retry.
			e.managedProcessors.Delete(shardID)
			return
		}
		managedProcessor.setState(processorStateStarted)
	}()
	go func() {
		timer := e.timeSource.NewTimer(processorAsyncOperationTimeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.Chan():
			e.logger.Error("shard processor start timed out", tag.ShardKey(shardID))
			e.metrics.Counter(metricsconstants.ShardDistributorExecutorProcessorStartTimeout).Inc(1)
		}
	}()
}

// stopManagerProcessor transitions the processor to stopping, removes it from the
// map synchronously, then fires 2 goroutines: one to call Stop() and one per-shard
// timeout watcher. Returns a done channel that closes when Stop() returns so callers
// that need to block (e.g. stopShardProcessors) can wait on it.
// No-ops if the processor is not found or already stopping; returns nil in that case.
func (e *executorImpl[SP]) stopManagerProcessor(shardID string) <-chan struct{} {
	managedProcessor, ok := e.managedProcessors.Load(shardID)
	if !ok || managedProcessor.getState() == processorStateStopping {
		return nil
	}
	e.metrics.Counter(metricsconstants.ShardDistributorExecutorShardsStopped).Inc(1)
	managedProcessor.setState(processorStateStopping)
	// Delete synchronously so the next heartbeat sees a clean state immediately
	// and can re-add the shard if the server re-assigns it.
	e.managedProcessors.Delete(shardID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		managedProcessor.processor.Stop()
	}()
	go func() {
		timer := e.timeSource.NewTimer(processorAsyncOperationTimeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.Chan():
			e.logger.Error("shard processor stop timed out", tag.ShardKey(shardID))
			e.metrics.Counter(metricsconstants.ShardDistributorExecutorProcessorStopTimeout).Inc(1)
		}
	}()
	return done
}

func (e *executorImpl[SP]) shardCleanUpLoop(ctx context.Context) {
	// We don't run the loop for invalid durations
	if e.ttlShard <= 0 {

		return
	}
	shardCleanUpTimer := e.timeSource.NewTimer(backoff.JitDuration(e.ttlShard, heartbeatJitterCoeff))
	defer shardCleanUpTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopC:
			return
		case <-shardCleanUpTimer.Chan():
			e.processorsToLastUse.Range(func(shardID string, time time.Time) bool {
				if time.Add(e.ttlShard).Before(e.timeSource.Now()) {
					if e.getMigrationMode() == types.MigrationModeONBOARDED {
						mp, ok := e.managedProcessors.Load(shardID)
						if ok {
							mp.processor.SetShardStatus(types.ShardStatusDONE)
						}
					} else {
						e.deleteShards([]string{shardID})
					}
					e.processorsToLastUse.Delete(shardID)
				}
				return true
			})
		}
	}
}

// compareAssignments compares the local assignments with the heartbeat response assignments
// return error if the assignment are not the same and emits convergence or divergence metrics
func (e *executorImpl[SP]) compareAssignments(heartbeatAssignments map[string]*types.ShardAssignment) error {
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
				tag.ShardKey(shardID))
			e.emitMetricsConvergence(false)
			return ErrAssignmentDivergenceLocalShard
		}
	}

	// Check if all heartbeat READY assignments are in local assignments
	for shardID, assignment := range heartbeatAssignments {
		if assignment.Status == types.AssignmentStatusREADY {
			if !localAssignments[shardID] {
				e.logger.Warn("assignment divergence: heartbeat shard not in local",
					tag.ShardKey(shardID))
				e.emitMetricsConvergence(false)
				return ErrAssignmentDivergenceHeartbeatShard
			}
		}
	}

	e.emitMetricsConvergence(true)
	return nil
}

func (e *executorImpl[SP]) emitMetricsConvergence(converged bool) {
	if converged {
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentConvergence).Inc(1)
	} else {
		e.metrics.Counter(metricsconstants.ShardDistributorExecutorAssignmentDivergence).Inc(1)
	}
}

func (e *executorImpl[SP]) GetNamespace() string {
	return e.namespace
}

func (e *executorImpl[SP]) SetMetadata(metadata map[string]string) {
	e.metadata.Set(metadata)
}

func (e *executorImpl[SP]) GetMetadata() map[string]string {
	return e.metadata.Get()
}
