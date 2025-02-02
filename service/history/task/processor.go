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
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/task"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

type processorImpl struct {
	sync.RWMutex

	priorityAssigner PriorityAssigner
	hostScheduler    task.Scheduler
	shardSchedulers  map[shard.Context]task.Scheduler

	status        int32
	options       *task.SchedulerOptions[int]
	shardOptions  *task.SchedulerOptions[int]
	logger        log.Logger
	metricsClient metrics.Client
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
	hostScheduler, err := createTaskScheduler(options, logger, metricsClient)
	if err != nil {
		return nil, err
	}
	logger.Debug("Host level task scheduler is created", tag.Dynamic("scheduler_options", options.String()))

	var shardOptions *task.SchedulerOptions[int]
	if config.TaskSchedulerShardWorkerCount() > 0 {
		shardOptions, err = task.NewSchedulerOptions[int](
			config.TaskSchedulerType(),
			config.TaskSchedulerShardQueueSize(),
			config.TaskSchedulerShardWorkerCount,
			1,
			taskToChannelKeyFn,
			channelKeyToWeightFn,
		)
		if err != nil {
			return nil, err
		}
		logger.Debug("Shard level task scheduler is enabled", tag.Dynamic("scheduler_options", shardOptions.String()))
	}

	return &processorImpl{
		priorityAssigner: priorityAssigner,
		hostScheduler:    hostScheduler,
		shardSchedulers:  make(map[shard.Context]task.Scheduler),
		status:           common.DaemonStatusInitialized,
		options:          options,
		shardOptions:     shardOptions,
		logger:           logger,
		metricsClient:    metricsClient,
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

	p.Lock()
	defer p.Unlock()

	for shard, scheduler := range p.shardSchedulers {
		delete(p.shardSchedulers, shard)
		scheduler.Stop()
	}

	p.logger.Info("Queue task processor stopped.")
}

func (p *processorImpl) StopShardProcessor(shard shard.Context) {
	p.Lock()
	scheduler, ok := p.shardSchedulers[shard]
	if !ok {
		p.Unlock()
		return
	}

	delete(p.shardSchedulers, shard)
	p.Unlock()

	// don't hold the lock while stopping the scheduler
	scheduler.Stop()
}

func (p *processorImpl) Submit(task Task) error {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return err
	}

	submitted, err := p.hostScheduler.TrySubmit(task)
	if err != nil {
		return err
	}

	if submitted {
		return nil
	}

	shardScheduler, err := p.getOrCreateShardTaskScheduler(task.GetShard())
	if err != nil {
		return err
	}

	if shardScheduler != nil {
		return shardScheduler.Submit(task)
	}

	// if shard level scheduler is disabled
	return p.hostScheduler.Submit(task)
}

func (p *processorImpl) TrySubmit(task Task) (bool, error) {
	if err := p.priorityAssigner.Assign(task); err != nil {
		return false, err
	}

	submitted, err := p.hostScheduler.TrySubmit(task)
	if err != nil {
		return false, err
	}

	if submitted {
		return true, nil
	}

	shardScheduler, err := p.getOrCreateShardTaskScheduler(task.GetShard())
	if err != nil {
		return false, err
	}

	if shardScheduler != nil {
		return shardScheduler.TrySubmit(task)
	}

	// if shard level scheduler is disabled
	return false, nil
}

func (p *processorImpl) getOrCreateShardTaskScheduler(shard shard.Context) (task.Scheduler, error) {
	if p.shardOptions == nil {
		return nil, nil
	}

	p.RLock()
	if scheduler, ok := p.shardSchedulers[shard]; ok {
		p.RUnlock()
		return scheduler, nil
	}
	p.RUnlock()

	p.Lock()
	if scheduler, ok := p.shardSchedulers[shard]; ok {
		p.Unlock()
		return scheduler, nil
	}

	if !p.isRunning() {
		p.Unlock()
		return nil, errTaskProcessorNotRunning
	}

	scheduler, err := createTaskScheduler(p.shardOptions, p.logger.WithTags(tag.ShardID(shard.GetShardID())), p.metricsClient)
	if err != nil {
		p.Unlock()
		return nil, err
	}

	p.shardSchedulers[shard] = scheduler
	p.Unlock()

	// don't hold the lock while starting the scheduler
	scheduler.Start()
	p.logger.Debug("Shard level task scheduler is started",
		tag.ShardID(shard.GetShardID()),
		tag.Dynamic("scheduler_options", p.shardOptions.String()),
	)
	return scheduler, nil
}

func (p *processorImpl) isRunning() bool {
	return atomic.LoadInt32(&p.status) == common.DaemonStatusStarted
}

func createTaskScheduler(
	options *task.SchedulerOptions[int],
	logger log.Logger,
	metricsClient metrics.Client,
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
			options.WRRSchedulerOptions,
		)
	default:
		// the scheduler type has already been verified when initializing the processor
		panic(fmt.Sprintf("Unknown task scheduler type, %v", options.SchedulerType))
	}

	return scheduler, err
}
