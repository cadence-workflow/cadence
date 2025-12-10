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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testNamespaceFixed     = "test-fixed"
	_testNamespaceEphemeral = "test-ephemeral"
)

func TestGetShardOwner(t *testing.T) {
	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{
				Name:     _testNamespaceFixed,
				Type:     config.NamespaceTypeFixed,
				ShardNum: 32,
			},
			{
				Name: _testNamespaceEphemeral,
				Type: config.NamespaceTypeEphemeral,
			},
		},
	}

	tests := []struct {
		name           string
		request        *types.GetShardOwnerRequest
		setupMocks     func(mockStore *store.MockStore)
		expectedOwner  string
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "InvalidNamespace",
			request: &types.GetShardOwnerRequest{
				Namespace: "namespace not found invalidNamespace",
				ShardKey:  "1",
			},
			expectedError:  true,
			expectedErrMsg: "namespace not found",
		},
		{
			name: "LookupError",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceFixed,
				ShardKey:  "1",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "1").Return(nil, errors.New("lookup error"))
			},
			expectedError:  true,
			expectedErrMsg: "lookup error",
		},
		{
			name: "Existing_Success_Fixed",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceFixed,
				ShardKey:  "123",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "123").Return(&store.ShardOwner{
					ExecutorID: "owner1",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},
		{
			name: "ShardNotFound_Fixed",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceFixed,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "NON-EXISTING-SHARD").Return(nil, store.ErrShardNotFound)
			},
			expectedError:  true,
			expectedErrMsg: "shard not found",
		},
		{
			name: "Existing_Success_Ephemeral",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "123",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "123").Return(&store.ShardOwner{
					ExecutorID: "owner1",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},
		{
			name: "ShardNotFound_Ephemeral",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusACTIVE},
						"owner2": {Status: types.ExecutorStatusACTIVE},
					},
					ShardAssignments: map[string]store.AssignedState{
						"owner1": {
							AssignedShards: map[string]*types.ShardAssignment{
								"shard1": {Status: types.AssignmentStatusREADY},
								"shard2": {Status: types.AssignmentStatusREADY},
								"shard3": {Status: types.AssignmentStatusREADY},
							},
						},
						"owner2": {
							AssignedShards: map[string]*types.ShardAssignment{
								"shard4": {Status: types.AssignmentStatusREADY},
							},
						},
					},
				}, nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner2").Return(&store.ShardOwner{
					ExecutorID: "owner2",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
				// owner2 has the fewest shards assigned, so we assign the shard to it
				mockStore.EXPECT().AssignShard(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD", "owner2").Return(nil)
			},
			expectedOwner: "owner2",
			expectedError: false,
		},
		{
			name: "ShardNotFound_Ephemeral_GetStateFailure",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(nil, errors.New("get state failure"))
			},
			expectedError:  true,
			expectedErrMsg: "get state failure",
		},
		{
			name: "ShardNotFound_Ephemeral_AssignShardFailure",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusACTIVE},
					},
					ShardAssignments: map[string]store.AssignedState{
						"owner1": {AssignedShards: map[string]*types.ShardAssignment{}},
					},
				}, nil)
				mockStore.EXPECT().AssignShard(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD", "owner1").Return(errors.New("assign shard failure"))
			},
			expectedError:  true,
			expectedErrMsg: "assign shard failure",
		},
		{
			name: "ShardNotFound_Ephemeral_LoadBased",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "new-shard",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "new-shard").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusACTIVE},
						"owner2": {Status: types.ExecutorStatusACTIVE},
					},
					ShardAssignments: map[string]store.AssignedState{
						"owner1": {
							AssignedShards: map[string]*types.ShardAssignment{
								"shard1": {Status: types.AssignmentStatusREADY},
								"shard2": {Status: types.AssignmentStatusREADY},
							},
						},
						"owner2": {
							AssignedShards: map[string]*types.ShardAssignment{
								"shard3": {Status: types.AssignmentStatusREADY},
							},
						},
					},
					ShardStats: map[string]store.ShardStatistics{
						"shard1": {SmoothedLoad: 2.5},
						"shard2": {SmoothedLoad: 1.0},
						"shard3": {SmoothedLoad: 0.5},
					},
				}, nil)
				// owner1 total load: 2.5 + 1.0 = 3.5
				// owner2 total load: 0.5
				// Should pick owner2 (least loaded)
				mockStore.EXPECT().AssignShard(gomock.Any(), _testNamespaceEphemeral, "new-shard", "owner2").Return(nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner2").Return(&store.ShardOwner{
					ExecutorID: "owner2",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwner: "owner2",
			expectedError: false,
		},
		{
			name: "ShardNotFound_Ephemeral_AllExecutorsDraining",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "new-shard",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "new-shard").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusDRAINING},
					},
					ShardAssignments: map[string]store.AssignedState{
						"owner1": {
							AssignedShards: map[string]*types.ShardAssignment{
								"shard1": {Status: types.AssignmentStatusREADY},
							},
						},
					},
				}, nil)
			},
			expectedError:  true,
			expectedErrMsg: "no active executors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			logger := testlogger.New(t)
			mockStorage := store.NewMockStore(ctrl)

			handler := &handlerImpl{
				logger:               logger,
				shardDistributionCfg: cfg,
				storage:              mockStorage,
			}
			if tt.setupMocks != nil {
				tt.setupMocks(mockStorage)
			}
			resp, err := handler.GetShardOwner(context.Background(), tt.request)
			if tt.expectedError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedOwner, resp.Owner)
				require.Equal(t, tt.request.Namespace, resp.Namespace)
				expectedMetadata := map[string]string{"ip": "127.0.0.1", "port": "1234"}
				require.Equal(t, expectedMetadata, resp.Metadata)
			}
		})
	}
}

