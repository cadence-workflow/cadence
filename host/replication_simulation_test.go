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

//go:build !race && replicationsim
// +build !race,replicationsim

/*
To run locally:

1. Pick a scenario from the existing config files host/testdata/replication_simulation_.*.yaml or add a new one

2. Run the scenario
`./scripts/run_replication_simulator.sh default`

Full test logs can be found at test.log file. Event json logs can be found at replication-simulator-output folder.
See the run_replication_simulator.sh script for more details about how to parse events.
*/
package host

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	grpcClient "github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/common/types"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"
	"gopkg.in/yaml.v2"
)

const (
	defaultTestCase = "testdata/replication_simulation_default.yaml"
	domainName      = "test-domain"
)

type ReplicationSimulationConfig struct {
	Clusters map[string]*Cluster `yaml:"clusters"`

	PrimaryCluster string `yaml:"primaryCluster"`
}

type Cluster struct {
	GRPCEndpoint string `yaml:"grpcEndpoint"`

	AdminClient    admin.Client    `yaml:"-"`
	FrontendClient frontend.Client `yaml:"-"`
}

func TestReplicationSimulation(t *testing.T) {
	flag.Parse()

	logf(t, "Starting Replication Simulation")
	startTime := time.Now()

	simCfg := mustLoadReplSimConf(t)

	for _, c := range simCfg.Clusters {
		mustInitClients(t, c)
	}

	mustRegisterDomain(t, simCfg)

	// TODO: implement rest of the simulation
	// - override dynamicconfigs in config/dynamicconfig/replication_simulation.yaml as needed
	// - run workflows based on given config
	// - do failover at specified time based on config
	// - validate workflows finished successfully

	// Print the test summary.
	// Don't change the start/end line format as it is used by scripts to parse the summary info
	executionTime := time.Since(startTime)
	testSummary := []string{}
	testSummary = append(testSummary, "Simulation Summary:")
	testSummary = append(testSummary, fmt.Sprintf("Simulation Duration: %v", executionTime))
	testSummary = append(testSummary, "End of Simulation Summary")
	fmt.Println(strings.Join(testSummary, "\n"))
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

func mustInitClients(t *testing.T, c *Cluster) {
	t.Helper()
	outbounds := transport.Outbounds{Unary: grpc.NewTransport().NewSingleOutbound(c.GRPCEndpoint)}
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
	c.FrontendClient = grpcClient.NewFrontendClient(
		apiv1.NewDomainAPIYARPCClient(clientConfig),
		apiv1.NewWorkflowAPIYARPCClient(clientConfig),
		apiv1.NewWorkerAPIYARPCClient(clientConfig),
		apiv1.NewVisibilityAPIYARPCClient(clientConfig),
	)

	c.AdminClient = grpcClient.NewAdminClient(adminv1.NewAdminAPIYARPCClient(clientConfig))
}

func mustRegisterDomain(t *testing.T, simCfg *ReplicationSimulationConfig) {
	var clusters []*types.ClusterReplicationConfiguration
	for name := range simCfg.Clusters {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: name,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := simCfg.mustGetPrimaryFrontendClient(t).RegisterDomain(ctx, &types.RegisterDomainRequest{
		Name:                                   domainName,
		Clusters:                               clusters,
		WorkflowExecutionRetentionPeriodInDays: 1,
		ActiveClusterName:                      simCfg.PrimaryCluster,
		IsGlobalDomain:                         true,
	})
	require.NoError(t, err, "failed to register domain")
}

func (s *ReplicationSimulationConfig) mustGetPrimaryFrontendClient(t *testing.T) frontend.Client {
	t.Helper()
	primaryCluster, ok := s.Clusters[s.PrimaryCluster]
	require.True(t, ok, "Primary cluster not found in the config")
	require.NotNil(t, primaryCluster.FrontendClient, "Primary cluster frontend client not initialized")
	return primaryCluster.FrontendClient
}

func (s *ReplicationSimulationConfig) mustGetPrimaryAdminClient(t *testing.T) admin.Client {
	t.Helper()
	primaryCluster, ok := s.Clusters[s.PrimaryCluster]
	require.True(t, ok, "Primary cluster not found in the config")
	require.NotNil(t, primaryCluster.AdminClient, "Primary cluster admin client not initialized")
	return primaryCluster.AdminClient
}
