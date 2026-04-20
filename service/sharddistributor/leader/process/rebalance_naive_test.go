package process

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/metrics"
	config "github.com/uber/cadence/service/sharddistributor/config"
)

func TestRebalanceNaiveByReportedLoad(t *testing.T) {
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

			distributionChanged := processor.rebalanceNaiveByReportedLoad(tc.shardLoad, tc.currentAssignments, metrics.NoopScope)

			assert.Equal(t, tc.expectedDistributionChange, distributionChanged, "distribution change mismatch")
			assert.Equal(t, tc.expectedAssignments, tc.currentAssignments, "final assignments mismatch")
		})
	}
}
