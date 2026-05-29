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
	"time"

	"go.uber.org/multierr"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/constants"
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
	}

	ProcessorImpl struct {
		shardID    int
		mgr        persistence.HistoryTaskDLQManager
		executors  map[int]TaskExecutor // persistence.HistoryTaskCategoryID* → executor
		pageSize   int
		interval   dynamicproperties.DurationPropertyFnWithShardIDFilter
		domainMode dynamicproperties.StringPropertyFnWithDomainFilter
		enabled    dynamicproperties.BoolPropertyFn
		timeSource clock.TimeSource
		logger     log.Logger

		status    int32
		ctx       context.Context
		cancel    context.CancelFunc
		wg        sync.WaitGroup
		processMu sync.Mutex // serializes ProcessShard and ProcessPartition
	}
)

var _ Processor = (*ProcessorImpl)(nil)

// NewProcessor creates a Processor that reads from the history task DLQ for shardID.
//
// executors maps persistence.HistoryTaskCategoryID* constants to the appropriate
// historyqueuev2 executor for each task type.
//
// interval controls how often the background loop calls ProcessShard.
func NewProcessor(
	shardID int,
	mgr persistence.HistoryTaskDLQManager,
	executors map[int]TaskExecutor,
	pageSize int,
	interval dynamicproperties.DurationPropertyFnWithShardIDFilter,
	domainMode dynamicproperties.StringPropertyFnWithDomainFilter,
	enabled dynamicproperties.BoolPropertyFn,
	timeSource clock.TimeSource,
	logger log.Logger,
) *ProcessorImpl {
	return &ProcessorImpl{
		shardID:    shardID,
		mgr:        mgr,
		executors:  executors,
		pageSize:   pageSize,
		interval:   interval,
		domainMode: domainMode,
		enabled:    enabled,
		timeSource: timeSource,
		logger:     logger,
		status:     common.DaemonStatusInitialized,
		cancel:     func() {}, // no-op until Start() sets the real cancel
	}
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

// processLoop is the background goroutine that periodically calls ProcessShard.
// It reads the interval on every tick so that dynamic-config changes take effect
// without a restart.
func (p *ProcessorImpl) processLoop() {
	defer p.wg.Done()
	defer func() { log.CapturePanic(recover(), p.logger, nil) }()

	timer := p.timeSource.NewTimer(p.interval(p.shardID))
	defer timer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-timer.Chan():
			if p.enabled() {
				if err := p.ProcessShard(p.ctx); err != nil {
					p.logger.Error("DLQ periodic shard sweep failed",
						tag.ShardID(p.shardID),
						tag.Error(err),
					)
				}
			}
			timer.Reset(p.interval(p.shardID))
		}
	}
}

func (p *ProcessorImpl) ProcessShard(ctx context.Context) error {
	p.processMu.Lock()
	defer p.processMu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ackLevels, err := p.mgr.GetAckLevels(ctx, persistence.HistoryDLQGetAckLevelsRequest{
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
	ackLevels, err := p.mgr.GetAckLevels(ctx, persistence.HistoryDLQGetAckLevelsRequest{
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

// processAckLevels attempts to process every ack level entry. All partitions are
// attempted regardless of individual failures; all errors are combined and returned.
func (p *ProcessorImpl) processAckLevels(ctx context.Context, ackLevels []persistence.HistoryDLQAckLevel) error {
	var errs error
	for _, al := range ackLevels {
		if err := p.processAckLevel(ctx, al); err != nil {
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

// processAckLevel pages through the tasks for one (partition, taskType) and executes
// each one. It stops at the first execution failure, then advances the ack level to
// the last successfully executed task key.
func (p *ProcessorImpl) processAckLevel(ctx context.Context, al persistence.HistoryDLQAckLevel) error {
	if p.domainMode(al.DomainID) != constants.HistoryTaskDLQModeEnabled {
		p.logger.Debug("DLQ not enabled for domain, skipping ack level processing", tag.ShardID(p.shardID), tag.WorkflowDomainID(al.DomainID))
		return nil
	}

	executor, ok := p.executors[al.TaskCategory.ID()]
	if !ok {
		return fmt.Errorf("no executor registered for task type %d", al.TaskCategory.ID())
	}

	var (
		pageToken   []byte
		lastGoodKey *persistence.HistoryTaskKey
		firstErr    error
	)
	// Start just past the current ack position.
	minKey := persistence.NewHistoryTaskKey(al.AckLevelVisibilityTS, al.AckLevelTaskID).Next()
	// TODO(c-warren): Pass in max read level from the shard context
	maxKey := persistence.NewHistoryTaskKey(time.Unix(1<<62, 0), 0)

	for {
		resp, err := p.mgr.GetTasks(ctx, persistence.HistoryDLQGetTasksRequest{
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

		for _, t := range resp.Tasks {
			if err := executor.Execute(ctx, t); err != nil {
				if handledErr := executor.HandleErr(err); handledErr != nil {
					firstErr = handledErr
					break
				}
				// ackable error: log and skip this task, advance past it
				p.logger.Warn("skipping ackable DLQ task execution error",
					tag.WorkflowDomainID(al.DomainID),
					tag.Error(err),
				)
			}
			k := t.GetTaskKey()
			lastGoodKey = &k
		}

		if firstErr != nil || len(resp.NextPageToken) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}

	if lastGoodKey != nil {
		if err := p.advanceAckLevel(ctx, al, *lastGoodKey); err != nil {
			return multierr.Append(err, firstErr)
		}
	}
	return firstErr
}

// advanceAckLevel updates the persistent ack level and then removes the acknowledged
// tasks. UpdateAckLevel runs first so that a crash between the two steps only leaves
// orphaned rows (which DeleteTasks can clean up on the next run).
func (p *ProcessorImpl) advanceAckLevel(ctx context.Context, al persistence.HistoryDLQAckLevel, newKey persistence.HistoryTaskKey) error {
	if err := p.mgr.UpdateAckLevel(ctx, persistence.HistoryDLQUpdateAckLevelRequest{
		ShardID:                   al.ShardID,
		DomainID:                  al.DomainID,
		ClusterAttributeScope:     al.ClusterAttributeScope,
		ClusterAttributeName:      al.ClusterAttributeName,
		TaskCategory:              al.TaskCategory,
		UpdatedInclusiveReadLevel: newKey,
	}); err != nil {
		return fmt.Errorf("update DLQ ack level: %w", err)
	}
	if err := p.mgr.DeleteTasks(ctx, persistence.HistoryDLQDeleteTasksRequest{
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
