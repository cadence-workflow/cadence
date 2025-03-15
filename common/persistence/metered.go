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

package persistence

import (
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
)

// This file defines method for persistence requests/responses that affects metered persistence wrapper.

// For responses that require metrics for empty response Len() int should be defined.

func (r *GetReplicationTasksResponse) Len() int {
	return len(r.Tasks)
}

func (r *GetHistoryTasksResponse) Len() int {
	return len(r.Tasks)
}

func (r *GetTasksResponse) Len() int {
	return len(r.Tasks)
}

func (r *ListDomainsResponse) Len() int {
	return len(r.Domains)
}

func (r *ReadHistoryBranchResponse) Len() int {
	return len(r.HistoryEvents)
}

func (r *ListCurrentExecutionsResponse) Len() int {
	return len(r.Executions)
}

func (r QueueMessageList) Len() int {
	return len(r)
}

func (r GetAllHistoryTreeBranchesResponse) Len() int {
	return len(r.Branches)
}

// If MetricTags() []metrics.Tag is defined, then metrics will be emitted for the request.

func (r ReadHistoryBranchRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r AppendHistoryNodesRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r DeleteHistoryBranchRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r ForkHistoryBranchRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r GetHistoryTreeRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r CompleteTaskRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r CompleteTasksLessThanRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r CreateTasksRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r DeleteTaskListRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r GetTasksRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r LeaseTaskListRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r UpdateTaskListRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r GetTaskListSizeRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

// For Execution manager there are extra rules.
// If GetDomainName() string is defined, then the request will have extra log and shard metrics.
// GetExtraLogTags() []tag.Tag is defined, then the request will have extra log tags.

func (r *CreateWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r *IsWorkflowExecutionExistsRequest) GetDomainName() string {
	return r.DomainName
}

func (r *PutReplicationTaskToDLQRequest) MetricTags() []metrics.Tag {
	return []metrics.Tag{metrics.DomainTag(r.DomainName)}
}

func (r *CreateWorkflowExecutionRequest) GetExtraLogTags() []tag.Tag {
	if r == nil || r.NewWorkflowSnapshot.ExecutionInfo == nil {
		return nil
	}
	return []tag.Tag{tag.WorkflowID(r.NewWorkflowSnapshot.ExecutionInfo.WorkflowID)}
}

func (r *UpdateWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r *UpdateWorkflowExecutionRequest) GetExtraLogTags() []tag.Tag {
	if r == nil || r.UpdateWorkflowMutation.ExecutionInfo == nil {
		return nil
	}
	return []tag.Tag{tag.WorkflowID(r.UpdateWorkflowMutation.ExecutionInfo.WorkflowID)}
}

func (r GetWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r GetWorkflowExecutionRequest) GetExtraLogTags() []tag.Tag {
	return []tag.Tag{tag.WorkflowID(r.Execution.WorkflowID)}
}

func (r ConflictResolveWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r DeleteWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r DeleteWorkflowExecutionRequest) GetExtraLogTags() []tag.Tag {
	return []tag.Tag{tag.WorkflowID(r.WorkflowID)}
}

func (r DeleteCurrentWorkflowExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r DeleteCurrentWorkflowExecutionRequest) GetExtraLogTags() []tag.Tag {
	return []tag.Tag{tag.WorkflowID(r.WorkflowID)}
}

func (r GetCurrentExecutionRequest) GetDomainName() string {
	return r.DomainName
}

func (r GetCurrentExecutionRequest) GetExtraLogTags() []tag.Tag {
	return []tag.Tag{tag.WorkflowID(r.WorkflowID)}
}
