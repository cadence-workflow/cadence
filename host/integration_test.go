// Copyright (c) 2021 Uber Technologies, Inc.
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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine/engineimpl"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/matching/tasklist"
)

func TestIntegrationSuite(t *testing.T) {
	flag.Parse()

	configPath := "testdata/integration_test_cluster.yaml"
	// TODO: remove this logic once we deprecate history queue v1
	if os.Getenv("ENABLE_QUEUE_V2") == "true" {
		configPath = "testdata/integration_queuev2_cluster.yaml"
		if os.Getenv("ENABLE_QUEUE_V2_ALERT") == "true" {
			configPath = "testdata/integration_queuev2_with_alert_cluster.yaml"
		}
	}
	clusterConfig, err := GetTestClusterConfig(configPath)
	if err != nil {
		panic(err)
	}

	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(IntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *IntegrationSuite) SetupSuite() {
	s.setupSuite()
}

func (s *IntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
}

func (s *IntegrationSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

func (s *IntegrationSuite) TestStartWorkflowExecution() {
	id := "integration-start-workflow-test"
	wt := "integration-start-workflow-test-type"
	tl := "integration-start-workflow-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		Header: &types.Header{
			Fields: map[string][]byte{
				"test-key":    []byte("test-value"),
				"empty-value": {},
			},
		},
	}

	ctx, cancel := createContext()
	defer cancel()
	we0, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	ctx, cancel = createContext()
	defer cancel()
	we1, err1 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err1)
	s.Equal(we0.RunID, we1.RunID)

	newRequest := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	ctx, cancel = createContext()
	defer cancel()
	_, err2 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NotNil(err2)
	s.IsType(&types.WorkflowExecutionAlreadyStartedError{}, err2)
	s.T().Logf("Unable to start workflow execution: %v\n", err2.Error())
}

func (s *IntegrationSuite) TestStartWorkflowExecution_StartTimestampMatch() {
	id := "integration-start-workflow-start-timestamp-test"
	wt := "integration-start-workflow-start-timestamptest-type"
	tl := "integration-start-workflow-start-timestamp-test-tasklist"
	identity := "worker1"

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        &types.WorkflowType{Name: wt},
		TaskList:                            &types.TaskList{Name: tl},
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we0, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	var historyStartTime time.Time
	ctx, cancel = createContext()
	defer cancel()
	histResp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we0.GetRunID(),
		},
	})
	s.NoError(err)

	for _, event := range histResp.GetHistory().GetEvents() {
		if event.GetEventType() == types.EventTypeWorkflowExecutionStarted {
			historyStartTime = time.Unix(0, event.GetTimestamp())
			break
		}
	}

	ctx, cancel = createContext()
	defer cancel()
	descResp, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we0.GetRunID(),
		},
	})
	s.NoError(err)
	s.WithinDuration(
		historyStartTime,
		time.Unix(0, descResp.GetWorkflowExecutionInfo().GetStartTime()),
		time.Millisecond,
	)

	var listResp *types.ListOpenWorkflowExecutionsResponse
	for i := 0; i != 20; i++ {
		ctx, cancel := createContext()
		listResp, err = s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain: s.DomainName,
			StartTimeFilter: &types.StartTimeFilter{
				EarliestTime: common.Int64Ptr(historyStartTime.Add(-time.Minute).UnixNano()),
				LatestTime:   common.Int64Ptr(time.Now().UnixNano()),
			},
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: id,
				RunID:      we0.GetRunID(),
			},
		})
		cancel()
		s.NoError(err)
		if len(listResp.Executions) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if listResp == nil || len(listResp.Executions) == 0 {
		s.Fail("unable to get workflow visibility records")
	}

	s.WithinDuration(
		historyStartTime,
		time.Unix(0, listResp.Executions[0].GetStartTime()),
		time.Millisecond,
	)
}

func (s *IntegrationSuite) TestStartWorkflowExecution_IDReusePolicy() {
	id := "integration-start-workflow-id-reuse-test"
	wt := "integration-start-workflow-id-reuse-type"
	tl := "integration-start-workflow-id-reuse-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	createStartRequest := func(policy types.WorkflowIDReusePolicy) *types.StartWorkflowExecutionRequest {
		return &types.StartWorkflowExecutionRequest{
			RequestID:                           uuid.New(),
			Domain:                              s.DomainName,
			WorkflowID:                          id,
			WorkflowType:                        workflowType,
			TaskList:                            taskList,
			Input:                               nil,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
			Identity:                            identity,
			WorkflowIDReusePolicy:               &policy,
		}
	}

	request := createStartRequest(types.WorkflowIDReusePolicyAllowDuplicateFailedOnly)
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	// Test policies when workflow is running
	policies := []types.WorkflowIDReusePolicy{
		types.WorkflowIDReusePolicyAllowDuplicateFailedOnly,
		types.WorkflowIDReusePolicyAllowDuplicate,
		types.WorkflowIDReusePolicyRejectDuplicate,
	}
	for _, policy := range policies {
		newRequest := createStartRequest(policy)
		ctx, cancel := createContext()
		_, err1 := s.Engine.StartWorkflowExecution(ctx, newRequest)
		cancel()
		s.Error(err1)
		s.IsType(&types.WorkflowExecutionAlreadyStartedError{}, err1)
	}

	// Test TerminateIfRunning policy when workflow is running
	policy := types.WorkflowIDReusePolicyTerminateIfRunning
	newRequest := createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	we1, err1 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NoError(err1)
	s.NotEqual(we.GetRunID(), we1.GetRunID())
	// verify terminate status
	executionTerminated := false
GetHistoryLoop:
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
		})
		cancel()
		s.Nil(err)
		history := historyResponse.History

		lastEvent := history.Events[len(history.Events)-1]
		if lastEvent.GetEventType() != types.EventTypeWorkflowExecutionTerminated {
			s.Logger.Warn("Execution not terminated yet.")
			time.Sleep(100 * time.Millisecond)
			continue GetHistoryLoop
		}

		terminateEventAttributes := lastEvent.WorkflowExecutionTerminatedEventAttributes
		s.Equal(engineimpl.TerminateIfRunningReason, terminateEventAttributes.GetReason())
		s.Equal(fmt.Sprintf(engineimpl.TerminateIfRunningDetailsTemplate, we1.GetRunID()), string(terminateEventAttributes.Details))
		s.Equal(execution.IdentityHistoryService, terminateEventAttributes.GetIdentity())
		executionTerminated = true
		break GetHistoryLoop
	}
	s.True(executionTerminated)

	ctx, cancel = createContext()
	defer cancel()
	// Terminate current workflow execution
	err = s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we1.RunID,
		},
		Reason:   "kill workflow",
		Identity: identity,
	})
	s.Nil(err)

	// test policy AllowDuplicateFailedOnly
	policy = types.WorkflowIDReusePolicyAllowDuplicateFailedOnly
	newRequest = createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	we2, err2 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NoError(err2)
	s.NotEqual(we1.GetRunID(), we2.GetRunID())
	// complete workflow instead of terminate
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		return []byte(strconv.Itoa(0)), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}
	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	ctx, cancel = createContext()
	defer cancel()
	// duplicate requests
	we3, err3 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NoError(err3)
	s.Equal(we2.GetRunID(), we3.GetRunID())
	// new request, same policy
	newRequest = createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	_, err3 = s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.Error(err3)
	s.IsType(&types.WorkflowExecutionAlreadyStartedError{}, err3)

	// test policy RejectDuplicate
	policy = types.WorkflowIDReusePolicyRejectDuplicate
	newRequest = createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	_, err3 = s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.Error(err3)
	s.IsType(&types.WorkflowExecutionAlreadyStartedError{}, err3)

	// test policy AllowDuplicate
	policy = types.WorkflowIDReusePolicyAllowDuplicate
	newRequest = createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	we4, err4 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NoError(err4)
	s.NotEqual(we3.GetRunID(), we4.GetRunID())

	// complete workflow
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// test policy TerminateIfRunning
	policy = types.WorkflowIDReusePolicyTerminateIfRunning
	newRequest = createStartRequest(policy)
	ctx, cancel = createContext()
	defer cancel()
	we5, err5 := s.Engine.StartWorkflowExecution(ctx, newRequest)
	s.NoError(err5)
	s.NotEqual(we4.GetRunID(), we5.GetRunID())
}

func (s *IntegrationSuite) TestTerminateWorkflow() {
	id := "integration-terminate-workflow-test"
	wt := "integration-terminate-workflow-test-type"
	tl := "integration-terminate-workflow-test-tasklist"
	identity := "worker1"
	activityName := "activity_type1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	activityCount := int32(1)
	activityCounter := int32(0)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if activityCounter < activityCount {
			activityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityCounter))

			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(activityCounter)),
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		}

		return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {

		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	terminateReason := "terminate reason."
	terminateDetails := []byte("terminate details.")
	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		Reason:   terminateReason,
		Details:  terminateDetails,
		Identity: identity,
	})
	s.Nil(err)

	executionTerminated := false
GetHistoryLoop:
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
		})
		cancel()
		s.Nil(err)
		history := historyResponse.History

		lastEvent := history.Events[len(history.Events)-1]
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionTerminated {
			s.Logger.Warn("Execution not terminated yet.")
			time.Sleep(100 * time.Millisecond)
			continue GetHistoryLoop
		}

		terminateEventAttributes := lastEvent.WorkflowExecutionTerminatedEventAttributes
		s.Equal(terminateReason, terminateEventAttributes.Reason)
		s.Equal(terminateDetails, terminateEventAttributes.Details)
		s.Equal(identity, terminateEventAttributes.Identity)
		executionTerminated = true
		break GetHistoryLoop
	}

	s.True(executionTerminated)

	newExecutionStarted := false
StartNewExecutionLoop:
	for i := 0; i < 10; i++ {
		request := &types.StartWorkflowExecutionRequest{
			RequestID:                           uuid.New(),
			Domain:                              s.DomainName,
			WorkflowID:                          id,
			WorkflowType:                        workflowType,
			TaskList:                            taskList,
			Input:                               nil,
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
			Identity:                            identity,
		}

		ctx, cancel := createContext()
		newExecution, err := s.Engine.StartWorkflowExecution(ctx, request)
		cancel()
		if err != nil {
			s.Logger.Warn("Start New Execution failed. Error", tag.Error(err))
			time.Sleep(100 * time.Millisecond)
			continue StartNewExecutionLoop
		}

		s.Logger.Info("New Execution Started with the same ID", tag.WorkflowID(id),
			tag.WorkflowRunID(newExecution.RunID))
		newExecutionStarted = true
		break StartNewExecutionLoop
	}

	s.True(newExecutionStarted)
}

func (s *IntegrationSuite) TestSequentialWorkflow() {
	RunSequentialWorkflow(
		s,
		"integration-sequential-workflow-test",
		"integration-sequential-workflow-test-type",
		"integration-sequential-workflow-test-tasklist",
		0,
	)
}

