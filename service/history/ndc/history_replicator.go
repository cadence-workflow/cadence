// Copyright (c) 2019 Uber Technologies, Inc.
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

package ndc

import (
	ctx "context"
	"fmt"
	"runtime"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

const (
	mutableStateMissingMessage = "Resend events due to missing mutable state"
)

type (
	// HistoryReplicator handles history replication task
	HistoryReplicator interface {
		ApplyEvents(
			ctx ctx.Context,
			request *types.ReplicateEventsV2Request,
		) error
	}

	historyReplicatorImpl struct {
		shard              shard.Context
		clusterMetadata    cluster.Metadata
		historyV2Manager   persistence.HistoryManager
		historySerializer  persistence.PayloadSerializer
		metricsClient      metrics.Client
		domainCache        cache.DomainCache
		executionCache     execution.Cache
		eventsReapplier    EventsReapplier
		transactionManager transactionManager
		logger             log.Logger

		newBranchManagerFn    newBranchManagerFn
		newConflictResolverFn newConflictResolverFn
		newWorkflowResetterFn newWorkflowResetterFn
		newStateBuilderFn     newStateBuilderFn
		newMutableStateFn     newMutableStateFn

		// refactored functions for a better testability
		newReplicationTaskFn                                         newReplicationTaskFn
		applyStartEventsFn                                           applyStartEventsFn
		applyNonStartEventsPrepareBranchFn                           applyNonStartEventsPrepareBranchFn
		applyNonStartEventsPrepareMutableStateFn                     applyNonStartEventsPrepareMutableStateFn
		applyNonStartEventsToCurrentBranchFn                         applyNonStartEventsToCurrentBranchFn
		applyNonStartEventsToNoneCurrentBranchFn                     applyNonStartEventsToNoneCurrentBranchFn
		applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn    applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn
		applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn
		applyNonStartEventsMissingMutableStateFn                     applyNonStartEventsMissingMutableStateFn
		applyNonStartEventsResetWorkflowFn                           applyNonStartEventsResetWorkflowFn
	}

	newStateBuilderFn func(
		mutableState execution.MutableState,
		logger log.Logger) execution.StateBuilder

	newMutableStateFn func(
		domainEntry *cache.DomainCacheEntry,
		logger log.Logger,
	) execution.MutableState

	newBranchManagerFn func(
		context execution.Context,
		mutableState execution.MutableState,
		logger log.Logger,
	) branchManager

	newConflictResolverFn func(
		context execution.Context,
		mutableState execution.MutableState,
		logger log.Logger,
	) conflictResolver

	newWorkflowResetterFn func(
		domainID string,
		workflowID string,
		baseRunID string,
		newContext execution.Context,
		newRunID string,
		logger log.Logger,
	) WorkflowResetter

	newReplicationTaskFn func(
		clusterMetadata cluster.Metadata,
		historySerializer persistence.PayloadSerializer,
		taskStartTime time.Time,
		logger log.Logger,
		request *types.ReplicateEventsV2Request,
	) (replicationTask, error)

	applyStartEventsFn func(
		ctx ctx.Context,
		context execution.Context,
		releaseFn execution.ReleaseFunc,
		task replicationTask,
		domainCache cache.DomainCache,
		newMutableState newMutableStateFn,
		newStateBuilder newStateBuilderFn,
		transactionManager transactionManager,
		logger log.Logger,
		shard shard.Context,
		clusterMetadata cluster.Metadata,
	) (retError error)

	applyNonStartEventsPrepareBranchFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		task replicationTask,
		newBranchManager newBranchManagerFn,
	) (bool, int, error)

	applyNonStartEventsPrepareMutableStateFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		branchIndex int,
		task replicationTask,
		newConflictResolver newConflictResolverFn,
	) (execution.MutableState, bool, error)

	applyNonStartEventsToCurrentBranchFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		isRebuilt bool,
		releaseFn execution.ReleaseFunc,
		task replicationTask,
		newStateBuilder newStateBuilderFn,
		clusterMetadata cluster.Metadata,
		shard shard.Context,
		logger log.Logger,
		transactionManager transactionManager,
	) error

	applyNonStartEventsToNoneCurrentBranchFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		branchIndex int,
		releaseFn execution.ReleaseFunc,
		task replicationTask,
		r *historyReplicatorImpl,
		logger log.Logger,
	) error

	applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn func(
		ctx ctx.Context,
		context execution.Context,
		releaseFn execution.ReleaseFunc,
		task replicationTask,
		r *historyReplicatorImpl,
	) error

	applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		branchIndex int,
		releaseFn execution.ReleaseFunc,
		task replicationTask,
		transactionManager transactionManager,
		clusterMetadata cluster.Metadata,
		shard shard.Context,
		logger log.Logger,
	) error

	applyNonStartEventsMissingMutableStateFn func(
		ctx ctx.Context,
		newContext execution.Context,
		task replicationTask,
		newWorkflowResetter newWorkflowResetterFn,
	) (execution.MutableState, error)

	applyNonStartEventsResetWorkflowFn func(
		ctx ctx.Context,
		context execution.Context,
		mutableState execution.MutableState,
		task replicationTask,
		newStateBuilder newStateBuilderFn,
		transactionManager transactionManager,
		clusterMetadata cluster.Metadata,
		logger log.Logger,
		shard shard.Context,
	) error
)

