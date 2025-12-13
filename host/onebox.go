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

package host

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"github.com/uber-go/tally"
	"github.com/uber-go/tally/prometheus"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	cwsc "go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"

	adminClient "github.com/uber/cadence/client/admin"
	frontendClient "github.com/uber/cadence/client/frontend"
	historyClient "github.com/uber/cadence/client/history"
	matchingClient "github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	carchiver "github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/asyncworkflow/queue"
	"github.com/uber/cadence/common/authorization"
	"github.com/uber/cadence/common/cache"
	cc "github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/isolationgroup/isolationgroupapi"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/wrappers/metered"
	"github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/rpc"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend"
	"github.com/uber/cadence/service/history"
	"github.com/uber/cadence/service/matching"
	"github.com/uber/cadence/service/worker"
	"github.com/uber/cadence/service/worker/archiver"
	"github.com/uber/cadence/service/worker/asyncworkflow"
	"github.com/uber/cadence/service/worker/indexer"
	"github.com/uber/cadence/service/worker/replicator"
)

// Cadence hosts all of cadence services in one process
type Cadence interface {
	Start() error
	Stop()
	GetAdminClient() adminClient.Client
	GetFrontendClient() frontendClient.Client
	FrontendHost() membership.HostInfo
	GetHistoryClient() historyClient.Client
	GetMatchingClient() matchingClient.Client
	GetMatchingClients() []matchingClient.Client
	GetExecutionManagerFactory() persistence.ExecutionManagerFactory
}

