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

package nosql

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

// Fixed domain values for now
const (
	defaultCloseTTLSeconds = 86400
	openExecutionTTLBuffer = int64(86400) // setting it to a day to account for shard going down
)

type nosqlVisibilityStore struct {
	sortByCloseTime bool
	nosqlStore
}

// newNoSQLVisibilityStore is used to create an instance of VisibilityStore implementation
func newNoSQLVisibilityStore(
	listClosedOrderingByCloseTime bool,
	cfg config.ShardedNoSQL,
	logger log.Logger,
	metricsClient metrics.Client,
	dc *persistence.DynamicConfiguration,
) (persistence.VisibilityStore, error) {
	shardedStore, err := newShardedNosqlStore(cfg, logger, metricsClient, dc)
	if err != nil {
		return nil, err
	}
	return &nosqlVisibilityStore{
		sortByCloseTime: listClosedOrderingByCloseTime,
		nosqlStore:      shardedStore.GetDefaultShard(),
	}, nil
}

func (v *nosqlVisibilityStore) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *persistence.InternalRecordWorkflowExecutionStartedRequest,
) error {
	ttl := int64(request.WorkflowTimeout.Seconds()) + openExecutionTTLBuffer

	err := v.db.InsertVisibility(ctx, ttl, &nosqlplugin.VisibilityRowForInsert{
		DomainID: request.DomainUUID,
		VisibilityRow: nosqlplugin.VisibilityRow{
			WorkflowID:             request.WorkflowID,
			RunID:                  request.RunID,
			TypeName:               request.WorkflowTypeName,
			StartTime:              request.StartTimestamp,
			ExecutionTime:          request.ExecutionTimestamp,
			Memo:                   request.Memo,
			TaskList:               request.TaskList,
			IsCron:                 request.IsCron,
			NumClusters:            request.NumClusters,
			UpdateTime:             request.UpdateTimestamp,
			ShardID:                request.ShardID,
			ExecutionStatus:        request.ExecutionStatus,
			CronSchedule:           request.CronSchedule,
			ScheduledExecutionTime: request.ScheduledExecutionTime,
		},
	})
	if err != nil {
		return convertCommonErrors(v.db, "RecordWorkflowExecutionStarted", err)
	}

	return nil
}

func (v *nosqlVisibilityStore) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *persistence.InternalRecordWorkflowExecutionClosedRequest,
) error {
	// Find how long to keep the row
	retention := request.RetentionPeriod
	if retention == 0 {
		retention = defaultCloseTTLSeconds * time.Second
	}

	err := v.db.UpdateVisibility(ctx, int64(retention.Seconds()), &nosqlplugin.VisibilityRowForUpdate{
		DomainID:          request.DomainUUID,
		UpdateOpenToClose: true,
		VisibilityRow: nosqlplugin.VisibilityRow{
			WorkflowID:    request.WorkflowID,
			RunID:         request.RunID,
			TypeName:      request.WorkflowTypeName,
			StartTime:     request.StartTimestamp,
			ExecutionTime: request.ExecutionTimestamp,
			Memo:          request.Memo,
			TaskList:      request.TaskList,
			IsCron:        request.IsCron,
			NumClusters:   request.NumClusters,
			// closed workflow attributes
			Status:                 &request.Status,
			CloseTime:              request.CloseTimestamp,
			HistoryLength:          request.HistoryLength,
			UpdateTime:             request.UpdateTimestamp,
			CronSchedule:           request.CronSchedule,
			ExecutionStatus:        request.ExecutionStatus,
			ScheduledExecutionTime: request.ScheduledExecutionTime,
		},
	})

	if err != nil {
		return convertCommonErrors(v.db, "RecordWorkflowExecutionClosed", err)
	}
	return nil
}

func (v *nosqlVisibilityStore) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *persistence.InternalRecordWorkflowExecutionUninitializedRequest,
) error {
	// temporary: not implemented, only implemented for ES
	return nil
}

func (v *nosqlVisibilityStore) UpsertWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalUpsertWorkflowExecutionRequest,
) error {
	if persistence.IsNopUpsertWorkflowRequest(request) {
		return nil
	}
	return persistence.ErrVisibilityOperationNotSupported
}

