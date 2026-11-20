package semaphore

import (
	"time"

	"github.com/uber/cadence/common/clock"
)

// maxTimeBetweenTaskDeletes is how long after one delete, failed or not, the next can run. Each delete
// writes one range tombstone, so this caps a busy bucket at one tombstone per interval.
const maxTimeBetweenTaskDeletes = 10 * time.Second

// taskGC deletes every task at or below the ack level
type taskGC struct {
	db             *taskDB
	timeSource     clock.TimeSource
	ackLevel       int64
	lastDeleteTime time.Time
}

func newTaskGC(db *taskDB, timeSource clock.TimeSource) *taskGC {
	return &taskGC{db: db, timeSource: timeSource}
}

// run deletes the tasks up to ackLevel once maxTimeBetweenTaskDeletes has passed since the last
// delete.
func (gc *taskGC) run(ackLevel int64) {
	gc.tryDeleteNextBatch(ackLevel, false)
}

// runNow deletes the tasks up to ackLevel without waiting.
func (gc *taskGC) runNow(ackLevel int64) {
	gc.tryDeleteNextBatch(ackLevel, true)
}

func (gc *taskGC) tryDeleteNextBatch(ackLevel int64, ignoreTimeCond bool) {
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
	return backlog > 0 && (ignoreTimeCond || gc.timeSource.Since(gc.lastDeleteTime) > maxTimeBetweenTaskDeletes)
}
