// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package tokenbucket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
)

func TestRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := New(99, ts)
	for i := 0; i < 2; i++ {
		total, attempts := testRpsEnforcedHelper(tb, ts, 11, 90, 10)
		assert.Equal(t, 90, total, "Token bucket failed to enforce limit")
		assert.Equal(t, 9, attempts, "Token bucket gave out tokens too quickly")
		ts.Advance(time.Millisecond * 101)
		ok, _ := tb.TryConsume(9)
		assert.True(t, ok, "Token bucket failed to enforce limit")
		ok, _ = tb.TryConsume(1)
		assert.False(t, ok, "Token bucket failed to enforce limit")
		ts.Advance(time.Second)
	}
}

func TestLowRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := New(3, ts)

	total, attempts := testRpsEnforcedHelper(tb, ts, 10, 3, 1)
	assert.Equal(t, 3, total, "Token bucket failed to enforce limit")
	assert.Equal(t, 3, attempts, "Token bucket gave out tokens too quickly")
}

func TestDynamicRpsEnforced(t *testing.T) {
	rpsConfigFn, rpsPtr := getTestRPSConfigFn(99)
	ts := clock.NewMockedTimeSource()
	dtb := NewDynamicTokenBucket(rpsConfigFn, ts)
	total, attempts := testRpsEnforcedHelper(dtb, ts, 11, 90, 10)
	assert.Equal(t, 90, total, "Token bucket failed to enforce limit")
	assert.Equal(t, 9, attempts, "Token bucket gave out tokens too quickly")
	ts.Advance(time.Second)

	*rpsPtr = 3
	total, attempts = testRpsEnforcedHelper(dtb, ts, 10, 3, 1)
	assert.Equal(t, 3, total, "Token bucket failed to enforce limit")
	assert.Equal(t, 3, attempts, "Token bucket gave out tokens too quickly")
}

func testRpsEnforcedHelper(tb TokenBucket, ts clock.MockedTimeSource, maxAttempts, tokenNeeded, consumeRate int) (total, attempts int) {
	total = 0
	attempts = 1
	for ; attempts < maxAttempts+1; attempts++ {
		for c := 0; c < 2; c++ {
			if ok, _ := tb.TryConsume(consumeRate); ok {
				total += consumeRate
			}
		}
		if total >= tokenNeeded {
			break
		}
		ts.Advance(time.Millisecond * 101)
	}
	return
}

func getTestRPSConfigFn(defaultValue int) (dynamicproperties.IntPropertyFn, *int) {
	rps := defaultValue
	return func(_ ...dynamicproperties.FilterOption) int {
		return rps
	}, &rps
}

func TestPriorityRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := NewPriorityTokenBucket(1, 99, ts) // behavior same to tokenBucketImpl

	for i := 0; i < 2; i++ {
		total := 0
		attempts := 1
		for ; attempts < 11; attempts++ {
			for c := 0; c < 2; c++ {
				if ok, _ := tb.GetToken(0, 10); ok {
					total += 10
				}
			}

			if total >= 90 {
				break
			}
			ts.Advance(time.Millisecond * 101)
		}
		assert.Equal(t, 90, total, "Token bucket failed to enforce limit")
		assert.Equal(t, 9, attempts, "Token bucket gave out tokens too quickly")

		ts.Advance(time.Millisecond * 101)
		ok, _ := tb.GetToken(0, 9)
		assert.True(t, ok, "Token bucket failed to enforce limit")
		ok, _ = tb.GetToken(0, 1)
		assert.False(t, ok, "Token bucket failed to enforce limit")
		ts.Advance(time.Second)
	}
}

func TestPriorityLowRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := NewPriorityTokenBucket(1, 3, ts) // behavior same to tokenBucketImpl

	total := 0
	attempts := 1
	for ; attempts < 10; attempts++ {
		for c := 0; c < 2; c++ {
			if ok, _ := tb.GetToken(0, 1); ok {
				total++
			}
		}
		if total >= 3 {
			break
		}
		ts.Advance(time.Millisecond * 101)
	}
	assert.Equal(t, 3, total, "Token bucket failed to enforce limit")
	assert.Equal(t, 3, attempts, "Token bucket gave out tokens too quickly")
}

func TestPriorityTokenBucket(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := NewPriorityTokenBucket(2, 100, ts)

	for i := 0; i < 2; i++ {
		ok2, _ := tb.GetToken(1, 1)
		assert.False(t, ok2)
		ok, _ := tb.GetToken(0, 10)
		assert.True(t, ok)
		ts.Advance(time.Millisecond * 101)
	}

	for i := 0; i < 2; i++ {
		ok, _ := tb.GetToken(0, 9)
		assert.True(t, ok) // 1 token remaining in 1st bucket, 0 in 2nd
		ok2, _ := tb.GetToken(1, 1)
		assert.False(t, ok2)
		ts.Advance(time.Millisecond * 101)
		ok2, _ = tb.GetToken(1, 2)
		assert.False(t, ok2)
		ok2, _ = tb.GetToken(1, 1)
		assert.True(t, ok2)
	}
}

