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
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

const (
	numTasksEstimationDecay = 0.6
)

var (
	loadQueueTaskThrottleRetryDelay = 5 * time.Second
	persistenceOperationRetryPolicy = common.CreatePersistenceRetryPolicy()
)

type (
	transferTaskKey struct {
		taskID int64
	}

	transferQueueProcessorBase struct {
		*processorBase

		taskInitializer task.Initializer

		notifyCh  chan struct{}
		processCh chan struct{}

		// for managing if a processing queue collection should be processed
		startJitterTimer *time.Timer
		processingLock   sync.Mutex
		backoffTimer     map[int]*time.Timer
		shouldProcess    map[int]bool

		// for estimating the look ahead taskID during split
		lastSplitTime           time.Time
		lastMaxReadLevel        int64
		estimatedTasksPerMinute int64

		// for validating if the queue failed to load any tasks
		validator *transferQueueValidator

		processQueueCollectionsFn func()
		updateAckLevelFn          func() (bool, task.Key, error)
	}
)

func newTransferQueueProcessorBase(
	shard shard.Context,
	processingQueueStates []ProcessingQueueState,
	taskProcessor task.Processor,
	options *queueProcessorOptions,
	updateMaxReadLevel updateMaxReadLevelFn,
	updateClusterAckLevel updateClusterAckLevelFn,
	updateProcessingQueueStates updateProcessingQueueStatesFn,
	queueShutdown queueShutdownFn,
	taskFilter task.Filter,
	taskExecutor task.Executor,
	logger log.Logger,
	metricsClient metrics.Client,
) *transferQueueProcessorBase {
	processorBase := newProcessorBase(
		shard,
		processingQueueStates,
		taskProcessor,
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		queueShutdown,
		logger.WithTags(tag.ComponentTransferQueue),
		metricsClient,
	)

	queueType := task.QueueTypeActiveTransfer
	if options.MetricScope == metrics.TransferStandbyQueueProcessorScope {
		queueType = task.QueueTypeStandbyTransfer
	}

	transferQueueProcessorBase := &transferQueueProcessorBase{
		processorBase: processorBase,
		taskInitializer: func(taskInfo persistence.Task) task.Task {
			return task.NewHistoryTask(
				shard,
				taskInfo,
				queueType,
				task.InitializeLoggerForTask(shard.GetShardID(), taskInfo, logger),
				taskFilter,
				taskExecutor,
				taskProcessor,
				processorBase.redispatcher.AddTask,
				shard.GetConfig().TaskCriticalRetryCount,
			)
		},
		notifyCh:         make(chan struct{}, 1),
		processCh:        make(chan struct{}, 1),
		backoffTimer:     make(map[int]*time.Timer),
		shouldProcess:    make(map[int]bool),
		lastSplitTime:    time.Time{},
		lastMaxReadLevel: 0,
	}

	transferQueueProcessorBase.processQueueCollectionsFn = transferQueueProcessorBase.processQueueCollections
	transferQueueProcessorBase.updateAckLevelFn = transferQueueProcessorBase.updateAckLevel

	if shard.GetConfig().EnableDebugMode && options.EnableValidator() {
		transferQueueProcessorBase.validator = newTransferQueueValidator(
			transferQueueProcessorBase,
			options.ValidationInterval,
			logger,
			processorBase.metricsScope,
		)
	}

	return transferQueueProcessorBase
}

func (t *transferQueueProcessorBase) Start() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	t.logger.Info("Transfer queue processor state changed", tag.LifeCycleStarting)
	defer t.logger.Info("Transfer queue processor state changed", tag.LifeCycleStarted)

	t.redispatcher.Start()

	// trigger an initial (maybe delayed) load of tasks
	if startJitter := t.options.MaxStartJitterInterval(); startJitter > 0 {
		t.startJitterTimer = time.AfterFunc(
			time.Duration(rand.Int63n(int64(startJitter))),
			func() {
				t.notifyAllQueueCollections()
			},
		)
	} else {
		t.notifyAllQueueCollections()
	}

	t.shutdownWG.Add(1)
	go t.processorPump()
}

