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

package tasklist

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	taskCompleterImpl struct {
		domainCache     cache.DomainCache
		taskListID      *Identifier
		clusterMetadata cluster.Metadata
		historyService  history.Client
		scope           metrics.Scope
		logger          log.Logger
		throttleRetry   *backoff.ThrottleRetry
	}
)

// DomainIsActiveInThisClusterError type
type DomainIsActiveInThisClusterError struct {
	Message string
}

// Implement the Error method for DomainIsActiveInThisClusterError
func (e *DomainIsActiveInThisClusterError) Error() string {
	return e.Message
}

var (
	errWorkflowExecutionInfoIsNil      = errors.New("workflow execution info is nil")
	errTaskTypeNotSupported            = errors.New("task type not supported")
	errTaskNotStarted                  = errors.New("task not started")
	errDomainIsActive                  = &DomainIsActiveInThisClusterError{Message: "domain is active"}
	historyServiceOperationRetryPolicy = common.CreateTaskCompleterRetryPolicy()
)

func newTaskCompleter(tlMgr *taskListManagerImpl, retryPolicy backoff.RetryPolicy) TaskCompleter {
	return &taskCompleterImpl{
		domainCache:     tlMgr.domainCache,
		historyService:  tlMgr.historyService,
		taskListID:      tlMgr.taskListID,
		clusterMetadata: tlMgr.clusterMetadata,
		scope:           tlMgr.scope,
		logger:          tlMgr.logger,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(retryPolicy),
			backoff.WithRetryableError(isRetryableError),
		),
	}
}

func (tc *taskCompleterImpl) CompleteTaskIfStarted(ctx context.Context, task *InternalTask) error {
	op := func() (err error) {
		domainEntry, err := tc.domainCache.GetDomainByID(task.Event.TaskInfo.DomainID)
		if err != nil {
			return fmt.Errorf("unable to fetch domain from cache: %w", err)
		}

		if isActive, _ := domainEntry.IsActiveIn(tc.clusterMetadata.GetCurrentClusterName()); isActive {
			return errDomainIsActive
		}

		req := &types.HistoryDescribeWorkflowExecutionRequest{
			DomainUUID: task.Event.TaskInfo.DomainID,
			Request: &types.DescribeWorkflowExecutionRequest{
				Domain: task.domainName,
				Execution: &types.WorkflowExecution{
					WorkflowID: task.Event.WorkflowID,
					RunID:      task.Event.RunID,
				},
			},
		}

		workflowExecutionResponse, err := tc.historyService.DescribeWorkflowExecution(ctx, req)

		if errors.As(err, new(*types.EntityNotExistsError)) {
			tc.logger.Warn("Workflow execution not found while attempting to complete task on standby cluster", tag.WorkflowID(task.Event.WorkflowID), tag.WorkflowRunID(task.Event.RunID))
			return nil
		} else if err != nil {
			return fmt.Errorf("unable to fetch workflow execution from the history service: %w", err)
		}

		if workflowExecutionResponse.WorkflowExecutionInfo == nil {
			return errWorkflowExecutionInfoIsNil
		}

		isTaskStarted, err := tc.isTaskStarted(task, workflowExecutionResponse)
		if err != nil {
			return fmt.Errorf("unable to determine if task has started: %w", err)
		}

		if isTaskStarted {
			task.Finish(nil)

			tc.scope.IncCounter(metrics.StandbyClusterTasksCompletedCounterPerTaskList)

			return nil
		}

		tc.scope.IncCounter(metrics.StandbyClusterTasksNotStartedCounterPerTaskList)

		return errTaskNotStarted
	}

	err := tc.throttleRetry.Do(ctx, op)

	if !errors.Is(err, errDomainIsActive) && !errors.Is(err, errTaskNotStarted) {
		tc.scope.IncCounter(metrics.StandbyClusterTasksCompletionFailurePerTaskList)
		tc.logger.Error("Error completing task on domain's standby cluster", tag.Error(err))
	}

	return err
}

func (tc *taskCompleterImpl) isTaskStarted(task *InternalTask, workflowExecutionResponse *types.DescribeWorkflowExecutionResponse) (bool, error) {
	if workflowExecutionResponse.WorkflowExecutionInfo.CloseStatus != nil {
		return true, nil
	}

	// taskType can only be Activity or Decision, leaving the default case for future proofing
	switch tc.taskListID.taskType {
	case persistence.TaskListTypeActivity:
		return isActivityTaskStarted(task, workflowExecutionResponse), nil
	case persistence.TaskListTypeDecision:
		return isDecisionTaskStarted(task, workflowExecutionResponse), nil
	default:
		return false, errTaskTypeNotSupported
	}
}

func isDecisionTaskStarted(task *InternalTask, workflowExecutionResponse *types.DescribeWorkflowExecutionResponse) bool {
	// if there is no pending decision task, that means that this task has been already started
	if workflowExecutionResponse.PendingDecision == nil {
		return true
	}

	// if the scheduleID is different from the pending decision scheduleID or the state is started, then the decision task with the task's scheduleID has already been started
	if task.Event.ScheduleID < workflowExecutionResponse.PendingDecision.ScheduleID || (task.Event.ScheduleID == workflowExecutionResponse.PendingDecision.ScheduleID && *workflowExecutionResponse.PendingDecision.State == types.PendingDecisionStateStarted) {
		return true
	}

	return false
}

func isActivityTaskStarted(task *InternalTask, workflowExecutionResponse *types.DescribeWorkflowExecutionResponse) bool {
	// if the scheduleID is different from all pending tasks' scheduleID or the pending activity has PendingActivityStateStarted, then the activity task with the task's scheduleID has already been started
	for _, pendingActivity := range workflowExecutionResponse.PendingActivities {
		if task.Event.ScheduleID == pendingActivity.ScheduleID {
			if *pendingActivity.State == types.PendingActivityStateStarted {
				return true
			}
			return false
		}
	}
	return true
}

func isRetryableError(err error) bool {
	// entityNotExistsError is returned when the workflow is not found in the database
	var domainIsActiveInThisClusterError *DomainIsActiveInThisClusterError
	switch {
	case errors.As(err, &domainIsActiveInThisClusterError):
		return false
	}
	return true
}
