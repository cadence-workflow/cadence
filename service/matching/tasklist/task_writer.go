// Copyright (c) 2020 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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

package tasklist

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/event"
)

type (
	writeTaskResponse struct {
		err                 error
		persistenceResponse *persistence.CreateTasksResponse
	}

	writeTaskRequest struct {
		taskInfo   *persistence.TaskInfo
		responseCh chan<- *writeTaskResponse
	}

	taskIDBlock struct {
		start int64
		end   int64
	}

	// taskWriter writes tasks sequentially to persistence
	taskWriter struct {
		db             *taskListDB
		config         *config.TaskListConfig
		taskListID     *Identifier
		taskAckManager messaging.AckManager
		appendCh       chan *writeTaskRequest
		taskIDBlock    taskIDBlock
		maxReadLevel   int64
		stopped        int64 // set to 1 if the writer is stopped or is shutting down
		logger         log.Logger
		scope          metrics.Scope
		stopCh         chan struct{} // shutdown signal for all routines in this class
		throttleRetry  *backoff.ThrottleRetry
		handleErr      func(error) error
		onFatalErr     func()
	}
)

// errShutdown indicates that the task list is shutting down
var errShutdown = errors.New("task list shutting down")

func newTaskWriter(tlMgr *taskListManagerImpl) *taskWriter {
	return &taskWriter{
		db:             tlMgr.db,
		config:         tlMgr.config,
		taskListID:     tlMgr.taskListID,
		taskAckManager: tlMgr.taskAckManager,
		stopCh:         make(chan struct{}),
		appendCh:       make(chan *writeTaskRequest, tlMgr.config.OutstandingTaskAppendsThreshold()),
		logger:         tlMgr.logger,
		scope:          tlMgr.scope,
		handleErr:      tlMgr.handleErr,
		onFatalErr:     tlMgr.Stop,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(persistenceOperationRetryPolicy),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
	}
}

func (w *taskWriter) Start() error {
	// Make sure to grab the range first before starting task writer, as it needs the range to initialize maxReadLevel
	state, err := w.renewLeaseWithRetry()
	if err != nil {
		return err
	}

	w.taskAckManager.SetAckLevel(state.ackLevel)
	block := rangeIDToTaskIDBlock(state.rangeID, w.config.RangeSize)
	w.taskIDBlock = block
	w.maxReadLevel = block.start - 1
	go w.taskWriterLoop()
	return nil
}

// Stop stops the taskWriter
func (w *taskWriter) Stop() {
	if atomic.CompareAndSwapInt64(&w.stopped, 0, 1) {
		close(w.stopCh)
	}
}

func (w *taskWriter) isStopped() bool {
	return atomic.LoadInt64(&w.stopped) == 1
}

func (w *taskWriter) appendTask(ctx context.Context, taskInfo *persistence.TaskInfo) (*persistence.CreateTasksResponse, error) {
	// If the writer has already stopped, it returns an error
	if w.isStopped() {
		return nil, errShutdown
	}

	// Create a result channel for this request.
	// Use buffer size 1 so writer can send one response even if this call already timed out.
	// Without a buffer, writer can block forever when no receiver is waiting.
	ch := make(chan *writeTaskResponse, 1)

	req := &writeTaskRequest{
		taskInfo:   taskInfo,
		responseCh: ch,
	}

	select { // Try to append a task into the queue
	case w.appendCh <- req: // If enqueue succeed
		select { // Wait for the task result after the enqueue
		case r := <-ch: // If the writer sends the task result after the DB I/O task
			return r.persistenceResponse, r.err
		case <-ctx.Done(): // If the writer didn't send the task result within a timeout threshold
			return nil, ctx.Err()
		case <-w.stopCh:
			// if we are shutting down, this request will never make
			// it to cassandra, just bail out and fail this request
			return nil, errShutdown
		}
	case <-ctx.Done(): // If enqueue failed due to context timeout
		return nil, ctx.Err()
	default: // channel is full, throttle
		return nil, createServiceBusyError("Too many outstanding appends to the TaskList")
	}
}

func (w *taskWriter) GetMaxReadLevel() int64 {
	return atomic.LoadInt64(&w.maxReadLevel)
}

func (w *taskWriter) allocTaskIDs(count int) ([]int64, error) {
	result := make([]int64, count)
	for i := 0; i < count; i++ {
		if w.taskIDBlock.start > w.taskIDBlock.end {
			// we ran out of current allocation block
			newBlock, err := w.allocTaskIDBlock(w.taskIDBlock.end)
			if err != nil {
				return nil, err
			}
			w.taskIDBlock = newBlock
		}
		result[i] = w.taskIDBlock.start
		w.taskIDBlock.start++
	}
	return result, nil
}

