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
	"errors"

	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
)

const (
	// ShardReportQuery is the query name for the query used to get a single shard's report
	ShardReportQuery = "shard_report"
	// ShardStatusQuery is the query name for the query used to get the status of all shards
	ShardStatusQuery = "shard_status"
	// ShardStatusSummaryQuery is the query name for the query used to get the shard status -> counts map
	ShardStatusSummaryQuery = "shard_status_summary"
	// AggregateReportQuery is the query name for the query used to get the aggregate result of all finished shards
	AggregateReportQuery = "aggregate_report"
	// ShardSizeQuery is the query name for the query used to get the number of executions per shard in sorted order
	ShardSizeQuery = "shard_size"
	// AllResultsQuery returns filenames / paginating data for corruptions and failures in this workflow,
	// for shards which have finished processing.  This works for both scanner and fixer, and the return structures
	// are very similar.
	//
	// This data is also available for a single shard under ShardReportQuery, but using that requires
	// re-querying repeatedly if more than that single shard's data is desired, e.g. for manual
	// troubleshooting purposes.
	AllResultsQuery = "all_results"

	scanShardReportChan = "scanShardReportChan"
)

// ManagerCB is a function which returns invariant manager for scanner.
type ManagerCB func(
	context.Context,
	persistence.Retryer,
	ScanShardActivityParams,
	cache.DomainCache,
) invariant.Manager

// IteratorCB is a function which returns iterator for scanner.
type IteratorCB func(
	context.Context,
	persistence.Retryer,
	ScanShardActivityParams,
) pagination.Iterator

// ScannerWorkflow is a workflow which scans and checks entities in a shard.
type ScannerWorkflow struct {
	Aggregator *ShardScanResultAggregator
	Params     ScannerWorkflowParams
	Name       string
	Shards     []int
}

// ScannerHooks allows provide manager and iterator for different types of scanners.
type ScannerHooks struct {
	Manager          ManagerCB
	Iterator         IteratorCB
	GetScannerConfig func(scanner ScannerContext) CustomScannerConfig
}

// NewScannerWorkflow creates instance of shard scanner
func NewScannerWorkflow(
	ctx workflow.Context,
	name string,
	params ScannerWorkflowParams,
) (*ScannerWorkflow, error) {

	if name == "" {
		return nil, errors.New("workflow name is not provided")
	}

	if err := params.Shards.Validate(); err != nil {
		return nil, err
	}

	shards, minShard, maxShard := params.Shards.Flatten()
	wf := ScannerWorkflow{
		Name:       name,
		Params:     params,
		Aggregator: NewShardScanResultAggregator(shards, minShard, maxShard),
		Shards:     shards,
	}

	for name, fn := range getScanHandlers(wf.Aggregator) {
		if err := workflow.SetQueryHandler(ctx, name, fn); err != nil {
			return nil, err
		}
	}

	return &wf, nil
}

