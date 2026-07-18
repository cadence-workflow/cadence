// Copyright (c) 2017-2021 Uber Technologies, Inc.
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

package nosql

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

// Implements ExecutionStore
type nosqlExecutionStore struct {
	shardedNosqlStore
	taskSerializer serialization.TaskSerializer
}

// newNoSQLExecutionStore creates a host-level ExecutionStore that routes by request ShardID.
func newNoSQLExecutionStore(
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	taskSerializer serialization.TaskSerializer,
	dc *persistence.DynamicConfiguration,
) (persistence.ExecutionStore, error) {
	s, err := newShardedNosqlStore(cfg, logger, metricsClient, dc, true)
	if err != nil {
		return nil, err
	}
	return &nosqlExecutionStore{
		shardedNosqlStore: s,
		taskSerializer:    taskSerializer,
	}, nil
}

func resolveRequestShardID(requestShardID *int, operation string, logger log.Logger) (int, error) {
	if requestShardID == nil {
		err := &types.BadRequestError{Message: "execution persistence request missing shard ID"}
		if logger != nil {
			logger.Warn("execution persistence request missing shard ID", tag.OperationName(operation), tag.Error(err))
		}
		return 0, err
	}
	return *requestShardID, nil
}

func (d *nosqlExecutionStore) storeShardForRequest(requestShardID *int, operation string) (int, *nosqlStore, error) {
	shardID, err := resolveRequestShardID(requestShardID, operation, d.GetLogger())
	if err != nil {
		return 0, nil, err
	}
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return 0, nil, err
	}
	return shardID, storeShard, nil
}

func (d *nosqlExecutionStore) CreateWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalCreateWorkflowExecutionRequest,
) (*persistence.CreateWorkflowExecutionResponse, error) {

	newWorkflow := request.NewWorkflowSnapshot
	executionInfo := newWorkflow.ExecutionInfo
	lastWriteVersion := newWorkflow.LastWriteVersion
	domainID := executionInfo.DomainID
	workflowID := executionInfo.WorkflowID
	runID := executionInfo.RunID

	if err := persistence.ValidateCreateWorkflowModeState(
		request.Mode,
		newWorkflow,
	); err != nil {
		return nil, err
	}

	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "CreateWorkflowExecution")
	if err != nil {
		return nil, err
	}

	workflowRequestWriteMode, err := getWorkflowRequestWriteMode(request.WorkflowRequestMode)
	if err != nil {
		return nil, err
	}

	currentWorkflowWriteReq, err := d.prepareCurrentWorkflowRequestForCreateWorkflowTxn(domainID, workflowID, runID, executionInfo, lastWriteVersion, request, shardID)
	if err != nil {
		return nil, err
	}

	workflowExecutionWriteReq, err := d.prepareCreateWorkflowExecutionRequestWithMaps(&newWorkflow, request.CurrentTimeStamp)
	if err != nil {
		return nil, err
	}

	tasksByCategory := map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask{}
	err = d.prepareNoSQLTasksForWorkflowTxn(
		domainID, workflowID, runID,
		newWorkflow.TasksByCategory,
		tasksByCategory,
	)
	if err != nil {
		return nil, err
	}

	workflowRequests := d.prepareWorkflowRequestRows(domainID, workflowID, runID, newWorkflow.WorkflowRequests, nil, shardID)
	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	activeClusterSelectionPolicyRow := d.prepareActiveClusterSelectionPolicyRow(domainID, workflowID, runID, executionInfo.ActiveClusterSelectionPolicy, shardID)

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: shardID,
		RangeID: request.RangeID,
	}

	err = storeShard.db.InsertWorkflowExecutionWithTasks(
		ctx,
		workflowRequestsWriteRequest,
		currentWorkflowWriteReq,
		workflowExecutionWriteReq,
		tasksByCategory,
		activeClusterSelectionPolicyRow,
		shardCondition,
	)
	if err != nil {
		conditionFailureErr, isConditionFailedError := err.(*nosqlplugin.WorkflowOperationConditionFailure)
		if isConditionFailedError {
			switch {
			case conditionFailureErr.UnknownConditionFailureDetails != nil:
				return nil, &persistence.ShardOwnershipLostError{
					ShardID: shardID,
					Msg:     *conditionFailureErr.UnknownConditionFailureDetails,
				}
			case conditionFailureErr.ShardRangeIDNotMatch != nil:
				return nil, &persistence.ShardOwnershipLostError{
					ShardID: shardID,
					Msg: fmt.Sprintf("Failed to create workflow execution.  Request RangeID: %v, Actual RangeID: %v",
						request.RangeID, *conditionFailureErr.ShardRangeIDNotMatch),
				}
			case conditionFailureErr.CurrentWorkflowConditionFailInfo != nil:
				return nil, &persistence.CurrentWorkflowConditionFailedError{
					Msg: *conditionFailureErr.CurrentWorkflowConditionFailInfo,
				}
			case conditionFailureErr.WorkflowExecutionAlreadyExists != nil:
				return nil, &persistence.WorkflowExecutionAlreadyStartedError{
					Msg:              conditionFailureErr.WorkflowExecutionAlreadyExists.OtherInfo,
					StartRequestID:   conditionFailureErr.WorkflowExecutionAlreadyExists.CreateRequestID,
					RunID:            conditionFailureErr.WorkflowExecutionAlreadyExists.RunID,
					State:            conditionFailureErr.WorkflowExecutionAlreadyExists.State,
					CloseStatus:      conditionFailureErr.WorkflowExecutionAlreadyExists.CloseStatus,
					LastWriteVersion: conditionFailureErr.WorkflowExecutionAlreadyExists.LastWriteVersion,
				}
			case conditionFailureErr.DuplicateRequest != nil:
				return nil, &persistence.DuplicateRequestError{
					RequestType: conditionFailureErr.DuplicateRequest.RequestType,
					RunID:       conditionFailureErr.DuplicateRequest.RunID,
				}
			default:
				// If ever runs into this branch, there is bug in the code either in here, or in the implementation of nosql plugin
				err := fmt.Errorf("unsupported conditionFailureReason error")
				d.GetLogger().Error("A code bug exists in persistence layer, please investigate ASAP", tag.Error(err))
				return nil, err
			}
		}
		return nil, convertCommonErrors(storeShard.db, "CreateWorkflowExecution", err)
	}

	return &persistence.CreateWorkflowExecutionResponse{}, nil
}

