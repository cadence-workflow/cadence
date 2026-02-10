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
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	simTypes "github.com/uber/cadence/simulation/replication/types"
)

func TestReplicationSimulation(t *testing.T) {
	flag.Parse()

	simTypes.Logf(t, "Starting Replication Simulation")

	simTypes.Logf(t, "Sleeping for 30 seconds to allow services to start/warmup")
	time.Sleep(30 * time.Second)

	// load config
	simCfg, err := simTypes.LoadConfig()
	require.NoError(t, err, "failed to load config")

	// initialize replication simulation
	sim := simTypes.NewReplicationSimulation()

	// initialize cadence clients
	for clusterName := range simCfg.Clusters {
		simCfg.MustInitClientsFor(t, clusterName)
	}

	simTypes.Logf(t, "Registering domains")
	for domainName, domainCfg := range simCfg.Domains {
		simTypes.Logf(t, "Domain: %s", domainName)
		simCfg.MustRegisterDomain(t, domainName, domainCfg)
	}

	// wait for domain data to be replicated and workers to start.
	waitUntilWorkersReady(t)

	sort.Slice(simCfg.Operations, func(i, j int) bool {
		return simCfg.Operations[i].At < simCfg.Operations[j].At
	})

	startTime := time.Now().UTC()
	simTypes.Logf(t, "Simulation start time: %v", startTime)
	for i, op := range simCfg.Operations {
		waitForOpTime(t, op, startTime)
		var err error
		switch op.Type {
		case simTypes.ReplicationSimulationOperationStartWorkflow:
			err = startWorkflow(t, op, simCfg, sim)
		case simTypes.ReplicationSimulationOperationResetWorkflow:
			err = resetWorkflow(t, op, simCfg)
		case simTypes.ReplicationSimulationOperationChangeActiveClusters:
			err = changeActiveClusters(t, op, simCfg)
		case simTypes.ReplicationSimulationOperationValidate:
			err = validate(t, op, simCfg, sim)
		case simTypes.ReplicationSimulationOperationQueryWorkflow:
			err = queryWorkflow(t, op, simCfg, sim)
		case simTypes.ReplicationSimulationOperationSignalWithStartWorkflow:
			err = signalWithStartWorkflow(t, op, simCfg, sim)
		case simTypes.ReplicationSimulationOperationValidateWorkflowReplication:
			err = validateWorkflowReplication(t, op, simCfg)
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
	var testSummary []string
	testSummary = append(testSummary, "Simulation Summary:")
	testSummary = append(testSummary, fmt.Sprintf("Simulation Duration: %v", executionTime))
	testSummary = append(testSummary, "End of Simulation Summary")
	fmt.Println(strings.Join(testSummary, "\n"))
}

func startWorkflow(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
	sim *simTypes.ReplicationSimulation,
) error {
	t.Helper()

	simTypes.Logf(t, "Starting workflow: %s on domain %s on cluster: %s", op.WorkflowID, op.Domain, op.Cluster)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if op.WorkflowExecutionStartToCloseTimeout == 0 || op.WorkflowExecutionStartToCloseTimeout < op.WorkflowDuration {
		return fmt.Errorf("workflow execution start to close timeout must be specified and should be greater than workflow duration")
	}

	input := mustJSON(t, &simTypes.WorkflowInput{
		Duration:             op.WorkflowDuration,
		ActivityCount:        op.ActivityCount,
		ChildWorkflowID:      op.ChildWorkflowID,
		ChildWorkflowTimeout: op.ChildWorkflowTimeout,
	})
	workflowIDReusePolicy := types.WorkflowIDReusePolicyAllowDuplicate.Ptr()
	if op.WorkflowIDReusePolicy != nil {
		workflowIDReusePolicy = op.WorkflowIDReusePolicy
	}
	resp, err := simCfg.MustGetFrontendClient(t, op.Cluster).StartWorkflowExecution(ctx,
		&types.StartWorkflowExecutionRequest{
			RequestID:                           uuid.New(),
			Domain:                              op.Domain,
			WorkflowID:                          op.WorkflowID,
			WorkflowType:                        &types.WorkflowType{Name: op.WorkflowType},
			TaskList:                            &types.TaskList{Name: simTypes.TasklistName},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32((op.WorkflowExecutionStartToCloseTimeout).Seconds())),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(5),
			Input:                               input,
			WorkflowIDReusePolicy:               workflowIDReusePolicy,
			DelayStartSeconds:                   common.Int32Ptr(op.DelayStartSeconds),
			CronSchedule:                        op.CronSchedule,
			ActiveClusterSelectionPolicy:        op.ActiveClusterSelectionPolicy,
		})

	if err != nil {
		if op.Want.Error != "" {
			if strings.Contains(err.Error(), op.Want.Error) {
				simTypes.Logf(t, "Start workflow got expected error: %s on domain: %s on cluster: %s. Error: %s", op.WorkflowID, op.Domain, op.Cluster, err.Error())
				return nil
			}
			return fmt.Errorf("expected error: %s, but got: %s", op.Want.Error, err.Error())
		}
		return err
	}

	runID := resp.GetRunID()
	simTypes.Logf(t, "Started workflow: %s on domain: %s on cluster: %s. RunID: %s", op.WorkflowID, op.Domain, op.Cluster, runID)

	// Store RunID if runIDKey is specified
	if op.RunIDKey != "" {
		sim.StoreRunID(op.RunIDKey, runID)
		simTypes.Logf(t, "Stored RunID %s with key: %s", runID, op.RunIDKey)
	}

	return nil
}

func resetWorkflow(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
) error {
	t.Helper()

	simTypes.Logf(t, "Resetting workflow: %s on domain %s on cluster: %s", op.WorkflowID, op.Domain, op.Cluster)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := simCfg.MustGetFrontendClient(t, op.Cluster).DescribeWorkflowExecution(ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain: op.Domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: op.WorkflowID,
			},
		})

	if err != nil {
		return fmt.Errorf("failed to describe workflow %s: %w", op.WorkflowID, err)
	}

	simTypes.Logf(t, "DescribeWorkflowExecution workflowID: %s on domain: %s on cluster: %s. RunID: %s", op.WorkflowID, op.Domain, op.Cluster, resp.GetWorkflowExecutionInfo().Execution.RunID)

	resetResp, err := simCfg.MustGetFrontendClient(t, op.Cluster).ResetWorkflowExecution(ctx,
		&types.ResetWorkflowExecutionRequest{
			Domain: op.Domain,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: op.WorkflowID,
				RunID:      resp.GetWorkflowExecutionInfo().Execution.RunID,
			},
			Reason:                "resetting workflow",
			DecisionFinishEventID: op.EventID,
		})

	if err != nil {
		if op.Want.Error != "" {
			if strings.Contains(err.Error(), op.Want.Error) {
				simTypes.Logf(t, "Expected error: %s", op.Want.Error)
				return nil
			}
			return fmt.Errorf("expected error: %s, but got: %s", op.Want.Error, err.Error())
		}
		simTypes.Logf(t, err.Error())
		return err
	}

	if op.Want.Error != "" {
		return fmt.Errorf("expected error: %s, but got nil", op.Want.Error)
	}

	simTypes.Logf(t, "Reset workflowID: %s on domain: %s on cluster: %s. RunID: %s", op.WorkflowID, op.Domain, op.Cluster, resetResp.GetRunID())

	return nil
}

