// Copyright (c) 2017-2020 Uber Technologies Inc.
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

package thrift

import (
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/types"
)

var (
	FromScanWorkflowExecutionsRequest            = FromListWorkflowExecutionsRequest
	ToScanWorkflowExecutionsRequest              = ToListWorkflowExecutionsRequest
	FromRecordActivityTaskHeartbeatByIDResponse  = FromRecordActivityTaskHeartbeatResponse
	ToRecordActivityTaskHeartbeatByIDResponse    = ToRecordActivityTaskHeartbeatResponse
	FromScanWorkflowExecutionsResponse           = FromListWorkflowExecutionsResponse
	ToScanWorkflowExecutionsResponse             = ToListWorkflowExecutionsResponse
	FromSignalWithStartWorkflowExecutionResponse = FromStartWorkflowExecutionResponse
	ToSignalWithStartWorkflowExecutionResponse   = ToStartWorkflowExecutionResponse
)

// FromAccessDeniedError converts internal AccessDeniedError type to thrift
func FromAccessDeniedError(t *types.AccessDeniedError) *shared.AccessDeniedError {
	if t == nil {
		return nil
	}
	return &shared.AccessDeniedError{
		Message: t.Message,
	}
}

// ToAccessDeniedError converts thrift AccessDeniedError type to internal
func ToAccessDeniedError(t *shared.AccessDeniedError) *types.AccessDeniedError {
	if t == nil {
		return nil
	}
	return &types.AccessDeniedError{
		Message: t.Message,
	}
}

// FromActivityLocalDispatchInfo converts internal ActivityLocalDispatchInfo type to thrift
func FromActivityLocalDispatchInfo(t *types.ActivityLocalDispatchInfo) *shared.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	return &shared.ActivityLocalDispatchInfo{
		ActivityId:                      &t.ActivityID,
		ScheduledTimestamp:              t.ScheduledTimestamp,
		StartedTimestamp:                t.StartedTimestamp,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		TaskToken:                       t.TaskToken,
	}
}

// ToActivityLocalDispatchInfo converts thrift ActivityLocalDispatchInfo type to internal
func ToActivityLocalDispatchInfo(t *shared.ActivityLocalDispatchInfo) *types.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	return &types.ActivityLocalDispatchInfo{
		ActivityID:                      t.GetActivityId(),
		ScheduledTimestamp:              t.ScheduledTimestamp,
		StartedTimestamp:                t.StartedTimestamp,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		TaskToken:                       t.TaskToken,
	}
}

// FromActivityTaskCancelRequestedEventAttributes converts internal ActivityTaskCancelRequestedEventAttributes type to thrift
func FromActivityTaskCancelRequestedEventAttributes(t *types.ActivityTaskCancelRequestedEventAttributes) *shared.ActivityTaskCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskCancelRequestedEventAttributes{
		ActivityId:                   &t.ActivityID,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToActivityTaskCancelRequestedEventAttributes converts thrift ActivityTaskCancelRequestedEventAttributes type to internal
func ToActivityTaskCancelRequestedEventAttributes(t *shared.ActivityTaskCancelRequestedEventAttributes) *types.ActivityTaskCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCancelRequestedEventAttributes{
		ActivityID:                   t.GetActivityId(),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromActivityTaskCanceledEventAttributes converts internal ActivityTaskCanceledEventAttributes type to thrift
func FromActivityTaskCanceledEventAttributes(t *types.ActivityTaskCanceledEventAttributes) *shared.ActivityTaskCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskCanceledEventAttributes{
		Details:                      t.Details,
		LatestCancelRequestedEventId: &t.LatestCancelRequestedEventID,
		ScheduledEventId:             &t.ScheduledEventID,
		StartedEventId:               &t.StartedEventID,
		Identity:                     &t.Identity,
	}
}

// ToActivityTaskCanceledEventAttributes converts thrift ActivityTaskCanceledEventAttributes type to internal
func ToActivityTaskCanceledEventAttributes(t *shared.ActivityTaskCanceledEventAttributes) *types.ActivityTaskCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCanceledEventAttributes{
		Details:                      t.Details,
		LatestCancelRequestedEventID: t.GetLatestCancelRequestedEventId(),
		ScheduledEventID:             t.GetScheduledEventId(),
		StartedEventID:               t.GetStartedEventId(),
		Identity:                     t.GetIdentity(),
	}
}

// FromActivityTaskCompletedEventAttributes converts internal ActivityTaskCompletedEventAttributes type to thrift
func FromActivityTaskCompletedEventAttributes(t *types.ActivityTaskCompletedEventAttributes) *shared.ActivityTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskCompletedEventAttributes{
		Result:           t.Result,
		ScheduledEventId: &t.ScheduledEventID,
		StartedEventId:   &t.StartedEventID,
		Identity:         &t.Identity,
	}
}

// ToActivityTaskCompletedEventAttributes converts thrift ActivityTaskCompletedEventAttributes type to internal
func ToActivityTaskCompletedEventAttributes(t *shared.ActivityTaskCompletedEventAttributes) *types.ActivityTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCompletedEventAttributes{
		Result:           t.Result,
		ScheduledEventID: t.GetScheduledEventId(),
		StartedEventID:   t.GetStartedEventId(),
		Identity:         t.GetIdentity(),
	}
}

// FromActivityTaskFailedEventAttributes converts internal ActivityTaskFailedEventAttributes type to thrift
func FromActivityTaskFailedEventAttributes(t *types.ActivityTaskFailedEventAttributes) *shared.ActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskFailedEventAttributes{
		Reason:           t.Reason,
		Details:          t.Details,
		ScheduledEventId: &t.ScheduledEventID,
		StartedEventId:   &t.StartedEventID,
		Identity:         &t.Identity,
	}
}

// ToActivityTaskFailedEventAttributes converts thrift ActivityTaskFailedEventAttributes type to internal
func ToActivityTaskFailedEventAttributes(t *shared.ActivityTaskFailedEventAttributes) *types.ActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskFailedEventAttributes{
		Reason:           t.Reason,
		Details:          t.Details,
		ScheduledEventID: t.GetScheduledEventId(),
		StartedEventID:   t.GetStartedEventId(),
		Identity:         t.GetIdentity(),
	}
}

// FromActivityTaskScheduledEventAttributes converts internal ActivityTaskScheduledEventAttributes type to thrift
func FromActivityTaskScheduledEventAttributes(t *types.ActivityTaskScheduledEventAttributes) *shared.ActivityTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskScheduledEventAttributes{
		ActivityId:                    &t.ActivityID,
		ActivityType:                  FromActivityType(t.ActivityType),
		Domain:                        t.Domain,
		TaskList:                      FromTaskList(t.TaskList),
		Input:                         t.Input,
		ScheduleToCloseTimeoutSeconds: t.ScheduleToCloseTimeoutSeconds,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       t.HeartbeatTimeoutSeconds,
		DecisionTaskCompletedEventId:  &t.DecisionTaskCompletedEventID,
		RetryPolicy:                   FromRetryPolicy(t.RetryPolicy),
		Header:                        FromHeader(t.Header),
	}
}

// ToActivityTaskScheduledEventAttributes converts thrift ActivityTaskScheduledEventAttributes type to internal
func ToActivityTaskScheduledEventAttributes(t *shared.ActivityTaskScheduledEventAttributes) *types.ActivityTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskScheduledEventAttributes{
		ActivityID:                    t.GetActivityId(),
		ActivityType:                  ToActivityType(t.ActivityType),
		Domain:                        t.Domain,
		TaskList:                      ToTaskList(t.TaskList),
		Input:                         t.Input,
		ScheduleToCloseTimeoutSeconds: t.ScheduleToCloseTimeoutSeconds,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       t.HeartbeatTimeoutSeconds,
		DecisionTaskCompletedEventID:  t.GetDecisionTaskCompletedEventId(),
		RetryPolicy:                   ToRetryPolicy(t.RetryPolicy),
		Header:                        ToHeader(t.Header),
	}
}

// FromActivityTaskStartedEventAttributes converts internal ActivityTaskStartedEventAttributes type to thrift
func FromActivityTaskStartedEventAttributes(t *types.ActivityTaskStartedEventAttributes) *shared.ActivityTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskStartedEventAttributes{
		ScheduledEventId:   &t.ScheduledEventID,
		Identity:           &t.Identity,
		RequestId:          &t.RequestID,
		Attempt:            &t.Attempt,
		LastFailureReason:  t.LastFailureReason,
		LastFailureDetails: t.LastFailureDetails,
	}
}

// ToActivityTaskStartedEventAttributes converts thrift ActivityTaskStartedEventAttributes type to internal
func ToActivityTaskStartedEventAttributes(t *shared.ActivityTaskStartedEventAttributes) *types.ActivityTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskStartedEventAttributes{
		ScheduledEventID:   t.GetScheduledEventId(),
		Identity:           t.GetIdentity(),
		RequestID:          t.GetRequestId(),
		Attempt:            t.GetAttempt(),
		LastFailureReason:  t.LastFailureReason,
		LastFailureDetails: t.LastFailureDetails,
	}
}

// FromActivityTaskTimedOutEventAttributes converts internal ActivityTaskTimedOutEventAttributes type to thrift
func FromActivityTaskTimedOutEventAttributes(t *types.ActivityTaskTimedOutEventAttributes) *shared.ActivityTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ActivityTaskTimedOutEventAttributes{
		Details:            t.Details,
		ScheduledEventId:   &t.ScheduledEventID,
		StartedEventId:     &t.StartedEventID,
		TimeoutType:        FromTimeoutType(t.TimeoutType),
		LastFailureReason:  t.LastFailureReason,
		LastFailureDetails: t.LastFailureDetails,
	}
}

// ToActivityTaskTimedOutEventAttributes converts thrift ActivityTaskTimedOutEventAttributes type to internal
func ToActivityTaskTimedOutEventAttributes(t *shared.ActivityTaskTimedOutEventAttributes) *types.ActivityTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskTimedOutEventAttributes{
		Details:            t.Details,
		ScheduledEventID:   t.GetScheduledEventId(),
		StartedEventID:     t.GetStartedEventId(),
		TimeoutType:        ToTimeoutType(t.TimeoutType),
		LastFailureReason:  t.LastFailureReason,
		LastFailureDetails: t.LastFailureDetails,
	}
}

// FromActivityType converts internal ActivityType type to thrift
func FromActivityType(t *types.ActivityType) *shared.ActivityType {
	if t == nil {
		return nil
	}
	return &shared.ActivityType{
		Name: &t.Name,
	}
}

// ToActivityType converts thrift ActivityType type to internal
func ToActivityType(t *shared.ActivityType) *types.ActivityType {
	if t == nil {
		return nil
	}
	return &types.ActivityType{
		Name: t.GetName(),
	}
}

// FromArchivalStatus converts internal ArchivalStatus type to thrift
func FromArchivalStatus(t *types.ArchivalStatus) *shared.ArchivalStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case types.ArchivalStatusDisabled:
		v := shared.ArchivalStatusDisabled
		return &v
	case types.ArchivalStatusEnabled:
		v := shared.ArchivalStatusEnabled
		return &v
	}
	panic("unexpected enum value")
}

// ToArchivalStatus converts thrift ArchivalStatus type to internal
func ToArchivalStatus(t *shared.ArchivalStatus) *types.ArchivalStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.ArchivalStatusDisabled:
		v := types.ArchivalStatusDisabled
		return &v
	case shared.ArchivalStatusEnabled:
		v := types.ArchivalStatusEnabled
		return &v
	}
	panic("unexpected enum value")
}

// FromBadBinaries converts internal BadBinaries type to thrift
func FromBadBinaries(t *types.BadBinaries) *shared.BadBinaries {
	if t == nil {
		return nil
	}
	return &shared.BadBinaries{
		Binaries: FromBadBinaryInfoMap(t.Binaries),
	}
}

// ToBadBinaries converts thrift BadBinaries type to internal
func ToBadBinaries(t *shared.BadBinaries) *types.BadBinaries {
	if t == nil {
		return nil
	}
	return &types.BadBinaries{
		Binaries: ToBadBinaryInfoMap(t.Binaries),
	}
}

// FromBadBinaryInfo converts internal BadBinaryInfo type to thrift
func FromBadBinaryInfo(t *types.BadBinaryInfo) *shared.BadBinaryInfo {
	if t == nil {
		return nil
	}
	return &shared.BadBinaryInfo{
		Reason:          &t.Reason,
		Operator:        &t.Operator,
		CreatedTimeNano: t.CreatedTimeNano,
	}
}

// ToBadBinaryInfo converts thrift BadBinaryInfo type to internal
func ToBadBinaryInfo(t *shared.BadBinaryInfo) *types.BadBinaryInfo {
	if t == nil {
		return nil
	}
	return &types.BadBinaryInfo{
		Reason:          t.GetReason(),
		Operator:        t.GetOperator(),
		CreatedTimeNano: t.CreatedTimeNano,
	}
}

// FromBadRequestError converts internal BadRequestError type to thrift
func FromBadRequestError(t *types.BadRequestError) *shared.BadRequestError {
	if t == nil {
		return nil
	}
	return &shared.BadRequestError{
		Message: t.Message,
	}
}

// ToBadRequestError converts thrift BadRequestError type to internal
func ToBadRequestError(t *shared.BadRequestError) *types.BadRequestError {
	if t == nil {
		return nil
	}
	return &types.BadRequestError{
		Message: t.Message,
	}
}

// FromCancelExternalWorkflowExecutionFailedCause converts internal CancelExternalWorkflowExecutionFailedCause type to thrift
func FromCancelExternalWorkflowExecutionFailedCause(t *types.CancelExternalWorkflowExecutionFailedCause) *shared.CancelExternalWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		v := shared.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return &v
	case types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		v := shared.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return &v
	}
	panic("unexpected enum value")
}

// ToCancelExternalWorkflowExecutionFailedCause converts thrift CancelExternalWorkflowExecutionFailedCause type to internal
func ToCancelExternalWorkflowExecutionFailedCause(t *shared.CancelExternalWorkflowExecutionFailedCause) *types.CancelExternalWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		v := types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return &v
	case shared.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		v := types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return &v
	}
	panic("unexpected enum value")
}

// FromCancelTimerDecisionAttributes converts internal CancelTimerDecisionAttributes type to thrift
func FromCancelTimerDecisionAttributes(t *types.CancelTimerDecisionAttributes) *shared.CancelTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.CancelTimerDecisionAttributes{
		TimerId: &t.TimerID,
	}
}

// ToCancelTimerDecisionAttributes converts thrift CancelTimerDecisionAttributes type to internal
func ToCancelTimerDecisionAttributes(t *shared.CancelTimerDecisionAttributes) *types.CancelTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelTimerDecisionAttributes{
		TimerID: t.GetTimerId(),
	}
}

// FromCancelTimerFailedEventAttributes converts internal CancelTimerFailedEventAttributes type to thrift
func FromCancelTimerFailedEventAttributes(t *types.CancelTimerFailedEventAttributes) *shared.CancelTimerFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.CancelTimerFailedEventAttributes{
		TimerId:                      &t.TimerID,
		Cause:                        &t.Cause,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Identity:                     &t.Identity,
	}
}

// ToCancelTimerFailedEventAttributes converts thrift CancelTimerFailedEventAttributes type to internal
func ToCancelTimerFailedEventAttributes(t *shared.CancelTimerFailedEventAttributes) *types.CancelTimerFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelTimerFailedEventAttributes{
		TimerID:                      t.GetTimerId(),
		Cause:                        t.GetCause(),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Identity:                     t.GetIdentity(),
	}
}

// FromCancelWorkflowExecutionDecisionAttributes converts internal CancelWorkflowExecutionDecisionAttributes type to thrift
func FromCancelWorkflowExecutionDecisionAttributes(t *types.CancelWorkflowExecutionDecisionAttributes) *shared.CancelWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.CancelWorkflowExecutionDecisionAttributes{
		Details: t.Details,
	}
}

// ToCancelWorkflowExecutionDecisionAttributes converts thrift CancelWorkflowExecutionDecisionAttributes type to internal
func ToCancelWorkflowExecutionDecisionAttributes(t *shared.CancelWorkflowExecutionDecisionAttributes) *types.CancelWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelWorkflowExecutionDecisionAttributes{
		Details: t.Details,
	}
}

// FromCancellationAlreadyRequestedError converts internal CancellationAlreadyRequestedError type to thrift
func FromCancellationAlreadyRequestedError(t *types.CancellationAlreadyRequestedError) *shared.CancellationAlreadyRequestedError {
	if t == nil {
		return nil
	}
	return &shared.CancellationAlreadyRequestedError{
		Message: t.Message,
	}
}

// ToCancellationAlreadyRequestedError converts thrift CancellationAlreadyRequestedError type to internal
func ToCancellationAlreadyRequestedError(t *shared.CancellationAlreadyRequestedError) *types.CancellationAlreadyRequestedError {
	if t == nil {
		return nil
	}
	return &types.CancellationAlreadyRequestedError{
		Message: t.Message,
	}
}

// FromChildWorkflowExecutionCanceledEventAttributes converts internal ChildWorkflowExecutionCanceledEventAttributes type to thrift
func FromChildWorkflowExecutionCanceledEventAttributes(t *types.ChildWorkflowExecutionCanceledEventAttributes) *shared.ChildWorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionCanceledEventAttributes{
		Details:           t.Details,
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  &t.InitiatedEventID,
		StartedEventId:    &t.StartedEventID,
	}
}

// ToChildWorkflowExecutionCanceledEventAttributes converts thrift ChildWorkflowExecutionCanceledEventAttributes type to internal
func ToChildWorkflowExecutionCanceledEventAttributes(t *shared.ChildWorkflowExecutionCanceledEventAttributes) *types.ChildWorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionCanceledEventAttributes{
		Details:           t.Details,
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.GetInitiatedEventId(),
		StartedEventID:    t.GetStartedEventId(),
	}
}

// FromChildWorkflowExecutionCompletedEventAttributes converts internal ChildWorkflowExecutionCompletedEventAttributes type to thrift
func FromChildWorkflowExecutionCompletedEventAttributes(t *types.ChildWorkflowExecutionCompletedEventAttributes) *shared.ChildWorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionCompletedEventAttributes{
		Result:            t.Result,
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  &t.InitiatedEventID,
		StartedEventId:    &t.StartedEventID,
	}
}

// ToChildWorkflowExecutionCompletedEventAttributes converts thrift ChildWorkflowExecutionCompletedEventAttributes type to internal
func ToChildWorkflowExecutionCompletedEventAttributes(t *shared.ChildWorkflowExecutionCompletedEventAttributes) *types.ChildWorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionCompletedEventAttributes{
		Result:            t.Result,
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.GetInitiatedEventId(),
		StartedEventID:    t.GetStartedEventId(),
	}
}

// FromChildWorkflowExecutionFailedCause converts internal ChildWorkflowExecutionFailedCause type to thrift
func FromChildWorkflowExecutionFailedCause(t *types.ChildWorkflowExecutionFailedCause) *shared.ChildWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning:
		v := shared.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning
		return &v
	}
	panic("unexpected enum value")
}

// ToChildWorkflowExecutionFailedCause converts thrift ChildWorkflowExecutionFailedCause type to internal
func ToChildWorkflowExecutionFailedCause(t *shared.ChildWorkflowExecutionFailedCause) *types.ChildWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning:
		v := types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning
		return &v
	}
	panic("unexpected enum value")
}

// FromChildWorkflowExecutionFailedEventAttributes converts internal ChildWorkflowExecutionFailedEventAttributes type to thrift
func FromChildWorkflowExecutionFailedEventAttributes(t *types.ChildWorkflowExecutionFailedEventAttributes) *shared.ChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionFailedEventAttributes{
		Reason:            t.Reason,
		Details:           t.Details,
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  &t.InitiatedEventID,
		StartedEventId:    &t.StartedEventID,
	}
}

// ToChildWorkflowExecutionFailedEventAttributes converts thrift ChildWorkflowExecutionFailedEventAttributes type to internal
func ToChildWorkflowExecutionFailedEventAttributes(t *shared.ChildWorkflowExecutionFailedEventAttributes) *types.ChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionFailedEventAttributes{
		Reason:            t.Reason,
		Details:           t.Details,
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.GetInitiatedEventId(),
		StartedEventID:    t.GetStartedEventId(),
	}
}

// FromChildWorkflowExecutionStartedEventAttributes converts internal ChildWorkflowExecutionStartedEventAttributes type to thrift
func FromChildWorkflowExecutionStartedEventAttributes(t *types.ChildWorkflowExecutionStartedEventAttributes) *shared.ChildWorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            &t.Domain,
		InitiatedEventId:  &t.InitiatedEventID,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		Header:            FromHeader(t.Header),
	}
}

// ToChildWorkflowExecutionStartedEventAttributes converts thrift ChildWorkflowExecutionStartedEventAttributes type to internal
func ToChildWorkflowExecutionStartedEventAttributes(t *shared.ChildWorkflowExecutionStartedEventAttributes) *types.ChildWorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            t.GetDomain(),
		InitiatedEventID:  t.GetInitiatedEventId(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		Header:            ToHeader(t.Header),
	}
}

// FromChildWorkflowExecutionTerminatedEventAttributes converts internal ChildWorkflowExecutionTerminatedEventAttributes type to thrift
func FromChildWorkflowExecutionTerminatedEventAttributes(t *types.ChildWorkflowExecutionTerminatedEventAttributes) *shared.ChildWorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  &t.InitiatedEventID,
		StartedEventId:    &t.StartedEventID,
	}
}

// ToChildWorkflowExecutionTerminatedEventAttributes converts thrift ChildWorkflowExecutionTerminatedEventAttributes type to internal
func ToChildWorkflowExecutionTerminatedEventAttributes(t *shared.ChildWorkflowExecutionTerminatedEventAttributes) *types.ChildWorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.GetInitiatedEventId(),
		StartedEventID:    t.GetStartedEventId(),
	}
}

// FromChildWorkflowExecutionTimedOutEventAttributes converts internal ChildWorkflowExecutionTimedOutEventAttributes type to thrift
func FromChildWorkflowExecutionTimedOutEventAttributes(t *types.ChildWorkflowExecutionTimedOutEventAttributes) *shared.ChildWorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ChildWorkflowExecutionTimedOutEventAttributes{
		TimeoutType:       FromTimeoutType(t.TimeoutType),
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  &t.InitiatedEventID,
		StartedEventId:    &t.StartedEventID,
	}
}

// ToChildWorkflowExecutionTimedOutEventAttributes converts thrift ChildWorkflowExecutionTimedOutEventAttributes type to internal
func ToChildWorkflowExecutionTimedOutEventAttributes(t *shared.ChildWorkflowExecutionTimedOutEventAttributes) *types.ChildWorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionTimedOutEventAttributes{
		TimeoutType:       ToTimeoutType(t.TimeoutType),
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.GetInitiatedEventId(),
		StartedEventID:    t.GetStartedEventId(),
	}
}

// FromClientVersionNotSupportedError converts internal ClientVersionNotSupportedError type to thrift
func FromClientVersionNotSupportedError(t *types.ClientVersionNotSupportedError) *shared.ClientVersionNotSupportedError {
	if t == nil {
		return nil
	}
	return &shared.ClientVersionNotSupportedError{
		FeatureVersion:    t.FeatureVersion,
		ClientImpl:        t.ClientImpl,
		SupportedVersions: t.SupportedVersions,
	}
}

// ToClientVersionNotSupportedError converts thrift ClientVersionNotSupportedError type to internal
func ToClientVersionNotSupportedError(t *shared.ClientVersionNotSupportedError) *types.ClientVersionNotSupportedError {
	if t == nil {
		return nil
	}
	return &types.ClientVersionNotSupportedError{
		FeatureVersion:    t.FeatureVersion,
		ClientImpl:        t.ClientImpl,
		SupportedVersions: t.SupportedVersions,
	}
}

// FromFeatureNotEnabledError converts internal FeatureNotEnabledError type to thrift
func FromFeatureNotEnabledError(t *types.FeatureNotEnabledError) *shared.FeatureNotEnabledError {
	if t == nil {
		return nil
	}
	return &shared.FeatureNotEnabledError{
		FeatureFlag: t.FeatureFlag,
	}
}

// ToFeatureNotEnabledError converts thrift FeatureNotEnabledError type to internal
func ToFeatureNotEnabledError(t *shared.FeatureNotEnabledError) *types.FeatureNotEnabledError {
	if t == nil {
		return nil
	}
	return &types.FeatureNotEnabledError{
		FeatureFlag: t.FeatureFlag,
	}
}

// FromAdminCloseShardRequest converts internal CloseShardRequest type to thrift
func FromAdminCloseShardRequest(t *types.CloseShardRequest) *shared.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &shared.CloseShardRequest{
		ShardID: &t.ShardID,
	}
}

// ToAdminCloseShardRequest converts thrift CloseShardRequest type to internal
func ToAdminCloseShardRequest(t *shared.CloseShardRequest) *types.CloseShardRequest {
	if t == nil {
		return nil
	}
	return &types.CloseShardRequest{
		ShardID: t.GetShardID(),
	}
}

// FromGetClusterInfoResponse converts internal ClusterInfo type to thrift
func FromGetClusterInfoResponse(t *types.ClusterInfo) *shared.ClusterInfo {
	if t == nil {
		return nil
	}
	return &shared.ClusterInfo{
		SupportedClientVersions: FromSupportedClientVersions(t.SupportedClientVersions),
	}
}

// ToGetClusterInfoResponse converts thrift ClusterInfo type to internal
func ToGetClusterInfoResponse(t *shared.ClusterInfo) *types.ClusterInfo {
	if t == nil {
		return nil
	}
	return &types.ClusterInfo{
		SupportedClientVersions: ToSupportedClientVersions(t.SupportedClientVersions),
	}
}

// FromClusterReplicationConfiguration converts internal ClusterReplicationConfiguration type to thrift
func FromClusterReplicationConfiguration(t *types.ClusterReplicationConfiguration) *shared.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &shared.ClusterReplicationConfiguration{
		ClusterName: &t.ClusterName,
	}
}

// ToClusterReplicationConfiguration converts thrift ClusterReplicationConfiguration type to internal
func ToClusterReplicationConfiguration(t *shared.ClusterReplicationConfiguration) *types.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &types.ClusterReplicationConfiguration{
		ClusterName: t.GetClusterName(),
	}
}

// FromCompleteWorkflowExecutionDecisionAttributes converts internal CompleteWorkflowExecutionDecisionAttributes type to thrift
func FromCompleteWorkflowExecutionDecisionAttributes(t *types.CompleteWorkflowExecutionDecisionAttributes) *shared.CompleteWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.CompleteWorkflowExecutionDecisionAttributes{
		Result: t.Result,
	}
}

// ToCompleteWorkflowExecutionDecisionAttributes converts thrift CompleteWorkflowExecutionDecisionAttributes type to internal
func ToCompleteWorkflowExecutionDecisionAttributes(t *shared.CompleteWorkflowExecutionDecisionAttributes) *types.CompleteWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CompleteWorkflowExecutionDecisionAttributes{
		Result: t.Result,
	}
}

// FromContinueAsNewInitiator converts internal ContinueAsNewInitiator type to thrift
func FromContinueAsNewInitiator(t *types.ContinueAsNewInitiator) *shared.ContinueAsNewInitiator {
	if t == nil {
		return nil
	}
	switch *t {
	case types.ContinueAsNewInitiatorDecider:
		v := shared.ContinueAsNewInitiatorDecider
		return &v
	case types.ContinueAsNewInitiatorRetryPolicy:
		v := shared.ContinueAsNewInitiatorRetryPolicy
		return &v
	case types.ContinueAsNewInitiatorCronSchedule:
		v := shared.ContinueAsNewInitiatorCronSchedule
		return &v
	}
	panic("unexpected enum value")
}

// ToContinueAsNewInitiator converts thrift ContinueAsNewInitiator type to internal
func ToContinueAsNewInitiator(t *shared.ContinueAsNewInitiator) *types.ContinueAsNewInitiator {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.ContinueAsNewInitiatorDecider:
		v := types.ContinueAsNewInitiatorDecider
		return &v
	case shared.ContinueAsNewInitiatorRetryPolicy:
		v := types.ContinueAsNewInitiatorRetryPolicy
		return &v
	case shared.ContinueAsNewInitiatorCronSchedule:
		v := types.ContinueAsNewInitiatorCronSchedule
		return &v
	}
	panic("unexpected enum value")
}

// FromContinueAsNewWorkflowExecutionDecisionAttributes converts internal ContinueAsNewWorkflowExecutionDecisionAttributes type to thrift
func FromContinueAsNewWorkflowExecutionDecisionAttributes(t *types.ContinueAsNewWorkflowExecutionDecisionAttributes) *shared.ContinueAsNewWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		BackoffStartIntervalInSeconds:       t.BackoffStartIntervalInSeconds,
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		Initiator:                           FromContinueAsNewInitiator(t.Initiator),
		FailureReason:                       t.FailureReason,
		FailureDetails:                      t.FailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		CronSchedule:                        &t.CronSchedule,
		Header:                              FromHeader(t.Header),
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		JitterStartSeconds:                  t.JitterStartSeconds,
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

// ToContinueAsNewWorkflowExecutionDecisionAttributes converts thrift ContinueAsNewWorkflowExecutionDecisionAttributes type to internal
func ToContinueAsNewWorkflowExecutionDecisionAttributes(t *shared.ContinueAsNewWorkflowExecutionDecisionAttributes) *types.ContinueAsNewWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		BackoffStartIntervalInSeconds:       t.BackoffStartIntervalInSeconds,
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		FailureReason:                       t.FailureReason,
		FailureDetails:                      t.FailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		CronSchedule:                        t.GetCronSchedule(),
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		JitterStartSeconds:                  t.JitterStartSeconds,
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

// FromCountWorkflowExecutionsRequest converts internal CountWorkflowExecutionsRequest type to thrift
func FromCountWorkflowExecutionsRequest(t *types.CountWorkflowExecutionsRequest) *shared.CountWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &shared.CountWorkflowExecutionsRequest{
		Domain: &t.Domain,
		Query:  &t.Query,
	}
}

// ToCountWorkflowExecutionsRequest converts thrift CountWorkflowExecutionsRequest type to internal
func ToCountWorkflowExecutionsRequest(t *shared.CountWorkflowExecutionsRequest) *types.CountWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.CountWorkflowExecutionsRequest{
		Domain: t.GetDomain(),
		Query:  t.GetQuery(),
	}
}

// FromCountWorkflowExecutionsResponse converts internal CountWorkflowExecutionsResponse type to thrift
func FromCountWorkflowExecutionsResponse(t *types.CountWorkflowExecutionsResponse) *shared.CountWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &shared.CountWorkflowExecutionsResponse{
		Count: &t.Count,
	}
}

// ToCountWorkflowExecutionsResponse converts thrift CountWorkflowExecutionsResponse type to internal
func ToCountWorkflowExecutionsResponse(t *shared.CountWorkflowExecutionsResponse) *types.CountWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.CountWorkflowExecutionsResponse{
		Count: t.GetCount(),
	}
}

// FromCurrentBranchChangedError converts internal CurrentBranchChangedError type to thrift
func FromCurrentBranchChangedError(t *types.CurrentBranchChangedError) *shared.CurrentBranchChangedError {
	if t == nil {
		return nil
	}
	return &shared.CurrentBranchChangedError{
		Message:            t.Message,
		CurrentBranchToken: t.CurrentBranchToken,
	}
}

// ToCurrentBranchChangedError converts thrift CurrentBranchChangedError type to internal
func ToCurrentBranchChangedError(t *shared.CurrentBranchChangedError) *types.CurrentBranchChangedError {
	if t == nil {
		return nil
	}
	return &types.CurrentBranchChangedError{
		Message:            t.Message,
		CurrentBranchToken: t.CurrentBranchToken,
	}
}

// FromDataBlob converts internal DataBlob type to thrift
func FromDataBlob(t *types.DataBlob) *shared.DataBlob {
	if t == nil {
		return nil
	}
	return &shared.DataBlob{
		EncodingType: FromEncodingType(t.EncodingType),
		Data:         t.Data,
	}
}

// ToDataBlob converts thrift DataBlob type to internal
func ToDataBlob(t *shared.DataBlob) *types.DataBlob {
	if t == nil {
		return nil
	}
	return &types.DataBlob{
		EncodingType: ToEncodingType(t.EncodingType),
		Data:         t.Data,
	}
}

// FromDecision converts internal Decision type to thrift
func FromDecision(t *types.Decision) *shared.Decision {
	if t == nil {
		return nil
	}
	return &shared.Decision{
		DecisionType:                                             FromDecisionType(t.DecisionType),
		ScheduleActivityTaskDecisionAttributes:                   FromScheduleActivityTaskDecisionAttributes(t.ScheduleActivityTaskDecisionAttributes),
		StartTimerDecisionAttributes:                             FromStartTimerDecisionAttributes(t.StartTimerDecisionAttributes),
		CompleteWorkflowExecutionDecisionAttributes:              FromCompleteWorkflowExecutionDecisionAttributes(t.CompleteWorkflowExecutionDecisionAttributes),
		FailWorkflowExecutionDecisionAttributes:                  FromFailWorkflowExecutionDecisionAttributes(t.FailWorkflowExecutionDecisionAttributes),
		RequestCancelActivityTaskDecisionAttributes:              FromRequestCancelActivityTaskDecisionAttributes(t.RequestCancelActivityTaskDecisionAttributes),
		CancelTimerDecisionAttributes:                            FromCancelTimerDecisionAttributes(t.CancelTimerDecisionAttributes),
		CancelWorkflowExecutionDecisionAttributes:                FromCancelWorkflowExecutionDecisionAttributes(t.CancelWorkflowExecutionDecisionAttributes),
		RequestCancelExternalWorkflowExecutionDecisionAttributes: FromRequestCancelExternalWorkflowExecutionDecisionAttributes(t.RequestCancelExternalWorkflowExecutionDecisionAttributes),
		RecordMarkerDecisionAttributes:                           FromRecordMarkerDecisionAttributes(t.RecordMarkerDecisionAttributes),
		ContinueAsNewWorkflowExecutionDecisionAttributes:         FromContinueAsNewWorkflowExecutionDecisionAttributes(t.ContinueAsNewWorkflowExecutionDecisionAttributes),
		StartChildWorkflowExecutionDecisionAttributes:            FromStartChildWorkflowExecutionDecisionAttributes(t.StartChildWorkflowExecutionDecisionAttributes),
		SignalExternalWorkflowExecutionDecisionAttributes:        FromSignalExternalWorkflowExecutionDecisionAttributes(t.SignalExternalWorkflowExecutionDecisionAttributes),
		UpsertWorkflowSearchAttributesDecisionAttributes:         FromUpsertWorkflowSearchAttributesDecisionAttributes(t.UpsertWorkflowSearchAttributesDecisionAttributes),
	}
}

// ToDecision converts thrift Decision type to internal
func ToDecision(t *shared.Decision) *types.Decision {
	if t == nil {
		return nil
	}
	return &types.Decision{
		DecisionType:                                             ToDecisionType(t.DecisionType),
		ScheduleActivityTaskDecisionAttributes:                   ToScheduleActivityTaskDecisionAttributes(t.ScheduleActivityTaskDecisionAttributes),
		StartTimerDecisionAttributes:                             ToStartTimerDecisionAttributes(t.StartTimerDecisionAttributes),
		CompleteWorkflowExecutionDecisionAttributes:              ToCompleteWorkflowExecutionDecisionAttributes(t.CompleteWorkflowExecutionDecisionAttributes),
		FailWorkflowExecutionDecisionAttributes:                  ToFailWorkflowExecutionDecisionAttributes(t.FailWorkflowExecutionDecisionAttributes),
		RequestCancelActivityTaskDecisionAttributes:              ToRequestCancelActivityTaskDecisionAttributes(t.RequestCancelActivityTaskDecisionAttributes),
		CancelTimerDecisionAttributes:                            ToCancelTimerDecisionAttributes(t.CancelTimerDecisionAttributes),
		CancelWorkflowExecutionDecisionAttributes:                ToCancelWorkflowExecutionDecisionAttributes(t.CancelWorkflowExecutionDecisionAttributes),
		RequestCancelExternalWorkflowExecutionDecisionAttributes: ToRequestCancelExternalWorkflowExecutionDecisionAttributes(t.RequestCancelExternalWorkflowExecutionDecisionAttributes),
		RecordMarkerDecisionAttributes:                           ToRecordMarkerDecisionAttributes(t.RecordMarkerDecisionAttributes),
		ContinueAsNewWorkflowExecutionDecisionAttributes:         ToContinueAsNewWorkflowExecutionDecisionAttributes(t.ContinueAsNewWorkflowExecutionDecisionAttributes),
		StartChildWorkflowExecutionDecisionAttributes:            ToStartChildWorkflowExecutionDecisionAttributes(t.StartChildWorkflowExecutionDecisionAttributes),
		SignalExternalWorkflowExecutionDecisionAttributes:        ToSignalExternalWorkflowExecutionDecisionAttributes(t.SignalExternalWorkflowExecutionDecisionAttributes),
		UpsertWorkflowSearchAttributesDecisionAttributes:         ToUpsertWorkflowSearchAttributesDecisionAttributes(t.UpsertWorkflowSearchAttributesDecisionAttributes),
	}
}

// FromDecisionTaskCompletedEventAttributes converts internal DecisionTaskCompletedEventAttributes type to thrift
func FromDecisionTaskCompletedEventAttributes(t *types.DecisionTaskCompletedEventAttributes) *shared.DecisionTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.DecisionTaskCompletedEventAttributes{
		ExecutionContext: t.ExecutionContext,
		ScheduledEventId: &t.ScheduledEventID,
		StartedEventId:   &t.StartedEventID,
		Identity:         &t.Identity,
		BinaryChecksum:   &t.BinaryChecksum,
	}
}

// ToDecisionTaskCompletedEventAttributes converts thrift DecisionTaskCompletedEventAttributes type to internal
func ToDecisionTaskCompletedEventAttributes(t *shared.DecisionTaskCompletedEventAttributes) *types.DecisionTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskCompletedEventAttributes{
		ExecutionContext: t.ExecutionContext,
		ScheduledEventID: t.GetScheduledEventId(),
		StartedEventID:   t.GetStartedEventId(),
		Identity:         t.GetIdentity(),
		BinaryChecksum:   t.GetBinaryChecksum(),
	}
}

