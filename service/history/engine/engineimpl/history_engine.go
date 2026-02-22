// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/client/wrappers/retryable"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	cndc "github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/proto"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/decision"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/failover"
	"github.com/uber/cadence/service/history/ndc"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/replication"
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/workflow"
	"github.com/uber/cadence/service/worker/archiver"
)

const (
	defaultQueryFirstDecisionTaskWaitTime = time.Second
	queryFirstDecisionTaskCheckInterval   = 200 * time.Millisecond
	contextLockTimeout                    = 500 * time.Millisecond
	longPollCompletionBuffer              = 50 * time.Millisecond

	// TerminateIfRunningReason reason for terminateIfRunning
	TerminateIfRunningReason = "TerminateIfRunning Policy"
	// TerminateIfRunningDetailsTemplate details template for terminateIfRunning
	TerminateIfRunningDetailsTemplate = "New runID: %s"
)

var (
	errDomainDeprecated = &types.BadRequestError{Message: "Domain is deprecated."}
)

type historyEngineImpl struct {
	currentClusterName        string
	shard                     shard.Context
	timeSource                clock.TimeSource
	decisionHandler           decision.Handler
	clusterMetadata           cluster.Metadata
	historyV2Mgr              persistence.HistoryManager
	executionManager          persistence.ExecutionManager
	visibilityMgr             persistence.VisibilityManager
	queueProcessors           map[persistence.HistoryTaskCategory]queue.Processor
	nDCReplicator             ndc.HistoryReplicator
	nDCActivityReplicator     ndc.ActivityReplicator
	historyEventNotifier      events.Notifier
	tokenSerializer           common.TaskTokenSerializer
	executionCache            execution.Cache
	metricsClient             metrics.Client
	logger                    log.Logger
	throttledLogger           log.Logger
	activeClusterManager      activecluster.Manager
	config                    *config.Config
	archivalClient            archiver.Client
	workflowResetter          reset.WorkflowResetter
	replicationTaskProcessors []replication.TaskProcessor
	replicationAckManager     replication.TaskAckManager
	replicationTaskStore      *replication.TaskStore
	replicationHydrator       replication.TaskHydrator
	replicationMetricsEmitter *replication.MetricsEmitterImpl
	eventsReapplier           ndc.EventsReapplier
	matchingClient            matching.Client
	rawMatchingClient         matching.Client
	clientChecker             client.VersionChecker
	replicationDLQHandler     replication.DLQHandler
	failoverMarkerNotifier    failover.MarkerNotifier

	updateWithActionFn func(
		context.Context,
		log.Logger,
		execution.Cache,
		string,
		types.WorkflowExecution,
		bool,
		time.Time,
		func(wfContext execution.Context, mutableState execution.MutableState) error,
	) error
}

var (
	// FailedWorkflowCloseState is a set of failed workflow close states, used for start workflow policy
	// for start workflow execution API
	FailedWorkflowCloseState = map[int]bool{
		persistence.WorkflowCloseStatusFailed:     true,
		persistence.WorkflowCloseStatusCanceled:   true,
		persistence.WorkflowCloseStatusTerminated: true,
		persistence.WorkflowCloseStatusTimedOut:   true,
	}
)