// Start starts a shard scanner workflow.
func (wf *ScannerWorkflow) Start(ctx workflow.Context) error {
	activityCtx := getShortActivityContext(ctx)

	var resolvedConfig ResolvedScannerWorkflowConfig
	if err := workflow.ExecuteActivity(activityCtx, ActivityScannerConfig, ScannerConfigActivityParams{
		Overwrites: wf.Params.ScannerWorkflowConfigOverwrites,
	}).Get(ctx, &resolvedConfig); err != nil {
		return err
	}

	if !resolvedConfig.GenericScannerConfig.Enabled {
		return nil
	}

	shardReportChan := workflow.GetSignalChannel(ctx, scanShardReportChan)
	for i := 0; i < resolvedConfig.GenericScannerConfig.Concurrency; i++ {
		idx := i
		workflow.Go(ctx, func(ctx workflow.Context) {
			batches := getShardBatches(resolvedConfig.GenericScannerConfig.ActivityBatchSize, resolvedConfig.GenericScannerConfig.Concurrency, wf.Shards, idx)
			for _, batch := range batches {
				activityCtx = getLongActivityContext(ctx)
				var reports []ScanReport
				if err := workflow.ExecuteActivity(activityCtx, ActivityScanShard, ScanShardActivityParams{
					Shards:                  batch,
					PageSize:                resolvedConfig.GenericScannerConfig.PageSize,
					BlobstoreFlushThreshold: resolvedConfig.GenericScannerConfig.BlobstoreFlushThreshold,
					ScannerConfig:           resolvedConfig.CustomScannerConfig,
				}).Get(ctx, &reports); err != nil {
					errStr := err.Error()
					shardReportChan.Send(ctx, ScanReportError{
						Reports:  nil,
						ErrorStr: &errStr,
					})
					return
				}
				shardReportChan.Send(ctx, ScanReportError{
					Reports:  reports,
					ErrorStr: nil,
				})
			}
		})
	}

	for i := 0; i < len(wf.Shards); {
		var reportErr ScanReportError

		shardReportChan.Receive(ctx, &reportErr)

		if reportErr.ErrorStr != nil {
			return errors.New(*reportErr.ErrorStr)
		}
		for _, report := range reportErr.Reports {
			wf.Aggregator.AddReport(report)
			i++
		}
	}

	activityCtx = getShortActivityContext(ctx)
	summary := wf.Aggregator.GetStatusSummary()

	return workflow.ExecuteActivity(activityCtx, ActivityScannerEmitMetrics, ScannerEmitMetricsActivityParams{
		ShardSuccessCount:            summary[ShardStatusSuccess],
		ShardControlFlowFailureCount: summary[ShardStatusControlFlowFailure],
		AggregateReportResult:        wf.Aggregator.GetAggregateReport(),
		ShardDistributionStats:       wf.Aggregator.GetShardDistributionStats(),
	}).Get(ctx, nil)

}

func getScanHandlers(aggregator *ShardScanResultAggregator) map[string]interface{} {
	return map[string]interface{}{
		ShardReportQuery: func(shardID int) (*ScanReport, error) {
			return aggregator.GetReport(shardID)
		},
		ShardStatusQuery: func(req PaginatedShardQueryRequest) (*ShardStatusQueryResult, error) {
			return aggregator.GetStatusResult(req)
		},
		ShardStatusSummaryQuery: func() (ShardStatusSummaryResult, error) {
			return aggregator.GetStatusSummary(), nil
		},
		AggregateReportQuery: func() (AggregateScanReportResult, error) {
			return aggregator.GetAggregateReport(), nil
		},
		ShardCorruptKeysQuery: func(req PaginatedShardQueryRequest) (*ShardCorruptKeysQueryResult, error) {
			return aggregator.GetCorruptionKeys(req)
		},
		ShardSizeQuery: func(req ShardSizeQueryRequest) (ShardSizeQueryResult, error) {
			return aggregator.GetShardSizeQueryResult(req)
		},
		AllResultsQuery: func() (map[int]ScanResult, error) {
			return aggregator.GetAllScanResults()
		},
	}
}

func getShardBatches(
	batchSize int,
	concurrency int,
	shards []int,
	workerIdx int,
) [][]int {
	batchIndices := getBatchIndices(batchSize, concurrency, len(shards), workerIdx)
	var result [][]int
	for _, batch := range batchIndices {
		var curr []int
		for _, i := range batch {
			curr = append(curr, shards[i])
		}
		result = append(result, curr)
	}
	return result
}

// NewScannerHooks is used to have per scanner iterator and invariant manager
func NewScannerHooks(manager ManagerCB, iterator IteratorCB, config func(scanner ScannerContext) CustomScannerConfig) (*ScannerHooks, error) {
	if manager == nil || iterator == nil || config == nil {
		return nil, errors.New("all scanner hooks args are required")
	}

	return &ScannerHooks{
		Manager:          manager,
		Iterator:         iterator,
		GetScannerConfig: config,
	}, nil
}