// changeActiveClusters modifies the active clusters for a domain
// It can be used to change the active cluster for a domain or a sub-set of the AttributeScopes for the domain
func changeActiveClusters(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	descResp, err := simCfg.MustGetFrontendClient(t, simCfg.PrimaryCluster).DescribeDomain(ctx, &types.DescribeDomainRequest{Name: common.StringPtr(op.Domain)})
	if err != nil {
		return fmt.Errorf("failed to describe domain %s: %w", op.Domain, err)
	}

	updateDomainRequest := &types.UpdateDomainRequest{
		Name:                     op.Domain,
		ActiveClusters:           &types.ActiveClusters{},
		FailoverTimeoutInSeconds: op.FailoverTimeout,
	}

	if op.NewActiveCluster != "" {
		fromCluster := descResp.ReplicationConfiguration.ActiveClusterName
		toCluster := op.NewActiveCluster
		simTypes.Logf(t, "Changing active clusters for domain %s from %s to %s", op.Domain, fromCluster, toCluster)
		updateDomainRequest.ActiveClusterName = &toCluster
	}

	if !op.NewClusterAttributes.IsEmpty() {
		updateDomainRequest.ActiveClusters.AttributeScopes = op.NewClusterAttributes.ToAttributeScopes()
		simTypes.Logf(t, "Changing cluster attributes for domain %s from %+v to %+v", op.Domain, descResp.ReplicationConfiguration.ActiveClusters, updateDomainRequest.ActiveClusters)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = simCfg.MustGetFrontendClient(t, simCfg.PrimaryCluster).UpdateDomain(ctx, updateDomainRequest)
	if err != nil {
		return fmt.Errorf("failed to update ActiveClusters, err: %w", err)
	}

	simTypes.Logf(t, "Completed change to ActiveClusters")
	return nil
}

func queryWorkflow(t *testing.T, op *simTypes.Operation, simCfg *simTypes.ReplicationSimulationConfig, sim *simTypes.ReplicationSimulation) error {
	t.Helper()

	simTypes.Logf(t, "Querying workflow: %s on domain %s on cluster: %s", op.WorkflowID, op.Domain, op.Cluster)

	frontendCl := simCfg.MustGetFrontendClient(t, op.Cluster)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consistencyLevel := types.QueryConsistencyLevelEventual.Ptr()
	if op.ConsistencyLevel == "strong" {
		consistencyLevel = types.QueryConsistencyLevelStrong.Ptr()
	}

	// Prepare workflow execution - use specific RunID if provided via runIDKey
	executionRequest := &types.WorkflowExecution{
		WorkflowID: op.WorkflowID,
	}
	if op.RunIDKey != "" {
		if runID, err := sim.GetRunID(op.RunIDKey); err == nil && runID != "" {
			executionRequest.RunID = runID
			simTypes.Logf(t, "Using stored RunID %s for query (key: %s)", runID, op.RunIDKey)
		} else {
			return fmt.Errorf("runIDKey %s specified but no RunID found in registry", op.RunIDKey)
		}
	}

	queryResp, err := frontendCl.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
		Domain:    op.Domain,
		Execution: executionRequest,
		Query: &types.WorkflowQuery{
			QueryType: op.Query,
		},
		QueryConsistencyLevel: consistencyLevel,
	})
	if err != nil {
		return err
	}

	got := mustParseJSON(t, queryResp.GetQueryResult())
	want := op.Want.QueryResult
	if !reflect.DeepEqual(want, got) {
		return fmt.Errorf("query result mismatch. want: %v (type: %T), got: %v (type: %T)", want, want, got, got)
	}

	simTypes.Logf(t, "Query workflow: %s on domain: %s on cluster: %s. Query result: %v", op.WorkflowID, op.Domain, op.Cluster, got)

	return nil
}

