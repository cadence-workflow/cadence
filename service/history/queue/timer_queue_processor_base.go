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
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
	"github.com/uber/cadence/service/history/task"
)

var (
	maximumTimerTaskKey = newTimerTaskKey(
		time.Unix(0, math.MaxInt64),
		0,
	)
)

type (
	timerTaskKey struct {
		visibilityTimestamp time.Time
		taskID              int64
	}

	timeTaskReadProgress struct {
		currentQueue  ProcessingQueue
		readLevel     task.Key
		maxReadLevel  task.Key
		nextPageToken []byte
	}

	timerQueueProcessorBase struct {
		*processorBase

		taskInitializer task.Initializer
		clusterName     string
		pollTimeLock    sync.Mutex
		backoffTimer    map[int]*time.Timer
		nextPollTime    map[int]time.Time
		timerGate       clock.TimerGate

		// timer notification
		newTimerCh  chan struct{}
		newTimeLock sync.Mutex
		newTime     time.Time

		processingQueueReadProgress map[int]timeTaskReadProgress

		updateAckLevelFn                 func() (bool, task.Key, error)
		splitProcessingQueueCollectionFn func(splitPolicy ProcessingQueueSplitPolicy, upsertPollTimeFn func(int, time.Time))
	}

	filteredTimerTasksResponse struct {
		timerTasks         []persistence.Task
		lookAheadTimestamp time.Time
		nextPageToken      []byte
	}
)

func newTimerQueueProcessorBase(
	clusterName string,
	shard shard.Context,
	processingQueueStates []ProcessingQueueState,
	taskProcessor task.Processor,
	timerGate clock.TimerGate,
	options *queueProcessorOptions,
	updateMaxReadLevel updateMaxReadLevelFn,
	updateClusterAckLevel updateClusterAckLevelFn,
	updateProcessingQueueStates updateProcessingQueueStatesFn,
	queueShutdown queueShutdownFn,
	taskFilter task.Filter,
	taskExecutor task.Executor,
	logger log.Logger,
	metricsClient metrics.Client,
) *timerQueueProcessorBase {
	processorBase := newProcessorBase(
		shard,
		processingQueueStates,
		taskProcessor,
		options,
		updateMaxReadLevel,
		updateClusterAckLevel,
		updateProcessingQueueStates,
		queueShutdown,
		logger.WithTags(tag.ComponentTimerQueue),
		metricsClient,
	)

	queueType := task.QueueTypeActiveTimer
	if options.MetricScope == metrics.TimerStandbyQueueProcessorScope {
		queueType = task.QueueTypeStandbyTimer
	}

	t := &timerQueueProcessorBase{
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
		clusterName:                 clusterName,
		backoffTimer:                make(map[int]*time.Timer),
		nextPollTime:                make(map[int]time.Time),
		timerGate:                   timerGate,
		newTimerCh:                  make(chan struct{}, 1),
		processingQueueReadProgress: make(map[int]timeTaskReadProgress),
	}

	t.updateAckLevelFn = t.updateAckLevel
	t.splitProcessingQueueCollectionFn = t.splitProcessingQueueCollection
	return t
}

func (t *timerQueueProcessorBase) Start() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}

	t.logger.Info("Timer queue processor state changed", tag.LifeCycleStarting)
	defer t.logger.Info("Timer queue processor state changed", tag.LifeCycleStarted)

	t.redispatcher.Start()

	newPollTime := time.Time{}
	if startJitter := t.options.MaxStartJitterInterval(); startJitter > 0 {
		now := t.shard.GetTimeSource().Now()
		newPollTime = now.Add(time.Duration(rand.Int63n(int64(startJitter))))
	}
	for _, queueCollections := range t.processingQueueCollections {
		t.upsertPollTime(queueCollections.Level(), newPollTime)
	}

	t.shutdownWG.Add(1)
	go t.processorPump()
}

