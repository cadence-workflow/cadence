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
	"github.com/uber/cadence/common/checksum"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/serialization"
	"github.com/uber/cadence/common/types"
)

var FixedTime = time.Date(2025, 1, 6, 15, 0, 0, 0, time.UTC)

func newTestNosqlExecutionStoreWithTaskSerializer(db nosqlplugin.DB, logger log.Logger, taskSerializer serialization.TaskSerializer) *nosqlExecutionStore {
	return &nosqlExecutionStore{
		shardID:        1,
		nosqlStore:     nosqlStore{logger: logger, db: db, dc: &persistence.DynamicConfiguration{EnableHistoryTaskDualWriteMode: func(...dynamicproperties.FilterOption) bool { return true }}},
		taskSerializer: taskSerializer,
	}
}

func TestNosqlExecutionStoreUtils(t *testing.T) {
	testCases := []struct {
		name       string
		setupStore func(*nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error)
		input      *persistence.InternalWorkflowSnapshot
		validate   func(*testing.T, *nosqlplugin.WorkflowExecutionRequest, error)
	}{
		{
			name: "PrepareCreateWorkflowExecutionRequestWithMaps - Success",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				workflowSnapshot := &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					VersionHistories: &persistence.DataBlob{
						Encoding: constants.EncodingTypeJSON,
						Data:     []byte(`[{"Branches":[{"BranchID":"test-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
					},
				}
				return store.prepareCreateWorkflowExecutionRequestWithMaps(workflowSnapshot, FixedTime)
			},
			input: &persistence.InternalWorkflowSnapshot{},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				if err == nil {
					assert.NotNil(t, req)
				}
			},
		},
		{
			name: "PrepareCreateWorkflowExecutionRequestWithMaps - Nil Checksum",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				workflowSnapshot := &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					VersionHistories: &persistence.DataBlob{
						Encoding: constants.EncodingTypeJSON,
						Data:     []byte(`[{"Branches":[{"BranchID":"test-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
					},
					Checksum: checksum.Checksum{Value: nil},
				}
				return store.prepareCreateWorkflowExecutionRequestWithMaps(workflowSnapshot, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req.Checksums)
			},
		},

		{
			name: "PrepareCreateWorkflowExecutionRequestWithMaps - Empty VersionHistories",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				// Testing with an empty VersionHistories (which previously caused an error)
				workflowSnapshot := &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "test-domain-id-2",
						WorkflowID: "test-workflow-id-2",
						RunID:      "test-run-id-2",
					},
					VersionHistories: &persistence.DataBlob{
						Encoding: constants.EncodingTypeJSON,
						Data:     []byte("[]"), // Empty VersionHistories
					},
				}
				return store.prepareCreateWorkflowExecutionRequestWithMaps(workflowSnapshot, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req.VersionHistories)
				assert.Equal(t, "[]", string(req.VersionHistories.Data))
			},
		},
		{
			name: "PrepareResetWorkflowExecutionRequestWithMapsAndEventBuffer - Success",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				resetWorkflow := &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "reset-domain-id",
						WorkflowID: "reset-workflow-id",
						RunID:      "reset-run-id",
					},
					LastWriteVersion: 123,
					Checksum:         checksum.Checksum{Version: 1},
					VersionHistories: &persistence.DataBlob{Encoding: constants.EncodingTypeJSON, Data: []byte(`[{"Branches":[{"BranchID":"reset-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`)},
					ActivityInfos:    []*persistence.InternalActivityInfo{{ScheduleID: 1}},
					TimerInfos:       []*persistence.TimerInfo{{TimerID: "timerID"}},
					ChildExecutionInfos: []*persistence.InternalChildExecutionInfo{
						{InitiatedID: 1, StartedID: 2},
					},
					RequestCancelInfos: []*persistence.RequestCancelInfo{{InitiatedID: 1}},
					SignalInfos:        []*persistence.SignalInfo{{InitiatedID: 1}},
					SignalRequestedIDs: []string{"signalRequestedID"},
					Condition:          999,
				}
				return store.prepareResetWorkflowExecutionRequestWithMapsAndEventBuffer(resetWorkflow, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req)
				assert.Equal(t, nosqlplugin.WorkflowExecutionMapsWriteModeReset, req.MapsWriteMode)
				assert.Equal(t, nosqlplugin.EventBufferWriteModeClear, req.EventBufferWriteMode)
				assert.Equal(t, int64(999), *req.PreviousNextEventIDCondition)
			},
		},
		{
			name: "PrepareResetWorkflowExecutionRequestWithMapsAndEventBuffer - Malformed VersionHistories",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				resetWorkflow := &persistence.InternalWorkflowSnapshot{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "domain-id-malformed-vh",
						WorkflowID: "workflow-id-malformed-vh",
						RunID:      "run-id-malformed-vh",
					},
					LastWriteVersion: 456,
					Checksum:         checksum.Checksum{Version: 1},
					VersionHistories: &persistence.DataBlob{Encoding: constants.EncodingTypeJSON, Data: []byte("{malformed}")},
				}
				return store.prepareResetWorkflowExecutionRequestWithMapsAndEventBuffer(resetWorkflow, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req)
			},
		},
		{
			name: "PrepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer - Successful Update Request Preparation",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				workflowMutation := &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{
						DomainID:   "domainID-success",
						WorkflowID: "workflowID-success",
						RunID:      "runID-success",
					},
				}
				return store.prepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer(workflowMutation, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req)
			},
		},
		{
			name: "PrepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer - Incomplete WorkflowMutation",
			setupStore: func(store *nosqlExecutionStore) (*nosqlplugin.WorkflowExecutionRequest, error) {
				workflowMutation := &persistence.InternalWorkflowMutation{
					ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{ // Partially populated for the test
						DomainID: "domainID-incomplete",
					},
				}
				return store.prepareUpdateWorkflowExecutionRequestWithMapsAndEventBuffer(workflowMutation, FixedTime)
			},
			validate: func(t *testing.T, req *nosqlplugin.WorkflowExecutionRequest, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, req)
				assert.Equal(t, "domainID-incomplete", req.DomainID) // Example assertion
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(mockCtrl)
			store := newTestNosqlExecutionStore(mockDB, log.NewNoop())

			req, err := tc.setupStore(store)
			tc.validate(t, req, err)
		})
	}

}

