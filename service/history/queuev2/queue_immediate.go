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

package queuev2

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

type (
	immediateQueue struct {
		base     *queueBase
		notifyCh chan struct{}

		status       int32
		shutdownWG   sync.WaitGroup
		ctx          context.Context
		cancel       func()
		pollTimer    clock.Timer
		lastPollTime time.Time
	}
)

func NewImmediateQueue(
	shard shard.Context,
	category persistence.HistoryTaskCategory,
	taskProcessor task.Processor,
	taskExecutor task.Executor,
	logger log.Logger,
	metricsClient metrics.Client,
	metricsScope metrics.Scope,
	options *Options,
) Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &immediateQueue{
		base: newQueueBase(
			shard,
			taskProcessor,
			logger,
			metricsClient,
			metricsScope,
			category,
			taskExecutor,
			options,
		),
		notifyCh: make(chan struct{}, 1),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (q *immediateQueue) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	q.base.logger.Info("History queue state changed", tag.LifeCycleStarting)
	defer q.base.logger.Info("History queue state changed", tag.LifeCycleStarted)

	q.base.Start()

	q.shutdownWG.Add(1)
	go q.processEventLoop()

	q.notify()
}

func (q *immediateQueue) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	q.base.logger.Info("History queue state changed", tag.LifeCycleStopping)
	defer q.base.logger.Info("History queue state changed", tag.LifeCycleStopped)

	q.cancel()
	q.shutdownWG.Wait()

	q.base.Stop()
}

func (q *immediateQueue) Category() persistence.HistoryTaskCategory {
	return q.base.Category()
}

func (q *immediateQueue) FailoverDomain(domainIDs map[string]struct{}) {
	q.base.FailoverDomain(domainIDs)
}

func (q *immediateQueue) HandleAction(ctx context.Context, clusterName string, action *queue.Action) (*queue.ActionResult, error) {
	return q.base.HandleAction(ctx, clusterName, action)
}

func (q *immediateQueue) UnlockTaskProcessing() {
	q.base.UnlockTaskProcessing()
}

func (q *immediateQueue) LockTaskProcessing() {
	q.base.LockTaskProcessing()
}

func (q *immediateQueue) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	numTasks := len(info.Tasks)
	if numTasks == 0 {
		return
	}

	q.notify()
	q.base.metricsScope.AddCounter(metrics.NewHistoryTaskCounter, int64(numTasks))
}

func (q *immediateQueue) notify() {
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

func (q *immediateQueue) processPollTimer() {
	// NOTE: ideally when new tasks are written to the queue, a notification should be sent to the notifyCh,
	// thus this periodic poll is not needed, but we keep it for now to provide a fallback mechanism in case
	// there is a bug in the notification mechanism
	if q.lastPollTime.Add(q.base.options.PollBackoffInterval()).Before(q.base.timeSource.Now()) {
		newTasks := q.base.processNewTasks()
		if newTasks {
			// TODO: consider changing it to warn level
			q.base.logger.Info("processing new tasks because poll timer fired")
		}
		q.lastPollTime = q.base.timeSource.Now()
	}

	q.pollTimer.Reset(backoff.JitDuration(
		q.base.options.MaxPollInterval(),
		q.base.options.MaxPollIntervalJitterCoefficient(),
	))
}

func (q *immediateQueue) processEventLoop() {
	defer q.shutdownWG.Done()

	q.pollTimer = q.base.timeSource.NewTimer(backoff.JitDuration(
		q.base.options.MaxPollInterval(),
		q.base.options.MaxPollIntervalJitterCoefficient(),
	))
	defer q.pollTimer.Stop()
	for {
		select {
		case <-q.notifyCh:
			q.base.processNewTasks()
			q.lastPollTime = q.base.timeSource.Now()
		case <-q.pollTimer.Chan():
			q.processPollTimer()
		case <-q.base.updateQueueStateTimer.Chan():
			q.base.updateQueueState(q.ctx)
		case alert := <-q.base.alertCh:
			q.base.handleAlert(q.ctx, alert)
		case <-q.ctx.Done():
			return
		}
	}
}
