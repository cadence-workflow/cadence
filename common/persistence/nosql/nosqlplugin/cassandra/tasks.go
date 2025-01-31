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

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
)

const (
	// Row types for table tasks
	rowTypeTask = iota
	rowTypeTaskList
)

const (
	taskListTaskID = -12345
	initialRangeID = 1 // Id of the first range of a new task list
)

// SelectTaskList returns a single tasklist row.
// Return IsNotFoundError if the row doesn't exist
func (db *cdb) SelectTaskList(ctx context.Context, filter *nosqlplugin.TaskListFilter) (*nosqlplugin.TaskListRow, error) {
	query := db.session.Query(templateGetTaskList,
		filter.DomainID,
		filter.TaskListName,
		filter.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
	).WithContext(ctx)
	var rangeID int64
	var tlDB map[string]interface{}
	err := query.Scan(&rangeID, &tlDB)
	if err != nil {
		return nil, err
	}
	ackLevel := tlDB["ack_level"].(int64)
	taskListKind := tlDB["kind"].(int)
	lastUpdatedTime := tlDB["last_updated"].(time.Time)

	return &nosqlplugin.TaskListRow{
		DomainID:     filter.DomainID,
		TaskListName: filter.TaskListName,
		TaskListType: filter.TaskListType,

		TaskListKind:            taskListKind,
		LastUpdatedTime:         lastUpdatedTime,
		AckLevel:                ackLevel,
		RangeID:                 rangeID,
		AdaptivePartitionConfig: toTaskListPartitionConfig(tlDB["adaptive_partition_config"]),
	}, nil
}

func toTaskListPartitionConfig(v interface{}) *persistence.TaskListPartitionConfig {
	if v == nil {
		return nil
	}
	partition := v.(map[string]interface{})
	if len(partition) == 0 {
		return nil
	}
	version := partition["version"].(int64)
	numRead := partition["num_read_partitions"].(int)
	numWrite := partition["num_write_partitions"].(int)
	readPartitions := toTaskListPartitions(partition["read_partitions"])
	writePartitions := toTaskListPartitions(partition["write_partitions"])
	// If they're out of sync, go with the value of num_*_partitions. This is necessary only while support for
	// read_partitions and write_partitions rolls out
	if numRead != len(readPartitions) {
		readPartitions = createDefaultPartitions(numRead)
	}
	if numWrite != len(writePartitions) {
		writePartitions = createDefaultPartitions(numWrite)
	}
	return &persistence.TaskListPartitionConfig{
		Version:         version,
		ReadPartitions:  readPartitions,
		WritePartitions: writePartitions,
	}
}

func createDefaultPartitions(num int) map[int]*persistence.TaskListPartition {
	partitions := make(map[int]*persistence.TaskListPartition, num)
	for i := 0; i < num; i++ {
		partitions[i] = &persistence.TaskListPartition{}
	}
	return partitions
}

func toTaskListPartitions(values any) map[int]*persistence.TaskListPartition {
	if values == nil {
		return nil
	}
	partitions, ok := values.(map[int]map[string]any)
	if !ok || len(partitions) == 0 {
		return nil
	}
	result := make(map[int]*persistence.TaskListPartition, len(partitions))
	for id, p := range partitions {
		partition := toTaskListPartition(p)
		if partition != nil {
			result[id] = partition
		}
	}
	return result
}

func toTaskListPartition(partition map[string]any) *persistence.TaskListPartition {
	if len(partition) == 0 {
		return nil
	}
	isolationGroups := partition["isolation_groups"].([]string)
	return &persistence.TaskListPartition{
		IsolationGroups: isolationGroups,
	}
}

func fromTaskListPartitionConfig(config *persistence.TaskListPartitionConfig) map[string]interface{} {
	if config == nil {
		return nil
	}
	return map[string]interface{}{
		"version":              config.Version,
		"num_read_partitions":  len(config.ReadPartitions),
		"num_write_partitions": len(config.WritePartitions),
		"read_partitions":      fromTaskListPartitions(config.ReadPartitions),
		"write_partitions":     fromTaskListPartitions(config.WritePartitions),
	}
}

func fromTaskListPartitions(partitions map[int]*persistence.TaskListPartition) map[int]any {
	if len(partitions) == 0 {
		return nil
	}
	result := make(map[int]any, len(partitions))
	for id, partition := range partitions {
		result[id] = fromTaskListPartition(partition)
	}
	return result
}

func fromTaskListPartition(partition *persistence.TaskListPartition) any {
	if partition == nil {
		return nil
	}
	return map[string]any{
		"isolation_groups": partition.IsolationGroups,
	}
}

// InsertTaskList insert a single tasklist row
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *cdb) InsertTaskList(ctx context.Context, row *nosqlplugin.TaskListRow) error {
	timeStamp := db.timeSrc.Now()
	query := db.session.Query(templateInsertTaskListQuery,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
		initialRangeID,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		0,
		row.TaskListKind,
		row.LastUpdatedTime,
		fromTaskListPartitionConfig(row.AdaptivePartitionConfig),
		timeStamp,
	).WithContext(ctx)

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		return err
	}

	return handleTaskListAppliedError(applied, previous)
}

