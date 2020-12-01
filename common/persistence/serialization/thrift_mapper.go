package serialization

import (
	"time"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/.gen/go/sqlblobs"
	"github.com/uber/cadence/common"
)

func shardInfoToThrift(info *ShardInfo) *sqlblobs.ShardInfo {
	if info == nil {
		return nil
	}
	result := &sqlblobs.ShardInfo{
		StolenSinceRenew:                      info.StolenSinceRenew,
		ReplicationAckLevel:                   info.ReplicationAckLevel,
		TransferAckLevel:                      info.TransferAckLevel,
		DomainNotificationVersion:             info.DomainNotificationVersion,
		ClusterTransferAckLevel:               info.ClusterTransferAckLevel,
		Owner:                                 info.Owner,
		ClusterReplicationLevel:               info.ClusterReplicationLevel,
		PendingFailoverMarkers:                info.PendingFailoverMarkers,
		PendingFailoverMarkersEncoding:        info.PendingFailoverMarkersEncoding,
		ReplicationDlqAckLevel:                info.ReplicationDlqAckLevel,
		TransferProcessingQueueStates:         info.TransferProcessingQueueStates,
		TransferProcessingQueueStatesEncoding: info.TransferProcessingQueueStatesEncoding,
		TimerProcessingQueueStates:            info.TimerProcessingQueueStates,
		TimerProcessingQueueStatesEncoding:    info.TimerProcessingQueueStatesEncoding,
		UpdatedAtNanos:                        unixNanoPtr(info.UpdatedAt),
		TimerAckLevelNanos:                    unixNanoPtr(info.TimerAckLevel),
	}
	if info.ClusterTimerAckLevel != nil {
		result.ClusterTimerAckLevel = make(map[string]int64, len(info.ClusterTimerAckLevel))
		for k, v := range info.ClusterTimerAckLevel {
			result.ClusterTimerAckLevel[k] = v.UnixNano()
		}
	}
	return result
}

func shardInfoFromThrift(info *sqlblobs.ShardInfo) *ShardInfo {
	if info == nil {
		return nil
	}

	result := &ShardInfo{
		StolenSinceRenew:                      info.StolenSinceRenew,
		ReplicationAckLevel:                   info.ReplicationAckLevel,
		TransferAckLevel:                      info.TransferAckLevel,
		DomainNotificationVersion:             info.DomainNotificationVersion,
		ClusterTransferAckLevel:               info.ClusterTransferAckLevel,
		Owner:                                 info.Owner,
		ClusterReplicationLevel:               info.ClusterReplicationLevel,
		PendingFailoverMarkers:                info.PendingFailoverMarkers,
		PendingFailoverMarkersEncoding:        info.PendingFailoverMarkersEncoding,
		ReplicationDlqAckLevel:                info.ReplicationDlqAckLevel,
		TransferProcessingQueueStates:         info.TransferProcessingQueueStates,
		TransferProcessingQueueStatesEncoding: info.TransferProcessingQueueStatesEncoding,
		TimerProcessingQueueStates:            info.TimerProcessingQueueStates,
		TimerProcessingQueueStatesEncoding:    info.TimerProcessingQueueStatesEncoding,
		UpdatedAt:                             timePtr(info.UpdatedAtNanos),
		TimerAckLevel:                         timePtr(info.TimerAckLevelNanos),
	}
	if info.ClusterTimerAckLevel != nil {
		result.ClusterTimerAckLevel = make(map[string]time.Time, len(info.ClusterTimerAckLevel))
		for k, v := range info.ClusterTimerAckLevel {
			result.ClusterTimerAckLevel[k] = time.Unix(0, v)
		}
	}
	return result
}

