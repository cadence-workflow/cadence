// Copyright (c) 2017 Uber Technologies, Inc.
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
	"encoding/json"
	"fmt"
	"time"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/thrift"
)

type (
	sqlVisibilityStore struct {
		sqlStore
	}

	visibilityPageToken struct {
		Time  time.Time
		RunID string
	}
)

// NewSQLVisibilityStore creates an instance of ExecutionStore
func NewSQLVisibilityStore(cfg config.SQL, logger log.Logger) (p.VisibilityStore, error) {
	db, err := NewSQLDB(&cfg)
	if err != nil {
		return nil, err
	}
	return &sqlVisibilityStore{
		sqlStore: sqlStore{
			db:     db,
			logger: logger,
		},
	}, nil
}

func (s *sqlVisibilityStore) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *p.InternalRecordWorkflowExecutionStartedRequest,
) error {
	_, err := s.db.InsertIntoVisibility(ctx, &sqlplugin.VisibilityRow{
		DomainID:               request.DomainUUID,
		WorkflowID:             request.WorkflowID,
		RunID:                  request.RunID,
		StartTime:              request.StartTimestamp,
		ExecutionTime:          request.ExecutionTimestamp,
		WorkflowTypeName:       request.WorkflowTypeName,
		Memo:                   request.Memo.Data,
		Encoding:               string(request.Memo.GetEncoding()),
		IsCron:                 request.IsCron,
		CronSchedule:           request.CronSchedule,
		NumClusters:            request.NumClusters,
		UpdateTime:             request.UpdateTimestamp,
		ShardID:                request.ShardID,
		ExecutionStatus:        int32(request.ExecutionStatus),
		ScheduledExecutionTime: request.ScheduledExecutionTime,
	})

	if err != nil {
		return convertCommonErrors(s.db, "RecordWorkflowExecutionStarted", "", err)
	}
	return nil
}

func (s *sqlVisibilityStore) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *p.InternalRecordWorkflowExecutionClosedRequest,
) error {
	closeTime := request.CloseTimestamp

	// Map CloseStatus to ExecutionStatus
	executionStatus := types.WorkflowExecutionStatusCompleted // default
	switch request.Status {
	case types.WorkflowExecutionCloseStatusCompleted:
		executionStatus = types.WorkflowExecutionStatusCompleted
	case types.WorkflowExecutionCloseStatusFailed:
		executionStatus = types.WorkflowExecutionStatusFailed
	case types.WorkflowExecutionCloseStatusCanceled:
		executionStatus = types.WorkflowExecutionStatusCanceled
	case types.WorkflowExecutionCloseStatusTerminated:
		executionStatus = types.WorkflowExecutionStatusTerminated
	case types.WorkflowExecutionCloseStatusContinuedAsNew:
		executionStatus = types.WorkflowExecutionStatusContinuedAsNew
	case types.WorkflowExecutionCloseStatusTimedOut:
		executionStatus = types.WorkflowExecutionStatusTimedOut
	}

	result, err := s.db.ReplaceIntoVisibility(ctx, &sqlplugin.VisibilityRow{
		DomainID:               request.DomainUUID,
		WorkflowID:             request.WorkflowID,
		RunID:                  request.RunID,
		StartTime:              request.StartTimestamp,
		ExecutionTime:          request.ExecutionTimestamp,
		WorkflowTypeName:       request.WorkflowTypeName,
		CloseTime:              &closeTime,
		CloseStatus:            common.Int32Ptr(int32(*thrift.FromWorkflowExecutionCloseStatus(&request.Status))),
		HistoryLength:          &request.HistoryLength,
		Memo:                   request.Memo.Data,
		Encoding:               string(request.Memo.GetEncoding()),
		IsCron:                 request.IsCron,
		CronSchedule:           request.CronSchedule,
		NumClusters:            request.NumClusters,
		UpdateTime:             request.UpdateTimestamp,
		ShardID:                request.ShardID,
		ExecutionStatus:        int32(executionStatus),
		ScheduledExecutionTime: request.ScheduledExecutionTime,
	})
	if err != nil {
		return convertCommonErrors(s.db, "RecordWorkflowExecutionClosed", "", err)
	}
	noRowsAffected, err := result.RowsAffected()
	if err != nil {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("RecordWorkflowExecutionClosed rowsAffected error: %v", err),
		}
	}
	if noRowsAffected > 2 { // either adds a new row or deletes old row and adds new row
		return &types.InternalServiceError{
			Message: fmt.Sprintf("RecordWorkflowExecutionClosed unexpected numRows (%v) updated", noRowsAffected),
		}
	}
	return nil
}

