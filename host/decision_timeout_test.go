// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package host

import (
	"flag"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func TestDecisionTimeoutMaxAttemptsIntegrationSuite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_decision_timeout_cluster.yaml")
	if err != nil {
		panic(err)
	}

	clusterConfig.HistoryDynamicConfigOverrides = map[dynamicproperties.Key]interface{}{
		dynamicproperties.DecisionRetryMaxAttempts:    1,
		dynamicproperties.EnforceDecisionTaskAttempts: true,
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(DecisionTimeoutMaxAttemptsIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *DecisionTimeoutMaxAttemptsIntegrationSuite) SetupSuite() {
	s.setupSuite()
}

func (s *DecisionTimeoutMaxAttemptsIntegrationSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *DecisionTimeoutMaxAttemptsIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

func (s *DecisionTimeoutMaxAttemptsIntegrationSuite) TestDecisionTimeoutExceedsMaxAttempts() {
	id := "integration-decision-timeout-max-attempts-test"
	wt := "integration-decision-timeout-max-attempts-test-type"
	tl := "integration-decision-timeout-max-attempts-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1), // 1 second timeout
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.NoError(err)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(resp.RunID))

	poller := &TaskPoller{
		Engine:   s.Engine,
		Domain:   s.DomainName,
		TaskList: taskList,
		Identity: identity,
		Logger:   s.Logger,
		T:        s.T(),
	}

	poller.PollAndProcessDecisionTask(false, true)
	s.Logger.Info("Waiting for decision task timeout (attempt 0)")
	time.Sleep(2 * time.Second)

	poller.PollAndProcessDecisionTask(false, true)
	s.Logger.Info("Waiting for decision task timeout (attempt 1)")
	time.Sleep(4 * time.Second)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp.RunID,
	}

	ctx, cancel = createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    s.DomainName,
		Execution: we,
	})
	s.NoError(err)

	history := historyResponse.History
	lastEvent := history.Events[len(history.Events)-1]
	s.Equal(types.EventTypeWorkflowExecutionTerminated, lastEvent.GetEventType(),
		"Expected workflow to be terminated, but last event was %v", lastEvent.GetEventType())
	s.NotNil(lastEvent.WorkflowExecutionTerminatedEventAttributes)
	s.Equal(common.FailureReasonDecisionAttemptsExceedsLimit,
		lastEvent.WorkflowExecutionTerminatedEventAttributes.Reason)
}
