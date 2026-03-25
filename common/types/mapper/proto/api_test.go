// Copyright (c) 2021 Uber Technologies Inc.
//
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

package proto

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestActivityLocalDispatchInfo(t *testing.T) {
	for _, item := range []*types.ActivityLocalDispatchInfo{nil, {}, &testdata.ActivityLocalDispatchInfo} {
		assert.Equal(t, item, ToActivityLocalDispatchInfo(FromActivityLocalDispatchInfo(item)))
	}
}
func TestActivityTaskCancelRequestedEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskCancelRequestedEventAttributes{nil, {}, &testdata.ActivityTaskCancelRequestedEventAttributes} {
		assert.Equal(t, item, ToActivityTaskCancelRequestedEventAttributes(FromActivityTaskCancelRequestedEventAttributes(item)))
	}
}
func TestActivityTaskCanceledEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskCanceledEventAttributes{nil, {}, &testdata.ActivityTaskCanceledEventAttributes} {
		assert.Equal(t, item, ToActivityTaskCanceledEventAttributes(FromActivityTaskCanceledEventAttributes(item)))
	}
}
func TestActivityTaskCompletedEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskCompletedEventAttributes{nil, {}, &testdata.ActivityTaskCompletedEventAttributes} {
		assert.Equal(t, item, ToActivityTaskCompletedEventAttributes(FromActivityTaskCompletedEventAttributes(item)))
	}
}
func TestActivityTaskFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskFailedEventAttributes{nil, {}, &testdata.ActivityTaskFailedEventAttributes} {
		assert.Equal(t, item, ToActivityTaskFailedEventAttributes(FromActivityTaskFailedEventAttributes(item)))
	}
}
func TestActivityTaskScheduledEventAttributes(t *testing.T) {
	// since proto definition for Domain field doesn't have pointer, To(From(item)) won't be equal to item when item's Domain is a nil pointer
	// this is fine as the code using this field will check both if the field is a nil pointer and if it's a pointer to an empty string.
	for _, item := range []*types.ActivityTaskScheduledEventAttributes{nil, {Domain: common.StringPtr("")}, &testdata.ActivityTaskScheduledEventAttributes} {
		assert.Equal(t, item, ToActivityTaskScheduledEventAttributes(FromActivityTaskScheduledEventAttributes(item)))
	}
}
func TestActivityTaskStartedEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskStartedEventAttributes{nil, {}, &testdata.ActivityTaskStartedEventAttributes} {
		assert.Equal(t, item, ToActivityTaskStartedEventAttributes(FromActivityTaskStartedEventAttributes(item)))
	}
}
func TestActivityTaskTimedOutEventAttributes(t *testing.T) {
	for _, item := range []*types.ActivityTaskTimedOutEventAttributes{nil, {}, &testdata.ActivityTaskTimedOutEventAttributes} {
		assert.Equal(t, item, ToActivityTaskTimedOutEventAttributes(FromActivityTaskTimedOutEventAttributes(item)))
	}
}
func TestActivityType(t *testing.T) {
	for _, item := range []*types.ActivityType{nil, {}, &testdata.ActivityType} {
		assert.Equal(t, item, ToActivityType(FromActivityType(item)))
	}
}
func TestBadBinaries(t *testing.T) {
	for _, item := range []*types.BadBinaries{nil, {}, &testdata.BadBinaries} {
		assert.Equal(t, item, ToBadBinaries(FromBadBinaries(item)))
	}
}
func TestBadBinaryInfo(t *testing.T) {
	for _, item := range []*types.BadBinaryInfo{nil, {}, &testdata.BadBinaryInfo} {
		assert.Equal(t, item, ToBadBinaryInfo(FromBadBinaryInfo(item)))
	}
}
func TestCancelTimerDecisionAttributes(t *testing.T) {
	for _, item := range []*types.CancelTimerDecisionAttributes{nil, {}, &testdata.CancelTimerDecisionAttributes} {
		assert.Equal(t, item, ToCancelTimerDecisionAttributes(FromCancelTimerDecisionAttributes(item)))
	}
}
func TestCancelTimerFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.CancelTimerFailedEventAttributes{nil, {}, &testdata.CancelTimerFailedEventAttributes} {
		assert.Equal(t, item, ToCancelTimerFailedEventAttributes(FromCancelTimerFailedEventAttributes(item)))
	}
}
func TestCancelWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.CancelWorkflowExecutionDecisionAttributes{nil, {}, &testdata.CancelWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToCancelWorkflowExecutionDecisionAttributes(FromCancelWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestChildWorkflowExecutionCanceledEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionCanceledEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionCanceledEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionCanceledEventAttributes(FromChildWorkflowExecutionCanceledEventAttributes(item)))
	}
}
func TestChildWorkflowExecutionCompletedEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionCompletedEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionCompletedEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionCompletedEventAttributes(FromChildWorkflowExecutionCompletedEventAttributes(item)))
	}
}
func TestChildWorkflowExecutionFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionFailedEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionFailedEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionFailedEventAttributes(FromChildWorkflowExecutionFailedEventAttributes(item)))
	}
}
func TestChildWorkflowExecutionStartedEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionStartedEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionStartedEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionStartedEventAttributes(FromChildWorkflowExecutionStartedEventAttributes(item)))
	}
}
func TestChildWorkflowExecutionTerminatedEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionTerminatedEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionTerminatedEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionTerminatedEventAttributes(FromChildWorkflowExecutionTerminatedEventAttributes(item)))
	}
}
func TestChildWorkflowExecutionTimedOutEventAttributes(t *testing.T) {
	for _, item := range []*types.ChildWorkflowExecutionTimedOutEventAttributes{nil, {}, &testdata.ChildWorkflowExecutionTimedOutEventAttributes} {
		assert.Equal(t, item, ToChildWorkflowExecutionTimedOutEventAttributes(FromChildWorkflowExecutionTimedOutEventAttributes(item)))
	}
}
func TestClusterReplicationConfiguration(t *testing.T) {
	for _, item := range []*types.ClusterReplicationConfiguration{nil, {}, &testdata.ClusterReplicationConfiguration} {
		assert.Equal(t, item, ToClusterReplicationConfiguration(FromClusterReplicationConfiguration(item)))
	}
}
func TestCompleteWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.CompleteWorkflowExecutionDecisionAttributes{nil, {}, &testdata.CompleteWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToCompleteWorkflowExecutionDecisionAttributes(FromCompleteWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestContinueAsNewWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.ContinueAsNewWorkflowExecutionDecisionAttributes{nil, {}, &testdata.ContinueAsNewWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToContinueAsNewWorkflowExecutionDecisionAttributes(FromContinueAsNewWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestCountWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.CountWorkflowExecutionsRequest{nil, {}, &testdata.CountWorkflowExecutionsRequest} {
		assert.Equal(t, item, ToCountWorkflowExecutionsRequest(FromCountWorkflowExecutionsRequest(item)))
	}
}
func TestCountWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.CountWorkflowExecutionsResponse{nil, {}, &testdata.CountWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToCountWorkflowExecutionsResponse(FromCountWorkflowExecutionsResponse(item)))
	}
}
func TestDataBlob(t *testing.T) {
	for _, item := range []*types.DataBlob{nil, {}, &testdata.DataBlob} {
		assert.Equal(t, item, ToDataBlob(FromDataBlob(item)))
	}
}
func TestDecisionTaskCompletedEventAttributes(t *testing.T) {
	for _, item := range []*types.DecisionTaskCompletedEventAttributes{nil, {}, &testdata.DecisionTaskCompletedEventAttributes} {
		assert.Equal(t, item, ToDecisionTaskCompletedEventAttributes(FromDecisionTaskCompletedEventAttributes(item)))
	}
}
func TestDecisionTaskFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.DecisionTaskFailedEventAttributes{nil, {}, &testdata.DecisionTaskFailedEventAttributes} {
		assert.Equal(t, item, ToDecisionTaskFailedEventAttributes(FromDecisionTaskFailedEventAttributes(item)))
	}
}
func TestDecisionTaskScheduledEventAttributes(t *testing.T) {
	for _, item := range []*types.DecisionTaskScheduledEventAttributes{nil, {}, &testdata.DecisionTaskScheduledEventAttributes} {
		assert.Equal(t, item, ToDecisionTaskScheduledEventAttributes(FromDecisionTaskScheduledEventAttributes(item)))
	}
}
func TestDecisionTaskStartedEventAttributes(t *testing.T) {
	for _, item := range []*types.DecisionTaskStartedEventAttributes{nil, {}, &testdata.DecisionTaskStartedEventAttributes} {
		assert.Equal(t, item, ToDecisionTaskStartedEventAttributes(FromDecisionTaskStartedEventAttributes(item)))
	}
}
func TestDecisionTaskTimedOutEventAttributes(t *testing.T) {
	for _, item := range []*types.DecisionTaskTimedOutEventAttributes{nil, {}, &testdata.DecisionTaskTimedOutEventAttributes} {
		assert.Equal(t, item, ToDecisionTaskTimedOutEventAttributes(FromDecisionTaskTimedOutEventAttributes(item)))
	}
}
func TestDeleteDomainRequest(t *testing.T) {
	for _, item := range []*types.DeleteDomainRequest{nil, {}, &testdata.DeleteDomainRequest} {
		assert.Equal(t, item, ToDeleteDomainRequest(FromDeleteDomainRequest(item)))
	}
}
func TestDeprecateDomainRequest(t *testing.T) {
	for _, item := range []*types.DeprecateDomainRequest{nil, {}, &testdata.DeprecateDomainRequest} {
		assert.Equal(t, item, ToDeprecateDomainRequest(FromDeprecateDomainRequest(item)))
	}
}
func TestDescribeDomainRequest(t *testing.T) {
	for _, item := range []*types.DescribeDomainRequest{
		&testdata.DescribeDomainRequest_ID,
		&testdata.DescribeDomainRequest_Name,
	} {
		assert.Equal(t, item, ToDescribeDomainRequest(FromDescribeDomainRequest(item)))
	}
	assert.Nil(t, ToDescribeDomainRequest(nil))
	assert.Nil(t, FromDescribeDomainRequest(nil))
	assert.Panics(t, func() { ToDescribeDomainRequest(&apiv1.DescribeDomainRequest{}) })
	assert.Panics(t, func() { FromDescribeDomainRequest(&types.DescribeDomainRequest{}) })
}
func TestDescribeDomainResponse_Domain(t *testing.T) {
	for _, item := range []*types.DescribeDomainResponse{nil, &testdata.DescribeDomainResponse} {
		assert.Equal(t, item, ToDescribeDomainResponseDomain(FromDescribeDomainResponseDomain(item)))
	}
}
func TestDescribeDomainResponse(t *testing.T) {
	for _, item := range []*types.DescribeDomainResponse{nil, &testdata.DescribeDomainResponse} {
		assert.Equal(t, item, ToDescribeDomainResponse(FromDescribeDomainResponse(item)))
	}
}
func TestDescribeTaskListRequest(t *testing.T) {
	for _, item := range []*types.DescribeTaskListRequest{nil, {}, &testdata.DescribeTaskListRequest} {
		assert.Equal(t, item, ToDescribeTaskListRequest(FromDescribeTaskListRequest(item)))
	}
}
func TestDescribeTaskListResponse(t *testing.T) {
	for _, item := range []*types.DescribeTaskListResponse{nil, {}, &testdata.DescribeTaskListResponse} {
		assert.Equal(t, item, ToDescribeTaskListResponse(FromDescribeTaskListResponse(item)))
	}
}
func TestDescribeWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.DescribeWorkflowExecutionRequest{nil, {}, &testdata.DescribeWorkflowExecutionRequest} {
		assert.Equal(t, item, ToDescribeWorkflowExecutionRequest(FromDescribeWorkflowExecutionRequest(item)))
	}
}
func TestDescribeWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.DescribeWorkflowExecutionResponse{nil, {}, &testdata.DescribeWorkflowExecutionResponse} {
		assert.Equal(t, item, ToDescribeWorkflowExecutionResponse(FromDescribeWorkflowExecutionResponse(item)))
	}
}
func TestDiagnoseWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.DiagnoseWorkflowExecutionRequest{nil, {}, &testdata.DiagnoseWorkflowExecutionRequest} {
		assert.Equal(t, item, ToDiagnoseWorkflowExecutionRequest(FromDiagnoseWorkflowExecutionRequest(item)))
	}
}
func TestDiagnoseWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.DiagnoseWorkflowExecutionResponse{nil, {}, &testdata.DiagnoseWorkflowExecutionResponse} {
		assert.Equal(t, item, ToDiagnoseWorkflowExecutionResponse(FromDiagnoseWorkflowExecutionResponse(item)))
	}
}
func TestExternalWorkflowExecutionCancelRequestedEventAttributes(t *testing.T) {
	for _, item := range []*types.ExternalWorkflowExecutionCancelRequestedEventAttributes{nil, {}, &testdata.ExternalWorkflowExecutionCancelRequestedEventAttributes} {
		assert.Equal(t, item, ToExternalWorkflowExecutionCancelRequestedEventAttributes(FromExternalWorkflowExecutionCancelRequestedEventAttributes(item)))
	}
}
func TestExternalWorkflowExecutionSignaledEventAttributes(t *testing.T) {
	for _, item := range []*types.ExternalWorkflowExecutionSignaledEventAttributes{nil, {}, &testdata.ExternalWorkflowExecutionSignaledEventAttributes} {
		assert.Equal(t, item, ToExternalWorkflowExecutionSignaledEventAttributes(FromExternalWorkflowExecutionSignaledEventAttributes(item)))
	}
}
func TestFailWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.FailWorkflowExecutionDecisionAttributes{nil, {}, &testdata.FailWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToFailWorkflowExecutionDecisionAttributes(FromFailWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestGetClusterInfoResponse(t *testing.T) {
	for _, item := range []*types.ClusterInfo{nil, {}, &testdata.ClusterInfo} {
		assert.Equal(t, item, ToGetClusterInfoResponse(FromGetClusterInfoResponse(item)))
	}
}
func TestGetSearchAttributesResponse(t *testing.T) {
	for _, item := range []*types.GetSearchAttributesResponse{nil, {}, &testdata.GetSearchAttributesResponse} {
		assert.Equal(t, item, ToGetSearchAttributesResponse(FromGetSearchAttributesResponse(item)))
	}
}
func TestGetWorkflowExecutionHistoryRequest(t *testing.T) {
	for _, item := range []*types.GetWorkflowExecutionHistoryRequest{nil, {}, &testdata.GetWorkflowExecutionHistoryRequest} {
		assert.Equal(t, item, ToGetWorkflowExecutionHistoryRequest(FromGetWorkflowExecutionHistoryRequest(item)))
	}
}
func TestGetWorkflowExecutionHistoryResponse(t *testing.T) {
	for _, item := range []*types.GetWorkflowExecutionHistoryResponse{nil, {}, &testdata.GetWorkflowExecutionHistoryResponse} {
		assert.Equal(t, item, ToGetWorkflowExecutionHistoryResponse(FromGetWorkflowExecutionHistoryResponse(item)))
	}
}
func TestHeader(t *testing.T) {
	for _, item := range []*types.Header{nil, {}, &testdata.Header} {
		assert.Equal(t, item, ToHeader(FromHeader(item)))
	}
}
func TestHealthResponse(t *testing.T) {
	for _, item := range []*types.HealthStatus{nil, {}, &testdata.HealthStatus} {
		assert.Equal(t, item, ToHealthResponse(FromHealthResponse(item)))
	}
}
func TestHistory(t *testing.T) {
	for _, item := range []*types.History{nil, &testdata.History} {
		assert.Equal(t, item, ToHistory(FromHistory(item)))
	}
}
func TestListArchivedWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.ListArchivedWorkflowExecutionsRequest{nil, {}, &testdata.ListArchivedWorkflowExecutionsRequest} {
		assert.Equal(t, item, ToListArchivedWorkflowExecutionsRequest(FromListArchivedWorkflowExecutionsRequest(item)))
	}
}
func TestListArchivedWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.ListArchivedWorkflowExecutionsResponse{nil, {}, &testdata.ListArchivedWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToListArchivedWorkflowExecutionsResponse(FromListArchivedWorkflowExecutionsResponse(item)))
	}
}
func TestListClosedWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.ListClosedWorkflowExecutionsResponse{nil, {}, &testdata.ListClosedWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToListClosedWorkflowExecutionsResponse(FromListClosedWorkflowExecutionsResponse(item)))
	}
}
func TestListDomainsRequest(t *testing.T) {
	for _, item := range []*types.ListDomainsRequest{nil, {}, &testdata.ListDomainsRequest} {
		assert.Equal(t, item, ToListDomainsRequest(FromListDomainsRequest(item)))
	}
}
func TestListDomainsResponse(t *testing.T) {
	for _, item := range []*types.ListDomainsResponse{nil, {}, &testdata.ListDomainsResponse} {
		assert.Equal(t, item, ToListDomainsResponse(FromListDomainsResponse(item)))
	}
}
func TestListOpenWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.ListOpenWorkflowExecutionsResponse{nil, {}, &testdata.ListOpenWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToListOpenWorkflowExecutionsResponse(FromListOpenWorkflowExecutionsResponse(item)))
	}
}
func TestListTaskListPartitionsRequest(t *testing.T) {
	for _, item := range []*types.ListTaskListPartitionsRequest{nil, {}, &testdata.ListTaskListPartitionsRequest} {
		assert.Equal(t, item, ToListTaskListPartitionsRequest(FromListTaskListPartitionsRequest(item)))
	}
}
func TestListTaskListPartitionsResponse(t *testing.T) {
	for _, item := range []*types.ListTaskListPartitionsResponse{nil, {}, &testdata.ListTaskListPartitionsResponse} {
		assert.Equal(t, item, ToListTaskListPartitionsResponse(FromListTaskListPartitionsResponse(item)))
	}
}
func TestListWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.ListWorkflowExecutionsRequest{nil, {}, &testdata.ListWorkflowExecutionsRequest} {
		assert.Equal(t, item, ToListWorkflowExecutionsRequest(FromListWorkflowExecutionsRequest(item)))
	}
}
func TestListWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.ListWorkflowExecutionsResponse{nil, {}, &testdata.ListWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToListWorkflowExecutionsResponse(FromListWorkflowExecutionsResponse(item)))
	}
}
func TestMarkerRecordedEventAttributes(t *testing.T) {
	for _, item := range []*types.MarkerRecordedEventAttributes{nil, {}, &testdata.MarkerRecordedEventAttributes} {
		assert.Equal(t, item, ToMarkerRecordedEventAttributes(FromMarkerRecordedEventAttributes(item)))
	}
}
func TestMemo(t *testing.T) {
	for _, item := range []*types.Memo{nil, {}, &testdata.Memo} {
		assert.Equal(t, item, ToMemo(FromMemo(item)))
	}
}
func TestPendingActivityInfo(t *testing.T) {
	for _, item := range []*types.PendingActivityInfo{nil, {}, &testdata.PendingActivityInfo} {
		assert.Equal(t, item, ToPendingActivityInfo(FromPendingActivityInfo(item)))
	}
}
func TestPendingChildExecutionInfo(t *testing.T) {
	for _, item := range []*types.PendingChildExecutionInfo{nil, {}, &testdata.PendingChildExecutionInfo} {
		assert.Equal(t, item, ToPendingChildExecutionInfo(FromPendingChildExecutionInfo(item)))
	}
}
func TestPendingDecisionInfo(t *testing.T) {
	for _, item := range []*types.PendingDecisionInfo{nil, {}, &testdata.PendingDecisionInfo} {
		assert.Equal(t, item, ToPendingDecisionInfo(FromPendingDecisionInfo(item)))
	}
}
func TestPollForActivityTaskRequest(t *testing.T) {
	for _, item := range []*types.PollForActivityTaskRequest{nil, {}, &testdata.PollForActivityTaskRequest} {
		assert.Equal(t, item, ToPollForActivityTaskRequest(FromPollForActivityTaskRequest(item)))
	}
}
func TestPollForActivityTaskResponse(t *testing.T) {
	for _, item := range []*types.PollForActivityTaskResponse{nil, {}, &testdata.PollForActivityTaskResponse} {
		assert.Equal(t, item, ToPollForActivityTaskResponse(FromPollForActivityTaskResponse(item)))
	}
}
func TestPollForDecisionTaskRequest(t *testing.T) {
	for _, item := range []*types.PollForDecisionTaskRequest{nil, {}, &testdata.PollForDecisionTaskRequest} {
		assert.Equal(t, item, ToPollForDecisionTaskRequest(FromPollForDecisionTaskRequest(item)))
	}
}
func TestPollForDecisionTaskResponse(t *testing.T) {
	for _, item := range []*types.PollForDecisionTaskResponse{nil, {}, &testdata.PollForDecisionTaskResponse} {
		assert.Equal(t, item, ToPollForDecisionTaskResponse(FromPollForDecisionTaskResponse(item)))
	}
}
func TestPollerInfo(t *testing.T) {
	for _, item := range []*types.PollerInfo{nil, {}, &testdata.PollerInfo} {
		assert.Equal(t, item, ToPollerInfo(FromPollerInfo(item)))
	}
}
func TestQueryRejected(t *testing.T) {
	for _, item := range []*types.QueryRejected{nil, {}, &testdata.QueryRejected} {
		assert.Equal(t, item, ToQueryRejected(FromQueryRejected(item)))
	}
}
func TestQueryWorkflowRequest(t *testing.T) {
	for _, item := range []*types.QueryWorkflowRequest{nil, {}, &testdata.QueryWorkflowRequest} {
		assert.Equal(t, item, ToQueryWorkflowRequest(FromQueryWorkflowRequest(item)))
	}
}
func TestQueryWorkflowResponse(t *testing.T) {
	for _, item := range []*types.QueryWorkflowResponse{nil, {}, &testdata.QueryWorkflowResponse} {
		assert.Equal(t, item, ToQueryWorkflowResponse(FromQueryWorkflowResponse(item)))
	}
}
func TestRecordActivityTaskHeartbeatByIDRequest(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskHeartbeatByIDRequest{nil, {}, &testdata.RecordActivityTaskHeartbeatByIDRequest} {
		assert.Equal(t, item, ToRecordActivityTaskHeartbeatByIDRequest(FromRecordActivityTaskHeartbeatByIDRequest(item)))
	}
}
func TestRecordActivityTaskHeartbeatByIDResponse(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskHeartbeatResponse{nil, {}, &testdata.RecordActivityTaskHeartbeatResponse} {
		assert.Equal(t, item, ToRecordActivityTaskHeartbeatByIDResponse(FromRecordActivityTaskHeartbeatByIDResponse(item)))
	}
}
func TestRecordActivityTaskHeartbeatRequest(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskHeartbeatRequest{nil, {}, &testdata.RecordActivityTaskHeartbeatRequest} {
		assert.Equal(t, item, ToRecordActivityTaskHeartbeatRequest(FromRecordActivityTaskHeartbeatRequest(item)))
	}
}
func TestRecordActivityTaskHeartbeatResponse(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskHeartbeatResponse{nil, {}, &testdata.RecordActivityTaskHeartbeatResponse} {
		assert.Equal(t, item, ToRecordActivityTaskHeartbeatResponse(FromRecordActivityTaskHeartbeatResponse(item)))
	}
}
func TestRecordMarkerDecisionAttributes(t *testing.T) {
	for _, item := range []*types.RecordMarkerDecisionAttributes{nil, {}, &testdata.RecordMarkerDecisionAttributes} {
		assert.Equal(t, item, ToRecordMarkerDecisionAttributes(FromRecordMarkerDecisionAttributes(item)))
	}
}

