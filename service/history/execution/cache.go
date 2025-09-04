// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination cache_mock.go

package execution

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pborman/uuid"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

// Cache is a cache that holds workflow execution context
type Cache interface {
	cache.Cache

	// GetOrCreateCurrentWorkflowExecution gets or creates workflow execution context for the current run
	GetOrCreateCurrentWorkflowExecution(
		ctx context.Context,
		domainID string,
		workflowID string,
	) (Context, ReleaseFunc, error)

	// GetAndCreateWorkflowExecution is for analyzing mutableState, it will try getting Context from cache
	// and also load from database
	GetAndCreateWorkflowExecution(
		ctx context.Context,
		domainID string,
		execution types.WorkflowExecution,
	) (Context, Context, ReleaseFunc, bool, error)

	// GetOrCreateWorkflowExecutionForBackground gets or creates workflow execution context with background context
	GetOrCreateWorkflowExecutionForBackground(
		domainID string,
		execution types.WorkflowExecution,
	) (Context, ReleaseFunc, error)

	// GetOrCreateWorkflowExecutionWithTimeout gets or creates workflow execution context with timeout
	GetOrCreateWorkflowExecutionWithTimeout(
		domainID string,
		execution types.WorkflowExecution,
		timeout time.Duration,
	) (Context, ReleaseFunc, error)

	// GetOrCreateWorkflowExecution gets or creates workflow execution context
	GetOrCreateWorkflowExecution(
		ctx context.Context,
		domainID string,
		execution types.WorkflowExecution,
	) (Context, ReleaseFunc, error)
}

type (
	// ReleaseFunc releases workflow execution context
	ReleaseFunc func(err error)

	// Cache caches workflow execution context
	cacheImpl struct {
		cache.Cache
		shard            shard.Context
		executionManager persistence.ExecutionManager
		disabled         bool
		logger           log.Logger
		metricsClient    metrics.Client
		config           *config.Config
	}
)

var (
	// NoopReleaseFn is an no-op implementation for the ReleaseFunc type
	NoopReleaseFn ReleaseFunc = func(err error) {}
)

const (
	cacheNotReleased int32 = 0
	cacheReleased    int32 = 1
)

// NewCache creates a new workflow execution context cache
func NewCache(shard shard.Context) Cache {
	opts := &cache.Options{}
	config := shard.GetConfig()
	opts.InitialCapacity = config.HistoryCacheInitialSize()
	opts.TTL = config.HistoryCacheTTL()
	opts.Pin = true
	opts.MaxCount = config.HistoryCacheMaxSize()
	opts.MetricsScope = shard.GetMetricsClient().Scope(metrics.HistoryExecutionCacheScope).Tagged(metrics.ShardIDTag(shard.GetShardID()))
	opts.Logger = shard.GetLogger().WithTags(tag.ComponentHistoryCache)
	opts.IsSizeBased = config.EnableSizeBasedHistoryExecutionCache
	opts.MaxSize = config.ExecutionCacheMaxByteSize

	return &cacheImpl{
		Cache:            cache.New(opts),
		shard:            shard,
		executionManager: shard.GetExecutionManager(),
		logger:           shard.GetLogger().WithTags(tag.ComponentHistoryCache),
		metricsClient:    shard.GetMetricsClient(),
		config:           config,
	}
}

// GetOrCreateCurrentWorkflowExecution gets or creates workflow execution context for the current run
func (c *cacheImpl) GetOrCreateCurrentWorkflowExecution(
	ctx context.Context,
	domainID string,
	workflowID string,
) (Context, ReleaseFunc, error) {

	scope := metrics.HistoryCacheGetOrCreateCurrentScope
	c.metricsClient.IncCounter(scope, metrics.CacheRequests)
	sw := c.metricsClient.StartTimer(scope, metrics.CacheLatency)
	defer sw.Stop()

	// using empty run ID as current workflow run ID
	runID := ""
	execution := types.WorkflowExecution{
		WorkflowID: workflowID,
		RunID:      runID,
	}

	return c.getOrCreateWorkflowExecutionInternal(
		ctx,
		domainID,
		execution,
		scope,
		true,
	)
}

