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
	"fmt"
	"sort"

	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
)

const (

	// ShardStatusSuccess indicates the scan on the shard ran successfully
	ShardStatusSuccess ShardStatus = "success"
	// ShardStatusControlFlowFailure indicates the scan on the shard failed
	ShardStatusControlFlowFailure ShardStatus = "control_flow_failure"
	// ShardStatusRunning indicates the shard has not completed yet
	ShardStatusRunning ShardStatus = "running"

	maxShardQueryResult = 1000
)

type (
	// ShardStatus is the type which indicates the status of a shard scan.
	ShardStatus string

	// ShardStatusResult indicates the status for all shards
	ShardStatusResult map[int]ShardStatus

	// ShardStatusSummaryResult indicates the counts of shards in each status
	ShardStatusSummaryResult map[ShardStatus]int

	// AggregateScanReportResult indicates the result of summing together all
	// shard reports which have finished scan.
	AggregateScanReportResult ScanStats

	// AggregateFixReportResult indicates the result of summing together all
	// shard reports that have finished for fix.
	AggregateFixReportResult FixStats

	// ShardCorruptKeysResult is a map of all shards which have finished scan successfully and have at least one corruption
	ShardCorruptKeysResult map[int]store.Keys

	// ScanReportError is a type that is used to send either error or report on a channel.
	// Exactly one of Report and ErrorStr should be non-nil.
	ScanReportError struct {
		Reports  []ScanReport
		ErrorStr *string
	}

	// FixerCorruptedKeysActivityResult is the result of fixerCorruptedKeysActivity
	FixerCorruptedKeysActivityResult struct {
		CorruptedKeys             []CorruptedKeysEntry
		MinShard                  *int
		MaxShard                  *int
		ShardQueryPaginationToken ShardQueryPaginationToken
	}

	// CorruptedKeysEntry is a pair of shardID and corrupted keys
	CorruptedKeysEntry struct {
		ShardID       int
		CorruptedKeys store.Keys
	}

	// ScanShardHeartbeatDetails is the heartbeat details for scan shard
	ScanShardHeartbeatDetails struct {
		LastShardIndexHandled int
		Reports               []ScanReport
	}

	// FixShardHeartbeatDetails is the heartbeat details for the fix shard
	FixShardHeartbeatDetails struct {
		LastShardIndexHandled int
		Reports               []FixReport
	}

	// FixReportError is a type that is used to send either error or report on a channel.
	// Exactly one of Report and ErrorStr should be non-nil.
	FixReportError struct {
		Reports  []FixReport
		ErrorStr *string
	}

	// ShardSizeQueryRequest is the request used for ShardSizeQuery.
	// The following must be true: 0 <= StartIndex < EndIndex <= len(shards successfully finished)
	// The following must be true: EndIndex - StartIndex <= maxShardQueryResult.
	// StartIndex is inclusive, EndIndex is exclusive.
	ShardSizeQueryRequest struct {
		StartIndex int
		EndIndex   int
	}

	// PaginatedShardQueryRequest is the request used for queries which return results over all shards
	PaginatedShardQueryRequest struct {
		// StartingShardID is the first shard to start iteration from.
		// Setting to nil will start iteration from the beginning of the shards.
		StartingShardID *int
		// LimitShards indicates the maximum number of results that can be returned.
		// If nil or larger than allowed maximum, will default to maximum allowed.
		LimitShards *int
	}

	// ShardQueryPaginationToken is used to return information used to make the next query
	ShardQueryPaginationToken struct {
		// NextShardID is one greater than the highest shard returned in the current query.
		// NextShardID is nil if IsDone is true.
		// It is possible to get NextShardID != nil and on the next call to get an empty result with IsDone = true.
		NextShardID *int
		IsDone      bool
	}

	// ShardStatusQueryResult is the query result for ShardStatusQuery
	ShardStatusQueryResult struct {
		Result                    ShardStatusResult
		ShardQueryPaginationToken ShardQueryPaginationToken
	}

	// ShardCorruptKeysQueryResult is the query result for ShardCorruptKeysQuery
	ShardCorruptKeysQueryResult struct {
		Result                    ShardCorruptKeysResult
		ShardQueryPaginationToken ShardQueryPaginationToken
	}

	// ShardSizeQueryResult is the result from ShardSizeQuery.
	// Contains sorted list of shards, sorted by the number of executions per shard.
	ShardSizeQueryResult []ShardSizeTuple

	// ShardSizeTuple indicates the size and sorted index of a single shard
	ShardSizeTuple struct {
		ShardID       int
		EntitiesCount int64
	}
	// ShardDistributionStats contains stats on the distribution of executions in shards.
	// It is used by the ScannerEmitMetricsActivityParams.
	ShardDistributionStats struct {
		Max    int64
		Median int64
		Min    int64
		P90    int64
		P75    int64
		P25    int64
		P10    int64
	}

	// ShardFixResultAggregator is used to keep aggregated fix metrics
	ShardFixResultAggregator struct {
		minShard int
		maxShard int

		reports       map[int]FixReport
		domainStats   map[string]*FixStats
		status        ShardStatusResult
		statusSummary ShardStatusSummaryResult
		aggregation   AggregateFixReportResult
	}

	// ShardScanResultAggregator is used to keep aggregated scan metrics
	ShardScanResultAggregator struct {
		minShard int
		maxShard int

		reports        map[int]ScanReport
		domainStats    map[string]*ScanStats
		status         ShardStatusResult
		statusSummary  ShardStatusSummaryResult
		aggregation    AggregateScanReportResult
		shardSizes     ShardSizeQueryResult
		corruptionKeys map[int]store.Keys
	}

	// DomainReportQueryRequest is the request used for queries which return stats broken by domains
	DomainReportQueryRequest struct {
		// DomainID specifies a single domain for which the stats are requested
		// Setting to nil indicates stats for all domains are requested
		DomainID *string
	}

	// DomainReportQueryResult is the query result for DomainReportQuery in the scanner workflow
	DomainScanReportQueryResult struct {
		Reports []DomainScanStats
	}

	// DomainFixReportQueryResult is the query result for DomainReportQuery in the fixer workflow
	DomainFixReportQueryResult struct {
		Reports []DomainFixStats
	}
)