// FromDecisionTaskFailedCause converts internal DecisionTaskFailedCause type to thrift
func FromDecisionTaskFailedCause(t *types.DecisionTaskFailedCause) *shared.DecisionTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.DecisionTaskFailedCauseUnhandledDecision:
		v := shared.DecisionTaskFailedCauseUnhandledDecision
		return &v
	case types.DecisionTaskFailedCauseBadScheduleActivityAttributes:
		v := shared.DecisionTaskFailedCauseBadScheduleActivityAttributes
		return &v
	case types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes:
		v := shared.DecisionTaskFailedCauseBadRequestCancelActivityAttributes
		return &v
	case types.DecisionTaskFailedCauseBadStartTimerAttributes:
		v := shared.DecisionTaskFailedCauseBadStartTimerAttributes
		return &v
	case types.DecisionTaskFailedCauseBadCancelTimerAttributes:
		v := shared.DecisionTaskFailedCauseBadCancelTimerAttributes
		return &v
	case types.DecisionTaskFailedCauseBadRecordMarkerAttributes:
		v := shared.DecisionTaskFailedCauseBadRecordMarkerAttributes
		return &v
	case types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseBadContinueAsNewAttributes:
		v := shared.DecisionTaskFailedCauseBadContinueAsNewAttributes
		return &v
	case types.DecisionTaskFailedCauseStartTimerDuplicateID:
		v := shared.DecisionTaskFailedCauseStartTimerDuplicateID
		return &v
	case types.DecisionTaskFailedCauseResetStickyTasklist:
		v := shared.DecisionTaskFailedCauseResetStickyTasklist
		return &v
	case types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure:
		v := shared.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
		return &v
	case types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseBadStartChildExecutionAttributes:
		v := shared.DecisionTaskFailedCauseBadStartChildExecutionAttributes
		return &v
	case types.DecisionTaskFailedCauseForceCloseDecision:
		v := shared.DecisionTaskFailedCauseForceCloseDecision
		return &v
	case types.DecisionTaskFailedCauseFailoverCloseDecision:
		v := shared.DecisionTaskFailedCauseFailoverCloseDecision
		return &v
	case types.DecisionTaskFailedCauseBadSignalInputSize:
		v := shared.DecisionTaskFailedCauseBadSignalInputSize
		return &v
	case types.DecisionTaskFailedCauseResetWorkflow:
		v := shared.DecisionTaskFailedCauseResetWorkflow
		return &v
	case types.DecisionTaskFailedCauseBadBinary:
		v := shared.DecisionTaskFailedCauseBadBinary
		return &v
	case types.DecisionTaskFailedCauseScheduleActivityDuplicateID:
		v := shared.DecisionTaskFailedCauseScheduleActivityDuplicateID
		return &v
	case types.DecisionTaskFailedCauseBadSearchAttributes:
		v := shared.DecisionTaskFailedCauseBadSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// ToDecisionTaskFailedCause converts thrift DecisionTaskFailedCause type to internal
func ToDecisionTaskFailedCause(t *shared.DecisionTaskFailedCause) *types.DecisionTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.DecisionTaskFailedCauseUnhandledDecision:
		v := types.DecisionTaskFailedCauseUnhandledDecision
		return &v
	case shared.DecisionTaskFailedCauseBadScheduleActivityAttributes:
		v := types.DecisionTaskFailedCauseBadScheduleActivityAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadRequestCancelActivityAttributes:
		v := types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadStartTimerAttributes:
		v := types.DecisionTaskFailedCauseBadStartTimerAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadCancelTimerAttributes:
		v := types.DecisionTaskFailedCauseBadCancelTimerAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadRecordMarkerAttributes:
		v := types.DecisionTaskFailedCauseBadRecordMarkerAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadContinueAsNewAttributes:
		v := types.DecisionTaskFailedCauseBadContinueAsNewAttributes
		return &v
	case shared.DecisionTaskFailedCauseStartTimerDuplicateID:
		v := types.DecisionTaskFailedCauseStartTimerDuplicateID
		return &v
	case shared.DecisionTaskFailedCauseResetStickyTasklist:
		v := types.DecisionTaskFailedCauseResetStickyTasklist
		return &v
	case shared.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure:
		v := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
		return &v
	case shared.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseBadStartChildExecutionAttributes:
		v := types.DecisionTaskFailedCauseBadStartChildExecutionAttributes
		return &v
	case shared.DecisionTaskFailedCauseForceCloseDecision:
		v := types.DecisionTaskFailedCauseForceCloseDecision
		return &v
	case shared.DecisionTaskFailedCauseFailoverCloseDecision:
		v := types.DecisionTaskFailedCauseFailoverCloseDecision
		return &v
	case shared.DecisionTaskFailedCauseBadSignalInputSize:
		v := types.DecisionTaskFailedCauseBadSignalInputSize
		return &v
	case shared.DecisionTaskFailedCauseResetWorkflow:
		v := types.DecisionTaskFailedCauseResetWorkflow
		return &v
	case shared.DecisionTaskFailedCauseBadBinary:
		v := types.DecisionTaskFailedCauseBadBinary
		return &v
	case shared.DecisionTaskFailedCauseScheduleActivityDuplicateID:
		v := types.DecisionTaskFailedCauseScheduleActivityDuplicateID
		return &v
	case shared.DecisionTaskFailedCauseBadSearchAttributes:
		v := types.DecisionTaskFailedCauseBadSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// FromDecisionTaskFailedEventAttributes converts internal DecisionTaskFailedEventAttributes type to thrift
func FromDecisionTaskFailedEventAttributes(t *types.DecisionTaskFailedEventAttributes) *shared.DecisionTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.DecisionTaskFailedEventAttributes{
		ScheduledEventId: &t.ScheduledEventID,
		StartedEventId:   &t.StartedEventID,
		Cause:            FromDecisionTaskFailedCause(t.Cause),
		Details:          t.Details,
		Identity:         &t.Identity,
		Reason:           t.Reason,
		BaseRunId:        &t.BaseRunID,
		NewRunId:         &t.NewRunID,
		ForkEventVersion: &t.ForkEventVersion,
		BinaryChecksum:   &t.BinaryChecksum,
		RequestId:        &t.RequestID,
	}
}

// ToDecisionTaskFailedEventAttributes converts thrift DecisionTaskFailedEventAttributes type to internal
func ToDecisionTaskFailedEventAttributes(t *shared.DecisionTaskFailedEventAttributes) *types.DecisionTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskFailedEventAttributes{
		ScheduledEventID: t.GetScheduledEventId(),
		StartedEventID:   t.GetStartedEventId(),
		Cause:            ToDecisionTaskFailedCause(t.Cause),
		Details:          t.Details,
		Identity:         t.GetIdentity(),
		Reason:           t.Reason,
		BaseRunID:        t.GetBaseRunId(),
		NewRunID:         t.GetNewRunId(),
		ForkEventVersion: t.GetForkEventVersion(),
		BinaryChecksum:   t.GetBinaryChecksum(),
		RequestID:        t.GetRequestId(),
	}
}

// FromDecisionTaskScheduledEventAttributes converts internal DecisionTaskScheduledEventAttributes type to thrift
func FromDecisionTaskScheduledEventAttributes(t *types.DecisionTaskScheduledEventAttributes) *shared.DecisionTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.DecisionTaskScheduledEventAttributes{
		TaskList:                   FromTaskList(t.TaskList),
		StartToCloseTimeoutSeconds: t.StartToCloseTimeoutSeconds,
		Attempt:                    &t.Attempt,
	}
}

// ToDecisionTaskScheduledEventAttributes converts thrift DecisionTaskScheduledEventAttributes type to internal
func ToDecisionTaskScheduledEventAttributes(t *shared.DecisionTaskScheduledEventAttributes) *types.DecisionTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskScheduledEventAttributes{
		TaskList:                   ToTaskList(t.TaskList),
		StartToCloseTimeoutSeconds: t.StartToCloseTimeoutSeconds,
		Attempt:                    t.GetAttempt(),
	}
}

// FromDecisionTaskStartedEventAttributes converts internal DecisionTaskStartedEventAttributes type to thrift
func FromDecisionTaskStartedEventAttributes(t *types.DecisionTaskStartedEventAttributes) *shared.DecisionTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.DecisionTaskStartedEventAttributes{
		ScheduledEventId: &t.ScheduledEventID,
		Identity:         &t.Identity,
		RequestId:        &t.RequestID,
	}
}

// ToDecisionTaskStartedEventAttributes converts thrift DecisionTaskStartedEventAttributes type to internal
func ToDecisionTaskStartedEventAttributes(t *shared.DecisionTaskStartedEventAttributes) *types.DecisionTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskStartedEventAttributes{
		ScheduledEventID: t.GetScheduledEventId(),
		Identity:         t.GetIdentity(),
		RequestID:        t.GetRequestId(),
	}
}

// FromDecisionTaskTimedOutCause converts internal DecisionTaskTimedOutCause type to thrift
func FromDecisionTaskTimedOutCause(t *types.DecisionTaskTimedOutCause) *shared.DecisionTaskTimedOutCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.DecisionTaskTimedOutCauseTimeout:
		v := shared.DecisionTaskTimedOutCauseTimeout
		return &v
	case types.DecisionTaskTimedOutCauseReset:
		v := shared.DecisionTaskTimedOutCauseReset
		return &v
	}
	panic("unexpected enum value")
}

// ToDecisionTaskTimedOutCause converts thrift DecisionTaskTimedOutCause type to internal
func ToDecisionTaskTimedOutCause(t *shared.DecisionTaskTimedOutCause) *types.DecisionTaskTimedOutCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.DecisionTaskTimedOutCauseTimeout:
		v := types.DecisionTaskTimedOutCauseTimeout
		return &v
	case shared.DecisionTaskTimedOutCauseReset:
		v := types.DecisionTaskTimedOutCauseReset
		return &v
	}
	panic("unexpected enum value")
}

// FromDecisionTaskTimedOutEventAttributes converts internal DecisionTaskTimedOutEventAttributes type to thrift
func FromDecisionTaskTimedOutEventAttributes(t *types.DecisionTaskTimedOutEventAttributes) *shared.DecisionTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.DecisionTaskTimedOutEventAttributes{
		ScheduledEventId: &t.ScheduledEventID,
		StartedEventId:   &t.StartedEventID,
		TimeoutType:      FromTimeoutType(t.TimeoutType),
		BaseRunId:        &t.BaseRunID,
		NewRunId:         &t.NewRunID,
		ForkEventVersion: &t.ForkEventVersion,
		Reason:           &t.Reason,
		Cause:            FromDecisionTaskTimedOutCause(t.Cause),
		RequestId:        &t.RequestID,
	}
}

// ToDecisionTaskTimedOutEventAttributes converts thrift DecisionTaskTimedOutEventAttributes type to internal
func ToDecisionTaskTimedOutEventAttributes(t *shared.DecisionTaskTimedOutEventAttributes) *types.DecisionTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskTimedOutEventAttributes{
		ScheduledEventID: t.GetScheduledEventId(),
		StartedEventID:   t.GetStartedEventId(),
		TimeoutType:      ToTimeoutType(t.TimeoutType),
		BaseRunID:        t.GetBaseRunId(),
		NewRunID:         t.GetNewRunId(),
		ForkEventVersion: t.GetForkEventVersion(),
		Reason:           t.GetReason(),
		Cause:            ToDecisionTaskTimedOutCause(t.Cause),
		RequestID:        t.GetRequestId(),
	}
}

// FromDecisionType converts internal DecisionType type to thrift
func FromDecisionType(t *types.DecisionType) *shared.DecisionType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.DecisionTypeScheduleActivityTask:
		v := shared.DecisionTypeScheduleActivityTask
		return &v
	case types.DecisionTypeRequestCancelActivityTask:
		v := shared.DecisionTypeRequestCancelActivityTask
		return &v
	case types.DecisionTypeStartTimer:
		v := shared.DecisionTypeStartTimer
		return &v
	case types.DecisionTypeCompleteWorkflowExecution:
		v := shared.DecisionTypeCompleteWorkflowExecution
		return &v
	case types.DecisionTypeFailWorkflowExecution:
		v := shared.DecisionTypeFailWorkflowExecution
		return &v
	case types.DecisionTypeCancelTimer:
		v := shared.DecisionTypeCancelTimer
		return &v
	case types.DecisionTypeCancelWorkflowExecution:
		v := shared.DecisionTypeCancelWorkflowExecution
		return &v
	case types.DecisionTypeRequestCancelExternalWorkflowExecution:
		v := shared.DecisionTypeRequestCancelExternalWorkflowExecution
		return &v
	case types.DecisionTypeRecordMarker:
		v := shared.DecisionTypeRecordMarker
		return &v
	case types.DecisionTypeContinueAsNewWorkflowExecution:
		v := shared.DecisionTypeContinueAsNewWorkflowExecution
		return &v
	case types.DecisionTypeStartChildWorkflowExecution:
		v := shared.DecisionTypeStartChildWorkflowExecution
		return &v
	case types.DecisionTypeSignalExternalWorkflowExecution:
		v := shared.DecisionTypeSignalExternalWorkflowExecution
		return &v
	case types.DecisionTypeUpsertWorkflowSearchAttributes:
		v := shared.DecisionTypeUpsertWorkflowSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// ToDecisionType converts thrift DecisionType type to internal
func ToDecisionType(t *shared.DecisionType) *types.DecisionType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.DecisionTypeScheduleActivityTask:
		v := types.DecisionTypeScheduleActivityTask
		return &v
	case shared.DecisionTypeRequestCancelActivityTask:
		v := types.DecisionTypeRequestCancelActivityTask
		return &v
	case shared.DecisionTypeStartTimer:
		v := types.DecisionTypeStartTimer
		return &v
	case shared.DecisionTypeCompleteWorkflowExecution:
		v := types.DecisionTypeCompleteWorkflowExecution
		return &v
	case shared.DecisionTypeFailWorkflowExecution:
		v := types.DecisionTypeFailWorkflowExecution
		return &v
	case shared.DecisionTypeCancelTimer:
		v := types.DecisionTypeCancelTimer
		return &v
	case shared.DecisionTypeCancelWorkflowExecution:
		v := types.DecisionTypeCancelWorkflowExecution
		return &v
	case shared.DecisionTypeRequestCancelExternalWorkflowExecution:
		v := types.DecisionTypeRequestCancelExternalWorkflowExecution
		return &v
	case shared.DecisionTypeRecordMarker:
		v := types.DecisionTypeRecordMarker
		return &v
	case shared.DecisionTypeContinueAsNewWorkflowExecution:
		v := types.DecisionTypeContinueAsNewWorkflowExecution
		return &v
	case shared.DecisionTypeStartChildWorkflowExecution:
		v := types.DecisionTypeStartChildWorkflowExecution
		return &v
	case shared.DecisionTypeSignalExternalWorkflowExecution:
		v := types.DecisionTypeSignalExternalWorkflowExecution
		return &v
	case shared.DecisionTypeUpsertWorkflowSearchAttributes:
		v := types.DecisionTypeUpsertWorkflowSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// FromDeleteDomainRequest converts internal DeleteDomainRequest type to thrift
func FromDeleteDomainRequest(t *types.DeleteDomainRequest) *shared.DeleteDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.DeleteDomainRequest{
		Name:          &t.Name,
		SecurityToken: &t.SecurityToken,
	}
}

// ToDeleteDomainRequest converts thrift DeleteDomainRequest type to internal
func ToDeleteDomainRequest(t *shared.DeleteDomainRequest) *types.DeleteDomainRequest {
	if t == nil {
		return nil
	}
	return &types.DeleteDomainRequest{
		Name:          t.GetName(),
		SecurityToken: t.GetSecurityToken(),
	}
}

// FromDeprecateDomainRequest converts internal DeprecateDomainRequest type to thrift
func FromDeprecateDomainRequest(t *types.DeprecateDomainRequest) *shared.DeprecateDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.DeprecateDomainRequest{
		Name:          &t.Name,
		SecurityToken: &t.SecurityToken,
	}
}

// ToDeprecateDomainRequest converts thrift DeprecateDomainRequest type to internal
func ToDeprecateDomainRequest(t *shared.DeprecateDomainRequest) *types.DeprecateDomainRequest {
	if t == nil {
		return nil
	}
	return &types.DeprecateDomainRequest{
		Name:          t.GetName(),
		SecurityToken: t.GetSecurityToken(),
	}
}

// FromDescribeDomainRequest converts internal DescribeDomainRequest type to thrift
func FromDescribeDomainRequest(t *types.DescribeDomainRequest) *shared.DescribeDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeDomainRequest{
		Name: t.Name,
		UUID: t.UUID,
	}
}

// ToDescribeDomainRequest converts thrift DescribeDomainRequest type to internal
func ToDescribeDomainRequest(t *shared.DescribeDomainRequest) *types.DescribeDomainRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeDomainRequest{
		Name: t.Name,
		UUID: t.UUID,
	}
}

// FromDescribeDomainResponse converts internal DescribeDomainResponse type to thrift
func FromDescribeDomainResponse(t *types.DescribeDomainResponse) *shared.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeDomainResponse{
		DomainInfo:               FromDomainInfo(t.DomainInfo),
		Configuration:            FromDomainConfiguration(t.Configuration),
		ReplicationConfiguration: FromDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          &t.FailoverVersion,
		IsGlobalDomain:           &t.IsGlobalDomain,
		FailoverInfo:             FromFailoverInfo(t.GetFailoverInfo()),
	}
}

// ToDescribeDomainResponse converts thrift DescribeDomainResponse type to internal
func ToDescribeDomainResponse(t *shared.DescribeDomainResponse) *types.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeDomainResponse{
		DomainInfo:               ToDomainInfo(t.DomainInfo),
		Configuration:            ToDomainConfiguration(t.Configuration),
		ReplicationConfiguration: ToDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          t.GetFailoverVersion(),
		IsGlobalDomain:           t.GetIsGlobalDomain(),
		FailoverInfo:             ToFailoverInfo(t.FailoverInfo),
	}
}

// FromFailoverDomainRequest converts internal FailoverDomainRequest type to thrift
func FromFailoverDomainRequest(t *types.FailoverDomainRequest) *shared.FailoverDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.FailoverDomainRequest{
		DomainName:              &t.DomainName,
		DomainActiveClusterName: t.DomainActiveClusterName,
		ActiveClusters:          FromActiveClusters(t.ActiveClusters),
	}
}

// ToFailoverDomainRequest converts thrift FailoverDomainRequest type to internal
func ToFailoverDomainRequest(t *shared.FailoverDomainRequest) *types.FailoverDomainRequest {
	if t == nil {
		return nil
	}
	return &types.FailoverDomainRequest{
		DomainName:              t.GetDomainName(),
		DomainActiveClusterName: t.DomainActiveClusterName,
		ActiveClusters:          ToActiveClusters(t.ActiveClusters),
	}
}

// FromFailoverDomainResponse converts internal FailoverDomainResponse type to thrift
func FromFailoverDomainResponse(t *types.FailoverDomainResponse) *shared.FailoverDomainResponse {
	if t == nil {
		return nil
	}
	return &shared.FailoverDomainResponse{
		DomainInfo:               FromDomainInfo(t.DomainInfo),
		Configuration:            FromDomainConfiguration(t.Configuration),
		ReplicationConfiguration: FromDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          &t.FailoverVersion,
		IsGlobalDomain:           &t.IsGlobalDomain,
	}
}

// ToFailoverDomainResponse converts thrift FailoverDomainResponse type to internal
func ToFailoverDomainResponse(t *shared.FailoverDomainResponse) *types.FailoverDomainResponse {
	if t == nil {
		return nil
	}
	return &types.FailoverDomainResponse{
		DomainInfo:               ToDomainInfo(t.DomainInfo),
		Configuration:            ToDomainConfiguration(t.Configuration),
		ReplicationConfiguration: ToDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          t.GetFailoverVersion(),
		IsGlobalDomain:           t.GetIsGlobalDomain(),
	}
}

// FromAdminDescribeHistoryHostRequest converts internal DescribeHistoryHostRequest type to thrift
func FromAdminDescribeHistoryHostRequest(t *types.DescribeHistoryHostRequest) *shared.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeHistoryHostRequest{
		HostAddress:      t.HostAddress,
		ShardIdForHost:   t.ShardIDForHost,
		ExecutionForHost: FromWorkflowExecution(t.ExecutionForHost),
	}
}

// ToAdminDescribeHistoryHostRequest converts thrift DescribeHistoryHostRequest type to internal
func ToAdminDescribeHistoryHostRequest(t *shared.DescribeHistoryHostRequest) *types.DescribeHistoryHostRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeHistoryHostRequest{
		HostAddress:      t.HostAddress,
		ShardIDForHost:   t.ShardIdForHost,
		ExecutionForHost: ToWorkflowExecution(t.ExecutionForHost),
	}
}

// FromAdminDescribeShardDistributionRequest converts internal DescribeHistoryHostRequest type to thrift
func FromAdminDescribeShardDistributionRequest(t *types.DescribeShardDistributionRequest) *shared.DescribeShardDistributionRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeShardDistributionRequest{
		PageSize: &t.PageSize,
		PageID:   &t.PageID,
	}
}

// ToAdminDescribeShardDistributionRequest converts thrift DescribeHistoryHostRequest type to internal
func ToAdminDescribeShardDistributionRequest(t *shared.DescribeShardDistributionRequest) *types.DescribeShardDistributionRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeShardDistributionRequest{
		PageSize: t.GetPageSize(),
		PageID:   t.GetPageID(),
	}
}

// FromAdminDescribeHistoryHostResponse converts internal DescribeHistoryHostResponse type to thrift
func FromAdminDescribeHistoryHostResponse(t *types.DescribeHistoryHostResponse) *shared.DescribeHistoryHostResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeHistoryHostResponse{
		NumberOfShards:        &t.NumberOfShards,
		ShardIDs:              t.ShardIDs,
		DomainCache:           FromDomainCacheInfo(t.DomainCache),
		ShardControllerStatus: &t.ShardControllerStatus,
		Address:               &t.Address,
	}
}

// ToAdminDescribeHistoryHostResponse converts thrift DescribeHistoryHostResponse type to internal
func ToAdminDescribeHistoryHostResponse(t *shared.DescribeHistoryHostResponse) *types.DescribeHistoryHostResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeHistoryHostResponse{
		NumberOfShards:        t.GetNumberOfShards(),
		ShardIDs:              t.ShardIDs,
		DomainCache:           ToDomainCacheInfo(t.DomainCache),
		ShardControllerStatus: t.GetShardControllerStatus(),
		Address:               t.GetAddress(),
	}
}

// FromAdminDescribeShardDistributionResponse converts internal DescribeHistoryHostResponse type to thrift
func FromAdminDescribeShardDistributionResponse(t *types.DescribeShardDistributionResponse) *shared.DescribeShardDistributionResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeShardDistributionResponse{
		NumberOfShards: &t.NumberOfShards,
		Shards:         t.Shards,
	}
}

// ToAdminDescribeShardDistributionResponse converts thrift DescribeHistoryHostResponse type to internal
func ToAdminDescribeShardDistributionResponse(t *shared.DescribeShardDistributionResponse) *types.DescribeShardDistributionResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeShardDistributionResponse{
		NumberOfShards: t.GetNumberOfShards(),
		Shards:         t.Shards,
	}
}

// FromAdminDescribeQueueRequest converts internal DescribeQueueRequest type to thrift
func FromAdminDescribeQueueRequest(t *types.DescribeQueueRequest) *shared.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeQueueRequest{
		ShardID:     &t.ShardID,
		ClusterName: &t.ClusterName,
		Type:        t.Type,
	}
}

// ToAdminDescribeQueueRequest converts thrift DescribeQueueRequest type to internal
func ToAdminDescribeQueueRequest(t *shared.DescribeQueueRequest) *types.DescribeQueueRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueRequest{
		ShardID:     t.GetShardID(),
		ClusterName: t.GetClusterName(),
		Type:        t.Type,
	}
}

// FromAdminDescribeQueueResponse converts internal DescribeQueueResponse type to thrift
func FromAdminDescribeQueueResponse(t *types.DescribeQueueResponse) *shared.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

// ToAdminDescribeQueueResponse converts thrift DescribeQueueResponse type to internal
func ToAdminDescribeQueueResponse(t *shared.DescribeQueueResponse) *types.DescribeQueueResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeQueueResponse{
		ProcessingQueueStates: t.ProcessingQueueStates,
	}
}

// FromDescribeTaskListRequest converts internal DescribeTaskListRequest type to thrift
func FromDescribeTaskListRequest(t *types.DescribeTaskListRequest) *shared.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeTaskListRequest{
		Domain:                &t.Domain,
		TaskList:              FromTaskList(t.TaskList),
		TaskListType:          FromTaskListType(t.TaskListType),
		IncludeTaskListStatus: &t.IncludeTaskListStatus,
	}
}

// ToDescribeTaskListRequest converts thrift DescribeTaskListRequest type to internal
func ToDescribeTaskListRequest(t *shared.DescribeTaskListRequest) *types.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeTaskListRequest{
		Domain:                t.GetDomain(),
		TaskList:              ToTaskList(t.TaskList),
		TaskListType:          ToTaskListType(t.TaskListType),
		IncludeTaskListStatus: t.GetIncludeTaskListStatus(),
	}
}

// FromDescribeTaskListResponse converts internal DescribeTaskListResponse type to thrift
func FromDescribeTaskListResponse(t *types.DescribeTaskListResponse) *shared.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeTaskListResponse{
		Pollers:        FromPollerInfoArray(t.Pollers),
		TaskListStatus: FromTaskListStatus(t.TaskListStatus),
		TaskList:       FromTaskList(t.TaskList),
	}
}

// ToDescribeTaskListResponse converts thrift DescribeTaskListResponse type to internal
func ToDescribeTaskListResponse(t *shared.DescribeTaskListResponse) *types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeTaskListResponse{
		Pollers:        ToPollerInfoArray(t.Pollers),
		TaskListStatus: ToTaskListStatus(t.TaskListStatus),
		TaskList:       ToTaskList(t.TaskList),
	}
}

// FromDescribeWorkflowExecutionRequest converts internal DescribeWorkflowExecutionRequest type to thrift
func FromDescribeWorkflowExecutionRequest(t *types.DescribeWorkflowExecutionRequest) *shared.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeWorkflowExecutionRequest{
		Domain:                &t.Domain,
		Execution:             FromWorkflowExecution(t.Execution),
		QueryConsistencyLevel: FromQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// ToDescribeWorkflowExecutionRequest converts thrift DescribeWorkflowExecutionRequest type to internal
func ToDescribeWorkflowExecutionRequest(t *shared.DescribeWorkflowExecutionRequest) *types.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeWorkflowExecutionRequest{
		Domain:                t.GetDomain(),
		Execution:             ToWorkflowExecution(t.Execution),
		QueryConsistencyLevel: ToQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// FromDescribeWorkflowExecutionResponse converts internal DescribeWorkflowExecutionResponse type to thrift
func FromDescribeWorkflowExecutionResponse(t *types.DescribeWorkflowExecutionResponse) *shared.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: FromWorkflowExecutionConfiguration(t.ExecutionConfiguration),
		WorkflowExecutionInfo:  FromWorkflowExecutionInfo(t.WorkflowExecutionInfo),
		PendingActivities:      FromPendingActivityInfoArray(t.PendingActivities),
		PendingChildren:        FromPendingChildExecutionInfoArray(t.PendingChildren),
		PendingDecision:        FromPendingDecisionInfo(t.PendingDecision),
	}
}

// ToDescribeWorkflowExecutionResponse converts thrift DescribeWorkflowExecutionResponse type to internal
func ToDescribeWorkflowExecutionResponse(t *shared.DescribeWorkflowExecutionResponse) *types.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: ToWorkflowExecutionConfiguration(t.ExecutionConfiguration),
		WorkflowExecutionInfo:  ToWorkflowExecutionInfo(t.WorkflowExecutionInfo),
		PendingActivities:      ToPendingActivityInfoArray(t.PendingActivities),
		PendingChildren:        ToPendingChildExecutionInfoArray(t.PendingChildren),
		PendingDecision:        ToPendingDecisionInfo(t.PendingDecision),
	}
}

// FromDiagnoseWorkflowExecutionRequest converts internal DiagnoseWorkflowExecutionRequest type to thrift
func FromDiagnoseWorkflowExecutionRequest(t *types.DiagnoseWorkflowExecutionRequest) *shared.DiagnoseWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.DiagnoseWorkflowExecutionRequest{
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.GetWorkflowExecution()),
		Identity:          &t.Identity,
	}
}

// ToDiagnoseWorkflowExecutionRequest converts thrift DiagnoseWorkflowExecutionRequest type to internal
func ToDiagnoseWorkflowExecutionRequest(t *shared.DiagnoseWorkflowExecutionRequest) *types.DiagnoseWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.DiagnoseWorkflowExecutionRequest{
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.GetWorkflowExecution()),
		Identity:          t.GetIdentity(),
	}
}

// FromDiagnoseWorkflowExecutionResponse converts internal DiagnoseWorkflowExecutionResponse type to thrift
func FromDiagnoseWorkflowExecutionResponse(t *types.DiagnoseWorkflowExecutionResponse) *shared.DiagnoseWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &shared.DiagnoseWorkflowExecutionResponse{
		Domain:                      &t.Domain,
		DiagnosticWorkflowExecution: FromWorkflowExecution(t.GetDiagnosticWorkflowExecution()),
	}
}

// ToDiagnoseWorkflowExecutionResponse converts thrift DiagnoseeWorkflowExecutionResponse type to internal
func ToDiagnoseWorkflowExecutionResponse(t *shared.DiagnoseWorkflowExecutionResponse) *types.DiagnoseWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.DiagnoseWorkflowExecutionResponse{
		Domain:                      t.GetDomain(),
		DiagnosticWorkflowExecution: ToWorkflowExecution(t.GetDiagnosticWorkflowExecution()),
	}
}

// FromDomainAlreadyExistsError converts internal DomainAlreadyExistsError type to thrift
func FromDomainAlreadyExistsError(t *types.DomainAlreadyExistsError) *shared.DomainAlreadyExistsError {
	if t == nil {
		return nil
	}
	return &shared.DomainAlreadyExistsError{
		Message: t.Message,
	}
}

// ToDomainAlreadyExistsError converts thrift DomainAlreadyExistsError type to internal
func ToDomainAlreadyExistsError(t *shared.DomainAlreadyExistsError) *types.DomainAlreadyExistsError {
	if t == nil {
		return nil
	}
	return &types.DomainAlreadyExistsError{
		Message: t.Message,
	}
}

// FromDomainCacheInfo converts internal DomainCacheInfo type to thrift
func FromDomainCacheInfo(t *types.DomainCacheInfo) *shared.DomainCacheInfo {
	if t == nil {
		return nil
	}
	return &shared.DomainCacheInfo{
		NumOfItemsInCacheByID:   &t.NumOfItemsInCacheByID,
		NumOfItemsInCacheByName: &t.NumOfItemsInCacheByName,
	}
}

// ToDomainCacheInfo converts thrift DomainCacheInfo type to internal
func ToDomainCacheInfo(t *shared.DomainCacheInfo) *types.DomainCacheInfo {
	if t == nil {
		return nil
	}
	return &types.DomainCacheInfo{
		NumOfItemsInCacheByID:   t.GetNumOfItemsInCacheByID(),
		NumOfItemsInCacheByName: t.GetNumOfItemsInCacheByName(),
	}
}

// FromDomainConfiguration converts internal DomainConfiguration type to thrift
func FromDomainConfiguration(t *types.DomainConfiguration) *shared.DomainConfiguration {
	if t == nil {
		return nil
	}
	return &shared.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: &t.WorkflowExecutionRetentionPeriodInDays,
		EmitMetric:                             &t.EmitMetric,
		BadBinaries:                            FromBadBinaries(t.BadBinaries),
		HistoryArchivalStatus:                  FromArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalURI:                     &t.HistoryArchivalURI,
		VisibilityArchivalStatus:               FromArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalURI:                  &t.VisibilityArchivalURI,
		Isolationgroups:                        FromIsolationGroupConfig(t.IsolationGroups),
		AsyncWorkflowConfiguration:             FromDomainAsyncWorkflowConfiguraton(t.AsyncWorkflowConfig),
	}
}

// ToDomainConfiguration converts thrift DomainConfiguration type to internal
func ToDomainConfiguration(t *shared.DomainConfiguration) *types.DomainConfiguration {
	if t == nil {
		return nil
	}
	return &types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: t.GetWorkflowExecutionRetentionPeriodInDays(),
		EmitMetric:                             t.GetEmitMetric(),
		BadBinaries:                            ToBadBinaries(t.BadBinaries),
		HistoryArchivalStatus:                  ToArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalURI:                     t.GetHistoryArchivalURI(),
		VisibilityArchivalStatus:               ToArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalURI:                  t.GetVisibilityArchivalURI(),
		IsolationGroups:                        ToIsolationGroupConfig(t.Isolationgroups),
		AsyncWorkflowConfig:                    ToDomainAsyncWorkflowConfiguraton(t.AsyncWorkflowConfiguration),
	}
}

// FromDomainInfo converts internal DomainInfo type to thrift
func FromDomainInfo(t *types.DomainInfo) *shared.DomainInfo {
	if t == nil {
		return nil
	}
	return &shared.DomainInfo{
		Name:        &t.Name,
		Status:      FromDomainStatus(t.Status),
		Description: &t.Description,
		OwnerEmail:  &t.OwnerEmail,
		Data:        t.Data,
		UUID:        &t.UUID,
	}
}

// ToDomainInfo converts thrift DomainInfo type to internal
func ToDomainInfo(t *shared.DomainInfo) *types.DomainInfo {
	if t == nil {
		return nil
	}
	return &types.DomainInfo{
		Name:        t.GetName(),
		Status:      ToDomainStatus(t.Status),
		Description: t.GetDescription(),
		OwnerEmail:  t.GetOwnerEmail(),
		Data:        t.Data,
		UUID:        t.GetUUID(),
	}
}

// FromFailoverInfo converts internal FailoverInfo type to thrift
func FromFailoverInfo(t *types.FailoverInfo) *shared.FailoverInfo {
	if t == nil {
		return nil
	}
	return &shared.FailoverInfo{
		FailoverVersion:         &t.FailoverVersion,
		FailoverStartTimestamp:  &t.FailoverStartTimestamp,
		FailoverExpireTimestamp: &t.FailoverExpireTimestamp,
		CompletedShardCount:     &t.CompletedShardCount,
		PendingShards:           t.GetPendingShards(),
	}
}

// ToFailoverInfo converts thrift FailoverInfo type to internal
func ToFailoverInfo(t *shared.FailoverInfo) *types.FailoverInfo {
	if t == nil {
		return nil
	}
	return &types.FailoverInfo{
		FailoverVersion:         t.GetFailoverVersion(),
		FailoverStartTimestamp:  t.GetFailoverStartTimestamp(),
		FailoverExpireTimestamp: t.GetFailoverExpireTimestamp(),
		CompletedShardCount:     t.GetCompletedShardCount(),
		PendingShards:           t.GetPendingShards(),
	}
}

// FromDomainNotActiveError converts internal DomainNotActiveError type to thrift
func FromDomainNotActiveError(t *types.DomainNotActiveError) *shared.DomainNotActiveError {
	if t == nil {
		return nil
	}
	return &shared.DomainNotActiveError{
		Message:        t.Message,
		DomainName:     t.DomainName,
		CurrentCluster: t.CurrentCluster,
		ActiveCluster:  t.ActiveCluster,
		ActiveClusters: t.ActiveClusters,
	}
}

// ToDomainNotActiveError converts thrift DomainNotActiveError type to internal
func ToDomainNotActiveError(t *shared.DomainNotActiveError) *types.DomainNotActiveError {
	if t == nil {
		return nil
	}
	return &types.DomainNotActiveError{
		Message:        t.Message,
		DomainName:     t.DomainName,
		CurrentCluster: t.CurrentCluster,
		ActiveCluster:  t.ActiveCluster,
		ActiveClusters: t.ActiveClusters,
	}
}

// FromDomainReplicationConfiguration converts internal DomainReplicationConfiguration type to thrift
func FromDomainReplicationConfiguration(t *types.DomainReplicationConfiguration) *shared.DomainReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &shared.DomainReplicationConfiguration{
		ActiveClusterName: &t.ActiveClusterName,
		Clusters:          FromClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusters:    FromActiveClusters(t.ActiveClusters),
	}
}

// ToDomainReplicationConfiguration converts thrift DomainReplicationConfiguration type to internal
func ToDomainReplicationConfiguration(t *shared.DomainReplicationConfiguration) *types.DomainReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &types.DomainReplicationConfiguration{
		ActiveClusterName: t.GetActiveClusterName(),
		Clusters:          ToClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusters:    ToActiveClusters(t.ActiveClusters),
	}
}

// FromActiveClusters converts internal ActiveClusters type to thrift
func FromActiveClusters(t *types.ActiveClusters) *shared.ActiveClusters {
	if t == nil {
		return nil
	}

	var activeClustersByClusterAttribute map[string]*shared.ClusterAttributeScope
	if t.AttributeScopes != nil {
		activeClustersByClusterAttribute = make(map[string]*shared.ClusterAttributeScope)
		for scopeType, scope := range t.AttributeScopes {
			activeClustersByClusterAttribute[scopeType] = FromClusterAttributeScope(&scope)
		}
	}

	return &shared.ActiveClusters{
		ActiveClustersByClusterAttribute: activeClustersByClusterAttribute,
	}
}

// ToActiveClusters converts thrift ActiveClusters type to internal
func ToActiveClusters(t *shared.ActiveClusters) *types.ActiveClusters {
	if t == nil {
		return nil
	}

	var attributeScopes map[string]types.ClusterAttributeScope
	if t.ActiveClustersByClusterAttribute != nil {
		attributeScopes = make(map[string]types.ClusterAttributeScope)
		for scopeType, scope := range t.ActiveClustersByClusterAttribute {
			if converted := ToClusterAttributeScope(scope); converted != nil {
				attributeScopes[scopeType] = *converted
			}
		}
	}

	return &types.ActiveClusters{
		AttributeScopes: attributeScopes,
	}
}

// FromClusterAttributeScope converts internal ClusterAttributeScope type to thrift
func FromClusterAttributeScope(t *types.ClusterAttributeScope) *shared.ClusterAttributeScope {
	if t == nil {
		return nil
	}

	var clusterAttributes map[string]*shared.ActiveClusterInfo
	if len(t.ClusterAttributes) > 0 {
		clusterAttributes = make(map[string]*shared.ActiveClusterInfo)
		for name, clusterInfo := range t.ClusterAttributes {
			clusterAttributes[name] = &shared.ActiveClusterInfo{
				ActiveClusterName: &clusterInfo.ActiveClusterName,
				FailoverVersion:   &clusterInfo.FailoverVersion,
			}
		}
	}

	return &shared.ClusterAttributeScope{
		ClusterAttributes: clusterAttributes,
	}
}

// ToClusterAttributeScope converts thrift ClusterAttributeScope type to internal
func ToClusterAttributeScope(t *shared.ClusterAttributeScope) *types.ClusterAttributeScope {
	if t == nil {
		return nil
	}

	var clusterAttributes map[string]types.ActiveClusterInfo
	if len(t.ClusterAttributes) > 0 {
		clusterAttributes = make(map[string]types.ActiveClusterInfo)
		for name, clusterInfo := range t.ClusterAttributes {
			if clusterInfo != nil {
				clusterAttributes[name] = types.ActiveClusterInfo{
					ActiveClusterName: *clusterInfo.ActiveClusterName,
					FailoverVersion:   *clusterInfo.FailoverVersion,
				}
			}
		}
	}

	return &types.ClusterAttributeScope{
		ClusterAttributes: clusterAttributes,
	}
}

// FromDomainStatus converts internal DomainStatus type to thrift
func FromDomainStatus(t *types.DomainStatus) *shared.DomainStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case types.DomainStatusRegistered:
		v := shared.DomainStatusRegistered
		return &v
	case types.DomainStatusDeprecated:
		v := shared.DomainStatusDeprecated
		return &v
	case types.DomainStatusDeleted:
		v := shared.DomainStatusDeleted
		return &v
	}
	panic("unexpected enum value")
}

// ToDomainStatus converts thrift DomainStatus type to internal
func ToDomainStatus(t *shared.DomainStatus) *types.DomainStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.DomainStatusRegistered:
		v := types.DomainStatusRegistered
		return &v
	case shared.DomainStatusDeprecated:
		v := types.DomainStatusDeprecated
		return &v
	case shared.DomainStatusDeleted:
		v := types.DomainStatusDeleted
		return &v
	}
	panic("unexpected enum value")
}

