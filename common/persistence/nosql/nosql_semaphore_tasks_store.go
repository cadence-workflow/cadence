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

package nosql

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	initialSemaphoreRangeID  = 1 // range_id of a freshly-created bucket control row
	initialSemaphoreAckLevel = 0 // ack_level of a freshly-created bucket control row
)

type nosqlSemaphoreTaskStore struct {
	nosqlStore
}

// newNoSQLSemaphoreTaskStore creates an instance of the SemaphoreTaskStore implementation
func newNoSQLSemaphoreTaskStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.SemaphoreTaskStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlSemaphoreTaskStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

// LeaseSemaphoreBucket claims or renews single-writer ownership of a bucket by bumping the control
// row's range_id, creating the control row if the bucket is used for the first time.
func (m *nosqlSemaphoreTaskStore) LeaseSemaphoreBucket(
	ctx context.Context,
	request *persistence.LeaseSemaphoreBucketRequest,
) (*persistence.LeaseSemaphoreBucketResponse, error) {
	now := time.Now().UTC()
	current, selectErr := m.db.SelectSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlFilter{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
	})

	if selectErr != nil {
		if m.db.IsNotFoundError(selectErr) { // first use of this bucket
			newRow := &nosqlplugin.SemaphoreTaskControlRow{
				DomainID:      request.DomainID,
				SemaphoreName: request.SemaphoreName,
				Bucket:        request.Bucket,
				RangeID:       initialSemaphoreRangeID,
				AckLevel:      initialSemaphoreAckLevel,
				CreatedTime:   now,
			}
			if err := m.db.InsertSemaphoreTaskControlRow(ctx, newRow); err != nil {
				return nil, m.toConditionOrCommonError("LeaseSemaphoreBucket", err)
			}
			return &persistence.LeaseSemaphoreBucketResponse{
				RangeID:  newRow.RangeID,
				AckLevel: newRow.AckLevel,
			}, nil
		}
		return nil, convertCommonErrors(m.db, "LeaseSemaphoreBucket", selectErr)
	}

	newRangeID := current.RangeID + 1
	if err := m.db.UpdateSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:         request.DomainID,
		SemaphoreName:    request.SemaphoreName,
		Bucket:           request.Bucket,
		RangeID:          newRangeID,
		AckLevel:         current.AckLevel,
		CurrentTimeStamp: now,
	}, current.RangeID); err != nil {
		return nil, m.toConditionOrCommonError("LeaseSemaphoreBucket", err)
	}
	return &persistence.LeaseSemaphoreBucketResponse{
		RangeID:  newRangeID,
		AckLevel: current.AckLevel,
	}, nil
}

// GetSemaphoreBucket reads a bucket's control row (range_id, ack_level).
func (m *nosqlSemaphoreTaskStore) GetSemaphoreBucket(
	ctx context.Context,
	request *persistence.GetSemaphoreBucketRequest,
) (*persistence.GetSemaphoreBucketResponse, error) {
	row, err := m.db.SelectSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlFilter{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreBucket", err)
	}
	return &persistence.GetSemaphoreBucketResponse{
		RangeID:  row.RangeID,
		AckLevel: row.AckLevel,
	}, nil
}

// UpdateSemaphoreBucket advances the ack_level cursor, fenced by the current RangeID.
func (m *nosqlSemaphoreTaskStore) UpdateSemaphoreBucket(
	ctx context.Context,
	request *persistence.UpdateSemaphoreBucketRequest,
) (*persistence.UpdateSemaphoreBucketResponse, error) {
	if err := m.db.UpdateSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:         request.DomainID,
		SemaphoreName:    request.SemaphoreName,
		Bucket:           request.Bucket,
		RangeID:          request.RangeID,
		AckLevel:         request.AckLevel,
		CurrentTimeStamp: time.Now().UTC(),
	}, request.RangeID); err != nil {
		return nil, m.toConditionOrCommonError("UpdateSemaphoreBucket", err)
	}
	return &persistence.UpdateSemaphoreBucketResponse{}, nil
}

// CreateSemaphoreTasks enqueues task rows, fenced by the bucket's RangeID.
func (m *nosqlSemaphoreTaskStore) CreateSemaphoreTasks(
	ctx context.Context,
	request *persistence.CreateSemaphoreTasksRequest,
) (*persistence.CreateSemaphoreTasksResponse, error) {
	tasks := make([]*nosqlplugin.SemaphoreTaskRow, 0, len(request.Tasks))
	for _, w := range request.Tasks {
		tasks = append(tasks, &nosqlplugin.SemaphoreTaskRow{
			DomainID:        request.DomainID,
			SemaphoreName:   request.SemaphoreName,
			Bucket:          request.Bucket,
			TaskID:          w.TaskID,
			WorkflowID:      w.WorkflowID,
			RunID:           w.RunID,
			HoldID:          w.HoldID,
			AcquireDeadline: w.AcquireDeadline,
			CreatedTime:     w.CreatedTime,
		})
	}
	control := &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
		RangeID:       request.RangeID,
	}
	if err := m.db.InsertSemaphoreTasks(ctx, tasks, control); err != nil {
		return nil, m.toConditionOrCommonError("CreateSemaphoreTasks", err)
	}
	return &persistence.CreateSemaphoreTasksResponse{}, nil
}

