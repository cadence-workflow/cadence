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

package execution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/engine"
	"github.com/uber/cadence/service/history/events"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/history/shard"
)

func TestIsOperationPossiblySuccessfulError(t *testing.T) {
	assert.False(t, isOperationPossiblySuccessfulError(nil))
	assert.False(t, isOperationPossiblySuccessfulError(&types.WorkflowExecutionAlreadyStartedError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&persistence.WorkflowExecutionAlreadyStartedError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&persistence.CurrentWorkflowConditionFailedError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&persistence.ConditionFailedError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&types.ServiceBusyError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&types.LimitExceededError{}))
	assert.False(t, isOperationPossiblySuccessfulError(&persistence.ShardOwnershipLostError{}))
	assert.True(t, isOperationPossiblySuccessfulError(&persistence.TimeoutError{}))
	assert.False(t, isOperationPossiblySuccessfulError(NewConflictError(t, &persistence.ConditionFailedError{})))
	assert.True(t, isOperationPossiblySuccessfulError(context.DeadlineExceeded))
}

func TestMergeContinueAsNewReplicationTasks(t *testing.T) {
	testCases := []struct {
		name                    string
		updateMode              persistence.UpdateWorkflowMode
		currentWorkflowMutation *persistence.WorkflowMutation
		newWorkflowSnapshot     *persistence.WorkflowSnapshot
		wantErr                 bool
		assertErr               func(*testing.T, error)
	}{
		{
			name: "current workflow does not continue as new",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusCompleted,
				},
			},
			wantErr: false,
		},
		{
			name: "update workflow as zombie and continue as new without new zombie workflow",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
			},
			updateMode: persistence.UpdateWorkflowModeBypassCurrent,
			wantErr:    false,
		},
		{
			name: "continue as new on the passive side",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
			},
			updateMode: persistence.UpdateWorkflowModeUpdateCurrent,
			wantErr:    false,
		},
		{
			name: "continue as new on the active side, but new workflow is not provided",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{},
					},
				},
			},
			updateMode: persistence.UpdateWorkflowModeUpdateCurrent,
			wantErr:    true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.Contains(t, err.Error(), "unable to find replication task from new workflow for continue as new replication")
			},
		},
		{
			name: "continue as new on the active side, but new workflow has no replication task",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{},
					},
				},
			},
			newWorkflowSnapshot: &persistence.WorkflowSnapshot{},
			updateMode:          persistence.UpdateWorkflowModeUpdateCurrent,
			wantErr:             true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.Contains(t, err.Error(), "unable to find replication task from new workflow for continue as new replication")
			},
		},
		{
			name: "continue as new on the active side, but current workflow has no history replication task",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.SyncActivityTask{},
					},
				},
			},
			newWorkflowSnapshot: &persistence.WorkflowSnapshot{
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{},
					},
				},
			},
			updateMode: persistence.UpdateWorkflowModeUpdateCurrent,
			wantErr:    true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
				assert.Contains(t, err.Error(), "unable to find replication task from current workflow for continue as new replication")
			},
		},
		{
			name: "continue as new on the active side",
			currentWorkflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					CloseStatus: persistence.WorkflowCloseStatusContinuedAsNew,
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{},
					},
				},
			},
			newWorkflowSnapshot: &persistence.WorkflowSnapshot{
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{},
					},
				},
			},
			updateMode: persistence.UpdateWorkflowModeUpdateCurrent,
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := log.NewNoop()
			err := mergeContinueAsNewReplicationTasks(logger, tc.updateMode, tc.currentWorkflowMutation, tc.newWorkflowSnapshot)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNotifyTasksFromWorkflowSnapshot(t *testing.T) {
	testCases := []struct {
		name             string
		workflowSnapShot *persistence.WorkflowSnapshot
		history          events.PersistedBlobs
		persistenceError bool
		mockSetup        func(*engine.MockEngine)
	}{
		{
			name: "Success case",
			workflowSnapShot: &persistence.WorkflowSnapshot{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				VersionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte{1, 2, 3},
						},
					},
				},
				ActivityInfos: []*persistence.ActivityInfo{
					{
						Version:    1,
						ScheduleID: 11,
					},
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: {
						&persistence.ActivityTask{
							TaskList: "test-tl",
						},
					},
					persistence.HistoryTaskCategoryTimer: {
						&persistence.ActivityTimeoutTask{
							Attempt: 10,
						},
					},
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{
							FirstEventID: 1,
							NextEventID:  10,
						},
					},
				},
			},
			history: events.PersistedBlobs{
				events.PersistedBlob{},
			},
			persistenceError: true,
			mockSetup: func(mockEngine *engine.MockEngine) {
				mockEngine.EXPECT().NotifyNewTransferTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.ActivityTask{
							TaskList: "test-tl",
						},
					},
					PersistenceError: true,
				})
				mockEngine.EXPECT().NotifyNewTimerTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.ActivityTimeoutTask{
							Attempt: 10,
						},
					},
					PersistenceError: true,
					ScheduleInMemory: true,
				})
				mockEngine.EXPECT().NotifyNewReplicationTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.HistoryReplicationTask{
							FirstEventID: 1,
							NextEventID:  10,
						},
					},
					VersionHistories: &persistence.VersionHistories{
						CurrentVersionHistoryIndex: 0,
						Histories: []*persistence.VersionHistory{
							{
								BranchToken: []byte{1, 2, 3},
							},
						},
					},
					Activities: map[int64]*persistence.ActivityInfo{
						11: {
							Version:    1,
							ScheduleID: 11,
						},
					},
					History: events.PersistedBlobs{
						events.PersistedBlob{},
					},
					PersistenceError: true,
				})
			},
		},
		{
			name: "nil snapshot",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEngine := engine.NewMockEngine(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockEngine)
			}
			notifyTasksFromWorkflowSnapshot(mockEngine, tc.workflowSnapShot, tc.history, tc.persistenceError)
		})
	}
}

func TestNotifyTasksFromWorkflowMutation(t *testing.T) {
	testCases := []struct {
		name             string
		workflowMutation *persistence.WorkflowMutation
		history          events.PersistedBlobs
		persistenceError bool
		mockSetup        func(*engine.MockEngine)
	}{
		{
			name: "Success case",
			workflowMutation: &persistence.WorkflowMutation{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				VersionHistories: &persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte{1, 2, 3},
						},
					},
				},
				UpsertActivityInfos: []*persistence.ActivityInfo{
					{
						Version:    1,
						ScheduleID: 11,
					},
				},
				TasksByCategory: map[persistence.HistoryTaskCategory][]persistence.Task{
					persistence.HistoryTaskCategoryTransfer: {
						&persistence.ActivityTask{
							TaskList: "test-tl",
						},
					},
					persistence.HistoryTaskCategoryTimer: {
						&persistence.ActivityTimeoutTask{
							Attempt: 10,
						},
					},
					persistence.HistoryTaskCategoryReplication: {
						&persistence.HistoryReplicationTask{
							FirstEventID: 1,
							NextEventID:  10,
						},
					},
				},
			},
			history: events.PersistedBlobs{
				events.PersistedBlob{},
			},
			persistenceError: true,
			mockSetup: func(mockEngine *engine.MockEngine) {
				mockEngine.EXPECT().NotifyNewTransferTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.ActivityTask{
							TaskList: "test-tl",
						},
					},
					PersistenceError: true,
				})
				mockEngine.EXPECT().NotifyNewTimerTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.ActivityTimeoutTask{
							Attempt: 10,
						},
					},
					PersistenceError: true,
					ScheduleInMemory: true,
				})
				mockEngine.EXPECT().NotifyNewReplicationTasks(&hcommon.NotifyTaskInfo{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					Tasks: []persistence.Task{
						&persistence.HistoryReplicationTask{
							FirstEventID: 1,
							NextEventID:  10,
						},
					},
					VersionHistories: &persistence.VersionHistories{
						CurrentVersionHistoryIndex: 0,
						Histories: []*persistence.VersionHistory{
							{
								BranchToken: []byte{1, 2, 3},
							},
						},
					},
					Activities: map[int64]*persistence.ActivityInfo{
						11: {
							Version:    1,
							ScheduleID: 11,
						},
					},
					History: events.PersistedBlobs{
						events.PersistedBlob{},
					},
					PersistenceError: true,
				})
			},
		},
		{
			name: "nil mutation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockEngine := engine.NewMockEngine(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockEngine)
			}
			notifyTasksFromWorkflowMutation(mockEngine, tc.workflowMutation, tc.history, tc.persistenceError)
		})
	}
}

func TestActivityInfosToMap(t *testing.T) {
	testCases := []struct {
		name       string
		activities []*persistence.ActivityInfo
		want       map[int64]*persistence.ActivityInfo
	}{
		{
			name: "non-empty",
			activities: []*persistence.ActivityInfo{
				{
					Version:    1,
					ScheduleID: 11,
				},
				{
					Version:    2,
					ScheduleID: 12,
				},
			},
			want: map[int64]*persistence.ActivityInfo{
				11: {
					Version:    1,
					ScheduleID: 11,
				},
				12: {
					Version:    2,
					ScheduleID: 12,
				},
			},
		},
		{
			name:       "empty slice",
			activities: []*persistence.ActivityInfo{},
			want:       map[int64]*persistence.ActivityInfo{},
		},
		{
			name: "nil slice",
			want: map[int64]*persistence.ActivityInfo{},
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.want, activityInfosToMap(tc.activities))
	}
}

