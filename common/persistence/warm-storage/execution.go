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

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

func NewWarmStorageExecutionManager(
	log log.Logger,
	metrics metrics.Client,
	mgr persistence.ExecutionManager,
	arc archiver.ExecutionArchiver,
) (WarmStorageExecutionManager, error) {

	return &executionWarmStorageImpl{
		ExecutionManager: mgr,
		log:              log,
		metrics:          metrics,
	}, nil
}

type executionWarmStorageImpl struct {
	persistence.ExecutionManager

	dc      dynamicconfig.Client
	log     log.Logger
	metrics metrics.Client
	//archiveCfg archiver.ArchivalConfig
	archiver archiver.ExecutionArchiver
}

var _ persistence.ExecutionManager = &executionWarmStorageImpl{}

func (w *executionWarmStorageImpl) GetWorkflowExecution(
	ctx context.Context,
	request *persistence.GetWorkflowExecutionRequest,
) (*persistence.GetWorkflowExecutionResponse, error) {
	// 'happy-path', the workflow's still running and it's found
	// in the database. Return it immediately
	res, lookupErr := w.ExecutionManager.GetWorkflowExecution(ctx, request)
	if lookupErr == nil {
		res.StorageLocation = persistence.StorageLocationDatabase
		return res, nil
	}

	// if the lookup *is not* a not-found error, handle that by
	// returning it unchanged
	var e *types.EntityNotExistsError
	if !errors.As(lookupErr, &e) {
		return nil, lookupErr
	}

	// the error is a not-found error. Look for it in warm storage
	req := archiver.GetExecutionRequest{
		DomainID:   request.DomainID,
		WorkflowID: request.Execution.GetWorkflowID(),
		RunID:      request.Execution.GetRunID(),
	}
	archiverRes, err := w.archiver.GetWorkflowExecutionForPersistence(ctx, &req)
	if err != nil {
		return nil, err
	}
	return &persistence.GetWorkflowExecutionResponse{
		State:           archiverRes.MutableState,
		StorageLocation: persistence.StorageLocationWarmStorage,
	}, nil
}

// Todo (david.porter) considering adding support for current records
//func (w *executionWarmStorageImpl) GetCurrentExecution(
//	ctx context.Context,
//	request *persistence.GetCurrentExecutionRequest,
//) (*persistence.GetCurrentExecutionResponse, error) {
//	res, err := w.ExecutionManager.GetCurrentExecution(ctx, request)
//	if err == nil {
//		return res, nil
//	}
//	var e *types.EntityNotExistsError
//	if errors.As(err, &e) {
//		return w.warmStorage.GetCurrentExecution(ctx, request)
//	}
//	return nil, err
//}

func (w *executionWarmStorageImpl) IsWorkflowExecutionExists(
	ctx context.Context,
	request *persistence.IsWorkflowExecutionExistsRequest,
) (*persistence.IsWorkflowExecutionExistsResponse, error) {
	res, lookupErr := w.ExecutionManager.IsWorkflowExecutionExists(ctx, request)
	if lookupErr == nil {
		return res, nil
	}

	// if the error returned is *not* a not-found error, then return it
	// since that's not something this middleware can handle
	var e *types.EntityNotExistsError
	if !errors.As(lookupErr, &e) {
		return res, lookupErr
	}

	req := archiver.GetExecutionRequest{
		DomainID:   request.DomainID,
		WorkflowID: request.WorkflowID,
		RunID:      request.RunID,
	}

	archiverRes, err := w.archiver.GetWorkflowExecutionForPersistence(ctx, &req)
	if err == nil && archiverRes.MutableState != nil {
		return &persistence.IsWorkflowExecutionExistsResponse{Exists: true}, nil
	}

	var e2 *types.EntityNotExistsError
	if errors.As(err, &e2) {
		return &persistence.IsWorkflowExecutionExistsResponse{Exists: false}, nil
	}
	if err != nil {
		w.log.Error("lookup error while trying to check if workflow exists",
			tag.Error(err),
			tag.WorkflowID(req.WorkflowID),
			tag.WorkflowDomainID(req.DomainID),
			tag.WorkflowRunID(req.RunID),
		)
		return &persistence.IsWorkflowExecutionExistsResponse{Exists: false}, nil
	}
	w.log.Error("programmatic error: got neither error nor expected response from warm storage",
		tag.WorkflowID(req.WorkflowID),
		tag.WorkflowDomainID(req.DomainID),
		tag.WorkflowRunID(req.RunID),
	)
	panic("Unexpected state found while trying to look up workflow")
}