func domainInfoToThrift(info *DomainInfo) *sqlblobs.DomainInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.DomainInfo{
		Name:                        info.Name,
		Description:                 info.Description,
		Owner:                       info.Owner,
		Status:                      info.Status,
		EmitMetric:                  info.EmitMetric,
		ArchivalBucket:              info.ArchivalBucket,
		ArchivalStatus:              info.ArchivalStatus,
		ConfigVersion:               info.ConfigVersion,
		NotificationVersion:         info.NotificationVersion,
		FailoverNotificationVersion: info.FailoverNotificationVersion,
		FailoverVersion:             info.FailoverVersion,
		ActiveClusterName:           info.ActiveClusterName,
		Clusters:                    info.Clusters,
		Data:                        info.Data,
		BadBinaries:                 info.BadBinaries,
		BadBinariesEncoding:         info.BadBinariesEncoding,
		HistoryArchivalStatus:       info.HistoryArchivalStatus,
		HistoryArchivalURI:          info.HistoryArchivalURI,
		VisibilityArchivalStatus:    info.VisibilityArchivalStatus,
		VisibilityArchivalURI:       info.VisibilityArchivalURI,
		PreviousFailoverVersion:     info.PreviousFailoverVersion,
		RetentionDays:               durationToDays(info.RetentionDays),
		FailoverEndTime:             unixNanoPtr(info.FailoverEndTime),
		LastUpdatedTime:             unixNanoPtr(info.LastUpdatedTime),
	}
}

func domainInfoFromThrift(info *sqlblobs.DomainInfo) *DomainInfo {
	if info == nil {
		return nil
	}
	return &DomainInfo{
		Name:                        info.Name,
		Description:                 info.Description,
		Owner:                       info.Owner,
		Status:                      info.Status,
		EmitMetric:                  info.EmitMetric,
		ArchivalBucket:              info.ArchivalBucket,
		ArchivalStatus:              info.ArchivalStatus,
		ConfigVersion:               info.ConfigVersion,
		NotificationVersion:         info.NotificationVersion,
		FailoverNotificationVersion: info.FailoverNotificationVersion,
		FailoverVersion:             info.FailoverVersion,
		ActiveClusterName:           info.ActiveClusterName,
		Clusters:                    info.Clusters,
		Data:                        info.Data,
		BadBinaries:                 info.BadBinaries,
		BadBinariesEncoding:         info.BadBinariesEncoding,
		HistoryArchivalStatus:       info.HistoryArchivalStatus,
		HistoryArchivalURI:          info.HistoryArchivalURI,
		VisibilityArchivalStatus:    info.VisibilityArchivalStatus,
		VisibilityArchivalURI:       info.VisibilityArchivalURI,
		PreviousFailoverVersion:     info.PreviousFailoverVersion,
		RetentionDays:               daysToDuration(info.RetentionDays),
		FailoverEndTime:             timePtr(info.FailoverEndTime),
		LastUpdatedTime:             timePtr(info.LastUpdatedTime),
	}
}

func historyTreeInfoToThrift(info *HistoryTreeInfo) *sqlblobs.HistoryTreeInfo {
	if info == nil {
		return nil
	}
	result := &sqlblobs.HistoryTreeInfo{
		CreatedTimeNanos: unixNanoPtr(info.CreatedTime),
		Info:             info.Info,
	}
	if info.Ancestors != nil {
		result.Ancestors = make([]*shared.HistoryBranchRange, len(info.Ancestors), len(info.Ancestors))
		for i, a := range info.Ancestors {
			result.Ancestors[i] = &shared.HistoryBranchRange{
				BranchID:    a.BranchID,
				BeginNodeID: a.BeginNodeID,
				EndNodeID:   a.EndNodeID,
			}
		}
	}
	return result
}

func historyTreeInfoFromThrift(info *sqlblobs.HistoryTreeInfo) *HistoryTreeInfo {
	if info == nil {
		return nil
	}
	result := &HistoryTreeInfo{
		CreatedTime: timePtr(info.CreatedTimeNanos),
		Info:        info.Info,
	}
	if info.Ancestors != nil {
		result.Ancestors = make([]*HistoryBranchRange, len(info.Ancestors), len(info.Ancestors))
		for i, a := range info.Ancestors {
			result.Ancestors[i] = &HistoryBranchRange{
				BranchID:    a.BranchID,
				BeginNodeID: a.BeginNodeID,
				EndNodeID:   a.EndNodeID,
			}
		}
	}
	return result
}

