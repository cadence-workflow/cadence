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
	"sync/atomic"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/logging"
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
		config           *Config
	}
)

const (
	cacheNotReleased int32 = 0
	cacheReleased    int32 = 1
)

var (
	// ErrTryLock is a temporary error that is thrown by the API
	// when it loses the race to create workflow execution context
	ErrTryLock = &workflow.InternalServiceError{Message: "Failed to acquire lock, backoff and retry"}
)

func newHistoryCache(shard ShardContext, logger bark.Logger) *historyCache {
	opts := &cache.Options{}
	config := shard.GetConfig()
	opts.InitialCapacity = config.HistoryCacheInitialSize
	opts.TTL = config.HistoryCacheTTL
	opts.Pin = true

	return &historyCache{
		Cache:            cache.New(config.HistoryCacheMaxSize, opts),
		shard:            shard,
		executionManager: shard.GetExecutionManager(),
		logger: logger.WithFields(bark.Fields{
			logging.TagWorkflowComponent: logging.TagValueHistoryCacheComponent,
		}),
		config: config,
	}
}

func (c *historyCache) getOrCreateWorkflowExecution(domainID string,
	execution workflow.WorkflowExecution) (*workflowExecutionContext, releaseWorkflowExecutionFunc, error) {
	if execution.GetWorkflowId() == "" {
		return nil, nil, &workflow.BadRequestError{Message: "Can't load workflow execution.  WorkflowId not set."}
	}

	// RunID is not provided, lets try to retrieve the RunID for current active execution
	if execution.GetRunId() == "" {
		response, err := c.getCurrentExecutionWithRetry(&persistence.GetCurrentExecutionRequest{
			DomainID:   domainID,
			WorkflowID: execution.GetWorkflowId(),
		})

		if err != nil {
			return nil, nil, err
		}

		execution.RunId = common.StringPtr(response.RunID)
	} else if uuid.Parse(execution.GetRunId()) == nil { // immediately return if invalid runID
		return nil, nil, &workflow.BadRequestError{Message: "RunID is not valid UUID."}
	}

	// Test hook for disabling the cache
	if c.disabled {
		return newWorkflowExecutionContext(domainID, execution, c.shard, c.executionManager, c.logger), func(error) {}, nil
	}

	key := execution.GetRunId()
	context, cacheHit := c.Get(key).(*workflowExecutionContext)
	if !cacheHit {
		// Let's create the workflow execution context
		context = newWorkflowExecutionContext(domainID, execution, c.shard, c.executionManager, c.logger)
		elem, err := c.PutIfNotExist(key, context)
		if err != nil {
			return nil, nil, err
		}
		context = elem.(*workflowExecutionContext)
	}

	// This will create a closure on every request.
	// Consider revisiting this if it causes too much GC activity
	status := cacheNotReleased
	releaseFunc := func(err error) {
		if atomic.CompareAndSwapInt32(&status, cacheNotReleased, cacheReleased) {
			if err != nil {
				// TODO see issue #668, there are certain type or errors which can bypass the clear
				context.clear()
			}
			context.Unlock()
			c.Release(key)
		}
	}

	context.Lock()
	return context, releaseFunc, nil
}

func (c *historyCache) getCurrentExecutionWithRetry(
	request *persistence.GetCurrentExecutionRequest) (*persistence.GetCurrentExecutionResponse, error) {
	var response *persistence.GetCurrentExecutionResponse
	op := func() error {
		var err error
		response, err = c.executionManager.GetCurrentExecution(request)

		return err
	}

	err := backoff.Retry(op, persistenceOperationRetryPolicy, common.IsPersistenceTransientError)
	if err != nil {
		return nil, err
	}

	return response, nil
}
