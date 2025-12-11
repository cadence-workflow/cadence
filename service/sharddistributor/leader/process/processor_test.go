package process

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

type testDependencies struct {
	ctrl       *gomock.Controller
	store      *store.MockStore
	election   *store.MockElection
	timeSource clock.MockedTimeSource
	factory    Factory
	cfg        config.Namespace
}

func setupProcessorTest(t *testing.T, namespaceType string) *testDependencies {
	ctrl := gomock.NewController(t)
	mockedClock := clock.NewMockedTimeSource()
	return &testDependencies{
		ctrl:       ctrl,
		store:      store.NewMockStore(ctrl),
		election:   store.NewMockElection(ctrl),
		timeSource: mockedClock,
		factory: NewProcessorFactory(
			testlogger.New(t),
			metrics.NewNoopMetricsClient(),
			mockedClock,
			config.ShardDistribution{
				Process: config.LeaderProcess{
					Period:       time.Second,
					HeartbeatTTL: time.Second,
				},
			},
		),
		cfg: config.Namespace{Name: "test-ns", ShardNum: 2, Type: namespaceType, Mode: config.MigrationModeONBOARDED},
	}
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

	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusDRAINING},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: int64(1)}, nil)

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

		staleCutoff := now.Add(-11 * time.Second)
		shardStats := map[string]store.ShardStatistics{
			"shard-1": {SmoothedLoad: 1.0, LastUpdateTime: now, LastMoveTime: now},
			"shard-2": {SmoothedLoad: 2.0, LastUpdateTime: now, LastMoveTime: now},
			"shard-3": {SmoothedLoad: 3.0, LastUpdateTime: staleCutoff, LastMoveTime: staleCutoff},
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

func TestRunLoop_MigrationNotOnboarded(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeFixed)
	mocks.cfg.Mode = config.MigrationModeDISTRIBUTEDPASSTHROUGH
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	ctx, cancel := context.WithCancel(context.Background())

	mocks.store.EXPECT().Subscribe(gomock.Any(), mocks.cfg.Name).Return(make(chan int64), nil)
	// We explicitly verify that the state is not queried
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: 0}, nil).Times(0)

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

func TestLoadBalance_Convergence(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	// Setup: ExecA is overloaded (150), ExecB is underloaded (50). Mean is 100.
	// We expect shards to move from A to B.
	execA, execB := "exec-A", "exec-B"

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := mocks.timeSource.Now()

	for i := range 50 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 3.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	for sID := range shardStats {
		var owner string
		if _, ok := assignments[execA].AssignedShards[sID]; ok {
			owner = execA
		} else {
			owner = execB
		}
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, sID).Return(&store.ShardOwner{ExecutorID: owner}, nil).AnyTimes()
	}

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			newAssignments := request.NewState.ShardAssignments
			assert.Less(t, len(newAssignments[execA].AssignedShards), 50, "Overloaded executor should shed shards")
			assert.Greater(t, len(newAssignments[execB].AssignedShards), 50, "Underloaded executor should receive shards")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestLoadBalance_NoMoveNeeded(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := mocks.timeSource.Now()

	for i := range 51 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}
	for i := range 49 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	// Expect AssignShards to NOT be called
	mocks.store.EXPECT().AssignShards(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestLoadBalance_BudgetConstraint(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB, execC, execD := "exec-A", "exec-B", "exec-C", "exec-D"

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execC: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execD: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := mocks.timeSource.Now()

	for i := range 50 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := 0; i < 50; i++ {
		sID := fmt.Sprintf("C-%d", i)
		assignments[execC].AssignedShards[sID] = &types.ShardAssignment{}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := 0; i < 25; i++ {
		sID := fmt.Sprintf("D-%d", i)
		assignments[execD].AssignedShards[sID] = &types.ShardAssignment{}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 0.2, LastUpdateTime: now}
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execD: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	for sID := range shardStats {
		var owner string
		if _, ok := assignments[execA].AssignedShards[sID]; ok {
			owner = execA
		} else if _, ok := assignments[execB].AssignedShards[sID]; ok {
			owner = execB
		} else if _, ok := assignments[execC].AssignedShards[sID]; ok {
			owner = execC
		} else {
			owner = execD
		}
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, sID).Return(&store.ShardOwner{ExecutorID: owner}, nil).AnyTimes()
	}

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			newAssignments := request.NewState.ShardAssignments
			shardsOnA := len(newAssignments[execA].AssignedShards)
			shardsOnB := len(newAssignments[execB].AssignedShards)
			shardsOnC := len(newAssignments[execC].AssignedShards)
			shardsOnD := len(newAssignments[execD].AssignedShards)
			assert.Equal(t, 148, shardsOnA+shardsOnB+shardsOnC, "Exec a, b and c should loose 2 shards")
			assert.Equal(t, 27, shardsOnD, "ExecD should have gained two shard")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

// TestLoadBalance_MultiMovePerCycle tests whether the move budget is applied within a single rebalance cycle.
func TestLoadBalance_MultiMovePerCycle(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	shardStats := make(map[string]store.ShardStatistics)

	for i := range 100 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 0.1, LastUpdateTime: now}
	}

	totalShards := len(shardStats)
	expectedBudget := int(math.Ceil(processor.cfg.MoveBudgetProportion * float64(totalShards)))

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	for sID := range shardStats {
		var owner string
		if _, ok := assignments[execA].AssignedShards[sID]; ok {
			owner = execA
		} else {
			owner = execB
		}
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, sID).Return(&store.ShardOwner{ExecutorID: owner}, nil).AnyTimes()
	}

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			newAssignments := request.NewState.ShardAssignments
			assert.Equal(t, 100-expectedBudget, len(newAssignments[execA].AssignedShards), "ExecA should lose budgeted shards")
			assert.Equal(t, 50+expectedBudget, len(newAssignments[execB].AssignedShards), "ExecB should gain budgeted shards")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