var _ HistoryReplicator = (*historyReplicatorImpl)(nil)

var errPanic = errors.NewInternalFailureError("encounter panic")

// NewHistoryReplicator creates history replicator
func NewHistoryReplicator(
	shard shard.Context,
	executionCache execution.Cache,
	eventsReapplier EventsReapplier,
	logger log.Logger,
) HistoryReplicator {

	transactionManager := newTransactionManager(shard, executionCache, eventsReapplier, logger)
	replicator := &historyReplicatorImpl{
		shard:              shard,
		clusterMetadata:    shard.GetService().GetClusterMetadata(),
		historyV2Manager:   shard.GetHistoryManager(),
		historySerializer:  persistence.NewPayloadSerializer(),
		metricsClient:      shard.GetMetricsClient(),
		domainCache:        shard.GetDomainCache(),
		executionCache:     executionCache,
		transactionManager: transactionManager,
		eventsReapplier:    eventsReapplier,
		logger:             logger.WithTags(tag.ComponentHistoryReplicator),

		newBranchManagerFn: func(
			context execution.Context,
			mutableState execution.MutableState,
			logger log.Logger,
		) branchManager {
			return newBranchManager(shard, context, mutableState, logger)
		},
		newConflictResolverFn: func(
			context execution.Context,
			mutableState execution.MutableState,
			logger log.Logger,
		) conflictResolver {
			return newConflictResolver(shard, context, mutableState, logger)
		},
		newWorkflowResetterFn: func(
			domainID string,
			workflowID string,
			baseRunID string,
			newContext execution.Context,
			newRunID string,
			logger log.Logger,
		) WorkflowResetter {
			return NewWorkflowResetter(shard, transactionManager, domainID, workflowID, baseRunID, newContext, newRunID, logger)
		},
		newStateBuilderFn: func(
			state execution.MutableState,
			logger log.Logger,
		) execution.StateBuilder {

			return execution.NewStateBuilder(
				shard,
				logger,
				state,
			)
		},
		newMutableStateFn: func(
			domainEntry *cache.DomainCacheEntry,
			logger log.Logger,
		) execution.MutableState {
			return execution.NewMutableStateBuilderWithVersionHistories(
				shard,
				logger,
				domainEntry,
				constants.EmptyVersion,
			)
		},
		newReplicationTaskFn:                                         newReplicationTask,
		applyStartEventsFn:                                           applyStartEvents,
		applyNonStartEventsPrepareBranchFn:                           applyNonStartEventsPrepareBranch,
		applyNonStartEventsPrepareMutableStateFn:                     applyNonStartEventsPrepareMutableState,
		applyNonStartEventsToCurrentBranchFn:                         applyNonStartEventsToCurrentBranch,
		applyNonStartEventsToNoneCurrentBranchFn:                     applyNonStartEventsToNoneCurrentBranch,
		applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn:    applyNonStartEventsToNoneCurrentBranchWithContinueAsNew,
		applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn: applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew,
		applyNonStartEventsMissingMutableStateFn:                     applyNonStartEventsMissingMutableState,
		applyNonStartEventsResetWorkflowFn:                           applyNonStartEventsResetWorkflow,
	}

	return replicator
}

