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
	"strings"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

var _ nosqlplugin.WorkflowCRUD = (*cdb)(nil)

func (db *cdb) InsertWorkflowExecutionWithTasks(
	ctx context.Context,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	execution *nosqlplugin.WorkflowExecutionRequest,
	transferTasks []*nosqlplugin.TransferTask,
	crossClusterTasks []*nosqlplugin.CrossClusterTask,
	replicationTasks []*nosqlplugin.ReplicationTask,
	timerTasks []*nosqlplugin.TimerTask,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	shardID := shardCondition.ShardID
	domainID := execution.DomainID
	workflowID := execution.WorkflowID
	timeStamp := db.timeSrc.Now()

	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	err := insertOrUpsertWorkflowRequestRow(batch, requests, timeStamp)
	if err != nil {
		return err
	}
	err = createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest, timeStamp)
	if err != nil {
		return err
	}

	err = createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, execution, timeStamp)
	if err != nil {
		return err
	}

	createTransferTasks(batch, shardID, domainID, workflowID, transferTasks, timeStamp)
	createReplicationTasks(batch, shardID, domainID, workflowID, replicationTasks, timeStamp)
	createCrossClusterTasks(batch, shardID, domainID, workflowID, crossClusterTasks, timeStamp)
	createTimerTasks(batch, shardID, domainID, workflowID, timerTasks, timeStamp)
	assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)

	return executeCreateWorkflowBatchTransaction(ctx, db.session, batch, currentWorkflowRequest, execution, shardCondition)
}

func (db *cdb) SelectCurrentWorkflow(
	ctx context.Context,
	shardID int, domainID, workflowID string,
) (*nosqlplugin.CurrentWorkflowRow, error) {
	query := db.session.Query(templateGetCurrentExecutionQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		permanentRunID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
	).WithContext(ctx)

	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return nil, err
	}

	currentRunID := result["current_run_id"].(gocql.UUID).String()
	executionInfo := parseWorkflowExecutionInfo(result["execution"].(map[string]interface{}))
	lastWriteVersion := common.EmptyVersion
	if result["workflow_last_write_version"] != nil {
		lastWriteVersion = result["workflow_last_write_version"].(int64)
	}
	return &nosqlplugin.CurrentWorkflowRow{
		ShardID:          shardID,
		DomainID:         domainID,
		WorkflowID:       workflowID,
		RunID:            currentRunID,
		CreateRequestID:  executionInfo.CreateRequestID,
		State:            executionInfo.State,
		CloseStatus:      executionInfo.CloseStatus,
		LastWriteVersion: lastWriteVersion,
	}, nil
}

func (db *cdb) UpdateWorkflowExecutionWithTasks(
	ctx context.Context,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	mutatedExecution *nosqlplugin.WorkflowExecutionRequest,
	insertedExecution *nosqlplugin.WorkflowExecutionRequest,
	resetExecution *nosqlplugin.WorkflowExecutionRequest,
	transferTasks []*nosqlplugin.TransferTask,
	crossClusterTasks []*nosqlplugin.CrossClusterTask,
	replicationTasks []*nosqlplugin.ReplicationTask,
	timerTasks []*nosqlplugin.TimerTask,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	shardID := shardCondition.ShardID
	var domainID, workflowID string
	var previousNextEventIDCondition int64
	timeStamp := db.timeSrc.Now()
	if mutatedExecution != nil {
		domainID = mutatedExecution.DomainID
		workflowID = mutatedExecution.WorkflowID
		previousNextEventIDCondition = *mutatedExecution.PreviousNextEventIDCondition
	} else if resetExecution != nil {
		domainID = resetExecution.DomainID
		workflowID = resetExecution.WorkflowID
		previousNextEventIDCondition = *resetExecution.PreviousNextEventIDCondition
	} else {
		return fmt.Errorf("at least one of mutatedExecution and resetExecution should be provided")
	}

	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	err := insertOrUpsertWorkflowRequestRow(batch, requests, timeStamp)
	if err != nil {
		return err
	}
	err = createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest, timeStamp)
	if err != nil {
		return err
	}

	if mutatedExecution != nil {
		err = updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(batch, shardID, domainID, workflowID, mutatedExecution, timeStamp)
		if err != nil {
			return err
		}
	}

	if insertedExecution != nil {
		err = createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, insertedExecution, timeStamp)
		if err != nil {
			return err
		}
	}

	if resetExecution != nil {
		err = resetWorkflowExecutionAndMapsAndEventBuffer(batch, shardID, domainID, workflowID, resetExecution, timeStamp)
		if err != nil {
			return err
		}
	}

	createTransferTasks(batch, shardID, domainID, workflowID, transferTasks, timeStamp)
	createReplicationTasks(batch, shardID, domainID, workflowID, replicationTasks, timeStamp)
	createCrossClusterTasks(batch, shardID, domainID, workflowID, crossClusterTasks, timeStamp)
	createTimerTasks(batch, shardID, domainID, workflowID, timerTasks, timeStamp)
	assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)

	return executeUpdateWorkflowBatchTransaction(ctx, db.session, batch, currentWorkflowRequest, previousNextEventIDCondition, shardCondition)
}