func signalWithStartWorkflow(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
	sim *simTypes.ReplicationSimulation,
) error {
	t.Helper()
	simTypes.Logf(t, "SignalWithStart workflow: %s on domain %s on cluster: %s", op.WorkflowID, op.Domain, op.Cluster)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	frontendCl := simCfg.MustGetFrontendClient(t, op.Cluster)

	workflowIDReusePolicy := types.WorkflowIDReusePolicyAllowDuplicate.Ptr()
	if op.WorkflowIDReusePolicy != nil {
		workflowIDReusePolicy = op.WorkflowIDReusePolicy
	}
	signalResp, err := frontendCl.SignalWithStartWorkflowExecution(ctx, &types.SignalWithStartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              op.Domain,
		WorkflowID:                          op.WorkflowID,
		WorkflowIDReusePolicy:               workflowIDReusePolicy,
		WorkflowType:                        &types.WorkflowType{Name: op.WorkflowType},
		SignalName:                          op.SignalName,
		SignalInput:                         mustJSON(t, op.SignalInput),
		TaskList:                            &types.TaskList{Name: simTypes.TasklistName},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(int32((op.WorkflowExecutionStartToCloseTimeout).Seconds())),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(5),
		Input:                               mustJSON(t, &simTypes.WorkflowInput{Duration: op.WorkflowDuration, ActivityCount: op.ActivityCount}),
		ActiveClusterSelectionPolicy:        op.ActiveClusterSelectionPolicy,
	})
	if err != nil {
		return err
	}

	runID := signalResp.GetRunID()
	simTypes.Logf(t, "SignalWithStart workflow: %s on domain %s on cluster: %s. RunID: %s", op.WorkflowID, op.Domain, op.Cluster, runID)

	// Store RunID if runIDKey is specified
	if op.RunIDKey != "" {
		sim.StoreRunID(op.RunIDKey, runID)
		simTypes.Logf(t, "Stored RunID %s with key: %s", runID, op.RunIDKey)
	}

	return nil
}

