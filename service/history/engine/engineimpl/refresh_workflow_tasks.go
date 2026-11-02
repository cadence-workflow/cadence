// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

// RefreshWorkflowTasks creates the tasks of the run again. Scopes limit the refresh to some groups of tasks. When
// scopes is empty, all tasks are created.
func (e *historyEngineImpl) RefreshWorkflowTasks(
	ctx context.Context,
	domainUUID string,
	workflowExecution types.WorkflowExecution,
	scopes []*types.RefreshWorkflowTasksScope,
) (retError error) {
	// the scopes are checked before the run is loaded, so a scope that is not valid never causes a full refresh
	if err := validateRefreshWorkflowTasksScopes(scopes); err != nil {
		return err
	}

	domainEntry, err := e.shard.GetDomainCache().GetDomainByID(domainUUID)
	if err != nil {
		return err
	}
	domainID := domainEntry.GetInfo().ID

	wfContext, release, err := e.executionCache.GetOrCreateWorkflowExecution(ctx, domainID, workflowExecution)
	if err != nil {
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		return err
	}

	if len(scopes) == 0 {
		err = e.refreshAllTasks(ctx, mutableState)
	} else {
		err = e.refreshScopedTasks(ctx, mutableState, scopes)
	}
	if err != nil {
		return err
	}

	err = wfContext.UpdateWorkflowExecutionTasks(ctx, e.shard.GetTimeSource().Now())
	if err != nil {
		return err
	}
	return nil
}

// refreshAllTasks creates all tasks of the run again
func (e *historyEngineImpl) refreshAllTasks(ctx context.Context, mutableState execution.MutableState) error {
	mutableStateTaskRefresher := execution.NewMutableStateTaskRefresher(
		e.shard.GetConfig(),
		e.shard.GetClusterMetadata(),
		e.shard.GetDomainCache(),
		e.shard.GetEventsCache(),
		e.shard.GetShardID(),
		e.logger,
	)
	return mutableStateTaskRefresher.RefreshTasks(ctx, mutableState.GetExecutionInfo().StartTimestamp, mutableState)
}

// refreshScopedTasks creates only the tasks of the scopes again
func (e *historyEngineImpl) refreshScopedTasks(ctx context.Context, mutableState execution.MutableState, scopes []*types.RefreshWorkflowTasksScope) error {
	for _, scope := range scopes {
		switch scope.GetScopeType() {
		case types.RefreshWorkflowTasksScopeTypeRecordChildExecutionCompleted:
			if err := e.refreshRecordChildExecutionCompletedTask(ctx, mutableState); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateRefreshWorkflowTasksScopes rejects a scope without a known type. The proto mapper maps an unknown type to nil,
// so an unknown type and a type that is not set get the same error.
func validateRefreshWorkflowTasksScopes(scopes []*types.RefreshWorkflowTasksScope) error {
	errScopeType := &types.BadRequestError{Message: "Refresh scope type is not set or is unknown."}
	for _, scope := range scopes {
		if scope == nil || scope.ScopeType == nil {
			return errScopeType
		}
		switch *scope.ScopeType {
		case types.RefreshWorkflowTasksScopeTypeRecordChildExecutionCompleted:
		default:
			return errScopeType
		}
	}
	return nil
}

// refreshRecordChildExecutionCompletedTask creates only the task that sends the completion of a closed child workflow
// to its parent. Only the active cluster runs this task, so the run must be active on this cluster.
func (e *historyEngineImpl) refreshRecordChildExecutionCompletedTask(ctx context.Context, mutableState execution.MutableState) error {
	if mutableState.IsWorkflowExecutionRunning() {
		return &types.BadRequestError{Message: "Workflow execution is running, so it sends its completion when it closes."}
	}
	if !mutableState.HasParentExecution() {
		return &types.BadRequestError{Message: "Workflow execution does not have a parent."}
	}
	executionInfo := mutableState.GetExecutionInfo()
	if _, err := e.getActiveDomainByWorkflow(ctx, executionInfo.DomainID, executionInfo.WorkflowID, executionInfo.RunID); err != nil {
		return err
	}
	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return err
	}
	mutableState.AddTransferTasks(&persistence.RecordChildExecutionCompletedTask{
		WorkflowIdentifier: persistence.WorkflowIdentifier{
			DomainID:   executionInfo.DomainID,
			WorkflowID: executionInfo.WorkflowID,
			RunID:      executionInfo.RunID,
		},
		TaskData: persistence.TaskData{
			// TaskID and VisibilityTimestamp are set by shard context. The executor runs the task only when its version
			// is the last write version of the run.
			Version: lastWriteVersion,
		},
		TargetDomainID:   executionInfo.ParentDomainID,
		TargetWorkflowID: executionInfo.ParentWorkflowID,
		TargetRunID:      executionInfo.ParentRunID,
		TaskList:         executionInfo.TaskList,
	})
	return nil
}
