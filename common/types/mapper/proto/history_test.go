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

	historyv1 "github.com/uber/cadence/.gen/proto/history/v1"
	sharedv1 "github.com/uber/cadence/.gen/proto/shared/v1"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestHistoryCloseShardRequest(t *testing.T) {
	for _, item := range []*types.CloseShardRequest{nil, {}, &testdata.HistoryCloseShardRequest} {
		assert.Equal(t, item, ToHistoryCloseShardRequest(FromHistoryCloseShardRequest(item)))
	}
}
func TestHistoryDescribeHistoryHostRequest(t *testing.T) {
	for _, item := range []*types.DescribeHistoryHostRequest{nil, {}, &testdata.HistoryDescribeHistoryHostRequest} {
		assert.Equal(t, item, ToHistoryDescribeHistoryHostRequest(FromHistoryDescribeHistoryHostRequest(item)))
	}
}
func TestHistoryDescribeHistoryHostResponse(t *testing.T) {
	for _, item := range []*types.DescribeHistoryHostResponse{nil, {}, &testdata.HistoryDescribeHistoryHostResponse} {
		assert.Equal(t, item, ToHistoryDescribeHistoryHostResponse(FromHistoryDescribeHistoryHostResponse(item)))
	}
}
func TestHistoryDescribeMutableStateRequest(t *testing.T) {
	for _, item := range []*types.DescribeMutableStateRequest{nil, {}, &testdata.HistoryDescribeMutableStateRequest} {
		assert.Equal(t, item, ToHistoryDescribeMutableStateRequest(FromHistoryDescribeMutableStateRequest(item)))
	}
}
func TestHistoryDescribeMutableStateResponse(t *testing.T) {
	for _, item := range []*types.DescribeMutableStateResponse{nil, {}, &testdata.HistoryDescribeMutableStateResponse} {
		assert.Equal(t, item, ToHistoryDescribeMutableStateResponse(FromHistoryDescribeMutableStateResponse(item)))
	}
}
func TestHistoryDescribeQueueRequest(t *testing.T) {
	for _, item := range []*types.DescribeQueueRequest{nil, {}, &testdata.HistoryDescribeQueueRequest} {
		assert.Equal(t, item, ToHistoryDescribeQueueRequest(FromHistoryDescribeQueueRequest(item)))
	}
}
func TestHistoryDescribeQueueResponse(t *testing.T) {
	for _, item := range []*types.DescribeQueueResponse{nil, {}, &testdata.HistoryDescribeQueueResponse} {
		assert.Equal(t, item, ToHistoryDescribeQueueResponse(FromHistoryDescribeQueueResponse(item)))
	}
}
func TestHistoryDescribeWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistoryDescribeWorkflowExecutionRequest{nil, {}, &testdata.HistoryDescribeWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistoryDescribeWorkflowExecutionRequest(FromHistoryDescribeWorkflowExecutionRequest(item)))
	}
}
func TestHistoryDescribeWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.DescribeWorkflowExecutionResponse{nil, {}, &testdata.HistoryDescribeWorkflowExecutionResponse} {
		assert.Equal(t, item, ToHistoryDescribeWorkflowExecutionResponse(FromHistoryDescribeWorkflowExecutionResponse(item)))
	}
}
func TestHistoryGetDLQReplicationMessagesRequest(t *testing.T) {
	for _, item := range []*types.GetDLQReplicationMessagesRequest{nil, {}, &testdata.HistoryGetDLQReplicationMessagesRequest} {
		assert.Equal(t, item, ToHistoryGetDLQReplicationMessagesRequest(FromHistoryGetDLQReplicationMessagesRequest(item)))
	}
}
func TestHistoryGetDLQReplicationMessagesResponse(t *testing.T) {
	for _, item := range []*types.GetDLQReplicationMessagesResponse{nil, {}, &testdata.HistoryGetDLQReplicationMessagesResponse} {
		assert.Equal(t, item, ToHistoryGetDLQReplicationMessagesResponse(FromHistoryGetDLQReplicationMessagesResponse(item)))
	}
}
func TestHistoryGetMutableStateRequest(t *testing.T) {
	for _, item := range []*types.GetMutableStateRequest{nil, {}, &testdata.HistoryGetMutableStateRequest} {
		assert.Equal(t, item, ToHistoryGetMutableStateRequest(FromHistoryGetMutableStateRequest(item)))
	}
}
func TestHistoryGetMutableStateResponse(t *testing.T) {
	for _, item := range []*types.GetMutableStateResponse{nil, &testdata.HistoryGetMutableStateResponse} {
		assert.Equal(t, item, ToHistoryGetMutableStateResponse(FromHistoryGetMutableStateResponse(item)))
	}
}
func TestHistoryGetReplicationMessagesRequest(t *testing.T) {
	for _, item := range []*types.GetReplicationMessagesRequest{nil, {}, &testdata.HistoryGetReplicationMessagesRequest} {
		assert.Equal(t, item, ToHistoryGetReplicationMessagesRequest(FromHistoryGetReplicationMessagesRequest(item)))
	}
}
func TestHistoryGetReplicationMessagesResponse(t *testing.T) {
	for _, item := range []*types.GetReplicationMessagesResponse{nil, {}, &testdata.HistoryGetReplicationMessagesResponse} {
		assert.Equal(t, item, ToHistoryGetReplicationMessagesResponse(FromHistoryGetReplicationMessagesResponse(item)))
	}
}
func TestHistoryCountDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.CountDLQMessagesRequest{nil, {}, &testdata.HistoryCountDLQMessagesRequest} {
		assert.Equal(t, item, ToHistoryCountDLQMessagesRequest(FromHistoryCountDLQMessagesRequest(item)))
	}
}
func TestHistoryCountDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.HistoryCountDLQMessagesResponse{nil, {}, &testdata.HistoryCountDLQMessagesResponse} {
		assert.Equal(t, item, ToHistoryCountDLQMessagesResponse(FromHistoryCountDLQMessagesResponse(item)))
	}
}
func TestHistoryMergeDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.MergeDLQMessagesRequest{nil, {}, &testdata.HistoryMergeDLQMessagesRequest} {
		assert.Equal(t, item, ToHistoryMergeDLQMessagesRequest(FromHistoryMergeDLQMessagesRequest(item)))
	}
}
func TestHistoryMergeDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.MergeDLQMessagesResponse{nil, {}, &testdata.HistoryMergeDLQMessagesResponse} {
		assert.Equal(t, item, ToHistoryMergeDLQMessagesResponse(FromHistoryMergeDLQMessagesResponse(item)))
	}
}
func TestHistoryNotifyFailoverMarkersRequest(t *testing.T) {
	for _, item := range []*types.NotifyFailoverMarkersRequest{nil, {}, &testdata.HistoryNotifyFailoverMarkersRequest} {
		assert.Equal(t, item, ToHistoryNotifyFailoverMarkersRequest(FromHistoryNotifyFailoverMarkersRequest(item)))
	}
}
func TestHistoryPollMutableStateRequest(t *testing.T) {
	for _, item := range []*types.PollMutableStateRequest{nil, {}, &testdata.HistoryPollMutableStateRequest} {
		assert.Equal(t, item, ToHistoryPollMutableStateRequest(FromHistoryPollMutableStateRequest(item)))
	}
}
func TestHistoryPollMutableStateResponse(t *testing.T) {
	for _, item := range []*types.PollMutableStateResponse{nil, &testdata.HistoryPollMutableStateResponse} {
		assert.Equal(t, item, ToHistoryPollMutableStateResponse(FromHistoryPollMutableStateResponse(item)))
	}
}
func TestHistoryPurgeDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.PurgeDLQMessagesRequest{nil, {}, &testdata.HistoryPurgeDLQMessagesRequest} {
		assert.Equal(t, item, ToHistoryPurgeDLQMessagesRequest(FromHistoryPurgeDLQMessagesRequest(item)))
	}
}
func TestHistoryQueryWorkflowRequest(t *testing.T) {
	for _, item := range []*types.HistoryQueryWorkflowRequest{nil, {}, &testdata.HistoryQueryWorkflowRequest} {
		assert.Equal(t, item, ToHistoryQueryWorkflowRequest(FromHistoryQueryWorkflowRequest(item)))
	}
}
func TestHistoryQueryWorkflowResponse(t *testing.T) {
	for _, item := range []*types.HistoryQueryWorkflowResponse{nil, &testdata.HistoryQueryWorkflowResponse} {
		assert.Equal(t, item, ToHistoryQueryWorkflowResponse(FromHistoryQueryWorkflowResponse(item)))
	}
	assert.Nil(t, FromHistoryQueryWorkflowResponse(&types.HistoryQueryWorkflowResponse{}))
}
func TestHistoryReadDLQMessagesRequest(t *testing.T) {
	for _, item := range []*types.ReadDLQMessagesRequest{nil, {}, &testdata.HistoryReadDLQMessagesRequest} {
		assert.Equal(t, item, ToHistoryReadDLQMessagesRequest(FromHistoryReadDLQMessagesRequest(item)))
	}
}
func TestHistoryReadDLQMessagesResponse(t *testing.T) {
	for _, item := range []*types.ReadDLQMessagesResponse{nil, {}, &testdata.HistoryReadDLQMessagesResponse} {
		assert.Equal(t, item, ToHistoryReadDLQMessagesResponse(FromHistoryReadDLQMessagesResponse(item)))
	}
}
func TestHistoryReapplyEventsRequest(t *testing.T) {
	for _, item := range []*types.HistoryReapplyEventsRequest{nil, &testdata.HistoryReapplyEventsRequest} {
		assert.Equal(t, item, ToHistoryReapplyEventsRequest(FromHistoryReapplyEventsRequest(item)))
	}
	assert.Nil(t, FromHistoryReapplyEventsRequest(&types.HistoryReapplyEventsRequest{}))
}
func TestHistoryRecordActivityTaskHeartbeatRequest(t *testing.T) {
	for _, item := range []*types.HistoryRecordActivityTaskHeartbeatRequest{nil, {}, &testdata.HistoryRecordActivityTaskHeartbeatRequest} {
		assert.Equal(t, item, ToHistoryRecordActivityTaskHeartbeatRequest(FromHistoryRecordActivityTaskHeartbeatRequest(item)))
	}
}
func TestHistoryRecordActivityTaskHeartbeatResponse(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskHeartbeatResponse{nil, {}, &testdata.HistoryRecordActivityTaskHeartbeatResponse} {
		assert.Equal(t, item, ToHistoryRecordActivityTaskHeartbeatResponse(FromHistoryRecordActivityTaskHeartbeatResponse(item)))
	}
}
func TestHistoryRecordActivityTaskStartedRequest(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskStartedRequest{nil, {}, &testdata.HistoryRecordActivityTaskStartedRequest} {
		assert.Equal(t, item, ToHistoryRecordActivityTaskStartedRequest(FromHistoryRecordActivityTaskStartedRequest(item)))
	}
}
func TestHistoryRecordActivityTaskStartedResponse(t *testing.T) {
	for _, item := range []*types.RecordActivityTaskStartedResponse{nil, {}, &testdata.HistoryRecordActivityTaskStartedResponse} {
		assert.Equal(t, item, ToHistoryRecordActivityTaskStartedResponse(FromHistoryRecordActivityTaskStartedResponse(item)))
	}
}
func TestHistoryRecordChildExecutionCompletedRequest(t *testing.T) {
	for _, item := range []*types.RecordChildExecutionCompletedRequest{nil, {}, &testdata.HistoryRecordChildExecutionCompletedRequest} {
		assert.Equal(t, item, ToHistoryRecordChildExecutionCompletedRequest(FromHistoryRecordChildExecutionCompletedRequest(item)))
	}
}
func TestHistoryRecordDecisionTaskStartedRequest(t *testing.T) {
	for _, item := range []*types.RecordDecisionTaskStartedRequest{nil, {}, &testdata.HistoryRecordDecisionTaskStartedRequest} {
		assert.Equal(t, item, ToHistoryRecordDecisionTaskStartedRequest(FromHistoryRecordDecisionTaskStartedRequest(item)))
	}
}
func TestHistoryRecordDecisionTaskStartedResponse(t *testing.T) {
	for _, item := range []*types.RecordDecisionTaskStartedResponse{nil, {}, &testdata.HistoryRecordDecisionTaskStartedResponse} {
		assert.Equal(t, item, ToHistoryRecordDecisionTaskStartedResponse(FromHistoryRecordDecisionTaskStartedResponse(item)))
	}
}
func TestHistoryRefreshWorkflowTasksRequest(t *testing.T) {
	for _, item := range []*types.HistoryRefreshWorkflowTasksRequest{nil, &testdata.HistoryRefreshWorkflowTasksRequest} {
		assert.Equal(t, item, ToHistoryRefreshWorkflowTasksRequest(FromHistoryRefreshWorkflowTasksRequest(item)))
	}
	assert.Nil(t, FromHistoryRefreshWorkflowTasksRequest(&types.HistoryRefreshWorkflowTasksRequest{}))
}
func TestHistoryRemoveSignalMutableStateRequest(t *testing.T) {
	for _, item := range []*types.RemoveSignalMutableStateRequest{nil, {}, &testdata.HistoryRemoveSignalMutableStateRequest} {
		assert.Equal(t, item, ToHistoryRemoveSignalMutableStateRequest(FromHistoryRemoveSignalMutableStateRequest(item)))
	}
}
func TestHistoryRemoveTaskRequest(t *testing.T) {
	for _, item := range []*types.RemoveTaskRequest{nil, {}, &testdata.HistoryRemoveTaskRequest} {
		assert.Equal(t, item, ToHistoryRemoveTaskRequest(FromHistoryRemoveTaskRequest(item)))
	}
}
func TestHistoryReplicateEventsV2Request(t *testing.T) {
	for _, item := range []*types.ReplicateEventsV2Request{nil, {}, &testdata.HistoryReplicateEventsV2Request} {
		assert.Equal(t, item, ToHistoryReplicateEventsV2Request(FromHistoryReplicateEventsV2Request(item)))
	}
}
func TestHistoryRequestCancelWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistoryRequestCancelWorkflowExecutionRequest{nil, {}, &testdata.HistoryRequestCancelWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistoryRequestCancelWorkflowExecutionRequest(FromHistoryRequestCancelWorkflowExecutionRequest(item)))
	}
}
func TestHistoryResetQueueRequest(t *testing.T) {
	for _, item := range []*types.ResetQueueRequest{nil, {}, &testdata.HistoryResetQueueRequest} {
		assert.Equal(t, item, ToHistoryResetQueueRequest(FromHistoryResetQueueRequest(item)))
	}
}
func TestHistoryResetStickyTaskListRequest(t *testing.T) {
	for _, item := range []*types.HistoryResetStickyTaskListRequest{nil, {}, &testdata.HistoryResetStickyTaskListRequest} {
		assert.Equal(t, item, ToHistoryResetStickyTaskListRequest(FromHistoryResetStickyTaskListRequest(item)))
	}
}
func TestHistoryResetWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistoryResetWorkflowExecutionRequest{nil, {}, &testdata.HistoryResetWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistoryResetWorkflowExecutionRequest(FromHistoryResetWorkflowExecutionRequest(item)))
	}
}
func TestHistoryResetWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.ResetWorkflowExecutionResponse{nil, {}, &testdata.HistoryResetWorkflowExecutionResponse} {
		assert.Equal(t, item, ToHistoryResetWorkflowExecutionResponse(FromHistoryResetWorkflowExecutionResponse(item)))
	}
}
func TestHistoryRespondActivityTaskCanceledRequest(t *testing.T) {
	for _, item := range []*types.HistoryRespondActivityTaskCanceledRequest{nil, {}, &testdata.HistoryRespondActivityTaskCanceledRequest} {
		assert.Equal(t, item, ToHistoryRespondActivityTaskCanceledRequest(FromHistoryRespondActivityTaskCanceledRequest(item)))
	}
}
func TestHistoryRespondActivityTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.HistoryRespondActivityTaskCompletedRequest{nil, {}, &testdata.HistoryRespondActivityTaskCompletedRequest} {
		assert.Equal(t, item, ToHistoryRespondActivityTaskCompletedRequest(FromHistoryRespondActivityTaskCompletedRequest(item)))
	}
}
func TestHistoryRespondActivityTaskFailedRequest(t *testing.T) {
	for _, item := range []*types.HistoryRespondActivityTaskFailedRequest{nil, {}, &testdata.HistoryRespondActivityTaskFailedRequest} {
		assert.Equal(t, item, ToHistoryRespondActivityTaskFailedRequest(FromHistoryRespondActivityTaskFailedRequest(item)))
	}
}
func TestHistoryRespondDecisionTaskCompletedRequest(t *testing.T) {
	for _, item := range []*types.HistoryRespondDecisionTaskCompletedRequest{nil, {}, &testdata.HistoryRespondDecisionTaskCompletedRequest} {
		assert.Equal(t, item, ToHistoryRespondDecisionTaskCompletedRequest(FromHistoryRespondDecisionTaskCompletedRequest(item)))
	}
}
func TestHistoryRespondDecisionTaskCompletedResponse(t *testing.T) {
	for _, item := range []*types.HistoryRespondDecisionTaskCompletedResponse{nil, {}, &testdata.HistoryRespondDecisionTaskCompletedResponse} {
		assert.Equal(t, item, ToHistoryRespondDecisionTaskCompletedResponse(FromHistoryRespondDecisionTaskCompletedResponse(item)))
	}
}
func TestHistoryRespondDecisionTaskFailedRequest(t *testing.T) {
	for _, item := range []*types.HistoryRespondDecisionTaskFailedRequest{nil, {}, &testdata.HistoryRespondDecisionTaskFailedRequest} {
		assert.Equal(t, item, ToHistoryRespondDecisionTaskFailedRequest(FromHistoryRespondDecisionTaskFailedRequest(item)))
	}
}
func TestHistoryScheduleDecisionTaskRequest(t *testing.T) {
	for _, item := range []*types.ScheduleDecisionTaskRequest{nil, {}, &testdata.HistoryScheduleDecisionTaskRequest} {
		assert.Equal(t, item, ToHistoryScheduleDecisionTaskRequest(FromHistoryScheduleDecisionTaskRequest(item)))
	}
}
func TestHistorySignalWithStartWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistorySignalWithStartWorkflowExecutionRequest{nil, {}, &testdata.HistorySignalWithStartWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistorySignalWithStartWorkflowExecutionRequest(FromHistorySignalWithStartWorkflowExecutionRequest(item)))
	}
}
func TestHistorySignalWithStartWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionResponse{nil, {}, &testdata.HistorySignalWithStartWorkflowExecutionResponse} {
		assert.Equal(t, item, ToHistorySignalWithStartWorkflowExecutionResponse(FromHistorySignalWithStartWorkflowExecutionResponse(item)))
	}
}
func TestHistorySignalWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistorySignalWorkflowExecutionRequest{nil, {}, &testdata.HistorySignalWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistorySignalWorkflowExecutionRequest(FromHistorySignalWorkflowExecutionRequest(item)))
	}
}
func TestHistoryStartWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistoryStartWorkflowExecutionRequest{nil, {}, &testdata.HistoryStartWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistoryStartWorkflowExecutionRequest(FromHistoryStartWorkflowExecutionRequest(item)))
	}
}
func TestHistoryStartWorkflowExecutionResponse(t *testing.T) {
	for _, item := range []*types.StartWorkflowExecutionResponse{nil, {}, &testdata.HistoryStartWorkflowExecutionResponse} {
		assert.Equal(t, item, ToHistoryStartWorkflowExecutionResponse(FromHistoryStartWorkflowExecutionResponse(item)))
	}
}
func TestHistorySyncActivityRequest(t *testing.T) {
	for _, item := range []*types.SyncActivityRequest{nil, {}, &testdata.HistorySyncActivityRequest} {
		assert.Equal(t, item, ToHistorySyncActivityRequest(FromHistorySyncActivityRequest(item)))
	}
}
func TestHistorySyncShardStatusRequest(t *testing.T) {
	for _, item := range []*types.SyncShardStatusRequest{nil, {}, &testdata.HistorySyncShardStatusRequest} {
		assert.Equal(t, item, ToHistorySyncShardStatusRequest(FromHistorySyncShardStatusRequest(item)))
	}
}
func TestHistoryTerminateWorkflowExecutionRequest(t *testing.T) {
	for _, item := range []*types.HistoryTerminateWorkflowExecutionRequest{nil, {}, &testdata.HistoryTerminateWorkflowExecutionRequest} {
		assert.Equal(t, item, ToHistoryTerminateWorkflowExecutionRequest(FromHistoryTerminateWorkflowExecutionRequest(item)))
	}
}

