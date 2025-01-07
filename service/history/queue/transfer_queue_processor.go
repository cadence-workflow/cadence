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
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig"
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
	"github.com/uber/cadence/service/history/reset"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
	"github.com/uber/cadence/service/history/workflowcache"
	"github.com/uber/cadence/service/worker/archiver"
)

var (
	errUnexpectedQueueTask = errors.New("unexpected queue task")
	errProcessorShutdown   = errors.New("queue processor has been shutdown")

	maximumTransferTaskKey = newTransferTaskKey(math.MaxInt64)
)

type transferQueueProcessor struct {
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

	ackLevel                int64
	taskAllocator           TaskAllocator
	activeTaskExecutor      task.Executor
	activeQueueProcessor    *transferQueueProcessorBase
	standbyQueueProcessors  map[string]*transferQueueProcessorBase
	failoverQueueProcessors []*transferQueueProcessorBase
}

// NewTransferQueueProcessor creates a new transfer QueueProcessor
func NewTransferQueueProcessor(
	shard shard.Context,
	historyEngine engine.Engine,
	taskProcessor task.Processor,
	executionCache execution.Cache,
	workflowResetter reset.WorkflowResetter,
	archivalClient archiver.Client,
	executionCheck invariant.Invariant,
	wfIDCache workflowcache.WFCache,
	ratelimitInternalPerWorkflowID dynamicconfig.BoolPropertyFnWithDomainFilter,
) Processor {
	logger := shard.GetLogger().WithTags(tag.ComponentTransferQueue)
	currentClusterName := shard.GetClusterMetadata().GetCurrentClusterName()
	config := shard.GetConfig()
	taskAllocator := NewTaskAllocator(shard)

	activeTaskExecutor := task.NewTransferActiveTaskExecutor(
		shard,
		archivalClient,
		executionCache,
		workflowResetter,
		logger,
		config,
		wfIDCache,
		ratelimitInternalPerWorkflowID,
	)

	activeQueueProcessor := newTransferQueueActiveProcessor(
		shard,
		taskProcessor,
		taskAllocator,
		activeTaskExecutor,
		logger,
	)

	standbyQueueProcessors := make(map[string]*transferQueueProcessorBase)
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
		standbyTaskExecutor := task.NewTransferStandbyTaskExecutor(
			shard,
			archivalClient,
			executionCache,
			historyResender,
			logger,
			clusterName,
			config,
		)
		standbyQueueProcessors[clusterName] = newTransferQueueStandbyProcessor(
			clusterName,
			shard,
			taskProcessor,
			taskAllocator,
			standbyTaskExecutor,
			logger,
		)
	}

	return &transferQueueProcessor{
		shard:                  shard,
		historyEngine:          historyEngine,
		taskProcessor:          taskProcessor,
		config:                 config,
		currentClusterName:     currentClusterName,
		metricsClient:          shard.GetMetricsClient(),
		logger:                 logger,
		status:                 common.DaemonStatusInitialized,
		shutdownChan:           make(chan struct{}),
		ackLevel:               shard.GetTransferAckLevel(),
		taskAllocator:          taskAllocator,
		activeTaskExecutor:     activeTaskExecutor,
		activeQueueProcessor:   activeQueueProcessor,
		standbyQueueProcessors: standbyQueueProcessors,
	}
}

func (t *transferQueueProcessor) Start() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	t.activeQueueProcessor.Start()
	for _, standbyQueueProcessor := range t.standbyQueueProcessors {
		standbyQueueProcessor.Start()
	}

	t.shutdownWG.Add(1)
	go t.completeTransferLoop()
}

