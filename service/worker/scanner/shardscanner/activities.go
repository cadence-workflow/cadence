// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package shardscanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/activity"

	c "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/store"
)

const (
	// ActivityScannerEmitMetrics is the activity name for scannerEmitMetricsActivity
	ActivityScannerEmitMetrics = "cadence-sys-shardscanner-emit-metrics-activity"
	// ActivityScannerConfig is the activity name scannerConfigActivity
	ActivityScannerConfig = "cadence-sys-shardscanner-config-activity"
	// ActivityFixerConfig is the activity name fixerConfigActivity
	ActivityFixerConfig = "cadence-sys-shardscanner-fixer-config-activity"
	// ActivityScanShard is the activity name for scanShardActivity
	ActivityScanShard = "cadence-sys-shardscanner-scanshard-activity"
	// ActivityFixerCorruptedKeys is the activity name for fixerCorruptedKeysActivity
	ActivityFixerCorruptedKeys = "cadence-sys-shardscanner-corruptedkeys-activity"
	// ActivityFixShard is the activity name for fixShardActivity
	ActivityFixShard = "cadence-sys-shardscanner-fixshard-activity"
	// ShardCorruptKeysQuery is the query name for the query used to get all completed shards with at least one corruption
	ShardCorruptKeysQuery = "shard_corrupt_keys"
)

// scannerConfigActivity will read dynamic config, apply overwrites and return a resolved config.
func scannerConfigActivity(
	activityCtx context.Context,
	params ScannerConfigActivityParams,
) (ResolvedScannerWorkflowConfig, error) {
	ctx, err := GetScannerContext(activityCtx)
	if err != nil {
		return ResolvedScannerWorkflowConfig{}, err
	}
	dc := ctx.Config.DynamicParams

	result := ResolvedScannerWorkflowConfig{
		GenericScannerConfig: GenericScannerConfig{
			Enabled:                 dc.ScannerEnabled(),
			Concurrency:             dc.Concurrency(),
			PageSize:                dc.PageSize(),
			BlobstoreFlushThreshold: dc.BlobstoreFlushThreshold(),
			ActivityBatchSize:       dc.ActivityBatchSize(),
		},
	}

	if ctx.Hooks != nil && ctx.Hooks.GetScannerConfig != nil {
		result.CustomScannerConfig = ctx.Hooks.GetScannerConfig(ctx)
	}

	overwrites := params.Overwrites.GenericScannerConfig
	if overwrites.Enabled != nil {
		result.GenericScannerConfig.Enabled = *overwrites.Enabled
	}
	if overwrites.Concurrency != nil {
		result.GenericScannerConfig.Concurrency = *overwrites.Concurrency
	}
	if overwrites.PageSize != nil {
		result.GenericScannerConfig.PageSize = *overwrites.PageSize
	}
	if overwrites.BlobstoreFlushThreshold != nil {
		result.GenericScannerConfig.BlobstoreFlushThreshold = *overwrites.BlobstoreFlushThreshold
	}

	if overwrites.ActivityBatchSize != nil {
		result.GenericScannerConfig.ActivityBatchSize = *overwrites.ActivityBatchSize
	}

	if params.Overwrites.CustomScannerConfig != nil {
		result.CustomScannerConfig = *params.Overwrites.CustomScannerConfig
	}

	return result, nil
}

// scanShardActivity will scan a collection of shards for invariant violations.
func scanShardActivity(
	activityCtx context.Context,
	params ScanShardActivityParams,
) ([]ScanReport, error) {
	heartbeatDetails := ScanShardHeartbeatDetails{
		LastShardIndexHandled: -1,
		Reports:               nil,
	}
	ctx, err := GetScannerContext(activityCtx)
	if err != nil {
		return nil, err
	}

	if activity.HasHeartbeatDetails(activityCtx) {
		if err := activity.GetHeartbeatDetails(activityCtx, &heartbeatDetails); err != nil {
			ctx.Logger.Error("getting heartbeat details", tag.Error(err))
			return nil, err
		}
	}
	for i := heartbeatDetails.LastShardIndexHandled + 1; i < len(params.Shards); i++ {
		currentShardID := params.Shards[i]
		shardReport, err := scanShard(activityCtx, params, currentShardID, heartbeatDetails)
		if err != nil {
			ctx.Logger.Error("scanning shard", tag.Error(err))
			return nil, err
		}
		heartbeatDetails = ScanShardHeartbeatDetails{
			LastShardIndexHandled: i,
			Reports:               append(heartbeatDetails.Reports, *shardReport),
		}
	}
	return heartbeatDetails.Reports, nil
}

