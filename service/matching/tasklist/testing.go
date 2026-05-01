// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

package tasklist

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/emirpasic/gods/maps/treemap"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
)

var _ persistence.TaskManager = (*TestTaskManager)(nil) // Asserts that interface is indeed implemented

type (
	TestTaskManager struct {
		sync.Mutex
		t          *testing.T
		taskLists  map[Identifier]*testTaskListManager
		logger     log.Logger
		timeSource clock.TimeSource
	}

	testTaskListManager struct {
		sync.RWMutex
		rangeID                 int64
		ackLevel                int64
		kind                    int
		createTaskCount         int
		tasks                   *treemap.Map
		adaptivePartitionConfig *persistence.TaskListPartitionConfig
	}
)

func newTestTaskListManager() *testTaskListManager {
	return &testTaskListManager{tasks: treemap.NewWith(int64Comparator)}
}

func NewTestTaskManager(t *testing.T, logger log.Logger, timeSource clock.TimeSource) *TestTaskManager {
	return &TestTaskManager{
		t:          t,
		taskLists:  make(map[Identifier]*testTaskListManager),
		logger:     logger,
		timeSource: timeSource,
	}
}

func (m *TestTaskManager) GetName() string {
	return "test"
}

func (m *TestTaskManager) Close() {
}

// LeaseTaskList provides a mock function with given fields: ctx, request
func (m *TestTaskManager) LeaseTaskList(
	_ context.Context,
	request *persistence.LeaseTaskListRequest,
) (*persistence.LeaseTaskListResponse, error) {
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, request.DomainID, request.TaskList, request.TaskType))
	tlm.Lock()
	defer tlm.Unlock()
	if request.RangeID > 0 && request.RangeID != tlm.rangeID {
		return nil, &persistence.ConditionFailedError{
			Msg: fmt.Sprintf("leaseTaskList:renew failed: taskList:%v, taskListType:%v, haveRangeID:%v, gotRangeID:%v",
				request.TaskList, request.TaskType, request.RangeID, tlm.rangeID),
		}
	}
	tlm.rangeID++
	tlm.kind = request.TaskListKind
	m.logger.Debug(fmt.Sprintf("testTaskManager.LeaseTaskList rangeID=%v", tlm.rangeID))

	return &persistence.LeaseTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			AckLevel:                tlm.ackLevel,
			DomainID:                request.DomainID,
			Name:                    request.TaskList,
			TaskType:                request.TaskType,
			RangeID:                 tlm.rangeID,
			Kind:                    tlm.kind,
			AdaptivePartitionConfig: tlm.adaptivePartitionConfig,
		},
	}, nil
}

func (m *TestTaskManager) GetTaskList(
	_ context.Context,
	request *persistence.GetTaskListRequest,
) (*persistence.GetTaskListResponse, error) {
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, request.DomainID, request.TaskList, request.TaskType))
	tlm.RLock()
	defer tlm.RUnlock()
	return &persistence.GetTaskListResponse{
		TaskListInfo: &persistence.TaskListInfo{
			AckLevel:                tlm.ackLevel,
			DomainID:                request.DomainID,
			Name:                    request.TaskList,
			TaskType:                request.TaskType,
			RangeID:                 tlm.rangeID,
			Kind:                    tlm.kind,
			AdaptivePartitionConfig: tlm.adaptivePartitionConfig,
		},
	}, nil
}

// UpdateTaskList provides a mock function with given fields: ctx, request
func (m *TestTaskManager) UpdateTaskList(
	_ context.Context,
	request *persistence.UpdateTaskListRequest,
) (*persistence.UpdateTaskListResponse, error) {
	m.logger.Debug(fmt.Sprintf("testTaskManager.UpdateTaskList taskListInfo=%v, ackLevel=%v", request.TaskListInfo, request.TaskListInfo.AckLevel))

	tli := request.TaskListInfo
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, tli.DomainID, tli.Name, tli.TaskType))

	tlm.Lock()
	defer tlm.Unlock()
	if tlm.rangeID != tli.RangeID {
		return nil, &persistence.ConditionFailedError{
			Msg: fmt.Sprintf("Failed to update task list: name=%v, type=%v, expected rangeID=%v, input rangeID=%v", tli.Name, tli.TaskType, tlm.rangeID, tli.RangeID),
		}
	}
	tlm.ackLevel = tli.AckLevel
	return &persistence.UpdateTaskListResponse{}, nil
}