func TestRegisterDomainRequestFuzz(t *testing.T) {
	t.Run("round trip from internal", func(t *testing.T) {
		testutils.EnsureFuzzCoverage(t, []string{
			"nil", "empty", "filled",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			// Configure fuzzer to generate valid enum values and reasonable day ranges
			fuzzer := f.Funcs(
				func(e *types.ArchivalStatus, c fuzz.Continue) {
					*e = types.ArchivalStatus(c.Intn(2)) // 0-1 are valid values (Disabled=0, Enabled=1)
				},
				func(days *int32, c fuzz.Continue) {
					// Generate reasonable retention period values to avoid precision loss in conversion
					*days = int32(c.Intn(10000)) // 0-9999 days is reasonable range
				},
			).NilChance(0.3)

			var orig *types.RegisterDomainRequest
			fuzzer.Fuzz(&orig)
			out := ToRegisterDomainRequest(FromRegisterDomainRequest(orig))

			// Proto RegisterDomainRequest doesn't support EmitMetric field, it's always fixed on
			if orig != nil {
				expected := *orig                          // Copy the struct
				expected.EmitMetric = common.BoolPtr(true) // this is a legacy field which is always true. It's probably safe to remove
				assert.Equal(t, &expected, out, "RegisterDomainRequest did not survive round-tripping")
			} else {
				assert.Equal(t, orig, out, "RegisterDomainRequest did not survive round-tripping")
			}

			if orig == nil {
				return "nil"
			}
			if orig.Name == "" && orig.ActiveClusterName == "" && orig.ActiveClusters == nil {
				return "empty"
			}
			return "filled"
		})
	})
}
func TestRequestCancelActivityTaskDecisionAttributes(t *testing.T) {
	for _, item := range []*types.RequestCancelActivityTaskDecisionAttributes{nil, {}, &testdata.RequestCancelActivityTaskDecisionAttributes} {
		assert.Equal(t, item, ToRequestCancelActivityTaskDecisionAttributes(FromRequestCancelActivityTaskDecisionAttributes(item)))
	}
}
func TestRequestCancelActivityTaskFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.RequestCancelActivityTaskFailedEventAttributes{nil, {}, &testdata.RequestCancelActivityTaskFailedEventAttributes} {
		assert.Equal(t, item, ToRequestCancelActivityTaskFailedEventAttributes(FromRequestCancelActivityTaskFailedEventAttributes(item)))
	}
}
func TestRequestCancelExternalWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.RequestCancelExternalWorkflowExecutionDecisionAttributes{nil, {}, &testdata.RequestCancelExternalWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToRequestCancelExternalWorkflowExecutionDecisionAttributes(FromRequestCancelExternalWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestRequestCancelExternalWorkflowExecutionFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{nil, {}, &testdata.RequestCancelExternalWorkflowExecutionFailedEventAttributes} {
		assert.Equal(t, item, ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(item)))
	}
}
func TestRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t *testing.T) {
	for _, item := range []*types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{nil, {}, &testdata.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes} {
		assert.Equal(t, item, ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(item)))
	}
}
func TestRequestCancelWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.RequestCancelWorkflowExecutionRequest{nil, {}, &testdata.RequestCancelWorkflowExecutionRequest} {
		assert.Equal(t, item, ToRequestCancelWorkflowExecutionRequest(FromRequestCancelWorkflowExecutionRequest(item)))
	}
}
func TestResetPointInfo(t *testing.T) {
	for _, item := range []*types.ResetPointInfo{nil, {}, &testdata.ResetPointInfo} {
		assert.Equal(t, item, ToResetPointInfo(FromResetPointInfo(item)))
	}
}
func TestResetPoints(t *testing.T) {
	for _, item := range []*types.ResetPoints{nil, {}, &testdata.ResetPoints} {
		assert.Equal(t, item, ToResetPoints(FromResetPoints(item)))
	}
}
func TestResetStickyTaskListRequest(t *testing.T) {
	for _, item := range []*types.ResetStickyTaskListRequest{nil, {}, &testdata.ResetStickyTaskListRequest} {
		assert.Equal(t, item, ToResetStickyTaskListRequest(FromResetStickyTaskListRequest(item)))
	}
}
func TestResetWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.ResetWorkflowExecutionRequest{nil, {}, &testdata.ResetWorkflowExecutionRequest} {
		assert.Equal(t, item, ToResetWorkflowExecutionRequest(FromResetWorkflowExecutionRequest(item)))
	}
}
func TestResetWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.ResetWorkflowExecutionResponse{nil, {}, &testdata.ResetWorkflowExecutionResponse} {
		assert.Equal(t, item, ToResetWorkflowExecutionResponse(FromResetWorkflowExecutionResponse(item)))
	}
}
func TestRespondActivityTaskCanceledByIDRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskCanceledByIDRequest{nil, {}, &testdata.RespondActivityTaskCanceledByIDRequest} {
		assert.Equal(t, item, ToRespondActivityTaskCanceledByIDRequest(FromRespondActivityTaskCanceledByIDRequest(item)))
	}
}
func TestRespondActivityTaskCanceledRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskCanceledRequest{nil, {}, &testdata.RespondActivityTaskCanceledRequest} {
		assert.Equal(t, item, ToRespondActivityTaskCanceledRequest(FromRespondActivityTaskCanceledRequest(item)))
	}
}
func TestRespondActivityTaskCompletedByIDRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskCompletedByIDRequest{nil, {}, &testdata.RespondActivityTaskCompletedByIDRequest} {
		assert.Equal(t, item, ToRespondActivityTaskCompletedByIDRequest(FromRespondActivityTaskCompletedByIDRequest(item)))
	}
}
func TestRespondActivityTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskCompletedRequest{nil, {}, &testdata.RespondActivityTaskCompletedRequest} {
		assert.Equal(t, item, ToRespondActivityTaskCompletedRequest(FromRespondActivityTaskCompletedRequest(item)))
	}
}
func TestRespondActivityTaskFailedByIDRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskFailedByIDRequest{nil, {}, &testdata.RespondActivityTaskFailedByIDRequest} {
		assert.Equal(t, item, ToRespondActivityTaskFailedByIDRequest(FromRespondActivityTaskFailedByIDRequest(item)))
	}
}
func TestRespondActivityTaskFailedRequest(t *testing.T) {
	for _, item := range []*types.RespondActivityTaskFailedRequest{nil, {}, &testdata.RespondActivityTaskFailedRequest} {
		assert.Equal(t, item, ToRespondActivityTaskFailedRequest(FromRespondActivityTaskFailedRequest(item)))
	}
}
func TestRespondDecisionTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondDecisionTaskCompletedRequest{nil, {}, &testdata.RespondDecisionTaskCompletedRequest} {
		assert.Equal(t, item, ToRespondDecisionTaskCompletedRequest(FromRespondDecisionTaskCompletedRequest(item)))
	}
}
func TestRespondDecisionTaskCompletedResponse(t *testing.T) {
	for _, item := range []*types.RespondDecisionTaskCompletedResponse{nil, {}, &testdata.RespondDecisionTaskCompletedResponse} {
		assert.Equal(t, item, ToRespondDecisionTaskCompletedResponse(FromRespondDecisionTaskCompletedResponse(item)))
	}
}
func TestRespondDecisionTaskFailedRequest(t *testing.T) {
	for _, item := range []*types.RespondDecisionTaskFailedRequest{nil, {}, &testdata.RespondDecisionTaskFailedRequest} {
		assert.Equal(t, item, ToRespondDecisionTaskFailedRequest(FromRespondDecisionTaskFailedRequest(item)))
	}
}
func TestRespondQueryTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondQueryTaskCompletedRequest{nil, {}, &testdata.RespondQueryTaskCompletedRequest} {
		assert.Equal(t, item, ToRespondQueryTaskCompletedRequest(FromRespondQueryTaskCompletedRequest(item)))
	}
}
func TestRetryPolicy(t *testing.T) {
	for _, item := range []*types.RetryPolicy{nil, {}, &testdata.RetryPolicy} {
		assert.Equal(t, item, ToRetryPolicy(FromRetryPolicy(item)))
	}
}
func TestScanWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.ListWorkflowExecutionsRequest{nil, {}, &testdata.ListWorkflowExecutionsRequest} {
		assert.Equal(t, item, ToScanWorkflowExecutionsRequest(FromScanWorkflowExecutionsRequest(item)))
	}
}
func TestScanWorkflowExecutionsResponse(t *testing.T) {
	for _, item := range []*types.ListWorkflowExecutionsResponse{nil, {}, &testdata.ListWorkflowExecutionsResponse} {
		assert.Equal(t, item, ToScanWorkflowExecutionsResponse(FromScanWorkflowExecutionsResponse(item)))
	}
}
func TestScheduleActivityTaskDecisionAttributes(t *testing.T) {
	for _, item := range []*types.ScheduleActivityTaskDecisionAttributes{nil, {}, &testdata.ScheduleActivityTaskDecisionAttributes} {
		assert.Equal(t, item, ToScheduleActivityTaskDecisionAttributes(FromScheduleActivityTaskDecisionAttributes(item)))
	}
}
func TestSearchAttributes(t *testing.T) {
	for _, item := range []*types.SearchAttributes{nil, {}, &testdata.SearchAttributes} {
		assert.Equal(t, item, ToSearchAttributes(FromSearchAttributes(item)))
	}
}
func TestSignalExternalWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionDecisionAttributes{nil, {}, &testdata.SignalExternalWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionDecisionAttributes(FromSignalExternalWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestSignalExternalWorkflowExecutionFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionFailedEventAttributes{nil, {}, &testdata.SignalExternalWorkflowExecutionFailedEventAttributes} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionFailedEventAttributes(FromSignalExternalWorkflowExecutionFailedEventAttributes(item)))
	}
}
func TestSignalExternalWorkflowExecutionInitiatedEventAttributes(t *testing.T) {
	for _, item := range []*types.SignalExternalWorkflowExecutionInitiatedEventAttributes{nil, {}, &testdata.SignalExternalWorkflowExecutionInitiatedEventAttributes} {
		assert.Equal(t, item, ToSignalExternalWorkflowExecutionInitiatedEventAttributes(FromSignalExternalWorkflowExecutionInitiatedEventAttributes(item)))
	}
}
func TestSignalWithStartWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.SignalWithStartWorkflowExecutionRequest{nil, {}, &testdata.SignalWithStartWorkflowExecutionRequest} {
		assert.Equal(t, item, ToSignalWithStartWorkflowExecutionRequest(FromSignalWithStartWorkflowExecutionRequest(item)))
	}
}
func TestSignalWithStartWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionResponse{nil, {}, &testdata.StartWorkflowExecutionResponse} {
		assert.Equal(t, item, ToSignalWithStartWorkflowExecutionResponse(FromSignalWithStartWorkflowExecutionResponse(item)))
	}
}
func TestSignalWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.SignalWorkflowExecutionRequest{nil, {}, &testdata.SignalWorkflowExecutionRequest} {
		assert.Equal(t, item, ToSignalWorkflowExecutionRequest(FromSignalWorkflowExecutionRequest(item)))
	}
}
func TestStartChildWorkflowExecutionDecisionAttributes(t *testing.T) {
	for _, item := range []*types.StartChildWorkflowExecutionDecisionAttributes{nil, {}, &testdata.StartChildWorkflowExecutionDecisionAttributes} {
		assert.Equal(t, item, ToStartChildWorkflowExecutionDecisionAttributes(FromStartChildWorkflowExecutionDecisionAttributes(item)))
	}
}
func TestStartChildWorkflowExecutionFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.StartChildWorkflowExecutionFailedEventAttributes{nil, {}, &testdata.StartChildWorkflowExecutionFailedEventAttributes} {
		assert.Equal(t, item, ToStartChildWorkflowExecutionFailedEventAttributes(FromStartChildWorkflowExecutionFailedEventAttributes(item)))
	}
}
func TestStartChildWorkflowExecutionInitiatedEventAttributes(t *testing.T) {
	for _, item := range []*types.StartChildWorkflowExecutionInitiatedEventAttributes{nil, {}, &testdata.StartChildWorkflowExecutionInitiatedEventAttributes} {
		assert.Equal(t, item, ToStartChildWorkflowExecutionInitiatedEventAttributes(FromStartChildWorkflowExecutionInitiatedEventAttributes(item)))
	}
}
func TestStartTimeFilter(t *testing.T) {
	for _, item := range []*types.StartTimeFilter{nil, {}, &testdata.StartTimeFilter} {
		assert.Equal(t, item, ToStartTimeFilter(FromStartTimeFilter(item)))
	}
}
func TestStartTimerDecisionAttributes(t *testing.T) {
	for _, item := range []*types.StartTimerDecisionAttributes{nil, {}, &testdata.StartTimerDecisionAttributes} {
		assert.Equal(t, item, ToStartTimerDecisionAttributes(FromStartTimerDecisionAttributes(item)))
	}
}
func TestStartWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionRequest{nil, {}, &testdata.StartWorkflowExecutionRequest} {
		assert.Equal(t, item, ToStartWorkflowExecutionRequest(FromStartWorkflowExecutionRequest(item)))
	}
}
func TestStartWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionResponse{nil, {}, &testdata.StartWorkflowExecutionResponse} {
		assert.Equal(t, item, ToStartWorkflowExecutionResponse(FromStartWorkflowExecutionResponse(item)))
	}
}
func TestStartWorkflowExecutionAsyncRequest(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionAsyncRequest{nil, {}, &testdata.StartWorkflowExecutionAsyncRequest} {
		assert.Equal(t, item, ToStartWorkflowExecutionAsyncRequest(FromStartWorkflowExecutionAsyncRequest(item)))
	}
}
func TestStartWorkflowExecutionAsyncResponse(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionAsyncResponse{nil, {}, &testdata.StartWorkflowExecutionAsyncResponse} {
		assert.Equal(t, item, ToStartWorkflowExecutionAsyncResponse(FromStartWorkflowExecutionAsyncResponse(item)))
	}
}
func TestSignalWithStartWorkflowExecutionAsyncRequest(t *testing.T) {
	for _, item := range []*types.SignalWithStartWorkflowExecutionAsyncRequest{nil, {}, &testdata.SignalWithStartWorkflowExecutionAsyncRequest} {
		assert.Equal(t, item, ToSignalWithStartWorkflowExecutionAsyncRequest(FromSignalWithStartWorkflowExecutionAsyncRequest(item)))
	}
}
func TestSignalWithStartWorkflowExecutionAsyncResponse(t *testing.T) {
	for _, item := range []*types.SignalWithStartWorkflowExecutionAsyncResponse{nil, {}, &testdata.SignalWithStartWorkflowExecutionAsyncResponse} {
		assert.Equal(t, item, ToSignalWithStartWorkflowExecutionAsyncResponse(FromSignalWithStartWorkflowExecutionAsyncResponse(item)))
	}
}
func TestStatusFilter(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionCloseStatus{nil, &testdata.WorkflowExecutionCloseStatus} {
		assert.Equal(t, item, ToStatusFilter(FromStatusFilter(item)))
	}
}
func TestStickyExecutionAttributes(t *testing.T) {
	for _, item := range []*types.StickyExecutionAttributes{nil, {}, &testdata.StickyExecutionAttributes} {
		assert.Equal(t, item, ToStickyExecutionAttributes(FromStickyExecutionAttributes(item)))
	}
}
func TestSupportedClientVersions(t *testing.T) {
	for _, item := range []*types.SupportedClientVersions{nil, {}, &testdata.SupportedClientVersions} {
		assert.Equal(t, item, ToSupportedClientVersions(FromSupportedClientVersions(item)))
	}
}
func TestTaskIDBlock(t *testing.T) {
	for _, item := range []*types.TaskIDBlock{nil, {}, &testdata.TaskIDBlock} {
		assert.Equal(t, item, ToTaskIDBlock(FromTaskIDBlock(item)))
	}
}
func TestTaskList(t *testing.T) {
	for _, item := range []*types.TaskList{nil, {}, &testdata.TaskList} {
		assert.Equal(t, item, ToTaskList(FromTaskList(item)))
	}
}
func TestTaskListMetadata(t *testing.T) {
	for _, item := range []*types.TaskListMetadata{nil, {}, &testdata.TaskListMetadata} {
		assert.Equal(t, item, ToTaskListMetadata(FromTaskListMetadata(item)))
	}
}
func TestTaskListPartitionMetadata(t *testing.T) {
	for _, item := range []*types.TaskListPartitionMetadata{nil, {}, &testdata.TaskListPartitionMetadata} {
		assert.Equal(t, item, ToTaskListPartitionMetadata(FromTaskListPartitionMetadata(item)))
	}
}
func TestTaskListStatus(t *testing.T) {
	for _, item := range []*types.TaskListStatus{nil, {}, &testdata.TaskListStatus} {
		assert.Equal(t, item, ToTaskListStatus(FromTaskListStatus(item)))
	}
}
func TestTerminateWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.TerminateWorkflowExecutionRequest{nil, {}, &testdata.TerminateWorkflowExecutionRequest} {
		assert.Equal(t, item, ToTerminateWorkflowExecutionRequest(FromTerminateWorkflowExecutionRequest(item)))
	}
}
func TestTimerCanceledEventAttributes(t *testing.T) {
	for _, item := range []*types.TimerCanceledEventAttributes{nil, {}, &testdata.TimerCanceledEventAttributes} {
		assert.Equal(t, item, ToTimerCanceledEventAttributes(FromTimerCanceledEventAttributes(item)))
	}
}
func TestTimerFiredEventAttributes(t *testing.T) {
	for _, item := range []*types.TimerFiredEventAttributes{nil, {}, &testdata.TimerFiredEventAttributes} {
		assert.Equal(t, item, ToTimerFiredEventAttributes(FromTimerFiredEventAttributes(item)))
	}
}
func TestTimerStartedEventAttributes(t *testing.T) {
	for _, item := range []*types.TimerStartedEventAttributes{nil, {}, &testdata.TimerStartedEventAttributes} {
		assert.Equal(t, item, ToTimerStartedEventAttributes(FromTimerStartedEventAttributes(item)))
	}
}
func TestUpdateDomainRequest(t *testing.T) {
	for _, item := range []*types.UpdateDomainRequest{nil, {}, &testdata.UpdateDomainRequest} {
		assert.Equal(t, item, ToUpdateDomainRequest(FromUpdateDomainRequest(item)))
	}
}
func TestFailoverDomainRequest(t *testing.T) {
	// Test round-trip conversion for standard testdata
	for _, item := range []*types.FailoverDomainRequest{nil, {}, &testdata.FailoverDomainRequest, &testdata.FailoverDomainRequest_OnlyActiveClusters} {
		assert.Equal(t, item, ToFailoverDomainRequest(FromFailoverDomainRequest(item)))
	}

	// Test specific edge cases for proto3 empty string handling
	t.Run("empty DomainActiveClusterName should map to nil pointer", func(t *testing.T) {
		input := &apiv1.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: "",
			ActiveClusters:          nil,
		}
		expected := &types.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: nil,
			ActiveClusters:          nil,
		}
		result := ToFailoverDomainRequest(input)
		assert.Equal(t, expected, result)
		assert.Nil(t, result.DomainActiveClusterName,
			"DomainActiveClusterName should be nil when proto field is empty string, not pointer to empty string")
	})

	t.Run("non-empty DomainActiveClusterName should map to pointer", func(t *testing.T) {
		input := &apiv1.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: "cluster1",
			ActiveClusters:          nil,
		}
		expected := &types.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: common.StringPtr("cluster1"),
			ActiveClusters:          nil,
		}
		assert.Equal(t, expected, ToFailoverDomainRequest(input))
	})

	t.Run("with ActiveClusters and empty DomainActiveClusterName", func(t *testing.T) {
		input := &apiv1.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: "",
			ActiveClusters: &apiv1.ActiveClusters{
				ActiveClustersByClusterAttribute: map[string]*apiv1.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]*apiv1.ActiveClusterInfo{
							"london": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   1,
							},
						},
					},
				},
			},
		}
		expected := &types.FailoverDomainRequest{
			DomainName:              "test-domain",
			DomainActiveClusterName: nil,
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"location": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"london": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   1,
							},
						},
					},
				},
			},
		}
		result := ToFailoverDomainRequest(input)
		assert.Equal(t, expected, result)
		assert.Nil(t, result.DomainActiveClusterName,
			"DomainActiveClusterName should be nil when proto field is empty string")
	})
}
func TestUpdateDomainResponse(t *testing.T) {
	for _, item := range []*types.UpdateDomainResponse{nil, &testdata.UpdateDomainResponse} {
		assert.Equal(t, item, ToUpdateDomainResponse(FromUpdateDomainResponse(item)))
	}
}
func TestUpsertWorkflowSearchAttributesDecisionAttributes(t *testing.T) {
	for _, item := range []*types.UpsertWorkflowSearchAttributesDecisionAttributes{nil, {}, &testdata.UpsertWorkflowSearchAttributesDecisionAttributes} {
		assert.Equal(t, item, ToUpsertWorkflowSearchAttributesDecisionAttributes(FromUpsertWorkflowSearchAttributesDecisionAttributes(item)))
	}
}
func TestUpsertWorkflowSearchAttributesEventAttributes(t *testing.T) {
	for _, item := range []*types.UpsertWorkflowSearchAttributesEventAttributes{nil, {}, &testdata.UpsertWorkflowSearchAttributesEventAttributes} {
		assert.Equal(t, item, ToUpsertWorkflowSearchAttributesEventAttributes(FromUpsertWorkflowSearchAttributesEventAttributes(item)))
	}
}
func TestWorkerVersionInfo(t *testing.T) {
	for _, item := range []*types.WorkerVersionInfo{nil, {}, &testdata.WorkerVersionInfo} {
		assert.Equal(t, item, ToWorkerVersionInfo(FromWorkerVersionInfo(item)))
	}
}
func TestWorkflowExecution(t *testing.T) {
	for _, item := range []*types.WorkflowExecution{nil, {}, &testdata.WorkflowExecution} {
		assert.Equal(t, item, ToWorkflowExecution(FromWorkflowExecution(item)))
	}
	assert.Empty(t, ToWorkflowID(nil))
	assert.Empty(t, ToRunID(nil))
}
func TestExternalExecutionInfo(t *testing.T) {
	assert.Nil(t, FromExternalExecutionInfoFields(nil, nil))
	assert.Nil(t, ToExternalWorkflowExecution(nil))
	assert.Nil(t, ToExternalInitiatedID(nil))

	info := FromExternalExecutionInfoFields(nil, common.Int64Ptr(testdata.EventID1))
	assert.Nil(t, ToExternalWorkflowExecution(nil))
	assert.Equal(t, testdata.EventID1, *ToExternalInitiatedID(info))

	info = FromExternalExecutionInfoFields(&testdata.WorkflowExecution, nil)
	assert.Equal(t, testdata.WorkflowExecution, *ToExternalWorkflowExecution(info))
	assert.Equal(t, int64(0), *ToExternalInitiatedID(info))

	info = FromExternalExecutionInfoFields(&testdata.WorkflowExecution, common.Int64Ptr(testdata.EventID1))
	assert.Equal(t, testdata.WorkflowExecution, *ToExternalWorkflowExecution(info))
	assert.Equal(t, testdata.EventID1, *ToExternalInitiatedID(info))
}
func TestWorkflowExecutionCancelRequestedEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionCancelRequestedEventAttributes{nil, {}, &testdata.WorkflowExecutionCancelRequestedEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionCancelRequestedEventAttributes(FromWorkflowExecutionCancelRequestedEventAttributes(item)))
	}
}
func TestWorkflowExecutionCanceledEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionCanceledEventAttributes{nil, {}, &testdata.WorkflowExecutionCanceledEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionCanceledEventAttributes(FromWorkflowExecutionCanceledEventAttributes(item)))
	}
}
func TestWorkflowExecutionCompletedEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionCompletedEventAttributes{nil, {}, &testdata.WorkflowExecutionCompletedEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionCompletedEventAttributes(FromWorkflowExecutionCompletedEventAttributes(item)))
	}
}
func TestWorkflowExecutionConfiguration(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionConfiguration{nil, {}, &testdata.WorkflowExecutionConfiguration} {
		assert.Equal(t, item, ToWorkflowExecutionConfiguration(FromWorkflowExecutionConfiguration(item)))
	}
}
func TestWorkflowExecutionContinuedAsNewEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionContinuedAsNewEventAttributes{nil, {}, &testdata.WorkflowExecutionContinuedAsNewEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionContinuedAsNewEventAttributes(FromWorkflowExecutionContinuedAsNewEventAttributes(item)))
	}
}
func TestWorkflowExecutionFailedEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionFailedEventAttributes{nil, {}, &testdata.WorkflowExecutionFailedEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionFailedEventAttributes(FromWorkflowExecutionFailedEventAttributes(item)))
	}
}
func TestWorkflowExecutionFilter(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionFilter{nil, {}, &testdata.WorkflowExecutionFilter} {
		assert.Equal(t, item, ToWorkflowExecutionFilter(FromWorkflowExecutionFilter(item)))
	}
}
func TestParentExecutionInfo(t *testing.T) {
	for _, item := range []*types.ParentExecutionInfo{nil, {}, &testdata.ParentExecutionInfo} {
		assert.Equal(t, item, ToParentExecutionInfo(FromParentExecutionInfo(item)))
	}
}
func TestParentExecutionInfoFields(t *testing.T) {
	assert.Nil(t, FromParentExecutionInfoFields(nil, nil, nil, nil))
	info := FromParentExecutionInfoFields(nil, nil, testdata.ParentExecutionInfo.Execution, nil)
	assert.Equal(t, "", *ToParentDomainID(info))
	assert.Equal(t, "", *ToParentDomainName(info))
	assert.Equal(t, testdata.ParentExecutionInfo.Execution, ToParentWorkflowExecution(info))
	assert.Equal(t, int64(0), *ToParentInitiatedID(info))
	info = FromParentExecutionInfoFields(
		&testdata.ParentExecutionInfo.DomainUUID,
		&testdata.ParentExecutionInfo.Domain,
		testdata.ParentExecutionInfo.Execution,
		&testdata.ParentExecutionInfo.InitiatedID)
	assert.Equal(t, testdata.ParentExecutionInfo.DomainUUID, *ToParentDomainID(info))
	assert.Equal(t, testdata.ParentExecutionInfo.Domain, *ToParentDomainName(info))
	assert.Equal(t, testdata.ParentExecutionInfo.Execution, ToParentWorkflowExecution(info))
	assert.Equal(t, testdata.ParentExecutionInfo.InitiatedID, *ToParentInitiatedID(info))
}
func TestWorkflowExecutionInfo(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionInfo{nil, {}, &testdata.WorkflowExecutionInfo, &testdata.CronWorkflowExecutionInfo, &testdata.WorkflowExecutionInfoEphemeral} {
		assert.Equal(t, item, ToWorkflowExecutionInfo(FromWorkflowExecutionInfo(item)))
	}
}

