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
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
)

type processorImpl struct {
	sync.RWMutex

	priorityAssigner PriorityAssigner
	hostScheduler    task.Scheduler

	status        int32
	options       *task.SchedulerOptions[int]
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
	logger.Debug("Host level task scheduler is created", tag.Dynamic("scheduler_options", options.String()))

	return &processorImpl{
		priorityAssigner: priorityAssigner,
		hostScheduler:    hostScheduler,
		status:           common.DaemonStatusInitialized,
		options:          options,
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

	p.logger.Info("Queue task processor started.")
}

func (p *processorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.hostScheduler.Stop()

	p.logger.Info("Queue task processor stopped.")
}

func (p *processorImpl) Submit(task Task) error {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return err
	}
	return p.hostScheduler.Submit(task)
}

func (p *processorImpl) TrySubmit(task Task) (bool, error) {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return false, err
	}

	return p.hostScheduler.TrySubmit(task)
}

func (p *processorImpl) isRunning() bool {
	return atomic.LoadInt32(&p.status) == common.DaemonStatusStarted
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
