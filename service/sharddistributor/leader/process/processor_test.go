package process

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	metricmocks "github.com/uber/cadence/common/metrics/mocks"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/config/configtest"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type testDependencies struct {
	ctrl       *gomock.Controller
	store      *store.MockStore
	election   *store.MockElection
	timeSource clock.MockedTimeSource
	factory    Factory
	cfg        config.Namespace
	sdConfig   *config.Config
}

func setupProcessorTest(t *testing.T, namespaceType string) *testDependencies {
	migrationConfig := configtest.NewTestMigrationConfig(t,
		configtest.ConfigEntry{
			Key:   dynamicproperties.ShardDistributorMigrationMode,
			Value: config.MigrationModeONBOARDED})
	return setupProcessorTestWithMigrationConfig(t, namespaceType, migrationConfig)
}

func setupProcessorTestWithMigrationConfig(t *testing.T, namespaceType string, migrationConfig *config.Config) *testDependencies {
	ctrl := gomock.NewController(t)
	mockedClock := clock.NewMockedTimeSource()
	deps := &testDependencies{
		ctrl:       ctrl,
		store:      store.NewMockStore(ctrl),
		election:   store.NewMockElection(ctrl),
		timeSource: mockedClock,
		cfg:        config.Namespace{Name: "test-ns", ShardNum: 2, Type: namespaceType, Mode: config.MigrationModeONBOARDED},
	}
	deps.sdConfig = &config.Config{
		LoadBalancingMode: func(namespace string) string {
			return config.LoadBalancingModeNAIVE
		},
		LoadBalancingNaive: config.LoadBalancingNaiveConfig{
			MaxDeviation: func(namespace string) float64 {
				return 2.0
			},
		},
		MigrationMode: migrationConfig.MigrationMode,
	}

	deps.factory = NewProcessorFactory(
		testlogger.New(t),
		metrics.NewNoopMetricsClient(),
		mockedClock,
		config.ShardDistribution{
			Process: config.LeaderProcess{
				Period:       time.Second,
				HeartbeatTTL: time.Second,
			},
		},
		deps.sdConfig,
	)
	return deps
}

func TestRunAndTerminate(t *testing.T) {
	defer goleak.VerifyNone(t)

	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election)
	ctx, cancel := context.WithCancel(context.Background())

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: 0}, nil).AnyTimes()
	mocks.store.EXPECT().Subscribe(gomock.Any(), mocks.cfg.Name).Return(make(chan int64), nil).AnyTimes()

	err := processor.Run(ctx)
	require.NoError(t, err)

	err = processor.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "processor is already running")

	err = processor.Terminate(context.Background())
	require.NoError(t, err)

	err = processor.Terminate(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "processor has not been started")

	cancel()
}

func TestRebalanceShards_InitialDistribution(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: 1}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(nil, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(nil, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			assert.Len(t, request.NewState.ShardAssignments, 2)
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].AssignedShards, 1)
			assert.Len(t, request.NewState.ShardAssignments["exec-2"].AssignedShards, 1)
			assert.Lenf(t, request.NewState.ShardAssignments["exec-1"].ShardHandoverStats, 0, "no handover stats should be present on initial assignment")
			assert.Lenf(t, request.NewState.ShardAssignments["exec-2"].ShardHandoverStats, 0, "no handover stats should be present on initial assignment")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), processor.lastAppliedRevision)
}

func TestRebalanceShards_ExecutorRemoved(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		"exec-2": {Status: types.ExecutorStatusDRAINING, LastHeartbeat: now},
	}
	assignments := map[string]store.AssignedState{
		"exec-2": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
				"1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		ShardAssignments: assignments,
		GlobalRevision:   1,
	}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(&store.ShardOwner{ExecutorID: "exec-2"}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(&store.ShardOwner{ExecutorID: "exec-1"}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].AssignedShards, 2)
			assert.Len(t, request.NewState.ShardAssignments["exec-2"].AssignedShards, 0)
			assert.Lenf(t, request.NewState.ShardAssignments["exec-1"].ShardHandoverStats, 1, "only shard 0 should have handover stats")
			assert.Lenf(t, request.NewState.ShardAssignments["exec-2"].ShardHandoverStats, 0, "no handover stats should be present for drained executor")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_ExecutorStale(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now.Add(-2 * time.Second)},
	}
	assignments := map[string]store.AssignedState{
		"exec-1": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
			},
			ModRevision: 1,
		},
		"exec-2": {
			AssignedShards: map[string]*types.ShardAssignment{
				"1": {Status: types.AssignmentStatusREADY},
			},
			ModRevision: 1,
		},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		ShardAssignments: assignments,
		GlobalRevision:   1,
	}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(&store.ShardOwner{ExecutorID: "exec-1"}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(&store.ShardOwner{ExecutorID: "exec-2"}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			assert.Len(t, request.NewState.ShardAssignments, 1)
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].AssignedShards, 2)
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].ShardHandoverStats, 1, "only shard 1 should have handover stats")
			assert.Equal(t, request.ExecutorsToDelete, map[string]int64{"exec-2": 1})
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_NoActiveExecutors(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusDRAINING, LastHeartbeat: now},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: int64(1)}, nil)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_NoActiveExecutors_OneStaleExecutors(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusDRAINING, LastHeartbeat: now},
		"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now.Add(-2 * time.Second)},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: int64(1)}, nil)

	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().DeleteAssignedStates(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, namespace string, executorIDs []string, _ store.GuardFunc) error {
			assert.ElementsMatch(t, []string{"exec-2"}, executorIDs)
			assert.Equal(t, mocks.cfg.Name, namespace)
			return nil
		})

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_NoActiveExecutors_AllStaleExecutors(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusDRAINING, LastHeartbeat: now.Add(-2 * time.Second)},
		"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now.Add(-2 * time.Second)},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: int64(1)}, nil)

	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().DeleteAssignedStates(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, namespace string, executorIDs []string, _ store.GuardFunc) error {
			assert.ElementsMatch(t, []string{"exec-1", "exec-2"}, executorIDs)
			assert.Equal(t, mocks.cfg.Name, namespace)
			return nil
		})

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_NoRebalanceNeeded(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	processor.lastAppliedRevision = 1

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: int64(1)}, nil)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestCleanupStaleExecutors(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	now := mocks.timeSource.Now()

	heartbeats := map[string]store.HeartbeatState{
		"exec-active": {LastHeartbeat: now},
		"exec-stale":  {LastHeartbeat: now.Add(-2 * time.Second)},
	}

	namespaceState := &store.NamespaceState{Executors: heartbeats}

	staleExecutors := processor.identifyStaleExecutors(namespaceState)
	assert.Equal(t, map[string]int64{"exec-stale": 0}, staleExecutors)
}

