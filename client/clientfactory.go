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

package client

import (
	"fmt"
	"time"

	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc/api/transport"

	"github.com/uber/cadence/.gen/go/admin/adminserviceclient"
	"github.com/uber/cadence/.gen/go/cadence/workflowserviceclient"
	"github.com/uber/cadence/.gen/go/history/historyserviceclient"
	"github.com/uber/cadence/.gen/go/matching/matchingserviceclient"
	historyv1 "github.com/uber/cadence/.gen/proto/history/v1"
	matchingv1 "github.com/uber/cadence/.gen/proto/matching/v1"
	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/client/wrappers/errorinjectors"
	"github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/client/wrappers/metered"
	"github.com/uber/cadence/client/wrappers/thrift"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
)

type (
	// Factory can be used to create RPC clients for cadence services
	Factory interface {
		NewHistoryClient() (history.Client, history.PeerResolver, error)
		NewMatchingClient(domainIDToName DomainIDToNameFunc) (matching.Client, error)

		NewHistoryClientWithTimeout(timeout time.Duration) (history.Client, history.PeerResolver, error)
		NewMatchingClientWithTimeout(domainIDToName DomainIDToNameFunc, timeout time.Duration, longPollTimeout time.Duration) (matching.Client, error)

		NewAdminClientWithTimeoutAndConfig(config transport.ClientConfig, timeout time.Duration, largeTimeout time.Duration) (admin.Client, error)
		NewFrontendClientWithTimeoutAndConfig(config transport.ClientConfig, timeout time.Duration, longPollTimeout time.Duration) (frontend.Client, error)

		NewShardDistributorClient() (sharddistributor.Client, error)
		NewShardDistributorClientWithTimeout(timeout time.Duration) (sharddistributor.Client, error)
	}

	// DomainIDToNameFunc maps a domainID to domain name. Returns error when mapping is not possible.
	DomainIDToNameFunc func(string) (string, error)

	rpcClientFactory struct {
		rpcFactory            rpc.Factory
		resolver              membership.Resolver
		metricsClient         metrics.Client
		dynConfig             *dynamicconfig.Collection
		numberOfHistoryShards int
		allIsolationGroups    func() []string
		logger                log.Logger
	}
)

// NewRPCClientFactory creates an instance of client factory that knows how to dispatch RPC calls.
func NewRPCClientFactory(
	rpcFactory rpc.Factory,
	resolver membership.Resolver,
	metricsClient metrics.Client,
	dc *dynamicconfig.Collection,
	numberOfHistoryShards int,
	allIsolationGroups func() []string,
	logger log.Logger,
) Factory {
	return &rpcClientFactory{
		rpcFactory:            rpcFactory,
		resolver:              resolver,
		metricsClient:         metricsClient,
		dynConfig:             dc,
		numberOfHistoryShards: numberOfHistoryShards,
		allIsolationGroups:    allIsolationGroups,
		logger:                logger,
	}
}

func (cf *rpcClientFactory) NewHistoryClient() (history.Client, history.PeerResolver, error) {
	return cf.NewHistoryClientWithTimeout(timeoutwrapper.HistoryDefaultTimeout)
}

func (cf *rpcClientFactory) NewMatchingClient(domainIDToName DomainIDToNameFunc) (matching.Client, error) {
	return cf.NewMatchingClientWithTimeout(domainIDToName, timeoutwrapper.MatchingDefaultTimeout, timeoutwrapper.MatchingDefaultLongPollTimeout)
}

func (cf *rpcClientFactory) NewHistoryClientWithTimeout(timeout time.Duration) (history.Client, history.PeerResolver, error) {
	var rawClient history.Client
	var namedPort = membership.PortTchannel

	outboundConfig := cf.rpcFactory.GetDispatcher().ClientConfig(service.History)
	if rpc.IsGRPCOutbound(outboundConfig) {
		rawClient = grpc.NewHistoryClient(historyv1.NewHistoryAPIYARPCClient(outboundConfig))
		namedPort = membership.PortGRPC
	} else {
		rawClient = thrift.NewHistoryClient(historyserviceclient.New(outboundConfig))
	}

	peerResolver := history.NewPeerResolver(cf.numberOfHistoryShards, cf.resolver, namedPort)

	client := history.NewClient(
		cf.numberOfHistoryShards,
		cf.rpcFactory.GetMaxMessageSize(),
		rawClient,
		peerResolver,
		cf.logger,
	)
	if errorRate := cf.dynConfig.GetFloat64Property(dynamicconfig.HistoryErrorInjectionRate)(); errorRate != 0 {
		client = errorinjectors.NewHistoryClient(client, errorRate, cf.logger)
	}
	if cf.metricsClient != nil {
		client = metered.NewHistoryClient(client, cf.metricsClient)
	}
	client = timeoutwrapper.NewHistoryClient(client, timeout)
	return client, peerResolver, nil
}

