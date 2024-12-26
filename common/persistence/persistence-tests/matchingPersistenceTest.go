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

package persistencetests

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	// MatchingPersistenceSuite contains matching persistence tests
	MatchingPersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

// TimePrecision is needed to account for database timestamp precision.
// Cassandra only provides milliseconds timestamp precision, so we need to use tolerance when doing comparison
const TimePrecision = 2 * time.Millisecond

// SetupSuite implementation
func (s *MatchingPersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// TearDownSuite implementation
func (s *MatchingPersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// SetupTest implementation
func (s *MatchingPersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

// TestCreateTask test
func (s *MatchingPersistenceSuite) TestCreateTask() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "11adbd1b-f164-4ea7-b2f3-2e857a5048f1"
	workflowExecution := types.WorkflowExecution{WorkflowID: "create-task-test",
		RunID: "c949447a-691a-4132-8b2a-a5b38106793c"}
	partitionConfig := map[string]string{"userid": uuid.New()}
	task0, err0 := s.CreateDecisionTask(ctx, domainID, workflowExecution, "a5b38106793c", 5, partitionConfig)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	tasks1, err1 := s.CreateActivityTasks(ctx, domainID, workflowExecution, map[int64]string{
		10: "a5b38106793c"}, partitionConfig)
	s.NoError(err1)
	s.NotNil(tasks1, "Expected valid task identifiers.")
	s.Equal(1, len(tasks1), "expected single valid task identifier.")
	for _, t := range tasks1 {
		s.NotEmpty(t, "Expected non empty task identifier.")
	}

	tasks := map[int64]string{
		20: uuid.New(),
		30: uuid.New(),
		40: uuid.New(),
		50: uuid.New(),
		60: uuid.New(),
	}
	tasks2, err2 := s.CreateActivityTasks(ctx, domainID, workflowExecution, tasks, partitionConfig)
	s.NoError(err2)
	s.Equal(5, len(tasks2), "expected single valid task identifier.")

	for sid, tlName := range tasks {
		resp, err := s.GetTasks(ctx, domainID, tlName, p.TaskListTypeActivity, 100)
		s.NoError(err)
		s.Equal(1, len(resp.Tasks))
		s.Equal(domainID, resp.Tasks[0].DomainID)
		s.Equal(workflowExecution.WorkflowID, resp.Tasks[0].WorkflowID)
		s.Equal(workflowExecution.RunID, resp.Tasks[0].RunID)
		s.Equal(sid, resp.Tasks[0].ScheduleID)
		s.True(resp.Tasks[0].CreatedTime.UnixNano() > 0)
		if s.TaskMgr.GetName() != "cassandra" && s.TaskMgr.GetName() != "shardedNosql" {
			// cassandra uses TTL and expiry isn't stored as part of task state
			s.True(time.Now().Before(resp.Tasks[0].Expiry))
			s.True(resp.Tasks[0].Expiry.Before(time.Now().Add((defaultScheduleToStartTimeout + 1) * time.Second)))
		}
		s.Equal(partitionConfig, resp.Tasks[0].PartitionConfig)
	}
}

// TestGetDecisionTasks test
func (s *MatchingPersistenceSuite) TestGetDecisionTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "aeac8287-527b-4b35-80a9-667cb47e7c6d"
	workflowExecution := types.WorkflowExecution{WorkflowID: "get-decision-task-test",
		RunID: "db20f7e2-1a1e-40d9-9278-d8b886738e05"}
	taskList := "d8b886738e05"
	partitionConfig := map[string]string{"userid": uuid.New()}
	task0, err0 := s.CreateDecisionTask(ctx, domainID, workflowExecution, taskList, 5, partitionConfig)
	s.NoError(err0)
	s.NotNil(task0, "Expected non empty task identifier.")

	tasks1Response, err1 := s.GetTasks(ctx, domainID, taskList, p.TaskListTypeDecision, 1)
	s.NoError(err1)
	s.NotNil(tasks1Response.Tasks, "expected valid list of tasks.")
	s.Equal(1, len(tasks1Response.Tasks), "Expected 1 decision task.")
	s.Equal(int64(5), tasks1Response.Tasks[0].ScheduleID)
	s.Equal(partitionConfig, tasks1Response.Tasks[0].PartitionConfig)
}

func (s *MatchingPersistenceSuite) TestGetTaskListSize() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	workflowExecution := types.WorkflowExecution{WorkflowID: "get-decision-task-test",
		RunID: "db20f7e2-1a1e-40d9-9278-d8b886738e05"}
	taskList := "d8b886738e05"
	partitionConfig := map[string]string{"userid": uuid.New()}

	size, err1 := s.GetDecisionTaskListSize(ctx, domainID, taskList, 0)
	s.NoError(err1)
	s.Equal(int64(0), size)

	task0, err0 := s.CreateDecisionTask(ctx, domainID, workflowExecution, taskList, 5, partitionConfig)
	s.NoError(err0)

	size, err1 = s.GetDecisionTaskListSize(ctx, domainID, taskList, task0)
	s.NoError(err1)
	s.Equal(int64(0), size)

	size, err1 = s.GetDecisionTaskListSize(ctx, domainID, taskList, task0-1)
	s.NoError(err1)
	s.Equal(int64(1), size)
}

