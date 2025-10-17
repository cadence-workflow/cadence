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
package engineimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
)

func TestTerminateWorkflowExecution(t *testing.T) {
	tests := []struct {
		name               string
		terminationRequest types.HistoryTerminateWorkflowExecutionRequest
		setupMocks         func(*testing.T, *testdata.EngineForTest)
		wantErr            bool
	}{
		{
			name: "domain is not active",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID:        constants.TestDomainID,
				ChildWorkflowOnly: true,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:            constants.TestDomainName,
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:            "Test termination",
					Identity:          "testRunner", // Specifically testing child workflow scenario
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: "aaa"}, nil)
			},
			wantErr: true,
		},
		{
			name: "runid is not uuid",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: "not-a-uuid"},
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, "not-a-uuid").Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.Anything).
					Return(nil, errors.New("invalid UUID")).Once()
			},
			wantErr: true,
		},
		{
			name: "failed to get workflow execution",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				eft.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(nil, errors.New("some random error")).Once()
			},
			wantErr: true,
		},
		{
			name: "child workflow parent mismatch",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID:        constants.TestDomainID,
				ChildWorkflowOnly: true,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:            constants.TestDomainName,
					WorkflowExecution: &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:            "Test termination",
					Identity:          "testRunner", // Specifically testing child workflow scenario
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				// Mock the retrieval of the workflow execution details
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				getExecResp := &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:         constants.TestDomainID,
							WorkflowID:       constants.TestWorkflowID,
							RunID:            constants.TestRunID,
							ParentWorkflowID: "other-parent-id",
							ParentRunID:      "other-parent-runid",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(getExecResp, nil).Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				// Mock the retrieval of the workflow's history branch
				historyBranchResp := &persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						{
							ID:                                      1,
							WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{},
						},
					},
				}
				historyMgr := eft.ShardCtx.Resource.HistoryMgr
				historyMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(historyBranchResp, nil).
					Once()

				// Mock the update of the workflow execution
				var _ *persistence.UpdateWorkflowExecutionRequest
				updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(updateExecResp, &types.EntityNotExistsError{Message: "Workflow execution not found due to parent mismatch"}).
					Once()

				// Mock the update of the shard's range ID
				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)

				// Mock appending history nodes
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "valid first execution run ID",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:              constants.TestDomainName,
					WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:              "Test termination",
					Identity:            "testRunner",
					FirstExecutionRunID: "",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				getExecResp := &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:            constants.TestDomainID,
							WorkflowID:          constants.TestWorkflowID,
							RunID:               constants.TestRunID,
							FirstExecutionRunID: "",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(getExecResp, nil).Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(&persistence.UpdateWorkflowExecutionResponse{}, nil).
					Once()

				// Mock GetCurrentExecution call
				getCurrentExecReq := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				getCurrentExecResp := &persistence.GetCurrentExecutionResponse{
					RunID:       constants.TestRunID,
					State:       persistence.WorkflowStateRunning,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetCurrentExecution", mock.Anything, getCurrentExecReq).
					Return(getCurrentExecResp, nil).Once()

				// Mock the retrieval of the workflow's history branch
				historyBranchResp := &persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						{
							ID: 1,
							WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
								FirstExecutionRunID: "fetched-first-run-id", // The ID to be fetched
							},
						},
					},
				}
				historyMgr := eft.ShardCtx.Resource.HistoryMgr
				historyMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(historyBranchResp, nil).
					Once()

				eft.ShardCtx.Resource.HistoryMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(historyBranchResp, nil).
					Once()
				// Mock the update of the workflow execution
				var _ *persistence.UpdateWorkflowExecutionRequest
				updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(updateExecResp, &types.EntityNotExistsError{Message: "Workflow execution not found due to parent mismatch"}).
					Once()

				// Mock the update of the shard's range ID
				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)

				// Mock appending history nodes
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "successful termination of a running workflow",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain: constants.TestDomainName,
					WorkflowExecution: &types.WorkflowExecution{
						WorkflowID: constants.TestWorkflowID,
						RunID:      constants.TestRunID,
					},
					Reason:   "Test termination",
					Identity: "testRunner",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
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
					Return(getExecResp, nil).
					Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

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
					Return(historyBranchResp, nil).
					Once()
				var _ *persistence.UpdateWorkflowExecutionRequest
				updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						var ok bool
						_, ok = args.Get(1).(*persistence.UpdateWorkflowExecutionRequest)
						if !ok {
							t.Fatalf("failed to cast input to *persistence.UpdateWorkflowExecutionRequest, type is %T", args.Get(1))
						}
					}).
					Return(updateExecResp, nil).
					Once()

				// UpdateShard prep. this is needed to update the shard's rangeID for failure cases.
				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once() // Adjust the return values based on your test case needs
			},
			wantErr: false,
		},
		{
			name: "first execution run ID matches",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:              constants.TestDomainName,
					WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:              "Test termination",
					Identity:            "testRunner",
					FirstExecutionRunID: "matching-first-run-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				getExecResp := &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:            constants.TestDomainID,
							WorkflowID:          constants.TestWorkflowID,
							RunID:               constants.TestRunID,
							FirstExecutionRunID: "matching-first-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(getExecResp, nil).Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, "").Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				getCurrentExecReq := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				getCurrentExecResp := &persistence.GetCurrentExecutionResponse{
					RunID:       constants.TestRunID,
					State:       persistence.WorkflowStateRunning,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetCurrentExecution", mock.Anything, getCurrentExecReq).
					Return(getCurrentExecResp, nil).Once()

				historyMgr := eft.ShardCtx.Resource.HistoryMgr
				historyMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(&persistence.ReadHistoryBranchResponse{}, nil).
					Once()

				updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(updateExecResp, nil).Once()

				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)

				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "first execution run ID does not match",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:              constants.TestDomainName,
					WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:              "Test termination",
					Identity:            "testRunner",
					FirstExecutionRunID: "non-matching-first-run-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				getExecResp := &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:            constants.TestDomainID,
							WorkflowID:          constants.TestWorkflowID,
							RunID:               constants.TestRunID,
							FirstExecutionRunID: "matching-first-run-id",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(getExecResp, nil).Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, "").Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				getCurrentExecReq := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				getCurrentExecResp := &persistence.GetCurrentExecutionResponse{
					RunID:       constants.TestRunID,
					State:       persistence.WorkflowStateRunning,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetCurrentExecution", mock.Anything, getCurrentExecReq).
					Return(getCurrentExecResp, nil).Once()

				historyMgr := eft.ShardCtx.Resource.HistoryMgr
				historyMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(&persistence.ReadHistoryBranchResponse{}, nil).
					Once()

				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(&persistence.UpdateWorkflowExecutionResponse{}, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "load first execution run ID from start event",
			terminationRequest: types.HistoryTerminateWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				TerminateRequest: &types.TerminateWorkflowExecutionRequest{
					Domain:              constants.TestDomainName,
					WorkflowExecution:   &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					Reason:              "Test termination",
					Identity:            "testRunner",
					FirstExecutionRunID: "fetched-first-run-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				getExecReq := &persistence.GetWorkflowExecutionRequest{
					DomainID:   constants.TestDomainID,
					Execution:  types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: constants.TestRunID},
					DomainName: constants.TestDomainName,
					RangeID:    1,
				}
				getExecResp := &persistence.GetWorkflowExecutionResponse{
					State: &persistence.WorkflowMutableState{
						ExecutionInfo: &persistence.WorkflowExecutionInfo{
							DomainID:            constants.TestDomainID,
							WorkflowID:          constants.TestWorkflowID,
							RunID:               constants.TestRunID,
							FirstExecutionRunID: "",
						},
						ExecutionStats: &persistence.ExecutionStats{},
					},
					MutableStateStats: &persistence.MutableStateStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetWorkflowExecution", mock.Anything, getExecReq).
					Return(getExecResp, nil).Once()

				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, "").Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				getCurrentExecReq := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: constants.TestWorkflowID,
					DomainName: constants.TestDomainName,
				}
				getCurrentExecResp := &persistence.GetCurrentExecutionResponse{
					RunID:       constants.TestRunID,
					State:       persistence.WorkflowStateRunning,
					CloseStatus: persistence.WorkflowCloseStatusNone,
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("GetCurrentExecution", mock.Anything, getCurrentExecReq).
					Return(getCurrentExecResp, nil).Once()

				historyBranchResp := &persistence.ReadHistoryBranchResponse{
					HistoryEvents: []*types.HistoryEvent{
						{
							ID: 1,
							WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
								FirstExecutionRunID: "fetched-first-run-id",
							},
						},
					},
				}
				historyMgr := eft.ShardCtx.Resource.HistoryMgr
				historyMgr.
					On("ReadHistoryBranch", mock.Anything, mock.Anything).
					Return(historyBranchResp, nil).
					Once()

				updateExecResp := &persistence.UpdateWorkflowExecutionResponse{
					MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{},
				}
				eft.ShardCtx.Resource.ExecutionMgr.
					On("UpdateWorkflowExecution", mock.Anything, mock.Anything).
					Return(updateExecResp, nil).Once()

				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)

				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			tc.setupMocks(t, eft)

			err := eft.Engine.TerminateWorkflowExecution(
				context.Background(),
				&tc.terminationRequest,
			)

			if (err != nil) != tc.wantErr {
				t.Fatalf("TerminateWorkflowExecution() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