func TestHistoryGetCrossClusterTasksRequest(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksRequest{nil, {}, &testdata.HistoryGetCrossClusterTasksRequest} {
		assert.Equal(t, item, ToHistoryGetCrossClusterTasksRequest(FromHistoryGetCrossClusterTasksRequest(item)))
	}
}

func TestHistoryGetCrossClusterTasksResponse(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksResponse{nil, {}, &testdata.HistoryGetCrossClusterTasksResponse} {
		assert.Equal(t, item, ToHistoryGetCrossClusterTasksResponse(FromHistoryGetCrossClusterTasksResponse(item)))
	}
}

func TestHistoryRespondCrossClusterTasksCompletedRequest(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedRequest{nil, {}, &testdata.HistoryRespondCrossClusterTasksCompletedRequest} {
		assert.Equal(t, item, ToHistoryRespondCrossClusterTasksCompletedRequest(FromHistoryRespondCrossClusterTasksCompletedRequest(item)))
	}
}

func TestHistoryRespondCrossClusterTasksCompletedResponse(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedResponse{nil, {}, &testdata.HistoryRespondCrossClusterTasksCompletedResponse} {
		assert.Equal(t, item, ToHistoryRespondCrossClusterTasksCompletedResponse(FromHistoryRespondCrossClusterTasksCompletedResponse(item)))
	}
}

