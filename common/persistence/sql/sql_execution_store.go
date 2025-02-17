// Copyright (c) 2018 Uber Technologies, Inc.
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
	"encoding/json"
	"fmt"
	"math"
	"runtime/debug"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	emptyWorkflowID       string = ""
	emptyReplicationRunID string = "30000000-5000-f000-f000-000000000000"
)

type sqlExecutionStore struct {
	sqlStore
	shardID                                int
	txExecuteShardLockedFn                 func(context.Context, int, string, int64, func(sqlplugin.Tx) error) error
	lockCurrentExecutionIfExistsFn         func(context.Context, sqlplugin.Tx, int, serialization.UUID, string) (*sqlplugin.CurrentExecutionsRow, error)
	createOrUpdateCurrentExecutionFn       func(context.Context, sqlplugin.Tx, p.CreateWorkflowMode, int, serialization.UUID, string, serialization.UUID, int, int, string, int64, int64) error
	assertNotCurrentExecutionFn            func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID) error
	assertRunIDAndUpdateCurrentExecutionFn func(context.Context, sqlplugin.Tx, int, serialization.UUID, string, serialization.UUID, serialization.UUID, string, int, int, int64, int64) error
	applyWorkflowSnapshotTxAsNewFn         func(context.Context, sqlplugin.Tx, int, *p.InternalWorkflowSnapshot, serialization.Parser) error
	applyWorkflowMutationTxFn              func(context.Context, sqlplugin.Tx, int, *p.InternalWorkflowMutation, serialization.Parser) error
	applyWorkflowSnapshotTxAsResetFn       func(context.Context, sqlplugin.Tx, int, *p.InternalWorkflowSnapshot, serialization.Parser) error
}

var _ p.ExecutionStore = (*sqlExecutionStore)(nil)

// NewSQLExecutionStore creates an instance of ExecutionStore
func NewSQLExecutionStore(
	db sqlplugin.DB,
	logger log.Logger,
	shardID int,
	parser serialization.Parser,
	dc *p.DynamicConfiguration,
) (p.ExecutionStore, error) {

	store := &sqlExecutionStore{
		shardID:                                shardID,
		lockCurrentExecutionIfExistsFn:         lockCurrentExecutionIfExists,
		createOrUpdateCurrentExecutionFn:       createOrUpdateCurrentExecution,
		assertNotCurrentExecutionFn:            assertNotCurrentExecution,
		assertRunIDAndUpdateCurrentExecutionFn: assertRunIDAndUpdateCurrentExecution,
		applyWorkflowSnapshotTxAsNewFn:         applyWorkflowSnapshotTxAsNew,
		applyWorkflowMutationTxFn:              applyWorkflowMutationTx,
		applyWorkflowSnapshotTxAsResetFn:       applyWorkflowSnapshotTxAsReset,
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
			parser: parser,
			dc:     dc,
		},
	}
	store.txExecuteShardLockedFn = store.txExecuteShardLocked
	return store, nil
}

// txExecuteShardLocked executes f under transaction and with read lock on shard row
func (m *sqlExecutionStore) txExecuteShardLocked(
	ctx context.Context,
	dbShardID int,
	operation string,
	rangeID int64,
	fn func(tx sqlplugin.Tx) error,
) error {

	return m.txExecute(ctx, dbShardID, operation, func(tx sqlplugin.Tx) error {
		if err := readLockShard(ctx, tx, m.shardID, rangeID); err != nil {
			return err
		}
		err := fn(tx)
		if err != nil {
			return err
		}
		return nil
	})
}

func (m *sqlExecutionStore) GetShardID() int {
	return m.shardID
}

func (m *sqlExecutionStore) CreateWorkflowExecution(
	ctx context.Context,
	request *p.InternalCreateWorkflowExecutionRequest,
) (response *p.CreateWorkflowExecutionResponse, err error) {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(m.shardID, m.db.GetTotalNumDBShards())

	err = m.txExecuteShardLockedFn(ctx, dbShardID, "CreateWorkflowExecution", request.RangeID, func(tx sqlplugin.Tx) error {
		response, err = m.createWorkflowExecutionTx(ctx, tx, request)
		return err
	})
	return
}

func (m *sqlExecutionStore) createWorkflowExecutionTx(
	ctx context.Context,
	tx sqlplugin.Tx,
	request *p.InternalCreateWorkflowExecutionRequest,
) (*p.CreateWorkflowExecutionResponse, error) {

	newWorkflow := request.NewWorkflowSnapshot
	executionInfo := newWorkflow.ExecutionInfo
	startVersion := newWorkflow.StartVersion
	lastWriteVersion := newWorkflow.LastWriteVersion
	shardID := m.shardID
	domainID := serialization.MustParseUUID(executionInfo.DomainID)
	workflowID := executionInfo.WorkflowID
	runID := serialization.MustParseUUID(executionInfo.RunID)

	if err := p.ValidateCreateWorkflowModeState(
		request.Mode,
		newWorkflow,
	); err != nil {
		return nil, err
	}

	var err error
	var row *sqlplugin.CurrentExecutionsRow
	if row, err = m.lockCurrentExecutionIfExistsFn(ctx, tx, m.shardID, domainID, workflowID); err != nil {
		return nil, err
	}

	// current workflow record check
	if row != nil {
		// current run ID, last write version, current workflow state check
		switch request.Mode {
		case p.CreateWorkflowModeBrandNew:
			return nil, &p.WorkflowExecutionAlreadyStartedError{
				Msg:              fmt.Sprintf("Workflow execution already running. WorkflowId: %v", row.WorkflowID),
				StartRequestID:   row.CreateRequestID,
				RunID:            row.RunID.String(),
				State:            int(row.State),
				CloseStatus:      int(row.CloseStatus),
				LastWriteVersion: row.LastWriteVersion,
			}

		case p.CreateWorkflowModeWorkflowIDReuse:
			if request.PreviousLastWriteVersion != row.LastWriteVersion {
				return nil, &p.CurrentWorkflowConditionFailedError{
					Msg: fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, "+
						"LastWriteVersion: %v, PreviousLastWriteVersion: %v",
						workflowID, row.LastWriteVersion, request.PreviousLastWriteVersion),
				}
			}
			if row.State != p.WorkflowStateCompleted {
				return nil, &p.CurrentWorkflowConditionFailedError{
					Msg: fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, "+
						"State: %v, Expected: %v",
						workflowID, row.State, p.WorkflowStateCompleted),
				}
			}
			runIDStr := row.RunID.String()
			if runIDStr != request.PreviousRunID {
				return nil, &p.CurrentWorkflowConditionFailedError{
					Msg: fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, "+
						"RunID: %v, PreviousRunID: %v",
						workflowID, runIDStr, request.PreviousRunID),
				}
			}

		case p.CreateWorkflowModeZombie:
			// zombie workflow creation with existence of current record, this is a noop
			if err := assertRunIDMismatch(serialization.MustParseUUID(executionInfo.RunID), row.RunID); err != nil {
				return nil, err
			}

		case p.CreateWorkflowModeContinueAsNew:
			runIDStr := row.RunID.String()
			if runIDStr != request.PreviousRunID {
				return nil, &p.CurrentWorkflowConditionFailedError{
					Msg: fmt.Sprintf("Workflow execution creation condition failed. WorkflowId: %v, "+
						"RunID: %v, PreviousRunID: %v",
						workflowID, runIDStr, request.PreviousRunID),
				}
			}

		default:
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf(
					"CreteWorkflowExecution: unknown mode: %v",
					request.Mode,
				),
			}
		}
	}

	if err := m.createOrUpdateCurrentExecutionFn(
		ctx,
		tx,
		request.Mode,
		m.shardID,
		domainID,
		workflowID,
		runID,
		executionInfo.State,
		executionInfo.CloseStatus,
		executionInfo.CreateRequestID,
		startVersion,
		lastWriteVersion); err != nil {
		return nil, err
	}

	if err := m.applyWorkflowSnapshotTxAsNewFn(ctx, tx, shardID, &request.NewWorkflowSnapshot, m.parser); err != nil {
		return nil, err
	}

	return &p.CreateWorkflowExecutionResponse{}, nil
}

