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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination processor_mock.go -self_package github.com/uber/cadence/service/history/asyncworkflowqueue

// Package asyncworkflowqueue contains the per-shard background daemon that garbage
// collects history-backed async-workflow-queue messages behind the committed ack level.
package asyncworkflowqueue

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
)

type (
	// Processor is a per-shard background daemon that periodically range-deletes
	// async-workflow-queue messages that are at or behind the committed ack level.
	//
	// Start/Stop manage a background goroutine that periodically calls processShard
	// for the shard this processor was created for. This is GC only; it does NOT move
	// poison messages to the DLQ.
	Processor interface {
		common.Daemon
	}

	processorImpl struct {
		shardID    int
		mgr        persistence.AsyncWorkflowQueueManager
		enabled    dynamicproperties.BoolPropertyFn
		interval   dynamicproperties.DurationPropertyFnWithShardIDFilter
		timeSource clock.TimeSource
		metrics    metrics.Client
		logger     log.Logger

		status int32
		ctx    context.Context
		cancel context.CancelFunc
		wg     sync.WaitGroup
	}

	// ProcessorParams are the dependencies needed to build a Processor.
	ProcessorParams struct {
		ShardID    int
		Manager    persistence.AsyncWorkflowQueueManager
		Enabled    dynamicproperties.BoolPropertyFn
		Interval   dynamicproperties.DurationPropertyFnWithShardIDFilter
		TimeSource clock.TimeSource
		Metrics    metrics.Client
		Logger     log.Logger
	}
)

var _ Processor = (*processorImpl)(nil)

// NewProcessor creates a Processor from the given dependencies.
func NewProcessor(params ProcessorParams) Processor {
	return &processorImpl{
		shardID:    params.ShardID,
		mgr:        params.Manager,
		enabled:    params.Enabled,
		interval:   params.Interval,
		timeSource: params.TimeSource,
		metrics:    params.Metrics,
		logger:     params.Logger,
		status:     common.DaemonStatusInitialized,
		cancel:     func() {}, // no-op until Start() sets the real cancel
	}
}

// NewProcessorFromShard is a convenience constructor that derives the shard-scoped
// dependencies (shard ID, async queue manager, time source, metrics, logger) from
// the shard context and delegates to NewProcessor.
func NewProcessorFromShard(
	shard shard.Context,
	enabled dynamicproperties.BoolPropertyFn,
	interval dynamicproperties.DurationPropertyFnWithShardIDFilter,
) Processor {
	return NewProcessor(ProcessorParams{
		ShardID:    shard.GetShardID(),
		Manager:    shard.GetService().GetAsyncWorkflowQueueManager(),
		Enabled:    enabled,
		Interval:   interval,
		TimeSource: shard.GetTimeSource(),
		Metrics:    shard.GetMetricsClient(),
		Logger:     shard.GetLogger(),
	})
}

// Start starts the processor and launches the background GC loop.
func (p *processorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.logger.Debug("Async workflow queue GC starting", tag.ShardID(p.shardID))
	p.wg.Add(1)
	go p.processLoop()
	p.logger.Debug("Async workflow queue GC started", tag.ShardID(p.shardID))
}

// Stop signals the background loop to exit and waits for it to finish. Idempotent.
func (p *processorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	p.logger.Debug("Async workflow queue GC stopping", tag.ShardID(p.shardID))
	p.cancel()
	p.wg.Wait()
	p.logger.Debug("Async workflow queue GC stopped", tag.ShardID(p.shardID))
}

// processLoop is the background goroutine that periodically calls processShard.
// It reads the interval on every tick so that dynamic-config changes take effect
// without a restart, and it is gated by the enabled() flag.
func (p *processorImpl) processLoop() {
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
				p.processShard(p.ctx)
			}
			timer.Reset(p.interval(p.shardID))
		}
	}
}

// processShard range-deletes messages at or behind the committed ack level for this
// shard. Errors are logged and counted; the next tick retries.
func (p *processorImpl) processShard(ctx context.Context) {
	scope := p.metrics.Scope(metrics.AsyncWorkflowQueueGCScope)

	ackResp, err := p.mgr.GetAckLevel(ctx, &persistence.GetAsyncWorkflowAckLevelRequest{
		ShardID: p.shardID,
	})
	if err != nil {
		scope.IncCounter(metrics.AsyncWorkflowQueueGCFailuresCounter)
		p.logger.Error("failed to get async workflow queue ack level",
			tag.ShardID(p.shardID),
			tag.Error(err),
		)
		return
	}

	// A never-consumed queue reports EmptyMessageID (-1); there is nothing to GC.
	if ackResp.AckLevel <= constants.EmptyMessageID {
		return
	}

	if err := p.mgr.RangeDeleteMessages(ctx, &persistence.RangeDeleteAsyncWorkflowMessagesRequest{
		ShardID:               p.shardID,
		InclusiveEndMessageID: ackResp.AckLevel,
	}); err != nil {
		scope.IncCounter(metrics.AsyncWorkflowQueueGCFailuresCounter)
		p.logger.Error("failed to range-delete async workflow queue messages",
			tag.ShardID(p.shardID),
			tag.Error(err),
		)
		return
	}
	scope.IncCounter(metrics.AsyncWorkflowQueueGCSweepsCounter)
}