func (s *IntegrationSuite) TestDelayStartWorkflow() {
	startWorkflowTS := time.Now()
	RunSequentialWorkflow(
		s,
		"integration-delay-start-workflow-test",
		"integration-delay-start-workflow-test-type",
		"integration-delay-start-workflow-test-tasklist",
		10,
	)

	targetBackoffDuration := time.Second * 10
	backoffDurationTolerance := time.Millisecond * 4000
	backoffDuration := time.Since(startWorkflowTS)
	s.True(
		backoffDuration > targetBackoffDuration,
		"Backoff duration(%f s) should have been at least 5 seconds",
		time.Duration(backoffDuration).Round(time.Millisecond).Seconds(),
	)
	s.True(
		backoffDuration < targetBackoffDuration+backoffDurationTolerance,
		"Integration test too long: %f seconds",
		time.Duration(backoffDuration).Round(time.Millisecond).Seconds(),
	)
}

func (s *IntegrationSuite) TestSignalDoesNotOverrideDelayStart() {
	id := "integration-signal-delay-start-test"
	wt := "integration-signal-delay-start-test-type"
	tl := "integration-signal-delay-start-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	// Start workflow with a very large delay (300s) so it won't fire during the test
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(600),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
		DelayStartSeconds:                   common.Int32Ptr(300),
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.NoError(err)
	s.NotNil(we)

	// Signal the workflow before the delay expires
	signalName := "test-signal"
	signalInput := []byte(`"payload"`)
	ctx2, cancel2 := createContext()
	defer cancel2()
	err = s.Engine.SignalWorkflowExecution(ctx2, &types.SignalWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		SignalName: signalName,
		Input:      signalInput,
		Identity:   identity,
	})
	s.NoError(err)

	// Describe the workflow and verify no decision task is pending
	ctx3, cancel3 := createContext()
	defer cancel3()
	descResp, err := s.Engine.DescribeWorkflowExecution(ctx3, &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.NoError(err)
	s.NotNil(descResp)

	// Workflow should still be open (no close time, no close status)
	s.Nil(descResp.WorkflowExecutionInfo.CloseStatus)

	// No decision task should have been scheduled — the signal must not override DelayStart
	s.Nil(descResp.PendingDecision, "Signal should not schedule a decision task before DelayStart expires")
}

func RunSequentialWorkflow(
	s *IntegrationSuite,
	workflowID string,
	workflowTypeStr string,
	taskListStr string,
	delayStartSeconds int32,
) {
	identity := "worker1"
	activityName := "activity_type1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = workflowTypeStr

	taskList := &types.TaskList{}
	taskList.Name = taskListStr

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          workflowID,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		DelayStartSeconds:                   common.Int32Ptr(delayStartSeconds),
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	activityCount := int32(10)
	activityCounter := int32(0)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if activityCounter < activityCount {
			activityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityCounter))

			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(activityCounter)),
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: taskListStr},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		}

		workflowComplete = true
		return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	expectedActivity := int32(1)
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		s.Equal(workflowID, execution.WorkflowID)
		s.Equal(activityName, activityType.Name)
		id, _ := strconv.Atoi(ActivityID)
		s.Equal(int(expectedActivity), id)
		buf := bytes.NewReader(input)
		var in int32
		binary.Read(buf, binary.LittleEndian, &in)
		s.Equal(expectedActivity, in)
		expectedActivity++

		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	for i := 0; i < 10; i++ {
		_, err := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err)
		if i%2 == 0 {
			err = poller.PollAndProcessActivityTask(false)
		} else { // just for testing respondActivityTaskCompleteByID
			err = poller.PollAndProcessActivityTaskWithID(false)
		}
		s.Logger.Info("PollAndProcessActivityTask", tag.Error(err))
		s.Nil(err)
	}

	s.False(workflowComplete)
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestCompleteDecisionTaskAndCreateNewOne() {
	id := "integration-complete-decision-create-new-test"
	wt := "integration-complete-decision-create-new-test-type"
	tl := "integration-complete-decision-create-new-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	decisionCount := 0
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if decisionCount < 2 {
			decisionCount++
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "test-marker",
				},
			}}, nil
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		StickyTaskList:  taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, newTask, err := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		true,
		true,
		int64(0),
		1,
		true,
		nil)
	s.Nil(err)
	s.NotNil(newTask)
	s.NotNil(newTask.DecisionTask)

	s.Equal(int64(3), newTask.DecisionTask.GetPreviousStartedEventID())
	s.Equal(int64(7), newTask.DecisionTask.GetStartedEventID())
	s.Equal(4, len(newTask.DecisionTask.History.Events))
	s.Equal(types.EventTypeDecisionTaskCompleted, newTask.DecisionTask.History.Events[0].GetEventType())
	s.Equal(types.EventTypeMarkerRecorded, newTask.DecisionTask.History.Events[1].GetEventType())
	s.Equal(types.EventTypeDecisionTaskScheduled, newTask.DecisionTask.History.Events[2].GetEventType())
	s.Equal(types.EventTypeDecisionTaskStarted, newTask.DecisionTask.History.Events[3].GetEventType())
}

func (s *IntegrationSuite) TestDecisionAndActivityTimeoutsWorkflow() {
	id := "integration-timeouts-workflow-test"
	wt := "integration-timeouts-workflow-test-type"
	tl := "integration-timeouts-workflow-test-tasklist"
	identity := "worker1"
	activityName := "activity_timer"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	activityCount := int32(4)
	activityCounter := int32(0)

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if activityCounter < activityCount {
			activityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityCounter))

			return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(activityCounter)),
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(1),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(1),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(1),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(1),
				},
			}}, nil
		}

		s.Logger.Info("Completing types.")

		workflowComplete = true
		return []byte(strconv.Itoa(int(activityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		s.Equal(id, execution.WorkflowID)
		s.Equal(activityName, activityType.Name)
		s.Logger.Info("Activity ID", tag.WorkflowActivityID(ActivityID))
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	for i := 0; i < 8; i++ {
		dropDecisionTask := (i%2 == 0)
		s.Logger.Info("Calling Decision Task", tag.Counter(i))
		var err error
		if dropDecisionTask {
			_, err = poller.PollAndProcessDecisionTask(false, true)
		} else {
			_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, int64(1))
		}
		if err != nil {
			ctx, cancel := createContext()
			historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
				Domain: s.DomainName,
				Execution: &types.WorkflowExecution{
					WorkflowID: id,
					RunID:      we.RunID,
				},
			})
			cancel()
			s.Nil(err)
			history := historyResponse.History
			PrettyPrintHistory(history, s.Logger)
		}
		s.True(err == nil || err == tasklist.ErrNoTasks, "%v", err)
		if !dropDecisionTask {
			s.Logger.Info("Calling Activity Task: %d", tag.Counter(i))
			err = poller.PollAndProcessActivityTask(i%4 == 0)
			s.True(err == nil || err == tasklist.ErrNoTasks)
		}
	}

	s.Logger.Info("Waiting for workflow to complete", tag.WorkflowRunID(we.RunID))

	s.False(workflowComplete)
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestWorkflowRetry() {
	id := "integration-wf-retry-test"
	wt := "integration-wf-retry-type"
	tl := "integration-wf-retry-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	initialIntervalInSeconds := 1
	backoffCoefficient := 1.5
	maximumAttempts := 5
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    int32(initialIntervalInSeconds),
			MaximumAttempts:             int32(maximumAttempts),
			MaximumIntervalInSeconds:    1,
			NonRetriableErrorReasons:    []string{"bad-bug"},
			BackoffCoefficient:          backoffCoefficient,
			ExpirationIntervalInSeconds: 100,
		},
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	var executions []*types.WorkflowExecution

	attemptCount := 0

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		executions = append(executions, execution)
		attemptCount++
		if attemptCount == maximumAttempts {
			return nil, []*types.Decision{
				{
					DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
					CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
						Result: []byte("succeed-after-retry"),
					},
				}}, nil
		}
		return nil, []*types.Decision{
			{
				DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
				FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr("retryable-error"),
					Details: nil,
				},
			}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	describeWorkflowExecution := func(execution *types.WorkflowExecution) (*types.DescribeWorkflowExecutionResponse, error) {
		ctx, cancel := createContext()
		defer cancel()
		return s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
			Domain:    s.DomainName,
			Execution: execution,
		})
	}

	for i := 0; i != maximumAttempts; i++ {
		_, err := poller.PollAndProcessDecisionTask(false, false)
		s.True(err == nil, err)
		events := s.getHistory(s.DomainName, executions[i])
		if i == maximumAttempts-1 {
			s.Equal(types.EventTypeWorkflowExecutionCompleted, events[len(events)-1].GetEventType())
		} else {
			s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, events[len(events)-1].GetEventType())
		}
		s.Equal(int32(i), events[0].GetWorkflowExecutionStartedEventAttributes().GetAttempt())

		dweResponse, err := describeWorkflowExecution(executions[i])
		s.Nil(err)
		backoff := time.Duration(0)
		if i > 0 {
			backoff = time.Duration(float64(initialIntervalInSeconds)*math.Pow(backoffCoefficient, float64(i-1))) * time.Second
			// retry backoff cannot larger than MaximumIntervalInSeconds
			if backoff > time.Second {
				backoff = time.Second
			}
		}
		expectedExecutionTime := dweResponse.WorkflowExecutionInfo.GetStartTime() + backoff.Nanoseconds()
		s.Equal(expectedExecutionTime, dweResponse.WorkflowExecutionInfo.GetExecutionTime())
	}
}

func (s *IntegrationSuite) TestWorkflowRetryFailures() {
	id := "integration-wf-retry-failures-test"
	wt := "integration-wf-retry-failures-type"
	tl := "integration-wf-retry-failures-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	workflowImpl := func(attempts int, errorReason string, executions *[]*types.WorkflowExecution) decisionTaskHandler {
		attemptCount := 0

		dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
			previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
			*executions = append(*executions, execution)
			attemptCount++
			if attemptCount == attempts {
				return nil, []*types.Decision{
					{
						DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
						CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
							Result: []byte("succeed-after-retry"),
						},
					}}, nil
			}
			return nil, []*types.Decision{
				{
					DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
					FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
						// Reason:  common.StringPtr("retryable-error"),
						Reason:  common.StringPtr(errorReason),
						Details: nil,
					},
				}}, nil
		}

		return dtHandler
	}

	// Fail using attempt
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			MaximumAttempts:             3,
			MaximumIntervalInSeconds:    1,
			NonRetriableErrorReasons:    []string{"bad-bug"},
			BackoffCoefficient:          1,
			ExpirationIntervalInSeconds: 100,
		},
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	executions := []*types.WorkflowExecution{}
	dtHandler := workflowImpl(5, "retryable-error", &executions)
	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)
	events := s.getHistory(s.DomainName, executions[0])
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, events[len(events)-1].GetEventType())
	s.Equal(int32(0), events[0].GetWorkflowExecutionStartedEventAttributes().GetAttempt())

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)
	events = s.getHistory(s.DomainName, executions[1])
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, events[len(events)-1].GetEventType())
	s.Equal(int32(1), events[0].GetWorkflowExecutionStartedEventAttributes().GetAttempt())

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)
	events = s.getHistory(s.DomainName, executions[2])
	s.Equal(types.EventTypeWorkflowExecutionFailed, events[len(events)-1].GetEventType())
	s.Equal(int32(2), events[0].GetWorkflowExecutionStartedEventAttributes().GetAttempt())

	// Fail error reason
	request = &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			MaximumAttempts:             3,
			MaximumIntervalInSeconds:    1,
			NonRetriableErrorReasons:    []string{"bad-bug"},
			BackoffCoefficient:          1,
			ExpirationIntervalInSeconds: 100,
		},
	}

	ctx, cancel = createContext()
	defer cancel()
	we, err0 = s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	executions = []*types.WorkflowExecution{}
	dtHandler = workflowImpl(5, "bad-bug", &executions)
	poller = &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)
	events = s.getHistory(s.DomainName, executions[0])
	s.Equal(types.EventTypeWorkflowExecutionFailed, events[len(events)-1].GetEventType())
	s.Equal(int32(0), events[0].GetWorkflowExecutionStartedEventAttributes().GetAttempt())

}

