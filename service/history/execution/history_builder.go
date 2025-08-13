// Copyright (c) 2020 Uber Technologies, Inc.
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

package execution

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// HistoryBuilder builds and stores workflow history events
	HistoryBuilder struct {
		transientHistory []*types.HistoryEvent
		history          []*types.HistoryEvent
		msBuilder        MutableState
	}
)

// NewHistoryBuilder creates a new history builder
func NewHistoryBuilder(msBuilder MutableState) *HistoryBuilder {
	return &HistoryBuilder{
		transientHistory: []*types.HistoryEvent{},
		history:          []*types.HistoryEvent{},
		msBuilder:        msBuilder,
	}
}

// NewHistoryBuilderFromEvents creates a new history builder based on the given workflow history events
func NewHistoryBuilderFromEvents(history []*types.HistoryEvent, msBuilder MutableState) *HistoryBuilder {
	return &HistoryBuilder{
		history:   history,
		msBuilder: msBuilder,
	}
}

// AddWorkflowExecutionStartedEvent adds WorkflowExecutionStarted event to history
// originalRunID is the runID when the WorkflowExecutionStarted event is written
// firstRunID is the very first runID along the chain of ContinueAsNew and Reset
func (b *HistoryBuilder) AddWorkflowExecutionStartedEvent(
	startRequest *types.HistoryStartWorkflowExecutionRequest,
	previousExecution *persistence.WorkflowExecutionInfo,
	firstRunID string,
	originalRunID string,
	firstScheduledTime time.Time,
) *types.HistoryEvent {

	var prevRunID string
	var resetPoints *types.ResetPoints
	if previousExecution != nil {
		prevRunID = previousExecution.RunID
		resetPoints = previousExecution.AutoResetPoints
	}

	request := startRequest.StartRequest
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionStarted)

	var scheduledTime *time.Time
	if request.CronSchedule != "" {
		// first scheduled time is only necessary for cron workflows.
		scheduledTime = &firstScheduledTime
	}
	attributes := &types.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        request.WorkflowType,
		TaskList:                            request.TaskList,
		Header:                              request.Header,
		Input:                               request.Input,
		ExecutionStartToCloseTimeoutSeconds: request.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      request.TaskStartToCloseTimeoutSeconds,
		ContinuedExecutionRunID:             prevRunID,
		PrevAutoResetPoints:                 resetPoints,
		Identity:                            request.Identity,
		RetryPolicy:                         request.RetryPolicy,
		Attempt:                             startRequest.GetAttempt(),
		ExpirationTimestamp:                 startRequest.ExpirationTimestamp,
		CronSchedule:                        request.CronSchedule,
		CronOverlapPolicy:                   request.CronOverlapPolicy,
		LastCompletionResult:                startRequest.LastCompletionResult,
		ContinuedFailureReason:              startRequest.ContinuedFailureReason,
		ContinuedFailureDetails:             startRequest.ContinuedFailureDetails,
		Initiator:                           startRequest.ContinueAsNewInitiator,
		FirstDecisionTaskBackoffSeconds:     startRequest.FirstDecisionTaskBackoffSeconds,
		FirstExecutionRunID:                 firstRunID,
		FirstScheduleTime:                   scheduledTime,
		OriginalExecutionRunID:              originalRunID,
		Memo:                                request.Memo,
		SearchAttributes:                    request.SearchAttributes,
		JitterStartSeconds:                  request.JitterStartSeconds,
		PartitionConfig:                     startRequest.PartitionConfig,
		RequestID:                           request.RequestID,
		ActiveClusterSelectionPolicy:        request.ActiveClusterSelectionPolicy,
	}
	if parentInfo := startRequest.ParentExecutionInfo; parentInfo != nil {
		attributes.ParentWorkflowDomainID = &parentInfo.DomainUUID
		attributes.ParentWorkflowDomain = &parentInfo.Domain
		attributes.ParentWorkflowExecution = parentInfo.Execution
		attributes.ParentInitiatedEventID = &parentInfo.InitiatedID
	}
	event.WorkflowExecutionStartedEventAttributes = attributes

	return b.addEventToHistory(event)
}

