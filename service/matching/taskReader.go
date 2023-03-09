// Copyright (c) 2017-2020 Uber Technologies Inc.

// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.

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

package matching

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var epochStartTime = time.Unix(0, 0)

type (
	taskReader struct {
		taskBuffer     chan *persistence.TaskInfo // tasks loaded from persistence
		notifyC        chan struct{}              // Used as signal to notify pump of new tasks
		tlMgr          *taskListManagerImpl
		taskListID     *taskListID
		config         *taskListConfig
		db             *taskListDB
		taskWriter     *taskWriter
		taskGC         *taskGC
		taskAckManager messaging.AckManager
		// The cancel objects are to cancel the ratelimiter Wait in dispatchBufferedTasks. The ideal
		// approach is to use request-scoped contexts and use a unique one for each call to Wait. However
		// in order to cancel it on shutdown, we need a new goroutine for each call that would wait on
		// the shutdown channel. To optimize on efficiency, we instead create one and tag it on the struct
		// so the cancel can be called directly on shutdown.
		cancelCtx     context.Context
		cancelFunc    context.CancelFunc
		stopped       int64 // set to 1 if the reader is stopped or is shutting down
		logger        log.Logger
		scope         metrics.Scope
		throttleRetry *backoff.ThrottleRetry
		handleErr     func(error) error
		onFatalErr    func()
		dispatchTask  func(context.Context, *InternalTask) error
	}
)

func newTaskReader(tlMgr *taskListManagerImpl) *taskReader {
	ctx, cancel := context.WithCancel(context.Background())
	return &taskReader{
		tlMgr:          tlMgr,
		taskListID:     tlMgr.taskListID,
		config:         tlMgr.config,
		db:             tlMgr.db,
		taskWriter:     tlMgr.taskWriter,
		taskGC:         tlMgr.taskGC,
		taskAckManager: tlMgr.taskAckManager,
		cancelCtx:      ctx,
		cancelFunc:     cancel,
		notifyC:        make(chan struct{}, 1),
		// we always dequeue the head of the buffer and try to dispatch it to a poller
		// so allocate one less than desired target buffer size
		taskBuffer:   make(chan *persistence.TaskInfo, tlMgr.config.GetTasksBatchSize()-1),
		logger:       tlMgr.logger,
		scope:        tlMgr.scope,
		handleErr:    tlMgr.handleErr,
		onFatalErr:   tlMgr.Stop,
		dispatchTask: tlMgr.DispatchTask,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(persistenceOperationRetryPolicy),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
	}
}

func (tr *taskReader) Start() {
	tr.Signal()
	go tr.dispatchBufferedTasks()
	go tr.getTasksPump()
}

func (tr *taskReader) Stop() {
	if atomic.CompareAndSwapInt64(&tr.stopped, 0, 1) {
		tr.cancelFunc()
		if err := tr.persistAckLevel(); err != nil {
			tr.logger.Error("Persistent store operation failure",
				tag.StoreOperationUpdateTaskList,
				tag.Error(err))
		}
		tr.taskGC.RunNow(tr.taskAckManager.GetAckLevel())
	}
}

func (tr *taskReader) Signal() {
	var event struct{}
	select {
	case tr.notifyC <- event:
	default: // channel already has an event, don't block
	}
}

func (tr *taskReader) dispatchBufferedTasks() {
dispatchLoop:
	for {
		select {
		case taskInfo, ok := <-tr.taskBuffer:
			if !ok { // Task list getTasks pump is shutdown
				break dispatchLoop
			}
			task := newInternalTask(taskInfo, tr.completeTask, types.TaskSourceDbBacklog, "", false, nil)
			for {
				err := tr.dispatchTask(tr.cancelCtx, task)
				if err == nil {
					break
				}
				if err == context.Canceled {
					tr.logger.Info("Tasklist manager context is cancelled, shutting down")
					break dispatchLoop
				}
				// this should never happen unless there is a bug - don't drop the task
				tr.scope.IncCounter(metrics.BufferThrottlePerTaskListCounter)
				tr.logger.Error("taskReader: unexpected error dispatching task", tag.Error(err))
				runtime.Gosched()
			}
		case <-tr.cancelCtx.Done():
			break dispatchLoop
		}
	}
}

func (tr *taskReader) getTasksPump() {
	defer close(tr.taskBuffer)

	updateAckTimer := time.NewTimer(tr.config.UpdateAckInterval())
	defer updateAckTimer.Stop()
getTasksPumpLoop:
	for {
		select {
		case <-tr.cancelCtx.Done():
			break getTasksPumpLoop
		case <-tr.notifyC:
			{
				tasks, readLevel, isReadBatchDone, err := tr.getTaskBatch()
				if err != nil {
					tr.Signal() // re-enqueue the event
					// TODO: Should we ever stop retrying on db errors?
					continue getTasksPumpLoop
				}

				if len(tasks) == 0 {
					tr.taskAckManager.SetReadLevel(readLevel)
					if !isReadBatchDone {
						tr.Signal()
					}
					continue getTasksPumpLoop
				}

				if !tr.addTasksToBuffer(tasks) {
					break getTasksPumpLoop
				}
				// There maybe more tasks. We yield now, but signal pump to check again later.
				tr.Signal()
			}
		case <-updateAckTimer.C:
			{
				if err := tr.handleErr(tr.persistAckLevel()); err != nil {
					tr.logger.Error("Persistent store operation failure",
						tag.StoreOperationUpdateTaskList,
						tag.Error(err))
					// keep going as saving ack is not critical
				}
				tr.Signal() // periodically signal pump to check persistence for tasks
				updateAckTimer = time.NewTimer(tr.config.UpdateAckInterval())
			}
		}
		scope := tr.scope.Tagged(getTaskListTypeTag(tr.taskListID.taskType))
		scope.UpdateGauge(metrics.TaskBacklogPerTaskListGauge, float64(tr.taskAckManager.GetBacklogCount()))
	}

}

