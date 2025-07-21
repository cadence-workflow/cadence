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
	"sync"
	"time"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

var _ nosqlplugin.WorkflowCRUD = (*cdb)(nil)

// Reuse maps to reduce GC pressure during high-throughput operations
var (
	taskMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}
	resultMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{})
		},
	}
)

// dynamicBatchSize computes an optimal batch size based on the data volume and operation complexity.
// If the operation is expected to be heavy (lots of tasks or large data), it will use smaller batch sizes.
// This helps to avoid OOM and database overloads.
func dynamicBatchSize(taskCount int, dataVolume int) int {
	if taskCount > 100 || dataVolume > 1024*1024*4 { // >100 tasks or >4MB data
		return 20
	}
	if taskCount > 20 || dataVolume > 1024*1024 {
		return 50
	}
	return 100
}

// addToBatches splits operations into optimally sized batches
func addToBatches(batches *[]*gocql.Batch, currentBatch *gocql.Batch, ctx context.Context, session gocql.Session, currentOpCount *int, batchSize int) *gocql.Batch {
	if *currentOpCount >= batchSize && len(currentBatch.Entries) > 0 {
		*batches = append(*batches, currentBatch)
		*currentOpCount = 0
		return session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	}
	return currentBatch
}

func (db *cdb) InsertWorkflowExecutionWithTasks(
	ctx context.Context,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	execution *nosqlplugin.WorkflowExecutionRequest,
	tasksByCategory map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask,
	activeClusterSelectionPolicyRow *nosqlplugin.ActiveClusterSelectionPolicyRow,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	shardID := shardCondition.ShardID
	domainID := execution.DomainID
	workflowID := execution.WorkflowID
	timeStamp := execution.CurrentTimeStamp

	// Estimate rough data volume for dynamic batch size.
	taskCount := 0
	dataVolume := 0
	for _, tasks := range tasksByCategory {
		taskCount += len(tasks)
		for _, t := range tasks {
			if t.Task != nil {
				dataVolume += len(t.Task.Data)
			}
		}
	}
	batchSize := dynamicBatchSize(taskCount, dataVolume)

	var batches []*gocql.Batch
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	opCount := 0

	if err := insertWorkflowActiveClusterSelectionPolicyRow(batch, activeClusterSelectionPolicyRow, timeStamp); err != nil {
		return err
	}
	opCount++
	
	if err := insertOrUpsertWorkflowRequestRow(batch, requests, timeStamp); err != nil {
		return err
	}
	opCount++
	
	if err := createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest, timeStamp); err != nil {
		return err
	}
	opCount++
	
	if err := createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, execution, timeStamp); err != nil {
		return err
	}
	opCount++

	createTasksByCategory(batch, shardID, domainID, workflowID, timeStamp, tasksByCategory)
	opCount += taskCount

	assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)
	opCount++

	// Split into multiple batches if needed
	batch = addToBatches(&batches, batch, ctx, db.session, &opCount, batchSize)

	// Add final batch if it has entries
	if len(batch.Entries) > 0 {
		batches = append(batches, batch)
	}

	// Execute all prepared batches in sequence; fail fast on error
	for _, b := range batches {
		if err := executeCreateWorkflowBatchTransaction(ctx, db.session, b, currentWorkflowRequest, execution, shardCondition); err != nil {
			return err
		}
	}

	return nil
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

	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		// Clear map before returning to pool
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()

	if err := query.MapScan(result); err != nil {
		return nil, err
	}

	currentRunID := ""
	if v, ok := result["current_run_id"].(gocql.UUID); ok {
		currentRunID = v.String()
	} else if v, ok := result["current_run_id"].(string); ok {
		currentRunID = v
	}
	
	execVal, ok := result["execution"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse workflow execution info")
	}
	
	executionInfo := parseWorkflowExecutionInfo(execVal)
	lastWriteVersion := constants.EmptyVersion
	if v, ok := result["workflow_last_write_version"].(int64); ok {
		lastWriteVersion = v
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
	tasksByCategory map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	shardID := shardCondition.ShardID
	var domainID, workflowID string
	var previousNextEventIDCondition int64
	var timeStamp time.Time
	
	if mutatedExecution != nil {
		domainID = mutatedExecution.DomainID
		workflowID = mutatedExecution.WorkflowID
		if mutatedExecution.PreviousNextEventIDCondition == nil {
			return fmt.Errorf("mutatedExecution.PreviousNextEventIDCondition must not be nil")
		}
		previousNextEventIDCondition = *mutatedExecution.PreviousNextEventIDCondition
		timeStamp = mutatedExecution.CurrentTimeStamp
	} else if resetExecution != nil {
		domainID = resetExecution.DomainID
		workflowID = resetExecution.WorkflowID
		if resetExecution.PreviousNextEventIDCondition == nil {
			return fmt.Errorf("resetExecution.PreviousNextEventIDCondition must not be nil")
		}
		previousNextEventIDCondition = *resetExecution.PreviousNextEventIDCondition
		timeStamp = resetExecution.CurrentTimeStamp
	} else {
		return fmt.Errorf("at least one of mutatedExecution and resetExecution should be provided")
	}

	// Estimate task count for batch sizing
	taskCount := 0
	for _, tasks := range tasksByCategory {
		taskCount += len(tasks)
	}
	batchSize := dynamicBatchSize(taskCount, 0)

	var batches []*gocql.Batch
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	opCount := 0

	if err := insertOrUpsertWorkflowRequestRow(batch, requests, timeStamp); err != nil {
		return err
	}
	opCount++
	
	if err := createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest, timeStamp); err != nil {
		return err
	}
	opCount++

	if mutatedExecution != nil {
		if err := updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(batch, shardID, domainID, workflowID, mutatedExecution, timeStamp); err != nil {
			return err
		}
		opCount++
	}

	if insertedExecution != nil {
		if err := createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, insertedExecution, timeStamp); err != nil {
			return err
		}
		opCount++
	}

	if resetExecution != nil {
		if err := resetWorkflowExecutionAndMapsAndEventBuffer(batch, shardID, domainID, workflowID, resetExecution, timeStamp); err != nil {
			return err
		}
		opCount++
	}

	createTasksByCategory(batch, shardID, domainID, workflowID, timeStamp, tasksByCategory)
	opCount += taskCount

	assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)
	opCount++

	// Split into multiple batches if needed
	batch = addToBatches(&batches, batch, ctx, db.session, &opCount, batchSize)

	// Add final batch if it has entries
	if len(batch.Entries) > 0 {
		batches = append(batches, batch)
	}

	// Execute all batches
	for _, b := range batches {
		if err := executeUpdateWorkflowBatchTransaction(ctx, db.session, b, currentWorkflowRequest, previousNextEventIDCondition, shardCondition); err != nil {
			return err
		}
	}
	
	return nil
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

	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		// Clear map before returning to pool
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()

	if err := query.MapScan(result); err != nil {
		return nil, err
	}

	state := &nosqlplugin.WorkflowExecution{}
	execMap, ok := result["execution"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse workflow execution info")
	}
	
	info := parseWorkflowExecutionInfo(execMap)
	state.ExecutionInfo = info

	// Parse version histories
	if vh, ok := result["version_histories"].([]byte); ok {
		if enc, ok2 := result["version_histories_encoding"].(string); ok2 {
			state.VersionHistories = persistence.NewDataBlob(vh, constants.EncodingType(enc))
		}
	}

	// TODO: remove this after all 2DC workflows complete
	if repState, ok := result["replication_state"].(map[string]interface{}); ok {
		state.ReplicationState = parseReplicationState(repState)
	}

	// Parse activity infos with reused map
	activityInfos := make(map[int64]*persistence.InternalActivityInfo)
	if aMap, ok := result["activity_map"].(map[int64]map[string]interface{}); ok {
		for key, value := range aMap {
			info := parseActivityInfo(domainID, value)
			activityInfos[key] = info
		}
	}
	state.ActivityInfos = activityInfos

	// Parse timer infos with reused map
	timerInfos := make(map[string]*persistence.TimerInfo)
	if tMap, ok := result["timer_map"].(map[string]map[string]interface{}); ok {
		for key, value := range tMap {
			info := parseTimerInfo(value)
			timerInfos[key] = info
		}
	}
	state.TimerInfos = timerInfos

	// Parse child execution infos with reused map
	childExecutionInfos := make(map[int64]*persistence.InternalChildExecutionInfo)
	if cMap, ok := result["child_executions_map"].(map[int64]map[string]interface{}); ok {
		for key, value := range cMap {
			info := parseChildExecutionInfo(value)
			childExecutionInfos[key] = info
		}
	}
	state.ChildExecutionInfos = childExecutionInfos

	// Parse request cancel infos with reused map
	requestCancelInfos := make(map[int64]*persistence.RequestCancelInfo)
	if rMap, ok := result["request_cancel_map"].(map[int64]map[string]interface{}); ok {
		for key, value := range rMap {
			info := parseRequestCancelInfo(value)
			requestCancelInfos[key] = info
		}
	}
	state.RequestCancelInfos = requestCancelInfos

	// Parse signal infos with reused map
	signalInfos := make(map[int64]*persistence.SignalInfo)
	if sMap, ok := result["signal_map"].(map[int64]map[string]interface{}); ok {
		for key, value := range sMap {
			info := parseSignalInfo(value)
			signalInfos[key] = info
		}
	}
	state.SignalInfos = signalInfos

	// Parse signal requested IDs
	signalRequestedIDs := make(map[string]struct{})
	if sList := mustConvertToSlice(result["signal_requested"]); sList != nil {
		for _, v := range sList {
			switch id := v.(type) {
			case gocql.UUID:
				signalRequestedIDs[id.String()] = struct{}{}
			case string:
				signalRequestedIDs[id] = struct{}{}
			}
		}
	}
	state.SignalRequestedIDs = signalRequestedIDs

	// Parse buffered events - pre-allocate slice with estimated capacity
	bufferedEventsBlobs := make([]*persistence.DataBlob, 0, 2)
	if eList, ok := result["buffered_events_list"].([]map[string]interface{}); ok && len(eList) > 0 {
		for _, v := range eList {
			blob := parseHistoryEventBatchBlob(v)
			bufferedEventsBlobs = append(bufferedEventsBlobs, blob)
		}
	}
	state.BufferedEvents = bufferedEventsBlobs

	// Parse checksum
	if csumMap, ok := result["checksum"].(map[string]interface{}); ok {
		state.Checksum = parseChecksum(csumMap)
	}
	
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
	
	// Estimate capacity to avoid reallocations
	estimated := pageSize
	if estimated < 32 {
		estimated = 32
	}
	
	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()
	
	var executions []*persistence.CurrentWorkflowExecution = make([]*persistence.CurrentWorkflowExecution, 0, estimated)
	
	for iter.MapScan(result) {
		runID := ""
		if v, ok := result["run_id"].(gocql.UUID); ok {
			runID = v.String()
		} else if v, ok := result["run_id"].(string); ok {
			runID = v
		}
		
		if runID != permanentRunID {
			for k := range result {
				delete(result, k)
			}
			continue
		}
		
		did := ""
		if v, ok := result["domain_id"].(gocql.UUID); ok {
			did = v.String()
		} else if v, ok := result["domain_id"].(string); ok {
			did = v
		}
		
		stateInt := 0
		if si, ok := result["workflow_state"].(int); ok {
			stateInt = si
		} else if si, ok := result["workflow_state"].(int32); ok {
			stateInt = int(si)
		}
		
		cid := ""
		if v, ok := result["current_run_id"].(gocql.UUID); ok {
			cid = v.String()
		} else if v, ok := result["current_run_id"].(string); ok {
			cid = v
		}
		
		workflowID, _ := result["workflow_id"].(string)
		
		executions = append(executions, &persistence.CurrentWorkflowExecution{
			DomainID:     did,
			WorkflowID:   workflowID,
			RunID:        permanentRunID,
			State:        stateInt,
			CurrentRunID: cid,
		})
		
		// Clear map for reuse
		for k := range result {
			delete(result, k)
		}
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
			Message: "SelectAllWorkflowExecutions operation failed. Not able to create query iterator.",
		}
	}

	// Estimate capacity to avoid reallocations
	estimated := pageSize
	if estimated < 32 {
		estimated = 32
	}
	
	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()
	
	var executions []*persistence.InternalListConcreteExecutionsEntity = make([]*persistence.InternalListConcreteExecutionsEntity, 0, estimated)
	
	for iter.MapScan(result) {
		runID := ""
		if v, ok := result["run_id"].(gocql.UUID); ok {
			runID = v.String()
		} else if v, ok := result["run_id"].(string); ok {
			runID = v
		}
		
		if runID == permanentRunID {
			for k := range result {
				delete(result, k)
			}
			continue
		}
		
		execMap, ok := result["execution"].(map[string]interface{})
		if !ok {
			for k := range result {
				delete(result, k)
			}
			continue
		}
		
		vh, ok := result["version_histories"].([]byte)
		enc, ok2 := result["version_histories_encoding"].(string)
		
		entity := &persistence.InternalListConcreteExecutionsEntity{
			ExecutionInfo:    parseWorkflowExecutionInfo(execMap),
			VersionHistories: persistence.NewDataBlob(nil, ""),
		}
		
		if ok && ok2 {
			entity.VersionHistories = persistence.NewDataBlob(vh, constants.EncodingType(enc))
		}
		
		executions = append(executions, entity)
		
		// Clear map for reuse
		for k := range result {
			delete(result, k)
		}
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

	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()
	
	if err := query.MapScan(result); err != nil {
		if db.client.IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	
	return true, nil
}

func (db *cdb) SelectTransferTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	query := db.session.Query(templateGetTransferTasksQuery,
		shardID,
		rowTypeTransferTask,
		rowTypeTransferDomainID,
		rowTypeTransferWorkflowID,
		rowTypeTransferRunID,
		defaultVisibilityTimestamp,
		inclusiveMinTaskID,
		exclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, nil, &types.InternalServiceError{
			Message: "SelectTransferTasksOrderByTaskID operation failed. Not able to create query iterator.",
		}
	}

	// Estimate capacity to avoid reallocations
	estimated := pageSize
	if estimated < 32 {
		estimated = 32
	}
	
	var tasks []*nosqlplugin.HistoryMigrationTask = make([]*nosqlplugin.HistoryMigrationTask, 0, estimated)
	task := taskMapPool.Get().(map[string]interface{})
	
	defer func() {
		// Clear and return to pool
		for k := range task {
			delete(task, k)
		}
		taskMapPool.Put(task)
	}()
	
	for iter.MapScan(task) {
		transferMap, ok := task["transfer"].(map[string]interface{})
		if !ok {
			for k := range task {
				delete(task, k)
			}
			continue
		}
		
		t := parseTransferTaskInfo(transferMap)
		taskID, _ := task["task_id"].(int64)
		data, _ := task["data"].([]byte)
		encoding, _ := task["data_encoding"].(string)
		taskBlob := persistence.NewDataBlob(data, constants.EncodingType(encoding))

		tasks = append(tasks, &nosqlplugin.HistoryMigrationTask{
			Transfer: t,
			Task:     taskBlob,
			TaskID:   taskID,
		})
		
		// Clear map for reuse
		for k := range task {
			delete(task, k)
		}
	}
	
	nextPageToken := getNextPageToken(iter)
	return tasks, nextPageToken, iter.Close()
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

func (db *cdb) SelectTimerTasksOrderByVisibilityTime(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTime, exclusiveMaxTime time.Time) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
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
			Message: "SelectTimerTasksOrderByVisibilityTime operation failed. Not able to create query iterator.",
		}
	}

	// Estimate capacity to avoid reallocations
	estimated := pageSize
	if estimated < 32 {
		estimated = 32
	}
	
	var timers []*nosqlplugin.HistoryMigrationTask = make([]*nosqlplugin.HistoryMigrationTask, 0, estimated)
	task := taskMapPool.Get().(map[string]interface{})
	
	defer func() {
		// Clear and return to pool
		for k := range task {
			delete(task, k)
		}
		taskMapPool.Put(task)
	}()
	
	for iter.MapScan(task) {
		timerMap, ok := task["timer"].(map[string]interface{})
		if !ok {
			for k := range task {
				delete(task, k)
			}
			continue
		}
		
		t := parseTimerTaskInfo(timerMap)
		taskID, _ := task["task_id"].(int64)
		scheduledTime, _ := task["visibility_ts"].(time.Time)
		data, _ := task["data"].([]byte)
		encoding, _ := task["data_encoding"].(string)
		taskBlob := persistence.NewDataBlob(data, constants.EncodingType(encoding))

		timers = append(timers, &nosqlplugin.HistoryMigrationTask{
			Timer:         t,
			Task:          taskBlob,
			TaskID:        taskID,
			ScheduledTime: scheduledTime,
		})
		
		// Clear map for reuse
		for k := range task {
			delete(task, k)
		}
	}
	
	nextPageToken := getNextPageToken(iter)
	return timers, nextPageToken, iter.Close()
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

