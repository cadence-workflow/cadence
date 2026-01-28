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

//go:generate mockgen -package $GOPACKAGE -destination virtual_queue_mock.go github.com/uber/cadence/service/history/queuev2 VirtualQueue
package queuev2

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/service/history/task"
)

var (
	taskSchedulerThrottleBackoffInterval = time.Second * 5
	taskReaderErrorBackoffInterval       = time.Second
)

type (
	VirtualQueue interface {
		common.Daemon
		// GetState return the current state of the virtual queue
		GetState() []VirtualSliceState
		// UpdateAndGetState update the state of the virtual queue and return the current state
		UpdateAndGetState() []VirtualSliceState
		// MergeSlices merge the incoming slices into the virtual queue, this is used when we want to merge slices to a non-root virtual queue
		MergeSlices(...VirtualSlice)
		// MergeWithLastSlice merge the incoming slice with the last slice in the virtual queue, this is used when we want to add a new slice to the root virtual queue to avoid nullify the effect of AppendSlices
		MergeWithLastSlice(VirtualSlice)
		// AppendSlices append the incoming slices to the virtual queue, this is used when we want to add a new slice to the root virtual queue to prevent infinite growth of the virtual slice
		AppendSlices(...VirtualSlice)
		// IterateSlices iterate over the slices in the virtual queue
		IterateSlices(func(VirtualSlice))
		// ClearSlices calls the Clear method of the slices that satisfy the predicate function
		ClearSlices(func(VirtualSlice) bool)
		// SplitSlices applies the split function to the slices in the virtual queue and return the remaining slices that should be kept in the virtual queue and whether the split is applied
		SplitSlices(func(VirtualSlice) (remaining []VirtualSlice, split bool))
		// Pause pauses the virtual queue for a while
		Pause(time.Duration)
		// InsertSingleTask inserts a single task to the virtual queue. Return false if the task's timestamp is out of range of the current queue slice or the task does not satisfy the predicate
		InsertSingleTask(task task.Task) bool
		// ResetProgress removes the scheduled tasks after the given time
		ResetProgress(persistence.HistoryTaskKey)
	}

	VirtualQueueOptions struct {
		PageSize                             dynamicproperties.IntPropertyFn
		MaxPendingTasksCount                 dynamicproperties.IntPropertyFn
		PollBackoffInterval                  dynamicproperties.DurationPropertyFn
		PollBackoffIntervalJitterCoefficient dynamicproperties.FloatPropertyFn
	}

	virtualQueueImpl struct {
		queueOptions        *VirtualQueueOptions
		processor           task.Processor
		rescheduler         task.Rescheduler
		logger              log.Logger
		metricsScope        metrics.Scope
		timeSource          clock.TimeSource
		taskLoadRateLimiter quotas.Limiter
		monitor             Monitor

		sync.RWMutex
		status          int32
		wg              sync.WaitGroup
		ctx             context.Context
		cancel          func()
		notifyCh        chan struct{}
		pauseController PauseController
		virtualSlices   *list.List
		sliceToRead     *list.Element
	}
)

func NewVirtualQueue(
	processor task.Processor,
	rescheduler task.Rescheduler,
	logger log.Logger,
	metricsScope metrics.Scope,
	timeSource clock.TimeSource,
	taskLoadRateLimiter quotas.Limiter,
	monitor Monitor,
	virtualSlices []VirtualSlice,
	queueOptions *VirtualQueueOptions,
) VirtualQueue {
	ctx, cancel := context.WithCancel(context.Background())

	sliceList := list.New()
	for _, slice := range virtualSlices {
		sliceList.PushBack(slice)
	}

	return &virtualQueueImpl{
		queueOptions:        queueOptions,
		processor:           processor,
		rescheduler:         rescheduler,
		logger:              logger,
		metricsScope:        metricsScope,
		timeSource:          timeSource,
		taskLoadRateLimiter: taskLoadRateLimiter,
		monitor:             monitor,

		status:          common.DaemonStatusInitialized,
		ctx:             ctx,
		cancel:          cancel,
		notifyCh:        make(chan struct{}, 1),
		pauseController: NewPauseController(timeSource),
		virtualSlices:   sliceList,
		sliceToRead:     sliceList.Front(),
	}
}