func (t *transferQueueProcessor) Stop() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	if !t.shard.GetConfig().QueueProcessorEnableGracefulSyncShutdown() {
		t.activeQueueProcessor.Stop()
		for _, standbyQueueProcessor := range t.standbyQueueProcessors {
			standbyQueueProcessor.Stop()
		}

		close(t.shutdownChan)
		common.AwaitWaitGroup(&t.shutdownWG, time.Minute)
		return
	}

	// close the shutdown channel so processor pump goroutine drains tasks and then stop the processors
	close(t.shutdownChan)
	if !common.AwaitWaitGroup(&t.shutdownWG, gracefulShutdownTimeout) {
		t.logger.Warn("transferQueueProcessor timed out on shut down", tag.LifeCycleStopTimedout)
	}
	t.activeQueueProcessor.Stop()
	for _, standbyQueueProcessor := range t.standbyQueueProcessors {
		standbyQueueProcessor.Stop()
	}

	if len(t.failoverQueueProcessors) > 0 {
		t.logger.Info("Shutting down failover transfer queues", tag.Counter(len(t.failoverQueueProcessors)))
		for _, failoverQueueProcessor := range t.failoverQueueProcessors {
			failoverQueueProcessor.Stop()
		}
	}
}

func (t *transferQueueProcessor) NotifyNewTask(clusterName string, info *hcommon.NotifyTaskInfo) {
	if len(info.Tasks) == 0 {
		return
	}

	if clusterName == t.currentClusterName {
		t.activeQueueProcessor.notifyNewTask(info)
		return
	}

	standbyQueueProcessor, ok := t.standbyQueueProcessors[clusterName]
	if !ok {
		panic(fmt.Sprintf("Cannot find transfer processor for %s.", clusterName))
	}
	standbyQueueProcessor.notifyNewTask(info)
}

func (t *transferQueueProcessor) FailoverDomain(domainIDs map[string]struct{}) {
	// Failover queue is used to scan all inflight tasks, if queue processor is not
	// started, there's no inflight task and we don't need to create a failover processor.
	// Also the HandleAction will be blocked if queue processor processing loop is not running.
	if atomic.LoadInt32(&t.status) != common.DaemonStatusStarted {
		return
	}

	minLevel := t.shard.GetTransferClusterAckLevel(t.currentClusterName)
	standbyClusterName := t.currentClusterName
	for clusterName := range t.shard.GetClusterMetadata().GetEnabledClusterInfo() {
		ackLevel := t.shard.GetTransferClusterAckLevel(clusterName)
		if ackLevel < minLevel {
			minLevel = ackLevel
			standbyClusterName = clusterName
		}
	}

	maxReadLevel := int64(0)
	actionResult, err := t.HandleAction(context.Background(), t.currentClusterName, NewGetStateAction())
	if err != nil {
		t.logger.Error("Transfer Failover Failed", tag.WorkflowDomainIDs(domainIDs), tag.Error(err))
		if err == errProcessorShutdown {
			// processor/shard already shutdown, we don't need to create failover queue processor
			return
		}
		// other errors should never be returned for GetStateAction
		panic(fmt.Sprintf("unknown error for GetStateAction: %v", err))
	}
	for _, queueState := range actionResult.GetStateActionResult.States {
		queueReadLevel := queueState.ReadLevel().(transferTaskKey).taskID
		if maxReadLevel < queueReadLevel {
			maxReadLevel = queueReadLevel
		}
	}
	// maxReadLevel is exclusive, so add 1
	maxReadLevel++

	t.logger.Info("Transfer Failover Triggered",
		tag.WorkflowDomainIDs(domainIDs),
		tag.MinLevel(minLevel),
		tag.MaxLevel(maxReadLevel))

	updateShardAckLevel, failoverQueueProcessor := newTransferQueueFailoverProcessor(
		t.shard,
		t.taskProcessor,
		t.taskAllocator,
		t.activeTaskExecutor,
		t.logger,
		minLevel,
		maxReadLevel,
		domainIDs,
		standbyClusterName,
	)

	// NOTE: READ REF BEFORE MODIFICATION
	// ref: historyEngine.go registerDomainFailoverCallback function
	err = updateShardAckLevel(newTransferTaskKey(minLevel))
	if err != nil {
		t.logger.Error("Error update shard ack level", tag.Error(err))
	}

	// Failover queue processors are started on the fly when domains are failed over.
	// Failover queue processors will be stopped when the transfer queue instance is stopped (due to restart or shard movement).
	// This means the failover queue processor might not finish its job.
	// There is no mechanism to re-start ongoing failover queue processors in the new shard owner.
	t.failoverQueueProcessors = append(t.failoverQueueProcessors, failoverQueueProcessor)
	failoverQueueProcessor.Start()
}