// TestGetTasksWithNoMaxReadLevel test
func (s *MatchingPersistenceSuite) TestGetTasksWithNoMaxReadLevel() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "f1116985-d1f1-40e0-aba9-83344db915bc"
	workflowExecution := types.WorkflowExecution{WorkflowID: "complete-decision-task-test",
		RunID: "2aa0a74e-16ee-4f27-983d-48b07ec1915d"}
	taskList := "48b07ec1915d"
	_, err0 := s.CreateActivityTasks(ctx, domainID, workflowExecution, map[int64]string{
		10: taskList,
		20: taskList,
		30: taskList,
		40: taskList,
		50: taskList,
	}, nil)
	s.NoError(err0)

	nTasks := 5
	firstTaskID := s.GetNextSequenceNumber() - int64(nTasks)

	testCases := []struct {
		batchSz   int
		readLevel int64
		taskIDs   []int64
	}{
		{1, -1, []int64{firstTaskID}},
		{2, firstTaskID, []int64{firstTaskID + 1, firstTaskID + 2}},
		{5, firstTaskID + 2, []int64{firstTaskID + 3, firstTaskID + 4}},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("tc_%v_%v", tc.batchSz, tc.readLevel), func() {
			response, err := s.TaskMgr.GetTasks(ctx, &p.GetTasksRequest{
				DomainID:  domainID,
				TaskList:  taskList,
				TaskType:  p.TaskListTypeActivity,
				BatchSize: tc.batchSz,
				ReadLevel: tc.readLevel,
			})
			s.NoError(err)
			s.Equal(len(tc.taskIDs), len(response.Tasks), "wrong number of tasks")
			for i := range tc.taskIDs {
				s.Equal(tc.taskIDs[i], response.Tasks[i].TaskID, "wrong set of tasks")
			}
		})
	}
}

