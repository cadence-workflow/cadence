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
	"fmt"
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
	// ConcreteExecutionsScannerWFTypeName defines workflow type name for concrete executions scanner
	ConcreteExecutionsScannerWFTypeName   = "cadence-sys-executions-scanner-workflow"
	concreteExecutionsScannerWFID         = "cadence-sys-executions-scanner"
	concreteExecutionsScannerTaskListName = "cadence-sys-executions-scanner-tasklist-0"

	// ConcreteExecutionsFixerWFTypeName defines workflow type name for concrete executions fixer
	ConcreteExecutionsFixerWFTypeName   = "cadence-sys-executions-fixer-workflow"
	concreteExecutionsFixerWFID         = "cadence-sys-executions-fixer"
	concreteExecutionsFixerTaskListName = "cadence-sys-executions-fixer-tasklist-0"
)

// ConcreteScannerWorkflow starts concrete executions scanner.
func ConcreteScannerWorkflow(ctx workflow.Context, params shardscanner.ScannerWorkflowParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ConcreteExecutionsScannerWorkflow", zap.Any("Params", params))

	wf, err := shardscanner.NewScannerWorkflow(ctx, ConcreteExecutionsScannerWFTypeName, params)
	if err != nil {
		logger.Error("Failed to create new scanner workflow", zap.Error(err))
		return err
	}

	err = wf.Start(ctx)
	if err != nil {
		logger.Error("Failed to start scanner workflow", zap.Error(err))
	}
	return err
}

// ConcreteFixerWorkflow starts concrete executions fixer.
func ConcreteFixerWorkflow(ctx workflow.Context, params shardscanner.FixerWorkflowParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ConcreteExecutionsFixerWorkflow", zap.Any("Params", params))

	wf, err := shardscanner.NewFixerWorkflow(ctx, ConcreteExecutionsFixerWFTypeName, params)
	if err != nil {
		logger.Error("Failed to create new fixer workflow", zap.Error(err))
		return err
	}

	err = wf.Start(ctx)
	if err != nil {
		logger.Error("Failed to start fixer workflow", zap.Error(err))
	}
	return err
}

// concreteExecutionScannerHooks provides hooks for concrete executions scanner
func concreteExecutionScannerHooks() *shardscanner.ScannerHooks {
	h, err := shardscanner.NewScannerHooks(concreteExecutionScannerManager, concreteExecutionScannerIterator, concreteExecutionCustomScannerConfig)
	if err != nil {
		return nil
	}
	return h
}

// concreteExecutionFixerHooks provides hooks needed for concrete executions fixer.
func concreteExecutionFixerHooks() *shardscanner.FixerHooks {
	h, err := shardscanner.NewFixerHooks(concreteExecutionFixerManager, concreteExecutionFixerIterator, concreteExecutionCustomFixerConfig)
	if err != nil {
		return nil
	}
	return h
}

// concreteExecutionScannerManager provides invariant manager for concrete execution scanner
func concreteExecutionScannerManager(
	ctx context.Context,
	pr persistence.Retryer,
	params shardscanner.ScanShardActivityParams,
	domainCache cache.DomainCache,
) invariant.Manager {

	collections := ParseCollections(params.ScannerConfig)

	var ivs []invariant.Invariant
	for _, fn := range ConcreteExecutionType.ToInvariants(collections, zap.NewNop()) {
		ivs = append(ivs, fn(pr, domainCache))
	}

	return invariant.NewInvariantManager(ivs)
}

// concreteExecutionScannerIterator provides iterator for concrete execution scanner.
func concreteExecutionScannerIterator(
	ctx context.Context,
	pr persistence.Retryer,
	params shardscanner.ScanShardActivityParams,
) pagination.Iterator {
	it := ConcreteExecutionType.ToIterator()
	return it(ctx, pr, params.PageSize)

}

// concreteExecutionFixerIterator provides iterator for concrete execution fixer.
func concreteExecutionFixerIterator(ctx context.Context, client blobstore.Client, keys store.Keys, _ shardscanner.FixShardActivityParams) store.ScanOutputIterator {
	return store.NewBlobstoreIterator(ctx, client, keys, ConcreteExecutionType.ToBlobstoreEntity())
}

