// Copyright (c) 2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination context_mock.go -self_package github.com/uber/cadence/service/history/execution

package execution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/locks"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/shard"
)

const (
	defaultRemoteCallTimeout = 30 * time.Second
	checksumErrorRetryCount  = 3
	maxLockDuration          = 1 * time.Second
)

type conflictError struct {
	Cause error
}

func (e *conflictError) Error() string {
	return fmt.Sprintf("conditional update failed: %v", e.Cause)
}

func (e *conflictError) Unwrap() error {
	return e.Cause
}

// NewConflictError is only public because it used in workflow/util_test.go
// TODO: refactor those tests
func NewConflictError(_ *testing.T, cause error) error {
	return &conflictError{cause}
}

// IsConflictError checks whether a conflict has occurred while updating a workflow execution
func IsConflictError(err error) bool {
	var e *conflictError
	return errors.As(err, &e)
}

type (
	// Context is the processing context for all operations on workflow execution
	Context interface {
		GetDomainName() string
		GetDomainID() string
		GetExecution() *types.WorkflowExecution

		GetWorkflowExecution() MutableState
		SetWorkflowExecution(mutableState MutableState)
		LoadWorkflowExecution(ctx context.Context) (MutableState, error)
		LoadWorkflowExecutionWithTaskVersion(ctx context.Context, incomingVersion int64) (MutableState, error)
		LoadExecutionStats(ctx context.Context) (*persistence.ExecutionStats, error)
		Clear()

		Lock(ctx context.Context) error
		Unlock()

		GetHistorySize() int64
		SetHistorySize(size int64)

		ReapplyEvents(
			eventBatches []*persistence.WorkflowEvents,
		) error

		PersistStartWorkflowBatchEvents(
			ctx context.Context,
			workflowEvents *persistence.WorkflowEvents,
		) (events.PersistedBlob, error)
		PersistNonStartWorkflowBatchEvents(
			ctx context.Context,
			workflowEvents *persistence.WorkflowEvents,
		) (events.PersistedBlob, error)

		CreateWorkflowExecution(
			ctx context.Context,
			newWorkflow *persistence.WorkflowSnapshot,
			persistedHistory events.PersistedBlob,
			createMode persistence.CreateWorkflowMode,
			prevRunID string,
			// TODO(active-active): only passing prevLastWriteVersion might not be enough for active-active workflows, we may consider passing ActiveClusterSelectionPolicy as well
			// and include ActiveClusterSelectionPolicy in conditional update of current execution record
			prevLastWriteVersion int64,
			workflowRequestMode persistence.CreateWorkflowRequestMode,
		) error
		ConflictResolveWorkflowExecution(
			ctx context.Context,
			now time.Time,
			conflictResolveMode persistence.ConflictResolveWorkflowMode,
			resetMutableState MutableState,
			newContext Context,
			newMutableState MutableState,
			currentContext Context,
			currentMutableState MutableState,
			currentTransactionPolicy *TransactionPolicy,
		) error
		UpdateWorkflowExecutionAsActive(
			ctx context.Context,
			now time.Time,
		) error
		UpdateWorkflowExecutionWithNewAsActive(
			ctx context.Context,
			now time.Time,
			newContext Context,
			newMutableState MutableState,
		) error
		UpdateWorkflowExecutionAsPassive(
			ctx context.Context,
			now time.Time,
		) error
		UpdateWorkflowExecutionWithNewAsPassive(
			ctx context.Context,
			now time.Time,
			newContext Context,
			newMutableState MutableState,
		) error
		UpdateWorkflowExecutionWithNew(
			ctx context.Context,
			now time.Time,
			updateMode persistence.UpdateWorkflowMode,
			newContext Context,
			newMutableState MutableState,
			currentWorkflowTransactionPolicy TransactionPolicy,
			newWorkflowTransactionPolicy *TransactionPolicy,
			workflowRequestMode persistence.CreateWorkflowRequestMode,
		) error
		UpdateWorkflowExecutionTasks(
			ctx context.Context,
			now time.Time,
		) error

		cache.Sizeable
	}
)

