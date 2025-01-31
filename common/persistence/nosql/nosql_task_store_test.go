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

package nosql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	TestDomainID     = "test-domain-id"
	TestDomainName   = "test-domain"
	TestTaskListName = "test-tasklist"
	TestTaskType     = persistence.TaskListTypeDecision
	TestWorkflowID   = "test-workflow-id"
	TestRunID        = "test-run-id"
)

func TestNewNoSQLStore(t *testing.T) {
	registerCassandraMock(t)
	cfg := getValidShardedNoSQLConfig()

	store, err := newNoSQLTaskStore(cfg, log.NewNoop(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, store)
}

func setupNoSQLStoreMocks(t *testing.T) (*nosqlTaskStore, *nosqlplugin.MockDB) {
	ctrl := gomock.NewController(t)
	dbMock := nosqlplugin.NewMockDB(ctrl)

	nosqlSt := nosqlStore{
		logger: log.NewNoop(),
		db:     dbMock,
	}

	shardedNosqlStoreMock := NewMockshardedNosqlStore(ctrl)
	shardedNosqlStoreMock.EXPECT().
		GetStoreShardByTaskList(
			TestDomainID,
			TestTaskListName,
			TestTaskType).
		Return(&nosqlSt, nil).
		AnyTimes()
	shardedNosqlStoreMock.EXPECT().GetLogger().Return(log.NewNoop()).AnyTimes()

	store := &nosqlTaskStore{
		shardedNosqlStore: shardedNosqlStoreMock,
	}

	return store, dbMock
}

func TestGetOrphanTasks(t *testing.T) {
	store, _ := setupNoSQLStoreMocks(t)

	// We just expect the function to return an error so we don't need to check the result
	_, err := store.GetOrphanTasks(context.Background(), nil)

	var expectedErr *types.InternalServiceError
	assert.ErrorAs(t, err, &expectedErr)
	assert.ErrorContains(t, err, "Unimplemented call to GetOrphanTasks for NoSQL")
}

func TestGetTaskListSize(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	db.EXPECT().GetTasksCount(
		gomock.Any(),
		&nosqlplugin.TasksFilter{
			TaskListFilter: *getDecisionTaskListFilter(),
			MinTaskID:      456,
		},
	).Return(int64(123), nil)

	size, err := store.GetTaskListSize(context.Background(), &persistence.GetTaskListSizeRequest{
		DomainID:     TestDomainID,
		DomainName:   TestDomainName,
		TaskListName: TestTaskListName,
		TaskListType: int(types.TaskListTypeDecision),
		AckLevel:     456,
	})

	assert.NoError(t, err)
	assert.Equal(t,
		&persistence.GetTaskListSizeResponse{Size: 123},
		size,
	)
}

func TestLeaseTaskList_emptyTaskList(t *testing.T) {
	store, _ := setupNoSQLStoreMocks(t)

	req := getValidLeaseTaskListRequest()
	req.TaskList = ""
	_, err := store.LeaseTaskList(context.Background(), req)

	assert.ErrorAs(t, err, new(*types.InternalServiceError))
	assert.ErrorContains(t, err, "LeaseTaskList requires non empty task list")
}

func TestLeaseTaskList_selectErr(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	req := getValidLeaseTaskListRequest()
	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(nil, assert.AnError)
	// The error is _not_ a NotFoundError
	db.EXPECT().IsNotFoundError(assert.AnError).Return(false).Times(2)
	db.EXPECT().IsTimeoutError(assert.AnError).Return(true).Times(1)

	_, err := store.LeaseTaskList(context.Background(), req)

	assert.Error(t, err)
	// The function does not wrap the error, it just adds it to the message
	assert.ErrorContains(t, err, assert.AnError.Error())
}

func TestLeaseTaskList_selectErrNotFound(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	req := getValidLeaseTaskListRequest()
	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(nil, assert.AnError)
	// The error _is_ a NotFoundError
	db.EXPECT().IsNotFoundError(assert.AnError).Return(true)

	// We then expect the tasklist to be inserted
	db.EXPECT().InsertTaskList(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, taskList *nosqlplugin.TaskListRow) error {
			tl := getExpectedTaskListRow()
			checkTaskListRowExpected(t, tl, taskList)
			return nil
		})

	resp, err := store.LeaseTaskList(context.Background(), req)

	assert.NoError(t, err)
	checkTaskListInfoExpected(t, resp.TaskListInfo)
}

