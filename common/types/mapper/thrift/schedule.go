// Copyright (c) 2017-2024 Uber Technologies, Inc.
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
	"github.com/uber/cadence/common/types"
)

// --- Enums ---

func FromScheduleOverlapPolicy(t types.ScheduleOverlapPolicy) *shared.ScheduleOverlapPolicy {
	v := shared.ScheduleOverlapPolicy(t)
	return &v
}

func ToScheduleOverlapPolicy(t *shared.ScheduleOverlapPolicy) types.ScheduleOverlapPolicy {
	if t == nil {
		return types.ScheduleOverlapPolicyInvalid
	}
	return types.ScheduleOverlapPolicy(*t)
}

func FromScheduleCatchUpPolicy(t types.ScheduleCatchUpPolicy) *shared.ScheduleCatchUpPolicy {
	v := shared.ScheduleCatchUpPolicy(t)
	return &v
}

func ToScheduleCatchUpPolicy(t *shared.ScheduleCatchUpPolicy) types.ScheduleCatchUpPolicy {
	if t == nil {
		return types.ScheduleCatchUpPolicyInvalid
	}
	return types.ScheduleCatchUpPolicy(*t)
}

// --- Core Types ---

func FromScheduleSpec(t *types.ScheduleSpec) *shared.ScheduleSpec {
	if t == nil {
		return nil
	}
	return &shared.ScheduleSpec{
		CronExpression:  common.StringPtr(t.CronExpression),
		StartTimeNano:   timeValToNano(t.StartTime),
		EndTimeNano:     timeValToNano(t.EndTime),
		JitterInSeconds: durationToSeconds(t.Jitter),
	}
}

func ToScheduleSpec(t *shared.ScheduleSpec) *types.ScheduleSpec {
	if t == nil {
		return nil
	}
	return &types.ScheduleSpec{
		CronExpression: t.GetCronExpression(),
		StartTime:      nanoToTimeVal(t.StartTimeNano),
		EndTime:        nanoToTimeVal(t.EndTimeNano),
		Jitter:         secondsToDuration(t.JitterInSeconds),
	}
}

func FromStartWorkflowAction(t *types.StartWorkflowAction) *shared.ScheduleStartWorkflowAction {
	if t == nil {
		return nil
	}
	return &shared.ScheduleStartWorkflowAction{
		WorkflowType:                        FromWorkflowType(t.WorkflowType),
		TaskList:                            FromTaskList(t.TaskList),
		Input:                               t.Input,
		WorkflowIdPrefix:                    common.StringPtr(t.WorkflowIDPrefix),
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		RetryPolicy:                         FromRetryPolicy(t.RetryPolicy),
		Memo:                                FromMemo(t.Memo),
		SearchAttributes:                    FromSearchAttributes(t.SearchAttributes),
	}
}

func ToStartWorkflowAction(t *shared.ScheduleStartWorkflowAction) *types.StartWorkflowAction {
	if t == nil {
		return nil
	}
	return &types.StartWorkflowAction{
		WorkflowType:                        ToWorkflowType(t.WorkflowType),
		TaskList:                            ToTaskList(t.TaskList),
		Input:                               t.Input,
		WorkflowIDPrefix:                    t.GetWorkflowIdPrefix(),
		ExecutionStartToCloseTimeoutSeconds: t.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      t.TaskStartToCloseTimeoutSeconds,
		RetryPolicy:                         ToRetryPolicy(t.RetryPolicy),
		Memo:                                ToMemo(t.Memo),
		SearchAttributes:                    ToSearchAttributes(t.SearchAttributes),
	}
}

func FromScheduleAction(t *types.ScheduleAction) *shared.ScheduleAction {
	if t == nil {
		return nil
	}
	return &shared.ScheduleAction{
		StartWorkflow: FromStartWorkflowAction(t.StartWorkflow),
	}
}

func ToScheduleAction(t *shared.ScheduleAction) *types.ScheduleAction {
	if t == nil {
		return nil
	}
	return &types.ScheduleAction{
		StartWorkflow: ToStartWorkflowAction(t.StartWorkflow),
	}
}

