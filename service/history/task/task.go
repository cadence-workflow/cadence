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
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
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

var (
	// ErrTaskDiscarded is the error indicating that the timer / transfer task is pending for too long and discarded.
	ErrTaskDiscarded = errors.New("passive task pending for too long")
	// ErrTaskPendingActive is the error indicating that the task should be re-dispatched
	ErrTaskPendingActive = errors.New("redispatch the task while the domain is pending-active")
)

type (
	taskImpl struct {
		sync.Mutex
		Info

		shard              shard.Context
		state              ctask.State
		priority           int
		attempt            int
		timeSource         clock.TimeSource
		submitTime         time.Time
		logger             log.Logger
		eventLogger        eventLogger
		scopeIdx           int
		scope              metrics.Scope // initialized when processing task to make the initialization parallel
		taskExecutor       Executor
		taskProcessor      Processor
		redispatchFn       func(task Task)
		criticalRetryCount dynamicconfig.IntPropertyFn

		// TODO: following three fields should be removed after new task lifecycle is implemented
		taskFilter        Filter
		queueType         QueueType
		shouldProcessTask bool

		// if true, the skip the execution of the task and only emit a metric
		shadowTask bool
	}
)

// NewTimerTask creates a new timer task
func NewTimerTask(
	shard shard.Context,
	taskInfo Info,
	queueType QueueType,
	logger log.Logger,
	taskFilter Filter,
	taskExecutor Executor,
	taskProcessor Processor,
	redispatchFn func(task Task),
	criticalRetryCount dynamicconfig.IntPropertyFn,
) Task {
	return newTask(
		shard,
		taskInfo,
		queueType,
		GetTimerTaskMetricScope(taskInfo.GetTaskType(), queueType == QueueTypeActiveTimer),
		logger,
		taskFilter,
		taskExecutor,
		taskProcessor,
		criticalRetryCount,
		redispatchFn,
	)
}

// NewTransferTask creates a new transfer task
func NewTransferTask(
	shard shard.Context,
	taskInfo Info,
	queueType QueueType,
	logger log.Logger,
	taskFilter Filter,
	taskExecutor Executor,
	taskProcessor Processor,
	redispatchFn func(task Task),
	criticalRetryCount dynamicconfig.IntPropertyFn,
) Task {
	return newTask(
		shard,
		taskInfo,
		queueType,
		GetTransferTaskMetricsScope(taskInfo.GetTaskType(), queueType == QueueTypeActiveTransfer),
		logger,
		taskFilter,
		taskExecutor,
		taskProcessor,
		criticalRetryCount,
		redispatchFn,
	)
}

func newTask(
	shard shard.Context,
	taskInfo Info,
	queueType QueueType,
	scopeIdx int,
	logger log.Logger,
	taskFilter Filter,
	taskExecutor Executor,
	taskProcessor Processor,
	criticalRetryCount dynamicconfig.IntPropertyFn,
	redispatchFn func(task Task),
) *taskImpl {
	timeSource := shard.GetTimeSource()
	var eventLogger eventLogger
	if shard.GetConfig().EnableDebugMode &&
		(queueType == QueueTypeActiveTimer || queueType == QueueTypeActiveTransfer) &&
		shard.GetConfig().EnableTaskInfoLogByDomainID(taskInfo.GetDomainID()) {
		eventLogger = newEventLogger(logger, timeSource, defaultTaskEventLoggerSize)
		eventLogger.AddEvent("Created task")
	}

	return &taskImpl{
		Info:               taskInfo,
		shard:              shard,
		state:              ctask.TaskStatePending,
		priority:           noPriority,
		queueType:          queueType,
		scopeIdx:           scopeIdx,
		scope:              nil,
		logger:             logger,
		eventLogger:        eventLogger,
		attempt:            0,
		submitTime:         timeSource.Now(),
		timeSource:         timeSource,
		criticalRetryCount: criticalRetryCount,
		redispatchFn:       redispatchFn,
		taskFilter:         taskFilter,
		taskExecutor:       taskExecutor,
		taskProcessor:      taskProcessor,
	}
}

func (t *taskImpl) Execute() error {
	// TODO: after mergering active and standby queue,
	// the task should be smart enough to tell if it should be
	// processed as active or standby and use the corresponding
	// task executor.
	if t.scope == nil {
		t.scope = getOrCreateDomainTaggedScope(t.shard, t.scopeIdx, t.GetDomainID(), t.logger)
	}

	var err error
	t.shouldProcessTask, err = t.taskFilter(t.Info)
	if err != nil {
		logEvent(t.eventLogger, "TaskFilter execution failed", err)
		time.Sleep(loadDomainEntryForTaskRetryDelay)
		return err
	}

	if t.shadowTask {
		if t.shouldProcessTask {
			t.scope.IncCounter(metrics.TaskShadowRequestsPerDomain)
		}
		return nil
	}

	executionStartTime := t.timeSource.Now()

	defer func() {
		if t.shouldProcessTask {
			t.scope.IncCounter(metrics.TaskRequestsPerDomain)
			t.scope.RecordTimer(metrics.TaskProcessingLatencyPerDomain, time.Since(executionStartTime))
		}
	}()

	logEvent(t.eventLogger, "Executing task", t.shouldProcessTask)
	return t.taskExecutor.Execute(t, t.shouldProcessTask)
}

