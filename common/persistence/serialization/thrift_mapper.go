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

package serialization

import (
	"time"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

func shardInfoToThrift(info *ShardInfo) *sqlblobs.ShardInfo {
	if info == nil {
		return nil
	}
	result := &sqlblobs.ShardInfo{
		StolenSinceRenew:                          &info.StolenSinceRenew,
		ReplicationAckLevel:                       &info.ReplicationAckLevel,
		TransferAckLevel:                          &info.TransferAckLevel,
		DomainNotificationVersion:                 &info.DomainNotificationVersion,
		ClusterTransferAckLevel:                   info.ClusterTransferAckLevel,
		Owner:                                     &info.Owner,
		ClusterReplicationLevel:                   info.ClusterReplicationLevel,
		PendingFailoverMarkers:                    info.PendingFailoverMarkers,
		PendingFailoverMarkersEncoding:            &info.PendingFailoverMarkersEncoding,
		ReplicationDlqAckLevel:                    info.ReplicationDlqAckLevel,
		TransferProcessingQueueStates:             info.TransferProcessingQueueStates,
		TransferProcessingQueueStatesEncoding:     &info.TransferProcessingQueueStatesEncoding,
		CrossClusterProcessingQueueStates:         info.CrossClusterProcessingQueueStates,
		CrossClusterProcessingQueueStatesEncoding: &info.CrossClusterProcessingQueueStatesEncoding,
		TimerProcessingQueueStates:                info.TimerProcessingQueueStates,
		TimerProcessingQueueStatesEncoding:        &info.TimerProcessingQueueStatesEncoding,
		UpdatedAtNanos:                            timeToUnixNanoPtr(info.UpdatedAt),
		TimerAckLevelNanos:                        timeToUnixNanoPtr(info.TimerAckLevel),
	}
	if info.ClusterTimerAckLevel != nil {
		result.ClusterTimerAckLevel = make(map[string]int64, len(info.ClusterTimerAckLevel))
		for k, v := range info.ClusterTimerAckLevel {
			result.ClusterTimerAckLevel[k] = v.UnixNano()
		}
	}
	if info.QueueStates != nil {
		result.QueueStates = make(map[int32]*shared.QueueState, len(info.QueueStates))
		for k, v := range info.QueueStates {
			result.QueueStates[k] = thrift.FromQueueState(v)
		}
	}
	return result
}

func shardInfoFromThrift(info *sqlblobs.ShardInfo) *ShardInfo {
	if info == nil {
		return nil
	}

	result := &ShardInfo{
		StolenSinceRenew:                          info.GetStolenSinceRenew(),
		ReplicationAckLevel:                       info.GetReplicationAckLevel(),
		TransferAckLevel:                          info.GetTransferAckLevel(),
		DomainNotificationVersion:                 info.GetDomainNotificationVersion(),
		ClusterTransferAckLevel:                   info.ClusterTransferAckLevel,
		Owner:                                     info.GetOwner(),
		ClusterReplicationLevel:                   info.ClusterReplicationLevel,
		PendingFailoverMarkers:                    info.PendingFailoverMarkers,
		PendingFailoverMarkersEncoding:            info.GetPendingFailoverMarkersEncoding(),
		ReplicationDlqAckLevel:                    info.ReplicationDlqAckLevel,
		TransferProcessingQueueStates:             info.TransferProcessingQueueStates,
		TransferProcessingQueueStatesEncoding:     info.GetTransferProcessingQueueStatesEncoding(),
		CrossClusterProcessingQueueStates:         info.CrossClusterProcessingQueueStates,
		CrossClusterProcessingQueueStatesEncoding: info.GetCrossClusterProcessingQueueStatesEncoding(),
		TimerProcessingQueueStates:                info.TimerProcessingQueueStates,
		TimerProcessingQueueStatesEncoding:        info.GetTimerProcessingQueueStatesEncoding(),
		UpdatedAt:                                 timeFromUnixNano(info.GetUpdatedAtNanos()),
		TimerAckLevel:                             timeFromUnixNano(info.GetTimerAckLevelNanos()),
	}
	if info.ClusterTimerAckLevel != nil {
		result.ClusterTimerAckLevel = make(map[string]time.Time, len(info.ClusterTimerAckLevel))
		for k, v := range info.ClusterTimerAckLevel {
			result.ClusterTimerAckLevel[k] = time.Unix(0, v)
		}
	}
	if info.QueueStates != nil {
		result.QueueStates = make(map[int32]*types.QueueState, len(info.QueueStates))
		for k, v := range info.QueueStates {
			result.QueueStates[k] = thrift.ToQueueState(v)
		}
	}
	return result
}

func domainInfoToThrift(info *DomainInfo) *sqlblobs.DomainInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.DomainInfo{
		Name:                                 &info.Name,
		Description:                          &info.Description,
		Owner:                                &info.Owner,
		Status:                               &info.Status,
		EmitMetric:                           &info.EmitMetric,
		ArchivalBucket:                       &info.ArchivalBucket,
		ArchivalStatus:                       &info.ArchivalStatus,
		ConfigVersion:                        &info.ConfigVersion,
		NotificationVersion:                  &info.NotificationVersion,
		FailoverNotificationVersion:          &info.FailoverNotificationVersion,
		FailoverVersion:                      &info.FailoverVersion,
		ActiveClusterName:                    &info.ActiveClusterName,
		ActiveClustersConfiguration:          info.ActiveClustersConfig,
		ActiveClustersConfigurationEncoding:  &info.ActiveClustersConfigEncoding,
		Clusters:                             info.Clusters,
		Data:                                 info.Data,
		BadBinaries:                          info.BadBinaries,
		BadBinariesEncoding:                  &info.BadBinariesEncoding,
		HistoryArchivalStatus:                &info.HistoryArchivalStatus,
		HistoryArchivalURI:                   &info.HistoryArchivalURI,
		VisibilityArchivalStatus:             &info.VisibilityArchivalStatus,
		VisibilityArchivalURI:                &info.VisibilityArchivalURI,
		PreviousFailoverVersion:              &info.PreviousFailoverVersion,
		RetentionDays:                        durationToDaysInt16Ptr(info.Retention),
		FailoverEndTime:                      unixNanoPtr(info.FailoverEndTimestamp),
		LastUpdatedTime:                      timeToUnixNanoPtr(info.LastUpdatedTimestamp),
		IsolationGroupsConfiguration:         info.IsolationGroups,
		IsolationGroupsConfigurationEncoding: &info.IsolationGroupsEncoding,
		AsyncWorkflowConfiguration:           info.AsyncWorkflowConfig,
		AsyncWorkflowConfigurationEncoding:   &info.AsyncWorkflowConfigEncoding,
	}
}

