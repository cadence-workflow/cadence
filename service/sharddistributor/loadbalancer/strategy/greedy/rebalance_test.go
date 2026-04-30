package greedy

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const testNamespace = "test-namespace"

func testGreedyConfig() config.LoadBalancingGreedyConfig {
	return config.LoadBalancingGreedyConfig{
		PerShardCooldown: func(namespace string) time.Duration {
			return time.Minute
		},
		MoveBudgetProportion: func(namespace string) float64 {
			return 0.01
		},
		HysteresisUpperBand: func(namespace string) float64 {
			return 1.15
		},
		HysteresisLowerBand: func(namespace string) float64 {
			return 0.90
		},
		SevereImbalanceRatio: func(namespace string) float64 {
			return 1.3
		},
	}
}

// TestLoadBalance_Convergence verifies the balancer moves shards from an overloaded executor to an underloaded one.
func TestLoadBalance_Convergence(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB := "exec-A", "exec-B"
	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	currentAssignments := map[string][]string{
		execA: {},
		execB: {},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := time.Now().UTC()

	for i := range 50 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 3.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execB] = append(currentAssignments[execB], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)
	assert.Less(t, len(currentAssignments[execA]), 50, "Overloaded executor should shed shards")
	assert.Greater(t, len(currentAssignments[execB]), 50, "Underloaded executor should receive shards")
}

// TestLoadBalance_SkipsNonBeneficialHotShard verifies we skip hot shards that would not improve balance.
func TestLoadBalance_SkipsNonBeneficialHotShard(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB := "exec-A", "exec-B"
	now := time.Now().UTC()

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

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)
	assert.True(t, slices.Contains(currentAssignments[execB], "warm"))
	assert.False(t, slices.Contains(currentAssignments[execB], "hot"))
}

// TestLoadBalance_NoMoveNeeded verifies the balancer does nothing when already within hysteresis bands.
func TestLoadBalance_NoMoveNeeded(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB := "exec-A", "exec-B"
	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	currentAssignments := map[string][]string{
		execA: {},
		execB: {},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := time.Now().UTC()

	for i := range 51 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}
	for i := range 49 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execB] = append(currentAssignments[execB], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.Empty(t, moves)
	assert.Len(t, currentAssignments[execA], 51)
	assert.Len(t, currentAssignments[execB], 49)
}

// TestLoadBalance_SevereImbalance_AllowsMoveWithoutDestinations verifies severe imbalance can trigger a relaxed destination set.
func TestLoadBalance_SevereImbalance_AllowsMoveWithoutDestinations(t *testing.T) {
	cfg := testGreedyConfig()

	cfg.HysteresisLowerBand = func(namespace string) float64 {
		return 0.1 // make destinations strict
	}
	cfg.SevereImbalanceRatio = func(namespace string) float64 {
		return 2.0
	}

	execA, execB, execC, execD, execE := "exec-A", "exec-B", "exec-C", "exec-D", "exec-E"
	now := time.Now().UTC()

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
	expectedBudget := computeMoveBudget(len(shardStats), cfg.MoveBudgetProportion(testNamespace))

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)

	assert.Len(t, currentAssignments[execA], initialA-expectedBudget, "execA should shed budgeted shards")
	totalOther := len(currentAssignments[execB]) + len(currentAssignments[execC]) + len(currentAssignments[execD]) + len(currentAssignments[execE])
	assert.Equal(t, initialOther+expectedBudget, totalOther, "Destinations should gain budgeted shards")
}

// TestLoadBalance_NoDestinations_NotSevere verifies we do not relax destinations without severe imbalance.
func TestLoadBalance_NoDestinations_NotSevere(t *testing.T) {
	cfg := testGreedyConfig()

	cfg.HysteresisLowerBand = func(namespace string) float64 {
		return 0.1 // make destinations very strict
	}
	cfg.SevereImbalanceRatio = func(namespace string) float64 {
		return 10.0
	}

	execA, execB := "exec-A", "exec-B"
	now := time.Now().UTC()

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

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.Empty(t, moves)
	assert.Len(t, currentAssignments[execA], 10)
	assert.Len(t, currentAssignments[execB], 1)
}

