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
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

const (
	redispatchBackoffCoefficient     = 1.05
	redispatchMaxBackoffInternval    = 2 * time.Minute
	redispatchFailureBackoffInterval = 2 * time.Second
)

type (
	// RedispatcherOptions configs redispatch interval
	RedispatcherOptions struct {
		TaskRedispatchInterval dynamicconfig.DurationPropertyFn
	}

	redispatcherImpl struct {
		sync.Mutex

		taskProcessor Processor
		timeSource    clock.TimeSource
		logger        log.Logger
		metricsScope  metrics.Scope

		status        int32
		shutdownCh    chan struct{}
		shutdownWG    sync.WaitGroup
		timerGate     clock.TimerGate
		backoffPolicy backoff.RetryPolicy
		pq            collection.Queue[redispatchTask]
	}

	redispatchTask struct {
		task           Task
		redispatchTime time.Time
	}
)

// NewRedispatcher creates a new task Redispatcher
func NewRedispatcher(
	taskProcessor Processor,
	timeSource clock.TimeSource,
	options *RedispatcherOptions,
	logger log.Logger,
	metricsScope metrics.Scope,
) Redispatcher {
	backoffPolicy := backoff.NewExponentialRetryPolicy(options.TaskRedispatchInterval())
	backoffPolicy.SetBackoffCoefficient(redispatchBackoffCoefficient)
	backoffPolicy.SetMaximumInterval(redispatchMaxBackoffInternval)
	backoffPolicy.SetExpirationInterval(backoff.NoInterval)

	return &redispatcherImpl{
		taskProcessor: taskProcessor,
		timeSource:    timeSource,
		logger:        logger,
		metricsScope:  metricsScope,
		status:        common.DaemonStatusInitialized,
		shutdownCh:    make(chan struct{}),
		timerGate:     clock.NewTimerGate(timeSource),
		backoffPolicy: backoffPolicy,
		pq:            collection.NewPriorityQueue(redispatchTaskCompareLess),
	}
}

func (r *redispatcherImpl) Start() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	r.shutdownWG.Add(1)
	go r.redispatchLoop()

	r.logger.Info("Task redispatcher started.", tag.LifeCycleStarted)
}

func (r *redispatcherImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(r.shutdownCh)
	r.timerGate.Stop()
	if success := common.AwaitWaitGroup(&r.shutdownWG, time.Minute); !success {
		r.logger.Warn("Task redispatcher timedout on shutdown.", tag.LifeCycleStopTimedout)
	}

	r.logger.Info("Task redispatcher stopped.", tag.LifeCycleStopped)
}

func (r *redispatcherImpl) AddTask(task Task) {
	attempt := task.GetAttempt()

	r.Lock()
	t := r.getRedispatchTime(attempt)
	r.pq.Add(redispatchTask{
		task:           task,
		redispatchTime: t,
	})
	r.Unlock()

	r.timerGate.Update(t)
}

// TODO: this method is removed from Temporal's codebase in this PR: https://github.com/temporalio/temporal/pull/3038
// Let's review if this method is needed in our codebase later.
func (r *redispatcherImpl) Redispatch(targetSize int) {
	r.Lock()
	defer r.Unlock()

	queueSize := r.sizeLocked()
	r.metricsScope.RecordTimer(metrics.TaskRedispatchQueuePendingTasksTimer, time.Duration(queueSize))

	now := r.timeSource.Now()
	totalRedispatched := 0
	var failToSubmit []redispatchTask
	for !r.pq.IsEmpty() && totalRedispatched < targetSize {
		item, _ := r.pq.Peek() // error is impossible because we've checked that the queue is not empty
		if item.redispatchTime.After(now) {
			break
		}
		submitted, err := r.taskProcessor.TrySubmit(item.task)
		if err != nil {
			if r.isStopped() {
				// if error is due to shard shutdown
				break
			}
			// otherwise it might be error from domain cache etc, add
			// the task to redispatch queue so that it can be retried
			r.logger.Error("Failed to redispatch task", tag.Error(err))
		}
		r.pq.Remove()
		if err != nil || !submitted {
			failToSubmit = append(failToSubmit, item)
		} else {
			totalRedispatched++
		}
	}

	for _, item := range failToSubmit {
		r.pq.Add(item)
	}
}

func (r *redispatcherImpl) Size() int {
	r.Lock()
	defer r.Unlock()

	return r.sizeLocked()
}

func (r *redispatcherImpl) redispatchLoop() {
	defer r.shutdownWG.Done()

	for {
		select {
		case <-r.shutdownCh:
			return
		case <-r.timerGate.Chan():
			r.redispatchTasks()
		}
	}
}

func (r *redispatcherImpl) redispatchTasks() {
	r.Lock()
	defer r.Unlock()

	if r.isStopped() {
		return
	}

	queueSize := r.sizeLocked()
	r.metricsScope.RecordTimer(metrics.TaskRedispatchQueuePendingTasksTimer, time.Duration(queueSize))

	now := r.timeSource.Now()
	for !r.pq.IsEmpty() {
		item, _ := r.pq.Peek() // error is impossible because we've checked that the queue is not empty
		if item.redispatchTime.After(now) {
			break
		}
		submitted, err := r.taskProcessor.TrySubmit(item.task)
		if err != nil {
			if r.isStopped() {
				// if error is due to shard shutdown
				break
			}
			// otherwise it might be error from domain cache etc, add
			// the task to redispatch queue so that it can be retried
			r.logger.Error("Failed to redispatch task", tag.Error(err))
		}
		r.pq.Remove()
		if err != nil || !submitted {
			// failed to submit, enqueue again
			item.redispatchTime = r.timeSource.Now().Add(redispatchFailureBackoffInterval)
			r.pq.Add(item)
		}
	}
	if !r.pq.IsEmpty() {
		item, _ := r.pq.Peek()
		r.timerGate.Update(item.redispatchTime)
	}
}

func (r *redispatcherImpl) sizeLocked() int {
	return r.pq.Len()
}

func (r *redispatcherImpl) isStopped() bool {
	return atomic.LoadInt32(&r.status) == common.DaemonStatusStopped
}

func (r *redispatcherImpl) getRedispatchTime(attempt int) time.Time {
	// note that elapsedTime (the first parameter) is not relevant when
	// the retry policy has not expiration intervaly(0, attempt)))
	return r.timeSource.Now().Add(r.backoffPolicy.ComputeNextDelay(0, attempt))
}

func redispatchTaskCompareLess(
	this redispatchTask,
	that redispatchTask,
) bool {
	return this.redispatchTime.Before(that.redispatchTime)
}