func (m *sqlExecutionStore) getExecutions(
	ctx context.Context,
	request *p.InternalGetWorkflowExecutionRequest,
	domainID serialization.UUID,
	wfID string,
	runID serialization.UUID,
) ([]sqlplugin.ExecutionsRow, error) {
	executions, err := m.db.SelectFromExecutions(ctx, &sqlplugin.ExecutionsFilter{
		ShardID: m.shardID, DomainID: domainID, WorkflowID: wfID, RunID: runID})

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf(
					"Workflow execution not found.  WorkflowId: %v, RunId: %v",
					request.Execution.GetWorkflowID(),
					request.Execution.GetRunID(),
				),
			}
		}
		return nil, convertCommonErrors(m.db, "GetWorkflowExecution", "", err)
	}

	if len(executions) == 0 {
		return nil, &types.EntityNotExistsError{
			Message: fmt.Sprintf(
				"Workflow execution not found.  WorkflowId: %v, RunId: %v",
				request.Execution.GetWorkflowID(),
				request.Execution.GetRunID(),
			),
		}
	}

	if len(executions) != 1 {
		return nil, &types.InternalServiceError{
			Message: "GetWorkflowExecution return more than one results.",
		}
	}
	return executions, nil
}

func (m *sqlExecutionStore) GetWorkflowExecution(
	ctx context.Context,
	request *p.InternalGetWorkflowExecutionRequest,
) (resp *p.InternalGetWorkflowExecutionResponse, e error) {
	recoverPanic := func(recovered interface{}, err *error) {
		if recovered != nil {
			*err = fmt.Errorf("DB operation panicked: %v %s", recovered, debug.Stack())
		}
	}

	domainID := serialization.MustParseUUID(request.DomainID)
	runID := serialization.MustParseUUID(request.Execution.RunID)
	wfID := request.Execution.WorkflowID

	var executions []sqlplugin.ExecutionsRow
	var activityInfos map[int64]*p.InternalActivityInfo
	var timerInfos map[string]*p.TimerInfo
	var childExecutionInfos map[int64]*p.InternalChildExecutionInfo
	var requestCancelInfos map[int64]*p.RequestCancelInfo
	var signalInfos map[int64]*p.SignalInfo
	var bufferedEvents []*p.DataBlob
	var signalsRequested map[string]struct{}

	g, childCtx := errgroup.WithContext(ctx)

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		activityInfos, e = getActivityInfoMap(
			childCtx, m.db, m.shardID, domainID, wfID, runID, m.parser)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		timerInfos, e = getTimerInfoMap(
			childCtx, m.db, m.shardID, domainID, wfID, runID, m.parser)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		childExecutionInfos, e = getChildExecutionInfoMap(
			childCtx, m.db, m.shardID, domainID, wfID, runID, m.parser)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		requestCancelInfos, e = getRequestCancelInfoMap(
			childCtx, m.db, m.shardID, domainID, wfID, runID, m.parser)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		signalInfos, e = getSignalInfoMap(
			childCtx, m.db, m.shardID, domainID, wfID, runID, m.parser)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		bufferedEvents, e = getBufferedEvents(
			childCtx, m.db, m.shardID, domainID, wfID, runID)
		return e
	})

	g.Go(func() (e error) {
		defer func() { recoverPanic(recover(), &e) }()
		signalsRequested, e = getSignalsRequested(
			childCtx, m.db, m.shardID, domainID, wfID, runID)
		return e
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	// there is a race condition with delete workflow. What could happen is that
	// a delete workflow transaction can be committed between 2 concurrent read operations
	// and in that case we can get checksum error because data is partially read.
	// Since checksum is stored in the executions table, we make it the last step of reading,
	// in this case, either we read full data with checksum or we don't get checksum and return error.
	executions, err = m.getExecutions(ctx, request, domainID, wfID, runID)
	if err != nil {
		return nil, err
	}

	state, err := m.populateWorkflowMutableState(executions[0])
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("GetWorkflowExecution: failed. Error: %v", err),
		}
	}
	// if we have checksum, we need to make sure the rangeID did not change
	// if the rangeID changed, it means the shard ownership might have changed
	// and the workflow might have been updated when we read the data, so the data
	// we read might not be from a consistent view, the checksum validation might fail
	// in that case, we clear the checksum data so that we will not perform the validation
	if state.ChecksumData != nil {
		row, err := m.db.SelectFromShards(ctx, &sqlplugin.ShardsFilter{ShardID: int64(m.shardID)})
		if err != nil {
			return nil, convertCommonErrors(m.db, "GetWorkflowExecution", "", err)
		}
		if row.RangeID != request.RangeID {
			// The GetWorkflowExecution operation will not be impacted by this. ChecksumData is purely for validation purposes.
			m.logger.Warn("GetWorkflowExecution's checksum is discarded. The shard might have changed owner.")
			state.ChecksumData = nil
		}
	}

	state.ActivityInfos = activityInfos
	state.TimerInfos = timerInfos
	state.ChildExecutionInfos = childExecutionInfos
	state.RequestCancelInfos = requestCancelInfos
	state.SignalInfos = signalInfos
	state.BufferedEvents = bufferedEvents
	state.SignalRequestedIDs = signalsRequested

	return &p.InternalGetWorkflowExecutionResponse{State: state}, nil
}

