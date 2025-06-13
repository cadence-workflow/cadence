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
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/history/workflowcache"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	transferQueueFactory struct {
		taskProcessor  task.Processor
		archivalClient archiver.Client
		wfIDCache      workflowcache.WFCache
	}
)

func NewTransferQueueFactory(
	taskProcessor task.Processor,
	archivalClient archiver.Client,
	wfIDCache workflowcache.WFCache,
) queue.Factory {
	return &transferQueueFactory{taskProcessor, archivalClient, wfIDCache}
}

func (f *transferQueueFactory) Category() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryTransfer
}

func (f *transferQueueFactory) isQueueV2Enabled(shard shard.Context) bool {
	return shard.GetConfig().EnableTransferQueueV2(shard.GetShardID())
}

func (f *transferQueueFactory) CreateQueue(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	if f.isQueueV2Enabled(shard) {
		return f.createQueuev2(shard, executionCache, openExecutionCheck)
	}
	return f.createQueuev1(shard, executionCache, openExecutionCheck)
}

func (f *transferQueueFactory) createQueuev1(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	workflowResetter := reset.NewWorkflowResetter(shard, executionCache, shard.GetLogger())
	return queue.NewTransferQueueProcessor(
		shard,
		f.taskProcessor,
		executionCache,
		workflowResetter,
		f.archivalClient,
		openExecutionCheck,
		f.wfIDCache,
	)
}

func (f *transferQueueFactory) createQueuev2(
	shard shard.Context,
	executionCache execution.Cache,
	openExecutionCheck invariant.Invariant,
) queue.Processor {
	logger := shard.GetLogger().WithTags(tag.ComponentTransferQueueV2)
	workflowResetter := reset.NewWorkflowResetter(shard, executionCache, logger)
	activeTaskExecutor := task.NewTransferActiveTaskExecutor(
		shard,
		f.archivalClient,
		executionCache,
		workflowResetter,
		logger,
		shard.GetConfig(),
		f.wfIDCache,
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
	standbyTaskExecutor := task.NewTransferStandbyTaskExecutor(
		shard,
		f.archivalClient,
		executionCache,
		historyResender,
		logger,
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
	return NewImmediateQueue(
		shard,
		persistence.HistoryTaskCategoryTransfer,
		f.taskProcessor,
		executorWrapper,
		logger,
		shard.GetMetricsClient(),
		shard.GetMetricsClient().Scope(metrics.TransferQueueProcessorV2Scope).Tagged(metrics.ShardIDTag(shard.GetShardID())),
		&Options{
			PageSize:                             config.TransferTaskBatchSize,
			DeleteBatchSize:                      config.TransferTaskDeleteBatchSize,
			MaxPollRPS:                           config.TransferProcessorMaxPollRPS,
			MaxPollInterval:                      config.TransferProcessorMaxPollInterval,
			MaxPollIntervalJitterCoefficient:     config.TransferProcessorMaxPollIntervalJitterCoefficient,
			UpdateAckInterval:                    config.TransferProcessorUpdateAckInterval,
			UpdateAckIntervalJitterCoefficient:   config.TransferProcessorUpdateAckIntervalJitterCoefficient,
			MaxRedispatchQueueSize:               config.TransferProcessorMaxRedispatchQueueSize,
			PollBackoffInterval:                  config.QueueProcessorPollBackoffInterval,
			PollBackoffIntervalJitterCoefficient: config.QueueProcessorPollBackoffIntervalJitterCoefficient,
			EnableValidator:                      config.TransferProcessorEnableValidator,
			ValidationInterval:                   config.TransferProcessorValidationInterval,
			MaxStartJitterInterval:               dynamicproperties.GetDurationPropertyFn(0),
			RedispatchInterval:                   config.ActiveTaskRedispatchInterval,
		},
	)
}