type (
	contextImpl struct {
		domainID          string
		workflowExecution types.WorkflowExecution
		shard             shard.Context
		executionManager  persistence.ExecutionManager
		logger            log.Logger
		metricsClient     metrics.Client

		mutex           locks.Mutex
		lockTime        time.Time
		maxLockDuration time.Duration
		mutableState    MutableState
		stats           *persistence.ExecutionStats

		appendHistoryNodesFn                  func(context.Context, string, types.WorkflowExecution, *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error)
		persistStartWorkflowBatchEventsFn     func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		persistNonStartWorkflowBatchEventsFn  func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		getWorkflowExecutionFn                func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error)
		createWorkflowExecutionFn             func(context.Context, *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error)
		updateWorkflowExecutionFn             func(context.Context, *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error)
		updateWorkflowExecutionWithNewFn      func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		notifyTasksFromWorkflowSnapshotFn     func(*persistence.WorkflowSnapshot, events.PersistedBlobs, bool)
		notifyTasksFromWorkflowMutationFn     func(*persistence.WorkflowMutation, events.PersistedBlobs, bool)
		emitSessionUpdateStatsFn              func(string, *persistence.MutableStateUpdateSessionStats)
		emitWorkflowHistoryStatsFn            func(string, int, int)
		emitWorkflowCompletionStatsFn         func(string, string, string, string, string, *types.HistoryEvent)
		mergeContinueAsNewReplicationTasksFn  func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error
		updateWorkflowExecutionEventReapplyFn func(persistence.UpdateWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error
		conflictResolveEventReapplyFn         func(persistence.ConflictResolveWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error
		emitLargeWorkflowShardIDStatsFn       func(int64, int64, int64, int64)
		emitWorkflowExecutionStatsFn          func(string, *persistence.MutableStateStats, int64)
		createMutableStateFn                  func(shard.Context, log.Logger, *cache.DomainCacheEntry) MutableState
	}
)

var _ Context = (*contextImpl)(nil)

// NewContext creates a new workflow execution context
func NewContext(
	domainID string,
	execution types.WorkflowExecution,
	shard shard.Context,
	executionManager persistence.ExecutionManager,
	logger log.Logger,
) Context {
	logger = logger.WithTags(tag.WorkflowDomainID(domainID), tag.WorkflowID(execution.GetWorkflowID()), tag.WorkflowRunID(execution.GetRunID()))
	ctx := &contextImpl{
		domainID:          domainID,
		workflowExecution: execution,
		shard:             shard,
		executionManager:  executionManager,
		logger:            logger,
		metricsClient:     shard.GetMetricsClient(),
		mutex:             locks.NewMutex(),
		maxLockDuration:   maxLockDuration,
		stats: &persistence.ExecutionStats{
			HistorySize: 0,
		},

		appendHistoryNodesFn: func(ctx context.Context, domainID string, workflowExecution types.WorkflowExecution, request *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error) {
			return appendHistoryV2EventsWithRetry(ctx, shard, common.CreatePersistenceRetryPolicy(), domainID, workflowExecution, request)
		},
		getWorkflowExecutionFn: func(ctx context.Context, request *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
			return getWorkflowExecutionWithRetry(ctx, shard, logger.Helper(), common.CreatePersistenceRetryPolicy(), request)
		},
		createWorkflowExecutionFn: func(ctx context.Context, request *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error) {
			return createWorkflowExecutionWithRetry(ctx, shard, logger.Helper(), common.CreatePersistenceRetryPolicy(), request)
		},
		updateWorkflowExecutionFn: func(ctx context.Context, request *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
			return updateWorkflowExecutionWithRetry(ctx, shard, logger.Helper(), common.CreatePersistenceRetryPolicy(), request)
		},
		notifyTasksFromWorkflowSnapshotFn: func(snapshot *persistence.WorkflowSnapshot, blobs events.PersistedBlobs, persistentError bool) {
			notifyTasksFromWorkflowSnapshot(shard.GetEngine(), snapshot, blobs, persistentError)
		},
		notifyTasksFromWorkflowMutationFn: func(snapshot *persistence.WorkflowMutation, blobs events.PersistedBlobs, persistentError bool) {
			notifyTasksFromWorkflowMutation(shard.GetEngine(), snapshot, blobs, persistentError)
		},
		emitSessionUpdateStatsFn: func(domainName string, stats *persistence.MutableStateUpdateSessionStats) {
			emitSessionUpdateStats(shard.GetMetricsClient(), domainName, stats)
		},
		emitWorkflowHistoryStatsFn: func(domainName string, historySize int, eventCount int) {
			emitWorkflowHistoryStats(shard.GetMetricsClient(), domainName, historySize, eventCount)
		},
		emitWorkflowCompletionStatsFn: func(domainName, workflowType, workflowID, runID, taskList string, event *types.HistoryEvent) {
			emitWorkflowCompletionStats(shard.GetMetricsClient(), logger.Helper(), domainName, workflowType, workflowID, runID, taskList, event)
		},
		emitWorkflowExecutionStatsFn: func(domainName string, stats *persistence.MutableStateStats, historySize int64) {
			emitWorkflowExecutionStats(shard.GetMetricsClient(), domainName, stats, historySize)
		},
		mergeContinueAsNewReplicationTasksFn: func(updateMode persistence.UpdateWorkflowMode, mutation *persistence.WorkflowMutation, snapshot *persistence.WorkflowSnapshot) error {
			return mergeContinueAsNewReplicationTasks(logger, updateMode, mutation, snapshot)
		},
		createMutableStateFn: NewMutableStateBuilder,
	}
	ctx.persistStartWorkflowBatchEventsFn = ctx.PersistStartWorkflowBatchEvents
	ctx.persistNonStartWorkflowBatchEventsFn = ctx.PersistNonStartWorkflowBatchEvents
	ctx.updateWorkflowExecutionEventReapplyFn = ctx.updateWorkflowExecutionEventReapply
	ctx.conflictResolveEventReapplyFn = ctx.conflictResolveEventReapply
	ctx.emitLargeWorkflowShardIDStatsFn = ctx.emitLargeWorkflowShardIDStats
	ctx.updateWorkflowExecutionWithNewFn = ctx.UpdateWorkflowExecutionWithNew
	return ctx
}

func (c *contextImpl) Lock(ctx context.Context) error {
	err := c.mutex.Lock(ctx)
	if err != nil {
		return err
	}
	c.lockTime = time.Now()
	return nil
}

func (c *contextImpl) Unlock() {
	defer c.mutex.Unlock()

	if c.lockTime.IsZero() { // skip logging if the lock is never acquired
		return
	}
	elapsed := time.Since(c.lockTime)
	c.metricsClient.RecordTimer(metrics.WorkflowContextScope, metrics.WorkflowContextLockLatency, elapsed)
	if elapsed > c.maxLockDuration {
		c.maxLockDuration = elapsed
		c.logger.Info("workflow context lock is released. this is logged only when it's longer than maxLockDuration", tag.WorkflowContextLockLatency(elapsed))
	}
}

func (c *contextImpl) Clear() {
	c.metricsClient.IncCounter(metrics.WorkflowContextScope, metrics.WorkflowContextCleared)
	c.mutableState = nil
	c.stats = &persistence.ExecutionStats{
		HistorySize: 0,
	}
}

func (c *contextImpl) GetDomainID() string {
	return c.domainID
}

func (c *contextImpl) GetExecution() *types.WorkflowExecution {
	return &c.workflowExecution
}

func (c *contextImpl) GetDomainName() string {
	domainName, err := c.shard.GetDomainCache().GetDomainName(c.domainID)
	if err != nil {
		return ""
	}
	return domainName
}

func (c *contextImpl) GetHistorySize() int64 {
	return c.stats.HistorySize
}

func (c *contextImpl) SetHistorySize(size int64) {
	c.stats.HistorySize = size
	if c.mutableState != nil {
		c.mutableState.SetHistorySize(size)
	}
}

func (c *contextImpl) LoadExecutionStats(
	ctx context.Context,
) (*persistence.ExecutionStats, error) {
	_, err := c.LoadWorkflowExecution(ctx)
	if err != nil {
		return nil, err
	}
	return c.stats, nil
}

func isChecksumError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "checksum mismatch error")
}

func (c *contextImpl) LoadWorkflowExecutionWithTaskVersion(
	ctx context.Context,
	incomingVersion int64,
) (MutableState, error) {
	domainEntry, err := c.shard.GetDomainCache().GetDomainByID(c.domainID)
	if err != nil {
		return nil, err
	}

	if c.mutableState == nil {
		var response *persistence.GetWorkflowExecutionResponse
		for i := 0; i < checksumErrorRetryCount; i++ {
			response, err = c.getWorkflowExecutionFn(ctx, &persistence.GetWorkflowExecutionRequest{
				DomainID:   c.domainID,
				Execution:  c.workflowExecution,
				DomainName: domainEntry.GetInfo().Name,
			})
			if err != nil {
				return nil, err
			}
			c.mutableState = c.createMutableStateFn(c.shard, c.logger, domainEntry)
			err = c.mutableState.Load(ctx, response.State)
			if err == nil {
				break
			} else if !isChecksumError(err) {
				c.logger.Error("failed to load mutable state", tag.Error(err))
				break
			}
			// backoff before retry
			c.shard.GetTimeSource().Sleep(time.Millisecond * 100)
		}
		if isChecksumError(err) {
			c.metricsClient.IncCounter(metrics.WorkflowContextScope, metrics.StaleMutableStateCounter)
			c.logger.Error("encounter stale mutable state after retry", tag.Error(err))
		}

		c.stats = response.State.ExecutionStats

		// finally emit execution and session stats
		c.emitWorkflowExecutionStatsFn(domainEntry.GetInfo().Name, response.MutableStateStats, c.stats.HistorySize)
	}
	flushBeforeReady, err := c.mutableState.StartTransaction(ctx, domainEntry, incomingVersion)
	if err != nil {
		return nil, err
	}
	if !flushBeforeReady {
		return c.mutableState, nil
	}
	c.logger.Debug("LoadWorkflowExecutionWithTaskVersion calling UpdateWorkflowExecutionAsActive", tag.WorkflowID(c.workflowExecution.GetWorkflowID()))
	if err = c.UpdateWorkflowExecutionAsActive(ctx, c.shard.GetTimeSource().Now()); err != nil {
		return nil, err
	}
	flushBeforeReady, err = c.mutableState.StartTransaction(ctx, domainEntry, incomingVersion)
	if err != nil {
		return nil, err
	}
	if flushBeforeReady {
		return nil, &types.InternalServiceError{
			Message: "workflowExecutionContext counter flushBeforeReady status after loading mutable state from DB",
		}
	}
	return c.mutableState, nil
}

