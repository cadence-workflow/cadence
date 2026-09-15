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
	"time"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
)

// taskNotifier owns the task notification path: given a persistence request and the error its
// write returned, it decides whether the transfer/timer queue processors should be told about the
// tasks that request carried.
//
// It holds no reference back to the shard. Everything it needs from the shard arrives either as a
// value at construction or behind one of the two function fields below.
type taskNotifier struct {
	shardID         int
	config          *config.Config
	clusterMetadata cluster.Metadata
	logger          log.Logger

	// getEngine must be late-bound: the history engine is attached to the shard after the shard
	// is constructed, via SetEngine. The shard's engine field is read and written without
	// synchronization, so resolving it through a method value here is identical to calling
	// GetEngine() inline, which is what this code did before it moved.
	getEngine func() engine.Engine

	// getCurrentTime returns a cluster's current time WITHOUT taking the shard lock; the caller
	// is required to already hold it. Every notification originates from a persistence write
	// performed inside the shard's critical section, so that precondition holds by construction
	// today. See notifyTasks.
	getCurrentTime func(cluster string) time.Time
}

func newTaskNotifier(
	shardID int,
	config *config.Config,
	clusterMetadata cluster.Metadata,
	logger log.Logger,
	getEngine func() engine.Engine,
	getCurrentTime func(cluster string) time.Time,
) *taskNotifier {
	return &taskNotifier{
		shardID:         shardID,
		config:          config,
		clusterMetadata: clusterMetadata,
		logger:          logger,
		getEngine:       getEngine,
		getCurrentTime:  getCurrentTime,
	}
}

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
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromCreateWorkflowExecution(
	request *persistence.CreateWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		n.notifyTasksFromSnapshot(&request.NewWorkflowSnapshot, persistenceError)
		return
	}
	n.logNotifyTaskDroppedOnPersistenceError(err, snapshotTasks(&request.NewWorkflowSnapshot))
}

// notifyTasksFromUpdateWorkflowExecution sends task notifications for an UpdateWorkflowExecution operation.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromUpdateWorkflowExecution(
	request *persistence.UpdateWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		n.notifyTasksFromMutation(&request.UpdateWorkflowMutation, persistenceError)
		n.notifyTasksFromSnapshot(request.NewWorkflowSnapshot, persistenceError)
		return
	}
	n.logNotifyTaskDroppedOnPersistenceError(err,
		mutationTasks(&request.UpdateWorkflowMutation),
		snapshotTasks(request.NewWorkflowSnapshot),
	)
}

// notifyTasksFromConflictResolveWorkflowExecution sends task notifications for a ConflictResolveWorkflowExecution operation.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromConflictResolveWorkflowExecution(
	request *persistence.ConflictResolveWorkflowExecutionRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		n.notifyTasksFromSnapshot(&request.ResetWorkflowSnapshot, persistenceError)
		n.notifyTasksFromSnapshot(request.NewWorkflowSnapshot, persistenceError)
		n.notifyTasksFromMutation(request.CurrentWorkflowMutation, persistenceError)
		return
	}
	n.logNotifyTaskDroppedOnPersistenceError(err,
		snapshotTasks(&request.ResetWorkflowSnapshot),
		snapshotTasks(request.NewWorkflowSnapshot),
		mutationTasks(request.CurrentWorkflowMutation),
	)
}

// notifyTasksFromCreateHistoryTasks sends task notifications for a CreateHistoryTasks operation,
// which is how DLQ tasks are re-injected into the executions table.
// Unlike the other notifyTasksFrom* functions, one such write can span multiple executions, so
// there is no single WorkflowExecutionInfo to notify with; ExecutionInfo is left nil since none of
// the transfer/timer notification consumers dereference it.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromCreateHistoryTasks(
	request *persistence.CreateHistoryTasksRequest,
	err error,
) {
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		n.notifyTasks(nil, request.TasksByCategory, persistenceError)
		return
	}
	n.logNotifyTaskDroppedOnPersistenceError(err, request.TasksByCategory)
}

// notifyTasksFromCreateFailoverMarkerTasks sends task notifications for a CreateFailoverMarkerTasks
// operation. Failover markers are replication-category tasks, and notifyTasks reads only the
// transfer and timer categories, so this is a no-op in practice. It exists so that no task-carrying
// persistence write is exempt from the notification path by construction.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromCreateFailoverMarkerTasks(
	request *persistence.CreateFailoverMarkersRequest,
	err error,
) {
	tasksByCategory := make(persistence.HistoryTasksByCategory, 1)
	for _, marker := range request.Markers {
		tasksByCategory[persistence.HistoryTaskCategoryReplication] = append(
			tasksByCategory[persistence.HistoryTaskCategoryReplication], marker)
	}
	if notify, persistenceError := isNotifyTaskNeeded(err); notify {
		n.notifyTasks(nil, tasksByCategory, persistenceError)
		return
	}
	n.logNotifyTaskDroppedOnPersistenceError(err, tasksByCategory)
}

