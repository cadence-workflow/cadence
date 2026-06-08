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
	shardedNosqlStore
}

// newNoSQLHistoryDLQTaskStore creates an instance of HistoryDLQTaskStore backed by NoSQL.
func newNoSQLHistoryDLQTaskStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.HistoryDLQTaskStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, false)
	if err != nil {
		return nil, err
	}
	return &nosqlHistoryDLQTaskStore{
		shardedNosqlStore: shardedStore,
	}, nil
}

func (sh *nosqlHistoryDLQTaskStore) GetName() string {
	return sh.GetDefaultShard().db.PluginName()
}

// CreateHistoryDLQTask writes a task to the history DLQ.
func (sh *nosqlHistoryDLQTaskStore) CreateHistoryDLQTask(
	ctx context.Context,
	request persistence.InternalCreateHistoryDLQTaskRequest,
) error {
	if request.TaskBlob == nil {
		sh.GetLogger().Warn("unable to persist history DLQ task: task blob is required")
		return &persistence.InvalidPersistenceRequestError{
			Msg: "unable to persist history DLQ task: task blob is required",
		}
	}

	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return err
	}

	row := &nosqlplugin.HistoryDLQTaskRow{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
		TaskType:              request.TaskType,
		TaskID:                request.TaskID,
		WorkflowID:            request.WorkflowID,
		RunID:                 request.RunID,
		Version:               request.Version,
		VisibilityTimestamp:   request.VisibilityTimestamp,
		Data:                  request.TaskBlob.Data,
		DataEncoding:          string(request.TaskBlob.Encoding),
		CreatedAt:             request.CreatedAt,
	}

	err = storeShard.db.InsertHistoryDLQTaskRow(ctx, row)
	if err != nil {
		return convertCommonErrors(storeShard.db, "CreateHistoryDLQTask", err)
	}
	return nil
}

// GetHistoryDLQTasks reads paginated tasks from the history DLQ.
func (sh *nosqlHistoryDLQTaskStore) GetHistoryDLQTasks(
	ctx context.Context,
	request persistence.HistoryDLQGetTasksRequest,
) (persistence.InternalGetHistoryDLQTasksResponse, error) {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return persistence.InternalGetHistoryDLQTasksResponse{}, err
	}

	rows, nextPageToken, err := storeShard.db.SelectHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskFilter{
		ShardID:                  request.ShardID,
		DomainID:                 request.DomainID,
		ClusterAttributeScope:    request.ClusterAttributeScope,
		ClusterAttributeName:     request.ClusterAttributeName,
		TaskType:                 request.TaskCategory.ID(),
		InclusiveMinVisibilityTS: request.InclusiveMinTaskKey.GetScheduledTime(),
		InclusiveMinTaskID:       request.InclusiveMinTaskKey.GetTaskID(),
		ExclusiveMaxVisibilityTS: request.ExclusiveMaxTaskKey.GetScheduledTime(),
		ExclusiveMaxTaskID:       request.ExclusiveMaxTaskKey.GetTaskID(),
		PageSize:                 request.PageSize,
		NextPageToken:            request.NextPageToken,
	})
	if err != nil {
		return persistence.InternalGetHistoryDLQTasksResponse{}, convertCommonErrors(storeShard.db, "GetHistoryDLQTasks", err)
	}

	tasks := make([]*persistence.InternalHistoryDLQTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, &persistence.InternalHistoryDLQTask{
			DomainID:              row.DomainID,
			ClusterAttributeScope: row.ClusterAttributeScope,
			ClusterAttributeName:  row.ClusterAttributeName,
			TaskCategory:          row.TaskType,
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

// RangeDeleteHistoryDLQTasks deletes all tasks strictly before the exclusive max key.
func (sh *nosqlHistoryDLQTaskStore) RangeDeleteHistoryDLQTasks(
	ctx context.Context,
	request persistence.HistoryDLQDeleteTasksRequest,
) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return err
	}

	err = storeShard.db.RangeDeleteHistoryDLQTaskRows(ctx, nosqlplugin.HistoryDLQTaskRangeDeleteFilter{
		ShardID:                  request.ShardID,
		DomainID:                 request.DomainID,
		ClusterAttributeScope:    request.ClusterAttributeScope,
		ClusterAttributeName:     request.ClusterAttributeName,
		TaskType:                 request.TaskCategory.ID(),
		ExclusiveMaxVisibilityTS: request.ExclusiveMaxTaskKey.GetScheduledTime(),
		ExclusiveMaxTaskID:       request.ExclusiveMaxTaskKey.GetTaskID(),
	})
	if err != nil {
		return convertCommonErrors(storeShard.db, "RangeDeleteHistoryDLQTasks", err)
	}
	return nil
}

// GetHistoryDLQAckLevels reads ack-level rows for a shard, filtered by task category in application code.
func (sh *nosqlHistoryDLQTaskStore) GetHistoryDLQAckLevels(
	ctx context.Context,
	request persistence.HistoryDLQGetAckLevelsRequest,
) (persistence.InternalGetHistoryDLQAckLevelsResponse, error) {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.ShardID)
	if err != nil {
		return persistence.InternalGetHistoryDLQAckLevelsResponse{}, err
	}

	rows, err := storeShard.db.SelectHistoryDLQAckLevelRows(ctx, nosqlplugin.HistoryDLQAckLevelFilter{
		ShardID:               request.ShardID,
		DomainID:              request.DomainID,
		ClusterAttributeScope: request.ClusterAttributeScope,
		ClusterAttributeName:  request.ClusterAttributeName,
	})
	if err != nil {
		return persistence.InternalGetHistoryDLQAckLevelsResponse{}, convertCommonErrors(storeShard.db, "GetHistoryDLQAckLevels", err)
	}

	ackLevels := make([]*persistence.InternalHistoryDLQAckLevel, 0, len(rows))
	for _, row := range rows {
		ackLevels = append(ackLevels, &persistence.InternalHistoryDLQAckLevel{
			ShardID:               row.ShardID,
			DomainID:              row.DomainID,
			ClusterAttributeScope: row.ClusterAttributeScope,
			ClusterAttributeName:  row.ClusterAttributeName,
			TaskCategory:          row.TaskType,
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
func (sh *nosqlHistoryDLQTaskStore) UpdateHistoryDLQAckLevel(
	ctx context.Context,
	request persistence.InternalUpdateHistoryDLQAckLevelRequest,
) error {
	storeShard, err := sh.GetStoreShardByHistoryShard(request.Row.ShardID)
	if err != nil {
		return err
	}

	err = storeShard.db.InsertOrUpdateHistoryDLQAckLevelRow(ctx, &nosqlplugin.HistoryDLQAckLevelRow{
		ShardID:               request.Row.ShardID,
		DomainID:              request.Row.DomainID,
		ClusterAttributeScope: request.Row.ClusterAttributeScope,
		ClusterAttributeName:  request.Row.ClusterAttributeName,
		TaskType:              request.Row.TaskCategory,
		AckLevelVisibilityTS:  request.Row.AckLevelVisibilityTS,
		AckLevelTaskID:        request.Row.AckLevelTaskID,
		LastUpdatedAt:         request.Row.LastUpdatedAt,
	})
	if err != nil {
		return convertCommonErrors(storeShard.db, "UpdateHistoryDLQAckLevel", err)
	}
	return nil
}
