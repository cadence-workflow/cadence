package executions

import (
	"context"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/worker/scanner/executions/common"
	"github.com/uber/cadence/service/worker/scanner/executions/shard"
)

const (
	ScannerConfigActivityName    = "cadence-sys-executions-scanner-config-activity"
	ScannerScanShardActivityName = "cadence-sys-executions-scanner-scan-shard-activity"
	ScannerEmitMetricsActivityName = "cadence-sys-executions-scanner-emit-metrics-activity"
)

type (
	ScannerConfigActivityParams struct {
		Overwrites ScannerWorkflowConfigOverwrites
	}

	ScanShardActivityParams struct {
		ShardID int
		ExecutionsPageSize int
		BlobstoreFlushThreshold int
		InvariantCollections InvariantCollections
	}

	ScannerEmitMetricsActivityParams struct {
		ShardStatusResult ShardStatusResult
		AggregateReportResult AggregateReportResult
	}
)

// ScannerEmitMetricsActivity will emit metrics for a complete run of scanner
func ScannerEmitMetricsActivity(
	activityCtx context.Context,
	params ScannerEmitMetricsActivityParams,
) {
	scope := activityCtx.Value(ScannerContextKey).(ScannerContext).Scope.Tagged(metrics.ActivityTypeTag(ScannerEmitMetricsActivityName))
	shardSuccess := 0
	shardControlFlowFailure := 0
	for _, v := range params.ShardStatusResult {
		switch v {
		case ShardStatusSuccess:
			shardSuccess++
		case ShardStatusControlFlowFailure:
			shardControlFlowFailure++
		}
	}
	scope.UpdateGauge(metrics.CadenceShardSuccessGauge, float64(shardSuccess))
	scope.UpdateGauge(metrics.CadenceShardFailureGauge, float64(shardControlFlowFailure))

	agg := params.AggregateReportResult
	scope.UpdateGauge(metrics.ScannerExecutionsGauge, float64(agg.ExecutionsCount))
	scope.UpdateGauge(metrics.ScannerCorruptedGauge, float64(agg.CorruptedCount))
	scope.UpdateGauge(metrics.ScannerCheckFailedGauge, float64(agg.CheckFailedCount))
	scope.UpdateGauge(metrics.ScannerCorruptedOpenExecutionGauge, float64(agg.CorruptedOpenExecutionCount))
	for k, v := range agg.CorruptionByType {
		scope.Tagged(metrics.InvariantTypeTag(string(k))).UpdateGauge(metrics.ScannerCorruptionByTypeGauge, float64(v))
	}
}

// ScanShardActivity will scan all executions in a shard and check for invariant violations.
func ScanShardActivity(
	activityCtx context.Context,
	params ScanShardActivityParams,
) (*common.ShardScanReport, error) {
	ctx := activityCtx.Value(ScannerContextKey).(ScannerContext)
	resources := ctx.Resource
	scope := ctx.Scope.Tagged(metrics.ActivityTypeTag(ScannerScanShardActivityName))
	sw := scope.StartTimer(metrics.CadenceLatency)
	defer sw.Stop()
	execManager, err := resources.GetExecutionManager(params.ShardID)
	if err != nil {
		scope.IncCounter(metrics.CadenceFailures)
		return nil, err
	}
	var collections []common.InvariantCollection
	if params.InvariantCollections.InvariantCollectionHistory {
		collections = append(collections, common.InvariantCollectionHistory)
	}
	if params.InvariantCollections.InvariantCollectionMutableState {
		collections = append(collections, common.InvariantCollectionMutableState)
	}
	pr := common.NewPersistenceRetryer(execManager, resources.GetHistoryManager())
	scanner := shard.NewScanner(
		params.ShardID,
		pr,
		params.ExecutionsPageSize,
		resources.GetBlobstoreClient(),
		params.BlobstoreFlushThreshold,
		collections)
	report := scanner.Scan()
	if report.Result.ControlFlowFailure != nil {
		scope.IncCounter(metrics.CadenceFailures)
	}
	return &report, nil
}

// ScannerConfigActivity will read dynamic config, apply overwrites and return a resolved config.
func ScannerConfigActivity(
	activityCtx context.Context,
	params ScannerConfigActivityParams,
) (ResolvedScannerWorkflowConfig, error) {
	dc := activityCtx.Value(ScannerContextKey).(ScannerContext).ScannerWorkflowDynamicConfig
	result := ResolvedScannerWorkflowConfig{
		Enabled: dc.Enabled(),
		Concurrency: dc.Concurrency(),
		ExecutionsPageSize: dc.ExecutionsPageSize(),
		BlobstoreFlushThreshold: dc.BlobstoreFlushThreshold(),
		InvariantCollections: InvariantCollections{
			InvariantCollectionMutableState: dc.DynamicConfigInvariantCollections.InvariantCollectionMutableState(),
			InvariantCollectionHistory: dc.DynamicConfigInvariantCollections.InvariantCollectionHistory(),
		},
	}
	overwrites := params.Overwrites
	if overwrites.Enabled != nil {
		result.Enabled = *overwrites.Enabled
	}
	if overwrites.Concurrency != nil {
		result.Concurrency = *overwrites.Concurrency
	}
	if overwrites.ExecutionsPageSize != nil {
		result.ExecutionsPageSize = *overwrites.ExecutionsPageSize
	}
	if overwrites.BlobstoreFlushThreshold != nil {
		result.BlobstoreFlushThreshold = *overwrites.BlobstoreFlushThreshold
	}
	if overwrites.InvariantCollections != nil {
		result.InvariantCollections = *overwrites.InvariantCollections
	}
	return result, nil
}