func TestLeaseTaskList_BadRenew(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	req := getValidLeaseTaskListRequest()
	req.RangeID = 1 // Greater than 0, so we are trying to renew

	taskListRow := getExpectedTaskListRow()
	taskListRow.RangeID = 5 // The range ID in the DB is different from the one in the request

	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(taskListRow, nil)

	_, err := store.LeaseTaskList(context.Background(), req)

	assert.ErrorAs(t, err, new(*persistence.ConditionFailedError))
	expectedMessage := "leaseTaskList:renew failed: taskList:test-tasklist, taskListType:0, haveRangeID:1, gotRangeID:5"
	assert.ErrorContains(t, err, expectedMessage)
}

func TestLeaseTaskList_Renew(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	taskListRow := getExpectedTaskListRow()
	taskListRow.RangeID = 0 // The range ID in the DB is the same as the one in the request

	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(taskListRow, nil)
	db.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any(), int64(0)).
		DoAndReturn(func(ctx context.Context, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			checkTaskListRowExpected(t, getExpectedTaskListRow(), taskList)
			return nil
		})

	resp, err := store.LeaseTaskList(context.Background(), getValidLeaseTaskListRequest())

	assert.NoError(t, err)
	checkTaskListInfoExpected(t, resp.TaskListInfo)
}

func TestLeaseTaskList_RenewUpdateFailed_OperationConditionFailure(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	taskListRow := getExpectedTaskListRow()
	taskListRow.RangeID = 0 // The range ID in the DB is the same as the one in the request

	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(taskListRow, nil)
	db.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any(), int64(0)).
		DoAndReturn(func(ctx context.Context, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			checkTaskListRowExpected(t, getExpectedTaskListRow(), taskList)
			return &nosqlplugin.TaskOperationConditionFailure{RangeID: 10}
		})

	_, err := store.LeaseTaskList(context.Background(), getValidLeaseTaskListRequest())

	assert.ErrorAs(t, err, new(*persistence.ConditionFailedError))
	expectedMessage := "leaseTaskList: taskList:test-tasklist, taskListType:0, haveRangeID:1, gotRangeID:10"
	assert.ErrorContains(t, err, expectedMessage)
}

func TestLeaseTaskList_RenewUpdateFailed_OtherError(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	taskListRow := getExpectedTaskListRow()
	taskListRow.RangeID = 0 // The range ID in the DB is the same as the one in the request

	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(taskListRow, nil)
	db.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any(), int64(0)).
		DoAndReturn(func(ctx context.Context, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			checkTaskListRowExpected(t, getExpectedTaskListRow(), taskList)
			return assert.AnError
		})
	db.EXPECT().IsNotFoundError(assert.AnError).Return(true)

	_, err := store.LeaseTaskList(context.Background(), getValidLeaseTaskListRequest())

	assert.Error(t, err)
	// The function does not wrap the error, it just adds it to the message
	assert.ErrorContains(t, err, assert.AnError.Error())
}

func TestGetTaskList_Success(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	taskListRow := getExpectedTaskListRow()
	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(taskListRow, nil)
	resp, err := store.GetTaskList(context.Background(), getValidGetTaskListRequest())

	assert.NoError(t, err)
	checkTaskListInfoExpected(t, resp.TaskListInfo)
}

func TestGetTaskList_NotFound(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	db.EXPECT().SelectTaskList(gomock.Any(), getDecisionTaskListFilter()).Return(nil, errors.New("not found"))
	db.EXPECT().IsNotFoundError(gomock.Any()).Return(true)
	resp, err := store.GetTaskList(context.Background(), getValidGetTaskListRequest())

	assert.ErrorAs(t, err, new(*types.EntityNotExistsError))
	assert.Nil(t, resp)
}

func TestUpdateTaskList(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	db.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any(), int64(1)).DoAndReturn(
		func(ctx context.Context, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			checkTaskListRowExpected(t, getExpectedTaskListRowWithPartitionConfig(), taskList)
			return nil
		},
	)

	resp, err := store.UpdateTaskList(context.Background(), &persistence.UpdateTaskListRequest{
		TaskListInfo: getExpectedTaskListInfo(),
	})

	assert.NoError(t, err)
	assert.Equal(t, &persistence.UpdateTaskListResponse{}, resp)
}