func workflowExecutionInfoToThrift(info *WorkflowExecutionInfo) *sqlblobs.WorkflowExecutionInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.WorkflowExecutionInfo{
		ParentDomainID:                          MustParsePtrUUID(info.ParentDomainID),
		ParentWorkflowID:                        info.ParentWorkflowID,
		ParentRunID:                             MustParsePtrUUID(info.ParentRunID),
		InitiatedID:                             info.InitiatedID,
		CompletionEventBatchID:                  info.CompletionEventBatchID,
		CompletionEvent:                         info.CompletionEvent,
		CompletionEventEncoding:                 info.CompletionEventEncoding,
		TaskList:                                info.TaskList,
		WorkflowTypeName:                        info.WorkflowTypeName,
		WorkflowTimeoutSeconds:                  durationToSeconds(info.WorkflowTimeoutSeconds),
		DecisionTaskTimeoutSeconds:              durationToSeconds(info.DecisionTaskTimeoutSeconds),
		ExecutionContext:                        info.ExecutionContext,
		State:                                   info.State,
		CloseStatus:                             info.CloseStatus,
		StartVersion:                            info.StartVersion,
		LastWriteEventID:                        info.LastWriteEventID,
		LastEventTaskID:                         info.LastEventTaskID,
		LastFirstEventID:                        info.LastFirstEventID,
		LastProcessedEvent:                      info.LastProcessedEvent,
		StartTimeNanos:                          unixNanoPtr(info.StartTime),
		LastUpdatedTimeNanos:                    unixNanoPtr(info.LastUpdatedTime),
		DecisionVersion:                         info.DecisionVersion,
		DecisionScheduleID:                      info.DecisionScheduleID,
		DecisionStartedID:                       info.DecisionStartedID,
		DecisionTimeout:                         durationToSeconds(info.DecisionTimeout),
		DecisionAttempt:                         info.DecisionAttempt,
		DecisionStartedTimestampNanos:           unixNanoPtr(info.DecisionStartedTimestamp),
		DecisionScheduledTimestampNanos:         unixNanoPtr(info.DecisionScheduledTimestamp),
		CancelRequested:                         info.CancelRequested,
		DecisionOriginalScheduledTimestampNanos: unixNanoPtr(info.DecisionOriginalScheduledTimestamp),
		CreateRequestID:                         info.CreateRequestID,
		DecisionRequestID:                       info.DecisionRequestID,
		CancelRequestID:                         info.CancelRequestID,
		StickyTaskList:                          info.StickyTaskList,
		StickyScheduleToStartTimeout:            durationToSecondsInt64(info.StickyScheduleToStartTimeout),
		RetryAttempt:                            info.RetryAttempt,
		RetryInitialIntervalSeconds:             durationToSeconds(info.RetryInitialIntervalSeconds),
		RetryMaximumIntervalSeconds:             durationToSeconds(info.RetryMaximumIntervalSeconds),
		RetryMaximumAttempts:                    info.RetryMaximumAttempts,
		RetryExpirationSeconds:                  durationToSeconds(info.RetryExpirationSeconds),
		RetryBackoffCoefficient:                 info.RetryBackoffCoefficient,
		RetryExpirationTimeNanos:                unixNanoPtr(info.RetryExpirationTime),
		RetryNonRetryableErrors:                 info.RetryNonRetryableErrors,
		HasRetryPolicy:                          info.HasRetryPolicy,
		CronSchedule:                            info.CronSchedule,
		EventStoreVersion:                       info.EventStoreVersion,
		EventBranchToken:                        info.EventBranchToken,
		SignalCount:                             info.SignalCount,
		HistorySize:                             info.HistorySize,
		ClientLibraryVersion:                    info.ClientLibraryVersion,
		ClientFeatureVersion:                    info.ClientFeatureVersion,
		ClientImpl:                              info.ClientImpl,
		AutoResetPoints:                         info.AutoResetPoints,
		AutoResetPointsEncoding:                 info.AutoResetPointsEncoding,
		SearchAttributes:                        info.SearchAttributes,
		Memo:                                    info.Memo,
		VersionHistories:                        info.VersionHistories,
		VersionHistoriesEncoding:                info.VersionHistoriesEncoding,
	}
}

