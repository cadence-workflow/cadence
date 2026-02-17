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

package mongodb

import (
	"context"
	"time"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

var _ nosqlplugin.WorkflowCRUD = (*mdb)(nil)

func (db *mdb) InsertWorkflowExecutionWithTasks(
	ctx context.Context,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	execution *nosqlplugin.WorkflowExecutionRequest,
	tasksByCategory map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask,
	activeClusterSelectionPolicyRow *nosqlplugin.ActiveClusterSelectionPolicyRow,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	panic("TODO")
}

func (db *mdb) UpdateWorkflowExecutionWithTasks(
	ctx context.Context,
	requests *nosqlplugin.WorkflowRequestsWriteRequest,
	currentWorkflowRequest *nosqlplugin.CurrentWorkflowWriteRequest,
	mutatedExecution *nosqlplugin.WorkflowExecutionRequest,
	insertedExecution *nosqlplugin.WorkflowExecutionRequest,
	activeClusterSelectionPolicyRow *nosqlplugin.ActiveClusterSelectionPolicyRow,
	resetExecution *nosqlplugin.WorkflowExecutionRequest,
	tasksByCategory map[persistence.HistoryTaskCategory][]*nosqlplugin.HistoryMigrationTask,
	shardCondition *nosqlplugin.ShardCondition,
) error {
	panic("TODO")
}

func (db *mdb) SelectCurrentWorkflow(ctx context.Context, shardID int, domainID, workflowID string) (*nosqlplugin.CurrentWorkflowRow, error) {
	panic("TODO")
}

func (db *mdb) SelectWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) (*nosqlplugin.WorkflowExecution, error) {
	panic("TODO")
}

func (db *mdb) DeleteCurrentWorkflow(ctx context.Context, shardID int, domainID, workflowID, currentRunIDCondition string) error {
	panic("TODO")
}

func (db *mdb) DeleteWorkflowExecution(ctx context.Context, shardID int, domainID, workflowID, runID string) error {
	panic("TODO")
}

func (db *mdb) SelectAllCurrentWorkflows(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.CurrentWorkflowExecution, []byte, error) {
	panic("TODO")
}

func (db *mdb) SelectAllWorkflowExecutions(ctx context.Context, shardID int, pageToken []byte, pageSize int) ([]*persistence.InternalListConcreteExecutionsEntity, []byte, error) {
	panic("TODO")
}

func (db *mdb) IsWorkflowExecutionExists(ctx context.Context, shardID int, domainID, workflowID, runID string) (bool, error) {
	panic("TODO")
}

func (db *mdb) SelectTransferTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	panic("TODO")
}

func (db *mdb) DeleteTransferTask(ctx context.Context, shardID int, taskID int64) error {
	panic("TODO")
}

func (db *mdb) RangeDeleteTransferTasks(ctx context.Context, shardID int, inclusiveBeginTaskID, exclusiveEndTaskID int64) error {
	panic("TODO")
}

func (db *mdb) SelectTimerTasksOrderByVisibilityTime(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTime, exclusiveMaxTime time.Time) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	panic("TODO")
}

func (db *mdb) DeleteTimerTask(ctx context.Context, shardID int, taskID int64, visibilityTimestamp time.Time) error {
	panic("TODO")
}

func (db *mdb) RangeDeleteTimerTasks(ctx context.Context, shardID int, inclusiveMinTime, exclusiveMaxTime time.Time) error {
	panic("TODO")
}

func (db *mdb) SelectReplicationTasksOrderByTaskID(ctx context.Context, shardID, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	panic("TODO")
}

func (db *mdb) DeleteReplicationTask(ctx context.Context, shardID int, taskID int64) error {
	panic("TODO")
}

func (db *mdb) RangeDeleteReplicationTasks(ctx context.Context, shardID int, exclusiveEndTaskID int64) error {
	panic("TODO")
}

func (db *mdb) InsertReplicationTask(ctx context.Context, tasks []*nosqlplugin.HistoryMigrationTask, condition nosqlplugin.ShardCondition) error {
	panic("TODO")
}

func (db *mdb) DeleteCrossClusterTask(ctx context.Context, shardID int, targetCluster string, taskID int64) error {
	panic("TODO")
}

func (db *mdb) InsertReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, task *nosqlplugin.HistoryMigrationTask) error {
	panic("TODO")
}

func (db *mdb) SelectReplicationDLQTasksOrderByTaskID(ctx context.Context, shardID int, sourceCluster string, pageSize int, pageToken []byte, inclusiveMinTaskID, exclusiveMaxTaskID int64) ([]*nosqlplugin.HistoryMigrationTask, []byte, error) {
	panic("TODO")
}

func (db *mdb) SelectReplicationDLQTasksCount(ctx context.Context, shardID int, sourceCluster string) (int64, error) {
	panic("TODO")
}

func (db *mdb) DeleteReplicationDLQTask(ctx context.Context, shardID int, sourceCluster string, taskID int64) error {
	panic("TODO")
}

func (db *mdb) RangeDeleteReplicationDLQTasks(ctx context.Context, shardID int, sourceCluster string, inclusiveBeginTaskID, exclusiveEndTaskID int64) error {
	panic("TODO")
}

func (db *mdb) SelectActiveClusterSelectionPolicy(ctx context.Context, shardID int, domainID, wfID, rID string) (*nosqlplugin.ActiveClusterSelectionPolicyRow, error) {
	panic("TODO")
}

func (db *mdb) DeleteActiveClusterSelectionPolicy(ctx context.Context, shardID int, domainID, wfID, rID string) error {
	panic("TODO")
}

func (db *mdb) DeleteWorkflowTimerTask(ctx context.Context, shardID int, visibilityTimestamp time.Time, taskID int64) error {
	panic("TODO")
}
