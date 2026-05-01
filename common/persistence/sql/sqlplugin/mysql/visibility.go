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
	"errors"
	"fmt"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	templateCreateWorkflowExecutionStarted = `INSERT IGNORE INTO executions_visibility (` +
		`domain_id, workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	templateCreateWorkflowExecutionClosed = `REPLACE INTO executions_visibility (` +
		`domain_id, workflow_id, run_id, start_time, execution_time, workflow_type_name, close_time, close_status, history_length, memo, encoding, is_cron, num_clusters, update_time, shard_id, execution_status, cron_schedule, scheduled_execution_time) ` +
		`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// RunID condition is needed for correct pagination
	templateConditions = ` AND domain_id = ?
		 AND start_time >= ?
		 AND start_time <= ?
 		 AND (run_id > ? OR start_time < ?)
         ORDER BY start_time DESC, run_id
         LIMIT ?`

	templateOpenFieldNames = `workflow_id, run_id, start_time, execution_time, workflow_type_name, memo, encoding, is_cron, update_time, shard_id`
	templateOpenSelect     = `SELECT ` + templateOpenFieldNames + ` FROM executions_visibility WHERE close_status IS NULL `

	templateClosedSelect = `SELECT ` + templateOpenFieldNames + `, close_time, close_status, history_length
		 FROM executions_visibility WHERE close_status IS NOT NULL `

	templateGetOpenWorkflowExecutions = templateOpenSelect + templateConditions

	templateGetClosedWorkflowExecutions = templateClosedSelect + templateConditions

	templateGetOpenWorkflowExecutionsByType = templateOpenSelect + `AND workflow_type_name = ?` + templateConditions

	templateGetClosedWorkflowExecutionsByType = templateClosedSelect + `AND workflow_type_name = ?` + templateConditions

	templateGetOpenWorkflowExecutionsByID = templateOpenSelect + `AND workflow_id = ?` + templateConditions

	templateGetClosedWorkflowExecutionsByID = templateClosedSelect + `AND workflow_id = ?` + templateConditions

	templateGetClosedWorkflowExecutionsByStatus = templateClosedSelect + `AND close_status = ?` + templateConditions

	templateGetClosedWorkflowExecution = `SELECT workflow_id, run_id, start_time, execution_time, memo, encoding, close_time, workflow_type_name, close_status, history_length, is_cron, update_time, shard_id
		 FROM executions_visibility
		 WHERE domain_id = ? AND close_status IS NOT NULL
		 AND run_id = ?`

	templateDeleteWorkflowExecution = "DELETE FROM executions_visibility WHERE domain_id=? AND run_id=?"
)

var errCloseParams = errors.New("missing one of {closeStatus, closeTime, historyLength} params")

// InsertIntoVisibility inserts a row into visibility table. If an row already exist,
// its left as such and no update will be made
func (mdb *DB) InsertIntoVisibility(ctx context.Context, row *sqlplugin.VisibilityRow) (sql.Result, error) {
	row.StartTime = mdb.converter.ToDateTime(row.StartTime)
	scheduledExecutionTime := mdb.converter.ToDateTime(row.ScheduledExecutionTime)
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
		row.ShardID,
		row.ExecutionStatus,
		row.CronSchedule,
		scheduledExecutionTime)
}

// ReplaceIntoVisibility replaces an existing row if it exist or creates a new row in visibility table
func (mdb *DB) ReplaceIntoVisibility(ctx context.Context, row *sqlplugin.VisibilityRow) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(row.DomainID, mdb.GetTotalNumDBShards())
	switch {
	case row.CloseStatus != nil && row.CloseTime != nil && row.HistoryLength != nil:
		row.StartTime = mdb.converter.ToDateTime(row.StartTime)
		closeTime := mdb.converter.ToDateTime(*row.CloseTime)
		scheduledExecutionTime := mdb.converter.ToDateTime(row.ScheduledExecutionTime)
		return mdb.driver.ExecContext(ctx,
			dbShardID,
			templateCreateWorkflowExecutionClosed,
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
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			scheduledExecutionTime)
	default:
		return nil, errCloseParams
	}
}