// GetWorkflowExecution should only be used in tests
func (c *contextImpl) GetWorkflowExecution() MutableState {
	return c.mutableState
}

// SetWorkflowExecution should only be used in tests
func (c *contextImpl) SetWorkflowExecution(mutableState MutableState) {
	c.mutableState = mutableState
}

func (c *contextImpl) LoadWorkflowExecution(
	ctx context.Context,
) (MutableState, error) {
	// Use empty version to skip incoming task version validation
	return c.LoadWorkflowExecutionWithTaskVersion(ctx, constants.EmptyVersion)
}

func (c *contextImpl) CreateWorkflowExecution(
	ctx context.Context,
	newWorkflow *persistence.WorkflowSnapshot,
	persistedHistory events.PersistedBlob,
	createMode persistence.CreateWorkflowMode,
	prevRunID string,
	prevLastWriteVersion int64,
	workflowRequestMode persistence.CreateWorkflowRequestMode,
) (retError error) {

	defer func() {
		if retError != nil {
			c.Clear()
		}
	}()
	domain, errorDomainName := c.shard.GetDomainCache().GetDomainName(c.domainID)
	if errorDomainName != nil {
		return errorDomainName
	}
	err := validateWorkflowRequestsAndMode(newWorkflow.WorkflowRequests, workflowRequestMode)
	if err != nil {
		if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
			return err
		}
		c.logger.Error("workflow requests and mode validation error", tag.Error(err))
	}
	createRequest := &persistence.CreateWorkflowExecutionRequest{
		// workflow create mode & prev run ID & version
		Mode:                     createMode,
		PreviousRunID:            prevRunID,
		PreviousLastWriteVersion: prevLastWriteVersion,
		NewWorkflowSnapshot:      *newWorkflow,
		WorkflowRequestMode:      workflowRequestMode,
		DomainName:               domain,
	}

	historySize := int64(len(persistedHistory.Data))
	historySize += c.GetHistorySize()
	c.SetHistorySize(historySize)
	createRequest.NewWorkflowSnapshot.ExecutionStats = &persistence.ExecutionStats{
		HistorySize: historySize,
	}

	resp, err := c.createWorkflowExecutionFn(ctx, createRequest)
	if err != nil {
		if isOperationPossiblySuccessfulError(err) {
			c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, events.PersistedBlobs{persistedHistory}, true)
		}
		return err
	}

	c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, events.PersistedBlobs{persistedHistory}, false)

	// finally emit session stats
	c.emitSessionUpdateStatsFn(domain, resp.MutableStateUpdateSessionStats)

	return nil
}

func (c *contextImpl) ConflictResolveWorkflowExecution(
	ctx context.Context,
	now time.Time,
	conflictResolveMode persistence.ConflictResolveWorkflowMode,
	resetMutableState MutableState,
	newContext Context,
	newMutableState MutableState,
	currentContext Context,
	currentMutableState MutableState,
	currentTransactionPolicy *TransactionPolicy,
) (retError error) {
	defer func() {
		if retError != nil {
			c.Clear()
		}
	}()

	resetWorkflow, resetWorkflowEventsSeq, err := resetMutableState.CloseTransactionAsSnapshot(now, TransactionPolicyPassive)
	if err != nil {
		return err
	}

	domain, errorDomainName := c.shard.GetDomainCache().GetDomainName(c.domainID)
	if errorDomainName != nil {
		return errorDomainName
	}
	var persistedBlobs events.PersistedBlobs
	resetHistorySize := c.GetHistorySize()
	for _, workflowEvents := range resetWorkflowEventsSeq {
		blob, err := c.persistNonStartWorkflowBatchEventsFn(ctx, workflowEvents)
		if err != nil {
			return err
		}
		resetHistorySize += int64(len(blob.Data))
		persistedBlobs = append(persistedBlobs, blob)
	}
	c.SetHistorySize(resetHistorySize)
	resetWorkflow.ExecutionStats = &persistence.ExecutionStats{
		HistorySize: resetHistorySize,
	}

	var newWorkflow *persistence.WorkflowSnapshot
	var newWorkflowEventsSeq []*persistence.WorkflowEvents
	if newContext != nil && newMutableState != nil {
		defer func() {
			if retError != nil {
				newContext.Clear()
			}
		}()
		newWorkflow, newWorkflowEventsSeq, err = newMutableState.CloseTransactionAsSnapshot(now, TransactionPolicyPassive)
		if err != nil {
			return err
		}
		if len(resetWorkflow.WorkflowRequests) != 0 && len(newWorkflow.WorkflowRequests) != 0 {
			if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
				return &types.InternalServiceError{Message: "workflow requests are only expected to be generated from either reset workflow or continue-as-new workflow for ConflictResolveWorkflowExecution"}
			}
			c.logger.Error("workflow requests are only expected to be generated from either reset workflow or continue-as-new workflow for ConflictResolveWorkflowExecution", tag.Number(int64(len(resetWorkflow.WorkflowRequests))), tag.NextNumber(int64(len(newWorkflow.WorkflowRequests))))
		}
		newWorkflowSizeSize := newContext.GetHistorySize()
		startEvents := newWorkflowEventsSeq[0]
		blob, err := c.persistStartWorkflowBatchEventsFn(ctx, startEvents)
		if err != nil {
			return err
		}
		newWorkflowSizeSize += int64(len(blob.Data))
		newContext.SetHistorySize(newWorkflowSizeSize)
		newWorkflow.ExecutionStats = &persistence.ExecutionStats{
			HistorySize: newWorkflowSizeSize,
		}
		persistedBlobs = append(persistedBlobs, blob)
	}

	var currentWorkflow *persistence.WorkflowMutation
	var currentWorkflowEventsSeq []*persistence.WorkflowEvents
	if currentContext != nil && currentMutableState != nil && currentTransactionPolicy != nil {
		defer func() {
			if retError != nil {
				currentContext.Clear()
			}
		}()
		c.logger.Debug("ConflictResolveWorkflowExecution calling CloseTransactionAsMutation", tag.WorkflowID(c.workflowExecution.GetWorkflowID()), tag.Dynamic("policy", *currentTransactionPolicy))
		currentWorkflow, currentWorkflowEventsSeq, err = currentMutableState.CloseTransactionAsMutation(now, *currentTransactionPolicy)
		if err != nil {
			return err
		}
		if len(currentWorkflow.WorkflowRequests) != 0 {
			if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
				return &types.InternalServiceError{Message: "workflow requests are not expected from current workflow for ConflictResolveWorkflowExecution"}
			}
			c.logger.Error("workflow requests are not expected from current workflow for ConflictResolveWorkflowExecution", tag.Counter(len(currentWorkflow.WorkflowRequests)))
		}
		currentWorkflowSize := currentContext.GetHistorySize()
		for _, workflowEvents := range currentWorkflowEventsSeq {
			blob, err := c.persistNonStartWorkflowBatchEventsFn(ctx, workflowEvents)
			if err != nil {
				return err
			}
			currentWorkflowSize += int64(len(blob.Data))
			persistedBlobs = append(persistedBlobs, blob)
		}
		currentContext.SetHistorySize(currentWorkflowSize)
		currentWorkflow.ExecutionStats = &persistence.ExecutionStats{
			HistorySize: currentWorkflowSize,
		}
	}

	if err := c.conflictResolveEventReapplyFn(
		conflictResolveMode,
		resetWorkflowEventsSeq,
		newWorkflowEventsSeq,
		// current workflow events will not participate in the events reapplication
	); err != nil {
		return err
	}
	resp, err := c.shard.ConflictResolveWorkflowExecution(ctx, &persistence.ConflictResolveWorkflowExecutionRequest{
		// RangeID , this is set by shard context
		Mode:                    conflictResolveMode,
		ResetWorkflowSnapshot:   *resetWorkflow,
		NewWorkflowSnapshot:     newWorkflow,
		CurrentWorkflowMutation: currentWorkflow,
		WorkflowRequestMode:     persistence.CreateWorkflowRequestModeReplicated,
		// Encoding, this is set by shard context
		DomainName: domain,
	})
	if err != nil {
		if isOperationPossiblySuccessfulError(err) {
			c.notifyTasksFromWorkflowSnapshotFn(resetWorkflow, persistedBlobs, true)
			c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, persistedBlobs, true)
			c.notifyTasksFromWorkflowMutationFn(currentWorkflow, persistedBlobs, true)
		}
		return err
	}

	workflowState, workflowCloseState := resetMutableState.GetWorkflowStateCloseStatus()
	// Current branch changed and notify the watchers
	c.shard.GetEngine().NotifyNewHistoryEvent(events.NewNotification(
		c.domainID,
		&c.workflowExecution,
		resetMutableState.GetLastFirstEventID(),
		resetMutableState.GetNextEventID(),
		resetMutableState.GetPreviousStartedEventID(),
		workflowState,
		workflowCloseState,
		resetMutableState.GetVersionHistories().Duplicate(),
	))

	c.notifyTasksFromWorkflowSnapshotFn(resetWorkflow, persistedBlobs, false)
	c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, persistedBlobs, false)
	c.notifyTasksFromWorkflowMutationFn(currentWorkflow, persistedBlobs, false)

	// finally emit session stats
	c.emitWorkflowHistoryStatsFn(domain, int(c.stats.HistorySize), int(resetMutableState.GetNextEventID()-1))
	c.emitSessionUpdateStatsFn(domain, resp.MutableStateUpdateSessionStats)
	// emit workflow completion stats if any
	if resetWorkflow.ExecutionInfo.State == persistence.WorkflowStateCompleted {
		if event, err := resetMutableState.GetCompletionEvent(ctx); err == nil {
			workflowType := resetWorkflow.ExecutionInfo.WorkflowTypeName
			taskList := resetWorkflow.ExecutionInfo.TaskList
			c.emitWorkflowCompletionStatsFn(domain, workflowType, c.workflowExecution.GetWorkflowID(), c.workflowExecution.GetRunID(), taskList, event)
		}
	}
	return nil
}

