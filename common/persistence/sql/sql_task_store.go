// Copyright (c) 2020 Uber Technologies, Inc.
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
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

type sqlTaskStore struct {
	sqlStore
	nShards int
}

var (
	stickyTasksListsTTL = time.Hour * 24
)

// newTaskPersistence creates a new instance of TaskManager
func newTaskPersistence(
	db sqlplugin.DB,
	nShards int,
	log log.Logger,
	parser serialization.Parser,
) (persistence.TaskStore, error) {
	return &sqlTaskStore{
		sqlStore: sqlStore{
			db:     db,
			logger: log,
			parser: parser,
		},
		nShards: nShards,
	}, nil
}

func (m *sqlTaskStore) GetTaskListSize(ctx context.Context, request *persistence.GetTaskListSizeRequest) (*persistence.GetTaskListSizeResponse, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskListName, m.db.GetTotalNumDBShards())
	domainID := serialization.MustParseUUID(request.DomainID)
	size, err := m.db.GetTasksCount(ctx, &sqlplugin.TasksFilter{
		ShardID:      dbShardID,
		DomainID:     domainID,
		TaskListName: request.TaskListName,
		TaskType:     int64(request.TaskListType),
		MinTaskID:    &request.AckLevel,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetTaskListSize", "", err)
	}
	return &persistence.GetTaskListSizeResponse{Size: size}, nil
}

func (m *sqlTaskStore) LeaseTaskList(
	ctx context.Context,
	request *persistence.LeaseTaskListRequest,
) (*persistence.LeaseTaskListResponse, error) {
	var rangeID int64
	var ackLevel int64
	dbShardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskList, m.db.GetTotalNumDBShards())

	domainID := serialization.MustParseUUID(request.DomainID)
	rows, err := m.db.SelectFromTaskLists(ctx, &sqlplugin.TaskListsFilter{
		ShardID:  dbShardID,
		DomainID: &domainID,
		Name:     &request.TaskList,
		TaskType: common.Int64Ptr(int64(request.TaskType))})
	if err != nil {
		if err == sql.ErrNoRows {
			tlInfo := &serialization.TaskListInfo{
				AckLevel:        ackLevel,
				Kind:            int16(request.TaskListKind),
				ExpiryTimestamp: time.Unix(0, 0),
				LastUpdated:     time.Now(),
			}
			blob, err := m.parser.TaskListInfoToBlob(tlInfo)
			if err != nil {
				return nil, err
			}
			row := sqlplugin.TaskListsRow{
				ShardID:      dbShardID,
				DomainID:     domainID,
				Name:         request.TaskList,
				TaskType:     int64(request.TaskType),
				Data:         blob.Data,
				DataEncoding: string(blob.Encoding),
			}
			rows = []sqlplugin.TaskListsRow{row}
			if m.db.SupportsTTL() && request.TaskListKind == persistence.TaskListKindSticky {
				rowWithTTL := sqlplugin.TaskListsRowWithTTL{
					TaskListsRow: row,
					TTL:          stickyTasksListsTTL,
				}
				if _, err := m.db.InsertIntoTaskListsWithTTL(ctx, &rowWithTTL); err != nil {
					return nil, convertCommonErrors(m.db, "LeaseTaskListWithTTL", fmt.Sprintf("Failed to make task list %v of type %v.", request.TaskList, request.TaskType), err)
				}
			} else {
				if _, err := m.db.InsertIntoTaskLists(ctx, &row); err != nil {
					return nil, convertCommonErrors(m.db, "LeaseTaskList", fmt.Sprintf("Failed to make task list %v of type %v.", request.TaskList, request.TaskType), err)
				}
			}
		} else {
			return nil, convertCommonErrors(m.db, "LeaseTaskList", "Failed to check if task list existed.", err)
		}
	}

	row := rows[0]
	if request.RangeID > 0 && request.RangeID != row.RangeID {
		return nil, &persistence.ConditionFailedError{
			Msg: fmt.Sprintf("leaseTaskList:renew failed:taskList:%v, taskListType:%v, haveRangeID:%v, gotRangeID:%v",
				request.TaskList, request.TaskType, rangeID, row.RangeID),
		}
	}

	tlInfo, err := m.parser.TaskListInfoFromBlob(row.Data, row.DataEncoding)
	if err != nil {
		return nil, err
	}

	var resp *persistence.LeaseTaskListResponse
	err = m.txExecute(ctx, dbShardID, "LeaseTaskList", func(tx sqlplugin.Tx) error {
		rangeID = row.RangeID
		ackLevel = tlInfo.GetAckLevel()
		// We need to separately check the condition and do the
		// update because we want to throw different error codes.
		// Since we need to do things separately (in a transaction), we need to take a lock.
		err1 := lockTaskList(ctx, tx, dbShardID, domainID, request.TaskList, request.TaskType, rangeID)
		if err1 != nil {
			return err1
		}
		now := time.Now()
		tlInfo.LastUpdated = now
		blob, err1 := m.parser.TaskListInfoToBlob(tlInfo)
		if err1 != nil {
			return err1
		}
		row := &sqlplugin.TaskListsRow{
			ShardID:      dbShardID,
			DomainID:     row.DomainID,
			RangeID:      row.RangeID + 1,
			Name:         row.Name,
			TaskType:     row.TaskType,
			Data:         blob.Data,
			DataEncoding: string(blob.Encoding),
		}
		var result sql.Result
		if tlInfo.GetKind() == persistence.TaskListKindSticky && m.db.SupportsTTL() {
			result, err1 = tx.UpdateTaskListsWithTTL(ctx, &sqlplugin.TaskListsRowWithTTL{
				TaskListsRow: *row,
				TTL:          stickyTasksListsTTL,
			})
		} else {
			result, err1 = tx.UpdateTaskLists(ctx, row)
		}
		if err1 != nil {
			return err1
		}
		rowsAffected, err1 := result.RowsAffected()
		if err1 != nil {
			return fmt.Errorf("rowsAffected error: %v", err1)
		}
		if rowsAffected == 0 {
			return fmt.Errorf("%v rows affected instead of 1", rowsAffected)
		}
		resp = &persistence.LeaseTaskListResponse{TaskListInfo: &persistence.TaskListInfo{
			DomainID:                request.DomainID,
			Name:                    request.TaskList,
			TaskType:                request.TaskType,
			RangeID:                 rangeID + 1,
			AckLevel:                ackLevel,
			Kind:                    request.TaskListKind,
			LastUpdated:             now,
			AdaptivePartitionConfig: fromSerializationTaskListPartitionConfig(tlInfo.AdaptivePartitionConfig),
		}}
		return nil
	})
	return resp, err
}

