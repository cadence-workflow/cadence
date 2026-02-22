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

package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	cadence_errors "github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/tasklist"
	"github.com/uber/cadence/service/sharddistributor/client/executorclient"
)

func mustNewIdentifier(t *testing.T, domainID, taskListName string, taskListType int) *tasklist.Identifier {
	t.Helper()
	id, err := tasklist.NewIdentifier(domainID, taskListName, taskListType)
	require.NoError(t, err)
	return id
}

// newMockManagerWithTaskListID returns a MockManager with TaskListID() stubbed to return id (AnyTimes).
func newMockManagerWithTaskListID(ctrl *gomock.Controller, id *tasklist.Identifier) *tasklist.MockManager {
	mgr := tasklist.NewMockManager(ctrl)
	mgr.EXPECT().TaskListID().Return(id).AnyTimes()
	return mgr
}

func TestGetTaskListsByDomain(t *testing.T) {
	testCases := []struct {
		name           string
		mockSetup      func(*cache.MockDomainCache, map[tasklist.Identifier]*tasklist.MockManager, map[tasklist.Identifier]*tasklist.MockManager)
		returnAllKinds bool
		wantErr        bool
		want           *types.GetTaskListsByDomainResponse
	}{
		{
			name: "domain cache error",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockTaskListManagers map[tasklist.Identifier]*tasklist.MockManager, mockStickyManagers map[tasklist.Identifier]*tasklist.MockManager) {
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("", errors.New("cache failure"))
			},
			wantErr: true,
		},
		{
			name: "success",
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockTaskListManagers map[tasklist.Identifier]*tasklist.MockManager, mockStickyManagers map[tasklist.Identifier]*tasklist.MockManager) {
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("test-domain-id", nil)
				for id, mockManager := range mockTaskListManagers {
					if id.GetDomainID() == "test-domain-id" {
						mockManager.EXPECT().GetTaskListKind().Return(types.TaskListKindNormal)
						mockManager.EXPECT().DescribeTaskList(false).Return(&types.DescribeTaskListResponse{
							Pollers: []*types.PollerInfo{
								{
									Identity: fmt.Sprintf("test-poller-%s", id.GetRoot()),
								},
							},
						})
					}
				}
				for id, mockManager := range mockStickyManagers {
					if id.GetDomainID() == "test-domain-id" {
						mockManager.EXPECT().GetTaskListKind().Return(types.TaskListKindSticky)
					}
				}
			},
			wantErr: false,
			want: &types.GetTaskListsByDomainResponse{
				DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{
					"decision0": {
						Pollers: []*types.PollerInfo{
							{
								Identity: "test-poller-decision0",
							},
						},
					},
				},
				ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{
					"activity0": {
						Pollers: []*types.PollerInfo{
							{
								Identity: "test-poller-activity0",
							},
						},
					},
				},
			},
		},
		{
			name:           "success - all kinds",
			returnAllKinds: true,
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockTaskListManagers map[tasklist.Identifier]*tasklist.MockManager, mockStickyManagers map[tasklist.Identifier]*tasklist.MockManager) {
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("test-domain-id", nil)
				for id, mockManager := range mockTaskListManagers {
					if id.GetDomainID() == "test-domain-id" {
						mockManager.EXPECT().DescribeTaskList(false).Return(&types.DescribeTaskListResponse{
							Pollers: []*types.PollerInfo{
								{
									Identity: fmt.Sprintf("test-poller-%s", id.GetRoot()),
								},
							},
						})
					}
				}
				for id, mockManager := range mockStickyManagers {
					if id.GetDomainID() == "test-domain-id" {
						mockManager.EXPECT().DescribeTaskList(false).Return(&types.DescribeTaskListResponse{
							Pollers: []*types.PollerInfo{
								{
									Identity: fmt.Sprintf("sticky-poller-%s", id.GetRoot()),
								},
							},
						})
					}
				}
			},
			wantErr: false,
			want: &types.GetTaskListsByDomainResponse{
				DecisionTaskListMap: map[string]*types.DescribeTaskListResponse{
					"decision0": {
						Pollers: []*types.PollerInfo{
							{
								Identity: "test-poller-decision0",
							},
						},
					},
					"sticky0": {
						Pollers: []*types.PollerInfo{
							{
								Identity: "sticky-poller-sticky0",
							},
						},
					},
				},
				ActivityTaskListMap: map[string]*types.DescribeTaskListResponse{
					"activity0": {
						Pollers: []*types.PollerInfo{
							{
								Identity: "test-poller-activity0",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			decisionTasklistID := mustNewIdentifier(t, "test-domain-id", "decision0", 0)
			activityTasklistID := mustNewIdentifier(t, "test-domain-id", "activity0", 1)
			otherDomainTasklistID := mustNewIdentifier(t, "other-domain-id", "other0", 0)
			stickyTasklistID := mustNewIdentifier(t, "test-domain-id", "sticky0", 0)
			mockDecisionTaskListManager := newMockManagerWithTaskListID(mockCtrl, decisionTasklistID)
			mockActivityTaskListManager := newMockManagerWithTaskListID(mockCtrl, activityTasklistID)
			mockOtherDomainTaskListManager := newMockManagerWithTaskListID(mockCtrl, otherDomainTasklistID)
			mockStickyManager := newMockManagerWithTaskListID(mockCtrl, stickyTasklistID)
			mockTaskListManagers := map[tasklist.Identifier]*tasklist.MockManager{
				*decisionTasklistID:    mockDecisionTaskListManager,
				*activityTasklistID:    mockActivityTaskListManager,
				*otherDomainTasklistID: mockOtherDomainTaskListManager,
			}
			mockStickyManagers := map[tasklist.Identifier]*tasklist.MockManager{
				*stickyTasklistID: mockStickyManager,
			}
			tc.mockSetup(mockDomainCache, mockTaskListManagers, mockStickyManagers)

			taskListRegistry := tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient())
			engine := &matchingEngineImpl{
				domainCache:      mockDomainCache,
				taskListRegistry: taskListRegistry,
				config: &config.Config{
					EnableReturnAllTaskListKinds: func(opts ...dynamicproperties.FilterOption) bool {
						return tc.returnAllKinds
					},
				},
			}
			taskListRegistry.Register(*decisionTasklistID, mockDecisionTaskListManager)
			taskListRegistry.Register(*activityTasklistID, mockActivityTaskListManager)
			taskListRegistry.Register(*otherDomainTasklistID, mockOtherDomainTaskListManager)
			taskListRegistry.Register(*stickyTasklistID, mockStickyManager)
			resp, err := engine.GetTaskListsByDomain(nil, &types.GetTaskListsByDomainRequest{Domain: "test-domain"})

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestListTaskListPartitions(t *testing.T) {
	testCases := []struct {
		name      string
		req       *types.MatchingListTaskListPartitionsRequest
		mockSetup func(*cache.MockDomainCache, *membership.MockResolver)
		wantErr   bool
		want      *types.ListTaskListPartitionsResponse
	}{
		{
			name: "domain cache error",
			req: &types.MatchingListTaskListPartitionsRequest{
				Domain: "test-domain",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
			},
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockResolver *membership.MockResolver) {
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("", errors.New("cache failure"))
			},
			wantErr: true,
		},
		{
			name: "invalid tasklist name",
			req: &types.MatchingListTaskListPartitionsRequest{
				Domain: "test-domain",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/invalid-tasklist-name",
				},
			},
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockResolver *membership.MockResolver) {
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("test-domain-id", nil)
			},
			wantErr: true,
		},
		{
			name: "success",
			req: &types.MatchingListTaskListPartitionsRequest{
				Domain: "test-domain",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
			},
			mockSetup: func(mockDomainCache *cache.MockDomainCache, mockResolver *membership.MockResolver) {
				// activity tasklist
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("test-domain-id", nil)
				mockResolver.EXPECT().Lookup(gomock.Any(), "test-tasklist").Return(membership.NewHostInfo("addr2"), nil)
				mockResolver.EXPECT().Lookup(gomock.Any(), "/__cadence_sys/test-tasklist/1").Return(membership.HostInfo{}, errors.New("some error"))
				mockResolver.EXPECT().Lookup(gomock.Any(), "/__cadence_sys/test-tasklist/2").Return(membership.NewHostInfo("addr3"), nil)
				// decision tasklist
				mockDomainCache.EXPECT().GetDomainID("test-domain").Return("test-domain-id", nil)
				mockResolver.EXPECT().Lookup(gomock.Any(), "test-tasklist").Return(membership.NewHostInfo("addr0"), nil)
				mockResolver.EXPECT().Lookup(gomock.Any(), "/__cadence_sys/test-tasklist/1").Return(membership.HostInfo{}, errors.New("some error"))
				mockResolver.EXPECT().Lookup(gomock.Any(), "/__cadence_sys/test-tasklist/2").Return(membership.NewHostInfo("addr1"), nil)
			},
			wantErr: false,
			want: &types.ListTaskListPartitionsResponse{
				DecisionTaskListPartitions: []*types.TaskListPartitionMetadata{
					{
						Key:           "test-tasklist",
						OwnerHostName: "addr0",
					},
					{
						Key:           "/__cadence_sys/test-tasklist/1",
						OwnerHostName: "",
					},
					{
						Key:           "/__cadence_sys/test-tasklist/2",
						OwnerHostName: "addr1",
					},
				},
				ActivityTaskListPartitions: []*types.TaskListPartitionMetadata{
					{
						Key:           "test-tasklist",
						OwnerHostName: "addr2",
					},
					{
						Key:           "/__cadence_sys/test-tasklist/1",
						OwnerHostName: "",
					},
					{
						Key:           "/__cadence_sys/test-tasklist/2",
						OwnerHostName: "addr3",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockResolver := membership.NewMockResolver(mockCtrl)
			tc.mockSetup(mockDomainCache, mockResolver)

			engine := &matchingEngineImpl{
				domainCache:        mockDomainCache,
				membershipResolver: mockResolver,
				config: &config.Config{
					NumTasklistWritePartitions: dynamicproperties.GetIntPropertyFilteredByTaskListInfo(3),
				},
			}
			resp, err := engine.ListTaskListPartitions(nil, tc.req)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestCancelOutstandingPoll(t *testing.T) {
	testCases := []struct {
		name      string
		req       *types.CancelOutstandingPollRequest
		mockSetup func(mockCtrl *gomock.Controller, processor *tasklist.MockManager, executor *executorclient.MockExecutor[tasklist.ShardProcessor])
		wantErr   bool
	}{
		{
			name: "invalid tasklist name",
			req: &types.CancelOutstandingPollRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/invalid-tasklist-name",
				},
				PollerID: "test-poller-id",
			},
			mockSetup: func(mockCtrl *gomock.Controller, mockManager *tasklist.MockManager, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			wantErr: true,
		},
		{
			name: "success",
			req: &types.CancelOutstandingPollRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				PollerID: "test-poller-id",
			},
			mockSetup: func(mockCtrl *gomock.Controller, mockManager *tasklist.MockManager, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().CancelPoller("test-poller-id")
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			tasklistID := mustNewIdentifier(t, "test-domain-id", "test-tasklist", 0)
			mockManager := newMockManagerWithTaskListID(mockCtrl, tasklistID)
			executor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
			tc.mockSetup(mockCtrl, mockManager, executor)
			taskListRegistry := tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient())
			engine := &matchingEngineImpl{
				taskListRegistry: taskListRegistry,
				executor:         executor,
			}
			taskListRegistry.Register(*tasklistID, mockManager)
			hCtx := &handlerContext{Context: context.Background()}
			err := engine.CancelOutstandingPoll(hCtx, tc.req)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRespondQueryTaskCompleted(t *testing.T) {
	testCases := []struct {
		name         string
		req          *types.MatchingRespondQueryTaskCompletedRequest
		queryTaskMap map[string]chan *queryResult
		wantErr      bool
	}{
		{
			name: "success",
			req: &types.MatchingRespondQueryTaskCompletedRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskID: "id-0",
			},
			queryTaskMap: map[string]chan *queryResult{
				"id-0": make(chan *queryResult, 1),
			},
			wantErr: false,
		},
		{
			name: "query task not found",
			req: &types.MatchingRespondQueryTaskCompletedRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskID: "id-0",
			},
			queryTaskMap: map[string]chan *queryResult{},
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &matchingEngineImpl{
				lockableQueryTaskMap: lockableQueryTaskMap{
					queryTaskMap: tc.queryTaskMap,
				},
			}
			err := engine.RespondQueryTaskCompleted(&handlerContext{scope: metrics.NewNoopMetricsClient().Scope(0)}, tc.req)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestQueryWorkflow(t *testing.T) {
	testCases := []struct {
		name      string
		req       *types.MatchingQueryWorkflowRequest
		hCtx      *handlerContext
		mockSetup func(*tasklist.MockManager, *lockableQueryTaskMap, *gomock.Controller, *executorclient.MockExecutor[tasklist.ShardProcessor])
		wantErr   bool
		want      *types.MatchingQueryWorkflowResponse
	}{
		{
			name: "invalid tasklist name",
			req: &types.MatchingQueryWorkflowRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/invalid-tasklist-name",
				},
			},
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap, mockCtrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			wantErr: true,
		},
		{
			name: "sticky worker unavailable",
			req: &types.MatchingQueryWorkflowRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
					Kind: types.TaskListKindSticky.Ptr(),
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap, mockCtrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().HasPollerAfter(gomock.Any()).Return(false)
			},
			wantErr: true,
		},
		{
			name: "failed to dispatch query task",
			req: &types.MatchingQueryWorkflowRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap, mockCtrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().DispatchQueryTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantErr: true,
		},
		{
			name: "success",
			req: &types.MatchingQueryWorkflowRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
			},
			hCtx: &handlerContext{
				Context: func() context.Context {
					return context.Background()
				}(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap, mockCtrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().DispatchQueryTask(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, taskID string, request *types.MatchingQueryWorkflowRequest) (*types.MatchingQueryWorkflowResponse, error) {
					queryResChan, ok := queryResultMap.get(taskID)
					if !ok {
						return nil, errors.New("cannot find query result channel by taskID")
					}
					queryResChan <- &queryResult{workerResponse: &types.MatchingRespondQueryTaskCompletedRequest{
						TaskID: taskID,
						CompletedRequest: &types.RespondQueryTaskCompletedRequest{
							QueryResult: []byte("some result"),
						},
					}}
					return nil, nil
				})
				mockManager.EXPECT().TaskListPartitionConfig().Return(&types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				})
			},
			wantErr: false,
			want: &types.MatchingQueryWorkflowResponse{
				QueryResult: []byte("some result"),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			tasklistID := mustNewIdentifier(t, "test-domain-id", "test-tasklist", 0)
			mockManager := newMockManagerWithTaskListID(mockCtrl, tasklistID)
			executor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
			taskListRegistry := tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient())
			engine := &matchingEngineImpl{
				taskListRegistry:     taskListRegistry,
				timeSource:           clock.NewRealTimeSource(),
				lockableQueryTaskMap: lockableQueryTaskMap{queryTaskMap: make(map[string]chan *queryResult)},
				executor:             executor,
			}
			taskListRegistry.Register(*tasklistID, mockManager)
			tc.mockSetup(mockManager, &engine.lockableQueryTaskMap, mockCtrl, executor)
			resp, err := engine.QueryWorkflow(tc.hCtx, tc.req)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestWaitForQueryResult(t *testing.T) {
	testCases := []struct {
		name      string
		result    *queryResult
		mockSetup func(*client.MockVersionChecker)
		wantErr   bool
		assertErr func(*testing.T, error)
		want      *types.MatchingQueryWorkflowResponse
	}{
		{
			name: "internal error",
			result: &queryResult{
				internalError: errors.New("some error"),
			},
			mockSetup: func(mockVersionChecker *client.MockVersionChecker) {},
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, "some error", err.Error())
			},
			wantErr: true,
		},
		{
			name: "strong consistency query not supported",
			result: &queryResult{
				workerResponse: &types.MatchingRespondQueryTaskCompletedRequest{
					CompletedRequest: &types.RespondQueryTaskCompletedRequest{
						WorkerVersionInfo: &types.WorkerVersionInfo{
							Impl:           "uber-go",
							FeatureVersion: "1.0.0",
						},
					},
				},
			},
			mockSetup: func(mockVersionChecker *client.MockVersionChecker) {
				mockVersionChecker.EXPECT().SupportsConsistentQuery("uber-go", "1.0.0").Return(errors.New("version error"))
			},
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, "version error", err.Error())
			},
			wantErr: true,
		},
		{
			name: "success - query task completed",
			result: &queryResult{
				workerResponse: &types.MatchingRespondQueryTaskCompletedRequest{
					CompletedRequest: &types.RespondQueryTaskCompletedRequest{
						WorkerVersionInfo: &types.WorkerVersionInfo{
							Impl:           "uber-go",
							FeatureVersion: "1.0.0",
						},
						CompletedType: types.QueryTaskCompletedTypeCompleted.Ptr(),
						QueryResult:   []byte("some result"),
					},
				},
			},
			mockSetup: func(mockVersionChecker *client.MockVersionChecker) {
				mockVersionChecker.EXPECT().SupportsConsistentQuery("uber-go", "1.0.0").Return(nil)
			},
			wantErr: false,
			want: &types.MatchingQueryWorkflowResponse{
				QueryResult: []byte("some result"),
			},
		},
		{
			name: "query task failed",
			result: &queryResult{
				workerResponse: &types.MatchingRespondQueryTaskCompletedRequest{
					CompletedRequest: &types.RespondQueryTaskCompletedRequest{
						WorkerVersionInfo: &types.WorkerVersionInfo{
							Impl:           "uber-go",
							FeatureVersion: "1.0.0",
						},
						CompletedType: types.QueryTaskCompletedTypeFailed.Ptr(),
						ErrorMessage:  "query failed",
					},
				},
			},
			mockSetup: func(mockVersionChecker *client.MockVersionChecker) {
				mockVersionChecker.EXPECT().SupportsConsistentQuery("uber-go", "1.0.0").Return(nil)
			},
			assertErr: func(t *testing.T, err error) {
				var e *types.QueryFailedError
				assert.ErrorAs(t, err, &e)
				assert.Equal(t, "query failed", e.Message)
			},
			wantErr: true,
		},
		{
			name: "unknown query result",
			result: &queryResult{
				workerResponse: &types.MatchingRespondQueryTaskCompletedRequest{
					CompletedRequest: &types.RespondQueryTaskCompletedRequest{
						WorkerVersionInfo: &types.WorkerVersionInfo{
							Impl:           "uber-go",
							FeatureVersion: "1.0.0",
						},
						CompletedType: types.QueryTaskCompletedType(100).Ptr(),
					},
				},
			},
			mockSetup: func(mockVersionChecker *client.MockVersionChecker) {
				mockVersionChecker.EXPECT().SupportsConsistentQuery("uber-go", "1.0.0").Return(nil)
			},
			assertErr: func(t *testing.T, err error) {
				var e *types.InternalServiceError
				assert.ErrorAs(t, err, &e)
				assert.Equal(t, "unknown query completed type", e.Message)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockVersionChecker := client.NewMockVersionChecker(mockCtrl)
			tc.mockSetup(mockVersionChecker)
			engine := &matchingEngineImpl{
				versionChecker: mockVersionChecker,
			}
			hCtx := &handlerContext{
				Context: context.Background(),
			}
			ch := make(chan *queryResult, 1)
			ch <- tc.result
			resp, err := engine.waitForQueryResult(hCtx, true, ch)
			if tc.wantErr {
				require.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, resp)
			}
		})
	}
}

func TestIsShuttingDown(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(0)
	mockCtrl := gomock.NewController(t)
	mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
	mockDomainCache.EXPECT().RegisterDomainChangeCallback(service.Matching, gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).Times(1)
	mockExecutor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
	mockExecutor.EXPECT().Start(gomock.Any())
	mockExecutor.EXPECT().Stop()
	e := matchingEngineImpl{
		domainCache:        mockDomainCache,
		shutdownCompletion: &wg,
		shutdown:           make(chan struct{}),
		executor:           mockExecutor,
		taskListRegistry:   tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient()),
	}
	e.Start()
	assert.False(t, e.isShuttingDown())
	e.Stop()
	assert.True(t, e.isShuttingDown())
}

func TestErrIfShardOwnershipLost(t *testing.T) {
	taskListID, err := tasklist.NewIdentifier("test-domain-id", "test-tasklist", persistence.TaskListTypeActivity)
	require.NoError(t, err)
	selfIdentity := "this-host"

	testCases := []struct {
		name                 string
		enableOwnershipGuard bool
		onboardedToSD        bool
		setupMocks           func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl)
		expectedErr          error
	}{
		{
			name:                 "ownership guard not enabled",
			enableOwnershipGuard: false,
			expectedErr:          nil,
		},
		{
			name: "engine is shutting down",
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				close(engine.shutdown)
			},
			enableOwnershipGuard: true,
			expectedErr: &cadence_errors.TaskListNotOwnedByHostError{
				OwnedByIdentity: "not known",
				MyIdentity:      selfIdentity,
				TasklistName:    taskListID.GetName(),
			},
		},
		{
			name:          "ownership lookup in SD fails",
			onboardedToSD: true,
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				executor.EXPECT().GetShardProcess(gomock.Any(), taskListID.GetName()).Return(nil, errors.New("sd down"))
			},
			enableOwnershipGuard: true,
			expectedErr:          errors.New("failed to lookup ownership in SD"),
		},
		{
			name:          "shard process not found in SD",
			onboardedToSD: true,
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				executor.EXPECT().GetShardProcess(gomock.Any(), taskListID.GetName()).Return(nil, nil)
			},
			enableOwnershipGuard: true,
			expectedErr: &cadence_errors.TaskListNotOwnedByHostError{
				OwnedByIdentity: "not known", // SD does not provide owner identity.
				MyIdentity:      selfIdentity,
				TasklistName:    taskListID.GetName(),
			},
		},
		{
			name:          "owned when shard process exists in SD",
			onboardedToSD: true,
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				executor.EXPECT().GetShardProcess(gomock.Any(), taskListID.GetName()).Return(tasklist.NewMockShardProcessor(ctrl), nil)
			},
			enableOwnershipGuard: true,
			expectedErr:          nil,
		},
		{
			name:                 "legacy ownership lookup fails",
			enableOwnershipGuard: true,
			onboardedToSD:        false,
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				resolver.EXPECT().Lookup(service.Matching, taskListID.GetName()).
					Return(membership.NewDetailedHostInfo("", "", nil), errors.New("resolver down"))
			},
			expectedErr: errors.New("failed to lookup task list owner"),
		},
		{
			name:                 "legacy owner mismatch returns not-owned error",
			enableOwnershipGuard: true,
			onboardedToSD:        false,
			setupMocks: func(ctrl *gomock.Controller, resolver *membership.MockResolver, executor *executorclient.MockExecutor[tasklist.ShardProcessor], engine *matchingEngineImpl) {
				resolver.EXPECT().Lookup(service.Matching, taskListID.GetName()).
					Return(membership.NewDetailedHostInfo("", "another-host", nil), nil)
			},
			expectedErr: &cadence_errors.TaskListNotOwnedByHostError{
				OwnedByIdentity: "another-host",
				MyIdentity:      selfIdentity,
				TasklistName:    taskListID.GetName(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			resolver := membership.NewMockResolver(ctrl)
			executor := executorclient.NewMockExecutor[tasklist.ShardProcessor](ctrl)
			executor.EXPECT().IsOnboardedToSD().Return(tc.onboardedToSD).AnyTimes()

			engine := &matchingEngineImpl{
				shutdown:           make(chan struct{}),
				executor:           executor,
				membershipResolver: resolver,
				selfAddress:        membership.NewDetailedHostInfo("", selfIdentity, nil),
				config: &config.Config{
					EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool {
						return tc.enableOwnershipGuard
					},
				},
				logger: log.NewNoop(),
			}

			if tc.setupMocks != nil {
				tc.setupMocks(ctrl, resolver, executor, engine)
			}

			err := engine.errIfShardOwnershipLost(context.Background(), taskListID)

			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				var expectedNotOwnedErr *cadence_errors.TaskListNotOwnedByHostError
				if errors.As(tc.expectedErr, &expectedNotOwnedErr) {
					var actualNotOwnedErr *cadence_errors.TaskListNotOwnedByHostError
					require.ErrorAs(t, err, &actualNotOwnedErr)
					assert.Equal(t, expectedNotOwnedErr, actualNotOwnedErr)
				} else {
					require.ErrorContains(t, err, tc.expectedErr.Error())
				}
			}
		})
	}
}

