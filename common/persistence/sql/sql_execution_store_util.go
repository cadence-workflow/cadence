// Copyright (c) 2017-2020 Uber Technologies, Inc.
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

package sql

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"time"

	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

func applyWorkflowMutationTx(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	workflowMutation *p.InternalWorkflowMutation,
	parser serialization.Parser,
	taskSerializer serialization.TaskSerializer,
) error {

	executionInfo := workflowMutation.ExecutionInfo
	versionHistories := workflowMutation.VersionHistories
	workflowChecksum := workflowMutation.ChecksumData
	startVersion := workflowMutation.StartVersion
	lastWriteVersion := workflowMutation.LastWriteVersion
	domainID := serialization.MustParseUUID(executionInfo.DomainID)
	workflowID := executionInfo.WorkflowID
	runID := serialization.MustParseUUID(executionInfo.RunID)

	// TODO Remove me if UPDATE holds the lock to the end of a transaction
	if err := lockAndCheckNextEventID(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID,
		workflowMutation.Condition); err != nil {
		return err
	}

	if err := updateExecution(
		ctx,
		tx,
		executionInfo,
		versionHistories,
		workflowChecksum,
		startVersion,
		lastWriteVersion,
		shardID,
		parser); err != nil {
		return err
	}

	if err := applyTasks(
		ctx,
		tx,
		shardID,
		workflowMutation.TasksByCategory,
		taskSerializer,
	); err != nil {
		return err
	}

	if err := updateActivityInfos(
		ctx,
		tx,
		workflowMutation.UpsertActivityInfos,
		workflowMutation.DeleteActivityInfos,
		shardID,
		domainID,
		workflowID,
		runID,
		parser,
	); err != nil {
		return err
	}

	if err := updateTimerInfos(
		ctx,
		tx,
		workflowMutation.UpsertTimerInfos,
		workflowMutation.DeleteTimerInfos,
		shardID,
		domainID,
		workflowID,
		runID,
		parser,
	); err != nil {
		return err
	}

	if err := updateChildExecutionInfos(
		ctx,
		tx,
		workflowMutation.UpsertChildExecutionInfos,
		workflowMutation.DeleteChildExecutionInfos,
		shardID,
		domainID,
		workflowID,
		runID,
		parser,
	); err != nil {
		return err
	}

	if err := updateRequestCancelInfos(
		ctx,
		tx,
		workflowMutation.UpsertRequestCancelInfos,
		workflowMutation.DeleteRequestCancelInfos,
		shardID,
		domainID,
		workflowID,
		runID,
		parser,
	); err != nil {
		return err
	}

	if err := updateSignalInfos(
		ctx,
		tx,
		workflowMutation.UpsertSignalInfos,
		workflowMutation.DeleteSignalInfos,
		shardID,
		domainID,
		workflowID,
		runID,
		parser,
	); err != nil {
		return err
	}

	if err := updateSignalsRequested(
		ctx,
		tx,
		workflowMutation.UpsertSignalRequestedIDs,
		workflowMutation.DeleteSignalRequestedIDs,
		shardID,
		domainID,
		workflowID,
		runID,
	); err != nil {
		return err
	}

	if workflowMutation.ClearBufferedEvents {
		if err := deleteBufferedEvents(
			ctx,
			tx,
			shardID,
			domainID,
			workflowID,
			runID,
		); err != nil {
			return err
		}
	}

	return updateBufferedEvents(
		ctx,
		tx,
		workflowMutation.NewBufferedEvents,
		shardID,
		domainID,
		workflowID,
		runID,
	)
}

