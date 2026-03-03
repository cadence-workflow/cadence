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

// --- State/info type mappers ---

func FromSchedulePauseInfo(t *types.SchedulePauseInfo) *apiv1.SchedulePauseInfo {
	if t == nil {
		return nil
	}
	return &apiv1.SchedulePauseInfo{
		Reason:   t.Reason,
		PausedAt: timeToTimestamp(&t.PausedAt),
		PausedBy: t.PausedBy,
	}
}

func ToSchedulePauseInfo(t *apiv1.SchedulePauseInfo) *types.SchedulePauseInfo {
	if t == nil {
		return nil
	}
	return &types.SchedulePauseInfo{
		Reason:   t.Reason,
		PausedAt: timestampToTimeVal(t.PausedAt),
		PausedBy: t.PausedBy,
	}
}

func FromScheduleState(t *types.ScheduleState) *apiv1.ScheduleState {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleState{
		Paused:    t.Paused,
		PauseInfo: FromSchedulePauseInfo(t.PauseInfo),
	}
}

func ToScheduleState(t *apiv1.ScheduleState) *types.ScheduleState {
	if t == nil {
		return nil
	}
	return &types.ScheduleState{
		Paused:    t.Paused,
		PauseInfo: ToSchedulePauseInfo(t.PauseInfo),
	}
}

func FromBackfillInfo(t *types.BackfillInfo) *apiv1.BackfillInfo {
	if t == nil {
		return nil
	}
	return &apiv1.BackfillInfo{
		BackfillId:    t.BackfillID,
		StartTime:     timeToTimestamp(&t.StartTime),
		EndTime:       timeToTimestamp(&t.EndTime),
		RunsCompleted: t.RunsCompleted,
		RunsTotal:     t.RunsTotal,
	}
}

func ToBackfillInfo(t *apiv1.BackfillInfo) *types.BackfillInfo {
	if t == nil {
		return nil
	}
	return &types.BackfillInfo{
		BackfillID:    t.BackfillId,
		StartTime:     timestampToTimeVal(t.StartTime),
		EndTime:       timestampToTimeVal(t.EndTime),
		RunsCompleted: t.RunsCompleted,
		RunsTotal:     t.RunsTotal,
	}
}

func FromBackfillInfoArray(t []*types.BackfillInfo) []*apiv1.BackfillInfo {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.BackfillInfo, len(t))
	for i := range t {
		v[i] = FromBackfillInfo(t[i])
	}
	return v
}

func ToBackfillInfoArray(t []*apiv1.BackfillInfo) []*types.BackfillInfo {
	if t == nil {
		return nil
	}
	v := make([]*types.BackfillInfo, len(t))
	for i := range t {
		v[i] = ToBackfillInfo(t[i])
	}
	return v
}

func FromScheduleInfo(t *types.ScheduleInfo) *apiv1.ScheduleInfo {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleInfo{
		LastRunTime:      timeToTimestamp(&t.LastRunTime),
		NextRunTime:      timeToTimestamp(&t.NextRunTime),
		TotalRuns:        t.TotalRuns,
		CreateTime:       timeToTimestamp(&t.CreateTime),
		LastUpdateTime:   timeToTimestamp(&t.LastUpdateTime),
		OngoingBackfills: FromBackfillInfoArray(t.OngoingBackfills),
	}
}

func ToScheduleInfo(t *apiv1.ScheduleInfo) *types.ScheduleInfo {
	if t == nil {
		return nil
	}
	return &types.ScheduleInfo{
		LastRunTime:      timestampToTimeVal(t.LastRunTime),
		NextRunTime:      timestampToTimeVal(t.NextRunTime),
		TotalRuns:        t.TotalRuns,
		CreateTime:       timestampToTimeVal(t.CreateTime),
		LastUpdateTime:   timestampToTimeVal(t.LastUpdateTime),
		OngoingBackfills: ToBackfillInfoArray(t.OngoingBackfills),
	}
}

func FromScheduleListEntry(t *types.ScheduleListEntry) *apiv1.ScheduleListEntry {
	if t == nil {
		return nil
	}
	return &apiv1.ScheduleListEntry{
		ScheduleId:     t.ScheduleID,
		WorkflowType:   FromWorkflowType(t.WorkflowType),
		State:          FromScheduleState(t.State),
		CronExpression: t.CronExpression,
	}
}

func ToScheduleListEntry(t *apiv1.ScheduleListEntry) *types.ScheduleListEntry {
	if t == nil {
		return nil
	}
	return &types.ScheduleListEntry{
		ScheduleID:     t.ScheduleId,
		WorkflowType:   ToWorkflowType(t.WorkflowType),
		State:          ToScheduleState(t.State),
		CronExpression: t.CronExpression,
	}
}

