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

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

// Implements ExecutionStore
type nosqlExecutionStore struct {
	shardID int
	nosqlStore
	taskSerializer serialization.TaskSerializer
}

// NewExecutionStore is used to create an instance of ExecutionStore implementation
func NewExecutionStore(
	shardID int,
	db nosqlplugin.DB,
	logger log.Logger,
	taskSerializer serialization.TaskSerializer,
	dc *persistence.DynamicConfiguration,
) (persistence.ExecutionStore, error) {
	return &nosqlExecutionStore{
		nosqlStore: nosqlStore{
			logger: logger,
			db:     db,
			dc:     dc,
		},
		shardID:        shardID,
		taskSerializer: taskSerializer,
	}, nil
}

func (d *nosqlExecutionStore) GetShardID() int {
	return d.shardID
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

	workflowRequestWriteMode, err := getWorkflowRequestWriteMode(request.WorkflowRequestMode)
	if err != nil {
		return nil, err
	}

	currentWorkflowWriteReq, err := d.prepareCurrentWorkflowRequestForCreateWorkflowTxn(domainID, workflowID, runID, executionInfo, lastWriteVersion, request)
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

	workflowRequests := d.prepareWorkflowRequestRows(domainID, workflowID, runID, newWorkflow.WorkflowRequests, nil)
	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	activeClusterSelectionPolicyRow := d.prepareActiveClusterSelectionPolicyRow(domainID, workflowID, runID, executionInfo.ActiveClusterSelectionPolicy)

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: d.shardID,
		RangeID: request.RangeID,
	}

	err = d.db.InsertWorkflowExecutionWithTasks(
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
					ShardID: d.shardID,
					Msg:     *conditionFailureErr.UnknownConditionFailureDetails,
				}
			case conditionFailureErr.ShardRangeIDNotMatch != nil:
				return nil, &persistence.ShardOwnershipLostError{
					ShardID: d.shardID,
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
				d.logger.Error("A code bug exists in persistence layer, please investigate ASAP", tag.Error(err))
				return nil, err
			}
		}
		return nil, convertCommonErrors(d.db, "CreateWorkflowExecution", err)
	}

	return &persistence.CreateWorkflowExecutionResponse{}, nil
}

func (d *nosqlExecutionStore) GetWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalGetWorkflowExecutionRequest,
) (*persistence.InternalGetWorkflowExecutionResponse, error) {

	execution := request.Execution
	state, err := d.db.SelectWorkflowExecution(ctx, d.shardID, request.DomainID, execution.WorkflowID, execution.RunID)
	if err != nil {
		if d.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v, RunId: %v",
					execution.WorkflowID, execution.RunID),
			}
		}

		return nil, convertCommonErrors(d.db, "GetWorkflowExecution", err)
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
					ShardID:          d.shardID,
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
					ShardID:          d.shardID,
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
	workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, runID, updateWorkflow.WorkflowRequests, workflowRequests)

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
		)

		err = d.prepareNoSQLTasksForWorkflowTxn(
			domainID, workflowID, newWorkflow.ExecutionInfo.RunID,
			newWorkflow.TasksByCategory,
			tasksByCategory,
		)
		if err != nil {
			return err
		}
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, newWorkflow.ExecutionInfo.RunID, newWorkflow.WorkflowRequests, workflowRequests)
	}

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: d.shardID,
		RangeID: request.RangeID,
	}

	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	err = d.db.UpdateWorkflowExecutionWithTasks(
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

	return d.processUpdateWorkflowResult(err, request.RangeID)
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
				ShardID:          d.shardID,
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
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, currentWorkflow.ExecutionInfo.RunID, currentWorkflow.WorkflowRequests, workflowRequests)
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
	workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, resetWorkflow.ExecutionInfo.RunID, resetWorkflow.WorkflowRequests, workflowRequests)

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
		workflowRequests = d.prepareWorkflowRequestRows(domainID, workflowID, newWorkflow.ExecutionInfo.RunID, newWorkflow.WorkflowRequests, workflowRequests)
	}

	shardCondition := &nosqlplugin.ShardCondition{
		ShardID: d.shardID,
		RangeID: request.RangeID,
	}

	workflowRequestsWriteRequest := &nosqlplugin.WorkflowRequestsWriteRequest{
		Rows:      workflowRequests,
		WriteMode: workflowRequestWriteMode,
	}

	err = d.db.UpdateWorkflowExecutionWithTasks(
		ctx, workflowRequestsWriteRequest, currentWorkflowWriteReq,
		mutateExecution, insertExecution, nil, resetExecution,
		tasksByCategory,
		shardCondition)
	return d.processUpdateWorkflowResult(err, request.RangeID)
}

