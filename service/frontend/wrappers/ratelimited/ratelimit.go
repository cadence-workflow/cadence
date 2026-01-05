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

package ratelimited

import (
	"context"
	"time"

	"github.com/uber/cadence/common/quotas"
	"github.com/uber/cadence/common/types"
)

// ratelimitType differentiates between the three categories of ratelimiters
type ratelimitType int

const (
	ratelimitTypeUser ratelimitType = iota + 1
	ratelimitTypeWorker
	ratelimitTypeWorkerPoll
	ratelimitTypeVisibility
	ratelimitTypeAsync
)

var ErrRateLimited = types.ServiceBusyError{Message: "Too many outstanding requests to the cadence service"}

func (h *apiHandler) allowDomain(ctx context.Context, requestType ratelimitType, domain string) error {
	var policy quotas.Policy
	switch requestType {
	case ratelimitTypeUser:
		policy = h.userRateLimiter
	case ratelimitTypeWorker, ratelimitTypeWorkerPoll:
		policy = h.workerRateLimiter
	case ratelimitTypeVisibility:
		policy = h.visibilityRateLimiter
	case ratelimitTypeAsync:
		policy = h.asyncRateLimiter
	default:
		panic("coding error, unrecognized request ratelimit type value")
	}

	// If it is a poll request and there is a maxWorkerPollDelay configured, use Wait()
	// to potentially wait for a token. Otherwise, use Allow() for an immediate check
	if requestType == ratelimitTypeWorkerPoll {
		waitTime := h.maxWorkerPollDelay(domain)
		if waitTime > 0 {
			return h.waitForPolicy(ctx, waitTime, policy, domain)
		}
	}
	if !policy.Allow(quotas.Info{Domain: domain}) {
		return ErrRateLimited
	}
	return nil
}

func (h *apiHandler) waitForPolicy(ctx context.Context, waitTime time.Duration, policy quotas.Policy, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, waitTime*time.Microsecond)
	defer cancel()
	err := policy.Wait(ctx, quotas.Info{Domain: domain})
	if err != nil {
		ctxErr := ctx.Err()
		switch ctxErr {
		case nil:
			return ErrRateLimited // rate limited
		case context.DeadlineExceeded:
			// Race condition: context deadline hit right around wait completion
			if !policy.Allow(quotas.Info{Domain: domain}) {
				return ErrRateLimited
			}
			return nil
		default:
			return ctxErr
		}
	}
	return nil
}