// Edge Case: Stop doesn't stop TimerGate if timerQueueProcessorBase is only initiliazed without starting
// As a result, TimerGate needs to be stopped separately
// One way to fix this is to make sure TimerGate doesn't start daemon loop on initilization and requires explicit Start
func (t *timerQueueProcessorBase) Stop() {
	if !atomic.CompareAndSwapInt32(&t.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}

	t.logger.Info("Timer queue processor state changed", tag.LifeCycleStopping)

	t.timerGate.Stop()
	close(t.shutdownCh)
	t.pollTimeLock.Lock()
	for _, timer := range t.backoffTimer {
		timer.Stop()
	}
	t.pollTimeLock.Unlock()

	if success := common.AwaitWaitGroup(&t.shutdownWG, gracefulShutdownTimeout); !success {
		t.logger.Warn("timerQueueProcessorBase timed out on shut down", tag.LifeCycleStopTimedout)
	}

	t.redispatcher.Stop()
	t.logger.Info("Timer queue processor state changed", tag.LifeCycleStopped)
}

func (t *timerQueueProcessorBase) processorPump() {
	defer t.shutdownWG.Done()
	updateAckTimer := time.NewTimer(backoff.JitDuration(t.options.UpdateAckInterval(), t.options.UpdateAckIntervalJitterCoefficient()))
	defer updateAckTimer.Stop()
	splitQueueTimer := time.NewTimer(backoff.JitDuration(t.options.SplitQueueInterval(), t.options.SplitQueueIntervalJitterCoefficient()))
	defer splitQueueTimer.Stop()

	for {
		select {
		case <-t.shutdownCh:
			return
		case <-t.timerGate.Chan():
			t.updateTimerGates()
		case <-updateAckTimer.C:
			if stopPump := t.handleAckLevelUpdate(updateAckTimer); stopPump {
				if !t.options.EnableGracefulSyncShutdown() {
					go t.Stop()
					return
				}

				t.Stop()
				return
			}
		case <-t.newTimerCh:
			t.handleNewTimer()
		case <-splitQueueTimer.C:
			t.splitQueue(splitQueueTimer)
		case notification := <-t.actionNotifyCh:
			t.handleActionNotification(notification)
		}
	}
}

