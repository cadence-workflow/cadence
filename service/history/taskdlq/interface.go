// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -destination interface_mock.go github.com/uber/cadence/service/history/taskdlq HistoryTaskDLQStore,TaskExecutor

package taskdlq

import (
	"context"
	"time"

	"github.com/uber/cadence/common/persistence"
)

type (
	// HistoryTaskDLQStore defines the methods required from the persistence layer for the History Task DLQ Processor.
	HistoryTaskDLQStore interface {
		// GetAckLevels returns all DLQ partitions for a shard with their current ack levels.
		// Implementations must populate AckLevel.ExclusiveMaxTaskKey with the current ack level
		// of the source (normal) queue so that the processor does not scan tasks that may still
		// be in-flight in that queue.
		GetAckLevels(ctx context.Context, shardID int) ([]AckLevel, error)

		// GetAckLevelsForPartition returns ack levels for all task types within a specific
		// (domain, clusterAttributeScope, clusterAttributeName) partition.
		// Implementations must populate AckLevel.ExclusiveMaxTaskKey as described in GetAckLevels.
		GetAckLevelsForPartition(ctx context.Context, shardID int, domainID, clusterAttributeScope, clusterAttributeName string) ([]AckLevel, error)

		// GetTasks returns tasks from a DLQ partition starting at the inclusive min key.
		GetTasks(ctx context.Context, request GetTasksRequest) (GetTasksResponse, error)

		// UpdateAckLevel persists the new ack level for a partition.
		UpdateAckLevel(ctx context.Context, request UpdateAckLevelRequest) error

		// DeleteTasks removes tasks up to and including the given key from a DLQ partition.
		DeleteTasks(ctx context.Context, request DeleteTasksRequest) error
	}

	// TaskExecutor executes a single DLQ task synchronously.
	// Callers should implement this using the historyqueuev2 executor machinery.
	TaskExecutor interface {
		Execute(ctx context.Context, task persistence.Task) error
		// HandleErr classifies the error returned by Execute. It returns nil if the error
		// is ackable (the task can be skipped and processing continues to the next task),
		// or a non-nil error if execution must stop and the ack level must not advance past
		// this task.
		HandleErr(err error) error
	}

	// AckLevel identifies one DLQ partition and its current processing watermark.
	AckLevel struct {
		ShardID               int
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		// TaskType is a persistence.HistoryTaskCategoryID* constant
		// (1=Transfer, 2=Timer, 3=Replication).
		TaskType             int
		AckLevelVisibilityTS time.Time
		AckLevelTaskID       int64
		// ExclusiveMaxTaskKey bounds the DLQ scan to tasks that were committed to the DLQ
		// before the source queue had processed this far.
		ExclusiveMaxTaskKey persistence.HistoryTaskKey
	}

	// GetTasksRequest specifies what tasks to fetch from a DLQ partition.
	GetTasksRequest struct {
		ShardID               int
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		TaskType              int
		InclusiveMinTaskKey   persistence.HistoryTaskKey
		ExclusiveMaxTaskKey   persistence.HistoryTaskKey
		PageSize              int
		NextPageToken         []byte
	}

	// GetTasksResponse carries tasks returned from the DLQ store.
	GetTasksResponse struct {
		Tasks         []persistence.Task
		NextPageToken []byte
	}

	// UpdateAckLevelRequest specifies the new ack watermark for a partition.
	UpdateAckLevelRequest struct {
		ShardID               int
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		TaskType              int
		AckLevelVisibilityTS  time.Time
		AckLevelTaskID        int64
	}

	// DeleteTasksRequest asks the store to remove tasks with key < ExclusiveMaxTaskKey from a DLQ partition.
	DeleteTasksRequest struct {
		ShardID               int
		DomainID              string
		ClusterAttributeScope string
		ClusterAttributeName  string
		TaskType              int
		ExclusiveMaxTaskKey   persistence.HistoryTaskKey
	}
)