func (r *historyReplicatorImpl) ApplyEvents(
	ctx ctx.Context,
	request *types.ReplicateEventsV2Request,
) (retError error) {

	startTime := time.Now()
	task, err := r.newReplicationTaskFn(
		r.clusterMetadata,
		r.historySerializer,
		startTime,
		r.logger,
		request,
	)
	if err != nil {
		return err
	}

	return r.applyEvents(ctx, task)
}

func (r *historyReplicatorImpl) applyEvents(
	ctx ctx.Context,
	task replicationTask,
) (retError error) {

	context, releaseFn, err := r.executionCache.GetOrCreateWorkflowExecution(
		ctx,
		task.getDomainID(),
		*task.getExecution(),
	)
	if err != nil {
		// for get workflow execution context, with valid run id
		// err will not be of type EntityNotExistsError
		return err
	}
	defer func() {
		if rec := recover(); rec != nil {
			releaseFn(errPanic)
			stack := make([]byte, 1<<14) // 16kb
			n := runtime.Stack(stack, false)
			stack = stack[:n]
			if retError == nil {
				retError = fmt.Errorf("panic: %v, stack: %s", rec, stack)
			} else {
				retError = fmt.Errorf("panic: %v, stack: %s, previous err: %w", rec, stack, retError)
			}
		} else {
			releaseFn(retError)
		}
	}()

	switch task.getFirstEvent().GetEventType() {
	case types.EventTypeWorkflowExecutionStarted:
		return r.applyStartEventsFn(ctx, context, releaseFn, task, r.domainCache,
			r.newMutableStateFn, r.newStateBuilderFn, r.transactionManager, r.logger, r.shard, r.clusterMetadata)

	default:
		// apply events, other than simple start workflow execution
		// the continue as new + start workflow execution combination will also be processed here
		mutableState, err := context.LoadWorkflowExecutionWithTaskVersion(ctx, task.getVersion())
		switch err.(type) {
		case nil:
			// Sanity check to make only 3DC mutable state here
			if mutableState.GetVersionHistories() == nil {
				return execution.ErrMissingVersionHistories
			}

			doContinue, branchIndex, err := r.applyNonStartEventsPrepareBranchFn(ctx, context, mutableState, task, r.newBranchManagerFn)
			if err != nil {
				return err
			} else if !doContinue {
				r.metricsClient.IncCounter(metrics.ReplicateHistoryEventsScope, metrics.DuplicateReplicationEventsCounter)
				return nil
			}

			mutableState, isRebuilt, err := r.applyNonStartEventsPrepareMutableStateFn(ctx, context, mutableState, branchIndex, task, r.newConflictResolverFn)
			if err != nil {
				return err
			}

			if mutableState.GetVersionHistories().GetCurrentVersionHistoryIndex() == branchIndex {
				return r.applyNonStartEventsToCurrentBranchFn(ctx, context, mutableState, isRebuilt, releaseFn, task,
					r.newStateBuilderFn, r.clusterMetadata, r.shard, r.logger, r.transactionManager)
			}
			// passed in r because there's a recursive call within applyNonStartEventsToNoneCurrentBranchWithContinueAsNew
			return r.applyNonStartEventsToNoneCurrentBranchFn(ctx, context, mutableState, branchIndex, releaseFn, task, r, r.logger)

		case *types.EntityNotExistsError:
			// mutable state not created, check if is workflow reset
			mutableState, err := r.applyNonStartEventsMissingMutableStateFn(ctx, context, task, r.newWorkflowResetterFn)
			if err != nil {
				return err
			}

			return r.applyNonStartEventsResetWorkflowFn(ctx, context, mutableState, task,
				r.newStateBuilderFn, r.transactionManager, r.clusterMetadata, r.logger, r.shard)

		default:
			// unable to get mutable state, return err so we can retry the task later
			return err
		}
	}
}

