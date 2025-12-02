// Copyright (c) 2016 Uber Technologies, Inc.
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
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/tchannel"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/persistence/persistence-tests/testcluster"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin/sqlite"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/environment"
)

type (
	// IntegrationBase is a base struct for integration tests
	IntegrationBase struct {
		suite.Suite

		TestCluster              *TestCluster
		TestClusterConfig        *TestClusterConfig
		Engine                   FrontendClient
		HistoryClient            HistoryClient
		AdminClient              AdminClient
		Logger                   log.Logger
		DomainName               string
		SecondaryDomainName      string
		TestRawHistoryDomainName string
		ForeignDomainName        string
		ArchivalDomainName       string
		ActiveActiveDomainName   string
		DefaultTestCluster       testcluster.PersistenceTestCluster
		VisibilityTestCluster    testcluster.PersistenceTestCluster
	}

	IntegrationBaseParams struct {
		T                     *testing.T
		DefaultTestCluster    testcluster.PersistenceTestCluster
		VisibilityTestCluster testcluster.PersistenceTestCluster
		TestClusterConfig     *TestClusterConfig
	}
)

func NewIntegrationBase(params IntegrationBaseParams) *IntegrationBase {
	return &IntegrationBase{
		DefaultTestCluster:    params.DefaultTestCluster,
		VisibilityTestCluster: params.VisibilityTestCluster,
		TestClusterConfig:     params.TestClusterConfig,
	}
}

func (s *IntegrationBase) setupSuite() {
	s.SetupLogger()

	if s.TestClusterConfig.FrontendAddress != "" {
		s.Logger.Info("Running integration test against specified frontend", tag.Address(TestFlags.FrontendAddr))
		channel, err := tchannel.NewChannelTransport(tchannel.ServiceName("cadence-frontend"))
		s.Require().NoError(err)
		dispatcher := yarpc.NewDispatcher(yarpc.Config{
			Name: "unittest",
			Outbounds: yarpc.Outbounds{
				"cadence-frontend": {Unary: channel.NewSingleOutbound(TestFlags.FrontendAddr)},
			},
			InboundMiddleware: yarpc.InboundMiddleware{
				Unary: &versionMiddleware{},
			},
		})
		if err := dispatcher.Start(); err != nil {
			s.Logger.Fatal("Failed to create outbound transport channel", tag.Error(err))
		}

		s.Engine = NewFrontendClient(dispatcher)
		s.HistoryClient = NewHistoryClient(dispatcher)
		s.AdminClient = NewAdminClient(dispatcher)
	} else {
		s.Logger.Info("Running integration test against test cluster")
		clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
		dc := persistence.DynamicConfiguration{
			EnableSQLAsyncTransaction:                dynamicproperties.GetBoolPropertyFn(false),
			EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
			PersistenceSampleLoggingRate:             dynamicproperties.GetIntPropertyFn(100),
			EnableShardIDMetrics:                     dynamicproperties.GetBoolPropertyFn(true),
			EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
			ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
			SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
			ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
			HistoryNodeDeleteBatchSize:               dynamicproperties.GetIntPropertyFn(1000),
		}
		params := pt.TestBaseParams{
			DefaultTestCluster:    s.DefaultTestCluster,
			VisibilityTestCluster: s.VisibilityTestCluster,
			ClusterMetadata:       clusterMetadata,
			DynamicConfiguration:  dc,
		}
		cluster, err := NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
		s.Require().NoError(err)
		s.TestCluster = cluster
		s.Engine = s.TestCluster.GetFrontendClient()
		s.HistoryClient = s.TestCluster.GetHistoryClient()
		s.AdminClient = s.TestCluster.GetAdminClient()
	}
	s.TestRawHistoryDomainName = "TestRawHistoryDomain"
	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(
		s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))
	s.Require().NoError(
		s.RegisterDomain(s.TestRawHistoryDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))
	s.ForeignDomainName = s.RandomizeStr("integration-foreign-test-domain")
	s.Require().NoError(
		s.RegisterDomain(s.ForeignDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.Require().NoError(s.registerArchivalDomain())
	s.ActiveActiveDomainName = s.RandomizeStr("integration-active-active-test-domain")
	s.Require().NoError(s.RegisterDomain(s.ActiveActiveDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", &types.ActiveClusters{
		AttributeScopes: map[string]types.ClusterAttributeScope{
			"region": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					"us-east": {ActiveClusterName: s.TestCluster.testBase.ClusterMetadata.GetCurrentClusterName()},
				},
			},
			"city": {
				ClusterAttributes: map[string]types.ActiveClusterInfo{
					"tokyo": {ActiveClusterName: s.TestCluster.testBase.ClusterMetadata.GetCurrentClusterName()},
				},
			},
		},
	}))

	// this sleep is necessary because domainv2 cache gets refreshed in the
	// background only every domainCacheRefreshInterval period
	time.Sleep(cache.DomainCacheRefreshInterval + time.Second)
}

