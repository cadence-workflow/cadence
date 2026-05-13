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
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
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

// SelectHistoryDLQTaskRows reads paginated tasks from the history DLQ within the given bounds.
func (db *CDB) SelectHistoryDLQTaskRows(
	ctx context.Context,
	filter nosqlplugin.HistoryDLQTaskFilter,
) ([]*nosqlplugin.HistoryDLQTaskRow, []byte, error) {
	query := db.session.Query(templateSelectHistoryDLQTaskRowsQuery,
		filter.ShardID,
		filter.DomainID,
		filter.ClusterAttributeScope,
		filter.ClusterAttributeName,
		filter.TaskType,
		filter.InclusiveMinVisibilityTS,
		filter.InclusiveMinTaskID,
		filter.ExclusiveMaxVisibilityTS,
		filter.ExclusiveMaxTaskID,
	).WithContext(ctx)

	if filter.PageSize > 0 {
		query = query.PageSize(filter.PageSize)
	}

	if len(filter.NextPageToken) > 0 {
		query = query.PageState(filter.NextPageToken)
	}

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectHistoryDLQTaskRows operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.HistoryDLQTaskRow
	row := &nosqlplugin.HistoryDLQTaskRow{}
	for iter.Scan(
		&row.ShardID,
		&row.DomainID,
		&row.ClusterAttributeScope,
		&row.ClusterAttributeName,
		&row.TaskType,
		&row.VisibilityTimestamp,
		&row.TaskID,
		&row.Data,
		&row.DataEncoding,
		&row.CreatedAt,
	) {
		rows = append(rows, row)
		row = &nosqlplugin.HistoryDLQTaskRow{}

		if filter.PageSize > 0 && len(rows) >= filter.PageSize {
			break
		}
	}

	nextPageToken := iter.PageState()
	if err := iter.Close(); err != nil {
		return nil, nil, err
	}
	return rows, nextPageToken, nil
}

// RangeDeleteHistoryDLQTaskRows deletes all tasks up to and including the given ack-level bounds.
func (db *CDB) RangeDeleteHistoryDLQTaskRows(
	ctx context.Context,
	filter nosqlplugin.HistoryDLQTaskRangeDeleteFilter,
) error {
	query := db.session.Query(templateRangeDeleteHistoryDLQTaskRowsQuery,
		filter.ShardID,
		filter.DomainID,
		filter.ClusterAttributeScope,
		filter.ClusterAttributeName,
		filter.TaskType,
		filter.ExclusiveMaxVisibilityTS,
		filter.ExclusiveMaxTaskID,
	).WithContext(ctx)
	return query.Exec()
}

// SelectHistoryDLQAckLevelRows reads ack-level rows for a shard.
// If domainID is non-empty the query is restricted to that domain.
// If clusterAttributeScope and clusterAttributeName are also non-empty it is
// further restricted to that cluster attribute. All filtering is done at the
// DB layer using the partition/clustering key columns.
func (db *CDB) SelectHistoryDLQAckLevelRows(
	ctx context.Context,
	filter nosqlplugin.HistoryDLQAckLevelFilter,
) ([]*nosqlplugin.HistoryDLQAckLevelRow, error) {
	var iter gocql.Iter
	switch {
	case filter.DomainID != "" && filter.ClusterAttributeScope != "" && filter.ClusterAttributeName != "":
		iter = db.session.Query(templateSelectHistoryDLQAckLevelRowsByClusterAttributeQuery,
			filter.ShardID, filter.DomainID, filter.ClusterAttributeScope, filter.ClusterAttributeName,
		).WithContext(ctx).Iter()
	case filter.DomainID != "":
		iter = db.session.Query(templateSelectHistoryDLQAckLevelRowsByDomainQuery,
			filter.ShardID, filter.DomainID,
		).WithContext(ctx).Iter()
	default:
		iter = db.session.Query(templateSelectHistoryDLQAckLevelRowsQuery,
			filter.ShardID,
		).WithContext(ctx).Iter()
	}

	if iter == nil {
		return nil, &types.InternalServiceError{
			Message: "SelectHistoryDLQAckLevelRows operation failed. Not able to create query iterator.",
		}
	}

	var rows []*nosqlplugin.HistoryDLQAckLevelRow
	row := make(map[string]interface{})
	for iter.MapScan(row) {
		rows = append(rows, parseHistoryDLQAckLevelRow(filter.ShardID, row))
		row = make(map[string]interface{})
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}
	return rows, nil
}

// InsertOrUpdateHistoryDLQAckLevelRow upserts a single ack-level row.
func (db *CDB) InsertOrUpdateHistoryDLQAckLevelRow(
	ctx context.Context,
	row *nosqlplugin.HistoryDLQAckLevelRow,
) error {
	query := db.session.Query(templateUpsertHistoryDLQAckLevelRowQuery,
		row.ShardID,
		row.DomainID,
		row.ClusterAttributeScope,
		row.ClusterAttributeName,
		row.TaskType,
		row.AckLevelVisibilityTS,
		row.AckLevelTaskID,
		row.LastUpdatedAt,
	).WithContext(ctx)
	return query.Exec()
}

func parseHistoryDLQAckLevelRow(shardID int, row map[string]interface{}) *nosqlplugin.HistoryDLQAckLevelRow {
	return &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               shardID,
		DomainID:              row["domain_id"].(string),
		ClusterAttributeScope: row["cluster_attribute_scope"].(string),
		ClusterAttributeName:  row["cluster_attribute_name"].(string),
		TaskType:              row["task_type"].(int),
		AckLevelVisibilityTS:  row["ack_level_visibility_ts"].(time.Time),
		AckLevelTaskID:        row["ack_level_task_id"].(int64),
		LastUpdatedAt:         row["last_updated_at"].(time.Time),
	}
}
