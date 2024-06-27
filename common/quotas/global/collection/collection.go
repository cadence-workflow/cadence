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

/*
Package collection contains the limiting-host ratelimit usage tracking and enforcing logic,
which acts as a quotas.Collection.

At a very high level, this wraps a [quotas.Limiter] to do a few additional things
in the context of the [github.com/uber/cadence/common/quotas/global] ratelimiter system:
  - keep track of usage per key (quotas.Limiter does not support this natively, nor should it)
  - periodically report usage to each key's "aggregator" host (batched and fanned out in parallel)
  - apply the aggregator's returned per-key RPS limits to future requests
  - fall back to the wrapped limiter in case of failures (handled internally in internal.FallbackLimiter)
*/
package collection

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/quotas/global/collection/internal"
	"github.com/uber/cadence/common/quotas/global/rpc"
	"github.com/uber/cadence/common/quotas/global/shared"
)

type (
	// Collection wraps three kinds of ratelimiter collections:
	//   1. a "global" collection, which tracks usage, and is weighted to match request patterns.
	//   2. a "local" collection, which tracks usage, and can be optionally used (and/or shadowed) instead of "global"
	//   3. a "disabled" collection, which does NOT track usage, and is used to bypass as much of this Collection as possible
	//
	// 1 is the reason this Collection exists - limiter-usage has to be tracked and submitted to aggregating hosts to
	// drive the whole "global load-balanced ratelimiter" system.  internally, this will fall back to a local collection
	// if there is insufficient data or too many errors.
	//
	// 2 is the previous non-global behavior, where ratelimits are determined locally, more simply, and load information is not shared.
	// this is a lower-cost and MUCH less complex system, and it SHOULD be used if your Cadence cluster receives requests
	// in a roughly random way (e.g. any client-side request goes to a roughly-fair roughly-random Frontend host).
	//
	// 3 disables as much of 1 and 2 as possible, and is intended to be temporary.
	// it is essentially a maximum-safety fallback during initial rollout, and should be removed once 2 is demonstrated
	// to be safe enough to use in all cases.
	//
	// And last but not least:
	// 1's local-fallback and 2 MUST NOT share ratelimiter instances, or the local instances will be double-counted when shadowing.
	// they should likely be configured to behave identically, but they need to be separate instances.
	Collection struct {
		updateInterval dynamicconfig.DurationPropertyFn

		// targetRPS is a small type-casting wrapper around dynamicconfig.IntPropertyFnWithDomainFilter
		// to prevent accidentally using the wrong key type.
		targetRPS func(lkey shared.LocalKey) int
		// keyModes is a small type-casting wrapper around dynamicconfig.StringPropertyWithRatelimitKeyFilter
		// to prevent accidentally using the wrong key type.
		keyModes func(gkey shared.GlobalKey) string

		logger log.Logger
		scope  metrics.Scope
		aggs   rpc.Client

		global   *internal.AtomicMap[shared.LocalKey, *internal.FallbackLimiter]
		local    *internal.AtomicMap[shared.LocalKey, internal.CountedLimiter]
		disabled quotas.ICollection
		km       shared.KeyMapper

		ctx       context.Context // context used for background operations, canceled when stopping
		ctxCancel func()
		stopped   chan struct{} // closed when stopping is complete

		// now exists largely for tests, elsewhere it is always time.Now
		timesource clock.TimeSource
	}

	// calls is a small temporary pair[T] container
	calls struct {
		allowed, rejected int
	}

	// basically an enum for key values.
	// when plug-in behavior is allowed, this will eventually be from a parsed string,
	// and values (aside from disabled) are not known at compile time.
	keyMode string
)

// current known values.
//
// all unknown values become modeDisabled, which should also be used for
// the default fallback in switch statements.
var (
	modeDisabled          keyMode = "disabled"
	modeLocal             keyMode = "local"
	modeGlobal            keyMode = "global"
	modeLocalShadowGlobal keyMode = "local-shadow-global"
	modeGlobalShadowLocal keyMode = "global-shadow-local"
)

