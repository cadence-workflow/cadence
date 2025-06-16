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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/history/task"
)

type (
	VirtualQueue interface {
		common.Daemon
		GetState() []VirtualSliceState
		UpdateAndGetState() []VirtualSliceState
		MergeSlices(...VirtualSlice)
	}

	VirtualQueueOptions struct {
		PageSize dynamicproperties.IntPropertyFn
	}

	virtualQueueImpl struct {
		options      *VirtualQueueOptions
		processor    task.Processor
		redispatcher task.Redispatcher
		logger       log.Logger
		metricsScope metrics.Scope
		timeSource   clock.TimeSource

		sync.RWMutex
		status        int32
		wg            sync.WaitGroup
		ctx           context.Context
		cancel        func()
		notifyCh      chan struct{}
		virtualSlices *list.List
		sliceToRead   *list.Element
	}
)

func NewVirtualQueue(
	processor task.Processor,
	redispatcher task.Redispatcher,
	logger log.Logger,
	metricsScope metrics.Scope,
	timeSource clock.TimeSource,
	virtualSlices []VirtualSlice,
	options *VirtualQueueOptions,
) VirtualQueue {
	ctx, cancel := context.WithCancel(context.Background())

	sliceList := list.New()
	for _, slice := range virtualSlices {
		sliceList.PushBack(slice)
	}

	return &virtualQueueImpl{
		options:      options,
		processor:    processor,
		redispatcher: redispatcher,
		logger:       logger,
		metricsScope: metricsScope,
		timeSource:   timeSource,

		status:        common.DaemonStatusInitialized,
		ctx:           ctx,
		cancel:        cancel,
		notifyCh:      make(chan struct{}, 1),
		virtualSlices: sliceList,
		sliceToRead:   sliceList.Front(),
	}
}

func (q *virtualQueueImpl) Start() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	q.wg.Add(1)
	go q.run()

	q.notify()

	q.logger.Info("Virtual queue state changed", tag.LifeCycleStarted)
}

func (q *virtualQueueImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&q.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	q.cancel()
	q.wg.Wait()

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
		if state.IsEmpty() {
			q.virtualSlices.Remove(e)
		} else {
			states = append(states, state)
		}
	}
	return states
}

func (q *virtualQueueImpl) MergeSlices(incomingSlices ...VirtualSlice) {
	q.Lock()
	defer q.Unlock()

	mergedSlices := list.New()

	currentSliceElement := q.virtualSlices.Front()
	incomingSliceIdx := 0

	for currentSliceElement != nil && incomingSliceIdx < len(incomingSlices) {
		currentSlice := currentSliceElement.Value.(VirtualSlice)
		incomingSlice := incomingSlices[incomingSliceIdx]

		if currentSlice.GetState().Range.InclusiveMinTaskKey.Compare(incomingSlice.GetState().Range.InclusiveMinTaskKey) < 0 {
			appendOrMergeSlice(mergedSlices, currentSlice)
			currentSliceElement = currentSliceElement.Next()
		} else {
			appendOrMergeSlice(mergedSlices, incomingSlice)
			incomingSliceIdx++
		}
	}
	for ; currentSliceElement != nil; currentSliceElement = currentSliceElement.Next() {
		appendOrMergeSlice(mergedSlices, currentSliceElement.Value.(VirtualSlice))
	}
	for _, slice := range incomingSlices[incomingSliceIdx:] {
		appendOrMergeSlice(mergedSlices, slice)
	}

	q.virtualSlices.Init()
	q.virtualSlices = mergedSlices
	q.resetNextReadSliceLocked()
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
	q.RLock()
	defer q.RUnlock()

	if q.sliceToRead == nil {
		return
	}

	// TODO: do not load task if there are too many pending tasks

	sliceToRead := q.sliceToRead.Value.(VirtualSlice)
	tasks, err := sliceToRead.GetTasks(q.ctx, q.options.PageSize())
	if err != nil {
		q.logger.Error("Virtual queue failed to get tasks", tag.Error(err))
		return
	}

	now := q.timeSource.Now()
	for _, task := range tasks {
		scheduledTime := task.GetTaskKey().GetScheduledTime()
		if now.Before(scheduledTime) {
			q.redispatcher.RedispatchTask(task, scheduledTime)
			continue
		}

		submitted, err := q.processor.TrySubmit(task)
		if err != nil {
			select {
			case <-q.ctx.Done():
				return
			default:
				q.logger.Error("Virtual queue failed to submit task", tag.Error(err))
			}
		}
		if !submitted {
			q.redispatcher.AddTask(task)
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

func appendOrMergeSlice(slices *list.List, incomingSlice VirtualSlice) {
	if slices.Len() == 0 {
		slices.PushBack(incomingSlice)
		return
	}

	lastElement := slices.Back()
	lastSlice := lastElement.Value.(VirtualSlice)
	mergedSlices, merged := lastSlice.TryMergeWithVirtualSlice(incomingSlice)
	if !merged {
		slices.PushBack(incomingSlice)
		return
	}

	slices.Remove(lastElement)
	for _, mergedSlice := range mergedSlices {
		slices.PushBack(mergedSlice)
	}
}
