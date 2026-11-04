package types

import "time"

// BatchOperationType identifies which operation a BatchOperation carries.
type BatchOperationType int32

const (
	BatchOperationTypeInvalid BatchOperationType = iota
	BatchOperationTypeTerminate
	BatchOperationTypeCancel
	BatchOperationTypeSignal
	BatchOperationTypeReplicate
	BatchOperationTypeRefresh
)

func (e BatchOperationType) Ptr() *BatchOperationType {
	return &e
}

func (e BatchOperationType) String() string {
	switch e {
	case BatchOperationTypeTerminate:
		return "Terminate"
	case BatchOperationTypeCancel:
		return "Cancel"
	case BatchOperationTypeSignal:
		return "Signal"
	case BatchOperationTypeReplicate:
		return "Replicate"
	case BatchOperationTypeRefresh:
		return "Refresh"
	}
	return "Invalid"
}

// BatchOperationState is the lifecycle state of a batch.
type BatchOperationState int32

const (
	BatchOperationStateInvalid BatchOperationState = iota
	BatchOperationStateRunning
	BatchOperationStateCompleted
	BatchOperationStateFailed
	BatchOperationStateStopped
)

func (e BatchOperationState) Ptr() *BatchOperationState {
	return &e
}

func (e BatchOperationState) String() string {
	switch e {
	case BatchOperationStateRunning:
		return "Running"
	case BatchOperationStateCompleted:
		return "Completed"
	case BatchOperationStateFailed:
		return "Failed"
	case BatchOperationStateStopped:
		return "Stopped"
	}
	return "Invalid"
}

// BatchOperation is the action applied to every workflow matching a batch's query.
// Exactly one of the Xxx fields is set, as identified by OperationType.
type BatchOperation struct {
	OperationType           *BatchOperationType      `json:"operationType,omitempty"`
	TerminateBatchOperation *TerminateBatchOperation `json:"terminateBatchOperation,omitempty"`
	CancelBatchOperation    *CancelBatchOperation    `json:"cancelBatchOperation,omitempty"`
	SignalBatchOperation    *SignalBatchOperation    `json:"signalBatchOperation,omitempty"`
	ReplicateBatchOperation *ReplicateBatchOperation `json:"replicateBatchOperation,omitempty"`
	RefreshBatchOperation   *RefreshBatchOperation   `json:"refreshBatchOperation,omitempty"`
}

func (v *BatchOperation) GetOperationType() (o BatchOperationType) {
	if v != nil && v.OperationType != nil {
		return *v.OperationType
	}
	return
}

func (v *BatchOperation) GetTerminateBatchOperation() *TerminateBatchOperation {
	if v != nil {
		return v.TerminateBatchOperation
	}
	return nil
}

func (v *BatchOperation) GetCancelBatchOperation() *CancelBatchOperation {
	if v != nil {
		return v.CancelBatchOperation
	}
	return nil
}

func (v *BatchOperation) GetSignalBatchOperation() *SignalBatchOperation {
	if v != nil {
		return v.SignalBatchOperation
	}
	return nil
}

func (v *BatchOperation) GetReplicateBatchOperation() *ReplicateBatchOperation {
	if v != nil {
		return v.ReplicateBatchOperation
	}
	return nil
}

func (v *BatchOperation) GetRefreshBatchOperation() *RefreshBatchOperation {
	if v != nil {
		return v.RefreshBatchOperation
	}
	return nil
}

// TerminateBatchOperation terminates each matching workflow.
type TerminateBatchOperation struct {
	// TerminateChildren: whether to also terminate child workflows. Defaults to true if not set.
	TerminateChildren *bool `json:"terminateChildren,omitempty"`
}

func (v *TerminateBatchOperation) GetTerminateChildren() (o bool) {
	if v != nil && v.TerminateChildren != nil {
		return *v.TerminateChildren
	}
	return
}