// UpdateTaskList updates a single tasklist row
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *cdb) UpdateTaskList(
	ctx context.Context,
	row *nosqlplugin.TaskListRow,
	previousRangeID int64,
) error {
	timeStamp := db.timeSrc.Now()
	query := db.session.Query(templateUpdateTaskListQuery,
		row.RangeID,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		row.AckLevel,
		row.TaskListKind,
		row.LastUpdatedTime,
		fromTaskListPartitionConfig(row.AdaptivePartitionConfig),
		timeStamp,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
		previousRangeID,
	).WithContext(ctx)

	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		return err
	}

	return handleTaskListAppliedError(applied, previous)
}

func handleTaskListAppliedError(applied bool, previous map[string]interface{}) error {
	if !applied {
		// NOTE: Cassandra only returns the conflicted columns in this results
		rangeID := previous["range_id"].(int64)
		var columns []string
		for k, v := range previous {
			columns = append(columns, fmt.Sprintf("%s=%v", k, v))
		}
		return &nosqlplugin.TaskOperationConditionFailure{
			RangeID: rangeID,
			Details: strings.Join(columns, ","),
		}
	}
	return nil
}

// UpdateTaskList updates a single tasklist row, and set an TTL on the record
// Return TaskOperationConditionFailure if the condition doesn't meet
// Ignore TTL if it's not supported, which becomes exactly the same as UpdateTaskList, but ListTaskList must be
// implemented for TaskListScavenger
func (db *cdb) UpdateTaskListWithTTL(
	ctx context.Context,
	ttlSeconds int64,
	row *nosqlplugin.TaskListRow,
	previousRangeID int64,
) error {
	timeStamp := db.timeSrc.Now()
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	// part 1 is used to set TTL on primary key as UPDATE can't set TTL for primary key
	batch.Query(templateUpdateTaskListQueryWithTTLPart1,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
		timeStamp,
		ttlSeconds,
	)
	// part 2 is for CAS and setting TTL for the rest of the columns
	batch.Query(templateUpdateTaskListQueryWithTTLPart2,
		ttlSeconds,
		row.RangeID,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		row.AckLevel,
		row.TaskListKind,
		timeStamp,
		fromTaskListPartitionConfig(row.AdaptivePartitionConfig),
		timeStamp,
		row.DomainID,
		row.TaskListName,
		row.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
		previousRangeID,
	)
	previous := make(map[string]interface{})
	applied, _, err := db.session.MapExecuteBatchCAS(batch, previous)
	if err != nil {
		return err
	}
	return handleTaskListAppliedError(applied, previous)
}

// ListTaskList returns all tasklists.
// Noop if TTL is already implemented in other methods
func (db *cdb) ListTaskList(ctx context.Context, pageSize int, nextPageToken []byte) (*nosqlplugin.ListTaskListResult, error) {
	return nil, &types.InternalServiceError{
		Message: "unsupported operation",
	}
}

// DeleteTaskList deletes a single tasklist row
// Return TaskOperationConditionFailure if the condition doesn't meet
func (db *cdb) DeleteTaskList(ctx context.Context, filter *nosqlplugin.TaskListFilter, previousRangeID int64) error {
	query := db.session.Query(templateDeleteTaskListQuery,
		filter.DomainID,
		filter.TaskListName,
		filter.TaskListType,
		rowTypeTaskList,
		taskListTaskID,
		previousRangeID,
	).WithContext(ctx).Consistency(cassandraAllConslevel)
	previous := make(map[string]interface{})
	applied, err := query.MapScanCAS(previous)
	if err != nil {
		if !db.isCassandraConsistencyError(err) {
			return err
		}
		db.logger.Warn("unable to complete the delete operation due to consistency issue", tag.Error(err))
		applied, err = query.Consistency(cassandraDefaultConsLevel).MapScanCAS(previous)
		if err != nil {
			return err
		}
	}
	return handleTaskListAppliedError(applied, previous)
}

