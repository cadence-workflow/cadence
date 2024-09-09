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

package cadence

import (
	"log"
	"time"

	"github.com/startreedata/pinot-client-go/pinot"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/compatibility"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/blobstore/filestore"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/configstore"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log/loggerimpl"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/peerprovider/ringpopprovider"
	"github.com/uber/cadence/common/persistence"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/frontend"
	"github.com/uber/cadence/service/history"
	"github.com/uber/cadence/service/matching"
	"github.com/uber/cadence/service/worker"
)

type (
	server struct {
		name   string
		cfg    *config.Config
		doneC  chan struct{}
		daemon common.Daemon
	}
)

// newServer returns a new instance of a daemon
// that represents a cadence service
func newServer(service string, cfg *config.Config) common.Daemon {
	return &server{
		cfg:   cfg,
		name:  service,
		doneC: make(chan struct{}),
	}
}

// Start starts the server
func (s *server) Start() {
	s.daemon = s.startService()
}

// Stop stops the server
func (s *server) Stop() {

	if s.daemon == nil {
		return
	}

	select {
	case <-s.doneC:
	default:
		s.daemon.Stop()
		select {
		case <-s.doneC:
		case <-time.After(time.Minute):
			log.Printf("timed out waiting for server %v to exit\n", s.name)
		}
	}
}

