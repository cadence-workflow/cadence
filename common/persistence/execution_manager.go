// Copyright (c) 2017-2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package persistence

import (
	"context"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

type (
	// executionManagerImpl implements ExecutionManager based on ExecutionStore, statsComputer and PayloadSerializer
	executionManagerImpl struct {
		serializer    PayloadSerializer
		persistence   ExecutionStore
		statsComputer statsComputer
		logger        log.Logger
		timeSrc       clock.TimeSource
	}
)

var _ ExecutionManager = (*executionManagerImpl)(nil)

// NewExecutionManagerImpl returns new ExecutionManager
func NewExecutionManagerImpl(
	persistence ExecutionStore,
	logger log.Logger,
	serializer PayloadSerializer,
) ExecutionManager {
	return &executionManagerImpl{
		serializer:    serializer,
		persistence:   persistence,
		statsComputer: statsComputer{},
		logger:        logger,
		timeSrc:       clock.NewRealTimeSource(),
	}
}

func (m *executionManagerImpl) GetName() string {
	return m.persistence.GetName()
}

func (m *executionManagerImpl) GetShardID() int {
	return m.persistence.GetShardID()
}

// The below three APIs are related to serialization/deserialization

func (m *executionManagerImpl) GetWorkflowExecution(
	ctx context.Context,
	request *GetWorkflowExecutionRequest,
) (*GetWorkflowExecutionResponse, error) {

	internalRequest := &InternalGetWorkflowExecutionRequest{
		DomainID:  request.DomainID,
		Execution: request.Execution,
		RangeID:   request.RangeID,
	}
	response, err := m.persistence.GetWorkflowExecution(ctx, internalRequest)
	if err != nil {
		return nil, err
	}
	newResponse := &GetWorkflowExecutionResponse{
		State: &WorkflowMutableState{
			TimerInfos:         response.State.TimerInfos,
			RequestCancelInfos: response.State.RequestCancelInfos,
			SignalInfos:        response.State.SignalInfos,
			SignalRequestedIDs: response.State.SignalRequestedIDs,
			ReplicationState:   response.State.ReplicationState, // TODO: remove this after all 2DC workflows complete
			Checksum:           response.State.Checksum,
		},
	}

	newResponse.State.ActivityInfos, err = m.DeserializeActivityInfos(response.State.ActivityInfos)
	if err != nil {
		return nil, err
	}
	newResponse.State.ChildExecutionInfos, err = m.DeserializeChildExecutionInfos(response.State.ChildExecutionInfos)
	if err != nil {
		return nil, err
	}
	newResponse.State.BufferedEvents, err = m.DeserializeBufferedEvents(response.State.BufferedEvents)
	if err != nil {
		return nil, err
	}
	newResponse.State.ExecutionInfo, newResponse.State.ExecutionStats, err = m.DeserializeExecutionInfo(response.State.ExecutionInfo)
	if err != nil {
		return nil, err
	}
	versionHistories, err := m.DeserializeVersionHistories(response.State.VersionHistories)
	if err != nil {
		return nil, err
	}
	newResponse.State.VersionHistories = versionHistories
	newResponse.MutableStateStats = m.statsComputer.computeMutableStateStats(response)

	if len(newResponse.State.Checksum.Value) == 0 {
		newResponse.State.Checksum, err = m.serializer.DeserializeChecksum(response.State.ChecksumData)
		if err != nil {
			return nil, err
		}
	}

	return newResponse, nil
}

func (m *executionManagerImpl) DeserializeExecutionInfo(
	info *InternalWorkflowExecutionInfo,
) (*WorkflowExecutionInfo, *ExecutionStats, error) {

	completionEvent, err := m.serializer.DeserializeEvent(info.CompletionEvent)
	if err != nil {
		return nil, nil, err
	}

	autoResetPoints, err := m.serializer.DeserializeResetPoints(info.AutoResetPoints)
	if err != nil {
		return nil, nil, err
	}

	activeClusterSelectionPolicy, err := m.serializer.DeserializeActiveClusterSelectionPolicy(info.ActiveClusterSelectionPolicy)
	if err != nil {
		return nil, nil, err
	}

	newInfo := &WorkflowExecutionInfo{
		CompletionEvent: completionEvent,

		DomainID:                           info.DomainID,
		WorkflowID:                         info.WorkflowID,
		RunID:                              info.RunID,
		FirstExecutionRunID:                info.FirstExecutionRunID,
		ParentDomainID:                     info.ParentDomainID,
		ParentWorkflowID:                   info.ParentWorkflowID,
		ParentRunID:                        info.ParentRunID,
		InitiatedID:                        info.InitiatedID,
		CompletionEventBatchID:             info.CompletionEventBatchID,
		TaskList:                           info.TaskList,
		TaskListKind:                       info.TaskListKind,
		IsCron:                             len(info.CronSchedule) > 0,
		WorkflowTypeName:                   info.WorkflowTypeName,
		WorkflowTimeout:                    int32(info.WorkflowTimeout.Seconds()),
		DecisionStartToCloseTimeout:        int32(info.DecisionStartToCloseTimeout.Seconds()),
		ExecutionContext:                   info.ExecutionContext,
		State:                              info.State,
		CloseStatus:                        info.CloseStatus,
		LastFirstEventID:                   info.LastFirstEventID,
		LastEventTaskID:                    info.LastEventTaskID,
		NextEventID:                        info.NextEventID,
		LastProcessedEvent:                 info.LastProcessedEvent,
		StartTimestamp:                     info.StartTimestamp,
		LastUpdatedTimestamp:               info.LastUpdatedTimestamp,
		CreateRequestID:                    info.CreateRequestID,
		SignalCount:                        info.SignalCount,
		DecisionVersion:                    info.DecisionVersion,
		DecisionScheduleID:                 info.DecisionScheduleID,
		DecisionStartedID:                  info.DecisionStartedID,
		DecisionRequestID:                  info.DecisionRequestID,
		DecisionTimeout:                    int32(info.DecisionTimeout.Seconds()),
		DecisionAttempt:                    info.DecisionAttempt,
		DecisionStartedTimestamp:           info.DecisionStartedTimestamp.UnixNano(),
		DecisionScheduledTimestamp:         info.DecisionScheduledTimestamp.UnixNano(),
		DecisionOriginalScheduledTimestamp: info.DecisionOriginalScheduledTimestamp.UnixNano(),
		CancelRequested:                    info.CancelRequested,
		CancelRequestID:                    info.CancelRequestID,
		StickyTaskList:                     info.StickyTaskList,
		StickyScheduleToStartTimeout:       int32(info.StickyScheduleToStartTimeout.Seconds()),
		ClientLibraryVersion:               info.ClientLibraryVersion,
		ClientFeatureVersion:               info.ClientFeatureVersion,
		ClientImpl:                         info.ClientImpl,
		Attempt:                            info.Attempt,
		HasRetryPolicy:                     info.HasRetryPolicy,
		InitialInterval:                    int32(info.InitialInterval.Seconds()),
		BackoffCoefficient:                 info.BackoffCoefficient,
		MaximumInterval:                    int32(info.MaximumInterval.Seconds()),
		ExpirationTime:                     info.ExpirationTime,
		MaximumAttempts:                    info.MaximumAttempts,
		NonRetriableErrors:                 info.NonRetriableErrors,
		BranchToken:                        info.BranchToken,
		CronSchedule:                       info.CronSchedule,
		CronOverlapPolicy:                  info.CronOverlapPolicy,
		ExpirationSeconds:                  int32(info.ExpirationInterval.Seconds()),
		AutoResetPoints:                    autoResetPoints,
		SearchAttributes:                   info.SearchAttributes,
		Memo:                               info.Memo,
		PartitionConfig:                    info.PartitionConfig,
		ActiveClusterSelectionPolicy:       activeClusterSelectionPolicy,
	}
	newStats := &ExecutionStats{
		HistorySize: info.HistorySize,
	}
	return newInfo, newStats, nil
}

func (m *executionManagerImpl) DeserializeBufferedEvents(
	blobs []*DataBlob,
) ([]*types.HistoryEvent, error) {

	events := make([]*types.HistoryEvent, 0)
	for _, b := range blobs {
		history, err := m.serializer.DeserializeBatchEvents(b)
		if err != nil {
			return nil, err
		}
		events = append(events, history...)
	}
	return events, nil
}

func (m *executionManagerImpl) DeserializeChildExecutionInfos(
	infos map[int64]*InternalChildExecutionInfo,
) (map[int64]*ChildExecutionInfo, error) {

	newInfos := make(map[int64]*ChildExecutionInfo)
	for k, v := range infos {
		initiatedEvent, err := m.serializer.DeserializeEvent(v.InitiatedEvent)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.DeserializeEvent(v.StartedEvent)
		if err != nil {
			return nil, err
		}
		c := &ChildExecutionInfo{
			InitiatedEvent: initiatedEvent,
			StartedEvent:   startedEvent,

			Version:               v.Version,
			InitiatedID:           v.InitiatedID,
			InitiatedEventBatchID: v.InitiatedEventBatchID,
			StartedID:             v.StartedID,
			StartedWorkflowID:     v.StartedWorkflowID,
			StartedRunID:          v.StartedRunID,
			CreateRequestID:       v.CreateRequestID,
			DomainID:              v.DomainID,
			DomainNameDEPRECATED:  v.DomainNameDEPRECATED,
			WorkflowTypeName:      v.WorkflowTypeName,
			ParentClosePolicy:     v.ParentClosePolicy,
		}

		// Needed for backward compatibility reason.
		// ChildWorkflowExecutionStartedEvent was only used by transfer queue processing of StartChildWorkflow.
		// Updated the code to instead directly read WorkflowId and RunId from mutable state
		// Existing mutable state won't have those values set so instead use started event to set StartedWorkflowID and
		// StartedRunID on the mutable state before passing it to application
		if startedEvent != nil && startedEvent.ChildWorkflowExecutionStartedEventAttributes != nil &&
			startedEvent.ChildWorkflowExecutionStartedEventAttributes.WorkflowExecution != nil {
			startedExecution := startedEvent.ChildWorkflowExecutionStartedEventAttributes.WorkflowExecution
			c.StartedWorkflowID = startedExecution.GetWorkflowID()
			c.StartedRunID = startedExecution.GetRunID()
		}
		newInfos[k] = c
	}
	return newInfos, nil
}

func (m *executionManagerImpl) DeserializeActivityInfos(
	infos map[int64]*InternalActivityInfo,
) (map[int64]*ActivityInfo, error) {

	newInfos := make(map[int64]*ActivityInfo)
	for k, v := range infos {
		scheduledEvent, err := m.serializer.DeserializeEvent(v.ScheduledEvent)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.DeserializeEvent(v.StartedEvent)
		if err != nil {
			return nil, err
		}
		a := &ActivityInfo{
			ScheduledEvent: scheduledEvent,
			StartedEvent:   startedEvent,

			Version:                                 v.Version,
			ScheduleID:                              v.ScheduleID,
			ScheduledEventBatchID:                   v.ScheduledEventBatchID,
			ScheduledTime:                           v.ScheduledTime,
			StartedID:                               v.StartedID,
			StartedTime:                             v.StartedTime,
			ActivityID:                              v.ActivityID,
			RequestID:                               v.RequestID,
			Details:                                 v.Details,
			ScheduleToStartTimeout:                  int32(v.ScheduleToStartTimeout.Seconds()),
			ScheduleToCloseTimeout:                  int32(v.ScheduleToCloseTimeout.Seconds()),
			StartToCloseTimeout:                     int32(v.StartToCloseTimeout.Seconds()),
			HeartbeatTimeout:                        int32(v.HeartbeatTimeout.Seconds()),
			CancelRequested:                         v.CancelRequested,
			CancelRequestID:                         v.CancelRequestID,
			LastHeartBeatUpdatedTime:                v.LastHeartBeatUpdatedTime,
			TimerTaskStatus:                         v.TimerTaskStatus,
			Attempt:                                 v.Attempt,
			DomainID:                                v.DomainID,
			StartedIdentity:                         v.StartedIdentity,
			TaskList:                                v.TaskList,
			TaskListKind:                            v.TaskListKind,
			HasRetryPolicy:                          v.HasRetryPolicy,
			InitialInterval:                         int32(v.InitialInterval.Seconds()),
			BackoffCoefficient:                      v.BackoffCoefficient,
			MaximumInterval:                         int32(v.MaximumInterval.Seconds()),
			ExpirationTime:                          v.ExpirationTime,
			MaximumAttempts:                         v.MaximumAttempts,
			NonRetriableErrors:                      v.NonRetriableErrors,
			LastFailureReason:                       v.LastFailureReason,
			LastWorkerIdentity:                      v.LastWorkerIdentity,
			LastFailureDetails:                      v.LastFailureDetails,
			LastHeartbeatTimeoutVisibilityInSeconds: v.LastHeartbeatTimeoutVisibilityInSeconds,
		}
		newInfos[k] = a
	}
	return newInfos, nil
}

func (m *executionManagerImpl) UpdateWorkflowExecution(
	ctx context.Context,
	request *UpdateWorkflowExecutionRequest,
) (*UpdateWorkflowExecutionResponse, error) {

	serializedWorkflowMutation, err := m.SerializeWorkflowMutation(&request.UpdateWorkflowMutation, request.Encoding)
	if err != nil {
		return nil, err
	}
	var serializedNewWorkflowSnapshot *InternalWorkflowSnapshot
	if request.NewWorkflowSnapshot != nil {
		serializedNewWorkflowSnapshot, err = m.SerializeWorkflowSnapshot(request.NewWorkflowSnapshot, request.Encoding)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalUpdateWorkflowExecutionRequest{
		RangeID: request.RangeID,

		Mode: request.Mode,

		UpdateWorkflowMutation: *serializedWorkflowMutation,
		NewWorkflowSnapshot:    serializedNewWorkflowSnapshot,

		WorkflowRequestMode: request.WorkflowRequestMode,

		CurrentTimeStamp: m.timeSrc.Now(),
	}
	msuss := m.statsComputer.computeMutableStateUpdateStats(newRequest)
	err = m.persistence.UpdateWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}
	return &UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: msuss}, nil
}