func TestUpdateTaskList_Sticky(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	db.EXPECT().UpdateTaskListWithTTL(gomock.Any(), stickyTaskListTTL, gomock.Any(), int64(1)).DoAndReturn(
		func(ctx context.Context, ttlSeconds int64, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			expectedTaskList := getExpectedTaskListRowWithPartitionConfig()
			expectedTaskList.TaskListKind = int(types.TaskListKindSticky)
			checkTaskListRowExpected(t, expectedTaskList, taskList)
			return nil
		},
	)

	taskListInfo := getExpectedTaskListInfo()
	taskListInfo.Kind = int(types.TaskListKindSticky)

	resp, err := store.UpdateTaskList(context.Background(), &persistence.UpdateTaskListRequest{
		TaskListInfo: taskListInfo,
	})

	assert.NoError(t, err)
	assert.Equal(t, &persistence.UpdateTaskListResponse{}, resp)
}

func TestUpdateTaskList_ConditionFailure(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	db.EXPECT().UpdateTaskList(gomock.Any(), gomock.Any(), int64(1)).DoAndReturn(
		func(ctx context.Context, taskList *nosqlplugin.TaskListRow, previousRangeID int64) error {
			checkTaskListRowExpected(t, getExpectedTaskListRowWithPartitionConfig(), taskList)
			return &nosqlplugin.TaskOperationConditionFailure{Details: "test-details"}
		},
	)

	_, err := store.UpdateTaskList(context.Background(), &persistence.UpdateTaskListRequest{
		TaskListInfo: getExpectedTaskListInfo(),
	})

	var expectedErr *persistence.ConditionFailedError
	assert.ErrorAs(t, err, &expectedErr)
	assert.ErrorContains(t, err, "Failed to update task list. name: test-tasklist, type: 0, rangeID: 1, columns: (test-details)")
}

func TestDeleteTaskList(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)
	db.EXPECT().DeleteTaskList(gomock.Any(), getDecisionTaskListFilter(), int64(0)).Return(nil)

	err := store.DeleteTaskList(context.Background(), getValidDeleteTaskListRequest())
	assert.NoError(t, err)
}

func TestDeleteTaskList_ConditionFailure(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)
	db.EXPECT().DeleteTaskList(gomock.Any(), getDecisionTaskListFilter(), int64(0)).Return(
		&nosqlplugin.TaskOperationConditionFailure{Details: "test-details"},
	)

	err := store.DeleteTaskList(context.Background(), getValidDeleteTaskListRequest())

	var expectedErr *persistence.ConditionFailedError
	assert.ErrorAs(t, err, &expectedErr)
	assert.ErrorContains(t, err, "Failed to delete task list. name: test-tasklist, type: 0, rangeID: 0, columns: (test-details)")
}

func TestGetTasks(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)
	now := time.Unix(123, 456)

	taskrow1 := nosqlplugin.TaskRow{
		DomainID:        TestDomainID,
		TaskListName:    TestTaskListName,
		TaskListType:    int(types.TaskListTypeDecision),
		TaskID:          5,
		WorkflowID:      TestWorkflowID,
		RunID:           TestRunID,
		ScheduledID:     0,
		CreatedTime:     now,
		PartitionConfig: nil,
	}

	taskrow2 := taskrow1
	taskrow2.TaskID = 6

	db.EXPECT().SelectTasks(gomock.Any(), &nosqlplugin.TasksFilter{
		TaskListFilter: *getDecisionTaskListFilter(),
		BatchSize:      100,
		MinTaskID:      1,
		MaxTaskID:      15,
	}).Return([]*nosqlplugin.TaskRow{&taskrow1, &taskrow2}, nil)

	resp, err := store.GetTasks(context.Background(), getValidGetTasksRequest())

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Tasks, 2)
	taskRowEqualTaskInfo(t, taskrow1, resp.Tasks[0])
	taskRowEqualTaskInfo(t, taskrow2, resp.Tasks[1])
}

func TestGetTasks_Empty(t *testing.T) {
	store, _ := setupNoSQLStoreMocks(t)

	request := getValidGetTasksRequest()
	// Set the max read level to be less than the min read level
	request.ReadLevel = 10
	request.MaxReadLevel = common.Int64Ptr(5)
	resp, err := store.GetTasks(context.Background(), request)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Tasks)
}

