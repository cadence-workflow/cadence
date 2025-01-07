// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/ndc"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/worker/archiver"
)

type timerQueueProcessor struct {
	shard         shard.Context
	historyEngine engine.Engine
	taskProcessor task.Processor

	config             *config.Config
	currentClusterName string

	metricsClient metrics.Client
	logger        log.Logger

	status       int32
	shutdownChan chan struct{}
	shutdownWG   sync.WaitGroup

	ackLevel                time.Time
	taskAllocator           TaskAllocator
	activeTaskExecutor      task.Executor
	activeQueueProcessor    *timerQueueProcessorBase
	standbyQueueProcessors  map[string]*timerQueueProcessorBase
	standbyTaskExecutors    []task.Executor
	standbyQueueTimerGates  map[string]RemoteTimerGate
	failoverQueueProcessors []*timerQueueProcessorBase
}

// NewTimerQueueProcessor creates a new timer QueueProcessor
func NewTimerQueueProcessor(
	shard shard.Context,
	historyEngine engine.Engine,
	taskProcessor task.Processor,
	executionCache execution.Cache,
	archivalClient archiver.Client,
	executionCheck invariant.Invariant,
) Processor {
	logger := shard.GetLogger().WithTags(tag.ComponentTimerQueue)
	currentClusterName := shard.GetClusterMetadata().GetCurrentClusterName()
	config := shard.GetConfig()
	taskAllocator := NewTaskAllocator(shard)

	activeTaskExecutor := task.NewTimerActiveTaskExecutor(
		shard,
		archivalClient,
		executionCache,
		logger,
		shard.GetMetricsClient(),
		config,
	)

	activeQueueProcessor := newTimerQueueActiveProcessor(
		currentClusterName,
		shard,
		historyEngine,
		taskProcessor,
		taskAllocator,
		activeTaskExecutor,
		logger,
	)

	standbyTaskExecutors := make([]task.Executor, 0, len(shard.GetClusterMetadata().GetRemoteClusterInfo()))
	standbyQueueProcessors := make(map[string]*timerQueueProcessorBase)
	standbyQueueTimerGates := make(map[string]RemoteTimerGate)
	for clusterName := range shard.GetClusterMetadata().GetRemoteClusterInfo() {
		historyResender := ndc.NewHistoryResender(
			shard.GetDomainCache(),
			shard.GetService().GetClientBean().GetRemoteAdminClient(clusterName),
			func(ctx context.Context, request *types.ReplicateEventsV2Request) error {
				return historyEngine.ReplicateEventsV2(ctx, request)
			},
			config.StandbyTaskReReplicationContextTimeout,
			executionCheck,
			shard.GetLogger(),
		)
		standbyTaskExecutor := task.NewTimerStandbyTaskExecutor(
			shard,
			archivalClient,
			executionCache,
			historyResender,
			logger,
			shard.GetMetricsClient(),
			clusterName,
			config,
		)
		standbyTaskExecutors = append(standbyTaskExecutors, standbyTaskExecutor)
		standbyQueueProcessors[clusterName], standbyQueueTimerGates[clusterName] = newTimerQueueStandbyProcessor(
			clusterName,
			shard,
			historyEngine,
			taskProcessor,
			taskAllocator,
			standbyTaskExecutor,
			logger,
		)
	}

	return &timerQueueProcessor{
		shard:         shard,
		historyEngine: historyEngine,
		taskProcessor: taskProcessor,

		config:             config,
		currentClusterName: currentClusterName,

		metricsClient: shard.GetMetricsClient(),
		logger:        logger,

		status:       common.DaemonStatusInitialized,
		shutdownChan: make(chan struct{}),

		ackLevel:               shard.GetTimerAckLevel(),
		taskAllocator:          taskAllocator,
		activeTaskExecutor:     activeTaskExecutor,
		activeQueueProcessor:   activeQueueProcessor,
		standbyQueueProcessors: standbyQueueProcessors,
		standbyQueueTimerGates: standbyQueueTimerGates,
		standbyTaskExecutors:   standbyTaskExecutors,
	}
}

func (t *timerQueueProcessor) Start() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	t.activeQueueProcessor.Start()
	for _, standbyQueueProcessor := range t.standbyQueueProcessors {
		standbyQueueProcessor.Start()
	}

	t.shutdownWG.Add(1)
	go t.completeTimerLoop()
}

