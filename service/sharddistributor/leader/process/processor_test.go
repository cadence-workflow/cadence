package process

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/leader/store"
)

var (
	// Updated test config to use ShardNum instead of Range.
	_testNamespaceCfg = config.Namespace{
		Name:     "test-namespace",
		Type:     config.NamespaceTypeFixed,
		ShardNum: 10, // 10 total shards, from 0-9.
	}
	_testLeaderElectionCfg = config.LeaderElection{
		Process: config.LeaderProcess{
			Period: 10 * time.Second,
		},
	}
)

// TestLifecycle verifies the Run and Terminate methods.
func TestLifecycle(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore)

	// Mock dependencies for the run loop.
	mockStore.EXPECT().Subscribe(gomock.Any()).Return(make(chan int64), nil)
	mockStore.EXPECT().GetState(gomock.Any()).Return(nil, nil, int64(0), nil).MinTimes(1)

	// Act & Assert
	err := processor.Run(context.Background())
	require.NoError(t, err, "First Run should succeed")

	err = processor.Run(context.Background())
	assert.Error(t, err, "Second Run should fail")

	err = processor.Terminate(context.Background())
	require.NoError(t, err, "First Terminate should succeed")

	err = processor.Terminate(context.Background())
	assert.Error(t, err, "Second Terminate should fail")
}

// TestRebalance_InitialAssignment tests distribution of shards to active executors.
func TestRebalance_InitialAssignment(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore)

	executors := []string{"executor-1", "executor-2"}
	heartbeats := map[string]store.HeartbeatState{
		executors[0]: {ExecutorID: executors[0], State: store.ExecutorStateActive},
		executors[1]: {ExecutorID: executors[1], State: store.ExecutorStateActive},
	}

	mockStore.EXPECT().GetState(gomock.Any()).Return(heartbeats, nil, int64(100), nil)

	var capturedState map[string]store.AssignedState
	mockStore.EXPECT().AssignShards(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, newState map[string]store.AssignedState) error {
			capturedState = newState
			return nil
		},
	)

	// Act
	processor.(*namespaceProcessor).rebalanceShards(context.Background())

	// Assert
	require.NotNil(t, capturedState)
	assert.Len(t, capturedState, 2, "Assignments should be created for both executors")

	// 10 shards / 2 executors = 5 shards each
	assert.Len(t, capturedState[executors[0]].AssignedShards, 5)
	assert.Len(t, capturedState[executors[1]].AssignedShards, 5)

	totalAssigned := make(map[string]struct{})
	for _, state := range capturedState {
		for shardID := range state.AssignedShards {
			totalAssigned[shardID] = struct{}{}
		}
	}
	assert.Len(t, totalAssigned, 10, "All 10 shards should be assigned")
}

// TestRebalance_ExecutorLeaves tests that shards from a non-active executor are reassigned.
func TestRebalance_ExecutorLeaves(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore)

	heartbeats := map[string]store.HeartbeatState{
		"executor-1": {State: store.ExecutorStateActive},
		"executor-2": {State: store.ExecutorStateActive},
		"executor-3": {State: store.ExecutorStateDraining},
	}

	// Shard IDs are now 0-9.
	assignments := map[string]store.AssignedState{
		"executor-1": {AssignedShards: map[string]store.ShardAssignment{"1": {}, "2": {}, "3": {}}},
		"executor-2": {AssignedShards: map[string]store.ShardAssignment{"4": {}, "5": {}, "6": {}}},
		"executor-3": {AssignedShards: map[string]store.ShardAssignment{"7": {}, "8": {}, "9": {}, "10": {}}},
	}

	mockStore.EXPECT().GetState(gomock.Any()).Return(heartbeats, assignments, int64(100), nil)

	var capturedState map[string]store.AssignedState
	mockStore.EXPECT().AssignShards(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, newState map[string]store.AssignedState) error {
			capturedState = newState
			return nil
		},
	)

	// Act
	processor.(*namespaceProcessor).rebalanceShards(context.Background())

	// Assert
	require.NotNil(t, capturedState)
	assert.Len(t, capturedState, 2)
	assert.NotContains(t, capturedState, "executor-3")

	// 4 shards from executor-3 are redistributed (2 each). Initial 3 + 2 = 5.
	assert.Len(t, capturedState["executor-1"].AssignedShards, 5)
	assert.Len(t, capturedState["executor-2"].AssignedShards, 5)
}

// TestRebalance_StaleRevision ensures no action is taken for an old revision.
func TestRebalance_StaleRevision(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore).(*namespaceProcessor)

	// Set the processor's last applied revision to 100.
	processor.lastAppliedRevision = 100

	// Mock GetState to return a revision that is NOT newer.
	mockStore.EXPECT().GetState(gomock.Any()).Return(nil, nil, int64(100), nil)
	// AssignShards should NOT be called.
	mockStore.EXPECT().AssignShards(gomock.Any(), gomock.Any()).Times(0)

	// Act
	processor.rebalanceShards(context.Background())
}

// TestRebalance_AssignShardsFailure ensures revision is not updated on failure.
func TestRebalance_AssignShardsFailure(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore).(*namespaceProcessor)

	// Start with no revision applied.
	processor.lastAppliedRevision = 0

	// Mock GetState to return a new revision and one active executor.
	heartbeats := map[string]store.HeartbeatState{"executor-1": {State: store.ExecutorStateActive}}
	mockStore.EXPECT().GetState(gomock.Any()).Return(heartbeats, nil, int64(101), nil)
	// Mock AssignShards to return an error.
	mockStore.EXPECT().AssignShards(gomock.Any(), gomock.Any()).Return(errors.New("etcd commit failed"))

	// Act
	processor.rebalanceShards(context.Background())

	// Assert
	// The key assertion is that lastAppliedRevision was NOT updated because the assignment failed.
	// This allows the processor to retry on the next trigger.
	assert.Equal(t, int64(0), processor.lastAppliedRevision)
}

// TestRebalance_NoActiveExecutors ensures it handles the edge case gracefully.
func TestRebalance_NoActiveExecutors(t *testing.T) {
	// Arrange
	ctrl := gomock.NewController(t)
	mockStore := store.NewMockShardStore(ctrl)
	factory := NewProcessorFactory(testlogger.New(t), clock.NewRealTimeSource(), _testLeaderElectionCfg)
	processor := factory.CreateProcessor(_testNamespaceCfg, mockStore).(*namespaceProcessor)

	// Mock GetState to return executors that are not active.
	heartbeats := map[string]store.HeartbeatState{"executor-1": {State: store.ExecutorStateDraining}}
	mockStore.EXPECT().GetState(gomock.Any()).Return(heartbeats, nil, int64(102), nil)
	// AssignShards should NOT be called.
	mockStore.EXPECT().AssignShards(gomock.Any(), gomock.Any()).Times(0)

	// Act
	processor.rebalanceShards(context.Background())

	// Assert
	// No panic should occur, and the lastAppliedRevision should not be updated because no work was done.
	assert.Equal(t, int64(0), processor.lastAppliedRevision)
}