func (d *nosqlExecutionStore) GetWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalGetWorkflowExecutionRequest,
) (*persistence.InternalGetWorkflowExecutionResponse, error) {

	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "GetWorkflowExecution")
	if err != nil {
		return nil, err
	}
	execution := request.Execution
	state, err := storeShard.db.SelectWorkflowExecution(ctx, shardID, request.DomainID, execution.WorkflowID, execution.RunID)
	if err != nil {
		if storeShard.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v, RunId: %v",
					execution.WorkflowID, execution.RunID),
			}
		}

		return nil, convertCommonErrors(storeShard.db, "GetWorkflowExecution", err)
	}

	return &persistence.InternalGetWorkflowExecutionResponse{State: state}, nil
}

func (d *nosqlExecutionStore) UpdateWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalUpdateWorkflowExecutionRequest,
) error {
	updateWorkflow := request.UpdateWorkflowMutation
	newWorkflow := request.NewWorkflowSnapshot

	executionInfo := updateWorkflow.ExecutionInfo
	domainID := executionInfo.DomainID
	workflowID := executionInfo.WorkflowID
	runID := executionInfo.RunID

	if err := persistence.ValidateUpdateWorkflowModeState(
		request.Mode,
		updateWorkflow,
		newWorkflow,
	); err != nil {
		return err
	}

	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "UpdateWorkflowExecution")
	if err != nil {
		return err
	}

	workflowRequestWriteMode, err := getWorkflowRequestWriteMode(request.WorkflowRequestMode)
	if err != nil {
		return err
	}
	var currentWorkflowWriteReq *nosqlplugin.CurrentWorkflowWriteRequest

	switch request.Mode {
	case persistence.UpdateWorkflowModeIgnoreCurrent:
		currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
			WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
		}
	case persistence.UpdateWorkflowModeBypassCurrent:
		if err := d.assertNotCurrentExecution(
			ctx,
			shardID,
			domainID,
			workflowID,
			runID); err != nil {
			return err
		}
		currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
			WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
		}

	case persistence.UpdateWorkflowModeUpdateCurrent:
		if newWorkflow != nil {
			newExecutionInfo := newWorkflow.ExecutionInfo
			newLastWriteVersion := newWorkflow.LastWriteVersion
			newDomainID := newExecutionInfo.DomainID
			// TODO: ?? would it change at all ??
			newWorkflowID := newExecutionInfo.WorkflowID
			newRunID := newExecutionInfo.RunID

			if domainID != newDomainID {
				return &types.InternalServiceError{
					Message: "UpdateWorkflowExecution: cannot continue as new to another domain",
				}
			}

			currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					ShardID:          shardID,
					DomainID:         newDomainID,
					WorkflowID:       newWorkflowID,
					RunID:            newRunID,
					State:            newExecutionInfo.State,
					CloseStatus:      newExecutionInfo.CloseStatus,
					CreateRequestID:  newExecutionInfo.CreateRequestID,
					LastWriteVersion: newLastWriteVersion,
				},
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: &runID,
				},
			}
		} else {
			lastWriteVersion := updateWorkflow.LastWriteVersion

			currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Row: nosqlplugin.CurrentWorkflowRow{
					ShardID:          shardID,
					DomainID:         domainID,
					WorkflowID:       workflowID,
					RunID:            runID,
					State:            executionInfo.State,
					CloseStatus:      executionInfo.CloseStatus,
					CreateRequestID:  executionInfo.CreateRequestID,
					LastWriteVersion: lastWriteVersion,
				},
				Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
					CurrentRunID: &runID,
				},
			}
		}

	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("UpdateWorkflowExecution: unknown mode: %v", request.Mode),
		}
	}

	var mutateExecution, insertExecution *nosqlplugin.WorkflowExecutionRequest
	var activeClusterSelectionPolicyRow *nosqlplugin.ActiveClusterSelectionPolicyRow
	var workflowRequests []*nosqlplugin.WorkflowRequestRow
	tasksByCategory := map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask{}

	// 1. current
	mutateExecution, err = d.prepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer(&updateWorkflow, request.CurrentTimeStamp)
	if err != nil {
		return err
	}
	err = d.prepareNoSQLTasksForWorkflowTxn(
		domainID, workflowID, updateWorkflow.ExecutionInfo.RunID,
		updateWorkflow.TasksByCategory,
		tasksByCategory,
	)
	if err != nil {
		return err
	}
	workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, runID, updateWorkflow.WorkflowRequests, workflowRequests, shardID)

	// 2. new
	if newWorkflow != nil {
		insertExecution, err = d.prepareCreateWorkflowExecutionRequestWithMaps(newWorkflow, request.CurrentTimeStamp)
		if err != nil {
			return err
		}

		activeClusterSelectionPolicyRow = d.prepareActiveClusterSelectionPolicyRow(
			domainID,
			newWorkflow.ExecutionInfo.WorkflowID,
			newWorkflow.ExecutionInfo.RunID,
			newWorkflow.ExecutionInfo.ActiveClusterSelectionPolicy,
			shardID,
		)

		err = d.prepareNoSQLTasksForWorkflowTxn(
			domainID, workflowID, newWorkflow.ExecutionInfo.RunID,
			newWorkflow.TasksByCategory,
			tasksByCategory,
		)
		if err != nil {
			return err
		}
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, newWorkflow.ExecutionInfo.RunID, newWorkflow.WorkflowRequests, workflowRequests, shardID)
	}

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: shardID,
		RangeID: request.RangeID,
	}

	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	err = storeShard.db.UpdateWorkflowExecutionWithTasks(
		ctx,
		workflowRequestsWriteRequest,
		currentWorkflowWriteReq,
		mutateExecution,
		insertExecution,
		activeClusterSelectionPolicyRow,
		nil, // no workflow to reset here
		tasksByCategory,
		shardCondition,
	)

	return d.processUpdateWorkflowResult(err, request.RangeID, shardID)
}

