package history

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host"
)

const (
	defaultTestCase = "testdata/history_simulation_default.yaml"
)

type HistorySimulationSuite struct {
	*require.Assertions
	*host.IntegrationBase
}

func TestHistorySimulation(t *testing.T) {
	flag.Parse()

	confPath := os.Getenv("HISTORY_SIMULATION_CONFIG")
	if confPath == "" {
		confPath = defaultTestCase
	}
	clusterConfig, err := host.GetTestClusterConfig(confPath)
	if err != nil {
		t.Fatalf("failed creating cluster config from %s, err: %v", confPath, err)
	}
	testCluster := host.NewPersistenceTestCluster(t, clusterConfig)

	s := new(HistorySimulationSuite)
	params := host.IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = host.NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *HistorySimulationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := host.NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicproperties.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicproperties.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.DefaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := host.NewCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))
	s.SecondaryDomainName = s.RandomizeStr("unused-test-domain")
	s.Require().NoError(s.RegisterDomain(s.SecondaryDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, ""))

	time.Sleep(2 * time.Second)
}

func (s *HistorySimulationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *HistorySimulationSuite) TearDownSuite() {
	// Sleep for a while to ensure all metrics are emitted/scraped by prometheus
	time.Sleep(5 * time.Second)
	s.TearDownBaseSuite()
}

func (s *HistorySimulationSuite) TestHistorySimulation() {

}