// NewEngineWithShardContext creates an instance of history engine
func NewEngineWithShardContext(
	shard shard.Context,
	visibilityMgr persistence.VisibilityManager,
	matching matching.Client,
	historyEventNotifier events.Notifier,
	config *config.Config,
	replicationTaskFetchers replication.TaskFetchers,
	rawMatchingClient matching.Client,
	failoverCoordinator failover.Coordinator,
	queueFactories []queue.Factory,
) engine.Engine {
	currentClusterName := shard.GetService().GetClusterMetadata().GetCurrentClusterName()

	logger := shard.GetLogger()
	executionManager := shard.GetExecutionManager()
	historyV2Manager := shard.GetHistoryManager()
	executionCache := execution.NewCache(shard)
	failoverMarkerNotifier := failover.NewMarkerNotifier(shard, config, failoverCoordinator)
	replicationHydrator := replication.NewDeferredTaskHydrator(shard.GetShardID(), historyV2Manager, executionCache, shard.GetDomainCache())
	replicationTaskStore := replication.NewTaskStore(
		shard.GetConfig(),
		shard.GetClusterMetadata(),
		shard.GetDomainCache(),
		shard.GetMetricsClient(),
		shard.GetLogger(),
		replicationHydrator,
		shard.GetReplicationBudgetManager(),
		shard.GetShardID(),
		shard.GetTimeSource(),
	)
	replicationDynamicTaskBatchSizer := replication.NewDynamicTaskBatchSizer(shard.GetShardID(), logger, config, shard.GetMetricsClient())
	replicationReader := replication.NewTaskReader(shard.GetShardID(), executionManager)

	historyEngImpl := &historyEngineImpl{
		currentClusterName:   currentClusterName,
		shard:                shard,
		clusterMetadata:      shard.GetClusterMetadata(),
		timeSource:           shard.GetTimeSource(),
		historyV2Mgr:         historyV2Manager,
		executionManager:     executionManager,
		visibilityMgr:        visibilityMgr,
		tokenSerializer:      common.NewJSONTaskTokenSerializer(),
		executionCache:       executionCache,
		logger:               logger.WithTags(tag.ComponentHistoryEngine),
		throttledLogger:      shard.GetThrottledLogger().WithTags(tag.ComponentHistoryEngine),
		activeClusterManager: shard.GetActiveClusterManager(),
		metricsClient:        shard.GetMetricsClient(),
		historyEventNotifier: historyEventNotifier,
		config:               config,
		workflowResetter: reset.NewWorkflowResetter(
			shard,
			executionCache,
			logger,
		),
		matchingClient:         matching,
		rawMatchingClient:      rawMatchingClient,
		clientChecker:          client.NewVersionChecker(),
		failoverMarkerNotifier: failoverMarkerNotifier,
		replicationHydrator:    replicationHydrator,
		replicationAckManager: replication.NewTaskAckManager(
			shard.GetShardID(),
			shard,
			shard.GetMetricsClient(),
			shard.GetLogger(),
			replicationReader,
			replicationTaskStore,
			shard.GetTimeSource(),
			config,
			proto.ReplicationMessagesSize,
			replicationDynamicTaskBatchSizer,
		),
		replicationTaskStore: replicationTaskStore,
		replicationMetricsEmitter: replication.NewMetricsEmitter(
			shard.GetShardID(), shard, replicationReader, shard.GetMetricsClient()),
		updateWithActionFn: workflow.UpdateWithAction,
		queueProcessors:    make(map[persistence.HistoryTaskCategory]queue.Processor),
	}
	historyEngImpl.decisionHandler = decision.NewHandler(
		shard,
		historyEngImpl.executionCache,
		historyEngImpl.tokenSerializer,
	)
	pRetry := persistence.NewPersistenceRetryer(
		shard.GetExecutionManager(),
		shard.GetHistoryManager(),
		common.CreatePersistenceRetryPolicy(),
	)
	openExecutionCheck := invariant.NewConcreteExecutionExists(pRetry, shard.GetDomainCache())

	for _, factory := range queueFactories {
		historyEngImpl.queueProcessors[factory.Category()] = factory.CreateQueue(
			shard,
			executionCache,
			openExecutionCheck,
		)
	}

	historyEngImpl.eventsReapplier = ndc.NewEventsReapplier(shard.GetMetricsClient(), logger)

	historyEngImpl.nDCReplicator = ndc.NewHistoryReplicator(
		shard,
		executionCache,
		historyEngImpl.eventsReapplier,
		logger,
	)
	historyEngImpl.nDCActivityReplicator = ndc.NewActivityReplicator(
		shard,
		executionCache,
		logger,
	)

	var replicationTaskProcessors []replication.TaskProcessor
	replicationTaskExecutors := make(map[string]replication.TaskExecutor)
	// Intentionally use the raw client to create its own retry policy
	historyRawClient := shard.GetService().GetClientBean().GetHistoryClient()
	historyRetryableClient := retryable.NewHistoryClient(
		historyRawClient,
		common.CreateReplicationServiceBusyRetryPolicy(),
		common.IsServiceBusyError,
	)
	resendFunc := func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
		return historyRetryableClient.ReplicateEventsV2(ctx, request)
	}
	for _, replicationTaskFetcher := range replicationTaskFetchers.GetFetchers() {
		sourceCluster := replicationTaskFetcher.GetSourceCluster()
		historyResender := cndc.NewHistoryResender(
			shard.GetDomainCache(),
			shard.GetService().GetClientBean(),
			resendFunc,
			nil,
			openExecutionCheck,
			shard.GetLogger().WithTags(tag.ComponentReplicatorQueue, tag.ActiveClusterName(sourceCluster)),
		)
		replicationTaskExecutor := replication.NewTaskExecutor(
			sourceCluster,
			shard,
			shard.GetDomainCache(),
			historyResender,
			historyEngImpl,
			shard.GetMetricsClient(),
			shard.GetLogger().WithTags(tag.ComponentReplicatorQueue, tag.ActiveClusterName(sourceCluster)),
		)
		replicationTaskExecutors[sourceCluster] = replicationTaskExecutor

		replicationTaskProcessor := replication.NewTaskProcessor(
			shard,
			historyEngImpl,
			config,
			shard.GetMetricsClient(),
			replicationTaskFetcher,
			replicationTaskExecutor,
			shard.GetTimeSource(),
		)
		replicationTaskProcessors = append(replicationTaskProcessors, replicationTaskProcessor)
	}
	historyEngImpl.replicationTaskProcessors = replicationTaskProcessors
	replicationMessageHandler := replication.NewDLQHandler(shard, replicationTaskExecutors)
	historyEngImpl.replicationDLQHandler = replicationMessageHandler

	shard.SetEngine(historyEngImpl)
	return historyEngImpl
}