func (m *sqlExecutionStore) UpdateWorkflowExecution(
	ctx context.Context,
	request *p.InternalUpdateWorkflowExecutionRequest,
) error {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(m.shardID, m.db.GetTotalNumDBShards())
	return m.txExecuteShardLockedFn(ctx, dbShardID, "UpdateWorkflowExecution", request.RangeID, func(tx sqlplugin.Tx) error {
		return m.updateWorkflowExecutionTx(ctx, tx, request)
	})
}

func (m *sqlExecutionStore) updateWorkflowExecutionTx(
	ctx context.Context,
	tx sqlplugin.Tx,
	request *p.InternalUpdateWorkflowExecutionRequest,
) error {

	updateWorkflow := request.UpdateWorkflowMutation
	newWorkflow := request.NewWorkflowSnapshot

	executionInfo := updateWorkflow.ExecutionInfo
	domainID := serialization.MustParseUUID(executionInfo.DomainID)
	workflowID := executionInfo.WorkflowID
	runID := serialization.MustParseUUID(executionInfo.RunID)
	shardID := m.shardID

	if err := p.ValidateUpdateWorkflowModeState(
		request.Mode,
		updateWorkflow,
		newWorkflow,
	); err != nil {
		return err
	}

	switch request.Mode {
	case p.UpdateWorkflowModeIgnoreCurrent:
		// no-op
	case p.UpdateWorkflowModeBypassCurrent:
		if err := m.assertNotCurrentExecutionFn(
			ctx,
			tx,
			shardID,
			domainID,
			workflowID,
			runID); err != nil {
			return err
		}

	case p.UpdateWorkflowModeUpdateCurrent:
		if newWorkflow != nil {
			newExecutionInfo := newWorkflow.ExecutionInfo
			startVersion := newWorkflow.StartVersion
			lastWriteVersion := newWorkflow.LastWriteVersion
			newDomainID := serialization.MustParseUUID(newExecutionInfo.DomainID)
			newRunID := serialization.MustParseUUID(newExecutionInfo.RunID)

			if !bytes.Equal(domainID, newDomainID) {
				return &types.InternalServiceError{
					Message: "UpdateWorkflowExecution: cannot continue as new to another domain",
				}
			}

			if err := m.assertRunIDAndUpdateCurrentExecutionFn(
				ctx,
				tx,
				shardID,
				domainID,
				workflowID,
				newRunID,
				runID,
				newWorkflow.ExecutionInfo.CreateRequestID,
				newWorkflow.ExecutionInfo.State,
				newWorkflow.ExecutionInfo.CloseStatus,
				startVersion,
				lastWriteVersion); err != nil {
				return err
			}
		} else {
			startVersion := updateWorkflow.StartVersion
			lastWriteVersion := updateWorkflow.LastWriteVersion
			// this is only to update the current record
			if err := m.assertRunIDAndUpdateCurrentExecutionFn(
				ctx,
				tx,
				shardID,
				domainID,
				workflowID,
				runID,
				runID,
				executionInfo.CreateRequestID,
				executionInfo.State,
				executionInfo.CloseStatus,
				startVersion,
				lastWriteVersion); err != nil {
				return err
			}
		}

	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("UpdateWorkflowExecution: unknown mode: %v", request.Mode),
		}
	}

	if err := m.applyWorkflowMutationTxFn(ctx, tx, shardID, &updateWorkflow, m.parser); err != nil {
		return err
	}
	if newWorkflow != nil {
		if err := m.applyWorkflowSnapshotTxAsNewFn(ctx, tx, shardID, newWorkflow, m.parser); err != nil {
			return err
		}
	}
	return nil
}

func (m *sqlExecutionStore) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *p.InternalConflictResolveWorkflowExecutionRequest,
) error {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(m.shardID, m.db.GetTotalNumDBShards())
	return m.txExecuteShardLockedFn(ctx, dbShardID, "ConflictResolveWorkflowExecution", request.RangeID, func(tx sqlplugin.Tx) error {
		return m.conflictResolveWorkflowExecutionTx(ctx, tx, request)
	})
}

func (m *sqlExecutionStore) conflictResolveWorkflowExecutionTx(
	ctx context.Context,
	tx sqlplugin.Tx,
	request *p.InternalConflictResolveWorkflowExecutionRequest,
) error {

	currentWorkflow := request.CurrentWorkflowMutation
	resetWorkflow := request.ResetWorkflowSnapshot
	newWorkflow := request.NewWorkflowSnapshot

	shardID := m.shardID

	domainID := serialization.MustParseUUID(resetWorkflow.ExecutionInfo.DomainID)
	workflowID := resetWorkflow.ExecutionInfo.WorkflowID

	if err := p.ValidateConflictResolveWorkflowModeState(
		request.Mode,
		resetWorkflow,
		newWorkflow,
		currentWorkflow,
	); err != nil {
		return err
	}

	switch request.Mode {
	case p.ConflictResolveWorkflowModeBypassCurrent:
		if err := m.assertNotCurrentExecutionFn(
			ctx,
			tx,
			shardID,
			domainID,
			workflowID,
			serialization.MustParseUUID(resetWorkflow.ExecutionInfo.RunID)); err != nil {
			return err
		}

	case p.ConflictResolveWorkflowModeUpdateCurrent:
		executionInfo := resetWorkflow.ExecutionInfo
		startVersion := resetWorkflow.StartVersion
		lastWriteVersion := resetWorkflow.LastWriteVersion
		if newWorkflow != nil {
			executionInfo = newWorkflow.ExecutionInfo
			startVersion = newWorkflow.StartVersion
			lastWriteVersion = newWorkflow.LastWriteVersion
		}
		runID := serialization.MustParseUUID(executionInfo.RunID)
		createRequestID := executionInfo.CreateRequestID
		state := executionInfo.State
		closeStatus := executionInfo.CloseStatus

		if currentWorkflow != nil {
			prevRunID := serialization.MustParseUUID(currentWorkflow.ExecutionInfo.RunID)

			if err := m.assertRunIDAndUpdateCurrentExecutionFn(
				ctx,
				tx,
				m.shardID,
				domainID,
				workflowID,
				runID,
				prevRunID,
				createRequestID,
				state,
				closeStatus,
				startVersion,
				lastWriteVersion); err != nil {
				return err
			}
		} else {
			// reset workflow is current
			prevRunID := serialization.MustParseUUID(resetWorkflow.ExecutionInfo.RunID)

			if err := m.assertRunIDAndUpdateCurrentExecutionFn(
				ctx,
				tx,
				m.shardID,
				domainID,
				workflowID,
				runID,
				prevRunID,
				createRequestID,
				state,
				closeStatus,
				startVersion,
				lastWriteVersion); err != nil {
				return err
			}
		}

	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("ConflictResolveWorkflowExecution: unknown mode: %v", request.Mode),
		}
	}

	if err := m.applyWorkflowSnapshotTxAsResetFn(ctx, tx, shardID, &resetWorkflow, m.parser); err != nil {
		return err
	}
	if currentWorkflow != nil {
		if err := m.applyWorkflowMutationTxFn(ctx, tx, shardID, currentWorkflow, m.parser); err != nil {
			return err
		}
	}
	if newWorkflow != nil {
		if err := m.applyWorkflowSnapshotTxAsNewFn(ctx, tx, shardID, newWorkflow, m.parser); err != nil {
			return err
		}
	}
	return nil
}

