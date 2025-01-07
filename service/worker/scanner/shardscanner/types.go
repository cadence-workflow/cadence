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
	"fmt"
	"time"

	"go.uber.org/cadence"
	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/common/resource"
)

const (
	// ErrScanWorkflowNotClosed indicates fix was attempted on scan workflow which was not finished
	ErrScanWorkflowNotClosed = "scan workflow is not closed, only can run fix on output of finished scan workflow"
	// ErrSerialization indicates a serialization or deserialization error occurred
	ErrSerialization = "encountered serialization error"
	// ErrMissingHooks indicates scanner is not providing hooks to Invariant manager or Iterator
	ErrMissingHooks = "hooks are not provided for this scanner"
)

type (
	contextKey string

	// ScannerContext is the resource that is available in activities under ShardScanner context key
	ScannerContext struct {
		Resource resource.Resource
		Hooks    *ScannerHooks
		Scope    metrics.Scope
		Config   *ScannerConfig
		Logger   log.Logger
	}

	// FixerContext is the resource that is available to activities under ShardFixer key
	FixerContext struct {
		Resource resource.Resource
		Hooks    *FixerHooks
		Scope    metrics.Scope
		Config   *ScannerConfig
		Logger   log.Logger
	}

	// ScannerEmitMetricsActivityParams is the parameter for scannerEmitMetricsActivity
	ScannerEmitMetricsActivityParams struct {
		ShardSuccessCount            int
		ShardControlFlowFailureCount int
		AggregateReportResult        AggregateScanReportResult
		ShardDistributionStats       ShardDistributionStats
	}

	// ShardRange identifies a set of shards based on min (inclusive) and max (exclusive)
	ShardRange struct {
		Min int
		Max int
	}

	// Shards identify the shards that should be scanned.
	// Exactly one of List or Range should be non-nil.
	Shards struct {
		List  []int
		Range *ShardRange
	}

	// ScannerWorkflowParams are the parameters to the scan workflow
	ScannerWorkflowParams struct {
		Shards                          Shards
		ScannerWorkflowConfigOverwrites ScannerWorkflowConfigOverwrites
	}

	// ScannerConfigActivityParams is the parameter for scannerConfigActivity
	ScannerConfigActivityParams struct {
		Overwrites ScannerWorkflowConfigOverwrites
	}

	// ScanShardActivityParams is the parameter for scanShardActivity
	ScanShardActivityParams struct {
		Shards                  []int
		PageSize                int
		BlobstoreFlushThreshold int
		ScannerConfig           CustomScannerConfig
	}

	// FixerWorkflowParams are the parameters to the fix workflow
	FixerWorkflowParams struct {
		ScannerWorkflowWorkflowID     string
		ScannerWorkflowRunID          string
		FixerWorkflowConfigOverwrites FixerWorkflowConfigOverwrites
	}

	// ScanReport is the report of running Scan on a single shard.
	ScanReport struct {
		ShardID     int
		Stats       ScanStats
		Result      ScanResult
		DomainStats map[string]*ScanStats
	}

	// DomainStats is the report of stats for one domain
	DomainScanStats struct {
		DomainID string
		Stats    ScanStats
	}

	// DomainStats is the report of stats for one domain
	DomainFixStats struct {
		DomainID string
		Stats    FixStats
	}

	// ScanStats indicates the stats of entities which were handled by shard Scan.
	ScanStats struct {
		EntitiesCount    int64
		CorruptedCount   int64
		CheckFailedCount int64
		CorruptionByType map[invariant.Name]int64
	}

	// ScanResult indicates the result of running scan on a shard.
	// Exactly one of ControlFlowFailure or ScanKeys will be non-nil
	ScanResult struct {
		ShardScanKeys      *ScanKeys
		ControlFlowFailure *ControlFlowFailure
	}

	// ScanKeys are the keys to the blobs that were uploaded during scan.
	// Keys can be nil if there were no uploads.
	ScanKeys struct {
		Corrupt *store.Keys
		Failed  *store.Keys
	}

	// FixReport is the report of running Fix on a single shard
	FixReport struct {
		ShardID     int
		Stats       FixStats
		Result      FixResult
		DomainStats map[string]*FixStats
	}

	// FixStats indicates the stats of executions that were handled by shard Fix.
	FixStats struct {
		EntitiesCount int64
		FixedCount    int64
		SkippedCount  int64
		FailedCount   int64
	}

	// FixResult indicates the result of running fix on a shard.
	// Exactly one of ControlFlowFailure or FixKeys will be non-nil.
	FixResult struct {
		ShardFixKeys       *FixKeys
		ControlFlowFailure *ControlFlowFailure
	}

	// FixKeys are the keys to the blobs that were uploaded during fix.
	// Keys can be nil if there were no uploads.
	FixKeys struct {
		Skipped *store.Keys
		Failed  *store.Keys
		Fixed   *store.Keys
	}

	// ControlFlowFailure indicates an error occurred which makes it impossible to
	// even attempt to check or fix one or more execution(s). Note that it is not a ControlFlowFailure
	// if a check or fix fails, it is only a ControlFlowFailure if
	// an error is encountered which makes even attempting to check or fix impossible.
	ControlFlowFailure struct {
		Info        string
		InfoDetails string
	}

	// FixerCorruptedKeysActivityParams is the parameter for fixerCorruptedKeysActivity
	FixerCorruptedKeysActivityParams struct {
		ScannerWorkflowWorkflowID string
		ScannerWorkflowRunID      string
		StartingShardID           *int
	}

	// FixShardActivityParams is the parameter for fixShardActivity
	FixShardActivityParams struct {
		CorruptedKeysEntries        []CorruptedKeysEntry
		ResolvedFixerWorkflowConfig ResolvedFixerWorkflowConfig

		// EnabledInvariants contains all known invariants for fixer, with "true" or "false" values.
		// In current code it should never be empty.
		//
		// If empty, EnabledInvariants came from old serialized data prior to this field existing,
		// and the historical list of invariants should be used.  This should be a one-time event
		// after upgrading.
		EnabledInvariants CustomScannerConfig
	}

	// CustomScannerConfig is used to pass key/value parameters between shardscanner activity and scanner/fixer implementations.
	// this is used to have activities with better consistency, as the workflow records one config and uses it for all shards,
	// even after config / code changes.
	//
	// It is currently only used to pass invariant names to control whether they are enabled or not.
	// Please do not use this for other purposes.
	CustomScannerConfig map[string]string

	// GenericScannerConfig is a generic params for all shard scanners
	GenericScannerConfig struct {
		Enabled                 bool
		Concurrency             int
		PageSize                int
		BlobstoreFlushThreshold int
		ActivityBatchSize       int
	}

	// GenericScannerConfigOverwrites allows to override generic params
	GenericScannerConfigOverwrites struct {
		Enabled                 *bool
		Concurrency             *int
		PageSize                *int
		BlobstoreFlushThreshold *int
		ActivityBatchSize       *int
	}

	// ResolvedScannerWorkflowConfig is the resolved config after reading dynamic config
	// and applying overwrites.
	ResolvedScannerWorkflowConfig struct {
		GenericScannerConfig GenericScannerConfig
		CustomScannerConfig  CustomScannerConfig
	}

	// ScannerWorkflowConfigOverwrites enables overwriting the values in dynamic config.
	// If provided workflow will favor overwrites over dynamic config.
	// Any overwrites that are left as nil will fall back to using dynamic config.
	ScannerWorkflowConfigOverwrites struct {
		GenericScannerConfig GenericScannerConfigOverwrites
		CustomScannerConfig  *CustomScannerConfig
	}

	// DynamicParams is the dynamic config for scanner workflow.
	DynamicParams struct {
		ScannerEnabled          dynamicconfig.BoolPropertyFn
		FixerEnabled            dynamicconfig.BoolPropertyFn
		Concurrency             dynamicconfig.IntPropertyFn
		PageSize                dynamicconfig.IntPropertyFn
		BlobstoreFlushThreshold dynamicconfig.IntPropertyFn
		ActivityBatchSize       dynamicconfig.IntPropertyFn
		AllowDomain             dynamicconfig.BoolPropertyFnWithDomainFilter
	}

	// ScannerConfig is the config for ShardScanner workflow
	ScannerConfig struct {
		ScannerWFTypeName    string
		FixerWFTypeName      string
		ScannerHooks         func() *ScannerHooks
		FixerHooks           func() *FixerHooks
		DynamicParams        DynamicParams
		DynamicCollection    *dynamicconfig.Collection
		StartWorkflowOptions client.StartWorkflowOptions
		StartFixerOptions    client.StartWorkflowOptions
	}

	// FixerWorkflowConfigOverwrites enables overwriting the default values.
	// If provided workflow will favor overwrites over defaults.
	// Any overwrites that are left as nil will fall back to defaults.
	FixerWorkflowConfigOverwrites struct {
		Concurrency             *int
		BlobstoreFlushThreshold *int
		ActivityBatchSize       *int
	}

	// ResolvedFixerWorkflowConfig is the resolved config after reading defaults and applying overwrites.
	ResolvedFixerWorkflowConfig struct {
		Concurrency             int
		BlobstoreFlushThreshold int
		ActivityBatchSize       int
	}
)

