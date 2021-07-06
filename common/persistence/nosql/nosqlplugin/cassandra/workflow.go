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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

func (db *cdb) InsertWorkflowExecutionWithTasks(
	ctx context.Context,
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

	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	err := db.createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest)
	if err != nil {
		return err
	}

	err = db.createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, execution)
	if err != nil {
		return err
	}

	err = db.createTransferTasks(batch, shardID, domainID, workflowID, transferTasks)
	if err != nil {
		return err
	}
	err = db.createReplicationTasks(batch, shardID, domainID, workflowID, replicationTasks)
	if err != nil {
		return err
	}
	err = db.createCrossClusterTasks(batch, shardID, domainID, workflowID, crossClusterTasks)
	if err != nil {
		return err
	}
	err = db.createTimerTasks(batch, shardID, domainID, workflowID, timerTasks)
	if err != nil {
		return err
	}

	err = db.assertShardRangeID(batch, shardID, shardCondition.RangeID)
	if err != nil {
		return err
	}

	return db.executeCreateWorkflowBatchTransaction(batch, currentWorkflowRequest, execution, shardCondition)
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
	executionInfo := createWorkflowExecutionInfo(result["execution"].(map[string]interface{}))
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

	err := db.createOrUpdateCurrentWorkflow(batch, shardID, domainID, workflowID, currentWorkflowRequest)
	if err != nil {
		return err
	}

	if mutatedExecution != nil {
		err = db.updateWorkflowExecutionAndEventBufferWithMergeAndDeleteMaps(batch, shardID, domainID, workflowID, mutatedExecution)
		if err != nil {
			return err
		}
	}

	if insertedExecution != nil {
		err = db.createWorkflowExecutionWithMergeMaps(batch, shardID, domainID, workflowID, insertedExecution)
		if err != nil {
			return err
		}
	}

	if resetExecution != nil {
		err = db.resetWorkflowExecutionAndMapsAndEventBuffer(batch, shardID, domainID, workflowID, resetExecution)
		if err != nil {
			return err
		}
	}

	err = db.createTransferTasks(batch, shardID, domainID, workflowID, transferTasks)
	if err != nil {
		return err
	}
	err = db.createReplicationTasks(batch, shardID, domainID, workflowID, replicationTasks)
	if err != nil {
		return err
	}
	err = db.createCrossClusterTasks(batch, shardID, domainID, workflowID, crossClusterTasks)
	if err != nil {
		return err
	}
	err = db.createTimerTasks(batch, shardID, domainID, workflowID, timerTasks)
	if err != nil {
		return err
	}

	err = db.assertShardRangeID(batch, shardID, shardCondition.RangeID)
	if err != nil {
		return err
	}

	return db.executeUpdateWorkflowBatchTransaction(batch, currentWorkflowRequest, previousNextEventIDCondition, shardCondition)
}
