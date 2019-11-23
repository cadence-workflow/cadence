// Copyright (c) 2019 Uber Technologies, Inc.
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

package postgres

import (
	"database/sql"
	"github.com/uber/cadence/common/persistence/sql/storage/sqldb"
)

const (
	executionsColumns = `shard_id, domain_id, workflow_id, run_id, next_event_id, last_write_version, data, data_encoding`

	createExecutionQuery = `INSERT INTO executions(` + executionsColumns + `)
 VALUES(:shard_id, :domain_id, :workflow_id, :run_id, :next_event_id, :last_write_version, :data, :data_encoding)`

	updateExecutionQuery = `UPDATE executions SET
 next_event_id = :next_event_id, last_write_version = :last_write_version, data = :data, data_encoding = :data_encoding
 WHERE shard_id = :shard_id AND domain_id = :domain_id AND workflow_id = :workflow_id AND run_id = :run_id`

	getExecutionQuery = `SELECT ` + executionsColumns + ` FROM executions
 WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4`

	deleteExecutionQuery = `DELETE FROM executions 
 WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4`

	lockExecutionQueryBase = `SELECT next_event_id FROM executions 
 WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4`

	writeLockExecutionQuery = lockExecutionQueryBase + ` FOR UPDATE`
	readLockExecutionQuery  = lockExecutionQueryBase + ` FOR SHARE`

	createCurrentExecutionQuery = `INSERT INTO current_executions
(shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version) VALUES
(:shard_id, :domain_id, :workflow_id, :run_id, :create_request_id, :state, :close_status, :start_version, :last_write_version)`

	deleteCurrentExecutionQuery = "DELETE FROM current_executions WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4"

	getCurrentExecutionQuery = `SELECT
shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version
FROM current_executions WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3`

	lockCurrentExecutionJoinExecutionsQuery = `SELECT
ce.shard_id, ce.domain_id, ce.workflow_id, ce.run_id, ce.create_request_id, ce.state, ce.close_status, ce.start_version, e.last_write_version
FROM current_executions ce
INNER JOIN executions e ON e.shard_id = ce.shard_id AND e.domain_id = ce.domain_id AND e.workflow_id = ce.workflow_id AND e.run_id = ce.run_id
WHERE ce.shard_id = $1 AND ce.domain_id = $2 AND ce.workflow_id = $3 FOR UPDATE`

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
 FROM transfer_tasks WHERE shard_id = $1 AND task_id > $2 AND task_id <= $3`

	createTransferTasksQuery = `INSERT INTO transfer_tasks(shard_id, task_id, data, data_encoding) 
 VALUES(:shard_id, :task_id, :data, :data_encoding)`

	deleteTransferTaskQuery      = `DELETE FROM transfer_tasks WHERE shard_id = $1 AND task_id = $2`
	rangeDeleteTransferTaskQuery = `DELETE FROM transfer_tasks WHERE shard_id = $1 AND task_id > $2 AND task_id <= $3`

	createTimerTasksQuery = `INSERT INTO timer_tasks (shard_id, visibility_timestamp, task_id, data, data_encoding)
  VALUES (:shard_id, :visibility_timestamp, :task_id, :data, :data_encoding)`

	getTimerTasksQuery = `SELECT visibility_timestamp, task_id, data, data_encoding FROM timer_tasks 
  WHERE shard_id = $1 
  AND ((visibility_timestamp >= $2 AND task_id >= $3) OR visibility_timestamp > $4) 
  AND visibility_timestamp < $5
  ORDER BY visibility_timestamp,task_id LIMIT $6`

	deleteTimerTaskQuery      = `DELETE FROM timer_tasks WHERE shard_id = $1 AND visibility_timestamp = $2 AND task_id = $3`
	rangeDeleteTimerTaskQuery = `DELETE FROM timer_tasks WHERE shard_id = $1 AND visibility_timestamp >= $2 AND visibility_timestamp < $3`

	createReplicationTasksQuery = `INSERT INTO replication_tasks (shard_id, task_id, data, data_encoding) 
  VALUES(:shard_id, :task_id, :data, :data_encoding)`

	getReplicationTasksQuery = `SELECT task_id, data, data_encoding FROM replication_tasks WHERE 
shard_id = $1 AND
task_id > $2 AND
task_id <= $3 
ORDER BY task_id LIMIT $4`

	deleteReplicationTaskQuery = `DELETE FROM replication_tasks WHERE shard_id = $1 AND task_id = $2`

	getReplicationTasksDLQQuery = `SELECT task_id, data, data_encoding FROM replication_tasks_dlq WHERE 
source_cluster_name = $1 AND
shard_id = $2 AND
task_id > $3 AND
task_id <= $4
ORDER BY task_id LIMIT $5`

	bufferedEventsColumns     = `shard_id, domain_id, workflow_id, run_id, data, data_encoding`
	createBufferedEventsQuery = `INSERT INTO buffered_events(` + bufferedEventsColumns + `)
VALUES (:shard_id, :domain_id, :workflow_id, :run_id, :data, :data_encoding)`

	deleteBufferedEventsQuery = `DELETE FROM buffered_events WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4`
	getBufferedEventsQuery    = `SELECT data, data_encoding FROM buffered_events WHERE shard_id = $1 AND domain_id = $2 AND workflow_id = $3 AND run_id = $4`

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
)


// InsertIntoExecutions inserts a row into executions table
func (mdb *db) InsertIntoExecutions(row *sqldb.ExecutionsRow) (sql.Result, error) {
	return mdb.conn.NamedExec(createExecutionQuery, row)
}

// UpdateExecutions updates a single row in executions table
func (mdb *db) UpdateExecutions(row *sqldb.ExecutionsRow) (sql.Result, error) {
	return mdb.conn.NamedExec(updateExecutionQuery, row)
}

// SelectFromExecutions reads a single row from executions table
func (mdb *db) SelectFromExecutions(filter *sqldb.ExecutionsFilter) (*sqldb.ExecutionsRow, error) {
	var row sqldb.ExecutionsRow
	err := mdb.conn.Get(&row, getExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	if err != nil {
		return nil, err
	}
	return &row, err
}

// DeleteFromExecutions deletes a single row from executions table
func (mdb *db) DeleteFromExecutions(filter *sqldb.ExecutionsFilter) (sql.Result, error) {
	return mdb.conn.Exec(deleteExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// ReadLockExecutions acquires a write lock on a single row in executions table
func (mdb *db) ReadLockExecutions(filter *sqldb.ExecutionsFilter) (int, error) {
	var nextEventID int
	err := mdb.conn.Get(&nextEventID, readLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// WriteLockExecutions acquires a write lock on a single row in executions table
func (mdb *db) WriteLockExecutions(filter *sqldb.ExecutionsFilter) (int, error) {
	var nextEventID int
	err := mdb.conn.Get(&nextEventID, writeLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// InsertIntoCurrentExecutions inserts a single row into current_executions table
func (mdb *db) InsertIntoCurrentExecutions(row *sqldb.CurrentExecutionsRow) (sql.Result, error) {
	return mdb.conn.NamedExec(createCurrentExecutionQuery, row)
}

// UpdateCurrentExecutions updates a single row in current_executions table
func (mdb *db) UpdateCurrentExecutions(row *sqldb.CurrentExecutionsRow) (sql.Result, error) {
	return mdb.conn.NamedExec(updateCurrentExecutionsQuery, row)
}

// SelectFromCurrentExecutions reads one or more rows from current_executions table
func (mdb *db) SelectFromCurrentExecutions(filter *sqldb.CurrentExecutionsFilter) (*sqldb.CurrentExecutionsRow, error) {
	var row sqldb.CurrentExecutionsRow
	err := mdb.conn.Get(&row, getCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return &row, err
}

// DeleteFromCurrentExecutions deletes a single row in current_executions table
func (mdb *db) DeleteFromCurrentExecutions(filter *sqldb.CurrentExecutionsFilter) (sql.Result, error) {
	return mdb.conn.Exec(deleteCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// LockCurrentExecutions acquires a write lock on a single row in current_executions table
func (mdb *db) LockCurrentExecutions(filter *sqldb.CurrentExecutionsFilter) (*sqldb.CurrentExecutionsRow, error) {
	var row sqldb.CurrentExecutionsRow
	err := mdb.conn.Get(&row, lockCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return &row, err
}

// LockCurrentExecutionsJoinExecutions joins a row in current_executions with executions table and acquires a
// write lock on the result
func (mdb *db) LockCurrentExecutionsJoinExecutions(filter *sqldb.CurrentExecutionsFilter) ([]sqldb.CurrentExecutionsRow, error) {
	var rows []sqldb.CurrentExecutionsRow
	err := mdb.conn.Select(&rows, lockCurrentExecutionJoinExecutionsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return rows, err
}

// InsertIntoTransferTasks inserts one or more rows into transfer_tasks table
func (mdb *db) InsertIntoTransferTasks(rows []sqldb.TransferTasksRow) (sql.Result, error) {
	return mdb.conn.NamedExec(createTransferTasksQuery, rows)
}

// SelectFromTransferTasks reads one or more rows from transfer_tasks table
func (mdb *db) SelectFromTransferTasks(filter *sqldb.TransferTasksFilter) ([]sqldb.TransferTasksRow, error) {
	var rows []sqldb.TransferTasksRow
	err := mdb.conn.Select(&rows, getTransferTasksQuery, filter.ShardID, *filter.MinTaskID, *filter.MaxTaskID)
	if err != nil {
		return nil, err
	}
	return rows, err
}

// DeleteFromTransferTasks deletes one or more rows from transfer_tasks table
func (mdb *db) DeleteFromTransferTasks(filter *sqldb.TransferTasksFilter) (sql.Result, error) {
	if filter.MinTaskID != nil {
		return mdb.conn.Exec(rangeDeleteTransferTaskQuery, filter.ShardID, *filter.MinTaskID, *filter.MaxTaskID)
	}
	return mdb.conn.Exec(deleteTransferTaskQuery, filter.ShardID, *filter.TaskID)
}

// InsertIntoTimerTasks inserts one or more rows into timer_tasks table
func (mdb *db) InsertIntoTimerTasks(rows []sqldb.TimerTasksRow) (sql.Result, error) {
	for i := range rows {
		rows[i].VisibilityTimestamp = mdb.converter.ToMySQLDateTime(rows[i].VisibilityTimestamp)
	}
	return mdb.conn.NamedExec(createTimerTasksQuery, rows)
}

// SelectFromTimerTasks reads one or more rows from timer_tasks table
func (mdb *db) SelectFromTimerTasks(filter *sqldb.TimerTasksFilter) ([]sqldb.TimerTasksRow, error) {
	var rows []sqldb.TimerTasksRow
	*filter.MinVisibilityTimestamp = mdb.converter.ToMySQLDateTime(*filter.MinVisibilityTimestamp)
	*filter.MaxVisibilityTimestamp = mdb.converter.ToMySQLDateTime(*filter.MaxVisibilityTimestamp)
	err := mdb.conn.Select(&rows, getTimerTasksQuery, filter.ShardID, *filter.MinVisibilityTimestamp,
		filter.TaskID, *filter.MinVisibilityTimestamp, *filter.MaxVisibilityTimestamp, *filter.PageSize)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].VisibilityTimestamp = mdb.converter.FromMySQLDateTime(rows[i].VisibilityTimestamp)
	}
	return rows, err
}

// DeleteFromTimerTasks deletes one or more rows from timer_tasks table
func (mdb *db) DeleteFromTimerTasks(filter *sqldb.TimerTasksFilter) (sql.Result, error) {
	if filter.MinVisibilityTimestamp != nil {
		*filter.MinVisibilityTimestamp = mdb.converter.ToMySQLDateTime(*filter.MinVisibilityTimestamp)
		*filter.MaxVisibilityTimestamp = mdb.converter.ToMySQLDateTime(*filter.MaxVisibilityTimestamp)
		return mdb.conn.Exec(rangeDeleteTimerTaskQuery, filter.ShardID, *filter.MinVisibilityTimestamp, *filter.MaxVisibilityTimestamp)
	}
	*filter.VisibilityTimestamp = mdb.converter.ToMySQLDateTime(*filter.VisibilityTimestamp)
	return mdb.conn.Exec(deleteTimerTaskQuery, filter.ShardID, *filter.VisibilityTimestamp, filter.TaskID)
}

// InsertIntoBufferedEvents inserts one or more rows into buffered_events table
func (mdb *db) InsertIntoBufferedEvents(rows []sqldb.BufferedEventsRow) (sql.Result, error) {
	return mdb.conn.NamedExec(createBufferedEventsQuery, rows)
}

// SelectFromBufferedEvents reads one or more rows from buffered_events table
func (mdb *db) SelectFromBufferedEvents(filter *sqldb.BufferedEventsFilter) ([]sqldb.BufferedEventsRow, error) {
	var rows []sqldb.BufferedEventsRow
	err := mdb.conn.Select(&rows, getBufferedEventsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	for i := 0; i < len(rows); i++ {
		rows[i].DomainID = filter.DomainID
		rows[i].WorkflowID = filter.WorkflowID
		rows[i].RunID = filter.RunID
		rows[i].ShardID = filter.ShardID
	}
	return rows, err
}

// DeleteFromBufferedEvents deletes one or more rows from buffered_events table
func (mdb *db) DeleteFromBufferedEvents(filter *sqldb.BufferedEventsFilter) (sql.Result, error) {
	return mdb.conn.Exec(deleteBufferedEventsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}

// InsertIntoReplicationTasks inserts one or more rows into replication_tasks table
func (mdb *db) InsertIntoReplicationTasks(rows []sqldb.ReplicationTasksRow) (sql.Result, error) {
	return mdb.conn.NamedExec(createReplicationTasksQuery, rows)
}

// SelectFromReplicationTasks reads one or more rows from replication_tasks table
func (mdb *db) SelectFromReplicationTasks(filter *sqldb.ReplicationTasksFilter) ([]sqldb.ReplicationTasksRow, error) {
	var rows []sqldb.ReplicationTasksRow
	err := mdb.conn.Select(&rows, getReplicationTasksQuery, filter.ShardID, filter.MinTaskID, filter.MaxTaskID, filter.PageSize)
	return rows, err
}

// DeleteFromReplicationTasks deletes one or more rows from replication_tasks table
func (mdb *db) DeleteFromReplicationTasks(shardID, taskID int) (sql.Result, error) {
	return mdb.conn.Exec(deleteReplicationTaskQuery, shardID, taskID)
}

// InsertIntoReplicationTasksDLQ inserts one or more rows into replication_tasks_dlq table
func (mdb *db) InsertIntoReplicationTasksDLQ(row *sqldb.ReplicationTaskDLQRow) (sql.Result, error) {
	return mdb.conn.NamedExec(insertReplicationTaskDLQQuery, row)
}

// SelectFromReplicationTasksDLQ reads one or more rows from replication_tasks_dlq table
func (mdb *db) SelectFromReplicationTasksDLQ(filter *sqldb.ReplicationTasksDLQFilter) ([]sqldb.ReplicationTasksRow, error) {
	var rows []sqldb.ReplicationTasksRow
	err := mdb.conn.Select(
		&rows, getReplicationTasksDLQQuery,
		filter.SourceClusterName,
		filter.ShardID,
		filter.MinTaskID,
		filter.MaxTaskID,
		filter.PageSize)
	return rows, err
}
