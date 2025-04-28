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

package domaindeprecation

import "time"

const (
	// ErrDomainDoesNotExistNonRetryable is error reason used for Cadence non-retryable errors
	ErrDomainDoesNotExistNonRetryable = "domain does not exist"
	// ErrAccessDeniedNonRetryable is permission error that require manual intervention
	ErrAccessDeniedNonRetryable = "AccessDeniedError"
	// ErrWorkflowAlreadyCompletedNonRetryable is error that indicates workflow execution already completed
	ErrWorkflowAlreadyCompletedNonRetryable = "WorkflowExecutionAlreadyCompletedError"

	// DefaultRPS is the default RPS
	DefaultRPS = 50
	// DefaultConcurrency is the default concurrency
	DefaultConcurrency = 5
	// DefaultPageSize is the default page size
	DefaultPageSize = 1000
	// DefaultAttemptsOnRetryableError is the default value for AttemptsOnRetryableError
	DefaultAttemptsOnRetryableError = 50
	// DefaultActivityHeartBeatTimeout is the default value for ActivityHeartBeatTimeout
	DefaultActivityHeartBeatTimeout = time.Second * 10
	// DefaultMaxActivityRetries is the default value for MaxActivityRetries
	DefaultMaxActivityRetries = 4
)
