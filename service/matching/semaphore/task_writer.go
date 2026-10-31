package semaphore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/persistence"
)

const (
	// taskIDRangeSize is how many task ids one range_id reserves. A large block keeps renewals rare.
	taskIDRangeSize = 100_000
	// taskAppendBufferSize is how many appends can wait for the writer before appendTask blocks.
	taskAppendBufferSize = 100
	// maxTaskWriteBatchSize is the most tasks the writer puts in one write.
	maxTaskWriteBatchSize = 100
)

type (
	writeTaskRequest struct {
		task *persistence.SemaphoreTask
		// responseCh has room for the answer, so the writer never waits on a caller that gave up.
		responseCh chan error
	}

	// taskIDBlock is a range of task ids, start to end inclusive.
	taskIDBlock struct {
		start int64
		end   int64
	}

	// taskWriter assigns task ids and writes waiters to semaphore_tasks from one goroutine.
	taskWriter struct {
		db             *taskDB
		taskAckManager messaging.AckManager
		logger         log.Logger
		rangeSize      int64

		throttleRetry *backoff.ThrottleRetry

		appendCh     chan *writeTaskRequest
		taskIDBlock  taskIDBlock
		maxReadLevel int64

		stopped int64         // set to 1 if the writer is stopped or is shutting down
		stopCh  chan struct{} // shutdown signal for the writer goroutine and pending appends

		// fatalCh is closed when this host has lost the bucket. The writer only reports it; the
		// manager stops the bucket.
		fatalCh   chan struct{}
		fatalOnce sync.Once
	}
)

// rangeIDToTaskIDBlock returns the block of task ids that rangeID reserves.
func rangeIDToTaskIDBlock(rangeID, rangeSize int64) taskIDBlock {
	return taskIDBlock{
		start: (rangeID-1)*rangeSize + 1,
		end:   rangeID * rangeSize,
	}
}

// newTaskWriter builds the writer from its manager's fields.
func newTaskWriter(m *semaphoreManagerImpl) *taskWriter {
	return &taskWriter{
		db:             m.taskDB,
		taskAckManager: m.taskAckManager,
		logger:         m.logger,
		rangeSize:      taskIDRangeSize,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(common.CreatePersistenceRetryPolicy()),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
		appendCh: make(chan *writeTaskRequest, taskAppendBufferSize),
		stopCh:   make(chan struct{}),
		fatalCh:  make(chan struct{}),
	}
}

// claim takes the bucket and sets the ack level, task id block and maxReadLevel from it. Call it
// once, before start.
func (w *taskWriter) claim() error {
	state, err := w.db.claim()
	if err != nil {
		return err
	}
	w.taskAckManager.SetAckLevel(state.ackLevel)
	w.taskIDBlock = rangeIDToTaskIDBlock(state.rangeID, w.rangeSize)
	atomic.StoreInt64(&w.maxReadLevel, w.taskIDBlock.start-1)
	return nil
}

// start runs the writer goroutine. Call claim first.
func (w *taskWriter) start() {
	go w.writeLoop()
}

// stop signals the writer to exit without waiting. Later appends get ErrNotReady.
func (w *taskWriter) stop() {
	if atomic.CompareAndSwapInt64(&w.stopped, 0, 1) {
		close(w.stopCh)
	}
}

func (w *taskWriter) isStopped() bool {
	return atomic.LoadInt64(&w.stopped) == 1
}

// fatal returns a channel that is closed once this host has lost the bucket.
func (w *taskWriter) fatal() <-chan struct{} {
	return w.fatalCh
}

func (w *taskWriter) signalFatal() {
	w.fatalOnce.Do(func() { close(w.fatalCh) })
}

// getMaxReadLevel returns the highest task id written so far. The reader reads only up to it.
func (w *taskWriter) getMaxReadLevel() int64 {
	return atomic.LoadInt64(&w.maxReadLevel)
}

