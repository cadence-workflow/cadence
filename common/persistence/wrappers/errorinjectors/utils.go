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

package errorinjectors

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

// _randomStubFunc is a stub randomized function that could be overriden in tests.
// It introduces randomness to timeout and unhandled errors, to mimic retriable db isues.
var _randomStubFunc = func() bool {
	// forward the call with 50% chance
	return rand.Intn(2) == 0
}

func shouldForwardCallToPersistence(
	err error,
) bool {
	if err == nil {
		return true
	}

	if err == ErrFakeTimeout || err == errors.ErrFakeUnhandled {
		return _randomStubFunc()
	}

	return false
}

func generateFakeError(
	errorRate float64,
	starttime time.Time,
) error {
	// don't inject errors before 30 seconds of startup to avoid overwhelming the system
	// on startup
	if time.Since(starttime) < 30*time.Second {
		return nil
	}

	randFl := rand.Float64()
	if randFl < errorRate {
		return fakeErrors[rand.Intn(len(fakeErrors))]
	}

	return nil
}

var (
	// ErrFakeTimeout is a fake persistence timeout error.
	ErrFakeTimeout = &persistence.TimeoutError{Msg: "Fake Persistence Timeout Error."}
)

var (
	fakeErrors = []error{
		errors.ErrFakeServiceBusy,
		errors.ErrFakeInternalService,
		ErrFakeTimeout,
		errors.ErrFakeUnhandled,
	}
)

func isFakeError(err error) bool {
	for _, fakeErr := range fakeErrors {
		if err == fakeErr {
			return true
		}
	}

	return false
}

const (
	msgInjectedFakeErr = "Injected fake persistence error"
)

func logErr(logger log.Logger, objectMethod string, fakeErr error, forwardCall bool, err error) {
	logger.Error(msgInjectedFakeErr,
		getOperationFromMethodName(objectMethod),
		tag.Error(fakeErr),
		tag.Bool(forwardCall),
		tag.StoreError(err),
	)
}

func getOperationFromMethodName(op string) tag.Tag {
	var t *tag.Tag
	switch {
	case strings.HasPrefix(op, "ConfigStoreManager"):
		t = configManagerTags(op)
	case strings.HasPrefix(op, "DomainManager"):
		t = domainManagerTags(op)
	case strings.HasPrefix(op, "HistoryManager"):
		t = historyManagerTags(op)
	case strings.HasPrefix(op, "ShardManager"):
		t = shardManagerTags(op)
	case strings.HasPrefix(op, "ExecutionManager"):
		t = executionManagerTags(op)
	case strings.HasPrefix(op, "VisibilityManager"):
		t = visibilityManagerTags(op)
	case strings.HasPrefix(op, "QueueManager"):
		t = queueManagerTags(op)
	case strings.HasPrefix(op, "TaskManager"):
		t = taskManagerTags(op)
	}
	if t == nil {
		panic(fmt.Sprintf("No tag defined for operation %s", op))
	}
	return *t
}

func configManagerTags(op string) *tag.Tag {
	switch op {
	case "ConfigStoreManager.FetchDynamicConfig":
		return &tag.StoreOperationFetchDynamicConfig
	case "ConfigStoreManager.UpdateDynamicConfig":
		return &tag.StoreOperationUpdateDynamicConfig
	}
	return nil
}

func domainManagerTags(op string) *tag.Tag {
	switch op {
	case "DomainManager.CreateDomain":
		return &tag.StoreOperationCreateDomain
	case "DomainManager.GetDomain":
		return &tag.StoreOperationGetDomain
	case "DomainManager.DeleteDomain":
		return &tag.StoreOperationDeleteDomain
	case "DomainManager.DeleteDomainByName":
		return &tag.StoreOperationDeleteDomainByName
	case "DomainManager.ListDomains":
		return &tag.StoreOperationListDomains
	case "DomainManager.GetMetadata":
		return &tag.StoreOperationGetMetadata
	case "DomainManager.UpdateDomain":
		return &tag.StoreOperationUpdateDomain
	}
	return nil
}

func historyManagerTags(op string) *tag.Tag {
	switch op {
	case "HistoryManager.AppendHistoryNodes":
		return &tag.StoreOperationAppendHistoryNodes
	case "HistoryManager.DeleteHistoryBranch":
		return &tag.StoreOperationDeleteHistoryBranch
	case "HistoryManager.ForkHistoryBranch":
		return &tag.StoreOperationForkHistoryBranch
	case "HistoryManager.GetHistoryTree":
		return &tag.StoreOperationGetHistoryTree
	case "HistoryManager.ReadHistoryBranch":
		return &tag.StoreOperationReadHistoryBranch
	case "HistoryManager.ReadHistoryBranchByBatch":
		return &tag.StoreOperationReadHistoryBranchByBatch
	case "HistoryManager.ReadRawHistoryBranch":
		return &tag.StoreOperationReadRawHistoryBranch
	case "HistoryManager.GetAllHistoryTreeBranches":
		return &tag.StoreOperationGetAllHistoryTreeBranches
	}
	return nil
}