// FromEncodingType converts internal EncodingType type to thrift
func FromEncodingType(t *types.EncodingType) *shared.EncodingType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.EncodingTypeThriftRW:
		v := shared.EncodingTypeThriftRW
		return &v
	case types.EncodingTypeJSON:
		v := shared.EncodingTypeJSON
		return &v
	}
	panic("unexpected enum value")
}

// ToEncodingType converts thrift EncodingType type to internal
func ToEncodingType(t *shared.EncodingType) *types.EncodingType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.EncodingTypeThriftRW:
		v := types.EncodingTypeThriftRW
		return &v
	case shared.EncodingTypeJSON:
		v := types.EncodingTypeJSON
		return &v
	}
	panic("unexpected enum value")
}

// FromEntityNotExistsError converts internal EntityNotExistsError type to thrift
func FromEntityNotExistsError(t *types.EntityNotExistsError) *shared.EntityNotExistsError {
	if t == nil {
		return nil
	}
	return &shared.EntityNotExistsError{
		Message:        t.Message,
		CurrentCluster: &t.CurrentCluster,
		ActiveCluster:  &t.ActiveCluster,
		ActiveClusters: t.ActiveClusters,
	}
}

// ToEntityNotExistsError converts thrift EntityNotExistsError type to internal
func ToEntityNotExistsError(t *shared.EntityNotExistsError) *types.EntityNotExistsError {
	if t == nil {
		return nil
	}
	return &types.EntityNotExistsError{
		Message:        t.Message,
		CurrentCluster: t.GetCurrentCluster(),
		ActiveCluster:  t.GetActiveCluster(),
		ActiveClusters: t.GetActiveClusters(),
	}
}

// FromWorkflowExecutionAlreadyCompletedError converts internal WorkflowExecutionAlreadyCompletedError type to thrift
func FromWorkflowExecutionAlreadyCompletedError(t *types.WorkflowExecutionAlreadyCompletedError) *shared.WorkflowExecutionAlreadyCompletedError {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionAlreadyCompletedError{
		Message: t.Message,
	}
}

// ToWorkflowExecutionAlreadyCompletedError converts thrift WorkflowExecutionAlreadyCompletedError type to internal
func ToWorkflowExecutionAlreadyCompletedError(t *shared.WorkflowExecutionAlreadyCompletedError) *types.WorkflowExecutionAlreadyCompletedError {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionAlreadyCompletedError{
		Message: t.Message,
	}
}

// FromEventType converts internal EventType type to thrift
func FromEventType(t *types.EventType) *shared.EventType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.EventTypeWorkflowExecutionStarted:
		v := shared.EventTypeWorkflowExecutionStarted
		return &v
	case types.EventTypeWorkflowExecutionCompleted:
		v := shared.EventTypeWorkflowExecutionCompleted
		return &v
	case types.EventTypeWorkflowExecutionFailed:
		v := shared.EventTypeWorkflowExecutionFailed
		return &v
	case types.EventTypeWorkflowExecutionTimedOut:
		v := shared.EventTypeWorkflowExecutionTimedOut
		return &v
	case types.EventTypeDecisionTaskScheduled:
		v := shared.EventTypeDecisionTaskScheduled
		return &v
	case types.EventTypeDecisionTaskStarted:
		v := shared.EventTypeDecisionTaskStarted
		return &v
	case types.EventTypeDecisionTaskCompleted:
		v := shared.EventTypeDecisionTaskCompleted
		return &v
	case types.EventTypeDecisionTaskTimedOut:
		v := shared.EventTypeDecisionTaskTimedOut
		return &v
	case types.EventTypeDecisionTaskFailed:
		v := shared.EventTypeDecisionTaskFailed
		return &v
	case types.EventTypeActivityTaskScheduled:
		v := shared.EventTypeActivityTaskScheduled
		return &v
	case types.EventTypeActivityTaskStarted:
		v := shared.EventTypeActivityTaskStarted
		return &v
	case types.EventTypeActivityTaskCompleted:
		v := shared.EventTypeActivityTaskCompleted
		return &v
	case types.EventTypeActivityTaskFailed:
		v := shared.EventTypeActivityTaskFailed
		return &v
	case types.EventTypeActivityTaskTimedOut:
		v := shared.EventTypeActivityTaskTimedOut
		return &v
	case types.EventTypeActivityTaskCancelRequested:
		v := shared.EventTypeActivityTaskCancelRequested
		return &v
	case types.EventTypeRequestCancelActivityTaskFailed:
		v := shared.EventTypeRequestCancelActivityTaskFailed
		return &v
	case types.EventTypeActivityTaskCanceled:
		v := shared.EventTypeActivityTaskCanceled
		return &v
	case types.EventTypeTimerStarted:
		v := shared.EventTypeTimerStarted
		return &v
	case types.EventTypeTimerFired:
		v := shared.EventTypeTimerFired
		return &v
	case types.EventTypeCancelTimerFailed:
		v := shared.EventTypeCancelTimerFailed
		return &v
	case types.EventTypeTimerCanceled:
		v := shared.EventTypeTimerCanceled
		return &v
	case types.EventTypeWorkflowExecutionCancelRequested:
		v := shared.EventTypeWorkflowExecutionCancelRequested
		return &v
	case types.EventTypeWorkflowExecutionCanceled:
		v := shared.EventTypeWorkflowExecutionCanceled
		return &v
	case types.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
		v := shared.EventTypeRequestCancelExternalWorkflowExecutionInitiated
		return &v
	case types.EventTypeRequestCancelExternalWorkflowExecutionFailed:
		v := shared.EventTypeRequestCancelExternalWorkflowExecutionFailed
		return &v
	case types.EventTypeExternalWorkflowExecutionCancelRequested:
		v := shared.EventTypeExternalWorkflowExecutionCancelRequested
		return &v
	case types.EventTypeMarkerRecorded:
		v := shared.EventTypeMarkerRecorded
		return &v
	case types.EventTypeWorkflowExecutionSignaled:
		v := shared.EventTypeWorkflowExecutionSignaled
		return &v
	case types.EventTypeWorkflowExecutionTerminated:
		v := shared.EventTypeWorkflowExecutionTerminated
		return &v
	case types.EventTypeWorkflowExecutionContinuedAsNew:
		v := shared.EventTypeWorkflowExecutionContinuedAsNew
		return &v
	case types.EventTypeStartChildWorkflowExecutionInitiated:
		v := shared.EventTypeStartChildWorkflowExecutionInitiated
		return &v
	case types.EventTypeStartChildWorkflowExecutionFailed:
		v := shared.EventTypeStartChildWorkflowExecutionFailed
		return &v
	case types.EventTypeChildWorkflowExecutionStarted:
		v := shared.EventTypeChildWorkflowExecutionStarted
		return &v
	case types.EventTypeChildWorkflowExecutionCompleted:
		v := shared.EventTypeChildWorkflowExecutionCompleted
		return &v
	case types.EventTypeChildWorkflowExecutionFailed:
		v := shared.EventTypeChildWorkflowExecutionFailed
		return &v
	case types.EventTypeChildWorkflowExecutionCanceled:
		v := shared.EventTypeChildWorkflowExecutionCanceled
		return &v
	case types.EventTypeChildWorkflowExecutionTimedOut:
		v := shared.EventTypeChildWorkflowExecutionTimedOut
		return &v
	case types.EventTypeChildWorkflowExecutionTerminated:
		v := shared.EventTypeChildWorkflowExecutionTerminated
		return &v
	case types.EventTypeSignalExternalWorkflowExecutionInitiated:
		v := shared.EventTypeSignalExternalWorkflowExecutionInitiated
		return &v
	case types.EventTypeSignalExternalWorkflowExecutionFailed:
		v := shared.EventTypeSignalExternalWorkflowExecutionFailed
		return &v
	case types.EventTypeExternalWorkflowExecutionSignaled:
		v := shared.EventTypeExternalWorkflowExecutionSignaled
		return &v
	case types.EventTypeUpsertWorkflowSearchAttributes:
		v := shared.EventTypeUpsertWorkflowSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// ToEventType converts thrift EventType type to internal
func ToEventType(t *shared.EventType) *types.EventType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.EventTypeWorkflowExecutionStarted:
		v := types.EventTypeWorkflowExecutionStarted
		return &v
	case shared.EventTypeWorkflowExecutionCompleted:
		v := types.EventTypeWorkflowExecutionCompleted
		return &v
	case shared.EventTypeWorkflowExecutionFailed:
		v := types.EventTypeWorkflowExecutionFailed
		return &v
	case shared.EventTypeWorkflowExecutionTimedOut:
		v := types.EventTypeWorkflowExecutionTimedOut
		return &v
	case shared.EventTypeDecisionTaskScheduled:
		v := types.EventTypeDecisionTaskScheduled
		return &v
	case shared.EventTypeDecisionTaskStarted:
		v := types.EventTypeDecisionTaskStarted
		return &v
	case shared.EventTypeDecisionTaskCompleted:
		v := types.EventTypeDecisionTaskCompleted
		return &v
	case shared.EventTypeDecisionTaskTimedOut:
		v := types.EventTypeDecisionTaskTimedOut
		return &v
	case shared.EventTypeDecisionTaskFailed:
		v := types.EventTypeDecisionTaskFailed
		return &v
	case shared.EventTypeActivityTaskScheduled:
		v := types.EventTypeActivityTaskScheduled
		return &v
	case shared.EventTypeActivityTaskStarted:
		v := types.EventTypeActivityTaskStarted
		return &v
	case shared.EventTypeActivityTaskCompleted:
		v := types.EventTypeActivityTaskCompleted
		return &v
	case shared.EventTypeActivityTaskFailed:
		v := types.EventTypeActivityTaskFailed
		return &v
	case shared.EventTypeActivityTaskTimedOut:
		v := types.EventTypeActivityTaskTimedOut
		return &v
	case shared.EventTypeActivityTaskCancelRequested:
		v := types.EventTypeActivityTaskCancelRequested
		return &v
	case shared.EventTypeRequestCancelActivityTaskFailed:
		v := types.EventTypeRequestCancelActivityTaskFailed
		return &v
	case shared.EventTypeActivityTaskCanceled:
		v := types.EventTypeActivityTaskCanceled
		return &v
	case shared.EventTypeTimerStarted:
		v := types.EventTypeTimerStarted
		return &v
	case shared.EventTypeTimerFired:
		v := types.EventTypeTimerFired
		return &v
	case shared.EventTypeCancelTimerFailed:
		v := types.EventTypeCancelTimerFailed
		return &v
	case shared.EventTypeTimerCanceled:
		v := types.EventTypeTimerCanceled
		return &v
	case shared.EventTypeWorkflowExecutionCancelRequested:
		v := types.EventTypeWorkflowExecutionCancelRequested
		return &v
	case shared.EventTypeWorkflowExecutionCanceled:
		v := types.EventTypeWorkflowExecutionCanceled
		return &v
	case shared.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
		v := types.EventTypeRequestCancelExternalWorkflowExecutionInitiated
		return &v
	case shared.EventTypeRequestCancelExternalWorkflowExecutionFailed:
		v := types.EventTypeRequestCancelExternalWorkflowExecutionFailed
		return &v
	case shared.EventTypeExternalWorkflowExecutionCancelRequested:
		v := types.EventTypeExternalWorkflowExecutionCancelRequested
		return &v
	case shared.EventTypeMarkerRecorded:
		v := types.EventTypeMarkerRecorded
		return &v
	case shared.EventTypeWorkflowExecutionSignaled:
		v := types.EventTypeWorkflowExecutionSignaled
		return &v
	case shared.EventTypeWorkflowExecutionTerminated:
		v := types.EventTypeWorkflowExecutionTerminated
		return &v
	case shared.EventTypeWorkflowExecutionContinuedAsNew:
		v := types.EventTypeWorkflowExecutionContinuedAsNew
		return &v
	case shared.EventTypeStartChildWorkflowExecutionInitiated:
		v := types.EventTypeStartChildWorkflowExecutionInitiated
		return &v
	case shared.EventTypeStartChildWorkflowExecutionFailed:
		v := types.EventTypeStartChildWorkflowExecutionFailed
		return &v
	case shared.EventTypeChildWorkflowExecutionStarted:
		v := types.EventTypeChildWorkflowExecutionStarted
		return &v
	case shared.EventTypeChildWorkflowExecutionCompleted:
		v := types.EventTypeChildWorkflowExecutionCompleted
		return &v
	case shared.EventTypeChildWorkflowExecutionFailed:
		v := types.EventTypeChildWorkflowExecutionFailed
		return &v
	case shared.EventTypeChildWorkflowExecutionCanceled:
		v := types.EventTypeChildWorkflowExecutionCanceled
		return &v
	case shared.EventTypeChildWorkflowExecutionTimedOut:
		v := types.EventTypeChildWorkflowExecutionTimedOut
		return &v
	case shared.EventTypeChildWorkflowExecutionTerminated:
		v := types.EventTypeChildWorkflowExecutionTerminated
		return &v
	case shared.EventTypeSignalExternalWorkflowExecutionInitiated:
		v := types.EventTypeSignalExternalWorkflowExecutionInitiated
		return &v
	case shared.EventTypeSignalExternalWorkflowExecutionFailed:
		v := types.EventTypeSignalExternalWorkflowExecutionFailed
		return &v
	case shared.EventTypeExternalWorkflowExecutionSignaled:
		v := types.EventTypeExternalWorkflowExecutionSignaled
		return &v
	case shared.EventTypeUpsertWorkflowSearchAttributes:
		v := types.EventTypeUpsertWorkflowSearchAttributes
		return &v
	}
	panic("unexpected enum value")
}

// FromExternalWorkflowExecutionCancelRequestedEventAttributes converts internal ExternalWorkflowExecutionCancelRequestedEventAttributes type to thrift
func FromExternalWorkflowExecutionCancelRequestedEventAttributes(t *types.ExternalWorkflowExecutionCancelRequestedEventAttributes) *shared.ExternalWorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventId:  &t.InitiatedEventID,
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
	}
}

// ToExternalWorkflowExecutionCancelRequestedEventAttributes converts thrift ExternalWorkflowExecutionCancelRequestedEventAttributes type to internal
func ToExternalWorkflowExecutionCancelRequestedEventAttributes(t *shared.ExternalWorkflowExecutionCancelRequestedEventAttributes) *types.ExternalWorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventID:  t.GetInitiatedEventId(),
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

// FromExternalWorkflowExecutionSignaledEventAttributes converts internal ExternalWorkflowExecutionSignaledEventAttributes type to thrift
func FromExternalWorkflowExecutionSignaledEventAttributes(t *types.ExternalWorkflowExecutionSignaledEventAttributes) *shared.ExternalWorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventId:  &t.InitiatedEventID,
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Control:           t.Control,
	}
}

// ToExternalWorkflowExecutionSignaledEventAttributes converts thrift ExternalWorkflowExecutionSignaledEventAttributes type to internal
func ToExternalWorkflowExecutionSignaledEventAttributes(t *shared.ExternalWorkflowExecutionSignaledEventAttributes) *types.ExternalWorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventID:  t.GetInitiatedEventId(),
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Control:           t.Control,
	}
}

// FromFailWorkflowExecutionDecisionAttributes converts internal FailWorkflowExecutionDecisionAttributes type to thrift
func FromFailWorkflowExecutionDecisionAttributes(t *types.FailWorkflowExecutionDecisionAttributes) *shared.FailWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.FailWorkflowExecutionDecisionAttributes{
		Reason:  t.Reason,
		Details: t.Details,
	}
}

// ToFailWorkflowExecutionDecisionAttributes converts thrift FailWorkflowExecutionDecisionAttributes type to internal
func ToFailWorkflowExecutionDecisionAttributes(t *shared.FailWorkflowExecutionDecisionAttributes) *types.FailWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.FailWorkflowExecutionDecisionAttributes{
		Reason:  t.Reason,
		Details: t.Details,
	}
}

// FromGetSearchAttributesResponse converts internal GetSearchAttributesResponse type to thrift
func FromGetSearchAttributesResponse(t *types.GetSearchAttributesResponse) *shared.GetSearchAttributesResponse {
	if t == nil {
		return nil
	}
	return &shared.GetSearchAttributesResponse{
		Keys: FromIndexedValueTypeMap(t.Keys),
	}
}

// ToGetSearchAttributesResponse converts thrift GetSearchAttributesResponse type to internal
func ToGetSearchAttributesResponse(t *shared.GetSearchAttributesResponse) *types.GetSearchAttributesResponse {
	if t == nil {
		return nil
	}
	return &types.GetSearchAttributesResponse{
		Keys: ToIndexedValueTypeMap(t.Keys),
	}
}

// FromGetWorkflowExecutionHistoryRequest converts internal GetWorkflowExecutionHistoryRequest type to thrift
func FromGetWorkflowExecutionHistoryRequest(t *types.GetWorkflowExecutionHistoryRequest) *shared.GetWorkflowExecutionHistoryRequest {
	if t == nil {
		return nil
	}
	return &shared.GetWorkflowExecutionHistoryRequest{
		Domain:                 &t.Domain,
		Execution:              FromWorkflowExecution(t.Execution),
		MaximumPageSize:        &t.MaximumPageSize,
		NextPageToken:          t.NextPageToken,
		WaitForNewEvent:        &t.WaitForNewEvent,
		HistoryEventFilterType: FromHistoryEventFilterType(t.HistoryEventFilterType),
		SkipArchival:           &t.SkipArchival,
		QueryConsistencyLevel:  FromQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// ToGetWorkflowExecutionHistoryRequest converts thrift GetWorkflowExecutionHistoryRequest type to internal
func ToGetWorkflowExecutionHistoryRequest(t *shared.GetWorkflowExecutionHistoryRequest) *types.GetWorkflowExecutionHistoryRequest {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionHistoryRequest{
		Domain:                 t.GetDomain(),
		Execution:              ToWorkflowExecution(t.Execution),
		MaximumPageSize:        t.GetMaximumPageSize(),
		NextPageToken:          t.NextPageToken,
		WaitForNewEvent:        t.GetWaitForNewEvent(),
		HistoryEventFilterType: ToHistoryEventFilterType(t.HistoryEventFilterType),
		SkipArchival:           t.GetSkipArchival(),
		QueryConsistencyLevel:  ToQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// FromGetWorkflowExecutionHistoryResponse converts internal GetWorkflowExecutionHistoryResponse type to thrift
func FromGetWorkflowExecutionHistoryResponse(t *types.GetWorkflowExecutionHistoryResponse) *shared.GetWorkflowExecutionHistoryResponse {
	if t == nil {
		return nil
	}
	return &shared.GetWorkflowExecutionHistoryResponse{
		History:       FromHistory(t.History),
		RawHistory:    FromDataBlobArray(t.RawHistory),
		NextPageToken: t.NextPageToken,
		Archived:      &t.Archived,
	}
}

// ToGetWorkflowExecutionHistoryResponse converts thrift GetWorkflowExecutionHistoryResponse type to internal
func ToGetWorkflowExecutionHistoryResponse(t *shared.GetWorkflowExecutionHistoryResponse) *types.GetWorkflowExecutionHistoryResponse {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionHistoryResponse{
		History:       ToHistory(t.History),
		RawHistory:    ToDataBlobArray(t.RawHistory),
		NextPageToken: t.NextPageToken,
		Archived:      t.GetArchived(),
	}
}

// FromHeader converts internal Header type to thrift
func FromHeader(t *types.Header) *shared.Header {
	if t == nil {
		return nil
	}
	return &shared.Header{
		Fields: t.Fields,
	}
}

// ToHeader converts thrift Header type to internal
func ToHeader(t *shared.Header) *types.Header {
	if t == nil {
		return nil
	}
	return &types.Header{
		Fields: t.Fields,
	}
}

// FromHistory converts internal History type to thrift
func FromHistory(t *types.History) *shared.History {
	if t == nil {
		return nil
	}
	return &shared.History{
		Events: FromHistoryEventArray(t.Events),
	}
}

// ToHistory converts thrift History type to internal
func ToHistory(t *shared.History) *types.History {
	if t == nil {
		return nil
	}
	return &types.History{
		Events: ToHistoryEventArray(t.Events),
	}
}

// FromHistoryBranch converts internal HistoryBranch type to thrift
func FromHistoryBranch(t *types.HistoryBranch) *shared.HistoryBranch {
	if t == nil {
		return nil
	}
	return &shared.HistoryBranch{
		TreeID:    &t.TreeID,
		BranchID:  &t.BranchID,
		Ancestors: FromHistoryBranchRangeArray(t.Ancestors),
	}
}

// ToHistoryBranch converts thrift HistoryBranch type to internal
func ToHistoryBranch(t *shared.HistoryBranch) *types.HistoryBranch {
	if t == nil {
		return nil
	}
	return &types.HistoryBranch{
		TreeID:    *t.TreeID,
		BranchID:  *t.BranchID,
		Ancestors: ToHistoryBranchRangeArray(t.Ancestors),
	}
}

// FromHistoryBranchRange converts internal HistoryBranchRange type to thrift
func FromHistoryBranchRange(t *types.HistoryBranchRange) *shared.HistoryBranchRange {
	if t == nil {
		return nil
	}
	return &shared.HistoryBranchRange{
		BranchID:    &t.BranchID,
		BeginNodeID: &t.BeginNodeID,
		EndNodeID:   &t.EndNodeID,
	}
}

// ToHistoryBranchRange converts thrift HistoryBranchRange type to internal
func ToHistoryBranchRange(t *shared.HistoryBranchRange) *types.HistoryBranchRange {
	if t == nil {
		return nil
	}
	return &types.HistoryBranchRange{
		BranchID:    *t.BranchID,
		BeginNodeID: *t.BeginNodeID,
		EndNodeID:   *t.EndNodeID,
	}
}

// FromHistoryEvent converts internal HistoryEvent type to thrift
func FromHistoryEvent(t *types.HistoryEvent) *shared.HistoryEvent {
	if t == nil {
		return nil
	}
	return &shared.HistoryEvent{
		EventId:                                 &t.ID,
		Timestamp:                               t.Timestamp,
		EventType:                               FromEventType(t.EventType),
		Version:                                 &t.Version,
		TaskId:                                  &t.TaskID,
		WorkflowExecutionStartedEventAttributes: FromWorkflowExecutionStartedEventAttributes(t.WorkflowExecutionStartedEventAttributes),
		WorkflowExecutionCompletedEventAttributes:                      FromWorkflowExecutionCompletedEventAttributes(t.WorkflowExecutionCompletedEventAttributes),
		WorkflowExecutionFailedEventAttributes:                         FromWorkflowExecutionFailedEventAttributes(t.WorkflowExecutionFailedEventAttributes),
		WorkflowExecutionTimedOutEventAttributes:                       FromWorkflowExecutionTimedOutEventAttributes(t.WorkflowExecutionTimedOutEventAttributes),
		DecisionTaskScheduledEventAttributes:                           FromDecisionTaskScheduledEventAttributes(t.DecisionTaskScheduledEventAttributes),
		DecisionTaskStartedEventAttributes:                             FromDecisionTaskStartedEventAttributes(t.DecisionTaskStartedEventAttributes),
		DecisionTaskCompletedEventAttributes:                           FromDecisionTaskCompletedEventAttributes(t.DecisionTaskCompletedEventAttributes),
		DecisionTaskTimedOutEventAttributes:                            FromDecisionTaskTimedOutEventAttributes(t.DecisionTaskTimedOutEventAttributes),
		DecisionTaskFailedEventAttributes:                              FromDecisionTaskFailedEventAttributes(t.DecisionTaskFailedEventAttributes),
		ActivityTaskScheduledEventAttributes:                           FromActivityTaskScheduledEventAttributes(t.ActivityTaskScheduledEventAttributes),
		ActivityTaskStartedEventAttributes:                             FromActivityTaskStartedEventAttributes(t.ActivityTaskStartedEventAttributes),
		ActivityTaskCompletedEventAttributes:                           FromActivityTaskCompletedEventAttributes(t.ActivityTaskCompletedEventAttributes),
		ActivityTaskFailedEventAttributes:                              FromActivityTaskFailedEventAttributes(t.ActivityTaskFailedEventAttributes),
		ActivityTaskTimedOutEventAttributes:                            FromActivityTaskTimedOutEventAttributes(t.ActivityTaskTimedOutEventAttributes),
		TimerStartedEventAttributes:                                    FromTimerStartedEventAttributes(t.TimerStartedEventAttributes),
		TimerFiredEventAttributes:                                      FromTimerFiredEventAttributes(t.TimerFiredEventAttributes),
		ActivityTaskCancelRequestedEventAttributes:                     FromActivityTaskCancelRequestedEventAttributes(t.ActivityTaskCancelRequestedEventAttributes),
		RequestCancelActivityTaskFailedEventAttributes:                 FromRequestCancelActivityTaskFailedEventAttributes(t.RequestCancelActivityTaskFailedEventAttributes),
		ActivityTaskCanceledEventAttributes:                            FromActivityTaskCanceledEventAttributes(t.ActivityTaskCanceledEventAttributes),
		TimerCanceledEventAttributes:                                   FromTimerCanceledEventAttributes(t.TimerCanceledEventAttributes),
		CancelTimerFailedEventAttributes:                               FromCancelTimerFailedEventAttributes(t.CancelTimerFailedEventAttributes),
		MarkerRecordedEventAttributes:                                  FromMarkerRecordedEventAttributes(t.MarkerRecordedEventAttributes),
		WorkflowExecutionSignaledEventAttributes:                       FromWorkflowExecutionSignaledEventAttributes(t.WorkflowExecutionSignaledEventAttributes),
		WorkflowExecutionTerminatedEventAttributes:                     FromWorkflowExecutionTerminatedEventAttributes(t.WorkflowExecutionTerminatedEventAttributes),
		WorkflowExecutionCancelRequestedEventAttributes:                FromWorkflowExecutionCancelRequestedEventAttributes(t.WorkflowExecutionCancelRequestedEventAttributes),
		WorkflowExecutionCanceledEventAttributes:                       FromWorkflowExecutionCanceledEventAttributes(t.WorkflowExecutionCanceledEventAttributes),
		RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes),
		RequestCancelExternalWorkflowExecutionFailedEventAttributes:    FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(t.RequestCancelExternalWorkflowExecutionFailedEventAttributes),
		ExternalWorkflowExecutionCancelRequestedEventAttributes:        FromExternalWorkflowExecutionCancelRequestedEventAttributes(t.ExternalWorkflowExecutionCancelRequestedEventAttributes),
		WorkflowExecutionContinuedAsNewEventAttributes:                 FromWorkflowExecutionContinuedAsNewEventAttributes(t.WorkflowExecutionContinuedAsNewEventAttributes),
		StartChildWorkflowExecutionInitiatedEventAttributes:            FromStartChildWorkflowExecutionInitiatedEventAttributes(t.StartChildWorkflowExecutionInitiatedEventAttributes),
		StartChildWorkflowExecutionFailedEventAttributes:               FromStartChildWorkflowExecutionFailedEventAttributes(t.StartChildWorkflowExecutionFailedEventAttributes),
		ChildWorkflowExecutionStartedEventAttributes:                   FromChildWorkflowExecutionStartedEventAttributes(t.ChildWorkflowExecutionStartedEventAttributes),
		ChildWorkflowExecutionCompletedEventAttributes:                 FromChildWorkflowExecutionCompletedEventAttributes(t.ChildWorkflowExecutionCompletedEventAttributes),
		ChildWorkflowExecutionFailedEventAttributes:                    FromChildWorkflowExecutionFailedEventAttributes(t.ChildWorkflowExecutionFailedEventAttributes),
		ChildWorkflowExecutionCanceledEventAttributes:                  FromChildWorkflowExecutionCanceledEventAttributes(t.ChildWorkflowExecutionCanceledEventAttributes),
		ChildWorkflowExecutionTimedOutEventAttributes:                  FromChildWorkflowExecutionTimedOutEventAttributes(t.ChildWorkflowExecutionTimedOutEventAttributes),
		ChildWorkflowExecutionTerminatedEventAttributes:                FromChildWorkflowExecutionTerminatedEventAttributes(t.ChildWorkflowExecutionTerminatedEventAttributes),
		SignalExternalWorkflowExecutionInitiatedEventAttributes:        FromSignalExternalWorkflowExecutionInitiatedEventAttributes(t.SignalExternalWorkflowExecutionInitiatedEventAttributes),
		SignalExternalWorkflowExecutionFailedEventAttributes:           FromSignalExternalWorkflowExecutionFailedEventAttributes(t.SignalExternalWorkflowExecutionFailedEventAttributes),
		ExternalWorkflowExecutionSignaledEventAttributes:               FromExternalWorkflowExecutionSignaledEventAttributes(t.ExternalWorkflowExecutionSignaledEventAttributes),
		UpsertWorkflowSearchAttributesEventAttributes:                  FromUpsertWorkflowSearchAttributesEventAttributes(t.UpsertWorkflowSearchAttributesEventAttributes),
	}
}

// ToHistoryEvent converts thrift HistoryEvent type to internal
func ToHistoryEvent(t *shared.HistoryEvent) *types.HistoryEvent {
	if t == nil {
		return nil
	}
	return &types.HistoryEvent{
		ID:                                      t.GetEventId(),
		Timestamp:                               t.Timestamp,
		EventType:                               ToEventType(t.EventType),
		Version:                                 t.GetVersion(),
		TaskID:                                  t.GetTaskId(),
		WorkflowExecutionStartedEventAttributes: ToWorkflowExecutionStartedEventAttributes(t.WorkflowExecutionStartedEventAttributes),
		WorkflowExecutionCompletedEventAttributes:                      ToWorkflowExecutionCompletedEventAttributes(t.WorkflowExecutionCompletedEventAttributes),
		WorkflowExecutionFailedEventAttributes:                         ToWorkflowExecutionFailedEventAttributes(t.WorkflowExecutionFailedEventAttributes),
		WorkflowExecutionTimedOutEventAttributes:                       ToWorkflowExecutionTimedOutEventAttributes(t.WorkflowExecutionTimedOutEventAttributes),
		DecisionTaskScheduledEventAttributes:                           ToDecisionTaskScheduledEventAttributes(t.DecisionTaskScheduledEventAttributes),
		DecisionTaskStartedEventAttributes:                             ToDecisionTaskStartedEventAttributes(t.DecisionTaskStartedEventAttributes),
		DecisionTaskCompletedEventAttributes:                           ToDecisionTaskCompletedEventAttributes(t.DecisionTaskCompletedEventAttributes),
		DecisionTaskTimedOutEventAttributes:                            ToDecisionTaskTimedOutEventAttributes(t.DecisionTaskTimedOutEventAttributes),
		DecisionTaskFailedEventAttributes:                              ToDecisionTaskFailedEventAttributes(t.DecisionTaskFailedEventAttributes),
		ActivityTaskScheduledEventAttributes:                           ToActivityTaskScheduledEventAttributes(t.ActivityTaskScheduledEventAttributes),
		ActivityTaskStartedEventAttributes:                             ToActivityTaskStartedEventAttributes(t.ActivityTaskStartedEventAttributes),
		ActivityTaskCompletedEventAttributes:                           ToActivityTaskCompletedEventAttributes(t.ActivityTaskCompletedEventAttributes),
		ActivityTaskFailedEventAttributes:                              ToActivityTaskFailedEventAttributes(t.ActivityTaskFailedEventAttributes),
		ActivityTaskTimedOutEventAttributes:                            ToActivityTaskTimedOutEventAttributes(t.ActivityTaskTimedOutEventAttributes),
		TimerStartedEventAttributes:                                    ToTimerStartedEventAttributes(t.TimerStartedEventAttributes),
		TimerFiredEventAttributes:                                      ToTimerFiredEventAttributes(t.TimerFiredEventAttributes),
		ActivityTaskCancelRequestedEventAttributes:                     ToActivityTaskCancelRequestedEventAttributes(t.ActivityTaskCancelRequestedEventAttributes),
		RequestCancelActivityTaskFailedEventAttributes:                 ToRequestCancelActivityTaskFailedEventAttributes(t.RequestCancelActivityTaskFailedEventAttributes),
		ActivityTaskCanceledEventAttributes:                            ToActivityTaskCanceledEventAttributes(t.ActivityTaskCanceledEventAttributes),
		TimerCanceledEventAttributes:                                   ToTimerCanceledEventAttributes(t.TimerCanceledEventAttributes),
		CancelTimerFailedEventAttributes:                               ToCancelTimerFailedEventAttributes(t.CancelTimerFailedEventAttributes),
		MarkerRecordedEventAttributes:                                  ToMarkerRecordedEventAttributes(t.MarkerRecordedEventAttributes),
		WorkflowExecutionSignaledEventAttributes:                       ToWorkflowExecutionSignaledEventAttributes(t.WorkflowExecutionSignaledEventAttributes),
		WorkflowExecutionTerminatedEventAttributes:                     ToWorkflowExecutionTerminatedEventAttributes(t.WorkflowExecutionTerminatedEventAttributes),
		WorkflowExecutionCancelRequestedEventAttributes:                ToWorkflowExecutionCancelRequestedEventAttributes(t.WorkflowExecutionCancelRequestedEventAttributes),
		WorkflowExecutionCanceledEventAttributes:                       ToWorkflowExecutionCanceledEventAttributes(t.WorkflowExecutionCanceledEventAttributes),
		RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes),
		RequestCancelExternalWorkflowExecutionFailedEventAttributes:    ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(t.RequestCancelExternalWorkflowExecutionFailedEventAttributes),
		ExternalWorkflowExecutionCancelRequestedEventAttributes:        ToExternalWorkflowExecutionCancelRequestedEventAttributes(t.ExternalWorkflowExecutionCancelRequestedEventAttributes),
		WorkflowExecutionContinuedAsNewEventAttributes:                 ToWorkflowExecutionContinuedAsNewEventAttributes(t.WorkflowExecutionContinuedAsNewEventAttributes),
		StartChildWorkflowExecutionInitiatedEventAttributes:            ToStartChildWorkflowExecutionInitiatedEventAttributes(t.StartChildWorkflowExecutionInitiatedEventAttributes),
		StartChildWorkflowExecutionFailedEventAttributes:               ToStartChildWorkflowExecutionFailedEventAttributes(t.StartChildWorkflowExecutionFailedEventAttributes),
		ChildWorkflowExecutionStartedEventAttributes:                   ToChildWorkflowExecutionStartedEventAttributes(t.ChildWorkflowExecutionStartedEventAttributes),
		ChildWorkflowExecutionCompletedEventAttributes:                 ToChildWorkflowExecutionCompletedEventAttributes(t.ChildWorkflowExecutionCompletedEventAttributes),
		ChildWorkflowExecutionFailedEventAttributes:                    ToChildWorkflowExecutionFailedEventAttributes(t.ChildWorkflowExecutionFailedEventAttributes),
		ChildWorkflowExecutionCanceledEventAttributes:                  ToChildWorkflowExecutionCanceledEventAttributes(t.ChildWorkflowExecutionCanceledEventAttributes),
		ChildWorkflowExecutionTimedOutEventAttributes:                  ToChildWorkflowExecutionTimedOutEventAttributes(t.ChildWorkflowExecutionTimedOutEventAttributes),
		ChildWorkflowExecutionTerminatedEventAttributes:                ToChildWorkflowExecutionTerminatedEventAttributes(t.ChildWorkflowExecutionTerminatedEventAttributes),
		SignalExternalWorkflowExecutionInitiatedEventAttributes:        ToSignalExternalWorkflowExecutionInitiatedEventAttributes(t.SignalExternalWorkflowExecutionInitiatedEventAttributes),
		SignalExternalWorkflowExecutionFailedEventAttributes:           ToSignalExternalWorkflowExecutionFailedEventAttributes(t.SignalExternalWorkflowExecutionFailedEventAttributes),
		ExternalWorkflowExecutionSignaledEventAttributes:               ToExternalWorkflowExecutionSignaledEventAttributes(t.ExternalWorkflowExecutionSignaledEventAttributes),
		UpsertWorkflowSearchAttributesEventAttributes:                  ToUpsertWorkflowSearchAttributesEventAttributes(t.UpsertWorkflowSearchAttributesEventAttributes),
	}
}

// FromHistoryEventFilterType converts internal HistoryEventFilterType type to thrift
func FromHistoryEventFilterType(t *types.HistoryEventFilterType) *shared.HistoryEventFilterType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.HistoryEventFilterTypeAllEvent:
		v := shared.HistoryEventFilterTypeAllEvent
		return &v
	case types.HistoryEventFilterTypeCloseEvent:
		v := shared.HistoryEventFilterTypeCloseEvent
		return &v
	}
	panic("unexpected enum value")
}

// ToHistoryEventFilterType converts thrift HistoryEventFilterType type to internal
func ToHistoryEventFilterType(t *shared.HistoryEventFilterType) *types.HistoryEventFilterType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.HistoryEventFilterTypeAllEvent:
		v := types.HistoryEventFilterTypeAllEvent
		return &v
	case shared.HistoryEventFilterTypeCloseEvent:
		v := types.HistoryEventFilterTypeCloseEvent
		return &v
	}
	panic("unexpected enum value")
}

// FromIndexedValueType converts internal IndexedValueType type to thrift
func FromIndexedValueType(t types.IndexedValueType) shared.IndexedValueType {

	switch t {
	case types.IndexedValueTypeString:
		return shared.IndexedValueTypeString
	case types.IndexedValueTypeKeyword:
		return shared.IndexedValueTypeKeyword
	case types.IndexedValueTypeInt:
		return shared.IndexedValueTypeInt
	case types.IndexedValueTypeDouble:
		return shared.IndexedValueTypeDouble
	case types.IndexedValueTypeBool:
		return shared.IndexedValueTypeBool
	case types.IndexedValueTypeDatetime:
		return shared.IndexedValueTypeDatetime
	}
	panic("unexpected enum value")
}

// ToIndexedValueType converts thrift IndexedValueType type to internal
func ToIndexedValueType(t shared.IndexedValueType) types.IndexedValueType {

	switch t {
	case shared.IndexedValueTypeString:
		return types.IndexedValueTypeString
	case shared.IndexedValueTypeKeyword:
		return types.IndexedValueTypeKeyword
	case shared.IndexedValueTypeInt:
		return types.IndexedValueTypeInt
	case shared.IndexedValueTypeDouble:
		return types.IndexedValueTypeDouble
	case shared.IndexedValueTypeBool:
		return types.IndexedValueTypeBool
	case shared.IndexedValueTypeDatetime:
		return types.IndexedValueTypeDatetime
	}
	panic("unexpected enum value")
}

// FromInternalDataInconsistencyError converts internal InternalDataInconsistencyError type to thrift
func FromInternalDataInconsistencyError(t *types.InternalDataInconsistencyError) *shared.InternalDataInconsistencyError {
	if t == nil {
		return nil
	}
	return &shared.InternalDataInconsistencyError{
		Message: t.Message,
	}
}

// ToInternalDataInconsistencyError converts thrift InternalDataInconsistencyError type to internal
func ToInternalDataInconsistencyError(t *shared.InternalDataInconsistencyError) *types.InternalDataInconsistencyError {
	if t == nil {
		return nil
	}
	return &types.InternalDataInconsistencyError{
		Message: t.Message,
	}
}

// FromInternalServiceError converts internal InternalServiceError type to thrift
func FromInternalServiceError(t *types.InternalServiceError) *shared.InternalServiceError {
	if t == nil {
		return nil
	}
	return &shared.InternalServiceError{
		Message: t.Message,
	}
}

