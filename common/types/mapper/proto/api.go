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

package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func FromActivityLocalDispatchInfo(t *types.ActivityLocalDispatchInfo) *apiv1.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityLocalDispatchInfo{
		ActivityId:                 t.ActivityID,
		ScheduledTime:              unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:                unixNanoToTime(t.StartedTimestamp),
		ScheduledTimeOfThisAttempt: unixNanoToTime(t.ScheduledTimestampOfThisAttempt),
		TaskToken:                  t.TaskToken,
	}
}

func ToActivityLocalDispatchInfo(t *apiv1.ActivityLocalDispatchInfo) *types.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	return &types.ActivityLocalDispatchInfo{
		ActivityID:                      t.ActivityId,
		ScheduledTimestamp:              timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:                timeToUnixNano(t.StartedTime),
		ScheduledTimestampOfThisAttempt: timeToUnixNano(t.ScheduledTimeOfThisAttempt),
		TaskToken:                       t.TaskToken,
	}
}

func FromActivityTaskCancelRequestedEventAttributes(t *types.ActivityTaskCancelRequestedEventAttributes) *apiv1.ActivityTaskCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskCancelRequestedEventAttributes{
		ActivityId:                   t.ActivityID,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToActivityTaskCancelRequestedEventAttributes(t *apiv1.ActivityTaskCancelRequestedEventAttributes) *types.ActivityTaskCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCancelRequestedEventAttributes{
		ActivityID:                   t.ActivityId,
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

func FromActivityTaskCanceledEventAttributes(t *types.ActivityTaskCanceledEventAttributes) *apiv1.ActivityTaskCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskCanceledEventAttributes{
		Details:                      FromPayload(t.Details),
		LatestCancelRequestedEventId: t.LatestCancelRequestedEventID,
		ScheduledEventId:             t.ScheduledEventID,
		StartedEventId:               t.StartedEventID,
		Identity:                     t.Identity,
	}
}

func ToActivityTaskCanceledEventAttributes(t *apiv1.ActivityTaskCanceledEventAttributes) *types.ActivityTaskCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCanceledEventAttributes{
		Details:                      ToPayload(t.Details),
		LatestCancelRequestedEventID: t.LatestCancelRequestedEventId,
		ScheduledEventID:             t.ScheduledEventId,
		StartedEventID:               t.StartedEventId,
		Identity:                     t.Identity,
	}
}

func FromActivityTaskCompletedEventAttributes(t *types.ActivityTaskCompletedEventAttributes) *apiv1.ActivityTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskCompletedEventAttributes{
		Result:           FromPayload(t.Result),
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		Identity:         t.Identity,
	}
}

func ToActivityTaskCompletedEventAttributes(t *apiv1.ActivityTaskCompletedEventAttributes) *types.ActivityTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskCompletedEventAttributes{
		Result:           ToPayload(t.Result),
		ScheduledEventID: t.ScheduledEventId,
		StartedEventID:   t.StartedEventId,
		Identity:         t.Identity,
	}
}

func FromActivityTaskFailedEventAttributes(t *types.ActivityTaskFailedEventAttributes) *apiv1.ActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskFailedEventAttributes{
		Failure:          FromFailure(t.Reason, t.Details),
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		Identity:         t.Identity,
	}
}

func ToActivityTaskFailedEventAttributes(t *apiv1.ActivityTaskFailedEventAttributes) *types.ActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskFailedEventAttributes{
		Reason:           ToFailureReason(t.Failure),
		Details:          ToFailureDetails(t.Failure),
		ScheduledEventID: t.ScheduledEventId,
		StartedEventID:   t.StartedEventId,
		Identity:         t.Identity,
	}
}

func FromActivityTaskScheduledEventAttributes(t *types.ActivityTaskScheduledEventAttributes) *apiv1.ActivityTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskScheduledEventAttributes{
		ActivityId:                   t.ActivityID,
		ActivityType:                 FromActivityType(t.ActivityType),
		Domain:                       t.GetDomain(),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ScheduleToCloseTimeout:       secondsToDuration(t.ScheduleToCloseTimeoutSeconds),
		ScheduleToStartTimeout:       secondsToDuration(t.ScheduleToStartTimeoutSeconds),
		StartToCloseTimeout:          secondsToDuration(t.StartToCloseTimeoutSeconds),
		HeartbeatTimeout:             secondsToDuration(t.HeartbeatTimeoutSeconds),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		Header:                       FromHeader(t.Header),
	}
}

func ToActivityTaskScheduledEventAttributes(t *apiv1.ActivityTaskScheduledEventAttributes) *types.ActivityTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskScheduledEventAttributes{
		ActivityID:                    t.ActivityId,
		ActivityType:                  ToActivityType(t.ActivityType),
		Domain:                        &t.Domain,
		TaskList:                      ToTaskList(t.TaskList),
		Input:                         ToPayload(t.Input),
		ScheduleToCloseTimeoutSeconds: durationToSeconds(t.ScheduleToCloseTimeout),
		ScheduleToStartTimeoutSeconds: durationToSeconds(t.ScheduleToStartTimeout),
		StartToCloseTimeoutSeconds:    durationToSeconds(t.StartToCloseTimeout),
		HeartbeatTimeoutSeconds:       durationToSeconds(t.HeartbeatTimeout),
		DecisionTaskCompletedEventID:  t.DecisionTaskCompletedEventId,
		RetryPolicy:                   ToRetryPolicy(t.RetryPolicy),
		Header:                        ToHeader(t.Header),
	}
}

func FromActivityTaskStartedEventAttributes(t *types.ActivityTaskStartedEventAttributes) *apiv1.ActivityTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskStartedEventAttributes{
		ScheduledEventId: t.ScheduledEventID,
		Identity:         t.Identity,
		RequestId:        t.RequestID,
		Attempt:          t.Attempt,
		LastFailure:      FromFailure(t.LastFailureReason, t.LastFailureDetails),
	}
}

func ToActivityTaskStartedEventAttributes(t *apiv1.ActivityTaskStartedEventAttributes) *types.ActivityTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskStartedEventAttributes{
		ScheduledEventID:   t.ScheduledEventId,
		Identity:           t.Identity,
		RequestID:          t.RequestId,
		Attempt:            t.Attempt,
		LastFailureReason:  ToFailureReason(t.LastFailure),
		LastFailureDetails: ToFailureDetails(t.LastFailure),
	}
}

func FromActivityTaskTimedOutEventAttributes(t *types.ActivityTaskTimedOutEventAttributes) *apiv1.ActivityTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityTaskTimedOutEventAttributes{
		Details:          FromPayload(t.Details),
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		TimeoutType:      FromTimeoutType(t.TimeoutType),
		LastFailure:      FromFailure(t.LastFailureReason, t.LastFailureDetails),
	}
}

func ToActivityTaskTimedOutEventAttributes(t *apiv1.ActivityTaskTimedOutEventAttributes) *types.ActivityTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ActivityTaskTimedOutEventAttributes{
		Details:            ToPayload(t.Details),
		ScheduledEventID:   t.ScheduledEventId,
		StartedEventID:     t.StartedEventId,
		TimeoutType:        ToTimeoutType(t.TimeoutType),
		LastFailureReason:  ToFailureReason(t.LastFailure),
		LastFailureDetails: ToFailureDetails(t.LastFailure),
	}
}

func FromActivityType(t *types.ActivityType) *apiv1.ActivityType {
	if t == nil {
		return nil
	}
	return &apiv1.ActivityType{
		Name: t.Name,
	}
}

func ToActivityType(t *apiv1.ActivityType) *types.ActivityType {
	if t == nil {
		return nil
	}
	return &types.ActivityType{
		Name: t.Name,
	}
}

func FromArchivalStatus(t *types.ArchivalStatus) apiv1.ArchivalStatus {
	if t == nil {
		return apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID
	}
	switch *t {
	case types.ArchivalStatusDisabled:
		return apiv1.ArchivalStatus_ARCHIVAL_STATUS_DISABLED
	case types.ArchivalStatusEnabled:
		return apiv1.ArchivalStatus_ARCHIVAL_STATUS_ENABLED
	}
	panic("unexpected enum value")
}

func ToArchivalStatus(t apiv1.ArchivalStatus) *types.ArchivalStatus {
	switch t {
	case apiv1.ArchivalStatus_ARCHIVAL_STATUS_INVALID:
		return nil
	case apiv1.ArchivalStatus_ARCHIVAL_STATUS_DISABLED:
		return types.ArchivalStatusDisabled.Ptr()
	case apiv1.ArchivalStatus_ARCHIVAL_STATUS_ENABLED:
		return types.ArchivalStatusEnabled.Ptr()
	}
	panic("unexpected enum value")
}

func FromBadBinaries(t *types.BadBinaries) *apiv1.BadBinaries {
	if t == nil {
		return nil
	}
	return &apiv1.BadBinaries{
		Binaries: FromBadBinaryInfoMap(t.Binaries),
	}
}

func ToBadBinaries(t *apiv1.BadBinaries) *types.BadBinaries {
	if t == nil {
		return nil
	}
	return &types.BadBinaries{
		Binaries: ToBadBinaryInfoMap(t.Binaries),
	}
}

func FromBadBinaryInfo(t *types.BadBinaryInfo) *apiv1.BadBinaryInfo {
	if t == nil {
		return nil
	}
	return &apiv1.BadBinaryInfo{
		Reason:      t.Reason,
		Operator:    t.Operator,
		CreatedTime: unixNanoToTime(t.CreatedTimeNano),
	}
}

func ToBadBinaryInfo(t *apiv1.BadBinaryInfo) *types.BadBinaryInfo {
	if t == nil {
		return nil
	}
	return &types.BadBinaryInfo{
		Reason:          t.Reason,
		Operator:        t.Operator,
		CreatedTimeNano: timeToUnixNano(t.CreatedTime),
	}
}

func FromCancelExternalWorkflowExecutionFailedCause(t *types.CancelExternalWorkflowExecutionFailedCause) apiv1.CancelExternalWorkflowExecutionFailedCause {
	if t == nil {
		return apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		return apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION
	case types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		return apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED
	}
	panic("unexpected enum value")
}

func ToCancelExternalWorkflowExecutionFailedCause(t apiv1.CancelExternalWorkflowExecutionFailedCause) *types.CancelExternalWorkflowExecutionFailedCause {
	switch t {
	case apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID:
		return nil
	case apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION:
		return types.CancelExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr()
	case apiv1.CancelExternalWorkflowExecutionFailedCause_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED:
		return types.CancelExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr()
	}
	panic("unexpected enum value")
}

func FromCancelTimerDecisionAttributes(t *types.CancelTimerDecisionAttributes) *apiv1.CancelTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.CancelTimerDecisionAttributes{
		TimerId: t.TimerID,
	}
}

func ToCancelTimerDecisionAttributes(t *apiv1.CancelTimerDecisionAttributes) *types.CancelTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelTimerDecisionAttributes{
		TimerID: t.TimerId,
	}
}

func FromCancelTimerFailedEventAttributes(t *types.CancelTimerFailedEventAttributes) *apiv1.CancelTimerFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.CancelTimerFailedEventAttributes{
		TimerId:                      t.TimerID,
		Cause:                        t.Cause,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Identity:                     t.Identity,
	}
}

func ToCancelTimerFailedEventAttributes(t *apiv1.CancelTimerFailedEventAttributes) *types.CancelTimerFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelTimerFailedEventAttributes{
		TimerID:                      t.TimerId,
		Cause:                        t.Cause,
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Identity:                     t.Identity,
	}
}

func FromCancelWorkflowExecutionDecisionAttributes(t *types.CancelWorkflowExecutionDecisionAttributes) *apiv1.CancelWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.CancelWorkflowExecutionDecisionAttributes{
		Details: FromPayload(t.Details),
	}
}

func ToCancelWorkflowExecutionDecisionAttributes(t *apiv1.CancelWorkflowExecutionDecisionAttributes) *types.CancelWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CancelWorkflowExecutionDecisionAttributes{
		Details: ToPayload(t.Details),
	}
}

func FromChildWorkflowExecutionCanceledEventAttributes(t *types.ChildWorkflowExecutionCanceledEventAttributes) *apiv1.ChildWorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionCanceledEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		StartedEventId:    t.StartedEventID,
		Details:           FromPayload(t.Details),
	}
}

func ToChildWorkflowExecutionCanceledEventAttributes(t *apiv1.ChildWorkflowExecutionCanceledEventAttributes) *types.ChildWorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionCanceledEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		StartedEventID:    t.StartedEventId,
		Details:           ToPayload(t.Details),
	}
}

func FromChildWorkflowExecutionCompletedEventAttributes(t *types.ChildWorkflowExecutionCompletedEventAttributes) *apiv1.ChildWorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionCompletedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		StartedEventId:    t.StartedEventID,
		Result:            FromPayload(t.Result),
	}
}

func ToChildWorkflowExecutionCompletedEventAttributes(t *apiv1.ChildWorkflowExecutionCompletedEventAttributes) *types.ChildWorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionCompletedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		StartedEventID:    t.StartedEventId,
		Result:            ToPayload(t.Result),
	}
}

func FromChildWorkflowExecutionFailedCause(t *types.ChildWorkflowExecutionFailedCause) apiv1.ChildWorkflowExecutionFailedCause {
	if t == nil {
		return apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning:
		return apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING
	}
	panic("unexpected enum value")
}

func ToChildWorkflowExecutionFailedCause(t apiv1.ChildWorkflowExecutionFailedCause) *types.ChildWorkflowExecutionFailedCause {
	switch t {
	case apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID:
		return nil
	case apiv1.ChildWorkflowExecutionFailedCause_CHILD_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_RUNNING:
		return types.ChildWorkflowExecutionFailedCauseWorkflowAlreadyRunning.Ptr()
	}
	panic("unexpected enum value")
}

func FromChildWorkflowExecutionFailedEventAttributes(t *types.ChildWorkflowExecutionFailedEventAttributes) *apiv1.ChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionFailedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		StartedEventId:    t.StartedEventID,
		Failure:           FromFailure(t.Reason, t.Details),
	}
}

func ToChildWorkflowExecutionFailedEventAttributes(t *apiv1.ChildWorkflowExecutionFailedEventAttributes) *types.ChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionFailedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		StartedEventID:    t.StartedEventId,
		Reason:            ToFailureReason(t.Failure),
		Details:           ToFailureDetails(t.Failure),
	}
}

func FromChildWorkflowExecutionStartedEventAttributes(t *types.ChildWorkflowExecutionStartedEventAttributes) *apiv1.ChildWorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		Header:            FromHeader(t.Header),
	}
}

func ToChildWorkflowExecutionStartedEventAttributes(t *apiv1.ChildWorkflowExecutionStartedEventAttributes) *types.ChildWorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionStartedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		Header:            ToHeader(t.Header),
	}
}

func FromChildWorkflowExecutionTerminatedEventAttributes(t *types.ChildWorkflowExecutionTerminatedEventAttributes) *apiv1.ChildWorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		StartedEventId:    t.StartedEventID,
	}
}

func ToChildWorkflowExecutionTerminatedEventAttributes(t *apiv1.ChildWorkflowExecutionTerminatedEventAttributes) *types.ChildWorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionTerminatedEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		StartedEventID:    t.StartedEventId,
	}
}

func FromChildWorkflowExecutionTimedOutEventAttributes(t *types.ChildWorkflowExecutionTimedOutEventAttributes) *apiv1.ChildWorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ChildWorkflowExecutionTimedOutEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      FromWorkflowType(t.WorkflowType),
		InitiatedEventId:  t.InitiatedEventID,
		StartedEventId:    t.StartedEventID,
		TimeoutType:       FromTimeoutType(t.TimeoutType),
	}
}

func ToChildWorkflowExecutionTimedOutEventAttributes(t *apiv1.ChildWorkflowExecutionTimedOutEventAttributes) *types.ChildWorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ChildWorkflowExecutionTimedOutEventAttributes{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:      ToWorkflowType(t.WorkflowType),
		InitiatedEventID:  t.InitiatedEventId,
		StartedEventID:    t.StartedEventId,
		TimeoutType:       ToTimeoutType(t.TimeoutType),
	}
}

func FromClusterReplicationConfiguration(t *types.ClusterReplicationConfiguration) *apiv1.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &apiv1.ClusterReplicationConfiguration{
		ClusterName: t.ClusterName,
	}
}

func ToClusterReplicationConfiguration(t *apiv1.ClusterReplicationConfiguration) *types.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	return &types.ClusterReplicationConfiguration{
		ClusterName: t.ClusterName,
	}
}

func FromCompleteWorkflowExecutionDecisionAttributes(t *types.CompleteWorkflowExecutionDecisionAttributes) *apiv1.CompleteWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.CompleteWorkflowExecutionDecisionAttributes{
		Result: FromPayload(t.Result),
	}
}

func ToCompleteWorkflowExecutionDecisionAttributes(t *apiv1.CompleteWorkflowExecutionDecisionAttributes) *types.CompleteWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.CompleteWorkflowExecutionDecisionAttributes{
		Result: ToPayload(t.Result),
	}
}

func FromContinueAsNewInitiator(t *types.ContinueAsNewInitiator) apiv1.ContinueAsNewInitiator {
	if t == nil {
		return apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID
	}
	switch *t {
	case types.ContinueAsNewInitiatorDecider:
		return apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_DECIDER
	case types.ContinueAsNewInitiatorRetryPolicy:
		return apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_RETRY_POLICY
	case types.ContinueAsNewInitiatorCronSchedule:
		return apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_CRON_SCHEDULE
	}
	panic("unexpected enum value")
}

func ToContinueAsNewInitiator(t apiv1.ContinueAsNewInitiator) *types.ContinueAsNewInitiator {
	switch t {
	case apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_INVALID:
		return nil
	case apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_DECIDER:
		return types.ContinueAsNewInitiatorDecider.Ptr()
	case apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_RETRY_POLICY:
		return types.ContinueAsNewInitiatorRetryPolicy.Ptr()
	case apiv1.ContinueAsNewInitiator_CONTINUE_AS_NEW_INITIATOR_CRON_SCHEDULE:
		return types.ContinueAsNewInitiatorCronSchedule.Ptr()
	}
	panic("unexpected enum value")
}

func FromContinueAsNewWorkflowExecutionDecisionAttributes(t *types.ContinueAsNewWorkflowExecutionDecisionAttributes) *apiv1.ContinueAsNewWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		BackoffStartInterval:         secondsToDuration(t.BackoffStartIntervalInSeconds),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		Initiator:                    FromContinueAsNewInitiator(t.Initiator),
		Failure:                      FromFailure(t.FailureReason, t.FailureDetails),
		LastCompletionResult:         FromPayload(t.LastCompletionResult),
		CronSchedule:                 t.CronSchedule,
		Header:                       FromHeader(t.Header),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
		JitterStart:                  secondsToDuration(t.JitterStartSeconds),
	}
}

func ToContinueAsNewWorkflowExecutionDecisionAttributes(t *apiv1.ContinueAsNewWorkflowExecutionDecisionAttributes) *types.ContinueAsNewWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		BackoffStartIntervalInSeconds:       durationToSeconds(t.BackoffStartInterval),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		FailureReason:                       ToFailureReason(t.Failure),
		FailureDetails:                      ToFailureDetails(t.Failure),
		LastCompletionResult:                ToPayload(t.LastCompletionResult),
		CronSchedule:                        t.CronSchedule,
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		JitterStartSeconds:                  durationToSeconds(t.JitterStart),
	}
}

func FromCountWorkflowExecutionsRequest(t *types.CountWorkflowExecutionsRequest) *apiv1.CountWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.CountWorkflowExecutionsRequest{
		Domain: t.Domain,
		Query:  t.Query,
	}
}

func ToCountWorkflowExecutionsRequest(t *apiv1.CountWorkflowExecutionsRequest) *types.CountWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.CountWorkflowExecutionsRequest{
		Domain: t.Domain,
		Query:  t.Query,
	}
}

func FromCountWorkflowExecutionsResponse(t *types.CountWorkflowExecutionsResponse) *apiv1.CountWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.CountWorkflowExecutionsResponse{
		Count: t.Count,
	}
}

func ToCountWorkflowExecutionsResponse(t *apiv1.CountWorkflowExecutionsResponse) *types.CountWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.CountWorkflowExecutionsResponse{
		Count: t.Count,
	}
}

func FromDataBlob(t *types.DataBlob) *apiv1.DataBlob {
	if t == nil {
		return nil
	}
	return &apiv1.DataBlob{
		EncodingType: FromEncodingType(t.EncodingType),
		Data:         t.Data,
	}
}

func ToDataBlob(t *apiv1.DataBlob) *types.DataBlob {
	if t == nil {
		return nil
	}
	return &types.DataBlob{
		EncodingType: ToEncodingType(t.EncodingType),
		Data:         t.Data,
	}
}

func FromDecisionTaskCompletedEventAttributes(t *types.DecisionTaskCompletedEventAttributes) *apiv1.DecisionTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.DecisionTaskCompletedEventAttributes{
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		Identity:         t.Identity,
		BinaryChecksum:   t.BinaryChecksum,
		ExecutionContext: t.ExecutionContext,
	}
}

func ToDecisionTaskCompletedEventAttributes(t *apiv1.DecisionTaskCompletedEventAttributes) *types.DecisionTaskCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskCompletedEventAttributes{
		ScheduledEventID: t.ScheduledEventId,
		StartedEventID:   t.StartedEventId,
		Identity:         t.Identity,
		BinaryChecksum:   t.BinaryChecksum,
		ExecutionContext: t.ExecutionContext,
	}
}

func FromDecisionTaskFailedCause(t *types.DecisionTaskFailedCause) apiv1.DecisionTaskFailedCause {
	if t == nil {
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.DecisionTaskFailedCauseUnhandledDecision:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_UNHANDLED_DECISION
	case types.DecisionTaskFailedCauseBadScheduleActivityAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SCHEDULE_ACTIVITY_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadStartTimerAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_START_TIMER_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadCancelTimerAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CANCEL_TIMER_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadRecordMarkerAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_RECORD_MARKER_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_COMPLETE_WORKFLOW_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_FAIL_WORKFLOW_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CANCEL_WORKFLOW_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadContinueAsNewAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CONTINUE_AS_NEW_ATTRIBUTES
	case types.DecisionTaskFailedCauseStartTimerDuplicateID:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_START_TIMER_DUPLICATE_ID
	case types.DecisionTaskFailedCauseResetStickyTasklist:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_RESET_STICKY_TASK_LIST
	case types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_WORKFLOW_WORKER_UNHANDLED_FAILURE
	case types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SIGNAL_WORKFLOW_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseBadStartChildExecutionAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_START_CHILD_EXECUTION_ATTRIBUTES
	case types.DecisionTaskFailedCauseForceCloseDecision:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_FORCE_CLOSE_DECISION
	case types.DecisionTaskFailedCauseFailoverCloseDecision:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_FAILOVER_CLOSE_DECISION
	case types.DecisionTaskFailedCauseBadSignalInputSize:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SIGNAL_INPUT_SIZE
	case types.DecisionTaskFailedCauseResetWorkflow:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_RESET_WORKFLOW
	case types.DecisionTaskFailedCauseBadBinary:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_BINARY
	case types.DecisionTaskFailedCauseScheduleActivityDuplicateID:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_SCHEDULE_ACTIVITY_DUPLICATE_ID
	case types.DecisionTaskFailedCauseBadSearchAttributes:
		return apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SEARCH_ATTRIBUTES
	}
	panic("unexpected enum value")
}

func ToDecisionTaskFailedCause(t apiv1.DecisionTaskFailedCause) *types.DecisionTaskFailedCause {
	switch t {
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_INVALID:
		return nil
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_UNHANDLED_DECISION:
		return types.DecisionTaskFailedCauseUnhandledDecision.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SCHEDULE_ACTIVITY_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadScheduleActivityAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_REQUEST_CANCEL_ACTIVITY_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadRequestCancelActivityAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_START_TIMER_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadStartTimerAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CANCEL_TIMER_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadCancelTimerAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_RECORD_MARKER_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadRecordMarkerAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_COMPLETE_WORKFLOW_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadCompleteWorkflowExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_FAIL_WORKFLOW_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadFailWorkflowExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CANCEL_WORKFLOW_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadCancelWorkflowExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_REQUEST_CANCEL_EXTERNAL_WORKFLOW_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadRequestCancelExternalWorkflowExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_CONTINUE_AS_NEW_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadContinueAsNewAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_START_TIMER_DUPLICATE_ID:
		return types.DecisionTaskFailedCauseStartTimerDuplicateID.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_RESET_STICKY_TASK_LIST:
		return types.DecisionTaskFailedCauseResetStickyTasklist.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_WORKFLOW_WORKER_UNHANDLED_FAILURE:
		return types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SIGNAL_WORKFLOW_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadSignalWorkflowExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_START_CHILD_EXECUTION_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadStartChildExecutionAttributes.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_FORCE_CLOSE_DECISION:
		return types.DecisionTaskFailedCauseForceCloseDecision.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_FAILOVER_CLOSE_DECISION:
		return types.DecisionTaskFailedCauseFailoverCloseDecision.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SIGNAL_INPUT_SIZE:
		return types.DecisionTaskFailedCauseBadSignalInputSize.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_RESET_WORKFLOW:
		return types.DecisionTaskFailedCauseResetWorkflow.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_BINARY:
		return types.DecisionTaskFailedCauseBadBinary.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_SCHEDULE_ACTIVITY_DUPLICATE_ID:
		return types.DecisionTaskFailedCauseScheduleActivityDuplicateID.Ptr()
	case apiv1.DecisionTaskFailedCause_DECISION_TASK_FAILED_CAUSE_BAD_SEARCH_ATTRIBUTES:
		return types.DecisionTaskFailedCauseBadSearchAttributes.Ptr()
	}
	panic("unexpected enum value")
}

func FromDecisionTaskFailedEventAttributes(t *types.DecisionTaskFailedEventAttributes) *apiv1.DecisionTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.DecisionTaskFailedEventAttributes{
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		Cause:            FromDecisionTaskFailedCause(t.Cause),
		Failure:          FromFailure(t.Reason, t.Details),
		Identity:         t.Identity,
		BaseRunId:        t.BaseRunID,
		NewRunId:         t.NewRunID,
		ForkEventVersion: t.ForkEventVersion,
		BinaryChecksum:   t.BinaryChecksum,
		RequestId:        t.RequestID,
	}
}

func ToDecisionTaskFailedEventAttributes(t *apiv1.DecisionTaskFailedEventAttributes) *types.DecisionTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskFailedEventAttributes{
		ScheduledEventID: t.ScheduledEventId,
		StartedEventID:   t.StartedEventId,
		Cause:            ToDecisionTaskFailedCause(t.Cause),
		Reason:           ToFailureReason(t.Failure),
		Details:          ToFailureDetails(t.Failure),
		Identity:         t.Identity,
		BaseRunID:        t.BaseRunId,
		NewRunID:         t.NewRunId,
		ForkEventVersion: t.ForkEventVersion,
		BinaryChecksum:   t.BinaryChecksum,
		RequestID:        t.RequestId,
	}
}

