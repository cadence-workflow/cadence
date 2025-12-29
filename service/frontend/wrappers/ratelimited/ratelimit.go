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

	"github.com/uber/cadence/common/quotas"
)

// ratelimitType differentiates between the three categories of ratelimiters
type ratelimitType int

const (
	ratelimitTypeUser ratelimitType = iota + 1
	ratelimitTypeWorker
	ratelimitTypeVisibility
	ratelimitTypeAsync
)

func (h *apiHandler) allowDomain(ctx context.Context, requestType ratelimitType, domain string) (bool, error) {
	info := quotas.Info{Domain: domain}
	var policy quotas.Policy
	switch requestType {
	case ratelimitTypeUser:
		policy = h.userRateLimiter
	case ratelimitTypeWorker:
		policy = h.workerRateLimiter
	case ratelimitTypeVisibility:
		policy = h.visibilityRateLimiter
	case ratelimitTypeAsync:
		policy = h.asyncRateLimiter
	default:
		panic("coding error, unrecognized request ratelimit type value")
	}
	
	// If context has a deadline, use Wait() to potentially wait for a token
	// Otherwise, use Allow() for an immediate check
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		err := policy.Wait(ctx, info)
		if err != nil {
			ctxErr := ctx.Err()
			switch ctxErr {
				case nil:
					return false, nil // rate limited
				case context.DeadlineExceeded:
					return policy.Allow(info), nil // Race condition: context deadline hit right around wait completion
				default:
					return false, ctxErr
			}
		}
		return true, nil
	}
	
	return policy.Allow(info), nil
}
