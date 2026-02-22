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

//go:generate mockgen -package $GOPACKAGE -destination task_serializer_mock.go github.com/uber/cadence/common/persistence/serialization TaskSerializer

package serialization

import (
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	TaskSerializer interface {
		SerializeTask(persistence.HistoryTaskCategory, persistence.Task) (persistence.DataBlob, error)
		DeserializeTask(persistence.HistoryTaskCategory, *persistence.DataBlob) (persistence.Task, error)
	}

	taskSerializerImpl struct {
		parser Parser
	}
)

func NewTaskSerializer(parser Parser) TaskSerializer {
	return &taskSerializerImpl{
		parser: parser,
	}
}

func (s *taskSerializerImpl) SerializeTask(category persistence.HistoryTaskCategory, task persistence.Task) (persistence.DataBlob, error) {
	switch category.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		return s.serializeTransferTask(task)
	case persistence.HistoryTaskCategoryIDTimer:
		return s.serializeTimerTask(task)
	case persistence.HistoryTaskCategoryIDReplication:
		return s.serializeReplicationTask(task)
	default:
		return persistence.DataBlob{}, fmt.Errorf("unknown category ID: %v", category.ID())
	}
}

func (s *taskSerializerImpl) DeserializeTask(category persistence.HistoryTaskCategory, blob *persistence.DataBlob) (persistence.Task, error) {
	switch category.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		return s.deserializeTransferTask(blob)
	case persistence.HistoryTaskCategoryIDTimer:
		return s.deserializeTimerTask(blob)
	case persistence.HistoryTaskCategoryIDReplication:
		return s.deserializeReplicationTask(blob)
	default:
		return nil, fmt.Errorf("unknown category ID: %v", category.ID())
	}
}

func (s *taskSerializerImpl) serializeTransferTask(task persistence.Task) (persistence.DataBlob, error) {
	info := &TransferTaskInfo{
		TaskType:            int16(task.GetTaskType()),
		TargetWorkflowID:    persistence.TransferTaskTransferTargetWorkflowID,
		Version:             task.GetVersion(),
		VisibilityTimestamp: task.GetVisibilityTimestamp(),
	}
	switch t := task.(type) {
	case *persistence.ActivityTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TaskList = t.TaskList
		info.ScheduleID = t.ScheduleID
	case *persistence.DecisionTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TaskList = t.TaskList
		info.ScheduleID = t.ScheduleID
		info.OriginalTaskList = t.OriginalTaskList
	case *persistence.CancelExecutionTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TargetWorkflowID = t.TargetWorkflowID
		if t.TargetRunID != "" {
			info.TargetRunID = MustParseUUID(t.TargetRunID)
		}
		info.TargetChildWorkflowOnly = t.TargetChildWorkflowOnly
		info.ScheduleID = t.InitiatedID
		info.TaskList = t.TaskList
	case *persistence.SignalExecutionTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TargetWorkflowID = t.TargetWorkflowID
		if t.TargetRunID != "" {
			info.TargetRunID = MustParseUUID(t.TargetRunID)
		}
		info.TargetChildWorkflowOnly = t.TargetChildWorkflowOnly
		info.ScheduleID = t.InitiatedID
		info.TaskList = t.TaskList
	case *persistence.StartChildExecutionTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TargetWorkflowID = t.TargetWorkflowID
		info.ScheduleID = t.InitiatedID
		info.TaskList = t.TaskList
	case *persistence.RecordChildExecutionCompletedTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TargetDomainID = MustParseUUID(t.TargetDomainID)
		info.TargetWorkflowID = t.TargetWorkflowID
		if t.TargetRunID != "" {
			info.TargetRunID = MustParseUUID(t.TargetRunID)
		}
		info.TaskList = t.TaskList
	case *persistence.CloseExecutionTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	case *persistence.RecordWorkflowStartedTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	case *persistence.RecordWorkflowClosedTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	case *persistence.ResetWorkflowTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	case *persistence.UpsertWorkflowSearchAttributesTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	default:
		return persistence.DataBlob{}, &types.InternalServiceError{
			Message: fmt.Sprintf("Unknown transfer type: %v", task.GetTaskType()),
		}
	}
	return s.parser.TransferTaskInfoToBlob(info)
}

