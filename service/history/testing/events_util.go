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

package testing

import (
	"github.com/uber/cadence/common"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
)

// AddWorkflowExecutionStartedEventWithParent adds WorkflowExecutionStarted event with parent workflow info
func AddWorkflowExecutionStartedEventWithParent(
	builder execution.MutableState,
	workflowExecution types.WorkflowExecution,
	workflowType string,
	taskList string,
	input []byte,
	executionStartToCloseTimeout,
	taskStartToCloseTimeout int32,
	parentInfo *types.ParentExecutionInfo,
	identity string,
) *types.HistoryEvent {

	startRequest := &types.StartWorkflowExecutionRequest{
		WorkflowID:                          workflowExecution.WorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: workflowType},
		TaskList:                            &types.TaskList{Name: taskList},
		Input:                               input,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeout),
		Identity:                            identity,
	}

	event, _ := builder.AddWorkflowExecutionStartedEvent(
		workflowExecution,
		&types.HistoryStartWorkflowExecutionRequest{
			DomainUUID:          constants.TestDomainID,
			StartRequest:        startRequest,
			ParentExecutionInfo: parentInfo,
		},
	)

	return event
}

// AddWorkflowExecutionStartedEvent adds WorkflowExecutionStarted event
func AddWorkflowExecutionStartedEvent(
	builder execution.MutableState,
	workflowExecution types.WorkflowExecution,
	workflowType string,
	taskList string,
	input []byte,
	executionStartToCloseTimeout int32,
	taskStartToCloseTimeout int32,
	identity string,
) *types.HistoryEvent {
	return AddWorkflowExecutionStartedEventWithParent(builder, workflowExecution, workflowType, taskList, input,
		executionStartToCloseTimeout, taskStartToCloseTimeout, nil, identity)
}

// AddDecisionTaskScheduledEvent adds DecisionTaskScheduled event
func AddDecisionTaskScheduledEvent(
	builder execution.MutableState,
) *execution.DecisionInfo {
	di, _ := builder.AddDecisionTaskScheduledEvent(false)
	return di
}

// AddDecisionTaskStartedEvent adds DecisionTaskStarted event
func AddDecisionTaskStartedEvent(
	builder execution.MutableState,
	scheduleID int64,
	taskList string,
	identity string,
) *types.HistoryEvent {
	return AddDecisionTaskStartedEventWithRequestID(builder, scheduleID, constants.TestRunID, taskList, identity)
}

// AddDecisionTaskStartedEventWithRequestID adds DecisionTaskStarted event with requestID
func AddDecisionTaskStartedEventWithRequestID(
	builder execution.MutableState,
	scheduleID int64,
	requestID string,
	taskList string,
	identity string,
) *types.HistoryEvent {
	event, _, _ := builder.AddDecisionTaskStartedEvent(scheduleID, requestID, &types.PollForDecisionTaskRequest{
		TaskList: &types.TaskList{Name: taskList},
		Identity: identity,
	})

	return event
}

// AddDecisionTaskCompletedEvent adds DecisionTaskCompleted event
func AddDecisionTaskCompletedEvent(
	builder execution.MutableState,
	scheduleID int64,
	startedID int64,
	context []byte,
	identity string,
) *types.HistoryEvent {
	event, _ := builder.AddDecisionTaskCompletedEvent(scheduleID, startedID, &types.RespondDecisionTaskCompletedRequest{
		ExecutionContext: context,
		Identity:         identity,
	}, commonconstants.DefaultHistoryMaxAutoResetPoints)

	builder.FlushBufferedEvents() //nolint:errcheck

	return event
}

// AddActivityTaskScheduledEvent adds ActivityTaskScheduled event
func AddActivityTaskScheduledEvent(
	builder execution.MutableState,
	decisionCompletedID int64,
	activityID string,
	activityType string,
	taskList string,
	input []byte,
	scheduleToCloseTimeout int32,
	scheduleToStartTimeout int32,
	startToCloseTimeout int32,
	heartbeatTimeout int32,
) (*types.HistoryEvent,
	*persistence.ActivityInfo) {

	event, ai, _, _, _, _ := builder.AddActivityTaskScheduledEvent(nil, decisionCompletedID, &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    activityID,
		ActivityType:                  &types.ActivityType{Name: activityType},
		TaskList:                      &types.TaskList{Name: taskList, Kind: types.TaskListKindNormal.Ptr()},
		Input:                         input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(scheduleToCloseTimeout),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(scheduleToStartTimeout),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(startToCloseTimeout),
		HeartbeatTimeoutSeconds:       common.Int32Ptr(heartbeatTimeout),
	}, false,
	)

	return event, ai
}

