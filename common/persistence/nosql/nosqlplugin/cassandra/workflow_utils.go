// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package cassandra

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

func executeCreateWorkflowBatchTransaction(
	ctx context.Context,
	session gocql.Session,
	batch gocql.Batch,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	execution *nosqlplugin.WorkflowExecutionRequest,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	previous := make(map[string]interface{})
	applied, iter, err := session.MapExecuteBatchCAS(batch, previous)
	defer func() {
		if iter != nil {
			_ = iter.Close()
		}
	}()

	if err != nil {
		return err
	}

	if applied {
		return nil
	}

	requestRangeID := shardCondition.RangeID
	requestConditionalRunID := ""
	if currentWorkflowRequest.Condition != nil {
		requestConditionalRunID = currentWorkflowRequest.Condition.GetCurrentRunID()
	}
	// There can be two reasons why the query does not get applied. Either the RangeID has changed, or
	// the workflow is already started. Check the row info returned by Cassandra to figure out which one it is.
	rangeIDMismatch := false
	actualRangeID := int64(0)
	currentExecutionAlreadyExists := false
	var actualExecution map[string]interface{}
	runIDMismatch := false
	actualCurrRunID := ""
	lastWriteVersionMismatch := false
	actualLastWriteVersion := int64(constants.EmptyVersion)
	stateMismatch := false
	actualState := int(0)
	concreteExecutionAlreadyExists := false
	workflowRequestAlreadyExists := false
	workflowRequestID := ""
	requestRowType := int(0)
	var allPrevious []map[string]interface{}

	for {
		rowType, ok := previous["type"].(int)
		if !ok {
			// This should never happen, as all our rows have the type field.
			break
		}
		runID := previous["run_id"].(gocql.UUID).String()
		if rowType == rowTypeShard {
			if actualRangeID, ok = previous["range_id"].(int64); ok && actualRangeID != requestRangeID {
				// UpdateWorkflowExecution failed because rangeID was modified
				rangeIDMismatch = true
			}
		} else if rowType == rowTypeExecution && runID == permanentRunID {
			if currentWorkflowRequest.WriteMode == nosqlplugin.CurrentWorkflowWriteModeInsert {
				currentExecutionAlreadyExists = true
				actualExecution, _ = previous["execution"].(map[string]interface{})
				if actualExecution != nil {
					if previous["workflow_last_write_version"] != nil {
						actualLastWriteVersion = previous["workflow_last_write_version"].(int64)
					}
				}
			} else if currentWorkflowRequest.WriteMode == nosqlplugin.CurrentWorkflowWriteModeUpdate {
				if actualCurrRunID = previous["current_run_id"].(gocql.UUID).String(); requestConditionalRunID != "" && actualCurrRunID != requestConditionalRunID {
					runIDMismatch = true
				}
				if currentWorkflowRequest.Condition != nil && currentWorkflowRequest.Condition.LastWriteVersion != nil {
					ok := false
					if actualLastWriteVersion, ok = previous["workflow_last_write_version"].(int64); ok && *currentWorkflowRequest.Condition.LastWriteVersion != actualLastWriteVersion {
						lastWriteVersionMismatch = true
					}
				}
				if currentWorkflowRequest.Condition != nil && currentWorkflowRequest.Condition.State != nil {
					ok := false
					if actualState, ok = previous["workflow_state"].(int); ok && *currentWorkflowRequest.Condition.State != actualState {
						stateMismatch = true
					}
				}
			}
		} else if rowType == rowTypeExecution && execution.RunID == runID {
			concreteExecutionAlreadyExists = true
			if previous["workflow_last_write_version"] != nil {
				actualLastWriteVersion = previous["workflow_last_write_version"].(int64)
			}
		} else if isRequestRowType(rowType) {
			workflowRequestAlreadyExists = true
			workflowRequestID = runID
			requestRowType = rowType
		}

		allPrevious = append(allPrevious, previous)
		previous = make(map[string]interface{})
		if !iter.MapScan(previous) {
			// Cassandra returns the actual row that caused a condition failure, so we should always return
			// from the checks above, but just in case.
			break
		}
	}

	if rangeIDMismatch {
		return &nosqlplugin.WorkflowOperationConditionFailure{
			ShardRangeIDNotMatch: common.Int64Ptr(actualRangeID),
		}
	}
	if workflowRequestAlreadyExists {
		result := make(map[string]interface{})
		if err := session.Query(
			templateGetLatestWorkflowRequestQuery,
			shardCondition.ShardID,
			requestRowType,
			execution.DomainID,
			execution.WorkflowID,
			workflowRequestID,
			defaultVisibilityTimestamp,
		).WithContext(ctx).MapScan(result); err != nil {
			return err
		}
		runID, ok := result["current_run_id"].(gocql.UUID)
		if !ok {
			return fmt.Errorf("corrupted data detected. DomainID: %v, WorkflowId: %v, RequestID: %v, RequestType: %v", execution.DomainID, execution.WorkflowID, workflowRequestID, requestRowType)
		}
		requestType, err := fromRequestRowType(requestRowType)
		if err != nil {
			return err
		}
		return &nosqlplugin.WorkflowOperationConditionFailure{
			DuplicateRequest: &nosqlplugin.DuplicateRequest{
				RequestType: requestType,
				RunID:       runID.String(),
			},
		}
	}
	// CreateWorkflowExecution failed because there is already a current execution record for this workflow
	if currentExecutionAlreadyExists {
		if actualExecution != nil {
			executionInfo := parseWorkflowExecutionInfo(actualExecution)
			msg := fmt.Sprintf("Workflow execution already running. WorkflowId: %v, RunId: %v", currentWorkflowRequest.Row.WorkflowID, executionInfo.RunID)
			return &nosqlplugin.WorkflowOperationConditionFailure{
				WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
					OtherInfo:        msg,
					CreateRequestID:  executionInfo.CreateRequestID,
					RunID:            executionInfo.RunID,
					State:            executionInfo.State,
					CloseStatus:      executionInfo.CloseStatus,
					LastWriteVersion: actualLastWriteVersion,
				},
			}
		}
		msg := fmt.Sprintf("Workflow execution already running. WorkflowId: %v", currentWorkflowRequest.Row.WorkflowID)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			CurrentWorkflowConditionFailInfo: &msg,
		}
	}
	if runIDMismatch {
		msg := fmt.Sprintf("Workflow execution creation condition failed by mismatch runID. WorkflowId: %v, Expected Current RunID: %v, Actual Current RunID: %v",
			currentWorkflowRequest.Row.WorkflowID, currentWorkflowRequest.Condition.GetCurrentRunID(), actualCurrRunID)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			CurrentWorkflowConditionFailInfo: &msg,
		}
	}
	if lastWriteVersionMismatch {
		msg := fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, Expected Version: %v, Actual Version: %v",
			currentWorkflowRequest.Row.WorkflowID, *currentWorkflowRequest.Condition.LastWriteVersion, actualLastWriteVersion)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			CurrentWorkflowConditionFailInfo: &msg,
		}
	}
	if stateMismatch {
		msg := fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, Expected State: %v, Actual State: %v",
			currentWorkflowRequest.Row.WorkflowID, *currentWorkflowRequest.Condition.State, actualState)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			CurrentWorkflowConditionFailInfo: &msg,
		}
	}
	if concreteExecutionAlreadyExists {
		msg := fmt.Sprintf("Workflow execution already running. WorkflowId: %v, RunId: %v", execution.WorkflowID, execution.RunID)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
				OtherInfo:        msg,
				CreateRequestID:  execution.CreateRequestID,
				RunID:            execution.RunID,
				State:            execution.State,
				CloseStatus:      execution.CloseStatus,
				LastWriteVersion: actualLastWriteVersion,
			},
		}
	}

	// At this point we only know that the write was not applied.
	var columns []string
	columnID := 0
	for _, previous := range allPrevious {
		for k, v := range previous {
			columns = append(columns, fmt.Sprintf("%v: %s=%v", columnID, k, v))
		}
		columnID++
	}
	return newUnknownConditionFailureReason(shardCondition.RangeID, columns)
}

