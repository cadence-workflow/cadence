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
	"context"
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
)

func TestMultiStageRateLimiterBlockedByDomainRps(t *testing.T) {
	t.Parallel()
	policy := newFixedRpsMultiStageRateLimiter(t, 2, 1)
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
	policy := newFixedRpsMultiStageRateLimiter(t, 1, 2)
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
	policy := newFixedRpsMultiStageRateLimiter(t, 2, 1) // should allow 1/s per domain, 2/s total

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

func TestMultiStageRateLimiterWait(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name        string
		policy      Policy
		info        Info
		expectedErr error
	}{
		{
			name:        "both allow",
			policy:      newFixedRpsMultiStageRateLimiter(t, 2, 1),
			info:        Info{Domain: defaultDomain},
			expectedErr: nil,
		},
		{
			name:        "global blocks",
			policy:      newFixedRpsMultiStageRateLimiter(t, 0, 1),
			info:        Info{Domain: defaultDomain},
			expectedErr: clock.ErrCannotWait,
		},
		{
			name:        "domain blocks",
			policy:      newFixedRpsMultiStageRateLimiter(t, 2, 0),
			info:        Info{Domain: defaultDomain},
			expectedErr: clock.ErrCannotWait,
		},
		{
			name:        "both block",
			policy:      newFixedRpsMultiStageRateLimiter(t, 0, 0),
			info:        Info{Domain: defaultDomain},
			expectedErr: clock.ErrCannotWait,
		},
		{
			name:        "empty domain uses global only - allow",
			policy:      newFixedRpsMultiStageRateLimiter(t, 1, 0),
			info:        Info{Domain: ""},
			expectedErr: nil,
		},
		{
			name:        "empty domain uses global only - block",
			policy:      newFixedRpsMultiStageRateLimiter(t, 0, 0),
			info:        Info{Domain: ""},
			expectedErr: clock.ErrCannotWait,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := tc.policy
			err := policy.Wait(ctx, tc.info)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func BenchmarkMultiStageRateLimiter(b *testing.B) {
	policy := newFixedRpsMultiStageRateLimiter(b, defaultRps, defaultRps)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: defaultDomain})
	}
}

