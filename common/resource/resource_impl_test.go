// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package resource

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	persistenceClient "github.com/uber/cadence/common/persistence/client"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
)

func TestStartStop(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Panic: %v, stacktrace: %s", r, debug.Stack())
		}
	}()

	ctrl := gomock.NewController(t)
	serviceName := "test-service"
	hostName := "test-host"
	metricsCl := metrics.NewNoopMetricsClient()
	logger := testlogger.New(t)
	dc := dynamicconfig.NewInMemoryClient()

	// membership resolver mocks
	memberRes := membership.NewMockResolver(ctrl)
	memberRes.EXPECT().MemberCount(gomock.Any()).Return(4, nil).AnyTimes()
	memberRes.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	memberRes.EXPECT().Unsubscribe(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	memberRes.EXPECT().Start().AnyTimes()
	memberRes.EXPECT().Stop().AnyTimes()
	selfHostInfo := membership.NewHostInfo("localhost:0")
	memberRes.EXPECT().WhoAmI().Return(selfHostInfo, nil).AnyTimes()

	// pprof mocks
	pprof := common.NewMockPProfInitializer(ctrl)
	pprof.EXPECT().Start().Return(nil).Times(1)

	// rpc mocks
	clusterMetadata := cluster.NewMetadata(1, "primary-cluster", "primary-cluster", map[string]config.ClusterInformation{
		"primary-cluster":   {InitialFailoverVersion: 1, Enabled: true, RPCTransport: "tchannel", RPCAddress: "localhost:0"},
		"secondary-cluster": {InitialFailoverVersion: 1, Enabled: true, RPCTransport: "tchannel", RPCAddress: "localhost:0"},
	}, nil, metricsCl, logger)
	directOutboundPCF := rpc.NewDirectPeerChooserFactory(serviceName, logger, metricsCl)
	directConnRetainFn := func(opts ...dynamicconfig.FilterOption) bool { return false }
	pcf := rpc.NewMockPeerChooserFactory(ctrl)
	peerChooser := rpc.NewMockPeerChooser(ctrl)
	peerChooser.EXPECT().Start().Return(nil).AnyTimes()
	peerChooser.EXPECT().Stop().Return(nil).AnyTimes()
	pcf.EXPECT().CreatePeerChooser(gomock.Any(), gomock.Any()).Return(peerChooser, nil).AnyTimes()
	ob := rpc.CombineOutbounds(
		rpc.NewCrossDCOutbounds(clusterMetadata.GetAllClusterInfo(), pcf),
		rpc.NewDirectOutboundBuilder(service.History, true, nil, directOutboundPCF, directConnRetainFn),
		rpc.NewDirectOutboundBuilder(service.Matching, true, nil, directOutboundPCF, directConnRetainFn),
	)
	rpcFac := rpc.NewFactory(logger, rpc.Params{
		ServiceName:      serviceName,
		TChannelAddress:  "localhost:0",
		GRPCAddress:      "localhost:0",
		OutboundsBuilder: ob,
	})

	// persistence mocks
	persistenceClientBean := persistenceClient.NewMockBean(ctrl)
	domainMgr := persistence.NewMockDomainManager(ctrl)
	domainMgr.EXPECT().GetMetadata(gomock.Any()).Return(&persistence.GetMetadataResponse{
		NotificationVersion: 2,
	}, nil).AnyTimes()
	domain := &persistence.GetDomainResponse{
		Info: &persistence.DomainInfo{
			ID:   "test-domain-id",
			Name: "test-domain-name",
			Data: map[string]string{"k1": "v1"},
		},
		Config:              &persistence.DomainConfig{},
		ReplicationConfig:   &persistence.DomainReplicationConfig{},
		NotificationVersion: 1, // should be less than notification version for this domain to be loaded by cache.
	}
	domainMgr.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(&persistence.ListDomainsResponse{
		Domains: []*persistence.GetDomainResponse{domain},
	}, nil).AnyTimes()
	domainMgr.EXPECT().GetDomain(gomock.Any(), gomock.Any()).Return(domain, nil).AnyTimes()
	domainReplMgr := persistence.NewMockQueueManager(ctrl)
	historyMgr := persistence.NewMockHistoryManager(ctrl)
	taskMgr := persistence.NewMockTaskManager(ctrl)
	visMgr := persistence.NewMockVisibilityManager(ctrl)
	shardMgr := persistence.NewMockShardManager(ctrl)
	execMgr := persistence.NewMockExecutionManager(ctrl)
	persistenceClientBean.EXPECT().GetDomainManager().Return(domainMgr).AnyTimes()
	persistenceClientBean.EXPECT().GetTaskManager().Return(taskMgr).AnyTimes()
	persistenceClientBean.EXPECT().GetVisibilityManager().Return(visMgr).AnyTimes()
	persistenceClientBean.EXPECT().GetShardManager().Return(shardMgr).AnyTimes()
	persistenceClientBean.EXPECT().GetExecutionManager(gomock.Any()).Return(execMgr, nil).AnyTimes()
	persistenceClientBean.EXPECT().GetDomainReplicationQueueManager().Return(domainReplMgr).AnyTimes()
	persistenceClientBean.EXPECT().GetHistoryManager().Return(historyMgr).AnyTimes()
	persistenceClientBean.EXPECT().Close().Times(1)

	// archiver provider mocks
	archiveProvider := &provider.MockArchiverProvider{}
	archiveProvider.On("RegisterBootstrapContainer", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)

	// params
	params := &Params{
		Name:     serviceName,
		HostName: hostName,
		PersistenceConfig: config.Persistence{
			NumHistoryShards: 1024,
			DefaultStore:     "nosql-store",
			DataStores: map[string]config.DataStore{
				"nosql-store": {
					NoSQL: &config.NoSQL{},
				},
			},
		},
		ClusterMetadata:    clusterMetadata,
		Logger:             logger,
		MetricScope:        tally.NoopScope,
		MetricsClient:      metricsCl,
		RPCFactory:         rpcFac,
		MembershipResolver: memberRes,
		DynamicConfig:      dc,
		TimeSource:         clock.NewRealTimeSource(),
		PProfInitializer:   pprof,
		NewPersistenceBeanFn: func(persistenceClient.Factory, *persistenceClient.Params, *service.Config) (persistenceClient.Bean, error) {
			return persistenceClientBean, nil
		},
		ArchiverProvider:           archiveProvider,
		AsyncWorkflowQueueProvider: queue.NewMockProvider(ctrl),
	}

	// bare minimum service config
	svcCfg := &service.Config{
		ThrottledLoggerMaxRPS: func(opts ...dynamicconfig.FilterOption) int {
			return 100
		},
		PersistenceGlobalMaxQPS: func(opts ...dynamicconfig.FilterOption) int {
			return 100
		},
		PersistenceMaxQPS: func(opts ...dynamicconfig.FilterOption) int {
			return 100
		},
	}

	i, err := New(params, serviceName, svcCfg)
	if err != nil {
		t.Fatal(err)
	}

	i.Start()
	i.Start() // should be no-op
	defer func() {
		i.Stop()
		i.Stop() // should be no-op
		goleak.VerifyNone(t)
	}()

	time.Sleep(100 * time.Millisecond) // wait for go routines to start

	// validations
	assert.Equal(t, serviceName, i.GetServiceName())
	assert.Equal(t, selfHostInfo, i.GetHostInfo())
	assert.Equal(t, params.ClusterMetadata, i.GetClusterMetadata())
	gotDomain, err := i.GetDomainCache().GetDomainByID("test-domain-id")
	assert.NoError(t, err)
	assert.Equal(t, domain.Info, gotDomain.GetInfo())
	assert.NotNil(t, i.GetDomainMetricsScopeCache())
	assert.NotNil(t, i.GetTimeSource())
	assert.NotNil(t, i.GetPayloadSerializer())
	assert.Equal(t, metricsCl, i.GetMetricsClient())
	assert.Equal(t, params.MessagingClient, i.GetMessagingClient())
	assert.Equal(t, params.BlobstoreClient, i.GetBlobstoreClient())
	assert.Equal(t, params.ArchivalMetadata, i.GetArchivalMetadata())
	assert.Equal(t, archiveProvider, i.GetArchiverProvider())
	assert.NotNil(t, i.GetDomainReplicationQueue())
	assert.Equal(t, memberRes, i.GetMembershipResolver())
	assert.Equal(t, params.PublicClient, i.GetSDKClient())
	assert.NotNil(t, i.GetFrontendRawClient())
	assert.NotNil(t, i.GetFrontendClient())
	assert.NotNil(t, i.GetMatchingRawClient())
	assert.NotNil(t, i.GetMatchingClient())
	assert.NotNil(t, i.GetHistoryRawClient())
	assert.NotNil(t, i.GetHistoryClient())
	assert.NotNil(t, i.GetRatelimiterAggregatorsClient())
	assert.NotNil(t, i.GetRemoteAdminClient("secondary-cluster"))
	assert.NotNil(t, i.GetRemoteFrontendClient("secondary-cluster"))
	assert.NotNil(t, i.GetClientBean())
	assert.Equal(t, domainMgr, i.GetDomainManager())
	assert.Equal(t, taskMgr, i.GetTaskManager())
	assert.Equal(t, visMgr, i.GetVisibilityManager())
	assert.Equal(t, shardMgr, i.GetShardManager())
	assert.Equal(t, historyMgr, i.GetHistoryManager())
	em, err := i.GetExecutionManager(3)
	assert.NoError(t, err)
	assert.Equal(t, execMgr, em)
	assert.Equal(t, persistenceClientBean, i.GetPersistenceBean())
	assert.Equal(t, hostName, i.GetHostName())
	assert.NotNil(t, i.GetLogger())
	assert.NotNil(t, i.GetThrottledLogger())
	assert.NotNil(t, i.GetDispatcher())
	assert.NotNil(t, i.GetIsolationGroupState())
	assert.Nil(t, i.GetIsolationGroupStore())
	assert.Equal(t, params.AsyncWorkflowQueueProvider, i.GetAsyncWorkflowQueueProvider())
}