func FromDecisionTaskScheduledEventAttributes(t *types.DecisionTaskScheduledEventAttributes) *apiv1.DecisionTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.DecisionTaskScheduledEventAttributes{
		TaskList:            FromTaskList(t.TaskList),
		StartToCloseTimeout: secondsToDuration(t.StartToCloseTimeoutSeconds),
		Attempt:             int32(t.Attempt),
	}
}

func ToDecisionTaskScheduledEventAttributes(t *apiv1.DecisionTaskScheduledEventAttributes) *types.DecisionTaskScheduledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskScheduledEventAttributes{
		TaskList:                   ToTaskList(t.TaskList),
		StartToCloseTimeoutSeconds: durationToSeconds(t.StartToCloseTimeout),
		Attempt:                    int64(t.Attempt),
	}
}

func FromDecisionTaskStartedEventAttributes(t *types.DecisionTaskStartedEventAttributes) *apiv1.DecisionTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.DecisionTaskStartedEventAttributes{
		ScheduledEventId: t.ScheduledEventID,
		Identity:         t.Identity,
		RequestId:        t.RequestID,
	}
}

func ToDecisionTaskStartedEventAttributes(t *apiv1.DecisionTaskStartedEventAttributes) *types.DecisionTaskStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskStartedEventAttributes{
		ScheduledEventID: t.ScheduledEventId,
		Identity:         t.Identity,
		RequestID:        t.RequestId,
	}
}

func FromDecisionTaskTimedOutEventAttributes(t *types.DecisionTaskTimedOutEventAttributes) *apiv1.DecisionTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.DecisionTaskTimedOutEventAttributes{
		ScheduledEventId: t.ScheduledEventID,
		StartedEventId:   t.StartedEventID,
		TimeoutType:      FromTimeoutType(t.TimeoutType),
		BaseRunId:        t.BaseRunID,
		NewRunId:         t.NewRunID,
		ForkEventVersion: t.ForkEventVersion,
		Reason:           t.Reason,
		Cause:            FromDecisionTaskTimedOutCause(t.Cause),
		RequestId:        t.RequestID,
	}
}

func ToDecisionTaskTimedOutEventAttributes(t *apiv1.DecisionTaskTimedOutEventAttributes) *types.DecisionTaskTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.DecisionTaskTimedOutEventAttributes{
		ScheduledEventID: t.ScheduledEventId,
		StartedEventID:   t.StartedEventId,
		TimeoutType:      ToTimeoutType(t.TimeoutType),
		BaseRunID:        t.BaseRunId,
		NewRunID:         t.NewRunId,
		ForkEventVersion: t.ForkEventVersion,
		Reason:           t.Reason,
		Cause:            ToDecisionTaskTimedOutCause(t.Cause),
		RequestID:        t.RequestId,
	}
}

func FromDeprecateDomainRequest(t *types.DeprecateDomainRequest) *apiv1.DeprecateDomainRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DeprecateDomainRequest{
		Name:          t.Name,
		SecurityToken: t.SecurityToken,
	}
}

func ToDeprecateDomainRequest(t *apiv1.DeprecateDomainRequest) *types.DeprecateDomainRequest {
	if t == nil {
		return nil
	}
	return &types.DeprecateDomainRequest{
		Name:          t.Name,
		SecurityToken: t.SecurityToken,
	}
}

func FromDescribeDomainRequest(t *types.DescribeDomainRequest) *apiv1.DescribeDomainRequest {
	if t == nil {
		return nil
	}
	if t.UUID != nil {
		return &apiv1.DescribeDomainRequest{DescribeBy: &apiv1.DescribeDomainRequest_Id{Id: *t.UUID}}
	}
	if t.Name != nil {
		return &apiv1.DescribeDomainRequest{DescribeBy: &apiv1.DescribeDomainRequest_Name{Name: *t.Name}}
	}
	panic("neither oneof field is set for DescribeDomainRequest")
}

func ToDescribeDomainRequest(t *apiv1.DescribeDomainRequest) *types.DescribeDomainRequest {
	if t == nil {
		return nil
	}
	switch describeBy := t.DescribeBy.(type) {
	case *apiv1.DescribeDomainRequest_Id:
		return &types.DescribeDomainRequest{UUID: common.StringPtr(describeBy.Id)}
	case *apiv1.DescribeDomainRequest_Name:
		return &types.DescribeDomainRequest{Name: common.StringPtr(describeBy.Name)}
	}
	panic("neither oneof field is set for DescribeDomainRequest")
}

func FromDescribeDomainResponseDomain(t *types.DescribeDomainResponse) *apiv1.Domain {
	if t == nil {
		return nil
	}
	domain := apiv1.Domain{
		FailoverVersion: t.FailoverVersion,
		IsGlobalDomain:  t.IsGlobalDomain,
	}
	if info := t.DomainInfo; info != nil {
		domain.Id = info.UUID
		domain.Name = info.Name
		domain.Status = FromDomainStatus(info.Status)
		domain.Description = info.Description
		domain.OwnerEmail = info.OwnerEmail
		domain.Data = info.Data
	}
	if config := t.Configuration; config != nil {
		domain.IsolationGroups = FromIsolationGroupConfig(config.IsolationGroups)
		domain.WorkflowExecutionRetentionPeriod = daysToDuration(&config.WorkflowExecutionRetentionPeriodInDays)
		domain.BadBinaries = FromBadBinaries(config.BadBinaries)
		domain.HistoryArchivalStatus = FromArchivalStatus(config.HistoryArchivalStatus)
		domain.HistoryArchivalUri = config.HistoryArchivalURI
		domain.VisibilityArchivalStatus = FromArchivalStatus(config.VisibilityArchivalStatus)
		domain.VisibilityArchivalUri = config.VisibilityArchivalURI
		domain.AsyncWorkflowConfig = FromDomainAsyncWorkflowConfiguraton(config.AsyncWorkflowConfig)
	}
	if repl := t.ReplicationConfiguration; repl != nil {
		domain.ActiveClusterName = repl.ActiveClusterName
		domain.Clusters = FromClusterReplicationConfigurationArray(repl.Clusters)
	}
	if info := t.GetFailoverInfo(); info != nil {
		domain.FailoverInfo = FromFailoverInfo(t.GetFailoverInfo())
	}
	return &domain
}

func FromDescribeDomainResponse(t *types.DescribeDomainResponse) *apiv1.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeDomainResponse{
		Domain: FromDescribeDomainResponseDomain(t),
	}
}

func ToDescribeDomainResponseDomain(t *apiv1.Domain) *types.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name:        t.Name,
			Status:      ToDomainStatus(t.Status),
			Description: t.Description,
			OwnerEmail:  t.OwnerEmail,
			Data:        t.Data,
			UUID:        t.Id,
		},
		Configuration: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: common.Int32Default(durationToDays(t.WorkflowExecutionRetentionPeriod)),
			EmitMetric:                             true,
			BadBinaries:                            ToBadBinaries(t.BadBinaries),
			HistoryArchivalStatus:                  ToArchivalStatus(t.HistoryArchivalStatus),
			HistoryArchivalURI:                     t.HistoryArchivalUri,
			VisibilityArchivalStatus:               ToArchivalStatus(t.VisibilityArchivalStatus),
			VisibilityArchivalURI:                  t.VisibilityArchivalUri,
			IsolationGroups:                        ToIsolationGroupConfig(t.IsolationGroups),
			AsyncWorkflowConfig:                    ToDomainAsyncWorkflowConfiguraton(t.AsyncWorkflowConfig),
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: t.ActiveClusterName,
			Clusters:          ToClusterReplicationConfigurationArray(t.Clusters),
		},
		FailoverVersion: t.FailoverVersion,
		IsGlobalDomain:  t.IsGlobalDomain,
		FailoverInfo:    ToFailoverInfo(t.FailoverInfo),
	}
}

func ToDescribeDomainResponse(t *apiv1.DescribeDomainResponse) *types.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	response := ToDescribeDomainResponseDomain(t.Domain)
	return response
}

func FromFailoverInfo(t *types.FailoverInfo) *apiv1.FailoverInfo {
	if t == nil {
		return nil
	}
	return &apiv1.FailoverInfo{
		FailoverVersion:         t.GetFailoverVersion(),
		FailoverStartTimestamp:  unixNanoToTime(&t.FailoverStartTimestamp),
		FailoverExpireTimestamp: unixNanoToTime(&t.FailoverExpireTimestamp),
		CompletedShardCount:     t.GetCompletedShardCount(),
		PendingShards:           t.GetPendingShards(),
	}
}

func ToFailoverInfo(t *apiv1.FailoverInfo) *types.FailoverInfo {
	if t == nil {
		return nil
	}
	return &types.FailoverInfo{
		FailoverVersion:         t.GetFailoverVersion(),
		FailoverStartTimestamp:  *timeToUnixNano(t.GetFailoverStartTimestamp()),
		FailoverExpireTimestamp: *timeToUnixNano(t.GetFailoverExpireTimestamp()),
		CompletedShardCount:     t.GetCompletedShardCount(),
		PendingShards:           t.GetPendingShards(),
	}
}

func FromDescribeTaskListRequest(t *types.DescribeTaskListRequest) *apiv1.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeTaskListRequest{
		Domain:                t.Domain,
		TaskList:              FromTaskList(t.TaskList),
		TaskListType:          FromTaskListType(t.TaskListType),
		IncludeTaskListStatus: t.IncludeTaskListStatus,
	}
}

func ToDescribeTaskListRequest(t *apiv1.DescribeTaskListRequest) *types.DescribeTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeTaskListRequest{
		Domain:                t.Domain,
		TaskList:              ToTaskList(t.TaskList),
		TaskListType:          ToTaskListType(t.TaskListType),
		IncludeTaskListStatus: t.IncludeTaskListStatus,
	}
}

func FromDescribeTaskListResponse(t *types.DescribeTaskListResponse) *apiv1.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeTaskListResponse{
		Pollers:         FromPollerInfoArray(t.Pollers),
		TaskListStatus:  FromTaskListStatus(t.TaskListStatus),
		PartitionConfig: FromAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func ToDescribeTaskListResponse(t *apiv1.DescribeTaskListResponse) *types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeTaskListResponse{
		Pollers:         ToPollerInfoArray(t.Pollers),
		TaskListStatus:  ToTaskListStatus(t.TaskListStatus),
		PartitionConfig: ToAPITaskListPartitionConfig(t.PartitionConfig),
	}
}

func FromDescribeWorkflowExecutionRequest(t *types.DescribeWorkflowExecutionRequest) *apiv1.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func ToDescribeWorkflowExecutionRequest(t *apiv1.DescribeWorkflowExecutionRequest) *types.DescribeWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeWorkflowExecutionRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromDescribeWorkflowExecutionResponse(t *types.DescribeWorkflowExecutionResponse) *apiv1.DescribeWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeWorkflowExecutionResponse{
		ExecutionConfiguration: FromWorkflowExecutionConfiguration(t.ExecutionConfiguration),
		WorkflowExecutionInfo:  FromWorkflowExecutionInfo(t.WorkflowExecutionInfo),
		PendingActivities:      FromPendingActivityInfoArray(t.PendingActivities),
		PendingChildren:        FromPendingChildExecutionInfoArray(t.PendingChildren),
		PendingDecision:        FromPendingDecisionInfo(t.PendingDecision),
	}
}

func ToDescribeWorkflowExecutionResponse(t *apiv1.DescribeWorkflowExecutionResponse) *types.DescribeWorkflowExecutionResponse {
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

func FromDiagnoseWorkflowExecutionRequest(t *types.DiagnoseWorkflowExecutionRequest) *apiv1.DiagnoseWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DiagnoseWorkflowExecutionRequest{
		Domain:            t.GetDomain(),
		WorkflowExecution: FromWorkflowExecution(t.GetWorkflowExecution()),
	}
}

func ToDiagnoseWorkflowExecutionRequest(t *apiv1.DiagnoseWorkflowExecutionRequest) *types.DiagnoseWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.DiagnoseWorkflowExecutionRequest{
		Domain:            t.GetDomain(),
		WorkflowExecution: ToWorkflowExecution(t.GetWorkflowExecution()),
		Identity:          t.Identity,
	}
}

func FromDiagnoseWorkflowExecutionResponse(t *types.DiagnoseWorkflowExecutionResponse) *apiv1.DiagnoseWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DiagnoseWorkflowExecutionResponse{
		Domain:                      t.GetDomain(),
		DiagnosticWorkflowExecution: FromWorkflowExecution(t.GetDiagnosticWorkflowExecution()),
	}
}

func ToDiagnoseWorkflowExecutionResponse(t *apiv1.DiagnoseWorkflowExecutionResponse) *types.DiagnoseWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.DiagnoseWorkflowExecutionResponse{
		Domain:                      t.GetDomain(),
		DiagnosticWorkflowExecution: ToWorkflowExecution(t.GetDiagnosticWorkflowExecution()),
	}
}

func FromDomainStatus(t *types.DomainStatus) apiv1.DomainStatus {
	if t == nil {
		return apiv1.DomainStatus_DOMAIN_STATUS_INVALID
	}
	switch *t {
	case types.DomainStatusRegistered:
		return apiv1.DomainStatus_DOMAIN_STATUS_REGISTERED
	case types.DomainStatusDeprecated:
		return apiv1.DomainStatus_DOMAIN_STATUS_DEPRECATED
	case types.DomainStatusDeleted:
		return apiv1.DomainStatus_DOMAIN_STATUS_DELETED
	}
	panic("unexpected enum value")
}

func ToDomainStatus(t apiv1.DomainStatus) *types.DomainStatus {
	switch t {
	case apiv1.DomainStatus_DOMAIN_STATUS_INVALID:
		return nil
	case apiv1.DomainStatus_DOMAIN_STATUS_REGISTERED:
		return types.DomainStatusRegistered.Ptr()
	case apiv1.DomainStatus_DOMAIN_STATUS_DEPRECATED:
		return types.DomainStatusDeprecated.Ptr()
	case apiv1.DomainStatus_DOMAIN_STATUS_DELETED:
		return types.DomainStatusDeleted.Ptr()
	}
	panic("unexpected enum value")
}

func FromEncodingType(t *types.EncodingType) apiv1.EncodingType {
	if t == nil {
		return apiv1.EncodingType_ENCODING_TYPE_INVALID
	}
	switch *t {
	case types.EncodingTypeThriftRW:
		return apiv1.EncodingType_ENCODING_TYPE_THRIFTRW
	case types.EncodingTypeJSON:
		return apiv1.EncodingType_ENCODING_TYPE_JSON
	}
	panic("unexpected enum value")
}

func ToEncodingType(t apiv1.EncodingType) *types.EncodingType {
	switch t {
	case apiv1.EncodingType_ENCODING_TYPE_INVALID:
		return nil
	case apiv1.EncodingType_ENCODING_TYPE_THRIFTRW:
		return types.EncodingTypeThriftRW.Ptr()
	case apiv1.EncodingType_ENCODING_TYPE_JSON:
		return types.EncodingTypeJSON.Ptr()
	case apiv1.EncodingType_ENCODING_TYPE_PROTO3:
		panic("not supported yet")
	}
	panic("unexpected enum value")
}

func FromEventFilterType(t *types.HistoryEventFilterType) apiv1.EventFilterType {
	if t == nil {
		return apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID
	}
	switch *t {
	case types.HistoryEventFilterTypeAllEvent:
		return apiv1.EventFilterType_EVENT_FILTER_TYPE_ALL_EVENT
	case types.HistoryEventFilterTypeCloseEvent:
		return apiv1.EventFilterType_EVENT_FILTER_TYPE_CLOSE_EVENT
	}
	panic("unexpected enum value")
}

func ToEventFilterType(t apiv1.EventFilterType) *types.HistoryEventFilterType {
	switch t {
	case apiv1.EventFilterType_EVENT_FILTER_TYPE_INVALID:
		return nil
	case apiv1.EventFilterType_EVENT_FILTER_TYPE_ALL_EVENT:
		return types.HistoryEventFilterTypeAllEvent.Ptr()
	case apiv1.EventFilterType_EVENT_FILTER_TYPE_CLOSE_EVENT:
		return types.HistoryEventFilterTypeCloseEvent.Ptr()
	}
	panic("unexpected enum value")
}

func FromExternalWorkflowExecutionCancelRequestedEventAttributes(t *types.ExternalWorkflowExecutionCancelRequestedEventAttributes) *apiv1.ExternalWorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventId:  t.InitiatedEventID,
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
	}
}

func ToExternalWorkflowExecutionCancelRequestedEventAttributes(t *apiv1.ExternalWorkflowExecutionCancelRequestedEventAttributes) *types.ExternalWorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ExternalWorkflowExecutionCancelRequestedEventAttributes{
		InitiatedEventID:  t.InitiatedEventId,
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromExternalWorkflowExecutionSignaledEventAttributes(t *types.ExternalWorkflowExecutionSignaledEventAttributes) *apiv1.ExternalWorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventId:  t.InitiatedEventID,
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Control:           t.Control,
	}
}

func ToExternalWorkflowExecutionSignaledEventAttributes(t *apiv1.ExternalWorkflowExecutionSignaledEventAttributes) *types.ExternalWorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.ExternalWorkflowExecutionSignaledEventAttributes{
		InitiatedEventID:  t.InitiatedEventId,
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Control:           t.Control,
	}
}

func FromFailWorkflowExecutionDecisionAttributes(t *types.FailWorkflowExecutionDecisionAttributes) *apiv1.FailWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.FailWorkflowExecutionDecisionAttributes{
		Failure: FromFailure(t.Reason, t.Details),
	}
}

func ToFailWorkflowExecutionDecisionAttributes(t *apiv1.FailWorkflowExecutionDecisionAttributes) *types.FailWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.FailWorkflowExecutionDecisionAttributes{
		Reason:  ToFailureReason(t.Failure),
		Details: ToFailureDetails(t.Failure),
	}
}

func FromGetClusterInfoResponse(t *types.ClusterInfo) *apiv1.GetClusterInfoResponse {
	if t == nil {
		return nil
	}
	return &apiv1.GetClusterInfoResponse{
		SupportedClientVersions: FromSupportedClientVersions(t.SupportedClientVersions),
	}
}

func ToGetClusterInfoResponse(t *apiv1.GetClusterInfoResponse) *types.ClusterInfo {
	if t == nil {
		return nil
	}
	return &types.ClusterInfo{
		SupportedClientVersions: ToSupportedClientVersions(t.SupportedClientVersions),
	}
}

func FromGetSearchAttributesResponse(t *types.GetSearchAttributesResponse) *apiv1.GetSearchAttributesResponse {
	if t == nil {
		return nil
	}
	return &apiv1.GetSearchAttributesResponse{
		Keys: FromIndexedValueTypeMap(t.Keys),
	}
}

func ToGetSearchAttributesResponse(t *apiv1.GetSearchAttributesResponse) *types.GetSearchAttributesResponse {
	if t == nil {
		return nil
	}
	return &types.GetSearchAttributesResponse{
		Keys: ToIndexedValueTypeMap(t.Keys),
	}
}

func FromGetWorkflowExecutionHistoryRequest(t *types.GetWorkflowExecutionHistoryRequest) *apiv1.GetWorkflowExecutionHistoryRequest {
	if t == nil {
		return nil
	}
	return &apiv1.GetWorkflowExecutionHistoryRequest{
		Domain:                 t.Domain,
		WorkflowExecution:      FromWorkflowExecution(t.Execution),
		PageSize:               t.MaximumPageSize,
		NextPageToken:          t.NextPageToken,
		WaitForNewEvent:        t.WaitForNewEvent,
		HistoryEventFilterType: FromEventFilterType(t.HistoryEventFilterType),
		SkipArchival:           t.SkipArchival,
	}
}

func ToGetWorkflowExecutionHistoryRequest(t *apiv1.GetWorkflowExecutionHistoryRequest) *types.GetWorkflowExecutionHistoryRequest {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionHistoryRequest{
		Domain:                 t.Domain,
		Execution:              ToWorkflowExecution(t.WorkflowExecution),
		MaximumPageSize:        t.PageSize,
		NextPageToken:          t.NextPageToken,
		WaitForNewEvent:        t.WaitForNewEvent,
		HistoryEventFilterType: ToEventFilterType(t.HistoryEventFilterType),
		SkipArchival:           t.SkipArchival,
	}
}

func FromGetWorkflowExecutionHistoryResponse(t *types.GetWorkflowExecutionHistoryResponse) *apiv1.GetWorkflowExecutionHistoryResponse {
	if t == nil {
		return nil
	}
	return &apiv1.GetWorkflowExecutionHistoryResponse{
		History:       FromHistory(t.History),
		RawHistory:    FromDataBlobArray(t.RawHistory),
		NextPageToken: t.NextPageToken,
		Archived:      t.Archived,
	}
}

func ToGetWorkflowExecutionHistoryResponse(t *apiv1.GetWorkflowExecutionHistoryResponse) *types.GetWorkflowExecutionHistoryResponse {
	if t == nil {
		return nil
	}
	return &types.GetWorkflowExecutionHistoryResponse{
		History:       ToHistory(t.History),
		RawHistory:    ToDataBlobArray(t.RawHistory),
		NextPageToken: t.NextPageToken,
		Archived:      t.Archived,
	}
}

func FromHeader(t *types.Header) *apiv1.Header {
	if t == nil {
		return nil
	}
	return &apiv1.Header{
		Fields: FromPayloadMap(t.Fields),
	}
}

func ToHeader(t *apiv1.Header) *types.Header {
	if t == nil {
		return nil
	}
	return &types.Header{
		Fields: ToPayloadMap(t.Fields),
	}
}

func FromHealthResponse(t *types.HealthStatus) *apiv1.HealthResponse {
	if t == nil {
		return nil
	}
	return &apiv1.HealthResponse{
		Ok:      t.Ok,
		Message: t.Msg,
	}
}

func ToHealthResponse(t *apiv1.HealthResponse) *types.HealthStatus {
	if t == nil {
		return nil
	}
	return &types.HealthStatus{
		Ok:  t.Ok,
		Msg: t.Message,
	}
}

func FromHistory(t *types.History) *apiv1.History {
	if t == nil {
		return nil
	}
	return &apiv1.History{
		Events: FromHistoryEventArray(t.Events),
	}
}

func ToHistory(t *apiv1.History) *types.History {
	if t == nil {
		return nil
	}
	return &types.History{
		Events: ToHistoryEventArray(t.Events),
	}
}

func FromIndexedValueType(t types.IndexedValueType) apiv1.IndexedValueType {
	switch t {
	case types.IndexedValueTypeString:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_STRING
	case types.IndexedValueTypeKeyword:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_KEYWORD
	case types.IndexedValueTypeInt:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INT
	case types.IndexedValueTypeDouble:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_DOUBLE
	case types.IndexedValueTypeBool:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_BOOL
	case types.IndexedValueTypeDatetime:
		return apiv1.IndexedValueType_INDEXED_VALUE_TYPE_DATETIME
	}
	panic("unexpected enum value")
}

func ToIndexedValueType(t apiv1.IndexedValueType) types.IndexedValueType {
	switch t {
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INVALID:
		panic("received IndexedValueType_INDEXED_VALUE_TYPE_INVALID")
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_STRING:
		return types.IndexedValueTypeString
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_KEYWORD:
		return types.IndexedValueTypeKeyword
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_INT:
		return types.IndexedValueTypeInt
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_DOUBLE:
		return types.IndexedValueTypeDouble
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_BOOL:
		return types.IndexedValueTypeBool
	case apiv1.IndexedValueType_INDEXED_VALUE_TYPE_DATETIME:
		return types.IndexedValueTypeDatetime
	}
	panic("unexpected enum value")
}

func FromListArchivedWorkflowExecutionsRequest(t *types.ListArchivedWorkflowExecutionsRequest) *apiv1.ListArchivedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListArchivedWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func ToListArchivedWorkflowExecutionsRequest(t *apiv1.ListArchivedWorkflowExecutionsRequest) *types.ListArchivedWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListArchivedWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func FromListArchivedWorkflowExecutionsResponse(t *types.ListArchivedWorkflowExecutionsResponse) *apiv1.ListArchivedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListArchivedWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func ToListArchivedWorkflowExecutionsResponse(t *apiv1.ListArchivedWorkflowExecutionsResponse) *types.ListArchivedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListArchivedWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func FromListClosedWorkflowExecutionsResponse(t *types.ListClosedWorkflowExecutionsResponse) *apiv1.ListClosedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListClosedWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func ToListClosedWorkflowExecutionsResponse(t *apiv1.ListClosedWorkflowExecutionsResponse) *types.ListClosedWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListClosedWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func FromListDomainsRequest(t *types.ListDomainsRequest) *apiv1.ListDomainsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListDomainsRequest{
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func ToListDomainsRequest(t *apiv1.ListDomainsRequest) *types.ListDomainsRequest {
	if t == nil {
		return nil
	}
	return &types.ListDomainsRequest{
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func FromListDomainsResponse(t *types.ListDomainsResponse) *apiv1.ListDomainsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListDomainsResponse{
		Domains:       FromDescribeDomainResponseArray(t.Domains),
		NextPageToken: t.NextPageToken,
	}
}

func ToListDomainsResponse(t *apiv1.ListDomainsResponse) *types.ListDomainsResponse {
	if t == nil {
		return nil
	}
	return &types.ListDomainsResponse{
		Domains:       ToDescribeDomainResponseArray(t.Domains),
		NextPageToken: t.NextPageToken,
	}
}

func FromListOpenWorkflowExecutionsResponse(t *types.ListOpenWorkflowExecutionsResponse) *apiv1.ListOpenWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListOpenWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func ToListOpenWorkflowExecutionsResponse(t *apiv1.ListOpenWorkflowExecutionsResponse) *types.ListOpenWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListOpenWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func FromListTaskListPartitionsRequest(t *types.ListTaskListPartitionsRequest) *apiv1.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListTaskListPartitionsRequest{
		Domain:   t.Domain,
		TaskList: FromTaskList(t.TaskList),
	}
}

func ToListTaskListPartitionsRequest(t *apiv1.ListTaskListPartitionsRequest) *types.ListTaskListPartitionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListTaskListPartitionsRequest{
		Domain:   t.Domain,
		TaskList: ToTaskList(t.TaskList),
	}
}

func FromListTaskListPartitionsResponse(t *types.ListTaskListPartitionsResponse) *apiv1.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: FromTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: FromTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

func ToListTaskListPartitionsResponse(t *apiv1.ListTaskListPartitionsResponse) *types.ListTaskListPartitionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListTaskListPartitionsResponse{
		ActivityTaskListPartitions: ToTaskListPartitionMetadataArray(t.ActivityTaskListPartitions),
		DecisionTaskListPartitions: ToTaskListPartitionMetadataArray(t.DecisionTaskListPartitions),
	}
}

