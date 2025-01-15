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
	"context"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/analytics"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
)

const (
	linkToTimeoutsRunbook = "https://cadenceworkflow.io/docs/workflow-troubleshooting/timeouts/"
	linkToFailuresRunbook = "https://cadenceworkflow.io/docs/workflow-troubleshooting/activity-failures/"
	linkToRetriesRunbook  = "https://cadenceworkflow.io/docs/workflow-troubleshooting/retries"
	WfDiagnosticsAppName  = "workflow-diagnostics"
)

type identifyIssuesParams struct {
	Execution *types.WorkflowExecution
	Domain    string
}

func (w *dw) identifyIssues(ctx context.Context, info identifyIssuesParams) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)

	frontendClient := w.clientBean.GetFrontendClient()
	history, err := frontendClient.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    info.Domain,
		Execution: info.Execution,
	})
	if err != nil {
		return nil, err
	}

	for _, inv := range w.invariants {
		issues, err := inv.Check(ctx, invariant.InvariantCheckInput{
			WorkflowExecutionHistory: history,
			Domain:                   info.Domain,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, issues...)
	}

	return result, nil
}

type rootCauseIssuesParams struct {
	Domain string
	Issues []invariant.InvariantCheckResult
}

func (w *dw) rootCauseIssues(ctx context.Context, info rootCauseIssuesParams) ([]invariant.InvariantRootCauseResult, error) {
	result := make([]invariant.InvariantRootCauseResult, 0)

	for _, inv := range w.invariants {
		rootCause, err := inv.RootCause(ctx, invariant.InvariantRootCauseInput{
			Domain: info.Domain,
			Issues: info.Issues,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, rootCause...)
	}

	return result, nil
}

func (w *dw) emitUsageLogs(ctx context.Context, info analytics.WfDiagnosticsUsageData) error {
	if w.messagingClient == nil {
		// skip emitting logs if messaging client is not provided since it is optional
		w.logger.Error("messaging client is not provided, skipping emitting wf-diagnostics usage logs", tag.WorkflowDomainName(info.Domain))
		return nil
	}
	return w.emit(ctx, info, w.messagingClient)
}

func (w *dw) emit(ctx context.Context, info analytics.WfDiagnosticsUsageData, client messaging.Client) error {
	producer, err := client.NewProducer(WfDiagnosticsAppName)
	if err != nil {
		// skip emitting logs if producer cannot be created since it is optional
		w.logger.Error("producer creation failed, skipping emitting wf-diagnostics usage logs", tag.WorkflowDomainName(info.Domain))
		return nil
	}
	emitter := analytics.NewEmitter(analytics.EmitterParams{
		Producer: producer,
	})
	return emitter.EmitUsageData(ctx, info)
}
