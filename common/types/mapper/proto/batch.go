package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common/types"
)

func FromBatchOperationState(s types.BatchOperationState) apiv1.BatchOperationState {
	switch s {
	case types.BatchOperationStateRunning:
		return apiv1.BatchOperationState_BATCH_OPERATION_STATE_RUNNING
	case types.BatchOperationStateCompleted:
		return apiv1.BatchOperationState_BATCH_OPERATION_STATE_COMPLETED
	case types.BatchOperationStateFailed:
		return apiv1.BatchOperationState_BATCH_OPERATION_STATE_FAILED
	case types.BatchOperationStateStopped:
		return apiv1.BatchOperationState_BATCH_OPERATION_STATE_STOPPED
	}
	return apiv1.BatchOperationState_BATCH_OPERATION_STATE_INVALID
}

func ToBatchOperationState(s apiv1.BatchOperationState) types.BatchOperationState {
	switch s {
	case apiv1.BatchOperationState_BATCH_OPERATION_STATE_RUNNING:
		return types.BatchOperationStateRunning
	case apiv1.BatchOperationState_BATCH_OPERATION_STATE_COMPLETED:
		return types.BatchOperationStateCompleted
	case apiv1.BatchOperationState_BATCH_OPERATION_STATE_FAILED:
		return types.BatchOperationStateFailed
	case apiv1.BatchOperationState_BATCH_OPERATION_STATE_STOPPED:
		return types.BatchOperationStateStopped
	}
	return types.BatchOperationStateInvalid
}

func FromTerminateBatchOperation(t *types.TerminateBatchOperation) *apiv1.TerminateBatchOperation {
	if t == nil {
		return nil
	}
	return &apiv1.TerminateBatchOperation{
		TerminateChildren: fromBoolValue(t.TerminateChildren),
	}
}

func ToTerminateBatchOperation(t *apiv1.TerminateBatchOperation) *types.TerminateBatchOperation {
	if t == nil {
		return nil
	}
	return &types.TerminateBatchOperation{
		TerminateChildren: toBoolValue(t.TerminateChildren),
	}
}

func FromCancelBatchOperation(t *types.CancelBatchOperation) *apiv1.CancelBatchOperation {
	if t == nil {
		return nil
	}
	return &apiv1.CancelBatchOperation{
		CancelChildren: fromBoolValue(t.CancelChildren),
	}
}

func ToCancelBatchOperation(t *apiv1.CancelBatchOperation) *types.CancelBatchOperation {
	if t == nil {
		return nil
	}
	return &types.CancelBatchOperation{
		CancelChildren: toBoolValue(t.CancelChildren),
	}
}

func FromSignalBatchOperation(t *types.SignalBatchOperation) *apiv1.SignalBatchOperation {
	if t == nil {
		return nil
	}
	return &apiv1.SignalBatchOperation{
		SignalName: t.SignalName,
		Input:      FromPayload(t.Input),
	}
}

func ToSignalBatchOperation(t *apiv1.SignalBatchOperation) *types.SignalBatchOperation {
	if t == nil {
		return nil
	}
	return &types.SignalBatchOperation{
		SignalName: t.SignalName,
		Input:      ToPayload(t.Input),
	}
}

func FromReplicateBatchOperation(t *types.ReplicateBatchOperation) *apiv1.ReplicateBatchOperation {
	if t == nil {
		return nil
	}
	return &apiv1.ReplicateBatchOperation{
		SourceCluster: t.SourceCluster,
		TargetCluster: t.TargetCluster,
	}
}

func ToReplicateBatchOperation(t *apiv1.ReplicateBatchOperation) *types.ReplicateBatchOperation {
	if t == nil {
		return nil
	}
	return &types.ReplicateBatchOperation{
		SourceCluster: t.SourceCluster,
		TargetCluster: t.TargetCluster,
	}
}

func FromRefreshBatchOperation(t *types.RefreshBatchOperation) *apiv1.RefreshBatchOperation {
	if t == nil {
		return nil
	}
	return &apiv1.RefreshBatchOperation{}
}

