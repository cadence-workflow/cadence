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

package algorithm

import (
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/multierr"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/quotas/global/shared"
)

// just simplifies newForTest usage as most tests only care about rate
func defaultConfig(rate time.Duration) configSnapshot {
	return configSnapshot{
		// now:    , ignored

		// intentionally avoiding 0.5 because it cannot tell if the code uses
		// `weight` or `1-weight`, which is usually relevant.
		//
		// 0.1 is relatively human-math-friendly for a single step,
		// but is otherwise arbitrary.
		weight:     0.1,
		rate:       rate,
		decayAfter: 2 * rate,
		gcAfter:    10 * rate,
	}
}

func newValid(t testlogger.TestingT, snap configSnapshot) (*impl, clock.MockedTimeSource) {
	return newForTest(t, snap, true)
}

func newForTest(t testlogger.TestingT, snap configSnapshot, validate bool) (*impl, clock.MockedTimeSource) {
	cfg := Config{
		NewDataWeight: func(_ ...dynamicproperties.FilterOption) float64 {
			return snap.weight
		},
		UpdateInterval: func(_ ...dynamicproperties.FilterOption) time.Duration {
			return snap.rate
		},
		DecayAfter: func(_ ...dynamicproperties.FilterOption) time.Duration {
			return snap.decayAfter
		},
		GcAfter: func(_ ...dynamicproperties.FilterOption) time.Duration {
			return snap.gcAfter
		},
	}
	var agg *impl

	if validate {
		l, obs := testlogger.NewObserved(t)
		t.Cleanup(func() {
			shared.AssertNoSanityCheckFailures(t, obs.TakeAll())
		})
		i, err := New(metrics.NewNoopMetricsClient(), l, cfg)
		require.NoError(t, err)
		agg = i.(*impl)
	} else {
		// need to build by hand, New returns nil on err
		agg = &impl{
			cfg:   cfg,
			scope: metrics.NewNoopMetricsClient().Scope(metrics.GlobalRatelimiterAggregator),
			usage: make(map[Limit]map[Identity]requests),
			clock: nil,
		}
	}

	underlying := agg
	tick := clock.NewMockedTimeSource()
	underlying.clock = tick

	// adjust time to get rid of sub-second output, it's just harder to read.
	// doesn't matter if this goes forward or backward.
	tick.Advance(tick.Now().Sub(tick.Now().Round(time.Second)))

	return underlying, tick
}

func TestEmitsMetrics(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			t.Log("This test is sensitive about bucket sizes, but they aren't actually important.")
			t.Log("If bucket sizes have changed, just update the test to create enough data / use the new values, so each step is unique")
		}
	})

	assertHistogramContents := func(name string, snap tally.HistogramSnapshot, expected map[float64]int64) {
		t.Helper()
		for bucket, val := range snap.Values() { // {bucket_boundary: value}
			assert.Equal(t, expected[bucket], val, "bucket %v has unexpected value for histogram name %v", bucket, name)
		}
	}
	assertAllHistogramContents := func(snap tally.Snapshot, contents map[string]map[float64]int64) {
		t.Helper()
		for _, data := range snap.Histograms() { // {"full.name+tags":{tags, values, etc}}
			name := strings.TrimPrefix(data.Name(), "test.global_ratelimiter_") // common prefix for this test
			exp, ok := contents[name]
			if !ok {
				// keys remain between snapshots, so they're a bit of a pain.
				// values are zeroed by each snapshot though.
				for bucket, val := range data.Values() {
					assert.Zerof(t, val, "ignored key %v (trimmed: %v) in bucket %v has non-zero value, cannot be ignored", data.Name(), name, bucket)
				}
			} else {
				assertHistogramContents(name, data, exp)
			}
		}
	}

	agg, _ := newValid(t, defaultConfig(time.Second))
	ts := tally.NewTestScope("test", nil)
	agg.scope = metrics.NewClient(ts, metrics.History, metrics.HistogramMigration{}).Scope(metrics.GlobalRatelimiterAggregator)

	h1, h2 := Identity("host 1"), Identity("host 2")
	key := Limit("key")

	err := agg.Update(UpdateParams{
		ID:      h1,
		Load:    map[Limit]Requests{key: {1, 1}},
		Elapsed: time.Second,
	})
	require.NoError(t, err)
	snap := ts.Snapshot()
	assertAllHistogramContents(snap, map[string]map[float64]int64{
		"initialized":   {1: 1}, // one key was created
		"reinitialized": {0: 1},
		"updated":       {0: 1},
		"decayed":       {0: 1},
	})

	err = agg.Update(UpdateParams{
		ID:      h2,
		Load:    map[Limit]Requests{key: {1, 1}},
		Elapsed: time.Second,
	})
	require.NoError(t, err)
	snap = ts.Snapshot()
	assertAllHistogramContents(snap, map[string]map[float64]int64{
		"initialized":   {1: 1}, // keys are disjoint, so another key was created
		"reinitialized": {0: 1},
		"updated":       {0: 1},
		"decayed":       {0: 1},
	})

	err = agg.Update(UpdateParams{
		ID:      h1,
		Load:    map[Limit]Requests{key: {1, 1}},
		Elapsed: time.Second,
	})
	require.NoError(t, err)
	snap = ts.Snapshot()
	assertAllHistogramContents(snap, map[string]map[float64]int64{
		"initialized":   {0: 1},
		"reinitialized": {0: 1},
		"updated":       {1: 1}, // h1 was updated
		"decayed":       {0: 1},
	})

	_, err = agg.HostUsage(h1, []Limit{key})
	require.NoError(t, err)
	snap = ts.Snapshot()
	assertAllHistogramContents(snap, map[string]map[float64]int64{
		"limits_queried":      {1: 1}, // one limit exists
		"host_limits_queried": {2: 1}, // two hosts have data for that limit
		"removed_limits":      {0: 1}, // none removed
		"removed_host_limits": {0: 1}, // none removed
	})
}

