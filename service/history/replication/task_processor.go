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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pborman/uuid"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/reconciliation"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

const (
	dropSyncShardTaskTimeThreshold = 10 * time.Minute
	replicationTimeout             = 30 * time.Second
	dlqErrorRetryWait              = time.Second
	dlqMetricsEmitTimerInterval    = 5 * time.Minute
	dlqMetricsEmitTimerCoefficient = 0.05
)

var (
	// ErrUnknownReplicationTask is the error to indicate unknown replication task type
	ErrUnknownReplicationTask = &types.BadRequestError{Message: "unknown replication task"}
)

type (
	// TaskProcessor is responsible for processing replication tasks for a shard.
	TaskProcessor interface {
		common.Daemon
	}

	// taskProcessorImpl is responsible for processing replication tasks for a shard.
	taskProcessorImpl struct {
		currentCluster    string
		sourceCluster     string
		status            int32
		shard             shard.Context
		historyEngine     engine.Engine
		historySerializer persistence.PayloadSerializer
		config            *config.Config
		metricsClient     metrics.Client
		logger            log.Logger
		taskExecutor      TaskExecutor
		hostRateLimiter   quotas.Limiter
		shardRateLimiter  quotas.Limiter

		taskRetryPolicy backoff.RetryPolicy
		dlqRetryPolicy  backoff.RetryPolicy
		noTaskRetrier   backoff.Retrier

		lastProcessedMessageID int64
		lastRetrievedMessageID int64

		requestChan   chan<- *request
		syncShardChan chan *types.SyncShardStatus
		done          chan struct{}
		wg            sync.WaitGroup
	}

	request struct {
		token    *types.ReplicationToken
		respChan chan<- *types.ReplicationMessages
	}
)

var _ TaskProcessor = (*taskProcessorImpl)(nil)

// NewTaskProcessor creates a new replication task processor.
func NewTaskProcessor(
	shard shard.Context,
	historyEngine engine.Engine,
	config *config.Config,
	metricsClient metrics.Client,
	taskFetcher TaskFetcher,
	taskExecutor TaskExecutor,
	clock clock.TimeSource,
) TaskProcessor {
	shardID := shard.GetShardID()
	sourceCluster := taskFetcher.GetSourceCluster()
	firstRetryPolicy := backoff.NewExponentialRetryPolicy(config.ReplicationTaskProcessorErrorRetryWait(shardID))
	firstRetryPolicy.SetMaximumAttempts(config.ReplicationTaskProcessorErrorRetryMaxAttempts(shardID))
	secondRetryPolicy := backoff.NewExponentialRetryPolicy(config.ReplicationTaskProcessorErrorSecondRetryWait(shardID))
	secondRetryPolicy.SetMaximumInterval(config.ReplicationTaskProcessorErrorSecondRetryMaxWait(shardID))
	secondRetryPolicy.SetExpirationInterval(config.ReplicationTaskProcessorErrorSecondRetryExpiration(shardID))
	taskRetryPolicy := backoff.NewMultiPhasesRetryPolicy(firstRetryPolicy, secondRetryPolicy)

	dlqRetryPolicy := backoff.NewExponentialRetryPolicy(dlqErrorRetryWait)
	dlqRetryPolicy.SetExpirationInterval(backoff.NoInterval)

	noTaskBackoffPolicy := backoff.NewExponentialRetryPolicy(config.ReplicationTaskProcessorNoTaskRetryWait(shardID))
	noTaskBackoffPolicy.SetBackoffCoefficient(1)
	noTaskBackoffPolicy.SetExpirationInterval(backoff.NoInterval)
	noTaskRetrier := backoff.NewRetrier(noTaskBackoffPolicy, clock)
	return &taskProcessorImpl{
		currentCluster:         shard.GetClusterMetadata().GetCurrentClusterName(),
		sourceCluster:          sourceCluster,
		status:                 common.DaemonStatusInitialized,
		shard:                  shard,
		historyEngine:          historyEngine,
		historySerializer:      persistence.NewPayloadSerializer(),
		config:                 config,
		metricsClient:          metricsClient,
		logger:                 shard.GetLogger().WithTags(tag.SourceCluster(sourceCluster), tag.ShardID(shardID)),
		taskExecutor:           taskExecutor,
		hostRateLimiter:        taskFetcher.GetRateLimiter(),
		shardRateLimiter:       quotas.NewDynamicRateLimiter(config.ReplicationTaskProcessorShardQPS.AsFloat64()),
		taskRetryPolicy:        taskRetryPolicy,
		dlqRetryPolicy:         dlqRetryPolicy,
		noTaskRetrier:          noTaskRetrier,
		requestChan:            taskFetcher.GetRequestChan(shardID),
		syncShardChan:          make(chan *types.SyncShardStatus, 1),
		done:                   make(chan struct{}),
		lastProcessedMessageID: constants.EmptyMessageID,
		lastRetrievedMessageID: constants.EmptyMessageID,
	}
}