func (t *timerQueueProcessorBase) processQueueCollections(levels map[int]struct{}) {
	for _, queueCollection := range t.processingQueueCollections {
		level := queueCollection.Level()
		if _, ok := levels[level]; !ok {
			continue
		}

		activeQueue := queueCollection.ActiveQueue()
		if activeQueue == nil {
			// process for this queue collection has finished
			// it's possible that new queue will be added to this collection later though,
			// pollTime will be updated after split/merge
			t.logger.Debug("Active queue is nil for timer queue at this level", tag.QueueLevel(level))
			continue
		}

		t.upsertPollTime(level, t.shard.GetCurrentTime(t.clusterName).Add(backoff.JitDuration(
			t.options.MaxPollInterval(),
			t.options.MaxPollIntervalJitterCoefficient(),
		)))

		var nextPageToken []byte
		readLevel := activeQueue.State().ReadLevel()
		maxReadLevel := minTaskKey(activeQueue.State().MaxLevel(), t.updateMaxReadLevel())
		domainFilter := activeQueue.State().DomainFilter()

		if progress, ok := t.processingQueueReadProgress[level]; ok {
			if progress.currentQueue == activeQueue {
				readLevel = progress.readLevel
				maxReadLevel = progress.maxReadLevel
				nextPageToken = progress.nextPageToken
			}
			delete(t.processingQueueReadProgress, level)
		}

		if !readLevel.Less(maxReadLevel) {
			// notify timer gate about the min time
			t.upsertPollTime(level, readLevel.(timerTaskKey).visibilityTimestamp)
			t.logger.Debug("Skipping processing timer queue at this level because readLevel >= maxReadLevel", tag.QueueLevel(level))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), loadQueueTaskThrottleRetryDelay)
		if err := t.rateLimiter.Wait(ctx); err != nil {
			cancel()
			if level == defaultProcessingQueueLevel {
				t.upsertPollTime(level, time.Time{})
			} else {
				t.setupBackoffTimer(level)
			}
			continue
		}
		cancel()

		resp, err := t.readAndFilterTasks(readLevel, maxReadLevel, nextPageToken)
		if err != nil {
			t.logger.Error("Processor unable to retrieve tasks", tag.Error(err))
			t.upsertPollTime(level, time.Time{}) // re-enqueue the event
			continue
		}

		tasks := make(map[task.Key]task.Task)
		taskChFull := false
		submittedCount := 0
		for _, taskInfo := range resp.timerTasks {
			if !domainFilter.Filter(taskInfo.GetDomainID()) {
				continue
			}

			task := t.taskInitializer(taskInfo)
			tasks[newTimerTaskKey(taskInfo.GetVisibilityTimestamp(), taskInfo.GetTaskID())] = task
			submitted, err := t.submitTask(task)
			if err != nil {
				// only err here is due to the fact that processor has been shutdown
				// return instead of continue
				return
			}
			taskChFull = taskChFull || !submitted
			if submitted {
				submittedCount++
			}
		}
		t.logger.Debugf("Submitted %d timer tasks successfully out of %d tasks", submittedCount, len(resp.timerTasks))

		var newReadLevel task.Key
		if len(resp.nextPageToken) == 0 {
			newReadLevel = maxReadLevel
			if !resp.lookAheadTimestamp.IsZero() {
				// lookAheadTask may exist only when nextPageToken is empty
				// notice that lookAheadTask.VisibilityTimestamp may be larger than shard max read level,
				// which means new tasks can be generated before that timestamp. This issue is solved by
				// upsertPollTime whenever there are new tasks
				lookAheadTimestamp := resp.lookAheadTimestamp
				t.upsertPollTime(level, lookAheadTimestamp)
				newReadLevel = minTaskKey(newReadLevel, newTimerTaskKey(lookAheadTimestamp, 0))
				t.logger.Debugf("nextPageToken is empty for timer queue at level %d so setting newReadLevel to max(lookAheadTask.timestamp: %v, maxReadLevel: %v)", level, lookAheadTimestamp, maxReadLevel)
			} else {
				// else we have no idea when the next poll should happen
				// rely on notifyNewTask to trigger the next poll even for non-default queue.
				// another option for non-default queue is that we can setup a backoff timer to check back later
				t.logger.Debugf("nextPageToken is empty for timer queue at level %d and there' no lookAheadTask. setting readLevel to maxReadLevel: %v", level, maxReadLevel)
			}
		} else {
			// more tasks should be loaded for this processing queue
			// record the current progress and update the poll time
			if level == defaultProcessingQueueLevel || !taskChFull {
				t.logger.Debugf("upserting poll time for timer queue at level %d because nextPageToken is not empty and !taskChFull", level)
				t.upsertPollTime(level, time.Time{})
			} else {
				t.logger.Debugf("setting up backoff timer for timer queue at level %d because nextPageToken is not empty and taskChFull", level)
				t.setupBackoffTimer(level)
			}
			t.processingQueueReadProgress[level] = timeTaskReadProgress{
				currentQueue:  activeQueue,
				readLevel:     readLevel,
				maxReadLevel:  maxReadLevel,
				nextPageToken: resp.nextPageToken,
			}
			if len(resp.timerTasks) > 0 {
				newReadLevel = newTimerTaskKey(resp.timerTasks[len(resp.timerTasks)-1].GetVisibilityTimestamp(), 0)
			}
		}
		queueCollection.AddTasks(tasks, newReadLevel)
	}
}

// splitQueue splits the processing queue collection based on some policy
// and resets the timer with jitter for next run
func (t *timerQueueProcessorBase) splitQueue(splitQueueTimer *time.Timer) {
	splitPolicy := t.initializeSplitPolicy(
		func(key task.Key, domainID string) task.Key {
			return newTimerTaskKey(
				key.(timerTaskKey).visibilityTimestamp.Add(
					t.options.SplitLookAheadDurationByDomainID(domainID),
				),
				0,
			)
		},
	)

	t.splitProcessingQueueCollectionFn(splitPolicy, t.upsertPollTime)

	splitQueueTimer.Reset(backoff.JitDuration(
		t.options.SplitQueueInterval(),
		t.options.SplitQueueIntervalJitterCoefficient(),
	))
}

// handleAckLevelUpdate updates ack level and resets timer with jitter.
// returns true if processing should be terminated
func (t *timerQueueProcessorBase) handleAckLevelUpdate(updateAckTimer *time.Timer) bool {
	processFinished, _, err := t.updateAckLevelFn()
	var errShardClosed *shard.ErrShardClosed
	if errors.As(err, &errShardClosed) || (err == nil && processFinished) {
		return true
	}
	updateAckTimer.Reset(backoff.JitDuration(
		t.options.UpdateAckInterval(),
		t.options.UpdateAckIntervalJitterCoefficient(),
	))
	return false
}