func (s *IntegrationSuite) TestCronWorkflow() {
	id := "integration-wf-cron-test"
	wt := "integration-wf-cron-type"
	tl := "integration-wf-cron-tasklist"
	identity := "worker1"
	cronSchedule := "@every 3s"

	targetBackoffDuration := time.Second * 3
	backoffDurationTolerance := time.Millisecond * 500

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	memo := &types.Memo{
		Fields: map[string][]byte{"memoKey": []byte("memoVal")},
	}
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{"CustomKeywordField": []byte(`"1"`)},
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		CronSchedule:                        cronSchedule, // minimum interval by standard spec is 1m (* * * * *), use non-standard descriptor for short interval for test
		Memo:                                memo,
		SearchAttributes:                    searchAttr,
	}

	startWorkflowTS := time.Now()
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	var executions []*types.WorkflowExecution

	attemptCount := 0

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		executions = append(executions, execution)
		attemptCount++
		if attemptCount == 2 {
			return nil, []*types.Decision{
				{
					DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
					CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
						Result: []byte("cron-test-result"),
					},
				}}, nil
		}
		return nil, []*types.Decision{
			{
				DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
				FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr("cron-test-error"),
					Details: nil,
				},
			}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startWorkflowTS.UnixNano())
	startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())

	// Sleep some time before checking the open executions.
	// This will not cost extra time as the polling for first decision task will be blocked for 3 seconds.
	time.Sleep(2 * time.Second)
	ctx, cancel = createContext()
	defer cancel()
	resp, err := s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
		Domain:          s.DomainName,
		MaximumPageSize: 100,
		StartTimeFilter: startFilter,
		ExecutionFilter: &types.WorkflowExecutionFilter{
			WorkflowID: id,
		},
	})
	s.Nil(err)
	s.Equal(1, len(resp.GetExecutions()))
	executionInfo := resp.GetExecutions()[0]
	s.Equal(targetBackoffDuration.Nanoseconds(), executionInfo.GetExecutionTime()-executionInfo.GetStartTime())

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)

	// Make sure the cron workflow start running at a proper time, in this case 3 seconds after the
	// startWorkflowExecution request
	backoffDuration := time.Since(startWorkflowTS)
	s.True(backoffDuration > targetBackoffDuration)
	s.True(backoffDuration < targetBackoffDuration+backoffDurationTolerance)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)

	s.Equal(3, attemptCount)
	ctx, cancel = createContext()
	defer cancel()
	terminateErr := s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
		},
	})
	s.NoError(terminateErr)
	events := s.getHistory(s.DomainName, executions[0])
	lastEvent := events[len(events)-1]
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())
	attributes := lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
	s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
	s.Equal("cron-test-error", attributes.GetFailureReason())
	s.Equal(0, len(attributes.GetLastCompletionResult()))
	s.Equal(memo, attributes.Memo)
	s.Equal(searchAttr, attributes.SearchAttributes)

	events = s.getHistory(s.DomainName, executions[1])
	lastEvent = events[len(events)-1]
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())
	attributes = lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
	s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
	s.Equal("", attributes.GetFailureReason())
	s.Equal("cron-test-result", string(attributes.GetLastCompletionResult()))
	s.Equal(memo, attributes.Memo)
	s.Equal(searchAttr, attributes.SearchAttributes)

	events = s.getHistory(s.DomainName, executions[2])
	lastEvent = events[len(events)-1]
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())
	attributes = lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
	s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
	s.Equal("cron-test-error", attributes.GetFailureReason())
	s.Equal("cron-test-result", string(attributes.GetLastCompletionResult()))
	s.Equal(memo, attributes.Memo)
	s.Equal(searchAttr, attributes.SearchAttributes)

	startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())
	var closedExecutions []*types.WorkflowExecutionInfo
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err := s.Engine.ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: id,
			},
		})
		cancel()
		s.Nil(err)
		if len(resp.GetExecutions()) == 4 {
			closedExecutions = resp.GetExecutions()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.NotNil(closedExecutions)
	ctx, cancel = createContext()
	defer cancel()
	dweResponse, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.Nil(err)
	expectedExecutionTime := dweResponse.WorkflowExecutionInfo.GetStartTime() + 3*time.Second.Nanoseconds()
	s.Equal(expectedExecutionTime, dweResponse.WorkflowExecutionInfo.GetExecutionTime())

	sort.Slice(closedExecutions, func(i, j int) bool {
		return closedExecutions[i].GetStartTime() < closedExecutions[j].GetStartTime()
	})
	lastExecution := closedExecutions[0]

	for i := 1; i != 4; i++ {
		executionInfo := closedExecutions[i]
		expectedBackoff := executionInfo.GetExecutionTime() - lastExecution.GetExecutionTime()
		// The execution time calculate based on last execution close time
		// However, the current execution time is based on the current start time
		// This code is to remove the diff between current start time and last execution close time
		// TODO: Remove this line once we unify the time source
		executionTimeDiff := executionInfo.GetStartTime() - lastExecution.GetCloseTime()
		// The backoff between any two executions should be multiplier of the target backoff duration which is 3 in this test
		// However, it's difficult to guarantee accuracy within a second, we allows 1s as buffering...
		backoffSeconds := int(time.Duration(expectedBackoff - executionTimeDiff).Round(time.Second).Seconds())
		targetBackoffSeconds := int(targetBackoffDuration.Seconds())
		targetDiff := int(math.Abs(float64(backoffSeconds%targetBackoffSeconds-3))) % 3

		s.True(
			targetDiff <= 1,
			"Still Flaky?:((%v-%v) - (%v-%v)), backoffSeconds: %v, targetBackoffSeconds: %v, targetDiff:%v",
			backoffSeconds,
			executionInfo.GetExecutionTime(),
			lastExecution.GetExecutionTime(),
			executionInfo.GetStartTime(),
			lastExecution.GetCloseTime(),
			targetBackoffSeconds,
			targetDiff,
		)
		lastExecution = executionInfo
	}
}

func (s *IntegrationSuite) TestCronWorkflowTimeout() {
	id := "integration-wf-cron-timeout-test"
	wt := "integration-wf-cron-timeout-type"
	tl := "integration-wf-cron-timeout-tasklist"
	identity := "worker1"
	cronSchedule := "@every 3s"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	memo := &types.Memo{
		Fields: map[string][]byte{"memoKey": []byte("memoVal")},
	}
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{"CustomKeywordField": []byte(`"1"`)},
	}
	retryPolicy := &types.RetryPolicy{
		InitialIntervalInSeconds:    1,
		BackoffCoefficient:          2.0,
		MaximumIntervalInSeconds:    1,
		ExpirationIntervalInSeconds: 1,
		MaximumAttempts:             0,
		NonRetriableErrorReasons:    []string{"bad-error"},
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1), // set workflow timeout to 1s
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		CronSchedule:                        cronSchedule, // minimum interval by standard spec is 1m (* * * * *), use non-standard descriptor for short interval for test
		Memo:                                memo,
		SearchAttributes:                    searchAttr,
		RetryPolicy:                         retryPolicy,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	var executions []*types.WorkflowExecution
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		executions = append(executions, execution)
		return nil, []*types.Decision{
			{
				DecisionType: types.DecisionTypeStartTimer.Ptr(),

				StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
					TimerID:                   "timer-id",
					StartToFireTimeoutSeconds: common.Int64Ptr(5),
				},
			}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)

	time.Sleep(1 * time.Second) // wait for workflow timeout

	// check when workflow timeout, continueAsNew event contains expected fields
	events := s.getHistory(s.DomainName, executions[0])
	lastEvent := events[len(events)-1]
	s.Equal(types.EventTypeWorkflowExecutionContinuedAsNew, lastEvent.GetEventType())
	attributes := lastEvent.WorkflowExecutionContinuedAsNewEventAttributes
	s.Equal(types.ContinueAsNewInitiatorCronSchedule, attributes.GetInitiator())
	s.Equal("cadenceInternal:Timeout START_TO_CLOSE", attributes.GetFailureReason())
	s.Equal(memo, attributes.Memo)
	s.Equal(searchAttr, attributes.SearchAttributes)

	firstStartWorkflowEvent := events[0]
	s.NotNil(firstStartWorkflowEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.True(err == nil, err)

	// check new run contains expected fields
	events = s.getHistory(s.DomainName, executions[1])
	firstEvent := events[0]
	s.Equal(types.EventTypeWorkflowExecutionStarted, firstEvent.GetEventType())
	startAttributes := firstEvent.WorkflowExecutionStartedEventAttributes
	s.Equal(memo, startAttributes.Memo)
	s.Equal(searchAttr, startAttributes.SearchAttributes)

	s.NotNil(firstEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp)
	// test that the new workflow has a different expiration timestamp from the first workflow
	s.True(*firstEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp >
		*firstStartWorkflowEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp)

	// terminate cron
	ctx, cancel = createContext()
	defer cancel()
	terminateErr := s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
		},
	})
	s.NoError(terminateErr)
}

