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

// Package internal protects these types' concurrency primitives and other
// internals from accidental misuse.
package internal

import (
	"context"
	"math"
	"sync/atomic"

	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/quotas"
)

type (
	// FallbackLimiter wraps a "primary" [rate.Limiter] with a "fallback" Limiter (i.e. a [github.com/uber/cadence/common/quotas.Limiter])
	// to use before the primary is fully ready, or after too many attempts to update the "primary" limit have failed.
	//
	// Intended use is:
	//   - "primary" is the global-load-balanced ratelimiter, with weights updating every few seconds
	//   - "fallback" is the "rps / num-of-hosts" ratelimiters we use many other places, which do not need to exchange data to work
	//   - When "primary" is not yet initialized (no updates yet at startup) or the global-load-balancing system is broken (too many failures),
	//     this limiter internally switches to the "fallback" limiter until it recovers.  Otherwise, "primary" is preferred.
	//   - both limiters are called all the time to keep their used-tokens roughly in sync (via shadowedLimiter), so they
	//     can be switched between without allowing bursts due to the "other" limit's token bucket filling up.
	//
	// The owning global/collection.Collection uses this as follows:
	//   - every couple seconds, Collect() usage information from all limiters
	//   - submit this to aggregator hosts, which return how much weight to allow this host per limiter
	//   - per limiter:
	//     - if a weight was returned, Update(rps) it so the new value is used in the future
	//     - else, call FailedUpdate() to shorten the "use fallback" fuse, possibly switching to the fallback by doing so
	//
	// During this sequence, a requested limit may not be returned by an aggregator for two major reasons,
	// and will result in a FailedUpdate() call to shorten a "use fallback logic" fuse:
	//   1. this limit is legitimately unused (or idle too long) and no data exists for it
	//   2. ring re-sharding led to losing track of the limit, and it is now owned by a host with insufficient data
	//
	// To mitigate the impact of the second case, missing data is temporarily ignored, and the previously-configured
	// Update rate is used without any changes.  This gives the aggregating host time to fill in its data, and a later
	// cycle should return "real" values that match actual usage, Update-ing this limiter.
	//
	// If no data has been returned for a sufficiently long time, the "primary" ratelimit will be ignored, and
	// the fallback limit will be used exclusively.  This is intended as a safety fallback, e.g. during initial
	// rollout/rollback and outage scenarios, normal use is not expected to rely on it.
	//
	// -----
	//
	// Implementation notes:
	//   - atomics are used as values instead of pointers simply for data locality, which requires FallbackLimiter
	//     to be used as a pointer.  `go vet -copylocks` should be enough to ensure this is done correctly, which is
	//     enforced by `make lint`.
	//   - the variety of atomics here is almost certainly NOT necessary for performance,
	//     it was just possible and simple enough to use, and it makes deadlocks trivially impossible.
	//     but rate.Limiter already use mutexes internally, so adding another to track counts / fallback
	//     decisions / etc should be entirely fine, if it becomes needed.
	FallbackLimiter struct {
		// number of failed updates, for deciding when to use fallback
		failedUpdates atomic.Int64
		// counts of what was allowed vs not.
		// this is not collected by a wrapper because it would hide the limit-setting methods,
		// but it is otherwise identical to a CountedLimiter around this FallbackLimiter.
		usage AtomicUsage

		// primary / desired limiter, based on load-balanced information.
		//
		// note that use and modification is NOT synchronized externally,
		// so updates and deciding when to use the fallback must be done carefully
		// to avoid undesired combinations when they interleave.
		//
		// if needed or desired, just switch to locks, they should be more than
		// fast enough because the limiter already uses locking internally.
		primary clock.Ratelimiter
		// fallback used when failedUpdates exceeds maxFailedUpdates,
		// or prior to the first Update call with globally-adjusted values.
		fallback quotas.Limiter
	}
)

const (
	// when failed updates exceeds this value, use the fallback
	maxFailedUpdates = 9

	// at startup / new limits in use, use the fallback logic.
	// that's expected / normal behavior as we have no data yet.
	//
	// positive values risk being interpreted as a "failure" though, so start deeply
	// negative, so it can be identified as "still starting up".
	// min-int64 has more than enough room to "count" failed updates for eons
	// without becoming positive, so it should not risk being misinterpreted.
	initialFailedUpdates = math.MinInt64
)

