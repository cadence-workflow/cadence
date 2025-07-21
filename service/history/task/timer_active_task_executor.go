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
	"fmt"
	"time"

	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/simulation"
	"github.com/uber/cadence/service/worker/archiver"
)

const (
	scanWorkflowTimeout = 30 * time.Second
)

var (
	normalDecisionTypeTag = metrics.DecisionTypeTag("normal")
	stickyDecisionTypeTag = metrics.DecisionTypeTag("sticky")
)

type (
	timerActiveTaskExecutor struct {
		*timerTaskExecutorBase
	}
)

// NewTimerActiveTaskExecutor creates a new task executor for active timer task
func NewTimerActiveTaskExecutor(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	logger log.Logger,
	metricsClient metrics.Client,
	config *config.Config,
) Executor {
	return &timerActiveTaskExecutor{
		timerTaskExecutorBase: newTimerTaskExecutorBase(
			shard,
			archiverClient,
			executionCache,
			logger,
			metricsClient,
			config,
		),
	}
}

func (t *timerActiveTaskExecutor) Execute(task Task) (metrics.Scope, error) {
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
	scope := getOrCreateDomainTaggedScope(t.shard, GetTimerTaskMetricScope(task.GetTaskType(), true), task.GetDomainID(), t.logger)
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
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeActivityRetryTimerTask(ctx, timerTask)
	case *persistence.WorkflowBackoffTimerTask:
		ctx, cancel := context.WithTimeout(t.ctx, taskDefaultTimeout)
		defer cancel()
		return scope, t.executeWorkflowBackoffTimerTask(ctx, timerTask)
	case *persistence.DeleteHistoryEventTask:
		ctx, cancel := context.WithTimeout(t.ctx, time.Duration(t.config.DeleteHistoryEventContextTimeout())*time.Second)
		defer cancel()
		return scope, t.executeDeleteHistoryEventTask(ctx, timerTask)
	default:
		return scope, errUnknownTimerTask
	}
}

func (t *timerActiveTaskExecutor) executeUserTimerTimeoutTask(
	ctx context.Context,
	task *persistence.UserTimerTask,
) (retError error) {
	t.logger.Debug("Processing user timer",
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.TaskID(task.TaskID),
	)
	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, task.EventID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	timerSequence := execution.NewTimerSequence(mutableState)
	referenceTime := t.shard.GetTimeSource().Now()
	resurrectionCheckMinDelay := t.config.ResurrectionCheckMinDelay(mutableState.GetDomainEntry().GetInfo().Name)
	updateMutableState := false
	debugLog := t.logger.Debug
	if t.config.EnableDebugMode && t.config.EnableTimerDebugLogByDomainID(task.DomainID) {
		debugLog = t.logger.Info
	}

	// initialized when a timer with delay >= resurrectionCheckMinDelay
	// is encountered, so that we don't need to scan history multiple times
	// where there're multiple timers with high delay
	var resurrectedTimer map[string]struct{}
	scanWorkflowCtx, cancel := context.WithTimeout(t.ctx, scanWorkflowTimeout)
	defer cancel()

	sortedUserTimers := timerSequence.LoadAndSortUserTimers()
	debugLog("Sorted user timers",
		tag.WorkflowDomainID(task.DomainID),
		tag.WorkflowID(task.WorkflowID),
		tag.WorkflowRunID(task.RunID),
		tag.Counter(len(sortedUserTimers)),
	)

Loop:
	for _, timerSequenceID := range sortedUserTimers {
		timerInfo, ok := mutableState.GetUserTimerInfoByEventID(timerSequenceID.EventID)
		if !ok {
			errString := fmt.Sprintf("failed to find in user timer event ID: %v", timerSequenceID.EventID)
			t.logger.Error(errString)
			return &types.InternalServiceError{Message: errString}
		}

		delay, expired := timerSequence.IsExpired(referenceTime, timerSequenceID)
		debugLog("Processing user timer sequence id",
			tag.WorkflowDomainID(task.DomainID),
			tag.WorkflowID(task.WorkflowID),
			tag.WorkflowRunID(task.RunID),
			tag.TaskID(task.TaskID),
			tag.WorkflowTimerID(timerInfo.TimerID),
			tag.WorkflowScheduleID(timerInfo.StartedID),
			tag.Dynamic("timer-sequence-id", timerSequenceID),
			tag.Dynamic("timer-info", timerInfo),
			tag.Dynamic("delay", delay),
			tag.Dynamic("expired", expired),
		)

		if !expired {
			// timer sequence IDs are sorted, once there is one timer
			// sequence ID not expired, all after that wil not expired
			break Loop
		}

		if delay >= resurrectionCheckMinDelay || resurrectedTimer != nil {
			if resurrectedTimer == nil {
				// overwrite the context here as scan history may take a long time to complete
				// ctx will also be used by other operations like updateWorkflow
				ctx = scanWorkflowCtx
				resurrectedTimer, err = execution.GetResurrectedTimers(ctx, t.shard, mutableState)
				if err != nil {
					t.logger.Error("Timer resurrection check failed", tag.Error(err))
					return err
				}
			}

			if _, ok := resurrectedTimer[timerInfo.TimerID]; ok {
				// found timer resurrection
				domainName := mutableState.GetDomainEntry().GetInfo().Name
				t.metricsClient.Scope(metrics.TimerQueueProcessorScope, metrics.DomainTag(domainName)).IncCounter(metrics.TimerResurrectionCounter)
				t.logger.Warn("Encounter resurrected timer, skip",
					tag.WorkflowDomainID(task.DomainID),
					tag.WorkflowID(task.WorkflowID),
					tag.WorkflowRunID(task.RunID),
					tag.TaskID(task.TaskID),
					tag.WorkflowTimerID(timerInfo.TimerID),
					tag.WorkflowScheduleID(timerInfo.StartedID), // timerStartedEvent is basically scheduled event
				)

				// remove resurrected timer from mutable state
				if err := mutableState.DeleteUserTimer(timerInfo.TimerID); err != nil {
					return err
				}
				updateMutableState = true
				continue Loop
			}
		}

		if _, err := mutableState.AddTimerFiredEvent(timerInfo.TimerID); err != nil {
			return err
		}
		updateMutableState = true

		debugLog("User timer fired",
			tag.WorkflowDomainID(task.DomainID),
			tag.WorkflowID(task.WorkflowID),
			tag.WorkflowRunID(task.RunID),
			tag.TaskID(task.TaskID),
			tag.WorkflowTimerID(timerInfo.TimerID),
			tag.WorkflowScheduleID(timerInfo.StartedID),
			tag.WorkflowNextEventID(mutableState.GetNextEventID()),
		)

	}

	if !updateMutableState {
		return nil
	}

	return t.updateWorkflowExecution(ctx, wfContext, mutableState, updateMutableState)
}