func (db *cdb) SelectWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) (*nosqlplugin.WorkflowExecution, error) {
	query := db.session.Query(templateGetWorkflowExecutionQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
	).WithContext(ctx)

	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return nil, err
	}

	state := &nosqlplugin.WorkflowExecution{}
	info := parseWorkflowExecutionInfo(result["execution"].(map[string]interface{}))
	state.ExecutionInfo = info
	state.VersionHistories = persistence.NewDataBlob(result["version_histories"].([]byte), common.EncodingType(result["version_histories_encoding"].(string)))
	// TODO: remove this after all 2DC workflows complete
	replicationState := parseReplicationState(result["replication_state"].(map[string]interface{}))
	state.ReplicationState = replicationState

	activityInfos := make(map[int64]*persistence.InternalActivityInfo)
	aMap := result["activity_map"].(map[int64]map[string]interface{})
	for key, value := range aMap {
		info := parseActivityInfo(domainID, value)
		activityInfos[key] = info
	}
	state.ActivityInfos = activityInfos

	timerInfos := make(map[string]*persistence.TimerInfo)
	tMap := result["timer_map"].(map[string]map[string]interface{})
	for key, value := range tMap {
		info := parseTimerInfo(value)
		timerInfos[key] = info
	}
	state.TimerInfos = timerInfos

	childExecutionInfos := make(map[int64]*persistence.InternalChildExecutionInfo)
	cMap := result["child_executions_map"].(map[int64]map[string]interface{})
	for key, value := range cMap {
		info := parseChildExecutionInfo(value)
		childExecutionInfos[key] = info
	}
	state.ChildExecutionInfos = childExecutionInfos

	requestCancelInfos := make(map[int64]*persistence.RequestCancelInfo)
	rMap := result["request_cancel_map"].(map[int64]map[string]interface{})
	for key, value := range rMap {
		info := parseRequestCancelInfo(value)
		requestCancelInfos[key] = info
	}
	state.RequestCancelInfos = requestCancelInfos

	signalInfos := make(map[int64]*persistence.SignalInfo)
	sMap := result["signal_map"].(map[int64]map[string]interface{})
	for key, value := range sMap {
		info := parseSignalInfo(value)
		signalInfos[key] = info
	}
	state.SignalInfos = signalInfos

	signalRequestedIDs := make(map[string]struct{})
	sList := mustConvertToSlice(result["signal_requested"])
	for _, v := range sList {
		signalRequestedIDs[v.(gocql.UUID).String()] = struct{}{}
	}
	state.SignalRequestedIDs = signalRequestedIDs

	eList := result["buffered_events_list"].([]map[string]interface{})
	bufferedEventsBlobs := make([]*persistence.DataBlob, 0, len(eList))
	for _, v := range eList {
		blob := parseHistoryEventBatchBlob(v)
		bufferedEventsBlobs = append(bufferedEventsBlobs, blob)
	}
	state.BufferedEvents = bufferedEventsBlobs

	state.Checksum = parseChecksum(result["checksum"].(map[string]interface{}))
	return state, nil
}

func (db *cdb) DeleteCurrentWorkflow(ctx context.Context, shardID int, domainID, workflowID, currentRunIDCondition string) error {
	query := db.session.Query(templateDeleteWorkflowExecutionCurrentRowQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		permanentRunID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
		currentRunIDCondition,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) DeleteWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) error {
	query := db.session.Query(templateDeleteWorkflowExecutionMutableStateQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) SelectAllCurrentWorkflows(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.CurrentWorkflowExecution, []byte, error) {
	query := db.session.Query(
		templateListCurrentExecutionsQuery,
		shardID,
		rowTypeExecution,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectAllCurrentWorkflows operation failed. Not able to create query iterator.",
		}
	}
	result := make(map[string]interface{})
	var executions []*persistence.CurrentWorkflowExecution
	for iter.MapScan(result) {
		runID := result["run_id"].(gocql.UUID).String()
		if runID != permanentRunID {
			result = make(map[string]interface{})
			continue
		}
		executions = append(executions, &persistence.CurrentWorkflowExecution{
			DomainID:     result["domain_id"].(gocql.UUID).String(),
			WorkflowID:   result["workflow_id"].(string),
			RunID:        permanentRunID,
			State:        result["workflow_state"].(int),
			CurrentRunID: result["current_run_id"].(gocql.UUID).String(),
		})
		result = make(map[string]interface{})
	}
	nextPageToken := getNextPageToken(iter)

	return executions, nextPageToken, iter.Close()
}