func (m *sqlTaskStore) GetTaskList(
	ctx context.Context,
	request *persistence.GetTaskListRequest,
) (*persistence.GetTaskListResponse, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskList, m.db.GetTotalNumDBShards())

	domainID := serialization.MustParseUUID(request.DomainID)
	rows, err := m.db.SelectFromTaskLists(ctx, &sqlplugin.TaskListsFilter{
		ShardID:  dbShardID,
		DomainID: &domainID,
		Name:     &request.TaskList,
		TaskType: common.Int64Ptr(int64(request.TaskType))})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetTaskList", "", err)
	}
	row := rows[0]
	tlInfo, err := m.parser.TaskListInfoFromBlob(row.Data, row.DataEncoding)
	if err != nil {
		return nil, err
	}
	return &persistence.GetTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID:                request.DomainID,
			Name:                    request.TaskList,
			TaskType:                request.TaskType,
			RangeID:                 row.RangeID,
			AckLevel:                tlInfo.AckLevel,
			Kind:                    int(tlInfo.Kind),
			Expiry:                  tlInfo.ExpiryTimestamp,
			LastUpdated:             tlInfo.LastUpdated,
			AdaptivePartitionConfig: fromSerializationTaskListPartitionConfig(tlInfo.AdaptivePartitionConfig),
		},
	}, nil
}

