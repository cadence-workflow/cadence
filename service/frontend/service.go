// Copyright (c) 2019 Uber Technologies, Inc.
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

package frontend

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/quotas/global/collection"
	"github.com/uber/cadence/common/quotas/permember"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/frontend/admin"
	"github.com/uber/cadence/service/frontend/api"
	"github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/wrappers/accesscontrolled"
	"github.com/uber/cadence/service/frontend/wrappers/clusterredirection"
	"github.com/uber/cadence/service/frontend/wrappers/grpc"
	"github.com/uber/cadence/service/frontend/wrappers/metered"
	"github.com/uber/cadence/service/frontend/wrappers/ratelimited"
	"github.com/uber/cadence/service/frontend/wrappers/thrift"
	"github.com/uber/cadence/service/frontend/wrappers/versioncheck"
)

// Service represents the cadence-frontend service
type Service struct {
	resource.Resource

	status                 int32
	handler                *api.WorkflowHandler
	adminHandler           admin.Handler
	stopC                  chan struct{}
	config                 *config.Config
	params                 *resource.Params
	ratelimiterCollections globalRatelimiterCollections
	dc                     *dynamicconfig.Collection
}

// NewService builds a new cadence-frontend service
func NewService(
	params *resource.Params,
) (resource.Resource, error) {

	isAdvancedVisExistInConfig := len(params.PersistenceConfig.AdvancedVisibilityStore) != 0
	dc := dynamicconfig.NewCollection(
		params.DynamicConfig,
		params.Logger,
		dynamicproperties.ClusterNameFilter(params.ClusterMetadata.GetCurrentClusterName()),
	)
	serviceConfig := config.NewConfig(
		dc,
		params.PersistenceConfig.NumHistoryShards,
		isAdvancedVisExistInConfig,
		params.HostName,
		params.Logger,
	)

	serviceResource, err := resource.New(
		params,
		service.Frontend,
		&service.Config{
			PersistenceMaxQPS:       serviceConfig.PersistenceMaxQPS,
			PersistenceGlobalMaxQPS: serviceConfig.PersistenceGlobalMaxQPS,
			ThrottledLoggerMaxRPS:   serviceConfig.ThrottledLogRPS,

			WriteVisibilityStoreName:        nil, // frontend service never write
			EnableLogCustomerQueryParameter: serviceConfig.EnableLogCustomerQueryParameter,
			ReadVisibilityStoreName:         serviceConfig.ReadVisibilityStoreName,

			EnableDBVisibilitySampling:                  serviceConfig.EnableVisibilitySampling,
			EnableReadDBVisibilityFromClosedExecutionV2: serviceConfig.EnableReadFromClosedExecutionV2,
			DBVisibilityListMaxQPS:                      serviceConfig.VisibilityListMaxQPS,
			WriteDBVisibilityOpenMaxQPS:                 nil, // frontend service never write
			WriteDBVisibilityClosedMaxQPS:               nil, // frontend service never write

			ESVisibilityListMaxQPS:     serviceConfig.ESVisibilityListMaxQPS,
			ESIndexMaxResultWindow:     serviceConfig.ESIndexMaxResultWindow,
			ValidSearchAttributes:      serviceConfig.ValidSearchAttributes,
			IsErrorRetryableFunction:   common.FrontendRetry,
			PinotOptimizedQueryColumns: serviceConfig.PinotOptimizedQueryColumns,
		},
	)
	if err != nil {
		return nil, err
	}

	return &Service{
		Resource: serviceResource,
		status:   common.DaemonStatusInitialized,
		config:   serviceConfig,
		stopC:    make(chan struct{}),
		params:   params,
		dc:       dc,
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.GetLogger()
	logger.Info("frontend starting")

	// domain handler's shared between admin and workflow handler, so instantiate it centrally and share it
	dh := domain.NewHandler(
		s.config.DomainConfig,
		s.GetLogger(),
		s.GetDomainManager(),
		s.GetDomainAuditManager(),
		s.GetClusterMetadata(),
		domain.NewDomainReplicator(s.GetDomainReplicationQueue(), s.GetLogger()),
		s.GetArchivalMetadata(),
		s.GetArchiverProvider(),
		s.GetTimeSource(),
	)

	// Base handler
	s.handler = api.NewWorkflowHandler(s, s.config, client.NewVersionChecker(), dh)

	collections, err := s.createGlobalQuotaCollections()
	if err != nil {
		logger.Fatal("constructing ratelimiter collections", tag.Error(err))
	}
	userRateLimiter := quotas.NewMultiStageRateLimiter(quotas.NewDynamicRateLimiter(s.config.UserRPS.AsFloat64()), collections.user)
	workerRateLimiter := quotas.NewMultiStageRateLimiter(quotas.NewDynamicRateLimiter(s.config.WorkerRPS.AsFloat64()), collections.worker)
	visibilityRateLimiter := quotas.NewMultiStageRateLimiter(quotas.NewDynamicRateLimiter(s.config.VisibilityRPS.AsFloat64()), collections.visibility)
	asyncRateLimiter := quotas.NewMultiStageRateLimiter(quotas.NewDynamicRateLimiter(s.config.AsyncRPS.AsFloat64()), collections.async)

	// Additional decorations
	var handler api.Handler = s.handler
	handler = versioncheck.NewAPIHandler(handler, s.config, client.NewVersionChecker())
	handler = ratelimited.NewAPIHandler(handler, s.GetDomainCache(), userRateLimiter, workerRateLimiter, visibilityRateLimiter, asyncRateLimiter, s.config.MaxWorkerPollDelay, s.dc)
	handler = metered.NewAPIHandler(handler, s.GetLogger(), s.GetMetricsClient(), s.GetDomainCache(), s.config)
	if s.params.ClusterRedirectionPolicy != nil {
		handler = clusterredirection.NewAPIHandler(handler, s, s.config, *s.params.ClusterRedirectionPolicy)
	}
	handler = accesscontrolled.NewAPIHandler(handler, s, s.params.Authorizer, s.params.AuthorizationConfig)

	// Register the latest (most decorated) handler
	thriftHandler := thrift.NewAPIHandler(handler)
	thriftHandler.Register(s.GetDispatcher())

	grpcHandler := grpc.NewAPIHandler(handler)
	grpcHandler.Register(s.GetDispatcher())

	s.adminHandler = admin.NewHandler(s, s.params, s.config, dh)
	s.adminHandler = accesscontrolled.NewAdminHandler(s.adminHandler, s, s.params.Authorizer, s.params.AuthorizationConfig)

	adminThriftHandler := thrift.NewAdminHandler(s.adminHandler)
	adminThriftHandler.Register(s.GetDispatcher())

	adminGRPCHandler := grpc.NewAdminHandler(s.adminHandler)
	adminGRPCHandler.Register(s.GetDispatcher())

	// must start resource first
	s.Resource.Start()

	startCtx, cancel := context.WithTimeout(context.Background(), time.Second) // should take nearly no time at all
	defer cancel()
	if err := collections.user.OnStart(startCtx); err != nil {
		logger.Fatal("failed to start user global ratelimiter collection", tag.Error(err))
	}
	if err := collections.worker.OnStart(startCtx); err != nil {
		logger.Fatal("failed to start worker global ratelimiter collection", tag.Error(err))
	}
	if err := collections.visibility.OnStart(startCtx); err != nil {
		logger.Fatal("failed to start visibility global ratelimiter collection", tag.Error(err))
	}
	if err := collections.async.OnStart(startCtx); err != nil {
		logger.Fatal("failed to start async global ratelimiter collection", tag.Error(err))
	}
	cancel()
	s.ratelimiterCollections = collections // save so they can be stopped later

	s.handler.Start()
	s.adminHandler.Start()

	// base (service is not started in frontend or admin handler) in case of race condition in yarpc registration function

	logger.Info("frontend started")

	<-s.stopC
}

type globalRatelimiterCollections struct {
	user, worker, visibility, async *collection.Collection
}

// ratelimiterCollections contains the "base" ratelimiters that make up both:
// - "local" limits, which do not use global-load-balancing to adjust to request load
// - fallbacks within "global" limits, for when the global-load information cannot be retrieved (startup, errors, etc)
type ratelimiterCollections struct {
	user, worker, visibility, async *quotas.Collection
}

func (s *Service) createGlobalQuotaCollections() (globalRatelimiterCollections, error) {
	create := func(name string, local, global *quotas.Collection, targetRPS dynamicproperties.IntPropertyFnWithDomainFilter) (*collection.Collection, error) {
		c, err := collection.New(
			name,
			local,
			global,
			s.config.GlobalRatelimiterUpdateInterval,
			targetRPS,
			s.config.GlobalRatelimiterKeyMode,
			s.GetRatelimiterAggregatorsClient(),
			s.GetLogger(),
			s.GetMetricsClient(),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating %v collection: %w", name, err)
		}
		return c, nil
	}
	var combinedErr error

	// to safely shadow global ratelimits, we must make duplicate *quota.Collection collections
	// so they do not share data when the global limiter decides to use its local fallback.
	// these are then combined into the global/algorithm.Collection to handle all limiting calls
	local, global := s.createBaseLimiters(), s.createBaseLimiters()

	user, err := create("user", local.user, global.user, s.config.GlobalDomainUserRPS)
	combinedErr = multierr.Combine(combinedErr, err)

	worker, err := create("worker", local.worker, global.worker, s.config.GlobalDomainWorkerRPS)
	combinedErr = multierr.Combine(combinedErr, err)

	visibility, err := create("visibility", local.visibility, global.visibility, s.config.GlobalDomainVisibilityRPS)
	combinedErr = multierr.Combine(combinedErr, err)

	async, err := create("async", local.async, global.async, s.config.GlobalDomainAsyncRPS)
	combinedErr = multierr.Combine(combinedErr, err)

	return globalRatelimiterCollections{
		user:       user,
		worker:     worker,
		visibility: visibility,
		async:      async,
	}, combinedErr
}
func (s *Service) createBaseLimiters() ratelimiterCollections {
	create := func(shared, perInstance dynamicproperties.IntPropertyFnWithDomainFilter) *quotas.Collection {
		return quotas.NewCollection(permember.NewPerMemberDynamicRateLimiterFactory(
			service.Frontend,
			shared,
			perInstance,
			s.GetMembershipResolver(),
		))
	}
	return ratelimiterCollections{
		user:       create(s.config.GlobalDomainUserRPS, s.config.MaxDomainUserRPSPerInstance),
		worker:     create(s.config.GlobalDomainWorkerRPS, s.config.MaxDomainWorkerRPSPerInstance),
		visibility: create(s.config.GlobalDomainVisibilityRPS, s.config.MaxDomainVisibilityRPSPerInstance),
		async:      create(s.config.GlobalDomainAsyncRPS, s.config.MaxDomainAsyncRPSPerInstance),
	}
}

// Stop stops the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	// initiate graceful shutdown:
	// 1. Fail rpc health check, this will cause client side load balancer to stop forwarding requests to this node
	// 2. wait for failure detection time
	// 3. stop taking new requests by returning InternalServiceError
	// 4. Wait for a second
	// 5. Stop everything forcefully and return

	requestDrainTime := min(time.Second, s.config.ShutdownDrainDuration())
	failureDetectionTime := max(0, s.config.ShutdownDrainDuration()-requestDrainTime)

	s.GetLogger().Info("ShutdownHandler: Updating rpc health status to ShuttingDown")
	s.handler.UpdateHealthStatus(api.HealthStatusShuttingDown)

	s.GetLogger().Info("ShutdownHandler: Waiting for others to discover I am unhealthy")
	time.Sleep(failureDetectionTime)

	s.handler.Stop()
	s.adminHandler.Stop()

	s.stopRatelimiters()

	s.GetLogger().Info("ShutdownHandler: Draining traffic")
	time.Sleep(requestDrainTime)

	close(s.stopC)
	s.Resource.Stop()
	s.params.Logger.Info("frontend stopped")
}

func (s *Service) stopRatelimiters() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second) // should take nearly no time at all
	defer cancel()
	if err := s.ratelimiterCollections.user.OnStop(ctx); err != nil {
		s.GetLogger().Error("failed to stop user global ratelimiter collection", tag.Error(err))
	}
	if err := s.ratelimiterCollections.worker.OnStop(ctx); err != nil {
		s.GetLogger().Error("failed to stop worker global ratelimiter collection", tag.Error(err))
	}
	if err := s.ratelimiterCollections.visibility.OnStop(ctx); err != nil {
		s.GetLogger().Error("failed to stop visibility global ratelimiter collection", tag.Error(err))
	}
	if err := s.ratelimiterCollections.async.OnStop(ctx); err != nil {
		s.GetLogger().Error("failed to stop async global ratelimiter collection", tag.Error(err))
	}
}