// AddDecisionTaskScheduledEvent adds DecisionTaskScheduled event to history
func (b *HistoryBuilder) AddDecisionTaskScheduledEvent(taskList string,
	startToCloseTimeoutSeconds int32, attempt int64) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeDecisionTaskScheduled)
	event.DecisionTaskScheduledEventAttributes = getDecisionTaskScheduledEventAttributes(taskList, startToCloseTimeoutSeconds, attempt)

	return b.addEventToHistory(event)
}

// AddTransientDecisionTaskScheduledEvent adds transient DecisionTaskScheduled event
func (b *HistoryBuilder) AddTransientDecisionTaskScheduledEvent(taskList string,
	startToCloseTimeoutSeconds int32, attempt int64, timestamp int64) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEventWithTimestamp(types.EventTypeDecisionTaskScheduled, timestamp)
	event.DecisionTaskScheduledEventAttributes = getDecisionTaskScheduledEventAttributes(taskList, startToCloseTimeoutSeconds, attempt)

	return b.addTransientEvent(event)
}

// AddDecisionTaskStartedEvent adds DecisionTaskStarted event to history
func (b *HistoryBuilder) AddDecisionTaskStartedEvent(scheduleEventID int64, requestID string,
	identity string) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeDecisionTaskStarted)
	event.DecisionTaskStartedEventAttributes = getDecisionTaskStartedEventAttributes(scheduleEventID, requestID, identity)

	return b.addEventToHistory(event)
}

// AddTransientDecisionTaskStartedEvent adds transient DecisionTaskStarted event
func (b *HistoryBuilder) AddTransientDecisionTaskStartedEvent(scheduleEventID int64, requestID string,
	identity string, timestamp int64) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEventWithTimestamp(types.EventTypeDecisionTaskStarted, timestamp)
	event.DecisionTaskStartedEventAttributes = getDecisionTaskStartedEventAttributes(scheduleEventID, requestID, identity)

	return b.addTransientEvent(event)
}

// AddDecisionTaskCompletedEvent adds DecisionTaskCompleted event to history
func (b *HistoryBuilder) AddDecisionTaskCompletedEvent(scheduleEventID, StartedEventID int64,
	request *types.RespondDecisionTaskCompletedRequest) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeDecisionTaskCompleted)
	event.DecisionTaskCompletedEventAttributes = &types.DecisionTaskCompletedEventAttributes{
		ExecutionContext: request.ExecutionContext,
		ScheduledEventID: scheduleEventID,
		StartedEventID:   StartedEventID,
		Identity:         request.Identity,
		BinaryChecksum:   request.BinaryChecksum,
	}

	return b.addEventToHistory(event)
}

// AddDecisionTaskTimedOutEvent adds DecisionTaskTimedOut event to history
func (b *HistoryBuilder) AddDecisionTaskTimedOutEvent(
	scheduleEventID int64,
	startedEventID int64,
	timeoutType types.TimeoutType,
	baseRunID string,
	newRunID string,
	forkEventVersion int64,
	reason string,
	cause types.DecisionTaskTimedOutCause,
	resetRequestID string,
) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeDecisionTaskTimedOut)
	event.DecisionTaskTimedOutEventAttributes = &types.DecisionTaskTimedOutEventAttributes{
		ScheduledEventID: scheduleEventID,
		StartedEventID:   startedEventID,
		TimeoutType:      timeoutType.Ptr(),
		BaseRunID:        baseRunID,
		NewRunID:         newRunID,
		ForkEventVersion: forkEventVersion,
		Reason:           reason,
		Cause:            cause.Ptr(),
		RequestID:        resetRequestID,
	}

	return b.addEventToHistory(event)
}

// AddDecisionTaskFailedEvent adds DecisionTaskFailed event to history
func (b *HistoryBuilder) AddDecisionTaskFailedEvent(attr types.DecisionTaskFailedEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeDecisionTaskFailed)
	event.DecisionTaskFailedEventAttributes = &attr

	return b.addEventToHistory(event)
}

