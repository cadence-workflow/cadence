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
	"context"
	"fmt"
	"time"

	"github.com/startreedata/pinot-client-go/pinot"
	"github.com/uber-go/tally"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/compatibility"

	sharddistributorv1 "github.com/uber/cadence/.gen/proto/sharddistributor/v1"
	sharddistributorClient "github.com/uber/cadence/client/sharddistributor"
	"github.com/uber/cadence/client/wrappers/errorinjectors"
	"github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/client/wrappers/metered"
	"github.com/uber/cadence/client/wrappers/retryable"
	timeoutwrapper "github.com/uber/cadence/client/wrappers/timeout"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/blobstore/filestore"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/isolationgroup/isolationgroupapi"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/peerprovider/ringpopprovider"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/frontend"
	"github.com/uber/cadence/service/history"
	"github.com/uber/cadence/service/matching"
	"github.com/uber/cadence/service/sharddistributor/client/spectatorclient"
	"github.com/uber/cadence/service/worker"
	diagnosticsInvariant "github.com/uber/cadence/service/worker/diagnostics/invariant"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/failure"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/retry"
	"github.com/uber/cadence/service/worker/diagnostics/invariant/timeout"
)

type (
	server struct {
		name             string
		cfg              config.Config
		logger           log.Logger
		doneC            chan struct{}
		daemon           common.Daemon
		dynamicCfgClient dynamicconfig.Client
		scope            tally.Scope
		metricsClient    metrics.Client
	}
)

// newServer returns a new instance of a daemon
// that represents a cadence service
func newServer(service string, cfg config.Config, logger log.Logger, dynamicCfgClient dynamicconfig.Client, scope tally.Scope, metricsClient metrics.Client) common.Daemon {
	return &server{
		cfg:              cfg,
		name:             service,
		doneC:            make(chan struct{}),
		logger:           logger,
		dynamicCfgClient: dynamicCfgClient,
		scope:            scope,
		metricsClient:    metricsClient,
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
			s.logger.Warn("timed out waiting for server to exit")
		}
	}
}