func TestCreateWorkflowExecutionWithRetry(t *testing.T) {
	testCases := []struct {
		name      string
		request   *persistence.CreateWorkflowExecutionRequest
		mockSetup func(*shard.MockContext)
		want      *persistence.CreateWorkflowExecutionResponse
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			request: &persistence.CreateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().CreateWorkflowExecution(gomock.Any(), &persistence.CreateWorkflowExecutionRequest{
					RangeID: 100,
				}).Return(&persistence.CreateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.CreateWorkflowExecutionResponse{
				MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
		{
			name: "workflow already started error",
			request: &persistence.CreateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &persistence.WorkflowExecutionAlreadyStartedError{})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &persistence.WorkflowExecutionAlreadyStartedError{})
			},
		},
		{
			name: "timeout error",
			request: &persistence.CreateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &persistence.TimeoutError{})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &persistence.TimeoutError{})
			},
		},
		{
			name: "retry succeeds",
			request: &persistence.CreateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{})
				mockShard.EXPECT().CreateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.CreateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.CreateWorkflowExecutionResponse{
				MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			policy := backoff.NewExponentialRetryPolicy(time.Millisecond)
			policy.SetMaximumAttempts(1)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard)
			}
			resp, err := createWorkflowExecutionWithRetry(context.Background(), mockShard, testlogger.New(t), policy, tc.request)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestUpdateWorkflowExecutionWithRetry(t *testing.T) {
	testCases := []struct {
		name      string
		request   *persistence.UpdateWorkflowExecutionRequest
		mockSetup func(*shard.MockContext)
		want      *persistence.UpdateWorkflowExecutionResponse
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			request: &persistence.UpdateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().UpdateWorkflowExecution(gomock.Any(), &persistence.UpdateWorkflowExecutionRequest{
					RangeID: 100,
				}).Return(&persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.UpdateWorkflowExecutionResponse{
				MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
		{
			name: "condition failed error",
			request: &persistence.UpdateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &persistence.ConditionFailedError{})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &conflictError{})
			},
		},
		{
			name: "timeout error",
			request: &persistence.UpdateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &persistence.TimeoutError{})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &persistence.TimeoutError{})
			},
		},
		{
			name: "retry succeeds",
			request: &persistence.UpdateWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{})
				mockShard.EXPECT().UpdateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.UpdateWorkflowExecutionResponse{
				MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			policy := backoff.NewExponentialRetryPolicy(time.Millisecond)
			policy.SetMaximumAttempts(1)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard)
			}
			resp, err := updateWorkflowExecutionWithRetry(context.Background(), mockShard, testlogger.New(t), policy, tc.request)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestAppendHistoryV2EventsWithRetry(t *testing.T) {
	testCases := []struct {
		name      string
		domainID  string
		execution types.WorkflowExecution
		request   *persistence.AppendHistoryNodesRequest
		mockSetup func(*shard.MockContext)
		want      *persistence.AppendHistoryNodesResponse
		wantErr   bool
	}{
		{
			name:     "Success case",
			domainID: "test-domain-id",
			execution: types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			request: &persistence.AppendHistoryNodesRequest{
				IsNewBranch: true,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().AppendHistoryV2Events(gomock.Any(), &persistence.AppendHistoryNodesRequest{
					IsNewBranch: true,
				}, "test-domain-id", types.WorkflowExecution{WorkflowID: "test-workflow-id", RunID: "test-run-id"}).Return(&persistence.AppendHistoryNodesResponse{
					DataBlob: persistence.DataBlob{},
				}, nil)
			},
			want: &persistence.AppendHistoryNodesResponse{
				DataBlob: persistence.DataBlob{},
			},
			wantErr: false,
		},
		{
			name:     "retry success",
			domainID: "test-domain-id",
			execution: types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			request: &persistence.AppendHistoryNodesRequest{
				IsNewBranch: true,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{})
				mockShard.EXPECT().AppendHistoryV2Events(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&persistence.AppendHistoryNodesResponse{
					DataBlob: persistence.DataBlob{},
				}, nil)
			},
			want: &persistence.AppendHistoryNodesResponse{
				DataBlob: persistence.DataBlob{},
			},
			wantErr: false,
		},
		{
			name:     "non retryable error",
			domainID: "test-domain-id",
			execution: types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			request: &persistence.AppendHistoryNodesRequest{
				IsNewBranch: true,
			},
			mockSetup: func(mockShard *shard.MockContext) {
				mockShard.EXPECT().AppendHistoryV2Events(gomock.Any(), &persistence.AppendHistoryNodesRequest{
					IsNewBranch: true,
				}, "test-domain-id", types.WorkflowExecution{WorkflowID: "test-workflow-id", RunID: "test-run-id"}).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			policy := backoff.NewExponentialRetryPolicy(time.Millisecond)
			policy.SetMaximumAttempts(1)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard)
			}
			resp, err := appendHistoryV2EventsWithRetry(context.Background(), mockShard, policy, tc.domainID, tc.execution, tc.request)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestPersistStartWorkflowBatchEvents(t *testing.T) {
	testCases := []struct {
		name                     string
		workflowEvents           *persistence.WorkflowEvents
		mockSetup                func(*shard.MockContext, *cache.MockDomainCache)
		mockAppendHistoryNodesFn func(context.Context, string, types.WorkflowExecution, *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error)
		wantErr                  bool
		want                     events.PersistedBlob
		assertErr                func(*testing.T, error)
	}{
		{
			name:           "empty events",
			workflowEvents: &persistence.WorkflowEvents{},
			wantErr:        true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &types.InternalServiceError{})
				assert.Contains(t, err.Error(), "cannot persist first workflow events with empty events")
			},
		},
		{
			name: "failed to get domain name",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("", errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, err, errors.New("some error"))
			},
		},
		{
			name: "failed to append history nodes",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockAppendHistoryNodesFn: func(context.Context, string, types.WorkflowExecution, *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error) {
				return nil, errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, err, errors.New("some error"))
			},
		},
		{
			name: "success",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
				BranchToken: []byte{1, 2, 3},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockAppendHistoryNodesFn: func(ctx context.Context, domainID string, execution types.WorkflowExecution, req *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error) {
				assert.Equal(t, &persistence.AppendHistoryNodesRequest{
					IsNewBranch: true,
					Info:        "::",
					BranchToken: []byte{1, 2, 3},
					Events: []*types.HistoryEvent{
						{
							ID: 1,
						},
					},
					DomainName: "test-domain",
				}, req)
				return &persistence.AppendHistoryNodesResponse{
					DataBlob: persistence.DataBlob{
						Data: []byte("123"),
					},
				}, nil
			},
			want: events.PersistedBlob{
				DataBlob: persistence.DataBlob{
					Data: []byte("123"),
				},
				BranchToken:  []byte{1, 2, 3},
				FirstEventID: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache)
			}
			ctx := &contextImpl{
				shard: mockShard,
			}
			if tc.mockAppendHistoryNodesFn != nil {
				ctx.appendHistoryNodesFn = tc.mockAppendHistoryNodesFn
			}
			got, err := ctx.PersistStartWorkflowBatchEvents(context.Background(), tc.workflowEvents)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestPersistNonStartWorkflowBatchEvents(t *testing.T) {
	testCases := []struct {
		name                     string
		workflowEvents           *persistence.WorkflowEvents
		mockSetup                func(*shard.MockContext, *cache.MockDomainCache)
		mockAppendHistoryNodesFn func(context.Context, string, types.WorkflowExecution, *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error)
		wantErr                  bool
		want                     events.PersistedBlob
		assertErr                func(*testing.T, error)
	}{
		{
			name:           "empty events",
			workflowEvents: &persistence.WorkflowEvents{},
			wantErr:        false,
			want:           events.PersistedBlob{},
		},
		{
			name: "failed to get domain name",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("", errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, err, errors.New("some error"))
			},
		},
		{
			name: "failed to append history nodes",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockAppendHistoryNodesFn: func(context.Context, string, types.WorkflowExecution, *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error) {
				return nil, errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, err, errors.New("some error"))
			},
		},
		{
			name: "success",
			workflowEvents: &persistence.WorkflowEvents{
				Events: []*types.HistoryEvent{
					{
						ID: 1,
					},
				},
				BranchToken: []byte{1, 2, 3},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockAppendHistoryNodesFn: func(ctx context.Context, domainID string, execution types.WorkflowExecution, req *persistence.AppendHistoryNodesRequest) (*persistence.AppendHistoryNodesResponse, error) {
				assert.Equal(t, &persistence.AppendHistoryNodesRequest{
					IsNewBranch: false,
					BranchToken: []byte{1, 2, 3},
					Events: []*types.HistoryEvent{
						{
							ID: 1,
						},
					},
					DomainName: "test-domain",
				}, req)
				return &persistence.AppendHistoryNodesResponse{
					DataBlob: persistence.DataBlob{
						Data: []byte("123"),
					},
				}, nil
			},
			want: events.PersistedBlob{
				DataBlob: persistence.DataBlob{
					Data: []byte("123"),
				},
				BranchToken:  []byte{1, 2, 3},
				FirstEventID: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache)
			}
			ctx := &contextImpl{
				shard: mockShard,
			}
			if tc.mockAppendHistoryNodesFn != nil {
				ctx.appendHistoryNodesFn = tc.mockAppendHistoryNodesFn
			}
			got, err := ctx.PersistNonStartWorkflowBatchEvents(context.Background(), tc.workflowEvents)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestCreateWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                                  string
		newWorkflow                           *persistence.WorkflowSnapshot
		history                               events.PersistedBlob
		createMode                            persistence.CreateWorkflowMode
		prevRunID                             string
		prevLastWriteVersion                  int64
		createWorkflowRequestMode             persistence.CreateWorkflowRequestMode
		mockCreateWorkflowExecutionFn         func(context.Context, *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error)
		mockNotifyTasksFromWorkflowSnapshotFn func(*persistence.WorkflowSnapshot, events.PersistedBlobs, bool)
		mockEmitSessionUpdateStatsFn          func(string, *persistence.MutableStateUpdateSessionStats)
		wantErr                               bool
	}{
		{
			name: "failed to create workflow execution with possibly success error",
			newWorkflow: &persistence.WorkflowSnapshot{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
			history: events.PersistedBlob{
				DataBlob: persistence.DataBlob{
					Data: []byte("123"),
				},
				BranchToken:  []byte{1, 2, 3},
				FirstEventID: 1,
			},
			createMode:                persistence.CreateWorkflowModeContinueAsNew,
			prevRunID:                 "test-prev-run-id",
			prevLastWriteVersion:      123,
			createWorkflowRequestMode: persistence.CreateWorkflowRequestModeReplicated,
			mockCreateWorkflowExecutionFn: func(context.Context, *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error) {
				return nil, &types.InternalServiceError{}
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(_ *persistence.WorkflowSnapshot, _ events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError)
			},
			wantErr: true,
		},
		{
			name: "failed to validate workflow requests",
			newWorkflow: &persistence.WorkflowSnapshot{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
				WorkflowRequests: []*persistence.WorkflowRequest{{}, {}},
			},
			history: events.PersistedBlob{
				DataBlob: persistence.DataBlob{
					Data: []byte("123"),
				},
				BranchToken:  []byte{1, 2, 3},
				FirstEventID: 1,
			},
			createMode:                persistence.CreateWorkflowModeContinueAsNew,
			prevRunID:                 "test-prev-run-id",
			prevLastWriteVersion:      123,
			createWorkflowRequestMode: persistence.CreateWorkflowRequestModeNew,
			wantErr:                   true,
		},
		{
			name: "success",
			newWorkflow: &persistence.WorkflowSnapshot{
				ExecutionInfo: &persistence.WorkflowExecutionInfo{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			},
			history: events.PersistedBlob{
				DataBlob: persistence.DataBlob{
					Data: []byte("123"),
				},
				BranchToken:  []byte{1, 2, 3},
				FirstEventID: 1,
			},
			createMode:                persistence.CreateWorkflowModeContinueAsNew,
			prevRunID:                 "test-prev-run-id",
			prevLastWriteVersion:      123,
			createWorkflowRequestMode: persistence.CreateWorkflowRequestModeReplicated,
			mockCreateWorkflowExecutionFn: func(ctx context.Context, req *persistence.CreateWorkflowExecutionRequest) (*persistence.CreateWorkflowExecutionResponse, error) {
				assert.Equal(t, &persistence.CreateWorkflowExecutionRequest{
					Mode:                     persistence.CreateWorkflowModeContinueAsNew,
					PreviousRunID:            "test-prev-run-id",
					PreviousLastWriteVersion: 123,
					NewWorkflowSnapshot: persistence.WorkflowSnapshot{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 3,
						},
					},
					WorkflowRequestMode: persistence.CreateWorkflowRequestModeReplicated,
					DomainName:          "test-domain",
				}, req)
				return &persistence.CreateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(newWorkflow *persistence.WorkflowSnapshot, history events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, &persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
				}, newWorkflow)
				assert.Equal(t, events.PersistedBlobs{
					{
						DataBlob: persistence.DataBlob{
							Data: []byte("123"),
						},
						BranchToken:  []byte{1, 2, 3},
						FirstEventID: 1,
					},
				}, history)
				assert.Equal(t, false, persistenceError)
			},
			mockEmitSessionUpdateStatsFn: func(domainName string, stats *persistence.MutableStateUpdateSessionStats) {
				assert.Equal(t, "test-domain", domainName)
				assert.Equal(t, &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				}, stats)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
			mockShard.EXPECT().GetConfig().Return(&config.Config{
				EnableStrongIdempotencySanityCheck: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
			}).AnyTimes()
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			ctx := &contextImpl{
				logger:        testlogger.New(t),
				shard:         mockShard,
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			}
			if tc.mockCreateWorkflowExecutionFn != nil {
				ctx.createWorkflowExecutionFn = tc.mockCreateWorkflowExecutionFn
			}
			if tc.mockNotifyTasksFromWorkflowSnapshotFn != nil {
				ctx.notifyTasksFromWorkflowSnapshotFn = tc.mockNotifyTasksFromWorkflowSnapshotFn
			}
			if tc.mockEmitSessionUpdateStatsFn != nil {
				ctx.emitSessionUpdateStatsFn = tc.mockEmitSessionUpdateStatsFn
			}
			err := ctx.CreateWorkflowExecution(context.Background(), tc.newWorkflow, tc.history, tc.createMode, tc.prevRunID, tc.prevLastWriteVersion, tc.createWorkflowRequestMode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateWorkflowExecutionTasks(t *testing.T) {
	testCases := []struct {
		name                                  string
		mockSetup                             func(*shard.MockContext, *cache.MockDomainCache, *MockMutableState)
		mockUpdateWorkflowExecutionFn         func(context.Context, *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error)
		mockNotifyTasksFromWorkflowMutationFn func(*persistence.WorkflowMutation, events.PersistedBlobs, bool)
		mockEmitSessionUpdateStatsFn          func(string, *persistence.MutableStateUpdateSessionStats)
		wantErr                               bool
		assertErr                             func(*testing.T, error)
	}{
		{
			name: "CloseTransactionAsMutation failed",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "found unexpected new events",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{{}}, nil)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
			},
		},
		{
			name: "found unexpected workflow requests",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{
					WorkflowRequests: []*persistence.WorkflowRequest{{}},
				}, []*persistence.WorkflowEvents{}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					EnableStrongIdempotencySanityCheck: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "UpdateWorkflowExecutionTask can only be used for persisting new workflow tasks, but found new workflow requests"}, err)
			},
		},
		{
			name: "domain cache error",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("", errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "update workflow failed with possibly success error",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockUpdateWorkflowExecutionFn: func(_ context.Context, request *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
				return nil, &types.InternalServiceError{}
			},
			mockNotifyTasksFromWorkflowMutationFn: func(_ *persistence.WorkflowMutation, _ events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError, "case: update workflow failed with possibly success error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, &types.InternalServiceError{}, err)
			},
		},
		{
			name: "success",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
				}, []*persistence.WorkflowEvents{}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockUpdateWorkflowExecutionFn: func(_ context.Context, request *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
				assert.Equal(t, &persistence.UpdateWorkflowExecutionRequest{
					UpdateWorkflowMutation: persistence.WorkflowMutation{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					Mode:       persistence.UpdateWorkflowModeIgnoreCurrent,
					DomainName: "test-domain",
				}, request, "case: success")
				return &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil
			},
			mockNotifyTasksFromWorkflowMutationFn: func(mutation *persistence.WorkflowMutation, history events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, &persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
					},
					ExecutionStats: &persistence.ExecutionStats{},
				}, mutation, "case: success")
				assert.Nil(t, history, "case: success")
				assert.Equal(t, false, persistenceError, "case: success")
			},
			mockEmitSessionUpdateStatsFn: func(domainName string, stats *persistence.MutableStateUpdateSessionStats) {
				assert.Equal(t, "test-domain", domainName, "case: success")
				assert.Equal(t, &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				}, stats, "case: success")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockMutableState := NewMockMutableState(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache, mockMutableState)
			}
			ctx := &contextImpl{
				logger:        testlogger.New(t),
				shard:         mockShard,
				mutableState:  mockMutableState,
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			}
			if tc.mockUpdateWorkflowExecutionFn != nil {
				ctx.updateWorkflowExecutionFn = tc.mockUpdateWorkflowExecutionFn
			}
			if tc.mockNotifyTasksFromWorkflowMutationFn != nil {
				ctx.notifyTasksFromWorkflowMutationFn = tc.mockNotifyTasksFromWorkflowMutationFn
			}
			if tc.mockEmitSessionUpdateStatsFn != nil {
				ctx.emitSessionUpdateStatsFn = tc.mockEmitSessionUpdateStatsFn
			}
			err := ctx.UpdateWorkflowExecutionTasks(context.Background(), time.Unix(0, 0))
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})

	}
}