func applyStartEvents(
	ctx ctx.Context,
	context execution.Context,
	releaseFn execution.ReleaseFunc,
	task replicationTask,
	domainCache cache.DomainCache,
	newMutableState newMutableStateFn,
	newStateBuilder newStateBuilderFn,
	transactionManager transactionManager,
	logger log.Logger,
	shard shard.Context,
	clusterMetadata cluster.Metadata,
) (retError error) {

	domainEntry, err := domainCache.GetDomainByID(task.getDomainID())
	if err != nil {
		return err
	}
	requestID := uuid.New() // requestID used for start workflow execution request.  This is not on the history event.
	// since it's replicated from the other cluster, we don't care the active cluster policy, because the failover version will be updated after ApplyEvents
	mutableState := newMutableState(domainEntry, task.getLogger())
	stateBuilder := newStateBuilder(mutableState, task.getLogger())

	// use state builder for workflow mutable state mutation
	_, err = stateBuilder.ApplyEvents(
		task.getDomainID(),
		requestID,
		*task.getExecution(),
		task.getEvents(),
		task.getNewEvents(),
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to apply events when applyStartEvents",
			tag.Error(err),
		)
		return err
	}

	err = transactionManager.createWorkflow(
		ctx,
		task.getEventTime(),
		execution.NewWorkflow(
			ctx,
			clusterMetadata,
			context,
			mutableState,
			releaseFn,
			logger,
		),
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to create workflow when applyStartEvents",
			tag.Error(err),
		)
	} else {
		notify(task.getSourceCluster(), task.getEventTime(), logger, shard, clusterMetadata)
	}
	return err
}

func applyNonStartEventsPrepareBranch(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	task replicationTask,
	newBranchManager newBranchManagerFn,
) (bool, int, error) {

	incomingVersionHistory := task.getVersionHistory()
	branchManager := newBranchManager(context, mutableState, task.getLogger())
	doContinue, versionHistoryIndex, err := branchManager.prepareVersionHistory(
		ctx,
		incomingVersionHistory,
		task.getFirstEvent().ID,
		task.getFirstEvent().Version,
	)
	switch err.(type) {
	case nil:
		return doContinue, versionHistoryIndex, nil
	case *types.RetryTaskV2Error:
		// replication message can arrive out of order
		// do not log
		return false, 0, err
	default:
		task.getLogger().Error(
			"nDCHistoryReplicator unable to prepare version history when applyNonStartEventsPrepareBranch",
			tag.Error(err),
		)
		return false, 0, err
	}
}

func applyNonStartEventsPrepareMutableState(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	branchIndex int,
	task replicationTask,
	newConflictResolver newConflictResolverFn,
) (execution.MutableState, bool, error) {

	incomingVersion := task.getVersion()
	conflictResolver := newConflictResolver(context, mutableState, task.getLogger())
	mutableState, isRebuilt, err := conflictResolver.prepareMutableState(
		ctx,
		branchIndex,
		incomingVersion,
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to prepare mutable state when applyNonStartEventsPrepareMutableState",
			tag.Error(err),
		)
	}
	return mutableState, isRebuilt, err
}

func applyNonStartEventsToCurrentBranch(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	isRebuilt bool,
	releaseFn execution.ReleaseFunc,
	task replicationTask,
	newStateBuilder newStateBuilderFn,
	clusterMetadata cluster.Metadata,
	shard shard.Context,
	logger log.Logger,
	transactionManager transactionManager,
) error {

	requestID := uuid.New() // requestID used for start workflow execution request.  This is not on the history event.
	stateBuilder := newStateBuilder(mutableState, task.getLogger())
	newMutableState, err := stateBuilder.ApplyEvents(
		task.getDomainID(),
		requestID,
		*task.getExecution(),
		task.getEvents(),
		task.getNewEvents(),
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to apply events when applyNonStartEventsToCurrentBranch",
			tag.Error(err),
		)
		return err
	}

	targetWorkflow := execution.NewWorkflow(
		ctx,
		clusterMetadata,
		context,
		mutableState,
		releaseFn,
		logger,
	)

	var newWorkflow execution.Workflow
	if newMutableState != nil {
		newExecutionInfo := newMutableState.GetExecutionInfo()
		newContext := execution.NewContext(
			newExecutionInfo.DomainID,
			types.WorkflowExecution{
				WorkflowID: newExecutionInfo.WorkflowID,
				RunID:      newExecutionInfo.RunID,
			},
			shard,
			shard.GetExecutionManager(),
			logger,
		)

		newWorkflow = execution.NewWorkflow(
			ctx,
			clusterMetadata,
			newContext,
			newMutableState,
			execution.NoopReleaseFn,
			logger,
		)
	}

	err = transactionManager.updateWorkflow(
		ctx,
		task.getEventTime(),
		isRebuilt,
		targetWorkflow,
		newWorkflow,
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to update workflow when applyNonStartEventsToCurrentBranch",
			tag.Error(err),
		)
	} else {
		notify(task.getSourceCluster(), task.getEventTime(), logger, shard, clusterMetadata)
	}
	return err
}

