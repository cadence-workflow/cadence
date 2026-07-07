// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package taskdlq

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/multierr"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
)

type (
	// Processor reads tasks from the history task DLQ and executes them synchronously.
	//
	// Start/Stop manage a background goroutine that periodically calls ProcessShard for
	// the shard this processor was created for. ProcessPartition is the on-demand failover
	// path and can be called at any time regardless of daemon state.
	Processor interface {
		common.Daemon

		// ProcessShard sweeps all DLQ partitions for a shard (periodic path).
		// Errors in individual partitions are logged and skipped; the combined
		// error is returned after all partitions have been attempted.
		ProcessShard(ctx context.Context) error

		// ProcessPartition processes all task types for a specific
		// (domain, clusterAttributeScope, clusterAttributeName) partition
		// within a shard.
		// Returns errors for all partitions that could not be processed.
		ProcessPartition(ctx context.Context, domainID, clusterAttributeScope, clusterAttributeName string) error

		// FailoverPartitions schedules on-demand re-injection of the given DLQ partitions.
		// It is non-blocking and safe to call from the domain failover callback (which must not
		// block on DB work): the partitions are queued and processed by the background loop,
		// which preempts any in-progress periodic ProcessShard sweep so failed-over partitions
		// are reprocessed first.
		FailoverPartitions(partitions []Partition)
	}

	// Partition identifies a single DLQ partition to reprocess on demand.
	// An empty ClusterAttributeScope/ClusterAttributeName targets the domain's default
	// partition (and, for the underlying by-domain query, every partition of the domain).
	Partition struct {
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
	}

	ProcessorImpl struct {
		shardID       int
		mgr           persistence.HistoryTaskDLQManager
		reinjector    TaskReinjector
		pageSize      int
		interval      dynamicproperties.DurationPropertyFnWithShardIDFilter
		domainMode    dynamicproperties.StringPropertyFnWithDomainFilter
		enabled       dynamicproperties.BoolPropertyFn
		timeSource    clock.TimeSource
		metricsClient metrics.Client
		logger        log.Logger

		status    int32
		ctx       context.Context
		cancel    context.CancelFunc
		wg        sync.WaitGroup
		processMu sync.Mutex // serializes ProcessShard and ProcessPartition

		// failoverMu guards the failover coordination state below.
		failoverMu sync.Mutex
		// pendingFailover holds partitions awaiting on-demand reprocessing, keyed by
		// Partition.String() so a burst of failovers for the same partition collapses to one.
		pendingFailover map[string]Partition
		// sweepCancel cancels the in-flight periodic sweep so a failover can preempt it; nil
		// when no sweep is running.
		sweepCancel context.CancelFunc
		// failoverSignal wakes an idle loop when pendingFailover becomes non-empty. Buffered
		// size 1 and sent non-blocking, so it coalesces: one wake-up drains everything pending.
		failoverSignal chan struct{}
	}

	// ProcessorParams are the dependencies needed to build a Processor.
	ProcessorParams struct {
		ShardID       int
		Manager       persistence.HistoryTaskDLQManager
		Reinjector    TaskReinjector
		PageSize      int
		Interval      dynamicproperties.DurationPropertyFnWithShardIDFilter
		DomainMode    dynamicproperties.StringPropertyFnWithDomainFilter
		Enabled       dynamicproperties.BoolPropertyFn
		TimeSource    clock.TimeSource
		MetricsClient metrics.Client
		Logger        log.Logger
	}
)

var _ Processor = (*ProcessorImpl)(nil)

// NewProcessor creates a Processor from the given dependencies.
//
// The processor will periodically process the DLQ for the entire shard,
// and will process a domain/clusterAttribute pair on demand.
func NewProcessor(params ProcessorParams) *ProcessorImpl {
	return &ProcessorImpl{
		shardID:         params.ShardID,
		mgr:             params.Manager,
		reinjector:      params.Reinjector,
		pageSize:        params.PageSize,
		interval:        params.Interval,
		domainMode:      params.DomainMode,
		enabled:         params.Enabled,
		timeSource:      params.TimeSource,
		metricsClient:   params.MetricsClient,
		logger:          params.Logger,
		status:          common.DaemonStatusInitialized,
		cancel:          func() {}, // no-op until Start() sets the real cancel
		pendingFailover: make(map[string]Partition),
		failoverSignal:  make(chan struct{}, 1),
	}
}

