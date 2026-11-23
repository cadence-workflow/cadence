// Copyright (c) 2017-2021 Uber Technologies, Inc.
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

package workflow

import (
	"errors"

	"github.com/uber/cadence/common/types"
)

var (
	// ErrStaleState is the error returned during state update indicating that cached mutable state could be stale
	ErrStaleState = errors.New("cache mutable state could potentially be stale")
	// ErrMaxAttemptsExceeded is exported temporarily for integration test
	ErrMaxAttemptsExceeded = errors.New("maximum attempts exceeded to update history")
	// ErrActivityTaskNotFound is the error to indicate activity task could be duplicate and activity already completed
	ErrActivityTaskNotFound = &types.EntityNotExistsError{Message: "activity task not found"}
	// ErrNotExists is the error to indicate workflow doesn't exist
	ErrNotExists = &types.EntityNotExistsError{Message: "workflow execution already completed"}
	// ErrAlreadyCompleted is the error to indicate workflow execution already completed
	ErrAlreadyCompleted = &types.WorkflowExecutionAlreadyCompletedError{Message: "workflow execution already completed"}
	// ErrParentMismatch is the error to parent execution is given and mismatch
	ErrParentMismatch = &types.EntityNotExistsError{Message: "workflow parent does not match"}
	// ErrDeserializingToken is the error to indicate task token is invalid
	ErrDeserializingToken = &types.BadRequestError{Message: "error deserializing task token"}
	// ErrSerializingToken is the error to indicate task token can not be serialized
	ErrSerializingToken = &types.BadRequestError{Message: "error serializing task token"}
	// ErrSignalOverSize is the error to indicate signal input size is > 256K
	ErrSignalOverSize = &types.BadRequestError{Message: "signal input size is over 256K"}
	// ErrCancellationAlreadyRequested is the error indicating cancellation for target workflow is already requested
	ErrCancellationAlreadyRequested = &types.CancellationAlreadyRequestedError{Message: "cancellation already requested for this workflow execution"}
	// ErrSignalsLimitExceeded is the error indicating limit reached for maximum number of signal events
	ErrSignalsLimitExceeded = &types.LimitExceededError{Message: "exceeded workflow execution limit for signal events"}
	// ErrQueryEnteredInvalidState is error indicating query entered invalid state
	ErrQueryEnteredInvalidState = &types.BadRequestError{Message: "query entered invalid state, this should be impossible"}
	// ErrQueryWorkflowBeforeFirstDecision is error indicating that query was attempted before first decision task completed
	ErrQueryWorkflowBeforeFirstDecision = &types.QueryFailedError{Message: "workflow must handle at least one decision task before it can be queried"}
	// ErrConsistentQueryNotEnabled is error indicating that consistent query was requested but either cluster or domain does not enable consistent query
	ErrConsistentQueryNotEnabled = &types.BadRequestError{Message: "cluster or domain does not enable strongly consistent query but strongly consistent query was requested"}
	// ErrConsistentQueryBufferExceeded is error indicating that too many consistent queries have been buffered and until buffered queries are finished new consistent queries cannot be buffered
	ErrConsistentQueryBufferExceeded = &types.InternalServiceError{Message: "consistent query buffer is full, cannot accept new consistent queries"}
	// ErrConcurrentStartRequest is error indicating there is an outstanding start workflow request. The incoming request fails to acquires the lock before the outstanding request finishes.
	ErrConcurrentStartRequest = &types.ServiceBusyError{Message: "an outstanding start workflow request is in-progress. Failed to acquire the resource."}
	// ErrImportInProgress is the error indicating that the workflow is an import in progress, which accepts no external change
	ErrImportInProgress = &types.BadRequestError{Message: "workflow import is in progress"}
)