func TestHistoryGetTaskInfoRequest(t *testing.T) {
	for _, item := range []*types.GetFailoverInfoRequest{nil, {}, &testdata.GetFailoverInfoRequest} {
		assert.Equal(t, item, ToHistoryGetFailoverInfoRequest(FromHistoryGetFailoverInfoRequest(item)))
	}
}

func TestHistoryGetTaskInfoResponse(t *testing.T) {
	for _, item := range []*types.GetFailoverInfoResponse{nil, {}, &testdata.GetFailoverInfoResponse} {
		assert.Equal(t, item, ToHistoryGetFailoverInfoResponse(FromHistoryGetFailoverInfoResponse(item)))
	}
}

func TestRatelimitUpdate(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		for _, item := range []*types.RatelimitUpdateResponse{nil, {}, &testdata.RatelimitUpdateResponse} {
			assert.Equal(t, item, ToHistoryRatelimitUpdateResponse(FromHistoryRatelimitUpdateResponse(item)))
		}
		for _, item := range []*types.RatelimitUpdateRequest{nil, {}, &testdata.RatelimitUpdateRequest} {
			assert.Equal(t, item, ToHistoryRatelimitUpdateRequest(FromHistoryRatelimitUpdateRequest(item)))
		}
	})

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, ToHistoryRatelimitUpdateRequest(nil), "request to internal")
		assert.Nil(t, FromHistoryRatelimitUpdateResponse(nil), "response from internal")
	})

	t.Run("nil Any contents", func(t *testing.T) {
		assert.Equal(t, &types.RatelimitUpdateRequest{Any: nil}, ToHistoryRatelimitUpdateRequest(&historyv1.RatelimitUpdateRequest{Data: nil}), "request to internal")
		assert.Equal(t, &historyv1.RatelimitUpdateResponse{Data: nil}, FromHistoryRatelimitUpdateResponse(&types.RatelimitUpdateResponse{Any: nil}), "response from internal")
	})

	t.Run("with Any contents", func(t *testing.T) {
		internal := &types.Any{ValueType: "test", Value: []byte(`test data`)}
		proto := &sharedv1.Any{ValueType: "test", Value: []byte(`test data`)}
		assert.Equal(t, &types.RatelimitUpdateRequest{Any: internal}, ToHistoryRatelimitUpdateRequest(&historyv1.RatelimitUpdateRequest{Data: proto}), "request to internal")
		assert.Equal(t, &historyv1.RatelimitUpdateResponse{Data: proto}, FromHistoryRatelimitUpdateResponse(&types.RatelimitUpdateResponse{Any: internal}), "response from internal")
	})
}

