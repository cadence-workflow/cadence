// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package sampled

import (
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
)

type priorityTokenBucket struct {
	sync.Mutex
	tokens                 []int
	fillRate               int
	nextRefillTime         time.Time
	overflowRps            int
	overflowTokens         int
	nextOverflowRefillTime time.Time
	timeSource             clock.TimeSource
}

const (
	millisPerSecond = 1000
	refillRate      = 100 * time.Millisecond
)

// newFullPriorityTokenBucket creates a priority rate limiter with every
// priority bucket initially full. Lower priorities receive capacity only when
// higher-priority buckets do not use their full share.
func newFullPriorityTokenBucket(numOfPriority, rps int, timeSource clock.TimeSource) PriorityRateLimiter {
	tb := &priorityTokenBucket{
		tokens:      make([]int, numOfPriority),
		timeSource:  timeSource,
		fillRate:    (rps * 100) / millisPerSecond,
		overflowRps: rps - (10 * ((rps * 100) / millisPerSecond)),
	}
	tb.refill(tb.timeSource.Now())
	for i := 1; i < numOfPriority; i++ {
		tb.nextRefillTime = time.Time{}
		tb.refill(tb.timeSource.Now())
	}
	return tb
}

func (tb *priorityTokenBucket) GetToken(priority, count int) (bool, time.Duration) {
	now := tb.timeSource.Now()
	tb.Lock()
	defer tb.Unlock()

	tb.refill(now)
	nextRefillTime := tb.nextRefillTime.Sub(now)
	if tb.tokens[priority] < count {
		return false, nextRefillTime
	}
	tb.tokens[priority] -= count
	return true, nextRefillTime
}

func (tb *priorityTokenBucket) refill(now time.Time) {
	tb.refillOverflow(now)
	if !tb.isRefillDue(now) {
		return
	}

	more := tb.fillRate
	for i := 0; i < len(tb.tokens); i++ {
		tb.tokens[i] += more
		if tb.tokens[i] > tb.fillRate {
			more = tb.tokens[i] - tb.fillRate
			tb.tokens[i] = tb.fillRate
		} else {
			break
		}
	}
	if tb.overflowTokens > 0 {
		tb.tokens[0]++
		tb.overflowTokens--
	}
	tb.nextRefillTime = now.Add(refillRate)
}

func (tb *priorityTokenBucket) refillOverflow(now time.Time) {
	if tb.overflowRps < 1 {
		return
	}
	if tb.isOverflowRefillDue(now) {
		tb.overflowTokens = tb.overflowRps
		tb.nextOverflowRefillTime = now.Add(time.Second)
	}
}

func (tb *priorityTokenBucket) isRefillDue(now time.Time) bool {
	return now.Compare(tb.nextRefillTime) >= 0
}

func (tb *priorityTokenBucket) isOverflowRefillDue(now time.Time) bool {
	return now.Compare(tb.nextOverflowRefillTime) >= 0
}
