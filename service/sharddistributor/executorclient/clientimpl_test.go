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
			MigrationMode: types.MigrationModeONBOARDED,
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
		MigrationMode: types.MigrationModeONBOARDED,
	}, nil)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock1.EXPECT().GetShardReport().Return(ShardReport{ShardLoad: 0.123, Status: types.ShardStatusREADY})
	shardProcessorMock2 := NewMockShardProcessor(ctrl)
	shardProcessorMock2.EXPECT().GetShardReport().Return(ShardReport{ShardLoad: 0.456, Status: types.ShardStatusREADY})

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
	shardAssignments, _, err := executor.heartbeat(context.Background())

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

func TestAssignShards(t *testing.T) {
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
	executor.AssignShardsFromLocalLogic(context.Background(), newAssignment)
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

func TestHeartbeat_WithMigrationMode(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Test that heartbeat returns migration mode correctly
	shardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)
	shardDistributorClient.EXPECT().Heartbeat(gomock.Any(),
		&types.ExecutorHeartbeatRequest{
			Namespace:          "test-namespace",
			ExecutorID:         "test-executor-id",
			Status:             types.ExecutorStatusACTIVE,
			ShardStatusReports: map[string]*types.ShardStatusReport{},
		}, gomock.Any()).Return(&types.ExecutorHeartbeatResponse{
		ShardAssignments: map[string]*types.ShardAssignment{
			"test-shard-id1": {Status: types.AssignmentStatusREADY},
		},
		MigrationMode: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
	}, nil)

	executor := &executorImpl[*MockShardProcessor]{
		logger:                 log.NewNoop(),
		shardDistributorClient: shardDistributorClient,
		namespace:              "test-namespace",
		executorID:             "test-executor-id",
		metrics:                tally.NoopScope,
		migrationMode:          types.MigrationModeINVALID,
	}

	shardAssignments, migrationMode, err := executor.heartbeat(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, len(shardAssignments))
	assert.Equal(t, types.MigrationModeDISTRIBUTEDPASSTHROUGH, migrationMode)
	assert.Equal(t, types.MigrationModeDISTRIBUTEDPASSTHROUGH, executor.migrationMode)
}

func TestHeartbeat_MigrationModeTransition(t *testing.T) {
	ctrl := gomock.NewController(t)

	shardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)
	shardDistributorClient.EXPECT().Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).Return(&types.ExecutorHeartbeatResponse{
		ShardAssignments: map[string]*types.ShardAssignment{},
		MigrationMode:    types.MigrationModeONBOARDED,
	}, nil)

	executor := &executorImpl[*MockShardProcessor]{
		logger:                 log.NewNoop(),
		shardDistributorClient: shardDistributorClient,
		namespace:              "test-namespace",
		executorID:             "test-executor-id",
		metrics:                tally.NoopScope,
		migrationMode:          types.MigrationModeDISTRIBUTEDPASSTHROUGH,
	}

	_, migrationMode, err := executor.heartbeat(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, types.MigrationModeONBOARDED, migrationMode)
	assert.Equal(t, types.MigrationModeONBOARDED, executor.migrationMode)
}

func TestHeartbeatLoop_LocalPassthrough_SkipsHeartbeat(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockShardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)

	// No heartbeat should be called for LOCAL_PASSTHROUGH mode
	mockShardDistributorClient.EXPECT().Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	mockTimeSource := clock.NewMockedTimeSource()

	executor := &executorImpl[*MockShardProcessor]{
		logger:                 log.NewNoop(),
		metrics:                tally.NoopScope,
		shardDistributorClient: mockShardDistributorClient,
		namespace:              "test-namespace",
		stopC:                  make(chan struct{}),
		heartBeatInterval:      10 * time.Second,
		managedProcessors:      syncgeneric.Map[string, *managedProcessor[*MockShardProcessor]]{},
		executorID:             "test-executor-id",
		timeSource:             mockTimeSource,
		migrationMode:          types.MigrationModeLOCALPASSTHROUGH,
	}

	executor.Start(context.Background())
	defer executor.Stop()

	// Give some time for the heartbeat loop to potentially run (it shouldn't)
	time.Sleep(10 * time.Millisecond)
}

func TestHeartbeatLoop_LocalPassthroughShadow_SkipsAssignment(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockShardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)

	// Heartbeat should be called but assignment should not be applied
	mockShardDistributorClient.EXPECT().Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&types.ExecutorHeartbeatResponse{
			ShardAssignments: map[string]*types.ShardAssignment{
				"test-shard-id1": {Status: types.AssignmentStatusREADY},
			},
			MigrationMode: types.MigrationModeLOCALPASSTHROUGHSHADOW,
		}, nil)

	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
	// No shard processor should be created
	mockShardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Times(0)

	mockTimeSource := clock.NewMockedTimeSource()

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
		migrationMode:          types.MigrationModeONBOARDED,
	}

	executor.Start(context.Background())
	defer executor.Stop()

	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(15 * time.Second)
	time.Sleep(10 * time.Millisecond)
	mockTimeSource.BlockUntil(1)

	// Assert that no shards were assigned
	_, err := executor.GetShardProcess("test-shard-id1")
	assert.Error(t, err)
}

