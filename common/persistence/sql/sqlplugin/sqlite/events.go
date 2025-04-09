package sqlite

import (
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	deleteHistoryNodesQuery = `WITH events_to_delete AS (
    SELECT shard_id, tree_id, branch_id, node_id, txn_id
    FROM history_node
    WHERE shard_id = ? AND tree_id = ? AND branch_id = ? AND node_id >= ?
    ORDER BY shard_id, tree_id, branch_id, node_id, txn_id
    LIMIT ?
)

DELETE FROM history_node
WHERE (shard_id, tree_id, branch_id, node_id, txn_id) IN (SELECT shard_id, tree_id, branch_id, node_id, txn_id FROM events_to_delete);`
)

// DeleteFromHistoryNode deletes one or more rows from history_node table
func (mdb *DB) DeleteFromHistoryNode(ctx context.Context, filter *sqlplugin.HistoryNodeFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, deleteHistoryNodesQuery, filter.ShardID, filter.TreeID, filter.BranchID, *filter.MinNodeID, filter.PageSize)
}
