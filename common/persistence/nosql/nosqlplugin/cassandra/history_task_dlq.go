// Copyright (c) 2020 Uber Technologies, Inc.
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

import (
	"context"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

// InsertHistoryDLQTaskRow writes a task to the history DLQ.
// Uses the dedicated history_task_dlq table, partitioned by
// (shard_id, domain_id, cluster_attribute_scope, cluster_attribute_name).
func (db *CDB) InsertHistoryDLQTaskRow(
	ctx context.Context,
	task *nosqlplugin.HistoryDLQTaskRow,
) error {
	query := db.session.Query(templateInsertHistoryDLQTaskRowQuery,
		task.ShardID,
		task.DomainID,
		task.ClusterAttributeScope,
		task.ClusterAttributeName,
		task.TaskType,
		task.VisibilityTimestamp,
		task.TaskID,
		task.WorkflowID,
		task.RunID,
		task.Version,
		task.Data,
		task.DataEncoding,
		task.CreatedAt,
	).WithContext(ctx)
	return query.Exec()
}