func (c *contextImpl) UpdateWorkflowExecutionAsActive(
	ctx context.Context,
	now time.Time,
) error {
	c.logger.Debug("UpdateWorkflowExecutionAsActive calling UpdateWorkflowExecutionWithNew",
		tag.WorkflowID(c.workflowExecution.GetWorkflowID()),
		tag.Dynamic("current policy", TransactionPolicyActive),
		tag.Dynamic("new policy", nil),
	)
	return c.updateWorkflowExecutionWithNewFn(ctx, now, persistence.UpdateWorkflowModeUpdateCurrent, nil, nil, TransactionPolicyActive, nil, persistence.CreateWorkflowRequestModeNew)
}

func (c *contextImpl) UpdateWorkflowExecutionWithNewAsActive(
	ctx context.Context,
	now time.Time,
	newContext Context,
	newMutableState MutableState,
) error {
	c.logger.Debug("UpdateWorkflowExecutionWithNewAsActive calling UpdateWorkflowExecutionWithNew",
		tag.WorkflowID(c.workflowExecution.GetWorkflowID()),
		tag.Dynamic("current policy", TransactionPolicyActive),
		tag.Dynamic("new policy", TransactionPolicyActive),
	)
	return c.updateWorkflowExecutionWithNewFn(ctx, now, persistence.UpdateWorkflowModeUpdateCurrent, newContext, newMutableState, TransactionPolicyActive, TransactionPolicyActive.Ptr(), persistence.CreateWorkflowRequestModeNew)
}

func (c *contextImpl) UpdateWorkflowExecutionAsPassive(
	ctx context.Context,
	now time.Time,
) error {
	c.logger.Debug("UpdateWorkflowExecutionAsPassive calling UpdateWorkflowExecutionWithNew",
		tag.WorkflowID(c.workflowExecution.GetWorkflowID()),
		tag.Dynamic("current policy", TransactionPolicyPassive),
		tag.Dynamic("new policy", nil),
	)
	return c.updateWorkflowExecutionWithNewFn(ctx, now, persistence.UpdateWorkflowModeUpdateCurrent, nil, nil, TransactionPolicyPassive, nil, persistence.CreateWorkflowRequestModeReplicated)
}

func (c *contextImpl) UpdateWorkflowExecutionWithNewAsPassive(
	ctx context.Context,
	now time.Time,
	newContext Context,
	newMutableState MutableState,
) error {
	c.logger.Debug("UpdateWorkflowExecutionWithNewAsPassive calling UpdateWorkflowExecutionWithNew",
		tag.WorkflowID(c.workflowExecution.GetWorkflowID()),
		tag.Dynamic("current policy", TransactionPolicyPassive),
		tag.Dynamic("new policy", TransactionPolicyPassive),
	)
	return c.updateWorkflowExecutionWithNewFn(ctx, now, persistence.UpdateWorkflowModeUpdateCurrent, newContext, newMutableState, TransactionPolicyPassive, TransactionPolicyPassive.Ptr(), persistence.CreateWorkflowRequestModeReplicated)
}