func (t *timerActiveTaskExecutor) executeActivityTimeoutTask(
	ctx context.Context,
	task *persistence.ActivityTimeoutTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if err != nil {
		return fmt.Errorf("unable to find domainID: %v, err: %v", task.DomainID, err)
	}

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, task.EventID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	wfType := mutableState.GetWorkflowType()
	if wfType == nil {
		return fmt.Errorf("unable to find workflow type, task %v", task)
	}

	timerSequence := execution.NewTimerSequence(mutableState)
	referenceTime := t.shard.GetTimeSource().Now()
	resurrectionCheckMinDelay := t.config.ResurrectionCheckMinDelay(mutableState.GetDomainEntry().GetInfo().Name)
	updateMutableState := false
	scheduleDecision := false

	// initialized when an activity timer with delay >= resurrectionCheckMinDelay
	// is encountered, so that we don't need to scan history multiple times
	// where there're multiple timers with high delay
	var resurrectedActivity map[int64]struct{}
	scanWorkflowCtx, cancel := context.WithTimeout(t.ctx, scanWorkflowTimeout)
	defer cancel()

	// need to clear activity heartbeat timer task mask for new activity timer task creation
	// NOTE: LastHeartbeatTimeoutVisibilityInSeconds is for deduping heartbeat timer creation as it's possible
	// one heartbeat task was persisted multiple times with different taskIDs due to the retry logic
	// for updating workflow execution. In that case, only one new heartbeat timeout task should be
	// created.
	isHeartBeatTask := task.TimeoutType == int(types.TimeoutTypeHeartbeat)
	activityInfo, ok := mutableState.GetActivityInfo(task.EventID)
	if isHeartBeatTask && ok && activityInfo.LastHeartbeatTimeoutVisibilityInSeconds <= task.VisibilityTimestamp.Unix() {
		activityInfo.TimerTaskStatus = activityInfo.TimerTaskStatus &^ execution.TimerTaskStatusCreatedHeartbeat
		if err := mutableState.UpdateActivity(activityInfo); err != nil {
			return err
		}
		updateMutableState = true
	}

Loop:
	for _, timerSequenceID := range timerSequence.LoadAndSortActivityTimers() {
		activityInfo, ok := mutableState.GetActivityInfo(timerSequenceID.EventID)
		if !ok || timerSequenceID.Attempt < activityInfo.Attempt {
			// handle 2 cases:
			// 1. !ok
			//  this case can happen since each activity can have 4 timers
			//  and one of those 4 timers may have fired in this loop
			// 2. timerSequenceID.attempt < activityInfo.Attempt
			//  retry could update activity attempt, should not timeouts new attempt
			// 3. it's a resurrected activity and has already been deleted in this loop
			continue Loop
		}

		delay, expired := timerSequence.IsExpired(referenceTime, timerSequenceID)
		if !expired {
			// timer sequence IDs are sorted, once there is one timer
			// sequence ID not expired, all after that wil not expired
			break Loop
		}

		if delay >= resurrectionCheckMinDelay || resurrectedActivity != nil {
			if resurrectedActivity == nil {
				// overwrite the context here as scan history may take a long time to complete
				// ctx will also be used by other operations like updateWorkflow
				ctx = scanWorkflowCtx
				resurrectedActivity, err = execution.GetResurrectedActivities(ctx, t.shard, mutableState)
				if err != nil {
					t.logger.Error("Activity resurrection check failed", tag.Error(err))
					return err
				}
			}

			if _, ok := resurrectedActivity[activityInfo.ScheduleID]; ok {
				// found activity resurrection
				domainName := mutableState.GetDomainEntry().GetInfo().Name
				t.metricsClient.Scope(metrics.TimerQueueProcessorScope, metrics.DomainTag(domainName)).IncCounter(metrics.ActivityResurrectionCounter)
				t.logger.Warn("Encounter resurrected activity, skip",
					tag.WorkflowDomainID(task.DomainID),
					tag.WorkflowID(task.WorkflowID),
					tag.WorkflowRunID(task.RunID),
					tag.TaskType(task.GetTaskType()),
					tag.TaskID(task.TaskID),
					tag.WorkflowActivityID(activityInfo.ActivityID),
					tag.WorkflowScheduleID(activityInfo.ScheduleID),
				)

				// remove resurrected activity from mutable state
				if err := mutableState.DeleteActivity(activityInfo.ScheduleID); err != nil {
					return err
				}
				updateMutableState = true
				continue Loop
			}
		}

		// check if it's possible that the timeout is due to activity task lost
		if timerSequenceID.TimerType == execution.TimerTypeScheduleToStart {
			domainName, err := t.shard.GetDomainCache().GetDomainName(mutableState.GetExecutionInfo().DomainID)
			if err == nil && activityInfo.ScheduleToStartTimeout >= int32(t.config.ActivityMaxScheduleToStartTimeoutForRetry(domainName).Seconds()) {
				// note that we ignore the race condition for the dynamic config value change here as it's only for metric and logging purpose.
				// theoratically the check only applies to activities with retry policy
				// however for activities without retry policy, we also want to check the potential task lost and emit the metric
				// so reuse the same config value as a threshold so that the metric only got emitted if the activity has been started after a long time.
				t.metricsClient.Scope(metrics.TimerActiveTaskActivityTimeoutScope, metrics.DomainTag(domainName)).IncCounter(metrics.ActivityLostCounter)
				t.logger.Warn("Potentially activity task lost",
					tag.WorkflowDomainName(domainName),
					tag.WorkflowID(task.WorkflowID),
					tag.WorkflowRunID(task.RunID),
					tag.WorkflowScheduleID(activityInfo.ScheduleID),
				)
			}
		}

		if ok, err := mutableState.RetryActivity(
			activityInfo,
			execution.TimerTypeToReason(timerSequenceID.TimerType),
			nil,
		); err != nil {
			return err
		} else if ok {
			updateMutableState = true
			continue Loop
		}

		t.emitTimeoutMetricScopeWithDomainTag(
			mutableState.GetExecutionInfo().DomainID,
			metrics.TimerActiveTaskActivityTimeoutScope,
			timerSequenceID.TimerType,
			metrics.WorkflowTypeTag(wfType.GetName()),
		)

		t.logger.Info("Activity timed out",
			tag.WorkflowDomainName(domainName),
			tag.WorkflowDomainID(task.GetDomainID()),
			tag.WorkflowID(task.GetWorkflowID()),
			tag.WorkflowRunID(task.GetRunID()),
			tag.ScheduleAttempt(task.Attempt),
			tag.FailoverVersion(task.GetVersion()),
			tag.ActivityTimeoutType(shared.TimeoutType(timerSequenceID.TimerType)),
		)

		if _, err := mutableState.AddActivityTaskTimedOutEvent(
			activityInfo.ScheduleID,
			activityInfo.StartedID,
			execution.TimerTypeToInternal(timerSequenceID.TimerType),
			activityInfo.Details,
		); err != nil {
			return err
		}
		updateMutableState = true
		scheduleDecision = true
	}

	if !updateMutableState {
		return nil
	}
	return t.updateWorkflowExecution(ctx, wfContext, mutableState, scheduleDecision)
}

