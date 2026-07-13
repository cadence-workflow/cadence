// Copyright (c) 2026 Uber Technologies, Inc.
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
	selectActiveClusterSelectionPolicyQry = `
	SELECT shard_id, domain_id, workflow_id, run_id, data, data_encoding
	FROM active_cluster_selection_policy
	WHERE shard_id=$1 AND domain_id=$2 AND workflow_id=$3 AND run_id=$4
	`

	insertActiveClusterSelectionPolicyQry = `
	INSERT INTO active_cluster_selection_policy (shard_id, domain_id, workflow_id, run_id, data, data_encoding)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (shard_id, domain_id, workflow_id, run_id) DO NOTHING
	`

	deleteActiveClusterSelectionPolicyQry = `
	DELETE FROM active_cluster_selection_policy
	WHERE shard_id=$1 AND domain_id=$2 AND workflow_id=$3 AND run_id=$4
	`
)

func (pdb *db) InsertIntoActiveClusterSelectionPolicy(ctx context.Context, row *sqlplugin.ActiveClusterSelectionPolicyRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(row.ShardID, pdb.GetTotalNumDBShards())
	return pdb.driver.ExecContext(ctx, dbShardID, insertActiveClusterSelectionPolicyQry, row.ShardID, row.DomainID, row.WorkflowID, row.RunID, row.Data, row.DataEncoding)
}

func (pdb *db) SelectFromActiveClusterSelectionPolicy(ctx context.Context, filter *sqlplugin.ActiveClusterSelectionPolicyFilter) (*sqlplugin.ActiveClusterSelectionPolicyRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, pdb.GetTotalNumDBShards())
	var row sqlplugin.ActiveClusterSelectionPolicyRow
	err := pdb.driver.GetContext(ctx, dbShardID, &row, selectActiveClusterSelectionPolicyQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (pdb *db) DeleteFromActiveClusterSelectionPolicy(ctx context.Context, filter *sqlplugin.ActiveClusterSelectionPolicyFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(filter.ShardID, pdb.GetTotalNumDBShards())
	return pdb.driver.ExecContext(ctx, dbShardID, deleteActiveClusterSelectionPolicyQry, filter.ShardID, filter.DomainID, filter.WorkflowID, filter.RunID)
}