// AddActivityTaskScheduledEvent adds ActivityTaskScheduled event to history
func (b *HistoryBuilder) AddActivityTaskScheduledEvent(decisionCompletedEventID int64,
	attributes *types.ScheduleActivityTaskDecisionAttributes, taskList *types.TaskList) *types.HistoryEvent {

	var domain *string
	if attributes.Domain != "" {
		// for backward compatibility
		// old releases will encounter issues if Domain field is a pointer to an empty string.
		domain = common.StringPtr(attributes.Domain)
	}

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskScheduled)
	event.ActivityTaskScheduledEventAttributes = &types.ActivityTaskScheduledEventAttributes{
		ActivityID:                    attributes.ActivityID,
		ActivityType:                  attributes.ActivityType,
		TaskList:                      taskList,
		Header:                        attributes.Header,
		Input:                         attributes.Input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(common.Int32Default(attributes.ScheduleToCloseTimeoutSeconds)),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(common.Int32Default(attributes.ScheduleToStartTimeoutSeconds)),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(common.Int32Default(attributes.StartToCloseTimeoutSeconds)),
		HeartbeatTimeoutSeconds:       common.Int32Ptr(common.Int32Default(attributes.HeartbeatTimeoutSeconds)),
		DecisionTaskCompletedEventID:  decisionCompletedEventID,
		RetryPolicy:                   attributes.RetryPolicy,
		Domain:                        domain,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskStartedEvent adds ActivityTaskStarted event to history
func (b *HistoryBuilder) AddActivityTaskStartedEvent(
	scheduleEventID int64,
	attempt int32,
	requestID string,
	identity string,
	lastFailureReason string,
	lastFailureDetails []byte,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskStarted)
	event.ActivityTaskStartedEventAttributes = &types.ActivityTaskStartedEventAttributes{
		ScheduledEventID:   scheduleEventID,
		Attempt:            attempt,
		Identity:           identity,
		RequestID:          requestID,
		LastFailureReason:  common.StringPtr(lastFailureReason),
		LastFailureDetails: lastFailureDetails,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskCompletedEvent adds ActivityTaskCompleted event to history
func (b *HistoryBuilder) AddActivityTaskCompletedEvent(scheduleEventID, startedEventID int64,
	request *types.RespondActivityTaskCompletedRequest) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskCompleted)
	event.ActivityTaskCompletedEventAttributes = &types.ActivityTaskCompletedEventAttributes{
		Result:           request.Result,
		ScheduledEventID: scheduleEventID,
		StartedEventID:   startedEventID,
		Identity:         request.Identity,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskFailedEvent adds ActivityTaskFailed event to history
func (b *HistoryBuilder) AddActivityTaskFailedEvent(scheduleEventID, StartedEventID int64,
	request *types.RespondActivityTaskFailedRequest) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskFailed)
	event.ActivityTaskFailedEventAttributes = &types.ActivityTaskFailedEventAttributes{
		Reason:           common.StringPtr(common.StringDefault(request.Reason)),
		Details:          request.Details,
		ScheduledEventID: scheduleEventID,
		StartedEventID:   StartedEventID,
		Identity:         request.Identity,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskTimedOutEvent adds ActivityTaskTimedOut event to history
func (b *HistoryBuilder) AddActivityTaskTimedOutEvent(
	scheduleEventID,
	startedEventID int64,
	timeoutType types.TimeoutType,
	lastHeartBeatDetails []byte,
	lastFailureReason string,
	lastFailureDetail []byte,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskTimedOut)
	event.ActivityTaskTimedOutEventAttributes = &types.ActivityTaskTimedOutEventAttributes{
		ScheduledEventID:   scheduleEventID,
		StartedEventID:     startedEventID,
		TimeoutType:        &timeoutType,
		Details:            lastHeartBeatDetails,
		LastFailureReason:  common.StringPtr(lastFailureReason),
		LastFailureDetails: lastFailureDetail,
	}

	return b.addEventToHistory(event)
}

// AddCompletedWorkflowEvent adds WorkflowExecutionCompleted event to history
func (b *HistoryBuilder) AddCompletedWorkflowEvent(decisionCompletedEventID int64,
	attributes *types.CompleteWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionCompleted)
	event.WorkflowExecutionCompletedEventAttributes = &types.WorkflowExecutionCompletedEventAttributes{
		Result:                       attributes.Result,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
	}

	return b.addEventToHistory(event)
}

// AddFailWorkflowEvent adds WorkflowExecutionFailed event to history
func (b *HistoryBuilder) AddFailWorkflowEvent(decisionCompletedEventID int64,
	attributes *types.FailWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionFailed)
	event.WorkflowExecutionFailedEventAttributes = &types.WorkflowExecutionFailedEventAttributes{
		Reason:                       common.StringPtr(common.StringDefault(attributes.Reason)),
		Details:                      attributes.Details,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
	}

	return b.addEventToHistory(event)
}