func shardManagerTags(op string) *tag.Tag {
	switch op {
	case "ShardManager.CreateShard":
		return &tag.StoreOperationCreateShard
	case "ShardManager.GetShard":
		return &tag.StoreOperationGetShard
	case "ShardManager.UpdateShard":
		return &tag.StoreOperationUpdateShard
	}
	return nil
}

func executionManagerTags(op string) *tag.Tag {
	switch op {
	case "ExecutionManager.CreateWorkflowExecution":
		return &tag.StoreOperationCreateWorkflowExecution
	case "ExecutionManager.GetWorkflowExecution":
		return &tag.StoreOperationGetWorkflowExecution
	case "ExecutionManager.UpdateWorkflowExecution":
		return &tag.StoreOperationUpdateWorkflowExecution
	case "ExecutionManager.ResetWorkflowExecution":
		return &tag.StoreOperationResetWorkflowExecution
	case "ExecutionManager.DeleteWorkflowExecution":
		return &tag.StoreOperationDeleteWorkflowExecution
	case "ExecutionManager.GetCurrentExecution":
		return &tag.StoreOperationGetCurrentExecution
	case "ExecutionManager.ConflictResolveWorkflowExecution":
		return &tag.StoreOperationConflictResolveWorkflowExecution
	case "ExecutionManager.DeleteCurrentWorkflowExecution":
		return &tag.StoreOperationDeleteCurrentWorkflowExecution
	case "ExecutionManager.ListCurrentExecutions":
		return &tag.StoreOperationListCurrentExecution
	case "ExecutionManager.IsWorkflowExecutionExists":
		return &tag.StoreOperationIsWorkflowExecutionExists
	case "ExecutionManager.ListConcreteExecutions":
		return &tag.StoreOperationListConcreteExecution
	case "ExecutionManager.GetTransferTasks":
		return &tag.StoreOperationGetTransferTasks
	case "ExecutionManager.GetCrossClusterTasks":
		return &tag.StoreOperationGetCrossClusterTasks
	case "ExecutionManager.GetReplicationTasks":
		return &tag.StoreOperationGetReplicationTasks
	case "ExecutionManager.CompleteTransferTask":
		return &tag.StoreOperationCompleteTransferTask
	case "ExecutionManager.GetHistoryTasks":
		return &tag.StoreOperationGetHistoryTasks
	case "ExecutionManager.CompleteHistoryTask":
		return &tag.StoreOperationCompleteHistoryTask
	case "ExecutionManager.RangeCompleteHistoryTask":
		return &tag.StoreOperationRangeCompleteHistoryTask
	case "ExecutionManager.CompleteCrossClusterTask":
		return &tag.StoreOperationCompleteCrossClusterTask
	case "ExecutionManager.CompleteReplicationTask":
		return &tag.StoreOperationCompleteReplicationTask
	case "ExecutionManager.PutReplicationTaskToDLQ":
		return &tag.StoreOperationPutReplicationTaskToDLQ
	case "ExecutionManager.GetReplicationTasksFromDLQ":
		return &tag.StoreOperationGetReplicationTasksFromDLQ
	case "ExecutionManager.GetReplicationDLQSize":
		return &tag.StoreOperationGetReplicationDLQSize
	case "ExecutionManager.DeleteReplicationTaskFromDLQ":
		return &tag.StoreOperationDeleteReplicationTaskFromDLQ
	case "ExecutionManager.RangeDeleteReplicationTaskFromDLQ":
		return &tag.StoreOperationRangeDeleteReplicationTaskFromDLQ
	case "ExecutionManager.GetTimerIndexTasks":
		return &tag.StoreOperationGetTimerIndexTasks
	case "ExecutionManager.CompleteTimerTask":
		return &tag.StoreOperationCompleteTimerTask
	case "ExecutionManager.CreateFailoverMarkerTasks":
		return &tag.StoreOperationCreateFailoverMarkerTasks
	case "ExecutionManager.GetActiveClusterSelectionPolicy":
		return &tag.StoreOperationGetActiveClusterSelectionPolicy
	case "ExecutionManager.DeleteActiveClusterSelectionPolicy":
		return &tag.StoreOperationDeleteActiveClusterSelectionPolicy
	}
	return nil
}

