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
	"strconv"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestWorkflowRetryPolicyTimeout() {
	id := "integration-workflow-retry-policy-timeout-test"
	wt := "integration-workflow-retry-policy-timeout-test-type"
	tl := "integration-workflow-retry-policy-timeout-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

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
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(3),
		Identity:                            identity,
		RetryPolicy:                         retryPolicy,
	}
	now := time.Now()
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	time.Sleep(3 * time.Second) // wait 1 second for timeout

	ctx, cancel = createContext()
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
		},
	})
	s.Nil(err)
	history := historyResponse.History
	firstEvent := history.Events[0]
	s.NotNil(firstEvent)
	s.Equal(firstEvent.EventType, types.EventTypeWorkflowExecutionStarted.Ptr())
	expirationTime := firstEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp
	s.NotNil(expirationTime)
	delta := time.Unix(0, *expirationTime).Sub(now)
	s.True(delta > 0*time.Second)
	s.True(delta < 2*time.Second)

	lastEvent := history.Events[len(history.Events)-1]
	delta = time.Unix(0, common.Int64Default(lastEvent.Timestamp)).Sub(time.Unix(0, common.Int64Default(firstEvent.Timestamp)))
	s.True(delta > 2*time.Second)
	s.True(delta < 4*time.Second)
}

func (s *IntegrationSuite) TestWorkflowRetryPolicyContinueAsNewAsRetry() {
	id := "integration-workflow-retry-policy-continue-as-new-retry-test"
	wt := "integration-workflow-retry-policy-continue-as-new-retry-test-type"
	tl := "integration-workflow-retry-policy-continue-as-new-retry-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	retryPolicy := &types.RetryPolicy{
		InitialIntervalInSeconds:    1,
		BackoffCoefficient:          2.0,
		MaximumIntervalInSeconds:    3,
		ExpirationIntervalInSeconds: 3,
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
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		RetryPolicy:                         retryPolicy,
	}
	now := time.Now()
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		return []byte(strconv.Itoa(0)), []*types.Decision{{
			DecisionType: types.DecisionTypeFailWorkflowExecution.Ptr(),
			FailWorkflowExecutionDecisionAttributes: &types.FailWorkflowExecutionDecisionAttributes{
				Reason: common.StringPtr("fail-reason"),
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
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	time.Sleep(3 * time.Second) // wait 1 second for timeout

	var history *types.History
	var lastEvent *types.HistoryEvent

GetHistoryLoop:
	for i := 0; i < 20; i++ {
		ctx, cancel := createContext()
		historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
			},
		})
		cancel()
		s.Nil(err)
		history = historyResponse.History

		lastEvent = history.Events[len(history.Events)-1]
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionTimedOut {
			s.Logger.Warn("Execution not timedout yet.")
			time.Sleep(200 * time.Millisecond)
			continue GetHistoryLoop
		}

		timeoutEventAttributes := lastEvent.WorkflowExecutionTimedOutEventAttributes
		s.Equal(types.TimeoutTypeStartToClose, *timeoutEventAttributes.TimeoutType)
		break GetHistoryLoop
	}

	firstEvent := history.Events[0]
	s.Equal(firstEvent.WorkflowExecutionStartedEventAttributes.Attempt, int32(1))

	delta := time.Unix(0, common.Int64Default(firstEvent.WorkflowExecutionStartedEventAttributes.ExpirationTimestamp)).Sub(now)
	s.True(delta > 2*time.Second)
	s.True(delta < 4*time.Second)
}
