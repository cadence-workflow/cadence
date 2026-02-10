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
	"errors"
	"fmt"
	"strings"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	templateCreateWorkflowExecutionStarted = `INSERT INTO executions_visibility (` +
		`domain_id, workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
         ON CONFLICT (domain_id, run_id) DO NOTHING`

	templateCreateWorkflowExecutionClosed = `INSERT INTO executions_visibility (` +
		`domain_id, workflow_id, run_id, start_time, execution_time, workflow_type_name, close_time, close_status, history_length, memo, encoding, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (domain_id, run_id) DO UPDATE
		  SET workflow_id = excluded.workflow_id,
		      start_time = excluded.start_time,
		      execution_time = excluded.execution_time,
              workflow_type_name = excluded.workflow_type_name,
			  close_time = excluded.close_time,
			  close_status = excluded.close_status,
			  history_length = excluded.history_length,
			  memo = excluded.memo,
			  encoding = excluded.encoding,
				is_cron = excluded.is_cron,
				num_clusters = excluded.num_clusters,
				update_time = excluded.update_time,
				shard_id = excluded.shard_id,
				execution_status = excluded.execution_status,
				cron_schedule = excluded.cron_schedule,
				scheduled_execution_time = excluded.scheduled_execution_time`

	// RunID condition is needed for correct pagination
	templateConditions1 = ` AND domain_id = $1
		 AND start_time >= $2
		 AND start_time <= $3
 		 AND (run_id > $4 OR start_time < $5)
         ORDER BY start_time DESC, run_id
         LIMIT $6`

	templateConditions2 = ` AND domain_id = $2
		 AND start_time >= $3
		 AND start_time <= $4
 		 AND (run_id > $5 OR start_time < $6)
         ORDER BY start_time DESC, run_id
         LIMIT $7`

	templateOpenFieldNames = `workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, is_cron, update_time, shard_id`
	templateOpenSelect     = `SELECT ` + templateOpenFieldNames + ` FROM executions_visibility WHERE close_status IS NULL `

	templateClosedSelect = `SELECT ` + templateOpenFieldNames + `, close_time, close_status, history_length
		 FROM executions_visibility WHERE close_status IS NOT NULL `

	templateGetOpenWorkflowExecutions = templateOpenSelect + templateConditions1

	templateGetClosedWorkflowExecutions = templateClosedSelect + templateConditions1

	templateGetOpenWorkflowExecutionsByType = templateOpenSelect + `AND workflow_type_name = $1` + templateConditions2

	templateGetClosedWorkflowExecutionsByType = templateClosedSelect + `AND workflow_type_name = $1` + templateConditions2

	templateGetOpenWorkflowExecutionsByID = templateOpenSelect + `AND workflow_id = $1` + templateConditions2

	templateGetClosedWorkflowExecutionsByID = templateClosedSelect + `AND workflow_id = $1` + templateConditions2

	templateGetClosedWorkflowExecutionsByStatus = templateClosedSelect + `AND close_status = $1` + templateConditions2

	templateGetClosedWorkflowExecution = `SELECT workflow_id, run_id, start_time, execution_time, memo, encoding, close_time, workflow_type_name, close_status, history_length, is_cron, update_time, shard_id
		 FROM executions_visibility
		 WHERE domain_id = $1 AND close_status IS NOT NULL
		 AND run_id = $2`

	templateDeleteWorkflowExecution = "DELETE FROM executions_visibility WHERE domain_id=$1 AND run_id=$2"
)

var errCloseParams = errors.New("missing one of {closeStatus, closeTime, historyLength} params")