func (d *nosqlExecutionStore) ConflictResolveWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalConflictResolveWorkflowExecutionRequest,
) error {
	currentWorkflow := request.CurrentWorkflowMutation
	resetWorkflow := request.ResetWorkflowSnapshot
	newWorkflow := request.NewWorkflowSnapshot

	domainID := resetWorkflow.ExecutionInfo.DomainID
	workflowID := resetWorkflow.ExecutionInfo.WorkflowID

	if err := persistence.ValidateConflictResolveWorkflowModeState(
		request.Mode,
		resetWorkflow,
		newWorkflow,
		currentWorkflow,
	); err != nil {
		return err
	}

	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "ConflictResolveWorkflowExecution")
	if err != nil {
		return err
	}

	workflowRequestWriteMode, err := getWorkflowRequestWriteMode(request.WorkflowRequestMode)
	if err != nil {
		return err
	}
	var currentWorkflowWriteReq *nosqlplugin.CurrentWorkflowWriteRequest
	var prevRunID string

	switch request.Mode {
	case persistence.ConflictResolveWorkflowModeBypassCurrent:
		if err := d.assertNotCurrentExecution(
			ctx,
			shardID,
			domainID,
			workflowID,
			resetWorkflow.ExecutionInfo.RunID); err != nil {
			return err
		}
		currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
			WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
		}
	case persistence.ConflictResolveWorkflowModeUpdateCurrent:
		executionInfo := resetWorkflow.ExecutionInfo
		lastWriteVersion := resetWorkflow.LastWriteVersion
		if newWorkflow != nil {
			executionInfo = newWorkflow.ExecutionInfo
			lastWriteVersion = newWorkflow.LastWriteVersion
		}

		if currentWorkflow != nil {
			prevRunID = currentWorkflow.ExecutionInfo.RunID
		} else {
			// reset workflow is current
			prevRunID = resetWorkflow.ExecutionInfo.RunID
		}
		currentWorkflowWriteReq = &nosqlplugin.CurrentWorkflowWriteRequest{
			WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
			Row: nosqlplugin.CurrentWorkflowRow{
				ShardID:          shardID,
				DomainID:         domainID,
				WorkflowID:       workflowID,
				RunID:            executionInfo.RunID,
				State:            executionInfo.State,
				CloseStatus:      executionInfo.CloseStatus,
				CreateRequestID:  executionInfo.CreateRequestID,
				LastWriteVersion: lastWriteVersion,
			},
			Condition: &nosqlplugin.CurrentWorkflowWriteCondition{
				CurrentRunID: &prevRunID,
			},
		}

	default:
		return &types.InternalServiceError{
			Message: fmt.Sprintf("ConflictResolveWorkflowExecution: unknown mode: %v", request.Mode),
		}
	}

	var mutateExecution, insertExecution, resetExecution *nosqlplugin.WorkflowExecutionRequest
	var workflowRequests []*nosqlplugin.WorkflowRequestRow
	tasksByCategory := map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask{}

	// 1. current
	if currentWorkflow != nil {
		mutateExecution, err = d.prepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer(currentWorkflow, request.CurrentTimeStamp)
		if err != nil {
			return err
		}
		err = d.prepareNoSQLTasksForWorkflowTxn(
			domainID, workflowID, currentWorkflow.ExecutionInfo.RunID,
			currentWorkflow.TasksByCategory,
			tasksByCategory,
		)
		if err != nil {
			return err
		}
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, currentWorkflow.ExecutionInfo.RunID, currentWorkflow.WorkflowRequests, workflowRequests, shardID)
	}

	// 2. reset
	resetExecution, err = d.prepareResetWorkflowExecutionRequestWithMapsAndEventBuffer(&resetWorkflow, request.CurrentTimeStamp)
	if err != nil {
		return err
	}
	err = d.prepareNoSQLTasksForWorkflowTxn(
		domainID, workflowID, resetWorkflow.ExecutionInfo.RunID,
		resetWorkflow.TasksByCategory,
		tasksByCategory,
	)
	if err != nil {
		return err
	}
	workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, resetWorkflow.ExecutionInfo.RunID, resetWorkflow.WorkflowRequests, workflowRequests, shardID)

	// 3. new
	if newWorkflow != nil {
		insertExecution, err = d.prepareCreateWorkflowExecutionRequestWithMaps(newWorkflow, request.CurrentTimeStamp)
		if err != nil {
			return err
		}

		// TODO(active-active): make changes here to insert active cluster selection policy.

		err = d.prepareNoSQLTasksForWorkflowTxn(
			domainID, workflowID, newWorkflow.ExecutionInfo.RunID,
			newWorkflow.TasksByCategory,
			tasksByCategory,
		)
		if err != nil {
			return err
		}
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, newWorkflow.ExecutionInfo.RunID, newWorkflow.WorkflowRequests, workflowRequests, shardID)
	}

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: shardID,
		RangeID: request.RangeID,
	}

	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	err = storeShard.db.UpdateWorkflowExecutionWithTasks(
		ctx, workflowRequestsWriteRequest, currentWorkflowWriteReq,
		mutateExecution, insertExecution, nil, resetExecution,
		tasksByCategory,
		shardCondition)
	return d.processUpdateWorkflowResult(err, request.RangeID, shardID)
}