// ToInternalServiceError converts thrift InternalServiceError type to internal
func ToInternalServiceError(t *shared.InternalServiceError) *types.InternalServiceError {
	if t == nil {
		return nil
	}
	return &types.InternalServiceError{
		Message: t.Message,
	}
}

// FromLimitExceededError converts internal LimitExceededError type to thrift
func FromLimitExceededError(t *types.LimitExceededError) *shared.LimitExceededError {
	if t == nil {
		return nil
	}
	return &shared.LimitExceededError{
		Message: t.Message,
	}
}

// ToLimitExceededError converts thrift LimitExceededError type to internal
func ToLimitExceededError(t *shared.LimitExceededError) *types.LimitExceededError {
	if t == nil {
		return nil
	}
	return &types.LimitExceededError{
		Message: t.Message,
	}
}

// FromListArchivedWorkflowExecutionsRequest converts internal ListArchivedWorkflowExecutionsRequest type to thrift
func FromListArchivedWorkflowExecutionsRequest(t *types.ListArchivedWorkflowExecutionsRequest) *shared.ListArchivedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListArchivedWorkflowExecutionsRequest{
		Domain:        &t.Domain,
		PageSize:      &t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         &t.Query,
	}
}

// ToListArchivedWorkflowExecutionsRequest converts thrift ListArchivedWorkflowExecutionsRequest type to internal
func ToListArchivedWorkflowExecutionsRequest(t *shared.ListArchivedWorkflowExecutionsRequest) *types.ListArchivedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListArchivedWorkflowExecutionsRequest{
		Domain:        t.GetDomain(),
		PageSize:      t.GetPageSize(),
		NextPageToken: t.NextPageToken,
		Query:         t.GetQuery(),
	}
}

// FromListArchivedWorkflowExecutionsResponse converts internal ListArchivedWorkflowExecutionsResponse type to thrift
func FromListArchivedWorkflowExecutionsResponse(t *types.ListArchivedWorkflowExecutionsResponse) *shared.ListArchivedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListArchivedWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// ToListArchivedWorkflowExecutionsResponse converts thrift ListArchivedWorkflowExecutionsResponse type to internal
func ToListArchivedWorkflowExecutionsResponse(t *shared.ListArchivedWorkflowExecutionsResponse) *types.ListArchivedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListArchivedWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// FromListClosedWorkflowExecutionsRequest converts internal ListClosedWorkflowExecutionsRequest type to thrift
func FromListClosedWorkflowExecutionsRequest(t *types.ListClosedWorkflowExecutionsRequest) *shared.ListClosedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListClosedWorkflowExecutionsRequest{
		Domain:          &t.Domain,
		MaximumPageSize: &t.MaximumPageSize,
		NextPageToken:   t.NextPageToken,
		StartTimeFilter: FromStartTimeFilter(t.StartTimeFilter),
		ExecutionFilter: FromWorkflowExecutionFilter(t.ExecutionFilter),
		TypeFilter:      FromWorkflowTypeFilter(t.TypeFilter),
		StatusFilter:    FromWorkflowExecutionCloseStatus(t.StatusFilter),
	}
}

// ToListClosedWorkflowExecutionsRequest converts thrift ListClosedWorkflowExecutionsRequest type to internal
func ToListClosedWorkflowExecutionsRequest(t *shared.ListClosedWorkflowExecutionsRequest) *types.ListClosedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListClosedWorkflowExecutionsRequest{
		Domain:          t.GetDomain(),
		MaximumPageSize: t.GetMaximumPageSize(),
		NextPageToken:   t.NextPageToken,
		StartTimeFilter: ToStartTimeFilter(t.StartTimeFilter),
		ExecutionFilter: ToWorkflowExecutionFilter(t.ExecutionFilter),
		TypeFilter:      ToWorkflowTypeFilter(t.TypeFilter),
		StatusFilter:    ToWorkflowExecutionCloseStatus(t.StatusFilter),
	}
}

// FromListClosedWorkflowExecutionsResponse converts internal ListClosedWorkflowExecutionsResponse type to thrift
func FromListClosedWorkflowExecutionsResponse(t *types.ListClosedWorkflowExecutionsResponse) *shared.ListClosedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListClosedWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// ToListClosedWorkflowExecutionsResponse converts thrift ListClosedWorkflowExecutionsResponse type to internal
func ToListClosedWorkflowExecutionsResponse(t *shared.ListClosedWorkflowExecutionsResponse) *types.ListClosedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListClosedWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// FromListDomainsRequest converts internal ListDomainsRequest type to thrift
func FromListDomainsRequest(t *types.ListDomainsRequest) *shared.ListDomainsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListDomainsRequest{
		PageSize:      &t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

// ToListDomainsRequest converts thrift ListDomainsRequest type to internal
func ToListDomainsRequest(t *shared.ListDomainsRequest) *types.ListDomainsRequest {
	if t == nil {
		return nil
	}
	return &types.ListDomainsRequest{
		PageSize:      t.GetPageSize(),
		NextPageToken: t.NextPageToken,
	}
}

// FromListDomainsResponse converts internal ListDomainsResponse type to thrift
func FromListDomainsResponse(t *types.ListDomainsResponse) *shared.ListDomainsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListDomainsResponse{
		Domains:       FromDescribeDomainResponseArray(t.Domains),
		NextPageToken: t.NextPageToken,
	}
}

// ToListDomainsResponse converts thrift ListDomainsResponse type to internal
func ToListDomainsResponse(t *shared.ListDomainsResponse) *types.ListDomainsResponse {
	if t == nil {
		return nil
	}
	return &types.ListDomainsResponse{
		Domains:       ToDescribeDomainResponseArray(t.Domains),
		NextPageToken: t.NextPageToken,
	}
}

// ToListFailoverHistoryRequest converts thrift ListFailoverHistoryRequest type to internal
func ToListFailoverHistoryRequest(t *shared.ListFailoverHistoryRequest) *types.ListFailoverHistoryRequest {
	if t == nil {
		return nil
	}

	return &types.ListFailoverHistoryRequest{
		Filters:    ToListFailoverHistoryRequestFilters(t.Filters),
		Pagination: ToPaginationOptions(t.Pagination),
	}
}

// FromListFailoverHistoryRequest converts internal ListFailoverHistoryRequest type to thrift
func FromListFailoverHistoryRequest(t *types.ListFailoverHistoryRequest) *shared.ListFailoverHistoryRequest {
	if t == nil {
		return nil
	}

	return &shared.ListFailoverHistoryRequest{
		Filters:    FromListFailoverHistoryRequestFilters(t.Filters),
		Pagination: FromPaginationOptions(t.Pagination),
	}
}

// FromListFailoverHistoryResponse converts internal ListFailoverHistoryResponse type to thrift
func FromListFailoverHistoryResponse(t *types.ListFailoverHistoryResponse) *shared.ListFailoverHistoryResponse {
	if t == nil {
		return nil
	}
	return &shared.ListFailoverHistoryResponse{
		FailoverEvents: FromFailoverEventArray(t.FailoverEvents),
		NextPageToken:  t.NextPageToken,
	}
}

// ToListFailoverHistoryResponse converts thrift ListFailoverHistoryResponse type to internal
func ToListFailoverHistoryResponse(t *shared.ListFailoverHistoryResponse) *types.ListFailoverHistoryResponse {
	if t == nil {
		return nil
	}
	return &types.ListFailoverHistoryResponse{
		FailoverEvents: ToFailoverEventArray(t.FailoverEvents),
		NextPageToken:  t.NextPageToken,
	}
}

// ToListFailoverHistoryRequestFilters converts thrift ListFailoverHistoryRequestFilters type to internal
func ToListFailoverHistoryRequestFilters(t *shared.ListFailoverHistoryRequestFilters) *types.ListFailoverHistoryRequestFilters {
	if t == nil {
		return nil
	}
	return &types.ListFailoverHistoryRequestFilters{
		DomainID: t.GetDomainID(),
	}
}

// FromListFailoverHistoryRequestFilters converts internal ListFailoverHistoryRequestFilters type to thrift
func FromListFailoverHistoryRequestFilters(t *types.ListFailoverHistoryRequestFilters) *shared.ListFailoverHistoryRequestFilters {
	if t == nil {
		return nil
	}
	return &shared.ListFailoverHistoryRequestFilters{
		DomainID: &t.DomainID,
	}
}

// ToPaginationOptions converts thrift PaginationOptions type to internal
func ToPaginationOptions(t *shared.PaginationOptions) *types.PaginationOptions {
	if t == nil {
		return nil
	}
	return &types.PaginationOptions{
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

// FromPaginationOptions converts internal PaginationOptions type to thrift
func FromPaginationOptions(t *types.PaginationOptions) *shared.PaginationOptions {
	if t == nil {
		return nil
	}
	return &shared.PaginationOptions{
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

// ToFailoverEvent converts thrift FailoverEvent type to internal
func ToFailoverEvent(t *shared.FailoverEvent) *types.FailoverEvent {
	if t == nil {
		return nil
	}
	return &types.FailoverEvent{
		ID:               t.ID,
		CreatedTime:      t.CreatedTime,
		FailoverType:     ToFailoverType(t.FailoverType),
		ClusterFailovers: ToClusterFailoverArray(t.ClusterFailovers),
	}
}

// FromFailoverEvent converts internal FailoverEvent type to thrift
func FromFailoverEvent(t *types.FailoverEvent) *shared.FailoverEvent {
	if t == nil {
		return nil
	}
	return &shared.FailoverEvent{
		ID:               t.ID,
		CreatedTime:      t.CreatedTime,
		FailoverType:     FromFailoverType(t.FailoverType),
		ClusterFailovers: FromClusterFailoverArray(t.ClusterFailovers),
	}
}

// ToFailoverEventArray converts thrift FailoverEvent array type to internal
func ToFailoverEventArray(t []*shared.FailoverEvent) []*types.FailoverEvent {
	if t == nil {
		return nil
	}
	v := make([]*types.FailoverEvent, len(t))
	for i := range t {
		v[i] = ToFailoverEvent(t[i])
	}
	return v
}

// FromFailoverEventArray converts internal FailoverEvent array type to thrift
func FromFailoverEventArray(t []*types.FailoverEvent) []*shared.FailoverEvent {
	if t == nil {
		return nil
	}
	v := make([]*shared.FailoverEvent, len(t))
	for i := range t {
		v[i] = FromFailoverEvent(t[i])
	}
	return v
}

// ToFailoverType converts thrift FailoverType type to internal
func ToFailoverType(t *shared.FailoverType) *types.FailoverType {
	if t == nil {
		return nil
	}
	v := types.FailoverType(*t)
	return &v
}

// FromFailoverType converts internal FailoverType type to thrift
func FromFailoverType(t *types.FailoverType) *shared.FailoverType {
	if t == nil {
		return nil
	}
	v := shared.FailoverType(*t)
	return &v
}

// ToClusterFailover converts thrift ClusterFailover type to internal
func ToClusterFailover(t *shared.ClusterFailover) *types.ClusterFailover {
	if t == nil {
		return nil
	}
	return &types.ClusterFailover{
		FromCluster:      ToActiveClusterInfo(t.FromCluster),
		ToCluster:        ToActiveClusterInfo(t.ToCluster),
		ClusterAttribute: ToClusterAttribute(t.ClusterAttribute),
	}
}

// FromClusterFailover converts internal ClusterFailover type to thrift
func FromClusterFailover(t *types.ClusterFailover) *shared.ClusterFailover {
	if t == nil {
		return nil
	}
	return &shared.ClusterFailover{
		FromCluster:      FromActiveClusterInfo(t.FromCluster),
		ToCluster:        FromActiveClusterInfo(t.ToCluster),
		ClusterAttribute: FromClusterAttribute(t.ClusterAttribute),
	}
}

// ToClusterFailoverArray converts thrift ClusterFailover array type to internal
func ToClusterFailoverArray(t []*shared.ClusterFailover) []*types.ClusterFailover {
	if t == nil {
		return nil
	}
	v := make([]*types.ClusterFailover, len(t))
	for i := range t {
		v[i] = ToClusterFailover(t[i])
	}
	return v
}

// FromClusterFailoverArray converts internal ClusterFailover array type to thrift
func FromClusterFailoverArray(t []*types.ClusterFailover) []*shared.ClusterFailover {
	if t == nil {
		return nil
	}
	v := make([]*shared.ClusterFailover, len(t))
	for i := range t {
		v[i] = FromClusterFailover(t[i])
	}
	return v
}

// ToActiveClusterInfo converts thrift ActiveClusterInfo type to internal
func ToActiveClusterInfo(t *shared.ActiveClusterInfo) *types.ActiveClusterInfo {
	if t == nil {
		return nil
	}
	return &types.ActiveClusterInfo{
		ActiveClusterName: t.GetActiveClusterName(),
		FailoverVersion:   t.GetFailoverVersion(),
	}
}

// FromActiveClusterInfo converts internal ActiveClusterInfo type to thrift
func FromActiveClusterInfo(t *types.ActiveClusterInfo) *shared.ActiveClusterInfo {
	if t == nil {
		return nil
	}
	return &shared.ActiveClusterInfo{
		ActiveClusterName: &t.ActiveClusterName,
		FailoverVersion:   &t.FailoverVersion,
	}
}

// FromListOpenWorkflowExecutionsRequest converts internal ListOpenWorkflowExecutionsRequest type to thrift
func FromListOpenWorkflowExecutionsRequest(t *types.ListOpenWorkflowExecutionsRequest) *shared.ListOpenWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListOpenWorkflowExecutionsRequest{
		Domain:          &t.Domain,
		MaximumPageSize: &t.MaximumPageSize,
		NextPageToken:   t.NextPageToken,
		StartTimeFilter: FromStartTimeFilter(t.StartTimeFilter),
		ExecutionFilter: FromWorkflowExecutionFilter(t.ExecutionFilter),
		TypeFilter:      FromWorkflowTypeFilter(t.TypeFilter),
	}
}

// ToListOpenWorkflowExecutionsRequest converts thrift ListOpenWorkflowExecutionsRequest type to internal
func ToListOpenWorkflowExecutionsRequest(t *shared.ListOpenWorkflowExecutionsRequest) *types.ListOpenWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListOpenWorkflowExecutionsRequest{
		Domain:          t.GetDomain(),
		MaximumPageSize: t.GetMaximumPageSize(),
		NextPageToken:   t.NextPageToken,
		StartTimeFilter: ToStartTimeFilter(t.StartTimeFilter),
		ExecutionFilter: ToWorkflowExecutionFilter(t.ExecutionFilter),
		TypeFilter:      ToWorkflowTypeFilter(t.TypeFilter),
	}
}

// FromListOpenWorkflowExecutionsResponse converts internal ListOpenWorkflowExecutionsResponse type to thrift
func FromListOpenWorkflowExecutionsResponse(t *types.ListOpenWorkflowExecutionsResponse) *shared.ListOpenWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListOpenWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// ToListOpenWorkflowExecutionsResponse converts thrift ListOpenWorkflowExecutionsResponse type to internal
func ToListOpenWorkflowExecutionsResponse(t *shared.ListOpenWorkflowExecutionsResponse) *types.ListOpenWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListOpenWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// FromListTaskListPartitionsRequest converts internal ListTaskListPartitionsRequest type to thrift
func FromListTaskListPartitionsRequest(t *types.ListTaskListPartitionsRequest) *shared.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListTaskListPartitionsRequest{
		Domain:   &t.Domain,
		TaskList: FromTaskList(t.TaskList),
	}
}

// ToListTaskListPartitionsRequest converts thrift ListTaskListPartitionsRequest type to internal
func ToListTaskListPartitionsRequest(t *shared.ListTaskListPartitionsRequest) *types.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListTaskListPartitionsRequest{
		Domain:   t.GetDomain(),
		TaskList: ToTaskList(t.TaskList),
	}
}

// FromListTaskListPartitionsResponse converts internal ListTaskListPartitionsResponse type to thrift
func FromListTaskListPartitionsResponse(t *types.ListTaskListPartitionsResponse) *shared.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: FromTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: FromTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

// ToListTaskListPartitionsResponse converts thrift ListTaskListPartitionsResponse type to internal
func ToListTaskListPartitionsResponse(t *shared.ListTaskListPartitionsResponse) *types.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: ToTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: ToTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

// FromGetTaskListsByDomainRequest converts internal GetTaskListsByDomainRequest type to thrift
func FromGetTaskListsByDomainRequest(t *types.GetTaskListsByDomainRequest) *shared.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.GetTaskListsByDomainRequest{
		DomainName: &t.Domain,
	}
}

// ToGetTaskListsByDomainRequest converts thrift GetTaskListsByDomainRequest type to internal
func ToGetTaskListsByDomainRequest(t *shared.GetTaskListsByDomainRequest) *types.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &types.GetTaskListsByDomainRequest{
		Domain: t.GetDomainName(),
	}
}

// FromGetTaskListsByDomainResponse converts internal GetTaskListsByDomainResponse type to thrift
func FromGetTaskListsByDomainResponse(t *types.GetTaskListsByDomainResponse) *shared.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &shared.GetTaskListsByDomainResponse{
		DecisionTaskListMap: FromDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: FromDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

// ToGetTaskListsByDomainResponse converts thrift GetTaskListsByDomainResponse type to internal
func ToGetTaskListsByDomainResponse(t *shared.GetTaskListsByDomainResponse) *types.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: ToDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: ToDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

// FromDescribeTaskListResponseMap converts internal DescribeTaskListResponse map type to thrift
func FromDescribeTaskListResponseMap(t map[string]*types.DescribeTaskListResponse) map[string]*shared.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*shared.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = FromDescribeTaskListResponse(value)
	}
	return taskListMap
}

// ToDescribeTaskListResponseMap converts thrift DescribeTaskListResponse map type to internal
func ToDescribeTaskListResponseMap(t map[string]*shared.DescribeTaskListResponse) map[string]*types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*types.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = ToDescribeTaskListResponse(value)
	}
	return taskListMap
}

// FromListWorkflowExecutionsRequest converts internal ListWorkflowExecutionsRequest type to thrift
func FromListWorkflowExecutionsRequest(t *types.ListWorkflowExecutionsRequest) *shared.ListWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &shared.ListWorkflowExecutionsRequest{
		Domain:        &t.Domain,
		PageSize:      &t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         &t.Query,
	}
}

// ToListWorkflowExecutionsRequest converts thrift ListWorkflowExecutionsRequest type to internal
func ToListWorkflowExecutionsRequest(t *shared.ListWorkflowExecutionsRequest) *types.ListWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsRequest{
		Domain:        t.GetDomain(),
		PageSize:      t.GetPageSize(),
		NextPageToken: t.NextPageToken,
		Query:         t.GetQuery(),
	}
}

// FromListWorkflowExecutionsResponse converts internal ListWorkflowExecutionsResponse type to thrift
func FromListWorkflowExecutionsResponse(t *types.ListWorkflowExecutionsResponse) *shared.ListWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &shared.ListWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// ToListWorkflowExecutionsResponse converts thrift ListWorkflowExecutionsResponse type to internal
func ToListWorkflowExecutionsResponse(t *shared.ListWorkflowExecutionsResponse) *types.ListWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

// FromMarkerRecordedEventAttributes converts internal MarkerRecordedEventAttributes type to thrift
func FromMarkerRecordedEventAttributes(t *types.MarkerRecordedEventAttributes) *shared.MarkerRecordedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.MarkerRecordedEventAttributes{
		MarkerName:                   &t.MarkerName,
		Details:                      t.Details,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Header:                       FromHeader(t.Header),
	}
}

// ToMarkerRecordedEventAttributes converts thrift MarkerRecordedEventAttributes type to internal
func ToMarkerRecordedEventAttributes(t *shared.MarkerRecordedEventAttributes) *types.MarkerRecordedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.MarkerRecordedEventAttributes{
		MarkerName:                   t.GetMarkerName(),
		Details:                      t.Details,
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Header:                       ToHeader(t.Header),
	}
}

// FromMemo converts internal Memo type to thrift
func FromMemo(t *types.Memo) *shared.Memo {
	if t == nil {
		return nil
	}
	return &shared.Memo{
		Fields: t.Fields,
	}
}

// ToMemo converts thrift Memo type to internal
func ToMemo(t *shared.Memo) *types.Memo {
	if t == nil {
		return nil
	}
	return &types.Memo{
		Fields: t.Fields,
	}
}

// FromParentClosePolicy converts internal ParentClosePolicy type to thrift
func FromParentClosePolicy(t *types.ParentClosePolicy) *shared.ParentClosePolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case types.ParentClosePolicyAbandon:
		v := shared.ParentClosePolicyAbandon
		return &v
	case types.ParentClosePolicyRequestCancel:
		v := shared.ParentClosePolicyRequestCancel
		return &v
	case types.ParentClosePolicyTerminate:
		v := shared.ParentClosePolicyTerminate
		return &v
	}
	panic("unexpected enum value")
}

// ToParentClosePolicy converts thrift ParentClosePolicy type to internal
func ToParentClosePolicy(t *shared.ParentClosePolicy) *types.ParentClosePolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.ParentClosePolicyAbandon:
		v := types.ParentClosePolicyAbandon
		return &v
	case shared.ParentClosePolicyRequestCancel:
		v := types.ParentClosePolicyRequestCancel
		return &v
	case shared.ParentClosePolicyTerminate:
		v := types.ParentClosePolicyTerminate
		return &v
	}
	panic("unexpected enum value")
}

// FromPendingActivityInfo converts internal PendingActivityInfo type to thrift
func FromPendingActivityInfo(t *types.PendingActivityInfo) *shared.PendingActivityInfo {
	if t == nil {
		return nil
	}
	return &shared.PendingActivityInfo{
		ActivityID:             &t.ActivityID,
		ActivityType:           FromActivityType(t.ActivityType),
		State:                  FromPendingActivityState(t.State),
		HeartbeatDetails:       t.HeartbeatDetails,
		LastHeartbeatTimestamp: t.LastHeartbeatTimestamp,
		LastStartedTimestamp:   t.LastStartedTimestamp,
		Attempt:                &t.Attempt,
		MaximumAttempts:        &t.MaximumAttempts,
		ScheduledTimestamp:     t.ScheduledTimestamp,
		ExpirationTimestamp:    t.ExpirationTimestamp,
		LastFailureReason:      t.LastFailureReason,
		LastWorkerIdentity:     &t.LastWorkerIdentity,
		LastFailureDetails:     t.LastFailureDetails,
		StartedWorkerIdentity:  &t.StartedWorkerIdentity,
		ScheduleID:             &t.ScheduleID,
	}
}

// ToPendingActivityInfo converts thrift PendingActivityInfo type to internal
func ToPendingActivityInfo(t *shared.PendingActivityInfo) *types.PendingActivityInfo {
	if t == nil {
		return nil
	}
	return &types.PendingActivityInfo{
		ActivityID:             t.GetActivityID(),
		ActivityType:           ToActivityType(t.ActivityType),
		State:                  ToPendingActivityState(t.State),
		HeartbeatDetails:       t.HeartbeatDetails,
		LastHeartbeatTimestamp: t.LastHeartbeatTimestamp,
		LastStartedTimestamp:   t.LastStartedTimestamp,
		Attempt:                t.GetAttempt(),
		MaximumAttempts:        t.GetMaximumAttempts(),
		ScheduledTimestamp:     t.ScheduledTimestamp,
		ExpirationTimestamp:    t.ExpirationTimestamp,
		LastFailureReason:      t.LastFailureReason,
		LastWorkerIdentity:     t.GetLastWorkerIdentity(),
		LastFailureDetails:     t.LastFailureDetails,
		StartedWorkerIdentity:  t.GetStartedWorkerIdentity(),
		ScheduleID:             t.GetScheduleID(),
	}
}

// FromPendingActivityState converts internal PendingActivityState type to thrift
func FromPendingActivityState(t *types.PendingActivityState) *shared.PendingActivityState {
	if t == nil {
		return nil
	}
	switch *t {
	case types.PendingActivityStateScheduled:
		v := shared.PendingActivityStateScheduled
		return &v
	case types.PendingActivityStateStarted:
		v := shared.PendingActivityStateStarted
		return &v
	case types.PendingActivityStateCancelRequested:
		v := shared.PendingActivityStateCancelRequested
		return &v
	}
	panic("unexpected enum value")
}

// ToPendingActivityState converts thrift PendingActivityState type to internal
func ToPendingActivityState(t *shared.PendingActivityState) *types.PendingActivityState {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.PendingActivityStateScheduled:
		v := types.PendingActivityStateScheduled
		return &v
	case shared.PendingActivityStateStarted:
		v := types.PendingActivityStateStarted
		return &v
	case shared.PendingActivityStateCancelRequested:
		v := types.PendingActivityStateCancelRequested
		return &v
	}
	panic("unexpected enum value")
}

// FromPendingChildExecutionInfo converts internal PendingChildExecutionInfo type to thrift
func FromPendingChildExecutionInfo(t *types.PendingChildExecutionInfo) *shared.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	return &shared.PendingChildExecutionInfo{
		Domain:            &t.Domain,
		WorkflowID:        &t.WorkflowID,
		RunID:             &t.RunID,
		WorkflowTypName:   &t.WorkflowTypeName,
		InitiatedID:       &t.InitiatedID,
		ParentClosePolicy: FromParentClosePolicy(t.ParentClosePolicy),
	}
}

// ToPendingChildExecutionInfo converts thrift PendingChildExecutionInfo type to internal
func ToPendingChildExecutionInfo(t *shared.PendingChildExecutionInfo) *types.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	return &types.PendingChildExecutionInfo{
		Domain:            t.GetDomain(),
		WorkflowID:        t.GetWorkflowID(),
		RunID:             t.GetRunID(),
		WorkflowTypeName:  t.GetWorkflowTypName(),
		InitiatedID:       t.GetInitiatedID(),
		ParentClosePolicy: ToParentClosePolicy(t.ParentClosePolicy),
	}
}

// FromPendingDecisionInfo converts internal PendingDecisionInfo type to thrift
func FromPendingDecisionInfo(t *types.PendingDecisionInfo) *shared.PendingDecisionInfo {
	if t == nil {
		return nil
	}
	return &shared.PendingDecisionInfo{
		State:                      FromPendingDecisionState(t.State),
		ScheduledTimestamp:         t.ScheduledTimestamp,
		StartedTimestamp:           t.StartedTimestamp,
		Attempt:                    &t.Attempt,
		OriginalScheduledTimestamp: t.OriginalScheduledTimestamp,
		ScheduleID:                 &t.ScheduleID,
	}
}

// ToPendingDecisionInfo converts thrift PendingDecisionInfo type to internal
func ToPendingDecisionInfo(t *shared.PendingDecisionInfo) *types.PendingDecisionInfo {
	if t == nil {
		return nil
	}
	return &types.PendingDecisionInfo{
		State:                      ToPendingDecisionState(t.State),
		ScheduledTimestamp:         t.ScheduledTimestamp,
		StartedTimestamp:           t.StartedTimestamp,
		Attempt:                    t.GetAttempt(),
		OriginalScheduledTimestamp: t.OriginalScheduledTimestamp,
		ScheduleID:                 t.GetScheduleID(),
	}
}

// FromPendingDecisionState converts internal PendingDecisionState type to thrift
func FromPendingDecisionState(t *types.PendingDecisionState) *shared.PendingDecisionState {
	if t == nil {
		return nil
	}
	switch *t {
	case types.PendingDecisionStateScheduled:
		v := shared.PendingDecisionStateScheduled
		return &v
	case types.PendingDecisionStateStarted:
		v := shared.PendingDecisionStateStarted
		return &v
	}
	panic("unexpected enum value")
}

// ToPendingDecisionState converts thrift PendingDecisionState type to internal
func ToPendingDecisionState(t *shared.PendingDecisionState) *types.PendingDecisionState {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.PendingDecisionStateScheduled:
		v := types.PendingDecisionStateScheduled
		return &v
	case shared.PendingDecisionStateStarted:
		v := types.PendingDecisionStateStarted
		return &v
	}
	panic("unexpected enum value")
}

// FromPollForActivityTaskRequest converts internal PollForActivityTaskRequest type to thrift
func FromPollForActivityTaskRequest(t *types.PollForActivityTaskRequest) *shared.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &shared.PollForActivityTaskRequest{
		Domain:           &t.Domain,
		TaskList:         FromTaskList(t.TaskList),
		Identity:         &t.Identity,
		TaskListMetadata: FromTaskListMetadata(t.TaskListMetadata),
	}
}

// ToPollForActivityTaskRequest converts thrift PollForActivityTaskRequest type to internal
func ToPollForActivityTaskRequest(t *shared.PollForActivityTaskRequest) *types.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.PollForActivityTaskRequest{
		Domain:           t.GetDomain(),
		TaskList:         ToTaskList(t.TaskList),
		Identity:         t.GetIdentity(),
		TaskListMetadata: ToTaskListMetadata(t.TaskListMetadata),
	}
}

// FromPollForActivityTaskResponse converts internal PollForActivityTaskResponse type to thrift
func FromPollForActivityTaskResponse(t *types.PollForActivityTaskResponse) *shared.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &shared.PollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               FromWorkflowExecution(t.WorkflowExecution),
		ActivityId:                      &t.ActivityID,
		ActivityType:                    FromActivityType(t.ActivityType),
		Input:                           t.Input,
		ScheduledTimestamp:              t.ScheduledTimestamp,
		ScheduleToCloseTimeoutSeconds:   t.ScheduleToCloseTimeoutSeconds,
		StartedTimestamp:                t.StartedTimestamp,
		StartToCloseTimeoutSeconds:      t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:         t.HeartbeatTimeoutSeconds,
		Attempt:                         &t.Attempt,
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    FromWorkflowType(t.WorkflowType),
		WorkflowDomain:                  &t.WorkflowDomain,
		Header:                          FromHeader(t.Header),
		AutoConfigHint:                  FromAutoConfigHint(t.AutoConfigHint),
	}
}

// ToPollForActivityTaskResponse converts thrift PollForActivityTaskResponse type to internal
func ToPollForActivityTaskResponse(t *shared.PollForActivityTaskResponse) *types.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &types.PollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               ToWorkflowExecution(t.WorkflowExecution),
		ActivityID:                      t.GetActivityId(),
		ActivityType:                    ToActivityType(t.ActivityType),
		Input:                           t.Input,
		ScheduledTimestamp:              t.ScheduledTimestamp,
		ScheduleToCloseTimeoutSeconds:   t.ScheduleToCloseTimeoutSeconds,
		StartedTimestamp:                t.StartedTimestamp,
		StartToCloseTimeoutSeconds:      t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:         t.HeartbeatTimeoutSeconds,
		Attempt:                         t.GetAttempt(),
		ScheduledTimestampOfThisAttempt: t.ScheduledTimestampOfThisAttempt,
		HeartbeatDetails:                t.HeartbeatDetails,
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.GetWorkflowDomain(),
		Header:                          ToHeader(t.Header),
		AutoConfigHint:                  ToAutoConfigHint(t.AutoConfigHint),
	}
}

// FromPollForDecisionTaskRequest converts internal PollForDecisionTaskRequest type to thrift
func FromPollForDecisionTaskRequest(t *types.PollForDecisionTaskRequest) *shared.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &shared.PollForDecisionTaskRequest{
		Domain:         &t.Domain,
		TaskList:       FromTaskList(t.TaskList),
		Identity:       &t.Identity,
		BinaryChecksum: &t.BinaryChecksum,
	}
}

// ToPollForDecisionTaskRequest converts thrift PollForDecisionTaskRequest type to internal
func ToPollForDecisionTaskRequest(t *shared.PollForDecisionTaskRequest) *types.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.PollForDecisionTaskRequest{
		Domain:         t.GetDomain(),
		TaskList:       ToTaskList(t.TaskList),
		Identity:       t.GetIdentity(),
		BinaryChecksum: t.GetBinaryChecksum(),
	}
}

// FromPollForDecisionTaskResponse converts internal PollForDecisionTaskResponse type to thrift
func FromPollForDecisionTaskResponse(t *types.PollForDecisionTaskResponse) *shared.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &shared.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              FromWorkflowType(t.WorkflowType),
		PreviousStartedEventId:    t.PreviousStartedEventID,
		StartedEventId:            &t.StartedEventID,
		Attempt:                   &t.Attempt,
		BacklogCountHint:          &t.BacklogCountHint,
		History:                   FromHistory(t.History),
		NextPageToken:             t.NextPageToken,
		Query:                     FromWorkflowQuery(t.Query),
		WorkflowExecutionTaskList: FromTaskList(t.WorkflowExecutionTaskList),
		ScheduledTimestamp:        t.ScheduledTimestamp,
		StartedTimestamp:          t.StartedTimestamp,
		Queries:                   FromWorkflowQueryMap(t.Queries),
		NextEventId:               &t.NextEventID,
		TotalHistoryBytes:         &t.TotalHistoryBytes,
		AutoConfigHint:            FromAutoConfigHint(t.AutoConfigHint),
	}
}

// ToPollForDecisionTaskResponse converts thrift PollForDecisionTaskResponse type to internal
func ToPollForDecisionTaskResponse(t *shared.PollForDecisionTaskResponse) *types.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &types.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              ToWorkflowType(t.WorkflowType),
		PreviousStartedEventID:    t.PreviousStartedEventId,
		StartedEventID:            t.GetStartedEventId(),
		Attempt:                   t.GetAttempt(),
		BacklogCountHint:          t.GetBacklogCountHint(),
		History:                   ToHistory(t.History),
		NextPageToken:             t.NextPageToken,
		Query:                     ToWorkflowQuery(t.Query),
		WorkflowExecutionTaskList: ToTaskList(t.WorkflowExecutionTaskList),
		ScheduledTimestamp:        t.ScheduledTimestamp,
		StartedTimestamp:          t.StartedTimestamp,
		Queries:                   ToWorkflowQueryMap(t.Queries),
		NextEventID:               t.GetNextEventId(),
		TotalHistoryBytes:         t.GetTotalHistoryBytes(),
		AutoConfigHint:            ToAutoConfigHint(t.AutoConfigHint),
	}
}

// FromPollerInfo converts internal PollerInfo type to thrift
func FromPollerInfo(t *types.PollerInfo) *shared.PollerInfo {
	if t == nil {
		return nil
	}
	return &shared.PollerInfo{
		LastAccessTime: t.LastAccessTime,
		Identity:       &t.Identity,
		RatePerSecond:  &t.RatePerSecond,
	}
}

// ToPollerInfo converts thrift PollerInfo type to internal
func ToPollerInfo(t *shared.PollerInfo) *types.PollerInfo {
	if t == nil {
		return nil
	}
	return &types.PollerInfo{
		LastAccessTime: t.LastAccessTime,
		Identity:       t.GetIdentity(),
		RatePerSecond:  t.GetRatePerSecond(),
	}
}

// FromQueryConsistencyLevel converts internal QueryConsistencyLevel type to thrift
func FromQueryConsistencyLevel(t *types.QueryConsistencyLevel) *shared.QueryConsistencyLevel {
	if t == nil {
		return nil
	}
	switch *t {
	case types.QueryConsistencyLevelEventual:
		v := shared.QueryConsistencyLevelEventual
		return &v
	case types.QueryConsistencyLevelStrong:
		v := shared.QueryConsistencyLevelStrong
		return &v
	}
	panic("unexpected enum value")
}

// ToQueryConsistencyLevel converts thrift QueryConsistencyLevel type to internal
func ToQueryConsistencyLevel(t *shared.QueryConsistencyLevel) *types.QueryConsistencyLevel {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.QueryConsistencyLevelEventual:
		v := types.QueryConsistencyLevelEventual
		return &v
	case shared.QueryConsistencyLevelStrong:
		v := types.QueryConsistencyLevelStrong
		return &v
	}
	panic("unexpected enum value")
}

// FromQueryFailedError converts internal QueryFailedError type to thrift
func FromQueryFailedError(t *types.QueryFailedError) *shared.QueryFailedError {
	if t == nil {
		return nil
	}
	return &shared.QueryFailedError{
		Message: t.Message,
	}
}

// ToQueryFailedError converts thrift QueryFailedError type to internal
func ToQueryFailedError(t *shared.QueryFailedError) *types.QueryFailedError {
	if t == nil {
		return nil
	}
	return &types.QueryFailedError{
		Message: t.Message,
	}
}

// FromQueryRejectCondition converts internal QueryRejectCondition type to thrift
func FromQueryRejectCondition(t *types.QueryRejectCondition) *shared.QueryRejectCondition {
	if t == nil {
		return nil
	}
	switch *t {
	case types.QueryRejectConditionNotOpen:
		v := shared.QueryRejectConditionNotOpen
		return &v
	case types.QueryRejectConditionNotCompletedCleanly:
		v := shared.QueryRejectConditionNotCompletedCleanly
		return &v
	}
	panic("unexpected enum value")
}

// ToQueryRejectCondition converts thrift QueryRejectCondition type to internal
func ToQueryRejectCondition(t *shared.QueryRejectCondition) *types.QueryRejectCondition {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.QueryRejectConditionNotOpen:
		v := types.QueryRejectConditionNotOpen
		return &v
	case shared.QueryRejectConditionNotCompletedCleanly:
		v := types.QueryRejectConditionNotCompletedCleanly
		return &v
	}
	panic("unexpected enum value")
}

// FromQueryRejected converts internal QueryRejected type to thrift
func FromQueryRejected(t *types.QueryRejected) *shared.QueryRejected {
	if t == nil {
		return nil
	}
	return &shared.QueryRejected{
		CloseStatus: FromWorkflowExecutionCloseStatus(t.CloseStatus),
	}
}

// ToQueryRejected converts thrift QueryRejected type to internal
func ToQueryRejected(t *shared.QueryRejected) *types.QueryRejected {
	if t == nil {
		return nil
	}
	return &types.QueryRejected{
		CloseStatus: ToWorkflowExecutionCloseStatus(t.CloseStatus),
	}
}

// FromQueryResultType converts internal QueryResultType type to thrift
func FromQueryResultType(t *types.QueryResultType) *shared.QueryResultType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.QueryResultTypeAnswered:
		v := shared.QueryResultTypeAnswered
		return &v
	case types.QueryResultTypeFailed:
		v := shared.QueryResultTypeFailed
		return &v
	}
	panic("unexpected enum value")
}

// ToQueryResultType converts thrift QueryResultType type to internal
func ToQueryResultType(t *shared.QueryResultType) *types.QueryResultType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.QueryResultTypeAnswered:
		v := types.QueryResultTypeAnswered
		return &v
	case shared.QueryResultTypeFailed:
		v := types.QueryResultTypeFailed
		return &v
	}
	panic("unexpected enum value")
}

// FromQueryTaskCompletedType converts internal QueryTaskCompletedType type to thrift
func FromQueryTaskCompletedType(t *types.QueryTaskCompletedType) *shared.QueryTaskCompletedType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.QueryTaskCompletedTypeCompleted:
		v := shared.QueryTaskCompletedTypeCompleted
		return &v
	case types.QueryTaskCompletedTypeFailed:
		v := shared.QueryTaskCompletedTypeFailed
		return &v
	}
	panic("unexpected enum value")
}

// ToQueryTaskCompletedType converts thrift QueryTaskCompletedType type to internal
func ToQueryTaskCompletedType(t *shared.QueryTaskCompletedType) *types.QueryTaskCompletedType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.QueryTaskCompletedTypeCompleted:
		v := types.QueryTaskCompletedTypeCompleted
		return &v
	case shared.QueryTaskCompletedTypeFailed:
		v := types.QueryTaskCompletedTypeFailed
		return &v
	}
	panic("unexpected enum value")
}

