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

	"github.com/stretchr/testify/assert"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
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
func TestRegisterDomainRequest(t *testing.T) {
	for _, item := range []*types.RegisterDomainRequest{nil, {EmitMetric: common.BoolPtr(true)}, &testdata.RegisterDomainRequest} {
		assert.Equal(t, item, ToRegisterDomainRequest(FromRegisterDomainRequest(item)))
	}
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
	for _, item := range []*types.WorkflowExecutionInfo{nil, {}, &testdata.WorkflowExecutionInfo, &testdata.CronWorkflowExecutionInfo} {
		assert.Equal(t, item, ToWorkflowExecutionInfo(FromWorkflowExecutionInfo(item)))
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
