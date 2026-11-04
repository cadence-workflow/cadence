package api

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// StartBatchOperation starts a batch. Implementation tracked separately; this
// is a placeholder so the BatchAPI contract and its authorization wrapper can
// land ahead of the per-operation business logic.
func (wh *WorkflowHandler) StartBatchOperation(
	ctx context.Context,
	request *types.StartBatchOperationRequest,
) (*types.StartBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	return nil, &types.BadRequestError{Message: "StartBatchOperation is not yet implemented."}
}

// DescribeBatchOperation returns the details and progress of a single batch.
// Implementation tracked separately; this is a placeholder so the BatchAPI
// contract and its authorization wrapper can land ahead of the per-operation
// business logic.
func (wh *WorkflowHandler) DescribeBatchOperation(
	ctx context.Context,
	request *types.DescribeBatchOperationRequest,
) (*types.DescribeBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	return nil, &types.BadRequestError{Message: "DescribeBatchOperation is not yet implemented."}
}

// ListBatchOperations returns the batches targeting a domain. Implementation
// tracked separately; this is a placeholder so the BatchAPI contract and its
// authorization wrapper can land ahead of the per-operation business logic.
func (wh *WorkflowHandler) ListBatchOperations(
	ctx context.Context,
	request *types.ListBatchOperationsRequest,
) (*types.ListBatchOperationsResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	return nil, &types.BadRequestError{Message: "ListBatchOperations is not yet implemented."}
}

// StopBatchOperation stops a running batch. Implementation tracked
// separately; this is a placeholder so the BatchAPI contract and its
// authorization wrapper can land ahead of the per-operation business logic.
func (wh *WorkflowHandler) StopBatchOperation(
	ctx context.Context,
	request *types.StopBatchOperationRequest,
) (*types.StopBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	return nil, &types.BadRequestError{Message: "StopBatchOperation is not yet implemented."}
}

// UpdateBatchOperation changes the rate and concurrency of a running batch.
// Implementation tracked separately; this is a placeholder so the BatchAPI
// contract and its authorization wrapper can land ahead of the per-operation
// business logic.
func (wh *WorkflowHandler) UpdateBatchOperation(
	ctx context.Context,
	request *types.UpdateBatchOperationRequest,
) (*types.UpdateBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	return nil, &types.BadRequestError{Message: "UpdateBatchOperation is not yet implemented."}
}