// NewProcessorFromShard is a convenience constructor that derives the shard-scoped
// dependencies (shard ID, reinjector, DLQ manager, time source, metrics, logger)
// from the shard context and delegates to NewProcessor.
func NewProcessorFromShard(
	shard shard.Context,
	// TODO(c-warren): Convert pageSize to a dynamic property.
	pageSize int,
	interval dynamicproperties.DurationPropertyFnWithShardIDFilter,
	domainMode dynamicproperties.StringPropertyFnWithDomainFilter,
	enabled dynamicproperties.BoolPropertyFn,
) *ProcessorImpl {
	return NewProcessor(ProcessorParams{
		ShardID:       shard.GetShardID(),
		Manager:       shard.GetService().GetHistoryTaskDLQManager(),
		Reinjector:    shard,
		PageSize:      pageSize,
		Interval:      interval,
		DomainMode:    domainMode,
		Enabled:       enabled,
		TimeSource:    shard.GetTimeSource(),
		MetricsClient: shard.GetMetricsClient(),
		Logger:        shard.GetLogger(),
	})
}

// Start starts the processor and launches the background processing loop.
func (p *ProcessorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.logger.Debug("DLQ processor starting", tag.ShardID(p.shardID))
	p.wg.Add(1)
	go p.processLoop()
	p.logger.Debug("DLQ processor started", tag.ShardID(p.shardID))
}

// Stop signals the background loop to exit and waits for it to finish. Idempotent.
func (p *ProcessorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	p.logger.Debug("DLQ processor stopping", tag.ShardID(p.shardID))
	p.cancel()
	p.wg.Wait()
	p.logger.Debug("DLQ processor stopped", tag.ShardID(p.shardID))
}

// processLoop is the background goroutine that periodically calls ProcessShard and drains
// on-demand failovers. It reads the interval on every tick so dynamic-config changes take
// effect without a restart.
//
// The periodic sweep runs synchronously (runSweep) under a cancelable context published to
// p.sweepCancel, so an on-demand failover can preempt it via FailoverPartitions -> cancel().
// After a sweep returns (whether it completed or was preempted) the loop drains any pending
// failovers, guaranteeing failed-over partitions are reprocessed before the next sweep.
// ProcessShard is resumable (it reads ack levels fresh on each run), so a preempted sweep
// simply picks up where it left off on the next tick.
func (p *ProcessorImpl) processLoop() {
	defer p.wg.Done()
	defer func() { log.CapturePanic(recover(), p.logger, nil) }()

	timer := p.timeSource.NewTimer(p.interval(p.shardID))
	defer timer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.failoverSignal:
			// A failover arrived while the loop was idle; drain it.
			p.processPendingFailovers()
		case <-timer.Chan():
			p.runSweep()
			// A failover may have preempted the sweep; drain it before the next tick.
			p.processPendingFailovers()
			timer.Reset(p.interval(p.shardID))
		}
	}
}

// runSweep runs one periodic ProcessShard synchronously under a cancelable context, publishing
// its cancel func to p.sweepCancel so FailoverPartitions can preempt it. A canceled context is
// expected (a failover preempted the sweep) and is not logged as an error; ProcessShard resumes
// from its persisted ack levels on the next tick.
func (p *ProcessorImpl) runSweep() {
	if !p.enabled() {
		return
	}
	ctx, cancel := context.WithCancel(p.ctx)
	p.failoverMu.Lock()
	p.sweepCancel = cancel
	p.failoverMu.Unlock()
	defer func() {
		p.failoverMu.Lock()
		p.sweepCancel = nil
		p.failoverMu.Unlock()
		cancel()
	}()

	if err := p.ProcessShard(ctx); err != nil && ctx.Err() == nil {
		p.logger.Error("DLQ periodic shard sweep failed",
			tag.ShardID(p.shardID),
			tag.Error(err),
		)
	}
}

// processPendingFailovers drains the pending failover set and reprocesses each partition.
// It runs on the loop goroutine after any in-progress sweep has returned, so ProcessPartition
// can acquire processMu immediately. Partitions that arrive while this runs re-signal the loop and
// are handled on the next iteration (the signal is coalesced, so nothing is lost).
func (p *ProcessorImpl) processPendingFailovers() {
	p.failoverMu.Lock()
	parts := make([]Partition, 0, len(p.pendingFailover))
	for _, part := range p.pendingFailover {
		parts = append(parts, part)
	}
	p.pendingFailover = make(map[string]Partition)
	p.failoverMu.Unlock()

	if !p.enabled() {
		return
	}
	for _, part := range parts {
		if p.ctx.Err() != nil {
			return
		}
		if err := p.ProcessPartition(p.ctx, part.DomainID, part.ClusterAttributeScope, part.ClusterAttributeName); err != nil {
			p.logger.Error("DLQ failover partition reprocessing failed",
				tag.ShardID(p.shardID),
				tag.WorkflowDomainID(part.DomainID),
				tag.Dynamic("cluster-attribute-scope", part.ClusterAttributeScope),
				tag.Dynamic("cluster-attribute-name", part.ClusterAttributeName),
				tag.Error(err),
			)
		}
	}
}