const (
	// stop sending an idle key after N zero-valued updates in a row.
	// this will delete unused limiters locally, keeping collections small,
	// and also allow the aggregator to delete the key soon after.
	//
	// this has two major consequences:
	//  1. the next usage will use the local fallback, which may unfairly allow
	//     more requests than it should if other hosts are consuming the whole
	//     quota.  shorter values increase this effect.
	//  2. since zero values are submitted until garbage-collected, this host
	//     will repeatedly reduce its weight for a key until GC occurs.  if a
	//     request occurs during this window, it will take that many rounds to
	//     get back to the weight it left off at, because it is not starting from
	//     a zero state.  larger values increase this effect.
	//
	// currently, it is believed that this should be kept relatively small, to
	// get closer to the desired behavior: new frontend requests allow a small
	// number through (local fallback) until rebalanced, and idle time between
	// those requests does not very strongly penalize an abnormal-host caller
	// (e.g. leading to 0.0001 rps or similar).
	gcAfterIdle = 5 // TODO: change to time-based, like aggregator?
)

func New(
	name string,
	// quotas for "local only" behavior.
	// used for both "local" and "disabled" behavior, though "local" wraps the values.
	local quotas.ICollection,
	// quotas for the global limiter's internal fallback.
	//
	// this should be configured the same as the local collection, but it
	// MUST NOT actually be the same collection, or shadowing will double-count
	// events on the fallback.
	global quotas.ICollection,
	updateInterval dynamicconfig.DurationPropertyFn,
	targetRPS dynamicconfig.IntPropertyFnWithDomainFilter,
	keyModes dynamicconfig.StringPropertyWithRatelimitKeyFilter,
	aggs rpc.Client,
	logger log.Logger,
	met metrics.Client,
) (*Collection, error) {
	if local == global {
		return nil, errors.New("local and global collections must be different")
	}

	globalCollection := internal.NewAtomicMap(func(key shared.LocalKey) *internal.FallbackLimiter {
		return internal.NewFallbackLimiter(global.For(string(key)))
	})
	localCollection := internal.NewAtomicMap(func(key shared.LocalKey) internal.CountedLimiter {
		return internal.NewCountedLimiter(local.For(string(key)))
	})
	ctx, cancel := context.WithCancel(context.Background())
	c := &Collection{
		global:         globalCollection,
		local:          localCollection,
		disabled:       local,
		aggs:           aggs,
		updateInterval: updateInterval,
		targetRPS: func(lkey shared.LocalKey) int {
			// wrapper just ensures only local keys are used, as each
			// collection uses a separate dynamic config value.
			//
			// local keys also help ensure that "local" and "disabled"
			// use the same underlying golang.org/x/time/rate.Limiter,
			// so switching between them is practically a noop.
			return targetRPS(string(lkey))
		},

		logger: logger.WithTags(tag.ComponentGlobalRatelimiter, tag.GlobalRatelimiterCollectionName(name)),
		scope:  met.Scope(metrics.FrontendGlobalRatelimiter).Tagged(metrics.GlobalRatelimiterCollectionName(name)),
		keyModes: func(gkey shared.GlobalKey) string {
			// all collections share a single dynamic config key,
			// so they must use the global key to uniquely identify all keys.
			//
			// in the future I think we may want to move the target RPS configs
			// to use this same strategy, to keep user-facing limits together and
			// easier to notice.
			return keyModes(string(gkey))
		},
		km: shared.PrefixKey(name + ":"),

		ctx:       ctx,
		ctxCancel: cancel,
		stopped:   make(chan struct{}),

		// override externally in tests
		timesource: clock.NewRealTimeSource(),
	}
	return c, nil
}

func (c *Collection) TestOverrides(t *testing.T, timesource *clock.MockedTimeSource, km *shared.KeyMapper) {
	t.Helper()
	if timesource != nil {
		c.timesource = *timesource
	}
	if km != nil {
		c.km = *km
	}
}

// OnStart follows fx's OnStart hook semantics.
func (c *Collection) OnStart(ctx context.Context) error {
	go func() {
		defer func() {
			// bad but not worth crashing the process
			log.CapturePanic(recover(), c.logger, nil)
		}()
		defer func() {
			close(c.stopped)
		}()
		c.backgroundUpdateLoop()
	}()

	return nil
}

// OnStop follows fx's OnStop hook semantics.
func (c *Collection) OnStop(ctx context.Context) error {
	c.ctxCancel()
	// context timeout is for everything to shut down, but this one should be fast.
	// don't wait very long.
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		return fmt.Errorf("collection failed to stop, context canceled: %w", ctx.Err())
	case <-c.stopped:
		return nil
	}
}