func (cf *rpcClientFactory) NewMatchingClientWithTimeout(
	domainIDToName DomainIDToNameFunc,
	timeout time.Duration,
	longPollTimeout time.Duration,
) (matching.Client, error) {
	var rawClient matching.Client
	var namedPort = membership.PortTchannel
	outboundConfig := cf.rpcFactory.GetDispatcher().ClientConfig(service.Matching)
	if rpc.IsGRPCOutbound(outboundConfig) {
		rawClient = grpc.NewMatchingClient(matchingv1.NewMatchingAPIYARPCClient(outboundConfig))
		namedPort = membership.PortGRPC
	} else {
		rawClient = thrift.NewMatchingClient(matchingserviceclient.New(outboundConfig))
	}

	peerResolver := matching.NewPeerResolver(cf.resolver, namedPort)

	partitionConfigProvider := matching.NewPartitionConfigProvider(cf.logger, cf.metricsClient, domainIDToName, cf.dynConfig)
	defaultLoadBalancer := matching.NewLoadBalancer(partitionConfigProvider)
	roundRobinLoadBalancer := matching.NewRoundRobinLoadBalancer(partitionConfigProvider)
	weightedLoadBalancer := matching.NewWeightedLoadBalancer(roundRobinLoadBalancer, partitionConfigProvider, cf.logger)
	igLoadBalancer := matching.NewIsolationLoadBalancer(weightedLoadBalancer, partitionConfigProvider, domainIDToName, cf.dynConfig)
	loadBalancers := map[string]matching.LoadBalancer{
		"random":      defaultLoadBalancer,
		"round-robin": roundRobinLoadBalancer,
		"weighted":    weightedLoadBalancer,
		"isolation":   igLoadBalancer,
	}
	client := matching.NewClient(
		rawClient,
		peerResolver,
		matching.NewMultiLoadBalancer(defaultLoadBalancer, loadBalancers, domainIDToName, cf.dynConfig, cf.logger),
		partitionConfigProvider,
	)
	client = timeoutwrapper.NewMatchingClient(client, longPollTimeout, timeout)
	if errorRate := cf.dynConfig.GetFloat64Property(dynamicconfig.MatchingErrorInjectionRate)(); errorRate != 0 {
		client = errorinjectors.NewMatchingClient(client, errorRate, cf.logger)
	}
	if cf.metricsClient != nil {
		client = metered.NewMatchingClient(client, cf.metricsClient)
	}
	return client, nil
}

func (cf *rpcClientFactory) NewAdminClientWithTimeoutAndConfig(
	config transport.ClientConfig,
	timeout time.Duration,
	largeTimeout time.Duration,
) (admin.Client, error) {
	var client admin.Client
	if rpc.IsGRPCOutbound(config) {
		client = grpc.NewAdminClient(adminv1.NewAdminAPIYARPCClient(config))
	} else {
		client = thrift.NewAdminClient(adminserviceclient.New(config))
	}

	client = timeoutwrapper.NewAdminClient(client, largeTimeout, timeout)
	if errorRate := cf.dynConfig.GetFloat64Property(dynamicconfig.AdminErrorInjectionRate)(); errorRate != 0 {
		client = errorinjectors.NewAdminClient(client, errorRate, cf.logger)
	}
	if cf.metricsClient != nil {
		client = metered.NewAdminClient(client, cf.metricsClient)
	}
	return client, nil
}

func (cf *rpcClientFactory) NewFrontendClientWithTimeoutAndConfig(
	config transport.ClientConfig,
	timeout time.Duration,
	longPollTimeout time.Duration,
) (frontend.Client, error) {
	var client frontend.Client
	if rpc.IsGRPCOutbound(config) {
		client = grpc.NewFrontendClient(
			apiv1.NewDomainAPIYARPCClient(config),
			apiv1.NewWorkflowAPIYARPCClient(config),
			apiv1.NewWorkerAPIYARPCClient(config),
			apiv1.NewVisibilityAPIYARPCClient(config),
		)
	} else {
		client = thrift.NewFrontendClient(workflowserviceclient.New(config))
	}

	client = timeoutwrapper.NewFrontendClient(client, longPollTimeout, timeout)
	if errorRate := cf.dynConfig.GetFloat64Property(dynamicconfig.FrontendErrorInjectionRate)(); errorRate != 0 {
		client = errorinjectors.NewFrontendClient(client, errorRate, cf.logger)
	}
	if cf.metricsClient != nil {
		client = metered.NewFrontendClient(client, cf.metricsClient)
	}
	return client, nil
}

func (cf *rpcClientFactory) NewShardDistributorClient() (sharddistributor.Client, error) {
	return cf.NewShardDistributorClientWithTimeout(timeoutwrapper.ShardDistributorDefaultTimeout)
}

func (cf *rpcClientFactory) NewShardDistributorClientWithTimeout(
	timeout time.Duration,
) (sharddistributor.Client, error) {
	outboundConfig, ok := cf.rpcFactory.GetDispatcher().OutboundConfig(service.ShardDistributor)
	// If no outbound config is found, it means the service is not enabled, we just return nil as we don't want to
	// break existing configs.
	if !ok {
		return nil, nil
	}

	if !rpc.IsGRPCOutbound(outboundConfig) {
		return nil, fmt.Errorf("shard distributor client does not support non-GRPC outbound")
	}

	client := grpc.NewShardDistributorClient(
		sharddistributorv1.NewShardDistributorAPIYARPCClient(outboundConfig),
	)

	client = timeoutwrapper.NewShardDistributorClient(client, timeout)
	if errorRate := cf.dynConfig.GetFloat64Property(dynamicconfig.ShardDistributorErrorInjectionRate)(); errorRate != 0 {
		client = errorinjectors.NewShardDistributorClient(client, errorRate, cf.logger)
	}
	if cf.metricsClient != nil {
		client = metered.NewShardDistributorClient(client, cf.metricsClient)
	}

	return client, nil
}