type (
	cadenceImpl struct {
		frontendService  common.Daemon
		matchingServices []common.Daemon
		historyServices  []common.Daemon

		adminClient                   adminClient.Client
		frontendClient                frontendClient.Client
		historyClient                 historyClient.Client
		matchingClients               []matchingClient.Client
		logger                        log.Logger
		clusterMetadata               cluster.Metadata
		persistenceConfig             config.Persistence
		messagingClient               messaging.Client
		domainManager                 persistence.DomainManager
		historyV2Mgr                  persistence.HistoryManager
		executionMgrFactory           persistence.ExecutionManagerFactory
		domainReplicationQueue        domain.ReplicationQueue
		shutdownCh                    chan struct{}
		shutdownWG                    sync.WaitGroup
		clusterNo                     int // cluster number
		replicator                    *replicator.Replicator
		clientWorker                  archiver.ClientWorker
		indexer                       *indexer.Indexer
		archiverMetadata              carchiver.ArchivalMetadata
		archiverProvider              provider.ArchiverProvider
		historyConfig                 *HistoryConfig
		matchingConfig                *MatchingConfig
		esConfig                      *config.ElasticSearchConfig
		esClient                      elasticsearch.GenericClient
		workerConfig                  *WorkerConfig
		mockAdminClient               map[string]adminClient.Client
		domainReplicationTaskExecutor domain.ReplicationTaskExecutor
		authorizationConfig           config.Authorization
		pinotConfig                   *config.PinotVisibilityConfig
		pinotClient                   pinot.GenericClient
		asyncWFQueues                 map[string]config.AsyncWorkflowQueueProvider
		timeSource                    clock.TimeSource
		dynamicClient                 dynamicconfig.Client

		// dynamicconfig overrides per service
		frontendDynCfgOverrides map[dynamicproperties.Key]interface{}
		historyDynCfgOverrides  map[dynamicproperties.Key]interface{}
		matchingDynCfgOverrides map[dynamicproperties.Key]interface{}
		workerDynCfgOverrides   map[dynamicproperties.Key]interface{}
	}

	// HistoryConfig contains configs for history service
	HistoryConfig struct {
		// When MockClient is set, rest of the configs are ignored, history service is not started
		// and mock history client is passed to other services
		MockClient HistoryClient

		NumHistoryShards       int
		NumHistoryHosts        int
		HistoryCountLimitError int
		HistoryCountLimitWarn  int
		SimulationConfig       HistorySimulationConfig
	}

	HistorySimulationConfig struct {
		NumWorkflows int
		// WorkflowDeletionJitterRange defines the duration in minutes for workflow close tasks jittering
		// defaults to 0 to remove jittering
		WorkflowDeletionJitterRange int
		// EnableTransferQueueV2 enables queue v2 framework for transfer queue
		EnableTransferQueueV2 bool
		// EnableTimerQueueV2 enables queue v2 framework for timer queue
		EnableTimerQueueV2 bool
	}

	MatchingConfig struct {
		// number of matching host can be at most 4 due to existing static port assignments in onebox.go.
		// can be changed easily.
		NumMatchingHosts int
		SimulationConfig MatchingSimulationConfig
	}

	MatchingSimulationConfig struct {
		// Number of task list write partitions defaults to 1
		TaskListWritePartitions int

		// Number of task list read partitions defaults to 1
		TaskListReadPartitions int

		// At most N polls will be forwarded at a time. defaults to 20
		ForwarderMaxOutstandingPolls int

		// At most N tasks will be forwarded at a time. defaults to 1
		ForwarderMaxOutstandingTasks int

		// Forwarder rps limit defaults to 10
		ForwarderMaxRatePerSecond int

		// Children per node. defaults to 20
		ForwarderMaxChildrenPerNode int

		// LocalPollWaitTime. defaults to 0ms.
		LocalPollWaitTime time.Duration

		// LocalTaskWaitTime. defaults to 0ms.
		LocalTaskWaitTime time.Duration

		// RecordDecisionTaskStartedTime. The amount of time spent by History to complete RecordDecisionTaskStarted
		RecordDecisionTaskStartedTime time.Duration

		// TasklistLoadBalancerStrategy the strategy of load balancer. defaults to "random".
		TasklistLoadBalancerStrategy string

		// The pollers that will be created to process
		Pollers []SimulationPollerConfiguration

		Tasks []SimulationTaskConfiguration

		Backlogs []SimulationBacklogConfiguration

		// GetPartitionConfigFromDB indicates whether to get the partition config from DB or not.
		// This is a prerequisite for adaptive scaler.
		GetPartitionConfigFromDB bool

		// Adaptive scaler configurations
		EnableAdaptiveScaler                bool
		PartitionDownscaleFactor            float64
		PartitionUpscaleRPS                 int
		PartitionUpscaleSustainedDuration   time.Duration
		PartitionDownscaleSustainedDuration time.Duration
		AdaptiveScalerUpdateInterval        time.Duration
		QPSTrackerInterval                  time.Duration
		TaskIsolationDuration               time.Duration
	}

	SimulationPollerConfiguration struct {
		// The isolation group that pollers will be created with. Optional.
		IsolationGroup string
		// The number of pollers that will be created with this configuration. Defaults to 1
		NumPollers int
		// TaskProcessTime. The amount of time spent by the poller in-between requests. Defaults to 1ms
		TaskProcessTime time.Duration
		// Poll request timeout defaults to 15 seconds
		PollTimeout time.Duration
	}

	SimulationTaskConfiguration struct {
		// The isolation groups that tasks will be evenly distributed between
		IsolationGroups []string

		// Number of task generators defaults to 1
		NumTaskGenerators int

		// Upper limit of tasks to generate. Task generators will stop if total number of tasks generated reaches MaxTaskToGenerate during simulation
		// Defaults to 2k
		MaxTaskToGenerate int

		// Task generation QPS. Defaults to 40.
		TasksPerSecond int

		// The burst value for the rate limiter for task generation. Controls the maximum number of AddTask requests
		// that can be sent concurrently. For example, if you have TasksPerSecond, TasksBurst, and NumTaskGenerators all
		// set to 10 then every second you'll get 10 tasks added right at the start of the second. If you instead set
		// TasksBurst to 1 then you'd get a steady stream of tasks, with one task every 100ms.
		TasksBurst int

		// OverTime is a list of TasksProduceSpec that will be used to change the qps over time.
		// Each item has a duration and they will be applied in the given order.
		// If this is set, TasksPerSecond and TasksBurst will be ignored.
		OverTime []TasksProduceSpec
	}

	TasksProduceSpec struct {
		// Task generation qps
		TasksPerSecond int

		// The burst value for the rate limiter for task generation.
		TasksBurst int

		// The duration for which the settings will be applied.
		// If the duration is unset, the settings will be applied indefinitely.
		Duration *time.Duration
	}

	SimulationBacklogConfiguration struct {
		// The partition number
		Partition int // Do not set it to 0, because it's not guaranteed to add backlog to partition 0
		// The backlog count
		BacklogCount int
		// The weight of each isolation group, can be empty
		IsolationGroups map[string]int
	}

	// CadenceParams contains everything needed to bootstrap Cadence
	CadenceParams struct {
		ClusterMetadata               cluster.Metadata
		PersistenceConfig             config.Persistence
		MessagingClient               messaging.Client
		DomainManager                 persistence.DomainManager
		HistoryV2Mgr                  persistence.HistoryManager
		ExecutionMgrFactory           persistence.ExecutionManagerFactory
		DomainReplicationQueue        domain.ReplicationQueue
		Logger                        log.Logger
		ClusterNo                     int
		ArchiverMetadata              carchiver.ArchivalMetadata
		ArchiverProvider              provider.ArchiverProvider
		EnableReadHistoryFromArchival bool
		HistoryConfig                 *HistoryConfig
		MatchingConfig                *MatchingConfig
		ESConfig                      *config.ElasticSearchConfig
		ESClient                      elasticsearch.GenericClient
		WorkerConfig                  *WorkerConfig
		MockAdminClient               map[string]adminClient.Client
		DomainReplicationTaskExecutor domain.ReplicationTaskExecutor
		AuthorizationConfig           config.Authorization
		PinotConfig                   *config.PinotVisibilityConfig
		PinotClient                   pinot.GenericClient
		AsyncWFQueues                 map[string]config.AsyncWorkflowQueueProvider
		TimeSource                    clock.TimeSource
		DynamicClient                 dynamicconfig.Client

		FrontendDynCfgOverrides map[dynamicproperties.Key]interface{}
		HistoryDynCfgOverrides  map[dynamicproperties.Key]interface{}
		MatchingDynCfgOverrides map[dynamicproperties.Key]interface{}
		WorkerDynCfgOverrides   map[dynamicproperties.Key]interface{}
	}
)