func TestHeartbeatLoop_DistributedPassthrough_AppliesAssignment(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	mockShardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)

	mockShardDistributorClient.EXPECT().Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&types.ExecutorHeartbeatResponse{
			ShardAssignments: map[string]*types.ShardAssignment{
				"test-shard-id1": {Status: types.AssignmentStatusREADY},
			},
			MigrationMode: types.MigrationModeDISTRIBUTEDPASSTHROUGH,
		}, nil)

	mockShardProcessor := NewMockShardProcessor(ctrl)
	mockShardProcessor.EXPECT().Start(gomock.Any())
	mockShardProcessor.EXPECT().Stop()

	mockShardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
	mockShardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(mockShardProcessor, nil)

	mockTimeSource := clock.NewMockedTimeSource()

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
		migrationMode:          types.MigrationModeONBOARDED,
	}

	executor.Start(context.Background())
	defer executor.Stop()

	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(15 * time.Second)
	time.Sleep(10 * time.Millisecond)
	mockTimeSource.BlockUntil(1)

	// Assert that the shard was assigned
	processor, err := executor.GetShardProcess("test-shard-id1")
	assert.NoError(t, err)
	assert.Equal(t, mockShardProcessor, processor)
}

func TestCompareAssignments_Converged(t *testing.T) {
	ctrl := gomock.NewController(t)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)

	testScope := tally.NewTestScope("test", nil)
	executor := &executorImpl[*MockShardProcessor]{
		logger:  log.NewNoop(),
		metrics: testScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

	heartbeatAssignments := map[string]*types.ShardAssignment{
		"test-shard-id1": {Status: types.AssignmentStatusREADY},
		"test-shard-id2": {Status: types.AssignmentStatusREADY},
	}

	executor.compareAssignments(heartbeatAssignments)

	// Verify convergence metric was emitted
	snapshot := testScope.Snapshot()
	assert.Equal(t, int64(1), snapshot.Counters()["test.shard_distributor_executor_assignment_convergence+"].Value())
}

func TestCompareAssignments_Diverged_MissingShard(t *testing.T) {
	ctrl := gomock.NewController(t)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)

	testScope := tally.NewTestScope("test", nil)
	executor := &executorImpl[*MockShardProcessor]{
		logger:  log.NewNoop(),
		metrics: testScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

	// Heartbeat only has shard 1, local has both 1 and 2
	heartbeatAssignments := map[string]*types.ShardAssignment{
		"test-shard-id1": {Status: types.AssignmentStatusREADY},
	}

	executor.compareAssignments(heartbeatAssignments)

	// Verify divergence metric was emitted
	snapshot := testScope.Snapshot()
	assert.Equal(t, int64(1), snapshot.Counters()["test.shard_distributor_executor_assignment_divergence+"].Value())
}

func TestCompareAssignments_Diverged_ExtraShard(t *testing.T) {
	ctrl := gomock.NewController(t)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)

	testScope := tally.NewTestScope("test", nil)
	executor := &executorImpl[*MockShardProcessor]{
		logger:  log.NewNoop(),
		metrics: testScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))

	// Heartbeat has shards 1 and 2, local only has 1
	heartbeatAssignments := map[string]*types.ShardAssignment{
		"test-shard-id1": {Status: types.AssignmentStatusREADY},
		"test-shard-id2": {Status: types.AssignmentStatusREADY},
	}

	executor.compareAssignments(heartbeatAssignments)

	// Verify divergence metric was emitted
	snapshot := testScope.Snapshot()
	assert.Equal(t, int64(1), snapshot.Counters()["test.shard_distributor_executor_assignment_divergence+"].Value())
}

func TestCompareAssignments_Diverged_WrongStatus(t *testing.T) {
	ctrl := gomock.NewController(t)

	shardProcessorMock1 := NewMockShardProcessor(ctrl)

	testScope := tally.NewTestScope("test", nil)
	executor := &executorImpl[*MockShardProcessor]{
		logger:  log.NewNoop(),
		metrics: testScope,
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))

	// Heartbeat has shard 1 but with INVALID status
	heartbeatAssignments := map[string]*types.ShardAssignment{
		"test-shard-id1": {Status: types.AssignmentStatusINVALID},
	}

	executor.compareAssignments(heartbeatAssignments)

	// Verify divergence metric was emitted
	snapshot := testScope.Snapshot()
	assert.Equal(t, int64(1), snapshot.Counters()["test.shard_distributor_executor_assignment_divergence+"].Value())
}