// AddTimeoutWorkflowEvent adds WorkflowExecutionTimedout event to history
func (b *HistoryBuilder) AddTimeoutWorkflowEvent() *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionTimedOut)
	event.WorkflowExecutionTimedOutEventAttributes = &types.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: types.TimeoutTypeStartToClose.Ptr(),
	}

	return b.addEventToHistory(event)
}

// AddWorkflowExecutionTerminatedEvent add WorkflowExecutionTerminated event to history
func (b *HistoryBuilder) AddWorkflowExecutionTerminatedEvent(
	reason string,
	details []byte,
	identity string,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionTerminated)
	event.WorkflowExecutionTerminatedEventAttributes = &types.WorkflowExecutionTerminatedEventAttributes{
		Reason:   reason,
		Details:  details,
		Identity: identity,
	}

	return b.addEventToHistory(event)
}

// AddContinuedAsNewEvent adds WorkflowExecutionContinuedAsNew event to history
func (b *HistoryBuilder) AddContinuedAsNewEvent(decisionCompletedEventID int64, newRunID string,
	attributes *types.ContinueAsNewWorkflowExecutionDecisionAttributes) *types.HistoryEvent {

	initiator := attributes.Initiator
	if initiator == nil {
		initiator = types.ContinueAsNewInitiatorDecider.Ptr()
	}

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionContinuedAsNew)
	event.WorkflowExecutionContinuedAsNewEventAttributes = &types.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunID:                   newRunID,
		WorkflowType:                        attributes.WorkflowType,
		TaskList:                            attributes.TaskList,
		Header:                              attributes.Header,
		Input:                               attributes.Input,
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      attributes.TaskStartToCloseTimeoutSeconds,
		DecisionTaskCompletedEventID:        decisionCompletedEventID,
		BackoffStartIntervalInSeconds:       common.Int32Ptr(attributes.GetBackoffStartIntervalInSeconds()),
		Initiator:                           initiator,
		FailureReason:                       attributes.FailureReason,
		FailureDetails:                      attributes.FailureDetails,
		LastCompletionResult:                attributes.LastCompletionResult,
		Memo:                                attributes.Memo,
		SearchAttributes:                    attributes.SearchAttributes,
		JitterStartSeconds:                  attributes.JitterStartSeconds,
		CronOverlapPolicy:                   attributes.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:        attributes.ActiveClusterSelectionPolicy,
	}

	return b.addEventToHistory(event)
}

// AddTimerStartedEvent adds TimerStart event to history
func (b *HistoryBuilder) AddTimerStartedEvent(decisionCompletedEventID int64,
	request *types.StartTimerDecisionAttributes) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeTimerStarted)
	event.TimerStartedEventAttributes = &types.TimerStartedEventAttributes{
		TimerID:                      request.TimerID,
		StartToFireTimeoutSeconds:    request.StartToFireTimeoutSeconds,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
	}

	return b.addEventToHistory(event)
}

// AddTimerFiredEvent adds TimerFired event to history
func (b *HistoryBuilder) AddTimerFiredEvent(
	StartedEventID int64,
	TimerID string,
) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeTimerFired)
	event.TimerFiredEventAttributes = &types.TimerFiredEventAttributes{
		TimerID:        TimerID,
		StartedEventID: StartedEventID,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskCancelRequestedEvent add ActivityTaskCancelRequested event to history
func (b *HistoryBuilder) AddActivityTaskCancelRequestedEvent(decisionCompletedEventID int64,
	activityID string) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskCancelRequested)
	event.ActivityTaskCancelRequestedEventAttributes = &types.ActivityTaskCancelRequestedEventAttributes{
		ActivityID:                   activityID,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
	}

	return b.addEventToHistory(event)
}

// AddRequestCancelActivityTaskFailedEvent add RequestCancelActivityTaskFailed event to history
func (b *HistoryBuilder) AddRequestCancelActivityTaskFailedEvent(decisionCompletedEventID int64,
	activityID string, cause string) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeRequestCancelActivityTaskFailed)
	event.RequestCancelActivityTaskFailedEventAttributes = &types.RequestCancelActivityTaskFailedEventAttributes{
		ActivityID:                   activityID,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
		Cause:                        cause,
	}

	return b.addEventToHistory(event)
}