// startService starts a service with the given name and config
func (s *server) startService() common.Daemon {
	svcCfg, err := s.cfg.GetServiceConfig(s.name)
	if err != nil {
		s.logger.Fatal(err.Error())
	}

	params := resource.Params{
		Name:              service.FullName(s.name),
		Logger:            s.logger.WithTags(tag.Service(service.FullName(s.name))),
		PersistenceConfig: s.cfg.Persistence,
		DynamicConfig:     s.dynamicCfgClient,
		RPCConfig:         svcCfg.RPC,
	}

	clusterGroupMetadata := s.cfg.ClusterGroupMetadata
	dc := dynamicconfig.NewCollection(
		params.DynamicConfig,
		params.Logger,
		dynamicproperties.ClusterNameFilter(clusterGroupMetadata.CurrentClusterName),
	)

	params.MetricScope = s.scope
	params.MetricsClient = s.metricsClient

	rpcParams, err := rpc.NewParams(params.Name, &s.cfg, dc, params.Logger, params.MetricsClient)
	if err != nil {
		s.logger.Fatal("error creating rpc factory params", tag.Error(err))
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
		rpcFactory.GetTChannel(),
		membership.PortMap{
			membership.PortGRPC:     svcCfg.RPC.GRPCPort,
			membership.PortTchannel: svcCfg.RPC.Port,
		},
		params.Logger,
	)

	if err != nil {
		s.logger.Fatal("ringpop provider failed", tag.Error(err))
	}

	shardDistributorClient := s.createShardDistributorClient(params, dc)
	var spectator spectatorclient.Spectator
	if shardDistributorClient != nil && len(s.cfg.ShardDistributorMatchingConfig.Namespaces) > 0 {
		if len(s.cfg.ShardDistributorMatchingConfig.Namespaces) > 1 {
			s.logger.Fatal("spectator does not support multiple namespaces", tag.Value(s.cfg.ShardDistributorMatchingConfig.Namespaces))
		}
		spectatorParams := spectatorclient.Params{
			Client:       shardDistributorClient,
			MetricsScope: params.MetricScope,
			Logger:       params.Logger,
			Config:       s.cfg.ShardDistributorMatchingConfig,
			TimeSource:   clock.NewRealTimeSource(),
		}
		namespace := s.cfg.ShardDistributorMatchingConfig.Namespaces[0].Namespace
		spectator, err = spectatorclient.NewSpectatorWithNamespace(
			spectatorParams,
			namespace,
		)
		if err != nil {
			s.logger.Fatal("error creating spectator", tag.Error(err))
		}

		// Start the spectator to begin watching namespace state
		if err := spectator.Start(context.Background()); err != nil {
			s.logger.Fatal("error starting spectator", tag.Error(err))
		}
	} else {
		s.logger.Warn("Shard distributor client not configured, spectator will not be started")
	}

	params.HashRings = make(map[string]membership.SingleProvider)
	for _, s := range service.ListWithRing {
		params.HashRings[s] = membership.NewHashring(s, peerProvider, clock.NewRealTimeSource(), params.Logger, params.MetricsClient.Scope(metrics.HashringScope))
	}

	wrappedRings := s.wrapHashRingsWithShardDistributor(params.HashRings, spectator, dc, params.Logger)

	params.MembershipResolver, err = membership.NewResolver(
		peerProvider,
		params.MetricsClient,
		params.Logger,
		wrappedRings,
	)

	if err != nil {
		s.logger.Fatal("error creating membership monitor", tag.Error(err))
	}
	params.PProfInitializer = svcCfg.PProf.NewInitializer(params.Logger)

	params.ClusterRedirectionPolicy = s.cfg.ClusterGroupMetadata.ClusterRedirectionPolicy

	params.GetIsolationGroups = getFromDynamicConfig(params, dc)

	params.ClusterMetadata = cluster.NewMetadata(
		*clusterGroupMetadata,
		dc.GetBoolPropertyFilteredByDomain(dynamicproperties.UseNewInitialFailoverVersion),
		params.MetricsClient,
		params.Logger,
	)

	advancedVisMode := dc.GetStringProperty(
		dynamicproperties.WriteVisibilityStoreName,
	)()
	isAdvancedVisEnabled := common.IsAdvancedVisibilityWritingEnabled(advancedVisMode, params.PersistenceConfig.IsAdvancedVisibilityConfigExist())
	if isAdvancedVisEnabled {
		params.MessagingClient = kafka.NewKafkaClient(&s.cfg.Kafka, params.MetricsClient, params.Logger, params.MetricScope, isAdvancedVisEnabled)
	} else {
		params.MessagingClient = nil
	}

	if isAdvancedVisEnabled {
		s.setupVisibilityClients(&params)
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

	params.ArchiverProvider = provider.NewArchiverProvider(s.cfg.Archival.History.Provider, s.cfg.Archival.Visibility.Provider)
	params.PersistenceConfig.TransactionSizeLimit = dc.GetIntProperty(dynamicproperties.TransactionSizeLimit)
	params.PersistenceConfig.ErrorInjectionRate = dc.GetFloat64Property(dynamicproperties.PersistenceErrorInjectionRate)
	params.AuthorizationConfig = s.cfg.Authorization
	params.BlobstoreClient, err = filestore.NewFilestoreClient(s.cfg.Blobstore.Filestore)
	if err != nil {
		s.logger.Warn("failed to create file blobstore client, will continue startup without it: %v", tag.Error(err))
		params.BlobstoreClient = nil
	}

	params.AsyncWorkflowQueueProvider, err = queue.NewAsyncQueueProvider(s.cfg.AsyncWorkflowQueues)
	if err != nil {
		s.logger.Fatal("error creating async queue provider", tag.Error(err))
	}

	params.KafkaConfig = s.cfg.Kafka
	params.DiagnosticsInvariants = []diagnosticsInvariant.Invariant{timeout.NewInvariant(timeout.Params{Client: params.PublicClient}), failure.NewInvariant(), retry.NewInvariant()}
	params.ShardDistributorMatchingConfig = s.cfg.ShardDistributorMatchingConfig

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
	default:
		params.Logger.Fatal("unknown service", tag.Service(params.Name))
	}
	if err != nil {
		params.Logger.Fatal("Fail to start "+s.name+" service ", tag.Error(err))
	}

	go execute(daemon, s.doneC)

	return daemon
}

func (*server) wrapHashRingsWithShardDistributor(
	hashRings map[string]membership.SingleProvider,
	spectator spectatorclient.Spectator,
	dc *dynamicconfig.Collection,
	logger log.Logger,
) map[string]membership.SingleProvider {
	if _, ok := hashRings[service.Matching]; ok {
		hashRings[service.Matching] = membership.NewShardDistributorResolver(
			spectator,
			dc.GetStringProperty(dynamicproperties.MatchingShardDistributionMode),
			hashRings[service.Matching],
			logger,
		)
	}
	return hashRings
}

func (*server) createShardDistributorClient(
	params resource.Params,
	dc *dynamicconfig.Collection,
) sharddistributorClient.Client {
	shardDistributorClientConfig, ok := params.RPCFactory.GetDispatcher().OutboundConfig(service.ShardDistributor)
	var shardDistributorClient sharddistributorClient.Client
	if ok {
		if !rpc.IsGRPCOutbound(shardDistributorClientConfig) {
			params.Logger.Error("shard distributor client does not support non-GRPC outbound will fail back to hashring")
			return nil
		}
		if shardDistributorClientConfig.Outbounds.Stream == nil {
			params.Logger.Error("shard distributor client does not support stream outbound will fail back to hashring")
			return nil
		}

		shardDistributorClient = grpc.NewShardDistributorClient(
			sharddistributorv1.NewShardDistributorAPIYARPCClient(shardDistributorClientConfig),
		)

		shardDistributorClient = timeoutwrapper.NewShardDistributorClient(shardDistributorClient, timeoutwrapper.ShardDistributorDefaultTimeout)
		shardDistributorClient = retryable.NewShardDistributorClient(
			shardDistributorClient,
			common.CreateShardDistributorServiceRetryPolicy(),
			common.IsServiceTransientError,
		)
		if errorRate := dc.GetFloat64Property(dynamicproperties.ShardDistributorErrorInjectionRate)(); errorRate != 0 {
			shardDistributorClient = errorinjectors.NewShardDistributorClient(shardDistributorClient, errorRate, params.Logger)
		}
		if params.MetricsClient != nil {
			shardDistributorClient = metered.NewShardDistributorClient(shardDistributorClient, params.MetricsClient)
		}
	}
	return shardDistributorClient
}