// TestLoadBalance_BudgetConstraint verifies the balancer respects the move budget per pass.
func TestLoadBalance_BudgetConstraint(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB, execC, execD := "exec-A", "exec-B", "exec-C", "exec-D"

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execC: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execD: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	currentAssignments := map[string][]string{
		execA: {},
		execB: {},
		execC: {},
		execD: {},
	}
	shardStats := make(map[string]store.ShardStatistics)
	now := time.Now().UTC()

	for i := range 50 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execB] = append(currentAssignments[execB], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("C-%d", i)
		assignments[execC].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execC] = append(currentAssignments[execC], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 25 {
		sID := fmt.Sprintf("D-%d", i)
		assignments[execD].AssignedShards[sID] = &types.ShardAssignment{}
		currentAssignments[execD] = append(currentAssignments[execD], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 0.2, LastUpdateTime: now}
	}

	totalShards := len(shardStats)
	expectedBudget := computeMoveBudget(totalShards, cfg.MoveBudgetProportion(testNamespace))
	initialHot := len(assignments[execA].AssignedShards) + len(assignments[execB].AssignedShards) + len(assignments[execC].AssignedShards)
	initialD := len(assignments[execD].AssignedShards)
	expectedHotAfter := initialHot - expectedBudget
	expectedDAfter := initialD + expectedBudget

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execD: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)
	shardsOnA := len(currentAssignments[execA])
	shardsOnB := len(currentAssignments[execB])
	shardsOnC := len(currentAssignments[execC])
	shardsOnD := len(currentAssignments[execD])
	assert.Equal(t, expectedHotAfter, shardsOnA+shardsOnB+shardsOnC, "Hot executors should shed budgeted shards")
	assert.Equal(t, expectedDAfter, shardsOnD, "execD should gain budgeted shards")
}

// TestLoadBalance_MultiMovePerCycle verifies multiple moves can be planned within a single pass up to the budget.
func TestLoadBalance_MultiMovePerCycle(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB := "exec-A", "exec-B"
	now := time.Now().UTC()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: make(map[string]*types.ShardAssignment)},
		execB: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	currentAssignments := map[string][]string{
		execA: {},
		execB: {},
	}
	shardStats := make(map[string]store.ShardStatistics)

	for i := range 100 {
		sID := fmt.Sprintf("A-%d", i)
		assignments[execA].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execA] = append(currentAssignments[execA], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 2.0, LastUpdateTime: now}
	}
	for i := range 50 {
		sID := fmt.Sprintf("B-%d", i)
		assignments[execB].AssignedShards[sID] = &types.ShardAssignment{Status: types.AssignmentStatusREADY}
		currentAssignments[execB] = append(currentAssignments[execB], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 0.1, LastUpdateTime: now}
	}

	totalShards := len(shardStats)
	expectedBudget := computeMoveBudget(totalShards, cfg.MoveBudgetProportion(testNamespace))

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)
	assert.Equal(t, 100-expectedBudget, len(currentAssignments[execA]), "ExecA should shed budgeted shards")
	assert.Equal(t, 50+expectedBudget, len(currentAssignments[execB]), "ExecB should gain budgeted shards")
}

// TestLoadBalance_PerShardCooldownSkipsHotShard verifies a recently moved hot shard is skipped due to cooldown.
func TestLoadBalance_PerShardCooldownSkipsHotShard(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB := "exec-A", "exec-B"
	now := time.Now().UTC()
	cooldown := cfg.PerShardCooldown(testNamespace)
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

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)
	assert.True(t, slices.Contains(currentAssignments[execB], "hot-2"), "eligible hot shard should move")
	assert.False(t, slices.Contains(currentAssignments[execB], "hot-1"), "recently moved shard should not move")
}