func (m *sqlExecutionStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.DeleteWorkflowExecutionRequest,
) error {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(m.shardID, m.db.GetTotalNumDBShards())
	domainID := serialization.MustParseUUID(request.DomainID)
	runID := serialization.MustParseUUID(request.RunID)
	wfID := request.WorkflowID
	return m.txExecute(ctx, dbShardID, "DeleteWorkflowExecution", func(tx sqlplugin.Tx) error {
		if _, err := tx.DeleteFromExecutions(ctx, &sqlplugin.ExecutionsFilter{
			ShardID:    m.shardID,
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteWorkflowExecution", "", err)
		}
		if _, err := tx.DeleteFromActivityInfoMaps(ctx, &sqlplugin.ActivityInfoMapsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromActivityInfoMaps", "", err)
		}
		if _, err := tx.DeleteFromTimerInfoMaps(ctx, &sqlplugin.TimerInfoMapsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromTimerInfoMaps", "", err)
		}
		if _, err := tx.DeleteFromChildExecutionInfoMaps(ctx, &sqlplugin.ChildExecutionInfoMapsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromChildExecutionInfoMaps", "", err)
		}
		if _, err := tx.DeleteFromRequestCancelInfoMaps(ctx, &sqlplugin.RequestCancelInfoMapsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromRequestCancelInfoMaps", "", err)
		}
		if _, err := tx.DeleteFromSignalInfoMaps(ctx, &sqlplugin.SignalInfoMapsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromSignalInfoMaps", "", err)
		}
		if _, err := tx.DeleteFromBufferedEvents(ctx, &sqlplugin.BufferedEventsFilter{
			ShardID:    m.shardID,
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromBufferedEvents", "", err)
		}
		if _, err := tx.DeleteFromSignalsRequestedSets(ctx, &sqlplugin.SignalsRequestedSetsFilter{
			ShardID:    int64(m.shardID),
			DomainID:   domainID,
			WorkflowID: wfID,
			RunID:      runID,
		}); err != nil {
			return convertCommonErrors(tx, "DeleteFromSignalsRequestedSets", "", err)
		}
		return nil
	})
}

// its possible for a new run of the same workflow to have started after the run we are deleting
// here was finished. In that case, current_executions table will have the same workflowID but different
// runID. The following code will delete the row from current_executions if and only if the runID is
// same as the one we are trying to delete here
func (m *sqlExecutionStore) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *p.DeleteCurrentWorkflowExecutionRequest,
) error {

	domainID := serialization.MustParseUUID(request.DomainID)
	runID := serialization.MustParseUUID(request.RunID)
	_, err := m.db.DeleteFromCurrentExecutions(ctx, &sqlplugin.CurrentExecutionsFilter{
		ShardID:    int64(m.shardID),
		DomainID:   domainID,
		WorkflowID: request.WorkflowID,
		RunID:      runID,
	})
	if err != nil {
		return convertCommonErrors(m.db, "DeleteCurrentWorkflowExecution", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) GetCurrentExecution(
	ctx context.Context,
	request *p.GetCurrentExecutionRequest,
) (*p.GetCurrentExecutionResponse, error) {

	row, err := m.db.SelectFromCurrentExecutions(ctx, &sqlplugin.CurrentExecutionsFilter{
		ShardID:    int64(m.shardID),
		DomainID:   serialization.MustParseUUID(request.DomainID),
		WorkflowID: request.WorkflowID,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetCurrentExecution", "", err)
	}
	return &p.GetCurrentExecutionResponse{
		StartRequestID:   row.CreateRequestID,
		RunID:            row.RunID.String(),
		State:            int(row.State),
		CloseStatus:      int(row.CloseStatus),
		LastWriteVersion: row.LastWriteVersion,
	}, nil
}

func (m *sqlExecutionStore) ListCurrentExecutions(
	_ context.Context,
	_ *p.ListCurrentExecutionsRequest,
) (*p.ListCurrentExecutionsResponse, error) {
	return nil, &types.InternalServiceError{Message: "Not yet implemented"}
}

func (m *sqlExecutionStore) IsWorkflowExecutionExists(
	_ context.Context,
	_ *p.IsWorkflowExecutionExistsRequest,
) (*p.IsWorkflowExecutionExistsResponse, error) {
	return nil, &types.InternalServiceError{Message: "Not yet implemented"}
}

func (m *sqlExecutionStore) ListConcreteExecutions(
	ctx context.Context,
	request *p.ListConcreteExecutionsRequest,
) (*p.InternalListConcreteExecutionsResponse, error) {

	filter := &sqlplugin.ExecutionsFilter{}
	if len(request.PageToken) > 0 {
		err := gobDeserialize(request.PageToken, &filter)
		if err != nil {
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf("ListConcreteExecutions failed. Error: %v", err),
			}
		}
	} else {
		filter = &sqlplugin.ExecutionsFilter{
			ShardID:    m.shardID,
			WorkflowID: "",
		}
	}
	filter.Size = request.PageSize

	executions, err := m.db.SelectFromExecutions(ctx, filter)
	if err != nil {
		if err == sql.ErrNoRows {
			return &p.InternalListConcreteExecutionsResponse{}, nil
		}
		return nil, convertCommonErrors(m.db, "ListConcreteExecutions", "", err)
	}

	if len(executions) == 0 {
		return &p.InternalListConcreteExecutionsResponse{}, nil
	}
	lastExecution := executions[len(executions)-1]
	nextFilter := &sqlplugin.ExecutionsFilter{
		ShardID:    m.shardID,
		WorkflowID: lastExecution.WorkflowID,
	}
	token, err := gobSerialize(nextFilter)
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("ListConcreteExecutions failed. Error: %v", err),
		}
	}
	concreteExecutions, err := m.populateInternalListConcreteExecutions(executions)
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("ListConcreteExecutions failed. Error: %v", err),
		}
	}

	return &p.InternalListConcreteExecutionsResponse{
		Executions:    concreteExecutions,
		NextPageToken: token,
	}, nil
}

