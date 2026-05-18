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

package replication

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	workerfixer "github.com/uber/cadence/service/worker/fixer"
)

const (
	defaultBeginningMessageID         = -1
	dlqProcessorTimerCoefficient      = 0.05
	dlqProcessorRetryInitialInterval  = 100 * time.Millisecond
)

var (
	errInvalidCluster = &types.BadRequestError{Message: "Invalid target cluster name."}
)

type (
	// DLQHandler is the interface handles replication DLQ messages
	DLQHandler interface {
		common.Daemon

		GetMessageCount(
			ctx context.Context,
			forceFetch bool,
		) (map[string]int64, error)
		ReadMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
			pageSize int,
			pageToken []byte,
		) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error)
		PurgeMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
		) error
		MergeMessages(
			ctx context.Context,
			sourceCluster string,
			lastMessageID int64,
			pageSize int,
			pageToken []byte,
		) ([]byte, error)
	}

	dlqHandlerImpl struct {
		taskExecutors map[string]TaskExecutor
		shard         shard.Context
		config        *config.Config
		logger        log.Logger
		metricsClient metrics.Client
		ctx           context.Context
		cancel        context.CancelFunc
		done          chan struct{}
		wg            sync.WaitGroup
		status        int32
		timeSource    clock.TimeSource
		fixer         workerfixer.Fixer

		mu           sync.Mutex
		latestCounts map[string]int64
	}
)

var _ DLQHandler = (*dlqHandlerImpl)(nil)

// NewDLQHandler initialize the replication message DLQ handler
func NewDLQHandler(
	shard shard.Context,
	taskExecutors map[string]TaskExecutor,
	fixer workerfixer.Fixer,
) DLQHandler {

	if taskExecutors == nil {
		panic("Failed to initialize replication DLQ handler due to nil task executors")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &dlqHandlerImpl{
		shard:         shard,
		taskExecutors: taskExecutors,
		config:        shard.GetConfig(),
		logger:        shard.GetLogger(),
		metricsClient: shard.GetMetricsClient(),
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
		timeSource:    clock.NewRealTimeSource(),
		fixer:         fixer,
	}
}

// Start starts the DLQ handler
func (r *dlqHandlerImpl) Start() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	r.wg.Add(1)
	go r.emitDLQSizeMetricsLoop()
	processorEnabled := r.config.ReplicationDLQProcessorEnabled()
	if processorEnabled {
		r.wg.Add(1)
		go r.processorLoop()
	}
	r.logger.Info("DLQ handler started.", tag.Value(processorEnabled))
}

// Stop stops the DLQ handler
func (r *dlqHandlerImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&r.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	r.logger.Debug("DLQ handler shutting down.")
	r.cancel()
	close(r.done)

	if !common.AwaitWaitGroup(&r.wg, 10*time.Second) {
		r.logger.Warn("DLQ handler timed out on shutdown.")
	}
}

