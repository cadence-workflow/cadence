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
	// Test data for schedule spec
	scheduleTime1 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)     // March 1, 2026
	scheduleTime4 = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC) // December 31, 2026

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
)
