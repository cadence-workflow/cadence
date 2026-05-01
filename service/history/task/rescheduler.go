package task

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	ctask "github.com/uber/cadence/common/task"
)

const (
	taskChanFullBackoff                  = 2 * time.Second
	taskChanFullBackoffJitterCoefficient = 0.5

	reschedulerPQCleanupDuration          = 3 * time.Minute
	reschedulerPQCleanupJitterCoefficient = 0.15
)

type (
	rescheduledTask struct {
		task           Task
		rescheduleTime time.Time
	}

	reschedulerImpl struct {
		scheduler    Processor
		timeSource   clock.TimeSource
		logger       log.Logger
		metricsScope metrics.Scope

		status     int32
		ctx        context.Context
		cancel     context.CancelFunc
		shutdownWG sync.WaitGroup

		timerGate clock.TimerGate

		sync.Mutex
		pqMap          map[DomainPriorityKey]collection.Queue[rescheduledTask]
		numExecutables int
	}
)

func NewRescheduler(
	scheduler Processor,
	timeSource clock.TimeSource,
	logger log.Logger,
	metricsScope metrics.Scope,
) Rescheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &reschedulerImpl{
		scheduler:    scheduler,
		timeSource:   timeSource,
		logger:       logger,
		metricsScope: metricsScope,

		status: common.DaemonStatusInitialized,
		ctx:    ctx,
		cancel: cancel,

		timerGate: clock.NewTimerGate(timeSource),

		pqMap: make(map[DomainPriorityKey]collection.Queue[rescheduledTask]),
	}
}

func (r *reschedulerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	r.shutdownWG.Add(1)
	go r.rescheduleLoop()

	r.logger.Info("Task rescheduler started.", tag.LifeCycleStarted)
}

func (r *reschedulerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	r.cancel()
	r.timerGate.Stop()

	if success := common.AwaitWaitGroup(&r.shutdownWG, time.Minute); !success {
		r.logger.Warn("Task rescheduler timedout on shutdown.", tag.LifeCycleStopTimedout)
	}

	r.logger.Info("Task rescheduler stopped.", tag.LifeCycleStopped)
}

func (r *reschedulerImpl) RescheduleTask(
	task Task,
	rescheduleTime time.Time,
) {
	r.Lock()
	pq := r.getOrCreatePQLocked(DomainPriorityKey{
		DomainID: task.GetDomainID(),
		Priority: task.Priority(),
	})
	pq.Add(rescheduledTask{
		task:           task,
		rescheduleTime: rescheduleTime,
	})
	r.numExecutables++
	r.Unlock()
	r.timerGate.Update(rescheduleTime)

	if r.isStopped() {
		r.drain()
	}
}

func (r *reschedulerImpl) RescheduleDomains(
	domainIDs map[string]struct{},
) {
	r.Lock()
	defer r.Unlock()

	now := r.timeSource.Now()
	updatedRescheduleTime := false
	for key, pq := range r.pqMap {
		if _, ok := domainIDs[key.DomainID]; !ok {
			continue
		}

		updatedRescheduleTime = true
		// set reschedule time for all tasks in this pq to be now
		items := make([]rescheduledTask, 0, pq.Len())
		for !pq.IsEmpty() {
			rescheduled, _ := pq.Remove()
			// scheduled queue pre-fetches tasks,
			// so we need to make sure the reschedule time is not before the task scheduled time
			scheduleTime := rescheduled.task.GetTaskKey().GetScheduledTime()
			if now.Before(scheduleTime) {
				rescheduled.rescheduleTime = scheduleTime
			} else {
				rescheduled.rescheduleTime = now
			}
			items = append(items, rescheduled)
		}
		r.pqMap[key] = r.newPriorityQueue(items)
	}

	// then update timer gate to trigger the actual reschedule
	if updatedRescheduleTime {
		r.timerGate.Update(now)
	}
}

func (r *reschedulerImpl) Size() int {
	r.Lock()
	defer r.Unlock()

	return r.numExecutables
}

func (r *reschedulerImpl) rescheduleLoop() {
	defer r.shutdownWG.Done()

	cleanupTimer := r.timeSource.NewTimer(backoff.JitDuration(
		reschedulerPQCleanupDuration,
		reschedulerPQCleanupJitterCoefficient,
	))
	defer cleanupTimer.Stop()

	for {
		select {
		case <-r.ctx.Done():
			r.drain()
			return
		case <-r.timerGate.Chan():
			r.reschedule()
		case <-cleanupTimer.Chan():
			r.cleanupPQ()
			cleanupTimer.Reset(backoff.JitDuration(
				reschedulerPQCleanupDuration,
				reschedulerPQCleanupJitterCoefficient,
			))
		}
	}

}

func (r *reschedulerImpl) reschedule() {
	r.Lock()
	defer r.Unlock()

	r.metricsScope.UpdateGauge(metrics.ReschedulerTaskCountGauge, float64(r.numExecutables))
	now := r.timeSource.Now()
	for _, pq := range r.pqMap {
		for !pq.IsEmpty() {
			rescheduled, _ := pq.Peek()

			if rescheduleTime := rescheduled.rescheduleTime; now.Before(rescheduleTime) {
				r.timerGate.Update(rescheduleTime)
				break
			}

			task := rescheduled.task
			if task.State() != ctask.TaskStatePending {
				pq.Remove()
				r.numExecutables--
				continue
			}

			if task.GetAttempt() == 0 {
				task.SetInitialSubmitTime(now)
			}
			submitted, err := r.scheduler.TrySubmit(task)
			if err != nil {
				if r.isStopped() {
					// if error is due to shard shutdown
					break
				}
				// otherwise it might be error from domain cache etc, add
				// the task to reschedule queue so that it can be retried
				r.logger.Error("Failed to reschedule task", tag.Error(err))
			}
			if !submitted {
				r.timerGate.Update(now.Add(backoff.JitDuration(taskChanFullBackoff, taskChanFullBackoffJitterCoefficient)))
				break
			}

			pq.Remove()
			r.numExecutables--
		}
	}
}

func (r *reschedulerImpl) cleanupPQ() {
	r.Lock()
	defer r.Unlock()

	for key, pq := range r.pqMap {
		if pq.IsEmpty() {
			delete(r.pqMap, key)
		}
	}
}

func (r *reschedulerImpl) drain() {
	r.Lock()
	defer r.Unlock()

	for key, pq := range r.pqMap {
		for !pq.IsEmpty() {
			pq.Remove()
		}
		delete(r.pqMap, key)
	}

	r.numExecutables = 0
}

func (r *reschedulerImpl) isStopped() bool {
	return atomic.LoadInt32(&r.status) == common.DaemonStatusStopped
}

func (r *reschedulerImpl) getOrCreatePQLocked(
	key DomainPriorityKey,
) collection.Queue[rescheduledTask] {
	if pq, ok := r.pqMap[key]; ok {
		return pq
	}

	pq := r.newPriorityQueue(nil)
	r.pqMap[key] = pq
	return pq
}

func (r *reschedulerImpl) newPriorityQueue(
	items []rescheduledTask,
) collection.Queue[rescheduledTask] {
	return collection.NewPriorityQueue(rescheduledTaskCompareLess, items...)
}

func rescheduledTaskCompareLess(
	this rescheduledTask,
	that rescheduledTask,
) bool {
	return this.rescheduleTime.Before(that.rescheduleTime)
}