// NewCadence returns an instance that hosts full cadence in one process
func NewCadence(params *CadenceParams) Cadence {
	return &cadenceImpl{
		logger:                        params.Logger,
		clusterMetadata:               params.ClusterMetadata,
		persistenceConfig:             params.PersistenceConfig,
		messagingClient:               params.MessagingClient,
		domainManager:                 params.DomainManager,
		historyV2Mgr:                  params.HistoryV2Mgr,
		executionMgrFactory:           params.ExecutionMgrFactory,
		domainReplicationQueue:        params.DomainReplicationQueue,
		shutdownCh:                    make(chan struct{}),
		clusterNo:                     params.ClusterNo,
		esConfig:                      params.ESConfig,
		esClient:                      params.ESClient,
		archiverMetadata:              params.ArchiverMetadata,
		archiverProvider:              params.ArchiverProvider,
		historyConfig:                 params.HistoryConfig,
		matchingConfig:                params.MatchingConfig,
		workerConfig:                  params.WorkerConfig,
		mockAdminClient:               params.MockAdminClient,
		domainReplicationTaskExecutor: params.DomainReplicationTaskExecutor,
		authorizationConfig:           params.AuthorizationConfig,
		pinotConfig:                   params.PinotConfig,
		pinotClient:                   params.PinotClient,
		asyncWFQueues:                 params.AsyncWFQueues,
		timeSource:                    params.TimeSource,
		dynamicClient:                 params.DynamicClient,
		frontendDynCfgOverrides:       params.FrontendDynCfgOverrides,
		historyDynCfgOverrides:        params.HistoryDynCfgOverrides,
		matchingDynCfgOverrides:       params.MatchingDynCfgOverrides,
		workerDynCfgOverrides:         params.WorkerDynCfgOverrides,
	}
}

func (c *cadenceImpl) enableWorker() bool {
	return c.workerConfig.EnableArchiver || c.workerConfig.EnableIndexer || c.workerConfig.EnableReplicator || c.workerConfig.EnableAsyncWFConsumer
}

func (c *cadenceImpl) Start() error {
	hosts := make(map[string][]membership.HostInfo)
	hosts[service.Frontend] = []membership.HostInfo{c.FrontendHost()}
	hosts[service.Matching] = c.MatchingHosts()
	hosts[service.History] = c.HistoryHosts()
	if c.enableWorker() {
		hosts[service.Worker] = []membership.HostInfo{c.WorkerServiceHost()}
	}

	// create cadence-system domain, this must be created before starting
	// the services - so directly use the metadataManager to create this
	if err := c.createSystemDomain(); err != nil {
		return err
	}

	var startWG sync.WaitGroup
	startWG.Add(1)
	go c.startHistory(hosts, &startWG)
	startWG.Wait()

	startWG.Add(1)
	go c.startMatching(hosts, &startWG)
	startWG.Wait()

	startWG.Add(1)
	go c.startFrontend(hosts, &startWG)
	startWG.Wait()

	if c.enableWorker() {
		startWG.Add(1)
		go c.startWorker(hosts, &startWG)
		startWG.Wait()
	}

	return nil
}

func (c *cadenceImpl) Stop() {
	serviceCount := 3
	if c.enableWorker() {
		serviceCount++
	}

	c.shutdownWG.Add(serviceCount)
	c.frontendService.Stop()
	for _, historyService := range c.historyServices {
		historyService.Stop()
	}
	for _, matchingService := range c.matchingServices {
		matchingService.Stop()
	}

	if c.workerConfig.EnableReplicator {
		c.replicator.Stop()
	}
	if c.workerConfig.EnableArchiver {
		c.clientWorker.Stop()
	}
	close(c.shutdownCh)
	c.shutdownWG.Wait()
}

func newHost(tchan uint16) membership.HostInfo {
	address := "127.0.0.1"
	return membership.NewDetailedHostInfo(
		fmt.Sprintf("%s:%d", address, tchan),
		fmt.Sprintf("%s_%d", address, tchan),
		membership.PortMap{
			membership.PortTchannel: tchan,
			membership.PortGRPC:     tchan + 10,
		},
	)

}

func (c *cadenceImpl) FrontendHost() membership.HostInfo {
	var tchan uint16
	switch c.clusterNo {
	case 0:
		tchan = 7104
	case 1:
		tchan = 8104
	case 2:
		tchan = 9104
	case 3:
		tchan = 10104
	default:
		tchan = 7104
	}

	return newHost(tchan)

}

func (c *cadenceImpl) FrontendPProfPort() int {
	switch c.clusterNo {
	case 0:
		return 7105
	case 1:
		return 8105
	case 2:
		return 9105
	case 3:
		return 10105
	default:
		return 7105
	}
}

func (c *cadenceImpl) HistoryHosts() []membership.HostInfo {
	var hosts []membership.HostInfo
	var startPort int
	switch c.clusterNo {
	case 0:
		startPort = 7201
	case 1:
		startPort = 8201
	case 2:
		startPort = 9201
	case 3:
		startPort = 10201
	default:
		startPort = 7201
	}
	for i := 0; i < c.historyConfig.NumHistoryHosts; i++ {
		port := startPort + i
		hosts = append(hosts, newHost(uint16(port)))
	}

	c.logger.Info("History hosts", tag.Value(hosts))
	return hosts
}

func (c *cadenceImpl) HistoryPProfPorts() []int {
	var ports []int
	var startPort int
	switch c.clusterNo {
	case 0:
		startPort = 7301
	case 1:
		startPort = 8301
	case 2:
		startPort = 9301
	case 3:
		startPort = 10301
	default:
		startPort = 7301
	}
	for i := 0; i < c.historyConfig.NumHistoryHosts; i++ {
		port := startPort + i
		ports = append(ports, port)
	}

	c.logger.Info("History pprof ports", tag.Value(ports))
	return ports
}