// Start starts the processor
func (p *taskProcessorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	p.wg.Add(3)
	go p.processorLoop()
	go p.syncShardStatusLoop()
	go p.cleanupReplicationTaskLoop()
	p.logger.Info("ReplicationTaskProcessor started.")
}

// Stop stops the processor
func (p *taskProcessorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&p.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	p.logger.Debug("ReplicationTaskProcessor shutting down.")
	close(p.done)

	if !common.AwaitWaitGroup(&p.wg, 10*time.Second) {
		p.logger.Warn("ReplicationTaskProcessor timed out on shutdown.")
	} else {
		p.logger.Info("ReplicationTaskProcessor shutdown completed")
	}
}

func (p *taskProcessorImpl) processorLoop() {
	defer func() {
		p.logger.Debug("Closing replication task processor.", tag.ReadLevel(p.lastRetrievedMessageID))
		p.wg.Done()
	}()

Loop:
	for {
		respChan := make(chan *types.ReplicationMessages, 1)
		// TODO: when we support prefetching, LastRetrievedMessageID can be different than LastProcessedMessageID

		if p.isShuttingDown() {
			return
		}

		select {
		case <-p.done:
			// shard is closing
			return
		case p.requestChan <- &request{
			token: &types.ReplicationToken{
				ShardID:                int32(p.shard.GetShardID()),
				LastRetrievedMessageID: p.lastRetrievedMessageID,
				LastProcessedMessageID: p.lastProcessedMessageID,
			},
			respChan: respChan,
		}:
			// signal sent, continue to process replication messages
		}

		select {
		case response, ok := <-respChan:
			if !ok {
				p.logger.Debug("Fetch replication messages chan closed.")
				continue Loop
			}

			p.logger.Debug("Got fetch replication messages response.",
				tag.ReadLevel(response.GetLastRetrievedMessageID()),
				tag.Bool(response.GetHasMore()),
				tag.Counter(len(response.GetReplicationTasks())),
			)

			p.taskProcessingStartWait()
			p.processResponse(response)
		case <-p.done:
			return
		}
	}
}

func (p *taskProcessorImpl) cleanupReplicationTaskLoop() {
	defer p.wg.Done()

	shardID := p.shard.GetShardID()
	timer := time.NewTimer(backoff.JitDuration(
		p.config.ReplicationTaskProcessorCleanupInterval(shardID),
		p.config.ReplicationTaskProcessorCleanupJitterCoefficient(shardID),
	))
	defer timer.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-timer.C:
			if err := p.cleanupAckedReplicationTasks(); err != nil {
				p.logger.Error("Failed to clean up replication messages.", tag.Error(err))
				p.metricsClient.Scope(metrics.ReplicationTaskCleanupScope).IncCounter(metrics.ReplicationTaskCleanupFailure)
			}
			timer.Reset(backoff.JitDuration(
				p.config.ReplicationTaskProcessorCleanupInterval(shardID),
				p.config.ReplicationTaskProcessorCleanupJitterCoefficient(shardID),
			))
		}
	}
}