func (m *executionManagerImpl) SerializeUpsertChildExecutionInfos(
	infos []*ChildExecutionInfo,
	encoding constants.EncodingType,
) ([]*InternalChildExecutionInfo, error) {

	newInfos := make([]*InternalChildExecutionInfo, 0)
	for _, v := range infos {
		initiatedEvent, err := m.serializer.SerializeEvent(v.InitiatedEvent, encoding)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.SerializeEvent(v.StartedEvent, encoding)
		if err != nil {
			return nil, err
		}
		i := &InternalChildExecutionInfo{
			InitiatedEvent: initiatedEvent,
			StartedEvent:   startedEvent,

			Version:               v.Version,
			InitiatedID:           v.InitiatedID,
			InitiatedEventBatchID: v.InitiatedEventBatchID,
			CreateRequestID:       v.CreateRequestID,
			StartedID:             v.StartedID,
			StartedWorkflowID:     v.StartedWorkflowID,
			StartedRunID:          v.StartedRunID,
			DomainID:              v.DomainID,
			DomainNameDEPRECATED:  v.DomainNameDEPRECATED,
			WorkflowTypeName:      v.WorkflowTypeName,
			ParentClosePolicy:     v.ParentClosePolicy,
		}
		newInfos = append(newInfos, i)
	}
	return newInfos, nil
}