func (t *transferQueueProcessorBase) Stop() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	t.logger.Info("Transfer queue processor state changed", tag.LifeCycleStopping)

	close(t.shutdownCh)
	if t.startJitterTimer != nil {
		t.startJitterTimer.Stop()
	}
	t.processingLock.Lock()
	for _, timer := range t.backoffTimer {
		timer.Stop()
	}
	for level := range t.shouldProcess {
		t.shouldProcess[level] = false
	}
	t.processingLock.Unlock()

	if success := common.AwaitWaitGroup(&t.shutdownWG, gracefulShutdownTimeout); !success {
		t.logger.Warn("transferQueueProcessorBase timed out on shut down", tag.LifeCycleStopTimedout)
	}

	t.redispatcher.Stop()
	t.logger.Info("Transfer queue processor state changed", tag.LifeCycleStopped)
}

func (t *transferQueueProcessorBase) notifyNewTask(info *hcommon.NotifyTaskInfo) {
	select {
	case t.notifyCh <- struct{}{}:
	default:
	}

	if info.ExecutionInfo != nil && t.validator != nil {
		// executionInfo will be nil when notifyNewTask is called to trigger a scan, for example during domain failover or sync shard.
		t.validator.addTasks(info)
	}
}

func (t *transferQueueProcessorBase) readyForProcess(level int) {
	t.processingLock.Lock()
	defer t.processingLock.Unlock()

	if _, ok := t.backoffTimer[level]; ok {
		// current level is being throttled
		return
	}

	t.shouldProcess[level] = true

	// trigger the actual processing
	select {
	case t.processCh <- struct{}{}:
	default:
	}
}

func (t *transferQueueProcessorBase) setupBackoffTimer(level int) {
	t.processingLock.Lock()
	defer t.processingLock.Unlock()

	if _, ok := t.backoffTimer[level]; ok {
		// there's an existing backoff timer, no-op
		// this case should not happen
		return
	}

	t.metricsScope.IncCounter(metrics.ProcessingQueueThrottledCounter)
	t.logger.Info("Throttled processing queue", tag.QueueLevel(level))

	backoffDuration := backoff.JitDuration(
		t.options.PollBackoffInterval(),
		t.options.PollBackoffIntervalJitterCoefficient(),
	)
	t.backoffTimer[level] = time.AfterFunc(backoffDuration, func() {
		select {
		case <-t.shutdownCh:
			return
		default:
		}

		t.processingLock.Lock()
		defer t.processingLock.Unlock()

		t.shouldProcess[level] = true
		delete(t.backoffTimer, level)

		// trigger the actual processing
		select {
		case t.processCh <- struct{}{}:
		default:
		}
	})
}

func (t *transferQueueProcessorBase) processorPump() {
	defer t.shutdownWG.Done()

	updateAckTimer := time.NewTimer(backoff.JitDuration(
		t.options.UpdateAckInterval(),
		t.options.UpdateAckIntervalJitterCoefficient(),
	))
	defer updateAckTimer.Stop()

	splitQueueTimer := time.NewTimer(backoff.JitDuration(
		t.options.SplitQueueInterval(),
		t.options.SplitQueueIntervalJitterCoefficient(),
	))
	defer splitQueueTimer.Stop()

	maxPollTimer := time.NewTimer(backoff.JitDuration(
		t.options.MaxPollInterval(),
		t.options.MaxPollIntervalJitterCoefficient(),
	))
	defer maxPollTimer.Stop()

	for {
		select {
		case <-t.shutdownCh:
			return
		case <-t.notifyCh:
			// notify all queue collections as they are waiting for the notification when there's
			// no more task to process. For non-default queue, if we choose to do periodic polling
			// in the future, then we don't need to notify them.
			t.notifyAllQueueCollections()
		case <-maxPollTimer.C:
			t.notifyAllQueueCollections()
			maxPollTimer.Reset(backoff.JitDuration(
				t.options.MaxPollInterval(),
				t.options.MaxPollIntervalJitterCoefficient(),
			))
		case <-t.processCh:
			maxRedispatchQueueSize := t.options.MaxRedispatchQueueSize()
			if redispathSize := t.redispatcher.Size(); redispathSize > maxRedispatchQueueSize {
				t.logger.Debugf("Transfer queue has too many pending tasks in re-dispatch queue: %v > maxRedispatchQueueSize: %v, block loading tasks from persistence", redispathSize, maxRedispatchQueueSize)
				t.redispatcher.Redispatch(maxRedispatchQueueSize)
				if redispathSize := t.redispatcher.Size(); redispathSize > maxRedispatchQueueSize {
					// if redispatcher still has a large number of tasks
					// this only happens when system is under very high load
					// we should backoff here instead of keeping submitting tasks to task processor
					t.logger.Debugf("Transfer queue still has too many pending tasks in re-dispatch queue: %v > maxRedispatchQueueSize: %v, backing off for %v", redispathSize, maxRedispatchQueueSize, t.options.PollBackoffInterval())
					time.Sleep(backoff.JitDuration(
						t.options.PollBackoffInterval(),
						t.options.PollBackoffIntervalJitterCoefficient(),
					))
				}
				// re-enqueue the event to see if we need keep re-dispatching or load new tasks from persistence
				select {
				case t.processCh <- struct{}{}:
				default:
				}
			} else {
				t.processQueueCollectionsFn()
			}
		case <-updateAckTimer.C:
			processFinished, _, err := t.updateAckLevelFn()
			var errShardClosed *shard.ErrShardClosed
			if errors.As(err, &errShardClosed) || (err == nil && processFinished) {
				if !t.options.EnableGracefulSyncShutdown() {
					go t.Stop()
					return
				}

				t.Stop()
				return
			}
			updateAckTimer.Reset(backoff.JitDuration(
				t.options.UpdateAckInterval(),
				t.options.UpdateAckIntervalJitterCoefficient(),
			))
		case <-splitQueueTimer.C:
			t.splitQueue()
			splitQueueTimer.Reset(backoff.JitDuration(
				t.options.SplitQueueInterval(),
				t.options.SplitQueueIntervalJitterCoefficient(),
			))
		case notification := <-t.actionNotifyCh:
			t.handleActionNotification(notification)
		}
	}
}