func TestCleanupStaleShardStats(t *testing.T) {
	t.Run("stale shard stats are deleted", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		now := mocks.timeSource.Now().UTC()

		heartbeats := map[string]store.HeartbeatState{
			"exec-active": {LastHeartbeat: now, Status: types.ExecutorStatusACTIVE},
			"exec-stale":  {LastHeartbeat: now.Add(-2 * time.Second)},
		}

		assignments := map[string]store.AssignedState{
			"exec-active": {
				AssignedShards: map[string]*types.ShardAssignment{
					"shard-1": {Status: types.AssignmentStatusREADY},
					"shard-2": {Status: types.AssignmentStatusREADY},
				},
			},
			"exec-stale": {
				AssignedShards: map[string]*types.ShardAssignment{
					"shard-3": {Status: types.AssignmentStatusREADY},
				},
			},
		}

		shardStats := map[string]store.ShardStatistics{
			"shard-1": {SmoothedLoad: 1.0, LastUpdateTime: now, LastMoveTime: now},
			"shard-2": {SmoothedLoad: 2.0, LastUpdateTime: now, LastMoveTime: now},
			"shard-3": {SmoothedLoad: 3.0, LastUpdateTime: now.Add(-2 * time.Second), LastMoveTime: now.Add(-2 * time.Second)},
		}

		namespaceState := &store.NamespaceState{
			Executors:        heartbeats,
			ShardAssignments: assignments,
			ShardStats:       shardStats,
		}

		staleShardStats := processor.identifyStaleShardStats(namespaceState)
		assert.Equal(t, []string{"shard-3"}, staleShardStats)
	})

	t.Run("recent shard stats are preserved", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		now := mocks.timeSource.Now()

		expiredExecutor := now.Add(-2 * time.Second)
		namespaceState := &store.NamespaceState{
			Executors: map[string]store.HeartbeatState{
				"exec-stale": {LastHeartbeat: expiredExecutor},
			},
			ShardAssignments: map[string]store.AssignedState{},
			ShardStats: map[string]store.ShardStatistics{
				"shard-1": {SmoothedLoad: 5.0, LastUpdateTime: now, LastMoveTime: now},
			},
		}

		staleShardStats := processor.identifyStaleShardStats(namespaceState)
		assert.Empty(t, staleShardStats)
	})

}

func TestRebalance_StoreErrors(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	expectedErr := errors.New("store is down")

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(nil, expectedErr)
	err := processor.rebalanceShards(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())

	now := mocks.timeSource.Now()
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:      map[string]store.HeartbeatState{"e": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now}},
		GlobalRevision: 1,
	}, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(nil, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(nil, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).Return(expectedErr)
	err = processor.rebalanceShards(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestRunLoop_SubscriptionError(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	expectedErr := errors.New("subscription failed")
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: 0}, nil)
	mocks.store.EXPECT().Subscribe(gomock.Any(), mocks.cfg.Name).Return(nil, expectedErr)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		processor.runRebalancingLoop(context.Background())
	}()
	wg.Wait()
}

func TestRunLoop_ContextCancellation(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	ctx, cancel := context.WithCancel(context.Background())

	// Setup for the initial call to rebalanceShards and the subscription
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: 0}, nil)
	mocks.store.EXPECT().Subscribe(gomock.Any(), mocks.cfg.Name).Return(make(chan int64), nil)

	processor.wg.Add(1)
	// Run the process in a separate goroutine to avoid blocking the test
	go processor.runProcess(ctx)

	// Wait for the two loops (rebalance and cleanup) to create their tickers
	mocks.timeSource.BlockUntil(2)

	// Now, cancel the context to signal the loops to stop
	cancel()

	// Wait for the main process loop to exit gracefully
	processor.wg.Wait()
}