func (t *timerQueueProcessor) Stop() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	if !t.shard.GetConfig().QueueProcessorEnableGracefulSyncShutdown() {
		t.activeQueueProcessor.Stop()
		// stop active executor after queue processor
		t.activeTaskExecutor.Stop()
		for _, standbyQueueProcessor := range t.standbyQueueProcessors {
			standbyQueueProcessor.Stop()
		}

		// stop standby executors after queue processors
		for _, standbyTaskExecutor := range t.standbyTaskExecutors {
			standbyTaskExecutor.Stop()
		}

		close(t.shutdownChan)
		common.AwaitWaitGroup(&t.shutdownWG, time.Minute)
		return
	}

	// close the shutdown channel first so processor pumps drains tasks
	// and then stop the processors
	close(t.shutdownChan)
	if !common.AwaitWaitGroup(&t.shutdownWG, gracefulShutdownTimeout) {
		t.logger.Warn("transferQueueProcessor timed out on shut down", tag.LifeCycleStopTimedout)
	}
	t.activeQueueProcessor.Stop()

	for _, standbyQueueProcessor := range t.standbyQueueProcessors {
		standbyQueueProcessor.Stop()
	}

	// stop standby executors after queue processors
	for _, standbyTaskExecutor := range t.standbyTaskExecutors {
		standbyTaskExecutor.Stop()
	}

	if len(t.failoverQueueProcessors) > 0 {
		t.logger.Info("Shutting down failover timer queues", tag.Counter(len(t.failoverQueueProcessors)))
		for _, failoverQueueProcessor := range t.failoverQueueProcessors {
			failoverQueueProcessor.Stop()
		}
	}

	t.activeTaskExecutor.Stop()
}

func (t *timerQueueProcessor) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	if clusterName == t.currentClusterName {
		t.activeQueueProcessor.notifyNewTimers(info.Tasks)
		return
	}

	standbyQueueProcessor, ok := t.standbyQueueProcessors[clusterName]
	if !ok {
		panic(fmt.Sprintf("Cannot find standby timer processor for %s.", clusterName))
	}

	standbyQueueTimerGate, ok := t.standbyQueueTimerGates[clusterName]
	if !ok {
		panic(fmt.Sprintf("Cannot find standby timer gate for %s.", clusterName))
	}

	curTime := t.shard.GetCurrentTime(clusterName)
	standbyQueueTimerGate.SetCurrentTime(curTime)
	t.logger.Debug("Current time for standby queue timergate is updated", tag.ClusterName(clusterName), tag.Timestamp(curTime))
	standbyQueueProcessor.notifyNewTimers(info.Tasks)
}