func TestPrepareTasksForWorkflowTxn(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func(*serialization.MockTaskSerializer)
		setupStore func(*nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error)
		validate   func(*testing.T, []*nosqlplugin.HistoryMigrationTask, error)
	}{{
		name: "PrepareTimerTasksForWorkflowTxn - Successful Timer Tasks Preparation",
		setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
			mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).
				Return(persistence.DataBlob{
					Data:     []byte("timer"),
					Encoding: constants.EncodingTypeThriftRW,
				}, nil)
		},
		setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
			timerTasks := []persistence.Task{
				&persistence.DecisionTimeoutTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(), TaskID: 1,
					},
					EventID: 2, TimeoutType: 1, ScheduleAttempt: 1},
			}
			tasks, err := store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", timerTasks)
			assert.NoError(t, err)
			assert.NotEmpty(t, tasks)
			return nil, err
		},
		validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {},
	},
		{
			name:       "PrepareTimerTasksForWorkflowTxn - Unsupported Timer Task Type",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				timerTasks := []persistence.Task{
					&dummyTaskType{
						VisibilityTimestamp: time.Now(),
						TaskID:              1,
					},
				}
				return store.prepareTimerTasksForWorkflowTxn("domainID-unsupported", "workflowID-unsupported", "runID-unsupported", timerTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			},
		},
		{
			name:       "PrepareTimerTasksForWorkflowTxn - Zero Tasks",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				return store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", []persistence.Task{})
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Empty(t, tasks)
			},
		},
		{
			name: "PrepareTimerTasksForWorkflowTxn - ActivityTimeoutTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("timer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				timerTasks := []persistence.Task{
					&persistence.ActivityTimeoutTask{
						TaskData: persistence.TaskData{
							Version:             1,
							TaskID:              2,
							VisibilityTimestamp: time.Now(),
						},
						EventID: 3,
						Attempt: 2,
					},
				}
				return store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", timerTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				assert.Equal(t, int64(3), tasks[0].Timer.EventID)
				assert.Equal(t, int64(2), tasks[0].Timer.ScheduleAttempt)
				assert.Equal(t, []byte("timer"), tasks[0].Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, tasks[0].Task.Encoding)
			},
		},
		{
			name: "PrepareTimerTasksForWorkflowTxn - UserTimerTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("timer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				timerTasks := []persistence.Task{
					&persistence.UserTimerTask{
						TaskData: persistence.TaskData{
							Version:             1,
							TaskID:              3,
							VisibilityTimestamp: time.Now(),
						},
						EventID: 4,
					},
				}
				return store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", timerTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				assert.Equal(t, int64(4), tasks[0].Timer.EventID)
				assert.Equal(t, []byte("timer"), tasks[0].Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, tasks[0].Task.Encoding)
			},
		},
		{
			name: "PrepareTimerTasksForWorkflowTxn - ActivityRetryTimerTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("timer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				timerTasks := []persistence.Task{
					&persistence.ActivityRetryTimerTask{
						TaskData: persistence.TaskData{
							Version:             1,
							TaskID:              4,
							VisibilityTimestamp: time.Now(),
						},
						EventID: 5,
						Attempt: 3,
					},
				}
				return store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", timerTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				assert.Equal(t, int64(5), tasks[0].Timer.EventID)
				assert.Equal(t, int64(3), tasks[0].Timer.ScheduleAttempt)
				assert.Equal(t, []byte("timer"), tasks[0].Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, tasks[0].Task.Encoding)
			},
		},
		{
			name: "PrepareTimerTasksForWorkflowTxn - WorkflowBackoffTimerTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTimer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("timer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				timerTasks := []persistence.Task{
					&persistence.WorkflowBackoffTimerTask{
						TaskData: persistence.TaskData{
							Version:             1,
							TaskID:              5,
							VisibilityTimestamp: time.Now(),
						},
					},
				}
				return store.prepareTimerTasksForWorkflowTxn("domainID", "workflowID", "runID", timerTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				assert.Equal(t, []byte("timer"), tasks[0].Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, tasks[0].Task.Encoding)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(mockCtrl)
			mockTaskSerializer := serialization.NewMockTaskSerializer(mockCtrl)
			store := newTestNosqlExecutionStoreWithTaskSerializer(mockDB, log.NewNoop(), mockTaskSerializer)
			tc.setupMocks(mockTaskSerializer)

			tasks, err := tc.setupStore(store)
			tc.validate(t, tasks, err)
		})
	}
}

