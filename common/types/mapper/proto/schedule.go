// Copyright (c) 2026 Uber Technologies, Inc.
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

package proto

import (
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"

	"github.com/uber/cadence/common/types"
)

// --- Enum mappers ---

func FromScheduleOverlapPolicy(p types.ScheduleOverlapPolicy) apiv1.ScheduleOverlapPolicy {
	switch p {
	case types.ScheduleOverlapPolicySkipNew:
		return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_SKIP_NEW
	case types.ScheduleOverlapPolicyBuffer:
		return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_BUFFER
	case types.ScheduleOverlapPolicyConcurrent:
		return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_CONCURRENT
	case types.ScheduleOverlapPolicyCancelPrevious:
		return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_CANCEL_PREVIOUS
	case types.ScheduleOverlapPolicyTerminatePrevious:
		return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_TERMINATE_PREVIOUS
	}
	return apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_INVALID
}

func ToScheduleOverlapPolicy(p apiv1.ScheduleOverlapPolicy) types.ScheduleOverlapPolicy {
	switch p {
	case apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_SKIP_NEW:
		return types.ScheduleOverlapPolicySkipNew
	case apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_BUFFER:
		return types.ScheduleOverlapPolicyBuffer
	case apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_CONCURRENT:
		return types.ScheduleOverlapPolicyConcurrent
	case apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_CANCEL_PREVIOUS:
		return types.ScheduleOverlapPolicyCancelPrevious
	case apiv1.ScheduleOverlapPolicy_SCHEDULE_OVERLAP_POLICY_TERMINATE_PREVIOUS:
		return types.ScheduleOverlapPolicyTerminatePrevious
	}
	return types.ScheduleOverlapPolicyInvalid
}

func FromScheduleCatchUpPolicy(p types.ScheduleCatchUpPolicy) apiv1.ScheduleCatchUpPolicy {
	switch p {
	case types.ScheduleCatchUpPolicySkip:
		return apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_SKIP
	case types.ScheduleCatchUpPolicyOne:
		return apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_ONE
	case types.ScheduleCatchUpPolicyAll:
		return apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_ALL
	}
	return apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_INVALID
}

func ToScheduleCatchUpPolicy(p apiv1.ScheduleCatchUpPolicy) types.ScheduleCatchUpPolicy {
	switch p {
	case apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_SKIP:
		return types.ScheduleCatchUpPolicySkip
	case apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_ONE:
		return types.ScheduleCatchUpPolicyOne
	case apiv1.ScheduleCatchUpPolicy_SCHEDULE_CATCH_UP_POLICY_ALL:
		return types.ScheduleCatchUpPolicyAll
	}
	return types.ScheduleCatchUpPolicyInvalid
}

// --- Core type mappers ---

func FromScheduleSpec(t *types.ScheduleSpec) *apiv1.ScheduleSpec {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleSpec{
		CronExpression: t.CronExpression,
		StartTime:      timeToTimestamp(&t.StartTime),
		EndTime:        timeToTimestamp(&t.EndTime),
		Jitter:         durationToDurationProto(t.Jitter),
	}
}

func ToScheduleSpec(t *apiv1.ScheduleSpec) *types.ScheduleSpec {
	if t == nil {
		return nil
	}
	return &types.ScheduleSpec{
		CronExpression: t.CronExpression,
		StartTime:      timestampToTimeVal(t.StartTime),
		EndTime:        timestampToTimeVal(t.EndTime),
		Jitter:         durationProtoToDuration(t.Jitter),
	}
}

func FromStartWorkflowAction(t *types.StartWorkflowAction) *apiv1.ScheduleAction_StartWorkflowAction {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleAction_StartWorkflowAction{
		WorkflowType:                 FromWorkflowType(t.WorkflowType),
		TaskList:                     FromTaskList(t.TaskList),
		Input:                        FromPayload(t.Input),
		WorkflowIdPrefix:             t.WorkflowIDPrefix,
		ExecutionStartToCloseTimeout: secondsToDuration(t.ExecutionStartToCloseTimeoutSeconds),
		TaskStartToCloseTimeout:      secondsToDuration(t.TaskStartToCloseTimeoutSeconds),
		RetryPolicy:                  FromRetryPolicy(t.RetryPolicy),
		Memo:                         FromMemo(t.Memo),
		SearchAttributes:             FromSearchAttributes(t.SearchAttributes),
	}
}

func ToStartWorkflowAction(t *apiv1.ScheduleAction_StartWorkflowAction) *types.StartWorkflowAction {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowAction{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               ToPayload(t.Input),
		WorkflowIDPrefix:                    t.WorkflowIdPrefix,
		ExecutionStartToCloseTimeoutSeconds: durationToSeconds(t.ExecutionStartToCloseTimeout),
		TaskStartToCloseTimeoutSeconds:      durationToSeconds(t.TaskStartToCloseTimeout),
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
	}
}

func FromScheduleAction(t *types.ScheduleAction) *apiv1.ScheduleAction {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleAction{
		StartWorkflow: FromStartWorkflowAction(t.StartWorkflow),
	}
}

func ToScheduleAction(t *apiv1.ScheduleAction) *types.ScheduleAction {
	if t == nil {
		return nil
	}
	return &types.ScheduleAction{
		StartWorkflow: ToStartWorkflowAction(t.StartWorkflow),
	}
}

func FromSchedulePolicies(t *types.SchedulePolicies) *apiv1.SchedulePolicies {
	if t == nil {
		return nil
	}
	return &apiv1.SchedulePolicies{
		OverlapPolicy:    FromScheduleOverlapPolicy(t.OverlapPolicy),
		CatchUpPolicy:    FromScheduleCatchUpPolicy(t.CatchUpPolicy),
		CatchUpWindow:    durationToDurationProto(t.CatchUpWindow),
		PauseOnFailure:   t.PauseOnFailure,
		BufferLimit:      t.BufferLimit,
		ConcurrencyLimit: t.ConcurrencyLimit,
	}
}

func ToSchedulePolicies(t *apiv1.SchedulePolicies) *types.SchedulePolicies {
	if t == nil {
		return nil
	}
	return &types.SchedulePolicies{
		OverlapPolicy:    ToScheduleOverlapPolicy(t.OverlapPolicy),
		CatchUpPolicy:    ToScheduleCatchUpPolicy(t.CatchUpPolicy),
		CatchUpWindow:    durationProtoToDuration(t.CatchUpWindow),
		PauseOnFailure:   t.PauseOnFailure,
		BufferLimit:      t.BufferLimit,
		ConcurrencyLimit: t.ConcurrencyLimit,
	}
}