func scanShard(
	activityCtx context.Context,
	params ScanShardActivityParams,
	shardID int,
	heartbeatDetails ScanShardHeartbeatDetails,
) (*ScanReport, error) {
	ctx, err := GetScannerContext(activityCtx)
	if err != nil {
		return nil, err
	}
	info := activity.GetInfo(activityCtx)

	scope := ctx.Scope.Tagged(
		metrics.ActivityTypeTag(ActivityScanShard),
		metrics.WorkflowTypeTag(info.WorkflowType.Name),
		metrics.DomainTag(constants.SystemLocalDomainName),
	)
	sw := scope.StartTimer(metrics.CadenceLatency)
	defer sw.Stop()

	if ctx.Hooks == nil {
		return nil, cadence.NewCustomError(ErrMissingHooks)
	}

	resources := ctx.Resource
	execManager, err := resources.GetExecutionManager(shardID)
	if err != nil {
		scope.IncCounter(metrics.CadenceFailures)
		return nil, err
	}

	pr := persistence.NewPersistenceRetryer(execManager, resources.GetHistoryManager(), c.CreatePersistenceRetryPolicy())

	scanner := NewScanner(
		shardID,
		ctx.Hooks.Iterator(activityCtx, pr, params),
		resources.GetBlobstoreClient(),
		params.BlobstoreFlushThreshold,
		ctx.Hooks.Manager(activityCtx, pr, params, resources.GetDomainCache()),
		func() { activity.RecordHeartbeat(activityCtx, heartbeatDetails) },
		scope,
		resources.GetDomainCache(),
	)
	report := scanner.Scan(activityCtx)
	if report.Result.ControlFlowFailure != nil {
		scope.IncCounter(metrics.CadenceFailures)
	}
	return &report, nil
}

// fixerCorruptedKeysActivity will fetch the keys of blobs from shards with corruptions from a completed scan workflow.
// If scan workflow is not closed or if query fails activity will return an error.
// Accepts as input the shard to start query at and returns a next page token, therefore this activity can
// be used to do pagination.
func fixerCorruptedKeysActivity(
	activityCtx context.Context,
	params FixerCorruptedKeysActivityParams,
) (*FixerCorruptedKeysActivityResult, error) {
	ctx, err := GetFixerContext(activityCtx)
	if err != nil {
		return nil, err
	}

	client := ctx.Resource.GetSDKClient()
	if params.ScannerWorkflowRunID == "" {
		listResp, err := client.ListClosedWorkflowExecutions(activityCtx, &shared.ListClosedWorkflowExecutionsRequest{
			Domain:          c.StringPtr(constants.SystemLocalDomainName),
			MaximumPageSize: c.Int32Ptr(10),
			NextPageToken:   nil,
			StartTimeFilter: &shared.StartTimeFilter{
				EarliestTime: c.Int64Ptr(0),
				LatestTime:   c.Int64Ptr(time.Now().UnixNano()),
			},
			ExecutionFilter: &shared.WorkflowExecutionFilter{
				WorkflowId: c.StringPtr(params.ScannerWorkflowWorkflowID),
			},
		})
		if err != nil {
			return nil, err
		}
		if len(listResp.Executions) > 10 {
			return nil, errors.New("got unexpected number of executions back from list")
		}
		// ListClosedWorkflowExecutions API doesn't support querying by workflow ID and status filter at the same time,
		// and we want to avoid using a scan result with Terminated state.
		for _, executionInfo := range listResp.Executions {
			if *executionInfo.CloseStatus == shared.WorkflowExecutionCloseStatusContinuedAsNew {
				params.ScannerWorkflowRunID = *executionInfo.Execution.RunId
				break
			}
		}
		if len(params.ScannerWorkflowRunID) == 0 {
			return nil, errors.New("failed to find a recent scanner workflow execution with ContinuedAsNew status")
		}
	}

	descResp, err := client.DescribeWorkflowExecution(activityCtx, &shared.DescribeWorkflowExecutionRequest{
		Domain: c.StringPtr(constants.SystemLocalDomainName),
		Execution: &shared.WorkflowExecution{
			WorkflowId: c.StringPtr(params.ScannerWorkflowWorkflowID),
			RunId:      c.StringPtr(params.ScannerWorkflowRunID),
		},
	})
	if err != nil {
		return nil, err
	}
	if descResp.WorkflowExecutionInfo.CloseStatus == nil {
		return nil, cadence.NewCustomError(ErrScanWorkflowNotClosed)
	}
	queryArgs := PaginatedShardQueryRequest{
		StartingShardID: params.StartingShardID,
	}
	queryArgsBytes, err := json.Marshal(queryArgs)
	if err != nil {
		return nil, cadence.NewCustomError(ErrSerialization)
	}
	queryResp, err := client.QueryWorkflow(activityCtx, &shared.QueryWorkflowRequest{
		Domain: c.StringPtr(constants.SystemLocalDomainName),
		Execution: &shared.WorkflowExecution{
			WorkflowId: c.StringPtr(params.ScannerWorkflowWorkflowID),
			RunId:      c.StringPtr(params.ScannerWorkflowRunID),
		},
		Query: &shared.WorkflowQuery{
			QueryType: c.StringPtr(ShardCorruptKeysQuery),
			QueryArgs: queryArgsBytes,
		},
	})
	if err != nil {
		return nil, err
	}
	queryResult := &ShardCorruptKeysQueryResult{}
	if err := json.Unmarshal(queryResp.QueryResult, &queryResult); err != nil {
		return nil, cadence.NewCustomError(ErrSerialization)
	}
	var corrupted []CorruptedKeysEntry
	var minShardID *int
	var maxShardID *int
	for sid, keys := range queryResult.Result {
		if minShardID == nil || *minShardID > sid {
			minShardID = c.IntPtr(sid)
		}
		if maxShardID == nil || *maxShardID < sid {
			maxShardID = c.IntPtr(sid)
		}
		corrupted = append(corrupted, CorruptedKeysEntry{
			ShardID:       sid,
			CorruptedKeys: keys,
		})
	}
	return &FixerCorruptedKeysActivityResult{
		CorruptedKeys:             corrupted,
		MinShard:                  minShardID,
		MaxShard:                  maxShardID,
		ShardQueryPaginationToken: queryResult.ShardQueryPaginationToken,
	}, nil
}

