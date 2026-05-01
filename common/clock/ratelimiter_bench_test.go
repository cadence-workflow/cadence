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

package clock

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/atomic"
	"golang.org/x/time/rate"
)

/*
Benchmark data can be seen in the testdata folder, for relevant machines.

Interpretation:
  - for essentially all operations, mostly regardless of sequential / parallel / how parallel,
    the added latency is ~50%, and even extremes are less than double.
    this seems entirely tolerable and safe to use.
  - "reserve and cancel" sequentially is actually *faster* with the wrapper, probably due to fewer time advancements.
    with parallel usage, real vs wrapper is basically a tie.
  - mock-time ratelimiter is faster than real-time in essentially all cases, as you would hope.
    though not by much, unless waiting was part of the test.
  - real *rate.Limiter instances have some pathological behavior that... might disqualify them from safe use tbh.
    essentially this summarizes as "time.Now() is not globally monotonic", and it can lead to significant over and under limiting.

The real *rate.Limiter instances show these quirks during parallel tests, due to time rewinding and jumping forward repeatedly
from the *rate.Limiter's point of view (as it has an internal last-Now value protected by a mutex):
 1. `Allow()`-like calls (both literally `Allow()`, and `Reserve()` followed by sometimes `reservation.Cancel()`)
    can allow both significantly more or significantly fewer calls than they should.
 2. `Wait()` can allow calls through at a faster rate than it should.  At peak, a bit over 2x.
    (and probably slower, but sleeping is not guaranteed to wake up at a precise time so would still be correct behavior)

Essentially, when one call sets the time to "now", and an out-of-sync call sets it to "now-1ms" (time is acquired outside
the limiter's lock, and they may make progress out of order), a token may be restored when a time-consistent limiter would
not do so.

You can recreate this by hand by feeding a ratelimiter "now" and "now+1s" (>=2x the limit) randomly, and watching what it
allows / what the value of Tokens() is as time passes.  Doing this *literally* by hand reproduces it, e.g. pressing enter
on a command-line loop that does this - high frequency is not necessary, just illogical "time travel" outside tight bounds.

The first can be seen in m1_mac_go1.21.txt by comparing these tests:
  - BenchmarkLimiter/allow/parallel/real-4 (note the near-1µs time per allow, which serial calls match)
  - BenchmarkLimiter/allow/parallel/real-8 (time-per-allow is now 231ns, over 4x more allowed than it should)
  - BenchmarkLimiter/reserve-canceling-every-3/parallel/real-2 (1.5ms per allow, around 1,000x fewer allowed than it should)
  - BenchmarkLimiter/reserve-canceling-every-3/parallel/real-8 (450ns per allow, around 2x more than it should)

"real with pinned time" behaves similarly, for similar reasons: canceling can time-travel.
pinned time does make the behavior when called on a single goroutine roughly perfect though.

And the second can be seen in m1_mac_go1.21.txt by comparing these tests:
  - BenchmarkLimiter/wait/parallel/1µs_rate/real (almost exactly 1µs per iteration)
  - BenchmarkLimiter/wait/parallel/1µs_rate/real-8 (~500ns, twice as fast as it should allow)

I have not tried to verify Wait's cause by hand, but it certainly seems like time-thrashing explains it as well.
This is also supported by the wrapper's time-locking *completely* eliminating this flaw, as all iterations take
almost exactly 1µs as they should.

---

All machines I've tried it on behave similarly, though whether one parallel test triggers misbehavior or not
is fairly random.  Higher parallelism triggers it more, and it seems *fairly* likely that at least one misbehaves
in a full `-cpu 1,2,4,8,...[>=cores]` suite, and sometimes many more.
*/
func BenchmarkLimiter(b *testing.B) {
	const (
		normalLimit      = time.Microsecond // pretty arbitrary but has a good blend of allow and deny
		burst            = 1000             // arbitrary, but more interesting than 1, and better matches how we use limiters (rate/s == burst)
		allowedPeriodFmt = "time per allow: %9s"
	)

	type runType func(b *testing.B, each func(int) bool)

	// runs a callback in a sequential benchmark, and tracks allowed/denied metrics
	var runSerial runType = func(b *testing.B, each func(int) bool) {
		b.Helper()
		allowed, denied := 0, 0
		for i := 0; i < b.N; i++ {
			if each(i) {
				allowed++
			} else {
				denied++
			}
		}
		as := fmt.Sprintf("allowed: %v,", allowed)
		ds := fmt.Sprintf("denied: %v,", denied)
		allowedPeriod := fmt.Sprintf(allowedPeriodFmt, "n/a")
		if allowed > 0 {
			allowedPeriod = fmt.Sprintf(allowedPeriodFmt, fmt.Sprint(b.Elapsed()/time.Duration(allowed)))
		}
		b.Logf("%-20s %-20s %s", as, ds, allowedPeriod) // allows controlling whitespace better
	}
	// runs a callback in a parallel benchmark, and tracks allowed/denied metrics
	var runParallel runType = func(b *testing.B, each func(int) bool) {
		b.Helper()
		var allowed, denied atomic.Int64
		b.RunParallel(func(pb *testing.PB) {
			n := 0
			for pb.Next() {
				if each(n) {
					allowed.Inc()
				} else {
					denied.Inc()
				}
				n++
			}
		})
		as := fmt.Sprintf("allowed: %v,", allowed.Load())
		ds := fmt.Sprintf("denied: %v,", denied.Load())
		allowedPeriod := fmt.Sprintf(allowedPeriodFmt, "n/a")
		if allowed.Load() > 0 {
			allowedPeriod = fmt.Sprintf(allowedPeriodFmt, fmt.Sprint(b.Elapsed()/time.Duration(allowed.Load())))
		}
		b.Logf("%-20s %-20s %s", as, ds, allowedPeriod) // allows controlling whitespace better
	}

	both := map[string]runType{
		"serial":   runSerial,
		"parallel": runParallel,
	}

	// Allow is a significant amount of our ratelimiter usage,
	// so this should probably take top priority.
	b.Run("allow", func(b *testing.B) {
		for name, runner := range both {
			b.Run(name, func(b *testing.B) {
				b.Run("real", func(b *testing.B) {
					// very fast to jump back and forth, rather than slamming into "deny"
					rl := rate.NewLimiter(rate.Every(normalLimit), burst)
					runner(b, func(i int) bool {
						return rl.Allow()
					})
				})
				b.Run("wrapped", func(b *testing.B) {
					rl := NewRatelimiter(rate.Every(normalLimit), burst)
					runner(b, func(i int) bool {
						return rl.Allow()
					})
				})
				b.Run("mocked timesource", func(b *testing.B) {
					ts := NewMockedTimeSource()
					rl := NewRateLimiterWithTimeSource(ts, rate.Every(normalLimit), burst)
					runner(b, func(i int) bool {
						// adjusted by eye, to try to very roughly match the above values for the final runs.
						// anything not extremely higher or lower is probably fine.
						ts.Advance(normalLimit / 5)
						return rl.Allow()
					})
				})
			})
		}
	})

	// Reserve is used when tiering ratelimiters, which is only done in a few
	// primarily-user-facing limits that aren't super high perf need...
	// ... BUT Reserve is used in the wrapper's Wait, so this serves to separate
	// out its cost from the Wait benchmarks.
	cancelNth := 3
	b.Run(fmt.Sprintf("reserve-canceling-every-%v", cancelNth), func(b *testing.B) {
		for name, runner := range both {
			b.Run(name, func(b *testing.B) {
				/*
					CAUTION: in parallel, these real ratelimiters run quickly, but note how
					frequently requests are allowed.  Both more and less than intended, sometimes
					by multiple orders of magnitude.

					example:
						BenchmarkLimiter/reserve-canceling-every-3/parallel/real-2
							ratelimiter_test.go:136: allowed: 752,        denied: 7519799,     time per allow: 1.558596ms
							                                                         1,500x too few! ----------^
						BenchmarkLimiter/reserve-canceling-every-3/parallel/real-8
							ratelimiter_test.go:136: allowed: 2651622,    denied: 1415746,     time per allow:     449ns
							                                                            2x too many! --------------^

					this occurs because the ratelimiter gathers `time.Now()` before locking, which
					leads to *handling* time in a non-monotonic way depending on what goroutines
					acquire the lock / in which order.

					this is one of the reasons this wrapper was built.
				*/
				b.Run("real", func(b *testing.B) {
					rl := rate.NewLimiter(rate.Every(normalLimit), burst)
					runner(b, func(i int) (allowed bool) {
						r := rl.Reserve()
						allowed = r.OK() && r.Delay() == 0
						canceled := i%cancelNth == 0
						allowed = allowed && !canceled
						if !allowed {
							// all not-allowed calls must be canceled, as they will not be "used".
							// otherwise they will still be "used", and tokens will go negative,
							// possibly far into the negatives.
							r.Cancel()
							return
						}
						return
					})
				})
				b.Run("real pinned time", func(b *testing.B) {
					rl := rate.NewLimiter(rate.Every(normalLimit), burst)
					runner(b, func(i int) (allowed bool) {
						// expected to be faster as the limiter's time does not
						// advance as often, and "now" is only gathered once.
						//
						// but this is not correct with concurrent use, so it's
						// purely synthetic and serves as a lower bound only.
						now := time.Now()
						r := rl.ReserveN(now, 1)
						allowed = r.OK() && r.DelayFrom(now) == 0
						canceled := i%cancelNth == 0
						allowed = allowed && !canceled
						if !allowed {
							r.CancelAt(now)
							return
						}
						return
					})
				})

				// calls Reserve on the wrapped limiter and sometimes cancels, sequentially or in parallel
				runWrapped := func(b *testing.B, rl Ratelimiter, runner runType, advance func()) {
					b.Helper()
					runner(b, func(i int) (allowed bool) {
						// not b.Helper because no logs occur inside here, and it has runtime cost
						if advance != nil {
							advance() // for mock time
						}
						r := rl.Reserve()
						allowed = r.Allow()
						canceled := i%cancelNth == 0
						allowed = allowed && !canceled
						r.Used(allowed)
						return
					})
				}
				b.Run("wrapped", func(b *testing.B) {
					rl := NewRatelimiter(rate.Every(normalLimit), burst)
					runWrapped(b, rl, runner, nil)
				})
				b.Run("mocked timesource", func(b *testing.B) {
					ts := NewMockedTimeSource()
					rl := NewRateLimiterWithTimeSource(ts, rate.Every(normalLimit), burst)
					runWrapped(b, rl, runner, func() {
						// any value should work, but smaller than normalLimit revealed
						// some issues in the past, where time-thrashing would lead to
						// extremely low allowed calls.
						//
						// this is now resolved with synchronous cancels when reservations
						// are not synchronously allowed, but it seems worth keeping to
						// reveal regressions.
						ts.Advance(normalLimit / 10)
					})
				})
			})
		}
	})

	// Wait is implemented quite differently between real and wrapped.
	// For the most part our use is in batches, where queueing is expected.
	// This makes benchmarks kinda synthetic.  Queueing won't run any faster than the intended rate.
	//
	// Still, the allowed events per second should be similar (when limited).
	//
	// Unfortunately there are some rate.Limiter misbehaviors and a very high chance
	// of CPU latency impacting this test, so this is not asserted, only logged.
	b.Run("wait", func(b *testing.B) {
		for name, runner := range both {
			b.Run(name, func(b *testing.B) {
				durations := []time.Duration{
					// try an insanely fast refresh, to see maximum throughput with "zero" waiting.
					// intentionally avoiding 1 nanosecond in case it gets special-cased, like 0 and math.MaxInt64.
					2 * time.Nanosecond,
					// try a rate that should contend and wait heavily.
					// this SHOULD be how long the per-iteration time is (when not mocked)... but read comments below.
					normalLimit,
				}
				for _, dur := range durations {
					b.Run(fmt.Sprintf("%v rate", dur), func(b *testing.B) {
						limit := rate.Every(dur)
						// set a wait timeout long enough to allow ~all attempts.
						//
						// b.Run seems to target <1s, but very fast or very slow things can extend it,
						// and up to about 2s seems normal for these.
						// because of that, 5s occasionally timed out, but 10s seems fine.
						timeout := 20 * time.Second
						ctx, cancel := context.WithTimeout(context.Background(), timeout)
						defer cancel()

						b.Run("real", func(b *testing.B) {
							// sometimes misbehaves, allowing too many or too few requests through!
							rl := rate.NewLimiter(limit, burst)
							runner(b, func(i int) bool {
								return rl.Wait(ctx) == nil
							})
						})
						b.Run("wrapped", func(b *testing.B) {
							rl := NewRatelimiter(limit, burst)
							runner(b, func(i int) bool {
								return rl.Wait(ctx) == nil
							})
						})
						b.Run("mocked timesource", func(b *testing.B) {
							ts := NewMockedTimeSource()
							rl := NewRateLimiterWithTimeSource(ts, limit, burst)
							ctx := &timesourceContext{
								ts:       ts,
								deadline: time.Now().Add(timeout),
							}
							// make sure tests don't block super long even if they misbehave,
							// by canceling the context after enough real-world time has passed.
							defer time.AfterFunc(timeout, func() {
								ts.Advance(timeout)
							}).Stop()
							runner(b, func(i int) bool {
								// advance enough to restore a token so it does not block forever.
								// "mock-wait times out when it should" is asserted in the tests.
								ts.Advance(dur)
								return rl.Wait(ctx) == nil
							})
						})

						// should be true if the round took longer than timeout,
						// as the AfterFunc will fire -> advance time -> close context -> then the round finishes.
						if ctx.Err() != nil {
							b.Errorf("benchmark likely invalid, did not complete before context timeout (%v)", timeout)
						}
					})
				}
			})
		}
	})
}