func (tr *taskReader) getTaskBatchWithRange(readLevel int64, maxReadLevel int64) ([]*persistence.TaskInfo, error) {
	var response *persistence.GetTasksResponse
	op := func() (err error) {
		response, err = tr.db.GetTasks(readLevel, maxReadLevel, tr.config.GetTasksBatchSize())
		return
	}
	err := tr.throttleRetry.Do(context.Background(), op)
	if err != nil {
		tr.logger.Error("Persistent store operation failure",
			tag.StoreOperationGetTasks,
			tag.Error(err),
			tag.WorkflowTaskListName(tr.taskListID.name),
			tag.WorkflowTaskListType(tr.taskListID.taskType))
		return nil, err
	}
	return response.Tasks, nil
}

// Returns a batch of tasks from persistence starting form current read level.
// Also return a number that can be used to update readLevel
// Also return a bool to indicate whether read is finished
func (tr *taskReader) getTaskBatch() ([]*persistence.TaskInfo, int64, bool, error) {
	var tasks []*persistence.TaskInfo
	readLevel := tr.taskAckManager.GetReadLevel()
	maxReadLevel := tr.taskWriter.GetMaxReadLevel()

	// counter i is used to break and let caller check whether tasklist is still alive and need resume read.
	for i := 0; i < 10 && readLevel < maxReadLevel; i++ {
		upper := readLevel + tr.config.RangeSize
		if upper > maxReadLevel {
			upper = maxReadLevel
		}
		tasks, err := tr.getTaskBatchWithRange(readLevel, upper)
		if err != nil {
			return nil, readLevel, true, err
		}
		// return as long as it grabs any tasks
		if len(tasks) > 0 {
			return tasks, upper, true, nil
		}
		readLevel = upper
	}
	return tasks, readLevel, readLevel == maxReadLevel, nil // caller will update readLevel when no task grabbed
}

func (tr *taskReader) isTaskExpired(t *persistence.TaskInfo, now time.Time) bool {
	return t.Expiry.After(epochStartTime) && time.Now().After(t.Expiry)
}

func (tr *taskReader) addTasksToBuffer(tasks []*persistence.TaskInfo) bool {
	now := time.Now()
	for _, t := range tasks {
		if tr.isTaskExpired(t, now) {
			tr.scope.IncCounter(metrics.ExpiredTasksPerTaskListCounter)
			// Also increment readLevel for expired tasks otherwise it could result in
			// looping over the same tasks if all tasks read in the batch are expired
			tr.taskAckManager.SetReadLevel(t.TaskID)
			continue
		}
		if !tr.addSingleTaskToBuffer(t) {
			return false // we are shutting down the task list
		}
	}
	return true
}

func (tr *taskReader) addSingleTaskToBuffer(task *persistence.TaskInfo) bool {
	err := tr.taskAckManager.ReadItem(task.TaskID)
	if err != nil {
		tr.logger.Fatal("critical bug when adding item to ackManager", tag.Error(err))
	}
	for {
		select {
		case tr.taskBuffer <- task:
			return true
		case <-tr.cancelCtx.Done():
			return false
		}
	}
}

func (tr *taskReader) persistAckLevel() error {
	ackLevel := tr.taskAckManager.GetAckLevel()
	if ackLevel >= 0 {
		maxReadLevel := tr.taskWriter.GetMaxReadLevel()
		scope := tr.scope.Tagged(getTaskListTypeTag(tr.taskListID.taskType))
		// note: this metrics is only an estimation for the lag. taskID in DB may not be continuous,
		// especially when task list ownership changes.
		scope.UpdateGauge(metrics.TaskLagPerTaskListGauge, float64(maxReadLevel-ackLevel))

		return tr.db.UpdateState(ackLevel)
	}
	return nil
}

// completeTask marks a task as processed. Only tasks created by taskReader (i.e. backlog from db) reach
// here. As part of completion:
//   - task is deleted from the database when err is nil
//   - new task is created and current task is deleted when err is not nil
func (tr *taskReader) completeTask(task *persistence.TaskInfo, err error) {
	if err != nil {
		// failed to start the task.
		// We cannot just remove it from persistence because then it will be lost.
		// We handle this by writing the task back to persistence with a higher taskID.
		// This will allow subsequent tasks to make progress, and hopefully by the time this task is picked-up
		// again the underlying reason for failing to start will be resolved.
		// Note that RecordTaskStarted only fails after retrying for a long time, so a single task will not be
		// re-written to persistence frequently.
		op := func() error {
			wf := &types.WorkflowExecution{WorkflowID: task.WorkflowID, RunID: task.RunID}
			_, err := tr.taskWriter.appendTask(wf, task)
			return err
		}
		err = tr.throttleRetry.Do(context.Background(), op)
		if err != nil {
			// OK, we also failed to write to persistence.
			// This should only happen in very extreme cases where persistence is completely down.
			// We still can't lose the old task so we just unload the entire task list
			tr.logger.Error("Failed to complete task", tag.Error(err))
			tr.onFatalErr()
			return
		}
		tr.Signal()
	}
	ackLevel := tr.taskAckManager.AckItem(task.TaskID)
	tr.taskGC.Run(ackLevel)
}