func (c *contextImpl) UpdateWorkflowExecutionTasks(
	ctx context.Context,
	now time.Time,
) (retError error) {
	defer func() {
		if retError != nil {
			c.Clear()
		}
	}()

	c.logger.Debug("UpdateWorkflowExecutionTask calling CloseTransactionAsMutation", tag.WorkflowID(c.workflowExecution.GetWorkflowID()))
	currentWorkflow, currentWorkflowEventsSeq, err := c.mutableState.CloseTransactionAsMutation(now, TransactionPolicyPassive)
	if err != nil {
		return err
	}

	if len(currentWorkflowEventsSeq) != 0 {
		return &types.InternalServiceError{
			Message: "UpdateWorkflowExecutionTask can only be used for persisting new workflow tasks, but found new history events",
		}
	}
	domainName, errorDomainName := c.shard.GetDomainCache().GetDomainName(c.domainID)
	if errorDomainName != nil {
		return errorDomainName
	}
	if len(currentWorkflow.WorkflowRequests) != 0 {
		if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domainName) {
			return &types.InternalServiceError{Message: "UpdateWorkflowExecutionTask can only be used for persisting new workflow tasks, but found new workflow requests"}
		}
		c.logger.Error("UpdateWorkflowExecutionTask can only be used for persisting new workflow tasks, but found new workflow requests", tag.Counter(len(currentWorkflow.WorkflowRequests)))
	}
	currentWorkflow.ExecutionStats = &persistence.ExecutionStats{
		HistorySize: c.GetHistorySize(),
	}
	resp, err := c.updateWorkflowExecutionFn(ctx, &persistence.UpdateWorkflowExecutionRequest{
		// RangeID , this is set by shard context
		Mode:                   persistence.UpdateWorkflowModeIgnoreCurrent,
		UpdateWorkflowMutation: *currentWorkflow,
		// Encoding, this is set by shard context
		DomainName: domainName,
	})
	if err != nil {
		if isOperationPossiblySuccessfulError(err) {
			c.notifyTasksFromWorkflowMutationFn(currentWorkflow, nil, true)
		}
		return err
	}
	// notify current workflow tasks
	c.notifyTasksFromWorkflowMutationFn(currentWorkflow, nil, false)
	c.emitSessionUpdateStatsFn(domainName, resp.MutableStateUpdateSessionStats)
	return nil
}

func (c *contextImpl) UpdateWorkflowExecutionWithNew(
	ctx context.Context,
	now time.Time,
	updateMode persistence.UpdateWorkflowMode,
	newContext Context,
	newMutableState MutableState,
	currentWorkflowTransactionPolicy TransactionPolicy,
	newWorkflowTransactionPolicy *TransactionPolicy,
	workflowRequestMode persistence.CreateWorkflowRequestMode,
) (retError error) {
	defer func() {
		if retError != nil {
			c.Clear()
		}
	}()

	c.logger.Debug("UpdateWorkflowExecutionWithNew calling CloseTransactionAsMutation", tag.WorkflowID(c.workflowExecution.GetWorkflowID()), tag.Dynamic("policy", currentWorkflowTransactionPolicy))

	currentWorkflow, currentWorkflowEventsSeq, err := c.mutableState.CloseTransactionAsMutation(now, currentWorkflowTransactionPolicy)
	if err != nil {
		return err
	}
	domain, errorDomainName := c.shard.GetDomainCache().GetDomainName(c.domainID)
	if errorDomainName != nil {
		return errorDomainName
	}
	err = validateWorkflowRequestsAndMode(currentWorkflow.WorkflowRequests, workflowRequestMode)
	if err != nil {
		if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
			return err
		}
		c.logger.Error("workflow requests and mode validation error", tag.Error(err))
	}
	var persistedBlobs events.PersistedBlobs
	currentWorkflowSize := c.GetHistorySize()
	oldWorkflowSize := currentWorkflowSize
	currentWorkflowHistoryCount := c.mutableState.GetNextEventID() - 1
	oldWorkflowHistoryCount := currentWorkflowHistoryCount
	for _, workflowEvents := range currentWorkflowEventsSeq {
		blob, err := c.persistNonStartWorkflowBatchEventsFn(ctx, workflowEvents)
		if err != nil {
			return err
		}
		currentWorkflowHistoryCount += int64(len(workflowEvents.Events))
		currentWorkflowSize += int64(len(blob.Data))
		persistedBlobs = append(persistedBlobs, blob)
	}
	c.SetHistorySize(currentWorkflowSize)
	currentWorkflow.ExecutionStats = &persistence.ExecutionStats{
		HistorySize: currentWorkflowSize,
	}

	var newWorkflow *persistence.WorkflowSnapshot
	var newWorkflowEventsSeq []*persistence.WorkflowEvents
	if newContext != nil && newMutableState != nil && newWorkflowTransactionPolicy != nil {
		defer func() {
			if retError != nil {
				newContext.Clear()
			}
		}()
		newWorkflow, newWorkflowEventsSeq, err = newMutableState.CloseTransactionAsSnapshot(
			now,
			*newWorkflowTransactionPolicy,
		)
		if err != nil {
			return err
		}
		if len(newWorkflow.WorkflowRequests) != 0 && len(currentWorkflow.WorkflowRequests) != 0 {
			if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
				return &types.InternalServiceError{Message: "workflow requests are only expected to be generated from one workflow for UpdateWorkflowExecution"}
			}
			c.logger.Error("workflow requests are only expected to be generated from one workflow for UpdateWorkflowExecution", tag.Number(int64(len(currentWorkflow.WorkflowRequests))), tag.NextNumber(int64(len(newWorkflow.WorkflowRequests))))
		}

		err := validateWorkflowRequestsAndMode(newWorkflow.WorkflowRequests, workflowRequestMode)
		if err != nil {
			if c.shard.GetConfig().EnableStrongIdempotencySanityCheck(domain) {
				return err
			}
			c.logger.Error("workflow requests and mode validation error", tag.Error(err))
		}
		newWorkflowSizeSize := newContext.GetHistorySize()
		startEvents := newWorkflowEventsSeq[0]
		firstEventID := startEvents.Events[0].ID
		var blob events.PersistedBlob
		if firstEventID == constants.FirstEventID {
			blob, err = c.persistStartWorkflowBatchEventsFn(ctx, startEvents)
			if err != nil {
				return err
			}
		} else {
			// NOTE: This is the case for reset workflow, reset workflow already inserted a branch record
			blob, err = c.persistNonStartWorkflowBatchEventsFn(ctx, startEvents)
			if err != nil {
				return err
			}
		}

		persistedBlobs = append(persistedBlobs, blob)
		newWorkflowSizeSize += int64(len(blob.Data))
		newContext.SetHistorySize(newWorkflowSizeSize)
		newWorkflow.ExecutionStats = &persistence.ExecutionStats{
			HistorySize: newWorkflowSizeSize,
		}
	}

	if err := c.mergeContinueAsNewReplicationTasksFn(updateMode, currentWorkflow, newWorkflow); err != nil {
		return err
	}

	if err := c.updateWorkflowExecutionEventReapplyFn(updateMode, currentWorkflowEventsSeq, newWorkflowEventsSeq); err != nil {
		return err
	}
	resp, err := c.updateWorkflowExecutionFn(ctx, &persistence.UpdateWorkflowExecutionRequest{
		// RangeID , this is set by shard context
		Mode:                   updateMode,
		UpdateWorkflowMutation: *currentWorkflow,
		NewWorkflowSnapshot:    newWorkflow,
		WorkflowRequestMode:    workflowRequestMode,
		// Encoding, this is set by shard context
		DomainName: domain,
	})
	if err != nil {
		if isOperationPossiblySuccessfulError(err) {
			c.notifyTasksFromWorkflowMutationFn(currentWorkflow, persistedBlobs, true)
			c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, persistedBlobs, true)
		}
		return err
	}

	// for any change in the workflow, send a event
	workflowState, workflowCloseState := c.mutableState.GetWorkflowStateCloseStatus()
	c.shard.GetEngine().NotifyNewHistoryEvent(events.NewNotification(
		c.domainID,
		&c.workflowExecution,
		c.mutableState.GetLastFirstEventID(),
		c.mutableState.GetNextEventID(),
		c.mutableState.GetPreviousStartedEventID(),
		workflowState,
		workflowCloseState,
		c.mutableState.GetVersionHistories().Duplicate(),
	))

	// notify current workflow tasks
	c.notifyTasksFromWorkflowMutationFn(currentWorkflow, persistedBlobs, false)
	// notify new workflow tasks
	c.notifyTasksFromWorkflowSnapshotFn(newWorkflow, persistedBlobs, false)

	// finally emit session stats
	c.emitWorkflowHistoryStatsFn(domain, int(c.stats.HistorySize), int(c.mutableState.GetNextEventID()-1))
	c.emitSessionUpdateStatsFn(domain, resp.MutableStateUpdateSessionStats)
	c.emitLargeWorkflowShardIDStatsFn(currentWorkflowSize-oldWorkflowSize, oldWorkflowHistoryCount, oldWorkflowSize, currentWorkflowHistoryCount)
	// emit workflow completion stats if any
	if currentWorkflow.ExecutionInfo.State == persistence.WorkflowStateCompleted {
		if event, err := c.mutableState.GetCompletionEvent(ctx); err == nil {
			workflowType := currentWorkflow.ExecutionInfo.WorkflowTypeName
			taskList := currentWorkflow.ExecutionInfo.TaskList
			c.emitWorkflowCompletionStatsFn(domain, workflowType, c.workflowExecution.GetWorkflowID(), c.workflowExecution.GetRunID(), taskList, event)
		}
	}

	return nil
}

