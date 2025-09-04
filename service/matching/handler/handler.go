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

package handler

import (
	"context"
	"sync"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	dynamicquotas "github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
)

type (
	// handlerImpl is an implementation for matching service independent of wire protocol
	handlerImpl struct {
		engine            Engine
		metricsClient     metrics.Client
		startWG           sync.WaitGroup
		userRateLimiter   quotas.Policy
		workerRateLimiter quotas.Policy
		logger            log.Logger
		throttledLogger   log.Logger
		domainCache       cache.DomainCache
	}
)

var (
	errMatchingHostThrottle = &types.ServiceBusyError{Message: "Matching host rps exceeded"}
)

// NewHandler creates a thrift handler for the matching service
func NewHandler(
	engine Engine,
	config *config.Config,
	domainCache cache.DomainCache,
	metricsClient metrics.Client,
	logger log.Logger,
	throttledLogger log.Logger,
) Handler {
	handler := &handlerImpl{
		metricsClient: metricsClient,
		userRateLimiter: quotas.NewMultiStageRateLimiter(
			quotas.NewDynamicRateLimiter(config.UserRPS.AsFloat64()),
			quotas.NewCollection(dynamicquotas.NewFallbackDynamicRateLimiterFactory(
				config.DomainUserRPS,
				config.UserRPS,
			)),
		),
		workerRateLimiter: quotas.NewMultiStageRateLimiter(
			quotas.NewDynamicRateLimiter(config.WorkerRPS.AsFloat64()),
			quotas.NewCollection(dynamicquotas.NewFallbackDynamicRateLimiterFactory(
				config.DomainWorkerRPS,
				config.WorkerRPS,
			)),
		),
		engine:          engine,
		logger:          logger,
		throttledLogger: throttledLogger,
		domainCache:     domainCache,
	}
	// prevent us from trying to serve requests before matching engine is started and ready
	handler.startWG.Add(1)
	return handler
}

// Start starts the handler
func (h *handlerImpl) Start() {
	h.engine.Start()
	h.startWG.Done()
}

// Stop stops the handler
func (h *handlerImpl) Stop() {
	h.engine.Stop()
}

// Health is for health check
func (h *handlerImpl) Health(ctx context.Context) (*types.HealthStatus, error) {
	h.startWG.Wait()
	h.logger.Debug("Matching service health check endpoint reached.")
	hs := &types.HealthStatus{Ok: true, Msg: "matching good"}
	return hs, nil
}

func (h *handlerImpl) newHandlerContext(
	ctx context.Context,
	domainName string,
	taskList *types.TaskList,
	scope metrics.ScopeIdx,
) *handlerContext {
	return newHandlerContext(
		ctx,
		domainName,
		taskList,
		h.metricsClient,
		scope,
		h.logger,
	)
}

// AddActivityTask - adds an activity task.
func (h *handlerImpl) AddActivityTask(
	ctx context.Context,
	request *types.AddActivityTaskRequest,
) (resp *types.AddActivityTaskResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetTaskList(),
		metrics.MatchingAddActivityTaskScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if request.GetForwardedFrom() != "" {
		hCtx.scope.IncCounter(metrics.ForwardedPerTaskListCounter)
	}

	if ok := h.workerRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	resp, err := h.engine.AddActivityTask(hCtx, request)
	return resp, hCtx.handleErr(err)
}

// AddDecisionTask - adds a decision task.
func (h *handlerImpl) AddDecisionTask(
	ctx context.Context,
	request *types.AddDecisionTaskRequest,
) (resp *types.AddDecisionTaskResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetTaskList(),
		metrics.MatchingAddDecisionTaskScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if request.GetForwardedFrom() != "" {
		hCtx.scope.IncCounter(metrics.ForwardedPerTaskListCounter)
	}

	if ok := h.workerRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	resp, err := h.engine.AddDecisionTask(hCtx, request)
	return resp, hCtx.handleErr(err)
}

// PollForActivityTask - long poll for an activity task.
func (h *handlerImpl) PollForActivityTask(
	ctx context.Context,
	request *types.MatchingPollForActivityTaskRequest,
) (resp *types.MatchingPollForActivityTaskResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetPollRequest().GetTaskList(),
		metrics.MatchingPollForActivityTaskScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if request.GetForwardedFrom() != "" {
		hCtx.scope.IncCounter(metrics.ForwardedPerTaskListCounter)
	}

	if ok := h.workerRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	if _, err := common.ValidateLongPollContextTimeoutIsSet(ctx,
		"PollForActivityTask",
		h.throttledLogger,
	); err != nil {
		return nil, hCtx.handleErr(err)
	}

	response, err := h.engine.PollForActivityTask(hCtx, request)
	return response, hCtx.handleErr(err)
}

// PollForDecisionTask - long poll for a decision task.
func (h *handlerImpl) PollForDecisionTask(
	ctx context.Context,
	request *types.MatchingPollForDecisionTaskRequest,
) (resp *types.MatchingPollForDecisionTaskResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetPollRequest().GetTaskList(),
		metrics.MatchingPollForDecisionTaskScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if request.GetForwardedFrom() != "" {
		hCtx.scope.IncCounter(metrics.ForwardedPerTaskListCounter)
	}

	if ok := h.workerRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	if _, err := common.ValidateLongPollContextTimeoutIsSet(
		ctx,
		"PollForDecisionTask",
		h.throttledLogger,
	); err != nil {
		return nil, hCtx.handleErr(err)
	}

	response, err := h.engine.PollForDecisionTask(hCtx, request)
	return response, hCtx.handleErr(err)
}

