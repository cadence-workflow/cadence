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

//go:build !race && asyncwfintegration
// +build !race,asyncwfintegration

/*
This is the single-region integration test for the history-backed async workflow
queue. Unlike the kafka-backed variant (async_wf_test.go) it needs no external
message broker: the frontend producer enqueues via the history
EnqueueAsyncWorkflowMessage RPC, and the history service's per-shard consumer
processes the messages from the async_workflow_queue* Cassandra tables.

To run locally, run against the same docker cluster used by the kafka async test
(the history queue only needs the Cassandra dependency, kafka is unused):

	docker compose -f docker/github_actions/docker-compose-local-async-wf.yml run --rm integration-test-async-wf

or, more directly, against a running Cassandra:

	go test -tags asyncwfintegration -run TestAsyncWFHistoryIntegrationSuite ./host/...
*/
package host

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	"github.com/uber/cadence/common/types"

	_ "github.com/uber/cadence/common/asyncworkflow/queue/history" // needed to load history asyncworkflow queue provider
)

type AsyncWFHistoryIntegrationSuite struct {
	*require.Assertions
	*IntegrationBase
}

func TestAsyncWFHistoryIntegrationSuite(t *testing.T) {
	flag.Parse()

	confPath := "testdata/integration_async_wf_with_history_cluster.yaml"
	clusterConfig, err := GetTestClusterConfig(confPath)
	if err != nil {
		t.Fatalf("failed creating cluster config from %s, err: %v", confPath, err)
	}

	clusterConfig.TimeSource = clock.NewMockedTimeSource()
	clusterConfig.FrontendDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.FrontendFailoverCoolDown:        time.Duration(0),
		dynamicproperties.EnableReadFromClosedExecutionV2: true,
	}
	// Consumption happens inside the history service: enable the per-shard queue
	// consumer (default off) so enqueued messages are actually processed.
	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.AsyncWorkflowQueueConsumerEnabled: true,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(AsyncWFHistoryIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *AsyncWFHistoryIntegrationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
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
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.domainCacheRefresh()
}

func (s *AsyncWFHistoryIntegrationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *AsyncWFHistoryIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

// enableHistoryAsyncQueue points the test domain at the predefined history-backed
// queue ("test-async-wf-queue", declared in
// testdata/integration_async_wf_with_history_cluster.yaml). Both the frontend
// producer and the worker consumer resolve this predefined queue to the same
// history-backed provider.
func (s *AsyncWFHistoryIntegrationSuite) enableHistoryAsyncQueue() {
	ctx, cancel := createContext()
	defer cancel()

	_, err := s.AdminClient.UpdateDomainAsyncWorkflowConfiguraton(ctx, &types.UpdateDomainAsyncWorkflowConfiguratonRequest{
		Domain: s.DomainName,
		Configuration: &types.AsyncWorkflowConfiguration{
			Enabled:             true,
			PredefinedQueueName: "test-async-wf-queue",
		},
	})
	s.Require().NoError(err, "failed to enable history-backed async workflow queue for domain")

	s.domainCacheRefresh()
}

// waitForWorkflowStart polls DescribeWorkflowExecution until the async-started
// workflow appears (i.e. the history service's per-shard consumer processed the
// message from the history-backed queue and called frontend.StartWorkflowExecution)
// or the poll budget is exhausted. There is no decider/poller, so we only assert
// the execution was created, not that it made progress.
func (s *AsyncWFHistoryIntegrationSuite) waitForWorkflowStart(t *testing.T, wfID string) {
	t.Helper()
	for i := 0; i < 30; i++ {
		ctx, cancel := createContext()
		resp, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: wfID,
			},
		})
		cancel()

		if err != nil {
			t.Logf("Workflow execution not found yet. DescribeWorkflowExecution() returned err: %v", err)
			time.Sleep(time.Second)
			s.TestClusterConfig.TimeSource.Advance(time.Second)
			continue
		}
		if resp.GetWorkflowExecutionInfo() != nil {
			t.Logf("DescribeWorkflowExecution() found the execution: %#v", resp.GetWorkflowExecutionInfo())
			return
		}
		time.Sleep(time.Second)
		s.TestClusterConfig.TimeSource.Advance(time.Second)
	}

	t.Fatal("Async started workflow not found")
}