func applyNonStartEventsToNoneCurrentBranch(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	branchIndex int,
	releaseFn execution.ReleaseFunc,
	task replicationTask,
	r *historyReplicatorImpl,
	logger log.Logger,
) error {

	if len(task.getNewEvents()) != 0 {
		return r.applyNonStartEventsToNoneCurrentBranchWithContinueAsNewFn(
			ctx,
			context,
			releaseFn,
			task,
			r,
		)
	}

	return r.applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNewFn(
		ctx,
		context,
		mutableState,
		branchIndex,
		releaseFn,
		task,
		r.transactionManager,
		r.clusterMetadata,
		r.shard,
		logger,
	)
}

func applyNonStartEventsToNoneCurrentBranchWithoutContinueAsNew(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	branchIndex int,
	releaseFn execution.ReleaseFunc,
	task replicationTask,
	transactionManager transactionManager,
	clusterMetadata cluster.Metadata,
	shard shard.Context,
	logger log.Logger,
) error {

	versionHistoryItem := persistence.NewVersionHistoryItem(
		task.getLastEvent().ID,
		task.getLastEvent().Version,
	)
	versionHistories := mutableState.GetVersionHistories()
	if versionHistories == nil {
		return execution.ErrMissingVersionHistories
	}
	versionHistory, err := versionHistories.GetVersionHistory(branchIndex)
	if err != nil {
		return err
	}
	if err = versionHistory.AddOrUpdateItem(versionHistoryItem); err != nil {
		return err
	}

	err = transactionManager.backfillWorkflow(
		ctx,
		task.getEventTime(),
		execution.NewWorkflow(
			ctx,
			clusterMetadata,
			context,
			mutableState,
			releaseFn,
			logger,
		),
		&persistence.WorkflowEvents{
			DomainID:    task.getDomainID(),
			WorkflowID:  task.getExecution().GetWorkflowID(),
			RunID:       task.getExecution().GetRunID(),
			BranchToken: versionHistory.GetBranchToken(),
			Events:      task.getEvents(),
		},
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to backfill workflow when applyNonStartEventsToNoneCurrentBranch",
			tag.Error(err),
		)
		return err
	}
	return nil
}

func applyNonStartEventsToNoneCurrentBranchWithContinueAsNew(
	ctx ctx.Context,
	context execution.Context,
	releaseFn execution.ReleaseFunc,
	task replicationTask,
	r *historyReplicatorImpl,
) error {

	// workflow backfill to non current branch with continue as new
	// first, release target workflow lock & create the new workflow as zombie
	// NOTE: need to release target workflow due to target workflow
	//  can potentially be the current workflow causing deadlock

	// 1. clear all in memory changes & release target workflow lock
	// 2. apply new workflow first
	// 3. apply target workflow

	// step 1
	context.Clear()
	releaseFn(nil)

	// step 2
	startTime := time.Now()
	task, newTask, err := task.splitTask(startTime)
	if err != nil {
		return err
	}
	if err := r.applyEvents(ctx, newTask); err != nil {
		newTask.getLogger().Error(
			"nDCHistoryReplicator unable to create new workflow when applyNonStartEventsToNoneCurrentBranchWithContinueAsNew",
			tag.Error(err),
		)
		return err
	}

	// step 3
	if err := r.applyEvents(ctx, task); err != nil {
		newTask.getLogger().Error(
			"nDCHistoryReplicator unable to create target workflow when applyNonStartEventsToNoneCurrentBranchWithContinueAsNew",
			tag.Error(err),
		)
		return err
	}
	return nil
}