func notifyTasksFromWorkflowSnapshot(
	engine engine.Engine,
	workflowSnapShot *persistence.WorkflowSnapshot,
	history events.PersistedBlobs,
	persistenceError bool,
) {
	if workflowSnapShot == nil {
		return
	}

	notifyTasks(
		engine,
		workflowSnapShot.ExecutionInfo,
		workflowSnapShot.VersionHistories,
		workflowSnapShot.ActivityInfos,
		workflowSnapShot.TasksByCategory,
		history,
		persistenceError,
	)
}

func notifyTasksFromWorkflowMutation(
	engine engine.Engine,
	workflowMutation *persistence.WorkflowMutation,
	history events.PersistedBlobs,
	persistenceError bool,
) {
	if workflowMutation == nil {
		return
	}

	notifyTasks(
		engine,
		workflowMutation.ExecutionInfo,
		workflowMutation.VersionHistories,
		workflowMutation.UpsertActivityInfos,
		workflowMutation.TasksByCategory,
		history,
		persistenceError,
	)
}

func activityInfosToMap(ais []*persistence.ActivityInfo) map[int64]*persistence.ActivityInfo {
	m := make(map[int64]*persistence.ActivityInfo, len(ais))
	for _, ai := range ais {
		m[ai.ScheduleID] = ai
	}
	return m
}

func notifyTasks(
	engine engine.Engine,
	executionInfo *persistence.WorkflowExecutionInfo,
	versionHistories *persistence.VersionHistories,
	activities []*persistence.ActivityInfo,
	tasksByCategory map[persistence.HistoryTaskCategory][]persistence.Task,
	history events.PersistedBlobs,
	persistenceError bool,
) {
	transferTaskInfo := &hcommon.NotifyTaskInfo{
		ExecutionInfo:    executionInfo,
		Tasks:            tasksByCategory[persistence.HistoryTaskCategoryTransfer],
		PersistenceError: persistenceError,
	}
	timerTaskInfo := &hcommon.NotifyTaskInfo{
		ExecutionInfo:    executionInfo,
		Tasks:            tasksByCategory[persistence.HistoryTaskCategoryTimer],
		PersistenceError: persistenceError,
		ScheduleInMemory: true,
	}
	replicationTaskInfo := &hcommon.NotifyTaskInfo{
		ExecutionInfo:    executionInfo,
		Tasks:            tasksByCategory[persistence.HistoryTaskCategoryReplication],
		VersionHistories: versionHistories,
		Activities:       activityInfosToMap(activities),
		History:          history,
		PersistenceError: persistenceError,
	}

	// TODO: unify these methods
	engine.NotifyNewTransferTasks(transferTaskInfo)
	engine.NotifyNewTimerTasks(timerTaskInfo)
	engine.NotifyNewReplicationTasks(replicationTaskInfo)
}

func mergeContinueAsNewReplicationTasks(
	logger log.Logger,
	updateMode persistence.UpdateWorkflowMode,
	currentWorkflowMutation *persistence.WorkflowMutation,
	newWorkflowSnapshot *persistence.WorkflowSnapshot,
) error {

	if currentWorkflowMutation.ExecutionInfo.CloseStatus != persistence.WorkflowCloseStatusContinuedAsNew {
		return nil
	} else if updateMode == persistence.UpdateWorkflowModeBypassCurrent && newWorkflowSnapshot == nil {
		// update current workflow as zombie & continue as new without new zombie workflow
		// this case can be valid if new workflow is already created by resend
		return nil
	}

	// current workflow is doing continue as new

	// it is possible that continue as new is done as part of passive logic
	if len(currentWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryReplication]) == 0 {
		logger.Debug("mergeContinueAsNewReplicationTasks: no replication task",
			tag.WorkflowDomainID(currentWorkflowMutation.ExecutionInfo.DomainID),
			tag.WorkflowID(currentWorkflowMutation.ExecutionInfo.WorkflowID),
			tag.WorkflowRunID(currentWorkflowMutation.ExecutionInfo.RunID),
		)
		return nil
	}

	if newWorkflowSnapshot == nil || len(newWorkflowSnapshot.TasksByCategory[persistence.HistoryTaskCategoryReplication]) != 1 {
		return &types.InternalServiceError{
			Message: "unable to find replication task from new workflow for continue as new replication",
		}
	}

	// merge the new run first event batch replication task
	// to current event batch replication task
	newRunTask := newWorkflowSnapshot.TasksByCategory[persistence.HistoryTaskCategoryReplication][0].(*persistence.HistoryReplicationTask)
	newWorkflowSnapshot.TasksByCategory[persistence.HistoryTaskCategoryReplication] = nil

	newRunBranchToken := newRunTask.BranchToken
	taskUpdated := false
	for _, replicationTask := range currentWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryReplication] {
		if task, ok := replicationTask.(*persistence.HistoryReplicationTask); ok {
			taskUpdated = true
			task.NewRunBranchToken = newRunBranchToken
			logger.Debug("mergeContinueAsNewReplicationTasks: updated replication task",
				tag.WorkflowDomainID(currentWorkflowMutation.ExecutionInfo.DomainID),
				tag.WorkflowID(currentWorkflowMutation.ExecutionInfo.WorkflowID),
				tag.WorkflowRunID(currentWorkflowMutation.ExecutionInfo.RunID),
				tag.Dynamic("taskid", task.TaskID),
				tag.Dynamic("version", task.Version),
				tag.Dynamic("visibility_ts", task.VisibilityTimestamp.Format(time.RFC3339)),
			)
		}
	}
	if !taskUpdated {
		return &types.InternalServiceError{
			Message: "unable to find replication task from current workflow for continue as new replication",
		}
	}
	return nil
}

