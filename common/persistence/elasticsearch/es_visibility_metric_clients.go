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

package elasticsearch

import (
	"context"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type visibilityMetricsClient struct {
	metricClient metrics.Client
	persistence  p.VisibilityManager
	logger       log.Logger
}

var _ p.VisibilityManager = (*visibilityMetricsClient)(nil)

// NewVisibilityMetricsClient wrap visibility client with metrics
func NewVisibilityMetricsClient(persistence p.VisibilityManager, metricClient metrics.Client, logger log.Logger) p.VisibilityManager {
	return &visibilityMetricsClient{
		persistence:  persistence,
		metricClient: metricClient,
		logger:       logger,
	}
}

func (p *visibilityMetricsClient) GetName() string {
	return p.persistence.GetName()
}

func (p *visibilityMetricsClient) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *p.RecordWorkflowExecutionStartedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchRecordWorkflowExecutionStartedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)

	err := p.persistence.RecordWorkflowExecutionStarted(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchRecordWorkflowExecutionStartedScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *p.RecordWorkflowExecutionClosedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchRecordWorkflowExecutionClosedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	err := p.persistence.RecordWorkflowExecutionClosed(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchRecordWorkflowExecutionClosedScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *p.RecordWorkflowExecutionUninitializedRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchRecordWorkflowExecutionUninitializedScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	err := p.persistence.RecordWorkflowExecutionUninitialized(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchRecordWorkflowExecutionUninitializedScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) UpsertWorkflowExecution(
	ctx context.Context,
	request *p.UpsertWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchUpsertWorkflowExecutionScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	err := p.persistence.UpsertWorkflowExecution(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchUpsertWorkflowExecutionScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListOpenWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListOpenWorkflowExecutions(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListOpenWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListClosedWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListClosedWorkflowExecutions(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListClosedWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByTypeRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListOpenWorkflowExecutionsByTypeScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)

	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListOpenWorkflowExecutionsByType(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListOpenWorkflowExecutionsByTypeScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByTypeRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListClosedWorkflowExecutionsByTypeScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListClosedWorkflowExecutionsByType(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListClosedWorkflowExecutionsByTypeScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByWorkflowIDRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListOpenWorkflowExecutionsByWorkflowIDScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListOpenWorkflowExecutionsByWorkflowIDScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByWorkflowIDRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListClosedWorkflowExecutionsByWorkflowIDScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListClosedWorkflowExecutionsByWorkflowIDScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *p.ListClosedWorkflowExecutionsByStatusRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListClosedWorkflowExecutionsByStatusScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ListClosedWorkflowExecutionsByStatus(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListClosedWorkflowExecutionsByStatusScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) GetClosedWorkflowExecution(
	ctx context.Context,
	request *p.GetClosedWorkflowExecutionRequest,
) (*p.GetClosedWorkflowExecutionResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchGetClosedWorkflowExecutionScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)

	response, err := p.persistence.GetClosedWorkflowExecution(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchGetClosedWorkflowExecutionScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ListWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByQueryRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchListWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)

	response, err := p.persistence.ListWorkflowExecutions(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchListWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) ScanWorkflowExecutions(
	ctx context.Context,
	request *p.ListWorkflowExecutionsByQueryRequest,
) (*p.ListWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchScanWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	response, err := p.persistence.ScanWorkflowExecutions(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchScanWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) CountWorkflowExecutions(
	ctx context.Context,
	request *p.CountWorkflowExecutionsRequest,
) (*p.CountWorkflowExecutionsResponse, error) {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchCountWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)

	response, err := p.persistence.CountWorkflowExecutions(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchCountWorkflowExecutionsScope, err)
	}

	return response, err
}

func (p *visibilityMetricsClient) DeleteWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchDeleteWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	err := p.persistence.DeleteWorkflowExecution(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchDeleteWorkflowExecutionsScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *p.VisibilityDeleteWorkflowExecutionRequest,
) error {

	scopeWithDomainTag := p.metricClient.Scope(metrics.ElasticsearchDeleteWorkflowExecutionsScope, metrics.DomainTag(request.Domain))
	scopeWithDomainTag.IncCounter(metrics.ElasticsearchRequestsPerDomain)
	sw := scopeWithDomainTag.StartTimer(metrics.ElasticsearchLatencyPerDomain)
	err := p.persistence.DeleteWorkflowExecution(ctx, request)
	sw.Stop()

	if err != nil {
		p.updateErrorMetric(scopeWithDomainTag, metrics.ElasticsearchDeleteWorkflowExecutionsScope, err)
	}

	return err
}

func (p *visibilityMetricsClient) updateErrorMetric(scopeWithDomainTag metrics.Scope, scope metrics.ScopeIdx, err error) {

	switch err.(type) {
	case *types.BadRequestError:
		scopeWithDomainTag.IncCounter(metrics.ElasticsearchErrBadRequestCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.ElasticsearchFailuresPerDomain)

	case *types.ServiceBusyError:
		scopeWithDomainTag.IncCounter(metrics.ElasticsearchErrBusyCounterPerDomain)
		scopeWithDomainTag.IncCounter(metrics.ElasticsearchFailuresPerDomain)
	default:
		p.logger.Error("Operation failed with internal error.", tag.MetricScope(int(scope)), tag.Error(err))
		scopeWithDomainTag.IncCounter(metrics.ElasticsearchFailuresPerDomain)
	}
}

func (p *visibilityMetricsClient) Close() {
	p.persistence.Close()
}
