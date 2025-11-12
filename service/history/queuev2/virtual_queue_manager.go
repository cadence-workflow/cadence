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

//go:generate mockgen -destination virtual_queue_manager_mock.go -package queuev2 github.com/uber/cadence/service/history/queuev2 VirtualQueueManager
package queuev2

import (
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

const (
	rootQueueID = 0
)

type (
	VirtualQueueManagerOptions struct {
		RootQueueOptions                *VirtualQueueOptions
		NonRootQueueOptions             *VirtualQueueOptions
		VirtualSliceForceAppendInterval dynamicproperties.DurationPropertyFn
	}
	VirtualQueueManager interface {
		common.Daemon
		VirtualQueues() map[int64]VirtualQueue
		GetOrCreateVirtualQueue(int64) VirtualQueue
		UpdateAndGetState() map[int64][]VirtualSliceState
		// Add a new virtual slice to the root queue. This is used when new tasks are generated and max read level is updated.
		// By default, all new tasks belong to the root queue, so we need to add a new virtual slice to the root queue.
		AddNewVirtualSliceToRootQueue(VirtualSlice)
		// Insert a single task to the current slice. Return false if the task's timestamp is out of range of the current slice.
		InsertSingleTask(task.Task) bool
		// ResetProgress resets the progress of the virtual queue to the given key. Pending tasks after the given key are cancelled.
		ResetProgress(persistence.HistoryTaskKey)
	}

	virtualQueueManagerImpl struct {
		processor           task.Processor
		taskInitializer     task.Initializer
		rescheduler         task.Rescheduler
		queueReader         QueueReader
		logger              log.Logger
		metricsScope        metrics.Scope
		timeSource          clock.TimeSource
		taskLoadRateLimiter quotas.Limiter
		monitor             Monitor
		queueManagerOptions *VirtualQueueManagerOptions

		sync.RWMutex
		status               int32
		virtualQueues        map[int64]VirtualQueue
		createVirtualQueueFn func(int64, ...VirtualSlice) VirtualQueue

		nextForceNewSliceTime time.Time
	}
)

func NewVirtualQueueManager(
	processor task.Processor,
	rescheduler task.Rescheduler,
	taskInitializer task.Initializer,
	queueReader QueueReader,
	logger log.Logger,
	metricsScope metrics.Scope,
	timeSource clock.TimeSource,
	taskLoadRateLimiter quotas.Limiter,
	monitor Monitor,
	queueManagerOptions *VirtualQueueManagerOptions,
	virtualQueueStates map[int64][]VirtualSliceState,
) VirtualQueueManager {
	virtualQueues := make(map[int64]VirtualQueue)
	for queueID, states := range virtualQueueStates {
		virtualSlices := make([]VirtualSlice, len(states))
		for i, state := range states {
			virtualSlices[i] = NewVirtualSlice(state, taskInitializer, queueReader, NewPendingTaskTracker(), logger)
		}
		var opts *VirtualQueueOptions
		if queueID == rootQueueID {
			opts = queueManagerOptions.RootQueueOptions
		} else {
			opts = queueManagerOptions.NonRootQueueOptions
		}
		virtualQueues[queueID] = NewVirtualQueue(processor, rescheduler, logger.WithTags(tag.VirtualQueueID(queueID)), metricsScope, timeSource, taskLoadRateLimiter, monitor, virtualSlices, opts)
	}
	return &virtualQueueManagerImpl{
		processor:           processor,
		taskInitializer:     taskInitializer,
		queueReader:         queueReader,
		rescheduler:         rescheduler,
		logger:              logger,
		metricsScope:        metricsScope,
		timeSource:          timeSource,
		taskLoadRateLimiter: taskLoadRateLimiter,
		monitor:             monitor,
		queueManagerOptions: queueManagerOptions,
		status:              common.DaemonStatusInitialized,
		virtualQueues:       virtualQueues,
		createVirtualQueueFn: func(queueID int64, s ...VirtualSlice) VirtualQueue {
			var opts *VirtualQueueOptions
			if queueID == rootQueueID {
				opts = queueManagerOptions.RootQueueOptions
			} else {
				opts = queueManagerOptions.NonRootQueueOptions
			}
			return NewVirtualQueue(processor, rescheduler, logger.WithTags(tag.VirtualQueueID(queueID)), metricsScope, timeSource, taskLoadRateLimiter, monitor, s, opts)
		},
	}
}

func (m *virtualQueueManagerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&m.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	m.RLock()
	defer m.RUnlock()

	for _, vq := range m.virtualQueues {
		vq.Start()
	}
}

func (m *virtualQueueManagerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&m.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	m.RLock()
	defer m.RUnlock()

	for _, vq := range m.virtualQueues {
		vq.Stop()
	}
}

func (m *virtualQueueManagerImpl) VirtualQueues() map[int64]VirtualQueue {
	m.RLock()
	defer m.RUnlock()
	return m.virtualQueues
}

func (m *virtualQueueManagerImpl) GetOrCreateVirtualQueue(queueID int64) VirtualQueue {
	m.RLock()
	if vq, ok := m.virtualQueues[queueID]; ok {
		m.RUnlock()
		return vq
	}
	m.RUnlock()

	m.Lock()
	defer m.Unlock()
	if vq, ok := m.virtualQueues[queueID]; ok {
		return vq
	}
	m.virtualQueues[queueID] = m.createVirtualQueueFn(queueID)
	m.virtualQueues[queueID].Start()
	return m.virtualQueues[queueID]
}

func (m *virtualQueueManagerImpl) UpdateAndGetState() map[int64][]VirtualSliceState {
	m.Lock()
	defer m.Unlock()

	virtualQueueStates := make(map[int64][]VirtualSliceState)
	for key, vq := range m.virtualQueues {
		state := vq.UpdateAndGetState()
		if len(state) > 0 {
			virtualQueueStates[key] = state
		} else if key != rootQueueID {
			vq.Stop()
			delete(m.virtualQueues, key)
		}
	}
	return virtualQueueStates
}

func (m *virtualQueueManagerImpl) AddNewVirtualSliceToRootQueue(s VirtualSlice) {
	m.RLock()
	if vq, ok := m.virtualQueues[rootQueueID]; ok {
		m.RUnlock()
		m.appendOrMergeSlice(vq, s)
		return
	}
	m.RUnlock()

	m.Lock()
	defer m.Unlock()
	if vq, ok := m.virtualQueues[rootQueueID]; ok {
		m.appendOrMergeSlice(vq, s)
		return
	}

	m.virtualQueues[rootQueueID] = m.createVirtualQueueFn(rootQueueID, s)
	m.virtualQueues[rootQueueID].Start()
}

func (m *virtualQueueManagerImpl) InsertSingleTask(t task.Task) bool {
	m.Lock()
	defer m.Unlock()

	for _, vq := range m.virtualQueues {
		if vq.InsertSingleTask(t) {
			return true
		}
	}

	return false
}

func (m *virtualQueueManagerImpl) ResetProgress(key persistence.HistoryTaskKey) {
	m.Lock()
	defer m.Unlock()
	for _, vq := range m.virtualQueues {
		vq.ResetProgress(key)
	}
}

func (m *virtualQueueManagerImpl) appendOrMergeSlice(vq VirtualQueue, s VirtualSlice) {
	now := m.timeSource.Now()
	newVirtualSliceState := s.GetState()
	// TODO: we should set a limit on the number of virtual slices to prevent the size of queue state from being too large to be stored in database
	if now.After(m.nextForceNewSliceTime) {
		m.logger.Debug("append new slice to virtual queue", tag.Dynamic("currentTime", now), tag.Dynamic("nextForceNewSliceTime", m.nextForceNewSliceTime), tag.Dynamic("inclusiveMinTaskKey", newVirtualSliceState.Range.InclusiveMinTaskKey), tag.Dynamic("exclusiveMaxTaskKey", newVirtualSliceState.Range.ExclusiveMaxTaskKey))
		vq.AppendSlices(s)
		m.nextForceNewSliceTime = now.Add(m.queueManagerOptions.VirtualSliceForceAppendInterval())
		return
	}
	m.logger.Debug("merge slice to virtual queue", tag.Dynamic("currentTime", now), tag.Dynamic("nextForceNewSliceTime", m.nextForceNewSliceTime), tag.Dynamic("inclusiveMinTaskKey", newVirtualSliceState.Range.InclusiveMinTaskKey), tag.Dynamic("exclusiveMaxTaskKey", newVirtualSliceState.Range.ExclusiveMaxTaskKey))
	vq.MergeWithLastSlice(s)
}