// --- Fuzz tests for history mapper functions ---

// HistoryStartWorkflowExecutionRequestFuzzer ensures ContinuedFailureReason is non-nil
// when ContinuedFailureDetails is non-nil. FromFailure(nil, details) returns nil (drops
// details when reason is nil), so a nil reason with non-nil details breaks the round-trip.
func HistoryStartWorkflowExecutionRequestFuzzer(r *types.HistoryStartWorkflowExecutionRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.ContinuedFailureReason == nil && len(r.ContinuedFailureDetails) > 0 {
		reason := ""
		r.ContinuedFailureReason = &reason
	}
}

// HistoryQueryWorkflowResponseFuzzer ensures Response is always non-nil.
// FromHistoryQueryWorkflowResponse returns nil when t.Response is nil, which breaks
// the round-trip (To(nil) == nil != &{Response: nil}).
func HistoryQueryWorkflowResponseFuzzer(r *types.HistoryQueryWorkflowResponse, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.Response == nil {
		r.Response = &types.QueryWorkflowResponse{}
	}
}

// HistoryReapplyEventsRequestFuzzer ensures Request is always non-nil.
// FromHistoryReapplyEventsRequest returns nil when t.Request is nil, which breaks
// the round-trip (To(nil) == nil != &{Request: nil}).
func HistoryReapplyEventsRequestFuzzer(r *types.HistoryReapplyEventsRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.Request == nil {
		r.Request = &types.ReapplyEventsRequest{}
	}
}