func (p *taskProcessorImpl) cleanupAckedReplicationTasks() error {
	minAckLevel := int64(math.MaxInt64)
	for clusterName := range p.shard.GetClusterMetadata().GetRemoteClusterInfo() {
		ackLevel := p.shard.GetQueueClusterAckLevel(persistence.HistoryTaskCategoryReplication, clusterName).GetTaskID()
		if ackLevel < minAckLevel {
			minAckLevel = ackLevel
		}
	}
	p.logger.Debug("Cleaning up replication task queue.", tag.ReadLevel(minAckLevel))
	p.metricsClient.Scope(metrics.ReplicationTaskCleanupScope).IncCounter(metrics.ReplicationTaskCleanupCount)
	maxReadLevel := p.shard.UpdateIfNeededAndGetQueueMaxReadLevel(
		persistence.HistoryTaskCategoryReplication,
		p.currentCluster,
	).GetTaskID()
	lagCount := int(maxReadLevel - minAckLevel)
	scope := p.metricsClient.Scope(metrics.ReplicationTaskFetcherScope,
		metrics.TargetClusterTag(p.currentCluster),
	)
	scope.RecordTimer(metrics.ReplicationTasksLag, time.Duration(lagCount))
	scope.IntExponentialHistogram(metrics.ExponentialReplicationTasksLag, lagCount)
	for {
		pageSize := p.config.ReplicatorTaskDeleteBatchSize()
		resp, err := p.shard.GetExecutionManager().RangeCompleteHistoryTask(
			context.Background(),
			&persistence.RangeCompleteHistoryTaskRequest{
				TaskCategory:        persistence.HistoryTaskCategoryReplication,
				ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(minAckLevel + 1),
				PageSize:            pageSize,
			},
		)
		if err != nil {
			return err
		}
		if !persistence.HasMoreRowsToDelete(resp.TasksCompleted, pageSize) {
			break
		}
	}
	return nil
}

func (p *taskProcessorImpl) processResponse(response *types.ReplicationMessages) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("processResponse encountered panic.", tag.Value(r))
			panic(r)
		}
	}()

	select {
	case p.syncShardChan <- response.GetSyncShardStatus():
	default:
	}

	scope := p.metricsClient.Scope(metrics.ReplicationTaskFetcherScope, metrics.TargetClusterTag(p.sourceCluster))
	batchRequestStartTime := time.Now()
	ctx := context.Background()
	for _, replicationTask := range response.ReplicationTasks {
		// TODO: move to MultiStageRateLimiter
		_ = p.hostRateLimiter.Wait(ctx)
		_ = p.shardRateLimiter.Wait(ctx)
		err := p.processSingleTask(replicationTask)
		if err != nil {
			// Encounter error and skip updating ack levels
			// TODO: Does this behavior make sense? If ack levels are not updated the whole batch will have to be re-fetched via processor loop.
			// Potential improvements:
			// 1. Update ack levels for the tasks that were processed successfully.
			// 2. Emit logs/metrics for these cases that we give up on updating ack levels.
			return
		}
	}

	// Note here we check replication tasks instead of hasMore. The expectation is that in a steady state
	// we will receive replication tasks but hasMore is false (meaning that we are always catching up).
	// So hasMore might not be a good indicator for additional wait.
	if len(response.ReplicationTasks) == 0 {
		backoffDuration := p.noTaskRetrier.NextBackOff()
		time.Sleep(backoffDuration)
	} else {
		appliedLatency := time.Since(batchRequestStartTime)
		scope.RecordTimer(metrics.ReplicationTasksAppliedLatency, appliedLatency)
		scope.ExponentialHistogram(metrics.ExponentialReplicationTasksAppliedLatency, appliedLatency)
	}

	if p.isShuttingDown() {
		// avoid updating ack-levels if there's a shutdown as well remembering that GetReplication messages is a *write* api
		// (keeping track of consumer offsets), so we have to be careful what data it's sent
		return
	}

	p.lastProcessedMessageID = response.GetLastRetrievedMessageID()
	p.lastRetrievedMessageID = response.GetLastRetrievedMessageID()
	scope.UpdateGauge(metrics.LastRetrievedMessageID, float64(p.lastRetrievedMessageID))
	p.noTaskRetrier.Reset()
}

