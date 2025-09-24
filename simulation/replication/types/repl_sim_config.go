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

package types

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	adminv1 "github.com/uber/cadence-idl/go/proto/admin/v1"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/transport/grpc"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	grpcClient "github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/common/types"
)

type ReplicationSimulationOperation string

const (
	ReplicationSimulationOperationStartWorkflow               ReplicationSimulationOperation = "start_workflow"
	ReplicationSimulationOperationResetWorkflow               ReplicationSimulationOperation = "reset_workflow"
	ReplicationSimulationOperationChangeActiveClusters        ReplicationSimulationOperation = "change_active_clusters"
	ReplicationSimulationOperationValidate                    ReplicationSimulationOperation = "validate"
	ReplicationSimulationOperationQueryWorkflow               ReplicationSimulationOperation = "query_workflow"
	ReplicationSimulationOperationSignalWithStartWorkflow     ReplicationSimulationOperation = "signal_with_start_workflow"
	ReplicationSimulationOperationMigrateDomainToActiveActive ReplicationSimulationOperation = "migrate_domain_to_active_active"
	ReplicationSimulationOperationValidateWorkflowReplication ReplicationSimulationOperation = "validate_workflow_replication"
)

type ReplicationSimulationConfig struct {
	// Clusters is the map of all clusters
	Clusters map[string]*Cluster `yaml:"clusters"`

	// PrimaryCluster is used for domain registration
	PrimaryCluster string `yaml:"primaryCluster"`

	Domains map[string]ReplicationDomainConfig `yaml:"domains"`

	Operations []*Operation `yaml:"operations"`
}

type ReplicationDomainConfig struct {
	ActiveClusterName string `yaml:"activeClusterName"`

	ActiveClustersByRegion map[string]string `yaml:"activeClustersByRegion"`
}

type Operation struct {
	Type          ReplicationSimulationOperation `yaml:"op"`
	At            time.Duration                  `yaml:"at"`
	Cluster       string                         `yaml:"cluster"`
	SourceCluster string                         `yaml:"sourceCluster"`
	TargetCluster string                         `yaml:"targetCluster"`

	WorkflowType                         string        `yaml:"workflowType"`
	WorkflowID                           string        `yaml:"workflowID"`
	WorkflowExecutionStartToCloseTimeout time.Duration `yaml:"workflowExecutionStartToCloseTimeout"`
	WorkflowDuration                     time.Duration `yaml:"workflowDuration"`
	ActivityCount                        int           `yaml:"activityCount"`
	DelayStartSeconds                    int32         `yaml:"delayStartSeconds"`
	CronSchedule                         string        `yaml:"cronSchedule"`

	Query            string `yaml:"query"`
	ConsistencyLevel string `yaml:"consistencyLevel"`

	SignalName  string `yaml:"signalName"`
	SignalInput any    `yaml:"signalInput"`

	EventID int64 `yaml:"eventID"`

	Domain                    string            `yaml:"domain"`
	NewActiveCluster          string            `yaml:"newActiveCluster"`
	NewActiveClustersByRegion map[string]string `yaml:"newActiveClustersByRegion"`
	FailoverTimeout           *int32            `yaml:"failoverTimeoutSec"`

	// RunIDKey specifies a key to store/retrieve RunID for this operation
	RunIDKey string `yaml:"runIDKey"`

	Want Validation `yaml:"want"`
}

type Validation struct {
	Status                      string `yaml:"status"`
	StartedByWorkersInCluster   string `yaml:"startedByWorkersInCluster"`
	CompletedByWorkersInCluster string `yaml:"completedByWorkersInCluster"`
	Error                       string `yaml:"error"`
	QueryResult                 any    `yaml:"queryResult"`
}

type Cluster struct {
	GRPCEndpoint string `yaml:"grpcEndpoint"`

	AdminClient    admin.Client    `yaml:"-"`
	FrontendClient frontend.Client `yaml:"-"`
}

func LoadConfig() (*ReplicationSimulationConfig, error) {
	path := os.Getenv("REPLICATION_SIMULATION_CONFIG")
	if path == "" {
		path = DefaultTestCase
	}
	confContent, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ReplicationSimulationConfig
	err = yaml.Unmarshal(confContent, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	fmt.Printf("Loaded config from path: %s\n", path)
	return &cfg, nil
}

func (s *ReplicationSimulationConfig) MustGetFrontendClient(t *testing.T, clusterName string) frontend.Client {
	t.Helper()
	cluster, ok := s.Clusters[clusterName]
	require.True(t, ok, "Cluster %s not found in the config", clusterName)
	require.NotNil(t, cluster.FrontendClient, "Cluster %s frontend client not initialized", clusterName)
	return cluster.FrontendClient
}

func (s *ReplicationSimulationConfig) MustInitClientsFor(t *testing.T, clusterName string) {
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
	Logf(t, "Initialized clients for cluster %s", clusterName)
}

func (s *ReplicationSimulationConfig) IsActiveActiveDomain(domainName string) bool {
	return len(s.Domains[domainName].ActiveClustersByRegion) > 0
}

func (s *ReplicationSimulationConfig) MustRegisterDomain(t *testing.T, domainName string, domainCfg ReplicationDomainConfig) {
	Logf(t, "Registering domain: %s", domainName)

	var clusters []*types.ClusterReplicationConfiguration
	for name := range s.Clusters {
		clusters = append(clusters, &types.ClusterReplicationConfiguration{
			ClusterName: name,
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := &types.RegisterDomainRequest{
		Name:                                   domainName,
		Clusters:                               clusters,
		WorkflowExecutionRetentionPeriodInDays: 1,
		IsGlobalDomain:                         true,
	}

	if len(domainCfg.ActiveClusterName) > 0 {
		req.ActiveClusterName = domainCfg.ActiveClusterName
	} else if len(domainCfg.ActiveClustersByRegion) > 0 {
		req.ActiveClustersByRegion = domainCfg.ActiveClustersByRegion
	} else {
		require.Fail(t, "activeClusterName or activeClustersByRegion is required but missing for domain %s", domainName)
	}

	err := s.MustGetFrontendClient(t, s.PrimaryCluster).RegisterDomain(ctx, req)

	if err != nil {
		if _, ok := err.(*types.DomainAlreadyExistsError); !ok {
			require.NoError(t, err, "failed to register domain")
		} else {
			Logf(t, "Domains already exists: %s", domainName)
		}
		return
	}

	Logf(t, "Registered domain: %s", domainName)
}