// TestLoadBalance_NoDestinations verifies no moves are made when no executor is eligible as a destination.
func TestLoadBalance_NoDestinations(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB, execC, execD, execE := "exec-A", "exec-B", "exec-C", "exec-D", "exec-E"
	now := time.Now().UTC()

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
	currentAssignments := map[string][]string{
		execA: {"s1"},
		execB: {"s2"},
		execC: {"s3"},
		execD: {"s4"},
		execE: {"s5"},
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

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.Empty(t, moves)
	assert.Equal(t, []string{"s1"}, currentAssignments[execA])
	assert.Equal(t, []string{"s2"}, currentAssignments[execB])
	assert.Equal(t, []string{"s3"}, currentAssignments[execC])
	assert.Equal(t, []string{"s4"}, currentAssignments[execD])
	assert.Equal(t, []string{"s5"}, currentAssignments[execE])
}

// TestLoadBalance_ExecutorRemovedFromDestination verifies destinations are removed once they cross the lower band.
func TestLoadBalance_ExecutorRemovedFromDestination(t *testing.T) {
	cfg := testGreedyConfig()

	execA, execB, execC, execF := "exec-A", "exec-B", "exec-C", "exec-F"
	now := time.Now().UTC()

	assignments := map[string]store.AssignedState{
		execA: {AssignedShards: map[string]*types.ShardAssignment{"sa_1": {}, "sa_2": {}}},
		execB: {AssignedShards: map[string]*types.ShardAssignment{"sb_1": {}}},
		execC: {AssignedShards: map[string]*types.ShardAssignment{"sc_1": {}, "sc_2": {}}},
		execF: {AssignedShards: make(map[string]*types.ShardAssignment)},
	}
	currentAssignments := map[string][]string{
		execA: {"sa_1", "sa_2"},
		execB: {"sb_1"},
		execC: {"sc_1", "sc_2"},
		execF: {},
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
		currentAssignments[execF] = append(currentAssignments[execF], sID)
		shardStats[sID] = store.ShardStatistics{SmoothedLoad: 1.0, LastUpdateTime: now}
	}

	namespaceState := &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			execA: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execB: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execC: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
			execF: {Status: types.ExecutorStatusACTIVE, LastHeartbeat: now},
		},
		ShardAssignments: assignments,
		ShardStats:       shardStats,
	}

	moves, err := Rebalance(cfg, testNamespace, namespaceState, currentAssignments, now, metrics.NoopScope)
	require.NoError(t, err)
	require.NotEmpty(t, moves)
	applyMoves(t, currentAssignments, moves)

	shardsOnA := len(currentAssignments[execA])
	shardsOnC := len(currentAssignments[execC])

	// execB starts as the only destination. After receiving one hot shard it exceeds
	// the lower hysteresis band, so it should accept at most one shard this cycle.
	assert.Len(t, currentAssignments[execB], 2, "Destination execB should gain only one shard")

	// One of the hot sources (execA or execC) should lose a shard to execB.
	// With multi-move planning, the remaining hot source may move a shard to the
	// newly underloaded executor, so final counts are non-deterministic between A/C.
	assert.Equal(t, 3, shardsOnA+shardsOnC, "Exactly one shard should move from {A,C} to execB")
	assert.True(t, shardsOnA == 1 || shardsOnC == 1, "Either execA or execC should shed a shard")
	assert.False(t, shardsOnA == 1 && shardsOnC == 1, "Not both execA and execC should shed a shard without receiving one")

	assert.Len(t, currentAssignments[execF], 108, "Filler executor execF should be untouched")
}

func applyMoves(t *testing.T, assignments map[string][]string, moves []plan.Move) {
	t.Helper()

	for _, move := range moves {
		idx := slices.Index(assignments[move.From], move.ShardID)
		require.NotEqual(t, -1, idx, "planned move source should contain shard")

		assignments[move.From] = append(assignments[move.From][:idx], assignments[move.From][idx+1:]...)
		assignments[move.To] = append(assignments[move.To], move.ShardID)
	}
}
