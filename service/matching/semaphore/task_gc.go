package semaphore

import (
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common/clock"
)

const (
	// taskDeleteBatchSize is how many finished tasks start a delete without waiting.
	taskDeleteBatchSize = 100
	// maxTimeBetweenTaskDeletes is how long fewer than taskDeleteBatchSize finished tasks wait for a
	// delete.
	maxTimeBetweenTaskDeletes = time.Second
)

// taskGC deletes finished tasks, the ones at or below the ack level. Each delete covers every task
// up to the ack level, so it also removes any an earlier host left behind. Only one caller runs a
// delete at a time; the others return.
type taskGC struct {
	lock           int64
	db             *taskDB
	timeSource     clock.TimeSource
	ackLevel       int64
	lastDeleteTime time.Time
}

func newTaskGC(db *taskDB, timeSource clock.TimeSource) *taskGC {
	return &taskGC{db: db, timeSource: timeSource}
}

// run deletes the tasks up to ackLevel once taskDeleteBatchSize are waiting, or once
// maxTimeBetweenTaskDeletes has passed since the last delete.
func (gc *taskGC) run(ackLevel int64) {
	gc.tryDeleteNextBatch(ackLevel, false)
}

// runNow deletes the tasks up to ackLevel without waiting.
func (gc *taskGC) runNow(ackLevel int64) {
	gc.tryDeleteNextBatch(ackLevel, true)
}

func (gc *taskGC) tryDeleteNextBatch(ackLevel int64, ignoreTimeCond bool) {
	if !gc.tryLock() {
		return
	}
	defer gc.unlock()
	if !gc.checkPrecond(ackLevel, ignoreTimeCond) {
		return
	}
	gc.lastDeleteTime = gc.timeSource.Now()
	// One call deletes every task up to ackLevel, so there is no batch loop. A failed delete is
	// covered by the next one.
	if err := gc.db.completeTasksUpTo(ackLevel); err != nil {
		return
	}
	gc.ackLevel = ackLevel
}

func (gc *taskGC) checkPrecond(ackLevel int64, ignoreTimeCond bool) bool {
	backlog := ackLevel - gc.ackLevel
	if backlog >= taskDeleteBatchSize {
		return true
	}
	return backlog > 0 && (ignoreTimeCond || gc.timeSource.Since(gc.lastDeleteTime) > maxTimeBetweenTaskDeletes)
}

func (gc *taskGC) tryLock() bool {
	return atomic.CompareAndSwapInt64(&gc.lock, 0, 1)
}

func (gc *taskGC) unlock() {
	atomic.StoreInt64(&gc.lock, 0)
}
