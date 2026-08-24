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

	// Control-row (type=1) queries: carry the range_id fence and the ack_level cursor.

	templateGetSemaphoreTaskControlRowQuery = `SELECT range_id, ack_level ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ?`

	templateInsertSemaphoreTaskControlRowQuery = `INSERT INTO semaphore_tasks (` +
		`domain_id, semaphore_name, bucket, type, task_id, range_id, ack_level, created_time` +
		`) VALUES (?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`

	templateUpdateSemaphoreTaskControlRowQuery = `UPDATE semaphore_tasks SET range_id = ?, ack_level = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ? ` +
		`IF range_id = ?`

	// templateUpdateSemaphoreTaskControlRangeIDQuery re-asserts range_id (a no-op write to the same
	// value) inside the task-insert batch, fencing out a stale writer. It never touches ack_level.
	templateUpdateSemaphoreTaskControlRangeIDQuery = `UPDATE semaphore_tasks SET range_id = ? ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id = ? ` +
		`IF range_id = ?`

	// Task-row (type=0) queries.

	templateCreateSemaphoreTaskQuery = `INSERT INTO semaphore_tasks (` +
		`domain_id, semaphore_name, bucket, type, task_id, task, acquire_deadline, created_time` +
		`) VALUES (?, ?, ?, ?, ?, ` + templateSemaphoreTaskType + `, ?, ?)`

	templateGetSemaphoreTasksQuery = `SELECT task_id, task, acquire_deadline, created_time ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? ` +
		`AND task_id > ? AND task_id <= ?`

	templateGetSemaphoreTasksCountQuery = `SELECT count(1) as count ` +
		`FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? AND task_id > ?`

	templateDeleteSemaphoreTasksLessThanQuery = `DELETE FROM semaphore_tasks ` +
		`WHERE domain_id = ? AND semaphore_name = ? AND bucket = ? AND type = ? ` +
		`AND task_id > ? AND task_id <= ?`
)