// startService starts a service with the given name and config
func (s *server) startService() common.Daemon {
	svcCfg, err := s.cfg.GetServiceConfig(s.name)
	if err != nil {
		log.Fatal(err.Error())
	}

	params := resource.Params{}
	params.Name = service.FullName(s.name)

	zapLogger, err := s.cfg.Log.NewZapLogger()
	if err != nil {
		log.Fatal("failed to create the zap logger, err: ", err.Error())
	}
	params.Logger = loggerimpl.NewLogger(zapLogger).WithTags(tag.Service(params.Name))

	params.PersistenceConfig = s.cfg.Persistence

	err = nil
	if s.cfg.DynamicConfig.Client == "" {
		params.Logger.Warn("falling back to legacy file based dynamicClientConfig")
		params.DynamicConfig, err = dynamicconfig.NewFileBasedClient(&s.cfg.DynamicConfigClient, params.Logger, s.doneC)
	} else {
		switch s.cfg.DynamicConfig.Client {
		case dynamicconfig.ConfigStoreClient:
			params.Logger.Info("initialising ConfigStore dynamic config client")
			params.DynamicConfig, err = configstore.NewConfigStoreClient(
				&s.cfg.DynamicConfig.ConfigStore,
				&s.cfg.Persistence,
				params.Logger,
				persistence.DynamicConfig,
			)
		case dynamicconfig.FileBasedClient:
			params.Logger.Info("initialising File Based dynamic config client")
			params.DynamicConfig, err = dynamicconfig.NewFileBasedClient(&s.cfg.DynamicConfig.FileBased, params.Logger, s.doneC)
		default:
			params.Logger.Info("initialising NOP dynamic config client")
			params.DynamicConfig = dynamicconfig.NewNopClient()
		}
	}

	if err != nil {
		params.Logger.Error("creating dynamic config client failed, using no-op config client instead", tag.Error(err))
		params.DynamicConfig = dynamicconfig.NewNopClient()
	}

	clusterGroupMetadata := s.cfg.ClusterGroupMetadata
	dc := dynamicconfig.NewCollection(
		params.DynamicConfig,
		params.Logger,
		dynamicconfig.ClusterNameFilter(clusterGroupMetadata.CurrentClusterName),
	)

	params.MetricScope = svcCfg.Metrics.NewScope(params.Logger, params.Name)

	rpcParams, err := rpc.NewParams(params.Name, s.cfg, dc)
	if err != nil {
		log.Fatalf("error creating rpc factory params: %v", err)
	}
	rpcParams.OutboundsBuilder = rpc.CombineOutbounds(
		rpcParams.OutboundsBuilder,
		rpc.NewCrossDCOutbounds(clusterGroupMetadata.ClusterGroup, rpc.NewDNSPeerChooserFactory(s.cfg.PublicClient.RefreshInterval, params.Logger)),
	)
	rpcFactory := rpc.NewFactory(params.Logger, rpcParams)
	params.RPCFactory = rpcFactory

	peerProvider, err := ringpopprovider.New(
		params.Name,
		&s.cfg.Ringpop,
		rpcFactory.GetChannel(),
		membership.PortMap{
			membership.PortGRPC:     svcCfg.RPC.GRPCPort,
			membership.PortTchannel: svcCfg.RPC.Port,
		},
		params.Logger,
	)

	if err != nil {
		log.Fatalf("ringpop provider failed: %v", err)
	}

	params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, params.Logger))

	params.MembershipResolver, err = membership.NewResolver(
		peerProvider,
		params.Logger,
		params.MetricsClient,
	)
	if err != nil {
		log.Fatalf("error creating membership monitor: %v", err)
	}
	params.PProfInitializer = svcCfg.PProf.NewInitializer(params.Logger)

	params.ClusterRedirectionPolicy = s.cfg.ClusterGroupMetadata.ClusterRedirectionPolicy

	params.ClusterMetadata = cluster.NewMetadata(
		clusterGroupMetadata.FailoverVersionIncrement,
		clusterGroupMetadata.PrimaryClusterName,
		clusterGroupMetadata.CurrentClusterName,
		clusterGroupMetadata.ClusterGroup,
		dc.GetBoolPropertyFilteredByDomain(dynamicconfig.UseNewInitialFailoverVersion),
		params.MetricsClient,
		params.Logger,
	)

	advancedVisMode := dc.GetStringProperty(
		dynamicconfig.AdvancedVisibilityWritingMode,
	)()
	isAdvancedVisEnabled := common.IsAdvancedVisibilityWritingEnabled(advancedVisMode, params.PersistenceConfig.IsAdvancedVisibilityConfigExist())
	if isAdvancedVisEnabled {
		params.MessagingClient = kafka.NewKafkaClient(&s.cfg.Kafka, params.MetricsClient, params.Logger, params.MetricScope, isAdvancedVisEnabled)
	} else {
		params.MessagingClient = nil
	}

	if isAdvancedVisEnabled {
		// verify config of advanced visibility store
		advancedVisStoreKey := s.cfg.Persistence.AdvancedVisibilityStore
		advancedVisStore, ok := s.cfg.Persistence.DataStores[advancedVisStoreKey]
		if !ok {
			log.Fatalf("not able to find advanced visibility store in config: %v", advancedVisStoreKey)
		}

		// there are 3 circumstances:
		// 1. advanced visibility store == elasticsearch, use ESClient and visibilityDualManager
		// 2. advanced visibility store == pinot and in process of migration, use ESClient, PinotClient and and visibilityTripleManager
		// 3. advanced visibility store == pinot and not migrating, use PinotClient and visibilityDualManager
		if params.PersistenceConfig.AdvancedVisibilityStore == common.PinotVisibilityStoreName {
			if advancedVisStore.Pinot.Migration.Enabled {
				esVisibilityStore, ok := s.cfg.Persistence.DataStores[common.ESVisibilityStoreName]
				if !ok {
					log.Fatalf("not able to find elasticsearch visibility store in config")
				}
				params.ESConfig = esVisibilityStore.ElasticSearch
				params.ESConfig.SetUsernamePassword()
				esClient, err := elasticsearch.NewGenericClient(params.ESConfig, params.Logger)
				if err != nil {
					log.Fatalf("error creating elastic search client: %v", err)
				}
				params.ESClient = esClient

				// verify index name
				indexName, ok := params.ESConfig.Indices[common.VisibilityAppName]
				if !ok || len(indexName) == 0 {
					log.Fatalf("elastic search config missing visibility index")
				}
			} else {
				params.ESClient = nil
			}
			params.PinotConfig = advancedVisStore.Pinot
			pinotBroker := params.PinotConfig.Broker
			pinotRawClient, err := pinot.NewFromBrokerList([]string{pinotBroker})
			if err != nil || pinotRawClient == nil {
				log.Fatalf("Creating Pinot visibility client failed: %v", err)
			}
			pinotClient := pnt.NewPinotClient(pinotRawClient, params.Logger, params.PinotConfig)
			params.PinotClient = pinotClient
		} else {
			params.ESConfig = advancedVisStore.ElasticSearch
			params.ESConfig.SetUsernamePassword()
			esClient, err := elasticsearch.NewGenericClient(params.ESConfig, params.Logger)
			if err != nil {
				log.Fatalf("error creating elastic search client: %v", err)
			}
			params.ESClient = esClient

			// verify index name
			indexName, ok := params.ESConfig.Indices[common.VisibilityAppName]
			if !ok || len(indexName) == 0 {
				log.Fatalf("elastic search config missing visibility index")
			}
		}
	}

	publicClientConfig := params.RPCFactory.GetDispatcher().ClientConfig(rpc.OutboundPublicClient)
	if rpc.IsGRPCOutbound(publicClientConfig) {
		params.PublicClient = compatibility.NewThrift2ProtoAdapter(
			apiv1.NewDomainAPIYARPCClient(publicClientConfig),
			apiv1.NewWorkflowAPIYARPCClient(publicClientConfig),
			apiv1.NewWorkerAPIYARPCClient(publicClientConfig),
			apiv1.NewVisibilityAPIYARPCClient(publicClientConfig),
		)
	} else {
		params.PublicClient = workflowserviceclient.New(publicClientConfig)
	}

	params.ArchivalMetadata = archiver.NewArchivalMetadata(
		dc,
		s.cfg.Archival.History.Status,
		s.cfg.Archival.History.EnableRead,
		s.cfg.Archival.Visibility.Status,
		s.cfg.Archival.Visibility.EnableRead,
		&s.cfg.DomainDefaults.Archival,
	)

	params.ArchiverProvider = provider.NewArchiverProvider(
		s.cfg.Archival.History.Provider,
		s.cfg.Archival.Visibility.Provider,
		s.cfg.Archival.Execution.Provider,
	)
	params.PersistenceConfig.TransactionSizeLimit = dc.GetIntProperty(dynamicconfig.TransactionSizeLimit)
	params.PersistenceConfig.ErrorInjectionRate = dc.GetFloat64Property(dynamicconfig.PersistenceErrorInjectionRate)
	params.AuthorizationConfig = s.cfg.Authorization
	params.BlobstoreClient, err = filestore.NewFilestoreClient(s.cfg.Blobstore.Filestore)
	if err != nil {
		log.Printf("failed to create file blobstore client, will continue startup without it: %v", err)
		params.BlobstoreClient = nil
	}

	params.AsyncWorkflowQueueProvider, err = queue.NewAsyncQueueProvider(s.cfg.AsyncWorkflowQueues)
	if err != nil {
		log.Fatalf("error creating async queue provider: %v", err)
	}

	params.Logger.Info("Starting service " + s.name)

	var daemon common.Daemon

	switch params.Name {
	case service.Frontend:
		daemon, err = frontend.NewService(&params)
	case service.History:
		daemon, err = history.NewService(&params)
	case service.Matching:
		daemon, err = matching.NewService(&params)
	case service.Worker:
		daemon, err = worker.NewService(&params)
	}
	if err != nil {
		params.Logger.Fatal("Fail to start "+s.name+" service ", tag.Error(err))
	}

	go execute(daemon, s.doneC)

	return daemon
}

// execute runs the daemon in a separate go routine
func execute(d common.Daemon, doneC chan struct{}) {
	d.Start()
	close(doneC)
}