func TestMissedUpdateHandling(t *testing.T) {
	agg, tick := newValid(t, configSnapshot{
		weight:     0.1,
		rate:       time.Second,
		decayAfter: 2 * time.Second,
		gcAfter:    10 * time.Second,
	})

	h1, h2 := Identity("host 1"), Identity("host 2")
	key := Limit("key")
	err := agg.Update(UpdateParams{
		ID:      h1,
		Load:    map[Limit]Requests{key: {1, 1}},
		Elapsed: time.Second,
	})
	require.NoError(t, err)
	err = agg.Update(UpdateParams{
		ID:      h2,
		Load:    map[Limit]Requests{key: {1, 1}},
		Elapsed: time.Second,
	})
	require.NoError(t, err)

	// sanity-check the initial values
	usage, err := agg.HostUsage(h1, []Limit{key})
	require.NoError(t, err)
	assert.Len(t, usage, 1)                        // 1 key known, should always be true
	assert.Equal(t, PerSecond(2), usage[key].Used) // only 2 accepted requests

	// move to 1 second later == expected update rate.
	// should still match the original values as it's not excessively delayed.
	tick.Advance(time.Second)
	usage, err = agg.HostUsage(h1, []Limit{key})
	assert.Len(t, usage, 1)
	assert.Equal(t, PerSecond(2), usage[key].Used) // still 2 allowed

	// advance to 1.5 seconds: over the expected rate but not beyond decay-after,
	// which should still not change anything.
	tick.Advance(time.Second / 2)
	usage, err = agg.HostUsage(h1, []Limit{key})
	assert.Len(t, usage, 1)
	assert.Equal(t, PerSecond(2), usage[key].Used) // still 2 allowed

	// advance another second, to 2.5s total.
	// exactly 2s and 3s are being avoided because nanosecond-equal times are highly unlikely,
	// and the behavior at that time doesn't actually matter, so it doesn't matter if it changes.
	//
	// 2.5s is beyond decayAfter (2s) so it should retroactively count missed updates,
	// immediately freeing up RPS as if it was tracking 0s all along, because we're assuming
	// it has been inactive for some reason.
	//
	// 2 full updates have been missed, so: 2 => 1.8 => 1.62
	tick.Advance(time.Second)
	usage, err = agg.HostUsage(h1, []Limit{key})
	assert.Len(t, usage, 1)
	assert.Equal(t, PerSecond(1.62), usage[key].Used) // reduced from 2

	// advance to 3.5 seconds, for 3 total missed updates:
	// 2 => 1.8 => 1.62 => 1.4580000000000002
	tick.Advance(time.Second)
	usage, err = agg.HostUsage(h1, []Limit{key})
	assert.Len(t, usage, 1)                                    // still tracking the key / not GC'd
	assert.InDelta(t, float64(usage[key].Used), 1.458, 0.0001) // further reduced
}