func (r *dlqHandlerImpl) GetMessageCount(ctx context.Context, forceFetch bool) (map[string]int64, error) {
	r.mu.Lock()
	needFetch := forceFetch || r.latestCounts == nil
	r.mu.Unlock()

	if needFetch {
		if err := r.fetchAndEmitMessageCount(ctx); err != nil {
			return nil, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latestCounts, nil
}

func (r *dlqHandlerImpl) fetchAndEmitMessageCount(ctx context.Context) error {
	shardID := strconv.Itoa(r.shard.GetShardID())
	result := map[string]int64{}
	var lastErr error
	for sourceCluster := range r.taskExecutors {
		request := persistence.GetReplicationDLQSizeRequest{SourceClusterName: sourceCluster, ShardID: common.Ptr(r.shard.GetShardID())}
		response, err := r.shard.GetExecutionManager().GetReplicationDLQSize(ctx, &request)
		if err != nil {
			r.logger.Error("failed to get replication DLQ size", tag.SourceCluster(sourceCluster), tag.Error(err))
			r.metricsClient.Scope(metrics.ReplicationDLQStatsScope).IncCounter(metrics.ReplicationDLQProbeFailed)
			lastErr = err
			continue
		}
		r.metricsClient.Scope(
			metrics.ReplicationDLQStatsScope,
			metrics.SourceClusterTag(sourceCluster),
			metrics.InstanceTag(shardID),
		).UpdateGauge(metrics.ReplicationDLQSize, float64(response.Size))

		if response.Size > 0 {
			result[sourceCluster] = response.Size
		}
	}

	r.mu.Lock()
	if lastErr == nil {
		r.latestCounts = result
	}
	r.mu.Unlock()

	return lastErr
}

func (r *dlqHandlerImpl) emitDLQSizeMetricsLoop() {
	defer r.wg.Done()

	getInterval := func() time.Duration {
		return backoff.JitDuration(
			dlqMetricsEmitTimerInterval,
			dlqMetricsEmitTimerCoefficient,
		)
	}

	timer := r.timeSource.NewTimer(getInterval())
	defer timer.Stop()

	for {
		select {
		case <-timer.Chan():
			r.fetchAndEmitMessageCount(r.ctx)
			timer.Reset(getInterval())
		case <-r.done:
			return
		}
	}
}

func (r *dlqHandlerImpl) processorLoop() {
	defer r.wg.Done()

	timer := r.timeSource.NewTimer(initialDLQProcessorDelay(r.config.ReplicationDLQProcessorRescanInterval()))
	defer timer.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-timer.Chan():
			for sourceCluster := range r.taskExecutors {
				r.scanAndProcess(sourceCluster)
			}
			timer.Reset(backoff.JitDuration(
				r.config.ReplicationDLQProcessorRescanInterval(),
				dlqProcessorTimerCoefficient,
			))
		}
	}
}

func initialDLQProcessorDelay(rescanInterval time.Duration) time.Duration {
	if rescanInterval <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(rescanInterval)))
}

func (r *dlqHandlerImpl) scanAndProcess(sourceCluster string) {
	pageSize := r.config.ReplicationDLQProcessorPageSize()
	var pageToken []byte
	for {
		select {
		case <-r.done:
			return
		default:
		}
		resp, err := r.shard.GetExecutionManager().GetReplicationTasksFromDLQ(
			r.ctx,
			&persistence.GetReplicationTasksFromDLQRequest{
				SourceClusterName: sourceCluster,
				ReadLevel:         defaultBeginningMessageID + 1,
				MaxReadLevel:      math.MaxInt64,
				BatchSize:         pageSize,
				NextPageToken:     pageToken,
				ShardID:           common.Ptr(r.shard.GetShardID()),
			},
		)
		if err != nil {
			r.logger.Error("DLQ processor failed to scan tasks",
				tag.SourceCluster(sourceCluster),
				tag.Error(err),
			)
			return
		}
		tasks, _, err := r.hydrateDLQTasks(r.ctx, sourceCluster, resp.Tasks)
		if err != nil {
			r.logger.Error("DLQ processor failed to hydrate tasks",
				tag.SourceCluster(sourceCluster),
				tag.Error(err),
			)
			return
		}
		for _, task := range tasks {
			select {
			case <-r.done:
				return
			default:
			}
			r.processTask(sourceCluster, task)
		}
		pageToken = resp.NextPageToken
		if len(pageToken) == 0 {
			return
		}
	}
}