func (t *timerActiveTaskExecutor) executeDecisionTimeoutTask(
	ctx context.Context,
	task *persistence.DecisionTimeoutTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if err != nil {
		return fmt.Errorf("unable to find domainID: %v, err: %v", task.DomainID, err)
	}

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, task.EventID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	wfType := mutableState.GetWorkflowType()
	if wfType == nil {
		return fmt.Errorf("unable to find workflow type, task %v", task)
	}

	scheduleID := task.EventID
	decision, ok := mutableState.GetDecisionInfo(scheduleID)
	if !ok {
		t.logger.Debug("Potentially duplicate", tag.TaskID(task.TaskID), tag.WorkflowScheduleID(scheduleID), tag.TaskType(persistence.TaskTypeDecisionTimeout))
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, decision.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	if decision.Attempt != task.ScheduleAttempt {
		return nil
	}

	scheduleDecision := false
	isStickyDecision := mutableState.GetExecutionInfo().StickyTaskList != ""
	decisionTypeTag := normalDecisionTypeTag
	if isStickyDecision {
		decisionTypeTag = stickyDecisionTypeTag
	}
	tags := []metrics.Tag{metrics.WorkflowTypeTag(wfType.GetName()), decisionTypeTag}
	switch execution.TimerTypeFromInternal(types.TimeoutType(task.TimeoutType)) {
	case execution.TimerTypeStartToClose:
		t.emitTimeoutMetricScopeWithDomainTag(
			mutableState.GetExecutionInfo().DomainID,
			metrics.TimerActiveTaskDecisionTimeoutScope,
			execution.TimerTypeStartToClose,
			tags...,
		)
		if _, err := mutableState.AddDecisionTaskTimedOutEvent(
			decision.ScheduleID,
			decision.StartedID,
		); err != nil {
			return err
		}
		scheduleDecision = true

	case execution.TimerTypeScheduleToStart:
		if decision.StartedID != constants.EmptyEventID {
			// decision has already started
			return nil
		}

		if !isStickyDecision {
			t.logger.Warn("Potential lost normal decision task",
				tag.WorkflowDomainName(domainName),
				tag.WorkflowDomainID(task.GetDomainID()),
				tag.WorkflowID(task.GetWorkflowID()),
				tag.WorkflowRunID(task.GetRunID()),
				tag.WorkflowScheduleID(scheduleID),
				tag.ScheduleAttempt(task.ScheduleAttempt),
				tag.FailoverVersion(task.GetVersion()),
			)
		}

		t.emitTimeoutMetricScopeWithDomainTag(
			mutableState.GetExecutionInfo().DomainID,
			metrics.TimerActiveTaskDecisionTimeoutScope,
			execution.TimerTypeScheduleToStart,
			tags...,
		)
		_, err := mutableState.AddDecisionTaskScheduleToStartTimeoutEvent(scheduleID)
		if err != nil {
			return err
		}
		scheduleDecision = true
	}

	return t.updateWorkflowExecution(ctx, wfContext, mutableState, scheduleDecision)
}

func (t *timerActiveTaskExecutor) executeWorkflowBackoffTimerTask(
	ctx context.Context,
	task *persistence.WorkflowBackoffTimerTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	if task.TimeoutType == persistence.WorkflowBackoffTimeoutTypeRetry {
		t.metricsClient.IncCounter(metrics.TimerActiveTaskWorkflowBackoffTimerScope, metrics.WorkflowRetryBackoffTimerCount)
	} else {
		t.metricsClient.IncCounter(metrics.TimerActiveTaskWorkflowBackoffTimerScope, metrics.WorkflowCronBackoffTimerCount)
	}

	if mutableState.HasProcessedOrPendingDecision() {
		// already has decision task
		return nil
	}

	// schedule first decision task
	return t.updateWorkflowExecution(ctx, wfContext, mutableState, true)
}

func (t *timerActiveTaskExecutor) executeActivityRetryTimerTask(
	ctx context.Context,
	task *persistence.ActivityRetryTimerTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, task.EventID)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	// generate activity task
	scheduledID := task.EventID
	activityInfo, ok := mutableState.GetActivityInfo(scheduledID)
	if !ok || task.Attempt < int64(activityInfo.Attempt) || activityInfo.StartedID != constants.EmptyEventID {
		if ok {
			t.logger.Info("Duplicate activity retry timer task",
				tag.WorkflowID(mutableState.GetExecutionInfo().WorkflowID),
				tag.WorkflowRunID(mutableState.GetExecutionInfo().RunID),
				tag.WorkflowDomainID(mutableState.GetExecutionInfo().DomainID),
				tag.WorkflowScheduleID(activityInfo.ScheduleID),
				tag.Attempt(activityInfo.Attempt),
				tag.FailoverVersion(activityInfo.Version),
				tag.TimerTaskStatus(activityInfo.TimerTaskStatus),
				tag.ScheduleAttempt(task.Attempt))
		}
		return nil
	}
	ok, err = verifyTaskVersion(t.shard, t.logger, task.DomainID, activityInfo.Version, task.Version, task)
	if err != nil || !ok {
		return err
	}

	domainID := task.DomainID
	targetDomainID := domainID
	if activityInfo.DomainID != "" {
		targetDomainID = activityInfo.DomainID
	} else {
		// TODO remove this block after Mar, 1th, 2020
		//  previously, DomainID in activity info is not used, so need to get
		//  schedule event from DB checking whether activity to be scheduled
		//  belongs to this domain
		scheduledEvent, err := mutableState.GetActivityScheduledEvent(ctx, scheduledID)
		if err != nil {
			return err
		}
		if scheduledEvent.ActivityTaskScheduledEventAttributes.GetDomain() != "" {
			domainEntry, err := t.shard.GetDomainCache().GetDomain(scheduledEvent.ActivityTaskScheduledEventAttributes.GetDomain())
			if err != nil {
				return &types.InternalServiceError{Message: "unable to re-schedule activity across domain."}
			}
			targetDomainID = domainEntry.GetInfo().ID
		}
	}

	execution := types.WorkflowExecution{
		WorkflowID: task.WorkflowID,
		RunID:      task.RunID}
	taskList := &types.TaskList{
		Name: activityInfo.TaskList,
	}
	scheduleToStartTimeout := activityInfo.ScheduleToStartTimeout

	release(nil) // release earlier as we don't need the lock anymore

	shouldPush, err := shouldPushToMatching(ctx, t.shard, task)
	if err != nil {
		return err
	}
	if !shouldPush {
		return nil
	}

	_, err = t.shard.GetService().GetMatchingClient().AddActivityTask(ctx, &types.AddActivityTaskRequest{
		DomainUUID:                    targetDomainID,
		SourceDomainUUID:              domainID,
		Execution:                     &execution,
		TaskList:                      taskList,
		ScheduleID:                    scheduledID,
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(scheduleToStartTimeout),
		PartitionConfig:               mutableState.GetExecutionInfo().PartitionConfig,
	})
	return err
}

