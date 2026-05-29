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

//go:generate mockgen -package $GOPACKAGE -destination visibility_manager_interfaces_mock.go -self_package github.com/uber/cadence/common/persistence github.com/uber/cadence/common/persistence VisibilityManager
// Generate rate limiter wrapper.
//go:generate gowrap gen -g -p . -i VisibilityManager -t ./wrappers/templates/ratelimited.tmpl -o wrappers/ratelimited/visibility_generated.go

// Generate error injection wrapper.
//go:generate gowrap gen -g -p . -i VisibilityManager -t ./wrappers/templates/errorinjector.tmpl -o wrappers/errorinjectors/visibility_generated.go

// Generate metered wrapper.
//go:generate gowrap gen -g -p . -i VisibilityManager -t ./wrappers/templates/metered.tmpl -o wrappers/metered/visibility_generated.go

package persistence

import (
	"context"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/types"
)

// Interfaces for the Visibility Store.
// This is a secondary store that is eventually consistent with the main
// executions store, and stores workflow execution records for visibility
// purposes.

// ErrVisibilityOperationNotSupported is an error which indicates that operation is not supported in selected persistence
var ErrVisibilityOperationNotSupported = &types.BadRequestError{Message: "Operation is not supported"}

type (
	// RecordWorkflowExecutionStartedRequest is used to add a record of a newly
	// started execution
	RecordWorkflowExecutionStartedRequest struct {
		DomainUUID                  string
		Domain                      string // not persisted, used as config filter key
		Execution                   types.WorkflowExecution
		WorkflowTypeName            string
		StartTimestamp              int64
		ExecutionTimestamp          int64
		WorkflowTimeout             int64 // not persisted, used for cassandra ttl
		TaskID                      int64 // not persisted, used as condition update version for ES
		Memo                        *types.Memo
		TaskList                    string
		IsCron                      bool
		NumClusters                 int16
		ClusterAttributeScope       string
		ClusterAttributeName        string
		UpdateTimestamp             int64 // unit is unix nano, consistent with start/execution timestamp, same in other requests
		SearchAttributes            map[string][]byte
		ShardID                     int16
		ExecutionStatus             types.WorkflowExecutionStatus
		CronSchedule                string
		ScheduledExecutionTimestamp int64 // unit is unix nano, used to record the actual execution timestamp if it's a cron workflow
	}

	// RecordWorkflowExecutionClosedRequest is used to add a record of a newly
	// closed execution
	RecordWorkflowExecutionClosedRequest struct {
		DomainUUID                  string
		Domain                      string // not persisted, used as config filter key
		Execution                   types.WorkflowExecution
		WorkflowTypeName            string
		StartTimestamp              int64
		ExecutionTimestamp          int64
		CloseTimestamp              int64
		Status                      types.WorkflowExecutionCloseStatus
		HistoryLength               int64
		RetentionSeconds            int64
		TaskID                      int64 // not persisted, used as condition update version for ES
		Memo                        *types.Memo
		TaskList                    string
		IsCron                      bool
		CronSchedule                string
		NumClusters                 int16
		ClusterAttributeScope       string
		ClusterAttributeName        string
		UpdateTimestamp             int64
		SearchAttributes            map[string][]byte
		ShardID                     int16
		ExecutionStatus             types.WorkflowExecutionStatus
		ScheduledExecutionTimestamp int64
	}

	// RecordWorkflowExecutionUninitializedRequest is used to add a record of a newly uninitialized execution
	RecordWorkflowExecutionUninitializedRequest struct {
		DomainUUID       string
		Domain           string
		Execution        types.WorkflowExecution
		WorkflowTypeName string
		UpdateTimestamp  int64
		ShardID          int64
	}

	// UpsertWorkflowExecutionRequest is used to upsert workflow execution
	UpsertWorkflowExecutionRequest struct {
		DomainUUID                  string
		Domain                      string // not persisted, used as config filter key
		Execution                   types.WorkflowExecution
		WorkflowTypeName            string
		StartTimestamp              int64
		ExecutionTimestamp          int64
		WorkflowTimeout             int64 // not persisted, used for cassandra ttl
		TaskID                      int64 // not persisted, used as condition update version for ES
		Memo                        *types.Memo
		TaskList                    string
		IsCron                      bool
		NumClusters                 int16
		ClusterAttributeScope       string
		ClusterAttributeName        string
		UpdateTimestamp             int64
		SearchAttributes            map[string][]byte
		ShardID                     int64
		ExecutionStatus             types.WorkflowExecutionStatus
		CronSchedule                string
		ScheduledExecutionTimestamp int64 // unit is unix nano, used to record the actual execution timestamp if it's a cron workflow
	}

	// ListWorkflowExecutionsRequest is used to list executions in a domain
	ListWorkflowExecutionsRequest struct {
		DomainUUID string
		Domain     string // domain name is not persisted, but used as config filter key
		// The earliest end of the time range
		EarliestTime int64
		// The latest end of the time range
		LatestTime int64
		// Maximum number of workflow executions per page
		PageSize int
		// Token to continue reading next page of workflow executions.
		// Pass in empty slice for first page.
		NextPageToken []byte
	}

	// ListWorkflowExecutionsByQueryRequest is used to list executions in a domain
	ListWorkflowExecutionsByQueryRequest struct {
		DomainUUID string
		Domain     string // domain name is not persisted, but used as config filter key
		PageSize   int    // Maximum number of workflow executions per page
		// Token to continue reading next page of workflow executions.
		// Pass in empty slice for first page.
		NextPageToken []byte
		Query         string
	}

	// ListWorkflowExecutionsResponse is the response to ListWorkflowExecutionsRequest
	ListWorkflowExecutionsResponse struct {
		Executions []*types.WorkflowExecutionInfo
		// Token to read next page if there are more workflow executions beyond page size.
		// Use this to set NextPageToken on ListWorkflowExecutionsRequest to read the next page.
		NextPageToken []byte
	}

	// CountWorkflowExecutionsRequest is request from CountWorkflowExecutions
	CountWorkflowExecutionsRequest struct {
		DomainUUID string
		Domain     string // domain name is not persisted, but used as config filter key
		Query      string
	}

	// CountWorkflowExecutionsResponse is response to CountWorkflowExecutions
	CountWorkflowExecutionsResponse struct {
		Count int64
	}

	// ListWorkflowExecutionsByTypeRequest is used to list executions of
	// a specific type in a domain
	ListWorkflowExecutionsByTypeRequest struct {
		ListWorkflowExecutionsRequest
		WorkflowTypeName string
	}

	// ListWorkflowExecutionsByWorkflowIDRequest is used to list executions that
	// have specific WorkflowID in a domain
	ListWorkflowExecutionsByWorkflowIDRequest struct {
		ListWorkflowExecutionsRequest
		WorkflowID string
	}

	// ListClosedWorkflowExecutionsByStatusRequest is used to list executions that
	// have specific close status
	ListClosedWorkflowExecutionsByStatusRequest struct {
		ListWorkflowExecutionsRequest
		Status types.WorkflowExecutionCloseStatus
	}

	// GetClosedWorkflowExecutionRequest is used retrieve the record for a specific execution
	GetClosedWorkflowExecutionRequest struct {
		DomainUUID string
		Domain     string // domain name is not persisted, but used as config filter key
		Execution  types.WorkflowExecution
	}

	// GetClosedWorkflowExecutionResponse is the response to GetClosedWorkflowExecutionRequest
	GetClosedWorkflowExecutionResponse struct {
		Execution *types.WorkflowExecutionInfo
	}

	// VisibilityDeleteWorkflowExecutionRequest contains the request params for DeleteWorkflowExecution call
	VisibilityDeleteWorkflowExecutionRequest struct {
		DomainID   string
		Domain     string
		RunID      string
		WorkflowID string
		TaskID     int64
	}

	VisibilityAdminDeletionKey string

	// VisibilityManager is used to manage the visibility store
	VisibilityManager interface {
		Closeable
		GetName() string
		RecordWorkflowExecutionStarted(ctx context.Context, request *RecordWorkflowExecutionStartedRequest) error
		RecordWorkflowExecutionClosed(ctx context.Context, request *RecordWorkflowExecutionClosedRequest) error
		RecordWorkflowExecutionUninitialized(ctx context.Context, request *RecordWorkflowExecutionUninitializedRequest) error
		UpsertWorkflowExecution(ctx context.Context, request *UpsertWorkflowExecutionRequest) error
		ListOpenWorkflowExecutions(ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error)
		ListClosedWorkflowExecutions(ctx context.Context, request *ListWorkflowExecutionsRequest) (*ListWorkflowExecutionsResponse, error)
		ListOpenWorkflowExecutionsByType(ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error)
		ListClosedWorkflowExecutionsByType(ctx context.Context, request *ListWorkflowExecutionsByTypeRequest) (*ListWorkflowExecutionsResponse, error)
		ListOpenWorkflowExecutionsByWorkflowID(ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error)
		ListClosedWorkflowExecutionsByWorkflowID(ctx context.Context, request *ListWorkflowExecutionsByWorkflowIDRequest) (*ListWorkflowExecutionsResponse, error)
		ListClosedWorkflowExecutionsByStatus(ctx context.Context, request *ListClosedWorkflowExecutionsByStatusRequest) (*ListWorkflowExecutionsResponse, error)
		DeleteWorkflowExecution(ctx context.Context, request *VisibilityDeleteWorkflowExecutionRequest) error
		ListWorkflowExecutions(ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error)
		ScanWorkflowExecutions(ctx context.Context, request *ListWorkflowExecutionsByQueryRequest) (*ListWorkflowExecutionsResponse, error)
		CountWorkflowExecutions(ctx context.Context, request *CountWorkflowExecutionsRequest) (*CountWorkflowExecutionsResponse, error)
		// NOTE: GetClosedWorkflowExecution is only for persistence testing, currently no index is supported for filtering by RunID
		GetClosedWorkflowExecution(ctx context.Context, request *GetClosedWorkflowExecutionRequest) (*GetClosedWorkflowExecutionResponse, error)
		DeleteUninitializedWorkflowExecution(ctx context.Context, request *VisibilityDeleteWorkflowExecutionRequest) error
	}
)

// IsNopUpsertWorkflowRequest return whether upsert request should be no-op
func IsNopUpsertWorkflowRequest(request *InternalUpsertWorkflowExecutionRequest) bool {
	_, exist := request.SearchAttributes[definition.CadenceChangeVersion]
	return exist
}
