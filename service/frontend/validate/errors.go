// Copyright (c) 2021 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package validate

import (
	"github.com/google/uuid"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/config"
)

var (
	ErrInvalidFilters                             = &types.BadRequestError{Message: "Request Filters are invalid, unable to parse."}
	ErrDomainNotSet                               = &types.BadRequestError{Message: "Domain not set on request."}
	ErrTaskTokenNotSet                            = &types.BadRequestError{Message: "Task token not set on request."}
	ErrInvalidTaskToken                           = &types.BadRequestError{Message: "Invalid TaskToken."}
	ErrTaskListNotSet                             = &types.BadRequestError{Message: "TaskList is not set on request."}
	ErrTaskListTypeNotSet                         = &types.BadRequestError{Message: "TaskListType is not set on request."}
	ErrExecutionNotSet                            = &types.BadRequestError{Message: "Execution is not set on request."}
	ErrWorkflowIDNotSet                           = &types.BadRequestError{Message: "WorkflowId is not set on request."}
	ErrActivityIDNotSet                           = &types.BadRequestError{Message: "ActivityID is not set on request."}
	ErrSignalNameNotSet                           = &types.BadRequestError{Message: "SignalName is not set on request."}
	ErrInvalidRunID                               = &types.BadRequestError{Message: "Invalid RunId."}
	ErrInvalidNextPageToken                       = &types.BadRequestError{Message: "Invalid NextPageToken."}
	ErrNextPageTokenRunIDMismatch                 = &types.BadRequestError{Message: "RunID in the request does not match the NextPageToken."}
	ErrQueryNotSet                                = &types.BadRequestError{Message: "WorkflowQuery is not set on request."}
	ErrQueryTypeNotSet                            = &types.BadRequestError{Message: "QueryType is not set on request."}
	ErrRequestNotSet                              = &types.BadRequestError{Message: "Request is nil."}
	ErrNoPermission                               = &types.BadRequestError{Message: "No permission to do this operation."}
	ErrWorkflowTypeNotSet                         = &types.BadRequestError{Message: "WorkflowType is not set on request."}
	ErrInvalidRetention                           = &types.BadRequestError{Message: "RetentionDays is invalid."}
	ErrInvalidExecutionStartToCloseTimeoutSeconds = &types.BadRequestError{Message: "A valid ExecutionStartToCloseTimeoutSeconds is not set on request."}
	ErrInvalidTaskStartToCloseTimeoutSeconds      = &types.BadRequestError{Message: "A valid TaskStartToCloseTimeoutSeconds is not set on request."}
	ErrInvalidDelayStartSeconds                   = &types.BadRequestError{Message: "A valid DelayStartSeconds is not set on request."}
	ErrInvalidJitterStartSeconds                  = &types.BadRequestError{Message: "A valid JitterStartSeconds is not set on request (negative)."}
	ErrInvalidJitterStartSeconds2                 = &types.BadRequestError{Message: "A valid JitterStartSeconds is not set on request (larger than cron duration)."}
	ErrQueryDisallowedForDomain                   = &types.BadRequestError{Message: "Domain is not allowed to query, please contact cadence team to re-enable queries."}
	ErrClusterNameNotSet                          = &types.BadRequestError{Message: "Cluster name is not set."}
	ErrEmptyReplicationInfo                       = &types.BadRequestError{Message: "Replication task info is not set."}
	ErrEmptyQueueType                             = &types.BadRequestError{Message: "Queue type is not set."}
	ErrDomainInLockdown                           = &types.BadRequestError{Message: "Domain is not accepting fail overs at this time due to lockdown."}
	ErrShuttingDown                               = &types.InternalServiceError{Message: "Shutting down"}
	ErrBatchIDNotSet                              = &types.BadRequestError{Message: "BatchID is not set on request."}
	ErrBatchOperationNotSet                       = &types.BadRequestError{Message: "Operation is not set on request."}
	ErrBatchOperationTypeNotSet                   = &types.BadRequestError{Message: "Operation.OperationType is not set or invalid on request."}

	// Err for archival
	ErrHistoryNotFound = &types.BadRequestError{Message: "Requested workflow history not found, may have passed retention period."}

	// Err for string too long
	ErrDomainTooLong       = &types.BadRequestError{Message: "Domain length exceeds limit."}
	ErrWorkflowTypeTooLong = &types.BadRequestError{Message: "WorkflowType length exceeds limit."}
	ErrWorkflowIDTooLong   = &types.BadRequestError{Message: "WorkflowID length exceeds limit."}
	ErrSignalNameTooLong   = &types.BadRequestError{Message: "SignalName length exceeds limit."}
	ErrTaskListTooLong     = &types.BadRequestError{Message: "TaskList length exceeds limit."}
	ErrRequestIDTooLong    = &types.BadRequestError{Message: "RequestID length exceeds limit."}
	ErrIdentityTooLong     = &types.BadRequestError{Message: "Identity length exceeds limit."}
)

func CheckPermission(
	config *config.Config,
	securityToken string,
) error {
	if config.EnableAdminProtection() {
		if securityToken == "" {
			return ErrNoPermission
		}
		requiredToken := config.AdminOperationToken()
		if securityToken != requiredToken {
			return ErrNoPermission
		}
	}
	return nil
}

func CheckExecution(w *types.WorkflowExecution) error {
	if w == nil {
		return ErrExecutionNotSet
	}
	if w.GetWorkflowID() == "" {
		return ErrWorkflowIDNotSet
	}
	if w.GetRunID() != "" {
		if _, err := uuid.Parse(w.GetRunID()); err != nil {
			return ErrInvalidRunID
		}
	}
	return nil
}