func (s *IntegrationSuite) TestSequential_UserTimers() {
	id := "integration-sequential-user-timers-test"
	wt := "integration-sequential-user-timers-test-type"
	tl := "integration-sequential-user-timers-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	timerCount := int32(4)
	timerCounter := int32(0)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if timerCounter < timerCount {
			timerCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, timerCounter))
			return []byte(strconv.Itoa(int(timerCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeStartTimer.Ptr(),
				StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
					TimerID:                   fmt.Sprintf("timer-id-%d", timerCounter),
					StartToFireTimeoutSeconds: common.Int64Ptr(1),
				},
			}}, nil
		}

		workflowComplete = true
		return []byte(strconv.Itoa(int(timerCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	for i := 0; i < 4; i++ {
		_, err := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask: completed")
		s.Nil(err)
	}

	s.False(workflowComplete)
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestRateLimitBufferedEvents() {
	id := "integration-rate-limit-buffered-events-test"
	wt := "integration-rate-limit-buffered-events-test-type"
	tl := "integration-rate-limit-buffered-events-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	workflowComplete := false
	signalsSent := false
	signalCount := 0
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, h *types.History) ([]byte, []*types.Decision, error) {

		// Count signals
		for _, event := range h.Events[previousStartedEventID:] {
			if event.GetEventType() == types.EventTypeWorkflowExecutionSignaled {
				signalCount++
			}
		}

		if !signalsSent {
			signalsSent = true
			// Buffered Signals
			for i := 0; i < 100; i++ {
				buf := new(bytes.Buffer)
				binary.Write(buf, binary.LittleEndian, int64(i))
				s.Nil(s.sendSignal(s.DomainName, workflowExecution, "SignalName", buf.Bytes(), identity))
			}

			buf := new(bytes.Buffer)
			binary.Write(buf, binary.LittleEndian, int64(101))
			signalErr := s.sendSignal(s.DomainName, workflowExecution, "SignalName", buf.Bytes(), identity)
			s.Nil(signalErr)

			// this decision will be ignored as he decision task is already failed
			return nil, []*types.Decision{}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// first decision to send 101 signals, the last signal will force fail decision and flush buffered events.
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.EqualError(err, "Decision task not found.")

	// Process signal in decider
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.True(workflowComplete)
	s.Equal(101, signalCount) // check that all 101 signals are received.
}

func (s *IntegrationSuite) TestBufferedEvents() {
	id := "integration-buffered-events-test"
	wt := "integration-buffered-events-test-type"
	tl := "integration-buffered-events-test-tasklist"
	identity := "worker1"
	signalName := "buffered-signal"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	workflowComplete := false
	signalSent := false
	var signalEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if !signalSent {
			signalSent = true

			ctx, cancel := createContext()
			defer cancel()
			// this will create new event when there is in-flight decision task, and the new event will be buffered
			err := s.Engine.SignalWorkflowExecution(ctx,
				&types.SignalWorkflowExecutionRequest{
					Domain: s.DomainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: id,
					},
					SignalName: "buffered-signal",
					Input:      []byte("buffered-signal-input"),
					Identity:   identity,
				})
			s.NoError(err)
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    "1",
					ActivityType:                  &types.ActivityType{Name: "test-activity-type"},
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         []byte("test-input"),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if previousStartedEventID > 0 && signalEvent == nil {
			for _, event := range history.Events[previousStartedEventID:] {
				if *event.EventType == types.EventTypeWorkflowExecutionSignaled {
					signalEvent = event
				}
			}
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// first decision, which sends signal and the signal event should be buffered to append after first decision closed
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	ctx, cancel = createContext()
	defer cancel()
	// check history, the signal event should be after the complete decision task
	histResp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.NoError(err)
	s.NotNil(histResp.History.Events)
	s.True(len(histResp.History.Events) >= 6)
	s.Equal(histResp.History.Events[3].GetEventType(), types.EventTypeDecisionTaskCompleted)
	s.Equal(histResp.History.Events[4].GetEventType(), types.EventTypeActivityTaskScheduled)
	s.Equal(histResp.History.Events[5].GetEventType(), types.EventTypeWorkflowExecutionSignaled)

	// Process signal in decider
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(signalEvent)
	s.Equal(signalName, signalEvent.WorkflowExecutionSignaledEventAttributes.SignalName)
	s.Equal(identity, signalEvent.WorkflowExecutionSignaledEventAttributes.Identity)
	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestDescribeWorkflowExecution() {
	id := "integration-describe-wfe-test"
	wt := "integration-describe-wfe-test-type"
	tl := "integration-describe-wfe-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	childID := id + "-child"
	childType := wt + "-child"
	childTaskList := tl + "-child"

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		CronOverlapPolicy:                   types.CronOverlapPolicySkipped.Ptr(),
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	describeWorkflowExecution := func() (*types.DescribeWorkflowExecutionResponse, error) {
		ctx, cancel := createContext()
		defer cancel()
		return s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
		})
	}
	dweResponse, err := describeWorkflowExecution()
	s.Nil(err)
	s.True(nil == dweResponse.WorkflowExecutionInfo.CloseTime)
	s.Equal(int64(2), dweResponse.WorkflowExecutionInfo.HistoryLength) // WorkflowStarted, DecisionScheduled
	s.Equal(dweResponse.WorkflowExecutionInfo.GetStartTime(), dweResponse.WorkflowExecutionInfo.GetExecutionTime())

	// decider logic
	workflowComplete := false
	signalSent := false
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if !signalSent {
			signalSent = true

			s.NoError(err)
			return nil, []*types.Decision{
				{
					DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
					ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
						ActivityID:                    "1",
						ActivityType:                  &types.ActivityType{Name: "test-activity-type"},
						TaskList:                      &types.TaskList{Name: tl},
						Input:                         []byte("test-input"),
						ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
						ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
						StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
						HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
					},
				},
				{
					DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
					StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
						WorkflowID:                          childID,
						WorkflowType:                        &types.WorkflowType{Name: childType},
						TaskList:                            &types.TaskList{Name: childTaskList},
						Input:                               []byte("child-workflow-input"),
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(200),
						TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
						Control:                             nil,
						CronOverlapPolicy:                   types.CronOverlapPolicySkipped.Ptr(),
					},
				},
			}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// first decision to schedule new activity
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// wait for child workflow to start
	for i := 0; i != 10; i++ {
		dweResponse, err = describeWorkflowExecution()
		s.Nil(err)
		if len(dweResponse.PendingChildren) == 1 &&
			dweResponse.PendingChildren[0].GetRunID() != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.NotEmpty(dweResponse.PendingChildren[0].GetRunID(), "unable to start child workflow")
	s.True(nil == dweResponse.WorkflowExecutionInfo.CloseStatus)
	// DecisionStarted, DecisionCompleted, ActivityScheduled, ChildWorkflowInit, ChildWorkflowStarted, DecisionTaskScheduled
	s.Equal(int64(8), dweResponse.WorkflowExecutionInfo.HistoryLength)
	s.Equal(1, len(dweResponse.PendingActivities))
	s.Equal("test-activity-type", dweResponse.PendingActivities[0].ActivityType.GetName())
	s.Equal(int64(0), dweResponse.PendingActivities[0].GetLastHeartbeatTimestamp())
	s.Equal(1, len(dweResponse.PendingChildren))
	s.Equal(s.DomainName, dweResponse.PendingChildren[0].GetDomain())
	s.Equal(childID, dweResponse.PendingChildren[0].GetWorkflowID())
	s.Equal(childType, dweResponse.PendingChildren[0].GetWorkflowTypeName())
	s.Equal(types.CronOverlapPolicySkipped, *dweResponse.WorkflowExecutionInfo.CronOverlapPolicy)
	s.Equal(false, dweResponse.WorkflowExecutionInfo.IsCron)

	// process activity task
	err = poller.PollAndProcessActivityTask(false)

	dweResponse, err = describeWorkflowExecution()
	s.Nil(err)
	s.True(nil == dweResponse.WorkflowExecutionInfo.CloseStatus)
	s.Equal(int64(10), dweResponse.WorkflowExecutionInfo.HistoryLength) // ActivityTaskStarted, ActivityTaskCompleted
	s.Equal(0, len(dweResponse.PendingActivities))

	// Process signal in decider
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	s.True(workflowComplete)

	dweResponse, err = describeWorkflowExecution()
	s.Nil(err)
	s.Equal(types.WorkflowExecutionCloseStatusCompleted, *dweResponse.WorkflowExecutionInfo.CloseStatus)
	s.Equal(int64(13), dweResponse.WorkflowExecutionInfo.HistoryLength) // DecisionStarted, DecisionCompleted, WorkflowCompleted
}

func (s *IntegrationSuite) TestVisibility() {
	startTime := time.Now().UnixNano()

	// Start 2 workflow executions
	id1 := "integration-visibility-test1"
	id2 := "integration-visibility-test2"
	wt := "integration-visibility-test-type"
	tl := "integration-visibility-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	startRequest := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id1,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	startResponse, err0 := s.Engine.StartWorkflowExecution(ctx, startRequest)
	s.Nil(err0)

	// Now complete one of the executions
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		return []byte{}, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err1 := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err1)

	// wait until the start workflow is done
	var nextToken []byte
	historyEventFilterType := types.HistoryEventFilterTypeCloseEvent
	for {
		ctx, cancel := createContext()
		historyResponse, historyErr := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: startRequest.Domain,
			Execution: &types.WorkflowExecution{
				WorkflowID: startRequest.WorkflowID,
				RunID:      startResponse.RunID,
			},
			WaitForNewEvent:        true,
			HistoryEventFilterType: &historyEventFilterType,
			NextPageToken:          nextToken,
		})
		cancel()
		s.Nil(historyErr)
		if len(historyResponse.NextPageToken) == 0 {
			break
		}

		nextToken = historyResponse.NextPageToken
	}

	startRequest = &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id2,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel = createContext()
	defer cancel()
	_, err2 := s.Engine.StartWorkflowExecution(ctx, startRequest)
	s.Nil(err2)

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startTime)
	startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())

	closedCount := 0
	openCount := 0

	var historyLength int64
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err3 := s.Engine.ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
		})
		cancel()
		s.Nil(err3)
		closedCount = len(resp.Executions)
		if closedCount == 1 {
			historyLength = resp.Executions[0].HistoryLength
			break
		}
		s.Logger.Info("Closed WorkflowExecution is not yet visible")
		time.Sleep(100 * time.Millisecond)
	}
	s.Equal(1, closedCount)
	s.Equal(int64(5), historyLength)

	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err4 := s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
		})
		cancel()
		s.Nil(err4)
		openCount = len(resp.Executions)
		if openCount == 1 {
			break
		}
		s.Logger.Info("Open WorkflowExecution is not yet visible")
		time.Sleep(100 * time.Millisecond)
	}
	s.Equal(1, openCount)
}

