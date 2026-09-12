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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

func TestRatelimiter(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"real", "mocked"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := func() MockedTimeSource { return nil }
			if name == "mocked" {
				ts = NewMockedTimeSource
			}
			// needs to be a bit coarser than the comparison tests, because time
			// is less tightly controlled / mock time relies on a background loop.
			granularity := 100 * time.Millisecond
			assertRatelimiterBasicsWork(t, ts, granularity)
		})
	}
}

// check relatively basic properties of the ratelimiter wrapper,
// and make sure that it behaves the same with both real time (makeTimesource returns nil)
// and with mocked time (simulated by advancing mock-time in a background goroutine).
//
// broken out because it's just very deeply indented otherwise.
func assertRatelimiterBasicsWork(t *testing.T, makeTimesource func() MockedTimeSource, granularity time.Duration) {
	// all "events" take place at 2*granularity
	event := granularity * 2

	makeLimiter := func(ts MockedTimeSource, limit rate.Limit, burst int) (rl Ratelimiter, sleep func(time.Duration)) {
		if ts == nil {
			return NewRatelimiter(limit, burst), time.Sleep
		}
		return NewRateLimiterWithTimeSource(ts, limit, burst), ts.Advance
	}

	now := func(ts MockedTimeSource) time.Time {
		if ts == nil {
			return time.Now()
		}
		return ts.Now()
	}

	// creates a context that will time out with the passed timesource, or real time if nil
	contextWithTimeout := func(ts TimeSource, dur time.Duration) (ctx context.Context, cancel context.CancelFunc) {
		if ts == nil {
			// could be a timesourceContext with a real timesource,
			// but I'm avoiding that until it's fleshed out fully and tested separately.
			return context.WithTimeout(context.Background(), dur)
		}
		ctx = &timesourceContext{
			ts:       ts,
			deadline: ts.Now().Add(dur),
		}
		return ctx, func() {}
	}

	// advances mock time in the background until stopped, at [granularity/10] per millisecond tick.
	// currently this leads to tests that are about 10x faster than real time, but "faster" is not
	// a goal in this context.
	//
	// "different" is however good for making sure we don't rely on real time somehow,
	// so try to keep it either faster or slower, whatever makes for a decently non-flaky suite.
	//
	// flawed ratelimiter behavior when using mock time should also cause the fuzz test to fail
	// eventually, so failures here are, hopefully, only due to flawed tests or excessive noise.
	advanceTime := func(ts MockedTimeSource) (cleanup func()) {
		if ts == nil {
			return func() {} // nothing to do
		}
		stop, done := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(done)
			// this is not a great way to do mock-time tests, but it does automatically
			// work in just about all cases, without needing customization.
			//
			// ideally, in other kinds of tests, you should wait for ts.BlockUntil(..)
			// or some other event, and then advance time semi-precisely, and fall back
			// to this strategy only when that isn't usable for some reason.
			//
			// one extremely nice quality of mocked time though, even when done like this:
			// time simply pauses while debugging, and nothing "artificially" times out
			// due to being blocked, regardless of how long you let things sit.
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					ts.Advance(granularity / 10)
				}
			}
		}()
		return func() {
			close(stop)
			<-done
		}
	}

	t.Run("sets burst and limit", func(t *testing.T) {
		t.Parallel()
		// should set on construction
		rl, _ := makeLimiter(makeTimesource(), rate.Every(time.Second), 3)
		assert.EqualValues(t, 3, rl.Burst())
		assert.Equal(t, rate.Every(time.Second), rl.Limit())

		// and when calling the setters
		rl.SetBurst(5)
		rl.SetLimit(rate.Every(time.Millisecond))
		assert.EqualValues(t, 5, rl.Burst())
		assert.Equal(t, rate.Every(time.Millisecond), rl.Limit())
	})
	t.Run("limits requests to within available burst tokens", func(t *testing.T) {
		t.Parallel()
		rl, _ := makeLimiter(makeTimesource(), rate.Every(time.Second), 3)
		assert.EqualValues(t, 3, rl.Tokens(), "should have tokens to allow exactly 3 calls, and not exceed burst of 3")
		assert.True(t, rl.Allow())
		assert.InDeltaf(t, 2, rl.Tokens(), 0.1, "Allow should consume a token")
		assert.True(t, rl.Allow())
		assert.InDelta(t, 1, rl.Tokens(), 0.1)
		assert.True(t, rl.Allow())
		assert.InDelta(t, 0, rl.Tokens(), 0.1)
		assert.False(t, rl.Allow(), "should not allow after burstable tokens are consumed")
		assert.InDelta(t, 0, rl.Tokens(), 0.1)
		assert.False(t, rl.Allow())
	})
	t.Run("recovers tokens as time passes", func(t *testing.T) {
		t.Parallel()
		rl, sleep := makeLimiter(makeTimesource(), rate.Every(event), 1)
		assert.True(t, rl.Allow())
		assert.False(t, rl.Allow())
		sleep(event)
		assert.True(t, rl.Allow(), "should have recovered one token")
		assert.False(t, rl.Allow())
	})
	t.Run("waits until tokens are available", func(t *testing.T) {
		t.Parallel()
		ts := makeTimesource()
		rl, _ := makeLimiter(ts, rate.Every(event), 1)
		started := now(ts) // must be collected before Allow, or sleep time could legitimately be lower than the rate
		assert.True(t, rl.Allow())
		assert.False(t, rl.Allow())

		stopTime := advanceTime(ts)
		ctx, cancel := contextWithTimeout(ts, event)
		defer cancel()
		err := rl.Wait(ctx)
		stopTime()

		assert.NoError(t, err, "Wait should have waited and then allowed the event")
		elapsed := now(ts).Sub(started)
		assert.Truef(t, elapsed >= event-granularity && elapsed < event+granularity,
			"elapsed time outside bounds, should be %v <= %v < %v",
			event-granularity, elapsed, event+granularity)

		assert.False(t, rl.Allow(), "Wait must consume a token")
	})
	t.Run("reserve", func(t *testing.T) {
		t.Parallel()
		// simple stuff
		for name, tc := range map[string]struct {
			do          func(t *testing.T, rl Ratelimiter, sleep func(time.Duration))
			tokenChange float64
		}{
			"consumes a token": {
				do: func(t *testing.T, rl Ratelimiter, sleep func(time.Duration)) {
					r := rl.Reserve()
					assert.True(t, r.Allow())
					r.Used(true)
				},
				tokenChange: -1,
			},
			"restores a token when canceling": {
				do: func(t *testing.T, rl Ratelimiter, sleep func(time.Duration)) {
					r := rl.Reserve()
					assert.True(t, r.Allow())
					r.Used(false)
				},
				tokenChange: 0,
			},
			"does not restore if there are interleaving calls": {
				do: func(t *testing.T, rl Ratelimiter, sleep func(time.Duration)) {
					r := rl.Reserve()
					assert.True(t, r.Allow())
					sleep(1) // advance time by any amount
					rl.Allow()
					r.Used(false)
				},
				tokenChange: -2, // reservation did not restore back to -1
			},
			"does not restore if Tokens is called in before canceling": {
				do: func(t *testing.T, rl Ratelimiter, sleep func(time.Duration)) {
					// to make a possibly-surprising quirk explicit:
					// tokens advances the limiter's "now" value, which means
					// canceling an allowed token does not work.
					r := rl.Reserve()
					assert.True(t, r.Allow())
					sleep(1)    // advance time by any amount
					rl.Tokens() // advance time internally
					r.Used(false)
				},
				tokenChange: -1, // -1 due to Tokens call
			},
			"does not go negative": {
				do: func(t *testing.T, rl Ratelimiter, sleep func(time.Duration)) {
					rl.Allow()
					rl.Allow()
					assert.InDeltaf(t, 0, rl.Tokens(), 0.1, "should have consumed all tokens")
					r := rl.Reserve()
					assert.False(t, r.Allow())
					r.Used(false)
				},
				tokenChange: -2, // would be -3 if it went negative
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ts := makeTimesource()
				rl, sleep := makeLimiter(ts, rate.Every(event), 2)

				before := rl.Tokens()
				stopTime := advanceTime(ts)
				tc.do(t, rl, sleep)
				stopTime()
				after := rl.Tokens()

				assert.InDeltaf(t, tc.tokenChange, after-before, 0.1, "tokens should have changed from %v to %v, but was %v", before, before+tc.tokenChange, after)
			})
		}
	})
	t.Run("wait", func(t *testing.T) {
		t.Parallel()
		wait := func(ts MockedTimeSource, periods float64, rl Ratelimiter) int64 {
			ctx, cancel := contextWithTimeout(ts, time.Duration(periods*float64(event)))
			defer cancel()
			if rl.Wait(ctx) == nil {
				return 1
			}
			return 0
		}
		parWait := func(ts MockedTimeSource, periods float64, count *atomic.Int64, rl Ratelimiter) {
			if wait(ts, periods, rl) == 1 {
				count.Inc()
			}
		}
		for name, tc := range map[string]struct {
			drainFirst       bool
			do               func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64
			latency          float64 // num of [granularity]s that should elapse
			allowed          int64
			tokenChange      float64
			allowLowerTokens bool // concurrent waits cannot be guaranteed to return tokens, negatives are possible
		}{
			"allows immediately with free tokens": {
				drainFirst: false,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					return wait(ts, 3, rl) // anything longer than 1
				},
				allowed:     1,
				tokenChange: -1,
			},
			"fails immediately with too-short timeout": {
				drainFirst: true,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					return wait(ts, 0.1, rl)
				},
				latency: 0,
				allowed: 0,
			},
			"waits for one token": {
				drainFirst: true,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					return wait(ts, 2, rl)
				},
				latency: 1,
				allowed: 1,
			},
			"concurrently waits for 2 tokens": {
				drainFirst: true,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					count := atomic.Int64{}
					var g errgroup.Group
					for i := 0; i < 2; i++ {
						g.Go(func() error {
							parWait(ts, 2.5, &count, rl) // slightly longer than 2
							return nil
						})
					}
					g.Wait()
					return count.Load()
				},
				latency: 2,
				allowed: 2,
			},
			"one wins and one fails when concurrently waiting": {
				drainFirst: true,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					count := atomic.Int64{}
					var g errgroup.Group
					for i := 0; i < 2; i++ {
						g.Go(func() error {
							parWait(ts, 1.5, &count, rl) // slightly longer than 1
							return nil
						})
					}
					g.Wait()
					// only one can win
					return count.Load()
				},
				latency:          1,
				allowed:          1,
				allowLowerTokens: true, // tokens are not guaranteed to be returned
			},
			"immediately drains 2, waits for 2, fails 2": {
				drainFirst: false,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					count := atomic.Int64{}
					var g errgroup.Group
					for i := 0; i < 6; i++ {
						g.Go(func() error {
							parWait(ts, 2.5, &count, rl) // slightly longer than 2, so 2 waiters succeed
							return nil
						})
					}
					g.Wait()
					return count.Load()
				},
				latency:          2,
				allowed:          2,
				tokenChange:      -2,
				allowLowerTokens: true, // tokens are not guaranteed to be returned
			},
			"cancel while waiting": {
				drainFirst: true,
				do: func(t *testing.T, rl Ratelimiter, ts MockedTimeSource) int64 {
					var wg sync.WaitGroup
					count := atomic.Int64{}
					wg.Add(2)
					ctx, cancel := context.WithTimeout(context.Background(), event*3/2)
					go func() {
						defer wg.Done()
						// a bit too much of a mess to thread this through nicely
						if ts == nil {
							<-time.After(event / 2) // cancel half way through the wait
							cancel()
						} else {
							ts.AfterFunc(event/2, func() {
								cancel()
							})
						}
					}()
					go func() {
						defer wg.Done()
						if rl.Wait(ctx) == nil {
							count.Inc()
						}
					}()
					wg.Wait()
					return count.Load()
				},
				allowed:     0,
				latency:     0.5, // unfortunately allows 0 currently, but token change should ensure time passed
				tokenChange: 0.5, // only waited for half an event -> recovered half a token
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ts := makeTimesource()
				rl, _ := makeLimiter(ts, rate.Every(granularity*2), 2)
				if tc.drainFirst {
					assert.True(t, rl.Allow())
					assert.True(t, rl.Allow())
					assert.False(t, rl.Allow())
				}

				before := rl.Tokens()
				start := now(ts)
				stopTime := advanceTime(ts)
				tc.do(t, rl, ts)
				stopTime()
				after := rl.Tokens()
				elapsed := now(ts).Sub(start)
				target := time.Duration(tc.latency * float64(granularity*2))

				if tc.allowLowerTokens {
					// some tests can reasonably fail to restore a used token, so they can go lower than would be ideal
					assert.True(t, after-before <= tc.tokenChange+0.5, "tokens should have changed from %0.2f to ~%0.2f (or lower), but was %0.2f -> %0.2f", before, before+tc.tokenChange, before, after)
				} else {
					// tokens should be fairly precise (cpu noise can make this fuzzy)
					assert.InDeltaf(t, tc.tokenChange, after-before, 0.5, "tokens should have changed from %0.2f to %0.2f (+/- 0.5 for cpu noise), but was %0.2f -> %0.2f", before, before+tc.tokenChange, before, after)
				}
				assert.True(t,
					elapsed > (target-granularity) && elapsed < (target+granularity),
					"should have taken %v < (actual) %v < %v",
					target-granularity, elapsed, target+granularity,
				)
			})
		}
	})
}