func ToRefreshBatchOperation(t *apiv1.RefreshBatchOperation) *types.RefreshBatchOperation {
	if t == nil {
		return nil
	}
	return &types.RefreshBatchOperation{}
}

func FromBatchOperation(t *types.BatchOperation) *apiv1.BatchOperation {
	if t == nil {
		return nil
	}
	op := &apiv1.BatchOperation{}
	switch t.GetOperationType() {
	case types.BatchOperationTypeTerminate:
		op.Operation = &apiv1.BatchOperation_Terminate{
			Terminate: FromTerminateBatchOperation(t.TerminateBatchOperation),
		}
	case types.BatchOperationTypeCancel:
		op.Operation = &apiv1.BatchOperation_Cancel{
			Cancel: FromCancelBatchOperation(t.CancelBatchOperation),
		}
	case types.BatchOperationTypeSignal:
		op.Operation = &apiv1.BatchOperation_Signal{
			Signal: FromSignalBatchOperation(t.SignalBatchOperation),
		}
	case types.BatchOperationTypeReplicate:
		op.Operation = &apiv1.BatchOperation_Replicate{
			Replicate: FromReplicateBatchOperation(t.ReplicateBatchOperation),
		}
	case types.BatchOperationTypeRefresh:
		op.Operation = &apiv1.BatchOperation_Refresh{
			Refresh: FromRefreshBatchOperation(t.RefreshBatchOperation),
		}
	}
	return op
}

func ToBatchOperation(t *apiv1.BatchOperation) *types.BatchOperation {
	if t == nil {
		return nil
	}
	op := &types.BatchOperation{}
	switch attr := t.Operation.(type) {
	case *apiv1.BatchOperation_Terminate:
		op.OperationType = types.BatchOperationTypeTerminate.Ptr()
		op.TerminateBatchOperation = ToTerminateBatchOperation(attr.Terminate)
	case *apiv1.BatchOperation_Cancel:
		op.OperationType = types.BatchOperationTypeCancel.Ptr()
		op.CancelBatchOperation = ToCancelBatchOperation(attr.Cancel)
	case *apiv1.BatchOperation_Signal:
		op.OperationType = types.BatchOperationTypeSignal.Ptr()
		op.SignalBatchOperation = ToSignalBatchOperation(attr.Signal)
	case *apiv1.BatchOperation_Replicate:
		op.OperationType = types.BatchOperationTypeReplicate.Ptr()
		op.ReplicateBatchOperation = ToReplicateBatchOperation(attr.Replicate)
	case *apiv1.BatchOperation_Refresh:
		op.OperationType = types.BatchOperationTypeRefresh.Ptr()
		op.RefreshBatchOperation = ToRefreshBatchOperation(attr.Refresh)
	}
	return op
}

func FromBatchOperationLimits(t *types.BatchOperationLimits) *apiv1.BatchOperationLimits {
	if t == nil {
		return nil
	}
	return &apiv1.BatchOperationLimits{
		Rps:         t.RPS,
		Concurrency: t.Concurrency,
		PageSize:    t.PageSize,
	}
}

func ToBatchOperationLimits(t *apiv1.BatchOperationLimits) *types.BatchOperationLimits {
	if t == nil {
		return nil
	}
	return &types.BatchOperationLimits{
		RPS:         t.Rps,
		Concurrency: t.Concurrency,
		PageSize:    t.PageSize,
	}
}

func FromBatchOperationProgress(t *types.BatchOperationProgress) *apiv1.BatchOperationProgress {
	if t == nil {
		return nil
	}
	return &apiv1.BatchOperationProgress{
		TotalEstimate: t.TotalEstimate,
		SuccessCount:  t.SuccessCount,
		ErrorCount:    t.ErrorCount,
	}
}