func applyNonStartEventsMissingMutableState(
	ctx ctx.Context,
	newContext execution.Context,
	task replicationTask,
	newWorkflowResetter newWorkflowResetterFn,
) (execution.MutableState, error) {

	// for non reset workflow execution replication task, just do re-replication
	if !task.isWorkflowReset() {
		firstEvent := task.getFirstEvent()
		return nil, newNDCRetryTaskErrorWithHint(
			mutableStateMissingMessage,
			task.getDomainID(),
			task.getWorkflowID(),
			task.getRunID(),
			nil,
			nil,
			common.Int64Ptr(firstEvent.ID),
			common.Int64Ptr(firstEvent.Version),
		)
	}

	decisionTaskEvent := task.getFirstEvent()
	baseEventID := decisionTaskEvent.ID - 1
	baseRunID, newRunID, baseEventVersion, _ := task.getWorkflowResetMetadata()

	workflowResetter := newWorkflowResetter(
		task.getDomainID(),
		task.getWorkflowID(),
		baseRunID,
		newContext,
		newRunID,
		task.getLogger(),
	)

	resetMutableState, err := workflowResetter.ResetWorkflow(
		ctx,
		task.getEventTime(),
		baseEventID,
		baseEventVersion,
		task.getFirstEvent().ID,
		task.getVersion(),
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to reset workflow when applyNonStartEventsMissingMutableState",
			tag.Error(err),
		)
		return nil, err
	}
	return resetMutableState, nil
}

func applyNonStartEventsResetWorkflow(
	ctx ctx.Context,
	context execution.Context,
	mutableState execution.MutableState,
	task replicationTask,
	newStateBuilder newStateBuilderFn,
	transactionManager transactionManager,
	clusterMetadata cluster.Metadata,
	logger log.Logger,
	shard shard.Context,
) error {

	requestID := uuid.New() // requestID used for start workflow execution request.  This is not on the history event.
	stateBuilder := newStateBuilder(mutableState, task.getLogger())
	_, err := stateBuilder.ApplyEvents(
		task.getDomainID(),
		requestID,
		*task.getExecution(),
		task.getEvents(),
		task.getNewEvents(),
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to apply events when applyNonStartEventsResetWorkflow",
			tag.Error(err),
		)
		return err
	}

	targetWorkflow := execution.NewWorkflow(
		ctx,
		clusterMetadata,
		context,
		mutableState,
		execution.NoopReleaseFn,
		logger,
	)

	err = transactionManager.createWorkflow(
		ctx,
		task.getEventTime(),
		targetWorkflow,
	)
	if err != nil {
		task.getLogger().Error(
			"nDCHistoryReplicator unable to create workflow when applyNonStartEventsResetWorkflow",
			tag.Error(err),
		)
	} else {
		notify(task.getSourceCluster(), task.getEventTime(), logger, shard, clusterMetadata)
	}
	return err
}

func notify(
	clusterName string,
	now time.Time,
	logger log.Logger,
	shard shard.Context,
	clusterMetadata cluster.Metadata,
) {
	if clusterName == clusterMetadata.GetCurrentClusterName() {
		// this is a valid use case for testing, but not for production
		logger.Warn("nDCHistoryReplicator applying events generated by current cluster")
		return
	}
	now = now.Add(-shard.GetConfig().StandbyClusterDelay())
	shard.SetCurrentTime(clusterName, now)
	logger.Debugf("History replicator setting current time to: %v for clusterName %v", now, clusterName)
}

func newNDCRetryTaskErrorWithHint(
	message string,
	domainID string,
	workflowID string,
	runID string,
	startEventID *int64,
	startEventVersion *int64,
	endEventID *int64,
	endEventVersion *int64,
) error {

	return &types.RetryTaskV2Error{
		Message:           message,
		DomainID:          domainID,
		WorkflowID:        workflowID,
		RunID:             runID,
		StartEventID:      startEventID,
		StartEventVersion: startEventVersion,
		EndEventID:        endEventID,
		EndEventVersion:   endEventVersion,
	}
}