func (m *sqlTaskStore) UpdateTaskList(
	ctx context.Context,
	request *persistence.UpdateTaskListRequest,
) (*persistence.UpdateTaskListResponse, error) {
	dbShardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.TaskListInfo.DomainID, request.TaskListInfo.Name, m.db.GetTotalNumDBShards())
	domainID := serialization.MustParseUUID(request.TaskListInfo.DomainID)
	tlInfo := &serialization.TaskListInfo{
		AckLevel:                request.TaskListInfo.AckLevel,
		Kind:                    int16(request.TaskListInfo.Kind),
		ExpiryTimestamp:         time.Unix(0, 0),
		LastUpdated:             time.Now(),
		AdaptivePartitionConfig: toSerializationTaskListPartitionConfig(request.TaskListInfo.AdaptivePartitionConfig),
	}
	if request.TaskListInfo.Kind == persistence.TaskListKindSticky {
		tlInfo.ExpiryTimestamp = stickyTaskListExpiry()
	}

	var resp *persistence.UpdateTaskListResponse
	blob, err := m.parser.TaskListInfoToBlob(tlInfo)
	if err != nil {
		return nil, err
	}
	err = m.txExecute(ctx, dbShardID, "UpdateTaskList", func(tx sqlplugin.Tx) error {
		err1 := lockTaskList(
			ctx, tx, dbShardID, domainID, request.TaskListInfo.Name, request.TaskListInfo.TaskType, request.TaskListInfo.RangeID)
		if err1 != nil {
			return err1
		}
		var result sql.Result
		row := &sqlplugin.TaskListsRow{
			ShardID:      dbShardID,
			DomainID:     domainID,
			RangeID:      request.TaskListInfo.RangeID,
			Name:         request.TaskListInfo.Name,
			TaskType:     int64(request.TaskListInfo.TaskType),
			Data:         blob.Data,
			DataEncoding: string(blob.Encoding),
		}
		if m.db.SupportsTTL() && request.TaskListInfo.Kind == persistence.TaskListKindSticky {
			result, err1 = tx.UpdateTaskListsWithTTL(ctx, &sqlplugin.TaskListsRowWithTTL{
				TaskListsRow: *row,
				TTL:          stickyTasksListsTTL,
			})
		} else {
			result, err1 = tx.UpdateTaskLists(ctx, row)
		}
		if err1 != nil {
			return err1
		}
		rowsAffected, err1 := result.RowsAffected()
		if err1 != nil {
			return err1
		}
		if rowsAffected != 1 {
			return fmt.Errorf("%v rows were affected instead of 1", rowsAffected)
		}
		resp = &persistence.UpdateTaskListResponse{}
		return nil
	})
	return resp, err
}

type taskListPageToken struct {
	ShardID  int
	DomainID serialization.UUID
	Name     string
	TaskType int64
}

// ListTaskList lists tasklist from DB
// DomainID translates into byte array in SQL. The minUUID is not the minimum byte array.
func (m *sqlTaskStore) ListTaskList(
	ctx context.Context,
	request *persistence.ListTaskListRequest,
) (*persistence.ListTaskListResponse, error) {
	pageToken := taskListPageToken{DomainID: serialization.UUID{}}
	if len(request.PageToken) > 0 {
		if err := gobDeserialize(request.PageToken, &pageToken); err != nil {
			return nil, &types.InternalServiceError{Message: fmt.Sprintf("error deserializing page token: %v", err)}
		}
	} else {
		pageToken = taskListPageToken{TaskType: math.MinInt16, DomainID: serialization.UUID{}}
	}
	var err error
	var rows []sqlplugin.TaskListsRow
	for pageToken.ShardID < m.nShards {
		rows, err = m.db.SelectFromTaskLists(ctx, &sqlplugin.TaskListsFilter{
			ShardID:             pageToken.ShardID,
			DomainIDGreaterThan: &pageToken.DomainID,
			NameGreaterThan:     &pageToken.Name,
			TaskTypeGreaterThan: &pageToken.TaskType,
			PageSize:            &request.PageSize,
		})
		if err != nil {
			return nil, convertCommonErrors(m.db, "ListTaskList", "", err)
		}
		if len(rows) > 0 {
			break
		}
		pageToken = taskListPageToken{ShardID: pageToken.ShardID + 1, TaskType: math.MinInt16, DomainID: serialization.UUID{}}
	}

	var nextPageToken []byte
	switch {
	case len(rows) >= request.PageSize:
		lastRow := &rows[request.PageSize-1]
		nextPageToken, err = gobSerialize(&taskListPageToken{
			ShardID:  pageToken.ShardID,
			DomainID: lastRow.DomainID,
			Name:     lastRow.Name,
			TaskType: lastRow.TaskType,
		})
	case pageToken.ShardID+1 < m.nShards:
		nextPageToken, err = gobSerialize(&taskListPageToken{ShardID: pageToken.ShardID + 1, TaskType: math.MinInt16, DomainID: serialization.UUID{}})
	}

	if err != nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("error serializing nextPageToken:%v", err)}
	}

	resp := &persistence.ListTaskListResponse{
		Items:         make([]persistence.TaskListInfo, len(rows)),
		NextPageToken: nextPageToken,
	}

	for i := range rows {
		info, err := m.parser.TaskListInfoFromBlob(rows[i].Data, rows[i].DataEncoding)
		if err != nil {
			return nil, err
		}
		resp.Items[i].DomainID = rows[i].DomainID.String()
		resp.Items[i].Name = rows[i].Name
		resp.Items[i].TaskType = int(rows[i].TaskType)
		resp.Items[i].RangeID = rows[i].RangeID
		resp.Items[i].Kind = int(info.GetKind())
		resp.Items[i].AckLevel = info.GetAckLevel()
		resp.Items[i].Expiry = info.GetExpiryTimestamp()
		resp.Items[i].LastUpdated = info.GetLastUpdated()
	}

	return resp, nil
}

