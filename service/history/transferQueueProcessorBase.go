// Copyright (c) 2017 Uber Technologies, Inc.
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

package history

import (
	"github.com/uber-common/bark"
	m "github.com/uber/cadence/.gen/go/matching"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/persistence"
)

type (
	maxReadAckLevel func() int64

	updateTransferAckLevel func(ackLevel int64) error
	transferQueueShutdown  func() error

	transferQueueProcessorBase struct {
		shard                  ShardContext
		options                *QueueProcessorOptions
		executionManager       persistence.ExecutionManager
		visibilityMgr          persistence.VisibilityManager
		matchingClient         matching.Client
		maxReadAckLevel        maxReadAckLevel
		updateTransferAckLevel updateTransferAckLevel
		transferQueueShutdown  transferQueueShutdown
		logger                 bark.Logger
	}
)

func newTransferQueueProcessorBase(shard ShardContext, options *QueueProcessorOptions,
	visibilityMgr persistence.VisibilityManager, matchingClient matching.Client,
	maxReadAckLevel maxReadAckLevel, updateTransferAckLevel updateTransferAckLevel,
	transferQueueShutdown transferQueueShutdown, logger bark.Logger) *transferQueueProcessorBase {
	return &transferQueueProcessorBase{
		shard:                  shard,
		options:                options,
		executionManager:       shard.GetExecutionManager(),
		visibilityMgr:          visibilityMgr,
		matchingClient:         matchingClient,
		maxReadAckLevel:        maxReadAckLevel,
		updateTransferAckLevel: updateTransferAckLevel,
		transferQueueShutdown:  transferQueueShutdown,
		logger:                 logger,
	}
}

func (t *transferQueueProcessorBase) readTasks(readLevel int64) ([]queueTaskInfo, bool, error) {
	response, err := t.executionManager.GetTransferTasks(&persistence.GetTransferTasksRequest{
		ReadLevel:    readLevel,
		MaxReadLevel: t.maxReadAckLevel(),
		BatchSize:    t.options.BatchSize(),
	})

	if err != nil {
		return nil, false, err
	}

	tasks := make([]queueTaskInfo, len(response.Tasks))
	for i := range response.Tasks {
		tasks[i] = response.Tasks[i]
	}

	return tasks, len(response.NextPageToken) != 0, nil
}

func (t *transferQueueProcessorBase) completeTask(taskID int64) error {
	// this is a no op on the for transfer queue active / standby processor
	return nil
}

func (t *transferQueueProcessorBase) updateAckLevel(ackLevel int64) error {
	return t.updateTransferAckLevel(ackLevel)
}

func (t *transferQueueProcessorBase) queueShutdown() error {
	return t.transferQueueShutdown()
}

func (t *transferQueueProcessorBase) pushActivity(task *persistence.TransferTaskInfo, activityScheduleToStartTimeout int32) error {
	if task.TaskType != persistence.TransferTaskTypeActivityTask {
		t.logger.WithField(logging.TagTaskType, task.GetTaskType()).Fatal("Cannnot process non activity task")
	}

	err := t.matchingClient.AddActivityTask(nil, &m.AddActivityTaskRequest{
		DomainUUID:       common.StringPtr(task.TargetDomainID),
		SourceDomainUUID: common.StringPtr(task.DomainID),
		Execution: &workflow.WorkflowExecution{
			WorkflowId: common.StringPtr(task.WorkflowID),
			RunId:      common.StringPtr(task.RunID),
		},
		TaskList:                      &workflow.TaskList{Name: &task.TaskList},
		ScheduleId:                    &task.ScheduleID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(activityScheduleToStartTimeout),
	})

	return err
}

func (t *transferQueueProcessorBase) pushDecision(task *persistence.TransferTaskInfo, tasklist *workflow.TaskList, decisionScheduleToStartTimeout int32) error {
	if task.TaskType != persistence.TransferTaskTypeDecisionTask {
		t.logger.WithField(logging.TagTaskType, task.GetTaskType()).Fatal("Cannnot process non decision task")
	}

	err := t.matchingClient.AddDecisionTask(nil, &m.AddDecisionTaskRequest{
		DomainUUID: common.StringPtr(task.DomainID),
		Execution: &workflow.WorkflowExecution{
			WorkflowId: common.StringPtr(task.WorkflowID),
			RunId:      common.StringPtr(task.RunID),
		},
		TaskList:                      tasklist,
		ScheduleId:                    common.Int64Ptr(task.ScheduleID),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(decisionScheduleToStartTimeout),
	})

	return err
}

func (t *transferQueueProcessorBase) recordWorkflowStarted(
	domainID string, execution workflow.WorkflowExecution, workflowTypeName string,
	startTimeUnixNano int64, workflowTimeout int32) error {

	return t.visibilityMgr.RecordWorkflowExecutionStarted(&persistence.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       domainID,
		Execution:        execution,
		WorkflowTypeName: workflowTypeName,
		StartTimestamp:   startTimeUnixNano,
		WorkflowTimeout:  int64(workflowTimeout),
	})
}

func (t *transferQueueProcessorBase) recordWorkflowClosed(
	domainID string, execution workflow.WorkflowExecution, workflowTypeName string,
	startTimeUnixNano int64, endTimeUnixNano int64, closeStatus workflow.WorkflowExecutionCloseStatus,
	historyLength int64) error {
	// Record closing in visibility store
	retentionSeconds := int64(0)
	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		if _, ok := err.(*workflow.EntityNotExistsError); !ok {
			return err
		}
		// it is possible that the domain got deleted. Use default retention.
	} else {
		// retention in domain config is in days, convert to seconds
		retentionSeconds = int64(domainEntry.GetConfig().Retention) * 24 * 60 * 60
	}

	return t.visibilityMgr.RecordWorkflowExecutionClosed(&persistence.RecordWorkflowExecutionClosedRequest{
		DomainUUID:       domainID,
		Execution:        execution,
		WorkflowTypeName: workflowTypeName,
		StartTimestamp:   startTimeUnixNano,
		CloseTimestamp:   endTimeUnixNano,
		Status:           closeStatus,
		HistoryLength:    historyLength,
		RetentionSeconds: retentionSeconds,
	})
}