func (m *executionManagerImpl) SerializeUpsertActivityInfos(
	infos []*ActivityInfo,
	encoding constants.EncodingType,
) ([]*InternalActivityInfo, error) {

	newInfos := make([]*InternalActivityInfo, 0)
	for _, v := range infos {
		scheduledEvent, err := m.serializer.SerializeEvent(v.ScheduledEvent, encoding)
		if err != nil {
			return nil, err
		}
		startedEvent, err := m.serializer.SerializeEvent(v.StartedEvent, encoding)
		if err != nil {
			return nil, err
		}
		i := &InternalActivityInfo{
			Version:                                 v.Version,
			ScheduleID:                              v.ScheduleID,
			ScheduledEventBatchID:                   v.ScheduledEventBatchID,
			ScheduledEvent:                          scheduledEvent,
			ScheduledTime:                           v.ScheduledTime,
			StartedID:                               v.StartedID,
			StartedEvent:                            startedEvent,
			StartedTime:                             v.StartedTime,
			ActivityID:                              v.ActivityID,
			RequestID:                               v.RequestID,
			Details:                                 v.Details,
			ScheduleToStartTimeout:                  common.SecondsToDuration(int64(v.ScheduleToStartTimeout)),
			ScheduleToCloseTimeout:                  common.SecondsToDuration(int64(v.ScheduleToCloseTimeout)),
			StartToCloseTimeout:                     common.SecondsToDuration(int64(v.StartToCloseTimeout)),
			HeartbeatTimeout:                        common.SecondsToDuration(int64(v.HeartbeatTimeout)),
			CancelRequested:                         v.CancelRequested,
			CancelRequestID:                         v.CancelRequestID,
			LastHeartBeatUpdatedTime:                v.LastHeartBeatUpdatedTime,
			TimerTaskStatus:                         v.TimerTaskStatus,
			Attempt:                                 v.Attempt,
			DomainID:                                v.DomainID,
			StartedIdentity:                         v.StartedIdentity,
			TaskList:                                v.TaskList,
			TaskListKind:                            v.TaskListKind,
			HasRetryPolicy:                          v.HasRetryPolicy,
			InitialInterval:                         common.SecondsToDuration(int64(v.InitialInterval)),
			BackoffCoefficient:                      v.BackoffCoefficient,
			MaximumInterval:                         common.SecondsToDuration(int64(v.MaximumInterval)),
			ExpirationTime:                          v.ExpirationTime,
			MaximumAttempts:                         v.MaximumAttempts,
			NonRetriableErrors:                      v.NonRetriableErrors,
			LastFailureReason:                       v.LastFailureReason,
			LastWorkerIdentity:                      v.LastWorkerIdentity,
			LastFailureDetails:                      v.LastFailureDetails,
			LastHeartbeatTimeoutVisibilityInSeconds: v.LastHeartbeatTimeoutVisibilityInSeconds,
		}
		newInfos = append(newInfos, i)
	}
	return newInfos, nil
}