// concreteExecutionFixerManager provides invariant manager for concrete execution fixer.
func concreteExecutionFixerManager(_ context.Context, pr persistence.Retryer, params shardscanner.FixShardActivityParams, domainCache cache.DomainCache) invariant.Manager {
	// convert to invariants.
	// this may produce an empty list if it all fixers are intentionally disabled,
	// or if the list came from a previous version of the server which lacked this config.
	var collections []invariant.Collection
	for k, v := range params.EnabledInvariants {
		if v == strconv.FormatBool(true) {
			ivc, err := invariant.CollectionString(k)
			if err != nil {
				panic(fmt.Sprintf("invalid collection name: %v", err)) // error includes the name
			}
			collections = append(collections, ivc)
		}
	}

	var ivs []invariant.Invariant
	for _, fn := range ConcreteExecutionType.ToInvariants(collections, zap.NewNop()) {
		ivs = append(ivs, fn(pr, domainCache))
	}
	return invariant.NewInvariantManager(ivs)
}

// concreteExecutionCustomScannerConfig resolves dynamic config for concrete executions scanner.
func concreteExecutionCustomScannerConfig(ctx shardscanner.ScannerContext) shardscanner.CustomScannerConfig {
	res := shardscanner.CustomScannerConfig{}

	if ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsScannerInvariantCollectionHistory)() {
		res[invariant.CollectionHistory.String()] = strconv.FormatBool(true)
	}
	if ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsScannerInvariantCollectionMutableState)() {
		res[invariant.CollectionMutableState.String()] = strconv.FormatBool(true)
	}
	if ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsScannerInvariantCollectionStale)() {
		res[invariant.CollectionStale.String()] = strconv.FormatBool(true)
	}

	return res
}

// concreteExecutionCustomFixerConfig resolves dynamic config for concrete executions scanner.
func concreteExecutionCustomFixerConfig(ctx shardscanner.FixerContext) shardscanner.CustomScannerConfig {
	res := shardscanner.CustomScannerConfig{}

	// unlike scanner, fixer expects keys to exist when both true and false, to differentiate from pre-config behavior
	res[invariant.CollectionHistory.String()] = strconv.FormatBool(
		ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsFixerInvariantCollectionHistory)(),
	)
	res[invariant.CollectionMutableState.String()] = strconv.FormatBool(
		ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsFixerInvariantCollectionMutableState)(),
	)
	res[invariant.CollectionStale.String()] = strconv.FormatBool(
		ctx.Config.DynamicCollection.GetBoolProperty(dynamicproperties.ConcreteExecutionsFixerInvariantCollectionStale)(),
	)

	return res
}

// ConcreteExecutionConfig configures concrete execution scanner
func ConcreteExecutionConfig(dc *dynamicconfig.Collection) *shardscanner.ScannerConfig {
	return &shardscanner.ScannerConfig{
		ScannerWFTypeName: ConcreteExecutionsScannerWFTypeName,
		FixerWFTypeName:   ConcreteExecutionsFixerWFTypeName,
		DynamicParams: shardscanner.DynamicParams{
			ScannerEnabled:          dc.GetBoolProperty(dynamicproperties.ConcreteExecutionsScannerEnabled),
			FixerEnabled:            dc.GetBoolProperty(dynamicproperties.ConcreteExecutionFixerEnabled),
			Concurrency:             dc.GetIntProperty(dynamicproperties.ConcreteExecutionsScannerConcurrency),
			PageSize:                dc.GetIntProperty(dynamicproperties.ConcreteExecutionsScannerPersistencePageSize),
			BlobstoreFlushThreshold: dc.GetIntProperty(dynamicproperties.ConcreteExecutionsScannerBlobstoreFlushThreshold),
			ActivityBatchSize:       dc.GetIntProperty(dynamicproperties.ConcreteExecutionsScannerActivityBatchSize),
			AllowDomain:             dc.GetBoolPropertyFilteredByDomain(dynamicproperties.ConcreteExecutionFixerDomainAllow),
		},
		DynamicCollection: dc,
		ScannerHooks:      concreteExecutionScannerHooks,
		FixerHooks:        concreteExecutionFixerHooks,
		StartWorkflowOptions: cclient.StartWorkflowOptions{
			ID:                           concreteExecutionsScannerWFID,
			TaskList:                     concreteExecutionsScannerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        cclient.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
		StartFixerOptions: cclient.StartWorkflowOptions{
			ID:                           concreteExecutionsFixerWFID,
			TaskList:                     concreteExecutionsFixerTaskListName,
			ExecutionStartToCloseTimeout: 20 * 365 * 24 * time.Hour,
			WorkflowIDReusePolicy:        cclient.WorkflowIDReusePolicyAllowDuplicate,
			CronSchedule:                 "*/5 * * * *",
		},
	}
}
