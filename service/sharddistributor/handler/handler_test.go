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
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	_testNamespaceFixed     = "test-fixed"
	_testNamespaceEphemeral = "test-ephemeral"
)

// newTestHandler creates a handlerImpl wired with a real shardBatcher backed by
// the provided store mock. The batcher is started and the handler is returned
// ready to use; callers should call Stop() when done.
func newTestHandler(t *testing.T, cfg config.ShardDistribution, mockStore *store.MockStore) *handlerImpl {
	t.Helper()
	handler := &handlerImpl{
		logger:               testlogger.New(t),
		shardDistributionCfg: cfg,
		cfg:                  newTestShardDistributorConfig(config.LoadBalancingModeNAIVE),
		storage:              mockStore,
	}
	handler.batcher = newShardBatcher(clock.NewRealTimeSource(), 10*time.Millisecond, handler.assignEphemeralBatch)
	handler.batcher.Start()
	t.Cleanup(handler.batcher.Stop)
	return handler
}

func newTestShardDistributorConfig(mode string) *config.Config {
	return &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return mode
		},
	}
}

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
			// ShardNotFound for an ephemeral namespace routes to the batcher, which
			// calls assignEphemeralBatch. This case validates the routing only;
			// detailed assignment behaviour is covered in TestAssignEphemeralBatch.
			name: "ShardNotFound_Ephemeral_RoutesToBatcher",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").Return(nil, store.ErrShardNotFound)
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}},
				}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner1").Return(&store.ShardOwner{
					ExecutorID: "owner1",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},
		{
			// A version conflict from AssignShards causes the batcher to return
			// ErrVersionConflict. getOrAssignEphemeralShard re-reads storage on
			// retry; here the concurrent winner has already written the assignment
			// so the second GetShardOwner call succeeds and no second batcher
			// submission is required.
			name: "Ephemeral_VersionConflict_ResolvedByStorageRead",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				// Initial lookup — shard absent.
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").
					Return(nil, store.ErrShardNotFound)

				// Batcher fires: GetState + AssignShards returns a version conflict.
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}},
				}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).
					Return(store.ErrVersionConflict)

				// Retry: re-read finds the shard already assigned by the concurrent winner.
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").
					Return(&store.ShardOwner{
						ExecutorID: "owner1",
						Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
					}, nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},
		{
			// A version conflict from AssignShards is retried; on the retry
			// GetShardOwner still returns ErrShardNotFound, so the batcher is
			// re-submitted and this time AssignShards succeeds.
			name: "Ephemeral_VersionConflict_RetriedAndSucceeds",
			request: &types.GetShardOwnerRequest{
				Namespace: _testNamespaceEphemeral,
				ShardKey:  "NON-EXISTING-SHARD",
			},
			setupMocks: func(mockStore *store.MockStore) {
				// Initial lookup — shard absent.
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").
					Return(nil, store.ErrShardNotFound)

				// First batcher attempt: version conflict.
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}},
				}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("assign ephemeral shards: %w", store.ErrVersionConflict))

				// Retry re-read — shard still absent, so batcher is submitted again.
				mockStore.EXPECT().GetShardOwner(gomock.Any(), _testNamespaceEphemeral, "NON-EXISTING-SHARD").
					Return(nil, store.ErrShardNotFound)

				// Second batcher attempt: succeeds.
				mockStore.EXPECT().GetState(gomock.Any(), _testNamespaceEphemeral).Return(&store.NamespaceState{
					Executors:        map[string]store.HeartbeatState{"owner1": {Status: types.ExecutorStatusACTIVE}},
					ShardAssignments: map[string]store.AssignedState{"owner1": {AssignedShards: map[string]*types.ShardAssignment{}}},
				}, nil)
				mockStore.EXPECT().AssignShards(gomock.Any(), _testNamespaceEphemeral, gomock.Any(), gomock.Any()).Return(nil)
				mockStore.EXPECT().GetExecutor(gomock.Any(), _testNamespaceEphemeral, "owner1").Return(&store.ShardOwner{
					ExecutorID: "owner1",
					Metadata:   map[string]string{"ip": "127.0.0.1", "port": "1234"},
				}, nil)
			},
			expectedOwner: "owner1",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockStorage := store.NewMockStore(ctrl)

			handler := newTestHandler(t, cfg, mockStorage)

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
