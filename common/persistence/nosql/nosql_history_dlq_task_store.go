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
		WorkflowID:            request.WorkflowID,
		RunID:                 request.RunID,
		Version:               request.Version,
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