// FromQueryWorkflowRequest converts internal QueryWorkflowRequest type to thrift
func FromQueryWorkflowRequest(t *types.QueryWorkflowRequest) *shared.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &shared.QueryWorkflowRequest{
		Domain:                &t.Domain,
		Execution:             FromWorkflowExecution(t.Execution),
		Query:                 FromWorkflowQuery(t.Query),
		QueryRejectCondition:  FromQueryRejectCondition(t.QueryRejectCondition),
		QueryConsistencyLevel: FromQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// ToQueryWorkflowRequest converts thrift QueryWorkflowRequest type to internal
func ToQueryWorkflowRequest(t *shared.QueryWorkflowRequest) *types.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.QueryWorkflowRequest{
		Domain:                t.GetDomain(),
		Execution:             ToWorkflowExecution(t.Execution),
		Query:                 ToWorkflowQuery(t.Query),
		QueryRejectCondition:  ToQueryRejectCondition(t.QueryRejectCondition),
		QueryConsistencyLevel: ToQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

// FromQueryWorkflowResponse converts internal QueryWorkflowResponse type to thrift
func FromQueryWorkflowResponse(t *types.QueryWorkflowResponse) *shared.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &shared.QueryWorkflowResponse{
		QueryResult:   t.QueryResult,
		QueryRejected: FromQueryRejected(t.QueryRejected),
	}
}

// ToQueryWorkflowResponse converts thrift QueryWorkflowResponse type to internal
func ToQueryWorkflowResponse(t *shared.QueryWorkflowResponse) *types.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.QueryWorkflowResponse{
		QueryResult:   t.QueryResult,
		QueryRejected: ToQueryRejected(t.QueryRejected),
	}
}

// FromAdminReapplyEventsRequest converts internal ReapplyEventsRequest type to thrift
func FromAdminReapplyEventsRequest(t *types.ReapplyEventsRequest) *shared.ReapplyEventsRequest {
	if t == nil {
		return nil
	}
	return &shared.ReapplyEventsRequest{
		DomainName:        &t.DomainName,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Events:            FromDataBlob(t.Events),
	}
}

// ToAdminReapplyEventsRequest converts thrift ReapplyEventsRequest type to internal
func ToAdminReapplyEventsRequest(t *shared.ReapplyEventsRequest) *types.ReapplyEventsRequest {
	if t == nil {
		return nil
	}
	return &types.ReapplyEventsRequest{
		DomainName:        t.GetDomainName(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Events:            ToDataBlob(t.Events),
	}
}

// FromRecordActivityTaskHeartbeatByIDRequest converts internal RecordActivityTaskHeartbeatByIDRequest type to thrift
func FromRecordActivityTaskHeartbeatByIDRequest(t *types.RecordActivityTaskHeartbeatByIDRequest) *shared.RecordActivityTaskHeartbeatByIDRequest {
	if t == nil {
		return nil
	}
	return &shared.RecordActivityTaskHeartbeatByIDRequest{
		Domain:     &t.Domain,
		WorkflowID: &t.WorkflowID,
		RunID:      &t.RunID,
		ActivityID: &t.ActivityID,
		Details:    t.Details,
		Identity:   &t.Identity,
	}
}

// ToRecordActivityTaskHeartbeatByIDRequest converts thrift RecordActivityTaskHeartbeatByIDRequest type to internal
func ToRecordActivityTaskHeartbeatByIDRequest(t *shared.RecordActivityTaskHeartbeatByIDRequest) *types.RecordActivityTaskHeartbeatByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatByIDRequest{
		Domain:     t.GetDomain(),
		WorkflowID: t.GetWorkflowID(),
		RunID:      t.GetRunID(),
		ActivityID: t.GetActivityID(),
		Details:    t.Details,
		Identity:   t.GetIdentity(),
	}
}

// FromRecordActivityTaskHeartbeatRequest converts internal RecordActivityTaskHeartbeatRequest type to thrift
func FromRecordActivityTaskHeartbeatRequest(t *types.RecordActivityTaskHeartbeatRequest) *shared.RecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &shared.RecordActivityTaskHeartbeatRequest{
		TaskToken: t.TaskToken,
		Details:   t.Details,
		Identity:  &t.Identity,
	}
}

// ToRecordActivityTaskHeartbeatRequest converts thrift RecordActivityTaskHeartbeatRequest type to internal
func ToRecordActivityTaskHeartbeatRequest(t *shared.RecordActivityTaskHeartbeatRequest) *types.RecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: t.TaskToken,
		Details:   t.Details,
		Identity:  t.GetIdentity(),
	}
}

// FromRecordActivityTaskHeartbeatResponse converts internal RecordActivityTaskHeartbeatResponse type to thrift
func FromRecordActivityTaskHeartbeatResponse(t *types.RecordActivityTaskHeartbeatResponse) *shared.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &shared.RecordActivityTaskHeartbeatResponse{
		CancelRequested: &t.CancelRequested,
	}
}

// ToRecordActivityTaskHeartbeatResponse converts thrift RecordActivityTaskHeartbeatResponse type to internal
func ToRecordActivityTaskHeartbeatResponse(t *shared.RecordActivityTaskHeartbeatResponse) *types.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.GetCancelRequested(),
	}
}

// FromRecordMarkerDecisionAttributes converts internal RecordMarkerDecisionAttributes type to thrift
func FromRecordMarkerDecisionAttributes(t *types.RecordMarkerDecisionAttributes) *shared.RecordMarkerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.RecordMarkerDecisionAttributes{
		MarkerName: &t.MarkerName,
		Details:    t.Details,
		Header:     FromHeader(t.Header),
	}
}

// ToRecordMarkerDecisionAttributes converts thrift RecordMarkerDecisionAttributes type to internal
func ToRecordMarkerDecisionAttributes(t *shared.RecordMarkerDecisionAttributes) *types.RecordMarkerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RecordMarkerDecisionAttributes{
		MarkerName: t.GetMarkerName(),
		Details:    t.Details,
		Header:     ToHeader(t.Header),
	}
}

// FromRefreshWorkflowTasksRequest converts internal RefreshWorkflowTasksRequest type to thrift
func FromRefreshWorkflowTasksRequest(t *types.RefreshWorkflowTasksRequest) *shared.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &shared.RefreshWorkflowTasksRequest{
		Domain:    &t.Domain,
		Execution: FromWorkflowExecution(t.Execution),
	}
}

// ToRefreshWorkflowTasksRequest converts thrift RefreshWorkflowTasksRequest type to internal
func ToRefreshWorkflowTasksRequest(t *shared.RefreshWorkflowTasksRequest) *types.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &types.RefreshWorkflowTasksRequest{
		Domain:    t.GetDomain(),
		Execution: ToWorkflowExecution(t.Execution),
	}
}

// FromRegisterDomainRequest converts internal RegisterDomainRequest type to thrift
func FromRegisterDomainRequest(t *types.RegisterDomainRequest) *shared.RegisterDomainRequest {
	if t == nil {
		return nil
	}
	return &shared.RegisterDomainRequest{
		Name:                                   &t.Name,
		Description:                            &t.Description,
		OwnerEmail:                             &t.OwnerEmail,
		WorkflowExecutionRetentionPeriodInDays: &t.WorkflowExecutionRetentionPeriodInDays,
		EmitMetric:                             t.EmitMetric,
		Clusters:                               FromClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusterName:                      &t.ActiveClusterName,
		ActiveClusters:                         FromActiveClusters(t.ActiveClusters),
		Data:                                   t.Data,
		SecurityToken:                          &t.SecurityToken,
		IsGlobalDomain:                         &t.IsGlobalDomain,
		HistoryArchivalStatus:                  FromArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalURI:                     &t.HistoryArchivalURI,
		VisibilityArchivalStatus:               FromArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalURI:                  &t.VisibilityArchivalURI,
	}
}

// ToRegisterDomainRequest converts thrift RegisterDomainRequest type to internal
func ToRegisterDomainRequest(t *shared.RegisterDomainRequest) *types.RegisterDomainRequest {
	if t == nil {
		return nil
	}
	return &types.RegisterDomainRequest{
		Name:                                   t.GetName(),
		Description:                            t.GetDescription(),
		OwnerEmail:                             t.GetOwnerEmail(),
		WorkflowExecutionRetentionPeriodInDays: t.GetWorkflowExecutionRetentionPeriodInDays(),
		EmitMetric:                             t.EmitMetric,
		Clusters:                               ToClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusterName:                      t.GetActiveClusterName(),
		ActiveClusters:                         ToActiveClusters(t.ActiveClusters),
		Data:                                   t.Data,
		SecurityToken:                          t.GetSecurityToken(),
		IsGlobalDomain:                         t.GetIsGlobalDomain(),
		HistoryArchivalStatus:                  ToArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalURI:                     t.GetHistoryArchivalURI(),
		VisibilityArchivalStatus:               ToArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalURI:                  t.GetVisibilityArchivalURI(),
	}
}

// FromRemoteSyncMatchedError converts internal RemoteSyncMatchedError type to thrift
func FromRemoteSyncMatchedError(t *types.RemoteSyncMatchedError) *shared.RemoteSyncMatchedError {
	if t == nil {
		return nil
	}
	return &shared.RemoteSyncMatchedError{
		Message: t.Message,
	}
}

// ToRemoteSyncMatchedError converts thrift RemoteSyncMatchedError type to internal
func ToRemoteSyncMatchedError(t *shared.RemoteSyncMatchedError) *types.RemoteSyncMatchedError {
	if t == nil {
		return nil
	}
	return &types.RemoteSyncMatchedError{
		Message: t.Message,
	}
}

// FromAdminRemoveTaskRequest converts internal RemoveTaskRequest type to thrift
func FromAdminRemoveTaskRequest(t *types.RemoveTaskRequest) *shared.RemoveTaskRequest {
	if t == nil {
		return nil
	}
	return &shared.RemoveTaskRequest{
		ShardID:             &t.ShardID,
		Type:                t.Type,
		TaskID:              &t.TaskID,
		VisibilityTimestamp: t.VisibilityTimestamp,
		ClusterName:         &t.ClusterName,
	}
}

// ToAdminRemoveTaskRequest converts thrift RemoveTaskRequest type to internal
func ToAdminRemoveTaskRequest(t *shared.RemoveTaskRequest) *types.RemoveTaskRequest {
	if t == nil {
		return nil
	}
	return &types.RemoveTaskRequest{
		ShardID:             t.GetShardID(),
		Type:                t.Type,
		TaskID:              t.GetTaskID(),
		VisibilityTimestamp: t.VisibilityTimestamp,
		ClusterName:         t.GetClusterName(),
	}
}

// FromRequestCancelActivityTaskDecisionAttributes converts internal RequestCancelActivityTaskDecisionAttributes type to thrift
func FromRequestCancelActivityTaskDecisionAttributes(t *types.RequestCancelActivityTaskDecisionAttributes) *shared.RequestCancelActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelActivityTaskDecisionAttributes{
		ActivityId: &t.ActivityID,
	}
}

// ToRequestCancelActivityTaskDecisionAttributes converts thrift RequestCancelActivityTaskDecisionAttributes type to internal
func ToRequestCancelActivityTaskDecisionAttributes(t *shared.RequestCancelActivityTaskDecisionAttributes) *types.RequestCancelActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelActivityTaskDecisionAttributes{
		ActivityID: t.GetActivityId(),
	}
}

// FromRequestCancelActivityTaskFailedEventAttributes converts internal RequestCancelActivityTaskFailedEventAttributes type to thrift
func FromRequestCancelActivityTaskFailedEventAttributes(t *types.RequestCancelActivityTaskFailedEventAttributes) *shared.RequestCancelActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelActivityTaskFailedEventAttributes{
		ActivityId:                   &t.ActivityID,
		Cause:                        &t.Cause,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToRequestCancelActivityTaskFailedEventAttributes converts thrift RequestCancelActivityTaskFailedEventAttributes type to internal
func ToRequestCancelActivityTaskFailedEventAttributes(t *shared.RequestCancelActivityTaskFailedEventAttributes) *types.RequestCancelActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelActivityTaskFailedEventAttributes{
		ActivityID:                   t.GetActivityId(),
		Cause:                        t.GetCause(),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromRequestCancelExternalWorkflowExecutionDecisionAttributes converts internal RequestCancelExternalWorkflowExecutionDecisionAttributes type to thrift
func FromRequestCancelExternalWorkflowExecutionDecisionAttributes(t *types.RequestCancelExternalWorkflowExecutionDecisionAttributes) *shared.RequestCancelExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		Domain:            &t.Domain,
		WorkflowId:        &t.WorkflowID,
		RunId:             &t.RunID,
		Control:           t.Control,
		ChildWorkflowOnly: &t.ChildWorkflowOnly,
	}
}

// ToRequestCancelExternalWorkflowExecutionDecisionAttributes converts thrift RequestCancelExternalWorkflowExecutionDecisionAttributes type to internal
func ToRequestCancelExternalWorkflowExecutionDecisionAttributes(t *shared.RequestCancelExternalWorkflowExecutionDecisionAttributes) *types.RequestCancelExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.GetDomain(),
		WorkflowID:        t.GetWorkflowId(),
		RunID:             t.GetRunId(),
		Control:           t.Control,
		ChildWorkflowOnly: t.GetChildWorkflowOnly(),
	}
}

// FromRequestCancelExternalWorkflowExecutionFailedEventAttributes converts internal RequestCancelExternalWorkflowExecutionFailedEventAttributes type to thrift
func FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(t *types.RequestCancelExternalWorkflowExecutionFailedEventAttributes) *shared.RequestCancelExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        FromCancelExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Domain:                       &t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		InitiatedEventId:             &t.InitiatedEventID,
		Control:                      t.Control,
	}
}

// ToRequestCancelExternalWorkflowExecutionFailedEventAttributes converts thrift RequestCancelExternalWorkflowExecutionFailedEventAttributes type to internal
func ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(t *shared.RequestCancelExternalWorkflowExecutionFailedEventAttributes) *types.RequestCancelExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        ToCancelExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Domain:                       t.GetDomain(),
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		InitiatedEventID:             t.GetInitiatedEventId(),
		Control:                      t.Control,
	}
}

// FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes converts internal RequestCancelExternalWorkflowExecutionInitiatedEventAttributes type to thrift
func FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t *types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) *shared.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Domain:                       &t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		Control:                      t.Control,
		ChildWorkflowOnly:            &t.ChildWorkflowOnly,
	}
}

// ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes converts thrift RequestCancelExternalWorkflowExecutionInitiatedEventAttributes type to internal
func ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t *shared.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) *types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Domain:                       t.GetDomain(),
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		Control:                      t.Control,
		ChildWorkflowOnly:            t.GetChildWorkflowOnly(),
	}
}

// FromRequestCancelWorkflowExecutionRequest converts internal RequestCancelWorkflowExecutionRequest type to thrift
func FromRequestCancelWorkflowExecutionRequest(t *types.RequestCancelWorkflowExecutionRequest) *shared.RequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.RequestCancelWorkflowExecutionRequest{
		Domain:              &t.Domain,
		WorkflowExecution:   FromWorkflowExecution(t.WorkflowExecution),
		Identity:            &t.Identity,
		RequestId:           &t.RequestID,
		Cause:               &t.Cause,
		FirstExecutionRunID: &t.FirstExecutionRunID,
	}
}

// ToRequestCancelWorkflowExecutionRequest converts thrift RequestCancelWorkflowExecutionRequest type to internal
func ToRequestCancelWorkflowExecutionRequest(t *shared.RequestCancelWorkflowExecutionRequest) *types.RequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.RequestCancelWorkflowExecutionRequest{
		Domain:              t.GetDomain(),
		WorkflowExecution:   ToWorkflowExecution(t.WorkflowExecution),
		Identity:            t.GetIdentity(),
		RequestID:           t.GetRequestId(),
		Cause:               t.GetCause(),
		FirstExecutionRunID: t.GetFirstExecutionRunID(),
	}
}

// FromResetPointInfo converts internal ResetPointInfo type to thrift
func FromResetPointInfo(t *types.ResetPointInfo) *shared.ResetPointInfo {
	if t == nil {
		return nil
	}
	return &shared.ResetPointInfo{
		BinaryChecksum:           &t.BinaryChecksum,
		RunId:                    &t.RunID,
		FirstDecisionCompletedId: &t.FirstDecisionCompletedID,
		CreatedTimeNano:          t.CreatedTimeNano,
		ExpiringTimeNano:         t.ExpiringTimeNano,
		Resettable:               &t.Resettable,
	}
}

// ToResetPointInfo converts thrift ResetPointInfo type to internal
func ToResetPointInfo(t *shared.ResetPointInfo) *types.ResetPointInfo {
	if t == nil {
		return nil
	}
	return &types.ResetPointInfo{
		BinaryChecksum:           t.GetBinaryChecksum(),
		RunID:                    t.GetRunId(),
		FirstDecisionCompletedID: t.GetFirstDecisionCompletedId(),
		CreatedTimeNano:          t.CreatedTimeNano,
		ExpiringTimeNano:         t.ExpiringTimeNano,
		Resettable:               t.GetResettable(),
	}
}

// FromResetPoints converts internal ResetPoints type to thrift
func FromResetPoints(t *types.ResetPoints) *shared.ResetPoints {
	if t == nil {
		return nil
	}
	return &shared.ResetPoints{
		Points: FromResetPointInfoArray(t.Points),
	}
}

// ToResetPoints converts thrift ResetPoints type to internal
func ToResetPoints(t *shared.ResetPoints) *types.ResetPoints {
	if t == nil {
		return nil
	}
	return &types.ResetPoints{
		Points: ToResetPointInfoArray(t.Points),
	}
}

// FromAdminResetQueueRequest converts internal ResetQueueRequest type to thrift
func FromAdminResetQueueRequest(t *types.ResetQueueRequest) *shared.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &shared.ResetQueueRequest{
		ShardID:     &t.ShardID,
		ClusterName: &t.ClusterName,
		Type:        t.Type,
	}
}

// ToAdminResetQueueRequest converts thrift ResetQueueRequest type to internal
func ToAdminResetQueueRequest(t *shared.ResetQueueRequest) *types.ResetQueueRequest {
	if t == nil {
		return nil
	}
	return &types.ResetQueueRequest{
		ShardID:     t.GetShardID(),
		ClusterName: t.GetClusterName(),
		Type:        t.Type,
	}
}

// FromResetStickyTaskListRequest converts internal ResetStickyTaskListRequest type to thrift
func FromResetStickyTaskListRequest(t *types.ResetStickyTaskListRequest) *shared.ResetStickyTaskListRequest {
	if t == nil {
		return nil
	}
	return &shared.ResetStickyTaskListRequest{
		Domain:    &t.Domain,
		Execution: FromWorkflowExecution(t.Execution),
	}
}

// ToResetStickyTaskListRequest converts thrift ResetStickyTaskListRequest type to internal
func ToResetStickyTaskListRequest(t *shared.ResetStickyTaskListRequest) *types.ResetStickyTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.ResetStickyTaskListRequest{
		Domain:    t.GetDomain(),
		Execution: ToWorkflowExecution(t.Execution),
	}
}

// FromResetStickyTaskListResponse converts internal ResetStickyTaskListResponse type to thrift
func FromResetStickyTaskListResponse(t *types.ResetStickyTaskListResponse) *shared.ResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &shared.ResetStickyTaskListResponse{}
}

// ToResetStickyTaskListResponse converts thrift ResetStickyTaskListResponse type to internal
func ToResetStickyTaskListResponse(t *shared.ResetStickyTaskListResponse) *types.ResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.ResetStickyTaskListResponse{}
}

// FromResetWorkflowExecutionRequest converts internal ResetWorkflowExecutionRequest type to thrift
func FromResetWorkflowExecutionRequest(t *types.ResetWorkflowExecutionRequest) *shared.ResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.ResetWorkflowExecutionRequest{
		Domain:                &t.Domain,
		WorkflowExecution:     FromWorkflowExecution(t.WorkflowExecution),
		Reason:                &t.Reason,
		DecisionFinishEventId: &t.DecisionFinishEventID,
		RequestId:             &t.RequestID,
		SkipSignalReapply:     &t.SkipSignalReapply,
	}
}

// ToResetWorkflowExecutionRequest converts thrift ResetWorkflowExecutionRequest type to internal
func ToResetWorkflowExecutionRequest(t *shared.ResetWorkflowExecutionRequest) *types.ResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.ResetWorkflowExecutionRequest{
		Domain:                t.GetDomain(),
		WorkflowExecution:     ToWorkflowExecution(t.WorkflowExecution),
		Reason:                t.GetReason(),
		DecisionFinishEventID: t.GetDecisionFinishEventId(),
		RequestID:             t.GetRequestId(),
		SkipSignalReapply:     t.GetSkipSignalReapply(),
	}
}

// FromResetWorkflowExecutionResponse converts internal ResetWorkflowExecutionResponse type to thrift
func FromResetWorkflowExecutionResponse(t *types.ResetWorkflowExecutionResponse) *shared.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &shared.ResetWorkflowExecutionResponse{
		RunId: &t.RunID,
	}
}

// ToResetWorkflowExecutionResponse converts thrift ResetWorkflowExecutionResponse type to internal
func ToResetWorkflowExecutionResponse(t *shared.ResetWorkflowExecutionResponse) *types.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.ResetWorkflowExecutionResponse{
		RunID: t.GetRunId(),
	}
}

// FromRespondActivityTaskCanceledByIDRequest converts internal RespondActivityTaskCanceledByIDRequest type to thrift
func FromRespondActivityTaskCanceledByIDRequest(t *types.RespondActivityTaskCanceledByIDRequest) *shared.RespondActivityTaskCanceledByIDRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskCanceledByIDRequest{
		Domain:     &t.Domain,
		WorkflowID: &t.WorkflowID,
		RunID:      &t.RunID,
		ActivityID: &t.ActivityID,
		Details:    t.Details,
		Identity:   &t.Identity,
	}
}

// ToRespondActivityTaskCanceledByIDRequest converts thrift RespondActivityTaskCanceledByIDRequest type to internal
func ToRespondActivityTaskCanceledByIDRequest(t *shared.RespondActivityTaskCanceledByIDRequest) *types.RespondActivityTaskCanceledByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCanceledByIDRequest{
		Domain:     t.GetDomain(),
		WorkflowID: t.GetWorkflowID(),
		RunID:      t.GetRunID(),
		ActivityID: t.GetActivityID(),
		Details:    t.Details,
		Identity:   t.GetIdentity(),
	}
}

// FromRespondActivityTaskCanceledRequest converts internal RespondActivityTaskCanceledRequest type to thrift
func FromRespondActivityTaskCanceledRequest(t *types.RespondActivityTaskCanceledRequest) *shared.RespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskCanceledRequest{
		TaskToken: t.TaskToken,
		Details:   t.Details,
		Identity:  &t.Identity,
	}
}

// ToRespondActivityTaskCanceledRequest converts thrift RespondActivityTaskCanceledRequest type to internal
func ToRespondActivityTaskCanceledRequest(t *shared.RespondActivityTaskCanceledRequest) *types.RespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCanceledRequest{
		TaskToken: t.TaskToken,
		Details:   t.Details,
		Identity:  t.GetIdentity(),
	}
}

// FromRespondActivityTaskCompletedByIDRequest converts internal RespondActivityTaskCompletedByIDRequest type to thrift
func FromRespondActivityTaskCompletedByIDRequest(t *types.RespondActivityTaskCompletedByIDRequest) *shared.RespondActivityTaskCompletedByIDRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskCompletedByIDRequest{
		Domain:     &t.Domain,
		WorkflowID: &t.WorkflowID,
		RunID:      &t.RunID,
		ActivityID: &t.ActivityID,
		Result:     t.Result,
		Identity:   &t.Identity,
	}
}

// ToRespondActivityTaskCompletedByIDRequest converts thrift RespondActivityTaskCompletedByIDRequest type to internal
func ToRespondActivityTaskCompletedByIDRequest(t *shared.RespondActivityTaskCompletedByIDRequest) *types.RespondActivityTaskCompletedByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCompletedByIDRequest{
		Domain:     t.GetDomain(),
		WorkflowID: t.GetWorkflowID(),
		RunID:      t.GetRunID(),
		ActivityID: t.GetActivityID(),
		Result:     t.Result,
		Identity:   t.GetIdentity(),
	}
}

// FromRespondActivityTaskCompletedRequest converts internal RespondActivityTaskCompletedRequest type to thrift
func FromRespondActivityTaskCompletedRequest(t *types.RespondActivityTaskCompletedRequest) *shared.RespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskCompletedRequest{
		TaskToken: t.TaskToken,
		Result:    t.Result,
		Identity:  &t.Identity,
	}
}

// ToRespondActivityTaskCompletedRequest converts thrift RespondActivityTaskCompletedRequest type to internal
func ToRespondActivityTaskCompletedRequest(t *shared.RespondActivityTaskCompletedRequest) *types.RespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCompletedRequest{
		TaskToken: t.TaskToken,
		Result:    t.Result,
		Identity:  t.GetIdentity(),
	}
}

// FromRespondActivityTaskFailedByIDRequest converts internal RespondActivityTaskFailedByIDRequest type to thrift
func FromRespondActivityTaskFailedByIDRequest(t *types.RespondActivityTaskFailedByIDRequest) *shared.RespondActivityTaskFailedByIDRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskFailedByIDRequest{
		Domain:     &t.Domain,
		WorkflowID: &t.WorkflowID,
		RunID:      &t.RunID,
		ActivityID: &t.ActivityID,
		Reason:     t.Reason,
		Details:    t.Details,
		Identity:   &t.Identity,
	}
}

// ToRespondActivityTaskFailedByIDRequest converts thrift RespondActivityTaskFailedByIDRequest type to internal
func ToRespondActivityTaskFailedByIDRequest(t *shared.RespondActivityTaskFailedByIDRequest) *types.RespondActivityTaskFailedByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskFailedByIDRequest{
		Domain:     t.GetDomain(),
		WorkflowID: t.GetWorkflowID(),
		RunID:      t.GetRunID(),
		ActivityID: t.GetActivityID(),
		Reason:     t.Reason,
		Details:    t.Details,
		Identity:   t.GetIdentity(),
	}
}

// FromRespondActivityTaskFailedRequest converts internal RespondActivityTaskFailedRequest type to thrift
func FromRespondActivityTaskFailedRequest(t *types.RespondActivityTaskFailedRequest) *shared.RespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondActivityTaskFailedRequest{
		TaskToken: t.TaskToken,
		Reason:    t.Reason,
		Details:   t.Details,
		Identity:  &t.Identity,
	}
}

// ToRespondActivityTaskFailedRequest converts thrift RespondActivityTaskFailedRequest type to internal
func ToRespondActivityTaskFailedRequest(t *shared.RespondActivityTaskFailedRequest) *types.RespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskFailedRequest{
		TaskToken: t.TaskToken,
		Reason:    t.Reason,
		Details:   t.Details,
		Identity:  t.GetIdentity(),
	}
}

// FromRespondDecisionTaskCompletedRequest converts internal RespondDecisionTaskCompletedRequest type to thrift
func FromRespondDecisionTaskCompletedRequest(t *types.RespondDecisionTaskCompletedRequest) *shared.RespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondDecisionTaskCompletedRequest{
		TaskToken:                  t.TaskToken,
		Decisions:                  FromDecisionArray(t.Decisions),
		ExecutionContext:           t.ExecutionContext,
		Identity:                   &t.Identity,
		StickyAttributes:           FromStickyExecutionAttributes(t.StickyAttributes),
		ReturnNewDecisionTask:      &t.ReturnNewDecisionTask,
		ForceCreateNewDecisionTask: &t.ForceCreateNewDecisionTask,
		BinaryChecksum:             &t.BinaryChecksum,
		QueryResults:               FromWorkflowQueryResultMap(t.QueryResults),
	}
}

// ToRespondDecisionTaskCompletedRequest converts thrift RespondDecisionTaskCompletedRequest type to internal
func ToRespondDecisionTaskCompletedRequest(t *shared.RespondDecisionTaskCompletedRequest) *types.RespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskCompletedRequest{
		TaskToken:                  t.TaskToken,
		Decisions:                  ToDecisionArray(t.Decisions),
		ExecutionContext:           t.ExecutionContext,
		Identity:                   t.GetIdentity(),
		StickyAttributes:           ToStickyExecutionAttributes(t.StickyAttributes),
		ReturnNewDecisionTask:      t.GetReturnNewDecisionTask(),
		ForceCreateNewDecisionTask: t.GetForceCreateNewDecisionTask(),
		BinaryChecksum:             t.GetBinaryChecksum(),
		QueryResults:               ToWorkflowQueryResultMap(t.QueryResults),
	}
}

// FromRespondDecisionTaskCompletedResponse converts internal RespondDecisionTaskCompletedResponse type to thrift
func FromRespondDecisionTaskCompletedResponse(t *types.RespondDecisionTaskCompletedResponse) *shared.RespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &shared.RespondDecisionTaskCompletedResponse{
		DecisionTask:                FromPollForDecisionTaskResponse(t.DecisionTask),
		ActivitiesToDispatchLocally: FromActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

// ToRespondDecisionTaskCompletedResponse converts thrift RespondDecisionTaskCompletedResponse type to internal
func ToRespondDecisionTaskCompletedResponse(t *shared.RespondDecisionTaskCompletedResponse) *types.RespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskCompletedResponse{
		DecisionTask:                ToPollForDecisionTaskResponse(t.DecisionTask),
		ActivitiesToDispatchLocally: ToActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

// FromRespondDecisionTaskFailedRequest converts internal RespondDecisionTaskFailedRequest type to thrift
func FromRespondDecisionTaskFailedRequest(t *types.RespondDecisionTaskFailedRequest) *shared.RespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondDecisionTaskFailedRequest{
		TaskToken:      t.TaskToken,
		Cause:          FromDecisionTaskFailedCause(t.Cause),
		Details:        t.Details,
		Identity:       &t.Identity,
		BinaryChecksum: &t.BinaryChecksum,
	}
}

// ToRespondDecisionTaskFailedRequest converts thrift RespondDecisionTaskFailedRequest type to internal
func ToRespondDecisionTaskFailedRequest(t *shared.RespondDecisionTaskFailedRequest) *types.RespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskFailedRequest{
		TaskToken:      t.TaskToken,
		Cause:          ToDecisionTaskFailedCause(t.Cause),
		Details:        t.Details,
		Identity:       t.GetIdentity(),
		BinaryChecksum: t.GetBinaryChecksum(),
	}
}

// FromRespondQueryTaskCompletedRequest converts internal RespondQueryTaskCompletedRequest type to thrift
func FromRespondQueryTaskCompletedRequest(t *types.RespondQueryTaskCompletedRequest) *shared.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondQueryTaskCompletedRequest{
		TaskToken:         t.TaskToken,
		CompletedType:     FromQueryTaskCompletedType(t.CompletedType),
		QueryResult:       t.QueryResult,
		ErrorMessage:      &t.ErrorMessage,
		WorkerVersionInfo: FromWorkerVersionInfo(t.WorkerVersionInfo),
	}
}

// ToRespondQueryTaskCompletedRequest converts thrift RespondQueryTaskCompletedRequest type to internal
func ToRespondQueryTaskCompletedRequest(t *shared.RespondQueryTaskCompletedRequest) *types.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondQueryTaskCompletedRequest{
		TaskToken:         t.TaskToken,
		CompletedType:     ToQueryTaskCompletedType(t.CompletedType),
		QueryResult:       t.QueryResult,
		ErrorMessage:      t.GetErrorMessage(),
		WorkerVersionInfo: ToWorkerVersionInfo(t.WorkerVersionInfo),
	}
}

// FromRetryPolicy converts internal RetryPolicy type to thrift
func FromRetryPolicy(t *types.RetryPolicy) *shared.RetryPolicy {
	if t == nil {
		return nil
	}
	return &shared.RetryPolicy{
		InitialIntervalInSeconds:    &t.InitialIntervalInSeconds,
		BackoffCoefficient:          &t.BackoffCoefficient,
		MaximumIntervalInSeconds:    &t.MaximumIntervalInSeconds,
		MaximumAttempts:             &t.MaximumAttempts,
		NonRetriableErrorReasons:    t.NonRetriableErrorReasons,
		ExpirationIntervalInSeconds: &t.ExpirationIntervalInSeconds,
	}
}

// ToRetryPolicy converts thrift RetryPolicy type to internal
func ToRetryPolicy(t *shared.RetryPolicy) *types.RetryPolicy {
	if t == nil {
		return nil
	}
	return &types.RetryPolicy{
		InitialIntervalInSeconds:    t.GetInitialIntervalInSeconds(),
		BackoffCoefficient:          t.GetBackoffCoefficient(),
		MaximumIntervalInSeconds:    t.GetMaximumIntervalInSeconds(),
		MaximumAttempts:             t.GetMaximumAttempts(),
		NonRetriableErrorReasons:    t.NonRetriableErrorReasons,
		ExpirationIntervalInSeconds: t.GetExpirationIntervalInSeconds(),
	}
}

// FromRetryTaskV2Error converts internal RetryTaskV2Error type to thrift
func FromRetryTaskV2Error(t *types.RetryTaskV2Error) *shared.RetryTaskV2Error {
	if t == nil {
		return nil
	}
	return &shared.RetryTaskV2Error{
		Message:           t.Message,
		DomainId:          &t.DomainID,
		WorkflowId:        &t.WorkflowID,
		RunId:             &t.RunID,
		StartEventId:      t.StartEventID,
		StartEventVersion: t.StartEventVersion,
		EndEventId:        t.EndEventID,
		EndEventVersion:   t.EndEventVersion,
	}
}

// ToRetryTaskV2Error converts thrift RetryTaskV2Error type to internal
func ToRetryTaskV2Error(t *shared.RetryTaskV2Error) *types.RetryTaskV2Error {
	if t == nil {
		return nil
	}
	return &types.RetryTaskV2Error{
		Message:           t.Message,
		DomainID:          t.GetDomainId(),
		WorkflowID:        t.GetWorkflowId(),
		RunID:             t.GetRunId(),
		StartEventID:      t.StartEventId,
		StartEventVersion: t.StartEventVersion,
		EndEventID:        t.EndEventId,
		EndEventVersion:   t.EndEventVersion,
	}
}

// FromRestartWorkflowExecutionRequest converts internal RestartWorkflowExecutionRequest type to thrift
func FromRestartWorkflowExecutionRequest(t *types.RestartWorkflowExecutionRequest) *shared.RestartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.RestartWorkflowExecutionRequest{
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Identity:          &t.Identity,
	}
}

// ToRestartWorkflowExecutionRequest converts thrift RestartWorkflowExecutionRequest type to internal
func ToRestartWorkflowExecutionRequest(t *shared.RestartWorkflowExecutionRequest) *types.RestartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.RestartWorkflowExecutionRequest{
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Identity:          t.GetIdentity(),
	}
}

// FromScheduleActivityTaskDecisionAttributes converts internal ScheduleActivityTaskDecisionAttributes type to thrift
func FromScheduleActivityTaskDecisionAttributes(t *types.ScheduleActivityTaskDecisionAttributes) *shared.ScheduleActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.ScheduleActivityTaskDecisionAttributes{
		ActivityId:                    &t.ActivityID,
		ActivityType:                  FromActivityType(t.ActivityType),
		Domain:                        &t.Domain,
		TaskList:                      FromTaskList(t.TaskList),
		Input:                         t.Input,
		ScheduleToCloseTimeoutSeconds: t.ScheduleToCloseTimeoutSeconds,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       t.HeartbeatTimeoutSeconds,
		RetryPolicy:                   FromRetryPolicy(t.RetryPolicy),
		Header:                        FromHeader(t.Header),
		RequestLocalDispatch:          &t.RequestLocalDispatch,
	}
}

// ToScheduleActivityTaskDecisionAttributes converts thrift ScheduleActivityTaskDecisionAttributes type to internal
func ToScheduleActivityTaskDecisionAttributes(t *shared.ScheduleActivityTaskDecisionAttributes) *types.ScheduleActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    t.GetActivityId(),
		ActivityType:                  ToActivityType(t.ActivityType),
		Domain:                        t.GetDomain(),
		TaskList:                      ToTaskList(t.TaskList),
		Input:                         t.Input,
		ScheduleToCloseTimeoutSeconds: t.ScheduleToCloseTimeoutSeconds,
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    t.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       t.HeartbeatTimeoutSeconds,
		RetryPolicy:                   ToRetryPolicy(t.RetryPolicy),
		Header:                        ToHeader(t.Header),
		RequestLocalDispatch:          t.GetRequestLocalDispatch(),
	}
}

// FromSearchAttributes converts internal SearchAttributes type to thrift
func FromSearchAttributes(t *types.SearchAttributes) *shared.SearchAttributes {
	if t == nil {
		return nil
	}
	return &shared.SearchAttributes{
		IndexedFields: t.IndexedFields,
	}
}

// ToSearchAttributes converts thrift SearchAttributes type to internal
func ToSearchAttributes(t *shared.SearchAttributes) *types.SearchAttributes {
	if t == nil {
		return nil
	}
	return &types.SearchAttributes{
		IndexedFields: t.IndexedFields,
	}
}

// FromServiceBusyError converts internal ServiceBusyError type to thrift
func FromServiceBusyError(t *types.ServiceBusyError) *shared.ServiceBusyError {
	if t == nil {
		return nil
	}
	return &shared.ServiceBusyError{
		Message: t.Message,
		Reason:  &t.Reason,
	}
}

// ToServiceBusyError converts thrift ServiceBusyError type to internal
func ToServiceBusyError(t *shared.ServiceBusyError) *types.ServiceBusyError {
	if t == nil {
		return nil
	}
	return &types.ServiceBusyError{
		Message: t.Message,
		Reason:  t.GetReason(),
	}
}

// FromSignalExternalWorkflowExecutionDecisionAttributes converts internal SignalExternalWorkflowExecutionDecisionAttributes type to thrift
func FromSignalExternalWorkflowExecutionDecisionAttributes(t *types.SignalExternalWorkflowExecutionDecisionAttributes) *shared.SignalExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain:            &t.Domain,
		Execution:         FromWorkflowExecution(t.Execution),
		SignalName:        &t.SignalName,
		Input:             t.Input,
		Control:           t.Control,
		ChildWorkflowOnly: &t.ChildWorkflowOnly,
	}
}

// ToSignalExternalWorkflowExecutionDecisionAttributes converts thrift SignalExternalWorkflowExecutionDecisionAttributes type to internal
func ToSignalExternalWorkflowExecutionDecisionAttributes(t *shared.SignalExternalWorkflowExecutionDecisionAttributes) *types.SignalExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.GetDomain(),
		Execution:         ToWorkflowExecution(t.Execution),
		SignalName:        t.GetSignalName(),
		Input:             t.Input,
		Control:           t.Control,
		ChildWorkflowOnly: t.GetChildWorkflowOnly(),
	}
}

// FromSignalExternalWorkflowExecutionFailedCause converts internal SignalExternalWorkflowExecutionFailedCause type to thrift
func FromSignalExternalWorkflowExecutionFailedCause(t *types.SignalExternalWorkflowExecutionFailedCause) *shared.SignalExternalWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		v := shared.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return &v
	case types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		v := shared.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return &v
	}
	panic("unexpected enum value")
}