func (m *sqlTaskStore) DeleteTaskList(
	ctx context.Context,
	request *persistence.DeleteTaskListRequest,
) error {
	shardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskListName, m.db.GetTotalNumDBShards())
	domainID := serialization.MustParseUUID(request.DomainID)
	result, err := m.db.DeleteFromTaskLists(ctx, &sqlplugin.TaskListsFilter{
		ShardID:  shardID,
		DomainID: &domainID,
		Name:     &request.TaskListName,
		TaskType: common.Int64Ptr(int64(request.TaskListType)),
		RangeID:  &request.RangeID,
	})
	if err != nil {
		return convertCommonErrors(m.db, "DeleteTaskList", "", err)
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{Message: fmt.Sprintf("rowsAffected returned error:%v", err)}
	}
	if nRows != 1 {
		return &types.InternalServiceError{Message: fmt.Sprintf("delete failed: %v rows affected instead of 1", nRows)}
	}
	return nil
}

func (m *sqlTaskStore) CreateTasks(
	ctx context.Context,
	request *persistence.CreateTasksRequest,
) (*persistence.CreateTasksResponse, error) {
	var tasksRows []sqlplugin.TasksRow
	var tasksRowsWithTTL []sqlplugin.TasksRowWithTTL
	if m.db.SupportsTTL() {
		tasksRowsWithTTL = make([]sqlplugin.TasksRowWithTTL, len(request.Tasks))
	} else {
		tasksRows = make([]sqlplugin.TasksRow, len(request.Tasks))
	}

	dbShardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.TaskListInfo.DomainID, request.TaskListInfo.Name, m.db.GetTotalNumDBShards())

	for i, v := range request.Tasks {
		var expiryTime time.Time
		var ttl time.Duration
		if v.Data.ScheduleToStartTimeoutSeconds > 0 {
			ttl = time.Duration(v.Data.ScheduleToStartTimeoutSeconds) * time.Second
			if m.db.SupportsTTL() {
				maxAllowedTTL, err := m.db.MaxAllowedTTL()
				if err != nil {
					return nil, err
				}
				if ttl > *maxAllowedTTL {
					ttl = *maxAllowedTTL
				}
			}
			expiryTime = time.Now().Add(ttl)
		}
		blob, err := m.parser.TaskInfoToBlob(&serialization.TaskInfo{
			WorkflowID:       v.Data.WorkflowID,
			RunID:            serialization.MustParseUUID(v.Data.RunID),
			ScheduleID:       v.Data.ScheduleID,
			ExpiryTimestamp:  expiryTime,
			CreatedTimestamp: time.Now(),
			PartitionConfig:  v.Data.PartitionConfig,
		})
		if err != nil {
			return nil, err
		}

		currTasksRow := sqlplugin.TasksRow{
			ShardID:      dbShardID,
			DomainID:     serialization.MustParseUUID(v.Data.DomainID),
			TaskListName: request.TaskListInfo.Name,
			TaskType:     int64(request.TaskListInfo.TaskType),
			TaskID:       v.TaskID,
			Data:         blob.Data,
			DataEncoding: string(blob.Encoding),
		}
		if m.db.SupportsTTL() {
			currTasksRowWithTTL := sqlplugin.TasksRowWithTTL{
				TasksRow: currTasksRow,
			}
			if ttl > 0 {
				currTasksRowWithTTL.TTL = &ttl
			}
			tasksRowsWithTTL[i] = currTasksRowWithTTL
		} else {
			tasksRows[i] = currTasksRow
		}

	}
	var resp *persistence.CreateTasksResponse
	err := m.txExecute(ctx, dbShardID, "CreateTasks", func(tx sqlplugin.Tx) error {
		if m.db.SupportsTTL() {
			if _, err := tx.InsertIntoTasksWithTTL(ctx, tasksRowsWithTTL); err != nil {
				return err
			}
		} else {
			if _, err := tx.InsertIntoTasks(ctx, tasksRows); err != nil {
				return err
			}
		}

		// Lock task list before committing.
		err1 := lockTaskList(ctx, tx,
			dbShardID,
			serialization.MustParseUUID(request.TaskListInfo.DomainID),
			request.TaskListInfo.Name,
			request.TaskListInfo.TaskType, request.TaskListInfo.RangeID)
		if err1 != nil {
			return err1
		}
		resp = &persistence.CreateTasksResponse{}
		return nil
	})
	return resp, err
}

