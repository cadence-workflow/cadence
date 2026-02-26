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
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type weightedRoundRobinTaskSchedulerImpl[K comparable] struct {
	sync.RWMutex

	status       int32
	taskPool     TaskPool
	ctx          context.Context
	cancel       context.CancelFunc
	notifyCh     chan struct{}
	dispatcherWG sync.WaitGroup
	logger       log.Logger
	metricsScope metrics.Scope
	options      *WeightedRoundRobinTaskSchedulerOptions[K]

	processor Processor
}

const (
	wRRTaskProcessorQueueSize    = 1
	defaultUpdateWeightsInterval = 5 * time.Second
)

var (
	// ErrTaskSchedulerClosed is the error returned when submitting task to a stopped scheduler
	ErrTaskSchedulerClosed = errors.New("task scheduler has already shutdown")
	// ErrTaskWeightZero is the error returned when submitting task with weight 0
	ErrTaskWeightZero = errors.New("task weight must be greater than zero")
)

// NewWeightedRoundRobinTaskScheduler creates a new WRR task scheduler
func NewWeightedRoundRobinTaskScheduler[K comparable](
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	processor Processor,
	options *WeightedRoundRobinTaskSchedulerOptions[K],
) (Scheduler, error) {
	metricsScope := metricsClient.Scope(metrics.TaskSchedulerScope)
	ctx, cancel := context.WithCancel(context.Background())

	taskPool := NewWeightedRoundRobinTaskPool[K](
		logger,
		metricsClient,
		timeSource,
		&WeightedRoundRobinTaskPoolOptions[K]{
			QueueSize:            options.QueueSize,
			TaskToChannelKeyFn:   options.TaskToChannelKeyFn,
			ChannelKeyToWeightFn: options.ChannelKeyToWeightFn,
		},
	)

	scheduler := &weightedRoundRobinTaskSchedulerImpl[K]{
		status:       common.DaemonStatusInitialized,
		taskPool:     taskPool,
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

func (w *weightedRoundRobinTaskSchedulerImpl[K]) Start() {
	if !atomic.CompareAndSwapInt32(&w.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	w.taskPool.Start()

	w.dispatcherWG.Add(w.options.DispatcherCount)
	for i := 0; i != w.options.DispatcherCount; i++ {
		go w.dispatcher()
	}
	w.logger.Info("Weighted round robin task scheduler started.")
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) Stop() {
	if !atomic.CompareAndSwapInt32(&w.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	w.cancel()
	w.taskPool.Stop()

	if success := common.AwaitWaitGroup(&w.dispatcherWG, time.Minute); !success {
		w.logger.Warn("Weighted round robin task scheduler timedout on shutdown.")
	}

	w.logger.Info("Weighted round robin task scheduler shutdown.")
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) Submit(task PriorityTask) error {
	w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
	sw := w.metricsScope.StartTimer(metrics.PriorityTaskSubmitLatency)
	defer sw.Stop()

	err := w.taskPool.Submit(task)
	if err == nil {
		w.notifyDispatcher()
	}
	return err
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) TrySubmit(
	task PriorityTask,
) (bool, error) {
	submitted, err := w.taskPool.TrySubmit(task)
	if submitted {
		w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
		w.notifyDispatcher()
	}
	return submitted, err
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) dispatcher() {
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

func (w *weightedRoundRobinTaskSchedulerImpl[K]) dispatchTasks() {
	for w.taskPool.Len() > 0 {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		task, ok := w.taskPool.GetNextTask()
		if !ok {
			// No more tasks available
			return
		}

		if err := w.processor.Submit(task); err != nil {
			w.logger.Error("fail to submit task to processor", tag.Error(err))
			task.Nack(err)
		}
	}
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) notifyDispatcher() {
	select {
	case w.notifyCh <- struct{}{}:
		// sent a notification to the dispatcher
	default:
		// do not block if there's already a notification
	}
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) isStopped() bool {
	return atomic.LoadInt32(&w.status) == common.DaemonStatusStopped
}

func drainAndNackPriorityTask(taskCh <-chan PriorityTask) {
	for {
		select {
		case task := <-taskCh:
			task.Nack(nil)
		default:
			return
		}
	}
}