func TestGetTasklistsNotOwned(t *testing.T) {

	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)

	resolver.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("self", "host123", nil), nil)

	tl1 := mustNewIdentifier(t, "", "tl1", 0)
	tl2 := mustNewIdentifier(t, "", "tl2", 0)
	tl3 := mustNewIdentifier(t, "", "tl3", 0)

	tl1m := newMockManagerWithTaskListID(ctrl, tl1)
	tl2m := newMockManagerWithTaskListID(ctrl, tl2)
	tl3m := newMockManagerWithTaskListID(ctrl, tl3)

	resolver.EXPECT().Lookup(service.Matching, tl1.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl2.GetName()).Return(membership.NewDetailedHostInfo("", "host456", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl3.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)

	e := matchingEngineImpl{
		shutdown:           make(chan struct{}),
		membershipResolver: resolver,
		taskListRegistry:   tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient()),
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		logger: log.NewNoop(),
	}
	e.taskListRegistry.Register(*tl1, tl1m)
	e.taskListRegistry.Register(*tl2, tl2m)
	e.taskListRegistry.Register(*tl3, tl3m)

	tls, err := e.getNonOwnedTasklistsLocked()
	assert.NoError(t, err)

	assert.Equal(t, []tasklist.Manager{tl2m}, tls)
}