func (m *sqlTaskStore) GetTasks(
	ctx context.Context,
	request *persistence.GetTasksRequest,
) (*persistence.GetTasksResponse, error) {
	shardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskList, m.db.GetTotalNumDBShards())
	rows, err := m.db.SelectFromTasks(ctx, &sqlplugin.TasksFilter{
		ShardID:      shardID,
		DomainID:     serialization.MustParseUUID(request.DomainID),
		TaskListName: request.TaskList,
		TaskType:     int64(request.TaskType),
		MinTaskID:    &request.ReadLevel,
		MaxTaskID:    request.MaxReadLevel,
		PageSize:     &request.BatchSize,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetTasks", "", err)
	}

	var tasks = make([]*persistence.TaskInfo, len(rows))
	for i, v := range rows {
		info, err := m.parser.TaskInfoFromBlob(v.Data, v.DataEncoding)
		if err != nil {
			return nil, err
		}
		tasks[i] = &persistence.TaskInfo{
			DomainID:        request.DomainID,
			WorkflowID:      info.GetWorkflowID(),
			RunID:           info.RunID.String(),
			TaskID:          v.TaskID,
			ScheduleID:      info.GetScheduleID(),
			Expiry:          info.GetExpiryTimestamp(),
			CreatedTime:     info.GetCreatedTimestamp(),
			PartitionConfig: info.GetPartitionConfig(),
		}
	}

	return &persistence.GetTasksResponse{Tasks: tasks}, nil
}

func (m *sqlTaskStore) CompleteTask(
	ctx context.Context,
	request *persistence.CompleteTaskRequest,
) error {
	taskID := request.TaskID
	taskList := request.TaskList
	shardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(taskList.DomainID, taskList.Name, m.db.GetTotalNumDBShards())
	_, err := m.db.DeleteFromTasks(ctx, &sqlplugin.TasksFilter{
		ShardID:      shardID,
		DomainID:     serialization.MustParseUUID(taskList.DomainID),
		TaskListName: taskList.Name,
		TaskType:     int64(taskList.TaskType),
		TaskID:       &taskID})
	if err != nil {
		return convertCommonErrors(m.db, "CompleteTask", "", err)
	}
	return nil
}

func (m *sqlTaskStore) CompleteTasksLessThan(
	ctx context.Context,
	request *persistence.CompleteTasksLessThanRequest,
) (*persistence.CompleteTasksLessThanResponse, error) {
	shardID := sqlplugin.GetDBShardIDFromDomainIDAndTasklist(request.DomainID, request.TaskListName, m.db.GetTotalNumDBShards())
	result, err := m.db.DeleteFromTasks(ctx, &sqlplugin.TasksFilter{
		ShardID:              shardID,
		DomainID:             serialization.MustParseUUID(request.DomainID),
		TaskListName:         request.TaskListName,
		TaskType:             int64(request.TaskType),
		TaskIDLessThanEquals: &request.TaskID,
		Limit:                &request.Limit,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "CompleteTasksLessThan", "", err)
	}
	nRows, err := result.RowsAffected()
	if err != nil {
		return nil, &types.InternalServiceError{
			Message: fmt.Sprintf("rowsAffected returned error: %v", err),
		}
	}
	return &persistence.CompleteTasksLessThanResponse{TasksCompleted: int(nRows)}, nil
}

