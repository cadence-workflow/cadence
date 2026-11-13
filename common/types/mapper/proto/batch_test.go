package proto

import (
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestTerminateBatchOperation(t *testing.T) {
	for _, item := range []*types.TerminateBatchOperation{nil, {}, &testdata.TerminateBatchOperation} {
		assert.Equal(t, item, ToTerminateBatchOperation(FromTerminateBatchOperation(item)))
	}
}

func TestCancelBatchOperation(t *testing.T) {
	for _, item := range []*types.CancelBatchOperation{nil, {}, &testdata.CancelBatchOperation} {
		assert.Equal(t, item, ToCancelBatchOperation(FromCancelBatchOperation(item)))
	}
}

func TestSignalBatchOperation(t *testing.T) {
	for _, item := range []*types.SignalBatchOperation{nil, {}, &testdata.SignalBatchOperation} {
		assert.Equal(t, item, ToSignalBatchOperation(FromSignalBatchOperation(item)))
	}
}

func TestReplicateBatchOperation(t *testing.T) {
	for _, item := range []*types.ReplicateBatchOperation{nil, {}, &testdata.ReplicateBatchOperation} {
		assert.Equal(t, item, ToReplicateBatchOperation(FromReplicateBatchOperation(item)))
	}
}

func TestRefreshBatchOperation(t *testing.T) {
	for _, item := range []*types.RefreshBatchOperation{nil, {}, &testdata.RefreshBatchOperation} {
		assert.Equal(t, item, ToRefreshBatchOperation(FromRefreshBatchOperation(item)))
	}
}

func TestBatchOperation(t *testing.T) {
	for _, item := range []*types.BatchOperation{nil, {}, &testdata.BatchOperation} {
		assert.Equal(t, item, ToBatchOperation(FromBatchOperation(item)))
	}
}

func TestBatchOperationLimits(t *testing.T) {
	for _, item := range []*types.BatchOperationLimits{nil, {}, &testdata.BatchOperationLimits} {
		assert.Equal(t, item, ToBatchOperationLimits(FromBatchOperationLimits(item)))
	}
}

func TestBatchOperationProgress(t *testing.T) {
	for _, item := range []*types.BatchOperationProgress{nil, {}, &testdata.BatchOperationProgress} {
		assert.Equal(t, item, ToBatchOperationProgress(FromBatchOperationProgress(item)))
	}
}

func TestBatchOperationListEntry(t *testing.T) {
	for _, item := range []*types.BatchOperationListEntry{nil, {}, &testdata.BatchOperationListEntry} {
		assert.Equal(t, item, ToBatchOperationListEntry(FromBatchOperationListEntry(item)))
	}
}

func TestStartBatchOperationRequest(t *testing.T) {
	for _, item := range []*types.StartBatchOperationRequest{nil, {}, &testdata.StartBatchOperationRequest} {
		assert.Equal(t, item, ToStartBatchOperationRequest(FromStartBatchOperationRequest(item)))
	}
}

func TestStartBatchOperationResponse(t *testing.T) {
	for _, item := range []*types.StartBatchOperationResponse{nil, {}, &testdata.StartBatchOperationResponse} {
		assert.Equal(t, item, ToStartBatchOperationResponse(FromStartBatchOperationResponse(item)))
	}
}

func TestDescribeBatchOperationRequest(t *testing.T) {
	for _, item := range []*types.DescribeBatchOperationRequest{nil, {}, &testdata.DescribeBatchOperationRequest} {
		assert.Equal(t, item, ToDescribeBatchOperationRequest(FromDescribeBatchOperationRequest(item)))
	}
}

func TestDescribeBatchOperationResponse(t *testing.T) {
	for _, item := range []*types.DescribeBatchOperationResponse{nil, {}, &testdata.DescribeBatchOperationResponse} {
		assert.Equal(t, item, ToDescribeBatchOperationResponse(FromDescribeBatchOperationResponse(item)))
	}
}

func TestListBatchOperationsRequest(t *testing.T) {
	for _, item := range []*types.ListBatchOperationsRequest{nil, {}, &testdata.ListBatchOperationsRequest} {
		assert.Equal(t, item, ToListBatchOperationsRequest(FromListBatchOperationsRequest(item)))
	}
}

func TestListBatchOperationsResponse(t *testing.T) {
	for _, item := range []*types.ListBatchOperationsResponse{nil, {}, &testdata.ListBatchOperationsResponse} {
		assert.Equal(t, item, ToListBatchOperationsResponse(FromListBatchOperationsResponse(item)))
	}
}

func TestStopBatchOperationRequest(t *testing.T) {
	for _, item := range []*types.StopBatchOperationRequest{nil, {}, &testdata.StopBatchOperationRequest} {
		assert.Equal(t, item, ToStopBatchOperationRequest(FromStopBatchOperationRequest(item)))
	}
}

func TestStopBatchOperationResponse(t *testing.T) {
	for _, item := range []*types.StopBatchOperationResponse{nil, {}, &testdata.StopBatchOperationResponse} {
		assert.Equal(t, item, ToStopBatchOperationResponse(FromStopBatchOperationResponse(item)))
	}
}

func TestUpdateBatchOperationRequest(t *testing.T) {
	for _, item := range []*types.UpdateBatchOperationRequest{nil, {}, &testdata.UpdateBatchOperationRequest} {
		assert.Equal(t, item, ToUpdateBatchOperationRequest(FromUpdateBatchOperationRequest(item)))
	}
}

func TestUpdateBatchOperationResponse(t *testing.T) {
	for _, item := range []*types.UpdateBatchOperationResponse{nil, {}, &testdata.UpdateBatchOperationResponse} {
		assert.Equal(t, item, ToUpdateBatchOperationResponse(FromUpdateBatchOperationResponse(item)))
	}
}

// BatchOperationTypeFuzzer constrains the discriminator to a valid, non-invalid
// BatchOperationType. Invalid (0) is excluded because FromBatchOperation leaves
// the proto oneof unset for it, which does not round-trip back to Invalid.
func BatchOperationTypeFuzzer(e *types.BatchOperationType, c fuzz.Continue) {
	*e = types.BatchOperationType(c.Intn(5) + 1) // 1-5: Terminate, Cancel, Signal, Replicate, Refresh
}

func BatchOperationStateFuzzer(e *types.BatchOperationState, c fuzz.Continue) {
	*e = types.BatchOperationState(c.Intn(5)) // 0-4: Invalid, Running, Completed, Failed, Stopped
}

// batchTimeFuzzer never produces a zero time.Time. timeToTimestamp maps a zero
// time to a nil proto timestamp, and timestampToTime maps nil back to a nil
// *time.Time (not a zero time.Time), so a zero input would never round-trip.
func batchTimeFuzzer(t *time.Time, c fuzz.Continue) {
	*t = time.Unix(c.Int63n(testutils.MaxSafeTimestampSeconds)+1, c.Int63n(testutils.NanosecondsPerSecond))
}

// batchOperationFuzzer forces OperationType to a valid variant and clears every
// sibling attribute field inconsistent with it, since the mapper only ever
// encodes/decodes the single field matching the discriminator.
func batchOperationFuzzer(d *types.BatchOperation, c fuzz.Continue) {
	c.Fuzz(d)

	opType := types.BatchOperationType(c.Intn(5) + 1)
	d.OperationType = &opType

	d.TerminateBatchOperation = nil
	d.CancelBatchOperation = nil
	d.SignalBatchOperation = nil
	d.ReplicateBatchOperation = nil
	d.RefreshBatchOperation = nil

	switch opType {
	case types.BatchOperationTypeTerminate:
		c.Fuzz(&d.TerminateBatchOperation)
	case types.BatchOperationTypeCancel:
		c.Fuzz(&d.CancelBatchOperation)
	case types.BatchOperationTypeSignal:
		c.Fuzz(&d.SignalBatchOperation)
	case types.BatchOperationTypeReplicate:
		c.Fuzz(&d.ReplicateBatchOperation)
	case types.BatchOperationTypeRefresh:
		d.RefreshBatchOperation = &types.RefreshBatchOperation{}
	}
}

func TestTerminateBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTerminateBatchOperation, ToTerminateBatchOperation)
}

func TestCancelBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCancelBatchOperation, ToCancelBatchOperation)
}

func TestSignalBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalBatchOperation, ToSignalBatchOperation)
}

func TestReplicateBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromReplicateBatchOperation, ToReplicateBatchOperation)
}

func TestRefreshBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRefreshBatchOperation, ToRefreshBatchOperation)
}

func TestBatchOperationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBatchOperation, ToBatchOperation,
		testutils.WithCustomFuncs(
			batchOperationFuzzer,
			BatchOperationTypeFuzzer,
		),
	)
}

func TestBatchOperationLimitsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBatchOperationLimits, ToBatchOperationLimits)
}

func TestBatchOperationProgressFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBatchOperationProgress, ToBatchOperationProgress)
}

func TestBatchOperationListEntryFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBatchOperationListEntry, ToBatchOperationListEntry,
		testutils.WithCustomFuncs(BatchOperationStateFuzzer, batchTimeFuzzer),
	)
}

func TestStartBatchOperationRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartBatchOperationRequest, ToStartBatchOperationRequest,
		testutils.WithCustomFuncs(
			batchOperationFuzzer,
			BatchOperationTypeFuzzer,
		),
	)
}

func TestStartBatchOperationResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartBatchOperationResponse, ToStartBatchOperationResponse)
}

func TestDescribeBatchOperationRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDescribeBatchOperationRequest, ToDescribeBatchOperationRequest)
}

func TestDescribeBatchOperationResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDescribeBatchOperationResponse, ToDescribeBatchOperationResponse,
		testutils.WithCustomFuncs(
			batchOperationFuzzer,
			BatchOperationTypeFuzzer,
			BatchOperationStateFuzzer,
			batchTimeFuzzer,
		),
	)
}

func TestListBatchOperationsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListBatchOperationsRequest, ToListBatchOperationsRequest,
		testutils.WithCustomFuncs(BatchOperationStateFuzzer),
	)
}

func TestListBatchOperationsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListBatchOperationsResponse, ToListBatchOperationsResponse,
		testutils.WithCustomFuncs(BatchOperationStateFuzzer, batchTimeFuzzer),
	)
}

func TestStopBatchOperationRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStopBatchOperationRequest, ToStopBatchOperationRequest)
}

func TestStopBatchOperationResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStopBatchOperationResponse, ToStopBatchOperationResponse)
}

func TestUpdateBatchOperationRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromUpdateBatchOperationRequest, ToUpdateBatchOperationRequest)
}

func TestUpdateBatchOperationResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromUpdateBatchOperationResponse, ToUpdateBatchOperationResponse)
}