func TestShutDownTasklistsNotOwned(t *testing.T) {

	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)

	resolver.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("self", "host123", nil), nil)

	tl1 := mustNewIdentifier(t, "", "tl1", 0)
	tl2 := mustNewIdentifier(t, "", "tl2", 0)
	tl3 := mustNewIdentifier(t, "", "tl3", 0)

	tl1m := newMockManagerWithTaskListID(ctrl, tl1)
	tl2m := newMockManagerWithTaskListID(ctrl, tl2)
	tl3m := newMockManagerWithTaskListID(ctrl, tl3)

	resolver.EXPECT().Lookup(service.Matching, tl1.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl2.GetName()).Return(membership.NewDetailedHostInfo("", "host456", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl3.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)

	e := matchingEngineImpl{
		shutdown:           make(chan struct{}),
		membershipResolver: resolver,
		taskListRegistry:   tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient()),
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		metricsClient: metrics.NewNoopMetricsClient(),
		logger:        log.NewNoop(),
	}
	e.taskListRegistry.Register(*tl1, tl1m)
	e.taskListRegistry.Register(*tl2, tl2m)
	e.taskListRegistry.Register(*tl3, tl3m)

	wg := sync.WaitGroup{}

	wg.Add(1)

	tl2m.EXPECT().String().AnyTimes()

	tl2m.EXPECT().Stop().Do(func() {
		wg.Done()
	})

	err := e.shutDownNonOwnedTasklists()
	wg.Wait()

	assert.NoError(t, err)
}

