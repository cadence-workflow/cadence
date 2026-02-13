// The MIT License (MIT)
//
// Copyright (c) 2017-2022 Uber Technologies Inc.
//
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

package replication

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
)

type (
	// TaskAckManager is the ack manager for replication tasks
	TaskAckManager struct {
		ackLevels ackLevelStore

		scope  metrics.Scope
		logger log.Logger

		reader taskReader
		store  *TaskStore

		// replicationMessagesSizeFn is the function to calculate the size of types.ReplicationMessages
		replicationMessagesSizeFn types.ReplicationMessagesSizeFn

		// maxReplicationMessagesSize is the max size of types.ReplicationMessages
		// that can be sent in a single RPC call
		maxReplicationMessagesSize int

		dynamicTaskBatchSizer DynamicTaskBatchSizer
		timeSource            clock.TimeSource
	}

	ackLevelStore interface {
		UpdateIfNeededAndGetQueueMaxReadLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey
		GetQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string) persistence.HistoryTaskKey
		UpdateQueueClusterAckLevel(category persistence.HistoryTaskCategory, cluster string, ackLevel persistence.HistoryTaskKey) error
	}
	taskReader interface {
		Read(ctx context.Context, readLevel int64, maxReadLevel int64, batchSize int) ([]persistence.Task, bool, error)
	}
)

// NewTaskAckManager initializes a new replication task ack manager
func NewTaskAckManager(
	shardID int,
	ackLevels ackLevelStore,
	metricsClient metrics.Client,
	logger log.Logger,
	reader taskReader,
	store *TaskStore,
	timeSource clock.TimeSource,
	config *config.Config,
	replicationMessagesSizeFn types.ReplicationMessagesSizeFn,
	dynamicTaskBatchSizer DynamicTaskBatchSizer,
) TaskAckManager {

	return TaskAckManager{
		ackLevels: ackLevels,
		scope: metricsClient.Scope(
			metrics.ReplicatorQueueProcessorScope,
			metrics.InstanceTag(strconv.Itoa(shardID)),
		),
		logger:     logger.WithTags(tag.ComponentReplicationAckManager),
		reader:     reader,
		store:      store,
		timeSource: timeSource,

		maxReplicationMessagesSize: config.MaxResponseSize,
		replicationMessagesSizeFn:  replicationMessagesSizeFn,
		dynamicTaskBatchSizer:      dynamicTaskBatchSizer,
	}
}

func (t *TaskAckManager) GetTasks(ctx context.Context, pollingCluster string, lastReadTaskID int64) (_ *types.ReplicationMessages, err error) {
	result, err := t.getTasks(ctx, pollingCluster, lastReadTaskID)
	t.dynamicTaskBatchSizer.analyse(err, result)
	if err != nil {
		return nil, err
	}

	return result.msgs, nil
}

// getTasksResult contains the result of a TaskAckManager.getTasks
// It is used to adjust the task batch size by DynamicTaskBatchSizer
type getTasksResult struct {
	previousReadTaskID int64
	lastReadTaskID     int64
	msgs               *types.ReplicationMessages
	taskInfos          []persistence.Task
	isShrunk           bool
}