func (s *taskSerializerImpl) deserializeTransferTask(blob *persistence.DataBlob) (persistence.Task, error) {
	info, err := s.parser.TransferTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	if err != nil {
		return nil, err
	}
	var task persistence.Task
	workflowIdentifier := persistence.WorkflowIdentifier{
		DomainID:   info.DomainID.String(),
		WorkflowID: info.GetWorkflowID(),
		RunID:      info.RunID.String(),
	}
	taskData := persistence.TaskData{
		Version:             info.GetVersion(),
		VisibilityTimestamp: info.GetVisibilityTimestamp(),
	}
	switch info.GetTaskType() {
	case persistence.TransferTaskTypeDecisionTask:
		task = &persistence.DecisionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     info.TargetDomainID.String(),
			TaskList:           info.GetTaskList(),
			ScheduleID:         info.GetScheduleID(),
			OriginalTaskList:   info.GetOriginalTaskList(),
		}
	case persistence.TransferTaskTypeActivityTask:
		task = &persistence.ActivityTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     info.TargetDomainID.String(),
			TaskList:           info.GetTaskList(),
			ScheduleID:         info.GetScheduleID(),
		}
	case persistence.TransferTaskTypeCloseExecution:
		task = &persistence.CloseExecutionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeCancelExecution:
		task = &persistence.CancelExecutionTask{
			WorkflowIdentifier:      workflowIdentifier,
			TaskData:                taskData,
			TargetDomainID:          info.TargetDomainID.String(),
			TargetWorkflowID:        info.GetTargetWorkflowID(),
			TargetRunID:             info.TargetRunID.String(),
			TargetChildWorkflowOnly: info.GetTargetChildWorkflowOnly(),
			InitiatedID:             info.GetScheduleID(),
			TaskList:                info.GetTaskList(),
		}
	case persistence.TransferTaskTypeStartChildExecution:
		task = &persistence.StartChildExecutionTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     info.TargetDomainID.String(),
			TargetWorkflowID:   info.GetTargetWorkflowID(),
			InitiatedID:        info.GetScheduleID(),
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeSignalExecution:
		task = &persistence.SignalExecutionTask{
			WorkflowIdentifier:      workflowIdentifier,
			TaskData:                taskData,
			TargetDomainID:          info.TargetDomainID.String(),
			TargetWorkflowID:        info.GetTargetWorkflowID(),
			TargetRunID:             info.TargetRunID.String(),
			TargetChildWorkflowOnly: info.GetTargetChildWorkflowOnly(),
			InitiatedID:             info.GetScheduleID(),
			TaskList:                info.GetTaskList(),
		}
	case persistence.TransferTaskTypeRecordWorkflowStarted:
		task = &persistence.RecordWorkflowStartedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeResetWorkflow:
		task = &persistence.ResetWorkflowTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeUpsertWorkflowSearchAttributes:
		task = &persistence.UpsertWorkflowSearchAttributesTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeRecordWorkflowClosed:
		task = &persistence.RecordWorkflowClosedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TransferTaskTypeRecordChildExecutionCompleted:
		task = &persistence.RecordChildExecutionCompletedTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TargetDomainID:     info.TargetDomainID.String(),
			TargetWorkflowID:   info.GetTargetWorkflowID(),
			TargetRunID:        info.TargetRunID.String(),
			TaskList:           info.GetTaskList(),
		}
	default:
		return nil, fmt.Errorf("unknown transfer task type: %v", info.GetTaskType())
	}
	return task, nil
}

func (s *taskSerializerImpl) serializeTimerTask(task persistence.Task) (persistence.DataBlob, error) {
	info := &TimerTaskInfo{
		TaskType: int16(task.GetTaskType()),
		Version:  task.GetVersion(),
		EventID:  constants.EmptyEventID,
	}
	switch t := task.(type) {
	case *persistence.DecisionTimeoutTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.EventID = t.EventID
		info.TimeoutType = common.Int16Ptr(int16(t.TimeoutType))
		info.ScheduleAttempt = t.ScheduleAttempt
		info.TaskList = t.TaskList
	case *persistence.ActivityTimeoutTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.EventID = t.EventID
		info.TimeoutType = common.Int16Ptr(int16(t.TimeoutType))
		info.ScheduleAttempt = t.Attempt
		info.TaskList = t.TaskList
	case *persistence.UserTimerTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.EventID = t.EventID
		info.TaskList = t.TaskList
	case *persistence.ActivityRetryTimerTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.EventID = t.EventID
		info.ScheduleAttempt = int64(t.Attempt)
		info.TaskList = t.TaskList
	case *persistence.WorkflowBackoffTimerTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TimeoutType = common.Int16Ptr(int16(t.TimeoutType))
		info.TaskList = t.TaskList
	case *persistence.WorkflowTimeoutTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	case *persistence.DeleteHistoryEventTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.TaskList = t.TaskList
	default:
		return persistence.DataBlob{}, &types.InternalServiceError{
			Message: fmt.Sprintf("Unknown timer task: %v", task.GetTaskType()),
		}
	}
	return s.parser.TimerTaskInfoToBlob(info)
}

