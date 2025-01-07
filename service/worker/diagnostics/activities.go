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

	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/worker/diagnostics/analytics"
	"github.com/uber/cadence/service/worker/diagnostics/invariant"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/failure"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/retry"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/timeout"
)

const (
	linkToTimeoutsRunbook = "https://cadenceworkflow.io/docs/workflow-troubleshooting/timeouts/"
	linkToFailuresRunbook = "https://cadenceworkflow.io/docs/workflow-troubleshooting/activity-failures/"
	WfDiagnosticsAppName  = "workflow-diagnostics"
)

type retrieveExecutionHistoryInputParams struct {
	Domain    string
	Execution *types.WorkflowExecution
}

func (w *dw) retrieveExecutionHistory(ctx context.Context, info retrieveExecutionHistoryInputParams) (*types.GetWorkflowExecutionHistoryResponse, error) {
	frontendClient := w.clientBean.GetFrontendClient()
	return frontendClient.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    info.Domain,
		Execution: info.Execution,
	})
}

type identifyIssuesParams struct {
	History *types.GetWorkflowExecutionHistoryResponse
	Domain  string
}

func (w *dw) identifyIssues(ctx context.Context, info identifyIssuesParams) ([]invariant.InvariantCheckResult, error) {
	result := make([]invariant.InvariantCheckResult, 0)

	timeoutInvariant := timeout.NewInvariant(timeout.NewTimeoutParams{
		WorkflowExecutionHistory: info.History,
		Domain:                   info.Domain,
		ClientBean:               w.clientBean,
	})
	timeoutIssues, err := timeoutInvariant.Check(ctx)
	if err != nil {
		return nil, err
	}
	result = append(result, timeoutIssues...)

	failureInvariant := failure.NewInvariant(failure.Params{
		WorkflowExecutionHistory: info.History,
		Domain:                   info.Domain,
	})
	failureIssues, err := failureInvariant.Check(ctx)
	if err != nil {
		return nil, err
	}
	result = append(result, failureIssues...)

	retryInvariant := retry.NewInvariant(retry.Params{
		WorkflowExecutionHistory: info.History,
	})
	retryIssues, err := retryInvariant.Check(ctx)
	if err != nil {
		return nil, err
	}
	result = append(result, retryIssues...)

	return result, nil
}

type rootCauseIssuesParams struct {
	History *types.GetWorkflowExecutionHistoryResponse
	Domain  string
	Issues  []invariant.InvariantCheckResult
}

func (w *dw) rootCauseIssues(ctx context.Context, info rootCauseIssuesParams) ([]invariant.InvariantRootCauseResult, error) {
	result := make([]invariant.InvariantRootCauseResult, 0)
	timeoutInvariant := timeout.NewInvariant(timeout.NewTimeoutParams{
		WorkflowExecutionHistory: info.History,
		ClientBean:               w.clientBean,
		Domain:                   info.Domain,
	})
	timeoutRC, err := timeoutInvariant.RootCause(ctx, info.Issues)
	if err != nil {
		return nil, err
	}
	result = append(result, timeoutRC...)
	failureInvariant := failure.NewInvariant(failure.Params{
		WorkflowExecutionHistory: info.History,
		Domain:                   info.Domain,
	})
	failureRC, err := failureInvariant.RootCause(ctx, info.Issues)
	if err != nil {
		return nil, err
	}
	result = append(result, failureRC...)

	return result, nil
}

func (w *dw) emitUsageLogs(ctx context.Context, info analytics.WfDiagnosticsUsageData) error {
	client := w.newMessagingClient()
	return emit(ctx, info, client)
}

func (w *dw) newMessagingClient() messaging.Client {
	return kafka.NewKafkaClient(&w.kafkaCfg, w.metricsClient, w.logger, w.tallyScope, true)
}

func emit(ctx context.Context, info analytics.WfDiagnosticsUsageData, client messaging.Client) error {
	producer, err := client.NewProducer(WfDiagnosticsAppName)
	if err != nil {
		return err
	}
	emitter := analytics.NewEmitter(analytics.EmitterParams{
		Producer: producer,
	})
	return emitter.EmitUsageData(ctx, info)
}
