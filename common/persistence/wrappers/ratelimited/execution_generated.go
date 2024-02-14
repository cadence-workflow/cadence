// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ratelimited

// Code generated by gowrap. DO NOT EDIT.
// template: ../templates/ratelimited.tmpl
// gowrap: http://github.com/hexdigest/gowrap

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
)

// ratelimitedExecutionManager implements persistence.ExecutionManager interface instrumented with rate limiter.
type ratelimitedExecutionManager struct {
	wrapped     persistence.ExecutionManager
	rateLimiter quotas.Limiter
}

// NewExecutionManager creates a new instance of ExecutionManager with ratelimiter.
func NewExecutionManager(
	wrapped persistence.ExecutionManager,
	rateLimiter quotas.Limiter,
) persistence.ExecutionManager {
	return &ratelimitedExecutionManager{
		wrapped:     wrapped,
		rateLimiter: rateLimiter,
	}
}

func (c *ratelimitedExecutionManager) Close() {
	c.wrapped.Close()
	return
}

func (c *ratelimitedExecutionManager) CompleteCrossClusterTask(ctx context.Context, request *persistence.CompleteCrossClusterTaskRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CompleteCrossClusterTask(ctx, request)
}

func (c *ratelimitedExecutionManager) CompleteReplicationTask(ctx context.Context, request *persistence.CompleteReplicationTaskRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CompleteReplicationTask(ctx, request)
}

func (c *ratelimitedExecutionManager) CompleteTimerTask(ctx context.Context, request *persistence.CompleteTimerTaskRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CompleteTimerTask(ctx, request)
}

func (c *ratelimitedExecutionManager) CompleteTransferTask(ctx context.Context, request *persistence.CompleteTransferTaskRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CompleteTransferTask(ctx, request)
}

func (c *ratelimitedExecutionManager) ConflictResolveWorkflowExecution(ctx context.Context, request *persistence.ConflictResolveWorkflowExecutionRequest) (cp1 *persistence.ConflictResolveWorkflowExecutionResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.ConflictResolveWorkflowExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) CreateFailoverMarkerTasks(ctx context.Context, request *persistence.CreateFailoverMarkersRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CreateFailoverMarkerTasks(ctx, request)
}

func (c *ratelimitedExecutionManager) CreateWorkflowExecution(ctx context.Context, request *persistence.CreateWorkflowExecutionRequest) (cp1 *persistence.CreateWorkflowExecutionResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.CreateWorkflowExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) DeleteCurrentWorkflowExecution(ctx context.Context, request *persistence.DeleteCurrentWorkflowExecutionRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.DeleteCurrentWorkflowExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) DeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.DeleteReplicationTaskFromDLQRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (c *ratelimitedExecutionManager) DeleteWorkflowExecution(ctx context.Context, request *persistence.DeleteWorkflowExecutionRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.DeleteWorkflowExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) GetCrossClusterTasks(ctx context.Context, request *persistence.GetCrossClusterTasksRequest) (gp1 *persistence.GetCrossClusterTasksResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetCrossClusterTasks(ctx, request)
}

func (c *ratelimitedExecutionManager) GetCurrentExecution(ctx context.Context, request *persistence.GetCurrentExecutionRequest) (gp1 *persistence.GetCurrentExecutionResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetCurrentExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) GetName() (s1 string) {
	return c.wrapped.GetName()
}

func (c *ratelimitedExecutionManager) GetReplicationDLQSize(ctx context.Context, request *persistence.GetReplicationDLQSizeRequest) (gp1 *persistence.GetReplicationDLQSizeResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetReplicationDLQSize(ctx, request)
}

func (c *ratelimitedExecutionManager) GetReplicationTasks(ctx context.Context, request *persistence.GetReplicationTasksRequest) (gp1 *persistence.GetReplicationTasksResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetReplicationTasks(ctx, request)
}

