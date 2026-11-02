// Copyright (c) 2017-2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2021 Temporal Technologies Inc.
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

package engineimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
)

func TestRefreshWorkflowTasks(t *testing.T) {
	tests := []struct {
		name              string
		execution         types.WorkflowExecution
		getWFExecErr      error
		readHistBranchErr error
		updateWFExecErr   error
		wantErr           bool
	}{
		{
			name: "runid is not uuid",
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      "not-a-uuid",
			},
			wantErr: true,
		},
		{
			name:         "failed to get workflow execution",
			getWFExecErr: errors.New("some random error"),
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			wantErr: true,
		},
		{
			name:              "failed to get workflow start event",
			readHistBranchErr: errors.New("some random error"),
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			wantErr: true,
		},
		{
			name: "failed to update workflow execution",
			// returning TimeoutError because it doesn't get retried
			updateWFExecErr: &persistence.TimeoutError{},
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			wantErr: true,
		},
		{
			name: "success",
			execution: types.WorkflowExecution{
				WorkflowID: constants.TestWorkflowID,
				RunID:      constants.TestRunID,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			// GetWorkflowExecution prep
			getExecReq := &persistence.GetWorkflowExecutionRequest{
				ShardID:    common.Ptr(0),
				DomainID:   constants.TestDomainID,
				Execution:  tc.execution,
				DomainName: constants.TestDomainName,
				RangeID:    1,
			}
			getExecResp := &persistence.GetWorkflowExecutionResponse{
				State: &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					ExecutionStats: &persistence.ExecutionStats{},
				},
				MutableStateStats: &persistence.MutableStateStats{},
			}
			eft.ShardCtx.Resource.ExecutionMgr.
				On("GetWorkflowExecution", mock.Anything, getExecReq).
				Return(getExecResp, tc.getWFExecErr).
				Once()

			eft.ShardCtx.Resource.ActiveClusterMgr.
				EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, tc.execution.WorkflowID, tc.execution.RunID).
				Return(&types.ActiveClusterInfo{ActiveClusterName: "test-active-cluster"}, nil).MaxTimes(1)

			// ReadHistoryBranch prep
			historyBranchResp := &persistence.ReadHistoryBranchResponse{
				HistoryEvents: []*types.HistoryEvent{
					// first event.
					{
						ID:                                      1,
						WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
					},
				},
			}
			historyMgr := eft.ShardCtx.Resource.HistoryMgr
			historyMgr.
				On("ReadHistoryBranch", mock.Anything, mock.Anything).
				Return(historyBranchResp, tc.readHistBranchErr).
				Once()

			// UpdateWorkflowExecution prep
			var gotUpdateExecReq *persistence.UpdateWorkflowExecutionRequest
			updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
				MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
			}
			eft.ShardCtx.Resource.ExecutionMgr.
				On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					var ok bool
					gotUpdateExecReq, ok = args.Get(1).(*persistence.UpdateWorkflowExecutionRequest)
					if !ok {
						t.Fatalf("failed to cast input to *persistence.UpdateWorkflowExecutionRequest, type is %T", args.Get(1))
					}
				}).
				Return(updateExecResp, tc.updateWFExecErr).
				Once()

			// UpdateShard prep. this is needed to update the shard's rangeID for failure cases.
			eft.ShardCtx.Resource.ShardMgr.
				On("UpdateShard", mock.Anything, mock.Anything).
				Return(nil)

			// Call RefreshWorkflowTasks
			err := eft.Engine.RefreshWorkflowTasks(
				context.Background(),
				constants.TestDomainID,
				tc.execution,
				nil,
			)

			// Error validations
			if (err != nil) != tc.wantErr {
				t.Fatalf("RefreshWorkflowTasks() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}

			// UpdateWorkflowExecutionRequest validations
			if gotUpdateExecReq == nil {
				t.Fatal("UpdateWorkflowExecutionRequest is nil")
			}

			if gotUpdateExecReq.RangeID != 1 {
				t.Errorf("got RangeID %v, want 1", gotUpdateExecReq.RangeID)
			}

			if gotUpdateExecReq.DomainName != constants.TestDomainName {
				t.Errorf("got DomainName %v, want %v", gotUpdateExecReq.DomainName, constants.TestDomainName)
			}

			if gotUpdateExecReq.Mode != persistence.UpdateWorkflowModeIgnoreCurrent {
				t.Errorf("got Mode %v, want %v", gotUpdateExecReq.Mode, persistence.UpdateWorkflowModeIgnoreCurrent)
			}

			if len(gotUpdateExecReq.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer]) != 2 {
				t.Errorf("got %v TimerTasks, want 2", len(gotUpdateExecReq.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer]))
			} else {
				timer0, ok := gotUpdateExecReq.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer][0].(*persistence.WorkflowTimeoutTask)
				if !ok {
					t.Fatalf("failed to cast TimerTask[0] to *persistence.WorkflowTimeoutTask, type is %T", timer0)
				}

				timer1, ok := gotUpdateExecReq.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer][1].(*persistence.DecisionTimeoutTask)
				if !ok {
					t.Fatalf("failed to cast TimerTask[0] to *persistence.WorkflowTimeoutTask, type is %T", timer1)
				}
			}
		})
	}
}