// NewShardFixResultAggregator returns an instance of ShardFixResultAggregator
func NewShardFixResultAggregator(
	corruptKeys []CorruptedKeysEntry,
	minShard int,
	maxShard int,
) *ShardFixResultAggregator {
	status := make(map[int]ShardStatus, len(corruptKeys))
	for _, s := range corruptKeys {
		status[s.ShardID] = ShardStatusRunning
	}
	statusSummary := map[ShardStatus]int{
		ShardStatusRunning:            len(corruptKeys),
		ShardStatusSuccess:            0,
		ShardStatusControlFlowFailure: 0,
	}
	return &ShardFixResultAggregator{
		minShard: minShard,
		maxShard: maxShard,

		reports:       make(map[int]FixReport),
		domainStats:   make(map[string]*FixStats),
		status:        status,
		statusSummary: statusSummary,
		aggregation:   AggregateFixReportResult{},
	}
}

// GetStatusSummary returns scan status summary.
func (a *ShardScanResultAggregator) GetStatusSummary() ShardStatusSummaryResult {
	return a.statusSummary
}

// GetStatusSummary returns fix status summary.
func (a *ShardFixResultAggregator) GetStatusSummary() ShardStatusSummaryResult {
	return a.statusSummary
}

// GetAggregation returns scan aggregation.
func (a *ShardFixResultAggregator) GetAggregation() AggregateFixReportResult {
	return a.aggregation
}

// GetStatusResult returns paginated results for a range of shards
func (a *ShardFixResultAggregator) GetStatusResult(req PaginatedShardQueryRequest) (*ShardStatusQueryResult, error) {
	return getStatusResult(a.minShard, a.maxShard, req, a.status)
}

// GetStatusResult returns stats broken by domain IDs
func (a *ShardFixResultAggregator) GetDomainStatus(req DomainReportQueryRequest) (*DomainFixReportQueryResult, error) {
	responseStats := []DomainFixStats{}

	if req.DomainID == nil {
		domainIDs := make([]string, 0)
		for domainID := range a.domainStats {
			domainIDs = append(domainIDs, domainID)
		}
		sort.Strings(domainIDs)
		for _, domainID := range domainIDs {
			responseStats = append(responseStats, DomainFixStats{
				DomainID: domainID,
				Stats:    *a.domainStats[domainID],
			})
		}
	} else if domainStats, ok := a.domainStats[*req.DomainID]; ok {
		responseStats = append(responseStats, DomainFixStats{
			DomainID: *req.DomainID,
			Stats:    *domainStats,
		})
	} else {
		return nil, fmt.Errorf("domain %v does not exist or relevant stats have not been reported yet", req.DomainID)
	}

	result := &DomainFixReportQueryResult{
		Reports: responseStats,
	}

	return result, nil
}

