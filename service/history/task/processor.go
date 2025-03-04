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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
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
	hostScheduler    task.Scheduler
	shadowScheduler  task.Scheduler

	shadowMode    string
	status        int32
	logger        log.Logger
	metricsClient metrics.Client
	timeSource    clock.TimeSource
}

const (
	schedulerShadowModeShadow        = "shadow"
	schedulerShadowModeReverseShadow = "reverse-shadow"
	schedulerShadowModeReverse       = "reverse"
)

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
	taskToChannelKeyFn := func(t task.PriorityTask) int {
		return t.Priority()
	}
	channelKeyToWeightFn := func(priority int) int {
		weights, err := common.ConvertDynamicConfigMapPropertyToIntMap(config.TaskSchedulerRoundRobinWeights())
		if err != nil {
			logger.Error("failed to convert dynamic config map to int map", tag.Error(err))
			weights = dynamicconfig.DefaultTaskSchedulerRoundRobinWeights
		}
		weight, ok := weights[priority]
		if !ok {
			logger.Error("weights not found for task priority", tag.Dynamic("priority", priority), tag.Dynamic("weights", weights))
		}
		return weight
	}
	options, err := task.NewSchedulerOptions[int](
		config.TaskSchedulerType(),
		config.TaskSchedulerQueueSize(),
		config.TaskSchedulerWorkerCount,
		config.TaskSchedulerDispatcherCount(),
		taskToChannelKeyFn,
		channelKeyToWeightFn,
	)
	if err != nil {
		return nil, err
	}
	hostScheduler, err := createTaskScheduler(options, logger, metricsClient, timeSource)
	if err != nil {
		return nil, err
	}

	var shadowScheduler task.Scheduler
	shadowMode := config.TaskSchedulerShadowMode()
	if shadowMode == "shadow" || shadowMode == "reverse-shadow" || shadowMode == "reverse" {
		// create shadow scheduler
		taskToChannelKeyFn := func(t task.PriorityTask) DomainPriorityKey {
			var domainID string
			tt, ok := t.(Task)
			if ok {
				domainID = tt.GetDomainID()
			}
			return DomainPriorityKey{
				DomainID: domainID,
				Priority: t.Priority(),
			}
		}
		channelKeyToWeightFn := func(k DomainPriorityKey) int {
			var weights map[int]int
			domainName, err := domainCache.GetDomainName(k.DomainID)
			if err != nil {
				logger.Error("failed to get domain name from cache, use default round robin weights", tag.Error(err))
				weights = dynamicconfig.DefaultTaskSchedulerRoundRobinWeights
			} else {
				weights, err = common.ConvertDynamicConfigMapPropertyToIntMap(config.TaskSchedulerDomainRoundRobinWeights(domainName))
				if err != nil {
					logger.Error("failed to convert dynamic config map to int map, use default round robin weights", tag.Error(err))
					weights = dynamicconfig.DefaultTaskSchedulerRoundRobinWeights
				}
			}
			weight, ok := weights[k.Priority]
			if !ok {
				logger.Error("weights not found for task priority", tag.Dynamic("priority", k.Priority), tag.Dynamic("weights", weights))
			}
			return weight
		}
		shadowScheduler, err = task.NewWeightedRoundRobinTaskScheduler(
			logger,
			metricsClient,
			timeSource,
			&task.WeightedRoundRobinTaskSchedulerOptions[DomainPriorityKey]{
				QueueSize:            config.TaskSchedulerQueueSize(),
				WorkerCount:          config.TaskSchedulerWorkerCount,
				DispatcherCount:      config.TaskSchedulerDispatcherCount(),
				RetryPolicy:          common.CreateTaskProcessingRetryPolicy(),
				TaskToChannelKeyFn:   taskToChannelKeyFn,
				ChannelKeyToWeightFn: channelKeyToWeightFn,
			},
		)
	}

	return &processorImpl{
		priorityAssigner: priorityAssigner,
		hostScheduler:    hostScheduler,
		shadowScheduler:  shadowScheduler,
		shadowMode:       shadowMode,
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

	p.hostScheduler.Start()
	if p.shadowScheduler != nil {
		p.shadowScheduler.Start()
	}

	p.logger.Info("Queue task processor started.")
}

func (p *processorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.hostScheduler.Stop()
	if p.shadowScheduler != nil {
		p.shadowScheduler.Stop()
	}

	p.logger.Info("Queue task processor stopped.")
}

func (p *processorImpl) Submit(task Task) error {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return err
	}
	scheduler, shadowScheduler := p.getSchedulerAndShadowScheduler()
	if shadowScheduler != nil {
		submitted, err := p.shadowScheduler.TrySubmit(task.ShadowCopy())
		if !submitted {
			p.logger.Warn("Failed to submit task to shadow scheduler", tag.Error(err))
		}
	}
	return scheduler.Submit(task)
}

func (p *processorImpl) TrySubmit(task Task) (bool, error) {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return false, err
	}

	scheduler, shadowScheduler := p.getSchedulerAndShadowScheduler()
	if shadowScheduler != nil {
		submitted, err := p.shadowScheduler.TrySubmit(task.ShadowCopy())
		if !submitted {
			p.logger.Warn("Failed to submit task to shadow scheduler", tag.Error(err))
		}
	}
	return scheduler.TrySubmit(task)
}

func (p *processorImpl) getSchedulerAndShadowScheduler() (task.Scheduler, task.Scheduler) {
	if p.shadowMode == "reverse-shadow" {
		return p.shadowScheduler, p.hostScheduler
	} else if p.shadowMode == "reverse" {
		return p.shadowScheduler, nil
	} else if p.shadowMode == "shadow" {
		return p.hostScheduler, p.shadowScheduler
	}
	return p.hostScheduler, nil
}

func createTaskScheduler(
	options *task.SchedulerOptions[int],
	logger log.Logger,
	metricsClient metrics.Client,
	timeSource clock.TimeSource,
) (task.Scheduler, error) {
	var scheduler task.Scheduler
	var err error
	switch options.SchedulerType {
	case task.SchedulerTypeFIFO:
		scheduler = task.NewFIFOTaskScheduler(
			logger,
			metricsClient,
			options.FIFOSchedulerOptions,
		)
	case task.SchedulerTypeWRR:
		scheduler, err = task.NewWeightedRoundRobinTaskScheduler(
			logger,
			metricsClient,
			timeSource,
			options.WRRSchedulerOptions,
		)
	default:
		// the scheduler type has already been verified when initializing the processor
		panic(fmt.Sprintf("Unknown task scheduler type, %v", options.SchedulerType))
	}

	return scheduler, err
}