func (t *transferQueueProcessor) HandleAction(
	ctx context.Context,
	clusterName string,
	action *Action,
) (*ActionResult, error) {
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

func (t *transferQueueProcessor) LockTaskProcessing() {
	t.taskAllocator.Lock()
}

func (t *transferQueueProcessor) UnlockTaskProcessing() {
	t.taskAllocator.Unlock()
}

func (t *transferQueueProcessor) drain() {
	// before shutdown, make sure the ack level is up to date
	if err := t.completeTransfer(); err != nil {
		t.logger.Error("Failed to complete transfer task during shutdown", tag.Error(err))
	}
}

func (t *transferQueueProcessor) completeTransferLoop() {
	defer t.shutdownWG.Done()

	completeTimer := time.NewTimer(t.config.TransferProcessorCompleteTransferInterval())
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
			for attempt := 0; attempt < t.config.TransferProcessorCompleteTransferFailureRetryCount(); attempt++ {
				err := t.completeTransfer()
				if err == nil {
					break
				}

				t.logger.Error("Failed to complete transfer task", tag.Error(err))
				var errShardClosed *shard.ErrShardClosed
				if errors.As(err, &errShardClosed) {
					// shard closed, trigger shutdown and bail out
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

			completeTimer.Reset(t.config.TransferProcessorCompleteTransferInterval())
		}
	}
}

func (t *transferQueueProcessor) completeTransfer() error {
	newAckLevel := maximumTransferTaskKey
	actionResult, err := t.HandleAction(context.Background(), t.currentClusterName, NewGetStateAction())
	if err != nil {
		return err
	}
	for _, queueState := range actionResult.GetStateActionResult.States {
		newAckLevel = minTaskKey(newAckLevel, queueState.AckLevel())
	}

	for standbyClusterName := range t.standbyQueueProcessors {
		actionResult, err := t.HandleAction(context.Background(), standbyClusterName, NewGetStateAction())
		if err != nil {
			return err
		}
		for _, queueState := range actionResult.GetStateActionResult.States {
			newAckLevel = minTaskKey(newAckLevel, queueState.AckLevel())
		}
	}

	for _, failoverInfo := range t.shard.GetAllTransferFailoverLevels() {
		failoverLevel := newTransferTaskKey(failoverInfo.MinLevel)
		if newAckLevel == nil {
			newAckLevel = failoverLevel
		} else {
			newAckLevel = minTaskKey(newAckLevel, failoverLevel)
		}
	}

	if newAckLevel == nil {
		panic("Unable to get transfer queue processor ack level")
	}

	newAckLevelTaskID := newAckLevel.(transferTaskKey).taskID
	t.logger.Debug(fmt.Sprintf("Start completing transfer task from: %v, to %v.", t.ackLevel, newAckLevelTaskID))
	if t.ackLevel >= newAckLevelTaskID {
		return nil
	}

	t.metricsClient.Scope(metrics.TransferQueueProcessorScope).
		Tagged(metrics.ShardIDTag(t.shard.GetShardID())).
		IncCounter(metrics.TaskBatchCompleteCounter)

	for {
		pageSize := t.config.TransferTaskDeleteBatchSize()
		resp, err := t.shard.GetExecutionManager().RangeCompleteTransferTask(context.Background(), &persistence.RangeCompleteTransferTaskRequest{
			ExclusiveBeginTaskID: t.ackLevel,
			InclusiveEndTaskID:   newAckLevelTaskID,
			PageSize:             pageSize, // pageSize may or may not be honored
		})
		if err != nil {
			return err
		}
		if !persistence.HasMoreRowsToDelete(resp.TasksCompleted, pageSize) {
			break
		}
	}

	t.ackLevel = newAckLevelTaskID

	return t.shard.UpdateTransferAckLevel(newAckLevelTaskID)
}

func newTransferQueueActiveProcessor(
	shard shard.Context,
	taskProcessor task.Processor,
	taskAllocator TaskAllocator,
	taskExecutor task.Executor,
	logger log.Logger,
) *transferQueueProcessorBase {
	config := shard.GetConfig()
	options := newTransferQueueProcessorOptions(config, true, false)

	currentClusterName := shard.GetClusterMetadata().GetCurrentClusterName()
	logger = logger.WithTags(tag.ClusterName(currentClusterName))

	taskFilter := func(taskInfo task.Info) (bool, error) {
		task, ok := taskInfo.(*persistence.TransferTaskInfo)
		if !ok {
			return false, errUnexpectedQueueTask
		}
		if notRegistered, err := isDomainNotRegistered(shard, task.DomainID); notRegistered && err == nil {
			logger.Info("Domain is not in registered status, skip task in active transfer queue.", tag.WorkflowDomainID(task.DomainID), tag.Value(taskInfo))
			return false, nil
		}
		return taskAllocator.VerifyActiveTask(task.DomainID, task)
	}

	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(shard.GetTransferMaxReadLevel())
	}

	updateClusterAckLevel := func(ackLevel task.Key) error {
		taskID := ackLevel.(transferTaskKey).taskID
		return shard.UpdateTransferClusterAckLevel(currentClusterName, taskID)
	}

	updateProcessingQueueStates := func(states []ProcessingQueueState) error {
		pStates := convertToPersistenceTransferProcessingQueueStates(states)
		return shard.UpdateTransferProcessingQueueStates(currentClusterName, pStates)
	}

	queueShutdown := func() error {
		return nil
	}

	return newTransferQueueProcessorBase(
		shard,
		loadTransferProcessingQueueStates(currentClusterName, shard, options, logger),
		taskProcessor,
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		queueShutdown,
		taskFilter,
		taskExecutor,
		logger,
		shard.GetMetricsClient(),
	)
}

