// Copyright (c) 2017 Uber Technologies, Inc.
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

package mysql

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	executionsColumns = `shard_id, domain_id, workflow_id, run_id, next_event_id, last_write_version, data, data_encoding`

	createExecutionQuery = `INSERT INTO executions(` + executionsColumns + `)
 VALUES(:shard_id, :domain_id, :workflow_id, :run_id, :next_event_id, :last_write_version, :data, :data_encoding)`

	updateExecutionQuery = `UPDATE executions SET
 next_event_id = :next_event_id, last_write_version = :last_write_version, data = :data, data_encoding = :data_encoding
 WHERE shard_id = :shard_id AND domain_id = :domain_id AND workflow_id = :workflow_id AND run_id = :run_id`

	getExecutionQuery = `SELECT ` + executionsColumns + ` FROM executions
 WHERE shard_id = ? AND domain_id = ? AND workflow_id = ? AND run_id = ?`

	listExecutionQuery = `SELECT ` + executionsColumns + ` FROM executions
 WHERE shard_id = ? AND workflow_id > ? ORDER BY workflow_id LIMIT ?`

	deleteExecutionQuery = `DELETE FROM executions
 WHERE shard_id = ? AND domain_id = ? AND workflow_id = ? AND run_id = ?`

	lockExecutionQueryBase = `SELECT next_event_id FROM executions
 WHERE shard_id = ? AND domain_id = ? AND workflow_id = ? AND run_id = ?`

	writeLockExecutionQuery = lockExecutionQueryBase + ` FOR UPDATE`
	readLockExecutionQuery  = lockExecutionQueryBase + ` LOCK IN SHARE MODE`

	createCurrentExecutionQuery = `INSERT INTO current_executions
(shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version) VALUES
(:shard_id, :domain_id, :workflow_id, :run_id, :create_request_id, :state, :close_status, :start_version, :last_write_version)`

	deleteCurrentExecutionQuery = "DELETE FROM current_executions WHERE shard_id=? AND domain_id=? AND workflow_id=? AND run_id=?"

	getCurrentExecutionQuery = `SELECT
shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version
FROM current_executions WHERE shard_id = ? AND domain_id = ? AND workflow_id = ?`

	lockCurrentExecutionJoinExecutionsQuery = `SELECT
ce.shard_id, ce.domain_id, ce.workflow_id, ce.run_id, ce.create_request_id, ce.state, ce.close_status, ce.start_version, e.last_write_version
FROM current_executions ce
INNER JOIN executions e ON e.shard_id = ce.shard_id AND e.domain_id = ce.domain_id AND e.workflow_id = ce.workflow_id AND e.run_id = ce.run_id
WHERE ce.shard_id = ? AND ce.domain_id = ? AND ce.workflow_id = ? FOR UPDATE`

	lockCurrentExecutionQuery = getCurrentExecutionQuery + ` FOR UPDATE`

	updateCurrentExecutionsQuery = `UPDATE current_executions SET
run_id = :run_id,
create_request_id = :create_request_id,
state = :state,
close_status = :close_status,
start_version = :start_version,
last_write_version = :last_write_version
WHERE
shard_id = :shard_id AND
domain_id = :domain_id AND
workflow_id = :workflow_id
`

	getTransferTasksQuery = `SELECT task_id, data, data_encoding
FROM transfer_tasks WHERE shard_id = ? AND task_id > ? AND task_id <= ? ORDER BY shard_id, task_id LIMIT ?`

	createTransferTasksQuery = `INSERT INTO transfer_tasks(shard_id, task_id, data, data_encoding)
 VALUES(:shard_id, :task_id, :data, :data_encoding)`

	deleteTransferTaskQuery             = `DELETE FROM transfer_tasks WHERE shard_id = ? AND task_id = ?`
	rangeDeleteTransferTaskQuery        = `DELETE FROM transfer_tasks WHERE shard_id = ? AND task_id > ? AND task_id <= ?`
	rangeDeleteTransferTaskByBatchQuery = rangeDeleteTransferTaskQuery + ` ORDER BY task_id LIMIT ?`

	getCrossClusterTasksQuery = `SELECT task_id, data, data_encoding
FROM cross_cluster_tasks WHERE target_cluster = ? AND shard_id = ? AND task_id > ? AND task_id <= ? ORDER BY task_id LIMIT ?`

	createCrossClusterTasksQuery = `INSERT INTO cross_cluster_tasks(target_cluster, shard_id, task_id, data, data_encoding)
VALUES(:target_cluster, :shard_id, :task_id, :data, :data_encoding)`

	deleteCrossClusterTaskQuery             = `DELETE FROM cross_cluster_tasks WHERE target_cluster = ? AND shard_id = ? AND task_id = ?`
	rangeDeleteCrossClusterTaskQuery        = `DELETE FROM cross_cluster_tasks WHERE target_cluster = ? AND shard_id = ? AND task_id > ? AND task_id <= ?`
	rangeDeleteCrossClusterTaskByBatchQuery = rangeDeleteCrossClusterTaskQuery + ` ORDER BY task_id LIMIT ?`

	createTimerTasksQuery = `INSERT INTO timer_tasks (shard_id, visibility_timestamp, task_id, data, data_encoding)
  VALUES (:shard_id, :visibility_timestamp, :task_id, :data, :data_encoding)`

	getTimerTasksQuery = `SELECT visibility_timestamp, task_id, data, data_encoding FROM timer_tasks
  WHERE shard_id = ?
  AND ((visibility_timestamp >= ? AND task_id >= ?) OR visibility_timestamp > ?)
  AND visibility_timestamp < ?
  ORDER BY visibility_timestamp,task_id LIMIT ?`

	deleteTimerTaskQuery             = `DELETE FROM timer_tasks WHERE shard_id = ? AND visibility_timestamp = ? AND task_id = ?`
	rangeDeleteTimerTaskQuery        = `DELETE FROM timer_tasks WHERE shard_id = ? AND visibility_timestamp >= ? AND visibility_timestamp < ?`
	rangeDeleteTimerTaskByBatchQuery = rangeDeleteTimerTaskQuery + ` ORDER BY visibility_timestamp,task_id LIMIT ?`

	createReplicationTasksQuery = `INSERT INTO replication_tasks (shard_id, task_id, data, data_encoding)
  VALUES(:shard_id, :task_id, :data, :data_encoding)`

	getReplicationTasksQuery = `SELECT task_id, data, data_encoding FROM replication_tasks WHERE
shard_id = ? AND
task_id > ? AND
task_id <= ?
ORDER BY task_id LIMIT ?`

	deleteReplicationTaskQuery             = `DELETE FROM replication_tasks WHERE shard_id = ? AND task_id = ?`
	rangeDeleteReplicationTaskQuery        = `DELETE FROM replication_tasks WHERE shard_id = ? AND task_id <= ?`
	rangeDeleteReplicationTaskByBatchQuery = rangeDeleteReplicationTaskQuery + ` ORDER BY task_id LIMIT ?`

	getReplicationTasksDLQQuery = `SELECT task_id, data, data_encoding FROM replication_tasks_dlq WHERE
source_cluster_name = ? AND
shard_id = ? AND
task_id > ? AND
task_id <= ?
ORDER BY task_id LIMIT ?`

	getReplicationTaskDLQQuery = `SELECT count(1) as count FROM replication_tasks_dlq WHERE
source_cluster_name = ? AND
shard_id = ?`

	bufferedEventsColumns     = `shard_id, domain_id, workflow_id, run_id, data, data_encoding`
	createBufferedEventsQuery = `INSERT INTO buffered_events(` + bufferedEventsColumns + `)
VALUES (:shard_id, :domain_id, :workflow_id, :run_id, :data, :data_encoding)`

	deleteBufferedEventsQuery = `DELETE FROM buffered_events WHERE shard_id=? AND domain_id=? AND workflow_id=? AND run_id=?`
	getBufferedEventsQuery    = `SELECT data, data_encoding FROM buffered_events WHERE
shard_id=? AND domain_id=? AND workflow_id=? AND run_id=?`

	insertReplicationTaskDLQQuery = `
INSERT INTO replication_tasks_dlq
            (source_cluster_name,
             shard_id,
             task_id,
             data,
             data_encoding)
VALUES     (:source_cluster_name,
            :shard_id,
            :task_id,
            :data,
            :data_encoding)
`
	deleteReplicationTaskFromDLQQuery = `
	DELETE FROM replication_tasks_dlq
		WHERE source_cluster_name = ?
		AND shard_id = ?
		AND task_id = ?`

	rangeDeleteReplicationTaskFromDLQQuery = `
	DELETE FROM replication_tasks_dlq
		WHERE source_cluster_name = ?
		AND shard_id = ?
		AND task_id > ?
		AND task_id <= ?`
	rangeDeleteReplicationTaskFromDLQByBatchQuery = rangeDeleteReplicationTaskFromDLQQuery + ` ORDER BY task_id LIMIT ?`
)

