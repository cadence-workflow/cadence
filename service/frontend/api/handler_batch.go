package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

// batchOperationWorkflowID derives a deterministic workflow ID so retries
// with the same (domain, batchID) resolve to the same workflow.
func batchOperationWorkflowID(domainName, batchID string) string {
	return fmt.Sprintf("%s%s:%s", batchOperationWorkflowIDPrefix, domainName, batchID)
}

// batchOperationRetryPolicy mirrors the CLI's copyRetryPolicyFromWorkflow.
func batchOperationRetryPolicy() *types.RetryPolicy {
	return &types.RetryPolicy{
		InitialIntervalInSeconds:    int32(batcher.BatchActivityRetryPolicy.InitialInterval.Seconds()),
		BackoffCoefficient:          batcher.BatchActivityRetryPolicy.BackoffCoefficient,
		MaximumIntervalInSeconds:    int32(batcher.BatchActivityRetryPolicy.MaximumInterval.Seconds()),
		NonRetriableErrorReasons:    batcher.BatchActivityRetryPolicy.NonRetriableErrorReasons,
		ExpirationIntervalInSeconds: int32(batcher.BatchActivityRetryPolicy.ExpirationInterval.Seconds()),
	}
}

// batchOperationMemo matches tools/cli's getWorkflowMemo encoding.
func batchOperationMemo(reason string) (*types.Memo, error) {
	reasonBytes, err := json.Marshal(reason)
	if err != nil {
		return nil, err
	}
	return &types.Memo{Fields: map[string][]byte{"Reason": reasonBytes}}, nil
}

// batchOperationSearchAttributes sets CustomDomain/Operator, matching the
// CLI's start path, so batches started here show up in domain-scoped listing.
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
// consumes as JSON input, normalized so consumers like cadence-web that only
// read the legacy fields can still determine the operation.
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

// validateStartBatchOperationRequest also runs batcher.ValidateParams so a
// malformed request fails here instead of on the workflow's first decision.
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

// StartBatchOperation validates the request and starts (or, for a retried
// BatchID, reuses) the cadence-batcher workflow under the hood.
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
			// Idempotent retry: treat as success rather than an error.
			return &types.StartBatchOperationResponse{BatchID: batchID}, nil
		}
		return nil, err
	}

	return &types.StartBatchOperationResponse{BatchID: batchID}, nil
}

func validateDescribeBatchOperationRequest(request *types.DescribeBatchOperationRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if request.GetBatchID() == "" {
		return validate.ErrBatchIDNotSet
	}
	return nil
}

func batchOperationStateFromCloseStatus(closeStatus *types.WorkflowExecutionCloseStatus) types.BatchOperationState {
	if closeStatus == nil {
		return types.BatchOperationStateRunning
	}
	switch *closeStatus {
	case types.WorkflowExecutionCloseStatusCompleted, types.WorkflowExecutionCloseStatusContinuedAsNew:
		return types.BatchOperationStateCompleted
	case types.WorkflowExecutionCloseStatusFailed, types.WorkflowExecutionCloseStatusTimedOut:
		return types.BatchOperationStateFailed
	case types.WorkflowExecutionCloseStatusCanceled, types.WorkflowExecutionCloseStatusTerminated:
		return types.BatchOperationStateStopped
	default:
		return types.BatchOperationStateInvalid
	}
}

func heartBeatDetailsFromPendingActivities(pendingActivities []*types.PendingActivityInfo) *batcher.HeartBeatDetails {
	if len(pendingActivities) == 0 {
		return nil
	}
	var hbd batcher.HeartBeatDetails
	if err := json.Unmarshal(pendingActivities[0].HeartbeatDetails, &hbd); err != nil {
		return nil
	}
	return &hbd
}

func batchOperationProgressFromHeartBeatDetails(hbd *batcher.HeartBeatDetails) *types.BatchOperationProgress {
	if hbd == nil {
		return nil
	}
	return &types.BatchOperationProgress{
		TotalEstimate: hbd.TotalEstimate,
		SuccessCount:  int64(hbd.SuccessCount),
		ErrorCount:    int64(hbd.ErrorCount),
	}
}

func batchOperationLimitsFromHeartBeatDetails(hbd *batcher.HeartBeatDetails) *types.BatchOperationLimits {
	if hbd == nil {
		return nil
	}
	return &types.BatchOperationLimits{
		RPS:         int32(hbd.RPS),
		Concurrency: int32(hbd.Concurrency),
	}
}

func timePtrFromNanos(nanos *int64) *time.Time {
	if nanos == nil {
		return nil
	}
	return common.TimePtr(time.Unix(0, *nanos))
}

// DescribeBatchOperation returns the status and progress of a single batch.
func (wh *WorkflowHandler) DescribeBatchOperation(
	ctx context.Context,
	request *types.DescribeBatchOperationRequest,
) (*types.DescribeBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := validateDescribeBatchOperationRequest(request); err != nil {
		return nil, err
	}

	domainName := request.GetDomain()
	if _, err := wh.GetDomainCache().GetDomainID(domainName); err != nil {
		return nil, err
	}

	wfID := batchOperationWorkflowID(domainName, request.GetBatchID())
	resp, err := wh.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
		Domain:    constants.BatcherLocalDomainName,
		Execution: &types.WorkflowExecution{WorkflowID: wfID},
	})
	if err != nil {
		return nil, err
	}

	info := resp.GetWorkflowExecutionInfo()

	var reason string
	if info.Memo != nil {
		_ = json.Unmarshal(info.Memo.Fields["Reason"], &reason)
	}
	var startedBy string
	if info.SearchAttributes != nil {
		_ = json.Unmarshal(info.SearchAttributes.IndexedFields["Operator"], &startedBy)
	}

	hbd := heartBeatDetailsFromPendingActivities(resp.PendingActivities)

	return &types.DescribeBatchOperationResponse{
		Reason:    reason,
		State:     batchOperationStateFromCloseStatus(info.CloseStatus),
		Progress:  batchOperationProgressFromHeartBeatDetails(hbd),
		Limits:    batchOperationLimitsFromHeartBeatDetails(hbd),
		StartedBy: startedBy,
		StartTime: timePtrFromNanos(info.StartTime),
		CloseTime: timePtrFromNanos(info.CloseTime),
	}, nil
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
