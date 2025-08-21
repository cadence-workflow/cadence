// Copyright (c) 2017-2021 Uber Technologies Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestResetWorkflow() {
	id := "integration-reset-workflow-test"
	wt := "integration-reset-workflow-test-type"
	tl := "integration-reset-workflow-test-taskqueue"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}

	tasklist := &types.TaskList{Name: tl}

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            tasklist,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.NoError(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.GetRunID()))

	// workflow logic
	workflowComplete := false
	activityData := int32(1)
	activityCount := 3
	isFirstTaskProcessed := false
	isSecondTaskProcessed := false
	var firstActivityCompletionEvent *types.HistoryEvent
	wtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !isFirstTaskProcessed {
			// Schedule 3 activities on first workflow task
			isFirstTaskProcessed = true
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityData))

			var scheduleActivityCommands []*types.Decision
			for i := 1; i <= activityCount; i++ {
				scheduleActivityCommands = append(scheduleActivityCommands, &types.Decision{
					DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
					ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
						ActivityID:                    strconv.Itoa(i),
						ActivityType:                  &types.ActivityType{Name: "ResetActivity"},
						TaskList:                      tasklist,
						Input:                         buf.Bytes(),
						ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
						ScheduleToStartTimeoutSeconds: common.Int32Ptr(100),
						StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
						HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
					},
				})
			}

			return nil, scheduleActivityCommands, nil
		} else if !isSecondTaskProcessed {
			// Confirm one activity completion on second workflow task
			isSecondTaskProcessed = true
			for _, event := range history.Events[previousStartedEventID:] {
				if event.GetEventType() == types.EventTypeActivityTaskCompleted {
					firstActivityCompletionEvent = event
					return nil, []*types.Decision{}, nil
				}
			}
		}

		// Complete workflow after reset
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
		TaskList:        tasklist,
		Identity:        identity,
		DecisionHandler: wtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Process first workflow decision task to schedule activities
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessWorkflowTask", tag.Error(err))
	s.NoError(err)

	// Process one activity task which also creates second workflow task
	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("Poll and process first activity", tag.Error(err))
	s.NoError(err)

	// Process second workflow task which checks activity completion
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("Poll and process second workflow task", tag.Error(err))
	s.NoError(err)

	// Find reset point (last completed decision task)
	events := s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.GetRunID(),
	})
	var lastDecisionCompleted *types.HistoryEvent
	for _, event := range events {
		if event.GetEventType() == types.EventTypeDecisionTaskCompleted {
			lastDecisionCompleted = event
		}
	}

	ctx, cancel = createContext()
	defer cancel()
	// FIRST reset: Reset workflow execution, current is open
	resp, err := s.Engine.ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		Reason:                "reset execution from test",
		DecisionFinishEventID: lastDecisionCompleted.ID,
		RequestID:             uuid.New(),
	})
	s.NoError(err)

	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("Poll and process second activity", tag.Error(err))
	s.NoError(err)

	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("Poll and process third activity", tag.Error(err))
	s.NoError(err)

	s.NotNil(firstActivityCompletionEvent)
	s.False(workflowComplete)

	// get the history of the first run again
	events = s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.GetRunID(),
	})
	var firstRunStartTimestamp int64
	var lastEvent *types.HistoryEvent
	for _, event := range events {
		if event.GetEventType() == types.EventTypeWorkflowExecutionStarted {
			firstRunStartTimestamp = event.GetTimestamp()
		}
		if event.GetEventType() == types.EventTypeDecisionTaskCompleted {
			lastDecisionCompleted = event
		}
		lastEvent = event
	}
	// assert the first run is closed, terminated by the previous reset
	s.Equal(types.EventTypeWorkflowExecutionTerminated, lastEvent.GetEventType())

	// check the start time of mutable state for the second run,
	// it should be reset time although the start event is reused
	ctx, cancel = createContext()
	defer cancel()
	descResp, err := s.Engine.DescribeWorkflowExecution(ctx, &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      resp.GetRunID(),
		},
	})
	s.NoError(err)
	s.True(descResp.WorkflowExecutionInfo.GetStartTime() > firstRunStartTimestamp)

	ctx, cancel = createContext()
	defer cancel()
	// SECOND reset: reset the first run again, to exercise the code path of resetting closed workflow
	resp, err = s.Engine.ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.GetRunID(),
		},
		Reason:                "reset execution from test",
		DecisionFinishEventID: lastDecisionCompleted.ID,
		RequestID:             uuid.New(),
	})
	s.NoError(err)
	newRunID := resp.GetRunID()

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("Poll and process final decision task", tag.Error(err))
	s.NoError(err)
	s.True(workflowComplete)

	// get the history of the newRunID
	events = s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      newRunID,
	})
	for _, event := range events {
		if event.GetEventType() == types.EventTypeDecisionTaskCompleted {
			lastDecisionCompleted = event
		}
		lastEvent = event
	}
	// assert the new run is closed, completed by decision task
	s.Equal(types.EventTypeWorkflowExecutionCompleted, lastEvent.GetEventType())

	ctx, cancel = createContext()
	defer cancel()
	// THIRD reset: reset the workflow run that is after a reset
	_, err = s.Engine.ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      newRunID,
		},
		Reason:                "reset execution from test",
		DecisionFinishEventID: lastDecisionCompleted.ID,
		RequestID:             uuid.New(),
	})
	s.NoError(err)
}

