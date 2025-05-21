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

package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/simulation"
	"github.com/uber/cadence/service/worker/archiver"
)

var (
	errUnexpectedTask   = errors.New("unexpected task")
	errUnknownTimerTask = errors.New("unknown timer task")
)

type (
	timerStandbyTaskExecutor struct {
		*timerTaskExecutorBase

		clusterName     string
		historyResender ndc.HistoryResender
	}
)

// NewTimerStandbyTaskExecutor creates a new task executor for standby timer task
func NewTimerStandbyTaskExecutor(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	historyResender ndc.HistoryResender,
	logger log.Logger,
	metricsClient metrics.Client,
	clusterName string,
	config *config.Config,
) Executor {
	return &timerStandbyTaskExecutor{
		timerTaskExecutorBase: newTimerTaskExecutorBase(
			shard,
			archiverClient,
			executionCache,
			logger,
			metricsClient,
			config,
		),
		clusterName:     clusterName,
		historyResender: historyResender,
	}
}

func (t *timerStandbyTaskExecutor) Execute(task Task) (metrics.Scope, error) {
	simulation.LogEvents(simulation.E{
		EventName:  simulation.EventNameExecuteHistoryTask,
		Host:       t.shard.GetConfig().HostName,
		ShardID:    t.shard.GetShardID(),
		DomainID:   task.GetDomainID(),
		WorkflowID: task.GetWorkflowID(),
		RunID:      task.GetRunID(),
		Payload: map[string]any{
			"task_category": persistence.HistoryTaskCategoryTimer.Name(),
			"task_type":     task.GetTaskType(),
			"task_key":      task.GetTaskKey(),
		},
	})
	scope := getOrCreateDomainTaggedScope(t.shard, GetTimerTaskMetricScope(task.GetTaskType(), false), task.GetDomainID(), t.logger)
	switch timerTask := task.GetInfo().(type) {
	case *persistence.UserTimerTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeUserTimerTimeoutTask(ctx, timerTask)
	case *persistence.ActivityTimeoutTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeActivityTimeoutTask(ctx, timerTask)
	case *persistence.DecisionTimeoutTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeDecisionTimeoutTask(ctx, timerTask)
	case *persistence.WorkflowTimeoutTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeWorkflowTimeoutTask(ctx, timerTask)
	case *persistence.ActivityRetryTimerTask:
		// retry backoff timer should not get created on passive cluster
		// TODO: add error logs
		return scope, nil
	case *persistence.WorkflowBackoffTimerTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeWorkflowBackoffTimerTask(ctx, timerTask)
	case *persistence.DeleteHistoryEventTask:
		// special timeout for delete history event
		deleteHistoryEventContext, deleteHistoryEventCancel := context.WithTimeout(t.ctx, time.Duration(t.config.DeleteHistoryEventContextTimeout())*time.Second)
		defer deleteHistoryEventCancel()
		return scope, t.executeDeleteHistoryEventTask(deleteHistoryEventContext, timerTask)
	default:
		return scope, errUnknownTimerTask
	}
}

