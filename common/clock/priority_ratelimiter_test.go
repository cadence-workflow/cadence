package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPriorityRatelimiterRpsEnforced(t *testing.T) {
	ts := NewMockedTimeSource()
	rl := NewPriorityRatelimiter(1, 99, ts) // behavior same as the standard limiter

	for i := 0; i < 2; i++ {
		total := 0
		attempts := 1
		for ; attempts < 11; attempts++ {
			for c := 0; c < 2; c++ {
				if ok, _ := rl.GetToken(0, 10); ok {
					total += 10
				}
			}

			if total >= 90 {
				break
			}
			ts.Advance(time.Millisecond * 101)
		}
		assert.Equal(t, 90, total, "rate limiter failed to enforce limit")
		assert.Equal(t, 9, attempts, "rate limiter gave out tokens too quickly")

		ts.Advance(time.Millisecond * 101)
		ok, _ := rl.GetToken(0, 9)
		assert.True(t, ok, "rate limiter failed to enforce limit")
		ok, _ = rl.GetToken(0, 1)
		assert.False(t, ok, "rate limiter failed to enforce limit")
		ts.Advance(time.Second)
	}
}

func TestPriorityRatelimiterLowRpsEnforced(t *testing.T) {
	ts := NewMockedTimeSource()
	rl := NewPriorityRatelimiter(1, 3, ts) // behavior same as the standard limiter

	total := 0
	attempts := 1
	for ; attempts < 10; attempts++ {
		for c := 0; c < 2; c++ {
			if ok, _ := rl.GetToken(0, 1); ok {
				total++
			}
		}
		if total >= 3 {
			break
		}
		ts.Advance(time.Millisecond * 101)
	}
	assert.Equal(t, 3, total, "rate limiter failed to enforce limit")
	assert.Equal(t, 3, attempts, "rate limiter gave out tokens too quickly")
}

func TestPriorityRatelimiter(t *testing.T) {
	ts := NewMockedTimeSource()
	rl := NewPriorityRatelimiter(2, 100, ts)

	for i := 0; i < 2; i++ {
		ok2, _ := rl.GetToken(1, 1)
		assert.False(t, ok2)
		ok, _ := rl.GetToken(0, 10)
		assert.True(t, ok)
		ts.Advance(time.Millisecond * 101)
	}

	for i := 0; i < 2; i++ {
		ok, _ := rl.GetToken(0, 9)
		assert.True(t, ok) // 1 token remaining in 1st bucket, 0 in 2nd
		ok2, _ := rl.GetToken(1, 1)
		assert.False(t, ok2)
		ts.Advance(time.Millisecond * 101)
		ok2, _ = rl.GetToken(1, 2)
		assert.False(t, ok2)
		ok2, _ = rl.GetToken(1, 1)
		assert.True(t, ok2)
	}
}

func TestFullPriorityRatelimiter(t *testing.T) {
	ts := NewMockedTimeSource()
	rl := NewFullPriorityRatelimiter(2, 100, ts)

	ok2, _ := rl.GetToken(1, 10)
	assert.True(t, ok2)

	for i := 0; i < 2; i++ {
		ok2, _ := rl.GetToken(1, 1)
		assert.False(t, ok2)
		ok, _ := rl.GetToken(0, 10)
		assert.True(t, ok)
		ts.Advance(time.Millisecond * 101)
	}

	ok2, _ = rl.GetToken(1, 1)
	assert.False(t, ok2)
	ts.Advance(time.Millisecond * 101)
	ok2, _ = rl.GetToken(1, 5)
	assert.True(t, ok2)
	ts.Advance(time.Millisecond * 101)
	ok2, _ = rl.GetToken(1, 15)
	assert.False(t, ok2)
	ok2, _ = rl.GetToken(1, 10)
	assert.True(t, ok2)
	ok, _ := rl.GetToken(0, 10)
	assert.True(t, ok)
}
