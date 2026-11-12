package testdata

import (
	"time"

	"github.com/uber/cadence/common/types"
)

var (
	// batchStartTime/batchCloseTime use time.Unix (Local zone) rather than
	// time.Date(...UTC) because timestampToTime in the proto mapper produces
	// time.Unix results, and the two don't compare equal despite the same instant.
	batchStartTime = time.Unix(1767258000, 0)
	batchCloseTime = time.Unix(1767261600, 0)

	TerminateBatchOperation = types.TerminateBatchOperation{
		TerminateChildren: boolPtr(true),
	}

	CancelBatchOperation = types.CancelBatchOperation{
		CancelChildren: boolPtr(false),
	}

	SignalBatchOperation = types.SignalBatchOperation{
		SignalName: "my-signal",
		Input:      []byte("signal-input"),
	}

	ReplicateBatchOperation = types.ReplicateBatchOperation{
		SourceCluster: "cluster-a",
		TargetCluster: "cluster-b",
	}

	RefreshBatchOperation = types.RefreshBatchOperation{}

	BatchOperation = types.BatchOperation{
		OperationType:        types.BatchOperationTypeSignal.Ptr(),
		SignalBatchOperation: &SignalBatchOperation,
	}

	BatchOperationLimits = types.BatchOperationLimits{
		RPS:         10,
		Concurrency: 5,
		PageSize:    100,
	}

	BatchOperationProgress = types.BatchOperationProgress{
		TotalEstimate: 1000,
		SuccessCount:  900,
		ErrorCount:    10,
	}

	BatchOperationListEntry = types.BatchOperationListEntry{
		BatchID:   "my-batch-id",
		State:     types.BatchOperationStateRunning,
		StartedBy: "my-identity",
		StartTime: &batchStartTime,
		CloseTime: &batchCloseTime,
	}

	StartBatchOperationRequest = types.StartBatchOperationRequest{
		Domain:    DomainName,
		BatchID:   "my-batch-id",
		Query:     "WorkflowType = 'my-workflow-type'",
		Operation: &BatchOperation,
		Reason:    "my-reason",
		Identity:  "my-identity",
		Limits:    &BatchOperationLimits,
	}

	StartBatchOperationResponse = types.StartBatchOperationResponse{
		BatchID: "my-batch-id",
	}

	DescribeBatchOperationRequest = types.DescribeBatchOperationRequest{
		Domain:  DomainName,
		BatchID: "my-batch-id",
	}

	DescribeBatchOperationResponse = types.DescribeBatchOperationResponse{
		Operation: &BatchOperation,
		Query:     "WorkflowType = 'my-workflow-type'",
		Reason:    "my-reason",
		State:     types.BatchOperationStateRunning,
		Progress:  &BatchOperationProgress,
		Limits:    &BatchOperationLimits,
		StartedBy: "my-identity",
		StartTime: &batchStartTime,
		CloseTime: &batchCloseTime,
	}

	ListBatchOperationsRequest = types.ListBatchOperationsRequest{
		Domain:        DomainName,
		State:         types.BatchOperationStateRunning,
		PageSize:      100,
		NextPageToken: []byte("next-page-token"),
	}

	ListBatchOperationsResponse = types.ListBatchOperationsResponse{
		BatchOperations: []*types.BatchOperationListEntry{&BatchOperationListEntry},
		NextPageToken:   []byte("next-page-token"),
	}

	StopBatchOperationRequest = types.StopBatchOperationRequest{
		Domain:   DomainName,
		BatchID:  "my-batch-id",
		Reason:   "my-reason",
		Identity: "my-identity",
	}

	StopBatchOperationResponse = types.StopBatchOperationResponse{}

	UpdateBatchOperationRequest = types.UpdateBatchOperationRequest{
		Domain:      DomainName,
		BatchID:     "my-batch-id",
		RPS:         10,
		Concurrency: 5,
		Identity:    "my-identity",
	}

	UpdateBatchOperationResponse = types.UpdateBatchOperationResponse{}
)

func boolPtr(b bool) *bool {
	return &b
}