// CancelBatchOperation requests cancellation of each matching workflow.
type CancelBatchOperation struct {
	// CancelChildren: whether to also cancel child workflows. Defaults to false if not set.
	CancelChildren *bool `json:"cancelChildren,omitempty"`
}

func (v *CancelBatchOperation) GetCancelChildren() (o bool) {
	if v != nil && v.CancelChildren != nil {
		return *v.CancelChildren
	}
	return
}

// SignalBatchOperation sends a signal to each matching workflow.
type SignalBatchOperation struct {
	SignalName string `json:"signalName,omitempty"`
	Input      []byte `json:"input,omitempty"`
}

func (v *SignalBatchOperation) GetSignalName() (o string) {
	if v != nil {
		return v.SignalName
	}
	return
}

func (v *SignalBatchOperation) GetInput() []byte {
	if v != nil {
		return v.Input
	}
	return nil
}

// ReplicateBatchOperation re-sends replication tasks for each matching workflow.
// The batch must run in the source cluster. Like the single-workflow
// AdminAPI.ResendReplicationTasks, this requires admin access to the domain;
// write access alone is not sufficient.
type ReplicateBatchOperation struct {
	SourceCluster string `json:"sourceCluster,omitempty"`
	TargetCluster string `json:"targetCluster,omitempty"`
}

func (v *ReplicateBatchOperation) GetSourceCluster() (o string) {
	if v != nil {
		return v.SourceCluster
	}
	return
}

func (v *ReplicateBatchOperation) GetTargetCluster() (o string) {
	if v != nil {
		return v.TargetCluster
	}
	return
}

// RefreshBatchOperation refreshes the tasks of each matching workflow. Like
// the single-workflow RefreshWorkflowTasks, this requires admin access to
// the domain; write access alone is not sufficient.
type RefreshBatchOperation struct {
}

// BatchOperationLimits controls how fast a batch is applied. The server
// substitutes defaults for any field left unset.
type BatchOperationLimits struct {
	// RPS: operations per second across the whole batch.
	RPS int32 `json:"rps,omitempty"`
	// Concurrency: number of workflows processed in parallel.
	Concurrency int32 `json:"concurrency,omitempty"`
	// PageSize: number of workflows fetched from visibility per page.
	PageSize int32 `json:"pageSize,omitempty"`
}

func (v *BatchOperationLimits) GetRPS() (o int32) {
	if v != nil {
		return v.RPS
	}
	return
}

func (v *BatchOperationLimits) GetConcurrency() (o int32) {
	if v != nil {
		return v.Concurrency
	}
	return
}

func (v *BatchOperationLimits) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

// BatchOperationProgress reports how far a batch has got.
type BatchOperationProgress struct {
	// TotalEstimate: estimated total, taken when the batch starts. Visibility is
	// eventually consistent, so this is not exact.
	TotalEstimate int64 `json:"totalEstimate,omitempty"`
	SuccessCount  int64 `json:"successCount,omitempty"`
	ErrorCount    int64 `json:"errorCount,omitempty"`
}

func (v *BatchOperationProgress) GetTotalEstimate() (o int64) {
	if v != nil {
		return v.TotalEstimate
	}
	return
}

func (v *BatchOperationProgress) GetSuccessCount() (o int64) {
	if v != nil {
		return v.SuccessCount
	}
	return
}

func (v *BatchOperationProgress) GetErrorCount() (o int64) {
	if v != nil {
		return v.ErrorCount
	}
	return
}

// BatchOperationListEntry is the summary of a batch returned by ListBatchOperations.
type BatchOperationListEntry struct {
	BatchID   string              `json:"batchID,omitempty"`
	State     BatchOperationState `json:"state,omitempty"`
	StartedBy string              `json:"startedBy,omitempty"`
	StartTime *time.Time          `json:"startTime,omitempty"`
	CloseTime *time.Time          `json:"closeTime,omitempty"`
}

func (v *BatchOperationListEntry) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