func FromGetTaskListsByDomainRequest(t *types.GetTaskListsByDomainRequest) *apiv1.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &apiv1.GetTaskListsByDomainRequest{
		Domain: t.Domain,
	}
}

func ToGetTaskListsByDomainRequest(t *apiv1.GetTaskListsByDomainRequest) *types.GetTaskListsByDomainRequest {
	if t == nil {
		return nil
	}
	return &types.GetTaskListsByDomainRequest{
		Domain: t.Domain,
	}
}

func FromGetTaskListsByDomainResponse(t *types.GetTaskListsByDomainResponse) *apiv1.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &apiv1.GetTaskListsByDomainResponse{
		DecisionTaskListMap: FromDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: FromDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

func ToGetTaskListsByDomainResponse(t *apiv1.GetTaskListsByDomainResponse) *types.GetTaskListsByDomainResponse {
	if t == nil {
		return nil
	}

	return &types.GetTaskListsByDomainResponse{
		DecisionTaskListMap: ToDescribeTaskListResponseMap(t.GetDecisionTaskListMap()),
		ActivityTaskListMap: ToDescribeTaskListResponseMap(t.GetActivityTaskListMap()),
	}
}

func FromDescribeTaskListResponseMap(t map[string]*types.DescribeTaskListResponse) map[string]*apiv1.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*apiv1.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = FromDescribeTaskListResponse(value)
	}
	return taskListMap
}

func ToDescribeTaskListResponseMap(t map[string]*apiv1.DescribeTaskListResponse) map[string]*types.DescribeTaskListResponse {
	if t == nil {
		return nil
	}
	taskListMap := make(map[string]*types.DescribeTaskListResponse, len(t))
	for key, value := range t {
		taskListMap[key] = ToDescribeTaskListResponse(value)
	}
	return taskListMap
}

func FromListWorkflowExecutionsRequest(t *types.ListWorkflowExecutionsRequest) *apiv1.ListWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func ToListWorkflowExecutionsRequest(t *apiv1.ListWorkflowExecutionsRequest) *types.ListWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func FromListWorkflowExecutionsResponse(t *types.ListWorkflowExecutionsResponse) *apiv1.ListWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func ToListWorkflowExecutionsResponse(t *apiv1.ListWorkflowExecutionsResponse) *types.ListWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func FromMarkerRecordedEventAttributes(t *types.MarkerRecordedEventAttributes) *apiv1.MarkerRecordedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.MarkerRecordedEventAttributes{
		MarkerName:                   t.MarkerName,
		Details:                      FromPayload(t.Details),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Header:                       FromHeader(t.Header),
	}
}

func ToMarkerRecordedEventAttributes(t *apiv1.MarkerRecordedEventAttributes) *types.MarkerRecordedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.MarkerRecordedEventAttributes{
		MarkerName:                   t.MarkerName,
		Details:                      ToPayload(t.Details),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Header:                       ToHeader(t.Header),
	}
}

func FromMemo(t *types.Memo) *apiv1.Memo {
	if t == nil {
		return nil
	}
	return &apiv1.Memo{
		Fields: FromPayloadMap(t.Fields),
	}
}

func ToMemo(t *apiv1.Memo) *types.Memo {
	if t == nil {
		return nil
	}
	return &types.Memo{
		Fields: ToPayloadMap(t.Fields),
	}
}

func FromParentClosePolicy(t *types.ParentClosePolicy) apiv1.ParentClosePolicy {
	if t == nil {
		return apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID
	}
	switch *t {
	case types.ParentClosePolicyAbandon:
		return apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_ABANDON
	case types.ParentClosePolicyRequestCancel:
		return apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_REQUEST_CANCEL
	case types.ParentClosePolicyTerminate:
		return apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_TERMINATE
	}
	panic("unexpected enum value")
}

func ToParentClosePolicy(t apiv1.ParentClosePolicy) *types.ParentClosePolicy {
	switch t {
	case apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_INVALID:
		return nil
	case apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_ABANDON:
		return types.ParentClosePolicyAbandon.Ptr()
	case apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_REQUEST_CANCEL:
		return types.ParentClosePolicyRequestCancel.Ptr()
	case apiv1.ParentClosePolicy_PARENT_CLOSE_POLICY_TERMINATE:
		return types.ParentClosePolicyTerminate.Ptr()
	}
	panic("unexpected enum value")
}

func FromPendingActivityInfo(t *types.PendingActivityInfo) *apiv1.PendingActivityInfo {
	if t == nil {
		return nil
	}
	return &apiv1.PendingActivityInfo{
		ActivityId:            t.ActivityID,
		ActivityType:          FromActivityType(t.ActivityType),
		State:                 FromPendingActivityState(t.State),
		HeartbeatDetails:      FromPayload(t.HeartbeatDetails),
		LastHeartbeatTime:     unixNanoToTime(t.LastHeartbeatTimestamp),
		LastStartedTime:       unixNanoToTime(t.LastStartedTimestamp),
		Attempt:               t.Attempt,
		MaximumAttempts:       t.MaximumAttempts,
		ScheduledTime:         unixNanoToTime(t.ScheduledTimestamp),
		ExpirationTime:        unixNanoToTime(t.ExpirationTimestamp),
		LastFailure:           FromFailure(t.LastFailureReason, t.LastFailureDetails),
		LastWorkerIdentity:    t.LastWorkerIdentity,
		StartedWorkerIdentity: t.StartedWorkerIdentity,
		ScheduleId:            t.ScheduleID,
	}
}

func ToPendingActivityInfo(t *apiv1.PendingActivityInfo) *types.PendingActivityInfo {
	if t == nil {
		return nil
	}
	return &types.PendingActivityInfo{
		ActivityID:             t.ActivityId,
		ActivityType:           ToActivityType(t.ActivityType),
		State:                  ToPendingActivityState(t.State),
		HeartbeatDetails:       ToPayload(t.HeartbeatDetails),
		LastHeartbeatTimestamp: timeToUnixNano(t.LastHeartbeatTime),
		LastStartedTimestamp:   timeToUnixNano(t.LastStartedTime),
		Attempt:                t.Attempt,
		MaximumAttempts:        t.MaximumAttempts,
		ScheduledTimestamp:     timeToUnixNano(t.ScheduledTime),
		ExpirationTimestamp:    timeToUnixNano(t.ExpirationTime),
		LastFailureReason:      ToFailureReason(t.LastFailure),
		LastFailureDetails:     ToFailureDetails(t.LastFailure),
		LastWorkerIdentity:     t.LastWorkerIdentity,
		StartedWorkerIdentity:  t.StartedWorkerIdentity,
		ScheduleID:             t.ScheduleId,
	}
}

func FromPendingActivityState(t *types.PendingActivityState) apiv1.PendingActivityState {
	if t == nil {
		return apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID
	}
	switch *t {
	case types.PendingActivityStateScheduled:
		return apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_SCHEDULED
	case types.PendingActivityStateStarted:
		return apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_STARTED
	case types.PendingActivityStateCancelRequested:
		return apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_CANCEL_REQUESTED
	}
	panic("unexpected enum value")
}

func ToPendingActivityState(t apiv1.PendingActivityState) *types.PendingActivityState {
	switch t {
	case apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_INVALID:
		return nil
	case apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_SCHEDULED:
		return types.PendingActivityStateScheduled.Ptr()
	case apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_STARTED:
		return types.PendingActivityStateStarted.Ptr()
	case apiv1.PendingActivityState_PENDING_ACTIVITY_STATE_CANCEL_REQUESTED:
		return types.PendingActivityStateCancelRequested.Ptr()
	}
	panic("unexpected enum value")
}

func FromPendingChildExecutionInfo(t *types.PendingChildExecutionInfo) *apiv1.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	return &apiv1.PendingChildExecutionInfo{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		WorkflowTypeName:  t.WorkflowTypeName,
		InitiatedId:       t.InitiatedID,
		ParentClosePolicy: FromParentClosePolicy(t.ParentClosePolicy),
	}
}

func ToPendingChildExecutionInfo(t *apiv1.PendingChildExecutionInfo) *types.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	return &types.PendingChildExecutionInfo{
		Domain:            t.Domain,
		WorkflowID:        ToWorkflowID(t.WorkflowExecution),
		RunID:             ToRunID(t.WorkflowExecution),
		WorkflowTypeName:  t.WorkflowTypeName,
		InitiatedID:       t.InitiatedId,
		ParentClosePolicy: ToParentClosePolicy(t.ParentClosePolicy),
	}
}

func FromPendingDecisionInfo(t *types.PendingDecisionInfo) *apiv1.PendingDecisionInfo {
	if t == nil {
		return nil
	}
	return &apiv1.PendingDecisionInfo{
		State:                 FromPendingDecisionState(t.State),
		ScheduledTime:         unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:           unixNanoToTime(t.StartedTimestamp),
		Attempt:               int32(t.Attempt),
		OriginalScheduledTime: unixNanoToTime(t.OriginalScheduledTimestamp),
		ScheduleId:            t.ScheduleID,
	}
}

func ToPendingDecisionInfo(t *apiv1.PendingDecisionInfo) *types.PendingDecisionInfo {
	if t == nil {
		return nil
	}
	return &types.PendingDecisionInfo{
		State:                      ToPendingDecisionState(t.State),
		ScheduledTimestamp:         timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:           timeToUnixNano(t.StartedTime),
		Attempt:                    int64(t.Attempt),
		OriginalScheduledTimestamp: timeToUnixNano(t.OriginalScheduledTime),
		ScheduleID:                 t.ScheduleId,
	}
}

func FromPendingDecisionState(t *types.PendingDecisionState) apiv1.PendingDecisionState {
	if t == nil {
		return apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID
	}
	switch *t {
	case types.PendingDecisionStateScheduled:
		return apiv1.PendingDecisionState_PENDING_DECISION_STATE_SCHEDULED
	case types.PendingDecisionStateStarted:
		return apiv1.PendingDecisionState_PENDING_DECISION_STATE_STARTED
	}
	panic("unexpected enum value")
}

func ToPendingDecisionState(t apiv1.PendingDecisionState) *types.PendingDecisionState {
	switch t {
	case apiv1.PendingDecisionState_PENDING_DECISION_STATE_INVALID:
		return nil
	case apiv1.PendingDecisionState_PENDING_DECISION_STATE_SCHEDULED:
		return types.PendingDecisionStateScheduled.Ptr()
	case apiv1.PendingDecisionState_PENDING_DECISION_STATE_STARTED:
		return types.PendingDecisionStateStarted.Ptr()
	}
	panic("unexpected enum value")
}

func FromPollForActivityTaskRequest(t *types.PollForActivityTaskRequest) *apiv1.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &apiv1.PollForActivityTaskRequest{
		Domain:           t.Domain,
		TaskList:         FromTaskList(t.TaskList),
		Identity:         t.Identity,
		TaskListMetadata: FromTaskListMetadata(t.TaskListMetadata),
	}
}

func ToPollForActivityTaskRequest(t *apiv1.PollForActivityTaskRequest) *types.PollForActivityTaskRequest {
	if t == nil {
		return nil
	}
	return &types.PollForActivityTaskRequest{
		Domain:           t.Domain,
		TaskList:         ToTaskList(t.TaskList),
		Identity:         t.Identity,
		TaskListMetadata: ToTaskListMetadata(t.TaskListMetadata),
	}
}

func FromPollForActivityTaskResponse(t *types.PollForActivityTaskResponse) *apiv1.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &apiv1.PollForActivityTaskResponse{
		TaskToken:                  t.TaskToken,
		WorkflowExecution:          FromWorkflowExecution(t.WorkflowExecution),
		ActivityId:                 t.ActivityID,
		ActivityType:               FromActivityType(t.ActivityType),
		Input:                      FromPayload(t.Input),
		ScheduledTime:              unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:                unixNanoToTime(t.StartedTimestamp),
		ScheduleToCloseTimeout:     secondsToDuration(t.ScheduleToCloseTimeoutSeconds),
		StartToCloseTimeout:        secondsToDuration(t.StartToCloseTimeoutSeconds),
		HeartbeatTimeout:           secondsToDuration(t.HeartbeatTimeoutSeconds),
		Attempt:                    t.Attempt,
		ScheduledTimeOfThisAttempt: unixNanoToTime(t.ScheduledTimestampOfThisAttempt),
		HeartbeatDetails:           FromPayload(t.HeartbeatDetails),
		WorkflowType:               FromWorkflowType(t.WorkflowType),
		WorkflowDomain:             t.WorkflowDomain,
		Header:                     FromHeader(t.Header),
		AutoConfigHint:             FromAutoConfigHint(t.AutoConfigHint),
	}
}

func ToPollForActivityTaskResponse(t *apiv1.PollForActivityTaskResponse) *types.PollForActivityTaskResponse {
	if t == nil {
		return nil
	}
	return &types.PollForActivityTaskResponse{
		TaskToken:                       t.TaskToken,
		WorkflowExecution:               ToWorkflowExecution(t.WorkflowExecution),
		ActivityID:                      t.ActivityId,
		ActivityType:                    ToActivityType(t.ActivityType),
		Input:                           ToPayload(t.Input),
		ScheduledTimestamp:              timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:                timeToUnixNano(t.StartedTime),
		ScheduleToCloseTimeoutSeconds:   durationToSeconds(t.ScheduleToCloseTimeout),
		StartToCloseTimeoutSeconds:      durationToSeconds(t.StartToCloseTimeout),
		HeartbeatTimeoutSeconds:         durationToSeconds(t.HeartbeatTimeout),
		Attempt:                         t.Attempt,
		ScheduledTimestampOfThisAttempt: timeToUnixNano(t.ScheduledTimeOfThisAttempt),
		HeartbeatDetails:                ToPayload(t.HeartbeatDetails),
		WorkflowType:                    ToWorkflowType(t.WorkflowType),
		WorkflowDomain:                  t.WorkflowDomain,
		Header:                          ToHeader(t.Header),
		AutoConfigHint:                  ToAutoConfigHint(t.AutoConfigHint),
	}
}

func FromPollForDecisionTaskRequest(t *types.PollForDecisionTaskRequest) *apiv1.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &apiv1.PollForDecisionTaskRequest{
		Domain:         t.Domain,
		TaskList:       FromTaskList(t.TaskList),
		Identity:       t.Identity,
		BinaryChecksum: t.BinaryChecksum,
	}
}

func ToPollForDecisionTaskRequest(t *apiv1.PollForDecisionTaskRequest) *types.PollForDecisionTaskRequest {
	if t == nil {
		return nil
	}
	return &types.PollForDecisionTaskRequest{
		Domain:         t.Domain,
		TaskList:       ToTaskList(t.TaskList),
		Identity:       t.Identity,
		BinaryChecksum: t.BinaryChecksum,
	}
}

func FromPollForDecisionTaskResponse(t *types.PollForDecisionTaskResponse) *apiv1.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &apiv1.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         FromWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              FromWorkflowType(t.WorkflowType),
		PreviousStartedEventId:    fromInt64Value(t.PreviousStartedEventID),
		StartedEventId:            t.StartedEventID,
		Attempt:                   t.Attempt,
		BacklogCountHint:          t.BacklogCountHint,
		History:                   FromHistory(t.History),
		NextPageToken:             t.NextPageToken,
		Query:                     FromWorkflowQuery(t.Query),
		WorkflowExecutionTaskList: FromTaskList(t.WorkflowExecutionTaskList),
		ScheduledTime:             unixNanoToTime(t.ScheduledTimestamp),
		StartedTime:               unixNanoToTime(t.StartedTimestamp),
		Queries:                   FromWorkflowQueryMap(t.Queries),
		NextEventId:               t.NextEventID,
		TotalHistoryBytes:         t.TotalHistoryBytes,
		AutoConfigHint:            FromAutoConfigHint(t.AutoConfigHint),
	}
}

func ToPollForDecisionTaskResponse(t *apiv1.PollForDecisionTaskResponse) *types.PollForDecisionTaskResponse {
	if t == nil {
		return nil
	}
	return &types.PollForDecisionTaskResponse{
		TaskToken:                 t.TaskToken,
		WorkflowExecution:         ToWorkflowExecution(t.WorkflowExecution),
		WorkflowType:              ToWorkflowType(t.WorkflowType),
		PreviousStartedEventID:    toInt64Value(t.PreviousStartedEventId),
		StartedEventID:            t.StartedEventId,
		Attempt:                   t.Attempt,
		BacklogCountHint:          t.BacklogCountHint,
		History:                   ToHistory(t.History),
		NextPageToken:             t.NextPageToken,
		Query:                     ToWorkflowQuery(t.Query),
		WorkflowExecutionTaskList: ToTaskList(t.WorkflowExecutionTaskList),
		ScheduledTimestamp:        timeToUnixNano(t.ScheduledTime),
		StartedTimestamp:          timeToUnixNano(t.StartedTime),
		Queries:                   ToWorkflowQueryMap(t.Queries),
		NextEventID:               t.NextEventId,
		TotalHistoryBytes:         t.TotalHistoryBytes,
		AutoConfigHint:            ToAutoConfigHint(t.AutoConfigHint),
	}
}

func FromPollerInfo(t *types.PollerInfo) *apiv1.PollerInfo {
	if t == nil {
		return nil
	}
	return &apiv1.PollerInfo{
		LastAccessTime: unixNanoToTime(t.LastAccessTime),
		Identity:       t.Identity,
		RatePerSecond:  t.RatePerSecond,
	}
}

func ToPollerInfo(t *apiv1.PollerInfo) *types.PollerInfo {
	if t == nil {
		return nil
	}
	return &types.PollerInfo{
		LastAccessTime: timeToUnixNano(t.LastAccessTime),
		Identity:       t.Identity,
		RatePerSecond:  t.RatePerSecond,
	}
}

func FromQueryConsistencyLevel(t *types.QueryConsistencyLevel) apiv1.QueryConsistencyLevel {
	if t == nil {
		return apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID
	}
	switch *t {
	case types.QueryConsistencyLevelEventual:
		return apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_EVENTUAL
	case types.QueryConsistencyLevelStrong:
		return apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_STRONG
	}
	panic("unexpected enum value")
}

func ToQueryConsistencyLevel(t apiv1.QueryConsistencyLevel) *types.QueryConsistencyLevel {
	switch t {
	case apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_INVALID:
		return nil
	case apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_EVENTUAL:
		return types.QueryConsistencyLevelEventual.Ptr()
	case apiv1.QueryConsistencyLevel_QUERY_CONSISTENCY_LEVEL_STRONG:
		return types.QueryConsistencyLevelStrong.Ptr()
	}
	panic("unexpected enum value")
}

func FromQueryRejectCondition(t *types.QueryRejectCondition) apiv1.QueryRejectCondition {
	if t == nil {
		return apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID
	}
	switch *t {
	case types.QueryRejectConditionNotOpen:
		return apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_OPEN
	case types.QueryRejectConditionNotCompletedCleanly:
		return apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_COMPLETED_CLEANLY
	}
	panic("unexpected enum value")
}

func ToQueryRejectCondition(t apiv1.QueryRejectCondition) *types.QueryRejectCondition {
	switch t {
	case apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_INVALID:
		return nil
	case apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_OPEN:
		return types.QueryRejectConditionNotOpen.Ptr()
	case apiv1.QueryRejectCondition_QUERY_REJECT_CONDITION_NOT_COMPLETED_CLEANLY:
		return types.QueryRejectConditionNotCompletedCleanly.Ptr()
	}
	panic("unexpected enum value")
}

func FromQueryRejected(t *types.QueryRejected) *apiv1.QueryRejected {
	if t == nil {
		return nil
	}
	return &apiv1.QueryRejected{
		CloseStatus: FromWorkflowExecutionCloseStatus(t.CloseStatus),
	}
}

func ToQueryRejected(t *apiv1.QueryRejected) *types.QueryRejected {
	if t == nil {
		return nil
	}
	return &types.QueryRejected{
		CloseStatus: ToWorkflowExecutionCloseStatus(t.CloseStatus),
	}
}

func FromQueryResultType(t *types.QueryResultType) apiv1.QueryResultType {
	if t == nil {
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID
	}
	switch *t {
	case types.QueryResultTypeAnswered:
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED
	case types.QueryResultTypeFailed:
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_FAILED
	}
	panic("unexpected enum value")
}

func ToQueryResultType(t apiv1.QueryResultType) *types.QueryResultType {
	switch t {
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID:
		return nil
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED:
		return types.QueryResultTypeAnswered.Ptr()
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_FAILED:
		return types.QueryResultTypeFailed.Ptr()
	}
	panic("unexpected enum value")
}

func FromQueryWorkflowRequest(t *types.QueryWorkflowRequest) *apiv1.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &apiv1.QueryWorkflowRequest{
		Domain:                t.Domain,
		WorkflowExecution:     FromWorkflowExecution(t.Execution),
		Query:                 FromWorkflowQuery(t.Query),
		QueryRejectCondition:  FromQueryRejectCondition(t.QueryRejectCondition),
		QueryConsistencyLevel: FromQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

func ToQueryWorkflowRequest(t *apiv1.QueryWorkflowRequest) *types.QueryWorkflowRequest {
	if t == nil {
		return nil
	}
	return &types.QueryWorkflowRequest{
		Domain:                t.Domain,
		Execution:             ToWorkflowExecution(t.WorkflowExecution),
		Query:                 ToWorkflowQuery(t.Query),
		QueryRejectCondition:  ToQueryRejectCondition(t.QueryRejectCondition),
		QueryConsistencyLevel: ToQueryConsistencyLevel(t.QueryConsistencyLevel),
	}
}

func FromQueryWorkflowResponse(t *types.QueryWorkflowResponse) *apiv1.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &apiv1.QueryWorkflowResponse{
		QueryResult:   FromPayload(t.QueryResult),
		QueryRejected: FromQueryRejected(t.QueryRejected),
	}
}

func ToQueryWorkflowResponse(t *apiv1.QueryWorkflowResponse) *types.QueryWorkflowResponse {
	if t == nil {
		return nil
	}
	return &types.QueryWorkflowResponse{
		QueryResult:   ToPayload(t.QueryResult),
		QueryRejected: ToQueryRejected(t.QueryRejected),
	}
}

func FromRecordActivityTaskHeartbeatByIDRequest(t *types.RecordActivityTaskHeartbeatByIDRequest) *apiv1.RecordActivityTaskHeartbeatByIDRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RecordActivityTaskHeartbeatByIDRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		ActivityId:        t.ActivityID,
		Details:           FromPayload(t.Details),
		Identity:          t.Identity,
	}
}

func ToRecordActivityTaskHeartbeatByIDRequest(t *apiv1.RecordActivityTaskHeartbeatByIDRequest) *types.RecordActivityTaskHeartbeatByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatByIDRequest{
		Domain:     t.Domain,
		WorkflowID: ToWorkflowID(t.WorkflowExecution),
		RunID:      ToRunID(t.WorkflowExecution),
		ActivityID: t.ActivityId,
		Details:    ToPayload(t.Details),
		Identity:   t.Identity,
	}
}

func FromRecordActivityTaskHeartbeatByIDResponse(t *types.RecordActivityTaskHeartbeatResponse) *apiv1.RecordActivityTaskHeartbeatByIDResponse {
	if t == nil {
		return nil
	}
	return &apiv1.RecordActivityTaskHeartbeatByIDResponse{
		CancelRequested: t.CancelRequested,
	}
}

func ToRecordActivityTaskHeartbeatByIDResponse(t *apiv1.RecordActivityTaskHeartbeatByIDResponse) *types.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.CancelRequested,
	}
}

func FromRecordActivityTaskHeartbeatRequest(t *types.RecordActivityTaskHeartbeatRequest) *apiv1.RecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RecordActivityTaskHeartbeatRequest{
		TaskToken: t.TaskToken,
		Details:   FromPayload(t.Details),
		Identity:  t.Identity,
	}
}

func ToRecordActivityTaskHeartbeatRequest(t *apiv1.RecordActivityTaskHeartbeatRequest) *types.RecordActivityTaskHeartbeatRequest {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatRequest{
		TaskToken: t.TaskToken,
		Details:   ToPayload(t.Details),
		Identity:  t.Identity,
	}
}

func FromRecordActivityTaskHeartbeatResponse(t *types.RecordActivityTaskHeartbeatResponse) *apiv1.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &apiv1.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.CancelRequested,
	}
}

func ToRecordActivityTaskHeartbeatResponse(t *apiv1.RecordActivityTaskHeartbeatResponse) *types.RecordActivityTaskHeartbeatResponse {
	if t == nil {
		return nil
	}
	return &types.RecordActivityTaskHeartbeatResponse{
		CancelRequested: t.CancelRequested,
	}
}

func FromRecordMarkerDecisionAttributes(t *types.RecordMarkerDecisionAttributes) *apiv1.RecordMarkerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RecordMarkerDecisionAttributes{
		MarkerName: t.MarkerName,
		Details:    FromPayload(t.Details),
		Header:     FromHeader(t.Header),
	}
}

func ToRecordMarkerDecisionAttributes(t *apiv1.RecordMarkerDecisionAttributes) *types.RecordMarkerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RecordMarkerDecisionAttributes{
		MarkerName: t.MarkerName,
		Details:    ToPayload(t.Details),
		Header:     ToHeader(t.Header),
	}
}

func FromRegisterDomainRequest(t *types.RegisterDomainRequest) *apiv1.RegisterDomainRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RegisterDomainRequest{
		Name:                             t.Name,
		Description:                      t.Description,
		OwnerEmail:                       t.OwnerEmail,
		WorkflowExecutionRetentionPeriod: daysToDuration(&t.WorkflowExecutionRetentionPeriodInDays),
		Clusters:                         FromClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusterName:                t.ActiveClusterName,
		Data:                             t.Data,
		SecurityToken:                    t.SecurityToken,
		IsGlobalDomain:                   t.IsGlobalDomain,
		HistoryArchivalStatus:            FromArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalUri:               t.HistoryArchivalURI,
		VisibilityArchivalStatus:         FromArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalUri:            t.VisibilityArchivalURI,
	}
}

func ToRegisterDomainRequest(t *apiv1.RegisterDomainRequest) *types.RegisterDomainRequest {
	if t == nil {
		return nil
	}
	return &types.RegisterDomainRequest{
		Name:                                   t.Name,
		Description:                            t.Description,
		OwnerEmail:                             t.OwnerEmail,
		WorkflowExecutionRetentionPeriodInDays: *durationToDays(t.WorkflowExecutionRetentionPeriod),
		EmitMetric:                             common.BoolPtr(true),
		Clusters:                               ToClusterReplicationConfigurationArray(t.Clusters),
		ActiveClusterName:                      t.ActiveClusterName,
		Data:                                   t.Data,
		SecurityToken:                          t.SecurityToken,
		IsGlobalDomain:                         t.IsGlobalDomain,
		HistoryArchivalStatus:                  ToArchivalStatus(t.HistoryArchivalStatus),
		HistoryArchivalURI:                     t.HistoryArchivalUri,
		VisibilityArchivalStatus:               ToArchivalStatus(t.VisibilityArchivalStatus),
		VisibilityArchivalURI:                  t.VisibilityArchivalUri,
	}
}