func TestUpdateTaskListPartitionConfig(t *testing.T) {
	testCases := []struct {
		name                 string
		req                  *types.MatchingUpdateTaskListPartitionConfigRequest
		enableAdaptiveScaler bool
		hCtx                 *handlerContext
		mockSetup            func(*tasklist.MockManager, *gomock.Controller, *executorclient.MockExecutor[tasklist.ShardProcessor])
		expectError          bool
		expectedError        string
	}{
		{
			name: "success",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(ctrl), nil)
				mockManager.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				}).Return(nil)
			},
			expectError: false,
		},
		{
			name: "tasklist manager error",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				executor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(ctrl), nil)
				mockManager.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				}).Return(errors.New("tasklist manager error"))
			},
			expectError:   true,
			expectedError: "tasklist manager error",
		},
		{
			name: "non root partition error",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/test-tasklist/1",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "Only root partition's partition config can be updated.",
		},
		{
			name: "invalid tasklist name",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "invalid partitioned task list name /__cadence_sys/test-tasklist",
		},
		{
			name: "nil partition config",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "Task list partition config is not set in the request.",
		},
		{
			name: "invalid tasklist kind",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
					Kind: types.TaskListKindSticky.Ptr(),
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "Only normal tasklist's partition config can be updated.",
		},
		{
			name: "manual update not allowed",
			req: &types.MatchingUpdateTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
					Kind: types.TaskListKindSticky.Ptr(),
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			enableAdaptiveScaler: true,
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, executor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "Manual update is not allowed because adaptive scaler is enabled.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain", nil)
			tasklistID := mustNewIdentifier(t, "test-domain-id", "test-tasklist", 1)
			mockManager := newMockManagerWithTaskListID(mockCtrl, tasklistID)
			mockExecutor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
			tc.mockSetup(mockManager, mockCtrl, mockExecutor)
			taskListRegistry := tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient())
			engine := &matchingEngineImpl{
				taskListRegistry: taskListRegistry,
				timeSource:       clock.NewRealTimeSource(),
				domainCache:      mockDomainCache,
				config: &config.Config{
					EnableAdaptiveScaler: dynamicproperties.GetBoolPropertyFilteredByTaskListInfo(tc.enableAdaptiveScaler),
				},
				executor: mockExecutor,
			}
			taskListRegistry.Register(*tasklistID, mockManager)
			_, err := engine.UpdateTaskListPartitionConfig(tc.hCtx, tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRefreshTaskListPartitionConfig(t *testing.T) {
	testCases := []struct {
		name          string
		req           *types.MatchingRefreshTaskListPartitionConfigRequest
		hCtx          *handlerContext
		mockSetup     func(*tasklist.MockManager, *gomock.Controller, *executorclient.MockExecutor[tasklist.ShardProcessor])
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/test-tasklist/1",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, mockCtrl *gomock.Controller, mockExecutor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				mockExecutor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				}).Return(nil)
			},
			expectError: false,
		},
		{
			name: "tasklist manager error",
			req: &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/test-tasklist/1",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, mockCtrl *gomock.Controller, mockExecutor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
				mockExecutor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil)
				mockManager.EXPECT().RefreshTaskListPartitionConfig(gomock.Any(), &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				}).Return(errors.New("tasklist manager error"))
			},
			expectError:   true,
			expectedError: "tasklist manager error",
		},
		{
			name: "invalid tasklist name",
			req: &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "/__cadence_sys/test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, mockExecutor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "invalid partitioned task list name /__cadence_sys/test-tasklist",
		},
		{
			name: "invalid tasklist kind",
			req: &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
					Kind: types.TaskListKindSticky.Ptr(),
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, mockCtrl *gomock.Controller, mockExecutor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "Only normal tasklist's partition config can be updated.",
		},
		{
			name: "invalid request for root partition",
			req: &types.MatchingRefreshTaskListPartitionConfigRequest{
				DomainUUID: "test-domain-id",
				TaskList: &types.TaskList{
					Name: "test-tasklist",
				},
				TaskListType: types.TaskListTypeActivity.Ptr(),
				PartitionConfig: &types.TaskListPartitionConfig{
					Version: 1,
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
					},
				},
			},
			hCtx: &handlerContext{
				Context: context.Background(),
			},
			mockSetup: func(mockManager *tasklist.MockManager, ctrl *gomock.Controller, mockExecutor *executorclient.MockExecutor[tasklist.ShardProcessor]) {
			},
			expectError:   true,
			expectedError: "PartitionConfig must be nil for root partition.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			tasklistID := mustNewIdentifier(t, "test-domain-id", "test-tasklist", 1)
			tasklistID2 := mustNewIdentifier(t, "test-domain-id", "/__cadence_sys/test-tasklist/1", 1)
			mockManager := newMockManagerWithTaskListID(mockCtrl, tasklistID)
			mockExecutor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
			tc.mockSetup(mockManager, mockCtrl, mockExecutor)
			taskListRegistry := tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient())
			engine := &matchingEngineImpl{
				taskListRegistry: taskListRegistry,
				timeSource:       clock.NewRealTimeSource(),
				executor:         mockExecutor,
			}
			taskListRegistry.Register(*tasklistID, mockManager)
			taskListRegistry.Register(*tasklistID2, mockManager)
			_, err := engine.RefreshTaskListPartitionConfig(tc.hCtx, tc.req)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_domainChangeCallback(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	mockDomainCache := cache.NewMockDomainCache(mockCtrl)

	clusters := []string{"cluster0", "cluster1"}

	tlGlobalDecision1 := mustNewIdentifier(t, "global-domain-1-id", "global-domain-1", persistence.TaskListTypeDecision)
	tlGlobalActivity1 := mustNewIdentifier(t, "global-domain-1-id", "global-domain-1", persistence.TaskListTypeActivity)
	tlGlobalDecision2 := mustNewIdentifier(t, "global-domain-2-id", "global-domain-2", persistence.TaskListTypeDecision)
	tlGlobalActivity2 := mustNewIdentifier(t, "global-domain-2-id", "global-domain-2", persistence.TaskListTypeActivity)
	tlGlobalSticky2 := mustNewIdentifier(t, "global-domain-2-id", "sticky-global-domain-2", persistence.TaskListTypeDecision)
	tlGlobalActivity3 := mustNewIdentifier(t, "global-domain-3-id", "global-domain-3", persistence.TaskListTypeActivity)
	tlGlobalDecision3 := mustNewIdentifier(t, "global-domain-3-id", "global-domain-3", persistence.TaskListTypeDecision)
	tlGlobalSticky3 := mustNewIdentifier(t, "global-domain-3-id", "sticky-global-domain-3", persistence.TaskListTypeDecision)
	tlLocalDecision1 := mustNewIdentifier(t, "local-domain-1-id", "local-domain-1", persistence.TaskListTypeDecision)
	tlLocalActivity1 := mustNewIdentifier(t, "local-domain-1-id", "local-domain-1", persistence.TaskListTypeActivity)
	tlActiveActiveDecision1 := mustNewIdentifier(t, "active-active-domain-1-id", "active-active-domain-1", persistence.TaskListTypeDecision)
	tlActiveActiveActivity1 := mustNewIdentifier(t, "active-active-domain-1-id", "active-active-domain-1", persistence.TaskListTypeActivity)

	newNormalManager := func(id *tasklist.Identifier) *tasklist.MockManager {
		mgr := newMockManagerWithTaskListID(mockCtrl, id)
		mgr.EXPECT().GetTaskListKind().Return(types.TaskListKindNormal).AnyTimes()
		mgr.EXPECT().DescribeTaskList(gomock.Any()).Return(&types.DescribeTaskListResponse{}).AnyTimes()
		return mgr
	}
	newStickyManager := func(id *tasklist.Identifier) *tasklist.MockManager {
		mgr := newMockManagerWithTaskListID(mockCtrl, id)
		mgr.EXPECT().GetTaskListKind().Return(types.TaskListKindSticky).AnyTimes()
		mgr.EXPECT().DescribeTaskList(gomock.Any()).Return(&types.DescribeTaskListResponse{}).AnyTimes()
		return mgr
	}

	mockGlobalDecision1 := newNormalManager(tlGlobalDecision1)
	mockGlobalActivity1 := newNormalManager(tlGlobalActivity1)
	mockGlobalDecision2 := newNormalManager(tlGlobalDecision2)
	mockGlobalActivity2 := newNormalManager(tlGlobalActivity2)
	mockGlobalSticky2 := newStickyManager(tlGlobalSticky2)
	mockGlobalDecision3 := newNormalManager(tlGlobalDecision3)
	mockGlobalActivity3 := newNormalManager(tlGlobalActivity3)
	mockGlobalSticky3 := newStickyManager(tlGlobalSticky3)
	mockLocalDecision1 := newNormalManager(tlLocalDecision1)
	mockLocalActivity1 := newNormalManager(tlLocalActivity1)
	mockActiveActiveDecision1 := newNormalManager(tlActiveActiveDecision1)
	mockActiveActiveActivity1 := newNormalManager(tlActiveActiveActivity1)

	mockExecutor := executorclient.NewMockExecutor[tasklist.ShardProcessor](mockCtrl)
	mockExecutor.EXPECT().GetShardProcess(gomock.Any(), gomock.Any()).Return(tasklist.NewMockShardProcessor(mockCtrl), nil).AnyTimes()

	engine := &matchingEngineImpl{
		domainCache:                 mockDomainCache,
		failoverNotificationVersion: 1,
		config:                      defaultTestConfig(),
		logger:                      log.NewNoop(),
		taskListRegistry:            tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient()),
		executor:                    mockExecutor,
	}
	engine.taskListRegistry.Register(*tlGlobalDecision1, mockGlobalDecision1)
	engine.taskListRegistry.Register(*tlGlobalActivity1, mockGlobalActivity1)
	engine.taskListRegistry.Register(*tlGlobalDecision2, mockGlobalDecision2)
	engine.taskListRegistry.Register(*tlGlobalActivity2, mockGlobalActivity2)
	engine.taskListRegistry.Register(*tlGlobalSticky2, mockGlobalSticky2)
	engine.taskListRegistry.Register(*tlGlobalDecision3, mockGlobalDecision3)
	engine.taskListRegistry.Register(*tlGlobalActivity3, mockGlobalActivity3)
	engine.taskListRegistry.Register(*tlGlobalSticky3, mockGlobalSticky3)
	engine.taskListRegistry.Register(*tlLocalDecision1, mockLocalDecision1)
	engine.taskListRegistry.Register(*tlLocalActivity1, mockLocalActivity1)
	engine.taskListRegistry.Register(*tlActiveActiveDecision1, mockActiveActiveDecision1)
	engine.taskListRegistry.Register(*tlActiveActiveActivity1, mockActiveActiveActivity1)

	// Eligible for failover handling is defined by isDomainEligibleToDisconnectPollers.
	mockGlobalDecision1.EXPECT().ReleaseBlockedPollers().Times(0)       // global-domain-1 has failover version 0 (<= current 1), so not eligible.
	mockGlobalActivity1.EXPECT().ReleaseBlockedPollers().Times(0)       // global-domain-1 has failover version 0 (<= current 1), so not eligible.
	mockGlobalDecision2.EXPECT().ReleaseBlockedPollers().Times(1)       // global-domain-2 is global, non-active-active, and version 4 > 1.
	mockGlobalActivity2.EXPECT().ReleaseBlockedPollers().Times(1)       // global-domain-2 is global, non-active-active, and version 4 > 1.
	mockGlobalSticky2.EXPECT().ReleaseBlockedPollers().Times(1)         // sticky task list under eligible global-domain-2.
	mockGlobalDecision3.EXPECT().ReleaseBlockedPollers().Times(1)       // global-domain-3 is eligible.
	mockGlobalActivity3.EXPECT().ReleaseBlockedPollers().Times(1)       // global-domain-3 is eligible.
	mockGlobalSticky3.EXPECT().ReleaseBlockedPollers().Times(1)         // sticky task list under eligible global-domain-3.
	mockLocalDecision1.EXPECT().ReleaseBlockedPollers().Times(0)        // local domains are not eligible.
	mockLocalActivity1.EXPECT().ReleaseBlockedPollers().Times(0)        // local domains are not eligible.
	mockActiveActiveDecision1.EXPECT().ReleaseBlockedPollers().Times(0) // active-active domains are not eligible.
	mockActiveActiveActivity1.EXPECT().ReleaseBlockedPollers().Times(0) // active-active domains are not eligible.

	domains := []*cache.DomainCacheEntry{
		cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "global-domain-1", ID: "global-domain-1-id"},
			nil,
			true,
			&persistence.DomainReplicationConfig{ActiveClusterName: clusters[0], Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: "cluster0"}, {ClusterName: "cluster1"}}},
			0,
			nil,
			0,
			0,
			6,
		),
		cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "global-domain-2", ID: "global-domain-2-id"},
			nil,
			true,
			&persistence.DomainReplicationConfig{ActiveClusterName: clusters[1], Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: "cluster0"}, {ClusterName: "cluster1"}}},
			0,
			nil,
			4,
			0,
			4,
		),
		cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "global-domain-3", ID: "global-domain-3-id"},
			nil,
			true,
			&persistence.DomainReplicationConfig{ActiveClusterName: clusters[1], Clusters: []*persistence.ClusterReplicationConfig{{ClusterName: "cluster0"}, {ClusterName: "cluster1"}}},
			0,
			nil,
			5,
			0,
			5,
		),
		cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "local-domain-1", ID: "local-domain-1-id"},
			nil,
			false,
			nil,
			0,
			nil,
			0,
			0,
			3,
		),
		cache.NewDomainCacheEntryForTest(
			&persistence.DomainInfo{Name: "active-active-domain-1", ID: "active-active-domain-1-id"},
			nil,
			true,
			&persistence.DomainReplicationConfig{ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"us-west": {
								ActiveClusterName: "cluster0",
								FailoverVersion:   1,
							},
						},
					},
				},
			}},
			0,
			nil,
			0,
			0,
			2,
		),
	}

	engine.domainChangeCallback(domains)

	assert.Equal(t, int64(5), engine.failoverNotificationVersion, "5 is the highest failover notification version in the fixtures (global-domain-3)")
}

