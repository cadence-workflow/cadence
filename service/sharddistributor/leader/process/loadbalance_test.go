package process

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// TestLoadBalance_Convergence verifies the balancer moves shards from an overloaded executor to an underloaded one.
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

// TestLoadBalance_SkipsNonBeneficialHotShard verifies we skip hot shards that would not improve balance.
func TestLoadBalance_SkipsNonBeneficialHotShard(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()

	// ExecA is overloaded, ExecB is underloaded.
	// "hot" is very large, and moving it would not reduce squared imbalance (gap < shard load).
	// "warm" is smaller and should be selected instead because it provides a positive benefit.
	currentAssignments := map[string][]string{
		execA: {"hot", "warm"},
		execB: {"b-1"},
	}
	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: map[string]store.AssignedState{
			execA: {AssignedShards: map[string]*types.ShardAssignment{"hot": {}, "warm": {}}},
			execB: {AssignedShards: map[string]*types.ShardAssignment{"b-1": {}}},
		},
		ShardStats: map[string]store.ShardStatistics{
			"hot":  {SmoothedLoad: 10, LastUpdateTime: now},
			"warm": {SmoothedLoad: 2, LastUpdateTime: now},
			"b-1":  {SmoothedLoad: 3, LastUpdateTime: now},
		},
	}

	changed, err := processor.loadBalance(currentAssignments, namespaceState, map[string]store.ShardState{}, nil)
	require.NoError(t, err)
	require.True(t, changed)
	assert.True(t, slices.Contains(currentAssignments[execB], "warm"))
	assert.False(t, slices.Contains(currentAssignments[execB], "hot"))
}

// TestLoadBalance_NoMoveNeeded verifies the balancer does nothing when already within hysteresis bands.
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