func FromRequestCancelActivityTaskDecisionAttributes(t *types.RequestCancelActivityTaskDecisionAttributes) *apiv1.RequestCancelActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelActivityTaskDecisionAttributes{
		ActivityId: t.ActivityID,
	}
}

func ToRequestCancelActivityTaskDecisionAttributes(t *apiv1.RequestCancelActivityTaskDecisionAttributes) *types.RequestCancelActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelActivityTaskDecisionAttributes{
		ActivityID: t.ActivityId,
	}
}

func FromRequestCancelActivityTaskFailedEventAttributes(t *types.RequestCancelActivityTaskFailedEventAttributes) *apiv1.RequestCancelActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelActivityTaskFailedEventAttributes{
		ActivityId:                   t.ActivityID,
		Cause:                        t.Cause,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToRequestCancelActivityTaskFailedEventAttributes(t *apiv1.RequestCancelActivityTaskFailedEventAttributes) *types.RequestCancelActivityTaskFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelActivityTaskFailedEventAttributes{
		ActivityID:                   t.ActivityId,
		Cause:                        t.Cause,
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

func FromRequestCancelExternalWorkflowExecutionDecisionAttributes(t *types.RequestCancelExternalWorkflowExecutionDecisionAttributes) *apiv1.RequestCancelExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		Control:           t.Control,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
	}
}

func ToRequestCancelExternalWorkflowExecutionDecisionAttributes(t *apiv1.RequestCancelExternalWorkflowExecutionDecisionAttributes) *types.RequestCancelExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.Domain,
		WorkflowID:        ToWorkflowID(t.WorkflowExecution),
		RunID:             ToRunID(t.WorkflowExecution),
		Control:           t.Control,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
	}
}

func FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(t *types.RequestCancelExternalWorkflowExecutionFailedEventAttributes) *apiv1.RequestCancelExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        FromCancelExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Domain:                       t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		InitiatedEventId:             t.InitiatedEventID,
		Control:                      t.Control,
	}
}

func ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(t *apiv1.RequestCancelExternalWorkflowExecutionFailedEventAttributes) *types.RequestCancelExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        ToCancelExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Domain:                       t.Domain,
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		InitiatedEventID:             t.InitiatedEventId,
		Control:                      t.Control,
	}
}

func FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t *types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) *apiv1.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Domain:                       t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		Control:                      t.Control,
		ChildWorkflowOnly:            t.ChildWorkflowOnly,
	}
}

func ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(t *apiv1.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes) *types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Domain:                       t.Domain,
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		Control:                      t.Control,
		ChildWorkflowOnly:            t.ChildWorkflowOnly,
	}
}

func FromRequestCancelWorkflowExecutionRequest(t *types.RequestCancelWorkflowExecutionRequest) *apiv1.RequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RequestCancelWorkflowExecutionRequest{
		Domain:              t.Domain,
		WorkflowExecution:   FromWorkflowExecution(t.WorkflowExecution),
		Identity:            t.Identity,
		RequestId:           t.RequestID,
		Cause:               t.Cause,
		FirstExecutionRunId: t.FirstExecutionRunID,
	}
}

func ToRequestCancelWorkflowExecutionRequest(t *apiv1.RequestCancelWorkflowExecutionRequest) *types.RequestCancelWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.RequestCancelWorkflowExecutionRequest{
		Domain:              t.Domain,
		WorkflowExecution:   ToWorkflowExecution(t.WorkflowExecution),
		Identity:            t.Identity,
		RequestID:           t.RequestId,
		Cause:               t.Cause,
		FirstExecutionRunID: t.FirstExecutionRunId,
	}
}

func FromResetPointInfo(t *types.ResetPointInfo) *apiv1.ResetPointInfo {
	if t == nil {
		return nil
	}
	return &apiv1.ResetPointInfo{
		BinaryChecksum:           t.BinaryChecksum,
		RunId:                    t.RunID,
		FirstDecisionCompletedId: t.FirstDecisionCompletedID,
		CreatedTime:              unixNanoToTime(t.CreatedTimeNano),
		ExpiringTime:             unixNanoToTime(t.ExpiringTimeNano),
		Resettable:               t.Resettable,
	}
}

func ToResetPointInfo(t *apiv1.ResetPointInfo) *types.ResetPointInfo {
	if t == nil {
		return nil
	}
	return &types.ResetPointInfo{
		BinaryChecksum:           t.BinaryChecksum,
		RunID:                    t.RunId,
		FirstDecisionCompletedID: t.FirstDecisionCompletedId,
		CreatedTimeNano:          timeToUnixNano(t.CreatedTime),
		ExpiringTimeNano:         timeToUnixNano(t.ExpiringTime),
		Resettable:               t.Resettable,
	}
}

func FromResetPoints(t *types.ResetPoints) *apiv1.ResetPoints {
	if t == nil {
		return nil
	}
	return &apiv1.ResetPoints{
		Points: FromResetPointInfoArray(t.Points),
	}
}

func ToResetPoints(t *apiv1.ResetPoints) *types.ResetPoints {
	if t == nil {
		return nil
	}
	return &types.ResetPoints{
		Points: ToResetPointInfoArray(t.Points),
	}
}

func FromResetStickyTaskListRequest(t *types.ResetStickyTaskListRequest) *apiv1.ResetStickyTaskListRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ResetStickyTaskListRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func ToResetStickyTaskListRequest(t *apiv1.ResetStickyTaskListRequest) *types.ResetStickyTaskListRequest {
	if t == nil {
		return nil
	}
	return &types.ResetStickyTaskListRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromResetWorkflowExecutionRequest(t *types.ResetWorkflowExecutionRequest) *apiv1.ResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ResetWorkflowExecutionRequest{
		Domain:                t.Domain,
		WorkflowExecution:     FromWorkflowExecution(t.WorkflowExecution),
		Reason:                t.Reason,
		DecisionFinishEventId: t.DecisionFinishEventID,
		RequestId:             t.RequestID,
		SkipSignalReapply:     t.SkipSignalReapply,
	}
}

func ToResetWorkflowExecutionRequest(t *apiv1.ResetWorkflowExecutionRequest) *types.ResetWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.ResetWorkflowExecutionRequest{
		Domain:                t.Domain,
		WorkflowExecution:     ToWorkflowExecution(t.WorkflowExecution),
		Reason:                t.Reason,
		DecisionFinishEventID: t.DecisionFinishEventId,
		RequestID:             t.RequestId,
		SkipSignalReapply:     t.SkipSignalReapply,
	}
}

func FromResetWorkflowExecutionResponse(t *types.ResetWorkflowExecutionResponse) *apiv1.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ResetWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToResetWorkflowExecutionResponse(t *apiv1.ResetWorkflowExecutionResponse) *types.ResetWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.ResetWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func ToRefreshWorkflowTasksRequest(t *apiv1.RefreshWorkflowTasksRequest) *types.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &types.RefreshWorkflowTasksRequest{
		Domain:    t.Domain,
		Execution: ToWorkflowExecution(t.WorkflowExecution),
	}
}

func FromRefreshWorkflowTasksRequest(t *types.RefreshWorkflowTasksRequest) *apiv1.RefreshWorkflowTasksRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RefreshWorkflowTasksRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
	}
}

func FromRespondActivityTaskCanceledByIDRequest(t *types.RespondActivityTaskCanceledByIDRequest) *apiv1.RespondActivityTaskCanceledByIDRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskCanceledByIDRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		ActivityId:        t.ActivityID,
		Details:           FromPayload(t.Details),
		Identity:          t.Identity,
	}
}

func ToRespondActivityTaskCanceledByIDRequest(t *apiv1.RespondActivityTaskCanceledByIDRequest) *types.RespondActivityTaskCanceledByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCanceledByIDRequest{
		Domain:     t.Domain,
		WorkflowID: ToWorkflowID(t.WorkflowExecution),
		RunID:      ToRunID(t.WorkflowExecution),
		ActivityID: t.ActivityId,
		Details:    ToPayload(t.Details),
		Identity:   t.Identity,
	}
}

func FromRespondActivityTaskCanceledRequest(t *types.RespondActivityTaskCanceledRequest) *apiv1.RespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskCanceledRequest{
		TaskToken: t.TaskToken,
		Details:   FromPayload(t.Details),
		Identity:  t.Identity,
	}
}

func ToRespondActivityTaskCanceledRequest(t *apiv1.RespondActivityTaskCanceledRequest) *types.RespondActivityTaskCanceledRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCanceledRequest{
		TaskToken: t.TaskToken,
		Details:   ToPayload(t.Details),
		Identity:  t.Identity,
	}
}

func FromRespondActivityTaskCompletedByIDRequest(t *types.RespondActivityTaskCompletedByIDRequest) *apiv1.RespondActivityTaskCompletedByIDRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskCompletedByIDRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		ActivityId:        t.ActivityID,
		Result:            FromPayload(t.Result),
		Identity:          t.Identity,
	}
}

func ToRespondActivityTaskCompletedByIDRequest(t *apiv1.RespondActivityTaskCompletedByIDRequest) *types.RespondActivityTaskCompletedByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCompletedByIDRequest{
		Domain:     t.Domain,
		WorkflowID: ToWorkflowID(t.WorkflowExecution),
		RunID:      ToRunID(t.WorkflowExecution),
		ActivityID: t.ActivityId,
		Result:     ToPayload(t.Result),
		Identity:   t.Identity,
	}
}

func FromRespondActivityTaskCompletedRequest(t *types.RespondActivityTaskCompletedRequest) *apiv1.RespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskCompletedRequest{
		TaskToken: t.TaskToken,
		Result:    FromPayload(t.Result),
		Identity:  t.Identity,
	}
}

func ToRespondActivityTaskCompletedRequest(t *apiv1.RespondActivityTaskCompletedRequest) *types.RespondActivityTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskCompletedRequest{
		TaskToken: t.TaskToken,
		Result:    ToPayload(t.Result),
		Identity:  t.Identity,
	}
}

func FromRespondActivityTaskFailedByIDRequest(t *types.RespondActivityTaskFailedByIDRequest) *apiv1.RespondActivityTaskFailedByIDRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskFailedByIDRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowRunPair(t.WorkflowID, t.RunID),
		ActivityId:        t.ActivityID,
		Failure:           FromFailure(t.Reason, t.Details),
		Identity:          t.Identity,
	}
}

func ToRespondActivityTaskFailedByIDRequest(t *apiv1.RespondActivityTaskFailedByIDRequest) *types.RespondActivityTaskFailedByIDRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskFailedByIDRequest{
		Domain:     t.Domain,
		WorkflowID: ToWorkflowID(t.WorkflowExecution),
		RunID:      ToRunID(t.WorkflowExecution),
		ActivityID: t.ActivityId,
		Reason:     ToFailureReason(t.Failure),
		Details:    ToFailureDetails(t.Failure),
		Identity:   t.Identity,
	}
}

func FromRespondActivityTaskFailedRequest(t *types.RespondActivityTaskFailedRequest) *apiv1.RespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondActivityTaskFailedRequest{
		TaskToken: t.TaskToken,
		Failure:   FromFailure(t.Reason, t.Details),
		Identity:  t.Identity,
	}
}

func ToRespondActivityTaskFailedRequest(t *apiv1.RespondActivityTaskFailedRequest) *types.RespondActivityTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondActivityTaskFailedRequest{
		TaskToken: t.TaskToken,
		Reason:    ToFailureReason(t.Failure),
		Details:   ToFailureDetails(t.Failure),
		Identity:  t.Identity,
	}
}

func FromRespondDecisionTaskCompletedRequest(t *types.RespondDecisionTaskCompletedRequest) *apiv1.RespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondDecisionTaskCompletedRequest{
		TaskToken:                  t.TaskToken,
		Decisions:                  FromDecisionArray(t.Decisions),
		ExecutionContext:           t.ExecutionContext,
		Identity:                   t.Identity,
		StickyAttributes:           FromStickyExecutionAttributes(t.StickyAttributes),
		ReturnNewDecisionTask:      t.ReturnNewDecisionTask,
		ForceCreateNewDecisionTask: t.ForceCreateNewDecisionTask,
		BinaryChecksum:             t.BinaryChecksum,
		QueryResults:               FromWorkflowQueryResultMap(t.QueryResults),
	}
}

func ToRespondDecisionTaskCompletedRequest(t *apiv1.RespondDecisionTaskCompletedRequest) *types.RespondDecisionTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskCompletedRequest{
		TaskToken:                  t.TaskToken,
		Decisions:                  ToDecisionArray(t.Decisions),
		ExecutionContext:           t.ExecutionContext,
		Identity:                   t.Identity,
		StickyAttributes:           ToStickyExecutionAttributes(t.StickyAttributes),
		ReturnNewDecisionTask:      t.ReturnNewDecisionTask,
		ForceCreateNewDecisionTask: t.ForceCreateNewDecisionTask,
		BinaryChecksum:             t.BinaryChecksum,
		QueryResults:               ToWorkflowQueryResultMap(t.QueryResults),
	}
}

func FromRespondDecisionTaskCompletedResponse(t *types.RespondDecisionTaskCompletedResponse) *apiv1.RespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &apiv1.RespondDecisionTaskCompletedResponse{
		DecisionTask:                FromPollForDecisionTaskResponse(t.DecisionTask),
		ActivitiesToDispatchLocally: FromActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

func ToRespondDecisionTaskCompletedResponse(t *apiv1.RespondDecisionTaskCompletedResponse) *types.RespondDecisionTaskCompletedResponse {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskCompletedResponse{
		DecisionTask:                ToPollForDecisionTaskResponse(t.DecisionTask),
		ActivitiesToDispatchLocally: ToActivityLocalDispatchInfoMap(t.ActivitiesToDispatchLocally),
	}
}

func FromRespondDecisionTaskFailedRequest(t *types.RespondDecisionTaskFailedRequest) *apiv1.RespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondDecisionTaskFailedRequest{
		TaskToken:      t.TaskToken,
		Cause:          FromDecisionTaskFailedCause(t.Cause),
		Details:        FromPayload(t.Details),
		Identity:       t.Identity,
		BinaryChecksum: t.BinaryChecksum,
	}
}

func ToRespondDecisionTaskFailedRequest(t *apiv1.RespondDecisionTaskFailedRequest) *types.RespondDecisionTaskFailedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondDecisionTaskFailedRequest{
		TaskToken:      t.TaskToken,
		Cause:          ToDecisionTaskFailedCause(t.Cause),
		Details:        ToPayload(t.Details),
		Identity:       t.Identity,
		BinaryChecksum: t.BinaryChecksum,
	}
}

func FromRespondQueryTaskCompletedRequest(t *types.RespondQueryTaskCompletedRequest) *apiv1.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RespondQueryTaskCompletedRequest{
		TaskToken: t.TaskToken,
		Result: &apiv1.WorkflowQueryResult{
			ResultType:   FromQueryTaskCompletedType(t.CompletedType),
			Answer:       FromPayload(t.QueryResult),
			ErrorMessage: t.ErrorMessage,
		},
		WorkerVersionInfo: FromWorkerVersionInfo(t.WorkerVersionInfo),
	}
}

func ToRespondQueryTaskCompletedRequest(t *apiv1.RespondQueryTaskCompletedRequest) *types.RespondQueryTaskCompletedRequest {
	if t == nil {
		return nil
	}
	return &types.RespondQueryTaskCompletedRequest{
		TaskToken:         t.TaskToken,
		CompletedType:     ToQueryTaskCompletedType(t.Result.ResultType),
		QueryResult:       ToPayload(t.Result.Answer),
		ErrorMessage:      t.Result.ErrorMessage,
		WorkerVersionInfo: ToWorkerVersionInfo(t.WorkerVersionInfo),
	}
}

func FromQueryTaskCompletedType(t *types.QueryTaskCompletedType) apiv1.QueryResultType {
	if t == nil {
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID
	}
	switch *t {
	case types.QueryTaskCompletedTypeCompleted:
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED
	case types.QueryTaskCompletedTypeFailed:
		return apiv1.QueryResultType_QUERY_RESULT_TYPE_FAILED
	}
	panic("unexpected enum value")
}

func ToQueryTaskCompletedType(t apiv1.QueryResultType) *types.QueryTaskCompletedType {
	switch t {
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_INVALID:
		return nil
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_ANSWERED:
		return types.QueryTaskCompletedTypeCompleted.Ptr()
	case apiv1.QueryResultType_QUERY_RESULT_TYPE_FAILED:
		return types.QueryTaskCompletedTypeFailed.Ptr()
	}
	panic("unexpected enum value")
}

func FromRetryPolicy(t *types.RetryPolicy) *apiv1.RetryPolicy {
	if t == nil {
		return nil
	}
	return &apiv1.RetryPolicy{
		InitialInterval:          secondsToDuration(common.Int32Ptr(t.InitialIntervalInSeconds)),
		BackoffCoefficient:       t.BackoffCoefficient,
		MaximumInterval:          secondsToDuration(common.Int32Ptr(t.MaximumIntervalInSeconds)),
		MaximumAttempts:          t.MaximumAttempts,
		NonRetryableErrorReasons: t.NonRetriableErrorReasons,
		ExpirationInterval:       secondsToDuration(common.Int32Ptr(t.ExpirationIntervalInSeconds)),
	}
}

func ToRetryPolicy(t *apiv1.RetryPolicy) *types.RetryPolicy {
	if t == nil {
		return nil
	}
	return &types.RetryPolicy{
		InitialIntervalInSeconds:    common.Int32Default(durationToSeconds(t.InitialInterval)),
		BackoffCoefficient:          t.BackoffCoefficient,
		MaximumIntervalInSeconds:    common.Int32Default(durationToSeconds(t.MaximumInterval)),
		MaximumAttempts:             t.MaximumAttempts,
		NonRetriableErrorReasons:    t.NonRetryableErrorReasons,
		ExpirationIntervalInSeconds: common.Int32Default(durationToSeconds(t.ExpirationInterval)),
	}
}

func FromRestartWorkflowExecutionResponse(t *types.RestartWorkflowExecutionResponse) *apiv1.RestartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.RestartWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToRestartWorkflowExecutionRequest(t *apiv1.RestartWorkflowExecutionRequest) *types.RestartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.RestartWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		Identity:          t.Identity,
	}
}

func FromScanWorkflowExecutionsRequest(t *types.ListWorkflowExecutionsRequest) *apiv1.ScanWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ScanWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func ToScanWorkflowExecutionsRequest(t *apiv1.ScanWorkflowExecutionsRequest) *types.ListWorkflowExecutionsRequest {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
		Query:         t.Query,
	}
}

func FromScanWorkflowExecutionsResponse(t *types.ListWorkflowExecutionsResponse) *apiv1.ScanWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ScanWorkflowExecutionsResponse{
		Executions:    FromWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func ToScanWorkflowExecutionsResponse(t *apiv1.ScanWorkflowExecutionsResponse) *types.ListWorkflowExecutionsResponse {
	if t == nil {
		return nil
	}
	return &types.ListWorkflowExecutionsResponse{
		Executions:    ToWorkflowExecutionInfoArray(t.Executions),
		NextPageToken: t.NextPageToken,
	}
}

func FromScheduleActivityTaskDecisionAttributes(t *types.ScheduleActivityTaskDecisionAttributes) *apiv1.ScheduleActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleActivityTaskDecisionAttributes{
		ActivityId:             t.ActivityID,
		ActivityType:           FromActivityType(t.ActivityType),
		Domain:                 t.Domain,
		TaskList:               FromTaskList(t.TaskList),
		Input:                  FromPayload(t.Input),
		ScheduleToCloseTimeout: secondsToDuration(t.ScheduleToCloseTimeoutSeconds),
		ScheduleToStartTimeout: secondsToDuration(t.ScheduleToStartTimeoutSeconds),
		StartToCloseTimeout:    secondsToDuration(t.StartToCloseTimeoutSeconds),
		HeartbeatTimeout:       secondsToDuration(t.HeartbeatTimeoutSeconds),
		RetryPolicy:            FromRetryPolicy(t.RetryPolicy),
		Header:                 FromHeader(t.Header),
		RequestLocalDispatch:   t.RequestLocalDispatch,
	}
}

func ToScheduleActivityTaskDecisionAttributes(t *apiv1.ScheduleActivityTaskDecisionAttributes) *types.ScheduleActivityTaskDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    t.ActivityId,
		ActivityType:                  ToActivityType(t.ActivityType),
		Domain:                        t.Domain,
		TaskList:                      ToTaskList(t.TaskList),
		Input:                         ToPayload(t.Input),
		ScheduleToCloseTimeoutSeconds: durationToSeconds(t.ScheduleToCloseTimeout),
		ScheduleToStartTimeoutSeconds: durationToSeconds(t.ScheduleToStartTimeout),
		StartToCloseTimeoutSeconds:    durationToSeconds(t.StartToCloseTimeout),
		HeartbeatTimeoutSeconds:       durationToSeconds(t.HeartbeatTimeout),
		RetryPolicy:                   ToRetryPolicy(t.RetryPolicy),
		Header:                        ToHeader(t.Header),
		RequestLocalDispatch:          t.RequestLocalDispatch,
	}
}

func FromSearchAttributes(t *types.SearchAttributes) *apiv1.SearchAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.SearchAttributes{
		IndexedFields: FromPayloadMap(t.IndexedFields),
	}
}

func ToSearchAttributes(t *apiv1.SearchAttributes) *types.SearchAttributes {
	if t == nil {
		return nil
	}
	return &types.SearchAttributes{
		IndexedFields: ToPayloadMap(t.IndexedFields),
	}
}

func FromSignalExternalWorkflowExecutionDecisionAttributes(t *types.SignalExternalWorkflowExecutionDecisionAttributes) *apiv1.SignalExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
		SignalName:        t.SignalName,
		Input:             FromPayload(t.Input),
		Control:           t.Control,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
	}
}

func ToSignalExternalWorkflowExecutionDecisionAttributes(t *apiv1.SignalExternalWorkflowExecutionDecisionAttributes) *types.SignalExternalWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionDecisionAttributes{
		Domain:            t.Domain,
		Execution:         ToWorkflowExecution(t.WorkflowExecution),
		SignalName:        t.SignalName,
		Input:             ToPayload(t.Input),
		Control:           t.Control,
		ChildWorkflowOnly: t.ChildWorkflowOnly,
	}
}

func FromSignalExternalWorkflowExecutionFailedCause(t *types.SignalExternalWorkflowExecutionFailedCause) apiv1.SignalExternalWorkflowExecutionFailedCause {
	if t == nil {
		return apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID
	}
	switch *t {
	case types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution:
		return apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION
	case types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted:
		return apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED
	}
	panic("unexpected enum value")
}

func ToSignalExternalWorkflowExecutionFailedCause(t apiv1.SignalExternalWorkflowExecutionFailedCause) *types.SignalExternalWorkflowExecutionFailedCause {
	switch t {
	case apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_INVALID:
		return nil
	case apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_UNKNOWN_EXTERNAL_WORKFLOW_EXECUTION:
		return types.SignalExternalWorkflowExecutionFailedCauseUnknownExternalWorkflowExecution.Ptr()
	case apiv1.SignalExternalWorkflowExecutionFailedCause_SIGNAL_EXTERNAL_WORKFLOW_EXECUTION_FAILED_CAUSE_WORKFLOW_ALREADY_COMPLETED:
		return types.SignalExternalWorkflowExecutionFailedCauseWorkflowAlreadyCompleted.Ptr()
	}
	panic("unexpected enum value")
}

func FromSignalExternalWorkflowExecutionFailedEventAttributes(t *types.SignalExternalWorkflowExecutionFailedEventAttributes) *apiv1.SignalExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.SignalExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        FromSignalExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Domain:                       t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		InitiatedEventId:             t.InitiatedEventID,
		Control:                      t.Control,
	}
}

func ToSignalExternalWorkflowExecutionFailedEventAttributes(t *apiv1.SignalExternalWorkflowExecutionFailedEventAttributes) *types.SignalExternalWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionFailedEventAttributes{
		Cause:                        ToSignalExternalWorkflowExecutionFailedCause(t.Cause),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Domain:                       t.Domain,
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		InitiatedEventID:             t.InitiatedEventId,
		Control:                      t.Control,
	}
}

func FromSignalExternalWorkflowExecutionInitiatedEventAttributes(t *types.SignalExternalWorkflowExecutionInitiatedEventAttributes) *apiv1.SignalExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Domain:                       t.Domain,
		WorkflowExecution:            FromWorkflowExecution(t.WorkflowExecution),
		SignalName:                   t.SignalName,
		Input:                        FromPayload(t.Input),
		Control:                      t.Control,
		ChildWorkflowOnly:            t.ChildWorkflowOnly,
	}
}

func ToSignalExternalWorkflowExecutionInitiatedEventAttributes(t *apiv1.SignalExternalWorkflowExecutionInitiatedEventAttributes) *types.SignalExternalWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.SignalExternalWorkflowExecutionInitiatedEventAttributes{
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Domain:                       t.Domain,
		WorkflowExecution:            ToWorkflowExecution(t.WorkflowExecution),
		SignalName:                   t.SignalName,
		Input:                        ToPayload(t.Input),
		Control:                      t.Control,
		ChildWorkflowOnly:            t.ChildWorkflowOnly,
	}
}

func FromSignalWithStartWorkflowExecutionRequest(t *types.SignalWithStartWorkflowExecutionRequest) *apiv1.SignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.SignalWithStartWorkflowExecutionRequest{
		StartRequest: &apiv1.StartWorkflowExecutionRequest{
			Domain:                       t.Domain,
			WorkflowId:                   t.WorkflowID,
			WorkflowType:                 FromWorkflowType(t.WorkflowType),
			TaskList:                     FromTaskList(t.TaskList),
			Input:                        FromPayload(t.Input),
			ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
			TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
			Identity:                     t.Identity,
			RequestId:                    t.RequestID,
			WorkflowIdReusePolicy:        FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
			RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
			CronSchedule:                 t.CronSchedule,
			Memo:                         FromMemo(t.Memo),
			SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
			Header:                       FromHeader(t.Header),
			DelayStart:                   secondsToDuration(t.DelayStartSeconds),
			JitterStart:                  secondsToDuration(t.JitterStartSeconds),
			FirstRunAt:                   unixNanoToTime(t.FirstRunAtTimestamp),
		},
		SignalName:  t.SignalName,
		SignalInput: FromPayload(t.SignalInput),
		Control:     t.Control,
	}
}

