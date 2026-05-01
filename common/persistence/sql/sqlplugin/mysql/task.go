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
	"fmt"

	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
)

const (
	taskListCreatePart = `INTO task_lists(shard_id, domain_id, name, task_type, range_id, data, data_encoding) ` +
		`VALUES (:shard_id, :domain_id, :name, :task_type, :range_id, :data, :data_encoding)`

	// (default range ID: initialRangeID == 1)
	createTaskListQry = `INSERT ` + taskListCreatePart

	updateTaskListQry = `UPDATE task_lists SET
range_id = :range_id,
data = :data,
data_encoding = :data_encoding
WHERE
shard_id = :shard_id AND
domain_id = :domain_id AND
name = :name AND
task_type = :task_type
`

	// This query uses pagination that is best understood by analogy to simple numbers.
	// Given a list of numbers
	// 	111
	//	113
	//	122
	//	211
	// where the hundreds digit corresponds to domain_id, the tens digit
	// corresponds to name, and the ones digit corresponds to task_type,
	// Imagine recurring queries with a limit of 1.
	// For the second query to skip the first result and return 112, it must allow equal values in hundreds & tens, but it's OK because the ones digit is higher.
	// For the third query, the ones digit is now lower but that's irrelevant because the tens digit is greater.
	// For the fourth query, both the tens digit and ones digit are now lower but that's again irrelevant because now the hundreds digit is higher.
	// This technique is useful since the size of the table can easily change between calls, making SKIP an unreliable method, while other db-specific things like rowids are not portable
	listTaskListQry = `SELECT domain_id, range_id, name, task_type, data, data_encoding ` +
		`FROM task_lists ` +
		`WHERE shard_id = ? AND ((domain_id = ? AND name = ? AND task_type > ?) OR (domain_id=? AND name > ?) OR (domain_id > ?)) ` +
		`ORDER BY domain_id,name,task_type LIMIT ?`

	getTaskListQry = `SELECT domain_id, range_id, name, task_type, data, data_encoding ` +
		`FROM task_lists ` +
		`WHERE shard_id = ? AND domain_id = ? AND name = ? AND task_type = ?`

	deleteTaskListQry = `DELETE FROM task_lists WHERE shard_id=? AND domain_id=? AND name=? AND task_type=? AND range_id=?`

	lockTaskListQry = `SELECT range_id FROM task_lists ` +
		`WHERE shard_id = ? AND domain_id = ? AND name = ? AND task_type = ? FOR UPDATE`

	getTaskMinMaxQry = `SELECT task_id, data, data_encoding ` +
		`FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id > ? AND task_id <= ? ` +
		` ORDER BY task_id LIMIT ?`

	getTaskMinQry = `SELECT task_id, data, data_encoding ` +
		`FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id > ? ORDER BY task_id LIMIT ?`

	getTasksCountQry = `SELECT count(1) as count ` +
		`FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id > ?`

	createTaskQry = `INSERT INTO ` +
		`tasks(domain_id, task_list_name, task_type, task_id, data, data_encoding) ` +
		`VALUES(:domain_id, :task_list_name, :task_type, :task_id, :data, :data_encoding)`

	deleteTaskQry = `DELETE FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id = ?`

	rangeDeleteTaskQry = `DELETE FROM tasks ` +
		`WHERE domain_id = ? AND task_list_name = ? AND task_type = ? AND task_id <= ? ` +
		`ORDER BY domain_id,task_list_name,task_type,task_id LIMIT ?`

	getOrphanTaskQry = `SELECT task_id, domain_id, task_list_name, task_type FROM tasks AS t ` +
		`WHERE NOT EXISTS ( ` +
		`	SELECT domain_id, name, task_type FROM task_lists AS tl ` +
		`	WHERE t.domain_id=tl.domain_id and t.task_list_name=tl.name and t.task_type=tl.task_type ` +
		`) LIMIT ?;`
)

// InsertIntoTasks inserts one or more rows into tasks table
func (mdb *DB) InsertIntoTasks(ctx context.Context, rows []sqlplugin.TasksRow) (sql.Result, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	return mdb.driver.NamedExecContext(ctx, rows[0].ShardID, createTaskQry, rows)
}

// SelectFromTasks reads one or more rows from tasks table
func (mdb *DB) SelectFromTasks(ctx context.Context, filter *sqlplugin.TasksFilter) ([]sqlplugin.TasksRow, error) {
	var err error
	var rows []sqlplugin.TasksRow
	switch {
	case filter.MaxTaskID != nil:
		err = mdb.driver.SelectContext(ctx, filter.ShardID, &rows, getTaskMinMaxQry, filter.DomainID,
			filter.TaskListName, filter.TaskType, *filter.MinTaskID, *filter.MaxTaskID, *filter.PageSize)
	default:
		err = mdb.driver.SelectContext(ctx, filter.ShardID, &rows, getTaskMinQry, filter.DomainID,
			filter.TaskListName, filter.TaskType, *filter.MinTaskID, *filter.PageSize)
	}
	if err != nil {
		return nil, err
	}
	return rows, err
}