func (v *BatchOperationListEntry) GetState() (o BatchOperationState) {
	if v != nil {
		return v.State
	}
	return
}

func (v *BatchOperationListEntry) GetStartedBy() (o string) {
	if v != nil {
		return v.StartedBy
	}
	return
}

func (v *BatchOperationListEntry) GetStartTime() *time.Time {
	if v != nil {
		return v.StartTime
	}
	return nil
}

func (v *BatchOperationListEntry) GetCloseTime() *time.Time {
	if v != nil {
		return v.CloseTime
	}
	return nil
}

// StartBatchOperationRequest is the request to StartBatchOperation.
type StartBatchOperationRequest struct {
	// Domain: the domain whose workflows the operation applies to.
	Domain string `json:"domain,omitempty"`
	// BatchID: client-provided batch identifier. Required. Scoped to the domain:
	// used for idempotency, retries with the same domain and batch ID are
	// deduplicated, but the same batch ID in a different domain is a separate batch.
	BatchID string `json:"batchID,omitempty"`
	// Query: visibility query selecting the workflows to operate on.
	Query     string                `json:"query,omitempty"`
	Operation *BatchOperation       `json:"operation,omitempty"`
	Reason    string                `json:"reason,omitempty"`
	Identity  string                `json:"identity,omitempty"`
	Limits    *BatchOperationLimits `json:"limits,omitempty"`
}

func (v *StartBatchOperationRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *StartBatchOperationRequest) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

func (v *StartBatchOperationRequest) GetQuery() (o string) {
	if v != nil {
		return v.Query
	}
	return
}

func (v *StartBatchOperationRequest) GetOperation() *BatchOperation {
	if v != nil {
		return v.Operation
	}
	return nil
}

