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
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type fifoTaskSchedulerImpl struct {
	status       int32
	logger       log.Logger
	metricsScope metrics.Scope
	options      *FIFOTaskSchedulerOptions
	dispatcherWG sync.WaitGroup
	taskCh       chan PriorityTask
	ctx          context.Context
	cancel       context.CancelFunc

	processor Processor
}

// NewFIFOTaskScheduler creates a new FIFO task scheduler
// it's an no-op implementation as it simply copy tasks from
// one task channel to another task channel.
// This scheduler is only for development purpose.
func NewFIFOTaskScheduler(
	logger log.Logger,
	metricsClient metrics.Client,
	options *FIFOTaskSchedulerOptions,
) Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &fifoTaskSchedulerImpl{
		status:       common.DaemonStatusInitialized,
		logger:       logger,
		metricsScope: metricsClient.Scope(metrics.TaskSchedulerScope),
		options:      options,
		taskCh:       make(chan PriorityTask, options.QueueSize),
		ctx:          ctx,
		cancel:       cancel,
		processor: NewParallelTaskProcessor(
			logger,
			metricsClient,
			&ParallelTaskProcessorOptions{
				QueueSize:   options.QueueSize,
				WorkerCount: options.WorkerCount,
				RetryPolicy: options.RetryPolicy,
			},
		),
	}
}

func (f *fifoTaskSchedulerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	f.processor.Start()

	f.dispatcherWG.Add(f.options.DispatcherCount)
	for i := 0; i != f.options.DispatcherCount; i++ {
		go f.dispatcher()
	}

	f.logger.Info("FIFO task scheduler started.")
}

func (f *fifoTaskSchedulerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&f.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	f.cancel()

	f.processor.Stop()

	f.drainAndNackTasks()

	if success := common.AwaitWaitGroup(&f.dispatcherWG, time.Minute); !success {
		f.logger.Warn("FIFO task scheduler timedout on shutdown.")
	}

	f.logger.Info("FIFO task scheduler shutdown.")
}

func (f *fifoTaskSchedulerImpl) Submit(task PriorityTask) error {
	f.metricsScope.IncCounter(metrics.ParallelTaskSubmitRequest)
	sw := f.metricsScope.StartTimer(metrics.ParallelTaskSubmitLatency)
	defer sw.Stop()

	if f.isStopped() {
		return ErrTaskSchedulerClosed
	}

	select {
	case f.taskCh <- task:
		if f.isStopped() {
			f.drainAndNackTasks()
		}
		return nil
	case <-f.ctx.Done():
		return ErrTaskSchedulerClosed
	}
}

func (f *fifoTaskSchedulerImpl) TrySubmit(task PriorityTask) (bool, error) {
	if f.isStopped() {
		return false, ErrTaskSchedulerClosed
	}

	select {
	case f.taskCh <- task:
		f.metricsScope.IncCounter(metrics.ParallelTaskSubmitRequest)
		if f.isStopped() {
			f.drainAndNackTasks()
		}
		return true, nil
	case <-f.ctx.Done():
		return false, ErrTaskSchedulerClosed
	default:
		return false, nil
	}
}

func (f *fifoTaskSchedulerImpl) dispatcher() {
	defer f.dispatcherWG.Done()

	for {
		select {
		case task := <-f.taskCh:
			if err := f.processor.Submit(task); err != nil {
				f.logger.Error("failed to submit task to processor", tag.Error(err))
				task.Nack(err)
			}
		case <-f.ctx.Done():
			return
		}
	}
}

func (f *fifoTaskSchedulerImpl) isStopped() bool {
	return atomic.LoadInt32(&f.status) == common.DaemonStatusStopped
}

func (f *fifoTaskSchedulerImpl) drainAndNackTasks() {
	for {
		select {
		case task := <-f.taskCh:
			task.Nack(nil)
		default:
			return
		}
	}
}