func FromScheduleListEntryArray(t []*types.ScheduleListEntry) []*apiv1.ScheduleListEntry {
	if t == nil {
		return nil
	}
	v := make([]*apiv1.ScheduleListEntry, len(t))
	for i := range t {
		v[i] = FromScheduleListEntry(t[i])
	}
	return v
}

func ToScheduleListEntryArray(t []*apiv1.ScheduleListEntry) []*types.ScheduleListEntry {
	if t == nil {
		return nil
	}
	v := make([]*types.ScheduleListEntry, len(t))
	for i := range t {
		v[i] = ToScheduleListEntry(t[i])
	}
	return v
}

// --- CRUD request/response mappers ---

func FromCreateScheduleRequest(t *types.CreateScheduleRequest) *apiv1.CreateScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.CreateScheduleRequest{
		Domain:           t.Domain,
		ScheduleId:       t.ScheduleID,
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		Memo:             FromMemo(t.Memo),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToCreateScheduleRequest(t *apiv1.CreateScheduleRequest) *types.CreateScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.CreateScheduleRequest{
		Domain:           t.Domain,
		ScheduleID:       t.ScheduleId,
		Spec:             ToScheduleSpec(t.Spec),
		Action:           ToScheduleAction(t.Action),
		Policies:         ToSchedulePolicies(t.Policies),
		Memo:             ToMemo(t.Memo),
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromCreateScheduleResponse(t *types.CreateScheduleResponse) *apiv1.CreateScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.CreateScheduleResponse{}
}

func ToCreateScheduleResponse(t *apiv1.CreateScheduleResponse) *types.CreateScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.CreateScheduleResponse{}
}

func FromDescribeScheduleRequest(t *types.DescribeScheduleRequest) *apiv1.DescribeScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeScheduleRequest{
		Domain:     t.Domain,
		ScheduleId: t.ScheduleID,
	}
}

func ToDescribeScheduleRequest(t *apiv1.DescribeScheduleRequest) *types.DescribeScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeScheduleRequest{
		Domain:     t.Domain,
		ScheduleID: t.ScheduleId,
	}
}

func FromDescribeScheduleResponse(t *types.DescribeScheduleResponse) *apiv1.DescribeScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DescribeScheduleResponse{
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		State:            FromScheduleState(t.State),
		Info:             FromScheduleInfo(t.Info),
		Memo:             FromMemo(t.Memo),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToDescribeScheduleResponse(t *apiv1.DescribeScheduleResponse) *types.DescribeScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.DescribeScheduleResponse{
		Spec:             ToScheduleSpec(t.Spec),
		Action:           ToScheduleAction(t.Action),
		Policies:         ToSchedulePolicies(t.Policies),
		State:            ToScheduleState(t.State),
		Info:             ToScheduleInfo(t.Info),
		Memo:             ToMemo(t.Memo),
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromUpdateScheduleRequest(t *types.UpdateScheduleRequest) *apiv1.UpdateScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.UpdateScheduleRequest{
		Domain:           t.Domain,
		ScheduleId:       t.ScheduleID,
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToUpdateScheduleRequest(t *apiv1.UpdateScheduleRequest) *types.UpdateScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateScheduleRequest{
		Domain:           t.Domain,
		ScheduleID:       t.ScheduleId,
		Spec:             ToScheduleSpec(t.Spec),
		Action:           ToScheduleAction(t.Action),
		Policies:         ToSchedulePolicies(t.Policies),
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromUpdateScheduleResponse(t *types.UpdateScheduleResponse) *apiv1.UpdateScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.UpdateScheduleResponse{}
}

func ToUpdateScheduleResponse(t *apiv1.UpdateScheduleResponse) *types.UpdateScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.UpdateScheduleResponse{}
}

func FromDeleteScheduleRequest(t *types.DeleteScheduleRequest) *apiv1.DeleteScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.DeleteScheduleRequest{
		Domain:     t.Domain,
		ScheduleId: t.ScheduleID,
	}
}

func ToDeleteScheduleRequest(t *apiv1.DeleteScheduleRequest) *types.DeleteScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.DeleteScheduleRequest{
		Domain:     t.Domain,
		ScheduleID: t.ScheduleId,
	}
}

func FromDeleteScheduleResponse(t *types.DeleteScheduleResponse) *apiv1.DeleteScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.DeleteScheduleResponse{}
}

func ToDeleteScheduleResponse(t *apiv1.DeleteScheduleResponse) *types.DeleteScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.DeleteScheduleResponse{}
}