func TestGC(t *testing.T) {
	h1, h2 := Identity("host 1"), Identity("host 2")
	key := Limit("key")

	// creates an aggregator and advances time 9 seconds, ensuring that data still exists.
	// advance 1 more second to trigger garbage collection.
	// this moves slightly beyond 9s to avoid testing the precise boundary time, as it's not relevant.
	setup := func(t *testing.T) (*impl, clock.MockedTimeSource) {
		agg, tick := newValid(t, configSnapshot{
			rate:    time.Second,
			gcAfter: 10 * time.Second,

			// irrelevant for these tests but must be non-zero:
			weight:     0.1,
			decayAfter: 2 * time.Second,
		})

		err := agg.Update(UpdateParams{ID: h1, Load: map[Limit]Requests{key: {1, 1}}, Elapsed: time.Second})
		require.NoError(t, err)
		err = agg.Update(UpdateParams{ID: h2, Load: map[Limit]Requests{key: {1, 1}}, Elapsed: time.Second})
		require.NoError(t, err)
		usage, err := agg.HostUsage(h1, []Limit{key})
		require.NoError(t, err)
		// sanity check that we have data
		require.Len(t, usage, 1, "sanity check: should have inserted limit's data")
		require.Equal(t, usage[key].Used, PerSecond(2), "sanity check: should have inserted usage data")

		// partially advance, sanity check.
		tick.Advance(9*time.Second + (time.Second / 10))
		usage, err = agg.HostUsage(h1, []Limit{key})
		require.Len(t, usage, 1, "sanity check: should have inserted limit's data after 9s")
		require.Equal(t, usage[key].Used, PerSecond(2*math.Pow(0.9, 9)), "sanity check: should have inserted usage data, reduced after 9s")

		return agg, tick
	}

	t.Run("no cleanup before expiration", func(t *testing.T) {
		agg, _ := setup(t)
		met, err := agg.GC()
		require.NoError(t, err)
		require.Equal(t, Metrics{
			HostLimits:        2,
			Limits:            1,
			RemovedHostLimits: 0,
			RemovedLimits:     0,
		}, met)
	})

	t.Run("cleans up during read", func(t *testing.T) {
		agg, tick := setup(t)

		tick.Advance(time.Second) // advance to 10th second
		// read it out, should detect out-of-date data and clean it up
		usage, err := agg.HostUsage(h1, []Limit{key})
		require.NoError(t, err)
		require.Len(t, usage, 0, "should be no data for h1")

		// internals should also be empty
		require.Len(t, agg.usage, 0) // also frees memory
	})
	t.Run("retains recent data while cleaning", func(t *testing.T) {
		agg, tick := setup(t)

		// refresh data for one host
		err := agg.Update(UpdateParams{ID: h1, Load: map[Limit]Requests{key: {1, 1}}, Elapsed: time.Second})
		require.NoError(t, err)
		tick.Advance(time.Second) // advance to 10th second

		// read both hosts.  h1 should exist, h2 should not
		usage, err := agg.HostUsage(h1, []Limit{key})
		require.NoError(t, err)
		require.NotZero(t, usage[key].Weight, "h1 was refreshed and should remain")
		require.NotZero(t, usage[key].Used, "h1 was refreshed and usage data should remain")
		usage, err = agg.HostUsage(h2, []Limit{key})
		require.Zero(t, usage[key].Weight, "h2 should have no weight at all")
		require.NotZero(t, usage[key].Used, "h1 was refreshed and usage data should remain")

		// internals should also be partly emptied
		require.Len(t, agg.usage, 1, "limit should remain")
	})
	t.Run("cleans up by explicit gc", func(t *testing.T) {
		agg, tick := setup(t)
		tick.Advance(time.Second)
		met, err := agg.GC()
		require.NoError(t, err)
		require.Equal(t, Metrics{
			HostLimits: 0, // none remain
			Limits:     0, // none remain

			RemovedHostLimits: 2, // h1 and h2 for the single key, both removed
			RemovedLimits:     1, // single key removed
		}, met)

		// internals should also be empty
		require.Len(t, agg.usage, 0)
	})
}

func TestMinorCoverage(t *testing.T) {
	// not overly useful tests, but coverage++
	t.Run("gc", func(t *testing.T) {
		// invalid config
		agg, _ := newForTest(t, configSnapshot{}, false)
		m, err := agg.GC()
		assert.Zero(t, m)
		assert.ErrorContains(t, err, "bad ratelimiter config")
	})
	t.Run("update", func(t *testing.T) {
		// invalid config
		agg, _ := newForTest(t, configSnapshot{}, false)
		err := agg.Update(UpdateParams{ID: "ignored", Load: nil, Elapsed: time.Second})
		assert.ErrorContains(t, err, "bad ratelimiter config")
	})
	t.Run("get-weights", func(t *testing.T) {
		// invalid config
		agg, _ := newForTest(t, configSnapshot{}, false)
		usage, err := agg.HostUsage("ignored", nil)
		assert.Zero(t, usage)
		assert.ErrorContains(t, err, "bad ratelimiter config")
	})

	// a bit more useful
	t.Run("config validation", func(t *testing.T) {
		err := configSnapshot{
			weight:     -1,
			rate:       time.Duration(0),
			decayAfter: -time.Second,
			gcAfter:    -time.Second,

			now: time.Time{}, // ignored
		}.validate()
		// should have the shared error string
		assert.ErrorContains(t, err, "bad ratelimiter config")
		// should have each sub-error
		assert.ErrorContains(t, err, "weight cannot be negative")
		assert.ErrorContains(t, err, "rate must be positive")
		assert.ErrorContains(t, err, "decay-after cannot be negative")
		assert.ErrorContains(t, err, "gc-after cannot be negative")
		assert.Len(t, multierr.Errors(err), 5, "should have 5 errors, 4 details and one general")
	})
	t.Run("fast scalar path", func(t *testing.T) {
		cfg := configSnapshot{
			gcAfter: time.Second,
		}
		assert.Zero(t, cfg.missedUpdateScalar(2*time.Second), "should multiply old data by exactly zero when beyond gc age")
	})

	// weird config coverage
	t.Run("irrational decayAfter", func(t *testing.T) {
		// specifically: exercises the "less than one missed update" branch
		now := time.Now().Round(time.Second)
		cfg := configSnapshot{
			now:        now,
			rate:       time.Second,
			decayAfter: time.Second / 4, // irrational but allowed: decay faster than expected update rate

			// effectively ignored
			weight:  0.1,
			gcAfter: time.Second * 2,
		}
		scale := cfg.missedUpdateScalar(time.Second / 2) // between update and decay periods
		assert.Equal(t, PerSecond(1), scale, "should not have decayed yet")
	})
}

