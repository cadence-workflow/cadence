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
	"unsafe"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
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

// For responses that require metrics for payload size ByteSize() uint64 should be defined.

func (r *GetReplicationTasksResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	size := uint64(int(unsafe.Sizeof(*r)) + len(r.NextPageToken))
	for _, v := range r.Tasks {
		size += v.ByteSize()
	}

	return size
}

func (r *ReplicationTaskInfo) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.DomainID) + len(r.WorkflowID) + len(r.RunID) + len(r.BranchToken) + len(r.NewRunBranchToken))
}

func (r *GetTimerIndexTasksResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	size := uint64(int(unsafe.Sizeof(*r)) + len(r.NextPageToken))
	for _, v := range r.Timers {
		size += v.ByteSize()
	}

	return size
}

func (r *TimerTaskInfo) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.DomainID) + len(r.WorkflowID) + len(r.RunID))
}

func (r *GetTasksResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	size := uint64(unsafe.Sizeof(*r))
	for _, v := range r.Tasks {
		size += v.ByteSize()
	}

	return size
}

func (r *TaskInfo) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.DomainID) + len(r.WorkflowID) + len(r.RunID) + estimateStringMapSize(r.PartitionConfig))
}

func (r *ListDomainsResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	size := uint64(int(unsafe.Sizeof(*r)) + len(r.NextPageToken))
	for _, v := range r.Domains {
		size += v.ByteSize()
	}

	return size
}

func (r *GetDomainResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(unsafe.Sizeof(*r)) + r.Info.ByteSize() + r.Config.ByteSize() + r.ReplicationConfig.ByteSize()
}

func (i *DomainInfo) ByteSize() uint64 {
	if i == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*i)) + len(i.ID) + len(i.Name) + len(i.Description) + len(i.OwnerEmail) + estimateStringMapSize(i.Data))
}

func (c *DomainConfig) ByteSize() uint64 {
	if c == nil {
		return 0
	}

	size := int(unsafe.Sizeof(*c)) + len(c.HistoryArchivalURI) + len(c.VisibilityArchivalURI)

	asyncWorkflowConfigSize := int(unsafe.Sizeof(c.AsyncWorkflowConfig)) + len(c.AsyncWorkflowConfig.PredefinedQueueName) + len(c.AsyncWorkflowConfig.QueueType)
	if c.AsyncWorkflowConfig.QueueConfig != nil {
		size += len(c.AsyncWorkflowConfig.QueueConfig.Data)
	}

	binariesSize := 0
	for key, value := range c.BadBinaries.Binaries {
		binariesSize += len(key)
		if value != nil {
			binariesSize += len(value.Reason) + len(value.Operator)
		}
	}

	isolationGroupsSize := 0
	for key, value := range c.IsolationGroups {
		binariesSize += len(key) + len(value.Name)
	}

	return uint64(size + asyncWorkflowConfigSize + binariesSize + isolationGroupsSize)
}

func (c *DomainReplicationConfig) ByteSize() uint64 {
	if c == nil {
		return 0
	}

	total := len(c.ActiveClusterName)
	for _, v := range c.Clusters {
		if v == nil {
			continue
		}
		total += len(v.ClusterName)
	}
	return uint64(total)
}

func estimateStringMapSize(m map[string]string) int {
	size := 0
	for key, value := range m {
		size += len(key) + len(value)
	}
	return size
}

func (r *ReadRawHistoryBranchResponse) Size2() uint64 {
	if r == nil {
		return 0
	}

	total := uint64(int(unsafe.Sizeof(*r)) + len(r.NextPageToken))
	for _, v := range r.HistoryEventBlobs {
		total += v.ByteSize()
	}
	return total
}

func (d *DataBlob) ByteSize() uint64 {
	if d == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*d)) + len(d.Data))
}

func (r *ListCurrentExecutionsResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	total := uint64(int(unsafe.Sizeof(*r)) + len(r.PageToken))
	for _, v := range r.Executions {
		total += v.ByteSize()
	}

	return total
}

func (r *CurrentWorkflowExecution) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.DomainID) + len(r.WorkflowID) + len(r.RunID) + len(r.CurrentRunID))
}

func (r *GetTransferTasksResponse) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	total := uint64(int(unsafe.Sizeof(*r)) + len(r.NextPageToken))
	for _, v := range r.Tasks {
		total += v.ByteSize()
	}

	return total
}

func (r *TransferTaskInfo) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.DomainID) + len(r.WorkflowID) + len(r.RunID) +
		len(r.TargetDomainID) + len(r.TargetWorkflowID) + len(r.TargetRunID) + len(r.TaskList))
}

func (r QueueMessageList) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	total := uint64(0)
	for _, v := range r {
		total += v.ByteSize()
	}

	return total
}

func (r *QueueMessage) ByteSize() uint64 {
	if r == nil {
		return 0
	}

	return uint64(int(unsafe.Sizeof(*r)) + len(r.Payload))
}

func (r GetAllHistoryTreeBranchesResponse) ByteSize() uint64 {
	total := uint64(int(unsafe.Sizeof(r)) + len(r.NextPageToken))
	for _, v := range r.Branches {
		total += v.ByteSize()
	}

	return total
}

func (r HistoryBranchDetail) ByteSize() uint64 {
	return uint64(int(unsafe.Sizeof(r)) + len(r.TreeID) + len(r.BranchID) + len(r.Info))
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
