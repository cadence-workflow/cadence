// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	domainUUID                = "test-domainUUID"
	mutableStateInCache       = "test-cache"
	mutableStateInDatabase    = "test-database"
	domainIDs                 = []string{"id1", "id2", "id3"}
	shardIDs                  = []int32{1, 2, 3}
	expectedNextEventID       = int64(123)
	nextEventID               = int64(456)
	previousStartedEventID    = int64(789)
	stickyTaskListTimeout     = int32(100)
	workflowCloseState        = int32(2)
	clientFeatureVersion      = "v1.0"
	clientImpl                = "test-client"
	currentBranchToken        = []byte("branch-token")
	requestID                 = "test-requestID"
	scheduleID                = int64(100)
	taskID                    = int64(200)
	startedTimestamp          = int64(123456789)
	attempt                   = int64(3)
	attempt32                 = int32(3)
	expirationTimestamp       = int64(1234567890)
	firstDecisionBackoff      = int32(60)
	partitionConfig           = map[string]string{"partition1": "value1"}
	scheduledTime             = int64(1617181920)
	startedTime               = int64(1617182920)
	lastHeartbeatTime         = int64(1617183920)
	lastFailureReason         = "test-failure"
	lastWorkerIdentity        = "test-worker"
	details                   = []byte("test-details")
	lastFailureDetails        = []byte("failure-details")
	versionHistory            = &VersionHistory{}
	sourceCluster             = "source-cluster"
	shardID                   = int64(1000)
	terminateRequest          = &TerminateWorkflowExecutionRequest{}
	externalWorkflowExecution = &WorkflowExecution{}
	childWorkflowOnly         = true
	pendingShards             = []int32{1, 2, 3}
	completedShardCount       = int32(5)
	workflowID                = "test-workflow-id"
	runID                     = "test-run-id"
	scheduledID               = int64(202)
	startedID                 = int64(303)
	version                   = int64(10)
)

func TestDescribeMutableStateRequest(t *testing.T) {
	testStruct := DescribeMutableStateRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *DescribeMutableStateRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res) // Should return empty string when struct is nil
}

func TestDescribeMutableStateResponse(t *testing.T) {
	testStruct := DescribeMutableStateResponse{
		MutableStateInCache:    mutableStateInCache,
		MutableStateInDatabase: mutableStateInDatabase,
	}

	assert.Equal(t, mutableStateInCache, testStruct.MutableStateInCache)
	assert.Equal(t, mutableStateInDatabase, testStruct.MutableStateInDatabase)
}

func TestHistoryDescribeWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryDescribeWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *HistoryDescribeWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res) // Should return empty string when struct is nil
}

func TestDomainFilter_GetDomainIDs(t *testing.T) {
	testStruct := DomainFilter{
		DomainIDs: domainIDs,
	}

	// Non-nil struct test
	res := testStruct.GetDomainIDs()
	assert.Equal(t, domainIDs, res)

	// Nil struct test
	var nilStruct *DomainFilter
	res = nilStruct.GetDomainIDs()
	assert.Empty(t, res) // Should return empty slice when struct is nil
}

func TestDomainFilter_GetReverseMatch(t *testing.T) {
	testStruct := DomainFilter{
		ReverseMatch: true,
	}

	// Non-nil struct test
	res := testStruct.GetReverseMatch()
	assert.True(t, res)

	// Nil struct test
	var nilStruct *DomainFilter
	res = nilStruct.GetReverseMatch()
	assert.False(t, res) // Should return false when struct is nil
}

func TestFailoverMarkerToken_GetShardIDs(t *testing.T) {
	testStruct := FailoverMarkerToken{
		ShardIDs: shardIDs,
	}

	// Non-nil struct test
	res := testStruct.GetShardIDs()
	assert.Equal(t, shardIDs, res)

	// Nil struct test
	var nilStruct *FailoverMarkerToken
	res = nilStruct.GetShardIDs()
	assert.Empty(t, res) // Should return empty slice when struct is nil
}

func TestFailoverMarkerToken_GetFailoverMarker(t *testing.T) {
	failoverMarker := &FailoverMarkerAttributes{}

	testStruct := FailoverMarkerToken{
		FailoverMarker: failoverMarker,
	}

	// Non-nil struct test
	res := testStruct.GetFailoverMarker()
	assert.Equal(t, failoverMarker, res)

	// Nil struct test
	var nilStruct *FailoverMarkerToken
	res = nilStruct.GetFailoverMarker()
	assert.Nil(t, res) // Should return nil when struct is nil
}

