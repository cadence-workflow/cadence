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

package executions

import (
	"context"
	"strconv"
	"time"

	cclient "go.uber.org/cadence/client"
	"go.uber.org/cadence/workflow"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/blobstore"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
	"github.com/uber/cadence/common/reconciliation/store"
	"github.com/uber/cadence/service/worker/scanner/shardscanner"
)

const (
	// currentExecutionsScannerWFID is the current execution scanner workflow ID
	currentExecutionsScannerWFID = "cadence-sys-current-executions-scanner"
	// CurrentExecutionsScannerWFTypeName is the current execution scanner workflow type
	CurrentExecutionsScannerWFTypeName = "cadence-sys-current-executions-scanner-workflow"
	// CurrentExecutionsScannerTaskListName is the current execution scanner workflow tasklist
	CurrentExecutionsScannerTaskListName = "cadence-sys-current-executions-scanner-tasklist-0"

	// CurrentExecutionsFixerWFTypeName is the current execution fixer workflow ID
	CurrentExecutionsFixerWFTypeName = "cadence-sys-current-executions-fixer-workflow"
	currentExecutionsFixerWFID       = "cadence-sys-current-executions-fixer"
	// CurrentExecutionsFixerTaskListName is the current execution fixer workflow tasklist
	CurrentExecutionsFixerTaskListName = "cadence-sys-current-executions-fixer-tasklist-0"
)

/*

!!!!!!!!!!!!!!
NOTE: Current execution fixers have never been run.
Beware drawing any conclusions from current-execution scanner/fixer code.
!!!!!!!!!!!!!!

While this code appears structurally complete, the wrong fixer manager is being
used, and we have apparently never fully enabled it in our production clusters.

It likely needs further checks and possibly a rewrite before attempting to use.

*/

// CurrentScannerWorkflow is the workflow that scans over all current executions
func CurrentScannerWorkflow(
	ctx workflow.Context,
	params shardscanner.ScannerWorkflowParams,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting CurrentScannerWorkflow", zap.Any("Params", params))

	wf, err := shardscanner.NewScannerWorkflow(ctx, CurrentExecutionsScannerWFTypeName, params)
	if err != nil {
		logger.Error("Failed to start new scanner workflow", zap.Error(err))
		return err
	}

	err = wf.Start(ctx)
	if err != nil {
		logger.Error("Failed to execute scanner workflow", zap.Error(err))
	}
	return err
}

// currentExecutionScannerHooks provides hooks for current executions scanner.
func currentExecutionScannerHooks() *shardscanner.ScannerHooks {
	wf, err := shardscanner.NewScannerHooks(currentExecutionScannerManager, currentExecutionScannerIterator, currentExecutionCustomScannerConfig)
	if err != nil {
		return nil
	}
	return wf
}

// currentExecutionScannerManager is the current execution scanner manager
func currentExecutionScannerManager(
	ctx context.Context,
	pr persistence.Retryer,
	params shardscanner.ScanShardActivityParams,
	domainCache cache.DomainCache,
) invariant.Manager {
	logger := zap.L()
	logger.Info("Creating invariant manager for current execution scanner", zap.Any("Params", params))
	var ivs []invariant.Invariant
	collections := ParseCollections(params.ScannerConfig)
	for _, fn := range CurrentExecutionType.ToInvariants(collections, zap.NewNop()) {
		ivs = append(ivs, fn(pr, domainCache))
	}
	return invariant.NewInvariantManager(ivs)
}

// CurrentFixerWorkflow starts current executions fixer.
func CurrentFixerWorkflow(
	ctx workflow.Context,
	params shardscanner.FixerWorkflowParams,
) error {
	wf, err := shardscanner.NewFixerWorkflow(ctx, CurrentExecutionsFixerWFTypeName, params)
	if err != nil {
		return err
	}
	return wf.Start(ctx)
}

// currentExecutionCustomScannerConfig resolves dynamic config for current executions scanner.
func currentExecutionCustomScannerConfig(ctx shardscanner.ScannerContext) shardscanner.CustomScannerConfig {
	res := shardscanner.CustomScannerConfig{}

	if ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.CurrentExecutionsScannerInvariantCollectionHistory)() {
		res[invariant.CollectionHistory.String()] = strconv.FormatBool(true)
	}
	if ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.CurrentExecutionsScannerInvariantCollectionMutableState)() {
		res[invariant.CollectionMutableState.String()] = strconv.FormatBool(true)
	}

	return res
}

// currentExecutionFixerHooks provides hooks for current executions fixer.
func currentExecutionFixerHooks() *shardscanner.FixerHooks {
	noCustomConfig := func(fixer shardscanner.FixerContext) shardscanner.CustomScannerConfig {
		return nil
	}
	// TODO: yes, this DOES incorrectly use the concrete execution fixer manager, which does not work.
	// It is retained for now to avoid making a lot of mostly-unrelated changes / fixes / cleanup.
	h, err := shardscanner.NewFixerHooks(concreteExecutionFixerManager, currentExecutionFixerIterator, noCustomConfig)
	if err != nil {
		return nil
	}
	return h
}

// CurrentExecutionConfig configures current execution scanner
func CurrentExecutionConfig(dc *dynamicconfig.Collection) *shardscanner.ScannerConfig {
	return &shardscanner.ScannerConfig{
		ScannerWFTypeName: CurrentExecutionsScannerWFTypeName,
		FixerWFTypeName:   CurrentExecutionsFixerWFTypeName,
		DynamicCollection: dc,
		DynamicParams: shardscanner.DynamicParams{
			ScannerEnabled:          dc.GetBoolProperty(dynamicproperties.CurrentExecutionsScannerEnabled),
			FixerEnabled:            dc.GetBoolProperty(dynamicproperties.CurrentExecutionFixerEnabled),
			Concurrency:             dc.GetIntProperty(dynamicproperties.CurrentExecutionsScannerConcurrency),
			PageSize:                dc.GetIntProperty(dynamicproperties.CurrentExecutionsScannerPersistencePageSize),
			BlobstoreFlushThreshold: dc.GetIntProperty(dynamicproperties.CurrentExecutionsScannerBlobstoreFlushThreshold),
			ActivityBatchSize:       dc.GetIntProperty(dynamicproperties.CurrentExecutionsScannerActivityBatchSize),
			AllowDomain:             dc.GetBoolPropertyFilteredByDomain(dynamicproperties.CurrentExecutionFixerDomainAllow),
		},
		ScannerHooks: currentExecutionScannerHooks,
		FixerHooks:   currentExecutionFixerHooks,
		StartWorkflowOptions: cclient.StartWorkflowOptions{
			ID:                           currentExecutionsScannerWFID,
			TaskList:                     CurrentExecutionsScannerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        cclient.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
		StartFixerOptions: cclient.StartWorkflowOptions{
			ID:                           currentExecutionsFixerWFID,
			TaskList:                     CurrentExecutionsFixerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        cclient.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
	}
}

// currentExecutionScannerIterator is the iterator of current executions
func currentExecutionScannerIterator(
	ctx context.Context,
	pr persistence.Retryer,
	params shardscanner.ScanShardActivityParams,
) pagination.Iterator {
	return CurrentExecutionType.ToIterator()(ctx, pr, params.PageSize)
}

// currentExecutionFixerIterator is the iterator of fixer execution
func currentExecutionFixerIterator(
	ctx context.Context,
	client blobstore.Client,
	keys store.Keys,
	_ shardscanner.FixShardActivityParams,
) store.ScanOutputIterator {
	return store.NewBlobstoreIterator(ctx, client, keys, CurrentExecutionType.ToBlobstoreEntity())
}
