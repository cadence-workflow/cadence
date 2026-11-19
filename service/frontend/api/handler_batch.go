package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

// batchOperationMemo encodes Reason the same way tools/cli's getWorkflowMemo
// does, plus Query and Operation so DescribeBatchOperation can report them
// back; the workflow's own start input isn't readable from DescribeWorkflowExecution.
func batchOperationMemo(reason, query string, operation *types.BatchOperation) (*types.Memo, error) {
	reasonBytes, err := json.Marshal(reason)
	if err != nil {
		return nil, err
	}
	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	operationBytes, err := json.Marshal(operation)
	if err != nil {
		return nil, err
	}
	return &types.Memo{Fields: map[string][]byte{
		"Reason":    reasonBytes,
		"Query":     queryBytes,
		"Operation": operationBytes,
	}}, nil
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

	memo, err := batchOperationMemo(request.GetReason(), request.GetQuery(), request.GetOperation())
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

	var reason, query string
	var operation *types.BatchOperation
	if info.Memo != nil {
		_ = json.Unmarshal(info.Memo.Fields["Reason"], &reason)
		_ = json.Unmarshal(info.Memo.Fields["Query"], &query)
		if raw, ok := info.Memo.Fields["Operation"]; ok {
			var decoded types.BatchOperation
			if json.Unmarshal(raw, &decoded) == nil {
				operation = &decoded
			}
		}
	}
	var startedBy string
	if info.SearchAttributes != nil {
		_ = json.Unmarshal(info.SearchAttributes.IndexedFields["Operator"], &startedBy)
	}

	hbd := heartBeatDetailsFromPendingActivities(resp.PendingActivities)

	return &types.DescribeBatchOperationResponse{
		Operation: operation,
		Query:     query,
		Reason:    reason,
		State:     batchOperationStateFromCloseStatus(info.CloseStatus),
		Progress:  batchOperationProgressFromHeartBeatDetails(hbd),
		Limits:    batchOperationLimitsFromHeartBeatDetails(hbd),
		StartedBy: startedBy,
		StartTime: timePtrFromNanos(info.StartTime),
		CloseTime: timePtrFromNanos(info.CloseTime),
	}, nil
}

func validateListBatchOperationsRequest(request *types.ListBatchOperationsRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	switch request.GetState() {
	case types.BatchOperationStateInvalid, types.BatchOperationStateRunning,
		types.BatchOperationStateCompleted, types.BatchOperationStateFailed,
		types.BatchOperationStateStopped:
		return nil
	default:
		return validate.ErrInvalidBatchOperationState
	}
}

// batchOperationStateQueryClause mirrors batchOperationStateFromCloseStatus,
// except it excludes ContinuedAsNew, which the caller always discards.
func batchOperationStateQueryClause(state types.BatchOperationState) string {
	switch state {
	case types.BatchOperationStateRunning:
		return " and CloseTime = missing"
	case types.BatchOperationStateCompleted:
		return fmt.Sprintf(" and CloseStatus = %d", types.WorkflowExecutionCloseStatusCompleted)
	case types.BatchOperationStateFailed:
		return fmt.Sprintf(" and (CloseStatus = %d or CloseStatus = %d)",
			types.WorkflowExecutionCloseStatusFailed, types.WorkflowExecutionCloseStatusTimedOut)
	case types.BatchOperationStateStopped:
		return fmt.Sprintf(" and (CloseStatus = %d or CloseStatus = %d)",
			types.WorkflowExecutionCloseStatusCanceled, types.WorkflowExecutionCloseStatusTerminated)
	default:
		return ""
	}
}

