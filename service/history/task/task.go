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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	ctask "github.com/uber/cadence/common/task"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

// redispatchError is the error indicating that the timer / transfer task should be redispatched and retried.
type redispatchError struct {
	Reason string
}

// Error explains why this task should be redispatched
func (r *redispatchError) Error() string {
	return fmt.Sprintf("Redispatch reason: %q", r.Reason)
}

func isRedispatchErr(err error) bool {
	var redispatchErr *redispatchError
	return errors.As(err, &redispatchErr)
}

const (
	activeTaskRedispatchInitialInterval  = 5 * time.Second
	standbyTaskRedispatchInitialInterval = 30 * time.Second
	redispatchBackoffCoefficient         = 1.05
	redispatchMaxBackoffInternval        = 2 * time.Minute
	redispatchFailureBackoffInterval     = 2 * time.Second
)

var (
	// ErrTaskDiscarded is the error indicating that the timer / transfer task is pending for too long and discarded.
	ErrTaskDiscarded = errors.New("passive task pending for too long")
	// ErrTaskPendingActive is the error indicating that the task should be re-dispatched
	ErrTaskPendingActive = errors.New("redispatch the task while the domain is pending-active")

	activeTaskRedispatchPolicy  = createTaskRedispatchPolicy(activeTaskRedispatchInitialInterval)
	standbyTaskRedispatchPolicy = createTaskRedispatchPolicy(standbyTaskRedispatchInitialInterval)
)

type (
	taskImpl struct {
		sync.Mutex
		persistence.Task

		shard                    shard.Context
		state                    ctask.State
		priority                 int
		attempt                  int
		timeSource               clock.TimeSource
		initialSubmitTime        time.Time
		logger                   log.Logger
		eventLogger              eventLogger
		scope                    metrics.Scope // initialized when processing task to make the initialization parallel
		taskExecutor             Executor
		taskProcessor            Processor
		redispatcher             Redispatcher
		rescheduler              Rescheduler
		criticalRetryCount       dynamicproperties.IntPropertyFn
		isPreviousExecutorActive bool

		// TODO: following three fields should be removed after new task lifecycle is implemented
		taskFilter        Filter
		queueType         QueueType
		shouldProcessTask bool
	}
)

func NewHistoryTask(
	shard shard.Context,
	taskInfo persistence.Task,
	queueType QueueType,
	logger log.Logger,
	taskFilter Filter,
	taskExecutor Executor,
	taskProcessor Processor,
	redispatcher Redispatcher,
	criticalRetryCount dynamicproperties.IntPropertyFn,
) Task {
	timeSource := shard.GetTimeSource()
	var eventLogger eventLogger
	if shard.GetConfig().EnableDebugMode &&
		(queueType == QueueTypeActiveTimer || queueType == QueueTypeActiveTransfer) &&
		shard.GetConfig().EnableTaskInfoLogByDomainID(taskInfo.GetDomainID()) {
		eventLogger = newEventLogger(logger, timeSource, defaultTaskEventLoggerSize)
		eventLogger.AddEvent("Created task")
	}

	return &taskImpl{
		Task:               taskInfo,
		shard:              shard,
		state:              ctask.TaskStatePending,
		priority:           noPriority,
		queueType:          queueType,
		scope:              metrics.NoopScope,
		logger:             logger,
		eventLogger:        eventLogger,
		attempt:            0,
		initialSubmitTime:  timeSource.Now(),
		timeSource:         timeSource,
		criticalRetryCount: criticalRetryCount,
		redispatcher:       redispatcher,
		taskFilter:         taskFilter,
		taskExecutor:       taskExecutor,
		taskProcessor:      taskProcessor,
	}
}

func NewHistoryTaskV2(
	shard shard.Context,
	taskInfo persistence.Task,
	queueType QueueType,
	logger log.Logger,
	taskExecutor Executor,
	taskProcessor Processor,
	rescheduler Rescheduler,
	criticalRetryCount dynamicproperties.IntPropertyFn,
) Task {
	timeSource := shard.GetTimeSource()
	var eventLogger eventLogger
	if shard.GetConfig().EnableDebugMode &&
		(queueType == QueueTypeActiveTimer || queueType == QueueTypeActiveTransfer) &&
		shard.GetConfig().EnableTaskInfoLogByDomainID(taskInfo.GetDomainID()) {
		eventLogger = newEventLogger(logger, timeSource, defaultTaskEventLoggerSize)
		eventLogger.AddEvent("Created task")
	}

	return &taskImpl{
		Task:               taskInfo,
		shard:              shard,
		state:              ctask.TaskStatePending,
		priority:           noPriority,
		queueType:          queueType,
		scope:              metrics.NoopScope,
		logger:             logger,
		eventLogger:        eventLogger,
		attempt:            0,
		initialSubmitTime:  timeSource.Now(),
		timeSource:         timeSource,
		criticalRetryCount: criticalRetryCount,
		rescheduler:        rescheduler,
		taskFilter:         func(task persistence.Task) (bool, error) { return true, nil },
		taskExecutor:       taskExecutor,
		taskProcessor:      taskProcessor,
	}
}

