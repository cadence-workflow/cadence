package history

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/cadence/worker"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/transport/grpc"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host"
	"github.com/uber/cadence/simulation/history/workflow"
)

const (
	defaultTestCase = "testdata/history_simulation_default.yaml"
)

type HistorySimulationSuite struct {
	*require.Assertions
	*host.IntegrationBase
	wfService workflowserviceclient.Interface
	wfClient  client.Client
	worker    worker.Worker
	taskList  string
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
		ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
		SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
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
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	time.Sleep(2 * time.Second)

	s.wfService = s.buildServiceClient()
	s.wfClient = client.NewClient(s.wfService, s.DomainName, nil)

	s.taskList = "history-simulation-tasklist"
	s.worker = worker.New(s.wfService, s.DomainName, s.taskList, worker.Options{})
	workflow.RegisterWorker(s.worker)
	if err := s.worker.Start(); err != nil {
		s.Logger.Fatal("Error when start worker", tag.Error(err))
	} else {
		s.Logger.Info("Worker started")
	}
}

func (s *HistorySimulationSuite) buildServiceClient() workflowserviceclient.Interface {
	cadenceClientName := "cadence-client"
	hostPort := "127.0.0.1:7114" // use grpc port
	if host.TestFlags.FrontendAddr != "" {
		hostPort = host.TestFlags.FrontendAddr
	}
	ch := grpc.NewTransport(
		grpc.ServerMaxRecvMsgSize(32*1024*1024),
		grpc.ClientMaxRecvMsgSize(32*1024*1024),
	)

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: cadenceClientName,
		Outbounds: yarpc.Outbounds{
			service.Frontend: {Unary: ch.NewSingleOutbound(hostPort)},
		},
	})
	if err := dispatcher.Start(); err != nil {
		s.Logger.Fatal("Failed to create outbound transport channel", tag.Error(err))
	}
	cc := dispatcher.ClientConfig(service.Frontend)
	return compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(cc),
		apiv1.NewWorkflowAPIYARPCClient(cc),
		apiv1.NewWorkerAPIYARPCClient(cc),
		apiv1.NewVisibilityAPIYARPCClient(cc),
	)
}

func (s *HistorySimulationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *HistorySimulationSuite) TearDownSuite() {
	s.worker.Stop()
	// Sleep for a while to ensure all metrics are emitted/scraped by prometheus
	time.Sleep(5 * time.Second)
	s.TearDownBaseSuite()
}

func (s *HistorySimulationSuite) TestHistorySimulation() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var runs []client.WorkflowRun
	for i := 0; i < 100; i++ {
		// set a short timeout so that timer tasks can be executed before complete
		workflowOptions := client.StartWorkflowOptions{
			TaskList:                        s.taskList,
			ExecutionStartToCloseTimeout:    120 * time.Second,
			DecisionTaskStartToCloseTimeout: 5 * time.Second,
		}
		we, err := s.wfClient.ExecuteWorkflow(ctx, workflowOptions, workflow.NoopWorkflow)
		if err != nil {
			s.Logger.Fatal("Start workflow with err", tag.Error(err))
		}
		s.NotNil(we)
		s.True(we.GetRunID() != "")
		s.Logger.Info("successfully start a workflow", tag.WorkflowID(we.GetID()), tag.WorkflowRunID(we.GetRunID()))
		runs = append(runs, we)
	}
	for _, we := range runs {
		s.NoError(we.Get(ctx, nil))
	}
	time.Sleep(120 * time.Second)
}