// ListBatchOperations lists the batches targeting a domain by querying the
// visibility records of the cadence-batcher workflows started for it.
func (wh *WorkflowHandler) ListBatchOperations(
	ctx context.Context,
	request *types.ListBatchOperationsRequest,
) (*types.ListBatchOperationsResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := validateListBatchOperationsRequest(request); err != nil {
		return nil, err
	}

	domainName := request.GetDomain()
	if _, err := wh.GetDomainCache().GetDomainID(domainName); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("CustomDomain = '%s'", domainName) + batchOperationStateQueryClause(request.GetState())

	resp, err := wh.ListWorkflowExecutions(ctx, &types.ListWorkflowExecutionsRequest{
		Domain:        constants.BatcherLocalDomainName,
		PageSize:      request.GetPageSize(),
		NextPageToken: request.GetNextPageToken(),
		Query:         query,
	})
	if err != nil {
		return nil, err
	}

	wfIDPrefix := batchOperationWorkflowIDPrefix + domainName + ":"
	entries := make([]*types.BatchOperationListEntry, 0, len(resp.GetExecutions()))
	for _, wf := range resp.GetExecutions() {
		wfID := wf.GetExecution().GetWorkflowID()
		if !strings.HasPrefix(wfID, wfIDPrefix) {
			// Skip legacy CLI batches, since their BatchID won't resolve in Describe/Stop/Update.
			continue
		}
		if wf.CloseStatus != nil && *wf.CloseStatus == types.WorkflowExecutionCloseStatusContinuedAsNew {
			// The continuation run has the same BatchID and a more current state.
			continue
		}
		var startedBy string
		if wf.SearchAttributes != nil {
			_ = json.Unmarshal(wf.SearchAttributes.IndexedFields["Operator"], &startedBy)
		}
		entries = append(entries, &types.BatchOperationListEntry{
			BatchID:   strings.TrimPrefix(wfID, wfIDPrefix),
			State:     batchOperationStateFromCloseStatus(wf.CloseStatus),
			StartedBy: startedBy,
			StartTime: timePtrFromNanos(wf.StartTime),
			CloseTime: timePtrFromNanos(wf.CloseTime),
		})
	}

	return &types.ListBatchOperationsResponse{
		BatchOperations: entries,
		NextPageToken:   resp.GetNextPageToken(),
	}, nil
}

func validateStopBatchOperationRequest(request *types.StopBatchOperationRequest) error {
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

// StopBatchOperation terminates the cadence-batcher workflow for a batch.
func (wh *WorkflowHandler) StopBatchOperation(
	ctx context.Context,
	request *types.StopBatchOperationRequest,
) (*types.StopBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := validateStopBatchOperationRequest(request); err != nil {
		return nil, err
	}

	domainName := request.GetDomain()
	if _, err := wh.GetDomainCache().GetDomainID(domainName); err != nil {
		return nil, err
	}

	wfID := batchOperationWorkflowID(domainName, request.GetBatchID())
	err := wh.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            constants.BatcherLocalDomainName,
		WorkflowExecution: &types.WorkflowExecution{WorkflowID: wfID},
		Reason:            request.GetReason(),
		Identity:          request.GetIdentity(),
	})
	if err != nil {
		var alreadyCompleted *types.WorkflowExecutionAlreadyCompletedError
		if errors.As(err, &alreadyCompleted) {
			// The batch is no longer running, so the desired state is
			// already reached: treat as success rather than an error.
			return &types.StopBatchOperationResponse{}, nil
		}
		return nil, err
	}

	return &types.StopBatchOperationResponse{}, nil
}

func validateUpdateBatchOperationRequest(request *types.UpdateBatchOperationRequest) error {
	if request == nil {
		return validate.ErrRequestNotSet
	}
	if request.GetDomain() == "" {
		return validate.ErrDomainNotSet
	}
	if request.GetBatchID() == "" {
		return validate.ErrBatchIDNotSet
	}
	if request.GetRPS() < 0 {
		return validate.ErrInvalidBatchRPS
	}
	if request.GetConcurrency() < 0 {
		return validate.ErrInvalidBatchConcurrency
	}
	if request.GetRPS() == 0 && request.GetConcurrency() == 0 {
		return validate.ErrBatchUpdateNoFieldsSet
	}
	return nil
}

// UpdateBatchOperation changes the RPS and/or concurrency of a running
// batch by signaling the cadence-batcher workflow; it does not stop or
// restart the batch.
func (wh *WorkflowHandler) UpdateBatchOperation(
	ctx context.Context,
	request *types.UpdateBatchOperationRequest,
) (*types.UpdateBatchOperationResponse, error) {
	if wh.isShuttingDown() {
		return nil, validate.ErrShuttingDown
	}
	if err := validateUpdateBatchOperationRequest(request); err != nil {
		return nil, err
	}

	domainName := request.GetDomain()
	if _, err := wh.GetDomainCache().GetDomainID(domainName); err != nil {
		return nil, err
	}

	signalInput, err := json.Marshal(batcher.TuneSignal{
		RPS:         int(request.GetRPS()),
		Concurrency: int(request.GetConcurrency()),
	})
	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("failed to encode batch tune signal: %v", err)}
	}

	wfID := batchOperationWorkflowID(domainName, request.GetBatchID())
	err = wh.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            constants.BatcherLocalDomainName,
		WorkflowExecution: &types.WorkflowExecution{WorkflowID: wfID},
		SignalName:        batcher.SignalNameTune,
		Input:             signalInput,
		Identity:          request.GetIdentity(),
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateBatchOperationResponse{}, nil
}
