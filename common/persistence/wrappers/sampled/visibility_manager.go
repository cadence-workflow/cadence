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

package sampled

import (
	"context"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

const (
	// To sample visibility request, open has only 1 bucket, closed has 2
	numOfPriorityForOpen   = 1
	numOfPriorityForClosed = 2
	numOfPriorityForList   = 1
)

// errPersistenceLimitExceededForList is the error indicating QPS limit reached for list visibility.
var errPersistenceLimitExceededForList = &types.ServiceBusyError{Message: "Persistence Max QPS Reached for List Operations."}

type visibilityManager struct {
	rateLimitersForOpen   RateLimiterFactory
	rateLimitersForClosed RateLimiterFactory
	rateLimitersForList   RateLimiterFactory
	persistence           persistence.VisibilityManager
	metricClient          metrics.Client
	logger                log.Logger
}

type (
	// Config is config for visibility
	Config struct {
		VisibilityOpenMaxQPS dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		// VisibilityClosedMaxQPS max QPS for record closed workflows
		VisibilityClosedMaxQPS dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
		// VisibilityListMaxQPS max QPS for list workflow
		VisibilityListMaxQPS dynamicproperties.IntPropertyFnWithDomainFilter `yaml:"-" json:"-"`
	}
)

type Params struct {
	Config                 *Config
	MetricClient           metrics.Client
	Logger                 log.Logger
	TimeSource             clock.TimeSource
	RateLimiterFactoryFunc RateLimiterFactoryFunc
}

// NewVisibilityManager creates a client to manage visibility with sampling
// For write requests, it will do sampling which will lose some records
// For read requests, it will do sampling which will return service busy errors.
// Note that this is different from NewVisibilityPersistenceRateLimitedClient which is overlapping with the read processing.
func NewVisibilityManager(persistence persistence.VisibilityManager, p Params) persistence.VisibilityManager {
	return &visibilityManager{
		persistence:           persistence,
		rateLimitersForOpen:   p.RateLimiterFactoryFunc(p.TimeSource, numOfPriorityForOpen, p.Config.VisibilityOpenMaxQPS),
		rateLimitersForClosed: p.RateLimiterFactoryFunc(p.TimeSource, numOfPriorityForClosed, p.Config.VisibilityClosedMaxQPS),
		rateLimitersForList:   p.RateLimiterFactoryFunc(p.TimeSource, numOfPriorityForList, p.Config.VisibilityListMaxQPS),
		metricClient:          p.MetricClient,
		logger:                p.Logger,
	}
}

func (p *visibilityManager) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *persistence.RecordWorkflowExecutionStartedRequest,
) error {
	domain := request.Domain
	domainID := request.DomainUUID

	rateLimiter := p.rateLimitersForOpen.GetRateLimiter(domain)
	if ok, _ := rateLimiter.GetToken(0, 1); ok {
		return p.persistence.RecordWorkflowExecutionStarted(ctx, request)
	}

	p.logger.Info("Request for open workflow is sampled",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowDomainName(domain),
		tag.WorkflowType(request.WorkflowTypeName),
		tag.WorkflowID(request.Execution.GetWorkflowID()),
		tag.WorkflowRunID(request.Execution.GetRunID()),
	)
	p.metricClient.IncCounter(metrics.PersistenceRecordWorkflowExecutionStartedScope, metrics.PersistenceSampledCounter)
	return nil
}

func (p *visibilityManager) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *persistence.RecordWorkflowExecutionClosedRequest,
) error {
	domain := request.Domain
	domainID := request.DomainUUID
	priority := getRequestPriority(request)

	rateLimiter := p.rateLimitersForClosed.GetRateLimiter(domain)
	if ok, _ := rateLimiter.GetToken(priority, 1); ok {
		return p.persistence.RecordWorkflowExecutionClosed(ctx, request)
	}

	p.logger.Info("Request for closed workflow is sampled",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowDomainName(domain),
		tag.WorkflowType(request.WorkflowTypeName),
		tag.WorkflowID(request.Execution.GetWorkflowID()),
		tag.WorkflowRunID(request.Execution.GetRunID()),
	)
	p.metricClient.IncCounter(metrics.PersistenceRecordWorkflowExecutionClosedScope, metrics.PersistenceSampledCounter)
	return nil
}