func (t *taskImpl) Execute() error {
	if t.State() != ctask.TaskStatePending {
		return nil
	}
	scheduleLatency := t.timeSource.Now().Sub(t.initialSubmitTime)
	var err error
	t.shouldProcessTask, err = t.taskFilter(t.Task)
	if err != nil {
		logEvent(t.eventLogger, "TaskFilter execution failed", err)
		time.Sleep(loadDomainEntryForTaskRetryDelay)
		return err
	}
	logEvent(t.eventLogger, "Executing task", t.shouldProcessTask)
	if !t.shouldProcessTask {
		return nil
	}

	executionStartTime := t.timeSource.Now()
	defer func() {
		t.scope.IncCounter(metrics.TaskRequestsPerDomain)
		t.scope.RecordTimer(metrics.TaskProcessingLatencyPerDomain, time.Since(executionStartTime))
		t.scope.ExponentialHistogram(metrics.ExponentialTaskProcessingLatencyPerDomain, time.Since(executionStartTime))
	}()
	executeResponse, err := t.taskExecutor.Execute(t)
	t.scope = executeResponse.Scope
	if t.GetAttempt() == 0 {
		// domain level metrics for the duration between task being submitted to task scheduler and being executed
		t.scope.RecordHistogramDuration(metrics.TaskScheduleLatencyPerDomain, scheduleLatency)
	}
	if t.isPreviousExecutorActive != executeResponse.IsActiveTask {
		t.resetAttempt()
	}
	t.isPreviousExecutorActive = executeResponse.IsActiveTask
	return err
}

func (t *taskImpl) resetAttempt() {
	t.Lock()
	defer t.Unlock()
	t.attempt = 0
}

