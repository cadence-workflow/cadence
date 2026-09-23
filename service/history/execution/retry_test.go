package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/types"
)

func TestShouldRetry(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name               string
		nextScheduledTime  time.Time
		currAttempt        int32
		maxAttempts        int32
		expirationTime     time.Time
		failureReason      string
		nonRetriableErrors []string
		failureCategory    types.FailureCategory
		expected           bool
	}{
		{
			name:              "fatal failure category",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryFatal,
			expected:          false,
		},
		{
			name:              "standard failure category allows retry",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:              "poll failure category allows retry",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryPoll,
			expected:          true,
		},
		{
			name:              "no policy - zero max attempts and zero expiration",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       0,
			expirationTime:    time.Time{},
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "unlimited retries with expiration set",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       0,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:              "max attempts reached - attempt equals max minus one",
			nextScheduledTime: baseTime.Add(time.Second),
			currAttempt:       4,
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "max attempts not reached - attempt less than max minus one",
			nextScheduledTime: baseTime.Add(time.Second),
			currAttempt:       3,
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:              "max attempts of 1 means no retry",
			nextScheduledTime: baseTime.Add(time.Second),
			currAttempt:       0,
			maxAttempts:       1,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "next schedule time after expiration",
			nextScheduledTime: baseTime.Add(2 * time.Hour),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "next schedule time before expiration",
			nextScheduledTime: baseTime.Add(30 * time.Minute),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:              "zero expiration time means unlimited",
			nextScheduledTime: baseTime.Add(time.Hour),
			maxAttempts:       5,
			expirationTime:    time.Time{},
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:              "cancel details exceeds limit",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureReason:     common.FailureReasonCancelDetailsExceedsLimit,
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "complete result exceeds limit",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureReason:     common.FailureReasonCompleteResultExceedsLimit,
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "heartbeat exceeds limit",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureReason:     common.FailureReasonHeartbeatExceedsLimit,
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "decision blob size exceeds limit",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureReason:     common.FailureReasonDecisionBlobSizeExceedsLimit,
			failureCategory:   types.FailureCategoryStandard,
			expected:          false,
		},
		{
			name:              "failure details exceeds limit is retryable",
			nextScheduledTime: baseTime.Add(time.Second),
			maxAttempts:       5,
			expirationTime:    baseTime.Add(time.Hour),
			failureReason:     common.FailureReasonFailureDetailsExceedsLimit,
			failureCategory:   types.FailureCategoryStandard,
			expected:          true,
		},
		{
			name:               "failure reason matches non-retriable error",
			nextScheduledTime:  baseTime.Add(time.Second),
			maxAttempts:        5,
			expirationTime:     baseTime.Add(time.Hour),
			failureReason:      "bad-reason",
			nonRetriableErrors: []string{"bad-reason", "ugly-reason"},
			failureCategory:    types.FailureCategoryStandard,
			expected:           false,
		},
		{
			name:               "failure reason does not match non-retriable errors",
			nextScheduledTime:  baseTime.Add(time.Second),
			maxAttempts:        5,
			expirationTime:     baseTime.Add(time.Hour),
			failureReason:      "good-reason",
			nonRetriableErrors: []string{"bad-reason", "ugly-reason"},
			failureCategory:    types.FailureCategoryStandard,
			expected:           true,
		},
		{
			name:               "empty non-retriable errors list",
			nextScheduledTime:  baseTime.Add(time.Second),
			maxAttempts:        5,
			expirationTime:     baseTime.Add(time.Hour),
			failureReason:      "any-reason",
			nonRetriableErrors: nil,
			failureCategory:    types.FailureCategoryStandard,
			expected:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := shouldRetry(
				tc.nextScheduledTime,
				tc.currAttempt,
				tc.maxAttempts,
				tc.expirationTime,
				tc.failureReason,
				tc.nonRetriableErrors,
				tc.failureCategory,
			)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetBackoffInterval(t *testing.T) {
	tests := []struct {
		name               string
		currAttempt        int32
		initInterval       int32
		maxInterval        int32
		backoffCoefficient float64
		expected           time.Duration
	}{
		{
			name:               "first attempt with coefficient 1",
			currAttempt:        0,
			initInterval:       1,
			maxInterval:        0,
			backoffCoefficient: 1,
			expected:           time.Second,
		},
		{
			name:               "exponential backoff - attempt 1",
			currAttempt:        1,
			initInterval:       1,
			maxInterval:        0,
			backoffCoefficient: 2,
			expected:           2 * time.Second,
		},
		{
			name:               "exponential backoff - attempt 2",
			currAttempt:        2,
			initInterval:       1,
			maxInterval:        0,
			backoffCoefficient: 2,
			expected:           4 * time.Second,
		},
		{
			name:               "exponential backoff - attempt 3",
			currAttempt:        3,
			initInterval:       1,
			maxInterval:        0,
			backoffCoefficient: 2,
			expected:           8 * time.Second,
		},
		{
			name:               "max interval caps the backoff",
			currAttempt:        4,
			initInterval:       1,
			maxInterval:        10,
			backoffCoefficient: 2,
			expected:           10 * time.Second,
		},
		{
			name:               "interval exactly at max",
			currAttempt:        3,
			initInterval:       1,
			maxInterval:        8,
			backoffCoefficient: 2,
			expected:           8 * time.Second,
		},
		{
			name:               "interval below max is not capped",
			currAttempt:        2,
			initInterval:       1,
			maxInterval:        10,
			backoffCoefficient: 2,
			expected:           4 * time.Second,
		},
		{
			name:               "overflow with max interval falls back to max",
			currAttempt:        64,
			initInterval:       1,
			maxInterval:        10,
			backoffCoefficient: 2,
			expected:           10 * time.Second,
		},
		{
			name:               "overflow without max interval returns no backoff",
			currAttempt:        64,
			initInterval:       1,
			maxInterval:        0,
			backoffCoefficient: 2,
			expected:           backoff.NoBackoff,
		},
		{
			name:               "zero init interval with max interval falls back to max",
			currAttempt:        0,
			initInterval:       0,
			maxInterval:        5,
			backoffCoefficient: 2,
			expected:           5 * time.Second,
		},
		{
			name:               "zero init interval without max interval returns no backoff",
			currAttempt:        0,
			initInterval:       0,
			maxInterval:        0,
			backoffCoefficient: 2,
			expected:           backoff.NoBackoff,
		},
		{
			name:               "larger init interval",
			currAttempt:        0,
			initInterval:       60,
			maxInterval:        6000,
			backoffCoefficient: 1,
			expected:           60 * time.Second,
		},
		{
			name:               "larger init interval with backoff",
			currAttempt:        2,
			initInterval:       10,
			maxInterval:        0,
			backoffCoefficient: 3,
			expected:           90 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := getBackoffInterval(
				tc.currAttempt,
				tc.initInterval,
				tc.maxInterval,
				tc.backoffCoefficient,
			)
			assert.Equal(t, tc.expected, result)
		})
	}
}