func domainInfoFromThrift(info *sqlblobs.DomainInfo) *DomainInfo {
	if info == nil {
		return nil
	}
	return &DomainInfo{
		Name:                         info.GetName(),
		Description:                  info.GetDescription(),
		Owner:                        info.GetOwner(),
		Status:                       info.GetStatus(),
		EmitMetric:                   info.GetEmitMetric(),
		ArchivalBucket:               info.GetArchivalBucket(),
		ArchivalStatus:               info.GetArchivalStatus(),
		ConfigVersion:                info.GetConfigVersion(),
		NotificationVersion:          info.GetNotificationVersion(),
		FailoverNotificationVersion:  info.GetFailoverNotificationVersion(),
		FailoverVersion:              info.GetFailoverVersion(),
		ActiveClusterName:            info.GetActiveClusterName(),
		ActiveClustersConfig:         info.GetActiveClustersConfiguration(),
		ActiveClustersConfigEncoding: info.GetActiveClustersConfigurationEncoding(),
		Clusters:                     info.Clusters,
		Data:                         info.Data,
		BadBinaries:                  info.BadBinaries,
		BadBinariesEncoding:          info.GetBadBinariesEncoding(),
		HistoryArchivalStatus:        info.GetHistoryArchivalStatus(),
		HistoryArchivalURI:           info.GetHistoryArchivalURI(),
		VisibilityArchivalStatus:     info.GetVisibilityArchivalStatus(),
		VisibilityArchivalURI:        info.GetVisibilityArchivalURI(),
		PreviousFailoverVersion:      info.GetPreviousFailoverVersion(),
		Retention:                    common.DaysToDuration(int32(info.GetRetentionDays())),
		FailoverEndTimestamp:         timePtr(info.FailoverEndTime),
		LastUpdatedTimestamp:         timeFromUnixNano(info.GetLastUpdatedTime()),
		IsolationGroups:              info.GetIsolationGroupsConfiguration(),
		IsolationGroupsEncoding:      info.GetIsolationGroupsConfigurationEncoding(),
		AsyncWorkflowConfig:          info.AsyncWorkflowConfiguration,
		AsyncWorkflowConfigEncoding:  info.GetAsyncWorkflowConfigurationEncoding(),
	}
}

func historyTreeInfoToThrift(info *HistoryTreeInfo) *sqlblobs.HistoryTreeInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.HistoryTreeInfo{
		CreatedTimeNanos: timeToUnixNanoPtr(info.CreatedTimestamp),
		Info:             &info.Info,
		Ancestors:        thrift.FromHistoryBranchRangeArray(info.Ancestors),
	}
}

func historyTreeInfoFromThrift(info *sqlblobs.HistoryTreeInfo) *HistoryTreeInfo {
	if info == nil {
		return nil
	}
	return &HistoryTreeInfo{
		CreatedTimestamp: timeFromUnixNano(info.GetCreatedTimeNanos()),
		Info:             info.GetInfo(),
		Ancestors:        thrift.ToHistoryBranchRangeArray(info.Ancestors),
	}
}

func taskListKindFromThrift(kind *shared.TaskListKind) types.TaskListKind {
	res := thrift.ToTaskListKind(kind)
	if res == nil {
		return types.TaskListKindNormal
	}
	return *res
}