// RespondActivityTaskFailedRequestFuzzer ensures Reason is non-nil when
// Details is non-nil. FromFailure(nil, details) returns nil (drops details when
// reason is nil), so a nil Reason with non-nil Details breaks the round-trip.
func RespondActivityTaskFailedRequestFuzzer(r *types.RespondActivityTaskFailedRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.Reason == nil && len(r.Details) > 0 {
		reason := ""
		r.Reason = &reason
	}
}

// SyncActivityRequestFuzzer ensures LastFailureReason is non-nil when
// LastFailureDetails is non-nil. FromFailure(nil, details) returns nil (drops
// details when reason is nil), so a nil reason with non-nil details breaks the
// round-trip.
func SyncActivityRequestFuzzer(r *types.SyncActivityRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.LastFailureReason == nil && len(r.LastFailureDetails) > 0 {
		reason := ""
		r.LastFailureReason = &reason
	}
}

// HistoryRefreshWorkflowTasksRequestFuzzer ensures Request is always non-nil.
// FromHistoryRefreshWorkflowTasksRequest returns nil when t.Request is nil, which breaks
// the round-trip (To(nil) == nil != &{Request: nil}).
func HistoryRefreshWorkflowTasksRequestFuzzer(r *types.HistoryRefreshWorkflowTasksRequest, c fuzz.Continue) {
	c.FuzzNoCustom(r)
	if r.Request == nil {
		r.Request = &types.RefreshWorkflowTasksRequest{}
	}
}

func TestHistoryCloseShardRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryCloseShardRequest, ToHistoryCloseShardRequest)
}

func TestHistoryDescribeHistoryHostResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryDescribeHistoryHostResponse, ToHistoryDescribeHistoryHostResponse)
}

func TestHistoryDescribeMutableStateRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryDescribeMutableStateRequest, ToHistoryDescribeMutableStateRequest)
}

func TestHistoryDescribeMutableStateResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryDescribeMutableStateResponse, ToHistoryDescribeMutableStateResponse)
}

func TestHistoryDescribeWorkflowExecutionRequestFuzz(t *testing.T) {
	// DescribeWorkflowExecutionRequest.QueryConsistencyLevel is an enum (0-1);
	// out-of-range values map to nil on the return path.
	testutils.RunMapperFuzzTest(t, FromHistoryDescribeWorkflowExecutionRequest, ToHistoryDescribeWorkflowExecutionRequest,
		testutils.WithCustomFuncs(QueryConsistencyLevelFuzzer),
	)
}

func TestHistoryGetDLQReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetDLQReplicationMessagesRequest, ToHistoryGetDLQReplicationMessagesRequest)
}

func TestHistoryGetFailoverInfoRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetFailoverInfoRequest, ToHistoryGetFailoverInfoRequest)
}

func TestHistoryGetFailoverInfoResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetFailoverInfoResponse, ToHistoryGetFailoverInfoResponse)
}

func TestHistoryGetMutableStateRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetMutableStateRequest, ToHistoryGetMutableStateRequest)
}

func TestHistoryGetReplicationMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetReplicationMessagesRequest, ToHistoryGetReplicationMessagesRequest)
}

func TestHistoryCountDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryCountDLQMessagesRequest, ToHistoryCountDLQMessagesRequest)
}

func TestHistoryCountDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryCountDLQMessagesResponse, ToHistoryCountDLQMessagesResponse)
}

func TestHistoryNotifyFailoverMarkersRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryNotifyFailoverMarkersRequest, ToHistoryNotifyFailoverMarkersRequest)
}

func TestHistoryPollMutableStateRequestFuzz(t *testing.T) {
	// [BUG] VersionHistoryItem exists in types.PollMutableStateRequest but is not mapped
	// in FromHistoryPollMutableStateRequest; it is silently dropped.
	testutils.RunMapperFuzzTest(t, FromHistoryPollMutableStateRequest, ToHistoryPollMutableStateRequest,
		testutils.WithExcludedFields("VersionHistoryItem"),
	)
}

func TestHistoryRecordActivityTaskHeartbeatRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRecordActivityTaskHeartbeatRequest, ToHistoryRecordActivityTaskHeartbeatRequest)
}

func TestHistoryRecordActivityTaskHeartbeatResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRecordActivityTaskHeartbeatResponse, ToHistoryRecordActivityTaskHeartbeatResponse)
}

func TestHistoryRecordActivityTaskStartedRequestFuzz(t *testing.T) {
	// Contains PollForActivityTaskRequest which has TaskList (TaskListKind, TaskListType).
	testutils.RunMapperFuzzTest(t, FromHistoryRecordActivityTaskStartedRequest, ToHistoryRecordActivityTaskStartedRequest,
		testutils.WithCommonEnumFuzzers(),
	)
}

func TestHistoryRecordChildExecutionCompletedRequestFuzz(t *testing.T) {
	// CompletionEvent is a HistoryEvent requiring complex enum handling;
	// it is tested comprehensively in api_test.go (TestHistoryEventFuzz).
	testutils.RunMapperFuzzTest(t, FromHistoryRecordChildExecutionCompletedRequest, ToHistoryRecordChildExecutionCompletedRequest,
		testutils.WithExcludedFields("CompletionEvent"),
	)
}

func TestHistoryRecordDecisionTaskStartedRequestFuzz(t *testing.T) {
	// Contains PollForDecisionTaskRequest which has TaskList (TaskListKind, TaskListType).
	testutils.RunMapperFuzzTest(t, FromHistoryRecordDecisionTaskStartedRequest, ToHistoryRecordDecisionTaskStartedRequest,
		testutils.WithCommonEnumFuzzers(),
	)
}

func TestHistoryRemoveSignalMutableStateRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRemoveSignalMutableStateRequest, ToHistoryRemoveSignalMutableStateRequest)
}

func TestHistoryReplicateEventsV2RequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryReplicateEventsV2Request, ToHistoryReplicateEventsV2Request,
		testutils.WithCustomFuncs(testutils.EncodingTypeFuzzer),
	)
}

func TestHistoryRequestCancelWorkflowExecutionRequestFuzz(t *testing.T) {
	// [BUG] ExternalInitiatedEventID: nil input maps to &0 (non-nil) via ToExternalInitiatedID.
	testutils.RunMapperFuzzTest(t, FromHistoryRequestCancelWorkflowExecutionRequest, ToHistoryRequestCancelWorkflowExecutionRequest,
		testutils.WithExcludedFields("ExternalInitiatedEventID"),
	)
}

func TestHistoryResetStickyTaskListRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryResetStickyTaskListRequest, ToHistoryResetStickyTaskListRequest)
}

func TestHistoryResetWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryResetWorkflowExecutionRequest, ToHistoryResetWorkflowExecutionRequest)
}

func TestHistoryResetWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryResetWorkflowExecutionResponse, ToHistoryResetWorkflowExecutionResponse)
}

func TestHistoryRespondActivityTaskCanceledRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRespondActivityTaskCanceledRequest, ToHistoryRespondActivityTaskCanceledRequest)
}

func TestHistoryRespondActivityTaskCompletedRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRespondActivityTaskCompletedRequest, ToHistoryRespondActivityTaskCompletedRequest)
}

