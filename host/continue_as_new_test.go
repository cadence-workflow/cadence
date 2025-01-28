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
	"strconv"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestContinueAsNewWorkflow() {
	id := "integration-continue-as-new-workflow-test"
	wt := "integration-continue-as-new-workflow-test-type"
	tl := "integration-continue-as-new-workflow-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	header := &types.Header{
		Fields: map[string][]byte{"tracing": []byte("sample payload")},
	}
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
		Header:                              header,
		Memo:                                memo,
		SearchAttributes:                    searchAttr,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	continueAsNewCount := int32(10)
	continueAsNewCounter := int32(0)
	var previousRunID string
	var lastRunStartedEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if continueAsNewCounter < continueAsNewCount {
			previousRunID = execution.GetRunID()
			continueAsNewCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, continueAsNewCounter))

			return []byte(strconv.Itoa(int(continueAsNewCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
				ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					WorkflowType:                        workflowType,
					TaskList:                            &types.TaskList{Name: tl},
					Input:                               buf.Bytes(),
					Header:                              header,
					Memo:                                memo,
					SearchAttributes:                    searchAttr,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
				},
			}}, nil
		}

		lastRunStartedEvent = history.Events[0]
		workflowComplete = true
		return []byte(strconv.Itoa(int(continueAsNewCounter))), []*types.Decision{{
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

	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		descResp, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
			Domain: s.DomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
			},
		})
		cancel()
		s.NoError(err)
		s.NotZero(descResp.WorkflowExecutionInfo.GetStartTime())

		_, err = poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err, strconv.Itoa(i))

	}

	s.False(workflowComplete)
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	s.True(workflowComplete)
	s.Equal(previousRunID, lastRunStartedEvent.WorkflowExecutionStartedEventAttributes.GetContinuedExecutionRunID())
	s.Equal(header, lastRunStartedEvent.WorkflowExecutionStartedEventAttributes.Header)
	s.Equal(memo, lastRunStartedEvent.WorkflowExecutionStartedEventAttributes.Memo)
	s.Equal(searchAttr, lastRunStartedEvent.WorkflowExecutionStartedEventAttributes.SearchAttributes)
}

func (s *IntegrationSuite) TestContinueAsNewWorkflow_Timeout() {
	id := "integration-continue-as-new-workflow-timeout-test"
	wt := "integration-continue-as-new-workflow-timeout-test-type"
	tl := "integration-continue-as-new-workflow-timeout-test-tasklist"
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
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	workflowComplete := false
	continueAsNewCount := int32(1)
	continueAsNewCounter := int32(0)
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if continueAsNewCounter < continueAsNewCount {
			continueAsNewCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, continueAsNewCounter))

			return []byte(strconv.Itoa(int(continueAsNewCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
				ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					WorkflowType:                        workflowType,
					TaskList:                            &types.TaskList{Name: tl},
					Input:                               buf.Bytes(),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(1), // set timeout to 1
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				},
			}}, nil
		}

		workflowComplete = true
		return []byte(strconv.Itoa(int(continueAsNewCounter))), []*types.Decision{{
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

	// process the decision and continue as new
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	s.False(workflowComplete)

	time.Sleep(1 * time.Second) // wait 1 second for timeout

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
}

func (s *IntegrationSuite) TestWorkflowContinueAsNew_TaskID() {
	id := "integration-wf-continue-as-new-task-id-test"
	wt := "integration-wf-continue-as-new-task-id-type"
	tl := "integration-wf-continue-as-new-task-id-tasklist"
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

	var executions []*types.WorkflowExecution

	continueAsNewed := false
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		executions = append(executions, execution)

		if !continueAsNewed {
			continueAsNewed = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
				ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
					WorkflowType:                        workflowType,
					TaskList:                            taskList,
					Input:                               nil,
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
				},
			}}, nil
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("succeed"),
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

	minTaskID := int64(0)
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	events := s.getHistory(s.DomainName, executions[0])
	s.True(len(events) != 0)
	for _, event := range events {
		s.True(event.TaskID > minTaskID)
		minTaskID = event.TaskID
	}

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	events = s.getHistory(s.DomainName, executions[1])
	s.True(len(events) != 0)
	for _, event := range events {
		s.True(event.TaskID > minTaskID)
		minTaskID = event.TaskID
	}
}

