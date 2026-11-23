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
	"github.com/uber/cadence/common/constants"
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
	// grantRecordConcurrency is the most grants one bucket records in History at once.
	grantRecordConcurrency = 10
)

// taskReader grants tokens to tasks in task id order, and records each grant in History in its own
// goroutine. It also saves the ack level and deletes finished tasks.
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

	taskBuffer        chan *persistence.SemaphoreTask
	grantTask         func(ctx context.Context, task *persistence.SemaphoreTask) (tokenID int, ok bool)
	recordTaskGranted func(ctx context.Context, task *persistence.SemaphoreTask, tokenID int) bool
	signalGrantLoop   func()
	// grantRecordLimiter caps the grants being recorded at grantRecordConcurrency.
	grantRecordLimiter chan struct{}
	// outstandingGrantCount counts the grants History hasn't answered yet.
	outstandingGrantCount int64
	// notifyC wakes the read loop. It holds at most one signal.
	notifyC chan struct{}
	// gcNotifyC wakes the read loop to delete finished tasks. It holds at most one signal.
	gcNotifyC chan struct{}

	// stop cancels cancelCtx, which ends the read loop, the grant loop and every grant still being
	// recorded in History.
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
		taskGC:             newTaskGC(m.taskDB, m.timeSource),
		taskBuffer:         make(chan *persistence.SemaphoreTask, getTasksBatchSize),
		grantTask:          m.grantTask,
		recordTaskGranted:  m.recordTaskGranted,
		signalGrantLoop:    m.signalGrantLoop,
		grantRecordLimiter: make(chan struct{}, grantRecordConcurrency),
		notifyC:            make(chan struct{}, 1),
		gcNotifyC:          make(chan struct{}, 1),
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		fatalCh:            make(chan struct{}),
	}
}

// start runs the read loop and the grant loop. Call it after the claim.
func (tr *taskReader) start() {
	tr.signal()
	tr.wg.Add(2)
	go tr.getTasksPump()
	go tr.dispatchBufferedTasks()
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

// signalGC wakes the read loop to delete finished tasks. It never blocks.
func (tr *taskReader) signalGC() {
	select {
	case tr.gcNotifyC <- struct{}{}:
	default: // a signal is already waiting
	}
}

// hasTasks reports whether any task is waiting for a token.
func (tr *taskReader) hasTasks() bool {
	if tr.taskAckManager.GetReadLevel() < tr.writer.getMaxReadLevel() {
		// Written but not read yet.
		return true
	}
	// backlog counts the tasks read but not finished. It is read first: a finished grant leaves
	// outstandingGrantCount before it leaves the backlog, so a race can only overcount.
	backlog := tr.taskAckManager.GetBacklogCount()
	// Tasks being recorded in History already have their token. The rest are waiting for one.
	waiting := backlog - atomic.LoadInt64(&tr.outstandingGrantCount)
	return waiting > 0
}

// dispatchBufferedTasks dispatches buffered tasks in order.
func (tr *taskReader) dispatchBufferedTasks() {
	defer tr.wg.Done()
	for {
		select {
		case <-tr.cancelCtx.Done():
			return
		case task := <-tr.taskBuffer:
			// Taking one frees buffer room, so wake the read loop.
			tr.signal()
			if !tr.dispatchTask(task) {
				return
			}
		}
	}
}

// dispatchTask grants a waiting task a token and records the grant in History in a new goroutine.
// It returns false only if the reader stops first.
func (tr *taskReader) dispatchTask(task *persistence.SemaphoreTask) bool {
	// 1. Take a place in grantRecordLimiter before the token, so no token is held while waiting.
	select {
	case tr.grantRecordLimiter <- struct{}{}:
	case <-tr.cancelCtx.Done():
		return false
	}

	// 2. Grant a token.
	tokenID, ok := tr.grantTask(tr.cancelCtx, task)
	if !ok {
		<-tr.grantRecordLimiter
		return false
	}
	if tokenID == constants.NoSemaphoreTokenID {
		// With ok true, no token means the task expired and holds none: nothing to record.
		<-tr.grantRecordLimiter
		tr.completeTask(task)
		return true
	}

	// 3. Record the grant in its own goroutine, so a slow History call doesn't hold up later tasks.
	atomic.AddInt64(&tr.outstandingGrantCount, 1)
	tr.wg.Add(1)
	go func() {
		defer tr.wg.Done()
		defer func() { <-tr.grantRecordLimiter }()
		recorded := tr.recordTaskGranted(tr.cancelCtx, task, tokenID)
		// Lower the count before the ack, as hasTasks expects.
		atomic.AddInt64(&tr.outstandingGrantCount, -1)
		// If the reader stopped first, leave the task unacked so the next load records it.
		if recorded {
			tr.completeTask(task)
		}
	}()
	return true
}

// completeTask acks a finished task and wakes the read loop to delete it.
func (tr *taskReader) completeTask(task *persistence.SemaphoreTask) {
	tr.taskAckManager.AckItem(task.TaskID)
	tr.signalGC()
}

// getTasksPump reads tasks into taskBuffer and deletes finished tasks when signaled, and saves the
// ack level on a timer.
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
		case <-tr.gcNotifyC:
			// A task was acked: delete the finished tasks.
			tr.taskGC.run(tr.taskAckManager.GetAckLevel())
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
