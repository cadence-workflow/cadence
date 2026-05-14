// Copyright (c) 2025 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -destination history_task_dlq_manager_mock.go github.com/uber/cadence/common/persistence HistoryTaskSerializer

package persistence

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

// HistoryTaskSerializer serializes and deserializes history tasks. It is a subset of
// serialization.TaskSerializer, declared here to avoid an import cycle between the
// persistence and persistence/serialization packages.
type HistoryTaskSerializer interface {
	SerializeTask(HistoryTaskCategory, Task) (DataBlob, error)
	DeserializeTask(HistoryTaskCategory, *DataBlob) (Task, error)
}

type historyTaskDLQManagerImpl struct {
	persistence    HistoryDLQTaskStore
	taskSerializer HistoryTaskSerializer
	logger         log.Logger
	timeSrc        clock.TimeSource
}

// NewHistoryTaskDLQManager creates a new HistoryTaskDLQManager.
func NewHistoryTaskDLQManager(
	persistence HistoryDLQTaskStore,
	taskSerializer HistoryTaskSerializer,
	logger log.Logger,
) HistoryTaskDLQManager {
	return &historyTaskDLQManagerImpl{
		persistence:    persistence,
		taskSerializer: taskSerializer,
		logger:         logger,
		timeSrc:        clock.NewRealTimeSource(),
	}
}

// CreateHistoryDLQTask serializes the task and writes it to the DLQ store.
func (m *historyTaskDLQManagerImpl) CreateHistoryDLQTask(
	ctx context.Context,
	request CreateHistoryDLQTaskRequest,
) error {
	blob, err := m.taskSerializer.SerializeTask(request.Task.GetTaskCategory(), request.Task)
	if err != nil {
		return fmt.Errorf("failed to serialize history DLQ task: %w", err)
	}
	return m.persistence.CreateHistoryDLQTask(ctx, InternalCreateHistoryDLQTaskRequest{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
		TaskType:              request.Task.GetTaskType(),
		TaskID:                request.Task.GetTaskID(),
		WorkflowID:            request.Task.GetWorkflowID(),
		RunID:                 request.Task.GetRunID(),
		Version:               request.Task.GetVersion(),
		VisibilityTimestamp:   request.Task.GetVisibilityTimestamp(),
		CreatedAt:             m.timeSrc.Now().UTC(),
		TaskBlob:              &DataBlob{Data: blob.Data, Encoding: blob.Encoding},
	})
}

// GetAckLevels returns DLQ partitions for the given shard and task category with their stored ack levels.
// Optionally filter to a specific partition by setting DomainID/ClusterAttributeScope/ClusterAttributeName.
func (m *historyTaskDLQManagerImpl) GetAckLevels(
	ctx context.Context,
	request HistoryDLQGetAckLevelsRequest,
) ([]HistoryDLQAckLevel, error) {
	resp, err := m.persistence.GetHistoryDLQAckLevels(ctx, request)
	if err != nil {
		return nil, err
	}
	filterByCategory := request.TaskCategory.ID() != 0
	taskTypeID := request.TaskCategory.ID()
	out := make([]HistoryDLQAckLevel, 0, len(resp.AckLevels))
	for _, row := range resp.AckLevels {
		if filterByCategory && row.TaskCategory != taskTypeID {
			continue
		}
		category, err := HistoryTaskCategoryFromID(row.TaskCategory)
		if err != nil {
			m.logger.Warn("skipping ack level with unknown task category ID",
				tag.ShardID(row.ShardID),
				tag.Dynamic("task-category-id", row.TaskCategory),
			)
			continue
		}
		out = append(out, HistoryDLQAckLevel{
			ShardID:               row.ShardID,
			DomainID:              row.DomainID,
			ClusterAttributeScope: row.ClusterAttributeScope,
			ClusterAttributeName:  row.ClusterAttributeName,
			TaskCategory:          category,
			AckLevelVisibilityTS:  row.AckLevelVisibilityTS,
			AckLevelTaskID:        row.AckLevelTaskID,
		})
	}
	return out, nil
}

// GetTasks returns deserialized tasks from a DLQ partition.
func (m *historyTaskDLQManagerImpl) GetTasks(
	ctx context.Context,
	request HistoryDLQGetTasksRequest,
) (HistoryDLQGetTasksResponse, error) {
	resp, err := m.persistence.GetHistoryDLQTasks(ctx, request)
	if err != nil {
		return HistoryDLQGetTasksResponse{}, err
	}

	tasks := make([]Task, 0, len(resp.Tasks))
	for _, raw := range resp.Tasks {
		task, err := m.taskSerializer.DeserializeTask(request.TaskCategory, raw.TaskPayload)
		if err != nil {
			return HistoryDLQGetTasksResponse{}, fmt.Errorf("failed to deserialize history DLQ task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return HistoryDLQGetTasksResponse{Tasks: tasks, NextPageToken: resp.NextPageToken}, nil
}

// UpdateAckLevel persists the new ack level for a partition.
func (m *historyTaskDLQManagerImpl) UpdateAckLevel(
	ctx context.Context,
	request HistoryDLQUpdateAckLevelRequest,
) error {
	return m.persistence.UpdateHistoryDLQAckLevel(ctx, InternalUpdateHistoryDLQAckLevelRequest{
		Row: InternalHistoryDLQAckLevel{
			ShardID:               request.ShardID,
			DomainID:              request.DomainID,
			ClusterAttributeScope: request.ClusterAttributeScope,
			ClusterAttributeName:  request.ClusterAttributeName,
			TaskCategory:          request.TaskCategory.ID(),
			AckLevelVisibilityTS:  request.UpdatedInclusiveReadLevel.GetScheduledTime(),
			AckLevelTaskID:        request.UpdatedInclusiveReadLevel.GetTaskID(),
			LastUpdatedAt:         m.timeSrc.Now().UTC(),
		},
	})
}

// DeleteTasks removes tasks with key < ExclusiveMaxTaskKey from a DLQ partition.
func (m *historyTaskDLQManagerImpl) DeleteTasks(
	ctx context.Context,
	request HistoryDLQDeleteTasksRequest,
) error {
	return m.persistence.RangeDeleteHistoryDLQTasks(ctx, request)
}

func (m *historyTaskDLQManagerImpl) GetName() string { return m.persistence.GetName() }
func (m *historyTaskDLQManagerImpl) Close()          { m.persistence.Close() }