func ToSignalWithStartWorkflowExecutionRequest(t *apiv1.SignalWithStartWorkflowExecutionRequest) *types.SignalWithStartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionRequest{
		Domain:                              t.StartRequest.Domain,
		WorkflowID:                          t.StartRequest.WorkflowId,
		WorkflowType:                        ToWorkflowType(t.StartRequest.WorkflowType),
		TaskList:                            ToTaskList(t.StartRequest.TaskList),
		Input:                               ToPayload(t.StartRequest.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.StartRequest.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.StartRequest.TaskStartToCloseTimeout),
		Identity:                            t.StartRequest.Identity,
		RequestID:                           t.StartRequest.RequestId,
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.StartRequest.WorkflowIdReusePolicy),
		SignalName:                          t.SignalName,
		SignalInput:                         ToPayload(t.SignalInput),
		Control:                             t.Control,
		RetryPolicy:                         ToRetryPolicy(t.StartRequest.RetryPolicy),
		CronSchedule:                        t.StartRequest.CronSchedule,
		Memo:                                ToMemo(t.StartRequest.Memo),
		SearchAttributes:                    ToSearchAttributes(t.StartRequest.SearchAttributes),
		Header:                              ToHeader(t.StartRequest.Header),
		DelayStartSeconds:                   durationToSeconds(t.StartRequest.DelayStart),
		JitterStartSeconds:                  durationToSeconds(t.StartRequest.JitterStart),
		FirstRunAtTimestamp:                 timeToUnixNano(t.StartRequest.FirstRunAt),
	}
}

func FromSignalWithStartWorkflowExecutionResponse(t *types.StartWorkflowExecutionResponse) *apiv1.SignalWithStartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.SignalWithStartWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToSignalWithStartWorkflowExecutionResponse(t *apiv1.SignalWithStartWorkflowExecutionResponse) *types.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromSignalWorkflowExecutionRequest(t *types.SignalWorkflowExecutionRequest) *apiv1.SignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.SignalWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		SignalName:        t.SignalName,
		SignalInput:       FromPayload(t.Input),
		Identity:          t.Identity,
		RequestId:         t.RequestID,
		Control:           t.Control,
	}
}

func ToSignalWorkflowExecutionRequest(t *apiv1.SignalWorkflowExecutionRequest) *types.SignalWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: ToWorkflowExecution(t.WorkflowExecution),
		SignalName:        t.SignalName,
		Input:             ToPayload(t.SignalInput),
		Identity:          t.Identity,
		RequestID:         t.RequestId,
		Control:           t.Control,
	}
}

func FromStartChildWorkflowExecutionDecisionAttributes(t *types.StartChildWorkflowExecutionDecisionAttributes) *apiv1.StartChildWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                       t.Domain,
		WorkflowId:                   t.WorkflowID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		ParentClosePolicy:            FromParentClosePolicy(t.ParentClosePolicy),
		Control:                      t.Control,
		WorkflowIdReusePolicy:        FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                 t.CronSchedule,
		Header:                       FromHeader(t.Header),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
	}
}

func ToStartChildWorkflowExecutionDecisionAttributes(t *apiv1.StartChildWorkflowExecutionDecisionAttributes) *types.StartChildWorkflowExecutionDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionDecisionAttributes{
		Domain:                              t.Domain,
		WorkflowID:                          t.WorkflowId,
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		ParentClosePolicy:                   ToParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.CronSchedule,
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
	}
}

func FromStartChildWorkflowExecutionFailedEventAttributes(t *types.StartChildWorkflowExecutionFailedEventAttributes) *apiv1.StartChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       t.Domain,
		WorkflowId:                   t.WorkflowID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		Cause:                        FromChildWorkflowExecutionFailedCause(t.Cause),
		Control:                      t.Control,
		InitiatedEventId:             t.InitiatedEventID,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToStartChildWorkflowExecutionFailedEventAttributes(t *apiv1.StartChildWorkflowExecutionFailedEventAttributes) *types.StartChildWorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionFailedEventAttributes{
		Domain:                       t.Domain,
		WorkflowID:                   t.WorkflowId,
		WorkflowType:                 ToWorkflowType(t.WorkflowType),
		Cause:                        ToChildWorkflowExecutionFailedCause(t.Cause),
		Control:                      t.Control,
		InitiatedEventID:             t.InitiatedEventId,
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

func FromStartChildWorkflowExecutionInitiatedEventAttributes(t *types.StartChildWorkflowExecutionInitiatedEventAttributes) *apiv1.StartChildWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                       t.Domain,
		WorkflowId:                   t.WorkflowID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		ParentClosePolicy:            FromParentClosePolicy(t.ParentClosePolicy),
		Control:                      t.Control,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		WorkflowIdReusePolicy:        FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                 t.CronSchedule,
		Header:                       FromHeader(t.Header),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
		DelayStart:                   secondsToDuration(t.DelayStartSeconds),
		JitterStart:                  secondsToDuration(t.JitterStartSeconds),
		FirstRunAt:                   unixNanoToTime(t.FirstRunAtTimestamp),
	}
}

func ToStartChildWorkflowExecutionInitiatedEventAttributes(t *apiv1.StartChildWorkflowExecutionInitiatedEventAttributes) *types.StartChildWorkflowExecutionInitiatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.StartChildWorkflowExecutionInitiatedEventAttributes{
		Domain:                              t.Domain,
		WorkflowID:                          t.WorkflowId,
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		ParentClosePolicy:                   ToParentClosePolicy(t.ParentClosePolicy),
		Control:                             t.Control,
		DecisionTaskCompletedEventID:        t.DecisionTaskCompletedEventId,
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.CronSchedule,
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		DelayStartSeconds:                   durationToSeconds(t.DelayStart),
		JitterStartSeconds:                  durationToSeconds(t.JitterStart),
		FirstRunAtTimestamp:                 timeToUnixNano(t.FirstRunAt),
	}
}

func FromStartTimeFilter(t *types.StartTimeFilter) *apiv1.StartTimeFilter {
	if t == nil {
		return nil
	}
	return &apiv1.StartTimeFilter{
		EarliestTime: unixNanoToTime(t.EarliestTime),
		LatestTime:   unixNanoToTime(t.LatestTime),
	}
}

func ToStartTimeFilter(t *apiv1.StartTimeFilter) *types.StartTimeFilter {
	if t == nil {
		return nil
	}
	return &types.StartTimeFilter{
		EarliestTime: timeToUnixNano(t.EarliestTime),
		LatestTime:   timeToUnixNano(t.LatestTime),
	}
}

func FromStartTimerDecisionAttributes(t *types.StartTimerDecisionAttributes) *apiv1.StartTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.StartTimerDecisionAttributes{
		TimerId:            t.TimerID,
		StartToFireTimeout: secondsToDuration(int64To32(t.StartToFireTimeoutSeconds)),
	}
}

func ToStartTimerDecisionAttributes(t *apiv1.StartTimerDecisionAttributes) *types.StartTimerDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.StartTimerDecisionAttributes{
		TimerID:                   t.TimerId,
		StartToFireTimeoutSeconds: int32To64(durationToSeconds(t.StartToFireTimeout)),
	}
}

func FromRestartWorkflowExecutionRequest(t *types.RestartWorkflowExecutionRequest) *apiv1.RestartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.RestartWorkflowExecutionRequest{
		Domain:            t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.WorkflowExecution),
		Identity:          t.Identity,
	}
}

func ToRestartWorkflowExecutionResponse(t *apiv1.RestartWorkflowExecutionResponse) *types.RestartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.RestartWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromSignalWithStartWorkflowExecutionAsyncRequest(t *types.SignalWithStartWorkflowExecutionAsyncRequest) *apiv1.SignalWithStartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &apiv1.SignalWithStartWorkflowExecutionAsyncRequest{
		Request: FromSignalWithStartWorkflowExecutionRequest(t.SignalWithStartWorkflowExecutionRequest),
	}
}

func ToSignalWithStartWorkflowExecutionAsyncRequest(t *apiv1.SignalWithStartWorkflowExecutionAsyncRequest) *types.SignalWithStartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionAsyncRequest{
		SignalWithStartWorkflowExecutionRequest: ToSignalWithStartWorkflowExecutionRequest(t.Request),
	}
}

func FromSignalWithStartWorkflowExecutionAsyncResponse(t *types.SignalWithStartWorkflowExecutionAsyncResponse) *apiv1.SignalWithStartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &apiv1.SignalWithStartWorkflowExecutionAsyncResponse{}
}

func ToSignalWithStartWorkflowExecutionAsyncResponse(t *apiv1.SignalWithStartWorkflowExecutionAsyncResponse) *types.SignalWithStartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &types.SignalWithStartWorkflowExecutionAsyncResponse{}
}

func FromStartWorkflowExecutionAsyncRequest(t *types.StartWorkflowExecutionAsyncRequest) *apiv1.StartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &apiv1.StartWorkflowExecutionAsyncRequest{
		Request: FromStartWorkflowExecutionRequest(t.StartWorkflowExecutionRequest),
	}
}

func ToStartWorkflowExecutionAsyncRequest(t *apiv1.StartWorkflowExecutionAsyncRequest) *types.StartWorkflowExecutionAsyncRequest {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionAsyncRequest{
		StartWorkflowExecutionRequest: ToStartWorkflowExecutionRequest(t.Request),
	}
}

func FromStartWorkflowExecutionAsyncResponse(t *types.StartWorkflowExecutionAsyncResponse) *apiv1.StartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &apiv1.StartWorkflowExecutionAsyncResponse{}
}

func ToStartWorkflowExecutionAsyncResponse(t *apiv1.StartWorkflowExecutionAsyncResponse) *types.StartWorkflowExecutionAsyncResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionAsyncResponse{}
}

func FromStartWorkflowExecutionRequest(t *types.StartWorkflowExecutionRequest) *apiv1.StartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.StartWorkflowExecutionRequest{
		Domain:                       t.Domain,
		WorkflowId:                   t.WorkflowID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		Identity:                     t.Identity,
		RequestId:                    t.RequestID,
		WorkflowIdReusePolicy:        FromWorkflowIDReusePolicy(t.WorkflowIDReusePolicy),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		CronSchedule:                 t.CronSchedule,
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
		Header:                       FromHeader(t.Header),
		DelayStart:                   secondsToDuration(t.DelayStartSeconds),
		JitterStart:                  secondsToDuration(t.JitterStartSeconds),
		FirstRunAt:                   unixNanoToTime(t.FirstRunAtTimeStamp),
	}
}

func ToStartWorkflowExecutionRequest(t *apiv1.StartWorkflowExecutionRequest) *types.StartWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionRequest{
		Domain:                              t.Domain,
		WorkflowID:                          t.WorkflowId,
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		Identity:                            t.Identity,
		RequestID:                           t.RequestId,
		WorkflowIDReusePolicy:               ToWorkflowIDReusePolicy(t.WorkflowIdReusePolicy),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		CronSchedule:                        t.CronSchedule,
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		Header:                              ToHeader(t.Header),
		DelayStartSeconds:                   durationToSeconds(t.DelayStart),
		JitterStartSeconds:                  durationToSeconds(t.JitterStart),
		FirstRunAtTimeStamp:                 timeToUnixNano(t.FirstRunAt),
	}
}

func FromStartWorkflowExecutionResponse(t *types.StartWorkflowExecutionResponse) *apiv1.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &apiv1.StartWorkflowExecutionResponse{
		RunId: t.RunID,
	}
}

func ToStartWorkflowExecutionResponse(t *apiv1.StartWorkflowExecutionResponse) *types.StartWorkflowExecutionResponse {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowExecutionResponse{
		RunID: t.RunId,
	}
}

func FromStatusFilter(t *types.WorkflowExecutionCloseStatus) *apiv1.StatusFilter {
	if t == nil {
		return nil
	}
	return &apiv1.StatusFilter{
		Status: FromWorkflowExecutionCloseStatus(t),
	}
}

func ToStatusFilter(t *apiv1.StatusFilter) *types.WorkflowExecutionCloseStatus {
	if t == nil {
		return nil
	}
	return ToWorkflowExecutionCloseStatus(t.Status)
}

func FromStickyExecutionAttributes(t *types.StickyExecutionAttributes) *apiv1.StickyExecutionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.StickyExecutionAttributes{
		WorkerTaskList:         FromTaskList(t.WorkerTaskList),
		ScheduleToStartTimeout: secondsToDuration(t.ScheduleToStartTimeoutSeconds),
	}
}

func ToStickyExecutionAttributes(t *apiv1.StickyExecutionAttributes) *types.StickyExecutionAttributes {
	if t == nil {
		return nil
	}
	return &types.StickyExecutionAttributes{
		WorkerTaskList:                ToTaskList(t.WorkerTaskList),
		ScheduleToStartTimeoutSeconds: durationToSeconds(t.ScheduleToStartTimeout),
	}
}

func FromSupportedClientVersions(t *types.SupportedClientVersions) *apiv1.SupportedClientVersions {
	if t == nil {
		return nil
	}
	return &apiv1.SupportedClientVersions{
		GoSdk:   t.GoSdk,
		JavaSdk: t.JavaSdk,
	}
}

func ToSupportedClientVersions(t *apiv1.SupportedClientVersions) *types.SupportedClientVersions {
	if t == nil {
		return nil
	}
	return &types.SupportedClientVersions{
		GoSdk:   t.GoSdk,
		JavaSdk: t.JavaSdk,
	}
}

func FromTaskIDBlock(t *types.TaskIDBlock) *apiv1.TaskIDBlock {
	if t == nil {
		return nil
	}
	return &apiv1.TaskIDBlock{
		StartId: t.StartID,
		EndId:   t.EndID,
	}
}

func ToTaskIDBlock(t *apiv1.TaskIDBlock) *types.TaskIDBlock {
	if t == nil {
		return nil
	}
	return &types.TaskIDBlock{
		StartID: t.StartId,
		EndID:   t.EndId,
	}
}

func FromTaskList(t *types.TaskList) *apiv1.TaskList {
	if t == nil {
		return nil
	}
	return &apiv1.TaskList{
		Name: t.Name,
		Kind: FromTaskListKind(t.Kind),
	}
}

func ToTaskList(t *apiv1.TaskList) *types.TaskList {
	if t == nil {
		return nil
	}
	return &types.TaskList{
		Name: t.Name,
		Kind: ToTaskListKind(t.Kind),
	}
}

func FromTaskListKind(t *types.TaskListKind) apiv1.TaskListKind {
	if t == nil {
		return apiv1.TaskListKind_TASK_LIST_KIND_INVALID
	}
	switch *t {
	case types.TaskListKindNormal:
		return apiv1.TaskListKind_TASK_LIST_KIND_NORMAL
	case types.TaskListKindSticky:
		return apiv1.TaskListKind_TASK_LIST_KIND_STICKY
	}
	panic("unexpected enum value")
}

func ToTaskListKind(t apiv1.TaskListKind) *types.TaskListKind {
	switch t {
	case apiv1.TaskListKind_TASK_LIST_KIND_INVALID:
		return nil
	case apiv1.TaskListKind_TASK_LIST_KIND_NORMAL:
		return types.TaskListKindNormal.Ptr()
	case apiv1.TaskListKind_TASK_LIST_KIND_STICKY:
		return types.TaskListKindSticky.Ptr()
	}
	panic("unexpected enum value")
}

func FromTaskListMetadata(t *types.TaskListMetadata) *apiv1.TaskListMetadata {
	if t == nil {
		return nil
	}
	return &apiv1.TaskListMetadata{
		MaxTasksPerSecond: fromDoubleValue(t.MaxTasksPerSecond),
	}
}

func ToTaskListMetadata(t *apiv1.TaskListMetadata) *types.TaskListMetadata {
	if t == nil {
		return nil
	}
	return &types.TaskListMetadata{
		MaxTasksPerSecond: toDoubleValue(t.MaxTasksPerSecond),
	}
}

func FromTaskListPartitionMetadata(t *types.TaskListPartitionMetadata) *apiv1.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	return &apiv1.TaskListPartitionMetadata{
		Key:           t.Key,
		OwnerHostName: t.OwnerHostName,
	}
}

func ToTaskListPartitionMetadata(t *apiv1.TaskListPartitionMetadata) *types.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	return &types.TaskListPartitionMetadata{
		Key:           t.Key,
		OwnerHostName: t.OwnerHostName,
	}
}

func FromTaskListStatus(t *types.TaskListStatus) *apiv1.TaskListStatus {
	if t == nil {
		return nil
	}
	return &apiv1.TaskListStatus{
		BacklogCountHint:      t.BacklogCountHint,
		ReadLevel:             t.ReadLevel,
		AckLevel:              t.AckLevel,
		RatePerSecond:         t.RatePerSecond,
		TaskIdBlock:           FromTaskIDBlock(t.TaskIDBlock),
		IsolationGroupMetrics: FromIsolationGroupMetricsMap(t.IsolationGroupMetrics),
		NewTasksPerSecond:     t.NewTasksPerSecond,
	}
}

func ToTaskListStatus(t *apiv1.TaskListStatus) *types.TaskListStatus {
	if t == nil {
		return nil
	}
	return &types.TaskListStatus{
		BacklogCountHint:      t.BacklogCountHint,
		ReadLevel:             t.ReadLevel,
		AckLevel:              t.AckLevel,
		RatePerSecond:         t.RatePerSecond,
		TaskIDBlock:           ToTaskIDBlock(t.TaskIdBlock),
		IsolationGroupMetrics: ToIsolationGroupMetricsMap(t.IsolationGroupMetrics),
		NewTasksPerSecond:     t.NewTasksPerSecond,
	}
}

func FromIsolationGroupMetrics(t *types.IsolationGroupMetrics) *apiv1.IsolationGroupMetrics {
	return &apiv1.IsolationGroupMetrics{
		NewTasksPerSecond: t.NewTasksPerSecond,
		PollerCount:       t.PollerCount,
	}
}

func ToIsolationGroupMetrics(t *apiv1.IsolationGroupMetrics) *types.IsolationGroupMetrics {
	return &types.IsolationGroupMetrics{
		NewTasksPerSecond: t.NewTasksPerSecond,
		PollerCount:       t.PollerCount,
	}
}

func FromTaskListType(t *types.TaskListType) apiv1.TaskListType {
	if t == nil {
		return apiv1.TaskListType_TASK_LIST_TYPE_INVALID
	}
	switch *t {
	case types.TaskListTypeDecision:
		return apiv1.TaskListType_TASK_LIST_TYPE_DECISION
	case types.TaskListTypeActivity:
		return apiv1.TaskListType_TASK_LIST_TYPE_ACTIVITY
	}
	panic("unexpected enum value")
}

func ToTaskListType(t apiv1.TaskListType) *types.TaskListType {
	switch t {
	case apiv1.TaskListType_TASK_LIST_TYPE_INVALID:
		return nil
	case apiv1.TaskListType_TASK_LIST_TYPE_DECISION:
		return types.TaskListTypeDecision.Ptr()
	case apiv1.TaskListType_TASK_LIST_TYPE_ACTIVITY:
		return types.TaskListTypeActivity.Ptr()
	}
	panic("unexpected enum value")
}

func FromTerminateWorkflowExecutionRequest(t *types.TerminateWorkflowExecutionRequest) *apiv1.TerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &apiv1.TerminateWorkflowExecutionRequest{
		Domain:              t.Domain,
		WorkflowExecution:   FromWorkflowExecution(t.WorkflowExecution),
		Reason:              t.Reason,
		Details:             FromPayload(t.Details),
		Identity:            t.Identity,
		FirstExecutionRunId: t.FirstExecutionRunID,
	}
}

func ToTerminateWorkflowExecutionRequest(t *apiv1.TerminateWorkflowExecutionRequest) *types.TerminateWorkflowExecutionRequest {
	if t == nil {
		return nil
	}
	return &types.TerminateWorkflowExecutionRequest{
		Domain:              t.Domain,
		WorkflowExecution:   ToWorkflowExecution(t.WorkflowExecution),
		Reason:              t.Reason,
		Details:             ToPayload(t.Details),
		Identity:            t.Identity,
		FirstExecutionRunID: t.FirstExecutionRunId,
	}
}

func FromTimeoutType(t *types.TimeoutType) apiv1.TimeoutType {
	if t == nil {
		return apiv1.TimeoutType_TIMEOUT_TYPE_INVALID
	}
	switch *t {
	case types.TimeoutTypeStartToClose:
		return apiv1.TimeoutType_TIMEOUT_TYPE_START_TO_CLOSE
	case types.TimeoutTypeScheduleToStart:
		return apiv1.TimeoutType_TIMEOUT_TYPE_SCHEDULE_TO_START
	case types.TimeoutTypeScheduleToClose:
		return apiv1.TimeoutType_TIMEOUT_TYPE_SCHEDULE_TO_CLOSE
	case types.TimeoutTypeHeartbeat:
		return apiv1.TimeoutType_TIMEOUT_TYPE_HEARTBEAT
	}
	panic("unexpected enum value")
}

func ToTimeoutType(t apiv1.TimeoutType) *types.TimeoutType {
	switch t {
	case apiv1.TimeoutType_TIMEOUT_TYPE_INVALID:
		return nil
	case apiv1.TimeoutType_TIMEOUT_TYPE_START_TO_CLOSE:
		return types.TimeoutTypeStartToClose.Ptr()
	case apiv1.TimeoutType_TIMEOUT_TYPE_SCHEDULE_TO_START:
		return types.TimeoutTypeScheduleToStart.Ptr()
	case apiv1.TimeoutType_TIMEOUT_TYPE_SCHEDULE_TO_CLOSE:
		return types.TimeoutTypeScheduleToClose.Ptr()
	case apiv1.TimeoutType_TIMEOUT_TYPE_HEARTBEAT:
		return types.TimeoutTypeHeartbeat.Ptr()
	}
	panic("unexpected enum value")
}

func FromDecisionTaskTimedOutCause(t *types.DecisionTaskTimedOutCause) apiv1.DecisionTaskTimedOutCause {
	if t == nil {
		return apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID
	}
	switch *t {
	case types.DecisionTaskTimedOutCauseTimeout:
		return apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_TIMEOUT
	case types.DecisionTaskTimedOutCauseReset:
		return apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_RESET
	}
	panic("unexpected enum value")
}

func ToDecisionTaskTimedOutCause(t apiv1.DecisionTaskTimedOutCause) *types.DecisionTaskTimedOutCause {
	switch t {
	case apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_INVALID:
		return nil
	case apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_TIMEOUT:
		return types.DecisionTaskTimedOutCauseTimeout.Ptr()
	case apiv1.DecisionTaskTimedOutCause_DECISION_TASK_TIMED_OUT_CAUSE_RESET:
		return types.DecisionTaskTimedOutCauseReset.Ptr()
	}
	panic("unexpected enum value")
}

func FromTimerCanceledEventAttributes(t *types.TimerCanceledEventAttributes) *apiv1.TimerCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.TimerCanceledEventAttributes{
		TimerId:                      t.TimerID,
		StartedEventId:               t.StartedEventID,
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Identity:                     t.Identity,
	}
}

func ToTimerCanceledEventAttributes(t *apiv1.TimerCanceledEventAttributes) *types.TimerCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerCanceledEventAttributes{
		TimerID:                      t.TimerId,
		StartedEventID:               t.StartedEventId,
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Identity:                     t.Identity,
	}
}

func FromTimerFiredEventAttributes(t *types.TimerFiredEventAttributes) *apiv1.TimerFiredEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.TimerFiredEventAttributes{
		TimerId:        t.TimerID,
		StartedEventId: t.StartedEventID,
	}
}

func ToTimerFiredEventAttributes(t *apiv1.TimerFiredEventAttributes) *types.TimerFiredEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerFiredEventAttributes{
		TimerID:        t.TimerId,
		StartedEventID: t.StartedEventId,
	}
}