func TestHistoryRespondActivityTaskFailedRequestFuzz(t *testing.T) {
	// [BUG] FromFailure(nil, details) returns nil when Reason is nil, silently dropping Details.
	// RespondActivityTaskFailedRequestFuzzer ensures Reason is non-nil when Details is set.
	testutils.RunMapperFuzzTest(t, FromHistoryRespondActivityTaskFailedRequest, ToHistoryRespondActivityTaskFailedRequest,
		testutils.WithCustomFuncs(RespondActivityTaskFailedRequestFuzzer),
	)
}

func TestHistoryRespondDecisionTaskCompletedResponseFuzz(t *testing.T) {
	// StartedResponse is a RecordDecisionTaskStartedResponse; tested separately in
	// TestHistoryRecordDecisionTaskStartedResponseFuzz.
	testutils.RunMapperFuzzTest(t, FromHistoryRespondDecisionTaskCompletedResponse, ToHistoryRespondDecisionTaskCompletedResponse,
		testutils.WithExcludedFields("StartedResponse"),
	)
}

func TestHistoryRespondDecisionTaskFailedRequestFuzz(t *testing.T) {
	// DecisionTaskFailedCause has 23 valid values (0-22); out-of-range values map to nil.
	testutils.RunMapperFuzzTest(t, FromHistoryRespondDecisionTaskFailedRequest, ToHistoryRespondDecisionTaskFailedRequest,
		testutils.WithCustomFuncs(DecisionTaskFailedCauseFuzzer),
	)
}

func TestHistoryScheduleDecisionTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryScheduleDecisionTaskRequest, ToHistoryScheduleDecisionTaskRequest)
}

func TestHistorySignalWithStartWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistorySignalWithStartWorkflowExecutionResponse, ToHistorySignalWithStartWorkflowExecutionResponse)
}

func TestHistorySignalWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistorySignalWorkflowExecutionRequest, ToHistorySignalWorkflowExecutionRequest)
}

func TestHistoryStartWorkflowExecutionResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryStartWorkflowExecutionResponse, ToHistoryStartWorkflowExecutionResponse)
}

func TestHistoryTerminateWorkflowExecutionRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryTerminateWorkflowExecutionRequest, ToHistoryTerminateWorkflowExecutionRequest)
}

func TestHistoryGetCrossClusterTasksRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryGetCrossClusterTasksRequest, ToHistoryGetCrossClusterTasksRequest)
}

func TestHistoryRespondCrossClusterTasksCompletedRequestFuzz(t *testing.T) {
	// TaskResponses contains CrossClusterTaskResponses with oneof constraints tested in shared_test.go.
	testutils.RunMapperFuzzTest(t, FromHistoryRespondCrossClusterTasksCompletedRequest, ToHistoryRespondCrossClusterTasksCompletedRequest,
		testutils.WithExcludedFields("TaskResponses"),
	)
}

func TestHistoryRatelimitUpdateRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRatelimitUpdateRequest, ToHistoryRatelimitUpdateRequest)
}

func TestHistoryRatelimitUpdateResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRatelimitUpdateResponse, ToHistoryRatelimitUpdateResponse)
}

func TestHistorySyncActivityRequestFuzz(t *testing.T) {
	// [BUG] LastFailureReason + LastFailureDetails merge into LastFailure (Failure object);
	// FromFailure(nil, details) drops details when reason is nil, breaking the round-trip.
	// SyncActivityRequestFuzzer ensures LastFailureReason is non-nil when Details is set.
	// WorkflowID + RunID merge into WorkflowExecution and split back correctly.
	testutils.RunMapperFuzzTest(t, FromHistorySyncActivityRequest, ToHistorySyncActivityRequest,
		testutils.WithCustomFuncs(SyncActivityRequestFuzzer),
	)
}

func TestHistoryMergeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryMergeDLQMessagesRequest, ToHistoryMergeDLQMessagesRequest,
		testutils.WithCustomFuncs(testutils.DLQTypeFuzzer),
	)
}

func TestHistoryMergeDLQMessagesResponseFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryMergeDLQMessagesResponse, ToHistoryMergeDLQMessagesResponse)
}

func TestHistoryPurgeDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryPurgeDLQMessagesRequest, ToHistoryPurgeDLQMessagesRequest,
		testutils.WithCustomFuncs(testutils.DLQTypeFuzzer),
	)
}

func TestHistoryReadDLQMessagesRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryReadDLQMessagesRequest, ToHistoryReadDLQMessagesRequest,
		testutils.WithCustomFuncs(testutils.DLQTypeFuzzer),
	)
}

func TestHistoryDescribeQueueRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryDescribeQueueRequest, ToHistoryDescribeQueueRequest,
		testutils.WithCustomFuncs(testutils.TaskTypeFuzzer),
	)
}

func TestHistoryRemoveTaskRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryRemoveTaskRequest, ToHistoryRemoveTaskRequest,
		testutils.WithCustomFuncs(testutils.TaskTypeFuzzer),
	)
}

func TestHistoryResetQueueRequestFuzz(t *testing.T) {
	testutils.RunMapperFuzzTest(t, FromHistoryResetQueueRequest, ToHistoryResetQueueRequest,
		testutils.WithCustomFuncs(testutils.TaskTypeFuzzer),
	)
}