func TestRapidlyCoalesces(t *testing.T) {
	// This test ensures that, regardless of starting weights, the algorithm
	// "rapidly" achieves near-actual weight distribution after a small number of rounds.
	//
	// Otherwise, the exact numbers here don't really matter, it's just handy to show the
	// behavior in semi-extreme scenarios.  Logs show a quick adjustment which is what we want.
	// If you're making changes, check with like 10k rounds to make sure it's stable.
	//
	// Time is also not advanced because it doesn't actually need to be advanced.
	// An update is an update, and the caller's elapsed time is assumed to be correct.
	agg, _ := newValid(t, configSnapshot{
		// Using 0.5 weight because that's what we expect to use IRL, and this test is
		// ensuring that weight is good enough for the behavior we want.
		// Weight-math-correctness is ensured by other tests.
		weight: 0.5,

		// irrelevant / ignored
		rate:       time.Second,
		decayAfter: 2 * time.Second,
		gcAfter:    10 * time.Second,
	})
	snapshot := func() configSnapshot {
		snap, err := agg.snapshot()
		require.NoError(t, err)
		return snap
	}

	key := Limit("start workflow")
	h1, h2, h3 := Identity("one"), Identity("two"), Identity("three")

	weights, used, met := agg.getWeightsLocked(key, snapshot())
	assert.Zero(t, weights, "should have no weights")
	assert.Zero(t, used, "should have no used RPS")
	assert.Zero(t, met, "should have processed no data while calculating")

	push := func(host Identity, accept, reject int) {
		err := agg.Update(UpdateParams{
			ID: host,
			Load: map[Limit]Requests{
				key: {
					Accepted: accept,
					Rejected: reject,
				},
			},
			Elapsed: time.Second, // 1s just to make rps in == rps out
		})
		require.NoError(t, err)
	}

	// init with anything <~1000, too large and even a small fraction of the original value can be too big.
	push(h1, rand.Intn(1000), rand.Intn(1000))
	push(h2, rand.Intn(1000), rand.Intn(1000))
	push(h3, rand.Intn(1000), rand.Intn(1000))

	// now update multiple times and make sure it gets to 90% within 4 steps == 12s (normally).
	//
	// 4 steps with 0.5 weight should mean only 0.5^4 => 6.25% of the original influence remains,
	// which feels pretty reasonable: after ~10 seconds (3s updates), the oldest data only has ~10% weight.
	const target = 10 + 200 + 999
	for i := 0; i < 4; i++ {
		weights, used, met = agg.getWeightsLocked(key, snapshot())
		t.Log("used:", used, "of actual:", target)
		t.Log("weights so far:", weights)
		t.Log("calculation metrics:", met)
		push(h1, 10, 10)
		push(h2, 200, 200)
		push(h3, 999, 999)
	}
	weights, used, met = agg.getWeightsLocked(key, snapshot())
	t.Log("used:", used, "of actual:", target)
	t.Log("weights so far:", weights)
	t.Log("calculation metrics:", met)

	// aggregated allowed-request values should be less than 10% off
	assert.InDeltaf(t, target, float64(used), target*0.1, "should have allowed >90%% of target rps by the 5th round") // actually ~94%
	// also check weights, they should be within 10%
	assert.InDeltaMapValues(t, map[Identity]float64{
		h1: 10 / 1209.0,  // 0.07698229407
		h2: 200 / 1209.0, // 0.1539645881
		h3: 999 / 1209.0, // 0.7690531178
	}, floaty(weights), 0.1, "should be close to true load balance")
}

// converts for testify/assert as it requires same types, not just same underlying type
func floaty[K comparable, V numeric](m map[K]V) map[K]float64 {
	out := make(map[K]float64, len(m))
	for k, v := range m {
		out[k] = float64(v)
	}
	return out
}

