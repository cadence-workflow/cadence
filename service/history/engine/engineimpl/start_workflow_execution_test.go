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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/events"
)

func TestStartWorkflowExecution(t *testing.T) {
	tests := []struct {
		name       string
		request    *types.HistoryStartWorkflowExecutionRequest
		setupMocks func(*testing.T, *testdata.EngineForTest)
		wantErr    bool
	}{
		{
			name: "start workflow execution success",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:       constants.TestDomainName,
					WorkflowID:   "workflow-id",
					WorkflowType: &types.WorkflowType{Name: "workflow-type"},
					TaskList: &types.TaskList{
						Name: "default-task-list",
					},
					Input:                               []byte("workflow input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),   // 10 seconds
					Identity:                            "workflow-starter",
					RequestID:                           "request-id-for-start",
					RetryPolicy: &types.RetryPolicy{
						InitialIntervalInSeconds:    1,
						BackoffCoefficient:          2.0,
						MaximumIntervalInSeconds:    10,
						MaximumAttempts:             5,
						ExpirationIntervalInSeconds: 3600, // 1 hour
					},
					Memo: &types.Memo{
						Fields: map[string][]byte{
							"key1": []byte("value1"),
						},
					},
					SearchAttributes: &types.SearchAttributes{
						IndexedFields: map[string][]byte{
							"CustomKeywordField": []byte("test"),
						},
					},
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.CreateWorkflowExecutionResponse{MutableStateUpdateSessionStats: &persistence.MutableStateUpdateSessionStats{}}, nil).Once()
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
			name: "failed to get workflow execution",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					Input:                               []byte("workflow input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),   // 10 seconds
					Identity:                            "workflow-starter",
					RequestID:                           "request-id-for-start",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("internal error")).Once()
			},
			wantErr: true,
		},
		{
			name: "prev mutable state version conflict",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					Input:                               []byte("workflow input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					TaskList: &types.TaskList{
						Name: "default-task-list",
					},
					Identity:              "workflow-starter",
					RequestID:             "request-id-for-start",
					WorkflowType:          &types.WorkflowType{Name: "workflow-type"},
					WorkflowIDReusePolicy: types.WorkflowIDReusePolicyAllowDuplicate.Ptr(),
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)

				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("version conflict")).Once()
				eft.ShardCtx.Resource.ExecutionMgr.On("UpdateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, errors.New("internal error")).Once()

				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
			},
			wantErr: true,
		},
		{
			name: "workflow ID reuse - terminate if running",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					Input:                               []byte("workflow input"),
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					TaskList: &types.TaskList{
						Name: "default-task-list",
					},
					Identity:              "workflow-starter",
					RequestID:             "request-id-for-start",
					WorkflowType:          &types.WorkflowType{Name: "workflow-type"},
					WorkflowIDReusePolicy: types.WorkflowIDReusePolicyTerminateIfRunning.Ptr(),
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				// Simulate the termination and recreation process
				eft.ShardCtx.Resource.ExecutionMgr.On("TerminateWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.CreateWorkflowExecutionResponse{}, nil).Once()
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
			name: "workflow ID reuse policy - reject duplicate",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					TaskList:                            &types.TaskList{Name: "default-task-list"},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),   // 10 seconds
					Identity:                            "workflow-starter",
					RequestID:                           "request-id-for-start",
					WorkflowIDReusePolicy:               types.WorkflowIDReusePolicyRejectDuplicate.Ptr(),
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil).AnyTimes()

				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(nil, &persistence.WorkflowExecutionAlreadyStartedError{
					StartRequestID: "existing-request-id",
					RunID:          "existing-run-id",
				}).Once()
				eft.ShardCtx.Resource.ShardMgr.
					On("UpdateShard", mock.Anything, mock.Anything).
					Return(nil)
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()
				historyV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			tc.setupMocks(t, eft)

			_, err := eft.Engine.StartWorkflowExecution(context.Background(), tc.request)
			if (err != nil) != tc.wantErr {
				t.Fatalf("%s: StartWorkflowExecution() error = %v, wantErr %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestStartWorkflowExecution_OrphanedHistoryCleanup(t *testing.T) {
	tests := []struct {
		name                 string
		request              *types.HistoryStartWorkflowExecutionRequest
		setupMocks           func(*testing.T, *testdata.EngineForTest)
		enableCleanupFlag    bool
		expectHistoryCleanup bool
		wantErr              bool
	}{
		{
			name: "cleanup orphaned history on WorkflowExecutionAlreadyStartedError with flag enabled",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					TaskList:                            &types.TaskList{Name: "default-task-list"},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "workflow-starter",
					RequestID:                           "request-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)

				var capturedBranchToken []byte
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Run(func(args mock.Arguments) {
						req := args.Get(1).(*persistence.AppendHistoryNodesRequest)
						capturedBranchToken = req.BranchToken
					}).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()

				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).
					Return(nil, &persistence.WorkflowExecutionAlreadyStartedError{
						StartRequestID: "different-request-id",
						RunID:          "existing-run-id",
						State:          persistence.WorkflowStateCompleted,
					}).Once()

				historyV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.DeleteHistoryBranchRequest) bool {
					return assert.Equal(t, capturedBranchToken, req.BranchToken) &&
						assert.Equal(t, constants.TestDomainName, req.DomainName)
				})).
					Return(nil).Once()
			},
			enableCleanupFlag:    true,
			expectHistoryCleanup: true,
			wantErr:              true,
		},
		{
			name: "no cleanup when flag disabled on WorkflowExecutionAlreadyStartedError",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					TaskList:                            &types.TaskList{Name: "default-task-list"},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "workflow-starter",
					RequestID:                           "request-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)

				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()

				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).
					Return(nil, &persistence.WorkflowExecutionAlreadyStartedError{
						StartRequestID: "different-request-id",
						RunID:          "existing-run-id",
						State:          persistence.WorkflowStateCompleted,
					}).Once()
			},
			enableCleanupFlag:    false,
			expectHistoryCleanup: false,
			wantErr:              true,
		},
		{
			name: "no cleanup on DuplicateRequestError with WorkflowRequestTypeStart (returns success)",
			request: &types.HistoryStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				StartRequest: &types.StartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					TaskList:                            &types.TaskList{Name: "default-task-list"},
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600),
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					Identity:                            "workflow-starter",
					RequestID:                           "request-id",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)

				eft.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).
					Return(nil, &types.EntityNotExistsError{}).Once()

				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).
					Return(&persistence.AppendHistoryNodesResponse{}, nil).Once()

				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).
					Return(nil, &persistence.DuplicateRequestError{
						RequestType: persistence.WorkflowRequestTypeStart,
						RunID:       "existing-run-id",
					}).Once()
				// No DeleteHistoryBranch mock - cleanup doesn't happen for this case
			},
			enableCleanupFlag:    true,
			expectHistoryCleanup: false, // Cleanup doesn't happen because success is returned
			wantErr:              false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			eft.ShardCtx.Resource.ShardMgr.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
			eft.ShardCtx.GetConfig().EnableCleanupOrphanedHistoryBranchOnWorkflowCreation = dynamicproperties.GetBoolPropertyFnFilteredByDomain(tc.enableCleanupFlag)

			tc.setupMocks(t, eft)

			_, err := eft.Engine.StartWorkflowExecution(context.Background(), tc.request)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectHistoryCleanup {
				eft.ShardCtx.Resource.HistoryMgr.AssertCalled(t, "DeleteHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.DeleteHistoryBranchRequest) bool {
					return req.BranchToken != nil
				}))
			} else {
				eft.ShardCtx.Resource.HistoryMgr.AssertNotCalled(t, "DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest"))
			}
		})
	}
}