func (d *nosqlExecutionStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *persistence.DeleteWorkflowExecutionRequest,
) error {
	err := d.db.DeleteWorkflowExecution(ctx, d.shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(d.db, "DeleteWorkflowExecution", err)
	}

	return nil
}

func (d *nosqlExecutionStore) DeleteCurrentWorkflowExecution(
	ctx context.Context,
	request *persistence.DeleteCurrentWorkflowExecutionRequest,
) error {
	err := d.db.DeleteCurrentWorkflow(ctx, d.shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(d.db, "DeleteCurrentWorkflowExecution", err)
	}

	return nil
}

func (d *nosqlExecutionStore) DeleteActiveClusterSelectionPolicy(
	ctx context.Context,
	domainID, workflowID, runID string,
) error {
	err := d.db.DeleteActiveClusterSelectionPolicy(ctx, d.shardID, domainID, workflowID, runID)
	if err != nil {
		return convertCommonErrors(d.db, "DeleteActiveClusterSelectionPolicy", err)
	}
	return nil
}

func (d *nosqlExecutionStore) GetCurrentExecution(
	ctx context.Context,
	request *persistence.GetCurrentExecutionRequest,
) (*persistence.GetCurrentExecutionResponse,
	error) {
	result, err := d.db.SelectCurrentWorkflow(ctx, d.shardID, request.DomainID, request.WorkflowID)

	if err != nil {
		if d.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v",
					request.WorkflowID),
			}
		}
		return nil, convertCommonErrors(d.db, "GetCurrentExecution", err)
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
	executions, token, err := d.db.SelectAllCurrentWorkflows(ctx, d.shardID, request.PageToken, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(d.db, "ListCurrentExecutions", err)
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
	exists, err := d.db.IsWorkflowExecutionExists(ctx, d.shardID, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return nil, convertCommonErrors(d.db, "IsWorkflowExecutionExists", err)
	}
	return &persistence.IsWorkflowExecutionExistsResponse{
		Exists: exists,
	}, nil
}

func (d *nosqlExecutionStore) ListConcreteExecutions(
	ctx context.Context,
	request *persistence.ListConcreteExecutionsRequest,
) (*persistence.InternalListConcreteExecutionsResponse, error) {
	executions, nextPageToken, err := d.db.SelectAllWorkflowExecutions(ctx, d.shardID, request.PageToken, request.PageSize)
	if err != nil {
		return nil, convertCommonErrors(d.db, "ListConcreteExecutions", err)
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
	err := d.db.InsertReplicationDLQTask(ctx, d.shardID, request.SourceClusterName, &nosqlplugin.HistoryMigrationTask{
		Replication: request.TaskInfo,
		Task:        nil, // TODO: encode task infor into datablob
	})
	if err != nil {
		return convertCommonErrors(d.db, "PutReplicationTaskToDLQ", err)
	}

	return nil
}

func (d *nosqlExecutionStore) GetReplicationTasksFromDLQ(
	ctx context.Context,
	request *persistence.GetReplicationTasksFromDLQRequest,
) (*persistence.GetHistoryTasksResponse, error) {
	if request.ReadLevel > request.MaxReadLevel {
		return nil, &types.BadRequestError{Message: "ReadLevel cannot be higher than MaxReadLevel"}
	}
	tasks, nextPageToken, err := d.db.SelectReplicationDLQTasksOrderByTaskID(ctx, d.shardID, request.SourceClusterName, request.BatchSize, request.NextPageToken, request.ReadLevel, request.MaxReadLevel)
	if err != nil {
		return nil, convertCommonErrors(d.db, "GetReplicationTasksFromDLQ", err)
	}
	var tTasks []persistence.Task
	for _, t := range tasks {
		task, err := t.Replication.ToTask()
		if err != nil {
			return nil, convertCommonErrors(d.db, "GetReplicationTasksFromDLQ", err)
		}
		tTasks = append(tTasks, task)
	}
	return &persistence.GetHistoryTasksResponse{
		Tasks:         tTasks,
		NextPageToken: nextPageToken,
	}, nil
}

func (d *nosqlExecutionStore) GetReplicationDLQSize(
	ctx context.Context,
	request *persistence.GetReplicationDLQSizeRequest,
) (*persistence.GetReplicationDLQSizeResponse, error) {

	size, err := d.db.SelectReplicationDLQTasksCount(ctx, d.shardID, request.SourceClusterName)
	if err != nil {
		return nil, convertCommonErrors(d.db, "GetReplicationDLQSize", err)
	}
	return &persistence.GetReplicationDLQSizeResponse{
		Size: size,
	}, nil
}

func (d *nosqlExecutionStore) DeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *persistence.DeleteReplicationTaskFromDLQRequest,
) error {

	err := d.db.DeleteReplicationDLQTask(ctx, d.shardID, request.SourceClusterName, request.TaskID)
	if err != nil {
		return convertCommonErrors(d.db, "DeleteReplicationTaskFromDLQ", err)
	}

	return nil
}