func (c *cadenceImpl) MatchingHosts() []membership.HostInfo {
	var hosts []membership.HostInfo
	var tchan uint16
	switch c.clusterNo {
	case 0:
		tchan = 7106
	case 1:
		tchan = 8106
	case 2:
		tchan = 9106
	case 3:
		tchan = 10106
	default:
		tchan = 7106
	}

	for i := 0; i < c.matchingConfig.NumMatchingHosts; i++ {
		port := tchan + uint16(i)
		hosts = append(hosts, newHost(uint16(port)))
	}

	c.logger.Info("Matching hosts", tag.Value(hosts))

	return hosts
}

func (c *cadenceImpl) MatchingPProfPorts() []int {
	var ports []int
	var startPort int
	switch c.clusterNo {
	case 0:
		startPort = 7206
	case 1:
		startPort = 8206
	case 2:
		startPort = 9206
	case 3:
		startPort = 10206
	default:
		startPort = 7206
	}

	for i := 0; i < c.matchingConfig.NumMatchingHosts; i++ {
		port := startPort + i
		ports = append(ports, port)
	}

	c.logger.Info("Matching pprof ports", tag.Value(ports))
	return ports
}

func (c *cadenceImpl) MatchingPrometheusPorts() []int {
	var ports []int
	var startPort int
	switch c.clusterNo {
	case 0:
		startPort = 7306
	case 1:
		startPort = 8306
	case 2:
		startPort = 9306
	case 3:
		startPort = 10306
	default:
		startPort = 7306
	}

	for i := 0; i < c.matchingConfig.NumMatchingHosts; i++ {
		port := startPort + i
		ports = append(ports, port)
	}

	c.logger.Info("Matching prometheus ports", tag.Value(ports))
	return ports
}

func (c *cadenceImpl) WorkerServiceHost() membership.HostInfo {
	var tchan uint16
	switch c.clusterNo {
	case 0:
		tchan = 7108
	case 1:
		tchan = 8108
	case 2:
		tchan = 9108
	case 3:
		tchan = 10108
	default:
		tchan = 7108
	}
	return newHost(tchan)
}

func (c *cadenceImpl) WorkerPProfPort() int {
	switch c.clusterNo {
	case 0:
		return 7109
	case 1:
		return 8109
	case 2:
		return 9109
	case 3:
		return 10109
	default:
		return 7109
	}
}

func (c *cadenceImpl) GetAdminClient() adminClient.Client {
	return c.adminClient
}

func (c *cadenceImpl) GetFrontendClient() frontendClient.Client {
	return c.frontendClient
}

func (c *cadenceImpl) GetHistoryClient() historyClient.Client {
	return c.historyClient
}

func (c *cadenceImpl) GetMatchingClient() matchingClient.Client {
	return c.matchingClients[0]
}

func (c *cadenceImpl) GetMatchingClients() []matchingClient.Client {
	return c.matchingClients
}

func (c *cadenceImpl) startFrontend(hosts map[string][]membership.HostInfo, startWG *sync.WaitGroup) {
	params := new(resource.Params)
	params.ClusterRedirectionPolicy = &config.ClusterRedirectionPolicy{}
	params.Name = service.Frontend
	params.Logger = c.logger
	params.ThrottledLogger = c.logger
	params.TimeSource = c.timeSource
	params.PProfInitializer = newPProfInitializerImpl(c.logger, c.FrontendPProfPort())
	params.MetricScope = tally.NewTestScope(service.Frontend, make(map[string]string))
	params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, c.logger), metrics.HistogramMigration{} /* default, only used in test setups */)
	params.RPCFactory = c.newRPCFactory(service.Frontend, c.FrontendHost(), params.MetricsClient)
	params.MembershipResolver = newMembershipResolver(params.Name, hosts, c.FrontendHost())
	params.ClusterMetadata = c.clusterMetadata
	params.MessagingClient = c.messagingClient
	params.DynamicConfig = newIntegrationConfigClient(c.dynamicClient, c.frontendDynCfgOverrides)
	params.ArchivalMetadata = c.archiverMetadata
	params.ArchiverProvider = c.archiverProvider
	params.ESConfig = c.esConfig
	params.ESClient = c.esClient
	params.PinotConfig = c.pinotConfig
	params.PinotClient = c.pinotClient
	params.GetIsolationGroups = getFromDynamicConfig(params)
	var err error
	authorizer, err := authorization.NewAuthorizer(c.authorizationConfig, params.Logger, nil)
	if err != nil {
		c.logger.Fatal("Unable to create authorizer", tag.Error(err))
	}
	params.Authorizer = authorizer
	params.PersistenceConfig, err = copyPersistenceConfig(c.persistenceConfig)
	if err != nil {
		c.logger.Fatal("Failed to copy persistence config for frontend", tag.Error(err))
	}

	if c.pinotConfig != nil {
		pinotDataStoreName := "pinot-visibility"
		params.PersistenceConfig.AdvancedVisibilityStore = pinotDataStoreName
		params.DynamicConfig.UpdateValue(dynamicproperties.ReadVisibilityStoreName, constants.VisibilityModePinot)
		params.PersistenceConfig.DataStores[pinotDataStoreName] = config.DataStore{
			Pinot: c.pinotConfig,
		}
	} else if c.esConfig != nil {
		esDataStoreName := "es-visibility"
		params.PersistenceConfig.AdvancedVisibilityStore = esDataStoreName
		params.PersistenceConfig.DataStores[esDataStoreName] = config.DataStore{
			ElasticSearch: c.esConfig,
		}
	}

	if c.asyncWFQueues != nil {
		params.AsyncWorkflowQueueProvider, err = queue.NewAsyncQueueProvider(c.asyncWFQueues)
		if err != nil {
			c.logger.Fatal("error creating async queue provider", tag.Error(err))
		}
	}

	frontendService, err := frontend.NewService(params)
	if err != nil {
		params.Logger.Fatal("unable to start frontend service", tag.Error(err))
	}

	if c.mockAdminClient != nil {
		clientBean := frontendService.GetClientBean()
		if clientBean != nil {
			for serviceName, client := range c.mockAdminClient {
				clientBean.SetRemoteAdminClient(serviceName, client)
			}
		}
	}

	c.frontendService = frontendService
	c.frontendClient = NewFrontendClient(frontendService.GetDispatcher())
	c.adminClient = NewAdminClient(frontendService.GetDispatcher())
	go frontendService.Start()

	c.logger.Info("Started frontend service")
	startWG.Done()

	<-c.shutdownCh
	c.shutdownWG.Done()
}