func workflowExecutionInfoFromThrift(info *sqlblobs.WorkflowExecutionInfo) *WorkflowExecutionInfo {
	if info == nil {
		return nil
	}
	return &WorkflowExecutionInfo{
		ParentDomainID:                     common.StringPtr(UUID(info.ParentDomainID).String()),
		ParentWorkflowID:                   info.ParentWorkflowID,
		ParentRunID:                        common.StringPtr(UUID(info.ParentRunID).String()),
		InitiatedID:                        info.InitiatedID,
		CompletionEventBatchID:             info.CompletionEventBatchID,
		CompletionEvent:                    info.CompletionEvent,
		CompletionEventEncoding:            info.CompletionEventEncoding,
		TaskList:                           info.TaskList,
		WorkflowTypeName:                   info.WorkflowTypeName,
		WorkflowTimeoutSeconds:             secondsToDuration(info.WorkflowTimeoutSeconds),
		DecisionTaskTimeoutSeconds:         secondsToDuration(info.DecisionTaskTimeoutSeconds),
		ExecutionContext:                   info.ExecutionContext,
		State:                              info.State,
		CloseStatus:                        info.CloseStatus,
		StartVersion:                       info.StartVersion,
		LastWriteEventID:                   info.LastWriteEventID,
		LastEventTaskID:                    info.LastEventTaskID,
		LastFirstEventID:                   info.LastFirstEventID,
		LastProcessedEvent:                 info.LastProcessedEvent,
		StartTime:                          timePtr(info.StartTimeNanos),
		LastUpdatedTime:                    timePtr(info.LastUpdatedTimeNanos),
		DecisionVersion:                    info.DecisionVersion,
		DecisionScheduleID:                 info.DecisionScheduleID,
		DecisionStartedID:                  info.DecisionStartedID,
		DecisionTimeout:                    secondsToDuration(info.DecisionTimeout),
		DecisionAttempt:                    info.DecisionAttempt,
		DecisionStartedTimestamp:           timePtr(info.DecisionStartedTimestampNanos),
		DecisionScheduledTimestamp:         timePtr(info.DecisionScheduledTimestampNanos),
		CancelRequested:                    info.CancelRequested,
		DecisionOriginalScheduledTimestamp: timePtr(info.DecisionOriginalScheduledTimestampNanos),
		CreateRequestID:                    info.CreateRequestID,
		DecisionRequestID:                  info.DecisionRequestID,
		CancelRequestID:                    info.CancelRequestID,
		StickyTaskList:                     info.StickyTaskList,
		StickyScheduleToStartTimeout:       secondsInt64ToDuration(info.StickyScheduleToStartTimeout),
		RetryAttempt:                       info.RetryAttempt,
		RetryInitialIntervalSeconds:        secondsToDuration(info.RetryInitialIntervalSeconds),
		RetryMaximumIntervalSeconds:        secondsToDuration(info.RetryMaximumIntervalSeconds),
		RetryMaximumAttempts:               info.RetryMaximumAttempts,
		RetryExpirationSeconds:             secondsToDuration(info.RetryExpirationSeconds),
		RetryBackoffCoefficient:            info.RetryBackoffCoefficient,
		RetryExpirationTime:                timePtr(info.RetryExpirationTimeNanos),
		RetryNonRetryableErrors:            info.RetryNonRetryableErrors,
		HasRetryPolicy:                     info.HasRetryPolicy,
		CronSchedule:                       info.CronSchedule,
		EventStoreVersion:                  info.EventStoreVersion,
		EventBranchToken:                   info.EventBranchToken,
		SignalCount:                        info.SignalCount,
		HistorySize:                        info.HistorySize,
		ClientLibraryVersion:               info.ClientLibraryVersion,
		ClientFeatureVersion:               info.ClientFeatureVersion,
		ClientImpl:                         info.ClientImpl,
		AutoResetPoints:                    info.AutoResetPoints,
		AutoResetPointsEncoding:            info.AutoResetPointsEncoding,
		SearchAttributes:                   info.SearchAttributes,
		Memo:                               info.Memo,
		VersionHistories:                   info.VersionHistories,
		VersionHistoriesEncoding:           info.VersionHistoriesEncoding,
	}
}

