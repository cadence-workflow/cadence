// Copyright (c) 2017 Uber Technologies, Inc.
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
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/tasklist"
)

type (
	decisionTaskHandler func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error)
	activityTaskHandler func(execution *types.WorkflowExecution, activityType *types.ActivityType,
		ActivityID string, input []byte, takeToken []byte) ([]byte, bool, error)

	queryHandler func(task *types.PollForDecisionTaskResponse) ([]byte, error)

	// TaskPoller is used in integration tests to poll decision or activity tasks
	TaskPoller struct {
		Engine                              FrontendClient
		Domain                              string
		TaskList                            *types.TaskList
		StickyTaskList                      *types.TaskList
		StickyScheduleToStartTimeoutSeconds *int32
		Identity                            string
		DecisionHandler                     decisionTaskHandler
		ActivityHandler                     activityTaskHandler
		QueryHandler                        queryHandler
		Logger                              log.Logger
		T                                   *testing.T
		CallOptions                         []yarpc.CallOption
	}
)

// PollAndProcessDecisionTask for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTask(dumpHistory bool, dropTask bool) (isQueryTask bool, err error) {
	return p.PollAndProcessDecisionTaskWithAttempt(dumpHistory, dropTask, false, false, int64(0))
}

// PollAndProcessDecisionTaskWithSticky for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTaskWithSticky(dumpHistory bool, dropTask bool) (isQueryTask bool, err error) {
	return p.PollAndProcessDecisionTaskWithAttempt(dumpHistory, dropTask, true, true, int64(0))
}

// PollAndProcessDecisionTaskWithoutRetry for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTaskWithoutRetry(dumpHistory bool, dropTask bool) (isQueryTask bool, err error) {
	return p.PollAndProcessDecisionTaskWithAttemptAndRetry(dumpHistory, dropTask, false, false, int64(0), 1)
}

// PollAndProcessDecisionTaskWithAttempt for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTaskWithAttempt(
	dumpHistory bool,
	dropTask bool,
	pollStickyTaskList bool,
	respondStickyTaskList bool,
	decisionAttempt int64,
) (isQueryTask bool, err error) {

	return p.PollAndProcessDecisionTaskWithAttemptAndRetry(
		dumpHistory,
		dropTask,
		pollStickyTaskList,
		respondStickyTaskList,
		decisionAttempt,
		5)
}

// PollAndProcessDecisionTaskWithAttemptAndRetry for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTaskWithAttemptAndRetry(
	dumpHistory bool,
	dropTask bool,
	pollStickyTaskList bool,
	respondStickyTaskList bool,
	decisionAttempt int64,
	retryCount int,
) (isQueryTask bool, err error) {

	isQueryTask, _, err = p.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		dumpHistory,
		dropTask,
		pollStickyTaskList,
		respondStickyTaskList,
		decisionAttempt,
		retryCount,
		false,
		nil)
	return isQueryTask, err
}

// PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision for decision tasks
func (p *TaskPoller) PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
	dumpHistory bool,
	dropTask bool,
	pollStickyTaskList bool,
	respondStickyTaskList bool,
	decisionAttempt int64,
	retryCount int,
	forceCreateNewDecision bool,
	queryResult *types.WorkflowQueryResult,
) (isQueryTask bool, newTask *types.RespondDecisionTaskCompletedResponse, err error) {
Loop:
	for attempt := 0; attempt < retryCount; attempt++ {

		taskList := p.TaskList
		if pollStickyTaskList {
			taskList = p.StickyTaskList
		}
		ctx, cancel := createContext()
		response, err1 := p.Engine.PollForDecisionTask(ctx, &types.PollForDecisionTaskRequest{
			Domain:   p.Domain,
			TaskList: taskList,
			Identity: p.Identity,
		}, p.CallOptions...)
		cancel()

		if err1 != nil {
			return false, nil, err1
		}

		if response == nil || len(response.TaskToken) == 0 {
			p.Logger.Info("Empty Decision task: Polling again.")
			continue Loop
		}

		if response.GetNextEventID() == 0 {
			p.Logger.Fatal("NextEventID is not set for decision or query task")
		}

		var events []*types.HistoryEvent
		if response.Query == nil || !pollStickyTaskList {
			// if not query task, should have some history events
			// for non sticky query, there should be events returned
			history := response.History
			if history == nil {
				p.Logger.Fatal("History is nil")
			}

			events = history.Events
			if len(events) == 0 {
				p.Logger.Fatal("History Events are empty")
			}

			nextPageToken := response.NextPageToken
			for nextPageToken != nil {
				ctx, cancel := createContext()
				resp, err2 := p.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
					Domain:        p.Domain,
					Execution:     response.WorkflowExecution,
					NextPageToken: nextPageToken,
				}, p.CallOptions...)
				cancel()

				if err2 != nil {
					return false, nil, err2
				}

				events = append(events, resp.History.Events...)
				nextPageToken = resp.NextPageToken
			}
		} else {
			// for sticky query, there should be NO events returned
			// since worker side already has the state machine and we do not intend to update that.
			history := response.History
			nextPageToken := response.NextPageToken
			if !(history == nil || (len(history.Events) == 0 && nextPageToken == nil)) {
				// if history is not nil, and contains events or next token
				p.Logger.Fatal("History is not empty for sticky query")
			}
		}

		if dropTask {
			p.Logger.Info("Dropping Decision task: ")
			return false, nil, nil
		}

		if dumpHistory {
			PrettyPrintHistory(response.History, p.Logger)
		}

		// handle query task response
		if response.Query != nil {
			blob, err := p.QueryHandler(response)

			completeRequest := &types.RespondQueryTaskCompletedRequest{TaskToken: response.TaskToken}
			if err != nil {
				completeType := types.QueryTaskCompletedTypeFailed
				completeRequest.CompletedType = &completeType
				completeRequest.ErrorMessage = err.Error()
			} else {
				completeType := types.QueryTaskCompletedTypeCompleted
				completeRequest.CompletedType = &completeType
				completeRequest.QueryResult = blob
			}

			ctx, cancel := createContext()
			taskErr := p.Engine.RespondQueryTaskCompleted(ctx, completeRequest, p.CallOptions...)
			cancel()
			return true, nil, taskErr
		}

		// handle normal decision task / non query task response
		var lastDecisionScheduleEvent *types.HistoryEvent
		for _, e := range events {
			if e.GetEventType() == types.EventTypeDecisionTaskScheduled {
				lastDecisionScheduleEvent = e
			}
		}
		if lastDecisionScheduleEvent != nil && decisionAttempt > 0 {
			require.Equal(p.T, decisionAttempt, lastDecisionScheduleEvent.DecisionTaskScheduledEventAttributes.GetAttempt())
		}

		executionCtx, decisions, err := p.DecisionHandler(response.WorkflowExecution, response.WorkflowType,
			common.Int64Default(response.PreviousStartedEventID), response.StartedEventID, response.History)
		if err != nil {
			p.Logger.Info("Failing Decision. Decision handler failed with error", tag.Error(err))
			ctx, cancel := createContext()
			taskErr := p.Engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
				TaskToken: response.TaskToken,
				Cause:     types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
				Details:   []byte(err.Error()),
				Identity:  p.Identity,
			}, p.CallOptions...)
			cancel()
			return isQueryTask, nil, taskErr
		}

		p.Logger.Info("Completing Decision.  Decisions", tag.Value(decisions))
		if !respondStickyTaskList {
			// non sticky tasklist
			ctx, cancel := createContext()
			newTask, err := p.Engine.RespondDecisionTaskCompleted(ctx, &types.RespondDecisionTaskCompletedRequest{
				TaskToken:                  response.TaskToken,
				Identity:                   p.Identity,
				ExecutionContext:           executionCtx,
				Decisions:                  decisions,
				ReturnNewDecisionTask:      forceCreateNewDecision,
				ForceCreateNewDecisionTask: forceCreateNewDecision,
				QueryResults:               getQueryResults(response.GetQueries(), queryResult),
			}, p.CallOptions...)
			cancel()
			return false, newTask, err
		}
		// sticky tasklist
		ctx, cancel = createContext()
		newTask, err := p.Engine.RespondDecisionTaskCompleted(
			ctx,
			&types.RespondDecisionTaskCompletedRequest{
				TaskToken:        response.TaskToken,
				Identity:         p.Identity,
				ExecutionContext: executionCtx,
				Decisions:        decisions,
				StickyAttributes: &types.StickyExecutionAttributes{
					WorkerTaskList:                p.StickyTaskList,
					ScheduleToStartTimeoutSeconds: p.StickyScheduleToStartTimeoutSeconds,
				},
				ReturnNewDecisionTask:      forceCreateNewDecision,
				ForceCreateNewDecisionTask: forceCreateNewDecision,
				QueryResults:               getQueryResults(response.GetQueries(), queryResult),
			},
			p.CallOptions...,
		)
		cancel()

		return false, newTask, err
	}

	return false, nil, tasklist.ErrNoTasks
}