func (s *IntegrationSuite) TestChildWorkflowExecution() {
	parentID := "integration-child-workflow-test-parent"
	childID := "integration-child-workflow-test-child"
	wtParent := "integration-child-workflow-test-parent-type"
	wtChild := "integration-child-workflow-test-child-type"
	tlParent := "integration-child-workflow-test-parent-tasklist"
	tlChild := "integration-child-workflow-test-child-tasklist"
	identity := "worker1"

	parentWorkflowType := &types.WorkflowType{}
	parentWorkflowType.Name = wtParent

	childWorkflowType := &types.WorkflowType{}
	childWorkflowType.Name = wtChild

	taskListParent := &types.TaskList{}
	taskListParent.Name = tlParent
	taskListChild := &types.TaskList{}
	taskListChild.Name = tlChild

	header := &types.Header{
		Fields: map[string][]byte{"tracing": []byte("sample payload")},
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          parentID,
		WorkflowType:                        parentWorkflowType,
		TaskList:                            taskListParent,
		Input:                               nil,
		Header:                              header,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	childComplete := false
	childExecutionStarted := false
	var startedEvent *types.HistoryEvent
	var completedEvent *types.HistoryEvent

	memoInfo, _ := json.Marshal("memo")
	memo := &types.Memo{
		Fields: map[string][]byte{
			"Info": memoInfo,
		},
	}
	attrValBytes, _ := json.Marshal("attrVal")
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			"CustomKeywordField": attrValBytes,
		},
	}

	// Parent Decider Logic
	dtHandlerParent := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		s.Logger.Info("Processing decision task for ", tag.WorkflowID(execution.WorkflowID))

		if execution.WorkflowID == parentID {
			if !childExecutionStarted {
				s.Logger.Info("Starting child execution.")
				childExecutionStarted = true

				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
					StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
						WorkflowID:                          childID,
						WorkflowType:                        childWorkflowType,
						TaskList:                            taskListChild,
						Input:                               []byte("child-workflow-input"),
						Header:                              header,
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(200),
						TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
						Control:                             nil,
						Memo:                                memo,
						SearchAttributes:                    searchAttr,
					},
				}}, nil
			} else if previousStartedEventID > 0 {
				for _, event := range history.Events[previousStartedEventID:] {
					if *event.EventType == types.EventTypeChildWorkflowExecutionStarted {
						startedEvent = event
						return nil, []*types.Decision{}, nil
					}

					if *event.EventType == types.EventTypeChildWorkflowExecutionCompleted {
						completedEvent = event
						return nil, []*types.Decision{{
							DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
							CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
								Result: []byte("Done."),
							},
						}}, nil
					}
				}
			}
		}

		return nil, nil, nil
	}

	var childStartedEvent *types.HistoryEvent
	// Child Decider Logic
	dtHandlerChild := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if previousStartedEventID <= 0 {
			childStartedEvent = history.Events[0]
		}

		s.Logger.Info("Processing decision task for Child ", tag.WorkflowID(execution.WorkflowID))
		childComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Child Done."),
			},
		}}, nil
	}

	pollerParent := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskListParent,
		Identity:        identity,
		DecisionHandler: dtHandlerParent,
		Logger:          s.Logger,
		T:               s.T(),
	}

	pollerChild := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskListChild,
		Identity:        identity,
		DecisionHandler: dtHandlerChild,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to start child execution
	_, err := pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.True(childExecutionStarted)

	// Process ChildExecution Started event and Process Child Execution and complete it
	_, err = pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	_, err = pollerChild.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(startedEvent)
	s.True(childComplete)
	s.NotNil(childStartedEvent)
	s.Equal(types.EventTypeWorkflowExecutionStarted, childStartedEvent.GetEventType())
	s.Equal(s.DomainName, childStartedEvent.WorkflowExecutionStartedEventAttributes.GetParentWorkflowDomain())
	s.Equal(parentID, childStartedEvent.WorkflowExecutionStartedEventAttributes.ParentWorkflowExecution.GetWorkflowID())
	s.Equal(we.GetRunID(), childStartedEvent.WorkflowExecutionStartedEventAttributes.ParentWorkflowExecution.GetRunID())
	s.Equal(startedEvent.ChildWorkflowExecutionStartedEventAttributes.GetInitiatedEventID(),
		childStartedEvent.WorkflowExecutionStartedEventAttributes.GetParentInitiatedEventID())
	s.Equal(header, startedEvent.ChildWorkflowExecutionStartedEventAttributes.Header)
	s.Equal(header, childStartedEvent.WorkflowExecutionStartedEventAttributes.Header)
	s.Equal(memo, childStartedEvent.WorkflowExecutionStartedEventAttributes.GetMemo())
	s.Equal(searchAttr, childStartedEvent.WorkflowExecutionStartedEventAttributes.GetSearchAttributes())

	// Process ChildExecution completed event and complete parent execution
	_, err = pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(completedEvent)
	completedAttributes := completedEvent.ChildWorkflowExecutionCompletedEventAttributes
	s.Equal(s.DomainName, completedAttributes.Domain)
	s.Equal(childID, completedAttributes.WorkflowExecution.WorkflowID)
	s.Equal(wtChild, completedAttributes.WorkflowType.Name)
	s.Equal([]byte("Child Done."), completedAttributes.Result)
}

func (s *IntegrationSuite) TestCronChildWorkflowExecution() {
	parentID := "integration-cron-child-workflow-test-parent"
	childID := "integration-cron-child-workflow-test-child"
	wtParent := "integration-cron-child-workflow-test-parent-type"
	wtChild := "integration-cron-child-workflow-test-child-type"
	tlParent := "integration-cron-child-workflow-test-parent-tasklist"
	tlChild := "integration-cron-child-workflow-test-child-tasklist"
	identity := "worker1"

	cronSchedule := "@every 3s"
	targetBackoffDuration := time.Second * 3
	backoffDurationTolerance := time.Second

	parentWorkflowType := &types.WorkflowType{}
	parentWorkflowType.Name = wtParent

	childWorkflowType := &types.WorkflowType{}
	childWorkflowType.Name = wtChild

	taskListParent := &types.TaskList{}
	taskListParent.Name = tlParent
	taskListChild := &types.TaskList{}
	taskListChild.Name = tlChild

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          parentID,
		WorkflowType:                        parentWorkflowType,
		TaskList:                            taskListParent,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	startParentWorkflowTS := time.Now()
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	childExecutionStarted := false
	var terminatedEvent *types.HistoryEvent
	var startChildWorkflowTS time.Time
	// Parent Decider Logic
	dtHandlerParent := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		s.Logger.Info("Processing decision task for ", tag.WorkflowID(execution.WorkflowID))

		if !childExecutionStarted {
			s.Logger.Info("Starting child execution.")
			childExecutionStarted = true
			startChildWorkflowTS = time.Now()
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
				StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
					WorkflowID:                          childID,
					WorkflowType:                        childWorkflowType,
					TaskList:                            taskListChild,
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(200),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
					Control:                             nil,
					CronSchedule:                        cronSchedule,
				},
			}}, nil
		}
		for _, event := range history.Events[previousStartedEventID:] {
			if *event.EventType == types.EventTypeChildWorkflowExecutionTerminated {
				terminatedEvent = event
				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
					CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
						Result: []byte("Done."),
					},
				}}, nil
			}
		}
		return nil, nil, nil
	}

	// Child Decider Logic
	dtHandlerChild := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		s.Logger.Info("Processing decision task for Child ", tag.WorkflowID(execution.WorkflowID))
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{},
		}}, nil
	}

	pollerParent := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskListParent,
		Identity:        identity,
		DecisionHandler: dtHandlerParent,
		Logger:          s.Logger,
		T:               s.T(),
	}

	pollerChild := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskListChild,
		Identity:        identity,
		DecisionHandler: dtHandlerChild,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to start child execution
	_, err := pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.True(childExecutionStarted)

	// Process ChildExecution Started event
	_, err = pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startChildWorkflowTS.UnixNano())
	for i := 0; i < 2; i++ {
		// Sleep some time before checking the open executions.
		// This will not cost extra time as the polling for first decision task will be blocked for 3 seconds.
		time.Sleep(2 * time.Second)
		startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())
		ctx, cancel := createContext()
		resp, err := s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: childID,
			},
		})
		cancel()
		s.Nil(err)
		s.Equal(1, len(resp.GetExecutions()))

		_, err = pollerChild.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err)

		backoffDuration := time.Since(startChildWorkflowTS)
		s.True(backoffDuration < targetBackoffDuration+backoffDurationTolerance)
		startChildWorkflowTS = time.Now()
	}

	ctx, cancel = createContext()
	defer cancel()
	// terminate the childworkflow
	terminateErr := s.Engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: childID,
		},
	})
	s.Nil(terminateErr)

	// Process ChildExecution terminated event and complete parent execution
	_, err = pollerParent.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(terminatedEvent)
	terminatedAttributes := terminatedEvent.ChildWorkflowExecutionTerminatedEventAttributes
	s.Equal(s.DomainName, terminatedAttributes.Domain)
	s.Equal(childID, terminatedAttributes.WorkflowExecution.WorkflowID)
	s.Equal(wtChild, terminatedAttributes.WorkflowType.Name)

	startFilter.EarliestTime = common.Int64Ptr(startParentWorkflowTS.UnixNano())
	startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())
	var closedExecutions []*types.WorkflowExecutionInfo
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err := s.Engine.ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
		})
		cancel()
		s.Nil(err)
		if len(resp.GetExecutions()) == 4 {
			closedExecutions = resp.GetExecutions()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	s.NotNil(closedExecutions)
	sort.Slice(closedExecutions, func(i, j int) bool {
		return closedExecutions[i].GetStartTime() < closedExecutions[j].GetStartTime()
	})
	// The first parent is not the cron workflow, only verify child workflow with cron schedule
	lastExecution := closedExecutions[1]
	for i := 2; i != 4; i++ {
		executionInfo := closedExecutions[i]
		// Round up the time precision to seconds
		expectedBackoff := executionInfo.GetExecutionTime()/1000000000 - lastExecution.GetExecutionTime()/1000000000
		// The execution time calculate based on last execution close time
		// However, the current execution time is based on the current start time
		// This code is to remove the diff between current start time and last execution close time
		// TODO: Remove this line once we unify the time source.
		executionTimeDiff := executionInfo.GetStartTime()/1000000000 - lastExecution.GetCloseTime()/1000000000
		// The backoff between any two executions should be multiplier of the target backoff duration which is 3 in this test
		s.Equal(int64(0), int64(expectedBackoff-executionTimeDiff)/1000000000%(targetBackoffDuration.Nanoseconds()/1000000000))
		lastExecution = executionInfo
	}
}

func (s *IntegrationSuite) TestWorkflowTimeout() {
	startTime := time.Now().UnixNano()

	id := "integration-workflow-timeout"
	wt := "integration-workflow-timeout-type"
	tl := "integration-workflow-timeout-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false

GetHistoryLoop:
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
		})
		cancel()
		s.Nil(err)
		history := historyResponse.History

		lastEvent := history.Events[len(history.Events)-1]
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionTimedOut {
			s.Logger.Warn("Execution not timedout yet.")
			time.Sleep(200 * time.Millisecond)
			continue GetHistoryLoop
		}

		timeoutEventAttributes := lastEvent.WorkflowExecutionTimedOutEventAttributes
		s.Equal(types.TimeoutTypeStartToClose, *timeoutEventAttributes.TimeoutType)
		workflowComplete = true
		break GetHistoryLoop
	}
	s.True(workflowComplete)

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startTime)
	startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())

	closedCount := 0
ListClosedLoop:
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err3 := s.Engine.ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: startFilter,
		})
		cancel()
		s.Nil(err3)
		closedCount = len(resp.Executions)
		if closedCount == 0 {
			s.Logger.Info("Closed WorkflowExecution is not yet visibile")
			time.Sleep(1000 * time.Millisecond)
			continue ListClosedLoop
		}
		break ListClosedLoop
	}
	s.Equal(1, closedCount)
}