func (w *taskWriter) allocTaskIDBlock(prevBlockEnd int64) (taskIDBlock, error) {
	currBlock := rangeIDToTaskIDBlock(w.db.RangeID(), w.config.RangeSize)
	if currBlock.end != prevBlockEnd {
		return taskIDBlock{},
			fmt.Errorf("allocTaskIDBlock: invalid state: prevBlockEnd:%v != currTaskIDBlock:%+v", prevBlockEnd, currBlock)
	}
	state, err := w.renewLeaseWithRetry()
	if err != nil {
		return taskIDBlock{}, err
	}
	return rangeIDToTaskIDBlock(state.rangeID, w.config.RangeSize), nil
}

func (w *taskWriter) renewLeaseWithRetry() (taskListState, error) {
	var newState taskListState
	op := func(ctx context.Context) (err error) {
		newState, err = w.db.RenewLease()
		return
	}
	w.scope.IncCounter(metrics.LeaseRequestPerTaskListCounter)
	err := w.throttleRetry.Do(context.Background(), op)
	if err != nil {
		w.scope.IncCounter(metrics.LeaseFailurePerTaskListCounter)
		w.onFatalErr()
		return newState, err
	}
	return newState, nil
}

func (w *taskWriter) taskWriterLoop() {
writerLoop:
	for {
		select {
		case request := <-w.appendCh:
			{
				if w.isStopped() {
					break writerLoop
				}

				// read a batch of requests from the channel
				reqs := []*writeTaskRequest{request}
				reqs = w.getWriteBatch(reqs)
				batchSize := len(reqs)

				maxReadLevel := int64(0)

				taskIDs, err := w.allocTaskIDs(batchSize)
				if err != nil {
					w.logger.Error("error allocating task ids",
						tag.Error(err),
					)
					w.sendWriteResponse(reqs, err, nil)
					continue writerLoop
				}

				tasks := []*persistence.CreateTaskInfo{}
				events := []event.E{}
				for i, req := range reqs {
					tasks = append(tasks, &persistence.CreateTaskInfo{
						TaskID: taskIDs[i],
						Data:   req.taskInfo,
					})
					kind := types.TaskListKind(w.db.taskListKind)
					events = append(events, event.E{
						TaskListName: w.db.taskListName,
						TaskListKind: &kind,
						TaskListType: w.db.taskType,
						TaskInfo:     *req.taskInfo,
						EventName:    "Task Written to DB",
					})
					maxReadLevel = taskIDs[i]
				}

				resp, err := w.db.CreateTasks(tasks)
				err = w.handleErr(err)
				if err != nil {
					w.logger.Error("Persistent store operation failure",
						tag.StoreOperationCreateTasks,
						tag.Error(err),
						tag.Number(taskIDs[0]),
						tag.NextNumber(taskIDs[batchSize-1]),
					)
				} else {
					event.Log(events...)
				}
				// Update the maxReadLevel after the writes are completed.
				if maxReadLevel > 0 {
					atomic.StoreInt64(&w.maxReadLevel, maxReadLevel)
				}

				w.sendWriteResponse(reqs, err, resp)
			}
		case <-w.stopCh:
			// we don't close the appendCh here
			// because that can cause "send on closed channel" panic on the appendTask()
			break writerLoop
		}
	}
}

func (w *taskWriter) getWriteBatch(reqs []*writeTaskRequest) []*writeTaskRequest {
readLoop:
	for i := 0; i < w.config.MaxTaskBatchSize(); i++ {
		select {
		case req := <-w.appendCh:
			reqs = append(reqs, req)
		default: // channel is empty, don't block
			break readLoop
		}
	}
	return reqs
}

func (w *taskWriter) sendWriteResponse(reqs []*writeTaskRequest,
	err error, persistenceResponse *persistence.CreateTasksResponse) {
	for _, req := range reqs {
		resp := &writeTaskResponse{
			err:                 err,
			persistenceResponse: persistenceResponse,
		}

		// appendTask() listens stopCh and terminates early without reading from responseCh.
		// Therefore we need to listen stopCh here to avoid getting stuck while pushing response to responseCh.
		select {
		case <-w.stopCh:
		case req.responseCh <- resp:
		}
	}
}