func workflowExecutionInfoToThrift(info *WorkflowExecutionInfo) *sqlblobs.WorkflowExecutionInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.WorkflowExecutionInfo{
		ParentDomainID:                          info.ParentDomainID,
		ParentWorkflowID:                        &info.ParentWorkflowID,
		ParentRunID:                             info.ParentRunID,
		InitiatedID:                             &info.InitiatedID,
		CompletionEventBatchID:                  info.CompletionEventBatchID,
		CompletionEvent:                         info.CompletionEvent,
		CompletionEventEncoding:                 &info.CompletionEventEncoding,
		TaskList:                                &info.TaskList,
		TaskListKind:                            thrift.FromTaskListKind(&info.TaskListKind),
		WorkflowTypeName:                        &info.WorkflowTypeName,
		WorkflowTimeoutSeconds:                  durationToSecondsInt32Ptr(info.WorkflowTimeout),
		DecisionTaskTimeoutSeconds:              durationToSecondsInt32Ptr(info.DecisionTaskTimeout),
		ExecutionContext:                        info.ExecutionContext,
		State:                                   &info.State,
		CloseStatus:                             &info.CloseStatus,
		StartVersion:                            &info.StartVersion,
		LastWriteEventID:                        info.LastWriteEventID,
		LastEventTaskID:                         &info.LastEventTaskID,
		LastFirstEventID:                        &info.LastFirstEventID,
		LastProcessedEvent:                      &info.LastProcessedEvent,
		StartTimeNanos:                          timeToUnixNanoPtr(info.StartTimestamp),
		LastUpdatedTimeNanos:                    timeToUnixNanoPtr(info.LastUpdatedTimestamp),
		DecisionVersion:                         &info.DecisionVersion,
		DecisionScheduleID:                      &info.DecisionScheduleID,
		DecisionStartedID:                       &info.DecisionStartedID,
		DecisionTimeout:                         durationToSecondsInt32Ptr(info.DecisionTimeout),
		DecisionAttempt:                         &info.DecisionAttempt,
		DecisionStartedTimestampNanos:           timeToUnixNanoPtr(info.DecisionStartedTimestamp),
		DecisionScheduledTimestampNanos:         timeToUnixNanoPtr(info.DecisionScheduledTimestamp),
		CancelRequested:                         &info.CancelRequested,
		DecisionOriginalScheduledTimestampNanos: timeToUnixNanoPtr(info.DecisionOriginalScheduledTimestamp),
		CreateRequestID:                         &info.CreateRequestID,
		DecisionRequestID:                       &info.DecisionRequestID,
		CancelRequestID:                         &info.CancelRequestID,
		StickyTaskList:                          &info.StickyTaskList,
		StickyScheduleToStartTimeout:            durationToSecondsInt64Ptr(info.StickyScheduleToStartTimeout),
		RetryAttempt:                            &info.RetryAttempt,
		RetryInitialIntervalSeconds:             durationToSecondsInt32Ptr(info.RetryInitialInterval),
		RetryMaximumIntervalSeconds:             durationToSecondsInt32Ptr(info.RetryMaximumInterval),
		RetryMaximumAttempts:                    &info.RetryMaximumAttempts,
		RetryExpirationSeconds:                  durationToSecondsInt32Ptr(info.RetryExpiration),
		RetryBackoffCoefficient:                 &info.RetryBackoffCoefficient,
		RetryExpirationTimeNanos:                timeToUnixNanoPtr(info.RetryExpirationTimestamp),
		RetryNonRetryableErrors:                 info.RetryNonRetryableErrors,
		HasRetryPolicy:                          &info.HasRetryPolicy,
		CronSchedule:                            &info.CronSchedule,
		CronOverlapPolicy:                       thrift.FromCronOverlapPolicy(&info.CronOverlapPolicy),
		EventStoreVersion:                       &info.EventStoreVersion,
		EventBranchToken:                        info.EventBranchToken,
		SignalCount:                             &info.SignalCount,
		HistorySize:                             &info.HistorySize,
		ClientLibraryVersion:                    &info.ClientLibraryVersion,
		ClientFeatureVersion:                    &info.ClientFeatureVersion,
		ClientImpl:                              &info.ClientImpl,
		AutoResetPoints:                         info.AutoResetPoints,
		AutoResetPointsEncoding:                 &info.AutoResetPointsEncoding,
		SearchAttributes:                        info.SearchAttributes,
		Memo:                                    info.Memo,
		VersionHistories:                        info.VersionHistories,
		VersionHistoriesEncoding:                &info.VersionHistoriesEncoding,
		FirstExecutionRunID:                     info.FirstExecutionRunID,
		PartitionConfig:                         info.PartitionConfig,
		Checksum:                                info.Checksum,
		ChecksumEncoding:                        &info.ChecksumEncoding,
		ActiveClusterSelectionPolicy:            info.ActiveClusterSelectionPolicy,
		ActiveClusterSelectionPolicyEncoding:    &info.ActiveClusterSelectionPolicyEncoding,
	}
}

