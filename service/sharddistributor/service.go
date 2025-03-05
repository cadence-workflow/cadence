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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/handler"
	"github.com/uber/cadence/service/sharddistributor/wrappers/grpc"
	"github.com/uber/cadence/service/sharddistributor/wrappers/metered"
)

// Service represents the shard distributor service
type Service struct {
	resource.Resource

	status           int32
	handler          handler.Handler
	stopC            chan struct{}
	config           *config.Config
	numHistoryShards int
	peerProvider     membership.PeerProvider

	matchingRing *membership.Ring
	historyRing  *membership.Ring
}

// NewService builds a new task manager service
func NewService(
	params *resource.Params,
	factory resource.ResourceFactory,
) (resource.Resource, error) {

	serviceConfig := config.NewConfig(
		dynamicconfig.NewCollection(
			params.DynamicConfig,
			params.Logger,
			dynamicconfig.ClusterNameFilter(params.ClusterMetadata.GetCurrentClusterName()),
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

	return &Service{
		Resource:         serviceResource,
		status:           common.DaemonStatusInitialized,
		config:           serviceConfig,
		stopC:            make(chan struct{}),
		numHistoryShards: params.PersistenceConfig.NumHistoryShards,

		matchingRing: matchingRing,
		historyRing:  historyRing,
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.GetLogger()
	logger.Info("shard distributor starting")

	rawHandler := handler.NewHandler(s.GetLogger(), s.GetMetricsClient(), s.matchingRing, s.historyRing)
	meteredHandler := metered.NewMetricsHandler(rawHandler, s.GetLogger(), s.GetMetricsClient())

	s.handler = meteredHandler

	grpcHandler := grpc.NewGRPCHandler(s.handler)
	grpcHandler.Register(s.GetDispatcher())

	s.Resource.Start()
	s.handler.Start()

	logger.Info("shard distributor started")

	<-s.stopC
}

func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	close(s.stopC)

	s.handler.Stop()
	s.Resource.Stop()

	s.GetLogger().Info("shard distributor stopped")
}
