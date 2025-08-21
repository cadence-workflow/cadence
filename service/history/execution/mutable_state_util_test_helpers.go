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

package execution

import (
	"encoding/json"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// CreatePersistenceMutableState creates a persistence mutable state based on the its in-memory version
func CreatePersistenceMutableState(t *testing.T, ms MutableState) *persistence.WorkflowMutableState {
	builder := ms.(*mutableStateBuilder)
	builder.FlushBufferedEvents() //nolint:errcheck
	info := CopyWorkflowExecutionInfo(t, builder.GetExecutionInfo())
	stats := &persistence.ExecutionStats{}
	activityInfos := make(map[int64]*persistence.ActivityInfo)
	for id, info := range builder.GetPendingActivityInfos() {
		activityInfos[id] = CopyActivityInfo(t, info)
	}
	timerInfos := make(map[string]*persistence.TimerInfo)
	for id, info := range builder.GetPendingTimerInfos() {
		timerInfos[id] = CopyTimerInfo(t, info)
	}
	cancellationInfos := make(map[int64]*persistence.RequestCancelInfo)
	for id, info := range builder.GetPendingRequestCancelExternalInfos() {
		cancellationInfos[id] = CopyCancellationInfo(t, info)
	}
	signalInfos := make(map[int64]*persistence.SignalInfo)
	for id, info := range builder.GetPendingSignalExternalInfos() {
		signalInfos[id] = CopySignalInfo(t, info)
	}
	childInfos := make(map[int64]*persistence.ChildExecutionInfo)
	for id, info := range builder.GetPendingChildExecutionInfos() {
		childInfos[id] = CopyChildInfo(t, info)
	}

	builder.FlushBufferedEvents() //nolint:errcheck
	var bufferedEvents []*types.HistoryEvent
	if len(builder.bufferedEvents) > 0 {
		bufferedEvents = append(bufferedEvents, builder.bufferedEvents...)
	}
	if len(builder.updateBufferedEvents) > 0 {
		bufferedEvents = append(bufferedEvents, builder.updateBufferedEvents...)
	}

	var versionHistories *persistence.VersionHistories
	if ms.GetVersionHistories() != nil {
		versionHistories = ms.GetVersionHistories().Duplicate()
	}
	return &persistence.WorkflowMutableState{
		ExecutionInfo:       info,
		ExecutionStats:      stats,
		ActivityInfos:       activityInfos,
		TimerInfos:          timerInfos,
		BufferedEvents:      bufferedEvents,
		SignalInfos:         signalInfos,
		RequestCancelInfos:  cancellationInfos,
		ChildExecutionInfos: childInfos,
		VersionHistories:    versionHistories,
	}
}

// CopyWorkflowExecutionInfo copies WorkflowExecutionInfo
func CopyWorkflowExecutionInfo(t *testing.T, sourceInfo *persistence.WorkflowExecutionInfo) *persistence.WorkflowExecutionInfo {
	return &persistence.WorkflowExecutionInfo{
		DomainID:                           sourceInfo.DomainID,
		WorkflowID:                         sourceInfo.WorkflowID,
		RunID:                              sourceInfo.RunID,
		FirstExecutionRunID:                sourceInfo.FirstExecutionRunID,
		ParentDomainID:                     sourceInfo.ParentDomainID,
		ParentWorkflowID:                   sourceInfo.ParentWorkflowID,
		ParentRunID:                        sourceInfo.ParentRunID,
		IsCron:                             sourceInfo.IsCron,
		InitiatedID:                        sourceInfo.InitiatedID,
		CompletionEventBatchID:             sourceInfo.CompletionEventBatchID,
		CompletionEvent:                    sourceInfo.CompletionEvent,
		TaskList:                           sourceInfo.TaskList,
		TaskListKind:                       sourceInfo.TaskListKind,
		StickyTaskList:                     sourceInfo.StickyTaskList,
		StickyScheduleToStartTimeout:       sourceInfo.StickyScheduleToStartTimeout,
		WorkflowTypeName:                   sourceInfo.WorkflowTypeName,
		WorkflowTimeout:                    sourceInfo.WorkflowTimeout,
		DecisionStartToCloseTimeout:        sourceInfo.DecisionStartToCloseTimeout,
		ExecutionContext:                   sourceInfo.ExecutionContext,
		State:                              sourceInfo.State,
		CloseStatus:                        sourceInfo.CloseStatus,
		LastFirstEventID:                   sourceInfo.LastFirstEventID,
		LastEventTaskID:                    sourceInfo.LastEventTaskID,
		NextEventID:                        sourceInfo.NextEventID,
		LastProcessedEvent:                 sourceInfo.LastProcessedEvent,
		StartTimestamp:                     sourceInfo.StartTimestamp,
		LastUpdatedTimestamp:               sourceInfo.LastUpdatedTimestamp,
		CreateRequestID:                    sourceInfo.CreateRequestID,
		SignalCount:                        sourceInfo.SignalCount,
		DecisionVersion:                    sourceInfo.DecisionVersion,
		DecisionScheduleID:                 sourceInfo.DecisionScheduleID,
		DecisionStartedID:                  sourceInfo.DecisionStartedID,
		DecisionRequestID:                  sourceInfo.DecisionRequestID,
		DecisionTimeout:                    sourceInfo.DecisionTimeout,
		DecisionAttempt:                    sourceInfo.DecisionAttempt,
		DecisionScheduledTimestamp:         sourceInfo.DecisionScheduledTimestamp,
		DecisionStartedTimestamp:           sourceInfo.DecisionStartedTimestamp,
		DecisionOriginalScheduledTimestamp: sourceInfo.DecisionOriginalScheduledTimestamp,
		CancelRequested:                    sourceInfo.CancelRequested,
		CancelRequestID:                    sourceInfo.CancelRequestID,
		CronSchedule:                       sourceInfo.CronSchedule,
		ClientLibraryVersion:               sourceInfo.ClientLibraryVersion,
		ClientFeatureVersion:               sourceInfo.ClientFeatureVersion,
		ClientImpl:                         sourceInfo.ClientImpl,
		AutoResetPoints:                    sourceInfo.AutoResetPoints,
		Memo:                               sourceInfo.Memo,
		SearchAttributes:                   sourceInfo.SearchAttributes,
		PartitionConfig:                    sourceInfo.PartitionConfig,
		Attempt:                            sourceInfo.Attempt,
		HasRetryPolicy:                     sourceInfo.HasRetryPolicy,
		InitialInterval:                    sourceInfo.InitialInterval,
		BackoffCoefficient:                 sourceInfo.BackoffCoefficient,
		MaximumInterval:                    sourceInfo.MaximumInterval,
		ExpirationTime:                     sourceInfo.ExpirationTime,
		MaximumAttempts:                    sourceInfo.MaximumAttempts,
		NonRetriableErrors:                 sourceInfo.NonRetriableErrors,
		BranchToken:                        sourceInfo.BranchToken,
		ExpirationSeconds:                  sourceInfo.ExpirationSeconds,
		CronOverlapPolicy:                  sourceInfo.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:       sourceInfo.ActiveClusterSelectionPolicy,
	}
}

// CopyActivityInfo copies ActivityInfo
func CopyActivityInfo(t *testing.T, sourceInfo *persistence.ActivityInfo) *persistence.ActivityInfo {
	details := slices.Clone(sourceInfo.Details)

	return &persistence.ActivityInfo{
		Version:                  sourceInfo.Version,
		ScheduleID:               sourceInfo.ScheduleID,
		ScheduledEventBatchID:    sourceInfo.ScheduledEventBatchID,
		ScheduledEvent:           deepCopyHistoryEvent(t, sourceInfo.ScheduledEvent),
		StartedID:                sourceInfo.StartedID,
		StartedEvent:             deepCopyHistoryEvent(t, sourceInfo.StartedEvent),
		ActivityID:               sourceInfo.ActivityID,
		RequestID:                sourceInfo.RequestID,
		Details:                  details,
		ScheduledTime:            sourceInfo.ScheduledTime,
		StartedTime:              sourceInfo.StartedTime,
		ScheduleToStartTimeout:   sourceInfo.ScheduleToStartTimeout,
		ScheduleToCloseTimeout:   sourceInfo.ScheduleToCloseTimeout,
		StartToCloseTimeout:      sourceInfo.StartToCloseTimeout,
		HeartbeatTimeout:         sourceInfo.HeartbeatTimeout,
		LastHeartBeatUpdatedTime: sourceInfo.LastHeartBeatUpdatedTime,
		CancelRequested:          sourceInfo.CancelRequested,
		CancelRequestID:          sourceInfo.CancelRequestID,
		TimerTaskStatus:          sourceInfo.TimerTaskStatus,
		Attempt:                  sourceInfo.Attempt,
		DomainID:                 sourceInfo.DomainID,
		StartedIdentity:          sourceInfo.StartedIdentity,
		TaskList:                 sourceInfo.TaskList,
		TaskListKind:             sourceInfo.TaskListKind,
		HasRetryPolicy:           sourceInfo.HasRetryPolicy,
		InitialInterval:          sourceInfo.InitialInterval,
		BackoffCoefficient:       sourceInfo.BackoffCoefficient,
		MaximumInterval:          sourceInfo.MaximumInterval,
		ExpirationTime:           sourceInfo.ExpirationTime,
		MaximumAttempts:          sourceInfo.MaximumAttempts,
		NonRetriableErrors:       sourceInfo.NonRetriableErrors,
		LastFailureReason:        sourceInfo.LastFailureReason,
		LastWorkerIdentity:       sourceInfo.LastWorkerIdentity,
		LastFailureDetails:       sourceInfo.LastFailureDetails,
		// Not written to database - This is used only for deduping heartbeat timer creation
		LastHeartbeatTimeoutVisibilityInSeconds: sourceInfo.LastHeartbeatTimeoutVisibilityInSeconds,
	}
}

// CopyTimerInfo copies TimerInfo
func CopyTimerInfo(t *testing.T, sourceInfo *persistence.TimerInfo) *persistence.TimerInfo {
	return &persistence.TimerInfo{
		Version:    sourceInfo.Version,
		TimerID:    sourceInfo.TimerID,
		StartedID:  sourceInfo.StartedID,
		ExpiryTime: sourceInfo.ExpiryTime,
		TaskStatus: sourceInfo.TaskStatus,
	}
}

// CopyCancellationInfo copies RequestCancelInfo
func CopyCancellationInfo(t *testing.T, sourceInfo *persistence.RequestCancelInfo) *persistence.RequestCancelInfo {
	return &persistence.RequestCancelInfo{
		Version:               sourceInfo.Version,
		InitiatedID:           sourceInfo.InitiatedID,
		InitiatedEventBatchID: sourceInfo.InitiatedEventBatchID,
		CancelRequestID:       sourceInfo.CancelRequestID,
	}
}

// CopySignalInfo copies SignalInfo
func CopySignalInfo(t *testing.T, sourceInfo *persistence.SignalInfo) *persistence.SignalInfo {
	return &persistence.SignalInfo{
		Version:               sourceInfo.Version,
		InitiatedEventBatchID: sourceInfo.InitiatedEventBatchID,
		InitiatedID:           sourceInfo.InitiatedID,
		SignalRequestID:       sourceInfo.SignalRequestID,
		SignalName:            sourceInfo.SignalName,
		Input:                 slices.Clone(sourceInfo.Input),
		Control:               slices.Clone(sourceInfo.Control),
	}
}

// CopyChildInfo copies ChildExecutionInfo
func CopyChildInfo(t *testing.T, sourceInfo *persistence.ChildExecutionInfo) *persistence.ChildExecutionInfo {
	return &persistence.ChildExecutionInfo{
		Version:               sourceInfo.Version,
		InitiatedID:           sourceInfo.InitiatedID,
		InitiatedEventBatchID: sourceInfo.InitiatedEventBatchID,
		StartedID:             sourceInfo.StartedID,
		StartedWorkflowID:     sourceInfo.StartedWorkflowID,
		StartedRunID:          sourceInfo.StartedRunID,
		CreateRequestID:       sourceInfo.CreateRequestID,
		DomainID:              sourceInfo.DomainID,
		DomainNameDEPRECATED:  sourceInfo.DomainNameDEPRECATED,
		WorkflowTypeName:      sourceInfo.WorkflowTypeName,
		ParentClosePolicy:     sourceInfo.ParentClosePolicy,
		InitiatedEvent:        deepCopyHistoryEvent(t, sourceInfo.InitiatedEvent),
		StartedEvent:          deepCopyHistoryEvent(t, sourceInfo.StartedEvent),
	}
}

func deepCopyHistoryEvent(t *testing.T, e *types.HistoryEvent) *types.HistoryEvent {
	if e == nil {
		return nil
	}
	bytes, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	var copy types.HistoryEvent
	err = json.Unmarshal(bytes, &copy)
	if err != nil {
		panic(err)
	}
	return &copy
}

// GetChildExecutionDomainEntry get domain entry for the child workflow
// NOTE: DomainName in ChildExecutionInfo is being deprecated, and
// we should always use DomainID field instead.
// this function exists for backward compatibility reason
func GetChildExecutionDomainEntry(
	t *testing.T,
	childInfo *persistence.ChildExecutionInfo,
	domainCache cache.DomainCache,
	parentDomainEntry *cache.DomainCacheEntry,
) (*cache.DomainCacheEntry, error) {
	if childInfo.DomainID != "" {
		return domainCache.GetDomainByID(childInfo.DomainID)
	}

	if childInfo.DomainNameDEPRECATED != "" {
		return domainCache.GetDomain(childInfo.DomainNameDEPRECATED)
	}

	return parentDomainEntry, nil
}