func (d *nosqlExecutionStore) RangeDeleteReplicationTaskFromDLQ(
	ctx context.Context,
	request *persistence.RangeDeleteReplicationTaskFromDLQRequest,
) (*persistence.RangeDeleteReplicationTaskFromDLQResponse, error) {

	err := d.db.RangeDeleteReplicationDLQTasks(ctx, d.shardID, request.SourceClusterName, request.InclusiveBeginTaskID, request.ExclusiveEndTaskID)
	if err != nil {
		return nil, convertCommonErrors(d.db, "RangeDeleteReplicationTaskFromDLQ", err)
	}

	return &persistence.RangeDeleteReplicationTaskFromDLQResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) CreateFailoverMarkerTasks(
	ctx context.Context,
	request *persistence.CreateFailoverMarkersRequest,
) error {

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

	err := d.db.InsertReplicationTask(ctx, nosqlTasks, nosqlplugin.ShardCondition{
		ShardID: d.shardID,
		RangeID: request.RangeID,
	})

	if err != nil {
		conditionFailureErr, isConditionFailedError := err.(*nosqlplugin.ShardOperationConditionFailure)
		if isConditionFailedError {
			return &persistence.ShardOwnershipLostError{
				ShardID: d.shardID,
				Msg: fmt.Sprintf("Failed to create workflow execution.  Request RangeID: %v, columns: (%v)",
					conditionFailureErr.RangeID, conditionFailureErr.Details),
			}
		}
	}
	return nil
}