func (t *timerQueueProcessor) FailoverDomain(domainIDs map[string]struct{}) {
	// Failover queue is used to scan all inflight tasks, if queue processor is not
	// started, there's no inflight task and we don't need to create a failover processor.
	// Also the HandleAction will be blocked if queue processor processing loop is not running.
	if atomic.LoadInt32(&t.status) != common.DaemonStatusStarted {
		return
	}

	minLevel := t.shard.GetTimerClusterAckLevel(t.currentClusterName)
	standbyClusterName := t.currentClusterName
	for clusterName := range t.shard.GetClusterMetadata().GetEnabledClusterInfo() {
		ackLevel := t.shard.GetTimerClusterAckLevel(clusterName)
		if ackLevel.Before(minLevel) {
			minLevel = ackLevel
			standbyClusterName = clusterName
		}
	}

	if standbyClusterName != t.currentClusterName {
		t.logger.Debugf("Timer queue failover will use minLevel: %v from standbyClusterName: %s", minLevel, standbyClusterName)
	} else {
		t.logger.Debugf("Timer queue failover will use minLevel: %v from current cluster: %s", minLevel, t.currentClusterName)
	}

	maxReadLevel := time.Time{}
	actionResult, err := t.HandleAction(context.Background(), t.currentClusterName, NewGetStateAction())
	if err != nil {
		t.logger.Error("Timer failover failed while getting queue states", tag.WorkflowDomainIDs(domainIDs), tag.Error(err))
		if err == errProcessorShutdown {
			// processor/shard already shutdown, we don't need to create failover queue processor
			return
		}
		// other errors should never be returned for GetStateAction
		panic(fmt.Sprintf("unknown error for GetStateAction: %v", err))
	}

	var maxReadLevelQueueLevel int
	for _, queueState := range actionResult.GetStateActionResult.States {
		queueReadLevel := queueState.ReadLevel().(timerTaskKey).visibilityTimestamp
		if maxReadLevel.Before(queueReadLevel) {
			maxReadLevel = queueReadLevel
			maxReadLevelQueueLevel = queueState.Level()
		}
	}

	if !maxReadLevel.IsZero() {
		t.logger.Debugf("Timer queue failover will use maxReadLevel: %v from queue at level: %v", maxReadLevel, maxReadLevelQueueLevel)
	}

	// TODO: Below Add call has no effect, understand the underlying intent and fix it.
	maxReadLevel.Add(1 * time.Millisecond)

	t.logger.Info("Timer Failover Triggered",
		tag.WorkflowDomainIDs(domainIDs),
		tag.MinLevel(minLevel.UnixNano()),
		tag.MaxLevel(maxReadLevel.UnixNano()),
	)

	updateClusterAckLevelFn, failoverQueueProcessor := newTimerQueueFailoverProcessor(
		standbyClusterName,
		t.shard,
		t.taskProcessor,
		t.taskAllocator,
		t.activeTaskExecutor,
		t.logger,
		minLevel,
		maxReadLevel,
		domainIDs,
	)

	// NOTE: READ REF BEFORE MODIFICATION
	// ref: historyEngine.go registerDomainFailoverCallback function
	err = updateClusterAckLevelFn(newTimerTaskKey(minLevel, 0))
	if err != nil {
		t.logger.Error("Error update shard ack level", tag.Error(err))
	}

	// Failover queue processors are started on the fly when domains are failed over.
	// Failover queue processors will be stopped when the timer queue instance is stopped (due to restart or shard movement).
	// This means the failover queue processor might not finish its job.
	// There is no mechanism to re-start ongoing failover queue processors in the new shard owner.
	t.failoverQueueProcessors = append(t.failoverQueueProcessors, failoverQueueProcessor)
	failoverQueueProcessor.Start()
}