// CompleteTask provides a mock function with given fields: ctx, request
func (m *TestTaskManager) CompleteTask(
	_ context.Context,
	request *persistence.CompleteTaskRequest,
) error {
	m.logger.Debug(fmt.Sprintf("testTaskManager.CompleteTask taskID=%v, ackLevel=%v", request.TaskID, request.TaskList.AckLevel))
	if request.TaskID <= 0 {
		panic(fmt.Errorf("Invalid taskID=%v", request.TaskID))
	}

	tli := request.TaskList
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, tli.DomainID, tli.Name, tli.TaskType))

	tlm.Lock()
	defer tlm.Unlock()

	tlm.tasks.Remove(request.TaskID)
	return nil
}

// CompleteTasksLessThan provides a mock function with given fields: ctx, request
func (m *TestTaskManager) CompleteTasksLessThan(
	_ context.Context,
	request *persistence.CompleteTasksLessThanRequest,
) (*persistence.CompleteTasksLessThanResponse, error) {
	m.logger.Debug(fmt.Sprintf("testTaskManager.CompleteTasksLessThan taskID=%v", request.TaskID))
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, request.DomainID, request.TaskListName, request.TaskType))
	tlm.Lock()
	defer tlm.Unlock()
	rowsDeleted := 0
	keys := tlm.tasks.Keys()
	for _, key := range keys {
		id := key.(int64)
		if id <= request.TaskID {
			tlm.tasks.Remove(id)
			rowsDeleted++
		}
	}
	return &persistence.CompleteTasksLessThanResponse{TasksCompleted: rowsDeleted}, nil
}

// ListTaskList provides a mock function with given fields: ctx, request
func (m *TestTaskManager) ListTaskList(
	_ context.Context,
	request *persistence.ListTaskListRequest,
) (*persistence.ListTaskListResponse, error) {
	return nil, fmt.Errorf("unsupported operation")
}

// DeleteTaskList provides a mock function with given fields: ctx, request
func (m *TestTaskManager) DeleteTaskList(
	_ context.Context,
	request *persistence.DeleteTaskListRequest,
) error {
	m.Lock()
	defer m.Unlock()
	key := NewTestTaskListID(m.t, request.DomainID, request.TaskListName, request.TaskListType)
	delete(m.taskLists, *key)
	return nil
}

// CreateTask provides a mock function with given fields: ctx, request
func (m *TestTaskManager) CreateTasks(
	_ context.Context,
	request *persistence.CreateTasksRequest,
) (*persistence.CreateTasksResponse, error) {
	domainID := request.TaskListInfo.DomainID
	taskList := request.TaskListInfo.Name
	taskType := request.TaskListInfo.TaskType
	rangeID := request.TaskListInfo.RangeID

	tlm := m.getTaskListManager(NewTestTaskListID(m.t, domainID, taskList, taskType))
	tlm.Lock()
	defer tlm.Unlock()

	// First validate the entire batch
	for _, task := range request.Tasks {
		m.logger.Debug(fmt.Sprintf("testTaskManager.CreateTask taskID=%v, rangeID=%v", task.TaskID, rangeID))
		if task.TaskID <= 0 {
			panic(fmt.Errorf("Invalid taskID=%v", task.TaskID))
		}

		if tlm.rangeID != rangeID {
			m.logger.Debug(fmt.Sprintf("testTaskManager.CreateTask ConditionFailedError taskID=%v, rangeID: %v, db rangeID: %v",
				task.TaskID, rangeID, tlm.rangeID))

			return nil, &persistence.ConditionFailedError{
				Msg: fmt.Sprintf("testTaskManager.CreateTask failed. TaskList: %v, taskType: %v, rangeID: %v, db rangeID: %v",
					taskList, taskType, rangeID, tlm.rangeID),
			}
		}
		_, ok := tlm.tasks.Get(task.TaskID)
		if ok {
			panic(fmt.Sprintf("Duplicated TaskID %v", task.TaskID))
		}
	}

	// Then insert all tasks if no errors
	for _, task := range request.Tasks {
		scheduleID := task.Data.ScheduleID
		info := &persistence.TaskInfo{
			DomainID:        domainID,
			WorkflowID:      task.Data.WorkflowID,
			RunID:           task.Data.RunID,
			ScheduleID:      scheduleID,
			TaskID:          task.TaskID,
			PartitionConfig: task.Data.PartitionConfig,
		}
		if task.Data.ScheduleToStartTimeoutSeconds != 0 {
			info.Expiry = m.timeSource.Now().Add(time.Duration(task.Data.ScheduleToStartTimeoutSeconds) * time.Second)
		}
		tlm.tasks.Put(task.TaskID, info)
		tlm.createTaskCount++
	}

	return &persistence.CreateTasksResponse{}, nil
}

