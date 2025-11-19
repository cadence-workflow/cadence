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

package host

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/startreedata/pinot-client-go/pinot"
	"github.com/uber-go/tally"

	adminClient "github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/archiver/filestore"
	"github.com/uber/cadence/common/archiver/provider"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/messaging/kafka"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql"
	persistencetests "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/persistence-tests/testcluster"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/mysql"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/postgres"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/testflags"

	// the import is a test dependency
	_ "github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql/public"
)

type (
	// TestCluster is a base struct for integration tests
	TestCluster struct {
		testBase     *persistencetests.TestBase
		archiverBase *ArchiverBase
		host         Cadence
		doneCh       chan struct{}
	}

	// ArchiverBase is a base struct for archiver provider being used in integration tests
	ArchiverBase struct {
		metadata                 archiver.ArchivalMetadata
		provider                 provider.ArchiverProvider
		historyStoreDirectory    string
		visibilityStoreDirectory string
		historyURI               string
		visibilityURI            string
	}

	// TestClusterConfig are config for a test cluster
	TestClusterConfig struct {
		FrontendAddress       string
		EnableArchival        bool
		IsPrimaryCluster      bool
		ClusterNo             int
		ClusterGroupMetadata  config.ClusterGroupMetadata
		MessagingClientConfig *MessagingClientConfig
		Persistence           persistencetests.TestBaseOptions
		HistoryConfig         *HistoryConfig
		MatchingConfig        *MatchingConfig
		ESConfig              *config.ElasticSearchConfig
		WorkerConfig          *WorkerConfig
		MockAdminClient       map[string]adminClient.Client
		PinotConfig           *config.PinotVisibilityConfig
		AsyncWFQueues         map[string]config.AsyncWorkflowQueueProvider
		DynamicClientConfig   dynamicconfig.FileBasedClientConfig

		// TimeSource is used to override the time source of internal components.
		// Note that most components don't respect this, and it's only used in a few places.
		// e.g. async workflow test's consumer manager and domain manager
		TimeSource                     clock.MockedTimeSource
		FrontendDynamicConfigOverrides map[dynamicproperties.Key]interface{}
		HistoryDynamicConfigOverrides  map[dynamicproperties.Key]interface{}
		MatchingDynamicConfigOverrides map[dynamicproperties.Key]interface{}
		WorkerDynamicConfigOverrides   map[dynamicproperties.Key]interface{}
	}

	// MessagingClientConfig is the config for messaging config
	MessagingClientConfig struct {
		UseMock     bool
		KafkaConfig *config.KafkaConfig
	}

	// WorkerConfig is the config for enabling/disabling cadence worker
	WorkerConfig struct {
		EnableArchiver        bool
		EnableIndexer         bool
		EnableReplicator      bool
		EnableAsyncWFConsumer bool
	}
)

const (
	defaultTestValueOfESIndexMaxResultWindow = 5
	defaultTestPersistenceTimeout            = 5 * time.Second
)