func (s *IntegrationBase) SetupLogger() {
	s.Logger = testlogger.New(s.T())
}

// GetTestClusterConfig return test cluster config
func GetTestClusterConfig(configFile string) (*TestClusterConfig, error) {
	if err := environment.SetupEnv(); err != nil {
		return nil, err
	}

	configLocation := configFile
	if TestFlags.TestClusterConfigFile != "" {
		configLocation = TestFlags.TestClusterConfigFile
	}
	// This is just reading a config so it's less of a security concern
	// #nosec
	confContent, err := os.ReadFile(configLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cluster config file %v: %v", configLocation, err)
	}
	confContent = []byte(os.ExpandEnv(string(confContent)))
	var options TestClusterConfig
	if err := yaml.Unmarshal(confContent, &options); err != nil {
		return nil, fmt.Errorf("failed to decode test cluster config %v", tag.Error(err))
	}

	options.FrontendAddress = TestFlags.FrontendAddr
	if options.ESConfig != nil {
		options.ESConfig.Indices[constants.VisibilityAppName] += uuid.New()
	}
	if options.Persistence.DBName == "" {
		options.Persistence.DBName = "test_" + pt.GenerateRandomDBName(10)
	}
	return &options, nil
}

// GetTestClusterConfigs return test cluster configs
func GetTestClusterConfigs(configFile string) ([]*TestClusterConfig, error) {
	if err := environment.SetupEnv(); err != nil {
		return nil, err
	}

	fileName := configFile
	if TestFlags.TestClusterConfigFile != "" {
		fileName = TestFlags.TestClusterConfigFile
	}

	confContent, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cluster config file %v: %v", fileName, err)
	}
	confContent = []byte(os.ExpandEnv(string(confContent)))

	var clusterConfigs []*TestClusterConfig
	if err := yaml.Unmarshal(confContent, &clusterConfigs); err != nil {
		return nil, fmt.Errorf("failed to decode test cluster config %v", tag.Error(err))
	}
	return clusterConfigs, nil
}

func (s *IntegrationBase) TearDownBaseSuite() {
	if s.TestCluster != nil {
		s.TestCluster.TearDownCluster()
		s.TestCluster = nil
		s.Engine = nil
		s.AdminClient = nil
	}
}

func (s *IntegrationBase) RegisterDomain(
	domain string,
	retentionDays int,
	historyArchivalStatus types.ArchivalStatus,
	historyArchivalURI string,
	visibilityArchivalStatus types.ArchivalStatus,
	visibilityArchivalURI string,
	activeClusters *types.ActiveClusters,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	isGlobalDomain := true
	if TestFlags.SQLPluginName == sqlite.PluginName {
		// There seems to be a bug in the SQLite plugin that causes integration tests to fail.
		// EnqueueMessage operation failed.  Error: sqlite3: database is locked
		isGlobalDomain = false
	}
	return s.Engine.RegisterDomain(ctx, &types.RegisterDomainRequest{
		Name:                                   domain,
		Description:                            domain,
		WorkflowExecutionRetentionPeriodInDays: int32(retentionDays),
		HistoryArchivalStatus:                  &historyArchivalStatus,
		HistoryArchivalURI:                     historyArchivalURI,
		VisibilityArchivalStatus:               &visibilityArchivalStatus,
		VisibilityArchivalURI:                  visibilityArchivalURI,
		ActiveClusters:                         activeClusters,
		IsGlobalDomain:                         isGlobalDomain,
	})
}

