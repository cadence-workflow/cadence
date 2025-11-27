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

package collection

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	dynamicquotas "github.com/uber/cadence/common/dynamicconfig/quotas"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/quotas/global/collection/internal"
	"github.com/uber/cadence/common/quotas/global/rpc"
	"github.com/uber/cadence/common/quotas/global/shared"
)

func TestLifecycleBasics(t *testing.T) {
	defer goleak.VerifyNone(t) // must shut down cleanly

	ctrl := gomock.NewController(t)
	logger, logs := testlogger.NewObserved(t)
	c, err := New(
		"test",
		quotas.NewCollection(quotas.NewMockLimiterFactory(ctrl)),
		quotas.NewCollection(quotas.NewMockLimiterFactory(ctrl)),
		func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Second },
		nil, // not used
		func(globalRatelimitKey string) string { return string(modeGlobal) },
		nil, // no rpc expected as there are no metrics to submit
		logger,
		metrics.NewNoopMetricsClient(),
	)
	require.NoError(t, err)
	mts := clock.NewMockedTimeSource()
	c.TestOverrides(t, &mts, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, c.OnStart(ctx))

	mts.BlockUntil(1)        // created timer in background updater
	mts.Advance(time.Second) // trigger the update
	// clockwork does not provide a way to wait for "a ticker or timer event was consumed",
	// so just sleep for a bit to allow that branch of the select statement to be followed.
	//
	// it seems *possible* for clockwork to provide this, maybe we should build it?
	time.Sleep(time.Millisecond)

	require.NoError(t, c.OnStop(ctx))

	// will fail if the tick did not begin processing, e.g. no tick or it raced with shutdown.
	// without a brief sleep after advancing time, this extremely rarely fails (try 10k tests to see)
	assert.Len(t, logs.FilterMessage("update tick").All(), 1, "no update logs, did it process the tick? all: %#v", logs.All())
}

func TestCollectionLimitersCollectMetrics(t *testing.T) {
	tests := map[string]struct {
		mode   keyMode
		local  internal.UsageMetrics
		global internal.UsageMetrics
	}{
		"disabled": {
			mode: modeDisabled,
		},
		"local": {
			mode:  modeLocal,
			local: internal.UsageMetrics{Allowed: 1},
		},
		"global": {
			mode:   modeGlobal,
			global: internal.UsageMetrics{Allowed: 1},
		},
		"local-shadow": {
			mode:   modeLocalShadowGlobal,
			local:  internal.UsageMetrics{Allowed: 1},
			global: internal.UsageMetrics{Allowed: 1},
		},
		"global-shadow": {
			mode:   modeGlobalShadowLocal,
			local:  internal.UsageMetrics{Allowed: 1},
			global: internal.UsageMetrics{Allowed: 1},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// anything non-zero
			localLimiters := dynamicquotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int {
				return 1
			})
			globalLimiters := dynamicquotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int {
				return 10
			})

			c, err := New(
				"test",
				quotas.NewCollection(localLimiters),
				quotas.NewCollection(globalLimiters),
				func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Second },
				func(domain string) int { return 5 },
				func(globalRatelimitKey string) string { return string(test.mode) },
				nil,
				testlogger.New(t),
				metrics.NewNoopMetricsClient(),
			)
			require.NoError(t, err)
			// not starting, in principle it could Collect() in the background and break this test.

			// perform one call
			_ = c.For("test").Allow()

			// check counts on limits
			globalEvents := make(map[shared.LocalKey]internal.UsageMetrics)
			c.global.Range(func(k shared.LocalKey, v *internal.FallbackLimiter) bool {
				usage, _, _ := v.Collect()
				_, ok := globalEvents[k]
				require.False(t, ok, "data key should not already exist: %v", k)
				globalEvents[k] = internal.UsageMetrics{
					Allowed:  usage.Allowed,
					Rejected: usage.Rejected,
				}
				return true
			})
			localEvents := make(map[shared.LocalKey]internal.UsageMetrics)
			c.local.Range(func(k shared.LocalKey, v internal.CountedLimiter) bool {
				_, ok := localEvents[k]
				require.False(t, ok, "data key should not already exist: %v", k)
				localEvents[k] = v.Collect()
				return true
			})

			if reflect.ValueOf(test.global).IsZero() {
				assert.Empty(t, globalEvents, "unexpected global events, should be none")
			} else {
				assert.Equal(t, map[shared.LocalKey]internal.UsageMetrics{"test": test.global}, globalEvents, "incorrect global metrics")
			}
			if reflect.ValueOf(test.local).IsZero() {
				assert.Empty(t, localEvents, "unexpected local events, should be none")
			} else {
				assert.Equal(t, map[shared.LocalKey]internal.UsageMetrics{"test": test.local}, localEvents, "incorrect local metrics")
			}
		})
	}
}

