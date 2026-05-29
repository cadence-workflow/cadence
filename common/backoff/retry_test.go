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

package backoff

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type (
	RetrySuite struct {
		*require.Assertions // override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test, not merely log an error
		suite.Suite
	}

	someError struct{}
)

func TestRetrySuite(t *testing.T) {
	suite.Run(t, new(RetrySuite))
}

func (s *RetrySuite) SetupTest() {
	s.Assertions = require.New(s.T()) // Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
}

func (s *RetrySuite) TestRetrySuccess() {
	i := 0
	op := func(ctx context.Context) error {
		i++

		if i == 5 {
			return nil
		}

		return &someError{}
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	policy.SetMaximumAttempts(10)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.NoError(err)
	s.Equal(5, i)
}

func (s *RetrySuite) TestRetryFailed() {
	i := 0
	op := func(ctx context.Context) error {
		i++

		if i == 7 {
			return nil
		}

		return &someError{}
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	policy.SetMaximumAttempts(5)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.Error(err)
}

func (s *RetrySuite) TestRetryFailedReturnPreviousError() {
	i := 0

	the5thError := fmt.Errorf("this is the error of the 5th attempt(4th retry attempt)")
	op := func(ctx context.Context) error {
		i++
		if i == 5 {
			return the5thError
		}
		if i == 7 {
			return nil
		}

		return &someError{}
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	// Note that this is retry attempts(maybe it should be renamed to SetMaximumRetryAttempts),
	// so the total attempts is 5+1=6
	policy.SetMaximumAttempts(5)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.Error(err)
	s.Equal(6, i)
	s.Equal(the5thError, err)
}

func (s *RetrySuite) TestIsRetryableSuccess() {
	i := 0
	op := func(ctx context.Context) error {
		i++

		if i == 5 {
			return nil
		}

		return &someError{}
	}

	isRetryable := func(err error) bool {
		if _, ok := err.(*someError); ok {
			return true
		}

		return false
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	policy.SetMaximumAttempts(10)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(isRetryable),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.NoError(err, "Retry count: %v", i)
	s.Equal(5, i)
}

func (s *RetrySuite) TestIsRetryableFailure() {
	i := 0
	op := func(ctx context.Context) error {
		i++

		if i == 5 {
			return nil
		}

		return &someError{}
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	policy.SetMaximumAttempts(10)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(IgnoreErrors([]error{&someError{}})),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.Error(err)
	s.Equal(1, i)
}

func (s *RetrySuite) TestRetryExpired() {
	i := 0
	op := func(ctx context.Context) error {
		i++
		time.Sleep(time.Second)
		return &someError{}
	}

	policy := NewExponentialRetryPolicy(10 * time.Millisecond)
	policy.SetExpirationInterval(NoInterval)
	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	err := throttleRetry.Do(ctx, op)
	s.Error(err)
	s.Equal(&someError{}, err)
	s.Equal(1, i)
}

func (s *RetrySuite) TestRetryExpiredReturnPreviousError() {
	i := 0
	prevErr := fmt.Errorf("previousError")
	op := func(ctx context.Context) error {
		i++
		if i == 1 {
			return prevErr
		}
		time.Sleep(time.Second)
		return &someError{}
	}

	policy := NewExponentialRetryPolicy(10 * time.Millisecond)
	policy.SetExpirationInterval(NoInterval)
	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := throttleRetry.Do(ctx, op)
	s.Error(err)
	s.Equal(prevErr, err)
	s.Equal(2, i)
}

func (s *RetrySuite) TestRetryThrottled() {
	i := 0
	throttleErr := fmt.Errorf("throttled")
	op := func(ctx context.Context) error {
		i++
		if i == 1 {
			return throttleErr
		}
		return nil
	}

	policy := NewExponentialRetryPolicy(time.Millisecond)
	policy.SetExpirationInterval(NoInterval)
	throttlePolicy := NewExponentialRetryPolicy(time.Second)
	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
		WithThrottlePolicy(throttlePolicy),
		WithThrottleError(func(e error) bool { return e == throttleErr }),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err := throttleRetry.Do(ctx, op)
	s.Error(err)
	s.Equal(throttleErr, err)
	s.Equal(1, i) // Because retry expires before next retry
}

func (s *RetrySuite) TestConcurrentRetrier() {
	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(10 * time.Millisecond)
	policy.SetMaximumAttempts(4)

	// Basic checks
	retrier := NewConcurrentRetrier(policy)
	retrier.Failed()
	s.Equal(int64(1), retrier.failureCount)
	retrier.Succeeded()
	s.Equal(int64(0), retrier.failureCount)
	sleepDuration := retrier.throttleInternal()
	s.Equal(done, sleepDuration)

	// Multiple count check.
	retrier.Failed()
	retrier.Failed()
	s.Equal(int64(2), retrier.failureCount)
	// Verify valid sleep times.
	ch := make(chan time.Duration, 3)
	go func() {
		for i := 0; i < 3; i++ {
			ch <- retrier.throttleInternal()
		}
	}()
	for i := 0; i < 3; i++ {
		val := <-ch
		fmt.Printf("Duration: %d\n", val)
		s.True(val > 0)
	}
	retrier.Succeeded()
	s.Equal(int64(0), retrier.failureCount)
	// Verify we don't have any sleep times.
	go func() {
		for i := 0; i < 3; i++ {
			ch <- retrier.throttleInternal()
		}
	}()
	for i := 0; i < 3; i++ {
		val := <-ch
		fmt.Printf("Duration: %d\n", val)
		s.Equal(done, val)
	}
}

func (s *RetrySuite) TestRetryCountInContext() {
	retryCounts := make([]int, 0)
	op := func(ctx context.Context) error {
		retryCount := ctx.Value(retryCountKey).(int)
		retryCounts = append(retryCounts, retryCount)
		if retryCount == 2 {
			return nil
		}
		return &someError{}
	}

	policy := NewExponentialRetryPolicy(1 * time.Millisecond)
	policy.SetMaximumInterval(5 * time.Millisecond)
	policy.SetMaximumAttempts(10)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		WithRetryableError(func(_ error) bool { return true }),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.NoError(err)
	s.Equal([]int{0, 1, 2}, retryCounts)
}

func (s *RetrySuite) TestContextDeadline() {
	cases := []struct {
		name            string
		policy          func() RetryPolicy
		enforceDeadline bool
		opts            []ThrottleRetryOption
		expectedMin     time.Duration
		expectedMax     time.Duration
	}{
		{
			name: "no deadline",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(NoInterval)
				return policy
			},
			enforceDeadline: true,
			expectedMin:     0,
			expectedMax:     0,
		},
		{
			name: "don't enforce deadline",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(time.Hour)
				return policy
			},
			enforceDeadline: false,
			expectedMin:     0,
			expectedMax:     0,
		},
		{
			name: "expiration interval",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(time.Hour)
				return policy
			},
			enforceDeadline: true,
			expectedMin:     50 * time.Minute,
			expectedMax:     70 * time.Minute,
		},
		{
			name: "operation timeout",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(NoInterval)
				return policy
			},
			enforceDeadline: true,
			opts: []ThrottleRetryOption{
				WithOperationTimeout(time.Hour),
			},
			expectedMin: 50 * time.Minute,
			expectedMax: 70 * time.Minute,
		},
		{
			name: "expiration interval lower, take lower",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(time.Hour)
				return policy
			},
			enforceDeadline: true,
			opts: []ThrottleRetryOption{
				WithOperationTimeout(10 * time.Hour),
			},
			expectedMin: 50 * time.Minute,
			expectedMax: 70 * time.Minute,
		},
		{
			name: "operation timeout lower, take lower",
			policy: func() RetryPolicy {
				policy := NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetExpirationInterval(10 * time.Hour)
				return policy
			},
			enforceDeadline: true,
			opts: []ThrottleRetryOption{
				WithOperationTimeout(time.Hour),
			},
			expectedMin: 50 * time.Minute,
			expectedMax: 70 * time.Minute,
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			var opts []ThrottleRetryOption
			opts = append(opts, WithRetryPolicy(tc.policy()))
			if tc.enforceDeadline {
				opts = append(opts, WithContextExpiration())
			}
			if len(tc.opts) > 0 {
				opts = append(opts, tc.opts...)
			}
			throttleRetry := NewThrottleRetry(opts...)
			var result time.Duration

			err := throttleRetry.Do(context.Background(), func(ctx context.Context) error {
				if deadline, ok := ctx.Deadline(); ok {
					result = deadline.Sub(time.Now())
				} else {
					result = 0
				}
				return nil
			})
			s.NoError(err, "somehow failed")
			s.GreaterOrEqual(result, tc.expectedMin)
			s.LessOrEqual(result, tc.expectedMax)
		})
	}
}

func (s *RetrySuite) TestOperationTimeoutIsRetryable() {
	attempts := 0
	op := func(ctx context.Context) error {
		retryCount := ctx.Value(retryCountKey).(int)
		attempts = max(attempts, retryCount)
		// Return nil, indicating success
		if retryCount == 3 {
			return nil
		}
		// Return the DeadlineExceeded err
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	policy := NewExponentialRetryPolicy(time.Nanosecond)
	policy.SetMaximumInterval(time.Nanosecond)
	policy.SetMaximumAttempts(10)

	throttleRetry := NewThrottleRetry(
		WithRetryPolicy(policy),
		// Operation timeout is retryable anyway
		WithRetryableError(func(_ error) bool { return false }),
		// Set a timeout such that it will instantly be canceled every time
		WithOperationTimeout(time.Nanosecond),
	)

	err := throttleRetry.Do(context.Background(), op)
	s.NoError(err, "somehow failed")
	s.Equal(3, attempts)
}

func (e *someError) Error() string {
	return "Some Error"
}
