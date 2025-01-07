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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/quotas"
)

func TestFallbackRegression(t *testing.T) {
	t.Run("primary should start with fallback limit", func(t *testing.T) {
		// checks for an issue in earlier versions, where a newly-enabled primary limit would start with an empty token bucket,
		// unfairly rejecting requests that other limiters would allow (as they are created with a full bucket at their target rate).
		//
		// this can be spotted by doing this sequence:
		// - create a new limiter, fallback of N per second
		//   - this starts the "primary" limiter with limit=0, burst=0.
		// - Allow() a request
		//   - the limiters here may disagree. this is fine, primary is not used yet.
		//   - however: this advanced the primary's internal "now" to now.  (this now happens only after update)
		// - update the limit to match the fallback
		//   - primary now has limit==N, burst==N, but tokens=0 and now=now.
		// - Allow() a request
		//   - fallback allows due to having N-1 tokens
		//   - primary rejects due to 0 tokens
		//     ^ this is the problem.
		//     ^ this could also happen if calling `ratelimiter.Limit()` sets "now" in the future, though this seems unlikely.
		//
		// this uses real time because sleeping is not necessary, and it ensures
		// that any time-advancing calls that occur too early will lead to ~zero tokens.

		limit := rate.Limit(10) // enough to accept all requests
		rl := NewFallbackLimiter(clock.NewRatelimiter(limit, int(limit)))

		// sanity check: this may be fine to change, but it will mean the test needs to be rewritten because the token bucket may not be empty.
		orig := rl.primary
		assert.Zero(t, orig.Burst(), "sanity check: default primary ratelimiter's burst is not zero, this test likely needs a rewrite")

		// simulate: call while still starting up, it should not touch the primary limiter.
		allowed := rl.Allow()
		before, starting, failing := rl.Collect()
		assert.True(t, allowed, "first request should have been allowed, on the fallback limiter")
		assert.True(t, starting, "should be true == in starting mode")
		assert.False(t, failing, "should be false == not failing")
		assert.Equal(t, UsageMetrics{
			Allowed:  1,
			Rejected: 0,
			Idle:     0,
		}, before)

		// update: this should set limits, and either fill the bucket or cause it to be filled on the next request
		rl.Update(limit)

		// call again: should be allowed, as this is the first time-touching request since it was created,
		// and the token bucket should have filled to match the first-update value.
		allowed = rl.Allow()
		after, starting, failing := rl.Collect()
		assert.True(t, allowed, "second request should have been allowed, on the primary limiter")
		assert.False(t, starting, "should be false == not in starting mode (using global)")
		assert.False(t, failing, "should be false == not failing")
		assert.Equal(t, UsageMetrics{
			Allowed:  1,
			Rejected: 0,
			Idle:     0,
		}, after)
		assert.InDeltaf(t,
			// Tokens() advances time, so this will not be precise.
			rl.primary.Tokens(), int(limit)-1, 0.1,
			"should have ~%v tokens: %v from the initial fill, minus 1 for the allow call",
			int(limit)-1, int(limit),
		)
	})
}