func (s *IntegrationSuite) TestChildWorkflowWithContinueAsNew() {
	parentID := "integration-child-workflow-with-continue-as-new-test-parent"
	childID := "integration-child-workflow-with-continue-as-new-test-child"
	wtParent := "integration-child-workflow-with-continue-as-new-test-parent-type"
	wtChild := "integration-child-workflow-with-continue-as-new-test-child-type"
	tl := "integration-child-workflow-with-continue-as-new-test-tasklist"
	identity := "worker1"

	parentWorkflowType := &types.WorkflowType{}
	parentWorkflowType.Name = wtParent

	childWorkflowType := &types.WorkflowType{}
	childWorkflowType.Name = wtChild

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          parentID,
		WorkflowType:                        parentWorkflowType,
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
	childComplete := false
	childExecutionStarted := false
	childData := int32(1)
	continueAsNewCount := int32(10)
	continueAsNewCounter := int32(0)
	var startedEvent *types.HistoryEvent
	var completedEvent *types.HistoryEvent
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		s.Logger.Info("Processing decision task for WorkflowID:", tag.WorkflowID(execution.WorkflowID))

		// Child Decider Logic
		if execution.WorkflowID == childID {
			if continueAsNewCounter < continueAsNewCount {
				continueAsNewCounter++
				buf := new(bytes.Buffer)
				s.Nil(binary.Write(buf, binary.LittleEndian, continueAsNewCounter))

				return []byte(strconv.Itoa(int(continueAsNewCounter))), []*types.Decision{{
					DecisionType: types.DecisionTypeContinueAsNewWorkflowExecution.Ptr(),
					ContinueAsNewWorkflowExecutionDecisionAttributes: &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
						Input: buf.Bytes(),
					},
				}}, nil
			}

			childComplete = true
			return nil, []*types.Decision{{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("Child Done."),
				},
			}}, nil
		}

		// Parent Decider Logic
		if execution.WorkflowID == parentID {
			if !childExecutionStarted {
				s.Logger.Info("Starting child execution.")
				childExecutionStarted = true
				buf := new(bytes.Buffer)
				s.Nil(binary.Write(buf, binary.LittleEndian, childData))

				return nil, []*types.Decision{{
					DecisionType: types.DecisionTypeStartChildWorkflowExecution.Ptr(),
					StartChildWorkflowExecutionDecisionAttributes: &types.StartChildWorkflowExecutionDecisionAttributes{
						Domain:       s.DomainName,
						WorkflowID:   childID,
						WorkflowType: childWorkflowType,
						Input:        buf.Bytes(),
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

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Make first decision to start child execution
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.True(childExecutionStarted)

	// Process ChildExecution Started event and all generations of child executions
	for i := 0; i < 11; i++ {
		s.Logger.Warn("decision: %v", tag.Counter(i))
		_, err = poller.PollAndProcessDecisionTask(false, false)
		s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
		s.Nil(err)
	}

	s.False(childComplete)
	s.NotNil(startedEvent)

	// Process Child Execution final decision to complete it
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.True(childComplete)

	// Process ChildExecution completed event and complete parent execution
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)
	s.NotNil(completedEvent)
	completedAttributes := completedEvent.ChildWorkflowExecutionCompletedEventAttributes
	s.Equal(s.DomainName, completedAttributes.Domain)
	s.Equal(childID, completedAttributes.WorkflowExecution.WorkflowID)
	s.NotEqual(startedEvent.ChildWorkflowExecutionStartedEventAttributes.WorkflowExecution.RunID,
		completedAttributes.WorkflowExecution.RunID)
	s.Equal(wtChild, completedAttributes.WorkflowType.Name)
	s.Equal([]byte("Child Done."), completedAttributes.Result)

	s.Logger.Info("Parent Workflow Execution History: ")
}