// TestLoadBalance_SevereImbalance_AllowsMoveWithoutDestinations verifies severe imbalance can trigger a relaxed destination set.
func TestLoadBalance_SevereImbalance_AllowsMoveWithoutDestinations(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	processor.cfg.LoadBalance.HysteresisLowerBand = 0.1 // make destinations strict
	processor.cfg.LoadBalance.SevereImbalanceRatio = 2.0

	execA, execB, execC, execD, execE := "exec-A", "exec-B", "exec-C", "exec-D", "exec-E"
	now := mocks.timeSource.Now()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"b-1": {}}},
		execC: {AssignedShards: map[string]*types.ShardAssignment{"c-1": {}}},
		execD: {AssignedShards: map[string]*types.ShardAssignment{"d-1": {}}},
		execE: {AssignedShards: map[string]*types.ShardAssignment{"e-1": {}}},
	}

	currentAssignments := map[string][]string{
		execA: {},
		execB: {"b-1"},
		execC: {"c-1"},
		execD: {"d-1"},
		execE: {"e-1"},
	}

	shardStats := map[string]store.ShardStatistics{
		"b-1": {SmoothedLoad: 50, LastUpdateTime: now},
		"c-1": {SmoothedLoad: 50, LastUpdateTime: now},
		"d-1": {SmoothedLoad: 50, LastUpdateTime: now},
		"e-1": {SmoothedLoad: 50, LastUpdateTime: now},
	}

	// Make execA very overloaded (100 shards * 10 load each = 1000).
	for i := range 100 {
		sID := fmt.Sprintf("a-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 10, LastUpdateTime: now}
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execD: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execE: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	initialA := len(currentAssignments[execA])
	initialOther := len(currentAssignments[execB]) + len(currentAssignments[execC]) + len(currentAssignments[execD]) + len(currentAssignments[execE])
	expectedBudget := computeMoveBudget(len(shardStats), processor.cfg.LoadBalance.MoveBudgetProportion)

	changed, err := processor.loadBalance(currentAssignments, namespaceState, map[string]store.ShardState{}, nil)
	require.NoError(t, err)
	require.True(t, changed)

	assert.Len(t, currentAssignments[execA], initialA-expectedBudget, "execA should shed budgeted shards")
	totalOther := len(currentAssignments[execB]) + len(currentAssignments[execC]) + len(currentAssignments[execD]) + len(currentAssignments[execE])
	assert.Equal(t, initialOther+expectedBudget, totalOther, "Destinations should gain budgeted shards")
}

// TestLoadBalance_NoDestinations_NotSevere verifies we do not relax destinations without severe imbalance.
func TestLoadBalance_NoDestinations_NotSevere(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	processor.cfg.LoadBalance.HysteresisLowerBand = 0.1 // make destinations very strict
	processor.cfg.LoadBalance.SevereImbalanceRatio = 10.0

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"b-1": {}}},
	}
	currentAssignments := map[string][]string{
		execA: {},
		execB: {"b-1"},
	}
	shardStats := map[string]store.ShardStatistics{
		"b-1": {SmoothedLoad: 50, LastUpdateTime: now},
	}
	for i := range 10 {
		sID := fmt.Sprintf("a-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 10, LastUpdateTime: now}
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	changed, err := processor.loadBalance(currentAssignments, namespaceState, map[string]store.ShardState{}, nil)
	require.NoError(t, err)
	require.False(t, changed)
	assert.Len(t, currentAssignments[execA], 10)
	assert.Len(t, currentAssignments[execB], 1)
}

// TestLoadBalance_BudgetConstraint verifies the balancer respects the move budget per pass.
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

	totalShards := len(shardStats)
	expectedBudget := computeMoveBudget(totalShards, processor.cfg.LoadBalance.MoveBudgetProportion)
	initialHot := len(assignments[execA].AssignedShards) + len(assignments[execB].AssignedShards) + len(assignments[execC].AssignedShards)
	initialD := len(assignments[execD].AssignedShards)
	expectedHotAfter := initialHot - expectedBudget
	expectedDAfter := initialD + expectedBudget

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
			assert.Equal(t, expectedHotAfter, shardsOnA+shardsOnB+shardsOnC, "Hot executors should shed budgeted shards")
			assert.Equal(t, expectedDAfter, shardsOnD, "execD should gain budgeted shards")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

// TestLoadBalance_MultiMovePerCycle verifies multiple moves can be planned within a single pass up to the budget.
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
	expectedBudget := computeMoveBudget(totalShards, processor.cfg.LoadBalance.MoveBudgetProportion)

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
			assert.Equal(t, 100-expectedBudget, len(newAssignments[execA].AssignedShards), "ExecA should shed budgeted shards")
			assert.Equal(t, 50+expectedBudget, len(newAssignments[execB].AssignedShards), "ExecB should gain budgeted shards")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

// TestLoadBalance_PerShardCooldownSkipsHotShard verifies a recently moved hot shard is skipped due to cooldown.
func TestLoadBalance_PerShardCooldownSkipsHotShard(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB := "exec-A", "exec-B"
	now := mocks.timeSource.Now()
	cooldown := processor.cfg.LoadBalance.PerShardCooldown
	require.True(t, cooldown > 0, "PerShardCooldown should be configured")
	recentMove := now.Add(-cooldown / 2)

	// ExecA has two hot shards. Hottest was moved recently and should be skipped.
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

	changed, err := processor.loadBalance(currentAssignments, namespaceState, map[string]store.ShardState{}, nil)
	require.NoError(t, err)
	require.True(t, changed)
	assert.True(t, slices.Contains(currentAssignments[execB], "hot-2"), "eligible hot shard should move")
	assert.False(t, slices.Contains(currentAssignments[execB], "hot-1"), "recently moved shard should not move")
}

// TestLoadBalance_NoDestinations verifies no moves are made when no executor is eligible as a destination.
func TestLoadBalance_NoDestinations(t *testing.T) {
	mocks := setupProcessorTest(t, config.NamespaceTypeEphemeral)
	defer mocks.ctrl.Finish()
	processor := mocks.factory.CreateProcessor(mocks.cfg, mocks.store, mocks.election).(*namespaceProcessor)

	execA, execB, execC, execD, execE := "exec-A", "exec-B", "exec-C", "exec-D", "exec-E"
	now := mocks.timeSource.Now()

	// Mean:	104
	// Upper:	119.6
	// Lower:	98.8
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

	// Expect AssignShards to NOT be called because no moves are possible.
	mocks.store.EXPECT().AssignShards(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}

// TestLoadBalance_ExecutorRemovedFromDestination verifies destinations are removed once they cross the lower band.
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
	// Around mean load (within upper and lower bound).
	// Enough shards to make move budget 2.
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
		} else {
			owner = execF
		}
		mocks.store.EXPECT().GetShardOwner(gomock.Any(), mocks.cfg.Name, sID).Return(&store.ShardOwner{ExecutorID: owner}, nil).AnyTimes()
	}

	mocks.election.EXPECT().Guard().Return(store.NopGuard())

	mocks.store.EXPECT().AssignShards(gomock.Any(), mocks.cfg.Name, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request store.AssignShardsRequest, _ store.GuardFunc) error {
			newAssignments := request.NewState.ShardAssignments
			shardsOnA := len(newAssignments[execA].AssignedShards)
			shardsOnC := len(newAssignments[execC].AssignedShards)

			// execB starts as the only destination. After receiving one hot shard it exceeds
			// the lower hysteresis band, so it should accept at most one shard this cycle.
			assert.Len(t, newAssignments[execB].AssignedShards, 2, "Destination execB should gain only one shard")

			// One of the hot sources (execA or execC) should lose a shard to execB.
			// With "multi-move" planning, the remaining hot source may move a shard to the
			// newly underloaded executor, so final counts are non-deterministic between A/C.
			assert.Equal(t, 3, shardsOnA+shardsOnC, "Exactly one shard should move from {A,C} to execB")
			assert.True(t, shardsOnA == 1 || shardsOnC == 1, "Either execA or execC should shed a shard")
			assert.False(t, shardsOnA == 1 && shardsOnC == 1, "Not both execA and execC should shed a shard without receiving one")

			assert.Len(t, newAssignments[execF].AssignedShards, 108, "Filler executor execF should be untouched")
			return nil
		},
	)

	err := processor.rebalanceShards(context.Background())
	require.NoError(t, err)
}