// ToSignalExternalWorkflowExecutionFailedCause converts thrift SignalExternalWorkflowExecutionFailedCause type to internal
func ToSignalExternalWorkflowExecutionFailedCause(t *shared.SignalExternalWorkflowExecutionFailedCause) *types.SignalExternalWorkflowExecutionFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		v := types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution
		return &v
	case shared.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		v := types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted
		return &v
	}
	panic("unexpected enum value")
}

// FromSignalExternalWorkflowExecutionFailedEventAttributes converts internal SignalExternalWorkflowExecutionFailedEventAttributes type to thrift
func FromSignalExternalWorkflowExecutionFailedEventAttributes(t *types.SignalExternalWorkflowExecutionFailedEventAttributes) *shared.SignalExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.SignalExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        FromSignalExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Domain:                       &t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		InitiatedEventId:             &t.InitiatedEventID,
		Control:                      t.Control,
	}
}

// ToSignalExternalWorkflowExecutionFailedEventAttributes converts thrift SignalExternalWorkflowExecutionFailedEventAttributes type to internal
func ToSignalExternalWorkflowExecutionFailedEventAttributes(t *shared.SignalExternalWorkflowExecutionFailedEventAttributes) *types.SignalExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        ToSignalExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Domain:                       t.GetDomain(),
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		InitiatedEventID:             t.GetInitiatedEventId(),
		Control:                      t.Control,
	}
}

// FromSignalExternalWorkflowExecutionInitiatedEventAttributes converts internal SignalExternalWorkflowExecutionInitiatedEventAttributes type to thrift
func FromSignalExternalWorkflowExecutionInitiatedEventAttributes(t *types.SignalExternalWorkflowExecutionInitiatedEventAttributes) *shared.SignalExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Domain:                       &t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		SignalName:                   &t.SignalName,
		Input:                        t.Input,
		Control:                      t.Control,
		ChildWorkflowOnly:            &t.ChildWorkflowOnly,
	}
}

// ToSignalExternalWorkflowExecutionInitiatedEventAttributes converts thrift SignalExternalWorkflowExecutionInitiatedEventAttributes type to internal
func ToSignalExternalWorkflowExecutionInitiatedEventAttributes(t *shared.SignalExternalWorkflowExecutionInitiatedEventAttributes) *types.SignalExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Domain:                       t.GetDomain(),
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		SignalName:                   t.GetSignalName(),
		Input:                        t.Input,
		Control:                      t.Control,
		ChildWorkflowOnly:            t.GetChildWorkflowOnly(),
	}
}

// FromSignalWithStartWorkflowExecutionRequest converts internal SignalWithStartWorkflowExecutionRequest type to thrift
func FromSignalWithStartWorkflowExecutionRequest(t *types.SignalWithStartWorkflowExecutionRequest) *shared.SignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.SignalWithStartWorkflowExecutionRequest{
		Domain:                              &t.Domain,
		WorkflowId:                          &t.WorkflowID,
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		Identity:                            &t.Identity,
		RequestId:                           &t.RequestID,
		WorkflowIdReusePolicy:               FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		SignalName:                          &t.SignalName,
		SignalInput:                         t.SignalInput,
		Control:                             t.Control,
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                        &t.CronSchedule,
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		Header:                              FromHeader(t.Header),
		FirstRunAtTimestamp:                 t.FirstRunAtTimestamp,
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// ToSignalWithStartWorkflowExecutionRequest converts thrift SignalWithStartWorkflowExecutionRequest type to internal
func ToSignalWithStartWorkflowExecutionRequest(t *shared.SignalWithStartWorkflowExecutionRequest) *types.SignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionRequest{
		Domain:                              t.GetDomain(),
		WorkflowID:                          t.GetWorkflowId(),
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		Identity:                            t.GetIdentity(),
		RequestID:                           t.GetRequestId(),
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		SignalName:                          t.GetSignalName(),
		SignalInput:                         t.SignalInput,
		Control:                             t.Control,
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.GetCronSchedule(),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		Header:                              ToHeader(t.Header),
		FirstRunAtTimestamp:                 t.FirstRunAtTimestamp,
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// FromSignalWorkflowExecutionRequest converts internal SignalWorkflowExecutionRequest type to thrift
func FromSignalWorkflowExecutionRequest(t *types.SignalWorkflowExecutionRequest) *shared.SignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.SignalWorkflowExecutionRequest{
		Domain:            &t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		SignalName:        &t.SignalName,
		Input:             t.Input,
		Identity:          &t.Identity,
		RequestId:         &t.RequestID,
		Control:           t.Control,
	}
}

// ToSignalWorkflowExecutionRequest converts thrift SignalWorkflowExecutionRequest type to internal
func ToSignalWorkflowExecutionRequest(t *shared.SignalWorkflowExecutionRequest) *types.SignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWorkflowExecutionRequest{
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		SignalName:        t.GetSignalName(),
		Input:             t.Input,
		Identity:          t.GetIdentity(),
		RequestID:         t.GetRequestId(),
		Control:           t.Control,
	}
}

// FromStartChildWorkflowExecutionDecisionAttributes converts internal StartChildWorkflowExecutionDecisionAttributes type to thrift
func FromStartChildWorkflowExecutionDecisionAttributes(t *types.StartChildWorkflowExecutionDecisionAttributes) *shared.StartChildWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                              &t.Domain,
		WorkflowId:                          &t.WorkflowID,
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ParentClosePolicy:                   FromParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		WorkflowIdReusePolicy:               FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                        &t.CronSchedule,
		Header:                              FromHeader(t.Header),
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// ToStartChildWorkflowExecutionDecisionAttributes converts thrift StartChildWorkflowExecutionDecisionAttributes type to internal
func ToStartChildWorkflowExecutionDecisionAttributes(t *shared.StartChildWorkflowExecutionDecisionAttributes) *types.StartChildWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                              t.GetDomain(),
		WorkflowID:                          t.GetWorkflowId(),
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ParentClosePolicy:                   ToParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.GetCronSchedule(),
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// FromStartChildWorkflowExecutionFailedEventAttributes converts internal StartChildWorkflowExecutionFailedEventAttributes type to thrift
func FromStartChildWorkflowExecutionFailedEventAttributes(t *types.StartChildWorkflowExecutionFailedEventAttributes) *shared.StartChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       &t.Domain,
		WorkflowId:                   &t.WorkflowID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		Cause:                        FromChildWorkflowExecutionFailedCause(t.Cause),
		Control:                      t.Control,
		InitiatedEventId:             &t.InitiatedEventID,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToStartChildWorkflowExecutionFailedEventAttributes converts thrift StartChildWorkflowExecutionFailedEventAttributes type to internal
func ToStartChildWorkflowExecutionFailedEventAttributes(t *shared.StartChildWorkflowExecutionFailedEventAttributes) *types.StartChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       t.GetDomain(),
		WorkflowID:                   t.GetWorkflowId(),
		WorkflowType:                 ToWorkflowType(t.WorkflowType),
		Cause:                        ToChildWorkflowExecutionFailedCause(t.Cause),
		Control:                      t.Control,
		InitiatedEventID:             t.GetInitiatedEventId(),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromStartChildWorkflowExecutionInitiatedEventAttributes converts internal StartChildWorkflowExecutionInitiatedEventAttributes type to thrift
func FromStartChildWorkflowExecutionInitiatedEventAttributes(t *types.StartChildWorkflowExecutionInitiatedEventAttributes) *shared.StartChildWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              &t.Domain,
		WorkflowId:                          &t.WorkflowID,
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ParentClosePolicy:                   FromParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		DecisionTaskCompletedEventId:        &t.DecisionTaskCompletedEventID,
		WorkflowIdReusePolicy:               FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                        &t.CronSchedule,
		Header:                              FromHeader(t.Header),
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		DelayStartSeconds:                   t.DelayStartSeconds,
		JitterStartSeconds:                  t.JitterStartSeconds,
		FirstRunAtTimestamp:                 t.FirstRunAtTimestamp,
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// ToStartChildWorkflowExecutionInitiatedEventAttributes converts thrift StartChildWorkflowExecutionInitiatedEventAttributes type to internal
func ToStartChildWorkflowExecutionInitiatedEventAttributes(t *shared.StartChildWorkflowExecutionInitiatedEventAttributes) *types.StartChildWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              t.GetDomain(),
		WorkflowID:                          t.GetWorkflowId(),
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ParentClosePolicy:                   ToParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		DecisionTaskCompletedEventID:        t.GetDecisionTaskCompletedEventId(),
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.GetCronSchedule(),
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		DelayStartSeconds:                   t.DelayStartSeconds,
		JitterStartSeconds:                  t.JitterStartSeconds,
		FirstRunAtTimestamp:                 t.FirstRunAtTimestamp,
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// FromStartTimeFilter converts internal StartTimeFilter type to thrift
func FromStartTimeFilter(t *types.StartTimeFilter) *shared.StartTimeFilter {
	if t == nil {
		return nil
	}
	return &shared.StartTimeFilter{
		EarliestTime: t.EarliestTime,
		LatestTime:   t.LatestTime,
	}
}

// ToStartTimeFilter converts thrift StartTimeFilter type to internal
func ToStartTimeFilter(t *shared.StartTimeFilter) *types.StartTimeFilter {
	if t == nil {
		return nil
	}
	return &types.StartTimeFilter{
		EarliestTime: t.EarliestTime,
		LatestTime:   t.LatestTime,
	}
}

// FromStartTimerDecisionAttributes converts internal StartTimerDecisionAttributes type to thrift
func FromStartTimerDecisionAttributes(t *types.StartTimerDecisionAttributes) *shared.StartTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.StartTimerDecisionAttributes{
		TimerId:                   &t.TimerID,
		StartToFireTimeoutSeconds: t.StartToFireTimeoutSeconds,
	}
}

// ToStartTimerDecisionAttributes converts thrift StartTimerDecisionAttributes type to internal
func ToStartTimerDecisionAttributes(t *shared.StartTimerDecisionAttributes) *types.StartTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.StartTimerDecisionAttributes{
		TimerID:                   t.GetTimerId(),
		StartToFireTimeoutSeconds: t.StartToFireTimeoutSeconds,
	}
}

func FromSignalWithStartWorkflowExecutionAsyncRequest(t *types.SignalWithStartWorkflowExecutionAsyncRequest) *shared.SignalWithStartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &shared.SignalWithStartWorkflowExecutionAsyncRequest{
		Request: FromSignalWithStartWorkflowExecutionRequest(t.SignalWithStartWorkflowExecutionRequest),
	}
}

func ToSignalWithStartWorkflowExecutionAsyncRequest(t *shared.SignalWithStartWorkflowExecutionAsyncRequest) *types.SignalWithStartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionAsyncRequest{
		SignalWithStartWorkflowExecutionRequest: ToSignalWithStartWorkflowExecutionRequest(t.Request),
	}
}

func FromSignalWithStartWorkflowExecutionAsyncResponse(t *types.SignalWithStartWorkflowExecutionAsyncResponse) *shared.SignalWithStartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &shared.SignalWithStartWorkflowExecutionAsyncResponse{}
}

func ToSignalWithStartWorkflowExecutionAsyncResponse(t *shared.SignalWithStartWorkflowExecutionAsyncResponse) *types.SignalWithStartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionAsyncResponse{}
}

func FromStartWorkflowExecutionAsyncRequest(t *types.StartWorkflowExecutionAsyncRequest) *shared.StartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &shared.StartWorkflowExecutionAsyncRequest{
		Request: FromStartWorkflowExecutionRequest(t.StartWorkflowExecutionRequest),
	}
}

func ToStartWorkflowExecutionAsyncRequest(t *shared.StartWorkflowExecutionAsyncRequest) *types.StartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionAsyncRequest{
		StartWorkflowExecutionRequest: ToStartWorkflowExecutionRequest(t.Request),
	}
}

func FromStartWorkflowExecutionAsyncResponse(t *types.StartWorkflowExecutionAsyncResponse) *shared.StartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &shared.StartWorkflowExecutionAsyncResponse{}
}

func ToStartWorkflowExecutionAsyncResponse(t *shared.StartWorkflowExecutionAsyncResponse) *types.StartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionAsyncResponse{}
}

// FromStartWorkflowExecutionRequest converts internal StartWorkflowExecutionRequest type to thrift
func FromStartWorkflowExecutionRequest(t *types.StartWorkflowExecutionRequest) *shared.StartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	thriftPolicy := FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy)
	return &shared.StartWorkflowExecutionRequest{
		Domain:                              &t.Domain,
		WorkflowId:                          &t.WorkflowID,
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		Identity:                            &t.Identity,
		RequestId:                           &t.RequestID,
		WorkflowIdReusePolicy:               FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                        &t.CronSchedule,
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		Header:                              FromHeader(t.Header),
		DelayStartSeconds:                   t.DelayStartSeconds,
		JitterStartSeconds:                  t.JitterStartSeconds,
		FirstRunAtTimestamp:                 t.FirstRunAtTimeStamp,
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        thriftPolicy,
	}
}

// ToStartWorkflowExecutionRequest converts thrift StartWorkflowExecutionRequest type to internal
func ToStartWorkflowExecutionRequest(t *shared.StartWorkflowExecutionRequest) *types.StartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionRequest{
		Domain:                              t.GetDomain(),
		WorkflowID:                          t.GetWorkflowId(),
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		Identity:                            t.GetIdentity(),
		RequestID:                           t.GetRequestId(),
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.GetCronSchedule(),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		Header:                              ToHeader(t.Header),
		DelayStartSeconds:                   t.DelayStartSeconds,
		JitterStartSeconds:                  t.JitterStartSeconds,
		FirstRunAtTimeStamp:                 t.FirstRunAtTimestamp,
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// FromRestartWorkflowExecutionResponse converts internal RestartWorkflowExecutionResponse type to thrift
func FromRestartWorkflowExecutionResponse(t *types.RestartWorkflowExecutionResponse) *shared.RestartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &shared.RestartWorkflowExecutionResponse{
		RunId: &t.RunID,
	}
}

// FromStartWorkflowExecutionResponse converts internal StartWorkflowExecutionResponse type to thrift
func FromStartWorkflowExecutionResponse(t *types.StartWorkflowExecutionResponse) *shared.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &shared.StartWorkflowExecutionResponse{
		RunId: &t.RunID,
	}
}

// ToStartWorkflowExecutionResponse converts thrift StartWorkflowExecutionResponse type to internal
func ToStartWorkflowExecutionResponse(t *shared.StartWorkflowExecutionResponse) *types.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: t.GetRunId(),
	}
}

// FromStickyExecutionAttributes converts internal StickyExecutionAttributes type to thrift
func FromStickyExecutionAttributes(t *types.StickyExecutionAttributes) *shared.StickyExecutionAttributes {
	if t == nil {
		return nil
	}
	return &shared.StickyExecutionAttributes{
		WorkerTaskList:                FromTaskList(t.WorkerTaskList),
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
	}
}

// ToStickyExecutionAttributes converts thrift StickyExecutionAttributes type to internal
func ToStickyExecutionAttributes(t *shared.StickyExecutionAttributes) *types.StickyExecutionAttributes {
	if t == nil {
		return nil
	}
	return &types.StickyExecutionAttributes{
		WorkerTaskList:                ToTaskList(t.WorkerTaskList),
		ScheduleToStartTimeoutSeconds: t.ScheduleToStartTimeoutSeconds,
	}
}

// FromSupportedClientVersions converts internal SupportedClientVersions type to thrift
func FromSupportedClientVersions(t *types.SupportedClientVersions) *shared.SupportedClientVersions {
	if t == nil {
		return nil
	}
	return &shared.SupportedClientVersions{
		GoSdk:   &t.GoSdk,
		JavaSdk: &t.JavaSdk,
	}
}

// ToSupportedClientVersions converts thrift SupportedClientVersions type to internal
func ToSupportedClientVersions(t *shared.SupportedClientVersions) *types.SupportedClientVersions {
	if t == nil {
		return nil
	}
	return &types.SupportedClientVersions{
		GoSdk:   t.GetGoSdk(),
		JavaSdk: t.GetJavaSdk(),
	}
}

// FromTaskIDBlock converts internal TaskIDBlock type to thrift
func FromTaskIDBlock(t *types.TaskIDBlock) *shared.TaskIDBlock {
	if t == nil {
		return nil
	}
	return &shared.TaskIDBlock{
		StartID: &t.StartID,
		EndID:   &t.EndID,
	}
}

// ToTaskIDBlock converts thrift TaskIDBlock type to internal
func ToTaskIDBlock(t *shared.TaskIDBlock) *types.TaskIDBlock {
	if t == nil {
		return nil
	}
	return &types.TaskIDBlock{
		StartID: t.GetStartID(),
		EndID:   t.GetEndID(),
	}
}

// FromTaskList converts internal TaskList type to thrift
func FromTaskList(t *types.TaskList) *shared.TaskList {
	if t == nil {
		return nil
	}
	return &shared.TaskList{
		Name: &t.Name,
		Kind: FromTaskListKind(t.Kind),
	}
}

func MigrateTaskList(name string, t *shared.TaskList) *types.TaskList {
	if t == nil && name != "" {
		return &types.TaskList{Name: name, Kind: types.TaskListKindNormal.Ptr()}
	}
	return ToTaskList(t)
}

// ToTaskList converts thrift TaskList type to internal
func ToTaskList(t *shared.TaskList) *types.TaskList {
	if t == nil {
		return nil
	}
	return &types.TaskList{
		Name: t.GetName(),
		Kind: ToTaskListKind(t.Kind),
	}
}

// FromTaskListKind converts internal TaskListKind type to thrift
func FromTaskListKind(t *types.TaskListKind) *shared.TaskListKind {
	if t == nil {
		return nil
	}
	switch *t {
	case types.TaskListKindNormal:
		return shared.TaskListKindNormal.Ptr()
	case types.TaskListKindSticky:
		return shared.TaskListKindSticky.Ptr()
	case types.TaskListKindEphemeral:
		return shared.TaskListKindEphemeral.Ptr()
	}
	panic("unexpected enum value")
}

// ToTaskListKind converts thrift TaskListKind type to internal
func ToTaskListKind(t *shared.TaskListKind) *types.TaskListKind {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.TaskListKindNormal:
		return types.TaskListKindNormal.Ptr()
	case shared.TaskListKindSticky:
		return types.TaskListKindSticky.Ptr()
	case shared.TaskListKindEphemeral:
		return types.TaskListKindEphemeral.Ptr()
	}
	panic("unexpected enum value")
}

// FromTaskListMetadata converts internal TaskListMetadata type to thrift
func FromTaskListMetadata(t *types.TaskListMetadata) *shared.TaskListMetadata {
	if t == nil {
		return nil
	}
	return &shared.TaskListMetadata{
		MaxTasksPerSecond: t.MaxTasksPerSecond,
	}
}

// ToTaskListMetadata converts thrift TaskListMetadata type to internal
func ToTaskListMetadata(t *shared.TaskListMetadata) *types.TaskListMetadata {
	if t == nil {
		return nil
	}
	return &types.TaskListMetadata{
		MaxTasksPerSecond: t.MaxTasksPerSecond,
	}
}

// FromTaskListPartitionMetadata converts internal TaskListPartitionMetadata type to thrift
func FromTaskListPartitionMetadata(t *types.TaskListPartitionMetadata) *shared.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	return &shared.TaskListPartitionMetadata{
		Key:           &t.Key,
		OwnerHostName: &t.OwnerHostName,
	}
}

// ToTaskListPartitionMetadata converts thrift TaskListPartitionMetadata type to internal
func ToTaskListPartitionMetadata(t *shared.TaskListPartitionMetadata) *types.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	return &types.TaskListPartitionMetadata{
		Key:           t.GetKey(),
		OwnerHostName: t.GetOwnerHostName(),
	}
}

// FromTaskListStatus converts internal TaskListStatus type to thrift
func FromTaskListStatus(t *types.TaskListStatus) *shared.TaskListStatus {
	if t == nil {
		return nil
	}
	return &shared.TaskListStatus{
		BacklogCountHint:      &t.BacklogCountHint,
		ReadLevel:             &t.ReadLevel,
		AckLevel:              &t.AckLevel,
		RatePerSecond:         &t.RatePerSecond,
		TaskIDBlock:           FromTaskIDBlock(t.TaskIDBlock),
		IsolationGroupMetrics: FromIsolationGroupMetricsMap(t.IsolationGroupMetrics),
		NewTasksPerSecond:     &t.NewTasksPerSecond,
		Empty:                 &t.Empty,
	}
}

// ToTaskListStatus converts thrift TaskListStatus type to internal
func ToTaskListStatus(t *shared.TaskListStatus) *types.TaskListStatus {
	if t == nil {
		return nil
	}
	return &types.TaskListStatus{
		BacklogCountHint:      t.GetBacklogCountHint(),
		ReadLevel:             t.GetReadLevel(),
		AckLevel:              t.GetAckLevel(),
		RatePerSecond:         t.GetRatePerSecond(),
		TaskIDBlock:           ToTaskIDBlock(t.TaskIDBlock),
		IsolationGroupMetrics: ToIsolationGroupMetricsMap(t.GetIsolationGroupMetrics()),
		NewTasksPerSecond:     t.GetNewTasksPerSecond(),
		Empty:                 t.GetEmpty(),
	}
}

func FromIsolationGroupMetrics(t *types.IsolationGroupMetrics) *shared.IsolationGroupMetrics {
	if t == nil {
		return nil
	}
	return &shared.IsolationGroupMetrics{
		NewTasksPerSecond: &t.NewTasksPerSecond,
		PollerCount:       &t.PollerCount,
	}
}

func ToIsolationGroupMetrics(t *shared.IsolationGroupMetrics) *types.IsolationGroupMetrics {
	return &types.IsolationGroupMetrics{
		NewTasksPerSecond: t.GetNewTasksPerSecond(),
		PollerCount:       t.GetPollerCount(),
	}
}

// FromTaskListType converts internal TaskListType type to thrift
func FromTaskListType(t *types.TaskListType) *shared.TaskListType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.TaskListTypeDecision:
		v := shared.TaskListTypeDecision
		return &v
	case types.TaskListTypeActivity:
		v := shared.TaskListTypeActivity
		return &v
	}
	panic("unexpected enum value")
}

// ToTaskListType converts thrift TaskListType type to internal
func ToTaskListType(t *shared.TaskListType) *types.TaskListType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.TaskListTypeDecision:
		v := types.TaskListTypeDecision
		return &v
	case shared.TaskListTypeActivity:
		v := types.TaskListTypeActivity
		return &v
	}
	panic("unexpected enum value")
}

// ToRestartWorkflowExecutionResponse converts thrift RestartWorkflowExecutionResponse type to internal
func ToRestartWorkflowExecutionResponse(t *shared.RestartWorkflowExecutionResponse) *types.RestartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.RestartWorkflowExecutionResponse{
		RunID: t.GetRunId(),
	}
}

// FromTerminateWorkflowExecutionRequest converts internal TerminateWorkflowExecutionRequest type to thrift
func FromTerminateWorkflowExecutionRequest(t *types.TerminateWorkflowExecutionRequest) *shared.TerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &shared.TerminateWorkflowExecutionRequest{
		Domain:              &t.Domain,
		WorkflowExecution:   FromWorkflowExecution(t.WorkflowExecution),
		Reason:              &t.Reason,
		Details:             t.Details,
		Identity:            &t.Identity,
		FirstExecutionRunID: &t.FirstExecutionRunID,
	}
}

// ToTerminateWorkflowExecutionRequest converts thrift TerminateWorkflowExecutionRequest type to internal
func ToTerminateWorkflowExecutionRequest(t *shared.TerminateWorkflowExecutionRequest) *types.TerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.TerminateWorkflowExecutionRequest{
		Domain:              t.GetDomain(),
		WorkflowExecution:   ToWorkflowExecution(t.WorkflowExecution),
		Reason:              t.GetReason(),
		Details:             t.Details,
		Identity:            t.GetIdentity(),
		FirstExecutionRunID: t.GetFirstExecutionRunID(),
	}
}

// FromTimeoutType converts internal TimeoutType type to thrift
func FromTimeoutType(t *types.TimeoutType) *shared.TimeoutType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.TimeoutTypeStartToClose:
		v := shared.TimeoutTypeStartToClose
		return &v
	case types.TimeoutTypeScheduleToStart:
		v := shared.TimeoutTypeScheduleToStart
		return &v
	case types.TimeoutTypeScheduleToClose:
		v := shared.TimeoutTypeScheduleToClose
		return &v
	case types.TimeoutTypeHeartbeat:
		v := shared.TimeoutTypeHeartbeat
		return &v
	}
	panic("unexpected enum value")
}

// ToTimeoutType converts thrift TimeoutType type to internal
func ToTimeoutType(t *shared.TimeoutType) *types.TimeoutType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.TimeoutTypeStartToClose:
		v := types.TimeoutTypeStartToClose
		return &v
	case shared.TimeoutTypeScheduleToStart:
		v := types.TimeoutTypeScheduleToStart
		return &v
	case shared.TimeoutTypeScheduleToClose:
		v := types.TimeoutTypeScheduleToClose
		return &v
	case shared.TimeoutTypeHeartbeat:
		v := types.TimeoutTypeHeartbeat
		return &v
	}
	panic("unexpected enum value")
}

// FromTimerCanceledEventAttributes converts internal TimerCanceledEventAttributes type to thrift
func FromTimerCanceledEventAttributes(t *types.TimerCanceledEventAttributes) *shared.TimerCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.TimerCanceledEventAttributes{
		TimerId:                      &t.TimerID,
		StartedEventId:               &t.StartedEventID,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Identity:                     &t.Identity,
	}
}

// ToTimerCanceledEventAttributes converts thrift TimerCanceledEventAttributes type to internal
func ToTimerCanceledEventAttributes(t *shared.TimerCanceledEventAttributes) *types.TimerCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerCanceledEventAttributes{
		TimerID:                      t.GetTimerId(),
		StartedEventID:               t.GetStartedEventId(),
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Identity:                     t.GetIdentity(),
	}
}

// FromTimerFiredEventAttributes converts internal TimerFiredEventAttributes type to thrift
func FromTimerFiredEventAttributes(t *types.TimerFiredEventAttributes) *shared.TimerFiredEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.TimerFiredEventAttributes{
		TimerId:        &t.TimerID,
		StartedEventId: &t.StartedEventID,
	}
}

// ToTimerFiredEventAttributes converts thrift TimerFiredEventAttributes type to internal
func ToTimerFiredEventAttributes(t *shared.TimerFiredEventAttributes) *types.TimerFiredEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerFiredEventAttributes{
		TimerID:        t.GetTimerId(),
		StartedEventID: t.GetStartedEventId(),
	}
}

// FromTimerStartedEventAttributes converts internal TimerStartedEventAttributes type to thrift
func FromTimerStartedEventAttributes(t *types.TimerStartedEventAttributes) *shared.TimerStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.TimerStartedEventAttributes{
		TimerId:                      &t.TimerID,
		StartToFireTimeoutSeconds:    t.StartToFireTimeoutSeconds,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToTimerStartedEventAttributes converts thrift TimerStartedEventAttributes type to internal
func ToTimerStartedEventAttributes(t *shared.TimerStartedEventAttributes) *types.TimerStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerStartedEventAttributes{
		TimerID:                      t.GetTimerId(),
		StartToFireTimeoutSeconds:    t.StartToFireTimeoutSeconds,
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromTransientDecisionInfo converts internal TransientDecisionInfo type to thrift
func FromTransientDecisionInfo(t *types.TransientDecisionInfo) *shared.TransientDecisionInfo {
	if t == nil {
		return nil
	}
	return &shared.TransientDecisionInfo{
		ScheduledEvent: FromHistoryEvent(t.ScheduledEvent),
		StartedEvent:   FromHistoryEvent(t.StartedEvent),
	}
}

// ToTransientDecisionInfo converts thrift TransientDecisionInfo type to internal
func ToTransientDecisionInfo(t *shared.TransientDecisionInfo) *types.TransientDecisionInfo {
	if t == nil {
		return nil
	}
	return &types.TransientDecisionInfo{
		ScheduledEvent: ToHistoryEvent(t.ScheduledEvent),
		StartedEvent:   ToHistoryEvent(t.StartedEvent),
	}
}

// FromUpdateDomainRequest converts internal UpdateDomainRequest type to thrift
func FromUpdateDomainRequest(t *types.UpdateDomainRequest) *shared.UpdateDomainRequest {
	if t == nil {
		return nil
	}
	request := shared.UpdateDomainRequest{
		Name:                     &t.Name,
		SecurityToken:            &t.SecurityToken,
		DeleteBadBinary:          t.DeleteBadBinary,
		FailoverTimeoutInSeconds: t.FailoverTimeoutInSeconds,
	}
	if t.Description != nil || t.OwnerEmail != nil || t.Data != nil {
		request.UpdatedInfo = &shared.UpdateDomainInfo{
			Description: t.Description,
			OwnerEmail:  t.OwnerEmail,
			Data:        t.Data,
		}
	}
	if t.WorkflowExecutionRetentionPeriodInDays != nil ||
		t.EmitMetric != nil ||
		t.BadBinaries != nil ||
		t.HistoryArchivalStatus != nil ||
		t.HistoryArchivalURI != nil ||
		t.VisibilityArchivalStatus != nil ||
		t.VisibilityArchivalURI != nil {
		request.Configuration = &shared.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: t.WorkflowExecutionRetentionPeriodInDays,
			EmitMetric:                             t.EmitMetric,
			BadBinaries:                            FromBadBinaries(t.BadBinaries),
			HistoryArchivalStatus:                  FromArchivalStatus(t.HistoryArchivalStatus),
			HistoryArchivalURI:                     t.HistoryArchivalURI,
			VisibilityArchivalStatus:               FromArchivalStatus(t.VisibilityArchivalStatus),
			VisibilityArchivalURI:                  t.VisibilityArchivalURI,
		}
	}
	if t.ActiveClusterName != nil || t.Clusters != nil || t.ActiveClusters != nil {
		request.ReplicationConfiguration = &shared.DomainReplicationConfiguration{
			ActiveClusterName: t.ActiveClusterName,
			Clusters:          FromClusterReplicationConfigurationArray(t.Clusters),
			ActiveClusters:    FromActiveClusters(t.ActiveClusters),
		}
	}
	return &request
}

// ToUpdateDomainRequest converts thrift UpdateDomainRequest type to internal
func ToUpdateDomainRequest(t *shared.UpdateDomainRequest) *types.UpdateDomainRequest {
	if t == nil {
		return nil
	}
	request := types.UpdateDomainRequest{
		Name:                     t.GetName(),
		SecurityToken:            t.GetSecurityToken(),
		DeleteBadBinary:          t.DeleteBadBinary,
		FailoverTimeoutInSeconds: t.FailoverTimeoutInSeconds,
	}
	if t.UpdatedInfo != nil {
		request.Description = t.UpdatedInfo.Description
		request.OwnerEmail = t.UpdatedInfo.OwnerEmail
		request.Data = t.UpdatedInfo.Data
	}
	if t.Configuration != nil {
		request.WorkflowExecutionRetentionPeriodInDays = t.Configuration.WorkflowExecutionRetentionPeriodInDays
		request.EmitMetric = t.Configuration.EmitMetric
		request.BadBinaries = ToBadBinaries(t.Configuration.BadBinaries)
		request.HistoryArchivalStatus = ToArchivalStatus(t.Configuration.HistoryArchivalStatus)
		request.HistoryArchivalURI = t.Configuration.HistoryArchivalURI
		request.VisibilityArchivalStatus = ToArchivalStatus(t.Configuration.VisibilityArchivalStatus)
		request.VisibilityArchivalURI = t.Configuration.VisibilityArchivalURI
	}
	if t.ReplicationConfiguration != nil {
		request.ActiveClusterName = t.ReplicationConfiguration.ActiveClusterName
		request.Clusters = ToClusterReplicationConfigurationArray(t.ReplicationConfiguration.Clusters)
		request.ActiveClusters = ToActiveClusters(t.ReplicationConfiguration.ActiveClusters)
	}
	return &request
}

// FromUpdateDomainResponse converts internal UpdateDomainResponse type to thrift
func FromUpdateDomainResponse(t *types.UpdateDomainResponse) *shared.UpdateDomainResponse {
	if t == nil {
		return nil
	}
	return &shared.UpdateDomainResponse{
		DomainInfo:               FromDomainInfo(t.DomainInfo),
		Configuration:            FromDomainConfiguration(t.Configuration),
		ReplicationConfiguration: FromDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          &t.FailoverVersion,
		IsGlobalDomain:           &t.IsGlobalDomain,
	}
}

// ToUpdateDomainResponse converts thrift UpdateDomainResponse type to internal
func ToUpdateDomainResponse(t *shared.UpdateDomainResponse) *types.UpdateDomainResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateDomainResponse{
		DomainInfo:               ToDomainInfo(t.DomainInfo),
		Configuration:            ToDomainConfiguration(t.Configuration),
		ReplicationConfiguration: ToDomainReplicationConfiguration(t.ReplicationConfiguration),
		FailoverVersion:          t.GetFailoverVersion(),
		IsGlobalDomain:           t.GetIsGlobalDomain(),
	}
}

// FromUpsertWorkflowSearchAttributesDecisionAttributes converts internal UpsertWorkflowSearchAttributesDecisionAttributes type to thrift
func FromUpsertWorkflowSearchAttributesDecisionAttributes(t *types.UpsertWorkflowSearchAttributesDecisionAttributes) *shared.UpsertWorkflowSearchAttributesDecisionAttributes {
	if t == nil {
		return nil
	}
	return &shared.UpsertWorkflowSearchAttributesDecisionAttributes{
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

// ToUpsertWorkflowSearchAttributesDecisionAttributes converts thrift UpsertWorkflowSearchAttributesDecisionAttributes type to internal
func ToUpsertWorkflowSearchAttributesDecisionAttributes(t *shared.UpsertWorkflowSearchAttributesDecisionAttributes) *types.UpsertWorkflowSearchAttributesDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.UpsertWorkflowSearchAttributesDecisionAttributes{
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

// FromUpsertWorkflowSearchAttributesEventAttributes converts internal UpsertWorkflowSearchAttributesEventAttributes type to thrift
func FromUpsertWorkflowSearchAttributesEventAttributes(t *types.UpsertWorkflowSearchAttributesEventAttributes) *shared.UpsertWorkflowSearchAttributesEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
	}
}

// ToUpsertWorkflowSearchAttributesEventAttributes converts thrift UpsertWorkflowSearchAttributesEventAttributes type to internal
func ToUpsertWorkflowSearchAttributesEventAttributes(t *shared.UpsertWorkflowSearchAttributesEventAttributes) *types.UpsertWorkflowSearchAttributesEventAttributes {
	if t == nil {
		return nil
	}
	return &types.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		SearchAttributes:             ToSearchAttributes(t.SearchAttributes),
	}
}

// FromVersionHistories converts internal VersionHistories type to thrift
func FromVersionHistories(t *types.VersionHistories) *shared.VersionHistories {
	if t == nil {
		return nil
	}
	return &shared.VersionHistories{
		CurrentVersionHistoryIndex: &t.CurrentVersionHistoryIndex,
		Histories:                  FromVersionHistoryArray(t.Histories),
	}
}

// ToVersionHistories converts thrift VersionHistories type to internal
func ToVersionHistories(t *shared.VersionHistories) *types.VersionHistories {
	if t == nil {
		return nil
	}
	return &types.VersionHistories{
		CurrentVersionHistoryIndex: t.GetCurrentVersionHistoryIndex(),
		Histories:                  ToVersionHistoryArray(t.Histories),
	}
}

// FromVersionHistory converts internal VersionHistory type to thrift
func FromVersionHistory(t *types.VersionHistory) *shared.VersionHistory {
	if t == nil {
		return nil
	}
	return &shared.VersionHistory{
		BranchToken: t.BranchToken,
		Items:       FromVersionHistoryItemArray(t.Items),
	}
}

// ToVersionHistory converts thrift VersionHistory type to internal
func ToVersionHistory(t *shared.VersionHistory) *types.VersionHistory {
	if t == nil {
		return nil
	}
	return &types.VersionHistory{
		BranchToken: t.BranchToken,
		Items:       ToVersionHistoryItemArray(t.Items),
	}
}

// FromVersionHistoryItem converts internal VersionHistoryItem type to thrift
func FromVersionHistoryItem(t *types.VersionHistoryItem) *shared.VersionHistoryItem {
	if t == nil {
		return nil
	}
	return &shared.VersionHistoryItem{
		EventID: &t.EventID,
		Version: &t.Version,
	}
}

// ToVersionHistoryItem converts thrift VersionHistoryItem type to internal
func ToVersionHistoryItem(t *shared.VersionHistoryItem) *types.VersionHistoryItem {
	if t == nil {
		return nil
	}
	return &types.VersionHistoryItem{
		EventID: t.GetEventID(),
		Version: t.GetVersion(),
	}
}

// FromWorkerVersionInfo converts internal WorkerVersionInfo type to thrift
func FromWorkerVersionInfo(t *types.WorkerVersionInfo) *shared.WorkerVersionInfo {
	if t == nil {
		return nil
	}
	return &shared.WorkerVersionInfo{
		Impl:           &t.Impl,
		FeatureVersion: &t.FeatureVersion,
	}
}

// ToWorkerVersionInfo converts thrift WorkerVersionInfo type to internal
func ToWorkerVersionInfo(t *shared.WorkerVersionInfo) *types.WorkerVersionInfo {
	if t == nil {
		return nil
	}
	return &types.WorkerVersionInfo{
		Impl:           t.GetImpl(),
		FeatureVersion: t.GetFeatureVersion(),
	}
}

// FromWorkflowExecution converts internal WorkflowExecution type to thrift
func FromWorkflowExecution(t *types.WorkflowExecution) *shared.WorkflowExecution {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecution{
		WorkflowId: &t.WorkflowID,
		RunId:      &t.RunID,
	}
}

// ToWorkflowExecution converts thrift WorkflowExecution type to internal
func ToWorkflowExecution(t *shared.WorkflowExecution) *types.WorkflowExecution {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecution{
		WorkflowID: t.GetWorkflowId(),
		RunID:      t.GetRunId(),
	}
}

// FromWorkflowExecutionAlreadyStartedError converts internal WorkflowExecutionAlreadyStartedError type to thrift
func FromWorkflowExecutionAlreadyStartedError(t *types.WorkflowExecutionAlreadyStartedError) *shared.WorkflowExecutionAlreadyStartedError {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionAlreadyStartedError{
		Message:        &t.Message,
		StartRequestId: &t.StartRequestID,
		RunId:          &t.RunID,
	}
}

// ToWorkflowExecutionAlreadyStartedError converts thrift WorkflowExecutionAlreadyStartedError type to internal
func ToWorkflowExecutionAlreadyStartedError(t *shared.WorkflowExecutionAlreadyStartedError) *types.WorkflowExecutionAlreadyStartedError {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionAlreadyStartedError{
		Message:        t.GetMessage(),
		StartRequestID: t.GetStartRequestId(),
		RunID:          t.GetRunId(),
	}
}

// FromWorkflowExecutionCancelRequestedEventAttributes converts internal WorkflowExecutionCancelRequestedEventAttributes type to thrift
func FromWorkflowExecutionCancelRequestedEventAttributes(t *types.WorkflowExecutionCancelRequestedEventAttributes) *shared.WorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                     &t.Cause,
		ExternalInitiatedEventId:  t.ExternalInitiatedEventID,
		ExternalWorkflowExecution: FromWorkflowExecution(t.ExternalWorkflowExecution),
		Identity:                  &t.Identity,
		RequestId:                 &t.RequestID,
	}
}

