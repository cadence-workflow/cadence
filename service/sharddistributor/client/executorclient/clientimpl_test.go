package executorclient

import (
	"context"
	"fmt"
	"sync"
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
	"github.com/uber/cadence/service/sharddistributor/client/executorclient/syncgeneric"
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
			Metadata:           make(map[string]string),
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
	processor1, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
	assert.NoError(t, err)
	assert.Equal(t, mockShardProcessor1, processor1)

	processor2, err := executor.GetShardProcess(context.Background(), "test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, mockShardProcessor2, processor2)

	// Check that non-owned-shard-id is not in the local cache, we check directly since we don't want to trigger a heartbeat
	_, ok := executor.managedProcessors.Load("non-owned-shard-id")
	assert.False(t, ok)
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
			Metadata: make(map[string]string),
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
		timeSource:            clock.NewMockedTimeSource(),
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
	processor2, err := executor.GetShardProcess(context.Background(), "test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock2, processor2)

	processor3, err := executor.GetShardProcess(context.Background(), "test-shard-id3")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock3, processor3)

	// Check that we do not have shard "test-shard-id1" in the local cache
	// we lookup directly since we don't want to trigger a heartbeat
	_, ok := executor.managedProcessors.Load("test-shard-id1")
	assert.False(t, ok)
}

func TestCleanUpShardsLoop_Stored(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Setup mocks
	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)
	shardProcessorMock3 := NewMockShardProcessor(ctrl)

	shardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)

	// Create the executor currently has shards 1 and 2 assigned to it
	executor := &executorImpl[*MockShardProcessor]{
		logger:                log.NewNoop(),
		shardProcessorFactory: shardProcessorFactory,
		metrics:               tally.NoopScope,
		ttlShard:              time.Hour,
		timeSource:            clock.NewMockedTimeSource(),
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

	// Assert that we now have the 2 shards in the assignment
	processor2, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock2, processor2)

	processor3, err := executor.GetShardProcess(context.Background(), "test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock3, processor3)

	_, ok := executor.processorsToLastUse.Load("test-shard-id1")
	assert.True(t, ok)
	_, ok = executor.processorsToLastUse.Load("test-shard-id2")
	assert.True(t, ok)
}

func TestCleanUpShardsLoop_UpdateTime(t *testing.T) {
	ctrl := gomock.NewController(t)

	// Setup mocks
	shardProcessorMock1 := NewMockShardProcessor(ctrl)
	shardProcessorMock2 := NewMockShardProcessor(ctrl)
	shardProcessorMock1.EXPECT().Stop()
	shardProcessorMock2.EXPECT().Stop()

	shardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)

	// Create the executor currently has shards 1 and 2 assigned to it
	executor := &executorImpl[*MockShardProcessor]{
		logger:                log.NewNoop(),
		shardProcessorFactory: shardProcessorFactory,
		metrics:               tally.NoopScope,
		ttlShard:              1 * time.Millisecond,
		timeSource:            clock.NewRealTimeSource(),
	}

	executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
	executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go executor.shardCleanUpLoop(ctx)

	// Assert that we now have the 2 shards in the assignment
	processor1, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock1, processor1)

	processor2, err := executor.GetShardProcess(context.Background(), "test-shard-id2")
	assert.NoError(t, err)
	assert.Equal(t, shardProcessorMock2, processor2)

	// wait for the execution of the loop
	time.Sleep(5 * time.Millisecond)

	_, ok := executor.processorsToLastUse.Load("test-shard-id1")
	assert.False(t, ok)
	_, ok = executor.processorsToLastUse.Load("test-shard-id2")
	assert.False(t, ok)
}