func (d *nosqlExecutionStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *persistence.DeleteWorkflowExecutionRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "DeleteWorkflowExecution")
	if err != nil {
		return err
	}
	err = storeShard.db.DeleteWorkflowExecution(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(storeShard.db, "DeleteWorkflowExecution", err)
	}

	return nil
}

func (d *nosqlExecutionStore) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *persistence.DeleteCurrentWorkflowExecutionRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "DeleteCurrentWorkflowExecution")
	if err != nil {
		return err
	}
	err = storeShard.db.DeleteCurrentWorkflow(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(storeShard.db, "DeleteCurrentWorkflowExecution", err)
	}

	return nil
}

func (d *nosqlExecutionStore) DeleteActiveClusterSelectionPolicy(
	ctx context.Context,
	request *persistence.DeleteActiveClusterSelectionPolicyRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "DeleteActiveClusterSelectionPolicy")
	if err != nil {
		return err
	}
	err = storeShard.db.DeleteActiveClusterSelectionPolicy(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(storeShard.db, "DeleteActiveClusterSelectionPolicy", err)
	}
	return nil
}

func (d *nosqlExecutionStore) GetCurrentExecution(
	ctx context.Context,
	request *persistence.GetCurrentExecutionRequest,
) (*persistence.GetCurrentExecutionResponse,
	error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "GetCurrentExecution")
	if err != nil {
		return nil, err
	}
	result, err := storeShard.db.SelectCurrentWorkflow(ctx, shardID, request.DomainID, request.WorkflowID)

	if err != nil {
		if storeShard.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v",
					request.WorkflowID),
			}
		}
		return nil, convertCommonErrors(storeShard.db, "GetCurrentExecution", err)
	}

	return &persistence.GetCurrentExecutionResponse{
		RunID:            result.RunID,
		StartRequestID:   result.CreateRequestID,
		State:            result.State,
		CloseStatus:      result.CloseStatus,
		LastWriteVersion: result.LastWriteVersion,
	}, nil
}