// AddActivityTaskScheduledEventWithRetry adds ActivityTaskScheduled event with retry policy
func AddActivityTaskScheduledEventWithRetry(
	builder execution.MutableState,
	decisionCompletedID int64,
	activityID string,
	activityType string,
	taskList string,
	input []byte,
	scheduleToCloseTimeout int32,
	scheduleToStartTimeout int32,
	startToCloseTimeout int32,
	heartbeatTimeout int32,
	retryPolicy *types.RetryPolicy,
) (*types.HistoryEvent, *persistence.ActivityInfo) {

	event, ai, _, _, _, _ := builder.AddActivityTaskScheduledEvent(nil, decisionCompletedID, &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    activityID,
		ActivityType:                  &types.ActivityType{Name: activityType},
		TaskList:                      &types.TaskList{Name: taskList},
		Input:                         input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(scheduleToCloseTimeout),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(scheduleToStartTimeout),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(startToCloseTimeout),
		HeartbeatTimeoutSeconds:       common.Int32Ptr(heartbeatTimeout),
		RetryPolicy:                   retryPolicy,
	}, false,
	)

	return event, ai
}

// AddActivityTaskStartedEvent adds ActivityTaskStarted event
func AddActivityTaskStartedEvent(
	builder execution.MutableState,
	scheduleID int64,
	identity string,
) *types.HistoryEvent {
	ai, _ := builder.GetActivityInfo(scheduleID)
	event, _ := builder.AddActivityTaskStartedEvent(ai, scheduleID, constants.TestRunID, identity)
	return event
}

// AddActivityTaskCompletedEvent adds ActivityTaskCompleted event
func AddActivityTaskCompletedEvent(
	builder execution.MutableState,
	scheduleID int64,
	startedID int64,
	result []byte,
	identity string,
) *types.HistoryEvent {
	event, _ := builder.AddActivityTaskCompletedEvent(scheduleID, startedID, &types.RespondActivityTaskCompletedRequest{
		Result:   result,
		Identity: identity,
	})

	return event
}

// AddActivityTaskFailedEvent adds ActivityTaskFailed event
func AddActivityTaskFailedEvent(
	builder execution.MutableState,
	scheduleID int64,
	startedID int64,
	reason string,
	details []byte,
	identity string,
) *types.HistoryEvent {
	event, _ := builder.AddActivityTaskFailedEvent(scheduleID, startedID, &types.RespondActivityTaskFailedRequest{
		Reason:   common.StringPtr(reason),
		Details:  details,
		Identity: identity,
	})

	return event
}

// AddTimerStartedEvent adds TimerStarted event
func AddTimerStartedEvent(
	builder execution.MutableState,
	decisionCompletedEventID int64,
	timerID string,
	timeOut int64,
) (*types.HistoryEvent, *persistence.TimerInfo) {
	event, ti, _ := builder.AddTimerStartedEvent(decisionCompletedEventID,
		&types.StartTimerDecisionAttributes{
			TimerID:                   timerID,
			StartToFireTimeoutSeconds: common.Int64Ptr(timeOut),
		})
	return event, ti
}

// AddTimerFiredEvent adds TimerFired event
func AddTimerFiredEvent(
	mutableState execution.MutableState,
	timerID string,
) *types.HistoryEvent {
	event, _ := mutableState.AddTimerFiredEvent(timerID)
	return event
}

// AddRequestCancelInitiatedEvent adds RequestCancelExternalWorkflowExecutionInitiated event
func AddRequestCancelInitiatedEvent(
	builder execution.MutableState,
	decisionCompletedEventID int64,
	cancelRequestID string,
	domain string,
	workflowID string,
	runID string,
) (*types.HistoryEvent, *persistence.RequestCancelInfo) {
	event, rci, _ := builder.AddRequestCancelExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID,
		cancelRequestID, &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
			Domain:     domain,
			WorkflowID: workflowID,
			RunID:      runID,
		})

	return event, rci
}

// AddCancelRequestedEvent adds ExternalWorkflowExecutionCancelRequested event
func AddCancelRequestedEvent(
	builder execution.MutableState,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
) *types.HistoryEvent {
	event, _ := builder.AddExternalWorkflowExecutionCancelRequested(initiatedID, domain, workflowID, runID)
	return event
}

