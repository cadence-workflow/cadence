// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package queue

import (
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

func newTimerQueueActiveProcessor(
	clusterName string,
	shard shard.Context,
	taskProcessor task.Processor,
	taskAllocator TaskAllocator,
	taskExecutor task.Executor,
	logger log.Logger,
) *timerQueueProcessorBase {
	config := shard.GetConfig()
	options := newTimerQueueProcessorOptions(config, true, false)

	logger = logger.WithTags(tag.ClusterName(clusterName))

	taskFilter := func(timer persistence.Task) (bool, error) {
		if timer.GetTaskCategory() != persistence.HistoryTaskCategoryTimer {
			return false, errUnexpectedQueueTask
		}
		if notRegistered, err := isDomainNotRegistered(shard, timer.GetDomainID()); notRegistered && err == nil {
			// Allow deletion tasks for deprecated domains
			if timer.GetTaskType() == persistence.TaskTypeDeleteHistoryEvent {
				return true, nil
			}

			logger.Info("Domain is not in registered status, skip task in active timer queue.", tag.WorkflowDomainID(timer.GetDomainID()), tag.Value(timer))
			return false, nil
		}

		return taskAllocator.VerifyActiveTask(timer.GetDomainID(), timer.GetWorkflowID(), timer.GetRunID(), timer)
	}

	updateMaxReadLevel := func() task.Key {
		return newTimerTaskKey(shard.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryTimer, clusterName).ScheduledTime, 0)
	}

	updateClusterAckLevel := func(ackLevel task.Key) error {
		return shard.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryTimer, clusterName, persistence.HistoryTaskKey{
			ScheduledTime: ackLevel.(timerTaskKey).visibilityTimestamp,
		})
	}

	updateProcessingQueueStates := func(states []ProcessingQueueState) error {
		pStates := convertToPersistenceTimerProcessingQueueStates(states)
		return shard.UpdateTimerProcessingQueueStates(clusterName, pStates)
	}

	queueShutdown := func() error {
		return nil
	}

	return newTimerQueueProcessorBase(
		clusterName,
		shard,
		loadTimerProcessingQueueStates(clusterName, shard, options, logger),
		taskProcessor,
		clock.NewTimerGate(shard.GetTimeSource()),
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		queueShutdown,
		taskFilter,
		taskExecutor,
		logger,
		shard.GetMetricsClient(),
	)
}
