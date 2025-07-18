package tasklist

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/service/matching/config"
)

func TestTaskListLimiter(t *testing.T) {
	const ttl = time.Second
	const defaultRps = 100
	cases := []struct {
		name     string
		fn       func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int)
		expected rate.Limit
	}{
		{
			name:     "no update",
			fn:       func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {},
			expected: rate.Limit(defaultRps),
		},
		{
			name: "take lower",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				limiter.ReportLimit(1)
			},
			expected: rate.Limit(1),
		},
		{
			name: "take lowest",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				limiter.ReportLimit(50)
				limiter.ReportLimit(10)
			},
			expected: rate.Limit(10),
		},
		{
			name: "ignore higher",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				limiter.ReportLimit(defaultRps + 1)
			},
			expected: rate.Limit(defaultRps),
		},
		{
			name: "take higher after ttl",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(ttl)
				limiter.ReportLimit(101)
			},
			expected: rate.Limit(101),
		},
		{
			name: "keep lower if reported within ttl",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(time.Millisecond)
				limiter.ReportLimit(defaultRps)
				mockClock.Advance(ttl - time.Nanosecond)
				limiter.ReportLimit(defaultRps + 1)
			},
			expected: rate.Limit(defaultRps),
		},
		{
			name: "taker higher if lower not recently reported",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(time.Millisecond)
				limiter.ReportLimit(defaultRps)
				mockClock.Advance(ttl + time.Nanosecond)
				limiter.ReportLimit(defaultRps + 1)
			},
			expected: rate.Limit(defaultRps + 1),
		},
		{
			name: "recalculate partitions on ttl expiration - lower limit",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(ttl)
				*numPartitions = 2
				limiter.ReportLimit(50)
			},
			expected: rate.Limit(25),
		},
		{
			name: "recalculate partitions on ttl expiration - equal limit",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(ttl)
				*numPartitions = 2
				limiter.ReportLimit(defaultRps)
			},
			expected: rate.Limit(50),
		},
		{
			name: "recalculate partitions on ttl expiration - higher limit",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				mockClock.Advance(ttl)
				*numPartitions = 2
				limiter.ReportLimit(defaultRps + 2)
			},
			expected: rate.Limit(51),
		},
		{
			name: "recalculate partitions on early updates",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				*numPartitions = 2
				limiter.ReportLimit(50)
			},
			expected: rate.Limit(25),
		},
		{
			name: "recalculate partitions even when keeping lower limit",
			fn: func(mockClock clock.MockedTimeSource, limiter *taskListLimiter, numPartitions *int) {
				// lastUpdate time
				limiter.ReportLimit(50)
				mockClock.Advance(time.Nanosecond)
				limiter.ReportLimit(50)
				mockClock.Advance(ttl - time.Nanosecond)
				*numPartitions = 2
				// 100 won't be applied because we received 50 within the TTL, but
				// it's been TTL since we last did an update so recalculate the partitions
				limiter.ReportLimit(100)
			},
			expected: rate.Limit(25),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clock.NewMockedTimeSource()
			scope := metrics.NoopScope
			numPartitions := 1
			tlConfig := &config.TaskListConfig{
				TaskDispatchRPSTTL: ttl,
				TaskDispatchRPS:    defaultRps,
				MinTaskThrottlingBurstSize: func() int {
					return 1
				},
			}
			limiter := newTaskListLimiter(mockClock, scope, tlConfig, func() int {
				return numPartitions
			})
			tc.fn(mockClock, limiter, &numPartitions)
			assert.Equal(t, tc.expected, limiter.Limit())
		})
	}
}