func (v *StartBatchOperationRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

func (v *StartBatchOperationRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

func (v *StartBatchOperationRequest) GetLimits() *BatchOperationLimits {
	if v != nil {
		return v.Limits
	}
	return nil
}

// StartBatchOperationResponse is the response from StartBatchOperation.
type StartBatchOperationResponse struct {
	BatchID string `json:"batchID,omitempty"`
}

func (v *StartBatchOperationResponse) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

// DescribeBatchOperationRequest is the request to DescribeBatchOperation.
type DescribeBatchOperationRequest struct {
	Domain  string `json:"domain,omitempty"`
	BatchID string `json:"batchID,omitempty"`
}

func (v *DescribeBatchOperationRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *DescribeBatchOperationRequest) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

// DescribeBatchOperationResponse is the response from DescribeBatchOperation.
type DescribeBatchOperationResponse struct {
	Operation *BatchOperation         `json:"operation,omitempty"`
	Query     string                  `json:"query,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	State     BatchOperationState     `json:"state,omitempty"`
	Progress  *BatchOperationProgress `json:"progress,omitempty"`
	Limits    *BatchOperationLimits   `json:"limits,omitempty"`
	StartedBy string                  `json:"startedBy,omitempty"`
	StartTime *time.Time              `json:"startTime,omitempty"`
	CloseTime *time.Time              `json:"closeTime,omitempty"`
}

func (v *DescribeBatchOperationResponse) GetOperation() *BatchOperation {
	if v != nil {
		return v.Operation
	}
	return nil
}

func (v *DescribeBatchOperationResponse) GetQuery() (o string) {
	if v != nil {
		return v.Query
	}
	return
}

func (v *DescribeBatchOperationResponse) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

func (v *DescribeBatchOperationResponse) GetState() (o BatchOperationState) {
	if v != nil {
		return v.State
	}
	return
}

func (v *DescribeBatchOperationResponse) GetProgress() *BatchOperationProgress {
	if v != nil {
		return v.Progress
	}
	return nil
}

func (v *DescribeBatchOperationResponse) GetLimits() *BatchOperationLimits {
	if v != nil {
		return v.Limits
	}
	return nil
}

func (v *DescribeBatchOperationResponse) GetStartedBy() (o string) {
	if v != nil {
		return v.StartedBy
	}
	return
}

func (v *DescribeBatchOperationResponse) GetStartTime() *time.Time {
	if v != nil {
		return v.StartTime
	}
	return nil
}

func (v *DescribeBatchOperationResponse) GetCloseTime() *time.Time {
	if v != nil {
		return v.CloseTime
	}
	return nil
}

// ListBatchOperationsRequest is the request to ListBatchOperations.
type ListBatchOperationsRequest struct {
	Domain string `json:"domain,omitempty"`
	// State: optional filter. If unset, batches in any state are returned.
	State         BatchOperationState `json:"state,omitempty"`
	PageSize      int32               `json:"pageSize,omitempty"`
	NextPageToken []byte              `json:"nextPageToken,omitempty"`
}

func (v *ListBatchOperationsRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *ListBatchOperationsRequest) GetState() (o BatchOperationState) {
	if v != nil {
		return v.State
	}
	return
}

func (v *ListBatchOperationsRequest) GetPageSize() (o int32) {
	if v != nil {
		return v.PageSize
	}
	return
}

func (v *ListBatchOperationsRequest) GetNextPageToken() []byte {
	if v != nil {
		return v.NextPageToken
	}
	return nil
}

// ListBatchOperationsResponse is the response from ListBatchOperations.
type ListBatchOperationsResponse struct {
	BatchOperations []*BatchOperationListEntry `json:"batchOperations,omitempty"`
	NextPageToken   []byte                     `json:"nextPageToken,omitempty"`
}

func (v *ListBatchOperationsResponse) GetBatchOperations() []*BatchOperationListEntry {
	if v != nil {
		return v.BatchOperations
	}
	return nil
}

func (v *ListBatchOperationsResponse) GetNextPageToken() []byte {
	if v != nil {
		return v.NextPageToken
	}
	return nil
}

// StopBatchOperationRequest is the request to StopBatchOperation.
type StopBatchOperationRequest struct {
	Domain   string `json:"domain,omitempty"`
	BatchID  string `json:"batchID,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Identity string `json:"identity,omitempty"`
}

func (v *StopBatchOperationRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *StopBatchOperationRequest) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

func (v *StopBatchOperationRequest) GetReason() (o string) {
	if v != nil {
		return v.Reason
	}
	return
}

func (v *StopBatchOperationRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// StopBatchOperationResponse is the response from StopBatchOperation.
type StopBatchOperationResponse struct {
}

// UpdateBatchOperationRequest is the request to UpdateBatchOperation.
type UpdateBatchOperationRequest struct {
	Domain  string `json:"domain,omitempty"`
	BatchID string `json:"batchID,omitempty"`
	// RPS: new operations per second across the whole batch. Zero leaves the
	// current value unchanged.
	RPS int32 `json:"rps,omitempty"`
	// Concurrency: new number of workflows processed in parallel. Zero leaves
	// the current value unchanged.
	Concurrency int32  `json:"concurrency,omitempty"`
	Identity    string `json:"identity,omitempty"`
}

func (v *UpdateBatchOperationRequest) GetDomain() (o string) {
	if v != nil {
		return v.Domain
	}
	return
}

func (v *UpdateBatchOperationRequest) GetBatchID() (o string) {
	if v != nil {
		return v.BatchID
	}
	return
}

func (v *UpdateBatchOperationRequest) GetRPS() (o int32) {
	if v != nil {
		return v.RPS
	}
	return
}

func (v *UpdateBatchOperationRequest) GetConcurrency() (o int32) {
	if v != nil {
		return v.Concurrency
	}
	return
}

func (v *UpdateBatchOperationRequest) GetIdentity() (o string) {
	if v != nil {
		return v.Identity
	}
	return
}

// UpdateBatchOperationResponse is the response from UpdateBatchOperation.
type UpdateBatchOperationResponse struct {
}