func TestAssignShardsFromLocalLogic(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := []struct {
		name   string
		params map[string]*types.ShardAssignment
		setup  func() *executorImpl[*MockShardProcessor]
		assert func(err error, executor *executorImpl[*MockShardProcessor])
	}{
		{
			name:   "AssignShardsFromLocalLogic fails if the namespace is onboarded",
			params: map[string]*types.ShardAssignment{},
			setup: func() *executorImpl[*MockShardProcessor] {
				// Create the executor currently has shards 1 and 2 assigned to it
				executor := &executorImpl[*MockShardProcessor]{
					logger:     log.NewNoop(),
					metrics:    tally.NoopScope,
					timeSource: clock.NewMockedTimeSource(),
				}
				executor.setMigrationMode(types.MigrationModeONBOARDED)
				return executor
			},
			assert: func(err error, executor *executorImpl[*MockShardProcessor]) {},
		},
		{
			name: "AssignShardsFromLocalLogic succeed with only logs if it is not possible to create a new shard processor",
			params: map[string]*types.ShardAssignment{
				"test-shard-id2": {Status: types.AssignmentStatusREADY},
				"test-shard-id3": {Status: types.AssignmentStatusREADY}},
			setup: func() *executorImpl[*MockShardProcessor] {
				shardProcessorMock1 := NewMockShardProcessor(ctrl)
				shardProcessorMock2 := NewMockShardProcessor(ctrl)

				shardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)

				// Setup mocks
				shardProcessorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(nil, assert.AnError)

				// Create the executor currently has shards 1 and 2 assigned to it
				executor := &executorImpl[*MockShardProcessor]{
					logger:                log.NewNoop(),
					shardProcessorFactory: shardProcessorFactory,
					metrics:               tally.NoopScope,
					timeSource:            clock.NewMockedTimeSource(),
				}
				executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
				executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

				return executor
			},
			assert: func(err error, executor *executorImpl[*MockShardProcessor]) {
				assert.NoError(t, err)
			},
		},
		{
			name: "AssignShardsFromLocalLogic succeed ",
			params: map[string]*types.ShardAssignment{
				"test-shard-id2": {Status: types.AssignmentStatusREADY},
				"test-shard-id3": {Status: types.AssignmentStatusREADY}},
			setup: func() *executorImpl[*MockShardProcessor] {
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
					timeSource:            clock.NewMockedTimeSource(),
				}

				executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
				executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

				// With the new assignment, shardProcessorMock3 should be started
				shardProcessorMock3.EXPECT().Start(gomock.Any())
				shardProcessorMock1.EXPECT().GetShardReport().Return(ShardReport{Status: types.ShardStatusREADY})
				shardProcessorMock2.EXPECT().GetShardReport().Return(ShardReport{Status: types.ShardStatusREADY})
				shardProcessorMock3.EXPECT().GetShardReport().Return(ShardReport{Status: types.ShardStatusREADY})
				return executor
			},
			assert: func(err error, executor *executorImpl[*MockShardProcessor]) {
				// Assert that we now have the 3 shards in the assignment
				processor1, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
				assert.NoError(t, err)
				assert.Equal(t, types.ShardStatusREADY, processor1.GetShardReport().Status)

				processor2, err := executor.GetShardProcess(context.Background(), "test-shard-id2")
				assert.NoError(t, err)
				assert.Equal(t, types.ShardStatusREADY, processor2.GetShardReport().Status)

				processor3, err := executor.GetShardProcess(context.Background(), "test-shard-id3")
				assert.NoError(t, err)
				assert.Equal(t, types.ShardStatusREADY, processor3.GetShardReport().Status)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := test.setup()
			err := executor.AssignShardsFromLocalLogic(context.Background(), test.params)
			time.Sleep(10 * time.Millisecond) // Force the updateShardAssignment goroutines to run
			test.assert(err, executor)
		})
	}
}