func (t *timerQueueProcessorBase) updateTimerGates() {
	maxRedispatchQueueSize := t.options.MaxRedispatchQueueSize()
	if t.redispatcher.Size() > maxRedispatchQueueSize {
		t.redispatcher.Redispatch(maxRedispatchQueueSize)
		if t.redispatcher.Size() > maxRedispatchQueueSize {
			// if redispatcher still has a large number of tasks
			// this only happens when system is under very high load
			// we should backoff here instead of keeping submitting tasks to task processor
			// don't call t.timerGate.Update(time.Now() + loadQueueTaskThrottleRetryDelay) as the time in
			// standby timer processor is not real time and is managed separately
			time.Sleep(backoff.JitDuration(
				t.options.PollBackoffInterval(),
				t.options.PollBackoffIntervalJitterCoefficient(),
			))
		}
		t.timerGate.Update(time.Time{})
		return
	}

	t.pollTimeLock.Lock()
	levels := make(map[int]struct{})
	now := t.shard.GetCurrentTime(t.clusterName)
	for level, pollTime := range t.nextPollTime {
		if !now.Before(pollTime) {
			levels[level] = struct{}{}
			delete(t.nextPollTime, level)
		} else {
			t.timerGate.Update(pollTime)
		}
	}
	t.pollTimeLock.Unlock()

	t.processQueueCollections(levels)
}

func (t *timerQueueProcessorBase) handleNewTimer() {
	t.newTimeLock.Lock()
	newTime := t.newTime
	t.newTime = time.Time{}
	t.newTimeLock.Unlock()

	// New Timer has arrived.
	t.metricsScope.IncCounter(metrics.NewTimerNotifyCounter)
	// notify all queue collections as they are waiting for the notification when there's
	// no more task to process. For non-default queue, we choose to do periodic polling
	// in the future, then we don't need to notify them.
	for _, queueCollection := range t.processingQueueCollections {
		t.upsertPollTime(queueCollection.Level(), newTime)
	}
}

func (t *timerQueueProcessorBase) handleActionNotification(notification actionNotification) {
	t.processorBase.handleActionNotification(notification, func() {
		switch notification.action.ActionType {
		case ActionTypeReset:
			t.upsertPollTime(defaultProcessingQueueLevel, time.Time{})
		}
	})
}

func (t *timerQueueProcessorBase) readAndFilterTasks(readLevel, maxReadLevel task.Key, nextPageToken []byte) (*filteredTimerTasksResponse, error) {
	resp, err := t.getTimerTasks(readLevel, maxReadLevel, nextPageToken, t.options.BatchSize())
	if err != nil {
		return nil, err
	}

	var lookAheadTimestamp time.Time
	filteredTasks := []persistence.Task{}
	for _, timerTask := range resp.Tasks {
		if !t.isProcessNow(timerTask.GetVisibilityTimestamp()) {
			// found the first task that is not ready to be processed yet.
			// reset NextPageToken so we can load more tasks starting from this lookAheadTask next time.
			lookAheadTimestamp = timerTask.GetVisibilityTimestamp()
			resp.NextPageToken = nil
			break
		}
		filteredTasks = append(filteredTasks, timerTask)
	}

	if len(resp.NextPageToken) == 0 && lookAheadTimestamp.IsZero() {
		// only look ahead within the processing queue boundary
		lookAheadTask, err := t.readLookAheadTask(maxReadLevel, maximumTimerTaskKey)
		if err != nil {
			// we don't know if look ahead task exists or not, but we know if it exists,
			// it's visibility timestamp is larger than or equal to maxReadLevel.
			// so, create a fake look ahead task so another load can be triggered at that time.
			lookAheadTimestamp = maxReadLevel.(timerTaskKey).visibilityTimestamp
		} else if lookAheadTask != nil {
			lookAheadTimestamp = lookAheadTask.GetVisibilityTimestamp()
		}
	}

	t.logger.Debugf("readAndFilterTasks returning %d tasks and lookAheadTimestamp: %#v for readLevel: %#v, maxReadLevel: %#v", len(filteredTasks), lookAheadTimestamp, readLevel, maxReadLevel)
	return &filteredTimerTasksResponse{
		timerTasks:         filteredTasks,
		lookAheadTimestamp: lookAheadTimestamp,
		nextPageToken:      resp.NextPageToken,
	}, nil
}