func activityInfoToThrift(info *ActivityInfo) *sqlblobs.ActivityInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ActivityInfo{
		Version:                       info.Version,
		ScheduledEventBatchID:         info.ScheduledEventBatchID,
		ScheduledEvent:                info.ScheduledEvent,
		ScheduledEventEncoding:        info.ScheduledEventEncoding,
		ScheduledTimeNanos:            unixNanoPtr(info.ScheduledTime),
		StartedID:                     info.StartedID,
		StartedEvent:                  info.StartedEvent,
		StartedEventEncoding:          info.StartedEventEncoding,
		StartedTimeNanos:              unixNanoPtr(info.StartedTime),
		ActivityID:                    info.ActivityID,
		RequestID:                     info.RequestID,
		ScheduleToStartTimeoutSeconds: durationToSeconds(info.ScheduleToStartTimeoutSeconds),
		ScheduleToCloseTimeoutSeconds: durationToSeconds(info.ScheduleToCloseTimeoutSeconds),
		StartToCloseTimeoutSeconds:    durationToSeconds(info.StartToCloseTimeoutSeconds),
		HeartbeatTimeoutSeconds:       durationToSeconds(info.HeartbeatTimeoutSeconds),
		CancelRequested:               info.CancelRequested,
		CancelRequestID:               info.CancelRequestID,
		TimerTaskStatus:               info.TimerTaskStatus,
		Attempt:                       info.Attempt,
		TaskList:                      info.TaskList,
		StartedIdentity:               info.StartedIdentity,
		HasRetryPolicy:                info.HasRetryPolicy,
		RetryInitialIntervalSeconds:   durationToSeconds(info.RetryInitialIntervalSeconds),
		RetryMaximumIntervalSeconds:   durationToSeconds(info.RetryMaximumIntervalSeconds),
		RetryMaximumAttempts:          info.RetryMaximumAttempts,
		RetryExpirationTimeNanos:      unixNanoPtr(info.RetryExpirationTime),
		RetryBackoffCoefficient:       info.RetryBackoffCoefficient,
		RetryNonRetryableErrors:       info.RetryNonRetryableErrors,
		RetryLastFailureReason:        info.RetryLastFailureReason,
		RetryLastWorkerIdentity:       info.RetryLastWorkerIdentity,
		RetryLastFailureDetails:       info.RetryLastFailureDetails,
	}
}

func activityInfoFromThrift(info *sqlblobs.ActivityInfo) *ActivityInfo {
	if info == nil {
		return nil
	}
	return &ActivityInfo{
		Version:                       info.Version,
		ScheduledEventBatchID:         info.ScheduledEventBatchID,
		ScheduledEvent:                info.ScheduledEvent,
		ScheduledEventEncoding:        info.ScheduledEventEncoding,
		ScheduledTime:                 timePtr(info.ScheduledTimeNanos),
		StartedID:                     info.StartedID,
		StartedEvent:                  info.StartedEvent,
		StartedEventEncoding:          info.StartedEventEncoding,
		StartedTime:                   timePtr(info.StartedTimeNanos),
		ActivityID:                    info.ActivityID,
		RequestID:                     info.RequestID,
		ScheduleToStartTimeoutSeconds: secondsToDuration(info.ScheduleToStartTimeoutSeconds),
		ScheduleToCloseTimeoutSeconds: secondsToDuration(info.ScheduleToCloseTimeoutSeconds),
		StartToCloseTimeoutSeconds:    secondsToDuration(info.StartToCloseTimeoutSeconds),
		HeartbeatTimeoutSeconds:       secondsToDuration(info.HeartbeatTimeoutSeconds),
		CancelRequested:               info.CancelRequested,
		CancelRequestID:               info.CancelRequestID,
		TimerTaskStatus:               info.TimerTaskStatus,
		Attempt:                       info.Attempt,
		TaskList:                      info.TaskList,
		StartedIdentity:               info.StartedIdentity,
		HasRetryPolicy:                info.HasRetryPolicy,
		RetryInitialIntervalSeconds:   secondsToDuration(info.RetryInitialIntervalSeconds),
		RetryMaximumIntervalSeconds:   secondsToDuration(info.RetryMaximumIntervalSeconds),
		RetryMaximumAttempts:          info.RetryMaximumAttempts,
		RetryExpirationTime:           timePtr(info.RetryExpirationTimeNanos),
		RetryBackoffCoefficient:       info.RetryBackoffCoefficient,
		RetryNonRetryableErrors:       info.RetryNonRetryableErrors,
		RetryLastFailureReason:        info.RetryLastFailureReason,
		RetryLastWorkerIdentity:       info.RetryLastWorkerIdentity,
		RetryLastFailureDetails:       info.RetryLastFailureDetails,
	}
}

