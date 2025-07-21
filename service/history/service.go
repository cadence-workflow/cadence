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

package history

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	commonResource "github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/handler"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/workflowcache"
	"github.com/uber/cadence/service/history/wrappers/grpc"
	"github.com/uber/cadence/service/history/wrappers/ratelimited"
	"github.com/uber/cadence/service/history/wrappers/thrift"
)

// Default values if not provided by dynamic config
const (
	defaultWorkflowIDCacheTTL      = 1 * time.Second
	defaultWorkflowIDCacheMaxCount = 10_000
	
	// Shutdown constants
	shutdownPropagationTime  = 300 * time.Millisecond  // Reduced from 400ms
	shutdownGracePeriod      = 1 * time.Second         // Reduced from 2s
	shutdownParallelism      = 4                       // Number of parallel shutdown operations
)

// Service represents the cadence-history service
type Service struct {
	resource.Resource

	status       int32
	handler      handler.Handler
	stopC        chan struct{}
	params       *commonResource.Params
	config       *config.Config
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc
	shutdownWG   sync.WaitGroup
}

// NewService builds a new cadence-history service
func NewService(
	params *commonResource.Params,
) (resource.Resource, error) {
	serviceConfig := config.New(
		dynamicconfig.NewCollection(
			params.DynamicConfig,
			params.Logger,
			dynamicproperties.ClusterNameFilter(params.ClusterMetadata.GetCurrentClusterName()),
		),
		params.PersistenceConfig.NumHistoryShards,
		params.RPCFactory.GetMaxMessageSize(),
		params.PersistenceConfig.IsAdvancedVisibilityConfigExist(),
		params.HostName)

	serviceResource, err := resource.New(
		params,
		service.History,
		serviceConfig,
	)
	if err != nil {
		return nil, err
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &Service{
		Resource:     serviceResource,
		status:       common.DaemonStatusInitialized,
		stopC:        make(chan struct{}),
		params:       params,
		config:       serviceConfig,
		shutdownCtx:  ctx,
		shutdownFunc: cancelFunc,
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.GetLogger()
	metricsClient := s.GetMetricsClient()
	logger.Info("elastic search config", tag.ESConfig(s.params.ESConfig))
	logger.Info("history starting")

	// Start tracking service startup time
	startTime := time.Now()
	defer func() {
		metricsClient.Timer(metrics.ServiceStartupTimeScope, time.Since(startTime))
	}()

	// Use dynamic config with fallback to defaults
	cacheTTL := defaultWorkflowIDCacheTTL
	if s.config.WorkflowCacheTTL != nil {
		cacheTTL = s.config.WorkflowCacheTTL()
	}
	
	cacheMaxSize := defaultWorkflowIDCacheMaxCount
	if s.config.WorkflowCacheMaxSize != nil {
		cacheMaxSize = s.config.WorkflowCacheMaxSize()
	}

	// Create workflow ID cache with adaptive sizing based on host load
	wfIDCache := workflowcache.New(workflowcache.Params{
		TTL:                    cacheTTL,
		ExternalLimiterFactory: quotas.NewSimpleDynamicRateLimiterFactory(s.config.WorkflowIDExternalRPS),
		InternalLimiterFactory: quotas.NewSimpleDynamicRateLimiterFactory(s.config.WorkflowIDInternalRPS),
		MaxCount:               cacheMaxSize,
		DomainCache:            s.Resource.GetDomainCache(),
		Logger:                 logger.WithTags(tag.ComponentCache),
		MetricsClient:          metricsClient,
		LoadMonitor:            s.Resource.GetHostInfoProvider(), // For adaptive caching based on host load
	})

	rawHandler := handler.NewHandler(s.Resource, s.config, wfIDCache)
	
	// Apply rate limiting wrapper
	s.handler = ratelimited.NewHistoryHandler(
		rawHandler,
		wfIDCache,
		logger.WithTags(tag.ComponentRateLimiter),
	)

	// Register both Thrift and gRPC handlers
	thriftHandler := thrift.NewThriftHandler(s.handler)
	thriftHandler.Register(s.GetDispatcher())

	grpcHandler := grpc.NewGRPCHandler(s.handler)
	grpcHandler.Register(s.GetDispatcher())

	// Must start resource first
	s.Resource.Start()
	s.handler.Start()

	metricsClient.IncCounter(metrics.HistoryServiceScope, metrics.ServiceStartedCount)
	logger.Info("history started", tag.Address(s.params.HostName))

	<-s.stopC
}

// Stop stops the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	// Create a context with timeout for coordinating shutdown
	shutdownTimeout := s.config.ShutdownDrainDuration()
	ctx, cancel := context.WithTimeout(s.shutdownCtx, shutdownTimeout)
	defer cancel()

	logger := s.GetLogger()
	metricsClient := s.GetMetricsClient()
	
	// Track shutdown time metrics
	shutdownStartTime := time.Now()
	defer func() {
		metricsClient.Timer(metrics.ServiceShutdownTimeScope, time.Since(shutdownStartTime))
	}()

	// Calculate total allocated shutdown time
	totalTime := shutdownTimeout
	remainingTime := totalTime

	// Step 1: Signal beginning of shutdown via context cancellation
	s.shutdownFunc()
	logger.Info("ShutdownHandler: Initiated shutdown sequence", 
		tag.TotalShutdownTime(totalTime),
		tag.RemainingTime(remainingTime))

	// Step 2: Remove from membership ring and wait for propagation (in parallel with initial preparation)
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		logger.Info("ShutdownHandler: Evicting self from membership ring")
		s.GetMembershipResolver().EvictSelf()
	}()
	
	// Step 3: Begin preparing handler to stop (can start before membership fully propagated)
	s.shutdownWG.Add(1)
	go func() {
		defer s.shutdownWG.Done()
		logger.Info("ShutdownHandler: Preparing handler to stop incoming requests")
		s.handler.PrepareToStop(remainingTime / 2) // Allocate half the remaining time
	}()
	
	// Wait for both operations to complete
	waitCh := make(chan struct{})
	go func() {
		s.shutdownWG.Wait()
		close(waitCh)
	}()
	
	// Wait for completion or timeout
	select {
	case <-waitCh:
		// Both operations completed successfully
	case <-time.After(shutdownPropagationTime):
		logger.Warn("ShutdownHandler: Parallel shutdown operations timed out, continuing with shutdown")
	}
	
	// Recalculate remaining time
	elapsedTime := time.Since(shutdownStartTime)
	if elapsedTime > totalTime {
		remainingTime = 0
	} else {
		remainingTime = totalTime - elapsedTime
	}
	
	// Step 4: Final preparations with a reduced grace period
	minGracePeriod := common.MinDuration(shutdownGracePeriod, remainingTime/4)
	logger.Info("ShutdownHandler: Waiting grace period before final shutdown", 
		tag.RemainingTime(remainingTime),
		tag.GracePeriod(minGracePeriod))
		
	// Check if we still have time for the grace period
	select {
	case <-time.After(minGracePeriod):
		// Grace period completed
	case <-ctx.Done():
		logger.Warn("ShutdownHandler: Shutdown timeout reached, forcing immediate shutdown")
	}
	
	// Signal to stop the service main loop and complete shutdown
	close(s.stopC)
	
	// Step 5: Perform final shutdown in the correct order
	logger.Info("ShutdownHandler: Stopping handler")
	s.handler.Stop()
	
	logger.Info("ShutdownHandler: Stopping resources")
	s.Resource.Stop()

	metricsClient.IncCounter(metrics.HistoryServiceScope, metrics.ServiceStoppedCount)
	elapsedTime = time.Since(shutdownStartTime)
	logger.Info("History service shutdown complete", 
		tag.ShutdownDuration(elapsedTime),
		tag.Address(s.params.HostName))
}