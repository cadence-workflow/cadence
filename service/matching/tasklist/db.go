// Copyright (c) 2017 Uber Technologies, Inc.
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
	"sync"
	"sync/atomic"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
)

type (
	taskListDB struct {
		sync.RWMutex
		domainID        string
		domainName      string
		taskListName    string
		taskListKind    int
		taskType        int
		rangeID         int64
		backlogCount    int64
		ackLevel        int64
		partitionConfig *persistence.TaskListPartitionConfig
		store           persistence.TaskManager
		logger          log.Logger
		timeSrc         clock.TimeSource
	}
	taskListState struct {
		rangeID  int64
		ackLevel int64
	}
)

// newTaskListDB returns an instance of an object that represents
// persistence view of a taskList. All mutations / reads to taskLists
// wrt persistence go through this object.
//
// This class will serialize writes to persistence that do condition updates. There are
// two reasons for doing this:
//   - To work around known Cassandra issue where concurrent LWT to the same partition cause timeout errors
//   - To provide the guarantee that there is only writer who updates taskList in persistence at any given point in time
//     This guarantee makes some of the other code simpler and there is no impact to perf because updates to tasklist are
//     spread out and happen in background routines
func newTaskListDB(store persistence.TaskManager, domainID string, domainName string, name string, taskType int, kind int, logger log.Logger) *taskListDB {
	return &taskListDB{
		domainID:     domainID,
		domainName:   domainName,
		taskListName: name,
		taskListKind: kind,
		taskType:     taskType,
		store:        store,
		logger:       logger,
		timeSrc:      clock.NewRealTimeSource(),
	}
}

// RangeID returns the current persistence view of rangeID
func (db *taskListDB) RangeID() int64 {
	db.RLock()
	defer db.RUnlock()
	return db.rangeID
}

// BacklogCount returns the current backlog size
func (db *taskListDB) BacklogCount() int64 {
	return atomic.LoadInt64(&db.backlogCount)
}

func (db *taskListDB) PartitionConfig() *persistence.TaskListPartitionConfig {
	db.RLock()
	defer db.RUnlock()
	return db.partitionConfig
}

// RenewLease renews the lease on a tasklist. If there is no previous lease,
// this method will attempt to steal tasklist from current owner
func (db *taskListDB) RenewLease() (taskListState, error) {
	db.Lock()
	defer db.Unlock()
	resp, err := db.store.LeaseTaskList(context.Background(), &persistence.LeaseTaskListRequest{
		DomainID:     db.domainID,
		TaskList:     db.taskListName,
		TaskType:     db.taskType,
		TaskListKind: db.taskListKind,
		RangeID:      atomic.LoadInt64(&db.rangeID),
		DomainName:   db.domainName,
		UpdatedTime:  db.timeSrc.Now(),
	})
	if err != nil {
		return taskListState{}, err
	}
	db.rangeID = resp.TaskListInfo.RangeID
	db.ackLevel = resp.TaskListInfo.AckLevel
	db.partitionConfig = resp.TaskListInfo.AdaptivePartitionConfig
	return taskListState{rangeID: db.rangeID, ackLevel: resp.TaskListInfo.AckLevel}, nil
}

// UpdateState updates the taskList state with the given value
func (db *taskListDB) UpdateState(ackLevel int64) error {
	db.Lock()
	defer db.Unlock()
	_, err := db.store.UpdateTaskList(context.Background(), &persistence.UpdateTaskListRequest{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID:                db.domainID,
			Name:                    db.taskListName,
			TaskType:                db.taskType,
			AckLevel:                ackLevel,
			RangeID:                 db.rangeID,
			Kind:                    db.taskListKind,
			AdaptivePartitionConfig: db.partitionConfig,
		},
		DomainName:  db.domainName,
		UpdatedTime: db.timeSrc.Now(),
	})
	if err != nil {
		return err
	}
	db.ackLevel = ackLevel
	return nil
}