func (c *cadenceImpl) startHistory(hosts map[string][]membership.HostInfo, startWG *sync.WaitGroup) {
	pprofPorts := c.HistoryPProfPorts()
	historyHosts := c.HistoryHosts()
	for i, hostport := range historyHosts {
		params := new(resource.Params)
		params.Name = service.History
		params.Logger = c.logger
		params.ThrottledLogger = c.logger
		params.TimeSource = c.timeSource
		params.PProfInitializer = newPProfInitializerImpl(c.logger, pprofPorts[i])
		params.MetricScope = tally.NewTestScope(service.History, make(map[string]string))
		params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, c.logger), metrics.HistogramMigration{} /* default, only used in test setups */)
		params.RPCFactory = c.newRPCFactory(service.History, hostport, params.MetricsClient)
		params.MembershipResolver = newMembershipResolver(params.Name, hosts, hostport)
		params.ClusterMetadata = c.clusterMetadata
		params.MessagingClient = c.messagingClient
		integrationClient := newIntegrationConfigClient(c.dynamicClient, c.historyDynCfgOverrides)
		c.overrideHistoryDynamicConfig(integrationClient)
		params.DynamicConfig = integrationClient
		params.PublicClient = newPublicClient(params.RPCFactory.GetDispatcher())
		params.ArchivalMetadata = c.archiverMetadata
		params.ArchiverProvider = c.archiverProvider
		params.ESConfig = c.esConfig
		params.ESClient = c.esClient
		params.PinotConfig = c.pinotConfig
		params.GetIsolationGroups = getFromDynamicConfig(params)

		var err error
		params.PersistenceConfig, err = copyPersistenceConfig(c.persistenceConfig)
		if err != nil {
			c.logger.Fatal("Failed to copy persistence config for history", tag.Error(err))
		}

		if c.pinotConfig != nil {
			pinotDataStoreName := "pinot-visibility"
			params.PersistenceConfig.AdvancedVisibilityStore = pinotDataStoreName
			params.PersistenceConfig.DataStores[pinotDataStoreName] = config.DataStore{
				Pinot:         c.pinotConfig,
				ElasticSearch: c.esConfig,
			}
			params.DynamicConfig.UpdateValue(dynamicproperties.WriteVisibilityStoreName, constants.VisibilityModePinot)
		} else if c.esConfig != nil {
			esDataStoreName := "es-visibility"
			params.PersistenceConfig.AdvancedVisibilityStore = esDataStoreName
			params.PersistenceConfig.DataStores[esDataStoreName] = config.DataStore{
				ElasticSearch: c.esConfig,
			}
		}

		historyService, err := history.NewService(params)
		if err != nil {
			params.Logger.Fatal("unable to start history service", tag.Error(err))
		}

		if c.mockAdminClient != nil {
			clientBean := historyService.GetClientBean()
			if clientBean != nil {
				for serviceName, client := range c.mockAdminClient {
					clientBean.SetRemoteAdminClient(serviceName, client)
				}
			}
		}

		// TODO: this is not correct when there are multiple history hosts as later client will overwrite previous ones.
		// However current interface for getting history client doesn't specify which client it needs and the tests that use this API
		// depends on the fact that there's only one history host.
		// Need to change those tests and modify the interface for getting history client.
		c.historyClient = NewHistoryClient(historyService.GetDispatcher())
		c.historyServices = append(c.historyServices, historyService)

		go historyService.Start()
	}

	startWG.Done()
	c.logger.Info(fmt.Sprintf("Started %d history services", len(c.historyServices)))

	<-c.shutdownCh
	c.shutdownWG.Done()
}