// NewFallbackLimiter returns a quotas.Limiter that uses a simpler fallback when necessary,
// and attempts to keep both the fallback and the "real" limiter "warm" by mirroring calls
// between the two regardless of which is being used.
	func NewFallbackLimiter(fallback quotas.Limiter) *FallbackLimiter {
	l := &FallbackLimiter{
		// start from 0 as a default, the limiter is unused until it is updated.
		//
		// caution: it's important to not call any time-advancing methods on this until
		// after the first update, so the token bucket will fill properly.
		//
		// this (partially) mimics how new ratelimiters start with a full token bucket,
		// but there does not seem to be any way to perfectly mimic it without using locks,
		// and hopefully precision is not needed.
		primary:  clock.NewRatelimiter(0, 0),
		fallback: fallback,
	}
	l.failedUpdates.Store(initialFailedUpdates)
	return l
}

// Collect returns the current allowed/rejected values and resets them to zero (like CountedLimiter).
// it also returns info about if this limiter is currently starting up or in a "use fallback due to failures" mode.
// "starting" is expected any time a limit is new (or not recently used), "failing" may happen rarely
// but should not ever be a steady event unless something is broken.
func (b *FallbackLimiter) Collect() (usage UsageMetrics, starting, failing bool) {
	usage = b.usage.Collect()
	starting, failing = b.mode()
	return usage, starting, failing
}

func (b *FallbackLimiter) mode() (startingUp, tooManyFailures bool) {
	failed := b.failedUpdates.Load()
	startingUp = failed < 0
	tooManyFailures = failed > maxFailedUpdates
	return startingUp, tooManyFailures
}

func (b *FallbackLimiter) useFallback() bool {
	starting, fallback := b.mode()
	return starting || fallback
}

// Update adjusts the underlying "primary" ratelimit, and resets the fallback fuse.
// This implies switching to the "primary" limiter - if that is not desired, call Reset() immediately after.
func (b *FallbackLimiter) Update(lim rate.Limit) {
	// caution: order here matters, to prevent potentially-old limiter values from being used
	// before they are updated.
	//
	// this is probably not going to be noticeable, but some users are sensitive to ANY
	// requests being ratelimited.  updating the fallback fuse last should be more reliable
	// in preventing that from happening when they would not otherwise be limited, e.g. with
	// the initial value of 0 burst, or if this rps was very low previously and is now high
	// (because the token bucket will be empty).
	defer func() {
		// reset the use-fallback fuse, which may also (re)enable using the "primary" limiter
		b.failedUpdates.Store(0)
	}()

	if b.primary.Limit() == lim {
		return
	}

	b.primary.SetLimit(lim)
	b.primary.SetBurst(max(1, int(lim))) // 0 burst blocks all requests, so allow at least 1 and rely on rps to fill sanely
}

// FailedUpdate should be called when a limit fails to update from an aggregator,
// possibly implying some kind of problem, which may be unique to this limit.
//
// After crossing a threshold of failures (currently 10), the fallback will be switched to.
func (b *FallbackLimiter) FailedUpdate() (failures int) {
	failures = int(b.failedUpdates.Add(1)) // always increment the count for monitoring purposes
	return failures
}

// Reset defers to the fallback limiter until an update is received.
//
// This is intended to be used when the current limit is no longer trustworthy for some reason,
// determined externally rather than from too many FailedUpdate calls.
func (b *FallbackLimiter) Reset() {
	b.failedUpdates.Store(initialFailedUpdates)
}

func (b *FallbackLimiter) Allow() bool {
	allowed := b.both().Allow()
	b.usage.Count(allowed)
	return allowed
}

func (b *FallbackLimiter) Wait(ctx context.Context) error {
	err := b.both().Wait(ctx)
	b.usage.Count(err == nil)
	return err
}

func (b *FallbackLimiter) Reserve() clock.Reservation {
	return countedReservation{
		wrapped: b.both().Reserve(),
		usage:   &b.usage,
	}
}

func (b *FallbackLimiter) Limit() rate.Limit {
	if b.useFallback() {
		return b.fallback.Limit()
	}
	return b.primary.Limit()
}

func (b *FallbackLimiter) FallbackLimit() rate.Limit {
	return b.fallback.Limit()
}

func (b *FallbackLimiter) both() quotas.Limiter {
	starting, failing := b.mode()
	if starting {
		// don't touch the primary until an update occurs,
		// to allow the token bucket to fill properly.
		return b.fallback
	}
	if failing {
		// keep shadowing calls, so the token buckets are similar.
		// this prevents allowing a full burst when recovering, which seems
		// reasonable as things were apparently unhealthy.
		return NewShadowedLimiter(b.fallback, b.primary)
	}
	return NewShadowedLimiter(b.primary, b.fallback)
}
