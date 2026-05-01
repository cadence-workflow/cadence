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

package invariant

import (
	"context"
	"encoding/json"

	"github.com/uber/cadence/common/types"
)

// InvariantCheckResult is the result from the invariant check
type InvariantCheckResult struct {
	IssueID       int
	InvariantType string
	Reason        string
	Metadata      []byte
}

// InvariantRootCauseResult is the root cause for the issues identified in the invariant check
type InvariantRootCauseResult struct {
	IssueID   int
	RootCause RootCause
	Metadata  []byte
}

type InvariantCheckInput struct {
	WorkflowExecutionHistory *types.GetWorkflowExecutionHistoryResponse
	Domain                   string
}

type InvariantRootCauseInput struct {
	Domain string
	Issues []InvariantCheckResult
}
type RootCause string

const (
	RootCauseTypeMissingPollers                        RootCause = "There are no pollers for the tasklist"
	RootCauseTypePollersStatus                         RootCause = "There are pollers for the tasklist. Check backlog status"
	RootCauseTypeNoHeartBeatTimeoutNoRetryPolicy       RootCause = "Heartbeat timeout and retry policy are not configured"
	RootCauseTypeHeartBeatingNotEnabledWithRetryPolicy RootCause = "Heartbeat timeout not enabled for activity but there is a retry policy configured"
	RootCauseTypeHeartBeatingEnabledWithoutRetryPolicy RootCause = "Heartbeat timeout enabled for activity but there is no retry policy configured"
	RootCauseTypeHeartBeatingEnabledMissingHeartbeat   RootCause = "Heartbeat timeout enabled for activity but timed out due to missing heartbeat"
	RootCauseTypeServiceSideIssue                      RootCause = "There is an issue in the worker service that is causing a failure. Check identity for service logs"
	RootCauseTypeServiceSidePanic                      RootCause = "There is a panic in the activity/workflow that is causing a failure"
	RootCauseTypeServiceSideCustomError                RootCause = "Customised error returned by the activity/workflow"
	RootCauseTypeBlobSizeLimit                         RootCause = "Workflow has exceeded the blob size limits configured for the domain"
)

func (r RootCause) String() string {
	return string(r)
}

// Invariant represents a condition of a workflow execution.
type Invariant interface {
	Check(context.Context, InvariantCheckInput) ([]InvariantCheckResult, error)
	RootCause(context.Context, InvariantRootCauseInput) ([]InvariantRootCauseResult, error)
}

func MarshalData(rc any) []byte {
	data, _ := json.Marshal(rc)
	return data
}