func (s *IntegrationSuite) TestDecisionTaskFailed() {
	id := "integration-decisiontask-failed-test"
	wt := "integration-decisiontask-failed-test-type"
	tl := "integration-decisiontask-failed-test-tasklist"
	identity := "worker1"
	activityName := "activity_type1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	workflowComplete := false
	activityScheduled := false
	activityData := int32(1)
	failureCount := 10
	signalCount := 0
	sendSignal := false
	lastDecisionTimestamp := int64(0)
	// var signalEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		// Count signals
		for _, event := range history.Events[previousStartedEventID:] {
			if event.GetEventType() == types.EventTypeWorkflowExecutionSignaled {
				signalCount++
			}
		}
		// Some signals received on this decision
		if signalCount == 1 {
			return nil, []*types.Decision{}, nil
		}

		// Send signals during decision
		if sendSignal {
			s.sendSignal(s.DomainName, workflowExecution, "signalC", nil, identity)
			s.sendSignal(s.DomainName, workflowExecution, "signalD", nil, identity)
			s.sendSignal(s.DomainName, workflowExecution, "signalE", nil, identity)
			sendSignal = false
		}

		if !activityScheduled {
			activityScheduled = true
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityData))

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    "1",
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if failureCount > 0 {
			// Otherwise decrement failureCount and keep failing decisions
			failureCount--
			return nil, nil, errors.New("Decider Panic")
		}

		workflowComplete = true
		time.Sleep(time.Second)
		s.Logger.Warn(fmt.Sprintf("PrevStarted: %v, StartedEventID: %v, Size: %v", previousStartedEventID, startedEventID,
			len(history.Events)))
		lastDecisionEvent := history.Events[startedEventID-1]
		s.Equal(types.EventTypeDecisionTaskStarted, lastDecisionEvent.GetEventType())
		lastDecisionTimestamp = lastDecisionEvent.GetTimestamp()
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	// activity handler
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {

		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// process activity
	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("PollAndProcessActivityTask", tag.Error(err))
	s.Nil(err)

	// fail decision 5 times
	for i := 0; i < 5; i++ {
		_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, false, int64(i))
		s.Nil(err)
	}

	err = s.sendSignal(s.DomainName, workflowExecution, "signalA", nil, identity)
	s.Nil(err, "failed to send signal to execution")

	// process signal
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.Equal(1, signalCount)

	// send another signal to trigger decision
	err = s.sendSignal(s.DomainName, workflowExecution, "signalB", nil, identity)
	s.Nil(err, "failed to send signal to execution")

	// fail decision 2 more times
	for i := 0; i < 2; i++ {
		_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, false, int64(i))
		s.Nil(err)
	}
	s.Equal(3, signalCount)

	// now send a signal during failed decision
	sendSignal = true
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, false, int64(2))
	s.Nil(err)
	s.Equal(4, signalCount)

	// fail decision 1 more times
	for i := 0; i < 2; i++ {
		_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, false, int64(i))
		s.Nil(err)
	}
	s.Equal(12, signalCount)

	// Make complete workflow decision
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, int64(2))
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.True(workflowComplete)
	s.Equal(16, signalCount)

	events := s.getHistory(s.DomainName, workflowExecution)
	var lastEvent *types.HistoryEvent
	var lastDecisionStartedEvent *types.HistoryEvent
	lastIdx := 0
	for i, e := range events {
		if e.GetEventType() == types.EventTypeDecisionTaskStarted {
			lastDecisionStartedEvent = e
			lastIdx = i
		}
		lastEvent = e
	}
	s.Equal(types.EventTypeWorkflowExecutionCompleted, lastEvent.GetEventType())
	s.Logger.Info(fmt.Sprintf("Last Decision Time: %v, Last Decision History Timestamp: %v, Complete Timestamp: %v",
		time.Unix(0, lastDecisionTimestamp), time.Unix(0, lastDecisionStartedEvent.GetTimestamp()),
		time.Unix(0, lastEvent.GetTimestamp())))
	s.Equal(lastDecisionTimestamp, lastDecisionStartedEvent.GetTimestamp())
	s.True(time.Duration(lastEvent.GetTimestamp()-lastDecisionTimestamp) >= time.Second)

	s.Equal(2, len(events)-lastIdx-1)
	decisionCompletedEvent := events[lastIdx+1]
	workflowCompletedEvent := events[lastIdx+2]
	s.Equal(types.EventTypeDecisionTaskCompleted, decisionCompletedEvent.GetEventType())
	s.Equal(types.EventTypeWorkflowExecutionCompleted, workflowCompletedEvent.GetEventType())
}

func (s *IntegrationSuite) TestGetPollerHistory() {
	WorkflowID := "integration-get-poller-history"
	workflowTypeName := "integration-get-poller-history-type"
	tasklistName := "integration-get-poller-history-tasklist"
	identity := "worker1"
	activityName := "activity_type1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = workflowTypeName

	taskList := &types.TaskList{}
	taskList.Name = tasklistName

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          WorkflowID,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
	// var signalEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !activityScheduled {
			activityScheduled = true
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityData))

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    "1",
					ActivityType:                  &types.ActivityType{Name: activityName},
					TaskList:                      taskList,
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(25),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(25),
				},
			}}, nil
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// this function poll events from history side
	testDescribeTaskList := func(domain string, tasklist *types.TaskList, tasklistType types.TaskListType) []*types.PollerInfo {
		ctx, cancel := createContext()
		defer cancel()
		listResp, err := s.Engine.ListTaskListPartitions(ctx, &types.ListTaskListPartitionsRequest{
			Domain:   domain,
			TaskList: tasklist,
		})
		s.NoError(err)
		pollers := make(map[string]*types.PollerInfo)
		var partitions []*types.TaskListPartitionMetadata
		if tasklistType == types.TaskListTypeActivity {
			partitions = listResp.ActivityTaskListPartitions
		} else {
			partitions = listResp.DecisionTaskListPartitions
		}
		for _, partition := range partitions {
			responseInner, errInner := s.Engine.DescribeTaskList(ctx, &types.DescribeTaskListRequest{
				Domain:       domain,
				TaskList:     &types.TaskList{Name: partition.Key, Kind: tasklist.Kind},
				TaskListType: &tasklistType,
			})
			s.Nil(errInner)
			for _, poller := range responseInner.Pollers {
				pollers[poller.GetIdentity()] = poller
			}
		}
		var results []*types.PollerInfo
		for _, poller := range pollers {
			results = append(results, poller)
		}
		return results
	}

	before := time.Now()

	// when no one polling on the tasklist (activity or decition), there shall be no poller information
	pollerInfos := testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeActivity)
	s.Empty(pollerInfos)
	pollerInfos = testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeDecision)
	s.Empty(pollerInfos)

	_, errDecision := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(errDecision)
	pollerInfos = testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeActivity)
	s.Empty(pollerInfos)
	pollerInfos = testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeDecision)
	s.Equal(1, len(pollerInfos))
	s.Equal(identity, pollerInfos[0].GetIdentity())
	s.True(time.Unix(0, pollerInfos[0].GetLastAccessTime()).After(before))
	s.NotEmpty(pollerInfos[0].GetLastAccessTime())

	errActivity := poller.PollAndProcessActivityTask(false)
	s.Nil(errActivity)
	pollerInfos = testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeActivity)
	s.Equal(1, len(pollerInfos))
	s.Equal(identity, pollerInfos[0].GetIdentity())
	s.True(time.Unix(0, pollerInfos[0].GetLastAccessTime()).After(before))
	s.NotEmpty(pollerInfos[0].GetLastAccessTime())
	pollerInfos = testDescribeTaskList(s.DomainName, taskList, types.TaskListTypeDecision)
	s.Equal(1, len(pollerInfos))
	s.Equal(identity, pollerInfos[0].GetIdentity())
	s.True(time.Unix(0, pollerInfos[0].GetLastAccessTime()).After(before))
	s.NotEmpty(pollerInfos[0].GetLastAccessTime())
}

func (s *IntegrationSuite) TestTransientDecisionTimeout() {
	id := "integration-transient-decision-timeout-test"
	wt := "integration-transient-decision-timeout-test-type"
	tl := "integration-transient-decision-timeout-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	workflowComplete := false
	failDecision := true
	signalCount := 0
	// var signalEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if failDecision {
			failDecision = false
			return nil, nil, errors.New("Decider Panic")
		}

		// Count signals
		for _, event := range history.Events[previousStartedEventID:] {
			if event.GetEventType() == types.EventTypeWorkflowExecutionSignaled {
				signalCount++
			}
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// First decision immediately fails and schedules a transient decision
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// Now send a signal when transient decision is scheduled
	err = s.sendSignal(s.DomainName, workflowExecution, "signalA", nil, identity)
	s.Nil(err, "failed to send signal to execution")

	// Drop decision task to cause a Decision Timeout
	_, err = poller.PollAndProcessDecisionTask(false, true)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// Now process signal and complete workflow execution
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, int64(1))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.Equal(1, signalCount)
	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestNoTransientDecisionAfterFlushBufferedEvents() {
	id := "integration-no-transient-decision-after-flush-buffered-events-test"
	wt := "integration-no-transient-decision-after-flush-buffered-events-test-type"
	tl := "integration-no-transient-decision-after-flush-buffered-events-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(20),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	workflowComplete := false
	continueAsNewAndSignal := false
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !continueAsNewAndSignal {
			continueAsNewAndSignal = true
			ctx, cancel := createContext()
			defer cancel()
			// this will create new event when there is in-flight decision task, and the new event will be buffered
			err := s.Engine.SignalWorkflowExecution(ctx,
				&types.SignalWorkflowExecutionRequest{
					Domain: s.DomainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: id,
					},
					SignalName: "buffered-signal-1",
					Input:      []byte("buffered-signal-input"),
					Identity:   identity,
				})
			s.NoError(err)

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
				ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					WorkflowType:                        workflowType,
					TaskList:                            taskList,
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1000),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(100),
				},
			}}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// fist decision, this try to do a continue as new but there is a buffered event,
	// so it will fail and create a new decision
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// second decision, which will complete the workflow
	// this expect the decision to have attempt == 0
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, 0)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestRelayDecisionTimeout() {
	id := "integration-relay-decision-timeout-test"
	wt := "integration-relay-decision-timeout-test-type"
	tl := "integration-relay-decision-timeout-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	workflowComplete, isFirst := false, true
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if isFirst {
			isFirst = false
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "test-marker",
				},
			}}, nil
		}
		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// First decision task complete with a marker decision, and request to relay decision (immediately return a new decision task)
	_, newTask, err := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		false,
		false,
		0,
		3,
		true,
		nil)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(newTask)
	s.NotNil(newTask.DecisionTask)

	time.Sleep(time.Second * 2) // wait 2s for relay decision to timeout
	decisionTaskTimeout := false
	for i := 0; i < 3; i++ {
		events := s.getHistory(s.DomainName, workflowExecution)
		if len(events) >= 8 {
			s.Equal(types.EventTypeDecisionTaskTimedOut, events[7].GetEventType())
			s.Equal(types.TimeoutTypeStartToClose, events[7].DecisionTaskTimedOutEventAttributes.GetTimeoutType())
			decisionTaskTimeout = true
			break
		}
		time.Sleep(time.Second)
	}
	// verify relay decision task timeout
	s.True(decisionTaskTimeout)

	// Now complete workflow
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, int64(1))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.True(workflowComplete)
}