func (t *taskImpl) HandleErr(err error) (retErr error) {
	logger := t.logger.Helper()

	defer func() {
		if retErr != nil {
			logEvent(t.eventLogger, "Failed to handle error", retErr)

			t.Lock()
			defer t.Unlock()

			t.attempt++
			if t.attempt > t.criticalRetryCount() {
				t.scope.RecordTimer(metrics.TaskAttemptTimerPerDomain, time.Duration(t.attempt))
				t.scope.IntExponentialHistogram(metrics.ExponentialTaskAttemptCountsPerDomain, t.attempt)
				logger.Error("Critical error processing task, retrying.",
					tag.Error(err),
					tag.OperationCritical,
					tag.TaskType(t.GetTaskType()),
					tag.AttemptCount(t.attempt),
				)
			}
		}
	}()

	if err == nil {
		return nil
	}

	logEvent(t.eventLogger, "Handling task processing error", err)

	if _, ok := err.(*types.EntityNotExistsError); ok {
		return nil
	} else if _, ok := err.(*types.WorkflowExecutionAlreadyCompletedError); ok {
		return nil
	}

	if _, ok := t.Task.(*persistence.CloseExecutionTask); ok &&
		err == execution.ErrMissingWorkflowStartEvent &&
		t.shard.GetConfig().EnableDropStuckTaskByDomainID(t.GetDomainID()) { // use domainID here to avoid accessing domainCache
		t.scope.IncCounter(metrics.TransferTaskMissingEventCounterPerDomain)
		logger.Error("Drop close execution transfer task due to corrupted workflow history", tag.Error(err), tag.LifeCycleProcessingFailed)
		return nil
	}

	if err == errWorkflowBusy {
		t.scope.IncCounter(metrics.TaskWorkflowBusyPerDomain)
		return err
	}

	if err == errWorkflowRateLimited {
		// metrics are emitted within the rate limiter
		return err
	}

	// If the shard were recently closed we just return an error, so we retry in a bit.
	var errShardClosed *shard.ErrShardClosed
	if errors.As(err, &errShardClosed) && time.Since(errShardClosed.ClosedAt) < shard.TimeBeforeShardClosedIsError {
		return err
	}

	// If the task list is not owned by the host we connected to we just return an error, so we retry in a bit
	// with the new membership information.
	var taskListNotOwnedByHostError *cadence_errors.TaskListNotOwnedByHostError
	if errors.As(err, &taskListNotOwnedByHostError) {
		t.scope.IncCounter(metrics.TaskListNotOwnedByHostCounterPerDomain)
		return err
	}

	// this is a transient error
	if isRedispatchErr(err) {
		t.scope.IncCounter(metrics.TaskStandbyRetryCounterPerDomain)
		return err
	}

	// this is a transient error during graceful failover
	if err == ErrTaskPendingActive {
		t.scope.IncCounter(metrics.TaskPendingActiveCounterPerDomain)
		return err
	}

	if err == ErrTaskDiscarded {
		t.scope.IncCounter(metrics.TaskDiscardedPerDomain)
		err = nil
	}

	if err == execution.ErrMissingVersionHistories {
		t.logger.Error("Encounter 2DC workflow during task processing.")
		t.scope.IncCounter(metrics.TaskUnsupportedPerDomain)
		err = nil
	}

	// using a fairly long timeout here because domain updates is an async process
	// which could take a fair while to be processed by the domain queue, the DB updated
	// the domain cache refeshed and then updated here.
	var e *types.DomainNotActiveError
	if errors.As(err, &e) || errors.Is(err, types.DomainNotActiveError{}) {
		if t.timeSource.Now().Sub(t.initialSubmitTime) > 5*cache.DomainCacheRefreshInterval {
			t.scope.IncCounter(metrics.TaskNotActiveCounterPerDomain)
			// If the domain is *still* not active, drop after a while.
			return nil
		}

		return err
	}

	t.scope.IncCounter(metrics.TaskFailuresPerDomain)

	if _, ok := err.(*persistence.CurrentWorkflowConditionFailedError); ok {
		logger.Error("More than 2 workflow are running.", tag.Error(err), tag.LifeCycleProcessingFailed)
		return nil
	}

	attempt := t.GetAttempt()
	if attempt > stickyTaskMaxRetryCount && common.IsStickyTaskConditionError(err) {
		// sticky task could end up into endless loop in rare cases and
		// cause worker to keep getting decision timeout unless restart.
		// return nil here to break the endless loop
		return nil
	}

	logger.Error("Fail to process task", tag.Error(err), tag.LifeCycleProcessingFailed, tag.AttemptCount(attempt))
	return err
}

func (t *taskImpl) RetryErr(err error) bool {
	if t.State() != ctask.TaskStatePending {
		return false
	}

	var errShardClosed *shard.ErrShardClosed
	if errors.As(err, &errShardClosed) || err == errWorkflowBusy || isRedispatchErr(err) || err == ErrTaskPendingActive || common.IsContextTimeoutError(err) {
		return false
	}

	return true
}

func (t *taskImpl) Ack() {
	logEvent(t.eventLogger, "Acked task")

	t.Lock()
	defer t.Unlock()

	if t.state != ctask.TaskStatePending {
		return
	}

	t.state = ctask.TaskStateAcked
	if t.shouldProcessTask {
		// Record attempt count as duration so timer mean ≈ average attempt count.
		t.scope.RecordTimer(metrics.TaskAttemptTimerPerDomain, time.Duration(t.attempt))
		// Use IntExponentialHistogram with Mid1To16k buckets (1–64k) for attempt counts
		t.scope.IntExponentialHistogram(metrics.ExponentialTaskAttemptCountsPerDomain, t.attempt)
		t.scope.RecordTimer(metrics.TaskLatencyPerDomain, time.Since(t.initialSubmitTime))
		t.scope.ExponentialHistogram(metrics.ExponentialTaskLatencyPerDomain, time.Since(t.initialSubmitTime))
		t.scope.RecordTimer(metrics.TaskQueueLatencyPerDomain, time.Since(t.GetVisibilityTimestamp()))
		t.scope.ExponentialHistogram(metrics.ExponentialTaskQueueLatencyPerDomain, time.Since(t.GetVisibilityTimestamp()))

	}

	if t.eventLogger != nil && t.shouldProcessTask && t.attempt != 0 {
		// only dump events when the task should be processed and has been retried
		t.eventLogger.FlushEvents("Task processing events")
	}
}