// GetOrphanTasks gets tasks from the tasks table that belong to a task_list no longer present
// in the task_lists table.
// TODO: Limit this query to a specific shard at a time. See https://github.com/uber/cadence/issues/4064
func (m *sqlTaskStore) GetOrphanTasks(ctx context.Context, request *persistence.GetOrphanTasksRequest) (*persistence.GetOrphanTasksResponse, error) {
	rows, err := m.db.GetOrphanTasks(ctx, &sqlplugin.OrphanTasksFilter{
		Limit: &request.Limit,
	})
	if err != nil {
		return nil, convertCommonErrors(m.db, "GetOrphanTasks", "", err)
	}

	var tasks = make([]*persistence.TaskKey, len(rows))
	for i, v := range rows {
		tasks[i] = &persistence.TaskKey{
			DomainID:     v.DomainID.String(),
			TaskListName: v.TaskListName,
			TaskType:     int(v.TaskType),
			TaskID:       v.TaskID,
		}
	}

	return &persistence.GetOrphanTasksResponse{Tasks: tasks}, nil
}

func lockTaskList(ctx context.Context, tx sqlplugin.Tx, shardID int, domainID serialization.UUID, name string, taskListType int, oldRangeID int64) error {
	rangeID, err := tx.LockTaskLists(ctx, &sqlplugin.TaskListsFilter{
		ShardID: shardID, DomainID: &domainID, Name: &name, TaskType: common.Int64Ptr(int64(taskListType))})

	switch err {
	case nil:
		if rangeID != oldRangeID {
			return &persistence.ConditionFailedError{
				Msg: fmt.Sprintf("Task list range ID was %v when it was should have been %v", rangeID, oldRangeID),
			}
		}
		return nil
	case sql.ErrNoRows:
		return &persistence.ConditionFailedError{
			Msg: "Task list does not exist.",
		}
	default:
		return convertCommonErrors(tx, "lockTaskList", "", err)
	}
}

func stickyTaskListExpiry() time.Time {
	return time.Now().Add(stickyTasksListsTTL)
}

func toSerializationTaskListPartitionConfig(c *persistence.TaskListPartitionConfig) *serialization.TaskListPartitionConfig {
	if c == nil {
		return nil
	}
	return &serialization.TaskListPartitionConfig{
		Version:            c.Version,
		NumReadPartitions:  int32(len(c.ReadPartitions)),
		NumWritePartitions: int32(len(c.WritePartitions)),
		ReadPartitions:     toSerializationTaskListPartitionMap(c.ReadPartitions),
		WritePartitions:    toSerializationTaskListPartitionMap(c.WritePartitions),
	}
}

func toSerializationTaskListPartitionMap(m map[int]*persistence.TaskListPartition) map[int32]*serialization.TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int32]*serialization.TaskListPartition, len(m))
	for id, p := range m {
		result[int32(id)] = &serialization.TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	return result
}

func fromSerializationTaskListPartitionConfig(c *serialization.TaskListPartitionConfig) *persistence.TaskListPartitionConfig {
	if c == nil {
		return nil
	}
	var read map[int]*persistence.TaskListPartition
	if int32(len(c.ReadPartitions)) == c.NumReadPartitions {
		read = fromSerializationTaskListPartitionMap(c.ReadPartitions)
	} else {
		read = createDefaultPartitions(c.NumReadPartitions)
	}
	var write map[int]*persistence.TaskListPartition
	if int32(len(c.WritePartitions)) == c.NumWritePartitions {
		write = fromSerializationTaskListPartitionMap(c.WritePartitions)
	} else {
		write = createDefaultPartitions(c.NumWritePartitions)
	}
	return &persistence.TaskListPartitionConfig{
		Version:         c.Version,
		ReadPartitions:  read,
		WritePartitions: write,
	}
}

func createDefaultPartitions(len int32) map[int]*persistence.TaskListPartition {
	partitions := make(map[int]*persistence.TaskListPartition, len)
	for i := 0; i < int(len); i++ {
		partitions[i] = &persistence.TaskListPartition{}
	}
	return partitions
}

func fromSerializationTaskListPartitionMap(m map[int32]*serialization.TaskListPartition) map[int]*persistence.TaskListPartition {
	if m == nil {
		return nil
	}
	result := make(map[int]*persistence.TaskListPartition, len(m))
	for id, p := range m {
		result[int(id)] = &persistence.TaskListPartition{IsolationGroups: p.IsolationGroups}
	}
	return result
}