// --- Action request/response mappers ---

func FromPauseScheduleRequest(t *types.PauseScheduleRequest) *apiv1.PauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.PauseScheduleRequest{
		Domain:     t.Domain,
		ScheduleId: t.ScheduleID,
		Reason:     t.Reason,
	}
}

func ToPauseScheduleRequest(t *apiv1.PauseScheduleRequest) *types.PauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.PauseScheduleRequest{
		Domain:     t.Domain,
		ScheduleID: t.ScheduleId,
		Reason:     t.Reason,
	}
}

func FromPauseScheduleResponse(t *types.PauseScheduleResponse) *apiv1.PauseScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.PauseScheduleResponse{}
}

func ToPauseScheduleResponse(t *apiv1.PauseScheduleResponse) *types.PauseScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.PauseScheduleResponse{}
}

func FromUnpauseScheduleRequest(t *types.UnpauseScheduleRequest) *apiv1.UnpauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.UnpauseScheduleRequest{
		Domain:        t.Domain,
		ScheduleId:    t.ScheduleID,
		Reason:        t.Reason,
		CatchUpPolicy: FromScheduleCatchUpPolicy(t.CatchUpPolicy),
	}
}

func ToUnpauseScheduleRequest(t *apiv1.UnpauseScheduleRequest) *types.UnpauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.UnpauseScheduleRequest{
		Domain:        t.Domain,
		ScheduleID:    t.ScheduleId,
		Reason:        t.Reason,
		CatchUpPolicy: ToScheduleCatchUpPolicy(t.CatchUpPolicy),
	}
}

func FromUnpauseScheduleResponse(t *types.UnpauseScheduleResponse) *apiv1.UnpauseScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.UnpauseScheduleResponse{}
}

func ToUnpauseScheduleResponse(t *apiv1.UnpauseScheduleResponse) *types.UnpauseScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.UnpauseScheduleResponse{}
}

func FromListSchedulesRequest(t *types.ListSchedulesRequest) *apiv1.ListSchedulesRequest {
	if t == nil {
		return nil
	}
	return &apiv1.ListSchedulesRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func ToListSchedulesRequest(t *apiv1.ListSchedulesRequest) *types.ListSchedulesRequest {
	if t == nil {
		return nil
	}
	return &types.ListSchedulesRequest{
		Domain:        t.Domain,
		PageSize:      t.PageSize,
		NextPageToken: t.NextPageToken,
	}
}

func FromListSchedulesResponse(t *types.ListSchedulesResponse) *apiv1.ListSchedulesResponse {
	if t == nil {
		return nil
	}
	return &apiv1.ListSchedulesResponse{
		Schedules:     FromScheduleListEntryArray(t.Schedules),
		NextPageToken: t.NextPageToken,
	}
}

func ToListSchedulesResponse(t *apiv1.ListSchedulesResponse) *types.ListSchedulesResponse {
	if t == nil {
		return nil
	}
	return &types.ListSchedulesResponse{
		Schedules:     ToScheduleListEntryArray(t.Schedules),
		NextPageToken: t.NextPageToken,
	}
}

func FromBackfillScheduleRequest(t *types.BackfillScheduleRequest) *apiv1.BackfillScheduleRequest {
	if t == nil {
		return nil
	}
	return &apiv1.BackfillScheduleRequest{
		Domain:        t.Domain,
		ScheduleId:    t.ScheduleID,
		StartTime:     timeToTimestamp(&t.StartTime),
		EndTime:       timeToTimestamp(&t.EndTime),
		OverlapPolicy: FromScheduleOverlapPolicy(t.OverlapPolicy),
		BackfillId:    t.BackfillID,
	}
}

func ToBackfillScheduleRequest(t *apiv1.BackfillScheduleRequest) *types.BackfillScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.BackfillScheduleRequest{
		Domain:        t.Domain,
		ScheduleID:    t.ScheduleId,
		StartTime:     timestampToTimeVal(t.StartTime),
		EndTime:       timestampToTimeVal(t.EndTime),
		OverlapPolicy: ToScheduleOverlapPolicy(t.OverlapPolicy),
		BackfillID:    t.BackfillId,
	}
}

func FromBackfillScheduleResponse(t *types.BackfillScheduleResponse) *apiv1.BackfillScheduleResponse {
	if t == nil {
		return nil
	}
	return &apiv1.BackfillScheduleResponse{}
}

func ToBackfillScheduleResponse(t *apiv1.BackfillScheduleResponse) *types.BackfillScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.BackfillScheduleResponse{}
}