// validate performs validation based on given operation config.
// validate function does not fail the test via t.Fail (or require.X).
// It runs in separate goroutine. It should return an error.
func validate(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
	sim *simTypes.ReplicationSimulation,
) error {
	t.Helper()

	simTypes.Logf(t, "Validating workflow: %s on cluster: %s", op.WorkflowID, op.Cluster)

	consistencyLevel := types.QueryConsistencyLevelEventual.Ptr()
	if op.ConsistencyLevel == "strong" {
		consistencyLevel = types.QueryConsistencyLevelStrong.Ptr()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Prepare workflow execution - use specific RunID if provided via runIDKey
	executionRequest := &types.WorkflowExecution{
		WorkflowID: op.WorkflowID,
	}
	if op.RunIDKey != "" {
		if runID, err := sim.GetRunID(op.RunIDKey); err == nil && runID != "" {
			executionRequest.RunID = runID
			simTypes.Logf(t, "Using stored RunID %s for validation (key: %s)", runID, op.RunIDKey)
		} else {
			return fmt.Errorf("runIDKey %s specified but no RunID found in registry", op.RunIDKey)
		}
	}

	resp, err := simCfg.MustGetFrontendClient(t, op.Cluster).DescribeWorkflowExecution(ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain:                op.Domain,
			Execution:             executionRequest,
			QueryConsistencyLevel: consistencyLevel,
		})
	if err != nil {
		return err
	}

	workflowStatus := resp.GetWorkflowExecutionInfo().GetCloseStatus()
	workflowCloseTime := resp.GetWorkflowExecutionInfo().GetCloseTime()
	switch op.Want.Status {
	case "completed":
		// Validate workflow completed
		if workflowStatus != types.WorkflowExecutionCloseStatusCompleted || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not completed. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	case "failed":
		// Validate workflow failed
		if workflowStatus != types.WorkflowExecutionCloseStatusFailed || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not failed. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	case "canceled":
		// Validate workflow canceled
		if workflowStatus != types.WorkflowExecutionCloseStatusCanceled || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not canceled. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	case "terminated":
		// Validate workflow terminated
		if workflowStatus != types.WorkflowExecutionCloseStatusTerminated || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not terminated. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	case "continued-as-new":
		// Validate workflow continued as new
		if workflowStatus != types.WorkflowExecutionCloseStatusContinuedAsNew || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not continued as new. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	case "timed-out":
		// Validate workflow timed out
		if workflowStatus != types.WorkflowExecutionCloseStatusTimedOut || workflowCloseTime == 0 {
			return fmt.Errorf("workflow %s not timed out. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	default:
		// Validate workflow is running
		if workflowCloseTime != 0 {
			return fmt.Errorf("workflow %s not running. status: %s, close time: %v", op.WorkflowID, workflowStatus, time.Unix(0, workflowCloseTime))
		}
	}

	simTypes.Logf(t, "Validated workflow: %s on cluster: %s. Status: %s, CloseTime: %v", op.WorkflowID, op.Cluster, resp.GetWorkflowExecutionInfo().GetCloseStatus(), time.Unix(0, resp.GetWorkflowExecutionInfo().GetCloseTime()))

	// Get history to validate the worker identity that started and completed the workflow
	// Some workflows start in cluster0 and complete in cluster1. This is to validate that
	var runID string
	if op.RunIDKey != "" {
		runID, err = sim.GetRunID(op.RunIDKey)
		if err != nil {
			return err
		}
	}
	history, err := getAllHistory(t, simCfg, op.Cluster, op.Domain, op.WorkflowID, runID)
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
	if op.Want.StartedByWorkersInCluster != "" && startedWorker != simTypes.WorkerIdentityFor(op.Want.StartedByWorkersInCluster, op.Domain) {
		return fmt.Errorf("workflow %s started by worker %s, expected %s", op.WorkflowID, startedWorker, simTypes.WorkerIdentityFor(op.Want.StartedByWorkersInCluster, op.Domain))
	}

	completedWorker, err := lastDecisionTaskWorker(history)
	if err != nil {
		return err
	}

	if op.Want.CompletedByWorkersInCluster != "" && completedWorker != simTypes.WorkerIdentityFor(op.Want.CompletedByWorkersInCluster, op.Domain) {
		return fmt.Errorf("workflow %s completed by worker %s, expected %s", op.WorkflowID, completedWorker, simTypes.WorkerIdentityFor(op.Want.CompletedByWorkersInCluster, op.Domain))
	}

	return nil
}

func validateWorkflowReplication(
	t *testing.T,
	op *simTypes.Operation,
	simCfg *simTypes.ReplicationSimulationConfig,
) error {
	t.Helper()

	simTypes.Logf(t, "Describing workflow: %s on domain %s on cluster: %s", op.WorkflowID, op.Domain, op.Cluster)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := simCfg.MustGetFrontendClient(t, op.SourceCluster).DescribeWorkflowExecution(ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain: op.Domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: op.WorkflowID,
			},
		})
	if err != nil {
		return err
	}

	sourceClusterWorkflowExecution := resp.GetWorkflowExecutionInfo().GetExecution()

	simTypes.Logf(t, "Described workflow: %s on domain: %s on cluster: %s. Status: %s, CloseTime: %v", op.WorkflowID, op.Domain, op.Cluster, resp.GetWorkflowExecutionInfo().GetCloseStatus(), time.Unix(0, resp.GetWorkflowExecutionInfo().GetCloseTime()))

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err = simCfg.MustGetFrontendClient(t, op.TargetCluster).DescribeWorkflowExecution(ctx,
		&types.DescribeWorkflowExecutionRequest{
			Domain: op.Domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: op.WorkflowID,
			},
		})
	if err != nil {
		return err
	}

	targetClusterWorkflowExecution := resp.GetWorkflowExecutionInfo().GetExecution()

	if !reflect.DeepEqual(sourceClusterWorkflowExecution, targetClusterWorkflowExecution) {
		return fmt.Errorf("workflow execution info mismatch between source cluster %s and target cluster %s for workflow %s. \nSource: %+v\nTarget: %+v", op.SourceCluster, op.TargetCluster, op.WorkflowID, *sourceClusterWorkflowExecution, *targetClusterWorkflowExecution)
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

func waitForOpTime(t *testing.T, op *simTypes.Operation, startTime time.Time) {
	t.Helper()
	d := startTime.Add(op.At).Sub(time.Now().UTC())
	if d > 0 {
		simTypes.Logf(t, "Waiting for next operation time (t + %ds). Will sleep for %ds", int(op.At.Seconds()), int(d.Seconds()))
		<-time.After(d)
	}

	simTypes.Logf(t, "Operation time (t + %ds) reached: %v", int(op.At.Seconds()), startTime.Add(op.At))
}

func getAllHistory(t *testing.T, simCfg *simTypes.ReplicationSimulationConfig, clusterName, domainName, wfID, runID string) ([]types.HistoryEvent, error) {
	frontendCl := simCfg.MustGetFrontendClient(t, clusterName)
	var nextPageToken []byte
	var history []types.HistoryEvent

	executionRequest := &types.WorkflowExecution{
		WorkflowID: wfID,
	}
	if runID != "" {
		executionRequest.RunID = runID
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		response, err := frontendCl.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain:                 domainName,
			Execution:              executionRequest,
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

func mustParseJSON(t *testing.T, v []byte) any {
	var result interface{}
	err := json.Unmarshal(v, &result)
	require.NoError(t, err, "failed to unmarshal from json")
	return result
}

func mustJSON(t *testing.T, v interface{}) []byte {
	data, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal to json")
	return data
}

func waitUntilWorkersReady(t *testing.T) {
	// workers expose :6060/health endpoint. Poll on them to check if they are healthy
	simTypes.Logf(t, "Waiting for workers to start and report healthy")
	workerEndpoints := []string{
		"http://cadence-worker0:6060/health",
		"http://cadence-worker1:6060/health",
	}

	client := &http.Client{Timeout: 1 * time.Second}

	for {
		allHealthy := true
		lastStatus := make(map[string]int, len(workerEndpoints))
		lastBody := make(map[string]string, len(workerEndpoints))
		lastErr := make(map[string]error, len(workerEndpoints))
		for _, endpoint := range workerEndpoints {
			resp, err := client.Get(endpoint)
			if err != nil {
				lastErr[endpoint] = err
				allHealthy = false
				break
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			lastStatus[endpoint] = resp.StatusCode
			if len(body) > 0 {
				lastBody[endpoint] = strings.TrimSpace(string(body))
			}
			if resp.StatusCode != http.StatusOK {
				allHealthy = false
				break
			}
		}

		if allHealthy {
			break
		}

		// Include details so CI logs are actionable (DNS vs connection vs 503 readiness).
		for _, endpoint := range workerEndpoints {
			if err := lastErr[endpoint]; err != nil {
				simTypes.Logf(t, "Workers are not reporting healthy yet (%s error: %v). Sleep for 2s and try again", endpoint, err)
			} else if status, ok := lastStatus[endpoint]; ok {
				if body, ok := lastBody[endpoint]; ok && body != "" {
					simTypes.Logf(t, "Workers are not reporting healthy yet (%s status: %d body: %s). Sleep for 2s and try again", endpoint, status, body)
				} else {
					simTypes.Logf(t, "Workers are not reporting healthy yet (%s status: %d). Sleep for 2s and try again", endpoint, status)
				}
			}
		}
		time.Sleep(2 * time.Second)
	}

	simTypes.Logf(t, "All workers are healthy")
}