// NewCluster creates and sets up the test cluster
func NewCluster(t *testing.T, options *TestClusterConfig, logger log.Logger, params persistencetests.TestBaseParams) (*TestCluster, error) {
	testBase := persistencetests.NewTestBaseFromParams(t, params)
	testBase.Setup()
	setupShards(testBase, options.HistoryConfig.NumHistoryShards, logger)
	archiverBase := newArchiverBase(options.EnableArchival, logger)
	messagingClient := getMessagingClient(options.MessagingClientConfig, logger)
	pConfig := testBase.Config()
	pConfig.NumHistoryShards = options.HistoryConfig.NumHistoryShards
	var esClient elasticsearch.GenericClient
	if options.WorkerConfig.EnableIndexer {
		var err error
		esClient, err = elasticsearch.NewGenericClient(options.ESConfig, logger)
		if err != nil {
			return nil, err
		}
		pConfig.AdvancedVisibilityStore = "es-visibility"
	}

	scope := tally.NewTestScope("integration-test", nil)
	metricsClient := metrics.NewClient(scope, metrics.ServiceIdx(0), metrics.HistogramMigration{} /* default, only used in test setups */)
	domainReplicationQueue := domain.NewReplicationQueue(
		testBase.DomainReplicationQueueMgr,
		options.ClusterGroupMetadata.CurrentClusterName,
		metricsClient,
		logger,
	)
	aConfig := noopAuthorizationConfig()
	doneCh := make(chan struct{})
	dynamicClient, err := dynamicconfig.NewFileBasedClient(
		&options.DynamicClientConfig,
		logger,
		doneCh,
	)
	if err != nil {
		return nil, err
	}
	cadenceParams := &CadenceParams{
		ClusterMetadata:               params.ClusterMetadata,
		PersistenceConfig:             pConfig,
		MessagingClient:               messagingClient,
		DomainManager:                 testBase.DomainManager,
		HistoryV2Mgr:                  testBase.HistoryV2Mgr,
		ExecutionMgrFactory:           testBase.ExecutionMgrFactory,
		DomainReplicationQueue:        domainReplicationQueue,
		Logger:                        logger,
		ClusterNo:                     options.ClusterNo,
		ESConfig:                      options.ESConfig,
		ESClient:                      esClient,
		ArchiverMetadata:              archiverBase.metadata,
		ArchiverProvider:              archiverBase.provider,
		HistoryConfig:                 options.HistoryConfig,
		MatchingConfig:                options.MatchingConfig,
		WorkerConfig:                  options.WorkerConfig,
		MockAdminClient:               options.MockAdminClient,
		DomainReplicationTaskExecutor: domain.NewReplicationTaskExecutor(testBase.DomainManager, newNoopDomainAuditManager(), clock.NewRealTimeSource(), logger, dynamicproperties.GetBoolPropertyFn(false)),
		AuthorizationConfig:           aConfig,
		AsyncWFQueues:                 options.AsyncWFQueues,
		TimeSource:                    options.TimeSource,
		DynamicClient:                 dynamicClient,
		FrontendDynCfgOverrides:       options.FrontendDynamicConfigOverrides,
		HistoryDynCfgOverrides:        options.HistoryDynamicConfigOverrides,
		MatchingDynCfgOverrides:       options.MatchingDynamicConfigOverrides,
		WorkerDynCfgOverrides:         options.WorkerDynamicConfigOverrides,
	}
	cluster := NewCadence(cadenceParams)
	if err := cluster.Start(); err != nil {
		return nil, err
	}

	return &TestCluster{testBase: testBase, archiverBase: archiverBase, host: cluster, doneCh: doneCh}, nil
}

func NewPinotTestCluster(t *testing.T, options *TestClusterConfig, logger log.Logger, params persistencetests.TestBaseParams) (*TestCluster, error) {
	testBase := persistencetests.NewTestBaseFromParams(t, params)
	testBase.Setup()
	setupShards(testBase, options.HistoryConfig.NumHistoryShards, logger)
	archiverBase := newArchiverBase(options.EnableArchival, logger)
	messagingClient := getMessagingClient(options.MessagingClientConfig, logger)
	pConfig := testBase.Config()
	pConfig.NumHistoryShards = options.HistoryConfig.NumHistoryShards
	var esClient elasticsearch.GenericClient
	var pinotClient pnt.GenericClient
	if options.WorkerConfig.EnableIndexer {
		var err error
		if options.PinotConfig.Migration.Enabled {
			esClient, err = elasticsearch.NewGenericClient(options.ESConfig, logger)
			if err != nil {
				return nil, err
			}
		}
	}
	pConfig.AdvancedVisibilityStore = "pinot-visibility"
	pinotBroker := options.PinotConfig.Broker
	pinotRawClient, err := pinot.NewFromBrokerList([]string{pinotBroker})
	if err != nil || pinotRawClient == nil {
		return nil, err
	}
	pinotClient = pnt.NewPinotClient(pinotRawClient, logger, options.PinotConfig)

	scope := tally.NewTestScope("integration-test", nil)
	metricsClient := metrics.NewClient(scope, metrics.ServiceIdx(0), metrics.HistogramMigration{} /* default, only used in test setups */)
	domainReplicationQueue := domain.NewReplicationQueue(
		testBase.DomainReplicationQueueMgr,
		options.ClusterGroupMetadata.CurrentClusterName,
		metricsClient,
		logger,
	)
	aConfig := noopAuthorizationConfig()
	doneCh := make(chan struct{})
	dynamicClient, err := dynamicconfig.NewFileBasedClient(
		&options.DynamicClientConfig,
		logger,
		doneCh,
	)
	if err != nil {
		return nil, err
	}
	cadenceParams := &CadenceParams{
		ClusterMetadata:               params.ClusterMetadata,
		PersistenceConfig:             pConfig,
		MessagingClient:               messagingClient,
		DomainManager:                 testBase.DomainManager,
		HistoryV2Mgr:                  testBase.HistoryV2Mgr,
		ExecutionMgrFactory:           testBase.ExecutionMgrFactory,
		DomainReplicationQueue:        domainReplicationQueue,
		Logger:                        logger,
		ClusterNo:                     options.ClusterNo,
		ESConfig:                      options.ESConfig,
		ESClient:                      esClient,
		ArchiverMetadata:              archiverBase.metadata,
		ArchiverProvider:              archiverBase.provider,
		HistoryConfig:                 options.HistoryConfig,
		MatchingConfig:                options.MatchingConfig,
		WorkerConfig:                  options.WorkerConfig,
		MockAdminClient:               options.MockAdminClient,
		DomainReplicationTaskExecutor: domain.NewReplicationTaskExecutor(testBase.DomainManager, newNoopDomainAuditManager(), clock.NewRealTimeSource(), logger, dynamicproperties.GetBoolPropertyFn(false)),
		AuthorizationConfig:           aConfig,
		PinotConfig:                   options.PinotConfig,
		PinotClient:                   pinotClient,
		DynamicClient:                 dynamicClient,
	}
	cluster := NewCadence(cadenceParams)
	if err := cluster.Start(); err != nil {
		return nil, err
	}

	return &TestCluster{testBase: testBase, archiverBase: archiverBase, host: cluster, doneCh: doneCh}, nil
}

