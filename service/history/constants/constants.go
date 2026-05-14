// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package constants

import "github.com/uber/cadence/common/types"

var (
	ErrDomainNotSet            = &types.BadRequestError{Message: "Domain not set on request."}
	ErrWorkflowExecutionNotSet = &types.BadRequestError{Message: "WorkflowExecution not set on request."}
	ErrTaskListNotSet          = &types.BadRequestError{Message: "Tasklist not set."}
	ErrRunIDNotValid           = &types.BadRequestError{Message: "RunID is not valid UUID."}
	ErrWorkflowIDNotSet        = &types.BadRequestError{Message: "WorkflowId is not set on request."}
	ErrSourceClusterNotSet     = &types.BadRequestError{Message: "Source Cluster not set on request."}
	ErrTimestampNotSet         = &types.BadRequestError{Message: "Timestamp not set on request."}
	ErrInvalidTaskType         = &types.BadRequestError{Message: "Invalid task type"}
	ErrHistoryHostThrottle     = &types.ServiceBusyError{Message: "History host rps exceeded"}
	ErrShuttingDown            = &types.InternalServiceError{Message: "Shutting down"}
)

const (
	// HistoryTaskDLQModeEnabled enables writing tasks to the DLQ.
	HistoryTaskDLQModeEnabled = "enabled"
	// HistoryTaskDLQModeDisabled disables writing tasks to the DLQ.
	HistoryTaskDLQModeDisabled = "disabled"
	// HistoryTaskDLQModeShadow enables writing tasks to the DLQ but does not process the task.
	HistoryTaskDLQModeShadow = "shadow"
)