func (t *timerQueueProcessor) HandleAction(ctx context.Context, clusterName string, action *Action) (*ActionResult, error) {
	var resultNotificationCh chan actionResultNotification
	var added bool
	if clusterName == t.currentClusterName {
		resultNotificationCh, added = t.activeQueueProcessor.addAction(ctx, action)
	} else {
		found := false
		for standbyClusterName, standbyProcessor := range t.standbyQueueProcessors {
			if clusterName == standbyClusterName {
				resultNotificationCh, added = standbyProcessor.addAction(ctx, action)
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("unknown cluster name: %v", clusterName)
		}
	}

	if !added {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errProcessorShutdown
	}

	select {
	case resultNotification := <-resultNotificationCh:
		return resultNotification.result, resultNotification.err
	case <-t.shutdownChan:
		return nil, errProcessorShutdown
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *timerQueueProcessor) LockTaskProcessing() {
	t.taskAllocator.Lock()
}

func (t *timerQueueProcessor) UnlockTaskProcessing() {
	t.taskAllocator.Unlock()
}

func (t *timerQueueProcessor) drain() {
	if !t.shard.GetConfig().QueueProcessorEnableGracefulSyncShutdown() {
		if err := t.completeTimer(context.Background()); err != nil {
			t.logger.Error("Failed to complete timer task during drain", tag.Error(err))
		}
		return
	}

	// when graceful shutdown is enabled for queue processor, use a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := t.completeTimer(ctx); err != nil {
		t.logger.Error("Failed to complete timer task during drain", tag.Error(err))
	}
}

func (t *timerQueueProcessor) completeTimerLoop() {
	defer t.shutdownWG.Done()

	completeTimer := time.NewTimer(t.config.TimerProcessorCompleteTimerInterval())
	defer completeTimer.Stop()

	// Create a retryTimer once, and reset it as needed
	retryTimer := time.NewTimer(0)
	defer retryTimer.Stop()
	// Stop it immediately because we don't want it to fire initially
	if !retryTimer.Stop() {
		<-retryTimer.C
	}
	for {
		select {
		case <-t.shutdownChan:
			t.drain()
			return
		case <-completeTimer.C:
			for attempt := 0; attempt < t.config.TimerProcessorCompleteTimerFailureRetryCount(); attempt++ {
				err := t.completeTimer(context.Background())
				if err == nil {
					break
				}

				t.logger.Error("Failed to complete timer task", tag.Error(err))
				var errShardClosed *shard.ErrShardClosed
				if errors.As(err, &errShardClosed) {
					if !t.shard.GetConfig().QueueProcessorEnableGracefulSyncShutdown() {
						go t.Stop()
						return
					}

					t.Stop()
					return
				}

				// Reset the retryTimer for the delay between attempts
				// TODO: the first retry has 0 backoff, revisit it to see if it's expected
				retryDuration := time.Duration(attempt*100) * time.Millisecond
				retryTimer.Reset(retryDuration)
				select {
				case <-t.shutdownChan:
					t.drain()
					return
				case <-retryTimer.C:
					// do nothing. retry loop will continue
				}
			}

			completeTimer.Reset(t.config.TimerProcessorCompleteTimerInterval())
		}
	}
}

func (t *timerQueueProcessor) completeTimer(ctx context.Context) error {
	newAckLevel := maximumTimerTaskKey
	actionResult, err := t.HandleAction(ctx, t.currentClusterName, NewGetStateAction())
	if err != nil {
		return err
	}
	for _, queueState := range actionResult.GetStateActionResult.States {
		newAckLevel = minTaskKey(newAckLevel, queueState.AckLevel())
	}

	for standbyClusterName := range t.standbyQueueProcessors {
		actionResult, err := t.HandleAction(ctx, standbyClusterName, NewGetStateAction())
		if err != nil {
			return err
		}
		for _, queueState := range actionResult.GetStateActionResult.States {
			newAckLevel = minTaskKey(newAckLevel, queueState.AckLevel())
		}
	}

	for _, failoverInfo := range t.shard.GetAllTimerFailoverLevels() {
		failoverLevel := newTimerTaskKey(failoverInfo.MinLevel, 0)
		newAckLevel = minTaskKey(newAckLevel, failoverLevel)
	}

	if newAckLevel == maximumTimerTaskKey {
		panic("Unable to get timer queue processor ack level")
	}

	newAckLevelTimestamp := newAckLevel.(timerTaskKey).visibilityTimestamp
	if !t.ackLevel.Before(newAckLevelTimestamp) {
		t.logger.Debugf("Skipping timer task completion because new ack level %v is not before ack level %v", newAckLevelTimestamp, t.ackLevel)
		return nil
	}

	t.logger.Debugf("Start completing timer task from: %v, to %v", t.ackLevel, newAckLevelTimestamp)
	t.metricsClient.Scope(metrics.TimerQueueProcessorScope).
		Tagged(metrics.ShardIDTag(t.shard.GetShardID())).
		IncCounter(metrics.TaskBatchCompleteCounter)

	totalDeleted := 0
	for {
		pageSize := t.config.TimerTaskDeleteBatchSize()
		resp, err := t.shard.GetExecutionManager().RangeCompleteTimerTask(ctx, &persistence.RangeCompleteTimerTaskRequest{
			InclusiveBeginTimestamp: t.ackLevel,
			ExclusiveEndTimestamp:   newAckLevelTimestamp,
			PageSize:                pageSize, // pageSize may or may not be honored
		})
		if err != nil {
			return err
		}

		totalDeleted += resp.TasksCompleted
		t.logger.Debug("Timer task batch deletion", tag.Dynamic("page-size", pageSize), tag.Dynamic("total-deleted", totalDeleted))
		if !persistence.HasMoreRowsToDelete(resp.TasksCompleted, pageSize) {
			break
		}
	}

	t.ackLevel = newAckLevelTimestamp

	return t.shard.UpdateTimerAckLevel(t.ackLevel)
}

func loadTimerProcessingQueueStates(
	clusterName string,
	shard shard.Context,
	options *queueProcessorOptions,
	logger log.Logger,
) []ProcessingQueueState {
	ackLevel := shard.GetTimerClusterAckLevel(clusterName)
	if options.EnableLoadQueueStates() {
		pStates := shard.GetTimerProcessingQueueStates(clusterName)
		if validateProcessingQueueStates(pStates, ackLevel) {
			return convertFromPersistenceTimerProcessingQueueStates(pStates)
		}

		logger.Error("Incompatible processing queue states and ackLevel",
			tag.Value(pStates),
			tag.ShardTimerAcks(ackLevel),
		)
	}

	return []ProcessingQueueState{
		NewProcessingQueueState(
			defaultProcessingQueueLevel,
			newTimerTaskKey(ackLevel, 0),
			maximumTimerTaskKey,
			NewDomainFilter(nil, true),
		),
	}
}