// GetAndCreateWorkflowExecution is for analyzing mutableState, it will try getting Context from cache
// and also load from database
func (c *cacheImpl) GetAndCreateWorkflowExecution(
	ctx context.Context,
	domainID string,
	execution types.WorkflowExecution,
) (Context, Context, ReleaseFunc, bool, error) {

	scope := metrics.HistoryCacheGetAndCreateScope
	c.metricsClient.IncCounter(scope, metrics.CacheRequests)
	sw := c.metricsClient.StartTimer(scope, metrics.CacheLatency)
	defer sw.Stop()

	if err := c.validateWorkflowExecutionInfo(ctx, domainID, &execution); err != nil {
		c.metricsClient.IncCounter(scope, metrics.CacheFailures)
		return nil, nil, nil, false, err
	}

	key := definition.NewWorkflowIdentifier(domainID, execution.GetWorkflowID(), execution.GetRunID())
	contextFromCache, cacheHit := c.Get(key).(Context)
	// TODO This will create a closure on every request.
	//  Consider revisiting this if it causes too much GC activity
	releaseFunc := NoopReleaseFn
	// If cache hit, we need to lock the cache to prevent race condition
	if cacheHit {
		if err := contextFromCache.Lock(ctx); err != nil {
			// ctx is done before lock can be acquired
			c.Release(key)
			c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.CacheFailures)
			c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.AcquireLockFailedCounter)
			return nil, nil, nil, false, err
		}
		releaseFunc = c.makeReleaseFunc(key, contextFromCache, false)
	} else {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.CacheMissCounter)
	}

	// Note, the one loaded from DB is not put into cache and don't affect any behavior
	contextFromDB := NewContext(domainID, execution, c.shard, c.executionManager, c.logger)
	return contextFromCache, contextFromDB, releaseFunc, cacheHit, nil
}

// GetOrCreateWorkflowExecutionForBackground gets or creates workflow execution context with background context
// currently only used in tests
func (c *cacheImpl) GetOrCreateWorkflowExecutionForBackground(
	domainID string,
	execution types.WorkflowExecution,
) (Context, ReleaseFunc, error) {

	return c.GetOrCreateWorkflowExecution(context.Background(), domainID, execution)
}

// GetOrCreateWorkflowExecutionWithTimeout gets or creates workflow execution context with timeout
func (c *cacheImpl) GetOrCreateWorkflowExecutionWithTimeout(
	domainID string,
	execution types.WorkflowExecution,
	timeout time.Duration,
) (Context, ReleaseFunc, error) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return c.GetOrCreateWorkflowExecution(ctx, domainID, execution)
}

// GetOrCreateWorkflowExecution gets or creates workflow execution context
func (c *cacheImpl) GetOrCreateWorkflowExecution(
	ctx context.Context,
	domainID string,
	execution types.WorkflowExecution,
) (Context, ReleaseFunc, error) {

	scope := metrics.HistoryCacheGetOrCreateScope
	c.metricsClient.IncCounter(scope, metrics.CacheRequests)
	sw := c.metricsClient.StartTimer(scope, metrics.CacheLatency)
	defer sw.Stop()

	if err := c.validateWorkflowExecutionInfo(ctx, domainID, &execution); err != nil {
		c.metricsClient.IncCounter(scope, metrics.CacheFailures)
		return nil, nil, err
	}

	return c.getOrCreateWorkflowExecutionInternal(
		ctx,
		domainID,
		execution,
		scope,
		false,
	)
}