func TestConcurrent(t *testing.T) {
	// essentially a fuzz-test for race purposes.
	// values aren't checked, but it shouldn't panic / shouldn't race / etc.
	// low timeout + real time clock to also have wall-clock changes race with logic.
	//
	// this test frequently reaches 100% coverage all on its own (currently),
	// though it's not guaranteed and that is not the intent.
	// other tests should cover sufficiently even if this test is skipped:
	// t.Skip("skipped to check coverage")

	const (
		updateRate      = time.Millisecond // fairly arbitrary, max gap between updates
		targetDuration  = 100 * updateRate // also minimum number of updates
		numHosts        = 10
		numUpdaters     = 10           // fairly arbitrary
		updatesPerBatch = numHosts / 3 // intentionally below len(hosts) to allow some to gc normally
	)

	agg, _ := newValid(t, configSnapshot{
		rate:       updateRate,
		decayAfter: 2 * updateRate,
		gcAfter:    3 * updateRate, // relatively low to trigger implicit gc, check coverage if changing the values

		weight: 0.1, // irrelevant but must be non-zero
	})
	agg.clock = clock.NewRealTimeSource()

	var hosts []Identity
	for i := 0; i < numHosts; i++ {
		hosts = append(hosts, Identity(fmt.Sprintf("host %d", i)))
	}
	var keys []Limit
	for i := 'a'; i <= 'z'; i++ {
		keys = append(keys, Limit(i))
	}

	do := make(chan struct{}, numUpdaters)
	var wg sync.WaitGroup

	// run some goroutines to update/read in batches
	for i := 0; i < numUpdaters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				_, ok := <-do
				if !ok {
					// chan's closed, stop
					return
				}
				num := rand.Intn(len(keys))
				updates := make(map[Limit]Requests, num)
				for i := 0; i < num; i++ {
					updates[keys[rand.Intn(len(keys))]] = Requests{
						Accepted: rand.Intn(100),
						Rejected: rand.Intn(100),
					}
				}
				host := hosts[rand.Intn(len(hosts))]

				// randomly update or read
				if rand.Intn(2) == 0 {
					err := agg.Update(UpdateParams{ID: host, Load: updates, Elapsed: updateRate})
					require.NoError(t, err)
				} else {
					_, err := agg.HostUsage(host, slices.Collect(maps.Keys(updates)))
					require.NoError(t, err)
				}
			}
		}()
	}

	// run a "trigger some work occasionally" goroutine
	wg.Add(1)
	start := time.Now()
	go func() {
		defer wg.Done()

		for {
			if time.Since(start) > targetDuration {
				close(do)
				return
			}
			// sleep a random portion of the update rate
			time.Sleep(time.Duration(rand.Intn(int(updateRate))))
			// allow a random number of updates.
			// should be non-blocking to further encourage racing, when possible
			for i := rand.Intn(updatesPerBatch); i > 0; i-- {
				do <- struct{}{}
			}
		}
	}()

	assert.True(t,
		common.AwaitWaitGroup(&wg, 5*targetDuration),
		"blocked test? still waiting after %v", 5*targetDuration)
	// non-racy even if waiting failed
	m, err := agg.GC()
	require.NoError(t, err)
	t.Logf("%#v", m) // should (usually) not be "full" + should (usually) remove some data
}