// DeleteFromVisibility deletes a row from visibility table if it exist
func (mdb *DB) DeleteFromVisibility(ctx context.Context, filter *sqlplugin.VisibilityFilter) (sql.Result, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(filter.DomainID, mdb.GetTotalNumDBShards())
	return mdb.driver.ExecContext(ctx, dbShardID, templateDeleteWorkflowExecution, filter.DomainID, filter.RunID)
}

// SelectFromVisibility reads one or more rows from visibility table
func (mdb *DB) SelectFromVisibility(ctx context.Context, filter *sqlplugin.VisibilityFilter) ([]sqlplugin.VisibilityRow, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainID(filter.DomainID, mdb.GetTotalNumDBShards())
	var err error
	var rows []sqlplugin.VisibilityRow
	if filter.MinStartTime != nil {
		*filter.MinStartTime = mdb.converter.ToDateTime(*filter.MinStartTime)
	}
	if filter.MaxStartTime != nil {
		*filter.MaxStartTime = mdb.converter.ToDateTime(*filter.MaxStartTime)
	}
	switch {
	case filter.MinStartTime == nil && filter.RunID != nil && filter.Closed:
		var row sqlplugin.VisibilityRow
		err = mdb.driver.GetContext(ctx, dbShardID, &row, templateGetClosedWorkflowExecution, filter.DomainID, *filter.RunID)
		if err == nil {
			rows = append(rows, row)
		}
	case filter.MinStartTime != nil && filter.WorkflowID != nil:
		qry := templateGetOpenWorkflowExecutionsByID
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutionsByID
		}
		err = mdb.driver.SelectContext(ctx,
			dbShardID,
			&rows,
			qry,
			*filter.WorkflowID,
			filter.DomainID,
			mdb.converter.ToDateTime(*filter.MinStartTime),
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.RunID,
			*filter.MinStartTime,
			*filter.PageSize)
	case filter.MinStartTime != nil && filter.WorkflowTypeName != nil:
		qry := templateGetOpenWorkflowExecutionsByType
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutionsByType
		}
		err = mdb.driver.SelectContext(ctx,
			dbShardID,
			&rows,
			qry,
			*filter.WorkflowTypeName,
			filter.DomainID,
			mdb.converter.ToDateTime(*filter.MinStartTime),
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.RunID,
			*filter.MaxStartTime,
			*filter.PageSize)
	case filter.MinStartTime != nil && filter.CloseStatus != nil:
		err = mdb.driver.SelectContext(ctx,
			dbShardID,
			&rows,
			templateGetClosedWorkflowExecutionsByStatus,
			*filter.CloseStatus,
			filter.DomainID,
			mdb.converter.ToDateTime(*filter.MinStartTime),
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.RunID,
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.PageSize)
	case filter.MinStartTime != nil:
		qry := templateGetOpenWorkflowExecutions
		if filter.Closed {
			qry = templateGetClosedWorkflowExecutions
		}
		err = mdb.driver.SelectContext(ctx,
			dbShardID,
			&rows,
			qry,
			filter.DomainID,
			mdb.converter.ToDateTime(*filter.MinStartTime),
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.RunID,
			mdb.converter.ToDateTime(*filter.MaxStartTime),
			*filter.PageSize)
	default:
		return nil, fmt.Errorf("invalid query filter")
	}
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].StartTime = mdb.converter.FromDateTime(rows[i].StartTime)
		rows[i].ExecutionTime = mdb.converter.FromDateTime(rows[i].ExecutionTime)
		if rows[i].CloseTime != nil {
			closeTime := mdb.converter.FromDateTime(*rows[i].CloseTime)
			rows[i].CloseTime = &closeTime
		}
	}
	return rows, err
}
