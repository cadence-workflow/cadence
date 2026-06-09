// Copyright (c) 2026 Uber Technologies, Inc.
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

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

type shardedExecutionStore struct {
	logger         log.Logger
	store          shardedNosqlStore
	taskSerializer serialization.TaskSerializer
}

var _ persistence.ExecutionStore = (*shardedExecutionStore)(nil)

func NewShardedExecutionStore(
	store shardedNosqlStore,
	logger log.Logger,
	taskSerializer serialization.TaskSerializer,
) persistence.ExecutionStore {
	return &shardedExecutionStore{
		logger:         logger,
		store:          store,
		taskSerializer: taskSerializer,
	}
}

func (d *shardedExecutionStore) GetName() string {
	return d.store.GetName()
}

// Close is a no-op; the underlying shardedNosqlStore is owned and closed by executionStoreFactory.
func (d *shardedExecutionStore) Close() {}

func (d *shardedExecutionStore) executionStore(requestShardID *int, operation string) (persistence.ExecutionStore, error) {
	if requestShardID == nil {
		err := &types.BadRequestError{Message: "execution persistence request missing shard ID"}
		if d.logger != nil {
			d.logger.Warn("execution persistence request missing shard ID", tag.OperationName(operation), tag.Error(err))
		}
		return nil, err
	}
	storeShard, err := d.store.GetStoreShardByHistoryShard(*requestShardID)
	if err != nil {
		return nil, err
	}
	return NewExecutionStore(*requestShardID, storeShard.db, d.logger, d.taskSerializer, storeShard.dc)
}

func (d *shardedExecutionStore) CreateWorkflowExecution(ctx context.Context, request *persistence.InternalCreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error) {
	store, err := d.executionStore(request.ShardID, "CreateWorkflowExecution")
	if err != nil {
		return nil, err
	}
	return store.CreateWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) GetWorkflowExecution(ctx context.Context, request *persistence.InternalGetWorkflowExecutionRequest) (*persistence.InternalGetWorkflowExecutionResponse, error) {
	store, err := d.executionStore(request.ShardID, "GetWorkflowExecution")
	if err != nil {
		return nil, err
	}
	return store.GetWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) UpdateWorkflowExecution(ctx context.Context, request *persistence.InternalUpdateWorkflowExecutionRequest) error {
	store, err := d.executionStore(request.ShardID, "UpdateWorkflowExecution")
	if err != nil {
		return err
	}
	return store.UpdateWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) ConflictResolveWorkflowExecution(ctx context.Context, request *persistence.InternalConflictResolveWorkflowExecutionRequest) error {
	store, err := d.executionStore(request.ShardID, "ConflictResolveWorkflowExecution")
	if err != nil {
		return err
	}
	return store.ConflictResolveWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) DeleteWorkflowExecution(ctx context.Context, request *persistence.DeleteWorkflowExecutionRequest) error {
	store, err := d.executionStore(request.ShardID, "DeleteWorkflowExecution")
	if err != nil {
		return err
	}
	return store.DeleteWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) DeleteCurrentWorkflowExecution(ctx context.Context, request *persistence.DeleteCurrentWorkflowExecutionRequest) error {
	store, err := d.executionStore(request.ShardID, "DeleteCurrentWorkflowExecution")
	if err != nil {
		return err
	}
	return store.DeleteCurrentWorkflowExecution(ctx, request)
}

func (d *shardedExecutionStore) GetCurrentExecution(ctx context.Context, request *persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error) {
	store, err := d.executionStore(request.ShardID, "GetCurrentExecution")
	if err != nil {
		return nil, err
	}
	return store.GetCurrentExecution(ctx, request)
}

func (d *shardedExecutionStore) IsWorkflowExecutionExists(ctx context.Context, request *persistence.IsWorkflowExecutionExistsRequest) (*persistence.IsWorkflowExecutionExistsResponse, error) {
	store, err := d.executionStore(request.ShardID, "IsWorkflowExecutionExists")
	if err != nil {
		return nil, err
	}
	return store.IsWorkflowExecutionExists(ctx, request)
}

func (d *shardedExecutionStore) PutReplicationTaskToDLQ(ctx context.Context, request *persistence.InternalPutReplicationTaskToDLQRequest) error {
	store, err := d.executionStore(request.ShardID, "PutReplicationTaskToDLQ")
	if err != nil {
		return err
	}
	return store.PutReplicationTaskToDLQ(ctx, request)
}