func applyWorkflowSnapshotTxAsReset(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	workflowSnapshot *p.InternalWorkflowSnapshot,
	parser serialization.Parser,
	taskSerializer serialization.TaskSerializer,
) error {

	executionInfo := workflowSnapshot.ExecutionInfo
	versionHistories := workflowSnapshot.VersionHistories
	workflowChecksum := workflowSnapshot.ChecksumData
	startVersion := workflowSnapshot.StartVersion
	lastWriteVersion := workflowSnapshot.LastWriteVersion
	domainID := serialization.MustParseUUID(executionInfo.DomainID)
	workflowID := executionInfo.WorkflowID
	runID := serialization.MustParseUUID(executionInfo.RunID)

	// TODO Is there a way to modify the various map tables without fear of other people adding rows after we delete, without locking the executions row?
	if err := lockAndCheckNextEventID(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID,
		workflowSnapshot.Condition); err != nil {
		return err
	}

	if err := updateExecution(
		ctx,
		tx,
		executionInfo,
		versionHistories,
		workflowChecksum,
		startVersion,
		lastWriteVersion,
		shardID,
		parser); err != nil {
		return err
	}

	if err := applyTasks(
		ctx,
		tx,
		shardID,
		workflowSnapshot.TasksByCategory,
		taskSerializer,
	); err != nil {
		return err
	}

	if err := deleteActivityInfoMap(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateActivityInfos(
		ctx,
		tx,
		workflowSnapshot.ActivityInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := deleteTimerInfoMap(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateTimerInfos(
		ctx,
		tx,
		workflowSnapshot.TimerInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := deleteChildExecutionInfoMap(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateChildExecutionInfos(
		ctx,
		tx,
		workflowSnapshot.ChildExecutionInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := deleteRequestCancelInfoMap(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateRequestCancelInfos(
		ctx,
		tx,
		workflowSnapshot.RequestCancelInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := deleteSignalInfoMap(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateSignalInfos(
		ctx,
		tx,
		workflowSnapshot.SignalInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := deleteSignalsRequestedSet(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	if err := updateSignalsRequested(
		ctx,
		tx,
		workflowSnapshot.SignalRequestedIDs,
		nil,
		shardID,
		domainID,
		workflowID,
		runID); err != nil {
		return err
	}

	return deleteBufferedEvents(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID)
}

func applyWorkflowSnapshotTxAsNew(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	workflowSnapshot *p.InternalWorkflowSnapshot,
	parser serialization.Parser,
	taskSerializer serialization.TaskSerializer,
) error {

	executionInfo := workflowSnapshot.ExecutionInfo
	versionHistories := workflowSnapshot.VersionHistories
	workflowChecksum := workflowSnapshot.ChecksumData
	startVersion := workflowSnapshot.StartVersion
	lastWriteVersion := workflowSnapshot.LastWriteVersion
	domainID := serialization.MustParseUUID(executionInfo.DomainID)
	workflowID := executionInfo.WorkflowID
	runID := serialization.MustParseUUID(executionInfo.RunID)

	// TODO(active-active): store active cluster selection policy row. It requires a new table in sql DB schemas.

	if err := createExecution(
		ctx,
		tx,
		executionInfo,
		versionHistories,
		workflowChecksum,
		startVersion,
		lastWriteVersion,
		shardID,
		parser); err != nil {
		return err
	}

	if err := applyTasks(
		ctx,
		tx,
		shardID,
		workflowSnapshot.TasksByCategory,
		taskSerializer,
	); err != nil {
		return err
	}

	if err := updateActivityInfos(
		ctx,
		tx,
		workflowSnapshot.ActivityInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := updateTimerInfos(
		ctx,
		tx,
		workflowSnapshot.TimerInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := updateChildExecutionInfos(
		ctx,
		tx,
		workflowSnapshot.ChildExecutionInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := updateRequestCancelInfos(
		ctx,
		tx,
		workflowSnapshot.RequestCancelInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	if err := updateSignalInfos(
		ctx,
		tx,
		workflowSnapshot.SignalInfos,
		nil,
		shardID,
		domainID,
		workflowID,
		runID,
		parser); err != nil {
		return err
	}

	return updateSignalsRequested(
		ctx,
		tx,
		workflowSnapshot.SignalRequestedIDs,
		nil,
		shardID,
		domainID,
		workflowID,
		runID)
}

func applyTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	tasksByCategory map[p.HistoryTaskCategory][]p.Task,
	taskSerializer serialization.TaskSerializer,
) error {
	var err error
	for c, tasks := range tasksByCategory {
		switch c.Type() {
		case p.HistoryTaskCategoryTypeImmediate:
			err = createImmediateTasks(ctx, tx, shardID, c.ID(), tasks, taskSerializer)
		case p.HistoryTaskCategoryTypeScheduled:
			err = createScheduledTasks(ctx, tx, shardID, c.ID(), tasks, taskSerializer)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// lockCurrentExecutionIfExists returns current execution or nil if none is found for the workflowID
// locking it in the DB
func lockCurrentExecutionIfExists(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
) (*sqlplugin.CurrentExecutionsRow, error) {

	rows, err := tx.LockCurrentExecutionsJoinExecutions(ctx, &sqlplugin.CurrentExecutionsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, convertCommonErrors(tx, "lockCurrentExecutionIfExists", fmt.Sprintf("Failed to get current_executions row for (shard,domain,workflow) = (%v, %v, %v).", shardID, domainID, workflowID), err)
		}
	}
	size := len(rows)
	if size > 1 {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("lockCurrentExecutionIfExists failed. Multiple current_executions rows for (shard,domain,workflow) = (%v, %v, %v).", shardID, domainID, workflowID),
		}
	}
	if size == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func createOrUpdateCurrentExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	createMode p.CreateWorkflowMode,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	state int,
	closeStatus int,
	createRequestID string,
	startVersion int64,
	lastWriteVersion int64,
) error {

	row := sqlplugin.CurrentExecutionsRow{
		ShardID:          int64(shardID),
		DomainID:         domainID,
		WorkflowID:       workflowID,
		RunID:            runID,
		CreateRequestID:  createRequestID,
		State:            state,
		CloseStatus:      closeStatus,
		StartVersion:     startVersion,
		LastWriteVersion: lastWriteVersion,
	}

	switch createMode {
	case p.CreateWorkflowModeContinueAsNew,
		p.CreateWorkflowModeWorkflowIDReuse:
		if err := updateCurrentExecution(
			ctx,
			tx,
			shardID,
			domainID,
			workflowID,
			runID,
			createRequestID,
			state,
			closeStatus,
			row.StartVersion,
			row.LastWriteVersion); err != nil {
			return err
		}
	case p.CreateWorkflowModeBrandNew:
		if _, err := tx.InsertIntoCurrentExecutions(ctx, &row); err != nil {
			return convertCommonErrors(tx, "createOrUpdateCurrentExecution", "Failed to insert into current_executions table.", err)
		}
	case p.CreateWorkflowModeZombie:
		// noop
	default:
		return fmt.Errorf("createOrUpdateCurrentExecution failed. Unknown workflow creation mode: %v", createMode)
	}

	return nil
}

func lockAndCheckNextEventID(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	condition int64,
) error {

	nextEventID, err := lockNextEventID(
		ctx,
		tx,
		shardID,
		domainID,
		workflowID,
		runID,
	)

	if err != nil {
		return err
	}
	if *nextEventID != condition {
		return &p.ConditionFailedError{
			Msg: fmt.Sprintf("lockAndCheckNextEventID failed. Next_event_id was %v when it should have been %v.", nextEventID, condition),
		}
	}
	return nil
}

func lockNextEventID(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) (*int64, error) {

	nextEventID, err := tx.WriteLockExecutions(ctx, &sqlplugin.ExecutionsFilter{
		ShardID:    shardID,
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf(
					"lockNextEventID failed. Unable to lock executions row with (shard, domain, workflow, run) = (%v,%v,%v,%v) which does not exist.",
					shardID,
					domainID,
					workflowID,
					runID,
				),
			}
		}
		return nil, convertCommonErrors(tx, "lockNextEventID", "", err)
	}
	result := int64(nextEventID)
	return &result, nil
}

func createImmediateTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	categoryID int,
	tasks []p.Task,
	taskSerializer serialization.TaskSerializer,
) error {
	switch categoryID {
	case p.HistoryTaskCategoryIDTransfer:
		return createTransferTasks(ctx, tx, tasks, shardID, taskSerializer)
	case p.HistoryTaskCategoryIDReplication:
		return createReplicationTasks(ctx, tx, tasks, shardID, taskSerializer)
	}
	// TODO: implement creating tasks for other categories
	return nil
}

func createScheduledTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	categoryID int,
	tasks []p.Task,
	taskSerializer serialization.TaskSerializer,
) error {
	switch categoryID {
	case p.HistoryTaskCategoryIDTimer:
		return createTimerTasks(ctx, tx, tasks, shardID, taskSerializer)
	}
	// TODO: implement creating tasks for other categories
	return nil
}

func createTransferTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	transferTasks []p.Task,
	shardID int,
	taskSerializer serialization.TaskSerializer,
) error {

	if len(transferTasks) == 0 {
		return nil
	}

	transferTasksRows := make([]sqlplugin.TransferTasksRow, len(transferTasks))
	for i, task := range transferTasks {
		blob, err := taskSerializer.SerializeTask(p.HistoryTaskCategoryTransfer, task)
		if err != nil {
			return err
		}
		transferTasksRows[i].ShardID = shardID
		transferTasksRows[i].TaskID = task.GetTaskID()
		transferTasksRows[i].Data = blob.Data
		transferTasksRows[i].DataEncoding = string(blob.Encoding)
	}

	result, err := tx.InsertIntoTransferTasks(ctx, transferTasksRows)
	if err != nil {
		return convertCommonErrors(tx, "createTransferTasks", "", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createTransferTasks failed. Could not verify number of rows inserted. Error: %v", err),
		}
	}

	if int(rowsAffected) != len(transferTasks) {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createTransferTasks failed. Inserted %v instead of %v rows into transfer_tasks. Error: %v", rowsAffected, len(transferTasks), err),
		}
	}

	return nil
}

func createReplicationTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	replicationTasks []p.Task,
	shardID int,
	taskSerializer serialization.TaskSerializer,
) error {

	if len(replicationTasks) == 0 {
		return nil
	}
	replicationTasksRows := make([]sqlplugin.ReplicationTasksRow, len(replicationTasks))

	for i, task := range replicationTasks {
		blob, err := taskSerializer.SerializeTask(p.HistoryTaskCategoryReplication, task)
		if err != nil {
			return err
		}
		replicationTasksRows[i].ShardID = shardID
		replicationTasksRows[i].TaskID = task.GetTaskID()
		replicationTasksRows[i].Data = blob.Data
		replicationTasksRows[i].DataEncoding = string(blob.Encoding)
	}

	result, err := tx.InsertIntoReplicationTasks(ctx, replicationTasksRows)
	if err != nil {
		return convertCommonErrors(tx, "createReplicationTasks", "", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createReplicationTasks failed. Could not verify number of rows inserted. Error: %v", err),
		}
	}

	if int(rowsAffected) != len(replicationTasks) {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createReplicationTasks failed. Inserted %v instead of %v rows into transfer_tasks. Error: %v", rowsAffected, len(replicationTasks), err),
		}
	}

	return nil
}

func createTimerTasks(
	ctx context.Context,
	tx sqlplugin.Tx,
	timerTasks []p.Task,
	shardID int,
	taskSerializer serialization.TaskSerializer,
) error {

	if len(timerTasks) == 0 {
		return nil
	}

	timerTasksRows := make([]sqlplugin.TimerTasksRow, len(timerTasks))

	for i, task := range timerTasks {
		blob, err := taskSerializer.SerializeTask(p.HistoryTaskCategoryTimer, task)
		if err != nil {
			return err
		}
		timerTasksRows[i].ShardID = shardID
		timerTasksRows[i].VisibilityTimestamp = task.GetVisibilityTimestamp()
		timerTasksRows[i].TaskID = task.GetTaskID()
		timerTasksRows[i].Data = blob.Data
		timerTasksRows[i].DataEncoding = string(blob.Encoding)
	}

	result, err := tx.InsertIntoTimerTasks(ctx, timerTasksRows)
	if err != nil {
		return convertCommonErrors(tx, "createTimerTasks", "", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createTimerTasks failed. Could not verify number of rows inserted. Error: %v", err),
		}
	}

	if int(rowsAffected) != len(timerTasks) {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createTimerTasks failed. Inserted %v instead of %v rows into timer_tasks.", rowsAffected, len(timerTasks)),
		}
	}

	return nil
}

func assertNotCurrentExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
) error {
	currentRow, err := tx.LockCurrentExecutions(ctx, &sqlplugin.CurrentExecutionsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			// allow bypassing no current record
			return nil
		}
		return convertCommonErrors(tx, "assertCurrentExecution", "Unable to load current record.", err)
	}
	return assertRunIDMismatch(runID, currentRow.RunID)
}

func assertRunIDAndUpdateCurrentExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	newRunID serialization.UUID,
	previousRunID serialization.UUID,
	createRequestID string,
	state int,
	closeStatus int,
	startVersion int64,
	lastWriteVersion int64,
) error {

	assertFn := func(currentRow *sqlplugin.CurrentExecutionsRow) error {
		if !bytes.Equal(currentRow.RunID, previousRunID) {
			return &p.ConditionFailedError{Msg: fmt.Sprintf(
				"assertRunIDAndUpdateCurrentExecution failed. Current run ID was %v, expected %v",
				currentRow.RunID,
				previousRunID,
			)}
		}
		return nil
	}
	if err := assertCurrentExecution(ctx, tx, shardID, domainID, workflowID, assertFn); err != nil {
		return err
	}

	return updateCurrentExecution(ctx, tx, shardID, domainID, workflowID, newRunID, createRequestID, state, closeStatus, startVersion, lastWriteVersion)
}

func assertCurrentExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	assertFn func(currentRow *sqlplugin.CurrentExecutionsRow) error,
) error {

	currentRow, err := tx.LockCurrentExecutions(ctx, &sqlplugin.CurrentExecutionsFilter{
		ShardID:    int64(shardID),
		DomainID:   domainID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return convertCommonErrors(tx, "assertCurrentExecution", "Unable to load current record.", err)
	}
	return assertFn(currentRow)
}

func assertRunIDMismatch(runID serialization.UUID, currentRunID serialization.UUID) error {
	// zombie workflow creation with existence of current record, this is a noop
	if bytes.Equal(currentRunID, runID) {
		return &p.ConditionFailedError{Msg: fmt.Sprintf(
			"assertRunIDMismatch failed. Current run ID was %v, input %v",
			currentRunID,
			runID,
		)}
	}
	return nil
}

func updateCurrentExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	shardID int,
	domainID serialization.UUID,
	workflowID string,
	runID serialization.UUID,
	createRequestID string,
	state int,
	closeStatus int,
	startVersion int64,
	lastWriteVersion int64,
) error {

	result, err := tx.UpdateCurrentExecutions(ctx, &sqlplugin.CurrentExecutionsRow{
		ShardID:          int64(shardID),
		DomainID:         domainID,
		WorkflowID:       workflowID,
		RunID:            runID,
		CreateRequestID:  createRequestID,
		State:            state,
		CloseStatus:      closeStatus,
		StartVersion:     startVersion,
		LastWriteVersion: lastWriteVersion,
	})
	if err != nil {
		return convertCommonErrors(tx, "updateCurrentExecution", "", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("updateCurrentExecution failed. Failed to check number of rows updated in current_executions table. Error: %v", err),
		}
	}
	if rowsAffected != 1 {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("updateCurrentExecution failed. %v rows of current_executions updated instead of 1.", rowsAffected),
		}
	}
	return nil
}

func buildExecutionRow(
	executionInfo *p.InternalWorkflowExecutionInfo,
	versionHistories *p.DataBlob,
	workflowChecksum *p.DataBlob,
	startVersion int64,
	lastWriteVersion int64,
	shardID int,
	parser serialization.Parser,
) (row *sqlplugin.ExecutionsRow, err error) {

	info := serialization.FromInternalWorkflowExecutionInfo(executionInfo)

	info.StartVersion = startVersion
	if versionHistories == nil {
		// this is allowed
	} else {
		info.VersionHistories = versionHistories.Data
		info.VersionHistoriesEncoding = string(versionHistories.GetEncoding())
	}
	if workflowChecksum != nil {
		info.Checksum = workflowChecksum.Data
		info.ChecksumEncoding = string(workflowChecksum.GetEncoding())
	}

	blob, err := parser.WorkflowExecutionInfoToBlob(info)
	if err != nil {
		return nil, err
	}

	return &sqlplugin.ExecutionsRow{
		ShardID:          shardID,
		DomainID:         serialization.MustParseUUID(executionInfo.DomainID),
		WorkflowID:       executionInfo.WorkflowID,
		RunID:            serialization.MustParseUUID(executionInfo.RunID),
		NextEventID:      int64(executionInfo.NextEventID),
		LastWriteVersion: lastWriteVersion,
		Data:             blob.Data,
		DataEncoding:     string(blob.Encoding),
	}, nil
}

func createExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	executionInfo *p.InternalWorkflowExecutionInfo,
	versionHistories *p.DataBlob,
	workflowChecksum *p.DataBlob,
	startVersion int64,
	lastWriteVersion int64,
	shardID int,
	parser serialization.Parser,
) error {

	// validate workflow state & close status
	if err := p.ValidateCreateWorkflowStateCloseStatus(
		executionInfo.State,
		executionInfo.CloseStatus); err != nil {
		return err
	}

	now := time.Now()
	// TODO: this case seems to be always false
	if executionInfo.StartTimestamp.IsZero() {
		executionInfo.StartTimestamp = now
	}

	row, err := buildExecutionRow(
		executionInfo,
		versionHistories,
		workflowChecksum,
		startVersion,
		lastWriteVersion,
		shardID,
		parser,
	)
	if err != nil {
		return err
	}
	result, err := tx.InsertIntoExecutions(ctx, row)
	if err != nil {
		if tx.IsDupEntryError(err) {
			return &p.WorkflowExecutionAlreadyStartedError{
				Msg:              fmt.Sprintf("Workflow execution already running. WorkflowId: %v", executionInfo.WorkflowID),
				StartRequestID:   executionInfo.CreateRequestID,
				RunID:            executionInfo.RunID,
				State:            executionInfo.State,
				CloseStatus:      executionInfo.CloseStatus,
				LastWriteVersion: row.LastWriteVersion,
			}
		}
		return convertCommonErrors(tx, "createExecution", "", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("createExecution failed. Failed to verify number of rows affected. Erorr: %v", err),
		}
	}
	if rowsAffected != 1 {
		return &types.EntityNotExistsError{
			Message: fmt.Sprintf("createExecution failed. Affected %v rows updated instead of 1.", rowsAffected),
		}
	}

	return nil
}

func updateExecution(
	ctx context.Context,
	tx sqlplugin.Tx,
	executionInfo *p.InternalWorkflowExecutionInfo,
	versionHistories *p.DataBlob,
	workflowChecksum *p.DataBlob,
	startVersion int64,
	lastWriteVersion int64,
	shardID int,
	parser serialization.Parser,
) error {

	// validate workflow state & close status
	if err := p.ValidateUpdateWorkflowStateCloseStatus(
		executionInfo.State,
		executionInfo.CloseStatus); err != nil {
		return err
	}

	row, err := buildExecutionRow(
		executionInfo,
		versionHistories,
		workflowChecksum,
		startVersion,
		lastWriteVersion,
		shardID,
		parser,
	)
	if err != nil {
		return err
	}
	result, err := tx.UpdateExecutions(ctx, row)
	if err != nil {
		return convertCommonErrors(tx, "updateExecution", "", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("updateExecution failed. Failed to verify number of rows affected. Erorr: %v", err),
		}
	}
	if rowsAffected != 1 {
		return &types.EntityNotExistsError{
			Message: fmt.Sprintf("updateExecution failed. Affected %v rows updated instead of 1.", rowsAffected),
		}
	}

	return nil
}