// HandlePartialDecision for decision task
func (p *TaskPoller) HandlePartialDecision(response *types.PollForDecisionTaskResponse) (
	*types.RespondDecisionTaskCompletedResponse, error) {
	if response == nil || len(response.TaskToken) == 0 {
		p.Logger.Info("Empty Decision task: Polling again.")
		return nil, nil
	}

	var events []*types.HistoryEvent
	history := response.History
	if history == nil {
		p.Logger.Fatal("History is nil")
	}

	events = history.Events
	if len(events) == 0 {
		p.Logger.Fatal("History Events are empty")
	}

	executionCtx, decisions, err := p.DecisionHandler(response.WorkflowExecution, response.WorkflowType,
		common.Int64Default(response.PreviousStartedEventID), response.StartedEventID, response.History)
	if err != nil {
		p.Logger.Info("Failing Decision. Decision handler failed with error: %v", tag.Error(err))
		ctx, cancel := createContext()
		defer cancel()
		return nil, p.Engine.RespondDecisionTaskFailed(ctx, &types.RespondDecisionTaskFailedRequest{
			TaskToken: response.TaskToken,
			Cause:     types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
			Details:   []byte(err.Error()),
			Identity:  p.Identity,
		}, p.CallOptions...)
	}

	p.Logger.Info("Completing Decision.  Decisions: %v", tag.Value(decisions))

	// sticky tasklist
	ctx, cancel := createContext()
	defer cancel()
	newTask, err := p.Engine.RespondDecisionTaskCompleted(
		ctx,
		&types.RespondDecisionTaskCompletedRequest{
			TaskToken:        response.TaskToken,
			Identity:         p.Identity,
			ExecutionContext: executionCtx,
			Decisions:        decisions,
			StickyAttributes: &types.StickyExecutionAttributes{
				WorkerTaskList:                p.StickyTaskList,
				ScheduleToStartTimeoutSeconds: p.StickyScheduleToStartTimeoutSeconds,
			},
			ReturnNewDecisionTask:      true,
			ForceCreateNewDecisionTask: true,
		},
		p.CallOptions...,
	)

	return newTask, err
}