// AddActivityTaskCanceledEvent adds ActivityTaskCanceled event to history
func (b *HistoryBuilder) AddActivityTaskCanceledEvent(scheduleEventID, startedEventID int64,
	latestCancelRequestedEventID int64, details []byte, identity string) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeActivityTaskCanceled)
	event.ActivityTaskCanceledEventAttributes = &types.ActivityTaskCanceledEventAttributes{
		ScheduledEventID:             scheduleEventID,
		StartedEventID:               startedEventID,
		LatestCancelRequestedEventID: latestCancelRequestedEventID,
		Details:                      details,
		Identity:                     identity,
	}

	return b.addEventToHistory(event)
}

// AddTimerCanceledEvent adds TimerCanceled event to history
func (b *HistoryBuilder) AddTimerCanceledEvent(startedEventID int64,
	decisionTaskCompletedEventID int64, timerID string, identity string) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeTimerCanceled)
	event.TimerCanceledEventAttributes = &types.TimerCanceledEventAttributes{
		StartedEventID:               startedEventID,
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		TimerID:                      timerID,
		Identity:                     identity,
	}

	return b.addEventToHistory(event)
}

// AddCancelTimerFailedEvent adds CancelTimerFailed event to history
func (b *HistoryBuilder) AddCancelTimerFailedEvent(timerID string, decisionTaskCompletedEventID int64,
	cause string, identity string) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeCancelTimerFailed)
	event.CancelTimerFailedEventAttributes = &types.CancelTimerFailedEventAttributes{
		TimerID:                      timerID,
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		Cause:                        cause,
		Identity:                     identity,
	}

	return b.addEventToHistory(event)
}

// AddWorkflowExecutionCancelRequestedEvent adds WorkflowExecutionCancelRequested event to history
func (b *HistoryBuilder) AddWorkflowExecutionCancelRequestedEvent(
	cause string,
	request *types.HistoryRequestCancelWorkflowExecutionRequest,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionCancelRequested)
	event.WorkflowExecutionCancelRequestedEventAttributes = &types.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                     cause,
		Identity:                  request.CancelRequest.Identity,
		ExternalInitiatedEventID:  request.ExternalInitiatedEventID,
		ExternalWorkflowExecution: request.ExternalWorkflowExecution,
		RequestID:                 request.CancelRequest.RequestID,
	}

	return b.addEventToHistory(event)
}

// AddWorkflowExecutionCanceledEvent adds WorkflowExecutionCanceled event to history
func (b *HistoryBuilder) AddWorkflowExecutionCanceledEvent(decisionTaskCompletedEventID int64,
	attributes *types.CancelWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionCanceled)
	event.WorkflowExecutionCanceledEventAttributes = &types.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		Details:                      attributes.Details,
	}

	return b.addEventToHistory(event)
}

// AddRequestCancelExternalWorkflowExecutionInitiatedEvent adds RequestCancelExternalWorkflowExecutionInitiated event to history
func (b *HistoryBuilder) AddRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionTaskCompletedEventID int64,
	request *types.RequestCancelExternalWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeRequestCancelExternalWorkflowExecutionInitiated)
	event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes = &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		Domain:                       request.Domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: request.WorkflowID,
			RunID:      request.RunID,
		},
		Control:           request.Control,
		ChildWorkflowOnly: request.ChildWorkflowOnly,
	}

	return b.addEventToHistory(event)
}

// AddRequestCancelExternalWorkflowExecutionFailedEvent adds RequestCancelExternalWorkflowExecutionFailed event to history
func (b *HistoryBuilder) AddRequestCancelExternalWorkflowExecutionFailedEvent(decisionTaskCompletedEventID, initiatedEventID int64,
	domain, workflowID, runID string, cause types.CancelExternalWorkflowExecutionFailedCause) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeRequestCancelExternalWorkflowExecutionFailed)
	event.RequestCancelExternalWorkflowExecutionFailedEventAttributes = &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		InitiatedEventID:             initiatedEventID,
		Domain:                       domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		Cause: cause.Ptr(),
	}

	return b.addEventToHistory(event)
}

