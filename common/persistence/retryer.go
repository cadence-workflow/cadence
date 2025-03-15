// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination retryer_mock.go -package persistence github.com/uber/cadence/common/persistence Retryer

package persistence

import (
	"context"

	"github.com/uber/cadence/common/backoff"
)

// Retryer is used to retry requests to persistence with provided retry policy
type Retryer interface {
	ListConcreteExecutions(context.Context, *ListConcreteExecutionsRequest) (*ListConcreteExecutionsResponse, error)
	ListCurrentExecutions(context.Context, *ListCurrentExecutionsRequest) (*ListCurrentExecutionsResponse, error)
	GetWorkflowExecution(context.Context, *GetWorkflowExecutionRequest) (*GetWorkflowExecutionResponse, error)
	GetCurrentExecution(context.Context, *GetCurrentExecutionRequest) (*GetCurrentExecutionResponse, error)
	IsWorkflowExecutionExists(context.Context, *IsWorkflowExecutionExistsRequest) (*IsWorkflowExecutionExistsResponse, error)
	ReadHistoryBranch(context.Context, *ReadHistoryBranchRequest) (*ReadHistoryBranchResponse, error)
	DeleteWorkflowExecution(context.Context, *DeleteWorkflowExecutionRequest) error
	DeleteCurrentWorkflowExecution(context.Context, *DeleteCurrentWorkflowExecutionRequest) error
	GetShardID() int
	GetHistoryTasks(context.Context, *GetHistoryTasksRequest) (*GetHistoryTasksResponse, error)
	CompleteTimerTask(ctx context.Context, request *CompleteTimerTaskRequest) error
}

type (
	persistenceRetryer struct {
		execManager    ExecutionManager
		historyManager HistoryManager
		throttleRetry  *backoff.ThrottleRetry
	}
)

// NewPersistenceRetryer constructs a new Retryer
func NewPersistenceRetryer(
	execManager ExecutionManager,
	historyManager HistoryManager,
	policy backoff.RetryPolicy,
) Retryer {
	return &persistenceRetryer{
		execManager:    execManager,
		historyManager: historyManager,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(policy),
			backoff.WithRetryableError(IsTransientError),
		),
	}
}

// ListConcreteExecutions retries ListConcreteExecutions
func (pr *persistenceRetryer) ListConcreteExecutions(
	ctx context.Context,
	req *ListConcreteExecutionsRequest,
) (*ListConcreteExecutionsResponse, error) {
	var resp *ListConcreteExecutionsResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.ListConcreteExecutions(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err == nil {
		return resp, nil
	}
	return nil, err
}

// GetWorkflowExecution retries GetWorkflowExecution
func (pr *persistenceRetryer) GetWorkflowExecution(
	ctx context.Context,
	req *GetWorkflowExecutionRequest,
) (*GetWorkflowExecutionResponse, error) {
	var resp *GetWorkflowExecutionResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.GetWorkflowExecution(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetCurrentExecution retries GetCurrentExecution
func (pr *persistenceRetryer) GetCurrentExecution(
	ctx context.Context,
	req *GetCurrentExecutionRequest,
) (*GetCurrentExecutionResponse, error) {
	var resp *GetCurrentExecutionResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.GetCurrentExecution(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ListCurrentExecutions retries ListCurrentExecutions
func (pr *persistenceRetryer) ListCurrentExecutions(
	ctx context.Context,
	req *ListCurrentExecutionsRequest,
) (*ListCurrentExecutionsResponse, error) {
	var resp *ListCurrentExecutionsResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.ListCurrentExecutions(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err == nil {
		return resp, nil
	}
	return nil, err
}

// IsWorkflowExecutionExists retries IsWorkflowExecutionExists
func (pr *persistenceRetryer) IsWorkflowExecutionExists(
	ctx context.Context,
	req *IsWorkflowExecutionExistsRequest,
) (*IsWorkflowExecutionExistsResponse, error) {
	var resp *IsWorkflowExecutionExistsResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.IsWorkflowExecutionExists(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ReadHistoryBranch retries ReadHistoryBranch
func (pr *persistenceRetryer) ReadHistoryBranch(
	ctx context.Context,
	req *ReadHistoryBranchRequest,
) (*ReadHistoryBranchResponse, error) {
	var resp *ReadHistoryBranchResponse
	op := func() error {
		var err error
		resp, err = pr.historyManager.ReadHistoryBranch(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteWorkflowExecution retries DeleteWorkflowExecution
func (pr *persistenceRetryer) DeleteWorkflowExecution(
	ctx context.Context,
	req *DeleteWorkflowExecutionRequest,
) error {
	op := func() error {
		return pr.execManager.DeleteWorkflowExecution(ctx, req)
	}
	return pr.throttleRetry.Do(ctx, op)
}

// DeleteCurrentWorkflowExecution retries DeleteCurrentWorkflowExecution
func (pr *persistenceRetryer) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	req *DeleteCurrentWorkflowExecutionRequest,
) error {
	op := func() error {
		return pr.execManager.DeleteCurrentWorkflowExecution(ctx, req)
	}
	return pr.throttleRetry.Do(ctx, op)
}

// GetShardID return shard id
func (pr *persistenceRetryer) GetShardID() int {
	return pr.execManager.GetShardID()
}

// GetHistoryTasks retries GetHistoryTasks
func (pr *persistenceRetryer) GetHistoryTasks(
	ctx context.Context,
	req *GetHistoryTasksRequest,
) (*GetHistoryTasksResponse, error) {
	var resp *GetHistoryTasksResponse
	op := func() error {
		var err error
		resp, err = pr.execManager.GetHistoryTasks(ctx, req)
		return err
	}
	err := pr.throttleRetry.Do(ctx, op)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// CompleteTimerTask is a retryable version of CompleteTimerTask method
func (pr *persistenceRetryer) CompleteTimerTask(
	ctx context.Context,
	request *CompleteTimerTaskRequest,
) error {
	op := func() error {
		return pr.execManager.CompleteTimerTask(ctx, request)
	}

	return pr.throttleRetry.Do(ctx, op)
}
