package executorclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/sharddistributorexecutor"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/executorclient/syncgeneric"
)

func TestHeartBeartLoop(t *testing.T) {
	// Insure that there are no goroutines leaked
	defer goleak.VerifyNone(t)

	// Create mocks
	ctrl := gomock.NewController(t)

	mockShardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)

	// We expect nothing is assigned to the executor, and we assign two shards to it
	mockShardDistributorClient.EXPECT().Heartbeat(gomock.Any(),
		&types.ExecutorHeartbeatRequest{
			Namespace:          "test-namespace",
			ExecutorID:         "test-executor-id",
			Status:             types.ExecutorStatusACTIVE,
			ShardStatusReports: make(map[string]*types.ShardStatusReport),
		}, gomock.Any()).
		Return(&types.ExecutorHeartbeatResponse{
			ShardAssignments: map[string]*types.ShardAssignment{
				"test-shard-id1": {Status: types.AssignmentStatusREADY},
				"test-shard-id2": {Status: types.AssignmentStatusREADY},
			},
		}, nil)

	// The two shards are assigned to the executor, so we expect them to be created, started and stopped
	mockShardProcessor1 := NewMockShardProcessor(ctrl)
	mockShardProcessor1.EXPECT().Start(gomock.Any())
	mockShardProcessor1.EXPECT().Stop()

	mockShardProcessor2 := NewMockShardProcessor(ctrl)
	mockShardProcessor2.EXPECT().Start(gomock.Any())
	mockShardProcessor2.EXPECT().Stop()

	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
	mockShardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(mockShardProcessor1, nil)
	mockShardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(mockShardProcessor2, nil)

	// We use a mock time source to control the heartbeat loop
	mockTimeSource := clock.NewMockedTimeSource()

	// Create the executor
	executor := &executorImpl[*MockShardProcessor]{
		logger:                 log.NewNoop(),
		metrics:                tally.NoopScope,
		shardDistributorClient: mockShardDistributorClient,
		shardProcessorFactory:  mockShardProcessorFactory,
		namespace:              "test-namespace",
		stopC:                  make(chan struct{}),
		heartBeatInterval:      10 * time.Second,
		managedProcessors:      syncgeneric.Map[string, *managedProcessor[*MockShardProcessor]]{},
		executorID:             "test-executor-id",
		timeSource:             mockTimeSource,
	}

	// Start the executor, and defer stopping it
	executor.Start(context.Background())
	defer executor.Stop()

	// Make sure the heartbeat loop has done an iteration and assigned the shards to the executor
	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(15 * time.Second)
	time.Sleep(10 * time.Millisecond) // Force the heartbeatloop goroutine to run
	mockTimeSource.BlockUntil(1)

	// Assert that the two shards are assigned to the executor
	processor1, err := executor.GetShardProcess("test-shard-id1")
	assert.NoError(t, err)
	assert.Equal(t, mockShardProcessor1, processor1)

	processor2, err := executor.GetShardProcess("test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, mockShardProcessor2, processor2)

	nonOwned, err := executor.GetShardProcess("non-owned-shard-id")
	assert.Error(t, err)
	assert.Nil(t, nonOwned)
}

func TestHeartbeat(t *testing.T) {
	// Setup mocks
	ctrl := gomock.NewController(t)

	// We have two shards assigned to the executor, and we expect a third shard to be assigned to it
	shardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)
	shardDistributorClient.EXPECT().Heartbeat(gomock.Any(),
		&types.ExecutorHeartbeatRequest{
			Namespace:  "test-namespace",
			ExecutorID: "test-executor-id",
			Status:     types.ExecutorStatusACTIVE,
			ShardStatusReports: map[string]*types.ShardStatusReport{
				"test-shard-id1": {Status: types.ShardStatusREADY, ShardLoad: 0.123},
				"test-shard-id2": {Status: types.ShardStatusREADY, ShardLoad: 0.456},
			},
		}, gomock.Any()).Return(&types.ExecutorHeartbeatResponse{
		ShardAssignments: map[string]*types.ShardAssignment{
			"test-shard-id1": {Status: types.AssignmentStatusREADY},
			"test-shard-id2": {Status: types.AssignmentStatusREADY},
			"test-shard-id3": {Status: types.AssignmentStatusREADY},
		},
	}, nil)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock1.EXPECT().GetShardLoad().Return(0.123)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)
	shardProcessorMock2.EXPECT().GetShardLoad().Return(0.456)

	// Create the executor
	executor := &executorImpl[*MockShardProcessor]{
		logger:                 log.NewNoop(),
		shardDistributorClient: shardDistributorClient,
		namespace:              "test-namespace",
		executorID:             "test-executor-id",
		metrics:                tally.NoopScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

	// Do the call to heartbeat
	shardAssignments, err := executor.heartbeat(context.Background())

	// Assert that we now have 3 shards in the assignment
	assert.NoError(t, err)
	assert.Equal(t, 3, len(shardAssignments))
	assert.Equal(t, types.AssignmentStatusREADY, shardAssignments["test-shard-id1"].Status)
	assert.Equal(t, types.AssignmentStatusREADY, shardAssignments["test-shard-id2"].Status)
	assert.Equal(t, types.AssignmentStatusREADY, shardAssignments["test-shard-id3"].Status)
}

func TestHeartBeartLoop_ShardAssignmentChange(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Setup mocks
	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)
	shardProcessorMock3 := NewMockShardProcessor(ctrl)

	shardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
	shardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(shardProcessorMock3, nil)

	// Create the executor currently has shards 1 and 2 assigned to it
	executor := &executorImpl[*MockShardProcessor]{
		logger:                log.NewNoop(),
		shardProcessorFactory: shardProcessorFactory,
		metrics:               tally.NoopScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

	// We expect to get a new assignment with shards 2 and 3 assigned to it
	newAssignment := map[string]*types.ShardAssignment{
		"test-shard-id2": {Status: types.AssignmentStatusREADY},
		"test-shard-id3": {Status: types.AssignmentStatusREADY},
	}

	// With the new assignment, shardProcessorMock1 should be stopped and shardProcessorMock3 should be started
	shardProcessorMock1.EXPECT().Stop()
	shardProcessorMock3.EXPECT().Start(gomock.Any())

	// Update the shard assignment
	executor.updateShardAssignment(context.Background(), newAssignment)
	time.Sleep(10 * time.Millisecond) // Force the updateShardAssignment goroutines to run

	// Assert that we now have the 2 shards in the assignment
	_, err := executor.GetShardProcess("test-shard-id1")
	assert.Error(t, err)

	processor2, err := executor.GetShardProcess("test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock2, processor2)

	processor3, err := executor.GetShardProcess("test-shard-id3")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock3, processor3)
}