func (m *executionManagerImpl) SerializeExecutionInfo(
	info *WorkflowExecutionInfo,
	stats *ExecutionStats,
	encoding constants.EncodingType,
) (*InternalWorkflowExecutionInfo, error) {

	if info == nil {
		return &InternalWorkflowExecutionInfo{}, nil
	}
	completionEvent, err := m.serializer.SerializeEvent(info.CompletionEvent, encoding)
	if err != nil {
		return nil, err
	}

	resetPoints, err := m.serializer.SerializeResetPoints(info.AutoResetPoints, encoding)
	if err != nil {
		return nil, err
	}

	activeClusterSelectionPolicy, err := m.serializer.SerializeActiveClusterSelectionPolicy(info.ActiveClusterSelectionPolicy, encoding)
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowExecutionInfo{
		DomainID:                           info.DomainID,
		WorkflowID:                         info.WorkflowID,
		RunID:                              info.RunID,
		FirstExecutionRunID:                info.FirstExecutionRunID,
		ParentDomainID:                     info.ParentDomainID,
		ParentWorkflowID:                   info.ParentWorkflowID,
		ParentRunID:                        info.ParentRunID,
		InitiatedID:                        info.InitiatedID,
		CompletionEventBatchID:             info.CompletionEventBatchID,
		CompletionEvent:                    completionEvent,
		TaskList:                           info.TaskList,
		TaskListKind:                       info.TaskListKind,
		WorkflowTypeName:                   info.WorkflowTypeName,
		WorkflowTimeout:                    common.SecondsToDuration(int64(info.WorkflowTimeout)),
		DecisionStartToCloseTimeout:        common.SecondsToDuration(int64(info.DecisionStartToCloseTimeout)),
		ExecutionContext:                   info.ExecutionContext,
		State:                              info.State,
		CloseStatus:                        info.CloseStatus,
		LastFirstEventID:                   info.LastFirstEventID,
		LastEventTaskID:                    info.LastEventTaskID,
		NextEventID:                        info.NextEventID,
		LastProcessedEvent:                 info.LastProcessedEvent,
		StartTimestamp:                     info.StartTimestamp,
		LastUpdatedTimestamp:               info.LastUpdatedTimestamp,
		CreateRequestID:                    info.CreateRequestID,
		SignalCount:                        info.SignalCount,
		DecisionVersion:                    info.DecisionVersion,
		DecisionScheduleID:                 info.DecisionScheduleID,
		DecisionStartedID:                  info.DecisionStartedID,
		DecisionRequestID:                  info.DecisionRequestID,
		DecisionTimeout:                    common.SecondsToDuration(int64(info.DecisionTimeout)),
		DecisionAttempt:                    info.DecisionAttempt,
		DecisionStartedTimestamp:           time.Unix(0, info.DecisionStartedTimestamp).UTC(),
		DecisionScheduledTimestamp:         time.Unix(0, info.DecisionScheduledTimestamp).UTC(),
		DecisionOriginalScheduledTimestamp: time.Unix(0, info.DecisionOriginalScheduledTimestamp).UTC(),
		CancelRequested:                    info.CancelRequested,
		CancelRequestID:                    info.CancelRequestID,
		StickyTaskList:                     info.StickyTaskList,
		StickyScheduleToStartTimeout:       common.SecondsToDuration(int64(info.StickyScheduleToStartTimeout)),
		ClientLibraryVersion:               info.ClientLibraryVersion,
		ClientFeatureVersion:               info.ClientFeatureVersion,
		ClientImpl:                         info.ClientImpl,
		AutoResetPoints:                    resetPoints,
		Attempt:                            info.Attempt,
		HasRetryPolicy:                     info.HasRetryPolicy,
		InitialInterval:                    common.SecondsToDuration(int64(info.InitialInterval)),
		BackoffCoefficient:                 info.BackoffCoefficient,
		MaximumInterval:                    common.SecondsToDuration(int64(info.MaximumInterval)),
		ExpirationTime:                     info.ExpirationTime,
		MaximumAttempts:                    info.MaximumAttempts,
		NonRetriableErrors:                 info.NonRetriableErrors,
		BranchToken:                        info.BranchToken,
		CronSchedule:                       info.CronSchedule,
		ExpirationInterval:                 common.SecondsToDuration(int64(info.ExpirationSeconds)),
		Memo:                               info.Memo,
		SearchAttributes:                   info.SearchAttributes,
		PartitionConfig:                    info.PartitionConfig,
		CronOverlapPolicy:                  info.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:       activeClusterSelectionPolicy,

		// attributes which are not related to mutable state
		HistorySize: stats.HistorySize,
	}, nil
}