func (p *taskProcessorImpl) syncShardStatusLoop() {
	defer p.wg.Done()

	timer := time.NewTimer(backoff.JitDuration(
		p.config.ShardSyncMinInterval(),
		p.config.ShardSyncTimerJitterCoefficient(),
	))
	defer timer.Stop()

	var syncShardTask *types.SyncShardStatus
	for {
		select {
		case syncShardRequest := <-p.syncShardChan:
			syncShardTask = syncShardRequest
		case <-timer.C:
			if err := p.handleSyncShardStatus(
				syncShardTask,
			); err != nil {
				p.logger.Error("failed to sync shard status", tag.Error(err))
				p.metricsClient.Scope(metrics.HistorySyncShardStatusScope).IncCounter(metrics.SyncShardFromRemoteFailure)
			}
			timer.Reset(backoff.JitDuration(
				p.config.ShardSyncMinInterval(),
				p.config.ShardSyncTimerJitterCoefficient(),
			))
		case <-p.done:
			return
		}
	}
}

func (p *taskProcessorImpl) handleSyncShardStatus(status *types.SyncShardStatus) error {
	if status == nil ||
		p.shard.GetTimeSource().Now().Sub(time.Unix(0, status.GetTimestamp())) > dropSyncShardTaskTimeThreshold {
		return nil
	}
	p.metricsClient.Scope(metrics.HistorySyncShardStatusScope).IncCounter(metrics.SyncShardFromRemoteCounter)
	ctx, cancel := context.WithTimeout(context.Background(), replicationTimeout)
	defer cancel()
	return p.historyEngine.SyncShardStatus(ctx, &types.SyncShardStatusRequest{
		SourceCluster: p.sourceCluster,
		ShardID:       int64(p.shard.GetShardID()),
		Timestamp:     status.Timestamp,
	})
}

func (p *taskProcessorImpl) processSingleTask(replicationTask *types.ReplicationTask) error {
	retryTransientError := func(ctx context.Context) error {
		throttleRetry := backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(p.taskRetryPolicy),
			backoff.WithRetryableError(isTransientRetryableError),
		)
		return throttleRetry.Do(ctx, func(ctx context.Context) error {
			select {
			case <-p.done:
				// if the processor is stopping, skip the task
				// the ack level will not update and the new shard owner will retry the task.
				return nil
			default:
				return p.processTaskOnce(replicationTask)
			}
		})
	}

	// Handle service busy error
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(common.CreateReplicationServiceBusyRetryPolicy()),
		backoff.WithRetryableError(common.IsServiceBusyError),
	)
	err := throttleRetry.Do(context.Background(), retryTransientError)

	switch {
	case err == nil:
		return nil
	case common.IsServiceBusyError(err):
		return err
	case err == execution.ErrMissingVersionHistories:
		// skip the workflow without version histories
		p.logger.Warn("Encounter workflow without version histories")
		return nil
	default:
		// handle error
	}

	// handle error to DLQ
	select {
	case <-p.done:
		p.logger.Warn("Skip adding new messages to DLQ.", tag.Error(err))
		return err
	default:
		request, err2 := p.generateDLQRequest(replicationTask)
		if err2 != nil {
			p.logger.Error("Failed to generate DLQ replication task.", tag.Error(err2))
			// We cannot deserialize the task. Dropping it.
			return nil
		}
		p.logger.Error("Failed to apply replication task after retry. Putting task into DLQ.",
			tag.WorkflowDomainID(request.TaskInfo.GetDomainID()),
			tag.WorkflowID(request.TaskInfo.GetWorkflowID()),
			tag.WorkflowRunID(request.TaskInfo.GetRunID()),
			tag.TaskID(request.TaskInfo.GetTaskID()),
			tag.TaskType(request.TaskInfo.GetTaskType()),
			tag.Error(err),
		)
		// TODO: uncomment this when the execution fixer workflow is ready
		// if err = p.triggerDataInconsistencyScan(replicationTask); err != nil {
		//	p.logger.Warn("Failed to trigger data scan", tag.Error(err))
		//	p.metricsClient.IncCounter(metrics.ReplicationDLQStatsScope, metrics.ReplicationDLQValidationFailed)
		// }
		return p.putReplicationTaskToDLQ(request)
	}
}

