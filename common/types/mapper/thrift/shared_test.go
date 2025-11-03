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

package thrift

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/testing/testdatagen"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestCrossClusterTaskInfoConversion(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskInfo{nil, {}, &testdata.CrossClusterTaskInfo} {
		assert.Equal(t, item, ToCrossClusterTaskInfo(FromCrossClusterTaskInfo(item)))
	}
}

func TestCrossClusterTaskRequestConversion(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskRequest{
		nil,
		{},
		&testdata.CrossClusterTaskRequestStartChildExecution,
		&testdata.CrossClusterTaskRequestCancelExecution,
		&testdata.CrossClusterTaskRequestSignalExecution,
	} {
		assert.Equal(t, item, ToCrossClusterTaskRequest(FromCrossClusterTaskRequest(item)))
	}
}

func TestCrossClusterTaskResponseConversion(t *testing.T) {
	for _, item := range []*types.CrossClusterTaskResponse{
		nil,
		{},
		&testdata.CrossClusterTaskResponseStartChildExecution,
		&testdata.CrossClusterTaskResponseCancelExecution,
		&testdata.CrossClusterTaskResponseSignalExecution,
	} {
		assert.Equal(t, item, ToCrossClusterTaskResponse(FromCrossClusterTaskResponse(item)))
	}
}

func TestCrossClusterTaskRequestArrayConversion(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestArray} {
		assert.Equal(t, item, ToCrossClusterTaskRequestArray(FromCrossClusterTaskRequestArray(item)))
	}
}

func TestCrossClusterTaskResponseArrayConversion(t *testing.T) {
	for _, item := range [][]*types.CrossClusterTaskResponse{nil, {}, testdata.CrossClusterTaskResponseArray} {
		assert.Equal(t, item, ToCrossClusterTaskResponseArray(FromCrossClusterTaskResponseArray(item)))
	}
}

func TestCrossClusterTaskRequestMapConversion(t *testing.T) {
	for _, item := range []map[int32][]*types.CrossClusterTaskRequest{nil, {}, testdata.CrossClusterTaskRequestMap} {
		assert.Equal(t, item, ToCrossClusterTaskRequestMap(FromCrossClusterTaskRequestMap(item)))
	}
}

func TestGetTaskFailedCauseMapConversion(t *testing.T) {
	for _, item := range []map[int32]types.GetTaskFailedCause{nil, {}, testdata.GetCrossClusterTaskFailedCauseMap} {
		assert.Equal(t, item, ToGetTaskFailedCauseMap(FromGetTaskFailedCauseMap(item)))
	}
}

func TestGetCrossClusterTasksRequestConversion(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksRequest{nil, {}, &testdata.GetCrossClusterTasksRequest} {
		assert.Equal(t, item, ToAdminGetCrossClusterTasksRequest(FromAdminGetCrossClusterTasksRequest(item)))
	}
}

func TestGetCrossClusterTasksResponseConversion(t *testing.T) {
	for _, item := range []*types.GetCrossClusterTasksResponse{nil, {}, &testdata.GetCrossClusterTasksResponse} {
		assert.Equal(t, item, ToAdminGetCrossClusterTasksResponse(FromAdminGetCrossClusterTasksResponse(item)))
	}
}

func TestRespondCrossClusterTasksCompletedRequestConversion(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedRequest{nil, {}, &testdata.RespondCrossClusterTasksCompletedRequest} {
		assert.Equal(t, item, ToAdminRespondCrossClusterTasksCompletedRequest(FromAdminRespondCrossClusterTasksCompletedRequest(item)))
	}
}

func TestRespondCrossClusterTasksCompletedResponseConversion(t *testing.T) {
	for _, item := range []*types.RespondCrossClusterTasksCompletedResponse{nil, {}, &testdata.RespondCrossClusterTasksCompletedResponse} {
		assert.Equal(t, item, ToAdminRespondCrossClusterTasksCompletedResponse(FromAdminRespondCrossClusterTasksCompletedResponse(item)))
	}
}

func TestGetFailoverInfoRequestConversion(t *testing.T) {
	for _, item := range []*types.GetFailoverInfoRequest{nil, {}, &testdata.GetFailoverInfoRequest} {
		assert.Equal(t, item, ToGetFailoverInfoRequest(FromGetFailoverInfoRequest(item)))
	}
}

func TestGetFailoverInfoResponseConversion(t *testing.T) {
	for _, item := range []*types.GetFailoverInfoResponse{nil, {}, &testdata.GetFailoverInfoResponse} {
		assert.Equal(t, item, ToGetFailoverInfoResponse(FromGetFailoverInfoResponse(item)))
	}
}

func TestCrossClusterApplyParentClosePolicyRequestAttributesConversion(t *testing.T) {
	item := testdata.CrossClusterApplyParentClosePolicyRequestAttributes
	assert.Equal(
		t,
		&item,
		ToCrossClusterApplyParentClosePolicyRequestAttributes(
			FromCrossClusterApplyParentClosePolicyRequestAttributes(&item),
		),
	)
}

func TestApplyParentClosePolicyAttributesConversion(t *testing.T) {
	item := testdata.ApplyParentClosePolicyAttributes
	assert.Equal(
		t,
		&item,
		ToApplyParentClosePolicyAttributes(
			FromApplyParentClosePolicyAttributes(&item),
		),
	)
}

func TestApplyParentClosePolicyResultConversion(t *testing.T) {
	item := testdata.ApplyParentClosePolicyResult
	assert.Equal(
		t,
		&item,
		ToApplyParentClosePolicyResult(
			FromApplyParentClosePolicyResult(&item),
		),
	)
}

func TestCrossClusterApplyParentClosePolicyResponseConversion(t *testing.T) {
	item := testdata.CrossClusterApplyParentClosePolicyResponseWithChildren
	assert.Equal(
		t,
		&item,
		ToCrossClusterApplyParentClosePolicyResponseAttributes(
			FromCrossClusterApplyParentClosePolicyResponseAttributes(&item),
		),
	)
}

func TestIsolationGroupToDomainBlobConversion(t *testing.T) {
	zone1 := "zone-1"
	zone2 := "zone-2"
	drained := shared.IsolationGroupStateDrained
	healthy := shared.IsolationGroupStateHealthy

	tests := map[string]struct {
		in          *types.IsolationGroupConfiguration
		expectedOut *shared.IsolationGroupConfiguration
	}{
		"valid input": {
			in: &types.IsolationGroupConfiguration{
				"zone-1": {
					Name:  zone1,
					State: types.IsolationGroupStateDrained,
				},
				"zone-2": {
					Name:  zone2,
					State: types.IsolationGroupStateHealthy,
				},
			},
			expectedOut: &shared.IsolationGroupConfiguration{
				IsolationGroups: []*shared.IsolationGroupPartition{
					{
						Name:  &zone1,
						State: &drained,
					},
					{
						Name:  &zone2,
						State: &healthy,
					},
				},
			},
		},
		"empty input": {
			in:          &types.IsolationGroupConfiguration{},
			expectedOut: &shared.IsolationGroupConfiguration{},
		},
		"nil input": {
			in:          nil,
			expectedOut: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			out := FromIsolationGroupConfig(td.in)
			assert.Equal(t, td.expectedOut, out)
			roundTrip := ToIsolationGroupConfig(out)
			assert.Equal(t, td.in, roundTrip)
		})
	}
}

func TestIsolationGroupFromDomainBlobConversion(t *testing.T) {
	zone1 := "zone-1"
	zone2 := "zone-2"
	drained := shared.IsolationGroupStateDrained
	healthy := shared.IsolationGroupStateHealthy

	tests := map[string]struct {
		in          *shared.IsolationGroupConfiguration
		expectedOut *types.IsolationGroupConfiguration
	}{
		"valid input": {
			in: &shared.IsolationGroupConfiguration{
				IsolationGroups: []*shared.IsolationGroupPartition{
					{
						Name:  &zone1,
						State: &drained,
					},
					{
						Name:  &zone2,
						State: &healthy,
					},
				},
			},
			expectedOut: &types.IsolationGroupConfiguration{
				"zone-1": {
					Name:  zone1,
					State: types.IsolationGroupStateDrained,
				},
				"zone-2": {
					Name:  zone2,
					State: types.IsolationGroupStateHealthy,
				},
			},
		},
		"empty input": {
			in:          &shared.IsolationGroupConfiguration{},
			expectedOut: &types.IsolationGroupConfiguration{},
		},
		"nil input": {
			in:          nil,
			expectedOut: nil,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			out := ToIsolationGroupConfig(td.in)
			assert.Equal(t, td.expectedOut, out)
			roundTrip := FromIsolationGroupConfig(out)
			assert.Equal(t, td.in, roundTrip)
		})
	}
}

func TestWorkflowExecutionInfoConversion(t *testing.T) {
	for _, item := range []*types.WorkflowExecutionInfo{nil, {}, &testdata.WorkflowExecutionInfo, &testdata.CronWorkflowExecutionInfo, &testdata.WorkflowExecutionInfoEphemeral} {
		assert.Equal(t, item, ToWorkflowExecutionInfo(FromWorkflowExecutionInfo(item)))
	}
}