func (s *IntegrationBase) domainCacheRefresh() {
	s.TestClusterConfig.TimeSource.Advance(cache.DomainCacheRefreshInterval + time.Second)
	// this sleep is necessary to yield execution to other goroutines. not 100% guaranteed to work
	time.Sleep(2 * time.Second)
}

func (s *IntegrationBase) RandomizeStr(id string) string {
	return fmt.Sprintf("%v-%v", id, uuid.New())
}

func (s *IntegrationBase) printWorkflowHistory(domain string, execution *types.WorkflowExecution) {
	events := s.getHistory(domain, execution)
	history := &types.History{}
	history.Events = events
	PrettyPrintHistory(history, s.Logger)
}

func (s *IntegrationBase) getHistory(domain string, execution *types.WorkflowExecution) []*types.HistoryEvent {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:          domain,
		Execution:       execution,
		MaximumPageSize: 5, // Use small page size to force pagination code path
	})
	s.Require().NoError(err)

	events := historyResponse.History.Events
	for historyResponse.NextPageToken != nil {
		ctx, cancel := createContext()
		historyResponse, err = s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain:        domain,
			Execution:     execution,
			NextPageToken: historyResponse.NextPageToken,
		})
		cancel()
		s.Require().NoError(err)
		events = append(events, historyResponse.History.Events...)
	}

	return events
}

// To register archival domain we can't use frontend API as the retention period is set to 0 for testing,
// and request will be rejected by frontend. Here we make a call directly to persistence to register
// the domain.
func (s *IntegrationBase) registerArchivalDomain() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTestPersistenceTimeout)
	defer cancel()

	s.ArchivalDomainName = s.RandomizeStr("integration-archival-enabled-domain")
	currentClusterName := s.TestCluster.testBase.ClusterMetadata.GetCurrentClusterName()
	domainRequest := &persistence.CreateDomainRequest{
		Info: &persistence.DomainInfo{
			ID:     uuid.New(),
			Name:   s.ArchivalDomainName,
			Status: persistence.DomainStatusRegistered,
		},
		Config: &persistence.DomainConfig{
			Retention:                0,
			HistoryArchivalStatus:    types.ArchivalStatusEnabled,
			HistoryArchivalURI:       s.TestCluster.archiverBase.historyURI,
			VisibilityArchivalStatus: types.ArchivalStatusEnabled,
			VisibilityArchivalURI:    s.TestCluster.archiverBase.visibilityURI,
			BadBinaries:              types.BadBinaries{Binaries: map[string]*types.BadBinaryInfo{}},
		},
		ReplicationConfig: &persistence.DomainReplicationConfig{
			ActiveClusterName: currentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: currentClusterName},
			},
		},
		IsGlobalDomain:  false,
		FailoverVersion: constants.EmptyVersion,
	}
	response, err := s.TestCluster.testBase.DomainManager.CreateDomain(ctx, domainRequest)
	if err == nil {
		s.Logger.Info("Register domain succeeded",
			tag.WorkflowDomainName(s.ArchivalDomainName),
			tag.WorkflowDomainID(response.ID),
		)
	}
	return err
}

// PrettyPrintHistory prints history in human readable format
func PrettyPrintHistory(history *types.History, logger log.Logger) {
	data, err := json.MarshalIndent(history, "", "    ")

	if err != nil {
		logger.Error("Error serializing history: %v\n", tag.Error(err))
	}

	fmt.Println("******************************************")
	fmt.Println("History", tag.DetailInfo(string(data)))
	fmt.Println("******************************************")
}