func (t *timerQueueProcessorBase) readLookAheadTask(lookAheadStartLevel task.Key, lookAheadMaxLevel task.Key) (persistence.Task, error) {
	resp, err := t.getTimerTasks(lookAheadStartLevel, lookAheadMaxLevel, nil, 1)
	if err != nil {
		return nil, err
	}

	if len(resp.Tasks) == 1 {
		return resp.Tasks[0], nil
	}
	return nil, nil
}

func (t *timerQueueProcessorBase) getTimerTasks(readLevel, maxReadLevel task.Key, nextPageToken []byte, batchSize int) (*persistence.GetHistoryTasksResponse, error) {
	request := &persistence.GetHistoryTasksRequest{
		TaskCategory: persistence.HistoryTaskCategoryTimer,
		InclusiveMinTaskKey: persistence.HistoryTaskKey{
			ScheduledTime: readLevel.(timerTaskKey).visibilityTimestamp,
		},
		ExclusiveMaxTaskKey: persistence.HistoryTaskKey{
			ScheduledTime: maxReadLevel.(timerTaskKey).visibilityTimestamp,
		},
		PageSize:      batchSize,
		NextPageToken: nextPageToken,
	}
	var err error
	var response *persistence.GetHistoryTasksResponse
	retryCount := t.shard.GetConfig().TimerProcessorGetFailureRetryCount()
	for attempt := 0; attempt < retryCount; attempt++ {
		response, err = t.shard.GetExecutionManager().GetHistoryTasks(context.Background(), request)
		if err == nil {
			return response, nil
		}
		backoff := time.Duration(attempt*100) * time.Millisecond
		t.logger.Debugf("Failed to get timer tasks from execution manager. error: %v, attempt: %d, retryCount: %d, backoff: %v", err, attempt, retryCount, backoff)
		time.Sleep(backoff)
	}
	return nil, err
}

func (t *timerQueueProcessorBase) isProcessNow(expiryTime time.Time) bool {
	if expiryTime.IsZero() {
		// return true, but somewhere probably have bug creating empty timerTask.
		t.logger.Warn("Timer task has timestamp zero")
	}
	return !t.shard.GetCurrentTime(t.clusterName).Before(expiryTime)
}

func (t *timerQueueProcessorBase) notifyNewTimers(timerTasks []persistence.Task) {
	if len(timerTasks) == 0 {
		return
	}

	isActive := t.options.MetricScope == metrics.TimerActiveQueueProcessorScope
	minNewTime := timerTasks[0].GetVisibilityTimestamp()
	shardIDTag := metrics.ShardIDTag(t.shard.GetShardID())
	for _, timerTask := range timerTasks {
		ts := timerTask.GetVisibilityTimestamp()
		if ts.Before(minNewTime) {
			minNewTime = ts
		}

		taskScopeIdx := task.GetTimerTaskMetricScope(
			timerTask.GetTaskType(),
			isActive,
		)
		t.metricsClient.Scope(taskScopeIdx).Tagged(shardIDTag).IncCounter(metrics.NewTimerNotifyCounter)
	}

	t.notifyNewTimer(minNewTime)
}

func (t *timerQueueProcessorBase) notifyNewTimer(newTime time.Time) {
	t.newTimeLock.Lock()
	defer t.newTimeLock.Unlock()

	if t.newTime.IsZero() || newTime.Before(t.newTime) {
		t.logger.Debugf("Updating newTime from %v to %v", t.newTime, newTime)
		t.newTime = newTime
		select {
		case t.newTimerCh <- struct{}{}:
			// Notified about new time.
		default:
			// Channel "full" -> drop and move on, this will happen only if service is in high load.
		}
	}
}

