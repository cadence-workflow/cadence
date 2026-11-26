package task

import (
	"context"
	"errors"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/simulation"
	"github.com/uber/cadence/service/worker/archiver"
)

var errUnknownVisibilityTask = errors.New("unknown visibility task")

type (
	// visibilityTaskExecutor processes tasks from the visibility queue.
	// Visibility records are built from local mutable state and have no cross-workflow side effects,
	// so the same executor handles tasks for both active and standby domains.
	visibilityTaskExecutor struct {
		*transferTaskExecutorBase
	}
)

// NewVisibilityTaskExecutor creates a new executor for the visibility queue
func NewVisibilityTaskExecutor(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	logger log.Logger,
	config *config.Config,
) Executor {
	return &visibilityTaskExecutor{
		transferTaskExecutorBase: newTransferTaskExecutorBase(
			shard,
			archiverClient,
			executionCache,
			logger,
			config,
			nil, // visibility tasks are not written to the DLQ yet
		),
	}
}

func (t *visibilityTaskExecutor) Execute(task Task) (ExecuteResponse, error) {
	simulation.LogEvents(simulation.E{
		EventName:  simulation.EventNameExecuteHistoryTask,
		Host:       t.shard.GetConfig().HostName,
		ShardID:    t.shard.GetShardID(),
		DomainID:   task.GetDomainID(),
		WorkflowID: task.GetWorkflowID(),
		RunID:      task.GetRunID(),
		Payload: map[string]any{
			"task_category": persistence.HistoryTaskCategoryVisibility.Name(),
			"task_type":     task.GetTaskType(),
			"task_key":      task.GetTaskKey(),
		},
	})
	scope := getOrCreateDomainTaggedScope(t.shard, getVisibilityTaskMetricsScope(task.GetTaskType()), task.GetDomainID(), t.logger)
	executeResponse := ExecuteResponse{
		Scope: scope,
		// there is no standby counterpart, so the task always uses the active retry and resubmit policy
		IsActiveTask: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskDefaultTimeout)
	defer cancel()

	switch visibilityTask := task.GetInfo().(type) {
	case *persistence.RecordWorkflowStartedTask:
		return executeResponse, t.processRecordWorkflowStartedOrUpsert(ctx, visibilityTask, true)
	case *persistence.UpsertWorkflowSearchAttributesTask:
		return executeResponse, t.processRecordWorkflowStartedOrUpsert(ctx, visibilityTask, false)
	case *persistence.RecordWorkflowClosedTask:
		return executeResponse, t.processRecordWorkflowClosed(ctx, visibilityTask)
	default:
		return executeResponse, errUnknownVisibilityTask
	}
}

// Empty func for now
func (t *visibilityTaskExecutor) Stop() {}

func (t *visibilityTaskExecutor) processRecordWorkflowStartedOrUpsert(
	ctx context.Context,
	task persistence.Task,
	recordStart bool,
) (retError error) {
	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.GetDomainID(),
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

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.VisibilityQueueProcessorV2Scope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || !mutableState.IsWorkflowExecutionRunning() {
		return nil
	}

	// verify task version for RecordWorkflowStarted.
	// upsert doesn't require verifyTask, because it is just a sync of mutableState.
	if recordStart {
		startVersion, err := mutableState.GetStartVersion()
		if err != nil {
			return err
		}
		ok, err := verifyTaskVersion(t.shard, t.logger, task.GetDomainID(), startVersion, task.GetVersion(), task)
		if err != nil || !ok {
			return err
		}
	}

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil {
		return err
	}

	record, err := t.newStartedVisibilityRecord(ctx, mutableState, domainEntry)
	if err != nil {
		return err
	}

	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	if recordStart {
		getOrCreateDomainTaggedScope(t.shard, metrics.VisibilityTaskRecordWorkflowStartedScope, task.GetDomainID(), t.logger).
			IncCounter(metrics.WorkflowStartedCount)
		return t.recordWorkflowStarted(ctx, task, record)
	}
	return t.upsertWorkflowExecution(ctx, task, record)
}

// processRecordWorkflowClosed writes the closed visibility record and archives visibility if enabled.
// Unlike CloseExecutionTask in the transfer queue, it never replies to the parent or applies the parent close policy.
func (t *visibilityTaskExecutor) processRecordWorkflowClosed(
	ctx context.Context,
	task *persistence.RecordWorkflowClosedTask,
) (retError error) {
	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.GetDomainID(),
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

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.VisibilityQueueProcessorV2Scope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil || mutableState.IsWorkflowExecutionRunning() {
		// running can happen if the workflow is reset
		return nil
	}

	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return err
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.GetDomainID(), lastWriteVersion, task.GetVersion(), task)
	if err != nil || !ok {
		return err
	}

	domainEntry, err := t.shard.GetDomainCache().GetDomainByID(task.GetDomainID())
	if err != nil {
		return err
	}

	completionEvent, err := mutableState.GetCompletionEvent(ctx)
	if err != nil {
		return err
	}
	record, err := t.newClosedVisibilityRecord(ctx, mutableState, domainEntry, completionEvent)
	if err != nil {
		return err
	}

	// release the context lock since we no longer need mutable state builder and
	// the rest of logic is making RPC call, which takes time.
	release(nil)

	return t.recordWorkflowClosed(ctx, task, record)
}

func getVisibilityTaskMetricsScope(taskType int) metrics.ScopeIdx {
	switch taskType {
	case persistence.TransferTaskTypeRecordWorkflowStarted:
		return metrics.VisibilityTaskRecordWorkflowStartedScope
	case persistence.TransferTaskTypeUpsertWorkflowSearchAttributes:
		return metrics.VisibilityTaskUpsertWorkflowSearchAttributesScope
	case persistence.TransferTaskTypeRecordWorkflowClosed:
		return metrics.VisibilityTaskRecordWorkflowClosedScope
	default:
		return metrics.VisibilityQueueProcessorV2Scope
	}
}