// AddExternalWorkflowExecutionCancelRequested adds ExternalWorkflowExecutionCancelRequested event to history
func (b *HistoryBuilder) AddExternalWorkflowExecutionCancelRequested(initiatedEventID int64,
	domain, workflowID, runID string) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeExternalWorkflowExecutionCancelRequested)
	event.ExternalWorkflowExecutionCancelRequestedEventAttributes = &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventID: initiatedEventID,
		Domain:           domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
	}

	return b.addEventToHistory(event)
}

// AddSignalExternalWorkflowExecutionInitiatedEvent adds SignalExternalWorkflowExecutionInitiated event to history
func (b *HistoryBuilder) AddSignalExternalWorkflowExecutionInitiatedEvent(decisionTaskCompletedEventID int64,
	attributes *types.SignalExternalWorkflowExecutionDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeSignalExternalWorkflowExecutionInitiated)
	event.SignalExternalWorkflowExecutionInitiatedEventAttributes = &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		Domain:                       attributes.Domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: attributes.Execution.WorkflowID,
			RunID:      attributes.Execution.RunID,
		},
		SignalName:        attributes.GetSignalName(),
		Input:             attributes.Input,
		Control:           attributes.Control,
		ChildWorkflowOnly: attributes.ChildWorkflowOnly,
	}

	return b.addEventToHistory(event)
}

// AddUpsertWorkflowSearchAttributesEvent adds UpsertWorkflowSearchAttributes event to history
func (b *HistoryBuilder) AddUpsertWorkflowSearchAttributesEvent(
	decisionTaskCompletedEventID int64,
	attributes *types.UpsertWorkflowSearchAttributesDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeUpsertWorkflowSearchAttributes)
	event.UpsertWorkflowSearchAttributesEventAttributes = &types.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		SearchAttributes:             attributes.GetSearchAttributes(),
	}

	return b.addEventToHistory(event)
}

// AddSignalExternalWorkflowExecutionFailedEvent adds SignalExternalWorkflowExecutionFailed event to history
func (b *HistoryBuilder) AddSignalExternalWorkflowExecutionFailedEvent(decisionTaskCompletedEventID, initiatedEventID int64,
	domain, workflowID, runID string, control []byte, cause types.SignalExternalWorkflowExecutionFailedCause) *types.HistoryEvent {

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeSignalExternalWorkflowExecutionFailed)
	event.SignalExternalWorkflowExecutionFailedEventAttributes = &types.SignalExternalWorkflowExecutionFailedEventAttributes{
		DecisionTaskCompletedEventID: decisionTaskCompletedEventID,
		InitiatedEventID:             initiatedEventID,
		Domain:                       domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		Cause:   cause.Ptr(),
		Control: control,
	}

	return b.addEventToHistory(event)
}

// AddExternalWorkflowExecutionSignaled adds ExternalWorkflowExecutionSignaled event to history
func (b *HistoryBuilder) AddExternalWorkflowExecutionSignaled(initiatedEventID int64,
	domain, workflowID, runID string, control []byte) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeExternalWorkflowExecutionSignaled)
	event.ExternalWorkflowExecutionSignaledEventAttributes = &types.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventID: initiatedEventID,
		Domain:           domain,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		Control: control,
	}

	return b.addEventToHistory(event)
}

// AddMarkerRecordedEvent adds MarkerRecorded event to history
func (b *HistoryBuilder) AddMarkerRecordedEvent(decisionCompletedEventID int64,
	attributes *types.RecordMarkerDecisionAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeMarkerRecorded)
	event.MarkerRecordedEventAttributes = &types.MarkerRecordedEventAttributes{
		MarkerName:                   attributes.MarkerName,
		Details:                      attributes.Details,
		DecisionTaskCompletedEventID: decisionCompletedEventID,
		Header:                       attributes.Header,
	}

	return b.addEventToHistory(event)
}

// AddWorkflowExecutionSignaledEvent adds WorkflowExecutionSignaled event to history
func (b *HistoryBuilder) AddWorkflowExecutionSignaledEvent(
	signalName string,
	input []byte,
	identity string,
	requestID string,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeWorkflowExecutionSignaled)
	event.WorkflowExecutionSignaledEventAttributes = &types.WorkflowExecutionSignaledEventAttributes{
		SignalName: signalName,
		Input:      input,
		Identity:   identity,
		RequestID:  requestID,
	}

	return b.addEventToHistory(event)
}