func TestRemoveShardsFromLocalLogic(t *testing.T) {
	ctrl := gomock.NewController(t)

	tests := []struct {
		name   string
		params []string
		setup  func() *executorImpl[*MockShardProcessor]
		assert func(err error, executor *executorImpl[*MockShardProcessor])
	}{
		{
			name:   "RemoveShardsFromLocalLogic fails if the namespace is onboarded",
			params: []string{},
			setup: func() *executorImpl[*MockShardProcessor] {
				// Create the executor currently has shards 1 and 2 assigned to it
				executor := &executorImpl[*MockShardProcessor]{
					logger:     log.NewNoop(),
					metrics:    tally.NoopScope,
					timeSource: clock.NewMockedTimeSource(),
				}
				executor.setMigrationMode(types.MigrationModeONBOARDED)
				return executor
			},
			assert: func(err error, executor *executorImpl[*MockShardProcessor]) {},
		},
		{
			name: "RemoveShardsFromLocalLogic succeed ",
			params: []string{
				"test-shard-id2",
				"test-shard-id3"},
			setup: func() *executorImpl[*MockShardProcessor] {
				shardProcessorMock1 := NewMockShardProcessor(ctrl)
				shardProcessorMock2 := NewMockShardProcessor(ctrl)
				// Create the executor currently has shards 1 and 2 assigned to it
				executor := &executorImpl[*MockShardProcessor]{
					logger:     log.NewNoop(),
					metrics:    tally.NoopScope,
					timeSource: clock.NewMockedTimeSource(),
				}

				executor.managedProcessors.Store("test-shard-id1", newManagedProcessor(shardProcessorMock1, processorStateStarted))
				executor.managedProcessors.Store("test-shard-id2", newManagedProcessor(shardProcessorMock2, processorStateStarted))

				// With the new assignment, shardProcessorMock2 should be stopped
				shardProcessorMock2.EXPECT().Stop()
				shardProcessorMock1.EXPECT().GetShardReport().Return(ShardReport{Status: types.ShardStatusREADY})
				return executor
			},
			assert: func(err error, executor *executorImpl[*MockShardProcessor]) {
				// Assert that we now have the 1 shard in the assignment
				processor1, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
				assert.NoError(t, err)
				assert.Equal(t, types.ShardStatusREADY, processor1.GetShardReport().Status)

				// Check that we do not have shard "test-shard-id2" in the local cache
				// we lookup directly since we don't want to trigger a heartbeat
				_, ok := executor.managedProcessors.Load("test-shard-id2")
				assert.False(t, ok)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor := test.setup()
			err := executor.RemoveShardsFromLocalLogic(test.params)
			time.Sleep(10 * time.Millisecond) // Force the updateShardAssignment goroutines to run
			test.assert(err, executor)
		})
	}
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
			Metadata:           make(map[string]string),
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
	}
	executor.setMigrationMode(types.MigrationModeINVALID)

	shardAssignments, migrationMode, err := executor.heartbeat(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 1, len(shardAssignments))
	assert.Equal(t, types.MigrationModeDISTRIBUTEDPASSTHROUGH, migrationMode)
	assert.Equal(t, types.MigrationModeDISTRIBUTEDPASSTHROUGH, executor.getMigrationMode())
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
	}
	executor.setMigrationMode(types.MigrationModeDISTRIBUTEDPASSTHROUGH)

	_, migrationMode, err := executor.heartbeat(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, types.MigrationModeONBOARDED, migrationMode)
	assert.Equal(t, types.MigrationModeONBOARDED, executor.getMigrationMode())
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
	}
	executor.setMigrationMode(types.MigrationModeLOCALPASSTHROUGH)

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
	}
	executor.setMigrationMode(types.MigrationModeONBOARDED)

	executor.Start(context.Background())
	defer executor.Stop()

	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(15 * time.Second)
	time.Sleep(10 * time.Millisecond)
	mockTimeSource.BlockUntil(1)

	// Assert that no shards were assigned, we check directly since we don't want to trigger a heartbeat
	_, ok := executor.managedProcessors.Load("test-shard-id1")
	assert.False(t, ok)
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
	}
	executor.setMigrationMode(types.MigrationModeONBOARDED)

	executor.Start(context.Background())
	defer executor.Stop()

	mockTimeSource.BlockUntil(1)
	mockTimeSource.Advance(15 * time.Second)
	time.Sleep(10 * time.Millisecond)
	mockTimeSource.BlockUntil(1)

	// Assert that the shard was assigned
	processor, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
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