func TestWorkflowExecutionInfo_MigrateTaskList(t *testing.T) {
	tlName := "foo"
	otherName := "bar"
	cases := []struct {
		name string
		in   *apiv1.WorkflowExecutionInfo
		out  *types.WorkflowExecutionInfo
	}{
		{
			name: "nil",
			in:   &apiv1.WorkflowExecutionInfo{},
			out: &types.WorkflowExecutionInfo{
				TaskList: nil,
			},
		},
		{
			name: "name only",
			in: &apiv1.WorkflowExecutionInfo{
				TaskList: tlName,
			},
			out: &types.WorkflowExecutionInfo{
				TaskList: &types.TaskList{Name: tlName, Kind: types.TaskListKindNormal.Ptr()},
			},
		},
		{
			name: "tl only",
			in: &apiv1.WorkflowExecutionInfo{
				TaskListInfo: &apiv1.TaskList{Name: tlName, Kind: apiv1.TaskListKind_TASK_LIST_KIND_NORMAL},
			},
			out: &types.WorkflowExecutionInfo{
				TaskList: &types.TaskList{Name: tlName, Kind: types.TaskListKindNormal.Ptr()},
			},
		},
		{
			name: "both",
			in: &apiv1.WorkflowExecutionInfo{
				TaskList:     otherName,
				TaskListInfo: &apiv1.TaskList{Name: tlName, Kind: apiv1.TaskListKind_TASK_LIST_KIND_NORMAL},
			},
			out: &types.WorkflowExecutionInfo{
				TaskList: &types.TaskList{Name: tlName, Kind: types.TaskListKindNormal.Ptr()},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.out, ToWorkflowExecutionInfo(tc.in))
		})
	}
}

func TestWorkflowExecutionSignaledEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionSignaledEventAttributes{nil, {}, &testdata.WorkflowExecutionSignaledEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionSignaledEventAttributes(FromWorkflowExecutionSignaledEventAttributes(item)))
	}
}
func TestWorkflowExecutionStartedEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionStartedEventAttributes{nil, {}, &testdata.WorkflowExecutionStartedEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionStartedEventAttributes(FromWorkflowExecutionStartedEventAttributes(item)))
	}
}
func TestWorkflowExecutionTerminatedEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionTerminatedEventAttributes{nil, {}, &testdata.WorkflowExecutionTerminatedEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionTerminatedEventAttributes(FromWorkflowExecutionTerminatedEventAttributes(item)))
	}
}
func TestWorkflowExecutionTimedOutEventAttributes(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionTimedOutEventAttributes{nil, {}, &testdata.WorkflowExecutionTimedOutEventAttributes} {
		assert.Equal(t, item, ToWorkflowExecutionTimedOutEventAttributes(FromWorkflowExecutionTimedOutEventAttributes(item)))
	}
}
func TestWorkflowQuery(t *testing.T) {
	for _, item := range []*types.WorkflowQuery{nil, {}, &testdata.WorkflowQuery} {
		assert.Equal(t, item, ToWorkflowQuery(FromWorkflowQuery(item)))
	}
}
func TestWorkflowQueryResult(t *testing.T) {
	for _, item := range []*types.WorkflowQueryResult{nil, {}, &testdata.WorkflowQueryResult} {
		assert.Equal(t, item, ToWorkflowQueryResult(FromWorkflowQueryResult(item)))
	}
}
func TestWorkflowType(t *testing.T) {
	for _, item := range []*types.WorkflowType{nil, {}, &testdata.WorkflowType} {
		assert.Equal(t, item, ToWorkflowType(FromWorkflowType(item)))
	}
}
func TestWorkflowTypeFilter(t *testing.T) {
	for _, item := range []*types.WorkflowTypeFilter{nil, {}, &testdata.WorkflowTypeFilter} {
		assert.Equal(t, item, ToWorkflowTypeFilter(FromWorkflowTypeFilter(item)))
	}
}
func TestDataBlobArray(t *testing.T) {
	for _, item := range [][]*types.DataBlob{nil, {}, testdata.DataBlobArray} {
		assert.Equal(t, item, ToDataBlobArray(FromDataBlobArray(item)))
	}
}
func TestHistoryEventArray(t *testing.T) {
	for _, item := range [][]*types.HistoryEvent{{}, testdata.HistoryEventArray} {
		assert.Equal(t, item, ToHistoryEventArray(FromHistoryEventArray(item)))
	}
}
func TestTaskListPartitionMetadataArray(t *testing.T) {
	for _, item := range [][]*types.TaskListPartitionMetadata{nil, {}, testdata.TaskListPartitionMetadataArray} {
		assert.Equal(t, item, ToTaskListPartitionMetadataArray(FromTaskListPartitionMetadataArray(item)))
	}
}
func TestDecisionArray(t *testing.T) {
	for _, item := range [][]*types.Decision{nil, {}, testdata.DecisionArray} {
		assert.Equal(t, item, ToDecisionArray(FromDecisionArray(item)))
	}
}
func TestPollerInfoArray(t *testing.T) {
	for _, item := range [][]*types.PollerInfo{nil, {}, testdata.PollerInfoArray} {
		assert.Equal(t, item, ToPollerInfoArray(FromPollerInfoArray(item)))
	}
}
func TestPendingChildExecutionInfoArray(t *testing.T) {
	for _, item := range [][]*types.PendingChildExecutionInfo{nil, {}, testdata.PendingChildExecutionInfoArray} {
		assert.Equal(t, item, ToPendingChildExecutionInfoArray(FromPendingChildExecutionInfoArray(item)))
	}
}
func TestWorkflowExecutionInfoArray(t *testing.T) {
	for _, item := range [][]*types.WorkflowExecutionInfo{nil, {}, testdata.WorkflowExecutionInfoArray} {
		assert.Equal(t, item, ToWorkflowExecutionInfoArray(FromWorkflowExecutionInfoArray(item)))
	}
}
func TestDescribeDomainResponseArray(t *testing.T) {
	for _, item := range [][]*types.DescribeDomainResponse{nil, {}, testdata.DescribeDomainResponseArray} {
		assert.Equal(t, item, ToDescribeDomainResponseArray(FromDescribeDomainResponseArray(item)))
	}
}
func TestResetPointInfoArray(t *testing.T) {
	for _, item := range [][]*types.ResetPointInfo{nil, {}, testdata.ResetPointInfoArray} {
		assert.Equal(t, item, ToResetPointInfoArray(FromResetPointInfoArray(item)))
	}
}
func TestPendingActivityInfoArray(t *testing.T) {
	for _, item := range [][]*types.PendingActivityInfo{nil, {}, testdata.PendingActivityInfoArray} {
		assert.Equal(t, item, ToPendingActivityInfoArray(FromPendingActivityInfoArray(item)))
	}
}
func TestClusterReplicationConfigurationArray(t *testing.T) {
	for _, item := range [][]*types.ClusterReplicationConfiguration{nil, {}, testdata.ClusterReplicationConfigurationArray} {
		assert.Equal(t, item, ToClusterReplicationConfigurationArray(FromClusterReplicationConfigurationArray(item)))
	}
}
func TestActivityLocalDispatchInfoMap(t *testing.T) {
	for _, item := range []map[string]*types.ActivityLocalDispatchInfo{nil, {}, testdata.ActivityLocalDispatchInfoMap} {
		assert.Equal(t, item, ToActivityLocalDispatchInfoMap(FromActivityLocalDispatchInfoMap(item)))
	}
}
func TestBadBinaryInfoMap(t *testing.T) {
	for _, item := range []map[string]*types.BadBinaryInfo{nil, {}, testdata.BadBinaryInfoMap} {
		assert.Equal(t, item, ToBadBinaryInfoMap(FromBadBinaryInfoMap(item)))
	}
}
func TestIndexedValueTypeMap(t *testing.T) {
	for _, item := range []map[string]types.IndexedValueType{nil, {}, testdata.IndexedValueTypeMap} {
		assert.Equal(t, item, ToIndexedValueTypeMap(FromIndexedValueTypeMap(item)))
	}
}
func TestWorkflowQueryMap(t *testing.T) {
	for _, item := range []map[string]*types.WorkflowQuery{nil, {}, testdata.WorkflowQueryMap} {
		assert.Equal(t, item, ToWorkflowQueryMap(FromWorkflowQueryMap(item)))
	}
}
func TestWorkflowQueryResultMap(t *testing.T) {
	for _, item := range []map[string]*types.WorkflowQueryResult{nil, {}, testdata.WorkflowQueryResultMap} {
		assert.Equal(t, item, ToWorkflowQueryResultMap(FromWorkflowQueryResultMap(item)))
	}
}
func TestPayload(t *testing.T) {
	for _, item := range [][]byte{nil, {}, testdata.Payload1} {
		assert.Equal(t, item, ToPayload(FromPayload(item)))
	}

	assert.Equal(t, []byte{}, ToPayload(&apiv1.Payload{
		Data: nil,
	}))
}
func TestPayloadMap(t *testing.T) {
	for _, item := range []map[string][]byte{nil, {}, testdata.PayloadMap} {
		assert.Equal(t, item, ToPayloadMap(FromPayloadMap(item)))
	}
}
func TestFailure(t *testing.T) {
	assert.Nil(t, FromFailure(nil, nil))
	assert.Nil(t, ToFailureReason(nil))
	assert.Nil(t, ToFailureDetails(nil))
	failure := FromFailure(&testdata.FailureReason, testdata.FailureDetails)
	assert.Equal(t, testdata.FailureReason, *ToFailureReason(failure))
	assert.Equal(t, testdata.FailureDetails, ToFailureDetails(failure))
}
func TestHistoryEvent(t *testing.T) {
	for _, item := range []*types.HistoryEvent{
		nil,
		&testdata.HistoryEvent_WorkflowExecutionStarted,
		&testdata.HistoryEvent_WorkflowExecutionCompleted,
		&testdata.HistoryEvent_WorkflowExecutionFailed,
		&testdata.HistoryEvent_WorkflowExecutionTimedOut,
		&testdata.HistoryEvent_DecisionTaskScheduled,
		&testdata.HistoryEvent_DecisionTaskStarted,
		&testdata.HistoryEvent_DecisionTaskCompleted,
		&testdata.HistoryEvent_DecisionTaskTimedOut,
		&testdata.HistoryEvent_DecisionTaskFailed,
		&testdata.HistoryEvent_ActivityTaskScheduled,
		&testdata.HistoryEvent_ActivityTaskStarted,
		&testdata.HistoryEvent_ActivityTaskCompleted,
		&testdata.HistoryEvent_ActivityTaskFailed,
		&testdata.HistoryEvent_ActivityTaskTimedOut,
		&testdata.HistoryEvent_ActivityTaskCancelRequested,
		&testdata.HistoryEvent_RequestCancelActivityTaskFailed,
		&testdata.HistoryEvent_ActivityTaskCanceled,
		&testdata.HistoryEvent_TimerStarted,
		&testdata.HistoryEvent_TimerFired,
		&testdata.HistoryEvent_CancelTimerFailed,
		&testdata.HistoryEvent_TimerCanceled,
		&testdata.HistoryEvent_WorkflowExecutionCancelRequested,
		&testdata.HistoryEvent_WorkflowExecutionCanceled,
		&testdata.HistoryEvent_RequestCancelExternalWorkflowExecutionInitiated,
		&testdata.HistoryEvent_RequestCancelExternalWorkflowExecutionFailed,
		&testdata.HistoryEvent_ExternalWorkflowExecutionCancelRequested,
		&testdata.HistoryEvent_MarkerRecorded,
		&testdata.HistoryEvent_WorkflowExecutionSignaled,
		&testdata.HistoryEvent_WorkflowExecutionTerminated,
		&testdata.HistoryEvent_WorkflowExecutionContinuedAsNew,
		&testdata.HistoryEvent_StartChildWorkflowExecutionInitiated,
		&testdata.HistoryEvent_StartChildWorkflowExecutionFailed,
		&testdata.HistoryEvent_ChildWorkflowExecutionStarted,
		&testdata.HistoryEvent_ChildWorkflowExecutionCompleted,
		&testdata.HistoryEvent_ChildWorkflowExecutionFailed,
		&testdata.HistoryEvent_ChildWorkflowExecutionCanceled,
		&testdata.HistoryEvent_ChildWorkflowExecutionTimedOut,
		&testdata.HistoryEvent_ChildWorkflowExecutionTerminated,
		&testdata.HistoryEvent_SignalExternalWorkflowExecutionInitiated,
		&testdata.HistoryEvent_SignalExternalWorkflowExecutionFailed,
		&testdata.HistoryEvent_ExternalWorkflowExecutionSignaled,
		&testdata.HistoryEvent_UpsertWorkflowSearchAttributes,
	} {
		assert.Equal(t, item, ToHistoryEvent(FromHistoryEvent(item)))
	}
	assert.Panics(t, func() { FromHistoryEvent(&types.HistoryEvent{}) })
}
func TestDecision(t *testing.T) {
	for _, item := range []*types.Decision{
		nil,
		&testdata.Decision_CancelTimer,
		&testdata.Decision_CancelWorkflowExecution,
		&testdata.Decision_CompleteWorkflowExecution,
		&testdata.Decision_ContinueAsNewWorkflowExecution,
		&testdata.Decision_FailWorkflowExecution,
		&testdata.Decision_RecordMarker,
		&testdata.Decision_RequestCancelActivityTask,
		&testdata.Decision_RequestCancelExternalWorkflowExecution,
		&testdata.Decision_ScheduleActivityTask,
		&testdata.Decision_SignalExternalWorkflowExecution,
		&testdata.Decision_StartChildWorkflowExecution,
		&testdata.Decision_StartTimer,
		&testdata.Decision_UpsertWorkflowSearchAttributes,
	} {
		assert.Equal(t, item, ToDecision(FromDecision(item)))
	}
	assert.Panics(t, func() { FromDecision(&types.Decision{}) })
}
func TestListClosedWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.ListClosedWorkflowExecutionsRequest{
		nil,
		{},
		&testdata.ListClosedWorkflowExecutionsRequest_ExecutionFilter,
		&testdata.ListClosedWorkflowExecutionsRequest_StatusFilter,
		&testdata.ListClosedWorkflowExecutionsRequest_TypeFilter,
	} {
		assert.Equal(t, item, ToListClosedWorkflowExecutionsRequest(FromListClosedWorkflowExecutionsRequest(item)))
	}
}

func TestListOpenWorkflowExecutionsRequest(t *testing.T) {
	for _, item := range []*types.ListOpenWorkflowExecutionsRequest{
		nil,
		{},
		&testdata.ListOpenWorkflowExecutionsRequest_ExecutionFilter,
		&testdata.ListOpenWorkflowExecutionsRequest_TypeFilter,
	} {
		assert.Equal(t, item, ToListOpenWorkflowExecutionsRequest(FromListOpenWorkflowExecutionsRequest(item)))
	}
}