// execute runs the daemon in a separate go routine
func execute(d common.Daemon, doneC chan struct{}) {
	d.Start()
	close(doneC)
}

// there are multiple circumstances:
// 1. advanced visibility store == elasticsearch, use ESClient and visibilityDualManager
// 2. advanced visibility store == pinot and in process of migration, use ESClient, PinotClient and and visibilityTripleManager
// 3. advanced visibility store == pinot and not migrating, use PinotClient and visibilityDualManager
// 4. advanced visibility store == opensearch and not migrating, this performs the same as 1, just use different version ES client and visibilityDualManager
// 5. advanced visibility store == opensearch and in process of migration, use ESClient and visibilityTripleManager
func (s *server) setupVisibilityClients(params *resource.Params) {
	advancedVisStoreKey := s.cfg.Persistence.AdvancedVisibilityStore
	advancedVisStore, ok := s.cfg.Persistence.DataStores[advancedVisStoreKey]
	if !ok {
		s.logger.Fatal("Cannot find advanced visibility store in config", tag.Value(advancedVisStoreKey))
	}

	// Handle advanced visibility store based on type and migration state
	switch advancedVisStoreKey {
	case constants.PinotVisibilityStoreName:
		s.setupPinotClient(params, advancedVisStore)
	case constants.OSVisibilityStoreName:
		s.setupOSClient(params, advancedVisStore)
	default: // Assume Elasticsearch by default
		s.setupESClient(params)
	}
}

func (s *server) setupPinotClient(params *resource.Params, advancedVisStore config.DataStore) {
	params.PinotConfig = advancedVisStore.Pinot
	pinotBroker := params.PinotConfig.Broker
	pinotRawClient, err := pinot.NewFromBrokerList([]string{pinotBroker})
	if err != nil || pinotRawClient == nil {
		s.logger.Fatal("Creating Pinot visibility client failed", tag.Error(err))
	}
	params.PinotClient = pnt.NewPinotClient(pinotRawClient, params.Logger, params.PinotConfig)
	if advancedVisStore.Pinot.Migration.Enabled {
		s.setupESClient(params)
	}
}

func (s *server) setupESClient(params *resource.Params) {
	esVisibilityStore, ok := s.cfg.Persistence.DataStores[constants.ESVisibilityStoreName]
	if !ok {
		s.logger.Fatal("Cannot find Elasticsearch visibility store in config")
	}

	params.ESConfig = esVisibilityStore.ElasticSearch
	params.ESConfig.SetUsernamePassword()

	esClient, err := elasticsearch.NewGenericClient(params.ESConfig, params.Logger)
	if err != nil {
		s.logger.Fatal("Error creating Elasticsearch client", tag.Error(err))
	}
	params.ESClient = esClient

	err = validateIndex(params.ESConfig)
	if err != nil {
		s.logger.Fatal("Error creating OpenSearch client", tag.Error(err))
	}
}

func (s *server) setupOSClient(params *resource.Params, advancedVisStore config.DataStore) {
	// OpenSearch client setup (same structure as Elasticsearch, just version difference)
	// This is only for migration purposes
	params.OSConfig = advancedVisStore.ElasticSearch
	params.OSConfig.SetUsernamePassword()

	osClient, err := elasticsearch.NewGenericClient(params.OSConfig, params.Logger)
	if err != nil {
		s.logger.Fatal("Error creating OpenSearch client", tag.Error(err))
	}
	params.OSClient = osClient

	err = validateIndex(params.OSConfig)
	if err != nil {
		s.logger.Fatal("Error creating OpenSearch client", tag.Error(err))
	}

	if advancedVisStore.ElasticSearch.Migration.Enabled {
		s.setupESClient(params)
	} else {
		// to avoid code duplication, we will use es-visibility and set the version to os2 instead of using os-visibility directly
		params.ESConfig = advancedVisStore.ElasticSearch
		params.ESConfig.SetUsernamePassword()
		params.ESClient = osClient
	}
}

func validateIndex(config *config.ElasticSearchConfig) error {
	indexName, ok := config.Indices[constants.VisibilityAppName]
	if !ok || len(indexName) == 0 {
		return fmt.Errorf("visibility index is missing in config")
	}
	return nil
}

func getFromDynamicConfig(params resource.Params, dc *dynamicconfig.Collection) func() []string {
	return func() []string {
		res, err := isolationgroupapi.MapAllIsolationGroupsResponse(dc.GetListProperty(dynamicproperties.AllIsolationGroups)())
		if err != nil {
			params.Logger.Error("failed to get isolation groups from config", tag.Error(err))
			return nil
		}
		return res
	}
}