// InsertIntoExecutions inserts a row into executions table
func (mdb *db) InsertIntoExecutions(ctx context.Context, row *sqlplugin.ExecutionsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(row.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createExecutionQuery, row)
}

// UpdateExecutions updates a single row in executions table
func (mdb *db) UpdateExecutions(ctx context.Context, row *sqlplugin.ExecutionsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(row.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, updateExecutionQuery, row)
}

// SelectFromExecutions reads a single row from executions table
// The list execution query result is order by workflow ID only. It may returns duplicate record with pagination.
func (mdb *db) SelectFromExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) ([]sqlplugin.ExecutionsRow, error) {
	var rows []sqlplugin.ExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	var err error
	if len(filter.DomainID) == 0 && filter.Size > 0 {
		err = mdb.driver.SelectContext(ctx, dbShardID, &rows, listExecutionQuery, filter.ShardID, filter.WorkflowID, filter.Size)
		if err != nil {
			return nil, err
		}
	} else {
		var row sqlplugin.ExecutionsRow
		err = mdb.driver.GetContext(ctx, dbShardID, &row, getExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return rows, err
}

// DeleteFromExecutions deletes a single row from executions table
func (mdb *db) DeleteFromExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// ReadLockExecutions acquires a write lock on a single row in executions table
func (mdb *db) ReadLockExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) (int, error) {
	var nextEventID int
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &nextEventID, readLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// WriteLockExecutions acquires a write lock on a single row in executions table
func (mdb *db) WriteLockExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) (int, error) {
	var nextEventID int
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &nextEventID, writeLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// InsertIntoCurrentExecutions inserts a single row into current_executions table
func (mdb *db) InsertIntoCurrentExecutions(ctx context.Context, row *sqlplugin.CurrentExecutionsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(row.ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createCurrentExecutionQuery, row)
}

// UpdateCurrentExecutions updates a single row in current_executions table
func (mdb *db) UpdateCurrentExecutions(ctx context.Context, row *sqlplugin.CurrentExecutionsRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(row.ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, updateCurrentExecutionsQuery, row)
}

// SelectFromCurrentExecutions reads one or more rows from current_executions table
func (mdb *db) SelectFromCurrentExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) (*sqlplugin.CurrentExecutionsRow, error) {
	var row sqlplugin.CurrentExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &row, getCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return &row, err
}

// DeleteFromCurrentExecutions deletes a single row in current_executions table
func (mdb *db) DeleteFromCurrentExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// LockCurrentExecutions acquires a write lock on a single row in current_executions table
func (mdb *db) LockCurrentExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) (*sqlplugin.CurrentExecutionsRow, error) {
	var row sqlplugin.CurrentExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &row, lockCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return &row, err
}