// PollAndProcessActivityTask for activity tasks
func (p *TaskPoller) PollAndProcessActivityTask(dropTask bool) error {
	for attempt := 0; attempt < 5; attempt++ {
		ctx, ctxCancel := createContext()
		response, err1 := p.Engine.PollForActivityTask(ctx, &types.PollForActivityTaskRequest{
			Domain:   p.Domain,
			TaskList: p.TaskList,
			Identity: p.Identity,
		}, p.CallOptions...)
		ctxCancel()

		if err1 != nil {
			return err1
		}

		if response == nil || len(response.TaskToken) == 0 {
			p.Logger.Info("Empty Activity task: Polling again.")
			return nil
		}

		if dropTask {
			p.Logger.Info("Dropping Activity task: ")
			return nil
		}
		p.Logger.Debug("Received Activity task", tag.Value(response))

		result, cancel, err2 := p.ActivityHandler(response.WorkflowExecution, response.ActivityType, response.ActivityID,
			response.Input, response.TaskToken)
		if cancel {
			p.Logger.Info("Executing RespondActivityTaskCanceled")
			ctx, ctxCancel := createContext()
			taskErr := p.Engine.RespondActivityTaskCanceled(ctx, &types.RespondActivityTaskCanceledRequest{
				TaskToken: response.TaskToken,
				Details:   []byte("details"),
				Identity:  p.Identity,
			}, p.CallOptions...)
			ctxCancel()
			return taskErr
		}

		if err2 != nil {
			ctx, ctxCancel := createContext()
			taskErr := p.Engine.RespondActivityTaskFailed(ctx, &types.RespondActivityTaskFailedRequest{
				TaskToken: response.TaskToken,
				Reason:    common.StringPtr(err2.Error()),
				Details:   []byte(err2.Error()),
				Identity:  p.Identity,
			}, p.CallOptions...)
			ctxCancel()
			return taskErr
		}

		ctx, ctxCancel = createContext()
		taskErr := p.Engine.RespondActivityTaskCompleted(ctx, &types.RespondActivityTaskCompletedRequest{
			TaskToken: response.TaskToken,
			Identity:  p.Identity,
			Result:    result,
		}, p.CallOptions...)
		ctxCancel()
		return taskErr
	}

	return tasklist.ErrNoTasks
}

// PollAndProcessActivityTaskWithID is similar to PollAndProcessActivityTask but using RespondActivityTask...ByID
func (p *TaskPoller) PollAndProcessActivityTaskWithID(dropTask bool) error {
	for attempt := 0; attempt < 5; attempt++ {
		ctx, ctxCancel := createContext()
		response, err1 := p.Engine.PollForActivityTask(ctx, &types.PollForActivityTaskRequest{
			Domain:   p.Domain,
			TaskList: p.TaskList,
			Identity: p.Identity,
		}, p.CallOptions...)
		ctxCancel()

		if err1 != nil {
			return err1
		}

		if response == nil || len(response.TaskToken) == 0 {
			p.Logger.Info("Empty Activity task: Polling again.")
			return nil
		}

		if response.GetActivityID() == "" {
			p.Logger.Info("Empty ActivityID")
			return nil
		}

		if dropTask {
			p.Logger.Info("Dropping Activity task: ")
			return nil
		}
		p.Logger.Debug("Received Activity task: %v", tag.Value(response))

		result, cancel, err2 := p.ActivityHandler(response.WorkflowExecution, response.ActivityType, response.ActivityID,
			response.Input, response.TaskToken)
		if cancel {
			p.Logger.Info("Executing RespondActivityTaskCanceled")
			ctx, ctxCancel := createContext()
			taskErr := p.Engine.RespondActivityTaskCanceledByID(ctx, &types.RespondActivityTaskCanceledByIDRequest{
				Domain:     p.Domain,
				WorkflowID: response.WorkflowExecution.GetWorkflowID(),
				RunID:      response.WorkflowExecution.GetRunID(),
				ActivityID: response.GetActivityID(),
				Details:    []byte("details"),
				Identity:   p.Identity,
			}, p.CallOptions...)
			ctxCancel()
			return taskErr
		}

		if err2 != nil {
			ctx, ctxCancel := createContext()
			taskErr := p.Engine.RespondActivityTaskFailedByID(ctx, &types.RespondActivityTaskFailedByIDRequest{
				Domain:     p.Domain,
				WorkflowID: response.WorkflowExecution.GetWorkflowID(),
				RunID:      response.WorkflowExecution.GetRunID(),
				ActivityID: response.GetActivityID(),
				Reason:     common.StringPtr(err2.Error()),
				Details:    []byte(err2.Error()),
				Identity:   p.Identity,
			}, p.CallOptions...)
			ctxCancel()
			return taskErr
		}

		ctx, ctxCancel = createContext()
		taskErr := p.Engine.RespondActivityTaskCompletedByID(ctx, &types.RespondActivityTaskCompletedByIDRequest{
			Domain:     p.Domain,
			WorkflowID: response.WorkflowExecution.GetWorkflowID(),
			RunID:      response.WorkflowExecution.GetRunID(),
			ActivityID: response.GetActivityID(),
			Identity:   p.Identity,
			Result:     result,
		}, p.CallOptions...)
		ctxCancel()
		return taskErr
	}

	return tasklist.ErrNoTasks
}