// AddReport adds fix report for a shard.
func (a *ShardFixResultAggregator) AddReport(report FixReport) {
	a.reports[report.ShardID] = report
	a.statusSummary[ShardStatusRunning]--
	if report.Result.ControlFlowFailure != nil {
		a.status[report.ShardID] = ShardStatusControlFlowFailure
		a.statusSummary[ShardStatusControlFlowFailure]++
	} else {
		a.status[report.ShardID] = ShardStatusSuccess
		a.statusSummary[ShardStatusSuccess]++
	}
	if report.Result.ShardFixKeys != nil {
		a.adjustAggregation(report.Stats, func(a, b int64) int64 { return a + b })
	}
	if report.DomainStats != nil {
		a.updateDomainStats(report)
	}
}

func (a *ShardFixResultAggregator) updateDomainStats(report FixReport) {
	for domainID, domainStats := range report.DomainStats {
		if _, ok := a.domainStats[domainID]; !ok {
			a.domainStats[domainID] = &FixStats{}
		}
		aggregateStats := a.domainStats[domainID]
		aggregateStats.EntitiesCount += domainStats.EntitiesCount
		aggregateStats.FixedCount += domainStats.FixedCount
		aggregateStats.SkippedCount += domainStats.SkippedCount
		aggregateStats.FailedCount += domainStats.FailedCount
	}
}

// GetReport returns fix report for a shard.
func (a *ShardFixResultAggregator) GetReport(shardID int) (*FixReport, error) {
	if _, ok := a.status[shardID]; !ok {
		return nil, fmt.Errorf("shard %v is not included in shards which will be processed", shardID)
	}
	if report, ok := a.reports[shardID]; ok {
		return &report, nil
	}
	return nil, fmt.Errorf("shard %v has not finished yet, check back later for report", shardID)
}

func (a *ShardFixResultAggregator) GetAllFixResults() (map[int]FixResult, error) {
	result := make(map[int]FixResult, len(a.reports))
	for k, v := range a.reports {
		if v.Result.Empty() {
			continue
		}
		result[k] = v.Result
	}
	return result, nil
}

func (a *ShardFixResultAggregator) adjustAggregation(stats FixStats, fn func(a, b int64) int64) {
	a.aggregation.EntitiesCount = fn(a.aggregation.EntitiesCount, stats.EntitiesCount)
	a.aggregation.SkippedCount = fn(a.aggregation.SkippedCount, stats.SkippedCount)
	a.aggregation.FailedCount = fn(a.aggregation.FailedCount, stats.FailedCount)
	a.aggregation.FixedCount = fn(a.aggregation.FixedCount, stats.FixedCount)
}

// NewShardScanResultAggregator returns aggregator for a scan result.
func NewShardScanResultAggregator(
	shards []int,
	minShard int,
	maxShard int,
) *ShardScanResultAggregator {
	status := make(map[int]ShardStatus)
	for _, s := range shards {
		status[s] = ShardStatusRunning
	}
	statusSummary := map[ShardStatus]int{
		ShardStatusSuccess:            0,
		ShardStatusControlFlowFailure: 0,
		ShardStatusRunning:            len(shards),
	}
	return &ShardScanResultAggregator{
		minShard: minShard,
		maxShard: maxShard,

		reports:       make(map[int]ScanReport),
		domainStats:   map[string]*ScanStats{},
		status:        status,
		statusSummary: statusSummary,
		shardSizes:    nil,
		aggregation: AggregateScanReportResult{
			CorruptionByType: make(map[invariant.Name]int64),
		},
		corruptionKeys: make(map[int]store.Keys),
	}
}

// GetAggregateReport returns aggregated scan report.
func (a *ShardScanResultAggregator) GetAggregateReport() AggregateScanReportResult {
	return a.aggregation
}

// GetShardSizeQueryResult returns shard size statistics.
func (a *ShardScanResultAggregator) GetShardSizeQueryResult(req ShardSizeQueryRequest) (ShardSizeQueryResult, error) {
	if req.StartIndex < 0 || req.StartIndex >= req.EndIndex || req.EndIndex > len(a.shardSizes) {
		return nil, fmt.Errorf("index out of bounds exception (required startIndex >= 0 && startIndex < endIndex && endIndex <= %v)", len(a.shardSizes))
	}
	if req.EndIndex-req.StartIndex > maxShardQueryResult {
		return nil, fmt.Errorf("too many shards requested, the limit is %v", maxShardQueryResult)
	}
	return a.shardSizes[req.StartIndex:req.EndIndex], nil
}