// AddRequestSignalInitiatedEvent adds SignalExternalWorkflowExecutionInitiated event
func AddRequestSignalInitiatedEvent(
	builder execution.MutableState,
	decisionCompletedEventID int64,
	signalRequestID string,
	domain string,
	workflowID string,
	runID string,
	signalName string,
	input []byte,
	control []byte,
) (*types.HistoryEvent, *persistence.SignalInfo) {
	event, si, _ := builder.AddSignalExternalWorkflowExecutionInitiatedEvent(decisionCompletedEventID, signalRequestID,
		&types.SignalExternalWorkflowExecutionDecisionAttributes{
			Domain: domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: workflowID,
				RunID:      runID,
			},
			SignalName: signalName,
			Input:      input,
			Control:    control,
		})

	return event, si
}

// AddSignaledEvent adds ExternalWorkflowExecutionSignaled event
func AddSignaledEvent(
	builder execution.MutableState,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	control []byte,
) *types.HistoryEvent {
	event, _ := builder.AddExternalWorkflowExecutionSignaled(initiatedID, domain, workflowID, runID, control)
	return event
}

// AddStartChildWorkflowExecutionInitiatedEvent adds ChildWorkflowExecutionInitiated event
func AddStartChildWorkflowExecutionInitiatedEvent(
	builder execution.MutableState,
	decisionCompletedID int64,
	createRequestID string,
	domain string,
	workflowID string,
	workflowType string,
	tasklist string,
	input []byte,
	executionStartToCloseTimeout int32,
	taskStartToCloseTimeout int32,
	retryPolicy *types.RetryPolicy,
) (*types.HistoryEvent,
	*persistence.ChildExecutionInfo) {

	event, cei, _ := builder.AddStartChildWorkflowExecutionInitiatedEvent(decisionCompletedID, createRequestID,
		&types.StartChildWorkflowExecutionDecisionAttributes{
			Domain:                              domain,
			WorkflowID:                          workflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowType},
			TaskList:                            &types.TaskList{Name: tasklist},
			Input:                               input,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(executionStartToCloseTimeout),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(taskStartToCloseTimeout),
			Control:                             nil,
			RetryPolicy:                         retryPolicy,
		})
	return event, cei
}

// AddChildWorkflowExecutionStartedEvent adds ChildWorkflowExecutionStarted event
func AddChildWorkflowExecutionStartedEvent(
	builder execution.MutableState,
	initiatedID int64,
	domain string,
	workflowID string,
	runID string,
	workflowType string,
) *types.HistoryEvent {
	event, _ := builder.AddChildWorkflowExecutionStartedEvent(
		domain,
		&types.WorkflowExecution{
			WorkflowID: workflowID,
			RunID:      runID,
		},
		&types.WorkflowType{Name: workflowType},
		initiatedID,
		&types.Header{},
	)
	return event
}

// AddChildWorkflowExecutionCompletedEvent adds ChildWorkflowExecutionCompleted event
func AddChildWorkflowExecutionCompletedEvent(
	builder execution.MutableState,
	initiatedID int64,
	childExecution *types.WorkflowExecution,
	attributes *types.WorkflowExecutionCompletedEventAttributes,
) *types.HistoryEvent {
	event, _ := builder.AddChildWorkflowExecutionCompletedEvent(initiatedID, childExecution, attributes)
	return event
}

// AddCompleteWorkflowEvent adds WorkflowExecutionCompleted event
func AddCompleteWorkflowEvent(
	builder execution.MutableState,
	decisionCompletedEventID int64,
	result []byte,
) *types.HistoryEvent {
	event, _ := builder.AddCompletedWorkflowEvent(decisionCompletedEventID, &types.CompleteWorkflowExecutionDecisionAttributes{
		Result: result,
	})
	return event
}

// AddFailWorkflowEvent adds WorkflowExecutionFailed event
func AddFailWorkflowEvent(
	builder execution.MutableState,
	decisionCompletedEventID int64,
	reason string,
	details []byte,
) *types.HistoryEvent {
	event, _ := builder.AddFailWorkflowEvent(decisionCompletedEventID, &types.FailWorkflowExecutionDecisionAttributes{
		Reason:  &reason,
		Details: details,
	})
	return event
}