func (db *cdb) SelectAllWorkflowExecutions(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.InternalListConcreteExecutionsEntity, []byte, error) {
	query := db.session.Query(
		templateListWorkflowExecutionQuery,
		shardID,
		rowTypeExecution,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectAllWorkflowExecutions operation failed.  Not able to create query iterator.",
		}
	}

	result := make(map[string]interface{})
	var executions []*persistence.InternalListConcreteExecutionsEntity
	for iter.MapScan(result) {
		runID := result["run_id"].(gocql.UUID).String()
		if runID == permanentRunID {
			result = make(map[string]interface{})
			continue
		}
		executions = append(executions, &persistence.InternalListConcreteExecutionsEntity{
			ExecutionInfo:    parseWorkflowExecutionInfo(result["execution"].(map[string]interface{})),
			VersionHistories: persistence.NewDataBlob(result["version_histories"].([]byte), common.EncodingType(result["version_histories_encoding"].(string))),
		})
		result = make(map[string]interface{})
	}
	nextPageToken := getNextPageToken(iter)

	return executions, nextPageToken, iter.Close()
}

func (db *cdb) IsWorkflowExecutionExists(ctx context.Context, shardID int, domainID, workflowID, runID string) (bool, error) {
	query := db.session.Query(templateIsWorkflowExecutionExistsQuery,
		shardID,
		rowTypeExecution,
		domainID,
		workflowID,
		runID,
		defaultVisibilityTimestamp,
		rowTypeExecutionTaskID,
	).WithContext(ctx)

	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		if db.client.IsNotFoundError(err) {
			return false, nil
		}

		return false, err
	}
	return true, nil
}

func (db *cdb) SelectTransferTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, exclusiveMinTaskID, inclusiveMaxTaskID int64) ([]*nosqlplugin.TransferTask, []byte, error) {
	// Reading transfer tasks need to be quorum level consistent, otherwise we could loose task
	query := db.session.Query(templateGetTransferTasksQuery,
		shardID,
		rowTypeTransferTask,
		rowTypeTransferDomainID,
		rowTypeTransferWorkflowID,
		rowTypeTransferRunID,
		defaultVisibilityTimestamp,
		exclusiveMinTaskID,
		inclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectTransferTasksOrderByTaskID operation failed.  Not able to create query iterator.",
		}
	}

	var tasks []*nosqlplugin.TransferTask
	task := make(map[string]interface{})
	for iter.MapScan(task) {
		t := parseTransferTaskInfo(task["transfer"].(map[string]interface{}))
		// Reset task map to get it ready for next scan
		task = make(map[string]interface{})

		tasks = append(tasks, t)
	}
	nextPageToken := getNextPageToken(iter)

	err := iter.Close()
	return tasks, nextPageToken, err
}