func (s *sqlVisibilityStore) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *p.InternalRecordWorkflowExecutionUninitializedRequest,
) error {
	// temporary: not implemented, only implemented for ES
	return nil
}

func (s *sqlVisibilityStore) UpsertWorkflowExecution(
	_ context.Context,
	request *p.InternalUpsertWorkflowExecutionRequest,
) error {
	if p.IsNopUpsertWorkflowRequest(request) {
		return nil
	}
	return p.ErrVisibilityOperationNotSupported
}

func (s *sqlVisibilityStore) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListOpenWorkflowExecutions", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:     request.DomainUUID,
				MinStartTime: &request.EarliestTime,
				MaxStartTime: &readLevel.Time,
				RunID:        &readLevel.RunID,
				PageSize:     &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListClosedWorkflowExecutions", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:     request.DomainUUID,
				MinStartTime: &request.EarliestTime,
				MaxStartTime: &readLevel.Time,
				Closed:       true,
				RunID:        &readLevel.RunID,
				PageSize:     &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsByTypeRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListOpenWorkflowExecutionsByType", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:         request.DomainUUID,
				MinStartTime:     &request.EarliestTime,
				MaxStartTime:     &readLevel.Time,
				RunID:            &readLevel.RunID,
				WorkflowTypeName: &request.WorkflowTypeName,
				PageSize:         &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsByTypeRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListClosedWorkflowExecutionsByType", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:         request.DomainUUID,
				MinStartTime:     &request.EarliestTime,
				MaxStartTime:     &readLevel.Time,
				Closed:           true,
				RunID:            &readLevel.RunID,
				WorkflowTypeName: &request.WorkflowTypeName,
				PageSize:         &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsByWorkflowIDRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListOpenWorkflowExecutionsByWorkflowID", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:     request.DomainUUID,
				MinStartTime: &request.EarliestTime,
				MaxStartTime: &readLevel.Time,
				RunID:        &readLevel.RunID,
				WorkflowID:   &request.WorkflowID,
				PageSize:     &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.InternalListWorkflowExecutionsByWorkflowIDRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListClosedWorkflowExecutionsByWorkflowID", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:     request.DomainUUID,
				MinStartTime: &request.EarliestTime,
				MaxStartTime: &readLevel.Time,
				Closed:       true,
				RunID:        &readLevel.RunID,
				WorkflowID:   &request.WorkflowID,
				PageSize:     &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *p.InternalListClosedWorkflowExecutionsByStatusRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return s.listWorkflowExecutions("ListClosedWorkflowExecutionsByStatus", request.NextPageToken, request.EarliestTime, request.LatestTime,
		func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error) {
			return s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
				DomainID:     request.DomainUUID,
				MinStartTime: &request.EarliestTime,
				MaxStartTime: &readLevel.Time,
				Closed:       true,
				RunID:        &readLevel.RunID,
				CloseStatus:  common.Int32Ptr(int32(*thrift.FromWorkflowExecutionCloseStatus(&request.Status))),
				PageSize:     &request.PageSize,
			})
		})
}

func (s *sqlVisibilityStore) GetClosedWorkflowExecution(
	ctx context.Context,
	request *p.InternalGetClosedWorkflowExecutionRequest,
) (*p.InternalGetClosedWorkflowExecutionResponse, error) {
	execution := request.Execution
	rows, err := s.db.SelectFromVisibility(ctx, &sqlplugin.VisibilityFilter{
		DomainID: request.DomainUUID,
		Closed:   true,
		RunID:    &execution.RunID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &types.EntityNotExistsError{
				Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v, RunId: %v",
					execution.GetWorkflowID(), execution.GetRunID()),
			}
		}
		return nil, convertCommonErrors(s.db, "GetClosedWorkflowExecution", "", err)
	}
	rows[0].DomainID = request.DomainUUID
	rows[0].RunID = execution.GetRunID()
	rows[0].WorkflowID = execution.GetWorkflowID()
	return &p.InternalGetClosedWorkflowExecutionResponse{Execution: s.rowToInfo(&rows[0])}, nil
}