func executeUpdateWorkflowBatchTransaction(
	ctx context.Context,
	session gocql.Session,
	batch gocql.Batch,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	previousNextEventIDCondition int64,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	previous := make(map[string]interface{})
	applied, iter, err := session.MapExecuteBatchCAS(batch, previous)
	defer func() {
		if iter != nil {
			_ = iter.Close()
		}
	}()

	if err != nil {
		return err
	}

	if applied {
		return nil
	}

	requestRunID := currentWorkflowRequest.Row.RunID
	requestRangeID := shardCondition.RangeID
	requestConditionalRunID := ""
	if currentWorkflowRequest.Condition != nil {
		requestConditionalRunID = currentWorkflowRequest.Condition.GetCurrentRunID()
	}

	// There can be three reasons why the query does not get applied: the RangeID has changed, or the next_event_id or current_run_id check failed.
	// Check the row info returned by Cassandra to figure out which one it is.
	rangeIDMismatch := false
	actualRangeID := int64(0)
	nextEventIDMismatch := false
	actualNextEventID := int64(0)
	runIDMismatch := false
	actualCurrRunID := ""
	workflowRequestAlreadyExists := false
	workflowRequestID := ""
	requestRowType := int(0)
	var allPrevious []map[string]interface{}

	for {
		rowType, ok := previous["type"].(int)
		if !ok {
			// This should never happen, as all our rows have the type field.
			break
		}

		runID := previous["run_id"].(gocql.UUID).String()

		if rowType == rowTypeShard {
			if actualRangeID, ok = previous["range_id"].(int64); ok && actualRangeID != requestRangeID {
				// UpdateWorkflowExecution failed because rangeID was modified
				rangeIDMismatch = true
			}
		} else if rowType == rowTypeExecution && runID == requestRunID {
			if actualNextEventID, ok = previous["next_event_id"].(int64); ok && actualNextEventID != previousNextEventIDCondition {
				// UpdateWorkflowExecution failed because next event ID is unexpected
				nextEventIDMismatch = true
			}
		} else if rowType == rowTypeExecution && runID == permanentRunID {
			// UpdateWorkflowExecution failed because current_run_id is unexpected
			if actualCurrRunID = previous["current_run_id"].(gocql.UUID).String(); requestConditionalRunID != "" && actualCurrRunID != requestConditionalRunID {
				// UpdateWorkflowExecution failed because next event ID is unexpected
				runIDMismatch = true
			}
		} else if isRequestRowType(rowType) {
			workflowRequestAlreadyExists = true
			workflowRequestID = runID
			requestRowType = rowType
		}

		allPrevious = append(allPrevious, previous)
		previous = make(map[string]interface{})
		if !iter.MapScan(previous) {
			// Cassandra returns the actual row that caused a condition failure, so we should always return
			// from the checks above, but just in case.
			break
		}
	}

	if rangeIDMismatch {
		return &nosqlplugin.WorkflowOperationConditionFailure{
			ShardRangeIDNotMatch: common.Int64Ptr(actualRangeID),
		}
	}

	if workflowRequestAlreadyExists {
		result := make(map[string]interface{})
		if err := session.Query(
			templateGetLatestWorkflowRequestQuery,
			shardCondition.ShardID,
			requestRowType,
			currentWorkflowRequest.Row.DomainID,
			currentWorkflowRequest.Row.WorkflowID,
			workflowRequestID,
			defaultVisibilityTimestamp,
		).WithContext(ctx).MapScan(result); err != nil {
			return err
		}
		runID, ok := result["current_run_id"].(gocql.UUID)
		if !ok {
			return fmt.Errorf("corrupted data detected. DomainID: %v, WorkflowId: %v, RequestID: %v, RequestType: %v", currentWorkflowRequest.Row.DomainID, currentWorkflowRequest.Row.WorkflowID, workflowRequestID, requestRowType)
		}
		requestType, err := fromRequestRowType(requestRowType)
		if err != nil {
			return err
		}
		return &nosqlplugin.WorkflowOperationConditionFailure{
			DuplicateRequest: &nosqlplugin.DuplicateRequest{
				RequestType: requestType,
				RunID:       runID.String(),
			},
		}
	}

	if runIDMismatch {
		msg := fmt.Sprintf("Failed to update mutable state. requestConditionalRunID: %v, Actual Value: %v",
			requestConditionalRunID, actualCurrRunID)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			CurrentWorkflowConditionFailInfo: &msg,
		}
	}

	if nextEventIDMismatch {
		msg := fmt.Sprintf("Failed to update mutable state. previousNextEventIDCondition: %v, actualNextEventID: %v, Request Current RunID: %v",
			previousNextEventIDCondition, actualNextEventID, requestRunID)
		return &nosqlplugin.WorkflowOperationConditionFailure{
			UnknownConditionFailureDetails: &msg,
		}
	}

	// At this point we only know that the write was not applied.
	var columns []string
	columnID := 0
	for _, previous := range allPrevious {
		for k, v := range previous {
			columns = append(columns, fmt.Sprintf("%v: %s=%v", columnID, k, v))
		}
		columnID++
	}

	msg := fmt.Sprintf("Failed to update mutable state. ShardID: %v, RangeID: %v, previousNextEventIDCondition: %v, requestConditionalRunID: %v, columns: (%v)",
		shardCondition.ShardID, requestRangeID, previousNextEventIDCondition, requestConditionalRunID, strings.Join(columns, ","))
	return &nosqlplugin.WorkflowOperationConditionFailure{
		UnknownConditionFailureDetails: &msg,
	}
}