func newTransferQueueStandbyProcessor(
	clusterName string,
	shard shard.Context,
	taskProcessor task.Processor,
	taskAllocator TaskAllocator,
	taskExecutor task.Executor,
	logger log.Logger,
) *transferQueueProcessorBase {
	config := shard.GetConfig()
	options := newTransferQueueProcessorOptions(config, false, false)

	logger = logger.WithTags(tag.ClusterName(clusterName))

	taskFilter := func(taskInfo task.Info) (bool, error) {
		task, ok := taskInfo.(*persistence.TransferTaskInfo)
		if !ok {
			return false, errUnexpectedQueueTask
		}
		if notRegistered, err := isDomainNotRegistered(shard, task.DomainID); notRegistered && err == nil {
			logger.Info("Domain is not in registered status, skip task in standby transfer queue.", tag.WorkflowDomainID(task.DomainID), tag.Value(taskInfo))
			return false, nil
		}
		if task.TaskType == persistence.TransferTaskTypeCloseExecution ||
			task.TaskType == persistence.TransferTaskTypeRecordWorkflowClosed {
			domainEntry, err := shard.GetDomainCache().GetDomainByID(task.DomainID)
			if err == nil {
				if domainEntry.HasReplicationCluster(clusterName) {
					// guarantee the processing of workflow execution close
					return true, nil
				}
			} else {
				if _, ok := err.(*types.EntityNotExistsError); !ok {
					// retry the task if failed to find the domain
					logger.Warn("Cannot find domain", tag.WorkflowDomainID(task.DomainID))
					return false, err
				}
				logger.Warn("Cannot find domain, default to not process task.", tag.WorkflowDomainID(task.DomainID), tag.Value(task))
				return false, nil
			}
		}
		return taskAllocator.VerifyStandbyTask(clusterName, task.DomainID, task)
	}

	updateMaxReadLevel := func() task.Key {
		return newTransferTaskKey(shard.GetTransferMaxReadLevel())
	}

	updateClusterAckLevel := func(ackLevel task.Key) error {
		taskID := ackLevel.(transferTaskKey).taskID
		return shard.UpdateTransferClusterAckLevel(clusterName, taskID)
	}

	updateProcessingQueueStates := func(states []ProcessingQueueState) error {
		pStates := convertToPersistenceTransferProcessingQueueStates(states)
		return shard.UpdateTransferProcessingQueueStates(clusterName, pStates)
	}

	queueShutdown := func() error {
		return nil
	}

	return newTransferQueueProcessorBase(
		shard,
		loadTransferProcessingQueueStates(clusterName, shard, options, logger),
		taskProcessor,
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		queueShutdown,
		taskFilter,
		taskExecutor,
		logger,
		shard.GetMetricsClient(),
	)
}

