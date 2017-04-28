package history

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-common/bark"

	"github.com/uber/cadence/.gen/go/history"
	m "github.com/uber/cadence/.gen/go/matching"
	workflow "github.com/uber/cadence/.gen/go/shared"
	hc "github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
)

const (
	transferTaskBatchSize              = 10
	transferProcessorMaxPollRPS        = 100
	transferProcessorMaxPollInterval   = 10 * time.Second
	transferProcessorUpdateAckInterval = 10 * time.Second
	taskWorkerCount                    = 10
)

type (
	transferQueueProcessorImpl struct {
		shard             ShardContext
		ackMgr            *ackManager
		executionManager  persistence.ExecutionManager
		visibilityManager persistence.VisibilityManager
		matchingClient    matching.Client
		historyClient     hc.Client
		cache             *historyCache
		rateLimiter       common.TokenBucket // Read rate limiter
		appendCh          chan struct{}
		isStarted         int32
		isStopped         int32
		shutdownWG        sync.WaitGroup
		shutdownCh        chan struct{}
		logger            bark.Logger
		metricsClient     metrics.Client
	}

	// ackManager is created by transferQueueProcessor to keep track of the transfer queue ackLevel for the shard.
	// It keeps track of read level when dispatching transfer tasks to processor and maintains a map of outstanding tasks.
	// Outstanding tasks map uses the task id sequencer as the key, which is used by updateAckLevel to move the ack level
	// for the shard when all preceding tasks are acknowledged.
	ackManager struct {
		processor    transferQueueProcessor
		shard        ShardContext
		executionMgr persistence.ExecutionManager
		logger       bark.Logger

		sync.RWMutex
		outstandingTasks map[int64]bool
		readLevel        int64
		maxReadLevel     int64
		ackLevel         int64
	}
)

func newTransferQueueProcessor(shard ShardContext, visibilityMgr persistence.VisibilityManager,
	matching matching.Client, historyClient hc.Client, cache *historyCache) transferQueueProcessor {
	executionManager := shard.GetExecutionManager()
	logger := shard.GetLogger()
	processor := &transferQueueProcessorImpl{
		shard:             shard,
		executionManager:  executionManager,
		matchingClient:    matching,
		historyClient:     historyClient,
		visibilityManager: visibilityMgr,
		cache:             cache,
		rateLimiter:       common.NewTokenBucket(transferProcessorMaxPollRPS, common.NewRealTimeSource()),
		appendCh:          make(chan struct{}, 1),
		shutdownCh:        make(chan struct{}),
		logger: logger.WithFields(bark.Fields{
			tagWorkflowComponent: tagValueTransferQueueComponent,
		}),
		metricsClient: shard.GetMetricsClient(),
	}
	processor.ackMgr = newAckManager(processor, shard, executionManager, logger)

	return processor
}

func newAckManager(processor transferQueueProcessor, shard ShardContext, executionMgr persistence.ExecutionManager,
	logger bark.Logger) *ackManager {
	ackLevel := shard.GetTransferAckLevel()
	return &ackManager{
		processor:        processor,
		shard:            shard,
		executionMgr:     executionMgr,
		outstandingTasks: make(map[int64]bool),
		readLevel:        ackLevel,
		ackLevel:         ackLevel,
		logger:           logger,
	}
}

func (t *transferQueueProcessorImpl) Start() {
	if !atomic.CompareAndSwapInt32(&t.isStarted, 0, 1) {
		return
	}

	logTransferQueueProcesorStartingEvent(t.logger)
	defer logTransferQueueProcesorStartedEvent(t.logger)

	t.shutdownWG.Add(1)
	t.NotifyNewTask()
	go t.processorPump()
}

func (t *transferQueueProcessorImpl) Stop() {
	if !atomic.CompareAndSwapInt32(&t.isStopped, 0, 1) {
		return
	}

	logTransferQueueProcesorShuttingDownEvent(t.logger)
	defer logTransferQueueProcesorShutdownEvent(t.logger)

	if atomic.LoadInt32(&t.isStarted) == 1 {
		close(t.shutdownCh)
	}

	if success := common.AwaitWaitGroup(&t.shutdownWG, time.Minute); !success {
		logTransferQueueProcesorShutdownTimedoutEvent(t.logger)
	}
}

func (t *transferQueueProcessorImpl) NotifyNewTask() {
	var event struct{}
	select {
	case t.appendCh <- event:
	default: // channel already has an event, don't block
	}
}