func (db *cdb) DeleteTransferTask(ctx context.Context, shardID int, taskID int64) error {
	query := db.session.Query(templateCompleteTransferTaskQuery,
		shardID,
		rowTypeTransferTask,
		rowTypeTransferDomainID,
		rowTypeTransferWorkflowID,
		rowTypeTransferRunID,
		defaultVisibilityTimestamp,
		taskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) RangeDeleteTransferTasks(ctx context.Context, shardID int, exclusiveBeginTaskID, inclusiveEndTaskID int64) error {
	query := db.session.Query(templateRangeCompleteTransferTaskQuery,
		shardID,
		rowTypeTransferTask,
		rowTypeTransferDomainID,
		rowTypeTransferWorkflowID,
		rowTypeTransferRunID,
		defaultVisibilityTimestamp,
		exclusiveBeginTaskID,
		inclusiveEndTaskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) SelectTimerTasksOrderByVisibilityTime(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTime, exclusiveMaxTime time.Time) ([]*nosqlplugin.TimerTask, []byte, error) {
	// Reading timer tasks need to be quorum level consistent, otherwise we could loose task
	minTimestamp := persistence.UnixNanoToDBTimestamp(inclusiveMinTime.UnixNano())
	maxTimestamp := persistence.UnixNanoToDBTimestamp(exclusiveMaxTime.UnixNano())
	query := db.session.Query(templateGetTimerTasksQuery,
		shardID,
		rowTypeTimerTask,
		rowTypeTimerDomainID,
		rowTypeTimerWorkflowID,
		rowTypeTimerRunID,
		minTimestamp,
		maxTimestamp,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectTimerTasksOrderByVisibilityTime operation failed.  Not able to create query iterator.",
		}
	}

	var timers []*nosqlplugin.TimerTask
	task := make(map[string]interface{})
	for iter.MapScan(task) {
		t := parseTimerTaskInfo(task["timer"].(map[string]interface{}))
		// Reset task map to get it ready for next scan
		task = make(map[string]interface{})

		timers = append(timers, t)
	}
	nextPageToken := getNextPageToken(iter)

	err := iter.Close()
	return timers, nextPageToken, err
}

func (db *cdb) DeleteTimerTask(ctx context.Context, shardID int, taskID int64, visibilityTimestamp time.Time) error {
	ts := persistence.UnixNanoToDBTimestamp(visibilityTimestamp.UnixNano())
	query := db.session.Query(templateCompleteTimerTaskQuery,
		shardID,
		rowTypeTimerTask,
		rowTypeTimerDomainID,
		rowTypeTimerWorkflowID,
		rowTypeTimerRunID,
		ts,
		taskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) RangeDeleteTimerTasks(ctx context.Context, shardID int, inclusiveMinTime, exclusiveMaxTime time.Time) error {
	start := persistence.UnixNanoToDBTimestamp(inclusiveMinTime.UnixNano())
	end := persistence.UnixNanoToDBTimestamp(exclusiveMaxTime.UnixNano())
	query := db.session.Query(templateRangeCompleteTimerTaskQuery,
		shardID,
		rowTypeTimerTask,
		rowTypeTimerDomainID,
		rowTypeTimerWorkflowID,
		rowTypeTimerRunID,
		start,
		end,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) SelectReplicationTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, exclusiveMinTaskID, inclusiveMaxTaskID int64) ([]*nosqlplugin.ReplicationTask, []byte, error) {
	// Reading replication tasks need to be quorum level consistent, otherwise we could loose task
	query := db.session.Query(templateGetReplicationTasksQuery,
		shardID,
		rowTypeReplicationTask,
		rowTypeReplicationDomainID,
		rowTypeReplicationWorkflowID,
		rowTypeReplicationRunID,
		defaultVisibilityTimestamp,
		exclusiveMinTaskID,
		inclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)
	return populateGetReplicationTasks(query)
}

func (db *cdb) DeleteReplicationTask(ctx context.Context, shardID int, taskID int64) error {
	query := db.session.Query(templateCompleteReplicationTaskQuery,
		shardID,
		rowTypeReplicationTask,
		rowTypeReplicationDomainID,
		rowTypeReplicationWorkflowID,
		rowTypeReplicationRunID,
		defaultVisibilityTimestamp,
		taskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) RangeDeleteReplicationTasks(ctx context.Context, shardID int, inclusiveEndTaskID int64) error {
	query := db.session.Query(templateCompleteReplicationTaskBeforeQuery,
		shardID,
		rowTypeReplicationTask,
		rowTypeReplicationDomainID,
		rowTypeReplicationWorkflowID,
		rowTypeReplicationRunID,
		defaultVisibilityTimestamp,
		inclusiveEndTaskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) SelectCrossClusterTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, targetCluster string, exclusiveMinTaskID, inclusiveMaxTaskID int64) ([]*nosqlplugin.CrossClusterTask, []byte, error) {
	// Reading cross-cluster tasks need to be quorum level consistent, otherwise we could loose task
	query := db.session.Query(templateGetCrossClusterTasksQuery,
		shardID,
		rowTypeCrossClusterTask,
		rowTypeCrossClusterDomainID,
		targetCluster, // workflowID field is used to store target cluster
		rowTypeCrossClusterRunID,
		defaultVisibilityTimestamp,
		exclusiveMinTaskID,
		inclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectCrossClusterTasksOrderByTaskID operation failed.  Not able to create query iterator.",
		}
	}

	var tasks []*nosqlplugin.CrossClusterTask
	task := make(map[string]interface{})
	for iter.MapScan(task) {
		t := parseCrossClusterTaskInfo(task["cross_cluster"].(map[string]interface{}))
		// Reset task map to get it ready for next scan
		task = make(map[string]interface{})

		tasks = append(tasks, &nosqlplugin.CrossClusterTask{
			TransferTask:  *t,
			TargetCluster: targetCluster,
		})
	}
	nextPageToken := getNextPageToken(iter)
	err := iter.Close()
	return tasks, nextPageToken, err
}

func (db *cdb) DeleteCrossClusterTask(ctx context.Context, shardID int, targetCluster string, taskID int64) error {
	query := db.session.Query(templateCompleteCrossClusterTaskQuery,
		shardID,
		rowTypeCrossClusterTask,
		rowTypeCrossClusterDomainID,
		targetCluster,
		rowTypeCrossClusterRunID,
		defaultVisibilityTimestamp,
		taskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) RangeDeleteCrossClusterTasks(ctx context.Context, shardID int, targetCluster string, exclusiveBeginTaskID, inclusiveEndTaskID int64) error {
	query := db.session.Query(templateRangeCompleteCrossClusterTaskQuery,
		shardID,
		rowTypeCrossClusterTask,
		rowTypeCrossClusterDomainID,
		targetCluster,
		rowTypeCrossClusterRunID,
		defaultVisibilityTimestamp,
		exclusiveBeginTaskID,
		inclusiveEndTaskID,
	).WithContext(ctx)

	return query.Exec()
}

func (db *cdb) InsertReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, task nosqlplugin.ReplicationTask) error {
	// Use source cluster name as the workflow id for replication dlq
	query := db.session.Query(templateCreateReplicationTaskQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		task.DomainID,
		task.WorkflowID,
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
		defaultVisibilityTimestamp,
		defaultVisibilityTimestamp,
		task.TaskID,
		db.timeSrc.Now(),
	).WithContext(ctx)

	return query.Exec()
}

func (db *cdb) SelectReplicationDLQTasksOrderByTaskID(ctx context.Context, shardID int, sourceCluster string, pageSize int, pageToken []byte, exclusiveMinTaskID, inclusiveMaxTaskID int64) ([]*nosqlplugin.ReplicationTask, []byte, error) {
	// Reading replication tasks need to be quorum level consistent, otherwise we could loose task
	query := db.session.Query(templateGetReplicationTasksQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		defaultVisibilityTimestamp,
		exclusiveMinTaskID,
		inclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	return populateGetReplicationTasks(query)
}

func (db *cdb) SelectReplicationDLQTasksCount(ctx context.Context, shardID int, sourceCluster string) (int64, error) {
	// Reading replication tasks need to be quorum level consistent, otherwise we could loose task
	query := db.session.Query(templateGetDLQSizeQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
	).WithContext(ctx)

	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return -1, err
	}

	queueSize := result["count"].(int64)
	return queueSize, nil
}

func (db *cdb) DeleteReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, taskID int64) error {
	query := db.session.Query(templateCompleteReplicationTaskQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		defaultVisibilityTimestamp,
		taskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) RangeDeleteReplicationDLQTasks(ctx context.Context, shardID int, sourceCluster string, exclusiveBeginTaskID, inclusiveEndTaskID int64) error {
	query := db.session.Query(templateRangeCompleteReplicationTaskQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		defaultVisibilityTimestamp,
		exclusiveBeginTaskID,
		inclusiveEndTaskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) InsertReplicationTask(ctx context.Context, tasks []*nosqlplugin.ReplicationTask, shardCondition nosqlplugin.ShardCondition) error {
	if len(tasks) == 0 {
		return nil
	}

	shardID := shardCondition.ShardID
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	timeStamp := db.timeSrc.Now()
	for _, task := range tasks {
		createReplicationTasks(batch, shardID, task.DomainID, task.WorkflowID, []*nosqlplugin.ReplicationTask{task}, timeStamp)
	}

	assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)

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
		rowType, ok := previous["type"].(int)
		if !ok {
			// This should never happen, as all our rows have the type field.
			panic("Encounter row type not found")
		}
		if rowType == rowTypeShard {
			if actualRangeID, ok := previous["range_id"].(int64); ok && actualRangeID != shardCondition.RangeID {
				// CreateWorkflowExecution failed because rangeID was modified
				return &nosqlplugin.ShardOperationConditionFailure{
					RangeID: actualRangeID,
				}
			}
		}

		// At this point we only know that the write was not applied.
		// It's much safer to return ShardOperationConditionFailure(which will become ShardOwnershipLostError later) as the default to force the application to reload
		// shard to recover from such errors
		var columns []string
		for k, v := range previous {
			columns = append(columns, fmt.Sprintf("%s=%v", k, v))
		}
		return &nosqlplugin.ShardOperationConditionFailure{
			RangeID: -1,
			Details: strings.Join(columns, ","),
		}
	}
	return nil
}