func (p *taskProcessorImpl) processTaskOnce(replicationTask *types.ReplicationTask) error {
	ts := p.shard.GetTimeSource()
	startTime := ts.Now()
	scope, err := p.taskExecutor.execute(replicationTask, false)

	if err != nil {
		p.updateFailureMetric(scope, err, p.shard.GetShardID())
	} else {
		now := ts.Now()
		mScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster))
		domainID := replicationTask.HistoryTaskV2Attributes.GetDomainID()
		var domainName string
		if domainID != "" {
			cachedName, errorDomainName := p.shard.GetDomainCache().GetDomainName(domainID)
			if errorDomainName != nil {
				return errorDomainName
			}
			domainName = cachedName
		}
		mScope = mScope.Tagged(metrics.DomainTag(domainName)) // use consistent tags so Prometheus does not break

		// emit single task processing latency
		mScope.ExponentialHistogram(metrics.ExponentialTaskProcessingLatency, now.Sub(startTime))
		// emit latency from task generated to task received
		mScope.ExponentialHistogram(
			metrics.ExponentialReplicationTaskLatency,
			now.Sub(time.Unix(0, replicationTask.GetCreationTime())),
		)

		// emit single task processing latency
		mScope.RecordTimer(metrics.TaskProcessingLatency, now.Sub(startTime))
		e2eLatency := now.Sub(time.Unix(0, replicationTask.GetCreationTime()))
		// emit latency from task generated to task received
		mScope.RecordTimer(metrics.ReplicationTaskLatency, e2eLatency)

		// if latency is not switched off, and exceeds threshold, emit structured log
		if p.config.ReplicationTaskProcessorLatencyLogThreshold() > 0 && e2eLatency >= p.config.ReplicationTaskProcessorLatencyLogThreshold() {
			attr := replicationTask.GetHistoryTaskV2Attributes()
			var workflowID, runID string
			if attr != nil {
				workflowID = attr.GetWorkflowID()
				runID = attr.GetRunID()
			}
			p.logger.Warn("high replication latency",
				tag.WorkflowDomainID(domainID),
				tag.WorkflowID(workflowID),
				tag.WorkflowRunID(runID),
				tag.ShardID(p.shard.GetShardID()),
				tag.SourceCluster(p.sourceCluster),
			)
		}

		// emit the number of replication tasks
		mScope.IncCounter(metrics.ReplicationTasksAppliedPerDomain)
		shardScope := p.metricsClient.Scope(scope, metrics.TargetClusterTag(p.sourceCluster), metrics.InstanceTag(strconv.Itoa(p.shard.GetShardID())))
		shardScope.IncCounter(metrics.ReplicationTasksApplied)
	}

	return err
}

func (p *taskProcessorImpl) putReplicationTaskToDLQ(request *persistence.PutReplicationTaskToDLQRequest) error {
	p.metricsClient.Scope(
		metrics.ReplicationDLQStatsScope,
		metrics.TargetClusterTag(p.sourceCluster),
		metrics.InstanceTag(strconv.Itoa(p.shard.GetShardID())),
	).UpdateGauge(
		metrics.ReplicationDLQMaxLevelGauge,
		float64(request.TaskInfo.GetTaskID()),
	)
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(p.dlqRetryPolicy),
		backoff.WithRetryableError(p.shouldRetryDLQ),
	)
	// The following is guaranteed to success or retry forever until processor is shutdown.
	return throttleRetry.Do(context.Background(), func(ctx context.Context) error {
		err := p.shard.GetExecutionManager().PutReplicationTaskToDLQ(ctx, request)
		if err != nil {
			p.logger.Error("Failed to put replication task to DLQ.", tag.Error(err))
			p.metricsClient.IncCounter(metrics.ReplicationTaskFetcherScope, metrics.ReplicationDLQFailed)
		}
		return err
	})
}

