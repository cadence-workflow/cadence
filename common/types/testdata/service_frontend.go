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
	RegisterDomainRequest = types.RegisterDomainRequest{
		Name:                                   DomainName,
		Description:                            DomainDescription,
		OwnerEmail:                             DomainOwnerEmail,
		WorkflowExecutionRetentionPeriodInDays: DomainRetention,
		EmitMetric:                             common.BoolPtr(DomainEmitMetric),
		Clusters:                               ClusterReplicationConfigurationArray,
		ActiveClusterName:                      ClusterName1,
		ActiveClusters:                         &ActiveClusters,
		Data:                                   DomainData,
		SecurityToken:                          SecurityToken,
		IsGlobalDomain:                         true,
		HistoryArchivalStatus:                  &ArchivalStatus,
		HistoryArchivalURI:                     HistoryArchivalURI,
		VisibilityArchivalStatus:               &ArchivalStatus,
		VisibilityArchivalURI:                  VisibilityArchivalURI,
	}
	DescribeDomainRequest_ID = types.DescribeDomainRequest{
		UUID: common.StringPtr(DomainID),
	}
	DescribeDomainRequest_Name = types.DescribeDomainRequest{
		Name: common.StringPtr(DomainName),
	}
	DescribeDomainResponse = types.DescribeDomainResponse{
		DomainInfo:               &DomainInfo,
		Configuration:            &DomainConfiguration,
		ReplicationConfiguration: &DomainReplicationConfiguration,
		FailoverVersion:          FailoverVersion1,
		IsGlobalDomain:           true,
	}
	ListDomainsRequest = types.ListDomainsRequest{
		PageSize:      PageSize,
		NextPageToken: NextPageToken,
	}
	ListDomainsResponse = types.ListDomainsResponse{
		Domains:       []*types.DescribeDomainResponse{&DescribeDomainResponse},
		NextPageToken: NextPageToken,
	}
	UpdateDomainRequest = types.UpdateDomainRequest{
		Name:                                   DomainName,
		Description:                            common.StringPtr(DomainDescription),
		OwnerEmail:                             common.StringPtr(DomainOwnerEmail),
		Data:                                   DomainData,
		WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(DomainRetention),
		BadBinaries:                            &BadBinaries,
		HistoryArchivalStatus:                  &ArchivalStatus,
		HistoryArchivalURI:                     common.StringPtr(HistoryArchivalURI),
		VisibilityArchivalStatus:               &ArchivalStatus,
		VisibilityArchivalURI:                  common.StringPtr(VisibilityArchivalURI),
		ActiveClusterName:                      common.StringPtr(ClusterName1),
		ActiveClusters:                         &ActiveClusters,
		Clusters:                               ClusterReplicationConfigurationArray,
		SecurityToken:                          SecurityToken,
		DeleteBadBinary:                        common.StringPtr(DeleteBadBinary),
		FailoverTimeoutInSeconds:               &Duration1,
	}
	FailoverDomainRequest = types.FailoverDomainRequest{
		DomainName:              DomainName,
		DomainActiveClusterName: common.StringPtr(ClusterName1),
		ActiveClusters:          &ActiveClusters,
	}
	FailoverDomainRequest_OnlyActiveClusters = types.FailoverDomainRequest{
		DomainName: DomainName,
		// Explicitly set to nil to test ActiveActive failovers
		DomainActiveClusterName: nil,
		ActiveClusters:          &ActiveClusters,
	}
	ActiveClusters = types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{
			"region": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					Region1: {
						ActiveClusterName: ClusterName1,
						FailoverVersion:   FailoverVersion1,
					},
					Region2: {
						ActiveClusterName: ClusterName2,
						FailoverVersion:   FailoverVersion2,
					},
				},
			},
		},
	}
	UpdateDomainResponse = types.UpdateDomainResponse{
		DomainInfo:               &DomainInfo,
		Configuration:            &DomainConfiguration,
		ReplicationConfiguration: &DomainReplicationConfiguration,
		FailoverVersion:          FailoverVersion1,
		IsGlobalDomain:           true,
	}
	DeleteDomainRequest = types.DeleteDomainRequest{
		Name:          DomainName,
		SecurityToken: SecurityToken,
	}
	DeprecateDomainRequest = types.DeprecateDomainRequest{
		Name:          DomainName,
		SecurityToken: SecurityToken,
	}
	ListWorkflowExecutionsRequest = types.ListWorkflowExecutionsRequest{
		Domain:        DomainName,
		PageSize:      PageSize,
		NextPageToken: NextPageToken,
		Query:         VisibilityQuery,
	}
	ListWorkflowExecutionsResponse = types.ListWorkflowExecutionsResponse{
		Executions:    WorkflowExecutionInfoArray,
		NextPageToken: NextPageToken,
	}
	ListOpenWorkflowExecutionsRequest_ExecutionFilter = types.ListOpenWorkflowExecutionsRequest{
		Domain:          DomainName,
		MaximumPageSize: PageSize,
		NextPageToken:   NextPageToken,
		StartTimeFilter: &StartTimeFilter,
		ExecutionFilter: &WorkflowExecutionFilter,
	}
	ListOpenWorkflowExecutionsRequest_TypeFilter = types.ListOpenWorkflowExecutionsRequest{
		Domain:          DomainName,
		MaximumPageSize: PageSize,
		NextPageToken:   NextPageToken,
		StartTimeFilter: &StartTimeFilter,
		TypeFilter:      &WorkflowTypeFilter,
	}
	ListOpenWorkflowExecutionsResponse = types.ListOpenWorkflowExecutionsResponse{
		Executions:    WorkflowExecutionInfoArray,
		NextPageToken: NextPageToken,
	}
	ListClosedWorkflowExecutionsRequest_ExecutionFilter = types.ListClosedWorkflowExecutionsRequest{
		Domain:          DomainName,
		MaximumPageSize: PageSize,
		NextPageToken:   NextPageToken,
		StartTimeFilter: &StartTimeFilter,
		ExecutionFilter: &WorkflowExecutionFilter,
	}
	ListClosedWorkflowExecutionsRequest_TypeFilter = types.ListClosedWorkflowExecutionsRequest{
		Domain:          DomainName,
		MaximumPageSize: PageSize,
		NextPageToken:   NextPageToken,
		StartTimeFilter: &StartTimeFilter,
		TypeFilter:      &WorkflowTypeFilter,
	}
	ListClosedWorkflowExecutionsRequest_StatusFilter = types.ListClosedWorkflowExecutionsRequest{
		Domain:          DomainName,
		MaximumPageSize: PageSize,
		NextPageToken:   NextPageToken,
		StartTimeFilter: &StartTimeFilter,
		StatusFilter:    &WorkflowExecutionCloseStatus,
	}
	ListClosedWorkflowExecutionsResponse = types.ListClosedWorkflowExecutionsResponse{
		Executions:    WorkflowExecutionInfoArray,
		NextPageToken: NextPageToken,
	}
	ListArchivedWorkflowExecutionsRequest = types.ListArchivedWorkflowExecutionsRequest{
		Domain:        DomainName,
		PageSize:      PageSize,
		NextPageToken: NextPageToken,
		Query:         VisibilityQuery,
	}
	ListArchivedWorkflowExecutionsResponse = types.ListArchivedWorkflowExecutionsResponse{
		Executions:    WorkflowExecutionInfoArray,
		NextPageToken: NextPageToken,
	}
	CountWorkflowExecutionsRequest = types.CountWorkflowExecutionsRequest{
		Domain: DomainName,
		Query:  VisibilityQuery,
	}
	CountWorkflowExecutionsResponse = types.CountWorkflowExecutionsResponse{
		Count: int64(8),
	}
	GetSearchAttributesResponse = types.GetSearchAttributesResponse{
		Keys: IndexedValueTypeMap,
	}
	PollForDecisionTaskRequest = types.PollForDecisionTaskRequest{
		Domain:         DomainName,
		TaskList:       &TaskList,
		Identity:       Identity,
		BinaryChecksum: Checksum,
	}
	PollForDecisionTaskResponse = types.PollForDecisionTaskResponse{
		TaskToken:                 TaskToken,
		WorkflowExecution:         &WorkflowExecution,
		WorkflowType:              &WorkflowType,
		PreviousStartedEventID:    common.Int64Ptr(EventID1),
		StartedEventID:            EventID2,
		Attempt:                   Attempt,
		BacklogCountHint:          BacklogCountHint,
		History:                   &History,
		NextPageToken:             NextPageToken,
		Query:                     &WorkflowQuery,
		WorkflowExecutionTaskList: &TaskList,
		ScheduledTimestamp:        &Timestamp1,
		StartedTimestamp:          &Timestamp2,
		Queries:                   WorkflowQueryMap,
		NextEventID:               EventID3,
		AutoConfigHint:            &AutoConfigHint,
	}
	RespondDecisionTaskCompletedRequest = types.RespondDecisionTaskCompletedRequest{
		TaskToken:                  TaskToken,
		Decisions:                  DecisionArray,
		ExecutionContext:           ExecutionContext,
		Identity:                   Identity,
		StickyAttributes:           &StickyExecutionAttributes,
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: true,
		BinaryChecksum:             Checksum,
		QueryResults:               WorkflowQueryResultMap,
	}
	RespondDecisionTaskCompletedResponse = types.RespondDecisionTaskCompletedResponse{
		DecisionTask:                &PollForDecisionTaskResponse,
		ActivitiesToDispatchLocally: ActivityLocalDispatchInfoMap,
	}
	RespondDecisionTaskFailedRequest = types.RespondDecisionTaskFailedRequest{
		TaskToken:      TaskToken,
		Cause:          &DecisionTaskFailedCause,
		Details:        Payload1,
		Identity:       Identity,
		BinaryChecksum: Checksum,
	}
	PollForActivityTaskRequest = types.PollForActivityTaskRequest{
		Domain:           DomainName,
		TaskList:         &TaskList,
		Identity:         Identity,
		TaskListMetadata: &TaskListMetadata,
	}
	PollForActivityTaskResponse = types.PollForActivityTaskResponse{
		TaskToken:                       TaskToken,
		WorkflowExecution:               &WorkflowExecution,
		ActivityID:                      ActivityID,
		ActivityType:                    &ActivityType,
		Input:                           Payload1,
		ScheduledTimestamp:              &Timestamp1,
		ScheduleToCloseTimeoutSeconds:   &Duration1,
		StartedTimestamp:                &Timestamp2,
		StartToCloseTimeoutSeconds:      &Duration2,
		HeartbeatTimeoutSeconds:         &Duration3,
		Attempt:                         Attempt,
		ScheduledTimestampOfThisAttempt: &Timestamp3,
		HeartbeatDetails:                Payload2,
		WorkflowType:                    &WorkflowType,
		WorkflowDomain:                  DomainName,
		Header:                          &Header,
		AutoConfigHint:                  &AutoConfigHint,
	}
	RespondActivityTaskCompletedRequest = types.RespondActivityTaskCompletedRequest{
		TaskToken: TaskToken,
		Result:    Payload1,
		Identity:  Identity,
	}
	RespondActivityTaskCompletedByIDRequest = types.RespondActivityTaskCompletedByIDRequest{
		Domain:     DomainName,
		WorkflowID: WorkflowID,
		RunID:      RunID,
		ActivityID: ActivityID,
		Result:     Payload1,
		Identity:   Identity,
	}
	RespondActivityTaskFailedRequest = types.RespondActivityTaskFailedRequest{
		TaskToken: TaskToken,
		Reason:    &FailureReason,
		Details:   FailureDetails,
		Identity:  Identity,
	}
	RespondActivityTaskFailedByIDRequest = types.RespondActivityTaskFailedByIDRequest{
		Domain:     DomainName,
		WorkflowID: WorkflowID,
		RunID:      RunID,
		ActivityID: ActivityID,
		Reason:     &FailureReason,
		Details:    FailureDetails,
		Identity:   Identity,
	}
	RespondActivityTaskCanceledRequest = types.RespondActivityTaskCanceledRequest{
		TaskToken: TaskToken,
		Details:   Payload1,
		Identity:  Identity,
	}
	RespondActivityTaskCanceledByIDRequest = types.RespondActivityTaskCanceledByIDRequest{
		Domain:     DomainName,
		WorkflowID: WorkflowID,
		RunID:      RunID,
		ActivityID: ActivityID,
		Details:    Payload1,
		Identity:   Identity,
	}
	RecordActivityTaskHeartbeatRequest = types.RecordActivityTaskHeartbeatRequest{
		TaskToken: TaskToken,
		Details:   Payload1,
		Identity:  Identity,
	}
	RecordActivityTaskHeartbeatResponse = types.RecordActivityTaskHeartbeatResponse{
		CancelRequested: true,
	}
	RecordActivityTaskHeartbeatByIDRequest = types.RecordActivityTaskHeartbeatByIDRequest{
		Domain:     DomainName,
		WorkflowID: WorkflowID,
		RunID:      RunID,
		ActivityID: ActivityID,
		Details:    Payload1,
		Identity:   Identity,
	}
	RespondQueryTaskCompletedRequest = types.RespondQueryTaskCompletedRequest{
		TaskToken:         TaskToken,
		CompletedType:     &QueryTaskCompletedType,
		QueryResult:       Payload1,
		ErrorMessage:      ErrorMessage,
		WorkerVersionInfo: &WorkerVersionInfo,
	}
	RequestCancelWorkflowExecutionRequest = types.RequestCancelWorkflowExecutionRequest{
		Domain:              DomainName,
		WorkflowExecution:   &WorkflowExecution,
		Identity:            Identity,
		RequestID:           RequestID,
		FirstExecutionRunID: RunID,
	}
	StartWorkflowExecutionRequest = types.StartWorkflowExecutionRequest{
		Domain:                              DomainName,
		WorkflowID:                          WorkflowID,
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		Identity:                            Identity,
		RequestID:                           RequestID,
		WorkflowIDReusePolicy:               &WorkflowIDReusePolicy,
		RetryPolicy:                         &RetryPolicy,
		CronSchedule:                        CronSchedule,
		CronOverlapPolicy:                   &CronOverlapPolicy,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		Header:                              &Header,
		FirstRunAtTimeStamp:                 &Timestamp1,
		ActiveClusterSelectionPolicy:        &ActiveClusterSelectionPolicyExternalEntity,
	}
	StartWorkflowExecutionResponse = types.StartWorkflowExecutionResponse{
		RunID: RunID,
	}
	StartWorkflowExecutionAsyncRequest = types.StartWorkflowExecutionAsyncRequest{
		StartWorkflowExecutionRequest: &StartWorkflowExecutionRequest,
	}
	StartWorkflowExecutionAsyncResponse = types.StartWorkflowExecutionAsyncResponse{}
	SignalWorkflowExecutionRequest      = types.SignalWorkflowExecutionRequest{
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		SignalName:        SignalName,
		Input:             Payload1,
		Identity:          Identity,
		RequestID:         RequestID,
		Control:           Control,
	}
	SignalWithStartWorkflowExecutionRequest = types.SignalWithStartWorkflowExecutionRequest{
		Domain:                              DomainName,
		WorkflowID:                          WorkflowID,
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		Identity:                            Identity,
		RequestID:                           RequestID,
		WorkflowIDReusePolicy:               &WorkflowIDReusePolicy,
		SignalName:                          SignalName,
		SignalInput:                         Payload2,
		Control:                             Control,
		RetryPolicy:                         &RetryPolicy,
		CronSchedule:                        CronSchedule,
		CronOverlapPolicy:                   &CronOverlapPolicy,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		Header:                              &Header,
		FirstRunAtTimestamp:                 &Timestamp1,
		ActiveClusterSelectionPolicy:        &ActiveClusterSelectionPolicyRegionSticky,
	}
	SignalWithStartWorkflowExecutionAsyncRequest = types.SignalWithStartWorkflowExecutionAsyncRequest{
		SignalWithStartWorkflowExecutionRequest: &SignalWithStartWorkflowExecutionRequest,
	}
	SignalWithStartWorkflowExecutionAsyncResponse = types.SignalWithStartWorkflowExecutionAsyncResponse{}
	ResetWorkflowExecutionRequest                 = types.ResetWorkflowExecutionRequest{
		Domain:                DomainName,
		WorkflowExecution:     &WorkflowExecution,
		Reason:                Reason,
		DecisionFinishEventID: EventID1,
		RequestID:             RequestID,
		SkipSignalReapply:     true,
	}
	ResetWorkflowExecutionResponse = types.ResetWorkflowExecutionResponse{
		RunID: RunID,
	}
	TerminateWorkflowExecutionRequest = types.TerminateWorkflowExecutionRequest{
		Domain:              DomainName,
		WorkflowExecution:   &WorkflowExecution,
		Reason:              Reason,
		Details:             Payload1,
		Identity:            Identity,
		FirstExecutionRunID: RunID,
	}
	DescribeWorkflowExecutionRequest = types.DescribeWorkflowExecutionRequest{
		Domain:                DomainName,
		Execution:             &WorkflowExecution,
		QueryConsistencyLevel: &QueryConsistencyLevel,
	}
	DescribeWorkflowExecutionResponse = types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: &WorkflowExecutionConfiguration,
		WorkflowExecutionInfo:  &WorkflowExecutionInfo,
		PendingActivities:      PendingActivityInfoArray,
		PendingChildren:        PendingChildExecutionInfoArray,
		PendingDecision:        &PendingDecisionInfo,
	}
	DiagnoseWorkflowExecutionRequest = types.DiagnoseWorkflowExecutionRequest{
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		Identity:          Identity,
	}
	DiagnoseWorkflowExecutionResponse = types.DiagnoseWorkflowExecutionResponse{
		Domain:                      DomainName,
		DiagnosticWorkflowExecution: &WorkflowExecution,
	}
	QueryWorkflowRequest = types.QueryWorkflowRequest{
		Domain:                DomainName,
		Execution:             &WorkflowExecution,
		Query:                 &WorkflowQuery,
		QueryRejectCondition:  &QueryRejectCondition,
		QueryConsistencyLevel: &QueryConsistencyLevel,
	}
	QueryWorkflowResponse = types.QueryWorkflowResponse{
		QueryResult:   Payload1,
		QueryRejected: &QueryRejected,
	}
	DescribeTaskListRequest = types.DescribeTaskListRequest{
		Domain:                DomainName,
		TaskList:              &TaskList,
		TaskListType:          &TaskListType,
		IncludeTaskListStatus: true,
	}
	DescribeTaskListResponse = types.DescribeTaskListResponse{
		Pollers:         PollerInfoArray,
		TaskListStatus:  &TaskListStatus,
		PartitionConfig: &TaskListPartitionConfig,
		TaskList:        &TaskList,
	}
	ListTaskListPartitionsRequest = types.ListTaskListPartitionsRequest{
		Domain:   DomainName,
		TaskList: &TaskList,
	}
	ListTaskListPartitionsResponse = types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: TaskListPartitionMetadataArray,
		DecisionTaskListPartitions: TaskListPartitionMetadataArray,
	}
	ResetStickyTaskListRequest = types.ResetStickyTaskListRequest{
		Domain:    DomainName,
		Execution: &WorkflowExecution,
	}
	ResetStickyTaskListResponse        = types.ResetStickyTaskListResponse{}
	GetWorkflowExecutionHistoryRequest = types.GetWorkflowExecutionHistoryRequest{
		Domain:                 DomainName,
		Execution:              &WorkflowExecution,
		MaximumPageSize:        PageSize,
		NextPageToken:          NextPageToken,
		WaitForNewEvent:        true,
		HistoryEventFilterType: &HistoryEventFilterType,
		SkipArchival:           true,
		QueryConsistencyLevel:  &QueryConsistencyLevel,
	}
	GetWorkflowExecutionHistoryResponse = types.GetWorkflowExecutionHistoryResponse{
		History:       &History,
		RawHistory:    DataBlobArray,
		NextPageToken: NextPageToken,
		Archived:      true,
	}

	DescribeDomainResponseArray = []*types.DescribeDomainResponse{
		&DescribeDomainResponse,
	}
)
