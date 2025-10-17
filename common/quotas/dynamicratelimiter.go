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

package quotas

import (
	"context"
	"math"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
)

const _ttl = time.Second * 5
const _minBurst = 1

// DynamicRateLimiter implements a dynamic config wrapper around the rate limiter,
// checks for updates to the dynamic config and updates the rate limiter accordingly
type DynamicRateLimiter struct {
	rps            RPSFunc
	rl             clock.Ratelimiter
	timeSource     clock.TimeSource
	ttl            time.Duration
	lastUpdateTime atomic.Pointer[time.Time]
	minBurst       int
	burstMultiplier float64
}

type DynamicRateLimiterOpts struct {
	TTL        time.Duration
	MinBurst   int
	BurstMultiplier float64
	TimeSource clock.TimeSource
}

var _defaultOpts = DynamicRateLimiterOpts{
	TTL:        _ttl,
	MinBurst:   _minBurst,
	BurstMultiplier: 1,
	TimeSource: clock.NewRealTimeSource(),
}

// NewDynamicRateLimiter returns a rate limiter which handles dynamic config
func NewDynamicRateLimiter(rps RPSFunc) Limiter {
	return NewDynamicRateLimiterWithOpts(rps, _defaultOpts)
}

func NewBurstyDynamicRateLimiter(rps RPSFunc, multiplier float64) Limiter {
	opts := _defaultOpts
	opts.BurstMultiplier = multiplier
	return NewDynamicRateLimiterWithOpts(rps, opts)
}

func NewDynamicRateLimiterWithOpts(rps RPSFunc, opts DynamicRateLimiterOpts) Limiter {
	ts := opts.TimeSource
	if ts == nil {
		ts = _defaultOpts.TimeSource
	}
	res := &DynamicRateLimiter{
		rps:        rps,
		timeSource: ts,
		ttl:        opts.TTL,
		minBurst:   opts.MinBurst,
		burstMultiplier: opts.BurstMultiplier,
	}
	now := res.timeSource.Now()
	res.lastUpdateTime.Store(&now)
	lim, burst := res.getLimitAndBurst()
	res.rl = clock.NewRateLimiterWithTimeSource(res.timeSource, lim, burst)
	return res
}

// Allow immediately returns with true or false indicating if a rate limit
// token is available or not
func (d *DynamicRateLimiter) Allow() bool {
	d.maybeRefreshRps()
	return d.rl.Allow()
}

// Wait waits up till deadline for a rate limit token
func (d *DynamicRateLimiter) Wait(ctx context.Context) error {
	d.maybeRefreshRps()
	return d.rl.Wait(ctx)
}

// Reserve reserves a rate limit token
func (d *DynamicRateLimiter) Reserve() clock.Reservation {
	d.maybeRefreshRps()
	return d.rl.Reserve()
}

func (d *DynamicRateLimiter) Limit() rate.Limit {
	d.maybeRefreshRps()
	return d.rl.Limit()
}

func (d *DynamicRateLimiter) maybeRefreshRps() {
	now := d.timeSource.Now()
	lastUpdated := d.lastUpdateTime.Load()
	if now.After(lastUpdated.Add(d.ttl-1)) && d.lastUpdateTime.CompareAndSwap(lastUpdated, &now) {
		d.rl.SetLimitAndBurst(d.getLimitAndBurst())
	}
}

func (d *DynamicRateLimiter) getLimitAndBurst() (rate.Limit, int) {
	rps := d.rps()
	burst := max(int(math.Ceil(rps)*d.burstMultiplier), d.minBurst)
	// If we have 0 rps we have to zero out the burst to immediately cut off new permits
	if rps == 0 {
		burst = 0
	}
	return rate.Limit(rps), burst
}