func (m *executionManagerImpl) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *ConflictResolveWorkflowExecutionRequest,
) (*ConflictResolveWorkflowExecutionResponse, error) {

	serializedResetWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&request.ResetWorkflowSnapshot, request.Encoding)
	if err != nil {
		return nil, err
	}
	var serializedCurrentWorkflowMutation *InternalWorkflowMutation
	if request.CurrentWorkflowMutation != nil {
		serializedCurrentWorkflowMutation, err = m.SerializeWorkflowMutation(request.CurrentWorkflowMutation, request.Encoding)
		if err != nil {
			return nil, err
		}
	}
	var serializedNewWorkflowMutation *InternalWorkflowSnapshot
	if request.NewWorkflowSnapshot != nil {
		serializedNewWorkflowMutation, err = m.SerializeWorkflowSnapshot(request.NewWorkflowSnapshot, request.Encoding)
		if err != nil {
			return nil, err
		}
	}

	newRequest := &InternalConflictResolveWorkflowExecutionRequest{
		RangeID: request.RangeID,

		Mode: request.Mode,

		ResetWorkflowSnapshot: *serializedResetWorkflowSnapshot,

		NewWorkflowSnapshot: serializedNewWorkflowMutation,

		CurrentWorkflowMutation: serializedCurrentWorkflowMutation,

		WorkflowRequestMode: request.WorkflowRequestMode,

		CurrentTimeStamp: m.timeSrc.Now(),
	}
	msuss := m.statsComputer.computeMutableStateConflictResolveStats(newRequest)
	err = m.persistence.ConflictResolveWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}
	return &ConflictResolveWorkflowExecutionResponse{MutableStateUpdateSessionStats: msuss}, nil
}