func TestRebalanceShards_WithUnassignedShardsButMigrationModeNotOnboarded(t *testing.T) {
	migrationConfig := configtest.NewTestMigrationConfig(t, configtest.ConfigEntry{
		Key:   dynamicproperties.ShardDistributorMigrationMode,
		Value: config.MigrationModeDISTRIBUTEDPASSTHROUGH})
	mocks := setupProcessorTestWithMigrationConfig(t, config.NamespaceTypeFixed, migrationConfig)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
	}
	// Note: shard "1" is missing from assignments
	assignments := map[string]store.AssignedState{
		"exec-1": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(nil, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(nil, nil)
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		ShardAssignments: assignments,
		GlobalRevision:   3,
	}, nil)
	// These are the expected calls in case of onboarding, with the assignment of new shards
	mocks.election.EXPECT().Guard().Return(store.NopGuard()).Times(0)
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].AssignedShards, 2, "Both shards should now be assigned to exec-1")
			return nil
		},
	).Times(0)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_ShadowModeWithStaleExecutors(t *testing.T) {
	t.Run("stale executors are deleted in shadow mode", func(t *testing.T) {
		migrationConfig := configtest.NewTestMigrationConfig(t, configtest.ConfigEntry{
			Key:   dynamicproperties.ShardDistributorMigrationMode,
			Value: config.MigrationModeDISTRIBUTEDPASSTHROUGH})
		mocks := setupProcessorTestWithMigrationConfig(t, config.NamespaceTypeFixed, migrationConfig)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		now := mocks.timeSource.Now()
		heartbeats := map[string]store.HeartbeatState{
			"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now.Add(-10 * time.Second)},
		}
		assignments := map[string]store.AssignedState{
			"exec-1": {
				AssignedShards: map[string]*types.ShardAssignment{
					"0": {Status: types.AssignmentStatusREADY},
				},
				ModRevision: 1,
			},
			"exec-2": {
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				ModRevision: 1,
			},
		}
		mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
			Executors:        heartbeats,
			ShardAssignments: assignments,
			GlobalRevision:   1,
		}, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(&store.ShardOwner{ExecutorID: "exec-1"}, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(&store.ShardOwner{ExecutorID: "exec-2"}, nil)
		mocks.election.EXPECT().Guard().Return(store.NopGuard())
		mocks.store.EXPECT().DeleteExecutors(gomock.Any(), mocks.cfg.Name, []string{"exec-2"}, gomock.Any()).Return(nil)

		err := processor.rebalanceShards(context.Background())
		require.NoError(t, err)
	})

	t.Run("delete executors error is non-blocking in shadow mode", func(t *testing.T) {
		migrationConfig := configtest.NewTestMigrationConfig(t, configtest.ConfigEntry{
			Key:   dynamicproperties.ShardDistributorMigrationMode,
			Value: config.MigrationModeDISTRIBUTEDPASSTHROUGH})
		mocks := setupProcessorTestWithMigrationConfig(t, config.NamespaceTypeFixed, migrationConfig)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		now := mocks.timeSource.Now()
		heartbeats := map[string]store.HeartbeatState{
			"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			"exec-2": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now.Add(-10 * time.Second)},
		}
		assignments := map[string]store.AssignedState{
			"exec-1": {
				AssignedShards: map[string]*types.ShardAssignment{
					"0": {Status: types.AssignmentStatusREADY},
				},
				ModRevision: 1,
			},
			"exec-2": {
				AssignedShards: map[string]*types.ShardAssignment{
					"1": {Status: types.AssignmentStatusREADY},
				},
				ModRevision: 1,
			},
		}
		mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
			Executors:        heartbeats,
			ShardAssignments: assignments,
			GlobalRevision:   1,
		}, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(&store.ShardOwner{ExecutorID: "exec-1"}, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(&store.ShardOwner{ExecutorID: "exec-2"}, nil)
		mocks.election.EXPECT().Guard().Return(store.NopGuard())
		mocks.store.EXPECT().DeleteExecutors(gomock.Any(), mocks.cfg.Name, []string{"exec-2"}, gomock.Any()).Return(errors.New("transaction failed"))

		err := processor.rebalanceShards(context.Background())
		require.NoError(t, err)
	})

	t.Run("no stale executors - delete not called in shadow mode", func(t *testing.T) {
		migrationConfig := configtest.NewTestMigrationConfig(t, configtest.ConfigEntry{
			Key:   dynamicproperties.ShardDistributorMigrationMode,
			Value: config.MigrationModeDISTRIBUTEDPASSTHROUGH})
		mocks := setupProcessorTestWithMigrationConfig(t, config.NamespaceTypeFixed, migrationConfig)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		now := mocks.timeSource.Now()
		heartbeats := map[string]store.HeartbeatState{
			"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		}
		assignments := map[string]store.AssignedState{
			"exec-1": {
				AssignedShards: map[string]*types.ShardAssignment{
					"0": {Status: types.AssignmentStatusREADY},
				},
			},
		}
		mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
			Executors:        heartbeats,
			ShardAssignments: assignments,
			GlobalRevision:   1,
		}, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(nil, nil)
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(nil, nil)
		// DeleteExecutors should not be called when there are no stale executors, thus Times(0)
		mocks.store.EXPECT().DeleteExecutors(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		err := processor.rebalanceShards(context.Background())
		require.NoError(t, err)
	})
}

func TestRebalanceShards_NoShardsToReassign(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
	}
	assignments := map[string]store.AssignedState{
		"exec-1": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
				"1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		ShardAssignments: assignments,
		GlobalRevision:   2,
	}, nil)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), processor.lastAppliedRevision, "Revision should be updated even if no shards were moved")
}

