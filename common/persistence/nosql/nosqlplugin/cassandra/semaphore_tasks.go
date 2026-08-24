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

package cassandra

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

const (
	// Row types for the `type` clustering column of semaphore_tasks.
	rowTypeSemaphoreTask    = 0 // task row: one queued acquire waiting for a token
	rowTypeSemaphoreControl = 1 // control row: carries the range_id fence and ack_level cursor

	// semaphoreControlTaskID is the sentinel task_id of the single control row in a bucket.
	// It is negative so it can never collide with a real (monotonically increasing) task_id.
	semaphoreControlTaskID = -12345
)

// SelectSemaphoreTaskControlRow returns a bucket's control row (range_id, ack_level).
func (db *CDB) SelectSemaphoreTaskControlRow(ctx context.Context, filter *nosqlplugin.SemaphoreTaskControlFilter) (*nosqlplugin.SemaphoreTaskControlRow, error) {
	query := db.session.Query(templateGetSemaphoreTaskControlRowQuery,
		filter.DomainID,
		filter.SemaphoreName,
		filter.Bucket,
		rowTypeSemaphoreControl,
		semaphoreControlTaskID,
	).WithContext(ctx)

	var rangeID, ackLevel int64
	if err := query.Scan(&rangeID, &ackLevel); err != nil {
		return nil, err
	}

	return &nosqlplugin.SemaphoreTaskControlRow{
		DomainID:      filter.DomainID,
		SemaphoreName: filter.SemaphoreName,
		Bucket:        filter.Bucket,
		RangeID:       rangeID,
		AckLevel:      ackLevel,
	}, nil
}

// InsertSemaphoreTaskControlRow inserts the control row with INSERT ... IF NOT EXISTS.
func (db *CDB) InsertSemaphoreTaskControlRow(ctx context.Context, row *nosqlplugin.SemaphoreTaskControlRow) error {
	query := db.session.Query(templateInsertSemaphoreTaskControlRowQuery,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreControl,
		semaphoreControlTaskID,
		row.RangeID,
		row.AckLevel,
		row.CreatedTime,
	).WithContext(ctx)

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		return err
	}
	return handleTaskListAppliedError(applied, previous)
}

// UpdateSemaphoreTaskControlRow updates range_id and ack_level, fenced by previousRangeID.
func (db *CDB) UpdateSemaphoreTaskControlRow(ctx context.Context, row *nosqlplugin.SemaphoreTaskControlRow, previousRangeID int64) error {
	query := db.session.Query(templateUpdateSemaphoreTaskControlRowQuery,
		row.RangeID,
		row.AckLevel,
		row.DomainID,
		row.SemaphoreName,
		row.Bucket,
		rowTypeSemaphoreControl,
		semaphoreControlTaskID,
		previousRangeID,
	).WithContext(ctx)

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		return err
	}
	return handleTaskListAppliedError(applied, previous)
}

// InsertSemaphoreTasks inserts a batch of task rows in a single LWT batch fenced by the
// control row's range_id, so a stale writer that has lost the bucket cannot enqueue.
func (db *CDB) InsertSemaphoreTasks(
	ctx context.Context,
	tasks []*nosqlplugin.SemaphoreTaskRow,
	controlCondition *nosqlplugin.SemaphoreTaskControlRow,
) error {
	if len(tasks) == 0 {
		// Nothing to write. Skip the batch rather than issuing a bare fence CAS, which would
		// cost a Paxos round to assert ownership no caller asked about.
		return nil
	}

	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	for _, w := range tasks {
		batch.Query(templateCreateSemaphoreTaskQuery,
			w.DomainID,
			w.SemaphoreName,
			w.Bucket,
			rowTypeSemaphoreTask,
			w.TaskID,
			w.WorkflowID, // task.workflow_id
			w.RunID,      // task.run_id
			w.HoldID,     // task.hold_id
			w.AcquireDeadline,
			w.CreatedTime,
		)
	}

	// Re-assert the control row's range_id (no-op write to the same value) as the batch fence.
	batch.Query(templateUpdateSemaphoreTaskControlRangeIDQuery,
		controlCondition.RangeID,
		controlCondition.DomainID,
		controlCondition.SemaphoreName,
		controlCondition.Bucket,
		rowTypeSemaphoreControl,
		semaphoreControlTaskID,
		controlCondition.RangeID,
	)

	previous := make(map[string]interface{})
	applied, _, err := db.session.MapExecuteBatchCAS(batch, previous)
	if err != nil {
		return err
	}
	return handleTaskListAppliedError(applied, previous)
}