func (c *cadenceImpl) startMatching(hosts map[string][]membership.HostInfo, startWG *sync.WaitGroup) {
	pprofPorts := c.MatchingPProfPorts()
	promPorts := c.MatchingPrometheusPorts()
	for i, hostport := range c.MatchingHosts() {
		hostport.Identity()
		matchingHost := fmt.Sprintf("matching-host-%d:%s", i, hostport.Identity())
		params := new(resource.Params)
		params.Name = service.Matching
		params.Logger = c.logger.WithTags(tag.Dynamic("matching-host", matchingHost))
		params.ThrottledLogger = c.logger
		params.TimeSource = c.timeSource
		params.PProfInitializer = newPProfInitializerImpl(c.logger, pprofPorts[i])
		metricsCfg := config.Metrics{
			Prometheus: &prometheus.Configuration{
				TimerType:     "histogram",
				ListenAddress: fmt.Sprintf(":%d", promPorts[i]),
			},
		}
		params.MetricScope = metricsCfg.NewScope(c.logger, service.Matching)
		params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, c.logger), metrics.HistogramMigration{} /* default, only used in test setups */)
		params.RPCFactory = c.newRPCFactory(service.Matching, hostport, params.MetricsClient)
		params.MembershipResolver = newMembershipResolver(params.Name, hosts, hostport)
		params.ClusterMetadata = c.clusterMetadata
		params.DynamicConfig = newIntegrationConfigClient(c.dynamicClient, c.matchingDynCfgOverrides)
		params.ArchivalMetadata = c.archiverMetadata
		params.ArchiverProvider = c.archiverProvider
		params.GetIsolationGroups = getFromDynamicConfig(params)

		var err error
		params.PersistenceConfig, err = copyPersistenceConfig(c.persistenceConfig)
		if err != nil {
			c.logger.Fatal("Failed to copy persistence config for matching", tag.Error(err))
		}

		if c.historyConfig.MockClient != nil {
			params.HistoryClientFn = func() historyClient.Client {
				return c.historyConfig.MockClient
			}
		}

		matchingService, err := matching.NewService(params)
		if err != nil {
			params.Logger.Fatal("unable to start matching service", tag.Error(err))
		}

		clientBean := matchingService.GetClientBean()
		if c.mockAdminClient != nil {
			for serviceName, client := range c.mockAdminClient {
				clientBean.SetRemoteAdminClient(serviceName, client)
			}
		}

		// When there are multiple matching hosts the last client will overwrite previous ones.
		// It should be fine because the underlying client bean logic should still pick the right destination.
		matchingClient, err := clientBean.GetMatchingClient(matchingService.GetDomainCache().GetDomainName)
		if err != nil {
			params.Logger.Fatal("unable to get matching client", tag.Error(err))
		}
		c.matchingClients = append(c.matchingClients, matchingClient)
		c.matchingServices = append(c.matchingServices, matchingService)
		go matchingService.Start()
	}

	startWG.Done()
	c.logger.Info(fmt.Sprintf("Started %d matching services", len(c.matchingServices)))

	<-c.shutdownCh
	c.shutdownWG.Done()
}

func (c *cadenceImpl) startWorker(hosts map[string][]membership.HostInfo, startWG *sync.WaitGroup) {
	defer c.shutdownWG.Done()

	params := new(resource.Params)
	params.Name = service.Worker
	params.Logger = c.logger
	params.ThrottledLogger = c.logger
	params.TimeSource = c.timeSource
	params.PProfInitializer = newPProfInitializerImpl(c.logger, c.WorkerPProfPort())
	params.MetricScope = tally.NewTestScope(service.Worker, make(map[string]string))
	params.MetricsClient = metrics.NewClient(params.MetricScope, service.GetMetricsServiceIdx(params.Name, c.logger), metrics.HistogramMigration{} /* default, only used in test setups */)
	params.RPCFactory = c.newRPCFactory(service.Worker, c.WorkerServiceHost(), params.MetricsClient)
	params.MembershipResolver = newMembershipResolver(params.Name, hosts, c.WorkerServiceHost())
	params.ClusterMetadata = c.clusterMetadata
	params.DynamicConfig = newIntegrationConfigClient(c.dynamicClient, c.workerDynCfgOverrides)
	params.ArchivalMetadata = c.archiverMetadata
	params.ArchiverProvider = c.archiverProvider
	params.GetIsolationGroups = getFromDynamicConfig(params)

	var err error
	params.PersistenceConfig, err = copyPersistenceConfig(c.persistenceConfig)
	if err != nil {
		c.logger.Fatal("Failed to copy persistence config for worker", tag.Error(err))
	}
	params.PublicClient = newPublicClient(params.RPCFactory.GetDispatcher())
	service := NewService(params)
	service.Start()

	var replicatorDomainCache cache.DomainCache
	if c.workerConfig.EnableReplicator {
		metadataManager := metered.NewDomainManager(c.domainManager, service.GetMetricsClient(), c.logger, &c.persistenceConfig, "onebox-worker", "onebox")
		replicatorDomainCache = cache.NewDomainCache(metadataManager, c.clusterMetadata, service.GetMetricsClient(), service.GetLogger())
		replicatorDomainCache.Start()
		defer replicatorDomainCache.Stop()
		c.startWorkerReplicator(service)
	}

	var clientWorkerDomainCache cache.DomainCache
	if c.workerConfig.EnableArchiver {
		metadataProxyManager := metered.NewDomainManager(c.domainManager, service.GetMetricsClient(), c.logger, &c.persistenceConfig, "onebox-worker", "onebox")
		clientWorkerDomainCache = cache.NewDomainCache(metadataProxyManager, c.clusterMetadata, service.GetMetricsClient(), service.GetLogger())
		clientWorkerDomainCache.Start()
		defer clientWorkerDomainCache.Stop()
		c.startWorkerClientWorker(params, service, clientWorkerDomainCache)
	}

	if c.workerConfig.EnableIndexer {
		c.startWorkerIndexer(params, service)
	}

	var asyncWFDomainCache cache.DomainCache
	if c.workerConfig.EnableAsyncWFConsumer {
		queueProvider, err := queue.NewAsyncQueueProvider(c.asyncWFQueues)
		if err != nil {
			c.logger.Fatal("error creating async queue provider", tag.Error(err))
		}

		metadataProxyManager := metered.NewDomainManager(
			c.domainManager,
			service.GetMetricsClient(),
			c.logger,
			&c.persistenceConfig,
			"onebox-worker",
			"onebox")
		asyncWFDomainCache = cache.NewDomainCache(
			metadataProxyManager,
			c.clusterMetadata,
			service.GetMetricsClient(),
			service.GetLogger(),
			cache.WithTimeSource(params.TimeSource))
		asyncWFDomainCache.Start()
		defer asyncWFDomainCache.Stop()
		cm := asyncworkflow.NewConsumerManager(
			service.GetLogger(),
			service.GetMetricsClient(),
			asyncWFDomainCache,
			queueProvider,
			c.frontendClient,
			asyncworkflow.WithTimeSource(params.TimeSource),
			asyncworkflow.WithRefreshInterval(time.Second),
		)
		cm.Start()
		defer cm.Stop()
	}

	c.logger.Info("Started worker service")
	startWG.Done()

	<-c.shutdownCh
	if c.workerConfig.EnableReplicator {
		replicatorDomainCache.Stop()
	}
	if c.workerConfig.EnableArchiver {
		clientWorkerDomainCache.Stop()
	}
}