func getShortActivityContext(ctx workflow.Context) workflow.Context {
	activityOptions := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    5 * time.Minute,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:          time.Second,
			BackoffCoefficient:       1.7,
			ExpirationInterval:       10 * time.Minute,
			NonRetriableErrorReasons: []string{ErrScanWorkflowNotClosed, ErrSerialization, ErrMissingHooks},
		},
	}
	return workflow.WithActivityOptions(ctx, activityOptions)
}

func getLongActivityContext(ctx workflow.Context) workflow.Context {
	activityOptions := workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    48 * time.Hour,
		HeartbeatTimeout:       time.Minute,
		RetryPolicy: &cadence.RetryPolicy{
			InitialInterval:          time.Second,
			BackoffCoefficient:       1.7,
			ExpirationInterval:       48 * time.Hour,
			NonRetriableErrorReasons: []string{ErrScanWorkflowNotClosed, ErrSerialization, ErrMissingHooks},
		},
	}
	return workflow.WithActivityOptions(ctx, activityOptions)
}

// GetCorruptedKeys is a workflow which is used to retrieve keys for fixer workflow.
func GetCorruptedKeys(
	ctx workflow.Context,
	params FixerWorkflowParams,
) (*FixerCorruptedKeysActivityResult, error) {
	fixerCorruptedKeysActivityParams := FixerCorruptedKeysActivityParams{
		ScannerWorkflowWorkflowID: params.ScannerWorkflowWorkflowID,
		ScannerWorkflowRunID:      params.ScannerWorkflowRunID,
		StartingShardID:           nil,
	}
	var minShardID *int
	var maxShardID *int
	var corruptKeys []CorruptedKeysEntry
	isFirst := true
	for isFirst || fixerCorruptedKeysActivityParams.StartingShardID != nil {
		isFirst = false
		corruptedKeysResult := &FixerCorruptedKeysActivityResult{}
		activityCtx := getShortActivityContext(ctx)
		if err := workflow.ExecuteActivity(
			activityCtx,
			ActivityFixerCorruptedKeys,
			fixerCorruptedKeysActivityParams).Get(ctx, &corruptedKeysResult); err != nil {
			return nil, err
		}

		fixerCorruptedKeysActivityParams.StartingShardID = corruptedKeysResult.ShardQueryPaginationToken.NextShardID
		if len(corruptedKeysResult.CorruptedKeys) == 0 {
			continue
		}
		corruptKeys = append(corruptKeys, corruptedKeysResult.CorruptedKeys...)
		if corruptedKeysResult.MinShard != nil && (minShardID == nil || *minShardID > *corruptedKeysResult.MinShard) {
			minShardID = corruptedKeysResult.MinShard
		}
		if corruptedKeysResult.MaxShard != nil && (maxShardID == nil || *maxShardID < *corruptedKeysResult.MaxShard) {
			maxShardID = corruptedKeysResult.MaxShard
		}
	}
	return &FixerCorruptedKeysActivityResult{
		CorruptedKeys: corruptKeys,
		MinShard:      minShardID,
		MaxShard:      maxShardID,
		ShardQueryPaginationToken: ShardQueryPaginationToken{
			NextShardID: nil,
			IsDone:      true,
		},
	}, nil
}

