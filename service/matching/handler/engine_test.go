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

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/client"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/matching/config"
	"github.com/uber/cadence/service/matching/tasklist"
)

func TestGetTaskListsByDomain(t *testing.T) {
	testCases := []struct {
		name      string
		mockSetup func(*cache.MockDomainCache, map[tasklist.Identifier]*tasklist.MockManager, map[tasklist.Identifier]*tasklist.MockManager)
		wantErr   bool
		want      *types.GetTaskListsByDomainResponse
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(mockCtrl)
			decisionTasklistID, err := tasklist.NewIdentifier("test-domain-id", "decision0", 0)
			require.NoError(t, err)
			activityTasklistID, err := tasklist.NewIdentifier("test-domain-id", "activity0", 1)
			require.NoError(t, err)
			otherDomainTasklistID, err := tasklist.NewIdentifier("other-domain-id", "other0", 0)
			require.NoError(t, err)
			mockDecisionTaskListManager := tasklist.NewMockManager(mockCtrl)
			mockActivityTaskListManager := tasklist.NewMockManager(mockCtrl)
			mockOtherDomainTaskListManager := tasklist.NewMockManager(mockCtrl)
			mockTaskListManagers := map[tasklist.Identifier]*tasklist.MockManager{
				*decisionTasklistID:    mockDecisionTaskListManager,
				*activityTasklistID:    mockActivityTaskListManager,
				*otherDomainTasklistID: mockOtherDomainTaskListManager,
			}
			stickyTasklistID, err := tasklist.NewIdentifier("test-domain-id", "sticky0", 0)
			require.NoError(t, err)
			mockStickyManager := tasklist.NewMockManager(mockCtrl)
			mockStickyManagers := map[tasklist.Identifier]*tasklist.MockManager{
				*stickyTasklistID: mockStickyManager,
			}
			tc.mockSetup(mockDomainCache, mockTaskListManagers, mockStickyManagers)

			engine := &matchingEngineImpl{
				domainCache: mockDomainCache,
				taskLists: map[tasklist.Identifier]tasklist.Manager{
					*decisionTasklistID:    mockDecisionTaskListManager,
					*activityTasklistID:    mockActivityTaskListManager,
					*otherDomainTasklistID: mockOtherDomainTaskListManager,
					*stickyTasklistID:      mockStickyManager,
				},
			}
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
		mockSetup func(*tasklist.MockManager)
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
				mockManager.EXPECT().CancelPoller("test-poller-id")
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockManager := tasklist.NewMockManager(mockCtrl)
			tc.mockSetup(mockManager)
			tasklistID, err := tasklist.NewIdentifier("test-domain-id", "test-tasklist", 0)
			require.NoError(t, err)
			engine := &matchingEngineImpl{
				taskLists: map[tasklist.Identifier]tasklist.Manager{
					*tasklistID: mockManager,
				},
			}
			err = engine.CancelOutstandingPoll(nil, tc.req)
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
		mockSetup func(*tasklist.MockManager, *lockableQueryTaskMap)
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
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap) {},
			wantErr:   true,
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
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap) {
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
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap) {
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
			mockSetup: func(mockManager *tasklist.MockManager, queryResultMap *lockableQueryTaskMap) {
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
			mockManager := tasklist.NewMockManager(mockCtrl)
			tasklistID, err := tasklist.NewIdentifier("test-domain-id", "test-tasklist", 0)
			require.NoError(t, err)
			engine := &matchingEngineImpl{
				taskLists: map[tasklist.Identifier]tasklist.Manager{
					*tasklistID: mockManager,
				},
				timeSource:           clock.NewRealTimeSource(),
				lockableQueryTaskMap: lockableQueryTaskMap{queryTaskMap: make(map[string]chan *queryResult)},
			}
			tc.mockSetup(mockManager, &engine.lockableQueryTaskMap)
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
	mockDomainCache := cache.NewMockDomainCache(gomock.NewController(t))
	mockDomainCache.EXPECT().RegisterDomainChangeCallback(service.Matching, gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
	mockDomainCache.EXPECT().UnregisterDomainChangeCallback(service.Matching).Times(1)
	e := matchingEngineImpl{
		domainCache:        mockDomainCache,
		shutdownCompletion: &wg,
		shutdown:           make(chan struct{}),
	}
	e.Start()
	assert.False(t, e.isShuttingDown())
	e.Stop()
	assert.True(t, e.isShuttingDown())
}

func TestGetTasklistsNotOwned(t *testing.T) {

	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)

	resolver.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("self", "host123", nil), nil)

	tl1, _ := tasklist.NewIdentifier("", "tl1", 0)
	tl2, _ := tasklist.NewIdentifier("", "tl2", 0)
	tl3, _ := tasklist.NewIdentifier("", "tl3", 0)

	tl1m := tasklist.NewMockManager(ctrl)
	tl2m := tasklist.NewMockManager(ctrl)
	tl3m := tasklist.NewMockManager(ctrl)

	resolver.EXPECT().Lookup(service.Matching, tl1.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl2.GetName()).Return(membership.NewDetailedHostInfo("", "host456", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl3.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)

	e := matchingEngineImpl{
		shutdown:           make(chan struct{}),
		membershipResolver: resolver,
		taskListsLock:      sync.RWMutex{},
		taskLists: map[tasklist.Identifier]tasklist.Manager{
			*tl1: tl1m,
			*tl2: tl2m,
			*tl3: tl3m,
		},
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		logger: log.NewNoop(),
	}

	tls, err := e.getNonOwnedTasklistsLocked()
	assert.NoError(t, err)

	assert.Equal(t, []tasklist.Manager{tl2m}, tls)
}

func TestShutDownTasklistsNotOwned(t *testing.T) {

	ctrl := gomock.NewController(t)
	resolver := membership.NewMockResolver(ctrl)

	resolver.EXPECT().WhoAmI().Return(membership.NewDetailedHostInfo("self", "host123", nil), nil)

	tl1, _ := tasklist.NewIdentifier("", "tl1", 0)
	tl2, _ := tasklist.NewIdentifier("", "tl2", 0)
	tl3, _ := tasklist.NewIdentifier("", "tl3", 0)

	tl1m := tasklist.NewMockManager(ctrl)
	tl2m := tasklist.NewMockManager(ctrl)
	tl3m := tasklist.NewMockManager(ctrl)

	resolver.EXPECT().Lookup(service.Matching, tl1.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl2.GetName()).Return(membership.NewDetailedHostInfo("", "host456", nil), nil)
	resolver.EXPECT().Lookup(service.Matching, tl3.GetName()).Return(membership.NewDetailedHostInfo("", "host123", nil), nil)

	e := matchingEngineImpl{
		shutdown:           make(chan struct{}),
		membershipResolver: resolver,
		taskListsLock:      sync.RWMutex{},
		taskLists: map[tasklist.Identifier]tasklist.Manager{
			*tl1: tl1m,
			*tl2: tl2m,
			*tl3: tl3m,
		},
		config: &config.Config{
			EnableTasklistOwnershipGuard: func(opts ...dynamicproperties.FilterOption) bool { return true },
		},
		metricsClient: metrics.NewNoopMetricsClient(),
		logger:        log.NewNoop(),
	}

	wg := sync.WaitGroup{}

	wg.Add(1)

	tl2m.EXPECT().TaskListID().Return(tl2).AnyTimes()
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
		mockSetup            func(*tasklist.MockManager)
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockManager := tasklist.NewMockManager(mockCtrl)
			tc.mockSetup(mockManager)
			tasklistID, err := tasklist.NewIdentifier("test-domain-id", "test-tasklist", 1)
			require.NoError(t, err)
			engine := &matchingEngineImpl{
				taskLists: map[tasklist.Identifier]tasklist.Manager{
					*tasklistID: mockManager,
				},
				timeSource:  clock.NewRealTimeSource(),
				domainCache: mockDomainCache,
				config: &config.Config{
					EnableAdaptiveScaler: dynamicproperties.GetBoolPropertyFilteredByTaskListInfo(tc.enableAdaptiveScaler),
				},
			}
			_, err = engine.UpdateTaskListPartitionConfig(tc.hCtx, tc.req)
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
		mockSetup     func(*tasklist.MockManager)
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
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
			mockSetup: func(mockManager *tasklist.MockManager) {
			},
			expectError:   true,
			expectedError: "PartitionConfig must be nil for root partition.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockManager := tasklist.NewMockManager(mockCtrl)
			tc.mockSetup(mockManager)
			tasklistID, err := tasklist.NewIdentifier("test-domain-id", "test-tasklist", 1)
			require.NoError(t, err)
			tasklistID2, err := tasklist.NewIdentifier("test-domain-id", "/__cadence_sys/test-tasklist/1", 1)
			require.NoError(t, err)
			engine := &matchingEngineImpl{
				taskLists: map[tasklist.Identifier]tasklist.Manager{
					*tasklistID:  mockManager,
					*tasklistID2: mockManager,
				},
				timeSource: clock.NewRealTimeSource(),
			}
			_, err = engine.RefreshTaskListPartitionConfig(tc.hCtx, tc.req)
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

	mockTaskListManagerGlobal1 := tasklist.NewMockManager(mockCtrl)
	mockTaskListManagerGlobal2 := tasklist.NewMockManager(mockCtrl)
	mockTaskListManagerGlobal3 := tasklist.NewMockManager(mockCtrl)
	mockTaskListManagerLocal1 := tasklist.NewMockManager(mockCtrl)
	mockTaskListManagerActiveActive1 := tasklist.NewMockManager(mockCtrl)

	tlIdentifierDecisionGlobal1, _ := tasklist.NewIdentifier("global-domain-1-id", "global-domain-1", persistence.TaskListTypeDecision)
	tlIdentifierActivityGlobal1, _ := tasklist.NewIdentifier("global-domain-1-id", "global-domain-1", persistence.TaskListTypeActivity)
	tlIdentifierDecisionGlobal2, _ := tasklist.NewIdentifier("global-domain-2-id", "global-domain-2", persistence.TaskListTypeDecision)
	tlIdentifierActivityGlobal2, _ := tasklist.NewIdentifier("global-domain-2-id", "global-domain-2", persistence.TaskListTypeActivity)
	tlIdentifierActivityGlobal3, _ := tasklist.NewIdentifier("global-domain-3-id", "global-domain-3", persistence.TaskListTypeActivity)
	tlIdentifierDecisionGlobal3, _ := tasklist.NewIdentifier("global-domain-3-id", "global-domain-3", persistence.TaskListTypeDecision)
	tlIdentifierDecisionLocal1, _ := tasklist.NewIdentifier("local-domain-1-id", "local-domain-1", persistence.TaskListTypeDecision)
	tlIdentifierActivityLocal1, _ := tasklist.NewIdentifier("local-domain-1-id", "local-domain-1", persistence.TaskListTypeActivity)
	tlIdentifierDecisionActiveActive1, _ := tasklist.NewIdentifier("active-active-domain-1-id", "active-active-domain-1", persistence.TaskListTypeDecision)
	tlIdentifierActivityActiveActive1, _ := tasklist.NewIdentifier("active-active-domain-1-id", "active-active-domain-1", persistence.TaskListTypeActivity)

	engine := &matchingEngineImpl{
		domainCache:                 mockDomainCache,
		failoverNotificationVersion: 1,
		config:                      defaultTestConfig(),
		logger:                      log.NewNoop(),
		taskLists: map[tasklist.Identifier]tasklist.Manager{
			*tlIdentifierDecisionGlobal1:       mockTaskListManagerGlobal1,
			*tlIdentifierActivityGlobal1:       mockTaskListManagerGlobal1,
			*tlIdentifierDecisionGlobal2:       mockTaskListManagerGlobal2,
			*tlIdentifierActivityGlobal2:       mockTaskListManagerGlobal2,
			*tlIdentifierDecisionGlobal3:       mockTaskListManagerGlobal3,
			*tlIdentifierActivityGlobal3:       mockTaskListManagerGlobal3,
			*tlIdentifierDecisionLocal1:        mockTaskListManagerLocal1,
			*tlIdentifierActivityLocal1:        mockTaskListManagerLocal1,
			*tlIdentifierDecisionActiveActive1: mockTaskListManagerActiveActive1,
			*tlIdentifierActivityActiveActive1: mockTaskListManagerActiveActive1,
		},
	}

	mockTaskListManagerGlobal1.EXPECT().ReleaseBlockedPollers().Times(0)
	mockTaskListManagerGlobal2.EXPECT().GetTaskListKind().Return(types.TaskListKindNormal).Times(2)
	mockTaskListManagerGlobal2.EXPECT().DescribeTaskList(gomock.Any()).Return(&types.DescribeTaskListResponse{}).Times(2)
	mockTaskListManagerGlobal2.EXPECT().ReleaseBlockedPollers().Times(2)
	mockTaskListManagerGlobal3.EXPECT().GetTaskListKind().Return(types.TaskListKindNormal).Times(2)
	mockTaskListManagerGlobal3.EXPECT().DescribeTaskList(gomock.Any()).Return(&types.DescribeTaskListResponse{}).Times(2)
	mockTaskListManagerGlobal3.EXPECT().ReleaseBlockedPollers().Return(errors.New("some-error")).Times(2)
	mockTaskListManagerLocal1.EXPECT().ReleaseBlockedPollers().Times(0)
	mockTaskListManagerActiveActive1.EXPECT().ReleaseBlockedPollers().Times(0)
	mockDomainCache.EXPECT().GetDomainID("global-domain-2").Return("global-domain-2-id", nil).Times(1)
	mockDomainCache.EXPECT().GetDomainID("global-domain-3").Return("global-domain-3-id", nil).Times(1)

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
				ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
					"us-west": {
						ActiveClusterName: "cluster0",
						FailoverVersion:   1,
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

	assert.Equal(t, int64(5), engine.failoverNotificationVersion)
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
		taskLists:                   map[tasklist.Identifier]tasklist.Manager{},
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