func FromTimerStartedEventAttributes(t *types.TimerStartedEventAttributes) *apiv1.TimerStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.TimerStartedEventAttributes{
		TimerId:                      t.TimerID,
		StartToFireTimeout:           secondsToDuration(int64To32(t.StartToFireTimeoutSeconds)),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToTimerStartedEventAttributes(t *apiv1.TimerStartedEventAttributes) *types.TimerStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.TimerStartedEventAttributes{
		TimerID:                      t.TimerId,
		StartToFireTimeoutSeconds:    int32To64(durationToSeconds(t.StartToFireTimeout)),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

const (
	DomainUpdateDescriptionField              = "description"
	DomainUpdateOwnerEmailField               = "owner_email"
	DomainUpdateDataField                     = "data"
	DomainUpdateRetentionPeriodField          = "workflow_execution_retention_period"
	DomainUpdateBadBinariesField              = "bad_binaries"
	DomainUpdateHistoryArchivalStatusField    = "history_archival_status"
	DomainUpdateHistoryArchivalURIField       = "history_archival_uri"
	DomainUpdateVisibilityArchivalStatusField = "visibility_archival_status"
	DomainUpdateVisibilityArchivalURIField    = "visibility_archival_uri"
	DomainUpdateActiveClusterNameField        = "active_cluster_name"
	DomainUpdateClustersField                 = "clusters"
	DomainUpdateDeleteBadBinaryField          = "delete_bad_binary"
	DomainUpdateFailoverTimeoutField          = "failover_timeout"
)

func FromUpdateDomainRequest(t *types.UpdateDomainRequest) *apiv1.UpdateDomainRequest {
	if t == nil {
		return nil
	}
	request := apiv1.UpdateDomainRequest{
		Name:          t.Name,
		SecurityToken: t.SecurityToken,
	}
	fields := []string{}

	if t.Description != nil {
		request.Description = *t.Description
		fields = append(fields, DomainUpdateDescriptionField)
	}
	if t.OwnerEmail != nil {
		request.OwnerEmail = *t.OwnerEmail
		fields = append(fields, DomainUpdateOwnerEmailField)
	}
	if t.Data != nil {
		request.Data = t.Data
		fields = append(fields, DomainUpdateDataField)
	}
	if t.WorkflowExecutionRetentionPeriodInDays != nil {
		request.WorkflowExecutionRetentionPeriod = daysToDuration(t.WorkflowExecutionRetentionPeriodInDays)
		fields = append(fields, DomainUpdateRetentionPeriodField)
	}
	// if t.EmitMetric != nil {} - DEPRECATED
	if t.BadBinaries != nil {
		request.BadBinaries = FromBadBinaries(t.BadBinaries)
		fields = append(fields, DomainUpdateBadBinariesField)
	}
	if t.HistoryArchivalStatus != nil {
		request.HistoryArchivalStatus = FromArchivalStatus(t.HistoryArchivalStatus)
		fields = append(fields, DomainUpdateHistoryArchivalStatusField)
	}
	if t.HistoryArchivalURI != nil {
		request.HistoryArchivalUri = *t.HistoryArchivalURI
		fields = append(fields, DomainUpdateHistoryArchivalURIField)
	}
	if t.VisibilityArchivalStatus != nil {
		request.VisibilityArchivalStatus = FromArchivalStatus(t.VisibilityArchivalStatus)
		fields = append(fields, DomainUpdateVisibilityArchivalStatusField)
	}
	if t.VisibilityArchivalURI != nil {
		request.VisibilityArchivalUri = *t.VisibilityArchivalURI
		fields = append(fields, DomainUpdateVisibilityArchivalURIField)
	}
	if t.ActiveClusterName != nil {
		request.ActiveClusterName = *t.ActiveClusterName
		fields = append(fields, DomainUpdateActiveClusterNameField)
	}
	if t.Clusters != nil {
		request.Clusters = FromClusterReplicationConfigurationArray(t.Clusters)
		fields = append(fields, DomainUpdateClustersField)
	}
	if t.DeleteBadBinary != nil {
		request.DeleteBadBinary = *t.DeleteBadBinary
		fields = append(fields, DomainUpdateDeleteBadBinaryField)
	}
	if t.FailoverTimeoutInSeconds != nil {
		request.FailoverTimeout = secondsToDuration(t.FailoverTimeoutInSeconds)
		fields = append(fields, DomainUpdateFailoverTimeoutField)
	}

	request.UpdateMask = newFieldMask(fields)

	return &request
}

func ToUpdateDomainRequest(t *apiv1.UpdateDomainRequest) *types.UpdateDomainRequest {
	if t == nil {
		return nil
	}
	request := types.UpdateDomainRequest{
		Name:          t.Name,
		SecurityToken: t.SecurityToken,
	}
	fs := newFieldSet(t.UpdateMask)

	if fs.isSet(DomainUpdateDescriptionField) {
		request.Description = common.StringPtr(t.Description)
	}
	if fs.isSet(DomainUpdateOwnerEmailField) {
		request.OwnerEmail = common.StringPtr(t.OwnerEmail)
	}
	if fs.isSet(DomainUpdateDataField) {
		request.Data = t.Data
	}
	if fs.isSet(DomainUpdateRetentionPeriodField) {
		request.WorkflowExecutionRetentionPeriodInDays = durationToDays(t.WorkflowExecutionRetentionPeriod)
	}
	if fs.isSet(DomainUpdateBadBinariesField) {
		request.BadBinaries = ToBadBinaries(t.BadBinaries)
	}
	if fs.isSet(DomainUpdateHistoryArchivalStatusField) {
		request.HistoryArchivalStatus = ToArchivalStatus(t.HistoryArchivalStatus)
	}
	if fs.isSet(DomainUpdateHistoryArchivalURIField) {
		request.HistoryArchivalURI = common.StringPtr(t.HistoryArchivalUri)
	}
	if fs.isSet(DomainUpdateVisibilityArchivalStatusField) {
		request.VisibilityArchivalStatus = ToArchivalStatus(t.VisibilityArchivalStatus)
	}
	if fs.isSet(DomainUpdateVisibilityArchivalURIField) {
		request.VisibilityArchivalURI = common.StringPtr(t.VisibilityArchivalUri)
	}
	if fs.isSet(DomainUpdateActiveClusterNameField) {
		request.ActiveClusterName = common.StringPtr(t.ActiveClusterName)
	}
	if fs.isSet(DomainUpdateClustersField) {
		request.Clusters = ToClusterReplicationConfigurationArray(t.Clusters)
	}
	if fs.isSet(DomainUpdateDeleteBadBinaryField) {
		request.DeleteBadBinary = common.StringPtr(t.DeleteBadBinary)
	}
	if fs.isSet(DomainUpdateFailoverTimeoutField) {
		request.FailoverTimeoutInSeconds = durationToSeconds(t.FailoverTimeout)
	}

	return &request
}

func FromUpdateDomainResponse(t *types.UpdateDomainResponse) *apiv1.UpdateDomainResponse {
	if t == nil {
		return nil
	}
	domain := &apiv1.Domain{
		FailoverVersion: t.FailoverVersion,
		IsGlobalDomain:  t.IsGlobalDomain,
	}
	if info := t.DomainInfo; info != nil {
		domain.Id = info.UUID
		domain.Name = info.Name
		domain.Status = FromDomainStatus(info.Status)
		domain.Description = info.Description
		domain.OwnerEmail = info.OwnerEmail
		domain.Data = info.Data
	}
	if config := t.Configuration; config != nil {
		domain.IsolationGroups = FromIsolationGroupConfig(config.IsolationGroups)
		domain.WorkflowExecutionRetentionPeriod = daysToDuration(&config.WorkflowExecutionRetentionPeriodInDays)
		domain.BadBinaries = FromBadBinaries(config.BadBinaries)
		domain.HistoryArchivalStatus = FromArchivalStatus(config.HistoryArchivalStatus)
		domain.HistoryArchivalUri = config.HistoryArchivalURI
		domain.VisibilityArchivalStatus = FromArchivalStatus(config.VisibilityArchivalStatus)
		domain.VisibilityArchivalUri = config.VisibilityArchivalURI
		domain.AsyncWorkflowConfig = FromDomainAsyncWorkflowConfiguraton(config.AsyncWorkflowConfig)
	}
	if repl := t.ReplicationConfiguration; repl != nil {
		domain.ActiveClusterName = repl.ActiveClusterName
		domain.Clusters = FromClusterReplicationConfigurationArray(repl.Clusters)
	}
	return &apiv1.UpdateDomainResponse{
		Domain: domain,
	}
}

func ToUpdateDomainResponse(t *apiv1.UpdateDomainResponse) *types.UpdateDomainResponse {
	if t == nil || t.Domain == nil {
		return nil
	}
	return &types.UpdateDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name:        t.Domain.Name,
			Status:      ToDomainStatus(t.Domain.Status),
			Description: t.Domain.Description,
			OwnerEmail:  t.Domain.OwnerEmail,
			Data:        t.Domain.Data,
			UUID:        t.Domain.Id,
		},
		Configuration: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: common.Int32Default(durationToDays(t.Domain.WorkflowExecutionRetentionPeriod)),
			EmitMetric:                             true,
			BadBinaries:                            ToBadBinaries(t.Domain.BadBinaries),
			HistoryArchivalStatus:                  ToArchivalStatus(t.Domain.HistoryArchivalStatus),
			HistoryArchivalURI:                     t.Domain.HistoryArchivalUri,
			VisibilityArchivalStatus:               ToArchivalStatus(t.Domain.VisibilityArchivalStatus),
			VisibilityArchivalURI:                  t.Domain.VisibilityArchivalUri,
			IsolationGroups:                        ToIsolationGroupConfig(t.Domain.IsolationGroups),
			AsyncWorkflowConfig:                    ToDomainAsyncWorkflowConfiguraton(t.Domain.AsyncWorkflowConfig),
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: t.Domain.ActiveClusterName,
			Clusters:          ToClusterReplicationConfigurationArray(t.Domain.Clusters),
		},
		FailoverVersion: t.Domain.FailoverVersion,
		IsGlobalDomain:  t.Domain.IsGlobalDomain,
	}
}

func FromUpsertWorkflowSearchAttributesDecisionAttributes(t *types.UpsertWorkflowSearchAttributesDecisionAttributes) *apiv1.UpsertWorkflowSearchAttributesDecisionAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.UpsertWorkflowSearchAttributesDecisionAttributes{
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToUpsertWorkflowSearchAttributesDecisionAttributes(t *apiv1.UpsertWorkflowSearchAttributesDecisionAttributes) *types.UpsertWorkflowSearchAttributesDecisionAttributes {
	if t == nil {
		return nil
	}
	return &types.UpsertWorkflowSearchAttributesDecisionAttributes{
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromUpsertWorkflowSearchAttributesEventAttributes(t *types.UpsertWorkflowSearchAttributesEventAttributes) *apiv1.UpsertWorkflowSearchAttributesEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
	}
}

func ToUpsertWorkflowSearchAttributesEventAttributes(t *apiv1.UpsertWorkflowSearchAttributesEventAttributes) *types.UpsertWorkflowSearchAttributesEventAttributes {
	if t == nil {
		return nil
	}
	return &types.UpsertWorkflowSearchAttributesEventAttributes{
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		SearchAttributes:             ToSearchAttributes(t.SearchAttributes),
	}
}

func FromWorkerVersionInfo(t *types.WorkerVersionInfo) *apiv1.WorkerVersionInfo {
	if t == nil {
		return nil
	}
	return &apiv1.WorkerVersionInfo{
		Impl:           t.Impl,
		FeatureVersion: t.FeatureVersion,
	}
}

func ToWorkerVersionInfo(t *apiv1.WorkerVersionInfo) *types.WorkerVersionInfo {
	if t == nil {
		return nil
	}
	return &types.WorkerVersionInfo{
		Impl:           t.Impl,
		FeatureVersion: t.FeatureVersion,
	}
}

func FromWorkflowRunPair(workflowID, runID string) *apiv1.WorkflowExecution {
	return &apiv1.WorkflowExecution{
		WorkflowId: workflowID,
		RunId:      runID,
	}
}

func ToWorkflowID(t *apiv1.WorkflowExecution) string {
	if t == nil {
		return ""
	}
	return t.WorkflowId
}

func ToRunID(t *apiv1.WorkflowExecution) string {
	if t == nil {
		return ""
	}
	return t.RunId
}

func FromWorkflowExecution(t *types.WorkflowExecution) *apiv1.WorkflowExecution {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecution{
		WorkflowId: t.WorkflowID,
		RunId:      t.RunID,
	}
}

func ToWorkflowExecution(t *apiv1.WorkflowExecution) *types.WorkflowExecution {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecution{
		WorkflowID: t.WorkflowId,
		RunID:      t.RunId,
	}
}

func FromExternalExecutionInfoFields(we *types.WorkflowExecution, initiatedID *int64) *apiv1.ExternalExecutionInfo {
	if we == nil && initiatedID == nil {
		return nil
	}
	return &apiv1.ExternalExecutionInfo{
		WorkflowExecution: FromWorkflowExecution(we),
		InitiatedId:       common.Int64Default(initiatedID), // TODO: we need to figure out whetherrr this field is needed or not
	}
}

func ToExternalWorkflowExecution(t *apiv1.ExternalExecutionInfo) *types.WorkflowExecution {
	if t == nil {
		return nil
	}
	return ToWorkflowExecution(t.WorkflowExecution)
}

func ToExternalInitiatedID(t *apiv1.ExternalExecutionInfo) *int64 {
	if t == nil {
		return nil
	}
	return &t.InitiatedId
}

func FromWorkflowExecutionCancelRequestedEventAttributes(t *types.WorkflowExecutionCancelRequestedEventAttributes) *apiv1.WorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                 t.Cause,
		ExternalExecutionInfo: FromExternalExecutionInfoFields(t.ExternalWorkflowExecution, t.ExternalInitiatedEventID),
		Identity:              t.Identity,
		RequestId:             t.RequestID,
	}
}

func ToWorkflowExecutionCancelRequestedEventAttributes(t *apiv1.WorkflowExecutionCancelRequestedEventAttributes) *types.WorkflowExecutionCancelRequestedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCancelRequestedEventAttributes{
		Cause:                     t.Cause,
		ExternalInitiatedEventID:  ToExternalInitiatedID(t.ExternalExecutionInfo),
		ExternalWorkflowExecution: ToExternalWorkflowExecution(t.ExternalExecutionInfo),
		Identity:                  t.Identity,
		RequestID:                 t.RequestId,
	}
}

func FromWorkflowExecutionCanceledEventAttributes(t *types.WorkflowExecutionCanceledEventAttributes) *apiv1.WorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		Details:                      FromPayload(t.Details),
	}
}

func ToWorkflowExecutionCanceledEventAttributes(t *apiv1.WorkflowExecutionCanceledEventAttributes) *types.WorkflowExecutionCanceledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCanceledEventAttributes{
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
		Details:                      ToPayload(t.Details),
	}
}

func FromWorkflowExecutionCloseStatus(t *types.WorkflowExecutionCloseStatus) apiv1.WorkflowExecutionCloseStatus {
	if t == nil {
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID
	}
	switch *t {
	case types.WorkflowExecutionCloseStatusCompleted:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_COMPLETED
	case types.WorkflowExecutionCloseStatusFailed:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_FAILED
	case types.WorkflowExecutionCloseStatusCanceled:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_CANCELED
	case types.WorkflowExecutionCloseStatusTerminated:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_TERMINATED
	case types.WorkflowExecutionCloseStatusContinuedAsNew:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_CONTINUED_AS_NEW
	case types.WorkflowExecutionCloseStatusTimedOut:
		return apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_TIMED_OUT
	}
	panic("unexpected enum value")
}

func ToWorkflowExecutionCloseStatus(t apiv1.WorkflowExecutionCloseStatus) *types.WorkflowExecutionCloseStatus {
	switch t {
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_INVALID:
		return nil
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_COMPLETED:
		return types.WorkflowExecutionCloseStatusCompleted.Ptr()
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_FAILED:
		return types.WorkflowExecutionCloseStatusFailed.Ptr()
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_CANCELED:
		return types.WorkflowExecutionCloseStatusCanceled.Ptr()
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_TERMINATED:
		return types.WorkflowExecutionCloseStatusTerminated.Ptr()
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_CONTINUED_AS_NEW:
		return types.WorkflowExecutionCloseStatusContinuedAsNew.Ptr()
	case apiv1.WorkflowExecutionCloseStatus_WORKFLOW_EXECUTION_CLOSE_STATUS_TIMED_OUT:
		return types.WorkflowExecutionCloseStatusTimedOut.Ptr()
	}
	panic("unexpected enum value")
}

func FromWorkflowExecutionCompletedEventAttributes(t *types.WorkflowExecutionCompletedEventAttributes) *apiv1.WorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionCompletedEventAttributes{
		Result:                       FromPayload(t.Result),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToWorkflowExecutionCompletedEventAttributes(t *apiv1.WorkflowExecutionCompletedEventAttributes) *types.WorkflowExecutionCompletedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionCompletedEventAttributes{
		Result:                       ToPayload(t.Result),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

func FromWorkflowExecutionConfiguration(t *types.WorkflowExecutionConfiguration) *apiv1.WorkflowExecutionConfiguration {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionConfiguration{
		TaskList:                     FromTaskList(t.TaskList),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
	}
}

func ToWorkflowExecutionConfiguration(t *apiv1.WorkflowExecutionConfiguration) *types.WorkflowExecutionConfiguration {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionConfiguration{
		TaskList:                            ToTaskList(t.TaskList),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
	}
}

func FromWorkflowExecutionContinuedAsNewEventAttributes(t *types.WorkflowExecutionContinuedAsNewEventAttributes) *apiv1.WorkflowExecutionContinuedAsNewEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunId:            t.NewExecutionRunID,
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
		BackoffStartInterval:         secondsToDuration(t.BackoffStartIntervalInSeconds),
		Initiator:                    FromContinueAsNewInitiator(t.Initiator),
		Failure:                      FromFailure(t.FailureReason, t.FailureDetails),
		LastCompletionResult:         FromPayload(t.LastCompletionResult),
		Header:                       FromHeader(t.Header),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
	}
}

func ToWorkflowExecutionContinuedAsNewEventAttributes(t *apiv1.WorkflowExecutionContinuedAsNewEventAttributes) *types.WorkflowExecutionContinuedAsNewEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionContinuedAsNewEventAttributes{
		NewExecutionRunID:                   t.NewExecutionRunId,
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		DecisionTaskCompletedEventID:        t.DecisionTaskCompletedEventId,
		BackoffStartIntervalInSeconds:       durationToSeconds(t.BackoffStartInterval),
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		FailureReason:                       ToFailureReason(t.Failure),
		FailureDetails:                      ToFailureDetails(t.Failure),
		LastCompletionResult:                ToPayload(t.LastCompletionResult),
		Header:                              ToHeader(t.Header),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
	}
}

func FromWorkflowExecutionFailedEventAttributes(t *types.WorkflowExecutionFailedEventAttributes) *apiv1.WorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionFailedEventAttributes{
		Failure:                      FromFailure(t.Reason, t.Details),
		DecisionTaskCompletedEventId: t.DecisionTaskCompletedEventID,
	}
}

func ToWorkflowExecutionFailedEventAttributes(t *apiv1.WorkflowExecutionFailedEventAttributes) *types.WorkflowExecutionFailedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionFailedEventAttributes{
		Reason:                       ToFailureReason(t.Failure),
		Details:                      ToFailureDetails(t.Failure),
		DecisionTaskCompletedEventID: t.DecisionTaskCompletedEventId,
	}
}

func FromWorkflowExecutionFilter(t *types.WorkflowExecutionFilter) *apiv1.WorkflowExecutionFilter {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionFilter{
		WorkflowId: t.WorkflowID,
		RunId:      t.RunID,
	}
}

func ToWorkflowExecutionFilter(t *apiv1.WorkflowExecutionFilter) *types.WorkflowExecutionFilter {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionFilter{
		WorkflowID: t.WorkflowId,
		RunID:      t.RunId,
	}
}

func FromParentExecutionInfo(t *types.ParentExecutionInfo) *apiv1.ParentExecutionInfo {
	if t == nil {
		return nil
	}
	return &apiv1.ParentExecutionInfo{
		DomainId:          t.DomainUUID,
		DomainName:        t.Domain,
		WorkflowExecution: FromWorkflowExecution(t.Execution),
		InitiatedId:       t.InitiatedID,
	}
}

func ToParentExecutionInfo(t *apiv1.ParentExecutionInfo) *types.ParentExecutionInfo {
	if t == nil {
		return nil
	}
	return &types.ParentExecutionInfo{
		DomainUUID:  t.DomainId,
		Domain:      t.DomainName,
		Execution:   ToWorkflowExecution(t.WorkflowExecution),
		InitiatedID: t.InitiatedId,
	}
}

func FromParentExecutionInfoFields(domainID, domainName *string, we *types.WorkflowExecution, initiatedID *int64) *apiv1.ParentExecutionInfo {
	if domainID == nil && domainName == nil && we == nil && initiatedID == nil {
		return nil
	}

	// ParentExecutionInfo wrapper was added to unify parent related fields.
	// However some fields may not be present:
	// - on older histories
	// - if conversion involves thrift data types
	// Fallback to zero values in those cases
	return &apiv1.ParentExecutionInfo{
		DomainId:          common.StringDefault(domainID),
		DomainName:        common.StringDefault(domainName),
		WorkflowExecution: FromWorkflowExecution(we),
		InitiatedId:       common.Int64Default(initiatedID),
	}
}

func ToParentDomainID(pei *apiv1.ParentExecutionInfo) *string {
	if pei == nil {
		return nil
	}
	return &pei.DomainId
}

func ToParentDomainName(pei *apiv1.ParentExecutionInfo) *string {
	if pei == nil {
		return nil
	}
	return &pei.DomainName
}

func ToParentWorkflowExecution(pei *apiv1.ParentExecutionInfo) *types.WorkflowExecution {
	if pei == nil {
		return nil
	}
	return ToWorkflowExecution(pei.WorkflowExecution)
}

func ToParentInitiatedID(pei *apiv1.ParentExecutionInfo) *int64 {
	if pei == nil {
		return nil
	}
	return &pei.InitiatedId
}

func FromWorkflowExecutionInfo(t *types.WorkflowExecutionInfo) *apiv1.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionInfo{
		WorkflowExecution:   FromWorkflowExecution(t.Execution),
		Type:                FromWorkflowType(t.Type),
		StartTime:           unixNanoToTime(t.StartTime),
		CloseTime:           unixNanoToTime(t.CloseTime),
		CloseStatus:         FromWorkflowExecutionCloseStatus(t.CloseStatus),
		HistoryLength:       t.HistoryLength,
		ParentExecutionInfo: FromParentExecutionInfoFields(t.ParentDomainID, t.ParentDomain, t.ParentExecution, t.ParentInitiatedID),
		ExecutionTime:       unixNanoToTime(t.ExecutionTime),
		Memo:                FromMemo(t.Memo),
		SearchAttributes:    FromSearchAttributes(t.SearchAttributes),
		AutoResetPoints:     FromResetPoints(t.AutoResetPoints),
		TaskList:            t.TaskList,
		PartitionConfig:     t.PartitionConfig,
		IsCron:              t.IsCron,
	}
}

func ToWorkflowExecutionInfo(t *apiv1.WorkflowExecutionInfo) *types.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionInfo{
		Execution:         ToWorkflowExecution(t.WorkflowExecution),
		Type:              ToWorkflowType(t.Type),
		StartTime:         timeToUnixNano(t.StartTime),
		CloseTime:         timeToUnixNano(t.CloseTime),
		CloseStatus:       ToWorkflowExecutionCloseStatus(t.CloseStatus),
		HistoryLength:     t.HistoryLength,
		ParentDomainID:    ToParentDomainID(t.ParentExecutionInfo),
		ParentDomain:      ToParentDomainName(t.ParentExecutionInfo),
		ParentExecution:   ToParentWorkflowExecution(t.ParentExecutionInfo),
		ParentInitiatedID: ToParentInitiatedID(t.ParentExecutionInfo),
		ExecutionTime:     timeToUnixNano(t.ExecutionTime),
		Memo:              ToMemo(t.Memo),
		SearchAttributes:  ToSearchAttributes(t.SearchAttributes),
		AutoResetPoints:   ToResetPoints(t.AutoResetPoints),
		TaskList:          t.TaskList,
		PartitionConfig:   t.PartitionConfig,
		IsCron:            t.IsCron,
	}
}

func FromWorkflowExecutionSignaledEventAttributes(t *types.WorkflowExecutionSignaledEventAttributes) *apiv1.WorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionSignaledEventAttributes{
		SignalName: t.SignalName,
		Input:      FromPayload(t.Input),
		Identity:   t.Identity,
		RequestId:  t.RequestID,
	}
}

func ToWorkflowExecutionSignaledEventAttributes(t *apiv1.WorkflowExecutionSignaledEventAttributes) *types.WorkflowExecutionSignaledEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionSignaledEventAttributes{
		SignalName: t.SignalName,
		Input:      ToPayload(t.Input),
		Identity:   t.Identity,
		RequestID:  t.RequestId,
	}
}

func FromWorkflowExecutionStartedEventAttributes(t *types.WorkflowExecutionStartedEventAttributes) *apiv1.WorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}

	return &apiv1.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		ParentExecutionInfo:          FromParentExecutionInfoFields(t.ParentWorkflowDomainID, t.ParentWorkflowDomain, t.ParentWorkflowExecution, t.ParentInitiatedEventID),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		ContinuedExecutionRunId:      t.ContinuedExecutionRunID,
		Initiator:                    FromContinueAsNewInitiator(t.Initiator),
		ContinuedFailure:             FromFailure(t.ContinuedFailureReason, t.ContinuedFailureDetails),
		LastCompletionResult:         FromPayload(t.LastCompletionResult),
		OriginalExecutionRunId:       t.OriginalExecutionRunID,
		Identity:                     t.Identity,
		FirstExecutionRunId:          t.FirstExecutionRunID,
		FirstScheduledTime:           timeToTimestamp(t.FirstScheduleTime),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		Attempt:                      t.Attempt,
		ExpirationTime:               unixNanoToTime(t.ExpirationTimestamp),
		CronSchedule:                 t.CronSchedule,
		FirstDecisionTaskBackoff:     secondsToDuration(t.FirstDecisionTaskBackoffSeconds),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
		PrevAutoResetPoints:          FromResetPoints(t.PrevAutoResetPoints),
		Header:                       FromHeader(t.Header),
		PartitionConfig:              t.PartitionConfig,
		RequestId:                    t.RequestID,
	}
}

func ToWorkflowExecutionStartedEventAttributes(t *apiv1.WorkflowExecutionStartedEventAttributes) *types.WorkflowExecutionStartedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionStartedEventAttributes{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		ParentWorkflowDomainID:              ToParentDomainID(t.ParentExecutionInfo),
		ParentWorkflowDomain:                ToParentDomainName(t.ParentExecutionInfo),
		ParentWorkflowExecution:             ToParentWorkflowExecution(t.ParentExecutionInfo),
		ParentInitiatedEventID:              ToParentInitiatedID(t.ParentExecutionInfo),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		ContinuedExecutionRunID:             t.ContinuedExecutionRunId,
		Initiator:                           ToContinueAsNewInitiator(t.Initiator),
		ContinuedFailureReason:              ToFailureReason(t.ContinuedFailure),
		ContinuedFailureDetails:             ToFailureDetails(t.ContinuedFailure),
		LastCompletionResult:                ToPayload(t.LastCompletionResult),
		OriginalExecutionRunID:              t.OriginalExecutionRunId,
		Identity:                            t.Identity,
		FirstExecutionRunID:                 t.FirstExecutionRunId,
		FirstScheduleTime:                   timestampToTime(t.FirstScheduledTime),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Attempt:                             t.Attempt,
		ExpirationTimestamp:                 timeToUnixNano(t.ExpirationTime),
		CronSchedule:                        t.CronSchedule,
		FirstDecisionTaskBackoffSeconds:     durationToSeconds(t.FirstDecisionTaskBackoff),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
		PrevAutoResetPoints:                 ToResetPoints(t.PrevAutoResetPoints),
		Header:                              ToHeader(t.Header),
		PartitionConfig:                     t.PartitionConfig,
		RequestID:                           t.RequestId,
	}
}

func FromWorkflowExecutionTerminatedEventAttributes(t *types.WorkflowExecutionTerminatedEventAttributes) *apiv1.WorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionTerminatedEventAttributes{
		Reason:   t.Reason,
		Details:  FromPayload(t.Details),
		Identity: t.Identity,
	}
}

func ToWorkflowExecutionTerminatedEventAttributes(t *apiv1.WorkflowExecutionTerminatedEventAttributes) *types.WorkflowExecutionTerminatedEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionTerminatedEventAttributes{
		Reason:   t.Reason,
		Details:  ToPayload(t.Details),
		Identity: t.Identity,
	}
}

func FromWorkflowExecutionTimedOutEventAttributes(t *types.WorkflowExecutionTimedOutEventAttributes) *apiv1.WorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: FromTimeoutType(t.TimeoutType),
	}
}

func ToWorkflowExecutionTimedOutEventAttributes(t *apiv1.WorkflowExecutionTimedOutEventAttributes) *types.WorkflowExecutionTimedOutEventAttributes {
	if t == nil {
		return nil
	}
	return &types.WorkflowExecutionTimedOutEventAttributes{
		TimeoutType: ToTimeoutType(t.TimeoutType),
	}
}

func FromWorkflowIDReusePolicy(t *types.WorkflowIDReusePolicy) apiv1.WorkflowIdReusePolicy {
	if t == nil {
		return apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID
	}
	switch *t {
	case types.WorkflowIDReusePolicyAllowDuplicateFailedOnly:
		return apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY
	case types.WorkflowIDReusePolicyAllowDuplicate:
		return apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE
	case types.WorkflowIDReusePolicyRejectDuplicate:
		return apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
	case types.WorkflowIDReusePolicyTerminateIfRunning:
		return apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING
	}
	panic("unexpected enum value")
}