func (t *transferQueueProcessorBase) notifyAllQueueCollections() {
	for _, queueCollection := range t.processingQueueCollections {
		t.readyForProcess(queueCollection.Level())
	}
}

func (t *transferQueueProcessorBase) processQueueCollections() {
	for _, queueCollection := range t.processingQueueCollections {
		level := queueCollection.Level()
		t.processingLock.Lock()
		if shouldProcess, ok := t.shouldProcess[level]; !ok || !shouldProcess {
			t.processingLock.Unlock()
			continue
		}
		t.shouldProcess[level] = false
		t.processingLock.Unlock()

		activeQueue := queueCollection.ActiveQueue()
		if activeQueue == nil {
			// process for this queue collection has finished
			// it's possible that new queue will be added to this collection later though,
			// pollTime will be updated after split/merge
			continue
		}

		readLevel := activeQueue.State().ReadLevel()
		maxReadLevel := minTaskKey(activeQueue.State().MaxLevel(), t.updateMaxReadLevel())
		domainFilter := activeQueue.State().DomainFilter()

		if !readLevel.Less(maxReadLevel) {
			// no task need to be processed for now, wait for new task notification
			// note that if taskID for new task is still less than readLevel, the notification
			// will just be a no-op and there's no DB requests.
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), loadQueueTaskThrottleRetryDelay)
		if err := t.rateLimiter.Wait(ctx); err != nil {
			cancel()
			if level != defaultProcessingQueueLevel {
				t.setupBackoffTimer(level)
			} else {
				t.readyForProcess(level)
			}
			continue
		}
		cancel()

		transferTaskInfos, more, err := t.readTasks(readLevel, maxReadLevel)
		if err != nil {
			t.logger.Error("Processor unable to retrieve tasks", tag.Error(err))
			t.readyForProcess(level) // re-enqueue the event
			continue
		}
		t.logger.Debug("load transfer tasks from database",
			tag.ReadLevel(readLevel.(transferTaskKey).taskID),
			tag.MaxLevel(maxReadLevel.(transferTaskKey).taskID),
			tag.Counter(len(transferTaskInfos)))

		tasks := make(map[task.Key]task.Task)
		taskChFull := false
		now := t.shard.GetTimeSource().Now()
		for _, taskInfo := range transferTaskInfos {
			if !domainFilter.Filter(taskInfo.GetDomainID()) {
				t.logger.Debug("transfer task filtered", tag.TaskID(taskInfo.GetTaskID()))
				continue
			}

			if persistence.IsTaskCorrupted(taskInfo) {
				t.logger.Error("Processing queue encountered a corrupted task", tag.Dynamic("task", taskInfo))
				t.metricsScope.IncCounter(metrics.CorruptedHistoryTaskCounter)
				continue
			}

			task := t.taskInitializer(taskInfo)
			tasks[newTransferTaskKey(taskInfo.GetTaskID())] = task
			t.metricsScope.RecordHistogramDuration(metrics.TaskEnqueueToFetchLatency, now.Sub(taskInfo.GetVisibilityTimestamp()))
			submitted, err := t.submitTask(task)
			if err != nil {
				// only err here is due to the fact that processor has been shutdown
				// return instead of continue
				return
			}
			taskChFull = taskChFull || !submitted
		}

		var newReadLevel task.Key
		if !more {
			newReadLevel = maxReadLevel
		} else {
			newReadLevel = newTransferTaskKey(transferTaskInfos[len(transferTaskInfos)-1].GetTaskID())
		}
		queueCollection.AddTasks(tasks, newReadLevel)
		if t.validator != nil {
			t.logger.Debug("ack transfer tasks",
				tag.ReadLevel(readLevel.(transferTaskKey).taskID),
				tag.MaxLevel(newReadLevel.(transferTaskKey).taskID),
				tag.Counter(len(tasks)))
			t.validator.ackTasks(level, readLevel, newReadLevel, tasks)
		}

		newActiveQueue := queueCollection.ActiveQueue()
		if more || (newActiveQueue != nil && newActiveQueue != activeQueue) {
			// more tasks for the current active queue or the active queue has changed
			if level != defaultProcessingQueueLevel && taskChFull {
				t.setupBackoffTimer(level)
			} else {
				t.readyForProcess(level)
			}
		}

		// else it means we don't have tasks to process for now
		// wait for new task notification
		// another option for non-default queue is that we can setup a backoff timer to check back later
	}
}

