package api

import (
	"context"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

// CreateSemaphore creates a semaphore and replicates it for a global domain.
func (wh *WorkflowHandler) CreateSemaphore(
	ctx context.Context,
	request *types.CreateSemaphoreRequest,
) (*types.CreateSemaphoreResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := wh.requestValidator.ValidateCreateSemaphoreRequest(ctx, request); err != nil {
		return nil, err
	}
	return wh.semaphoreHandler.CreateSemaphore(ctx, request)
}