func ToWorkflowIDReusePolicy(t apiv1.WorkflowIdReusePolicy) *types.WorkflowIDReusePolicy {
	switch t {
	case apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_INVALID:
		return nil
	case apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY:
		return types.WorkflowIDReusePolicyAllowDuplicateFailedOnly.Ptr()
	case apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE:
		return types.WorkflowIDReusePolicyAllowDuplicate.Ptr()
	case apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE:
		return types.WorkflowIDReusePolicyRejectDuplicate.Ptr()
	case apiv1.WorkflowIdReusePolicy_WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING:
		return types.WorkflowIDReusePolicyTerminateIfRunning.Ptr()
	}
	panic("unexpected enum value")
}

func FromWorkflowQuery(t *types.WorkflowQuery) *apiv1.WorkflowQuery {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowQuery{
		QueryType: t.QueryType,
		QueryArgs: FromPayload(t.QueryArgs),
	}
}

func ToWorkflowQuery(t *apiv1.WorkflowQuery) *types.WorkflowQuery {
	if t == nil {
		return nil
	}
	return &types.WorkflowQuery{
		QueryType: t.QueryType,
		QueryArgs: ToPayload(t.QueryArgs),
	}
}

func FromWorkflowQueryResult(t *types.WorkflowQueryResult) *apiv1.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowQueryResult{
		ResultType:   FromQueryResultType(t.ResultType),
		Answer:       FromPayload(t.Answer),
		ErrorMessage: t.ErrorMessage,
	}
}

func ToWorkflowQueryResult(t *apiv1.WorkflowQueryResult) *types.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	return &types.WorkflowQueryResult{
		ResultType:   ToQueryResultType(t.ResultType),
		Answer:       ToPayload(t.Answer),
		ErrorMessage: t.ErrorMessage,
	}
}

func FromWorkflowType(t *types.WorkflowType) *apiv1.WorkflowType {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowType{
		Name: t.Name,
	}
}

func ToWorkflowType(t *apiv1.WorkflowType) *types.WorkflowType {
	if t == nil {
		return nil
	}
	return &types.WorkflowType{
		Name: t.Name,
	}
}

func FromWorkflowTypeFilter(t *types.WorkflowTypeFilter) *apiv1.WorkflowTypeFilter {
	if t == nil {
		return nil
	}
	return &apiv1.WorkflowTypeFilter{
		Name: t.Name,
	}
}

func ToWorkflowTypeFilter(t *apiv1.WorkflowTypeFilter) *types.WorkflowTypeFilter {
	if t == nil {
		return nil
	}
	return &types.WorkflowTypeFilter{
		Name: t.Name,
	}
}

func FromDataBlobArray(t []*types.DataBlob) []*apiv1.DataBlob {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.DataBlob, len(t))
	for i := range t {
		v[i] = FromDataBlob(t[i])
	}
	return v
}

func ToDataBlobArray(t []*apiv1.DataBlob) []*types.DataBlob {
	if t == nil {
		return nil
	}
	v := make([]*types.DataBlob, len(t))
	for i := range t {
		v[i] = ToDataBlob(t[i])
	}
	return v
}

func FromHistoryEventArray(t []*types.HistoryEvent) []*apiv1.HistoryEvent {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.HistoryEvent, len(t))
	for i := range t {
		v[i] = FromHistoryEvent(t[i])
	}
	return v
}

func ToHistoryEventArray(t []*apiv1.HistoryEvent) []*types.HistoryEvent {
	if t == nil {
		return []*types.HistoryEvent{}
	}
	v := make([]*types.HistoryEvent, len(t))
	for i := range t {
		v[i] = ToHistoryEvent(t[i])
	}
	return v
}

func FromTaskListPartitionMetadataArray(t []*types.TaskListPartitionMetadata) []*apiv1.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.TaskListPartitionMetadata, len(t))
	for i := range t {
		v[i] = FromTaskListPartitionMetadata(t[i])
	}
	return v
}

func ToTaskListPartitionMetadataArray(t []*apiv1.TaskListPartitionMetadata) []*types.TaskListPartitionMetadata {
	if t == nil {
		return nil
	}
	v := make([]*types.TaskListPartitionMetadata, len(t))
	for i := range t {
		v[i] = ToTaskListPartitionMetadata(t[i])
	}
	return v
}

func FromDecisionArray(t []*types.Decision) []*apiv1.Decision {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.Decision, len(t))
	for i := range t {
		v[i] = FromDecision(t[i])
	}
	return v
}

func ToDecisionArray(t []*apiv1.Decision) []*types.Decision {
	if t == nil {
		return nil
	}
	v := make([]*types.Decision, len(t))
	for i := range t {
		v[i] = ToDecision(t[i])
	}
	return v
}

func FromPollerInfoArray(t []*types.PollerInfo) []*apiv1.PollerInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.PollerInfo, len(t))
	for i := range t {
		v[i] = FromPollerInfo(t[i])
	}
	return v
}

func ToPollerInfoArray(t []*apiv1.PollerInfo) []*types.PollerInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PollerInfo, len(t))
	for i := range t {
		v[i] = ToPollerInfo(t[i])
	}
	return v
}

func FromPendingChildExecutionInfoArray(t []*types.PendingChildExecutionInfo) []*apiv1.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.PendingChildExecutionInfo, len(t))
	for i := range t {
		v[i] = FromPendingChildExecutionInfo(t[i])
	}
	return v
}

func ToPendingChildExecutionInfoArray(t []*apiv1.PendingChildExecutionInfo) []*types.PendingChildExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PendingChildExecutionInfo, len(t))
	for i := range t {
		v[i] = ToPendingChildExecutionInfo(t[i])
	}
	return v
}

func FromWorkflowExecutionInfoArray(t []*types.WorkflowExecutionInfo) []*apiv1.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.WorkflowExecutionInfo, len(t))
	for i := range t {
		v[i] = FromWorkflowExecutionInfo(t[i])
	}
	return v
}

func ToWorkflowExecutionInfoArray(t []*apiv1.WorkflowExecutionInfo) []*types.WorkflowExecutionInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.WorkflowExecutionInfo, len(t))
	for i := range t {
		v[i] = ToWorkflowExecutionInfo(t[i])
	}
	return v
}

func FromDescribeDomainResponseArray(t []*types.DescribeDomainResponse) []*apiv1.Domain {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.Domain, len(t))
	for i := range t {
		v[i] = FromDescribeDomainResponseDomain(t[i])
	}
	return v
}

func ToDescribeDomainResponseArray(t []*apiv1.Domain) []*types.DescribeDomainResponse {
	if t == nil {
		return nil
	}
	v := make([]*types.DescribeDomainResponse, len(t))
	for i := range t {
		v[i] = ToDescribeDomainResponseDomain(t[i])
	}
	return v
}

func FromResetPointInfoArray(t []*types.ResetPointInfo) []*apiv1.ResetPointInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.ResetPointInfo, len(t))
	for i := range t {
		v[i] = FromResetPointInfo(t[i])
	}
	return v
}

func ToResetPointInfoArray(t []*apiv1.ResetPointInfo) []*types.ResetPointInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.ResetPointInfo, len(t))
	for i := range t {
		v[i] = ToResetPointInfo(t[i])
	}
	return v
}

func FromPendingActivityInfoArray(t []*types.PendingActivityInfo) []*apiv1.PendingActivityInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.PendingActivityInfo, len(t))
	for i := range t {
		v[i] = FromPendingActivityInfo(t[i])
	}
	return v
}

func ToPendingActivityInfoArray(t []*apiv1.PendingActivityInfo) []*types.PendingActivityInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.PendingActivityInfo, len(t))
	for i := range t {
		v[i] = ToPendingActivityInfo(t[i])
	}
	return v
}

func FromClusterReplicationConfigurationArray(t []*types.ClusterReplicationConfiguration) []*apiv1.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.ClusterReplicationConfiguration, len(t))
	for i := range t {
		v[i] = FromClusterReplicationConfiguration(t[i])
	}
	return v
}

func ToClusterReplicationConfigurationArray(t []*apiv1.ClusterReplicationConfiguration) []*types.ClusterReplicationConfiguration {
	if t == nil {
		return nil
	}
	v := make([]*types.ClusterReplicationConfiguration, len(t))
	for i := range t {
		v[i] = ToClusterReplicationConfiguration(t[i])
	}
	return v
}

func FromActivityLocalDispatchInfoMap(t map[string]*types.ActivityLocalDispatchInfo) map[string]*apiv1.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.ActivityLocalDispatchInfo, len(t))
	for key := range t {
		v[key] = FromActivityLocalDispatchInfo(t[key])
	}
	return v
}

func ToActivityLocalDispatchInfoMap(t map[string]*apiv1.ActivityLocalDispatchInfo) map[string]*types.ActivityLocalDispatchInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.ActivityLocalDispatchInfo, len(t))
	for key := range t {
		v[key] = ToActivityLocalDispatchInfo(t[key])
	}
	return v
}

func FromBadBinaryInfoMap(t map[string]*types.BadBinaryInfo) map[string]*apiv1.BadBinaryInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.BadBinaryInfo, len(t))
	for key := range t {
		v[key] = FromBadBinaryInfo(t[key])
	}
	return v
}

func ToBadBinaryInfoMap(t map[string]*apiv1.BadBinaryInfo) map[string]*types.BadBinaryInfo {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.BadBinaryInfo, len(t))
	for key := range t {
		v[key] = ToBadBinaryInfo(t[key])
	}
	return v
}

func FromIndexedValueTypeMap(t map[string]types.IndexedValueType) map[string]apiv1.IndexedValueType {
	if t == nil {
		return nil
	}
	v := make(map[string]apiv1.IndexedValueType, len(t))
	for key := range t {
		v[key] = FromIndexedValueType(t[key])
	}
	return v
}

func ToIndexedValueTypeMap(t map[string]apiv1.IndexedValueType) map[string]types.IndexedValueType {
	if t == nil {
		return nil
	}
	v := make(map[string]types.IndexedValueType, len(t))
	for key := range t {
		v[key] = ToIndexedValueType(t[key])
	}
	return v
}

func FromWorkflowQueryMap(t map[string]*types.WorkflowQuery) map[string]*apiv1.WorkflowQuery {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.WorkflowQuery, len(t))
	for key := range t {
		v[key] = FromWorkflowQuery(t[key])
	}
	return v
}

func ToWorkflowQueryMap(t map[string]*apiv1.WorkflowQuery) map[string]*types.WorkflowQuery {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.WorkflowQuery, len(t))
	for key := range t {
		v[key] = ToWorkflowQuery(t[key])
	}
	return v
}

func FromWorkflowQueryResultMap(t map[string]*types.WorkflowQueryResult) map[string]*apiv1.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.WorkflowQueryResult, len(t))
	for key := range t {
		v[key] = FromWorkflowQueryResult(t[key])
	}
	return v
}

func ToWorkflowQueryResultMap(t map[string]*apiv1.WorkflowQueryResult) map[string]*types.WorkflowQueryResult {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.WorkflowQueryResult, len(t))
	for key := range t {
		v[key] = ToWorkflowQueryResult(t[key])
	}
	return v
}

func FromIsolationGroupMetricsMap(t map[string]*types.IsolationGroupMetrics) map[string]*apiv1.IsolationGroupMetrics {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.IsolationGroupMetrics, len(t))
	for key := range t {
		v[key] = FromIsolationGroupMetrics(t[key])
	}
	return v
}

func ToIsolationGroupMetricsMap(t map[string]*apiv1.IsolationGroupMetrics) map[string]*types.IsolationGroupMetrics {
	if t == nil {
		return nil
	}
	v := make(map[string]*types.IsolationGroupMetrics, len(t))
	for key := range t {
		v[key] = ToIsolationGroupMetrics(t[key])
	}
	return v
}

func FromPayload(data []byte) *apiv1.Payload {
	if data == nil {
		return nil
	}
	return &apiv1.Payload{
		Data: data,
	}
}

func ToPayload(p *apiv1.Payload) []byte {
	if p == nil {
		return nil
	}
	if p.Data == nil {
		// FromPayload will not generate this case
		// however, Data field will be dropped by the encoding if it's empty
		// and receiver side will see nil for the Data field
		// since we already know p is not nil, Data field must be an empty byte array
		return []byte{}
	}
	return p.Data
}

func FromPayloadMap(t map[string][]byte) map[string]*apiv1.Payload {
	if t == nil {
		return nil
	}
	v := make(map[string]*apiv1.Payload, len(t))
	for key := range t {
		v[key] = FromPayload(t[key])
	}
	return v
}

func ToPayloadMap(t map[string]*apiv1.Payload) map[string][]byte {
	if t == nil {
		return nil
	}
	v := make(map[string][]byte, len(t))
	for key := range t {
		v[key] = ToPayload(t[key])
	}
	return v
}

func FromFailure(reason *string, details []byte) *apiv1.Failure {
	if reason == nil {
		return nil
	}
	return &apiv1.Failure{
		Reason:  *reason,
		Details: details,
	}
}

func ToFailureReason(failure *apiv1.Failure) *string {
	if failure == nil {
		return nil
	}
	return &failure.Reason
}

func ToFailureDetails(failure *apiv1.Failure) []byte {
	if failure == nil {
		return nil
	}
	return failure.Details
}

