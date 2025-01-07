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
	"context"
	"encoding/binary"
	"errors"
	"sync/atomic"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestQueryWorkflow_Sticky() {
	id := "interation-query-workflow-test-sticky"
	wt := "interation-query-workflow-test-sticky-type"
	tl := "interation-query-workflow-test-sticky-tasklist"
	stl := "interation-query-workflow-test-sticky-tasklist-sticky"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stl
	stickyScheduleToStartTimeoutSeconds := common.Int32Ptr(10)

	// decider logic
	activityScheduled := false
	activityData := int32(1)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
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

	// activity handler
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {

		return []byte("Activity Result."), false, nil
	}

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:                              s.engine,
		Domain:                              s.domainName,
		TaskList:                            taskList,
		Identity:                            identity,
		DecisionHandler:                     dtHandler,
		ActivityHandler:                     atHandler,
		QueryHandler:                        queryHandler,
		Logger:                              s.Logger,
		T:                                   s.T(),
		StickyTaskList:                      stickyTaskList,
		StickyScheduleToStartTimeoutSeconds: stickyScheduleToStartTimeoutSeconds,
	}

	// Make a request with stick tasklist to refresh the stickiness, otherwise we won't be able to add
	// decisions to the sticky tasklist
	ctx, cancel := createContext()
	defer cancel()
	resp, err := poller.Engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   poller.Domain,
		TaskList: poller.StickyTaskList,
		Identity: poller.Identity,
	})
	s.NotNil(resp)
	s.Equal(0, len(resp.TaskToken))
	s.Nil(err)

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
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
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution: response", tag.WorkflowRunID(we.RunID))

	// Make first decision to schedule activity
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, true, int64(0))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string) {
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
		})
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// call QueryWorkflow in separate goroutinue (because it is blocking). That will generate a query task
	go queryWorkflowFn(queryType)
	// process that query task, which should respond via RespondQueryTaskCompleted
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTaskWithSticky(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	}
	// wait until query result is ready
	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)

	go queryWorkflowFn("invalid-query-type")
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTaskWithSticky(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	}
	queryResult = <-queryResultCh
	s.NotNil(queryResult.Err)
	queryFailError, ok := queryResult.Err.(*types.QueryFailedError)
	s.True(ok)
	s.Equal("unknown-query-type", queryFailError.Message)
}

func (s *IntegrationSuite) TestQueryWorkflow_StickyTimeout() {
	id := "interation-query-workflow-test-sticky-timeout"
	wt := "interation-query-workflow-test-sticky-timeout-type"
	tl := "interation-query-workflow-test-sticky-timeout-tasklist"
	stl := "interation-query-workflow-test-sticky-timeout-tasklist-sticky"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stl
	stickyScheduleToStartTimeoutSeconds := common.Int32Ptr(10)

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
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
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
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

	// activity handler
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {

		return []byte("Activity Result."), false, nil
	}

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:                              s.engine,
		Domain:                              s.domainName,
		TaskList:                            taskList,
		Identity:                            identity,
		DecisionHandler:                     dtHandler,
		ActivityHandler:                     atHandler,
		QueryHandler:                        queryHandler,
		Logger:                              s.Logger,
		T:                                   s.T(),
		StickyTaskList:                      stickyTaskList,
		StickyScheduleToStartTimeoutSeconds: stickyScheduleToStartTimeoutSeconds,
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, true, int64(0))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string) {
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
		})
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// call QueryWorkflow in separate goroutinue (because it is blocking). That will generate a query task
	go queryWorkflowFn(queryType)
	// process that query task, which should respond via RespondQueryTaskCompleted
	for {
		// loop until process the query task
		// here we poll on normal tasklist, to simulate a worker crash and restart
		// on the server side, server will first try the sticky tasklist and then the normal tasklist
		isQueryTask, errInner := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	}
	// wait until query result is ready
	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)
}