func TestSignalWithStartWorkflowExecution(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*testing.T, *testdata.EngineForTest)
		request    *types.HistorySignalWithStartWorkflowExecutionRequest
		wantErr    bool
	}{
		{
			name: "signal and start workflow successfully",
			request: &types.HistorySignalWithStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          "workflow-id",
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					SignalName:                          "signal-name",
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					TaskList: &types.TaskList{
						Name: "default-task-list",
					},
					RequestID:   "request-id-for-start",
					SignalInput: []byte("signal-input"),
					Identity:    "tester",
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)
				// Mock GetCurrentExecution to simulate a non-existent current execution
				getCurrentExecReq := &persistence.GetCurrentExecutionRequest{
					DomainID:   constants.TestDomainID,
					WorkflowID: "workflow-id",
					DomainName: constants.TestDomainName,
				}
				getCurrentExecResp := &persistence.GetCurrentExecutionResponse{
					RunID:       "", // No current run ID indicates no current execution
					State:       persistence.WorkflowStateCompleted,
					CloseStatus: persistence.WorkflowCloseStatusCompleted,
				}
				eft.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, getCurrentExecReq).Return(getCurrentExecResp, &types.EntityNotExistsError{}).Once()
				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.CreateWorkflowExecutionResponse{}, nil)
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
			name: "terminate existing and start new workflow",
			request: &types.HistorySignalWithStartWorkflowExecutionRequest{
				DomainUUID: constants.TestDomainID,
				SignalWithStartRequest: &types.SignalWithStartWorkflowExecutionRequest{
					Domain:                              constants.TestDomainName,
					WorkflowID:                          constants.TestWorkflowID,
					WorkflowType:                        &types.WorkflowType{Name: "workflow-type"},
					SignalName:                          "signal-name",
					ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(3600), // 1 hour
					TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
					TaskList: &types.TaskList{
						Name: "default-task-list",
					},
					RequestID:             "request-id-for-start",
					SignalInput:           []byte("signal-input"),
					Identity:              "tester",
					WorkflowIDReusePolicy: (*types.WorkflowIDReusePolicy)(common.Int32Ptr(3)),
				},
			},
			setupMocks: func(t *testing.T, eft *testdata.EngineForTest) {
				domainEntry := &cache.DomainCacheEntry{}
				eft.ShardCtx.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(domainEntry, nil).AnyTimes()
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), constants.TestDomainID, nil).Return(&types.ActiveClusterInfo{ActiveClusterName: cluster.TestCurrentClusterName}, nil)

				// Simulate current workflow execution is running
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
				eft.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, getCurrentExecReq).Return(getCurrentExecResp, nil).Once()

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
				eft.ShardCtx.Resource.ActiveClusterMgr.EXPECT().GetActiveClusterInfoByWorkflow(gomock.Any(), constants.TestDomainID, constants.TestWorkflowID, constants.TestRunID).Return(&types.ActiveClusterInfo{ActiveClusterName: "test-active-cluster"}, nil).AnyTimes()
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
				// Expect termination of the current workflow
				eft.ShardCtx.Resource.ExecutionMgr.On("TerminateWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()

				// Expect creation of a new workflow execution
				eft.ShardCtx.Resource.ExecutionMgr.On("CreateWorkflowExecution", mock.Anything, mock.Anything).Return(&persistence.CreateWorkflowExecutionResponse{}, nil).Once()

				// Mocking additional interactions required by the workflow context and execution
				eft.ShardCtx.Resource.ShardMgr.On("UpdateShard", mock.Anything, mock.Anything).Return(nil)
				historyV2Mgr := eft.ShardCtx.Resource.HistoryMgr
				historyV2Mgr.On("AppendHistoryNodes", mock.Anything, mock.AnythingOfType("*persistence.AppendHistoryNodesRequest")).Return(&persistence.AppendHistoryNodesResponse{}, nil)
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

			response, err := eft.Engine.SignalWithStartWorkflowExecution(context.Background(), tc.request)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
			}
		})
	}
}