func newUnknownConditionFailureReason(
	rangeID int64,
	columns []string,
) *nosqlplugin.WorkflowOperationConditionFailure {
	msg := fmt.Sprintf("Failed to operate on workflow execution.  Request RangeID: %v, columns: (%v)",
		rangeID, strings.Join(columns, ","))

	return &nosqlplugin.WorkflowOperationConditionFailure{
		UnknownConditionFailureDetails: &msg,
	}
}

func assertShardRangeID(batch gocql.Batch, shardID int, rangeID int64, timeStamp time.Time) {
	batch.Query(templateUpdateLeaseQuery,
		rangeID,
		timeStamp,
		shardID,
		rowTypeShard,
		rowTypeShardDomainID,
		rowTypeShardWorkflowID,
		rowTypeShardRunID,
		defaultVisibilityTimestamp,
		rowTypeShardTaskID,
		rangeID,
	)
}

func createTasksByCategory(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	timeStamp time.Time,
	tasksByCategory map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask,
) {
	for c, tasks := range tasksByCategory {
		switch c.ID() {
		case persistence.HistoryTaskCategoryIDTransfer:
			createTransferTasks(batch, shardID, domainID, workflowID, tasks, timeStamp)
		case persistence.HistoryTaskCategoryIDTimer:
			createTimerTasks(batch, shardID, domainID, workflowID, tasks, timeStamp)
		case persistence.HistoryTaskCategoryIDReplication:
			createReplicationTasks(batch, shardID, domainID, workflowID, tasks, timeStamp)
		}
	}

	// TODO: implementing writing tasks for other categories
}

func createTimerTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	timerTasks []*nosqlplugin.HistoryMigrationTask,
	timeStamp time.Time,
) error {
	for _, timer := range timerTasks {
		task := timer.Timer
		taskBlob, taskEncoding := persistence.FromDataBlob(timer.Task)
		// Ignoring possible type cast errors.
		ts := persistence.UnixNanoToDBTimestamp(task.VisibilityTimestamp.UnixNano())

		batch.Query(templateCreateTimerTaskQuery,
			shardID,
			rowTypeTimerTask,
			rowTypeTimerDomainID,
			rowTypeTimerWorkflowID,
			rowTypeTimerRunID,
			domainID,
			workflowID,
			task.RunID,
			ts,
			task.TaskID,
			task.TaskType,
			task.TimeoutType,
			task.EventID,
			task.ScheduleAttempt,
			task.Version,
			taskBlob,
			taskEncoding,
			ts,
			task.TaskID,
			timeStamp,
		)
	}
	return nil
}

func createReplicationTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	replicationTasks []*nosqlplugin.HistoryMigrationTask,
	timeStamp time.Time,
) {
	for _, replication := range replicationTasks {
		task := replication.Replication
		taskBlob, taskEncoding := persistence.FromDataBlob(replication.Task)
		batch.Query(templateCreateReplicationTaskQuery,
			shardID,
			rowTypeReplicationTask,
			rowTypeReplicationDomainID,
			rowTypeReplicationWorkflowID,
			rowTypeReplicationRunID,
			domainID,
			workflowID,
			task.RunID,
			task.TaskID,
			task.TaskType,
			task.FirstEventID,
			task.NextEventID,
			task.Version,
			task.ScheduledID,
			persistence.EventStoreVersion,
			task.BranchToken,
			persistence.EventStoreVersion,
			task.NewRunBranchToken,
			task.CreationTime.UnixNano(),
			taskBlob,
			taskEncoding,
			// NOTE: use a constant here instead of task.VisibilityTimestamp so that we can query tasks with the same visibilityTimestamp
			defaultVisibilityTimestamp,
			task.TaskID,
			timeStamp,
		)
	}
}

func createTransferTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	transferTasks []*nosqlplugin.HistoryMigrationTask,
	timeStamp time.Time,
) {
	for _, transfer := range transferTasks {
		task := transfer.Transfer
		taskBlob, taskEncoding := persistence.FromDataBlob(transfer.Task)
		batch.Query(templateCreateTransferTaskQuery,
			shardID,
			rowTypeTransferTask,
			rowTypeTransferDomainID,
			rowTypeTransferWorkflowID,
			rowTypeTransferRunID,
			domainID,
			workflowID,
			task.RunID,
			task.VisibilityTimestamp,
			task.TaskID,
			task.TargetDomainID,
			task.TargetDomainIDs,
			task.TargetWorkflowID,
			task.TargetRunID,
			task.TargetChildWorkflowOnly,
			task.TaskList,
			task.TaskType,
			task.ScheduleID,
			task.RecordVisibility,
			task.Version,
			taskBlob,
			taskEncoding,
			// NOTE: use a constant here instead of task.VisibilityTimestamp so that we can query tasks with the same visibilityTimestamp
			defaultVisibilityTimestamp,
			task.TaskID,
			timeStamp,
		)
	}
}

