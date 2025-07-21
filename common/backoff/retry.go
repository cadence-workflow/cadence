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
	"sync"
	"time"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/types"
)

type retryCountKeyType string

const retryCountKey = retryCountKeyType("retryCount")

type (
	// Operation to retry
	Operation func(ctx context.Context) error

	// IsRetryable handler can be used to exclude certain errors during retry
	IsRetryable func(error) bool

	// ConcurrentRetrier is used for client-side throttling. It determines whether to
	// throttle outgoing traffic in case downstream backend server rejects
	// requests due to out-of-quota or server busy errors.
	ConcurrentRetrier struct {
		sync.Mutex
		retrier      Retrier // Backoff retrier
		failureCount int64   // Number of consecutive failures seen
	}

	// ThrottleRetryOption sets the options of ThrottleRetry
	ThrottleRetryOption func(*ThrottleRetry)

	// ThrottleRetry is used to run operation with retry and also avoid throttling dependencies
	ThrottleRetry struct {
		policy         RetryPolicy
		isRetryable    IsRetryable
		throttlePolicy RetryPolicy
		isThrottle     IsRetryable
		clock          clock.TimeSource
	}
)

// Throttle Sleep if there were failures since the last success call.
func (c *ConcurrentRetrier) Throttle() {
	c.throttleInternal()
}

func (c *ConcurrentRetrier) throttleInternal() time.Duration {
	next := done

	// Check if we have failure count.
	failureCount := c.failureCount
	if failureCount > 0 {
		defer c.Unlock()
		c.Lock()
		if c.failureCount > 0 {
			next = c.retrier.NextBackOff()
		}
	}

	if next != done {
		time.Sleep(next)
	}

	return next
}

// Succeeded marks client request succeeded.
func (c *ConcurrentRetrier) Succeeded() {
	defer c.Unlock()
	c.Lock()
	c.failureCount = 0
	c.retrier.Reset()
}

// Failed marks client request failed because backend is busy.
func (c *ConcurrentRetrier) Failed() {
	defer c.Unlock()
	c.Lock()
	c.failureCount++
}

// NewConcurrentRetrier returns an instance of concurrent backoff retrier.
func NewConcurrentRetrier(retryPolicy RetryPolicy) *ConcurrentRetrier {
	retrier := NewRetrier(retryPolicy, clock.NewRealTimeSource())
	return &ConcurrentRetrier{retrier: retrier}
}

// NewThrottleRetry returns a retry handler with given options
func NewThrottleRetry(opts ...ThrottleRetryOption) *ThrottleRetry {
	retryPolicy := NewExponentialRetryPolicy(50 * time.Millisecond)
	retryPolicy.SetMaximumInterval(2 * time.Second)
	throttlePolicy := NewExponentialRetryPolicy(time.Second)
	throttlePolicy.SetMaximumInterval(10 * time.Second)
	throttlePolicy.SetExpirationInterval(NoInterval)
	tr := &ThrottleRetry{
		policy: retryPolicy,
		isRetryable: func(_ error) bool {
			return false
		},
		throttlePolicy: throttlePolicy,
		isThrottle: func(err error) bool {
			_, ok := err.(*types.ServiceBusyError)
			return ok
		},
		clock: clock.NewRealTimeSource(),
	}
	for _, opt := range opts {
		opt(tr)
	}
	return tr
}

// WithRetryPolicy returns a setter setting the retry policy of ThrottleRetry
func WithRetryPolicy(policy RetryPolicy) ThrottleRetryOption {
	return func(tr *ThrottleRetry) {
		tr.policy = policy
	}
}

// WithThrottlePolicy returns setter setting the retry policy when operation returns throttle error
func WithThrottlePolicy(throttlePolicy RetryPolicy) ThrottleRetryOption {
	return func(tr *ThrottleRetry) {
		tr.throttlePolicy = throttlePolicy
	}
}

// WithRetryableError returns a setter setting the retryable error of ThrottleRetry
func WithRetryableError(isRetryable IsRetryable) ThrottleRetryOption {
	return func(tr *ThrottleRetry) {
		tr.isRetryable = isRetryable
	}
}

// WithThrottleError returns a setter setting the throttle error of ThrottleRetry
func WithThrottleError(isThrottle IsRetryable) ThrottleRetryOption {
	return func(tr *ThrottleRetry) {
		tr.isThrottle = isThrottle
	}
}

func WithClock(clock clock.TimeSource) ThrottleRetryOption {
	return func(tr *ThrottleRetry) {
		tr.clock = clock
	}
}

// Do function can be used to wrap any call with retry logic
func (tr *ThrottleRetry) Do(ctx context.Context, op Operation) error {
	var prevErr error
	var err error
	var next time.Duration

	r := NewRetrier(tr.policy, tr.clock)
	t := NewRetrier(tr.throttlePolicy, tr.clock)
	retryCount := 0
	for {
		// record the previous error before an operation
		prevErr = err

		// update context with retry count
		ctx = context.WithValue(ctx, retryCountKey, retryCount)
		// operation completed successfully. No need to retry.
		if err = op(ctx); err == nil {
			return nil
		}
		retryCount++

		// Check if the error is retryable
		if !tr.isRetryable(err) {
			// The returned error will be preferred to a previous one if one exists. That's because the
			// very last error is very likely a timeout error, and it's not useful for logging/troubleshooting
			if prevErr != nil {
				return prevErr
			}
			return err
		}

		if next = r.NextBackOff(); next == done {
			if prevErr != nil {
				return prevErr
			}
			return err
		}

		// check if the error is a throttle error
		if tr.isThrottle(err) {
			throttleBackOff := t.NextBackOff()
			if throttleBackOff != done && throttleBackOff > next {
				next = throttleBackOff
			}
		}

		select {
		case <-ctx.Done():
			if prevErr != nil {
				return prevErr
			}
			return err
		case <-tr.clock.After(next):
		}
	}
}

// IgnoreErrors can be used as IsRetryable handler for Retry function to exclude certain errors from the retry list
func IgnoreErrors(errorsToExclude []error) func(error) bool {
	return func(err error) bool {
		for _, errorToExclude := range errorsToExclude {
			if err == errorToExclude {
				return false
			}
		}

		return true
	}
}