// TestLoadBalance_PerShardCooldownSkipsHotShard verifies that a recently moved hottest shard is skipped,
// and the next hottest eligible shard is moved instead.
func TestLoadBalance_PerShardCooldownSkipsHotShard(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()
	cooldown := processor.cfg.PerShardCooldown
	if cooldown <= 0 {
		cooldown = _defaultPerShardCooldown
		processor.cfg.PerShardCooldown = cooldown
	}
	recentMove := now.Add(-cooldown / 2)

	// ExecA has two hot shards; hottest was moved recently and should be skipped.
	currentAssignments := map[string][]string{
		execA: {"hot-1", "hot-2", "a-1", "a-2", "a-3"},
		execB: {"b-1", "b-2", "b-3", "b-4", "b-5"},
	}
	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: map[string]*types.ShardAssignment{"hot-1": {}, "hot-2": {}, "a-1": {}, "a-2": {}, "a-3": {}}},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"b-1": {}, "b-2": {}, "b-3": {}, "b-4": {}, "b-5": {}}},
	}

	shardStats := map[string]store.ShardStatistics{
		"hot-1": {SmoothedLoad: 10.0, LastUpdateTime: now, LastMoveTime: recentMove},
		"hot-2": {SmoothedLoad: 9.0, LastUpdateTime: now},
		"a-1":   {SmoothedLoad: 1.0, LastUpdateTime: now},
		"a-2":   {SmoothedLoad: 1.0, LastUpdateTime: now},
		"a-3":   {SmoothedLoad: 1.0, LastUpdateTime: now},
		"b-1":   {SmoothedLoad: 0.1, LastUpdateTime: now},
		"b-2":   {SmoothedLoad: 0.1, LastUpdateTime: now},
		"b-3":   {SmoothedLoad: 0.1, LastUpdateTime: now},
		"b-4":   {SmoothedLoad: 0.1, LastUpdateTime: now},
		"b-5":   {SmoothedLoad: 0.1, LastUpdateTime: now},
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	// Use structuralChange=true to bypass global cooldown and isolate per-shard cooldown behavior.
	changed, err := processor.loadBalance(currentAssignments, namespaceState, map[string]store.ShardState{}, true, nil)
	require.NoError(t, err)
	require.True(t, changed)
	assert.True(t, slices.Contains(currentAssignments[execB], "hot-2"), "eligible hot shard should move")
	assert.False(t, slices.Contains(currentAssignments[execB], "hot-1"), "recently moved shard should not move")
}

