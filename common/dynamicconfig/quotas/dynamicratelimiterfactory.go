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

package quotas

import (
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/quotas"
)

// NewSimpleDynamicRateLimiterFactory creates a new LimiterFactory which creates
// a new DynamicRateLimiter for each domain, the RPS for the DynamicRateLimiter is given by the dynamic config
func NewSimpleDynamicRateLimiterFactory(rps dynamicconfig.IntPropertyFnWithDomainFilter) quotas.LimiterFactory {
	return dynamicRateLimiterFactory{
		rps: rps,
	}
}

type dynamicRateLimiterFactory struct {
	rps dynamicconfig.IntPropertyFnWithDomainFilter
}

// GetLimiter returns a new Limiter for the given domain
func (f dynamicRateLimiterFactory) GetLimiter(domain string) quotas.Limiter {
	return quotas.NewDynamicRateLimiter(func() float64 { return float64(f.rps(domain)) })
}
