// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddActivityTaskRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &AddActivityTaskRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &AddActivityTaskRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetExecution(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want *WorkflowExecution
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty execution",
			req:  &AddActivityTaskRequest{},
			want: nil,
		},
		{
			name: "execution",
			req:  &AddActivityTaskRequest{Execution: &WorkflowExecution{WorkflowID: "test-workflow-id"}},
			want: &WorkflowExecution{WorkflowID: "test-workflow-id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetExecution()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetSourceDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty source domain",
			req:  &AddActivityTaskRequest{},
			want: "",
		},
		{
			name: "source domain",
			req:  &AddActivityTaskRequest{SourceDomainUUID: "test-source-domain"},
			want: "test-source-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetSourceDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &AddActivityTaskRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &AddActivityTaskRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetScheduleID(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want int64
	}{
		{
			name: "nil request",
			req:  nil,
			want: 0,
		},
		{
			name: "empty schedule id",
			req:  &AddActivityTaskRequest{},
			want: 0,
		},
		{
			name: "schedule id",
			req:  &AddActivityTaskRequest{ScheduleID: 123},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetScheduleID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetScheduleToStartTimeoutSeconds(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want int32
	}{
		{
			name: "nil request",
			req:  nil,
			want: 0,
		},
		{
			name: "empty schedule to start timeout",
			req:  &AddActivityTaskRequest{},
			want: 0,
		},
		{
			name: "schedule to start timeout",
			req:  &AddActivityTaskRequest{ScheduleToStartTimeoutSeconds: func() *int32 { i := int32(123); return &i }()},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetScheduleToStartTimeoutSeconds()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetSource(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want TaskSource
	}{
		{
			name: "nil request",
			req:  nil,
			want: TaskSource(0),
		},
		{
			name: "empty source",
			req:  &AddActivityTaskRequest{},
			want: TaskSource(0),
		},
		{
			name: "source",
			req:  &AddActivityTaskRequest{Source: TaskSource(123).Ptr()},
			want: TaskSource(123),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetSource()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetForwardedFrom(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty forwarded from",
			req:  &AddActivityTaskRequest{},
			want: "",
		},
		{
			name: "forwarded from",
			req:  &AddActivityTaskRequest{ForwardedFrom: "test-forwarded-from"},
			want: "test-forwarded-from",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetForwardedFrom()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddActivityTaskRequest_GetPartitionConfig(t *testing.T) {
	tests := []struct {
		name string
		req  *AddActivityTaskRequest
		want map[string]string
	}{
		{
			name: "nil request",
			req:  nil,
			want: func() map[string]string { return nil }(),
		},
		{
			name: "empty partition config",
			req:  &AddActivityTaskRequest{},
			want: func() map[string]string { return nil }(),
		},
		{
			name: "partition config",
			req:  &AddActivityTaskRequest{PartitionConfig: map[string]string{"test-key": "test-value"}},
			want: map[string]string{"test-key": "test-value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPartitionConfig()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &AddDecisionTaskRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &AddDecisionTaskRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetExecution(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want *WorkflowExecution
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty execution",
			req:  &AddDecisionTaskRequest{},
			want: nil,
		},
		{
			name: "execution",
			req:  &AddDecisionTaskRequest{Execution: &WorkflowExecution{WorkflowID: "test-workflow-id"}},
			want: &WorkflowExecution{WorkflowID: "test-workflow-id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetExecution()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &AddDecisionTaskRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &AddDecisionTaskRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetScheduleID(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want int64
	}{
		{
			name: "nil request",
			req:  nil,
			want: 0,
		},
		{
			name: "empty schedule id",
			req:  &AddDecisionTaskRequest{},
			want: 0,
		},
		{
			name: "schedule id",
			req:  &AddDecisionTaskRequest{ScheduleID: 123},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetScheduleID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetScheduleToStartTimeoutSeconds(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want int32
	}{
		{
			name: "nil request",
			req:  nil,
			want: 0,
		},
		{
			name: "empty schedule to start timeout",
			req:  &AddDecisionTaskRequest{},
			want: 0,
		},
		{
			name: "schedule to start timeout",
			req:  &AddDecisionTaskRequest{ScheduleToStartTimeoutSeconds: func() *int32 { i := int32(123); return &i }()},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetScheduleToStartTimeoutSeconds()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetSource(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want TaskSource
	}{
		{
			name: "nil request",
			req:  nil,
			want: TaskSource(0),
		},
		{
			name: "empty source",
			req:  &AddDecisionTaskRequest{},
			want: TaskSource(0),
		},
		{
			name: "source",
			req:  &AddDecisionTaskRequest{Source: TaskSource(123).Ptr()},
			want: TaskSource(123),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetSource()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetForwardedFrom(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty forwarded from",
			req:  &AddDecisionTaskRequest{},
			want: "",
		},
		{
			name: "forwarded from",
			req:  &AddDecisionTaskRequest{ForwardedFrom: "test-forwarded-from"},
			want: "test-forwarded-from",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetForwardedFrom()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddDecisionTaskRequest_GetPartitionConfig(t *testing.T) {
	tests := []struct {
		name string
		req  *AddDecisionTaskRequest
		want map[string]string
	}{
		{
			name: "nil request",
			req:  nil,
			want: func() map[string]string { return nil }(),
		},
		{
			name: "empty partition config",
			req:  &AddDecisionTaskRequest{},
			want: func() map[string]string { return nil }(),
		},
		{
			name: "partition config",
			req:  &AddDecisionTaskRequest{PartitionConfig: map[string]string{"test-key": "test-value"}},
			want: map[string]string{"test-key": "test-value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPartitionConfig()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCancelOutstandingPollRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *CancelOutstandingPollRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &CancelOutstandingPollRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &CancelOutstandingPollRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCancelOutstandingPollRequest_GetTaskListType(t *testing.T) {
	tests := []struct {
		name string
		req  *CancelOutstandingPollRequest
		want int32
	}{
		{
			name: "nil request",
			req:  nil,
			want: 0,
		},
		{
			name: "empty task list type",
			req:  &CancelOutstandingPollRequest{},
			want: 0,
		},
		{
			name: "task list type",
			req:  &CancelOutstandingPollRequest{TaskListType: func() *int32 { var v int32; v = int32(TaskListTypeActivity); return &v }()},
			want: int32(TaskListTypeActivity),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskListType()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCancelOutstandingPollRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *CancelOutstandingPollRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &CancelOutstandingPollRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &CancelOutstandingPollRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCancelOutstandingPollRequest_GetPollerID(t *testing.T) {
	tests := []struct {
		name string
		req  *CancelOutstandingPollRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty poller id",
			req:  &CancelOutstandingPollRequest{},
			want: "",
		},
		{
			name: "poller id",
			req:  &CancelOutstandingPollRequest{PollerID: "test-poller-id"},
			want: "test-poller-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPollerID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingDescribeTaskListRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingDescribeTaskListRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingDescribeTaskListRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingDescribeTaskListRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingDescribeTaskListRequest_GetDescRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingDescribeTaskListRequest
		want *DescribeTaskListRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty desc request",
			req:  &MatchingDescribeTaskListRequest{},
			want: nil,
		},
		{
			name: "desc request",
			req:  &MatchingDescribeTaskListRequest{DescRequest: &DescribeTaskListRequest{Domain: "test-domain"}},
			want: &DescribeTaskListRequest{Domain: "test-domain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDescRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingListTaskListPartitionsRequest_GetDomain(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingListTaskListPartitionsRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingListTaskListPartitionsRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingListTaskListPartitionsRequest{Domain: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomain()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingListTaskListPartitionsRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingListTaskListPartitionsRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &MatchingListTaskListPartitionsRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &MatchingListTaskListPartitionsRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingGetTaskListsByDomainRequest_GetDomain(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingGetTaskListsByDomainRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingGetTaskListsByDomainRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingGetTaskListsByDomainRequest{Domain: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomain()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingPollForActivityTaskRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingPollForActivityTaskRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetPollerID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty poller id",
			req:  &MatchingPollForActivityTaskRequest{},
			want: "",
		},
		{
			name: "poller id",
			req:  &MatchingPollForActivityTaskRequest{PollerID: "test-poller-id"},
			want: "test-poller-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPollerID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetPollRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want *PollForActivityTaskRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty poll request",
			req:  &MatchingPollForActivityTaskRequest{},
			want: nil,
		},
		{
			name: "poll request",
			req:  &MatchingPollForActivityTaskRequest{PollRequest: &PollForActivityTaskRequest{Domain: "test-domain"}},
			want: &PollForActivityTaskRequest{Domain: "test-domain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPollRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &MatchingPollForActivityTaskRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &MatchingPollForActivityTaskRequest{PollRequest: &PollForActivityTaskRequest{TaskList: &TaskList{Name: "test-task-list"}}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetForwardedFrom(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty forwarded from",
			req:  &MatchingPollForActivityTaskRequest{},
			want: "",
		},
		{
			name: "forwarded from",
			req:  &MatchingPollForActivityTaskRequest{ForwardedFrom: "test-forwarded-from"},
			want: "test-forwarded-from",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetForwardedFrom()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForActivityTaskRequest_GetIsolationGroup(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForActivityTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty isolation group",
			req:  &MatchingPollForActivityTaskRequest{},
			want: "",
		},
		{
			name: "isolation group",
			req:  &MatchingPollForActivityTaskRequest{IsolationGroup: "test-isolation-group"},
			want: "test-isolation-group",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetIsolationGroup()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingPollForDecisionTaskRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetPollerID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty poller id",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: "",
		},
		{
			name: "poller id",
			req:  &MatchingPollForDecisionTaskRequest{PollerID: "test-poller-id"},
			want: "test-poller-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPollerID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetPollRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want *PollForDecisionTaskRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty poll request",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: nil,
		},
		{
			name: "poll request",
			req:  &MatchingPollForDecisionTaskRequest{PollRequest: &PollForDecisionTaskRequest{Domain: "test-domain"}},
			want: &PollForDecisionTaskRequest{Domain: "test-domain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetPollRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &MatchingPollForDecisionTaskRequest{PollRequest: &PollForDecisionTaskRequest{TaskList: &TaskList{Name: "test-task-list"}}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetForwardedFrom(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty forwarded from",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: "",
		},
		{
			name: "forwarded from",
			req:  &MatchingPollForDecisionTaskRequest{ForwardedFrom: "test-forwarded-from"},
			want: "test-forwarded-from",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetForwardedFrom()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskRequest_GetIsolationGroup(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingPollForDecisionTaskRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty isolation group",
			req:  &MatchingPollForDecisionTaskRequest{},
			want: "",
		},
		{
			name: "isolation group",
			req:  &MatchingPollForDecisionTaskRequest{IsolationGroup: "test-isolation-group"},
			want: "test-isolation-group",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetIsolationGroup()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetWorkflowExecution(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want *WorkflowExecution
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "empty workflow execution",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: nil,
		},
		{
			name: "workflow execution",
			resp: &MatchingPollForDecisionTaskResponse{WorkflowExecution: &WorkflowExecution{WorkflowID: "test-workflow-id"}},
			want: &WorkflowExecution{WorkflowID: "test-workflow-id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetWorkflowExecution()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetPreviousStartedEventID(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want int64
	}{
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name: "empty previous started event id",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: 0,
		},
		{
			name: "previous started event id",
			resp: &MatchingPollForDecisionTaskResponse{PreviousStartedEventID: func() *int64 { v := int64(123); return &v }()},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetPreviousStartedEventID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetNextEventID(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want int64
	}{
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name: "empty next event id",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: 0,
		},
		{
			name: "next event id",
			resp: &MatchingPollForDecisionTaskResponse{NextEventID: int64(123)},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetNextEventID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetStickyExecutionEnabled(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want bool
	}{
		{
			name: "nil response",
			resp: nil,
			want: false,
		},
		{
			name: "empty sticky execution enabled",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: false,
		},
		{
			name: "sticky execution enabled",
			resp: &MatchingPollForDecisionTaskResponse{StickyExecutionEnabled: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetStickyExecutionEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetBranchToken(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want []byte
	}{
		{
			name: "nil response",
			resp: nil,
			want: nil,
		},
		{
			name: "empty branch token",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: nil,
		},
		{
			name: "branch token",
			resp: &MatchingPollForDecisionTaskResponse{BranchToken: []byte("test-branch-token")},
			want: []byte("test-branch-token"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetBranchToken()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingPollForDecisionTaskResponse_GetTotalHistoryBytes(t *testing.T) {
	tests := []struct {
		name string
		resp *MatchingPollForDecisionTaskResponse
		want int64
	}{
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name: "empty total history bytes",
			resp: &MatchingPollForDecisionTaskResponse{},
			want: 0,
		},
		{
			name: "total history bytes",
			resp: &MatchingPollForDecisionTaskResponse{TotalHistoryBytes: 123},
			want: 123,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetTotalHistoryBytes()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingQueryWorkflowRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingQueryWorkflowRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingQueryWorkflowRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingQueryWorkflowRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingQueryWorkflowRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingQueryWorkflowRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &MatchingQueryWorkflowRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &MatchingQueryWorkflowRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingQueryWorkflowRequest_GetQueryRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingQueryWorkflowRequest
		want *QueryWorkflowRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty query request",
			req:  &MatchingQueryWorkflowRequest{},
			want: nil,
		},
		{
			name: "query request",
			req:  &MatchingQueryWorkflowRequest{QueryRequest: &QueryWorkflowRequest{Domain: "test-domain"}},
			want: &QueryWorkflowRequest{Domain: "test-domain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetQueryRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingQueryWorkflowRequest_GetForwardedFrom(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingQueryWorkflowRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty forwarded from",
			req:  &MatchingQueryWorkflowRequest{},
			want: "",
		},
		{
			name: "forwarded from",
			req:  &MatchingQueryWorkflowRequest{ForwardedFrom: "test-forwarded-from"},
			want: "test-forwarded-from",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetForwardedFrom()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingRespondQueryTaskCompletedRequest_GetDomainUUID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingRespondQueryTaskCompletedRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty domain",
			req:  &MatchingRespondQueryTaskCompletedRequest{},
			want: "",
		},
		{
			name: "domain",
			req:  &MatchingRespondQueryTaskCompletedRequest{DomainUUID: "test-domain"},
			want: "test-domain",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetDomainUUID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingRespondQueryTaskCompletedRequest_GetTaskList(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingRespondQueryTaskCompletedRequest
		want *TaskList
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty task list",
			req:  &MatchingRespondQueryTaskCompletedRequest{},
			want: nil,
		},
		{
			name: "task list",
			req:  &MatchingRespondQueryTaskCompletedRequest{TaskList: &TaskList{Name: "test-task-list"}},
			want: &TaskList{Name: "test-task-list"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskList()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingRespondQueryTaskCompletedRequest_GetTaskID(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingRespondQueryTaskCompletedRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty task id",
			req:  &MatchingRespondQueryTaskCompletedRequest{},
			want: "",
		},
		{
			name: "task id",
			req:  &MatchingRespondQueryTaskCompletedRequest{TaskID: "test-task-id"},
			want: "test-task-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskID()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingRespondQueryTaskCompletedRequest_GetCompletedRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingRespondQueryTaskCompletedRequest
		want *RespondQueryTaskCompletedRequest
	}{
		{
			name: "nil request",
			req:  nil,
			want: nil,
		},
		{
			name: "empty completed request",
			req:  &MatchingRespondQueryTaskCompletedRequest{},
			want: nil,
		},
		{
			name: "completed request",
			req:  &MatchingRespondQueryTaskCompletedRequest{CompletedRequest: &RespondQueryTaskCompletedRequest{ErrorMessage: "test-error-message"}},
			want: &RespondQueryTaskCompletedRequest{ErrorMessage: "test-error-message"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetCompletedRequest()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaskSource_Ptr(t *testing.T) {
	tests := []struct {
		name string
		s    TaskSource
		want *TaskSource
	}{
		{
			name: "nil",
			s:    TaskSource(0),
			want: func() *TaskSource { i := TaskSource(0); return &i }(),
		},
		{
			name: "non-nil",
			s:    TaskSource(123),
			want: func() *TaskSource { i := TaskSource(123); return &i }(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.Ptr()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaskSource_String(t *testing.T) {
	tests := []struct {
		name string
		s    TaskSource
		want string
	}{
		{
			name: "history",
			s:    TaskSource(0),
			want: "HISTORY",
		},
		{
			name: "db-backlog",
			s:    TaskSource(1),
			want: "DB_BACKLOG",
		},
		{
			name: "default",
			s:    TaskSource(123),
			want: "TaskSource(123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTaskSource_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want TaskSource
		err  error
	}{
		{
			name: "history",
			text: "HISTORY",
			want: TaskSource(0),
			err:  nil,
		},
		{
			name: "db_backlog",
			text: "DB_BACKLOG",
			want: TaskSource(1),
			err:  nil,
		},
		{
			name: "number in string",
			text: "123",
			want: TaskSource(123),
			err:  nil,
		},
		{
			name: "error",
			text: "unknown",
			err:  errors.New("unknown enum value \"UNKNOWN\" for \"TaskSource\": strconv.ParseInt: parsing \"UNKNOWN\": invalid syntax"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s TaskSource
			err := s.UnmarshalText([]byte(tt.text))
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, s)
			}
		})
	}
}

func TestTaskSource_MarshalText(t *testing.T) {
	tests := []struct {
		name string
		s    TaskSource
		want string
	}{
		{
			name: "history",
			s:    TaskSource(0),
			want: "HISTORY",
		},
		{
			name: "db-backlog",
			s:    TaskSource(1),
			want: "DB_BACKLOG",
		},
		{
			name: "default",
			s:    TaskSource(123),
			want: "TaskSource(123)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := tt.s.MarshalText()
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(b))
		})
	}
}

func TestMatchingUpdateTaskListPartitionConfigRequest_GetTaskListType(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingUpdateTaskListPartitionConfigRequest
		want TaskListType
	}{
		{
			name: "nil request",
			req:  nil,
			want: TaskListTypeDecision,
		},
		{
			name: "empty request",
			req:  &MatchingUpdateTaskListPartitionConfigRequest{},
			want: TaskListTypeDecision,
		},
		{
			name: "non empty request",
			req:  &MatchingUpdateTaskListPartitionConfigRequest{TaskListType: TaskListTypeActivity.Ptr()},
			want: TaskListTypeActivity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskListType()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchingRefreshTaskListPartitionConfigRequest_GetTaskListType(t *testing.T) {
	tests := []struct {
		name string
		req  *MatchingRefreshTaskListPartitionConfigRequest
		want TaskListType
	}{
		{
			name: "nil request",
			req:  nil,
			want: TaskListTypeDecision,
		},
		{
			name: "empty request",
			req:  &MatchingRefreshTaskListPartitionConfigRequest{},
			want: TaskListTypeDecision,
		},
		{
			name: "non empty request",
			req:  &MatchingRefreshTaskListPartitionConfigRequest{TaskListType: TaskListTypeActivity.Ptr()},
			want: TaskListTypeActivity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetTaskListType()
			assert.Equal(t, tt.want, got)
		})
	}
}