func (p *taskProcessorImpl) generateDLQRequest(
	replicationTask *types.ReplicationTask,
) (*persistence.PutReplicationTaskToDLQRequest, error) {
	switch *replicationTask.TaskType {
	case types.ReplicationTaskTypeSyncActivity:
		taskAttributes := replicationTask.GetSyncActivityTaskAttributes()
		domainName, err := p.shard.GetDomainCache().GetDomainName(taskAttributes.GetDomainID())
		if err != nil {
			return nil, err
		}
		return &persistence.PutReplicationTaskToDLQRequest{
			SourceClusterName: p.sourceCluster,
			TaskInfo: &persistence.ReplicationTaskInfo{
				DomainID:    taskAttributes.GetDomainID(),
				WorkflowID:  taskAttributes.GetWorkflowID(),
				RunID:       taskAttributes.GetRunID(),
				TaskID:      replicationTask.GetSourceTaskID(),
				TaskType:    persistence.ReplicationTaskTypeSyncActivity,
				ScheduledID: taskAttributes.GetScheduledID(),
			},
			DomainName: domainName,
		}, nil

	case types.ReplicationTaskTypeHistoryV2:
		taskAttributes := replicationTask.GetHistoryTaskV2Attributes()
		domainName, err := p.shard.GetDomainCache().GetDomainName(taskAttributes.GetDomainID())
		if err != nil {
			return nil, err
		}
		eventsDataBlob := persistence.NewDataBlobFromInternal(taskAttributes.GetEvents())
		events, err := p.historySerializer.DeserializeBatchEvents(eventsDataBlob)
		if err != nil {
			return nil, err
		}

		if len(events) == 0 {
			p.logger.Error("Empty events in a batch")
			return nil, fmt.Errorf("corrupted history event batch, empty events")
		}

		return &persistence.PutReplicationTaskToDLQRequest{
			SourceClusterName: p.sourceCluster,
			TaskInfo: &persistence.ReplicationTaskInfo{
				DomainID:     taskAttributes.GetDomainID(),
				WorkflowID:   taskAttributes.GetWorkflowID(),
				RunID:        taskAttributes.GetRunID(),
				TaskID:       replicationTask.GetSourceTaskID(),
				TaskType:     persistence.ReplicationTaskTypeHistory,
				FirstEventID: events[0].ID,
				NextEventID:  events[len(events)-1].ID + 1,
				Version:      events[0].Version,
			},
			DomainName: domainName,
		}, nil
	default:
		return nil, fmt.Errorf("unknown replication task type")
	}
}