// SelectSemaphoreTasks returns task rows in (ExclusiveMinTaskID, InclusiveMaxTaskID], paged.
func (db *CDB) SelectSemaphoreTasks(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) ([]*nosqlplugin.SemaphoreTaskRow, error) {
	query := db.session.Query(templateGetSemaphoreTasksQuery,
		filter.DomainID,
		filter.SemaphoreName,
		filter.Bucket,
		rowTypeSemaphoreTask,
		filter.ExclusiveMinTaskID,
		filter.InclusiveMaxTaskID,
	).PageSize(filter.BatchSize).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, fmt.Errorf("SelectSemaphoreTasks operation failed. Not able to create query iterator")
	}

	var response []*nosqlplugin.SemaphoreTaskRow
	row := make(map[string]interface{})
	for iter.MapScan(row) {
		taskID, ok := row["task_id"].(int64)
		if !ok { // no clustering key, so nothing identifies a task
			continue
		}
		taskMap, ok := row["task"].(map[string]interface{})
		if !ok {
			// The `task` column is null, so there is no payload to read. A plain assertion
			// panics here; an error would stall the bucket, since the reader keeps retrying
			// the same row. Skipping drops a queued acquire, so log it.
			db.logger.Warn("skipping semaphore task row with no task UDT",
				tag.WorkflowDomainID(filter.DomainID), tag.TaskID(taskID))
			continue
		}

		w := createSemaphoreTaskInfo(taskMap)
		w.DomainID = filter.DomainID
		w.SemaphoreName = filter.SemaphoreName
		w.Bucket = filter.Bucket
		w.TaskID = taskID
		if deadline, ok := row["acquire_deadline"].(time.Time); ok && !deadline.IsZero() {
			d := deadline
			w.AcquireDeadline = &d
		}
		if createdTime, ok := row["created_time"].(time.Time); ok {
			w.CreatedTime = createdTime
		}

		response = append(response, w)
		if len(response) == filter.BatchSize {
			break
		}
		row = make(map[string]interface{}) // reinitialize; MapScan fails to reuse a populated map
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}
	return response, nil
}

// createSemaphoreTaskInfo scans the frozen<semaphore_task> UDT map into a task row.
func createSemaphoreTaskInfo(result map[string]interface{}) *nosqlplugin.SemaphoreTaskRow {
	w := &nosqlplugin.SemaphoreTaskRow{}
	for k, v := range result {
		switch k {
		case "workflow_id":
			w.WorkflowID = v.(string)
		case "run_id":
			w.RunID = v.(gocql.UUID).String()
		case "hold_id":
			w.HoldID = v.(int64)
		}
	}
	return w
}

// GetSemaphoreTasksCount returns the number of task rows with task_id > ExclusiveMinTaskID.
func (db *CDB) GetSemaphoreTasksCount(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (int64, error) {
	query := db.session.Query(templateGetSemaphoreTasksCountQuery,
		filter.DomainID,
		filter.SemaphoreName,
		filter.Bucket,
		rowTypeSemaphoreTask,
		filter.ExclusiveMinTaskID,
	).WithContext(ctx)

	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return 0, err
	}
	return result["count"].(int64), nil
}

// RangeDeleteSemaphoreTasks deletes task rows in (ExclusiveMinTaskID, InclusiveMaxTaskID].
// NOTE: this ignores BatchSize and either deletes all tasks in the range or returns an error,
// because Cassandra cannot report the number of rows deleted.
func (db *CDB) RangeDeleteSemaphoreTasks(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (rowsDeleted int, err error) {
	query := db.session.Query(templateDeleteSemaphoreTasksLessThanQuery,
		filter.DomainID,
		filter.SemaphoreName,
		filter.Bucket,
		rowTypeSemaphoreTask,
		filter.ExclusiveMinTaskID,
		filter.InclusiveMaxTaskID,
	).WithContext(ctx)
	return persistence.UnknownNumRowsAffected, db.executeWithConsistencyAll(query)
}