func (c *Collection) keyMode(key shared.GlobalKey) keyMode {
	rawMode := keyMode(c.keyModes(key))
	switch rawMode {
	case modeLocal, modeGlobal, modeLocalShadowGlobal, modeGlobalShadowLocal, modeDisabled:
		c.logger.Debug("ratelimiter key mode", tag.Dynamic("key", key), tag.Dynamic("mode", rawMode))
		return rawMode
	default:
		c.logger.Error("ratelimiter key forcefully disabled, bad mode", tag.Dynamic("key", key), tag.Dynamic("mode", modeDisabled))
		return modeDisabled
	}
}

func (c *Collection) For(key string) quotas.Limiter {
	k := shared.LocalKey(key)
	gkey := c.km.LocalToGlobal(k)
	switch c.keyMode(gkey) {
	case modeLocal:
		return c.local.Load(k)
	case modeGlobal:
		return c.global.Load(k)
	case modeLocalShadowGlobal:
		return internal.NewShadowedLimiter(c.local.Load(k), c.global.Load(k))
	case modeGlobalShadowLocal:
		return internal.NewShadowedLimiter(c.global.Load(k), c.local.Load(k))

	default:
		// pass through to the fallback, as if this collection did not exist.
		// this means usage cannot be collected or shadowed, so changing to or
		// from "disable" may allow a burst of requests beyond intended limits.
		//
		// this is largely intended for safety during initial rollouts, not
		// normal use - normally "fallback" or "global" should be used.
		// "fallback" SHOULD behave the same, but with added monitoring, and
		// the ability to warm caches in either direction before switching.
		return c.disabled.For(key)
	}
}

func (c *Collection) shouldDeleteKey(mode keyMode, local bool) bool {
	if local {
		return !(mode == modeLocal || mode == modeLocalShadowGlobal || mode == modeGlobalShadowLocal)
	}
	return !(mode == modeGlobal || mode == modeLocalShadowGlobal || mode == modeGlobalShadowLocal)
}

func (c *Collection) backgroundUpdateLoop() {
	tickInterval := c.updateInterval()

	c.logger.Debug("update loop starting")

	ticker := c.timesource.NewTicker(tickInterval)
	defer ticker.Stop()

	lastGatherTime := c.timesource.Now()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.Chan():
			now := c.timesource.Now()
			c.logger.Debug("update tick")
			// update interval if it changed
			newTickInterval := c.updateInterval()
			if tickInterval != newTickInterval {
				tickInterval = newTickInterval
				ticker.Reset(newTickInterval)
			}

			// submit local metrics asynchronously, because there's no need to do it synchronously
			localMetricsDone := make(chan struct{})
			go func() {
				defer close(localMetricsDone)
				c.local.Range(func(k shared.LocalKey, v internal.CountedLimiter) bool {
					gkey := c.km.LocalToGlobal(k)
					counts := v.Collect()
					if counts.Idle > gcAfterIdle || c.shouldDeleteKey(c.keyMode(gkey), true) {
						c.logger.Debug(
							"deleting local ratelimiter",
							tag.Dynamic("local_ratelimit_key", gkey),
							tag.Dynamic("idle_count", counts.Idle > gcAfterIdle),
						)
						c.local.Delete(k)
						return true // continue iterating, possibly delete others too
					}

					c.sendMetrics(gkey, k, "local", counts)
					return true
				})
			}()

			capacity := c.global.Len()
			capacity += capacity / 10 // size may grow while we range, try to avoid reallocating in that case
			usage := make(map[shared.GlobalKey]rpc.Calls, capacity)
			startups, failings, globals := 0, 0, 0
			c.global.Range(func(k shared.LocalKey, v *internal.FallbackLimiter) bool {
				gkey := c.km.LocalToGlobal(k)
				counts, startup, failing := v.Collect()
				if counts.Idle > gcAfterIdle || c.shouldDeleteKey(c.keyMode(gkey), false) {
					c.logger.Debug(
						"deleting global ratelimiter",
						tag.GlobalRatelimiterKey(string(gkey)),
						tag.GlobalRatelimiterIdleCount(counts.Idle),
					)
					c.global.Delete(k)
					return true // continue iterating, possibly delete others too
				}

				if startup {
					startups++
				}
				if failing {
					failings++
				}
				if !(startup || failing) {
					globals++
				}
				usage[gkey] = rpc.Calls{
					Allowed:  counts.Allowed,
					Rejected: counts.Rejected,
				}
				c.sendMetrics(gkey, k, "global", counts)

				return true
			})

			// track how often we're using fallbacks vs non-fallbacks.
			// with per-host metrics this can tell us if a host is abnormal
			// compared to the rest of the cluster, e.g. persistent failing values.
			c.scope.RecordHistogramValue(metrics.GlobalRatelimiterStartupUsageHistogram, float64(startups))
			c.scope.RecordHistogramValue(metrics.GlobalRatelimiterFailingUsageHistogram, float64(failings))
			c.scope.RecordHistogramValue(metrics.GlobalRatelimiterGlobalUsageHistogram, float64(globals))

			if len(usage) > 0 {
				sw := c.scope.StartTimer(metrics.GlobalRatelimiterUpdateLatency)
				c.doUpdate(now.Sub(lastGatherTime), usage)
				sw.Stop()
			}

			<-localMetricsDone // should be much faster than doUpdate, unless it's no-opped

			lastGatherTime = now
		}
	}
}