func (c *ratelimitedExecutionManager) GetReplicationTasksFromDLQ(ctx context.Context, request *persistence.GetReplicationTasksFromDLQRequest) (gp1 *persistence.GetReplicationTasksFromDLQResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetReplicationTasksFromDLQ(ctx, request)
}

func (c *ratelimitedExecutionManager) GetShardID() (i1 int) {
	return c.wrapped.GetShardID()
}

func (c *ratelimitedExecutionManager) GetTimerIndexTasks(ctx context.Context, request *persistence.GetTimerIndexTasksRequest) (gp1 *persistence.GetTimerIndexTasksResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetTimerIndexTasks(ctx, request)
}

func (c *ratelimitedExecutionManager) GetTransferTasks(ctx context.Context, request *persistence.GetTransferTasksRequest) (gp1 *persistence.GetTransferTasksResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetTransferTasks(ctx, request)
}

func (c *ratelimitedExecutionManager) GetWorkflowExecution(ctx context.Context, request *persistence.GetWorkflowExecutionRequest) (gp1 *persistence.GetWorkflowExecutionResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.GetWorkflowExecution(ctx, request)
}

func (c *ratelimitedExecutionManager) IsWorkflowExecutionExists(ctx context.Context, request *persistence.IsWorkflowExecutionExistsRequest) (ip1 *persistence.IsWorkflowExecutionExistsResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.IsWorkflowExecutionExists(ctx, request)
}

func (c *ratelimitedExecutionManager) ListConcreteExecutions(ctx context.Context, request *persistence.ListConcreteExecutionsRequest) (lp1 *persistence.ListConcreteExecutionsResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.ListConcreteExecutions(ctx, request)
}

func (c *ratelimitedExecutionManager) ListCurrentExecutions(ctx context.Context, request *persistence.ListCurrentExecutionsRequest) (lp1 *persistence.ListCurrentExecutionsResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.ListCurrentExecutions(ctx, request)
}

func (c *ratelimitedExecutionManager) PutReplicationTaskToDLQ(ctx context.Context, request *persistence.PutReplicationTaskToDLQRequest) (err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.PutReplicationTaskToDLQ(ctx, request)
}

func (c *ratelimitedExecutionManager) RangeCompleteCrossClusterTask(ctx context.Context, request *persistence.RangeCompleteCrossClusterTaskRequest) (rp1 *persistence.RangeCompleteCrossClusterTaskResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.RangeCompleteCrossClusterTask(ctx, request)
}

func (c *ratelimitedExecutionManager) RangeCompleteReplicationTask(ctx context.Context, request *persistence.RangeCompleteReplicationTaskRequest) (rp1 *persistence.RangeCompleteReplicationTaskResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.RangeCompleteReplicationTask(ctx, request)
}

func (c *ratelimitedExecutionManager) RangeCompleteTimerTask(ctx context.Context, request *persistence.RangeCompleteTimerTaskRequest) (rp1 *persistence.RangeCompleteTimerTaskResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.RangeCompleteTimerTask(ctx, request)
}

func (c *ratelimitedExecutionManager) RangeCompleteTransferTask(ctx context.Context, request *persistence.RangeCompleteTransferTaskRequest) (rp1 *persistence.RangeCompleteTransferTaskResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.RangeCompleteTransferTask(ctx, request)
}

func (c *ratelimitedExecutionManager) RangeDeleteReplicationTaskFromDLQ(ctx context.Context, request *persistence.RangeDeleteReplicationTaskFromDLQRequest) (rp1 *persistence.RangeDeleteReplicationTaskFromDLQResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (c *ratelimitedExecutionManager) UpdateWorkflowExecution(ctx context.Context, request *persistence.UpdateWorkflowExecutionRequest) (up1 *persistence.UpdateWorkflowExecutionResponse, err error) {
	if ok := c.rateLimiter.Allow(); !ok {
		err = ErrPersistenceLimitExceeded
		return
	}
	return c.wrapped.UpdateWorkflowExecution(ctx, request)
}