func childExecutionInfoToThrift(info *ChildExecutionInfo) *sqlblobs.ChildExecutionInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ChildExecutionInfo{
		Version:                info.Version,
		InitiatedEventBatchID:  info.InitiatedEventBatchID,
		StartedID:              info.StartedID,
		InitiatedEvent:         info.InitiatedEvent,
		InitiatedEventEncoding: info.InitiatedEventEncoding,
		StartedWorkflowID:      info.StartedWorkflowID,
		StartedRunID:           MustParsePtrUUID(info.StartedRunID),
		StartedEvent:           info.StartedEvent,
		StartedEventEncoding:   info.StartedEventEncoding,
		CreateRequestID:        info.CreateRequestID,
		DomainName:             info.DomainName,
		WorkflowTypeName:       info.WorkflowTypeName,
		ParentClosePolicy:      info.ParentClosePolicy,
	}
}

func childExecutionInfoFromThrift(info *sqlblobs.ChildExecutionInfo) *ChildExecutionInfo {
	if info == nil {
		return nil
	}
	return &ChildExecutionInfo{
		Version:                info.Version,
		InitiatedEventBatchID:  info.InitiatedEventBatchID,
		StartedID:              info.StartedID,
		InitiatedEvent:         info.InitiatedEvent,
		InitiatedEventEncoding: info.InitiatedEventEncoding,
		StartedWorkflowID:      info.StartedWorkflowID,
		StartedRunID:           common.StringPtr(UUID(info.StartedRunID).String()),
		StartedEvent:           info.StartedEvent,
		StartedEventEncoding:   info.StartedEventEncoding,
		CreateRequestID:        info.CreateRequestID,
		DomainName:             info.DomainName,
		WorkflowTypeName:       info.WorkflowTypeName,
		ParentClosePolicy:      info.ParentClosePolicy,
	}
}

func signalInfoToThrift(info *SignalInfo) *sqlblobs.SignalInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.SignalInfo{
		Version:               info.Version,
		InitiatedEventBatchID: info.InitiatedEventBatchID,
		RequestID:             info.RequestID,
		Name:                  info.Name,
		Input:                 info.Input,
		Control:               info.Control,
	}
}

func signalInfoFromThrift(info *sqlblobs.SignalInfo) *SignalInfo {
	if info == nil {
		return nil
	}
	return &SignalInfo{
		Version:               info.Version,
		InitiatedEventBatchID: info.InitiatedEventBatchID,
		RequestID:             info.RequestID,
		Name:                  info.Name,
		Input:                 info.Input,
		Control:               info.Control,
	}
}

func requestCancelInfoToThrift(info *RequestCancelInfo) *sqlblobs.RequestCancelInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.RequestCancelInfo{
		Version:               info.Version,
		InitiatedEventBatchID: info.InitiatedEventBatchID,
		CancelRequestID:       info.CancelRequestID,
	}
}

func requestCancelInfoFromThrift(info *sqlblobs.RequestCancelInfo) *RequestCancelInfo {
	if info == nil {
		return nil
	}
	return &RequestCancelInfo{
		Version:               info.Version,
		InitiatedEventBatchID: info.InitiatedEventBatchID,
		CancelRequestID:       info.CancelRequestID,
	}
}