// AddStartChildWorkflowExecutionInitiatedEvent adds ChildWorkflowExecutionInitiated event to history
func (b *HistoryBuilder) AddStartChildWorkflowExecutionInitiatedEvent(
	decisionCompletedEventID int64,
	attributes *types.StartChildWorkflowExecutionDecisionAttributes,
	targetDomainName string,
) *types.HistoryEvent {

	domain := attributes.Domain
	if domain == "" {
		domain = targetDomainName
	}

	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeStartChildWorkflowExecutionInitiated)
	event.StartChildWorkflowExecutionInitiatedEventAttributes = &types.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              domain,
		WorkflowID:                          attributes.WorkflowID,
		WorkflowType:                        attributes.WorkflowType,
		TaskList:                            attributes.TaskList,
		Header:                              attributes.Header,
		Input:                               attributes.Input,
		ExecutionStartToCloseTimeoutSeconds: attributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      attributes.TaskStartToCloseTimeoutSeconds,
		Control:                             attributes.Control,
		DecisionTaskCompletedEventID:        decisionCompletedEventID,
		WorkflowIDReusePolicy:               attributes.WorkflowIDReusePolicy,
		RetryPolicy:                         attributes.RetryPolicy,
		CronSchedule:                        attributes.CronSchedule,
		Memo:                                attributes.Memo,
		SearchAttributes:                    attributes.SearchAttributes,
		ParentClosePolicy:                   attributes.GetParentClosePolicy().Ptr(),
		CronOverlapPolicy:                   attributes.CronOverlapPolicy,
		ActiveClusterSelectionPolicy:        attributes.ActiveClusterSelectionPolicy,
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionStartedEvent adds ChildWorkflowExecutionStarted event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionStartedEvent(
	domain string,
	execution *types.WorkflowExecution,
	workflowType *types.WorkflowType,
	initiatedID int64,
	header *types.Header,
) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionStarted)
	event.ChildWorkflowExecutionStartedEventAttributes = &types.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            domain,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		Header:            header,
	}

	return b.addEventToHistory(event)
}