func (s *IntegrationSuite) TestQueryWorkflow_NonSticky() {
	id := "integration-query-workflow-test-non-sticky"
	wt := "integration-query-workflow-test-non-sticky-type"
	tl := "integration-query-workflow-test-non-sticky-tasklist"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
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
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
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

	// activity handler
	atHandler := func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		return []byte("Activity Result."), false, nil
	}

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:          s.engine,
		Domain:          s.domainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		QueryHandler:    queryHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string, rejectCondition *types.QueryRejectCondition) {
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
			QueryRejectCondition: rejectCondition,
		})
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// call QueryWorkflow in separate goroutinue (because it is blocking). That will generate a query task
	go queryWorkflowFn(queryType, nil)
	// process that query task, which should respond via RespondQueryTaskCompleted
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	} // wait until query result is ready
	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)

	go queryWorkflowFn("invalid-query-type", nil)
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	}
	queryResult = <-queryResultCh
	s.NotNil(queryResult.Err)
	queryFailError, ok := queryResult.Err.(*types.QueryFailedError)
	s.True(ok)
	s.Equal("unknown-query-type", queryFailError.Message)

	// advance the state of the decider
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)

	go queryWorkflowFn(queryType, nil)
	// process that query task, which should respond via RespondQueryTaskCompleted
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	} // wait until query result is ready
	queryResult = <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString = string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)

	rejectCondition := types.QueryRejectConditionNotOpen
	go queryWorkflowFn(queryType, &rejectCondition)
	queryResult = <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.Nil(queryResult.Resp.QueryResult)
	s.NotNil(queryResult.Resp.QueryRejected.CloseStatus)
	s.Equal(types.WorkflowExecutionCloseStatusCompleted, *queryResult.Resp.QueryRejected.CloseStatus)

	rejectCondition = types.QueryRejectConditionNotCompletedCleanly
	go queryWorkflowFn(queryType, &rejectCondition)
	for {
		// loop until process the query task
		isQueryTask, errInner := poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(errInner)
		if isQueryTask {
			break
		}
	} // wait until query result is ready
	queryResult = <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString = string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)
}

func (s *IntegrationSuite) TestQueryWorkflow_Consistent_PiggybackQuery() {
	id := "integration-query-workflow-test-consistent-piggyback-query"
	wt := "integration-query-workflow-test-consistent-piggyback-query-type"
	tl := "integration-query-workflow-test-consistent-piggyback-query-tasklist"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
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
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
	var handledSignal atomic.Value
	handledSignal.Store(false)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(2),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if previousStartedEventID > 0 {
			for _, event := range history.Events[previousStartedEventID:] {
				if *event.EventType == types.EventTypeWorkflowExecutionSignaled {
					handledSignal.Store(true)
					return nil, []*types.Decision{}, nil
				}
			}
		}

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

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:          s.engine,
		Domain:          s.domainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		QueryHandler:    queryHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string, rejectCondition *types.QueryRejectCondition) {
		// before the query is answer the signal is not handled because the decision task is not dispatched
		// to the worker yet
		s.False(handledSignal.Load().(bool))
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
			QueryRejectCondition:  rejectCondition,
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		})
		// after the query is answered the signal is handled because query is consistent and since
		// signal came before query signal must be handled by the time query returns
		s.True(handledSignal.Load().(bool))
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// send signal to ensure there is an outstanding decision task to dispatch query on
	// otherwise query will just go through matching
	signalName := "my signal"
	signalInput := []byte("my signal input.")
	ctx, cancel = createContext()
	defer cancel()
	err = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		SignalName: signalName,
		Input:      signalInput,
		Identity:   identity,
	})
	s.Nil(err)

	// call QueryWorkflow in separate goroutine (because it is blocking). That will generate a query task
	// notice that the query comes after signal here but is consistent so it should reflect the state of the signal having been applied
	go queryWorkflowFn(queryType, nil)
	// ensure query has had enough time to at least start before a decision task is polled
	// if the decision task containing the signal is polled before query is started it will not impact
	// correctness but it will mean query will be able to be dispatched directly after signal
	// without being attached to the decision task signal is on
	<-time.After(time.Second)

	isQueryTask, _, errInner := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		false,
		false,
		int64(0),
		5,
		false,
		&types.WorkflowQueryResult{
			ResultType: types.QueryResultTypeAnswered.Ptr(),
			Answer:     []byte("consistent query result"),
		})

	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(errInner)
	s.False(isQueryTask)

	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("consistent query result", queryResultString)
}