// LockCurrentExecutionsJoinExecutions joins a row in current_executions with executions table and acquires a
// write lock on the result
func (mdb *db) LockCurrentExecutionsJoinExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) ([]sqlplugin.CurrentExecutionsRow, error) {
	var rows []sqlplugin.CurrentExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, lockCurrentExecutionJoinExecutionsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return rows, err
}

// InsertIntoTransferTasks inserts one or more rows into transfer_tasks table
func (mdb *db) InsertIntoTransferTasks(ctx context.Context, rows []sqlplugin.TransferTasksRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(rows[0].ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createTransferTasksQuery, rows)
}

// SelectFromTransferTasks reads one or more rows from transfer_tasks table
func (mdb *db) SelectFromTransferTasks(ctx context.Context, filter *sqlplugin.TransferTasksFilter) ([]sqlplugin.TransferTasksRow, error) {
	var rows []sqlplugin.TransferTasksRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getTransferTasksQuery, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	if err != nil {
		return nil, err
	}
	return rows, err
}

// DeleteFromTransferTasks deletes one row from transfer_tasks table
func (mdb *db) DeleteFromTransferTasks(ctx context.Context, filter *sqlplugin.TransferTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteTransferTaskQuery, filter.ShardID, filter.TaskID)
}

// RangeDeleteFromTransferTasks deletes multi rows from transfer_tasks table
func (mdb *db) RangeDeleteFromTransferTasks(ctx context.Context, filter *sqlplugin.TransferTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTransferTaskByBatchQuery, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTransferTaskQuery, filter.ShardID, filter.MinTaskID, filter.MaxTaskID)
}

// InsertIntoCrossClusterTasks inserts one or more rows into cross_cluster_tasks table
func (mdb *db) InsertIntoCrossClusterTasks(ctx context.Context, rows []sqlplugin.CrossClusterTasksRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(rows[0].ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createCrossClusterTasksQuery, rows)
}