func (d *nosqlExecutionStore) GetHistoryTasks(
	ctx context.Context,
	request *persistence.GetHistoryTasksRequest,
) (*persistence.GetHistoryTasksResponse, error) {
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.getImmediateHistoryTasks(ctx, request)
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.getScheduledHistoryTasks(ctx, request)
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) getImmediateHistoryTasks(
	ctx context.Context,
	request *persistence.GetHistoryTasksRequest,
) (*persistence.GetHistoryTasksResponse, error) {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		tasks, nextPageToken, err := d.db.SelectTransferTasksOrderByTaskID(ctx, d.shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(tasks))
		for _, t := range tasks {
			if d.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Transfer.ToTask()
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
				}
				tTasks = append(tTasks, task)
			}
		}
		return &persistence.GetHistoryTasksResponse{
			Tasks:         tTasks,
			NextPageToken: nextPageToken,
		}, nil
	case persistence.HistoryTaskCategoryIDReplication:
		tasks, nextPageToken, err := d.db.SelectReplicationTasksOrderByTaskID(ctx, d.shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(tasks))
		for _, t := range tasks {
			if d.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Replication.ToTask()
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetImmediateHistoryTasks", err)
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
) (*persistence.GetHistoryTasksResponse, error) {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		timers, nextPageToken, err := d.db.SelectTimerTasksOrderByVisibilityTime(ctx, d.shardID, request.PageSize, request.NextPageToken, request.InclusiveMinTaskKey.GetScheduledTime(), request.ExclusiveMaxTaskKey.GetScheduledTime())
		if err != nil {
			return nil, convertCommonErrors(d.db, "GetScheduledHistoryTasks", err)
		}
		tTasks := make([]persistence.Task, 0, len(timers))
		for _, t := range timers {
			if d.dc.ReadNoSQLHistoryTaskFromDataBlob() && t.Task != nil {
				task, err := d.taskSerializer.DeserializeTask(request.TaskCategory, t.Task)
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetScheduledHistoryTasks", err)
				}
				task.SetTaskID(t.TaskID)
				task.SetVisibilityTimestamp(t.ScheduledTime)
				tTasks = append(tTasks, task)
			} else {
				task, err := t.Timer.ToTask()
				if err != nil {
					return nil, convertCommonErrors(d.db, "GetScheduledHistoryTasks", err)
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
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.completeScheduledHistoryTask(ctx, request)
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.completeImmediateHistoryTask(ctx, request)
	default:
		return &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) completeScheduledHistoryTask(
	ctx context.Context,
	request *persistence.CompleteHistoryTaskRequest,
) error {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		err := d.db.DeleteTimerTask(ctx, d.shardID, request.TaskKey.GetTaskID(), request.TaskKey.GetScheduledTime())
		if err != nil {
			return convertCommonErrors(d.db, "CompleteScheduledHistoryTask", err)
		}
		return nil
	default:
		return &types.BadRequestError{Message: fmt.Sprintf("Unknown task category ID: %v", request.TaskCategory.ID())}
	}
}

func (d *nosqlExecutionStore) completeImmediateHistoryTask(
	ctx context.Context,
	request *persistence.CompleteHistoryTaskRequest,
) error {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		err := d.db.DeleteTransferTask(ctx, d.shardID, request.TaskKey.GetTaskID())
		if err != nil {
			return convertCommonErrors(d.db, "CompleteImmediateHistoryTask", err)
		}
		return nil
	case persistence.HistoryTaskCategoryIDReplication:
		err := d.db.DeleteReplicationTask(ctx, d.shardID, request.TaskKey.GetTaskID())
		if err != nil {
			return convertCommonErrors(d.db, "CompleteImmediateHistoryTask", err)
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
	switch request.TaskCategory.Type() {
	case persistence.HistoryTaskCategoryTypeScheduled:
		return d.rangeCompleteScheduledHistoryTask(ctx, request)
	case persistence.HistoryTaskCategoryTypeImmediate:
		return d.rangeCompleteImmediateHistoryTask(ctx, request)
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category type: %v", request.TaskCategory.Type())}
	}
}

func (d *nosqlExecutionStore) rangeCompleteScheduledHistoryTask(
	ctx context.Context,
	request *persistence.RangeCompleteHistoryTaskRequest,
) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTimer:
		err := d.db.RangeDeleteTimerTasks(ctx, d.shardID, request.InclusiveMinTaskKey.GetScheduledTime(), request.ExclusiveMaxTaskKey.GetScheduledTime())
		if err != nil {
			return nil, convertCommonErrors(d.db, "RangeCompleteTimerTask", err)
		}
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
	return &persistence.RangeCompleteHistoryTaskResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) rangeCompleteImmediateHistoryTask(
	ctx context.Context,
	request *persistence.RangeCompleteHistoryTaskRequest,
) (*persistence.RangeCompleteHistoryTaskResponse, error) {
	switch request.TaskCategory.ID() {
	case persistence.HistoryTaskCategoryIDTransfer:
		err := d.db.RangeDeleteTransferTasks(ctx, d.shardID, request.InclusiveMinTaskKey.GetTaskID(), request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(d.db, "RangeCompleteTransferTask", err)
		}
	case persistence.HistoryTaskCategoryIDReplication:
		err := d.db.RangeDeleteReplicationTasks(ctx, d.shardID, request.ExclusiveMaxTaskKey.GetTaskID())
		if err != nil {
			return nil, convertCommonErrors(d.db, "RangeCompleteReplicationTask", err)
		}
	default:
		return nil, &types.BadRequestError{Message: fmt.Sprintf("Unknown task category: %v", request.TaskCategory.ID())}
	}
	return &persistence.RangeCompleteHistoryTaskResponse{TasksCompleted: persistence.UnknownNumRowsAffected}, nil
}

func (d *nosqlExecutionStore) GetActiveClusterSelectionPolicy(
	ctx context.Context,
	domainID, wfID, rID string,
) (*persistence.DataBlob, error) {
	row, err := d.db.SelectActiveClusterSelectionPolicy(ctx, d.shardID, domainID, wfID, rID)
	if err != nil {
		if d.db.IsNotFoundError(err) {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Active cluster selection policy not found.  DomainId: %v, WorkflowId: %v, RunId: %v", domainID, wfID, rID),
			}
		}

		return nil, convertCommonErrors(d.db, "GetActiveClusterSelectionPolicy", err)
	}

	if row == nil {
		return nil, nil
	}

	return row.Policy, nil
}

func (d *nosqlExecutionStore) DeleteTimerTask(
	ctx context.Context,
	request *persistence.DeleteTimerTaskRequest,
) error {
	return d.db.DeleteWorkflowTimerTask(
		ctx,
		d.shardID,
		request.VisibilityTimestamp,
		request.TaskID,
	)
}