func (t *timerActiveTaskExecutor) executeWorkflowTimeoutTask(
	ctx context.Context,
	task *persistence.WorkflowTimeoutTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errWorkflowBusy
		}
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	startVersion, err := mutableState.GetStartVersion()
	if err != nil {
		return err
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.DomainID, startVersion, task.Version, task)
	if err != nil || !ok {
		return err
	}

	eventBatchFirstEventID := mutableState.GetNextEventID()

	timeoutReason := execution.TimerTypeToReason(execution.TimerTypeStartToClose)
	backoffInterval := mutableState.GetRetryBackoffDuration(timeoutReason)
	continueAsNewInitiator := types.ContinueAsNewInitiatorRetryPolicy
	if backoffInterval == backoff.NoBackoff {
		// check if a cron backoff is needed
		backoffInterval, err = mutableState.GetCronBackoffDuration(ctx)
		if err != nil {
			return err
		}
		continueAsNewInitiator = types.ContinueAsNewInitiatorCronSchedule
	}
	// ignore event id
	isCanceled, _ := mutableState.IsCancelRequested()
	if isCanceled || backoffInterval == backoff.NoBackoff {
		if err := timeoutWorkflow(mutableState, eventBatchFirstEventID); err != nil {
			return err
		}

		// We apply the update to execution using optimistic concurrency.  If it fails due to a conflict than reload
		// the history and try the operation again.
		return t.updateWorkflowExecution(ctx, wfContext, mutableState, false)
	}

	// workflow timeout, but a retry or cron is needed, so we do continue as new to retry or cron
	startEvent, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		return err
	}

	startAttributes := startEvent.WorkflowExecutionStartedEventAttributes
	continueAsNewAttributes := &types.ContinueAsNewWorkflowExecutionDecisionAttributes{
		WorkflowType:                        startAttributes.WorkflowType,
		TaskList:                            startAttributes.TaskList,
		Input:                               startAttributes.Input,
		ExecutionStartToCloseTimeoutSeconds: startAttributes.ExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      startAttributes.TaskStartToCloseTimeoutSeconds,
		BackoffStartIntervalInSeconds:       common.Int32Ptr(int32(backoffInterval.Seconds())),
		RetryPolicy:                         startAttributes.RetryPolicy,
		Initiator:                           continueAsNewInitiator.Ptr(),
		FailureReason:                       common.StringPtr(timeoutReason),
		CronSchedule:                        mutableState.GetExecutionInfo().CronSchedule,
		Header:                              startAttributes.Header,
		Memo:                                startAttributes.Memo,
		SearchAttributes:                    startAttributes.SearchAttributes,
		JitterStartSeconds:                  startAttributes.JitterStartSeconds,
	}
	newMutableState, err := retryWorkflow(
		ctx,
		mutableState,
		eventBatchFirstEventID,
		startAttributes.GetParentWorkflowDomain(),
		continueAsNewAttributes,
	)
	if err != nil {
		return err
	}

	newExecutionInfo := newMutableState.GetExecutionInfo()
	return wfContext.UpdateWorkflowExecutionWithNewAsActive(
		ctx,
		t.shard.GetTimeSource().Now(),
		execution.NewContext(
			newExecutionInfo.DomainID,
			types.WorkflowExecution{
				WorkflowID: newExecutionInfo.WorkflowID,
				RunID:      newExecutionInfo.RunID,
			},
			t.shard,
			t.shard.GetExecutionManager(),
			t.logger,
		),
		newMutableState,
	)
}

