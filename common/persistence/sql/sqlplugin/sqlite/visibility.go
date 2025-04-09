package sqlite

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	templateCreateWorkflowExecutionStarted = `INSERT OR IGNORE INTO executions_visibility (` +
		`domain_id, workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, is_cron, num_clusters, update_time, shard_id) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

// InsertIntoVisibility inserts a row into visibility table. If an row already exist,
// its left as such and no update will be made
func (mdb *DB) InsertIntoVisibility(ctx context.Context, row *sqlplugin.VisibilityRow) (sql.Result, error) {
	row.StartTime = mdb.converter.ToDateTime(row.StartTime)
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(row.DomainID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx,
		dbShardID,
		templateCreateWorkflowExecutionStarted,
		row.DomainID,
		row.WorkflowID,
		row.RunID,
		row.StartTime,
		row.ExecutionTime,
		row.WorkflowTypeName,
		row.Memo,
		row.Encoding,
		row.IsCron,
		row.NumClusters,
		row.UpdateTime,
		row.ShardID)
}
