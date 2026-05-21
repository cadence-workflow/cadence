package loadbalancer

import (
	"math"

	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

// EmitAssignmentImbalanceMetrics emits load imbalance metrics for the current
// assignment state. Smoothed-load metrics are only emitted for GREEDY mode.
func EmitAssignmentImbalanceMetrics(
	cfg *config.Config,
	namespace string,
	metricsScope metrics.Scope,
	assignments map[string][]string,
	namespaceState *store.NamespaceState,
) {
	if metricsScope == nil || namespaceState == nil {
		return
	}

	emitSmoothedLoadMetrics := cfg.GetLoadBalancingMode(namespace) == types.LoadBalancingModeGREEDY
	reportedLoads := make([]float64, 0, len(assignments))
	var smoothedLoads []float64
	if emitSmoothedLoadMetrics {
		smoothedLoads = make([]float64, 0, len(assignments))
	}

	totalAssigned := 0
	smoothedMissing := 0

	for executorID, shards := range assignments {
		reportedLoad := 0.0
		smoothedLoad := 0.0

		heartbeat, heartbeatOK := namespaceState.Executors[executorID]
		for _, shardID := range shards {
			totalAssigned++

			if !heartbeatOK || heartbeat.ReportedShards == nil {
				continue
			} else if shardReport, ok := heartbeat.ReportedShards[shardID]; ok && shardReport != nil {
				reportedLoad += shardReport.ShardLoad
			}

			if !emitSmoothedLoadMetrics {
				continue
			}
			if namespaceState.ShardStats == nil {
				smoothedMissing++
				continue
			}
			stats, ok := namespaceState.ShardStats[shardID]
			if !ok || stats.LastUpdateTime.IsZero() {
				smoothedMissing++
				continue
			}
			smoothedLoad += stats.SmoothedLoad
		}

		reportedLoads = append(reportedLoads, reportedLoad)
		if emitSmoothedLoadMetrics {
			smoothedLoads = append(smoothedLoads, smoothedLoad)
		}
	}

	metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentLoadMaxOverMean, maxOverMean(reportedLoads))
	metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentLoadCV, coefficientOfVariation(reportedLoads))
	if !emitSmoothedLoadMetrics {
		return
	}

	metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentSmoothedLoadMaxOverMean, maxOverMean(smoothedLoads))
	metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentSmoothedLoadCV, coefficientOfVariation(smoothedLoads))

	if totalAssigned == 0 {
		metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentSmoothedLoadMissingRatio, 0)
		return
	}

	metricsScope.UpdateGauge(metrics.ShardDistributorAssignmentSmoothedLoadMissingRatio, float64(smoothedMissing)/float64(totalAssigned))
}

func maxOverMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	total := 0.0
	maxValue := 0.0
	for _, value := range values {
		total += value
		if value > maxValue {
			maxValue = value
		}
	}
	mean := total / float64(len(values))
	if mean == 0 {
		return 0
	}
	return maxValue / mean
}

func coefficientOfVariation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	total := 0.0
	for _, value := range values {
		total += value
	}
	mean := total / float64(len(values))
	if mean == 0 {
		return 0
	}

	variance := 0.0
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	variance /= float64(len(values))

	return math.Sqrt(variance) / mean
}