// TestCompleteDecisionTask test
func (s *MatchingPersistenceSuite) TestCompleteDecisionTask() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := "f1116985-d1f1-40e0-aba9-83344db915bc"
	workflowExecution := types.WorkflowExecution{WorkflowID: "complete-decision-task-test",
		RunID: "2aa0a74e-16ee-4f27-983d-48b07ec1915d"}
	taskList := "48b07ec1915d"
	tasks0, err0 := s.CreateActivityTasks(ctx, domainID, workflowExecution, map[int64]string{
		10: taskList,
		20: taskList,
		30: taskList,
		40: taskList,
		50: taskList,
	}, nil)
	s.NoError(err0)
	s.NotNil(tasks0, "Expected non empty task identifier.")
	s.Equal(5, len(tasks0), "expected 5 valid task identifier.")
	for _, t := range tasks0 {
		s.NotEmpty(t, "Expected non empty task identifier.")
	}

	tasksWithID1Response, err1 := s.GetTasks(ctx, domainID, taskList, p.TaskListTypeActivity, 5)

	s.NoError(err1)
	tasksWithID1 := tasksWithID1Response.Tasks
	s.NotNil(tasksWithID1, "expected valid list of tasks.")

	s.Equal(5, len(tasksWithID1), "Expected 5 activity tasks.")
	for _, t := range tasksWithID1 {
		s.Equal(domainID, t.DomainID)
		s.Equal(workflowExecution.WorkflowID, t.WorkflowID)
		s.Equal(workflowExecution.RunID, t.RunID)
		s.True(t.TaskID > 0)

		err2 := s.CompleteTask(ctx, domainID, taskList, p.TaskListTypeActivity, t.TaskID, 100)
		s.NoError(err2)
	}
}

// TestCompleteTasksLessThan test
func (s *MatchingPersistenceSuite) TestCompleteTasksLessThan() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	taskList := "range-complete-task-tl0"
	wfExec := types.WorkflowExecution{
		WorkflowID: "range-complete-task-test",
		RunID:      uuid.New(),
	}
	_, err := s.CreateActivityTasks(ctx, domainID, wfExec, map[int64]string{
		10: taskList,
		20: taskList,
		30: taskList,
		40: taskList,
		50: taskList,
		60: taskList,
	}, nil)
	s.NoError(err)

	resp, err := s.GetTasks(ctx, domainID, taskList, p.TaskListTypeActivity, 10)
	s.NoError(err)
	s.NotNil(resp.Tasks)
	s.Equal(6, len(resp.Tasks), "getTasks returned wrong number of tasks")

	tasks := resp.Tasks

	testCases := []struct {
		taskID int64
		limit  int
		output []int64
	}{
		{
			taskID: tasks[5].TaskID,
			limit:  1,
			output: []int64{tasks[1].TaskID, tasks[2].TaskID, tasks[3].TaskID, tasks[4].TaskID, tasks[5].TaskID},
		},
		{
			taskID: tasks[5].TaskID,
			limit:  2,
			output: []int64{tasks[3].TaskID, tasks[4].TaskID, tasks[5].TaskID},
		},
		{
			taskID: tasks[5].TaskID,
			limit:  10,
			output: []int64{},
		},
	}

	remaining := len(resp.Tasks)
	req := &p.CompleteTasksLessThanRequest{DomainID: domainID, TaskListName: taskList, TaskType: p.TaskListTypeActivity, Limit: 1}

	for _, tc := range testCases {
		req.TaskID = tc.taskID
		req.Limit = tc.limit
		result, err := s.TaskMgr.CompleteTasksLessThan(ctx, req)
		s.NoError(err)
		resp, err := s.GetTasks(ctx, domainID, taskList, p.TaskListTypeActivity, 10)
		s.NoError(err)
		if result.TasksCompleted == p.UnknownNumRowsAffected {
			s.Equal(0, len(resp.Tasks), "expected all tasks to be deleted")
			break
		}
		s.Equal(remaining-len(tc.output), result.TasksCompleted, "expected only LIMIT number of rows to be deleted")
		s.Equal(len(tc.output), len(resp.Tasks), "rangeCompleteTask deleted wrong set of tasks")
		for i := range tc.output {
			s.Equal(tc.output[i], resp.Tasks[i].TaskID)
		}
		remaining = len(tc.output)
	}
}

