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
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testNamespaceFixed     = "test-fixed"
	_testNamespaceEphemeral = "test-ephemeral"
)

func TestGetShardOwner(t *testing.T) {
	cfg := config.ShardDistribution{
		Enabled: true,
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "1").Return("", errors.New("lookup error"))
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "123").Return("owner1", nil)
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceFixed, "NON-EXISTING-SHARD").Return("", store.ErrShardNotFound)
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "123").Return("owner1", nil)
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return("", store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return("", store.ErrShardNotFound)
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
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return("", store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}}}, nil)
				mockStore.EXPECT().AssignShard(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD", "owner1").Return(errors.New("assign shard failure"))
			},
			expectedError:  true,
			expectedErrMsg: "assign shard failure",
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
			}
		})
	}
}