func (t *timerStandbyTaskExecutor) executeUserTimerTimeoutTask(
	ctx context.Context,
	timerTask *persistence.UserTimerTask,
) error {

	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		timerSequence := execution.NewTimerSequence(mutableState)

	Loop:
		for _, timerSequenceID := range timerSequence.LoadAndSortUserTimers() {
			_, ok := mutableState.GetUserTimerInfoByEventID(timerSequenceID.EventID)
			if !ok {
				errString := fmt.Sprintf("failed to find in user timer event ID: %v", timerSequenceID.EventID)
				t.logger.Error(errString)
				return nil, &types.InternalServiceError{Message: errString}
			}

			if _, isExpired := timerSequence.IsExpired(
				timerTask.VisibilityTimestamp,
				timerSequenceID,
			); isExpired {
				return getHistoryResendInfo(mutableState)
			}
			// since the user timer are already sorted, so if there is one timer which will not expired
			// all user timer after this timer will not expired
			break Loop //nolint:staticcheck
		}
		// if there is no user timer expired, then we are good
		return nil, nil
	}

	return t.processTimer(
		ctx,
		timerTask,
		timerTask.EventID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			timerTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *timerStandbyTaskExecutor) executeActivityTimeoutTask(
	ctx context.Context,
	timerTask *persistence.ActivityTimeoutTask,
) error {

	// activity heartbeat timer task is a special snowflake.
	// normal activity timer task on the passive side will be generated by events related to activity in history replicator,
	// and the standby timer processor will only need to verify whether the timer task can be safely throw away.
	//
	// activity heartbeat timer task cannot be handled in the way mentioned above.
	// the reason is, there is no event driving the creation of new activity heartbeat timer.
	// although there will be an task syncing activity from remote, the task is not an event,
	// and cannot attempt to recreate a new activity timer task.
	//
	// the overall solution is to attempt to generate a new activity timer task whenever the
	// task passed in is safe to be throw away.

	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		timerSequence := execution.NewTimerSequence(mutableState)
		updateMutableState := false

	Loop:
		for _, timerSequenceID := range timerSequence.LoadAndSortActivityTimers() {
			_, ok := mutableState.GetActivityInfo(timerSequenceID.EventID)
			if !ok {
				errString := fmt.Sprintf("failed to find in memory activity timer: %v", timerSequenceID.EventID)
				t.logger.Error(errString)
				return nil, &types.InternalServiceError{Message: errString}
			}

			if _, isExpired := timerSequence.IsExpired(
				timerTask.VisibilityTimestamp,
				timerSequenceID,
			); isExpired {
				return getHistoryResendInfo(mutableState)
			}
			// since the activity timer are already sorted, so if there is one timer which will not expired
			// all activity timer after this timer will not expired
			break Loop //nolint:staticcheck
		}

		// for reason to update mutable state & generate a new activity task,
		// see comments at the beginning of this function.
		// NOTE: this is the only place in the standby logic where mutable state can be updated

		// need to clear the activity heartbeat timer task marks
		lastWriteVersion, err := mutableState.GetLastWriteVersion()
		if err != nil {
			return nil, err
		}

		// NOTE: LastHeartbeatTimeoutVisibilityInSeconds is for deduping heartbeat timer creation as it's possible
		// one heartbeat task was persisted multiple times with different taskIDs due to the retry logic
		// for updating workflow execution. In that case, only one new heartbeat timeout task should be
		// created.
		isHeartBeatTask := timerTask.TimeoutType == int(types.TimeoutTypeHeartbeat)
		activityInfo, ok := mutableState.GetActivityInfo(timerTask.EventID)
		if isHeartBeatTask && ok && activityInfo.LastHeartbeatTimeoutVisibilityInSeconds <= timerTask.VisibilityTimestamp.Unix() {
			activityInfo.TimerTaskStatus = activityInfo.TimerTaskStatus &^ execution.TimerTaskStatusCreatedHeartbeat
			if err := mutableState.UpdateActivity(activityInfo); err != nil {
				return nil, err
			}
			updateMutableState = true
		}

		// passive logic need to explicitly call create timer
		modified, err := timerSequence.CreateNextActivityTimer()
		if err != nil {
			return nil, err
		}
		updateMutableState = updateMutableState || modified

		if !updateMutableState {
			return nil, nil
		}

		now := t.getStandbyClusterTime()
		// we need to handcraft some of the variables
		// since the job being done here is update the activity and possibly write a timer task to DB
		// also need to reset the current version.
		t.logger.Debugf("executeActivityTimeoutTask calling UpdateCurrentVersion for domain %s, wfID %v, lastWriteVersion %v",
			timerTask.DomainID, timerTask.WorkflowID, lastWriteVersion)
		if err := mutableState.UpdateCurrentVersion(lastWriteVersion, true); err != nil {
			return nil, err
		}

		err = wfContext.UpdateWorkflowExecutionAsPassive(ctx, now)
		return nil, err
	}

	return t.processTimer(
		ctx,
		timerTask,
		timerTask.EventID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			timerTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *timerStandbyTaskExecutor) executeDecisionTimeoutTask(
	ctx context.Context,
	timerTask *persistence.DecisionTimeoutTask,
) error {

	// decision schedule to start timer won't be generated for sticky decision,
	// since sticky is cleared when applying events on passive.
	// for normal decision, we don't know if a schedule to start timeout timer
	// is generated or not since it's based on a dynamicconfig. On passive cluster,
	// a timer task will be generated based on passive cluster's config, however, it
	// may not match the active cluster.
	// so we simply ignore the schedule to start timer here as the decision task will be
	// pushed to matching without any timeout if's not started, and the workflow
	// can continue execution after failover.
	if timerTask.TimeoutType == int(types.TimeoutTypeScheduleToStart) {
		return nil
	}

	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		decision, isPending := mutableState.GetDecisionInfo(timerTask.EventID)
		if !isPending {
			return nil, nil
		}

		ok, err := verifyTaskVersion(t.shard, t.logger, timerTask.DomainID, decision.Version, timerTask.Version, timerTask)
		if err != nil || !ok {
			return nil, err
		}

		return getHistoryResendInfo(mutableState)
	}

	return t.processTimer(
		ctx,
		timerTask,
		timerTask.EventID,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			timerTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *timerStandbyTaskExecutor) executeWorkflowBackoffTimerTask(
	ctx context.Context,
	timerTask *persistence.WorkflowBackoffTimerTask,
) error {

	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		if mutableState.HasProcessedOrPendingDecision() {
			// if there is one decision already been processed
			// or has pending decision, meaning workflow has already running
			return nil, nil
		}

		// Note: do not need to verify task version here
		// logic can only go here if mutable state build's next event ID is 2
		// meaning history only contains workflow started event.
		// we can do the checking of task version vs workflow started version
		// however, workflow started version is immutable

		// active cluster will add first decision task after backoff timeout.
		// standby cluster should just call ack manager to retry this task
		// since we are stilling waiting for the first DecisionScheduledEvent to be replicated from active side.

		return getHistoryResendInfo(mutableState)
	}

	return t.processTimer(
		ctx,
		timerTask,
		0,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			timerTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *timerStandbyTaskExecutor) executeWorkflowTimeoutTask(
	ctx context.Context,
	timerTask *persistence.WorkflowTimeoutTask,
) error {

	actionFn := func(ctx context.Context, wfContext execution.Context, mutableState execution.MutableState) (interface{}, error) {

		// we do not need to notify new timer to base, since if there is no new event being replicated
		// checking again if the timer can be completed is meaningless

		startVersion, err := mutableState.GetStartVersion()
		if err != nil {
			return nil, err
		}
		ok, err := verifyTaskVersion(t.shard, t.logger, timerTask.DomainID, startVersion, timerTask.Version, timerTask)
		if err != nil || !ok {
			return nil, err
		}

		return getHistoryResendInfo(mutableState)
	}

	return t.processTimer(
		ctx,
		timerTask,
		0,
		actionFn,
		getStandbyPostActionFn(
			t.logger,
			timerTask,
			t.getCurrentTime,
			t.config.StandbyTaskMissingEventsResendDelay(),
			t.config.StandbyTaskMissingEventsDiscardDelay(),
			t.fetchHistoryFromRemote,
			standbyTaskPostActionTaskDiscarded,
		),
	)
}

