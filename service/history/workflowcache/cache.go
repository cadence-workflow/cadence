// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

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

//go:generate mockgen -package=$GOPACKAGE -destination=cache_mock.go github.com/uber/cadence/service/history/workflowcache WFCache

package workflowcache

import (
	"errors"
	"time"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
)

var (
	errDomainName = errors.New("failed to get domain name from domainID")
)

// WFCache is a per workflow cache used for workflow specific in memory data
type WFCache interface {
	AllowExternal(domainID string, workflowID string) bool
	AllowInternal(domainID string, workflowID string) bool
}

type wfCache struct {
	lru                            cache.Cache
	externalLimiterFactory         quotas.LimiterFactory
	internalLimiterFactory         quotas.LimiterFactory
	workflowIDCacheExternalEnabled dynamicconfig.BoolPropertyFnWithDomainFilter
	workflowIDCacheInternalEnabled dynamicconfig.BoolPropertyFnWithDomainFilter
	domainCache                    cache.DomainCache
	metricsClient                  metrics.Client
	logger                         log.Logger
	timeSource                     clock.TimeSource
	ratelimitExternalPerWorkflowID dynamicconfig.BoolPropertyFnWithDomainFilter
	ratelimitInternalPerWorkflowID dynamicconfig.BoolPropertyFnWithDomainFilter

	// we use functions to get cache items, and the current time, so we can mock it in unit tests
	getCacheItemFn func(domainName string, workflowID string) (*cacheValue, error)
}

type cacheKey struct {
	domainName string
	workflowID string
}

type cacheValue struct {
	externalRateLimiter quotas.Limiter
	internalRateLimiter quotas.Limiter
	externalCountMetric workflowIDCountMetric
	internalCountMetric workflowIDCountMetric
}

// Params is the parameters for a new WFCache
type Params struct {
	TTL                            time.Duration
	MaxCount                       int
	ExternalLimiterFactory         quotas.LimiterFactory
	InternalLimiterFactory         quotas.LimiterFactory
	WorkflowIDCacheExternalEnabled dynamicconfig.BoolPropertyFnWithDomainFilter
	WorkflowIDCacheInternalEnabled dynamicconfig.BoolPropertyFnWithDomainFilter
	DomainCache                    cache.DomainCache
	MetricsClient                  metrics.Client
	Logger                         log.Logger
	RatelimitExternalPerWorkflowID dynamicconfig.BoolPropertyFnWithDomainFilter
	RatelimitInternalPerWorkflowID dynamicconfig.BoolPropertyFnWithDomainFilter
}

// New creates a new WFCache
func New(params Params) WFCache {
	cache := &wfCache{
		lru: cache.New(&cache.Options{
			TTL:           params.TTL,
			Pin:           false,
			MaxCount:      params.MaxCount,
			ActivelyEvict: true,
		}),
		externalLimiterFactory:         params.ExternalLimiterFactory,
		internalLimiterFactory:         params.InternalLimiterFactory,
		workflowIDCacheExternalEnabled: params.WorkflowIDCacheExternalEnabled,
		workflowIDCacheInternalEnabled: params.WorkflowIDCacheInternalEnabled,
		domainCache:                    params.DomainCache,
		metricsClient:                  params.MetricsClient,
		timeSource:                     clock.NewRealTimeSource(),
		logger:                         params.Logger,
		ratelimitExternalPerWorkflowID: params.RatelimitExternalPerWorkflowID,
		ratelimitInternalPerWorkflowID: params.RatelimitInternalPerWorkflowID,
	}
	// We set getCacheItemFn to cache.getCacheItem so that we can mock it in unit tests
	cache.getCacheItemFn = cache.getCacheItem

	return cache
}

type rateLimitType int

const (
	external rateLimitType = iota
	internal
)