// InsertTasks inserts a batch of tasks
// Return IsConditionFailedError if the condition doesn't meet, and also the previous tasklist row
func (db *cdb) InsertTasks(
	ctx context.Context,
	tasksToInsert []*nosqlplugin.TaskRowForInsert,
	tasklistCondition *nosqlplugin.TaskListRow,
) error {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	domainID := tasklistCondition.DomainID
	taskListName := tasklistCondition.TaskListName
	taskListType := tasklistCondition.TaskListType
	timeStamp := db.timeSrc.Now()

	for _, task := range tasksToInsert {
		scheduleID := task.ScheduledID
		ttl := int64(task.TTLSeconds)
		if ttl <= 0 {
			batch.Query(templateCreateTaskQuery,
				domainID,
				taskListName,
				taskListType,
				rowTypeTask,
				task.TaskID,
				domainID,
				task.WorkflowID,
				task.RunID,
				scheduleID,
				task.CreatedTime,
				task.PartitionConfig,
				timeStamp,
			)
		} else {
			if ttl > maxCassandraTTL {
				ttl = maxCassandraTTL
			}
			batch.Query(templateCreateTaskWithTTLQuery,
				domainID,
				taskListName,
				taskListType,
				rowTypeTask,
				task.TaskID,
				domainID,
				task.WorkflowID,
				task.RunID,
				scheduleID,
				task.CreatedTime,
				task.PartitionConfig,
				timeStamp,
				ttl)
		}
	}

	// The following query is used to ensure that range_id didn't change
	batch.Query(templateUpdateTaskListRangeIDQuery,
		tasklistCondition.RangeID,
		timeStamp,
		domainID,
		taskListName,
		taskListType,
		rowTypeTaskList,
		taskListTaskID,
		tasklistCondition.RangeID,
	)

	previous := make(map[string]interface{})
	applied, _, err := db.session.MapExecuteBatchCAS(batch, previous)
	if err != nil {
		return err
	}
	return handleTaskListAppliedError(applied, previous)
}

// GetTasksCount returns number of tasks from a tasklist
func (db *cdb) GetTasksCount(ctx context.Context, filter *nosqlplugin.TasksFilter) (int64, error) {
	query := db.session.Query(templateGetTasksCountQuery,
		filter.DomainID,
		filter.TaskListName,
		filter.TaskListType,
		rowTypeTask,
		filter.MinTaskID,
	).WithContext(ctx)
	result := make(map[string]interface{})
	if err := query.MapScan(result); err != nil {
		return 0, err
	}

	queueSize := result["count"].(int64)
	return queueSize, nil
}

// SelectTasks return tasks that associated to a tasklist
func (db *cdb) SelectTasks(ctx context.Context, filter *nosqlplugin.TasksFilter) ([]*nosqlplugin.TaskRow, error) {
	// Reading tasklist tasks need to be quorum level consistent, otherwise we could lose tasks
	query := db.session.Query(templateGetTasksQuery,
		filter.DomainID,
		filter.TaskListName,
		filter.TaskListType,
		rowTypeTask,
		filter.MinTaskID,
		filter.MaxTaskID,
	).PageSize(filter.BatchSize).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, fmt.Errorf("selectTasks operation failed.  Not able to create query iterator")
	}

	var response []*nosqlplugin.TaskRow
	task := make(map[string]interface{})
PopulateTasks:
	for iter.MapScan(task) {
		taskID, ok := task["task_id"]
		if !ok { // no tasks, but static column record returned
			continue
		}

		// Extract the TTL value
		ttlValue, ttlExists := task["ttl"]

		// Check if TTL is null or an integer
		var ttl *int
		if ttlExists && ttlValue != nil {
			if ttlInt, ok := ttlValue.(int); ok {
				ttl = &ttlInt // TTL is an integer
			}
		}

		t := createTaskInfo(task["task"].(map[string]interface{}))
		t.TaskID = taskID.(int64)

		if ttl != nil {
			t.Expiry = db.timeSrc.Now().Add(time.Duration(*ttl) * time.Second)
		}

		response = append(response, t)
		if len(response) == filter.BatchSize {
			break PopulateTasks
		}
		task = make(map[string]interface{}) // Reinitialize map as initialized fails on unmarshalling
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return response, nil
}

func createTaskInfo(
	result map[string]interface{},
) *nosqlplugin.TaskRow {

	info := &nosqlplugin.TaskRow{}
	for k, v := range result {
		switch k {
		case "domain_id":
			info.DomainID = v.(gocql.UUID).String()
		case "workflow_id":
			info.WorkflowID = v.(string)
		case "run_id":
			info.RunID = v.(gocql.UUID).String()
		case "schedule_id":
			info.ScheduledID = v.(int64)
		case "created_time":
			info.CreatedTime = v.(time.Time)
		case "partition_config":
			info.PartitionConfig = v.(map[string]string)
		}
	}

	return info
}

// DeleteTask delete a batch tasks that taskIDs less than the row
// If TTL is not implemented, then should also return the number of rows deleted, otherwise persistence.UnknownNumRowsAffected
// NOTE: This API ignores the `BatchSize` request parameter i.e. either all tasks leq the task_id will be deleted or an error will
// be returned to the caller, because rowsDeleted is not supported by Cassandra
func (db *cdb) RangeDeleteTasks(ctx context.Context, filter *nosqlplugin.TasksFilter) (rowsDeleted int, err error) {
	query := db.session.Query(templateCompleteTasksLessThanQuery,
		filter.DomainID,
		filter.TaskListName,
		filter.TaskListType,
		rowTypeTask,
		filter.MinTaskID,
		filter.MaxTaskID,
	).WithContext(ctx)
	return persistence.UnknownNumRowsAffected, db.executeWithConsistencyAll(query)
}