func TestUpdateWorkflowExecutionWithNew(t *testing.T) {
	testCases := []struct {
		name                                      string
		updateMode                                persistence.UpdateWorkflowMode
		newContext                                Context
		currentWorkflowTransactionPolicy          TransactionPolicy
		newWorkflowTransactionPolicy              *TransactionPolicy
		workflowRequestMode                       persistence.CreateWorkflowRequestMode
		mockSetup                                 func(*shard.MockContext, *cache.MockDomainCache, *MockMutableState, *MockMutableState, *engine.MockEngine)
		mockPersistNonStartWorkflowBatchEventsFn  func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		mockPersistStartWorkflowBatchEventsFn     func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		mockUpdateWorkflowExecutionFn             func(context.Context, *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error)
		mockNotifyTasksFromWorkflowMutationFn     func(*persistence.WorkflowMutation, events.PersistedBlobs, bool)
		mockNotifyTasksFromWorkflowSnapshotFn     func(*persistence.WorkflowSnapshot, events.PersistedBlobs, bool)
		mockEmitSessionUpdateStatsFn              func(string, *persistence.MutableStateUpdateSessionStats)
		mockEmitWorkflowHistoryStatsFn            func(string, int, int)
		mockEmitLargeWorkflowShardIDStatsFn       func(int64, int64, int64, int64)
		mockEmitWorkflowCompletionStatsFn         func(string, string, string, string, string, *types.HistoryEvent)
		mockMergeContinueAsNewReplicationTasksFn  func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error
		mockUpdateWorkflowExecutionEventReapplyFn func(persistence.UpdateWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error
		wantErr                                   bool
		assertErr                                 func(*testing.T, error)
	}{
		{
			name:                             "CloseTransactionAsMutation failed",
			currentWorkflowTransactionPolicy: TransactionPolicyPassive,
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name:                             "PersistNonStartWorkflowBatchEvents failed",
			currentWorkflowTransactionPolicy: TransactionPolicyPassive,
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "CloseTransactionAsSnapshot failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(gomock.Any())
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "both current workflow and new workflow generate workflow requests",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					EnableStrongIdempotencySanityCheck: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				})
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{
					WorkflowRequests: []*persistence.WorkflowRequest{{}},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(gomock.Any())
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{
					WorkflowRequests: []*persistence.WorkflowRequest{{}},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "workflow requests are only expected to be generated from one workflow for UpdateWorkflowExecution"}, err)
			},
		},
		{
			name: "mergeContinueAsNewReplicationTasks failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(gomock.Any())
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockMergeContinueAsNewReplicationTasksFn: func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error {
				return errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "updateWorkflowExecutionEventReapply failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(gomock.Any())
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockMergeContinueAsNewReplicationTasksFn: func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error {
				return nil
			},
			mockUpdateWorkflowExecutionEventReapplyFn: func(persistence.UpdateWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error {
				return errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "updateWorkflowExecution failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(gomock.Any())
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockMergeContinueAsNewReplicationTasksFn: func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error {
				return nil
			},
			mockUpdateWorkflowExecutionEventReapplyFn: func(persistence.UpdateWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error {
				return nil
			},
			mockUpdateWorkflowExecutionFn: func(context.Context, *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
				return nil, errors.New("some error")
			},
			mockNotifyTasksFromWorkflowMutationFn: func(_ *persistence.WorkflowMutation, _ events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError, "case: updateWorkflowExecution failed")
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(_ *persistence.WorkflowSnapshot, _ events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError, "case: updateWorkflowExecution failed")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "success",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			updateMode:                       persistence.UpdateWorkflowModeUpdateCurrent,
			currentWorkflowTransactionPolicy: TransactionPolicyActive,
			newWorkflowTransactionPolicy:     TransactionPolicyActive.Ptr(),
			workflowRequestMode:              persistence.CreateWorkflowRequestModeReplicated,
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockEngine *engine.MockEngine) {

				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), TransactionPolicyActive).Return(&persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
						State:      persistence.WorkflowStateCompleted,
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("branchtoken"),
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 1,
									Version: 1,
								},
							},
						},
					},
				})
				mockMutableState.EXPECT().GetNextEventID().Return(int64(11))
				mockMutableState.EXPECT().SetHistorySize(int64(5))
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), TransactionPolicyActive).Return(&persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id2",
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateCompleted, persistence.WorkflowCloseStatusCompleted)
				mockShard.EXPECT().GetEngine().Return(mockEngine)
				mockEngine.EXPECT().NotifyNewHistoryEvent(gomock.Any())
				mockMutableState.EXPECT().GetLastFirstEventID().Return(int64(1))
				mockMutableState.EXPECT().GetNextEventID().Return(int64(10))
				mockMutableState.EXPECT().GetPreviousStartedEventID().Return(int64(12))
				mockMutableState.EXPECT().GetNextEventID().Return(int64(20))
				mockMutableState.EXPECT().GetCompletionEvent(gomock.Any()).Return(&types.HistoryEvent{
					ID: 123,
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				assert.Equal(t, &persistence.WorkflowEvents{
					Events: []*types.HistoryEvent{
						{
							ID: 2,
						},
					},
					BranchToken: []byte{1, 2, 3},
				}, history, "case: success")
				return events.PersistedBlob{
					DataBlob: persistence.DataBlob{
						Data: []byte{1, 2, 3, 4, 5},
					},
				}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				assert.Equal(t, &persistence.WorkflowEvents{
					Events: []*types.HistoryEvent{
						{
							ID: constants.FirstEventID,
						},
					},
					BranchToken: []byte{4},
				}, history, "case: success")
				return events.PersistedBlob{
					DataBlob: persistence.DataBlob{
						Data: []byte{4, 5},
					},
				}, nil
			},
			mockMergeContinueAsNewReplicationTasksFn: func(updateMode persistence.UpdateWorkflowMode, currentWorkflow *persistence.WorkflowMutation, newWorkflow *persistence.WorkflowSnapshot) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.Equal(t, &persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
						State:      persistence.WorkflowStateCompleted,
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: 5,
					},
				}, currentWorkflow, "case: success")
				assert.Equal(t, &persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id2",
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: 2,
					},
				}, newWorkflow, "case: success")
				return nil
			},
			mockUpdateWorkflowExecutionEventReapplyFn: func(updateMode persistence.UpdateWorkflowMode, currentEvents []*persistence.WorkflowEvents, newEvents []*persistence.WorkflowEvents) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.Equal(t, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, currentEvents, "case: success")
				assert.Equal(t, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, newEvents, "case: success")
				return nil
			},
			mockUpdateWorkflowExecutionFn: func(_ context.Context, req *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error) {
				assert.Equal(t, &persistence.UpdateWorkflowExecutionRequest{
					Mode: persistence.UpdateWorkflowModeUpdateCurrent,
					UpdateWorkflowMutation: persistence.WorkflowMutation{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
							State:      persistence.WorkflowStateCompleted,
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 5,
						},
					},
					NewWorkflowSnapshot: &persistence.WorkflowSnapshot{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id2",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 2,
						},
					},
					WorkflowRequestMode: persistence.CreateWorkflowRequestModeReplicated,
					DomainName:          "test-domain",
				}, req, "case: success")
				return &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil
			},
			mockNotifyTasksFromWorkflowMutationFn: func(currentWorkflow *persistence.WorkflowMutation, currentEvents events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, &persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
						State:      persistence.WorkflowStateCompleted,
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: 5,
					},
				}, currentWorkflow, "case: success")
				assert.Equal(t, events.PersistedBlobs{
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2, 3, 4, 5},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{4, 5},
						},
					},
				}, currentEvents, "case: success")
				assert.Equal(t, false, persistenceError)
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(newWorkflow *persistence.WorkflowSnapshot, newEvents events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, &persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id2",
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: 2,
					},
				}, newWorkflow, "case: success")
				assert.Equal(t, events.PersistedBlobs{
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2, 3, 4, 5},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{4, 5},
						},
					},
				}, newEvents, "case: success")
				assert.Equal(t, false, persistenceError, "case: success")
			},
			mockEmitWorkflowHistoryStatsFn: func(domainName string, size int, count int) {
				assert.Equal(t, 5, size, "case: success")
				assert.Equal(t, 19, count, "case: success")
			},
			mockEmitSessionUpdateStatsFn: func(domainName string, stats *persistence.MutableStateUpdateSessionStats) {
				assert.Equal(t, &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				}, stats, "case: success")
			},
			mockEmitLargeWorkflowShardIDStatsFn: func(blobSize int64, oldHistoryCount int64, oldHistorySize int64, newHistoryCount int64) {
				assert.Equal(t, int64(5), blobSize, "case: success")
				assert.Equal(t, int64(10), oldHistoryCount, "case: success")
				assert.Equal(t, int64(0), oldHistorySize, "case: success")
				assert.Equal(t, int64(11), newHistoryCount, "case: success")
			},
			mockEmitWorkflowCompletionStatsFn: func(domainName string, workflowType string, workflowID string, runID string, taskList string, lastEvent *types.HistoryEvent) {
				assert.Equal(t, &types.HistoryEvent{
					ID: 123,
				}, lastEvent, "case: success")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockMutableState := NewMockMutableState(mockCtrl)
			mockNewMutableState := NewMockMutableState(mockCtrl)
			mockEngine := engine.NewMockEngine(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache, mockMutableState, mockNewMutableState, mockEngine)
			}
			ctx := &contextImpl{
				logger:                                testlogger.New(t),
				shard:                                 mockShard,
				mutableState:                          mockMutableState,
				stats:                                 &persistence.ExecutionStats{},
				metricsClient:                         metrics.NewNoopMetricsClient(),
				persistNonStartWorkflowBatchEventsFn:  tc.mockPersistNonStartWorkflowBatchEventsFn,
				persistStartWorkflowBatchEventsFn:     tc.mockPersistStartWorkflowBatchEventsFn,
				updateWorkflowExecutionFn:             tc.mockUpdateWorkflowExecutionFn,
				notifyTasksFromWorkflowMutationFn:     tc.mockNotifyTasksFromWorkflowMutationFn,
				notifyTasksFromWorkflowSnapshotFn:     tc.mockNotifyTasksFromWorkflowSnapshotFn,
				emitSessionUpdateStatsFn:              tc.mockEmitSessionUpdateStatsFn,
				emitWorkflowHistoryStatsFn:            tc.mockEmitWorkflowHistoryStatsFn,
				mergeContinueAsNewReplicationTasksFn:  tc.mockMergeContinueAsNewReplicationTasksFn,
				updateWorkflowExecutionEventReapplyFn: tc.mockUpdateWorkflowExecutionEventReapplyFn,
				emitLargeWorkflowShardIDStatsFn:       tc.mockEmitLargeWorkflowShardIDStatsFn,
				emitWorkflowCompletionStatsFn:         tc.mockEmitWorkflowCompletionStatsFn,
			}
			err := ctx.UpdateWorkflowExecutionWithNew(context.Background(), time.Unix(0, 0), tc.updateMode, tc.newContext, mockNewMutableState, tc.currentWorkflowTransactionPolicy, tc.newWorkflowTransactionPolicy, tc.workflowRequestMode)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConflictResolveWorkflowExecution(t *testing.T) {
	testCases := []struct {
		name                                               string
		conflictResolveMode                                persistence.ConflictResolveWorkflowMode
		newContext                                         Context
		currentContext                                     Context
		currentWorkflowTransactionPolicy                   *TransactionPolicy
		mockSetup                                          func(*shard.MockContext, *cache.MockDomainCache, *MockMutableState, *MockMutableState, *MockMutableState, *engine.MockEngine)
		mockPersistNonStartWorkflowBatchEventsFn           func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		mockPersistStartWorkflowBatchEventsFn              func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error)
		mockUpdateWorkflowExecutionFn                      func(context.Context, *persistence.UpdateWorkflowExecutionRequest) (*persistence.UpdateWorkflowExecutionResponse, error)
		mockNotifyTasksFromWorkflowMutationFn              func(*persistence.WorkflowMutation, events.PersistedBlobs, bool)
		mockNotifyTasksFromWorkflowSnapshotFn              func(*persistence.WorkflowSnapshot, events.PersistedBlobs, bool)
		mockEmitSessionUpdateStatsFn                       func(string, *persistence.MutableStateUpdateSessionStats)
		mockEmitWorkflowHistoryStatsFn                     func(string, int, int)
		mockEmitLargeWorkflowShardIDStatsFn                func(int64, int64, int64, int64)
		mockEmitWorkflowCompletionStatsFn                  func(string, string, string, string, string, *types.HistoryEvent)
		mockMergeContinueAsNewReplicationTasksFn           func(persistence.UpdateWorkflowMode, *persistence.WorkflowMutation, *persistence.WorkflowSnapshot) error
		mockConflictResolveWorkflowExecutionEventReapplyFn func(persistence.ConflictResolveWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error
		wantErr                                            bool
		assertErr                                          func(*testing.T, error)
	}{
		{
			name: "resetMutableState CloseTransactionAsSnapshot failed",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "persistNonStartWorkflowEvents failed",
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "newMutableState CloseTransactionAsSnapshot failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "both reset workflow and new workflow generate workflow requests",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					EnableStrongIdempotencySanityCheck: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				})
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{
					WorkflowRequests: []*persistence.WorkflowRequest{
						{
							RequestType: persistence.WorkflowRequestTypeStart,
							RequestID:   "test",
							Version:     1,
						},
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{
					WorkflowRequests: []*persistence.WorkflowRequest{
						{
							RequestType: persistence.WorkflowRequestTypeStart,
							RequestID:   "test",
							Version:     1,
						},
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "workflow requests are only expected to be generated from either reset workflow or continue-as-new workflow for ConflictResolveWorkflowExecution"}, err)
			},
		},
		{
			name: "persistStartWorkflowEvents failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "currentMutableState CloseTransactionAsMutation failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("some error"))
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "currentMutableState generates workflow requests",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().GetConfig().Return(&config.Config{
					EnableStrongIdempotencySanityCheck: dynamicproperties.GetBoolPropertyFnFilteredByDomain(true),
				})
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{
					WorkflowRequests: []*persistence.WorkflowRequest{
						{
							RequestType: persistence.WorkflowRequestTypeStart,
							RequestID:   "test",
							Version:     1,
						},
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{5, 6},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				if history.BranchToken[0] == 1 {
					return events.PersistedBlob{}, nil
				}
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, &types.InternalServiceError{Message: "workflow requests are not expected from current workflow for ConflictResolveWorkflowExecution"}, err)
			},
		},
		{
			name: "currentMutableState persistNonStartWorkflowEvents failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{5, 6},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				if history.BranchToken[0] == 1 {
					return events.PersistedBlob{}, nil
				}
				return events.PersistedBlob{}, errors.New("some error")
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "conflictResolveEventReapply failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{5, 6},
					},
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockConflictResolveWorkflowExecutionEventReapplyFn: func(persistence.ConflictResolveWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error {
				return errors.New("some error")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "ConflictResolveWorkflowExecution failed",
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{5, 6},
					},
				}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(context.Context, *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				return events.PersistedBlob{}, nil
			},
			mockConflictResolveWorkflowExecutionEventReapplyFn: func(persistence.ConflictResolveWorkflowMode, []*persistence.WorkflowEvents, []*persistence.WorkflowEvents) error {
				return nil
			},
			mockNotifyTasksFromWorkflowMutationFn: func(currentWorkflow *persistence.WorkflowMutation, currentEvents events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError, "case: ConflictResolveWorkflowExecution failed")
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(newWorkflow *persistence.WorkflowSnapshot, newEvents events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, true, persistenceError, "case: ConflictResolveWorkflowExecution failed")
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name:                "ConflictResolveWorkflowExecution success",
			conflictResolveMode: persistence.ConflictResolveWorkflowModeUpdateCurrent,
			newContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentContext: &contextImpl{
				stats:         &persistence.ExecutionStats{},
				metricsClient: metrics.NewNoopMetricsClient(),
			},
			currentWorkflowTransactionPolicy: TransactionPolicyActive.Ptr(),
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResetMutableState *MockMutableState, mockNewMutableState *MockMutableState, mockMutableState *MockMutableState, mockEngine *engine.MockEngine) {
				mockResetMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id",
						State:      persistence.WorkflowStateCompleted,
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, nil)
				mockNewMutableState.EXPECT().CloseTransactionAsSnapshot(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowSnapshot{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id2",
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, nil)
				mockMutableState.EXPECT().CloseTransactionAsMutation(gomock.Any(), gomock.Any()).Return(&persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id0",
					},
				}, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 2,
							},
						},
						BranchToken: []byte{5, 6},
					},
				}, nil)
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
				mockShard.EXPECT().ConflictResolveWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.ConflictResolveWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{
						MutableStateSize: 123,
					},
				}, nil)
				mockResetMutableState.EXPECT().GetVersionHistories().Return(&persistence.VersionHistories{
					CurrentVersionHistoryIndex: 0,
					Histories: []*persistence.VersionHistory{
						{
							BranchToken: []byte("123"),
							Items: []*persistence.VersionHistoryItem{
								{
									EventID: 1,
									Version: 1,
								},
							},
						},
					},
				})
				mockResetMutableState.EXPECT().GetWorkflowStateCloseStatus().Return(persistence.WorkflowStateCompleted, persistence.WorkflowCloseStatusCompleted)
				mockShard.EXPECT().GetEngine().Return(mockEngine)
				mockEngine.EXPECT().NotifyNewHistoryEvent(gomock.Any())
				mockResetMutableState.EXPECT().GetLastFirstEventID().Return(int64(123))
				mockResetMutableState.EXPECT().GetNextEventID().Return(int64(456))
				mockResetMutableState.EXPECT().GetPreviousStartedEventID().Return(int64(789))
				mockResetMutableState.EXPECT().GetNextEventID().Return(int64(1111))
				mockResetMutableState.EXPECT().GetCompletionEvent(gomock.Any()).Return(&types.HistoryEvent{
					ID: 123,
				}, nil)
			},
			mockPersistNonStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				if history.BranchToken[0] == 1 {
					assert.Equal(t, &persistence.WorkflowEvents{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					}, history, "case: success")
					return events.PersistedBlob{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2, 3, 4, 5},
						},
					}, nil
				}

				assert.Equal(t, &persistence.WorkflowEvents{
					Events: []*types.HistoryEvent{
						{
							ID: 2,
						},
					},
					BranchToken: []byte{5, 6},
				}, history, "case: success")
				return events.PersistedBlob{
					DataBlob: persistence.DataBlob{
						Data: []byte{1, 2},
					},
				}, nil
			},
			mockPersistStartWorkflowBatchEventsFn: func(_ context.Context, history *persistence.WorkflowEvents) (events.PersistedBlob, error) {
				assert.Equal(t, &persistence.WorkflowEvents{
					Events: []*types.HistoryEvent{
						{
							ID: constants.FirstEventID,
						},
					},
					BranchToken: []byte{4},
				}, history, "case: success")
				return events.PersistedBlob{
					DataBlob: persistence.DataBlob{
						Data: []byte{3, 2},
					},
				}, nil
			},
			mockConflictResolveWorkflowExecutionEventReapplyFn: func(mode persistence.ConflictResolveWorkflowMode, resetEvents []*persistence.WorkflowEvents, newEvents []*persistence.WorkflowEvents) error {
				assert.Equal(t, persistence.ConflictResolveWorkflowModeUpdateCurrent, mode, "case: success")
				assert.Equal(t, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: 1,
							},
						},
						BranchToken: []byte{1, 2, 3},
					},
				}, resetEvents, "case: success")
				assert.Equal(t, []*persistence.WorkflowEvents{
					{
						Events: []*types.HistoryEvent{
							{
								ID: constants.FirstEventID,
							},
						},
						BranchToken: []byte{4},
					},
				}, newEvents, "case: success")
				return nil
			},
			mockNotifyTasksFromWorkflowMutationFn: func(currentWorkflow *persistence.WorkflowMutation, currentEvents events.PersistedBlobs, persistenceError bool) {
				assert.Equal(t, &persistence.WorkflowMutation{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
						RunID:      "test-run-id0",
					},
					ExecutionStats: &persistence.ExecutionStats{
						HistorySize: 2,
					},
				}, currentWorkflow, "case: success")
				assert.Equal(t, events.PersistedBlobs{
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2, 3, 4, 5},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{3, 2},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2},
						},
					},
				}, currentEvents, "case: success")
				assert.Equal(t, false, persistenceError, "case: success")
			},
			mockNotifyTasksFromWorkflowSnapshotFn: func(newWorkflow *persistence.WorkflowSnapshot, newEvents events.PersistedBlobs, persistenceError bool) {
				if newWorkflow.ExecutionInfo.RunID == "test-run-id" {
					assert.Equal(t, &persistence.WorkflowSnapshot{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
							State:      persistence.WorkflowStateCompleted,
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 5,
						},
					}, newWorkflow, "case: success")
				} else {
					assert.Equal(t, &persistence.WorkflowSnapshot{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id2",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 2,
						},
					}, newWorkflow, "case: success")
				}
				assert.Equal(t, events.PersistedBlobs{
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2, 3, 4, 5},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{3, 2},
						},
					},
					{
						DataBlob: persistence.DataBlob{
							Data: []byte{1, 2},
						},
					},
				}, newEvents, "case: success")
				assert.Equal(t, false, persistenceError, "case: success")
			},
			mockEmitWorkflowHistoryStatsFn: func(domainName string, size int, count int) {
				assert.Equal(t, 5, size, "case: success")
				assert.Equal(t, 1110, count, "case: success")
			},
			mockEmitSessionUpdateStatsFn: func(domainName string, stats *persistence.MutableStateUpdateSessionStats) {
				assert.Equal(t, &persistence.MutableStateUpdateSessionStats{
					MutableStateSize: 123,
				}, stats, "case: success")
			},
			mockEmitWorkflowCompletionStatsFn: func(domainName string, workflowType string, workflowID string, runID string, taskList string, lastEvent *types.HistoryEvent) {
				assert.Equal(t, &types.HistoryEvent{
					ID: 123,
				}, lastEvent, "case: success")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockResetMutableState := NewMockMutableState(mockCtrl)
			mockMutableState := NewMockMutableState(mockCtrl)
			mockNewMutableState := NewMockMutableState(mockCtrl)
			mockEngine := engine.NewMockEngine(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache, mockResetMutableState, mockNewMutableState, mockMutableState, mockEngine)
			}
			ctx := &contextImpl{
				logger:                               testlogger.New(t),
				shard:                                mockShard,
				stats:                                &persistence.ExecutionStats{},
				metricsClient:                        metrics.NewNoopMetricsClient(),
				persistNonStartWorkflowBatchEventsFn: tc.mockPersistNonStartWorkflowBatchEventsFn,
				persistStartWorkflowBatchEventsFn:    tc.mockPersistStartWorkflowBatchEventsFn,
				updateWorkflowExecutionFn:            tc.mockUpdateWorkflowExecutionFn,
				notifyTasksFromWorkflowMutationFn:    tc.mockNotifyTasksFromWorkflowMutationFn,
				notifyTasksFromWorkflowSnapshotFn:    tc.mockNotifyTasksFromWorkflowSnapshotFn,
				emitSessionUpdateStatsFn:             tc.mockEmitSessionUpdateStatsFn,
				emitWorkflowHistoryStatsFn:           tc.mockEmitWorkflowHistoryStatsFn,
				mergeContinueAsNewReplicationTasksFn: tc.mockMergeContinueAsNewReplicationTasksFn,
				conflictResolveEventReapplyFn:        tc.mockConflictResolveWorkflowExecutionEventReapplyFn,
				emitLargeWorkflowShardIDStatsFn:      tc.mockEmitLargeWorkflowShardIDStatsFn,
				emitWorkflowCompletionStatsFn:        tc.mockEmitWorkflowCompletionStatsFn,
			}
			err := ctx.ConflictResolveWorkflowExecution(context.Background(), time.Unix(0, 0), tc.conflictResolveMode, mockResetMutableState, tc.newContext, mockNewMutableState, tc.currentContext, mockMutableState, tc.currentWorkflowTransactionPolicy)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReapplyEvents(t *testing.T) {
	testCases := []struct {
		name         string
		eventBatches []*persistence.WorkflowEvents
		mockSetup    func(*shard.MockContext, *cache.MockDomainCache, *resource.Test, *engine.MockEngine, *activecluster.MockManager)
		wantErr      bool
	}{
		{
			name:         "empty input",
			eventBatches: []*persistence.WorkflowEvents{},
			wantErr:      false,
		},
		{
			name: "domain cache error",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID: "test-domain-id",
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, _ *resource.Test, _ *engine.MockEngine, _ *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "domain is pending active",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID: "test-domain-id",
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, _ *resource.Test, _ *engine.MockEngine, _ *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(cache.NewDomainCacheEntryForTest(nil, nil, true, nil, 0, common.Ptr(int64(1)), 0, 0, 0), nil)
			},
			wantErr: false,
		},
		{
			name: "domainID/workflowID mismatch",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID: "test-domain-id",
				},
				{
					DomainID: "test-domain-id2",
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, _ *resource.Test, _ *engine.MockEngine, _ *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(cache.NewDomainCacheEntryForTest(nil, nil, true, nil, 0, nil, 0, 0, 0), nil)
			},
			wantErr: true,
		},
		{
			name: "no signal events",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID: "test-domain-id",
				},
				{
					DomainID: "test-domain-id",
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, _ *resource.Test, _ *engine.MockEngine, _ *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(cache.NewDomainCacheEntryForTest(nil, nil, true, nil, 0, nil, 0, 0, 0), nil)
			},
			wantErr: false,
		},
		{
			name: "lookup workflow error",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					Events: []*types.HistoryEvent{
						{
							EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
						},
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResource *resource.Test, mockEngine *engine.MockEngine, mockActiveClusterManager *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(
					cache.NewGlobalDomainCacheEntryForTest(nil, nil, &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"region1": {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   1,
										},
									},
								},
							},
						},
					}, 0),
					nil)
				mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager)
				mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").Return(
					nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success - active-active apply to current cluster",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					Events: []*types.HistoryEvent{
						{
							EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
						},
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResource *resource.Test, mockEngine *engine.MockEngine, mockActiveClusterManager *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(
					cache.NewGlobalDomainCacheEntryForTest(nil, nil, &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"region1": {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   1,
										},
									},
								},
							},
						},
					}, 0),
					nil)
				mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager)
				mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").Return(
					&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestCurrentClusterName,
					}, nil)
				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata)
				mockShard.EXPECT().GetEngine().Return(mockEngine)
				mockEngine.EXPECT().ReapplyEvents(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id", []*types.HistoryEvent{
					{
						EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
					},
				}).Return(nil)
			},
		},
		{
			name: "success - active-active apply to remote cluster",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					Events: []*types.HistoryEvent{
						{
							EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
						},
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResource *resource.Test, mockEngine *engine.MockEngine, mockActiveClusterManager *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(
					cache.NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "test-domain"}, nil, &persistence.DomainReplicationConfig{
						ActiveClusterName: cluster.TestCurrentClusterName,
						ActiveClusters: &types.ActiveClusters{
							AttributeScopes: map[string]types.ClusterAttributeScope{
								"region": {
									ClusterAttributes: map[string]types.ActiveClusterInfo{
										"region1": {
											ActiveClusterName: cluster.TestCurrentClusterName,
											FailoverVersion:   1,
										},
									},
								},
							},
						},
					}, 0), nil)
				mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager)
				mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").Return(
					&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestAlternativeClusterName,
					}, nil)
				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata)
				mockShard.EXPECT().GetService().Return(mockResource).Times(2)
				mockResource.RemoteAdminClient.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "success - apply to current cluster",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					Events: []*types.HistoryEvent{
						{
							EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
						},
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, _ *resource.Test, mockEngine *engine.MockEngine, mockActiveClusterManager *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(cache.NewGlobalDomainCacheEntryForTest(nil, nil, &persistence.DomainReplicationConfig{ActiveClusterName: cluster.TestCurrentClusterName}, 0), nil)
				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata)
				mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager)
				mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").Return(
					&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestCurrentClusterName,
					}, nil)
				mockShard.EXPECT().GetEngine().Return(mockEngine)
				mockEngine.EXPECT().ReapplyEvents(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id", []*types.HistoryEvent{
					{
						EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
					},
				}).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "success - apply to remote cluster",
			eventBatches: []*persistence.WorkflowEvents{
				{
					DomainID:   "test-domain-id",
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
					Events: []*types.HistoryEvent{
						{
							EventType: types.EventTypeWorkflowExecutionSignaled.Ptr(),
						},
					},
				},
			},
			mockSetup: func(mockShard *shard.MockContext, mockDomainCache *cache.MockDomainCache, mockResource *resource.Test, mockEngine *engine.MockEngine, mockActiveClusterManager *activecluster.MockManager) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID("test-domain-id").Return(cache.NewGlobalDomainCacheEntryForTest(&persistence.DomainInfo{Name: "test-domain"}, nil, &persistence.DomainReplicationConfig{ActiveClusterName: cluster.TestAlternativeClusterName}, 0), nil)
				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata)
				mockShard.EXPECT().GetService().Return(mockResource).Times(2)
				mockShard.EXPECT().GetActiveClusterManager().Return(mockActiveClusterManager)
				mockActiveClusterManager.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), "test-domain-id", "test-workflow-id", "test-run-id").Return(
					&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestAlternativeClusterName,
					}, nil)
				mockResource.RemoteAdminClient.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockEngine := engine.NewMockEngine(mockCtrl)
			mockActiveClusterManager := activecluster.NewMockManager(mockCtrl)
			testResource := resource.NewTest(t, mockCtrl, metrics.Common)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockDomainCache, testResource, mockEngine, mockActiveClusterManager)
			}
			ctx := &contextImpl{
				shard: mockShard,
			}
			err := ctx.ReapplyEvents(tc.eventBatches)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetWorkflowExecutionWithRetry(t *testing.T) {
	testCases := []struct {
		name      string
		request   *persistence.GetWorkflowExecutionRequest
		mockSetup func(*shard.MockContext, *log.MockLogger, clock.MockedTimeSource)
		want      *persistence.GetWorkflowExecutionResponse
		wantErr   bool
		assertErr func(*testing.T, error)
	}{
		{
			name: "Success case",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), &persistence.GetWorkflowExecutionRequest{
					RangeID: 100,
				}).Return(&persistence.GetWorkflowExecutionResponse{
					MutableStateStats: &persistence.MutableStateStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.GetWorkflowExecutionResponse{
				MutableStateStats: &persistence.MutableStateStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
		{
			name: "entity not exists error",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.IsType(t, err, &types.EntityNotExistsError{})
			},
		},
		{
			name: "shard closed error recent",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &shard.ErrShardClosed{
					ClosedAt: timeSource.Now().Add(-shard.TimeBeforeShardClosedIsError / 2),
				})
				// We do _not_ expect a log call
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*shard.ErrShardClosed))
			},
		},
		{
			name: "shard closed error",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				err := &shard.ErrShardClosed{
					ClosedAt: timeSource.Now().Add(-shard.TimeBeforeShardClosedIsError * 2),
				}
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, err)
				expectLog(mockLogger, err)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*shard.ErrShardClosed))
			},
		},
		{
			name: "non retryable error",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
				expectLog(mockLogger, errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, errors.New("some error"), err)
			},
		},
		{
			name: "retry succeeds",
			request: &persistence.GetWorkflowExecutionRequest{
				RangeID: 100,
			},
			mockSetup: func(mockShard *shard.MockContext, mockLogger *log.MockLogger, timeSource clock.MockedTimeSource) {
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.ServiceBusyError{})
				mockShard.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).Return(&persistence.GetWorkflowExecutionResponse{
					MutableStateStats: &persistence.MutableStateStats{
						MutableStateSize: 123,
					},
				}, nil)
			},
			want: &persistence.GetWorkflowExecutionResponse{
				MutableStateStats: &persistence.MutableStateStats{
					MutableStateSize: 123,
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockLogger := log.NewMockLogger(gomock.NewController(t))
			mockLogger.EXPECT().Helper().Return(mockLogger)
			timeSource := clock.NewMockedTimeSource()
			mockShard.EXPECT().GetTimeSource().Return(timeSource).AnyTimes()
			policy := backoff.NewExponentialRetryPolicy(time.Millisecond)
			policy.SetMaximumAttempts(1)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockLogger, timeSource)
			}
			resp, err := getWorkflowExecutionWithRetry(context.Background(), mockShard, mockLogger, policy, tc.request)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func expectLog(mockLogger *log.MockLogger, err error) *gomock.Call {
	return mockLogger.EXPECT().Error(
		"Persistent fetch operation failure",
		[]tag.Tag{
			tag.StoreOperationGetWorkflowExecution,
			tag.Error(err),
		})
}

