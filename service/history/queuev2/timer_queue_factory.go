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

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	timerQueueFactory struct {
		taskProcessor  task.Processor
		archivalClient archiver.Client
	}
)

func NewTimerQueueFactory(
	taskProcessor task.Processor,
	archivalClient archiver.Client,
) queue.Factory {
	return &timerQueueFactory{
		taskProcessor:  taskProcessor,
		archivalClient: archivalClient,
	}
}

func (f *timerQueueFactory) Category() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryTimer
}

func (f *timerQueueFactory) isQueueV2Enabled(shard shard.Context) bool {
	return shard.GetConfig().EnableTimerQueueV2(shard.GetShardID())
}

func (f *timerQueueFactory) CreateQueue(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	if f.isQueueV2Enabled(shard) {
		return f.createQueuev2(shard, executionCache, openExecutionCheck)
	}
	return f.createQueuev1(shard, executionCache, openExecutionCheck)
}

func (f *timerQueueFactory) createQueuev1(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	return queue.NewTimerQueueProcessor(
		shard,
		f.taskProcessor,
		executionCache,
		f.archivalClient,
		openExecutionCheck,
	)
}

func (f *timerQueueFactory) createQueuev2(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	logger := shard.GetLogger().WithTags(tag.ComponentTimerQueueV2)
	activeTaskExecutor := task.NewTimerActiveTaskExecutor(
		shard,
		f.archivalClient,
		executionCache,
		logger,
		shard.GetMetricsClient(),
		shard.GetConfig(),
	)
	historyResender := ndc.NewHistoryResender(
		shard.GetDomainCache(),
		shard.GetService().GetClientBean(),
		func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
			return shard.GetEngine().ReplicateEventsV2(ctx, request)
		},
		shard.GetConfig().StandbyTaskReReplicationContextTimeout,
		openExecutionCheck,
		logger,
	)
	standbyTaskExecutor := task.NewTimerStandbyTaskExecutor(
		shard,
		f.archivalClient,
		executionCache,
		historyResender,
		logger,
		shard.GetMetricsClient(),
		shard.GetClusterMetadata().GetCurrentClusterName(),
		shard.GetConfig(),
	)
	executorWrapper := task.NewExecutorWrapper(
		shard.GetClusterMetadata().GetCurrentClusterName(),
		shard.GetDomainCache(),
		shard.GetActiveClusterManager(),
		activeTaskExecutor,
		standbyTaskExecutor,
		logger,
	)
	config := shard.GetConfig()
	return NewScheduledQueue(
		shard,
		persistence.HistoryTaskCategoryTimer,
		f.taskProcessor,
		executorWrapper,
		logger,
		shard.GetMetricsClient(),
		shard.GetMetricsClient().Scope(metrics.TimerQueueProcessorV2Scope).Tagged(metrics.ShardIDTag(shard.GetShardID())),
		&Options{
			PageSize:                             config.TimerTaskBatchSize,
			DeleteBatchSize:                      config.TimerTaskDeleteBatchSize,
			MaxPollRPS:                           config.TimerProcessorMaxPollRPS,
			MaxPollInterval:                      config.TimerProcessorMaxPollInterval,
			MaxPollIntervalJitterCoefficient:     config.TimerProcessorMaxPollIntervalJitterCoefficient,
			UpdateAckInterval:                    config.TimerProcessorUpdateAckInterval,
			UpdateAckIntervalJitterCoefficient:   config.TimerProcessorUpdateAckIntervalJitterCoefficient,
			MaxPendingTasksCount:                 config.QueueMaxPendingTaskCount,
			PollBackoffInterval:                  config.QueueProcessorPollBackoffInterval,
			PollBackoffIntervalJitterCoefficient: config.QueueProcessorPollBackoffIntervalJitterCoefficient,
			VirtualSliceForceAppendInterval:      config.VirtualSliceForceAppendInterval,
			MaxStartJitterInterval:               dynamicproperties.GetDurationPropertyFn(0),
			RedispatchInterval:                   config.ActiveTaskRedispatchInterval,
			CriticalPendingTaskCount:             config.QueueCriticalPendingTaskCount,
			EnablePendingTaskCountAlert:          func() bool { return config.EnableTimerQueueV2PendingTaskCountAlert(shard.GetShardID()) },
			MaxVirtualQueueCount:                 config.QueueMaxVirtualQueueCount,
		},
	)
}
