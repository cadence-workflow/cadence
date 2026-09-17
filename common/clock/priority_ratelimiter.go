package clock

import (
	"sync"
	"time"
)

type (
	// PriorityRatelimiter is a rate limiter that supports multiple priority levels.
	// There are n buckets for n priorities. Higher priority buckets are refilled
	// first, and tokens that overflow a full bucket flow to the next-lower priority.
	// Thread safe.
	PriorityRatelimiter interface {
		// GetToken attempts to take count tokens from the bucket with the given
		// priority. Priority 0 is the highest. Returns true on success, false
		// otherwise along with the duration until the next refill.
		GetToken(priority, count int) (bool, time.Duration)
	}

	priorityRatelimiter struct {
		sync.Mutex
		tokens         []int
		fillRate       int
		nextRefillTime time.Time
		// Because we divide the per-second quota equally
		// every 100 millis, there could be a remainder when
		// the desired rate is not a multiple 10 (1second/100Millis)
		// To overcome this, we keep track of left over remainder
		// and distribute this evenly during every fillInterval
		overflowRps            int
		overflowTokens         int
		nextOverflowRefillTime time.Time
		timeSource             TimeSource
	}
)

const (
	millisPerSecond = 1000
	priorityRefill  = 100 * time.Millisecond
)

// NewPriorityRatelimiter creates and returns a new priority rate limiter.
// It replenishes the top priority bucket every 100 milliseconds, and unused
// tokens flow to the next bucket. The idea comes from Dual Token Bucket
// Algorithms. Thread safe.
func NewPriorityRatelimiter(numOfPriority, rps int, timeSource TimeSource) PriorityRatelimiter {
	rl := new(priorityRatelimiter)
	rl.tokens = make([]int, numOfPriority)
	rl.timeSource = timeSource
	rl.fillRate = (rps * 100) / millisPerSecond
	rl.overflowRps = rps - (10 * rl.fillRate)
	rl.refill(rl.timeSource.Now())
	return rl
}

// NewFullPriorityRatelimiter creates and returns a new priority rate limiter with
// all buckets initialized with full tokens. With all buckets full, tokens from low
// priority buckets won't be missed initially, but may cause bursts.
func NewFullPriorityRatelimiter(numOfPriority, rps int, timeSource TimeSource) PriorityRatelimiter {
	rl := new(priorityRatelimiter)
	rl.tokens = make([]int, numOfPriority)
	rl.timeSource = timeSource
	rl.fillRate = (rps * 100) / millisPerSecond
	rl.overflowRps = rps - (10 * rl.fillRate)
	rl.refill(rl.timeSource.Now())
	for i := 1; i < numOfPriority; i++ {
		rl.nextRefillTime = time.Time{}
		rl.refill(rl.timeSource.Now())
	}
	return rl
}

func (rl *priorityRatelimiter) GetToken(priority, count int) (bool, time.Duration) {
	now := rl.timeSource.Now()
	rl.Lock()
	rl.refill(now)
	nextRefillTime := rl.nextRefillTime.Sub(now)
	if rl.tokens[priority] < count {
		rl.Unlock()
		return false, nextRefillTime
	}
	rl.tokens[priority] -= count
	rl.Unlock()
	return true, nextRefillTime
}

func (rl *priorityRatelimiter) refill(now time.Time) {
	rl.refillOverflow(now)
	if rl.isRefillDue(now) {
		more := rl.fillRate
		for i := 0; i < len(rl.tokens); i++ {
			rl.tokens[i] += more
			if rl.tokens[i] > rl.fillRate {
				more = rl.tokens[i] - rl.fillRate
				rl.tokens[i] = rl.fillRate
			} else {
				break
			}
		}
		if rl.overflowTokens > 0 {
			rl.tokens[0]++
			rl.overflowTokens--
		}
		rl.nextRefillTime = now.Add(priorityRefill)
	}
}

func (rl *priorityRatelimiter) refillOverflow(now time.Time) {
	if rl.overflowRps < 1 {
		return
	}
	if rl.isOverflowRefillDue(now) {
		rl.overflowTokens = rl.overflowRps
		rl.nextOverflowRefillTime = now.Add(time.Second)
	}
}

func (rl *priorityRatelimiter) isRefillDue(now time.Time) bool {
	return now.Compare(rl.nextRefillTime) >= 0
}

func (rl *priorityRatelimiter) isOverflowRefillDue(now time.Time) bool {
	return now.Compare(rl.nextOverflowRefillTime) >= 0
}