// SelectFromCrossClusterTasks reads one or more rows from cross_cluster_tasks table
func (mdb *db) SelectFromCrossClusterTasks(ctx context.Context, filter *sqlplugin.CrossClusterTasksFilter) ([]sqlplugin.CrossClusterTasksRow, error) {
	var rows []sqlplugin.CrossClusterTasksRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getCrossClusterTasksQuery, filter.TargetCluster, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	if err != nil {
		return nil, err
	}
	return rows, err
}

// DeleteFromCrossClusterTasks deletes one row from cross_cluster_tasks table
func (mdb *db) DeleteFromCrossClusterTasks(ctx context.Context, filter *sqlplugin.CrossClusterTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteCrossClusterTaskQuery, filter.TargetCluster, filter.ShardID, filter.TaskID)
}

// RangeDeleteFromCrossClusterTasks deletes multi rows from cross_cluster_tasks table
func (mdb *db) RangeDeleteFromCrossClusterTasks(ctx context.Context, filter *sqlplugin.CrossClusterTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteCrossClusterTaskByBatchQuery, filter.TargetCluster, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteCrossClusterTaskQuery, filter.TargetCluster, filter.ShardID, filter.MinTaskID, filter.MaxTaskID)
}

// InsertIntoTimerTasks inserts one or more rows into timer_tasks table
func (mdb *db) InsertIntoTimerTasks(ctx context.Context, rows []sqlplugin.TimerTasksRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(rows[0].ShardID, mdb.GetTotalNumDBShards())
	for i := range rows {
		rows[i].VisibilityTimestamp = mdb.converter.ToMySQLDateTime(rows[i].VisibilityTimestamp)
	}
	return mdb.driver.NamedExecContext(ctx, dbShardID, createTimerTasksQuery, rows)
}

// SelectFromTimerTasks reads one or more rows from timer_tasks table
func (mdb *db) SelectFromTimerTasks(ctx context.Context, filter *sqlplugin.TimerTasksFilter) ([]sqlplugin.TimerTasksRow, error) {
	var rows []sqlplugin.TimerTasksRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	filter.MinVisibilityTimestamp = mdb.converter.ToMySQLDateTime(filter.MinVisibilityTimestamp)
	filter.MaxVisibilityTimestamp = mdb.converter.ToMySQLDateTime(filter.MaxVisibilityTimestamp)
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getTimerTasksQuery, filter.ShardID, filter.MinVisibilityTimestamp,
		filter.TaskID, filter.MinVisibilityTimestamp, filter.MaxVisibilityTimestamp, filter.PageSize)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].VisibilityTimestamp = mdb.converter.FromMySQLDateTime(rows[i].VisibilityTimestamp)
	}
	return rows, err
}

// DeleteFromTimerTasks deletes one row from timer_tasks table
func (mdb *db) DeleteFromTimerTasks(ctx context.Context, filter *sqlplugin.TimerTasksFilter) (sql.Result, error) {
	filter.VisibilityTimestamp = mdb.converter.ToMySQLDateTime(filter.VisibilityTimestamp)
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteTimerTaskQuery, filter.ShardID, filter.VisibilityTimestamp, filter.TaskID)
}

// RangeDeleteFromTimerTasks deletes multi rows from timer_tasks table
func (mdb *db) RangeDeleteFromTimerTasks(ctx context.Context, filter *sqlplugin.TimerTasksFilter) (sql.Result, error) {
	filter.MinVisibilityTimestamp = mdb.converter.ToMySQLDateTime(filter.MinVisibilityTimestamp)
	filter.MaxVisibilityTimestamp = mdb.converter.ToMySQLDateTime(filter.MaxVisibilityTimestamp)
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTimerTaskByBatchQuery, filter.ShardID, filter.MinVisibilityTimestamp, filter.MaxVisibilityTimestamp, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTimerTaskQuery, filter.ShardID, filter.MinVisibilityTimestamp, filter.MaxVisibilityTimestamp)
}

// InsertIntoBufferedEvents inserts one or more rows into buffered_events table
func (mdb *db) InsertIntoBufferedEvents(ctx context.Context, rows []sqlplugin.BufferedEventsRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(rows[0].ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createBufferedEventsQuery, rows)
}

// SelectFromBufferedEvents reads one or more rows from buffered_events table
func (mdb *db) SelectFromBufferedEvents(ctx context.Context, filter *sqlplugin.BufferedEventsFilter) ([]sqlplugin.BufferedEventsRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	var rows []sqlplugin.BufferedEventsRow
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getBufferedEventsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
		rows[i].ShardID = filter.ShardID
	}
	return rows, err
}