// GetTasks provides a mock function with given fields: ctx, request
func (m *TestTaskManager) GetTasks(
	_ context.Context,
	request *persistence.GetTasksRequest,
) (*persistence.GetTasksResponse, error) {
	m.logger.Debug(fmt.Sprintf("testTaskManager.GetTasks readLevel=%v, maxReadLevel=%v", request.ReadLevel, *request.MaxReadLevel))

	tlm := m.getTaskListManager(NewTestTaskListID(m.t, request.DomainID, request.TaskList, request.TaskType))
	tlm.Lock()
	defer tlm.Unlock()
	var tasks []*persistence.TaskInfo

	it := tlm.tasks.Iterator()
	for it.Next() {
		taskID := it.Key().(int64)
		if taskID <= request.ReadLevel {
			continue
		}
		if taskID > *request.MaxReadLevel {
			break
		}
		tasks = append(tasks, it.Value().(*persistence.TaskInfo))
	}
	return &persistence.GetTasksResponse{
		Tasks: tasks,
	}, nil
}

func (m *TestTaskManager) GetTaskListSize(_ context.Context, request *persistence.GetTaskListSizeRequest) (*persistence.GetTaskListSizeResponse, error) {
	tlm := m.getTaskListManager(NewTestTaskListID(m.t, request.DomainID, request.TaskListName, request.TaskListType))
	tlm.Lock()
	defer tlm.Unlock()
	count := int64(0)
	it := tlm.tasks.Iterator()
	for it.Next() {
		taskID := it.Key().(int64)
		if taskID <= request.AckLevel {
			continue
		}
		count++
	}
	return &persistence.GetTaskListSizeResponse{Size: count}, nil
}

func (m *TestTaskManager) GetOrphanTasks(_ context.Context, request *persistence.GetOrphanTasksRequest) (*persistence.GetOrphanTasksResponse, error) {
	return &persistence.GetOrphanTasksResponse{}, nil
}

// getTaskCount returns number of tasks in a task list
func (m *TestTaskManager) GetTaskCount(taskList *Identifier) int {
	tlm := m.getTaskListManager(taskList)
	tlm.Lock()
	defer tlm.Unlock()
	return tlm.tasks.Size()
}

// getCreateTaskCount returns how many times CreateTask was called
func (m *TestTaskManager) GetCreateTaskCount(taskList *Identifier) int {
	tlm := m.getTaskListManager(taskList)
	tlm.Lock()
	defer tlm.Unlock()
	return tlm.createTaskCount
}

func (m *TestTaskManager) SetRangeID(taskList *Identifier, rangeID int64) {
	tlm := m.getTaskListManager(taskList)
	tlm.Lock()
	defer tlm.Unlock()
	tlm.rangeID = rangeID
}

func (m *TestTaskManager) GetRangeID(taskList *Identifier) int64 {
	tlm := m.getTaskListManager(taskList)
	tlm.Lock()
	defer tlm.Unlock()
	return tlm.rangeID
}

func (m *TestTaskManager) getTaskListManager(id *Identifier) *testTaskListManager {
	m.Lock()
	defer m.Unlock()
	result, ok := m.taskLists[*id]
	if ok {
		return result
	}
	result = newTestTaskListManager()
	m.taskLists[*id] = result
	return result
}

func (m *TestTaskManager) String() string {
	m.Lock()
	defer m.Unlock()
	var result string
	for id, tl := range m.taskLists {
		tl.Lock()
		if id.taskType == persistence.TaskListTypeActivity {
			result += "Activity"
		} else {
			result += "Decision"
		}
		result += " task list " + id.name
		result += "\n"
		result += fmt.Sprintf("AckLevel=%v\n", tl.ackLevel)
		result += fmt.Sprintf("CreateTaskCount=%v\n", tl.createTaskCount)
		result += fmt.Sprintf("RangeID=%v\n", tl.rangeID)
		result += "Tasks=\n"
		for _, t := range tl.tasks.Values() {
			result += spew.Sdump(t)
			result += "\n"

		}
		tl.Unlock()
	}
	return result
}

func NewTestTaskListID(t *testing.T, domainID string, taskListName string, taskType int) *Identifier {
	id, err := NewIdentifier(domainID, taskListName, taskType)
	if err != nil {
		t.Fatal("failed to create task list identifier for test", err)
	}
	return id
}

func int64Comparator(a, b interface{}) int {
	aAsserted := a.(int64)
	bAsserted := b.(int64)
	switch {
	case aAsserted > bAsserted:
		return 1
	case aAsserted < bAsserted:
		return -1
	default:
		return 0
	}
}