func visibilityManagerTags(op string) *tag.Tag {
	switch op {
	case "VisibilityManager.RecordWorkflowExecutionStarted":
		return &tag.StoreOperationRecordWorkflowExecutionStarted
	case "VisibilityManager.RecordWorkflowExecutionClosed":
		return &tag.StoreOperationRecordWorkflowExecutionClosed
	case "VisibilityManager.UpsertWorkflowExecution":
		return &tag.StoreOperationUpsertWorkflowExecution
	case "VisibilityManager.ListOpenWorkflowExecutions":
		return &tag.StoreOperationListOpenWorkflowExecutions
	case "VisibilityManager.ListClosedWorkflowExecutions":
		return &tag.StoreOperationListClosedWorkflowExecutions
	case "VisibilityManager.ListOpenWorkflowExecutionsByType":
		return &tag.StoreOperationListOpenWorkflowExecutionsByType
	case "VisibilityManager.ListClosedWorkflowExecutionsByType":
		return &tag.StoreOperationListClosedWorkflowExecutionsByType
	case "VisibilityManager.ListOpenWorkflowExecutionsByWorkflowID":
		return &tag.StoreOperationListOpenWorkflowExecutionsByWorkflowID
	case "VisibilityManager.ListClosedWorkflowExecutionsByWorkflowID":
		return &tag.StoreOperationListClosedWorkflowExecutionsByWorkflowID
	case "VisibilityManager.ListClosedWorkflowExecutionsByStatus":
		return &tag.StoreOperationListClosedWorkflowExecutionsByStatus
	case "VisibilityManager.GetClosedWorkflowExecution":
		return &tag.StoreOperationGetClosedWorkflowExecution
	case "VisibilityManager.DeleteWorkflowExecution":
		return &tag.StoreOperationDeleteWorkflowExecution
	case "VisibilityManager.ListWorkflowExecutions":
		return &tag.StoreOperationListWorkflowExecutions
	case "VisibilityManager.ScanWorkflowExecutions":
		return &tag.StoreOperationScanWorkflowExecutions
	case "VisibilityManager.CountWorkflowExecutions":
		return &tag.StoreOperationCountWorkflowExecutions
	case "VisibilityManager.DeleteUninitializedWorkflowExecution":
		return &tag.StoreOperationDeleteUninitializedWorkflowExecution
	case "VisibilityManager.RecordWorkflowExecutionUninitialized":
		return &tag.StoreOperationRecordWorkflowExecutionUninitialized
	}
	return nil
}

func taskManagerTags(op string) *tag.Tag {
	switch op {
	case "TaskManager.LeaseTaskList":
		return &tag.StoreOperationLeaseTaskList
	case "TaskManager.GetTaskList":
		return &tag.StoreOperationGetTaskList
	case "TaskManager.UpdateTaskList":
		return &tag.StoreOperationUpdateTaskList
	case "TaskManager.CreateTasks":
		return &tag.StoreOperationCreateTasks
	case "TaskManager.GetTasks":
		return &tag.StoreOperationGetTasks
	case "TaskManager.CompleteTask":
		return &tag.StoreOperationCompleteTask
	case "TaskManager.CompleteTasksLessThan":
		return &tag.StoreOperationCompleteTasksLessThan
	case "TaskManager.DeleteTaskList":
		return &tag.StoreOperationDeleteTaskList
	case "TaskManager.GetOrphanTasks":
		return &tag.StoreOperationGetOrphanTasks
	case "TaskManager.GetTaskListSize":
		return &tag.StoreOperationGetTaskListSize
	case "TaskManager.ListTaskList":
		return &tag.StoreOperationListTaskList
	}
	return nil
}

func queueManagerTags(op string) *tag.Tag {
	switch op {
	case "QueueManager.EnqueueMessage":
		return &tag.StoreOperationEnqueueMessage
	case "QueueManager.EnqueueMessageToDLQ":
		return &tag.StoreOperationEnqueueMessageToDLQ
	case "QueueManager.DeleteMessageFromDLQ":
		return &tag.StoreOperationDeleteMessageFromDLQ
	case "QueueManager.RangeDeleteMessagesFromDLQ":
		return &tag.StoreOperationRangeDeleteMessagesFromDLQ
	case "QueueManager.UpdateAckLevel":
		return &tag.StoreOperationUpdateAckLevel
	case "QueueManager.GetAckLevels":
		return &tag.StoreOperationGetAckLevels
	case "QueueManager.UpdateDLQAckLevel":
		return &tag.StoreOperationUpdateDLQAckLevel
	case "QueueManager.GetDLQAckLevels":
		return &tag.StoreOperationGetDLQAckLevels
	case "QueueManager.GetDLQSize":
		return &tag.StoreOperationGetDLQSize
	case "QueueManager.DeleteMessagesBefore":
		return &tag.StoreOperationDeleteMessagesBefore
	case "QueueManager.ReadMessages":
		return &tag.StoreOperationReadMessages
	case "QueueManager.ReadMessagesFromDLQ":
		return &tag.StoreOperationReadMessagesFromDLQ
	}
	return nil
}