func (c *contextImpl) PersistStartWorkflowBatchEvents(
	ctx context.Context,
	workflowEvents *persistence.WorkflowEvents,
) (events.PersistedBlob, error) {

	if len(workflowEvents.Events) == 0 {
		return events.PersistedBlob{}, &types.InternalServiceError{
			Message: "cannot persist first workflow events with empty events",
		}
	}

	domainID := workflowEvents.DomainID
	domainName, err := c.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return events.PersistedBlob{}, err
	}
	workflowID := workflowEvents.WorkflowID
	runID := workflowEvents.RunID
	execution := types.WorkflowExecution{
		WorkflowID: workflowEvents.WorkflowID,
		RunID:      workflowEvents.RunID,
	}

	resp, err := c.appendHistoryNodesFn(
		ctx,
		domainID,
		execution,
		&persistence.AppendHistoryNodesRequest{
			IsNewBranch: true,
			Info:        persistence.BuildHistoryGarbageCleanupInfo(domainID, workflowID, runID),
			BranchToken: workflowEvents.BranchToken,
			Events:      workflowEvents.Events,
			DomainName:  domainName,
			// TransactionID is set by shard context
		},
	)
	if err != nil {
		return events.PersistedBlob{}, err
	}
	return events.PersistedBlob{
		DataBlob:     resp.DataBlob,
		BranchToken:  workflowEvents.BranchToken,
		FirstEventID: workflowEvents.Events[0].ID,
	}, nil
}

func (c *contextImpl) PersistNonStartWorkflowBatchEvents(
	ctx context.Context,
	workflowEvents *persistence.WorkflowEvents,
) (events.PersistedBlob, error) {

	if len(workflowEvents.Events) == 0 {
		return events.PersistedBlob{}, nil // allow update workflow without events
	}

	domainID := workflowEvents.DomainID
	domainName, err := c.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return events.PersistedBlob{}, err
	}
	execution := types.WorkflowExecution{
		WorkflowID: workflowEvents.WorkflowID,
		RunID:      workflowEvents.RunID,
	}

	resp, err := c.appendHistoryNodesFn(
		ctx,
		domainID,
		execution,
		&persistence.AppendHistoryNodesRequest{
			IsNewBranch: false,
			BranchToken: workflowEvents.BranchToken,
			Events:      workflowEvents.Events,
			DomainName:  domainName,
			// TransactionID is set by shard context
		},
	)
	if err != nil {
		return events.PersistedBlob{}, err
	}
	return events.PersistedBlob{
		DataBlob:     resp.DataBlob,
		BranchToken:  workflowEvents.BranchToken,
		FirstEventID: workflowEvents.Events[0].ID,
	}, nil
}

func appendHistoryV2EventsWithRetry(
	ctx context.Context,
	shardContext shard.Context,
	retryPolicy backoff.RetryPolicy,
	domainID string,
	execution types.WorkflowExecution,
	request *persistence.AppendHistoryNodesRequest,
) (*persistence.AppendHistoryNodesResponse, error) {

	var resp *persistence.AppendHistoryNodesResponse
	op := func(ctx context.Context) error {
		var err error
		resp, err = shardContext.AppendHistoryV2Events(ctx, request, domainID, execution)
		return err
	}
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(persistence.IsTransientError),
	)
	err := throttleRetry.Do(ctx, op)
	return resp, err
}

func createWorkflowExecutionWithRetry(
	ctx context.Context,
	shardContext shard.Context,
	logger log.Logger,
	retryPolicy backoff.RetryPolicy,
	request *persistence.CreateWorkflowExecutionRequest,
) (*persistence.CreateWorkflowExecutionResponse, error) {
	logger = logger.Helper()

	var resp *persistence.CreateWorkflowExecutionResponse
	op := func(ctx context.Context) error {
		var err error
		resp, err = shardContext.CreateWorkflowExecution(ctx, request)
		return err
	}
	isRetryable := func(err error) bool {
		if _, ok := err.(*persistence.TimeoutError); ok {
			// TODO: is timeout error retryable for create workflow?
			// if we treat it as retryable, user may receive workflowAlreadyRunning error
			// on the first start workflow execution request.
			return false
		}
		return persistence.IsTransientError(err)
	}
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(isRetryable),
	)

	err := throttleRetry.Do(ctx, op)
	switch err.(type) {
	case nil:
		return resp, nil
	case *persistence.WorkflowExecutionAlreadyStartedError:
		// it is possible that workflow already exists and caller need to apply
		// workflow ID reuse policy
		return nil, err
	default:
		logger.Error(
			"Persistent store operation failure",
			tag.StoreOperationCreateWorkflowExecution,
			tag.Error(err),
		)
		return nil, err
	}
}

func getWorkflowExecutionWithRetry(
	ctx context.Context,
	shardContext shard.Context,
	logger log.Logger,
	retryPolicy backoff.RetryPolicy,
	request *persistence.GetWorkflowExecutionRequest,
) (*persistence.GetWorkflowExecutionResponse, error) {
	logger.Helper()

	var resp *persistence.GetWorkflowExecutionResponse
	op := func(ctx context.Context) error {
		var err error
		resp, err = shardContext.GetWorkflowExecution(ctx, request)
		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(persistence.IsTransientError),
	)
	err := throttleRetry.Do(ctx, op)
	switch err.(type) {
	case nil:
		return resp, nil
	case *types.EntityNotExistsError:
		// it is possible that workflow does not exists
		return nil, err
	default:
		// If error is shard closed, only log error if shard has been closed for a while,
		// otherwise always log
		var shardClosedError *shard.ErrShardClosed
		if !errors.As(err, &shardClosedError) || shardContext.GetTimeSource().Since(shardClosedError.ClosedAt) > shard.TimeBeforeShardClosedIsError {
			logger.Error("Persistent fetch operation failure", tag.StoreOperationGetWorkflowExecution, tag.Error(err))
		}

		return nil, err
	}
}

