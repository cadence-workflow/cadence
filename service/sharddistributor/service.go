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

package sharddistributor

import (
	"sync/atomic"

	"go.uber.org/fx"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/handler"
	"github.com/uber/cadence/service/sharddistributor/wrappers/grpc"
	"github.com/uber/cadence/service/sharddistributor/wrappers/metered"
)

// Service represents the shard distributor service
type Service struct {
	logger        log.Logger
	metricsClient metrics.Client
	dispatcher    *yarpc.Dispatcher

	handler      handler.Handler
	config       *config.Config
	peerProvider membership.PeerProvider

	matchingRing membership.SingleProvider
	historyRing  membership.SingleProvider

	// DEPRECATED: does not affect fx lifecycle.
	stopC    chan struct{}
	status   int32
	resource resource.Resource
}

type ServiceParams struct {
	fx.In

	Logger            log.Logger
	MetricsClient     metrics.Client
	RPCFactory        rpc.Factory
	DynamicCollection *dynamicconfig.Collection

	Lifecycle fx.Lifecycle

	HostName string `name:"hostname"`

	MembershipRings map[string]membership.SingleProvider
}

// FXService builds a new shard distributor service
func FXService(params ServiceParams) *Service {
	serviceConfig := config.NewConfig(params.DynamicCollection, params.HostName)

	logger := params.Logger.WithTags(tag.Service("shard-distributor"))

	dispatcher := params.RPCFactory.GetDispatcher()

	svc := &Service{
		config: serviceConfig,

		logger:        logger,
		metricsClient: params.MetricsClient,
		dispatcher:    dispatcher,
		matchingRing:  params.MembershipRings[service.Matching],
		historyRing:   params.MembershipRings[service.History],
	}

	rawHandler := handler.NewHandler(logger, params.MetricsClient, svc.matchingRing, svc.historyRing)
	svc.handler = metered.NewMetricsHandler(rawHandler, logger, params.MetricsClient)

	grpcHandler := grpc.NewGRPCHandler(svc.handler)
	grpcHandler.Register(svc.dispatcher)

	params.Lifecycle.Append(fx.StartStopHook(svc.Start, svc.Stop))
	return svc
}

// NewService is an adapter for legacy initialization without fx.
func NewService(
	params *resource.Params,
	factory resource.ResourceFactory,
) (*Service, error) {
	logger := params.Logger.WithTags(tag.Service("shard-distributor"))

	serviceConfig := config.NewConfig(
		dynamicconfig.NewCollection(
			params.DynamicConfig,
			logger,
			dynamicproperties.ClusterNameFilter(params.ClusterMetadata.GetCurrentClusterName()),
		),
		params.HostName,
	)

	serviceResource, err := factory.NewResource(
		params,
		service.ShardDistributor,
		&service.Config{
			PersistenceMaxQPS:        serviceConfig.PersistenceMaxQPS,
			PersistenceGlobalMaxQPS:  serviceConfig.PersistenceGlobalMaxQPS,
			ThrottledLoggerMaxRPS:    serviceConfig.ThrottledLogRPS,
			IsErrorRetryableFunction: common.IsServiceTransientError,
			// shard distributor doesn't need visibility config as it never read or write visibility
		},
	)
	if err != nil {
		return nil, err
	}

	matchingRing := params.HashRings[service.Matching]
	historyRing := params.HashRings[service.History]

	rawHandler := handler.NewHandler(logger, params.MetricsClient, matchingRing, historyRing)
	meteredHandler := metered.NewMetricsHandler(rawHandler, logger, params.MetricsClient)

	dispatcher := params.RPCFactory.GetDispatcher()

	grpcHandler := grpc.NewGRPCHandler(meteredHandler)
	grpcHandler.Register(dispatcher)

	return &Service{
		config:        serviceConfig,
		logger:        logger,
		metricsClient: params.MetricsClient,
		dispatcher:    dispatcher,
		handler:       meteredHandler,

		matchingRing: matchingRing,
		historyRing:  historyRing,

		// legacy components
		resource: serviceResource,
		stopC:    make(chan struct{}),
		status:   common.DaemonStatusInitialized,
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	s.logger.Info("starting")

	// legacy mode, if resouce is provided
	if s.resource != nil {
		s.resource.Start()
	}

	s.handler.Start()

	s.logger.Info("started")

	// legacy mode, fx Lifecyle requires component to start and return nil
	if s.stopC != nil {
		<-s.stopC
	}

	return
}

func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	// legacy mode, fx stops the component and ensures order/wait time between dependent components.
	if s.stopC != nil {
		close(s.stopC)
	}

	s.handler.Stop()
	if s.resource != nil {
		s.resource.Stop()
	}

	s.logger.Info("stopped")
}