func TestPrepareReplicationTasksForWorkflowTxn(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func(*serialization.MockTaskSerializer)
		setupStore func(*nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error)
		validate   func(*testing.T, []*nosqlplugin.HistoryMigrationTask, error)
	}{
		{
			name: "Successful Replication Tasks Preparation",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("replication"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				replicationTasks := []persistence.Task{
					&persistence.HistoryReplicationTask{
						TaskData: persistence.TaskData{
							Version: 1,
						},
					},
				}
				return store.prepareReplicationTasksForWorkflowTxn("domainID", "workflowID", "runID", replicationTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, tasks)
			},
		},
		{
			name:       "Handling Unknown Replication Task Type",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				replicationTasks := []persistence.Task{
					&dummyTaskType{
						VisibilityTimestamp: time.Now(),
						TaskID:              -1,
					},
				}
				return store.prepareReplicationTasksForWorkflowTxn("domainID", "workflowID", "runID", replicationTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			},
		},
		{
			name: "PrepareReplicationTasksForWorkflowTxn - SyncActivityTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("replication"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				replicationTasks := []persistence.Task{
					&persistence.SyncActivityTask{
						TaskData: persistence.TaskData{
							Version:             2,
							VisibilityTimestamp: time.Now(),
							TaskID:              2,
						},
						ScheduledID: 123,
					},
				}
				return store.prepareReplicationTasksForWorkflowTxn("domainID", "workflowID", "runID", replicationTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, persistence.ReplicationTaskTypeSyncActivity, task.Replication.TaskType)
				assert.Equal(t, int64(123), task.Replication.ScheduledID)
				assert.Equal(t, []byte("replication"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name: "PrepareReplicationTasksForWorkflowTxn - FailoverMarkerTask",
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryReplication, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("replication"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			setupStore: func(store *nosqlExecutionStore) ([]*nosqlplugin.HistoryMigrationTask, error) {
				replicationTasks := []persistence.Task{
					&persistence.FailoverMarkerTask{
						TaskData: persistence.TaskData{
							Version:             3,
							VisibilityTimestamp: time.Now(),
							TaskID:              3,
						},
						DomainID: "domainID",
					},
				}
				return store.prepareReplicationTasksForWorkflowTxn("domainID", "workflowID", "runID", replicationTasks)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, persistence.ReplicationTaskTypeFailoverMarker, task.Replication.TaskType)
				assert.Equal(t, "domainID", task.Replication.DomainID)
				assert.Equal(t, []byte("replication"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(mockCtrl)
			mockTaskSerializer := serialization.NewMockTaskSerializer(mockCtrl)
			store := newTestNosqlExecutionStoreWithTaskSerializer(mockDB, log.NewNoop(), mockTaskSerializer)
			tc.setupMocks(mockTaskSerializer)
			tasks, err := tc.setupStore(store)
			tc.validate(t, tasks, err)
		})
	}
}

func TestPrepareTransferTasksForWorkflowTxn(t *testing.T) {
	testCases := []struct {
		name       string
		tasks      []persistence.Task
		domainID   string
		workflowID string
		runID      string
		setupMocks func(*serialization.MockTaskSerializer)
		validate   func(*testing.T, []*nosqlplugin.HistoryMigrationTask, error)
	}{
		{
			name:       "CancelExecutionTask - Success",
			domainID:   "domainID-cancel",
			workflowID: "workflowID-cancel",
			runID:      "runID-cancel",
			tasks: []persistence.Task{
				&persistence.CancelExecutionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1002,
						Version:             1,
					},
					TargetDomainID:          "targetDomainID-cancel",
					TargetWorkflowID:        "targetWorkflowID-cancel",
					TargetRunID:             "targetRunID-cancel",
					TargetChildWorkflowOnly: true,
					InitiatedID:             1002,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, "targetDomainID-cancel", task.Transfer.TargetDomainID)
				assert.Equal(t, true, task.Transfer.TargetChildWorkflowOnly)
				assert.Equal(t, int64(1002), task.Transfer.TaskID)
				assert.Equal(t, int64(1), task.Transfer.Version)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "ActivityTask - Success",
			domainID:   "domainID-activity",
			workflowID: "workflowID-activity",
			runID:      "runID-activity",
			tasks: []persistence.Task{
				&persistence.ActivityTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1001,
						Version:             1,
					},
					TargetDomainID: "targetDomainID-activity",
					TaskList:       "taskList-activity",
					ScheduleID:     1001,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, persistence.TransferTaskTypeActivityTask, task.Transfer.TaskType)
				assert.Equal(t, "targetDomainID-activity", task.Transfer.TargetDomainID)
				assert.Equal(t, "taskList-activity", task.Transfer.TaskList)
				assert.Equal(t, int64(1001), task.Transfer.ScheduleID)
				assert.Equal(t, int64(1), task.Transfer.Version)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "DefaultTargetRunID - When Empty",
			domainID:   "domainID-default-runid",
			workflowID: "workflowID-default-runid",
			runID:      "runID-default-runid",
			tasks: []persistence.Task{
				&persistence.CancelExecutionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              2001,
						Version:             1,
					},
					TargetDomainID:          "targetDomainID-cancel",
					TargetWorkflowID:        "targetWorkflowID-cancel",
					TargetRunID:             "", // Intentionally left empty to trigger the defaulting logic
					TargetChildWorkflowOnly: true,
					InitiatedID:             2001,
				},
				&persistence.SignalExecutionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              2002,
						Version:             1,
					},
					TargetDomainID:          "targetDomainID-signal",
					TargetWorkflowID:        "targetWorkflowID-signal",
					TargetRunID:             "", // Intentionally left empty to trigger the defaulting logic
					TargetChildWorkflowOnly: false,
					InitiatedID:             2002,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil).Times(2)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				for _, task := range tasks {
					assert.Equal(t, persistence.TransferTaskTransferTargetRunID, task.Transfer.TargetRunID, "TargetRunID should default to TransferTaskTransferTargetRunID")
					assert.Equal(t, []byte("transfer"), task.Task.Data)
					assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
				}
			},
		},
		{
			name:       "SignalExecutionTask - Success",
			domainID:   "domainID-signal",
			workflowID: "workflowID-signal",
			runID:      "runID-signal",
			tasks: []persistence.Task{
				&persistence.SignalExecutionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1003,
						Version:             1,
					},
					TargetDomainID:          "targetDomainID-signal",
					TargetWorkflowID:        "targetWorkflowID-signal",
					TargetRunID:             "targetRunID-signal",
					TargetChildWorkflowOnly: true,
					InitiatedID:             1003,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, "targetDomainID-signal", task.Transfer.TargetDomainID)
				assert.Equal(t, true, task.Transfer.TargetChildWorkflowOnly)
				assert.Equal(t, int64(1003), task.Transfer.TaskID)
				assert.Equal(t, int64(1), task.Transfer.Version)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "StartChildExecutionTask - Success",
			domainID:   "domainID-start-child",
			workflowID: "workflowID-start-child",
			runID:      "runID-start-child",
			tasks: []persistence.Task{
				&persistence.StartChildExecutionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1004,
						Version:             1,
					},
					TargetDomainID:   "child-execution-domain-id",
					TargetWorkflowID: "child-workflow-id",
					InitiatedID:      1004,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, "child-execution-domain-id", task.Transfer.TargetDomainID)
				assert.Equal(t, "child-workflow-id", task.Transfer.TargetWorkflowID)
				assert.Equal(t, int64(1004), task.Transfer.TaskID)
				assert.Equal(t, int64(1), task.Transfer.Version)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "RecordChildExecutionCompletedTask - Success",
			domainID:   "domainID-record-child",
			workflowID: "workflowID-record-child",
			runID:      "runID-record-child",
			tasks: []persistence.Task{
				&persistence.RecordChildExecutionCompletedTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1005,
						Version:             1,
					},
					TargetDomainID:   "completed-child-domain-id",
					TargetWorkflowID: "completed-child-workflow-id",
					TargetRunID:      "completed-child-run-id",
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, "completed-child-domain-id", task.Transfer.TargetDomainID)
				assert.Equal(t, "completed-child-workflow-id", task.Transfer.TargetWorkflowID)
				assert.Equal(t, "completed-child-run-id", task.Transfer.TargetRunID)
				assert.Equal(t, int64(1005), task.Transfer.TaskID)
				assert.Equal(t, int64(1), task.Transfer.Version)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "DecisionTask - Success",
			domainID:   "domainID-decision",
			workflowID: "workflowID-decision",
			runID:      "runID-decision",
			tasks: []persistence.Task{
				&persistence.DecisionTask{
					TaskData: persistence.TaskData{
						VisibilityTimestamp: time.Now(),
						TaskID:              1001,
						Version:             1,
					},
					TargetDomainID: "targetDomainID-decision",
					TaskList:       "taskList-decision",
					ScheduleID:     1001,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {
				mockTaskSerializer.EXPECT().SerializeTask(persistence.HistoryTaskCategoryTransfer, gomock.Any()).
					Return(persistence.DataBlob{
						Data:     []byte("transfer"),
						Encoding: constants.EncodingTypeThriftRW,
					}, nil)
			},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.NoError(t, err)
				assert.Len(t, tasks, 1)
				task := tasks[0]
				assert.Equal(t, int64(1001), task.Transfer.TaskID)
				assert.Equal(t, "targetDomainID-decision", task.Transfer.TargetDomainID)
				assert.Equal(t, false, task.Transfer.RecordVisibility)
				assert.Equal(t, []byte("transfer"), task.Task.Data)
				assert.Equal(t, constants.EncodingTypeThriftRW, task.Task.Encoding)
			},
		},
		{
			name:       "Unsupported Task Type",
			domainID:   "domainID-unsupported",
			workflowID: "workflowID-unsupported",
			runID:      "runID-unsupported",
			tasks: []persistence.Task{
				&dummyTaskType{
					VisibilityTimestamp: time.Now(),
					TaskID:              9999,
				},
			},
			setupMocks: func(mockTaskSerializer *serialization.MockTaskSerializer) {},
			validate: func(t *testing.T, tasks []*nosqlplugin.HistoryMigrationTask, err error) {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(mockCtrl)
			mockTaskSerializer := serialization.NewMockTaskSerializer(mockCtrl)
			store := newTestNosqlExecutionStoreWithTaskSerializer(mockDB, log.NewNoop(), mockTaskSerializer)
			tc.setupMocks(mockTaskSerializer)

			tasks, err := store.prepareTransferTasksForWorkflowTxn(tc.domainID, tc.workflowID, tc.runID, tc.tasks)
			tc.validate(t, tasks, err)
		})
	}
}