func updateWorkflowExecutionWithRetry(
	ctx context.Context,
	shardContext shard.Context,
	logger log.Logger,
	retryPolicy backoff.RetryPolicy,
	request *persistence.UpdateWorkflowExecutionRequest,
) (*persistence.UpdateWorkflowExecutionResponse, error) {
	logger.Helper()

	var resp *persistence.UpdateWorkflowExecutionResponse
	op := func(ctx context.Context) error {
		var err error
		resp, err = shardContext.UpdateWorkflowExecution(ctx, request)
		return err
	}
	// Preparation for the task Validation.
	// metricsClient := c.shard.GetMetricsClient()
	// domainCache := c.shard.GetDomainCache()
	// executionManager := c.shard.GetExecutionManager()
	// historymanager := c.shard.GetHistoryManager()
	// zapLogger, _ := zap.NewProduction()
	// checker, _ := taskvalidator.NewWfChecker(zapLogger, metricsClient, domainCache, executionManager, historymanager)

	isRetryable := func(err error) bool {
		if _, ok := err.(*persistence.TimeoutError); ok {
			// timeout error is not retryable for update workflow execution
			return false
		}
		return persistence.IsTransientError(err)
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(isRetryable),
	)
	err := throttleRetry.Do(ctx, op)
	switch err.(type) {
	case nil:
		return resp, nil
	case *persistence.ConditionFailedError:
		return nil, &conflictError{err}
	default:
		logger.Error(
			"Persistent store operation failure",
			tag.StoreOperationUpdateWorkflowExecution,
			tag.Error(err),
			tag.Number(request.UpdateWorkflowMutation.Condition),
		)
		// TODO: Call the Task Validation here so that it happens whenever an error happen during Update.
		// err1 := checker.WorkflowCheckforValidation(
		//	ctx,
		//	c.workflowExecution.GetWorkflowID(),
		//	c.domainID,
		//	c.GetDomainName(),
		//	c.workflowExecution.GetRunID(),
		// )
		// if err1 != nil {
		//	return nil, err1
		// }
		return nil, err
	}
}

func (c *contextImpl) updateWorkflowExecutionEventReapply(
	updateMode persistence.UpdateWorkflowMode,
	eventBatch1 []*persistence.WorkflowEvents,
	eventBatch2 []*persistence.WorkflowEvents,
) error {

	if updateMode != persistence.UpdateWorkflowModeBypassCurrent {
		return nil
	}

	var eventBatches []*persistence.WorkflowEvents
	eventBatches = append(eventBatches, eventBatch1...)
	eventBatches = append(eventBatches, eventBatch2...)
	return c.ReapplyEvents(eventBatches)
}

func (c *contextImpl) conflictResolveEventReapply(
	conflictResolveMode persistence.ConflictResolveWorkflowMode,
	eventBatch1 []*persistence.WorkflowEvents,
	eventBatch2 []*persistence.WorkflowEvents,
) error {

	if conflictResolveMode != persistence.ConflictResolveWorkflowModeBypassCurrent {
		return nil
	}

	var eventBatches []*persistence.WorkflowEvents
	eventBatches = append(eventBatches, eventBatch1...)
	eventBatches = append(eventBatches, eventBatch2...)
	return c.ReapplyEvents(eventBatches)
}

func (c *contextImpl) ReapplyEvents(
	eventBatches []*persistence.WorkflowEvents,
) error {

	// NOTE: this function should only be used to workflow which is
	// not the caller, or otherwise deadlock will appear

	if len(eventBatches) == 0 {
		return nil
	}

	domainID := eventBatches[0].DomainID
	workflowID := eventBatches[0].WorkflowID
	runID := eventBatches[0].RunID
	domainCache := c.shard.GetDomainCache()
	domainEntry, err := domainCache.GetDomainByID(domainID)
	if err != nil {
		return err
	}
	if domainEntry.IsDomainPendingActive() {
		return nil
	}
	var reapplyEvents []*types.HistoryEvent
	for _, events := range eventBatches {
		if events.DomainID != domainID ||
			events.WorkflowID != workflowID {
			return &types.InternalServiceError{
				Message: "workflowExecutionContext encounter mismatch domainID / workflowID in events reapplication.",
			}
		}

		for _, event := range events.Events {
			switch event.GetEventType() {
			case types.EventTypeWorkflowExecutionSignaled:
				reapplyEvents = append(reapplyEvents, event)
			}
		}
	}
	if len(reapplyEvents) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultRemoteCallTimeout)
	defer cancel()

	activeClusterInfo, err := c.shard.GetActiveClusterManager().GetActiveClusterInfoByWorkflow(ctx, domainID, workflowID, runID)
	if err != nil {
		return err
	}

	if activeClusterInfo.ActiveClusterName == c.shard.GetClusterMetadata().GetCurrentClusterName() {
		return c.shard.GetEngine().ReapplyEvents(
			ctx,
			domainID,
			workflowID,
			runID,
			reapplyEvents,
		)
	}

	// Reapply events only reapply to the current run.
	// The run id is only used for reapply event de-duplication
	execution := &types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      runID,
	}
	clientBean := c.shard.GetService().GetClientBean()
	serializer := c.shard.GetService().GetPayloadSerializer()
	// The active cluster of the domain is the same as current cluster.
	// Use the history from the same cluster to reapply events
	reapplyEventsDataBlob, err := serializer.SerializeBatchEvents(
		reapplyEvents,
		constants.EncodingTypeThriftRW,
	)
	if err != nil {
		return err
	}
	// The active cluster of the domain is differ from the current cluster
	// Use frontend client to route this request to the active cluster
	// Reapplication only happens in active cluster
	sourceCluster, err := clientBean.GetRemoteAdminClient(activeClusterInfo.ActiveClusterName)
	if err != nil {
		return &types.InternalServiceError{
			Message: err.Error(),
		}
	}
	return sourceCluster.ReapplyEvents(
		ctx,
		&types.ReapplyEventsRequest{
			DomainName:        domainEntry.GetInfo().Name,
			WorkflowExecution: execution,
			Events:            reapplyEventsDataBlob.ToInternal(),
		},
	)
}

func (c *contextImpl) ByteSize() uint64 {

	var size int
	// Estimate size of strings
	size += len(c.domainID)

	size += len(c.workflowExecution.GetWorkflowID()) + len(c.workflowExecution.GetRunID())
	size += 3 * constants.StringSizeOverheadBytes

	size += 3 * 8 // logger
	size += 512   // MetricsClient estimation
	size += 256   // ExecutionManager estimation
	size += 8     // Mutex
	size += 1024  // Mutable-state estimation
	size += 8     // stats pointer

	size += 18 * 8 // 18 function pointers with 8 bytes each
	return uint64(size)
}

func isOperationPossiblySuccessfulError(err error) bool {
	switch err.(type) {
	case nil:
		return false
	case *types.WorkflowExecutionAlreadyStartedError,
		*persistence.WorkflowExecutionAlreadyStartedError,
		*persistence.CurrentWorkflowConditionFailedError,
		*persistence.ConditionFailedError,
		*types.ServiceBusyError,
		*types.LimitExceededError,
		*persistence.ShardOwnershipLostError:
		return false
	case *persistence.TimeoutError:
		return true
	default:
		return !IsConflictError(err)
	}
}

func validateWorkflowRequestsAndMode(requests []*persistence.WorkflowRequest, mode persistence.CreateWorkflowRequestMode) error {
	if mode != persistence.CreateWorkflowRequestModeNew {
		return nil
	}
	if len(requests) > 2 {
		return &types.InternalServiceError{Message: "too many workflow request entities generated from a single API request"}
	} else if len(requests) == 2 {
		// SignalWithStartWorkflow API can generate 2 workflow requests
		if (requests[0].RequestType == persistence.WorkflowRequestTypeStart && requests[1].RequestType == persistence.WorkflowRequestTypeSignal) ||
			(requests[1].RequestType == persistence.WorkflowRequestTypeStart && requests[0].RequestType == persistence.WorkflowRequestTypeSignal) {
			return nil
		}
		return &types.InternalServiceError{Message: "too many workflow request entities generated from a single API request"}
	}
	return nil
}
