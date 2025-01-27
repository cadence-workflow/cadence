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
	"fmt"
	"time"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/worker/diagnostics/analytics"
)

const (
	diagnosticsStarterWorkflow = "diagnostics-starter-workflow"
	emitUsageLogsActivity      = "emitUsageLogs"
	queryDiagnosticsReport     = "query-diagnostics-report"

	issueTypeTimeouts = "Timeout"
	issueTypeFailures = "Failure"
	issueTypeRetry    = "Retry"
)

type DiagnosticsStarterWorkflowInput struct {
	Domain     string
	Identity   string
	WorkflowID string
	RunID      string
}

type DiagnosticsStarterWorkflowResult struct {
	DiagnosticsResult    *DiagnosticsWorkflowResult
	DiagnosticsCompleted bool
}

func (w *dw) DiagnosticsStarterWorkflow(ctx workflow.Context, params DiagnosticsStarterWorkflowInput) (*DiagnosticsStarterWorkflowResult, error) {
	var diagWfResult DiagnosticsWorkflowResult
	workflowResult := DiagnosticsStarterWorkflowResult{
		DiagnosticsResult: &diagWfResult,
	}
	err := workflow.SetQueryHandler(ctx, queryDiagnosticsReport, func() (DiagnosticsStarterWorkflowResult, error) {
		return workflowResult, nil
	})
	if err != nil {
		return nil, err
	}

	future := workflow.ExecuteChildWorkflow(ctx, w.DiagnosticsWorkflow, DiagnosticsWorkflowInput{
		Domain:     params.Domain,
		WorkflowID: params.WorkflowID,
		RunID:      params.RunID,
	})

	var childWfExec workflow.Execution
	var childWfStart, childWfEnd time.Time
	if err = future.GetChildWorkflowExecution().Get(ctx, &childWfExec); err != nil {
		return nil, fmt.Errorf("Workflow Diagnostics start failed: %w", err)
	}
	childWfStart = workflow.Now(ctx)

	err = future.Get(ctx, &diagWfResult)
	if err != nil {
		return nil, fmt.Errorf("Workflow Diagnostics failed: %w", err)
	}
	workflowResult.DiagnosticsCompleted = true
	childWfEnd = workflow.Now(ctx)

	info := workflow.GetInfo(ctx)
	activityOptions := workflow.ActivityOptions{
		ScheduleToCloseTimeout: time.Second * 10,
		ScheduleToStartTimeout: time.Second * 5,
		StartToCloseTimeout:    time.Second * 5,
	}
	activityCtx := workflow.WithActivityOptions(ctx, activityOptions)
	err = workflow.ExecuteActivity(activityCtx, emitUsageLogsActivity, analytics.WfDiagnosticsUsageData{
		Domain:                params.Domain,
		WorkflowID:            params.WorkflowID,
		RunID:                 params.RunID,
		Identity:              params.Identity,
		IssueType:             getIssueType(diagWfResult),
		DiagnosticsWorkflowID: childWfExec.ID,
		DiagnosticsRunID:      childWfExec.RunID,
		DiagnosticsStartTime:  childWfStart,
		DiagnosticsEndTime:    childWfEnd,
	}).Get(ctx, nil)
	if err != nil {
		w.logger.Error("wf-diagnostics usage logs emission failed",
			tag.Error(err),
			tag.WorkflowID(info.WorkflowExecution.ID),
			tag.WorkflowRunID(info.WorkflowExecution.RunID))
	}

	return &workflowResult, nil
}

func getIssueType(result DiagnosticsWorkflowResult) string {
	var issueType string
	if result.Timeouts != nil {
		issueType = fmt.Sprintf("%s-%s", issueType, issueTypeTimeouts)
	}
	if result.Failures != nil {
		issueType = fmt.Sprintf("%s-%s", issueType, issueTypeFailures)
	}
	if result.Retries != nil {
		issueType = fmt.Sprintf("%s-%s", issueType, issueTypeRetry)
	}
	return issueType
}
