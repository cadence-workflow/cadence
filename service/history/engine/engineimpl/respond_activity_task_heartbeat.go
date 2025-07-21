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
	"fmt"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/workflow"
)

// RecordActivityTaskHeartbeat records an heartbeat for a task.
// This method can be used for two purposes.
// - For reporting liveness of the activity.
// - For reporting progress of the activity, this can be done even if the liveness is not configured.
func (e *historyEngineImpl) RecordActivityTaskHeartbeat(
	ctx context.Context,
	req *types.HistoryRecordActivityTaskHeartbeatRequest,
) (*types.RecordActivityTaskHeartbeatResponse, error) {

	domainEntry, err := e.getActiveDomainByID(req.DomainUUID)
	if err != nil {
		return nil, err
	}
	domainID := domainEntry.GetInfo().ID

	request := req.HeartbeatRequest
	token, err0 := e.tokenSerializer.Deserialize(request.TaskToken)
	if err0 != nil {
		return nil, workflow.ErrDeserializingToken
	}

	workflowExecution := types.WorkflowExecution{
		WorkflowID: token.WorkflowID,
		RunID:      token.RunID,
	}

	var cancelRequested bool
	err = workflow.UpdateWithAction(ctx, e.logger, e.executionCache, domainID, workflowExecution, false, e.timeSource.Now(),
		func(wfContext execution.Context, mutableState execution.MutableState) error {
			if !mutableState.IsWorkflowExecutionRunning() {
				e.logger.Debug("Heartbeat failed")
				return workflow.ErrAlreadyCompleted
			}

			scheduleID := token.ScheduleID
			if scheduleID == constants.EmptyEventID { // client call RecordActivityHeartbeatByID, so get scheduleID by activityID
				scheduleID, err0 = getScheduleID(token.ActivityID, mutableState)
				if err0 != nil {
					return err0
				}
			}
			ai, isRunning := mutableState.GetActivityInfo(scheduleID)

			// First check to see if cache needs to be refreshed as we could potentially have stale workflow execution in
			// some extreme cassandra failure cases.
			if !isRunning && scheduleID >= mutableState.GetNextEventID() {
				e.metricsClient.IncCounter(metrics.HistoryRecordActivityTaskHeartbeatScope, metrics.StaleMutableStateCounter)
				e.logger.Error("Encounter stale mutable state in RecordActivityTaskHeartbeat",
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowScheduleID(scheduleID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
				)
				return workflow.ErrStaleState
			}

			if !isRunning || ai.StartedID == constants.EmptyEventID ||
				(token.ScheduleID != constants.EmptyEventID && token.ScheduleAttempt != int64(ai.Attempt)) {
				e.logger.Warn(fmt.Sprintf(
					"Encounter non existing activity in RecordActivityTaskHeartbeat: isRunning: %t, ai: %#v, token: %#v.",
					isRunning, ai, token),
					tag.WorkflowDomainName(domainEntry.GetInfo().Name),
					tag.WorkflowID(workflowExecution.GetWorkflowID()),
					tag.WorkflowRunID(workflowExecution.GetRunID()),
					tag.WorkflowScheduleID(scheduleID),
					tag.WorkflowNextEventID(mutableState.GetNextEventID()),
				)

				return workflow.ErrActivityTaskNotFound
			}

			cancelRequested = ai.CancelRequested

			e.logger.Debug(fmt.Sprintf("Activity HeartBeat: scheduleEventID: %v, ActivityInfo: %+v, CancelRequested: %v",
				scheduleID, ai, cancelRequested))

			// Save progress and last HB reported time.
			mutableState.UpdateActivityProgress(ai, request)

			return nil
		})

	if err != nil {
		return &types.RecordActivityTaskHeartbeatResponse{}, err
	}

	return &types.RecordActivityTaskHeartbeatResponse{CancelRequested: cancelRequested}, nil
}