// DeleteFromBufferedEvents deletes one or more rows from buffered_events table
func (mdb *db) DeleteFromBufferedEvents(ctx context.Context, filter *sqlplugin.BufferedEventsFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteBufferedEventsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// InsertIntoReplicationTasks inserts one or more rows into replication_tasks table
func (mdb *db) InsertIntoReplicationTasks(ctx context.Context, rows []sqlplugin.ReplicationTasksRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(rows[0].ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, createReplicationTasksQuery, rows)
}

// SelectFromReplicationTasks reads one or more rows from replication_tasks table
func (mdb *db) SelectFromReplicationTasks(ctx context.Context, filter *sqlplugin.ReplicationTasksFilter) ([]sqlplugin.ReplicationTasksRow, error) {
	var rows []sqlplugin.ReplicationTasksRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, getReplicationTasksQuery, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	return rows, err
}

// DeleteFromReplicationTasks deletes one row from replication_tasks table
func (mdb *db) DeleteFromReplicationTasks(ctx context.Context, filter *sqlplugin.ReplicationTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteReplicationTaskQuery, filter.ShardID, filter.TaskID)
}

// RangeDeleteFromReplicationTasks deletes multi rows from replication_tasks table
func (mdb *db) RangeDeleteFromReplicationTasks(ctx context.Context, filter *sqlplugin.ReplicationTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteReplicationTaskByBatchQuery, filter.ShardID, filter.InclusiveEndTaskID, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteReplicationTaskQuery, filter.ShardID, filter.InclusiveEndTaskID)
}

// InsertIntoReplicationTasksDLQ inserts one or more rows into replication_tasks_dlq table
func (mdb *db) InsertIntoReplicationTasksDLQ(ctx context.Context, row *sqlplugin.ReplicationTaskDLQRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(row.ShardID, mdb.GetTotalNumDBShards())
	return mdb.driver.NamedExecContext(ctx, dbShardID, insertReplicationTaskDLQQuery, row)
}

// SelectFromReplicationTasksDLQ reads one or more rows from replication_tasks_dlq table
func (mdb *db) SelectFromReplicationTasksDLQ(ctx context.Context, filter *sqlplugin.ReplicationTasksDLQFilter) ([]sqlplugin.ReplicationTasksRow, error) {
	var rows []sqlplugin.ReplicationTasksRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(
		ctx,
		dbShardID,
		&rows,
		getReplicationTasksDLQQuery,
		filter.SourceClusterName,
		filter.ShardID,
		filter.MinTaskID,
		filter.MaxTaskID,
		filter.PageSize)
	return rows, err
}

// SelectFromReplicationDLQ reads one row from replication_tasks_dlq table
func (mdb *db) SelectFromReplicationDLQ(ctx context.Context, filter *sqlplugin.ReplicationTaskDLQFilter) (int64, error) {
	var size []int64
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if err := mdb.driver.SelectContext(
		ctx,
		dbShardID,
		&size,
		getReplicationTaskDLQQuery,
		filter.SourceClusterName,
		filter.ShardID,
	); err != nil {
		return 0, err
	}
	return size[0], nil
}

// DeleteMessageFromReplicationTasksDLQ deletes one row from replication_tasks_dlq table
func (mdb *db) DeleteMessageFromReplicationTasksDLQ(
	ctx context.Context,
	filter *sqlplugin.ReplicationTasksDLQFilter,
) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())

	return mdb.driver.ExecContext(
		ctx,
		dbShardID,
		deleteReplicationTaskFromDLQQuery,
		filter.SourceClusterName,
		filter.ShardID,
		filter.TaskID,
	)
}

// DeleteMessageFromReplicationTasksDLQ deletes one or more rows from replication_tasks_dlq table
func (mdb *db) RangeDeleteMessageFromReplicationTasksDLQ(
	ctx context.Context,
	filter *sqlplugin.ReplicationTasksDLQFilter,
) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(
			ctx,
			dbShardID,
			rangeDeleteReplicationTaskFromDLQByBatchQuery,
			filter.SourceClusterName,
			filter.ShardID,
			filter.TaskID,
			filter.InclusiveEndTaskID,
			filter.PageSize,
		)
	}

	return mdb.driver.ExecContext(
		ctx,
		dbShardID,
		rangeDeleteReplicationTaskFromDLQQuery,
		filter.SourceClusterName,
		filter.ShardID,
		filter.TaskID,
		filter.InclusiveEndTaskID,
	)
}