func (d *nosqlExecutionStore) ListCurrentExecutions(
	ctx context.Context,
	request *persistence.ListCurrentExecutionsRequest,
) (*persistence.ListCurrentExecutionsResponse, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "ListCurrentExecutions")
	if err != nil {
		return nil, err
	}
	executions, token, err := storeShard.db.SelectAllCurrentWorkflows(ctx, shardID, request.PageToken, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "ListCurrentExecutions", err)
	}
	return &persistence.ListCurrentExecutionsResponse{
		Executions: executions,
		PageToken:  token,
	}, nil
}

func (d *nosqlExecutionStore) IsWorkflowExecutionExists(
	ctx context.Context,
	request *persistence.IsWorkflowExecutionExistsRequest,
) (*persistence.IsWorkflowExecutionExistsResponse, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "IsWorkflowExecutionExists")
	if err != nil {
		return nil, err
	}
	exists, err := storeShard.db.IsWorkflowExecutionExists(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "IsWorkflowExecutionExists", err)
	}
	return &persistence.IsWorkflowExecutionExistsResponse{
		Exists: exists,
	}, nil
}

func (d *nosqlExecutionStore) ListConcreteExecutions(
	ctx context.Context,
	request *persistence.ListConcreteExecutionsRequest,
) (*persistence.InternalListConcreteExecutionsResponse, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "ListConcreteExecutions")
	if err != nil {
		return nil, err
	}
	executions, nextPageToken, err := storeShard.db.SelectAllWorkflowExecutions(ctx, shardID, request.PageToken, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "ListConcreteExecutions", err)
	}
	return &persistence.InternalListConcreteExecutionsResponse{
		Executions:    executions,
		NextPageToken: nextPageToken,
	}, nil
}

func (d *nosqlExecutionStore) PutReplicationTaskToDLQ(
	ctx context.Context,
	request *persistence.InternalPutReplicationTaskToDLQRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "PutReplicationTaskToDLQ")
	if err != nil {
		return err
	}
	err = storeShard.db.InsertReplicationDLQTask(ctx, shardID, request.SourceClusterName, &nosqlplugin.HistoryMigrationTask{
		Replication: request.TaskInfo,
		Task:        request.Task,
	})
	if err != nil {
		return convertCommonErrors(storeShard.db, "PutReplicationTaskToDLQ", err)
	}

	return nil
}

