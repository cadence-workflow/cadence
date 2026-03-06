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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/store"
)

func TestAssignEphemeralBatch(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		shardKeys      []string
		setupMocks     func(mockStore *store.MockStore)
		expectedOwners map[string]string // shardKey -> expected owner
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:      "AssignsToLeastLoadedExecutor",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"NON-EXISTING-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
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
				// owner2 has the fewest shards; expect a single batch AssignShards call.
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner2").Return(&store.ShardOwner{
					ExecutorID: "owner2",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwners: map[string]string{"NON-EXISTING-SHARD": "owner2"},
		},
		{
			name:      "SkipDrainingExecutor",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"NON-EXISTING-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusACTIVE},
						// owner2 is DRAINING, should be skipped even though it has fewer shards
						"owner2": {Status: types.ExecutorStatusDRAINING},
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
							// owner2 has fewer shards but is DRAINING, so should be skipped
							AssignedShards: map[string]*types.ShardAssignment{
								"shard4": {Status: types.AssignmentStatusREADY},
							},
						},
					},
				}, nil)
				// owner1 should be selected; single batch write
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner1").Return(&store.ShardOwner{
					ExecutorID: "owner1",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwners: map[string]string{"NON-EXISTING-SHARD": "owner1"},
		},
		{
			name:      "GetStateFailure",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"NON-EXISTING-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(nil, errors.New("get state failure"))
			},
			expectedError:  true,
			expectedErrMsg: "get state failure",
		},
		{
			// When two batches race and the first wins, the second gets
			// ErrVersionConflict from AssignShards. This is surfaced as an
			// internal error; the caller is expected to retry GetShardOwner,
			// which will then find the shard already assigned in the cache.
			name:      "VersionConflict",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"CONCURRENT-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}}}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(store.ErrVersionConflict)
			},
			expectedError:  true,
			expectedErrMsg: "version conflict",
		},
		{
			name:      "NoActiveExecutors",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"NON-EXISTING-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
				// All executors are DRAINING — none are eligible for assignment.
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors: map[string]store.HeartbeatState{
						"owner1": {Status: types.ExecutorStatusDRAINING},
						"owner2": {Status: types.ExecutorStatusDRAINING},
					},
					ShardAssignments: map[string]store.AssignedState{
						"owner1": {AssignedShards: map[string]*types.ShardAssignment{}},
						"owner2": {AssignedShards: map[string]*types.ShardAssignment{}},
					},
				}, nil)
			},
			expectedError:  true,
			expectedErrMsg: "no active executors available for namespace",
		},
		{
			name:      "AssignShardsFailure",
			namespace: _testNamespaceEphemeral,
			shardKeys: []string{"NON-EXISTING-SHARD"},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}}}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(errors.New("assign shards failure"))
			},
			expectedError:  true,
			expectedErrMsg: "assign shards failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStorage := store.NewMockStore(ctrl)

			h := &handlerImpl{
				logger:  testlogger.New(t),
				storage: mockStorage,
			}

			if tt.setupMocks != nil {
				tt.setupMocks(mockStorage)
			}

			results, err := h.assignEphemeralBatch(context.Background(), tt.namespace, tt.shardKeys)
			if tt.expectedError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
				require.Nil(t, results)
			} else {
				require.NoError(t, err)
				require.Len(t, results, len(tt.expectedOwners))
				for shardKey, expectedOwner := range tt.expectedOwners {
					require.Equal(t, expectedOwner, results[shardKey].Owner)
					require.Equal(t, tt.namespace, results[shardKey].Namespace)
					require.Equal(t, map[string]string{"ip": "127.0.0.1", "port": "1234"}, results[shardKey].Metadata)
				}
			}
		})
	}
}