func (t *transferQueueProcessorImpl) processorPump() {
	defer t.shutdownWG.Done()
	tasksCh := make(chan *persistence.TransferTaskInfo, transferTaskBatchSize)

	var workerWG sync.WaitGroup
	for i := 0; i < taskWorkerCount; i++ {
		workerWG.Add(1)
		go t.taskWorker(tasksCh, &workerWG)
	}

	pollTimer := time.NewTimer(transferProcessorMaxPollInterval)
	defer pollTimer.Stop()
	updateAckTimer := time.NewTimer(transferProcessorUpdateAckInterval)
	defer updateAckTimer.Stop()
	for {
		select {
		case <-t.shutdownCh:
			t.logger.Info("Transfer queue processor pump shutting down.")
			// This is the only pump which writes to tasksCh, so it is safe to close channel here
			close(tasksCh)
			if success := common.AwaitWaitGroup(&workerWG, 10*time.Second); !success {
				t.logger.Warn("Transfer queue processor timed out on worker shutdown.")
			}
			return
		case <-t.appendCh:
			t.processTransferTasks(tasksCh)
		case <-pollTimer.C:
			t.processTransferTasks(tasksCh)
		case <-updateAckTimer.C:
			t.ackMgr.updateAckLevel()
		}
	}
}

func (t *transferQueueProcessorImpl) processTransferTasks(tasksCh chan<- *persistence.TransferTaskInfo) {

	if !t.rateLimiter.Consume(1, transferProcessorMaxPollInterval) {
		t.NotifyNewTask() // re-enqueue the event
		return
	}

	tasks, err := t.ackMgr.readTransferTasks()

	if err != nil {
		t.logger.Warnf("Processor unable to retrieve transfer tasks: %v", err)
		t.NotifyNewTask() // re-enqueue the event
		return
	}

	if len(tasks) == 0 {
		return
	}

	for _, tsk := range tasks {
		tasksCh <- tsk
	}

	// There might be more task
	// We return now to yield, but enqueue an event to poll later
	t.NotifyNewTask()
	return
}

func (t *transferQueueProcessorImpl) taskWorker(tasksCh <-chan *persistence.TransferTaskInfo, workerWG *sync.WaitGroup) {
	defer workerWG.Done()
	for {
		select {
		case task, ok := <-tasksCh:
			if !ok {
				return
			}

			t.processTransferTask(task)
		}
	}
}

func (t *transferQueueProcessorImpl) processTransferTask(task *persistence.TransferTaskInfo) {
	t.logger.Debugf("Processing transfer task: %v, type: %v", task.TaskID, task.TaskType)
	t.metricsClient.AddCounter(metrics.HistoryProcessTransferTasksScope, metrics.TransferTasksProcessedCounter, 1)
ProcessRetryLoop:
	for retryCount := 1; retryCount <= 100; retryCount++ {
		select {
		case <-t.shutdownCh:
			return
		default:
			var err error
			domainID := task.DomainID
			targetDomainID := task.TargetDomainID
			execution := workflow.WorkflowExecution{WorkflowId: common.StringPtr(task.WorkflowID),
				RunId: common.StringPtr(task.RunID)}
			switch task.TaskType {
			case persistence.TransferTaskTypeActivityTask:
				{
					taskList := &workflow.TaskList{
						Name: &task.TaskList,
					}
					err = t.matchingClient.AddActivityTask(nil, &m.AddActivityTaskRequest{
						DomainUUID:       common.StringPtr(targetDomainID),
						SourceDomainUUID: common.StringPtr(domainID),
						Execution:        &execution,
						TaskList:         taskList,
						ScheduleId:       &task.ScheduleID,
					})
				}
			case persistence.TransferTaskTypeDecisionTask:
				{
					if task.ScheduleID == firstEventID+1 {
						err = t.recordWorkflowExecutionStarted(execution, task)
					}

					if err == nil {
						taskList := &workflow.TaskList{
							Name: &task.TaskList,
						}
						err = t.matchingClient.AddDecisionTask(nil, &m.AddDecisionTaskRequest{
							DomainUUID: common.StringPtr(domainID),
							Execution:  &execution,
							TaskList:   taskList,
							ScheduleId: &task.ScheduleID,
						})
					}
				}
			case persistence.TransferTaskTypeDeleteExecution:
				{
					context, release, _ := t.cache.getOrCreateWorkflowExecution(domainID, execution)

					// TODO: We need to keep completed executions for auditing purpose.  Need a design for keeping them around
					// for visibility purpose.
					var mb *mutableStateBuilder
					mb, err = context.loadWorkflowExecution()
					if err == nil {
						err = t.visibilityManager.RecordWorkflowExecutionClosed(&persistence.RecordWorkflowExecutionClosedRequest{
							DomainUUID:       task.DomainID,
							Execution:        execution,
							WorkflowTypeName: mb.executionInfo.WorkflowTypeName,
							StartTimestamp:   mb.executionInfo.StartTimestamp.UnixNano(),
							CloseTimestamp:   mb.executionInfo.LastUpdatedTimestamp.UnixNano(),
						})
						if err == nil {
							err = context.deleteWorkflowExecution()
						}
					}
					release()
				}

			case persistence.TransferTaskTypeCancelExecution:
				{
					var context *workflowExecutionContext
					var release releaseWorkflowExecutionFunc
					context, release, err = t.cache.getOrCreateWorkflowExecution(domainID, execution)
					if err == nil {
						// Load workflow execution.
						_, err = context.loadWorkflowExecution()
						if err == nil {
							cancelRequest := &history.RequestCancelWorkflowExecutionRequest{
								DomainUUID: common.StringPtr(targetDomainID),
								CancelRequest: &workflow.RequestCancelWorkflowExecutionRequest{
									WorkflowExecution: &workflow.WorkflowExecution{
										WorkflowId: common.StringPtr(task.TargetWorkflowID),
										RunId:      common.StringPtr(task.TargetRunID),
									},
									Identity: common.StringPtr("history-service"),
								},
								ExternalInitiatedEventId: common.Int64Ptr(task.ScheduleID),
								ExternalWorkflowExecution: &workflow.WorkflowExecution{
									WorkflowId: common.StringPtr(task.WorkflowID),
									RunId:      common.StringPtr(task.RunID),
								},
							}

							err = context.requestExternalCancelWorkflowExecutionWithRetry(
								t.historyClient,
								cancelRequest,
								task.ScheduleID)
						}
						release()
					}
				}
			}

			if err != nil {
				t.logger.WithField("error", err).Warn("Processor failed to create task")
				backoff := time.Duration(retryCount * 100)
				time.Sleep(backoff * time.Millisecond)
				continue ProcessRetryLoop
			}

			t.ackMgr.completeTask(task.TaskID)
			return
		}
	}

	// All attempts to process transfer task failed.  We won't be able to move the ackLevel so panic
	t.logger.Fatalf("Retry count exceeded for transfer taskID: %v", task.TaskID)
}

