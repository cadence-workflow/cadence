// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package task

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type hierarchicalWeightedRoundRobinTaskSchedulerImpl[K comparable, T Task] struct {
	sync.RWMutex

	status       int32
	pool         *hierarchicalWeightedRoundRobinTaskPoolImpl[K, T]
	ctx          context.Context
	cancel       context.CancelFunc
	notifyCh     chan struct{}
	dispatcherWG sync.WaitGroup
	logger       log.Logger
	metricsScope metrics.Scope
	options      *HierarchicalWeightedRoundRobinTaskPoolOptions[K, T]

	processor Processor
}

// NewHierarchicalWeightedRoundRobinTaskScheduler creates a new hierarchical WRR task scheduler
func NewHierarchicalWeightedRoundRobinTaskScheduler[K comparable, T Task](
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	processor Processor,
	options *HierarchicalWeightedRoundRobinTaskPoolOptions[K, T],
) (Scheduler[T], error) {
	metricsScope := metricsClient.Scope(metrics.TaskSchedulerScope)
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := &hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]{
		status: common.DaemonStatusInitialized,
		pool: newHierarchicalWeightedRoundRobinTaskPool[K](
			logger,
			metricsClient,
			timeSource,
			options,
		),
		ctx:          ctx,
		cancel:       cancel,
		notifyCh:     make(chan struct{}, 1),
		logger:       logger,
		metricsScope: metricsScope,
		options:      options,
		processor:    processor,
	}

	return scheduler, nil
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) Start() {
	if !atomic.CompareAndSwapInt32(&w.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
	w.pool.Start()

	w.dispatcherWG.Add(1)
	go w.dispatcher()
	w.logger.Info("Hierarchical weighted round robin task scheduler started.")
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) Stop() {
	if !atomic.CompareAndSwapInt32(&w.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	w.cancel()
	w.pool.Stop()

	if success := common.AwaitWaitGroup(&w.dispatcherWG, time.Minute); !success {
		w.logger.Warn("Hierarchical weighted round robin task scheduler timedout on shutdown.")
	}

	w.drainAndNackTasks()

	w.logger.Info("Hierarchical weighted round robin task scheduler stopped.")
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) Submit(task T) error {
	w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
	sw := w.metricsScope.StartTimer(metrics.PriorityTaskSubmitLatency)
	defer sw.Stop()

	if w.isStopped() {
		return ErrTaskSchedulerClosed
	}

	if err := w.pool.Enqueue(task); err != nil {
		return err
	}
	w.notifyDispatcher()
	return nil
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) TrySubmit(
	task T,
) (bool, error) {
	w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
	sw := w.metricsScope.StartTimer(metrics.PriorityTaskSubmitLatency)
	defer sw.Stop()

	if w.isStopped() {
		return false, ErrTaskSchedulerClosed
	}

	ok, err := w.pool.TryEnqueue(task)
	if ok {
		w.notifyDispatcher()
	}
	return ok, err
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) dispatcher() {
	defer w.dispatcherWG.Done()

	for {
		select {
		case <-w.notifyCh:
			w.dispatchTasks()
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) dispatchTasks() {
	for {
		if w.isStopped() {
			return
		}
		task, ok := w.pool.TryDequeue()
		if !ok {
			return
		}
		if err := w.processor.Submit(task); err != nil {
			w.logger.Error("fail to submit task to processor", tag.Error(err))
			task.Nack(err)
		}
	}
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) notifyDispatcher() {
	select {
	case w.notifyCh <- struct{}{}:
		// sent a notification to the dispatcher
	default:
		// do not block if there's already a notification
	}
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) isStopped() bool {
	return atomic.LoadInt32(&w.status) == common.DaemonStatusStopped
}

func (w *hierarchicalWeightedRoundRobinTaskSchedulerImpl[K, T]) drainAndNackTasks() {
	for {
		task, ok := w.pool.TryDequeue()
		if !ok {
			return
		}
		task.Nack(nil)
	}
}