// Start will spin up all the components needed to start serving this shard.
// Make sure all the components are loaded lazily so start can return immediately.  This is important because
// ShardController calls start sequentially for all the shards for a given host during startup.
func (e *historyEngineImpl) Start() {
	e.logger.Info("History engine state changed", tag.LifeCycleStarting)
	defer e.logger.Info("History engine state changed", tag.LifeCycleStarted)

	for _, processor := range e.queueProcessors {
		processor.Start()
	}
	e.replicationDLQHandler.Start()
	e.replicationMetricsEmitter.Start()

	// failover callback will try to create a failover queue processor to scan all inflight tasks
	// if domain needs to be failovered. However, in the multicursor queue logic, the scan range
	// can't be retrieved before the processor is started. If failover callback is registered
	// before queue processor is started, it may result in a deadline as to create the failover queue,
	// queue processor need to be started.
	e.registerDomainFailoverCallback()

	for _, replicationTaskProcessor := range e.replicationTaskProcessors {
		replicationTaskProcessor.Start()
	}
	if e.config.EnableGracefulFailover() {
		e.failoverMarkerNotifier.Start()
	}

}

// Stop the service.
func (e *historyEngineImpl) Stop() {
	e.logger.Info("History engine state changed", tag.LifeCycleStopping)
	defer e.logger.Info("History engine state changed", tag.LifeCycleStopped)

	for _, processor := range e.queueProcessors {
		processor.Stop()
	}
	e.replicationDLQHandler.Stop()
	e.replicationMetricsEmitter.Stop()

	for _, replicationTaskProcessor := range e.replicationTaskProcessors {
		replicationTaskProcessor.Stop()
	}

	e.failoverMarkerNotifier.Stop()

	// unset the failover callback
	e.shard.GetDomainCache().UnregisterDomainChangeCallback(createShardNameFromShardID(e.shard.GetShardID()))
}

// ScheduleDecisionTask schedules a decision if no outstanding decision found
func (e *historyEngineImpl) ScheduleDecisionTask(ctx context.Context, req *types.ScheduleDecisionTaskRequest) error {
	return e.decisionHandler.HandleDecisionTaskScheduled(ctx, req)
}

