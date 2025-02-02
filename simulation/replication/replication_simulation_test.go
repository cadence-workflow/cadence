// Copyright (c) 2018 Uber Technologies, Inc.
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

/*
To run locally:

1. Pick a scenario from the existing config files simulation/replication/testdata/replication_simulation_${scenario}.yaml or add a new one

2. Run the scenario
`./simulation/replication/run.sh default`

Full test logs can be found at test.log file. Event json logs can be found at replication-simulator-output folder.
See the run.sh script for more details about how to parse events.
*/
package replication

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/client/frontend"
	grpcClient "github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/common/types"
)

func TestReplicationSimulation(t *testing.T) {
	flag.Parse()

	logf(t, "Starting Replication Simulation")

	logf(t, "Sleeping for 30 seconds to allow services to start/warmup")
	time.Sleep(30 * time.Second)

	// load config
	simCfg := mustLoadReplSimConf(t)

	// initialize cadence clients
	for clusterName := range simCfg.Clusters {
		simCfg.mustInitClientsFor(t, clusterName)
	}

	mustRegisterDomain(t, simCfg)

	// wait for domain data to be replicated.
	// TODO: make domain replication interval configurable and reduce it
	time.Sleep(10 * time.Second)

	// initialize workers
	for clusterName := range simCfg.Clusters {
		simCfg.mustInitWorkerFor(t, clusterName)
	}

	startTime := time.Now().UTC()
	logf(t, "Simulation start time: %v", startTime)
	g := &errgroup.Group{}
	var maxAt time.Duration
	for _, op := range simCfg.Operations {
		op := op
		if op.At > maxAt {
			maxAt = op.At
		}
		switch op.Type {
		case ReplicationSimulationOperationStartWorkflow:
			g.Go(func() error {
				return startWorkflow(t, op, simCfg, startTime)
			})
		case ReplicationSimulationOperationFailover:
			g.Go(func() error {
				return failover(t, op, simCfg, startTime)
			})
		case ReplicationSimulationOperationValidate:
			g.Go(func() error {
				return validate(t, op, simCfg, startTime)
			})
		default:
			require.Failf(t, "unknown operation type", "operation type: %s", op.Type)
		}
	}

	logf(t, "Waiting for all operations to complete.. Latest operation time is %v", startTime.Add(maxAt))
	g.Wait()

	// Print the test summary.
	// Don't change the start/end line format as it is used by scripts to parse the summary info
	executionTime := time.Since(startTime)
	testSummary := []string{}
	testSummary = append(testSummary, "Simulation Summary:")
	testSummary = append(testSummary, fmt.Sprintf("Simulation Duration: %v", executionTime))
	testSummary = append(testSummary, "End of Simulation Summary")
	fmt.Println(strings.Join(testSummary, "\n"))
}

func startWorkflow(
	t *testing.T,
	op *Operation,
	simCfg *ReplicationSimulationConfig,
	startTime time.Time,
) error {
	t.Helper()

	waitForOpTime(t, op, startTime)

	logf(t, "Starting workflow: %s", op.WorkflowID)

	// TODO: start workflow in specified cluster

	return nil
}

func failover(
	t *testing.T,
	op *Operation,
	simCfg *ReplicationSimulationConfig,
	startTime time.Time,
) error {
	t.Helper()

	waitForOpTime(t, op, startTime)

	logf(t, "Failing over to cluster: %s", op.NewActiveCluster)

	// TODO: perform failover

	return nil
}

func validate(
	t *testing.T,
	op *Operation,
	simCfg *ReplicationSimulationConfig,
	startTime time.Time,
) error {
	t.Helper()

	waitForOpTime(t, op, startTime)

	logf(t, "Performing valiaton")

	// TODO: perform validation

	return nil
}

func waitForOpTime(t *testing.T, op *Operation, startTime time.Time) {
	t.Helper()
	<-time.After(startTime.Add(op.At).Sub(time.Now().UTC()))
	logf(t, "Operation time (t + %ds) reached: %v", op.At, startTime.Add(op.At))
}

func mustLoadReplSimConf(t *testing.T) *ReplicationSimulationConfig {
	t.Helper()

	path := os.Getenv("REPLICATION_SIMULATION_CONFIG")
	if path == "" {
		path = defaultTestCase
	}
	confContent, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read config file")

	var cfg ReplicationSimulationConfig
	err = yaml.Unmarshal(confContent, &cfg)
	require.NoError(t, err, "failed to unmarshal config")

	logf(t, "Loaded config from path: %s", path)
	return &cfg
}

