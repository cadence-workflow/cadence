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
	"fmt"
	"strings"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func (db *cdb) executeCreateWorkflowBatchTransaction(
	batch gocql.Batch,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	execution *nosqlplugin.WorkflowExecutionRequest,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	previous := make(map[string]interface{})
	applied, iter, err := db.session.MapExecuteBatchCAS(batch, previous)
	defer func() {
		if iter != nil {
			_ = iter.Close()
		}
	}()

	if err != nil {
		return err
	}

	if !applied {
		// There can be two reasons why the query does not get applied. Either the RangeID has changed, or
		// the workflow is already started. Check the row info returned by Cassandra to figure out which one it is.
	GetFailureReasonLoop:
		for {
			rowType, ok := previous["type"].(int)
			if !ok {
				// This should never happen, as all our rows have the type field.
				break GetFailureReasonLoop
			}
			runID := previous["run_id"].(gocql.UUID).String()

			if rowType == rowTypeShard {
				if rangeID, ok := previous["range_id"].(int64); ok && rangeID != shardCondition.RangeID {
					// CreateWorkflowExecution failed because rangeID was modified
					return &nosqlplugin.WorkflowOperationConditionFailure{
						ShardRangeIDNotMatch: common.Int64Ptr(rangeID),
					}
				}

			} else if rowType == rowTypeExecution && runID == permanentRunID {
				var columns []string
				for k, v := range previous {
					columns = append(columns, fmt.Sprintf("%s=%v", k, v))
				}

				if execution, ok := previous["execution"].(map[string]interface{}); ok {
					// CreateWorkflowExecution failed because it already exists
					executionInfo := createWorkflowExecutionInfo(execution)
					lastWriteVersion := common.EmptyVersion
					if previous["workflow_last_write_version"] != nil {
						lastWriteVersion = previous["workflow_last_write_version"].(int64)
					}

					msg := fmt.Sprintf("Workflow execution already running. WorkflowId: %v, RunId: %v, rangeID: %v, columns: (%v)",
						executionInfo.WorkflowID, executionInfo.RunID, shardCondition.RangeID, strings.Join(columns, ","))

					if currentWorkflowRequest.WriteMode == nosqlplugin.CurrentWorkflowWriteModeInsert {
						return &nosqlplugin.WorkflowOperationConditionFailure{
							WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
								OtherInfo:        msg,
								CreateRequestID:  executionInfo.CreateRequestID,
								RunID:            executionInfo.RunID,
								State:            executionInfo.State,
								CloseStatus:      executionInfo.CloseStatus,
								LastWriteVersion: lastWriteVersion,
							},
						}
					}
					return &nosqlplugin.WorkflowOperationConditionFailure{
						CurrentWorkflowConditionFailInfo: &msg,
					}

				}

				if prevRunID := previous["current_run_id"].(gocql.UUID).String(); prevRunID != currentWorkflowRequest.Condition.GetCurrentRunID() {
					// currentRunID on previous run has been changed, return to caller to handle
					msg := fmt.Sprintf("Workflow execution creation condition failed by mismatch runID. WorkflowId: %v, Expected Current RunID: %v, Actual Current RunID: %v",
						execution.WorkflowID, currentWorkflowRequest.Condition.GetCurrentRunID(), prevRunID)
					return &nosqlplugin.WorkflowOperationConditionFailure{
						CurrentWorkflowConditionFailInfo: &msg,
					}
				}

				msg := fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, CurrentRunID: %v, columns: (%v)",
					execution.WorkflowID, execution.RunID, strings.Join(columns, ","))
				return &nosqlplugin.WorkflowOperationConditionFailure{
					CurrentWorkflowConditionFailInfo: &msg,
				}
			} else if rowType == rowTypeExecution && execution.RunID == runID {
				msg := fmt.Sprintf("Workflow execution already running. WorkflowId: %v, RunId: %v, rangeID: %v",
					execution.WorkflowID, execution.RunID, shardCondition.RangeID)
				lastWriteVersion := common.EmptyVersion
				if previous["workflow_last_write_version"] != nil {
					lastWriteVersion = previous["workflow_last_write_version"].(int64)
				}
				return &nosqlplugin.WorkflowOperationConditionFailure{
					WorkflowExecutionAlreadyExists: &nosqlplugin.WorkflowExecutionAlreadyExists{
						OtherInfo:        msg,
						CreateRequestID:  execution.CreateRequestID,
						RunID:            execution.RunID,
						State:            execution.State,
						CloseStatus:      execution.CloseStatus,
						LastWriteVersion: lastWriteVersion,
					},
				}
			}

			previous = make(map[string]interface{})
			if !iter.MapScan(previous) {
				// Cassandra returns the actual row that caused a condition failure, so we should always return
				// from the checks above, but just in case.
				break GetFailureReasonLoop
			}
		}

		return newUnknownConditionFailureReason(shardCondition.RangeID, previous)
	}

	return nil
}

