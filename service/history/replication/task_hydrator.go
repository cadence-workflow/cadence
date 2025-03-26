// The MIT License (MIT)
//
// Copyright (c) 2017-2022 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package replication

import (
	"context"
	"errors"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/execution"
)

var errUnknownReplicationTask = errors.New("unknown replication task")

// TaskHydrator will enrich replication task with additional information from mutable state and history events.
// Mutable state and history providers can be either in-memory or persistence based implementations;
// depending whether we have available data already or need to load it.
type TaskHydrator struct {
	msProvider mutableStateProvider
	history    historyProvider
}

type (
	historyProvider interface {
		GetEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error)
		GetNextRunEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error)
	}

	mutableStateProvider interface {
		GetMutableState(ctx context.Context, domainID, workflowID, runID string) (mutableState, execution.ReleaseFunc, error)
	}

	mutableState interface {
		IsWorkflowExecutionRunning() bool
		GetActivityInfo(int64) (*persistence.ActivityInfo, bool)
		GetVersionHistories() *persistence.VersionHistories
	}
)

// NewImmediateTaskHydrator will enrich replication tasks with additional information that is immediately available.
func NewImmediateTaskHydrator(isRunning bool, vh *persistence.VersionHistories, activities map[int64]*persistence.ActivityInfo, blob, nextBlob *persistence.DataBlob) TaskHydrator {
	return TaskHydrator{
		history:    immediateHistoryProvider{blob: blob, nextBlob: nextBlob},
		msProvider: immediateMutableStateProvider{immediateMutableState{isRunning, activities, vh}},
	}
}

// NewDeferredTaskHydrator will enrich replication tasks with additional information that is not available on hand,
// but is rather loaded in a deferred way later from a database and cache.
func NewDeferredTaskHydrator(shardID int, historyManager persistence.HistoryManager, executionCache execution.Cache, domains domainCache) TaskHydrator {
	return TaskHydrator{
		history:    historyLoader{shardID, historyManager, domains},
		msProvider: mutableStateLoader{executionCache},
	}
}

// Hydrate will enrich replication task with additional information from mutable state and history events.
func (h TaskHydrator) Hydrate(ctx context.Context, task persistence.Task) (retTask *types.ReplicationTask, retErr error) {
	switch t := task.(type) {
	case *persistence.FailoverMarkerTask:
		return hydrateFailoverMarkerTask(t), nil
	}

	ms, release, err := h.msProvider.GetMutableState(ctx, task.GetDomainID(), task.GetWorkflowID(), task.GetRunID())
	defer func() {
		if release != nil {
			release(retErr)
		}
	}()

	if common.IsEntityNotExistsError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	switch t := task.(type) {
	case *persistence.SyncActivityTask:
		return hydrateSyncActivityTask(t, ms)
	case *persistence.HistoryReplicationTask:
		versionHistories := ms.GetVersionHistories()
		if versionHistories != nil {
			// Create a copy to release workflow lock early, as hydration will make a DB call, which may take a while
			versionHistories = versionHistories.Duplicate()
		}
		release(nil)
		return hydrateHistoryReplicationTask(ctx, t, versionHistories, h.history)
	default:
		return nil, errUnknownReplicationTask
	}
}

func hydrateFailoverMarkerTask(t *persistence.FailoverMarkerTask) *types.ReplicationTask {
	return &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeFailoverMarker.Ptr(),
		SourceTaskID: t.TaskID,
		FailoverMarkerAttributes: &types.FailoverMarkerAttributes{
			DomainID:        t.DomainID,
			FailoverVersion: t.Version,
		},
		CreationTime: common.Ptr(t.VisibilityTimestamp.UnixNano()),
	}
}

func hydrateSyncActivityTask(task *persistence.SyncActivityTask, ms mutableState) (*types.ReplicationTask, error) {
	if !ms.IsWorkflowExecutionRunning() {
		// workflow already finished, no need to process the replication task
		return nil, nil
	}

	activityInfo, ok := ms.GetActivityInfo(task.ScheduledID)
	if !ok {
		return nil, nil
	}

	var startedTime *int64
	if activityInfo.StartedID != constants.EmptyEventID {
		startedTime = timeToUnixNano(activityInfo.StartedTime)
	}

	// Version history uses when replicate the sync activity task
	var versionHistory *types.VersionHistory
	if versionHistories := ms.GetVersionHistories(); versionHistories != nil {
		currentVersionHistory, err := versionHistories.GetCurrentVersionHistory()
		if err != nil {
			return nil, err
		}
		versionHistory = currentVersionHistory.ToInternalType()
	}

	return &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeSyncActivity.Ptr(),
		SourceTaskID: task.TaskID,
		SyncActivityTaskAttributes: &types.SyncActivityTaskAttributes{
			DomainID:           task.DomainID,
			WorkflowID:         task.WorkflowID,
			RunID:              task.RunID,
			Version:            activityInfo.Version,
			ScheduledID:        activityInfo.ScheduleID,
			ScheduledTime:      timeToUnixNano(activityInfo.ScheduledTime),
			StartedID:          activityInfo.StartedID,
			StartedTime:        startedTime,
			LastHeartbeatTime:  timeToUnixNano(activityInfo.LastHeartBeatUpdatedTime),
			Details:            activityInfo.Details,
			Attempt:            activityInfo.Attempt,
			LastFailureReason:  common.Ptr(activityInfo.LastFailureReason),
			LastWorkerIdentity: activityInfo.LastWorkerIdentity,
			LastFailureDetails: activityInfo.LastFailureDetails,
			VersionHistory:     versionHistory,
		},
		CreationTime: common.Ptr(task.VisibilityTimestamp.UnixNano()),
	}, nil
}