type (
	FixShardConfigParams struct {
		// intentionally empty, no args needed currently.  just reserving arg space for future needs.
	}
	FixShardConfigResults struct {
		EnabledInvariants CustomScannerConfig
	}
)

// fixerConfigActivity returns a list of all enabled invariants for this fixer.
// The type of the workflow determines the type of the fixer (concrete, current, etc).
//
// It essentially mirrors scannerConfigActivity, but does not try to merge into a common structure.
func fixerConfigActivity(activityCtx context.Context, params FixShardConfigParams) (*FixShardConfigResults, error) {
	ctx, err := GetFixerContext(activityCtx)
	if err != nil {
		return nil, err
	}

	cfg := ctx.Hooks.GetFixerConfig(ctx)
	if len(cfg) == 0 {
		// sanity check for new code.  historically this field did not exist, now it is required to be populated.
		return nil, fmt.Errorf(`invalid empty fixer config, you must explicitly specify "true" or "false" for all relevant invariants`)
	}

	return &FixShardConfigResults{
		EnabledInvariants: cfg,
	}, nil
}

// fixShardActivity will fix a collection of shards.
func fixShardActivity(
	activityCtx context.Context,
	params FixShardActivityParams,
) ([]FixReport, error) {
	ctx, err := GetFixerContext(activityCtx)
	if err != nil {
		return nil, err
	}

	heartbeatDetails := FixShardHeartbeatDetails{
		LastShardIndexHandled: -1,
		Reports:               nil,
	}
	if activity.HasHeartbeatDetails(activityCtx) {
		if err := activity.GetHeartbeatDetails(activityCtx, &heartbeatDetails); err != nil {
			ctx.Logger.Error("getting heartbeat details", tag.Error(err))
			return nil, err
		}
	}
	for i := heartbeatDetails.LastShardIndexHandled + 1; i < len(params.CorruptedKeysEntries); i++ {
		currentShardID := params.CorruptedKeysEntries[i].ShardID
		currentKeys := params.CorruptedKeysEntries[i].CorruptedKeys
		shardReport, err := fixShard(activityCtx, params, currentShardID, currentKeys, heartbeatDetails)
		if err != nil {
			ctx.Logger.Error("fixing shard", tag.Error(err))
			return nil, err
		}
		heartbeatDetails = FixShardHeartbeatDetails{
			LastShardIndexHandled: i,
			Reports:               append(heartbeatDetails.Reports, *shardReport),
		}
	}
	return heartbeatDetails.Reports, nil
}