func (db *cdb) SelectReplicationTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	query := db.session.Query(templateGetReplicationTasksQuery,
		shardID,
		rowTypeReplicationTask,
		rowTypeReplicationDomainID,
		rowTypeReplicationWorkflowID,
		rowTypeReplicationRunID,
		defaultVisibilityTimestamp,
		inclusiveMinTaskID,
		exclusiveMaxTaskID,
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

func (db *cdb) InsertReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, replicationTask *nosqlplugin.HistoryMigrationTask) error {
	task := replicationTask.Replication
	taskBlob, taskEncoding := persistence.FromDataBlob(replicationTask.Task)
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
		taskBlob,
		taskEncoding,
		defaultVisibilityTimestamp,
		task.TaskID,
		task.CurrentTimeStamp,
	).WithContext(ctx)

	return query.Exec()
}

func (db *cdb) SelectReplicationDLQTasksOrderByTaskID(ctx context.Context, shardID int, sourceCluster string, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	query := db.session.Query(templateGetReplicationTasksQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		defaultVisibilityTimestamp,
		inclusiveMinTaskID,
		exclusiveMaxTaskID,
	).PageSize(pageSize).PageState(pageToken).WithContext(ctx)

	return populateGetReplicationTasks(query)
}

func (db *cdb) SelectReplicationDLQTasksCount(ctx context.Context, shardID int, sourceCluster string) (int64, error) {
	query := db.session.Query(templateGetDLQSizeQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
	).WithContext(ctx)

	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()
	
	if err := query.MapScan(result); err != nil {
		return -1, err
	}

	queueSize, ok := result["count"].(int64)
	if !ok {
		return -1, fmt.Errorf("failed to read queue count from result")
	}
	
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

func (db *cdb) RangeDeleteReplicationDLQTasks(ctx context.Context, shardID int, sourceCluster string, inclusiveBeginTaskID, exclusiveEndTaskID int64) error {
	query := db.session.Query(templateRangeCompleteReplicationTaskQuery,
		shardID,
		rowTypeDLQ,
		rowTypeDLQDomainID,
		sourceCluster,
		rowTypeDLQRunID,
		defaultVisibilityTimestamp,
		inclusiveBeginTaskID,
		exclusiveEndTaskID,
	).WithContext(ctx)

	return db.executeWithConsistencyAll(query)
}

func (db *cdb) InsertReplicationTask(ctx context.Context, tasks []*nosqlplugin.HistoryMigrationTask, shardCondition nosqlplugin.ShardCondition) error {
	if len(tasks) == 0 {
		return nil
	}

	shardID := shardCondition.ShardID

	// Dynamically batch replication tasks
	batchSize := dynamicBatchSize(len(tasks), 0)
	var batches []*gocql.Batch

	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(context.Background())
	timeStamp := tasks[0].Replication.CurrentTimeStamp
	count := 0
	
	for _, task := range tasks {
		createReplicationTasks(batch, shardID, task.Replication.DomainID, task.Replication.WorkflowID, []*nosqlplugin.HistoryMigrationTask{task}, timeStamp)
		count++
		
		if count >= batchSize {
			assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)
			batches = append(batches, batch)
			batch = db.session.NewBatch(gocql.LoggedBatch).WithContext(context.Background())
			count = 0
		}
	}
	
	if count > 0 && len(batch.Entries) > 0 {
		assertShardRangeID(batch, shardID, shardCondition.RangeID, timeStamp)
		batches = append(batches, batch)
	}

	// Reuse the same map for all batch executions to reduce allocations
	previous := resultMapPool.Get().(map[string]interface{})
	defer func() {
		// Clear and return to pool
		for k := range previous {
			delete(previous, k)
		}
		resultMapPool.Put(previous)
	}()
	
	for _, b := range batches {
		// Clear map before each use
		for k := range previous {
			delete(previous, k)
		}
		
		applied, iter, err := db.session.MapExecuteBatchCAS(b, previous)
		if iter != nil {
			defer iter.Close()
		}
		
		if err != nil {
			return err
		}
		
		if !applied {
			rowType, ok := previous["type"].(int)
			if !ok {
				panic("Encounter row type not found")
			}
			
			if rowType == rowTypeShard {
				if actualRangeID, ok := previous["range_id"].(int64); ok && actualRangeID != shardCondition.RangeID {
					return &nosqlplugin.ShardOperationConditionFailure{
						RangeID: actualRangeID,
					}
				}
			}
			
			var columns []string
			for k, v := range previous {
				columns = append(columns, fmt.Sprintf("%s=%v", k, v))
			}
			
			return &nosqlplugin.ShardOperationConditionFailure{
				RangeID: -1,
				Details: strings.Join(columns, ","),
			}
		}
	}
	
	return nil
}

func (db *cdb) SelectActiveClusterSelectionPolicy(ctx context.Context, shardID int, domainID, wfID, rID string) (*nosqlplugin.ActiveClusterSelectionPolicyRow, error) {
	query := db.session.Query(templateGetActiveClusterSelectionPolicyQuery,
		shardID,
		rowTypeWorkflowActiveClusterSelectionPolicy,
		domainID,
		wfID,
		rID,
		defaultVisibilityTimestamp,
		rowTypeWorkflowActiveClusterSelectionVersion,
	).WithContext(ctx)

	result := resultMapPool.Get().(map[string]interface{})
	defer func() {
		for k := range result {
			delete(result, k)
		}
		resultMapPool.Put(result)
	}()
	
	if err := query.MapScan(result); err != nil {
		if db.client.IsNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	policyData, _ := result["data"].([]byte)
	dataEncoding, _ := result["data_encoding"].(string)
	
	return &nosqlplugin.ActiveClusterSelectionPolicyRow{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: wfID,
		RunID:      rID,
		Policy:     persistence.NewDataBlob(policyData, constants.EncodingType(dataEncoding)),
	}, nil
}