func (d *nosqlExecutionStore) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *persistence.GetReplicationTasksFromDLQRequest,
) (*persistence.InternalGetReplicationDLQTasksResponse, error) {
	if request.ReadLevel > request.MaxReadLevel {
		return nil, &types.BadRequestError{Message: "ReadLevel cannot be higher than MaxReadLevel"}
	}
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "GetReplicationTasksFromDLQ")
	if err != nil {
		return nil, err
	}
	tasks, nextPageToken, err := storeShard.db.SelectReplicationDLQTasksOrderByTaskID(ctx, shardID, request.SourceClusterName, request.BatchSize, request.NextPageToken, request.ReadLevel, request.MaxReadLevel)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "GetReplicationTasksFromDLQ", err)
	}
	var dlqTasks []*persistence.InternalReplicationDLQTask
	for _, t := range tasks {
		r := t.Replication
		dlqTasks = append(dlqTasks, &persistence.InternalReplicationDLQTask{
			Info: &persistence.ReplicationTaskInfo{
				DomainID:          r.DomainID,
				WorkflowID:        r.WorkflowID,
				RunID:             r.RunID,
				TaskID:            r.TaskID,
				TaskType:          r.TaskType,
				FirstEventID:      r.FirstEventID,
				NextEventID:       r.NextEventID,
				Version:           r.Version,
				ScheduledID:       r.ScheduledID,
				BranchToken:       r.BranchToken,
				NewRunBranchToken: r.NewRunBranchToken,
				CreationTime:      r.CreationTime.UnixNano(),
			},
			Task: t.Task,
		})
	}
	return &persistence.InternalGetReplicationDLQTasksResponse{
		Tasks:         dlqTasks,
		NextPageToken: nextPageToken,
	}, nil
}

func (d *nosqlExecutionStore) GetReplicationDLQSize(
	ctx context.Context,
	request *persistence.GetReplicationDLQSizeRequest,
) (*persistence.GetReplicationDLQSizeResponse, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "GetReplicationDLQSize")
	if err != nil {
		return nil, err
	}
	size, err := storeShard.db.SelectReplicationDLQTasksCount(ctx, shardID, request.SourceClusterName)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "GetReplicationDLQSize", err)
	}
	return &persistence.GetReplicationDLQSizeResponse{
		Size: size,
	}, nil
}

func (d *nosqlExecutionStore) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *persistence.DeleteReplicationTaskFromDLQRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "DeleteReplicationTaskFromDLQ")
	if err != nil {
		return err
	}
	err = storeShard.db.DeleteReplicationDLQTask(ctx, shardID, request.SourceClusterName, request.TaskID)
	if err != nil {
		return convertCommonErrors(storeShard.db, "DeleteReplicationTaskFromDLQ", err)
	}

	return nil
}

func (d *nosqlExecutionStore) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *persistence.RangeDeleteReplicationTaskFromDLQRequest,
) (*persistence.RangeDeleteReplicationTaskFromDLQResponse, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "RangeDeleteReplicationTaskFromDLQ")
	if err != nil {
		return nil, err
	}
	err = storeShard.db.RangeDeleteReplicationDLQTasks(ctx, shardID, request.SourceClusterName, request.InclusiveBeginTaskID, request.ExclusiveEndTaskID)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "RangeDeleteReplicationTaskFromDLQ", err)
	}

	return &persistence.RangeDeleteReplicationTaskFromDLQResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) CreateFailoverMarkerTasks(
	ctx context.Context,
	request *persistence.CreateFailoverMarkersRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "CreateFailoverMarkerTasks")
	if err != nil {
		return err
	}

	var nosqlTasks []*nosqlplugin.HistoryMigrationTask
	for i, task := range request.Markers {
		ts := []persistence.Task{task}

		tasks, err := d.prepareReplicationTasksForWorkflowTxn(task.DomainID, rowTypeReplicationWorkflowID, rowTypeReplicationRunID, ts)
		if err != nil {
			return err
		}
		tasks[i].Replication.CurrentTimeStamp = request.CurrentTimeStamp
		nosqlTasks = append(nosqlTasks, tasks...)
	}

	err = storeShard.db.InsertReplicationTask(ctx, nosqlTasks, nosqlplugin.ShardCondition{
		ShardID: shardID,
		RangeID: request.RangeID,
	})

	if err != nil {
		conditionFailureErr, isConditionFailedError := err.(*nosqlplugin.ShardOperationConditionFailure)
		if isConditionFailedError {
			return &persistence.ShardOwnershipLostError{
				ShardID: shardID,
				Msg: fmt.Sprintf("Failed to create workflow execution.  Request RangeID: %v, columns: (%v)",
					conditionFailureErr.RangeID, conditionFailureErr.Details),
			}
		}
	}
	return nil
}