func noopAuthorizationConfig() config.Authorization {
	return config.Authorization{
		OAuthAuthorizer: config.OAuthAuthorizer{
			Enable: false,
		},
		NoopAuthorizer: config.NoopAuthorizer{
			Enable: true,
		},
	}
}

// NewClusterMetadata returns cluster metdata from config
func NewClusterMetadata(t *testing.T, options *TestClusterConfig) cluster.Metadata {
	if options.ClusterGroupMetadata.PrimaryClusterName == "" {
		return cluster.GetTestClusterMetadata(true)
	}
	return cluster.NewMetadata(
		options.ClusterGroupMetadata,
		func(domain string) bool { return false },
		metrics.NewNoopMetricsClient(),
		testlogger.New(t),
	)
}

func NewPersistenceTestCluster(t *testing.T, clusterConfig *TestClusterConfig) testcluster.PersistenceTestCluster {
	// NOTE: Override here to keep consistent. clusterConfig will be used in the test for some purposes.
	clusterConfig.Persistence.StoreType = TestFlags.PersistenceType
	clusterConfig.Persistence.DBPluginName = TestFlags.SQLPluginName

	var testCluster testcluster.PersistenceTestCluster
	var err error
	if TestFlags.PersistenceType == config.StoreTypeCassandra {
		// TODO refactor to support other NoSQL
		ops := clusterConfig.Persistence
		ops.DBPluginName = "cassandra"
		testflags.RequireCassandra(t)
		testCluster = nosql.NewTestCluster(t, nosql.TestClusterParams{
			PluginName:    ops.DBPluginName,
			KeySpace:      ops.DBName,
			Username:      ops.DBUsername,
			Password:      ops.DBPassword,
			Host:          ops.DBHost,
			Port:          ops.DBPort,
			ProtoVersion:  ops.ProtoVersion,
			SchemaBaseDir: "",
		})
	} else if TestFlags.PersistenceType == config.StoreTypeSQL {
		var ops *persistencetests.TestBaseOptions
		switch TestFlags.SQLPluginName {
		case mysql.PluginName:
			testflags.RequireMySQL(t)
			ops, err = mysql.GetTestClusterOption()
		case postgres.PluginName:
			testflags.RequirePostgres(t)
			ops, err = postgres.GetTestClusterOption()
		case sqlite.PluginName:
			ops = sqlite.GetTestClusterOption()
		default:
			t.Fatal("not supported plugin " + TestFlags.SQLPluginName)
		}

		if err != nil {
			t.Fatal(err)
		}
		testCluster, err = sql.NewTestCluster(TestFlags.SQLPluginName, clusterConfig.Persistence.DBName, ops.DBUsername, ops.DBPassword, ops.DBHost, ops.DBPort, ops.SchemaDir)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal("not supported storage type" + TestFlags.PersistenceType)
	}
	return testCluster
}

func newNoopDomainAuditManager() persistence.DomainAuditManager {
	return &noopDomainAuditManager{}
}

type noopDomainAuditManager struct{}

func (n *noopDomainAuditManager) GetName() string {
	return "noop"
}

func (n *noopDomainAuditManager) Close() {}

func (n *noopDomainAuditManager) CreateDomainAuditLog(ctx context.Context, request *persistence.CreateDomainAuditLogRequest) (*persistence.CreateDomainAuditLogResponse, error) {
	return &persistence.CreateDomainAuditLogResponse{}, nil
}

func (n *noopDomainAuditManager) GetDomainAuditLogs(ctx context.Context, request *persistence.GetDomainAuditLogsRequest) (*persistence.GetDomainAuditLogsResponse, error) {
	return &persistence.GetDomainAuditLogsResponse{}, nil
}