func workflowExecutionInfoFromThrift(info *sqlblobs.WorkflowExecutionInfo) *WorkflowExecutionInfo {
	if info == nil {
		return nil
	}
	return &WorkflowExecutionInfo{
		ParentDomainID:                       info.ParentDomainID,
		ParentWorkflowID:                     info.GetParentWorkflowID(),
		ParentRunID:                          info.ParentRunID,
		InitiatedID:                          info.GetInitiatedID(),
		CompletionEventBatchID:               info.CompletionEventBatchID,
		CompletionEvent:                      info.CompletionEvent,
		CompletionEventEncoding:              info.GetCompletionEventEncoding(),
		TaskList:                             info.GetTaskList(),
		TaskListKind:                         taskListKindFromThrift(info.TaskListKind),
		WorkflowTypeName:                     info.GetWorkflowTypeName(),
		WorkflowTimeout:                      common.SecondsToDuration(int64(info.GetWorkflowTimeoutSeconds())),
		DecisionTaskTimeout:                  common.SecondsToDuration(int64(info.GetDecisionTaskTimeoutSeconds())),
		ExecutionContext:                     info.ExecutionContext,
		State:                                info.GetState(),
		CloseStatus:                          info.GetCloseStatus(),
		StartVersion:                         info.GetStartVersion(),
		LastWriteEventID:                     info.LastWriteEventID,
		LastEventTaskID:                      info.GetLastEventTaskID(),
		LastFirstEventID:                     info.GetLastFirstEventID(),
		LastProcessedEvent:                   info.GetLastProcessedEvent(),
		StartTimestamp:                       timeFromUnixNano(info.GetStartTimeNanos()),
		LastUpdatedTimestamp:                 timeFromUnixNano(info.GetLastUpdatedTimeNanos()),
		DecisionVersion:                      info.GetDecisionVersion(),
		DecisionScheduleID:                   info.GetDecisionScheduleID(),
		DecisionStartedID:                    info.GetDecisionStartedID(),
		DecisionTimeout:                      common.SecondsToDuration(int64(info.GetDecisionTimeout())),
		DecisionAttempt:                      info.GetDecisionAttempt(),
		DecisionStartedTimestamp:             timeFromUnixNano(info.GetDecisionStartedTimestampNanos()),
		DecisionScheduledTimestamp:           timeFromUnixNano(info.GetDecisionScheduledTimestampNanos()),
		CancelRequested:                      info.GetCancelRequested(),
		DecisionOriginalScheduledTimestamp:   timeFromUnixNano(info.GetDecisionOriginalScheduledTimestampNanos()),
		CreateRequestID:                      info.GetCreateRequestID(),
		DecisionRequestID:                    info.GetDecisionRequestID(),
		CancelRequestID:                      info.GetCancelRequestID(),
		StickyTaskList:                       info.GetStickyTaskList(),
		StickyScheduleToStartTimeout:         common.SecondsToDuration(info.GetStickyScheduleToStartTimeout()),
		RetryAttempt:                         info.GetRetryAttempt(),
		RetryInitialInterval:                 common.SecondsToDuration(int64(info.GetRetryInitialIntervalSeconds())),
		RetryMaximumInterval:                 common.SecondsToDuration(int64(info.GetRetryMaximumIntervalSeconds())),
		RetryMaximumAttempts:                 info.GetRetryMaximumAttempts(),
		RetryExpiration:                      common.SecondsToDuration(int64(info.GetRetryExpirationSeconds())),
		RetryBackoffCoefficient:              info.GetRetryBackoffCoefficient(),
		RetryExpirationTimestamp:             timeFromUnixNano(info.GetRetryExpirationTimeNanos()),
		RetryNonRetryableErrors:              info.RetryNonRetryableErrors,
		HasRetryPolicy:                       info.GetHasRetryPolicy(),
		CronSchedule:                         info.GetCronSchedule(),
		CronOverlapPolicy:                    cronOverlapPolicyFromThrift(info.CronOverlapPolicy),
		EventStoreVersion:                    info.GetEventStoreVersion(),
		EventBranchToken:                     info.EventBranchToken,
		SignalCount:                          info.GetSignalCount(),
		HistorySize:                          info.GetHistorySize(),
		ClientLibraryVersion:                 info.GetClientLibraryVersion(),
		ClientFeatureVersion:                 info.GetClientFeatureVersion(),
		ClientImpl:                           info.GetClientImpl(),
		AutoResetPoints:                      info.AutoResetPoints,
		AutoResetPointsEncoding:              info.GetAutoResetPointsEncoding(),
		SearchAttributes:                     info.SearchAttributes,
		Memo:                                 info.Memo,
		VersionHistories:                     info.VersionHistories,
		VersionHistoriesEncoding:             info.GetVersionHistoriesEncoding(),
		FirstExecutionRunID:                  info.FirstExecutionRunID,
		PartitionConfig:                      info.PartitionConfig,
		IsCron:                               info.GetCronSchedule() != "",
		Checksum:                             info.Checksum,
		ChecksumEncoding:                     info.GetChecksumEncoding(),
		ActiveClusterSelectionPolicy:         info.ActiveClusterSelectionPolicy,
		ActiveClusterSelectionPolicyEncoding: info.GetActiveClusterSelectionPolicyEncoding(),
	}
}