func (d *shardedExecutionStore) GetReplicationTasksFromDLQ(ctx context.Context, request *persistence.GetReplicationTasksFromDLQRequest) (*persistence.InternalGetReplicationDLQTasksResponse, error) {
	store, err := d.executionStore(request.ShardID, "GetReplicationTasksFromDLQ")
	if err != nil {
		return nil, err
	}
	return store.GetReplicationTasksFromDLQ(ctx, request)
}

func (d *shardedExecutionStore) GetReplicationDLQSize(ctx context.Context, request *persistence.GetReplicationDLQSizeRequest) (*persistence.GetReplicationDLQSizeResponse, error) {
	store, err := d.executionStore(request.ShardID, "GetReplicationDLQSize")
	if err != nil {
		return nil, err
	}
	return store.GetReplicationDLQSize(ctx, request)
}

func (d *shardedExecutionStore) DeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.DeleteReplicationTaskFromDLQRequest) error {
	store, err := d.executionStore(request.ShardID, "DeleteReplicationTaskFromDLQ")
	if err != nil {
		return err
	}
	return store.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (d *shardedExecutionStore) RangeDeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.RangeDeleteReplicationTaskFromDLQRequest) (*persistence.RangeDeleteReplicationTaskFromDLQResponse, error) {
	store, err := d.executionStore(request.ShardID, "RangeDeleteReplicationTaskFromDLQ")
	if err != nil {
		return nil, err
	}
	return store.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (d *shardedExecutionStore) CreateFailoverMarkerTasks(ctx context.Context, request *persistence.CreateFailoverMarkersRequest) error {
	store, err := d.executionStore(request.ShardID, "CreateFailoverMarkerTasks")
	if err != nil {
		return err
	}
	return store.CreateFailoverMarkerTasks(ctx, request)
}

func (d *shardedExecutionStore) GetHistoryTasks(ctx context.Context, request *persistence.GetHistoryTasksRequest) (*persistence.GetHistoryTasksResponse, error) {
	store, err := d.executionStore(request.ShardID, "GetHistoryTasks")
	if err != nil {
		return nil, err
	}
	return store.GetHistoryTasks(ctx, request)
}

func (d *shardedExecutionStore) CompleteHistoryTask(ctx context.Context, request *persistence.CompleteHistoryTaskRequest) error {
	store, err := d.executionStore(request.ShardID, "CompleteHistoryTask")
	if err != nil {
		return err
	}
	return store.CompleteHistoryTask(ctx, request)
}

func (d *shardedExecutionStore) RangeCompleteHistoryTask(ctx context.Context, request *persistence.RangeCompleteHistoryTaskRequest) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	store, err := d.executionStore(request.ShardID, "RangeCompleteHistoryTask")
	if err != nil {
		return nil, err
	}
	return store.RangeCompleteHistoryTask(ctx, request)
}

func (d *shardedExecutionStore) SelectWorkflowTimerTasks(ctx context.Context, request *persistence.SelectWorkflowTimerTasksRequest) ([]persistence.HistoryTaskKey, error) {
	store, err := d.executionStore(&request.ShardID, "SelectWorkflowTimerTasks")
	if err != nil {
		return nil, err
	}
	return store.SelectWorkflowTimerTasks(ctx, request)
}

func (d *shardedExecutionStore) ListConcreteExecutions(ctx context.Context, request *persistence.ListConcreteExecutionsRequest) (*persistence.InternalListConcreteExecutionsResponse, error) {
	store, err := d.executionStore(request.ShardID, "ListConcreteExecutions")
	if err != nil {
		return nil, err
	}
	return store.ListConcreteExecutions(ctx, request)
}

func (d *shardedExecutionStore) ListCurrentExecutions(ctx context.Context, request *persistence.ListCurrentExecutionsRequest) (*persistence.ListCurrentExecutionsResponse, error) {
	store, err := d.executionStore(request.ShardID, "ListCurrentExecutions")
	if err != nil {
		return nil, err
	}
	return store.ListCurrentExecutions(ctx, request)
}

func (d *shardedExecutionStore) GetActiveClusterSelectionPolicy(ctx context.Context, request *persistence.GetActiveClusterSelectionPolicyRequest) (*persistence.DataBlob, error) {
	store, err := d.executionStore(request.ShardID, "GetActiveClusterSelectionPolicy")
	if err != nil {
		return nil, err
	}
	return store.GetActiveClusterSelectionPolicy(ctx, request)
}

func (d *shardedExecutionStore) DeleteActiveClusterSelectionPolicy(ctx context.Context, request *persistence.DeleteActiveClusterSelectionPolicyRequest) error {
	store, err := d.executionStore(request.ShardID, "DeleteActiveClusterSelectionPolicy")
	if err != nil {
		return err
	}
	return store.DeleteActiveClusterSelectionPolicy(ctx, request)
}
