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
	"encoding/json"
	"strconv"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func (s *IntegrationSuite) TestDecisionHeartbeatingWithEmptyResult() {
	id := uuid.New()
	wt := "integration-workflow-decision-heartbeating-local-activities"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{
		Name: tl,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	stickyTaskList := &types.TaskList{
		Name: "test-sticky-tasklist",
		Kind: types.TaskListKindSticky.Ptr(),
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(20),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(3),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	// start decision
	ctx, cancel = createContext()
	defer cancel()
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.Equal(int64(0), resp1.GetAttempt())
	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	taskToken := resp1.GetTaskToken()
	hbTimeout := 0
	for i := 0; i < 12; i++ {
		ctx, cancel := createContext()
		resp2, err2 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
			TaskToken: taskToken,
			Decisions: []*types.Decision{},
			StickyAttributes: &types.StickyExecutionAttributes{
				WorkerTaskList:                stickyTaskList,
				ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
			},
			ReturnNewDecisionTask:      true,
			ForceCreateNewDecisionTask: true,
		})
		cancel()
		if _, ok := err2.(*types.EntityNotExistsError); ok {
			hbTimeout++

			ctx, cancel := createContext()
			resp, err := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
				Domain:   s.domainName,
				TaskList: taskList,
				Identity: identity,
			})
			cancel()
			s.Nil(err)
			taskToken = resp.GetTaskToken()
		} else {
			s.Nil(err2)
			taskToken = resp2.DecisionTask.GetTaskToken()
		}
		time.Sleep(time.Second)
	}

	s.Equal(2, hbTimeout)

	ctx, cancel = createContext()
	defer cancel()
	resp5, err5 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken: taskToken,
		Decisions: []*types.Decision{
			{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("efg"),
				},
			},
		},
		StickyAttributes: &types.StickyExecutionAttributes{
			WorkerTaskList:                stickyTaskList,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
		},
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: false,
	})
	s.Nil(err5)
	s.Nil(resp5.DecisionTask)

	s.assertLastHistoryEvent(we, 41, types.EventTypeWorkflowExecutionCompleted)
}

