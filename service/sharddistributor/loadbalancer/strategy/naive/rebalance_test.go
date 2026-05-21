package naive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	config "github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/loadbalancer/plan"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const testNamespace = "test-namespace"

func testNaiveConfig(maxDeviation float64) config.LoadBalancingNaiveConfig {
	return config.LoadBalancingNaiveConfig{
		MaxDeviation: func(namespace string) float64 {
			return maxDeviation
		},
	}
}

func testNamespaceState(shardLoad map[string]float64) *store.NamespaceState {
	reportedShards := make(map[string]*types.ShardStatusReport, len(shardLoad))
	for shardID, load := range shardLoad {
		reportedShards[shardID] = &types.ShardStatusReport{ShardLoad: load}
	}

	return &store.NamespaceState{
		Executors: map[string]store.HeartbeatState{
			"reporting-executor": {
				Status:         types.ExecutorStatusACTIVE,
				ReportedShards: reportedShards,
			},
		},
	}
}

func TestPlanRebalanceNaiveByReportedLoad(t *testing.T) {
	cases := []struct {
		name                       string
		shardLoad                  map[string]float64
		currentAssignments         map[string][]string
		maxDeviation               float64
		expectedDistributionChange bool
		expectedMoves              []plan.Move
	}{
		{
			name:                       "single executor - no rebalance",
			shardLoad:                  map[string]float64{"shard-1": 10.0},
			currentAssignments:         map[string][]string{"exec-1": {"shard-1"}},
			maxDeviation:               2.0,
			expectedDistributionChange: false,
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
			// Planned move would leave exec-1 at 35.0 and exec-2 at 20.0.
			expectedMoves: []plan.Move{{ShardID: "shard-2", From: "exec-2", To: "exec-1"}},
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
			// Planned move would leave exec-1 at 50.0 and exec-2 at 30.0.
			expectedMoves: []plan.Move{{ShardID: "shard-3", From: "exec-2", To: "exec-1"}},
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
			// Planned move would leave exec-1 at 30.0, exec-2 at 30.0, exec-3 at 40.0, and exec-4 at 40.0.
			expectedMoves: []plan.Move{{ShardID: "shard-5", From: "exec-1", To: "exec-3"}},
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
			// Planned move would leave exec-1 at 10.0, exec-2 at 32.0, exec-3 at 25.0, and exec-4 at 30.0.
			expectedMoves: []plan.Move{{ShardID: "shard-6", From: "exec-3", To: "exec-2"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			originalAssignments := cloneAssignments(tc.currentAssignments)

			moves, err := PlanRebalance(
				testNaiveConfig(tc.maxDeviation),
				testNamespace,
				testNamespaceState(tc.shardLoad),
				tc.currentAssignments,
				log.NewNoop(),
				metrics.NoopScope,
			)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedDistributionChange, len(moves) > 0, "distribution change mismatch")
			assert.Equal(t, tc.expectedMoves, moves, "planned moves mismatch")
			assert.Equal(t, originalAssignments, tc.currentAssignments, "planner should not mutate current assignments")
		})
	}
}

func cloneAssignments(assignments map[string][]string) map[string][]string {
	cloned := make(map[string][]string, len(assignments))
	for executorID, shardIDs := range assignments {
		clonedShards := make([]string, len(shardIDs))
		copy(clonedShards, shardIDs)
		cloned[executorID] = clonedShards
	}
	return cloned
}
