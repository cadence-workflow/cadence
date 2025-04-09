// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