func TestCreateMutableState(t *testing.T) {
	tests := []struct {
		name           string
		domainEntry    *cache.DomainCacheEntry
		mockFn         func(ac *activecluster.MockManager)
		wantErr        bool
		wantVersion    int64
		wantErrMessage string
	}{
		{
			name: "create mutable state successfully, failover version is looked up from active cluster manager",
			domainEntry: getDomainCacheEntry(
				0,
				&types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   0,
								},
								"us-east": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   2,
								},
							},
						},
					},
				}),
			mockFn: func(ac *activecluster.MockManager) {
				ac.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestCurrentClusterName,
						FailoverVersion:   125,
					}, nil)
			},
			wantVersion: 125,
		},
		{
			name: "failed to create mutable state, current cluster is not the active cluster",
			domainEntry: getDomainCacheEntry(
				0,
				&types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   0,
								},
								"us-east": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   2,
								},
							},
						},
					},
				}),
			mockFn: func(ac *activecluster.MockManager) {
				ac.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.ActiveClusterInfo{
						ActiveClusterName: cluster.TestAlternativeClusterName,
						FailoverVersion:   125,
					}, nil)
			},
			wantErr:        true,
			wantErrMessage: "is active in cluster(s): [cluster1 cluster2]",
		},
		{
			name: "failed to create mutable state. GetActiveClusterInfoByClusterAttribute failed",
			domainEntry: getDomainCacheEntry(
				0,
				&types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   0,
								},
								"us-east": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   2,
								},
							},
						},
					},
				}),
			mockFn: func(ac *activecluster.MockManager) {
				ac.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantErr:        true,
			wantErrMessage: "some error",
		},
		{
			name: "failed to create mutable state, cluster attribute not found",
			domainEntry: getDomainCacheEntry(
				0,
				&types.ActiveClusters{
					AttributeScopes: map[string]types.ClusterAttributeScope{
						"region": {
							ClusterAttributes: map[string]types.ActiveClusterInfo{
								"us-west": {
									ActiveClusterName: "cluster1",
									FailoverVersion:   0,
								},
								"us-east": {
									ActiveClusterName: "cluster2",
									FailoverVersion:   2,
								},
							},
						},
					},
				}),
			mockFn: func(ac *activecluster.MockManager) {
				ac.EXPECT().GetActiveClusterInfoByClusterAttribute(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &activecluster.ClusterAttributeNotFoundError{})
			},
			wantErr:        true,
			wantErrMessage: "Cannot start workflow with a cluster attribute that is not found in the domain's metadata.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()
			engine := eft.Engine.(*historyEngineImpl)

			if tc.mockFn != nil {
				tc.mockFn(eft.ShardCtx.Resource.ActiveClusterMgr)
			}

			mutableState, err := engine.createMutableState(
				context.Background(),
				tc.domainEntry,
				"rid",
				&types.HistoryStartWorkflowExecutionRequest{
					StartRequest: &types.StartWorkflowExecutionRequest{},
				},
			)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMessage)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mutableState)
			}

			if err != nil {
				return
			}

			gotVer := mutableState.GetCurrentVersion()
			assert.Equal(t, tc.wantVersion, gotVer)
		})
	}
}

