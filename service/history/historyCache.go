// Copyright (c) 2017 Uber Technologies, Inc.
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

package history

import (
	"context"
	"sync/atomic"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/logging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"

	"github.com/pborman/uuid"
	"github.com/uber-common/bark"
	"github.com/uber/cadence/common"
)

type (
	releaseWorkflowExecutionFunc func(err error)

	historyCache struct {
		cache.Cache
		shard            ShardContext
		executionManager persistence.ExecutionManager
		disabled         bool
		logger           bark.Logger
		metricsClient    metrics.Client
		config           *Config
	}
)

const (
	cacheNotReleased int32 = 0
	cacheReleased    int32 = 1
)

func newHistoryCache(shard ShardContext) *historyCache {
	opts := &cache.Options{}
	config := shard.GetConfig()
	opts.InitialCapacity = config.HistoryCacheInitialSize()
	opts.TTL = config.HistoryCacheTTL()
	opts.Pin = true

	return &historyCache{
		Cache:            cache.New(config.HistoryCacheMaxSize(), opts),
		shard:            shard,
		executionManager: shard.GetExecutionManager(),
		logger: shard.GetLogger().WithFields(bark.Fields{
			logging.TagWorkflowComponent: logging.TagValueHistoryCacheComponent,
		}),
		metricsClient: shard.GetMetricsClient(),
		config:        config,
	}
}

func (c *historyCache) getOrCreateWorkflowExecution(domainID string,
	execution workflow.WorkflowExecution) (workflowExecutionContext, releaseWorkflowExecutionFunc, error) {
	return c.getOrCreateWorkflowExecutionWithTimeout(context.Background(), domainID, execution)
}

func (c *historyCache) validateWorkflowExecutionInfo(domainID string, execution *workflow.WorkflowExecution) error {
	if execution.GetWorkflowId() == "" {
		return &workflow.BadRequestError{Message: "Can't load workflow execution.  WorkflowId not set."}
	}

	// RunID is not provided, lets try to retrieve the RunID for current active execution
	if execution.GetRunId() == "" {
		response, err := c.getCurrentExecutionWithRetry(&persistence.GetCurrentExecutionRequest{
			DomainID:   domainID,
			WorkflowID: execution.GetWorkflowId(),
		})

		if err != nil {
			return err
		}

		execution.RunId = common.StringPtr(response.RunID)
	} else if uuid.Parse(execution.GetRunId()) == nil { // immediately return if invalid runID
		return &workflow.BadRequestError{Message: "RunID is not valid UUID."}
	}
	return nil
}

// For analyzing mutableState, we have to try get workflowExecutionContext from cache and also load from database
func (c *historyCache) getAndCreateWorkflowExecutionWithTimeout(ctx context.Context, domainID string,
	execution workflow.WorkflowExecution) (workflowExecutionContext, workflowExecutionContext,
	releaseWorkflowExecutionFunc, bool, error) {
	c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.HistoryCacheRequests)
	sw := c.metricsClient.StartTimer(metrics.HistoryCacheGetAndCreateScope, metrics.HistoryCacheLatency)
	defer sw.Stop()

	if err := c.validateWorkflowExecutionInfo(domainID, &execution); err != nil {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.HistoryCacheFailures)
		return nil, nil, nil, false, err
	}

	key := newWorkflowIdentifier(domainID, &execution)
	contextFromCache, cacheHit := c.Get(key).(workflowExecutionContext)
	releaseFunc := func(error) {}
	// If cache hit, we need to lock the cache to prevent race condition
	if cacheHit {
		if err := contextFromCache.lock(ctx); err != nil {
			// ctx is done before lock can be acquired
			c.Release(key)
			c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.HistoryCacheFailures)
			c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.AcquireLockFailedCounter)
			return nil, nil, nil, false, err
		}
		releaseFunc = c.makeReleaseFunc(key, cacheNotReleased, contextFromCache)
	} else {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetAndCreateScope, metrics.CacheMissCounter)
	}

	// Note, the one loaded from DB is not put into cache and don't affect any behavior
	contextFromDB := newWorkflowExecutionContext(domainID, execution, c.shard, c.executionManager, c.logger)
	return contextFromCache, contextFromDB, releaseFunc, cacheHit, nil
}

func (c *historyCache) getOrCreateWorkflowExecutionWithTimeout(ctx context.Context, domainID string,
	execution workflow.WorkflowExecution) (workflowExecutionContext, releaseWorkflowExecutionFunc, error) {
	c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.HistoryCacheRequests)
	sw := c.metricsClient.StartTimer(metrics.HistoryCacheGetOrCreateScope, metrics.HistoryCacheLatency)
	defer sw.Stop()

	if err := c.validateWorkflowExecutionInfo(domainID, &execution); err != nil {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.HistoryCacheFailures)
		return nil, nil, err
	}

	// Test hook for disabling the cache
	if c.disabled {
		return newWorkflowExecutionContext(domainID, execution, c.shard, c.executionManager, c.logger), func(error) {}, nil
	}

	key := newWorkflowIdentifier(domainID, &execution)
	workflowCtx, cacheHit := c.Get(key).(workflowExecutionContext)
	if !cacheHit {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.CacheMissCounter)
		// Let's create the workflow execution workflowCtx
		workflowCtx = newWorkflowExecutionContext(domainID, execution, c.shard, c.executionManager, c.logger)
		elem, err := c.PutIfNotExist(key, workflowCtx)
		if err != nil {
			c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.HistoryCacheFailures)
			return nil, nil, err
		}
		workflowCtx = elem.(workflowExecutionContext)
	}

	// This will create a closure on every request.
	// Consider revisiting this if it causes too much GC activity
	releaseFunc := c.makeReleaseFunc(key, cacheNotReleased, workflowCtx)

	if err := workflowCtx.lock(ctx); err != nil {
		// ctx is done before lock can be acquired
		c.Release(key)
		c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.HistoryCacheFailures)
		c.metricsClient.IncCounter(metrics.HistoryCacheGetOrCreateScope, metrics.AcquireLockFailedCounter)
		return nil, nil, err
	}
	return workflowCtx, releaseFunc, nil
}

func (c *historyCache) makeReleaseFunc(key workflowIdentifier, status int32, context workflowExecutionContext) func(error) {
	return func(err error) {
		if atomic.CompareAndSwapInt32(&status, cacheNotReleased, cacheReleased) {
			if err != nil {
				// TODO see issue #668, there are certain type or errors which can bypass the clear
				context.clear()
			}
			context.unlock()
			c.Release(key)
		}
	}
}

func (c *historyCache) getCurrentExecutionWithRetry(
	request *persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error) {
	c.metricsClient.IncCounter(metrics.HistoryCacheGetCurrentExecutionScope, metrics.HistoryCacheRequests)
	sw := c.metricsClient.StartTimer(metrics.HistoryCacheGetCurrentExecutionScope, metrics.HistoryCacheLatency)
	defer sw.Stop()

	var response *persistence.GetCurrentExecutionResponse
	op := func() error {
		var err error
		response, err = c.executionManager.GetCurrentExecution(request)

		return err
	}

	err := backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
	if err != nil {
		c.metricsClient.IncCounter(metrics.HistoryCacheGetCurrentExecutionScope, metrics.HistoryCacheFailures)
		return nil, err
	}

	return response, nil
}