func (m *sqlExecutionStore) GetTransferTasks(
	ctx context.Context,
	request *p.GetTransferTasksRequest,
) (*p.GetTransferTasksResponse, error) {
	minReadLevel := request.ReadLevel
	if len(request.NextPageToken) > 0 {
		readLevel, err := deserializePageToken(request.NextPageToken)
		if err != nil {
			return nil, convertCommonErrors(m.db, "GetTransferTasks", "failed to deserialize page token", err)
		}
		minReadLevel = readLevel
	}
	rows, err := m.db.SelectFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
		ShardID:   m.shardID,
		MinTaskID: minReadLevel,
		MaxTaskID: request.MaxReadLevel,
		PageSize:  request.BatchSize,
	})
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, convertCommonErrors(m.db, "GetTransferTasks", "", err)
		}
	}
	resp := &p.GetTransferTasksResponse{Tasks: make([]*p.TransferTaskInfo, len(rows))}
	for i, row := range rows {
		info, err := m.parser.TransferTaskInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		resp.Tasks[i] = &p.TransferTaskInfo{
			TaskID:                  row.TaskID,
			DomainID:                info.DomainID.String(),
			WorkflowID:              info.GetWorkflowID(),
			RunID:                   info.RunID.String(),
			VisibilityTimestamp:     info.GetVisibilityTimestamp(),
			TargetDomainID:          info.TargetDomainID.String(),
			TargetDomainIDs:         info.GetTargetDomainIDs(),
			TargetWorkflowID:        info.GetTargetWorkflowID(),
			TargetRunID:             info.TargetRunID.String(),
			TargetChildWorkflowOnly: info.GetTargetChildWorkflowOnly(),
			TaskList:                info.GetTaskList(),
			TaskType:                int(info.GetTaskType()),
			ScheduleID:              info.GetScheduleID(),
			Version:                 info.GetVersion(),
		}
	}
	if len(rows) > 0 {
		lastTaskID := rows[len(rows)-1].TaskID
		if lastTaskID < request.MaxReadLevel {
			resp.NextPageToken = serializePageToken(lastTaskID)
		}
	}
	return resp, nil
}

func (m *sqlExecutionStore) CompleteTransferTask(
	ctx context.Context,
	request *p.CompleteTransferTaskRequest,
) error {

	if _, err := m.db.DeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
		ShardID: m.shardID,
		TaskID:  request.TaskID,
	}); err != nil {
		return convertCommonErrors(m.db, "CompleteTransferTask", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) RangeCompleteTransferTask(
	ctx context.Context,
	request *p.RangeCompleteTransferTaskRequest,
) (*p.RangeCompleteTransferTaskResponse, error) {
	result, err := m.db.RangeDeleteFromTransferTasks(ctx, &sqlplugin.TransferTasksFilter{
		ShardID:   m.shardID,
		MinTaskID: request.ExclusiveBeginTaskID,
		MaxTaskID: request.InclusiveEndTaskID,
		PageSize:  request.PageSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteTransferTask", "", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteTransferTask", "", err)
	}
	return &p.RangeCompleteTransferTaskResponse{TasksCompleted: int(rowsDeleted)}, nil
}

func (m *sqlExecutionStore) GetCrossClusterTasks(
	ctx context.Context,
	request *p.GetCrossClusterTasksRequest,
) (*p.GetCrossClusterTasksResponse, error) {
	minReadLevel := request.ReadLevel
	if len(request.NextPageToken) > 0 {
		readLevel, err := deserializePageToken(request.NextPageToken)
		if err != nil {
			return nil, convertCommonErrors(m.db, "GetCrossClusterTasks", "failed to deserialize page token", err)
		}
		minReadLevel = readLevel
	}
	rows, err := m.db.SelectFromCrossClusterTasks(ctx, &sqlplugin.CrossClusterTasksFilter{
		TargetCluster: request.TargetCluster,
		ShardID:       m.shardID,
		MinTaskID:     minReadLevel,
		MaxTaskID:     request.MaxReadLevel,
		PageSize:      request.BatchSize,
	})
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, convertCommonErrors(m.db, "GetCrossClusterTasks", "", err)
		}
	}
	resp := &p.GetCrossClusterTasksResponse{Tasks: make([]*p.CrossClusterTaskInfo, len(rows))}
	for i, row := range rows {
		info, err := m.parser.CrossClusterTaskInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		resp.Tasks[i] = &p.CrossClusterTaskInfo{
			TaskID:                  row.TaskID,
			DomainID:                info.DomainID.String(),
			WorkflowID:              info.GetWorkflowID(),
			RunID:                   info.RunID.String(),
			VisibilityTimestamp:     info.GetVisibilityTimestamp(),
			TargetDomainID:          info.TargetDomainID.String(),
			TargetDomainIDs:         info.GetTargetDomainIDs(),
			TargetWorkflowID:        info.GetTargetWorkflowID(),
			TargetRunID:             info.TargetRunID.String(),
			TargetChildWorkflowOnly: info.GetTargetChildWorkflowOnly(),
			TaskList:                info.GetTaskList(),
			TaskType:                int(info.GetTaskType()),
			ScheduleID:              info.GetScheduleID(),
			Version:                 info.GetVersion(),
		}
	}
	if len(rows) > 0 {
		lastTaskID := rows[len(rows)-1].TaskID
		if lastTaskID < request.MaxReadLevel {
			resp.NextPageToken = serializePageToken(lastTaskID)
		}
	}
	return resp, nil

}

