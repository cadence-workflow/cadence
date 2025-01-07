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

package batcher

import (
	"time"

	"github.com/uber/cadence/common/types"
)

// TerminateParams is the parameters for terminating workflow
type TerminateParams struct {
	// this indicates whether to terminate children workflow. Default to true.
	// TODO https://github.com/uber/cadence/issues/2159
	// Ideally default should be childPolicy of the workflow. But it's currently totally broken.
	TerminateChildren *bool
}

// CancelParams is the parameters for canceling workflow
type CancelParams struct {
	// this indicates whether to cancel children workflow. Default to true.
	// TODO https://github.com/uber/cadence/issues/2159
	// Ideally default should be childPolicy of the workflow. But it's currently totally broken.
	CancelChildren *bool
}

// SignalParams is the parameters for signaling workflow
type SignalParams struct {
	SignalName string
	Input      string
}

// ReplicateParams is the parameters for replicating workflow
type ReplicateParams struct {
	SourceCluster string
	TargetCluster string
}

// BatchParams is the parameters for batch operation workflow
type BatchParams struct {
	// Target domain to execute batch operation
	DomainName string
	// To get the target workflows for processing
	Query string
	// Reason for the operation
	Reason string
	// Supporting: reset,terminate
	BatchType string

	// Below are all optional
	// TerminateParams is params only for BatchTypeTerminate
	TerminateParams TerminateParams
	// CancelParams is params only for BatchTypeCancel
	CancelParams CancelParams
	// SignalParams is params only for BatchTypeSignal
	SignalParams SignalParams
	// ReplicateParams is params only for BatchTypeReplicate
	ReplicateParams ReplicateParams
	// RPS of processing. Default to DefaultRPS
	// TODO we will implement smarter way than this static rate limiter: https://github.com/uber/cadence/issues/2138
	RPS int
	// Number of goroutines running in parallel to process
	Concurrency int
	// Number of workflows processed in a batch
	PageSize int
	// Number of attempts for each workflow to process in case of retryable error before giving up
	AttemptsOnRetryableError int
	// timeout for activity heartbeat
	ActivityHeartBeatTimeout time.Duration
	// Max number of attempts for the activity to retry. Default to 0 (unlimited)
	MaxActivityRetries int
	// errors that will not retry which consumes AttemptsOnRetryableError. Default to empty
	NonRetryableErrors []string
	// internal conversion for NonRetryableErrors
	_nonRetryableErrors map[string]struct{}
}

// HeartBeatDetails is the struct for heartbeat details
type HeartBeatDetails struct {
	PageToken   []byte
	CurrentPage int
	// This is just an estimation for visibility
	TotalEstimate int64
	// Number of workflows processed successfully
	SuccessCount int
	// Number of workflows that give up due to errors.
	ErrorCount int
}

type taskDetail struct {
	execution types.WorkflowExecution
	attempts  int
	// passing along the current heartbeat details to make heartbeat within a task so that it won't timeout
	hbd HeartBeatDetails
}