func (s *IntegrationSuite) TestQueryWorkflow_Consistent_Timeout() {
	id := "integration-query-workflow-test-consistent-timeout"
	wt := "integration-query-workflow-test-consistent-timeout-type"
	tl := "integration-query-workflow-test-consistent-timeout-tasklist"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10), // ensure longer than time takes to handle signal
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10), // ensure longer than time it takes to handle signal
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50), // ensure longer than time takes to handle signal
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if previousStartedEventID > 0 {
			for _, event := range history.Events[previousStartedEventID:] {
				if *event.EventType == types.EventTypeWorkflowExecutionSignaled {
					<-time.After(3 * time.Second) // take longer to respond to the decision task than the query waits for
					return nil, []*types.Decision{}, nil
				}
			}
		}

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

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:          s.engine,
		Domain:          s.domainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		QueryHandler:    queryHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string, rejectCondition *types.QueryRejectCondition) {
		shortCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		queryResp, err := s.engine.QueryWorkflow(shortCtx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
			QueryRejectCondition:  rejectCondition,
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		})
		cancel()
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	signalName := "my signal"
	signalInput := []byte("my signal input.")
	ctx, cancel = createContext()
	defer cancel()
	err = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		SignalName: signalName,
		Input:      signalInput,
		Identity:   identity,
	})
	s.Nil(err)

	// call QueryWorkflow in separate goroutine (because it is blocking). That will generate a query task
	// notice that the query comes after signal here but is consistent so it should reflect the state of the signal having been applied
	go queryWorkflowFn(queryType, nil)
	// ensure query has had enough time to at least start before a decision task is polled
	// if the decision task containing the signal is polled before query is started it will not impact
	// correctness but it will mean query will be able to be dispatched directly after signal
	// without being attached to the decision task signal is on
	<-time.After(time.Second)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)

	// wait for query to timeout
	queryResult := <-queryResultCh
	s.Error(queryResult.Err) // got a timeout error
}

func (s *IntegrationSuite) TestQueryWorkflow_Consistent_BlockedByStarted_NonSticky() {
	id := "integration-query-workflow-test-consistent-blocked-by-started-non-sticky"
	wt := "integration-query-workflow-test-consistent-blocked-by-started-non-sticky-type"
	tl := "integration-query-workflow-test-consistent-blocked-by-started-non-sticky-tasklist"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10), // ensure longer than time takes to handle signal
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// decider logic
	activityScheduled := false
	activityData := int32(1)
	var handledSignal atomic.Value
	handledSignal.Store(false)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10), // ensure longer than time it takes to handle signal
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50), // ensure longer than time takes to handle signal
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if previousStartedEventID > 0 {
			for _, event := range history.Events[previousStartedEventID:] {
				if *event.EventType == types.EventTypeWorkflowExecutionSignaled {
					// wait for some time to force decision task to stay in started state while query is issued
					<-time.After(5 * time.Second)
					handledSignal.Store(true)
					return nil, []*types.Decision{}, nil
				}
			}
		}

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

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:          s.engine,
		Domain:          s.domainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		ActivityHandler: atHandler,
		QueryHandler:    queryHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to schedule activity
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string, rejectCondition *types.QueryRejectCondition) {
		s.False(handledSignal.Load().(bool))
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
			QueryRejectCondition:  rejectCondition,
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		})
		s.True(handledSignal.Load().(bool))
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// send a signal that will take 5 seconds to handle
	// this causes the signal to still be outstanding at the time query arrives
	signalName := "my signal"
	signalInput := []byte("my signal input.")
	ctx, cancel = createContext()
	defer cancel()
	err = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		SignalName: signalName,
		Input:      signalInput,
		Identity:   identity,
	})
	s.Nil(err)

	go func() {
		// wait for decision task for signal to get started before querying workflow
		// since signal processing takes 5 seconds and we wait only one second to issue the query
		// at the time the query comes in there will be a started decision task
		// only once signal completes can queryWorkflow unblock
		<-time.After(time.Second)
		queryWorkflowFn(queryType, nil)
	}()

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.NoError(err)
	<-time.After(time.Second)

	// query should not have been dispatched on the decision task which contains signal
	// because signal was already outstanding by the time query arrived
	select {
	case <-queryResultCh:
		s.Fail("query should not be ready yet")
	default:
	}

	// now that started decision task completes poll for next task which will be a query task
	// containing the buffered query
	isQueryTask, err := poller.PollAndProcessDecisionTask(false, false)
	s.True(isQueryTask)
	s.NoError(err)

	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("query-result", queryResultString)
}