func (p *taskProcessorImpl) triggerDataInconsistencyScan(replicationTask *types.ReplicationTask) error {
	var failoverVersion int64
	var domainID string
	var workflowID string
	var runID string
	switch {
	case replicationTask.GetHistoryTaskV2Attributes() != nil:
		attr := replicationTask.GetHistoryTaskV2Attributes()
		versionHistoryItems := attr.GetVersionHistoryItems()
		if len(versionHistoryItems) == 0 {
			return errors.New("failed to trigger data scan due to invalid version history")
		}
		// version history items in same batch should be the same
		failoverVersion = versionHistoryItems[0].GetVersion()
		domainID = attr.GetDomainID()
		workflowID = attr.GetWorkflowID()
		runID = attr.GetRunID()
	case replicationTask.GetSyncActivityTaskAttributes() != nil:
		attr := replicationTask.GetSyncActivityTaskAttributes()
		failoverVersion = replicationTask.GetSyncActivityTaskAttributes().Version
		domainID = attr.GetDomainID()
		workflowID = attr.GetWorkflowID()
		runID = attr.GetRunID()
	default:
		return nil
	}
	clusterName, err := p.shard.GetClusterMetadata().ClusterNameForFailoverVersion(failoverVersion)
	if err != nil {
		return err
	}
	client, err := p.shard.GetService().GetClientBean().GetRemoteFrontendClient(clusterName)
	if err != nil {
		return err
	}
	fixExecution := entity.Execution{
		DomainID:   domainID,
		WorkflowID: workflowID,
		RunID:      runID,
		ShardID:    p.shard.GetShardID(),
	}
	fixExecutionInput, err := json.Marshal(fixExecution)
	if err != nil {
		return err
	}
	// Assume the workflow is corrupted, rely on invariant to validate it
	_, err = client.SignalWithStartWorkflowExecution(context.Background(), &types.SignalWithStartWorkflowExecutionRequest{
		Domain:                              constants.SystemLocalDomainName,
		WorkflowID:                          reconciliation.CheckDataCorruptionWorkflowID,
		WorkflowType:                        &types.WorkflowType{Name: reconciliation.CheckDataCorruptionWorkflowType},
		TaskList:                            &types.TaskList{Name: reconciliation.CheckDataCorruptionWorkflowTaskList},
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(reconciliation.CheckDataCorruptionWorkflowTimeoutInSeconds),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(reconciliation.CheckDataCorruptionWorkflowTaskTimeoutInSeconds),
		Identity:                            "cadence-history-replication",
		RequestID:                           uuid.New(),
		WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
		SignalName:                          reconciliation.CheckDataCorruptionWorkflowSignalName,
		SignalInput:                         fixExecutionInput,
	})
	return err
}

func isTransientRetryableError(err error) bool {
	switch err.(type) {
	case *types.BadRequestError:
		return false
	case *types.ServiceBusyError:
		return false
	default:
		return true
	}
}

func (p *taskProcessorImpl) shouldRetryDLQ(err error) bool {
	if err == nil {
		return false
	}

	select {
	case <-p.done:
		p.logger.Debug("ReplicationTaskProcessor shutting down.")
		return false
	default:
		return true
	}
}

func (p *taskProcessorImpl) updateFailureMetric(scope metrics.ScopeIdx, err error, shardID int) {
	// Always update failure counter for all replicator errors
	shardScope := p.metricsClient.Scope(scope, metrics.InstanceTag(strconv.Itoa(shardID)))
	shardScope.IncCounter(metrics.ReplicatorFailures)

	// Also update counter to distinguish between type of failures
	switch err := err.(type) {
	case *types.ShardOwnershipLostError:
		shardScope.IncCounter(metrics.CadenceErrShardOwnershipLostCounter)
	case *types.BadRequestError:
		shardScope.IncCounter(metrics.CadenceErrBadRequestCounter)
	case *types.DomainNotActiveError:
		shardScope.IncCounter(metrics.CadenceErrDomainNotActiveCounter)
	case *types.WorkflowExecutionAlreadyStartedError:
		shardScope.IncCounter(metrics.CadenceErrExecutionAlreadyStartedCounter)
	case *types.EntityNotExistsError:
		shardScope.IncCounter(metrics.CadenceErrEntityNotExistsCounter)
	case *types.WorkflowExecutionAlreadyCompletedError:
		shardScope.IncCounter(metrics.CadenceErrWorkflowExecutionAlreadyCompletedCounter)
	case *types.LimitExceededError:
		shardScope.IncCounter(metrics.CadenceErrLimitExceededCounter)
	case *yarpcerrors.Status:
		if err.Code() == yarpcerrors.CodeDeadlineExceeded {
			shardScope.IncCounter(metrics.CadenceErrContextTimeoutCounter)
		}
	}
}

func (p *taskProcessorImpl) taskProcessingStartWait() {
	shardID := p.shard.GetShardID()
	time.Sleep(backoff.JitDuration(
		p.config.ReplicationTaskProcessorStartWait(shardID),
		p.config.ReplicationTaskProcessorStartWaitJitterCoefficient(shardID),
	))
}

func (p *taskProcessorImpl) isShuttingDown() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}