func activityInfoToThrift(info *ActivityInfo) *sqlblobs.ActivityInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ActivityInfo{
		Version:                       &info.Version,
		ScheduledEventBatchID:         &info.ScheduledEventBatchID,
		ScheduledEvent:                info.ScheduledEvent,
		ScheduledEventEncoding:        &info.ScheduledEventEncoding,
		ScheduledTimeNanos:            timeToUnixNanoPtr(info.ScheduledTimestamp),
		StartedID:                     &info.StartedID,
		StartedEvent:                  info.StartedEvent,
		StartedEventEncoding:          &info.StartedEventEncoding,
		StartedTimeNanos:              timeToUnixNanoPtr(info.StartedTimestamp),
		ActivityID:                    &info.ActivityID,
		RequestID:                     &info.RequestID,
		ScheduleToStartTimeoutSeconds: durationToSecondsInt32Ptr(info.ScheduleToStartTimeout),
		ScheduleToCloseTimeoutSeconds: durationToSecondsInt32Ptr(info.ScheduleToCloseTimeout),
		StartToCloseTimeoutSeconds:    durationToSecondsInt32Ptr(info.StartToCloseTimeout),
		HeartbeatTimeoutSeconds:       durationToSecondsInt32Ptr(info.HeartbeatTimeout),
		CancelRequested:               &info.CancelRequested,
		CancelRequestID:               &info.CancelRequestID,
		TimerTaskStatus:               &info.TimerTaskStatus,
		Attempt:                       &info.Attempt,
		TaskList:                      &info.TaskList,
		TaskListKind:                  thrift.FromTaskListKind(&info.TaskListKind),
		StartedIdentity:               &info.StartedIdentity,
		HasRetryPolicy:                &info.HasRetryPolicy,
		RetryInitialIntervalSeconds:   durationToSecondsInt32Ptr(info.RetryInitialInterval),
		RetryMaximumIntervalSeconds:   durationToSecondsInt32Ptr(info.RetryMaximumInterval),
		RetryMaximumAttempts:          &info.RetryMaximumAttempts,
		RetryExpirationTimeNanos:      timeToUnixNanoPtr(info.RetryExpirationTimestamp),
		RetryBackoffCoefficient:       &info.RetryBackoffCoefficient,
		RetryNonRetryableErrors:       info.RetryNonRetryableErrors,
		RetryLastFailureReason:        &info.RetryLastFailureReason,
		RetryLastWorkerIdentity:       &info.RetryLastWorkerIdentity,
		RetryLastFailureDetails:       info.RetryLastFailureDetails,
	}
}

func activityInfoFromThrift(info *sqlblobs.ActivityInfo) *ActivityInfo {
	if info == nil {
		return nil
	}
	return &ActivityInfo{
		Version:                  info.GetVersion(),
		ScheduledEventBatchID:    info.GetScheduledEventBatchID(),
		ScheduledEvent:           info.ScheduledEvent,
		ScheduledEventEncoding:   info.GetScheduledEventEncoding(),
		ScheduledTimestamp:       timeFromUnixNano(info.GetScheduledTimeNanos()),
		StartedID:                info.GetStartedID(),
		StartedEvent:             info.StartedEvent,
		StartedEventEncoding:     info.GetStartedEventEncoding(),
		StartedTimestamp:         timeFromUnixNano(info.GetStartedTimeNanos()),
		ActivityID:               info.GetActivityID(),
		RequestID:                info.GetRequestID(),
		ScheduleToStartTimeout:   common.SecondsToDuration(int64(info.GetScheduleToStartTimeoutSeconds())),
		ScheduleToCloseTimeout:   common.SecondsToDuration(int64(info.GetScheduleToCloseTimeoutSeconds())),
		StartToCloseTimeout:      common.SecondsToDuration(int64(info.GetStartToCloseTimeoutSeconds())),
		HeartbeatTimeout:         common.SecondsToDuration(int64(info.GetHeartbeatTimeoutSeconds())),
		CancelRequested:          info.GetCancelRequested(),
		CancelRequestID:          info.GetCancelRequestID(),
		TimerTaskStatus:          info.GetTimerTaskStatus(),
		Attempt:                  info.GetAttempt(),
		TaskList:                 info.GetTaskList(),
		TaskListKind:             taskListKindFromThrift(info.TaskListKind),
		StartedIdentity:          info.GetStartedIdentity(),
		HasRetryPolicy:           info.GetHasRetryPolicy(),
		RetryInitialInterval:     common.SecondsToDuration(int64(info.GetRetryInitialIntervalSeconds())),
		RetryMaximumInterval:     common.SecondsToDuration(int64(info.GetRetryMaximumIntervalSeconds())),
		RetryMaximumAttempts:     info.GetRetryMaximumAttempts(),
		RetryExpirationTimestamp: timeFromUnixNano(info.GetRetryExpirationTimeNanos()),
		RetryBackoffCoefficient:  info.GetRetryBackoffCoefficient(),
		RetryNonRetryableErrors:  info.RetryNonRetryableErrors,
		RetryLastFailureReason:   info.GetRetryLastFailureReason(),
		RetryLastWorkerIdentity:  info.GetRetryLastWorkerIdentity(),
		RetryLastFailureDetails:  info.RetryLastFailureDetails,
	}
}

