// Copyright (c) 2020 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mutable_state_task_refresher_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"context"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/events"
)

var emptyTasks = []persistence.Task{}

type (
	// MutableStateTaskRefresher refreshes workflow transfer and timer tasks
	MutableStateTaskRefresher interface {
		RefreshTasks(ctx context.Context, startTime time.Time, mutableState MutableState) error
	}

	mutableStateTaskRefresherImpl struct {
		config          *config.Config
		clusterMetadata cluster.Metadata
		domainCache     cache.DomainCache
		eventsCache     events.Cache
		shardID         int
		logger          log.Logger

		newMutableStateTaskGeneratorFn                 func(log.Logger, cluster.Metadata, cache.DomainCache, MutableState) MutableStateTaskGenerator
		refreshTasksForWorkflowStartFn                 func(context.Context, time.Time, MutableState, MutableStateTaskGenerator) error
		refreshTasksForWorkflowCloseFn                 func(context.Context, MutableState, MutableStateTaskGenerator, int) error
		refreshTasksForRecordWorkflowStartedFn         func(context.Context, MutableState, MutableStateTaskGenerator) error
		refreshTasksForDecisionFn                      func(context.Context, MutableState, MutableStateTaskGenerator) error
		refreshTasksForActivityFn                      func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache, func(MutableState) TimerSequence) error
		refreshTasksForTimerFn                         func(context.Context, MutableState, MutableStateTaskGenerator, func(MutableState) TimerSequence) error
		refreshTasksForChildWorkflowFn                 func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForRequestCancelExternalWorkflowFn func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForSignalExternalWorkflowFn        func(context.Context, MutableState, MutableStateTaskGenerator, int, events.Cache) error
		refreshTasksForWorkflowSearchAttrFn            func(context.Context, MutableState, MutableStateTaskGenerator) error
	}
)

// NewMutableStateTaskRefresher creates a new task refresher for mutable state
func NewMutableStateTaskRefresher(
	config *config.Config,
	clusterMetadata cluster.Metadata,
	domainCache cache.DomainCache,
	eventsCache events.Cache,
	shardID int,
	logger log.Logger,
) MutableStateTaskRefresher {
	return &mutableStateTaskRefresherImpl{
		config:          config,
		clusterMetadata: clusterMetadata,
		domainCache:     domainCache,
		eventsCache:     eventsCache,
		shardID:         shardID,
		logger:          logger,

		newMutableStateTaskGeneratorFn:                 NewMutableStateTaskGenerator,
		refreshTasksForWorkflowStartFn:                 refreshTasksForWorkflowStart,
		refreshTasksForWorkflowCloseFn:                 refreshTasksForWorkflowClose,
		refreshTasksForRecordWorkflowStartedFn:         refreshTasksForRecordWorkflowStarted,
		refreshTasksForDecisionFn:                      refreshTasksForDecision,
		refreshTasksForActivityFn:                      refreshTasksForActivity,
		refreshTasksForTimerFn:                         refreshTasksForTimer,
		refreshTasksForChildWorkflowFn:                 refreshTasksForChildWorkflow,
		refreshTasksForRequestCancelExternalWorkflowFn: refreshTasksForRequestCancelExternalWorkflow,
		refreshTasksForSignalExternalWorkflowFn:        refreshTasksForSignalExternalWorkflow,
		refreshTasksForWorkflowSearchAttrFn:            refreshTasksForWorkflowSearchAttr,
	}
}