func (r *dlqHandlerImpl) processTask(sourceCluster string, task *types.ReplicationTask) {
	if task == nil {
		// hydrateDLQTasks already warned for this entry; nothing more to do here.
		return
	}

	domainID, workflowID, runID := task.GetWorkflowIdentifiers()
	taskLogger := r.logger.WithTags(
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(workflowID),
		tag.WorkflowRunID(runID),
		tag.TaskID(task.SourceTaskID),
		tag.SourceCluster(sourceCluster),
	)

	executor, ok := r.taskExecutors[sourceCluster]
	if !ok {
		taskLogger.Error("DLQ processor has no executor for source cluster")
		return
	}

	maxRetries := r.config.ReplicationDLQProcessorMaxRetries()
	retryPolicy := backoff.NewExponentialRetryPolicy(dlqProcessorRetryInitialInterval)
	retryPolicy.SetMaximumAttempts(maxRetries)
	throttle := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(retryPolicy),
		backoff.WithRetryableError(func(error) bool {
			select {
			case <-r.done:
				return false
			default:
				return true
			}
		}),
	)

	attempt := int32(0)
	err := throttle.Do(r.ctx, func(context.Context) error {
		_, execErr := executor.execute(task, true)
		if execErr != nil {
			taskLogger.Warn("DLQ processor failed to execute task",
				tag.Attempt(attempt),
				tag.Error(execErr),
			)
			attempt++
		}
		return execErr
	})

	if err == nil {
		// Success — point-delete so the next rescan skips this task.
		if delErr := r.shard.GetExecutionManager().DeleteReplicationTaskFromDLQ(
			r.ctx,
			&persistence.DeleteReplicationTaskFromDLQRequest{
				SourceClusterName: sourceCluster,
				TaskID:            task.SourceTaskID,
				ShardID:           common.Ptr(r.shard.GetShardID()),
			},
		); delErr != nil {
			taskLogger.Error("DLQ processor failed to delete task after successful execution", tag.Error(delErr))
		}
		return
	}

	// Shutdown is observed by IsRetryable above; bail without invoking fixer.
	select {
	case <-r.done:
		return
	default:
	}

	// All retries exhausted — invoke fixer and move on. Fixer is idempotent:
	// a fix already in progress for this (workflowID, runID) is a no-op. The
	// task stays in the DLQ; the next rescan will retry execution and
	// point-delete if the fix succeeded.
	taskLogger.Error("DLQ processor exhausted retries, invoking fixer",
		tag.Attempt(int32(maxRetries)),
		tag.Error(err),
	)
	if r.fixer == nil {
		return
	}
	if workflowID == "" || runID == "" {
		taskLogger.Warn("DLQ task lacks workflow identifiers, skipping fixer invocation")
		return
	}
	if err := r.fixer.Fix(r.ctx, domainID, workflowID, runID); err != nil {
		taskLogger.Warn("DLQ fixer failed", tag.Error(err))
	}
}

func (r *dlqHandlerImpl) ReadMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {

	return r.readMessagesWithAckLevel(
		ctx,
		sourceCluster,
		lastMessageID,
		pageSize,
		pageToken,
	)
}

func (r *dlqHandlerImpl) readMessagesWithAckLevel(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, []byte, error) {

	resp, err := r.shard.GetExecutionManager().GetReplicationTasksFromDLQ(
		ctx,
		&persistence.GetReplicationTasksFromDLQRequest{
			SourceClusterName: sourceCluster,
			ReadLevel:         defaultBeginningMessageID + 1,
			MaxReadLevel:      lastMessageID + 1,
			BatchSize:         pageSize,
			NextPageToken:     pageToken,
			ShardID:           common.Ptr(r.shard.GetShardID()),
		},
	)
	if err != nil {
		return nil, nil, nil, err
	}

	replicationTasks, taskInfo, err := r.hydrateDLQTasks(ctx, sourceCluster, resp.Tasks)
	if err != nil {
		return nil, nil, nil, err
	}

	return replicationTasks, taskInfo, resp.NextPageToken, nil
}