func TestGetTaskListsByDomainResponse(t *testing.T) {
	for _, item := range []*types.GetTaskListsByDomainResponse{nil, {}, &testdata.GetTaskListsByDomainResponse} {
		assert.Equal(t, item, ToMatchingGetTaskListsByDomainResponse(FromMatchingGetTaskListsByDomainResponse(item)))
	}
}

func TestFailoverInfo(t *testing.T) {
	for _, item := range []*types.FailoverInfo{
		nil,
		{},
		&testdata.FailoverInfo,
	} {
		assert.Equal(t, item, ToFailoverInfo(FromFailoverInfo(item)))
	}
}

func TestDescribeTaskListResponseMap(t *testing.T) {
	for _, item := range []map[string]*types.DescribeTaskListResponse{nil, {}, testdata.DescribeTaskListResponseMap} {
		assert.Equal(t, item, ToDescribeTaskListResponseMap(FromDescribeTaskListResponseMap(item)))
	}
}

func TestAPITaskListPartitionConfig(t *testing.T) {
	for _, item := range []*types.TaskListPartitionConfig{nil, {}, &testdata.TaskListPartitionConfig} {
		assert.Equal(t, item, ToAPITaskListPartitionConfig(FromAPITaskListPartitionConfig(item)))
	}
}

func TestToAPITaskListPartitionConfig(t *testing.T) {
	cases := []struct {
		name     string
		config   *apiv1.TaskListPartitionConfig
		expected *types.TaskListPartitionConfig
	}{
		{
			name: "happy path",
			config: &apiv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 2,
				ReadPartitions: map[int32]*apiv1.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int32]*apiv1.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
		},
		{
			name: "numbers only",
			config: &apiv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 2,
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {},
					1: {},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
					1: {},
				},
			},
		},
		{
			name: "number mismatch",
			config: &apiv1.TaskListPartitionConfig{
				Version:            1,
				NumReadPartitions:  2,
				NumWritePartitions: 1,
				ReadPartitions: map[int32]*apiv1.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int32]*apiv1.TaskListPartition{
					0: {IsolationGroups: []string{"baz"}},
					1: {IsolationGroups: []string{"bar"}},
				},
			},
			expected: &types.TaskListPartitionConfig{
				Version: 1,
				ReadPartitions: map[int]*types.TaskListPartition{
					0: {IsolationGroups: []string{"foo"}},
					1: {IsolationGroups: []string{"bar"}},
				},
				WritePartitions: map[int]*types.TaskListPartition{
					0: {},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := ToAPITaskListPartitionConfig(tc.config)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestActiveClustersConversion(t *testing.T) {
	testCases := []*types.ActiveClusters{
		nil,
		{},
		{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"us-west-1": {
							ActiveClusterName: "cluster1",
							FailoverVersion:   1,
						},
						"us-east-1": {
							ActiveClusterName: "cluster2",
							FailoverVersion:   2,
						},
					},
				},
			},
		},
		{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"us-west-1": {
							ActiveClusterName: "cluster1",
							FailoverVersion:   1,
						},
						"us-east-1": {
							ActiveClusterName: "cluster2",
							FailoverVersion:   2,
						},
					},
				},
				"datacenter": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"dc1": {
							ActiveClusterName: "cluster1",
							FailoverVersion:   10,
						},
					},
				},
			},
		},
		{
			AttributeScopes: map[string]types.ClusterAttributeScope{
				"region": {
					ClusterAttributes: map[string]types.ActiveClusterInfo{
						"us-west-1": {
							ActiveClusterName: "cluster1",
							FailoverVersion:   1,
						},
					},
				},
			},
		},
	}

	for _, original := range testCases {
		protoObj := FromActiveClusters(original)
		roundTripObj := ToActiveClusters(protoObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestClusterAttribute(t *testing.T) {
	for _, item := range []*types.ClusterAttribute{nil, {}, &testdata.ClusterAttribute} {
		assert.Equal(t, item, ToClusterAttribute(FromClusterAttribute(item)))
	}
}

func TestActiveClusterSelectionPolicy(t *testing.T) {
	for _, item := range []*types.ActiveClusterSelectionPolicy{
		nil,
		{},
		&testdata.ActiveClusterSelectionPolicyWithClusterAttribute,
		&testdata.ActiveClusterSelectionPolicyRegionSticky,
		&testdata.ActiveClusterSelectionPolicyExternalEntity,
	} {
		assert.Equal(t, item, ToActiveClusterSelectionPolicy(FromActiveClusterSelectionPolicy(item)))
	}
}

func TestClusterAttributeScopeConversion(t *testing.T) {
	testCases := []*types.ClusterAttributeScope{
		nil,
		{},
		{
			ClusterAttributes: map[string]types.ActiveClusterInfo{
				"us-west-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   1,
				},
				"us-east-1": {
					ActiveClusterName: "cluster2",
					FailoverVersion:   2,
				},
			},
		},
	}

	for _, original := range testCases {
		protoObj := FromClusterAttributeScope(original)
		roundTripObj := ToClusterAttributeScope(protoObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPaginationOptions(t *testing.T) {
	for _, item := range []*types.PaginationOptions{nil, {}, &testdata.PaginationOptions} {
		assert.Equal(t, item, ToPaginationOptions(FromPaginationOptions(item)))
	}
}

func TestListFailoverHistoryRequestFilters(t *testing.T) {
	for _, item := range []*types.ListFailoverHistoryRequestFilters{nil, {}, &testdata.ListFailoverHistoryRequestFilters} {
		assert.Equal(t, item, ToListFailoverHistoryRequestFilters(FromListFailoverHistoryRequestFilters(item)))
	}
}

func TestListFailoverHistoryRequest(t *testing.T) {
	for _, item := range []*types.ListFailoverHistoryRequest{nil, {}, &testdata.ListFailoverHistoryRequest} {
		assert.Equal(t, item, ToListFailoverHistoryRequest(FromListFailoverHistoryRequest(item)))
	}
}

func TestListFailoverHistoryResponse(t *testing.T) {
	for _, item := range []*types.ListFailoverHistoryResponse{nil, {}, &testdata.ListFailoverHistoryResponse} {
		assert.Equal(t, item, ToListFailoverHistoryResponse(FromListFailoverHistoryResponse(item)))
	}
}

func TestFailoverEvent(t *testing.T) {
	for _, item := range []*types.FailoverEvent{nil, {}, &testdata.FailoverEvent} {
		assert.Equal(t, item, ToFailoverEvent(FromFailoverEvent(item)))
	}
}

func TestFailoverEventArray(t *testing.T) {
	testCases := [][]*types.FailoverEvent{
		nil,
		{},
		{nil},
		{&testdata.FailoverEvent},
		{&testdata.FailoverEvent, nil, &testdata.FailoverEvent},
	}
	for _, item := range testCases {
		assert.Equal(t, item, ToFailoverEventArray(FromFailoverEventArray(item)))
	}
}

func TestListFailoverHistoryResponseMapping(t *testing.T) {
	fuzzer := testdatagen.New(t,
		func(v *types.FailoverEvent, c fuzz.Continue) {
			c.Fuzz(v)
			// Don't allow empty strings for ID - use nil or a non-empty string
			if v.ID != nil && *v.ID == "" {
				v.ID = nil
			}
		})
	for i := 0; i < 100; i++ {
		var response types.ListFailoverHistoryResponse
		fuzzer.Fuzz(&response)
		protoResponse := FromListFailoverHistoryResponse(&response)
		assert.Equal(t, &response, ToListFailoverHistoryResponse(protoResponse))
	}
}

func TestClusterFailover(t *testing.T) {
	for _, item := range []*types.ClusterFailover{nil, {}, &testdata.ClusterFailover} {
		assert.Equal(t, item, ToClusterFailover(FromClusterFailover(item)))
	}
}

func TestClusterFailoverArray(t *testing.T) {
	testCases := [][]*types.ClusterFailover{
		nil,
		{},
		{nil},
		{&testdata.ClusterFailover},
		{&testdata.ClusterFailover, nil, &testdata.ClusterFailover},
	}
	for _, item := range testCases {
		assert.Equal(t, item, ToClusterFailoverArray(FromClusterFailoverArray(item)))
	}
}

func TestActiveClusterInfo(t *testing.T) {
	for _, item := range []*types.ActiveClusterInfo{nil, {}, &testdata.ActiveClusterInfo1, &testdata.ActiveClusterInfo2} {
		assert.Equal(t, item, ToActiveClusterInfo(FromActiveClusterInfo(item)))
	}
}

func TestFailoverType(t *testing.T) {
	testCases := []struct {
		name     string
		input    *types.FailoverType
		expected apiv1.FailoverType
	}{
		{
			name:     "nil",
			input:    nil,
			expected: apiv1.FailoverType_FAILOVER_TYPE_INVALID,
		},
		{
			name:     "force",
			input:    types.FailoverTypeForce.Ptr(),
			expected: apiv1.FailoverType_FAILOVER_TYPE_FORCE,
		},
		{
			name:     "graceful",
			input:    types.FailoverTypeGraceful.Ptr(),
			expected: apiv1.FailoverType_FAILOVER_TYPE_GRACEFUL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FromFailoverType(tc.input)
			assert.Equal(t, tc.expected, result)

			// Test round-trip
			if tc.input != nil {
				roundTrip := ToFailoverType(result)
				assert.Equal(t, tc.input, roundTrip)
			}
		})
	}

	// Test ToFailoverType for all enum values
	assert.Equal(t, types.FailoverTypeForce.Ptr(), ToFailoverType(apiv1.FailoverType_FAILOVER_TYPE_FORCE))
	assert.Equal(t, types.FailoverTypeGraceful.Ptr(), ToFailoverType(apiv1.FailoverType_FAILOVER_TYPE_GRACEFUL))
	assert.Nil(t, ToFailoverType(apiv1.FailoverType_FAILOVER_TYPE_INVALID))
	assert.Nil(t, ToFailoverType(apiv1.FailoverType(999))) // Unknown value
}

func QueryConsistencyLevelFuzzer(e *types.QueryConsistencyLevel, c fuzz.Continue) {
	*e = types.QueryConsistencyLevel(c.Intn(2)) // 0-1: Eventual, Strong
}

func SignalExternalWorkflowExecutionFailedCauseFuzzer(e *types.SignalExternalWorkflowExecutionFailedCause, c fuzz.Continue) {
	*e = types.SignalExternalWorkflowExecutionFailedCause(c.Intn(2)) // 0-1: UnknownExternalWorkflowExecution, WorkflowAlreadyCompleted
}

func WorkflowIDReusePolicyFuzzer(e *types.WorkflowIDReusePolicy, c fuzz.Continue) {
	*e = types.WorkflowIDReusePolicy(c.Intn(4)) // 0-3: AllowDuplicateFailedOnly, AllowDuplicate, RejectDuplicate, TerminateIfRunning
}

func ActiveClusterSelectionStrategyFuzzer(e *types.ActiveClusterSelectionStrategy, c fuzz.Continue) {
	*e = types.ActiveClusterSelectionStrategy(c.Intn(2)) // 0-1: RegionSticky, ExternalEntity
}

func CronOverlapPolicyFuzzer(e *types.CronOverlapPolicy, c fuzz.Continue) {
	*e = types.CronOverlapPolicy(c.Intn(2)) // 0-1: Skipped, BufferOne
}

func DecisionTypeFuzzer(e *types.DecisionType, c fuzz.Continue) {
	*e = types.DecisionType(c.Intn(13))
}

func EventTypeFuzzer(e *types.EventType, c fuzz.Continue) {
	*e = types.EventType(c.Intn(42))
}

func TimeoutTypeFuzzer(e *types.TimeoutType, c fuzz.Continue) {
	*e = types.TimeoutType(c.Intn(4)) // 0-3: StartToClose, ScheduleToStart, ScheduleToClose, Heartbeat
}

func TaskListKindFuzzer(e *types.TaskListKind, c fuzz.Continue) {
	*e = types.TaskListKind(c.Intn(3)) // 0-2: Normal, Sticky, Ephemeral
}

func FailoverTypeFuzzer(e *types.FailoverType, c fuzz.Continue) {
	*e = types.FailoverType(c.Intn(2) + 1) // 1-2: Force, Graceful (skip 0=Invalid which maps to nil)
}

func ArchivalStatusFuzzer(e *types.ArchivalStatus, c fuzz.Continue) {
	*e = types.ArchivalStatus(c.Intn(2)) // 0-1: Disabled, Enabled
}

func ContinueAsNewInitiatorFuzzer(e *types.ContinueAsNewInitiator, c fuzz.Continue) {
	*e = types.ContinueAsNewInitiator(c.Intn(3)) // 0-2: Decider, RetryPolicy, CronSchedule
}

func CancelExternalWorkflowExecutionFailedCauseFuzzer(e *types.CancelExternalWorkflowExecutionFailedCause, c fuzz.Continue) {
	*e = types.CancelExternalWorkflowExecutionFailedCause(c.Intn(2)) // 0-1: UnknownExternalWorkflowExecution, WorkflowAlreadyCompleted
}

func ChildWorkflowExecutionFailedCauseFuzzer(e *types.ChildWorkflowExecutionFailedCause, c fuzz.Continue) {
	*e = types.ChildWorkflowExecutionFailedCause(c.Intn(2)) // 0-1: WorkflowAlreadyRunning, DeprecatedDomain
}

func WorkflowExecutionCloseStatusFuzzer(e *types.WorkflowExecutionCloseStatus, c fuzz.Continue) {
	*e = types.WorkflowExecutionCloseStatus(c.Intn(6)) // 0-5: Completed, Failed, Canceled, Terminated, ContinuedAsNew, TimedOut
}

func ParentClosePolicyFuzzer(e *types.ParentClosePolicy, c fuzz.Continue) {
	*e = types.ParentClosePolicy(c.Intn(3)) // 0-2: Abandon, RequestCancel, Terminate
}

func DecisionTaskTimedOutCauseFuzzer(e *types.DecisionTaskTimedOutCause, c fuzz.Continue) {
	*e = types.DecisionTaskTimedOutCause(c.Intn(2)) // 0-1: Timeout, Reset
}

func DecisionTaskFailedCauseFuzzer(e *types.DecisionTaskFailedCause, c fuzz.Continue) {
	*e = types.DecisionTaskFailedCause(c.Intn(23)) // 0-22: All DecisionTaskFailedCause values
}

func QueryRejectConditionFuzzer(e *types.QueryRejectCondition, c fuzz.Continue) {
	*e = types.QueryRejectCondition(c.Intn(2)) // 0-1: NotOpen, NotCompletedCleanly
}

func PendingActivityStateFuzzer(e *types.PendingActivityState, c fuzz.Continue) {
	*e = types.PendingActivityState(c.Intn(3)) // 0-2: Scheduled, Started, CancelRequested
}

func HistoryEventFilterTypeFuzzer(e *types.HistoryEventFilterType, c fuzz.Continue) {
	*e = types.HistoryEventFilterType(c.Intn(2)) // 0-1: AllEvent, CloseEvent
}

func IndexedValueTypeFuzzer(e *types.IndexedValueType, c fuzz.Continue) {
	*e = types.IndexedValueType(c.Intn(6)) // 0-5: String, Keyword, Int, Double, Bool, Datetime
}

func CompletedTypeFuzzer(e *types.QueryTaskCompletedType, c fuzz.Continue) {
	*e = types.QueryTaskCompletedType(c.Intn(2)) // 0-1: Completed, Failed
}

func ActiveClusterSelectionPolicyFuzzerClearAttribute(p *types.ActiveClusterSelectionPolicy, c fuzz.Continue) {
	// ActiveClusterSelectionPolicy requires string fields to match strategy
	// When strategy is nil, all string fields must be empty (mapper uses ClusterAttribute)
	// When strategy is set, only the relevant string fields should be set
	c.Fuzz(&p.ActiveClusterSelectionStrategy)
	if p.ActiveClusterSelectionStrategy == nil {
		p.StickyRegion = ""
		p.ExternalEntityType = ""
		p.ExternalEntityKey = ""
	} else {
		switch *p.ActiveClusterSelectionStrategy {
		case types.ActiveClusterSelectionStrategyRegionSticky:
			c.Fuzz(&p.StickyRegion)
			p.ExternalEntityType = ""
			p.ExternalEntityKey = ""
		case types.ActiveClusterSelectionStrategyExternalEntity:
			c.Fuzz(&p.ExternalEntityType)
			c.Fuzz(&p.ExternalEntityKey)
			p.StickyRegion = ""
		}
	}
	// ClusterAttribute is always cleared (mapper uses strategy+strings)
	p.ClusterAttribute = nil
}

func ActiveClusterSelectionPolicyFuzzerWithAttribute(p *types.ActiveClusterSelectionPolicy, c fuzz.Continue) {
	// ActiveClusterSelectionPolicy requires string fields to match strategy
	// When strategy is nil, all string fields must be empty (mapper uses ClusterAttribute)
	// When strategy is set, only the relevant string fields should be set
	c.Fuzz(&p.ActiveClusterSelectionStrategy)
	if p.ActiveClusterSelectionStrategy == nil {
		p.StickyRegion = ""
		p.ExternalEntityType = ""
		p.ExternalEntityKey = ""
	} else {
		switch *p.ActiveClusterSelectionStrategy {
		case types.ActiveClusterSelectionStrategyRegionSticky:
			c.Fuzz(&p.StickyRegion)
			p.ExternalEntityType = ""
			p.ExternalEntityKey = ""
		case types.ActiveClusterSelectionStrategyExternalEntity:
			p.StickyRegion = ""
			c.Fuzz(&p.ExternalEntityType)
			c.Fuzz(&p.ExternalEntityKey)
		}
	}
	c.Fuzz(&p.ClusterAttribute)
}

func ActiveClusterSelectionPolicyFuzzerNoCustom(p *types.ActiveClusterSelectionPolicy, c fuzz.Continue) {
	// Fuzz all fields first without custom fuzzers
	c.FuzzNoCustom(p)
	// Then clear mutually exclusive fields based on strategy
	if p.ActiveClusterSelectionStrategy != nil {
		switch *p.ActiveClusterSelectionStrategy {
		case types.ActiveClusterSelectionStrategyRegionSticky:
			p.ExternalEntityType = ""
			p.ExternalEntityKey = ""
		case types.ActiveClusterSelectionStrategyExternalEntity:
			p.StickyRegion = ""
		}
	} else {
		p.StickyRegion = ""
		p.ExternalEntityType = ""
		p.ExternalEntityKey = ""
	}
}

func TestDataBlobArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDataBlobArray, ToDataBlobArray,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestPendingActivityInfoArrayFuzz(t *testing.T) {
	// LastFailureReason and LastFailureDetails have asymmetric mapping:
	// FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	// Excluding both fields from comparison to handle this asymmetry
	testutils.RunMapperFuzzTest(t, FromPendingActivityInfoArray, ToPendingActivityInfoArray,
		testutils.WithExcludedFields("LastFailureReason", "LastFailureDetails"),
	)
}

func TestDeleteDomainRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDeleteDomainRequest, ToDeleteDomainRequest)
}

func TestDescribeWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDescribeWorkflowExecutionRequest, ToDescribeWorkflowExecutionRequest,
		testutils.WithCustomFuncs(QueryConsistencyLevelFuzzer),
	)
}

func TestRequestCancelWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelWorkflowExecutionRequest, ToRequestCancelWorkflowExecutionRequest)
}

func TestRespondActivityTaskFailedByIDRequestFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskFailedByIDRequest, ToRespondActivityTaskFailedByIDRequest,
		testutils.WithExcludedFields("Details"),
	)
}

func TestSignalExternalWorkflowExecutionFailedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalExternalWorkflowExecutionFailedEventAttributes, ToSignalExternalWorkflowExecutionFailedEventAttributes,
		testutils.WithCustomFuncs(
			SignalExternalWorkflowExecutionFailedCauseFuzzer,
		),
	)
}

func TestClusterReplicationConfigurationArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterReplicationConfigurationArray, ToClusterReplicationConfigurationArray)
}

func TestIndexedValueTypeMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromIndexedValueTypeMap, ToIndexedValueTypeMap,
		testutils.WithCustomFuncs(
			IndexedValueTypeFuzzer,
		),
	)
}

func TestPollForActivityTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPollForActivityTaskRequest, ToPollForActivityTaskRequest)
}

func TestResetPointInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetPointInfo, ToResetPointInfo)
}

func TestCancelTimerDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCancelTimerDecisionAttributes, ToCancelTimerDecisionAttributes)
}

func TestListDomainsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListDomainsRequest, ToListDomainsRequest)
}

func TestPollForActivityTaskResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPollForActivityTaskResponse, ToPollForActivityTaskResponse)
}

func TestTaskListMetadataFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskListMetadata, ToTaskListMetadata)
}

func TestListFailoverHistoryResponseFuzz(t *testing.T) {
	// FailoverEvent.ID: empty string is normalized to nil during conversion (ToFailoverEvent only sets ID if non-empty)
	testutils.RunMapperFuzzTest(t, FromListFailoverHistoryResponse, ToListFailoverHistoryResponse,
		testutils.WithCustomFuncs(
			FailoverTypeFuzzer,
		),
		testutils.WithExcludedFields("ID"),
	)
}

func TestDescribeTaskListResponseFuzz(t *testing.T) {
	// TaskListPartitionConfig has map[int] fields that get truncated to map[int32] in proto
	// From and To are non-invertable operations, so we rely on testdata to verify the mapping is correct
	testutils.RunMapperFuzzTest(t, FromDescribeTaskListResponse, ToDescribeTaskListResponse,
		testutils.WithExcludedFields("ReadPartitions", "WritePartitions"),
	)
}

func TestStartWorkflowExecutionAsyncRequestFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy: string fields must match strategy
	testutils.RunMapperFuzzTest(t, FromStartWorkflowExecutionAsyncRequest, ToStartWorkflowExecutionAsyncRequest,
		testutils.WithCustomFuncs(
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerClearAttribute,
		),
	)
}

func TestStartWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartWorkflowExecutionResponse, ToStartWorkflowExecutionResponse)
}

func TestListFailoverHistoryRequestFiltersFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListFailoverHistoryRequestFilters, ToListFailoverHistoryRequestFilters)
}

func TestChildWorkflowExecutionTerminatedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionTerminatedEventAttributes, ToChildWorkflowExecutionTerminatedEventAttributes)
}

func TestCountWorkflowExecutionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCountWorkflowExecutionsRequest, ToCountWorkflowExecutionsRequest)
}

func TestRequestCancelExternalWorkflowExecutionFailedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelExternalWorkflowExecutionFailedEventAttributes, ToRequestCancelExternalWorkflowExecutionFailedEventAttributes,
		testutils.WithCustomFuncs(
			CancelExternalWorkflowExecutionFailedCauseFuzzer,
		),
	)
}

func TestRequestCancelExternalWorkflowExecutionInitiatedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes, ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes)
}

func TestRespondDecisionTaskCompletedRequestFuzz(t *testing.T) {
	// Excluding both complex fields - tested separately in TestDecisionArrayFuzz and TestWorkflowQueryResultMapFuzz
	testutils.RunMapperFuzzTest(t, FromRespondDecisionTaskCompletedRequest, ToRespondDecisionTaskCompletedRequest,
		testutils.WithExcludedFields("Decisions", "QueryResults"),
	)
}

func TestWorkflowQueryResultMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowQueryResultMap, ToWorkflowQueryResultMap)
}

func TestDecisionTaskStartedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecisionTaskStartedEventAttributes, ToDecisionTaskStartedEventAttributes)
}

func TestResetWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetWorkflowExecutionResponse, ToResetWorkflowExecutionResponse)
}

func TestRespondActivityTaskCompletedByIDRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskCompletedByIDRequest, ToRespondActivityTaskCompletedByIDRequest)
}

func TestIsolationGroupMetricsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromIsolationGroupMetrics, ToIsolationGroupMetrics)
}

func TestFailoverDomainResponseFuzz(t *testing.T) {
	// [BUG?] EmitMetric is always set to true in the To* mapper, not set in From mapper
	// WorkflowExecutionRetentionPeriodInDays loses precision in days→duration→days conversion
	testutils.RunMapperFuzzTest(t, FromFailoverDomainResponse, ToFailoverDomainResponse,
		testutils.WithCustomFuncs(
			func(r *types.FailoverDomainResponse, c fuzz.Continue) {
				c.Fuzz(r)
				// Mapper always creates these nested structs, never nil
				if r.DomainInfo == nil {
					r.DomainInfo = &types.DomainInfo{}
				}
				if r.Configuration == nil {
					r.Configuration = &types.DomainConfiguration{}
				}
				if r.ReplicationConfiguration == nil {
					r.ReplicationConfiguration = &types.DomainReplicationConfiguration{}
				}
			},
			testutils.EncodingTypeFuzzer,
			testutils.IsolationGroupStateFuzzer,
			testutils.DomainStatusFuzzer,
			ArchivalStatusFuzzer,
		),
		testutils.WithExcludedFields("EmitMetric", "WorkflowExecutionRetentionPeriodInDays"),
	)
}

func TestFailoverEventArrayFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An empty string ID becomes nil, but the return trip translates it back to nil
	testutils.RunMapperFuzzTest(t, FromFailoverEventArray, ToFailoverEventArray,
		testutils.WithCustomFuncs(
			FailoverTypeFuzzer,
			func(e *types.FailoverEvent, c fuzz.Continue) {
				c.Fuzz(e)
				// Empty string for ID should be nil
				if e.ID != nil && *e.ID == "" {
					e.ID = nil
				}
			},
		),
	)
}

func TestWorkflowQueryMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowQueryMap, ToWorkflowQueryMap)
}

func TestActivityTaskCompletedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityTaskCompletedEventAttributes, ToActivityTaskCompletedEventAttributes)
}

func TestHealthResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHealthResponse, ToHealthResponse)
}

func TestListArchivedWorkflowExecutionsResponseFuzz(t *testing.T) {
	// Executions is tested in FromWorkflowExecutionInfoFuzz test
	testutils.RunMapperFuzzTest(t, FromListArchivedWorkflowExecutionsResponse, ToListArchivedWorkflowExecutionsResponse,
		testutils.WithExcludedFields("Executions"),
	)
}

func TestListOpenWorkflowExecutionsResponseFuzz(t *testing.T) {
	// Executions is tested in FromWorkflowExecutionInfoFuzz test
	testutils.RunMapperFuzzTest(t, FromListOpenWorkflowExecutionsResponse, ToListOpenWorkflowExecutionsResponse,
		testutils.WithExcludedFields("Executions"),
	)
}

func TestRespondDecisionTaskFailedRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondDecisionTaskFailedRequest, ToRespondDecisionTaskFailedRequest,
		testutils.WithCustomFuncs(
			DecisionTaskFailedCauseFuzzer,
		),
	)
}

func TestDescribeWorkflowExecutionResponseFuzz(t *testing.T) {
	t.Skip("All subfields are tested in dedicated fuzz tests")
}

func TestExternalWorkflowExecutionCancelRequestedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromExternalWorkflowExecutionCancelRequestedEventAttributes, ToExternalWorkflowExecutionCancelRequestedEventAttributes)
}

func TestSignalExternalWorkflowExecutionInitiatedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalExternalWorkflowExecutionInitiatedEventAttributes, ToSignalExternalWorkflowExecutionInitiatedEventAttributes)
}

func TestStartChildWorkflowExecutionInitiatedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartChildWorkflowExecutionInitiatedEventAttributes, ToStartChildWorkflowExecutionInitiatedEventAttributes,
		testutils.WithCustomFuncs(
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerWithAttribute,
		),
	)
}

func TestWorkflowExecutionTerminatedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionTerminatedEventAttributes, ToWorkflowExecutionTerminatedEventAttributes)
}

func TestWorkerVersionInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkerVersionInfo, ToWorkerVersionInfo)
}

func TestListOpenWorkflowExecutionsRequestFuzz(t *testing.T) {
	// ExecutionFilter and TypeFilter map to a proto oneof - only one can be set
	// If both are set, TypeFilter wins and ExecutionFilter is lost
	testutils.RunMapperFuzzTest(t, FromListOpenWorkflowExecutionsRequest, ToListOpenWorkflowExecutionsRequest,
		testutils.WithCustomFuncs(
			func(r *types.ListOpenWorkflowExecutionsRequest, c fuzz.Continue) {
				c.Fuzz(r)
				// If both filters are set, clear ExecutionFilter (TypeFilter takes precedence in mapper)
				if r.ExecutionFilter != nil && r.TypeFilter != nil {
					r.ExecutionFilter = nil
				}
			},
		),
	)
}

func TestGetTaskListsByDomainResponseFuzz(t *testing.T) {
	// SKIPPED: PartitionConfig is nested inside map values (map[string]*DescribeTaskListResponse)
	// clearFieldsIf cannot modify fields inside map values due to Go reflection limitations
	// TODO(c-warren): Implement map rebuilding in clearFieldsIf to support this pattern
	t.Skip("Map value field exclusion not yet supported")
	testutils.RunMapperFuzzTest(t, FromGetTaskListsByDomainResponse, ToGetTaskListsByDomainResponse)
}

func TestRefreshWorkflowTasksRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRefreshWorkflowTasksRequest, ToRefreshWorkflowTasksRequest)
}

func TestRestartWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRestartWorkflowExecutionResponse, ToRestartWorkflowExecutionResponse)
}

func TestScanWorkflowExecutionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromScanWorkflowExecutionsRequest, ToScanWorkflowExecutionsRequest)
}

func TestUpsertWorkflowSearchAttributesDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromUpsertWorkflowSearchAttributesDecisionAttributes, ToUpsertWorkflowSearchAttributesDecisionAttributes)
}

func TestWorkflowExecutionCompletedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionCompletedEventAttributes, ToWorkflowExecutionCompletedEventAttributes)
}

func TestWorkflowQueryFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowQuery, ToWorkflowQuery)
}

func TestHistoryEventFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryEvent, ToHistoryEvent,
		testutils.WithCustomFuncs(
			EventTypeFuzzer,
			DecisionTypeFuzzer,
			TimeoutTypeFuzzer,
			TaskListKindFuzzer,
			ContinueAsNewInitiatorFuzzer,
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerWithAttribute,
			func(h *types.HistoryEvent, c fuzz.Continue) {
				// Fuzz all fields first
				c.Fuzz(h)
				// Ensure EventType is always non-nil (required by mapper switch statement)
				if h.EventType == nil {
					eventType := types.EventType(c.Intn(42)) // 0-41: All EventType values
					h.EventType = &eventType
				}
			},
		),
	)
}

func TestActivityTaskFailedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromActivityTaskFailedEventAttributes, ToActivityTaskFailedEventAttributes,
		testutils.WithExcludedFields("Details"),
	)
}

func TestHistoryFuzz(t *testing.T) {
	t.Skip("HistoryEvent array testing covered by individual event attribute fuzz tests")
}

func TestResetWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetWorkflowExecutionRequest, ToResetWorkflowExecutionRequest)
}

func TestRespondActivityTaskCanceledByIDRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskCanceledByIDRequest, ToRespondActivityTaskCanceledByIDRequest)
}

func TestTimerCanceledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTimerCanceledEventAttributes, ToTimerCanceledEventAttributes)
}

func TestDiagnoseWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDiagnoseWorkflowExecutionResponse, ToDiagnoseWorkflowExecutionResponse)
}

func TestListArchivedWorkflowExecutionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListArchivedWorkflowExecutionsRequest, ToListArchivedWorkflowExecutionsRequest)
}

func TestChildWorkflowExecutionCanceledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionCanceledEventAttributes, ToChildWorkflowExecutionCanceledEventAttributes)
}

func TestRequestCancelExternalWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelExternalWorkflowExecutionDecisionAttributes, ToRequestCancelExternalWorkflowExecutionDecisionAttributes)
}

func TestDecisionFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecision, ToDecision,
		testutils.WithCustomFuncs(
			func(d *types.Decision, c fuzz.Continue) {
				// Fuzz all fields first
				c.Fuzz(d)
				// Ensure DecisionType is always non-nil (required by mapper switch statement)
				if d.DecisionType == nil {
					decisionType := types.DecisionType(c.Intn(13)) // 0-12: All DecisionType values
					d.DecisionType = &decisionType
				}
			},
			DecisionTypeFuzzer,
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerWithAttribute,
		),
	)
}

func TestDeprecateDomainRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDeprecateDomainRequest, ToDeprecateDomainRequest)
}

func TestFailoverDomainRequestFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An empty string DomainActiveClusterName becomes nil, but the return trip translates it back to nil
	// [Missing] Reason is not yet implemented in the mapper
	testutils.RunMapperFuzzTest(t, FromFailoverDomainRequest, ToFailoverDomainRequest,
		testutils.WithCustomFuncs(
			FailoverTypeFuzzer,
		),
		testutils.WithExcludedFields("DomainActiveClusterName", "Reason"),
	)
}

func TestChildWorkflowExecutionStartedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionStartedEventAttributes, ToChildWorkflowExecutionStartedEventAttributes)
}

func TestPollForDecisionTaskResponseFuzz(t *testing.T) {
	// History.Events: nil vs empty slice (mapper creates empty when nil)
	// Contains HistoryEvent array which needs comprehensive enum fuzzers
	testutils.RunMapperFuzzTest(t, FromPollForDecisionTaskResponse, ToPollForDecisionTaskResponse,
		testutils.WithCustomFuncs(
			func(h *types.History, c fuzz.Continue) {
				c.Fuzz(h)
				// Ensure Events is never nil to avoid nil vs empty slice mismatch
				if h.Events == nil {
					h.Events = []*types.HistoryEvent{}
				}
			},
			func(h *types.HistoryEvent, c fuzz.Continue) {
				// Fuzz all fields first
				c.Fuzz(h)
				// Ensure EventType is always non-nil (required by mapper switch statement)
				if h.EventType == nil {
					eventType := types.EventType(c.Intn(42)) // 0-41: All EventType values
					h.EventType = &eventType
				}
			},
			EventTypeFuzzer,
			DecisionTypeFuzzer,
			TimeoutTypeFuzzer,
			TaskListKindFuzzer,
			ContinueAsNewInitiatorFuzzer,
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerClearAttribute,
			SignalExternalWorkflowExecutionFailedCauseFuzzer,
			CancelExternalWorkflowExecutionFailedCauseFuzzer,
			ChildWorkflowExecutionFailedCauseFuzzer,
			WorkflowExecutionCloseStatusFuzzer,
			ParentClosePolicyFuzzer,
		),
	)
}

func TestDecisionArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecisionArray, ToDecisionArray,
		testutils.WithCustomFuncs(
			func(d *types.Decision, c fuzz.Continue) {
				// Fuzz all fields first
				c.Fuzz(d)
				// Ensure DecisionType is always non-nil (required by mapper switch statement)
				if d.DecisionType == nil {
					decisionType := types.DecisionType(c.Intn(13)) // 0-12: All DecisionType values
					d.DecisionType = &decisionType
				}
			},
			DecisionTypeFuzzer,
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerWithAttribute,
		),
	)
}

func TestClusterAttributeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterAttribute, ToClusterAttribute)
}

func TestDecisionTaskTimedOutEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecisionTaskTimedOutEventAttributes, ToDecisionTaskTimedOutEventAttributes,
		testutils.WithCustomFuncs(
			DecisionTaskTimedOutCauseFuzzer,
		),
	)
}

func TestIndexedValueTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t,
		func(v types.IndexedValueType) apiv1.IndexedValueType {
			return FromIndexedValueType(v)
		},
		func(v apiv1.IndexedValueType) types.IndexedValueType {
			return ToIndexedValueType(v)
		},
	)
}

func TestDescribeTaskListRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDescribeTaskListRequest, ToDescribeTaskListRequest)
}

func TestPollerInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPollerInfo, ToPollerInfo)
}

func TestTerminateWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTerminateWorkflowExecutionRequest, ToTerminateWorkflowExecutionRequest)
}

func TestListFailoverHistoryRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListFailoverHistoryRequest, ToListFailoverHistoryRequest)
}

func TestListClosedWorkflowExecutionsRequestFuzz(t *testing.T) {
	// ExecutionFilter, TypeFilter, and StatusFilter map to a proto oneof - only one can be set
	// If multiple are set, StatusFilter wins (checked last), then TypeFilter, then ExecutionFilter
	testutils.RunMapperFuzzTest(t, FromListClosedWorkflowExecutionsRequest, ToListClosedWorkflowExecutionsRequest,
		testutils.WithCustomFuncs(
			func(r *types.ListClosedWorkflowExecutionsRequest, c fuzz.Continue) {
				c.Fuzz(r)
				// Clear filters that would be lost in the oneof mapping
				if r.StatusFilter != nil {
					// StatusFilter wins - clear the others
					r.ExecutionFilter = nil
					r.TypeFilter = nil
				} else if r.TypeFilter != nil {
					// TypeFilter wins over ExecutionFilter
					r.ExecutionFilter = nil
				}
			},
		),
	)
}

func TestChildWorkflowExecutionFailedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionFailedEventAttributes, ToChildWorkflowExecutionFailedEventAttributes,
		testutils.WithExcludedFields("Details"),
	)
}

func TestListClosedWorkflowExecutionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListClosedWorkflowExecutionsResponse, ToListClosedWorkflowExecutionsResponse,
		testutils.WithExcludedFields("Executions"), // Executions is tested in FromWorkflowExecutionInfoFuzz test
	)
}

func TestRestartWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRestartWorkflowExecutionRequest, ToRestartWorkflowExecutionRequest)
}

func TestActivityTaskCanceledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityTaskCanceledEventAttributes, ToActivityTaskCanceledEventAttributes)
}

func TestListDomainsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListDomainsResponse, ToListDomainsResponse,
		testutils.WithExcludedFields("Domains"), // Domains is tested in TestDescribeDomainResponseDomainFuzz test
	)
}

func TestMarkerRecordedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMarkerRecordedEventAttributes, ToMarkerRecordedEventAttributes)
}

func TestTimerFiredEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTimerFiredEventAttributes, ToTimerFiredEventAttributes)
}

func TestUpdateDomainRequestFuzz(t *testing.T) {
	// [BUG?/DEPRECATED] EmitMetric is always set to true in the To* mapper, not set in From mapper
	// [Missing] FailoverReason is not yet implemented in the mapper
	// [BUG] WorkflowExecutionRetentionPeriodInDays loses precision in days→duration→days conversion
	// HistoryArchivalStatus, VisibilityArchivalStatus: only included in field mask if corresponding URI is non-nil
	testutils.RunMapperFuzzTest(t, FromUpdateDomainRequest, ToUpdateDomainRequest,
		testutils.WithCustomFuncs(
			testutils.EncodingTypeFuzzer,
			testutils.IsolationGroupStateFuzzer,
			testutils.DomainStatusFuzzer,
			ArchivalStatusFuzzer,
		),
		testutils.WithExcludedFields("EmitMetric", "FailoverReason", "WorkflowExecutionRetentionPeriodInDays", "HistoryArchivalStatus", "VisibilityArchivalStatus"),
	)
}

func TestActivityTaskTimedOutEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromActivityTaskTimedOutEventAttributes, ToActivityTaskTimedOutEventAttributes,
		testutils.WithExcludedFields("LastFailureDetails"),
	)
}

func TestChildWorkflowExecutionTimedOutEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionTimedOutEventAttributes, ToChildWorkflowExecutionTimedOutEventAttributes)
}

func TestDecisionTaskCompletedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecisionTaskCompletedEventAttributes, ToDecisionTaskCompletedEventAttributes)
}

func TestGetWorkflowExecutionHistoryResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetWorkflowExecutionHistoryResponse, ToGetWorkflowExecutionHistoryResponse,
		testutils.WithExcludedFields("History"), // History is tested in TestHistoryEventFuzz test
		testutils.WithCustomFuncs(
			testutils.EncodingTypeFuzzer,
		),
	)
}

func TestQueryRejectedFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromQueryRejected, ToQueryRejected)
}

func TestResetPointsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetPoints, ToResetPoints)
}

func TestSignalWithStartWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalWithStartWorkflowExecutionRequest, ToSignalWithStartWorkflowExecutionRequest,
		testutils.WithCustomFuncs(
			WorkflowIDReusePolicyFuzzer,
			CronOverlapPolicyFuzzer,
			ActiveClusterSelectionStrategyFuzzer,
			ActiveClusterSelectionPolicyFuzzerWithAttribute,
		),
	)
}

func TestWorkflowExecutionFailedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionFailedEventAttributes, ToWorkflowExecutionFailedEventAttributes,
		testutils.WithExcludedFields("Details"),
	)
}

func TestPendingDecisionInfoFuzz(t *testing.T) {
	// [BUG] Attempt field is truncated from int64 to int32 in proto mapper
	testutils.RunMapperFuzzTest(t, FromPendingDecisionInfo, ToPendingDecisionInfo,
		testutils.WithExcludedFields("Attempt"),
	)
}

func TestStickyExecutionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStickyExecutionAttributes, ToStickyExecutionAttributes)
}

func TestWorkflowExecutionFilterFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionFilter, ToWorkflowExecutionFilter)
}

func TestActivityTaskStartedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromActivityTaskStartedEventAttributes, ToActivityTaskStartedEventAttributes,
		testutils.WithExcludedFields("LastFailureDetails"),
	)
}

func TestDecisionTaskScheduledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDecisionTaskScheduledEventAttributes, ToDecisionTaskScheduledEventAttributes,
		testutils.WithCustomFuncs(
			func(v *types.DecisionTaskScheduledEventAttributes, c fuzz.Continue) {
				v.TaskList = &types.TaskList{}
				c.Fuzz(v.TaskList)
				if c.RandBool() {
					timeout := c.Int31()
					v.StartToCloseTimeoutSeconds = &timeout
				}
				// Attempt is int64 but proto is int32, so limit to int32 range
				v.Attempt = int64(c.Int31())
			},
		),
	)
}

func TestQueryWorkflowResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromQueryWorkflowResponse, ToQueryWorkflowResponse)
}

func TestSignalWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalWorkflowExecutionRequest, ToSignalWorkflowExecutionRequest)
}

func TestPendingChildExecutionInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPendingChildExecutionInfo, ToPendingChildExecutionInfo)
}

func TestPollForDecisionTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPollForDecisionTaskRequest, ToPollForDecisionTaskRequest)
}

func TestRecordMarkerDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRecordMarkerDecisionAttributes, ToRecordMarkerDecisionAttributes)
}

func TestWorkflowQueryResultFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowQueryResult, ToWorkflowQueryResult)
}

func TestStartChildWorkflowExecutionFailedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartChildWorkflowExecutionFailedEventAttributes, ToStartChildWorkflowExecutionFailedEventAttributes,
		testutils.WithCustomFuncs(
			func(e *types.ChildWorkflowExecutionFailedCause, c fuzz.Continue) {
				*e = types.ChildWorkflowExecutionFailedCause(0) // 0: WorkflowAlreadyRunning
			},
		),
	)
}

func TestTaskListStatusFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskListStatus, ToTaskListStatus)
}

func TestTaskListPartitionMetadataArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskListPartitionMetadataArray, ToTaskListPartitionMetadataArray)
}

func TestDecisionTaskFailedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if Reason is non-nil, so Details without Reason are dropped
	testutils.RunMapperFuzzTest(t, FromDecisionTaskFailedEventAttributes, ToDecisionTaskFailedEventAttributes,
		testutils.WithCustomFuncs(
			DecisionTaskFailedCauseFuzzer,
		),
		testutils.WithExcludedFields("Details"),
	)
}

func TestDescribeDomainRequestFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An empty string ID becomes nil, but the return trip translates it back to nil
	testutils.RunMapperFuzzTest(t, FromDescribeDomainRequest, ToDescribeDomainRequest,
		testutils.WithCustomFuncs(
			func(r *types.DescribeDomainRequest, c fuzz.Continue) {
				c.Fuzz(r)
				// Must have at least one of Name or UUID set (oneof field)
				if r.Name == nil && r.UUID == nil {
					if c.RandBool() {
						r.Name = new(string)
						*r.Name = c.RandString()
					} else {
						r.UUID = new(string)
						*r.UUID = c.RandString()
					}
				}
			},
		),
		testutils.WithExcludedFields("ID"),
	)
}

func TestResetStickyTaskListRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetStickyTaskListRequest, ToResetStickyTaskListRequest)
}

func TestActivityTaskCancelRequestedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityTaskCancelRequestedEventAttributes, ToActivityTaskCancelRequestedEventAttributes)
}

func TestBadBinaryInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBadBinaryInfo, ToBadBinaryInfo)
}

func TestCancelTimerFailedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCancelTimerFailedEventAttributes, ToCancelTimerFailedEventAttributes)
}

func TestRespondActivityTaskCompletedRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskCompletedRequest, ToRespondActivityTaskCompletedRequest)
}

func TestPaginationOptionsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPaginationOptions, ToPaginationOptions)
}

func TestCancelWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCancelWorkflowExecutionDecisionAttributes, ToCancelWorkflowExecutionDecisionAttributes)
}

func TestFailoverInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromFailoverInfo, ToFailoverInfo)
}

func TestRespondActivityTaskFailedRequestFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskFailedRequest, ToRespondActivityTaskFailedRequest,
		testutils.WithExcludedFields("Details"),
	)
}

func TestScanWorkflowExecutionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromScanWorkflowExecutionsResponse, ToScanWorkflowExecutionsResponse,
		testutils.WithExcludedFields("Executions"), // Executions is tested in FromWorkflowExecutionInfoFuzz test
	)
}

func TestScheduleActivityTaskDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromScheduleActivityTaskDecisionAttributes, ToScheduleActivityTaskDecisionAttributes)
}

func TestTaskListFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskList, ToTaskList)
}

func TestMemoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromMemo, ToMemo)
}

func TestResetPointInfoArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetPointInfoArray, ToResetPointInfoArray)
}

func TestSearchAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSearchAttributes, ToSearchAttributes)
}

func TestTaskListPartitionMetadataFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskListPartitionMetadata, ToTaskListPartitionMetadata)
}

func TestWorkflowExecutionSignaledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionSignaledEventAttributes, ToWorkflowExecutionSignaledEventAttributes)
}

func TestHistoryEventArrayFuzz(t *testing.T) {
	t.Skip("Tested in TestHistoryEventFuzz test")
}

func TestActivityLocalDispatchInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityLocalDispatchInfo, ToActivityLocalDispatchInfo)
}

func TestStartWorkflowExecutionRequestFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields
	testutils.RunMapperFuzzTest(t, FromStartWorkflowExecutionRequest, ToStartWorkflowExecutionRequest,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			WorkflowIDReusePolicyFuzzer,
		),
	)
}

func TestUpsertWorkflowSearchAttributesEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromUpsertWorkflowSearchAttributesEventAttributes, ToUpsertWorkflowSearchAttributesEventAttributes)
}

func TestExternalWorkflowExecutionSignaledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromExternalWorkflowExecutionSignaledEventAttributes, ToExternalWorkflowExecutionSignaledEventAttributes)
}

func TestWorkflowTypeFilterFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowTypeFilter, ToWorkflowTypeFilter)
}

func TestDescribeTaskListResponseMapFuzz(t *testing.T) {
	// Map[int] with int64 keys don't roundtrip correctly through proto int32
	// Use custom fuzzer to nil out ReadPartitions/WritePartitions in all map values
	testutils.RunMapperFuzzTest(t, FromDescribeTaskListResponseMap, ToDescribeTaskListResponseMap,
		testutils.WithCustomFuncs(
			func(m *map[string]*types.DescribeTaskListResponse, c fuzz.Continue) {
				c.Fuzz(m)
				// Exclude map[int] fields that don't roundtrip through proto
				for _, resp := range *m {
					if resp != nil && resp.PartitionConfig != nil {
						resp.PartitionConfig.ReadPartitions = nil
						resp.PartitionConfig.WritePartitions = nil
					}
				}
			},
		),
	)
}

func TestTimerStartedEventAttributesFuzz(t *testing.T) {
	// StartToFireTimeoutSeconds is int64 but proto converts through int32, so limit to int32 range
	testutils.RunMapperFuzzTest(t, FromTimerStartedEventAttributes, ToTimerStartedEventAttributes,
		testutils.WithCustomFuncs(
			func(v *types.TimerStartedEventAttributes, c fuzz.Continue) {
				c.Fuzz(v)
				if v.StartToFireTimeoutSeconds != nil {
					*v.StartToFireTimeoutSeconds = int64(c.Int31())
				}
			},
		),
	)
}

func TestActivityTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityType, ToActivityType)
}

func TestWorkflowExecutionFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecution, ToWorkflowExecution)
}

func TestCompleteWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCompleteWorkflowExecutionDecisionAttributes, ToCompleteWorkflowExecutionDecisionAttributes)
}

func TestListTaskListPartitionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListTaskListPartitionsRequest, ToListTaskListPartitionsRequest)
}

func TestCountWorkflowExecutionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromCountWorkflowExecutionsResponse, ToCountWorkflowExecutionsResponse)
}

func TestDataBlobFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDataBlob, ToDataBlob,
		testutils.WithCustomFuncs(
			testutils.EncodingTypeFuzzer,
		),
	)
}

func TestActiveClusterInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActiveClusterInfo, ToActiveClusterInfo)
}

func TestActiveClusterSelectionPolicyFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields based on Strategy
	testutils.RunMapperFuzzTest(t, FromActiveClusterSelectionPolicy, ToActiveClusterSelectionPolicy,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
		),
	)
}

func TestRespondActivityTaskCanceledRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondActivityTaskCanceledRequest, ToRespondActivityTaskCanceledRequest)
}

func TestPendingChildExecutionInfoArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPendingChildExecutionInfoArray, ToPendingChildExecutionInfoArray)
}

func TestRecordActivityTaskHeartbeatResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRecordActivityTaskHeartbeatResponse, ToRecordActivityTaskHeartbeatResponse)
}

func TestStartTimerDecisionAttributesFuzz(t *testing.T) {
	// StartToFireTimeoutSeconds is int64 but proto converts through int32
	testutils.RunMapperFuzzTest(t, FromStartTimerDecisionAttributes, ToStartTimerDecisionAttributes,
		testutils.WithCustomFuncs(
			func(v *types.StartTimerDecisionAttributes, c fuzz.Continue) {
				c.Fuzz(v)
				if v.StartToFireTimeoutSeconds != nil {
					val := int64(c.Int31())
					v.StartToFireTimeoutSeconds = &val
				}
			},
		),
	)
}

func TestSupportedClientVersionsFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSupportedClientVersions, ToSupportedClientVersions)
}

func TestClusterFailoverArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterFailoverArray, ToClusterFailoverArray)
}

func TestActiveClustersFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActiveClusters, ToActiveClusters)
}

func TestChildWorkflowExecutionCompletedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromChildWorkflowExecutionCompletedEventAttributes, ToChildWorkflowExecutionCompletedEventAttributes)
}

func TestClusterReplicationConfigurationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterReplicationConfiguration, ToClusterReplicationConfiguration)
}

func TestWorkflowExecutionInfoFuzz(t *testing.T) {
	// [BUG] UpdateTime missing from mapper (not sent over proto)
	// [Intended] ParentDomainID, ParentDomain: converted to empty string instead of nil when ParentExecutionInfo exists but field is empty
	// [Intended] ParentInitiatedID: converted to 0 instead of nil when ParentExecutionInfo exists but field is 0
	// [BUG] CronSchedule is not round trip safe with an empty string
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionInfo, ToWorkflowExecutionInfo,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom, // ActiveClusterSelectionPolicy has mutually exclusive fields based on Strategy
		),
		testutils.WithExcludedFields("UpdateTime", "ParentDomainID", "ParentDomain", "ParentInitiatedID", "CronSchedule"),
	)
}

func TestIsolationGroupMetricsMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromIsolationGroupMetricsMap, ToIsolationGroupMetricsMap)
}

func TestSignalExternalWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalExternalWorkflowExecutionDecisionAttributes, ToSignalExternalWorkflowExecutionDecisionAttributes)
}

func TestSignalWithStartWorkflowExecutionAsyncRequestFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields, WorkflowIDReusePolicy enum
	testutils.RunMapperFuzzTest(t, FromSignalWithStartWorkflowExecutionAsyncRequest, ToSignalWithStartWorkflowExecutionAsyncRequest,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			WorkflowIDReusePolicyFuzzer,
		),
	)
}

func TestPollerInfoArrayFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPollerInfoArray, ToPollerInfoArray)
}

func TestResetStickyTaskListResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromResetStickyTaskListResponse, ToResetStickyTaskListResponse)
}

func TestAPITaskListPartitionFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAPITaskListPartition, ToAPITaskListPartition)
}

func TestListWorkflowExecutionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListWorkflowExecutionsResponse, ToListWorkflowExecutionsResponse,
		testutils.WithExcludedFields("Executions"), // Executions is tested in FromWorkflowExecutionInfoFuzz test
	)
}

func TestParentExecutionInfoFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromParentExecutionInfo, ToParentExecutionInfo)
}

func TestFailWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromFailWorkflowExecutionDecisionAttributes, ToFailWorkflowExecutionDecisionAttributes,
		testutils.WithExcludedFields("Details"),
	)
}

func TestGetClusterInfoResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetClusterInfoResponse, ToGetClusterInfoResponse)
}

func TestSignalWithStartWorkflowExecutionAsyncResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalWithStartWorkflowExecutionAsyncResponse, ToSignalWithStartWorkflowExecutionAsyncResponse)
}

func TestDescribeDomainResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDescribeDomainResponse, ToDescribeDomainResponse,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			testutils.DomainStatusFuzzer,
			ArchivalStatusFuzzer,
			func(r *types.DescribeDomainResponse, c fuzz.Continue) {
				c.Fuzz(r)
				// [BUG] On a round-trip, if DomainInfo, Configuration, or ReplicationConfiguration are nil, they become non-nil
				if r.DomainInfo == nil {
					r.DomainInfo = &types.DomainInfo{}
				}
				if r.Configuration == nil {
					r.Configuration = &types.DomainConfiguration{}
				}
				if r.ReplicationConfiguration == nil {
					r.ReplicationConfiguration = &types.DomainReplicationConfiguration{}
				}
			},
		),
		testutils.WithExcludedFields("EmitMetric"), // EmitMetric is deprecated and permanently set to true
	)
}

func TestListWorkflowExecutionsRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListWorkflowExecutionsRequest, ToListWorkflowExecutionsRequest)
}

func TestRequestCancelActivityTaskFailedEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelActivityTaskFailedEventAttributes, ToRequestCancelActivityTaskFailedEventAttributes)
}

func TestStatusFilterFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStatusFilter, ToStatusFilter)
}

func TestPayloadFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPayload, ToPayload)
}

func TestRespondDecisionTaskCompletedResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondDecisionTaskCompletedResponse, ToRespondDecisionTaskCompletedResponse,
		testutils.WithCustomFuncs(
			func(h *types.History, c fuzz.Continue) {
				c.Fuzz(h)
				// nil Events slice becomes empty after proto roundtrip
				if h.Events == nil {
					h.Events = []*types.HistoryEvent{}
				}
			},
		),
	)
}

func TestStartTimeFilterFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartTimeFilter, ToStartTimeFilter)
}

func TestWorkflowExecutionContinuedAsNewEventAttributesFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields
	// JitterStartSeconds don't roundtrip correctly
	// [BUG] FailureDetails requires FailureReason to be set
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionContinuedAsNewEventAttributes, ToWorkflowExecutionContinuedAsNewEventAttributes,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			ContinueAsNewInitiatorFuzzer,
		),
		testutils.WithExcludedFields("JitterStartSeconds", "FailureDetails"),
	)
}

func TestGetSearchAttributesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetSearchAttributesResponse, ToGetSearchAttributesResponse)
}

func TestRecordActivityTaskHeartbeatByIDRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRecordActivityTaskHeartbeatByIDRequest, ToRecordActivityTaskHeartbeatByIDRequest)
}

func TestTaskIDBlockFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromTaskIDBlock, ToTaskIDBlock)
}

func TestActivityLocalDispatchInfoMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromActivityLocalDispatchInfoMap, ToActivityLocalDispatchInfoMap)
}

func TestHeaderFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHeader, ToHeader)
}

func TestQueryWorkflowRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromQueryWorkflowRequest, ToQueryWorkflowRequest,
		testutils.WithCustomFuncs(
			QueryRejectConditionFuzzer,
			QueryConsistencyLevelFuzzer,
		),
	)
}

func TestRecordActivityTaskHeartbeatByIDResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRecordActivityTaskHeartbeatByIDResponse, ToRecordActivityTaskHeartbeatByIDResponse)
}

func TestWorkflowExecutionConfigurationFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionConfiguration, ToWorkflowExecutionConfiguration)
}

func TestPendingActivityInfoFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	testutils.RunMapperFuzzTest(t, FromPendingActivityInfo, ToPendingActivityInfo,
		testutils.WithCustomFuncs(
			PendingActivityStateFuzzer,
		),
		testutils.WithExcludedFields("LastFailureDetails"),
	)
}

func TestDiagnoseWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromDiagnoseWorkflowExecutionRequest, ToDiagnoseWorkflowExecutionRequest)
}

func TestWorkflowExecutionStartedEventAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	// ActiveClusterSelectionPolicy has mutually exclusive fields
	// JitterStartSeconds don't roundtrip
	// Empty string fields become nil
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionStartedEventAttributes, ToWorkflowExecutionStartedEventAttributes,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			func(e *types.WorkflowExecutionStartedEventAttributes, c fuzz.Continue) {
				c.Fuzz(e)
				// Empty strings become nil after proto roundtrip
				if e.ParentWorkflowDomain != nil && *e.ParentWorkflowDomain == "" {
					e.ParentWorkflowDomain = nil
				}
				if e.ParentWorkflowDomainID != nil && *e.ParentWorkflowDomainID == "" {
					e.ParentWorkflowDomainID = nil
				}
			},
		),
		testutils.WithExcludedFields("JitterStartSeconds"),
	)
}

func TestGetWorkflowExecutionHistoryRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetWorkflowExecutionHistoryRequest, ToGetWorkflowExecutionHistoryRequest,
		testutils.WithCustomFuncs(
			HistoryEventFilterTypeFuzzer,
			QueryConsistencyLevelFuzzer,
		),
	)
}

func TestRequestCancelActivityTaskDecisionAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRequestCancelActivityTaskDecisionAttributes, ToRequestCancelActivityTaskDecisionAttributes)
}

func TestClusterFailoverFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterFailover, ToClusterFailover)
}

func TestDescribeDomainResponseDomainFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields based on Strategy
	// WorkflowExecutionRetentionPeriodInDays: nil→0 conversion
	// [BUG] DomainInfo, Configuration, ReplicationConfiguration must be non-nil
	testutils.RunMapperFuzzTest(t, FromDescribeDomainResponseDomain, ToDescribeDomainResponseDomain,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			testutils.DomainStatusFuzzer,
			ArchivalStatusFuzzer,
			func(r *types.DescribeDomainResponse, c fuzz.Continue) {
				c.Fuzz(r)
				// Proto mapper requires these to be non-nil
				if r.DomainInfo == nil {
					r.DomainInfo = &types.DomainInfo{}
				}
				if r.Configuration == nil {
					r.Configuration = &types.DomainConfiguration{}
				}
				if r.ReplicationConfiguration == nil {
					r.ReplicationConfiguration = &types.DomainReplicationConfiguration{}
				}
			},
		),
		testutils.WithExcludedFields("WorkflowExecutionRetentionPeriodInDays", "EmitMetric"),
	)
}

func TestRetryPolicyFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRetryPolicy, ToRetryPolicy)
}

func TestStartWorkflowExecutionAsyncResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromStartWorkflowExecutionAsyncResponse, ToStartWorkflowExecutionAsyncResponse)
}

func TestAPITaskListPartitionConfigFuzz(t *testing.T) {
	// From and To are non-invertable operations, so we rely on testdata to verify the mapping is correct
	testutils.RunMapperFuzzTest(t, FromAPITaskListPartitionConfig, ToAPITaskListPartitionConfig,
		testutils.WithExcludedFields("ReadPartitions", "WritePartitions"),
	)
}

func TestRespondQueryTaskCompletedRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRespondQueryTaskCompletedRequest, ToRespondQueryTaskCompletedRequest,
		testutils.WithCustomFuncs(
			CompletedTypeFuzzer,
		),
	)
}

func TestListTaskListPartitionsResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromListTaskListPartitionsResponse, ToListTaskListPartitionsResponse)
}

func TestWorkflowTypeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowType, ToWorkflowType)
}

func TestWorkflowExecutionInfoArrayFuzz(t *testing.T) {
	t.Skip("Tested in FromWorkflowExecutionInfoFuzz test")
}

func TestRecordActivityTaskHeartbeatRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromRecordActivityTaskHeartbeatRequest, ToRecordActivityTaskHeartbeatRequest)
}

func TestWorkflowExecutionCancelRequestedEventAttributesFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An external initiated event ID of 0 becomes nil, but the return trip translates it back to nil
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionCancelRequestedEventAttributes, ToWorkflowExecutionCancelRequestedEventAttributes,
		testutils.WithExcludedFields("ExternalInitiatedEventID"),
	)
}

func TestUpdateDomainResponseFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields based on Strategy
	// WorkflowExecutionRetentionPeriodInDays: nil→0 conversion
	// DomainInfo, Configuration, ReplicationConfiguration must be non-nil
	testutils.RunMapperFuzzTest(t, FromUpdateDomainResponse, ToUpdateDomainResponse,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			testutils.DomainStatusFuzzer,
			ArchivalStatusFuzzer,
			func(r *types.UpdateDomainResponse, c fuzz.Continue) {
				c.Fuzz(r)
				// Proto mapper requires these to be non-nil
				if r.DomainInfo == nil {
					r.DomainInfo = &types.DomainInfo{}
				}
				if r.Configuration == nil {
					r.Configuration = &types.DomainConfiguration{}
				}
				if r.ReplicationConfiguration == nil {
					r.ReplicationConfiguration = &types.DomainReplicationConfiguration{}
				}
			},
		),
		testutils.WithExcludedFields("WorkflowExecutionRetentionPeriodInDays", "EmitMetric"),
	)
}

func TestWorkflowExecutionCanceledEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionCanceledEventAttributes, ToWorkflowExecutionCanceledEventAttributes)
}

func TestWorkflowExecutionTimedOutEventAttributesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromWorkflowExecutionTimedOutEventAttributes, ToWorkflowExecutionTimedOutEventAttributes)
}

func TestPayloadMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromPayloadMap, ToPayloadMap)
}

func TestAutoConfigHintFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromAutoConfigHint, ToAutoConfigHint)
}

func TestBadBinariesFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBadBinaries, ToBadBinaries)
}

func TestSignalWithStartWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromSignalWithStartWorkflowExecutionResponse, ToSignalWithStartWorkflowExecutionResponse)
}

func TestStartChildWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	// ActiveClusterSelectionPolicy has mutually exclusive fields, WorkflowIDReusePolicy enum
	testutils.RunMapperFuzzTest(t, FromStartChildWorkflowExecutionDecisionAttributes, ToStartChildWorkflowExecutionDecisionAttributes,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			WorkflowIDReusePolicyFuzzer,
		),
	)
}

func TestActivityTaskScheduledEventAttributesFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An empty string domain becomes nil, but the return trip translates it back to nil - not empty string
	testutils.RunMapperFuzzTest(t, FromActivityTaskScheduledEventAttributes, ToActivityTaskScheduledEventAttributes,
		testutils.WithExcludedFields("Domain"),
	)
}

func TestGetTaskListsByDomainRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromGetTaskListsByDomainRequest, ToGetTaskListsByDomainRequest)
}

func TestFailoverEventFuzz(t *testing.T) {
	// [BUG] Non-symmetric mapping: An empty string ID becomes nil, but the return trip translates it back to nil
	testutils.RunMapperFuzzTest(t, FromFailoverEvent, ToFailoverEvent,
		testutils.WithCustomFuncs(
			FailoverTypeFuzzer,
			func(e *types.FailoverEvent, c fuzz.Continue) {
				c.Fuzz(e)
				// Empty string ID becomes nil
				if e.ID != nil && *e.ID == "" {
					e.ID = nil
				}
			},
		),
	)
}

func TestContinueAsNewWorkflowExecutionDecisionAttributesFuzz(t *testing.T) {
	// [BUG] FromFailure only creates a Failure object if reason is non-nil, so details without reason are dropped
	// ActiveClusterSelectionPolicy has mutually exclusive fields
	// JitterStartSeconds doesn't roundtrip correctly
	testutils.RunMapperFuzzTest(t, FromContinueAsNewWorkflowExecutionDecisionAttributes, ToContinueAsNewWorkflowExecutionDecisionAttributes,
		testutils.WithCustomFuncs(
			ActiveClusterSelectionPolicyFuzzerNoCustom,
			ContinueAsNewInitiatorFuzzer,
		),
		testutils.WithExcludedFields("JitterStartSeconds", "FailureDetails"),
	)
}

func TestClusterAttributeScopeFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromClusterAttributeScope, ToClusterAttributeScope)
}

func TestBadBinaryInfoMapFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromBadBinaryInfoMap, ToBadBinaryInfoMap)
}
