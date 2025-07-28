package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
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
		handler := NewExecutorHandler(mockStore, mockTimeSource)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(nil, nil, store.ErrExecutorNotFound)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat:  now.Unix(),
			State:          store.ExecutorStateActive,
			ReportedShards: make(map[string]store.ShardInfo),
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 2: Subsequent Heartbeat within the refresh rate (no update)
	t.Run("SubsequentHeartbeatWithinRate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		handler := NewExecutorHandler(mockStore, mockTimeSource)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			State:         store.ExecutorStateActive,
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
		handler := NewExecutorHandler(mockStore, mockTimeSource)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusACTIVE,
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			State:         store.ExecutorStateActive,
		}

		// Advance time
		mockTimeSource.Advance(_heartbeatRefreshRate + time.Second)

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat:  mockTimeSource.Now().Unix(),
			State:          store.ExecutorStateActive,
			ReportedShards: make(map[string]store.ShardInfo),
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 4: Status Change (with update)
	t.Run("StatusChange", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSourceAt(now)
		handler := NewExecutorHandler(mockStore, mockTimeSource)

		req := &types.ExecutorHeartbeatRequest{
			Namespace:  namespace,
			ExecutorID: executorID,
			Status:     types.ExecutorStatusDRAINING, // Status changed
		}

		previousHeartbeat := store.HeartbeatState{
			LastHeartbeat: now.Unix(),
			State:         store.ExecutorStateActive,
		}

		mockStore.EXPECT().GetHeartbeat(gomock.Any(), namespace, executorID).Return(&previousHeartbeat, nil, nil)
		mockStore.EXPECT().RecordHeartbeat(gomock.Any(), namespace, executorID, store.HeartbeatState{
			LastHeartbeat:  now.Unix(),
			State:          store.ExecutorStateDraining,
			ReportedShards: make(map[string]store.ShardInfo),
		})

		_, err := handler.Heartbeat(ctx, req)
		require.NoError(t, err)
	})

	// Test Case 5: Storage Error
	t.Run("StorageError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockStore := store.NewMockStore(ctrl)
		mockTimeSource := clock.NewMockedTimeSource()
		handler := NewExecutorHandler(mockStore, mockTimeSource)

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
				AssignedShards: make(map[string]store.ShardAssignment),
			},
			expectedResp: &types.ExecutorHeartbeatResponse{
				ShardAssignments: make(map[string]*types.ShardAssignment),
			},
		},
		{
			name: "Populated input",
			input: &store.AssignedState{
				AssignedShards: map[string]store.ShardAssignment{
					"shard-1": {Status: store.ShardStateReady},
					"shard-2": {Status: store.ShardStateReady},
				},
			},
			expectedResp: &types.ExecutorHeartbeatResponse{
				ShardAssignments: map[string]*types.ShardAssignment{
					"shard-1": {Status: types.AssignmentStatusREADY},
					"shard-2": {Status: types.AssignmentStatusREADY},
				},
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
			res := _convertResponse(tc.input)

			// Ensure ShardAssignments is not nil for comparison purposes
			if res.ShardAssignments == nil {
				res.ShardAssignments = make(map[string]*types.ShardAssignment)
			}
			require.Equal(t, tc.expectedResp, res)
		})
	}
}

func TestConvertReportedShards(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]*types.ShardStatusReport
		expected map[string]store.ShardInfo
	}{
		{
			name:     "Nil input",
			input:    nil,
			expected: make(map[string]store.ShardInfo),
		},
		{
			name:     "Empty input",
			input:    make(map[string]*types.ShardStatusReport),
			expected: make(map[string]store.ShardInfo),
		},
		{
			name: "Populated input",
			input: map[string]*types.ShardStatusReport{
				"shard-1": {Status: types.ShardStatusREADY, ShardLoad: 1.23},
				"shard-2": {Status: types.ShardStatusREADY, ShardLoad: 4.56},
			},
			expected: map[string]store.ShardInfo{
				"shard-1": {Status: store.ShardStateReady, ShardLoad: 1.23},
				"shard-2": {Status: store.ShardStateReady, ShardLoad: 4.56},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := _convertReportedShards(tc.input)
			require.Equal(t, tc.expected, res)
		})
	}
}
