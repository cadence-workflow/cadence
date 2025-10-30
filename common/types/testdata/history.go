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
	"github.com/pborman/uuid"

	"github.com/uber/cadence/.gen/go/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

var (
	History = types.History{
		Events: HistoryEventArray,
	}
	HistoryEventArray = []*types.HistoryEvent{
		&HistoryEvent_WorkflowExecutionStarted,
		&HistoryEvent_WorkflowExecutionStarted,
		&HistoryEvent_WorkflowExecutionCompleted,
		&HistoryEvent_WorkflowExecutionFailed,
		&HistoryEvent_WorkflowExecutionTimedOut,
		&HistoryEvent_DecisionTaskScheduled,
		&HistoryEvent_DecisionTaskStarted,
		&HistoryEvent_DecisionTaskCompleted,
		&HistoryEvent_DecisionTaskTimedOut,
		&HistoryEvent_DecisionTaskFailed,
		&HistoryEvent_ActivityTaskScheduled,
		&HistoryEvent_ActivityTaskStarted,
		&HistoryEvent_ActivityTaskCompleted,
		&HistoryEvent_ActivityTaskFailed,
		&HistoryEvent_ActivityTaskTimedOut,
		&HistoryEvent_ActivityTaskCancelRequested,
		&HistoryEvent_RequestCancelActivityTaskFailed,
		&HistoryEvent_ActivityTaskCanceled,
		&HistoryEvent_TimerStarted,
		&HistoryEvent_TimerFired,
		&HistoryEvent_CancelTimerFailed,
		&HistoryEvent_TimerCanceled,
		&HistoryEvent_WorkflowExecutionCancelRequested,
		&HistoryEvent_WorkflowExecutionCanceled,
		&HistoryEvent_RequestCancelExternalWorkflowExecutionInitiated,
		&HistoryEvent_RequestCancelExternalWorkflowExecutionFailed,
		&HistoryEvent_ExternalWorkflowExecutionCancelRequested,
		&HistoryEvent_MarkerRecorded,
		&HistoryEvent_WorkflowExecutionSignaled,
		&HistoryEvent_WorkflowExecutionTerminated,
		&HistoryEvent_WorkflowExecutionContinuedAsNew,
		&HistoryEvent_StartChildWorkflowExecutionInitiated,
		&HistoryEvent_StartChildWorkflowExecutionFailed,
		&HistoryEvent_ChildWorkflowExecutionStarted,
		&HistoryEvent_ChildWorkflowExecutionCompleted,
		&HistoryEvent_ChildWorkflowExecutionFailed,
		&HistoryEvent_ChildWorkflowExecutionCanceled,
		&HistoryEvent_ChildWorkflowExecutionTimedOut,
		&HistoryEvent_ChildWorkflowExecutionTerminated,
		&HistoryEvent_SignalExternalWorkflowExecutionInitiated,
		&HistoryEvent_SignalExternalWorkflowExecutionFailed,
		&HistoryEvent_ExternalWorkflowExecutionSignaled,
		&HistoryEvent_UpsertWorkflowSearchAttributes,
	}

	HistoryEvent_WorkflowExecutionStarted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionStarted.Ptr()
		e.WorkflowExecutionStartedEventAttributes = &WorkflowExecutionStartedEventAttributes
	})
	HistoryEvent_WorkflowExecutionCompleted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionCompleted.Ptr()
		e.WorkflowExecutionCompletedEventAttributes = &WorkflowExecutionCompletedEventAttributes
	})
	HistoryEvent_WorkflowExecutionFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionFailed.Ptr()
		e.WorkflowExecutionFailedEventAttributes = &WorkflowExecutionFailedEventAttributes
	})
	HistoryEvent_WorkflowExecutionTimedOut = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionTimedOut.Ptr()
		e.WorkflowExecutionTimedOutEventAttributes = &WorkflowExecutionTimedOutEventAttributes
	})
	HistoryEvent_DecisionTaskScheduled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeDecisionTaskScheduled.Ptr()
		e.DecisionTaskScheduledEventAttributes = &DecisionTaskScheduledEventAttributes
	})
	HistoryEvent_DecisionTaskStarted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeDecisionTaskStarted.Ptr()
		e.DecisionTaskStartedEventAttributes = &DecisionTaskStartedEventAttributes
	})
	HistoryEvent_DecisionTaskCompleted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeDecisionTaskCompleted.Ptr()
		e.DecisionTaskCompletedEventAttributes = &DecisionTaskCompletedEventAttributes
	})
	HistoryEvent_DecisionTaskTimedOut = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeDecisionTaskTimedOut.Ptr()
		e.DecisionTaskTimedOutEventAttributes = &DecisionTaskTimedOutEventAttributes
	})
	HistoryEvent_DecisionTaskFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeDecisionTaskFailed.Ptr()
		e.DecisionTaskFailedEventAttributes = &DecisionTaskFailedEventAttributes
	})
	HistoryEvent_ActivityTaskScheduled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskScheduled.Ptr()
		e.ActivityTaskScheduledEventAttributes = &ActivityTaskScheduledEventAttributes
	})
	HistoryEvent_ActivityTaskStarted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskStarted.Ptr()
		e.ActivityTaskStartedEventAttributes = &ActivityTaskStartedEventAttributes
	})
	HistoryEvent_ActivityTaskCompleted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskCompleted.Ptr()
		e.ActivityTaskCompletedEventAttributes = &ActivityTaskCompletedEventAttributes
	})
	HistoryEvent_ActivityTaskFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskFailed.Ptr()
		e.ActivityTaskFailedEventAttributes = &ActivityTaskFailedEventAttributes
	})
	HistoryEvent_ActivityTaskTimedOut = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskTimedOut.Ptr()
		e.ActivityTaskTimedOutEventAttributes = &ActivityTaskTimedOutEventAttributes
	})
	HistoryEvent_ActivityTaskCancelRequested = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskCancelRequested.Ptr()
		e.ActivityTaskCancelRequestedEventAttributes = &ActivityTaskCancelRequestedEventAttributes
	})
	HistoryEvent_RequestCancelActivityTaskFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeRequestCancelActivityTaskFailed.Ptr()
		e.RequestCancelActivityTaskFailedEventAttributes = &RequestCancelActivityTaskFailedEventAttributes
	})
	HistoryEvent_ActivityTaskCanceled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeActivityTaskCanceled.Ptr()
		e.ActivityTaskCanceledEventAttributes = &ActivityTaskCanceledEventAttributes
	})
	HistoryEvent_TimerStarted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeTimerStarted.Ptr()
		e.TimerStartedEventAttributes = &TimerStartedEventAttributes
	})
	HistoryEvent_TimerFired = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeTimerFired.Ptr()
		e.TimerFiredEventAttributes = &TimerFiredEventAttributes
	})
	HistoryEvent_CancelTimerFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeCancelTimerFailed.Ptr()
		e.CancelTimerFailedEventAttributes = &CancelTimerFailedEventAttributes
	})
	HistoryEvent_TimerCanceled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeTimerCanceled.Ptr()
		e.TimerCanceledEventAttributes = &TimerCanceledEventAttributes
	})
	HistoryEvent_WorkflowExecutionCancelRequested = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionCancelRequested.Ptr()
		e.WorkflowExecutionCancelRequestedEventAttributes = &WorkflowExecutionCancelRequestedEventAttributes
	})
	HistoryEvent_WorkflowExecutionCanceled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionCanceled.Ptr()
		e.WorkflowExecutionCanceledEventAttributes = &WorkflowExecutionCanceledEventAttributes
	})
	HistoryEvent_RequestCancelExternalWorkflowExecutionInitiated = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeRequestCancelExternalWorkflowExecutionInitiated.Ptr()
		e.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes = &RequestCancelExternalWorkflowExecutionInitiatedEventAttributes
	})
	HistoryEvent_RequestCancelExternalWorkflowExecutionFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeRequestCancelExternalWorkflowExecutionFailed.Ptr()
		e.RequestCancelExternalWorkflowExecutionFailedEventAttributes = &RequestCancelExternalWorkflowExecutionFailedEventAttributes
	})
	HistoryEvent_ExternalWorkflowExecutionCancelRequested = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeExternalWorkflowExecutionCancelRequested.Ptr()
		e.ExternalWorkflowExecutionCancelRequestedEventAttributes = &ExternalWorkflowExecutionCancelRequestedEventAttributes
	})
	HistoryEvent_MarkerRecorded = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeMarkerRecorded.Ptr()
		e.MarkerRecordedEventAttributes = &MarkerRecordedEventAttributes
	})
	HistoryEvent_WorkflowExecutionSignaled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionSignaled.Ptr()
		e.WorkflowExecutionSignaledEventAttributes = &WorkflowExecutionSignaledEventAttributes
	})
	HistoryEvent_WorkflowExecutionTerminated = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionTerminated.Ptr()
		e.WorkflowExecutionTerminatedEventAttributes = &WorkflowExecutionTerminatedEventAttributes
	})
	HistoryEvent_WorkflowExecutionContinuedAsNew = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeWorkflowExecutionContinuedAsNew.Ptr()
		e.WorkflowExecutionContinuedAsNewEventAttributes = &WorkflowExecutionContinuedAsNewEventAttributes
	})
	HistoryEvent_StartChildWorkflowExecutionInitiated = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeStartChildWorkflowExecutionInitiated.Ptr()
		e.StartChildWorkflowExecutionInitiatedEventAttributes = &StartChildWorkflowExecutionInitiatedEventAttributes
	})
	HistoryEvent_StartChildWorkflowExecutionFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeStartChildWorkflowExecutionFailed.Ptr()
		e.StartChildWorkflowExecutionFailedEventAttributes = &StartChildWorkflowExecutionFailedEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionStarted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionStarted.Ptr()
		e.ChildWorkflowExecutionStartedEventAttributes = &ChildWorkflowExecutionStartedEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionCompleted = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionCompleted.Ptr()
		e.ChildWorkflowExecutionCompletedEventAttributes = &ChildWorkflowExecutionCompletedEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionFailed.Ptr()
		e.ChildWorkflowExecutionFailedEventAttributes = &ChildWorkflowExecutionFailedEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionCanceled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionCanceled.Ptr()
		e.ChildWorkflowExecutionCanceledEventAttributes = &ChildWorkflowExecutionCanceledEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionTimedOut = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionTimedOut.Ptr()
		e.ChildWorkflowExecutionTimedOutEventAttributes = &ChildWorkflowExecutionTimedOutEventAttributes
	})
	HistoryEvent_ChildWorkflowExecutionTerminated = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeChildWorkflowExecutionTerminated.Ptr()
		e.ChildWorkflowExecutionTerminatedEventAttributes = &ChildWorkflowExecutionTerminatedEventAttributes
	})
	HistoryEvent_SignalExternalWorkflowExecutionInitiated = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeSignalExternalWorkflowExecutionInitiated.Ptr()
		e.SignalExternalWorkflowExecutionInitiatedEventAttributes = &SignalExternalWorkflowExecutionInitiatedEventAttributes
	})
	HistoryEvent_SignalExternalWorkflowExecutionFailed = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeSignalExternalWorkflowExecutionFailed.Ptr()
		e.SignalExternalWorkflowExecutionFailedEventAttributes = &SignalExternalWorkflowExecutionFailedEventAttributes
	})
	HistoryEvent_ExternalWorkflowExecutionSignaled = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeExternalWorkflowExecutionSignaled.Ptr()
		e.ExternalWorkflowExecutionSignaledEventAttributes = &ExternalWorkflowExecutionSignaledEventAttributes
	})
	HistoryEvent_UpsertWorkflowSearchAttributes = generateEvent(func(e *types.HistoryEvent) {
		e.EventType = types.EventTypeUpsertWorkflowSearchAttributes.Ptr()
		e.UpsertWorkflowSearchAttributesEventAttributes = &UpsertWorkflowSearchAttributesEventAttributes
	})

	WorkflowExecutionStartedEventAttributes = types.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        &WorkflowType,
		ParentWorkflowDomainID:              common.StringPtr(DomainID),
		ParentWorkflowDomain:                common.StringPtr(DomainName),
		ParentWorkflowExecution:             &WorkflowExecution,
		ParentInitiatedEventID:              common.Int64Ptr(EventID1),
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		ContinuedExecutionRunID:             RunID1,
		Initiator:                           &ContinueAsNewInitiator,
		ContinuedFailureReason:              &FailureReason,
		ContinuedFailureDetails:             FailureDetails,
		LastCompletionResult:                Payload2,
		OriginalExecutionRunID:              RunID2,
		Identity:                            Identity,
		FirstExecutionRunID:                 RunID3,
		RetryPolicy:                         &RetryPolicy,
		Attempt:                             Attempt,
		ExpirationTimestamp:                 &Timestamp1,
		CronSchedule:                        CronSchedule,
		FirstDecisionTaskBackoffSeconds:     &Duration3,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		PrevAutoResetPoints:                 &ResetPoints,
		Header:                              &Header,
		PartitionConfig:                     PartitionConfig,
		RequestID:                           RequestID,
		ActiveClusterSelectionPolicy:        &ActiveClusterSelectionPolicyExternalEntity,
		CronOverlapPolicy:                   &CronOverlapPolicy,
	}
	WorkflowExecutionCompletedEventAttributes = types.WorkflowExecutionCompletedEventAttributes{
		Result:                       Payload1,
		DecisionTaskCompletedEventID: EventID1,
	}
	WorkflowExecutionFailedEventAttributes = types.WorkflowExecutionFailedEventAttributes{
		Reason:                       &FailureReason,
		Details:                      FailureDetails,
		DecisionTaskCompletedEventID: EventID1,
	}
	WorkflowExecutionTimedOutEventAttributes = types.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: &TimeoutType,
	}
	DecisionTaskScheduledEventAttributes = types.DecisionTaskScheduledEventAttributes{
		TaskList:                   &TaskList,
		StartToCloseTimeoutSeconds: &Duration1,
		Attempt:                    Attempt,
	}
	DecisionTaskStartedEventAttributes = types.DecisionTaskStartedEventAttributes{
		ScheduledEventID: EventID1,
		Identity:         Identity,
		RequestID:        RequestID,
	}
	DecisionTaskCompletedEventAttributes = types.DecisionTaskCompletedEventAttributes{
		ExecutionContext: ExecutionContext,
		ScheduledEventID: EventID1,
		StartedEventID:   EventID2,
		Identity:         Identity,
		BinaryChecksum:   Checksum,
	}
	DecisionTaskTimedOutEventAttributes = types.DecisionTaskTimedOutEventAttributes{
		ScheduledEventID: EventID1,
		StartedEventID:   EventID2,
		TimeoutType:      &TimeoutType,
		BaseRunID:        RunID1,
		NewRunID:         RunID2,
		ForkEventVersion: Version1,
		Reason:           Reason,
		Cause:            &DecisionTaskTimedOutCause,
		RequestID:        RequestID,
	}
	DecisionTaskFailedEventAttributes = types.DecisionTaskFailedEventAttributes{
		ScheduledEventID: EventID1,
		StartedEventID:   EventID2,
		Cause:            &DecisionTaskFailedCause,
		Details:          FailureDetails,
		Identity:         Identity,
		Reason:           &FailureReason,
		BaseRunID:        RunID1,
		NewRunID:         RunID2,
		ForkEventVersion: Version1,
		BinaryChecksum:   Checksum,
		RequestID:        RequestID,
	}
	ActivityTaskScheduledEventAttributes = types.ActivityTaskScheduledEventAttributes{
		ActivityID:                    ActivityID,
		ActivityType:                  &ActivityType,
		Domain:                        common.StringPtr(DomainName),
		TaskList:                      &TaskList,
		Input:                         Payload1,
		ScheduleToCloseTimeoutSeconds: &Duration1,
		ScheduleToStartTimeoutSeconds: &Duration2,
		StartToCloseTimeoutSeconds:    &Duration3,
		HeartbeatTimeoutSeconds:       &Duration4,
		DecisionTaskCompletedEventID:  EventID1,
		RetryPolicy:                   &RetryPolicy,
		Header:                        &Header,
	}
	ActivityTaskStartedEventAttributes = types.ActivityTaskStartedEventAttributes{
		ScheduledEventID:   EventID1,
		Identity:           Identity,
		RequestID:          RequestID,
		Attempt:            Attempt,
		LastFailureReason:  &FailureReason,
		LastFailureDetails: FailureDetails,
	}
	ActivityTaskCompletedEventAttributes = types.ActivityTaskCompletedEventAttributes{
		Result:           Payload1,
		ScheduledEventID: EventID1,
		StartedEventID:   EventID2,
		Identity:         Identity,
	}
	ActivityTaskFailedEventAttributes = types.ActivityTaskFailedEventAttributes{
		Reason:           &FailureReason,
		Details:          FailureDetails,
		ScheduledEventID: EventID1,
		StartedEventID:   EventID2,
		Identity:         Identity,
	}
	ActivityTaskTimedOutEventAttributes = types.ActivityTaskTimedOutEventAttributes{
		Details:            Payload1,
		ScheduledEventID:   EventID1,
		StartedEventID:     EventID2,
		TimeoutType:        &TimeoutType,
		LastFailureReason:  &FailureReason,
		LastFailureDetails: FailureDetails,
	}
	TimerStartedEventAttributes = types.TimerStartedEventAttributes{
		TimerID:                      TimerID,
		StartToFireTimeoutSeconds:    common.Int64Ptr(int64(Duration1)),
		DecisionTaskCompletedEventID: EventID1,
	}
	TimerFiredEventAttributes = types.TimerFiredEventAttributes{
		TimerID:        TimerID,
		StartedEventID: EventID1,
	}
	ActivityTaskCancelRequestedEventAttributes = types.ActivityTaskCancelRequestedEventAttributes{
		ActivityID:                   ActivityID,
		DecisionTaskCompletedEventID: EventID1,
	}
	RequestCancelActivityTaskFailedEventAttributes = types.RequestCancelActivityTaskFailedEventAttributes{
		ActivityID:                   ActivityID,
		Cause:                        Cause,
		DecisionTaskCompletedEventID: EventID1,
	}
	ActivityTaskCanceledEventAttributes = types.ActivityTaskCanceledEventAttributes{
		Details:                      Payload1,
		LatestCancelRequestedEventID: EventID1,
		ScheduledEventID:             EventID2,
		StartedEventID:               EventID3,
		Identity:                     Identity,
	}
	TimerCanceledEventAttributes = types.TimerCanceledEventAttributes{
		TimerID:                      TimerID,
		StartedEventID:               EventID1,
		DecisionTaskCompletedEventID: EventID2,
		Identity:                     Identity,
	}
	CancelTimerFailedEventAttributes = types.CancelTimerFailedEventAttributes{
		TimerID:                      TimerID,
		Cause:                        Cause,
		DecisionTaskCompletedEventID: EventID1,
		Identity:                     Identity,
	}
	MarkerRecordedEventAttributes = types.MarkerRecordedEventAttributes{
		MarkerName:                   MarkerName,
		Details:                      Payload1,
		DecisionTaskCompletedEventID: EventID1,
		Header:                       &Header,
	}
	WorkflowExecutionSignaledEventAttributes = types.WorkflowExecutionSignaledEventAttributes{
		SignalName: SignalName,
		Input:      Payload1,
		Identity:   Identity,
		RequestID:  RequestID,
	}
	WorkflowExecutionTerminatedEventAttributes = types.WorkflowExecutionTerminatedEventAttributes{
		Reason:   Reason,
		Details:  Payload1,
		Identity: Identity,
	}
	WorkflowExecutionCancelRequestedEventAttributes = types.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                     Cause,
		ExternalInitiatedEventID:  common.Int64Ptr(EventID1),
		ExternalWorkflowExecution: &WorkflowExecution,
		Identity:                  Identity,
		RequestID:                 RequestID,
	}
	WorkflowExecutionCanceledEventAttributes = types.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventID: EventID1,
		Details:                      Payload1,
	}
	RequestCancelExternalWorkflowExecutionInitiatedEventAttributes = types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: EventID1,
		Domain:                       DomainName,
		WorkflowExecution:            &WorkflowExecution,
		Control:                      Control,
		ChildWorkflowOnly:            true,
	}
	RequestCancelExternalWorkflowExecutionFailedEventAttributes = types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        &CancelExternalWorkflowExecutionFailedCause,
		DecisionTaskCompletedEventID: EventID1,
		Domain:                       DomainName,
		WorkflowExecution:            &WorkflowExecution,
		InitiatedEventID:             EventID2,
		Control:                      Control,
	}
	ExternalWorkflowExecutionCancelRequestedEventAttributes = types.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventID:  EventID1,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
	}
	WorkflowExecutionContinuedAsNewEventAttributes = types.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunID:                   RunID1,
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		DecisionTaskCompletedEventID:        EventID1,
		BackoffStartIntervalInSeconds:       &Duration3,
		Initiator:                           &ContinueAsNewInitiator,
		FailureReason:                       &FailureReason,
		FailureDetails:                      FailureDetails,
		LastCompletionResult:                Payload2,
		Header:                              &Header,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		ActiveClusterSelectionPolicy:        &ActiveClusterSelectionPolicyExternalEntity,
		CronOverlapPolicy:                   &CronOverlapPolicy,
	}
	StartChildWorkflowExecutionInitiatedEventAttributes = types.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              DomainName,
		WorkflowID:                          WorkflowID,
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		ParentClosePolicy:                   &ParentClosePolicy,
		Control:                             Control,
		DecisionTaskCompletedEventID:        EventID1,
		WorkflowIDReusePolicy:               &WorkflowIDReusePolicy,
		RetryPolicy:                         &RetryPolicy,
		CronSchedule:                        CronSchedule,
		Header:                              &Header,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		DelayStartSeconds:                   &Duration3,
		JitterStartSeconds:                  &Duration4,
		FirstRunAtTimestamp:                 &Timestamp1,
		CronOverlapPolicy:                   &CronOverlapPolicy,
	}
	StartChildWorkflowExecutionFailedEventAttributes = types.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       DomainName,
		WorkflowID:                   WorkflowID,
		WorkflowType:                 &WorkflowType,
		Cause:                        &ChildWorkflowExecutionFailedCause,
		Control:                      Control,
		InitiatedEventID:             EventID1,
		DecisionTaskCompletedEventID: EventID2,
	}
	ChildWorkflowExecutionStartedEventAttributes = types.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            DomainName,
		InitiatedEventID:  EventID1,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		Header:            &Header,
	}
	ChildWorkflowExecutionCompletedEventAttributes = types.ChildWorkflowExecutionCompletedEventAttributes{
		Result:            Payload1,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		InitiatedEventID:  EventID1,
		StartedEventID:    EventID2,
	}
	ChildWorkflowExecutionFailedEventAttributes = types.ChildWorkflowExecutionFailedEventAttributes{
		Reason:            &FailureReason,
		Details:           FailureDetails,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		InitiatedEventID:  EventID1,
		StartedEventID:    EventID2,
	}
	ChildWorkflowExecutionCanceledEventAttributes = types.ChildWorkflowExecutionCanceledEventAttributes{
		Details:           Payload1,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		InitiatedEventID:  EventID1,
		StartedEventID:    EventID2,
	}
	ChildWorkflowExecutionTimedOutEventAttributes = types.ChildWorkflowExecutionTimedOutEventAttributes{
		TimeoutType:       &TimeoutType,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		InitiatedEventID:  EventID1,
		StartedEventID:    EventID2,
	}
	ChildWorkflowExecutionTerminatedEventAttributes = types.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		WorkflowType:      &WorkflowType,
		InitiatedEventID:  EventID1,
		StartedEventID:    EventID2,
	}
	SignalExternalWorkflowExecutionInitiatedEventAttributes = types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: EventID1,
		Domain:                       DomainName,
		WorkflowExecution:            &WorkflowExecution,
		SignalName:                   SignalName,
		Input:                        Payload1,
		Control:                      Control,
		ChildWorkflowOnly:            true,
	}
	SignalExternalWorkflowExecutionFailedEventAttributes = types.SignalExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        &SignalExternalWorkflowExecutionFailedCause,
		DecisionTaskCompletedEventID: EventID1,
		Domain:                       DomainName,
		WorkflowExecution:            &WorkflowExecution,
		InitiatedEventID:             EventID2,
		Control:                      Control,
	}
	ExternalWorkflowExecutionSignaledEventAttributes = types.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventID:  EventID1,
		Domain:            DomainName,
		WorkflowExecution: &WorkflowExecution,
		Control:           Control,
	}
	UpsertWorkflowSearchAttributesEventAttributes = types.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventID: EventID1,
		SearchAttributes:             &SearchAttributes,
	}
	GetFailoverInfoRequest = types.GetFailoverInfoRequest{
		DomainID: uuid.NewUUID().String(),
	}
	GetFailoverInfoResponse = types.GetFailoverInfoResponse{
		CompletedShardCount: 0,
		PendingShards:       []int32{1, 2, 3},
	}
	RatelimitUpdateRequest = types.RatelimitUpdateRequest{
		Any: &types.Any{
			ValueType: history.WeightedRatelimitUsageAnyType, // only correct value currently
			Value:     []byte("test request"),                // invalid contents, but not inspected in these tests
		},
	}
	RatelimitUpdateResponse = types.RatelimitUpdateResponse{
		Any: &types.Any{
			ValueType: history.WeightedRatelimitQuotasAnyType, // only correct value currently
			Value:     []byte("test response"),                // invalid contents, but not inspected in these tests
		},
	}
)

func generateEvent(modifier func(e *types.HistoryEvent)) types.HistoryEvent {
	e := types.HistoryEvent{
		ID:        EventID1,
		Timestamp: &Timestamp1,
		Version:   Version1,
		TaskID:    TaskID,
	}
	modifier(&e)
	return e
}