// hydrateDLQTasks resolves the full replication task payload for each DLQ entry.
// Entries whose payload was not delivered by the persistence layer are fetched
// from the source cluster via GetDLQReplicationMessages. Both returned slices are
// always the same length and parallel — replicationTasks[i] is nil when the task
// could not be resolved (e.g. source workflow deleted).
func (r *dlqHandlerImpl) hydrateDLQTasks(
	ctx context.Context,
	sourceCluster string,
	rawTasks []*persistence.ReplicationDLQTask,
) ([]*types.ReplicationTask, []*types.ReplicationTaskInfo, error) {

	hydrated := make(map[int64]*types.ReplicationTask, len(rawTasks))
	taskInfos := make([]*types.ReplicationTaskInfo, 0, len(rawTasks))
	var needHydration []*types.ReplicationTaskInfo

	for _, task := range rawTasks {
		info := task.Info
		if info == nil {
			return nil, nil, fmt.Errorf("nil task info in DLQ response")
		}
		ti := &types.ReplicationTaskInfo{
			DomainID:     info.DomainID,
			WorkflowID:   info.WorkflowID,
			RunID:        info.RunID,
			TaskType:     int16(info.TaskType),
			TaskID:       info.TaskID,
			Version:      info.Version,
			FirstEventID: info.FirstEventID,
			NextEventID:  info.NextEventID,
			ScheduledID:  info.ScheduledID,
		}
		taskInfos = append(taskInfos, ti)

		if task.Task != nil {
			hydrated[info.TaskID] = task.Task
		} else {
			needHydration = append(needHydration, ti)
		}
	}

	if len(needHydration) > 0 {
		remoteAdminClient, err := r.shard.GetService().GetClientBean().GetRemoteAdminClient(sourceCluster)
		if err != nil {
			return nil, nil, err
		}
		response, err := remoteAdminClient.GetDLQReplicationMessages(
			ctx,
			&types.GetDLQReplicationMessagesRequest{
				TaskInfos: needHydration,
			},
		)
		if err != nil {
			return nil, nil, err
		}
		for _, task := range response.ReplicationTasks {
			hydrated[task.SourceTaskID] = task
		}
	}

	// Build parallel slices of equal length. replicationTasks[i] is nil when the task
	// could not be hydrated (e.g. source workflow deleted) — callers must check for nil.
	replicationTasks := make([]*types.ReplicationTask, len(taskInfos))
	for i, ti := range taskInfos {
		rt, ok := hydrated[ti.TaskID]
		if !ok {
			r.logger.Warn("replication task not found after hydration",
				tag.WorkflowDomainID(ti.DomainID), tag.WorkflowID(ti.WorkflowID), tag.WorkflowRunID(ti.RunID), tag.TaskID(ti.TaskID))
		}
		replicationTasks[i] = rt // nil when not found
	}

	return replicationTasks, taskInfos, nil
}

func (r *dlqHandlerImpl) PurgeMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
) error {

	_, err := r.shard.GetExecutionManager().RangeDeleteReplicationTaskFromDLQ(
		ctx,
		&persistence.RangeDeleteReplicationTaskFromDLQRequest{
			SourceClusterName:    sourceCluster,
			InclusiveBeginTaskID: defaultBeginningMessageID + 1,
			ExclusiveEndTaskID:   lastMessageID + 1,
			ShardID:              common.Ptr(r.shard.GetShardID()),
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *dlqHandlerImpl) MergeMessages(
	ctx context.Context,
	sourceCluster string,
	lastMessageID int64,
	pageSize int,
	pageToken []byte,
) ([]byte, error) {

	if _, ok := r.taskExecutors[sourceCluster]; !ok {
		return nil, errInvalidCluster
	}

	tasks, rawTasks, token, err := r.readMessagesWithAckLevel(
		ctx,
		sourceCluster,
		lastMessageID,
		pageSize,
		pageToken,
	)
	if err != nil {
		return nil, err
	}

	replicationTasks := map[int64]*types.ReplicationTask{}
	for _, task := range tasks {
		if task != nil {
			replicationTasks[task.SourceTaskID] = task
		}
	}

	lastMessageID = defaultBeginningMessageID
	for _, raw := range rawTasks {
		if task, ok := replicationTasks[raw.TaskID]; ok {
			if _, err := r.taskExecutors[sourceCluster].execute(task, true); err != nil {
				return nil, err
			}
		}

		// If hydrated replication task does not exist in remote cluster - continue merging
		// Record lastMessageID with raw task id, so that they can be purged after.
		if lastMessageID < raw.TaskID {
			lastMessageID = raw.TaskID
		}
	}

	_, err = r.shard.GetExecutionManager().RangeDeleteReplicationTaskFromDLQ(
		ctx,
		&persistence.RangeDeleteReplicationTaskFromDLQRequest{
			SourceClusterName:    sourceCluster,
			InclusiveBeginTaskID: defaultBeginningMessageID + 1,
			ExclusiveEndTaskID:   lastMessageID + 1,
			ShardID:              common.Ptr(r.shard.GetShardID()),
		},
	)
	if err != nil {
		return nil, err
	}
	return token, nil
}