func TestCollectionSubmitsDataAndUpdates(t *testing.T) {
	defer goleak.VerifyNone(t) // must shut down cleanly

	ctrl := gomock.NewController(t)
	// anything non-zero
	limiters := dynamicquotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int {
		return 1
	})
	logger, observed := testlogger.NewObserved(t)
	aggs := rpc.NewMockClient(ctrl)
	c, err := New(
		"test",
		quotas.NewCollection(limiters),
		quotas.NewCollection(limiters),
		func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Second },
		func(domain string) int { return 10 }, // target 10 rps / one per 100ms
		func(globalRatelimitKey string) string { return string(modeGlobal) },
		aggs,
		logger,
		metrics.NewNoopMetricsClient(),
	)
	require.NoError(t, err)
	mts := clock.NewMockedTimeSource()
	c.TestOverrides(t, &mts, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, c.OnStart(ctx))

	// generate some data.
	// these start with the collections' limits, i.e. 1 token, so only one request is allowed.
	someLimiter := c.For("something")
	res := someLimiter.Reserve()
	assert.True(t, res.Allow(), "first request should have been allowed")
	res.Used(true)
	assert.False(t, someLimiter.Allow(), "second request on the same domain should have been rejected")
	assert.NoError(t, c.For("other").Wait(ctx), "request on a different domain should be allowed")

	// all limiters are now drained, and will take ~1s to recover normally.

	// prep for the calls
	called := make(chan struct{}, 1)
	aggs.EXPECT().Update(gomock.Any(), gomock.Any(), map[shared.GlobalKey]rpc.Calls{
		"test:other": {
			Allowed:  1,
			Rejected: 0,
		},
		"test:something": {
			Allowed:  1,
			Rejected: 1,
		},
	}).DoAndReturn(func(ctx context.Context, period time.Duration, load map[shared.GlobalKey]rpc.Calls) rpc.UpdateResult {
		called <- struct{}{}
		return rpc.UpdateResult{
			Weights: map[shared.GlobalKey]rpc.UpdateEntry{
				"test:something": {Weight: 1, UsedRPS: 2}, // should recover a token in 100ms
				// "test:other":   // not returned, should not change weight/rps and stay at 1s
			},
			Err: nil,
		}
	})

	mts.BlockUntil(1)        // need to have created timer in background updater
	mts.Advance(time.Second) // trigger the update.  also fills all ratelimiters.

	// wait until the calls occur
	select {
	case <-called:
	case <-time.After(time.Second / 2):
		// keep total wait shorter than 1s to avoid refilling the slow token, just in case
		t.Fatal("did not make an rpc call after 1/2s")
	}
	// crash if more calls occur
	close(called)

	// wait for the updates to be sent to the ratelimiters, and for at least one "something"'s 100ms token to recover
	time.Sleep(150 * time.Millisecond)

	// and make sure updates occurred
	assert.False(t, c.For("other").Allow(), "should be no recovered tokens yet on the slow limit")  // less than 1 second == no tokens
	assert.True(t, c.For("something").Allow(), "should have allowed one request on the fast limit") // over 100ms (got updated rate) == at least one token
	// specifically: because this is the first update to this limiter, it should now allow 10 requests, because the token bucket should be full.
	// just check once though, no need to be precise here.
	assert.True(t, c.For("something").Allow(), "after the initial update, the fast limiter should have extra tokens available")

	assert.NoError(t, c.OnStop(ctx))

	// and make sure no non-fatal errors were logged
	for _, log := range observed.All() {
		t.Logf("observed log: %v", log.Message)
		assert.Lessf(t, log.Level, zapcore.ErrorLevel, "should not be any error logs, got: %v", log.Message)
	}
}