// ToWorkflowExecutionCancelRequestedEventAttributes converts thrift WorkflowExecutionCancelRequestedEventAttributes type to internal
func ToWorkflowExecutionCancelRequestedEventAttributes(t *shared.WorkflowExecutionCancelRequestedEventAttributes) *types.WorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                     t.GetCause(),
		ExternalInitiatedEventID:  t.ExternalInitiatedEventId,
		ExternalWorkflowExecution: ToWorkflowExecution(t.ExternalWorkflowExecution),
		Identity:                  t.GetIdentity(),
		RequestID:                 t.GetRequestId(),
	}
}

// FromWorkflowExecutionCanceledEventAttributes converts internal WorkflowExecutionCanceledEventAttributes type to thrift
func FromWorkflowExecutionCanceledEventAttributes(t *types.WorkflowExecutionCanceledEventAttributes) *shared.WorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
		Details:                      t.Details,
	}
}

// ToWorkflowExecutionCanceledEventAttributes converts thrift WorkflowExecutionCanceledEventAttributes type to internal
func ToWorkflowExecutionCanceledEventAttributes(t *shared.WorkflowExecutionCanceledEventAttributes) *types.WorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
		Details:                      t.Details,
	}
}

// FromWorkflowExecutionCloseStatus converts internal WorkflowExecutionCloseStatus type to thrift
func FromWorkflowExecutionCloseStatus(t *types.WorkflowExecutionCloseStatus) *shared.WorkflowExecutionCloseStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case types.WorkflowExecutionCloseStatusCompleted:
		v := shared.WorkflowExecutionCloseStatusCompleted
		return &v
	case types.WorkflowExecutionCloseStatusFailed:
		v := shared.WorkflowExecutionCloseStatusFailed
		return &v
	case types.WorkflowExecutionCloseStatusCanceled:
		v := shared.WorkflowExecutionCloseStatusCanceled
		return &v
	case types.WorkflowExecutionCloseStatusTerminated:
		v := shared.WorkflowExecutionCloseStatusTerminated
		return &v
	case types.WorkflowExecutionCloseStatusContinuedAsNew:
		v := shared.WorkflowExecutionCloseStatusContinuedAsNew
		return &v
	case types.WorkflowExecutionCloseStatusTimedOut:
		v := shared.WorkflowExecutionCloseStatusTimedOut
		return &v
	}
	panic("unexpected enum value")
}

// ToWorkflowExecutionCloseStatus converts thrift WorkflowExecutionCloseStatus type to internal
func ToWorkflowExecutionCloseStatus(t *shared.WorkflowExecutionCloseStatus) *types.WorkflowExecutionCloseStatus {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.WorkflowExecutionCloseStatusCompleted:
		v := types.WorkflowExecutionCloseStatusCompleted
		return &v
	case shared.WorkflowExecutionCloseStatusFailed:
		v := types.WorkflowExecutionCloseStatusFailed
		return &v
	case shared.WorkflowExecutionCloseStatusCanceled:
		v := types.WorkflowExecutionCloseStatusCanceled
		return &v
	case shared.WorkflowExecutionCloseStatusTerminated:
		v := types.WorkflowExecutionCloseStatusTerminated
		return &v
	case shared.WorkflowExecutionCloseStatusContinuedAsNew:
		v := types.WorkflowExecutionCloseStatusContinuedAsNew
		return &v
	case shared.WorkflowExecutionCloseStatusTimedOut:
		v := types.WorkflowExecutionCloseStatusTimedOut
		return &v
	}
	panic("unexpected enum value")
}

// FromWorkflowExecutionCompletedEventAttributes converts internal WorkflowExecutionCompletedEventAttributes type to thrift
func FromWorkflowExecutionCompletedEventAttributes(t *types.WorkflowExecutionCompletedEventAttributes) *shared.WorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionCompletedEventAttributes{
		Result:                       t.Result,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToWorkflowExecutionCompletedEventAttributes converts thrift WorkflowExecutionCompletedEventAttributes type to internal
func ToWorkflowExecutionCompletedEventAttributes(t *shared.WorkflowExecutionCompletedEventAttributes) *types.WorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCompletedEventAttributes{
		Result:                       t.Result,
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromWorkflowExecutionConfiguration converts internal WorkflowExecutionConfiguration type to thrift
func FromWorkflowExecutionConfiguration(t *types.WorkflowExecutionConfiguration) *shared.WorkflowExecutionConfiguration {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionConfiguration{
		TaskList:                            FromTaskList(t.TaskList),
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
	}
}

// ToWorkflowExecutionConfiguration converts thrift WorkflowExecutionConfiguration type to internal
func ToWorkflowExecutionConfiguration(t *shared.WorkflowExecutionConfiguration) *types.WorkflowExecutionConfiguration {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionConfiguration{
		TaskList:                            ToTaskList(t.TaskList),
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
	}
}

// FromWorkflowExecutionContinuedAsNewEventAttributes converts internal WorkflowExecutionContinuedAsNewEventAttributes type to thrift
func FromWorkflowExecutionContinuedAsNewEventAttributes(t *types.WorkflowExecutionContinuedAsNewEventAttributes) *shared.WorkflowExecutionContinuedAsNewEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunId:                   &t.NewExecutionRunID,
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		DecisionTaskCompletedEventId:        &t.DecisionTaskCompletedEventID,
		BackoffStartIntervalInSeconds:       t.BackoffStartIntervalInSeconds,
		Initiator:                           FromContinueAsNewInitiator(t.Initiator),
		FailureReason:                       t.FailureReason,
		FailureDetails:                      t.FailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		Header:                              FromHeader(t.Header),
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

// ToWorkflowExecutionContinuedAsNewEventAttributes converts thrift WorkflowExecutionContinuedAsNewEventAttributes type to internal
func ToWorkflowExecutionContinuedAsNewEventAttributes(t *shared.WorkflowExecutionContinuedAsNewEventAttributes) *types.WorkflowExecutionContinuedAsNewEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunID:                   t.GetNewExecutionRunId(),
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		DecisionTaskCompletedEventID:        t.GetDecisionTaskCompletedEventId(),
		BackoffStartIntervalInSeconds:       t.BackoffStartIntervalInSeconds,
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		FailureReason:                       t.FailureReason,
		FailureDetails:                      t.FailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

// FromWorkflowExecutionFailedEventAttributes converts internal WorkflowExecutionFailedEventAttributes type to thrift
func FromWorkflowExecutionFailedEventAttributes(t *types.WorkflowExecutionFailedEventAttributes) *shared.WorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionFailedEventAttributes{
		Reason:                       t.Reason,
		Details:                      t.Details,
		DecisionTaskCompletedEventId: &t.DecisionTaskCompletedEventID,
	}
}

// ToWorkflowExecutionFailedEventAttributes converts thrift WorkflowExecutionFailedEventAttributes type to internal
func ToWorkflowExecutionFailedEventAttributes(t *shared.WorkflowExecutionFailedEventAttributes) *types.WorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionFailedEventAttributes{
		Reason:                       t.Reason,
		Details:                      t.Details,
		DecisionTaskCompletedEventID: t.GetDecisionTaskCompletedEventId(),
	}
}

// FromWorkflowExecutionFilter converts internal WorkflowExecutionFilter type to thrift
func FromWorkflowExecutionFilter(t *types.WorkflowExecutionFilter) *shared.WorkflowExecutionFilter {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionFilter{
		WorkflowId: &t.WorkflowID,
		RunId:      &t.RunID,
	}
}

// ToWorkflowExecutionFilter converts thrift WorkflowExecutionFilter type to internal
func ToWorkflowExecutionFilter(t *shared.WorkflowExecutionFilter) *types.WorkflowExecutionFilter {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionFilter{
		WorkflowID: t.GetWorkflowId(),
		RunID:      t.GetRunId(),
	}
}

// FromWorkflowExecutionInfo converts internal WorkflowExecutionInfo type to thrift
func FromWorkflowExecutionInfo(t *types.WorkflowExecutionInfo) *shared.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	tlName := ""
	if t.TaskList != nil {
		tlName = t.TaskList.Name
	}
	return &shared.WorkflowExecutionInfo{
		Execution:                    FromWorkflowExecution(t.Execution),
		Type:                         FromWorkflowType(t.Type),
		StartTime:                    t.StartTime,
		CloseTime:                    t.CloseTime,
		CloseStatus:                  FromWorkflowExecutionCloseStatus(t.CloseStatus),
		HistoryLength:                &t.HistoryLength,
		ParentDomainId:               t.ParentDomainID,
		ParentDomainName:             t.ParentDomain,
		ParentInitatedId:             t.ParentInitiatedID,
		ParentExecution:              FromWorkflowExecution(t.ParentExecution),
		ExecutionTime:                t.ExecutionTime,
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
		AutoResetPoints:              FromResetPoints(t.AutoResetPoints),
		TaskList:                     &tlName,
		TaskListInfo:                 FromTaskList(t.TaskList),
		IsCron:                       &t.IsCron,
		UpdateTime:                   t.UpdateTime,
		PartitionConfig:              t.PartitionConfig,
		CronOverlapPolicy:            FromCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy: FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// ToWorkflowExecutionInfo converts thrift WorkflowExecutionInfo type to internal
func ToWorkflowExecutionInfo(t *shared.WorkflowExecutionInfo) *types.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionInfo{
		Execution:                    ToWorkflowExecution(t.Execution),
		Type:                         ToWorkflowType(t.Type),
		StartTime:                    t.StartTime,
		CloseTime:                    t.CloseTime,
		CloseStatus:                  ToWorkflowExecutionCloseStatus(t.CloseStatus),
		HistoryLength:                t.GetHistoryLength(),
		ParentDomainID:               t.ParentDomainId,
		ParentDomain:                 t.ParentDomainName,
		ParentInitiatedID:            t.ParentInitatedId,
		ParentExecution:              ToWorkflowExecution(t.ParentExecution),
		ExecutionTime:                t.ExecutionTime,
		Memo:                         ToMemo(t.Memo),
		SearchAttributes:             ToSearchAttributes(t.SearchAttributes),
		AutoResetPoints:              ToResetPoints(t.AutoResetPoints),
		TaskList:                     MigrateTaskList(t.GetTaskList(), t.TaskListInfo),
		IsCron:                       t.GetIsCron(),
		UpdateTime:                   t.UpdateTime,
		PartitionConfig:              t.PartitionConfig,
		CronOverlapPolicy:            ToCronOverlapPolicy(t.CronOverlapPolicy),
		ActiveClusterSelectionPolicy: ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
	}
}

// FromWorkflowExecutionSignaledEventAttributes converts internal WorkflowExecutionSignaledEventAttributes type to thrift
func FromWorkflowExecutionSignaledEventAttributes(t *types.WorkflowExecutionSignaledEventAttributes) *shared.WorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionSignaledEventAttributes{
		SignalName: &t.SignalName,
		Input:      t.Input,
		Identity:   &t.Identity,
		RequestId:  &t.RequestID,
	}
}

// ToWorkflowExecutionSignaledEventAttributes converts thrift WorkflowExecutionSignaledEventAttributes type to internal
func ToWorkflowExecutionSignaledEventAttributes(t *shared.WorkflowExecutionSignaledEventAttributes) *types.WorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionSignaledEventAttributes{
		SignalName: t.GetSignalName(),
		Input:      t.Input,
		Identity:   t.GetIdentity(),
		RequestID:  t.GetRequestId(),
	}
}

// FromWorkflowExecutionStartedEventAttributes converts internal WorkflowExecutionStartedEventAttributes type to thrift
func FromWorkflowExecutionStartedEventAttributes(t *types.WorkflowExecutionStartedEventAttributes) *shared.WorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}

	return &shared.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		ParentWorkflowDomain:                t.ParentWorkflowDomain,
		ParentWorkflowExecution:             FromWorkflowExecution(t.ParentWorkflowExecution),
		ParentInitiatedEventId:              t.ParentInitiatedEventID,
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ContinuedExecutionRunId:             &t.ContinuedExecutionRunID,
		Initiator:                           FromContinueAsNewInitiator(t.Initiator),
		ContinuedFailureReason:              t.ContinuedFailureReason,
		ContinuedFailureDetails:             t.ContinuedFailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		OriginalExecutionRunId:              &t.OriginalExecutionRunID,
		Identity:                            &t.Identity,
		FirstExecutionRunId:                 &t.FirstExecutionRunID,
		FirstScheduledTimeNano:              timeToNano(t.FirstScheduleTime),
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		Attempt:                             &t.Attempt,
		ExpirationTimestamp:                 t.ExpirationTimestamp,
		CronSchedule:                        &t.CronSchedule,
		FirstDecisionTaskBackoffSeconds:     t.FirstDecisionTaskBackoffSeconds,
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
		PrevAutoResetPoints:                 FromResetPoints(t.PrevAutoResetPoints),
		Header:                              FromHeader(t.Header),
		PartitionConfig:                     t.PartitionConfig,
		RequestId:                           &t.RequestID,
		ActiveClusterSelectionPolicy:        FromActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   FromCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

// ToWorkflowExecutionStartedEventAttributes converts thrift WorkflowExecutionStartedEventAttributes type to internal
func ToWorkflowExecutionStartedEventAttributes(t *shared.WorkflowExecutionStartedEventAttributes) *types.WorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}

	return &types.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		ParentWorkflowDomain:                t.ParentWorkflowDomain,
		ParentWorkflowExecution:             ToWorkflowExecution(t.ParentWorkflowExecution),
		ParentInitiatedEventID:              t.ParentInitiatedEventId,
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		ContinuedExecutionRunID:             t.GetContinuedExecutionRunId(),
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		ContinuedFailureReason:              t.ContinuedFailureReason,
		ContinuedFailureDetails:             t.ContinuedFailureDetails,
		LastCompletionResult:                t.LastCompletionResult,
		OriginalExecutionRunID:              t.GetOriginalExecutionRunId(),
		Identity:                            t.GetIdentity(),
		FirstExecutionRunID:                 t.GetFirstExecutionRunId(),
		FirstScheduleTime:                   nanoToTime(t.FirstScheduledTimeNano),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Attempt:                             t.GetAttempt(),
		ExpirationTimestamp:                 t.ExpirationTimestamp,
		CronSchedule:                        t.GetCronSchedule(),
		FirstDecisionTaskBackoffSeconds:     t.FirstDecisionTaskBackoffSeconds,
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		PrevAutoResetPoints:                 ToResetPoints(t.PrevAutoResetPoints),
		Header:                              ToHeader(t.Header),
		PartitionConfig:                     t.PartitionConfig,
		RequestID:                           t.GetRequestId(),
		ActiveClusterSelectionPolicy:        ToActiveClusterSelectionPolicy(t.ActiveClusterSelectionPolicy),
		CronOverlapPolicy:                   ToCronOverlapPolicy(t.CronOverlapPolicy),
	}
}

func FromClusterAttribute(t *types.ClusterAttribute) *shared.ClusterAttribute {
	if t == nil {
		return nil
	}
	return &shared.ClusterAttribute{
		Scope: &t.Scope,
		Name:  &t.Name,
	}
}

func ToClusterAttribute(t *shared.ClusterAttribute) *types.ClusterAttribute {
	if t == nil {
		return nil
	}
	return &types.ClusterAttribute{
		Scope: t.GetScope(),
		Name:  t.GetName(),
	}
}

func FromActiveClusterSelectionPolicy(t *types.ActiveClusterSelectionPolicy) *shared.ActiveClusterSelectionPolicy {
	if t == nil {
		return nil
	}
	return &shared.ActiveClusterSelectionPolicy{
		Strategy:           FromActiveClusterSelectionStrategy(t.ActiveClusterSelectionStrategy),
		ExternalEntityType: &t.ExternalEntityType,
		ExternalEntityKey:  &t.ExternalEntityKey,
		StickyRegion:       &t.StickyRegion,
		ClusterAttribute:   FromClusterAttribute(t.ClusterAttribute),
	}
}

func ToActiveClusterSelectionPolicy(t *shared.ActiveClusterSelectionPolicy) *types.ActiveClusterSelectionPolicy {
	if t == nil {
		return nil
	}
	return &types.ActiveClusterSelectionPolicy{
		ActiveClusterSelectionStrategy: ToActiveClusterSelectionStrategy(t.Strategy),
		ExternalEntityType:             t.GetExternalEntityType(),
		ExternalEntityKey:              t.GetExternalEntityKey(),
		StickyRegion:                   t.GetStickyRegion(),
		ClusterAttribute:               ToClusterAttribute(t.ClusterAttribute),
	}
}

func FromActiveClusterSelectionStrategy(t *types.ActiveClusterSelectionStrategy) *shared.ActiveClusterSelectionStrategy {
	if t == nil {
		return nil
	}
	switch *t {
	case types.ActiveClusterSelectionStrategyRegionSticky:
		return shared.ActiveClusterSelectionStrategyRegionSticky.Ptr()
	case types.ActiveClusterSelectionStrategyExternalEntity:
		return shared.ActiveClusterSelectionStrategyExternalEntity.Ptr()
	}
	panic("unexpected enum value")
}

func ToActiveClusterSelectionStrategy(t *shared.ActiveClusterSelectionStrategy) *types.ActiveClusterSelectionStrategy {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.ActiveClusterSelectionStrategyRegionSticky:
		return types.ActiveClusterSelectionStrategyRegionSticky.Ptr()
	case shared.ActiveClusterSelectionStrategyExternalEntity:
		return types.ActiveClusterSelectionStrategyExternalEntity.Ptr()
	}
	panic("unexpected enum value")
}

// FromWorkflowExecutionTerminatedEventAttributes converts internal WorkflowExecutionTerminatedEventAttributes type to thrift
func FromWorkflowExecutionTerminatedEventAttributes(t *types.WorkflowExecutionTerminatedEventAttributes) *shared.WorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionTerminatedEventAttributes{
		Reason:   &t.Reason,
		Details:  t.Details,
		Identity: &t.Identity,
	}
}

// ToWorkflowExecutionTerminatedEventAttributes converts thrift WorkflowExecutionTerminatedEventAttributes type to internal
func ToWorkflowExecutionTerminatedEventAttributes(t *shared.WorkflowExecutionTerminatedEventAttributes) *types.WorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionTerminatedEventAttributes{
		Reason:   t.GetReason(),
		Details:  t.Details,
		Identity: t.GetIdentity(),
	}
}

// FromWorkflowExecutionTimedOutEventAttributes converts internal WorkflowExecutionTimedOutEventAttributes type to thrift
func FromWorkflowExecutionTimedOutEventAttributes(t *types.WorkflowExecutionTimedOutEventAttributes) *shared.WorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &shared.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: FromTimeoutType(t.TimeoutType),
	}
}

// ToWorkflowExecutionTimedOutEventAttributes converts thrift WorkflowExecutionTimedOutEventAttributes type to internal
func ToWorkflowExecutionTimedOutEventAttributes(t *shared.WorkflowExecutionTimedOutEventAttributes) *types.WorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: ToTimeoutType(t.TimeoutType),
	}
}

// FromWorkflowIDReusePolicy converts internal WorkflowIDReusePolicy type to thrift
func FromWorkflowIDReusePolicy(t *types.WorkflowIDReusePolicy) *shared.WorkflowIdReusePolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case types.WorkflowIDReusePolicyAllowDuplicateFailedOnly:
		v := shared.WorkflowIdReusePolicyAllowDuplicateFailedOnly
		return &v
	case types.WorkflowIDReusePolicyAllowDuplicate:
		v := shared.WorkflowIdReusePolicyAllowDuplicate
		return &v
	case types.WorkflowIDReusePolicyRejectDuplicate:
		v := shared.WorkflowIdReusePolicyRejectDuplicate
		return &v
	case types.WorkflowIDReusePolicyTerminateIfRunning:
		v := shared.WorkflowIdReusePolicyTerminateIfRunning
		return &v
	}
	panic("unexpected enum value")
}

// ToWorkflowIDReusePolicy converts thrift WorkflowIdReusePolicy type to internal
func ToWorkflowIDReusePolicy(t *shared.WorkflowIdReusePolicy) *types.WorkflowIDReusePolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.WorkflowIdReusePolicyAllowDuplicateFailedOnly:
		v := types.WorkflowIDReusePolicyAllowDuplicateFailedOnly
		return &v
	case shared.WorkflowIdReusePolicyAllowDuplicate:
		v := types.WorkflowIDReusePolicyAllowDuplicate
		return &v
	case shared.WorkflowIdReusePolicyRejectDuplicate:
		v := types.WorkflowIDReusePolicyRejectDuplicate
		return &v
	case shared.WorkflowIdReusePolicyTerminateIfRunning:
		v := types.WorkflowIDReusePolicyTerminateIfRunning
		return &v
	}
	panic("unexpected enum value")
}

// FromWorkflowQuery converts internal WorkflowQuery type to thrift
func FromWorkflowQuery(t *types.WorkflowQuery) *shared.WorkflowQuery {
	if t == nil {
		return nil
	}
	return &shared.WorkflowQuery{
		QueryType: &t.QueryType,
		QueryArgs: t.QueryArgs,
	}
}

// ToWorkflowQuery converts thrift WorkflowQuery type to internal
func ToWorkflowQuery(t *shared.WorkflowQuery) *types.WorkflowQuery {
	if t == nil {
		return nil
	}
	return &types.WorkflowQuery{
		QueryType: t.GetQueryType(),
		QueryArgs: t.QueryArgs,
	}
}

// FromWorkflowQueryResult converts internal WorkflowQueryResult type to thrift
func FromWorkflowQueryResult(t *types.WorkflowQueryResult) *shared.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	return &shared.WorkflowQueryResult{
		ResultType:   FromQueryResultType(t.ResultType),
		Answer:       t.Answer,
		ErrorMessage: &t.ErrorMessage,
	}
}

// ToWorkflowQueryResult converts thrift WorkflowQueryResult type to internal
func ToWorkflowQueryResult(t *shared.WorkflowQueryResult) *types.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	return &types.WorkflowQueryResult{
		ResultType:   ToQueryResultType(t.ResultType),
		Answer:       t.Answer,
		ErrorMessage: t.GetErrorMessage(),
	}
}

// FromWorkflowType converts internal WorkflowType type to thrift
func FromWorkflowType(t *types.WorkflowType) *shared.WorkflowType {
	if t == nil {
		return nil
	}
	return &shared.WorkflowType{
		Name: &t.Name,
	}
}

// ToWorkflowType converts thrift WorkflowType type to internal
func ToWorkflowType(t *shared.WorkflowType) *types.WorkflowType {
	if t == nil {
		return nil
	}
	return &types.WorkflowType{
		Name: t.GetName(),
	}
}

// FromWorkflowTypeFilter converts internal WorkflowTypeFilter type to thrift
func FromWorkflowTypeFilter(t *types.WorkflowTypeFilter) *shared.WorkflowTypeFilter {
	if t == nil {
		return nil
	}
	return &shared.WorkflowTypeFilter{
		Name: &t.Name,
	}
}

// ToWorkflowTypeFilter converts thrift WorkflowTypeFilter type to internal
func ToWorkflowTypeFilter(t *shared.WorkflowTypeFilter) *types.WorkflowTypeFilter {
	if t == nil {
		return nil
	}
	return &types.WorkflowTypeFilter{
		Name: t.GetName(),
	}
}

// FromPendingActivityInfoArray converts internal PendingActivityInfo type array to thrift
func FromPendingActivityInfoArray(t []*types.PendingActivityInfo) []*shared.PendingActivityInfo {
	if t == nil {
		return nil
	}
	v := make([]*shared.PendingActivityInfo, len(t))
	for i := range t {
		v[i] = FromPendingActivityInfo(t[i])
	}
	return v
}

// ToPendingActivityInfoArray converts thrift PendingActivityInfo type array to internal
func ToPendingActivityInfoArray(t []*shared.PendingActivityInfo) []*types.PendingActivityInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PendingActivityInfo, len(t))
	for i := range t {
		v[i] = ToPendingActivityInfo(t[i])
	}
	return v
}

// FromHistoryEventArray converts internal HistoryEvent type array to thrift
func FromHistoryEventArray(t []*types.HistoryEvent) []*shared.HistoryEvent {
	if t == nil {
		return nil
	}
	v := make([]*shared.HistoryEvent, len(t))
	for i := range t {
		v[i] = FromHistoryEvent(t[i])
	}
	return v
}

// ToHistoryEventArray converts thrift HistoryEvent type array to internal
func ToHistoryEventArray(t []*shared.HistoryEvent) []*types.HistoryEvent {
	if t == nil {
		return nil
	}
	v := make([]*types.HistoryEvent, len(t))
	for i := range t {
		v[i] = ToHistoryEvent(t[i])
	}
	return v
}

// FromWorkflowExecutionInfoArray converts internal WorkflowExecutionInfo type array to thrift
func FromWorkflowExecutionInfoArray(t []*types.WorkflowExecutionInfo) []*shared.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*shared.WorkflowExecutionInfo, len(t))
	for i := range t {
		v[i] = FromWorkflowExecutionInfo(t[i])
	}
	return v
}

// ToWorkflowExecutionInfoArray converts thrift WorkflowExecutionInfo type array to internal
func ToWorkflowExecutionInfoArray(t []*shared.WorkflowExecutionInfo) []*types.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.WorkflowExecutionInfo, len(t))
	for i := range t {
		v[i] = ToWorkflowExecutionInfo(t[i])
	}
	return v
}

// FromPollerInfoArray converts internal PollerInfo type array to thrift
func FromPollerInfoArray(t []*types.PollerInfo) []*shared.PollerInfo {
	if t == nil {
		return nil
	}
	v := make([]*shared.PollerInfo, len(t))
	for i := range t {
		v[i] = FromPollerInfo(t[i])
	}
	return v
}

// ToPollerInfoArray converts thrift PollerInfo type array to internal
func ToPollerInfoArray(t []*shared.PollerInfo) []*types.PollerInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PollerInfo, len(t))
	for i := range t {
		v[i] = ToPollerInfo(t[i])
	}
	return v
}

// FromClusterReplicationConfigurationArray converts internal ClusterReplicationConfiguration type array to thrift
func FromClusterReplicationConfigurationArray(t []*types.ClusterReplicationConfiguration) []*shared.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	v := make([]*shared.ClusterReplicationConfiguration, len(t))
	for i := range t {
		v[i] = FromClusterReplicationConfiguration(t[i])
	}
	return v
}

// ToClusterReplicationConfigurationArray converts thrift ClusterReplicationConfiguration type array to internal
func ToClusterReplicationConfigurationArray(t []*shared.ClusterReplicationConfiguration) []*types.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	v := make([]*types.ClusterReplicationConfiguration, len(t))
	for i := range t {
		v[i] = ToClusterReplicationConfiguration(t[i])
	}
	return v
}

// FromDataBlobArray converts internal DataBlob type array to thrift
func FromDataBlobArray(t []*types.DataBlob) []*shared.DataBlob {
	if t == nil {
		return nil
	}
	v := make([]*shared.DataBlob, len(t))
	for i := range t {
		v[i] = FromDataBlob(t[i])
	}
	return v
}

// ToDataBlobArray converts thrift DataBlob type array to internal
func ToDataBlobArray(t []*shared.DataBlob) []*types.DataBlob {
	if t == nil {
		return nil
	}
	v := make([]*types.DataBlob, len(t))
	for i := range t {
		v[i] = ToDataBlob(t[i])
	}
	return v
}

// FromDescribeDomainResponseArray converts internal DescribeDomainResponse type array to thrift
func FromDescribeDomainResponseArray(t []*types.DescribeDomainResponse) []*shared.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	v := make([]*shared.DescribeDomainResponse, len(t))
	for i := range t {
		v[i] = FromDescribeDomainResponse(t[i])
	}
	return v
}

// ToDescribeDomainResponseArray converts thrift DescribeDomainResponse type array to internal
func ToDescribeDomainResponseArray(t []*shared.DescribeDomainResponse) []*types.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	v := make([]*types.DescribeDomainResponse, len(t))
	for i := range t {
		v[i] = ToDescribeDomainResponse(t[i])
	}
	return v
}

// FromDecisionArray converts internal Decision type array to thrift
func FromDecisionArray(t []*types.Decision) []*shared.Decision {
	if t == nil {
		return nil
	}
	v := make([]*shared.Decision, len(t))
	for i := range t {
		v[i] = FromDecision(t[i])
	}
	return v
}

// ToDecisionArray converts thrift Decision type array to internal
func ToDecisionArray(t []*shared.Decision) []*types.Decision {
	if t == nil {
		return nil
	}
	v := make([]*types.Decision, len(t))
	for i := range t {
		v[i] = ToDecision(t[i])
	}
	return v
}

// FromVersionHistoryArray converts internal VersionHistory type array to thrift
func FromVersionHistoryArray(t []*types.VersionHistory) []*shared.VersionHistory {
	if t == nil {
		return nil
	}
	v := make([]*shared.VersionHistory, len(t))
	for i := range t {
		v[i] = FromVersionHistory(t[i])
	}
	return v
}

// ToVersionHistoryArray converts thrift VersionHistory type array to internal
func ToVersionHistoryArray(t []*shared.VersionHistory) []*types.VersionHistory {
	if t == nil {
		return nil
	}
	v := make([]*types.VersionHistory, len(t))
	for i := range t {
		v[i] = ToVersionHistory(t[i])
	}
	return v
}

// FromVersionHistoryItemArray converts internal VersionHistoryItem type array to thrift
func FromVersionHistoryItemArray(t []*types.VersionHistoryItem) []*shared.VersionHistoryItem {
	if t == nil {
		return nil
	}
	v := make([]*shared.VersionHistoryItem, len(t))
	for i := range t {
		v[i] = FromVersionHistoryItem(t[i])
	}
	return v
}

// ToVersionHistoryItemArray converts thrift VersionHistoryItem type array to internal
func ToVersionHistoryItemArray(t []*shared.VersionHistoryItem) []*types.VersionHistoryItem {
	if t == nil {
		return nil
	}
	v := make([]*types.VersionHistoryItem, len(t))
	for i := range t {
		v[i] = ToVersionHistoryItem(t[i])
	}
	return v
}

// FromTaskListPartitionMetadataArray converts internal TaskListPartitionMetadata type array to thrift
func FromTaskListPartitionMetadataArray(t []*types.TaskListPartitionMetadata) []*shared.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	v := make([]*shared.TaskListPartitionMetadata, len(t))
	for i := range t {
		v[i] = FromTaskListPartitionMetadata(t[i])
	}
	return v
}

// ToTaskListPartitionMetadataArray converts thrift TaskListPartitionMetadata type array to internal
func ToTaskListPartitionMetadataArray(t []*shared.TaskListPartitionMetadata) []*types.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	v := make([]*types.TaskListPartitionMetadata, len(t))
	for i := range t {
		v[i] = ToTaskListPartitionMetadata(t[i])
	}
	return v
}

// FromResetPointInfoArray converts internal ResetPointInfo type array to thrift
func FromResetPointInfoArray(t []*types.ResetPointInfo) []*shared.ResetPointInfo {
	if t == nil {
		return nil
	}
	v := make([]*shared.ResetPointInfo, len(t))
	for i := range t {
		v[i] = FromResetPointInfo(t[i])
	}
	return v
}

// ToResetPointInfoArray converts thrift ResetPointInfo type array to internal
func ToResetPointInfoArray(t []*shared.ResetPointInfo) []*types.ResetPointInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.ResetPointInfo, len(t))
	for i := range t {
		v[i] = ToResetPointInfo(t[i])
	}
	return v
}

// FromPendingChildExecutionInfoArray converts internal PendingChildExecutionInfo type array to thrift
func FromPendingChildExecutionInfoArray(t []*types.PendingChildExecutionInfo) []*shared.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*shared.PendingChildExecutionInfo, len(t))
	for i := range t {
		v[i] = FromPendingChildExecutionInfo(t[i])
	}
	return v
}

// ToPendingChildExecutionInfoArray converts thrift PendingChildExecutionInfo type array to internal
func ToPendingChildExecutionInfoArray(t []*shared.PendingChildExecutionInfo) []*types.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PendingChildExecutionInfo, len(t))
	for i := range t {
		v[i] = ToPendingChildExecutionInfo(t[i])
	}
	return v
}

// FromHistoryBranchRangeArray converts internal HistoryBranchRange type array to thrift
func FromHistoryBranchRangeArray(t []*types.HistoryBranchRange) []*shared.HistoryBranchRange {
	if t == nil {
		return nil
	}
	v := make([]*shared.HistoryBranchRange, len(t))
	for i := range t {
		v[i] = FromHistoryBranchRange(t[i])
	}
	return v
}

// ToHistoryBranchRangeArray converts thrift HistoryBranchRange type array to internal
func ToHistoryBranchRangeArray(t []*shared.HistoryBranchRange) []*types.HistoryBranchRange {
	if t == nil {
		return nil
	}
	v := make([]*types.HistoryBranchRange, len(t))
	for i := range t {
		v[i] = ToHistoryBranchRange(t[i])
	}
	return v
}

// FromIndexedValueTypeMap converts internal IndexedValueType type map to thrift
func FromIndexedValueTypeMap(t map[string]types.IndexedValueType) map[string]shared.IndexedValueType {
	if t == nil {
		return nil
	}
	v := make(map[string]shared.IndexedValueType, len(t))
	for key := range t {
		v[key] = FromIndexedValueType(t[key])
	}
	return v
}

// ToIndexedValueTypeMap converts thrift IndexedValueType type map to internal
func ToIndexedValueTypeMap(t map[string]shared.IndexedValueType) map[string]types.IndexedValueType {
	if t == nil {
		return nil
	}
	v := make(map[string]types.IndexedValueType, len(t))
	for key := range t {
		v[key] = ToIndexedValueType(t[key])
	}
	return v
}

// FromWorkflowQueryMap converts internal WorkflowQuery type map to thrift
func FromWorkflowQueryMap(t map[string]*types.WorkflowQuery) map[string]*shared.WorkflowQuery {
	if t == nil {
		return nil
	}
	v := make(map[string]*shared.WorkflowQuery, len(t))
	for key := range t {
		v[key] = FromWorkflowQuery(t[key])
	}
	return v
}

// ToWorkflowQueryMap converts thrift WorkflowQuery type map to internal
func ToWorkflowQueryMap(t map[string]*shared.WorkflowQuery) map[string]*types.WorkflowQuery {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.WorkflowQuery, len(t))
	for key := range t {
		v[key] = ToWorkflowQuery(t[key])
	}
	return v
}

// FromWorkflowQueryResultMap converts internal WorkflowQueryResult type map to thrift
func FromWorkflowQueryResultMap(t map[string]*types.WorkflowQueryResult) map[string]*shared.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	v := make(map[string]*shared.WorkflowQueryResult, len(t))
	for key := range t {
		v[key] = FromWorkflowQueryResult(t[key])
	}
	return v
}

// ToWorkflowQueryResultMap converts thrift WorkflowQueryResult type map to internal
func ToWorkflowQueryResultMap(t map[string]*shared.WorkflowQueryResult) map[string]*types.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.WorkflowQueryResult, len(t))
	for key := range t {
		v[key] = ToWorkflowQueryResult(t[key])
	}
	return v
}

// FromActivityLocalDispatchInfoMap converts internal ActivityLocalDispatchInfo type map to thrift
func FromActivityLocalDispatchInfoMap(t map[string]*types.ActivityLocalDispatchInfo) map[string]*shared.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*shared.ActivityLocalDispatchInfo, len(t))
	for key := range t {
		v[key] = FromActivityLocalDispatchInfo(t[key])
	}
	return v
}

// ToActivityLocalDispatchInfoMap converts thrift ActivityLocalDispatchInfo type map to internal
func ToActivityLocalDispatchInfoMap(t map[string]*shared.ActivityLocalDispatchInfo) map[string]*types.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.ActivityLocalDispatchInfo, len(t))
	for key := range t {
		v[key] = ToActivityLocalDispatchInfo(t[key])
	}
	return v
}

// FromBadBinaryInfoMap converts internal BadBinaryInfo type map to thrift
func FromBadBinaryInfoMap(t map[string]*types.BadBinaryInfo) map[string]*shared.BadBinaryInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*shared.BadBinaryInfo, len(t))
	for key := range t {
		v[key] = FromBadBinaryInfo(t[key])
	}
	return v
}

// ToBadBinaryInfoMap converts thrift BadBinaryInfo type map to internal
func ToBadBinaryInfoMap(t map[string]*shared.BadBinaryInfo) map[string]*types.BadBinaryInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.BadBinaryInfo, len(t))
	for key := range t {
		v[key] = ToBadBinaryInfo(t[key])
	}
	return v
}

func FromIsolationGroupMetricsMap(t map[string]*types.IsolationGroupMetrics) map[string]*shared.IsolationGroupMetrics {
	if t == nil {
		return nil
	}
	v := make(map[string]*shared.IsolationGroupMetrics, len(t))
	for key := range t {
		v[key] = FromIsolationGroupMetrics(t[key])
	}
	return v
}

func ToIsolationGroupMetricsMap(t map[string]*shared.IsolationGroupMetrics) map[string]*types.IsolationGroupMetrics {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.IsolationGroupMetrics, len(t))
	for key := range t {
		v[key] = ToIsolationGroupMetrics(t[key])
	}
	return v
}

// FromHistoryArray converts internal History array to thrift
func FromHistoryArray(t []*types.History) []*shared.History {
	if t == nil {
		return nil
	}
	v := make([]*shared.History, len(t))
	for i := range t {
		v[i] = FromHistory(t[i])
	}
	return v
}

// ToHistoryArray converts thrift History array to internal
func ToHistoryArray(t []*shared.History) []*types.History {
	if t == nil {
		return nil
	}
	v := make([]*types.History, len(t))
	for i := range t {
		v[i] = ToHistory(t[i])
	}
	return v
}

// FromCrossClusterTaskType converts internal CrossClusterTaskType type to thrift
func FromCrossClusterTaskType(t *types.CrossClusterTaskType) *shared.CrossClusterTaskType {
	if t == nil {
		return nil
	}
	switch *t {
	case types.CrossClusterTaskTypeStartChildExecution:
		v := shared.CrossClusterTaskTypeStartChildExecution
		return &v
	case types.CrossClusterTaskTypeCancelExecution:
		v := shared.CrossClusterTaskTypeCancelExecution
		return &v
	case types.CrossClusterTaskTypeSignalExecution:
		v := shared.CrossClusterTaskTypeSignalExecution
		return &v
	case types.CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete:
		v := shared.CrossClusterTaskTypeRecordChildWorkflowExecutionComplete
		return &v
	case types.CrossClusterTaskTypeApplyParentPolicy:
		v := shared.CrossClusterTaskTypeApplyParentClosePolicy
		return &v
	}
	panic("unexpected enum value")
}

// ToCrossClusterTaskType converts thrift CrossClusterTaskType type to internal
func ToCrossClusterTaskType(t *shared.CrossClusterTaskType) *types.CrossClusterTaskType {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.CrossClusterTaskTypeStartChildExecution:
		v := types.CrossClusterTaskTypeStartChildExecution
		return &v
	case shared.CrossClusterTaskTypeCancelExecution:
		v := types.CrossClusterTaskTypeCancelExecution
		return &v
	case shared.CrossClusterTaskTypeSignalExecution:
		v := types.CrossClusterTaskTypeSignalExecution
		return &v
	case shared.CrossClusterTaskTypeRecordChildWorkflowExecutionComplete:
		v := types.CrossClusterTaskTypeRecordChildWorkflowExeuctionComplete
		return &v
	case shared.CrossClusterTaskTypeApplyParentClosePolicy:
		v := types.CrossClusterTaskTypeApplyParentPolicy
		return &v
	}
	panic("unexpected enum value")
}

