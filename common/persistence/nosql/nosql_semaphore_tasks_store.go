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

// ClaimSemaphoreTaskBucket claims or renews single-writer ownership of a bucket by bumping the control
// row's range_id, creating the control row if the bucket is used for the first time.
func (m *nosqlSemaphoreTaskStore) ClaimSemaphoreTaskBucket(
	ctx context.Context,
	request *persistence.ClaimSemaphoreTaskBucketRequest,
) (*persistence.ClaimSemaphoreTaskBucketResponse, error) {
	now := time.Now().UTC()
	current, selectErr := m.db.SelectSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlFilter{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
	})

	if selectErr != nil {
		if m.db.IsNotFoundError(selectErr) { // first use of this bucket
			// Nothing deletes a semaphore control row — the range delete is scoped to task rows
			// (type 0), and the control row's task_id is negative. A renew against a missing row
			// therefore means the row was lost out of band. Recreating it would report a successful
			// renew of a bucket silently reset to ack_level 0. The re-grants that follow are caught
			// by the owner-row CAS, so this is about surfacing the loss, not preventing corruption.
			if request.RangeID > 0 {
				return nil, &persistence.ConditionFailedError{
					Msg: fmt.Sprintf("ClaimSemaphoreTaskBucket: control row missing for semaphore:%v, bucket:%v, haveRangeID:%v",
						request.SemaphoreName, request.Bucket, request.RangeID),
				}
			}
			newRow := &nosqlplugin.SemaphoreTaskControlRow{
				DomainID:      request.DomainID,
				SemaphoreName: request.SemaphoreName,
				Bucket:        request.Bucket,
				RangeID:       initialSemaphoreRangeID,
				AckLevel:      initialSemaphoreAckLevel,
				CreatedTime:   now,
			}
			if err := m.db.InsertSemaphoreTaskControlRow(ctx, newRow); err != nil {
				return nil, m.toConditionOrCommonError("ClaimSemaphoreTaskBucket", err)
			}
			return &persistence.ClaimSemaphoreTaskBucketResponse{
				RangeID:  newRow.RangeID,
				AckLevel: newRow.AckLevel,
			}, nil
		}
		return nil, convertCommonErrors(m.db, "ClaimSemaphoreTaskBucket", selectErr)
	}

	// A renew (RangeID > 0) asserts "I still hold N"; a mismatch means another host took the
	// bucket. This error is how the stale owner finds out, and what makes it unload — without
	// it the renew would bump range_id and take the bucket back, fencing out the live owner.
	if request.RangeID > 0 && request.RangeID != current.RangeID {
		return nil, &persistence.ConditionFailedError{
			Msg: fmt.Sprintf("ClaimSemaphoreTaskBucket:renew failed: semaphore:%v, bucket:%v, haveRangeID:%v, gotRangeID:%v",
				request.SemaphoreName, request.Bucket, request.RangeID, current.RangeID),
		}
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
		return nil, m.toConditionOrCommonError("ClaimSemaphoreTaskBucket", err)
	}
	return &persistence.ClaimSemaphoreTaskBucketResponse{
		RangeID:  newRangeID,
		AckLevel: current.AckLevel,
	}, nil
}

// GetSemaphoreTaskBucketState reads a bucket's control row (range_id, ack_level).
func (m *nosqlSemaphoreTaskStore) GetSemaphoreTaskBucketState(
	ctx context.Context,
	request *persistence.GetSemaphoreTaskBucketStateRequest,
) (*persistence.GetSemaphoreTaskBucketStateResponse, error) {
	row, err := m.db.SelectSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlFilter{
		DomainID:      request.DomainID,
		SemaphoreName: request.SemaphoreName,
		Bucket:        request.Bucket,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetSemaphoreTaskBucketState", err)
	}
	return &persistence.GetSemaphoreTaskBucketStateResponse{
		RangeID:  row.RangeID,
		AckLevel: row.AckLevel,
	}, nil
}

// UpdateSemaphoreTaskBucketState advances the ack_level cursor, fenced by the current RangeID.
func (m *nosqlSemaphoreTaskStore) UpdateSemaphoreTaskBucketState(
	ctx context.Context,
	request *persistence.UpdateSemaphoreTaskBucketStateRequest,
) (*persistence.UpdateSemaphoreTaskBucketStateResponse, error) {
	if err := m.db.UpdateSemaphoreTaskControlRow(ctx, &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:         request.DomainID,
		SemaphoreName:    request.SemaphoreName,
		Bucket:           request.Bucket,
		RangeID:          request.RangeID,
		AckLevel:         request.AckLevel,
		CurrentTimeStamp: time.Now().UTC(),
	}, request.RangeID); err != nil {
		return nil, m.toConditionOrCommonError("UpdateSemaphoreTaskBucketState", err)
	}
	return &persistence.UpdateSemaphoreTaskBucketStateResponse{}, nil
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
	// match nothing.
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

// RangeCompleteSemaphoreTasks range-deletes granted/expired tasks in (ReadLevel, AckLevel].
func (m *nosqlSemaphoreTaskStore) RangeCompleteSemaphoreTasks(
	ctx context.Context,
	request *persistence.RangeCompleteSemaphoreTasksRequest,
) (*persistence.RangeCompleteSemaphoreTasksResponse, error) {
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
		return nil, convertCommonErrors(m.db, "RangeCompleteSemaphoreTasks", err)
	}
	return &persistence.RangeCompleteSemaphoreTasksResponse{RowsDeleted: rowsDeleted}, nil
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