// CreateHistoryTasks allows direct injection of tasks without a corresponding workflow update.
// It supports transfer and timer tasks - replication tasks are not supported.
// Returns an error when an unsupported category is provided or when the task insertion fails.
func (d *nosqlExecutionStore) CreateHistoryTasks(
	ctx context.Context,
	request *persistence.CreateHistoryTasksRequest,
) error {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "CreateHistoryTasks")
	if err != nil {
		return err
	}

	for category := range request.TasksByCategory {
		if id := category.ID(); id != persistence.HistoryTaskCategoryIDTransfer && id != persistence.HistoryTaskCategoryIDTimer {
			return &types.BadRequestError{Message: fmt.Sprintf("CreateHistoryTasks only supports transfer and timer tasks, got category: %v", id)}
		}
	}

	outputTasks := make(map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask)
	// The transfer/timer prepare helpers read each task's own (domainID, workflowID, runID) from
	// its info, so no per-workflow grouping is needed here; the runID argument is unused for these
	// categories. The cassandra writer likewise derives each row's owner from the task itself.
	if err := d.prepareNoSQLTasksForWorkflowTxn("", "", "", request.TasksByCategory, outputTasks); err != nil {
		return err
	}

	err = storeShard.db.InsertHistoryTasks(ctx, outputTasks, request.CurrentTimeStamp, nosqlplugin.ShardCondition{
		ShardID: shardID,
		RangeID: request.RangeID,
	})

	if err != nil {
		conditionFailureErr, isConditionFailedError := err.(*nosqlplugin.ShardOperationConditionFailure)
		if isConditionFailedError {
			return &persistence.ShardOwnershipLostError{
				ShardID: shardID,
				Msg: fmt.Sprintf("Failed to create history tasks.  Request RangeID: %v, columns: (%v)",
					conditionFailureErr.RangeID, conditionFailureErr.Details),
			}
		}
		return err
	}
	return nil
}

func (d *nosqlExecutionStore) GetHistoryTasks(
	ctx context.Context,
	request *persistence.GetHistoryTasksRequest,
) (*persistence.GetHistoryTasksResponse, error) {
	shardID, _, err := d.storeShardForRequest(request.ShardID, "GetHistoryTasks")
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.getImmediateHistoryTasks(ctx, request, shardID)
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.getScheduledHistoryTasks(ctx, request, shardID)
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) getImmediateHistoryTasks(
	ctx context.Context,
	request *persistence.GetHistoryTasksRequest,
	shardID int,
) (*persistence.GetHistoryTasksResponse, error) {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		tasks, nextPageToken, err := storeShard.db.SelectTransferTasksOrderByTaskID(ctx, shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(tasks))
		for _, t := range tasks {
			if storeShard.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Transfer.ToTask()
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
				}
				tTasks = append(tTasks, task)
			}
		}
		return &persistence.GetHistoryTasksResponse{
			Tasks:         tTasks,
			NextPageToken: nextPageToken,
		}, nil
	case persistence.HistoryTaskCategoryIDReplication:
		tasks, nextPageToken, err := storeShard.db.SelectReplicationTasksOrderByTaskID(ctx, shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(tasks))
		for _, t := range tasks {
			if storeShard.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Replication.ToTask()
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetImmediateHistoryTasks", err)
				}
				tTasks = append(tTasks, task)
			}
		}
		return &persistence.GetHistoryTasksResponse{
			Tasks:         tTasks,
			NextPageToken: nextPageToken,
		}, nil
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
}

func (d *nosqlExecutionStore) getScheduledHistoryTasks(
	ctx context.Context,
	request *persistence.GetHistoryTasksRequest,
	shardID int,
) (*persistence.GetHistoryTasksResponse, error) {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		timers, nextPageToken, err := storeShard.db.SelectTimerTasksOrderByVisibilityTime(ctx, shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetScheduledTime(), request.ExclusiveMaxTaskKey.GetScheduledTime())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "GetScheduledHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(timers))
		for _, t := range timers {
			if storeShard.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetScheduledHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				task.SetVisibilityTimestamp(t.ScheduledTime)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Timer.ToTask()
				if err != nil {
					return nil, convertCommonErrors(storeShard.db, "GetScheduledHistoryTasks", err)
				}
				tTasks = append(tTasks, task)
			}
		}
		return &persistence.GetHistoryTasksResponse{
			Tasks:         tTasks,
			NextPageToken: nextPageToken,
		}, nil
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
}