func Test_registerDomainFailoverCallback(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockDomainCache := cache.NewMockDomainCache(ctrl)

	// Capture the registered catchUpFn
	var registeredCatchUpFn func(cache.DomainCache, cache.PrepareCallbackFn, cache.CallbackFn)
	mockDomainCache.EXPECT().RegisterDomainChangeCallback(
		service.Matching, // id of the callback
		gomock.Any(),     // catchUpFn
		gomock.Any(),     // lockTaskProcessingForDomainUpdate
		gomock.Any(),     // domainChangeCB
	).Do(func(_ string, catchUpFn, _, _ interface{}) {
		if fn, ok := catchUpFn.(cache.CatchUpFn); ok {
			registeredCatchUpFn = fn
		} else {
			t.Fatalf("Failed to convert catchUpFn to cache.CatchUpFn: got type %T", catchUpFn)
		}
	}).Times(1)

	engine := &matchingEngineImpl{
		domainCache:                 mockDomainCache,
		failoverNotificationVersion: 0,
		config:                      defaultTestConfig(),
		logger:                      log.NewNoop(),
		taskListRegistry:            tasklist.NewTaskListRegistry(metrics.NewNoopMetricsClient()),
	}

	engine.registerDomainFailoverCallback()

	t.Run("catchUpFn - No failoverNotificationVersion updates", func(t *testing.T) {
		mockDomainCache.EXPECT().GetAllDomain().Return(map[string]*cache.DomainCacheEntry{
			"uuid-domain1": cache.NewDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: "uuid-domain1", Name: "domain1"},
				nil,
				true,
				&persistence.DomainReplicationConfig{ActiveClusterName: "A"},
				0,
				nil,
				0,
				0,
				1,
			),
			"uuid-domain2": cache.NewDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: "uuid-domain2", Name: "domain2"},
				nil,
				true,
				&persistence.DomainReplicationConfig{ActiveClusterName: "A"},
				0,
				nil,
				0,
				0,
				4,
			),
		})

		prepareCalled := false
		callbackCalled := false
		prepare := func() { prepareCalled = true }
		callback := func([]*cache.DomainCacheEntry) { callbackCalled = true }

		if registeredCatchUpFn != nil {
			registeredCatchUpFn(mockDomainCache, prepare, callback)
			assert.False(t, prepareCalled, "prepareCallback should not be called")
			assert.False(t, callbackCalled, "callback should not be called")
		} else {
			assert.Fail(t, "catchUpFn was not registered")
		}

		assert.Equal(t, int64(0), engine.failoverNotificationVersion)
	})

	t.Run("catchUpFn - No failoverNotificationVersion updates", func(t *testing.T) {
		mockDomainCache.EXPECT().GetAllDomain().Return(map[string]*cache.DomainCacheEntry{
			"uuid-domain1": cache.NewDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: "uuid-domain1", Name: "domain1"},
				nil,
				true,
				&persistence.DomainReplicationConfig{ActiveClusterName: "A"},
				0,
				nil,
				3,
				0,
				3,
			),
			"uuid-domain2": cache.NewDomainCacheEntryForTest(
				&persistence.DomainInfo{ID: "uuid-domain2", Name: "domain2"},
				nil,
				true,
				&persistence.DomainReplicationConfig{ActiveClusterName: "A"},
				0,
				nil,
				2,
				0,
				4,
			),
		})

		prepareCalled := false
		callbackCalled := false
		prepare := func() { prepareCalled = true }
		callback := func([]*cache.DomainCacheEntry) { callbackCalled = true }

		if registeredCatchUpFn != nil {
			registeredCatchUpFn(mockDomainCache, prepare, callback)
			assert.False(t, prepareCalled, "prepareCallback should not be called")
			assert.False(t, callbackCalled, "callback should not be called")
		} else {
			assert.Fail(t, "catchUpFn was not registered")
		}

		assert.Equal(t, int64(3), engine.failoverNotificationVersion)
	})

}

