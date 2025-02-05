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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pborman/uuid"
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
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/client/frontend"
	grpcClient "github.com/uber/cadence/client/wrappers/grpc"
	"github.com/uber/cadence/common"
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
	time.Sleep(10 * time.Second)

	// initialize workers
	for clusterName := range simCfg.Clusters {
		simCfg.mustInitWorkerFor(t, clusterName)
	}

	sort.Slice(simCfg.Operations, func(i, j int) bool {
		return simCfg.Operations[i].At < simCfg.Operations[j].At
	})

	startTime := time.Now().UTC()
	logf(t, "Simulation start time: %v", startTime)
	for i, op := range simCfg.Operations {
		op := op
		waitForOpTime(t, op, startTime)
		var err error
		switch op.Type {
		case ReplicationSimulationOperationStartWorkflow:
			err = startWorkflow(t, op, simCfg)
		case ReplicationSimulationOperationFailover:
			err = failover(t, op, simCfg)
		case ReplicationSimulationOperationValidate:
			err = validate(t, op, simCfg)
		default:
			require.Failf(t, "unknown operation type", "operation type: %s", op.Type)
		}

		if err != nil {
			t.Fatalf("Operation %d failed: %v", i, err)
		}
	}

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
) error {
	t.Helper()

	logf(t, "Starting workflow: %s on cluster: %s", op.WorkflowID, op.Cluster)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := simCfg.mustGetFrontendClient(t, op.Cluster).StartWorkflowExecution(ctx,
		&types.StartWorkflowExecutionRequest{
			RequestID:                           uuid.New(),
			Domain:                              domainName,
			WorkflowID:                          op.WorkflowID,
			WorkflowType:                        &types.WorkflowType{Name: workflowName},
			TaskList:                            &types.TaskList{Name: tasklistName},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32((op.WorkflowDuration + 10*time.Second).Seconds())),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
			Input:                               mustJSON(t, &WorkflowInput{Duration: op.WorkflowDuration}),
		})

	if err != nil {
		return err
	}

	logf(t, "Started workflow: %s on cluster: %s. RunID: %s", op.WorkflowID, op.Cluster, resp.GetRunID())

	return nil
}

func failover(
	t *testing.T,
	op *Operation,
	simCfg *ReplicationSimulationConfig,
) error {
	t.Helper()

	logf(t, "Failing over to cluster: %s", op.NewActiveCluster)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	descResp, err := simCfg.mustGetFrontendClient(t, simCfg.PrimaryCluster).DescribeDomain(ctx, &types.DescribeDomainRequest{Name: common.StringPtr(domainName)})
	if err != nil {
		return fmt.Errorf("failed to describe domain %s: %w", domainName, err)
	}

	fromCluster := descResp.ReplicationConfiguration.ActiveClusterName
	toCluster := op.NewActiveCluster

	if fromCluster == toCluster {
		return fmt.Errorf("domain %s is already active in cluster %s so cannot perform failover", domainName, toCluster)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = simCfg.mustGetFrontendClient(t, simCfg.PrimaryCluster).UpdateDomain(ctx,
		&types.UpdateDomainRequest{
			Name:              domainName,
			ActiveClusterName: &toCluster,
		})
	if err != nil {
		return fmt.Errorf("failed to update ActiveClusterName, err: %w", err)
	}

	logf(t, "Failed over from %s to %s", fromCluster, toCluster)
	return nil
}

// validate performs validation based on given operation config.
// validate function does not fail the test via t.Fail (or require.X).
// It runs in separate goroutine. It should return an error.
func validate(
	t *testing.T,
	op *Operation,
	simCfg *ReplicationSimulationConfig,
) error {
	t.Helper()

	logf(t, "Validating workflow: %s on cluster: %s", op.WorkflowID, op.Cluster)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := simCfg.mustGetFrontendClient(t, op.Cluster).DescribeWorkflowExecution(ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain: domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: op.WorkflowID,
			},
		})
	if err != nil {
		return err
	}

	// Validate workflow completed
	if resp.GetWorkflowExecutionInfo().GetCloseStatus() != types.WorkflowExecutionCloseStatusCompleted {
		return fmt.Errorf("workflow %s not completed. status: %s", op.WorkflowID, resp.GetWorkflowExecutionInfo().GetCloseStatus())
	}

	logf(t, "Validated workflow: %s on cluster: %s. Status: %s, CloseTime: %v", op.WorkflowID, op.Cluster, resp.GetWorkflowExecutionInfo().GetCloseStatus(), time.Unix(0, resp.GetWorkflowExecutionInfo().GetCloseTime()))

	// Get history to validate the worker identity that started and completed the workflow
	// Some workflows start in cluster0 and complete in cluster1. This is to validate that
	history, err := getAllHistory(t, simCfg, op.Cluster, op.WorkflowID)
	if err != nil {
		return err
	}

	if len(history) == 0 {
		return fmt.Errorf("no history events found for workflow %s", op.WorkflowID)
	}

	startedWorker, err := firstDecisionTaskWorker(history)
	if err != nil {
		return err
	}
	if op.Want.StartedByWorkersInCluster != "" && startedWorker != workerIdentityFor(op.Want.StartedByWorkersInCluster) {
		return fmt.Errorf("workflow %s started by worker %s, expected %s", op.WorkflowID, startedWorker, workerIdentityFor(op.Want.StartedByWorkersInCluster))
	}

	completedWorker, err := lastDecisionTaskWorker(history)
	if err != nil {
		return err
	}

	if op.Want.CompletedByWorkersInCluster != "" && completedWorker != workerIdentityFor(op.Want.CompletedByWorkersInCluster) {
		return fmt.Errorf("workflow %s completed by worker %s, expected %s", op.WorkflowID, completedWorker, workerIdentityFor(op.Want.CompletedByWorkersInCluster))
	}

	return nil
}