func (m *sqlExecutionStore) CompleteCrossClusterTask(
	ctx context.Context,
	request *p.CompleteCrossClusterTaskRequest,
) error {
	if _, err := m.db.DeleteFromCrossClusterTasks(ctx, &sqlplugin.CrossClusterTasksFilter{
		TargetCluster: request.TargetCluster,
		ShardID:       m.shardID,
		TaskID:        request.TaskID,
	}); err != nil {
		return convertCommonErrors(m.db, "CompleteCrossClusterTask", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) RangeCompleteCrossClusterTask(
	ctx context.Context,
	request *p.RangeCompleteCrossClusterTaskRequest,
) (*p.RangeCompleteCrossClusterTaskResponse, error) {
	result, err := m.db.RangeDeleteFromCrossClusterTasks(ctx, &sqlplugin.CrossClusterTasksFilter{
		TargetCluster: request.TargetCluster,
		ShardID:       m.shardID,
		MinTaskID:     request.ExclusiveBeginTaskID,
		MaxTaskID:     request.InclusiveEndTaskID,
		PageSize:      request.PageSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteCrossClusterTask", "", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteCrossClusterTask", "", err)
	}
	return &p.RangeCompleteCrossClusterTaskResponse{TasksCompleted: int(rowsDeleted)}, nil
}

func (m *sqlExecutionStore) GetReplicationTasks(
	ctx context.Context,
	request *p.GetReplicationTasksRequest,
) (*p.InternalGetReplicationTasksResponse, error) {

	readLevel, maxReadLevelInclusive, err := getReadLevels(request)
	if err != nil {
		return nil, err
	}

	rows, err := m.db.SelectFromReplicationTasks(
		ctx,
		&sqlplugin.ReplicationTasksFilter{
			ShardID:   m.shardID,
			MinTaskID: readLevel,
			MaxTaskID: maxReadLevelInclusive,
			PageSize:  request.BatchSize,
		})

	switch err {
	case nil:
		return m.populateGetReplicationTasksResponse(rows, request.MaxReadLevel)
	case sql.ErrNoRows:
		return &p.InternalGetReplicationTasksResponse{}, nil
	default:
		return nil, convertCommonErrors(m.db, "GetReplicationTasks", "", err)
	}
}

func getReadLevels(request *p.GetReplicationTasksRequest) (readLevel int64, maxReadLevelInclusive int64, err error) {
	readLevel = request.ReadLevel
	if len(request.NextPageToken) > 0 {
		readLevel, err = deserializePageToken(request.NextPageToken)
		if err != nil {
			return 0, 0, err
		}
	}

	maxReadLevelInclusive = max(readLevel+int64(request.BatchSize), request.MaxReadLevel)
	return readLevel, maxReadLevelInclusive, nil
}

func (m *sqlExecutionStore) populateGetReplicationTasksResponse(
	rows []sqlplugin.ReplicationTasksRow,
	requestMaxReadLevel int64,
) (*p.InternalGetReplicationTasksResponse, error) {
	if len(rows) == 0 {
		return &p.InternalGetReplicationTasksResponse{}, nil
	}

	var tasks = make([]*p.InternalReplicationTaskInfo, len(rows))
	for i, row := range rows {
		info, err := m.parser.ReplicationTaskInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}

		tasks[i] = &p.InternalReplicationTaskInfo{
			TaskID:            row.TaskID,
			DomainID:          info.DomainID.String(),
			WorkflowID:        info.GetWorkflowID(),
			RunID:             info.RunID.String(),
			TaskType:          int(info.GetTaskType()),
			FirstEventID:      info.GetFirstEventID(),
			NextEventID:       info.GetNextEventID(),
			Version:           info.GetVersion(),
			ScheduledID:       info.GetScheduledID(),
			BranchToken:       info.GetBranchToken(),
			NewRunBranchToken: info.GetNewRunBranchToken(),
			CreationTime:      info.GetCreationTimestamp(),
		}
	}
	var nextPageToken []byte
	lastTaskID := rows[len(rows)-1].TaskID
	if lastTaskID < requestMaxReadLevel {
		nextPageToken = serializePageToken(lastTaskID)
	}
	return &p.InternalGetReplicationTasksResponse{
		Tasks:         tasks,
		NextPageToken: nextPageToken,
	}, nil
}

func (m *sqlExecutionStore) CompleteReplicationTask(
	ctx context.Context,
	request *p.CompleteReplicationTaskRequest,
) error {

	if _, err := m.db.DeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
		ShardID: m.shardID,
		TaskID:  request.TaskID,
	}); err != nil {
		return convertCommonErrors(m.db, "CompleteReplicationTask", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) RangeCompleteReplicationTask(
	ctx context.Context,
	request *p.RangeCompleteReplicationTaskRequest,
) (*p.RangeCompleteReplicationTaskResponse, error) {
	result, err := m.db.RangeDeleteFromReplicationTasks(ctx, &sqlplugin.ReplicationTasksFilter{
		ShardID:            m.shardID,
		InclusiveEndTaskID: request.InclusiveEndTaskID,
		PageSize:           request.PageSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteReplicationTask", "", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteReplicationTask", "", err)
	}
	return &p.RangeCompleteReplicationTaskResponse{TasksCompleted: int(rowsDeleted)}, nil
}

func (m *sqlExecutionStore) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *p.GetReplicationTasksFromDLQRequest,
) (*p.InternalGetReplicationTasksFromDLQResponse, error) {

	readLevel, maxReadLevelInclusive, err := getReadLevels(&request.GetReplicationTasksRequest)
	if err != nil {
		return nil, err
	}

	filter := sqlplugin.ReplicationTasksFilter{
		ShardID:   m.shardID,
		MinTaskID: readLevel,
		MaxTaskID: maxReadLevelInclusive,
		PageSize:  request.BatchSize,
	}
	rows, err := m.db.SelectFromReplicationTasksDLQ(ctx, &sqlplugin.ReplicationTasksDLQFilter{
		ReplicationTasksFilter: filter,
		SourceClusterName:      request.SourceClusterName,
	})

	switch err {
	case nil:
		return m.populateGetReplicationTasksResponse(rows, request.MaxReadLevel)
	case sql.ErrNoRows:
		return &p.InternalGetReplicationTasksResponse{}, nil
	default:
		return nil, convertCommonErrors(m.db, "GetReplicationTasksFromDLQ", "", err)
	}
}