func TestPickLeastLoadedExecutor(t *testing.T) {
	tests := []struct {
		name          string
		state         *store.NamespaceState
		expectedOwner string
		expectedLoad  float64
		expectedCount int
		expectedError bool
	}{
		{
			name: "SelectsLeastLoaded",
			state: &store.NamespaceState{
				Executors: map[string]store.HeartbeatState{
					"exec1": {Status: types.ExecutorStatusACTIVE},
					"exec2": {Status: types.ExecutorStatusACTIVE},
				},
				ShardAssignments: map[string]store.AssignedState{
					"exec1": {
						AssignedShards: map[string]*types.ShardAssignment{
							"shard1": {},
							"shard2": {},
							"shard3": {},
							"shard4": {},
						},
					},
					"exec2": {
						AssignedShards: map[string]*types.ShardAssignment{
							"shard4": {},
							"shard5": {},
						},
					},
				},
				ShardStats: map[string]store.ShardStatistics{
					"shard1": {SmoothedLoad: 1.0},
					"shard2": {SmoothedLoad: 2.0},
					"shard3": {SmoothedLoad: 0.5},
					"shard4": {SmoothedLoad: 0.25},
					"shard5": {SmoothedLoad: 1.0},
				},
			},
			expectedOwner: "exec2",
			expectedLoad:  1.25,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name: "SkipsNonActiveExecutors",
			state: &store.NamespaceState{
				Executors: map[string]store.HeartbeatState{
					"exec1": {Status: types.ExecutorStatusDRAINING},
					"exec2": {Status: types.ExecutorStatusACTIVE},
				},
				ShardAssignments: map[string]store.AssignedState{
					"exec1": {
						AssignedShards: map[string]*types.ShardAssignment{
							"shard1": {},
						},
					},
					"exec2": {
						AssignedShards: map[string]*types.ShardAssignment{
							"shard2": {},
							"shard3": {},
						},
					},
				},
				ShardStats: map[string]store.ShardStatistics{
					"shard1": {SmoothedLoad: 0.1},
					"shard2": {SmoothedLoad: 1.0},
					"shard3": {SmoothedLoad: 2.0},
				},
			},
			expectedOwner: "exec2",
			expectedLoad:  3.0,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "SelectsLeastLoaded_NoExecutors",
			state:         &store.NamespaceState{},
			expectedError: true,
		},
		{
			name: "SelectsLeastLoaded_NoActiveExecutors",
			state: &store.NamespaceState{
				Executors: map[string]store.HeartbeatState{
					"exec1": {Status: types.ExecutorStatusDRAINING},
				},
				ShardAssignments: map[string]store.AssignedState{
					"exec1": {
						AssignedShards: map[string]*types.ShardAssignment{
							"shard1": {},
						},
					},
				},
			},
			expectedError: true,
		},
		{
			name: "SelectsLeastLoaded_NoShards",
			state: &store.NamespaceState{
				ShardAssignments: map[string]store.AssignedState{},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, load, count, err := pickLeastLoadedExecutor(tt.state)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedOwner, owner)
				require.Equal(t, tt.expectedLoad, load)
				require.Equal(t, tt.expectedCount, count)

			}
		})
	}
}

func TestWatchNamespaceState(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := testlogger.New(t)
	mockStorage := store.NewMockStore(ctrl)
	mockServer := NewMockWatchNamespaceStateServer(ctrl)

	cfg := config.ShardDistribution{
		Namespaces: []config.Namespace{
			{Name: "test-ns", Type: config.NamespaceTypeFixed, ShardNum: 2},
		},
	}

	handler := &handlerImpl{
		logger:               logger,
		shardDistributionCfg: cfg,
		storage:              mockStorage,
		startWG:              sync.WaitGroup{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	updatesChan := make(chan map[*store.ShardOwner][]string, 1)
	unsubscribe := func() { close(updatesChan) }

	mockServer.EXPECT().Context().Return(ctx).AnyTimes()
	mockStorage.EXPECT().SubscribeToAssignmentChanges(gomock.Any(), "test-ns").Return(updatesChan, unsubscribe, nil)

	// Expect update send
	mockServer.EXPECT().Send(gomock.Any()).DoAndReturn(func(resp *types.WatchNamespaceStateResponse) error {
		require.Len(t, resp.Executors, 1)
		require.Equal(t, "executor-1", resp.Executors[0].ExecutorID)
		return nil
	})

	// Send update, then cancel
	go func() {
		time.Sleep(10 * time.Millisecond)
		updatesChan <- map[*store.ShardOwner][]string{
			{ExecutorID: "executor-1", Metadata: map[string]string{}}: {"shard-1"},
		}
		cancel()
	}()

	err := handler.WatchNamespaceState(&types.WatchNamespaceStateRequest{Namespace: "test-ns"}, mockServer)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