// TestLeaseAndUpdateTaskList test
func (s *MatchingPersistenceSuite) TestLeaseAndUpdateTaskList() {
	domainID := "00136543-72ad-4615-b7e9-44bca9775b45"
	taskList := "aaaaaaa"
	leaseTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	response, err := s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: p.TaskListTypeActivity,
	})
	s.NoError(err)
	tli := response.TaskListInfo
	s.EqualValues(1, tli.RangeID)
	s.EqualValues(0, tli.AckLevel)
	s.True(tli.LastUpdated.After(leaseTime) || tli.LastUpdated.Equal(leaseTime))
	s.EqualValues(p.TaskListKindNormal, tli.Kind)
	s.Nil(tli.AdaptivePartitionConfig)

	leaseTime = time.Now()
	response, err = s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: p.TaskListTypeActivity,
	})
	s.NoError(err)
	tli = response.TaskListInfo
	s.EqualValues(2, tli.RangeID)
	s.EqualValues(0, tli.AckLevel)
	s.True(tli.LastUpdated.After(leaseTime) || tli.LastUpdated.Equal(leaseTime))
	s.EqualValues(p.TaskListKindNormal, tli.Kind)
	s.Nil(tli.AdaptivePartitionConfig)

	_, err = s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: p.TaskListTypeActivity,
		RangeID:  1,
	})
	s.Error(err)
	_, ok := err.(*p.ConditionFailedError)
	s.True(ok)

	readPartitions := map[int]*p.TaskListPartition{
		0: {},
		1: {
			IsolationGroups: []string{"foo"},
		},
	}
	writePartitions := map[int]*p.TaskListPartition{
		0: {IsolationGroups: []string{"bar"}},
	}

	taskListInfo := &p.TaskListInfo{
		DomainID: domainID,
		Name:     taskList,
		TaskType: p.TaskListTypeActivity,
		RangeID:  2,
		AckLevel: 0,
		Kind:     p.TaskListKindNormal,
		AdaptivePartitionConfig: &p.TaskListPartitionConfig{
			Version:         1,
			ReadPartitions:  readPartitions,
			WritePartitions: writePartitions,
		},
	}
	_, err = s.TaskMgr.UpdateTaskList(ctx, &p.UpdateTaskListRequest{
		TaskListInfo: taskListInfo,
	})
	s.NoError(err)

	var resp *p.GetTaskListResponse
	resp, err = s.TaskMgr.GetTaskList(ctx, &p.GetTaskListRequest{
		DomainID: domainID,
		TaskList: taskList,
		TaskType: p.TaskListTypeActivity,
	})
	s.NoError(err)
	tli = resp.TaskListInfo
	s.EqualValues(2, tli.RangeID)
	s.EqualValues(0, tli.AckLevel)
	s.True(tli.LastUpdated.After(leaseTime) || tli.LastUpdated.Equal(leaseTime))
	s.EqualValues(p.TaskListKindNormal, tli.Kind)
	s.NotNil(tli.AdaptivePartitionConfig)
	s.EqualValues(1, tli.AdaptivePartitionConfig.Version)
	s.Equal(readPartitions, tli.AdaptivePartitionConfig.ReadPartitions)
	s.EqualValues(writePartitions, tli.AdaptivePartitionConfig.WritePartitions)

	taskListInfo.RangeID = 3
	_, err = s.TaskMgr.UpdateTaskList(ctx, &p.UpdateTaskListRequest{
		TaskListInfo: taskListInfo,
	})
	s.Error(err)
	_, ok = err.(*p.ConditionFailedError)
	s.True(ok)
}

// TestLeaseAndUpdateTaskListSticky test
func (s *MatchingPersistenceSuite) TestLeaseAndUpdateTaskListSticky() {
	domainID := uuid.New()
	taskList := "aaaaaaa"

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	response, err := s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
		DomainID:     domainID,
		TaskList:     taskList,
		TaskType:     p.TaskListTypeDecision,
		TaskListKind: p.TaskListKindSticky,
	})
	s.NoError(err)
	tli := response.TaskListInfo
	s.EqualValues(1, tli.RangeID)
	s.EqualValues(0, tli.AckLevel)
	s.EqualValues(p.TaskListKindSticky, tli.Kind)
	s.Nil(tli.AdaptivePartitionConfig)

	taskListInfo := &p.TaskListInfo{
		DomainID: domainID,
		Name:     taskList,
		TaskType: p.TaskListTypeDecision,
		RangeID:  tli.RangeID,
		AckLevel: 0,
		Kind:     p.TaskListKindSticky,
	}
	_, err = s.TaskMgr.UpdateTaskList(ctx, &p.UpdateTaskListRequest{
		TaskListInfo: taskListInfo,
	})
	s.NoError(err)
}