func hydrateHistoryReplicationTask(ctx context.Context, task *persistence.HistoryReplicationTask, versionHistories *persistence.VersionHistories, history historyProvider) (*types.ReplicationTask, error) {
	if versionHistories == nil {
		return nil, nil
	}

	_, versionHistory, err := versionHistories.FindFirstVersionHistoryByItem(persistence.NewVersionHistoryItem(task.FirstEventID, task.Version))
	if err != nil {
		return nil, err
	}

	// BranchToken will not set in get dlq replication message request
	if len(task.BranchToken) == 0 {
		task.BranchToken = versionHistory.GetBranchToken()
	}

	eventsBlob, err := history.GetEventBlob(ctx, task)
	if err != nil {
		return nil, err
	}

	newRunEventsBlob, err := history.GetNextRunEventBlob(ctx, task)
	if err != nil {
		return nil, err
	}

	return &types.ReplicationTask{
		TaskType:     types.ReplicationTaskTypeHistoryV2.Ptr(),
		SourceTaskID: task.TaskID,
		HistoryTaskV2Attributes: &types.HistoryTaskV2Attributes{
			DomainID:            task.DomainID,
			WorkflowID:          task.WorkflowID,
			RunID:               task.RunID,
			VersionHistoryItems: versionHistory.ToInternalType().Items,
			Events:              eventsBlob,
			NewRunEvents:        newRunEventsBlob,
		},
		CreationTime: common.Ptr(task.VisibilityTimestamp.UnixNano()),
	}, nil
}

// historyLoader loads history event blobs on demand from a database
type historyLoader struct {
	shardID int
	history persistence.HistoryManager
	domains domainCache
}

func (h historyLoader) GetEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	return h.getEventsBlob(ctx, task.DomainID, task.BranchToken, task.FirstEventID, task.NextEventID)
}

func (h historyLoader) GetNextRunEventBlob(ctx context.Context, task *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	if len(task.NewRunBranchToken) == 0 {
		return nil, nil
	}
	// only get the first batch
	return h.getEventsBlob(ctx, task.DomainID, task.NewRunBranchToken, constants.FirstEventID, constants.FirstEventID+1)
}

func (h historyLoader) getEventsBlob(ctx context.Context, domainID string, branchToken []byte, minEventID, maxEventID int64) (*types.DataBlob, error) {
	domain, err := h.domains.GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}

	resp, err := h.history.ReadRawHistoryBranch(ctx, &persistence.ReadHistoryBranchRequest{
		BranchToken: branchToken,
		MinEventID:  minEventID,
		MaxEventID:  maxEventID,
		PageSize:    2, // Load more than one to check for data inconsistency errors
		ShardID:     &h.shardID,
		DomainName:  domain.GetInfo().Name,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.HistoryEventBlobs) != 1 {
		return nil, &types.InternalDataInconsistencyError{Message: "replication hydrator encountered more than 1 NDC raw event batch"}
	}

	return resp.HistoryEventBlobs[0].ToInternal(), nil
}

// mutableStateLoader uses workflow execution cache to load mutable state
type mutableStateLoader struct {
	cache execution.Cache
}

func (l mutableStateLoader) GetMutableState(ctx context.Context, domainID, workflowID, runID string) (mutableState, execution.ReleaseFunc, error) {
	wfContext, release, err := l.cache.GetOrCreateWorkflowExecution(ctx, domainID, types.WorkflowExecution{WorkflowID: workflowID, RunID: runID})
	if err != nil {
		return nil, nil, err
	}

	mutableState, err := wfContext.LoadWorkflowExecution(ctx)
	if err != nil {
		release(err)
		return nil, nil, err
	}

	return mutableState, release, nil
}

func timeToUnixNano(t time.Time) *int64 {
	return common.Int64Ptr(t.UnixNano())
}

type immediateHistoryProvider struct {
	blob     *persistence.DataBlob
	nextBlob *persistence.DataBlob
}

func (h immediateHistoryProvider) GetEventBlob(_ context.Context, _ *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	if h.blob == nil {
		return nil, errors.New("history blob not set")
	}
	return h.blob.ToInternal(), nil
}

func (h immediateHistoryProvider) GetNextRunEventBlob(_ context.Context, _ *persistence.HistoryReplicationTask) (*types.DataBlob, error) {
	if h.nextBlob == nil {
		return nil, nil // Expected and common
	}
	return h.nextBlob.ToInternal(), nil
}

type immediateMutableStateProvider struct {
	ms immediateMutableState
}

func (r immediateMutableStateProvider) GetMutableState(_ context.Context, _, _, _ string) (mutableState, execution.ReleaseFunc, error) {
	return r.ms, execution.NoopReleaseFn, nil
}

type immediateMutableState struct {
	isRunning        bool
	activities       map[int64]*persistence.ActivityInfo
	versionHistories *persistence.VersionHistories
}

func (ms immediateMutableState) IsWorkflowExecutionRunning() bool {
	return ms.isRunning
}
func (ms immediateMutableState) GetActivityInfo(id int64) (*persistence.ActivityInfo, bool) {
	info, ok := ms.activities[id]
	return info, ok
}
func (ms immediateMutableState) GetVersionHistories() *persistence.VersionHistories {
	return ms.versionHistories
}