func TestRatelimiterCoverage(t *testing.T) {
	t.Run("time paradox", func(t *testing.T) {
		// this test's behavior should not be possible, as any reservation with
		// a time in the future must have been reserved in the future, which
		// would have advanced the internal latestNow to match that future-time.
		rl := NewRatelimiter(rate.Every(time.Second), 1)
		impl := rl.(*ratelimiter)
		r := rl.Reserve()
		latestNow := impl.latestNow
		require.True(t, r.Allow())
		rimpl := r.(*allowedReservation)
		// cheat, put the reservation into the future.
		rimpl.reservedAt = time.Now().Add(time.Second)
		rimpl.Used(false)
		assert.True(t, latestNow.Before(impl.latestNow), "ratelimiter-internal time should have advanced despite impossible sequence")
	})
	t.Run("zero burst", func(t *testing.T) {
		// we do not currently make use of this, but it's allowed by the API.
		rl := NewRatelimiter(rate.Every(time.Second), 0)
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour) // much longer than the rate
		defer cancel()
		started := time.Now()
		assert.Error(t, rl.Wait(ctx), "0 burst should never allow events, so it should fail immediately")
		elapsed := time.Since(started)
		assert.Less(t, elapsed, time.Second/10, "Wait should have returned almost immediately on impossible waits")
	})
	t.Run("mock limiter constructor", func(t *testing.T) {
		// covered by fuzz testing, but this gets it to 100% without fuzz.
		_ = NewRateLimiterWithTimeSource(NewMockedTimeSource(), 1, 1)
	})
}

// context which becomes Done() based on a TimeSource instead of real time
type timesourceContext struct {
	// does not contain a parent context as we currently have no need,
	// but a "real" one would for forwarding Value and deadline lookups.
	ts       TimeSource
	deadline time.Time
	mut      sync.Mutex
}

func (t *timesourceContext) Deadline() (deadline time.Time, ok bool) {
	t.mut.Lock()
	defer t.mut.Unlock()
	return t.deadline, !t.deadline.IsZero()
}

func (t *timesourceContext) Done() <-chan struct{} {
	// not currently expected to be used, but it would look like this:
	t.mut.Lock()
	defer t.mut.Unlock()
	c := make(chan struct{})
	delay := t.deadline.Sub(t.ts.Now())
	t.ts.AfterFunc(delay, func() {
		// this stack may leak if time is not advanced past it in tests.
		close(c)
	})
	return c
}

func (t *timesourceContext) Err() error {
	t.mut.Lock()
	defer t.mut.Unlock()
	if t.ts.Now().After(t.deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func (t *timesourceContext) Value(key any) any {
	panic("unimplemented")
}

var _ context.Context = (*timesourceContext)(nil)