func (s *MatchingPersistenceSuite) deleteAllTaskList() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	var nextPageToken []byte
	for {
		resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 10, PageToken: nextPageToken})
		s.NoError(err)
		for _, it := range resp.Items {
			err = s.TaskMgr.DeleteTaskList(ctx, &p.DeleteTaskListRequest{
				DomainID:     it.DomainID,
				TaskListName: it.Name,
				TaskListType: it.TaskType,
				RangeID:      it.RangeID,
			})
			s.NoError(err)
		}
		nextPageToken = resp.NextPageToken
		if nextPageToken == nil {
			break
		}
	}
}

// TestListWithOneTaskList test
func (s *MatchingPersistenceSuite) TestListWithOneTaskList() {
	if s.TaskMgr.GetName() == "cassandra" || s.TaskMgr.GetName() == "shardedNosql" {
		// ListTaskList API is currently not supported in cassandra
		return
	}
	s.deleteAllTaskList()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 10})
	s.NoError(err)
	s.Nil(resp.NextPageToken)
	s.Equal(0, len(resp.Items))

	rangeID := int64(0)
	ackLevel := int64(0)
	domainID := uuid.New()
	for i := 0; i < 10; i++ {
		rangeID++
		updatedTime := time.Now()
		_, err := s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
			DomainID:     domainID,
			TaskList:     "list-task-list-test-tl0",
			TaskType:     p.TaskListTypeActivity,
			TaskListKind: p.TaskListKindSticky,
		})
		s.NoError(err)

		resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 10})
		s.NoError(err)

		s.Equal(1, len(resp.Items))
		s.Equal(domainID, resp.Items[0].DomainID)
		s.Equal("list-task-list-test-tl0", resp.Items[0].Name)
		s.Equal(p.TaskListTypeActivity, resp.Items[0].TaskType)
		s.Equal(p.TaskListKindSticky, resp.Items[0].Kind)
		s.Equal(rangeID, resp.Items[0].RangeID)
		s.Equal(ackLevel, resp.Items[0].AckLevel)
		s.True(resp.Items[0].LastUpdated.After(updatedTime) || resp.Items[0].LastUpdated.Equal(updatedTime))

		ackLevel++
		updatedTime = time.Now()
		_, err = s.TaskMgr.UpdateTaskList(ctx, &p.UpdateTaskListRequest{
			TaskListInfo: &p.TaskListInfo{
				DomainID: domainID,
				Name:     "list-task-list-test-tl0",
				TaskType: p.TaskListTypeActivity,
				RangeID:  rangeID,
				AckLevel: ackLevel,
				Kind:     p.TaskListKindSticky,
			},
		})
		s.NoError(err)

		resp, err = s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 10})
		s.NoError(err)
		s.Equal(1, len(resp.Items))
		s.True(resp.Items[0].LastUpdated.After(updatedTime) || resp.Items[0].LastUpdated.Equal(updatedTime))
	}
	s.deleteAllTaskList()
}

