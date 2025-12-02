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
	"context"
	"database/sql"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	// below are templates for history_node table
	addHistoryNodesQuery = `INSERT INTO history_node (` +
		`shard_id, tree_id, branch_id, node_id, txn_id, data, data_encoding) ` +
		`VALUES (:shard_id, :tree_id, :branch_id, :node_id, :txn_id, :data, :data_encoding) `

	getHistoryNodesQuery = `SELECT node_id, txn_id, data, data_encoding FROM history_node ` +
		`WHERE shard_id = $1 AND tree_id = $2 AND branch_id = $3 AND node_id >= $4 and node_id < $5 ORDER BY shard_id, tree_id, branch_id, node_id, txn_id LIMIT $6 `

	deleteHistoryNodesQuery = `DELETE FROM history_node WHERE shard_id = $1 AND tree_id = $2 AND branch_id = $3 AND node_id >= $4`

	deleteHistoryNodesByBatchQuery = `DELETE FROM history_node WHERE shard_id = $1 AND tree_id = $2 AND branch_id = $3 AND (node_id,txn_id) IN (SELECT node_id,txn_id FROM
		history_node WHERE shard_id = $1 AND tree_id = $2 AND branch_id = $3 AND node_id >= $4 LIMIT $5)`

	// below are templates for history_tree table
	addHistoryTreeQuery = `INSERT INTO history_tree (` +
		`shard_id, tree_id, branch_id, data, data_encoding) ` +
		`VALUES (:shard_id, :tree_id, :branch_id, :data, :data_encoding) `

	getHistoryTreeQuery = `SELECT branch_id, data, data_encoding FROM history_tree WHERE shard_id = $1 AND tree_id = $2 `

	deleteHistoryTreeQuery = `DELETE FROM history_tree WHERE shard_id = $1 AND tree_id = $2 AND branch_id = $3 `

	getAllHistoryTreeQuery = `SELECT shard_id, tree_id, branch_id, data, data_encoding FROM history_tree WHERE (shard_id = $1 AND tree_id = $2 AND branch_id > $3) OR (shard_id = $1 AND tree_id > $2) OR (shard_id > $1) ORDER BY shard_id, tree_id, branch_id LIMIT $4`
)

// For history_node table:

// InsertIntoHistoryNode inserts a row into history_node table
func (pdb *db) InsertIntoHistoryNode(ctx context.Context, row *sqlplugin.HistoryNodeRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(row.TreeID, pdb.GetTotalNumDBShards())
	// NOTE: Query 5.6 doesn't support clustering order, to workaround, we let txn_id multiple by -1
	*row.TxnID *= -1
	return pdb.driver.NamedExecContext(ctx, dbShardID, addHistoryNodesQuery, row)
}

// SelectFromHistoryNode reads one or more rows from history_node table
func (pdb *db) SelectFromHistoryNode(ctx context.Context, filter *sqlplugin.HistoryNodeFilter) ([]sqlplugin.HistoryNodeRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, pdb.GetTotalNumDBShards())
	var rows []sqlplugin.HistoryNodeRow
	err := pdb.driver.SelectContext(ctx, dbShardID, &rows, getHistoryNodesQuery,
		filter.ShardID, filter.TreeID, filter.BranchID, *filter.MinNodeID, *filter.MaxNodeID, filter.PageSize)
	// NOTE: since we let txn_id multiple by -1 when inserting, we have to revert it back here
	for _, row := range rows {
		*row.TxnID *= -1
	}
	return rows, err
}

// DeleteFromHistoryNode deletes one or more rows from history_node table
func (pdb *db) DeleteFromHistoryNode(ctx context.Context, filter *sqlplugin.HistoryNodeFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, pdb.GetTotalNumDBShards())
	if filter.PageSize > 0 {
		return pdb.driver.ExecContext(ctx, dbShardID, deleteHistoryNodesByBatchQuery, filter.ShardID, filter.TreeID, filter.BranchID, *filter.MinNodeID, filter.PageSize)
	}
	return pdb.driver.ExecContext(ctx, dbShardID, deleteHistoryNodesQuery, filter.ShardID, filter.TreeID, filter.BranchID, *filter.MinNodeID)
}

// For history_tree table:

// InsertIntoHistoryTree inserts a row into history_tree table
func (pdb *db) InsertIntoHistoryTree(ctx context.Context, row *sqlplugin.HistoryTreeRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(row.TreeID, pdb.GetTotalNumDBShards())
	return pdb.driver.NamedExecContext(ctx, dbShardID, addHistoryTreeQuery, row)
}

// SelectFromHistoryTree reads one or more rows from history_tree table
func (pdb *db) SelectFromHistoryTree(ctx context.Context, filter *sqlplugin.HistoryTreeFilter) ([]sqlplugin.HistoryTreeRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, pdb.GetTotalNumDBShards())
	var rows []sqlplugin.HistoryTreeRow
	err := pdb.driver.SelectContext(ctx, dbShardID, &rows, getHistoryTreeQuery, filter.ShardID, filter.TreeID)
	return rows, err
}

// DeleteFromHistoryTree deletes one or more rows from history_tree table
func (pdb *db) DeleteFromHistoryTree(ctx context.Context, filter *sqlplugin.HistoryTreeFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, pdb.GetTotalNumDBShards())
	return pdb.driver.ExecContext(ctx, dbShardID, deleteHistoryTreeQuery, filter.ShardID, filter.TreeID, *filter.BranchID)
}

func (pdb *db) GetAllHistoryTreeBranches(ctx context.Context, filter *sqlplugin.HistoryTreeFilter) ([]sqlplugin.HistoryTreeRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromTreeID(filter.TreeID, pdb.GetTotalNumDBShards())
	var rows []sqlplugin.HistoryTreeRow
	err := pdb.driver.SelectContext(ctx, dbShardID, &rows, getAllHistoryTreeQuery, filter.ShardID, filter.TreeID, filter.BranchID, filter.PageSize)
	return rows, err
}