func (t *timerStandbyTaskExecutor) getStandbyClusterTime() time.Time {
	// time of remote cluster in the shard is delayed by "StandbyClusterDelay"
	// so to get the current accurate remote cluster time, need to add it back
	return t.shard.GetCurrentTime(t.clusterName).Add(t.shard.GetConfig().StandbyClusterDelay())
}

func (t *timerStandbyTaskExecutor) processTimer(
	ctx context.Context,
	timerTask persistence.Task,
	eventID int64,
	actionFn standbyActionFn,
	postActionFn standbyPostActionFn,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		timerTask.GetDomainID(),
		getWorkflowExecution(timerTask),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() {
		if isRedispatchErr(retError) {
			release(nil)
		} else {
			release(retError)
		}
	}()

	mutableState, err := loadMutableState(ctx, wfContext, timerTask, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, eventID)
	if err != nil {
		return err
	}
	if mutableState == nil {
		return nil
	}

	if !mutableState.IsWorkflowExecutionRunning() {
		// TODO: Check if workflow timeout timer comes to this point and then discarded.
		// workflow already finished, no need to process the timer
		return nil
	}

	historyResendInfo, err := actionFn(ctx, wfContext, mutableState)
	if err != nil {
		if t.logger.DebugOn() {
			t.logger.Debug("processTimer got error from actionFn",
				tag.Error(err),
				tag.WorkflowID(timerTask.GetWorkflowID()),
				tag.WorkflowRunID(timerTask.GetRunID()),
				tag.WorkflowDomainID(timerTask.GetDomainID()),
				tag.TaskID(timerTask.GetTaskID()),
				tag.TaskType(int(timerTask.GetTaskType())),
				tag.Timestamp(timerTask.GetVisibilityTimestamp()),
			)
		}
		return err
	}

	if t.logger.DebugOn() {
		t.logger.Debug("processTimer got historyResendInfo from actionFn",
			tag.WorkflowID(timerTask.GetWorkflowID()),
			tag.WorkflowRunID(timerTask.GetRunID()),
			tag.WorkflowDomainID(timerTask.GetDomainID()),
			tag.TaskID(timerTask.GetTaskID()),
			tag.TaskType(int(timerTask.GetTaskType())),
			tag.Timestamp(timerTask.GetVisibilityTimestamp()),
		)
	}

	release(nil)
	return postActionFn(ctx, timerTask, historyResendInfo, t.logger)
}