func TestRebalanceShards_WithUnassignedShards(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := mocks.timeSource.Now()
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
	}
	// Note: shard "1" is missing from assignments
	assignments := map[string]store.AssignedState{
		"exec-1": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "0").Return(nil, nil)
	mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, "1").Return(nil, nil)
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		ShardAssignments: assignments,
		GlobalRevision:   3,
	}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			assert.Len(t, request.NewState.ShardAssignments["exec-1"].AssignedShards, 2, "Both shards should now be assigned to exec-1")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestGetShards_Utility(t *testing.T) {
	t.Run("Fixed type", func(t *testing.T) {
		cfg := config.Namespace{Type: config.NamespaceTypeFixed, ShardNum: 5}
		shards := getShards(cfg, nil, nil)
		assert.Equal(t, []string{"0", "1", "2", "3", "4"}, shards)
	})

	t.Run("Ephemeral type", func(t *testing.T) {
		cfg := config.Namespace{Type: config.NamespaceTypeEphemeral}
		nsState := &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				"executor1": {
					AssignedShards: map[string]*types.ShardAssignment{
						"s0": {Status: types.AssignmentStatusREADY},
						"s1": {Status: types.AssignmentStatusREADY},
						"s2": {Status: types.AssignmentStatusREADY},
					},
				},
				"executor2": {
					AssignedShards: map[string]*types.ShardAssignment{
						"s3": {Status: types.AssignmentStatusREADY},
						"s4": {Status: types.AssignmentStatusREADY},
					},
				},
			},
		}
		shards := getShards(cfg, nsState, nil)
		slices.Sort(shards)
		assert.Equal(t, []string{"s0", "s1", "s2", "s3", "s4"}, shards)
	})

	t.Run("Ephemeral type with deleted shards", func(t *testing.T) {
		cfg := config.Namespace{Type: config.NamespaceTypeEphemeral}
		nsState := &store.NamespaceState{
			ShardAssignments: map[string]store.AssignedState{
				"executor1": {
					AssignedShards: map[string]*types.ShardAssignment{
						"s0": {Status: types.AssignmentStatusREADY},
						"s1": {Status: types.AssignmentStatusREADY},
						"s2": {Status: types.AssignmentStatusREADY},
					},
				},
				"executor2": {
					AssignedShards: map[string]*types.ShardAssignment{
						"s3": {Status: types.AssignmentStatusREADY},
						"s4": {Status: types.AssignmentStatusREADY},
					},
				},
			},
		}
		deletedShards := map[string]store.ShardState{
			"s0": {},
			"s1": {},
		}
		shards := getShards(cfg, nsState, deletedShards)
		slices.Sort(shards)
		assert.Equal(t, []string{"s2", "s3", "s4"}, shards)
	})

	// Unknown type
	t.Run("Other type", func(t *testing.T) {
		cfg := config.Namespace{Type: "other"}
		shards := getShards(cfg, nil, nil)
		assert.Nil(t, shards)
	})
}

func TestAssignShardsToEmptyExecutors(t *testing.T) {
	cases := []struct {
		name                       string
		inputAssignments           map[string][]string
		expectedAssignments        map[string][]string
		expectedDistributonChanged bool
	}{
		{
			name:                       "no executors",
			inputAssignments:           map[string][]string{},
			expectedAssignments:        map[string][]string{},
			expectedDistributonChanged: false,
		},
		{
			name: "no empty executors",
			inputAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-3", "shard-4", "shard-5", "shard-6"},
				"exec-2": {"shard-7", "shard-8"},
			},
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-3", "shard-4", "shard-5", "shard-6"},
				"exec-2": {"shard-7", "shard-8"},
			},
			expectedDistributonChanged: false,
		},
		{
			name: "empty executor",
			inputAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-3", "shard-4", "shard-5", "shard-6"},
				"exec-2": {"shard-7", "shard-8", "shard-9", "shard-10"},
				"exec-3": {},
			},
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-2", "shard-3", "shard-4", "shard-5", "shard-6"},
				"exec-2": {"shard-8", "shard-9", "shard-10"},
				"exec-3": {"shard-1", "shard-7"},
			},
			expectedDistributonChanged: true,
		},
		{
			name:                       "all empty executors",
			inputAssignments:           map[string][]string{"exec-1": {}, "exec-2": {}, "exec-3": {}},
			expectedAssignments:        map[string][]string{"exec-1": {}, "exec-2": {}, "exec-3": {}},
			expectedDistributonChanged: false,
		},
		{
			name: "multiple empty executors",
			inputAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-3", "shard-4", "shard-5", "shard-6", "shard-7", "shard-8", "shard-9", "shard-10"},
				"exec-2": {"shard-11", "shard-12", "shard-13", "shard-14", "shard-15", "shard-16", "shard-17"},
				"exec-3": {"shard-18", "shard-19", "shard-20", "shard-21", "shard-22", "shard-23", "shard-24"},
				"exec-4": {},
				"exec-5": {},
			},
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-4", "shard-5", "shard-6", "shard-7", "shard-8", "shard-9", "shard-10"},
				"exec-2": {"shard-14", "shard-15", "shard-16", "shard-17"},
				"exec-3": {"shard-20", "shard-21", "shard-22", "shard-23", "shard-24"},
				"exec-4": {"shard-1", "shard-18", "shard-12", "shard-3"},
				"exec-5": {"shard-11", "shard-2", "shard-19", "shard-13"},
			},
			expectedDistributonChanged: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actualDistributionChanged := assignShardsToEmptyExecutors(c.inputAssignments)

			assert.Equal(t, c.expectedAssignments, c.inputAssignments)
			assert.Equal(t, c.expectedDistributonChanged, actualDistributionChanged)
		})
	}
}

