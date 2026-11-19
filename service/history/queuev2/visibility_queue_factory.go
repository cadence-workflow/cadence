package queuev2

import (
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

type (
	visibilityQueueFactory struct {
		taskProcessor  task.Processor
		archivalClient archiver.Client
	}
)

// NewVisibilityQueueFactory creates a factory for the visibility queue.
// taskProcessor should be dedicated to the visibility queue, so a visibility backlog doesn't slow down transfer and timer tasks.
func NewVisibilityQueueFactory(
	taskProcessor task.Processor,
	archivalClient archiver.Client,
) queue.Factory {
	return &visibilityQueueFactory{taskProcessor, archivalClient}
}

func (f *visibilityQueueFactory) Category() persistence.HistoryTaskCategory {
	return persistence.HistoryTaskCategoryVisibility
}

// CreateQueue returns nil if the visibility queue is disabled for the shard
func (f *visibilityQueueFactory) CreateQueue(
	shard shard.Context,
	executionCache execution.Cache,
	_ invariant.Invariant,
) queue.Processor {
	if !shard.GetConfig().EnableVisibilityQueue(shard.GetShardID()) {
		return nil
	}

	logger := shard.GetLogger().WithTags(tag.ComponentVisibilityQueue)
	taskExecutor := task.NewVisibilityTaskExecutor(
		shard,
		f.archivalClient,
		executionCache,
		logger,
		shard.GetConfig(),
	)

	config := shard.GetConfig()
	metricsScope := shard.GetMetricsClient().Scope(metrics.VisibilityQueueProcessorV2Scope)
	shardMetricsScope := metricsScope.Tagged(metrics.ShardIDTag(shard.GetShardID()))
	options := &Options{
		PageSize:                             config.VisibilityTaskBatchSize,
		DeleteBatchSize:                      config.VisibilityTaskDeleteBatchSize,
		MaxPollRPS:                           config.VisibilityProcessorMaxPollRPS,
		MaxPollInterval:                      config.VisibilityProcessorMaxPollInterval,
		MaxPollIntervalJitterCoefficient:     config.VisibilityProcessorMaxPollIntervalJitterCoefficient,
		UpdateAckInterval:                    config.VisibilityProcessorUpdateAckInterval,
		UpdateAckIntervalJitterCoefficient:   config.VisibilityProcessorUpdateAckIntervalJitterCoefficient,
		MaxPendingTasksCount:                 config.QueueMaxPendingTaskCount,
		PollBackoffInterval:                  config.QueueProcessorPollBackoffInterval,
		PollBackoffIntervalJitterCoefficient: config.QueueProcessorPollBackoffIntervalJitterCoefficient,
		VirtualSliceForceAppendInterval:      config.VirtualSliceForceAppendInterval,
		MaxStartJitterInterval:               dynamicproperties.GetDurationPropertyFn(0),
		RedispatchInterval:                   config.ActiveTaskRedispatchInterval,
		CriticalPendingTaskCount:             config.QueueCriticalPendingTaskCount,
		EnablePendingTaskCountAlert:          func() bool { return false },
		MaxVirtualQueueCount:                 config.QueueMaxVirtualQueueCount,
	}

	reader := NewQueueReader(
		shard,
		persistence.HistoryTaskCategoryVisibility,
		config.VisibilityProcessorMaxPollInterval,
		config.VisibilityProcessorMaxPollIntervalJitterCoefficient,
	)

	return newImmediateQueue(
		shard,
		persistence.HistoryTaskCategoryVisibility,
		f.taskProcessor,
		taskExecutor,
		logger,
		shard.GetMetricsClient(),
		shardMetricsScope,
		reader,
		options,
	)
}
