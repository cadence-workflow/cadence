// Copyright (c) 2020 Uber Technologies, Inc.
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
	"time"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

const (
	domainPartition = 0
)

// InsertVisibility creates a new visibility record, return error is there is any.
// TODO: Cassandra implementation ignores search attributes
func (db *CDB) InsertVisibility(ctx context.Context, ttlSeconds int64, row *nosqlplugin.VisibilityRowForInsert) error {
	var query gocql.Query
	if ttlSeconds > maxCassandraTTL {
		query = db.session.Query(templateCreateWorkflowExecutionStarted,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			row.TypeName,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
		).WithContext(ctx)
	} else {
		query = db.session.Query(templateCreateWorkflowExecutionStartedWithTTL,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			row.TypeName,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
			ttlSeconds,
		).WithContext(ctx)
	}
	query = query.WithTimestamp(persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()))
	return query.Exec()
}

func (db *CDB) UpdateVisibility(ctx context.Context, ttlSeconds int64, row *nosqlplugin.VisibilityRowForUpdate) error {
	batch := db.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	if row.UpdateCloseToOpen {
		// TODO implement it when where is a need
		panic("not supported operation")
	}

	if row.UpdateOpenToClose {
		// First, remove execution from the open table
		batch.Query(templateDeleteWorkflowExecutionStarted,
			row.DomainID,
			domainPartition,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			row.RunID,
		)
	}

	// Next, add a row in the closed table.
	if ttlSeconds > maxCassandraTTL {
		batch.Query(templateCreateWorkflowExecutionClosed,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.CloseTime.UnixNano()),
			row.TypeName,
			row.Status,
			row.HistoryLength,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
		)
		// duplicate write to v2 to order by close time
		batch.Query(templateCreateWorkflowExecutionClosedV2,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.CloseTime.UnixNano()),
			row.TypeName,
			row.Status,
			row.HistoryLength,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
		)
	} else {
		batch.Query(templateCreateWorkflowExecutionClosedWithTTL,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.CloseTime.UnixNano()),
			row.TypeName,
			row.Status,
			row.HistoryLength,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
			ttlSeconds,
		)
		// duplicate write to v2 to order by close time
		batch.Query(templateCreateWorkflowExecutionClosedWithTTLV2,
			row.DomainID,
			domainPartition,
			row.WorkflowID,
			row.RunID,
			persistence.UnixNanoToDBTimestamp(row.StartTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.ExecutionTime.UnixNano()),
			persistence.UnixNanoToDBTimestamp(row.CloseTime.UnixNano()),
			row.TypeName,
			row.Status,
			row.HistoryLength,
			row.Memo.Data,
			row.Memo.GetEncoding(),
			row.TaskList,
			row.IsCron,
			row.NumClusters,
			row.UpdateTime,
			row.ShardID,
			row.ExecutionStatus,
			row.CronSchedule,
			persistence.UnixNanoToDBTimestamp(row.ScheduledExecutionTime.UnixNano()),
			ttlSeconds,
		)
	}

	// RecordWorkflowExecutionStarted is using StartTimestamp as
	// the timestamp to issue query to Cassandra
	// due to the fact that cross DC using mutable state creation time as workflow start time
	// and visibility using event time instead of last update time (#1501)
	// CloseTimestamp can be before StartTimestamp, meaning using CloseTimestamp
	// can cause the deletion of open visibility record to be ignored.
	queryTimeStamp := row.CloseTime
	if queryTimeStamp.Before(row.StartTime) {
		queryTimeStamp = row.StartTime.Add(time.Second)
	}
	batch = batch.WithTimestamp(persistence.UnixNanoToDBTimestamp(queryTimeStamp.UnixNano()))
	return db.session.ExecuteBatch(batch)
}

func (db *CDB) SelectOneClosedWorkflow(
	ctx context.Context,
	domainID, workflowID, runID string,
) (*nosqlplugin.VisibilityRow, error) {
	query := db.session.Query(templateGetClosedWorkflowExecution,
		domainID,
		domainPartition,
		workflowID,
		runID,
	).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, fmt.Errorf("not able to create query iterator")
	}

	wfexecution, has := readClosedWorkflowExecutionRecord(iter)
	if !has {
		// Special case: return nil,nil if not found(since we will deprecate it, it's not worth refactor to be consistent)
		return nil, nil
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return wfexecution, nil
}

// Noop for Cassandra as it already handle by TTL
func (db *CDB) DeleteVisibility(ctx context.Context, domainID, workflowID, runID string) error {
	// Normally we only depend on TTL for Cassandra visibility deletion but
	// we explicitly delete from open executions when an admin command is issued
	key := persistence.VisibilityAdminDeletionKey("visibilityAdminDelete")
	if v := ctx.Value(key); v != nil && v.(bool) {
		// Primary key is <domainId, domainPartition, startTime, runId>
		// to optimize showing open executions sorted by their start time
		// However, it is not useful for deletion since we don't have the start
		// time information. So, we need to get the record first with "allow
		// filtering", read startTime then delete it. This is okay because
		// it is only intended to be used for admin ops.

		record, err := db.openWorkflowByRunID(ctx, domainID, runID)
		if err != nil {
			return err
		}
		if record == nil {
			return nil // workflow not found, nothing to do
		}

		query := db.session.Query(templateDeleteVisibilityRecord,
			domainID,
			domainPartition,
			record.StartTime,
			runID,
		).WithContext(ctx)
		return db.executeWithConsistencyAll(query)
	}
	return nil
}