func (s *sqlVisibilityStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {
	_, err := s.db.DeleteFromVisibility(ctx, &sqlplugin.VisibilityFilter{
		DomainID: request.DomainID,
		RunID:    &request.RunID,
	})
	if err != nil {
		return convertCommonErrors(s.db, "DeleteWorkflowExecution", "", err)
	}
	return nil
}

func (s *sqlVisibilityStore) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {
	// temporary: not implemented, only implemented for ES
	return nil
}

func (s *sqlVisibilityStore) ListWorkflowExecutions(
	_ context.Context,
	_ *p.ListWorkflowExecutionsByQueryRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return nil, p.ErrVisibilityOperationNotSupported
}

func (s *sqlVisibilityStore) ScanWorkflowExecutions(
	_ context.Context,
	_ *p.ListWorkflowExecutionsByQueryRequest,
) (*p.InternalListWorkflowExecutionsResponse, error) {
	return nil, p.ErrVisibilityOperationNotSupported
}

func (s *sqlVisibilityStore) CountWorkflowExecutions(
	_ context.Context,
	_ *p.CountWorkflowExecutionsRequest,
) (*p.CountWorkflowExecutionsResponse, error) {
	return nil, p.ErrVisibilityOperationNotSupported
}

func (s *sqlVisibilityStore) rowToInfo(row *sqlplugin.VisibilityRow) *p.InternalVisibilityWorkflowExecutionInfo {
	if row.ExecutionTime.UnixNano() == 0 {
		row.ExecutionTime = row.StartTime
	}
	info := &p.InternalVisibilityWorkflowExecutionInfo{
		WorkflowID:             row.WorkflowID,
		RunID:                  row.RunID,
		TypeName:               row.WorkflowTypeName,
		StartTime:              row.StartTime,
		ExecutionTime:          row.ExecutionTime,
		IsCron:                 row.IsCron,
		CronSchedule:           row.CronSchedule,
		NumClusters:            row.NumClusters,
		Memo:                   p.NewDataBlob(row.Memo, constants.EncodingType(row.Encoding)),
		UpdateTime:             row.UpdateTime,
		ShardID:                row.ShardID,
		ExecutionStatus:        types.WorkflowExecutionStatus(row.ExecutionStatus),
		ScheduledExecutionTime: row.ScheduledExecutionTime,
	}
	if row.CloseStatus != nil {
		status := workflow.WorkflowExecutionCloseStatus(*row.CloseStatus)
		info.Status = thrift.ToWorkflowExecutionCloseStatus(&status)
		info.CloseTime = *row.CloseTime
		info.HistoryLength = *row.HistoryLength
	}
	return info
}

func (s *sqlVisibilityStore) listWorkflowExecutions(opName string, pageToken []byte, earliestTime time.Time, latestTime time.Time, selectOp func(readLevel *visibilityPageToken) ([]sqlplugin.VisibilityRow, error)) (*p.InternalListWorkflowExecutionsResponse, error) {
	var readLevel *visibilityPageToken
	var err error
	if len(pageToken) > 0 {
		readLevel, err = s.deserializePageToken(pageToken)
		if err != nil {
			return nil, err
		}
	} else {
		readLevel = &visibilityPageToken{Time: latestTime, RunID: ""}
	}
	rows, err := selectOp(readLevel)
	if err != nil {
		return nil, convertCommonErrors(s.db, opName, "", err)
	}
	if len(rows) == 0 {
		return &p.InternalListWorkflowExecutionsResponse{}, nil
	}

	var infos = make([]*p.InternalVisibilityWorkflowExecutionInfo, len(rows))
	for i, row := range rows {
		infos[i] = s.rowToInfo(&row)
	}
	var nextPageToken []byte
	lastRow := rows[len(rows)-1]
	lastStartTime := lastRow.StartTime
	if lastStartTime.Sub(earliestTime).Nanoseconds() > 0 {
		nextPageToken, err = s.serializePageToken(&visibilityPageToken{
			Time:  lastStartTime,
			RunID: lastRow.RunID,
		})
		if err != nil {
			return nil, err
		}
	}
	return &p.InternalListWorkflowExecutionsResponse{
		Executions:    infos,
		NextPageToken: nextPageToken,
	}, nil
}

func (s *sqlVisibilityStore) deserializePageToken(data []byte) (*visibilityPageToken, error) {
	var token visibilityPageToken
	err := json.Unmarshal(data, &token)
	return &token, err
}

func (s *sqlVisibilityStore) serializePageToken(token *visibilityPageToken) ([]byte, error) {
	data, err := json.Marshal(token)
	return data, err
}