func TestWorkflowExecutionInfo_MigrateTaskList(t *testing.T) {
	tlName := "foo"
	otherName := "bar"
	cases := []struct {
		name string
		in   *shared.WorkflowExecutionInfo
		out  *types.WorkflowExecutionInfo
	}{
		{
			name: "nil",
			in:   &shared.WorkflowExecutionInfo{},
			out: &types.WorkflowExecutionInfo{
				TaskList: nil,
			},
		},
		{
			name: "name only",
			in: &shared.WorkflowExecutionInfo{
				TaskList: &tlName,
			},
			out: &types.WorkflowExecutionInfo{
				TaskList: &types.TaskList{Name: tlName, Kind: types.TaskListKindNormal.Ptr()},
			},
		},
		{
			name: "tl only",
			in: &shared.WorkflowExecutionInfo{
				TaskListInfo: &shared.TaskList{Name: &tlName, Kind: shared.TaskListKindNormal.Ptr()},
			},
			out: &types.WorkflowExecutionInfo{
				TaskList: &types.TaskList{Name: tlName, Kind: types.TaskListKindNormal.Ptr()},
			},
		},
		{
			name: "both",
			in: &shared.WorkflowExecutionInfo{
				TaskList:     &otherName,
				TaskListInfo: &shared.TaskList{Name: &tlName, Kind: shared.TaskListKindNormal.Ptr()},
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

func TestActivityLocalDispatchInfoConversion(t *testing.T) {
	testCases := []*types.ActivityLocalDispatchInfo{
		nil,
		{},
		&testdata.ActivityLocalDispatchInfo,
	}

	for _, original := range testCases {
		thriftObj := FromActivityLocalDispatchInfo(original)
		roundTripObj := ToActivityLocalDispatchInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskCancelRequestedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskCancelRequestedEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskCancelRequestedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskCancelRequestedEventAttributes(original)
		roundTripObj := ToActivityTaskCancelRequestedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskCanceledEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskCanceledEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskCanceledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskCanceledEventAttributes(original)
		roundTripObj := ToActivityTaskCanceledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestAccessDeniedErrorConversion(t *testing.T) {
	testCases := []*types.AccessDeniedError{
		nil,
		{},
		&testdata.AccessDeniedError,
	}

	for _, original := range testCases {
		thriftObj := FromAccessDeniedError(original)
		roundTripObj := ToAccessDeniedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskCompletedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskCompletedEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskCompletedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskCompletedEventAttributes(original)
		roundTripObj := ToActivityTaskCompletedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskFailedEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskFailedEventAttributes(original)
		roundTripObj := ToActivityTaskFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskScheduledEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskScheduledEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskScheduledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskScheduledEventAttributes(original)
		roundTripObj := ToActivityTaskScheduledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskStartedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskStartedEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskStartedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskStartedEventAttributes(original)
		roundTripObj := ToActivityTaskStartedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityTaskTimedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ActivityTaskTimedOutEventAttributes{
		nil,
		{},
		&testdata.ActivityTaskTimedOutEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromActivityTaskTimedOutEventAttributes(original)
		roundTripObj := ToActivityTaskTimedOutEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestArchivalStatusConversion(t *testing.T) {
	enabledStatus := types.ArchivalStatus(1)
	disabledStatus := types.ArchivalStatus(0)
	testCases := []*types.ArchivalStatus{
		nil,
		&enabledStatus,
		&disabledStatus,
	}

	for _, original := range testCases {
		thriftObj := FromArchivalStatus(original)
		roundTripObj := ToArchivalStatus(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestBadBinariesConversion(t *testing.T) {
	testCases := []*types.BadBinaries{
		nil,
		{},
		&testdata.BadBinaries,
	}

	for _, original := range testCases {
		thriftObj := FromBadBinaries(original)
		roundTripObj := ToBadBinaries(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestBadBinaryInfoConversion(t *testing.T) {
	testCases := []*types.BadBinaryInfo{
		nil,
		{},
		&testdata.BadBinaryInfo,
	}

	for _, original := range testCases {
		thriftObj := FromBadBinaryInfo(original)
		roundTripObj := ToBadBinaryInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestBadRequestErrorConversion(t *testing.T) {
	testCases := []*types.BadRequestError{
		nil,
		{},
		{Message: "Error message for bad request"},
	}

	for _, original := range testCases {
		thriftObj := FromBadRequestError(original)
		roundTripObj := ToBadRequestError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCancelExternalWorkflowExecutionFailedCauseConversion(t *testing.T) {
	testCases := []*types.CancelExternalWorkflowExecutionFailedCause{
		nil,
		types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromCancelExternalWorkflowExecutionFailedCause(original)
		roundTripObj := ToCancelExternalWorkflowExecutionFailedCause(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCancelTimerDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.CancelTimerDecisionAttributes{
		nil,
		{},
		&testdata.CancelTimerDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromCancelTimerDecisionAttributes(original)
		roundTripObj := ToCancelTimerDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCancelTimerFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.CancelTimerFailedEventAttributes{
		nil,
		{},
		&testdata.CancelTimerFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromCancelTimerFailedEventAttributes(original)
		roundTripObj := ToCancelTimerFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCancelWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.CancelWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.CancelWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromCancelWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToCancelWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCancellationAlreadyRequestedErrorConversion(t *testing.T) {
	testCases := []*types.CancellationAlreadyRequestedError{
		nil,
		{},
		&testdata.CancellationAlreadyRequestedError,
	}

	for _, original := range testCases {
		thriftObj := FromCancellationAlreadyRequestedError(original)
		roundTripObj := ToCancellationAlreadyRequestedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionCanceledEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionCanceledEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionCanceledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionCanceledEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionCanceledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionCompletedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionCompletedEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionCompletedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionCompletedEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionCompletedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionFailedEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionFailedEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionStartedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionStartedEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionStartedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionStartedEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionStartedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionTerminatedEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionTerminatedEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionTerminatedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionTerminatedEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionTerminatedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestChildWorkflowExecutionTimedOutEventAttributesConversion(t *testing.T) {
	testCases := []*types.ChildWorkflowExecutionTimedOutEventAttributes{
		nil,
		{},
		&testdata.ChildWorkflowExecutionTimedOutEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromChildWorkflowExecutionTimedOutEventAttributes(original)
		roundTripObj := ToChildWorkflowExecutionTimedOutEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestClientVersionNotSupportedErrorConversion(t *testing.T) {
	testCases := []*types.ClientVersionNotSupportedError{
		nil,
		{},
		&testdata.ClientVersionNotSupportedError,
	}

	for _, original := range testCases {
		thriftObj := FromClientVersionNotSupportedError(original)
		roundTripObj := ToClientVersionNotSupportedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestFeatureNotEnabledErrorConversion(t *testing.T) {
	testCases := []*types.FeatureNotEnabledError{
		nil,
		{},
		{FeatureFlag: "test-feature-flag"},
	}

	for _, original := range testCases {
		thriftObj := FromFeatureNotEnabledError(original)
		roundTripObj := ToFeatureNotEnabledError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCloseShardRequestConversion(t *testing.T) {
	testCases := []*types.CloseShardRequest{
		nil,
		{},
		{ShardID: 5},
	}

	for _, original := range testCases {
		thriftObj := FromAdminCloseShardRequest(original)
		roundTripObj := ToAdminCloseShardRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestClusterInfoConversion(t *testing.T) {
	testCases := []*types.ClusterInfo{
		nil,
		{},
		&testdata.ClusterInfo,
	}

	for _, original := range testCases {
		thriftObj := FromGetClusterInfoResponse(original)
		roundTripObj := ToGetClusterInfoResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestClusterReplicationConfigurationConversion(t *testing.T) {
	testCases := []*types.ClusterReplicationConfiguration{
		nil,
		{},
		&testdata.ClusterReplicationConfiguration,
	}

	for _, original := range testCases {
		thriftObj := FromClusterReplicationConfiguration(original)
		roundTripObj := ToClusterReplicationConfiguration(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCompleteWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.CompleteWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.CompleteWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromCompleteWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToCompleteWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestContinueAsNewInitiatorConversion(t *testing.T) {
	testCases := []*types.ContinueAsNewInitiator{
		nil,
		types.ContinueAsNewInitiatorDecider.Ptr(),
		types.ContinueAsNewInitiatorRetryPolicy.Ptr(),
		types.ContinueAsNewInitiatorCronSchedule.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromContinueAsNewInitiator(original)
		roundTripObj := ToContinueAsNewInitiator(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestContinueAsNewWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.ContinueAsNewWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromContinueAsNewWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToContinueAsNewWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCountWorkflowExecutionsRequestConversion(t *testing.T) {
	testCases := []*types.CountWorkflowExecutionsRequest{
		nil,
		{},
		&testdata.CountWorkflowExecutionsRequest,
	}

	for _, original := range testCases {
		thriftObj := FromCountWorkflowExecutionsRequest(original)
		roundTripObj := ToCountWorkflowExecutionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCountWorkflowExecutionsResponseConversion(t *testing.T) {
	testCases := []*types.CountWorkflowExecutionsResponse{
		nil,
		{},
		&testdata.CountWorkflowExecutionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromCountWorkflowExecutionsResponse(original)
		roundTripObj := ToCountWorkflowExecutionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestCurrentBranchChangedErrorConversion(t *testing.T) {
	testCases := []*types.CurrentBranchChangedError{
		nil,
		{},
		&testdata.CurrentBranchChangedError,
	}

	for _, original := range testCases {
		thriftObj := FromCurrentBranchChangedError(original)
		roundTripObj := ToCurrentBranchChangedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDataBlobConversion(t *testing.T) {
	testCases := []*types.DataBlob{
		nil,
		{},
		&testdata.DataBlob,
	}

	for _, original := range testCases {
		thriftObj := FromDataBlob(original)
		roundTripObj := ToDataBlob(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionConverson(t *testing.T) {
	testCases := []*types.Decision{
		nil,
		{},
		{
			DecisionType:                           types.DecisionTypeScheduleActivityTask.Ptr(),
			ScheduleActivityTaskDecisionAttributes: &testdata.ScheduleActivityTaskDecisionAttributes,
		},
	}

	for _, original := range testCases {
		thriftObj := FromDecision(original)
		roundTripObj := ToDecision(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskCompletedEventAttributesConversion(t *testing.T) {
	testCases := []*types.DecisionTaskCompletedEventAttributes{
		nil,
		{},
		&testdata.DecisionTaskCompletedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskCompletedEventAttributes(original)
		roundTripObj := ToDecisionTaskCompletedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskFailedCauseConversion(t *testing.T) {
	testCases := []*types.DecisionTaskFailedCause{
		nil,
		types.DecisionTaskFailedCauseUnhandledDecision.Ptr(),
		types.DecisionTaskFailedCauseBadScheduleActivityAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadStartTimerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCancelTimerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRecordMarkerAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadContinueAsNewAttributes.Ptr(),
		types.DecisionTaskFailedCauseStartTimerDuplicateID.Ptr(),
		types.DecisionTaskFailedCauseResetStickyTasklist.Ptr(),
		types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
		types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseBadStartChildExecutionAttributes.Ptr(),
		types.DecisionTaskFailedCauseForceCloseDecision.Ptr(),
		types.DecisionTaskFailedCauseFailoverCloseDecision.Ptr(),
		types.DecisionTaskFailedCauseBadSignalInputSize.Ptr(),
		types.DecisionTaskFailedCauseResetWorkflow.Ptr(),
		types.DecisionTaskFailedCauseBadBinary.Ptr(),
		types.DecisionTaskFailedCauseScheduleActivityDuplicateID.Ptr(),
		types.DecisionTaskFailedCauseBadSearchAttributes.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskFailedCause(original)
		roundTripObj := ToDecisionTaskFailedCause(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.DecisionTaskFailedEventAttributes{
		nil,
		{},
		&testdata.DecisionTaskFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskFailedEventAttributes(original)
		roundTripObj := ToDecisionTaskFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskScheduledEventAttributesConversion(t *testing.T) {
	testCases := []*types.DecisionTaskScheduledEventAttributes{
		nil,
		{},
		&testdata.DecisionTaskScheduledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskScheduledEventAttributes(original)
		roundTripObj := ToDecisionTaskScheduledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskStartedEventAttributesConversion(t *testing.T) {
	testCases := []*types.DecisionTaskStartedEventAttributes{
		nil,
		{},
		&testdata.DecisionTaskStartedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskStartedEventAttributes(original)
		roundTripObj := ToDecisionTaskStartedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskTimedOutCauseConversion(t *testing.T) {
	testCases := []*types.DecisionTaskTimedOutCause{
		nil,
		types.DecisionTaskTimedOutCauseTimeout.Ptr(),
		types.DecisionTaskTimedOutCauseReset.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskTimedOutCause(original)
		roundTripObj := ToDecisionTaskTimedOutCause(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTaskTimedOutEventAttributesConversion(t *testing.T) {
	testCases := []*types.DecisionTaskTimedOutEventAttributes{
		nil,
		{},
		&testdata.DecisionTaskTimedOutEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromDecisionTaskTimedOutEventAttributes(original)
		roundTripObj := ToDecisionTaskTimedOutEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecisionTypeConversion(t *testing.T) {
	testCases := []*types.DecisionType{
		nil,
		types.DecisionTypeScheduleActivityTask.Ptr(),
		types.DecisionTypeRequestCancelActivityTask.Ptr(),
		types.DecisionTypeStartTimer.Ptr(),
		types.DecisionTypeCompleteWorkflowExecution.Ptr(),
		types.DecisionTypeFailWorkflowExecution.Ptr(),
		types.DecisionTypeCancelTimer.Ptr(),
		types.DecisionTypeCancelWorkflowExecution.Ptr(),
		types.DecisionTypeRequestCancelExternalWorkflowExecution.Ptr(),
		types.DecisionTypeRecordMarker.Ptr(),
		types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
		types.DecisionTypeStartChildWorkflowExecution.Ptr(),
		types.DecisionTypeSignalExternalWorkflowExecution.Ptr(),
		types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromDecisionType(original)
		roundTripObj := ToDecisionType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDeleteDomainRequestConversion(t *testing.T) {
	testCases := []*types.DeleteDomainRequest{
		nil,
		{},
		&testdata.DeleteDomainRequest,
	}

	for _, original := range testCases {
		thriftObj := FromDeleteDomainRequest(original)
		roundTripObj := ToDeleteDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDeprecateDomainRequestConversion(t *testing.T) {
	testCases := []*types.DeprecateDomainRequest{
		nil,
		{},
		&testdata.DeprecateDomainRequest,
	}

	for _, original := range testCases {
		thriftObj := FromDeprecateDomainRequest(original)
		roundTripObj := ToDeprecateDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeDomainRequestConversion(t *testing.T) {
	testCases := []*types.DescribeDomainRequest{
		nil,
		{},
		{Name: common.StringPtr("test-name"), UUID: common.StringPtr("test-uuid")},
	}

	for _, original := range testCases {
		thriftObj := FromDescribeDomainRequest(original)
		roundTripObj := ToDescribeDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeDomainResponseConversion(t *testing.T) {
	testCases := []*types.DescribeDomainResponse{
		nil,
		{},
		&testdata.DescribeDomainResponse,
	}

	for _, original := range testCases {
		thriftObj := FromDescribeDomainResponse(original)
		roundTripObj := ToDescribeDomainResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDecribeHistoryHostRequstConversion(t *testing.T) {
	testCases := []*types.DescribeHistoryHostRequest{
		nil,
		{},
		{HostAddress: common.StringPtr("test-host-address")},
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeHistoryHostRequest(original)
		roundTripObj := ToAdminDescribeHistoryHostRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeShardDistributionRequestConversion(t *testing.T) {
	testCases := []*types.DescribeShardDistributionRequest{
		nil,
		{},
		{PageSize: 100},
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeShardDistributionRequest(original)
		roundTripObj := ToAdminDescribeShardDistributionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeHistoryHostResponseConversion(t *testing.T) {
	testCases := []*types.DescribeHistoryHostResponse{
		nil,
		{},
		{NumberOfShards: 10, ShardIDs: []int32{1, 2, 3}, Address: "test-address"},
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeHistoryHostResponse(original)
		roundTripObj := ToAdminDescribeHistoryHostResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeShardDistributionResponseConversion(t *testing.T) {
	testCases := []*types.DescribeShardDistributionResponse{
		nil,
		{},
		{NumberOfShards: 10, Shards: map[int32]string{1: "test-host-1", 2: "test-host-2"}},
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeShardDistributionResponse(original)
		roundTripObj := ToAdminDescribeShardDistributionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeQueueRequestConversion(t *testing.T) {
	testCases := []*types.DescribeQueueRequest{
		nil,
		{},
		{ShardID: 10},
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeQueueRequest(original)
		roundTripObj := ToAdminDescribeQueueRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeQueueResponseConversion(t *testing.T) {
	testCases := []*types.DescribeQueueResponse{
		nil,
		{},
		&testdata.HistoryDescribeQueueResponse,
	}

	for _, original := range testCases {
		thriftObj := FromAdminDescribeQueueResponse(original)
		roundTripObj := ToAdminDescribeQueueResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeTaskListRequestConversion(t *testing.T) {
	testCases := []*types.DescribeTaskListRequest{
		nil,
		{},
		&testdata.DescribeTaskListRequest,
	}

	for _, original := range testCases {
		thriftObj := FromDescribeTaskListRequest(original)
		roundTripObj := ToDescribeTaskListRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeTaskListResponseConversion(t *testing.T) {
	testCases := []*types.DescribeTaskListResponse{
		nil,
		{},
		&testdata.DescribeTaskListResponse,
	}

	for _, original := range testCases {
		thriftObj := FromDescribeTaskListResponse(original)
		roundTripObj := ToDescribeTaskListResponse(thriftObj)
		opt := cmpopts.IgnoreFields(types.DescribeTaskListResponse{}, "PartitionConfig")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestDescribeWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.DescribeWorkflowExecutionRequest{
		nil,
		{},
		&testdata.DescribeWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromDescribeWorkflowExecutionRequest(original)
		roundTripObj := ToDescribeWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeWorkflowExecutionResponseConversion(t *testing.T) {
	testCases := []*types.DescribeWorkflowExecutionResponse{
		nil,
		{},
		&testdata.DescribeWorkflowExecutionResponse,
	}

	for _, original := range testCases {
		thriftObj := FromDescribeWorkflowExecutionResponse(original)
		roundTripObj := ToDescribeWorkflowExecutionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDiagnoseWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.DiagnoseWorkflowExecutionRequest{
		nil,
		{},
		&testdata.DiagnoseWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromDiagnoseWorkflowExecutionRequest(original)
		roundTripObj := ToDiagnoseWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDiagnoseWorkflowExecutionResponseConversion(t *testing.T) {
	testCases := []*types.DiagnoseWorkflowExecutionResponse{
		nil,
		{},
		&testdata.DiagnoseWorkflowExecutionResponse,
	}

	for _, original := range testCases {
		thriftObj := FromDiagnoseWorkflowExecutionResponse(original)
		roundTripObj := ToDiagnoseWorkflowExecutionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainAlreadyExistsErrorConversion(t *testing.T) {
	testCases := []*types.DomainAlreadyExistsError{
		nil,
		{},
		{Message: "test-message"},
	}

	for _, original := range testCases {
		thriftObj := FromDomainAlreadyExistsError(original)
		roundTripObj := ToDomainAlreadyExistsError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainCacheInfoConversion(t *testing.T) {
	testCases := []*types.DomainCacheInfo{
		nil,
		{},
		&testdata.DomainCacheInfo,
	}

	for _, original := range testCases {
		thriftObj := FromDomainCacheInfo(original)
		roundTripObj := ToDomainCacheInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainConfigurationConversion(t *testing.T) {
	testCases := []*types.DomainConfiguration{
		nil,
		{},
		&testdata.DomainConfiguration,
	}

	for _, original := range testCases {
		thriftObj := FromDomainConfiguration(original)
		roundTripObj := ToDomainConfiguration(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainInfoConversion(t *testing.T) {
	testCases := []*types.DomainInfo{
		nil,
		{},
		&testdata.DomainInfo,
	}

	for _, original := range testCases {
		thriftObj := FromDomainInfo(original)
		roundTripObj := ToDomainInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestFailoverInfoConversion(t *testing.T) {
	testCases := []*types.FailoverInfo{
		nil,
		{},
		&testdata.FailoverInfo,
	}

	for _, original := range testCases {
		thriftObj := FromFailoverInfo(original)
		roundTripObj := ToFailoverInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainNotActiveErrorConversion(t *testing.T) {
	testCases := []*types.DomainNotActiveError{
		nil,
		{},
		{Message: "test-message"},
		{Message: "test-message", DomainName: "test-domain", CurrentCluster: "test-current-cluster", ActiveCluster: "test-active-cluster"},
		{Message: "test-message", DomainName: "test-domain", CurrentCluster: "test-current-cluster", ActiveClusters: []string{"test-active-cluster-1", "test-active-cluster-2"}},
	}

	for _, original := range testCases {
		thriftObj := FromDomainNotActiveError(original)
		roundTripObj := ToDomainNotActiveError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainReplicationConfigurationConversion(t *testing.T) {
	testCases := []*types.DomainReplicationConfiguration{
		nil,
		{},
		&testdata.DomainReplicationConfiguration,
	}

	for _, original := range testCases {
		thriftObj := FromDomainReplicationConfiguration(original)
		roundTripObj := ToDomainReplicationConfiguration(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDomainStatusConversion(t *testing.T) {
	testCases := []*types.DomainStatus{
		nil,
		types.DomainStatusRegistered.Ptr(),
		types.DomainStatusDeprecated.Ptr(),
		types.DomainStatusDeleted.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromDomainStatus(original)
		roundTripObj := ToDomainStatus(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestEncodingTypeConversion(t *testing.T) {
	testCases := []*types.EncodingType{
		nil,
		types.EncodingTypeThriftRW.Ptr(),
		types.EncodingTypeJSON.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromEncodingType(original)
		roundTripObj := ToEncodingType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestEntityNotExistsErrorConversion(t *testing.T) {
	testCases := []*types.EntityNotExistsError{
		nil,
		{},
		&testdata.EntityNotExistsError,
	}

	for _, original := range testCases {
		thriftObj := FromEntityNotExistsError(original)
		roundTripObj := ToEntityNotExistsError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionAlreadyCompletedErrorConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionAlreadyCompletedError{
		nil,
		{},
		&testdata.WorkflowExecutionAlreadyCompletedError,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionAlreadyCompletedError(original)
		roundTripObj := ToWorkflowExecutionAlreadyCompletedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestEventTypeConversion(t *testing.T) {
	testCases := []*types.EventType{
		nil,
		types.EventTypeWorkflowExecutionStarted.Ptr(),
		types.EventTypeWorkflowExecutionCompleted.Ptr(),
		types.EventTypeWorkflowExecutionFailed.Ptr(),
		types.EventTypeWorkflowExecutionTimedOut.Ptr(),
		types.EventTypeDecisionTaskScheduled.Ptr(),
		types.EventTypeDecisionTaskStarted.Ptr(),
		types.EventTypeDecisionTaskCompleted.Ptr(),
		types.EventTypeDecisionTaskTimedOut.Ptr(),
		types.EventTypeDecisionTaskFailed.Ptr(),
		types.EventTypeActivityTaskScheduled.Ptr(),
		types.EventTypeActivityTaskStarted.Ptr(),
		types.EventTypeActivityTaskCompleted.Ptr(),
		types.EventTypeActivityTaskFailed.Ptr(),
		types.EventTypeActivityTaskTimedOut.Ptr(),
		types.EventTypeActivityTaskCancelRequested.Ptr(),
		types.EventTypeActivityTaskCanceled.Ptr(),
		types.EventTypeTimerStarted.Ptr(),
		types.EventTypeTimerFired.Ptr(),
		types.EventTypeCancelTimerFailed.Ptr(),
		types.EventTypeTimerCanceled.Ptr(),
		types.EventTypeWorkflowExecutionCancelRequested.Ptr(),
		types.EventTypeWorkflowExecutionCanceled.Ptr(),
		types.EventTypeRequestCancelExternalWorkflowExecutionInitiated.Ptr(),
		types.EventTypeRequestCancelExternalWorkflowExecutionFailed.Ptr(),
		types.EventTypeExternalWorkflowExecutionCancelRequested.Ptr(),
		types.EventTypeMarkerRecorded.Ptr(),
		types.EventTypeWorkflowExecutionSignaled.Ptr(),
		types.EventTypeWorkflowExecutionTerminated.Ptr(),
		types.EventTypeWorkflowExecutionContinuedAsNew.Ptr(),
		types.EventTypeStartChildWorkflowExecutionInitiated.Ptr(),
		types.EventTypeStartChildWorkflowExecutionFailed.Ptr(),
		types.EventTypeChildWorkflowExecutionStarted.Ptr(),
		types.EventTypeChildWorkflowExecutionCompleted.Ptr(),
		types.EventTypeChildWorkflowExecutionFailed.Ptr(),
		types.EventTypeChildWorkflowExecutionCanceled.Ptr(),
		types.EventTypeChildWorkflowExecutionTimedOut.Ptr(),
		types.EventTypeChildWorkflowExecutionTerminated.Ptr(),
		types.EventTypeSignalExternalWorkflowExecutionInitiated.Ptr(),
		types.EventTypeSignalExternalWorkflowExecutionFailed.Ptr(),
		types.EventTypeExternalWorkflowExecutionSignaled.Ptr(),
		types.EventTypeUpsertWorkflowSearchAttributes.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromEventType(original)
		roundTripObj := ToEventType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestExternalWorkflowExecutionSignaledEventAttributesConversion(t *testing.T) {
	testCases := []*types.ExternalWorkflowExecutionSignaledEventAttributes{
		nil,
		{},
		&testdata.ExternalWorkflowExecutionSignaledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromExternalWorkflowExecutionSignaledEventAttributes(original)
		roundTripObj := ToExternalWorkflowExecutionSignaledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestFailWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.FailWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.FailWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromFailWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToFailWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestGetSearchAttributesResponseConversion(t *testing.T) {
	testCases := []*types.GetSearchAttributesResponse{
		nil,
		{},
		&testdata.GetSearchAttributesResponse,
	}

	for _, original := range testCases {
		thriftObj := FromGetSearchAttributesResponse(original)
		roundTripObj := ToGetSearchAttributesResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestGetWorkflowExecutionHistoryRequestConversion(t *testing.T) {
	testCases := []*types.GetWorkflowExecutionHistoryRequest{
		nil,
		{},
		&testdata.GetWorkflowExecutionHistoryRequest,
	}

	for _, original := range testCases {
		thriftObj := FromGetWorkflowExecutionHistoryRequest(original)
		roundTripObj := ToGetWorkflowExecutionHistoryRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestGetWorkflowExecutionHistoryResponseConvesion(t *testing.T) {
	testCases := []*types.GetWorkflowExecutionHistoryResponse{
		nil,
		{},
		&testdata.GetWorkflowExecutionHistoryResponse,
	}

	for _, original := range testCases {
		thriftObj := FromGetWorkflowExecutionHistoryResponse(original)
		roundTripObj := ToGetWorkflowExecutionHistoryResponse(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestHeaderConversion(t *testing.T) {
	testCases := []*types.Header{
		nil,
		{},
		&testdata.Header,
	}

	for _, original := range testCases {
		thriftObj := FromHeader(original)
		roundTripObj := ToHeader(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestHistoryConversion(t *testing.T) {
	testCases := []*types.History{
		nil,
		{},
		&testdata.History,
	}

	for _, original := range testCases {
		thriftObj := FromHistory(original)
		roundTripObj := ToHistory(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestHistoryBranchConversion(t *testing.T) {
	testCases := []*types.HistoryBranch{
		nil,
		{},
		{TreeID: "test-tree-id", BranchID: "test-branch-id"},
	}

	for _, original := range testCases {
		thriftObj := FromHistoryBranch(original)
		roundTripObj := ToHistoryBranch(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestHistoryBranchRangeConversion(t *testing.T) {
	testCases := []*types.HistoryBranchRange{
		nil,
		{},
		{BranchID: "test-branch-id", BeginNodeID: 5, EndNodeID: 10},
	}

	for _, original := range testCases {
		thriftObj := FromHistoryBranchRange(original)
		roundTripObj := ToHistoryBranchRange(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestHistoryEventConversion(t *testing.T) {
	testCases := []*types.HistoryEvent{
		nil,
		{},
		{
			ID:                                      5,
			Timestamp:                               common.Int64Ptr(10),
			EventType:                               types.EventTypeWorkflowExecutionStarted.Ptr(),
			WorkflowExecutionStartedEventAttributes: &testdata.WorkflowExecutionStartedEventAttributes,
		},
	}

	for _, original := range testCases {
		thriftObj := FromHistoryEvent(original)
		roundTripObj := ToHistoryEvent(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestHistoryEventFilterTypeConversion(t *testing.T) {
	testCases := []*types.HistoryEventFilterType{
		nil,
		types.HistoryEventFilterTypeAllEvent.Ptr(),
		types.HistoryEventFilterTypeCloseEvent.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromHistoryEventFilterType(original)
		roundTripObj := ToHistoryEventFilterType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestIndexedValueTypeConversion(t *testing.T) {
	testCases := []types.IndexedValueType{
		types.IndexedValueTypeString,
		types.IndexedValueTypeKeyword,
		types.IndexedValueTypeInt,
		types.IndexedValueTypeDouble,
		types.IndexedValueTypeBool,
		types.IndexedValueTypeDatetime,
	}

	for _, original := range testCases {
		thriftObj := FromIndexedValueType(original)
		roundTripObj := ToIndexedValueType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestInternalDataInconsistencyErrorConversion(t *testing.T) {
	testCases := []*types.InternalDataInconsistencyError{
		nil,
		{},
		&testdata.InternalDataInconsistencyError,
	}

	for _, original := range testCases {
		thriftObj := FromInternalDataInconsistencyError(original)
		roundTripObj := ToInternalDataInconsistencyError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestInternalServiceErrorConversion(t *testing.T) {
	testCases := []*types.InternalServiceError{
		nil,
		{},
		&testdata.InternalServiceError,
	}

	for _, original := range testCases {
		thriftObj := FromInternalServiceError(original)
		roundTripObj := ToInternalServiceError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestLimitExceededErrorConversion(t *testing.T) {
	testCases := []*types.LimitExceededError{
		nil,
		{},
		&testdata.LimitExceededError,
	}

	for _, original := range testCases {
		thriftObj := FromLimitExceededError(original)
		roundTripObj := ToLimitExceededError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListArchivedWorkflowExecutionsRequestConversion(t *testing.T) {
	testCases := []*types.ListArchivedWorkflowExecutionsRequest{
		nil,
		{},
		&testdata.ListArchivedWorkflowExecutionsRequest,
	}

	for _, original := range testCases {
		thriftObj := FromListArchivedWorkflowExecutionsRequest(original)
		roundTripObj := ToListArchivedWorkflowExecutionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListArchivedWorkflowExecutionsResponseConversion(t *testing.T) {
	testCases := []*types.ListArchivedWorkflowExecutionsResponse{
		nil,
		{},
		&testdata.ListArchivedWorkflowExecutionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListArchivedWorkflowExecutionsResponse(original)
		roundTripObj := ToListArchivedWorkflowExecutionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListClosedWorkflowExecutionsRequestConversion(t *testing.T) {
	testCases := []*types.ListClosedWorkflowExecutionsRequest{
		nil,
		{},
		{Domain: "test-domain", MaximumPageSize: 100, NextPageToken: []byte("test-next-page-token")},
	}

	for _, original := range testCases {
		thriftObj := FromListClosedWorkflowExecutionsRequest(original)
		roundTripObj := ToListClosedWorkflowExecutionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListClosedWorkflowExecutionsResponseConversion(t *testing.T) {
	testCases := []*types.ListClosedWorkflowExecutionsResponse{
		nil,
		{},
		&testdata.ListClosedWorkflowExecutionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListClosedWorkflowExecutionsResponse(original)
		roundTripObj := ToListClosedWorkflowExecutionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListDomainsRequestConversion(t *testing.T) {
	testCases := []*types.ListDomainsRequest{
		nil,
		{},
		&testdata.ListDomainsRequest,
	}

	for _, original := range testCases {
		thriftObj := FromListDomainsRequest(original)
		roundTripObj := ToListDomainsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListDomainsResponseConversion(t *testing.T) {
	testCases := []*types.ListDomainsResponse{
		nil,
		{},
		&testdata.ListDomainsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListDomainsResponse(original)
		roundTripObj := ToListDomainsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListOpenWorkflowExecutionsRequestConversion(t *testing.T) {
	testCases := []*types.ListOpenWorkflowExecutionsRequest{
		nil,
		{},
		{Domain: "test-domain", MaximumPageSize: 100, NextPageToken: []byte("test-next-page-token")},
	}

	for _, original := range testCases {
		thriftObj := FromListOpenWorkflowExecutionsRequest(original)
		roundTripObj := ToListOpenWorkflowExecutionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListOpenWorkflowExecutionsResponseConversion(t *testing.T) {
	testCases := []*types.ListOpenWorkflowExecutionsResponse{
		nil,
		{},
		&testdata.ListOpenWorkflowExecutionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListOpenWorkflowExecutionsResponse(original)
		roundTripObj := ToListOpenWorkflowExecutionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListTaskListPartitionsRequestConversion(t *testing.T) {
	testCases := []*types.ListTaskListPartitionsRequest{
		nil,
		{},
		&testdata.ListTaskListPartitionsRequest,
	}

	for _, original := range testCases {
		thriftObj := FromListTaskListPartitionsRequest(original)
		roundTripObj := ToListTaskListPartitionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListTaskListPartitionsResponseConversion(t *testing.T) {
	testCases := []*types.ListTaskListPartitionsResponse{
		nil,
		{},
		&testdata.ListTaskListPartitionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListTaskListPartitionsResponse(original)
		roundTripObj := ToListTaskListPartitionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestGetTaskListsByDomainRequestConversion(t *testing.T) {
	testCases := []*types.GetTaskListsByDomainRequest{
		nil,
		{},
		&testdata.MatchingGetTaskListsByDomainRequest,
	}

	for _, original := range testCases {
		thriftObj := FromGetTaskListsByDomainRequest(original)
		roundTripObj := ToGetTaskListsByDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestGetTaskListsByDomainResponseConversion(t *testing.T) {
	testCases := []*types.GetTaskListsByDomainResponse{
		nil,
		{},
		&testdata.GetTaskListsByDomainResponse,
	}

	for _, original := range testCases {
		thriftObj := FromGetTaskListsByDomainResponse(original)
		roundTripObj := ToGetTaskListsByDomainResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestDescribeTaskListResponseMapConversion(t *testing.T) {
	testCases := []map[string]*types.DescribeTaskListResponse{
		nil,
		{},
		{"test-key": &testdata.DescribeTaskListResponse},
	}

	for _, original := range testCases {
		thriftObj := FromDescribeTaskListResponseMap(original)
		roundTripObj := ToDescribeTaskListResponseMap(thriftObj)
		opt := cmpopts.IgnoreFields(types.DescribeTaskListResponse{}, "PartitionConfig")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestListWorkflowExecutionsRequestConversion(t *testing.T) {
	testCases := []*types.ListWorkflowExecutionsRequest{
		nil,
		{},
		&testdata.ListWorkflowExecutionsRequest,
	}

	for _, original := range testCases {
		thriftObj := FromListWorkflowExecutionsRequest(original)
		roundTripObj := ToListWorkflowExecutionsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListWorkflowExecutionsResponseConversion(t *testing.T) {
	testCases := []*types.ListWorkflowExecutionsResponse{
		nil,
		{},
		&testdata.ListWorkflowExecutionsResponse,
	}

	for _, original := range testCases {
		thriftObj := FromListWorkflowExecutionsResponse(original)
		roundTripObj := ToListWorkflowExecutionsResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestMarkerRecordedEventAttributesConversion(t *testing.T) {
	testCases := []*types.MarkerRecordedEventAttributes{
		nil,
		{},
		&testdata.MarkerRecordedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromMarkerRecordedEventAttributes(original)
		roundTripObj := ToMarkerRecordedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestMemoConversion(t *testing.T) {
	testCases := []*types.Memo{
		nil,
		{},
		&testdata.Memo,
	}

	for _, original := range testCases {
		thriftObj := FromMemo(original)
		roundTripObj := ToMemo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestParentClosePolicyConversion(t *testing.T) {
	testCases := []*types.ParentClosePolicy{
		nil,
		types.ParentClosePolicyAbandon.Ptr(),
		types.ParentClosePolicyRequestCancel.Ptr(),
		types.ParentClosePolicyTerminate.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromParentClosePolicy(original)
		roundTripObj := ToParentClosePolicy(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPendingActivityInfoConversion(t *testing.T) {
	testCases := []*types.PendingActivityInfo{
		nil,
		{},
		&testdata.PendingActivityInfo,
	}

	for _, original := range testCases {
		thriftObj := FromPendingActivityInfo(original)
		roundTripObj := ToPendingActivityInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPendingActivityStateConversion(t *testing.T) {
	testCases := []*types.PendingActivityState{
		nil,
		types.PendingActivityStateScheduled.Ptr(),
		types.PendingActivityStateStarted.Ptr(),
		types.PendingActivityStateCancelRequested.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromPendingActivityState(original)
		roundTripObj := ToPendingActivityState(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPendingChildExecutionInfoConversion(t *testing.T) {
	testCases := []*types.PendingChildExecutionInfo{
		nil,
		{},
		&testdata.PendingChildExecutionInfo,
	}

	for _, original := range testCases {
		thriftObj := FromPendingChildExecutionInfo(original)
		roundTripObj := ToPendingChildExecutionInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPendingDecisionInfoConversion(t *testing.T) {
	testCases := []*types.PendingDecisionInfo{
		nil,
		{},
		&testdata.PendingDecisionInfo,
	}

	for _, original := range testCases {
		thriftObj := FromPendingDecisionInfo(original)
		roundTripObj := ToPendingDecisionInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPendingDecisionStateConversion(t *testing.T) {
	testCases := []*types.PendingDecisionState{
		nil,
		types.PendingDecisionStateScheduled.Ptr(),
		types.PendingDecisionStateStarted.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromPendingDecisionState(original)
		roundTripObj := ToPendingDecisionState(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPollForActivityTaskRequestConversion(t *testing.T) {
	testCases := []*types.PollForActivityTaskRequest{
		nil,
		{},
		&testdata.PollForActivityTaskRequest,
	}

	for _, original := range testCases {
		thriftObj := FromPollForActivityTaskRequest(original)
		roundTripObj := ToPollForActivityTaskRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPollForActivityTaskResponseConversion(t *testing.T) {
	testCases := []*types.PollForActivityTaskResponse{
		nil,
		{},
		&testdata.PollForActivityTaskResponse,
	}

	for _, original := range testCases {
		thriftObj := FromPollForActivityTaskResponse(original)
		roundTripObj := ToPollForActivityTaskResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPollForDecisionTaskRequestConversion(t *testing.T) {
	testCases := []*types.PollForDecisionTaskRequest{
		nil,
		{},
		&testdata.PollForDecisionTaskRequest,
	}

	for _, original := range testCases {
		thriftObj := FromPollForDecisionTaskRequest(original)
		roundTripObj := ToPollForDecisionTaskRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestPollForDecisionTaskResponseConversion(t *testing.T) {
	testCases := []*types.PollForDecisionTaskResponse{
		nil,
		{},
		&testdata.PollForDecisionTaskResponse,
	}

	for _, original := range testCases {
		thriftObj := FromPollForDecisionTaskResponse(original)
		roundTripObj := ToPollForDecisionTaskResponse(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestPollerInfoConversion(t *testing.T) {
	testCases := []*types.PollerInfo{
		nil,
		{},
		&testdata.PollerInfo,
	}

	for _, original := range testCases {
		thriftObj := FromPollerInfo(original)
		roundTripObj := ToPollerInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryConsistencyLevelConversion(t *testing.T) {
	testCases := []*types.QueryConsistencyLevel{
		nil,
		types.QueryConsistencyLevelEventual.Ptr(),
		types.QueryConsistencyLevelStrong.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromQueryConsistencyLevel(original)
		roundTripObj := ToQueryConsistencyLevel(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryFailedErrorConversion(t *testing.T) {
	testCases := []*types.QueryFailedError{
		nil,
		{},
		&testdata.QueryFailedError,
	}

	for _, original := range testCases {
		thriftObj := FromQueryFailedError(original)
		roundTripObj := ToQueryFailedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryRejectConditionConversion(t *testing.T) {
	testCases := []*types.QueryRejectCondition{
		nil,
		types.QueryRejectConditionNotOpen.Ptr(),
		types.QueryRejectConditionNotCompletedCleanly.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromQueryRejectCondition(original)
		roundTripObj := ToQueryRejectCondition(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryRejectedConversion(t *testing.T) {
	testCases := []*types.QueryRejected{
		nil,
		{},
		&testdata.QueryRejected,
	}

	for _, original := range testCases {
		thriftObj := FromQueryRejected(original)
		roundTripObj := ToQueryRejected(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryResultTypeConversion(t *testing.T) {
	testCases := []*types.QueryResultType{
		nil,
		types.QueryResultTypeAnswered.Ptr(),
		types.QueryResultTypeFailed.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromQueryResultType(original)
		roundTripObj := ToQueryResultType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryTaskCompletedTypeConversion(t *testing.T) {
	testCases := []*types.QueryTaskCompletedType{
		nil,
		types.QueryTaskCompletedTypeCompleted.Ptr(),
		types.QueryTaskCompletedTypeFailed.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromQueryTaskCompletedType(original)
		roundTripObj := ToQueryTaskCompletedType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryWorkflowRequestConversion(t *testing.T) {
	testCases := []*types.QueryWorkflowRequest{
		nil,
		{},
		&testdata.QueryWorkflowRequest,
	}

	for _, original := range testCases {
		thriftObj := FromQueryWorkflowRequest(original)
		roundTripObj := ToQueryWorkflowRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueryWorkflowResponseConversion(t *testing.T) {
	testCases := []*types.QueryWorkflowResponse{
		nil,
		{},
		&testdata.QueryWorkflowResponse,
	}

	for _, original := range testCases {
		thriftObj := FromQueryWorkflowResponse(original)
		roundTripObj := ToQueryWorkflowResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestReapplyEventsRequestConversion(t *testing.T) {
	testCases := []*types.ReapplyEventsRequest{
		nil,
		{},
		{DomainName: "test-domain-name", WorkflowExecution: &testdata.WorkflowExecution},
	}

	for _, original := range testCases {
		thriftObj := FromAdminReapplyEventsRequest(original)
		roundTripObj := ToAdminReapplyEventsRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRecordActivityTaskHeartbeatByIDRequestConversion(t *testing.T) {
	testCases := []*types.RecordActivityTaskHeartbeatByIDRequest{
		nil,
		{},
		&testdata.RecordActivityTaskHeartbeatByIDRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRecordActivityTaskHeartbeatByIDRequest(original)
		roundTripObj := ToRecordActivityTaskHeartbeatByIDRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRecordActivityTaskHeartbeatRequestConversion(t *testing.T) {
	testCases := []*types.RecordActivityTaskHeartbeatRequest{
		nil,
		{},
		&testdata.RecordActivityTaskHeartbeatRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRecordActivityTaskHeartbeatRequest(original)
		roundTripObj := ToRecordActivityTaskHeartbeatRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRecordActivityTaskHeartbeatResponseConversion(t *testing.T) {
	testCases := []*types.RecordActivityTaskHeartbeatResponse{
		nil,
		{},
		&testdata.RecordActivityTaskHeartbeatResponse,
	}

	for _, original := range testCases {
		thriftObj := FromRecordActivityTaskHeartbeatResponse(original)
		roundTripObj := ToRecordActivityTaskHeartbeatResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRecordMarkerDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.RecordMarkerDecisionAttributes{
		nil,
		{},
		&testdata.RecordMarkerDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromRecordMarkerDecisionAttributes(original)
		roundTripObj := ToRecordMarkerDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRefreshWorkflowTasksRequestConversion(t *testing.T) {
	testCases := []*types.RefreshWorkflowTasksRequest{
		nil,
		{},
		{Domain: "test-domain", Execution: &testdata.WorkflowExecution},
	}

	for _, original := range testCases {
		thriftObj := FromRefreshWorkflowTasksRequest(original)
		roundTripObj := ToRefreshWorkflowTasksRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRegisterDomainRequestConversion(t *testing.T) {
	testCases := []*types.RegisterDomainRequest{
		nil,
		{},
		&testdata.RegisterDomainRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRegisterDomainRequest(original)
		roundTripObj := ToRegisterDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRegisterDomainRequestFuzz(t *testing.T) {
	t.Run("round trip from internal", func(t *testing.T) {
		testutils.EnsureFuzzCoverage(t, []string{
			"nil", "empty", "filled",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			// Configure fuzzer to generate valid enum values
			fuzzer := f.Funcs(
				func(e *types.ArchivalStatus, c fuzz.Continue) {
					*e = types.ArchivalStatus(c.Intn(2)) // 0-1 are valid values (Disabled=0, Enabled=1)
				},
			).NilChance(0.3)

			var orig *types.RegisterDomainRequest
			fuzzer.Fuzz(&orig)
			out := ToRegisterDomainRequest(FromRegisterDomainRequest(orig))
			assert.Equal(t, orig, out, "RegisterDomainRequest did not survive round-tripping")

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

func TestRemoteSyncMatchedErrorConversion(t *testing.T) {
	testCases := []*types.RemoteSyncMatchedError{
		nil,
		{},
		&testdata.RemoteSyncMatchedError,
	}

	for _, original := range testCases {
		thriftObj := FromRemoteSyncMatchedError(original)
		roundTripObj := ToRemoteSyncMatchedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRemoveTaskRequestConversion(t *testing.T) {
	testCases := []*types.RemoveTaskRequest{
		nil,
		{},
		{ShardID: 10, Type: common.Int32Ptr(1), TaskID: 5, ClusterName: "test-cluster-name"},
	}

	for _, original := range testCases {
		thriftObj := FromAdminRemoveTaskRequest(original)
		roundTripObj := ToAdminRemoveTaskRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRequestCancelActivityTaskDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.RequestCancelActivityTaskDecisionAttributes{
		nil,
		{},
		&testdata.RequestCancelActivityTaskDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromRequestCancelActivityTaskDecisionAttributes(original)
		roundTripObj := ToRequestCancelActivityTaskDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRequestCancelActivityTaskFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.RequestCancelActivityTaskFailedEventAttributes{
		nil,
		{},
		&testdata.RequestCancelActivityTaskFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromRequestCancelActivityTaskFailedEventAttributes(original)
		roundTripObj := ToRequestCancelActivityTaskFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRequestCancelExternalWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.RequestCancelExternalWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromRequestCancelExternalWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToRequestCancelExternalWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRequestCancelExternalWorkflowExecutionFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		nil,
		{},
		&testdata.RequestCancelExternalWorkflowExecutionFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(original)
		roundTripObj := ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRequestCancelWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.RequestCancelWorkflowExecutionRequest{
		nil,
		{},
		&testdata.RequestCancelWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRequestCancelWorkflowExecutionRequest(original)
		roundTripObj := ToRequestCancelWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetPointInfoConversion(t *testing.T) {
	testCases := []*types.ResetPointInfo{
		nil,
		{},
		&testdata.ResetPointInfo,
	}

	for _, original := range testCases {
		thriftObj := FromResetPointInfo(original)
		roundTripObj := ToResetPointInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetPointsConversion(t *testing.T) {
	testCases := []*types.ResetPoints{
		nil,
		{},
		&testdata.ResetPoints,
	}

	for _, original := range testCases {
		thriftObj := FromResetPoints(original)
		roundTripObj := ToResetPoints(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetQueueRequestConversion(t *testing.T) {
	testCases := []*types.ResetQueueRequest{
		nil,
		{},
		{ShardID: 10, ClusterName: "test-cluster-name"},
	}

	for _, original := range testCases {
		thriftObj := FromAdminResetQueueRequest(original)
		roundTripObj := ToAdminResetQueueRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetStickyTaskListRequestConversion(t *testing.T) {
	testCases := []*types.ResetStickyTaskListRequest{
		nil,
		{},
		&testdata.ResetStickyTaskListRequest,
	}

	for _, original := range testCases {
		thriftObj := FromResetStickyTaskListRequest(original)
		roundTripObj := ToResetStickyTaskListRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetStickyTaskListResponseConversion(t *testing.T) {
	testCases := []*types.ResetStickyTaskListResponse{
		nil,
		{},
		&testdata.ResetStickyTaskListResponse,
	}

	for _, original := range testCases {
		thriftObj := FromResetStickyTaskListResponse(original)
		roundTripObj := ToResetStickyTaskListResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.ResetWorkflowExecutionRequest{
		nil,
		{},
		&testdata.ResetWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromResetWorkflowExecutionRequest(original)
		roundTripObj := ToResetWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestResetWorkflowExecutionResponseConversion(t *testing.T) {
	testCases := []*types.ResetWorkflowExecutionResponse{
		nil,
		{},
		&testdata.ResetWorkflowExecutionResponse,
	}

	for _, original := range testCases {
		thriftObj := FromResetWorkflowExecutionResponse(original)
		roundTripObj := ToResetWorkflowExecutionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskCanceledByIDRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskCanceledByIDRequest{
		nil,
		{},
		&testdata.RespondActivityTaskCanceledByIDRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskCanceledByIDRequest(original)
		roundTripObj := ToRespondActivityTaskCanceledByIDRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskCanceledRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskCanceledRequest{
		nil,
		{},
		&testdata.RespondActivityTaskCanceledRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskCanceledRequest(original)
		roundTripObj := ToRespondActivityTaskCanceledRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskCompletedByIDRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskCompletedByIDRequest{
		nil,
		{},
		&testdata.RespondActivityTaskCompletedByIDRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskCompletedByIDRequest(original)
		roundTripObj := ToRespondActivityTaskCompletedByIDRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskCompletedRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskCompletedRequest{
		nil,
		{},
		&testdata.RespondActivityTaskCompletedRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskCompletedRequest(original)
		roundTripObj := ToRespondActivityTaskCompletedRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskFailedByIDRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskFailedByIDRequest{
		nil,
		{},
		&testdata.RespondActivityTaskFailedByIDRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskFailedByIDRequest(original)
		roundTripObj := ToRespondActivityTaskFailedByIDRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondActivityTaskFailedRequestConversion(t *testing.T) {
	testCases := []*types.RespondActivityTaskFailedRequest{
		nil,
		{},
		&testdata.RespondActivityTaskFailedRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondActivityTaskFailedRequest(original)
		roundTripObj := ToRespondActivityTaskFailedRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondDecisionTaskCompletedRequestConversion(t *testing.T) {
	testCases := []*types.RespondDecisionTaskCompletedRequest{
		nil,
		{},
		&testdata.RespondDecisionTaskCompletedRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondDecisionTaskCompletedRequest(original)
		roundTripObj := ToRespondDecisionTaskCompletedRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondDecisionTaskFailedRequestConversion(t *testing.T) {
	testCases := []*types.RespondDecisionTaskFailedRequest{
		nil,
		{},
		&testdata.RespondDecisionTaskFailedRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondDecisionTaskFailedRequest(original)
		roundTripObj := ToRespondDecisionTaskFailedRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRespondQueryTaskCompletedRequestConversion(t *testing.T) {
	testCases := []*types.RespondQueryTaskCompletedRequest{
		nil,
		{},
		&testdata.RespondQueryTaskCompletedRequest,
	}

	for _, original := range testCases {
		thriftObj := FromRespondQueryTaskCompletedRequest(original)
		roundTripObj := ToRespondQueryTaskCompletedRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRetryPolicyConversion(t *testing.T) {
	testCases := []*types.RetryPolicy{
		nil,
		{},
		&testdata.RetryPolicy,
	}

	for _, original := range testCases {
		thriftObj := FromRetryPolicy(original)
		roundTripObj := ToRetryPolicy(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRetryTaskV2ErrorConversion(t *testing.T) {
	testCases := []*types.RetryTaskV2Error{
		nil,
		{},
		&testdata.RetryTaskV2Error,
	}

	for _, original := range testCases {
		thriftObj := FromRetryTaskV2Error(original)
		roundTripObj := ToRetryTaskV2Error(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRestartWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.RestartWorkflowExecutionRequest{
		nil,
		{},
		{Domain: "test-domain", WorkflowExecution: &testdata.WorkflowExecution},
	}

	for _, original := range testCases {
		thriftObj := FromRestartWorkflowExecutionRequest(original)
		roundTripObj := ToRestartWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestScheduleActivityTaskDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.ScheduleActivityTaskDecisionAttributes{
		nil,
		{},
		&testdata.ScheduleActivityTaskDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromScheduleActivityTaskDecisionAttributes(original)
		roundTripObj := ToScheduleActivityTaskDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSearchAttributesConversion(t *testing.T) {
	testCases := []*types.SearchAttributes{
		nil,
		{},
		&testdata.SearchAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromSearchAttributes(original)
		roundTripObj := ToSearchAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestServiceBusyErrorConversion(t *testing.T) {
	testCases := []*types.ServiceBusyError{
		nil,
		{},
		&testdata.ServiceBusyError,
	}

	for _, original := range testCases {
		thriftObj := FromServiceBusyError(original)
		roundTripObj := ToServiceBusyError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalExternalWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.SignalExternalWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.SignalExternalWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromSignalExternalWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToSignalExternalWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalExternalWorkflowExecutionFailedCauseConversion(t *testing.T) {
	testCases := []*types.SignalExternalWorkflowExecutionFailedCause{
		nil,
		types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr(),
		types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromSignalExternalWorkflowExecutionFailedCause(original)
		roundTripObj := ToSignalExternalWorkflowExecutionFailedCause(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalExternalWorkflowExecutionFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.SignalExternalWorkflowExecutionFailedEventAttributes{
		nil,
		{},
		&testdata.SignalExternalWorkflowExecutionFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromSignalExternalWorkflowExecutionFailedEventAttributes(original)
		roundTripObj := ToSignalExternalWorkflowExecutionFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalExternalWorkflowExecutionInitiatedEventAttributesConversion(t *testing.T) {
	testCases := []*types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		nil,
		{},
		&testdata.SignalExternalWorkflowExecutionInitiatedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromSignalExternalWorkflowExecutionInitiatedEventAttributes(original)
		roundTripObj := ToSignalExternalWorkflowExecutionInitiatedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalWithStartWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.SignalWithStartWorkflowExecutionRequest{
		nil,
		{},
		&testdata.SignalWithStartWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromSignalWithStartWorkflowExecutionRequest(original)
		roundTripObj := ToSignalWithStartWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.SignalWorkflowExecutionRequest{
		nil,
		{},
		&testdata.SignalWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromSignalWorkflowExecutionRequest(original)
		roundTripObj := ToSignalWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartChildWorkflowExecutionDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.StartChildWorkflowExecutionDecisionAttributes{
		nil,
		{},
		&testdata.StartChildWorkflowExecutionDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromStartChildWorkflowExecutionDecisionAttributes(original)
		roundTripObj := ToStartChildWorkflowExecutionDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartChildWorkflowExecutionFailedEventAttributesConversion(t *testing.T) {
	testCases := []*types.StartChildWorkflowExecutionFailedEventAttributes{
		nil,
		{},
		&testdata.StartChildWorkflowExecutionFailedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromStartChildWorkflowExecutionFailedEventAttributes(original)
		roundTripObj := ToStartChildWorkflowExecutionFailedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartChildWorkflowExecutionInitiatedEventAttributesConversion(t *testing.T) {
	testCases := []*types.StartChildWorkflowExecutionInitiatedEventAttributes{
		nil,
		{},
		&testdata.StartChildWorkflowExecutionInitiatedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromStartChildWorkflowExecutionInitiatedEventAttributes(original)
		roundTripObj := ToStartChildWorkflowExecutionInitiatedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartTimeFilterConversion(t *testing.T) {
	testCases := []*types.StartTimeFilter{
		nil,
		{},
		&testdata.StartTimeFilter,
	}

	for _, original := range testCases {
		thriftObj := FromStartTimeFilter(original)
		roundTripObj := ToStartTimeFilter(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartTimerDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.StartTimerDecisionAttributes{
		nil,
		{},
		&testdata.StartTimerDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromStartTimerDecisionAttributes(original)
		roundTripObj := ToStartTimerDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartWorkflowExecutionRequestConversion(t *testing.T) {
	testCases := []*types.StartWorkflowExecutionRequest{
		nil,
		{},
		&testdata.StartWorkflowExecutionRequest,
	}

	for _, original := range testCases {
		thriftObj := FromStartWorkflowExecutionRequest(original)
		roundTripObj := ToStartWorkflowExecutionRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartWorkflowExecutionAsyncRequestConversion(t *testing.T) {
	testCases := []*types.StartWorkflowExecutionAsyncRequest{
		nil,
		{},
		&testdata.StartWorkflowExecutionAsyncRequest,
	}

	for _, original := range testCases {
		thriftObj := FromStartWorkflowExecutionAsyncRequest(original)
		roundTripObj := ToStartWorkflowExecutionAsyncRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalWithStartWorkflowExecutionAsyncRequestConversion(t *testing.T) {
	testCases := []*types.SignalWithStartWorkflowExecutionAsyncRequest{
		nil,
		{},
		&testdata.SignalWithStartWorkflowExecutionAsyncRequest,
	}

	for _, original := range testCases {
		thriftObj := FromSignalWithStartWorkflowExecutionAsyncRequest(original)
		roundTripObj := ToSignalWithStartWorkflowExecutionAsyncRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestRestartWorkflowExecutionResponseConversion(t *testing.T) {
	testCases := []*types.RestartWorkflowExecutionResponse{
		nil,
		{},
		{RunID: "test-run-id"},
	}

	for _, original := range testCases {
		thriftObj := FromRestartWorkflowExecutionResponse(original)
		roundTripObj := ToRestartWorkflowExecutionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartWorkflowExecutionResponseConversion(t *testing.T) {
	testCases := []*types.StartWorkflowExecutionResponse{
		nil,
		{},
		{RunID: "test-run-id"},
	}

	for _, original := range testCases {
		thriftObj := FromStartWorkflowExecutionResponse(original)
		roundTripObj := ToStartWorkflowExecutionResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStartWorkflowExecutionAsyncResponseConversion(t *testing.T) {
	testCases := []*types.StartWorkflowExecutionAsyncResponse{
		nil,
		{},
		&testdata.StartWorkflowExecutionAsyncResponse,
	}

	for _, original := range testCases {
		thriftObj := FromStartWorkflowExecutionAsyncResponse(original)
		roundTripObj := ToStartWorkflowExecutionAsyncResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSignalWithStartWorkflowExecutionAsyncResponseConversion(t *testing.T) {
	testCases := []*types.SignalWithStartWorkflowExecutionAsyncResponse{
		nil,
		{},
		&testdata.SignalWithStartWorkflowExecutionAsyncResponse,
	}

	for _, original := range testCases {
		thriftObj := FromSignalWithStartWorkflowExecutionAsyncResponse(original)
		roundTripObj := ToSignalWithStartWorkflowExecutionAsyncResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStickyExecutionAttributesConversion(t *testing.T) {
	testCases := []*types.StickyExecutionAttributes{
		nil,
		{},
		&testdata.StickyExecutionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromStickyExecutionAttributes(original)
		roundTripObj := ToStickyExecutionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestSupportedClientVersionsConversion(t *testing.T) {
	testCases := []*types.SupportedClientVersions{
		nil,
		{},
		&testdata.SupportedClientVersions,
	}

	for _, original := range testCases {
		thriftObj := FromSupportedClientVersions(original)
		roundTripObj := ToSupportedClientVersions(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskIDBlockConversion(t *testing.T) {
	testCases := []*types.TaskIDBlock{
		nil,
		{},
		&testdata.TaskIDBlock,
	}

	for _, original := range testCases {
		thriftObj := FromTaskIDBlock(original)
		roundTripObj := ToTaskIDBlock(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskListConversion(t *testing.T) {
	testCases := []*types.TaskList{
		nil,
		{},
		&testdata.TaskList,
	}

	for _, original := range testCases {
		thriftObj := FromTaskList(original)
		roundTripObj := ToTaskList(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskListMetadataConversion(t *testing.T) {
	testCases := []*types.TaskListMetadata{
		nil,
		{},
		&testdata.TaskListMetadata,
	}

	for _, original := range testCases {
		thriftObj := FromTaskListMetadata(original)
		roundTripObj := ToTaskListMetadata(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskListPartitionMetadataConversion(t *testing.T) {
	testCases := []*types.TaskListPartitionMetadata{
		nil,
		{},
		&testdata.TaskListPartitionMetadata,
	}

	for _, original := range testCases {
		thriftObj := FromTaskListPartitionMetadata(original)
		roundTripObj := ToTaskListPartitionMetadata(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskListStatusConversion(t *testing.T) {
	testCases := []*types.TaskListStatus{
		nil,
		{},
		&testdata.TaskListStatus,
	}

	for _, original := range testCases {
		thriftObj := FromTaskListStatus(original)
		roundTripObj := ToTaskListStatus(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskListTypeConversion(t *testing.T) {
	testCases := []*types.TaskListType{
		nil,
		types.TaskListTypeDecision.Ptr(),
		types.TaskListTypeActivity.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromTaskListType(original)
		roundTripObj := ToTaskListType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTimeoutTypeConversion(t *testing.T) {
	testCases := []*types.TimeoutType{
		nil,
		types.TimeoutTypeStartToClose.Ptr(),
		types.TimeoutTypeScheduleToStart.Ptr(),
		types.TimeoutTypeScheduleToClose.Ptr(),
		types.TimeoutTypeHeartbeat.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromTimeoutType(original)
		roundTripObj := ToTimeoutType(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTimerCanceledEventAttributesConversion(t *testing.T) {
	testCases := []*types.TimerCanceledEventAttributes{
		nil,
		{},
		&testdata.TimerCanceledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromTimerCanceledEventAttributes(original)
		roundTripObj := ToTimerCanceledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTimerFiredEventAttributesConversion(t *testing.T) {
	testCases := []*types.TimerFiredEventAttributes{
		nil,
		{},
		&testdata.TimerFiredEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromTimerFiredEventAttributes(original)
		roundTripObj := ToTimerFiredEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTimerStartedEventAttributesConversion(t *testing.T) {
	testCases := []*types.TimerStartedEventAttributes{
		nil,
		{},
		&testdata.TimerStartedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromTimerStartedEventAttributes(original)
		roundTripObj := ToTimerStartedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTransientDecisionInfoConversion(t *testing.T) {
	testCases := []*types.TransientDecisionInfo{
		nil,
		{},
		&testdata.TransientDecisionInfo,
	}

	for _, original := range testCases {
		thriftObj := FromTransientDecisionInfo(original)
		roundTripObj := ToTransientDecisionInfo(thriftObj)
		opt := cmpopts.IgnoreFields(types.WorkflowExecutionStartedEventAttributes{}, "ParentWorkflowDomainID")
		if diff := cmp.Diff(original, roundTripObj, opt); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestUpdateDomainRequestConversion(t *testing.T) {
	testCases := []*types.UpdateDomainRequest{
		nil,
		{},
		&testdata.UpdateDomainRequest,
	}

	for _, original := range testCases {
		thriftObj := FromUpdateDomainRequest(original)
		roundTripObj := ToUpdateDomainRequest(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestUpdateDomainResponseConversion(t *testing.T) {
	testCases := []*types.UpdateDomainResponse{
		nil,
		{},
		&testdata.UpdateDomainResponse,
	}

	for _, original := range testCases {
		thriftObj := FromUpdateDomainResponse(original)
		roundTripObj := ToUpdateDomainResponse(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestUpsertWorkflowSearchAttributesDecisionAttributesConversion(t *testing.T) {
	testCases := []*types.UpsertWorkflowSearchAttributesDecisionAttributes{
		nil,
		{},
		&testdata.UpsertWorkflowSearchAttributesDecisionAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromUpsertWorkflowSearchAttributesDecisionAttributes(original)
		roundTripObj := ToUpsertWorkflowSearchAttributesDecisionAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestUpsertWorkflowSearchAttributesEventAttributesConversion(t *testing.T) {
	testCases := []*types.UpsertWorkflowSearchAttributesEventAttributes{
		nil,
		{},
		&testdata.UpsertWorkflowSearchAttributesEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromUpsertWorkflowSearchAttributesEventAttributes(original)
		roundTripObj := ToUpsertWorkflowSearchAttributesEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestVersionHistoriesConversion(t *testing.T) {
	testCases := []*types.VersionHistories{
		nil,
		{},
		&testdata.VersionHistories,
	}

	for _, original := range testCases {
		thriftObj := FromVersionHistories(original)
		roundTripObj := ToVersionHistories(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestVersionHistoryConversion(t *testing.T) {
	testCases := []*types.VersionHistory{
		nil,
		{},
		&testdata.VersionHistory,
	}

	for _, original := range testCases {
		thriftObj := FromVersionHistory(original)
		roundTripObj := ToVersionHistory(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestVersionHistoryItemConversion(t *testing.T) {
	testCases := []*types.VersionHistoryItem{
		nil,
		{},
		&testdata.VersionHistoryItem,
	}

	for _, original := range testCases {
		thriftObj := FromVersionHistoryItem(original)
		roundTripObj := ToVersionHistoryItem(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkerVersionInfoConversion(t *testing.T) {
	testCases := []*types.WorkerVersionInfo{
		nil,
		{},
		&testdata.WorkerVersionInfo,
	}

	for _, original := range testCases {
		thriftObj := FromWorkerVersionInfo(original)
		roundTripObj := ToWorkerVersionInfo(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionConversion(t *testing.T) {
	testCases := []*types.WorkflowExecution{
		nil,
		{},
		&testdata.WorkflowExecution,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecution(original)
		roundTripObj := ToWorkflowExecution(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionAlreadyStartedErrorConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionAlreadyStartedError{
		nil,
		{},
		&testdata.WorkflowExecutionAlreadyStartedError,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionAlreadyStartedError(original)
		roundTripObj := ToWorkflowExecutionAlreadyStartedError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionCancelRequestedEventAttributesConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionCancelRequestedEventAttributes{
		nil,
		{},
		&testdata.WorkflowExecutionCancelRequestedEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionCancelRequestedEventAttributes(original)
		roundTripObj := ToWorkflowExecutionCancelRequestedEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionCanceledEventAttributesConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionCanceledEventAttributes{
		nil,
		{},
		&testdata.WorkflowExecutionCanceledEventAttributes,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionCanceledEventAttributes(original)
		roundTripObj := ToWorkflowExecutionCanceledEventAttributes(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionCloseStatusConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionCloseStatus{
		nil,
		types.WorkflowExecutionCloseStatusCompleted.Ptr(),
		types.WorkflowExecutionCloseStatusFailed.Ptr(),
		types.WorkflowExecutionCloseStatusCanceled.Ptr(),
		types.WorkflowExecutionCloseStatusTerminated.Ptr(),
		types.WorkflowExecutionCloseStatusContinuedAsNew.Ptr(),
		types.WorkflowExecutionCloseStatusTimedOut.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionCloseStatus(original)
		roundTripObj := ToWorkflowExecutionCloseStatus(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowExecutionFilterConversion(t *testing.T) {
	testCases := []*types.WorkflowExecutionFilter{
		nil,
		{},
		&testdata.WorkflowExecutionFilter,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowExecutionFilter(original)
		roundTripObj := ToWorkflowExecutionFilter(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowIDReusePolicyConversion(t *testing.T) {
	testCases := []*types.WorkflowIDReusePolicy{
		nil,
		types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		types.WorkflowIDReusePolicyAllowDuplicateFailedOnly.Ptr(),
		types.WorkflowIDReusePolicyRejectDuplicate.Ptr(),
		types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowIDReusePolicy(original)
		roundTripObj := ToWorkflowIDReusePolicy(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestWorkflowTypeFilterConversion(t *testing.T) {
	testCases := []*types.WorkflowTypeFilter{
		nil,
		{},
		&testdata.WorkflowTypeFilter,
	}

	for _, original := range testCases {
		thriftObj := FromWorkflowTypeFilter(original)
		roundTripObj := ToWorkflowTypeFilter(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestHistoryBranchRangeArrayConversion(t *testing.T) {
	testCases := [][]*types.HistoryBranchRange{
		nil,
		{},
		{
			{BranchID: "test-branch-id-1", BeginNodeID: 1, EndNodeID: 2},
			{BranchID: "test-branch-id-2", BeginNodeID: 5, EndNodeID: 6},
		},
	}

	for _, original := range testCases {
		thriftObj := FromHistoryBranchRangeArray(original)
		roundTripObj := ToHistoryBranchRangeArray(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestActivityLocalDispatchInfoMapConversion(t *testing.T) {
	testCases := []map[string]*types.ActivityLocalDispatchInfo{
		nil,
		{},
		{
			"test-key-1": &testdata.ActivityLocalDispatchInfo,
			"test-key-2": &testdata.ActivityLocalDispatchInfo,
		},
	}

	for _, original := range testCases {
		thriftObj := FromActivityLocalDispatchInfoMap(original)
		roundTripObj := ToActivityLocalDispatchInfoMap(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestHistoryArrayConversion(t *testing.T) {
	testCases := [][]*types.History{
		nil,
		{},
		{
			{Events: []*types.HistoryEvent{{ID: 1}, {ID: 2}}},
			{Events: []*types.HistoryEvent{{ID: 3}, {ID: 4}}},
		},
	}

	for _, original := range testCases {
		thriftObj := FromHistoryArray(original)
		roundTripObj := ToHistoryArray(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestStickyWorkerUnavailableErrorConversion(t *testing.T) {
	testCases := []*types.StickyWorkerUnavailableError{
		nil,
		{},
		&testdata.StickyWorkerUnavailableError,
	}

	for _, original := range testCases {
		thriftObj := FromStickyWorkerUnavailableError(original)
		roundTripObj := ToStickyWorkerUnavailableError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestFromTaskListNotOwnedByHostError(t *testing.T) {
	testCases := []*cadence_errors.TaskListNotOwnedByHostError{
		nil,
		&testdata.TaskListNotOwnedByHostError,
	}

	for _, original := range testCases {
		thriftObj := FromTaskListNotOwnedByHostError(original)
		roundTripObj := ToTaskListNotOwnedByHostError(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestAny(t *testing.T) {
	t.Run("sanity check", func(t *testing.T) {
		// test the main fields are mapped correctly
		internal := types.Any{
			ValueType: "testing",
			Value:     []byte(`test`),
		}
		rpc := shared.Any{
			ValueType: common.StringPtr("testing"),
			Value:     []byte(`test`),
		}
		assert.Equal(t, &rpc, FromAny(&internal))
		assert.Equal(t, &internal, ToAny(&rpc))
	})
	t.Run("round trip nils", func(t *testing.T) {
		// somewhat annoying in fuzzing and there are few possibilities, so tested separately
		assert.Nil(t, FromAny(ToAny(nil)), "nil thrift -> internal -> thrift => should result in nil")
		assert.Nil(t, ToAny(FromAny(nil)), "nil internal -> thrift -> internal => should result in nil")
	})

	t.Run("round trip from internal", func(t *testing.T) {
		// fuzz test to check round trip data works as it should when starting from internal types
		testutils.EnsureFuzzCoverage(t, []string{
			"empty data", "filled data",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			var orig types.Any
			f.Fuzz(&orig)
			out := ToAny(FromAny(&orig))
			assert.Equal(t, &orig, out, "did not survive round-tripping")

			// report what branch of behavior was executed so we can ensure stable coverage
			if len(orig.Value) == 0 {
				return "empty data" // ignoring nil vs empty difference
			}
			return "filled data"
		})
	})
	t.Run("round trip from thrift", func(t *testing.T) {
		// fuzz test to check round trip data works as it should
		testutils.EnsureFuzzCoverage(t, []string{
			"nil id", "empty data", "filled data",
		}, func(t *testing.T, f *fuzz.Fuzzer) string {
			var orig shared.Any
			f.Fuzz(&orig)
			out := FromAny(ToAny(&orig))

			if orig.ValueType == nil {
				// internal type does not support nil strings, so it will be transformed to an empty string.
				assert.Equal(t, "", *out.ValueType, "empty value type did not survive round-tripping")
				assert.Equal(t, orig.Value, out.Value, "value did not survive round-tripping")
				return "nil id"
			}
			// non-nil ValueType can check all fields
			assert.Equal(t, &orig, out, "did not survive round-tripping")
			if len(orig.Value) == 0 {
				return "empty data" // ignoring nil vs empty difference
			}
			return "filled data"
		})
	})
}

func TestTaskKeyConversion(t *testing.T) {
	testCases := []*types.TaskKey{
		nil,
		&types.TaskKey{},
		&testdata.TaskKey,
	}

	for _, original := range testCases {
		thriftObj := FromTaskKey(original)
		roundTripObj := ToTaskKey(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestTaskRangeConversion(t *testing.T) {
	testCases := []*types.TaskRange{
		nil,
		&types.TaskRange{},
		&testdata.TaskRange,
	}

	for _, original := range testCases {
		thriftObj := FromTaskRange(original)
		roundTripObj := ToTaskRange(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestVirtualSliceStateConversion(t *testing.T) {
	testCases := []*types.VirtualSliceState{
		nil,
		&types.VirtualSliceState{},
		&testdata.VirtualSliceState,
	}

	for _, original := range testCases {
		thriftObj := FromVirtualSliceState(original)
		roundTripObj := ToVirtualSliceState(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestVirtualQueueStateConversion(t *testing.T) {
	testCases := []*types.VirtualQueueState{
		nil,
		&types.VirtualQueueState{},
		&testdata.VirtualQueueState,
	}

	for _, original := range testCases {
		thriftObj := FromVirtualQueueState(original)
		roundTripObj := ToVirtualQueueState(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestQueueStateConversion(t *testing.T) {
	testCases := []*types.QueueState{
		nil,
		&types.QueueState{},
		&testdata.QueueState,
	}

	for _, original := range testCases {
		thriftObj := FromQueueState(original)
		roundTripObj := ToQueueState(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

// TODO: The way we're testing mappers doesn't have good coverage. The round trip tests don't cover serialization and deserialization of thrift.
// We should also generate test data in thrift types and do round trip tests from thrift to internal and back to thrift.
func TestActiveClusterSelectionPolicyConversion(t *testing.T) {
	testCases := []*types.ActiveClusterSelectionPolicy{
		nil,
		{},
		&testdata.ActiveClusterSelectionPolicyExternalEntity,
		&testdata.ActiveClusterSelectionPolicyRegionSticky,
		&testdata.ActiveClusterSelectionPolicyWithClusterAttribute,
	}

	for _, original := range testCases {
		thriftObj := FromActiveClusterSelectionPolicy(original)
		roundTripObj := ToActiveClusterSelectionPolicy(thriftObj)
		assert.Equal(t, original, roundTripObj)
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
		thriftObj := FromActiveClusters(original)
		roundTripObj := ToActiveClusters(thriftObj)
		assert.Equal(t, original, roundTripObj)
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
		thriftObj := FromClusterAttributeScope(original)
		roundTripObj := ToClusterAttributeScope(thriftObj)
		assert.Equal(t, original, roundTripObj)
	}
}

func TestListFailoverHistoryResponseConversion(t *testing.T) {
	fuzzer := testdatagen.New(t)
	for i := 0; i < 100; i++ {
		var response types.ListFailoverHistoryResponse
		fuzzer.Fuzz(&response)
		thriftResponse := FromListFailoverHistoryResponse(&response)

		toResponse := ToListFailoverHistoryResponse(thriftResponse)
		assert.Equal(t, &response, toResponse)
	}
}