func FromSchedulePolicies(t *types.SchedulePolicies) *shared.SchedulePolicies {
	if t == nil {
		return nil
	}
	return &shared.SchedulePolicies{
		OverlapPolicy:          FromScheduleOverlapPolicy(t.OverlapPolicy),
		CatchUpPolicy:          FromScheduleCatchUpPolicy(t.CatchUpPolicy),
		CatchUpWindowInSeconds: durationToSeconds(t.CatchUpWindow),
		PauseOnFailure:         common.BoolPtr(t.PauseOnFailure),
		BufferLimit:            common.Int32Ptr(t.BufferLimit),
		ConcurrencyLimit:       common.Int32Ptr(t.ConcurrencyLimit),
	}
}

func ToSchedulePolicies(t *shared.SchedulePolicies) *types.SchedulePolicies {
	if t == nil {
		return nil
	}
	return &types.SchedulePolicies{
		OverlapPolicy:    ToScheduleOverlapPolicy(t.OverlapPolicy),
		CatchUpPolicy:    ToScheduleCatchUpPolicy(t.CatchUpPolicy),
		CatchUpWindow:    secondsToDuration(t.CatchUpWindowInSeconds),
		PauseOnFailure:   t.GetPauseOnFailure(),
		BufferLimit:      t.GetBufferLimit(),
		ConcurrencyLimit: t.GetConcurrencyLimit(),
	}
}

func FromSchedulePauseInfo(t *types.SchedulePauseInfo) *shared.SchedulePauseInfo {
	if t == nil {
		return nil
	}
	return &shared.SchedulePauseInfo{
		Reason:         common.StringPtr(t.Reason),
		PausedTimeNano: timeValToNano(t.PausedAt),
		PausedBy:       common.StringPtr(t.PausedBy),
	}
}

func ToSchedulePauseInfo(t *shared.SchedulePauseInfo) *types.SchedulePauseInfo {
	if t == nil {
		return nil
	}
	return &types.SchedulePauseInfo{
		Reason:   t.GetReason(),
		PausedAt: nanoToTimeVal(t.PausedTimeNano),
		PausedBy: t.GetPausedBy(),
	}
}

func FromScheduleState(t *types.ScheduleState) *shared.ScheduleState {
	if t == nil {
		return nil
	}
	return &shared.ScheduleState{
		Paused:    common.BoolPtr(t.Paused),
		PauseInfo: FromSchedulePauseInfo(t.PauseInfo),
	}
}

func ToScheduleState(t *shared.ScheduleState) *types.ScheduleState {
	if t == nil {
		return nil
	}
	return &types.ScheduleState{
		Paused:    t.GetPaused(),
		PauseInfo: ToSchedulePauseInfo(t.PauseInfo),
	}
}

func FromBackfillInfo(t *types.BackfillInfo) *shared.BackfillInfo {
	if t == nil {
		return nil
	}
	return &shared.BackfillInfo{
		BackfillId:    common.StringPtr(t.BackfillID),
		StartTimeNano: timeValToNano(t.StartTime),
		EndTimeNano:   timeValToNano(t.EndTime),
		RunsCompleted: common.Int32Ptr(t.RunsCompleted),
		RunsTotal:     common.Int32Ptr(t.RunsTotal),
	}
}

func ToBackfillInfo(t *shared.BackfillInfo) *types.BackfillInfo {
	if t == nil {
		return nil
	}
	return &types.BackfillInfo{
		BackfillID:    t.GetBackfillId(),
		StartTime:     nanoToTimeVal(t.StartTimeNano),
		EndTime:       nanoToTimeVal(t.EndTimeNano),
		RunsCompleted: t.GetRunsCompleted(),
		RunsTotal:     t.GetRunsTotal(),
	}
}

func FromScheduleInfo(t *types.ScheduleInfo) *shared.ScheduleInfo {
	if t == nil {
		return nil
	}
	return &shared.ScheduleInfo{
		LastRunTimeNano:    timeValToNano(t.LastRunTime),
		NextRunTimeNano:    timeValToNano(t.NextRunTime),
		TotalRuns:          common.Int64Ptr(t.TotalRuns),
		CreateTimeNano:     timeValToNano(t.CreateTime),
		LastUpdateTimeNano: timeValToNano(t.LastUpdateTime),
		OngoingBackfills:   fromBackfillInfoSlice(t.OngoingBackfills),
	}
}