func (t *timerStandbyTaskExecutor) fetchHistoryFromRemote(
	_ context.Context,
	taskInfo persistence.Task,
	postActionInfo interface{},
	_ log.Logger,
) error {

	if postActionInfo == nil {
		return nil
	}

	resendInfo := postActionInfo.(*historyResendInfo)

	t.metricsClient.IncCounter(metrics.HistoryRereplicationByTimerTaskScope, metrics.CadenceClientRequests)
	stopwatch := t.metricsClient.StartTimer(metrics.HistoryRereplicationByTimerTaskScope, metrics.CadenceClientLatency)
	defer stopwatch.Stop()

	var err error
	if resendInfo.lastEventID != nil && resendInfo.lastEventVersion != nil {
		// note history resender doesn't take in a context parameter, there's a separate dynamicconfig for
		// controlling the timeout for resending history.
		err = t.historyResender.SendSingleWorkflowHistory(
			t.clusterName,
			taskInfo.GetDomainID(),
			taskInfo.GetWorkflowID(),
			taskInfo.GetRunID(),
			resendInfo.lastEventID,
			resendInfo.lastEventVersion,
			nil,
			nil,
		)
	} else {
		err = &types.InternalServiceError{
			Message: fmt.Sprintf("incomplete historyResendInfo: %v", resendInfo),
		}
	}

	if err != nil {
		t.logger.Error("Error re-replicating history from remote.",
			tag.ShardID(t.shard.GetShardID()),
			tag.WorkflowDomainID(taskInfo.GetDomainID()),
			tag.WorkflowID(taskInfo.GetWorkflowID()),
			tag.WorkflowRunID(taskInfo.GetRunID()),
			tag.SourceCluster(t.clusterName),
			tag.Error(err),
		)
	} else if t.logger.DebugOn() {
		t.logger.Debug("Successfully re-replicated history from remote.",
			tag.WorkflowID(taskInfo.GetWorkflowID()),
			tag.WorkflowRunID(taskInfo.GetRunID()),
			tag.WorkflowDomainID(taskInfo.GetDomainID()),
			tag.TaskID(taskInfo.GetTaskID()),
			tag.TaskType(int(taskInfo.GetTaskType())),
			tag.SourceCluster(t.clusterName),
		)
	}

	// return error so task processing logic will retry
	return &redispatchError{Reason: "fetchHistoryFromRemote"}
}

func (t *timerStandbyTaskExecutor) getCurrentTime() time.Time {
	return t.shard.GetCurrentTime(t.clusterName)
}