func (d *nosqlExecutionStore) CompleteHistoryTask(
	ctx context.Context,
	request *persistence.CompleteHistoryTaskRequest,
) error {
	shardID, _, err := d.storeShardForRequest(request.ShardID, "CompleteHistoryTask")
	if err != nil {
		return err
	}
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.completeScheduledHistoryTask(ctx, request, shardID)
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.completeImmediateHistoryTask(ctx, request, shardID)
	default:
		return &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) completeScheduledHistoryTask(
	ctx context.Context,
	request *persistence.CompleteHistoryTaskRequest,
	shardID int,
) error {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		err = storeShard.db.DeleteTimerTask(ctx, shardID, request.TaskKeys)
		if err != nil {
			return convertCommonErrors(storeShard.db, "CompleteScheduledHistoryTask", err)
		}
		return nil
	default:
		return &types.BadRequestError{Message: fmt.Sprintf("Unknown task category ID: %v", request.TaskCategory.ID())}
	}
}

func (d *nosqlExecutionStore) completeImmediateHistoryTask(
	ctx context.Context,
	request *persistence.CompleteHistoryTaskRequest,
	shardID int,
) error {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		err = storeShard.db.DeleteTransferTask(ctx, shardID, request.TaskKeys)
		if err != nil {
			return convertCommonErrors(storeShard.db, "CompleteImmediateHistoryTask", err)
		}
		return nil
	case persistence.HistoryTaskCategoryIDReplication:
		err = storeShard.db.DeleteReplicationTask(ctx, shardID, request.TaskKeys)
		if err != nil {
			return convertCommonErrors(storeShard.db, "CompleteImmediateHistoryTask", err)
		}
		return nil
	default:
		return &types.BadRequestError{Message: fmt.Sprintf("Unknown task category ID: %v", request.TaskCategory.ID())}
	}
}

func (d *nosqlExecutionStore) RangeCompleteHistoryTask(
	ctx context.Context,
	request *persistence.RangeCompleteHistoryTaskRequest,
) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	shardID, _, err := d.storeShardForRequest(request.ShardID, "RangeCompleteHistoryTask")
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.rangeCompleteScheduledHistoryTask(ctx, request, shardID)
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.rangeCompleteImmediateHistoryTask(ctx, request, shardID)
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) rangeCompleteScheduledHistoryTask(
	ctx context.Context,
	request *persistence.RangeCompleteHistoryTaskRequest,
	shardID int,
) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		err = storeShard.db.RangeDeleteTimerTasks(ctx, shardID, request.InclusiveMinTaskKey.GetScheduledTime(), request.ExclusiveMaxTaskKey.GetScheduledTime())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "RangeCompleteTimerTask", err)
		}
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
	return &persistence.RangeCompleteHistoryTaskResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) rangeCompleteImmediateHistoryTask(
	ctx context.Context,
	request *persistence.RangeCompleteHistoryTaskRequest,
	shardID int,
) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	storeShard, err := d.GetStoreShardByHistoryShard(shardID)
	if err != nil {
		return nil, err
	}
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		err = storeShard.db.RangeDeleteTransferTasks(ctx, shardID, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "RangeCompleteTransferTask", err)
		}
	case persistence.HistoryTaskCategoryIDReplication:
		err = storeShard.db.RangeDeleteReplicationTasks(ctx, shardID, request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(storeShard.db, "RangeCompleteReplicationTask", err)
		}
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
	return &persistence.RangeCompleteHistoryTaskResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) GetActiveClusterSelectionPolicy(
	ctx context.Context,
	request *persistence.GetActiveClusterSelectionPolicyRequest,
) (*persistence.DataBlob, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "GetActiveClusterSelectionPolicy")
	if err != nil {
		return nil, err
	}
	row, err := storeShard.db.SelectActiveClusterSelectionPolicy(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		if storeShard.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Active cluster selection policy not found.  DomainId: %v, WorkflowId: %v, RunId: %v", request.DomainID, request.WorkflowID, request.RunID),
			}
		}

		return nil, convertCommonErrors(storeShard.db, "GetActiveClusterSelectionPolicy", err)
	}

	if row == nil {
		return nil, nil
	}

	return row.Policy, nil
}

func (d *nosqlExecutionStore) SelectWorkflowTimerTasks(
	ctx context.Context,
	request *persistence.SelectWorkflowTimerTasksRequest,
) ([]persistence.HistoryTaskKey, error) {
	shardID, storeShard, err := d.storeShardForRequest(request.ShardID, "SelectWorkflowTimerTasks")
	if err != nil {
		return nil, err
	}
	result, err := storeShard.db.SelectWorkflowTimerTasks(ctx, shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return nil, convertCommonErrors(storeShard.db, "SelectWorkflowTimerTasks", err)
	}
	return result, nil
}