func TestTogglingMode(t *testing.T) {
	defer goleak.VerifyNone(t)
	aggs := rpc.NewMockClient(gomock.NewController(t))
	var mode atomic.Value
	mode.Store(modeDisabled)
	c, err := New(
		"test",
		quotas.NewCollection(dynamicquotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int { return 1 })),
		quotas.NewCollection(dynamicquotas.NewSimpleDynamicRateLimiterFactory(func(domain string) int { return 1 })),
		func(opts ...dynamicproperties.FilterOption) time.Duration { return time.Second }, // update every second
		func(domain string) int { return 1 },
		func(globalRatelimitKey string) string { return string(mode.Load().(keyMode)) },
		aggs,
		testlogger.New(t),
		metrics.NewNoopMetricsClient(),
	)
	require.NoError(t, err)
	ts := clock.NewMockedTimeSource()
	c.TestOverrides(t, &ts, nil)

	require.NoError(t, c.OnStart(context.Background()), "failed to start")
	defer func() {
		// reduces goroutine-noise on failure
		require.NoError(t, c.OnStop(context.Background()), "failed to stop")
	}()

	ts.BlockUntil(1) // background ticker created

	// must tick "incrementally" or updates may be lost unrealistically
	tick := func(n int) {
		for i := 0; i < n; i++ {
			ts.Advance(time.Second)
			time.Sleep(10 * time.Millisecond) // allow code time to run to get back to waiting
		}
	}

	// a bit of shenaniganry:
	// use the same mock for all the sub-tests, but don't make any assertions on the number of calls.
	// if you do, it "blames" the parent test rather than the sub-test, which is hard to debug.
	//
	// this could be resolved by rerunning all steps <N for the Nth sub-test, but that's a fair bit
	// more boilerplate.
	remainingCalls := atomic.NewInt64(int64(0))
	aggs.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, time.Duration, map[shared.GlobalKey]rpc.Calls) rpc.UpdateResult {
			remainingCalls.Sub(1)
			// empty because this test doesn't change if global chooses to use its internal fallback
			return rpc.UpdateResult{}
		}).
		AnyTimes()
	// enforce actual call counts uniquely per test
	expectUpdateCall := func(t *testing.T, times int) {
		previousBudget := remainingCalls.Swap(int64(times))
		require.Zero(t, previousBudget, "should not have any remaining call budget before making an expectation, have %v and tried to add %v", previousBudget, times)
		t.Cleanup(func() {
			called := remainingCalls.Load()
			assert.Zero(t, called, "incorrect number of update calls performed, current budget is %v out of %v", called, times)
		})
	}

	t.Run("disabled should not make any internal limiters or update calls", func(t *testing.T) {
		mode.Store(modeDisabled)
		c.For("key").Allow()
		assert.Equal(t, c.local.Len(), 0, "local collection should be empty when disabled")
		assert.Equal(t, c.global.Len(), 0, "global collection should be empty when disabled")

		assertLimiterKeys(t, c.local, "local")
		assertLimiterKeys(t, c.global, "global")
		tick(1) // ensure no update calls occur
	})
	require.False(t, t.Failed(), "stopping early, already failed")
	t.Run("local creates only local keys", func(t *testing.T) {
		mode.Store(modeLocal)
		c.For("key").Allow()
		assertLimiterKeys(t, c.local, "local", "key")
		assertLimiterKeys(t, c.global, "global") // not in a global-accessing mode
		tick(1)                                  // ensure no update calls occur
	})
	require.False(t, t.Failed(), "stopping early, already failed")
	t.Run("local keys GC", func(t *testing.T) {
		tick(1)
		// not garbage-collected immediately
		assertLimiterKeys(t, c.local, "local", "key")
		tick(1)
		assertLimiterKeys(t, c.local, "local", "key")
		tick(10)

		assertLimiterKeys(t, c.local, "local")
		assertLimiterKeys(t, c.global, "global")
	})
	require.False(t, t.Failed(), "stopping early, already failed")
	t.Run("global-local creates both keys", func(t *testing.T) {
		mode.Store(modeLocalShadowGlobal)
		c.For("key").Allow()
		assertLimiterKeys(t, c.local, "local", "key")
		assertLimiterKeys(t, c.global, "global", "key")

		expectUpdateCall(t, 1) // global-touching mode should send an update request
		tick(1)
	})
	require.False(t, t.Failed(), "stopping early, already failed")
	t.Run("collections GC separately", func(t *testing.T) {
		// must start with data in both
		assertLimiterKeys(t, c.local, "local", "key")
		assertLimiterKeys(t, c.global, "global", "key")

		// switch modes so only global is retained
		mode.Store(modeGlobal)
		expectUpdateCall(t, 1) // global mode should send an update request
		tick(1)
		assertLimiterKeys(t, c.local, "local") // GC'd because it's no longer a local-touching mode
		assertLimiterKeys(t, c.global, "global", "key")
	})
	require.False(t, t.Failed(), "stopping early, already failed")
	t.Run("global keys GC", func(t *testing.T) {
		// sanity check
		assertLimiterKeys(t, c.local, "local") // GC'd because it's no longer a local-touching mode
		assertLimiterKeys(t, c.global, "global", "key")

		// should send ~4 more updates (5 after above), then delete data and stop updating.
		// exact number doesn't matter, just that it's less than the number of
		// ticks, to show that empty means "no update calls"
		expectUpdateCall(t, 4)
		tick(10)
		assertLimiterKeys(t, c.local, "local")
		assertLimiterKeys(t, c.global, "global")
	})
	t.Run("disabling stops updates and GCs immediately", func(t *testing.T) {
		mode.Store(modeGlobalShadowLocal)
		c.For("key").Allow()
		// sanity check
		assertLimiterKeys(t, c.local, "local", "key")
		assertLimiterKeys(t, c.global, "global", "key")

		mode.Store(modeDisabled)
		// should not cause an update despite global keys existing,
		// disabled mode should delete them before sending
		tick(1)
		// should be emptied in a single tick
		assertLimiterKeys(t, c.local, "local")
		assertLimiterKeys(t, c.global, "global")
	})
}