func (m *executionManagerImpl) CreateWorkflowExecution(
	ctx context.Context,
	request *CreateWorkflowExecutionRequest,
) (*CreateWorkflowExecutionResponse, error) {

	encoding := constants.EncodingTypeThriftRW

	serializedNewWorkflowSnapshot, err := m.SerializeWorkflowSnapshot(&request.NewWorkflowSnapshot, encoding)
	if err != nil {
		return nil, err
	}

	newRequest := &InternalCreateWorkflowExecutionRequest{
		RangeID: request.RangeID,

		Mode: request.Mode,

		PreviousRunID:            request.PreviousRunID,
		PreviousLastWriteVersion: request.PreviousLastWriteVersion,

		NewWorkflowSnapshot: *serializedNewWorkflowSnapshot,

		WorkflowRequestMode: request.WorkflowRequestMode,

		CurrentTimeStamp: m.timeSrc.Now(),
	}

	msuss := m.statsComputer.computeMutableStateCreateStats(newRequest)
	_, err = m.persistence.CreateWorkflowExecution(ctx, newRequest)
	if err != nil {
		return nil, err
	}
	return &CreateWorkflowExecutionResponse{MutableStateUpdateSessionStats: msuss}, nil
}

func (m *executionManagerImpl) SerializeWorkflowMutation(
	input *WorkflowMutation,
	encoding constants.EncodingType,
) (*InternalWorkflowMutation, error) {

	serializedExecutionInfo, err := m.SerializeExecutionInfo(
		input.ExecutionInfo,
		input.ExecutionStats,
		encoding,
	)
	if err != nil {
		return nil, err
	}
	serializedVersionHistories, err := m.SerializeVersionHistories(input.VersionHistories, encoding)
	if err != nil {
		return nil, err
	}
	serializedUpsertActivityInfos, err := m.SerializeUpsertActivityInfos(input.UpsertActivityInfos, encoding)
	if err != nil {
		return nil, err
	}
	serializedUpsertChildExecutionInfos, err := m.SerializeUpsertChildExecutionInfos(input.UpsertChildExecutionInfos, encoding)
	if err != nil {
		return nil, err
	}
	var serializedNewBufferedEvents *DataBlob
	if input.NewBufferedEvents != nil {
		serializedNewBufferedEvents, err = m.serializer.SerializeBatchEvents(input.NewBufferedEvents, encoding)
		if err != nil {
			return nil, err
		}
	}

	startVersion, err := getStartVersion(input.VersionHistories)
	if err != nil {
		return nil, err
	}
	lastWriteVersion, err := getLastWriteVersion(input.VersionHistories)
	if err != nil {
		return nil, err
	}
	checksumData, err := m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowMutation{
		ExecutionInfo:    serializedExecutionInfo,
		VersionHistories: serializedVersionHistories,
		StartVersion:     startVersion,
		LastWriteVersion: lastWriteVersion,

		UpsertActivityInfos:       serializedUpsertActivityInfos,
		DeleteActivityInfos:       input.DeleteActivityInfos,
		UpsertTimerInfos:          input.UpsertTimerInfos,
		DeleteTimerInfos:          input.DeleteTimerInfos,
		UpsertChildExecutionInfos: serializedUpsertChildExecutionInfos,
		DeleteChildExecutionInfos: input.DeleteChildExecutionInfos,
		UpsertRequestCancelInfos:  input.UpsertRequestCancelInfos,
		DeleteRequestCancelInfos:  input.DeleteRequestCancelInfos,
		UpsertSignalInfos:         input.UpsertSignalInfos,
		DeleteSignalInfos:         input.DeleteSignalInfos,
		UpsertSignalRequestedIDs:  input.UpsertSignalRequestedIDs,
		DeleteSignalRequestedIDs:  input.DeleteSignalRequestedIDs,
		NewBufferedEvents:         serializedNewBufferedEvents,
		ClearBufferedEvents:       input.ClearBufferedEvents,

		TasksByCategory: input.TasksByCategory,

		WorkflowRequests: input.WorkflowRequests,

		Condition:    input.Condition,
		Checksum:     input.Checksum,
		ChecksumData: checksumData,
	}, nil
}

