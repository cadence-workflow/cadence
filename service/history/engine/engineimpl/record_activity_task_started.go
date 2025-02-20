// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

func (e *historyEngineImpl) RecordActivityTaskStarted(
	ctx context.Context,
	request *types.RecordActivityTaskStartedRequest,
) (*types.RecordActivityTaskStartedResponse, error) {

	domainEntry, err := e.getActiveDomainByID(request.DomainUUID)
	if err != nil {
		return nil, err
	}

	domainInfo := domainEntry.GetInfo()

	domainID := domainInfo.ID
	domainName := domainInfo.Name

	workflowExecution := types.WorkflowExecution{
		WorkflowID: request.WorkflowExecution.WorkflowID,
		RunID:      request.WorkflowExecution.RunID,
	}

	var resurrectError error
	response := &types.RecordActivityTaskStartedResponse{}
	err = workflow.UpdateWithAction(ctx, e.logger, e.executionCache, domainID, workflowExecution, false, e.timeSource.Now(),
		func(wfContext execution.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				return workflow.ErrNotExists
			}

			scheduleID := request.GetScheduleID()
			requestID := request.GetRequestID()
			ai, isRunning := mutableState.GetActivityInfo(scheduleID)

			// RecordActivityTaskStarted is already past scheduleToClose timeout.
			// If at this point pending activity is still in mutable state it may be resurrected.
			// Otherwise it would be completed or timed out already.
			if isRunning && e.timeSource.Now().After(ai.ScheduledTime.Add(time.Duration(ai.ScheduleToCloseTimeout)*time.Second)) {
				resurrectedActivities, err := execution.GetResurrectedActivities(ctx, e.shard, mutableState)
				if err != nil {
					e.logger.Error("Activity resurrection check failed", tag.Error(err))
					return err
				}

				if _, ok := resurrectedActivities[scheduleID]; ok {
					// found activity resurrection
					domainName := mutableState.GetDomainEntry().GetInfo().Name
					e.metricsClient.IncCounter(metrics.HistoryRecordActivityTaskStartedScope, metrics.ActivityResurrectionCounter)
					e.logger.Error("Encounter resurrected activity, skip",
						tag.WorkflowDomainName(domainName),
						tag.WorkflowID(workflowExecution.GetWorkflowID()),
						tag.WorkflowRunID(workflowExecution.GetRunID()),
						tag.WorkflowScheduleID(scheduleID),
					)

					// remove resurrected activity from mutable state
					if err := mutableState.DeleteActivity(scheduleID); err != nil {
						return err
					}

					// save resurrection error but return nil here, so that mutable state would get updated in DB
					resurrectError = workflow.ErrActivityTaskNotFound
					return nil
				}
			}

			// First check to see if cache needs to be refreshed as we could potentially have stale workflow execution in
			// some extreme cassandra failure cases.
			if !isRunning && scheduleID >= mutableState.GetNextEventID() {
				e.metricsClient.IncCounter(metrics.HistoryRecordActivityTaskStartedScope, metrics.StaleMutableStateCounter)
				e.logger.Error("Encounter stale mutable state in RecordActivityTaskStarted",
					tag.WorkflowDomainName(domainName),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowScheduleID(scheduleID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
				)
				return workflow.ErrStaleState
			}

			// Check execution state to make sure task is in the list of outstanding tasks and it is not yet started.  If
			// task is not outstanding than it is most probably a duplicate and complete the task.
			if !isRunning {
				// Looks like ActivityTask already completed as a result of another call.
				// It is OK to drop the task at this point.
				e.logger.Debug("Potentially duplicate task.", tag.TaskID(request.GetTaskID()), tag.WorkflowScheduleID(scheduleID), tag.TaskType(persistence.TransferTaskTypeActivityTask))
				return workflow.ErrActivityTaskNotFound
			}

			scheduledEvent, err := mutableState.GetActivityScheduledEvent(ctx, scheduleID)
			if err != nil {
				return err
			}
			response.ScheduledEvent = scheduledEvent
			response.ScheduledTimestampOfThisAttempt = common.Int64Ptr(ai.ScheduledTime.UnixNano())

			response.Attempt = int64(ai.Attempt)
			response.HeartbeatDetails = ai.Details

			response.WorkflowType = mutableState.GetWorkflowType()
			response.WorkflowDomain = domainName

			if ai.StartedID != constants.EmptyEventID {
				// If activity is started as part of the current request scope then return a positive response
				if ai.RequestID == requestID {
					response.StartedTimestamp = common.Int64Ptr(ai.StartedTime.UnixNano())
					return nil
				}

				// Looks like ActivityTask already started as a result of another call.
				// It is OK to drop the task at this point.
				e.logger.Debug("Potentially duplicate task.", tag.TaskID(request.GetTaskID()), tag.WorkflowScheduleID(scheduleID), tag.TaskType(persistence.TransferTaskTypeActivityTask))
				return &types.EventAlreadyStartedError{Message: "Activity task already started."}
			}

			if _, err := mutableState.AddActivityTaskStartedEvent(
				ai, scheduleID, requestID, request.PollRequest.GetIdentity(),
			); err != nil {
				return err
			}

			response.StartedTimestamp = common.Int64Ptr(ai.StartedTime.UnixNano())

			return nil
		})

	if err != nil {
		return nil, err
	}
	if resurrectError != nil {
		return nil, resurrectError
	}

	return response, err
}