func TestRefreshWorkflowTasks_Scopes(t *testing.T) {
	const lastWriteVersion = int64(10)
	recordChildExecutionCompleted := []*types.RefreshWorkflowTasksScope{
		{ScopeType: types.RefreshWorkflowTasksScopeTypeRecordChildExecutionCompleted.Ptr()},
	}
	execution := types.WorkflowExecution{
		WorkflowID: constants.TestWorkflowID,
		RunID:      constants.TestRunID,
	}

	tests := []struct {
		name          string
		scopes        []*types.RefreshWorkflowTasksScope
		running       bool
		withoutParent bool
		activeCluster string
		wantLoad      bool
		wantErr       error
		wantTask      bool
	}{
		{
			name:          "closed child active on this cluster: only the record child execution completed task",
			scopes:        recordChildExecutionCompleted,
			activeCluster: cluster.TestCurrentClusterName,
			wantLoad:      true,
			wantTask:      true,
		},
		{
			name:          "closed child active on another cluster: domain not active",
			scopes:        recordChildExecutionCompleted,
			activeCluster: cluster.TestAlternativeClusterName,
			wantLoad:      true,
			wantErr:       &types.DomainNotActiveError{},
		},
		{
			name:          "running child: bad request",
			scopes:        recordChildExecutionCompleted,
			running:       true,
			activeCluster: cluster.TestCurrentClusterName,
			wantLoad:      true,
			wantErr:       &types.BadRequestError{},
		},
		{
			name:          "closed run without a parent: bad request",
			scopes:        recordChildExecutionCompleted,
			withoutParent: true,
			activeCluster: cluster.TestCurrentClusterName,
			wantLoad:      true,
			wantErr:       &types.BadRequestError{},
		},
		{
			// an unknown type never causes a full refresh
			name:    "unknown scope type: bad request",
			scopes:  []*types.RefreshWorkflowTasksScope{{ScopeType: types.RefreshWorkflowTasksScopeType(5).Ptr()}},
			wantErr: &types.BadRequestError{},
		},
		{
			name:    "scope without a type: bad request",
			scopes:  []*types.RefreshWorkflowTasksScope{{}},
			wantErr: &types.BadRequestError{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, execution.WorkflowID, execution.RunID).
				Return(&types.ActiveClusterInfo{ActiveClusterName: tc.activeCluster}, nil).AnyTimes()
			if tc.wantLoad {
				executionInfo := &persistence.WorkflowExecutionInfo{
					DomainID:           constants.TestDomainID,
					WorkflowID:         execution.WorkflowID,
					RunID:              execution.RunID,
					ParentDomainID:     constants.TestDomainID,
					ParentWorkflowID:   "parent-workflow-id",
					ParentRunID:        "parent-run-id",
					InitiatedID:        5,
					State:              persistence.WorkflowStateCompleted,
					CloseStatus:        persistence.WorkflowCloseStatusCompleted,
					NextEventID:        3,
					DecisionScheduleID: commonconstants.EmptyEventID,
					DecisionStartedID:  commonconstants.EmptyEventID,
				}
				if tc.running {
					executionInfo.State = persistence.WorkflowStateRunning
					executionInfo.CloseStatus = persistence.WorkflowCloseStatusNone
				}
				if tc.withoutParent {
					executionInfo.ParentDomainID = ""
					executionInfo.ParentWorkflowID = ""
					executionInfo.ParentRunID = ""
					executionInfo.InitiatedID = commonconstants.EmptyEventID
				}
				eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo:  executionInfo,
						ExecutionStats: &persistence.ExecutionStats{},
						VersionHistories: persistence.NewVersionHistories(persistence.NewVersionHistory(
							[]byte("branch-token"),
							[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(2, lastWriteVersion)},
						)),
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}, nil).Once()
			}
			var gotUpdate *persistence.UpdateWorkflowExecutionRequest
			if tc.wantTask {
				eft.ShardCtx.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						gotUpdate = args.Get(1).(*persistence.UpdateWorkflowExecutionRequest)
					}).
					Return(&persistence.UpdateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
			}

			err := eft.Engine.RefreshWorkflowTasks(context.Background(), constants.TestDomainID, execution, tc.scopes)
			if tc.wantErr != nil {
				assert.IsType(t, tc.wantErr, err)
				eft.ShardCtx.Resource.ExecutionMgr.AssertNotCalled(t, "UpdateWorkflowExecution", mock.Anything, mock.Anything)
			} else {
				assert.NoError(t, err)
			}
			if !tc.wantLoad {
				eft.ShardCtx.Resource.ExecutionMgr.AssertNotCalled(t, "GetWorkflowExecution", mock.Anything, mock.Anything)
			}
			if !tc.wantTask {
				return
			}
			// the refresh writes only the task that sends the completion to the parent
			assert.Empty(t, gotUpdate.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTimer])
			transferTasks := gotUpdate.UpdateWorkflowMutation.TasksByCategory[persistence.HistoryTaskCategoryTransfer]
			if assert.Len(t, transferTasks, 1) {
				task, ok := transferTasks[0].(*persistence.RecordChildExecutionCompletedTask)
				if assert.True(t, ok, "got task type %T", transferTasks[0]) {
					assert.Equal(t, lastWriteVersion, task.Version)
					assert.Equal(t, execution.RunID, task.RunID)
					assert.Equal(t, "parent-workflow-id", task.TargetWorkflowID)
					assert.Equal(t, "parent-run-id", task.TargetRunID)
				}
			}
		})
	}
}
