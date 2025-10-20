package handler

import (
	"context"
	"errors"
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

func TestHeartbeat(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"
	executorID := "test-executor"
	now := time.Now().UTC()

	// Test Case 1: First Heartbeat
	t.Run("FirstHeartbeat", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		shardDistributionCfg := config.ShardDistribution{}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(nil, nil, store.ErrExecutorNotFound)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 2: Subsequent Heartbeat within the refresh rate (no update)
	t.Run("SubsequentHeartbeatWithinRate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		shardDistributionCfg := config.ShardDistribution{}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 3: Subsequent Heartbeat after refresh rate (with update)
	t.Run("SubsequentHeartbeatAfterRate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		shardDistributionCfg := config.ShardDistribution{}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		}

		// Advance time
		mockTimeSource.Advance(_heartbeatRefreshRate + time.Second)

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat: mockTimeSource.Now().Unix(),
			Status:        types.ExecutorStatusACTIVE,
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 4: Status Change (with update)
	t.Run("StatusChange", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		shardDistributionCfg := config.ShardDistribution{}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusDRAINING, // Status changed
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusDRAINING,
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 5: Storage Error
	t.Run("StorageError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		expectedErr := errors.New("storage is down")
		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(nil, nil, expectedErr)

		_, err := handler.Heartbeat(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErr.Error())
	})

	// Test Case 6: Heartbeat with executor associated invalid migration mode
	t.Run("MigrationModeInvald", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{
			Namespaces: []config.Namespace{{Name: namespace, Mode: config.MigrationModeINVALID}},
		}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}
		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		}

		expectedErr := errors.New("migration mode is invalid")
		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)

		_, err := handler.Heartbeat(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErr.Error())
	})

	// Test Case 7: Heartbeat with executor associated with local passthrough mode
	t.Run("MigrationModeLocalPassthrough", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{
			Namespaces: []config.Namespace{{Name: namespace, Mode: config.MigrationModeLOCALPASSTHROUGH}},
		}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}
		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
		}

		expectedErr := errors.New("migration mode is local passthrough")
		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)

		_, err := handler.Heartbeat(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErr.Error())
	})

	// Test Case 8: Heartbeat with executor associated with local passthrough shadow
	t.Run("MigrationModeLocalPassthroughWithAssignmentChanges", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{
			Namespaces: []config.Namespace{{Name: namespace, Mode: config.MigrationModeLOCALPASSTHROUGHSHADOW}},
		}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
			ShardStatusReports: map[string]*types.ShardStatusReport{
				"shard0": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
			ReportedShards: map[string]*types.ShardStatusReport{
				"shard1": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		assignedState := store.AssignedState{
			AssignedShards: map[string]*types.ShardAssignment{
				"shard1": {Status: types.AssignmentStatusREADY},
			},
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, &assignedState, nil)
		mockStore.EXPECT().DeleteExecutors(gomock.Any(), namespace, []string{executorID}, gomock.Any()).Return(nil)
		mockStore.EXPECT().AssignShards(gomock.Any(), namespace, gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, namespace string, request store.AssignShardsRequest, guard store.GuardFunc) error {
				// Expect to Assign the shard in the request
				expectedRequest := store.AssignShardsRequest{
					NewState: &store.NamespaceState{
						ShardAssignments: map[string]store.AssignedState{
							executorID: {AssignedShards: map[string]*types.ShardAssignment{"shard0": {Status: types.AssignmentStatusREADY}}},
						},
					},
				}
				require.Equal(t, expectedRequest.NewState.ShardAssignments[executorID].AssignedShards, request.NewState.ShardAssignments[executorID].AssignedShards)
				return nil
			},
		)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
			ReportedShards: map[string]*types.ShardStatusReport{
				"shard0": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	},
	)

	// Test Case 9: Heartbeat with executor associated with distributed passthrough
	t.Run("MigrationModeDISTRIBUTEDPASSTHROUGHDeletionFailure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{
			Namespaces: []config.Namespace{{Name: namespace, Mode: config.MigrationModeLOCALPASSTHROUGHSHADOW}},
		}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
			ShardStatusReports: map[string]*types.ShardStatusReport{
				"shard0": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
			ReportedShards: map[string]*types.ShardStatusReport{
				"shard1": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		assignedState := store.AssignedState{
			AssignedShards: map[string]*types.ShardAssignment{
				"shard1": {Status: types.AssignmentStatusREADY},
			},
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, &assignedState, nil)
		expectedErr := errors.New("deletion failed")
		mockStore.EXPECT().DeleteExecutors(gomock.Any(), namespace, []string{executorID}, gomock.Any()).Return(expectedErr)

		_, err := handler.Heartbeat(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErr.Error())
	})

	// Test Case 10: Heartbeat with executor associated with distributed passthrough
	t.Run("MigrationModeDISTRIBUTEDPASSTHROUGHAssignmentFailure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		shardDistributionCfg := config.ShardDistribution{
			Namespaces: []config.Namespace{{Name: namespace, Mode: config.MigrationModeLOCALPASSTHROUGHSHADOW}},
		}
		handler := NewExecutorHandler(testlogger.New(t), mockStore, mockTimeSource, shardDistributionCfg)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
			ShardStatusReports: map[string]*types.ShardStatusReport{
				"shard0": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			Status:        types.ExecutorStatusACTIVE,
			ReportedShards: map[string]*types.ShardStatusReport{
				"shard1": {Status: types.ShardStatusREADY, ShardLoad: 1.0},
			},
		}

		assignedState := store.AssignedState{
			AssignedShards: map[string]*types.ShardAssignment{
				"shard1": {Status: types.AssignmentStatusREADY},
			},
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, &assignedState, nil)
		mockStore.EXPECT().DeleteExecutors(gomock.Any(), namespace, []string{executorID}, gomock.Any()).Return(nil)
		expectedErr := errors.New("assignemnt failed")
		mockStore.EXPECT().AssignShards(gomock.Any(), namespace, gomock.Any(), gomock.Any()).Return(expectedErr)

		_, err := handler.Heartbeat(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), expectedErr.Error())
	})
}