// TestStartWorkflowExecutionAsync_History_SLOW verifies the end-to-end history-backed
// async workflow path: the frontend enqueues the StartWorkflowExecution request
// via the history EnqueueAsyncWorkflowMessage RPC, and the history service's
// per-shard consumer processes it and actually starts the workflow.
func (s *AsyncWFHistoryIntegrationSuite) TestStartWorkflowExecutionAsync_History_SLOW() {
	s.enableHistoryAsyncQueue()

	s.T().Run("start workflow execution async succeeds and workflow starts", func(t *testing.T) {
		s.TestClusterConfig.TimeSource.Advance(time.Second)

		ctx, cancel := createContext()
		defer cancel()

		startTime := s.TestClusterConfig.TimeSource.Now().UnixNano()
		wfID := fmt.Sprintf("async-wf-history-integration-start-workflow-test-%d", startTime)
		wfType := "async-wf-history-integration-start-workflow-test-type"
		taskList := "async-wf-history-integration-start-workflow-test-tasklist"
		identity := "worker1"

		asyncReq := &types.StartWorkflowExecutionAsyncRequest{
			StartWorkflowExecutionRequest: &types.StartWorkflowExecutionRequest{
				RequestID:  uuid.New(),
				Domain:     s.DomainName,
				WorkflowID: wfID,
				WorkflowType: &types.WorkflowType{
					Name: wfType,
				},
				TaskList: &types.TaskList{
					Name: taskList,
				},
				Input:                               nil,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				Identity:                            identity,
			},
		}

		_, err := s.Engine.StartWorkflowExecutionAsync(ctx, asyncReq)
		if err != nil {
			t.Fatalf("StartWorkflowExecutionAsync() failed: %v", err)
		}

		s.waitForWorkflowStart(t, wfID)
	})
}

// TestSignalWithStartWorkflowExecutionAsync_History_SLOW verifies the history-backed
// async path for SignalWithStart: the frontend enqueues the
// SignalWithStartWorkflowExecution request and the history service's per-shard
// consumer processes it and starts (and signals) the workflow.
func (s *AsyncWFHistoryIntegrationSuite) TestSignalWithStartWorkflowExecutionAsync_History_SLOW() {
	s.enableHistoryAsyncQueue()

	s.T().Run("signal with start workflow execution async succeeds and workflow starts", func(t *testing.T) {
		s.TestClusterConfig.TimeSource.Advance(time.Second)

		ctx, cancel := createContext()
		defer cancel()

		startTime := s.TestClusterConfig.TimeSource.Now().UnixNano()
		wfID := fmt.Sprintf("async-wf-history-integration-signalwithstart-workflow-test-%d", startTime)
		wfType := "async-wf-history-integration-signalwithstart-workflow-test-type"
		taskList := "async-wf-history-integration-signalwithstart-workflow-test-tasklist"
		identity := "worker1"

		asyncReq := &types.SignalWithStartWorkflowExecutionAsyncRequest{
			SignalWithStartWorkflowExecutionRequest: &types.SignalWithStartWorkflowExecutionRequest{
				RequestID:  uuid.New(),
				Domain:     s.DomainName,
				WorkflowID: wfID,
				WorkflowType: &types.WorkflowType{
					Name: wfType,
				},
				TaskList: &types.TaskList{
					Name: taskList,
				},
				Input:                               nil,
				ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
				TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				Identity:                            identity,
				SignalName:                          "async-signal",
				SignalInput:                         []byte("async-signal-input"),
			},
		}

		_, err := s.Engine.SignalWithStartWorkflowExecutionAsync(ctx, asyncReq)
		if err != nil {
			t.Fatalf("SignalWithStartWorkflowExecutionAsync() failed: %v", err)
		}

		s.waitForWorkflowStart(t, wfID)
	})
}

// NOTE on the DLQ (async_workflow_queue_dlq) path:
//
// The consumer routes poison / undecodable messages to the DLQ table and advances
// its ack level so the queue is not blocked. Exercising this deterministically
// from a host integration test would require injecting a corrupt message into the
// async_workflow_queue table out-of-band (the frontend producer only ever writes
// well-formed, decodable messages), which is brittle against the real cluster.
// The DLQ / poison-message behavior is therefore covered by the consumer unit
// tests under service/history/asyncworkflowqueue rather than here.