func TestCompleteTask(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	// The main assertion is that the correct parameters are passed to the DB
	db.EXPECT().RangeDeleteTasks(gomock.Any(), &nosqlplugin.TasksFilter{
		TaskListFilter: *getDecisionTaskListFilter(),
		MinTaskID:      11,
		MaxTaskID:      12,
		BatchSize:      1,
	}).Return(1, nil)

	err := store.CompleteTask(context.Background(), &persistence.CompleteTaskRequest{
		TaskList: &persistence.TaskListInfo{
			DomainID: TestDomainID,
			Name:     TestTaskListName,
			TaskType: int(types.TaskListTypeDecision),
		},
		TaskID:     12,
		DomainName: TestDomainName,
	})

	assert.NoError(t, err)
}

func TestCompleteTasksLessThan(t *testing.T) {
	store, db := setupNoSQLStoreMocks(t)

	// The main assertion is that the correct parameters are passed to the DB
	db.EXPECT().RangeDeleteTasks(gomock.Any(), &nosqlplugin.TasksFilter{
		TaskListFilter: *getDecisionTaskListFilter(),
		MinTaskID:      0,
		MaxTaskID:      12,
		BatchSize:      100,
	}).Return(13, nil)

	resp, err := store.CompleteTasksLessThan(context.Background(), &persistence.CompleteTasksLessThanRequest{
		DomainID:     TestDomainID,
		TaskListName: TestTaskListName,
		TaskType:     0,
		TaskID:       12,
		Limit:        100,
		DomainName:   TestDomainName,
	})

	assert.NoError(t, err)
	assert.Equal(t, 13, resp.TasksCompleted)
}