func TestConvertResponse(t *testing.T) {
	testCases := []struct {
		name         string
		input        *store.AssignedState
		expectedResp *types.ExecutorHeartbeatResponse
	}{
		{
			name:  "Nil input",
			input: nil,
			expectedResp: &types.ExecutorHeartbeatResponse{
				ShardAssignments: make(map[string]*types.ShardAssignment),
			},
		},
		{
			name: "Empty input",
			input: &store.AssignedState{
				AssignedShards: make(map[string]*types.ShardAssignment),
			},
			expectedResp: &types.ExecutorHeartbeatResponse{
				ShardAssignments: make(map[string]*types.ShardAssignment),
				MigrationMode:    types.MigrationModeONBOARDED,
			},
		},
		{
			name: "Populated input",
			input: &store.AssignedState{
				AssignedShards: map[string]*types.ShardAssignment{
					"shard-1": {Status: types.AssignmentStatusREADY},
					"shard-2": {Status: types.AssignmentStatusREADY},
				},
			},
			expectedResp: &types.ExecutorHeartbeatResponse{
				ShardAssignments: map[string]*types.ShardAssignment{
					"shard-1": {Status: types.AssignmentStatusREADY},
					"shard-2": {Status: types.AssignmentStatusREADY},
				},
				MigrationMode: types.MigrationModeONBOARDED,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// In Go, you can't initialize a map in a struct to nil directly,
			// so we handle the nil case for ShardAssignments separately for comparison.
			if tc.expectedResp.ShardAssignments == nil {
				tc.expectedResp.ShardAssignments = make(map[string]*types.ShardAssignment)
			}
			res := _convertResponse(tc.input, types.MigrationModeONBOARDED)

			// Ensure ShardAssignments is not nil for comparison purposes
			if res.ShardAssignments == nil {
				res.ShardAssignments = make(map[string]*types.ShardAssignment)
			}
			require.Equal(t, tc.expectedResp, res)
		})
	}
}