func TestGetShardProcess_NonOwnedShard_Fails(t *testing.T) {
	cases := map[string]struct {
		migrationMode             types.MigrationMode
		expectedError             error
		shardsInCache             []string
		shardsReturnedOnHeartbeat map[string]*types.ShardAssignment
		heartbeatCallsExpected    int
		heartBeatError            error
		setupMocks                func(shardProcessorFactory *MockShardProcessorFactory[*MockShardProcessor], shardProcessor *MockShardProcessor)
	}{
		"empty cache local passthrough": {
			migrationMode:          types.MigrationModeLOCALPASSTHROUGH,
			expectedError:          fmt.Errorf("shard process not found for shard ID: test-shard-id1"),
			shardsInCache:          []string{},
			heartbeatCallsExpected: 0,
		},
		"shard found": {
			migrationMode:          types.MigrationModeONBOARDED,
			shardsInCache:          []string{"test-shard-id1"},
			heartbeatCallsExpected: 0,
		},
		"shard found on heartbeat": {
			migrationMode: types.MigrationModeONBOARDED,
			shardsInCache: []string{},
			shardsReturnedOnHeartbeat: map[string]*types.ShardAssignment{
				"test-shard-id1": {Status: types.AssignmentStatusREADY},
			},
			heartbeatCallsExpected: 1,
			setupMocks: func(processorFactory *MockShardProcessorFactory[*MockShardProcessor], processor *MockShardProcessor) {
				processorFactory.EXPECT().NewShardProcessor(gomock.Any()).Return(processor, nil)
				processor.EXPECT().Start(gomock.Any())
			},
		},
		"shard not found on heartbeat": {
			migrationMode:             types.MigrationModeONBOARDED,
			shardsInCache:             []string{},
			shardsReturnedOnHeartbeat: map[string]*types.ShardAssignment{},
			heartbeatCallsExpected:    1,
			expectedError:             fmt.Errorf("shard process not found for shard ID: test-shard-id1"),
		},
		"heartbeat error": {
			migrationMode:          types.MigrationModeONBOARDED,
			shardsInCache:          []string{},
			heartbeatCallsExpected: 1,
			expectedError:          fmt.Errorf("heartbeat and assign shards"),
			heartBeatError:         assert.AnError,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			shardDistributorClient := sharddistributorexecutor.NewMockClient(ctrl)
			shardDistributorClient.EXPECT().Heartbeat(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&types.ExecutorHeartbeatResponse{
					ShardAssignments: tc.shardsReturnedOnHeartbeat,
					MigrationMode:    tc.migrationMode,
				}, tc.heartBeatError).Times(tc.heartbeatCallsExpected)

			shardProcessorFactory := NewMockShardProcessorFactory[*MockShardProcessor](ctrl)
			if tc.setupMocks != nil {
				tc.setupMocks(shardProcessorFactory, NewMockShardProcessor(ctrl))
			}

			executor := &executorImpl[*MockShardProcessor]{
				logger:                 log.NewNoop(),
				shardProcessorFactory:  shardProcessorFactory,
				metrics:                tally.NoopScope,
				shardDistributorClient: shardDistributorClient,
				timeSource:             clock.NewMockedTimeSource(),
			}
			executor.setMigrationMode(tc.migrationMode)

			for _, shardID := range tc.shardsInCache {
				executor.managedProcessors.Store(shardID, newManagedProcessor(NewMockShardProcessor(ctrl), processorStateStarted))
			}

			_, err := executor.GetShardProcess(context.Background(), "test-shard-id1")
			if tc.expectedError != nil {
				assert.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecutorMetadata_SetAndGet(t *testing.T) {
	metadata := &syncExecutorMetadata{
		data: make(map[string]string),
	}

	// Set some metadata
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	metadata.Set(testData)

	// Get the metadata
	result := metadata.Get()

	assert.Equal(t, testData, result)
}

func TestExecutorMetadata_DefensiveCopy(t *testing.T) {
	metadata := &syncExecutorMetadata{
		data: map[string]string{
			"key1": "value1",
		},
	}

	// Get the metadata
	result := metadata.Get()

	// Modify the returned map
	result["key1"] = "modified"
	result["key2"] = "new"

	// Original metadata should be unchanged
	original := metadata.Get()
	assert.Equal(t, "value1", original["key1"])
	assert.NotContains(t, original, "key2")
}

func TestExecutorMetadata_ConcurrentAccess(t *testing.T) {
	metadata := &syncExecutorMetadata{
		data: make(map[string]string),
	}

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				metadata.Set(map[string]string{
					fmt.Sprintf("key-%d", id): fmt.Sprintf("value-%d-%d", id, j),
				})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = metadata.Get()
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have some data
	result := metadata.Get()
	assert.NotNil(t, result)
}

func TestExecutorMetadata_NilHandling(t *testing.T) {
	metadata := &syncExecutorMetadata{
		data: nil,
	}

	// Get should return empty map, not nil
	result := metadata.Get()
	assert.NotNil(t, result)
	assert.Empty(t, result)

	// Set with nil should work
	metadata.Set(nil)
	result = metadata.Get()
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestExecutorMetadata_EmptyMap(t *testing.T) {
	metadata := &syncExecutorMetadata{
		data: make(map[string]string),
	}

	result := metadata.Get()
	assert.NotNil(t, result)
	assert.Empty(t, result)

	// Set empty map
	metadata.Set(map[string]string{})
	result = metadata.Get()
	assert.NotNil(t, result)
	assert.Empty(t, result)
}