func ToScheduleInfo(t *shared.ScheduleInfo) *types.ScheduleInfo {
	if t == nil {
		return nil
	}
	return &types.ScheduleInfo{
		LastRunTime:      nanoToTimeVal(t.LastRunTimeNano),
		NextRunTime:      nanoToTimeVal(t.NextRunTimeNano),
		TotalRuns:        t.GetTotalRuns(),
		CreateTime:       nanoToTimeVal(t.CreateTimeNano),
		LastUpdateTime:   nanoToTimeVal(t.LastUpdateTimeNano),
		OngoingBackfills: toBackfillInfoSlice(t.OngoingBackfills),
	}
}

func FromScheduleListEntry(t *types.ScheduleListEntry) *shared.ScheduleListEntry {
	if t == nil {
		return nil
	}
	return &shared.ScheduleListEntry{
		ScheduleId:     common.StringPtr(t.ScheduleID),
		WorkflowType:   FromWorkflowType(t.WorkflowType),
		State:          FromScheduleState(t.State),
		CronExpression: common.StringPtr(t.CronExpression),
	}
}

func ToScheduleListEntry(t *shared.ScheduleListEntry) *types.ScheduleListEntry {
	if t == nil {
		return nil
	}
	return &types.ScheduleListEntry{
		ScheduleID:     t.GetScheduleId(),
		WorkflowType:   ToWorkflowType(t.WorkflowType),
		State:          ToScheduleState(t.State),
		CronExpression: t.GetCronExpression(),
	}
}

// --- Request/Response Types ---

func FromCreateScheduleRequest(t *types.CreateScheduleRequest) *shared.CreateScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.CreateScheduleRequest{
		Domain:           common.StringPtr(t.Domain),
		ScheduleId:       common.StringPtr(t.ScheduleID),
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		Memo:             FromMemo(t.Memo),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToCreateScheduleRequest(t *shared.CreateScheduleRequest) *types.CreateScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.CreateScheduleRequest{
		Domain:           t.GetDomain(),
		ScheduleID:       t.GetScheduleId(),
		Spec:             ToScheduleSpec(t.Spec),
		Action:           ToScheduleAction(t.Action),
		Policies:         ToSchedulePolicies(t.Policies),
		Memo:             ToMemo(t.Memo),
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromCreateScheduleResponse(t *types.CreateScheduleResponse) *shared.CreateScheduleResponse {
	if t == nil {
		return nil
	}
	return &shared.CreateScheduleResponse{
		ScheduleId: common.StringPtr(t.ScheduleID),
	}
}

func ToCreateScheduleResponse(t *shared.CreateScheduleResponse) *types.CreateScheduleResponse {
	if t == nil {
		return nil
	}
	return &types.CreateScheduleResponse{
		ScheduleID: t.GetScheduleId(),
	}
}

func FromDescribeScheduleRequest(t *types.DescribeScheduleRequest) *shared.DescribeScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.DescribeScheduleRequest{
		Domain:     common.StringPtr(t.Domain),
		ScheduleId: common.StringPtr(t.ScheduleID),
	}
}

func ToDescribeScheduleRequest(t *shared.DescribeScheduleRequest) *types.DescribeScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.DescribeScheduleRequest{
		Domain:     t.GetDomain(),
		ScheduleID: t.GetScheduleId(),
	}
}