func (db *CDB) SelectVisibility(ctx context.Context, filter *nosqlplugin.VisibilityFilter) (*nosqlplugin.SelectVisibilityResponse, error) {
	switch filter.FilterType {
	case nosqlplugin.AllOpen:
		return db.openSortedByStartTime(ctx, &filter.ListRequest)
	case nosqlplugin.AllClosed:
		switch filter.SortType {
		case nosqlplugin.SortByStartTime:
			return db.closedSortedByStartTime(ctx, &filter.ListRequest)
		case nosqlplugin.SortByClosedTime:
			return db.closedSortedByClosedTime(ctx, &filter.ListRequest)
		default:
			panic("not supported sorting type")
		}

	// by workflowType
	case nosqlplugin.OpenByWorkflowType:
		return db.openFilteredByWorkflowTypeSortedByStartTime(ctx, &filter.ListRequest, filter.WorkflowType)
	case nosqlplugin.ClosedByWorkflowType:
		switch filter.SortType {
		case nosqlplugin.SortByStartTime:
			return db.closedFilteredByWorkflowTypeSortedByStartTime(ctx, &filter.ListRequest, filter.WorkflowType)
		case nosqlplugin.SortByClosedTime:
			return db.closedFilteredByWorkflowTypeSortedByClosedTime(ctx, &filter.ListRequest, filter.WorkflowType)
		default:
			panic("not supported sorting type")
		}

	// by workflowID
	case nosqlplugin.OpenByWorkflowID:
		return db.openFilteredByWorkflowIDSortedByStartTime(ctx, &filter.ListRequest, filter.WorkflowID)
	case nosqlplugin.ClosedByWorkflowID:
		switch filter.SortType {
		case nosqlplugin.SortByStartTime:
			return db.closedFilteredByWorkflowIDSortedByStartTime(ctx, &filter.ListRequest, filter.WorkflowID)
		case nosqlplugin.SortByClosedTime:
			return db.closedFilteredByWorkflowIDSortedByClosedTime(ctx, &filter.ListRequest, filter.WorkflowID)
		default:
			panic("not supported sorting type")
		}

	// closeStatus
	case nosqlplugin.ClosedByClosedStatus:
		switch filter.SortType {
		case nosqlplugin.SortByStartTime:
			return db.closedFilteredByClosedStatusSortedByStartTime(ctx, &filter.ListRequest, filter.CloseStatus)
		case nosqlplugin.SortByClosedTime:
			return db.closedFilteredByClosedStatusSortedByClosedTime(ctx, &filter.ListRequest, filter.CloseStatus)
		default:
			panic("not supported sorting type")
		}
	default:
		panic("no supported filter type")
	}
}

func (db *CDB) openFilteredByWorkflowTypeSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowType string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetOpenWorkflowExecutionsByType,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowType,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readOpenWorkflowExecutionRecord)
}

func (db *CDB) closedFilteredByWorkflowTypeSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowType string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByType,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowType,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) closedFilteredByWorkflowTypeSortedByClosedTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowType string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByTypeSortByCloseTime,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowType,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) openFilteredByWorkflowIDSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowID string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetOpenWorkflowExecutionsByID,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowID,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readOpenWorkflowExecutionRecord)
}

func (db *CDB) openWorkflowByRunID(
	ctx context.Context,
	domainID string,
	runID string,
) (*persistence.InternalVisibilityWorkflowExecutionInfo, error) {
	query := db.session.Query(templateGetOpenWorkflowExecutionsByRunID,
		domainID,
		domainPartition,
		runID,
	).WithContext(ctx)

	iter := query.Iter()
	if iter == nil {
		return nil, fmt.Errorf("not able to create query iterator")
	}

	wfexecution, has := readOpenWorkflowExecutionRecord(iter)
	if !has {
		return nil, nil
	}

	if err := iter.Close(); err != nil {
		return nil, err
	}

	return wfexecution, nil
}

func (db *CDB) closedFilteredByWorkflowIDSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowID string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByID,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowID,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) closedFilteredByWorkflowIDSortedByClosedTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	workflowID string,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByIDSortByCloseTime,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		workflowID,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) closedFilteredByClosedStatusSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	closeStatus int32,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByStatus,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		closeStatus,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) closedFilteredByClosedStatusSortedByClosedTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
	closeStatus int32,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsByStatusSortByClosedTime,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
		closeStatus,
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) openSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetOpenWorkflowExecutions,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
	).Consistency(cassandraLowConslevel).WithContext(ctx)

	return processQuery(query, request, readOpenWorkflowExecutionRecord)
}