func timerInfoToThrift(info *TimerInfo) *sqlblobs.TimerInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TimerInfo{
		Version:         info.Version,
		StartedID:       info.StartedID,
		ExpiryTimeNanos: unixNanoPtr(info.ExpiryTime),
		TaskID:          info.TaskID,
	}
}

func timerInfoFromThrift(info *sqlblobs.TimerInfo) *TimerInfo {
	if info == nil {
		return nil
	}
	return &TimerInfo{
		Version:    info.Version,
		StartedID:  info.StartedID,
		ExpiryTime: timePtr(info.ExpiryTimeNanos),
		TaskID:     info.TaskID,
	}
}

func taskInfoToThrift(info *TaskInfo) *sqlblobs.TaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TaskInfo{
		WorkflowID:       info.WorkflowID,
		RunID:            MustParsePtrUUID(info.RunID),
		ScheduleID:       info.ScheduleID,
		ExpiryTimeNanos:  unixNanoPtr(info.ExpiryTime),
		CreatedTimeNanos: unixNanoPtr(info.CreatedTime),
	}
}

func taskInfoFromThrift(info *sqlblobs.TaskInfo) *TaskInfo {
	if info == nil {
		return nil
	}
	return &TaskInfo{
		WorkflowID:  info.WorkflowID,
		RunID:       common.StringPtr(UUID(info.RunID).String()),
		ScheduleID:  info.ScheduleID,
		ExpiryTime:  timePtr(info.ExpiryTimeNanos),
		CreatedTime: timePtr(info.CreatedTimeNanos),
	}
}

func taskListInfoToThrift(info *TaskListInfo) *sqlblobs.TaskListInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TaskListInfo{
		Kind:             info.Kind,
		AckLevel:         info.AckLevel,
		ExpiryTimeNanos:  unixNanoPtr(info.ExpiryTime),
		LastUpdatedNanos: unixNanoPtr(info.LastUpdated),
	}
}

func taskListInfoFromThrift(info *sqlblobs.TaskListInfo) *TaskListInfo {
	if info == nil {
		return nil
	}
	return &TaskListInfo{
		Kind:        info.Kind,
		AckLevel:    info.AckLevel,
		ExpiryTime:  timePtr(info.ExpiryTimeNanos),
		LastUpdated: timePtr(info.LastUpdatedNanos),
	}
}

func transferTaskInfoToThrift(info *TransferTaskInfo) *sqlblobs.TransferTaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TransferTaskInfo{
		DomainID:                 MustParsePtrUUID(info.DomainID),
		WorkflowID:               info.WorkflowID,
		RunID:                    MustParsePtrUUID(info.RunID),
		TaskType:                 info.TaskType,
		TargetDomainID:           MustParsePtrUUID(info.TargetDomainID),
		TargetWorkflowID:         info.TargetWorkflowID,
		TargetRunID:              MustParsePtrUUID(info.TargetRunID),
		TaskList:                 info.TaskList,
		TargetChildWorkflowOnly:  info.TargetChildWorkflowOnly,
		ScheduleID:               info.ScheduleID,
		Version:                  info.Version,
		VisibilityTimestampNanos: unixNanoPtr(info.VisibilityTimestamp),
	}
}

func transferTaskInfoFromThrift(info *sqlblobs.TransferTaskInfo) *TransferTaskInfo {
	if info == nil {
		return nil
	}
	return &TransferTaskInfo{
		DomainID:                common.StringPtr(UUID(info.DomainID).String()),
		WorkflowID:              info.WorkflowID,
		RunID:                   common.StringPtr(UUID(info.RunID).String()),
		TaskType:                info.TaskType,
		TargetDomainID:          common.StringPtr(UUID(info.TargetDomainID).String()),
		TargetWorkflowID:        info.TargetWorkflowID,
		TargetRunID:             common.StringPtr(UUID(info.TargetRunID).String()),
		TaskList:                info.TaskList,
		TargetChildWorkflowOnly: info.TargetChildWorkflowOnly,
		ScheduleID:              info.ScheduleID,
		Version:                 info.Version,
		VisibilityTimestamp:     timePtr(info.VisibilityTimestampNanos),
	}
}

