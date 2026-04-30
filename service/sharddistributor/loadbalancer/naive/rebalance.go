package naive

import (
	"fmt"
	"math"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// PlanRebalance updates currentAssignments with planned shard moves and returns whether the plan changed.
func PlanRebalance(
	cfg config.LoadBalancingNaiveConfig,
	namespace string,
	state *store.NamespaceState,
	currentAssignments map[string][]string,
	logger log.Logger,
	metricsScope metrics.Scope,
) (bool, error) {
	shardLoad := calcShardLoad(state)

	// no rebalance if there are no more than 1 executor
	if len(currentAssignments) < 2 {
		return false, nil
	}

	var (
		hottestExecutorLoad = float64(0)
		hottestExecutorID   = ""

		hottestShardID   = ""
		hottestShardLoad = float64(0)

		coldestExecutorLoad = math.MaxFloat64
		coldestExecutorID   = ""
	)

	// finding loads of hottest, coldest executors and hottest shard
	executorLoad := make(map[string]float64)
	for executorID, shardIDs := range currentAssignments {
		for _, shardID := range shardIDs {
			executorLoad[executorID] += shardLoad[shardID]
		}

		if executorLoad[executorID] <= coldestExecutorLoad {
			coldestExecutorLoad = executorLoad[executorID]
			coldestExecutorID = executorID
		}

		if executorLoad[executorID] >= hottestExecutorLoad {
			hottestExecutorLoad = executorLoad[executorID]
			hottestExecutorID = executorID

			var maxShardLoad = float64(0)
			for _, shardID := range shardIDs {
				if shardLoad[shardID] >= maxShardLoad {
					hottestShardID = shardID
					maxShardLoad = shardLoad[shardID]
				}
			}
			hottestShardLoad = maxShardLoad
		}
	}

	// no rebalance if a deviation between coldest and hottest executors less than maxDeviation
	if hottestExecutorLoad/coldestExecutorLoad < cfg.MaxDeviation(namespace) {
		return false, nil
	}

	// no rebalance if coldest executor becomes a hottest
	if coldestExecutorLoad+hottestShardLoad >= hottestExecutorLoad {
		return false, nil
	}

	logger.Info("Load-based shard move",
		tag.ShardKey(hottestShardID),
		tag.ShardExecutor(hottestExecutorID),
		tag.Dynamic("destination_executor", coldestExecutorID),
		tag.ShardLoad(fmt.Sprintf("%f", hottestShardLoad)),
		tag.Dynamic("hottest_executor_load", hottestExecutorLoad),
		tag.Dynamic("coldest_executor_load", coldestExecutorLoad),
		tag.Dynamic("load_ratio", hottestExecutorLoad/coldestExecutorLoad),
		tag.Dynamic("hottest_executor_shard_count", len(currentAssignments[hottestExecutorID])),
		tag.Dynamic("coldest_executor_shard_count", len(currentAssignments[coldestExecutorID])),
	)
	metricsScope.AddCounter(metrics.ShardDistributorAssignLoopLoadBasedMoves, 1)
	metricsScope.UpdateGauge(metrics.ShardDistributorAssignLoopMovedShardLoad, hottestShardLoad)

	// remove the hottest Shard from the hottest executor
	// put it to the coldest executor
	for i, shardID := range currentAssignments[hottestExecutorID] {
		if shardID == hottestShardID {
			currentAssignments[hottestExecutorID] = append(currentAssignments[hottestExecutorID][:i], currentAssignments[hottestExecutorID][i+1:]...)
		}
	}
	currentAssignments[coldestExecutorID] = append(currentAssignments[coldestExecutorID], hottestShardID)

	return true, nil
}

// calcShardLoad returns a map of shardID to its load based on the latest reported shard loads from executors
func calcShardLoad(namespaceState *store.NamespaceState) map[string]float64 {
	shardLoad := make(map[string]float64)
	for _, state := range namespaceState.Executors {
		for shardID, report := range state.ReportedShards {
			shardLoad[shardID] = report.ShardLoad
		}
	}
	return shardLoad
}
