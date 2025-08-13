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

package diagnostics

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/failure"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/retry"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/timeout"
)

const (
	diagnosticsWorkflow = "diagnostics-workflow"
	tasklist            = "diagnostics-wf-tasklist"

	identifyIssuesActivity  = "identifyIssues"
	rootCauseIssuesActivity = "rootCauseIssues"
)

type DiagnosticsWorkflowInput struct {
	Domain     string
	WorkflowID string
	RunID      string
}

type DiagnosticsWorkflowResult struct {
	Timeouts *timeoutDiagnostics
	Failures *failureDiagnostics
	Retries  *retryDiagnostics
}

type timeoutDiagnostics struct {
	Issues    []*timeoutIssuesResult
	RootCause []*timeoutRootCauseResult
	Runbook   string
}

type timeoutIssuesResult struct {
	IssueID       int
	InvariantType string
	Reason        string
	Metadata      *timeout.TimeoutIssuesMetadata
}

type timeoutRootCauseResult struct {
	IssueID       int
	RootCauseType string
	Metadata      *timeout.TimeoutRootcauseMetadata
}

type failureDiagnostics struct {
	Issues    []*failureIssuesResult
	RootCause []*failureRootCauseResult
	Runbook   string
}

type failureIssuesResult struct {
	IssueID       int
	InvariantType string
	Reason        string
	Metadata      *failure.FailureIssuesMetadata
}

type failureRootCauseResult struct {
	IssueID       int
	RootCauseType string
	Metadata      *failure.FailureRootcauseMetadata
}

type retryDiagnostics struct {
	Issues  []*retryIssuesResult
	Runbook string
}

type retryIssuesResult struct {
	IssueID       int
	InvariantType string
	Reason        string
	Metadata      retry.RetryMetadata
}

func (w *dw) DiagnosticsWorkflow(ctx workflow.Context, params DiagnosticsWorkflowInput) (*DiagnosticsWorkflowResult, error) {
	scope := w.metricsClient.Scope(metrics.DiagnosticsWorkflowScope, metrics.DomainTag(params.Domain))
	scope.IncCounter(metrics.DiagnosticsWorkflowStartedCount)
	sw := scope.StartTimer(metrics.DiagnosticsWorkflowExecutionLatency)
	defer sw.Stop()

	var timeoutsResult *timeoutDiagnostics
	var failureResult *failureDiagnostics
	var retryResult *retryDiagnostics
	var checkResult []invariant.InvariantCheckResult
	var rootCauseResult []invariant.InvariantRootCauseResult

	activityOptions := workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Second * 10,
		ScheduleToStartTimeout: time.Second * 5,
		StartToCloseTimeout:    time.Second * 5,
	}
	activityCtx := workflow.WithActivityOptions(ctx, activityOptions)

	err := workflow.ExecuteActivity(activityCtx, identifyIssuesActivity, identifyIssuesParams{
		Execution: &types.WorkflowExecution{
			WorkflowID: params.WorkflowID,
			RunID:      params.RunID,
		},
		Domain: params.Domain,
	}).Get(ctx, &checkResult)
	if err != nil {
		return nil, fmt.Errorf("IdentifyIssues: %w", err)
	}

	err = workflow.ExecuteActivity(activityCtx, rootCauseIssuesActivity, rootCauseIssuesParams{
		Domain: params.Domain,
		Issues: checkResult,
	}).Get(ctx, &rootCauseResult)
	if err != nil {
		return nil, fmt.Errorf("RootCauseIssues: %w", err)
	}

	timeoutIssues, err := retrieveTimeoutIssues(checkResult)
	if err != nil {
		return nil, fmt.Errorf("RetrieveTimeoutIssues: %w", err)
	}

	if len(timeoutIssues) > 0 {
		timeoutRootCause, err := retrieveTimeoutRootCause(rootCauseResult)
		if err != nil {
			return nil, fmt.Errorf("RetrieveTimeoutRootCause: %w", err)
		}
		timeoutsResult = &timeoutDiagnostics{
			Issues:    timeoutIssues,
			RootCause: timeoutRootCause,
			Runbook:   linkToTimeoutsRunbook,
		}
	}

	failureIssues, err := retrieveFailureIssues(checkResult)
	if err != nil {
		return nil, fmt.Errorf("RetrieveFailureIssues: %w", err)
	}

	if len(failureIssues) > 0 {
		failureRootCause, err := retrieveFailureRootCause(rootCauseResult)
		if err != nil {
			return nil, fmt.Errorf("RetrieveFailureRootCause: %w", err)
		}
		failureResult = &failureDiagnostics{
			Issues:    failureIssues,
			RootCause: failureRootCause,
			Runbook:   linkToFailuresRunbook,
		}
	}

	retryIssues, err := retrieveRetryIssues(checkResult)
	if err != nil {
		return nil, fmt.Errorf("RetrieveRetryIssues: %w", err)
	}

	if len(retryIssues) > 0 {
		retryResult = &retryDiagnostics{
			Issues:  retryIssues,
			Runbook: linkToRetriesRunbook,
		}
	}

	scope.IncCounter(metrics.DiagnosticsWorkflowSuccess)
	return &DiagnosticsWorkflowResult{
		Timeouts: timeoutsResult,
		Failures: failureResult,
		Retries:  retryResult,
	}, nil
}