func ToBatchOperationProgress(t *apiv1.BatchOperationProgress) *types.BatchOperationProgress {
	if t == nil {
		return nil
	}
	return &types.BatchOperationProgress{
		TotalEstimate: t.TotalEstimate,
		SuccessCount:  t.SuccessCount,
		ErrorCount:    t.ErrorCount,
	}
}

func FromBatchOperationListEntry(t *types.BatchOperationListEntry) *apiv1.BatchOperationListEntry {
	if t == nil {
		return nil
	}
	return &apiv1.BatchOperationListEntry{
		BatchId:   t.BatchID,
		State:     FromBatchOperationState(t.State),
		StartedBy: t.StartedBy,
		StartTime: timeToTimestamp(t.StartTime),
		CloseTime: timeToTimestamp(t.CloseTime),
	}
}

func ToBatchOperationListEntry(t *apiv1.BatchOperationListEntry) *types.BatchOperationListEntry {
	if t == nil {
		return nil
	}
	return &types.BatchOperationListEntry{
		BatchID:   t.BatchId,
		State:     ToBatchOperationState(t.State),
		StartedBy: t.StartedBy,
		StartTime: timestampToTime(t.StartTime),
		CloseTime: timestampToTime(t.CloseTime),
	}
}

func FromStartBatchOperationRequest(t *types.StartBatchOperationRequest) *apiv1.StartBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &apiv1.StartBatchOperationRequest{
		Domain:    t.Domain,
		BatchId:   t.BatchID,
		Query:     t.Query,
		Operation: FromBatchOperation(t.Operation),
		Reason:    t.Reason,
		Identity:  t.Identity,
		Limits:    FromBatchOperationLimits(t.Limits),
	}
}

func ToStartBatchOperationRequest(t *apiv1.StartBatchOperationRequest) *types.StartBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &types.StartBatchOperationRequest{
		Domain:    t.Domain,
		BatchID:   t.BatchId,
		Query:     t.Query,
		Operation: ToBatchOperation(t.Operation),
		Reason:    t.Reason,
		Identity:  t.Identity,
		Limits:    ToBatchOperationLimits(t.Limits),
	}
}

func FromStartBatchOperationResponse(t *types.StartBatchOperationResponse) *apiv1.StartBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &apiv1.StartBatchOperationResponse{
		BatchId: t.BatchID,
	}
}

func ToStartBatchOperationResponse(t *apiv1.StartBatchOperationResponse) *types.StartBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &types.StartBatchOperationResponse{
		BatchID: t.BatchId,
	}
}

func FromDescribeBatchOperationRequest(t *types.DescribeBatchOperationRequest) *apiv1.DescribeBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeBatchOperationRequest{
		Domain:  t.Domain,
		BatchId: t.BatchID,
	}
}

func ToDescribeBatchOperationRequest(t *apiv1.DescribeBatchOperationRequest) *types.DescribeBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeBatchOperationRequest{
		Domain:  t.Domain,
		BatchID: t.BatchId,
	}
}

func FromDescribeBatchOperationResponse(t *types.DescribeBatchOperationResponse) *apiv1.DescribeBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeBatchOperationResponse{
		Operation: FromBatchOperation(t.Operation),
		Query:     t.Query,
		Reason:    t.Reason,
		State:     FromBatchOperationState(t.State),
		Progress:  FromBatchOperationProgress(t.Progress),
		Limits:    FromBatchOperationLimits(t.Limits),
		StartedBy: t.StartedBy,
		StartTime: timeToTimestamp(t.StartTime),
		CloseTime: timeToTimestamp(t.CloseTime),
	}
}

func ToDescribeBatchOperationResponse(t *apiv1.DescribeBatchOperationResponse) *types.DescribeBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeBatchOperationResponse{
		Operation: ToBatchOperation(t.Operation),
		Query:     t.Query,
		Reason:    t.Reason,
		State:     ToBatchOperationState(t.State),
		Progress:  ToBatchOperationProgress(t.Progress),
		Limits:    ToBatchOperationLimits(t.Limits),
		StartedBy: t.StartedBy,
		StartTime: timestampToTime(t.StartTime),
		CloseTime: timestampToTime(t.CloseTime),
	}
}