func TestBoostRPS(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		targetLimit, fallbackLimit float64
		weight                     float64
		usedRPS                    float64
		resultLimit                float64
	}{
		"low weight fully used": {
			targetLimit:   10,
			fallbackLimit: 3,
			weight:        0.11,
			usedRPS:       10,
			resultLimit:   1.1, // no unused RPS free for boost, gets fair value
		},
		"low weight mostly unused": {
			targetLimit:   10,
			fallbackLimit: 3,
			weight:        0.11,
			usedRPS:       2,
			resultLimit:   3, // min(1.1 + 8, 3)
		},
		"high weight fully used": {
			targetLimit:   10,
			fallbackLimit: 3,
			weight:        0.89,
			usedRPS:       10,
			resultLimit:   8.9, // no unused RPS free for boost, gets fair value
		},
		"high weight mostly unused": {
			targetLimit:   10,
			fallbackLimit: 3,
			weight:        0.89,
			usedRPS:       2,
			resultLimit:   8.9, // should not be boosted beyond fair, nor limited below it
		},
		"usage over target": {
			targetLimit:   10,
			fallbackLimit: 3,
			weight:        0.089,
			usedRPS:       100,
			resultLimit:   0.89, // should be weight-based at worst, and not be reduced by excess usage
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.GreaterOrEqual(t, tc.weight, float64(0), "sanity check on weight")
			assert.LessOrEqual(t, tc.weight, float64(1), "sanity check on weight")
			result := boostRPS(rate.Limit(tc.targetLimit), rate.Limit(tc.fallbackLimit), tc.weight, tc.usedRPS)
			assert.InDeltaf(t, tc.resultLimit, float64(result), 0.01, "computed RPS does not match closely enough, should be %0.2f", tc.resultLimit)
		})
	}
}

func assertLimiterKeys[V any](t *testing.T, collection *internal.AtomicMap[shared.LocalKey, V], name string, keys ...shared.LocalKey) {
	t.Helper()
	found := map[shared.LocalKey]struct{}{}
	collection.Range(func(k shared.LocalKey, v V) bool {
		found[k] = struct{}{}
		return true
	})
	keyMap := map[shared.LocalKey]struct{}{}
	for _, k := range keys {
		keyMap[k] = struct{}{}
	}
	assert.Equal(t, keyMap, found, "unexpected keys in %v collection", name)
}