func logf(t *testing.T, msg string, args ...interface{}) {
	t.Helper()
	msg = time.Now().Format(time.RFC3339Nano) + "\t" + msg
	t.Logf(msg, args...)
}

func (s *ReplicationSimulationConfig) mustInitClientsFor(t *testing.T, clusterName string) {
	t.Helper()
	t.Helper()
	cluster, ok := s.Clusters[clusterName]
	require.True(t, ok, "Cluster %s not found in the config", clusterName)
	outbounds := transport.Outbounds{Unary: grpc.NewTransport().NewSingleOutbound(cluster.GRPCEndpoint)}
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: "cadence-client",
		Outbounds: yarpc.Outbounds{
			"cadence-frontend": outbounds,
		},
	})

	if err := dispatcher.Start(); err != nil {
		dispatcher.Stop()
		require.NoError(t, err, "failed to create outbound transport channel")
	}

	clientConfig := dispatcher.ClientConfig("cadence-frontend")
	cluster.FrontendClient = grpcClient.NewFrontendClient(
		apiv1.NewDomainAPIYARPCClient(clientConfig),
		apiv1.NewWorkflowAPIYARPCClient(clientConfig),
		apiv1.NewWorkerAPIYARPCClient(clientConfig),
		apiv1.NewVisibilityAPIYARPCClient(clientConfig),
	)

	cluster.AdminClient = grpcClient.NewAdminClient(adminv1.NewAdminAPIYARPCClient(clientConfig))
	logf(t, "Initialized clients for cluster %s", clusterName)
}

func mustRegisterDomain(t *testing.T, simCfg *ReplicationSimulationConfig) {
	logf(t, "Registering domain: %s", domainName)
	var clusters []*types.ClusterReplicationConfiguration
	for name := range simCfg.Clusters {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: name,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := simCfg.mustGetFrontendClient(t, simCfg.PrimaryCluster).RegisterDomain(ctx, &types.RegisterDomainRequest{
		Name:                                   domainName,
		Clusters:                               clusters,
		WorkflowExecutionRetentionPeriodInDays: 1,
		ActiveClusterName:                      simCfg.PrimaryCluster,
		IsGlobalDomain:                         true,
	})
	require.NoError(t, err, "failed to register domain")
	logf(t, "Registered domain: %s", domainName)
}

func (s *ReplicationSimulationConfig) mustGetFrontendClient(t *testing.T, clusterName string) frontend.Client {
	t.Helper()
	cluster, ok := s.Clusters[clusterName]
	require.True(t, ok, "Cluster %s not found in the config", clusterName)
	require.NotNil(t, cluster.FrontendClient, "Cluster %s frontend client not initialized", clusterName)
	return cluster.FrontendClient
}

func (s *ReplicationSimulationConfig) mustInitWorkerFor(t *testing.T, clusterName string) {
	t.Helper()
	cluster, ok := s.Clusters[clusterName]
	require.True(t, ok, "Cluster %s not found in the config", clusterName)

	config := zap.NewDevelopmentConfig()
	config.Level.SetLevel(zapcore.InfoLevel)

	logger, err := config.Build()
	require.NoError(t, err, "failed to create logger")

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: "worker",
		Outbounds: yarpc.Outbounds{
			"cadence-frontend": {Unary: grpc.NewTransport().NewSingleOutbound(cluster.GRPCEndpoint)},
		},
	})
	err = dispatcher.Start()
	require.NoError(t, err, "failed to create outbound transport channel")

	clientConfig := dispatcher.ClientConfig("cadence-frontend")

	cadenceClient := compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(clientConfig),
		apiv1.NewWorkflowAPIYARPCClient(clientConfig),
		apiv1.NewWorkerAPIYARPCClient(clientConfig),
		apiv1.NewVisibilityAPIYARPCClient(clientConfig),
	)

	workerOptions := worker.Options{
		Identity:     workerIdentityFor(clusterName),
		Logger:       logger,
		MetricsScope: tally.NewTestScope(tasklistName, map[string]string{"cluster": clusterName}),
	}

	w := worker.New(
		cadenceClient,
		domainName,
		tasklistName,
		workerOptions,
	)

	w.RegisterWorkflowWithOptions(testWorkflow, workflow.RegisterOptions{Name: workflowName})
	w.RegisterActivityWithOptions(testActivity, activity.RegisterOptions{Name: activityName})

	err = w.Start()
	require.NoError(t, err, "failed to start worker for cluster %s", clusterName)
	logf(t, "Started worker for cluster: %s", clusterName)
}

func workerIdentityFor(clusterName string) string {
	return fmt.Sprintf("worker-%s", clusterName)
}