func FromListBatchOperationsRequest(t *types.ListBatchOperationsRequest) *apiv1.ListBatchOperationsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListBatchOperationsRequest{
		Domain:        t.Domain,
		State:         FromBatchOperationState(t.State),
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func ToListBatchOperationsRequest(t *apiv1.ListBatchOperationsRequest) *types.ListBatchOperationsRequest {
	if t == nil {
		return nil
	}
	return &types.ListBatchOperationsRequest{
		Domain:        t.Domain,
		State:         ToBatchOperationState(t.State),
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func FromBatchOperationListEntryArray(t []*types.BatchOperationListEntry) []*apiv1.BatchOperationListEntry {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.BatchOperationListEntry, len(t))
	for i := range t {
		v[i] = FromBatchOperationListEntry(t[i])
	}
	return v
}

func ToBatchOperationListEntryArray(t []*apiv1.BatchOperationListEntry) []*types.BatchOperationListEntry {
	if t == nil {
		return nil
	}
	v := make([]*types.BatchOperationListEntry, len(t))
	for i := range t {
		v[i] = ToBatchOperationListEntry(t[i])
	}
	return v
}

func FromListBatchOperationsResponse(t *types.ListBatchOperationsResponse) *apiv1.ListBatchOperationsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListBatchOperationsResponse{
		BatchOperations: FromBatchOperationListEntryArray(t.BatchOperations),
		NextPageToken:   t.NextPageToken,
	}
}

func ToListBatchOperationsResponse(t *apiv1.ListBatchOperationsResponse) *types.ListBatchOperationsResponse {
	if t == nil {
		return nil
	}
	return &types.ListBatchOperationsResponse{
		BatchOperations: ToBatchOperationListEntryArray(t.BatchOperations),
		NextPageToken:   t.NextPageToken,
	}
}

func FromStopBatchOperationRequest(t *types.StopBatchOperationRequest) *apiv1.StopBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &apiv1.StopBatchOperationRequest{
		Domain:   t.Domain,
		BatchId:  t.BatchID,
		Reason:   t.Reason,
		Identity: t.Identity,
	}
}

func ToStopBatchOperationRequest(t *apiv1.StopBatchOperationRequest) *types.StopBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &types.StopBatchOperationRequest{
		Domain:   t.Domain,
		BatchID:  t.BatchId,
		Reason:   t.Reason,
		Identity: t.Identity,
	}
}

func FromStopBatchOperationResponse(t *types.StopBatchOperationResponse) *apiv1.StopBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &apiv1.StopBatchOperationResponse{}
}

func ToStopBatchOperationResponse(t *apiv1.StopBatchOperationResponse) *types.StopBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &types.StopBatchOperationResponse{}
}

func FromUpdateBatchOperationRequest(t *types.UpdateBatchOperationRequest) *apiv1.UpdateBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &apiv1.UpdateBatchOperationRequest{
		Domain:      t.Domain,
		BatchId:     t.BatchID,
		Rps:         t.RPS,
		Concurrency: t.Concurrency,
		Identity:    t.Identity,
	}
}

func ToUpdateBatchOperationRequest(t *apiv1.UpdateBatchOperationRequest) *types.UpdateBatchOperationRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateBatchOperationRequest{
		Domain:      t.Domain,
		BatchID:     t.BatchId,
		RPS:         t.Rps,
		Concurrency: t.Concurrency,
		Identity:    t.Identity,
	}
}

func FromUpdateBatchOperationResponse(t *types.UpdateBatchOperationResponse) *apiv1.UpdateBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &apiv1.UpdateBatchOperationResponse{}
}

func ToUpdateBatchOperationResponse(t *apiv1.UpdateBatchOperationResponse) *types.UpdateBatchOperationResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateBatchOperationResponse{}
}
