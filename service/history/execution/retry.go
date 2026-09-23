// Copyright (c) 2020 Uber Technologies, Inc.
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

package execution

import (
	"math"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/types"
)

// backoffInterval is calculated in seconds, then multiplied by time.Second. This scales it by 1e9 and would result
// in an overflow for any value above this.
var maxBackoffIntervalSeconds = int64(math.Trunc(math.MaxInt64 / 1e9))

func shouldRetry(
	nextScheduledTime time.Time,
	currAttempt int32,
	maxAttempts int32,
	expirationTime time.Time,
	failureReason string,
	nonRetriableErrors []string,
	failureCategory types.FailureCategory,
) bool {
	if failureCategory == types.FailureCategoryFatal {
		return false
	}
	if maxAttempts == 0 && expirationTime.IsZero() {
		return false
	}

	if maxAttempts > 0 && currAttempt >= maxAttempts-1 {
		// currAttempt starts from 0.
		// MaximumAttempts is the total attempts, including initial (non-retry) attempt.
		return false
	}
	if !expirationTime.IsZero() && nextScheduledTime.After(expirationTime) {
		return false
	}
	// make sure we don't retry size exceeded error reasons. Note that FailureReasonFailureDetailsExceedsLimit is retryable.
	if failureReason == common.FailureReasonCancelDetailsExceedsLimit ||
		failureReason == common.FailureReasonCompleteResultExceedsLimit ||
		failureReason == common.FailureReasonHeartbeatExceedsLimit ||
		failureReason == common.FailureReasonDecisionBlobSizeExceedsLimit {
		return false
	}

	// check if error is non-retriable
	for _, er := range nonRetriableErrors {
		if er == failureReason {
			return false
		}
	}

	return true
}

func getBackoffInterval(
	currAttempt int32,
	initInterval int32,
	maxInterval int32,
	backoffCoefficient float64,
) time.Duration {

	nextInterval := int64(float64(initInterval) * math.Pow(backoffCoefficient, float64(currAttempt)))
	// Handle overflows and ensure multiplying it by time.Second won't overflow later
	if nextInterval <= 0 || nextInterval > maxBackoffIntervalSeconds {
		if maxInterval > 0 {
			nextInterval = int64(maxInterval)
		} else {
			return backoff.NoBackoff
		}
	}

	if maxInterval > 0 && nextInterval > int64(maxInterval) {
		// cap next interval to MaxInterval
		nextInterval = int64(maxInterval)
	}

	return time.Duration(nextInterval) * time.Second
}