func (s *IntegrationSuite) TestTaskProcessingProtectionForRateLimitError() {
	id := "integration-task-processing-protection-for-rate-limit-error-test"
	wt := "integration-task-processing-protection-for-rate-limit-error-test-type"
	tl := "integration-task-processing-protection-for-rate-limit-error-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(601),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(600),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	workflowComplete := false
	signalCount := 0
	createUserTimer := false
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, h *types.History) ([]byte, []*types.Decision, error) {

		if !createUserTimer {
			createUserTimer = true

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeStartTimer.Ptr(),
				StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
					TimerID:                   "timer-id-1",
					StartToFireTimeoutSeconds: common.Int64Ptr(5),
				},
			}}, nil
		}

		// Count signals
		for _, event := range h.Events[previousStartedEventID:] {
			if event.GetEventType() == types.EventTypeWorkflowExecutionSignaled {
				signalCount++
			}
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Process first decision to create user timer
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// Send one signal to create a new decision
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, int64(0))
	s.Nil(s.sendSignal(s.DomainName, workflowExecution, "SignalName", buf.Bytes(), identity))

	// Drop decision to cause all events to be buffered from now on
	_, err = poller.PollAndProcessDecisionTask(false, true)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// Buffered 100 Signals
	for i := 1; i < 101; i++ {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, int64(i))
		s.Nil(s.sendSignal(s.DomainName, workflowExecution, "SignalName", buf.Bytes(), identity))
	}

	// 101 signal, which will fail the decision
	buf = new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, int64(101))
	signalErr := s.sendSignal(s.DomainName, workflowExecution, "SignalName", buf.Bytes(), identity)
	s.Nil(signalErr)

	// Process signal in decider
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, false, 0)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.True(workflowComplete)
	s.Equal(102, signalCount)
}

func (s *IntegrationSuite) TestStickyTimeout_NonTransientDecision() {
	id := "integration-sticky-timeout-non-transient-decision"
	wt := "integration-sticky-timeout-non-transient-decision-type"
	tl := "integration-sticky-timeout-non-transient-decision-tasklist"
	stl := "integration-sticky-timeout-non-transient-decision-tasklist-sticky"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stl
	stickyScheduleToStartTimeoutSeconds := common.Int32Ptr(2)

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	localActivityDone := false
	failureCount := 5
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !localActivityDone {
			localActivityDone = true

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "local activity marker",
					Details:    []byte("local activity data"),
				},
			}}, nil
		}

		if failureCount > 0 {
			// send a signal on third failure to be buffered, forcing a non-transient decision when buffer is flushed
			/*if failureCount == 3 {
				err := s.Engine.SignalWorkflowExecution(createContext(), &types.SignalWorkflowExecutionRequest{
					Domain:            s.DomainName,
					WorkflowExecution: workflowExecution,
					SignalName:        common.StringPtr("signalB"),
					Input:             []byte("signal input"),
					Identity:          identity,
					RequestID:         uuid.New(),
				})
				s.Nil(err)
			}*/
			failureCount--
			return nil, nil, errors.New("non deterministic error")
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:                              s.Engine,
		Domain:                              s.DomainName,
		TaskList:                            taskList,
		Identity:                            identity,
		DecisionHandler:                     dtHandler,
		Logger:                              s.Logger,
		T:                                   s.T(),
		StickyTaskList:                      stickyTaskList,
		StickyScheduleToStartTimeoutSeconds: stickyScheduleToStartTimeoutSeconds,
	}

	_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, true, int64(0))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.DomainName,
		WorkflowExecution: workflowExecution,
		SignalName:        "signalA",
		Input:             []byte("signal input"),
		Identity:          identity,
		RequestID:         uuid.New(),
	})
	s.NoError(err)

	// Wait for decision timeout
	stickyTimeout := false
WaitForStickyTimeoutLoop:
	for i := 0; i < 10; i++ {
		events := s.getHistory(s.DomainName, workflowExecution)
		for _, event := range events {
			if event.GetEventType() == types.EventTypeDecisionTaskTimedOut {
				s.Equal(types.TimeoutTypeScheduleToStart, event.DecisionTaskTimedOutEventAttributes.GetTimeoutType())
				stickyTimeout = true
				break WaitForStickyTimeoutLoop
			}
		}
		time.Sleep(time.Second)
	}
	s.True(stickyTimeout, "Decision not timed out.")

	for i := 0; i < 3; i++ {
		_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(i))
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err)
	}

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.DomainName,
		WorkflowExecution: workflowExecution,
		SignalName:        "signalB",
		Input:             []byte("signal input"),
		Identity:          identity,
		RequestID:         uuid.New(),
	})
	s.Nil(err)

	for i := 0; i < 2; i++ {
		_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(i))
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err)
	}

	decisionTaskFailed := false
	events := s.getHistory(s.DomainName, workflowExecution)
	for _, event := range events {
		if event.GetEventType() == types.EventTypeDecisionTaskFailed {
			decisionTaskFailed = true
			break
		}
	}
	s.True(decisionTaskFailed)

	// Complete workflow execution
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(2))
	s.NoError(err)

	// Assert for single decision task failed and workflow completion
	failedDecisions := 0
	workflowComplete := false
	events = s.getHistory(s.DomainName, workflowExecution)
	for _, event := range events {
		switch event.GetEventType() {
		case types.EventTypeDecisionTaskFailed:
			failedDecisions++
		case types.EventTypeWorkflowExecutionCompleted:
			workflowComplete = true
		}
	}
	s.True(workflowComplete, "Workflow not complete")
	s.Equal(2, failedDecisions, "Mismatched failed decision count")
}

func (s *IntegrationSuite) TestStickyTasklistResetThenTimeout() {
	id := "integration-reset-sticky-fire-schedule-to-start-timeout"
	wt := "integration-reset-sticky-fire-schedule-to-start-timeout-type"
	tl := "integration-reset-sticky-fire-schedule-to-start-timeout-tasklist"
	stl := "integration-reset-sticky-fire-schedule-to-start-timeout-tasklist-sticky"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stl
	stickyScheduleToStartTimeoutSeconds := common.Int32Ptr(2)

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	localActivityDone := false
	failureCount := 5
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !localActivityDone {
			localActivityDone = true

			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "local activity marker",
					Details:    []byte("local activity data"),
				},
			}}, nil
		}

		if failureCount > 0 {
			failureCount--
			return nil, nil, errors.New("non deterministic error")
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:                              s.Engine,
		Domain:                              s.DomainName,
		TaskList:                            taskList,
		Identity:                            identity,
		DecisionHandler:                     dtHandler,
		Logger:                              s.Logger,
		T:                                   s.T(),
		StickyTaskList:                      stickyTaskList,
		StickyScheduleToStartTimeoutSeconds: stickyScheduleToStartTimeoutSeconds,
	}

	_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, true, int64(0))
	s.Logger.Info("PollAndProcessDecisionTask: %v", tag.Error(err))
	s.Nil(err)

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.DomainName,
		WorkflowExecution: workflowExecution,
		SignalName:        "signalA",
		Input:             []byte("signal input"),
		Identity:          identity,
		RequestID:         uuid.New(),
	})
	s.NoError(err)

	ctx, cancel = createContext()
	defer cancel()
	// Reset sticky tasklist before sticky decision task starts
	s.Engine.ResetStickyTaskList(ctx, &types.ResetStickyTaskListRequest{
		Domain:    s.DomainName,
		Execution: workflowExecution,
	})

	// Wait for decision timeout
	stickyTimeout := false
WaitForStickyTimeoutLoop:
	for i := 0; i < 10; i++ {
		events := s.getHistory(s.DomainName, workflowExecution)
		for _, event := range events {
			if event.GetEventType() == types.EventTypeDecisionTaskTimedOut {
				s.Equal(types.TimeoutTypeScheduleToStart, event.DecisionTaskTimedOutEventAttributes.GetTimeoutType())
				stickyTimeout = true
				break WaitForStickyTimeoutLoop
			}
		}
		time.Sleep(time.Second)
	}
	s.True(stickyTimeout, "Decision not timed out.")

	for i := 0; i < 3; i++ {
		// we will jump from sticky decision to transient decision in this case (decisionAttempt increased)
		// even though the error is sticky scheduleToStart timeout
		// since we can't tell if the decision is a sticky decision or not when the timer fires
		// (stickiness cleared by the ResetStickyTaskList API call above)
		_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(i+1))
		s.Logger.Info("PollAndProcessDecisionTask: %v", tag.Error(err))
		s.Nil(err)
	}

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.DomainName,
		WorkflowExecution: workflowExecution,
		SignalName:        "signalB",
		Input:             []byte("signal input"),
		Identity:          identity,
		RequestID:         uuid.New(),
	})
	s.Nil(err)

	for i := 0; i < 2; i++ {
		_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(i))
		s.Logger.Info("PollAndProcessDecisionTask: %v", tag.Error(err))
		s.Nil(err)
	}

	decisionTaskFailed := false
	events := s.getHistory(s.DomainName, workflowExecution)
	for _, event := range events {
		if event.GetEventType() == types.EventTypeDecisionTaskFailed {
			decisionTaskFailed = true
			break
		}
	}
	s.True(decisionTaskFailed)

	// Complete workflow execution
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(true, false, false, true, int64(2))
	s.NoError(err)

	// Assert for single decision task failed and workflow completion
	failedDecisions := 0
	workflowComplete := false
	events = s.getHistory(s.DomainName, workflowExecution)
	for _, event := range events {
		switch event.GetEventType() {
		case types.EventTypeDecisionTaskFailed:
			failedDecisions++
		case types.EventTypeWorkflowExecutionCompleted:
			workflowComplete = true
		}
	}
	s.True(workflowComplete, "Workflow not complete")
	s.Equal(1, failedDecisions, "Mismatched failed decision count")
}

func (s *IntegrationSuite) TestBufferedEventsOutOfOrder() {
	id := "integration-buffered-events-out-of-order-test"
	wt := "integration-buffered-events-out-of-order-test-type"
	tl := "integration-buffered-events-out-of-order-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(20),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	// decider logic
	workflowComplete := false
	firstDecision := false
	secondDecision := false
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		s.Logger.Info(fmt.Sprintf("Decider called: first: %v, second: %v, complete: %v\n", firstDecision, secondDecision, workflowComplete))

		if !firstDecision {
			firstDecision = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "some random marker name",
					Details:    []byte("some random marker details"),
				},
			}, {
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    "Activity-1",
					ActivityType:                  &types.ActivityType{Name: "ActivityType"},
					Domain:                        s.DomainName,
					TaskList:                      taskList,
					Input:                         []byte("some random activity input"),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(100),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(100),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(100),
				},
			}}, nil
		}

		if !secondDecision {
			secondDecision = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "some random marker name",
					Details:    []byte("some random marker details"),
				},
			}}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}
	// activity handler
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// first decision, which will schedule an activity and add marker
	_, task, err := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		true,
		false,
		false,
		false,
		int64(0),
		1,
		true,
		nil)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// This will cause activity start and complete to be buffered
	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("pollAndProcessActivityTask", tag.Error(err))
	s.Nil(err)

	// second decision, completes another local activity and forces flush of buffered activity events
	newDecisionTask := task.GetDecisionTask()
	s.NotNil(newDecisionTask)
	task, err = poller.HandlePartialDecision(newDecisionTask)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(task)

	// third decision, which will close workflow
	newDecisionTask = task.GetDecisionTask()
	s.NotNil(newDecisionTask)
	task, err = poller.HandlePartialDecision(newDecisionTask)
	s.Logger.Info("pollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.Nil(task.DecisionTask)

	events := s.getHistory(s.DomainName, workflowExecution)
	var scheduleEvent, startedEvent, completedEvent *types.HistoryEvent
	for _, event := range events {
		switch event.GetEventType() {
		case types.EventTypeActivityTaskScheduled:
			scheduleEvent = event
		case types.EventTypeActivityTaskStarted:
			startedEvent = event
		case types.EventTypeActivityTaskCompleted:
			completedEvent = event
		}
	}

	s.NotNil(scheduleEvent)
	s.NotNil(startedEvent)
	s.NotNil(completedEvent)
	s.True(startedEvent.ID < completedEvent.ID)
	s.Equal(scheduleEvent.ID, startedEvent.ActivityTaskStartedEventAttributes.GetScheduledEventID())
	s.Equal(scheduleEvent.ID, completedEvent.ActivityTaskCompletedEventAttributes.GetScheduledEventID())
	s.Equal(startedEvent.ID, completedEvent.ActivityTaskCompletedEventAttributes.GetStartedEventID())
	s.True(workflowComplete)
}