// AddStartChildWorkflowExecutionFailedEvent adds ChildWorkflowExecutionFailed event to history
func (b *HistoryBuilder) AddStartChildWorkflowExecutionFailedEvent(initiatedID int64,
	cause types.ChildWorkflowExecutionFailedCause,
	initiatedEventAttributes *types.StartChildWorkflowExecutionInitiatedEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeStartChildWorkflowExecutionFailed)
	event.StartChildWorkflowExecutionFailedEventAttributes = &types.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       initiatedEventAttributes.Domain,
		WorkflowID:                   initiatedEventAttributes.WorkflowID,
		WorkflowType:                 initiatedEventAttributes.WorkflowType,
		InitiatedEventID:             initiatedID,
		DecisionTaskCompletedEventID: initiatedEventAttributes.DecisionTaskCompletedEventID,
		Control:                      initiatedEventAttributes.Control,
		Cause:                        cause.Ptr(),
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionCompletedEvent adds ChildWorkflowExecutionCompleted event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionCompletedEvent(domain string, execution *types.WorkflowExecution,
	workflowType *types.WorkflowType, initiatedID, startedID int64,
	completedAttributes *types.WorkflowExecutionCompletedEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionCompleted)
	event.ChildWorkflowExecutionCompletedEventAttributes = &types.ChildWorkflowExecutionCompletedEventAttributes{
		Domain:            domain,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		StartedEventID:    startedID,
		Result:            completedAttributes.Result,
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionFailedEvent adds ChildWorkflowExecutionFailed event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionFailedEvent(domain string, execution *types.WorkflowExecution,
	workflowType *types.WorkflowType, initiatedID, startedID int64,
	failedAttributes *types.WorkflowExecutionFailedEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionFailed)
	event.ChildWorkflowExecutionFailedEventAttributes = &types.ChildWorkflowExecutionFailedEventAttributes{
		Domain:            domain,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		StartedEventID:    startedID,
		Reason:            common.StringPtr(common.StringDefault(failedAttributes.Reason)),
		Details:           failedAttributes.Details,
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionCanceledEvent adds ChildWorkflowExecutionCanceled event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionCanceledEvent(domain string, execution *types.WorkflowExecution,
	workflowType *types.WorkflowType, initiatedID, startedID int64,
	canceledAttributes *types.WorkflowExecutionCanceledEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionCanceled)
	event.ChildWorkflowExecutionCanceledEventAttributes = &types.ChildWorkflowExecutionCanceledEventAttributes{
		Domain:            domain,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		StartedEventID:    startedID,
		Details:           canceledAttributes.Details,
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionTerminatedEvent adds ChildWorkflowExecutionTerminated event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionTerminatedEvent(domain string, execution *types.WorkflowExecution,
	workflowType *types.WorkflowType, initiatedID, startedID int64,
	terminatedAttributes *types.WorkflowExecutionTerminatedEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionTerminated)
	event.ChildWorkflowExecutionTerminatedEventAttributes = &types.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            domain,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		StartedEventID:    startedID,
	}

	return b.addEventToHistory(event)
}

// AddChildWorkflowExecutionTimedOutEvent adds ChildWorkflowExecutionTimedOut event to history
func (b *HistoryBuilder) AddChildWorkflowExecutionTimedOutEvent(domain string, execution *types.WorkflowExecution,
	workflowType *types.WorkflowType, initiatedID, startedID int64,
	timedOutAttributes *types.WorkflowExecutionTimedOutEventAttributes) *types.HistoryEvent {
	event := b.msBuilder.CreateNewHistoryEvent(types.EventTypeChildWorkflowExecutionTimedOut)
	event.ChildWorkflowExecutionTimedOutEventAttributes = &types.ChildWorkflowExecutionTimedOutEventAttributes{
		Domain:            domain,
		TimeoutType:       timedOutAttributes.TimeoutType,
		WorkflowExecution: execution,
		WorkflowType:      workflowType,
		InitiatedEventID:  initiatedID,
		StartedEventID:    startedID,
	}

	return b.addEventToHistory(event)
}

func (b *HistoryBuilder) addEventToHistory(event *types.HistoryEvent) *types.HistoryEvent {
	b.history = append(b.history, event)
	return event
}

func (b *HistoryBuilder) addTransientEvent(event *types.HistoryEvent) *types.HistoryEvent {
	b.transientHistory = append(b.transientHistory, event)
	return event
}

func newDecisionTaskScheduledEventWithInfo(eventID, timestamp int64, taskList string, startToCloseTimeoutSeconds int32, attempt int64) *types.HistoryEvent {
	event := createNewHistoryEvent(eventID, types.EventTypeDecisionTaskScheduled, timestamp)
	event.DecisionTaskScheduledEventAttributes = getDecisionTaskScheduledEventAttributes(taskList, startToCloseTimeoutSeconds, attempt)
	return event
}

func newDecisionTaskStartedEventWithInfo(eventID, timestamp int64, scheduledEventID int64, requestID string, identity string) *types.HistoryEvent {
	event := createNewHistoryEvent(eventID, types.EventTypeDecisionTaskStarted, timestamp)
	event.DecisionTaskStartedEventAttributes = getDecisionTaskStartedEventAttributes(scheduledEventID, requestID, identity)
	return event
}

func createNewHistoryEvent(eventID int64, eventType types.EventType, timestamp int64) *types.HistoryEvent {
	return &types.HistoryEvent{
		ID:        eventID,
		Timestamp: common.Int64Ptr(timestamp),
		EventType: eventType.Ptr(),
	}
}

func getDecisionTaskScheduledEventAttributes(taskList string, startToCloseTimeoutSeconds int32, attempt int64) *types.DecisionTaskScheduledEventAttributes {
	return &types.DecisionTaskScheduledEventAttributes{
		TaskList: &types.TaskList{
			Name: taskList,
		},
		StartToCloseTimeoutSeconds: common.Int32Ptr(startToCloseTimeoutSeconds),
		Attempt:                    attempt,
	}
}

func getDecisionTaskStartedEventAttributes(scheduledEventID int64, requestID string, identity string) *types.DecisionTaskStartedEventAttributes {
	return &types.DecisionTaskStartedEventAttributes{
		ScheduledEventID: scheduledEventID,
		Identity:         identity,
		RequestID:        requestID,
	}
}

// GetHistory gets workflow history stored inside history builder
func (b *HistoryBuilder) GetHistory() *types.History {
	history := types.History{Events: b.history}
	return &history
}