func firstDecisionTaskWorker(history []types.HistoryEvent) (string, error) {
	for _, event := range history {
		if event.GetEventType() == types.EventTypeDecisionTaskCompleted {
			return event.GetDecisionTaskCompletedEventAttributes().Identity, nil
		}
	}
	return "", fmt.Errorf("failed to find first decision task worker because there's no DecisionTaskCompleted event found in history")
}

func lastDecisionTaskWorker(history []types.HistoryEvent) (string, error) {
	for i := len(history) - 1; i >= 0; i-- {
		event := history[i]
		if event.GetEventType() == types.EventTypeDecisionTaskCompleted {
			return event.GetDecisionTaskCompletedEventAttributes().Identity, nil
		}
	}
	return "", fmt.Errorf("failed to find lastDecisionTaskWorker because there's no DecisionTaskCompleted event found in history")
}

func waitForOpTime(t *testing.T, op *Operation, startTime time.Time) {
	t.Helper()
	d := startTime.Add(op.At).Sub(time.Now().UTC())
	if d > 0 {
		logf(t, "Waiting for next operation time (t + %ds). Will sleep for %ds", int(op.At.Seconds()), int(d.Seconds()))
		<-time.After(d)
	}

	logf(t, "Operation time (t + %ds) reached: %v", int(op.At.Seconds()), startTime.Add(op.At))
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

func getAllHistory(t *testing.T, simCfg *ReplicationSimulationConfig, clusterName, wfID string) ([]types.HistoryEvent, error) {
	frontendCl := simCfg.mustGetFrontendClient(t, clusterName)
	var nextPageToken []byte
	var history []types.HistoryEvent
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		response, err := frontendCl.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: wfID,
			},
			MaximumPageSize:        1000,
			NextPageToken:          nextPageToken,
			WaitForNewEvent:        false,
			HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
			SkipArchival:           true,
		})
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to get history: %w", err)
		}

		for _, event := range response.GetHistory().GetEvents() {
			if event != nil {
				history = append(history, *event)
			}
		}

		if response.NextPageToken == nil {
			return history, nil
		}

		nextPageToken = response.NextPageToken
		time.Sleep(10 * time.Millisecond) // sleep to avoid throttling
	}
}

func mustJSON(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal to json")
	return data
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
