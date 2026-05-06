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

package nosql

import (
	"context"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

type nosqlHistoryDLQTaskStore struct {
	nosqlStore
}

// newNoSQLHistoryDLQTaskStore creates an instance of HistoryDLQTaskStore backed by NoSQL.
func newNoSQLHistoryDLQTaskStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.HistoryDLQTaskStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlHistoryDLQTaskStore{
		nosqlStore: shardedStore.GetDefaultShard(),
	}, nil
}

// CreateHistoryDLQTask writes a task to the history DLQ.
func (m *nosqlHistoryDLQTaskStore) CreateHistoryDLQTask(
	ctx context.Context,
	request persistence.InternalCreateHistoryDLQTaskRequest,
) error {
	if request.TaskBlob == nil {
		m.logger.Warn("unable to persist history DLQ task: task blob is required")
		return &persistence.InvalidPersistenceRequestError{
			Msg: "unable to persist history DLQ task: task blob is required",
		}
	}

	row := &nosqlplugin.HistoryDLQTaskRow{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
		TaskType:              request.TaskType,
		TaskID:                request.TaskID,
		VisibilityTimestamp:   request.VisibilityTimestamp,
		Data:                  request.TaskBlob.Data,
		DataEncoding:          string(request.TaskBlob.Encoding),
		CreatedAt:             request.CreatedAt,
	}

	err := m.db.InsertHistoryDLQTaskRow(
		ctx,
		row,
	)
	if err != nil {
		return convertCommonErrors(m.db, "CreateHistoryDLQTask", err)
	}
	return nil
}

func (m *nosqlHistoryDLQTaskStore) GetName() string { return m.db.PluginName() }
func (m *nosqlHistoryDLQTaskStore) Close()          {}

// GetHistoryDLQTasks reads paginated tasks from the history DLQ.
func (m *nosqlHistoryDLQTaskStore) GetHistoryDLQTasks(
	ctx context.Context,
	request persistence.InternalGetHistoryDLQTasksRequest,
) (persistence.InternalGetHistoryDLQTasksResponse, error) {
	rows, nextPageToken, err := m.db.SelectHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskFilter{
		ShardID:                  request.ShardID,
		DomainID:                 request.DomainID,
		ClusterAttributeScope:    request.ClusterAttributeScope,
		ClusterAttributeName:     request.ClusterAttributeName,
		TaskType:                 request.TaskType,
		ExclusiveMinVisibilityTS: request.ExclusiveMinVisibilityTS,
		ExclusiveMinTaskID:       request.ExclusiveMinTaskID,
		InclusiveMaxVisibilityTS: request.InclusiveMaxVisibilityTS,
		InclusiveMaxTaskID:       request.InclusiveMaxTaskID,
		PageSize:                 request.PageSize,
		NextPageToken:            request.NextPageToken,
	})
	if err != nil {
		return persistence.InternalGetHistoryDLQTasksResponse{}, convertCommonErrors(m.db, "GetHistoryDLQTasks", err)
	}

	tasks := make([]*persistence.InternalHistoryDLQTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, &persistence.InternalHistoryDLQTask{
			DomainID:              row.DomainID,
			ClusterAttributeScope: row.ClusterAttributeScope,
			ClusterAttributeName:  row.ClusterAttributeName,
			TaskType:              row.TaskType,
			VisibilityTimestamp:   row.VisibilityTimestamp,
			TaskID:                row.TaskID,
			TaskPayload:           &persistence.DataBlob{Data: row.Data, Encoding: constants.EncodingType(row.DataEncoding)},
			CreatedAt:             row.CreatedAt,
		})
	}
	return persistence.InternalGetHistoryDLQTasksResponse{
		Tasks:         tasks,
		NextPageToken: nextPageToken,
	}, nil
}

// RangeDeleteHistoryDLQTasks deletes all tasks up to and including the given ack-level bounds.
func (m *nosqlHistoryDLQTaskStore) RangeDeleteHistoryDLQTasks(
	ctx context.Context,
	request persistence.InternalRangeDeleteHistoryDLQTasksRequest,
) error {
	err := m.db.RangeDeleteHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskRangeDeleteFilter{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
		TaskType:              request.TaskType,
		AckLevelVisibilityTS:  request.AckLevelVisibilityTS,
		AckLevelTaskID:        request.AckLevelTaskID,
	})
	if err != nil {
		return convertCommonErrors(m.db, "RangeDeleteHistoryDLQTasks", err)
	}
	return nil
}

// GetHistoryDLQAckLevels reads ack-level rows for a shard.
func (m *nosqlHistoryDLQTaskStore) GetHistoryDLQAckLevels(
	ctx context.Context,
	request persistence.InternalGetHistoryDLQAckLevelsRequest,
) (persistence.InternalGetHistoryDLQAckLevelsResponse, error) {
	rows, err := m.db.SelectHistoryDLQAckLevelRows(ctx, nosqlplugin.HistoryDLQAckLevelFilter{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
	})
	if err != nil {
		return persistence.InternalGetHistoryDLQAckLevelsResponse{}, convertCommonErrors(m.db, "GetHistoryDLQAckLevels", err)
	}

	ackLevels := make([]*persistence.InternalHistoryDLQAckLevel, 0, len(rows))
	for _, row := range rows {
		ackLevels = append(ackLevels, &persistence.InternalHistoryDLQAckLevel{
			ShardID:               row.ShardID,
			DomainID:              row.DomainID,
			ClusterAttributeScope: row.ClusterAttributeScope,
			ClusterAttributeName:  row.ClusterAttributeName,
			TaskType:              row.TaskType,
			AckLevelVisibilityTS:  row.AckLevelVisibilityTS,
			AckLevelTaskID:        row.AckLevelTaskID,
			LastUpdatedAt:         row.LastUpdatedAt,
		})
	}
	return persistence.InternalGetHistoryDLQAckLevelsResponse{
		AckLevels: ackLevels,
	}, nil
}

// UpdateHistoryDLQAckLevel upserts a single ack-level row.
func (m *nosqlHistoryDLQTaskStore) UpdateHistoryDLQAckLevel(
	ctx context.Context,
	request persistence.InternalUpdateHistoryDLQAckLevelRequest,
) error {
	err := m.db.InsertOrUpdateHistoryDLQAckLevelRow(ctx, &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               request.Row.ShardID,
		DomainID:              request.Row.DomainID,
		ClusterAttributeScope: request.Row.ClusterAttributeScope,
		ClusterAttributeName:  request.Row.ClusterAttributeName,
		TaskType:              request.Row.TaskType,
		AckLevelVisibilityTS:  request.Row.AckLevelVisibilityTS,
		AckLevelTaskID:        request.Row.AckLevelTaskID,
		LastUpdatedAt:         request.Row.LastUpdatedAt,
	})
	if err != nil {
		return convertCommonErrors(m.db, "UpdateHistoryDLQAckLevel", err)
	}
	return nil
}