func (m *sqlExecutionStore) GetReplicationDLQSize(
	ctx context.Context,
	request *p.GetReplicationDLQSizeRequest,
) (*p.GetReplicationDLQSizeResponse, error) {

	size, err := m.db.SelectFromReplicationDLQ(ctx, &sqlplugin.ReplicationTaskDLQFilter{
		SourceClusterName: request.SourceClusterName,
		ShardID:           m.shardID,
	})

	switch err {
	case nil:
		return &p.GetReplicationDLQSizeResponse{
			Size: size,
		}, nil
	case sql.ErrNoRows:
		return &p.GetReplicationDLQSizeResponse{
			Size: 0,
		}, nil
	default:
		return nil, convertCommonErrors(m.db, "GetReplicationDLQSize", "", err)
	}
}

func (m *sqlExecutionStore) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *p.DeleteReplicationTaskFromDLQRequest,
) error {

	filter := sqlplugin.ReplicationTasksFilter{
		ShardID: m.shardID,
		TaskID:  request.TaskID,
	}

	if _, err := m.db.DeleteMessageFromReplicationTasksDLQ(ctx, &sqlplugin.ReplicationTasksDLQFilter{
		ReplicationTasksFilter: filter,
		SourceClusterName:      request.SourceClusterName,
	}); err != nil {
		return convertCommonErrors(m.db, "DeleteReplicationTaskFromDLQ", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *p.RangeDeleteReplicationTaskFromDLQRequest,
) (*p.RangeDeleteReplicationTaskFromDLQResponse, error) {
	filter := sqlplugin.ReplicationTasksFilter{
		ShardID:            m.shardID,
		TaskID:             request.ExclusiveBeginTaskID,
		InclusiveEndTaskID: request.InclusiveEndTaskID,
		PageSize:           request.PageSize,
	}
	result, err := m.db.RangeDeleteMessageFromReplicationTasksDLQ(ctx, &sqlplugin.ReplicationTasksDLQFilter{
		ReplicationTasksFilter: filter,
		SourceClusterName:      request.SourceClusterName,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeDeleteReplicationTaskFromDLQ", "", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeDeleteReplicationTaskFromDLQ", "", err)
	}
	return &p.RangeDeleteReplicationTaskFromDLQResponse{TasksCompleted: int(rowsDeleted)}, nil
}

func (m *sqlExecutionStore) CreateFailoverMarkerTasks(
	ctx context.Context,
	request *p.CreateFailoverMarkersRequest,
) error {
	dbShardID := sqlplugin.GetDBShardIDFromHistoryShardID(m.shardID, m.db.GetTotalNumDBShards())
	return m.txExecuteShardLockedFn(ctx, dbShardID, "CreateFailoverMarkerTasks", request.RangeID, func(tx sqlplugin.Tx) error {
		replicationTasksRows := make([]sqlplugin.ReplicationTasksRow, len(request.Markers))
		for i, task := range request.Markers {
			blob, err := m.parser.ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
				DomainID:                serialization.MustParseUUID(task.DomainID),
				WorkflowID:              emptyWorkflowID,
				RunID:                   serialization.MustParseUUID(emptyReplicationRunID),
				TaskType:                int16(task.GetType()),
				FirstEventID:            common.EmptyEventID,
				NextEventID:             common.EmptyEventID,
				Version:                 task.GetVersion(),
				ScheduledID:             common.EmptyEventID,
				EventStoreVersion:       p.EventStoreVersion,
				NewRunEventStoreVersion: p.EventStoreVersion,
				BranchToken:             nil,
				NewRunBranchToken:       nil,
				CreationTimestamp:       task.GetVisibilityTimestamp(),
			})
			if err != nil {
				return err
			}
			replicationTasksRows[i].ShardID = m.shardID
			replicationTasksRows[i].TaskID = task.GetTaskID()
			replicationTasksRows[i].Data = blob.Data
			replicationTasksRows[i].DataEncoding = string(blob.Encoding)
		}
		result, err := tx.InsertIntoReplicationTasks(ctx, replicationTasksRows)
		if err != nil {
			return convertCommonErrors(tx, "CreateFailoverMarkerTasks", "", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return &types.InternalServiceError{Message: fmt.Sprintf("CreateFailoverMarkerTasks failed. Could not verify number of rows inserted. Error: %v", err)}
		}
		if int(rowsAffected) != len(replicationTasksRows) {
			return &types.InternalServiceError{Message: fmt.Sprintf("CreateFailoverMarkerTasks failed. Inserted %v instead of %v rows into replication_tasks.", rowsAffected, len(replicationTasksRows))}
		}
		return nil
	})
}

type timerTaskPageToken struct {
	TaskID    int64
	Timestamp time.Time
}

func (t *timerTaskPageToken) serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t *timerTaskPageToken) deserialize(payload []byte) error {
	return json.Unmarshal(payload, t)
}

func (m *sqlExecutionStore) GetTimerIndexTasks(
	ctx context.Context,
	request *p.GetTimerIndexTasksRequest,
) (*p.GetTimerIndexTasksResponse, error) {

	pageToken := &timerTaskPageToken{TaskID: math.MinInt64, Timestamp: request.MinTimestamp}
	if len(request.NextPageToken) > 0 {
		if err := pageToken.deserialize(request.NextPageToken); err != nil {
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf("error deserializing timerTaskPageToken: %v", err),
			}
		}
	}

	rows, err := m.db.SelectFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
		ShardID:                m.shardID,
		MinVisibilityTimestamp: pageToken.Timestamp,
		TaskID:                 pageToken.TaskID,
		MaxVisibilityTimestamp: request.MaxTimestamp,
		PageSize:               request.BatchSize + 1,
	})

	if err != nil && err != sql.ErrNoRows {
		return nil, convertCommonErrors(m.db, "GetTimerIndexTasks", "", err)
	}

	resp := &p.GetTimerIndexTasksResponse{Timers: make([]*p.TimerTaskInfo, len(rows))}
	for i, row := range rows {
		info, err := m.parser.TimerTaskInfoFromBlob(row.Data, row.DataEncoding)
		if err != nil {
			return nil, err
		}
		resp.Timers[i] = &p.TimerTaskInfo{
			VisibilityTimestamp: row.VisibilityTimestamp,
			TaskID:              row.TaskID,
			DomainID:            info.DomainID.String(),
			WorkflowID:          info.GetWorkflowID(),
			RunID:               info.RunID.String(),
			TaskType:            int(info.GetTaskType()),
			TimeoutType:         int(info.GetTimeoutType()),
			EventID:             info.GetEventID(),
			ScheduleAttempt:     info.GetScheduleAttempt(),
			Version:             info.GetVersion(),
		}
	}

	if len(resp.Timers) > request.BatchSize {
		pageToken = &timerTaskPageToken{
			TaskID:    resp.Timers[request.BatchSize].TaskID,
			Timestamp: resp.Timers[request.BatchSize].VisibilityTimestamp,
		}
		resp.Timers = resp.Timers[:request.BatchSize]
		nextToken, err := pageToken.serialize()
		if err != nil {
			return nil, &types.InternalServiceError{
				Message: fmt.Sprintf("GetTimerTasks: error serializing page token: %v", err),
			}
		}
		resp.NextPageToken = nextToken
	}

	return resp, nil
}