// GetCorruptionKeys returns a list of corrupt keys
func (a *ShardScanResultAggregator) GetCorruptionKeys(req PaginatedShardQueryRequest) (*ShardCorruptKeysQueryResult, error) {
	startingShardID := a.minShard
	if req.StartingShardID != nil {
		startingShardID = *req.StartingShardID
	}
	if err := shardInBounds(a.minShard, a.maxShard, startingShardID); err != nil {
		return nil, err
	}
	limit := maxShardQueryResult
	if req.LimitShards != nil && *req.LimitShards > 0 && *req.LimitShards < maxShardQueryResult {
		limit = *req.LimitShards
	}
	result := make(map[int]store.Keys)
	currentShardID := startingShardID
	for len(result) < limit && currentShardID <= a.maxShard {
		keys, ok := a.corruptionKeys[currentShardID]
		if !ok {
			currentShardID++
			continue
		}
		result[currentShardID] = keys
		currentShardID++
	}
	if currentShardID > a.maxShard {
		return &ShardCorruptKeysQueryResult{
			Result: result,
			ShardQueryPaginationToken: ShardQueryPaginationToken{
				NextShardID: nil,
				IsDone:      true,
			},
		}, nil
	}
	return &ShardCorruptKeysQueryResult{
		Result: result,
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: &currentShardID,
			IsDone:      false,
		},
	}, nil
}

// GetStatusResult returns scan status for a range of shards.
func (a *ShardScanResultAggregator) GetStatusResult(req PaginatedShardQueryRequest) (*ShardStatusQueryResult, error) {
	return getStatusResult(a.minShard, a.maxShard, req, a.status)
}

// AddReport adds scan report for a shard.
func (a *ShardScanResultAggregator) AddReport(report ScanReport) {
	if report.Result.ShardScanKeys != nil {
		a.insertReportIntoSizes(report)
	}
	a.reports[report.ShardID] = report
	a.statusSummary[ShardStatusRunning]--
	if report.Result.ControlFlowFailure != nil {
		a.status[report.ShardID] = ShardStatusControlFlowFailure
		a.statusSummary[ShardStatusControlFlowFailure]++
	} else {
		a.status[report.ShardID] = ShardStatusSuccess
		a.statusSummary[ShardStatusSuccess]++
	}
	if report.Result.ShardScanKeys != nil {
		a.adjustAggregation(report.Stats, func(a, b int64) int64 { return a + b })
		if report.Result.ShardScanKeys.Corrupt != nil {
			a.corruptionKeys[report.ShardID] = *report.Result.ShardScanKeys.Corrupt
		}
	}
	if report.DomainStats != nil {
		a.updateDomainStats(report)
	}
}

func (a *ShardScanResultAggregator) updateDomainStats(report ScanReport) {
	for domainID, domainStats := range report.DomainStats {
		if _, ok := a.domainStats[domainID]; !ok {
			a.domainStats[domainID] = &ScanStats{
				CorruptionByType: make(map[invariant.Name]int64),
			}
		}
		aggregateStats := a.domainStats[domainID]
		aggregateStats.CorruptedCount += domainStats.CorruptedCount
		aggregateStats.CheckFailedCount += domainStats.CheckFailedCount
		aggregateStats.EntitiesCount += domainStats.EntitiesCount
		for corruptionType, count := range domainStats.CorruptionByType {
			aggregateStats.CorruptionByType[corruptionType] += count
		}

	}
}

func (a *ShardScanResultAggregator) insertReportIntoSizes(report ScanReport) {
	tuple := ShardSizeTuple{
		ShardID:       report.ShardID,
		EntitiesCount: report.Stats.EntitiesCount,
	}
	insertIndex := 0
	for insertIndex < len(a.shardSizes) {
		if a.shardSizes[insertIndex].EntitiesCount < tuple.EntitiesCount {
			break
		}
		insertIndex++
	}
	a.shardSizes = append(a.shardSizes, ShardSizeTuple{})
	copy(a.shardSizes[insertIndex+1:], a.shardSizes[insertIndex:])
	a.shardSizes[insertIndex] = tuple
}