func (r *mutableStateTaskRefresherImpl) RefreshTasks(
	ctx context.Context,
	startTime time.Time,
	mutableState MutableState,
) error {
	taskGenerator := r.newMutableStateTaskGeneratorFn(
		r.logger,
		r.clusterMetadata,
		r.domainCache,
		mutableState,
	)

	if err := r.refreshTasksForWorkflowStartFn(
		ctx,
		startTime,
		mutableState,
		taskGenerator,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForWorkflowCloseFn(
		ctx,
		mutableState,
		taskGenerator,
		r.config.WorkflowDeletionJitterRange(mutableState.GetDomainEntry().GetInfo().Name),
	); err != nil {
		return err
	}

	if err := r.refreshTasksForRecordWorkflowStartedFn(
		ctx,
		mutableState,
		taskGenerator,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForDecisionFn(
		ctx,
		mutableState,
		taskGenerator,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForActivityFn(
		ctx,
		mutableState,
		taskGenerator,
		r.shardID,
		r.eventsCache,
		NewTimerSequence,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForTimerFn(
		ctx,
		mutableState,
		taskGenerator,
		NewTimerSequence,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForChildWorkflowFn(
		ctx,
		mutableState,
		taskGenerator,
		r.shardID,
		r.eventsCache,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForRequestCancelExternalWorkflowFn(
		ctx,
		mutableState,
		taskGenerator,
		r.shardID,
		r.eventsCache,
	); err != nil {
		return err
	}

	if err := r.refreshTasksForSignalExternalWorkflowFn(
		ctx,
		mutableState,
		taskGenerator,
		r.shardID,
		r.eventsCache,
	); err != nil {
		return err
	}

	if common.IsAdvancedVisibilityWritingEnabled(r.config.WriteVisibilityStoreName(), r.config.IsAdvancedVisConfigExist) {
		if err := r.refreshTasksForWorkflowSearchAttrFn(
			ctx,
			mutableState,
			taskGenerator,
		); err != nil {
			return err
		}
	}

	return nil
}

func refreshTasksForWorkflowStart(
	ctx context.Context,
	startTime time.Time,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
) error {
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return err
	}

	if err := taskGenerator.GenerateWorkflowStartTasks(
		startTime,
		startEvent,
	); err != nil {
		return err
	}

	startAttr := startEvent.WorkflowExecutionStartedEventAttributes
	if !mutableState.HasProcessedOrPendingDecision() && startAttr.GetFirstDecisionTaskBackoffSeconds() > 0 {
		if err := taskGenerator.GenerateDelayedDecisionTasks(
			startEvent,
		); err != nil {
			return err
		}
	}

	return nil
}

func refreshTasksForWorkflowClose(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	workflowDeletionTaskJitterRange int,
) error {
	executionInfo := mutableState.GetExecutionInfo()
	if executionInfo.CloseStatus != persistence.WorkflowCloseStatusNone {
		closeEvent, err := mutableState.GetCompletionEvent(ctx)
		if err != nil {
			return err
		}
		return taskGenerator.GenerateWorkflowCloseTasks(
			closeEvent,
			workflowDeletionTaskJitterRange,
		)
	}
	return nil
}

func refreshTasksForRecordWorkflowStarted(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
) error {
	executionInfo := mutableState.GetExecutionInfo()
	if executionInfo.CloseStatus == persistence.WorkflowCloseStatusNone {
		startEvent, err := mutableState.GetStartEvent(ctx)
		if err != nil {
			return err
		}
		return taskGenerator.GenerateRecordWorkflowStartedTasks(
			startEvent,
		)
	}
	return nil
}

func refreshTasksForDecision(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
) error {
	if !mutableState.HasPendingDecision() {
		// no decision task at all
		return nil
	}

	decision, ok := mutableState.GetPendingDecision()
	if !ok {
		return &types.InternalServiceError{Message: "it could be a bug, cannot get pending decision"}
	}

	// decision already started
	if decision.StartedID != constants.EmptyEventID {
		return taskGenerator.GenerateDecisionStartTasks(
			decision.ScheduleID,
		)
	}

	// decision only scheduled
	return taskGenerator.GenerateDecisionScheduleTasks(
		decision.ScheduleID,
	)
}

func refreshTasksForActivity(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	shardID int,
	eventsCache events.Cache,
	newTimerSequenceFn func(MutableState) TimerSequence,
) error {
	currentBranchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return err
	}
	executionInfo := mutableState.GetExecutionInfo()
	pendingActivityInfos := mutableState.GetPendingActivityInfos()
	for _, activityInfo := range pendingActivityInfos {
		// clear all activity timer task mask for later activity timer task re-generation
		activityInfo.TimerTaskStatus = TimerTaskStatusNone
		// need to update activity timer task mask for which task is generated
		if err := mutableState.UpdateActivity(
			activityInfo,
		); err != nil {
			return err
		}
		if activityInfo.StartedID != constants.EmptyEventID {
			continue
		}
		scheduleEvent, err := eventsCache.GetEvent(
			ctx,
			shardID,
			executionInfo.DomainID,
			executionInfo.WorkflowID,
			executionInfo.RunID,
			activityInfo.ScheduledEventBatchID,
			activityInfo.ScheduleID,
			currentBranchToken,
		)
		if err != nil {
			return err
		}
		if err := taskGenerator.GenerateActivityTransferTasks(
			scheduleEvent,
		); err != nil {
			return err
		}
	}

	if _, err := newTimerSequenceFn(
		mutableState,
	).CreateNextActivityTimer(); err != nil {
		return err
	}

	return nil
}

func refreshTasksForTimer(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	newTimerSequenceFn func(MutableState) TimerSequence,
) error {
	pendingTimerInfos := mutableState.GetPendingTimerInfos()

	for _, timerInfo := range pendingTimerInfos {
		// clear all timer task mask for later timer task re-generation
		timerInfo.TaskStatus = TimerTaskStatusNone

		// need to update user timer task mask for which task is generated
		if err := mutableState.UpdateUserTimer(
			timerInfo,
		); err != nil {
			return err
		}
	}

	if _, err := newTimerSequenceFn(mutableState).CreateNextUserTimer(); err != nil {
		return err
	}

	return nil
}

func refreshTasksForChildWorkflow(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	shardID int,
	eventsCache events.Cache,
) error {
	currentBranchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return err
	}
	executionInfo := mutableState.GetExecutionInfo()
	pendingChildWorkflowInfos := mutableState.GetPendingChildExecutionInfos()
	for _, childWorkflowInfo := range pendingChildWorkflowInfos {
		if childWorkflowInfo.StartedID != constants.EmptyEventID {
			continue
		}
		scheduleEvent, err := eventsCache.GetEvent(
			ctx,
			shardID,
			executionInfo.DomainID,
			executionInfo.WorkflowID,
			executionInfo.RunID,
			childWorkflowInfo.InitiatedEventBatchID,
			childWorkflowInfo.InitiatedID,
			currentBranchToken,
		)
		if err != nil {
			return err
		}
		if err := taskGenerator.GenerateChildWorkflowTasks(
			scheduleEvent,
		); err != nil {
			return err
		}
	}
	return nil
}

func refreshTasksForRequestCancelExternalWorkflow(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	shardID int,
	eventsCache events.Cache,
) error {
	currentBranchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return err
	}
	executionInfo := mutableState.GetExecutionInfo()
	pendingRequestCancelInfos := mutableState.GetPendingRequestCancelExternalInfos()
	for _, requestCancelInfo := range pendingRequestCancelInfos {
		initiateEvent, err := eventsCache.GetEvent(
			ctx,
			shardID,
			executionInfo.DomainID,
			executionInfo.WorkflowID,
			executionInfo.RunID,
			requestCancelInfo.InitiatedEventBatchID,
			requestCancelInfo.InitiatedID,
			currentBranchToken,
		)
		if err != nil {
			return err
		}
		if err := taskGenerator.GenerateRequestCancelExternalTasks(
			initiateEvent,
		); err != nil {
			return err
		}
	}
	return nil
}

func refreshTasksForSignalExternalWorkflow(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
	shardID int,
	eventsCache events.Cache,
) error {
	currentBranchToken, err := mutableState.GetCurrentBranchToken()
	if err != nil {
		return err
	}
	executionInfo := mutableState.GetExecutionInfo()
	pendingSignalInfos := mutableState.GetPendingSignalExternalInfos()
	for _, signalInfo := range pendingSignalInfos {
		initiateEvent, err := eventsCache.GetEvent(
			ctx,
			shardID,
			executionInfo.DomainID,
			executionInfo.WorkflowID,
			executionInfo.RunID,
			signalInfo.InitiatedEventBatchID,
			signalInfo.InitiatedID,
			currentBranchToken,
		)
		if err != nil {
			return err
		}
		if err := taskGenerator.GenerateSignalExternalTasks(
			initiateEvent,
		); err != nil {
			return err
		}
	}
	return nil
}

func refreshTasksForWorkflowSearchAttr(
	ctx context.Context,
	mutableState MutableState,
	taskGenerator MutableStateTaskGenerator,
) error {
	return taskGenerator.GenerateWorkflowSearchAttrTasks()
}