func timerTaskInfoToThrift(info *TimerTaskInfo) *sqlblobs.TimerTaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.TimerTaskInfo{
		DomainID:        MustParsePtrUUID(info.DomainID),
		WorkflowID:      info.WorkflowID,
		RunID:           MustParsePtrUUID(info.RunID),
		TaskType:        info.TaskType,
		TimeoutType:     info.TimeoutType,
		Version:         info.Version,
		ScheduleAttempt: info.ScheduleAttempt,
		EventID:         info.EventID,
	}
}

func timerTaskInfoFromThrift(info *sqlblobs.TimerTaskInfo) *TimerTaskInfo {
	if info == nil {
		return nil
	}
	return &TimerTaskInfo{
		DomainID:        common.StringPtr(UUID(info.DomainID).String()),
		WorkflowID:      info.WorkflowID,
		RunID:           common.StringPtr(UUID(info.RunID).String()),
		TaskType:        info.TaskType,
		TimeoutType:     info.TimeoutType,
		Version:         info.Version,
		ScheduleAttempt: info.ScheduleAttempt,
		EventID:         info.EventID,
	}
}

func replicationTaskInfoToThrift(info *ReplicationTaskInfo) *sqlblobs.ReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &sqlblobs.ReplicationTaskInfo{
		DomainID:                MustParsePtrUUID(info.DomainID),
		WorkflowID:              info.WorkflowID,
		RunID:                   MustParsePtrUUID(info.RunID),
		TaskType:                info.TaskType,
		Version:                 info.Version,
		FirstEventID:            info.FirstEventID,
		NextEventID:             info.NextEventID,
		ScheduledID:             info.ScheduledID,
		EventStoreVersion:       info.EventStoreVersion,
		NewRunEventStoreVersion: info.NewRunEventStoreVersion,
		BranchToken:             info.BranchToken,
		NewRunBranchToken:       info.NewRunBranchToken,
		CreationTime:            unixNanoPtr(info.CreationTime),
	}
}

func replicationTaskInfoFromThrift(info *sqlblobs.ReplicationTaskInfo) *ReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &ReplicationTaskInfo{
		DomainID:                common.StringPtr(UUID(info.DomainID).String()),
		WorkflowID:              info.WorkflowID,
		RunID:                   common.StringPtr(UUID(info.RunID).String()),
		TaskType:                info.TaskType,
		Version:                 info.Version,
		FirstEventID:            info.FirstEventID,
		NextEventID:             info.NextEventID,
		ScheduledID:             info.ScheduledID,
		EventStoreVersion:       info.EventStoreVersion,
		NewRunEventStoreVersion: info.NewRunEventStoreVersion,
		BranchToken:             info.BranchToken,
		NewRunBranchToken:       info.NewRunBranchToken,
		CreationTime:            timePtr(info.CreationTime),
	}
}

func unixNanoPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	return common.Int64Ptr(t.UnixNano())
}

func timePtr(t *int64) *time.Time {
	if t == nil {
		return nil
	}
	return common.TimePtr(time.Unix(0, *t))
}

func durationToSeconds(t *time.Duration) *int32 {
	if t == nil {
		return nil
	}
	return common.Int32Ptr(int32(common.DurationToSeconds(*t)))
}

func durationToSecondsInt64(t *time.Duration) *int64 {
	if t == nil {
		return nil
	}
	return common.Int64Ptr(common.DurationToSeconds(*t))
}

func secondsInt64ToDuration(t *int64) *time.Duration {
	if t == nil {
		return nil
	}
	return common.DurationPtr(common.SecondsToDuration(*t))
}

func secondsToDuration(t *int32) *time.Duration {
	if t == nil {
		return nil
	}
	return common.DurationPtr(common.SecondsToDuration(int64(*t)))
}

func durationToDays(t *time.Duration) *int16 {
	if t == nil {
		return nil
	}
	return common.Int16Ptr(int16(common.DurationToDays(*t)))
}

func daysToDuration(t *int16) *time.Duration {
	if t == nil {
		return nil
	}
	return common.DurationPtr(common.DaysToDuration(int32(*t)))
}