func (p *TaskPoller) PollAndProcessActivities() context.CancelFunc {
	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	go p.pollLoop(ctx, &wg, p.doPollActivityTask)

	return func() {
		cancel()
		wg.Wait()
	}
}

func (p *TaskPoller) PollAndProcessDecisions() context.CancelFunc {
	var wg sync.WaitGroup
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	go p.pollLoop(ctx, &wg, p.doPollDecisionTask)

	return func() {
		cancel()
		wg.Wait()
	}
}

func (p *TaskPoller) pollLoop(ctx context.Context, wg *sync.WaitGroup, pollFunc func(context.Context) error) {
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		default:
			err := pollFunc(ctx)
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				p.Logger.Error("Error while polling", tag.Error(err))
			}
		}
	}
}

func (p *TaskPoller) doPollActivityTask(ctx context.Context) error {
	taskList := p.TaskList
	pollCtx, cancel := context.WithTimeout(ctx, time.Second*90)
	response, err := p.Engine.PollForActivityTask(pollCtx, &types.PollForActivityTaskRequest{
		Domain:   p.Domain,
		TaskList: taskList,
		Identity: p.Identity,
	}, p.CallOptions...)
	cancel()

	if err != nil {
		return err
	}

	if response == nil || len(response.TaskToken) == 0 {
		p.Logger.Info("Empty Decision task: Polling again.")
		return nil
	}

	result, shouldCancel, err := p.ActivityHandler(response.WorkflowExecution, response.ActivityType, response.ActivityID, response.Input, response.TaskToken)

	responseCtx, ctxCancel := context.WithTimeout(ctx, time.Second*5)
	defer ctxCancel()
	var responseErr error

	if err != nil {
		p.Logger.Info("Executed RespondActivityTaskFailed")
		responseErr = p.Engine.RespondActivityTaskFailed(responseCtx, &types.RespondActivityTaskFailedRequest{
			TaskToken: response.TaskToken,
			Reason:    common.StringPtr(err.Error()),
			Details:   []byte(err.Error()),
			Identity:  p.Identity,
		}, p.CallOptions...)
	} else if shouldCancel {
		p.Logger.Info("Executing RespondActivityTaskCanceled")
		responseErr = p.Engine.RespondActivityTaskCanceled(responseCtx, &types.RespondActivityTaskCanceledRequest{
			TaskToken: response.TaskToken,
			Details:   []byte("details"),
			Identity:  p.Identity,
		}, p.CallOptions...)
	} else {
		p.Logger.Info("Executing RespondActivityTaskCompleted")
		responseErr = p.Engine.RespondActivityTaskCompleted(responseCtx, &types.RespondActivityTaskCompletedRequest{
			TaskToken: response.TaskToken,
			Identity:  p.Identity,
			Result:    result,
		}, p.CallOptions...)
	}

	return responseErr
}