func (t *transferQueueProcessorBase) splitQueue() {
	currentTime := t.shard.GetTimeSource().Now()
	currentMaxReadLevel := t.updateMaxReadLevel().(transferTaskKey).taskID
	defer func() {
		t.lastSplitTime = currentTime
		t.lastMaxReadLevel = currentMaxReadLevel
	}()

	if currentMaxReadLevel-t.lastMaxReadLevel < 2<<(t.shard.GetConfig().RangeSizeBits-1) {
		// only update the estimation when rangeID is not renewed
		// note the threshold here is only an estimation. If the read level increased too much
		// we will drop that data point.
		numTasksPerMinute := (currentMaxReadLevel - t.lastMaxReadLevel) / int64(currentTime.Sub(t.lastSplitTime).Seconds()) * int64(time.Minute.Seconds())

		if t.estimatedTasksPerMinute == 0 {
			// set the initial value for the estimation
			t.estimatedTasksPerMinute = numTasksPerMinute
		} else {
			t.estimatedTasksPerMinute = int64(numTasksEstimationDecay*float64(t.estimatedTasksPerMinute) + (1-numTasksEstimationDecay)*float64(numTasksPerMinute))
		}
	}

	if t.lastSplitTime.IsZero() || t.estimatedTasksPerMinute == 0 {
		// skip the split as we can't estimate the look ahead taskID
		return
	}

	splitPolicy := t.initializeSplitPolicy(
		func(key task.Key, domainID string) task.Key {
			totalLookAhead := t.estimatedTasksPerMinute * int64(t.options.SplitLookAheadDurationByDomainID(domainID).Minutes())
			// ensure the above calculation doesn't overflow and cap the maximun look ahead interval
			totalLookAhead = max(min(totalLookAhead, 2<<t.shard.GetConfig().RangeSizeBits), 0)
			return newTransferTaskKey(key.(transferTaskKey).taskID + totalLookAhead)
		},
	)

	t.splitProcessingQueueCollection(splitPolicy, func(level int, _ time.Time) {
		t.readyForProcess(level)
	})
}

func (t *transferQueueProcessorBase) handleActionNotification(notification actionNotification) {
	t.processorBase.handleActionNotification(notification, func() {
		switch notification.action.ActionType {
		case ActionTypeReset:
			t.readyForProcess(defaultProcessingQueueLevel)
		}
	})
}

