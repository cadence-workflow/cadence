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
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// GetActivityInfo gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivityInfo(
	scheduleEventID int64,
) (*persistence.ActivityInfo, bool) {

	ai, ok := e.pendingActivityInfoIDs[scheduleEventID]
	return ai, ok
}

func (e *mutableStateBuilder) GetPendingActivityInfos() map[int64]*persistence.ActivityInfo {
	return e.pendingActivityInfoIDs
}

// GetActivityByActivityID gives details about an activity that is currently in progress.
func (e *mutableStateBuilder) GetActivityByActivityID(
	activityID string,
) (*persistence.ActivityInfo, bool) {

	eventID, ok := e.pendingActivityIDToEventID[activityID]
	if !ok {
		return nil, false
	}
	return e.GetActivityInfo(eventID)
}

func (e *mutableStateBuilder) UpdateActivityProgress(
	ai *persistence.ActivityInfo,
	request *types.RecordActivityTaskHeartbeatRequest,
) {
	ai.Version = e.GetCurrentVersion()
	ai.Details = request.Details
	ai.LastHeartBeatUpdatedTime = e.timeSource.Now()
	e.updateActivityInfos[ai.ScheduleID] = ai
	e.syncActivityTasks[ai.ScheduleID] = struct{}{}
}

// ReplicateActivityInfo replicate the necessary activity information
func (e *mutableStateBuilder) ReplicateActivityInfo(
	request *types.SyncActivityRequest,
	resetActivityTimerTaskStatus bool,
) error {
	ai, ok := e.pendingActivityInfoIDs[request.GetScheduledID()]
	if !ok {
		e.logError(
			fmt.Sprintf("unable to find activity event ID: %v in mutable state", request.GetScheduledID()),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		return ErrMissingActivityInfo
	}

	ai.Version = request.GetVersion()
	ai.ScheduledTime = time.Unix(0, request.GetScheduledTime())
	ai.StartedID = request.GetStartedID()
	ai.LastHeartBeatUpdatedTime = time.Unix(0, request.GetLastHeartbeatTime())
	if ai.StartedID == constants.EmptyEventID {
		ai.StartedTime = time.Time{}
	} else {
		ai.StartedTime = time.Unix(0, request.GetStartedTime())
	}
	ai.Details = request.GetDetails()
	ai.Attempt = request.GetAttempt()
	ai.LastFailureReason = request.GetLastFailureReason()
	ai.LastWorkerIdentity = request.GetLastWorkerIdentity()
	ai.LastFailureDetails = request.GetLastFailureDetails()

	if resetActivityTimerTaskStatus {
		ai.TimerTaskStatus = TimerTaskStatusNone
	}

	e.updateActivityInfos[ai.ScheduleID] = ai
	return nil
}

// UpdateActivity updates an activity
func (e *mutableStateBuilder) UpdateActivity(
	ai *persistence.ActivityInfo,
) error {

	if _, ok := e.pendingActivityInfoIDs[ai.ScheduleID]; !ok {
		e.logError(
			fmt.Sprintf("unable to find activity ID: %v in mutable state", ai.ActivityID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		return ErrMissingActivityInfo
	}

	e.pendingActivityInfoIDs[ai.ScheduleID] = ai
	e.updateActivityInfos[ai.ScheduleID] = ai
	return nil
}

// DeleteActivity deletes details about an activity.
func (e *mutableStateBuilder) DeleteActivity(
	scheduleEventID int64,
) error {

	if activityInfo, ok := e.pendingActivityInfoIDs[scheduleEventID]; ok {
		delete(e.pendingActivityInfoIDs, scheduleEventID)

		if _, ok = e.pendingActivityIDToEventID[activityInfo.ActivityID]; ok {
			delete(e.pendingActivityIDToEventID, activityInfo.ActivityID)
		} else {
			e.logError(
				fmt.Sprintf("unable to find activity ID: %v in mutable state", activityInfo.ActivityID),
				tag.ErrorTypeInvalidMutableStateAction,
			)
			// log data inconsistency instead of returning an error
			e.logDataInconsistency()
		}
	} else {
		e.logError(
			fmt.Sprintf("unable to find activity event id: %v in mutable state", scheduleEventID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		// log data inconsistency instead of returning an error
		e.logDataInconsistency()
	}

	delete(e.updateActivityInfos, scheduleEventID)
	e.deleteActivityInfos[scheduleEventID] = struct{}{}
	return nil
}

func (e *mutableStateBuilder) GetActivityScheduledEvent(
	ctx context.Context,
	scheduleEventID int64,
) (*types.HistoryEvent, error) {

	ai, ok := e.pendingActivityInfoIDs[scheduleEventID]
	if !ok {
		return nil, ErrMissingActivityInfo
	}

	// Needed for backward compatibility reason
	if ai.ScheduledEvent != nil {
		return ai.ScheduledEvent, nil
	}

	currentBranchToken, err := e.GetCurrentBranchToken()
	if err != nil {
		return nil, err
	}
	scheduledEvent, err := e.eventsCache.GetEvent(
		ctx,
		e.shard.GetShardID(),
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		ai.ScheduledEventBatchID,
		ai.ScheduleID,
		currentBranchToken,
	)
	if err != nil {
		// do not return the original error
		// since original error can be of type entity not exists
		// which can cause task processing side to fail silently
		// However, if the error is a persistence transient error,
		// we return the original error, because we fail to get
		// the event because of failure from database
		if persistence.IsTransientError(err) {
			return nil, err
		}
		return nil, ErrMissingActivityScheduledEvent
	}
	return scheduledEvent, nil
}

func (e *mutableStateBuilder) AddActivityTaskScheduledEvent(
	ctx context.Context,
	decisionCompletedEventID int64,
	attributes *types.ScheduleActivityTaskDecisionAttributes,
	dispatch bool,
) (*types.HistoryEvent, *persistence.ActivityInfo, *types.ActivityLocalDispatchInfo, bool, bool, error) {

	opTag := tag.WorkflowActionActivityTaskScheduled
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, nil, false, false, err
	}

	_, ok := e.GetActivityByActivityID(attributes.GetActivityID())
	if ok {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction)
		return nil, nil, nil, false, false, e.createCallerError(opTag)
	}

	pendingActivitiesCount := len(e.pendingActivityInfoIDs)

	if pendingActivitiesCount >= e.config.PendingActivitiesCountLimitError() {
		e.logger.Error("Pending activity count exceeds error limit",
			tag.WorkflowDomainName(e.GetDomainEntry().GetInfo().Name),
			tag.WorkflowID(e.executionInfo.WorkflowID),
			tag.WorkflowRunID(e.executionInfo.RunID),
			tag.Number(int64(pendingActivitiesCount)))

		if e.config.PendingActivityValidationEnabled() {
			return nil, nil, nil, false, false, ErrTooManyPendingActivities
		}
	} else if pendingActivitiesCount >= e.config.PendingActivitiesCountLimitWarn() && !e.pendingActivityWarningSent {
		e.logger.Warn("Pending activity count exceeds warn limit",
			tag.WorkflowDomainName(e.GetDomainEntry().GetInfo().Name),
			tag.WorkflowID(e.executionInfo.WorkflowID),
			tag.WorkflowRunID(e.executionInfo.RunID),
			tag.Number(int64(pendingActivitiesCount)))

		e.pendingActivityWarningSent = true
	}

	taskList := &types.TaskList{
		Name: attributes.TaskList.Name,
		Kind: attributes.TaskList.Kind,
	}
	if taskList.GetKind() == types.TaskListKindNormal && e.executionInfo.TaskListKind == types.TaskListKindEphemeral {
		taskList.Kind = types.TaskListKindEphemeral.Ptr()
	}

	event := e.hBuilder.AddActivityTaskScheduledEvent(decisionCompletedEventID, attributes, taskList)

	// Write the event to cache only on active cluster for processing on activity started or retried
	e.eventsCache.PutEvent(
		e.executionInfo.DomainID,
		e.executionInfo.WorkflowID,
		e.executionInfo.RunID,
		event.ID,
		event,
	)

	ai, err := e.ReplicateActivityTaskScheduledEvent(decisionCompletedEventID, event, true)
	if err != nil {
		return nil, nil, nil, false, false, err
	}
	activityStartedScope := e.metricsClient.Scope(metrics.HistoryRecordActivityTaskStartedScope)
	if e.config.EnableActivityLocalDispatchByDomain(e.domainEntry.GetInfo().Name) && attributes.RequestLocalDispatch {
		activityStartedScope.IncCounter(metrics.CadenceRequests)
		return event, ai, &types.ActivityLocalDispatchInfo{ActivityID: ai.ActivityID}, false, false, nil
	}
	started := false
	if dispatch {
		started = e.tryDispatchActivityTask(ctx, event, ai)
	}
	if started {
		activityStartedScope.IncCounter(metrics.CadenceRequests)
		return event, ai, nil, true, true, nil
	}

	if err := e.taskGenerator.GenerateActivityTransferTasks(event); err != nil {
		return nil, nil, nil, dispatch, false, err
	}

	return event, ai, nil, dispatch, false, err
}

func (e *mutableStateBuilder) tryDispatchActivityTask(
	ctx context.Context,
	scheduledEvent *types.HistoryEvent,
	ai *persistence.ActivityInfo,
) bool {
	taggedScope := e.metricsClient.Scope(metrics.HistoryScheduleDecisionTaskScope).Tagged(
		metrics.DomainTag(e.domainEntry.GetInfo().Name),
		metrics.WorkflowTypeTag(e.GetWorkflowType().Name),
		metrics.TaskListTag(ai.TaskList))
	taggedScope.IncCounter(metrics.DecisionTypeScheduleActivityDispatchCounter)
	_, err := e.shard.GetService().GetMatchingClient().AddActivityTask(ctx, &types.AddActivityTaskRequest{
		DomainUUID:       e.executionInfo.DomainID,
		SourceDomainUUID: e.domainEntry.GetInfo().ID,
		Execution: &types.WorkflowExecution{
			WorkflowID: e.executionInfo.WorkflowID,
			RunID:      e.executionInfo.RunID,
		},
		TaskList:                      &types.TaskList{Name: ai.TaskList},
		ScheduleID:                    scheduledEvent.ID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(ai.ScheduleToStartTimeout),
		ActivityTaskDispatchInfo: &types.ActivityTaskDispatchInfo{
			ScheduledEvent:                  scheduledEvent,
			StartedTimestamp:                common.Int64Ptr(e.timeSource.Now().UnixNano()),
			WorkflowType:                    e.GetWorkflowType(),
			WorkflowDomain:                  e.GetDomainEntry().GetInfo().Name,
			ScheduledTimestampOfThisAttempt: common.Int64Ptr(ai.ScheduledTime.UnixNano()),
		},
		PartitionConfig: e.executionInfo.PartitionConfig,
	})
	if err == nil {
		taggedScope.IncCounter(metrics.DecisionTypeScheduleActivityDispatchSucceedCounter)
		return true
	}
	return false
}

func (e *mutableStateBuilder) ReplicateActivityTaskScheduledEvent(
	firstEventID int64,
	event *types.HistoryEvent,
	skipTaskGeneration bool,
) (*persistence.ActivityInfo, error) {

	attributes := event.ActivityTaskScheduledEventAttributes
	targetDomainID := e.executionInfo.DomainID
	if attributes.GetDomain() != "" {
		var err error
		targetDomainID, err = e.shard.GetDomainCache().GetDomainID(attributes.GetDomain())
		if err != nil {
			return nil, err
		}
	}

	scheduleEventID := event.ID
	scheduleToCloseTimeout := attributes.GetScheduleToCloseTimeoutSeconds()

	ai := &persistence.ActivityInfo{
		Version:                  event.Version,
		ScheduleID:               scheduleEventID,
		ScheduledEventBatchID:    firstEventID,
		ScheduledTime:            time.Unix(0, event.GetTimestamp()),
		StartedID:                constants.EmptyEventID,
		StartedTime:              time.Time{},
		ActivityID:               attributes.ActivityID,
		DomainID:                 targetDomainID,
		ScheduleToStartTimeout:   attributes.GetScheduleToStartTimeoutSeconds(),
		ScheduleToCloseTimeout:   scheduleToCloseTimeout,
		StartToCloseTimeout:      attributes.GetStartToCloseTimeoutSeconds(),
		HeartbeatTimeout:         attributes.GetHeartbeatTimeoutSeconds(),
		CancelRequested:          false,
		CancelRequestID:          constants.EmptyEventID,
		LastHeartBeatUpdatedTime: time.Time{},
		TimerTaskStatus:          TimerTaskStatusNone,
		TaskList:                 attributes.TaskList.GetName(),
		TaskListKind:             attributes.TaskList.GetKind(),
		HasRetryPolicy:           attributes.RetryPolicy != nil,
	}

	if ai.HasRetryPolicy {
		ai.InitialInterval = attributes.RetryPolicy.GetInitialIntervalInSeconds()
		ai.BackoffCoefficient = attributes.RetryPolicy.GetBackoffCoefficient()
		ai.MaximumInterval = attributes.RetryPolicy.GetMaximumIntervalInSeconds()
		ai.MaximumAttempts = attributes.RetryPolicy.GetMaximumAttempts()
		ai.NonRetriableErrors = attributes.RetryPolicy.NonRetriableErrorReasons
		if attributes.RetryPolicy.GetExpirationIntervalInSeconds() != 0 {
			ai.ExpirationTime = ai.ScheduledTime.Add(time.Duration(attributes.RetryPolicy.GetExpirationIntervalInSeconds()) * time.Second)
		}
	}

	e.pendingActivityInfoIDs[scheduleEventID] = ai
	e.pendingActivityIDToEventID[ai.ActivityID] = scheduleEventID
	e.updateActivityInfos[ai.ScheduleID] = ai

	if !skipTaskGeneration {
		return ai, e.taskGenerator.GenerateActivityTransferTasks(event)
	}

	return ai, nil
}

func (e *mutableStateBuilder) addTransientActivityStartedEvent(
	scheduleEventID int64,
) error {

	ai, ok := e.GetActivityInfo(scheduleEventID)
	if !ok || ai.StartedID != constants.TransientEventID {
		return nil
	}

	// activity task was started (as transient event), we need to add it now.
	event := e.hBuilder.AddActivityTaskStartedEvent(scheduleEventID, ai.Attempt, ai.RequestID, ai.StartedIdentity,
		ai.LastFailureReason, ai.LastFailureDetails)
	if !ai.StartedTime.IsZero() {
		// overwrite started event time to the one recorded in ActivityInfo
		event.Timestamp = common.Int64Ptr(ai.StartedTime.UnixNano())
	}
	return e.ReplicateActivityTaskStartedEvent(event)
}

func (e *mutableStateBuilder) AddActivityTaskStartedEvent(
	ai *persistence.ActivityInfo,
	scheduleEventID int64,
	requestID string,
	identity string,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskStarted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if !ai.HasRetryPolicy {
		event := e.hBuilder.AddActivityTaskStartedEvent(scheduleEventID, ai.Attempt, requestID, identity,
			ai.LastFailureReason, ai.LastFailureDetails)
		if err := e.ReplicateActivityTaskStartedEvent(event); err != nil {
			return nil, err
		}
		return event, nil
	}

	// we might need to retry, so do not append started event just yet,
	// instead update mutable state and will record started event when activity task is closed
	ai.Version = e.GetCurrentVersion()
	ai.StartedID = constants.TransientEventID
	ai.RequestID = requestID
	ai.StartedTime = e.timeSource.Now()
	ai.LastHeartBeatUpdatedTime = ai.StartedTime
	ai.StartedIdentity = identity
	if err := e.UpdateActivity(ai); err != nil {
		return nil, err
	}
	e.syncActivityTasks[ai.ScheduleID] = struct{}{}
	return nil, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskStartedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskStartedEventAttributes
	scheduleID := attributes.GetScheduledEventID()
	ai, ok := e.GetActivityInfo(scheduleID)
	if !ok {
		e.logError(
			fmt.Sprintf("unable to find activity event id: %v in mutable state", scheduleID),
			tag.ErrorTypeInvalidMutableStateAction,
		)
		return ErrMissingActivityInfo
	}

	ai.Version = event.Version
	ai.StartedID = event.ID
	ai.RequestID = attributes.GetRequestID()
	ai.StartedTime = time.Unix(0, event.GetTimestamp())
	ai.LastHeartBeatUpdatedTime = ai.StartedTime
	e.updateActivityInfos[ai.ScheduleID] = ai
	return nil
}

func (e *mutableStateBuilder) AddActivityTaskCompletedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *types.RespondActivityTaskCompletedRequest,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCompleted
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if ai, ok := e.GetActivityInfo(scheduleEventID); !ok || ai.StartedID != startedEventID {
		e.logger.Warn(
			mutableStateInvalidHistoryActionMsg,
			opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskCompletedEvent(scheduleEventID, startedEventID, request)
	if err := e.ReplicateActivityTaskCompletedEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCompletedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskCompletedEventAttributes
	scheduleID := attributes.GetScheduledEventID()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskFailedEvent(
	scheduleEventID int64,
	startedEventID int64,
	request *types.RespondActivityTaskFailedRequest,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	if ai, ok := e.GetActivityInfo(scheduleEventID); !ok || ai.StartedID != startedEventID {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskFailedEvent(scheduleEventID, startedEventID, request)
	if err := e.ReplicateActivityTaskFailedEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskFailedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskFailedEventAttributes
	scheduleID := attributes.GetScheduledEventID()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
	timeoutType types.TimeoutType,
	lastHeartBeatDetails []byte,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskTimedOut
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ai, ok := e.GetActivityInfo(scheduleEventID)
	if !ok || ai.StartedID != startedEventID || ((timeoutType == types.TimeoutTypeStartToClose ||
		timeoutType == types.TimeoutTypeHeartbeat) && ai.StartedID == constants.EmptyEventID) {
		e.logger.Warn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowStartedID(startedEventID),
			tag.WorkflowTimeoutType(int64(timeoutType)))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskTimedOutEvent(scheduleEventID, startedEventID, timeoutType, lastHeartBeatDetails, ai.LastFailureReason, ai.LastFailureDetails)
	if err := e.ReplicateActivityTaskTimedOutEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskTimedOutEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskTimedOutEventAttributes
	scheduleID := attributes.GetScheduledEventID()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) AddActivityTaskCancelRequestedEvent(
	decisionCompletedEventID int64,
	activityID string,
	identity string,
) (*types.HistoryEvent, *persistence.ActivityInfo, error) {

	opTag := tag.WorkflowActionActivityTaskCancelRequested
	if err := e.checkMutability(opTag); err != nil {
		return nil, nil, err
	}

	// we need to add the cancel request event even if activity not in mutable state
	// if activity not in mutable state or already cancel requested,
	// we do not need to call the replication function
	actCancelReqEvent := e.hBuilder.AddActivityTaskCancelRequestedEvent(decisionCompletedEventID, activityID)

	ai, ok := e.GetActivityByActivityID(activityID)
	if !ok || ai.CancelRequested {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.Bool(ok),
			tag.WorkflowActivityID(activityID))

		return nil, nil, e.createCallerError(opTag)
	}

	if err := e.ReplicateActivityTaskCancelRequestedEvent(actCancelReqEvent); err != nil {
		return nil, nil, err
	}

	return actCancelReqEvent, ai, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCancelRequestedEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskCancelRequestedEventAttributes
	activityID := attributes.GetActivityID()
	ai, ok := e.GetActivityByActivityID(activityID)
	if !ok {
		// On active side, if the ActivityTaskCancelRequested is invalid, it will created a RequestCancelActivityTaskFailed
		// Passive will rely on active side logic
		return nil
	}

	ai.Version = event.Version

	// - We have the activity dispatched to worker.
	// - The activity might not be heartbeating, but the activity can still call RecordActivityHeartBeat()
	//   to see cancellation while reporting progress of the activity.
	ai.CancelRequested = true

	ai.CancelRequestID = event.ID
	e.updateActivityInfos[ai.ScheduleID] = ai
	return nil
}

func (e *mutableStateBuilder) AddRequestCancelActivityTaskFailedEvent(
	decisionCompletedEventID int64,
	activityID string,
	cause string,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCancelFailed
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	return e.hBuilder.AddRequestCancelActivityTaskFailedEvent(decisionCompletedEventID, activityID, cause), nil
}

func (e *mutableStateBuilder) AddActivityTaskCanceledEvent(
	scheduleEventID int64,
	startedEventID int64,
	latestCancelRequestedEventID int64,
	details []byte,
	identity string,
) (*types.HistoryEvent, error) {

	opTag := tag.WorkflowActionActivityTaskCanceled
	if err := e.checkMutability(opTag); err != nil {
		return nil, err
	}

	ai, ok := e.GetActivityInfo(scheduleEventID)
	if !ok || ai.StartedID != startedEventID {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID))
		return nil, e.createInternalServerError(opTag)
	}

	// Verify cancel request as well.
	if !ai.CancelRequested {
		e.logWarn(mutableStateInvalidHistoryActionMsg, opTag,
			tag.WorkflowEventID(e.GetNextEventID()),
			tag.ErrorTypeInvalidHistoryAction,
			tag.WorkflowScheduleID(scheduleEventID),
			tag.WorkflowActivityID(ai.ActivityID),
			tag.WorkflowStartedID(ai.StartedID))
		return nil, e.createInternalServerError(opTag)
	}

	if err := e.addTransientActivityStartedEvent(scheduleEventID); err != nil {
		return nil, err
	}
	event := e.hBuilder.AddActivityTaskCanceledEvent(scheduleEventID, startedEventID, latestCancelRequestedEventID,
		details, identity)
	if err := e.ReplicateActivityTaskCanceledEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *mutableStateBuilder) ReplicateActivityTaskCanceledEvent(
	event *types.HistoryEvent,
) error {

	attributes := event.ActivityTaskCanceledEventAttributes
	scheduleID := attributes.GetScheduledEventID()

	return e.DeleteActivity(scheduleID)
}

func (e *mutableStateBuilder) RetryActivity(
	ai *persistence.ActivityInfo,
	failureReason string,
	failureDetails []byte,
) (bool, error) {

	opTag := tag.WorkflowActionActivityTaskRetry
	if err := e.checkMutability(opTag); err != nil {
		return false, err
	}

	if !ai.HasRetryPolicy || ai.CancelRequested {
		return false, nil
	}

	now := e.timeSource.Now()

	backoffInterval := getBackoffInterval(
		now,
		ai.ExpirationTime,
		ai.Attempt,
		ai.MaximumAttempts,
		ai.InitialInterval,
		ai.MaximumInterval,
		ai.BackoffCoefficient,
		failureReason,
		ai.NonRetriableErrors,
	)
	if backoffInterval == backoff.NoBackoff {
		return false, nil
	}

	// a retry is needed, update activity info for next retry
	ai.Version = e.GetCurrentVersion()
	ai.Attempt++
	ai.ScheduledTime = now.Add(backoffInterval) // update to next schedule time
	ai.StartedID = constants.EmptyEventID
	ai.RequestID = ""
	ai.StartedTime = time.Time{}
	ai.TimerTaskStatus = TimerTaskStatusNone
	ai.LastFailureReason = failureReason
	ai.LastWorkerIdentity = ai.StartedIdentity
	ai.LastFailureDetails = failureDetails

	if err := e.taskGenerator.GenerateActivityRetryTasks(
		ai.ScheduleID,
	); err != nil {
		return false, err
	}

	e.updateActivityInfos[ai.ScheduleID] = ai
	e.syncActivityTasks[ai.ScheduleID] = struct{}{}
	return true, nil
}