// GetSemaphoreTasks reads task rows in (ReadLevel, MaxReadLevel].
func (m *nosqlSemaphoreTaskStore) GetSemaphoreTasks(
	ctx context.Context,
	request *persistence.GetSemaphoreTasksRequest,
) (*persistence.GetSemaphoreTasksResponse, error) {
	// An inverted range is a legitimate transient state (the reader has caught up to the
	// writer), not a caller error, so return empty rather than issuing a query that can only
	// match nothing. Mirrors nosqlTaskStore.GetTasks.
	if request.ReadLevel > request.MaxReadLevel {
		return &persistence.GetSemaphoreTasksResponse{}, nil
	}

	rows, err := m.db.SelectSemaphoreTasks(ctx, &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      request.DomainID,
			SemaphoreName: request.SemaphoreName,
			Bucket:        request.Bucket,
		},
		ExclusiveMinTaskID: request.ReadLevel,
		InclusiveMaxTaskID: request.MaxReadLevel,
		BatchSize:          request.BatchSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreTasks", err)
	}

	tasks := make([]*persistence.SemaphoreTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, semaphoreTaskRowToTask(row))
	}
	return &persistence.GetSemaphoreTasksResponse{Tasks: tasks}, nil
}

// CompleteSemaphoreTasksLessThan range-deletes granted/expired tasks in (ReadLevel, AckLevel].
func (m *nosqlSemaphoreTaskStore) CompleteSemaphoreTasksLessThan(
	ctx context.Context,
	request *persistence.CompleteSemaphoreTasksLessThanRequest,
) (*persistence.CompleteSemaphoreTasksLessThanResponse, error) {
	rowsDeleted, err := m.db.RangeDeleteSemaphoreTasks(ctx, &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      request.DomainID,
			SemaphoreName: request.SemaphoreName,
			Bucket:        request.Bucket,
		},
		ExclusiveMinTaskID: request.ReadLevel,
		InclusiveMaxTaskID: request.AckLevel,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "CompleteSemaphoreTasksLessThan", err)
	}
	return &persistence.CompleteSemaphoreTasksLessThanResponse{RowsDeleted: rowsDeleted}, nil
}

// GetSemaphoreTasksCount counts task rows with task_id > ReadLevel.
func (m *nosqlSemaphoreTaskStore) GetSemaphoreTasksCount(
	ctx context.Context,
	request *persistence.GetSemaphoreTasksCountRequest,
) (*persistence.GetSemaphoreTasksCountResponse, error) {
	count, err := m.db.GetSemaphoreTasksCount(ctx, &nosqlplugin.SemaphoreTasksFilter{
		SemaphoreTaskControlFilter: nosqlplugin.SemaphoreTaskControlFilter{
			DomainID:      request.DomainID,
			SemaphoreName: request.SemaphoreName,
			Bucket:        request.Bucket,
		},
		ExclusiveMinTaskID: request.ReadLevel,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreTasksCount", err)
	}
	return &persistence.GetSemaphoreTasksCountResponse{Count: count}, nil
}

// toConditionOrCommonError maps a range_id fence failure (TaskOperationConditionFailure, returned by
// both the IF NOT EXISTS insert and the IF range_id=? update) to a *persistence.ConditionFailedError,
// and any other error via convertCommonErrors.
func (m *nosqlSemaphoreTaskStore) toConditionOrCommonError(op string, err error) error {
	if conditionFailure, ok := err.(*nosqlplugin.TaskOperationConditionFailure); ok {
		return &persistence.ConditionFailedError{
			Msg: fmt.Sprintf("%v: semaphore bucket ownership fence failed, gotRangeID:%v", op, conditionFailure.RangeID),
		}
	}
	return convertCommonErrors(m.db, op, err)
}

func semaphoreTaskRowToTask(row *nosqlplugin.SemaphoreTaskRow) *persistence.SemaphoreTask {
	return &persistence.SemaphoreTask{
		TaskID:          row.TaskID,
		WorkflowID:      row.WorkflowID,
		RunID:           row.RunID,
		HoldID:          row.HoldID,
		AcquireDeadline: row.AcquireDeadline,
		CreatedTime:     row.CreatedTime,
	}
}
