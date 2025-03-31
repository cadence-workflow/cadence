package quotas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

const (
	_testDomain = "test-domain"
)

func TestNewSimpleDynamicRateLimiterFactory(t *testing.T) {
	factory := NewSimpleDynamicRateLimiterFactory(func(domain string) int {
		assert.Equal(t, _testDomain, domain)
		return 10
	})
	assert.Equal(t, rate.Limit(10), factory.GetLimiter(_testDomain).Limit())
}
