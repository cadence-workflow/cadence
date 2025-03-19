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
	"unsafe"
)

// This file defines method for persistence requests/responses that affects metered persistence wrapper.

// For responses that require metrics for empty response Len() int should be defined.

func (r *GetReplicationTasksResponse) Len() int {
	return len(r.Tasks)
}

func (r *GetTimerIndexTasksResponse) Len() int {
	return len(r.Timers)
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

func (r *GetTransferTasksResponse) Len() int {
	return len(r.Tasks)
}

func (r QueueMessageList) Len() int {
	return len(r)
}

func (r GetAllHistoryTreeBranchesResponse) Len() int {
	return len(r.Branches)
}

// For responses that require metrics for payload size EstimatePayloadSizeInBytes() int should be defined.

func (r *GetReplicationTasksResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*r)) + len(r.NextPageToken)
	for _, v := range r.Tasks {
		if v == nil {
			continue
		}

		size += int(unsafe.Sizeof(*v)) + len(v.DomainID) + len(v.WorkflowID) + len(v.RunID) + len(v.BranchToken) + len(v.NewRunBranchToken)
	}

	return size
}

func (r *GetTimerIndexTasksResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*r)) + len(r.NextPageToken)
	for _, v := range r.Timers {
		if v == nil {
			continue
		}

		size += int(unsafe.Sizeof(*v)) + len(v.DomainID) + len(v.WorkflowID) + len(v.RunID)
	}

	return size
}

func (r *GetTasksResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*r))
	for _, v := range r.Tasks {
		if v == nil {
			continue
		}

		size += int(unsafe.Sizeof(*v)) + len(v.DomainID) + len(v.WorkflowID) + len(v.RunID) + estimateStringMapSize(v.PartitionConfig)
	}

	return size
}

func (r *ListDomainsResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*r)) + len(r.NextPageToken)

	for _, v := range r.Domains {
		if v == nil {
			continue
		}

		size += int(unsafe.Sizeof(*v)) + estimateDomainInfoSize(v.Info) + estimateDomainConfigSize(v.Config) + estimateDomainReplicationConfigSize(v.ReplicationConfig)
	}

	return size
}

func estimateDomainInfoSize(info *DomainInfo) int {
	if info == nil {
		return 0
	}

	return int(unsafe.Sizeof(*info)) + len(info.ID) + len(info.Name) + len(info.Description) + len(info.OwnerEmail) + estimateStringMapSize(info.Data)
}

func estimateDomainConfigSize(info *DomainConfig) int {
	if info == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*info)) + len(info.HistoryArchivalURI) + len(info.VisibilityArchivalURI)

	asyncWorkflowConfigSize := int(unsafe.Sizeof(info.AsyncWorkflowConfig)) + len(info.AsyncWorkflowConfig.PredefinedQueueName) + len(info.AsyncWorkflowConfig.QueueType)
	if info.AsyncWorkflowConfig.QueueConfig != nil {
		size += len(info.AsyncWorkflowConfig.QueueConfig.Data)
	}

	binariesSize := 0
	for key, value := range info.BadBinaries.Binaries {
		binariesSize += len(key)
		if value != nil {
			binariesSize += len(value.Reason) + len(value.Operator)
		}
	}

	isolationGroupsSize := 0
	for key, value := range info.IsolationGroups {
		binariesSize += len(key) + len(value.Name)
	}

	return size + asyncWorkflowConfigSize + binariesSize + isolationGroupsSize
}

func estimateDomainReplicationConfigSize(info *DomainReplicationConfig) int {
	if info == nil {
		return 0
	}

	total := len(info.ActiveClusterName)
	for _, v := range info.Clusters {
		if v == nil {
			continue
		}
		total += len(v.ClusterName)
	}
	return total
}

func estimateStringMapSize(m map[string]string) int {
	size := 0
	for key, value := range m {
		size += len(key) + len(value)
	}
	return size
}

func (r *ReadRawHistoryBranchResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	total := int(unsafe.Sizeof(*r)) + len(r.NextPageToken)
	for _, v := range r.HistoryEventBlobs {
		if v == nil {
			continue
		}
		total += len(v.Data)
	}
	return total
}

func (r *ListCurrentExecutionsResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	total := int(unsafe.Sizeof(*r)) + len(r.PageToken)
	for _, v := range r.Executions {
		if v == nil {
			continue
		}
		total += int(unsafe.Sizeof(*v)) + len(v.DomainID) + len(v.WorkflowID) + len(v.RunID) + len(v.CurrentRunID)
	}

	return total
}

func (r *GetTransferTasksResponse) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	total := int(unsafe.Sizeof(*r)) + len(r.NextPageToken)
	for _, v := range r.Tasks {
		if v == nil {
			continue
		}
		total += int(unsafe.Sizeof(*v)) + len(v.DomainID) + len(v.WorkflowID) + len(v.RunID) +
			len(v.TargetDomainID) + len(v.TargetWorkflowID) + len(v.TargetRunID) + len(v.TaskList)
	}

	return total
}

func (r QueueMessageList) EstimatePayloadSizeInBytes() int {
	if r == nil {
		return 0
	}

	total := 0
	for _, v := range r {
		if v == nil {
			continue
		}
		total += int(unsafe.Sizeof(*v)) + len(v.Payload)
	}

	return total
}

func (r GetAllHistoryTreeBranchesResponse) EstimatePayloadSizeInBytes() int {
	total := int(unsafe.Sizeof(r)) + len(r.NextPageToken)
	for _, v := range r.Branches {
		total += int(unsafe.Sizeof(v)) + len(v.TreeID) + len(v.BranchID) + len(v.Info)
	}

	return total
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