func TestSimulate(t *testing.T) {
	// Semi-fuzzy simulated sequence with fully computed values, exercising most behaviors.
	//
	// Everything about this test is sensitive to changes in behavior,
	// so if that occurs just update the values after ensuring they're reasonable.
	// Exact matches after changes are not at all important.

	updateRate := 3 * time.Second // both expected and duration fed to update
	agg, tick := newValid(t, configSnapshot{
		// now:    , ignored
		weight:     0.75, // fairly fast adjustment, and semi-human-friendly math
		rate:       updateRate,
		decayAfter: 2 * updateRate,
		gcAfter:    10 * updateRate,
	})

	// keeping var == string simplifies copy/paste as we cannot log the var name
	start, query := Limit("start"), Limit("query")
	all := []Limit{start, query}
	h1, h2, h3 := Identity("one"), Identity("two"), Identity("three")

	snap, err := agg.snapshot()
	require.NoError(t, err)
	weights, used, met := agg.getWeightsLocked(start, snap)
	assert.Zero(t, weights, "should have no weights")
	assert.Zero(t, used, "should have no used RPS")
	assert.Zero(t, met, "should have processed no data while calculating")

	// just simplifies arg-construction
	push := func(host Identity, key Limit, accept, reject int) {
		err := agg.Update(UpdateParams{
			ID: host,
			Load: map[Limit]Requests{
				key: {
					Accepted: accept,
					Rejected: reject,
				},
			},
			Elapsed: time.Second,
		})
		require.NoError(t, err)
	}

	// these tests intentionally share data and run sequentially,
	// the sub-testing is mostly to help group semantics.

	// init with Some Numbers.  updates to different keys with the same timestamp
	// can be grouped or separate, it doesn't matter - only same-key changes behavior.
	push(h1, query, 5, 5)
	push(h1, start, 5, 5)
	tick.Advance(time.Second)
	push(h2, query, 1, 1)
	push(h2, start, 1, 1)
	tick.Advance(time.Second)
	push(h3, query, 0, 0)
	push(h3, start, 1, 1)
	tick.Advance(time.Second)

	// changing new-data-weight does not affect this test because
	// zero -> nonzero keeps 100% to jump-start the initial state,
	// rather than gradually growing from zero (which would be biased
	// towards lower values during movement - not bad, but not intended).
	t.Run("initial weights at 3s", func(t *testing.T) {
		// 3s elapsed, all fresh.  h1 is "old" but within decayAfter so it's assumed still valid
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)

		// h1 has most of the weight.
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.83, Used: 6},
			start: {Weight: 0.71, Used: 7}, // h3 uses more start than query, so h1 has less weight for start
		}, usage)

		usage, err = agg.HostUsage(h2, all)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.17, Used: 6}, // only h1 and h2 called this, so (0.83 + 0.17)==1
			start: {Weight: 0.14, Used: 7}, // h3 also had a small use, so less than query
		}, usage)

		usage, err = agg.HostUsage(h3, all)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0},    // no query at all
			start: {Weight: 0.14}, // same num of starts as h2
		}, usage)
	})
	if t.Failed() {
		return // later tests likely invalid, verify each step before moving to the next
	}

	// advance to second h2 update, skip the others.
	// no data has expired yet, but h1 is now at 5s old (closing in on decayAfter)
	tick.Advance(2 * time.Second)
	push(h2, query, 1, 2)
	push(h2, start, 3, 3)

	t.Run("increased h2 weight at 5s", func(t *testing.T) {
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)
		// h1's weight reduces due to increased h2 usage
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.78, Used: 6},   // accepted query requests did not change (1 before, 1 after})
			start: {Weight: 0.59, Used: 8.5}, // but start calls from h2 increased by 2 -> 0.75 weight -> +1.5 total
		}, usage)

		usage, err = agg.HostUsage(h2, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.22, Used: 6},   // h3 has no query, so h1+h2=1.0 but the balance has shifted a bit towards h2
			start: {Weight: 0.29, Used: 8.5}, // increased over last round due to more calls
			// plus a sanity check: same rps as h1 saw
		}, usage)

		usage, err = agg.HostUsage(h3, all)
		require.NoError(t, err)
		// h3 is almost idle but 0.1 weight changes slowly
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0, Used: 6},      // never sent any query requests
			start: {Weight: 0.12, Used: 8.5}, // decreased since last round, as relative usage is lower
			// plus a sanity check: same rps as h1 saw
		}, usage)
	})
	if t.Failed() {
		return // later tests likely invalid, verify each step before moving to the next
	}

	// advance to 10s.
	// this puts h1's original data beyond decayAfter, so it will act as if it
	// had received 0-valued updates, to reduce its weight.
	tick.Advance(5 * time.Second)
	push(h2, query, 5, 5)
	push(h2, start, 5, 5)
	push(h3, query, 5, 5)
	push(h3, start, 5, 5)

	t.Run("h1 decayed at 10s", func(t *testing.T) {
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.01, Used: 7.83}, // 10 accepted in last round, but only 0.75 weight, 6.0 -> 7.8 due to older data
			start: {Weight: 0.01, Used: 8.22}, // similar
		}, usage)
		usage, err = agg.HostUsage(h2, all)
		require.NoError(t, err)
		// h2 has slightly over half weight due to greater historical use
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.45},
			start: {Weight: 0.53}, // still more starts than queries
		}, usage)
		usage, err = agg.HostUsage(h3, all)
		require.NoError(t, err)
		// h2 slightly less, due to low historical + some used by h1
		expectSimilarUsage(t, map[Limit]HostUsage{
			// this looks odd, but it's higher due to lower historical *total* queries,
			// leading to a somewhat counter-intuitive:
			// - smaller numerator (lower calls by this host)
			// - smaller denominator (lower total calls)
			// - higher final value (smaller denominator has greater influence)
			query: {Weight: 0.54},
			start: {Weight: 0.46},
		}, usage)
	})
	if t.Failed() {
		return // later tests likely invalid, verify each step before moving to the next
	}

	// update everything, should flatten compared to previous round
	tick.Advance(1 * time.Second)
	push(h1, query, 5, 5)
	push(h1, start, 5, 5)
	push(h2, query, 5, 5)
	push(h2, start, 5, 5)
	push(h3, query, 5, 5)
	push(h3, start, 5, 5)

	t.Run("all equal at 11s is relatively flatter than before", func(t *testing.T) {
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)
		// historically lower weight = current lower weight, but a big jump from 0.01 before.
		//
		// this is likely a faster shift than we want in practice, as it'll make allowed-request
		// behavior quite jumpy, which is why the initial weight is likely to be around 0.5.
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.28, Used: 13.21}, // used is adjusting towards 15
			start: {Weight: 0.28, Used: 13.52},
		}, usage)
		// and weights are flattening towards 0.33
		usage, err = agg.HostUsage(h2, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.36},
			start: {Weight: 0.36},
		}, usage)
		usage, err = agg.HostUsage(h3, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.37},
			start: {Weight: 0.35},
		}, usage)
	})
	if t.Failed() {
		return // later tests likely invalid, verify each step before moving to the next
	}

	// do that again, should flatten further
	tick.Advance(1 * time.Second)
	push(h1, query, 5, 5)
	push(h1, start, 5, 5)
	push(h2, query, 5, 5)
	push(h2, start, 5, 5)
	push(h3, query, 5, 5)
	push(h3, start, 5, 5)

	t.Run("all equal at 12s is even flatter", func(t *testing.T) {
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)
		// still slightly below the others but it hardly matters now
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.32, Used: 14.55}, // Used RPS now quite close to 15
			start: {Weight: 0.32, Used: 14.58},
		}, usage)
		// weights flattening towards 0.33
		usage, err = agg.HostUsage(h2, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.34},
			start: {Weight: 0.34},
		}, usage)
		usage, err = agg.HostUsage(h3, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.34},
			start: {Weight: 0.34},
		}, usage)
	})
	if t.Failed() {
		return // later tests likely invalid, verify each step before moving to the next
	}

	// make everything fully expired so the "from expired data" branch is exercised too.
	// specifically this just needs to be beyond gcAfter for all data.
	tick.Advance(time.Hour)
	// push anything (need all keys to get same HostWeight map keys for `expectSimilarUsage`
	push(h1, query, 5, 5)
	push(h1, start, 5, 5)
	push(h2, query, 5, 5)
	push(h2, start, 5, 5)
	push(h3, query, 5, 5)
	push(h3, start, 5, 5)

	t.Run("updating expired data acts like deleted data", func(t *testing.T) {
		// should leap to exact values, not weighted-from-zero.
		usage, err := agg.HostUsage(h1, all)
		require.NoError(t, err)
		// still slightly below the others but it hardly matters now
		expectSimilarUsage(t, map[Limit]HostUsage{
			// note: used RPS is exactly 15. if weighting from or very near zero, rather than tossing
			// old data entirely, this would be: (0*0.25 + 15*0.75) == 11.25
			query: {Weight: 0.333, Used: 15},
			start: {Weight: 0.333, Used: 15},
		}, usage)
		// weights flattening towards 0.33
		usage, err = agg.HostUsage(h2, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.333},
			start: {Weight: 0.333},
		}, usage)
		usage, err = agg.HostUsage(h3, all)
		require.NoError(t, err)
		expectSimilarUsage(t, map[Limit]HostUsage{
			query: {Weight: 0.333},
			start: {Weight: 0.333},
		}, usage)
	})
}