// QueryWorkflow queries a given workflow synchronously and return the query result.
func (h *handlerImpl) QueryWorkflow(
	ctx context.Context,
	request *types.MatchingQueryWorkflowRequest,
) (resp *types.MatchingQueryWorkflowResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetTaskList(),
		metrics.MatchingQueryWorkflowScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if request.GetForwardedFrom() != "" {
		hCtx.scope.IncCounter(metrics.ForwardedPerTaskListCounter)
	}

	if ok := h.userRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	response, err := h.engine.QueryWorkflow(hCtx, request)
	return response, hCtx.handleErr(err)
}

// RespondQueryTaskCompleted responds a query task completed
func (h *handlerImpl) RespondQueryTaskCompleted(
	ctx context.Context,
	request *types.MatchingRespondQueryTaskCompletedRequest,
) (retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetTaskList(),
		metrics.MatchingRespondQueryTaskCompletedScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	// Count the request in the RPS, but we still accept it even if RPS is exceeded
	h.workerRateLimiter.Allow(quotas.Info{Domain: domainName})

	err := h.engine.RespondQueryTaskCompleted(hCtx, request)
	return hCtx.handleErr(err)
}

// CancelOutstandingPoll is used to cancel outstanding pollers
func (h *handlerImpl) CancelOutstandingPoll(ctx context.Context,
	request *types.CancelOutstandingPollRequest) (retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetTaskList(),
		metrics.MatchingCancelOutstandingPollScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	// Count the request in the RPS, but we still accept it even if RPS is exceeded
	h.workerRateLimiter.Allow(quotas.Info{Domain: domainName})

	err := h.engine.CancelOutstandingPoll(hCtx, request)
	return hCtx.handleErr(err)
}

// DescribeTaskList returns information about the target tasklist, right now this API returns the
// pollers which polled this tasklist in last few minutes. If includeTaskListStatus field is true,
// it will also return status of tasklist's ackManager (readLevel, ackLevel, backlogCountHint and taskIDBlock).
func (h *handlerImpl) DescribeTaskList(
	ctx context.Context,
	request *types.MatchingDescribeTaskListRequest,
) (resp *types.DescribeTaskListResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.GetDomainUUID())
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.GetDescRequest().GetTaskList(),
		metrics.MatchingDescribeTaskListScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if ok := h.userRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	response, err := h.engine.DescribeTaskList(hCtx, request)
	return response, hCtx.handleErr(err)
}

// ListTaskListPartitions returns information about partitions for a taskList
func (h *handlerImpl) ListTaskListPartitions(
	ctx context.Context,
	request *types.MatchingListTaskListPartitionsRequest,
) (resp *types.ListTaskListPartitionsResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	hCtx := newHandlerContext(
		ctx,
		request.GetDomain(),
		request.GetTaskList(),
		h.metricsClient,
		metrics.MatchingListTaskListPartitionsScope,
		h.logger,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if ok := h.userRateLimiter.Allow(quotas.Info{Domain: request.GetDomain()}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	response, err := h.engine.ListTaskListPartitions(hCtx, request)
	return response, hCtx.handleErr(err)
}

// GetTaskListsByDomain returns information about partitions for a taskList
func (h *handlerImpl) GetTaskListsByDomain(
	ctx context.Context,
	request *types.GetTaskListsByDomainRequest,
) (resp *types.GetTaskListsByDomainResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	hCtx := newHandlerContext(
		ctx,
		request.GetDomain(),
		nil,
		h.metricsClient,
		metrics.MatchingGetTaskListsByDomainScope,
		h.logger,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if ok := h.userRateLimiter.Allow(quotas.Info{Domain: request.GetDomain()}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	response, err := h.engine.GetTaskListsByDomain(hCtx, request)
	return response, hCtx.handleErr(err)
}

func (h *handlerImpl) UpdateTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingUpdateTaskListPartitionConfigRequest,
) (resp *types.MatchingUpdateTaskListPartitionConfigResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.DomainUUID)
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.TaskList,
		metrics.MatchingUpdateTaskListPartitionConfigScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	if ok := h.userRateLimiter.Allow(quotas.Info{Domain: domainName}); !ok {
		return nil, hCtx.handleErr(errMatchingHostThrottle)
	}

	response, err := h.engine.UpdateTaskListPartitionConfig(hCtx, request)
	return response, hCtx.handleErr(err)
}

func (h *handlerImpl) RefreshTaskListPartitionConfig(
	ctx context.Context,
	request *types.MatchingRefreshTaskListPartitionConfigRequest,
) (resp *types.MatchingRefreshTaskListPartitionConfigResponse, retError error) {
	defer func() { log.CapturePanic(recover(), h.logger, &retError) }()

	domainName := h.domainName(request.DomainUUID)
	hCtx := h.newHandlerContext(
		ctx,
		domainName,
		request.TaskList,
		metrics.MatchingRefreshTaskListPartitionConfigScope,
	)

	sw := hCtx.startProfiling(&h.startWG)
	defer sw.Stop()

	// Count the request in the RPS, but we still accept it even if RPS is exceeded
	h.userRateLimiter.Allow(quotas.Info{Domain: domainName})

	response, err := h.engine.RefreshTaskListPartitionConfig(hCtx, request)
	return response, hCtx.handleErr(err)
}

func (h *handlerImpl) domainName(id string) string {
	domainName, err := h.domainCache.GetDomainName(id)
	if err != nil {
		return ""
	}
	return domainName
}