// GetShardDistributionStats returns aggregated size statistics
func (a *ShardScanResultAggregator) GetShardDistributionStats() ShardDistributionStats {
	return ShardDistributionStats{
		Max:    a.shardSizes[0].EntitiesCount,
		Median: a.shardSizes[int(float64(len(a.shardSizes))*.5)].EntitiesCount,
		Min:    a.shardSizes[len(a.shardSizes)-1].EntitiesCount,
		P90:    a.shardSizes[int(float64(len(a.shardSizes))*.1)].EntitiesCount,
		P75:    a.shardSizes[int(float64(len(a.shardSizes))*.25)].EntitiesCount,
		P25:    a.shardSizes[int(float64(len(a.shardSizes))*.75)].EntitiesCount,
		P10:    a.shardSizes[int(float64(len(a.shardSizes))*.9)].EntitiesCount,
	}
}

// GetReport returns a report for a single shard.
func (a *ShardScanResultAggregator) GetReport(shardID int) (*ScanReport, error) {
	if _, ok := a.status[shardID]; !ok {
		return nil, fmt.Errorf("shard %v is not included in shards which will be processed", shardID)
	}
	if report, ok := a.reports[shardID]; ok {
		return &report, nil
	}
	return nil, fmt.Errorf("shard %v has not finished yet, check back later for report", shardID)
}

// GetDomainStatus returns stats broken by domain IDs
func (a *ShardScanResultAggregator) GetDomainStatus(req DomainReportQueryRequest) (*DomainScanReportQueryResult, error) {
	responseStats := []DomainScanStats{}

	if req.DomainID == nil {
		domainIDs := make([]string, 0)
		for domainID := range a.domainStats {
			domainIDs = append(domainIDs, domainID)
		}
		sort.Strings(domainIDs)
		for _, domainID := range domainIDs {
			responseStats = append(responseStats, DomainScanStats{
				DomainID: domainID,
				Stats:    *a.domainStats[domainID],
			})
		}
	} else if domainStats, ok := a.domainStats[*req.DomainID]; ok {
		responseStats = append(responseStats, DomainScanStats{
			DomainID: *req.DomainID,
			Stats:    *domainStats,
		})
	} else {
		return nil, fmt.Errorf("domain %v does not exist or relevant stats have not been reported yet", req.DomainID)
	}

	result := &DomainScanReportQueryResult{
		Reports: responseStats,
	}

	return result, nil
}

func (a *ShardScanResultAggregator) adjustAggregation(stats ScanStats, fn func(a, b int64) int64) {
	a.aggregation.EntitiesCount = fn(a.aggregation.EntitiesCount, stats.EntitiesCount)
	a.aggregation.CorruptedCount = fn(a.aggregation.CorruptedCount, stats.CorruptedCount)
	a.aggregation.CheckFailedCount = fn(a.aggregation.CheckFailedCount, stats.CheckFailedCount)
	for k, v := range stats.CorruptionByType {
		a.aggregation.CorruptionByType[k] = fn(a.aggregation.CorruptionByType[k], v)
	}
}

func (a *ShardScanResultAggregator) GetAllScanResults() (map[int]ScanResult, error) {
	result := make(map[int]ScanResult, len(a.reports))
	for k, v := range a.reports {
		if v.Result.Empty() {
			continue
		}
		result[k] = v.Result
	}
	return result, nil
}

func getStatusResult(
	minShardID int,
	maxShardID int,
	req PaginatedShardQueryRequest,
	status ShardStatusResult,
) (*ShardStatusQueryResult, error) {
	startingShardID := minShardID
	if req.StartingShardID != nil {
		startingShardID = *req.StartingShardID
	}
	if err := shardInBounds(minShardID, maxShardID, startingShardID); err != nil {
		return nil, err
	}
	limit := maxShardQueryResult
	if req.LimitShards != nil && *req.LimitShards > 0 && *req.LimitShards < maxShardQueryResult {
		limit = *req.LimitShards
	}
	result := make(map[int]ShardStatus)
	currentShardID := startingShardID
	for len(result) < limit && currentShardID <= maxShardID {
		status, ok := status[currentShardID]
		if !ok {
			currentShardID++
			continue
		}
		result[currentShardID] = status
		currentShardID++
	}
	if currentShardID > maxShardID {
		return &ShardStatusQueryResult{
			Result: result,
			ShardQueryPaginationToken: ShardQueryPaginationToken{
				NextShardID: nil,
				IsDone:      true,
			},
		}, nil
	}
	return &ShardStatusQueryResult{
		Result: result,
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: &currentShardID,
			IsDone:      false,
		},
	}, nil
}

func shardInBounds(minShardID, maxShardID, shardID int) error {
	if shardID > maxShardID || shardID < minShardID {
		return fmt.Errorf("requested shard %v is outside of bounds (min: %v and max: %v)", shardID, minShardID, maxShardID)
	}
	return nil
}
