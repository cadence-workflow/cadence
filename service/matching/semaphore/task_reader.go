package semaphore

import (
	"context"
	"errors"
	"sync"
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
	// getTasksBatchSize is the most waiters one read returns, and the buffer size.
	getTasksBatchSize = 100
	// readRangeSize is how many task ids one query covers.
	readRangeSize = 50_000
	// maxReadWindows is how many empty windows one pass reads before it yields to the loop.
	maxReadWindows = 10
)

// taskReader reads waiters into taskBuffer in task id order and saves the ack level.
//
// The ack level is saved on every tick, even when unchanged: the write is fenced on range_id, so
// it detects that another host claimed the bucket.
type taskReader struct {
	db             *taskDB
	writer         *taskWriter
	taskAckManager messaging.AckManager
	logger         log.Logger
	timeSource     clock.TimeSource
	updateInterval func() time.Duration
	readRangeSize  int64
	throttleRetry  *backoff.ThrottleRetry

	taskBuffer chan *persistence.SemaphoreTask
	// notifyC wakes the read loop. It holds at most one signal.
	notifyC chan struct{}

	// cancelCtx is canceled by stop. It ends the read loop.
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

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
		taskBuffer: make(chan *persistence.SemaphoreTask, getTasksBatchSize),
		notifyC:    make(chan struct{}, 1),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		fatalCh:    make(chan struct{}),
	}
}

// start runs the read loop, which reads waiters when signaled and saves the ack level on a timer.
// Call it after the writer's claim.
func (tr *taskReader) start() {
	tr.signal()
	tr.wg.Add(1)
	go tr.getTasksPump()
}

// stop stops the reader and waits for its goroutine to exit. Safe to call more than once, and
// before start.
//
// TODO: Once waiters are acked, save the ack level once more and delete the rows below it.
func (tr *taskReader) stop() {
	tr.cancelFunc()
	tr.wg.Wait()
}

// fatal returns a channel that is closed once this host has lost the bucket.
func (tr *taskReader) fatal() <-chan struct{} {
	return tr.fatalCh
}

func (tr *taskReader) signalFatal() {
	tr.fatalOnce.Do(func() { close(tr.fatalCh) })
}

// signal wakes the read loop to look for new waiters. It never blocks.
func (tr *taskReader) signal() {
	select {
	case tr.notifyC <- struct{}{}:
	default: // a signal is already waiting
	}
}

// hasWaiters reports whether a waiter is saved but not read, or read but not acked. The read level
// is checked first, since ReadItem counts a waiter in the backlog before moving the read level.
//
// TODO: Acquire calls it before granting, so a newcomer queues behind existing waiters.
func (tr *taskReader) hasWaiters() bool {
	if tr.taskAckManager.GetReadLevel() < tr.writer.getMaxReadLevel() {
		return true
	}
	return tr.taskAckManager.GetBacklogCount() > 0
}

// getTasksPump reads waiters into taskBuffer when signaled, and saves the ack level on a timer.
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
			// There may be new waiters: read the next ones into the buffer.
			if !tr.readTasks() {
				return
			}
		case <-updateAckTimer.Chan():
			// Save the ack level. The write also checks that this host still holds the bucket.
			err := tr.db.updateAckLevel(tr.taskAckManager.GetAckLevel())
			if errors.As(err, new(*persistence.ConditionFailedError)) {
				reportConditionFailed(tr.logger, err)
				tr.signalFatal()
				return
			}
			if err != nil {
				// Not fatal: the next save writes it again.
				tr.logger.Error("Semaphore task reader failed to save the ack level", tag.Error(err))
			}
			// Also look for waiters, in case a signal was missed.
			tr.signal()
			updateAckTimer.Reset(tr.updateInterval())
		}
	}
}

// readTasks reads as many waiters as taskBuffer has room for, so adding them never blocks. It
// returns false if the reader must stop.
func (tr *taskReader) readTasks() bool {
	room := cap(tr.taskBuffer) - len(tr.taskBuffer)
	if room == 0 {
		// Nothing to read into. Not a reason to stop: the loop that takes waiters from the buffer
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
	// There may be more waiters.
	tr.signal()
	return true
}

// getTaskBatch reads up to batchSize waiters in (readLevel, maxReadLevel], one window at a time,
// stopping at the first window with waiters. It returns the level reached and whether that is
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

// getTaskBatchWithRange reads up to batchSize waiters with ids in (readLevel, maxReadLevel],
// retrying transient errors.
func (tr *taskReader) getTaskBatchWithRange(readLevel, maxReadLevel int64, batchSize int) ([]*persistence.SemaphoreTask, error) {
	var tasks []*persistence.SemaphoreTask
	op := func(context.Context) error {
		var err error
		tasks, err = tr.db.getTasks(readLevel, maxReadLevel, batchSize)
		return err
	}
	if err := tr.throttleRetry.Do(context.Background(), op); err != nil {
		tr.logger.Error("Semaphore task reader failed to read waiters", tag.Error(err))
		return nil, err
	}
	return tasks, nil
}

// isTaskExpired reports whether task's acquire deadline has passed. A task with no deadline never
// expires.
func (tr *taskReader) isTaskExpired(task *persistence.SemaphoreTask) bool {
	return task.AcquireDeadline != nil && tr.timeSource.Now().After(*task.AcquireDeadline)
}

// addTaskToBuffer registers task with the ack manager and buffers it. A waiter past its deadline
// is skipped. It returns false once the reader must stop.
//
// TODO: History's acquire timeout must end a skipped waiter's hold.
func (tr *taskReader) addTaskToBuffer(task *persistence.SemaphoreTask) bool {
	if tr.isTaskExpired(task) {
		// Moves the read level past it, or a batch of only expired waiters would be read again.
		tr.taskAckManager.SetReadLevel(task.TaskID)
		return true
	}
	if err := tr.taskAckManager.ReadItem(task.TaskID); err != nil {
		tr.logger.Error("Semaphore task reader read a waiter out of order", tag.TaskID(task.TaskID), tag.Error(err))
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
