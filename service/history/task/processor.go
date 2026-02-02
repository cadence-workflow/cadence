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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
)

type DomainPriorityKey struct {
	DomainID string
	Priority int
}

type processorImpl struct {
	sync.RWMutex

	priorityAssigner PriorityAssigner
	taskProcessor    task.Processor
	scheduler        task.Scheduler

	status        int32
	logger        log.Logger
	metricsClient metrics.Client
	timeSource    clock.TimeSource
}

var (
	errTaskProcessorNotRunning = errors.New("queue task processor is not running")
)

// NewProcessor creates a new task processor
func NewProcessor(
	priorityAssigner PriorityAssigner,
	config *config.Config,
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
	domainCache cache.DomainCache,
) (Processor, error) {
	taskProcessor := task.NewParallelTaskProcessor(
		logger,
		metricsClient,
		&task.ParallelTaskProcessorOptions{
			QueueSize:   1,
			WorkerCount: config.TaskSchedulerWorkerCount,
			RetryPolicy: common.CreateTaskProcessingRetryPolicy(),
		},
	)
	taskToChannelKeyFn := func(t task.PriorityTask) DomainPriorityKey {
		var domainID string
		tt, ok := t.(Task)
		if ok {
			domainID = tt.GetDomainID()
		} else {
			logger.Error("incorrect task type for task scheduler, this should not happen, there must be a bug in our code")
		}
		return DomainPriorityKey{
			DomainID: domainID,
			Priority: t.Priority(),
		}
	}
	channelKeyToWeightFn := func(k DomainPriorityKey) int {
		return getDomainPriorityWeight(logger, config, domainCache, k)
	}
	scheduler, err := task.NewWeightedRoundRobinTaskScheduler(
		logger,
		metricsClient,
		timeSource,
		taskProcessor,
		&task.WeightedRoundRobinTaskSchedulerOptions[DomainPriorityKey]{
			QueueSize:            config.TaskSchedulerQueueSize(),
			DispatcherCount:      config.TaskSchedulerDispatcherCount(),
			TaskToChannelKeyFn:   taskToChannelKeyFn,
			ChannelKeyToWeightFn: channelKeyToWeightFn,
		},
	)
	if err != nil {
		return nil, err
	}
	return &processorImpl{
		priorityAssigner: priorityAssigner,
		taskProcessor:    taskProcessor,
		scheduler:        scheduler,
		status:           common.DaemonStatusInitialized,
		logger:           logger,
		metricsClient:    metricsClient,
		timeSource:       timeSource,
	}, nil
}

func (p *processorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.taskProcessor.Start()
	p.scheduler.Start()

	p.logger.Info("Queue task processor started.")
}

func (p *processorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.scheduler.Stop()
	p.taskProcessor.Stop()

	p.logger.Info("Queue task processor stopped.")
}

func (p *processorImpl) Submit(task Task) error {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return err
	}
	return p.scheduler.Submit(task)
}

func (p *processorImpl) TrySubmit(task Task) (bool, error) {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return false, err
	}
	return p.scheduler.TrySubmit(task)
}

func getDomainPriorityWeight(
	logger log.Logger,
	config *config.Config,
	domainCache cache.DomainCache,
	k DomainPriorityKey,
) int {
	var weights map[int]int
	domainName, err := domainCache.GetDomainName(k.DomainID)
	if err != nil {
		logger.Error("failed to get domain name from cache, use default round robin weights", tag.Error(err))
		weights = dynamicproperties.DefaultTaskSchedulerRoundRobinWeights
	} else {
		weights, err = dynamicproperties.ConvertDynamicConfigMapPropertyToIntMap(config.TaskSchedulerDomainRoundRobinWeights(domainName))
		if err != nil {
			logger.Error("failed to convert dynamic config map to int map, use default round robin weights", tag.Error(err))
			weights = dynamicproperties.DefaultTaskSchedulerRoundRobinWeights
		}
	}
	weight, ok := weights[k.Priority]
	if !ok {
		logger.Error("weights not found for task priority, default to 1", tag.Dynamic("priority", k.Priority), tag.Dynamic("weights", weights))
		weight = 1
	}
	return weight
}