func (t *transferQueueProcessorBase) readTasks(
	readLevel task.Key,
	maxReadLevel task.Key,
) ([]persistence.Task, bool, error) {

	var response *persistence.GetHistoryTasksResponse
	op := func(ctx context.Context) error {
		var err error
		response, err = t.shard.GetExecutionManager().GetHistoryTasks(ctx, &persistence.GetHistoryTasksRequest{
			TaskCategory:        persistence.HistoryTaskCategoryTransfer,
			InclusiveMinTaskKey: persistence.NewImmediateTaskKey(readLevel.(transferTaskKey).taskID + 1),
			ExclusiveMaxTaskKey: persistence.NewImmediateTaskKey(maxReadLevel.(transferTaskKey).taskID + 1),
			PageSize:            t.options.BatchSize(),
		})
		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(persistenceOperationRetryPolicy),
		backoff.WithRetryableError(persistence.IsBackgroundTransientError),
	)
	err := throttleRetry.Do(context.Background(), op)
	if err != nil {
		return nil, false, err
	}

	return response.Tasks, len(response.NextPageToken) != 0, nil
}

func newTransferTaskKey(taskID int64) task.Key {
	return transferTaskKey{
		taskID: taskID,
	}
}

func (k transferTaskKey) Less(
	key task.Key,
) bool {
	return k.taskID < key.(transferTaskKey).taskID
}

func newTransferQueueProcessorOptions(
	config *config.Config,
	isActive bool,
	isFailover bool,
) *queueProcessorOptions {
	options := &queueProcessorOptions{
		BatchSize:                            config.TransferTaskBatchSize,
		DeleteBatchSize:                      config.TransferTaskDeleteBatchSize,
		MaxPollRPS:                           config.TransferProcessorMaxPollRPS,
		MaxPollInterval:                      config.TransferProcessorMaxPollInterval,
		MaxPollIntervalJitterCoefficient:     config.TransferProcessorMaxPollIntervalJitterCoefficient,
		UpdateAckInterval:                    config.TransferProcessorUpdateAckInterval,
		UpdateAckIntervalJitterCoefficient:   config.TransferProcessorUpdateAckIntervalJitterCoefficient,
		MaxRedispatchQueueSize:               config.TransferProcessorMaxRedispatchQueueSize,
		SplitQueueInterval:                   config.TransferProcessorSplitQueueInterval,
		SplitQueueIntervalJitterCoefficient:  config.TransferProcessorSplitQueueIntervalJitterCoefficient,
		PollBackoffInterval:                  config.QueueProcessorPollBackoffInterval,
		PollBackoffIntervalJitterCoefficient: config.QueueProcessorPollBackoffIntervalJitterCoefficient,
		EnableValidator:                      config.TransferProcessorEnableValidator,
		ValidationInterval:                   config.TransferProcessorValidationInterval,
		EnableGracefulSyncShutdown:           config.QueueProcessorEnableGracefulSyncShutdown,
	}

	if isFailover {
		// disable queue split for failover processor
		options.EnableSplit = dynamicproperties.GetBoolPropertyFn(false)

		// disable persist and load processing queue states for failover processor as it will never be split
		options.EnablePersistQueueStates = dynamicproperties.GetBoolPropertyFn(false)
		options.EnableLoadQueueStates = dynamicproperties.GetBoolPropertyFn(false)

		options.MaxStartJitterInterval = config.TransferProcessorFailoverMaxStartJitterInterval
	} else {
		options.EnableSplit = config.QueueProcessorEnableSplit
		options.SplitMaxLevel = config.QueueProcessorSplitMaxLevel
		options.EnableRandomSplitByDomainID = config.QueueProcessorEnableRandomSplitByDomainID
		options.RandomSplitProbability = config.QueueProcessorRandomSplitProbability
		options.EnablePendingTaskSplitByDomainID = config.QueueProcessorEnablePendingTaskSplitByDomainID
		options.PendingTaskSplitThreshold = config.QueueProcessorPendingTaskSplitThreshold
		options.EnableStuckTaskSplitByDomainID = config.QueueProcessorEnableStuckTaskSplitByDomainID
		options.StuckTaskSplitThreshold = config.QueueProcessorStuckTaskSplitThreshold
		options.SplitLookAheadDurationByDomainID = config.QueueProcessorSplitLookAheadDurationByDomainID

		options.EnablePersistQueueStates = config.QueueProcessorEnablePersistQueueStates
		options.EnableLoadQueueStates = config.QueueProcessorEnableLoadQueueStates

		options.MaxStartJitterInterval = dynamicproperties.GetDurationPropertyFn(0)
	}

	if isActive {
		options.MetricScope = metrics.TransferActiveQueueProcessorScope
		options.RedispatchInterval = config.ActiveTaskRedispatchInterval
	} else {
		options.MetricScope = metrics.TransferStandbyQueueProcessorScope
		options.RedispatchInterval = config.StandbyTaskRedispatchInterval
	}

	return options
}
