// Copyright (c) 2019 Uber Technologies, Inc.
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

package quotas

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
)

const (
	defaultRps    = 2000
	defaultDomain = "test"
	_minBurst     = 10000
)

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()
	maxDispatch := 0.01
	rl := NewRateLimiter(&maxDispatch, time.Second, _minBurst)
	limiter := rl.goRateLimiter.Load().(clock.Ratelimiter)
	assert.Equal(t, _minBurst, limiter.Burst())
	assert.Equal(t, maxDispatch, float64(limiter.Limit()))
}

func TestSimpleRatelimiter(t *testing.T) {
	// largely for coverage, as this is a test-helper that is used in other packages
	l := NewSimpleRateLimiter(t, 5)
	assert.Equal(t, rate.Limit(5), l.Limit())
	assert.True(t, l.Allow(), "should allow one request through")
	updated := 3.0 // must be lower than current value or it will not update
	l.UpdateMaxDispatch(&updated)
	assert.Equal(t, rate.Limit(3), l.Limit(), "should have immediately updated to new lower value")
}

func TestMultiStageRateLimiterBlockedByDomainRps(t *testing.T) {
	t.Parallel()
	policy := newFixedRpsMultiStageRateLimiter(2, 1)
	check := func(suffix string) {
		assert.True(t, policy.Allow(Info{Domain: defaultDomain}), "first should work"+suffix)
		assert.False(t, policy.Allow(Info{Domain: defaultDomain}), "second should be limited"+suffix) // smaller local limit applies
		assert.False(t, policy.Allow(Info{Domain: defaultDomain}), "third should be limited"+suffix)
	}

	check("")
	// allow bucket to refill
	time.Sleep(time.Second)
	check(" after refresh")
}

func TestMultiStageRateLimiterBlockedByGlobalRps(t *testing.T) {
	t.Parallel()
	policy := newFixedRpsMultiStageRateLimiter(1, 2)
	check := func(suffix string) {
		assert.True(t, policy.Allow(Info{Domain: defaultDomain}), "first should work"+suffix)
		assert.False(t, policy.Allow(Info{Domain: defaultDomain}), "second should be limited"+suffix) // smaller global limit applies
		assert.False(t, policy.Allow(Info{Domain: defaultDomain}), "third should be limited"+suffix)
	}

	check("")
	// allow bucket to refill
	time.Sleep(time.Second)
	check(" after refill")
}

func TestMultiStageRateLimitingMultipleDomains(t *testing.T) {
	t.Parallel()
	policy := newFixedRpsMultiStageRateLimiter(2, 1) // should allow 1/s per domain, 2/s total

	check := func(suffix string) {
		assert.True(t, policy.Allow(Info{Domain: "one"}), "1:1 should work"+suffix)
		assert.False(t, policy.Allow(Info{Domain: "one"}), "1:2 should be limited"+suffix) // per domain limited

		assert.True(t, policy.Allow(Info{Domain: "two"}), "2:1 should work"+suffix)
		assert.False(t, policy.Allow(Info{Domain: "two"}), "2:2 should be limited"+suffix) // per domain limited + global limited

		// third domain should be entirely cut off by global and cannot perform any requests
		assert.False(t, policy.Allow(Info{Domain: "three"}), "3:1 should be limited"+suffix) // allowed by domain, but limited by global
	}

	check("")
	// allow bucket to refill
	time.Sleep(time.Second)
	check(" after refill")
}

func BenchmarkRateLimiter(b *testing.B) {
	rps := float64(defaultRps)
	limiter := NewRateLimiter(&rps, 2*time.Minute, defaultRps)
	for n := 0; n < b.N; n++ {
		limiter.Allow()
	}
}

func BenchmarkMultiStageRateLimiter(b *testing.B) {
	policy := newFixedRpsMultiStageRateLimiter(defaultRps, defaultRps)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: defaultDomain})
	}
}

func BenchmarkMultiStageRateLimiter20Domains(b *testing.B) {
	numDomains := 20
	policy := newFixedRpsMultiStageRateLimiter(defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func BenchmarkMultiStageRateLimiter100Domains(b *testing.B) {
	numDomains := 100
	policy := newFixedRpsMultiStageRateLimiter(defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func BenchmarkMultiStageRateLimiter1000Domains(b *testing.B) {
	numDomains := 1000
	policy := newFixedRpsMultiStageRateLimiter(defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func newFixedRpsMultiStageRateLimiter(globalRps float64, domainRps int) Policy {
	return NewMultiStageRateLimiter(
		NewDynamicRateLimiter(func() float64 {
			return globalRps
		}),
		NewCollection(NewSimpleDynamicRateLimiterFactory(func(domain string) int {
			return domainRps
		})),
	)
}
func getDomains(n int) []string {
	domains := make([]string, 0, n)
	for i := 0; i < n; i++ {
		domains = append(domains, fmt.Sprintf("domains%v", i))
	}
	return domains
}