func BenchmarkMultiStageRateLimiter20Domains(b *testing.B) {
	numDomains := 20
	policy := newFixedRpsMultiStageRateLimiter(b, defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func BenchmarkMultiStageRateLimiter100Domains(b *testing.B) {
	numDomains := 100
	policy := newFixedRpsMultiStageRateLimiter(b, defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func BenchmarkMultiStageRateLimiter1000Domains(b *testing.B) {
	numDomains := 1000
	policy := newFixedRpsMultiStageRateLimiter(b, defaultRps, defaultRps)
	domains := getDomains(numDomains)
	for n := 0; n < b.N; n++ {
		policy.Allow(Info{Domain: domains[n%numDomains]})
	}
}

func TestDynamicRateLimiter_RepeatedCalls(t *testing.T) {
	ttl := time.Second
	mockTime := clock.NewMockedTimeSource()
	// Use the number of times the function has been called as a counter and ensure that we make no extra calls
	rps := 0
	rpsFunc := func() float64 {
		res := rps
		rps = rps + 1
		return float64(res)
	}
	limiter := NewDynamicRateLimiterWithOpts(rpsFunc, DynamicRateLimiterOpts{
		TTL:        time.Second,
		MinBurst:   0,
		TimeSource: mockTime,
	})
	assert.Equal(t, limiter.Limit(), rate.Limit(0))
	assert.Equal(t, limiter.Limit(), rate.Limit(0))
	mockTime.Advance(ttl - 1)
	assert.Equal(t, limiter.Limit(), rate.Limit(0))
	mockTime.Advance(time.Nanosecond)
	assert.Equal(t, limiter.Limit(), rate.Limit(1))
	assert.Equal(t, limiter.Limit(), rate.Limit(1))
	// Offset the time from the TTL and ensure it's still respected
	mockTime.Advance(1500 * time.Millisecond)
	assert.Equal(t, limiter.Limit(), rate.Limit(2))
	mockTime.Advance(time.Second - 1)
	assert.Equal(t, limiter.Limit(), rate.Limit(2))
	mockTime.Advance(time.Nanosecond)
	assert.Equal(t, limiter.Limit(), rate.Limit(3))
}

func TestDynamicRateLimiter_Allow(t *testing.T) {
	mockTime := clock.NewMockedTimeSource()
	rps := 0
	limiter := NewDynamicRateLimiterWithOpts(func() float64 {
		return float64(rps)
	}, DynamicRateLimiterOpts{
		TTL:        time.Second,
		MinBurst:   1,
		TimeSource: mockTime,
	})
	assert.Equal(t, false, limiter.Allow())
	rps = 1
	assert.Equal(t, false, limiter.Allow())
	mockTime.Advance(time.Second)
	assert.Equal(t, true, limiter.Allow())
}

func TestDynamicRateLimiter_Limit(t *testing.T) {
	mockTime := clock.NewMockedTimeSource()
	rps := 0
	limiter := NewDynamicRateLimiterWithOpts(func() float64 {
		return float64(rps)
	}, DynamicRateLimiterOpts{
		TTL:        time.Second,
		MinBurst:   1,
		TimeSource: mockTime,
	})
	assert.Equal(t, rate.Limit(0), limiter.Limit())
	rps = 1
	assert.Equal(t, rate.Limit(0), limiter.Limit())
	mockTime.Advance(time.Second)
	assert.Equal(t, rate.Limit(1), limiter.Limit())
}

func TestDynamicRateLimiter_Reserve(t *testing.T) {
	mockTime := clock.NewMockedTimeSource()
	rps := 0
	limiter := NewDynamicRateLimiterWithOpts(func() float64 {
		return float64(rps)
	}, DynamicRateLimiterOpts{
		TTL:        time.Second,
		MinBurst:   1,
		TimeSource: mockTime,
	})

	res := limiter.Reserve()
	assert.Equal(t, false, res.Allow())
	res.Used(false)

	rps = 1
	res = limiter.Reserve()
	assert.Equal(t, false, res.Allow())
	res.Used(false)

	mockTime.Advance(time.Second)
	res = limiter.Reserve()
	assert.Equal(t, true, res.Allow())
	res.Used(true)
}

func TestDynamicRateLimiter_Wait(t *testing.T) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancelFunc)
	mockTime := clock.NewMockedTimeSource()
	rps := 0
	limiter := NewDynamicRateLimiterWithOpts(func() float64 {
		return float64(rps)
	}, DynamicRateLimiterOpts{
		TTL:        time.Second,
		MinBurst:   1,
		TimeSource: mockTime,
	})

	err := limiter.Wait(ctx)
	assert.ErrorIs(t, err, clock.ErrCannotWait)

	rps = 1
	err = limiter.Wait(ctx)
	assert.ErrorIs(t, err, clock.ErrCannotWait)

	mockTime.Advance(time.Second)
	err = limiter.Wait(ctx)
	assert.NoError(t, err)
}

func newFixedRpsMultiStageRateLimiter(t testing.TB, globalRps float64, domainRps int) Policy {
	return NewMultiStageRateLimiter(
		NewDynamicRateLimiter(func() float64 {
			return globalRps
		}),
		NewCollection(newStubFactory(t, domainRps)),
	)
}

func getDomains(n int) []string {
	domains := make([]string, 0, n)
	for i := 0; i < n; i++ {
		domains = append(domains, fmt.Sprintf("domains%v", i))
	}
	return domains
}

func newStubFactory(t testing.TB, rps int) *stubLimiterFactory {
	return &stubLimiterFactory{
		t:   t,
		rps: rps,
	}
}

type stubLimiterFactory struct {
	t   testing.TB
	rps int
}

func (s *stubLimiterFactory) GetLimiter(domain string) Limiter {
	return clock.NewRatelimiter(rate.Limit(s.rps), s.rps)
}