func (c *wfCache) allow(domainID string, workflowID string, rateLimitType rateLimitType) bool {
	domainName, err := c.domainCache.GetDomainName(domainID)
	if err != nil {
		c.logError(domainID, workflowID, errDomainName)
		// The cache is not enabled if the domain does not exist or there is an error getting it (fail open)
		return true
	}

	if !c.isWfCacheEnabled(rateLimitType, domainName) {
		// The cache is not enabled, so we allow the call through
		return true
	}

	c.metricsClient.
		Scope(metrics.HistoryClientWfIDCacheScope).
		UpdateGauge(metrics.WorkflowIDCacheSizeGauge, float64(c.lru.Size()))

	// Locking is not needed because both getCacheItem and the rate limiter are thread safe
	value, err := c.getCacheItemFn(domainName, workflowID)
	if err != nil {
		c.logError(domainID, workflowID, err)
		// If we can't get the cache item, we should allow the request through
		return true
	}

	switch rateLimitType {
	case external:
		value.externalCountMetric.updatePerDomainMaxWFRequestCount(domainName, c.timeSource, c.metricsClient, metrics.WorkflowIDCacheRequestsExternalMaxRequestsPerSecondsTimer)
		if !value.externalRateLimiter.Allow() {
			c.emitRateLimitMetrics(
				domainID,
				workflowID,
				domainName,
				"external",
				metrics.WorkflowIDCacheRequestsExternalRatelimitedCounter,
				c.ratelimitExternalPerWorkflowID,
			)
			return false
		}
		return true
	case internal:
		value.internalCountMetric.updatePerDomainMaxWFRequestCount(domainName, c.timeSource, c.metricsClient, metrics.WorkflowIDCacheRequestsInternalMaxRequestsPerSecondsTimer)
		if !value.internalRateLimiter.Allow() {
			c.emitRateLimitMetrics(
				domainID,
				workflowID,
				domainName,
				"internal",
				metrics.WorkflowIDCacheRequestsInternalRatelimitedCounter,
				c.ratelimitInternalPerWorkflowID,
			)
			return false
		}
		return true
	default:
		// This should never happen, and we fail open
		c.logError(domainID, workflowID, errors.New("unknown rate limit type"))
		return true
	}
}

func (c *wfCache) isWfCacheEnabled(rateLimitType rateLimitType, domainName string) bool {
	return rateLimitType == external && c.workflowIDCacheExternalEnabled(domainName) ||
		rateLimitType == internal && c.workflowIDCacheInternalEnabled(domainName)
}

func (c *wfCache) emitRateLimitMetrics(
	domainID string,
	workflowID string,
	domainName string,
	callType string,
	metric int,
	enabled dynamicconfig.BoolPropertyFnWithDomainFilter,
) {
	var mode string
	if enabled(domainName) {
		mode = "enabled"
	} else {
		mode = "shadow"
	}

	c.metricsClient.Scope(
		metrics.HistoryClientWfIDCacheScope,
		metrics.DomainTag(domainName),
		metrics.ModeTag(mode),
	).IncCounter(metric)
	c.logger.Info(
		"Rate limiting workflowID",
		tag.RequestType(callType),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowDomainName(domainName),
		tag.WorkflowID(workflowID),
		tag.Mode(mode),
	)
}

// AllowExternal returns true if the rate limiter for this domain/workflow allows an external request
func (c *wfCache) AllowExternal(domainID string, workflowID string) bool {
	return c.allow(domainID, workflowID, external)
}

// AllowInternal returns true if the rate limiter for this domain/workflow allows an internal request
func (c *wfCache) AllowInternal(domainID string, workflowID string) bool {
	return c.allow(domainID, workflowID, internal)
}

func (c *wfCache) getCacheItem(domainName string, workflowID string) (*cacheValue, error) {
	// The underlying lru cache is thread safe, so there is no need to lock
	key := cacheKey{
		domainName: domainName,
		workflowID: workflowID,
	}

	value, ok := c.lru.Get(key).(*cacheValue)

	if ok {
		return value, nil
	}

	value = &cacheValue{
		externalRateLimiter: c.externalLimiterFactory.GetLimiter(domainName),
		internalRateLimiter: c.internalLimiterFactory.GetLimiter(domainName),
	}
	// PutIfNotExist is thread safe, and will either return the value that was already in the cache or the value we just created
	// another thread might have inserted a value between the Get and PutIfNotExist, but that is ok
	// it should never return an error as we do not use Pin
	valueInterface, err := c.lru.PutIfNotExist(key, value)
	if err != nil {
		return nil, err
	}

	value, ok = valueInterface.(*cacheValue)

	// This should never happen, either the value was already in the cache or we just inserted it
	if !ok {
		return nil, errors.New("Failed to insert new value into cache")
	}

	return value, err
}

func (c *wfCache) logError(domainID string, workflowID string, err error) {
	c.logger.Error("Unexpected error from workflow cache",
		tag.Error(err),
		tag.WorkflowDomainID(domainID),
		tag.WorkflowID(workflowID),
		tag.WorkflowIDCacheSize(c.lru.Size()),
	)
}
