// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package shard

import (
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
)

// isOperationPossiblySuccessfulError returns true for errors where a persistence write
// may have succeeded despite the error being returned (e.g. timeout, unknown network error).
// Returns false for errors that definitively indicate the write did not occur.
//
// DuplicateRequestError returns false here because a duplicate write means no new tasks
// were created, so no notification is needed. This differs from the execution layer where
// the write "possibly succeeded" from the caller's perspective.
func isOperationPossiblySuccessfulError(err error) bool {
	if _, ok := err.(*persistence.DuplicateRequestError); ok {
		return false
	}
	return hcommon.IsOperationPossiblySuccessfulError(err)
}

// isNotifyTaskNeeded determines whether task notification should be sent based on the error returned from a persistence operation.
// Returns isNotify=true when the persistence write may have landed (success or ambiguous error), false for definitive failures.
// Returns persistenceError=true when the write outcome is uncertain (ambiguous error), so processors can handle duplicates.
func isNotifyTaskNeeded(err error) (notify, persistenceError bool) {
	if err == nil {
		return true, false
	}
	if isOperationPossiblySuccessfulError(err) {
		return true, true
	}
	return false, false
}

// notifyTasksFromCreateWorkflowExecution sends task notifications for a CreateWorkflowExecution operation.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasksFromCreateWorkflowExecution(
	request *persistence.CreateWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		s.notifyTasksFromSnapshot(&request.NewWorkflowSnapshot, persistenceError)
		return
	}
	s.logger.Info("notify tasks dropped due to persistence error", tag.Error(err))
}

// notifyTasksFromUpdateWorkflowExecution sends task notifications for an UpdateWorkflowExecution operation.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasksFromUpdateWorkflowExecution(
	request *persistence.UpdateWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		s.notifyTasksFromMutation(&request.UpdateWorkflowMutation, persistenceError)
		s.notifyTasksFromSnapshot(request.NewWorkflowSnapshot, persistenceError)
		return
	}
	s.logger.Info("notify tasks dropped due to persistence error", tag.Error(err))
}

// notifyTasksFromConflictResolveWorkflowExecution sends task notifications for a ConflictResolveWorkflowExecution operation.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasksFromConflictResolveWorkflowExecution(
	request *persistence.ConflictResolveWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		s.notifyTasksFromSnapshot(&request.ResetWorkflowSnapshot, persistenceError)
		s.notifyTasksFromSnapshot(request.NewWorkflowSnapshot, persistenceError)
		s.notifyTasksFromMutation(request.CurrentWorkflowMutation, persistenceError)
		return
	}
	s.logger.Info("notify tasks dropped due to persistence error", tag.Error(err))
}

// notifyTasks notifies the transfer and timer queue processors of new tasks.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasks(
	executionInfo *persistence.WorkflowExecutionInfo,
	tasksByCategory map[persistence.HistoryTaskCategory][]persistence.Task,
	persistenceError bool,
) {

	if transferTasks := tasksByCategory[persistence.HistoryTaskCategoryTransfer]; len(transferTasks) > 0 {
		s.GetEngine().NotifyNewTransferTasks(&hcommon.NotifyTaskInfo{
			ExecutionInfo:    executionInfo,
			Tasks:            transferTasks,
			PersistenceError: persistenceError,
		})
	}

	if timerTasks := tasksByCategory[persistence.HistoryTaskCategoryTimer]; len(timerTasks) > 0 {
		s.GetEngine().NotifyNewTimerTasks(&hcommon.NotifyTaskInfo{
			ExecutionInfo:       executionInfo,
			Tasks:               timerTasks,
			PersistenceError:    persistenceError,
			ClusterCurrentTimes: s.fetchClusterCurrentTimesLocked(timerTasks),
		})
	}
}

// notifyTasksFromSnapshot notifies queue processors of tasks from a workflow snapshot.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasksFromSnapshot(snapshot *persistence.WorkflowSnapshot, persistenceError bool) {
	if snapshot == nil {
		return
	}
	s.notifyTasks(snapshot.ExecutionInfo, snapshot.TasksByCategory, persistenceError)
}

// notifyTasksFromMutation notifies queue processors of tasks from a workflow mutation.
// Must be called while holding s.Lock().
func (s *contextImpl) notifyTasksFromMutation(mutation *persistence.WorkflowMutation, persistenceError bool) {
	if mutation == nil {
		return
	}
	s.notifyTasks(mutation.ExecutionInfo, mutation.TasksByCategory, persistenceError)
}