// logNotifyTaskDroppedOnPersistenceError logs dropped task IDs per category, but only for a category
// whose cached queue reader is in shadow mode, where the cache is validated against the DB and a
// dropped task surfaces as an observable mismatch. For other modes (and categories with no cached
// reader, e.g. replication) the log is just noise, so when no cache is shadowing it returns before
// touching the sources, keeping the steady-state path free of per-task work.
func (n *taskNotifier) logNotifyTaskDroppedOnPersistenceError(
	err error,
	sources ...map[persistence.HistoryTaskCategory][]persistence.Task,
) {
	timerCacheShadow := n.config.TimerProcessorCachedQueueReaderMode(n.shardID) == "shadow"
	transferCacheShadow := n.config.TransferProcessorCachedQueueReaderMode(n.shardID) == "shadow"
	if !timerCacheShadow && !transferCacheShadow {
		return
	}

	var droppedTimerTaskIDs, droppedTransferTaskIDs []int64
	for _, src := range sources {
		if timerCacheShadow {
			droppedTimerTaskIDs = appendTaskIDs(droppedTimerTaskIDs, src[persistence.HistoryTaskCategoryTimer])
		}
		if transferCacheShadow {
			droppedTransferTaskIDs = appendTaskIDs(droppedTransferTaskIDs, src[persistence.HistoryTaskCategoryTransfer])
		}
	}
	if len(droppedTimerTaskIDs) == 0 && len(droppedTransferTaskIDs) == 0 {
		return
	}
	n.logger.Info("notify tasks dropped due to persistence error",
		tag.Error(err),
		tag.Dynamic("droppedTimerTaskIDs", droppedTimerTaskIDs),
		tag.Dynamic("droppedTransferTaskIDs", droppedTransferTaskIDs),
	)
}

// notifyTasks notifies the transfer and timer queue processors of new tasks.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasks(
	executionInfo *persistence.WorkflowExecutionInfo,
	tasksByCategory map[persistence.HistoryTaskCategory][]persistence.Task,
	persistenceError bool,
) {

	if transferTasks := tasksByCategory[persistence.HistoryTaskCategoryTransfer]; len(transferTasks) > 0 {
		n.getEngine().NotifyNewTransferTasks(&hcommon.NotifyTaskInfo{
			ExecutionInfo:    executionInfo,
			Tasks:            transferTasks,
			PersistenceError: persistenceError,
		})
	}

	if timerTasks := tasksByCategory[persistence.HistoryTaskCategoryTimer]; len(timerTasks) > 0 {
		n.getEngine().NotifyNewTimerTasks(&hcommon.NotifyTaskInfo{
			ExecutionInfo:       executionInfo,
			Tasks:               timerTasks,
			PersistenceError:    persistenceError,
			ClusterCurrentTimes: n.fetchClusterCurrentTimesLocked(timerTasks),
		})
	}
}

// notifyTasksFromSnapshot notifies queue processors of tasks from a workflow snapshot.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromSnapshot(snapshot *persistence.WorkflowSnapshot, persistenceError bool) {
	if snapshot == nil {
		return
	}
	n.notifyTasks(snapshot.ExecutionInfo, snapshot.TasksByCategory, persistenceError)
}

// notifyTasksFromMutation notifies queue processors of tasks from a workflow mutation.
// Must be called while holding the shard lock.
func (n *taskNotifier) notifyTasksFromMutation(mutation *persistence.WorkflowMutation, persistenceError bool) {
	if mutation == nil {
		return
	}
	n.notifyTasks(mutation.ExecutionInfo, mutation.TasksByCategory, persistenceError)
}

// fetchClusterCurrentTimesLocked returns current times for all standby clusters referenced by timerTasks.
// Caller must hold the shard's Lock() or RLock(), since getCurrentTime does not take either.
func (n *taskNotifier) fetchClusterCurrentTimesLocked(timerTasks []persistence.Task) map[string]time.Time {
	currentCluster := n.clusterMetadata.GetCurrentClusterName()
	clusterTimes := make(map[string]time.Time)
	for _, task := range timerTasks {
		clusterName, err := n.clusterMetadata.ClusterNameForFailoverVersion(task.GetVersion())
		if err != nil || clusterName == currentCluster {
			continue
		}
		if _, exists := clusterTimes[clusterName]; !exists {
			clusterTimes[clusterName] = n.getCurrentTime(clusterName)
		}
	}
	return clusterTimes
}

func snapshotTasks(snapshot *persistence.WorkflowSnapshot) map[persistence.HistoryTaskCategory][]persistence.Task {
	if snapshot == nil {
		return nil
	}
	return snapshot.TasksByCategory
}

func mutationTasks(mutation *persistence.WorkflowMutation) map[persistence.HistoryTaskCategory][]persistence.Task {
	if mutation == nil {
		return nil
	}
	return mutation.TasksByCategory
}

func appendTaskIDs(ids []int64, tasks []persistence.Task) []int64 {
	for _, t := range tasks {
		ids = append(ids, t.GetTaskID())
	}
	return ids
}