func childExecutionInfoToThrift(info *ChildExecutionInfo) *sqlblobs.ChildExecutionInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ChildExecutionInfo{
		Version:                &info.Version,
		InitiatedEventBatchID:  &info.InitiatedEventBatchID,
		StartedID:              &info.StartedID,
		InitiatedEvent:         info.InitiatedEvent,
		InitiatedEventEncoding: &info.InitiatedEventEncoding,
		StartedWorkflowID:      &info.StartedWorkflowID,
		StartedRunID:           info.StartedRunID,
		StartedEvent:           info.StartedEvent,
		StartedEventEncoding:   &info.StartedEventEncoding,
		CreateRequestID:        &info.CreateRequestID,
		DomainID:               &info.DomainID,
		DomainName:             &info.DomainNameDEPRECATED,
		WorkflowTypeName:       &info.WorkflowTypeName,
		ParentClosePolicy:      &info.ParentClosePolicy,
	}
}

func childExecutionInfoFromThrift(info *sqlblobs.ChildExecutionInfo) *ChildExecutionInfo {
	if info == nil {
		return nil
	}
	return &ChildExecutionInfo{
		Version:                info.GetVersion(),
		InitiatedEventBatchID:  info.GetInitiatedEventBatchID(),
		StartedID:              info.GetStartedID(),
		InitiatedEvent:         info.InitiatedEvent,
		InitiatedEventEncoding: info.GetInitiatedEventEncoding(),
		StartedWorkflowID:      info.GetStartedWorkflowID(),
		StartedRunID:           info.GetStartedRunID(),
		StartedEvent:           info.StartedEvent,
		StartedEventEncoding:   info.GetStartedEventEncoding(),
		CreateRequestID:        info.GetCreateRequestID(),
		DomainID:               info.GetDomainID(),
		DomainNameDEPRECATED:   info.GetDomainName(),
		WorkflowTypeName:       info.GetWorkflowTypeName(),
		ParentClosePolicy:      info.GetParentClosePolicy(),
	}
}

func signalInfoToThrift(info *SignalInfo) *sqlblobs.SignalInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.SignalInfo{
		Version:               &info.Version,
		InitiatedEventBatchID: &info.InitiatedEventBatchID,
		RequestID:             &info.RequestID,
		Name:                  &info.Name,
		Input:                 info.Input,
		Control:               info.Control,
	}
}

func signalInfoFromThrift(info *sqlblobs.SignalInfo) *SignalInfo {
	if info == nil {
		return nil
	}
	return &SignalInfo{
		Version:               info.GetVersion(),
		InitiatedEventBatchID: info.GetInitiatedEventBatchID(),
		RequestID:             info.GetRequestID(),
		Name:                  info.GetName(),
		Input:                 info.Input,
		Control:               info.Control,
	}
}

func requestCancelInfoToThrift(info *RequestCancelInfo) *sqlblobs.RequestCancelInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.RequestCancelInfo{
		Version:               &info.Version,
		InitiatedEventBatchID: &info.InitiatedEventBatchID,
		CancelRequestID:       &info.CancelRequestID,
	}
}

func requestCancelInfoFromThrift(info *sqlblobs.RequestCancelInfo) *RequestCancelInfo {
	if info == nil {
		return nil
	}
	return &RequestCancelInfo{
		Version:               info.GetVersion(),
		InitiatedEventBatchID: info.GetInitiatedEventBatchID(),
		CancelRequestID:       info.GetCancelRequestID(),
	}
}

func timerInfoToThrift(info *TimerInfo) *sqlblobs.TimerInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TimerInfo{
		Version:         &info.Version,
		StartedID:       &info.StartedID,
		ExpiryTimeNanos: timeToUnixNanoPtr(info.ExpiryTimestamp),
		TaskID:          &info.TaskID,
	}
}

func timerInfoFromThrift(info *sqlblobs.TimerInfo) *TimerInfo {
	if info == nil {
		return nil
	}
	return &TimerInfo{
		Version:         info.GetVersion(),
		StartedID:       info.GetStartedID(),
		ExpiryTimestamp: timeFromUnixNano(info.GetExpiryTimeNanos()),
		TaskID:          info.GetTaskID(),
	}
}

func taskInfoToThrift(info *TaskInfo) *sqlblobs.TaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TaskInfo{
		WorkflowID:       &info.WorkflowID,
		RunID:            info.RunID,
		ScheduleID:       &info.ScheduleID,
		ExpiryTimeNanos:  timeToUnixNanoPtr(info.ExpiryTimestamp),
		CreatedTimeNanos: timeToUnixNanoPtr(info.CreatedTimestamp),
		PartitionConfig:  info.PartitionConfig,
	}
}

func taskInfoFromThrift(info *sqlblobs.TaskInfo) *TaskInfo {
	if info == nil {
		return nil
	}
	return &TaskInfo{
		WorkflowID:       info.GetWorkflowID(),
		RunID:            info.RunID,
		ScheduleID:       info.GetScheduleID(),
		ExpiryTimestamp:  timeFromUnixNano(info.GetExpiryTimeNanos()),
		CreatedTimestamp: timeFromUnixNano(info.GetCreatedTimeNanos()),
		PartitionConfig:  info.PartitionConfig,
	}
}