func TestNosqlExecutionStoreUtilsExtended(t *testing.T) {
	testCases := []struct {
		name       string
		setupStore func(store *nosqlExecutionStore) (interface{}, error)
		validate   func(t *testing.T, result interface{}, err error)
	}{
		{
			name: "PrepareActivityInfosForWorkflowTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				activityInfos := []*persistence.InternalActivityInfo{
					{
						ScheduleID:     1,
						ScheduledEvent: persistence.NewDataBlob([]byte("scheduled event data"), constants.EncodingTypeThriftRW),
						StartedEvent:   persistence.NewDataBlob([]byte("started event data"), constants.EncodingTypeThriftRW),
					},
				}
				return store.prepareActivityInfosForWorkflowTxn(activityInfos)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				infos, ok := result.(map[int64]*persistence.InternalActivityInfo)
				assert.True(t, ok)
				assert.Len(t, infos, 1)
				for _, info := range infos {
					assert.NotNil(t, info.ScheduledEvent)
					assert.NotNil(t, info.StartedEvent)
				}
			},
		},
		{
			name: "PrepareTimerInfosForWorkflowTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				timerInfos := []*persistence.TimerInfo{
					{
						TimerID: "timer1",
					},
				}
				return store.prepareTimerInfosForWorkflowTxn(timerInfos)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				infos, ok := result.(map[string]*persistence.TimerInfo)
				assert.True(t, ok)
				assert.Len(t, infos, 1)
				assert.NotNil(t, infos["timer1"])
			},
		},
		{
			name: "PrepareChildWFInfosForWorkflowTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				childWFInfos := []*persistence.InternalChildExecutionInfo{
					{
						InitiatedID:    1,
						InitiatedEvent: persistence.NewDataBlob([]byte("initiated event data"), constants.EncodingTypeThriftRW),
						StartedEvent:   persistence.NewDataBlob([]byte("started event data"), constants.EncodingTypeThriftRW),
					},
				}
				return store.prepareChildWFInfosForWorkflowTxn(childWFInfos)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				infos, ok := result.(map[int64]*persistence.InternalChildExecutionInfo)
				assert.True(t, ok)
				assert.Len(t, infos, 1)
				for _, info := range infos {
					assert.NotNil(t, info.InitiatedEvent)
					assert.NotNil(t, info.StartedEvent)
				}
			},
		},
		{
			name: "PrepareTimerInfosForWorkflowTxn - Nil Timer Info",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				return store.prepareTimerInfosForWorkflowTxn(nil)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Empty(t, result)
			},
		},
		{
			name: "PrepareChildWFInfosForWorkflowTxn - Nil Child Execution Info",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				return store.prepareChildWFInfosForWorkflowTxn(nil)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				assert.Empty(t, result)
			},
		},
		{
			name: "PrepareChildWFInfosForWorkflowTxn - Encoding Mismatch Error",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				childWFInfos := []*persistence.InternalChildExecutionInfo{
					{
						InitiatedID:    1,
						InitiatedEvent: persistence.NewDataBlob([]byte("initiated"), constants.EncodingTypeThriftRW),
						StartedEvent:   persistence.NewDataBlob([]byte("started"), constants.EncodingTypeJSON), // Encoding mismatch
					},
				}
				return store.prepareChildWFInfosForWorkflowTxn(childWFInfos)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		},
		{
			name: "PrepareRequestCancelsForWorkflowTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				requestCancels := []*persistence.RequestCancelInfo{
					{
						InitiatedID:     1,
						CancelRequestID: "cancel-1",
					},
					{
						InitiatedID:     2,
						CancelRequestID: "cancel-2",
					},
				}
				cancels, err := store.prepareRequestCancelsForWorkflowTxn(requestCancels)
				return cancels, err
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				cancels := result.(map[int64]*persistence.RequestCancelInfo)
				assert.Equal(t, 2, len(cancels))
				assert.Contains(t, cancels, int64(1))
				assert.Contains(t, cancels, int64(2))
			},
		},
		{
			name: "PrepareRequestCancelsForWorkflowTxn - Duplicate Initiated IDs",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				requestCancels := []*persistence.RequestCancelInfo{
					{
						InitiatedID:     1,
						CancelRequestID: "cancel-1",
					},
					{
						InitiatedID:     1, // Duplicate InitiatedID
						CancelRequestID: "cancel-1-duplicate",
					},
				}
				cancels, err := store.prepareRequestCancelsForWorkflowTxn(requestCancels)
				return cancels, err
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				cancels := result.(map[int64]*persistence.RequestCancelInfo)
				assert.Equal(t, 1, len(cancels))
				assert.Equal(t, "cancel-1-duplicate", cancels[1].CancelRequestID)
			},
		},
		{
			name: "PrepareSignalInfosForWorkflowTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				signalInfos := []*persistence.SignalInfo{
					{InitiatedID: 1, SignalRequestID: "signal-1"},
					{InitiatedID: 2, SignalRequestID: "signal-2"},
				}
				return store.prepareSignalInfosForWorkflowTxn(signalInfos)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				infos := result.(map[int64]*persistence.SignalInfo)
				assert.Equal(t, 2, len(infos))
				assert.Equal(t, "signal-1", infos[1].SignalRequestID)
				assert.Equal(t, "signal-2", infos[2].SignalRequestID)
			},
		},
		{
			name: "PrepareUpdateWorkflowExecutionTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "test-domain-id",
					WorkflowID:  "test-workflow-id",
					RunID:       "test-run-id",
					State:       persistence.WorkflowStateRunning,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				versionHistories := &persistence.DataBlob{
					Encoding: constants.EncodingTypeJSON,
					Data:     []byte(`[{"Branches":[{"BranchID":"test-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
				}
				checksum := checksum.Checksum{Version: 1,
					Value: []byte("create-checksum")}
				return store.prepareUpdateWorkflowExecutionTxn(executionInfo, versionHistories, checksum, time.Now(), 123)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				req := result.(*nosqlplugin.WorkflowExecutionRequest)
				assert.Equal(t, "test-domain-id", req.DomainID)
				assert.Equal(t, int64(123), req.LastWriteVersion)
			},
		},
		{
			name: "PrepareUpdateWorkflowExecutionTxn - Emptyvalues",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					DomainID:   "",
					WorkflowID: "",
					State:      persistence.WorkflowStateCompleted,
				}
				versionHistories := &persistence.DataBlob{
					Encoding: constants.EncodingTypeJSON,
					Data:     []byte(`[{"Branches":[{"BranchID":"branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
				}
				checksum := checksum.Checksum{Version: 1, Value: []byte("checksum")}
				// This should result in an error due to invalid executionInfo state for the creation scenario
				return store.prepareUpdateWorkflowExecutionTxn(executionInfo, versionHistories, checksum, time.Now(), 123)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err) // Expect an error due to invalid state
				assert.Nil(t, result)
			},
		},
		{
			name: "PrepareUpdateWorkflowExecutionTxn - Invalid Workflow State",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "domainID-invalid-state",
					WorkflowID:  "workflowID-invalid-state",
					RunID:       "runID-invalid-state",
					State:       343, // Invalid state
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				versionHistories := &persistence.DataBlob{
					Encoding: constants.EncodingTypeJSON,
					Data:     []byte(`[{"Branches":[{"BranchID":"branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
				}
				checksum := checksum.Checksum{Version: 1, Value: []byte("checksum")}
				return store.prepareUpdateWorkflowExecutionTxn(executionInfo, versionHistories, checksum, time.Now(), 123)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)  // Expect an error due to invalid workflow state
				assert.Nil(t, result) // No WorkflowExecutionRequest should be returned
			},
		},
		{
			name: "PrepareCreateWorkflowExecutionTxn - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "create-domain-id",
					WorkflowID:  "create-workflow-id",
					RunID:       "create-run-id",
					State:       persistence.WorkflowStateCreated,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				versionHistories := &persistence.DataBlob{
					Encoding: constants.EncodingTypeJSON,
					Data:     []byte(`[{"Branches":[{"BranchID":"create-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
				}
				checksum := checksum.Checksum{Version: 1, Value: []byte("create-checksum")}
				return store.prepareCreateWorkflowExecutionTxn(executionInfo, versionHistories, checksum, time.Now(), 123)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				req := result.(*nosqlplugin.WorkflowExecutionRequest)
				assert.Equal(t, "create-domain-id", req.DomainID)
				assert.Equal(t, int64(123), req.LastWriteVersion)
			},
		},
		{
			name: "PrepareCreateWorkflowExecutionTxn - Invalid State",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					DomainID:    "create-domain-id",
					WorkflowID:  "create-workflow-id",
					RunID:       "create-run-id",
					State:       232, // Invalid state for creating a workflow execution
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				versionHistories := &persistence.DataBlob{
					Encoding: constants.EncodingTypeJSON,
					Data:     []byte(`[{"Branches":[{"BranchID":"create-branch-id","BeginNodeID":1,"EndNodeID":2}]}]`),
				}
				checksum := checksum.Checksum{Version: 1, Value: []byte("create-checksum")}
				return store.prepareCreateWorkflowExecutionTxn(executionInfo, versionHistories, checksum, time.Now(), 123)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		},
		{
			name: "prepareCurrentWorkflowRequestForCreateWorkflowTxn - BrandNew mode",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
					CreateRequestID: "test-create-request-id",
				}
				request := &persistence.InternalCreateWorkflowExecutionRequest{
					Mode: persistence.CreateWorkflowModeBrandNew,
				}
				return store.prepareCurrentWorkflowRequestForCreateWorkflowTxn(
					"test-domain-id", "test-workflow-id", "test-run-id", executionInfo, 123, request)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				currentWorkflowReq, ok := result.(*nosqlplugin.CurrentWorkflowWriteRequest)
				assert.True(t, ok)
				assert.NotNil(t, currentWorkflowReq)
				assert.Equal(t, nosqlplugin.CurrentWorkflowWriteModeInsert, currentWorkflowReq.WriteMode)
			},
		},
		{
			name: "processUpdateWorkflowResult - CurrentWorkflowConditionFailInfo error",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					CurrentWorkflowConditionFailInfo: common.StringPtr("current workflow condition failed"),
				}
				return nil, store.processUpdateWorkflowResult(err, 99)
			},
			validate: func(t *testing.T, _ interface{}, err error) {
				assert.Error(t, err)
				_, ok := err.(*persistence.CurrentWorkflowConditionFailedError)
				assert.True(t, ok)
			},
		},
		{
			name: "processUpdateWorkflowResult - Success",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				return nil, store.processUpdateWorkflowResult(nil, 99)
			},
			validate: func(t *testing.T, _ interface{}, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "processUpdateWorkflowResult - ShardRangeIDNotMatch error",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				err := &nosqlplugin.WorkflowOperationConditionFailure{
					ShardRangeIDNotMatch: common.Int64Ptr(100),
				}
				return nil, store.processUpdateWorkflowResult(err, 99)
			},
			validate: func(t *testing.T, _ interface{}, err error) {
				assert.Error(t, err)
				_, ok := err.(*persistence.ShardOwnershipLostError)
				assert.True(t, ok)
			},
		},
		{
			name: "prepareCurrentWorkflowRequestForCreateWorkflowTxn - WorkflowIDReuse mode with non-completed state",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					State:           persistence.WorkflowStateRunning, // Simulate a running state
					CloseStatus:     persistence.WorkflowCloseStatusNone,
					CreateRequestID: "test-create-request-id",
				}
				request := &persistence.InternalCreateWorkflowExecutionRequest{
					Mode:                     persistence.CreateWorkflowModeWorkflowIDReuse,
					PreviousRunID:            "test-run-id",
					PreviousLastWriteVersion: 123, // Simulating a non-completed state with a valid version
				}
				return store.prepareCurrentWorkflowRequestForCreateWorkflowTxn(
					"test-domain-id", "test-workflow-id", "test-run-id", executionInfo, 123, request)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				_, ok := err.(*persistence.CurrentWorkflowConditionFailedError)
				assert.False(t, ok)
			},
		},
		{
			name: "CurrentWorkflowRequestForCreateWorkflowTxn - Zombie mode",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					State:           persistence.WorkflowStateCreated,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
					CreateRequestID: "create-request-id-zombie",
				}
				request := &persistence.InternalCreateWorkflowExecutionRequest{
					Mode:          persistence.CreateWorkflowModeZombie,
					PreviousRunID: "previous-run-id-zombie",
				}
				return store.prepareCurrentWorkflowRequestForCreateWorkflowTxn(
					"domain-id-zombie", "workflow-id-zombie", "run-id-zombie", executionInfo, 123, request)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				currentWorkflowReq := result.(*nosqlplugin.CurrentWorkflowWriteRequest)
				assert.Equal(t, nosqlplugin.CurrentWorkflowWriteModeNoop, currentWorkflowReq.WriteMode)
				assert.Equal(t, "create-request-id-zombie", currentWorkflowReq.Row.CreateRequestID)
			},
		},
		{
			name: "CurrentWorkflowRequestForCreateWorkflowTxn - ContinueAsNew mode",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				executionInfo := &persistence.InternalWorkflowExecutionInfo{
					State:           persistence.WorkflowStateRunning,
					CloseStatus:     persistence.WorkflowCloseStatusNone,
					CreateRequestID: "create-request-id-continueasnew",
				}
				request := &persistence.InternalCreateWorkflowExecutionRequest{
					Mode:          persistence.CreateWorkflowModeContinueAsNew,
					PreviousRunID: "previous-run-id-continueasnew",
				}
				return store.prepareCurrentWorkflowRequestForCreateWorkflowTxn(
					"domain-id-continueasnew", "workflow-id-continueasnew", "run-id-continueasnew", executionInfo, 123, request)
			},
			validate: func(t *testing.T, result interface{}, err error) {
				assert.NoError(t, err)
				currentWorkflowReq := result.(*nosqlplugin.CurrentWorkflowWriteRequest)
				assert.Equal(t, nosqlplugin.CurrentWorkflowWriteModeUpdate, currentWorkflowReq.WriteMode)
				assert.Equal(t, "create-request-id-continueasnew", currentWorkflowReq.Row.CreateRequestID)
				assert.NotNil(t, currentWorkflowReq.Condition)
				assert.Equal(t, "previous-run-id-continueasnew", *currentWorkflowReq.Condition.CurrentRunID)
			},
		},
		{
			name: "assertNotCurrentExecution - Success with different RunID",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				ctx := context.Background()
				mockDB := store.db.(*nosqlplugin.MockDB)
				mockDB.EXPECT().SelectCurrentWorkflow(
					gomock.Any(),
					store.shardID,
					"test-domain-id",
					"test-workflow-id",
				).Return(&nosqlplugin.CurrentWorkflowRow{
					RunID: "different-run-id",
				}, nil)
				return nil, store.assertNotCurrentExecution(ctx, "test-domain-id", "test-workflow-id", "expected-run-id")
			},
			validate: func(t *testing.T, _ interface{}, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "assertNotCurrentExecution - No current workflow",
			setupStore: func(store *nosqlExecutionStore) (interface{}, error) {
				ctx := context.Background()
				mockDB := store.db.(*nosqlplugin.MockDB)

				mockDB.EXPECT().SelectCurrentWorkflow(
					gomock.Any(),
					store.shardID,
					"test-domain-id",
					"test-workflow-id",
				).Return(nil, &types.EntityNotExistsError{})
				mockDB.EXPECT().IsNotFoundError(gomock.Any()).Return(true).AnyTimes()
				return nil, store.assertNotCurrentExecution(ctx, "test-domain-id", "test-workflow-id", "expected-run-id")
			},
			validate: func(t *testing.T, _ interface{}, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)

			mockDB := nosqlplugin.NewMockDB(mockCtrl)
			store := newTestNosqlExecutionStore(mockDB, log.NewNoop())

			result, err := tc.setupStore(store)
			tc.validate(t, result, err)
		})
	}
}

type dummyTaskType struct {
	persistence.Task
	VisibilityTimestamp time.Time
	TaskID              int64
}

func (d *dummyTaskType) GetTaskType() int {
	return 999 // Using a type that is not expected by the switch statement
}

func (d *dummyTaskType) GetVersion() int64 {
	return 1
}

func (d *dummyTaskType) SetVersion(version int64) {}

func (d *dummyTaskType) ToTransferTaskInfo() (*persistence.TransferTaskInfo, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyTaskType) ToTimerTaskInfo() (*persistence.TimerTaskInfo, error) {
	return nil, errors.New("not implemented")
}

func (d *dummyTaskType) ToInternalReplicationTaskInfo() (*types.ReplicationTaskInfo, error) {
	return nil, errors.New("not implemented")
}
