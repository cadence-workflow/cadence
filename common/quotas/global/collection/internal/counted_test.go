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

package internal

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/quotas"
)

func TestUsage(t *testing.T) {
	t.Run("tracks allow", func(t *testing.T) {
		ts := clock.NewMockedTimeSource()
		counted := NewCountedLimiter(clock.NewRateLimiterWithTimeSource(ts, 1, 1))

		assert.True(t, counted.Allow(), "should match wrapped limiter")
		assert.Equal(t, UsageMetrics{1, 0, 0}, counted.Collect())

		assert.False(t, counted.Allow(), "should match wrapped limiter")
		assert.Equal(t, UsageMetrics{0, 1, 0}, counted.Collect(), "previous collect should have reset counts, and should now have just a reject")
	})
	t.Run("tracks wait", func(t *testing.T) {
		ts := clock.NewMockedTimeSource()
		counted := NewCountedLimiter(clock.NewRateLimiterWithTimeSource(ts, 1, 1))

		// consume the available token
		requireQuickly(t, 100*time.Millisecond, func() {
			assert.NoError(t, counted.Wait(context.Background()), "should match wrapped limiter")
			assert.Equal(t, UsageMetrics{1, 0, 0}, counted.Collect())
		})
		// give up before the next token arrives
		requireQuickly(t, 100*time.Millisecond, func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			assert.Error(t, counted.Wait(ctx), "should match wrapped limiter")
			assert.Equal(t, UsageMetrics{0, 1, 0}, counted.Collect(), "previous collect should have reset counts, and should now have just a reject")
		})
		// wait for the next token to arrive
		requireQuickly(t, 100*time.Millisecond, func() {
			var g errgroup.Group
			g.Go(func() error {
				// waits for token to arrive
				assert.NoError(t, counted.Wait(context.Background()), "should match wrapped limiter")
				assert.Equal(t, UsageMetrics{1, 0, 0}, counted.Collect())
				return nil
			})
			g.Go(func() error {
				time.Sleep(time.Millisecond)
				ts.Advance(time.Second) // recover one token
				return nil
			})
			assert.NoError(t, g.Wait())
		})
	})
	t.Run("tracks reserve", func(t *testing.T) {
		ts := clock.NewMockedTimeSource()
		lim := NewCountedLimiter(clock.NewRateLimiterWithTimeSource(ts, 1, 1))

		r := lim.Reserve()
		assert.True(t, r.Allow(), "should have used the available burst")
		assert.Equal(t, UsageMetrics{0, 0, 1}, lim.Collect(), "allowed tokens should not be counted until they're used")

		r.Used(true)
		assert.Equal(t, UsageMetrics{1, 0, 0}, lim.Collect(), "using the token should reset idle and count allowed")

		r = lim.Reserve()
		assert.False(t, r.Allow(), "should not have a token available")
		r.Used(false)
		assert.Equal(t, UsageMetrics{0, 1, 0}, lim.Collect(), "not-allowed reservations count as rejection")
	})
	// largely for coverage
	t.Run("supports Limit", func(t *testing.T) {
		rps := rate.Limit(1)
		lim := NewCountedLimiter(clock.NewRateLimiterWithTimeSource(clock.NewMockedTimeSource(), rps, 1))
		assert.Equal(t, rps, lim.Limit())
	})
}

func TestRegression_ReserveCountsCorrectly(t *testing.T) {
	run := func(t *testing.T, lim quotas.Limiter, advance func(time.Duration), collect func() UsageMetrics) {
		allowed, returned, rejected := 0, 0, 0
		for i := 0; ; i++ {
			if rejected > 3 {
				// normal exit: some rejects occurred.
				break // just to get more than 1 to be more interesting
			}
			if i > 1_000 {
				// infinite loop guard because it's a real mess to debug
				t.Error("too many attempts, test is not sane. allowed:", allowed, "rejected:", rejected, "returned:", returned)
				break
			}

			r := lim.Reserve()

			if rand.Intn(2) == 0 {
				// time advancing before canceling should not affect this test because it is not concurrent,
				// so only do it sometimes to make sure that's true
				advance(time.Millisecond)
			}

			if r.Allow() {
				if i%2 == 0 {
					allowed++
					r.Used(true)
				} else {
					returned++
					r.Used(false)
				}
			} else {
				rejected++
				// try with both true and false.
				// expected use is to call with false on all rejects, but it should not be required
				r.Used(i%2 == 0)
			}
		}
		usage := collect()
		t.Logf("usage: %#v", usage)
		assert.NotZero(t, allowed, "should have allowed some requests")
		assert.Equal(t, allowed, usage.Allowed, "wrong num of requests allowed")
		assert.Equal(t, rejected, usage.Rejected, "wrong num of requests rejected")
		assert.Equal(t, 0, usage.Idle, "limiter should never be idle in this test")
	}

	t.Run("counted", func(t *testing.T) {
		// "base" counting-limiter should count correctly
		ts := clock.NewMockedTimeSource()
		wrapped := clock.NewRateLimiterWithTimeSource(ts, 1, 100)
		lim := NewCountedLimiter(wrapped)

		run(t, lim, ts.Advance, lim.Collect)
	})
	t.Run("shadowed", func(t *testing.T) {
		// "shadowed" should call the primary correctly at the very least
		ts := clock.NewMockedTimeSource()
		wrapped := clock.NewRateLimiterWithTimeSource(ts, 1, 100)
		counted := NewCountedLimiter(wrapped)
		lim := NewShadowedLimiter(counted, allowlimiter{})

		run(t, lim, ts.Advance, counted.Collect)
	})
	t.Run("fallback", func(t *testing.T) {
		// "fallback" uses a different implementation, but it should count exactly the same.
		// TODO: ideally it would actually be the same code, but that's a bit awkward due to needing different interfaces.
		ts := clock.NewMockedTimeSource()
		wrapped := clock.NewRateLimiterWithTimeSource(ts, 1, 100)
		l := NewFallbackLimiter(allowlimiter{})
		l.Update(1)         // allows using primary, else it calls the fallback
		l.primary = wrapped // cheat, just swap it out

		run(t, l, ts.Advance, func() UsageMetrics {
			u, _, _ := l.Collect()
			return u
		})
	})
}

// Wait-based tests can block forever if there's an issue, better to fail fast.
func requireQuickly(t *testing.T, timeout time.Duration, cb func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		cb()
	}()
	wait := time.NewTimer(timeout)
	defer wait.Stop()
	select {
	case <-done:
	case <-wait.C: // should be far faster
		t.Fatal("timed out waiting for callback to return")
	}
}
