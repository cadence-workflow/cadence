package sqlite

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockExecutionQueryBase = `SELECT next_event_id FROM executions
 WHERE shard_id = ? AND domain_id = ? AND workflow_id = ? AND run_id = ?`
	writeLockExecutionQuery = lockExecutionQueryBase
	readLockExecutionQuery  = lockExecutionQueryBase

	lockCurrentExecutionJoinExecutionsQuery = `SELECT
ce.shard_id, ce.domain_id, ce.workflow_id, ce.run_id, ce.create_request_id, ce.state, ce.close_status, ce.start_version, e.last_write_version
FROM current_executions ce
INNER JOIN executions e ON e.shard_id = ce.shard_id AND e.domain_id = ce.domain_id AND e.workflow_id = ce.workflow_id AND e.run_id = ce.run_id
WHERE ce.shard_id = ? AND ce.domain_id = ? AND ce.workflow_id = ?`

	getCurrentExecutionQuery = `SELECT
shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version
FROM current_executions WHERE shard_id = ? AND domain_id = ? AND workflow_id = ?`
	lockCurrentExecutionQuery = getCurrentExecutionQuery

	rangeDeleteTransferTaskQuery        = `DELETE FROM transfer_tasks WHERE shard_id = ? AND task_id >= ? AND task_id < ?`
	rangeDeleteTransferTaskByBatchQuery = `WITH tasks_to_delete AS (
    SELECT shard_id, task_id
    FROM transfer_tasks
    WHERE shard_id = ? AND task_id >= ? AND task_id < ?
    ORDER BY task_id
    LIMIT ?
)

DELETE FROM transfer_tasks
WHERE (shard_id, task_id) IN (SELECT shard_id, task_id FROM tasks_to_delete);`

	rangeDeleteReplicationTaskQuery        = `DELETE FROM replication_tasks WHERE shard_id = ? AND task_id < ?`
	rangeDeleteReplicationTaskByBatchQuery = `WITH tasks_to_delete AS (
    SELECT shard_id, task_id
    FROM replication_tasks
    WHERE shard_id = ? AND task_id < ?
    ORDER BY task_id
    LIMIT ?
)

DELETE FROM replication_tasks
WHERE (shard_id, task_id) IN (SELECT shard_id, task_id FROM tasks_to_delete);`

	rangeDeleteTimerTaskQuery        = `DELETE FROM timer_tasks WHERE shard_id = ? AND visibility_timestamp >= ? AND visibility_timestamp < ?`
	rangeDeleteTimerTaskByBatchQuery = `WITH tasks_to_delete AS (
    SELECT shard_id, visibility_timestamp, task_id
    FROM timer_tasks
    WHERE shard_id = ? AND visibility_timestamp >= ? AND visibility_timestamp < ?
    ORDER BY visibility_timestamp,task_id
    LIMIT ?
)

DELETE FROM timer_tasks
WHERE (shard_id, visibility_timestamp, task_id) IN (SELECT shard_id, visibility_timestamp, task_id FROM tasks_to_delete);`
)

// ReadLockExecutions acquires a write lock on a single row in executions table
func (mdb *DB) ReadLockExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) (int, error) {
	var nextEventID int
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &nextEventID, readLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// WriteLockExecutions acquires a write lock on a single row in executions table
func (mdb *DB) WriteLockExecutions(ctx context.Context, filter *sqlplugin.ExecutionsFilter) (int, error) {
	var nextEventID int
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &nextEventID, writeLockExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	return nextEventID, err
}

// LockCurrentExecutionsJoinExecutions joins a row in current_executions with executions table and acquires a
// write lock on the result
func (mdb *DB) LockCurrentExecutionsJoinExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) ([]sqlplugin.CurrentExecutionsRow, error) {
	var rows []sqlplugin.CurrentExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.SelectContext(ctx, dbShardID, &rows, lockCurrentExecutionJoinExecutionsQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return rows, err
}

// LockCurrentExecutions acquires a write lock on a single row in current_executions table
func (mdb *DB) LockCurrentExecutions(ctx context.Context, filter *sqlplugin.CurrentExecutionsFilter) (*sqlplugin.CurrentExecutionsRow, error) {
	var row sqlplugin.CurrentExecutionsRow
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(int(filter.ShardID), mdb.GetTotalNumDBShards())
	err := mdb.driver.GetContext(ctx, dbShardID, &row, lockCurrentExecutionQuery, filter.ShardID, filter.DomainID, filter.WorkflowID)
	return &row, err
}

// RangeDeleteFromTransferTasks deletes multi rows from transfer_tasks table
func (mdb *DB) RangeDeleteFromTransferTasks(ctx context.Context, filter *sqlplugin.TransferTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTransferTaskByBatchQuery, filter.ShardID, filter.InclusiveMinTaskID, filter.ExclusiveMaxTaskID, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTransferTaskQuery, filter.ShardID, filter.InclusiveMinTaskID, filter.ExclusiveMaxTaskID)
}

// RangeDeleteFromReplicationTasks deletes multi rows from replication_tasks table
func (mdb *DB) RangeDeleteFromReplicationTasks(ctx context.Context, filter *sqlplugin.ReplicationTasksFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteReplicationTaskByBatchQuery, filter.ShardID, filter.ExclusiveMaxTaskID, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteReplicationTaskQuery, filter.ShardID, filter.ExclusiveMaxTaskID)
}

// RangeDeleteFromTimerTasks deletes multi rows from timer_tasks table
func (mdb *DB) RangeDeleteFromTimerTasks(ctx context.Context, filter *sqlplugin.TimerTasksFilter) (sql.Result, error) {
	filter.MinVisibilityTimestamp = mdb.converter.ToDateTime(filter.MinVisibilityTimestamp)
	filter.MaxVisibilityTimestamp = mdb.converter.ToDateTime(filter.MaxVisibilityTimestamp)
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, mdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTimerTaskByBatchQuery, filter.ShardID, filter.MinVisibilityTimestamp, filter.MaxVisibilityTimestamp, filter.PageSize)
	}
	return mdb.driver.ExecContext(ctx, dbShardID, rangeDeleteTimerTaskQuery, filter.ShardID, filter.MinVisibilityTimestamp, filter.MaxVisibilityTimestamp)
}