type startFunc func() (*types.StartWorkflowExecutionResponse, error)

func (s *IntegrationSuite) TestStartWithMemo() {
	id := "integration-start-with-memo-test"
	wt := "integration-start-with-memo-test-type"
	tl := "integration-start-with-memo-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	memoInfo, _ := json.Marshal(id)
	memo := &types.Memo{
		Fields: map[string][]byte{
			"Info": memoInfo,
		},
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		Memo:                                memo,
	}

	fn := func() (*types.StartWorkflowExecutionResponse, error) {
		ctx, cancel := createContext()
		defer cancel()
		return s.Engine.StartWorkflowExecution(ctx, request)
	}
	s.startWithMemoHelper(fn, id, taskList, memo)
}

func (s *IntegrationSuite) TestSignalWithStartWithMemo() {
	id := "integration-signal-with-start-with-memo-test"
	wt := "integration-signal-with-start-with-memo-test-type"
	tl := "integration-signal-with-start-with-memo-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	memoInfo, _ := json.Marshal(id)
	memo := &types.Memo{
		Fields: map[string][]byte{
			"Info": memoInfo,
		},
	}

	signalName := "my signal"
	signalInput := []byte("my signal input.")
	request := &types.SignalWithStartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		SignalName:                          signalName,
		SignalInput:                         signalInput,
		Identity:                            identity,
		Memo:                                memo,
	}

	fn := func() (*types.StartWorkflowExecutionResponse, error) {
		ctx, cancel := createContext()
		defer cancel()
		return s.Engine.SignalWithStartWorkflowExecution(ctx, request)
	}
	s.startWithMemoHelper(fn, id, taskList, memo)
}

func (s *IntegrationSuite) TestCancelTimer() {
	id := "integration-cancel-timer-test"
	wt := "integration-cancel-timer-test-type"
	tl := "integration-cancel-timer-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	creatResp, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      creatResp.GetRunID(),
	}

	TimerID := "1"
	timerScheduled := false
	signalDelivered := false
	timerCancelled := false
	workflowComplete := false
	timer := int64(2000)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !timerScheduled {
			timerScheduled = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeStartTimer.Ptr(),
				StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
					TimerID:                   TimerID,
					StartToFireTimeoutSeconds: common.Int64Ptr(timer),
				},
			}}, nil
		}

		ctx, cancel := createContext()
		defer cancel()
		resp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain:          s.DomainName,
			Execution:       workflowExecution,
			MaximumPageSize: 200,
		})
		s.Nil(err)
		for _, event := range resp.History.Events {
			switch event.GetEventType() {
			case types.EventTypeWorkflowExecutionSignaled:
				signalDelivered = true
			case types.EventTypeTimerCanceled:
				timerCancelled = true
			}
		}

		if !signalDelivered {
			s.Fail("should receive a signal")
		}

		if !timerCancelled {
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeCancelTimer.Ptr(),
				CancelTimerDecisionAttributes: &types.CancelTimerDecisionAttributes{
					TimerID: TimerID,
				},
			}}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// schedule the timer
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.Nil(s.sendSignal(s.DomainName, workflowExecution, "random signal name", []byte("random signal payload"), identity))

	// receive the signal & cancel the timer
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.Nil(s.sendSignal(s.DomainName, workflowExecution, "random signal name", []byte("random signal payload"), identity))
	// complete the workflow
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.True(workflowComplete)

	ctx, cancel = createContext()
	defer cancel()
	resp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:          s.DomainName,
		Execution:       workflowExecution,
		MaximumPageSize: 200,
	})
	s.Nil(err)
	for _, event := range resp.History.Events {
		switch event.GetEventType() {
		case types.EventTypeWorkflowExecutionSignaled:
			signalDelivered = true
		case types.EventTypeTimerCanceled:
			timerCancelled = true
		case types.EventTypeTimerFired:
			s.Fail("timer got fired")
		}
	}
}

func (s *IntegrationSuite) TestCancelTimer_CancelFiredAndBuffered() {
	id := "integration-cancel-timer-fired-and-buffered-test"
	wt := "integration-cancel-timer-fired-and-buffered-test-type"
	tl := "integration-cancel-timer-fired-and-buffered-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1000),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	creatResp, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)
	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      creatResp.GetRunID(),
	}

	TimerID := 1
	timerScheduled := false
	signalDelivered := false
	timerCancelled := false
	workflowComplete := false
	timer := int64(4)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !timerScheduled {
			timerScheduled = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeStartTimer.Ptr(),
				StartTimerDecisionAttributes: &types.StartTimerDecisionAttributes{
					TimerID:                   fmt.Sprintf("%v", TimerID),
					StartToFireTimeoutSeconds: common.Int64Ptr(timer),
				},
			}}, nil
		}

		ctx, cancel := createContext()
		defer cancel()
		resp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain:          s.DomainName,
			Execution:       workflowExecution,
			MaximumPageSize: 200,
		})
		s.Nil(err)
		for _, event := range resp.History.Events {
			switch event.GetEventType() {
			case types.EventTypeWorkflowExecutionSignaled:
				signalDelivered = true
			case types.EventTypeTimerCanceled:
				timerCancelled = true
			}
		}

		if !signalDelivered {
			s.Fail("should receive a signal")
		}

		if !timerCancelled {
			time.Sleep(time.Duration(2*timer) * time.Second)
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeCancelTimer.Ptr(),
				CancelTimerDecisionAttributes: &types.CancelTimerDecisionAttributes{
					TimerID: fmt.Sprintf("%v", TimerID),
				},
			}}, nil
		}

		workflowComplete = true
		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: nil,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// schedule the timer
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.Nil(s.sendSignal(s.DomainName, workflowExecution, "random signal name", []byte("random signal payload"), identity))

	// receive the signal & cancel the timer
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.Nil(s.sendSignal(s.DomainName, workflowExecution, "random signal name", []byte("random signal payload"), identity))
	// complete the workflow
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: completed")
	s.Nil(err)

	s.True(workflowComplete)

	ctx, cancel = createContext()
	defer cancel()
	resp, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:          s.DomainName,
		Execution:       workflowExecution,
		MaximumPageSize: 200,
	})
	s.Nil(err)
	for _, event := range resp.History.Events {
		switch event.GetEventType() {
		case types.EventTypeWorkflowExecutionSignaled:
			signalDelivered = true
		case types.EventTypeTimerCanceled:
			timerCancelled = true
		case types.EventTypeTimerFired:
			s.Fail("timer got fired")
		}
	}
}

// helper function for TestStartWithMemo and TestSignalWithStartWithMemo to reduce duplicate code
func (s *IntegrationSuite) startWithMemoHelper(startFn startFunc, id string, taskList *types.TaskList, memo *types.Memo) {
	identity := "worker1"

	we, err0 := startFn()
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution: response", tag.WorkflowRunID(we.RunID))

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		return []byte(strconv.Itoa(1)), []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// verify open visibility
	var openExecutionInfo *types.WorkflowExecutionInfo
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err1 := s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: &types.StartTimeFilter{
				EarliestTime: common.Int64Ptr(0),
				LatestTime:   common.Int64Ptr(time.Now().UnixNano()),
			},
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: id,
			},
		})
		cancel()
		s.Nil(err1)
		if len(resp.Executions) == 1 {
			openExecutionInfo = resp.Executions[0]
			break
		}
		s.Logger.Info("Open WorkflowExecution is not yet visible")
		time.Sleep(100 * time.Millisecond)
	}
	s.NotNil(openExecutionInfo)
	s.Equal(memo, openExecutionInfo.Memo)

	// make progress of workflow
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask: %v", tag.Error(err))
	s.Nil(err)

	// verify history
	execution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, historyErr := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    s.DomainName,
		Execution: execution,
	})
	s.Nil(historyErr)
	history := historyResponse.History
	firstEvent := history.Events[0]
	s.Equal(types.EventTypeWorkflowExecutionStarted, firstEvent.GetEventType())
	startdEventAttributes := firstEvent.WorkflowExecutionStartedEventAttributes
	s.Equal(memo, startdEventAttributes.Memo)

	// verify DescribeWorkflowExecution result
	descRequest := &types.DescribeWorkflowExecutionRequest{
		Domain:    s.DomainName,
		Execution: execution,
	}
	ctx, cancel = createContext()
	defer cancel()
	descResp, err := s.Engine.DescribeWorkflowExecution(ctx, descRequest)
	s.Nil(err)
	s.Equal(memo, descResp.WorkflowExecutionInfo.Memo)

	// verify closed visibility
	var closedExecutionInfo *types.WorkflowExecutionInfo
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp, err1 := s.Engine.ListClosedWorkflowExecutions(ctx, &types.ListClosedWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: 100,
			StartTimeFilter: &types.StartTimeFilter{
				EarliestTime: common.Int64Ptr(0),
				LatestTime:   common.Int64Ptr(time.Now().UnixNano()),
			},
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: id,
			},
		})
		cancel()
		s.Nil(err1)
		if len(resp.Executions) == 1 {
			closedExecutionInfo = resp.Executions[0]
			break
		}
		s.Logger.Info("Closed WorkflowExecution is not yet visible")
		time.Sleep(100 * time.Millisecond)
	}
	s.NotNil(closedExecutionInfo)
	s.Equal(memo, closedExecutionInfo.Memo)
}

func (s *IntegrationSuite) sendSignal(domainName string, execution *types.WorkflowExecution, signalName string,
	input []byte, identity string) error {
	ctx, cancel := createContext()
	defer cancel()
	return s.Engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            domainName,
		WorkflowExecution: execution,
		SignalName:        signalName,
		Input:             input,
		Identity:          identity,
	})
}

// TestDescribeCluster tests that DescribeCluster API returns a valid response without error
func (s *IntegrationSuite) TestDescribeCluster() {
	ctx, cancel := createContext()
	defer cancel()

	response, err := s.AdminClient.DescribeCluster(ctx)
	s.Require().NoError(err)
	s.Require().NotNil(response)
}