func (q *virtualQueueImpl) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	q.pauseController.Subscribe("virtual-queue", q.notifyCh)
	q.wg.Add(1)
	go q.run()

	q.notify()

	q.logger.Info("Virtual queue state changed", tag.LifeCycleStarted)
}

func (q *virtualQueueImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	q.pauseController.Unsubscribe("virtual-queue")
	q.pauseController.Stop()

	q.cancel()
	q.wg.Wait()

	q.RLock()
	defer q.RUnlock()
	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		slice := e.Value.(VirtualSlice)
		slice.Clear()
	}

	q.logger.Info("Virtual queue state changed", tag.LifeCycleStopped)
}

func (q *virtualQueueImpl) GetState() []VirtualSliceState {
	q.RLock()
	defer q.RUnlock()

	states := make([]VirtualSliceState, 0, q.virtualSlices.Len())
	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		states = append(states, e.Value.(VirtualSlice).GetState())
	}
	return states
}

func (q *virtualQueueImpl) UpdateAndGetState() []VirtualSliceState {
	q.Lock()
	defer q.Unlock()

	states := make([]VirtualSliceState, 0, q.virtualSlices.Len())
	var next *list.Element
	for e := q.virtualSlices.Front(); e != nil; e = next {
		next = e.Next()
		slice := e.Value.(VirtualSlice)
		state := slice.UpdateAndGetState()
		if slice.IsEmpty() {
			q.virtualSlices.Remove(e)
			q.monitor.RemoveSlice(slice)
		} else {
			states = append(states, state)
			q.monitor.SetSlicePendingTaskCount(slice, slice.GetPendingTaskCount())
		}
	}
	return states
}

func (q *virtualQueueImpl) MergeSlices(incomingSlices ...VirtualSlice) {
	if len(incomingSlices) == 0 {
		return
	}

	q.Lock()
	defer q.Unlock()

	mergedSlices := list.New()

	currentSliceElement := q.virtualSlices.Front()
	incomingSliceIdx := 0

	for currentSliceElement != nil && incomingSliceIdx < len(incomingSlices) {
		currentSlice := currentSliceElement.Value.(VirtualSlice)
		incomingSlice := incomingSlices[incomingSliceIdx]

		if currentSlice.GetState().Range.InclusiveMinTaskKey.Compare(incomingSlice.GetState().Range.InclusiveMinTaskKey) < 0 {
			q.appendOrMergeSlice(mergedSlices, currentSlice)
			currentSliceElement = currentSliceElement.Next()
		} else {
			q.appendOrMergeSlice(mergedSlices, incomingSlice)
			incomingSliceIdx++
		}
	}
	for ; currentSliceElement != nil; currentSliceElement = currentSliceElement.Next() {
		q.appendOrMergeSlice(mergedSlices, currentSliceElement.Value.(VirtualSlice))
	}
	for _, slice := range incomingSlices[incomingSliceIdx:] {
		q.appendOrMergeSlice(mergedSlices, slice)
	}

	q.virtualSlices.Init()
	q.virtualSlices = mergedSlices
	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) MergeWithLastSlice(incomingSlice VirtualSlice) {
	q.Lock()
	defer q.Unlock()

	q.appendOrMergeSlice(q.virtualSlices, incomingSlice)
	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) AppendSlices(incomingSlices ...VirtualSlice) {
	if len(incomingSlices) == 0 {
		return
	}

	q.Lock()
	defer q.Unlock()

	for _, slice := range incomingSlices {
		q.virtualSlices.PushBack(slice)
	}

	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) IterateSlices(f func(VirtualSlice)) {
	q.RLock()
	defer q.RUnlock()

	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		f(e.Value.(VirtualSlice))
	}
}