func TestHistoryGetMutableStateResponseFuzz(t *testing.T) {
	// [BUG] WorkflowCloseState: nil input maps to a non-nil *int32 on To (always returns Int32Ptr).
	// IsWorkflowRunning is derived from WorkflowState == WORKFLOW_STATE_RUNNING on To and not
	// stored on From; excluding it avoids a false round-trip failure.
	// WorkflowStateFuzzer constrains WorkflowState (*int32) to valid values (0-5) to prevent
	// out-of-range proto enum mapping to INVALID breaking the round-trip.
	testutils.RunMapperFuzzTest(t, FromHistoryGetMutableStateResponse, ToHistoryGetMutableStateResponse,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithCustomFuncs(testutils.WorkflowStateFuzzer),
		testutils.WithExcludedFields("IsWorkflowRunning", "WorkflowCloseState"),
	)
}

func TestHistoryPollMutableStateResponseFuzz(t *testing.T) {
	// [BUG] WorkflowCloseState: nil input maps to a non-nil *int32 on To (always returns Int32Ptr).
	// WorkflowStateFuzzer constrains WorkflowState (*int32) to valid values (0-5).
	testutils.RunMapperFuzzTest(t, FromHistoryPollMutableStateResponse, ToHistoryPollMutableStateResponse,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithCustomFuncs(testutils.WorkflowStateFuzzer),
		testutils.WithExcludedFields("WorkflowCloseState"),
	)
}

func TestHistoryRecordActivityTaskStartedResponseFuzz(t *testing.T) {
	// [BUG] Attempt is int64 in types but int32 in proto; values outside int32 range are truncated.
	// ScheduledEvent is a HistoryEvent requiring complex enum handling;
	// it is tested comprehensively in api_test.go (TestHistoryEventFuzz).
	testutils.RunMapperFuzzTest(t, FromHistoryRecordActivityTaskStartedResponse, ToHistoryRecordActivityTaskStartedResponse,
		testutils.WithExcludedFields("Attempt", "ScheduledEvent"),
	)
}

func TestHistoryRecordDecisionTaskStartedResponseFuzz(t *testing.T) {
	// [BUG] Attempt is int64 in types but int32 in proto; values outside int32 range are truncated.
	// DecisionInfo contains TransientDecisionInfo which has HistoryEvent fields requiring complex
	// enum handling; it is tested comprehensively in api_test.go (TestHistoryEventFuzz).
	// WorkflowExecutionTaskList has TaskListKind/TaskListType enums; WithCommonEnumFuzzers covers them.
	testutils.RunMapperFuzzTest(t, FromHistoryRecordDecisionTaskStartedResponse, ToHistoryRecordDecisionTaskStartedResponse,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithExcludedFields("Attempt", "DecisionInfo"),
	)
}

func TestHistorySyncShardStatusRequestFuzz(t *testing.T) {
	// [BUG] ShardID is int64 in types but int32 in proto; values outside int32 range are truncated.
	testutils.RunMapperFuzzTest(t, FromHistorySyncShardStatusRequest, ToHistorySyncShardStatusRequest,
		testutils.WithExcludedFields("ShardID"),
	)
}

func TestHistoryQueryWorkflowResponseFuzz(t *testing.T) {
	// FromHistoryQueryWorkflowResponse returns nil when t.Response == nil, so round-trip
	// fails for nil Response. HistoryQueryWorkflowResponseFuzzer ensures Response is always set.
	// QueryRejected contains QueryRejectCondition which needs a constrained fuzzer.
	testutils.RunMapperFuzzTest(t, FromHistoryQueryWorkflowResponse, ToHistoryQueryWorkflowResponse,
		testutils.WithCustomFuncs(HistoryQueryWorkflowResponseFuzzer, QueryRejectConditionFuzzer),
	)
}

func TestHistoryReapplyEventsRequestFuzz(t *testing.T) {
	// FromHistoryReapplyEventsRequest returns nil when t.Request == nil, so round-trip
	// fails for nil Request. HistoryReapplyEventsRequestFuzzer ensures Request is always set.
	testutils.RunMapperFuzzTest(t, FromHistoryReapplyEventsRequest, ToHistoryReapplyEventsRequest,
		testutils.WithCustomFuncs(HistoryReapplyEventsRequestFuzzer, testutils.EncodingTypeFuzzer),
	)
}

func TestHistoryRefreshWorkflowTasksRequestFuzz(t *testing.T) {
	// FromHistoryRefreshWorkflowTasksRequest returns nil when t.Request == nil, so round-trip
	// fails for nil Request. HistoryRefreshWorkflowTasksRequestFuzzer ensures Request is always set.
	testutils.RunMapperFuzzTest(t, FromHistoryRefreshWorkflowTasksRequest, ToHistoryRefreshWorkflowTasksRequest,
		testutils.WithCustomFuncs(HistoryRefreshWorkflowTasksRequestFuzzer),
	)
}

func TestHistoryStartWorkflowExecutionRequestFuzz(t *testing.T) {
	// [BUG] StartRequest contains WorkflowIDReusePolicy (out-of-range → nil) and
	// ExternalEntityType/ExternalEntityKey fields not mapped through proto (silently dropped).
	// Exclude StartRequest; it is tested via api_test.go.
	// ContinueAsNewInitiator is an enum (0-2); HistoryContinueAsNewInitiatorFuzzer constrains it.
	// [BUG] ContinuedFailureDetails dropped when ContinuedFailureReason is nil (FromFailure);
	// HistoryStartWorkflowExecutionRequestFuzzer ensures reason is non-nil when details are set.
	testutils.RunMapperFuzzTest(t, FromHistoryStartWorkflowExecutionRequest, ToHistoryStartWorkflowExecutionRequest,
		testutils.WithCommonEnumFuzzers(),
		testutils.WithCustomFuncs(ContinueAsNewInitiatorFuzzer, HistoryStartWorkflowExecutionRequestFuzzer),
		testutils.WithExcludedFields("StartRequest"),
	)
}
