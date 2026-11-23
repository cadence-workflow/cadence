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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
)

func TestNewSimpleDynamicRateLimiterFactory(t *testing.T) {
	const _testDomain = "test-domain"

	factory := NewSimpleDynamicRateLimiterFactory(func(domain string) int {
		assert.Equal(t, _testDomain, domain)
		return 100
	})

	limiter := factory.GetLimiter(_testDomain)

	assert.Equal(t, rate.Limit(100), limiter.Limit())
}

func TestFactoryControlledRefill(t *testing.T) {
	ts := clock.NewMockedTimeSource()
	factory := NewSimpleDynamicRateLimiterFactoryWithTimeSource(func(string) int { return 5 }, ts)
	limiter := factory.GetLimiter("domain")
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Fatal("initial burst rejected")
		}
	}
	if limiter.Allow() {
		t.Fatal("exhausted burst allowed")
	}
	ts.Advance(200 * time.Millisecond)
	if !limiter.Allow() {
		t.Fatal("one token did not refill")
	}
	if limiter.Allow() {
		t.Fatal("more than one token refilled")
	}
}
