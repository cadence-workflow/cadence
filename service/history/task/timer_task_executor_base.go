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

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/worker/archiver"
)

var (
	taskRetryPolicy = common.CreateTaskProcessingRetryPolicy()
)

type (
	timerTaskExecutorBase struct {
		shard          shard.Context
		archiverClient archiver.Client
		executionCache execution.Cache
		logger         log.Logger
		metricsClient  metrics.Client
		config         *config.Config
		throttleRetry  *backoff.ThrottleRetry
		ctx            context.Context
		cancelFn       context.CancelFunc
	}
)

func newTimerTaskExecutorBase(
	shard shard.Context,
	archiverClient archiver.Client,
	executionCache execution.Cache,
	logger log.Logger,
	metricsClient metrics.Client,
	config *config.Config,
) *timerTaskExecutorBase {
	ctx, cancelFn := context.WithCancel(context.Background())
	return &timerTaskExecutorBase{
		shard:          shard,
		archiverClient: archiverClient,
		executionCache: executionCache,
		logger:         logger,
		metricsClient:  metricsClient,
		config:         config,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(taskRetryPolicy),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
		ctx:      ctx,
		cancelFn: cancelFn,
	}
}

func (t *timerTaskExecutorBase) executeDeleteHistoryEventTask(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
) (retError error) {

	wfContext, release, err := t.executionCache.GetOrCreateWorkflowExecutionWithTimeout(
		task.DomainID,
		getWorkflowExecution(task),
		taskGetExecutionContextTimeout,
	)
	if err != nil {
		return err
	}
	defer func() { release(retError) }()

	mutableState, err := loadMutableState(ctx, wfContext, task, t.metricsClient.Scope(metrics.TimerQueueProcessorScope), t.logger, 0)
	if err != nil {
		return err
	}
	if mutableState == nil {
		t.logger.Debug("could not load mutable state while attempting to clean up workflow",
			tag.WorkflowID(task.WorkflowID),
			tag.WorkflowRunID(task.RunID),
			tag.WorkflowDomainID(task.DomainID),
		)
		t.metricsClient.IncCounter(metrics.HistoryProcessDeleteHistoryEventScope, metrics.TimerProcessingDeletionTimerNoopDueToMutableStateNotLoading)
		return nil
	}
	if mutableState.IsWorkflowExecutionRunning() {
		t.logger.Warn("could not clean up workflow, it was running",
			tag.WorkflowID(task.WorkflowID),
			tag.WorkflowRunID(task.RunID),
			tag.WorkflowDomainID(task.DomainID),
		)
		t.metricsClient.IncCounter(metrics.HistoryProcessDeleteHistoryEventScope, metrics.TimerProcessingDeletionTimerNoopDueToMutableStateNotLoading)
		return nil
	}

	lastWriteVersion, err := mutableState.GetLastWriteVersion()
	if err != nil {
		return err
	}
	ok, err := verifyTaskVersion(t.shard, t.logger, task.DomainID, lastWriteVersion, task.Version, task)
	if err != nil || !ok {
		return err
	}

	domainCacheEntry, err := t.shard.GetDomainCache().GetDomainByID(task.DomainID)
	if err != nil {
		return err
	}
	clusterConfiguredForHistoryArchival := t.shard.GetService().GetArchivalMetadata().GetHistoryConfig().ClusterConfiguredForArchival()
	domainConfiguredForHistoryArchival := domainCacheEntry.GetConfig().HistoryArchivalStatus == types.ArchivalStatusEnabled
	archiveHistory := clusterConfiguredForHistoryArchival && domainConfiguredForHistoryArchival

	// TODO: @ycyang once archival backfill is in place cluster:paused && domain:enabled should be a nop rather than a delete
	if archiveHistory {
		t.metricsClient.IncCounter(metrics.HistoryProcessDeleteHistoryEventScope, metrics.WorkflowCleanupArchiveCount)
		return t.archiveWorkflow(ctx, task, wfContext, mutableState, domainCacheEntry)
	}

	t.metricsClient.IncCounter(metrics.HistoryProcessDeleteHistoryEventScope, metrics.WorkflowCleanupDeleteCount)
	return t.deleteWorkflow(ctx, task, wfContext, mutableState)
}

func (t *timerTaskExecutorBase) deleteWorkflow(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
	context execution.Context,
	msBuilder execution.MutableState,
) error {

	if err := t.deleteWorkflowHistory(ctx, task, msBuilder); err != nil {
		return err
	}

	if err := t.deleteWorkflowVisibility(ctx, task); err != nil {
		return err
	}

	if err := t.deleteCurrentWorkflowExecution(ctx, task); err != nil {
		return err
	}

	if err := t.deleteActiveClusterSelectionPolicy(ctx, task); err != nil {
		return err
	}

	// it must be the last one due to the nature of workflow execution deletion
	if err := t.deleteWorkflowExecution(ctx, task); err != nil {
		return err
	}

	// calling clear here to force accesses of mutable state to read database
	// if this is not called then callers will get mutable state even though its been removed from database
	context.Clear()
	return nil
}

