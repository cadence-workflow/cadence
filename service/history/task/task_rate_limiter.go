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

//go:generate mockgen -package $GOPACKAGE -destination task_rate_limiter_mock.go github.com/uber/cadence/service/history/task RateLimiter

package task

import (
	"context"

	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/dynamicconfig"
	dynamicquotas "github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/shard"
)

type (
	RateLimiter interface {
		Allow(Task) bool
		Wait(context.Context, Task) error
	}

	taskRateLimiterImpl struct {
		logger           log.Logger
		metricsScope     metrics.Scope
		domainCache      cache.DomainCache
		limiters         quotas.ICollection
		enabled          dynamicconfig.BoolPropertyFn
		enableShadowMode dynamicconfig.BoolPropertyFnWithDomainFilter
	}
)

func NewRateLimiter(
	logger log.Logger,
	metricsClient metrics.Client,
	domainCache cache.DomainCache,
	config *config.Config,
	controller shard.Controller,
) RateLimiter {
	rps := func(domain string) int {
		totalShards := float64(config.NumberOfShards)
		totalRPS := float64(config.TaskSchedulerGlobalDomainRPS(domain))
		numShards := float64(controller.NumShards())
		return int(totalRPS * numShards / totalShards)
	}
	limiterFactory := dynamicquotas.NewSimpleDynamicRateLimiterFactory(rps)
	return &taskRateLimiterImpl{
		logger:           logger,
		metricsScope:     metricsClient.Scope(metrics.TaskSchedulerRateLimiterScope),
		domainCache:      domainCache,
		enabled:          config.TaskSchedulerEnableRateLimiter,
		enableShadowMode: config.TaskSchedulerEnableRateLimiterShadowMode,
		limiters:         quotas.NewCollection(limiterFactory),
	}
}

func (r *taskRateLimiterImpl) Allow(t Task) bool {
	if !r.enabled() {
		return true
	}
	limiter, scope, shadow := r.getLimiter(t)
	allow := limiter.Allow()
	if allow {
		scope.IncCounter(metrics.TaskSchedulerAllowedCounterPerDomain)
		return true
	}
	scope.IncCounter(metrics.TaskSchedulerThrottledCounterPerDomain)
	return shadow
}

func (r *taskRateLimiterImpl) Wait(ctx context.Context, t Task) error {
	if !r.enabled() {
		return nil
	}
	limiter, _, _ := r.getLimiter(t)
	// wait has kinda complicated semantics and we haven't really used it and it's not super well understood
	// the current interface of rate limiter doesn't support shadow mode, so we are not supporting shadow mode for Wait method now
	// Besides, this code path is not hit in production right now
	return limiter.Wait(ctx)
}

func (r *taskRateLimiterImpl) getLimiter(t Task) (quotas.Limiter, metrics.Scope, bool) {
	domainID := t.GetDomainID()
	domainName, err := r.domainCache.GetDomainName(domainID)
	if err != nil {
		r.logger.Warn("failed to get domain name from domain cache", tag.WorkflowDomainID(domainID), tag.Error(err))
	}
	return r.limiters.For(domainName), r.metricsScope.Tagged(metrics.DomainTag(domainName)), r.enableShadowMode(domainName)
}