func TestFullPriorityTokenBucket(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := NewFullPriorityTokenBucket(2, 100, ts)

	ok2, _ := tb.GetToken(1, 10)
	assert.True(t, ok2)

	for i := 0; i < 2; i++ {
		ok2, _ := tb.GetToken(1, 1)
		assert.False(t, ok2)
		ok, _ := tb.GetToken(0, 10)
		assert.True(t, ok)
		ts.Advance(time.Millisecond * 101)
	}

	ok2, _ = tb.GetToken(1, 1)
	assert.False(t, ok2)
	ts.Advance(time.Millisecond * 101)
	ok2, _ = tb.GetToken(1, 5)
	assert.True(t, ok2)
	ts.Advance(time.Millisecond * 101)
	ok2, _ = tb.GetToken(1, 15)
	assert.False(t, ok2)
	ok2, _ = tb.GetToken(1, 10)
	assert.True(t, ok2)
	ok, _ := tb.GetToken(0, 10)
	assert.True(t, ok)
}

func TestTokenBucketConsume(t *testing.T) {
	t.Run("consume_wait", func(t *testing.T) {
		// make sure we don't deadlock inside the test.
		go panicOnTimeout(time.Minute)()

		ts := clock.NewMockedTimeSource()

		// I provide 10 rps, which means I can consume 10 tokens per second.
		// tokenBucketImpl will fill 1 token and then refill it every 100 milliseconds.
		tb := New(10, ts)

		// I consume 1 token, so the next Consume call will block until refill.
		ok, _ := tb.TryConsume(1)
		assert.True(t, ok)

		consumeFinished := make(chan struct{})
		go func() {
			success := tb.Consume(1, 2*refillRate)
			assert.True(t, success, "Consume must acquire token successfully")
			close(consumeFinished)
		}()

		// I need to make sure that goroutine with Consume call is blocked on time.Sleep.
		// BlockUntil awaits for at least 1 goroutine to be blocked on time.Sleep/timer/ticker.
		ts.BlockUntil(1)
		// Checking that consume is still blocked.
		select {
		case <-consumeFinished:
			assert.Fail(t, "Consume returned before refill")
		default:
		}
		// tb has internal retries to acquire tokens which happens at most backoffInterval (10 * time.Millisecond).
		// I advance time once, so the next iteration is still blocked.
		ts.Advance(backoffInterval + 1)
		// wait until Consume did one iteration and stopped on time.Sleep
		ts.BlockUntil(1)
		// Checking that consume is still blocked.
		select {
		case <-consumeFinished:
			assert.Fail(t, "Consume returned before refill")
		default:
		}
		// Advance time to the next refill time.
		ts.Advance(refillRate - backoffInterval + 1)
		// Advance should unblock Consume and it should return afterwards.
		// I don't provide a timeout, since I have a goroutine to panic on timeout.
		<-consumeFinished
	})
	t.Run("timeout", func(t *testing.T) {
		// make sure we don't deadlock inside the test.
		go panicOnTimeout(time.Minute)()

		ts := clock.NewMockedTimeSource()
		// I provide 10 rps, which means I can consume 10 tokens per second.
		// tokenBucketImpl will fill 1 token and then refill it every 100 milliseconds.
		tb := New(10, ts)

		// I consume 1 token, so the next Consume call will block until refill.
		ok, _ := tb.TryConsume(1)
		assert.True(t, ok)

		// This time in consume I provide a small timeout of backoffInterval/2.
		// So Consume will do only one internal iteration and then return before refill.
		consumeFinished := make(chan struct{})
		go func() {
			success := tb.Consume(1, backoffInterval/2)
			assert.False(t, success, "Consume must fail to acquire token")
			close(consumeFinished)
		}()

		// I need to make sure that goroutine with Consume call is blocked on time.Sleep.
		// BlockUntil awaits for at least 1 goroutine to be blocked on time.Sleep/timer/ticker.
		ts.BlockUntil(1)
		select {
		case <-consumeFinished:
			assert.Fail(t, "Consume returned before refill")
		default:
		}

		// advance time to wake Consume up and make it return before refill.
		ts.Advance(backoffInterval/2 + 1)
		<-consumeFinished
	})
}

func panicOnTimeout(d time.Duration) (cancel func()) {
	stop := make(chan struct{})
	go func() {
		select {
		case <-time.After(d):
			panic("timeout")
		case <-stop: // noop
		}
	}()
	return func() { close(stop) }
}