func TestLoadWorkflowExecutionWithTaskVersion(t *testing.T) {
	testCases := []struct {
		name                                 string
		mockSetup                            func(*shard.MockContext, *MockMutableState, *cache.MockDomainCache)
		mockGetWorkflowExecutionFn           func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error)
		mockEmitWorkflowExecutionStatsFn     func(string, *persistence.MutableStateStats, int64)
		mockUpdateWorkflowExecutionWithNewFn func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		wantErr                              bool
	}{
		{
			name: "domain cache failed",
			mockSetup: func(mockShard *shard.MockContext, mockMutableState *MockMutableState, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "getWorkflowExecutionFn failed",
			mockSetup: func(mockShard *shard.MockContext, mockMutableState *MockMutableState, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{
						Name: "test-domain",
					},
					nil,
					true,
					nil,
					0,
					nil, 0, 0, 0), nil)
			},
			mockGetWorkflowExecutionFn: func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
				return nil, errors.New("some error")
			},
			wantErr: true,
		},
		{
			name: "StartTransaction failed",
			mockSetup: func(mockShard *shard.MockContext, mockMutableState *MockMutableState, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{
					Name: "test-domain",
				}, nil, true, nil, 0, nil, 0, 0, 0), nil)
				mockMutableState.EXPECT().Load(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
				mockMutableState.EXPECT().StartTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("some error"))
			},
			mockGetWorkflowExecutionFn: func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
				return &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 123,
						},
					},
				}, nil
			},
			mockEmitWorkflowExecutionStatsFn: func(domainName string, stats *persistence.MutableStateStats, size int64) {
				assert.Equal(t, "test-domain", domainName)
				assert.Equal(t, int64(123), size)
			},
			wantErr: true,
		},
		{
			name: "do not need to flush",
			mockSetup: func(mockShard *shard.MockContext, mockMutableState *MockMutableState, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(
					&persistence.DomainInfo{
						Name: "test-domain",
					},
					nil,
					true,
					nil,
					0,
					nil,
					0,
					0,
					0), nil)
				mockMutableState.EXPECT().Load(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
				mockMutableState.EXPECT().StartTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
			},
			mockGetWorkflowExecutionFn: func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
				return &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 123,
						},
					},
				}, nil
			},
			mockEmitWorkflowExecutionStatsFn: func(domainName string, stats *persistence.MutableStateStats, size int64) {
				assert.Equal(t, "test-domain", domainName)
				assert.Equal(t, int64(123), size)
			},
			wantErr: false,
		},
		{
			name: "flush buffered events",
			mockSetup: func(mockShard *shard.MockContext, mockMutableState *MockMutableState, mockDomainCache *cache.MockDomainCache) {
				mockShard.EXPECT().GetDomainCache().Return(mockDomainCache)
				mockDomainCache.EXPECT().GetDomainByID(gomock.Any()).Return(cache.NewDomainCacheEntryForTest(&persistence.DomainInfo{
					Name: "test-domain",
				}, nil, true, nil, 0, nil, 0, 0, 0), nil)
				mockMutableState.EXPECT().Load(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
				mockMutableState.EXPECT().StartTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				mockMutableState.EXPECT().StartTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				mockShard.EXPECT().GetTimeSource().Return(clock.NewMockedTimeSource())
			},
			mockGetWorkflowExecutionFn: func(context.Context, *persistence.GetWorkflowExecutionRequest) (*persistence.GetWorkflowExecutionResponse, error) {
				return &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:   "test-domain-id",
							WorkflowID: "test-workflow-id",
							RunID:      "test-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{
							HistorySize: 123,
						},
					},
				}, nil
			},
			mockEmitWorkflowExecutionStatsFn: func(domainName string, stats *persistence.MutableStateStats, size int64) {
				assert.Equal(t, "test-domain", domainName)
				assert.Equal(t, int64(123), size)
			},
			mockUpdateWorkflowExecutionWithNewFn: func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(mockCtrl)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockMutableState := NewMockMutableState(mockCtrl)
			if tc.mockSetup != nil {
				tc.mockSetup(mockShard, mockMutableState, mockDomainCache)
			}

			ctx := &contextImpl{
				shard:  mockShard,
				logger: testlogger.New(t),
				createMutableStateFn: func(shard.Context, log.Logger, *cache.DomainCacheEntry) MutableState {
					return mockMutableState
				},
				getWorkflowExecutionFn:           tc.mockGetWorkflowExecutionFn,
				emitWorkflowExecutionStatsFn:     tc.mockEmitWorkflowExecutionStatsFn,
				updateWorkflowExecutionWithNewFn: tc.mockUpdateWorkflowExecutionWithNewFn,
			}
			got, err := ctx.LoadWorkflowExecutionWithTaskVersion(context.Background(), 123)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, mockMutableState, got)
			}
		})
	}
}