func expectSimilarUsage(
	t *testing.T,
	expected map[Limit]HostUsage,
	actual map[Limit]HostUsage) {
	t.Helper() // report caller's line as the logger, not this expect-er

	// check that host weights are similar
	eWeight := make(map[Limit]float64, len(expected))
	aWeight := make(map[Limit]float64, len(expected))
	for k, v := range expected {
		eWeight[k] = float64(v.Weight)
	}
	for k, v := range actual {
		aWeight[k] = float64(v.Weight)
	}

	// check host weights are similar
	if !assert.InDeltaMapValues(t, eWeight, aWeight, 0.01) {
		t.Logf("weight(s) differ by more than 0.01:\n"+
			"expected: %v\n"+
			"actual: %v", eWeight, aWeight)
		t.Logf("verify the weights by hand, and if it's correct just update them.  " +
			"values do not need to stay precise / there is no perfect value, " +
			"they just need to behave the way we want and should not change unexpectedly.")
	}

	// check that the used RPS is identical too, if non-zero
	eRps := make(map[Limit]float64, len(expected))
	aRps := make(map[Limit]float64, len(expected))
	for k, v := range expected {
		if v.Used != 0 {
			eRps[k] = float64(v.Used)
		}
	}
	for k, v := range actual {
		if v, ok := expected[k]; ok && v.Used == 0 {
			continue // ignore zeros in the expected map, as long as the key exists
		}
		aRps[k] = float64(v.Used)
	}
	if !assert.InDeltaMapValues(t, eWeight, aWeight, 0.01) {
		t.Logf("RPS(s) differ by more than 0.1:\n"+
			"expected: %v\n"+
			"actual: %v", eRps, aRps)
		t.Logf("verify the RPS values by hand, and if it's correct just update them.  " +
			"zero-RPS ignores the value, and can be used where we don't particularly care.")
	}
}