func getBatchIndices(
	batchSize int,
	concurrency int,
	sliceLength int,
	workerIdx int,
) [][]int {
	var batches [][]int
	var currBatch []int
	for i := 0; i < sliceLength; i++ {
		if i%concurrency == workerIdx {
			currBatch = append(currBatch, i)
			if len(currBatch) == batchSize {
				batches = append(batches, currBatch)
				currBatch = nil
			}
		}
	}
	if len(currBatch) > 0 {
		batches = append(batches, currBatch)
	}
	return batches
}

// Validate validates shard list or range
func (s Shards) Validate() error {
	if s.List == nil && s.Range == nil {
		return errors.New("must provide either List or Range")
	}
	if s.List != nil && s.Range != nil {
		return errors.New("only one of List or Range can be provided")
	}
	if s.List != nil && len(s.List) == 0 {
		return errors.New("empty List provided")
	}
	if s.Range != nil && s.Range.Max <= s.Range.Min {
		return errors.New("empty Range provided")
	}
	return nil
}

// Flatten flattens Shards to a list of shard IDs and finds the min/max shardID
func (s Shards) Flatten() ([]int, int, int) {
	shardList := s.List
	if len(shardList) == 0 {
		shardList = []int{}
		for i := s.Range.Min; i < s.Range.Max; i++ {
			shardList = append(shardList, i)
		}
	}
	min := shardList[0]
	max := shardList[0]
	for i := 1; i < len(shardList); i++ {
		if shardList[i] < min {
			min = shardList[i]
		}
		if shardList[i] > max {
			max = shardList[i]
		}
	}

	return shardList, min, max
}