func TestUpdateWorkflowExecutionAsActive(t *testing.T) {
	testCases := []struct {
		name                                 string
		mockUpdateWorkflowExecutionWithNewFn func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		wantErr                              bool
	}{
		{
			name: "success",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, workflowRequestMode persistence.CreateWorkflowRequestMode) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.Nil(t, newContext)
				assert.Nil(t, newMutableState)
				assert.Equal(t, TransactionPolicyActive, currentPolicy)
				assert.Nil(t, newPolicy)
				assert.Equal(t, persistence.CreateWorkflowRequestModeNew, workflowRequestMode)
				return nil
			},
			wantErr: false,
		},
		{
			name: "error case",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, _ persistence.CreateWorkflowRequestMode) error {
				return errors.New("some error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &contextImpl{
				updateWorkflowExecutionWithNewFn: tc.mockUpdateWorkflowExecutionWithNewFn,
				logger:                           testlogger.New(t),
				workflowExecution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			}
			err := ctx.UpdateWorkflowExecutionAsActive(context.Background(), time.Now())
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateWorkflowExecutionWithNewAsActive(t *testing.T) {
	testCases := []struct {
		name                                 string
		mockUpdateWorkflowExecutionWithNewFn func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		wantErr                              bool
	}{
		{
			name: "success",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, workflowRequestMode persistence.CreateWorkflowRequestMode) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.NotNil(t, newContext)
				assert.NotNil(t, newMutableState)
				assert.Equal(t, TransactionPolicyActive, currentPolicy)
				assert.Equal(t, TransactionPolicyActive.Ptr(), newPolicy)
				assert.Equal(t, persistence.CreateWorkflowRequestModeNew, workflowRequestMode)
				return nil
			},
			wantErr: false,
		},
		{
			name: "error case",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, _ persistence.CreateWorkflowRequestMode) error {
				return errors.New("some error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &contextImpl{
				updateWorkflowExecutionWithNewFn: tc.mockUpdateWorkflowExecutionWithNewFn,
				logger:                           testlogger.New(t),
				workflowExecution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			}
			err := ctx.UpdateWorkflowExecutionWithNewAsActive(context.Background(), time.Now(), &contextImpl{}, &mutableStateBuilder{})
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateWorkflowExecutionAsPassive(t *testing.T) {
	testCases := []struct {
		name                                 string
		mockUpdateWorkflowExecutionWithNewFn func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		wantErr                              bool
	}{
		{
			name: "success",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, workflowRequestMode persistence.CreateWorkflowRequestMode) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.Nil(t, newContext)
				assert.Nil(t, newMutableState)
				assert.Equal(t, TransactionPolicyPassive, currentPolicy)
				assert.Nil(t, newPolicy)
				assert.Equal(t, persistence.CreateWorkflowRequestModeReplicated, workflowRequestMode)
				return nil
			},
			wantErr: false,
		},
		{
			name: "error case",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, _ persistence.CreateWorkflowRequestMode) error {
				return errors.New("some error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &contextImpl{
				updateWorkflowExecutionWithNewFn: tc.mockUpdateWorkflowExecutionWithNewFn,
				logger:                           testlogger.New(t),
				workflowExecution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			}
			err := ctx.UpdateWorkflowExecutionAsPassive(context.Background(), time.Now())
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateWorkflowExecutionWithNewAsPassive(t *testing.T) {
	testCases := []struct {
		name                                 string
		mockUpdateWorkflowExecutionWithNewFn func(context.Context, time.Time, persistence.UpdateWorkflowMode, Context, MutableState, TransactionPolicy, *TransactionPolicy, persistence.CreateWorkflowRequestMode) error
		wantErr                              bool
	}{
		{
			name: "success",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, workflowRequestMode persistence.CreateWorkflowRequestMode) error {
				assert.Equal(t, persistence.UpdateWorkflowModeUpdateCurrent, updateMode)
				assert.NotNil(t, newContext)
				assert.NotNil(t, newMutableState)
				assert.Equal(t, TransactionPolicyPassive, currentPolicy)
				assert.Equal(t, TransactionPolicyPassive.Ptr(), newPolicy)
				assert.Equal(t, persistence.CreateWorkflowRequestModeReplicated, workflowRequestMode)
				return nil
			},
			wantErr: false,
		},
		{
			name: "error case",
			mockUpdateWorkflowExecutionWithNewFn: func(_ context.Context, _ time.Time, updateMode persistence.UpdateWorkflowMode, newContext Context, newMutableState MutableState, currentPolicy TransactionPolicy, newPolicy *TransactionPolicy, _ persistence.CreateWorkflowRequestMode) error {
				return errors.New("some error")
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &contextImpl{
				updateWorkflowExecutionWithNewFn: tc.mockUpdateWorkflowExecutionWithNewFn,
				logger:                           testlogger.New(t),
				workflowExecution: types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      "test-run-id",
				},
			}
			err := ctx.UpdateWorkflowExecutionWithNewAsPassive(context.Background(), time.Now(), &contextImpl{}, &mutableStateBuilder{})
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWorkflowRequestsAndMode(t *testing.T) {
	testCases := []struct {
		name     string
		requests []*persistence.WorkflowRequest
		mode     persistence.CreateWorkflowRequestMode
		wantErr  bool
	}{
		{
			name:    "Success - replicated mode",
			mode:    persistence.CreateWorkflowRequestModeReplicated,
			wantErr: false,
		},
		{
			name: "Success - new mode",
			requests: []*persistence.WorkflowRequest{
				{
					RequestType: persistence.WorkflowRequestTypeCancel,
					RequestID:   "abc",
					Version:     1,
				},
			},
			mode:    persistence.CreateWorkflowRequestModeNew,
			wantErr: false,
		},
		{
			name: "Success - new mode, signal with start",
			requests: []*persistence.WorkflowRequest{
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
				{
					RequestType: persistence.WorkflowRequestTypeStart,
					RequestID:   "abc",
					Version:     1,
				},
			},
			mode:    persistence.CreateWorkflowRequestModeNew,
			wantErr: false,
		},
		{
			name: "too many requests",
			requests: []*persistence.WorkflowRequest{
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
			},
			mode:    persistence.CreateWorkflowRequestModeNew,
			wantErr: true,
		},
		{
			name: "too many requests",
			requests: []*persistence.WorkflowRequest{
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
				{
					RequestType: persistence.WorkflowRequestTypeSignal,
					RequestID:   "abc",
					Version:     1,
				},
			},
			mode:    persistence.CreateWorkflowRequestModeNew,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWorkflowRequestsAndMode(tc.requests, tc.mode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