func (t *timerTaskExecutorBase) archiveWorkflow(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
	workflowContext execution.Context,
	msBuilder execution.MutableState,
	domainCacheEntry *cache.DomainCacheEntry,
) error {
	branchToken, err := msBuilder.GetCurrentBranchToken()
	if err != nil {
		return err
	}
	closeFailoverVersion, err := msBuilder.GetLastWriteVersion()
	if err != nil {
		return err
	}

	req := &archiver.ClientRequest{
		ArchiveRequest: &archiver.ArchiveRequest{
			DomainID:             task.DomainID,
			WorkflowID:           task.WorkflowID,
			RunID:                task.RunID,
			DomainName:           domainCacheEntry.GetInfo().Name,
			ShardID:              t.shard.GetShardID(),
			Targets:              []archiver.ArchivalTarget{archiver.ArchiveTargetHistory},
			URI:                  domainCacheEntry.GetConfig().HistoryArchivalURI,
			NextEventID:          msBuilder.GetNextEventID(),
			BranchToken:          branchToken,
			CloseFailoverVersion: closeFailoverVersion,
		},
		CallerService:        service.History,
		AttemptArchiveInline: false, // archive in workflow by default
	}
	executionStats, err := workflowContext.LoadExecutionStats(ctx)
	if err == nil && executionStats.HistorySize < int64(t.config.TimerProcessorHistoryArchivalSizeLimit()) {
		req.AttemptArchiveInline = true
	}

	archiveCtx, cancel := context.WithTimeout(ctx, t.config.TimerProcessorArchivalTimeLimit())
	defer cancel()
	resp, err := t.archiverClient.Archive(archiveCtx, req)
	if err != nil {
		return err
	}

	// delete workflow history if history archival is not needed or history as been archived inline
	if resp.HistoryArchivedInline {
		t.metricsClient.IncCounter(metrics.HistoryProcessDeleteHistoryEventScope, metrics.WorkflowCleanupDeleteHistoryInlineCount)
		if err := t.deleteWorkflowHistory(ctx, task, msBuilder); err != nil {
			return err
		}
	}
	// delete visibility record here regardless if it's been archived inline or not
	// since the entire record is included as part of the archive request.
	if err := t.deleteWorkflowVisibility(ctx, task); err != nil {
		return err
	}

	if err := t.deleteActiveClusterSelectionPolicy(ctx, task); err != nil {
		return err
	}

	if err := t.deleteCurrentWorkflowExecution(ctx, task); err != nil {
		return err
	}
	if err := t.deleteWorkflowExecution(ctx, task); err != nil {
		return err
	}
	// calling clear here to force accesses of mutable state to read database
	// if this is not called then callers will get mutable state even though its been removed from database
	workflowContext.Clear()
	return nil
}

func (t *timerTaskExecutorBase) deleteWorkflowExecution(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
) error {
	domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context) error {
		return t.shard.GetExecutionManager().DeleteWorkflowExecution(ctx, &persistence.DeleteWorkflowExecutionRequest{
			DomainID:   task.DomainID,
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
			DomainName: domainName,
		})
	}
	return t.throttleRetry.Do(ctx, op)
}

func (t *timerTaskExecutorBase) deleteCurrentWorkflowExecution(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
) error {
	domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if err != nil {
		return err
	}
	op := func(ctx context.Context) error {
		return t.shard.GetExecutionManager().DeleteCurrentWorkflowExecution(ctx, &persistence.DeleteCurrentWorkflowExecutionRequest{
			DomainID:   task.DomainID,
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
			DomainName: domainName,
		})
	}
	return t.throttleRetry.Do(ctx, op)
}

func (t *timerTaskExecutorBase) deleteActiveClusterSelectionPolicy(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
) error {
	op := func(ctx context.Context) error {
		return t.shard.GetExecutionManager().DeleteActiveClusterSelectionPolicy(ctx, task.DomainID, task.WorkflowID, task.RunID)
	}
	return t.throttleRetry.Do(ctx, op)
}

func (t *timerTaskExecutorBase) deleteWorkflowHistory(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
	msBuilder execution.MutableState,
) error {

	op := func(ctx context.Context) error {
		branchToken, err := msBuilder.GetCurrentBranchToken()
		if err != nil {
			return err
		}
		domainName, err := t.shard.GetDomainCache().GetDomainName(task.DomainID)
		if err != nil {
			return err
		}
		return t.shard.GetHistoryManager().DeleteHistoryBranch(ctx, &persistence.DeleteHistoryBranchRequest{
			BranchToken: branchToken,
			ShardID:     common.IntPtr(t.shard.GetShardID()),
			DomainName:  domainName,
		})

	}
	return t.throttleRetry.Do(ctx, op)
}

func (t *timerTaskExecutorBase) deleteWorkflowVisibility(
	ctx context.Context,
	task *persistence.DeleteHistoryEventTask,
) error {

	domain, errorDomainName := t.shard.GetDomainCache().GetDomainName(task.DomainID)
	if errorDomainName != nil {
		return errorDomainName
	}
	op := func(ctx context.Context) error {
		request := &persistence.VisibilityDeleteWorkflowExecutionRequest{
			DomainID:   task.DomainID,
			Domain:     domain,
			WorkflowID: task.WorkflowID,
			RunID:      task.RunID,
			TaskID:     task.TaskID,
		}
		// TODO: expose GetVisibilityManager method on shardContext interface
		return t.shard.GetService().GetVisibilityManager().DeleteWorkflowExecution(ctx, request) // delete from db
	}
	return t.throttleRetry.Do(ctx, op)
}

func (t *timerTaskExecutorBase) Stop() {
	t.logger.Info("Stopping timerTaskExecutorBase")
	t.cancelFn()
}
