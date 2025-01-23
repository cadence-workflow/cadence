// Copyright (c) 2021 Uber Technologies, Inc.
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

package mongodb

import (
	"context"
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// SelectTaskList returns a single tasklist row.
// Return IsNotFoundError if the row doesn't exist
func (db *mdb) SelectTaskList(ctx context.Context, filter *nosqlplugin.TaskListFilter) (*nosqlplugin.TaskListRow, error) {
	panic("TODO")
}

// InsertTaskList insert a single tasklist row
// Return IsConditionFailedError if the row already exists, and also the existing row
func (db *mdb) InsertTaskList(ctx context.Context, row *nosqlplugin.TaskListRow) error {
	panic("TODO")
}

// UpdateTaskList updates a single tasklist row
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *mdb) UpdateTaskList(
	ctx context.Context,
	row *nosqlplugin.TaskListRow,
	previousRangeID int64,
) error {
	panic("TODO")
}

// UpdateTaskList updates a single tasklist row, and set an TTL on the record
// Return TaskOperationConditionFailure if the condition doesn't meet
// Ignore TTL if it's not supported, which becomes exactly the same as UpdateTaskList, but ListTaskList must be
// implemented for TaskListScavenger
func (db *mdb) UpdateTaskListWithTTL(
	ctx context.Context,
	ttlSeconds int64,
	row *nosqlplugin.TaskListRow,
	previousRangeID int64,
	timeStamp time.Time,
) error {
	panic("TODO")
}

// ListTaskList returns all tasklists.
// Noop if TTL is already implemented in other methods
func (db *mdb) ListTaskList(ctx context.Context, pageSize int, nextPageToken []byte) (*nosqlplugin.ListTaskListResult, error) {
	panic("TODO")
}

// DeleteTaskList deletes a single tasklist row
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *mdb) DeleteTaskList(ctx context.Context, filter *nosqlplugin.TaskListFilter, previousRangeID int64) error {
	panic("TODO")
}

// InsertTasks inserts a batch of tasks
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *mdb) InsertTasks(
	ctx context.Context,
	tasksToInsert []*nosqlplugin.TaskRowForInsert,
	tasklistCondition *nosqlplugin.TaskListRow,
) error {
	panic("TODO")
}

// SelectTasks return tasks that associated to a tasklist
func (db *mdb) SelectTasks(ctx context.Context, filter *nosqlplugin.TasksFilter) ([]*nosqlplugin.TaskRow, error) {
	panic("TODO")
}

func (db *mdb) GetTasksCount(ctx context.Context, filter *nosqlplugin.TasksFilter) (int64, error) {
	panic("TODO")
}

// DeleteTask delete a batch tasks that taskIDs less than the row
// If TTL is not implemented, then should also return the number of rows deleted, otherwise persistence.UnknownNumRowsAffected
// NOTE: This API ignores the `BatchSize` request parameter i.e. either all tasks leq the task_id will be deleted or an error will
// be returned to the caller, because rowsDeleted is not supported by Cassandra
func (db *mdb) RangeDeleteTasks(ctx context.Context, filter *nosqlplugin.TasksFilter) (rowsDeleted int, err error) {
	panic("TODO")
}
