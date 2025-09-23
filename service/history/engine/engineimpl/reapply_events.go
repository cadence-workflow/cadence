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

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/ndc"
	"github.com/uber/cadence/service/history/workflow"
)

func (e *historyEngineImpl) ReapplyEvents(
	ctx context.Context,
	domainUUID string,
	workflowID string,
	runID string,
	reapplyEvents []*types.HistoryEvent,
) error {

	domainEntry, err := e.getActiveDomainByID(domainUUID)
	if err != nil {
		switch {
		case domainEntry != nil && domainEntry.IsDomainPendingActive():
			return nil
		default:
			return err
		}
	}
	domainID := domainEntry.GetInfo().ID
	// remove run id from the execution so that reapply events to the current run
	currentExecution := types.WorkflowExecution{
		WorkflowID: workflowID,
	}

	return workflow.UpdateWithActionFunc(
		ctx,
		e.logger,
		e.executionCache,
		domainID,
		currentExecution,
		e.timeSource.Now(),
		func(wfContext execution.Context, mutableState execution.MutableState) (*workflow.UpdateAction, error) {
			// Filter out reapply event from the same cluster
			toReapplyEvents := make([]*types.HistoryEvent, 0, len(reapplyEvents))
			lastWriteVersion, err := mutableState.GetLastWriteVersion()
			if err != nil {
				return nil, err
			}
			for _, event := range reapplyEvents {
				if event.Version == lastWriteVersion {
					// The reapply is from the same cluster. Ignoring.
					continue
				}
				dedupResource := definition.NewEventReappliedID(runID, event.ID, event.Version)
				if mutableState.IsResourceDuplicated(dedupResource) {
					// already apply the signal
					continue
				}
				toReapplyEvents = append(toReapplyEvents, event)
			}
			if len(toReapplyEvents) == 0 {
				return &workflow.UpdateAction{
					Noop: true,
				}, nil
			}

			if !mutableState.IsWorkflowExecutionRunning() {
				// need to reset target workflow (which is also the current workflow)
				// to accept events to be reapplied
				baseRunID := mutableState.GetExecutionInfo().RunID
				resetRunID := uuid.New()
				baseRebuildLastEventID := mutableState.GetPreviousStartedEventID()

				// TODO when https://github.com/uber/cadence/issues/2420 is finished, remove this block,
				//  since cannot reapply event to a finished workflow which had no decisions started
				if baseRebuildLastEventID == constants.EmptyEventID {
					e.logger.Warn("cannot reapply event to a finished workflow",
						tag.WorkflowDomainID(domainID),
						tag.WorkflowID(currentExecution.GetWorkflowID()),
					)
					e.metricsClient.IncCounter(metrics.HistoryReapplyEventsScope, metrics.EventReapplySkippedCount)
					return &workflow.UpdateAction{Noop: true}, nil
				}

				baseVersionHistories := mutableState.GetVersionHistories()
				if baseVersionHistories == nil {
					return nil, execution.ErrMissingVersionHistories
				}
				baseCurrentVersionHistory, err := baseVersionHistories.GetCurrentVersionHistory()
				if err != nil {
					return nil, err
				}
				baseRebuildLastEventVersion, err := baseCurrentVersionHistory.GetEventVersion(baseRebuildLastEventID)
				if err != nil {
					return nil, err
				}
				baseCurrentBranchToken := baseCurrentVersionHistory.GetBranchToken()
				baseNextEventID := mutableState.GetNextEventID()

				if err = e.workflowResetter.ResetWorkflow(
					ctx,
					domainID,
					workflowID,
					baseRunID,
					baseCurrentBranchToken,
					baseRebuildLastEventID,
					baseRebuildLastEventVersion,
					baseNextEventID,
					resetRunID,
					uuid.New(),
					execution.NewWorkflow(
						ctx,
						e.shard.GetClusterMetadata(),
						wfContext,
						mutableState,
						execution.NoopReleaseFn,
						e.logger,
					),
					ndc.EventsReapplicationResetWorkflowReason,
					toReapplyEvents,
					false,
				); err != nil {
					return nil, err
				}
				return &workflow.UpdateAction{
					Noop: true,
				}, nil
			}

			postActions := &workflow.UpdateAction{
				CreateDecision: true,
			}
			// Do not create decision task when the workflow is cron and the cron has not been started yet
			if mutableState.GetExecutionInfo().CronSchedule != "" && !mutableState.HasProcessedOrPendingDecision() {
				postActions.CreateDecision = false
			}
			reappliedEvents, err := e.eventsReapplier.ReapplyEvents(
				ctx,
				mutableState,
				toReapplyEvents,
				runID,
			)
			if err != nil {
				e.logger.Error("failed to re-apply stale events", tag.Error(err))
				return nil, &types.InternalServiceError{Message: "unable to re-apply stale events"}
			}
			if len(reappliedEvents) == 0 {
				return &workflow.UpdateAction{
					Noop: true,
				}, nil
			}
			return postActions, nil
		},
	)
}