func FromDescribeScheduleResponse(t *types.DescribeScheduleResponse) *shared.DescribeScheduleResponse {
	if t == nil {
		return nil
	}
	return &shared.DescribeScheduleResponse{
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		State:            FromScheduleState(t.State),
		Info:             FromScheduleInfo(t.Info),
		Memo:             FromMemo(t.Memo),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToDescribeScheduleResponse(t *shared.DescribeScheduleResponse) *types.DescribeScheduleResponse {
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

func FromUpdateScheduleRequest(t *types.UpdateScheduleRequest) *shared.UpdateScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.UpdateScheduleRequest{
		Domain:           common.StringPtr(t.Domain),
		ScheduleId:       common.StringPtr(t.ScheduleID),
		Spec:             FromScheduleSpec(t.Spec),
		Action:           FromScheduleAction(t.Action),
		Policies:         FromSchedulePolicies(t.Policies),
		SearchAttributes: FromSearchAttributes(t.SearchAttributes),
	}
}

func ToUpdateScheduleRequest(t *shared.UpdateScheduleRequest) *types.UpdateScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.UpdateScheduleRequest{
		Domain:           t.GetDomain(),
		ScheduleID:       t.GetScheduleId(),
		Spec:             ToScheduleSpec(t.Spec),
		Action:           ToScheduleAction(t.Action),
		Policies:         ToSchedulePolicies(t.Policies),
		SearchAttributes: ToSearchAttributes(t.SearchAttributes),
	}
}

func FromUpdateScheduleResponse(_ *types.UpdateScheduleResponse) *shared.UpdateScheduleResponse {
	return &shared.UpdateScheduleResponse{}
}

func ToUpdateScheduleResponse(_ *shared.UpdateScheduleResponse) *types.UpdateScheduleResponse {
	return &types.UpdateScheduleResponse{}
}

func FromDeleteScheduleRequest(t *types.DeleteScheduleRequest) *shared.DeleteScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.DeleteScheduleRequest{
		Domain:     common.StringPtr(t.Domain),
		ScheduleId: common.StringPtr(t.ScheduleID),
	}
}

func ToDeleteScheduleRequest(t *shared.DeleteScheduleRequest) *types.DeleteScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.DeleteScheduleRequest{
		Domain:     t.GetDomain(),
		ScheduleID: t.GetScheduleId(),
	}
}

func FromDeleteScheduleResponse(_ *types.DeleteScheduleResponse) *shared.DeleteScheduleResponse {
	return &shared.DeleteScheduleResponse{}
}

func ToDeleteScheduleResponse(_ *shared.DeleteScheduleResponse) *types.DeleteScheduleResponse {
	return &types.DeleteScheduleResponse{}
}

func FromPauseScheduleRequest(t *types.PauseScheduleRequest) *shared.PauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.PauseScheduleRequest{
		Domain:     common.StringPtr(t.Domain),
		ScheduleId: common.StringPtr(t.ScheduleID),
		Reason:     common.StringPtr(t.Reason),
		Identity:   common.StringPtr(t.Identity),
	}
}

func ToPauseScheduleRequest(t *shared.PauseScheduleRequest) *types.PauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.PauseScheduleRequest{
		Domain:     t.GetDomain(),
		ScheduleID: t.GetScheduleId(),
		Reason:     t.GetReason(),
		Identity:   t.GetIdentity(),
	}
}

func FromPauseScheduleResponse(_ *types.PauseScheduleResponse) *shared.PauseScheduleResponse {
	return &shared.PauseScheduleResponse{}
}

func ToPauseScheduleResponse(_ *shared.PauseScheduleResponse) *types.PauseScheduleResponse {
	return &types.PauseScheduleResponse{}
}

func FromUnpauseScheduleRequest(t *types.UnpauseScheduleRequest) *shared.UnpauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.UnpauseScheduleRequest{
		Domain:        common.StringPtr(t.Domain),
		ScheduleId:    common.StringPtr(t.ScheduleID),
		Reason:        common.StringPtr(t.Reason),
		CatchUpPolicy: FromScheduleCatchUpPolicy(t.CatchUpPolicy),
	}
}

func ToUnpauseScheduleRequest(t *shared.UnpauseScheduleRequest) *types.UnpauseScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.UnpauseScheduleRequest{
		Domain:        t.GetDomain(),
		ScheduleID:    t.GetScheduleId(),
		Reason:        t.GetReason(),
		CatchUpPolicy: ToScheduleCatchUpPolicy(t.CatchUpPolicy),
	}
}

func FromUnpauseScheduleResponse(_ *types.UnpauseScheduleResponse) *shared.UnpauseScheduleResponse {
	return &shared.UnpauseScheduleResponse{}
}

func ToUnpauseScheduleResponse(_ *shared.UnpauseScheduleResponse) *types.UnpauseScheduleResponse {
	return &types.UnpauseScheduleResponse{}
}