func (db *cdb) executeUpdateWorkflowBatchTransaction(
	batch gocql.Batch,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	PreviousNextEventIDCondition int64,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	previous := make(map[string]interface{})
	applied, iter, err := db.session.MapExecuteBatchCAS(batch, previous)
	defer func() {
		if iter != nil {
			_ = iter.Close()
		}
	}()

	if err != nil {
		return err
	}

	if !applied {
		requestRunID := currentWorkflowRequest.Row.RunID
		requestCondition := PreviousNextEventIDCondition
		requestRangeID := shardCondition.RangeID
		requestConditionalRunID := *currentWorkflowRequest.Condition.CurrentRunID

		// There can be three reasons why the query does not get applied: the RangeID has changed, or the next_event_id or current_run_id check failed.
		// Check the row info returned by Cassandra to figure out which one it is.
		rangeIDUnmatch := false
		actualRangeID := int64(0)
		nextEventIDUnmatch := false
		actualNextEventID := int64(0)
		runIDUnmatch := false
		actualCurrRunID := ""
		var allPrevious []map[string]interface{}

	GetFailureReasonLoop:
		for {
			rowType, ok := previous["type"].(int)
			if !ok {
				// This should never happen, as all our rows have the type field.
				break GetFailureReasonLoop
			}

			runID := previous["run_id"].(gocql.UUID).String()

			if rowType == rowTypeShard {
				if actualRangeID, ok = previous["range_id"].(int64); ok && actualRangeID != requestRangeID {
					// UpdateWorkflowExecution failed because rangeID was modified
					rangeIDUnmatch = true
				}
			} else if rowType == rowTypeExecution && runID == requestRunID {
				if actualNextEventID, ok = previous["next_event_id"].(int64); ok && actualNextEventID != requestCondition {
					// UpdateWorkflowExecution failed because next event ID is unexpected
					nextEventIDUnmatch = true
				}
			} else if rowType == rowTypeExecution && runID == permanentRunID {
				// UpdateWorkflowExecution failed because current_run_id is unexpected
				if actualCurrRunID = previous["current_run_id"].(gocql.UUID).String(); actualCurrRunID != requestConditionalRunID {
					// UpdateWorkflowExecution failed because next event ID is unexpected
					runIDUnmatch = true
				}
			}

			allPrevious = append(allPrevious, previous)
			previous = make(map[string]interface{})
			if !iter.MapScan(previous) {
				// Cassandra returns the actual row that caused a condition failure, so we should always return
				// from the checks above, but just in case.
				break GetFailureReasonLoop
			}
		}

		if rangeIDUnmatch {
			return &nosqlplugin.WorkflowOperationConditionFailure{
				ShardRangeIDNotMatch: common.Int64Ptr(actualRangeID),
			}
		}

		if runIDUnmatch {
			msg := fmt.Sprintf("Failed to update mutable state.  Request Condition: %v, Actual Value: %v, Request Current RunID: %v, Actual Value: %v",
				requestCondition, actualNextEventID, requestConditionalRunID, actualCurrRunID)
			return &nosqlplugin.WorkflowOperationConditionFailure{
				CurrentWorkflowConditionFailInfo: &msg,
			}
		}

		if nextEventIDUnmatch {
			msg := fmt.Sprintf("Failed to update mutable state.  Request Condition: %v, Actual Value: %v, Request Current RunID: %v, Actual Value: %v",
				requestCondition, actualNextEventID, requestConditionalRunID, actualCurrRunID)
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
		msg := fmt.Sprintf("Failed to update mutable state. ShardID: %v, RangeID: %v, Condition: %v, Request Current RunID: %v, columns: (%v)",
			shardCondition.ShardID, requestRangeID, requestCondition, requestConditionalRunID, strings.Join(columns, ","))
		return &nosqlplugin.WorkflowOperationConditionFailure{
			UnknownConditionFailureDetails: &msg,
		}
	}

	return nil
}

func newUnknownConditionFailureReason(
	rangeID int64,
	row map[string]interface{},
) *nosqlplugin.WorkflowOperationConditionFailure {
	// At this point we only know that the write was not applied.
	// It's much safer to return ShardOwnershipLostError as the default to force the application to reload
	// shard to recover from such errors
	var columns []string
	for k, v := range row {
		columns = append(columns, fmt.Sprintf("%s=%v", k, v))
	}

	msg := fmt.Sprintf("Failed to operate on workflow execution.  Request RangeID: %v, columns: (%v)",
		rangeID, strings.Join(columns, ","))

	return &nosqlplugin.WorkflowOperationConditionFailure{
		UnknownConditionFailureDetails: &msg,
	}
}

func (db *cdb) assertShardRangeID(batch gocql.Batch, shardID int, rangeID int64) error {
	batch.Query(templateUpdateLeaseQuery,
		rangeID,
		shardID,
		rowTypeShard,
		rowTypeShardDomainID,
		rowTypeShardWorkflowID,
		rowTypeShardRunID,
		defaultVisibilityTimestamp,
		rowTypeShardTaskID,
		rangeID,
	)
	return nil
}

func (db *cdb) createTimerTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	timerTasks []*nosqlplugin.TimerTask,
) error {
	for _, task := range timerTasks {
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
			task.Type,
			task.TimeoutType,
			task.EventID,
			task.Attempt,
			task.Version,
			ts,
			task.TaskID)
	}
	return nil
}