func resetSignalsRequested(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalReqIDs []string,
	timeStamp time.Time,
) error {
	batch.Query(templateResetSignalRequestedQuery,
		signalReqIDs,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func updateSignalsRequested(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalReqIDs []string,
	deleteSignalReqIDs []string,
	timeStamp time.Time,
) error {
	if len(signalReqIDs) > 0 {
		batch.Query(templateUpdateSignalRequestedQuery,
			signalReqIDs,
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	if len(deleteSignalReqIDs) > 0 {
		batch.Query(templateDeleteWorkflowExecutionSignalRequestedQuery,
			deleteSignalReqIDs,
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

func resetSignalInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalInfos map[int64]*persistence.SignalInfo,
	timeStamp time.Time,
) error {
	batch.Query(templateResetSignalInfoQuery,
		resetSignalInfoMap(signalInfos),
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetSignalInfoMap(signalInfos map[int64]*persistence.SignalInfo) map[int64]map[string]interface{} {
	sMap := make(map[int64]map[string]interface{})
	for _, s := range signalInfos {
		sInfo := make(map[string]interface{})
		sInfo["version"] = s.Version
		sInfo["initiated_id"] = s.InitiatedID
		sInfo["initiated_event_batch_id"] = s.InitiatedEventBatchID
		sInfo["signal_request_id"] = s.SignalRequestID
		sInfo["signal_name"] = s.SignalName
		sInfo["input"] = s.Input
		sInfo["control"] = s.Control
		sMap[s.InitiatedID] = sInfo
	}

	return sMap
}

func updateSignalInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalInfos map[int64]*persistence.SignalInfo,
	deleteInfos []int64,
	timeStamp time.Time,
) error {
	for _, c := range signalInfos {
		batch.Query(templateUpdateSignalInfoQuery,
			c.InitiatedID,
			c.Version,
			c.InitiatedID,
			c.InitiatedEventBatchID,
			c.SignalRequestID,
			c.SignalName,
			c.Input,
			c.Control,
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	// deleteInfos are the initiatedIDs for SignalInfo being deleted
	for _, deleteInfo := range deleteInfos {
		batch.Query(templateDeleteSignalInfoQuery,
			deleteInfo,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

func resetRequestCancelInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	requestCancelInfos map[int64]*persistence.RequestCancelInfo,
	timeStamp time.Time,
) error {
	batch.Query(templateResetRequestCancelInfoQuery,
		resetRequestCancelInfoMap(requestCancelInfos),
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetRequestCancelInfoMap(requestCancelInfos map[int64]*persistence.RequestCancelInfo) map[int64]map[string]interface{} {
	rcMap := make(map[int64]map[string]interface{})
	for _, rc := range requestCancelInfos {
		rcInfo := make(map[string]interface{})
		rcInfo["version"] = rc.Version
		rcInfo["initiated_id"] = rc.InitiatedID
		rcInfo["initiated_event_batch_id"] = rc.InitiatedEventBatchID
		rcInfo["cancel_request_id"] = rc.CancelRequestID
		rcMap[rc.InitiatedID] = rcInfo
	}

	return rcMap
}

func updateRequestCancelInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	requestCancelInfos map[int64]*persistence.RequestCancelInfo,
	deleteInfos []int64,
	timeStamp time.Time,
) error {
	for _, c := range requestCancelInfos {
		batch.Query(templateUpdateRequestCancelInfoQuery,
			c.InitiatedID,
			c.Version,
			c.InitiatedID,
			c.InitiatedEventBatchID,
			c.CancelRequestID,
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	// deleteInfos are the initiatedIDs for RequestCancelInfo being deleted
	for _, deleteInfo := range deleteInfos {
		batch.Query(templateDeleteRequestCancelInfoQuery,
			deleteInfo,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

func resetChildExecutionInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo,
	timeStamp time.Time,
) error {
	infoMap, err := resetChildExecutionInfoMap(childExecutionInfos)
	if err != nil {
		return err
	}
	batch.Query(templateResetChildExecutionInfoQuery,
		infoMap,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetChildExecutionInfoMap(childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo) (map[int64]map[string]interface{}, error) {
	cMap := make(map[int64]map[string]interface{})
	for _, c := range childExecutionInfos {
		cInfo := make(map[string]interface{})
		cInfo["version"] = c.Version
		cInfo["event_data_encoding"] = c.InitiatedEvent.GetEncodingString()
		cInfo["initiated_id"] = c.InitiatedID
		cInfo["initiated_event_batch_id"] = c.InitiatedEventBatchID
		cInfo["initiated_event"] = c.InitiatedEvent.Data
		cInfo["started_id"] = c.StartedID
		cInfo["started_event"] = c.StartedEvent.Data
		cInfo["create_request_id"] = c.CreateRequestID
		cInfo["started_workflow_id"] = c.StartedWorkflowID
		startedRunID := emptyRunID
		if c.StartedRunID != "" {
			startedRunID = c.StartedRunID
		}
		cInfo["started_run_id"] = startedRunID
		cInfo["domain_id"] = c.DomainID
		cInfo["domain_name"] = c.DomainNameDEPRECATED
		cInfo["workflow_type_name"] = c.WorkflowTypeName
		cInfo["parent_close_policy"] = int32(c.ParentClosePolicy)

		cMap[c.InitiatedID] = cInfo
	}

	return cMap, nil
}

func updateChildExecutionInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo,
	deleteInfos []int64,
	timeStamp time.Time,
) error {
	for _, c := range childExecutionInfos {
		batch.Query(templateUpdateChildExecutionInfoQuery,
			c.InitiatedID,
			c.Version,
			c.InitiatedID,
			c.InitiatedEventBatchID,
			c.InitiatedEvent.Data,
			c.StartedID,
			c.StartedWorkflowID,
			c.StartedRunID,
			c.StartedEvent.Data,
			c.CreateRequestID,
			c.InitiatedEvent.GetEncodingString(),
			c.DomainID,
			c.DomainNameDEPRECATED,
			c.WorkflowTypeName,
			int32(c.ParentClosePolicy),
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	// deleteInfos are the initiatedIDs for ChildInfo being deleted
	for _, deleteInfo := range deleteInfos {
		batch.Query(templateDeleteChildExecutionInfoQuery,
			deleteInfo,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

func resetTimerInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timerInfos map[string]*persistence.TimerInfo,
	timeStamp time.Time,
) error {
	batch.Query(templateResetTimerInfoQuery,
		resetTimerInfoMap(timerInfos),
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetTimerInfoMap(timerInfos map[string]*persistence.TimerInfo) map[string]map[string]interface{} {
	tMap := make(map[string]map[string]interface{})
	for _, t := range timerInfos {
		tInfo := make(map[string]interface{})
		tInfo["version"] = t.Version
		tInfo["timer_id"] = t.TimerID
		tInfo["started_id"] = t.StartedID
		tInfo["expiry_time"] = t.ExpiryTime
		// task_id is a misleading variable, it actually serves
		// the purpose of indicating whether a timer task is
		// generated for this timer info
		tInfo["task_id"] = t.TaskStatus

		tMap[t.TimerID] = tInfo
	}

	return tMap
}

func updateTimerInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timerInfos map[string]*persistence.TimerInfo,
	deleteInfos []string,
	timeStamp time.Time,
) error {
	for _, timerInfo := range timerInfos {
		batch.Query(templateUpdateTimerInfoQuery,
			timerInfo.TimerID,
			timerInfo.Version,
			timerInfo.TimerID,
			timerInfo.StartedID,
			timerInfo.ExpiryTime,
			timerInfo.TaskStatus,
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	for _, deleteInfo := range deleteInfos {
		batch.Query(templateDeleteTimerInfoQuery,
			deleteInfo,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

func resetActivityInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	activityInfos map[int64]*persistence.InternalActivityInfo,
	timeStamp time.Time,
) error {
	infoMap, err := resetActivityInfoMap(activityInfos)
	if err != nil {
		return err
	}

	batch.Query(templateResetActivityInfoQuery,
		infoMap,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetActivityInfoMap(activityInfos map[int64]*persistence.InternalActivityInfo) (map[int64]map[string]interface{}, error) {
	aMap := make(map[int64]map[string]interface{})
	for _, a := range activityInfos {
		aInfo := make(map[string]interface{})
		aInfo["version"] = a.Version
		aInfo["event_data_encoding"] = a.ScheduledEvent.GetEncodingString()
		aInfo["schedule_id"] = a.ScheduleID
		aInfo["scheduled_event_batch_id"] = a.ScheduledEventBatchID
		aInfo["scheduled_event"] = a.ScheduledEvent.Data
		aInfo["scheduled_time"] = a.ScheduledTime
		aInfo["started_id"] = a.StartedID
		aInfo["started_event"] = a.StartedEvent.Data
		aInfo["started_time"] = a.StartedTime
		aInfo["activity_id"] = a.ActivityID
		aInfo["request_id"] = a.RequestID
		aInfo["details"] = a.Details
		aInfo["schedule_to_start_timeout"] = int32(a.ScheduleToStartTimeout.Seconds())
		aInfo["schedule_to_close_timeout"] = int32(a.ScheduleToCloseTimeout.Seconds())
		aInfo["start_to_close_timeout"] = int32(a.StartToCloseTimeout.Seconds())
		aInfo["heart_beat_timeout"] = int32(a.HeartbeatTimeout.Seconds())
		aInfo["cancel_requested"] = a.CancelRequested
		aInfo["cancel_request_id"] = a.CancelRequestID
		aInfo["last_hb_updated_time"] = a.LastHeartBeatUpdatedTime
		aInfo["timer_task_status"] = a.TimerTaskStatus
		aInfo["attempt"] = a.Attempt
		aInfo["task_list"] = a.TaskList
		aInfo["started_identity"] = a.StartedIdentity
		aInfo["has_retry_policy"] = a.HasRetryPolicy
		aInfo["init_interval"] = int32(a.InitialInterval.Seconds())
		aInfo["backoff_coefficient"] = a.BackoffCoefficient
		aInfo["max_interval"] = int32(a.MaximumInterval.Seconds())
		aInfo["expiration_time"] = a.ExpirationTime
		aInfo["max_attempts"] = a.MaximumAttempts
		aInfo["non_retriable_errors"] = a.NonRetriableErrors
		aInfo["last_failure_reason"] = a.LastFailureReason
		aInfo["last_worker_identity"] = a.LastWorkerIdentity
		aInfo["last_failure_details"] = a.LastFailureDetails

		aMap[a.ScheduleID] = aInfo
	}

	return aMap, nil
}

func updateActivityInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	activityInfos map[int64]*persistence.InternalActivityInfo,
	deleteInfos []int64,
	timeStamp time.Time,
) error {
	for _, a := range activityInfos {
		batch.Query(templateUpdateActivityInfoQuery,
			a.ScheduleID,
			a.Version,
			a.ScheduleID,
			a.ScheduledEventBatchID,
			a.ScheduledEvent.Data,
			a.ScheduledTime,
			a.StartedID,
			a.StartedEvent.Data,
			a.StartedTime,
			a.ActivityID,
			a.RequestID,
			a.Details,
			int32(a.ScheduleToStartTimeout.Seconds()),
			int32(a.ScheduleToCloseTimeout.Seconds()),
			int32(a.StartToCloseTimeout.Seconds()),
			int32(a.HeartbeatTimeout.Seconds()),
			a.CancelRequested,
			a.CancelRequestID,
			a.LastHeartBeatUpdatedTime,
			a.TimerTaskStatus,
			a.Attempt,
			a.TaskList,
			int32(a.TaskListKind),
			a.StartedIdentity,
			a.HasRetryPolicy,
			int32(a.InitialInterval.Seconds()),
			a.BackoffCoefficient,
			int32(a.MaximumInterval.Seconds()),
			a.ExpirationTime,
			a.MaximumAttempts,
			a.NonRetriableErrors,
			a.LastFailureReason,
			a.LastWorkerIdentity,
			a.LastFailureDetails,
			a.ScheduledEvent.GetEncodingString(),
			timeStamp,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}

	for _, deleteInfo := range deleteInfos {
		batch.Query(templateDeleteActivityInfoQuery,
			deleteInfo,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			runID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID)
	}
	return nil
}

// NOTE: not sure we still need it. We keep the behavior for safe during refactoring
// In theory we can just return the input as output
// TODO: if possible, remove it in the future or add more comment of why we need this conversion
func convertToCassandraTimestamp(in time.Time) time.Time {
	return time.Unix(0, persistence.DBTimestampToUnixNano(persistence.UnixNanoToDBTimestamp(in.UnixNano())))
}

func getNextPageToken(iter gocql.Iter) []byte {
	nextPageToken := iter.PageState()
	newPageToken := make([]byte, len(nextPageToken))
	copy(newPageToken, nextPageToken)
	return newPageToken
}

func createWorkflowExecutionWithMergeMaps(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
	timeStamp time.Time,
) error {
	err := createWorkflowExecution(batch, shardID, domainID, workflowID, execution, timeStamp)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode != nosqlplugin.EventBufferWriteModeNone {
		return fmt.Errorf("should only support EventBufferWriteModeNone")
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeCreate {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeCreate")
	}

	err = updateActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos, nil, timeStamp)
	if err != nil {
		return err
	}
	err = updateTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos, nil, timeStamp)
	if err != nil {
		return err
	}
	err = updateChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos, nil, timeStamp)
	if err != nil {
		return err
	}
	err = updateRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos, nil, timeStamp)
	if err != nil {
		return err
	}
	err = updateSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos, nil, timeStamp)
	if err != nil {
		return err
	}
	return updateSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs, nil, timeStamp)
}

func resetWorkflowExecutionAndMapsAndEventBuffer(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
	timeStamp time.Time,
) error {
	err := updateWorkflowExecution(batch, shardID, domainID, workflowID, execution, timeStamp)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode != nosqlplugin.EventBufferWriteModeClear {
		return fmt.Errorf("should only support EventBufferWriteModeClear")
	}
	err = deleteBufferedEvents(batch, shardID, domainID, workflowID, execution.RunID, timeStamp)
	if err != nil {
		return err
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeReset {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeReset")
	}

	// This is another category of execution update where only the non-frozen column types in
	// Cassandra are updated to a previous state in the Execution Update flow.
	err = resetActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos, timeStamp)
	if err != nil {
		return err
	}
	err = resetTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos, timeStamp)
	if err != nil {
		return err
	}
	err = resetChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos, timeStamp)
	if err != nil {
		return err
	}
	err = resetRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos, timeStamp)
	if err != nil {
		return err
	}
	err = resetSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos, timeStamp)
	if err != nil {
		return err
	}
	return resetSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs, timeStamp)
}

func appendBufferedEvents(
	batch gocql.Batch,
	newBufferedEvents *persistence.DataBlob,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timeStamp time.Time,
) error {
	values := make(map[string]interface{})
	values["encoding_type"] = newBufferedEvents.Encoding
	values["version"] = int64(0)
	values["data"] = newBufferedEvents.Data
	newEventValues := []map[string]interface{}{values}
	batch.Query(templateAppendBufferedEventsQuery,
		newEventValues,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func deleteBufferedEvents(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timeStamp time.Time,
) error {
	batch.Query(templateDeleteBufferedEventsQuery,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
	timeStamp time.Time,
) error {
	err := updateWorkflowExecution(batch, shardID, domainID, workflowID, execution, timeStamp)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode == nosqlplugin.EventBufferWriteModeClear {
		err = deleteBufferedEvents(batch, shardID, domainID, workflowID, execution.RunID, timeStamp)
		if err != nil {
			return err
		}
	} else if execution.EventBufferWriteMode == nosqlplugin.EventBufferWriteModeAppend {
		err = appendBufferedEvents(batch, execution.NewBufferedEventBatch, shardID, domainID, workflowID, execution.RunID, timeStamp)
		if err != nil {
			return err
		}
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeUpdate {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeUpdate")
	}

	// In certain cases, some of the execution update cycles update particular columns asynchronously before reaching the final cycle.
	// Each of these functions are updating a non-frozen column type in Cassandra table.
	err = updateActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos, execution.ActivityInfoKeysToDelete, timeStamp)
	if err != nil {
		return err
	}
	err = updateTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos, execution.TimerInfoKeysToDelete, timeStamp)
	if err != nil {
		return err
	}
	err = updateChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos, execution.ChildWorkflowInfoKeysToDelete, timeStamp)
	if err != nil {
		return err
	}
	err = updateRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos, execution.RequestCancelInfoKeysToDelete, timeStamp)
	if err != nil {
		return err
	}
	err = updateSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos, execution.SignalInfoKeysToDelete, timeStamp)
	if err != nil {
		return err
	}
	return updateSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs, execution.SignalRequestedIDsKeysToDelete, timeStamp)
}

// updateWorkflowExecution is responsible for updating the execution state in different cycles until the
// Status changes to close at the Final update cycle. Information is updated linearly, and synchronization usually occurs in every cycle.
func updateWorkflowExecution(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
	timeStamp time.Time,
) error {
	execution.StartTimestamp = convertToCassandraTimestamp(execution.StartTimestamp)
	execution.LastUpdatedTimestamp = convertToCassandraTimestamp(execution.LastUpdatedTimestamp)

	batch.Query(templateUpdateWorkflowExecutionWithVersionHistoriesQuery,
		domainID,
		workflowID,
		execution.RunID,
		execution.FirstExecutionRunID,
		execution.ParentDomainID,
		execution.ParentWorkflowID,
		execution.ParentRunID,
		execution.InitiatedID,
		execution.CompletionEventBatchID,
		execution.CompletionEvent.Data,
		execution.CompletionEvent.GetEncodingString(),
		execution.TaskList,
		int32(execution.TaskListKind),
		execution.WorkflowTypeName,
		int32(execution.WorkflowTimeout.Seconds()),
		int32(execution.DecisionStartToCloseTimeout.Seconds()),
		execution.ExecutionContext,
		execution.State,
		execution.CloseStatus,
		execution.LastFirstEventID,
		execution.LastEventTaskID,
		execution.NextEventID,
		execution.LastProcessedEvent,
		execution.StartTimestamp,
		execution.LastUpdatedTimestamp,
		execution.CreateRequestID,
		execution.SignalCount,
		execution.HistorySize,
		execution.DecisionVersion,
		execution.DecisionScheduleID,
		execution.DecisionStartedID,
		execution.DecisionRequestID,
		int32(execution.DecisionTimeout.Seconds()),
		execution.DecisionAttempt,
		execution.DecisionStartedTimestamp.UnixNano(),
		execution.DecisionScheduledTimestamp.UnixNano(),
		execution.DecisionOriginalScheduledTimestamp.UnixNano(),
		execution.CancelRequested,
		execution.CancelRequestID,
		execution.StickyTaskList,
		int32(execution.StickyScheduleToStartTimeout.Seconds()),
		execution.ClientLibraryVersion,
		execution.ClientFeatureVersion,
		execution.ClientImpl,
		execution.AutoResetPoints.Data,
		execution.AutoResetPoints.GetEncoding(),
		execution.Attempt,
		execution.HasRetryPolicy,
		int32(execution.InitialInterval.Seconds()),
		execution.BackoffCoefficient,
		int32(execution.MaximumInterval.Seconds()),
		execution.ExpirationTime,
		execution.MaximumAttempts,
		execution.NonRetriableErrors,
		persistence.EventStoreVersion,
		execution.BranchToken,
		execution.CronSchedule,
		int32(execution.CronOverlapPolicy),
		int32(execution.ExpirationInterval.Seconds()),
		execution.SearchAttributes,
		execution.Memo,
		execution.PartitionConfig,
		execution.ActiveClusterSelectionPolicy.GetData(),
		execution.ActiveClusterSelectionPolicy.GetEncodingString(),
		execution.NextEventID,
		execution.VersionHistories.Data,
		execution.VersionHistories.GetEncodingString(),
		execution.Checksums.Version,
		execution.Checksums.Flavor,
		execution.Checksums.Value,
		execution.LastWriteVersion,
		execution.State,
		timeStamp,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		execution.RunID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
		execution.PreviousNextEventIDCondition)

	return nil
}

func createWorkflowExecution(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
	timeStamp time.Time,
) error {
	execution.StartTimestamp = convertToCassandraTimestamp(execution.StartTimestamp)
	execution.LastUpdatedTimestamp = convertToCassandraTimestamp(execution.LastUpdatedTimestamp)

	batch.Query(templateCreateWorkflowExecutionWithVersionHistoriesQuery,
		shardID,
		domainID,
		workflowID,
		execution.RunID,
		rowTypeExecution,
		domainID,
		workflowID,
		execution.RunID,
		execution.FirstExecutionRunID,
		execution.ParentDomainID,
		execution.ParentWorkflowID,
		execution.ParentRunID,
		execution.InitiatedID,
		execution.CompletionEventBatchID,
		execution.CompletionEvent.Data,
		execution.CompletionEvent.GetEncodingString(),
		execution.TaskList,
		int32(execution.TaskListKind),
		execution.WorkflowTypeName,
		int32(execution.WorkflowTimeout.Seconds()),
		int32(execution.DecisionStartToCloseTimeout.Seconds()),
		execution.ExecutionContext,
		execution.State,
		execution.CloseStatus,
		execution.LastFirstEventID,
		execution.LastEventTaskID,
		execution.NextEventID,
		execution.LastProcessedEvent,
		execution.StartTimestamp,
		execution.LastUpdatedTimestamp,
		execution.CreateRequestID,
		execution.SignalCount,
		execution.HistorySize,
		execution.DecisionVersion,
		execution.DecisionScheduleID,
		execution.DecisionStartedID,
		execution.DecisionRequestID,
		int32(execution.DecisionTimeout.Seconds()),
		execution.DecisionAttempt,
		execution.DecisionStartedTimestamp.UnixNano(),
		execution.DecisionScheduledTimestamp.UnixNano(),
		execution.DecisionOriginalScheduledTimestamp.UnixNano(),
		execution.CancelRequested,
		execution.CancelRequestID,
		execution.StickyTaskList,
		int32(execution.StickyScheduleToStartTimeout.Seconds()),
		execution.ClientLibraryVersion,
		execution.ClientFeatureVersion,
		execution.ClientImpl,
		execution.AutoResetPoints.Data,
		execution.AutoResetPoints.GetEncodingString(),
		execution.Attempt,
		execution.HasRetryPolicy,
		int32(execution.InitialInterval.Seconds()),
		execution.BackoffCoefficient,
		int32(execution.MaximumInterval.Seconds()),
		execution.ExpirationTime,
		execution.MaximumAttempts,
		execution.NonRetriableErrors,
		persistence.EventStoreVersion,
		execution.BranchToken,
		execution.CronSchedule,
		int32(execution.CronOverlapPolicy),
		int32(execution.ExpirationInterval.Seconds()),
		execution.SearchAttributes,
		execution.Memo,
		execution.PartitionConfig,
		execution.ActiveClusterSelectionPolicy.GetData(),
		execution.ActiveClusterSelectionPolicy.GetEncodingString(),
		execution.NextEventID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
		execution.VersionHistories.Data,
		execution.VersionHistories.GetEncodingString(),
		execution.Checksums.Version,
		execution.Checksums.Flavor,
		execution.Checksums.Value,
		execution.LastWriteVersion,
		execution.State,
		timeStamp,
	)
	return nil
}

func isRequestRowType(rowType int) bool {
	if rowType >= rowTypeWorkflowRequestStart && rowType <= rowTypeWorkflowRequestReset {
		return true
	}
	return false
}

func toRequestRowType(requestType persistence.WorkflowRequestType) (int, error) {
	switch requestType {
	case persistence.WorkflowRequestTypeStart:
		return rowTypeWorkflowRequestStart, nil
	case persistence.WorkflowRequestTypeSignal:
		return rowTypeWorkflowRequestSignal, nil
	case persistence.WorkflowRequestTypeCancel:
		return rowTypeWorkflowRequestCancel, nil
	case persistence.WorkflowRequestTypeReset:
		return rowTypeWorkflowRequestReset, nil
	default:
		return 0, fmt.Errorf("unknown workflow request type %v", requestType)
	}
}

func fromRequestRowType(rowType int) (persistence.WorkflowRequestType, error) {
	switch rowType {
	case rowTypeWorkflowRequestStart:
		return persistence.WorkflowRequestTypeStart, nil
	case rowTypeWorkflowRequestSignal:
		return persistence.WorkflowRequestTypeSignal, nil
	case rowTypeWorkflowRequestCancel:
		return persistence.WorkflowRequestTypeCancel, nil
	case rowTypeWorkflowRequestReset:
		return persistence.WorkflowRequestTypeReset, nil
	default:
		return persistence.WorkflowRequestType(0), fmt.Errorf("unknown request row type %v", rowType)
	}
}

func insertWorkflowActiveClusterSelectionPolicyRow(
	batch gocql.Batch,
	activeClusterSelectionPolicyRow *nosqlplugin.ActiveClusterSelectionPolicyRow,
	timeStamp time.Time,
) error {
	if activeClusterSelectionPolicyRow == nil || activeClusterSelectionPolicyRow.Policy == nil {
		return nil
	}

	batch.Query(templateInsertWorkflowActiveClusterSelectionPolicyRowQuery,
		activeClusterSelectionPolicyRow.ShardID,
		rowTypeWorkflowActiveClusterSelectionPolicy,
		activeClusterSelectionPolicyRow.DomainID,
		activeClusterSelectionPolicyRow.WorkflowID,
		activeClusterSelectionPolicyRow.RunID,
		defaultVisibilityTimestamp,
		rowTypeWorkflowActiveClusterSelectionVersion,
		timeStamp,
		activeClusterSelectionPolicyRow.Policy.Data,
		activeClusterSelectionPolicyRow.Policy.GetEncodingString(),
	)
	return nil
}

func insertOrUpsertWorkflowRequestRow(
	batch gocql.Batch,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	timeStamp time.Time,
) error {
	if requests == nil {
		return nil
	}
	var insertQuery string
	switch requests.WriteMode {
	case nosqlplugin.WorkflowRequestWriteModeInsert:
		insertQuery = templateInsertWorkflowRequestQuery
	case nosqlplugin.WorkflowRequestWriteModeUpsert:
		insertQuery = templateUpsertWorkflowRequestQuery
	default:
		return fmt.Errorf("unknown workflow request write mode %v", requests.WriteMode)
	}
	for _, row := range requests.Rows {
		rowType, err := toRequestRowType(row.RequestType)
		if err != nil {
			return err
		}
		batch.Query(insertQuery,
			row.ShardID,
			rowType,
			row.DomainID,
			row.WorkflowID,
			row.RequestID,
			defaultVisibilityTimestamp,
			emptyWorkflowRequestVersion*-1,
			row.RunID,
			timeStamp,
			workflowRequestTTLInSeconds,
		)
		batch.Query(insertQuery,
			row.ShardID,
			rowType,
			row.DomainID,
			row.WorkflowID,
			row.RequestID,
			defaultVisibilityTimestamp,
			row.Version*-1,
			row.RunID,
			timeStamp,
			workflowRequestTTLInSeconds,
		)
	}
	return nil
}

func createOrUpdateCurrentWorkflow(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	request *nosqlplugin.CurrentWorkflowWriteRequest,
	timeStamp time.Time,
) error {
	switch request.WriteMode {
	case nosqlplugin.CurrentWorkflowWriteModeNoop:
		return nil
	case nosqlplugin.CurrentWorkflowWriteModeInsert:
		batch.Query(templateCreateCurrentWorkflowExecutionQuery,
			shardID,
			rowTypeExecution,
			domainID,
			workflowID,
			permanentRunID,
			defaultVisibilityTimestamp,
			rowTypeExecutionTaskID,
			request.Row.RunID,
			request.Row.RunID,
			request.Row.CreateRequestID,
			request.Row.State,
			request.Row.CloseStatus,
			request.Row.LastWriteVersion,
			request.Row.State,
			timeStamp,
		)
	case nosqlplugin.CurrentWorkflowWriteModeUpdate:
		if request.Condition == nil || request.Condition.GetCurrentRunID() == "" {
			return fmt.Errorf("CurrentWorkflowWriteModeUpdate require Condition.CurrentRunID")
		}
		if request.Condition.LastWriteVersion != nil && request.Condition.State != nil {
			batch.Query(templateUpdateCurrentWorkflowExecutionForNewQuery,
				request.Row.RunID,
				request.Row.RunID,
				request.Row.CreateRequestID,
				request.Row.State,
				request.Row.CloseStatus,
				request.Row.LastWriteVersion,
				request.Row.State,
				timeStamp,
				shardID,
				rowTypeExecution,
				domainID,
				workflowID,
				permanentRunID,
				defaultVisibilityTimestamp,
				rowTypeExecutionTaskID,
				*request.Condition.CurrentRunID,
				*request.Condition.LastWriteVersion,
				*request.Condition.State,
			)
		} else {
			batch.Query(templateUpdateCurrentWorkflowExecutionQuery,
				request.Row.RunID,
				request.Row.RunID,
				request.Row.CreateRequestID,
				request.Row.State,
				request.Row.CloseStatus,
				request.Row.LastWriteVersion,
				request.Row.State,
				timeStamp,
				shardID,
				rowTypeExecution,
				domainID,
				workflowID,
				permanentRunID,
				defaultVisibilityTimestamp,
				rowTypeExecutionTaskID,
				*request.Condition.CurrentRunID,
			)
		}
	default:
		return fmt.Errorf("unknown mode %v", request.WriteMode)
	}
	return nil
}

func mustConvertToSlice(value interface{}) []interface{} {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = v.Index(i).Interface()
		}
		return result
	default:
		panic(fmt.Sprintf("Unable to convert %v to slice which is of type %T", value, value))
	}
}

func populateGetReplicationTasks(query gocql.Query) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "populateGetReplicationTasks operation failed.  Not able to create query iterator.",
		}
	}

	var tasks []*nosqlplugin.HistoryMigrationTask
	task := make(map[string]interface{})
	for iter.MapScan(task) {
		t := parseReplicationTaskInfo(task["replication"].(map[string]interface{}))
		taskID := task["task_id"].(int64)
		data := task["data"].([]byte)
		encoding := task["data_encoding"].(string)
		taskBlob := persistence.NewDataBlob(data, constants.EncodingType(encoding))
		// Reset task map to get it ready for next scan
		task = make(map[string]interface{})

		tasks = append(tasks, &nosqlplugin.HistoryMigrationTask{
			Replication: t,
			Task:        taskBlob,
			TaskID:      taskID,
		})
	}
	nextPageToken := getNextPageToken(iter)
	err := iter.Close()

	return tasks, nextPageToken, err
}
