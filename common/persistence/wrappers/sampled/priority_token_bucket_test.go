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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/clock"
)

func TestPriorityTokenBucketRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := newFullPriorityTokenBucket(1, 99, ts)

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
			ts.Advance(101 * time.Millisecond)
		}
		assert.Equal(t, 90, total, "token bucket failed to enforce limit")
		assert.Equal(t, 9, attempts, "token bucket gave out tokens too quickly")

		ts.Advance(101 * time.Millisecond)
		ok, _ := tb.GetToken(0, 9)
		assert.True(t, ok, "token bucket failed to enforce limit")
		ok, _ = tb.GetToken(0, 1)
		assert.False(t, ok, "token bucket failed to enforce limit")
		ts.Advance(time.Second)
	}
}

func TestPriorityTokenBucketLowRpsEnforced(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := newFullPriorityTokenBucket(1, 3, ts)

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
		ts.Advance(101 * time.Millisecond)
	}
	assert.Equal(t, 3, total, "token bucket failed to enforce limit")
	assert.Equal(t, 3, attempts, "token bucket gave out tokens too quickly")
}

func TestPriorityTokenBucketSpillsUnusedTokensToLowerPriority(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	tb := newFullPriorityTokenBucket(2, 100, ts)

	ok, _ := tb.GetToken(1, 10)
	assert.True(t, ok)

	for i := 0; i < 2; i++ {
		ok, _ := tb.GetToken(1, 1)
		assert.False(t, ok)
		ok, _ = tb.GetToken(0, 10)
		assert.True(t, ok)
		ts.Advance(101 * time.Millisecond)
	}

	ok, _ = tb.GetToken(1, 1)
	assert.False(t, ok)
	ts.Advance(101 * time.Millisecond)
	ok, _ = tb.GetToken(1, 5)
	assert.True(t, ok)
	ts.Advance(101 * time.Millisecond)
	ok, _ = tb.GetToken(1, 15)
	assert.False(t, ok)
	ok, _ = tb.GetToken(1, 10)
	assert.True(t, ok)
	ok, _ = tb.GetToken(0, 10)
	assert.True(t, ok)
}
