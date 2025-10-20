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

	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

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