func (db *CDB) closedSortedByStartTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutions,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

func (db *CDB) closedSortedByClosedTime(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	query := db.session.Query(templateGetClosedWorkflowExecutionsSortByCloseTime,
		request.DomainUUID,
		domainPartition,
		persistence.UnixNanoToDBTimestamp(request.EarliestTime.UnixNano()),
		persistence.UnixNanoToDBTimestamp(request.LatestTime.UnixNano()),
	).Consistency(cassandraLowConslevel).WithContext(ctx)
	return processQuery(query, request, readClosedWorkflowExecutionRecord)
}

type recorderReaderFunc func(iter gocql.Iter) (*persistence.InternalVisibilityWorkflowExecutionInfo, bool)

func processQuery(
	query gocql.Query,
	request *persistence.InternalListWorkflowExecutionsRequest,
	recorderReader recorderReaderFunc,
) (*nosqlplugin.SelectVisibilityResponse, error) {
	iter := query.PageSize(request.PageSize).PageState(request.NextPageToken).Iter()
	if iter == nil {
		// TODO: may return badRequestError
		return nil, fmt.Errorf("not able to create query iterator")
	}

	response := &nosqlplugin.SelectVisibilityResponse{}
	response.Executions = make([]*persistence.InternalVisibilityWorkflowExecutionInfo, 0)
	wfexecution, has := recorderReader(iter)
	for has {
		response.Executions = append(response.Executions, wfexecution)
		wfexecution, has = recorderReader(iter)
	}

	nextPageToken := iter.PageState()
	response.NextPageToken = make([]byte, len(nextPageToken))
	copy(response.NextPageToken, nextPageToken)
	if err := iter.Close(); err != nil {
		return nil, err
	}

	return response, nil
}

func readOpenWorkflowExecutionRecord(
	iter gocql.Iter,
) (*persistence.InternalVisibilityWorkflowExecutionInfo, bool) {
	var workflowID string
	var runID string
	var typeName string
	var startTime time.Time
	var executionTime time.Time
	var memo []byte
	var encoding string
	var taskList string
	var isCron bool
	var numClusters int16
	var updateTime time.Time
	var shardID int16
	var executionStatus int32
	var cronSchedule string
	var scheduledExecutionTime time.Time
	if iter.Scan(&workflowID, &runID, &startTime, &executionTime, &typeName, &memo, &encoding, &taskList, &isCron, &numClusters, &updateTime, &shardID, &executionStatus, &cronSchedule, &scheduledExecutionTime) {
		record := &persistence.InternalVisibilityWorkflowExecutionInfo{
			WorkflowID:             workflowID,
			RunID:                  runID,
			TypeName:               typeName,
			StartTime:              startTime,
			ExecutionTime:          executionTime,
			Memo:                   persistence.NewDataBlob(memo, constants.EncodingType(encoding)),
			TaskList:               taskList,
			IsCron:                 isCron,
			NumClusters:            numClusters,
			UpdateTime:             updateTime,
			ShardID:                shardID,
			ExecutionStatus:        types.WorkflowExecutionStatus(executionStatus),
			CronSchedule:           cronSchedule,
			ScheduledExecutionTime: scheduledExecutionTime,
		}
		return record, true
	}
	return nil, false
}

func readClosedWorkflowExecutionRecord(
	iter gocql.Iter,
) (*persistence.InternalVisibilityWorkflowExecutionInfo, bool) {
	var workflowID string
	var runID string
	var typeName string
	var startTime time.Time
	var executionTime time.Time
	var closeTime time.Time
	var status workflow.WorkflowExecutionCloseStatus
	var historyLength int64
	var memo []byte
	var encoding string
	var taskList string
	var isCron bool
	var numClusters int16
	var updateTime time.Time
	var shardID int16
	var executionStatus int32
	var cronSchedule string
	var scheduledExecutionTime time.Time
	if iter.Scan(&workflowID, &runID, &startTime, &executionTime, &closeTime, &typeName, &status, &historyLength, &memo, &encoding, &taskList, &isCron, &numClusters, &updateTime, &shardID, &executionStatus, &cronSchedule, &scheduledExecutionTime) {
		record := &persistence.InternalVisibilityWorkflowExecutionInfo{
			WorkflowID:             workflowID,
			RunID:                  runID,
			TypeName:               typeName,
			StartTime:              startTime,
			ExecutionTime:          executionTime,
			CloseTime:              closeTime,
			Status:                 thrift.ToWorkflowExecutionCloseStatus(&status),
			HistoryLength:          historyLength,
			Memo:                   persistence.NewDataBlob(memo, constants.EncodingType(encoding)),
			TaskList:               taskList,
			IsCron:                 isCron,
			NumClusters:            numClusters,
			UpdateTime:             updateTime,
			ShardID:                shardID,
			ExecutionStatus:        types.WorkflowExecutionStatus(executionStatus),
			CronSchedule:           cronSchedule,
			ScheduledExecutionTime: scheduledExecutionTime,
		}
		return record, true
	}
	return nil, false
}
