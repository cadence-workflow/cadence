package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
	"github.com/uber/cadence/service/worker/batcher"
)

const (
	batchOperationWorkflowIDPrefix = "cadence-sys-batch-operation:"
	batchOperationDecisionTimeout  = 10 // seconds
)

// batchOperationWorkflowID derives a deterministic workflow ID from the
// target domain and the caller-provided batch ID. Determinism is what makes
// BatchID-based idempotency possible: a retry with the same (domain, batchID)
// resolves to the same workflow instead of starting a duplicate.
func batchOperationWorkflowID(domainName, batchID string) string {
	return fmt.Sprintf("%s%s:%s", batchOperationWorkflowIDPrefix, domainName, batchID)
}

// batchOperationRetryPolicy mirrors the CLI's copyRetryPolicyFromWorkflow:
// the workflow-level retry policy is derived from the same activity retry
// policy the batch activity itself uses.
func batchOperationRetryPolicy() *types.RetryPolicy {
	return &types.RetryPolicy{
		InitialIntervalInSeconds:    int32(batcher.BatchActivityRetryPolicy.InitialInterval.Seconds()),
		BackoffCoefficient:          batcher.BatchActivityRetryPolicy.BackoffCoefficient,
		MaximumIntervalInSeconds:    int32(batcher.BatchActivityRetryPolicy.MaximumInterval.Seconds()),
		NonRetriableErrorReasons:    batcher.BatchActivityRetryPolicy.NonRetriableErrorReasons,
		ExpirationIntervalInSeconds: int32(batcher.BatchActivityRetryPolicy.ExpirationInterval.Seconds()),
	}
}

// batchOperationMemo and batchOperationSearchAttributes JSON-encode each
// field's value, matching the encoding tools/cli's getWorkflowMemo and
// serializeSearchAttributes use, so API-started and CLI-started batches
// store these fields in the same format.
func batchOperationMemo(reason string) (*types.Memo, error) {
	reasonBytes, err := json.Marshal(reason)
	if err != nil {
		return nil, err
	}
	return &types.Memo{Fields: map[string][]byte{"Reason": reasonBytes}}, nil
}

// batchOperationSearchAttributes sets CustomDomain and Operator, matching
// the CLI's start path. ListBatchJobs and domain-scoped batch listing query
// on CustomDomain, so without it batches started via this API would never
// appear in those results.
func batchOperationSearchAttributes(domainName, operator string) (*types.SearchAttributes, error) {
	domainBytes, err := json.Marshal(domainName)
	if err != nil {
		return nil, err
	}
	operatorBytes, err := json.Marshal(operator)
	if err != nil {
		return nil, err
	}
	return &types.SearchAttributes{IndexedFields: map[string][]byte{
		"CustomDomain": domainBytes,
		"Operator":     operatorBytes,
	}}, nil
}

// toBatchParams builds the batcher.BatchParams blob the batch workflow
// consumes as JSON input. BatchType and the matching legacy Xxx Params field
// are filled in via batcher.NormalizeOperation so that any consumer of the
// stored workflow input (e.g. cadence-web, which only reads the legacy
// fields) can still determine the operation without understanding Operation.
func toBatchParams(request *types.StartBatchOperationRequest) batcher.BatchParams {
	params := batcher.BatchParams{
		DomainName: request.GetDomain(),
		Query:      request.GetQuery(),
		Reason:     request.GetReason(),
		Operation:  request.GetOperation(),
	}
	if limits := request.GetLimits(); limits != nil {
		params.RPS = int(limits.GetRPS())
		params.Concurrency = int(limits.GetConcurrency())
		params.PageSize = int(limits.GetPageSize())
	}
	return batcher.NormalizeOperation(params)
}

// validateStartBatchOperationRequest checks the request shape, then runs the
// normalized params through the same validation the workflow itself applies
// (batcher.ValidateParams), so a malformed request (missing reason, a signal
// with no name, a replicate with no clusters, etc.) is rejected here instead
// of starting a workflow that immediately fails on its first decision.
func validateStartBatchOperationRequest(request *types.StartBatchOperationRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if request.GetBatchID() == "" {
		return validate.ErrBatchIDNotSet
	}
	if request.GetQuery() == "" {
		return validate.ErrQueryNotSet
	}
	if request.GetOperation() == nil {
		return validate.ErrBatchOperationNotSet
	}
	if batcher.BatchTypeFromOperation(request.GetOperation()) == "" {
		return validate.ErrBatchOperationTypeNotSet
	}
	if err := batcher.ValidateParams(toBatchParams(request)); err != nil {
		return &types.BadRequestError{Message: err.Error()}
	}
	return nil
}

// StartBatchOperation validates the request, translates it into the batch
// workflow's existing input shape, and starts (or, for a retried BatchID,
// reuses) the cadence-batcher workflow under the hood.
func (wh *WorkflowHandler) StartBatchOperation(
	ctx context.Context,
	request *types.StartBatchOperationRequest,
) (*types.StartBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := validateStartBatchOperationRequest(request); err != nil {
		return nil, err
	}

	domainName := request.GetDomain()
	if _, err := wh.GetDomainCache().GetDomainID(domainName); err != nil {
		return nil, err
	}

	batchParams := toBatchParams(request)

	inputBytes, err := json.Marshal(batchParams)
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to serialize batch workflow input: %v", err)}
	}

	memo, err := batchOperationMemo(request.GetReason())
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to encode batch job memo: %v", err)}
	}
	searchAttributes, err := batchOperationSearchAttributes(domainName, request.GetIdentity())
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to encode batch job search attributes: %v", err)}
	}

	batchID := request.GetBatchID()
	wfID := batchOperationWorkflowID(domainName, batchID)
	executionTimeout := common.Int32Ptr(int32(batcher.InfiniteDuration.Seconds()))
	decisionTimeout := common.Int32Ptr(batchOperationDecisionTimeout)

	_, err = wh.StartWorkflowExecution(ctx, &types.StartWorkflowExecutionRequest{
		Domain:                              constants.BatcherLocalDomainName,
		RequestID:                           uuid.New().String(),
		WorkflowID:                          wfID,
		WorkflowType:                        &types.WorkflowType{Name: batcher.BatchWFV2TypeName},
		TaskList:                            &types.TaskList{Name: batcher.BatcherTaskListName},
		Input:                               inputBytes,
		ExecutionStartToCloseTimeoutSeconds: executionTimeout,
		TaskStartToCloseTimeoutSeconds:      decisionTimeout,
		Memo:                                memo,
		SearchAttributes:                    searchAttributes,
		RetryPolicy:                         batchOperationRetryPolicy(),
	})
	if err != nil {
		var alreadyStarted *types.WorkflowExecutionAlreadyStartedError
		if errors.As(err, &alreadyStarted) {
			// BatchID is scoped to the domain for idempotency: a retry with the same
			// (domain, batchID) resolves to the already-started batch rather than erroring.
			return &types.StartBatchOperationResponse{BatchID: batchID}, nil
		}
		return nil, err
	}

	return &types.StartBatchOperationResponse{BatchID: batchID}, nil
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