func (db *cdb) createReplicationTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	transferTasks []*nosqlplugin.ReplicationTask,
) error {
	for _, task := range transferTasks {
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
			task.Type,
			task.FirstEventID,
			task.NextEventID,
			task.Version,
			task.ActivityScheduleID,
			persistence.EventStoreVersion,
			task.BranchToken,
			persistence.EventStoreVersion,
			task.NewRunBranchToken,
			task.VisibilityTimestamp.UnixNano(),
			// NOTE: use a constant here instead of task.VisibilityTimestamp so that we can query tasks with the same visibilityTimestamp
			defaultVisibilityTimestamp,
			task.TaskID)
	}
	return nil
}

func (db *cdb) createTransferTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	transferTasks []*nosqlplugin.TransferTask,
) error {
	for _, task := range transferTasks {
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
			task.TargetWorkflowID,
			task.TargetRunID,
			task.TargetChildWorkflowOnly,
			task.TaskList,
			task.Type,
			task.ScheduleID,
			task.RecordVisibility,
			task.Version,
			// NOTE: use a constant here instead of task.VisibilityTimestamp so that we can query tasks with the same visibilityTimestamp
			defaultVisibilityTimestamp,
			task.TaskID)
	}
	return nil
}

func (db *cdb) createCrossClusterTasks(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	transferTasks []*nosqlplugin.CrossClusterTask,
) error {
	for _, task := range transferTasks {
		batch.Query(templateCreateCrossClusterTaskQuery,
			shardID,
			rowTypeCrossClusterTask,
			rowTypeCrossClusterDomainID,
			task.TargetCluster,
			rowTypeCrossClusterRunID,
			domainID,
			workflowID,
			task.RunID,
			task.VisibilityTimestamp,
			task.TaskID,
			task.TargetDomainID,
			task.TargetWorkflowID,
			task.TargetRunID,
			task.TargetChildWorkflowOnly,
			task.TaskList,
			task.Type,
			task.ScheduleID,
			task.RecordVisibility,
			task.Version,
			// NOTE: use a constant here instead of task.VisibilityTimestamp so that we can query tasks with the same visibilityTimestamp
			defaultVisibilityTimestamp,
			task.TaskID,
		)
	}
	return nil
}