// appendTask gives task the next task id and writes it, returning once the write has finished.
// task itself is not changed.
func (w *taskWriter) appendTask(ctx context.Context, task *persistence.SemaphoreTask) error {
	req := &writeTaskRequest{task: task, responseCh: make(chan error, 1)}
	select {
	case w.appendCh <- req:
	case <-w.stopCh:
		return ErrNotReady
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-req.responseCh:
		return err
	case <-w.stopCh:
		// The writer may have answered just before it stopped.
		select {
		case err := <-req.responseCh:
			return err
		default:
			return ErrNotReady
		}
	case <-ctx.Done():
		// The write may still land. The reader grants it as usual, and History refuses the grant
		// if it no longer wants it.
		return ctx.Err()
	}
}

func (w *taskWriter) writeLoop() {
writerLoop:
	for {
		select {
		case req := <-w.appendCh:
			if w.isStopped() {
				break writerLoop
			}
			reqs := w.getWriteBatch([]*writeTaskRequest{req})

			taskIDs, err := w.allocTaskIDs(len(reqs))
			if err != nil {
				w.logger.Error("Semaphore task writer failed to allocate task ids", tag.Error(err))
				w.sendWriteResponse(reqs, w.handleErr(err))
				// Only a failed renewal gets here. Stopped now, not left to the manager, so pending
				// appends do not each try to renew.
				w.stop()
				continue writerLoop
			}

			// Copies, so the callers' tasks are not changed.
			tasks := make([]*persistence.SemaphoreTask, len(reqs))
			for i, req := range reqs {
				task := *req.task
				task.TaskID = taskIDs[i]
				tasks[i] = &task
			}

			err = w.handleErr(w.db.createTasks(tasks))
			if err != nil {
				w.logger.Error("Semaphore task writer failed to write tasks",
					tag.Error(err),
					tag.Number(taskIDs[0]),
					tag.NextNumber(taskIDs[len(taskIDs)-1]))
			} else {
				// Only after a successful write, so the reader never reads ids that did not land.
				atomic.StoreInt64(&w.maxReadLevel, taskIDs[len(taskIDs)-1])
			}
			w.sendWriteResponse(reqs, err)
		case <-w.stopCh:
			break writerLoop
		}
	}
}

// getWriteBatch adds to reqs whatever appends are already waiting, up to maxTaskWriteBatchSize.
func (w *taskWriter) getWriteBatch(reqs []*writeTaskRequest) []*writeTaskRequest {
readLoop:
	for len(reqs) < maxTaskWriteBatchSize {
		select {
		case req := <-w.appendCh:
			reqs = append(reqs, req)
		default: // channel is empty, don't block
			break readLoop
		}
	}
	return reqs
}

func (w *taskWriter) sendWriteResponse(reqs []*writeTaskRequest, err error) {
	for _, req := range reqs {
		req.responseCh <- err
	}
}

// allocTaskIDs returns the next count task ids, claiming a new range when the block runs out.
func (w *taskWriter) allocTaskIDs(count int) ([]int64, error) {
	taskIDs := make([]int64, count)
	for i := range taskIDs {
		if w.taskIDBlock.start > w.taskIDBlock.end {
			block, err := w.allocTaskIDBlock()
			if err != nil {
				return nil, err
			}
			w.taskIDBlock = block
		}
		taskIDs[i] = w.taskIDBlock.start
		w.taskIDBlock.start++
	}
	return taskIDs, nil
}

// allocTaskIDBlock claims the next range, retrying transient errors, and returns its block of task
// ids. Any failure unloads the bucket.
func (w *taskWriter) allocTaskIDBlock() (taskIDBlock, error) {
	var state taskBucketState
	op := func(context.Context) error {
		var err error
		state, err = w.db.claim()
		return err
	}
	if err := w.throttleRetry.Do(context.Background(), op); err != nil {
		w.signalFatal()
		return taskIDBlock{}, err
	}
	return rangeIDToTaskIDBlock(state.rangeID, w.rangeSize), nil
}

// handleErr reports the bucket lost on a ConditionFailedError: another host has claimed it.
func (w *taskWriter) handleErr(err error) error {
	if errors.As(err, new(*persistence.ConditionFailedError)) {
		reportConditionFailed(w.logger, err)
		w.signalFatal()
	}
	return err
}

// reportConditionFailed logs that another host has claimed the bucket.
func reportConditionFailed(logger log.Logger, err error) {
	logger.Info("Stopping semaphore bucket due to persistence condition failure.", tag.Error(err))
}
