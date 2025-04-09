package sqlite

import (
	"context"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	lockExecutionQueryBase = `SELECT next_event_id FROM executions
 WHERE shard_id = ? AND domain_id = ? AND workflow_id = ? AND run_id = ?`
	writeLockExecutionQuery = lockExecutionQueryBase

	lockCurrentExecutionJoinExecutionsQuery = `SELECT
ce.shard_id, ce.domain_id, ce.workflow_id, ce.run_id, ce.create_request_id, ce.state, ce.close_status, ce.start_version, e.last_write_version
FROM current_executions ce
INNER JOIN executions e ON e.shard_id = ce.shard_id AND e.domain_id = ce.domain_id AND e.workflow_id = ce.workflow_id AND e.run_id = ce.run_id
WHERE ce.shard_id = ? AND ce.domain_id = ? AND ce.workflow_id = ?`

	getCurrentExecutionQuery = `SELECT
shard_id, domain_id, workflow_id, run_id, create_request_id, state, close_status, start_version, last_write_version
FROM current_executions WHERE shard_id = ? AND domain_id = ? AND workflow_id = ?`
	lockCurrentExecutionQuery = getCurrentExecutionQuery
)

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