// FromCrossClusterTaskFailedCause converts internal CrossClusterTaskFailedCause type to thrift
func FromCrossClusterTaskFailedCause(t *types.CrossClusterTaskFailedCause) *shared.CrossClusterTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.CrossClusterTaskFailedCauseDomainNotActive:
		v := shared.CrossClusterTaskFailedCauseDomainNotActive
		return &v
	case types.CrossClusterTaskFailedCauseDomainNotExists:
		v := shared.CrossClusterTaskFailedCauseDomainNotExists
		return &v
	case types.CrossClusterTaskFailedCauseWorkflowAlreadyRunning:
		v := shared.CrossClusterTaskFailedCauseWorkflowAlreadyRunning
		return &v
	case types.CrossClusterTaskFailedCauseWorkflowNotExists:
		v := shared.CrossClusterTaskFailedCauseWorkflowNotExists
		return &v
	case types.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted:
		v := shared.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted
		return &v
	case types.CrossClusterTaskFailedCauseUncategorized:
		v := shared.CrossClusterTaskFailedCauseUncategorized
		return &v
	}
	panic("unexpected enum value")
}

// ToCrossClusterTaskFailedCause converts thrift CrossClusterTaskFailedCause type to internal
func ToCrossClusterTaskFailedCause(t *shared.CrossClusterTaskFailedCause) *types.CrossClusterTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.CrossClusterTaskFailedCauseDomainNotActive:
		v := types.CrossClusterTaskFailedCauseDomainNotActive
		return &v
	case shared.CrossClusterTaskFailedCauseDomainNotExists:
		v := types.CrossClusterTaskFailedCauseDomainNotExists
		return &v
	case shared.CrossClusterTaskFailedCauseWorkflowAlreadyRunning:
		v := types.CrossClusterTaskFailedCauseWorkflowAlreadyRunning
		return &v
	case shared.CrossClusterTaskFailedCauseWorkflowNotExists:
		v := types.CrossClusterTaskFailedCauseWorkflowNotExists
		return &v
	case shared.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted:
		v := types.CrossClusterTaskFailedCauseWorkflowAlreadyCompleted
		return &v
	case shared.CrossClusterTaskFailedCauseUncategorized:
		v := types.CrossClusterTaskFailedCauseUncategorized
		return &v
	}
	panic("unexpected enum value")
}

// FromGetTaskFailedCause converts internal GetTaskFailedCause to thrift
func FromGetTaskFailedCause(t *types.GetTaskFailedCause) *shared.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case types.GetTaskFailedCauseServiceBusy:
		v := shared.GetTaskFailedCauseServiceBusy
		return &v
	case types.GetTaskFailedCauseTimeout:
		v := shared.GetTaskFailedCauseTimeout
		return &v
	case types.GetTaskFailedCauseShardOwnershipLost:
		v := shared.GetTaskFailedCauseShardOwnershipLost
		return &v
	case types.GetTaskFailedCauseUncategorized:
		v := shared.GetTaskFailedCauseUncategorized
		return &v
	}
	panic("unexpected enum value")
}

// ToGetCrossClusterTaskFailedCause converts thrift GetTaskFailedCause type to internal
func ToGetCrossClusterTaskFailedCause(t *shared.GetTaskFailedCause) *types.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.GetTaskFailedCauseServiceBusy:
		v := types.GetTaskFailedCauseServiceBusy
		return &v
	case shared.GetTaskFailedCauseTimeout:
		v := types.GetTaskFailedCauseTimeout
		return &v
	case shared.GetTaskFailedCauseShardOwnershipLost:
		v := types.GetTaskFailedCauseShardOwnershipLost
		return &v
	case shared.GetTaskFailedCauseUncategorized:
		v := types.GetTaskFailedCauseUncategorized
		return &v
	}
	panic("unexpected enum value")
}

// FromCrossClusterTaskInfo converts internal CrossClusterTaskInfo type to thrift
func FromCrossClusterTaskInfo(t *types.CrossClusterTaskInfo) *shared.CrossClusterTaskInfo {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterTaskInfo{
		DomainID:            &t.DomainID,
		WorkflowID:          &t.WorkflowID,
		RunID:               &t.RunID,
		TaskType:            FromCrossClusterTaskType(t.TaskType),
		TaskState:           &t.TaskState,
		TaskID:              &t.TaskID,
		VisibilityTimestamp: t.VisibilityTimestamp,
	}
}

// ToCrossClusterTaskInfo converts thrift CrossClusterTaskInfo type to internal
func ToCrossClusterTaskInfo(t *shared.CrossClusterTaskInfo) *types.CrossClusterTaskInfo {
	if t == nil {
		return nil
	}
	return &types.CrossClusterTaskInfo{
		DomainID:            t.GetDomainID(),
		WorkflowID:          t.GetWorkflowID(),
		RunID:               t.GetRunID(),
		TaskType:            ToCrossClusterTaskType(t.TaskType),
		TaskState:           t.GetTaskState(),
		TaskID:              t.GetTaskID(),
		VisibilityTimestamp: t.VisibilityTimestamp,
	}
}

// FromCrossClusterStartChildExecutionRequestAttributes converts internal CrossClusterStartChildExecutionRequestAttributes type to thrift
func FromCrossClusterStartChildExecutionRequestAttributes(t *types.CrossClusterStartChildExecutionRequestAttributes) *shared.CrossClusterStartChildExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterStartChildExecutionRequestAttributes{
		TargetDomainID:           &t.TargetDomainID,
		RequestID:                &t.RequestID,
		InitiatedEventID:         &t.InitiatedEventID,
		InitiatedEventAttributes: FromStartChildWorkflowExecutionInitiatedEventAttributes(t.InitiatedEventAttributes),
		TargetRunID:              t.TargetRunID,
		PartitionConfig:          t.PartitionConfig,
	}
}

// ToCrossClusterStartChildExecutionRequestAttributes converts thrift CrossClusterStartChildExecutionRequestAttributes type to internal
func ToCrossClusterStartChildExecutionRequestAttributes(t *shared.CrossClusterStartChildExecutionRequestAttributes) *types.CrossClusterStartChildExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterStartChildExecutionRequestAttributes{
		TargetDomainID:           t.GetTargetDomainID(),
		RequestID:                t.GetRequestID(),
		InitiatedEventID:         t.GetInitiatedEventID(),
		InitiatedEventAttributes: ToStartChildWorkflowExecutionInitiatedEventAttributes(t.InitiatedEventAttributes),
		TargetRunID:              t.TargetRunID,
		PartitionConfig:          t.PartitionConfig,
	}
}

// FromCrossClusterStartChildExecutionResponseAttributes converts internal CrossClusterStartChildExecutionResponseAttributes type to thrift
func FromCrossClusterStartChildExecutionResponseAttributes(t *types.CrossClusterStartChildExecutionResponseAttributes) *shared.CrossClusterStartChildExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterStartChildExecutionResponseAttributes{
		RunID: &t.RunID,
	}
}

// ToCrossClusterStartChildExecutionResponseAttributes converts thrift CrossClusterStartChildExecutionResponseAttributes type to internal
func ToCrossClusterStartChildExecutionResponseAttributes(t *shared.CrossClusterStartChildExecutionResponseAttributes) *types.CrossClusterStartChildExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterStartChildExecutionResponseAttributes{
		RunID: t.GetRunID(),
	}
}

// FromCrossClusterCancelExecutionRequestAttributes converts internal CrossClusterCancelExecutionRequestAttributes type to thrift
func FromCrossClusterCancelExecutionRequestAttributes(t *types.CrossClusterCancelExecutionRequestAttributes) *shared.CrossClusterCancelExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterCancelExecutionRequestAttributes{
		TargetDomainID:    &t.TargetDomainID,
		TargetWorkflowID:  &t.TargetWorkflowID,
		TargetRunID:       &t.TargetRunID,
		RequestID:         &t.RequestID,
		InitiatedEventID:  &t.InitiatedEventID,
		ChildWorkflowOnly: &t.ChildWorkflowOnly,
	}
}

// ToCrossClusterCancelExecutionRequestAttributes converts thrift CrossClusterCancelExecutionRequestAttributes type to internal
func ToCrossClusterCancelExecutionRequestAttributes(t *shared.CrossClusterCancelExecutionRequestAttributes) *types.CrossClusterCancelExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterCancelExecutionRequestAttributes{
		TargetDomainID:    t.GetTargetDomainID(),
		TargetWorkflowID:  t.GetTargetWorkflowID(),
		TargetRunID:       t.GetTargetRunID(),
		RequestID:         t.GetRequestID(),
		InitiatedEventID:  t.GetInitiatedEventID(),
		ChildWorkflowOnly: t.GetChildWorkflowOnly(),
	}
}

// FromCrossClusterCancelExecutionResponseAttributes converts internal CrossClusterCancelExecutionResponseAttributes type to thrift
func FromCrossClusterCancelExecutionResponseAttributes(t *types.CrossClusterCancelExecutionResponseAttributes) *shared.CrossClusterCancelExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterCancelExecutionResponseAttributes{}
}

// ToCrossClusterCancelExecutionResponseAttributes converts thrift CrossClusterCancelExecutionResponseAttributes type to internal
func ToCrossClusterCancelExecutionResponseAttributes(t *shared.CrossClusterCancelExecutionResponseAttributes) *types.CrossClusterCancelExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterCancelExecutionResponseAttributes{}
}

// FromCrossClusterSignalExecutionRequestAttributes converts internal CrossClusterSignalExecutionRequestAttributes type to thrift
func FromCrossClusterSignalExecutionRequestAttributes(t *types.CrossClusterSignalExecutionRequestAttributes) *shared.CrossClusterSignalExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterSignalExecutionRequestAttributes{
		TargetDomainID:    &t.TargetDomainID,
		TargetWorkflowID:  &t.TargetWorkflowID,
		TargetRunID:       &t.TargetRunID,
		RequestID:         &t.RequestID,
		InitiatedEventID:  &t.InitiatedEventID,
		ChildWorkflowOnly: &t.ChildWorkflowOnly,
		SignalName:        &t.SignalName,
		SignalInput:       t.SignalInput,
		Control:           t.Control,
	}
}

// ToCrossClusterSignalExecutionRequestAttributes converts thrift CrossClusterSignalExecutionRequestAttributes type to internal
func ToCrossClusterSignalExecutionRequestAttributes(t *shared.CrossClusterSignalExecutionRequestAttributes) *types.CrossClusterSignalExecutionRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterSignalExecutionRequestAttributes{
		TargetDomainID:    t.GetTargetDomainID(),
		TargetWorkflowID:  t.GetTargetWorkflowID(),
		TargetRunID:       t.GetTargetRunID(),
		RequestID:         t.GetRequestID(),
		InitiatedEventID:  t.GetInitiatedEventID(),
		ChildWorkflowOnly: t.GetChildWorkflowOnly(),
		SignalName:        t.GetSignalName(),
		SignalInput:       t.SignalInput,
		Control:           t.Control,
	}
}

// FromCrossClusterSignalExecutionResponseAttributes converts internal CrossClusterSignalExecutionResponseAttributes type to thrift
func FromCrossClusterSignalExecutionResponseAttributes(t *types.CrossClusterSignalExecutionResponseAttributes) *shared.CrossClusterSignalExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterSignalExecutionResponseAttributes{}
}

// ToCrossClusterSignalExecutionResponseAttributes converts thrift CrossClusterSignalExecutionResponseAttributes type to internal
func ToCrossClusterSignalExecutionResponseAttributes(t *shared.CrossClusterSignalExecutionResponseAttributes) *types.CrossClusterSignalExecutionResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterSignalExecutionResponseAttributes{}
}

// FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes converts internal
// CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes type to thrift
func FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(
	t *types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes,
) *shared.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes{
		TargetDomainID:   &t.TargetDomainID,
		TargetWorkflowID: &t.TargetWorkflowID,
		TargetRunID:      &t.TargetRunID,
		InitiatedEventID: &t.InitiatedEventID,
		CompletionEvent:  FromHistoryEvent(t.CompletionEvent),
	}
}

// ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes converts thrift
// CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes type to internal
func ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(
	t *shared.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes,
) *types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes{
		TargetDomainID:   t.GetTargetDomainID(),
		TargetWorkflowID: t.GetTargetWorkflowID(),
		TargetRunID:      t.GetTargetRunID(),
		InitiatedEventID: t.GetInitiatedEventID(),
		CompletionEvent:  ToHistoryEvent(t.CompletionEvent),
	}
}

// FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes converts internal
// CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes type to thrift
func FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(
	t *types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes,
) *shared.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes{}
}

// ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes converts thrift
// CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes type to internal
func ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(
	t *shared.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes,
) *types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes{}
}

// FromApplyParentClosePolicyAttributes converts internal
// ApplyParentClosePolicyAttributes types to thrift
func FromApplyParentClosePolicyAttributes(
	t *types.ApplyParentClosePolicyAttributes,
) *shared.ApplyParentClosePolicyAttributes {
	if t == nil {
		return nil
	}
	return &shared.ApplyParentClosePolicyAttributes{
		ChildDomainID:     &t.ChildDomainID,
		ChildWorkflowID:   &t.ChildWorkflowID,
		ChildRunID:        &t.ChildRunID,
		ParentClosePolicy: FromParentClosePolicy(t.ParentClosePolicy),
	}
}

// ToApplyParentClosePolicyAttributes converts thrift
// ApplyParentClosePolicyAttributes types to internal
func ToApplyParentClosePolicyAttributes(
	t *shared.ApplyParentClosePolicyAttributes,
) *types.ApplyParentClosePolicyAttributes {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyAttributes{
		ChildDomainID:     t.GetChildDomainID(),
		ChildWorkflowID:   t.GetChildWorkflowID(),
		ChildRunID:        t.GetChildRunID(),
		ParentClosePolicy: ToParentClosePolicy(t.ParentClosePolicy),
	}
}

// FromApplyParentClosePolicyStatus converts thrift
// ApplyParentClosePolicyStatus types to internal
func FromApplyParentClosePolicyStatus(
	t *types.ApplyParentClosePolicyStatus,
) *shared.ApplyParentClosePolicyStatus {
	if t == nil {
		return nil
	}
	return &shared.ApplyParentClosePolicyStatus{
		Completed:   &t.Completed,
		FailedCause: FromCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// ToApplyParentClosePolicyStatus converts thrift
// ApplyParentClosePolicyStatus types to internal
func ToApplyParentClosePolicyStatus(
	t *shared.ApplyParentClosePolicyStatus,
) *types.ApplyParentClosePolicyStatus {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyStatus{
		Completed:   t.GetCompleted(),
		FailedCause: ToCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// FromApplyParentClosePolicyRequest converts thrift
// ApplyParentClosePolicyRequest types to internal
func FromApplyParentClosePolicyRequest(
	t *types.ApplyParentClosePolicyRequest,
) *shared.ApplyParentClosePolicyRequest {
	if t == nil {
		return nil
	}
	return &shared.ApplyParentClosePolicyRequest{
		Child:  FromApplyParentClosePolicyAttributes(t.Child),
		Status: FromApplyParentClosePolicyStatus(t.Status),
	}
}

// ToApplyParentClosePolicyRequest converts thrift
// ApplyParentClosePolicyRequest types to internal
func ToApplyParentClosePolicyRequest(
	t *shared.ApplyParentClosePolicyRequest,
) *types.ApplyParentClosePolicyRequest {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyRequest{
		Child:  ToApplyParentClosePolicyAttributes(t.Child),
		Status: ToApplyParentClosePolicyStatus(t.Status),
	}
}

// FromApplyParentClosePolicyRequestArray converts thrift
// ApplyParentClosePolicyRequestArray types to internal
func FromApplyParentClosePolicyRequestArray(
	t []*types.ApplyParentClosePolicyRequest,
) []*shared.ApplyParentClosePolicyRequest {
	if t == nil {
		return nil
	}
	v := make([]*shared.ApplyParentClosePolicyRequest, len(t))
	for i := range t {
		v[i] = FromApplyParentClosePolicyRequest(t[i])
	}
	return v
}

// ToApplyParentClosePolicyRequestArray converts internal
// ApplyParentClosePolicyRequestArray types to thrift
func ToApplyParentClosePolicyRequestArray(
	t []*shared.ApplyParentClosePolicyRequest,
) []*types.ApplyParentClosePolicyRequest {
	if t == nil {
		return nil
	}
	v := make([]*types.ApplyParentClosePolicyRequest, len(t))
	for i := range t {
		v[i] = ToApplyParentClosePolicyRequest(t[i])
	}
	return v
}

// FromApplyParentClosePolicyResult converts thrift
// ApplyParentClosePolicyResult types to internal
func FromApplyParentClosePolicyResult(
	t *types.ApplyParentClosePolicyResult,
) *shared.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	return &shared.ApplyParentClosePolicyResult{
		Child:       FromApplyParentClosePolicyAttributes(t.Child),
		FailedCause: FromCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// ToApplyParentClosePolicyResult converts thrift
// ApplyParentClosePolicyResult types to internal
func ToApplyParentClosePolicyResult(
	t *shared.ApplyParentClosePolicyResult,
) *types.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	return &types.ApplyParentClosePolicyResult{
		Child:       ToApplyParentClosePolicyAttributes(t.Child),
		FailedCause: ToCrossClusterTaskFailedCause(t.FailedCause),
	}
}

// FromApplyParentClosePolicyResultArray converts internal
// ApplyParentClosePolicyResultArray types to internal
func FromApplyParentClosePolicyResultArray(
	t []*types.ApplyParentClosePolicyResult,
) []*shared.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	v := make([]*shared.ApplyParentClosePolicyResult, len(t))
	for i := range t {
		v[i] = FromApplyParentClosePolicyResult(t[i])
	}
	return v
}

// ToApplyParentClosePolicyResultArray converts internal
// ApplyParentClosePolicyResultArray types to thrift
func ToApplyParentClosePolicyResultArray(
	t []*shared.ApplyParentClosePolicyResult,
) []*types.ApplyParentClosePolicyResult {
	if t == nil {
		return nil
	}
	v := make([]*types.ApplyParentClosePolicyResult, len(t))
	for i := range t {
		v[i] = ToApplyParentClosePolicyResult(t[i])
	}
	return v
}

// FromCrossClusterApplyParentClosePolicyRequestAttributes converts internal
// CrossClusterApplyParentClosePolicyRequestAttributes types to thrift
func FromCrossClusterApplyParentClosePolicyRequestAttributes(
	t *types.CrossClusterApplyParentClosePolicyRequestAttributes,
) *shared.CrossClusterApplyParentClosePolicyRequestAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterApplyParentClosePolicyRequestAttributes{
		Children: FromApplyParentClosePolicyRequestArray(t.Children),
	}
}

// ToCrossClusterApplyParentClosePolicyRequestAttributes converts thrift
// CrossClusterApplyParentClosePolicyRequestAttributes types to internal
func ToCrossClusterApplyParentClosePolicyRequestAttributes(
	t *shared.CrossClusterApplyParentClosePolicyRequestAttributes,
) *types.CrossClusterApplyParentClosePolicyRequestAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterApplyParentClosePolicyRequestAttributes{
		Children: ToApplyParentClosePolicyRequestArray(t.Children),
	}
}

// FromCrossClusterApplyParentClosePolicyResponseAttributes converts internal
// CrossClusterApplyParentClosePolicyResponseAttributes type to thrift
func FromCrossClusterApplyParentClosePolicyResponseAttributes(
	t *types.CrossClusterApplyParentClosePolicyResponseAttributes,
) *shared.CrossClusterApplyParentClosePolicyResponseAttributes {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterApplyParentClosePolicyResponseAttributes{
		ChildrenStatus: FromApplyParentClosePolicyResultArray(t.ChildrenStatus),
	}
}

// ToCrossClusterApplyParentClosePolicyResponseAttributes converts thrift
// CrossClusterApplyParentClosePolicyResponseAttributes type to internal
func ToCrossClusterApplyParentClosePolicyResponseAttributes(
	t *shared.CrossClusterApplyParentClosePolicyResponseAttributes,
) *types.CrossClusterApplyParentClosePolicyResponseAttributes {
	if t == nil {
		return nil
	}
	return &types.CrossClusterApplyParentClosePolicyResponseAttributes{
		ChildrenStatus: ToApplyParentClosePolicyResultArray(t.ChildrenStatus),
	}
}

// FromCrossClusterTaskRequest converts internal CrossClusterTaskRequest type to thrift
func FromCrossClusterTaskRequest(t *types.CrossClusterTaskRequest) *shared.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterTaskRequest{
		TaskInfo:                                       FromCrossClusterTaskInfo(t.TaskInfo),
		StartChildExecutionAttributes:                  FromCrossClusterStartChildExecutionRequestAttributes(t.StartChildExecutionAttributes),
		CancelExecutionAttributes:                      FromCrossClusterCancelExecutionRequestAttributes(t.CancelExecutionAttributes),
		SignalExecutionAttributes:                      FromCrossClusterSignalExecutionRequestAttributes(t.SignalExecutionAttributes),
		RecordChildWorkflowExecutionCompleteAttributes: FromCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		ApplyParentClosePolicyAttributes:               FromCrossClusterApplyParentClosePolicyRequestAttributes(t.ApplyParentClosePolicyAttributes),
	}
}

// ToCrossClusterTaskRequest converts thrift CrossClusterTaskRequest type to internal
func ToCrossClusterTaskRequest(t *shared.CrossClusterTaskRequest) *types.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	return &types.CrossClusterTaskRequest{
		TaskInfo:                                       ToCrossClusterTaskInfo(t.TaskInfo),
		StartChildExecutionAttributes:                  ToCrossClusterStartChildExecutionRequestAttributes(t.StartChildExecutionAttributes),
		CancelExecutionAttributes:                      ToCrossClusterCancelExecutionRequestAttributes(t.CancelExecutionAttributes),
		SignalExecutionAttributes:                      ToCrossClusterSignalExecutionRequestAttributes(t.SignalExecutionAttributes),
		RecordChildWorkflowExecutionCompleteAttributes: ToCrossClusterRecordChildWorkflowExecutionCompleteRequestAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		ApplyParentClosePolicyAttributes:               ToCrossClusterApplyParentClosePolicyRequestAttributes(t.ApplyParentClosePolicyAttributes),
	}
}

// FromCrossClusterTaskResponse converts internal CrossClusterTaskResponse type to thrift
func FromCrossClusterTaskResponse(t *types.CrossClusterTaskResponse) *shared.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	return &shared.CrossClusterTaskResponse{
		TaskID:                        &t.TaskID,
		TaskType:                      FromCrossClusterTaskType(t.TaskType),
		TaskState:                     &t.TaskState,
		FailedCause:                   FromCrossClusterTaskFailedCause(t.FailedCause),
		StartChildExecutionAttributes: FromCrossClusterStartChildExecutionResponseAttributes(t.StartChildExecutionAttributes),
		CancelExecutionAttributes:     FromCrossClusterCancelExecutionResponseAttributes(t.CancelExecutionAttributes),
		SignalExecutionAttributes:     FromCrossClusterSignalExecutionResponseAttributes(t.SignalExecutionAttributes),
		RecordChildWorkflowExecutionCompleteAttributes: FromCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		ApplyParentClosePolicyAttributes:               FromCrossClusterApplyParentClosePolicyResponseAttributes(t.ApplyParentClosePolicyAttributes),
	}
}

// ToCrossClusterTaskResponse converts thrift CrossClusterTaskResponse type to internal
func ToCrossClusterTaskResponse(t *shared.CrossClusterTaskResponse) *types.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	return &types.CrossClusterTaskResponse{
		TaskID:                        t.GetTaskID(),
		TaskType:                      ToCrossClusterTaskType(t.TaskType),
		TaskState:                     t.GetTaskState(),
		FailedCause:                   ToCrossClusterTaskFailedCause(t.FailedCause),
		StartChildExecutionAttributes: ToCrossClusterStartChildExecutionResponseAttributes(t.StartChildExecutionAttributes),
		CancelExecutionAttributes:     ToCrossClusterCancelExecutionResponseAttributes(t.CancelExecutionAttributes),
		SignalExecutionAttributes:     ToCrossClusterSignalExecutionResponseAttributes(t.SignalExecutionAttributes),
		RecordChildWorkflowExecutionCompleteAttributes: ToCrossClusterRecordChildWorkflowExecutionCompleteResponseAttributes(t.RecordChildWorkflowExecutionCompleteAttributes),
		ApplyParentClosePolicyAttributes:               ToCrossClusterApplyParentClosePolicyResponseAttributes(t.ApplyParentClosePolicyAttributes),
	}
}

// FromAdminGetCrossClusterTasksRequest converts internal GetCrossClusterTasksRequest type to thrift
func FromAdminGetCrossClusterTasksRequest(t *types.GetCrossClusterTasksRequest) *shared.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &shared.GetCrossClusterTasksRequest{
		ShardIDs:      t.ShardIDs,
		TargetCluster: &t.TargetCluster,
	}
}

// ToAdminGetCrossClusterTasksRequest converts thrift GetCrossClusterTasksRequest type to internal
func ToAdminGetCrossClusterTasksRequest(t *shared.GetCrossClusterTasksRequest) *types.GetCrossClusterTasksRequest {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksRequest{
		ShardIDs:      t.ShardIDs,
		TargetCluster: t.GetTargetCluster(),
	}
}

// FromCrossClusterTaskRequestArray converts internal CrossClusterTaskRequest type array to thrift
func FromCrossClusterTaskRequestArray(t []*types.CrossClusterTaskRequest) []*shared.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	v := make([]*shared.CrossClusterTaskRequest, len(t))
	for i := range t {
		v[i] = FromCrossClusterTaskRequest(t[i])
	}
	return v
}

// ToCrossClusterTaskRequestArray converts thrift CrossClusterTaskRequest type array to internal
func ToCrossClusterTaskRequestArray(t []*shared.CrossClusterTaskRequest) []*types.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	v := make([]*types.CrossClusterTaskRequest, len(t))
	for i := range t {
		v[i] = ToCrossClusterTaskRequest(t[i])
	}
	return v
}

// FromCrossClusterTaskRequestMap converts internal CrossClusterTaskRequest type map to thrift
func FromCrossClusterTaskRequestMap(t map[int32][]*types.CrossClusterTaskRequest) map[int32][]*shared.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	v := make(map[int32][]*shared.CrossClusterTaskRequest)
	for key := range t {
		v[key] = FromCrossClusterTaskRequestArray(t[key])
	}
	return v
}

// ToCrossClusterTaskRequestMap converts thrift CrossClusterTaskRequest type map to internal
func ToCrossClusterTaskRequestMap(t map[int32][]*shared.CrossClusterTaskRequest) map[int32][]*types.CrossClusterTaskRequest {
	if t == nil {
		return nil
	}
	v := make(map[int32][]*types.CrossClusterTaskRequest)
	for key := range t {
		v[key] = ToCrossClusterTaskRequestArray(t[key])
	}
	return v
}

// FromGetTaskFailedCauseMap converts internal GetTaskFailedCause type map to thrift
func FromGetTaskFailedCauseMap(t map[int32]types.GetTaskFailedCause) map[int32]shared.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	v := make(map[int32]shared.GetTaskFailedCause)
	for key, value := range t {
		v[key] = *FromGetTaskFailedCause(&value)
	}
	return v
}

// ToGetTaskFailedCauseMap converts thrift GetTaskFailedCause type map to internal
func ToGetTaskFailedCauseMap(t map[int32]shared.GetTaskFailedCause) map[int32]types.GetTaskFailedCause {
	if t == nil {
		return nil
	}
	v := make(map[int32]types.GetTaskFailedCause)
	for key, value := range t {
		v[key] = *ToGetCrossClusterTaskFailedCause(&value)
	}
	return v
}

// FromAdminGetCrossClusterTasksResponse converts internal GetCrossClusterTasksResponse type to thrift
func FromAdminGetCrossClusterTasksResponse(t *types.GetCrossClusterTasksResponse) *shared.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &shared.GetCrossClusterTasksResponse{
		TasksByShard:       FromCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: FromGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// ToAdminGetCrossClusterTasksResponse converts thrift GetCrossClusterTasksResponse type to internal
func ToAdminGetCrossClusterTasksResponse(t *shared.GetCrossClusterTasksResponse) *types.GetCrossClusterTasksResponse {
	if t == nil {
		return nil
	}
	return &types.GetCrossClusterTasksResponse{
		TasksByShard:       ToCrossClusterTaskRequestMap(t.TasksByShard),
		FailedCauseByShard: ToGetTaskFailedCauseMap(t.FailedCauseByShard),
	}
}

// FromCrossClusterTaskResponseArray converts internal CrossClusterTaskResponse type array to thrift
func FromCrossClusterTaskResponseArray(t []*types.CrossClusterTaskResponse) []*shared.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	v := make([]*shared.CrossClusterTaskResponse, len(t))
	for i := range t {
		v[i] = FromCrossClusterTaskResponse(t[i])
	}
	return v
}

// ToCrossClusterTaskResponseArray converts thrift CrossClusterTaskResponse type array to internal
func ToCrossClusterTaskResponseArray(t []*shared.CrossClusterTaskResponse) []*types.CrossClusterTaskResponse {
	if t == nil {
		return nil
	}
	v := make([]*types.CrossClusterTaskResponse, len(t))
	for i := range t {
		v[i] = ToCrossClusterTaskResponse(t[i])
	}
	return v
}

// FromAdminRespondCrossClusterTasksCompletedRequest converts internal RespondCrossClusterTasksCompletedRequest type to thrift
func FromAdminRespondCrossClusterTasksCompletedRequest(t *types.RespondCrossClusterTasksCompletedRequest) *shared.RespondCrossClusterTasksCompletedRequest {
	if t == nil {
		return nil
	}
	return &shared.RespondCrossClusterTasksCompletedRequest{
		ShardID:       &t.ShardID,
		TargetCluster: &t.TargetCluster,
		TaskResponses: FromCrossClusterTaskResponseArray(t.TaskResponses),
		FetchNewTasks: &t.FetchNewTasks,
	}
}

// ToAdminRespondCrossClusterTasksCompletedRequest converts thrift RespondCrossClusterTasksCompletedRequest type to internal
func ToAdminRespondCrossClusterTasksCompletedRequest(t *shared.RespondCrossClusterTasksCompletedRequest) *types.RespondCrossClusterTasksCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondCrossClusterTasksCompletedRequest{
		ShardID:       t.GetShardID(),
		TargetCluster: t.GetTargetCluster(),
		TaskResponses: ToCrossClusterTaskResponseArray(t.TaskResponses),
		FetchNewTasks: t.GetFetchNewTasks(),
	}
}

// FromAdminRespondCrossClusterTasksCompletedResponse converts internal RespondCrossClusterTasksCompletedResponse type to thrift
func FromAdminRespondCrossClusterTasksCompletedResponse(t *types.RespondCrossClusterTasksCompletedResponse) *shared.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &shared.RespondCrossClusterTasksCompletedResponse{
		Tasks: FromCrossClusterTaskRequestArray(t.Tasks),
	}
}

// ToAdminRespondCrossClusterTasksCompletedResponse converts thrift RespondCrossClusterTasksCompletedResponse type to internal
func ToAdminRespondCrossClusterTasksCompletedResponse(t *shared.RespondCrossClusterTasksCompletedResponse) *types.RespondCrossClusterTasksCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.RespondCrossClusterTasksCompletedResponse{
		Tasks: ToCrossClusterTaskRequestArray(t.Tasks),
	}
}

// FromStickyWorkerUnavailableError converts internal StickyWorkerUnavailableError type to thrift
func FromStickyWorkerUnavailableError(t *types.StickyWorkerUnavailableError) *shared.StickyWorkerUnavailableError {
	if t == nil {
		return nil
	}
	return &shared.StickyWorkerUnavailableError{
		Message: t.Message,
	}
}

// ToStickyWorkerUnavailableError converts thrift StickyWorkerUnavailableError type to internal
func ToStickyWorkerUnavailableError(t *shared.StickyWorkerUnavailableError) *types.StickyWorkerUnavailableError {
	if t == nil {
		return nil
	}
	return &types.StickyWorkerUnavailableError{
		Message: t.Message,
	}
}

// FromTaskListNotOwnedByHostError converts internal TaskListNotOwnedByHostError type to thrift
func FromTaskListNotOwnedByHostError(t *cadence_errors.TaskListNotOwnedByHostError) *shared.TaskListNotOwnedByHostError {
	if t == nil {
		return nil
	}
	return &shared.TaskListNotOwnedByHostError{
		OwnedByIdentity: t.OwnedByIdentity,
		MyIdentity:      t.MyIdentity,
		TasklistName:    t.TasklistName,
	}
}

// ToTaskListNotOwnedByHostError converts thrift TaskListNotOwnedByHostError type to internal
func ToTaskListNotOwnedByHostError(t *shared.TaskListNotOwnedByHostError) *cadence_errors.TaskListNotOwnedByHostError {
	if t == nil {
		return nil
	}
	return &cadence_errors.TaskListNotOwnedByHostError{
		OwnedByIdentity: t.OwnedByIdentity,
		MyIdentity:      t.MyIdentity,
		TasklistName:    t.TasklistName,
	}
}

// ToAny converts thrift Any type to internal
func ToAny(t *shared.Any) *types.Any {
	if t == nil {
		return nil
	}
	return &types.Any{
		ValueType: t.GetValueType(),
		Value:     t.Value,
	}
}

// FromAny converts internal Any type to thrift
func FromAny(t *types.Any) *shared.Any {
	if t == nil {
		return nil
	}
	dup := t.ValueType
	return &shared.Any{
		ValueType: &dup,
		Value:     t.Value,
	}
}

// FromWorkflowQuery converts internal WorkflowQuery type to thrift
func FromAutoConfigHint(t *types.AutoConfigHint) *shared.AutoConfigHint {
	if t == nil {
		return nil
	}
	return &shared.AutoConfigHint{
		PollerWaitTimeInMs: &t.PollerWaitTimeInMs,
		EnableAutoConfig:   &t.EnableAutoConfig,
	}
}

// ToWorkflowQuery converts thrift WorkflowQuery type to internal
func ToAutoConfigHint(t *shared.AutoConfigHint) *types.AutoConfigHint {
	if t == nil {
		return nil
	}
	return &types.AutoConfigHint{
		PollerWaitTimeInMs: *t.PollerWaitTimeInMs,
		EnableAutoConfig:   *t.EnableAutoConfig,
	}
}

func FromTaskKey(t *types.TaskKey) *shared.TaskKey {
	if t == nil {
		return nil
	}
	return &shared.TaskKey{
		TaskID:            common.Ptr(t.TaskID),
		ScheduledTimeNano: common.Ptr(t.ScheduledTimeNano),
	}
}

func ToTaskKey(t *shared.TaskKey) *types.TaskKey {
	if t == nil {
		return nil
	}
	return &types.TaskKey{
		TaskID:            t.GetTaskID(),
		ScheduledTimeNano: t.GetScheduledTimeNano(),
	}
}

func FromTaskRange(t *types.TaskRange) *shared.TaskRange {
	if t == nil {
		return nil
	}
	return &shared.TaskRange{
		InclusiveMin: FromTaskKey(t.InclusiveMin),
		ExclusiveMax: FromTaskKey(t.ExclusiveMax),
	}
}

func ToTaskRange(t *shared.TaskRange) *types.TaskRange {
	if t == nil {
		return nil
	}
	return &types.TaskRange{
		InclusiveMin: ToTaskKey(t.InclusiveMin),
		ExclusiveMax: ToTaskKey(t.ExclusiveMax),
	}
}

func FromVirtualSliceState(t *types.VirtualSliceState) *shared.VirtualSliceState {
	if t == nil {
		return nil
	}
	return &shared.VirtualSliceState{
		TaskRange: FromTaskRange(t.TaskRange),
		Predicate: FromPredicate(t.Predicate),
	}
}

func ToVirtualSliceState(t *shared.VirtualSliceState) *types.VirtualSliceState {
	if t == nil {
		return nil
	}
	return &types.VirtualSliceState{
		TaskRange: ToTaskRange(t.TaskRange),
		Predicate: ToPredicate(t.Predicate),
	}
}

func FromVirtualSliceStateArray(t []*types.VirtualSliceState) []*shared.VirtualSliceState {
	if t == nil {
		return nil
	}
	v := make([]*shared.VirtualSliceState, len(t))
	for i := range t {
		v[i] = FromVirtualSliceState(t[i])
	}
	return v
}

func ToVirtualSliceStateArray(t []*shared.VirtualSliceState) []*types.VirtualSliceState {
	if t == nil {
		return nil
	}
	v := make([]*types.VirtualSliceState, len(t))
	for i := range t {
		v[i] = ToVirtualSliceState(t[i])
	}
	return v
}

func FromVirtualQueueState(t *types.VirtualQueueState) *shared.VirtualQueueState {
	if t == nil {
		return nil
	}
	return &shared.VirtualQueueState{
		VirtualSliceStates: FromVirtualSliceStateArray(t.VirtualSliceStates),
	}
}

func ToVirtualQueueState(t *shared.VirtualQueueState) *types.VirtualQueueState {
	if t == nil {
		return nil
	}
	return &types.VirtualQueueState{
		VirtualSliceStates: ToVirtualSliceStateArray(t.VirtualSliceStates),
	}
}

func FromVirtualQueueStateMap(t map[int64]*types.VirtualQueueState) map[int64]*shared.VirtualQueueState {
	if t == nil {
		return nil
	}
	v := make(map[int64]*shared.VirtualQueueState, len(t))
	for key := range t {
		v[key] = FromVirtualQueueState(t[key])
	}
	return v
}

func ToVirtualQueueStateMap(t map[int64]*shared.VirtualQueueState) map[int64]*types.VirtualQueueState {
	if t == nil {
		return nil
	}
	v := make(map[int64]*types.VirtualQueueState, len(t))
	for key := range t {
		v[key] = ToVirtualQueueState(t[key])
	}
	return v
}

func FromQueueState(t *types.QueueState) *shared.QueueState {
	if t == nil {
		return nil
	}
	return &shared.QueueState{
		VirtualQueueStates:    FromVirtualQueueStateMap(t.VirtualQueueStates),
		ExclusiveMaxReadLevel: FromTaskKey(t.ExclusiveMaxReadLevel),
	}
}

func ToQueueState(t *shared.QueueState) *types.QueueState {
	if t == nil {
		return nil
	}
	return &types.QueueState{
		VirtualQueueStates:    ToVirtualQueueStateMap(t.VirtualQueueStates),
		ExclusiveMaxReadLevel: ToTaskKey(t.ExclusiveMaxReadLevel),
	}
}

func FromCronOverlapPolicy(t *types.CronOverlapPolicy) *shared.CronOverlapPolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case types.CronOverlapPolicyBufferOne:
		return shared.CronOverlapPolicyBufferone.Ptr()
	case types.CronOverlapPolicySkipped:
		return shared.CronOverlapPolicySkipped.Ptr()
	}
	panic("unexpected enum value")
}

func ToCronOverlapPolicy(t *shared.CronOverlapPolicy) *types.CronOverlapPolicy {
	if t == nil {
		return nil
	}
	switch *t {
	case shared.CronOverlapPolicyBufferone:
		return types.CronOverlapPolicyBufferOne.Ptr()
	case shared.CronOverlapPolicySkipped:
		return types.CronOverlapPolicySkipped.Ptr()
	}
	panic("unexpected enum value")
}