func (s *IntegrationSuite) TestQueryWorkflow_Consistent_NewDecisionTask_Sticky() {
	id := "integration-query-workflow-test-consistent-new-decision-task-sticky"
	wt := "integration-query-workflow-test-consistent-new-decision-task-sticky-type"
	tl := "integration-query-workflow-test-consistent-new-decision-task-sticky-tasklist"
	stl := "integration-query-workflow-test-consistent-new-decision-task-sticky-tasklist-sticky"
	identity := "worker1"
	activityName := "activity_type1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	stickyTaskList := &types.TaskList{}
	stickyTaskList.Name = stl
	stickyTaskList.Kind = types.TaskListKindSticky.Ptr()
	stickyScheduleToStartTimeoutSeconds := common.Int32Ptr(10)

	// decider logic
	activityScheduled := false
	activityData := int32(1)
	var handledSignal atomic.Value
	handledSignal.Store(false)
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
					TaskList:                      &types.TaskList{Name: tl},
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(10), // ensure longer than time it takes to handle signal
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50), // ensure longer than time takes to handle signal
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			}}, nil
		} else if previousStartedEventID > 0 {
			for _, event := range history.Events {
				if *event.EventType == types.EventTypeWorkflowExecutionSignaled {
					// wait for some time to force decision task to stay in started state while query is issued
					<-time.After(5 * time.Second)
					handledSignal.Store(true)
					return nil, []*types.Decision{}, nil
				}
			}
		}

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

	queryHandler := func(task *types.PollForDecisionTaskResponse) ([]byte, error) {
		s.NotNil(task.Query)
		s.NotNil(task.Query.QueryType)
		if task.Query.QueryType == queryType {
			return []byte("query-result"), nil
		}

		return nil, errors.New("unknown-query-type")
	}

	poller := &TaskPoller{
		Engine:                              s.engine,
		Domain:                              s.domainName,
		TaskList:                            taskList,
		Identity:                            identity,
		DecisionHandler:                     dtHandler,
		ActivityHandler:                     atHandler,
		QueryHandler:                        queryHandler,
		Logger:                              s.Logger,
		T:                                   s.T(),
		StickyTaskList:                      stickyTaskList,
		StickyScheduleToStartTimeoutSeconds: stickyScheduleToStartTimeoutSeconds,
	}

	// Make a request with stick tasklist to refresh the stickiness, otherwise we won't be able to add
	// decisions to the sticky tasklist
	ctx, cancel := createContext()
	defer cancel()
	resp, err := poller.Engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   poller.Domain,
		TaskList: poller.StickyTaskList,
		Identity: poller.Identity,
	})
	s.NotNil(resp)
	s.Equal(0, len(resp.TaskToken))
	s.Nil(err)

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10), // ensure longer than time takes to handle signal
		Identity:                            identity,
	}

	ctx, cancel = createContext()
	defer cancel()
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	// Make first decision to schedule activity
	_, err = poller.PollAndProcessDecisionTaskWithAttempt(false, false, false, true, int64(0))
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	type QueryResult struct {
		Resp *types.QueryWorkflowResponse
		Err  error
	}
	queryResultCh := make(chan QueryResult)
	queryWorkflowFn := func(queryType string, rejectCondition *types.QueryRejectCondition) {
		s.False(handledSignal.Load().(bool))
		ctx, cancel := createContext()
		defer cancel()
		queryResp, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			Query: &types.WorkflowQuery{
				QueryType: queryType,
			},
			QueryRejectCondition:  rejectCondition,
			QueryConsistencyLevel: types.QueryConsistencyLevelStrong.Ptr(),
		})
		s.True(handledSignal.Load().(bool))
		queryResultCh <- QueryResult{Resp: queryResp, Err: err}
	}

	// send a signal that will take 5 seconds to handle
	// this causes the signal to still be outstanding at the time query arrives
	signalName := "my signal"
	signalInput := []byte("my signal input.")
	ctx, cancel = createContext()
	defer cancel()
	err = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain: s.domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		SignalName: signalName,
		Input:      signalInput,
		Identity:   identity,
	})
	s.Nil(err)

	go func() {
		// wait for decision task for signal to get started before querying workflow
		// since signal processing takes 5 seconds and we wait only one second to issue the query
		// at the time the query comes in there will be a started decision task
		// only once signal completes can queryWorkflow unblock
		<-time.After(time.Second)

		// at this point there is a decision task started on the worker so this second signal will become buffered
		signalName := "my signal"
		signalInput := []byte("my signal input.")
		ctx, cancel := createContext()
		defer cancel()
		err = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
			Domain: s.domainName,
			WorkflowExecution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we.RunID,
			},
			SignalName: signalName,
			Input:      signalInput,
			Identity:   identity,
		})
		s.Nil(err)

		queryWorkflowFn(queryType, nil)
	}()

	_, err = poller.PollAndProcessDecisionTaskWithSticky(false, false)
	s.NoError(err)
	<-time.After(time.Second)

	// query should not have been dispatched on the decision task which contains signal
	// because signal was already outstanding by the time query arrived
	select {
	case <-queryResultCh:
		s.Fail("query should not be ready yet")
	default:
	}

	// now poll for decision task which contains the query which was buffered
	isQueryTask, _, errInner := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		true,
		true,
		int64(0),
		5,
		false,
		&types.WorkflowQueryResult{
			ResultType: types.QueryResultTypeAnswered.Ptr(),
			Answer:     []byte("consistent query result"),
		})

	// the task should not be a query task because at the time outstanding decision task completed
	// there existed a buffered event which triggered the creation of a new decision task which query was dispatched on
	s.False(isQueryTask)
	s.NoError(errInner)

	queryResult := <-queryResultCh
	s.NoError(queryResult.Err)
	s.NotNil(queryResult.Resp)
	s.NotNil(queryResult.Resp.QueryResult)
	s.Nil(queryResult.Resp.QueryRejected)
	queryResultString := string(queryResult.Resp.QueryResult)
	s.Equal("consistent query result", queryResultString)
}

func (s *IntegrationSuite) TestQueryWorkflow_BeforeFirstDecision() {
	id := "integration-test-query-workflow-before-first-decision"
	wt := "integration-test-query-workflow-before-first-decision-type"
	tl := "integration-test-query-workflow-before-first-decision-tasklist"
	identity := "worker1"
	queryType := "test-query"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
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
	we, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	workflowExecution := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.RunID,
	}

	ctx, cancel = createContext()
	defer cancel()
	// query workflow without any decision task should produce an error
	_, err := s.engine.QueryWorkflow(ctx, &types.QueryWorkflowRequest{
		Domain:    s.domainName,
		Execution: workflowExecution,
		Query: &types.WorkflowQuery{
			QueryType: queryType,
		},
	})
	s.IsType(&types.QueryFailedError{}, err)
	s.Equal("workflow must handle at least one decision task before it can be queried", err.(*types.QueryFailedError).Message)
}