// DeleteFromTasks deletes one or more rows from tasks table
func (mdb *DB) DeleteFromTasks(ctx context.Context, filter *sqlplugin.TasksFilter) (sql.Result, error) {
	if filter.TaskIDLessThanEquals != nil {
		if filter.Limit == nil || *filter.Limit == 0 {
			return nil, fmt.Errorf("missing limit parameter")
		}
		return mdb.driver.ExecContext(ctx, filter.ShardID, rangeDeleteTaskQry,
			filter.DomainID, filter.TaskListName, filter.TaskType, *filter.TaskIDLessThanEquals, *filter.Limit)
	}
	return mdb.driver.ExecContext(ctx, filter.ShardID, deleteTaskQry, filter.DomainID, filter.TaskListName, filter.TaskType, *filter.TaskID)
}

func (mdb *DB) GetOrphanTasks(ctx context.Context, filter *sqlplugin.OrphanTasksFilter) ([]sqlplugin.TaskKeyRow, error) {
	if filter.Limit == nil || *filter.Limit == 0 {
		return nil, fmt.Errorf("missing limit parameter")
	}
	var rows []sqlplugin.TaskKeyRow

	err := mdb.driver.SelectContext(ctx, sqlplugin.DbAllShards, &rows, getOrphanTaskQry, *filter.Limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// InsertIntoTaskLists inserts one or more rows into task_lists table
func (mdb *DB) InsertIntoTaskLists(ctx context.Context, row *sqlplugin.TaskListsRow) (sql.Result, error) {
	return mdb.driver.NamedExecContext(ctx, row.ShardID, createTaskListQry, row)
}

// UpdateTaskLists updates a row in task_lists table
func (mdb *DB) UpdateTaskLists(ctx context.Context, row *sqlplugin.TaskListsRow) (sql.Result, error) {
	return mdb.driver.NamedExecContext(ctx, row.ShardID, updateTaskListQry, row)
}

// SelectFromTaskLists reads one or more rows from task_lists table
func (mdb *DB) SelectFromTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
	switch {
	case filter.DomainID != nil && filter.Name != nil && filter.TaskType != nil:
		return mdb.selectFromTaskLists(ctx, filter)
	case filter.DomainIDGreaterThan != nil && filter.NameGreaterThan != nil && filter.TaskTypeGreaterThan != nil && filter.PageSize != nil:
		return mdb.rangeSelectFromTaskLists(ctx, filter)
	default:
		return nil, fmt.Errorf("invalid set of query filter params")
	}
}

func (mdb *DB) selectFromTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
	var err error
	var row sqlplugin.TaskListsRow
	err = mdb.driver.GetContext(ctx, filter.ShardID, &row, getTaskListQry, filter.ShardID, *filter.DomainID, *filter.Name, *filter.TaskType)
	if err != nil {
		return nil, err
	}
	return []sqlplugin.TaskListsRow{row}, err
}

func (mdb *DB) rangeSelectFromTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) ([]sqlplugin.TaskListsRow, error) {
	var err error
	var rows []sqlplugin.TaskListsRow
	err = mdb.driver.SelectContext(ctx, filter.ShardID, &rows, listTaskListQry,
		filter.ShardID, *filter.DomainIDGreaterThan, *filter.NameGreaterThan, *filter.TaskTypeGreaterThan, *filter.DomainIDGreaterThan, *filter.NameGreaterThan, *filter.DomainIDGreaterThan, *filter.PageSize)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].ShardID = filter.ShardID
	}
	return rows, nil
}

// DeleteFromTaskLists deletes a row from task_lists table
func (mdb *DB) DeleteFromTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) (sql.Result, error) {
	return mdb.driver.ExecContext(ctx, filter.ShardID, deleteTaskListQry, filter.ShardID, *filter.DomainID, *filter.Name, *filter.TaskType, *filter.RangeID)
}

// LockTaskLists locks a row in task_lists table
func (mdb *DB) LockTaskLists(ctx context.Context, filter *sqlplugin.TaskListsFilter) (int64, error) {
	var rangeID int64
	err := mdb.driver.GetContext(ctx, filter.ShardID, &rangeID, lockTaskListQry, filter.ShardID, *filter.DomainID, *filter.Name, *filter.TaskType)
	return rangeID, err
}

func (mdb *DB) GetTasksCount(ctx context.Context, filter *sqlplugin.TasksFilter) (int64, error) {
	var size []int64
	if err := mdb.driver.SelectContext(ctx, filter.ShardID, &size, getTasksCountQry, filter.DomainID, filter.TaskListName, filter.TaskType, *filter.MinTaskID); err != nil {
		return 0, err
	}
	return size[0], nil
}

// InsertIntoTasksWithTTL is not supported in MySQL
func (mdb *DB) InsertIntoTasksWithTTL(_ context.Context, _ []sqlplugin.TasksRowWithTTL) (sql.Result, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}

// InsertIntoTaskListsWithTTL is not supported in MySQL
func (mdb *DB) InsertIntoTaskListsWithTTL(_ context.Context, _ *sqlplugin.TaskListsRowWithTTL) (sql.Result, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}

// UpdateTaskListsWithTTL is not supported in MySQL
func (mdb *DB) UpdateTaskListsWithTTL(_ context.Context, _ *sqlplugin.TaskListsRowWithTTL) (sql.Result, error) {
	return nil, sqlplugin.ErrTTLNotSupported
}