func TestNewHandoverStats(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	now := time.Now().UTC()
	shardID := "shard-1"
	newExecutorID := "exec-new"

	type testCase struct {
		name             string
		getOwner         *store.ShardOwner
		getOwnerErr      error
		executors        map[string]store.HeartbeatState
		expectShardStats *store.ShardHandoverStats // nil means expect nil result
	}

	testCases := []testCase{
		{
			name:             "error other than shard not found -> stat without handover",
			getOwner:         nil,
			getOwnerErr:      errors.New("random error"),
			executors:        map[string]store.HeartbeatState{},
			expectShardStats: nil,
		},
		{
			name:             "ErrShardNotFound -> stat without handover",
			getOwner:         nil,
			getOwnerErr:      store.ErrShardNotFound,
			executors:        map[string]store.HeartbeatState{},
			expectShardStats: nil,
		},
		{
			name:        "same executor as previous -> nil",
			getOwner:    &store.ShardOwner{ExecutorID: newExecutorID},
			getOwnerErr: nil,
			executors: map[string]store.HeartbeatState{
				newExecutorID: {
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-10 * time.Second)},
			},
			expectShardStats: nil,
		},
		{
			name:             "prev executor different but heartbeat missing -> no handover",
			getOwner:         &store.ShardOwner{ExecutorID: "old-exec"},
			getOwnerErr:      nil,
			executors:        map[string]store.HeartbeatState{},
			expectShardStats: nil,
		},
		{
			name:        "prev executor ACTIVE -> emergency handover",
			getOwner:    &store.ShardOwner{ExecutorID: "old-active"},
			getOwnerErr: nil,
			executors: map[string]store.HeartbeatState{
				"old-active": {
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-10 * time.Second),
				},
			},
			expectShardStats: &store.ShardHandoverStats{
				HandoverType:                      types.HandoverTypeEMERGENCY,
				PreviousExecutorLastHeartbeatTime: now.Add(-10 * time.Second),
			},
		},
		{
			name:        "prev executor DRAINING -> graceful handover",
			getOwner:    &store.ShardOwner{ExecutorID: "old-draining"},
			getOwnerErr: nil,
			executors: map[string]store.HeartbeatState{
				"old-draining": {
					Status:        types.ExecutorStatusDRAINING,
					LastHeartbeat: now.Add(-10 * time.Second),
				},
			},
			expectShardStats: &store.ShardHandoverStats{
				HandoverType:                      types.HandoverTypeGRACEFUL,
				PreviousExecutorLastHeartbeatTime: now.Add(-10 * time.Second),
			},
		},
		{
			name:        "prev executor DRAINED -> graceful handover",
			getOwner:    &store.ShardOwner{ExecutorID: "old-drained"},
			getOwnerErr: nil,
			executors: map[string]store.HeartbeatState{
				"old-drained": {
					Status:        types.ExecutorStatusDRAINING,
					LastHeartbeat: now.Add(-10 * time.Second),
				},
			},
			expectShardStats: &store.ShardHandoverStats{
				HandoverType:                      types.HandoverTypeGRACEFUL,
				PreviousExecutorLastHeartbeatTime: now.Add(-10 * time.Second),
			},
		},
	}

	for _, tc := range testCases {
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, shardID).Return(tc.getOwner, tc.getOwnerErr)
		t.Run(tc.name, func(t *testing.T) {
			stat := processor.newHandoverStats(&store.NamespaceState{Executors: tc.executors}, shardID, newExecutorID)
			if tc.expectShardStats == nil {
				require.Nil(t, stat)
				return
			}
			require.NotNil(t, stat)
			require.Equal(t, tc.expectShardStats, stat)
		})
	}
}
func TestAddHandoverStatsToExecutorAssignedState(t *testing.T) {

	now := time.Now().UTC()
	executorID := "exec-1"
	shardIDs := []string{"shard-1", "shard-2"}

	for name, tc := range map[string]struct {
		name      string
		executors map[string]store.HeartbeatState

		getOwners    map[string]*store.ShardOwner
		getOwnerErrs map[string]error

		expected map[string]store.ShardHandoverStats
	}{
		"no previous owner for both shards": {
			getOwners:    map[string]*store.ShardOwner{"shard-1": nil, "shard-2": nil},
			getOwnerErrs: map[string]error{"shard-1": store.ErrShardNotFound, "shard-2": store.ErrShardNotFound},
			executors:    map[string]store.HeartbeatState{},
			expected:     map[string]store.ShardHandoverStats{},
		},
		"emergency handover for shard-1, no handover for shard-2": {
			getOwners: map[string]*store.ShardOwner{
				"shard-1": {ExecutorID: "old-active"},
				"shard-2": nil,
			},
			getOwnerErrs: map[string]error{
				"shard-1": nil,
				"shard-2": store.ErrShardNotFound,
			},
			executors: map[string]store.HeartbeatState{
				"old-active": {
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-10 * time.Second),
				},
			},
			expected: map[string]store.ShardHandoverStats{
				"shard-1": {
					HandoverType:                      types.HandoverTypeEMERGENCY,
					PreviousExecutorLastHeartbeatTime: now.Add(-10 * time.Second),
				},
			},
		},
		"graceful handover for shard-1": {
			getOwners: map[string]*store.ShardOwner{
				"shard-1": {ExecutorID: "old-draining"},
				"shard-2": nil,
			},
			getOwnerErrs: map[string]error{
				"shard-1": nil,
			},
			executors: map[string]store.HeartbeatState{
				"old-draining": {
					Status:        types.ExecutorStatusDRAINING,
					LastHeartbeat: now.Add(-20 * time.Second),
				},
			},
			expected: map[string]store.ShardHandoverStats{
				"shard-1": {
					HandoverType:                      types.HandoverTypeGRACEFUL,
					PreviousExecutorLastHeartbeatTime: now.Add(-20 * time.Second),
				},
			},
		},
		"same executor as previous, no handover": {
			getOwners: map[string]*store.ShardOwner{
				"shard-1": {ExecutorID: executorID},
				"shard-2": nil,
			},
			getOwnerErrs: map[string]error{
				"shard-1": nil,
			},
			executors: map[string]store.HeartbeatState{
				executorID: {
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now,
				},
			},
			expected: map[string]store.ShardHandoverStats{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
			defer mocks.ctrl.Finish()
			processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

			for _, shardID := range shardIDs {
				mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, shardID).Return(tc.getOwners[shardID], tc.getOwnerErrs[shardID]).AnyTimes()
			}
			namespaceState := &store.NamespaceState{
				Executors: tc.executors,
			}
			stats := processor.addHandoverStatsToExecutorAssignedState(namespaceState, executorID, shardIDs)
			assert.Equal(t, tc.expected, stats)
		})
	}
}