func (c *cacheImpl) getOrCreateWorkflowExecutionInternal(
	ctx context.Context,
	domainID string,
	execution types.WorkflowExecution,
	scope metrics.ScopeIdx,
	forceClearContext bool,
) (Context, ReleaseFunc, error) {

	// Test hook for disabling the cache
	if c.disabled {
		return NewContext(domainID, execution, c.shard, c.executionManager, c.logger), NoopReleaseFn, nil
	}

	key := definition.NewWorkflowIdentifier(domainID, execution.GetWorkflowID(), execution.GetRunID())
	workflowCtx, cacheHit := c.Get(key).(Context)
	if !cacheHit {
		c.metricsClient.IncCounter(scope, metrics.CacheMissCounter)
		// Let's create the workflow execution workflowCtx
		workflowCtx = NewContext(domainID, execution, c.shard, c.executionManager, c.logger)
		elem, err := c.PutIfNotExist(key, workflowCtx)
		if err != nil {
			c.metricsClient.IncCounter(scope, metrics.CacheFailures)
			return nil, nil, err
		}
		workflowCtx = elem.(Context)
	}

	// TODO This will create a closure on every request.
	//  Consider revisiting this if it causes too much GC activity
	releaseFunc := c.makeReleaseFunc(key, workflowCtx, forceClearContext)

	if err := workflowCtx.Lock(ctx); err != nil {
		// ctx is done before lock can be acquired
		c.Release(key)
		c.metricsClient.IncCounter(scope, metrics.CacheFailures)
		c.metricsClient.IncCounter(scope, metrics.AcquireLockFailedCounter)
		return nil, nil, err
	}
	return workflowCtx, releaseFunc, nil
}

func (c *cacheImpl) validateWorkflowExecutionInfo(
	ctx context.Context,
	domainID string,
	execution *types.WorkflowExecution,
) error {

	if execution.GetWorkflowID() == "" {
		return &types.BadRequestError{Message: "Can't load workflow execution.  WorkflowId not set."}
	}
	domainName, err := c.shard.GetDomainCache().GetDomainName(domainID)
	if err != nil {
		return err
	}
	// RunID is not provided, lets try to retrieve the RunID for current active execution
	if execution.GetRunID() == "" {
		response, err := c.getCurrentExecutionWithRetry(ctx, &persistence.GetCurrentExecutionRequest{
			DomainID:   domainID,
			WorkflowID: execution.GetWorkflowID(),
			DomainName: domainName,
		})

		if err != nil {
			return err
		}

		execution.RunID = response.RunID
	} else if uuid.Parse(execution.GetRunID()) == nil { // immediately return if invalid runID
		return &types.BadRequestError{Message: "RunID is not valid UUID."}
	}
	return nil
}

func (c *cacheImpl) makeReleaseFunc(
	key definition.WorkflowIdentifier,
	context Context,
	forceClearContext bool,
) func(error) {

	status := cacheNotReleased
	return func(err error) {
		defer func() {
			if atomic.CompareAndSwapInt32(&status, cacheNotReleased, cacheReleased) {
				if rec := recover(); rec != nil {
					context.Clear()
					context.Unlock()
					c.Release(key)
					panic(rec)
				} else {
					if err != nil || forceClearContext {
						// TODO see issue #668, there are certain type or errors which can bypass the clear
						context.Clear()
					}
					context.Unlock()
					c.Release(key)
				}
			}
		}()
	}
}

func (c *cacheImpl) getCurrentExecutionWithRetry(
	ctx context.Context,
	request *persistence.GetCurrentExecutionRequest,
) (*persistence.GetCurrentExecutionResponse, error) {

	c.metricsClient.IncCounter(metrics.HistoryCacheGetCurrentExecutionScope, metrics.CacheRequests)
	sw := c.metricsClient.StartTimer(metrics.HistoryCacheGetCurrentExecutionScope, metrics.CacheLatency)
	defer sw.Stop()

	var response *persistence.GetCurrentExecutionResponse
	op := func(ctx context.Context) error {
		var err error
		response, err = c.executionManager.GetCurrentExecution(ctx, request)

		return err
	}

	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(common.CreatePersistenceRetryPolicy()),
		backoff.WithRetryableError(persistence.IsTransientError),
	)
	err := throttleRetry.Do(ctx, op)
	if err != nil {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetCurrentExecutionScope, metrics.CacheFailures)
		return nil, err
	}

	return response, nil
}