func setupShards(testBase *persistencetests.TestBase, numHistoryShards int, logger log.Logger) {
	// shard 0 is always created, we create additional shards if needed
	for shardID := 1; shardID < numHistoryShards; shardID++ {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
		err := testBase.CreateShard(ctx, shardID, "", 0)
		if err != nil {
			cancel()
			logger.Fatal("Failed to create shard", tag.Error(err))
		}
		cancel()
	}
}

func newArchiverBase(enabled bool, logger log.Logger) *ArchiverBase {
	dcCollection := dynamicconfig.NewNopCollection()
	if !enabled {
		return &ArchiverBase{
			metadata: archiver.NewArchivalMetadata(dcCollection, "", false, "", false, &config.ArchivalDomainDefaults{}),
			provider: provider.NewNoOpArchiverProvider(),
		}
	}

	historyStoreDirectory, err := ioutil.TempDir("", "test-history-archival")
	if err != nil {
		logger.Fatal("Failed to create temp dir for history archival", tag.Error(err))
	}
	visibilityStoreDirectory, err := ioutil.TempDir("", "test-visibility-archival")
	if err != nil {
		logger.Fatal("Failed to create temp dir for visibility archival", tag.Error(err))
	}
	cfg := &config.FilestoreArchiver{
		FileMode: "0666",
		DirMode:  "0766",
	}
	node, err := config.ToYamlNode(cfg)
	if err != nil {
		logger.Fatal("Should be impossible: failed to convert filestore archiver config to a yaml node")
	}

	archiverProvider := provider.NewArchiverProvider(
		config.HistoryArchiverProvider{config.FilestoreConfig: node},
		config.VisibilityArchiverProvider{config.FilestoreConfig: node},
	)
	return &ArchiverBase{
		metadata: archiver.NewArchivalMetadata(dcCollection, "enabled", true, "enabled", true, &config.ArchivalDomainDefaults{
			History: config.HistoryArchivalDomainDefaults{
				Status: "enabled",
				URI:    "testScheme://test/history/archive/path",
			},
			Visibility: config.VisibilityArchivalDomainDefaults{
				Status: "enabled",
				URI:    "testScheme://test/visibility/archive/path",
			},
		}),
		provider:                 archiverProvider,
		historyStoreDirectory:    historyStoreDirectory,
		visibilityStoreDirectory: visibilityStoreDirectory,
		historyURI:               filestore.URIScheme + "://" + historyStoreDirectory,
		visibilityURI:            filestore.URIScheme + "://" + visibilityStoreDirectory,
	}
}

func getMessagingClient(config *MessagingClientConfig, logger log.Logger) messaging.Client {
	if config == nil || config.UseMock {
		return mocks.NewMockMessagingClient(&mocks.KafkaProducer{}, nil)
	}
	checkApp := len(config.KafkaConfig.Applications) != 0
	return kafka.NewKafkaClient(config.KafkaConfig, metrics.NewNoopMetricsClient(), logger, tally.NoopScope, checkApp)
}

// TearDownCluster tears down the test cluster
func (tc *TestCluster) TearDownCluster() {
	tc.host.Stop()
	tc.host = nil
	close(tc.doneCh)
	tc.testBase.TearDownWorkflowStore()
	os.RemoveAll(tc.archiverBase.historyStoreDirectory)
	os.RemoveAll(tc.archiverBase.visibilityStoreDirectory)
}

// GetFrontendClient returns a frontend client from the test cluster
func (tc *TestCluster) GetFrontendClient() FrontendClient {
	return tc.host.GetFrontendClient()
}

// GetAdminClient returns an admin client from the test cluster
func (tc *TestCluster) GetAdminClient() AdminClient {
	return tc.host.GetAdminClient()
}

// GetHistoryClient returns a history client from the test cluster
func (tc *TestCluster) GetHistoryClient() HistoryClient {
	return tc.host.GetHistoryClient()
}

// GetMatchingClient returns a matching client from the test cluster
func (tc *TestCluster) GetMatchingClient() MatchingClient {
	return tc.host.GetMatchingClient()
}

func (tc *TestCluster) GetMatchingClients() []MatchingClient {
	clients := tc.host.GetMatchingClients()
	result := make([]MatchingClient, 0, len(clients))
	for _, client := range clients {
		result = append(result, client)
	}
	return result
}

// GetExecutionManagerFactory returns an execution manager factory from the test cluster
func (tc *TestCluster) GetExecutionManagerFactory() persistence.ExecutionManagerFactory {
	return tc.host.GetExecutionManagerFactory()
}
