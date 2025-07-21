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
	DecisionArray = []*types.Decision{
		&Decision_CancelTimer,
		&Decision_CancelWorkflowExecution,
		&Decision_CompleteWorkflowExecution,
		&Decision_ContinueAsNewWorkflowExecution,
		&Decision_FailWorkflowExecution,
		&Decision_RecordMarker,
		&Decision_RequestCancelActivityTask,
		&Decision_RequestCancelExternalWorkflowExecution,
		&Decision_ScheduleActivityTask,
		&Decision_SignalExternalWorkflowExecution,
		&Decision_StartChildWorkflowExecution,
		&Decision_StartTimer,
		&Decision_UpsertWorkflowSearchAttributes,
	}

	Decision_CancelTimer = types.Decision{
		DecisionType:                  types.DecisionTypeCancelTimer.Ptr(),
		CancelTimerDecisionAttributes: &CancelTimerDecisionAttributes,
	}
	Decision_CancelWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeCancelWorkflowExecution.Ptr(),
		CancelWorkflowExecutionDecisionAttributes: &CancelWorkflowExecutionDecisionAttributes,
	}
	Decision_CompleteWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
		CompleteWorkflowExecutionDecisionAttributes: &CompleteWorkflowExecutionDecisionAttributes,
	}
	Decision_ContinueAsNewWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
		ContinueAsNewWorkflowExecutionDecisionAttributes: &ContinueAsNewWorkflowExecutionDecisionAttributes,
	}
	Decision_FailWorkflowExecution = types.Decision{
		DecisionType:                            types.DecisionTypeFailWorkflowExecution.Ptr(),
		FailWorkflowExecutionDecisionAttributes: &FailWorkflowExecutionDecisionAttributes,
	}
	Decision_RecordMarker = types.Decision{
		DecisionType:                   types.DecisionTypeRecordMarker.Ptr(),
		RecordMarkerDecisionAttributes: &RecordMarkerDecisionAttributes,
	}
	Decision_RequestCancelActivityTask = types.Decision{
		DecisionType: types.DecisionTypeRequestCancelActivityTask.Ptr(),
		RequestCancelActivityTaskDecisionAttributes: &RequestCancelActivityTaskDecisionAttributes,
	}
	Decision_RequestCancelExternalWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeRequestCancelExternalWorkflowExecution.Ptr(),
		RequestCancelExternalWorkflowExecutionDecisionAttributes: &RequestCancelExternalWorkflowExecutionDecisionAttributes,
	}
	Decision_ScheduleActivityTask = types.Decision{
		DecisionType:                           types.DecisionTypeScheduleActivityTask.Ptr(),
		ScheduleActivityTaskDecisionAttributes: &ScheduleActivityTaskDecisionAttributes,
	}
	Decision_SignalExternalWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeSignalExternalWorkflowExecution.Ptr(),
		SignalExternalWorkflowExecutionDecisionAttributes: &SignalExternalWorkflowExecutionDecisionAttributes,
	}
	Decision_StartChildWorkflowExecution = types.Decision{
		DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
		StartChildWorkflowExecutionDecisionAttributes: &StartChildWorkflowExecutionDecisionAttributes,
	}
	Decision_StartTimer = types.Decision{
		DecisionType:                 types.DecisionTypeStartTimer.Ptr(),
		StartTimerDecisionAttributes: &StartTimerDecisionAttributes,
	}
	Decision_UpsertWorkflowSearchAttributes = types.Decision{
		DecisionType: types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
		UpsertWorkflowSearchAttributesDecisionAttributes: &UpsertWorkflowSearchAttributesDecisionAttributes,
	}

	CancelTimerDecisionAttributes = types.CancelTimerDecisionAttributes{
		TimerID: TimerID,
	}
	CancelWorkflowExecutionDecisionAttributes = types.CancelWorkflowExecutionDecisionAttributes{
		Details: Payload1,
	}
	CompleteWorkflowExecutionDecisionAttributes = types.CompleteWorkflowExecutionDecisionAttributes{
		Result: Payload1,
	}
	ContinueAsNewWorkflowExecutionDecisionAttributes = types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		BackoffStartIntervalInSeconds:       &Duration3,
		RetryPolicy:                         &RetryPolicy,
		Initiator:                           &ContinueAsNewInitiator,
		FailureReason:                       &FailureReason,
		FailureDetails:                      FailureDetails,
		LastCompletionResult:                Payload2,
		CronSchedule:                        CronSchedule,
		Header:                              &Header,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		JitterStartSeconds:                  &Duration1,
		CronOverlapPolicy:                   &CronOverlapPolicy,
	}
	FailWorkflowExecutionDecisionAttributes = types.FailWorkflowExecutionDecisionAttributes{
		Reason:  &FailureReason,
		Details: FailureDetails,
	}
	RecordMarkerDecisionAttributes = types.RecordMarkerDecisionAttributes{
		MarkerName: MarkerName,
		Details:    Payload1,
		Header:     &Header,
	}
	RequestCancelActivityTaskDecisionAttributes = types.RequestCancelActivityTaskDecisionAttributes{
		ActivityID: ActivityID,
	}
	RequestCancelExternalWorkflowExecutionDecisionAttributes = types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		Domain:            DomainName,
		WorkflowID:        WorkflowID,
		RunID:             RunID,
		Control:           Control,
		ChildWorkflowOnly: true,
	}
	ScheduleActivityTaskDecisionAttributes = types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    ActivityID,
		ActivityType:                  &ActivityType,
		Domain:                        DomainName,
		TaskList:                      &TaskList,
		Input:                         Payload1,
		ScheduleToCloseTimeoutSeconds: &Duration1,
		ScheduleToStartTimeoutSeconds: &Duration2,
		StartToCloseTimeoutSeconds:    &Duration3,
		HeartbeatTimeoutSeconds:       &Duration4,
		RetryPolicy:                   &RetryPolicy,
		Header:                        &Header,
		RequestLocalDispatch:          true,
	}
	SignalExternalWorkflowExecutionDecisionAttributes = types.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain:            DomainName,
		Execution:         &WorkflowExecution,
		SignalName:        SignalName,
		Input:             Payload1,
		Control:           Control,
		ChildWorkflowOnly: true,
	}
	StartChildWorkflowExecutionDecisionAttributes = types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                              DomainName,
		WorkflowID:                          WorkflowID,
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		ExecutionStartToCloseTimeoutSeconds: &Duration1,
		TaskStartToCloseTimeoutSeconds:      &Duration2,
		ParentClosePolicy:                   &ParentClosePolicy,
		Control:                             Control,
		WorkflowIDReusePolicy:               &WorkflowIDReusePolicy,
		RetryPolicy:                         &RetryPolicy,
		CronSchedule:                        CronSchedule,
		Header:                              &Header,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
		CronOverlapPolicy:                   &CronOverlapPolicy,
	}
	StartTimerDecisionAttributes = types.StartTimerDecisionAttributes{
		TimerID:                   TimerID,
		StartToFireTimeoutSeconds: common.Int64Ptr(int64(Duration1)),
	}
	UpsertWorkflowSearchAttributesDecisionAttributes = types.UpsertWorkflowSearchAttributesDecisionAttributes{
		SearchAttributes: &SearchAttributes,
	}
)
