// Copyright (c) 2019 Uber Technologies, Inc.
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

package tasklist

import (
	"github.com/uber/cadence/common/isolationgroup"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// TODO: review the usage of InternalTask and provide a better abstraction
type (
	// genericTaskInfo contains the info for an activity or decision task
	genericTaskInfo struct {
		*persistence.TaskInfo
		completionFunc func(*persistence.TaskInfo, error)
	}
	// queryTaskInfo contains the info for a query task
	queryTaskInfo struct {
		TaskID  string
		Request *types.MatchingQueryWorkflowRequest
	}
	// startedTaskInfo contains info for any task received from
	// another matching host. This type of task is already marked as started
	startedTaskInfo struct {
		decisionTaskInfo *types.MatchingPollForDecisionTaskResponse
		activityTaskInfo *types.MatchingPollForActivityTaskResponse
	}
	// InternalTask represents an activity, decision, query or started (received from another host).
	// this struct is more like a union and only one of [ query, event, forwarded ] is
	// non-nil for any given task
	InternalTask struct {
		Event                    *genericTaskInfo // non-nil for activity or decision task that's locally generated
		Query                    *queryTaskInfo   // non-nil for a query task that's locally sync matched
		started                  *startedTaskInfo // non-nil for a task received from a parent partition which is already started
		domainName               string
		source                   types.TaskSource
		forwardedFrom            string     // name of the child partition this task is forwarded from (empty if not forwarded)
		isolationGroup           string     // isolation group of this task (empty if it can be polled by workers from any isolation group)
		ResponseC                chan error // non-nil only where there is a caller waiting for response (sync-match)
		BacklogCountHint         int64
		ActivityTaskDispatchInfo *types.ActivityTaskDispatchInfo
		AutoConfigHint           *types.AutoConfigHint // worker auto-scaler hint, which includes enable auto config flag and poller wait time on the matching engine
	}
)

func newInternalTask(
	info *persistence.TaskInfo,
	completionFunc func(*persistence.TaskInfo, error),
	source types.TaskSource,
	forwardedFrom string,
	forSyncMatch bool,
	activityTaskDispatchInfo *types.ActivityTaskDispatchInfo,
	isolationGroup string,
) *InternalTask {
	task := &InternalTask{
		Event: &genericTaskInfo{
			TaskInfo:       info,
			completionFunc: completionFunc,
		},
		source:                   source,
		forwardedFrom:            forwardedFrom,
		isolationGroup:           isolationGroup,
		ActivityTaskDispatchInfo: activityTaskDispatchInfo,
	}
	if forSyncMatch {
		task.ResponseC = make(chan error, 1)
	}

	// Rewrite the partitionConfig to match how we're dispatching it
	// OriginalIsolationGroup is populated here and isn't written to the DB. If it's already
	// present then it's a forwarded task and we should respect it.
	if configIsolationGroup, ok := task.Event.PartitionConfig[isolationgroup.GroupKey]; ok {
		partitionConfig := make(map[string]string, 3)
		if originalIsolationGroup, ok := task.Event.PartitionConfig[isolationgroup.OriginalGroupKey]; ok {
			partitionConfig[isolationgroup.OriginalGroupKey] = originalIsolationGroup
		} else {
			partitionConfig[isolationgroup.OriginalGroupKey] = configIsolationGroup
		}
		partitionConfig[isolationgroup.GroupKey] = isolationGroup
		partitionConfig[isolationgroup.WorkflowIDKey] = task.Event.PartitionConfig[isolationgroup.WorkflowIDKey]
		task.Event.PartitionConfig = partitionConfig
	}
	return task
}

func newInternalQueryTask(
	taskID string,
	request *types.MatchingQueryWorkflowRequest,
) *InternalTask {
	return &InternalTask{
		Query: &queryTaskInfo{
			TaskID:  taskID,
			Request: request,
		},
		forwardedFrom: request.GetForwardedFrom(),
		ResponseC:     make(chan error, 1),
	}
}

func newInternalStartedTask(info *startedTaskInfo) *InternalTask {
	return &InternalTask{started: info}
}

// isQuery returns true if the underlying task is a query task
func (task *InternalTask) IsQuery() bool {
	return task.Query != nil
}

// isStarted is true when this task is already marked as started
func (task *InternalTask) IsStarted() bool {
	return task.started != nil
}

// isForwarded returns true if the underlying task is forwarded by a remote matching host
// forwarded tasks are already marked as started in history
func (task *InternalTask) IsForwarded() bool {
	return task.forwardedFrom != ""
}

func (task *InternalTask) IsSyncMatch() bool {
	return task.ResponseC != nil
}

func (task *InternalTask) Info() persistence.TaskInfo {
	if task == nil || task.Event == nil || task.Event.TaskInfo == nil {
		return persistence.TaskInfo{}
	}

	return *task.Event.TaskInfo
}

func (task *InternalTask) WorkflowExecution() *types.WorkflowExecution {
	switch {
	case task.Event != nil:
		return &types.WorkflowExecution{WorkflowID: task.Event.WorkflowID, RunID: task.Event.RunID}
	case task.Query != nil:
		return task.Query.Request.GetQueryRequest().GetExecution()
	case task.started != nil && task.started.decisionTaskInfo != nil:
		return task.started.decisionTaskInfo.WorkflowExecution
	case task.started != nil && task.started.activityTaskInfo != nil:
		return task.started.activityTaskInfo.WorkflowExecution
	}
	return &types.WorkflowExecution{}
}

// pollForDecisionResponse returns the poll response for a decision task that is
// already marked as started. This method should only be called when isStarted() is true
func (task *InternalTask) PollForDecisionResponse() *types.MatchingPollForDecisionTaskResponse {
	if task.IsStarted() {
		return task.started.decisionTaskInfo
	}
	return nil
}

// pollForActivityResponse returns the poll response for an activity task that is
// already marked as started. This method should only be called when isStarted() is true
func (task *InternalTask) PollForActivityResponse() *types.MatchingPollForActivityTaskResponse {
	if task.IsStarted() {
		return task.started.activityTaskInfo
	}
	return nil
}

// finish marks a task as finished. Should be called after a poller picks up a task
// and marks it as started. If the task is unable to marked as started, then this
// method should be called with a non-nil error argument.
func (task *InternalTask) Finish(err error) {
	switch {
	case task.ResponseC != nil:
		task.ResponseC <- err
	case task.Event.completionFunc != nil:
		task.Event.completionFunc(task.Event.TaskInfo, err)
	}
}