func (t *transferQueueProcessorImpl) recordWorkflowExecutionStarted(
	execution workflow.WorkflowExecution, task *persistence.TransferTaskInfo) error {
	context, release, err := t.cache.getOrCreateWorkflowExecution(task.DomainID, execution)
	if err != nil {
		return err
	}
	defer release()
	mb, err := context.loadWorkflowExecution()
	if err != nil {
		return err
	}

	err = t.visibilityManager.RecordWorkflowExecutionStarted(&persistence.RecordWorkflowExecutionStartedRequest{
		DomainUUID:       task.DomainID,
		Execution:        execution,
		WorkflowTypeName: mb.executionInfo.WorkflowTypeName,
		StartTimestamp:   mb.executionInfo.StartTimestamp.UnixNano(),
	})

	return err
}

func (a *ackManager) readTransferTasks() ([]*persistence.TransferTaskInfo, error) {
	a.RLock()
	rLevel := a.readLevel
	a.RUnlock()
	response, err := a.executionMgr.GetTransferTasks(&persistence.GetTransferTasksRequest{
		ReadLevel:    rLevel,
		MaxReadLevel: a.shard.GetTransferMaxReadLevel(),
		BatchSize:    transferTaskBatchSize,
	})

	if err != nil {
		return nil, err
	}

	tasks := response.Tasks
	if len(tasks) == 0 {
		return tasks, nil
	}

	a.Lock()
	for _, task := range tasks {
		if a.readLevel >= task.TaskID {
			a.logger.Fatalf("Next task ID is less than current read level.  TaskID: %v, ReadLevel: %v", task.TaskID,
				a.readLevel)
		}
		a.logger.Debugf("Moving read level: %v", task.TaskID)
		a.readLevel = task.TaskID
		a.outstandingTasks[a.readLevel] = false
	}
	a.Unlock()

	return tasks, nil
}

func (a *ackManager) completeTask(taskID int64) {
	a.Lock()
	if _, ok := a.outstandingTasks[taskID]; ok {
		a.outstandingTasks[taskID] = true
	}
	a.Unlock()
}

func (a *ackManager) updateAckLevel() {
	updatedAckLevel := a.ackLevel
	a.Lock()
MoveAckLevelLoop:
	for current := a.ackLevel + 1; current <= a.readLevel; current++ {
		if acked, ok := a.outstandingTasks[current]; ok {
			if acked {
				err := a.executionMgr.CompleteTransferTask(&persistence.CompleteTransferTaskRequest{TaskID: current})

				if err != nil {
					a.logger.Warnf("Processor unable to complete transfer task '%v': %v", current, err)
					break MoveAckLevelLoop
				}
				a.logger.Debugf("Updating ack level: %v", current)
				a.ackLevel = current
				updatedAckLevel = current
				delete(a.outstandingTasks, current)
			} else {
				break MoveAckLevelLoop
			}
		}
	}
	a.Unlock()

	// Always update ackLevel to detect if the shared is stolen
	if err := a.shard.UpdateAckLevel(updatedAckLevel); err != nil {
		logOperationFailedEvent(a.logger, "Error updating ack level for shard", err)
	}

}

func minDuration(x, y time.Duration) time.Duration {
	if x < y {
		return x
	}

	return y
}