// TestListWithMultipleTaskList test
func (s *MatchingPersistenceSuite) TestListWithMultipleTaskList() {
	if s.TaskMgr.GetName() == "cassandra" || s.TaskMgr.GetName() == "shardedNosql" {
		// ListTaskList API is currently not supported in cassandra"
		return
	}
	s.deleteAllTaskList()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	domainID := uuid.New()
	tlNames := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("test-list-with-multiple-%v", i)
		_, err := s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
			DomainID:     domainID,
			TaskList:     name,
			TaskType:     p.TaskListTypeActivity,
			TaskListKind: p.TaskListKindNormal,
		})
		s.NoError(err)
		tlNames[name] = struct{}{}
		listedNames := make(map[string]struct{})
		var nextPageToken []byte
		for {
			resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 1, PageToken: nextPageToken})
			s.NoError(err)
			for _, it := range resp.Items {
				s.Equal(domainID, it.DomainID)
				s.Equal(p.TaskListTypeActivity, it.TaskType)
				s.Equal(p.TaskListKindNormal, it.Kind)
				_, ok := listedNames[it.Name]
				s.False(ok, "list API returns duplicate entries - have: %+v got:%v", listedNames, it.Name)
				listedNames[it.Name] = struct{}{}
			}
			nextPageToken = resp.NextPageToken
			if nextPageToken == nil {
				break
			}
		}
		s.Equal(tlNames, listedNames, "list API returned wrong set of task list names")
	}

	// final test again pagination
	total := 0
	var nextPageToken []byte
	for {
		resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{
			PageSize:  6,
			PageToken: nextPageToken,
		})
		s.NoError(err)
		total += len(resp.Items)
		if resp.NextPageToken == nil {
			break
		}
		nextPageToken = resp.NextPageToken
	}
	s.Equal(10, total)

	s.deleteAllTaskList()
	resp, err := s.TaskMgr.ListTaskList(ctx, &p.ListTaskListRequest{PageSize: 10})
	s.NoError(err)
	s.Nil(resp.NextPageToken)
	s.Equal(0, len(resp.Items))
}

func (s *MatchingPersistenceSuite) TestGetOrphanTasks() {
	if os.Getenv("SKIP_GET_ORPHAN_TASKS") != "" {
		s.T().Skipf("GetOrphanTasks not supported in %v", s.TaskMgr.GetName())
	}
	if s.TaskMgr.GetName() == "cassandra" || s.TaskMgr.GetName() == "shardedNosql" {
		// GetOrphanTasks API is currently not supported in cassandra"
		return
	}
	s.deleteAllTaskList()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	oresp, err := s.TaskMgr.GetOrphanTasks(ctx, &p.GetOrphanTasksRequest{Limit: 10})
	s.NoError(err)
	// existing orphans that caused by other tests
	existingOrphans := len(oresp.Tasks)

	domainID := uuid.New()
	name := "test-list-with-orphans"
	resp, err := s.TaskMgr.LeaseTaskList(ctx, &p.LeaseTaskListRequest{
		DomainID:     domainID,
		TaskList:     name,
		TaskType:     p.TaskListTypeActivity,
		TaskListKind: p.TaskListKindNormal,
	})
	s.NoError(err)

	wid := uuid.New()
	rid := uuid.New()
	s.TaskMgr.CreateTasks(ctx, &p.CreateTasksRequest{
		TaskListInfo: resp.TaskListInfo,
		Tasks: []*p.CreateTaskInfo{
			{
				Data: &p.TaskInfo{
					DomainID:                      domainID,
					WorkflowID:                    wid,
					RunID:                         rid,
					TaskID:                        0,
					ScheduleID:                    0,
					ScheduleToStartTimeoutSeconds: 0,
					Expiry:                        time.Now(),
					CreatedTime:                   time.Now(),
				},
				TaskID: 0,
			},
		},
	})

	oresp, err = s.TaskMgr.GetOrphanTasks(ctx, &p.GetOrphanTasksRequest{Limit: 10})
	s.NoError(err)

	s.Equal(existingOrphans, len(oresp.Tasks))

	s.deleteAllTaskList()

	oresp, err = s.TaskMgr.GetOrphanTasks(ctx, &p.GetOrphanTasksRequest{Limit: 10})
	s.NoError(err)

	s.Equal(existingOrphans+1, len(oresp.Tasks))
	found := false
	for _, it := range oresp.Tasks {
		if it.DomainID != domainID {
			continue
		}
		s.Equal(p.TaskListTypeActivity, it.TaskType)
		s.Equal(int64(0), it.TaskID)
		s.Equal(name, it.TaskListName)
		found = true
	}
	s.True(found)
}
