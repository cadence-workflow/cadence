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

func (s *IntegrationSuite) TestExternalRequestCancelWorkflowExecution() {
	id := "integration-request-cancel-workflow-test"
	wt := "integration-request-cancel-workflow-test-type"
	tl := "integration-request-cancel-workflow-test-tasklist"
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
			DecisionType: types.DecisionTypeCancelWorkflowExecution.Ptr(),
			CancelWorkflowExecutionDecisionAttributes: &types.CancelWorkflowExecutionDecisionAttributes{
				Details: []byte("Cancelled"),
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

	err = poller.PollAndProcessActivityTask(false)
	s.Logger.Info("PollAndProcessActivityTask", tag.Error(err))
	s.Nil(err)

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.RequestCancelWorkflowExecution(ctx, &types.RequestCancelWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.Nil(err)

	ctx, cancel = createContext()
	defer cancel()
	err = s.Engine.RequestCancelWorkflowExecution(ctx, &types.RequestCancelWorkflowExecutionRequest{
		Domain: s.DomainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.NotNil(err)
	s.IsType(&types.CancellationAlreadyRequestedError{}, err)

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	executionCancelled := false
GetHistoryLoop:
	for i := 1; i < 3; i++ {
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
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionCanceled {
			s.Logger.Warn("Execution not cancelled yet.")
			time.Sleep(100 * time.Millisecond)
			continue GetHistoryLoop
		}

		cancelledEventAttributes := lastEvent.WorkflowExecutionCanceledEventAttributes
		s.Equal("Cancelled", string(cancelledEventAttributes.Details))
		executionCancelled = true
		break GetHistoryLoop
	}
	s.True(executionCancelled)
}

func (s *IntegrationSuite) TestRequestCancelWorkflowDecisionExecution() {
	id := "integration-cancel-workflow-decision-test"
	wt := "integration-cancel-workflow-decision-test-type"
	tl := "integration-cancel-workflow-decision-test-tasklist"
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

	foreignRequest := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.ForeignDomainName,
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
	we2, err0 := s.Engine.StartWorkflowExecution(ctx, foreignRequest)
	s.Nil(err0)
	s.Logger.Info("StartWorkflowExecution on foreign Domain: %v,  response: %v \n", tag.WorkflowDomainName(s.ForeignDomainName), tag.WorkflowRunID(we2.RunID))

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
			DecisionType: types.DecisionTypeRequestCancelExternalWorkflowExecution.Ptr(),
			RequestCancelExternalWorkflowExecutionDecisionAttributes: &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
				Domain:     s.ForeignDomainName,
				WorkflowID: id,
				RunID:      we2.RunID,
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

	foreignActivityCount := int32(1)
	foreignActivityCounter := int32(0)
	foreignDtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {
		if foreignActivityCounter < foreignActivityCount {
			foreignActivityCounter++
			buf := new(bytes.Buffer)
			s.Nil(binary.Write(buf, binary.LittleEndian, foreignActivityCounter))

			return []byte(strconv.Itoa(int(foreignActivityCounter))), []*types.Decision{{
				DecisionType: types.DecisionTypeScheduleActivityTask.Ptr(),
				ScheduleActivityTaskDecisionAttributes: &types.ScheduleActivityTaskDecisionAttributes{
					ActivityID:                    strconv.Itoa(int(foreignActivityCounter)),
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

		return []byte(strconv.Itoa(int(foreignActivityCounter))), []*types.Decision{{
			DecisionType: types.DecisionTypeCancelWorkflowExecution.Ptr(),
			CancelWorkflowExecutionDecisionAttributes: &types.CancelWorkflowExecutionDecisionAttributes{
				Details: []byte("Cancelled"),
			},
		}}, nil
	}

	foreignPoller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.ForeignDomainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: foreignDtHandler,
		ActivityHandler: atHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// Start both current and foreign workflows to make some progress.
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	_, err = foreignPoller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("foreign PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	err = foreignPoller.PollAndProcessActivityTask(false)
	s.Logger.Info("foreign PollAndProcessActivityTask", tag.Error(err))
	s.Nil(err)

	// Cancel the foreign workflow with this decision request.
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	cancellationSent := false
	intiatedEventID := 10
CheckHistoryLoopForCancelSent:
	for i := 1; i < 10; i++ {
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

		lastEvent := history.Events[len(history.Events)-2]
		if *lastEvent.EventType != types.EventTypeExternalWorkflowExecutionCancelRequested {
			s.Logger.Info("Cancellation still not sent.")
			time.Sleep(100 * time.Millisecond)
			continue CheckHistoryLoopForCancelSent
		}

		externalWorkflowExecutionCancelRequestedEvent := lastEvent.ExternalWorkflowExecutionCancelRequestedEventAttributes
		s.Equal(int64(intiatedEventID), externalWorkflowExecutionCancelRequestedEvent.InitiatedEventID)
		s.Equal(id, externalWorkflowExecutionCancelRequestedEvent.WorkflowExecution.WorkflowID)
		s.Equal(we2.RunID, externalWorkflowExecutionCancelRequestedEvent.WorkflowExecution.RunID)

		cancellationSent = true
		break
	}

	s.True(cancellationSent)

	// Accept cancellation.
	_, err = foreignPoller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("foreign PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	executionCancelled := false
GetHistoryLoop:
	for i := 1; i < 10; i++ {
		ctx, cancel := createContext()
		historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
			Domain: s.ForeignDomainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: id,
				RunID:      we2.RunID,
			},
		})
		cancel()
		s.Nil(err)
		history := historyResponse.History

		lastEvent := history.Events[len(history.Events)-1]
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionCanceled {
			s.Logger.Warn("Execution not cancelled yet.")
			time.Sleep(100 * time.Millisecond)
			continue GetHistoryLoop
		}

		cancelledEventAttributes := lastEvent.WorkflowExecutionCanceledEventAttributes
		s.Equal("Cancelled", string(cancelledEventAttributes.Details))
		executionCancelled = true

		// Find cancel requested event and verify it.
		var cancelRequestEvent *types.HistoryEvent
		for _, x := range history.Events {
			if *x.EventType == types.EventTypeWorkflowExecutionCancelRequested {
				cancelRequestEvent = x
			}
		}

		s.NotNil(cancelRequestEvent)
		cancelRequestEventAttributes := cancelRequestEvent.WorkflowExecutionCancelRequestedEventAttributes
		s.Equal(int64(intiatedEventID), *cancelRequestEventAttributes.ExternalInitiatedEventID)
		s.Equal(id, cancelRequestEventAttributes.ExternalWorkflowExecution.WorkflowID)
		s.Equal(we.RunID, cancelRequestEventAttributes.ExternalWorkflowExecution.RunID)

		break GetHistoryLoop
	}
	s.True(executionCancelled)
}

func (s *IntegrationSuite) TestRequestCancelWorkflowDecisionExecution_UnKnownTarget() {
	id := "integration-cancel-unknown-workflow-decision-test"
	wt := "integration-cancel-unknown-workflow-decision-test-type"
	tl := "integration-cancel-unknown-workflow-decision-test-tasklist"
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
			DecisionType: types.DecisionTypeRequestCancelExternalWorkflowExecution.Ptr(),
			RequestCancelExternalWorkflowExecutionDecisionAttributes: &types.RequestCancelExternalWorkflowExecutionDecisionAttributes{
				Domain:     s.ForeignDomainName,
				WorkflowID: "workflow_not_exist",
				RunID:      we.RunID,
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

	// Start workflows to make some progress.
	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	// Cancel the foreign workflow with this decision request.
	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.Logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	cancellationSentFailed := false
	intiatedEventID := 10
CheckHistoryLoopForCancelSent:
	for i := 1; i < 10; i++ {
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

		lastEvent := history.Events[len(history.Events)-2]
		if *lastEvent.EventType != types.EventTypeRequestCancelExternalWorkflowExecutionFailed {
			s.Logger.Info("Cancellaton not cancelled yet.")
			time.Sleep(100 * time.Millisecond)
			continue CheckHistoryLoopForCancelSent
		}

		requestCancelExternalWorkflowExecutionFailedEvetn := lastEvent.RequestCancelExternalWorkflowExecutionFailedEventAttributes
		s.Equal(int64(intiatedEventID), requestCancelExternalWorkflowExecutionFailedEvetn.InitiatedEventID)
		s.Equal("workflow_not_exist", requestCancelExternalWorkflowExecutionFailedEvetn.WorkflowExecution.WorkflowID)
		s.Equal(we.RunID, requestCancelExternalWorkflowExecutionFailedEvetn.WorkflowExecution.RunID)

		cancellationSentFailed = true
		break
	}

	s.True(cancellationSentFailed)
}
