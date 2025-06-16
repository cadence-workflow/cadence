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
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

const (
	defaultBufferSize = 200

	redispatchBackoffCoefficient     = 1.05
	redispatchMaxBackoffInternval    = 2 * time.Minute
	redispatchFailureBackoffInterval = 2 * time.Second
)

type (
	redispatchNotification struct {
		targetSize int
		doneCh     chan struct{}
	}

	// RedispatcherOptions configs redispatch interval
	RedispatcherOptions struct {
		TaskRedispatchInterval dynamicproperties.DurationPropertyFn
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
		redispatchCh  chan redispatchNotification
		timerGate     clock.TimerGate
		backoffPolicy backoff.RetryPolicy
		pqMap         map[int]collection.Queue[redispatchTask] // priority -> redispatch queue
		taskChFull    map[int]bool                             // priority -> if taskCh is full
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
		redispatchCh:  make(chan redispatchNotification, 1),
		timerGate:     clock.NewTimerGate(timeSource),
		backoffPolicy: backoffPolicy,
		pqMap:         make(map[int]collection.Queue[redispatchTask]),
		taskChFull:    make(map[int]bool),
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
	priority := task.Priority()
	attempt := task.GetAttempt()

	r.Lock()
	pq := r.getOrCreatePQLocked(priority)
	t := r.getRedispatchTime(attempt)
	pq.Add(redispatchTask{
		task:           task,
		redispatchTime: t,
	})
	r.Unlock()

	r.timerGate.Update(t)
}

func (r *redispatcherImpl) RedispatchTask(task Task, t time.Time) {
	priority := task.Priority()

	r.Lock()
	pq := r.getOrCreatePQLocked(priority)
	pq.Add(redispatchTask{
		task:           task,
		redispatchTime: t,
	})
	r.Unlock()

	r.timerGate.Update(t)
}

// TODO: review this method, it doesn't seem to redispatch the tasks immediately
func (r *redispatcherImpl) Redispatch(targetSize int) {
	doneCh := make(chan struct{})
	ntf := redispatchNotification{
		targetSize: targetSize,
		doneCh:     doneCh,
	}

	select {
	case r.redispatchCh <- ntf:
		// block until the redispatch is done
		<-doneCh
	case <-r.shutdownCh:
		close(doneCh)
		return
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
			r.redispatchTasks(redispatchNotification{})
		case notification := <-r.redispatchCh:
			r.redispatchTasks(notification)
		}
	}
}

func (r *redispatcherImpl) redispatchTasks(notification redispatchNotification) {
	r.Lock()
	defer r.Unlock()

	defer func() {
		if notification.doneCh != nil {
			close(notification.doneCh)
		}
	}()

	if r.isStopped() {
		return
	}

	queueSize := r.sizeLocked()
	r.metricsScope.RecordTimer(metrics.TaskRedispatchQueuePendingTasksTimer, time.Duration(queueSize))

	// add some buffer here as new tasks may be added
	targetRedispatched := queueSize + defaultBufferSize - notification.targetSize
	if targetRedispatched <= 0 {
		// target size has already been met, no need to redispatch
		return
	}

	totalRedispatched := 0
	now := r.timeSource.Now()
	for priority := range r.pqMap {
		r.taskChFull[priority] = false
	}

	for priority, pq := range r.pqMap {
		// Note the third condition regarding taskChFull is not 100% accurate
		// since task may get a new, lower priority upon redispatch, and
		// the taskCh for the new priority may not be full.
		// But the current estimation should be good enough as task with
		// lower priority should be executed after high priority ones,
		// so it's ok to leave them in the queue
		for !pq.IsEmpty() && totalRedispatched < targetRedispatched && !r.taskChFull[priority] {
			item, _ := pq.Peek() // error is impossible because we've checked that the queue is not empty
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
			pq.Remove()
			newPriority := item.task.Priority()
			if err != nil || !submitted {
				// failed to submit, enqueue again
				item.redispatchTime = r.timeSource.Now().Add(redispatchFailureBackoffInterval)
				r.getOrCreatePQLocked(newPriority).Add(item)
			}
			if err == nil && !submitted {
				// task chan is full for the new priority
				r.taskChFull[newPriority] = true
			}
			if submitted {
				totalRedispatched++
			}
		}
		if !pq.IsEmpty() {
			item, _ := pq.Peek()
			r.timerGate.Update(item.redispatchTime)
		}
		if r.isStopped() {
			return
		}
	}
}

func (r *redispatcherImpl) sizeLocked() int {
	size := 0
	for _, queue := range r.pqMap {
		size += queue.Len()
	}
	return size
}

func (r *redispatcherImpl) isStopped() bool {
	return atomic.LoadInt32(&r.status) == common.DaemonStatusStopped
}

func (r *redispatcherImpl) getRedispatchTime(attempt int) time.Time {
	// note that elapsedTime (the first parameter) is not relevant when
	// the retry policy has not expiration intervaly(0, attempt)))
	return r.timeSource.Now().Add(r.backoffPolicy.ComputeNextDelay(0, attempt))
}

func (r *redispatcherImpl) getOrCreatePQLocked(priority int) collection.Queue[redispatchTask] {
	if pq, ok := r.pqMap[priority]; ok {
		return pq
	}

	pq := collection.NewPriorityQueue(redispatchTaskCompareLess)
	r.pqMap[priority] = pq
	return pq
}

func redispatchTaskCompareLess(
	this redispatchTask,
	that redispatchTask,
) bool {
	return this.redispatchTime.Before(that.redispatchTime)
}
