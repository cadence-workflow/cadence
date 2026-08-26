// Copyright (c) 2025 Uber Technologies, Inc.
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

package cassandra

const (
	// templateSemaphoreTaskType is the frozen<semaphore_task> UDT literal for a task row.
	templateSemaphoreTaskType = `{` +
		`workflow_id: ?, ` +
		`run_id: ?, ` +
		`hold_id: ? ` +
		`}`

	// Control-row (type=1) queries.

	// Read the range_id fence and the ack_level cursor.
	templateGetSemaphoreTaskControlRowQuery = `SELECT range_id, ack_level ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ?`

	// Creates a bucket's control row on first use. IF NOT EXISTS so two hosts racing the first
	// claim cannot both create it: one wins, the other gets a fence error.
	templateInsertSemaphoreTaskControlRowQuery = `INSERT INTO semaphore_tasks (` +
		`domain_id, semaphore_name, bucket, type, task_id, range_id, ack_level, created_time` +
		`) VALUES (?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	// Bumps range_id and writes ack_level, fenced on the caller's previous range_id. Both columns
	// are always written, so a caller changing one echoes the other back unchanged.
	templateUpdateSemaphoreTaskControlRowQuery = `UPDATE semaphore_tasks SET range_id = ?, ack_level = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ? ` +
		`IF range_id = ?`

	// Re-asserts range_id (a no-op write to the same value) inside the task-insert batch,
	// fencing out a stale writer. It never touches ack_level.
	templateUpdateSemaphoreTaskControlRangeIDQuery = `UPDATE semaphore_tasks SET range_id = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ? ` +
		`IF range_id = ?`

	// Task-row (type=0) queries.

	// Enqueues one waiter. Deliberately unconditional: the fence is the control-row re-assert that
	// rides the same batch, so this must never be issued on its own.
	templateCreateSemaphoreTaskQuery = `INSERT INTO semaphore_tasks (` +
		`domain_id, semaphore_name, bucket, type, task_id, task, acquire_deadline, created_time` +
		`) VALUES (?, ?, ?, ?, ?, ` + templateSemaphoreTaskType + `, ?, ?)`

	// Reads a page of the queue in (ExclusiveMinTaskID, InclusiveMaxTaskID], ascending by task_id
	// (the clustering order). type=0 keeps the control row out of the result.
	templateGetSemaphoreTasksQuery = `SELECT task_id, task, acquire_deadline, created_time ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? ` +
		`AND task_id > ? AND task_id <= ?`

	// Counts undrained tasks above a bound. This scans the rest of the partition, so it is a
	// diagnostic rather than a hot-path check.
	templateGetSemaphoreTasksCountQuery = `SELECT count(1) as count ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id > ?`

	// Range-deletes the drained prefix. Scoped to type=0, and the control row's task_id is
	// negative, so the fence row can never be caught by this.
	templateRangeDeleteSemaphoreTasksQuery = `DELETE FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? ` +
		`AND task_id > ? AND task_id <= ?`
)
