package process

import (
	"context"
	"errors"
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

func setupProcessorTest(t *testing.T) *testDependencies {
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
			config.LeaderElection{
				Process: config.LeaderProcess{
					Period:       time.Second,
					HeartbeatTTL: time.Second,
				},
			},
		),
		cfg: config.Namespace{Name: "test-ns", ShardNum: 2, Type: config.NamespaceTypeFixed},
	}
}

func TestRunAndTerminate(t *testing.T) {
	defer goleak.VerifyNone(t)

	mocks := setupProcessorTest(t)
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
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	state := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE},
		"exec-2": {Status: types.ExecutorStatusACTIVE},
	}
	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: state, GlobalRevision: 1}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, newState *store.NamespaceState, _ store.GuardFunc) error {
			assert.Len(t, newState.ShardAssignments, 2)
			assert.Len(t, newState.ShardAssignments["exec-1"].AssignedShards, 1)
			assert.Len(t, newState.ShardAssignments["exec-2"].AssignedShards, 1)
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), processor.lastAppliedRevision)
}

func TestRebalanceShards_ExecutorRemoved(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE},
		"exec-2": {Status: types.ExecutorStatusDRAINING},
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
		Shards:           nil,
		ShardAssignments: assignments,
		GlobalRevision:   1,
	}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, newState *store.NamespaceState, _ store.GuardFunc) error {
			assert.Len(t, newState.ShardAssignments["exec-1"].AssignedShards, 2)
			assert.Len(t, newState.ShardAssignments["exec-2"].AssignedShards, 0)
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_UpdatesShardStateOnReassign(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	// Initial state: exec-2 is draining, exec-1 is active.
	// Shards "0" and "1" are initially owned by exec-2.
	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE},
		"exec-2": {Status: types.ExecutorStatusDRAINING},
	}
	assignments := map[string]store.AssignedState{
		"exec-2": {
			AssignedShards: map[string]*types.ShardAssignment{
				"0": {Status: types.AssignmentStatusREADY},
				"1": {Status: types.AssignmentStatusREADY},
			},
		},
	}
	// This is the crucial part for the test: the initial state of the Shards map.
	shards := map[string]store.ShardState{
		"0": {ExecutorID: "exec-2", Revision: 101},
		"1": {ExecutorID: "exec-2", Revision: 102},
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:        heartbeats,
		Shards:           shards, // Provide the initial shard state.
		ShardAssignments: assignments,
		GlobalRevision:   2,
	}, nil)

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	// We expect AssignShards to be called with the updated state.
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, newState *store.NamespaceState, _ store.GuardFunc) error {
			// Assert that the assignments were moved to exec-1.
			assert.Len(t, newState.ShardAssignments["exec-1"].AssignedShards, 2)
			assert.Len(t, newState.ShardAssignments["exec-2"].AssignedShards, 0)

			// **Assert that the Shards map is correctly updated.**
			// The ExecutorID should be updated to the new owner.
			assert.Equal(t, "exec-1", newState.Shards["0"].ExecutorID)
			assert.Equal(t, "exec-1", newState.Shards["1"].ExecutorID)

			// The Revision should be preserved from the original state.
			assert.Equal(t, int64(101), newState.Shards["0"].Revision)
			assert.Equal(t, int64(102), newState.Shards["1"].Revision)
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestRebalanceShards_NoActiveExecutors(t *testing.T) {
	mocks := setupProcessorTest(t)
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
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	processor.lastAppliedRevision = 1

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{GlobalRevision: int64(1)}, nil)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestCleanupStaleExecutors(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	now := mocks.timeSource.Now()

	heartbeats := map[string]store.HeartbeatState{
		"exec-active": {LastHeartbeat: now.Unix()},
		"exec-stale":  {LastHeartbeat: now.Add(-2 * time.Second).Unix()},
	}

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{Executors: heartbeats}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().DeleteExecutors(gomock.Any(), mocks.cfg.Name, []string{"exec-stale"}, gomock.Any()).Return(nil)

	processor.cleanupStaleExecutors(context.Background())
}

func TestRebalance_StoreErrors(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	expectedErr := errors.New("store is down")

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(nil, expectedErr)
	err := processor.rebalanceShards(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:      map[string]store.HeartbeatState{"e": {Status: types.ExecutorStatusACTIVE}},
		GlobalRevision: 1,
	}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).Return(expectedErr)
	err = processor.rebalanceShards(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestCleanup_StoreErrors(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)
	expectedErr := errors.New("store is down")

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(nil, expectedErr)
	processor.cleanupStaleExecutors(context.Background())

	mocks.store.EXPECT().GetState(gomock.Any(), mocks.cfg.Name).Return(&store.NamespaceState{
		Executors:      map[string]store.HeartbeatState{"stale": {LastHeartbeat: 0}},
		GlobalRevision: 1,
	}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().DeleteExecutors(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).Return(expectedErr)
	processor.cleanupStaleExecutors(context.Background())
}

func TestRunLoop_SubscriptionError(t *testing.T) {
	mocks := setupProcessorTest(t)
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
	mocks := setupProcessorTest(t)
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

func TestRebalanceShards_NoShardsToReassign(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE},
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
		Shards:           nil,
		ShardAssignments: assignments,
		GlobalRevision:   2,
	}, nil)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), processor.lastAppliedRevision, "Revision should be updated even if no shards were moved")
}

func TestRebalanceShards_WithUnassignedShards(t *testing.T) {
	mocks := setupProcessorTest(t)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	heartbeats := map[string]store.HeartbeatState{
		"exec-1": {Status: types.ExecutorStatusACTIVE},
	}
	// Note: shard "1" is missing from assignments
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
		GlobalRevision:   3,
	}, nil)
	mocks.election.EXPECT().Guard().Return(store.NopGuard())
	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, newState *store.NamespaceState, _ store.GuardFunc) error {
			assert.Len(t, newState.ShardAssignments["exec-1"].AssignedShards, 2, "Both shards should now be assigned to exec-1")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

func TestGetShards_Utility(t *testing.T) {
	// Fixed type
	cfg := config.Namespace{Type: config.NamespaceTypeFixed, ShardNum: 5}
	shards := getShards(cfg)
	assert.Equal(t, []int64{0, 1, 2, 3, 4}, shards)

	// Other type
	cfg = config.Namespace{Type: "other"}
	shards = getShards(cfg)
	assert.Nil(t, shards)
}