func (t *timerActiveTaskExecutor) updateWorkflowExecution(
	ctx context.Context,
	wfContext execution.Context,
	mutableState execution.MutableState,
	scheduleNewDecision bool,
) error {

	var err error
	if scheduleNewDecision {
		// Schedule a new decision.
		err = execution.ScheduleDecision(mutableState)
		if err != nil {
			return err
		}
	}

	now := t.shard.GetTimeSource().Now()
	t.logger.Debugf("timerActiveTaskExecutor.updateWorkflowExecution calling UpdateWorkflowExecutionAsActive for wfID %s",
		mutableState.GetExecutionInfo().WorkflowID,
	)
	err = wfContext.UpdateWorkflowExecutionAsActive(ctx, now)
	if err != nil {
		// if is shard ownership error, the shard context will stop the entire history engine
		// we don't need to explicitly stop the queue processor here
		return err
	}

	return nil
}

func (t *timerActiveTaskExecutor) emitTimeoutMetricScopeWithDomainTag(
	domainID string,
	scope int,
	timerType execution.TimerType,
	tags ...metrics.Tag,
) {
	domainTag, err := getDomainTagByID(t.shard.GetDomainCache(), domainID)
	if err != nil {
		return
	}
	tags = append(tags, domainTag)

	metricsScope := t.metricsClient.Scope(scope, tags...)
	switch timerType {
	case execution.TimerTypeScheduleToStart:
		metricsScope.IncCounter(metrics.ScheduleToStartTimeoutCounter)
	case execution.TimerTypeScheduleToClose:
		metricsScope.IncCounter(metrics.ScheduleToCloseTimeoutCounter)
	case execution.TimerTypeStartToClose:
		metricsScope.IncCounter(metrics.StartToCloseTimeoutCounter)
	case execution.TimerTypeHeartbeat:
		metricsScope.IncCounter(metrics.HeartbeatTimeoutCounter)
	}
}