func retrieveTimeoutIssues(issues []invariant.InvariantCheckResult) ([]*timeoutIssuesResult, error) {
	result := make([]*timeoutIssuesResult, 0)
	for _, issue := range issues {
		switch issue.InvariantType {
		case timeout.TimeoutTypeExecution.String():
			var metadata timeout.ExecutionTimeoutMetadata
			err := json.Unmarshal(issue.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata: &timeout.TimeoutIssuesMetadata{
					ExecutionTimeout: &metadata,
				},
			})
		case timeout.TimeoutTypeActivity.String():
			var metadata timeout.ActivityTimeoutMetadata
			err := json.Unmarshal(issue.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata: &timeout.TimeoutIssuesMetadata{
					ActivityTimeout: &metadata,
				},
			})
		case timeout.TimeoutTypeChildWorkflow.String():
			var metadata timeout.ChildWfTimeoutMetadata
			err := json.Unmarshal(issue.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata: &timeout.TimeoutIssuesMetadata{
					ChildWfTimeout: &metadata,
				},
			})
		case timeout.TimeoutTypeDecision.String():
			var metadata timeout.DecisionTimeoutMetadata
			err := json.Unmarshal(issue.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata: &timeout.TimeoutIssuesMetadata{
					DecisionTimeout: &metadata,
				},
			})
		}
	}
	return result, nil
}

func retrieveTimeoutRootCause(rootCause []invariant.InvariantRootCauseResult) ([]*timeoutRootCauseResult, error) {
	result := make([]*timeoutRootCauseResult, 0)
	for _, rc := range rootCause {
		if rootCausePollersRelated(rc.RootCause) {
			var metadata timeout.PollersMetadata
			err := json.Unmarshal(rc.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutRootCauseResult{
				IssueID:       rc.IssueID,
				RootCauseType: rc.RootCause.String(),
				Metadata: &timeout.TimeoutRootcauseMetadata{
					PollersMetadata: &metadata,
				},
			})
		} else if rootCauseHeartBeatRelated(rc.RootCause) {
			var metadata timeout.HeartbeatingMetadata
			err := json.Unmarshal(rc.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &timeoutRootCauseResult{
				IssueID:       rc.IssueID,
				RootCauseType: rc.RootCause.String(),
				Metadata: &timeout.TimeoutRootcauseMetadata{
					HeartBeatingMetadata: &metadata,
				},
			})
		}
	}

	return result, nil
}

func retrieveFailureIssues(issues []invariant.InvariantCheckResult) ([]*failureIssuesResult, error) {
	result := make([]*failureIssuesResult, 0)
	for _, issue := range issues {
		if issue.InvariantType == failure.ActivityFailed.String() || issue.InvariantType == failure.WorkflowFailed.String() || issue.InvariantType == failure.DecisionCausedFailure.String() {
			var data failure.FailureIssuesMetadata
			err := json.Unmarshal(issue.Metadata, &data)
			if err != nil {
				return nil, err
			}
			result = append(result, &failureIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata:      &data,
			})
		}
	}
	return result, nil
}

func retrieveFailureRootCause(rootCause []invariant.InvariantRootCauseResult) ([]*failureRootCauseResult, error) {
	result := make([]*failureRootCauseResult, 0)
	for _, rc := range rootCause {
		if rc.RootCause == invariant.RootCauseTypeServiceSideIssue || rc.RootCause == invariant.RootCauseTypeServiceSidePanic || rc.RootCause == invariant.RootCauseTypeServiceSideCustomError {
			result = append(result, &failureRootCauseResult{
				IssueID:       rc.IssueID,
				RootCauseType: rc.RootCause.String(),
			})
		}
		if rc.RootCause == invariant.RootCauseTypeBlobSizeLimit {
			var metadata failure.BlobSizeMetadata
			err := json.Unmarshal(rc.Metadata, &metadata)
			if err != nil {
				return nil, err
			}
			result = append(result, &failureRootCauseResult{
				IssueID:       rc.IssueID,
				RootCauseType: rc.RootCause.String(),
				Metadata: &failure.FailureRootcauseMetadata{
					BlobSizeMetadata: &metadata,
				},
			})
		}
	}
	return result, nil
}

func retrieveRetryIssues(issues []invariant.InvariantCheckResult) ([]*retryIssuesResult, error) {
	result := make([]*retryIssuesResult, 0)
	for _, issue := range issues {
		if issueRetryRelated(issue) {
			var data retry.RetryMetadata
			err := json.Unmarshal(issue.Metadata, &data)
			if err != nil {
				return nil, err
			}
			result = append(result, &retryIssuesResult{
				IssueID:       issue.IssueID,
				InvariantType: issue.InvariantType,
				Reason:        issue.Reason,
				Metadata:      data,
			})
		}
	}
	return result, nil
}

func rootCauseHeartBeatRelated(rootCause invariant.RootCause) bool {
	for _, rc := range []invariant.RootCause{invariant.RootCauseTypeNoHeartBeatTimeoutNoRetryPolicy,
		invariant.RootCauseTypeHeartBeatingNotEnabledWithRetryPolicy,
		invariant.RootCauseTypeHeartBeatingEnabledWithoutRetryPolicy,
		invariant.RootCauseTypeHeartBeatingEnabledMissingHeartbeat} {
		if rc == rootCause {
			return true
		}
	}
	return false
}

func rootCausePollersRelated(rootCause invariant.RootCause) bool {
	for _, rc := range []invariant.RootCause{invariant.RootCauseTypePollersStatus, invariant.RootCauseTypeMissingPollers} {
		if rc == rootCause {
			return true
		}
	}
	return false
}

func issueRetryRelated(issue invariant.InvariantCheckResult) bool {
	for _, i := range []string{retry.WorkflowRetryIssue.String(), retry.ActivityRetryIssue.String(), retry.ActivityHeartbeatIssue.String()} {
		if issue.InvariantType == i {
			return true
		}
	}
	return false
}