func TestGetMutableStateRequest_GetDomainUUID(t *testing.T) {
	testStruct := GetMutableStateRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *GetMutableStateRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestGetMutableStateRequest_GetExpectedNextEventID(t *testing.T) {
	testStruct := GetMutableStateRequest{
		ExpectedNextEventID: expectedNextEventID,
	}

	// Non-nil struct test
	res := testStruct.GetExpectedNextEventID()
	assert.Equal(t, expectedNextEventID, res)

	// Nil struct test
	var nilStruct *GetMutableStateRequest
	res = nilStruct.GetExpectedNextEventID()
	assert.Equal(t, int64(0), res)
}

func TestGetMutableStateResponse_GetNextEventID(t *testing.T) {
	testStruct := GetMutableStateResponse{
		NextEventID: nextEventID,
	}

	// Non-nil struct test
	res := testStruct.GetNextEventID()
	assert.Equal(t, nextEventID, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetNextEventID()
	assert.Equal(t, int64(0), res)
}

func TestGetMutableStateResponse_GetPreviousStartedEventID(t *testing.T) {
	testStruct := GetMutableStateResponse{
		PreviousStartedEventID: &previousStartedEventID,
	}

	// Non-nil struct test
	res := testStruct.GetPreviousStartedEventID()
	assert.Equal(t, previousStartedEventID, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetPreviousStartedEventID()
	assert.Equal(t, int64(0), res)
}

func TestGetMutableStateResponse_GetStickyTaskList(t *testing.T) {
	stickyTaskList := &TaskList{}
	testStruct := GetMutableStateResponse{
		StickyTaskList: stickyTaskList,
	}

	// Non-nil struct test
	res := testStruct.GetStickyTaskList()
	assert.Equal(t, stickyTaskList, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetStickyTaskList()
	assert.Nil(t, res)
}

func TestGetMutableStateResponse_GetClientFeatureVersion(t *testing.T) {
	testStruct := GetMutableStateResponse{
		ClientFeatureVersion: clientFeatureVersion,
	}

	// Non-nil struct test
	res := testStruct.GetClientFeatureVersion()
	assert.Equal(t, clientFeatureVersion, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetClientFeatureVersion()
	assert.Equal(t, "", res)
}

func TestGetMutableStateResponse_GetClientImpl(t *testing.T) {
	testStruct := GetMutableStateResponse{
		ClientImpl: clientImpl,
	}

	// Non-nil struct test
	res := testStruct.GetClientImpl()
	assert.Equal(t, clientImpl, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetClientImpl()
	assert.Equal(t, "", res)
}

func TestGetMutableStateResponse_GetIsWorkflowRunning(t *testing.T) {
	testStruct := GetMutableStateResponse{
		IsWorkflowRunning: true,
	}

	// Non-nil struct test
	res := testStruct.GetIsWorkflowRunning()
	assert.True(t, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetIsWorkflowRunning()
	assert.False(t, res)
}

func TestGetMutableStateResponse_GetStickyTaskListScheduleToStartTimeout(t *testing.T) {
	testStruct := GetMutableStateResponse{
		StickyTaskListScheduleToStartTimeout: &stickyTaskListTimeout,
	}

	// Non-nil struct test
	res := testStruct.GetStickyTaskListScheduleToStartTimeout()
	assert.Equal(t, stickyTaskListTimeout, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetStickyTaskListScheduleToStartTimeout()
	assert.Equal(t, int32(0), res)
}

func TestGetMutableStateResponse_GetCurrentBranchToken(t *testing.T) {
	testStruct := GetMutableStateResponse{
		CurrentBranchToken: currentBranchToken,
	}

	// Non-nil struct test
	res := testStruct.GetCurrentBranchToken()
	assert.Equal(t, currentBranchToken, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetCurrentBranchToken()
	assert.Nil(t, res)
}

func TestGetMutableStateResponse_GetWorkflowCloseState(t *testing.T) {
	testStruct := GetMutableStateResponse{
		WorkflowCloseState: &workflowCloseState,
	}

	// Non-nil struct test
	res := testStruct.GetWorkflowCloseState()
	assert.Equal(t, workflowCloseState, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetWorkflowCloseState()
	assert.Equal(t, int32(0), res)
}

func TestGetMutableStateResponse_GetVersionHistories(t *testing.T) {
	versionHistories := &VersionHistories{}
	testStruct := GetMutableStateResponse{
		VersionHistories: versionHistories,
	}

	// Non-nil struct test
	res := testStruct.GetVersionHistories()
	assert.Equal(t, versionHistories, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetVersionHistories()
	assert.Nil(t, res)
}

func TestGetMutableStateResponse_GetIsStickyTaskListEnabled(t *testing.T) {
	testStruct := GetMutableStateResponse{
		IsStickyTaskListEnabled: true,
	}

	// Non-nil struct test
	res := testStruct.GetIsStickyTaskListEnabled()
	assert.True(t, res)

	// Nil struct test
	var nilStruct *GetMutableStateResponse
	res = nilStruct.GetIsStickyTaskListEnabled()
	assert.False(t, res)
}

func TestNotifyFailoverMarkersRequest_GetFailoverMarkerTokens(t *testing.T) {
	failoverMarkerTokens := []*FailoverMarkerToken{}
	testStruct := NotifyFailoverMarkersRequest{
		FailoverMarkerTokens: failoverMarkerTokens,
	}

	// Non-nil struct test
	res := testStruct.GetFailoverMarkerTokens()
	assert.Equal(t, failoverMarkerTokens, res)

	// Nil struct test
	var nilStruct *NotifyFailoverMarkersRequest
	res = nilStruct.GetFailoverMarkerTokens()
	assert.Nil(t, res)
}

func TestParentExecutionInfo_GetDomain(t *testing.T) {
	testStruct := ParentExecutionInfo{
		Domain: "test-domain",
	}

	// Non-nil struct test
	res := testStruct.GetDomain()
	assert.Equal(t, "test-domain", res)

	// Nil struct test
	var nilStruct *ParentExecutionInfo
	res = nilStruct.GetDomain()
	assert.Equal(t, "", res)
}

func TestParentExecutionInfo_GetExecution(t *testing.T) {
	execution := &WorkflowExecution{}
	testStruct := ParentExecutionInfo{
		Execution: execution,
	}

	// Non-nil struct test
	res := testStruct.GetExecution()
	assert.Equal(t, execution, res)

	// Nil struct test
	var nilStruct *ParentExecutionInfo
	res = nilStruct.GetExecution()
	assert.Nil(t, res)
}

func TestPollMutableStateRequest_GetDomainUUID(t *testing.T) {
	testStruct := PollMutableStateRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *PollMutableStateRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestPollMutableStateResponse_GetNextEventID(t *testing.T) {
	testStruct := PollMutableStateResponse{
		NextEventID: nextEventID,
	}

	// Non-nil struct test
	res := testStruct.GetNextEventID()
	assert.Equal(t, nextEventID, res)

	// Nil struct test
	var nilStruct *PollMutableStateResponse
	res = nilStruct.GetNextEventID()
	assert.Equal(t, int64(0), res)
}

func TestPollMutableStateResponse_GetLastFirstEventID(t *testing.T) {
	testStruct := PollMutableStateResponse{
		LastFirstEventID: nextEventID,
	}

	// Non-nil struct test
	res := testStruct.GetLastFirstEventID()
	assert.Equal(t, nextEventID, res)

	// Nil struct test
	var nilStruct *PollMutableStateResponse
	res = nilStruct.GetLastFirstEventID()
	assert.Equal(t, int64(0), res)
}

func TestPollMutableStateResponse_GetWorkflowCloseState(t *testing.T) {
	testStruct := PollMutableStateResponse{
		WorkflowCloseState: &workflowCloseState,
	}

	// Non-nil struct test
	res := testStruct.GetWorkflowCloseState()
	assert.Equal(t, workflowCloseState, res)

	// Nil struct test
	var nilStruct *PollMutableStateResponse
	res = nilStruct.GetWorkflowCloseState()
	assert.Equal(t, int32(0), res)
}

func TestProcessingQueueState_GetLevel(t *testing.T) {
	level := int32(2)
	testStruct := ProcessingQueueState{
		Level: &level,
	}

	// Non-nil struct test
	res := testStruct.GetLevel()
	assert.Equal(t, level, res)

	// Nil struct test
	var nilStruct *ProcessingQueueState
	res = nilStruct.GetLevel()
	assert.Equal(t, int32(0), res)
}

func TestProcessingQueueState_GetAckLevel(t *testing.T) {
	ackLevel := int64(1111)
	testStruct := ProcessingQueueState{
		AckLevel: &ackLevel,
	}

	// Non-nil struct test
	res := testStruct.GetAckLevel()
	assert.Equal(t, ackLevel, res)

	// Nil struct test
	var nilStruct *ProcessingQueueState
	res = nilStruct.GetAckLevel()
	assert.Equal(t, int64(0), res)
}

func TestProcessingQueueState_GetMaxLevel(t *testing.T) {
	maxLevel := int64(2222)
	testStruct := ProcessingQueueState{
		MaxLevel: &maxLevel,
	}

	// Non-nil struct test
	res := testStruct.GetMaxLevel()
	assert.Equal(t, maxLevel, res)

	// Nil struct test
	var nilStruct *ProcessingQueueState
	res = nilStruct.GetMaxLevel()
	assert.Equal(t, int64(0), res)
}

func TestProcessingQueueState_GetDomainFilter(t *testing.T) {
	domainFilter := &DomainFilter{}
	testStruct := ProcessingQueueState{
		DomainFilter: domainFilter,
	}

	// Non-nil struct test
	res := testStruct.GetDomainFilter()
	assert.Equal(t, domainFilter, res)

	// Nil struct test
	var nilStruct *ProcessingQueueState
	res = nilStruct.GetDomainFilter()
	assert.Nil(t, res)
}

func TestHistoryQueryWorkflowRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryQueryWorkflowRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *HistoryQueryWorkflowRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryQueryWorkflowRequest_GetRequest(t *testing.T) {
	request := &QueryWorkflowRequest{}
	testStruct := HistoryQueryWorkflowRequest{
		Request: request,
	}

	// Non-nil struct test
	res := testStruct.GetRequest()
	assert.Equal(t, request, res)

	// Nil struct test
	var nilStruct *HistoryQueryWorkflowRequest
	res = nilStruct.GetRequest()
	assert.Nil(t, res)
}

func TestHistoryQueryWorkflowResponse_GetResponse(t *testing.T) {
	response := &QueryWorkflowResponse{}
	testStruct := HistoryQueryWorkflowResponse{
		Response: response,
	}

	// Non-nil struct test
	res := testStruct.GetResponse()
	assert.Equal(t, response, res)

	// Nil struct test
	var nilStruct *HistoryQueryWorkflowResponse
	res = nilStruct.GetResponse()
	assert.Nil(t, res)
}

func TestHistoryReapplyEventsRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryReapplyEventsRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *HistoryReapplyEventsRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryReapplyEventsRequest_GetRequest(t *testing.T) {
	request := &ReapplyEventsRequest{}
	testStruct := HistoryReapplyEventsRequest{
		Request: request,
	}

	// Non-nil struct test
	res := testStruct.GetRequest()
	assert.Equal(t, request, res)

	// Nil struct test
	var nilStruct *HistoryReapplyEventsRequest
	res = nilStruct.GetRequest()
	assert.Nil(t, res)
}

func TestHistoryRecordActivityTaskHeartbeatRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *HistoryRecordActivityTaskHeartbeatRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestRecordActivityTaskStartedRequest_GetDomainUUID(t *testing.T) {
	testStruct := RecordActivityTaskStartedRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestRecordActivityTaskStartedRequest_GetScheduleID(t *testing.T) {
	testStruct := RecordActivityTaskStartedRequest{
		ScheduleID: scheduleID,
	}

	// Non-nil struct test
	res := testStruct.GetScheduleID()
	assert.Equal(t, scheduleID, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedRequest
	res = nilStruct.GetScheduleID()
	assert.Equal(t, int64(0), res)
}

func TestRecordActivityTaskStartedRequest_GetTaskID(t *testing.T) {
	testStruct := RecordActivityTaskStartedRequest{
		TaskID: taskID,
	}

	// Non-nil struct test
	res := testStruct.GetTaskID()
	assert.Equal(t, taskID, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedRequest
	res = nilStruct.GetTaskID()
	assert.Equal(t, int64(0), res)
}

func TestRecordActivityTaskStartedRequest_GetRequestID(t *testing.T) {
	testStruct := RecordActivityTaskStartedRequest{
		RequestID: requestID,
	}

	// Non-nil struct test
	res := testStruct.GetRequestID()
	assert.Equal(t, requestID, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedRequest
	res = nilStruct.GetRequestID()
	assert.Equal(t, "", res)
}

func TestRecordActivityTaskStartedResponse_GetAttempt(t *testing.T) {
	testStruct := RecordActivityTaskStartedResponse{
		Attempt: attempt,
	}

	// Non-nil struct test
	res := testStruct.GetAttempt()
	assert.Equal(t, attempt, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedResponse
	res = nilStruct.GetAttempt()
	assert.Equal(t, int64(0), res)
}

func TestRecordActivityTaskStartedResponse_GetScheduledTimestampOfThisAttempt(t *testing.T) {
	testStruct := RecordActivityTaskStartedResponse{
		ScheduledTimestampOfThisAttempt: &startedTimestamp,
	}

	// Non-nil struct test
	res := testStruct.GetScheduledTimestampOfThisAttempt()
	assert.Equal(t, startedTimestamp, res)

	// Nil struct test
	var nilStruct *RecordActivityTaskStartedResponse
	res = nilStruct.GetScheduledTimestampOfThisAttempt()
	assert.Equal(t, int64(0), res)
}

func TestRecordChildExecutionCompletedRequest_GetDomainUUID(t *testing.T) {
	testStruct := RecordChildExecutionCompletedRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *RecordChildExecutionCompletedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestRecordDecisionTaskStartedRequest_GetDomainUUID(t *testing.T) {
	testStruct := RecordDecisionTaskStartedRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestRecordDecisionTaskStartedRequest_GetScheduleID(t *testing.T) {
	testStruct := RecordDecisionTaskStartedRequest{
		ScheduleID: scheduleID,
	}

	// Non-nil struct test
	res := testStruct.GetScheduleID()
	assert.Equal(t, scheduleID, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedRequest
	res = nilStruct.GetScheduleID()
	assert.Equal(t, int64(0), res)
}

func TestRecordDecisionTaskStartedRequest_GetRequestID(t *testing.T) {
	testStruct := RecordDecisionTaskStartedRequest{
		RequestID: requestID,
	}

	// Non-nil struct test
	res := testStruct.GetRequestID()
	assert.Equal(t, requestID, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedRequest
	res = nilStruct.GetRequestID()
	assert.Equal(t, "", res)
}

func TestRecordDecisionTaskStartedResponse_GetPreviousStartedEventID(t *testing.T) {
	testStruct := RecordDecisionTaskStartedResponse{
		PreviousStartedEventID: &previousStartedEventID,
	}

	// Non-nil struct test
	res := testStruct.GetPreviousStartedEventID()
	assert.Equal(t, previousStartedEventID, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedResponse
	res = nilStruct.GetPreviousStartedEventID()
	assert.Equal(t, int64(0), res)
}

func TestRecordDecisionTaskStartedResponse_GetScheduledEventID(t *testing.T) {
	scheduledEventID := int64(1001)
	testStruct := RecordDecisionTaskStartedResponse{
		ScheduledEventID: scheduledEventID,
	}

	// Non-nil struct test
	res := testStruct.GetScheduledEventID()
	assert.Equal(t, scheduledEventID, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedResponse
	res = nilStruct.GetScheduledEventID()
	assert.Equal(t, int64(0), res)
}

func TestRecordDecisionTaskStartedResponse_GetAttempt(t *testing.T) {
	testStruct := RecordDecisionTaskStartedResponse{
		Attempt: attempt,
	}

	// Non-nil struct test
	res := testStruct.GetAttempt()
	assert.Equal(t, attempt, res)

	// Nil struct test
	var nilStruct *RecordDecisionTaskStartedResponse
	res = nilStruct.GetAttempt()
	assert.Equal(t, int64(0), res)
}

func TestRemoveSignalMutableStateRequest_GetDomainUUID(t *testing.T) {
	testStruct := RemoveSignalMutableStateRequest{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *RemoveSignalMutableStateRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestRemoveSignalMutableStateRequest_GetRequestID(t *testing.T) {
	testStruct := RemoveSignalMutableStateRequest{
		RequestID: requestID,
	}

	// Non-nil struct test
	res := testStruct.GetRequestID()
	assert.Equal(t, requestID, res)

	// Nil struct test
	var nilStruct *RemoveSignalMutableStateRequest
	res = nilStruct.GetRequestID()
	assert.Equal(t, "", res)
}

func TestReplicateEventsV2Request_GetDomainUUID(t *testing.T) {
	testStruct := ReplicateEventsV2Request{
		DomainUUID: domainUUID,
	}

	// Non-nil struct test
	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	// Nil struct test
	var nilStruct *ReplicateEventsV2Request
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRefreshWorkflowTasksRequest_GetRequest(t *testing.T) {
	// Define a sample RefreshWorkflowTasksRequest
	sampleRequest := &RefreshWorkflowTasksRequest{}

	// Test when the struct is non-nil and Request is non-nil
	testStruct := HistoryRefreshWorkflowTasksRequest{
		Request: sampleRequest,
	}

	res := testStruct.GetRequest()
	assert.Equal(t, sampleRequest, res)

	// Test when the struct is non-nil but Request is nil
	testStructWithNilRequest := HistoryRefreshWorkflowTasksRequest{
		Request: nil,
	}

	res = testStructWithNilRequest.GetRequest()
	assert.Nil(t, res)

	// Test when the struct itself is nil
	var nilStruct *HistoryRefreshWorkflowTasksRequest
	res = nilStruct.GetRequest()
	assert.Nil(t, res)
}

func TestHistoryRequestCancelWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRequestCancelWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRequestCancelWorkflowExecutionRequest_GetCancelRequest(t *testing.T) {
	cancelRequest := &RequestCancelWorkflowExecutionRequest{}
	testStruct := HistoryRequestCancelWorkflowExecutionRequest{
		CancelRequest: cancelRequest,
	}

	res := testStruct.GetCancelRequest()
	assert.Equal(t, cancelRequest, res)

	var nilStruct *HistoryRequestCancelWorkflowExecutionRequest
	res = nilStruct.GetCancelRequest()
	assert.Nil(t, res)
}

func TestHistoryRequestCancelWorkflowExecutionRequest_GetExternalWorkflowExecution(t *testing.T) {
	externalWorkflowExecution := &WorkflowExecution{}
	testStruct := HistoryRequestCancelWorkflowExecutionRequest{
		ExternalWorkflowExecution: externalWorkflowExecution,
	}

	res := testStruct.GetExternalWorkflowExecution()
	assert.Equal(t, externalWorkflowExecution, res)

	var nilStruct *HistoryRequestCancelWorkflowExecutionRequest
	res = nilStruct.GetExternalWorkflowExecution()
	assert.Nil(t, res)
}

func TestHistoryRequestCancelWorkflowExecutionRequest_GetChildWorkflowOnly(t *testing.T) {
	testStruct := HistoryRequestCancelWorkflowExecutionRequest{
		ChildWorkflowOnly: true,
	}

	res := testStruct.GetChildWorkflowOnly()
	assert.True(t, res)

	var nilStruct *HistoryRequestCancelWorkflowExecutionRequest
	res = nilStruct.GetChildWorkflowOnly()
	assert.False(t, res)
}

func TestHistoryResetStickyTaskListRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryResetStickyTaskListRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryResetStickyTaskListRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryResetWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryResetWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryResetWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRespondActivityTaskCanceledRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRespondActivityTaskCanceledRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRespondActivityTaskCanceledRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRespondActivityTaskCompletedRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRespondActivityTaskCompletedRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRespondActivityTaskCompletedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRespondActivityTaskFailedRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRespondActivityTaskFailedRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRespondActivityTaskFailedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRespondDecisionTaskCompletedRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRespondDecisionTaskCompletedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryRespondDecisionTaskCompletedRequest_GetCompleteRequest(t *testing.T) {
	completeRequest := &RespondDecisionTaskCompletedRequest{}
	testStruct := HistoryRespondDecisionTaskCompletedRequest{
		CompleteRequest: completeRequest,
	}

	res := testStruct.GetCompleteRequest()
	assert.Equal(t, completeRequest, res)

	var nilStruct *HistoryRespondDecisionTaskCompletedRequest
	res = nilStruct.GetCompleteRequest()
	assert.Nil(t, res)
}

func TestHistoryRespondDecisionTaskFailedRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryRespondDecisionTaskFailedRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryRespondDecisionTaskFailedRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestScheduleDecisionTaskRequest_GetDomainUUID(t *testing.T) {
	testStruct := ScheduleDecisionTaskRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *ScheduleDecisionTaskRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestShardOwnershipLostError_GetOwner(t *testing.T) {
	owner := "test-owner"
	testStruct := ShardOwnershipLostError{
		Owner: owner,
	}

	res := testStruct.GetOwner()
	assert.Equal(t, owner, res)

	var nilStruct *ShardOwnershipLostError
	res = nilStruct.GetOwner()
	assert.Equal(t, "", res)
}

func TestHistorySignalWithStartWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistorySignalWithStartWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistorySignalWithStartWorkflowExecutionRequest_GetPartitionConfig(t *testing.T) {
	partitionConfig := map[string]string{"partition1": "value1"}
	testStruct := HistorySignalWithStartWorkflowExecutionRequest{
		PartitionConfig: partitionConfig,
	}

	res := testStruct.GetPartitionConfig()
	assert.Equal(t, partitionConfig, res)

	var nilStruct *HistorySignalWithStartWorkflowExecutionRequest
	res = nilStruct.GetPartitionConfig()
	assert.Nil(t, res)
}

func TestHistorySignalWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistorySignalWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistorySignalWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistorySignalWorkflowExecutionRequest_GetChildWorkflowOnly(t *testing.T) {
	testStruct := HistorySignalWorkflowExecutionRequest{
		ChildWorkflowOnly: true,
	}

	res := testStruct.GetChildWorkflowOnly()
	assert.True(t, res)

	var nilStruct *HistorySignalWorkflowExecutionRequest
	res = nilStruct.GetChildWorkflowOnly()
	assert.False(t, res)
}

func TestHistoryStartWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryStartWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryStartWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryStartWorkflowExecutionRequest_GetAttempt(t *testing.T) {
	testStruct := HistoryStartWorkflowExecutionRequest{
		Attempt: attempt32,
	}

	res := testStruct.GetAttempt()
	assert.Equal(t, attempt32, res)

	var nilStruct *HistoryStartWorkflowExecutionRequest
	res = nilStruct.GetAttempt()
	assert.Equal(t, int32(0), res)
}

func TestHistoryStartWorkflowExecutionRequest_GetExpirationTimestamp(t *testing.T) {
	testStruct := HistoryStartWorkflowExecutionRequest{
		ExpirationTimestamp: &expirationTimestamp,
	}

	res := testStruct.GetExpirationTimestamp()
	assert.Equal(t, expirationTimestamp, res)

	var nilStruct *HistoryStartWorkflowExecutionRequest
	res = nilStruct.GetExpirationTimestamp()
	assert.Equal(t, int64(0), res)
}

func TestHistoryStartWorkflowExecutionRequest_GetFirstDecisionTaskBackoffSeconds(t *testing.T) {
	testStruct := HistoryStartWorkflowExecutionRequest{
		FirstDecisionTaskBackoffSeconds: &firstDecisionBackoff,
	}

	res := testStruct.GetFirstDecisionTaskBackoffSeconds()
	assert.Equal(t, firstDecisionBackoff, res)

	var nilStruct *HistoryStartWorkflowExecutionRequest
	res = nilStruct.GetFirstDecisionTaskBackoffSeconds()
	assert.Equal(t, int32(0), res)
}

func TestHistoryStartWorkflowExecutionRequest_GetPartitionConfig(t *testing.T) {
	testStruct := HistoryStartWorkflowExecutionRequest{
		PartitionConfig: partitionConfig,
	}

	res := testStruct.GetPartitionConfig()
	assert.Equal(t, partitionConfig, res)

	var nilStruct *HistoryStartWorkflowExecutionRequest
	res = nilStruct.GetPartitionConfig()
	assert.Nil(t, res)
}

func TestSyncActivityRequest_GetDomainID(t *testing.T) {
	testStruct := SyncActivityRequest{
		DomainID: domainUUID,
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestSyncActivityRequest_GetScheduledTime(t *testing.T) {
	testStruct := SyncActivityRequest{
		ScheduledTime: &scheduledTime,
	}

	res := testStruct.GetScheduledTime()
	assert.Equal(t, scheduledTime, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetScheduledTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetStartedTime(t *testing.T) {
	testStruct := SyncActivityRequest{
		StartedTime: &startedTime,
	}

	res := testStruct.GetStartedTime()
	assert.Equal(t, startedTime, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetStartedTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetLastHeartbeatTime(t *testing.T) {
	testStruct := SyncActivityRequest{
		LastHeartbeatTime: &lastHeartbeatTime,
	}

	res := testStruct.GetLastHeartbeatTime()
	assert.Equal(t, lastHeartbeatTime, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetLastHeartbeatTime()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetDetails(t *testing.T) {
	testStruct := SyncActivityRequest{
		Details: details,
	}

	res := testStruct.GetDetails()
	assert.Equal(t, details, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetDetails()
	assert.Nil(t, res)
}

func TestSyncActivityRequest_GetLastFailureReason(t *testing.T) {
	testStruct := SyncActivityRequest{
		LastFailureReason: &lastFailureReason,
	}

	res := testStruct.GetLastFailureReason()
	assert.Equal(t, lastFailureReason, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetLastFailureReason()
	assert.Equal(t, "", res)
}

func TestSyncActivityRequest_GetLastWorkerIdentity(t *testing.T) {
	testStruct := SyncActivityRequest{
		LastWorkerIdentity: lastWorkerIdentity,
	}

	res := testStruct.GetLastWorkerIdentity()
	assert.Equal(t, lastWorkerIdentity, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetLastWorkerIdentity()
	assert.Equal(t, "", res)
}

func TestSyncActivityRequest_GetLastFailureDetails(t *testing.T) {
	testStruct := SyncActivityRequest{
		LastFailureDetails: lastFailureDetails,
	}

	res := testStruct.GetLastFailureDetails()
	assert.Equal(t, lastFailureDetails, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetLastFailureDetails()
	assert.Nil(t, res)
}

func TestSyncActivityRequest_GetAttempt(t *testing.T) {
	testStruct := SyncActivityRequest{
		Attempt: attempt32,
	}

	res := testStruct.GetAttempt()
	assert.Equal(t, attempt32, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetAttempt()
	assert.Equal(t, int32(0), res)
}

func TestSyncActivityRequest_GetWorkflowID(t *testing.T) {
	testStruct := SyncActivityRequest{
		WorkflowID: workflowID,
	}

	res := testStruct.GetWorkflowID()
	assert.Equal(t, workflowID, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetWorkflowID()
	assert.Equal(t, "", res)
}

func TestSyncActivityRequest_GetRunID(t *testing.T) {
	testStruct := SyncActivityRequest{
		RunID: runID,
	}

	res := testStruct.GetRunID()
	assert.Equal(t, runID, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetRunID()
	assert.Equal(t, "", res)
}

func TestSyncActivityRequest_GetVersion(t *testing.T) {
	testStruct := SyncActivityRequest{
		Version: version,
	}

	res := testStruct.GetVersion()
	assert.Equal(t, version, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetVersion()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetScheduledID(t *testing.T) {
	testStruct := SyncActivityRequest{
		ScheduledID: scheduledID,
	}

	res := testStruct.GetScheduledID()
	assert.Equal(t, scheduledID, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetScheduledID()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetStartedID(t *testing.T) {
	testStruct := SyncActivityRequest{
		StartedID: startedID,
	}

	res := testStruct.GetStartedID()
	assert.Equal(t, startedID, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetStartedID()
	assert.Equal(t, int64(0), res)
}

func TestSyncActivityRequest_GetVersionHistory(t *testing.T) {
	testStruct := SyncActivityRequest{
		VersionHistory: versionHistory,
	}

	res := testStruct.GetVersionHistory()
	assert.Equal(t, versionHistory, res)

	var nilStruct *SyncActivityRequest
	res = nilStruct.GetVersionHistory()
	assert.Nil(t, res)
}

func TestSyncShardStatusRequest_GetSourceCluster(t *testing.T) {
	testStruct := SyncShardStatusRequest{
		SourceCluster: sourceCluster,
	}

	res := testStruct.GetSourceCluster()
	assert.Equal(t, sourceCluster, res)

	var nilStruct *SyncShardStatusRequest
	res = nilStruct.GetSourceCluster()
	assert.Equal(t, "", res)
}

func TestSyncShardStatusRequest_GetShardID(t *testing.T) {
	testStruct := SyncShardStatusRequest{
		ShardID: shardID,
	}

	res := testStruct.GetShardID()
	assert.Equal(t, shardID, res)

	var nilStruct *SyncShardStatusRequest
	res = nilStruct.GetShardID()
	assert.Equal(t, int64(0), res)
}

func TestSyncShardStatusRequest_GetTimestamp(t *testing.T) {
	testStruct := SyncShardStatusRequest{
		Timestamp: &startedTimestamp,
	}

	res := testStruct.GetTimestamp()
	assert.Equal(t, startedTimestamp, res)

	var nilStruct *SyncShardStatusRequest
	res = nilStruct.GetTimestamp()
	assert.Equal(t, int64(0), res)
}

func TestHistoryTerminateWorkflowExecutionRequest_GetDomainUUID(t *testing.T) {
	testStruct := HistoryTerminateWorkflowExecutionRequest{
		DomainUUID: domainUUID,
	}

	res := testStruct.GetDomainUUID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *HistoryTerminateWorkflowExecutionRequest
	res = nilStruct.GetDomainUUID()
	assert.Equal(t, "", res)
}

func TestHistoryTerminateWorkflowExecutionRequest_GetTerminateRequest(t *testing.T) {
	testStruct := HistoryTerminateWorkflowExecutionRequest{
		TerminateRequest: terminateRequest,
	}

	res := testStruct.GetTerminateRequest()
	assert.Equal(t, terminateRequest, res)

	var nilStruct *HistoryTerminateWorkflowExecutionRequest
	res = nilStruct.GetTerminateRequest()
	assert.Nil(t, res)
}

func TestHistoryTerminateWorkflowExecutionRequest_GetExternalWorkflowExecution(t *testing.T) {
	testStruct := HistoryTerminateWorkflowExecutionRequest{
		ExternalWorkflowExecution: externalWorkflowExecution,
	}

	res := testStruct.GetExternalWorkflowExecution()
	assert.Equal(t, externalWorkflowExecution, res)

	var nilStruct *HistoryTerminateWorkflowExecutionRequest
	res = nilStruct.GetExternalWorkflowExecution()
	assert.Nil(t, res)
}

func TestHistoryTerminateWorkflowExecutionRequest_GetChildWorkflowOnly(t *testing.T) {
	testStruct := HistoryTerminateWorkflowExecutionRequest{
		ChildWorkflowOnly: childWorkflowOnly,
	}

	res := testStruct.GetChildWorkflowOnly()
	assert.Equal(t, childWorkflowOnly, res)

	var nilStruct *HistoryTerminateWorkflowExecutionRequest
	res = nilStruct.GetChildWorkflowOnly()
	assert.False(t, res)
}

func TestGetFailoverInfoRequest_GetDomainID(t *testing.T) {
	testStruct := GetFailoverInfoRequest{
		DomainID: domainUUID,
	}

	res := testStruct.GetDomainID()
	assert.Equal(t, domainUUID, res)

	var nilStruct *GetFailoverInfoRequest
	res = nilStruct.GetDomainID()
	assert.Equal(t, "", res)
}

func TestGetFailoverInfoResponse_GetCompletedShardCount(t *testing.T) {
	testStruct := GetFailoverInfoResponse{
		CompletedShardCount: completedShardCount,
	}

	res := testStruct.GetCompletedShardCount()
	assert.Equal(t, completedShardCount, res)

	var nilStruct *GetFailoverInfoResponse
	res = nilStruct.GetCompletedShardCount()
	assert.Equal(t, int32(0), res)
}

func TestGetFailoverInfoResponse_GetPendingShards(t *testing.T) {
	testStruct := GetFailoverInfoResponse{
		PendingShards: pendingShards,
	}

	res := testStruct.GetPendingShards()
	assert.Equal(t, pendingShards, res)

	var nilStruct *GetFailoverInfoResponse
	res = nilStruct.GetPendingShards()
	assert.Nil(t, res)
}