func fixShard(
	activityCtx context.Context,
	params FixShardActivityParams,
	shardID int,
	corruptedKeys store.Keys,
	heartbeatDetails FixShardHeartbeatDetails,
) (*FixReport, error) {
	ctx, err := GetFixerContext(activityCtx)
	if err != nil {
		return nil, err
	}
	resource := ctx.Resource
	info := activity.GetInfo(activityCtx)
	scope := ctx.Scope.Tagged(
		metrics.ActivityTypeTag(ActivityFixShard),
		metrics.WorkflowTypeTag(info.WorkflowType.Name),
		metrics.DomainTag(constants.SystemLocalDomainName),
	)
	sw := scope.StartTimer(metrics.CadenceLatency)
	defer sw.Stop()

	if ctx.Hooks == nil {
		return nil, cadence.NewCustomError(ErrMissingHooks)
	}

	execManager, err := resource.GetExecutionManager(shardID)
	if err != nil {
		scope.IncCounter(metrics.CadenceFailures)
		return nil, err
	}

	pr := persistence.NewPersistenceRetryer(execManager, resource.GetHistoryManager(), c.CreatePersistenceRetryPolicy())

	fixer := NewFixer(
		activityCtx,
		shardID,
		ctx.Hooks.InvariantManager(activityCtx, pr, params, resource.GetDomainCache()),
		ctx.Hooks.Iterator(activityCtx, resource.GetBlobstoreClient(), corruptedKeys, params),
		resource.GetBlobstoreClient(),
		params.ResolvedFixerWorkflowConfig.BlobstoreFlushThreshold,
		func() { activity.RecordHeartbeat(activityCtx, heartbeatDetails) },
		resource.GetDomainCache(),
		ctx.Config.DynamicParams.AllowDomain,
		scope,
	)
	report := fixer.Fix()
	if report.Result.ControlFlowFailure != nil {
		scope.IncCounter(metrics.CadenceFailures)
	}
	return &report, nil
}

// scannerEmitMetricsActivity will emit metrics for a complete run of ShardScanner
func scannerEmitMetricsActivity(
	activityCtx context.Context,
	params ScannerEmitMetricsActivityParams,
) error {
	ctx, err := GetScannerContext(activityCtx)
	if err != nil {
		return err
	}
	info := activity.GetInfo(activityCtx)
	scope := ctx.Scope.Tagged(
		metrics.ActivityTypeTag(ActivityScannerEmitMetrics),
		metrics.WorkflowTypeTag(info.WorkflowType.Name),
		metrics.DomainTag(constants.SystemLocalDomainName),
	)
	scope.UpdateGauge(metrics.CadenceShardSuccessGauge, float64(params.ShardSuccessCount))
	scope.UpdateGauge(metrics.CadenceShardFailureGauge, float64(params.ShardControlFlowFailureCount))

	agg := params.AggregateReportResult
	scope.UpdateGauge(metrics.ScannerExecutionsGauge, float64(agg.EntitiesCount))
	scope.UpdateGauge(metrics.ScannerCorruptedGauge, float64(agg.CorruptedCount))
	scope.UpdateGauge(metrics.ScannerCheckFailedGauge, float64(agg.CheckFailedCount))
	for k, v := range agg.CorruptionByType {
		scope.Tagged(metrics.InvariantTypeTag(string(k))).UpdateGauge(metrics.ScannerCorruptionByTypeGauge, float64(v))
	}
	shardStats := params.ShardDistributionStats
	scope.UpdateGauge(metrics.ScannerShardSizeMaxGauge, float64(shardStats.Max))
	scope.UpdateGauge(metrics.ScannerShardSizeMedianGauge, float64(shardStats.Median))
	scope.UpdateGauge(metrics.ScannerShardSizeMinGauge, float64(shardStats.Min))
	scope.UpdateGauge(metrics.ScannerShardSizeNinetyGauge, float64(shardStats.P90))
	scope.UpdateGauge(metrics.ScannerShardSizeSeventyFiveGauge, float64(shardStats.P75))
	scope.UpdateGauge(metrics.ScannerShardSizeTwentyFiveGauge, float64(shardStats.P25))
	scope.UpdateGauge(metrics.ScannerShardSizeTenGauge, float64(shardStats.P10))
	return nil
}