func (db *taskListDB) UpdateTaskListPartitionConfig(partitionConfig *persistence.TaskListPartitionConfig) error {
	db.Lock()
	defer db.Unlock()
	_, err := db.store.UpdateTaskList(context.Background(), &persistence.UpdateTaskListRequest{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID:                db.domainID,
			Name:                    db.taskListName,
			TaskType:                db.taskType,
			AckLevel:                db.ackLevel,
			RangeID:                 db.rangeID,
			Kind:                    db.taskListKind,
			AdaptivePartitionConfig: partitionConfig,
		},
		DomainName:  db.domainName,
		UpdatedTime: db.timeSrc.Now(),
	})
	if err != nil {
		return err
	}
	db.partitionConfig = partitionConfig
	return nil
}

// CreateTasks creates a batch of given tasks for this task list
func (db *taskListDB) CreateTasks(tasks []*persistence.CreateTaskInfo) (*persistence.CreateTasksResponse, error) {
	db.Lock()
	defer db.Unlock()
	return db.store.CreateTasks(context.Background(), &persistence.CreateTasksRequest{
		TaskListInfo: &persistence.TaskListInfo{
			DomainID: db.domainID,
			Name:     db.taskListName,
			TaskType: db.taskType,
			RangeID:  db.rangeID,
		},
		Tasks:       tasks,
		DomainName:  db.domainName,
		CreatedTime: db.timeSrc.Now(),
	})
}

// GetTasks returns a batch of tasks between the given range
func (db *taskListDB) GetTasks(minTaskID int64, maxTaskID int64, batchSize int) (*persistence.GetTasksResponse, error) {
	return db.store.GetTasks(context.Background(), &persistence.GetTasksRequest{
		DomainID:     db.domainID,
		TaskList:     db.taskListName,
		TaskType:     db.taskType,
		BatchSize:    batchSize,
		ReadLevel:    minTaskID,  // exclusive
		MaxReadLevel: &maxTaskID, // inclusive
		DomainName:   db.domainName,
	})
}

// CompleteTasksLessThan deletes of tasks less than the given taskID. Limit is
// the upper bound of number of tasks that can be deleted by this method. It may
// or may not be honored
func (db *taskListDB) CompleteTasksLessThan(taskID int64, limit int) (int, error) {
	resp, err := db.store.CompleteTasksLessThan(context.Background(), &persistence.CompleteTasksLessThanRequest{
		DomainID:     db.domainID,
		TaskListName: db.taskListName,
		TaskType:     db.taskType,
		TaskID:       taskID,
		Limit:        limit,
		DomainName:   db.domainName,
	})
	if err != nil {
		db.logger.Error("Persistent store operation failure",
			tag.StoreOperationCompleteTasksLessThan,
			tag.Error(err),
			tag.TaskID(taskID),
			tag.TaskType(db.taskType),
			tag.WorkflowTaskListName(db.taskListName))
		return 0, err
	}
	return resp.TasksCompleted, nil
}

// GetTaskListSize gets the backlog size of a tasklist
func (db *taskListDB) GetTaskListSize(ackLevel int64) (int64, error) {
	resp, err := db.store.GetTaskListSize(context.Background(), &persistence.GetTaskListSizeRequest{
		DomainID:     db.domainID,
		DomainName:   db.domainName,
		TaskListName: db.taskListName,
		TaskListType: db.taskType,
		AckLevel:     ackLevel,
	})
	if err != nil {
		return 0, err
	}
	atomic.StoreInt64(&db.backlogCount, resp.Size)
	return resp.Size, nil
}

func (db *taskListDB) GetTaskListInfo(taskListName string) (*persistence.TaskListInfo, error) {
	resp, err := db.store.GetTaskList(context.Background(), &persistence.GetTaskListRequest{
		DomainID:   db.domainID,
		DomainName: db.domainName,
		TaskList:   taskListName,
		TaskType:   db.taskType,
	})
	if err != nil {
		return nil, err
	}
	return resp.TaskListInfo, nil
}