func (p *visibilityManager) UpsertWorkflowExecution(
	ctx context.Context,
	request *persistence.UpsertWorkflowExecutionRequest,
) error {
	domain := request.Domain
	domainID := request.DomainUUID

	rateLimiter := p.rateLimitersForClosed.GetRateLimiter(domain)
	if ok, _ := rateLimiter.GetToken(0, 1); ok {
		return p.persistence.UpsertWorkflowExecution(ctx, request)
	}

	p.logger.Info("Request for upsert workflow is sampled",
		tag.WorkflowDomainID(domainID),
		tag.WorkflowDomainName(domain),
		tag.WorkflowType(request.WorkflowTypeName),
		tag.WorkflowID(request.Execution.GetWorkflowID()),
		tag.WorkflowRunID(request.Execution.GetRunID()),
	)
	p.metricClient.IncCounter(metrics.PersistenceUpsertWorkflowExecutionScope, metrics.PersistenceSampledCounter)
	return nil
}

func (p *visibilityManager) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListOpenWorkflowExecutions"); err != nil {
		return nil, err
	}

	return p.persistence.ListOpenWorkflowExecutions(ctx, request)
}

func (p *visibilityManager) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListClosedWorkflowExecutions"); err != nil {
		return nil, err
	}

	return p.persistence.ListClosedWorkflowExecutions(ctx, request)
}

func (p *visibilityManager) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByTypeRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListOpenWorkflowExecutionsByType"); err != nil {
		return nil, err
	}

	return p.persistence.ListOpenWorkflowExecutionsByType(ctx, request)
}

func (p *visibilityManager) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByTypeRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListClosedWorkflowExecutionsByType"); err != nil {
		return nil, err
	}

	return p.persistence.ListClosedWorkflowExecutionsByType(ctx, request)
}

func (p *visibilityManager) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByWorkflowIDRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListOpenWorkflowExecutionsByWorkflowID"); err != nil {
		return nil, err
	}

	return p.persistence.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
}

func (p *visibilityManager) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByWorkflowIDRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListClosedWorkflowExecutionsByWorkflowID"); err != nil {
		return nil, err
	}

	return p.persistence.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
}

func (p *visibilityManager) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *persistence.ListClosedWorkflowExecutionsByStatusRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	if err := p.tryConsumeListToken(request.Domain, "ListClosedWorkflowExecutionsByStatus"); err != nil {
		return nil, err
	}

	return p.persistence.ListClosedWorkflowExecutionsByStatus(ctx, request)
}

func (p *visibilityManager) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *persistence.RecordWorkflowExecutionUninitializedRequest,
) error {
	return p.persistence.RecordWorkflowExecutionUninitialized(ctx, request)
}

func (p *visibilityManager) GetClosedWorkflowExecution(
	ctx context.Context,
	request *persistence.GetClosedWorkflowExecutionRequest,
) (*persistence.GetClosedWorkflowExecutionResponse, error) {
	return p.persistence.GetClosedWorkflowExecution(ctx, request)
}

func (p *visibilityManager) DeleteWorkflowExecution(
	ctx context.Context,
	request *persistence.VisibilityDeleteWorkflowExecutionRequest,
) error {
	return p.persistence.DeleteWorkflowExecution(ctx, request)
}

func (p *visibilityManager) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *persistence.VisibilityDeleteWorkflowExecutionRequest,
) error {
	return p.persistence.DeleteUninitializedWorkflowExecution(ctx, request)
}

func (p *visibilityManager) ListWorkflowExecutions(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByQueryRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	return p.persistence.ListWorkflowExecutions(ctx, request)
}

func (p *visibilityManager) ScanWorkflowExecutions(
	ctx context.Context,
	request *persistence.ListWorkflowExecutionsByQueryRequest,
) (*persistence.ListWorkflowExecutionsResponse, error) {
	return p.persistence.ScanWorkflowExecutions(ctx, request)
}

func (p *visibilityManager) CountWorkflowExecutions(
	ctx context.Context,
	request *persistence.CountWorkflowExecutionsRequest,
) (*persistence.CountWorkflowExecutionsResponse, error) {
	return p.persistence.CountWorkflowExecutions(ctx, request)
}

func (p *visibilityManager) Close() {
	p.persistence.Close()
}

func (p *visibilityManager) GetName() string {
	return p.persistence.GetName()
}

func getRequestPriority(request *persistence.RecordWorkflowExecutionClosedRequest) int {
	priority := 0
	if request.Status == types.WorkflowExecutionCloseStatusCompleted {
		priority = 1 // low priority for completed workflows
	}
	return priority
}

func (p *visibilityManager) tryConsumeListToken(domain, method string) error {
	rateLimiter := p.rateLimitersForList.GetRateLimiter(domain)
	ok, _ := rateLimiter.GetToken(0, 1)
	if ok {
		p.logger.Debug("List API request consumed QPS token", tag.WorkflowDomainName(domain), tag.Name(method))
		return nil
	}
	p.logger.Debug("List API request is being sampled", tag.WorkflowDomainName(domain), tag.Name(method))
	return errPersistenceLimitExceededForList
}