func (t *TaskAckManager) getTasks(ctx context.Context, pollingCluster string, lastReadTaskID int64) (*getTasksResult, error) {
	var oldestUnprocessedTaskID int64
	var oldestUnprocessedTaskTimestamp int64
	previousReadTaskID := t.ackLevels.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, pollingCluster).GetTaskID()

	if lastReadTaskID == constants.EmptyMessageID {
		lastReadTaskID = previousReadTaskID
	}

	// Keep timer (backwards compatible), dual-emit exponential histogram for migration.
	taskGeneratedStart := t.timeSource.Now()
	taskGeneratedTimer := t.scope.StartTimer(metrics.TaskLatency)
	defer taskGeneratedTimer.Stop()
	defer func() {
		t.scope.ExponentialHistogram(metrics.ExponentialTaskLatency, t.timeSource.Since(taskGeneratedStart))
	}()

	batchSize := t.dynamicTaskBatchSizer.value()
	t.scope.UpdateGauge(metrics.ReplicationTasksBatchSize, float64(batchSize))

	taskInfos, hasMore, err := t.reader.Read(ctx, lastReadTaskID, t.ackLevels.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryReplication, pollingCluster).GetTaskID(), batchSize)
	if err != nil {
		return nil, err
	}
	tasksFetched := len(taskInfos)
	t.scope.RecordTimer(metrics.ReplicationTasksFetched, time.Duration(tasksFetched))
	t.scope.IntExponentialHistogram(metrics.ExponentialReplicationTasksFetched, tasksFetched)

	// Happy path assumption - we will push all tasks to replication tasks.
	msgs := &types.ReplicationMessages{
		ReplicationTasks:       make([]*types.ReplicationTask, 0, len(taskInfos)),
		LastRetrievedMessageID: lastReadTaskID,
		HasMore:                hasMore,
	}

	if len(taskInfos) > 0 {
		// it does not matter if we can process task or not, but we need to know what was the oldest task information we have read.
		// tasks must be ordered by taskID/time.
		oldestUnprocessedTaskID = taskInfos[0].GetTaskID()
		oldestUnprocessedTaskTimestamp = taskInfos[0].GetVisibilityTimestamp().UnixNano()
	} else {
		// if there are no replication tasks, use the max read level as the oldest unprocessed task
		// this means there are no tasks to process, so the lag is 0
		oldestUnprocessedTaskID = t.ackLevels.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryReplication, pollingCluster).GetTaskID()
		oldestUnprocessedTaskTimestamp = t.timeSource.Now().UnixNano()
	}

	lagRaw := int(t.ackLevels.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryReplication, pollingCluster).GetTaskID() - oldestUnprocessedTaskID)
	t.scope.RecordTimer(metrics.ReplicationTasksLagRaw, time.Duration(lagRaw))
	t.scope.IntExponentialHistogram(metrics.ExponentialReplicationTasksLagRaw, lagRaw)
	t.scope.RecordHistogramDuration(metrics.ReplicationTasksDelay, time.Duration(oldestUnprocessedTaskTimestamp-t.timeSource.Now().UnixNano()))

	// hydrate the tasks
	for _, info := range taskInfos {
		task, err := t.store.Get(ctx, pollingCluster, info)
		if err != nil {
			if errors.As(err, new(*types.BadRequestError)) ||
				errors.As(err, new(*types.InternalDataInconsistencyError)) ||
				errors.As(err, new(*types.EntityNotExistsError)) {
				t.logger.Warn("Failed to get replication task.", tag.Error(err))
			} else {
				t.logger.Error("Failed to get replication task. Return what we have so far.", tag.Error(err))
				msgs.HasMore = true
				break
			}
		}

		msgs.LastRetrievedMessageID = info.GetTaskID()
		if task != nil {
			msgs.ReplicationTasks = append(msgs.ReplicationTasks, task)
		}
	}

	// Sometimes the total size of replication tasks can be larger than the max response size
	// It caused the replication lag until history.replicatorTaskBatchSize is not adjusted to a smaller value
	// To prevent the lag and manual actions, we stop adding more tasks to the batch if the total size exceeds the limit
	isShrunk, err := t.shrinkMessagesBySize(msgs)
	if err != nil {
		return nil, err
	}

	// Keep timers (backwards compatible), dual-emit exponential histograms for migration.
	replicationLag := time.Duration(t.ackLevels.UpdateIfNeededAndGetQueueMaxReadLevel(persistence.HistoryTaskCategoryReplication, pollingCluster).GetTaskID() - msgs.LastRetrievedMessageID)
	t.scope.RecordTimer(metrics.ReplicationTasksLag, replicationLag)
	t.scope.ExponentialHistogram(metrics.ExponentialReplicationTasksLag, replicationLag)

	tasksReturned := len(msgs.ReplicationTasks)
	t.scope.RecordTimer(metrics.ReplicationTasksReturned, time.Duration(tasksReturned))
	t.scope.IntExponentialHistogram(metrics.ExponentialReplicationTasksReturned, tasksReturned)

	tasksReturnedDiff := len(taskInfos) - len(msgs.ReplicationTasks)
	t.scope.RecordTimer(metrics.ReplicationTasksReturnedDiff, time.Duration(tasksReturnedDiff))
	t.scope.IntExponentialHistogram(metrics.ExponentialReplicationTasksReturnedDiff, tasksReturnedDiff)

	t.ackLevel(pollingCluster, lastReadTaskID)

	t.logger.Debug(
		"Get replication tasks",
		tag.SourceCluster(pollingCluster),
		tag.ShardReplicationAck(msgs.LastRetrievedMessageID),
		tag.ReadLevel(msgs.LastRetrievedMessageID),
	)

	return &getTasksResult{
		previousReadTaskID: previousReadTaskID,
		lastReadTaskID:     lastReadTaskID,
		msgs:               msgs,
		taskInfos:          taskInfos,
		isShrunk:           isShrunk,
	}, nil
}