func (p *ProcessorImpl) ProcessShard(ctx context.Context) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ackLevels, err := p.mgr.GetHistoryDLQAckLevels(ctx, persistence.HistoryDLQGetAckLevelsRequest{
		ShardID: p.shardID,
	})
	if err != nil {
		return fmt.Errorf("get DLQ ack levels for shard %d: %w", p.shardID, err)
	}
	return p.processAckLevels(ctx, ackLevels)
}

func (p *ProcessorImpl) ProcessPartition(ctx context.Context, domainID, clusterAttributeScope, clusterAttributeName string) error {
	// Fast-fail for direct callers; processAckLevel also guards each partition individually.
	if p.domainMode(domainID) != constants.HistoryTaskDLQModeEnabled {
		p.logger.Debug("DLQ not enabled for domain, skipping partition processing", tag.ShardID(p.shardID), tag.WorkflowDomainID(domainID))
		return nil
	}

	p.processMu.Lock()
	defer p.processMu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ackLevels, err := p.mgr.GetHistoryDLQAckLevels(ctx, persistence.HistoryDLQGetAckLevelsRequest{
		ShardID:               p.shardID,
		DomainID:              domainID,
		ClusterAttributeScope: clusterAttributeScope,
		ClusterAttributeName:  clusterAttributeName,
	})
	if err != nil {
		return fmt.Errorf("get DLQ ack levels for partition (shard=%d domain=%s scope=%s name=%s): %w",
			p.shardID, domainID, clusterAttributeScope, clusterAttributeName, err)
	}
	return p.processAckLevels(ctx, ackLevels)
}

// FailoverPartitions enqueues the given partitions for on-demand reprocessing by the background
// loop and preempts any in-progress periodic sweep so they run first. It never blocks the caller and
// never drops: partitions are deduplicated into a pending set keyed by Partition.String() (bounded by
// domains×attributes per failover), the in-flight sweep's context is canceled, and the wake-up signal
// is coalesced so one signal drains everything pending.
func (p *ProcessorImpl) FailoverPartitions(partitions []Partition) {
	if len(partitions) == 0 {
		return
	}

	p.failoverMu.Lock()
	for _, part := range partitions {
		p.pendingFailover[part.String()] = part
	}
	// Preempt the in-flight sweep (if any) so the loop drains these partitions promptly. cancel is
	// idempotent, so calling it after the sweep already finished is harmless.
	cancel := p.sweepCancel
	p.failoverMu.Unlock()

	if cancel != nil {
		cancel()
	}

	// Coalescing, non-blocking signal: wakes an idle loop. If one is already pending the loop will
	// pick up the newly-added partitions when it drains, so a full buffer is fine to ignore.
	select {
	case p.failoverSignal <- struct{}{}:
	default:
	}
}