// NewShardScannerContext sets scanner context up
func NewShardScannerContext(
	res resource.Resource,
	config *ScannerConfig,
) ScannerContext {
	return ScannerContext{
		Resource: res,
		Scope:    res.GetMetricsClient().Scope(metrics.ExecutionsScannerScope),
		Config:   config,
		Logger:   res.GetLogger().WithTags(tag.ComponentShardScanner),
		Hooks:    config.ScannerHooks(),
	}
}

// NewShardFixerContext sets fixer context up
func NewShardFixerContext(
	res resource.Resource,
	config *ScannerConfig,
) FixerContext {
	return FixerContext{
		Resource: res,
		Scope:    res.GetMetricsClient().Scope(metrics.ExecutionsFixerScope),
		Config:   config,
		Hooks:    config.FixerHooks(),
		Logger:   res.GetLogger().WithTags(tag.ComponentShardFixer),
	}
}

// NewContext provides context to be used as background activity context
func NewFixerContext(
	ctx context.Context,
	workflowName string,
	fixerContext FixerContext,
) context.Context {
	return context.WithValue(ctx, contextKey(workflowName), fixerContext)
}

// NewContext provides context to be used as background activity context
func NewScannerContext(
	ctx context.Context,
	workflowName string,
	scannerContext ScannerContext,
) context.Context {
	return context.WithValue(ctx, contextKey(workflowName), scannerContext)
}

// GetScannerContext extracts scanner context from activity context
func GetScannerContext(
	ctx context.Context,
) (ScannerContext, error) {
	info := activity.GetInfo(ctx)
	if info.WorkflowType == nil {
		return ScannerContext{}, fmt.Errorf("workflowType is nil")
	}
	val, ok := ctx.Value(contextKey(info.WorkflowType.Name)).(ScannerContext)
	if !ok {
		return ScannerContext{}, fmt.Errorf("context type is not %T for a key %q", val, info.WorkflowType.Name)
	}
	return val, nil
}

// GetFixerContext extracts fixer context from activity context
// it uses typed, private key to reduce access scope
func GetFixerContext(
	ctx context.Context,
) (FixerContext, error) {
	info := activity.GetInfo(ctx)
	if info.WorkflowType == nil {
		return FixerContext{}, fmt.Errorf("workflowType is nil")
	}
	val, ok := ctx.Value(contextKey(info.WorkflowType.Name)).(FixerContext)
	if !ok {
		return FixerContext{}, fmt.Errorf("context type is not %T for a key %q", val, info.WorkflowType.Name)
	}
	return val, nil
}

// Empty returns true if this ScanResult has no "real" data, e.g. only nils or empty values.
func (s *ScanResult) Empty() bool {
	if s == nil {
		return true
	}
	if s.ControlFlowFailure != nil && (*s.ControlFlowFailure != ControlFlowFailure{}) {
		return false // at least control flow failure has data
	}
	if s.ShardScanKeys != nil {
		if s.ShardScanKeys.Corrupt != nil && (*s.ShardScanKeys.Corrupt != store.Keys{}) {
			return false // corrupt data exists
		}
		if s.ShardScanKeys.Failed != nil && (*s.ShardScanKeys.Failed != store.Keys{}) {
			return false // failed data exists
		}
	}
	return true // both empty
}

// Empty returns true if this FixResult has no "real" data, e.g. only nils or empty values.
func (f *FixResult) Empty() bool {
	if f == nil {
		return true
	}
	if f.ControlFlowFailure != nil && (*f.ControlFlowFailure != ControlFlowFailure{}) {
		return false // at least control flow failure has data
	}
	if f.ShardFixKeys != nil {
		if f.ShardFixKeys.Fixed != nil && (*f.ShardFixKeys.Fixed != store.Keys{}) {
			return false // fixed data exists
		}
		if f.ShardFixKeys.Failed != nil && (*f.ShardFixKeys.Failed != store.Keys{}) {
			return false // failed data exists
		}
		if f.ShardFixKeys.Skipped != nil && (*f.ShardFixKeys.Skipped != store.Keys{}) {
			return false // skipped data exists
		}
	}
	return true // both empty
}
