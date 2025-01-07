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

package testdata

import (
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

var (
	HistoryCloseShardRequest           = AdminCloseShardRequest
	HistoryDescribeHistoryHostRequest  = types.DescribeHistoryHostRequest{}
	HistoryDescribeHistoryHostResponse = AdminDescribeHistoryHostResponse
	HistoryDescribeMutableStateRequest = types.DescribeMutableStateRequest{
		DomainUUID: DomainID,
		Execution:  &WorkflowExecution,
	}
	HistoryDescribeMutableStateResponse = types.DescribeMutableStateResponse{
		MutableStateInCache:    "MutableStateInCache",
		MutableStateInDatabase: "MutableStateInDatabase",
	}
	HistoryDescribeQueueRequest             = AdminDescribeQueueRequest
	HistoryDescribeQueueResponse            = AdminDescribeQueueResponse
	HistoryDescribeWorkflowExecutionRequest = types.HistoryDescribeWorkflowExecutionRequest{
		DomainUUID: DomainID,
		Request:    &DescribeWorkflowExecutionRequest,
	}
	HistoryDescribeWorkflowExecutionResponse = DescribeWorkflowExecutionResponse
	HistoryGetDLQReplicationMessagesRequest  = AdminGetDLQReplicationMessagesRequest
	HistoryGetDLQReplicationMessagesResponse = AdminGetDLQReplicationMessagesResponse
	HistoryGetMutableStateRequest            = types.GetMutableStateRequest{
		DomainUUID:          DomainID,
		Execution:           &WorkflowExecution,
		ExpectedNextEventID: EventID1,
		CurrentBranchToken:  BranchToken,
		VersionHistoryItem:  &VersionHistoryItem,
	}
	HistoryGetMutableStateResponse = types.GetMutableStateResponse{
		Execution:                            &WorkflowExecution,
		WorkflowType:                         &WorkflowType,
		NextEventID:                          EventID1,
		PreviousStartedEventID:               common.Int64Ptr(EventID2),
		LastFirstEventID:                     EventID3,
		TaskList:                             &TaskList,
		StickyTaskList:                       &TaskList,
		ClientLibraryVersion:                 ClientLibraryVersion,
		ClientFeatureVersion:                 FeatureVersion,
		ClientImpl:                           ClientImpl,
		IsWorkflowRunning:                    true,
		StickyTaskListScheduleToStartTimeout: &Duration1,
		EventStoreVersion:                    EventStoreVersion,
		CurrentBranchToken:                   BranchToken,
		WorkflowState:                        common.Int32Ptr(1), // persistence.WorkflowStateRunning
		WorkflowCloseState:                   common.Int32Ptr(6), // persistence.WorkflowCloseStatusTimedOut
		VersionHistories:                     &VersionHistories,
		IsStickyTaskListEnabled:              true,
		HistorySize:                          HistorySizeInBytes,
	}
	HistoryGetReplicationMessagesRequest  = AdminGetReplicationMessagesRequest
	HistoryGetReplicationMessagesResponse = AdminGetReplicationMessagesResponse
	HistoryCountDLQMessagesRequest        = types.CountDLQMessagesRequest{ForceFetch: true}
	HistoryCountDLQMessagesResponse       = types.HistoryCountDLQMessagesResponse{Entries: map[types.HistoryDLQCountKey]int64{types.HistoryDLQCountKey{1, "A"}: 10}}
	HistoryMergeDLQMessagesRequest        = AdminMergeDLQMessagesRequest
	HistoryMergeDLQMessagesResponse       = AdminMergeDLQMessagesResponse
	HistoryNotifyFailoverMarkersRequest   = types.NotifyFailoverMarkersRequest{
		FailoverMarkerTokens: FailoverMarkerTokenArray,
	}
	HistoryPollMutableStateRequest = types.PollMutableStateRequest{
		DomainUUID:          DomainID,
		Execution:           &WorkflowExecution,
		ExpectedNextEventID: EventID1,
		CurrentBranchToken:  BranchToken,
	}
	HistoryPollMutableStateResponse = types.PollMutableStateResponse{
		Execution:                            &WorkflowExecution,
		WorkflowType:                         &WorkflowType,
		NextEventID:                          EventID1,
		PreviousStartedEventID:               common.Int64Ptr(EventID2),
		LastFirstEventID:                     EventID3,
		TaskList:                             &TaskList,
		StickyTaskList:                       &TaskList,
		ClientLibraryVersion:                 ClientLibraryVersion,
		ClientFeatureVersion:                 FeatureVersion,
		ClientImpl:                           ClientImpl,
		StickyTaskListScheduleToStartTimeout: &Duration1,
		CurrentBranchToken:                   BranchToken,
		VersionHistories:                     &VersionHistories,
		WorkflowState:                        common.Int32Ptr(5), // persistence.WorkflowStateCorrupted
		WorkflowCloseState:                   common.Int32Ptr(6), // persistence.WorkflowCloseStatusTimedOut
	}
	HistoryPurgeDLQMessagesRequest = AdminPurgeDLQMessagesRequest
	HistoryQueryWorkflowRequest    = types.HistoryQueryWorkflowRequest{
		DomainUUID: DomainID,
		Request:    &QueryWorkflowRequest,
	}
	HistoryQueryWorkflowResponse = types.HistoryQueryWorkflowResponse{
		Response: &QueryWorkflowResponse,
	}
	HistoryReadDLQMessagesRequest  = AdminReadDLQMessagesRequest
	HistoryReadDLQMessagesResponse = AdminReadDLQMessagesResponse
	HistoryReapplyEventsRequest    = types.HistoryReapplyEventsRequest{
		DomainUUID: DomainID,
		Request:    &AdminReapplyEventsRequest,
	}
	HistoryRecordActivityTaskHeartbeatRequest = types.HistoryRecordActivityTaskHeartbeatRequest{
		DomainUUID:       DomainID,
		HeartbeatRequest: &RecordActivityTaskHeartbeatRequest,
	}
	HistoryRecordActivityTaskHeartbeatResponse = RecordActivityTaskHeartbeatResponse
	HistoryRecordActivityTaskStartedRequest    = types.RecordActivityTaskStartedRequest{
		DomainUUID:        DomainID,
		WorkflowExecution: &WorkflowExecution,
		ScheduleID:        EventID1,
		TaskID:            TaskID,
		RequestID:         RequestID,
		PollRequest:       &PollForActivityTaskRequest,
	}
	HistoryRecordActivityTaskStartedResponse = types.RecordActivityTaskStartedResponse{
		ScheduledEvent:                  &HistoryEvent_WorkflowExecutionStarted,
		StartedTimestamp:                &Timestamp1,
		Attempt:                         Attempt,
		ScheduledTimestampOfThisAttempt: &Timestamp2,
		HeartbeatDetails:                Payload1,
		WorkflowType:                    &WorkflowType,
		WorkflowDomain:                  DomainName,
	}
	HistoryRecordChildExecutionCompletedRequest = types.RecordChildExecutionCompletedRequest{
		DomainUUID:         DomainID,
		WorkflowExecution:  &WorkflowExecution,
		InitiatedID:        EventID1,
		CompletedExecution: &WorkflowExecution,
		CompletionEvent:    &HistoryEvent_WorkflowExecutionStarted,
		StartedID:          EventID2,
	}
	HistoryRecordDecisionTaskStartedRequest = types.RecordDecisionTaskStartedRequest{
		DomainUUID:        DomainID,
		WorkflowExecution: &WorkflowExecution,
		ScheduleID:        EventID1,
		TaskID:            TaskID,
		RequestID:         RequestID,
		PollRequest:       &PollForDecisionTaskRequest,
	}
	HistoryRecordDecisionTaskStartedResponse = types.RecordDecisionTaskStartedResponse{
		WorkflowType:              &WorkflowType,
		PreviousStartedEventID:    common.Int64Ptr(EventID1),
		ScheduledEventID:          EventID2,
		StartedEventID:            EventID3,
		NextEventID:               EventID4,
		Attempt:                   Attempt,
		StickyExecutionEnabled:    true,
		DecisionInfo:              &TransientDecisionInfo,
		WorkflowExecutionTaskList: &TaskList,
		EventStoreVersion:         EventStoreVersion,
		BranchToken:               BranchToken,
		ScheduledTimestamp:        &Timestamp1,
		StartedTimestamp:          &Timestamp2,
		Queries:                   WorkflowQueryMap,
		HistorySize:               HistorySizeInBytes,
	}
	HistoryRefreshWorkflowTasksRequest = types.HistoryRefreshWorkflowTasksRequest{
		DomainUIID: DomainID,
		Request:    &AdminRefreshWorkflowTasksRequest,
	}
	HistoryRemoveSignalMutableStateRequest = types.RemoveSignalMutableStateRequest{
		DomainUUID:        DomainID,
		WorkflowExecution: &WorkflowExecution,
		RequestID:         RequestID,
	}
	HistoryRemoveTaskRequest        = AdminRemoveTaskRequest
	HistoryReplicateEventsV2Request = types.ReplicateEventsV2Request{
		DomainUUID:          DomainID,
		WorkflowExecution:   &WorkflowExecution,
		VersionHistoryItems: VersionHistoryItemArray,
		Events:              &DataBlob,
		NewRunEvents:        &DataBlob,
	}
	HistoryRequestCancelWorkflowExecutionRequest = types.HistoryRequestCancelWorkflowExecutionRequest{
		DomainUUID:                DomainID,
		CancelRequest:             &RequestCancelWorkflowExecutionRequest,
		ExternalInitiatedEventID:  common.Int64Ptr(EventID1),
		ExternalWorkflowExecution: &WorkflowExecution,
		ChildWorkflowOnly:         true,
	}
	HistoryResetQueueRequest          = AdminResetQueueRequest
	HistoryResetStickyTaskListRequest = types.HistoryResetStickyTaskListRequest{
		DomainUUID: DomainID,
		Execution:  &WorkflowExecution,
	}
	HistoryResetWorkflowExecutionRequest = types.HistoryResetWorkflowExecutionRequest{
		DomainUUID:   DomainID,
		ResetRequest: &ResetWorkflowExecutionRequest,
	}
	HistoryResetWorkflowExecutionResponse = types.ResetWorkflowExecutionResponse{
		RunID: RunID,
	}
	HistoryRespondActivityTaskCanceledRequest = types.HistoryRespondActivityTaskCanceledRequest{
		DomainUUID:    DomainID,
		CancelRequest: &RespondActivityTaskCanceledRequest,
	}
	HistoryRespondActivityTaskCompletedRequest = types.HistoryRespondActivityTaskCompletedRequest{
		DomainUUID:      DomainID,
		CompleteRequest: &RespondActivityTaskCompletedRequest,
	}
	HistoryRespondActivityTaskFailedRequest = types.HistoryRespondActivityTaskFailedRequest{
		DomainUUID:    DomainID,
		FailedRequest: &RespondActivityTaskFailedRequest,
	}
	HistoryRespondDecisionTaskCompletedRequest = types.HistoryRespondDecisionTaskCompletedRequest{
		DomainUUID:      DomainID,
		CompleteRequest: &RespondDecisionTaskCompletedRequest,
	}
	HistoryRespondDecisionTaskCompletedResponse = types.HistoryRespondDecisionTaskCompletedResponse{
		StartedResponse:             &HistoryRecordDecisionTaskStartedResponse,
		ActivitiesToDispatchLocally: ActivityLocalDispatchInfoMap,
	}
	HistoryRespondDecisionTaskFailedRequest = types.HistoryRespondDecisionTaskFailedRequest{
		DomainUUID:    DomainID,
		FailedRequest: &RespondDecisionTaskFailedRequest,
	}
	HistoryScheduleDecisionTaskRequest = types.ScheduleDecisionTaskRequest{
		DomainUUID:        DomainID,
		WorkflowExecution: &WorkflowExecution,
		IsFirstDecision:   true,
	}
	HistorySignalWithStartWorkflowExecutionRequest = types.HistorySignalWithStartWorkflowExecutionRequest{
		DomainUUID:             DomainID,
		SignalWithStartRequest: &SignalWithStartWorkflowExecutionRequest,
		PartitionConfig:        PartitionConfig,
	}
	HistorySignalWithStartWorkflowExecutionResponse = types.StartWorkflowExecutionResponse{
		RunID: RunID,
	}
	HistorySignalWorkflowExecutionRequest = types.HistorySignalWorkflowExecutionRequest{
		DomainUUID:                DomainID,
		SignalRequest:             &SignalWorkflowExecutionRequest,
		ExternalWorkflowExecution: &WorkflowExecution,
		ChildWorkflowOnly:         true,
	}
	HistoryStartWorkflowExecutionRequest = types.HistoryStartWorkflowExecutionRequest{
		DomainUUID:                      DomainID,
		StartRequest:                    &StartWorkflowExecutionRequest,
		ParentExecutionInfo:             &ParentExecutionInfo,
		Attempt:                         Attempt,
		ExpirationTimestamp:             &Timestamp1,
		ContinueAsNewInitiator:          &ContinueAsNewInitiator,
		ContinuedFailureReason:          &FailureReason,
		ContinuedFailureDetails:         FailureDetails,
		LastCompletionResult:            Payload1,
		FirstDecisionTaskBackoffSeconds: &Duration1,
		PartitionConfig:                 PartitionConfig,
	}
	HistoryStartWorkflowExecutionResponse = types.StartWorkflowExecutionResponse{
		RunID: RunID,
	}
	HistorySyncActivityRequest = types.SyncActivityRequest{
		DomainID:           DomainID,
		WorkflowID:         WorkflowID,
		RunID:              RunID,
		Version:            Version1,
		ScheduledID:        EventID1,
		ScheduledTime:      &Timestamp1,
		StartedID:          EventID1,
		StartedTime:        &Timestamp2,
		LastHeartbeatTime:  &Timestamp3,
		Details:            Payload1,
		Attempt:            Attempt,
		LastFailureReason:  &FailureReason,
		LastWorkerIdentity: Identity,
		LastFailureDetails: FailureDetails,
		VersionHistory:     &VersionHistory,
	}
	HistorySyncShardStatusRequest = types.SyncShardStatusRequest{
		SourceCluster: ClusterName1,
		ShardID:       ShardID,
		Timestamp:     &Timestamp1,
	}
	HistoryTerminateWorkflowExecutionRequest = types.HistoryTerminateWorkflowExecutionRequest{
		DomainUUID:                DomainID,
		TerminateRequest:          &TerminateWorkflowExecutionRequest,
		ExternalWorkflowExecution: &WorkflowExecution,
		ChildWorkflowOnly:         true,
	}
	HistoryGetCrossClusterTasksRequest               = GetCrossClusterTasksRequest
	HistoryGetCrossClusterTasksResponse              = GetCrossClusterTasksResponse
	HistoryRespondCrossClusterTasksCompletedRequest  = RespondCrossClusterTasksCompletedRequest
	HistoryRespondCrossClusterTasksCompletedResponse = RespondCrossClusterTasksCompletedResponse
)
