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
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
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
