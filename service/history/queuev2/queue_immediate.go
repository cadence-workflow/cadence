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
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

type (
	immediateQueue struct {
		*queueBase
		notifyCh chan struct{}
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
	return &immediateQueue{
		queueBase: newQueueBase(
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
	}
}

func (q *immediateQueue) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	q.logger.Info("History queue state changed", tag.LifeCycleStarting)
	defer q.logger.Info("History queue state changed", tag.LifeCycleStarted)

	q.queueBase.Start()

	q.shutdownWG.Add(1)
	go q.processEventLoop()

	q.notify()
}

func (q *immediateQueue) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	q.logger.Info("History queue state changed", tag.LifeCycleStopping)
	defer q.logger.Info("History queue state changed", tag.LifeCycleStopped)

	q.cancel()
	q.shutdownWG.Wait()

	q.queueBase.Stop()
}

func (q *immediateQueue) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	if len(info.Tasks) == 0 {
		return
	}

	q.notify()
}

func (q *immediateQueue) notify() {
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

func (q *immediateQueue) processEventLoop() {
	defer q.shutdownWG.Done()

	for {
		select {
		case <-q.notifyCh:
			q.processNewTasks()
		case <-q.pollTimer.Chan():
			q.processPollTimer()
		case <-q.updateQueueStateTimer.Chan():
			q.updateQueueState()
		case <-q.ctx.Done():
			return
		}
	}
}