func TestRebalanceByShardLoad(t *testing.T) {
	cases := []struct {
		name                       string
		shardLoad                  map[string]float64
		currentAssignments         map[string][]string
		maxDeviation               float64
		expectedDistributionChange bool
		expectedAssignments        map[string][]string
	}{
		{
			name:                       "single executor - no rebalance",
			shardLoad:                  map[string]float64{"shard-1": 10.0},
			currentAssignments:         map[string][]string{"exec-1": {"shard-1"}},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments:        map[string][]string{"exec-1": {"shard-1"}},
		},
		{
			name: "balanced load - no rebalance needed",
			shardLoad: map[string]float64{
				"shard-1": 10.0,
				"shard-2": 10.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 10.0
				"exec-2": {"shard-2"}, // 10.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 10.0
				"exec-2": {"shard-2"}, // 10.0
			},
		},
		{
			name: "deviation below threshold - no rebalance",
			shardLoad: map[string]float64{
				"shard-1": 10.0,
				"shard-2": 15.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 10.0
				"exec-2": {"shard-2"}, // 15.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 10.0
				"exec-2": {"shard-2"}, // 15.0
			},
		},
		{
			name: "multiple shards - hottest moved",
			shardLoad: map[string]float64{
				"shard-1": 5.0,
				"shard-2": 30.0,
				"shard-3": 20.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"},            // 5.0
				"exec-2": {"shard-2", "shard-3"}, // 50.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: true,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2"}, // 35.0
				"exec-2": {"shard-3"},            // 20.0
			},
		},
		{
			name: "coldest would become hottest - no rebalance",
			shardLoad: map[string]float64{
				"shard-1": 10.0,
				"shard-2": 100.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 10.0
				"exec-2": {"shard-2"}, // 100.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
			},
		},
		{
			name: "multiple shards per executor",
			shardLoad: map[string]float64{
				"shard-1": 5.0, "shard-2": 5.0,
				"shard-3": 40.0, "shard-4": 30.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2"}, // 10.0
				"exec-2": {"shard-3", "shard-4"}, // 70.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: true,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-3"}, // 50.0
				"exec-2": {"shard-4"},                       // 30.0
			},
		},
		{
			name: "zero load shards - no rebalance",
			shardLoad: map[string]float64{
				"shard-1": 0.0,
				"shard-2": 50.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 0.0
				"exec-2": {"shard-2"}, // 50.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
			},
		},
		{
			name: "new shard load - equal shards - no rebalance",
			shardLoad: map[string]float64{
				"shard-2": 0.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"}, // 0.0
				"exec-2": {"shard-2"}, // 50.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
			},
		},
		{
			name: "four executors - balanced load",
			shardLoad: map[string]float64{
				"shard-1": 10.0,
				"shard-2": 10.0,
				"shard-3": 10.0,
				"shard-4": 10.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
				"exec-3": {"shard-3"},
				"exec-4": {"shard-4"},
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
				"exec-3": {"shard-3"},
				"exec-4": {"shard-4"},
			},
		}, {
			name: "four executors - one overloaded - no rebalance",
			shardLoad: map[string]float64{
				"shard-1": 10.0,
				"shard-2": 10.0,
				"shard-3": 10.0,
				"shard-4": 50.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
				"exec-3": {"shard-3"},
				"exec-4": {"shard-4"},
			},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1"},
				"exec-2": {"shard-2"},
				"exec-3": {"shard-3"},
				"exec-4": {"shard-4"},
			},
		}, {
			name: "four executors - uneven distribution - stale executor",
			shardLoad: map[string]float64{
				"shard-1": 15.0,
				"shard-2": 15.0,
				"shard-3": 15.0,
				"shard-4": 15.0,
				"shard-5": 40.0,
				"shard-6": 40.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2", "shard-5"}, // 70.0
				"exec-2": {"shard-3", "shard-4"},            // 30.0
				"exec-3": {},                                // 0.0
				"exec-4": {"shard-6"},                       // 40.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: true,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2"}, // 30.0
				"exec-2": {"shard-3", "shard-4"}, // 30.0
				"exec-3": {"shard-5"},            // 40.0
				"exec-4": {"shard-6"},            // 40.0
			},
		}, {
			name: "four executors - mixed load with multiple shards",
			shardLoad: map[string]float64{
				"shard-1": 5.0, "shard-2": 5.0,
				"shard-3": 5.0, "shard-4": 2.0,
				"shard-5": 25.0, "shard-6": 25.0,
				"shard-7": 15.0, "shard-8": 15.0,
			},
			currentAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2"}, // 10.0
				"exec-2": {"shard-3", "shard-4"}, // 7.0
				"exec-3": {"shard-5", "shard-6"}, // 50.0
				"exec-4": {"shard-7", "shard-8"}, // 30.0
			},
			maxDeviation:               2.0,
			expectedDistributionChange: true,
			expectedAssignments: map[string][]string{
				"exec-1": {"shard-1", "shard-2"},            // 10.0
				"exec-2": {"shard-3", "shard-4", "shard-6"}, // 32.0
				"exec-3": {"shard-5"},                       // 25.0
				"exec-4": {"shard-7", "shard-8"},            // 30.0
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
			mocks.cfg.Name = tc.name
			mocks.sdConfig.LoadBalancingNaive.MaxDeviation = func(namespace string) float64 {
				return tc.maxDeviation
			}
			defer mocks.ctrl.Finish()
			processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

			distributionChanged := processor.rebalanceByShardLoad(tc.shardLoad, tc.currentAssignments)

			assert.Equal(t, tc.expectedDistributionChange, distributionChanged, "distribution change mismatch")
			assert.Equal(t, tc.expectedAssignments, tc.currentAssignments, "final assignments mismatch")
		})
	}
}