func taskListPartitionConfigToThrift(info *TaskListPartitionConfig) *sqlblobs.TaskListPartitionConfig {
	if info == nil {
		return nil
	}
	return &sqlblobs.TaskListPartitionConfig{
		Version:            &info.Version,
		NumReadPartitions:  common.Int32Ptr(info.NumReadPartitions),
		NumWritePartitions: common.Int32Ptr(info.NumWritePartitions),
		ReadPartitions:     taskListPartitionMapToThrift(info.ReadPartitions),
		WritePartitions:    taskListPartitionMapToThrift(info.WritePartitions),
	}
}

func taskListPartitionMapToThrift(m map[int32]*TaskListPartition) map[int32]*sqlblobs.TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int32]*sqlblobs.TaskListPartition)
	for id, p := range m {
		result[id] = &sqlblobs.TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	return result
}

func taskListPartitionMapFromThrift(m map[int32]*sqlblobs.TaskListPartition) map[int32]*TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int32]*TaskListPartition)
	for id, p := range m {
		result[id] = &TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	return result
}

func taskListParititionConfigFromThrift(info *sqlblobs.TaskListPartitionConfig) *TaskListPartitionConfig {
	if info == nil {
		return nil
	}
	return &TaskListPartitionConfig{
		Version:            info.GetVersion(),
		NumReadPartitions:  info.GetNumReadPartitions(),
		NumWritePartitions: info.GetNumWritePartitions(),
		ReadPartitions:     taskListPartitionMapFromThrift(info.ReadPartitions),
		WritePartitions:    taskListPartitionMapFromThrift(info.WritePartitions),
	}
}

func taskListInfoToThrift(info *TaskListInfo) *sqlblobs.TaskListInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TaskListInfo{
		Kind:                    &info.Kind,
		AckLevel:                &info.AckLevel,
		ExpiryTimeNanos:         timeToUnixNanoPtr(info.ExpiryTimestamp),
		LastUpdatedNanos:        timeToUnixNanoPtr(info.LastUpdated),
		AdaptivePartitionConfig: taskListPartitionConfigToThrift(info.AdaptivePartitionConfig),
	}
}

func taskListInfoFromThrift(info *sqlblobs.TaskListInfo) *TaskListInfo {
	if info == nil {
		return nil
	}
	return &TaskListInfo{
		Kind:                    info.GetKind(),
		AckLevel:                info.GetAckLevel(),
		ExpiryTimestamp:         timeFromUnixNano(info.GetExpiryTimeNanos()),
		LastUpdated:             timeFromUnixNano(info.GetLastUpdatedNanos()),
		AdaptivePartitionConfig: taskListParititionConfigFromThrift(info.AdaptivePartitionConfig),
	}
}

func transferTaskInfoToThrift(info *TransferTaskInfo) *sqlblobs.TransferTaskInfo {
	if info == nil {
		return nil
	}
	thriftTaskInfo := &sqlblobs.TransferTaskInfo{
		DomainID:       info.DomainID,
		WorkflowID:     &info.WorkflowID,
		RunID:          info.RunID,
		TaskType:       &info.TaskType,
		TargetDomainID: info.TargetDomainID,
		// TargetDomainIDs will be assigned below
		TargetWorkflowID:         &info.TargetWorkflowID,
		TargetRunID:              info.TargetRunID,
		TaskList:                 &info.TaskList,
		TargetChildWorkflowOnly:  &info.TargetChildWorkflowOnly,
		ScheduleID:               &info.ScheduleID,
		Version:                  &info.Version,
		VisibilityTimestampNanos: timeToUnixNanoPtr(info.VisibilityTimestamp),
	}
	if len(info.TargetDomainIDs) > 0 {
		thriftTaskInfo.TargetDomainIDs = [][]byte{}
		for _, domainID := range info.TargetDomainIDs {
			thriftTaskInfo.TargetDomainIDs = append(thriftTaskInfo.TargetDomainIDs, domainID)
		}
	}
	return thriftTaskInfo
}

func transferTaskInfoFromThrift(info *sqlblobs.TransferTaskInfo) *TransferTaskInfo {
	if info == nil {
		return nil
	}
	transferTaskInfo := &TransferTaskInfo{
		DomainID:       info.DomainID,
		WorkflowID:     info.GetWorkflowID(),
		RunID:          info.RunID,
		TaskType:       info.GetTaskType(),
		TargetDomainID: info.TargetDomainID,
		// TargetDomainIDs will be assigned below
		TargetWorkflowID:        info.GetTargetWorkflowID(),
		TargetRunID:             info.TargetRunID,
		TaskList:                info.GetTaskList(),
		TargetChildWorkflowOnly: info.GetTargetChildWorkflowOnly(),
		ScheduleID:              info.GetScheduleID(),
		Version:                 info.GetVersion(),
		VisibilityTimestamp:     timeFromUnixNano(info.GetVisibilityTimestampNanos()),
	}
	if len(info.GetTargetDomainIDs()) > 0 {
		transferTaskInfo.TargetDomainIDs = []UUID{}
		for _, domainID := range info.GetTargetDomainIDs() {
			transferTaskInfo.TargetDomainIDs = append(transferTaskInfo.TargetDomainIDs, domainID)
		}
	}
	return transferTaskInfo
}