// InsertIntoVisibility inserts a row into visibility table. If an row already exist,
// its left as such and no update will be made
func (pdb *db) InsertIntoVisibility(ctx context.Context, row *sqlplugin.VisibilityRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(row.DomainID, pdb.GetTotalNumDBShards())
	row.StartTime = pdb.converter.ToPostgresDateTime(row.StartTime)
	return pdb.driver.ExecContext(ctx, dbShardID, templateCreateWorkflowExecutionStarted,
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

// ReplaceIntoVisibility replaces an existing row if it exist or creates a new row in visibility table
func (pdb *db) ReplaceIntoVisibility(ctx context.Context, row *sqlplugin.VisibilityRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(row.DomainID, pdb.GetTotalNumDBShards())
	switch {
	case row.CloseStatus != nil && row.CloseTime != nil && row.HistoryLength != nil:
		row.StartTime = pdb.converter.ToPostgresDateTime(row.StartTime)
		closeTime := pdb.converter.ToPostgresDateTime(*row.CloseTime)
		return pdb.driver.ExecContext(ctx, dbShardID, templateCreateWorkflowExecutionClosed,
			row.DomainID,
			row.WorkflowID,
			row.RunID,
			row.StartTime,
			row.ExecutionTime,
			row.WorkflowTypeName,
			closeTime,
			*row.CloseStatus,
			*row.HistoryLength,
			row.Memo,
			row.Encoding,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID)
	default:
		return nil, errCloseParams
	}
}

// DeleteFromVisibility deletes a row from visibility table if it exist
func (pdb *db) DeleteFromVisibility(ctx context.Context, filter *sqlplugin.VisibilityFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(filter.DomainID, pdb.GetTotalNumDBShards())
	return pdb.driver.ExecContext(ctx, dbShardID, templateDeleteWorkflowExecution, filter.DomainID, filter.RunID)
}

// SelectFromVisibility reads one or more rows from visibility table
func (pdb *db) SelectFromVisibility(ctx context.Context, filter *sqlplugin.VisibilityFilter) ([]sqlplugin.VisibilityRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(filter.DomainID, pdb.GetTotalNumDBShards())
	var err error
	var rows []sqlplugin.VisibilityRow
	if filter.MinStartTime != nil {
		*filter.MinStartTime = pdb.converter.ToPostgresDateTime(*filter.MinStartTime)
	}
	if filter.MaxStartTime != nil {
		*filter.MaxStartTime = pdb.converter.ToPostgresDateTime(*filter.MaxStartTime)
	}
	switch {
	case filter.MinStartTime == nil && filter.RunID != nil && filter.Closed:
		var row sqlplugin.VisibilityRow
		err = pdb.driver.GetContext(ctx, dbShardID, &row, templateGetClosedWorkflowExecution, filter.DomainID, *filter.RunID)
		if err == nil {
			rows = append(rows, row)
		}
	case filter.MinStartTime != nil && filter.WorkflowID != nil:
		qry := templateGetOpenWorkflowExecutionsByID
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutionsByID
		}
		err = pdb.driver.SelectContext(ctx, dbShardID, &rows,
			qry,
			*filter.WorkflowID,
			filter.DomainID,
			pdb.converter.ToPostgresDateTime(*filter.MinStartTime),
			pdb.converter.ToPostgresDateTime(*filter.MaxStartTime),
			*filter.RunID,
			*filter.MinStartTime,
			*filter.PageSize)
	case filter.MinStartTime != nil && filter.WorkflowTypeName != nil:
		qry := templateGetOpenWorkflowExecutionsByType
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutionsByType
		}
		err = pdb.driver.SelectContext(ctx, dbShardID, &rows,
			qry,
			*filter.WorkflowTypeName,
			filter.DomainID,
			pdb.converter.ToPostgresDateTime(*filter.MinStartTime),
			pdb.converter.ToPostgresDateTime(*filter.MaxStartTime),
			*filter.RunID,
			*filter.MaxStartTime,
			*filter.PageSize)
	case filter.MinStartTime != nil && filter.CloseStatus != nil:
		err = pdb.driver.SelectContext(ctx, dbShardID, &rows,
			templateGetClosedWorkflowExecutionsByStatus,
			*filter.CloseStatus,
			filter.DomainID,
			pdb.converter.ToPostgresDateTime(*filter.MinStartTime),
			pdb.converter.ToPostgresDateTime(*filter.MaxStartTime),
			*filter.RunID,
			pdb.converter.ToPostgresDateTime(*filter.MaxStartTime),
			*filter.PageSize)
	case filter.MinStartTime != nil:
		qry := templateGetOpenWorkflowExecutions
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutions
		}
		minSt := pdb.converter.ToPostgresDateTime(*filter.MinStartTime)
		maxSt := pdb.converter.ToPostgresDateTime(*filter.MaxStartTime)
		err = pdb.driver.SelectContext(ctx, dbShardID, &rows,
			qry,
			filter.DomainID,
			minSt,
			maxSt,
			*filter.RunID,
			maxSt,
			*filter.PageSize)
	default:
		return nil, fmt.Errorf("invalid query filter")
	}
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].StartTime = pdb.converter.FromPostgresDateTime(rows[i].StartTime)
		rows[i].ExecutionTime = pdb.converter.FromPostgresDateTime(rows[i].ExecutionTime)
		if rows[i].CloseTime != nil {
			closeTime := pdb.converter.FromPostgresDateTime(*rows[i].CloseTime)
			rows[i].CloseTime = &closeTime
		}
		rows[i].RunID = strings.TrimSpace(rows[i].RunID)
		rows[i].WorkflowID = strings.TrimSpace(rows[i].WorkflowID)
	}
	return rows, err
}