// TestLoadBalance_GlobalCooldownSkipsLoadOnlyPass verifies that a recent move causes a load-only pass
// to skip balancing entirely (no AssignShards call).
func TestLoadBalance_GlobalCooldownSkipsLoadOnlyPass(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()
	cooldown := processor.cfg.PerShardCooldown
	if cooldown <= 0 {
		cooldown = _defaultPerShardCooldown
		processor.cfg.PerShardCooldown = cooldown
	}
	recentMove := now.Add(-cooldown / 2)

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: map[string]*types.ShardAssignment{"s1": {}, "s2": {}}},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"s3": {}, "s4": {}}},
	}
	shardStats := map[string]store.ShardStatistics{
		"s1": {SmoothedLoad: 10.0, LastUpdateTime: now, LastMoveTime: recentMove},
		"s2": {SmoothedLoad: 9.0, LastUpdateTime: now},
		"s3": {SmoothedLoad: 0.1, LastUpdateTime: now},
		"s4": {SmoothedLoad: 0.1, LastUpdateTime: now},
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	// Load-only pass should be skipped due to global cooldown.
	mocks.store.EXPECT().AssignShards(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestLoadBalance_NoDestinations(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB, execC, execD, execE := "exec-A", "exec-B", "exec-C", "exec-D", "exec-E"
	now := mocks.timeSource.Now()

	// Mean:	104
	// Upper:	119,6
	// Lower	98,8
	shardStats := map[string]store.ShardStatistics{
		"s1": {SmoothedLoad: 120, LastUpdateTime: now},
		"s2": {SmoothedLoad: 100, LastUpdateTime: now},
		"s3": {SmoothedLoad: 100, LastUpdateTime: now},
		"s4": {SmoothedLoad: 100, LastUpdateTime: now},
		"s5": {SmoothedLoad: 100, LastUpdateTime: now},
	}
	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: map[string]*types.ShardAssignment{"s1": {Status: types.AssignmentStatusREADY}}},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"s2": {Status: types.AssignmentStatusREADY}}},
		execC: {AssignedShards: map[string]*types.ShardAssignment{"s3": {Status: types.AssignmentStatusREADY}}},
		execD: {AssignedShards: map[string]*types.ShardAssignment{"s4": {Status: types.AssignmentStatusREADY}}},
		execE: {AssignedShards: map[string]*types.ShardAssignment{"s5": {Status: types.AssignmentStatusREADY}}},
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execD: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execE: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	// Expect AssignShards to NOT be called because no moves are possible
	mocks.store.EXPECT().AssignShards(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestLoadBalance_ExecutorRemovedFromDestination(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB, execC, execF := "exec-A", "exec-B", "exec-C", "exec-F"
	now := mocks.timeSource.Now()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: map[string]*types.ShardAssignment{"sa_1": {}, "sa_2": {}}},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"sb_1": {}}},
		execC: {AssignedShards: map[string]*types.ShardAssignment{"sc_1": {}, "sc_2": {}}},
		execF: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	shardStats := map[string]store.ShardStatistics{
		"sa_1": {SmoothedLoad: 70, LastUpdateTime: now},
		"sa_2": {SmoothedLoad: 70, LastUpdateTime: now},
		"sb_1": {SmoothedLoad: 50, LastUpdateTime: now},
		"sc_1": {SmoothedLoad: 70, LastUpdateTime: now},
		"sc_2": {SmoothedLoad: 70, LastUpdateTime: now},
	}
	// Around mean load (Within upper and lower bound)
	// Enough shards to make move budget 2
	for i := range 108 {
		sID := fmt.Sprintf("sf_%d", i)
		assignments[execF].AssignedShards[sID] = &types.ShardAssignment{}
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execF: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
		GlobalRevision:   10,
	}, nil)

	for sID := range shardStats {
		var owner string
		if _, ok := assignments[execA].AssignedShards[sID]; ok {
			owner = execA
		} else if _, ok := assignments[execB].AssignedShards[sID]; ok {
			owner = execB
		} else if _, ok := assignments[execC].AssignedShards[sID]; ok {
			owner = execC
		} else if _, ok := assignments[execF].AssignedShards[sID]; ok {
			owner = execF
		}
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, sID).Return(&store.ShardOwner{ExecutorID: owner}, nil).AnyTimes()
	}

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	mocks.store.EXPECT().AssignShards(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			newAssignments := request.NewState.ShardAssignments
			shardsOnA := newAssignments[execA].AssignedShards
			shardsOnB := newAssignments[execB].AssignedShards
			shardsOnC := newAssignments[execC].AssignedShards

			assert.Len(t, shardsOnB, 2, "Destination 'execB' should have gained one shard")

			// One of the sources ('A' or 'C') should have lost a shard, but not both.
			lostFromA := len(shardsOnA) == 1
			lostFromC := len(shardsOnC) == 1
			assert.True(t, lostFromA || lostFromC, "A shard should have moved from either execA or execC")
			assert.False(t, lostFromA && lostFromC, "execB should not accept more shards when load exceeds lower bound")

			assert.Len(t, newAssignments[execF].AssignedShards, 108, "Filler executor should be untouched")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}