func (t *taskImpl) HandleErr(err error) (retErr error) {
	defer func() {
		if retErr != nil {
			logEvent(t.eventLogger, "Failed to handle error", retErr)

			t.Lock()
			defer t.Unlock()

			t.attempt++
			if t.attempt > t.criticalRetryCount() {
				t.scope.RecordTimer(metrics.TaskAttemptTimerPerDomain, time.Duration(t.attempt))
				t.logger.Error("Critical error processing task, retrying.",
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

	if transferTask, ok := t.Info.(*persistence.TransferTaskInfo); ok &&
		transferTask.TaskType == persistence.TransferTaskTypeCloseExecution &&
		err == execution.ErrMissingWorkflowStartEvent &&
		t.shard.GetConfig().EnableDropStuckTaskByDomainID(t.Info.GetDomainID()) { // use domainID here to avoid accessing domainCache
		t.scope.IncCounter(metrics.TransferTaskMissingEventCounterPerDomain)
		t.logger.Error("Drop close execution transfer task due to corrupted workflow history", tag.Error(err), tag.LifeCycleProcessingFailed)
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
		if t.timeSource.Now().Sub(t.submitTime) > 5*cache.DomainCacheRefreshInterval {
			t.scope.IncCounter(metrics.TaskNotActiveCounterPerDomain)
			// If the domain is *still* not active, drop after a while.
			return nil
		}

		return err
	}

	t.scope.IncCounter(metrics.TaskFailuresPerDomain)

	if _, ok := err.(*persistence.CurrentWorkflowConditionFailedError); ok {
		t.logger.Error("More than 2 workflow are running.", tag.Error(err), tag.LifeCycleProcessingFailed)
		return nil
	}

	if t.GetAttempt() > stickyTaskMaxRetryCount && common.IsStickyTaskConditionError(err) {
		// sticky task could end up into endless loop in rare cases and
		// cause worker to keep getting decision timeout unless restart.
		// return nil here to break the endless loop
		return nil
	}

	t.logger.Error("Fail to process task", tag.Error(err), tag.LifeCycleProcessingFailed)
	return err
}

func (t *taskImpl) RetryErr(err error) bool {
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

	t.state = ctask.TaskStateAcked

	if t.shadowTask {
		if t.shouldProcessTask {
			t.scope.RecordTimer(metrics.TaskShadowLatencyPerDomain, time.Since(t.submitTime))
		}
		return
	}

	if t.shouldProcessTask {
		t.scope.RecordTimer(metrics.TaskAttemptTimerPerDomain, time.Duration(t.attempt))
		t.scope.RecordTimer(metrics.TaskLatencyPerDomain, time.Since(t.submitTime))
		t.scope.RecordTimer(metrics.TaskQueueLatencyPerDomain, time.Since(t.GetVisibilityTimestamp()))

	}

	if t.eventLogger != nil && t.shouldProcessTask && t.attempt != 0 {
		// only dump events when the task should be processed and has been retried
		t.eventLogger.FlushEvents("Task processing events")
	}
}

func (t *taskImpl) Nack() {
	logEvent(t.eventLogger, "Nacked task")

	t.Lock()
	t.state = ctask.TaskStateNacked
	t.Unlock()

	if t.shadowTask {
		return
	}

	if t.shouldResubmitOnNack() {
		if submitted, _ := t.taskProcessor.TrySubmit(t); submitted {
			return
		}
	}

	t.redispatchFn(t)
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

func (t *taskImpl) GetInfo() Info {
	return t.Info
}

func (t *taskImpl) GetQueueType() QueueType {
	return t.queueType
}

func (t *taskImpl) shouldResubmitOnNack() bool {
	// TODO: for now only resubmit active task on Nack()
	// we can also consider resubmit standby tasks that fails due to certain error types
	// this may require change the Nack() interface to Nack(error)
	return t.GetAttempt() < activeTaskResubmitMaxAttempts &&
		(t.queueType == QueueTypeActiveTransfer || t.queueType == QueueTypeActiveTimer)
}

func (t *taskImpl) ShadowCopy() Task {
	return &taskImpl{
		Info:               t.Info,
		shard:              t.shard,
		state:              ctask.TaskStatePending,
		priority:           t.priority,
		queueType:          t.queueType,
		scopeIdx:           t.scopeIdx,
		scope:              nil,
		logger:             t.logger,
		eventLogger:        t.eventLogger,
		attempt:            0,
		submitTime:         t.timeSource.Now(),
		timeSource:         t.timeSource,
		criticalRetryCount: t.criticalRetryCount,
		redispatchFn:       nil,
		taskFilter:         t.taskFilter,
		taskExecutor:       nil,
		taskProcessor:      nil,
		shadowTask:         true,
	}
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
	scopeIdx int,
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
