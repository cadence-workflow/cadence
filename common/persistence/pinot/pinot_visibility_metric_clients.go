// Copyright (c) 2017 Uber Technologies, Inc.
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

package pinotvisibility

import (
	"context"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type pinotVisibilityMetricsClient struct {
	metricClient metrics.Client
	persistence  p.VisibilityManager
	logger       log.Logger
}

var _ p.VisibilityManager = (*pinotVisibilityMetricsClient)(nil)

// NewPinotVisibilityMetricsClient wrap visibility client with metrics client
func NewPinotVisibilityMetricsClient(persistence p.VisibilityManager, metricClient metrics.Client, logger log.Logger) p.VisibilityManager {
	return &pinotVisibilityMetricsClient{
		persistence:  persistence,
		metricClient: metricClient,
		logger:       logger,
	}
}

func (p *pinotVisibilityMetricsClient) GetName() string {
	return p.persistence.GetName()
}

func (p *pinotVisibilityMetricsClient) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *p.RecordWorkflowExecutionStartedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotRecordWorkflowExecutionStartedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.RecordWorkflowExecutionStarted(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotRecordWorkflowExecutionStartedScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *p.RecordWorkflowExecutionClosedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotRecordWorkflowExecutionClosedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.RecordWorkflowExecutionClosed(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotRecordWorkflowExecutionClosedScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *p.RecordWorkflowExecutionUninitializedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotRecordWorkflowExecutionUninitializedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.RecordWorkflowExecutionUninitialized(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotRecordWorkflowExecutionUninitializedScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) UpsertWorkflowExecution(
	ctx context.Context,
	request *p.UpsertWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotUpsertWorkflowExecutionScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.UpsertWorkflowExecution(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotUpsertWorkflowExecutionScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListOpenWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListOpenWorkflowExecutions(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListOpenWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListClosedWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListClosedWorkflowExecutions(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListClosedWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByTypeRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListOpenWorkflowExecutionsByTypeScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListOpenWorkflowExecutionsByType(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListOpenWorkflowExecutionsByTypeScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByTypeRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListClosedWorkflowExecutionsByTypeScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListClosedWorkflowExecutionsByType(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListClosedWorkflowExecutionsByTypeScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByWorkflowIDRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListOpenWorkflowExecutionsByWorkflowIDScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListOpenWorkflowExecutionsByWorkflowIDScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByWorkflowIDRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListClosedWorkflowExecutionsByWorkflowIDScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListClosedWorkflowExecutionsByWorkflowIDScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *p.ListClosedWorkflowExecutionsByStatusRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListClosedWorkflowExecutionsByStatusScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListClosedWorkflowExecutionsByStatus(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListClosedWorkflowExecutionsByStatusScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) GetClosedWorkflowExecution(
	ctx context.Context,
	request *p.GetClosedWorkflowExecutionRequest,
) (*p.GetClosedWorkflowExecutionResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotGetClosedWorkflowExecutionScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.GetClosedWorkflowExecution(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotGetClosedWorkflowExecutionScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ListWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByQueryRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotListWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ListWorkflowExecutions(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotListWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) ScanWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByQueryRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotScanWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.ScanWorkflowExecutions(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotScanWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) CountWorkflowExecutions(
	ctx context.Context,
	request *p.CountWorkflowExecutionsRequest,
) (*p.CountWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotCountWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	response, err := p.persistence.CountWorkflowExecutions(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotCountWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *pinotVisibilityMetricsClient) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotDeleteWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.DeleteWorkflowExecution(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotDeleteWorkflowExecutionsScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.PinotDeleteWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.PinotRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.PinotLatencyPerDomain)
	defer sw.Stop()
	err := p.persistence.DeleteWorkflowExecution(ctx, request)

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.PinotDeleteWorkflowExecutionsScope, err)
	}

	return err
}

func (p *pinotVisibilityMetricsClient) updateErrorMetric(scopeWithDomainTag metrics.Scope, scope metrics.ScopeIdx, err error) {

	switch err.(type) {
	case *types.BadRequestError:
		scopeWithDomainTag.IncCounter(metrics.PinotErrBadRequestCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.PinotFailuresPerDomain)

	case *types.ServiceBusyError:
		scopeWithDomainTag.IncCounter(metrics.PinotErrBusyCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.PinotFailuresPerDomain)
	default:
		p.logger.Error("Operation failed with internal error.", tag.MetricScope(int(scope)), tag.Error(err))
		scopeWithDomainTag.IncCounter(metrics.PinotFailuresPerDomain)
	}
}

func (p *pinotVisibilityMetricsClient) Close() {
	p.persistence.Close()
}