func TestLimiter(t *testing.T) {
	t.Run("uses fallback initially", func(t *testing.T) {
		m := quotas.NewMockLimiter(gomock.NewController(t))
		m.EXPECT().Allow().Times(1).Return(true)
		m.EXPECT().Allow().Times(2).Return(false)
		lim := NewFallbackLimiter(m)

		assert.True(t, lim.Allow(), "should return fallback's first response")
		assert.False(t, lim.Allow(), "should return fallback's second response")
		assert.False(t, lim.Allow(), "should return fallback's third response")

		usage, starting, failing := lim.Collect()
		assert.Equal(t, UsageMetrics{1, 2, 0}, usage, "usage metrics should match returned values")
		assert.True(t, starting, "should still be starting up")
		assert.False(t, failing, "should not be failing, still starting up")
	})
	t.Run("uses primary after update", func(t *testing.T) {
		lim := NewFallbackLimiter(allowlimiter{})
		lim.Update(1_000_000) // large enough to allow millisecond sleeps to refill

		time.Sleep(time.Millisecond) // allow some tokens to fill
		assert.True(t, lim.Allow(), "limiter allows after enough time has passed")
		assert.True(t, lim.Allow(), "limiter allows burst too")

		usage, startup, failing := lim.Collect()
		assert.False(t, failing, "should not use fallback limiter after update")
		assert.False(t, startup, "should not be starting up, has had an update")
		assert.Equal(t, UsageMetrics{2, 0, 0}, usage, "usage should match behavior")
	})

	t.Run("collecting usage data resets counts", func(t *testing.T) {
		lim := NewFallbackLimiter(allowlimiter{})
		lim.Update(1)
		lim.Allow()
		limit, _, _ := lim.Collect()
		assert.Equal(t, 1, limit.Allowed+limit.Rejected, "should count one request")
		limit, _, _ = lim.Collect()
		assert.Zero(t, limit.Allowed+limit.Rejected, "collect should have cleared the counts")
	})

	t.Run("use-fallback fuse", func(t *testing.T) {
		// duplicate to allow this test to be external, keep in sync by hand
		const maxFailedUpdates = 9
		t.Cleanup(func() {
			if t.Failed() { // notices sub-test failures
				t.Logf("maxFailedUpdates may be out of sync (%v), check hardcoded values", maxFailedUpdates)
			}
		})

		t.Run("falls back after too many failures", func(t *testing.T) {
			lim := NewFallbackLimiter(allowlimiter{}) // fallback behavior is ignored
			lim.Update(1)
			_, startup, failing := lim.Collect()
			require.False(t, failing, "should not be using fallback")
			require.False(t, startup, "should not be starting up, has had an update")

			// the bucket will fill from time 0 on the first update, ensuring the first request is allowed
			require.True(t, lim.Allow(), "rate.Limiter should start with a full bucket")

			// fail enough times to trigger a fallback
			for i := 0; i < maxFailedUpdates; i++ {
				// build up to the edge...
				lim.FailedUpdate()
				_, _, failing = lim.Collect()
				require.False(t, failing, "should not be using fallback after %n failed updates", i+1)
			}
			lim.FailedUpdate() // ... and push it over
			_, _, failing = lim.Collect()
			require.True(t, failing, "%vth update should switch to fallback", maxFailedUpdates+1)

			assert.True(t, lim.Allow(), "should return fallback's allowed request")
		})
		t.Run("failing many times does not accidentally switch away from startup mode", func(t *testing.T) {
			lim := NewFallbackLimiter(nil)
			for i := 0; i < maxFailedUpdates*10; i++ {
				lim.FailedUpdate()
				_, startup, failing := lim.Collect()
				require.True(t, startup, "should still be starting up %v failed updates", i+1)
				require.False(t, failing, "failing can only happen after startup finishess")
			}
		})
	})

	t.Run("coverage", func(t *testing.T) {
		// easy line to cover to bring to 100%
		lim := NewFallbackLimiter(nil)
		lim.Update(1)
		lim.Update(1) // should go down "no changes needed, return early" path
	})
}

func TestLimiterNotRacy(t *testing.T) {
	lim := NewFallbackLimiter(allowlimiter{})
	var g errgroup.Group
	const loops = 1000
	for i := 0; i < loops; i++ {
		// clear ~10% of the time
		if rand.Intn(10) == 0 {
			g.Go(func() error {
				lim.Reset()
				return nil
			})
		}
		// update ~10% of the time, fail the rest.
		// this should randomly clear occasionally via failures.
		if rand.Intn(10) == 0 {
			g.Go(func() error {
				lim.Update(rate.Limit(1 / rand.Float64())) // essentially never exercises "same value, do nothing" logic
				return nil
			})
		} else {
			g.Go(func() error {
				lim.FailedUpdate()
				return nil
			})
		}
		// collect occasionally
		if rand.Intn(10) == 0 {
			g.Go(func() error {
				lim.Collect()
				return nil
			})
		}
		g.Go(func() error {
			lim.Allow()
			return nil
		})
		g.Go(func() error {
			lim.Reserve().Used(rand.Int()%2 == 0)
			return nil
		})
		g.Go(func() error {
			lim.Limit()
			return nil
		})
		g.Go(func() error {
			lim.FallbackLimit()
			return nil
		})
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
			defer cancel()
			_ = lim.Wait(ctx)
			return nil
		})
	}
}

var _ quotas.Limiter = allowlimiter{}
var _ clock.Reservation = allowres{}

type allowlimiter struct{}
type allowres struct{}

func (allowlimiter) Allow() bool                  { return true }
func (a allowlimiter) Wait(context.Context) error { return nil }
func (a allowlimiter) Reserve() clock.Reservation { return allowres{} }
func (a allowlimiter) Limit() rate.Limit          { return rate.Inf }

func (a allowres) Allow() bool { return true }
func (a allowres) Used(bool)   {}