func (p *TaskPoller) doPollDecisionTask(ctx context.Context) error {
	taskList := p.TaskList
	pollCtx, cancel := context.WithTimeout(ctx, time.Second*90)
	response, err := p.Engine.PollForDecisionTask(pollCtx, &types.PollForDecisionTaskRequest{
		Domain:   p.Domain,
		TaskList: taskList,
		Identity: p.Identity,
	}, p.CallOptions...)
	cancel()

	if err != nil {
		return err
	}

	if response == nil || len(response.TaskToken) == 0 {
		p.Logger.Info("Empty Decision task: Polling again.")
		return nil
	}

	if response.GetNextEventID() == 0 {
		p.Logger.Fatal("NextEventID is not set for decision or query task")
	}

	var events []*types.HistoryEvent
	if response.Query == nil {
		// if not query task, should have some history events
		// for non sticky query, there should be events returned
		history := response.History
		if history == nil {
			p.Logger.Fatal("History is nil")
		}

		events = history.Events
		if len(events) == 0 {
			p.Logger.Fatal("History Events are empty")
		}

		nextPageToken := response.NextPageToken
		for nextPageToken != nil {
			historyCtx, cancel := context.WithTimeout(ctx, time.Second*90)
			resp, err2 := p.Engine.GetWorkflowExecutionHistory(historyCtx, &types.GetWorkflowExecutionHistoryRequest{
				Domain:        p.Domain,
				Execution:     response.WorkflowExecution,
				NextPageToken: nextPageToken,
			}, p.CallOptions...)
			cancel()

			if err2 != nil {
				return err2
			}

			events = append(events, resp.History.Events...)
			nextPageToken = resp.NextPageToken
		}
	} else {
		// for sticky query, there should be NO events returned
		// since worker side already has the state machine and we do not intend to update that.
		history := response.History
		nextPageToken := response.NextPageToken
		if !(history == nil || (len(history.Events) == 0 && nextPageToken == nil)) {
			// if history is not nil, and contains events or next token
			p.Logger.Fatal("History is not empty for sticky query")
		}
	}

	// handle query task response
	if response.Query != nil {
		blob, err := p.QueryHandler(response)

		completeRequest := &types.RespondQueryTaskCompletedRequest{TaskToken: response.TaskToken}
		if err != nil {
			completeType := types.QueryTaskCompletedTypeFailed
			completeRequest.CompletedType = &completeType
			completeRequest.ErrorMessage = err.Error()
		} else {
			completeType := types.QueryTaskCompletedTypeCompleted
			completeRequest.CompletedType = &completeType
			completeRequest.QueryResult = blob
		}

		respondCtx, cancel := context.WithTimeout(ctx, time.Second*90)
		taskErr := p.Engine.RespondQueryTaskCompleted(respondCtx, completeRequest, p.CallOptions...)
		cancel()
		return taskErr
	}
	executionCtx, decisions, err := p.DecisionHandler(response.WorkflowExecution, response.WorkflowType,
		common.Int64Default(response.PreviousStartedEventID), response.StartedEventID, response.History)
	if err != nil {
		p.Logger.Info("Failing Decision. Decision handler failed with error", tag.Error(err))
		respondCtx, cancel := context.WithTimeout(ctx, time.Second*90)
		taskErr := p.Engine.RespondDecisionTaskFailed(respondCtx, &types.RespondDecisionTaskFailedRequest{
			TaskToken: response.TaskToken,
			Cause:     types.DecisionTaskFailedCauseWorkflowWorkerUnhandledFailure.Ptr(),
			Details:   []byte(err.Error()),
			Identity:  p.Identity,
		}, p.CallOptions...)
		cancel()
		return taskErr
	}

	p.Logger.Info("Completing Decision.  Decisions", tag.Value(decisions))
	// non sticky tasklist
	respondCtx, cancel := context.WithTimeout(ctx, time.Second*90)
	_, err = p.Engine.RespondDecisionTaskCompleted(respondCtx, &types.RespondDecisionTaskCompletedRequest{
		TaskToken:                  response.TaskToken,
		Identity:                   p.Identity,
		ExecutionContext:           executionCtx,
		Decisions:                  decisions,
		ReturnNewDecisionTask:      false,
		ForceCreateNewDecisionTask: false,
	}, p.CallOptions...)
	cancel()

	return err
}

func createContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	return ctx, cancel
}

func getQueryResults(queries map[string]*types.WorkflowQuery, queryResult *types.WorkflowQueryResult) map[string]*types.WorkflowQueryResult {
	result := make(map[string]*types.WorkflowQueryResult)
	for k := range queries {
		result[k] = queryResult
	}
	return result
}