func (s *IntegrationSuite) TestDecisionHeartbeatingWithLocalActivitiesResult() {
	id := uuid.New()
	wt := "integration-workflow-decision-heartbeating-local-activities"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{
		Name: tl,
		Kind: types.TaskListKindNormal.Ptr(),
	}
	stikyTaskList := &types.TaskList{
		Name: "test-sticky-tasklist",
		Kind: types.TaskListKindSticky.Ptr(),
	}

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(20),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(5),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	// start decision
	ctx, cancel = createContext()
	defer cancel()
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.Equal(int64(0), resp1.GetAttempt())
	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	resp2, err2 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken: resp1.GetTaskToken(),
		Decisions: []*types.Decision{},
		StickyAttributes: &types.StickyExecutionAttributes{
			WorkerTaskList:                stikyTaskList,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
		},
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: true,
	})
	s.Nil(err2)

	ctx, cancel = createContext()
	defer cancel()
	resp3, err3 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken: resp2.DecisionTask.GetTaskToken(),
		Decisions: []*types.Decision{
			{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "localActivity1",
					Details:    []byte("abc"),
				},
			},
		},
		StickyAttributes: &types.StickyExecutionAttributes{
			WorkerTaskList:                stikyTaskList,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
		},
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: true,
	})
	s.Nil(err3)

	ctx, cancel = createContext()
	defer cancel()
	resp4, err4 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken: resp3.DecisionTask.GetTaskToken(),
		Decisions: []*types.Decision{
			{
				DecisionType: types.DecisionTypeRecordMarker.Ptr(),
				RecordMarkerDecisionAttributes: &types.RecordMarkerDecisionAttributes{
					MarkerName: "localActivity2",
					Details:    []byte("abc"),
				},
			},
		},
		StickyAttributes: &types.StickyExecutionAttributes{
			WorkerTaskList:                stikyTaskList,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
		},
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: true,
	})
	s.Nil(err4)

	ctx, cancel = createContext()
	defer cancel()
	resp5, err5 := s.engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken: resp4.DecisionTask.GetTaskToken(),
		Decisions: []*types.Decision{
			{
				DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
				CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
					Result: []byte("efg"),
				},
			},
		},
		StickyAttributes: &types.StickyExecutionAttributes{
			WorkerTaskList:                stikyTaskList,
			ScheduleToStartTimeoutSeconds: common.Int32Ptr(5),
		},
		ReturnNewDecisionTask:      true,
		ForceCreateNewDecisionTask: false,
	})
	s.Nil(err5)
	s.Nil(resp5.DecisionTask)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskCompleted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskCompleted,
		types.EventTypeMarkerRecorded,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskCompleted,
		types.EventTypeMarkerRecorded,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskCompleted,
		types.EventTypeWorkflowExecutionCompleted,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalBeforeRegularDecisionStarted() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	ctx, cancel = createContext()
	defer cancel()
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 3, types.EventTypeWorkflowExecutionSignaled)

	ctx, cancel = createContext()
	defer cancel()
	// start this transient decision, the attempt should be cleared and it becomes again a regular decision
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.Equal(int64(0), resp1.GetAttempt())
	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalAfterRegularDecisionStarted() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	ctx, cancel = createContext()
	defer cancel()
	// start decision to make signals into bufferedEvents
	_, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// this signal should be buffered
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalAfterRegularDecisionStartedAndFailDecision() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	cause := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure

	ctx, cancel = createContext()
	defer cancel()
	// start decision to make signals into bufferedEvents
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// this signal should be buffered
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 3, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// fail this decision to flush buffer, and then another decision will be scheduled
	err2 := s.engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
		TaskToken: resp1.GetTaskToken(),
		Cause:     &cause,
		Identity:  "integ test",
	})
	s.Nil(err2)
	s.assertLastHistoryEvent(we, 6, types.EventTypeDecisionTaskScheduled)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalBeforeTransientDecisionStarted() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	cause := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
			Domain:   s.domainName,
			TaskList: taskList,
			Identity: identity,
		})
		cancel()
		s.Nil(err1)
		s.Equal(int64(i), resp1.GetAttempt())
		if i == 0 {
			// first time is regular decision
			s.Equal(int64(3), resp1.GetStartedEventID())
		} else {
			// the rest is transient decision
			s.Equal(int64(6), resp1.GetStartedEventID())
		}

		ctx, cancel = createContext()
		err2 := s.engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
			TaskToken: resp1.GetTaskToken(),
			Cause:     &cause,
			Identity:  "integ test",
		})
		cancel()
		s.Nil(err2)
	}

	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 5, types.EventTypeWorkflowExecutionSignaled)

	ctx, cancel = createContext()
	defer cancel()
	// start this transient decision, the attempt should be cleared and it becomes again a regular decision
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.Equal(int64(0), resp1.GetAttempt())
	s.assertLastHistoryEvent(we, 7, types.EventTypeDecisionTaskStarted)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalAfterTransientDecisionStarted() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	cause := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
			Domain:   s.domainName,
			TaskList: taskList,
			Identity: identity,
		})
		cancel()
		s.Nil(err1)
		s.Equal(int64(i), resp1.GetAttempt())
		if i == 0 {
			// first time is regular decision
			s.Equal(int64(3), resp1.GetStartedEventID())
		} else {
			// the rest is transient decision
			s.Equal(int64(6), resp1.GetStartedEventID())
		}

		ctx, cancel = createContext()
		err2 := s.engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
			TaskToken: resp1.GetTaskToken(),
			Cause:     &cause,
			Identity:  "integ test",
		})
		cancel()
		s.Nil(err2)
	}

	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// start decision to make signals into bufferedEvents
	_, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// this signal should be buffered
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) TestWorkflowTerminationSignalAfterTransientDecisionStartedAndFailDecision() {
	id := uuid.New()
	wt := "integration-workflow-transient-decision-test-type"
	tl := id
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
		Identity:                            identity,
	}

	ctx, cancel := createContext()
	defer cancel()
	resp0, err0 := s.engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	we := &types.WorkflowExecution{
		WorkflowID: id,
		RunID:      resp0.RunID,
	}

	s.assertLastHistoryEvent(we, 2, types.EventTypeDecisionTaskScheduled)

	cause := types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure
	for i := 0; i < 10; i++ {
		ctx, cancel := createContext()
		resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
			Domain:   s.domainName,
			TaskList: taskList,
			Identity: identity,
		})
		cancel()
		s.Nil(err1)
		s.Equal(int64(i), resp1.GetAttempt())
		if i == 0 {
			// first time is regular decision
			s.Equal(int64(3), resp1.GetStartedEventID())
		} else {
			// the rest is transient decision
			s.Equal(int64(6), resp1.GetStartedEventID())
		}

		ctx, cancel = createContext()
		err2 := s.engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
			TaskToken: resp1.GetTaskToken(),
			Cause:     &cause,
			Identity:  "integ test",
		})
		cancel()
		s.Nil(err2)
	}

	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// start decision to make signals into bufferedEvents
	resp1, err1 := s.engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
		Domain:   s.domainName,
		TaskList: taskList,
		Identity: identity,
	})
	s.Nil(err1)

	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// this signal should be buffered
	err0 = s.engine.SignalWorkflowExecution(ctx, &types.SignalWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		SignalName:        "sig-for-integ-test",
		Input:             []byte(""),
		Identity:          "integ test",
		RequestID:         uuid.New(),
	})
	s.Nil(err0)
	s.assertLastHistoryEvent(we, 4, types.EventTypeDecisionTaskFailed)

	ctx, cancel = createContext()
	defer cancel()
	// fail this decision to flush buffer
	err2 := s.engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
		TaskToken: resp1.GetTaskToken(),
		Cause:     &cause,
		Identity:  "integ test",
	})
	s.Nil(err2)
	s.assertLastHistoryEvent(we, 6, types.EventTypeDecisionTaskScheduled)

	ctx, cancel = createContext()
	defer cancel()
	// then terminate the worklfow
	err := s.engine.TerminateWorkflowExecution(ctx, &types.TerminateWorkflowExecutionRequest{
		Domain:            s.domainName,
		WorkflowExecution: we,
		Reason:            "test-reason",
	})
	s.Nil(err)

	expectedHistory := []types.EventType{
		types.EventTypeWorkflowExecutionStarted,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeDecisionTaskStarted,
		types.EventTypeDecisionTaskFailed,
		types.EventTypeWorkflowExecutionSignaled,
		types.EventTypeDecisionTaskScheduled,
		types.EventTypeWorkflowExecutionTerminated,
	}
	s.assertHistory(we, expectedHistory)
}

func (s *IntegrationSuite) assertHistory(we *types.WorkflowExecution, expectedHistory []types.EventType) {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    s.domainName,
		Execution: we,
	})
	s.Nil(err)
	history := historyResponse.History
	data, err := json.MarshalIndent(history, "", "    ")
	s.Nil(err)
	s.Equal(len(expectedHistory), len(history.Events), string(data))
	for i, e := range history.Events {
		s.Equal(expectedHistory[i], e.GetEventType(), "%v, %v, %v", strconv.Itoa(i), e.GetEventType().String(), string(data))
	}
}

func (s *IntegrationSuite) assertLastHistoryEvent(we *types.WorkflowExecution, count int, eventType types.EventType) {
	ctx, cancel := createContext()
	defer cancel()
	historyResponse, err := s.engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain:    s.domainName,
		Execution: we,
	})
	s.Nil(err)
	history := historyResponse.History
	data, err := json.MarshalIndent(history, "", "    ")
	s.Nil(err)
	s.Equal(count, len(history.Events), string(data))
	s.Equal(eventType, history.Events[len(history.Events)-1].GetEventType(), string(data))
}