// ackLevel updates the ack level for the given cluster
func (t *TaskAckManager) ackLevel(pollingCluster string, lastReadTaskID int64) {
	if err := t.ackLevels.UpdateQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, pollingCluster, persistence.NewImmediateTaskKey(lastReadTaskID)); err != nil {
		t.logger.Error("error updating replication level for shard", tag.Error(err), tag.OperationFailed)
	}

	if err := t.store.Ack(pollingCluster, lastReadTaskID); err != nil {
		t.logger.Error("error updating replication level for hydrated task store", tag.Error(err), tag.OperationFailed)
	}
}

// shrinkMessagesBySize shrinks the replication messages by removing the last replication task until the total size is allowed
func (t *TaskAckManager) shrinkMessagesBySize(msgs *types.ReplicationMessages) (bool, error) {
	// if there are no replication tasks, do nothing
	if len(msgs.ReplicationTasks) == 0 {
		return false, nil
	}

	maxSize := t.maxReplicationMessagesSize
	isShrunk := false

	for {
		totalSize := t.replicationMessagesSizeFn(msgs)

		// if the total size is allowed, return the replication messages
		if totalSize < maxSize {
			return isShrunk, nil
		}

		lastTask := msgs.ReplicationTasks[len(msgs.ReplicationTasks)-1]
		t.logger.Warn("Replication messages size is too large. Shrinking the messages by removing the last replication task",
			tag.ReplicationMessagesTotalSize(totalSize),
			tag.ReplicationMessagesMaxSize(maxSize),
			tag.ReplicationTaskID(lastTask.SourceTaskID),
			tag.ReplicationTaskCreationTime(lastTask.CreationTime),
		)

		// change HasMore to true to indicate that there are more tasks to be fetched
		msgs.HasMore = true

		// remove the last replication task
		msgs.ReplicationTasks = msgs.ReplicationTasks[:len(msgs.ReplicationTasks)-1]

		// set isShrunk to true to indicate that the replication messages have been shrunk
		isShrunk = true

		// should never happen, but just in case
		// if there are no more replication tasks, return an error
		if len(msgs.ReplicationTasks) == 0 {
			return isShrunk, fmt.Errorf("replication messages size is too large and cannot be shrunk anymore, shard will be stuck until the message size is reduced or max size is increased")

		}

		// update the last retrieved message ID to the new last task ID
		msgs.LastRetrievedMessageID = msgs.ReplicationTasks[len(msgs.ReplicationTasks)-1].SourceTaskID
	}
}
