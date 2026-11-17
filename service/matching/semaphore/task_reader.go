package semaphore

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/persistence"
)

const (
	// getTasksBatchSize is the most tasks one read returns, and the buffer size.
	getTasksBatchSize = 100
	// readRangeSize is how many task ids one query covers.
	readRangeSize = 50_000
	// maxReadWindows is how many empty windows one pass reads before it yields to the loop.
	maxReadWindows = 10
)

// taskReader passes tasks to grantTask one at a time, in task id order. It saves the ack level on
// a timer and on stop, and deletes finished tasks. Each save also checks that this host still owns
// the bucket.
type taskReader struct {
	db             *taskDB
	writer         *taskWriter
	taskAckManager messaging.AckManager
	logger         log.Logger
	timeSource     clock.TimeSource
	updateInterval func() time.Duration
	readRangeSize  int64
	throttleRetry  *backoff.ThrottleRetry

	taskGC *taskGC

	taskBuffer      chan *persistence.SemaphoreTask
	grantTask       func(context.Context, *persistence.SemaphoreTask) bool
	signalGrantLoop func()
	// notifyC wakes the read loop. It holds at most one signal.
	notifyC chan struct{}

	// cancelCtx is canceled by stop. It ends the read loop.
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	// stopped is set to 1 by the first stop.
	stopped int64

	// fatalCh is closed when this host has lost the bucket. The reader only reports it; the
	// manager stops the bucket.
	fatalCh   chan struct{}
	fatalOnce sync.Once
}

// newTaskReader builds the reader from its manager's fields. Build the writer first.
func newTaskReader(m *semaphoreManagerImpl) *taskReader {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	return &taskReader{
		db:             m.taskDB,
		writer:         m.taskWriter,
		taskAckManager: m.taskAckManager,
		logger:         m.logger,
		timeSource:     m.timeSource,
		updateInterval: m.config.UpdateAckInterval,
		readRangeSize:  readRangeSize,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(common.CreatePersistenceRetryPolicy()),
			backoff.WithRetryableError(persistence.IsTransientError),
		),
		taskGC:          newTaskGC(m.taskDB, m.timeSource),
		taskBuffer:      make(chan *persistence.SemaphoreTask, getTasksBatchSize),
		grantTask:       m.grantTask,
		signalGrantLoop: m.signalGrantLoop,
		notifyC:         make(chan struct{}, 1),
		cancelCtx:       cancelCtx,
		cancelFunc:      cancelFunc,
		fatalCh:         make(chan struct{}),
	}
}

// start runs the read loop and the grant loop. Call it after the claim.
func (tr *taskReader) start() {
	tr.signal()
	tr.wg.Add(2)
	go tr.getTasksPump()
	go tr.grantTasksLoop()
}

func (tr *taskReader) stop() {
	if atomic.CompareAndSwapInt64(&tr.stopped, 0, 1) {
		tr.cancelFunc()
		tr.wg.Wait()
		if err := tr.persistAckLevel(); err != nil {
			tr.logger.Error("Semaphore task reader failed to save the ack level", tag.Error(err))
		}
		tr.taskGC.runNow(tr.taskAckManager.GetAckLevel())
	}
}

func (tr *taskReader) persistAckLevel() error {
	ackLevel := tr.taskAckManager.GetAckLevel()
	if ackLevel < 0 {
		return nil
	}
	return tr.db.updateAckLevel(ackLevel)
}

// fatal returns a channel that is closed once this host has lost the bucket.
func (tr *taskReader) fatal() <-chan struct{} {
	return tr.fatalCh
}

func (tr *taskReader) signalFatal() {
	tr.fatalOnce.Do(func() { close(tr.fatalCh) })
}

// signal wakes the read loop to look for new tasks. It never blocks.
func (tr *taskReader) signal() {
	select {
	case tr.notifyC <- struct{}{}:
	default: // a signal is already waiting
	}
}

// hasTasks reports whether any task is written but not acked. The read level is checked first,
// since a read counts the task in the backlog before it moves the read level.
func (tr *taskReader) hasTasks() bool {
	if tr.taskAckManager.GetReadLevel() < tr.writer.getMaxReadLevel() {
		return true
	}
	return tr.taskAckManager.GetBacklogCount() > 0
}

// grantTasksLoop grants buffered tasks in order, acks each once it is finished, and deletes
// finished tasks.
func (tr *taskReader) grantTasksLoop() {
	defer tr.wg.Done()
	for {
		select {
		case <-tr.cancelCtx.Done():
			return
		case task := <-tr.taskBuffer:
			// Taking one frees buffer room, so wake the read loop.
			tr.signal()
			if !tr.grantTask(tr.cancelCtx, task) {
				return
			}
			tr.taskGC.run(tr.taskAckManager.AckItem(task.TaskID))
		}
	}
}