func newTransferQueueFailoverProcessor(
	shardContext shard.Context,
	taskProcessor task.Processor,
	taskAllocator TaskAllocator,
	taskExecutor task.Executor,
	logger log.Logger,
	minLevel, maxLevel int64,
	domainIDs map[string]struct{},
	standbyClusterName string,
) (updateClusterAckLevelFn, *transferQueueProcessorBase) {
	config := shardContext.GetConfig()
	options := newTransferQueueProcessorOptions(config, true, true)

	currentClusterName := shardContext.GetService().GetClusterMetadata().GetCurrentClusterName()
	failoverUUID := uuid.New()
	logger = logger.WithTags(
		tag.ClusterName(currentClusterName),
		tag.WorkflowDomainIDs(domainIDs),
		tag.FailoverMsg("from: "+standbyClusterName),
	)

	taskFilter := func(taskInfo task.Info) (bool, error) {
		task, ok := taskInfo.(*persistence.TransferTaskInfo)
		if !ok {
			return false, errUnexpectedQueueTask
		}
		if notRegistered, err := isDomainNotRegistered(shardContext, task.DomainID); notRegistered && err == nil {
			logger.Info("Domain is not in registered status, skip task in failover transfer queue.", tag.WorkflowDomainID(task.DomainID), tag.Value(taskInfo))
			return false, nil
		}
		return taskAllocator.VerifyFailoverActiveTask(domainIDs, task.DomainID, task)
	}

	maxReadLevelTaskKey := newTransferTaskKey(maxLevel)
	updateMaxReadLevel := func() task.Key {
		return maxReadLevelTaskKey // this is a const
	}

	updateClusterAckLevel := func(ackLevel task.Key) error {
		taskID := ackLevel.(transferTaskKey).taskID
		return shardContext.UpdateTransferFailoverLevel(
			failoverUUID,
			shard.TransferFailoverLevel{
				StartTime:    shardContext.GetTimeSource().Now(),
				MinLevel:     minLevel,
				CurrentLevel: taskID,
				MaxLevel:     maxLevel,
				DomainIDs:    domainIDs,
			},
		)
	}

	queueShutdown := func() error {
		return shardContext.DeleteTransferFailoverLevel(failoverUUID)
	}

	processingQueueStates := []ProcessingQueueState{
		NewProcessingQueueState(
			defaultProcessingQueueLevel,
			newTransferTaskKey(minLevel),
			maxReadLevelTaskKey,
			NewDomainFilter(domainIDs, false),
		),
	}

	return updateClusterAckLevel, newTransferQueueProcessorBase(
		shardContext,
		processingQueueStates,
		taskProcessor,
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		nil,
		queueShutdown,
		taskFilter,
		taskExecutor,
		logger,
		shardContext.GetMetricsClient(),
	)
}

func loadTransferProcessingQueueStates(
	clusterName string,
	shard shard.Context,
	options *queueProcessorOptions,
	logger log.Logger,
) []ProcessingQueueState {
	ackLevel := shard.GetTransferClusterAckLevel(clusterName)
	if options.EnableLoadQueueStates() {
		pStates := shard.GetTransferProcessingQueueStates(clusterName)
		if validateProcessingQueueStates(pStates, ackLevel) {
			return convertFromPersistenceTransferProcessingQueueStates(pStates)
		}

		logger.Error("Incompatible processing queue states and ackLevel",
			tag.Value(pStates),
			tag.ShardTransferAcks(ackLevel),
		)
	}

	// LoadQueueStates is disabled or sanity check failed
	// fallback to use ackLevel
	return []ProcessingQueueState{
		NewProcessingQueueState(
			defaultProcessingQueueLevel,
			newTransferTaskKey(ackLevel),
			maximumTransferTaskKey,
			NewDomainFilter(nil, true),
		),
	}
}