func TestHandleCreateWorkflowExecutionFailureCleanup(t *testing.T) {
	// Known error types that should trigger cleanup
	knownCleanupError := &persistence.WorkflowExecutionAlreadyStartedError{
		Msg: "workflow already started",
	}

	tests := []struct {
		name                          string
		enableCleanupFlag             bool
		enableUninitializedRecordFlag bool
		err                           error
		workflowExecution             *types.WorkflowExecution
		historyBlob                   []byte
		startRequest                  *types.HistoryStartWorkflowExecutionRequest
		setupMocks                    func(*testdata.EngineForTest)
		expectDeleteHistoryBranch     bool
		expectDeleteVisibility        bool
	}{
		{
			name:              "workflowExecution is nil - returns early",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: nil,
			historyBlob:       []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "historyBlob is nil - returns early",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: nil,
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "timeout error - returns early without cleanup",
			enableCleanupFlag: true,
			err:               &persistence.TimeoutError{Msg: "timeout"},
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "unknown error - returns early without cleanup",
			enableCleanupFlag: true,
			err:               errors.New("some unknown error"),
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "cleanup disabled - returns early",
			enableCleanupFlag: false,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "startRequest is nil - returns early with bug log",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob:               []byte("branch-token"),
			startRequest:              nil,
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "workflowID is empty - returns early with bug log",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "runID is empty - returns early with bug log",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks:                func(eft *testdata.EngineForTest) {},
			expectDeleteHistoryBranch: false,
			expectDeleteVisibility:    false,
		},
		{
			name:              "cleanup path with WorkflowExecutionAlreadyStartedError - deletes history branch successfully",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks: func(eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.HistoryMgr.On("DeleteHistoryBranch", mock.Anything, mock.MatchedBy(func(req *persistence.DeleteHistoryBranchRequest) bool {
					return string(req.BranchToken) == "branch-token" &&
						req.DomainName == constants.TestDomainName
				})).Return(nil).Once()
			},
			expectDeleteHistoryBranch: true,
			expectDeleteVisibility:    false,
		},
		{
			name:              "cleanup path with DuplicateRequestError - deletes history branch successfully",
			enableCleanupFlag: true,
			err: &persistence.DuplicateRequestError{
				RequestType: persistence.WorkflowRequestTypeStart,
				RunID:       "existing-run-id",
			},
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks: func(eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.HistoryMgr.On("DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest")).
					Return(nil).Once()
			},
			expectDeleteHistoryBranch: true,
			expectDeleteVisibility:    false,
		},
		{
			name:              "cleanup path - delete history branch fails gracefully",
			enableCleanupFlag: true,
			err:               knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks: func(eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.HistoryMgr.On("DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest")).
					Return(errors.New("delete failed")).Once()
			},
			expectDeleteHistoryBranch: true,
			expectDeleteVisibility:    false,
		},
		{
			name:                          "cleanup path with visibility - deletes both history and visibility",
			enableCleanupFlag:             true,
			enableUninitializedRecordFlag: true,
			err:                           knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks: func(eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.HistoryMgr.On("DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest")).
					Return(nil).Once()
				eft.ShardCtx.Resource.VisibilityMgr.On("DeleteWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.VisibilityDeleteWorkflowExecutionRequest) bool {
					return req.WorkflowID == "wf-id" &&
						req.RunID == "run-id" &&
						req.Domain == constants.TestDomainName
				})).Return(nil).Once()
			},
			expectDeleteHistoryBranch: true,
			expectDeleteVisibility:    true,
		},
		{
			name:                          "cleanup path with visibility - visibility delete fails gracefully",
			enableCleanupFlag:             true,
			enableUninitializedRecordFlag: true,
			err:                           knownCleanupError,
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "wf-id",
				RunID:      "run-id",
			},
			historyBlob: []byte("branch-token"),
			startRequest: &types.HistoryStartWorkflowExecutionRequest{
				StartRequest: &types.StartWorkflowExecutionRequest{},
			},
			setupMocks: func(eft *testdata.EngineForTest) {
				eft.ShardCtx.Resource.HistoryMgr.On("DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest")).
					Return(nil).Once()
				eft.ShardCtx.Resource.VisibilityMgr.On("DeleteWorkflowExecution", mock.Anything, mock.AnythingOfType("*persistence.VisibilityDeleteWorkflowExecutionRequest")).
					Return(errors.New("visibility delete failed")).Once()
			},
			expectDeleteHistoryBranch: true,
			expectDeleteVisibility:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			eft.Engine.Start()
			defer eft.Engine.Stop()

			// Configure flags
			eft.ShardCtx.GetConfig().EnableCleanupOrphanedHistoryBranchOnWorkflowCreation = dynamicproperties.GetBoolPropertyFnFilteredByDomain(tc.enableCleanupFlag)
			eft.ShardCtx.GetConfig().EnableRecordWorkflowExecutionUninitialized = dynamicproperties.GetBoolPropertyFnFilteredByDomain(tc.enableUninitializedRecordFlag)

			// Setup mocks
			tc.setupMocks(eft)

			// Get the engine implementation
			engine := eft.Engine.(*historyEngineImpl)

			// Create domain entry
			domainEntry := cache.NewDomainCacheEntryForTest(
				&persistence.DomainInfo{
					ID:   constants.TestDomainID,
					Name: constants.TestDomainName,
				},
				nil,
				true,
				nil,
				0,
				nil,
				1,
				1,
				1,
			)

			// Create history blob if provided
			var historyBlob *events.PersistedBlob
			if tc.historyBlob != nil {
				historyBlob = &events.PersistedBlob{
					BranchToken: tc.historyBlob,
				}
			}

			// Call the function
			engine.handleCreateWorkflowExecutionFailureCleanup(
				context.Background(),
				tc.startRequest,
				domainEntry,
				tc.workflowExecution,
				historyBlob,
				false,
				tc.err,
			)

			// Assert mocks
			if tc.expectDeleteHistoryBranch {
				eft.ShardCtx.Resource.HistoryMgr.AssertCalled(t, "DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest"))
			} else {
				eft.ShardCtx.Resource.HistoryMgr.AssertNotCalled(t, "DeleteHistoryBranch", mock.Anything, mock.AnythingOfType("*persistence.DeleteHistoryBranchRequest"))
			}

			if tc.expectDeleteVisibility {
				eft.ShardCtx.Resource.VisibilityMgr.AssertCalled(t, "DeleteWorkflowExecution", mock.Anything, mock.AnythingOfType("*persistence.VisibilityDeleteWorkflowExecutionRequest"))
			} else if tc.enableUninitializedRecordFlag {
				// Only assert not called if the flag was enabled (otherwise it wouldn't be called anyway)
				eft.ShardCtx.Resource.VisibilityMgr.AssertNotCalled(t, "DeleteWorkflowExecution", mock.Anything, mock.AnythingOfType("*persistence.VisibilityDeleteWorkflowExecutionRequest"))
			}
		})
	}
}

func getDomainCacheEntry(domainFailoverVersion int64, cfg *types.ActiveClusters) *cache.DomainCacheEntry {
	// only thing we care in domain cache entry is the active clusters config
	return cache.NewDomainCacheEntryForTest(
		&persistence.DomainInfo{
			ID: "domain-id",
		},
		nil,
		true,
		&persistence.DomainReplicationConfig{
			ActiveClusters:    cfg,
			ActiveClusterName: "cluster0",
		},
		domainFailoverVersion,
		nil,
		1,
		1,
		1,
	)
}