func FromHistoryEvent(e *types.HistoryEvent) *apiv1.HistoryEvent {
	if e == nil {
		return nil
	}
	event := apiv1.HistoryEvent{
		EventId:   e.ID,
		EventTime: unixNanoToTime(e.Timestamp),
		Version:   e.Version,
		TaskId:    e.TaskID,
	}
	switch *e.EventType {
	case types.EventTypeWorkflowExecutionStarted:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionStartedEventAttributes{
			WorkflowExecutionStartedEventAttributes: FromWorkflowExecutionStartedEventAttributes(e.WorkflowExecutionStartedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionCompleted:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionCompletedEventAttributes{
			WorkflowExecutionCompletedEventAttributes: FromWorkflowExecutionCompletedEventAttributes(e.WorkflowExecutionCompletedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionFailed:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionFailedEventAttributes{
			WorkflowExecutionFailedEventAttributes: FromWorkflowExecutionFailedEventAttributes(e.WorkflowExecutionFailedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionTimedOut:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionTimedOutEventAttributes{
			WorkflowExecutionTimedOutEventAttributes: FromWorkflowExecutionTimedOutEventAttributes(e.WorkflowExecutionTimedOutEventAttributes),
		}
	case types.EventTypeDecisionTaskScheduled:
		event.Attributes = &apiv1.HistoryEvent_DecisionTaskScheduledEventAttributes{
			DecisionTaskScheduledEventAttributes: FromDecisionTaskScheduledEventAttributes(e.DecisionTaskScheduledEventAttributes),
		}
	case types.EventTypeDecisionTaskStarted:
		event.Attributes = &apiv1.HistoryEvent_DecisionTaskStartedEventAttributes{
			DecisionTaskStartedEventAttributes: FromDecisionTaskStartedEventAttributes(e.DecisionTaskStartedEventAttributes),
		}
	case types.EventTypeDecisionTaskCompleted:
		event.Attributes = &apiv1.HistoryEvent_DecisionTaskCompletedEventAttributes{
			DecisionTaskCompletedEventAttributes: FromDecisionTaskCompletedEventAttributes(e.DecisionTaskCompletedEventAttributes),
		}
	case types.EventTypeDecisionTaskTimedOut:
		event.Attributes = &apiv1.HistoryEvent_DecisionTaskTimedOutEventAttributes{
			DecisionTaskTimedOutEventAttributes: FromDecisionTaskTimedOutEventAttributes(e.DecisionTaskTimedOutEventAttributes),
		}
	case types.EventTypeDecisionTaskFailed:
		event.Attributes = &apiv1.HistoryEvent_DecisionTaskFailedEventAttributes{
			DecisionTaskFailedEventAttributes: FromDecisionTaskFailedEventAttributes(e.DecisionTaskFailedEventAttributes),
		}
	case types.EventTypeActivityTaskScheduled:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskScheduledEventAttributes{
			ActivityTaskScheduledEventAttributes: FromActivityTaskScheduledEventAttributes(e.ActivityTaskScheduledEventAttributes),
		}
	case types.EventTypeActivityTaskStarted:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskStartedEventAttributes{
			ActivityTaskStartedEventAttributes: FromActivityTaskStartedEventAttributes(e.ActivityTaskStartedEventAttributes),
		}
	case types.EventTypeActivityTaskCompleted:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskCompletedEventAttributes{
			ActivityTaskCompletedEventAttributes: FromActivityTaskCompletedEventAttributes(e.ActivityTaskCompletedEventAttributes),
		}
	case types.EventTypeActivityTaskFailed:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskFailedEventAttributes{
			ActivityTaskFailedEventAttributes: FromActivityTaskFailedEventAttributes(e.ActivityTaskFailedEventAttributes),
		}
	case types.EventTypeActivityTaskTimedOut:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskTimedOutEventAttributes{
			ActivityTaskTimedOutEventAttributes: FromActivityTaskTimedOutEventAttributes(e.ActivityTaskTimedOutEventAttributes),
		}
	case types.EventTypeActivityTaskCancelRequested:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskCancelRequestedEventAttributes{
			ActivityTaskCancelRequestedEventAttributes: FromActivityTaskCancelRequestedEventAttributes(e.ActivityTaskCancelRequestedEventAttributes),
		}
	case types.EventTypeRequestCancelActivityTaskFailed:
		event.Attributes = &apiv1.HistoryEvent_RequestCancelActivityTaskFailedEventAttributes{
			RequestCancelActivityTaskFailedEventAttributes: FromRequestCancelActivityTaskFailedEventAttributes(e.RequestCancelActivityTaskFailedEventAttributes),
		}
	case types.EventTypeActivityTaskCanceled:
		event.Attributes = &apiv1.HistoryEvent_ActivityTaskCanceledEventAttributes{
			ActivityTaskCanceledEventAttributes: FromActivityTaskCanceledEventAttributes(e.ActivityTaskCanceledEventAttributes),
		}
	case types.EventTypeTimerStarted:
		event.Attributes = &apiv1.HistoryEvent_TimerStartedEventAttributes{
			TimerStartedEventAttributes: FromTimerStartedEventAttributes(e.TimerStartedEventAttributes),
		}
	case types.EventTypeTimerFired:
		event.Attributes = &apiv1.HistoryEvent_TimerFiredEventAttributes{
			TimerFiredEventAttributes: FromTimerFiredEventAttributes(e.TimerFiredEventAttributes),
		}
	case types.EventTypeTimerCanceled:
		event.Attributes = &apiv1.HistoryEvent_TimerCanceledEventAttributes{
			TimerCanceledEventAttributes: FromTimerCanceledEventAttributes(e.TimerCanceledEventAttributes),
		}
	case types.EventTypeCancelTimerFailed:
		event.Attributes = &apiv1.HistoryEvent_CancelTimerFailedEventAttributes{
			CancelTimerFailedEventAttributes: FromCancelTimerFailedEventAttributes(e.CancelTimerFailedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionCancelRequested:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionCancelRequestedEventAttributes{
			WorkflowExecutionCancelRequestedEventAttributes: FromWorkflowExecutionCancelRequestedEventAttributes(e.WorkflowExecutionCancelRequestedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionCanceled:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionCanceledEventAttributes{
			WorkflowExecutionCanceledEventAttributes: FromWorkflowExecutionCanceledEventAttributes(e.WorkflowExecutionCanceledEventAttributes),
		}
	case types.EventTypeRequestCancelExternalWorkflowExecutionInitiated:
		event.Attributes = &apiv1.HistoryEvent_RequestCancelExternalWorkflowExecutionInitiatedEventAttributes{
			RequestCancelExternalWorkflowExecutionInitiatedEventAttributes: FromRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(e.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes),
		}
	case types.EventTypeRequestCancelExternalWorkflowExecutionFailed:
		event.Attributes = &apiv1.HistoryEvent_RequestCancelExternalWorkflowExecutionFailedEventAttributes{
			RequestCancelExternalWorkflowExecutionFailedEventAttributes: FromRequestCancelExternalWorkflowExecutionFailedEventAttributes(e.RequestCancelExternalWorkflowExecutionFailedEventAttributes),
		}
	case types.EventTypeExternalWorkflowExecutionCancelRequested:
		event.Attributes = &apiv1.HistoryEvent_ExternalWorkflowExecutionCancelRequestedEventAttributes{
			ExternalWorkflowExecutionCancelRequestedEventAttributes: FromExternalWorkflowExecutionCancelRequestedEventAttributes(e.ExternalWorkflowExecutionCancelRequestedEventAttributes),
		}
	case types.EventTypeMarkerRecorded:
		event.Attributes = &apiv1.HistoryEvent_MarkerRecordedEventAttributes{
			MarkerRecordedEventAttributes: FromMarkerRecordedEventAttributes(e.MarkerRecordedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionSignaled:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionSignaledEventAttributes{
			WorkflowExecutionSignaledEventAttributes: FromWorkflowExecutionSignaledEventAttributes(e.WorkflowExecutionSignaledEventAttributes),
		}
	case types.EventTypeWorkflowExecutionTerminated:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionTerminatedEventAttributes{
			WorkflowExecutionTerminatedEventAttributes: FromWorkflowExecutionTerminatedEventAttributes(e.WorkflowExecutionTerminatedEventAttributes),
		}
	case types.EventTypeWorkflowExecutionContinuedAsNew:
		event.Attributes = &apiv1.HistoryEvent_WorkflowExecutionContinuedAsNewEventAttributes{
			WorkflowExecutionContinuedAsNewEventAttributes: FromWorkflowExecutionContinuedAsNewEventAttributes(e.WorkflowExecutionContinuedAsNewEventAttributes),
		}
	case types.EventTypeStartChildWorkflowExecutionInitiated:
		event.Attributes = &apiv1.HistoryEvent_StartChildWorkflowExecutionInitiatedEventAttributes{
			StartChildWorkflowExecutionInitiatedEventAttributes: FromStartChildWorkflowExecutionInitiatedEventAttributes(e.StartChildWorkflowExecutionInitiatedEventAttributes),
		}
	case types.EventTypeStartChildWorkflowExecutionFailed:
		event.Attributes = &apiv1.HistoryEvent_StartChildWorkflowExecutionFailedEventAttributes{
			StartChildWorkflowExecutionFailedEventAttributes: FromStartChildWorkflowExecutionFailedEventAttributes(e.StartChildWorkflowExecutionFailedEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionStarted:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionStartedEventAttributes{
			ChildWorkflowExecutionStartedEventAttributes: FromChildWorkflowExecutionStartedEventAttributes(e.ChildWorkflowExecutionStartedEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionCompleted:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionCompletedEventAttributes{
			ChildWorkflowExecutionCompletedEventAttributes: FromChildWorkflowExecutionCompletedEventAttributes(e.ChildWorkflowExecutionCompletedEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionFailed:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionFailedEventAttributes{
			ChildWorkflowExecutionFailedEventAttributes: FromChildWorkflowExecutionFailedEventAttributes(e.ChildWorkflowExecutionFailedEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionCanceled:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionCanceledEventAttributes{
			ChildWorkflowExecutionCanceledEventAttributes: FromChildWorkflowExecutionCanceledEventAttributes(e.ChildWorkflowExecutionCanceledEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionTimedOut:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionTimedOutEventAttributes{
			ChildWorkflowExecutionTimedOutEventAttributes: FromChildWorkflowExecutionTimedOutEventAttributes(e.ChildWorkflowExecutionTimedOutEventAttributes),
		}
	case types.EventTypeChildWorkflowExecutionTerminated:
		event.Attributes = &apiv1.HistoryEvent_ChildWorkflowExecutionTerminatedEventAttributes{
			ChildWorkflowExecutionTerminatedEventAttributes: FromChildWorkflowExecutionTerminatedEventAttributes(e.ChildWorkflowExecutionTerminatedEventAttributes),
		}
	case types.EventTypeSignalExternalWorkflowExecutionInitiated:
		event.Attributes = &apiv1.HistoryEvent_SignalExternalWorkflowExecutionInitiatedEventAttributes{
			SignalExternalWorkflowExecutionInitiatedEventAttributes: FromSignalExternalWorkflowExecutionInitiatedEventAttributes(e.SignalExternalWorkflowExecutionInitiatedEventAttributes),
		}
	case types.EventTypeSignalExternalWorkflowExecutionFailed:
		event.Attributes = &apiv1.HistoryEvent_SignalExternalWorkflowExecutionFailedEventAttributes{
			SignalExternalWorkflowExecutionFailedEventAttributes: FromSignalExternalWorkflowExecutionFailedEventAttributes(e.SignalExternalWorkflowExecutionFailedEventAttributes),
		}
	case types.EventTypeExternalWorkflowExecutionSignaled:
		event.Attributes = &apiv1.HistoryEvent_ExternalWorkflowExecutionSignaledEventAttributes{
			ExternalWorkflowExecutionSignaledEventAttributes: FromExternalWorkflowExecutionSignaledEventAttributes(e.ExternalWorkflowExecutionSignaledEventAttributes),
		}
	case types.EventTypeUpsertWorkflowSearchAttributes:
		event.Attributes = &apiv1.HistoryEvent_UpsertWorkflowSearchAttributesEventAttributes{
			UpsertWorkflowSearchAttributesEventAttributes: FromUpsertWorkflowSearchAttributesEventAttributes(e.UpsertWorkflowSearchAttributesEventAttributes),
		}
	}
	return &event
}

func ToHistoryEvent(e *apiv1.HistoryEvent) *types.HistoryEvent {
	if e == nil {
		return nil
	}
	event := types.HistoryEvent{
		ID:        e.EventId,
		Timestamp: timeToUnixNano(e.EventTime),
		Version:   e.Version,
		TaskID:    e.TaskId,
	}
	switch attr := e.Attributes.(type) {
	case *apiv1.HistoryEvent_WorkflowExecutionStartedEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionStarted.Ptr()
		event.WorkflowExecutionStartedEventAttributes = ToWorkflowExecutionStartedEventAttributes(attr.WorkflowExecutionStartedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionCompletedEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionCompleted.Ptr()
		event.WorkflowExecutionCompletedEventAttributes = ToWorkflowExecutionCompletedEventAttributes(attr.WorkflowExecutionCompletedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionFailedEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionFailed.Ptr()
		event.WorkflowExecutionFailedEventAttributes = ToWorkflowExecutionFailedEventAttributes(attr.WorkflowExecutionFailedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionTimedOutEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionTimedOut.Ptr()
		event.WorkflowExecutionTimedOutEventAttributes = ToWorkflowExecutionTimedOutEventAttributes(attr.WorkflowExecutionTimedOutEventAttributes)
	case *apiv1.HistoryEvent_DecisionTaskScheduledEventAttributes:
		event.EventType = types.EventTypeDecisionTaskScheduled.Ptr()
		event.DecisionTaskScheduledEventAttributes = ToDecisionTaskScheduledEventAttributes(attr.DecisionTaskScheduledEventAttributes)
	case *apiv1.HistoryEvent_DecisionTaskStartedEventAttributes:
		event.EventType = types.EventTypeDecisionTaskStarted.Ptr()
		event.DecisionTaskStartedEventAttributes = ToDecisionTaskStartedEventAttributes(attr.DecisionTaskStartedEventAttributes)
	case *apiv1.HistoryEvent_DecisionTaskCompletedEventAttributes:
		event.EventType = types.EventTypeDecisionTaskCompleted.Ptr()
		event.DecisionTaskCompletedEventAttributes = ToDecisionTaskCompletedEventAttributes(attr.DecisionTaskCompletedEventAttributes)
	case *apiv1.HistoryEvent_DecisionTaskTimedOutEventAttributes:
		event.EventType = types.EventTypeDecisionTaskTimedOut.Ptr()
		event.DecisionTaskTimedOutEventAttributes = ToDecisionTaskTimedOutEventAttributes(attr.DecisionTaskTimedOutEventAttributes)
	case *apiv1.HistoryEvent_DecisionTaskFailedEventAttributes:
		event.EventType = types.EventTypeDecisionTaskFailed.Ptr()
		event.DecisionTaskFailedEventAttributes = ToDecisionTaskFailedEventAttributes(attr.DecisionTaskFailedEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskScheduledEventAttributes:
		event.EventType = types.EventTypeActivityTaskScheduled.Ptr()
		event.ActivityTaskScheduledEventAttributes = ToActivityTaskScheduledEventAttributes(attr.ActivityTaskScheduledEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskStartedEventAttributes:
		event.EventType = types.EventTypeActivityTaskStarted.Ptr()
		event.ActivityTaskStartedEventAttributes = ToActivityTaskStartedEventAttributes(attr.ActivityTaskStartedEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskCompletedEventAttributes:
		event.EventType = types.EventTypeActivityTaskCompleted.Ptr()
		event.ActivityTaskCompletedEventAttributes = ToActivityTaskCompletedEventAttributes(attr.ActivityTaskCompletedEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskFailedEventAttributes:
		event.EventType = types.EventTypeActivityTaskFailed.Ptr()
		event.ActivityTaskFailedEventAttributes = ToActivityTaskFailedEventAttributes(attr.ActivityTaskFailedEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskTimedOutEventAttributes:
		event.EventType = types.EventTypeActivityTaskTimedOut.Ptr()
		event.ActivityTaskTimedOutEventAttributes = ToActivityTaskTimedOutEventAttributes(attr.ActivityTaskTimedOutEventAttributes)
	case *apiv1.HistoryEvent_TimerStartedEventAttributes:
		event.EventType = types.EventTypeTimerStarted.Ptr()
		event.TimerStartedEventAttributes = ToTimerStartedEventAttributes(attr.TimerStartedEventAttributes)
	case *apiv1.HistoryEvent_TimerFiredEventAttributes:
		event.EventType = types.EventTypeTimerFired.Ptr()
		event.TimerFiredEventAttributes = ToTimerFiredEventAttributes(attr.TimerFiredEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskCancelRequestedEventAttributes:
		event.EventType = types.EventTypeActivityTaskCancelRequested.Ptr()
		event.ActivityTaskCancelRequestedEventAttributes = ToActivityTaskCancelRequestedEventAttributes(attr.ActivityTaskCancelRequestedEventAttributes)
	case *apiv1.HistoryEvent_RequestCancelActivityTaskFailedEventAttributes:
		event.EventType = types.EventTypeRequestCancelActivityTaskFailed.Ptr()
		event.RequestCancelActivityTaskFailedEventAttributes = ToRequestCancelActivityTaskFailedEventAttributes(attr.RequestCancelActivityTaskFailedEventAttributes)
	case *apiv1.HistoryEvent_ActivityTaskCanceledEventAttributes:
		event.EventType = types.EventTypeActivityTaskCanceled.Ptr()
		event.ActivityTaskCanceledEventAttributes = ToActivityTaskCanceledEventAttributes(attr.ActivityTaskCanceledEventAttributes)
	case *apiv1.HistoryEvent_TimerCanceledEventAttributes:
		event.EventType = types.EventTypeTimerCanceled.Ptr()
		event.TimerCanceledEventAttributes = ToTimerCanceledEventAttributes(attr.TimerCanceledEventAttributes)
	case *apiv1.HistoryEvent_CancelTimerFailedEventAttributes:
		event.EventType = types.EventTypeCancelTimerFailed.Ptr()
		event.CancelTimerFailedEventAttributes = ToCancelTimerFailedEventAttributes(attr.CancelTimerFailedEventAttributes)
	case *apiv1.HistoryEvent_MarkerRecordedEventAttributes:
		event.EventType = types.EventTypeMarkerRecorded.Ptr()
		event.MarkerRecordedEventAttributes = ToMarkerRecordedEventAttributes(attr.MarkerRecordedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionSignaledEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionSignaled.Ptr()
		event.WorkflowExecutionSignaledEventAttributes = ToWorkflowExecutionSignaledEventAttributes(attr.WorkflowExecutionSignaledEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionTerminatedEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionTerminated.Ptr()
		event.WorkflowExecutionTerminatedEventAttributes = ToWorkflowExecutionTerminatedEventAttributes(attr.WorkflowExecutionTerminatedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionCancelRequestedEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionCancelRequested.Ptr()
		event.WorkflowExecutionCancelRequestedEventAttributes = ToWorkflowExecutionCancelRequestedEventAttributes(attr.WorkflowExecutionCancelRequestedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionCanceledEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionCanceled.Ptr()
		event.WorkflowExecutionCanceledEventAttributes = ToWorkflowExecutionCanceledEventAttributes(attr.WorkflowExecutionCanceledEventAttributes)
	case *apiv1.HistoryEvent_RequestCancelExternalWorkflowExecutionInitiatedEventAttributes:
		event.EventType = types.EventTypeRequestCancelExternalWorkflowExecutionInitiated.Ptr()
		event.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes = ToRequestCancelExternalWorkflowExecutionInitiatedEventAttributes(attr.RequestCancelExternalWorkflowExecutionInitiatedEventAttributes)
	case *apiv1.HistoryEvent_RequestCancelExternalWorkflowExecutionFailedEventAttributes:
		event.EventType = types.EventTypeRequestCancelExternalWorkflowExecutionFailed.Ptr()
		event.RequestCancelExternalWorkflowExecutionFailedEventAttributes = ToRequestCancelExternalWorkflowExecutionFailedEventAttributes(attr.RequestCancelExternalWorkflowExecutionFailedEventAttributes)
	case *apiv1.HistoryEvent_ExternalWorkflowExecutionCancelRequestedEventAttributes:
		event.EventType = types.EventTypeExternalWorkflowExecutionCancelRequested.Ptr()
		event.ExternalWorkflowExecutionCancelRequestedEventAttributes = ToExternalWorkflowExecutionCancelRequestedEventAttributes(attr.ExternalWorkflowExecutionCancelRequestedEventAttributes)
	case *apiv1.HistoryEvent_WorkflowExecutionContinuedAsNewEventAttributes:
		event.EventType = types.EventTypeWorkflowExecutionContinuedAsNew.Ptr()
		event.WorkflowExecutionContinuedAsNewEventAttributes = ToWorkflowExecutionContinuedAsNewEventAttributes(attr.WorkflowExecutionContinuedAsNewEventAttributes)
	case *apiv1.HistoryEvent_StartChildWorkflowExecutionInitiatedEventAttributes:
		event.EventType = types.EventTypeStartChildWorkflowExecutionInitiated.Ptr()
		event.StartChildWorkflowExecutionInitiatedEventAttributes = ToStartChildWorkflowExecutionInitiatedEventAttributes(attr.StartChildWorkflowExecutionInitiatedEventAttributes)
	case *apiv1.HistoryEvent_StartChildWorkflowExecutionFailedEventAttributes:
		event.EventType = types.EventTypeStartChildWorkflowExecutionFailed.Ptr()
		event.StartChildWorkflowExecutionFailedEventAttributes = ToStartChildWorkflowExecutionFailedEventAttributes(attr.StartChildWorkflowExecutionFailedEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionStartedEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionStarted.Ptr()
		event.ChildWorkflowExecutionStartedEventAttributes = ToChildWorkflowExecutionStartedEventAttributes(attr.ChildWorkflowExecutionStartedEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionCompletedEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionCompleted.Ptr()
		event.ChildWorkflowExecutionCompletedEventAttributes = ToChildWorkflowExecutionCompletedEventAttributes(attr.ChildWorkflowExecutionCompletedEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionFailedEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionFailed.Ptr()
		event.ChildWorkflowExecutionFailedEventAttributes = ToChildWorkflowExecutionFailedEventAttributes(attr.ChildWorkflowExecutionFailedEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionCanceledEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionCanceled.Ptr()
		event.ChildWorkflowExecutionCanceledEventAttributes = ToChildWorkflowExecutionCanceledEventAttributes(attr.ChildWorkflowExecutionCanceledEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionTimedOutEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionTimedOut.Ptr()
		event.ChildWorkflowExecutionTimedOutEventAttributes = ToChildWorkflowExecutionTimedOutEventAttributes(attr.ChildWorkflowExecutionTimedOutEventAttributes)
	case *apiv1.HistoryEvent_ChildWorkflowExecutionTerminatedEventAttributes:
		event.EventType = types.EventTypeChildWorkflowExecutionTerminated.Ptr()
		event.ChildWorkflowExecutionTerminatedEventAttributes = ToChildWorkflowExecutionTerminatedEventAttributes(attr.ChildWorkflowExecutionTerminatedEventAttributes)
	case *apiv1.HistoryEvent_SignalExternalWorkflowExecutionInitiatedEventAttributes:
		event.EventType = types.EventTypeSignalExternalWorkflowExecutionInitiated.Ptr()
		event.SignalExternalWorkflowExecutionInitiatedEventAttributes = ToSignalExternalWorkflowExecutionInitiatedEventAttributes(attr.SignalExternalWorkflowExecutionInitiatedEventAttributes)
	case *apiv1.HistoryEvent_SignalExternalWorkflowExecutionFailedEventAttributes:
		event.EventType = types.EventTypeSignalExternalWorkflowExecutionFailed.Ptr()
		event.SignalExternalWorkflowExecutionFailedEventAttributes = ToSignalExternalWorkflowExecutionFailedEventAttributes(attr.SignalExternalWorkflowExecutionFailedEventAttributes)
	case *apiv1.HistoryEvent_ExternalWorkflowExecutionSignaledEventAttributes:
		event.EventType = types.EventTypeExternalWorkflowExecutionSignaled.Ptr()
		event.ExternalWorkflowExecutionSignaledEventAttributes = ToExternalWorkflowExecutionSignaledEventAttributes(attr.ExternalWorkflowExecutionSignaledEventAttributes)
	case *apiv1.HistoryEvent_UpsertWorkflowSearchAttributesEventAttributes:
		event.EventType = types.EventTypeUpsertWorkflowSearchAttributes.Ptr()
		event.UpsertWorkflowSearchAttributesEventAttributes = ToUpsertWorkflowSearchAttributesEventAttributes(attr.UpsertWorkflowSearchAttributesEventAttributes)
	}
	return &event
}

func FromDecision(d *types.Decision) *apiv1.Decision {
	if d == nil {
		return nil
	}
	decision := apiv1.Decision{}
	switch *d.DecisionType {
	case types.DecisionTypeScheduleActivityTask:
		decision.Attributes = &apiv1.Decision_ScheduleActivityTaskDecisionAttributes{
			ScheduleActivityTaskDecisionAttributes: FromScheduleActivityTaskDecisionAttributes(d.ScheduleActivityTaskDecisionAttributes),
		}
	case types.DecisionTypeRequestCancelActivityTask:
		decision.Attributes = &apiv1.Decision_RequestCancelActivityTaskDecisionAttributes{
			RequestCancelActivityTaskDecisionAttributes: FromRequestCancelActivityTaskDecisionAttributes(d.RequestCancelActivityTaskDecisionAttributes),
		}
	case types.DecisionTypeStartTimer:
		decision.Attributes = &apiv1.Decision_StartTimerDecisionAttributes{
			StartTimerDecisionAttributes: FromStartTimerDecisionAttributes(d.StartTimerDecisionAttributes),
		}
	case types.DecisionTypeCompleteWorkflowExecution:
		decision.Attributes = &apiv1.Decision_CompleteWorkflowExecutionDecisionAttributes{
			CompleteWorkflowExecutionDecisionAttributes: FromCompleteWorkflowExecutionDecisionAttributes(d.CompleteWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeFailWorkflowExecution:
		decision.Attributes = &apiv1.Decision_FailWorkflowExecutionDecisionAttributes{
			FailWorkflowExecutionDecisionAttributes: FromFailWorkflowExecutionDecisionAttributes(d.FailWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeCancelTimer:
		decision.Attributes = &apiv1.Decision_CancelTimerDecisionAttributes{
			CancelTimerDecisionAttributes: FromCancelTimerDecisionAttributes(d.CancelTimerDecisionAttributes),
		}
	case types.DecisionTypeCancelWorkflowExecution:
		decision.Attributes = &apiv1.Decision_CancelWorkflowExecutionDecisionAttributes{
			CancelWorkflowExecutionDecisionAttributes: FromCancelWorkflowExecutionDecisionAttributes(d.CancelWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeRequestCancelExternalWorkflowExecution:
		decision.Attributes = &apiv1.Decision_RequestCancelExternalWorkflowExecutionDecisionAttributes{
			RequestCancelExternalWorkflowExecutionDecisionAttributes: FromRequestCancelExternalWorkflowExecutionDecisionAttributes(d.RequestCancelExternalWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeRecordMarker:
		decision.Attributes = &apiv1.Decision_RecordMarkerDecisionAttributes{
			RecordMarkerDecisionAttributes: FromRecordMarkerDecisionAttributes(d.RecordMarkerDecisionAttributes),
		}
	case types.DecisionTypeContinueAsNewWorkflowExecution:
		decision.Attributes = &apiv1.Decision_ContinueAsNewWorkflowExecutionDecisionAttributes{
			ContinueAsNewWorkflowExecutionDecisionAttributes: FromContinueAsNewWorkflowExecutionDecisionAttributes(d.ContinueAsNewWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeStartChildWorkflowExecution:
		decision.Attributes = &apiv1.Decision_StartChildWorkflowExecutionDecisionAttributes{
			StartChildWorkflowExecutionDecisionAttributes: FromStartChildWorkflowExecutionDecisionAttributes(d.StartChildWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeSignalExternalWorkflowExecution:
		decision.Attributes = &apiv1.Decision_SignalExternalWorkflowExecutionDecisionAttributes{
			SignalExternalWorkflowExecutionDecisionAttributes: FromSignalExternalWorkflowExecutionDecisionAttributes(d.SignalExternalWorkflowExecutionDecisionAttributes),
		}
	case types.DecisionTypeUpsertWorkflowSearchAttributes:
		decision.Attributes = &apiv1.Decision_UpsertWorkflowSearchAttributesDecisionAttributes{
			UpsertWorkflowSearchAttributesDecisionAttributes: FromUpsertWorkflowSearchAttributesDecisionAttributes(d.UpsertWorkflowSearchAttributesDecisionAttributes),
		}
	}
	return &decision
}

func ToDecision(d *apiv1.Decision) *types.Decision {
	if d == nil {
		return nil
	}
	decision := types.Decision{}
	switch attr := d.Attributes.(type) {
	case *apiv1.Decision_ScheduleActivityTaskDecisionAttributes:
		decision.DecisionType = types.DecisionTypeScheduleActivityTask.Ptr()
		decision.ScheduleActivityTaskDecisionAttributes = ToScheduleActivityTaskDecisionAttributes(attr.ScheduleActivityTaskDecisionAttributes)
	case *apiv1.Decision_StartTimerDecisionAttributes:
		decision.DecisionType = types.DecisionTypeStartTimer.Ptr()
		decision.StartTimerDecisionAttributes = ToStartTimerDecisionAttributes(attr.StartTimerDecisionAttributes)
	case *apiv1.Decision_CompleteWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeCompleteWorkflowExecution.Ptr()
		decision.CompleteWorkflowExecutionDecisionAttributes = ToCompleteWorkflowExecutionDecisionAttributes(attr.CompleteWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_FailWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeFailWorkflowExecution.Ptr()
		decision.FailWorkflowExecutionDecisionAttributes = ToFailWorkflowExecutionDecisionAttributes(attr.FailWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_RequestCancelActivityTaskDecisionAttributes:
		decision.DecisionType = types.DecisionTypeRequestCancelActivityTask.Ptr()
		decision.RequestCancelActivityTaskDecisionAttributes = ToRequestCancelActivityTaskDecisionAttributes(attr.RequestCancelActivityTaskDecisionAttributes)
	case *apiv1.Decision_CancelTimerDecisionAttributes:
		decision.DecisionType = types.DecisionTypeCancelTimer.Ptr()
		decision.CancelTimerDecisionAttributes = ToCancelTimerDecisionAttributes(attr.CancelTimerDecisionAttributes)
	case *apiv1.Decision_CancelWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeCancelWorkflowExecution.Ptr()
		decision.CancelWorkflowExecutionDecisionAttributes = ToCancelWorkflowExecutionDecisionAttributes(attr.CancelWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_RequestCancelExternalWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeRequestCancelExternalWorkflowExecution.Ptr()
		decision.RequestCancelExternalWorkflowExecutionDecisionAttributes = ToRequestCancelExternalWorkflowExecutionDecisionAttributes(attr.RequestCancelExternalWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_RecordMarkerDecisionAttributes:
		decision.DecisionType = types.DecisionTypeRecordMarker.Ptr()
		decision.RecordMarkerDecisionAttributes = ToRecordMarkerDecisionAttributes(attr.RecordMarkerDecisionAttributes)
	case *apiv1.Decision_ContinueAsNewWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeContinueAsNewWorkflowExecution.Ptr()
		decision.ContinueAsNewWorkflowExecutionDecisionAttributes = ToContinueAsNewWorkflowExecutionDecisionAttributes(attr.ContinueAsNewWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_StartChildWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeStartChildWorkflowExecution.Ptr()
		decision.StartChildWorkflowExecutionDecisionAttributes = ToStartChildWorkflowExecutionDecisionAttributes(attr.StartChildWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_SignalExternalWorkflowExecutionDecisionAttributes:
		decision.DecisionType = types.DecisionTypeSignalExternalWorkflowExecution.Ptr()
		decision.SignalExternalWorkflowExecutionDecisionAttributes = ToSignalExternalWorkflowExecutionDecisionAttributes(attr.SignalExternalWorkflowExecutionDecisionAttributes)
	case *apiv1.Decision_UpsertWorkflowSearchAttributesDecisionAttributes:
		decision.DecisionType = types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr()
		decision.UpsertWorkflowSearchAttributesDecisionAttributes = ToUpsertWorkflowSearchAttributesDecisionAttributes(attr.UpsertWorkflowSearchAttributesDecisionAttributes)
	}
	return &decision
}

func FromListClosedWorkflowExecutionsRequest(r *types.ListClosedWorkflowExecutionsRequest) *apiv1.ListClosedWorkflowExecutionsRequest {
	if r == nil {
		return nil
	}
	request := apiv1.ListClosedWorkflowExecutionsRequest{
		Domain:          r.Domain,
		PageSize:        r.MaximumPageSize,
		NextPageToken:   r.NextPageToken,
		StartTimeFilter: FromStartTimeFilter(r.StartTimeFilter),
	}
	if r.ExecutionFilter != nil {
		request.Filters = &apiv1.ListClosedWorkflowExecutionsRequest_ExecutionFilter{
			ExecutionFilter: FromWorkflowExecutionFilter(r.ExecutionFilter),
		}
	}
	if r.TypeFilter != nil {
		request.Filters = &apiv1.ListClosedWorkflowExecutionsRequest_TypeFilter{
			TypeFilter: FromWorkflowTypeFilter(r.TypeFilter),
		}
	}
	if r.StatusFilter != nil {
		request.Filters = &apiv1.ListClosedWorkflowExecutionsRequest_StatusFilter{
			StatusFilter: FromStatusFilter(r.StatusFilter),
		}
	}

	return &request
}

func ToListClosedWorkflowExecutionsRequest(r *apiv1.ListClosedWorkflowExecutionsRequest) *types.ListClosedWorkflowExecutionsRequest {
	if r == nil {
		return nil
	}
	request := types.ListClosedWorkflowExecutionsRequest{
		Domain:          r.Domain,
		MaximumPageSize: r.PageSize,
		NextPageToken:   r.NextPageToken,
		StartTimeFilter: ToStartTimeFilter(r.StartTimeFilter),
	}
	switch filters := r.Filters.(type) {
	case *apiv1.ListClosedWorkflowExecutionsRequest_ExecutionFilter:
		request.ExecutionFilter = ToWorkflowExecutionFilter(filters.ExecutionFilter)
	case *apiv1.ListClosedWorkflowExecutionsRequest_TypeFilter:
		request.TypeFilter = ToWorkflowTypeFilter(filters.TypeFilter)
	case *apiv1.ListClosedWorkflowExecutionsRequest_StatusFilter:
		request.StatusFilter = ToStatusFilter(filters.StatusFilter)
	}

	return &request
}

func FromListOpenWorkflowExecutionsRequest(r *types.ListOpenWorkflowExecutionsRequest) *apiv1.ListOpenWorkflowExecutionsRequest {
	if r == nil {
		return nil
	}
	request := apiv1.ListOpenWorkflowExecutionsRequest{
		Domain:          r.Domain,
		PageSize:        r.MaximumPageSize,
		NextPageToken:   r.NextPageToken,
		StartTimeFilter: FromStartTimeFilter(r.StartTimeFilter),
	}
	if r.ExecutionFilter != nil {
		request.Filters = &apiv1.ListOpenWorkflowExecutionsRequest_ExecutionFilter{
			ExecutionFilter: FromWorkflowExecutionFilter(r.ExecutionFilter),
		}
	}
	if r.TypeFilter != nil {
		request.Filters = &apiv1.ListOpenWorkflowExecutionsRequest_TypeFilter{
			TypeFilter: FromWorkflowTypeFilter(r.TypeFilter),
		}
	}

	return &request
}

func ToListOpenWorkflowExecutionsRequest(r *apiv1.ListOpenWorkflowExecutionsRequest) *types.ListOpenWorkflowExecutionsRequest {
	if r == nil {
		return nil
	}
	request := types.ListOpenWorkflowExecutionsRequest{
		Domain:          r.Domain,
		MaximumPageSize: r.PageSize,
		NextPageToken:   r.NextPageToken,
		StartTimeFilter: ToStartTimeFilter(r.StartTimeFilter),
	}
	switch filters := r.Filters.(type) {
	case *apiv1.ListOpenWorkflowExecutionsRequest_ExecutionFilter:
		request.ExecutionFilter = ToWorkflowExecutionFilter(filters.ExecutionFilter)
	case *apiv1.ListOpenWorkflowExecutionsRequest_TypeFilter:
		request.TypeFilter = ToWorkflowTypeFilter(filters.TypeFilter)
	}
	return &request
}

func FromResetStickyTaskListResponse(t *types.ResetStickyTaskListResponse) *apiv1.ResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ResetStickyTaskListResponse{}
}

func ToResetStickyTaskListResponse(t *apiv1.ResetStickyTaskListResponse) *types.ResetStickyTaskListResponse {
	if t == nil {
		return nil
	}
	return &types.ResetStickyTaskListResponse{}
}

func FromAPITaskListPartitionConfig(t *types.TaskListPartitionConfig) *apiv1.TaskListPartitionConfig {
	if t == nil {
		return nil
	}
	return &apiv1.TaskListPartitionConfig{
		Version:            t.Version,
		NumReadPartitions:  int32(len(t.ReadPartitions)),
		NumWritePartitions: int32(len(t.WritePartitions)),
		ReadPartitions:     FromAPITaskListPartitionsMap(t.ReadPartitions),
		WritePartitions:    FromAPITaskListPartitionsMap(t.WritePartitions),
	}
}

func ToAPITaskListPartitionConfig(t *apiv1.TaskListPartitionConfig) *types.TaskListPartitionConfig {
	if t == nil {
		return nil
	}
	return &types.TaskListPartitionConfig{
		Version:         t.Version,
		ReadPartitions:  ToAPITaskListPartitionsMap(t.NumReadPartitions, t.ReadPartitions),
		WritePartitions: ToAPITaskListPartitionsMap(t.NumWritePartitions, t.WritePartitions),
	}
}

func FromAPITaskListPartition(t *types.TaskListPartition) *apiv1.TaskListPartition {
	if t == nil {
		return nil
	}
	return &apiv1.TaskListPartition{
		IsolationGroups: t.IsolationGroups,
	}
}

func ToAPITaskListPartition(t *apiv1.TaskListPartition) *types.TaskListPartition {
	if t == nil {
		return nil
	}
	return &types.TaskListPartition{
		IsolationGroups: t.IsolationGroups,
	}
}

func FromAPITaskListPartitionsMap(m map[int]*types.TaskListPartition) map[int32]*apiv1.TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int32]*apiv1.TaskListPartition, len(m))
	for id, p := range m {
		result[int32(id)] = FromAPITaskListPartition(p)
	}
	return result
}

func ToAPITaskListPartitionsMap(numPartitions int32, m map[int32]*apiv1.TaskListPartition) map[int]*types.TaskListPartition {
	if m == nil && numPartitions == 0 {
		return nil
	}
	result := make(map[int]*types.TaskListPartition, len(m))
	if numPartitions != int32(len(m)) {
		for i := int32(0); i < numPartitions; i++ {
			result[int(i)] = &types.TaskListPartition{}
		}
	} else {
		for id, p := range m {
			result[int(id)] = ToAPITaskListPartition(p)
		}
	}
	return result
}

func FromAutoConfigHint(t *types.AutoConfigHint) *apiv1.AutoConfigHint {
	if t == nil {
		return nil
	}
	return &apiv1.AutoConfigHint{
		PollerWaitTimeInMs: t.PollerWaitTimeInMs,
		EnableAutoConfig:   t.EnableAutoConfig,
	}
}

func ToAutoConfigHint(t *apiv1.AutoConfigHint) *types.AutoConfigHint {
	if t == nil {
		return nil
	}
	return &types.AutoConfigHint{
		PollerWaitTimeInMs: t.PollerWaitTimeInMs,
		EnableAutoConfig:   t.EnableAutoConfig,
	}
}