func crossClusterTaskInfoToThrift(info *CrossClusterTaskInfo) *sqlblobsCrossClusterTaskInfo {
	return transferTaskInfoToThrift(info)
}

func crossClusterTaskInfoFromThrift(info *sqlblobsCrossClusterTaskInfo) *CrossClusterTaskInfo {
	return transferTaskInfoFromThrift(info)
}

func timerTaskInfoToThrift(info *TimerTaskInfo) *sqlblobs.TimerTaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TimerTaskInfo{
		DomainID:        info.DomainID,
		WorkflowID:      &info.WorkflowID,
		RunID:           info.RunID,
		TaskType:        &info.TaskType,
		TimeoutType:     info.TimeoutType,
		Version:         &info.Version,
		ScheduleAttempt: &info.ScheduleAttempt,
		EventID:         &info.EventID,
	}
}

func timerTaskInfoFromThrift(info *sqlblobs.TimerTaskInfo) *TimerTaskInfo {
	if info == nil {
		return nil
	}
	return &TimerTaskInfo{
		DomainID:        info.DomainID,
		WorkflowID:      info.GetWorkflowID(),
		RunID:           info.RunID,
		TaskType:        info.GetTaskType(),
		TimeoutType:     info.TimeoutType,
		Version:         info.GetVersion(),
		ScheduleAttempt: info.GetScheduleAttempt(),
		EventID:         info.GetEventID(),
	}
}

func replicationTaskInfoToThrift(info *ReplicationTaskInfo) *sqlblobs.ReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ReplicationTaskInfo{
		DomainID:                info.DomainID,
		WorkflowID:              &info.WorkflowID,
		RunID:                   info.RunID,
		TaskType:                &info.TaskType,
		Version:                 &info.Version,
		FirstEventID:            &info.FirstEventID,
		NextEventID:             &info.NextEventID,
		ScheduledID:             &info.ScheduledID,
		EventStoreVersion:       &info.EventStoreVersion,
		NewRunEventStoreVersion: &info.NewRunEventStoreVersion,
		BranchToken:             info.BranchToken,
		NewRunBranchToken:       info.NewRunBranchToken,
		CreationTime:            timeToUnixNanoPtr(info.CreationTimestamp),
	}
}

func replicationTaskInfoFromThrift(info *sqlblobs.ReplicationTaskInfo) *ReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &ReplicationTaskInfo{
		DomainID:                info.DomainID,
		WorkflowID:              info.GetWorkflowID(),
		RunID:                   info.RunID,
		TaskType:                info.GetTaskType(),
		Version:                 info.GetVersion(),
		FirstEventID:            info.GetFirstEventID(),
		NextEventID:             info.GetNextEventID(),
		ScheduledID:             info.GetScheduledID(),
		EventStoreVersion:       info.GetEventStoreVersion(),
		NewRunEventStoreVersion: info.GetNewRunEventStoreVersion(),
		BranchToken:             info.BranchToken,
		NewRunBranchToken:       info.NewRunBranchToken,
		CreationTimestamp:       timeFromUnixNano(info.GetCreationTime()),
	}
}

var zeroTimeNanos = time.Time{}.UnixNano()

func unixNanoPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	return common.Int64Ptr(t.UnixNano())
}

func timeToUnixNanoPtr(t time.Time) *int64 {
	return common.Int64Ptr(t.UnixNano())
}

func timePtr(t *int64) *time.Time {
	if t == nil {
		return nil
	}
	// Calling UnixNano() on zero time is undefined an results in a number that is converted back to 1754-08-30T22:43:41Z
	// Handle such case explicitly
	if *t == zeroTimeNanos {
		return common.TimePtr(time.Time{})
	}
	return common.TimePtr(time.Unix(0, *t))
}

func timeFromUnixNano(t int64) time.Time {
	// Calling UnixNano() on zero time is undefined an results in a number that is converted back to 1754-08-30T22:43:41Z
	// Handle such case explicitly
	if t == zeroTimeNanos {
		return time.Time{}
	}
	return time.Unix(0, t)
}

func durationToSecondsInt32Ptr(t time.Duration) *int32 {
	return common.Int32Ptr(int32(common.DurationToSeconds(t)))
}

func durationToSecondsInt64Ptr(t time.Duration) *int64 {
	return common.Int64Ptr(common.DurationToSeconds(t))
}

func durationToDaysInt16Ptr(t time.Duration) *int16 {
	return common.Int16Ptr(int16(common.DurationToDays(t)))
}

func cronOverlapPolicyFromThrift(t *shared.CronOverlapPolicy) types.CronOverlapPolicy {
	if t == nil {
		return types.CronOverlapPolicySkipped
	}
	if result := thrift.ToCronOverlapPolicy(t); result != nil {
		return *result
	}
	return types.CronOverlapPolicySkipped
}