// fairly fuzzy but somewhat representative of expected use:
// benchmark "update a bunch of keys and get my load" requests, and accumulate data across all iterations.
// this is roughly what a server update operation will look like.
// there are intentionally overlaps in both hosts and keys to get multiple hosts per key and updates to existing data.
//
// while this obviously has major caveats and fake costs, the benchmark regularly runs several thousand iterations
// before it settles, filling a moderate amount of data along the way (e.g. consistently all 10k keys, and >200k records in memory).
//
// general results imply:
//   - host-weight reading takes a few times longer than updating, but on my laptop the
//     bench takes about 3-4ms per loop for the current 100/1000/10000 values, without mutexes.
//   - this is relatively CPU costly, but even if our largest cluster checks in every 3s to a *single*
//     host, this should work out with some room to spare... and it will be far more spread
//     out in practice due to the number of history hosts involved.
//   - an attempt to cache weight calculations per key until invalidated saved about 1/2 the cpu
//     with a perfect cache.
//   - ^ the cache seems not worth the complexity, but it'd be fairly easy to add if we change our minds
//
// since there is only a coarse mutex in the implementation itself, the whole aggregator forces itself
// to be processed serially.  if this turns out to be too costly, the easy fix is probably
// to shard the keys to e.g. 8 separate aggregators that can be processed concurrently.
//
// mutexes could be added to each Limit, but that could end up costing more in memory delays
// than the fine-grained locking gains us in concurrency flexibility.  benchmark first!
func BenchmarkNormalUse(b *testing.B) {
	updateRate := 3 * time.Second
	agg, _ := newValid(b, defaultConfig(updateRate))
	// intentionally using real clock source, time-gathering cost is relevant to benchmark
	agg.clock = clock.NewRealTimeSource()

	// aiming for vaguely realistic-ish values
	const (
		hosts       = 100   // num of hosts in pool, if rounds > hosts there will be duplicates (intentional)
		keysPerHost = 1000  // more domains than hosts, multiple limits per domain.
		globalKeys  = 10000 // each host gets ~10% of requests (not consistent across calls tho)
		requests    = 1000  // accept/reject counts
	)

	// rand and string-formatting is a non-trivial amount of load,
	// so this is prepared up-front so it can be excluded.
	type round struct {
		host    Identity
		load    map[Limit]Requests
		keys    []Limit
		elapsed time.Duration
	}

	rounds := make([]round, 0, b.N)
	for i := 0; i < b.N; i++ {
		keys := rand.Intn(keysPerHost)
		reqs := make(map[Limit]Requests, keys)
		for len(reqs) < keys {
			key := Limit(fmt.Sprintf("key %d", rand.Intn(globalKeys)))
			if _, ok := reqs[key]; ok {
				// duplicate, just try again.
				// dups wouldn't really invalidate the benchmark,
				// but it's easy to avoid and should make more stable results.
				continue
			}

			reqs[key] = Requests{
				// values don't really matter but there's no need for them all to use the same value
				Accepted: rand.Intn(requests),
				Rejected: rand.Intn(requests),
			}
		}
		rounds = append(rounds, round{
			host:    Identity(fmt.Sprintf("host %d", rand.Intn(hosts))),
			load:    reqs,
			keys:    slices.Collect(maps.Keys(reqs)),
			elapsed: time.Duration(rand.Int63n(updateRate.Nanoseconds())),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	sawnonzero := 0
	for _, r := range rounds { // == b.N times
		err := agg.Update(UpdateParams{ID: r.host, Load: r.load, Elapsed: r.elapsed})
		require.NoError(b, err)
		var unused map[Limit]PerSecond // ensure non-error second return for test safety
		usage, err := agg.HostUsage(r.host, r.keys)
		require.NoError(b, err)
		_ = unused // ignore unused rps
		if len(usage) > 0 {
			// 	wrote data and later read it out, benchmark is likely functional
			sawnonzero++
		}
	}
	b.StopTimer() // not benchmarking gc, just using it to show sanity-check metrics

	b.Log("N was:", b.N)
	m, err := agg.GC()
	require.NoError(b, err)
	b.Log("gc metrics:", m) // shows how many keys / hosts / etc we stored for manual validation

	// sanity check, as "all zero" should mean something like "always looking at nonexistent keys" / bad benchmark code.
	b.Log("nonzero results:", sawnonzero)
	assert.True(b, b.N < 500 || sawnonzero > 0, "no non-zero result found on a large enough benchmark, likely not benchmarking anything useful")
}

func TestHashmapIterationIsRandom(t *testing.T) {
	// is partial hashmap iteration reliably random each time so we can statistically ensure coverage,
	// or is it relatively fixed per map?
	// language spec is somewhat vague on the details here, so let's check.
	//
	// verdict: yes!  looks random each time.
	// so we can abuse it for an amortized / interruptible cleanup tool.

	const keys = 100
	m := map[int]struct{}{}
	for i := 0; i < keys; i++ {
		m[i] = struct{}{}
	}

	allObserved := make(map[int]struct{}, len(m))
	var orderObserved [][]int
	singlePass := func() {
		for i := 0; i < keys; i++ {
			observed := make([]int, 0, keys/10)
			for k := range m {
				allObserved[k] = struct{}{}
				observed = append(observed, k)
				if len(observed) == cap(observed) {
					break // interrupt part way through
				}
			}
			orderObserved = append(orderObserved, observed)
		}
	}

	// keep trying up to 10x to make it sufficiently-unlikely that randomness will fail.
	// with only a single round it fails like 5% of the time, which is reasonable behavior
	// but too much noise to allow to fail the test suite.
	for i := 0; i < 10 && len(allObserved) < keys; i++ {
		if i > 0 {
			t.Logf("insufficient keys observed (%d), trying again in round %d", len(allObserved), i+1)
		}
		singlePass()
	}

	// complain if it still hasn't observed all keys
	if !assert.Len(t, allObserved, keys) {
		// super noisy when successful, so only log when failing
		for idx, pass := range orderObserved {
			t.Log("Pass", idx)
			for _, keys := range pass {
				t.Log("\t", keys)
			}
		}
	}
}