func TestRefreshWorkflowTasks(t *testing.T) {
	testCases := []struct {
		name              string
		ctx               context.Context
		domainID          string
		workflowExecution *types.WorkflowExecution
		mockSetup         func(*history.MockClient)
		wantErr           bool
		assertErr         func(*testing.T, error)
	}{
		{
			name:     "success",
			ctx:      context.Background(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					&types.HistoryRefreshWorkflowTasksRequest{
						DomainUIID: "test-domain-id",
						Request: &types.RefreshWorkflowTasksRequest{
							Execution: &types.WorkflowExecution{
								WorkflowID: "test-workflow-id",
								RunID:      "test-run-id",
							},
						},
					},
				).Return(nil)
			},
			wantErr: false,
		},
		{
			name:     "entity not exists error - returns nil",
			ctx:      context.Background(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					gomock.Any(),
				).Return(&types.EntityNotExistsError{Message: "workflow not found"})
			},
			wantErr: false,
		},
		{
			name:     "internal service error",
			ctx:      context.Background(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					gomock.Any(),
				).Return(&types.InternalServiceError{Message: "internal error"})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var serviceErr *types.InternalServiceError
				assert.ErrorAs(t, err, &serviceErr)
				assert.Equal(t, "internal error", serviceErr.Message)
			},
		},
		{
			name:     "generic error propagated",
			ctx:      context.Background(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					gomock.Any(),
				).Return(errors.New("some error"))
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, "some error", err.Error())
			},
		},
		{
			name:     "workflow execution already completed error",
			ctx:      context.Background(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					gomock.Any(),
				).Return(&types.WorkflowExecutionAlreadyCompletedError{Message: "workflow already completed"})
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				var completedErr *types.WorkflowExecutionAlreadyCompletedError
				assert.ErrorAs(t, err, &completedErr)
				assert.Equal(t, "workflow already completed", completedErr.Message)
			},
		},
		{
			name:     "context canceled",
			ctx:      func() context.Context { ctx, cancel := context.WithCancel(context.Background()); cancel(); return ctx }(),
			domainID: "test-domain-id",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					gomock.Any(),
				).Return(context.Canceled)
			},
			wantErr: true,
			assertErr: func(t *testing.T, err error) {
				assert.Equal(t, context.Canceled, err)
			},
		},
		{
			name:     "empty domain id",
			ctx:      context.Background(),
			domainID: "",
			workflowExecution: &types.WorkflowExecution{
				WorkflowID: "test-workflow-id",
				RunID:      "test-run-id",
			},
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					&types.HistoryRefreshWorkflowTasksRequest{
						DomainUIID: "",
						Request: &types.RefreshWorkflowTasksRequest{
							Execution: &types.WorkflowExecution{
								WorkflowID: "test-workflow-id",
								RunID:      "test-run-id",
							},
						},
					},
				).Return(nil)
			},
			wantErr: false,
		},
		{
			name:              "nil workflow execution",
			ctx:               context.Background(),
			domainID:          "test-domain-id",
			workflowExecution: nil,
			mockSetup: func(mockHistoryService *history.MockClient) {
				mockHistoryService.EXPECT().RefreshWorkflowTasks(
					gomock.Any(),
					&types.HistoryRefreshWorkflowTasksRequest{
						DomainUIID: "test-domain-id",
						Request: &types.RefreshWorkflowTasksRequest{
							Execution: nil,
						},
					},
				).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockHistoryService := history.NewMockClient(mockCtrl)
			tc.mockSetup(mockHistoryService)

			engine := &matchingEngineImpl{
				historyService: mockHistoryService,
			}

			err := engine.refreshWorkflowTasks(tc.ctx, tc.domainID, tc.workflowExecution)

			if tc.wantErr {
				require.Error(t, err)
				if tc.assertErr != nil {
					tc.assertErr(t, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
