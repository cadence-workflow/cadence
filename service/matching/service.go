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

package matching

import (
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/handler"
	"github.com/uber/cadence/service/matching/wrappers/grpc"
	"github.com/uber/cadence/service/matching/wrappers/thrift"
)

// Service represents the cadence-matching service
type Service struct {
	resource.Resource

	status  int32
	handler handler.Handler
	stopC   chan struct{}
	config  *config.Config
}

// NewService builds a new cadence-matching service
func NewService(
	params *resource.Params,
) (resource.Resource, error) {

	serviceConfig := config.NewConfig(
		dynamicconfig.NewCollection(
			params.DynamicConfig,
			params.Logger,
			dynamicconfig.ClusterNameFilter(params.ClusterMetadata.GetCurrentClusterName()),
		),
		params.HostName,
		params.GetIsolationGroups,
	)

	serviceResource, err := resource.New(
		params,
		service.Matching,
		&service.Config{
			PersistenceMaxQPS:        serviceConfig.PersistenceMaxQPS,
			PersistenceGlobalMaxQPS:  serviceConfig.PersistenceGlobalMaxQPS,
			ThrottledLoggerMaxRPS:    serviceConfig.ThrottledLogRPS,
			IsErrorRetryableFunction: common.IsServiceTransientError,
			// matching doesn't need visibility config as it never read or write visibility
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
	}, nil
}

// Start starts the service
func (s *Service) Start() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	logger := s.GetLogger()
	logger.Info("matching starting")

	engine := handler.NewEngine(
		s.GetTaskManager(),
		s.GetClusterMetadata(),
		s.GetHistoryClient(),
		s.GetMatchingRawClient(), // Use non retry client inside matching
		s.config,
		s.GetLogger(),
		s.GetMetricsClient(),
		s.GetDomainCache(),
		s.GetMembershipResolver(),
		s.GetIsolationGroupState(),
		s.GetTimeSource(),
	)

	s.handler = handler.NewHandler(engine, s.config, s.GetDomainCache(), s.GetMetricsClient(), s.GetLogger(), s.GetThrottledLogger())

	thriftHandler := thrift.NewThriftHandler(s.handler)
	thriftHandler.Register(s.GetDispatcher())

	grpcHandler := grpc.NewGRPCHandler(s.handler)
	grpcHandler.Register(s.GetDispatcher())

	// must start base service first
	s.Resource.Start()
	s.handler.Start()

	logger.Info("matching started")

	<-s.stopC
}

// Stop stops the service
func (s *Service) Stop() {
	if !atomic.CompareAndSwapInt32(&s.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	// remove self from membership ring and wait for traffic to drain
	s.GetLogger().Info("ShutdownHandler: Evicting self from membership ring")
	s.GetMembershipResolver().EvictSelf()
	s.GetLogger().Info("ShutdownHandler: Waiting for others to discover I am unhealthy")
	time.Sleep(s.config.ShutdownDrainDuration())

	close(s.stopC)

	s.handler.Stop()
	s.Resource.Stop()

	s.GetLogger().Info("matching stopped")
}
