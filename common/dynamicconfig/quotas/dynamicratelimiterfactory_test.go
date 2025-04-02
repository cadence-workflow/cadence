package quotas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
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