func (c *cadenceImpl) startWorkerReplicator(svc Service) {
	c.replicator = replicator.NewReplicator(
		c.clusterMetadata,
		svc.GetClientBean(),
		c.logger,
		svc.GetMetricsClient(),
		svc.GetHostInfo(),
		svc.GetMembershipResolver(),
		c.domainReplicationQueue,
		c.domainReplicationTaskExecutor,
		time.Millisecond,
	)
	if err := c.replicator.Start(); err != nil {
		c.replicator.Stop()
		c.logger.Fatal("Fail to start replicator when start worker", tag.Error(err))
	}
}

func (c *cadenceImpl) startWorkerClientWorker(params *resource.Params, svc Service, domainCache cache.DomainCache) {
	workerConfig := worker.NewConfig(params)
	workerConfig.ArchiverConfig.ArchiverConcurrency = dynamicproperties.GetIntPropertyFn(10)
	historyArchiverBootstrapContainer := &carchiver.HistoryBootstrapContainer{
		HistoryV2Manager: c.historyV2Mgr,
		Logger:           c.logger,
		MetricsClient:    svc.GetMetricsClient(),
		ClusterMetadata:  c.clusterMetadata,
		DomainCache:      domainCache,
	}
	err := c.archiverProvider.RegisterBootstrapContainer(service.Worker, historyArchiverBootstrapContainer, &carchiver.VisibilityBootstrapContainer{})
	if err != nil {
		c.logger.Fatal("Failed to register archiver bootstrap container for worker service", tag.Error(err))
	}

	bc := &archiver.BootstrapContainer{
		PublicClient:     params.PublicClient,
		MetricsClient:    svc.GetMetricsClient(),
		Logger:           c.logger,
		HistoryV2Manager: c.historyV2Mgr,
		DomainCache:      domainCache,
		Config:           workerConfig.ArchiverConfig,
		ArchiverProvider: c.archiverProvider,
	}
	c.clientWorker = archiver.NewClientWorker(bc)
	if err := c.clientWorker.Start(); err != nil {
		c.clientWorker.Stop()
		c.logger.Fatal("Fail to start archiver when start worker", tag.Error(err))
	}
}

func (c *cadenceImpl) startWorkerIndexer(params *resource.Params, service Service) {
	params.DynamicConfig.UpdateValue(dynamicproperties.WriteVisibilityStoreName, constants.VisibilityModeES)
	workerConfig := worker.NewConfig(params)
	c.indexer = indexer.NewIndexer(
		workerConfig.IndexerCfg,
		c.messagingClient,
		c.esClient,
		c.esConfig.Indices[constants.VisibilityAppName],
		c.esConfig.ConsumerName,
		c.logger,
		service.GetMetricsClient())
	if err := c.indexer.Start(); err != nil {
		c.indexer.Stop()
		c.logger.Fatal("Fail to start indexer when start worker", tag.Error(err))
	}
}

func (c *cadenceImpl) createSystemDomain() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
	defer cancel()

	_, err := c.domainManager.CreateDomain(ctx, &persistence.CreateDomainRequest{
		Info: &persistence.DomainInfo{
			ID:          uuid.New(),
			Name:        "cadence-system",
			Status:      persistence.DomainStatusRegistered,
			Description: "Cadence system domain",
		},
		Config: &persistence.DomainConfig{
			Retention:                1,
			HistoryArchivalStatus:    types.ArchivalStatusDisabled,
			VisibilityArchivalStatus: types.ArchivalStatusDisabled,
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{},
		FailoverVersion:   constants.EmptyVersion,
	})
	if err != nil {
		if _, ok := err.(*types.DomainAlreadyExistsError); ok {
			return nil
		}
		return fmt.Errorf("failed to create cadence-system domain: %v", err)
	}
	return nil
}

func (c *cadenceImpl) GetExecutionManagerFactory() persistence.ExecutionManagerFactory {
	return c.executionMgrFactory
}

func (c *cadenceImpl) overrideHistoryDynamicConfig(client *dynamicClient) {
	client.OverrideValue(dynamicproperties.ReplicationTaskProcessorStartWait, time.Nanosecond)

	if c.workerConfig.EnableIndexer {
		client.OverrideValue(dynamicproperties.WriteVisibilityStoreName, constants.VisibilityModeES)
	}
	if c.historyConfig.HistoryCountLimitWarn != 0 {
		client.OverrideValue(dynamicproperties.HistoryCountLimitWarn, c.historyConfig.HistoryCountLimitWarn)
	}
	if c.historyConfig.HistoryCountLimitError != 0 {
		client.OverrideValue(dynamicproperties.HistoryCountLimitError, c.historyConfig.HistoryCountLimitError)
	}
}