func TestEmitExecutorMetric(t *testing.T) {
	tests := []struct {
		name           string
		executors      map[string]store.HeartbeatState
		expectedCounts map[types.ExecutorStatus]int
	}{
		{
			name:           "empty executors",
			executors:      map[string]store.HeartbeatState{},
			expectedCounts: map[types.ExecutorStatus]int{},
		},
		{
			name: "single active executor",
			executors: map[string]store.HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},
			},
			expectedCounts: map[types.ExecutorStatus]int{
				types.ExecutorStatusACTIVE: 1,
			},
		},
		{
			name: "multiple executors",
			executors: map[string]store.HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},
				"exec-2": {Status: types.ExecutorStatusACTIVE},
				"exec-3": {Status: types.ExecutorStatusDRAINING},
				"exec-4": {Status: types.ExecutorStatusDRAINED},
			},
			expectedCounts: map[types.ExecutorStatus]int{
				types.ExecutorStatusACTIVE:   2,
				types.ExecutorStatusDRAINING: 1,
				types.ExecutorStatusDRAINED:  1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
			defer mocks.ctrl.Finish()
			processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

			namespaceState := &store.NamespaceState{
				Executors: tt.executors,
			}

			metricsScope := &metricmocks.Scope{}

			for status, count := range tt.expectedCounts {
				taggedScope := &metricmocks.Scope{}
				metricsScope.On("Tagged", metrics.ExecutorStatusTag(status.String())).Return(taggedScope).Once()
				taggedScope.On("UpdateGauge", metrics.ShardDistributorTotalExecutors, float64(count)).Once()
			}

			processor.emitExecutorMetric(namespaceState, metricsScope)

			metricsScope.AssertExpectations(t)
		})
	}
}