func (t *timerQueueProcessorBase) upsertPollTime(level int, newPollTime time.Time) {
	t.pollTimeLock.Lock()
	defer t.pollTimeLock.Unlock()

	if _, ok := t.backoffTimer[level]; ok {
		// honor existing backoff timer
		t.logger.Debugf("Skipping upsertPollTime for timer queue at level %d because there's a backoff timer", level)
		return
	}

	currentPollTime, ok := t.nextPollTime[level]
	if !ok || newPollTime.Before(currentPollTime) {
		t.logger.Debugf("Updating poll timer for timer queue at level %d. CurrentPollTime: %v, newPollTime: %v", level, currentPollTime, newPollTime)
		t.nextPollTime[level] = newPollTime
		t.timerGate.Update(newPollTime)
		return
	}

	t.logger.Debugf("Skipping upsertPollTime for level %d because currentPollTime %v is before newPollTime %v", level, currentPollTime, newPollTime)
}

// setupBackoffTimer will trigger a poll for the specified processing queue collection
// after a certain period of (real) time. This means for standby timer, even if the cluster time
// has not been updated, the poll will still be triggered when the timer fired. Use this function
// for delaying the load for processing queue. If a poll should be triggered immediately
// use upsertPollTime.
func (t *timerQueueProcessorBase) setupBackoffTimer(level int) {
	t.pollTimeLock.Lock()
	defer t.pollTimeLock.Unlock()

	if _, ok := t.backoffTimer[level]; ok {
		// honor existing backoff timer
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

		t.pollTimeLock.Lock()
		defer t.pollTimeLock.Unlock()

		t.nextPollTime[level] = time.Time{}
		t.timerGate.Update(time.Time{})
		delete(t.backoffTimer, level)
	})
}

func newTimerTaskKey(visibilityTimestamp time.Time, taskID int64) task.Key {
	return timerTaskKey{
		visibilityTimestamp: visibilityTimestamp,
		taskID:              taskID,
	}
}

func (k timerTaskKey) Less(
	key task.Key,
) bool {
	timerKey := key.(timerTaskKey)
	if k.visibilityTimestamp.Equal(timerKey.visibilityTimestamp) {
		return k.taskID < timerKey.taskID
	}
	return k.visibilityTimestamp.Before(timerKey.visibilityTimestamp)
}

func (k timerTaskKey) String() string {
	return fmt.Sprintf("{visibilityTimestamp: %v, taskID: %v}", k.visibilityTimestamp, k.taskID)
}

func newTimerQueueProcessorOptions(
	config *config.Config,
	isActive bool,
	isFailover bool,
) *queueProcessorOptions {
	options := &queueProcessorOptions{
		BatchSize:                            config.TimerTaskBatchSize,
		DeleteBatchSize:                      config.TimerTaskDeleteBatchSize,
		MaxPollRPS:                           config.TimerProcessorMaxPollRPS,
		MaxPollInterval:                      config.TimerProcessorMaxPollInterval,
		MaxPollIntervalJitterCoefficient:     config.TimerProcessorMaxPollIntervalJitterCoefficient,
		UpdateAckInterval:                    config.TimerProcessorUpdateAckInterval,
		UpdateAckIntervalJitterCoefficient:   config.TimerProcessorUpdateAckIntervalJitterCoefficient,
		MaxRedispatchQueueSize:               config.TimerProcessorMaxRedispatchQueueSize,
		SplitQueueInterval:                   config.TimerProcessorSplitQueueInterval,
		SplitQueueIntervalJitterCoefficient:  config.TimerProcessorSplitQueueIntervalJitterCoefficient,
		PollBackoffInterval:                  config.QueueProcessorPollBackoffInterval,
		PollBackoffIntervalJitterCoefficient: config.QueueProcessorPollBackoffIntervalJitterCoefficient,
		EnableGracefulSyncShutdown:           config.QueueProcessorEnableGracefulSyncShutdown,
	}

	if isFailover {
		// disable queue split for failover processor
		options.EnableSplit = dynamicproperties.GetBoolPropertyFn(false)

		// disable persist and load processing queue states for failover processor as it will never be split
		options.EnablePersistQueueStates = dynamicproperties.GetBoolPropertyFn(false)
		options.EnableLoadQueueStates = dynamicproperties.GetBoolPropertyFn(false)

		options.MaxStartJitterInterval = config.TimerProcessorFailoverMaxStartJitterInterval
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
		options.MetricScope = metrics.TimerActiveQueueProcessorScope
		options.RedispatchInterval = config.ActiveTaskRedispatchInterval
	} else {
		options.MetricScope = metrics.TimerStandbyQueueProcessorScope
		options.RedispatchInterval = config.StandbyTaskRedispatchInterval
	}

	return options
}