func (q *virtualQueueImpl) ClearSlices(f func(VirtualSlice) bool) {
	q.Lock()
	defer q.Unlock()

	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		slice := e.Value.(VirtualSlice)
		if f(slice) {
			slice.Clear()
			q.monitor.SetSlicePendingTaskCount(slice, slice.GetPendingTaskCount())
		}
	}

	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) SplitSlices(f func(VirtualSlice) (remaining []VirtualSlice, split bool)) {
	q.Lock()
	defer q.Unlock()

	remainingSlices := list.New()
	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		slice := e.Value.(VirtualSlice)
		remaining, split := f(slice)
		if !split {
			remainingSlices.PushBack(slice)
			continue
		}

		q.monitor.RemoveSlice(slice)

		for _, remainingSlice := range remaining {
			remainingSlices.PushBack(remainingSlice)
			q.monitor.SetSlicePendingTaskCount(remainingSlice, remainingSlice.GetPendingTaskCount())
		}
	}

	q.virtualSlices.Init()
	q.virtualSlices = remainingSlices
	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) Pause(duration time.Duration) {
	q.pauseController.Pause(duration)
}

func (q *virtualQueueImpl) notify() {
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

func (q *virtualQueueImpl) run() {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.notifyCh:
			q.loadAndSubmitTasks()
		}
	}
}

func (q *virtualQueueImpl) loadAndSubmitTasks() {
	if err := q.taskLoadRateLimiter.Wait(q.ctx); err != nil {
		if q.ctx.Err() != nil {
			return
		}
		// this should never happen, but we log it for debugging purposes
		q.logger.Error("Virtual queue failed to wait for rate limiter", tag.Error(err))
	}

	q.Lock()
	defer q.Unlock()

	if q.sliceToRead == nil {
		return
	}

	pendingTaskCount := q.monitor.GetTotalPendingTaskCount()
	maxTaskCount := q.queueOptions.MaxPendingTasksCount()
	// TODO: review the metrics and remove this comment or change the metric from gauge to histogram
	q.metricsScope.UpdateGauge(metrics.PendingTaskGauge, float64(pendingTaskCount))
	if pendingTaskCount >= maxTaskCount {
		q.logger.Warn("Too many pending tasks, pause loading tasks for a while", tag.PendingTaskCount(pendingTaskCount), tag.MaxTaskCount(maxTaskCount))
		q.pauseController.Pause(q.queueOptions.PollBackoffInterval())
	}

	if q.pauseController.IsPaused() {
		// emit a metric indicating that the virtual queue is paused
		q.metricsScope.UpdateGauge(metrics.VirtualQueuePausedGauge, 1.0)
		q.logger.Debug("virtual queue is paused", tag.PendingTaskCount(pendingTaskCount), tag.MaxTaskCount(maxTaskCount))
		return
	}

	// emit a metric indicating that the virtual queue is alive
	q.metricsScope.UpdateGauge(metrics.VirtualQueueRunningGauge, 1.0)
	sliceToRead := q.sliceToRead.Value.(VirtualSlice)

	// This logic is to avoid the loop of loading tasks from max virtual queue -> pending task count exceeds critical task count -> unload tasks from max virtual queue
	// for non-root virtual queue, we know that maxTaskCount < criticalTaskCount
	remainingSize := maxTaskCount - pendingTaskCount
	if remainingSize <= 0 {
		remainingSize = 1
		q.logger.Error("unexpected error, virtual queue is not paused when pending task count exceeds max task count limit", tag.PendingTaskCount(pendingTaskCount), tag.MaxTaskCount(maxTaskCount))
	}
	pageSize := min(q.queueOptions.PageSize(), remainingSize)
	q.logger.Debug("getting tasks from virtual queue", tag.PendingTaskCount(pendingTaskCount), tag.MaxTaskCount(maxTaskCount), tag.Counter(pageSize))
	tasks, err := sliceToRead.GetTasks(q.ctx, pageSize)
	if err != nil {
		q.logger.Error("Virtual queue failed to get tasks", tag.Error(err))
		q.pauseController.Pause(taskReaderErrorBackoffInterval)
		return
	}
	q.logger.Debug("got tasks from virtual queue", tag.Counter(len(tasks)))

	q.monitor.SetSlicePendingTaskCount(sliceToRead, sliceToRead.GetPendingTaskCount())

	now := q.timeSource.Now()
	for _, task := range tasks {
		if err := q.submitOrRescheduleTask(now, task); err != nil {
			select {
			case <-q.ctx.Done():
				return
			default:
				q.logger.Error("Virtual queue failed to submit task", tag.Error(err))
			}
		}
	}

	if sliceToRead.HasMoreTasks() {
		q.notify()
		return
	}

	q.sliceToRead = q.sliceToRead.Next()
	if q.sliceToRead != nil {
		q.notify()
	}
}