// getTasksPump reads tasks into taskBuffer when signaled, and saves the ack level on a timer.
func (tr *taskReader) getTasksPump() {
	defer tr.wg.Done()
	// A timer reset after each save, not a ticker, so a change to the interval takes effect.
	updateAckTimer := tr.timeSource.NewTimer(tr.updateInterval())
	defer updateAckTimer.Stop()
	for {
		select {
		case <-tr.cancelCtx.Done():
			// Stop was called.
			return
		case <-tr.notifyC:
			// There may be new tasks: read the next ones into the buffer.
			if !tr.readTasks() {
				return
			}
		case <-updateAckTimer.Chan():
			// Save the ack level. The write also checks that this host still holds the bucket.
			err := tr.persistAckLevel()
			if errors.As(err, new(*persistence.ConditionFailedError)) {
				reportConditionFailed(tr.logger, err)
				tr.signalFatal()
				return
			}
			if err != nil {
				// Not fatal: the next save writes it again.
				tr.logger.Error("Semaphore task reader failed to save the ack level", tag.Error(err))
			}
			// Also look for new tasks and free tokens, in case a signal was missed.
			tr.signal()
			tr.signalGrantLoop()
			updateAckTimer.Reset(tr.updateInterval())
		}
	}
}

// readTasks reads as many tasks as taskBuffer has room for, so adding them never blocks. It
// returns false if the reader must stop.
func (tr *taskReader) readTasks() bool {
	room := cap(tr.taskBuffer) - len(tr.taskBuffer)
	if room == 0 {
		// Nothing to read into. Not a reason to stop: the loop that takes tasks from the buffer
		// signals the reader, and the next pass reads then.
		return true
	}
	initialReadLevel := tr.taskAckManager.GetReadLevel()
	maxReadLevel := tr.writer.getMaxReadLevel()

	tasks, readLevel, isReadBatchDone, err := tr.getTaskBatch(initialReadLevel, maxReadLevel, room)
	if err != nil {
		// Already retried; try again.
		tr.signal()
		return true
	}

	if len(tasks) == 0 {
		tr.taskAckManager.SetReadLevel(readLevel)
		if tr.taskAckManager.GetAckLevel() == initialReadLevel {
			tr.taskAckManager.SetAckLevel(readLevel)
		}
		if !isReadBatchDone {
			tr.signal()
		}
		return true
	}

	for _, task := range tasks {
		if !tr.addTaskToBuffer(task) {
			return false
		}
	}
	// There may be more tasks.
	tr.signal()
	return true
}

// getTaskBatch reads up to batchSize tasks in (readLevel, maxReadLevel], one window at a time,
// stopping at the first window with tasks. It returns the level reached and whether that is
// maxReadLevel.
func (tr *taskReader) getTaskBatch(readLevel, maxReadLevel int64, batchSize int) ([]*persistence.SemaphoreTask, int64, bool, error) {
	for i := 0; i < maxReadWindows && readLevel < maxReadLevel; i++ {
		upper := min(readLevel+tr.readRangeSize, maxReadLevel)
		tasks, err := tr.getTaskBatchWithRange(readLevel, upper, batchSize)
		if err != nil {
			return nil, readLevel, true, err
		}
		if len(tasks) > 0 {
			return tasks, upper, true, nil
		}
		readLevel = upper
	}
	return nil, readLevel, readLevel == maxReadLevel, nil
}

// getTaskBatchWithRange reads up to batchSize tasks with ids in (readLevel, maxReadLevel],
// retrying transient errors.
func (tr *taskReader) getTaskBatchWithRange(readLevel, maxReadLevel int64, batchSize int) ([]*persistence.SemaphoreTask, error) {
	var tasks []*persistence.SemaphoreTask
	op := func(context.Context) error {
		var err error
		tasks, err = tr.db.getTasks(readLevel, maxReadLevel, batchSize)
		return err
	}
	if err := tr.throttleRetry.Do(context.Background(), op); err != nil {
		tr.logger.Error("Semaphore task reader failed to read tasks", tag.Error(err))
		return nil, err
	}
	return tasks, nil
}

// addTaskToBuffer registers task with the ack manager and buffers it, even if expired: its owner
// may hold a token History never confirmed. It returns false once the reader must stop.
func (tr *taskReader) addTaskToBuffer(task *persistence.SemaphoreTask) bool {
	if err := tr.taskAckManager.ReadItem(task.TaskID); err != nil {
		tr.logger.Error("Semaphore task reader read a task out of order", tag.TaskID(task.TaskID), tag.Error(err))
		tr.signalFatal()
		return false
	}
	select {
	case tr.taskBuffer <- task:
		return true
	case <-tr.cancelCtx.Done():
		return false
	}
}
