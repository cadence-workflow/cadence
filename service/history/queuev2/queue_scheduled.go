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
	scheduledQueue struct {
		base *queueBase

		timerGate    clock.TimerGate
		newTimerCh   chan struct{}
		nextTimeLock sync.RWMutex
		nextTime     time.Time

		status     int32
		shutdownWG sync.WaitGroup
		ctx        context.Context
		cancel     func()
	}
)

func NewScheduledQueue(
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
	return &scheduledQueue{
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
		timerGate:  clock.NewTimerGate(shard.GetTimeSource()),
		newTimerCh: make(chan struct{}, 1),
		ctx:        ctx,
		cancel:     cancel,
		status:     common.DaemonStatusInitialized,
	}
}

func (q *scheduledQueue) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	q.base.logger.Info("History queue state changed", tag.LifeCycleStarting)
	defer q.base.logger.Info("History queue state changed", tag.LifeCycleStarted)

	q.base.Start()

	q.shutdownWG.Add(1)
	go q.processEventLoop()

	q.notify(time.Time{})
}

func (q *scheduledQueue) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	q.base.logger.Info("History queue state changed", tag.LifeCycleStopping)
	defer q.base.logger.Info("History queue state changed", tag.LifeCycleStopped)

	q.cancel()
	q.timerGate.Stop()
	q.shutdownWG.Wait()
	q.base.Stop()
}

func (q *scheduledQueue) Category() persistence.HistoryTaskCategory {
	return q.base.Category()
}

func (q *scheduledQueue) FailoverDomain(domainIDs map[string]struct{}) {
	q.base.FailoverDomain(domainIDs)
}

func (q *scheduledQueue) HandleAction(ctx context.Context, clusterName string, action *queue.Action) (*queue.ActionResult, error) {
	return q.base.HandleAction(ctx, clusterName, action)
}

func (q *scheduledQueue) LockTaskProcessing() {
	q.base.LockTaskProcessing()
}

func (q *scheduledQueue) UnlockTaskProcessing() {
	q.base.UnlockTaskProcessing()
}

func (q *scheduledQueue) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	numTasks := len(info.Tasks)
	if numTasks == 0 {
		return
	}

	q.base.logger.Debug(
		"New timer task notification received",
		tag.Dynamic("numTasks", numTasks),
		tag.Dynamic("scheduleInMemory", info.ScheduleInMemory),
		tag.Dynamic("persistenceError", info.PersistenceError),
		tag.Dynamic("shardId", q.base.shard.GetShardID()),
	)

	tasksToBeReadFromDB := make([]persistence.Task, 0)

	if info.ScheduleInMemory && !info.PersistenceError {
		for _, task := range info.Tasks {
			ts := task.GetVisibilityTimestamp()
			q.base.logger.Debug("Submitting task to an in-memory queue", tag.Dynamic("scheduledTime", ts), tag.Dynamic("shardId", q.base.shard.GetShardID()))

			if !q.base.insertSingleTask(q.base.taskInitializer(task)) {
				tasksToBeReadFromDB = append(tasksToBeReadFromDB, task)
			}
		}
	} else {
		tasksToBeReadFromDB = info.Tasks
	}

	var nextReadTime time.Time
	for _, task := range tasksToBeReadFromDB {
		ts := task.GetVisibilityTimestamp()
		if nextReadTime.IsZero() || ts.Before(nextReadTime) {
			nextReadTime = ts
		}
	}

	if !nextReadTime.IsZero() {
		q.base.resetProgress(persistence.NewHistoryTaskKey(nextReadTime, 0))
		q.notify(nextReadTime)
	}

	q.base.metricsScope.AddCounter(metrics.NewHistoryTaskCounter, int64(numTasks))
}

func (q *scheduledQueue) notify(t time.Time) {
	q.nextTimeLock.Lock()
	defer q.nextTimeLock.Unlock()

	if q.nextTime.IsZero() || t.Before(q.nextTime) {
		q.nextTime = t
		select {
		case q.newTimerCh <- struct{}{}:
		default:
		}
	}
}

func (q *scheduledQueue) processNextTime() {
	q.nextTimeLock.Lock()
	nextTime := q.nextTime
	q.nextTime = time.Time{}
	q.nextTimeLock.Unlock()

	q.timerGate.Update(nextTime)
}

func (q *scheduledQueue) processEventLoop() {
	defer q.shutdownWG.Done()

	for {
		select {
		case <-q.newTimerCh:
			q.processNextTime()
		case <-q.timerGate.Chan():
			q.base.processNewTasks()
			q.lookAheadTask()
		case <-q.base.updateQueueStateTimer.Chan():
			q.base.updateQueueState(q.ctx)
		case alert := <-q.base.alertCh:
			q.base.handleAlert(q.ctx, alert)
		case <-q.ctx.Done():
			return
		}
	}
}

func (q *scheduledQueue) lookAheadTask() {
	lookAheadMinTime := q.base.newVirtualSliceState.Range.InclusiveMinTaskKey.GetScheduledTime()
	lookAheadMaxTime := lookAheadMinTime.Add(backoff.JitDuration(
		q.base.options.MaxPollInterval(),
		q.base.options.MaxPollIntervalJitterCoefficient(),
	))

	resp, err := q.base.queueReader.GetTask(q.ctx, &GetTaskRequest{
		Progress: &GetTaskProgress{
			Range: Range{
				InclusiveMinTaskKey: persistence.NewHistoryTaskKey(lookAheadMinTime, 0),
				ExclusiveMaxTaskKey: persistence.NewHistoryTaskKey(lookAheadMaxTime, 0),
			},
			NextTaskKey: persistence.NewHistoryTaskKey(lookAheadMaxTime, 0),
		},
		Predicate: NewUniversalPredicate(),
		PageSize:  1,
	})
	if err != nil {
		q.timerGate.Update(lookAheadMinTime)
		q.base.logger.Error("Failed to look ahead task", tag.Error(err))
		return
	}

	if len(resp.Tasks) == 0 {
		q.timerGate.Update(lookAheadMaxTime)
		return
	}

	q.timerGate.Update(resp.Tasks[0].GetVisibilityTimestamp())
}
