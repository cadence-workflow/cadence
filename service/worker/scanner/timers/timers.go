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

package timers

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/cadence/client"
	"go.uber.org/cadence/workflow"

	"github.com/uber/cadence/common/blobstore"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/collection"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/fetcher"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/service/worker/scanner/shardscanner"
)

const (
	// ScannerWFTypeName defines workflow type name for concrete executions scanner
	ScannerWFTypeName   = "cadence-sys-timers-scanner-workflow"
	wfid                = "cadence-sys-timers-scanner"
	scannerTaskListName = "cadence-sys-timers-scanner-tasklist-0"

	// FixerWFTypeName defines workflow type name for timers fixer
	FixerWFTypeName   = "cadence-sys-timers-fixer-workflow"
	fixerTaskListName = "cadence-sys-timers-fixer-tasklist-0"
	fixerwfid         = "cadence-sys-timers-fixer"
	periodStartKey    = "period_start"
	periodEndKey      = "period_end"
)

// ScannerWorkflow starts timers scanner.
func ScannerWorkflow(
	ctx workflow.Context,
	params shardscanner.ScannerWorkflowParams,
) error {
	wf, err := shardscanner.NewScannerWorkflow(ctx, ScannerWFTypeName, params)
	if err != nil {
		return err
	}

	return wf.Start(ctx)
}

// FixerWorkflow starts timers executions fixer.
func FixerWorkflow(
	ctx workflow.Context,
	params shardscanner.FixerWorkflowParams,
) error {
	wf, err := shardscanner.NewFixerWorkflow(ctx, FixerWFTypeName, params)
	if err != nil {
		return err
	}

	return wf.Start(ctx)
}

// ScannerHooks provides hooks for timers scanner.
func ScannerHooks() *shardscanner.ScannerHooks {
	h, err := shardscanner.NewScannerHooks(Manager, Iterator, Config)
	if err != nil {
		return nil
	}

	return h
}

// FixerHooks provides hooks needed for timers fixer.
func FixerHooks() *shardscanner.FixerHooks {
	h, err := shardscanner.NewFixerHooks(FixerManager, FixerIterator, timerCustomConfig)
	if err != nil {
		return nil
	}
	return h
}

func timerCustomConfig(_ shardscanner.FixerContext) shardscanner.CustomScannerConfig {
	// must be non-empty to pass backwards-compat check,
	// and this allows safely adding more invariants in the future.
	//
	// currently this is not read anywhere because "fixer enabled"
	// means "run this one invariant's fixes".
	return map[string]string{
		invariant.TimerInvalidName: "true",
	}
}

// Manager provides invariant manager for timers scanner.
func Manager(
	_ context.Context,
	pr persistence.Retryer,
	_ shardscanner.ScanShardActivityParams,
	cache cache.DomainCache,
) invariant.Manager {
	return invariant.NewInvariantManager(getInvariants(pr, cache))
}

// Iterator provides iterator for timers scanner.
func Iterator(
	ctx context.Context,
	pr persistence.Retryer,
	params shardscanner.ScanShardActivityParams,
) pagination.Iterator {
	start, err := strconv.Atoi(params.ScannerConfig[periodStartKey])
	if err != nil {
		return nil
	}
	end, err := strconv.Atoi(params.ScannerConfig[periodEndKey])
	if err != nil {
		return nil
	}

	startAt := time.Now().Add(time.Hour * time.Duration(start))
	endAt := startAt.Add(time.Hour * time.Duration(end))

	return fetcher.TimerIterator(ctx, pr, startAt, endAt, params.PageSize)

}

// FixerIterator provides iterator for timers fixer.
func FixerIterator(
	ctx context.Context,
	client blobstore.Client,
	keys store.Keys,
	_ shardscanner.FixShardActivityParams,
) store.ScanOutputIterator {
	return store.NewBlobstoreIterator(ctx, client, keys, &entity.Timer{})
}

// FixerManager provides invariant manager for timers fixer.
func FixerManager(
	_ context.Context,
	pr persistence.Retryer,
	_ shardscanner.FixShardActivityParams,
	cache cache.DomainCache,
) invariant.Manager {
	return invariant.NewInvariantManager(getInvariants(pr, cache))
}

// Config resolves dynamic config for timers scanner.
func Config(ctx shardscanner.ScannerContext) shardscanner.CustomScannerConfig {
	res := shardscanner.CustomScannerConfig{}
	res[periodStartKey] = strconv.Itoa(ctx.Config.DynamicCollection.GetIntProperty(dynamicconfig.TimersScannerPeriodStart)())
	res[periodEndKey] = strconv.Itoa(ctx.Config.DynamicCollection.GetIntProperty(dynamicconfig.TimersScannerPeriodEnd)())
	return res
}

// ScannerConfig configures timers scanner
func ScannerConfig(dc *collection.Collection) *shardscanner.ScannerConfig {
	return &shardscanner.ScannerConfig{
		ScannerWFTypeName: ScannerWFTypeName,
		FixerWFTypeName:   FixerWFTypeName,
		DynamicParams: shardscanner.DynamicParams{
			ScannerEnabled:          dc.GetBoolProperty(dynamicconfig.TimersScannerEnabled),
			FixerEnabled:            dc.GetBoolProperty(dynamicconfig.TimersFixerEnabled),
			Concurrency:             dc.GetIntProperty(dynamicconfig.TimersScannerConcurrency),
			PageSize:                dc.GetIntProperty(dynamicconfig.TimersScannerPersistencePageSize),
			BlobstoreFlushThreshold: dc.GetIntProperty(dynamicconfig.TimersScannerBlobstoreFlushThreshold),
			ActivityBatchSize:       dc.GetIntProperty(dynamicconfig.TimersScannerActivityBatchSize),
			AllowDomain:             dc.GetBoolPropertyFilteredByDomain(dynamicconfig.TimersFixerDomainAllow),
		},
		DynamicCollection: dc,
		ScannerHooks:      ScannerHooks,
		FixerHooks:        FixerHooks,

		StartWorkflowOptions: client.StartWorkflowOptions{
			ID:                           wfid,
			TaskList:                     scannerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        client.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
		StartFixerOptions: client.StartWorkflowOptions{
			ID:                           fixerwfid,
			TaskList:                     fixerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        client.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
	}
}

func getInvariants(pr persistence.Retryer, cache cache.DomainCache) []invariant.Invariant {
	return []invariant.Invariant{
		invariant.NewTimerInvalid(pr, cache),
	}
}
