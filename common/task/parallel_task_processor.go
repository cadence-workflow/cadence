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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

type (
	// ParallelTaskProcessorOptions configs PriorityTaskProcessor
	ParallelTaskProcessorOptions struct {
		QueueSize   int
		WorkerCount dynamicproperties.IntPropertyFn
		RetryPolicy backoff.RetryPolicy
	}

	parallelTaskProcessorImpl struct {
		status           int32
		tasksCh          chan Task
		shutdownCh       chan struct{}
		workerShutdownCh []chan struct{}
		shutdownWG       sync.WaitGroup
		logger           log.Logger
		metricsScope     metrics.Scope
		options          *ParallelTaskProcessorOptions
	}
)

const (
	defaultMonitorTickerDuration = 5 * time.Second
)

var (
	// ErrTaskProcessorClosed is the error returned when submiting task to a stopped processor
	ErrTaskProcessorClosed = errors.New("task processor has already shutdown")
)

// NewParallelTaskProcessor creates a new PriorityTaskProcessor
func NewParallelTaskProcessor(
	logger log.Logger,
	metricsClient metrics.Client,
	options *ParallelTaskProcessorOptions,
) Processor {
	return &parallelTaskProcessorImpl{
		status:           common.DaemonStatusInitialized,
		tasksCh:          make(chan Task, options.QueueSize),
		shutdownCh:       make(chan struct{}),
		workerShutdownCh: make([]chan struct{}, 0, options.WorkerCount()),
		logger:           logger,
		metricsScope:     metricsClient.Scope(metrics.ParallelTaskProcessingScope),
		options:          options,
	}
}

func (p *parallelTaskProcessorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	initialWorkerCount := p.options.WorkerCount()

	p.shutdownWG.Add(initialWorkerCount)
	for i := 0; i < initialWorkerCount; i++ {
		shutdownCh := make(chan struct{})
		p.workerShutdownCh = append(p.workerShutdownCh, shutdownCh)
		go p.taskWorker(shutdownCh)
	}

	p.shutdownWG.Add(1)
	go p.workerMonitor(defaultMonitorTickerDuration)

	p.logger.Info("Parallel task processor started.")
}

func (p *parallelTaskProcessorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(p.shutdownCh)

	p.drainAndNackTasks()

	if success := common.AwaitWaitGroup(&p.shutdownWG, time.Minute); !success {
		p.logger.Warn("Parallel task processor timedout on shutdown.")
	}
	p.logger.Info("Parallel task processor shutdown.")
}

func (p *parallelTaskProcessorImpl) Submit(task Task) error {
	p.metricsScope.IncCounter(metrics.ParallelTaskSubmitRequest)
	sw := p.metricsScope.StartTimer(metrics.ParallelTaskSubmitLatency)
	defer sw.Stop()

	if p.isStopped() {
		return ErrTaskProcessorClosed
	}

	select {
	case p.tasksCh <- task:
		if p.isStopped() {
			p.drainAndNackTasks()
		}
		return nil
	case <-p.shutdownCh:
		return ErrTaskProcessorClosed
	}
}

func (p *parallelTaskProcessorImpl) taskWorker(shutdownCh chan struct{}) {
	defer p.shutdownWG.Done()

	for {
		select {
		case <-shutdownCh:
			return
		case task := <-p.tasksCh:
			p.executeTask(task, shutdownCh)
		}
	}
}

func (p *parallelTaskProcessorImpl) executeTask(task Task, shutdownCh chan struct{}) {
	sw := p.metricsScope.StartTimer(metrics.ParallelTaskTaskProcessingLatency)
	defer sw.Stop()

	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("recovered panic in task execution", tag.Dynamic("recovered-panic", r))
			task.HandleErr(fmt.Errorf("recovered panic: %v", r))
			task.Nack(nil)
		}
	}()

	op := func(ctx context.Context) error {
		if err := task.Execute(); err != nil {
			return task.HandleErr(err)
		}
		return nil
	}

	isRetryable := func(err error) bool {
		select {
		case <-shutdownCh:
			return false
		default:
		}

		return task.RetryErr(err)
	}
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(p.options.RetryPolicy),
		backoff.WithRetryableError(isRetryable),
	)

	if err := throttleRetry.Do(context.Background(), op); err != nil {
		// non-retryable error or exhausted all retries or worker shutdown
		task.Nack(err)
		return
	}

	// no error
	task.Ack()
}

func (p *parallelTaskProcessorImpl) workerMonitor(tickerDuration time.Duration) {
	defer p.shutdownWG.Done()

	ticker := time.NewTicker(tickerDuration)

	for {
		select {
		case <-p.shutdownCh:
			ticker.Stop()
			p.removeWorker(len(p.workerShutdownCh))
			return
		case <-ticker.C:
			targetWorkerCount := p.options.WorkerCount()
			currentWorkerCount := len(p.workerShutdownCh)
			p.addWorker(targetWorkerCount - currentWorkerCount)
			p.removeWorker(currentWorkerCount - targetWorkerCount)
		}
	}
}

func (p *parallelTaskProcessorImpl) addWorker(count int) {
	for i := 0; i < count; i++ {
		shutdownCh := make(chan struct{})
		p.workerShutdownCh = append(p.workerShutdownCh, shutdownCh)

		p.shutdownWG.Add(1)
		go p.taskWorker(shutdownCh)
	}
}

func (p *parallelTaskProcessorImpl) removeWorker(count int) {
	if count <= 0 {
		return
	}

	currentWorkerCount := len(p.workerShutdownCh)
	if count > currentWorkerCount {
		count = currentWorkerCount
	}

	shutdownChToClose := p.workerShutdownCh[currentWorkerCount-count:]
	p.workerShutdownCh = p.workerShutdownCh[:currentWorkerCount-count]

	for _, shutdownCh := range shutdownChToClose {
		close(shutdownCh)
	}
}

func (p *parallelTaskProcessorImpl) drainAndNackTasks() {
	for {
		select {
		case task := <-p.tasksCh:
			task.Nack(nil)
		default:
			return
		}
	}
}

func (p *parallelTaskProcessorImpl) isStopped() bool {
	return atomic.LoadInt32(&p.status) == common.DaemonStatusStopped
}