func (s *taskSerializerImpl) deserializeTimerTask(blob *persistence.DataBlob) (persistence.Task, error) {
	info, err := s.parser.TimerTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	if err != nil {
		return nil, err
	}
	var task persistence.Task
	workflowIdentifier := persistence.WorkflowIdentifier{
		DomainID:   info.DomainID.String(),
		WorkflowID: info.GetWorkflowID(),
		RunID:      info.RunID.String(),
	}
	taskData := persistence.TaskData{
		Version: info.GetVersion(),
	}
	switch info.GetTaskType() {
	case persistence.TaskTypeDecisionTimeout:
		task = &persistence.DecisionTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            info.GetEventID(),
			ScheduleAttempt:    info.GetScheduleAttempt(),
			TimeoutType:        int(info.GetTimeoutType()),
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeActivityTimeout:
		task = &persistence.ActivityTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            info.GetEventID(),
			Attempt:            info.GetScheduleAttempt(),
			TimeoutType:        int(info.GetTimeoutType()),
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeUserTimer:
		task = &persistence.UserTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            info.GetEventID(),
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeWorkflowTimeout:
		task = &persistence.WorkflowTimeoutTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeDeleteHistoryEvent:
		task = &persistence.DeleteHistoryEventTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeActivityRetryTimer:
		task = &persistence.ActivityRetryTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			EventID:            info.GetEventID(),
			Attempt:            info.GetScheduleAttempt(),
			TaskList:           info.GetTaskList(),
		}
	case persistence.TaskTypeWorkflowBackoffTimer:
		task = &persistence.WorkflowBackoffTimerTask{
			WorkflowIdentifier: workflowIdentifier,
			TaskData:           taskData,
			TimeoutType:        int(info.GetTimeoutType()),
			TaskList:           info.GetTaskList(),
		}
	default:
		return nil, fmt.Errorf("unknown timer task type: %v", info.GetTaskType())
	}
	return task, nil
}

func (s *taskSerializerImpl) serializeReplicationTask(task persistence.Task) (persistence.DataBlob, error) {
	info := &ReplicationTaskInfo{
		TaskType:                int16(task.GetTaskType()),
		FirstEventID:            constants.EmptyEventID,
		NextEventID:             constants.EmptyEventID,
		Version:                 task.GetVersion(),
		ScheduledID:             constants.EmptyEventID,
		EventStoreVersion:       persistence.EventStoreVersion,
		NewRunEventStoreVersion: persistence.EventStoreVersion,
		CreationTimestamp:       task.GetVisibilityTimestamp(),
	}
	switch t := task.(type) {
	case *persistence.HistoryReplicationTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.FirstEventID = t.FirstEventID
		info.NextEventID = t.NextEventID
		info.BranchToken = t.BranchToken
		info.NewRunBranchToken = t.NewRunBranchToken
	case *persistence.SyncActivityTask:
		info.DomainID = MustParseUUID(t.DomainID)
		info.WorkflowID = t.WorkflowID
		info.RunID = MustParseUUID(t.RunID)
		info.ScheduledID = t.ScheduledID
	case *persistence.FailoverMarkerTask:
		info.DomainID = MustParseUUID(t.DomainID)
	default:
		return persistence.DataBlob{}, &types.InternalServiceError{
			Message: fmt.Sprintf("Unknown replication task: %v", task.GetTaskType()),
		}
	}
	return s.parser.ReplicationTaskInfoToBlob(info)
}

func (s *taskSerializerImpl) deserializeReplicationTask(blob *persistence.DataBlob) (persistence.Task, error) {
	info, err := s.parser.ReplicationTaskInfoFromBlob(blob.Data, string(blob.Encoding))
	if err != nil {
		return nil, err
	}
	var task persistence.Task
	taskData := persistence.TaskData{
		Version:             info.GetVersion(),
		VisibilityTimestamp: info.GetCreationTimestamp(),
	}
	switch info.GetTaskType() {
	case persistence.ReplicationTaskTypeHistory:
		task = &persistence.HistoryReplicationTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   info.DomainID.String(),
				WorkflowID: info.GetWorkflowID(),
				RunID:      info.RunID.String(),
			},
			TaskData:          taskData,
			FirstEventID:      info.GetFirstEventID(),
			NextEventID:       info.GetNextEventID(),
			BranchToken:       info.BranchToken,
			NewRunBranchToken: info.NewRunBranchToken,
		}
	case persistence.ReplicationTaskTypeSyncActivity:
		task = &persistence.SyncActivityTask{
			WorkflowIdentifier: persistence.WorkflowIdentifier{
				DomainID:   info.DomainID.String(),
				WorkflowID: info.GetWorkflowID(),
				RunID:      info.RunID.String(),
			},
			TaskData:    taskData,
			ScheduledID: info.GetScheduledID(),
		}
	case persistence.ReplicationTaskTypeFailoverMarker:
		task = &persistence.FailoverMarkerTask{
			DomainID: info.DomainID.String(),
			TaskData: taskData,
		}
	}
	return task, nil
}