func (q *virtualQueueImpl) InsertSingleTask(task task.Task) bool {
	q.Lock()
	defer q.Unlock()

	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		slice := e.Value.(VirtualSlice)
		inserted := slice.InsertTask(task)
		if inserted {
			q.monitor.SetSlicePendingTaskCount(slice, slice.GetPendingTaskCount())
			now := q.timeSource.Now()
			if err := q.submitOrRescheduleTask(now, task); err != nil {
				q.logger.Error("Error submitting task to virtual queue", tag.Error(err))
			}

			return true
		}
	}

	return false
}

func (q *virtualQueueImpl) ResetProgress(key persistence.HistoryTaskKey) {
	q.Lock()
	defer q.Unlock()

	for e := q.virtualSlices.Front(); e != nil; e = e.Next() {
		slice := e.Value.(VirtualSlice)
		slice.ResetProgress(key)
		q.monitor.SetSlicePendingTaskCount(slice, slice.GetPendingTaskCount())

		if e == q.sliceToRead {
			break
		}
	}

	q.resetNextReadSliceLocked()
}

func (q *virtualQueueImpl) submitOrRescheduleTask(now time.Time, task task.Task) error {
	if persistence.IsTaskCorrupted(task) {
		q.logger.Error("Virtual queue encountered a corrupted task", tag.Dynamic("task", task))
		q.metricsScope.IncCounter(metrics.CorruptedHistoryTaskCounter)

		task.Ack()
		return nil
	}

	scheduledTime := task.GetTaskKey().GetScheduledTime()
	// if the scheduled time is in the future, we need to reschedule the task
	if now.Before(scheduledTime) {
		q.rescheduler.RescheduleTask(task, scheduledTime)
		return nil
	}
	// shard level metrics for the duration between a task being written to a queue and being fetched from it
	q.metricsScope.RecordHistogramDuration(metrics.TaskEnqueueToFetchLatency, now.Sub(task.GetVisibilityTimestamp()))
	task.SetInitialSubmitTime(now)
	submitted, err := q.processor.TrySubmit(task)

	if !submitted {
		q.metricsScope.IncCounter(metrics.ProcessingQueueThrottledCounter)
		q.rescheduler.RescheduleTask(task, q.timeSource.Now().Add(taskSchedulerThrottleBackoffInterval))
	}

	return err
}

func (q *virtualQueueImpl) resetNextReadSliceLocked() {
	q.sliceToRead = nil
	for element := q.virtualSlices.Front(); element != nil; element = element.Next() {
		if element.Value.(VirtualSlice).HasMoreTasks() {
			q.sliceToRead = element
			break
		}
	}

	if q.sliceToRead != nil {
		q.notify()
	}
}

func (q *virtualQueueImpl) appendOrMergeSlice(slices *list.List, incomingSlice VirtualSlice) {
	if slices.Len() == 0 {
		slices.PushBack(incomingSlice)
		q.monitor.SetSlicePendingTaskCount(incomingSlice, incomingSlice.GetPendingTaskCount())
		return
	}

	lastElement := slices.Back()
	lastSlice := lastElement.Value.(VirtualSlice)
	mergedSlices, merged := lastSlice.TryMergeWithVirtualSlice(incomingSlice)
	if !merged {
		slices.PushBack(incomingSlice)
		q.monitor.SetSlicePendingTaskCount(incomingSlice, incomingSlice.GetPendingTaskCount())
		return
	}

	slices.Remove(lastElement)
	q.monitor.RemoveSlice(lastSlice)
	q.monitor.RemoveSlice(incomingSlice) // incomingSlice may already be tracked by the monitor, so we need to remove it if it's tracked
	for _, mergedSlice := range mergedSlices {
		slices.PushBack(mergedSlice)
		q.monitor.SetSlicePendingTaskCount(mergedSlice, mergedSlice.GetPendingTaskCount())
	}
}