func TestEmitOldestExecutorHeartbeatLag(t *testing.T) {
	tests := []struct {
		name        string
		executors   map[string]store.HeartbeatState
		expectedLag *float64
	}{
		{
			name:        "empty executors",
			executors:   map[string]store.HeartbeatState{},
			expectedLag: nil,
		},
		{
			name: "single executor",
			executors: map[string]store.HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},
			},
			expectedLag: common.Float64Ptr(5000),
		},
		{
			name: "multiple executors",
			executors: map[string]store.HeartbeatState{
				"exec-1": {Status: types.ExecutorStatusACTIVE},   // 5 seconds
				"exec-2": {Status: types.ExecutorStatusACTIVE},   // 10 seconds (oldest)
				"exec-3": {Status: types.ExecutorStatusDRAINING}, // 3 seconds
			},
			expectedLag: common.Float64Ptr(10000), // 10 seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
			defer mocks.ctrl.Finish()
			processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

			now := mocks.timeSource.Now()

			if tt.name == "single executor" {
				tt.executors["exec-1"] = store.HeartbeatState{
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-5 * time.Second),
				}
			} else if tt.name == "multiple executors" {
				tt.executors["exec-1"] = store.HeartbeatState{
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-5 * time.Second),
				}
				tt.executors["exec-2"] = store.HeartbeatState{
					Status:        types.ExecutorStatusACTIVE,
					LastHeartbeat: now.Add(-10 * time.Second), // oldest
				}
				tt.executors["exec-3"] = store.HeartbeatState{
					Status:        types.ExecutorStatusDRAINING,
					LastHeartbeat: now.Add(-3 * time.Second),
				}
			}

			namespaceState := &store.NamespaceState{
				Executors: tt.executors,
			}

			metricsScope := &metricmocks.Scope{}

			if tt.expectedLag != nil {
				metricsScope.On("UpdateGauge", metrics.ShardDistributorOldestExecutorHeartbeatLag, *tt.expectedLag).Once()
			}

			processor.emitOldestExecutorHeartbeatLag(namespaceState, metricsScope)

			metricsScope.AssertExpectations(t)
		})
	}
}

func TestRunRebalanceTriggeringLoop(t *testing.T) {
	t.Run("no events from subscribe, trigger from ticker", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		updateChan := make(chan int64)
		triggerChan := make(chan string, 1)

		go processor.rebalanceTriggeringLoop(ctx, updateChan, triggerChan)

		// Wait for ticker to be created
		mocks.timeSource.BlockUntil(1)

		// Advance time to trigger the ticker
		mocks.timeSource.Advance(processor.cfg.Period)

		// Expect trigger from periodic reconciliation
		select {
		case reason := <-triggerChan:
			assert.Equal(t, "Periodic reconciliation triggered", reason)
		case <-time.After(time.Second):
			t.Fatal("expected trigger from ticker, but timed out")
		}

		cancel()
	})

	t.Run("events from subscribe before period, trigger from state change", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
		processor.lastAppliedRevision = 0

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		updateChan := make(chan int64, 1)
		triggerChan := make(chan string, 1)

		go processor.rebalanceTriggeringLoop(ctx, updateChan, triggerChan)

		// Wait for ticker to be created
		mocks.timeSource.BlockUntil(1)

		// Send a state change event before the ticker fires
		updateChan <- 1

		// Expect trigger from state change
		select {
		case reason := <-triggerChan:
			assert.Equal(t, "State change detected", reason)
		case <-time.After(time.Second):
			t.Fatal("expected trigger from state change, but timed out")
		}

		cancel()
	})

	t.Run("triggerChan full, multiple subscribe events, loop not stuck", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
		processor.lastAppliedRevision = 0

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use unbuffered channel for updates to ensure they are processed one at a time
		updateChan := make(chan int64)
		triggerChan := make(chan string, 1)

		go processor.rebalanceTriggeringLoop(ctx, updateChan, triggerChan)

		// Wait for ticker to be created
		mocks.timeSource.BlockUntil(1)

		// Don't read from triggerChan yet to keep it full
		// Send multiple state change events
		for i := int64(0); i <= 10; i++ {
			select {
			case updateChan <- i:
			case <-time.After(time.Second):
				// Expect that the loop is not stuck
				t.Fatalf("failed to send update %d, channel blocked", i)
			}
		}

		// Expect trigger from state change
		select {
		case reason := <-triggerChan:
			assert.Equal(t, "State change detected", reason)
		case <-time.After(time.Second):
			t.Fatal("expected trigger from state change, but timed out")
		}

		cancel()
	})

	t.Run("stale revision ignored", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
		processor.lastAppliedRevision = 5

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		updateChan := make(chan int64, 1)
		triggerChan := make(chan string, 1)

		go processor.rebalanceTriggeringLoop(ctx, updateChan, triggerChan)

		// Wait for ticker to be created
		mocks.timeSource.BlockUntil(1)

		// Send stale revision (less than or equal to lastAppliedRevision)
		updateChan <- 3

		// Should not trigger - verify by advancing ticker and getting that trigger instead
		mocks.timeSource.Advance(processor.cfg.Period)

		select {
		case reason := <-triggerChan:
			assert.Equal(t, "Periodic reconciliation triggered", reason)
		case <-time.After(time.Second):
			t.Fatal("expected trigger from ticker")
		}

		cancel()
	})

	t.Run("update channel closed stops loop", func(t *testing.T) {
		mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
		defer mocks.ctrl.Finish()
		processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

		ctx := context.Background()

		updateChan := make(chan int64)
		triggerChan := make(chan string, 1)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			processor.rebalanceTriggeringLoop(ctx, updateChan, triggerChan)
		}()

		// Wait for ticker to be created
		mocks.timeSource.BlockUntil(1)

		// Close update channel
		close(updateChan)

		// Wait for loop to exit
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Loop exited as expected
		case <-time.After(time.Second):
			t.Fatal("loop did not exit after updateChan closed")
		}
	})
}
