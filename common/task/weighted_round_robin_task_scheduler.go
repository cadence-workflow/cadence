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
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type weightedRoundRobinTaskSchedulerImpl[K comparable] struct {
	sync.RWMutex

	status       int32
	taskChs      map[K]chan PriorityTask
	shutdownCh   chan struct{}
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
)

// NewWeightedRoundRobinTaskScheduler creates a new WRR task scheduler
func NewWeightedRoundRobinTaskScheduler[K comparable](
	logger log.Logger,
	metricsClient metrics.Client,
	options *WeightedRoundRobinTaskSchedulerOptions[K],
) (Scheduler, error) {
	scheduler := &weightedRoundRobinTaskSchedulerImpl[K]{
		status:       common.DaemonStatusInitialized,
		taskChs:      make(map[K]chan PriorityTask),
		shutdownCh:   make(chan struct{}),
		notifyCh:     make(chan struct{}, 1),
		logger:       logger,
		metricsScope: metricsClient.Scope(metrics.TaskSchedulerScope),
		options:      options,
		processor: NewParallelTaskProcessor(
			logger,
			metricsClient,
			&ParallelTaskProcessorOptions{
				QueueSize:   wRRTaskProcessorQueueSize,
				WorkerCount: options.WorkerCount,
				RetryPolicy: options.RetryPolicy,
			},
		),
	}

	return scheduler, nil
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) Start() {
	if !atomic.CompareAndSwapInt32(&w.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	w.processor.Start()

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

	close(w.shutdownCh)

	w.processor.Stop()

	w.RLock()
	for _, taskCh := range w.taskChs {
		drainAndNackPriorityTask(taskCh)
	}
	w.RUnlock()

	if success := common.AwaitWaitGroup(&w.dispatcherWG, time.Minute); !success {
		w.logger.Warn("Weighted round robin task scheduler timedout on shutdown.")
	}

	w.logger.Info("Weighted round robin task scheduler shutdown.")
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) Submit(task PriorityTask) error {
	w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
	sw := w.metricsScope.StartTimer(metrics.PriorityTaskSubmitLatency)
	defer sw.Stop()

	if w.isStopped() {
		return ErrTaskSchedulerClosed
	}

	taskCh := w.getOrCreateTaskChan(task)
	select {
	case taskCh <- task:
		w.notifyDispatcher()
		if w.isStopped() {
			drainAndNackPriorityTask(taskCh)
		}
		return nil
	case <-w.shutdownCh:
		return ErrTaskSchedulerClosed
	}
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) TrySubmit(
	task PriorityTask,
) (bool, error) {
	if w.isStopped() {
		return false, ErrTaskSchedulerClosed
	}

	taskCh := w.getOrCreateTaskChan(task)

	select {
	case taskCh <- task:
		w.metricsScope.IncCounter(metrics.PriorityTaskSubmitRequest)
		if w.isStopped() {
			drainAndNackPriorityTask(taskCh)
		} else {
			w.notifyDispatcher()
		}
		return true, nil
	case <-w.shutdownCh:
		return false, ErrTaskSchedulerClosed
	default:
		return false, nil
	}
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) dispatcher() {
	defer w.dispatcherWG.Done()

	outstandingTasks := false
	taskChs := make(map[K]chan PriorityTask)

	for {
		if !outstandingTasks {
			// if no task is dispatched in the last round,
			// wait for a notification
			w.logger.Debug("Weighted round robin task scheduler is waiting for new task notification because there was no task dispatched in the last round.")
			select {
			case <-w.notifyCh:
				// block until there's a new task
				w.logger.Debug("Weighted round robin task scheduler got notification so will check for new tasks.")
			case <-w.shutdownCh:
				return
			}
		}

		outstandingTasks = false
		w.updateTaskChs(taskChs)
		for key, taskCh := range taskChs {
			weight := w.options.ChannelKeyToWeightFn(key)
		Submit_Loop:
			for i := 0; i < weight; i++ {
				select {
				case task := <-taskCh:
					// dispatched at least one task in this round
					outstandingTasks = true

					if err := w.processor.Submit(task); err != nil {
						w.logger.Error("fail to submit task to processor", tag.Error(err))
						task.Nack()
					}
				case <-w.shutdownCh:
					return
				default:
					// if no task, don't block. Skip to next priority
					break Submit_Loop
				}
			}
		}
	}
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) getOrCreateTaskChan(task PriorityTask) chan PriorityTask {
	key := w.options.TaskToChannelKeyFn(task)
	w.RLock()
	if taskCh, ok := w.taskChs[key]; ok {
		w.RUnlock()
		return taskCh
	}
	w.RUnlock()

	w.Lock()
	defer w.Unlock()
	if taskCh, ok := w.taskChs[key]; ok {
		return taskCh
	}
	taskCh := make(chan PriorityTask, w.options.QueueSize)
	w.taskChs[key] = taskCh
	return taskCh
}

func (w *weightedRoundRobinTaskSchedulerImpl[K]) updateTaskChs(taskChs map[K]chan PriorityTask) {
	w.RLock()
	defer w.RUnlock()

	for key, taskCh := range w.taskChs {
		if _, ok := taskChs[key]; !ok {
			taskChs[key] = taskCh
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
			task.Nack()
		default:
			return
		}
	}
}