func (m *executionManagerImpl) SerializeWorkflowSnapshot(
	input *WorkflowSnapshot,
	encoding constants.EncodingType,
) (*InternalWorkflowSnapshot, error) {

	serializedExecutionInfo, err := m.SerializeExecutionInfo(
		input.ExecutionInfo,
		input.ExecutionStats,
		encoding,
	)
	if err != nil {
		return nil, err
	}
	serializedVersionHistories, err := m.SerializeVersionHistories(input.VersionHistories, encoding)
	if err != nil {
		return nil, err
	}
	serializedActivityInfos, err := m.SerializeUpsertActivityInfos(input.ActivityInfos, encoding)
	if err != nil {
		return nil, err
	}
	serializedChildExecutionInfos, err := m.SerializeUpsertChildExecutionInfos(input.ChildExecutionInfos, encoding)
	if err != nil {
		return nil, err
	}

	startVersion, err := getStartVersion(input.VersionHistories)
	if err != nil {
		return nil, err
	}
	lastWriteVersion, err := getLastWriteVersion(input.VersionHistories)
	if err != nil {
		return nil, err
	}

	checksumData, err := m.serializer.SerializeChecksum(input.Checksum, constants.EncodingTypeJSON)
	if err != nil {
		return nil, err
	}

	return &InternalWorkflowSnapshot{
		ExecutionInfo:    serializedExecutionInfo,
		VersionHistories: serializedVersionHistories,
		StartVersion:     startVersion,
		LastWriteVersion: lastWriteVersion,

		ActivityInfos:       serializedActivityInfos,
		TimerInfos:          input.TimerInfos,
		ChildExecutionInfos: serializedChildExecutionInfos,
		RequestCancelInfos:  input.RequestCancelInfos,
		SignalInfos:         input.SignalInfos,
		SignalRequestedIDs:  input.SignalRequestedIDs,

		TasksByCategory: input.TasksByCategory,

		WorkflowRequests: input.WorkflowRequests,

		Condition:    input.Condition,
		Checksum:     input.Checksum,
		ChecksumData: checksumData,
	}, nil
}

func (m *executionManagerImpl) SerializeVersionHistories(
	versionHistories *VersionHistories,
	encoding constants.EncodingType,
) (*DataBlob, error) {

	if versionHistories == nil {
		return nil, nil
	}
	return m.serializer.SerializeVersionHistories(versionHistories.ToInternalType(), encoding)
}

func (m *executionManagerImpl) DeserializeVersionHistories(
	blob *DataBlob,
) (*VersionHistories, error) {

	if blob == nil {
		return nil, nil
	}
	versionHistories, err := m.serializer.DeserializeVersionHistories(blob)
	if err != nil {
		return nil, err
	}
	return NewVersionHistoriesFromInternalType(versionHistories), nil
}

func (m *executionManagerImpl) DeleteWorkflowExecution(
	ctx context.Context,
	request *DeleteWorkflowExecutionRequest,
) error {
	return m.persistence.DeleteWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *DeleteCurrentWorkflowExecutionRequest,
) error {
	return m.persistence.DeleteCurrentWorkflowExecution(ctx, request)
}

func (m *executionManagerImpl) GetCurrentExecution(
	ctx context.Context,
	request *GetCurrentExecutionRequest,
) (*GetCurrentExecutionResponse, error) {
	return m.persistence.GetCurrentExecution(ctx, request)
}

func (m *executionManagerImpl) ListCurrentExecutions(
	ctx context.Context,
	request *ListCurrentExecutionsRequest,
) (*ListCurrentExecutionsResponse, error) {
	return m.persistence.ListCurrentExecutions(ctx, request)
}

func (m *executionManagerImpl) IsWorkflowExecutionExists(
	ctx context.Context,
	request *IsWorkflowExecutionExistsRequest,
) (*IsWorkflowExecutionExistsResponse, error) {
	return m.persistence.IsWorkflowExecutionExists(ctx, request)
}

func (m *executionManagerImpl) ListConcreteExecutions(
	ctx context.Context,
	request *ListConcreteExecutionsRequest,
) (*ListConcreteExecutionsResponse, error) {
	response, err := m.persistence.ListConcreteExecutions(ctx, request)
	if err != nil {
		return nil, err
	}
	newResponse := &ListConcreteExecutionsResponse{
		Executions: make([]*ListConcreteExecutionsEntity, len(response.Executions)),
		PageToken:  response.NextPageToken,
	}
	for i, e := range response.Executions {
		info, _, err := m.DeserializeExecutionInfo(e.ExecutionInfo)
		if err != nil {
			return nil, err
		}
		vh, err := m.DeserializeVersionHistories(e.VersionHistories)
		if err != nil {
			return nil, err
		}
		newResponse.Executions[i] = &ListConcreteExecutionsEntity{
			ExecutionInfo:    info,
			VersionHistories: vh,
		}
	}
	return newResponse, nil
}

func (m *executionManagerImpl) PutReplicationTaskToDLQ(
	ctx context.Context,
	request *PutReplicationTaskToDLQRequest,
) error {
	internalRequest := &InternalPutReplicationTaskToDLQRequest{
		SourceClusterName: request.SourceClusterName,
		TaskInfo:          m.toInternalReplicationTaskInfo(request.TaskInfo),
	}
	return m.persistence.PutReplicationTaskToDLQ(ctx, internalRequest)
}

func (m *executionManagerImpl) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *GetReplicationTasksFromDLQRequest,
) (*GetHistoryTasksResponse, error) {
	return m.persistence.GetReplicationTasksFromDLQ(ctx, request)
}

func (m *executionManagerImpl) GetReplicationDLQSize(
	ctx context.Context,
	request *GetReplicationDLQSizeRequest,
) (*GetReplicationDLQSizeResponse, error) {
	return m.persistence.GetReplicationDLQSize(ctx, request)
}

func (m *executionManagerImpl) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *DeleteReplicationTaskFromDLQRequest,
) error {
	return m.persistence.DeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *RangeDeleteReplicationTaskFromDLQRequest,
) (*RangeDeleteReplicationTaskFromDLQResponse, error) {
	return m.persistence.RangeDeleteReplicationTaskFromDLQ(ctx, request)
}