func (c *Collection) sendMetrics(gkey shared.GlobalKey, lkey shared.LocalKey, limitType string, usage internal.UsageMetrics) {
	// emit quota information to make monitoring easier.
	// regrettably this will only be emitted when the key is (recently) in use, but
	// for active users this is probably sufficient.  other cases will probably need
	// a continual "emit all quotas" loop somewhere.
	c.scope.
		Tagged(metrics.GlobalRatelimiterKeyTag(string(gkey))).
		UpdateGauge(metrics.GlobalRatelimiterQuota, float64(c.targetRPS(lkey)))

	scope := c.scope.Tagged(
		metrics.GlobalRatelimiterKeyTag(string(gkey)),
		metrics.GlobalRatelimiterTypeTag(limitType),
	)
	scope.AddCounter(metrics.GlobalRatelimiterAllowedRequestsCount, int64(usage.Allowed))
	scope.AddCounter(metrics.GlobalRatelimiterRejectedRequestsCount, int64(usage.Rejected))
}

// doUpdate pushes usage data to aggregators. mutates `usage` as it runs.
func (c *Collection) doUpdate(since time.Duration, usage map[shared.GlobalKey]rpc.Calls) {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second) // TODO: configurable?  possibly even worth cutting off after 1s.
	defer cancel()
	res := c.aggs.Update(ctx, since, usage)
	if res.Err != nil {
		// should not happen outside pretty major errors, but may recover next time.
		c.logger.Error("aggregator update error", tag.Error(res.Err))
	}
	// either way, process all weights we did successfully retrieve.
	for gkey, weight := range res.Weights {
		delete(usage, gkey) // clean up the list so we know what was missed
		lkey, err := c.km.GlobalToLocal(gkey)
		if err != nil {
			// should not happen unless agg returns keys that were not asked for,
			// and are not for this collection
			c.logger.Error("bad global key structure returned", tag.Error(err))
			continue
		}

		if weight < 0 {
			// negative values cannot be valid, so they're a failure.
			//
			// this is largely for future-proofing and to cover all possibilities,
			// so unrecognized values lead to fallback behavior because they cannot be understood.
			c.global.Load(lkey).FailedUpdate()
		} else {
			target := float64(c.targetRPS(lkey))
			c.global.Load(lkey).Update(rate.Limit(weight * target))
		}
	}

	// mark all non-returned limits as failures.
	// this also handles request errors: all requested values are failed
	// because none of them have been deleted above.
	//
	// aside from request errors this should be rare and might represent a race
	// in the aggregator, but the semantics for handling it are pretty clear
	// either way.
	for globalkey := range usage {
		localKey, err := c.km.GlobalToLocal(globalkey)
		if err != nil {
			// should not happen, would require local mapping to be wrong
			c.logger.Error("bad global key structure in local-only path", tag.Error(err))
			continue
		}
		// requested but not returned, bump the fallback fuse
		c.global.Load(localKey).FailedUpdate()
	}
}