// All ack levels are processed regardless of individual failures.
// Returns an error when any of the ack levels cannot be processed
func (p *ProcessorImpl) processAckLevels(ctx context.Context, ackLevels []persistence.HistoryDLQAckLevel) error {
	var errs error
	for _, al := range ackLevels {
		// Preempt promptly when the sweep's context is canceled (e.g. a failover wants to run
		// its partitions first). ProcessShard is resumable, so stopping between partitions is safe.
		if err := ctx.Err(); err != nil {
			return multierr.Append(errs, err)
		}
		if err := p.processAckLevel(ctx, al); err != nil {
			// A canceled context means the sweep was preempted mid-partition, not a real
			// failure — return quietly so the failover partitions can run.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return multierr.Append(errs, ctxErr)
			}
			p.logger.Error("failed to process DLQ partition",
				tag.WorkflowDomainID(al.DomainID),
				tag.Dynamic("cluster-attribute-scope", al.ClusterAttributeScope),
				tag.Dynamic("cluster-attribute-name", al.ClusterAttributeName),
				tag.TaskType(al.TaskCategory.ID()),
				tag.Error(err),
			)
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}

// processAckLevel fetches and re-injects tasks for the given ack level.
// It reads all tasks from the current ack position to the shards max read level, and re-injects them
// to the executions table.
// Returns an error when the domain is not enabled or when the tasks cannot be fetched or re-injected.
func (p *ProcessorImpl) processAckLevel(ctx context.Context, al persistence.HistoryDLQAckLevel) error {
	if p.domainMode(al.DomainID) != constants.HistoryTaskDLQModeEnabled {
		p.logger.Debug("DLQ not enabled for domain, skipping ack level processing", tag.ShardID(p.shardID), tag.WorkflowDomainID(al.DomainID))
		return nil
	}

	// Reinjection only supports transfer and timer tasks (see ExecutionManager.CreateHistoryTasks).
	// Skip any other category (e.g. replication) so an ack level cannot block processing.
	if id := al.TaskCategory.ID(); id != persistence.HistoryTaskCategoryIDTransfer &&
		id != persistence.HistoryTaskCategoryIDTimer {
		p.logger.Debug("Skipping DLQ ack level for unsupported task category",
			tag.ShardID(p.shardID),
			tag.WorkflowDomainID(al.DomainID),
			tag.TaskType(al.TaskCategory.ID()))
		return nil
	}

	scope := p.metricsClient.Scope(metrics.HistoryTaskDLQProcessorScope, metrics.DomainTag(al.DomainID))

	var (
		pageToken   []byte
		lastGoodKey *persistence.HistoryTaskKey
		firstErr    error
	)
	// Start just past the current ack position.
	minKey := persistence.NewHistoryTaskKey(al.AckLevelVisibilityTS, al.AckLevelTaskID).Next()
	// TODO(c-warren): Pass in max read level from the shard context.
	maxKey := persistence.MaximumHistoryTaskKey

	for {
		// Preempt promptly between pages when the sweep is canceled; any pages already
		// reinjected are still acknowledged below so progress is not lost.
		if err := ctx.Err(); err != nil {
			firstErr = err
			break
		}
		resp, err := p.mgr.GetHistoryDLQTasks(ctx, persistence.HistoryDLQGetTasksRequest{
			ShardID:               al.ShardID,
			DomainID:              al.DomainID,
			ClusterAttributeScope: al.ClusterAttributeScope,
			ClusterAttributeName:  al.ClusterAttributeName,
			TaskCategory:          al.TaskCategory,
			InclusiveMinTaskKey:   minKey,
			ExclusiveMaxTaskKey:   maxKey,
			PageSize:              p.pageSize,
			NextPageToken:         pageToken,
		})
		if err != nil {
			firstErr = err
			break
		}

		if len(resp.Tasks) > 0 {
			scope.RecordHistogramValue(metrics.HistoryTaskDLQPageSizeBytes, float64(resp.PageSizeBytes))
			k := resp.Tasks[len(resp.Tasks)-1].GetTaskKey()
			if err := p.reinjector.ReinjectHistoryTasks(ctx, resp.Tasks); err != nil {
				scope.IncCounter(metrics.HistoryTaskDLQReinjectFailuresCounter)
				firstErr = err
				break
			}
			lastGoodKey = &k
		}

		if len(resp.NextPageToken) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}

	if lastGoodKey != nil {
		// Use the processor context, not the passed-in (possibly sweep-canceled) context, so that
		// pages already reinjected above are acknowledged and not re-injected on the next run.
		if err := p.advanceAckLevel(p.ctx, al, *lastGoodKey); err != nil {
			return multierr.Append(err, firstErr)
		}
	}
	return firstErr
}

// advanceAckLevel updates the persistent ack level and then removes the acknowledged
// tasks. UpdateAckLevel runs first so that a crash between the two steps only leaves
// orphaned rows (which DeleteTasks can clean up on the next run).
func (p *ProcessorImpl) advanceAckLevel(ctx context.Context, al persistence.HistoryDLQAckLevel, newKey persistence.HistoryTaskKey) error {
	if err := p.mgr.UpdateHistoryDLQAckLevel(ctx, persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: newKey,
	}); err != nil {
		return fmt.Errorf("update DLQ ack level: %w", err)
	}
	if err := p.mgr.DeleteHistoryDLQTasks(ctx, persistence.HistoryDLQDeleteTasksRequest{
		ShardID:               al.ShardID,
		DomainID:              al.DomainID,
		ClusterAttributeScope: al.ClusterAttributeScope,
		ClusterAttributeName:  al.ClusterAttributeName,
		TaskCategory:          al.TaskCategory,
		ExclusiveMaxTaskKey:   newKey.Next(),
	}); err != nil {
		p.logger.Error("failed to delete acknowledged DLQ tasks",
			tag.WorkflowDomainID(al.DomainID),
			tag.Error(err),
		)
	}
	return nil
}

func (p *Partition) String() string {
	return fmt.Sprintf("DomainID/%s/ClusterAttributeScope/%s/ClusterAttributeName/%s", p.DomainID, p.ClusterAttributeScope, p.ClusterAttributeName)
}