func TestCreateTasks(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name          string
		setupMock     func(*nosqlplugin.MockDB)
		request       *persistence.CreateTasksRequest
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertTasks(gomock.Any(), gomock.Any(), &nosqlplugin.TaskListRow{
					DomainID:     TestDomainID,
					TaskListName: TestTaskListName,
					TaskListType: TestTaskType,
					RangeID:      1,
				}).Do(func(_ context.Context, tasks []*nosqlplugin.TaskRowForInsert, _ *nosqlplugin.TaskListRow) {
					assert.Len(t, tasks, 1)
					assert.Equal(t, TestDomainID, tasks[0].DomainID)
					assert.Equal(t, "workflow1", tasks[0].WorkflowID)
					assert.Equal(t, "run1", tasks[0].RunID)
					assert.Equal(t, int64(100), tasks[0].TaskID)
					assert.Equal(t, int64(10), tasks[0].ScheduledID)
					assert.Equal(t, TestTaskType, tasks[0].TaskListType)
					assert.Equal(t, TestTaskListName, tasks[0].TaskListName)
					assert.Equal(t, 30, tasks[0].TTLSeconds)
				}).Return(nil).Times(1)
			},
			request: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: TestDomainID,
					Name:     TestTaskListName,
					TaskType: TestTaskType,
					RangeID:  1,
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						TaskID: 100,
						Data: &persistence.TaskInfo{
							WorkflowID:                    "workflow1",
							RunID:                         "run1",
							ScheduleID:                    10,
							PartitionConfig:               nil,
							ScheduleToStartTimeoutSeconds: 30,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "success - adding task with Expiry not expired",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertTasks(gomock.Any(), gomock.Any(), &nosqlplugin.TaskListRow{
					DomainID:     TestDomainID,
					TaskListName: TestTaskListName,
					TaskListType: TestTaskType,
					RangeID:      1,
				}).Do(func(_ context.Context, tasks []*nosqlplugin.TaskRowForInsert, _ *nosqlplugin.TaskListRow) {
					assert.Len(t, tasks, 1)
					assert.Equal(t, TestDomainID, tasks[0].DomainID)
					assert.Equal(t, "workflow1", tasks[0].WorkflowID)
					assert.Equal(t, "run1", tasks[0].RunID)
					assert.Equal(t, int64(100), tasks[0].TaskID)
					assert.Equal(t, int64(10), tasks[0].ScheduledID)
					assert.Equal(t, TestTaskType, tasks[0].TaskListType)
					assert.Equal(t, TestTaskListName, tasks[0].TaskListName)
					assert.Equal(t, int(now.Add(100*time.Second).Sub(tasks[0].CreatedTime).Seconds()), tasks[0].TTLSeconds)
				}).Return(nil).Times(1)
			},
			request: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: TestDomainID,
					Name:     TestTaskListName,
					TaskType: TestTaskType,
					RangeID:  1,
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						TaskID: 100,
						Data: &persistence.TaskInfo{
							WorkflowID:      "workflow1",
							RunID:           "run1",
							ScheduleID:      10,
							PartitionConfig: nil,
							Expiry:          now.Add(100 * time.Second),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "success - skipping task with Expiry expired",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertTasks(gomock.Any(), gomock.Any(), &nosqlplugin.TaskListRow{
					DomainID:     TestDomainID,
					TaskListName: TestTaskListName,
					TaskListType: TestTaskType,
					RangeID:      1,
				}).Do(func(_ context.Context, tasks []*nosqlplugin.TaskRowForInsert, _ *nosqlplugin.TaskListRow) {
					assert.Len(t, tasks, 0)
				}).Return(nil).Times(1)
			},
			request: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: TestDomainID,
					Name:     TestTaskListName,
					TaskType: TestTaskType,
					RangeID:  1,
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						TaskID: 100,
						Data: &persistence.TaskInfo{
							WorkflowID:      "workflow1",
							RunID:           "run1",
							ScheduleID:      10,
							PartitionConfig: nil,
							Expiry:          now,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "condition failure",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertTasks(gomock.Any(), gomock.Any(), gomock.Any()).Return(&nosqlplugin.TaskOperationConditionFailure{
					Details: "rangeID mismatch",
				}).Times(1)
			},
			request: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: TestDomainID,
					Name:     TestTaskListName,
					TaskType: TestTaskType,
					RangeID:  1,
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						TaskID: 100,
						Data: &persistence.TaskInfo{
							WorkflowID:                    "workflow1",
							RunID:                         "run1",
							ScheduleID:                    10,
							PartitionConfig:               nil,
							ScheduleToStartTimeoutSeconds: 30,
						},
					},
				},
			},
			expectError:   true,
			expectedError: "Failed to insert tasks. name: test-tasklist, type: 0, rangeID: 1, columns: (rangeID mismatch)",
		},
		{
			name: "generic db error",
			setupMock: func(dbMock *nosqlplugin.MockDB) {
				dbMock.EXPECT().InsertTasks(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("db error")).Times(1)
				dbMock.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			request: &persistence.CreateTasksRequest{
				TaskListInfo: &persistence.TaskListInfo{
					DomainID: TestDomainID,
					Name:     TestTaskListName,
					TaskType: TestTaskType,
					RangeID:  1,
				},
				Tasks: []*persistence.CreateTaskInfo{
					{
						TaskID: 100,
						Data: &persistence.TaskInfo{
							WorkflowID:                    "workflow1",
							RunID:                         "run1",
							ScheduleID:                    10,
							PartitionConfig:               nil,
							ScheduleToStartTimeoutSeconds: 30,
						},
					},
				},
			},
			expectError:   true,
			expectedError: "CreateTasks failed. Error: db error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mocks for each test case individually
			store, dbMock := setupNoSQLStoreMocks(t)

			// Setup test-specific mock behavior
			tc.setupMock(dbMock)

			// Execute the method under test
			resp, err := store.CreateTasks(context.Background(), tc.request)

			// Validate results
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func getValidLeaseTaskListRequest() *persistence.LeaseTaskListRequest {
	return &persistence.LeaseTaskListRequest{
		DomainID:     TestDomainID,
		DomainName:   TestDomainName,
		TaskList:     TestTaskListName,
		TaskType:     int(types.TaskListTypeDecision),
		TaskListKind: int(types.TaskListKindNormal),
		RangeID:      0,
	}
}

func getValidGetTaskListRequest() *persistence.GetTaskListRequest {
	return &persistence.GetTaskListRequest{
		DomainID:   TestDomainID,
		DomainName: TestDomainName,
		TaskList:   TestTaskListName,
		TaskType:   int(types.TaskListTypeDecision),
	}
}

func checkTaskListInfoExpected(t *testing.T, taskListInfo *persistence.TaskListInfo) {
	assert.Equal(t, TestDomainID, taskListInfo.DomainID)
	assert.Equal(t, TestTaskListName, taskListInfo.Name)
	assert.Equal(t, int(types.TaskListTypeDecision), taskListInfo.TaskType)
	assert.Equal(t, initialRangeID, taskListInfo.RangeID)
	assert.Equal(t, initialAckLevel, taskListInfo.AckLevel)
	assert.Equal(t, int(types.TaskListKindNormal), taskListInfo.Kind)
	assert.WithinDuration(t, time.Now(), taskListInfo.LastUpdated, time.Second)
}

func taskRowEqualTaskInfo(t *testing.T, taskrow1 nosqlplugin.TaskRow, taskInfo1 *persistence.TaskInfo) {
	assert.Equal(t, taskrow1.DomainID, taskInfo1.DomainID)
	assert.Equal(t, taskrow1.WorkflowID, taskInfo1.WorkflowID)
	assert.Equal(t, taskrow1.RunID, taskInfo1.RunID)
	assert.Equal(t, taskrow1.TaskID, taskInfo1.TaskID)
	assert.Equal(t, taskrow1.ScheduledID, taskInfo1.ScheduleID)
	assert.Equal(t, taskrow1.CreatedTime, taskInfo1.CreatedTime)
	assert.Equal(t, taskrow1.PartitionConfig, taskInfo1.PartitionConfig)
}

func getValidGetTasksRequest() *persistence.GetTasksRequest {
	return &persistence.GetTasksRequest{
		DomainID:   TestDomainID,
		DomainName: TestDomainName,
		TaskList:   TestTaskListName,
		TaskType:   int(types.TaskListTypeDecision),
		// The read level is the smallest taskID that we want to read, the maxReadLevel is the largest
		ReadLevel:    1,
		MaxReadLevel: common.Int64Ptr(15),
		BatchSize:    100,
	}
}

func getDecisionTaskListFilter() *nosqlplugin.TaskListFilter {
	return &nosqlplugin.TaskListFilter{
		DomainID:     TestDomainID,
		TaskListName: TestTaskListName,
		TaskListType: int(types.TaskListTypeDecision),
	}
}

func getExpectedTaskListRow() *nosqlplugin.TaskListRow {
	return &nosqlplugin.TaskListRow{
		DomainID:        TestDomainID,
		TaskListName:    TestTaskListName,
		TaskListType:    int(types.TaskListTypeDecision),
		RangeID:         initialRangeID,
		TaskListKind:    int(types.TaskListKindNormal),
		AckLevel:        initialAckLevel,
		LastUpdatedTime: time.Now(),
	}
}

func getExpectedTaskListRowWithPartitionConfig() *nosqlplugin.TaskListRow {
	return &nosqlplugin.TaskListRow{
		DomainID:        TestDomainID,
		TaskListName:    TestTaskListName,
		TaskListType:    int(types.TaskListTypeDecision),
		RangeID:         initialRangeID,
		TaskListKind:    int(types.TaskListKindNormal),
		AckLevel:        initialAckLevel,
		LastUpdatedTime: time.Now(),
		AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
			Version: 1,
			ReadPartitions: map[int]*persistence.TaskListPartition{
				0: {},
				1: {},
				2: {
					IsolationGroups: []string{"foo"},
				},
			},
			WritePartitions: map[int]*persistence.TaskListPartition{
				0: {},
				1: {
					IsolationGroups: []string{"bar"},
				},
			},
		},
	}
}

func checkTaskListRowExpected(t *testing.T, expectedRow *nosqlplugin.TaskListRow, taskList *nosqlplugin.TaskListRow) {
	// Check the duration
	assert.WithinDuration(t, expectedRow.LastUpdatedTime, taskList.LastUpdatedTime, time.Second)

	// Set the expected time to the actual time to make the comparison work
	expectedRow.LastUpdatedTime = taskList.LastUpdatedTime
	assert.Equal(t, expectedRow, taskList)
}

func getExpectedTaskListInfo() *persistence.TaskListInfo {
	return &persistence.TaskListInfo{
		DomainID:    TestDomainID,
		Name:        TestTaskListName,
		TaskType:    int(types.TaskListTypeDecision),
		RangeID:     initialRangeID,
		AckLevel:    initialAckLevel,
		Kind:        int(types.TaskListKindNormal),
		LastUpdated: time.Now(),
		AdaptivePartitionConfig: &persistence.TaskListPartitionConfig{
			Version: 1,
			ReadPartitions: map[int]*persistence.TaskListPartition{
				0: {},
				1: {},
				2: {
					IsolationGroups: []string{"foo"},
				},
			},
			WritePartitions: map[int]*persistence.TaskListPartition{
				0: {},
				1: {
					IsolationGroups: []string{"bar"},
				},
			},
		},
	}
}

func getValidDeleteTaskListRequest() *persistence.DeleteTaskListRequest {
	return &persistence.DeleteTaskListRequest{
		DomainID:     TestDomainID,
		DomainName:   TestDomainName,
		TaskListName: TestTaskListName,
		TaskListType: int(types.TaskListTypeDecision),
		RangeID:      0,
	}
}
