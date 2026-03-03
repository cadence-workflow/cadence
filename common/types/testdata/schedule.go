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

package testdata

import (
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

var (
	scheduleTime1 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	scheduleTime2 = time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)
	scheduleTime3 = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	scheduleTime4 = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	scheduleTime5 = time.Date(2026, 1, 15, 14, 0, 0, 0, time.UTC)

	ScheduleSpec = types.ScheduleSpec{
		CronExpression: "*/5 * * * *",
		StartTime:      scheduleTime1,
		EndTime:        scheduleTime4,
		Jitter:         30 * time.Second,
	}

	ScheduleStartWorkflowAction = types.StartWorkflowAction{
		WorkflowType:                        &WorkflowType,
		TaskList:                            &TaskList,
		Input:                               Payload1,
		WorkflowIDPrefix:                    "sched-wf-",
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		RetryPolicy:                         &RetryPolicy,
		Memo:                                &Memo,
		SearchAttributes:                    &SearchAttributes,
	}

	ScheduleAction = types.ScheduleAction{
		StartWorkflow: &ScheduleStartWorkflowAction,
	}

	SchedulePolicies = types.SchedulePolicies{
		OverlapPolicy:    types.ScheduleOverlapPolicyBuffer,
		CatchUpPolicy:    types.ScheduleCatchUpPolicyAll,
		CatchUpWindow:    time.Hour,
		PauseOnFailure:   true,
		BufferLimit:      5,
		ConcurrencyLimit: 2,
	}

	SchedulePauseInfo = types.SchedulePauseInfo{
		Reason:   "maintenance window",
		PausedAt: scheduleTime2,
		PausedBy: "sample_admin@uber.com",
	}

	ScheduleState = types.ScheduleState{
		Paused:    true,
		PauseInfo: &SchedulePauseInfo,
	}

	ScheduleBackfillInfo = types.BackfillInfo{
		BackfillID:    "backfill-001",
		StartTime:     scheduleTime1,
		EndTime:       scheduleTime3,
		RunsCompleted: 15,
		RunsTotal:     30,
	}

	ScheduleBackfillInfo2 = types.BackfillInfo{
		BackfillID:    "backfill-002",
		StartTime:     scheduleTime3,
		EndTime:       scheduleTime4,
		RunsCompleted: 0,
		RunsTotal:     10,
	}

	ScheduleInfo = types.ScheduleInfo{
		LastRunTime:      scheduleTime2,
		NextRunTime:      scheduleTime3,
		TotalRuns:        42,
		CreateTime:       scheduleTime5,
		LastUpdateTime:   scheduleTime2,
		OngoingBackfills: []*types.BackfillInfo{&ScheduleBackfillInfo, &ScheduleBackfillInfo2},
	}

	ScheduleListEntry = types.ScheduleListEntry{
		ScheduleID:     "my-schedule-id",
		WorkflowType:   &WorkflowType,
		State:          &ScheduleState,
		CronExpression: "*/5 * * * *",
	}

	CreateScheduleRequest = types.CreateScheduleRequest{
		Domain:           DomainName,
		ScheduleID:       "my-schedule-id",
		Spec:             &ScheduleSpec,
		Action:           &ScheduleAction,
		Policies:         &SchedulePolicies,
		Memo:             &Memo,
		SearchAttributes: &SearchAttributes,
	}

	CreateScheduleResponse = types.CreateScheduleResponse{}

	DescribeScheduleRequest = types.DescribeScheduleRequest{
		Domain:     DomainName,
		ScheduleID: "my-schedule-id",
	}

	DescribeScheduleResponse = types.DescribeScheduleResponse{
		Spec:             &ScheduleSpec,
		Action:           &ScheduleAction,
		Policies:         &SchedulePolicies,
		State:            &ScheduleState,
		Info:             &ScheduleInfo,
		Memo:             &Memo,
		SearchAttributes: &SearchAttributes,
	}

	UpdateScheduleRequest = types.UpdateScheduleRequest{
		Domain:           DomainName,
		ScheduleID:       "my-schedule-id",
		Spec:             &ScheduleSpec,
		Action:           &ScheduleAction,
		Policies:         &SchedulePolicies,
		SearchAttributes: &SearchAttributes,
	}

	UpdateScheduleResponse = types.UpdateScheduleResponse{}

	DeleteScheduleRequest = types.DeleteScheduleRequest{
		Domain:     DomainName,
		ScheduleID: "my-schedule-id",
	}

	DeleteScheduleResponse = types.DeleteScheduleResponse{}

	PauseScheduleRequest = types.PauseScheduleRequest{
		Domain:     DomainName,
		ScheduleID: "my-schedule-id",
		Reason:     "maintenance window",
	}

	PauseScheduleResponse = types.PauseScheduleResponse{}

	UnpauseScheduleRequest = types.UnpauseScheduleRequest{
		Domain:        DomainName,
		ScheduleID:    "my-schedule-id",
		Reason:        "maintenance complete",
		CatchUpPolicy: types.ScheduleCatchUpPolicyOne,
	}

	UnpauseScheduleResponse = types.UnpauseScheduleResponse{}

	ListSchedulesRequest = types.ListSchedulesRequest{
		Domain:        DomainName,
		PageSize:      10,
		NextPageToken: []byte("next-page-token"),
	}

	ListSchedulesResponse = types.ListSchedulesResponse{
		Schedules:     []*types.ScheduleListEntry{&ScheduleListEntry},
		NextPageToken: []byte("next-page-token-2"),
	}

	BackfillScheduleRequest = types.BackfillScheduleRequest{
		Domain:        DomainName,
		ScheduleID:    "my-schedule-id",
		StartTime:     scheduleTime1,
		EndTime:       scheduleTime3,
		OverlapPolicy: types.ScheduleOverlapPolicyConcurrent,
		BackfillID:    "backfill-003",
	}

	BackfillScheduleResponse = types.BackfillScheduleResponse{}
)
