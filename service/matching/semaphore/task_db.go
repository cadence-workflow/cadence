package semaphore

import (
	"context"
	"fmt"
	"sync"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// taskBucketState is the bucket's control row in semaphore_tasks, as a claim returned it.
type taskBucketState struct {
	rangeID  int64
	ackLevel int64
}

// taskDB calls semaphore_tasks for one bucket. Its writes are fenced by the range_id this host
// claimed, so they fail with ConditionFailedError once another host claims the bucket.
type taskDB struct {
	id     Identifier
	store  persistence.SemaphoreTaskManager
	logger log.Logger

	// mu sends the store calls one at a time, as the task list's does: concurrent conditional
	// writes to one partition time out.
	mu sync.Mutex
	// rangeID is zero until the first claim.
	rangeID int64
}

func newTaskDB(id Identifier, store persistence.SemaphoreTaskManager, logger log.Logger) *taskDB {
	return &taskDB{id: id, store: store, logger: logger}
}

// claim takes the bucket and bumps its range_id. The first claim passes 0 and takes the bucket
// from any holder; a renewal passes the held range_id and fails if another host claimed since.
func (db *taskDB) claim() (taskBucketState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	resp, err := db.store.ClaimSemaphoreTaskBucket(context.Background(), &persistence.ClaimSemaphoreTaskBucketRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		RangeID:       db.rangeID,
	})
	if err != nil {
		return taskBucketState{}, err
	}
	if resp == nil {
		return taskBucketState{}, &types.InternalServiceError{Message: fmt.Sprintf("no claim response returned for bucket %v", db.id)}
	}
	db.rangeID = resp.RangeID
	return taskBucketState{rangeID: resp.RangeID, ackLevel: resp.AckLevel}, nil
}

// updateAckLevel writes ackLevel to the control row. It does not bump range_id, so it also checks
// that this host still holds the bucket.
func (db *taskDB) updateAckLevel(ackLevel int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.store.UpdateSemaphoreTaskBucketState(context.Background(), &persistence.UpdateSemaphoreTaskBucketStateRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		RangeID:       db.rangeID,
		AckLevel:      ackLevel,
	})
	return err
}

// createTasks writes tasks in one batch.
func (db *taskDB) createTasks(tasks []*persistence.SemaphoreTask) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	_, err := db.store.CreateSemaphoreTasks(context.Background(), &persistence.CreateSemaphoreTasksRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		RangeID:       db.rangeID,
		Tasks:         tasks,
	})
	return err
}

// getTasks reads up to batchSize tasks with ids in (readLevel, maxReadLevel], in id order. A read
// is not a conditional write, so it does not take mu.
func (db *taskDB) getTasks(readLevel, maxReadLevel int64, batchSize int) ([]*persistence.SemaphoreTask, error) {
	resp, err := db.store.GetSemaphoreTasks(context.Background(), &persistence.GetSemaphoreTasksRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		ReadLevel:     readLevel,
		MaxReadLevel:  maxReadLevel,
		BatchSize:     batchSize,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, &types.InternalServiceError{Message: fmt.Sprintf("no tasks response returned for bucket %v", db.id)}
	}
	return resp.Tasks, nil
}

// completeTasksUpTo deletes every task with an id at or below ackLevel, and logs a failure. Not a
// conditional write, so it does not take mu.
func (db *taskDB) completeTasksUpTo(ackLevel int64) error {
	_, err := db.store.RangeCompleteSemaphoreTasks(context.Background(), &persistence.RangeCompleteSemaphoreTasksRequest{
		DomainID:      db.id.DomainID,
		SemaphoreName: db.id.SemaphoreName,
		Bucket:        db.id.Bucket,
		ReadLevel:     0,
		AckLevel:      ackLevel,
	})
	if err != nil {
		db.logger.Error("Persistent store operation failure",
			tag.StoreOperationCompleteTasksLessThan,
			tag.Error(err),
			tag.TaskID(ackLevel))
	}
	return err
}