func (s *IntegrationSuite) TestResetWorkflow_NoDecisionTaskCompleted() {
	id := "integration-reset-workflow-test-no-decision-completed"
	wt := "integration-reset-workflow-test-type--no-decision-completed"
	tl := "integration-reset-workflow-test-taskqueue-no-decision-completed"
	identity := "worker1"

	workflowType := &types.WorkflowType{Name: wt}

	tasklist := &types.TaskList{Name: tl}

	// Start workflow execution
	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            tasklist,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(2),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.NoError(err0)

	workflowComplete := false
	activityData := int32(1)
	isFirstTaskProcessed := false

	wtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		if !isFirstTaskProcessed {
			// Schedule 3 activities on first workflow task
			isFirstTaskProcessed = true
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, activityData))

			var scheduleActivityCommands []*types.Decision
			scheduleActivityCommands = append(scheduleActivityCommands, &types.Decision{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    "1",
					ActivityType:                  &types.ActivityType{Name: "ResetActivity"},
					TaskList:                      tasklist,
					Input:                         buf.Bytes(),
					ScheduleToCloseTimeoutSeconds: common.Int32Ptr(100),
					ScheduleToStartTimeoutSeconds: common.Int32Ptr(100),
					StartToCloseTimeoutSeconds:    common.Int32Ptr(50),
					HeartbeatTimeoutSeconds:       common.Int32Ptr(5),
				},
			})

			return nil, scheduleActivityCommands, nil
		}
		// Complete workflow after reset
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
		activityID string, input []byte, taskToken []byte) ([]byte, bool, error) {
		return []byte("Activity Result."), false, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        tasklist,
		Identity:        identity,
		DecisionHandler: wtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Process first workflow decision task to schedule activities
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessWorkflowTask", tag.Error(err))
	s.NoError(err)

	// Process one activity task which also creates second workflow task
	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("Poll and process first activity", tag.Error(err))
	s.NoError(err)

	// Find reset point (last completed decision task)
	events := s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.GetRunID(),
	})
	var lastDecisionScheduled *types.HistoryEvent
	for _, event := range events {
		if event.GetEventType() == types.EventTypeDecisionTaskScheduled {
			lastDecisionScheduled = event
		}
	}
	s.NotNil(lastDecisionScheduled)

	describeDomainRequest := &types.DescribeDomainRequest{
		Name: &s.DomainName,
	}

	ctx, cancel = createContext()
	defer cancel()

	describeDomainResponse, err := s.Engine.DescribeDomain(ctx, describeDomainRequest)
	s.NoError(err)

	recordDecisionStartedRequest := &types.RecordDecisionTaskStartedRequest{
		DomainUUID: describeDomainResponse.DomainInfo.UUID,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.GetRunID(),
		},
		ScheduleID: lastDecisionScheduled.ID,
		TaskID:     1,
		RequestID:  uuid.New(),
		PollRequest: &types.PollForDecisionTaskRequest{
			Domain:         s.DomainName,
			TaskList:       tasklist,
			Identity:       identity,
			BinaryChecksum: "",
		},
	}

	ctx, cancel = createContext()
	defer cancel()

	// Record decision task started, to simulate the case where decision task is started but not completed
	_, err = s.HistoryClient.RecordDecisionTaskStarted(ctx, recordDecisionStartedRequest)
	s.NoError(err)

	ctx, cancel = createContext()
	defer cancel()
	// FIRST reset: Reset workflow execution, current is open
	_, err = s.Engine.ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
		Reason:                "reset execution from test",
		DecisionFinishEventID: lastDecisionScheduled.ID + 1,
		RequestID:             uuid.New(),
	})
	s.NoError(err)

	// get the history of the first run again
	events = s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      we.GetRunID(),
	})
	var lastEvent *types.HistoryEvent
	for _, event := range events {
		lastEvent = event
	}
	// assert the first run is closed, terminated by the previous reset
	s.Equal(types.EventTypeWorkflowExecutionTerminated, lastEvent.GetEventType())

	ctx, cancel = createContext()
	defer cancel()
	// SECOND reset: reset the first run again, to exercise the code path of resetting closed workflow
	resp, err := s.Engine.ResetWorkflowExecution(ctx, &types.ResetWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.GetRunID(),
		},
		Reason:                "reset execution from test",
		DecisionFinishEventID: lastDecisionScheduled.ID + 2,
		RequestID:             uuid.New(),
	})
	s.NoError(err)

	newRunID := resp.GetRunID()

	workflowComplete = false
	isFirstTaskProcessed = false

	// Process first workflow decision task to schedule activities
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessWorkflowTask", tag.Error(err))
	s.NoError(err)

	s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      newRunID,
	})

	// Process one activity task which also creates second workflow task
	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("Poll and process first activity", tag.Error(err))
	s.NoError(err)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("Poll and process final decision task", tag.Error(err))
	s.NoError(err)
	s.True(workflowComplete)

	// get the history of the newRunID
	events = s.getHistory(s.DomainName, &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      newRunID,
	})
	for _, event := range events {
		lastEvent = event
	}
	// assert the new run is closed, completed by decision task
	s.Equal(types.EventTypeWorkflowExecutionCompleted, lastEvent.GetEventType())
}