// copyPersistenceConfig makes a deepcopy of persistence config.
// This is just a temp fix for the race condition of persistence config.
// The race condition happens because all the services are using the same datastore map in the config.
// Also all services will retry to modify the maxQPS field in the datastore during start up and use the modified maxQPS value to create a persistence factory.
func copyPersistenceConfig(pConfig config.Persistence) (config.Persistence, error) {
	copiedDataStores := make(map[string]config.DataStore)
	for name, value := range pConfig.DataStores {
		copiedDataStore := config.DataStore{}
		encodedDataStore, err := json.Marshal(value)
		if err != nil {
			return pConfig, err
		}

		if err = json.Unmarshal(encodedDataStore, &copiedDataStore); err != nil {
			return pConfig, err
		}
		copiedDataStores[name] = copiedDataStore
	}
	pConfig.DataStores = copiedDataStores
	return pConfig, nil
}

func newMembershipResolver(serviceName string, hosts map[string][]membership.HostInfo, currentHost membership.HostInfo) membership.Resolver {
	return NewSimpleResolver(serviceName, hosts, currentHost)
}

func newPProfInitializerImpl(logger log.Logger, port int) common.PProfInitializer {
	return &config.PProfInitializerImpl{
		PProf: &config.PProf{
			Port: port,
		},
		Logger: logger,
	}
}

func newPublicClient(dispatcher *yarpc.Dispatcher) cwsc.Interface {
	config := dispatcher.ClientConfig(rpc.OutboundPublicClient)
	return compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(config),
		apiv1.NewWorkflowAPIYARPCClient(config),
		apiv1.NewWorkerAPIYARPCClient(config),
		apiv1.NewVisibilityAPIYARPCClient(config),
	)
}

func (c *cadenceImpl) newRPCFactory(serviceName string, host membership.HostInfo, metricsCl metrics.Client) rpc.Factory {
	tchannelAddress, err := host.GetNamedAddress(membership.PortTchannel)
	if err != nil {
		c.logger.Fatal("failed to get PortTchannel port from host", tag.Value(host), tag.Error(err))
	}

	grpcAddress, err := host.GetNamedAddress(membership.PortGRPC)
	if err != nil {
		c.logger.Fatal("failed to get PortGRPC port from host", tag.Value(host), tag.Error(err))
	}

	frontendGrpcAddress, err := c.FrontendHost().GetNamedAddress(membership.PortGRPC)
	if err != nil {
		c.logger.Fatal("failed to get frontend PortGRPC", tag.Value(c.FrontendHost()), tag.Error(err))
	}

	directOutboundPCF := rpc.NewDirectPeerChooserFactory(serviceName, c.logger, metricsCl)
	directConnRetainFn := func(opts ...dynamicproperties.FilterOption) bool { return false }

	return rpc.NewFactory(c.logger, rpc.Params{
		ServiceName:     serviceName,
		TChannelAddress: tchannelAddress,
		GRPCAddress:     grpcAddress,
		InboundMiddleware: yarpc.InboundMiddleware{
			Unary: yarpc.UnaryInboundMiddleware(&versionMiddleware{}, &rpc.ClientPartitionConfigMiddleware{}, &rpc.ForwardPartitionConfigMiddleware{}),
		},
		OutboundMiddleware: yarpc.OutboundMiddleware{
			Unary: &rpc.ForwardPartitionConfigMiddleware{},
		},

		// For integration tests to generate client out of the same outbound.
		OutboundsBuilder: rpc.CombineOutbounds(
			rpc.NewSingleGRPCOutboundBuilder(testOutboundName(serviceName), serviceName, grpcAddress),
			rpc.NewSingleGRPCOutboundBuilder(rpc.OutboundPublicClient, service.Frontend, frontendGrpcAddress),
			rpc.NewCrossDCOutbounds(c.clusterMetadata.GetAllClusterInfo(), rpc.NewDNSPeerChooserFactory(0, c.logger)),
			rpc.NewDirectOutboundBuilder(service.History, true, nil, directOutboundPCF, directConnRetainFn),
			rpc.NewDirectOutboundBuilder(service.Matching, true, nil, directOutboundPCF, directConnRetainFn),
		),
	})
}

// testOutbound prefixes outbound with "test-" to not clash with other real Cadence outbounds.
func testOutboundName(name string) string {
	return "test-" + name
}

type versionMiddleware struct {
}

func (vm *versionMiddleware) Handle(ctx context.Context, req *transport.Request, resw transport.ResponseWriter, h transport.UnaryHandler) error {
	req.Headers = req.Headers.With(common.LibraryVersionHeaderName, "1.0.0").
		With(common.FeatureVersionHeaderName, cc.SupportedGoSDKVersion).
		With(common.ClientImplHeaderName, cc.GoSDK)

	return h.Handle(ctx, req, resw)
}

func getFromDynamicConfig(params *resource.Params) func() []string {
	return func() []string {
		list, err := params.DynamicConfig.GetListValue(dynamicproperties.AllIsolationGroups, nil)
		if err != nil {
			params.Logger.Error("failed to get isolation groups from config", tag.Error(err))
			return nil
		}
		res, err := isolationgroupapi.MapAllIsolationGroupsResponse(list)
		if err != nil {
			params.Logger.Error("failed to map isolation groups from config", tag.Error(err))
			return nil
		}
		return res
	}
}