func (m *sqlExecutionStore) CompleteTimerTask(
	ctx context.Context,
	request *p.CompleteTimerTaskRequest,
) error {

	if _, err := m.db.DeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
		ShardID:             m.shardID,
		VisibilityTimestamp: request.VisibilityTimestamp,
		TaskID:              request.TaskID,
	}); err != nil {
		return convertCommonErrors(m.db, "CompleteTimerTask", "", err)
	}
	return nil
}

func (m *sqlExecutionStore) RangeCompleteTimerTask(
	ctx context.Context,
	request *p.RangeCompleteTimerTaskRequest,
) (*p.RangeCompleteTimerTaskResponse, error) {
	result, err := m.db.RangeDeleteFromTimerTasks(ctx, &sqlplugin.TimerTasksFilter{
		ShardID:                m.shardID,
		MinVisibilityTimestamp: request.InclusiveBeginTimestamp,
		MaxVisibilityTimestamp: request.ExclusiveEndTimestamp,
		PageSize:               request.PageSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteTimerTask", "", err)
	}
	rowsDeleted, err := result.RowsAffected()
	if err != nil {
		return nil, convertCommonErrors(m.db, "RangeCompleteTimerTask", "", err)
	}
	return &p.RangeCompleteTimerTaskResponse{TasksCompleted: int(rowsDeleted)}, nil
}

func (m *sqlExecutionStore) PutReplicationTaskToDLQ(
	ctx context.Context,
	request *p.InternalPutReplicationTaskToDLQRequest,
) error {
	replicationTask := request.TaskInfo
	blob, err := m.parser.ReplicationTaskInfoToBlob(&serialization.ReplicationTaskInfo{
		DomainID:                serialization.MustParseUUID(replicationTask.DomainID),
		WorkflowID:              replicationTask.WorkflowID,
		RunID:                   serialization.MustParseUUID(replicationTask.RunID),
		TaskType:                int16(replicationTask.TaskType),
		FirstEventID:            replicationTask.FirstEventID,
		NextEventID:             replicationTask.NextEventID,
		Version:                 replicationTask.Version,
		ScheduledID:             replicationTask.ScheduledID,
		EventStoreVersion:       p.EventStoreVersion,
		NewRunEventStoreVersion: p.EventStoreVersion,
		BranchToken:             replicationTask.BranchToken,
		NewRunBranchToken:       replicationTask.NewRunBranchToken,
		CreationTimestamp:       replicationTask.CreationTime,
	})
	if err != nil {
		return err
	}

	row := &sqlplugin.ReplicationTaskDLQRow{
		SourceClusterName: request.SourceClusterName,
		ShardID:           m.shardID,
		TaskID:            replicationTask.TaskID,
		Data:              blob.Data,
		DataEncoding:      string(blob.Encoding),
	}

	_, err = m.db.InsertIntoReplicationTasksDLQ(ctx, row)

	// Tasks are immutable. So it's fine if we already persisted it before.
	// This can happen when tasks are retried (ack and cleanup can have lag on source side).
	if err != nil && !m.db.IsDupEntryError(err) {
		return convertCommonErrors(m.db, "PutReplicationTaskToDLQ", "", err)
	}

	return nil
}

func (m *sqlExecutionStore) populateWorkflowMutableState(
	execution sqlplugin.ExecutionsRow,
) (*p.InternalWorkflowMutableState, error) {

	info, err := m.parser.WorkflowExecutionInfoFromBlob(execution.Data, execution.DataEncoding)
	if err != nil {
		return nil, err
	}

	state := &p.InternalWorkflowMutableState{}
	state.ExecutionInfo = serialization.ToInternalWorkflowExecutionInfo(info)
	state.ExecutionInfo.DomainID = execution.DomainID.String()
	state.ExecutionInfo.WorkflowID = execution.WorkflowID
	state.ExecutionInfo.RunID = execution.RunID.String()
	state.ExecutionInfo.NextEventID = execution.NextEventID
	// TODO: remove this after all 2DC workflows complete
	if info.LastWriteEventID != nil {
		state.ReplicationState = &p.ReplicationState{}
		state.ReplicationState.StartVersion = info.GetStartVersion()
		state.ReplicationState.LastWriteVersion = execution.LastWriteVersion
		state.ReplicationState.LastWriteEventID = info.GetLastWriteEventID()
	}

	if info.GetVersionHistories() != nil {
		state.VersionHistories = p.NewDataBlob(
			info.GetVersionHistories(),
			common.EncodingType(info.GetVersionHistoriesEncoding()),
		)
	}

	if info.GetChecksum() != nil {
		state.ChecksumData = p.NewDataBlob(
			info.GetChecksum(),
			common.EncodingType(info.GetChecksumEncoding()),
		)
	}

	return state, nil
}

func (m *sqlExecutionStore) populateInternalListConcreteExecutions(
	executions []sqlplugin.ExecutionsRow,
) ([]*p.InternalListConcreteExecutionsEntity, error) {

	concreteExecutions := make([]*p.InternalListConcreteExecutionsEntity, 0, len(executions))
	for _, execution := range executions {
		mutableState, err := m.populateWorkflowMutableState(execution)
		if err != nil {
			return nil, err
		}

		concreteExecution := &p.InternalListConcreteExecutionsEntity{
			ExecutionInfo:    mutableState.ExecutionInfo,
			VersionHistories: mutableState.VersionHistories,
		}
		concreteExecutions = append(concreteExecutions, concreteExecution)
	}
	return concreteExecutions, nil
}
