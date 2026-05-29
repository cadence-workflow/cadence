// Copyright (c) 2025 Uber Technologies, Inc.
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

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

// CallerBypass encapsulates the logic for bypassing rate limits based on caller type
type CallerBypass struct {
	bypassCallerTypes dynamicproperties.ListPropertyFn
}

// NewCallerBypass creates a new CallerBypass with the given bypass caller types configuration
func NewCallerBypass(bypassCallerTypes dynamicproperties.ListPropertyFn) CallerBypass {
	return CallerBypass{
		bypassCallerTypes: bypassCallerTypes,
	}
}

// AllowLimiter checks if a request should be allowed through a Limiter.
// It first checks the limiter's Allow() method, and if that returns false,
// it checks if the caller type should bypass rate limiting.
func (c CallerBypass) AllowLimiter(ctx context.Context, limiter Limiter) bool {
	if limiter.Allow() {
		return true
	}
	return c.ShouldBypass(ctx)
}

// AllowPolicy checks if a request should be allowed through a Policy.
// It first checks the policy's Allow() method, and if that returns false,
// it checks if the caller type should bypass rate limiting.
func (c CallerBypass) AllowPolicy(ctx context.Context, policy Policy, info Info) bool {
	if policy.Allow(info) {
		return true
	}
	return c.ShouldBypass(ctx)
}

// ShouldBypass checks if the caller type from the context should bypass rate limiting
// based on the configured bypass caller types.
func (c CallerBypass) ShouldBypass(ctx context.Context) bool {
	if c.bypassCallerTypes == nil {
		return false
	}

	callerInfo := types.GetCallerInfoFromContext(ctx)
	bypassCallerTypes := c.bypassCallerTypes()

	for _, bypassType := range bypassCallerTypes {
		if bypassTypeStr, ok := bypassType.(string); ok {
			if types.ParseCallerType(bypassTypeStr) == callerInfo.GetCallerType() {
				return true
			}
		}
	}
	return false
}