func (m *executionManagerImpl) CreateFailoverMarkerTasks(
	ctx context.Context,
	request *CreateFailoverMarkersRequest,
) error {
	request.CurrentTimeStamp = m.timeSrc.Now()
	return m.persistence.CreateFailoverMarkerTasks(ctx, request)
}

func (m *executionManagerImpl) GetActiveClusterSelectionPolicy(
	ctx context.Context,
	domainID, wfID, rID string,
) (*types.ActiveClusterSelectionPolicy, error) {
	blob, err := m.persistence.GetActiveClusterSelectionPolicy(ctx, domainID, wfID, rID)
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, &types.EntityNotExistsError{
			Message: "active cluster selection policy not found",
		}
	}
	policy, err := m.serializer.DeserializeActiveClusterSelectionPolicy(blob)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (m *executionManagerImpl) DeleteActiveClusterSelectionPolicy(
	ctx context.Context,
	domainID, workflowID, runID string,
) error {
	return m.persistence.DeleteActiveClusterSelectionPolicy(ctx, domainID, workflowID, runID)
}

func (m *executionManagerImpl) Close() {
	m.persistence.Close()
}

func (m *executionManagerImpl) fromInternalReplicationTaskInfos(internalInfos []*InternalReplicationTaskInfo) []*ReplicationTaskInfo {
	if internalInfos == nil {
		return nil
	}
	infos := make([]*ReplicationTaskInfo, len(internalInfos))
	for i := 0; i < len(internalInfos); i++ {
		infos[i] = m.fromInternalReplicationTaskInfo(internalInfos[i])
	}
	return infos
}

func (m *executionManagerImpl) fromInternalReplicationTaskInfo(internalInfo *InternalReplicationTaskInfo) *ReplicationTaskInfo {
	if internalInfo == nil {
		return nil
	}
	return &ReplicationTaskInfo{
		DomainID:          internalInfo.DomainID,
		WorkflowID:        internalInfo.WorkflowID,
		RunID:             internalInfo.RunID,
		TaskID:            internalInfo.TaskID,
		TaskType:          internalInfo.TaskType,
		FirstEventID:      internalInfo.FirstEventID,
		NextEventID:       internalInfo.NextEventID,
		Version:           internalInfo.Version,
		ScheduledID:       internalInfo.ScheduledID,
		BranchToken:       internalInfo.BranchToken,
		NewRunBranchToken: internalInfo.NewRunBranchToken,
		CreationTime:      internalInfo.CreationTime.UnixNano(),
	}
}

func (m *executionManagerImpl) toInternalReplicationTaskInfo(info *ReplicationTaskInfo) *InternalReplicationTaskInfo {
	if info == nil {
		return nil
	}
	return &InternalReplicationTaskInfo{
		DomainID:          info.DomainID,
		WorkflowID:        info.WorkflowID,
		RunID:             info.RunID,
		TaskID:            info.TaskID,
		TaskType:          info.TaskType,
		FirstEventID:      info.FirstEventID,
		NextEventID:       info.NextEventID,
		Version:           info.Version,
		ScheduledID:       info.ScheduledID,
		BranchToken:       info.BranchToken,
		NewRunBranchToken: info.NewRunBranchToken,
		CreationTime:      time.Unix(0, info.CreationTime).UTC(),
		CurrentTimeStamp:  m.timeSrc.Now(),
	}
}

func (m *executionManagerImpl) GetHistoryTasks(
	ctx context.Context,
	request *GetHistoryTasksRequest,
) (*GetHistoryTasksResponse, error) {
	return m.persistence.GetHistoryTasks(ctx, request)
}

func (m *executionManagerImpl) CompleteHistoryTask(
	ctx context.Context,
	request *CompleteHistoryTaskRequest,
) error {
	return m.persistence.CompleteHistoryTask(ctx, request)
}

func (m *executionManagerImpl) RangeCompleteHistoryTask(
	ctx context.Context,
	request *RangeCompleteHistoryTaskRequest,
) (*RangeCompleteHistoryTaskResponse, error) {
	return m.persistence.RangeCompleteHistoryTask(ctx, request)
}

func getStartVersion(
	versionHistories *VersionHistories,
) (int64, error) {

	if versionHistories == nil {
		return constants.EmptyVersion, nil
	}

	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return 0, err
	}
	versionHistoryItem, err := versionHistory.GetFirstItem()
	if err != nil {
		return 0, err
	}
	return versionHistoryItem.Version, nil
}

func getLastWriteVersion(
	versionHistories *VersionHistories,
) (int64, error) {

	if versionHistories == nil {
		return constants.EmptyVersion, nil
	}

	versionHistory, err := versionHistories.GetCurrentVersionHistory()
	if err != nil {
		return 0, err
	}
	versionHistoryItem, err := versionHistory.GetLastItem()
	if err != nil {
		return 0, err
	}
	return versionHistoryItem.Version, nil
}