func (t *taskImpl) Nack(err error) {
	if t.State() != ctask.TaskStatePending {
		return
	}

	logEvent(t.eventLogger, "Nacked task")

	if t.shouldResubmitOnNack(err) {
		if submitted, _ := t.taskProcessor.TrySubmit(t); submitted {
			return
		}
	}

	if t.rescheduler != nil {
		t.rescheduler.RescheduleTask(t, t.timeSource.Now().Add(t.backoffDuration(t.GetAttempt())))
	} else if t.redispatcher != nil {
		t.redispatcher.RedispatchTask(t, t.timeSource.Now().Add(t.backoffDuration(t.GetAttempt())))
	}
}

func (t *taskImpl) Cancel() {
	t.Lock()
	defer t.Unlock()

	if t.state == ctask.TaskStatePending {
		t.state = ctask.TaskStateCanceled
	}
}

func (t *taskImpl) State() ctask.State {
	t.Lock()
	defer t.Unlock()

	return t.state
}

func (t *taskImpl) Priority() int {
	t.Lock()
	defer t.Unlock()

	return t.priority
}

func (t *taskImpl) SetPriority(priority int) {
	t.Lock()
	defer t.Unlock()

	t.priority = priority
}

func (t *taskImpl) GetShard() shard.Context {
	return t.shard
}

func (t *taskImpl) GetAttempt() int {
	t.Lock()
	defer t.Unlock()

	return t.attempt
}

func (t *taskImpl) GetInfo() persistence.Task {
	return t.Task
}

func (t *taskImpl) GetQueueType() QueueType {
	return t.queueType
}

func (t *taskImpl) SetInitialSubmitTime(submitTime time.Time) {
	t.Lock()
	defer t.Unlock()

	t.initialSubmitTime = submitTime
}

func (t *taskImpl) shouldResubmitOnNack(err error) bool {
	// TODO: for now only resubmit active task on Nack()
	// we can also consider resubmit standby tasks that fails due to certain error types
	// this may require change the Nack() interface to Nack(error)
	if errors.Is(err, errDomainBecomesActive) {
		return true
	}
	return t.GetAttempt() < activeTaskResubmitMaxAttempts &&
		(t.queueType == QueueTypeActiveTransfer || t.queueType == QueueTypeActiveTimer || ((t.queueType == QueueTypeTransfer || t.queueType == QueueTypeTimer) && t.isPreviousExecutorActive))
}

func (t *taskImpl) backoffDuration(attempt int) time.Duration {
	// TODO: we might need to consider using the same backoff policy for active and standby tasks,
	// but we keep them separate for now so that we can compare metrics between history queue v1 and v2
	if t.isPreviousExecutorActive {
		return activeTaskRedispatchPolicy.ComputeNextDelay(0, attempt)
	}
	return standbyTaskRedispatchPolicy.ComputeNextDelay(0, attempt)
}

func logEvent(
	eventLogger eventLogger,
	msg string,
	detail ...interface{},
) {
	if eventLogger != nil {
		eventLogger.AddEvent(msg, detail...)
	}
}

// getOrCreateDomainTaggedScope returns cached domain-tagged metrics scope if exists
// otherwise, it creates a new domain-tagged scope, cache and return the scope
func getOrCreateDomainTaggedScope(
	shard shard.Context,
	scopeIdx metrics.ScopeIdx,
	domainID string,
	logger log.Logger,
) metrics.Scope {
	scopeCache := shard.GetService().GetDomainMetricsScopeCache()
	scope, ok := scopeCache.Get(domainID, scopeIdx)
	if !ok {
		domainTag, err := getDomainTagByID(shard.GetDomainCache(), domainID)
		scope = shard.GetMetricsClient().Scope(scopeIdx, domainTag)
		if err == nil {
			scopeCache.Put(domainID, scopeIdx, scope)
		} else {
			logger.Error("Unable to get domainName", tag.Error(err))
		}
	}
	return scope
}

func getDomainTagByID(
	domainCache cache.DomainCache,
	domainID string,
) (metrics.Tag, error) {
	domainName, err := domainCache.GetDomainName(domainID)
	if err != nil {
		return metrics.DomainUnknownTag(), err
	}
	return metrics.DomainTag(domainName), nil
}

func createTaskRedispatchPolicy(initialInterval time.Duration) backoff.RetryPolicy {
	backoffPolicy := backoff.NewExponentialRetryPolicy(initialInterval)
	backoffPolicy.SetBackoffCoefficient(redispatchBackoffCoefficient)
	backoffPolicy.SetMaximumInterval(redispatchMaxBackoffInternval)
	backoffPolicy.SetExpirationInterval(backoff.NoInterval)
	return backoffPolicy
}
