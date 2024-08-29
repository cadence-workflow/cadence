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

package warm_storage

import (
	"context"
	"errors"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type WarmStorageManager interface {
	GetWorkflowExecution(ctx context.Context, req *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error)
	GetCurrentExecution(ctx context.Context, req *persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error)
	IsWorkflowExecutionExists(ctx context.Context, req *persistence.IsWorkflowExecutionExistsRequest) (*persistence.IsWorkflowExecutionExistsResponse, error)
	GetWorkflowExecutionHistory(ctx context.Context, request persistence.GetAllHistoryTreeBranchesRequest) (*persistence.InternalGetWorkflowExecutionResponse, error)
}

type ReadWarmStorage interface {
	GetCurrentExecution(ctx persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error)
	GetWorkflowExecution(ctx context.Context, request persistence.GetWorkflowExecutionRequest) (*persistence.InternalGetWorkflowExecutionResponse, error)
	IsWorkflowExecutionExists(ctx context.Context, req *persistence.IsWorkflowExecutionExistsRequest) (*persistence.IsWorkflowExecutionExistsResponse, error)
	GetWorkflowExecutionHistory(ctx context.Context, request persistence.GetAllHistoryTreeBranchesRequest) (*persistence.InternalGetWorkflowExecutionResponse, error)
}

type Params struct {
	log            log.Logger
	metrics        metrics.Client
	executionMgr   persistence.ExecutionManager
	warmStorageMgr WarmStorageManager
}

func NewWarmStorageManager(log log.Logger, metrics metrics.Client) (WarmStorageManager, error) {
	return nil, nil
}

func NewWarmStorageLayer(params Params) (persistence.ExecutionManager, error) {
	warmStorage := params.warmStorageMgr
	if params.warmStorageMgr == nil {
		w, err := NewWarmStorageManager(params.log, params.metrics)
		if err != nil {
			return nil, err
		}
		warmStorage = w
	}

	return &executionWarmStorageImpl{
		ExecutionManager: params.executionMgr,
		warmStorage:      warmStorage,
	}, nil
}

type executionWarmStorageImpl struct {
	persistence.ExecutionManager

	warmStorage WarmStorageManager
}

var _ persistence.ExecutionManager = &executionWarmStorageImpl{}

func (w *executionWarmStorageImpl) GetWorkflowExecution(
	ctx context.Context,
	request *persistence.GetWorkflowExecutionRequest,
) (*persistence.GetWorkflowExecutionResponse, error) {
	res, err := w.ExecutionManager.GetWorkflowExecution(ctx, request)
	if err == nil {
		return res, nil
	}
	var e *types.EntityNotExistsError
	if errors.As(err, &e) {
		return w.warmStorage.GetWorkflowExecution(ctx, request)
	}
	return nil, err
}

func (w *executionWarmStorageImpl) GetCurrentExecution(
	ctx context.Context,
	request *persistence.GetCurrentExecutionRequest,
) (*persistence.GetCurrentExecutionResponse, error) {
	res, err := w.ExecutionManager.GetCurrentExecution(ctx, request)
	if err == nil {
		return res, nil
	}
	var e *types.EntityNotExistsError
	if errors.As(err, &e) {
		return w.warmStorage.GetCurrentExecution(ctx, request)
	}
	return nil, err
}

func (w *executionWarmStorageImpl) IsWorkflowExecutionExists(
	ctx context.Context,
	request *persistence.IsWorkflowExecutionExistsRequest,
) (*persistence.IsWorkflowExecutionExistsResponse, error) {
	res, err := w.ExecutionManager.IsWorkflowExecutionExists(ctx, request)
	if err == nil {
		return res, nil
	}
	var e *types.EntityNotExistsError
	if errors.As(err, &e) {
		return w.warmStorage.IsWorkflowExecutionExists(ctx, request)
	}
	return nil, err
}