func FromBackfillScheduleRequest(t *types.BackfillScheduleRequest) *shared.BackfillScheduleRequest {
	if t == nil {
		return nil
	}
	return &shared.BackfillScheduleRequest{
		Domain:        common.StringPtr(t.Domain),
		ScheduleId:    common.StringPtr(t.ScheduleID),
		StartTimeNano: timeValToNano(t.StartTime),
		EndTimeNano:   timeValToNano(t.EndTime),
		OverlapPolicy: FromScheduleOverlapPolicy(t.OverlapPolicy),
		BackfillId:    common.StringPtr(t.BackfillID),
	}
}

func ToBackfillScheduleRequest(t *shared.BackfillScheduleRequest) *types.BackfillScheduleRequest {
	if t == nil {
		return nil
	}
	return &types.BackfillScheduleRequest{
		Domain:        t.GetDomain(),
		ScheduleID:    t.GetScheduleId(),
		StartTime:     nanoToTimeVal(t.StartTimeNano),
		EndTime:       nanoToTimeVal(t.EndTimeNano),
		OverlapPolicy: ToScheduleOverlapPolicy(t.OverlapPolicy),
		BackfillID:    t.GetBackfillId(),
	}
}

func FromBackfillScheduleResponse(_ *types.BackfillScheduleResponse) *shared.BackfillScheduleResponse {
	return &shared.BackfillScheduleResponse{}
}

func ToBackfillScheduleResponse(_ *shared.BackfillScheduleResponse) *types.BackfillScheduleResponse {
	return &types.BackfillScheduleResponse{}
}

func FromListSchedulesRequest(t *types.ListSchedulesRequest) *shared.ListSchedulesRequest {
	if t == nil {
		return nil
	}
	return &shared.ListSchedulesRequest{
		Domain:        common.StringPtr(t.Domain),
		PageSize:      common.Int32Ptr(t.PageSize),
		NextPageToken: t.NextPageToken,
	}
}

func ToListSchedulesRequest(t *shared.ListSchedulesRequest) *types.ListSchedulesRequest {
	if t == nil {
		return nil
	}
	return &types.ListSchedulesRequest{
		Domain:        t.GetDomain(),
		PageSize:      t.GetPageSize(),
		NextPageToken: t.NextPageToken,
	}
}

func FromListSchedulesResponse(t *types.ListSchedulesResponse) *shared.ListSchedulesResponse {
	if t == nil {
		return nil
	}
	return &shared.ListSchedulesResponse{
		Schedules:     fromScheduleListEntrySlice(t.Schedules),
		NextPageToken: t.NextPageToken,
	}
}

func ToListSchedulesResponse(t *shared.ListSchedulesResponse) *types.ListSchedulesResponse {
	if t == nil {
		return nil
	}
	return &types.ListSchedulesResponse{
		Schedules:     toScheduleListEntrySlice(t.Schedules),
		NextPageToken: t.NextPageToken,
	}
}

// --- Slice helpers ---

func fromBackfillInfoSlice(t []*types.BackfillInfo) []*shared.BackfillInfo {
	if t == nil {
		return nil
	}
	result := make([]*shared.BackfillInfo, len(t))
	for i, v := range t {
		result[i] = FromBackfillInfo(v)
	}
	return result
}

func toBackfillInfoSlice(t []*shared.BackfillInfo) []*types.BackfillInfo {
	if t == nil {
		return nil
	}
	result := make([]*types.BackfillInfo, len(t))
	for i, v := range t {
		result[i] = ToBackfillInfo(v)
	}
	return result
}

func fromScheduleListEntrySlice(t []*types.ScheduleListEntry) []*shared.ScheduleListEntry {
	if t == nil {
		return nil
	}
	result := make([]*shared.ScheduleListEntry, len(t))
	for i, v := range t {
		result[i] = FromScheduleListEntry(v)
	}
	return result
}

func toScheduleListEntrySlice(t []*shared.ScheduleListEntry) []*types.ScheduleListEntry {
	if t == nil {
		return nil
	}
	result := make([]*types.ScheduleListEntry, len(t))
	for i, v := range t {
		result[i] = ToScheduleListEntry(v)
	}
	return result
}