func (e *historyEngineImpl) ReplicateEventsV2(ctx context.Context, replicateRequest *types.ReplicateEventsV2Request) error {
	return e.nDCReplicator.ApplyEvents(ctx, replicateRequest)
}

func (e *historyEngineImpl) SyncShardStatus(ctx context.Context, request *types.SyncShardStatusRequest) error {

	clusterName := request.GetSourceCluster()
	now := time.Unix(0, request.GetTimestamp())

	// here there are 3 main things
	// 1. update the view of remote cluster's shard time
	// 2. notify the timer gate in the timer queue standby processor
	// 3. notify the transfer (essentially a no op, just put it here so it looks symmetric)
	e.shard.SetCurrentTime(clusterName, now)
	for _, processor := range e.queueProcessors {
		processor.NotifyNewTask(clusterName, &hcommon.NotifyTaskInfo{Tasks: []persistence.Task{}})
	}
	return nil
}

func (e *historyEngineImpl) SyncActivity(ctx context.Context, request *types.SyncActivityRequest) (retError error) {

	return e.nDCActivityReplicator.SyncActivity(ctx, request)
}

func (e *historyEngineImpl) newDomainNotActiveError(
	domainEntry *cache.DomainCacheEntry,
	failoverVersion int64,
) error {
	activeClusterName, err := e.shard.GetClusterMetadata().ClusterNameForFailoverVersion(failoverVersion)
	if err != nil {
		activeClusterName = "_unknown_"
	}

	return domainEntry.NewDomainNotActiveError(e.currentClusterName, activeClusterName)
}

func (e *historyEngineImpl) checkForHistoryCorruptions(ctx context.Context, mutableState execution.MutableState) (bool, error) {
	domainName := mutableState.GetDomainEntry().GetInfo().Name
	if !e.config.EnableHistoryCorruptionCheck(domainName) {
		return false, nil
	}

	// Ensure that we can obtain start event. Failing to do so means corrupted history or resurrected mutable state record.
	_, err := mutableState.GetStartEvent(ctx)
	if err != nil {
		info := mutableState.GetExecutionInfo()
		// Mark workflow as corrupted. So that new one can be restarted.
		info.State = persistence.WorkflowStateCorrupted

		e.logger.Error("history corruption check failed",
			tag.WorkflowDomainName(domainName),
			tag.WorkflowID(info.WorkflowID),
			tag.WorkflowRunID(info.RunID),
			tag.WorkflowType(info.WorkflowTypeName),
			tag.Error(err))

		if errors.Is(err, execution.ErrMissingWorkflowStartEvent) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

func getScheduleID(activityID string, mutableState execution.MutableState) (int64, error) {
	if activityID == "" {
		return 0, &types.BadRequestError{Message: "Neither ActivityID nor ScheduleID is provided"}
	}
	activityInfo, ok := mutableState.GetActivityByActivityID(activityID)
	if !ok {
		return 0, &types.BadRequestError{Message: "Cannot locate Activity ScheduleID"}
	}
	return activityInfo.ScheduleID, nil
}

func (e *historyEngineImpl) getActiveDomainByWorkflow(ctx context.Context, domainID, workflowID, runID string) (*cache.DomainCacheEntry, error) {
	activeClusterInfo, err := e.activeClusterManager.GetActiveClusterInfoByWorkflow(ctx, domainID, workflowID, runID)
	if err != nil {
		return nil, err
	}
	domain, err := e.shard.GetDomainCache().GetDomainByID(domainID)
	if err != nil {
		return nil, err
	}
	if activeClusterInfo.ActiveClusterName == e.clusterMetadata.GetCurrentClusterName() {
		return domain, nil
	}
	return nil, domain.NewDomainNotActiveError(e.clusterMetadata.GetCurrentClusterName(), activeClusterInfo.ActiveClusterName)
}

func createShardNameFromShardID(shardID int) string {
	return fmt.Sprintf("history-engine-%d", shardID)
}