func (v *nosqlVisibilityStore) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	resp, err := v.db.SelectVisibility(ctx, &nosqlplugin.VisibilityFilter{
		ListRequest: *request,
		FilterType:  nosqlplugin.AllOpen,
		SortType:    nosqlplugin.SortByStartTime,
	})
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListOpenWorkflowExecutions", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	var filter *nosqlplugin.VisibilityFilter
	if v.sortByCloseTime {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: *request,
			FilterType:  nosqlplugin.AllClosed,
			SortType:    nosqlplugin.SortByClosedTime,
		}
	} else {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: *request,
			FilterType:  nosqlplugin.AllClosed,
			SortType:    nosqlplugin.SortByStartTime,
		}
	}
	resp, err := v.db.SelectVisibility(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListClosedWorkflowExecutions", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsByTypeRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	resp, err := v.db.SelectVisibility(ctx, &nosqlplugin.VisibilityFilter{
		ListRequest:  request.InternalListWorkflowExecutionsRequest,
		FilterType:   nosqlplugin.OpenByWorkflowType,
		SortType:     nosqlplugin.SortByStartTime,
		WorkflowType: request.WorkflowTypeName,
	})
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListOpenWorkflowExecutionsByType", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsByTypeRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	var filter *nosqlplugin.VisibilityFilter
	if v.sortByCloseTime {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest:  request.InternalListWorkflowExecutionsRequest,
			FilterType:   nosqlplugin.ClosedByWorkflowType,
			SortType:     nosqlplugin.SortByClosedTime,
			WorkflowType: request.WorkflowTypeName,
		}
	} else {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest:  request.InternalListWorkflowExecutionsRequest,
			FilterType:   nosqlplugin.ClosedByWorkflowType,
			SortType:     nosqlplugin.SortByStartTime,
			WorkflowType: request.WorkflowTypeName,
		}
	}
	resp, err := v.db.SelectVisibility(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListClosedWorkflowExecutionsByType", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsByWorkflowIDRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	resp, err := v.db.SelectVisibility(ctx, &nosqlplugin.VisibilityFilter{
		ListRequest: request.InternalListWorkflowExecutionsRequest,
		FilterType:  nosqlplugin.OpenByWorkflowID,
		SortType:    nosqlplugin.SortByStartTime,
		WorkflowID:  request.WorkflowID,
	})
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListOpenWorkflowExecutionsByWorkflowID", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *persistence.InternalListWorkflowExecutionsByWorkflowIDRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	var filter *nosqlplugin.VisibilityFilter
	if v.sortByCloseTime {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: request.InternalListWorkflowExecutionsRequest,
			FilterType:  nosqlplugin.ClosedByWorkflowID,
			SortType:    nosqlplugin.SortByClosedTime,
			WorkflowID:  request.WorkflowID,
		}
	} else {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: request.InternalListWorkflowExecutionsRequest,
			FilterType:  nosqlplugin.ClosedByWorkflowID,
			SortType:    nosqlplugin.SortByStartTime,
			WorkflowID:  request.WorkflowID,
		}
	}
	resp, err := v.db.SelectVisibility(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListClosedWorkflowExecutionsByWorkflowID", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *persistence.InternalListClosedWorkflowExecutionsByStatusRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	var filter *nosqlplugin.VisibilityFilter
	if v.sortByCloseTime {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: request.InternalListWorkflowExecutionsRequest,
			FilterType:  nosqlplugin.ClosedByClosedStatus,
			SortType:    nosqlplugin.SortByClosedTime,
			CloseStatus: int32(request.Status),
		}
	} else {
		filter = &nosqlplugin.VisibilityFilter{
			ListRequest: request.InternalListWorkflowExecutionsRequest,
			FilterType:  nosqlplugin.ClosedByClosedStatus,
			SortType:    nosqlplugin.SortByStartTime,
			CloseStatus: int32(request.Status),
		}
	}
	resp, err := v.db.SelectVisibility(ctx, filter)
	if err != nil {
		return nil, convertCommonErrors(v.db, "ListClosedWorkflowExecutionsByStatus", err)
	}

	return &persistence.InternalListWorkflowExecutionsResponse{
		Executions:    resp.Executions,
		NextPageToken: resp.NextPageToken,
	}, nil
}

func (v *nosqlVisibilityStore) GetClosedWorkflowExecution(
	ctx context.Context,
	request *persistence.InternalGetClosedWorkflowExecutionRequest,
) (*persistence.InternalGetClosedWorkflowExecutionResponse, error) {
	wfexecution, err := v.db.SelectOneClosedWorkflow(ctx, request.DomainUUID, request.Execution.GetWorkflowID(), request.Execution.GetRunID())

	if err != nil {
		return nil, convertCommonErrors(v.db, "GetClosedWorkflowExecution", err)
	}
	if wfexecution == nil {
		// Special case: this API return nil,nil if not found(since we will deprecate it, it's not worth refactor to be consistent)
		return nil, &types.EntityNotExistsError{
			Message: fmt.Sprintf("Workflow execution not found.  WorkflowId: %v, RunId: %v",
				request.Execution.GetWorkflowID(), request.Execution.GetRunID()),
		}
	}
	return &persistence.InternalGetClosedWorkflowExecutionResponse{
		Execution: wfexecution,
	}, nil
}

func (v *nosqlVisibilityStore) DeleteWorkflowExecution(
	ctx context.Context,
	request *persistence.VisibilityDeleteWorkflowExecutionRequest,
) error {
	err := v.db.DeleteVisibility(ctx, request.DomainID, request.WorkflowID, request.RunID)
	if err != nil {
		return convertCommonErrors(v.db, "DeleteWorkflowExecution", err)
	}
	return nil
}

func (v *nosqlVisibilityStore) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *persistence.VisibilityDeleteWorkflowExecutionRequest,
) error {
	// temporary: not implemented, only implemented for ES
	return nil
}

func (v *nosqlVisibilityStore) ListWorkflowExecutions(
	_ context.Context,
	_ *persistence.ListWorkflowExecutionsByQueryRequest,
) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	return nil, persistence.ErrVisibilityOperationNotSupported
}

func (v *nosqlVisibilityStore) ScanWorkflowExecutions(
	_ context.Context,
	_ *persistence.ListWorkflowExecutionsByQueryRequest) (*persistence.InternalListWorkflowExecutionsResponse, error) {
	return nil, persistence.ErrVisibilityOperationNotSupported
}

func (v *nosqlVisibilityStore) CountWorkflowExecutions(
	_ context.Context,
	_ *persistence.CountWorkflowExecutionsRequest,
) (*persistence.CountWorkflowExecutionsResponse, error) {
	return nil, persistence.ErrVisibilityOperationNotSupported
}