func (db *cdb) resetSignalsRequested(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalReqIDs []string,
) error {
	batch.Query(templateResetSignalRequestedQuery,
		signalReqIDs,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func (db *cdb) updateSignalsRequested(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalReqIDs []string,
	deleteSignalReqIDs []string,
) error {

	if len(signalReqIDs) > 0 {
		batch.Query(templateUpdateSignalRequestedQuery,
			signalReqIDs,
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

func (db *cdb) resetSignalInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalInfos map[int64]*persistence.SignalInfo,
) error {
	batch.Query(templateResetSignalInfoQuery,
		resetSignalInfoMap(signalInfos),
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetSignalInfoMap(
	signalInfos map[int64]*persistence.SignalInfo,
) map[int64]map[string]interface{} {

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

func (db *cdb) updateSignalInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	signalInfos map[int64]*persistence.SignalInfo,
	deleteInfos []int64,
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

func (db *cdb) resetRequestCancelInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	requestCancelInfos map[int64]*persistence.RequestCancelInfo,
) error {
	batch.Query(templateResetRequestCancelInfoQuery,
		resetRequestCancelInfoMap(requestCancelInfos),
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetRequestCancelInfoMap(
	requestCancelInfos map[int64]*persistence.RequestCancelInfo,
) map[int64]map[string]interface{} {

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

func (db *cdb) updateRequestCancelInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	requestCancelInfos map[int64]*persistence.RequestCancelInfo,
	deleteInfos []int64,
) error {

	for _, c := range requestCancelInfos {
		batch.Query(templateUpdateRequestCancelInfoQuery,
			c.InitiatedID,
			c.Version,
			c.InitiatedID,
			c.InitiatedEventBatchID,
			c.CancelRequestID,
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

func (db *cdb) resetChildExecutionInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo,
) error {
	infoMap, err := resetChildExecutionInfoMap(childExecutionInfos)
	if err != nil {
		return err
	}
	batch.Query(templateResetChildExecutionInfoQuery,
		infoMap,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetChildExecutionInfoMap(
	childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo,
) (map[int64]map[string]interface{}, error) {

	cMap := make(map[int64]map[string]interface{})
	for _, c := range childExecutionInfos {
		cInfo := make(map[string]interface{})
		initiatedEventData, initiatedEncoding := persistence.FromDataBlob(c.InitiatedEvent)
		startedEventData, startEncoding := persistence.FromDataBlob(c.StartedEvent)
		if c.StartedEvent != nil && initiatedEncoding != startEncoding {
			return nil, persistence.NewCadenceSerializationError(fmt.Sprintf("expect to have the same encoding, but %v != %v", initiatedEncoding, startEncoding))
		}
		cInfo["version"] = c.Version
		cInfo["event_data_encoding"] = initiatedEncoding
		cInfo["initiated_id"] = c.InitiatedID
		cInfo["initiated_event_batch_id"] = c.InitiatedEventBatchID
		cInfo["initiated_event"] = initiatedEventData
		cInfo["started_id"] = c.StartedID
		cInfo["started_event"] = startedEventData
		cInfo["create_request_id"] = c.CreateRequestID
		cInfo["started_workflow_id"] = c.StartedWorkflowID
		startedRunID := emptyRunID
		if c.StartedRunID != "" {
			startedRunID = c.StartedRunID
		}
		cInfo["started_run_id"] = startedRunID
		cInfo["domain_name"] = c.DomainName
		cInfo["workflow_type_name"] = c.WorkflowTypeName
		cInfo["parent_close_policy"] = int32(c.ParentClosePolicy)

		cMap[c.InitiatedID] = cInfo
	}

	return cMap, nil
}

func (db *cdb) updateChildExecutionInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	childExecutionInfos map[int64]*persistence.InternalChildExecutionInfo,
	deleteInfos []int64,
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
			c.DomainName,
			c.WorkflowTypeName,
			int32(c.ParentClosePolicy),
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

func (db *cdb) resetTimerInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timerInfos map[string]*persistence.TimerInfo,
) error {
	batch.Query(templateResetTimerInfoQuery,
		resetTimerInfoMap(timerInfos),
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetTimerInfoMap(
	timerInfos map[string]*persistence.TimerInfo,
) map[string]map[string]interface{} {

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

func (db *cdb) updateTimerInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	timerInfos map[string]*persistence.TimerInfo,
	deleteInfos []string,
) error {
	for _, timerInfo := range timerInfos {
		batch.Query(templateUpdateTimerInfoQuery,
			timerInfo.TimerID,
			timerInfo.Version,
			timerInfo.TimerID,
			timerInfo.StartedID,
			timerInfo.ExpiryTime,
			timerInfo.TaskStatus,
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

func (db *cdb) resetActivityInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	activityInfos map[int64]*persistence.InternalActivityInfo,
) error {
	infoMap, err := resetActivityInfoMap(activityInfos)
	if err != nil {
		return err
	}

	batch.Query(templateResetActivityInfoQuery,
		infoMap,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func resetActivityInfoMap(
	activityInfos map[int64]*persistence.InternalActivityInfo,
) (map[int64]map[string]interface{}, error) {

	aMap := make(map[int64]map[string]interface{})
	for _, a := range activityInfos {
		scheduledEventData, scheduleEncoding := persistence.FromDataBlob(a.ScheduledEvent)
		startedEventData, startEncoding := persistence.FromDataBlob(a.StartedEvent)
		if a.StartedEvent != nil && scheduleEncoding != startEncoding {
			return nil, persistence.NewCadenceSerializationError(fmt.Sprintf("expect to have the same encoding, but %v != %v", scheduleEncoding, startEncoding))
		}
		aInfo := make(map[string]interface{})
		aInfo["version"] = a.Version
		aInfo["event_data_encoding"] = scheduleEncoding
		aInfo["schedule_id"] = a.ScheduleID
		aInfo["scheduled_event_batch_id"] = a.ScheduledEventBatchID
		aInfo["scheduled_event"] = scheduledEventData
		aInfo["scheduled_time"] = a.ScheduledTime
		aInfo["started_id"] = a.StartedID
		aInfo["started_event"] = startedEventData
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

func (db *cdb) updateActivityInfos(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
	activityInfos map[int64]*persistence.InternalActivityInfo,
	deleteInfos []int64,
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
func (db *cdb) convertToCassandraTimestamp(in time.Time) time.Time {
	return time.Unix(0, persistence.DBTimestampToUnixNano(persistence.UnixNanoToDBTimestamp(in.UnixNano())))
}

func (db *cdb) createWorkflowExecutionWithMergeMaps(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
) error {
	err := db.createWorkflowExecution(batch, shardID, domainID, workflowID, execution)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode != nosqlplugin.EventBufferWriteModeNone {
		return fmt.Errorf("should only support EventBufferWriteModeNone")
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeMerge {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeMerge")
	}

	err = db.updateActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos, nil)
	if err != nil {
		return err
	}
	err = db.updateTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos, nil)
	if err != nil {
		return err
	}
	err = db.updateChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos, nil)
	if err != nil {
		return err
	}
	err = db.updateRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos, nil)
	if err != nil {
		return err
	}
	err = db.updateSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos, nil)
	if err != nil {
		return err
	}
	return db.updateSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs, nil)
}

func (db *cdb) resetWorkflowExecutionAndResetMapsAndEventBuffer(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
) error {
	err := db.updateWorkflowExecution(batch, shardID, domainID, workflowID, execution)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode != nosqlplugin.EventBufferWriteModeClear {
		return fmt.Errorf("should only support EventBufferWriteModeClear")
	}
	err = deleteBufferedEvents(batch, shardID, domainID, workflowID, execution.RunID)
	if err != nil {
		return err
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeReset {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeReset")
	}

	err = db.resetActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos)
	if err != nil {
		return err
	}
	err = db.resetTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos)
	if err != nil {
		return err
	}
	err = db.resetChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos)
	if err != nil {
		return err
	}
	err = db.resetRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos)
	if err != nil {
		return err
	}
	err = db.resetSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos)
	if err != nil {
		return err
	}
	return db.resetSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs)
}

func appendBufferedEvents(
	batch gocql.Batch,
	newBufferedEvents *persistence.DataBlob,
	shardID int,
	domainID string,
	workflowID string,
	runID string,
) error {
	values := make(map[string]interface{})
	values["encoding_type"] = newBufferedEvents.Encoding
	values["version"] = int64(0)
	values["data"] = newBufferedEvents.Data
	newEventValues := []map[string]interface{}{values}
	batch.Query(templateAppendBufferedEventsQuery,
		newEventValues,
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
) error {
	batch.Query(templateDeleteBufferedEventsQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID)
	return nil
}

func (db *cdb) updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
) error {
	err := db.updateWorkflowExecution(batch, shardID, domainID, workflowID, execution)
	if err != nil {
		return err
	}

	if execution.EventBufferWriteMode == nosqlplugin.EventBufferWriteModeClear {
		err = deleteBufferedEvents(batch, shardID, domainID, workflowID, execution.RunID)
		if err != nil {
			return err
		}
	} else if execution.EventBufferWriteMode == nosqlplugin.EventBufferWriteModeAppend {
		err = appendBufferedEvents(batch, execution.NewBufferedEventBatch, shardID, domainID, workflowID, execution.RunID)
		if err != nil {
			return err
		}
	}

	if execution.MapsWriteMode != nosqlplugin.WorkflowExecutionMapsWriteModeMergeAndDelete {
		return fmt.Errorf("should only support WorkflowExecutionMapsWriteModeMergeAndDelete")
	}

	err = db.updateActivityInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ActivityInfos, execution.ActivityInfoKeysToDelete)
	if err != nil {
		return err
	}
	err = db.updateTimerInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.TimerInfos, execution.TimerInfoKeysToDelete)
	if err != nil {
		return err
	}
	err = db.updateChildExecutionInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.ChildWorkflowInfos, execution.ChildWorkflowInfoKeysToDelete)
	if err != nil {
		return err
	}
	err = db.updateRequestCancelInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.RequestCancelInfos, execution.RequestCancelInfoKeysToDelete)
	if err != nil {
		return err
	}
	err = db.updateSignalInfos(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalInfos, execution.SignalInfoKeysToDelete)
	if err != nil {
		return err
	}
	return db.updateSignalsRequested(batch, shardID, domainID, workflowID, execution.RunID, execution.SignalRequestedIDs, execution.SignalRequestedIDsKeysToDelete)
}

func (db *cdb) updateWorkflowExecution(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
) error {
	execution.StartTimestamp = db.convertToCassandraTimestamp(execution.StartTimestamp)
	execution.LastUpdatedTimestamp = db.convertToCassandraTimestamp(execution.LastUpdatedTimestamp)

	batch.Query(templateUpdateWorkflowExecutionWithVersionHistoriesQuery,
		domainID,
		workflowID,
		execution.RunID,
		execution.ParentDomainID,
		execution.ParentWorkflowID,
		execution.ParentRunID,
		execution.InitiatedID,
		execution.CompletionEventBatchID,
		execution.CompletionEvent.Data,
		execution.CompletionEvent.GetEncodingString(),
		execution.TaskList,
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
		int32(execution.ExpirationSeconds.Seconds()),
		execution.SearchAttributes,
		execution.Memo,
		execution.NextEventID,
		execution.VersionHistories.Data,
		execution.VersionHistories.GetEncodingString(),
		execution.Checksums.Version,
		execution.Checksums.Flavor,
		execution.Checksums.Value,
		execution.LastWriteVersion,
		execution.State,
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

func (db *cdb) createWorkflowExecution(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	execution *nosqlplugin.WorkflowExecutionRequest,
) error {
	execution.StartTimestamp = db.convertToCassandraTimestamp(execution.StartTimestamp)
	execution.LastUpdatedTimestamp = db.convertToCassandraTimestamp(execution.LastUpdatedTimestamp)

	batch.Query(templateCreateWorkflowExecutionWithVersionHistoriesQuery,
		shardID,
		domainID,
		workflowID,
		execution.RunID,
		rowTypeExecution,
		domainID,
		workflowID,
		execution.RunID,
		execution.ParentDomainID,
		execution.ParentWorkflowID,
		execution.ParentRunID,
		execution.InitiatedID,
		execution.CompletionEventBatchID,
		execution.CompletionEvent.Data,
		execution.CompletionEvent.GetEncodingString(),
		execution.TaskList,
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
		int32(execution.ExpirationSeconds.Seconds()),
		execution.SearchAttributes,
		execution.Memo,
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
	)
	return nil
}

func (db *cdb) createOrUpdateCurrentWorkflow(
	batch gocql.Batch,
	shardID int,
	domainID string,
	workflowID string,
	request *nosqlplugin.CurrentWorkflowWriteRequest,
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

func createWorkflowExecutionInfo(
	result map[string]interface{},
) *persistence.InternalWorkflowExecutionInfo {

	info := &persistence.InternalWorkflowExecutionInfo{}
	var completionEventData []byte
	var completionEventEncoding common.EncodingType
	var autoResetPoints []byte
	var autoResetPointsEncoding common.EncodingType

	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "parent_domain_id":
			info.ParentDomainID = v.(gocql.UUID).String()
			if info.ParentDomainID == emptyDomainID {
				info.ParentDomainID = ""
			}
		case "parent_workflow_id":
			info.ParentWorkflowID = v.(string)
		case "parent_run_id":
			info.ParentRunID = v.(gocql.UUID).String()
			if info.ParentRunID == emptyRunID {
				info.ParentRunID = ""
			}
		case "initiated_id":
			info.InitiatedID = v.(int64)
		case "completion_event_batch_id":
			info.CompletionEventBatchID = v.(int64)
		case "completion_event":
			completionEventData = v.([]byte)
		case "completion_event_data_encoding":
			completionEventEncoding = common.EncodingType(v.(string))
		case "auto_reset_points":
			autoResetPoints = v.([]byte)
		case "auto_reset_points_encoding":
			autoResetPointsEncoding = common.EncodingType(v.(string))
		case "task_list":
			info.TaskList = v.(string)
		case "workflow_type_name":
			info.WorkflowTypeName = v.(string)
		case "workflow_timeout":
			info.WorkflowTimeout = common.SecondsToDuration(int64(v.(int)))
		case "decision_task_timeout":
			info.DecisionStartToCloseTimeout = common.SecondsToDuration(int64(v.(int)))
		case "execution_context":
			info.ExecutionContext = v.([]byte)
		case "state":
			info.State = v.(int)
		case "close_status":
			info.CloseStatus = v.(int)
		case "last_first_event_id":
			info.LastFirstEventID = v.(int64)
		case "last_event_task_id":
			info.LastEventTaskID = v.(int64)
		case "next_event_id":
			info.NextEventID = v.(int64)
		case "last_processed_event":
			info.LastProcessedEvent = v.(int64)
		case "start_time":
			info.StartTimestamp = v.(time.Time)
		case "last_updated_time":
			info.LastUpdatedTimestamp = v.(time.Time)
		case "create_request_id":
			info.CreateRequestID = v.(gocql.UUID).String()
		case "signal_count":
			info.SignalCount = int32(v.(int))
		case "history_size":
			info.HistorySize = v.(int64)
		case "decision_version":
			info.DecisionVersion = v.(int64)
		case "decision_schedule_id":
			info.DecisionScheduleID = v.(int64)
		case "decision_started_id":
			info.DecisionStartedID = v.(int64)
		case "decision_request_id":
			info.DecisionRequestID = v.(string)
		case "decision_timeout":
			info.DecisionTimeout = common.SecondsToDuration(int64(v.(int)))
		case "decision_attempt":
			info.DecisionAttempt = v.(int64)
		case "decision_timestamp":
			info.DecisionStartedTimestamp = time.Unix(0, v.(int64))
		case "decision_scheduled_timestamp":
			info.DecisionScheduledTimestamp = time.Unix(0, v.(int64))
		case "decision_original_scheduled_timestamp":
			info.DecisionOriginalScheduledTimestamp = time.Unix(0, v.(int64))
		case "cancel_requested":
			info.CancelRequested = v.(bool)
		case "cancel_request_id":
			info.CancelRequestID = v.(string)
		case "sticky_task_list":
			info.StickyTaskList = v.(string)
		case "sticky_schedule_to_start_timeout":
			info.StickyScheduleToStartTimeout = common.SecondsToDuration(int64(v.(int)))
		case "client_library_version":
			info.ClientLibraryVersion = v.(string)
		case "client_feature_version":
			info.ClientFeatureVersion = v.(string)
		case "client_impl":
			info.ClientImpl = v.(string)
		case "attempt":
			info.Attempt = int32(v.(int))
		case "has_retry_policy":
			info.HasRetryPolicy = v.(bool)
		case "init_interval":
			info.InitialInterval = common.SecondsToDuration(int64(v.(int)))
		case "backoff_coefficient":
			info.BackoffCoefficient = v.(float64)
		case "max_interval":
			info.MaximumInterval = common.SecondsToDuration(int64(v.(int)))
		case "max_attempts":
			info.MaximumAttempts = int32(v.(int))
		case "expiration_time":
			info.ExpirationTime = v.(time.Time)
		case "non_retriable_errors":
			info.NonRetriableErrors = v.([]string)
		case "branch_token":
			info.BranchToken = v.([]byte)
		case "cron_schedule":
			info.CronSchedule = v.(string)
		case "expiration_seconds":
			info.ExpirationSeconds = common.SecondsToDuration(int64(v.(int)))
		case "search_attributes":
			info.SearchAttributes = v.(map[string][]byte)
		case "memo":
			info.Memo = v.(map[string][]byte)
		}
	}
	info.CompletionEvent = persistence.NewDataBlob(completionEventData, completionEventEncoding)
	info.AutoResetPoints = persistence.NewDataBlob(autoResetPoints, autoResetPointsEncoding)
	return info
}
