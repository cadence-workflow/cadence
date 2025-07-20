// Copyright (c) 2022 Uber Technologies, Inc.
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

package permember

import (
	"math"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/quotas"
)

// PerMember allows creating per instance RPS based on globalRPS averaged by member count for a given service.
// If member count can not be retrieved or globalRPS is not provided it falls back to instanceRPS.
func PerMember(service string, globalRPS, instanceRPS float64, resolver membership.Resolver) float64 {
	if globalRPS <= 0 {
		return instanceRPS
	}

	memberCount, err := resolver.MemberCount(service)
	if err != nil || memberCount < 1 {
		return instanceRPS
	}

	avgQuota := math.Max(globalRPS/float64(memberCount), 1)
	return math.Min(avgQuota, instanceRPS)
}

// NewPerMemberDynamicRateLimiterFactory creates a new LimiterFactory which creates
// a new DynamicRateLimiter for each domain, the RPS for the DynamicRateLimiter is given
// by the globalRPS and averaged by member count for a given service.
// instanceRPS is used as a fallback if globalRPS is not provided.
func NewPerMemberDynamicRateLimiterFactory(
	service string,
	globalRPS dynamicproperties.IntPropertyFnWithDomainFilter,
	instanceRPS dynamicproperties.IntPropertyFnWithDomainFilter,
	resolver membership.Resolver,
) quotas.LimiterFactory {
	return perMemberFactory{
		service:     service,
		globalRPS:   globalRPS,
		instanceRPS: instanceRPS,
		resolver:    resolver,
	}
}

func NewPerMemberBurstyDynamicRateLimiterFactory(
	service string,
	globalRPS dynamicproperties.IntPropertyFnWithDomainFilter,
	instanceRPS dynamicproperties.IntPropertyFnWithDomainFilter,
	resolver membership.Resolver,
	burstMultiplier float64,
) quotas.LimiterFactory {
	return perMemberFactory{
		service:     service,
		globalRPS:   globalRPS,
		instanceRPS: instanceRPS,
		resolver:    resolver,
		burstMultiplier: burstMultiplier,
	}
}

type perMemberFactory struct {
	service     string
	globalRPS   dynamicproperties.IntPropertyFnWithDomainFilter
	instanceRPS dynamicproperties.IntPropertyFnWithDomainFilter
	resolver    membership.Resolver
	burstMultiplier float64
}

func (f perMemberFactory) GetLimiter(domain string) quotas.Limiter {
	return quotas.NewBurstyDynamicRateLimiter(func() float64 {
		return PerMember(
			f.service,
			float64(f.globalRPS(domain)),
			float64(f.instanceRPS(domain)),
			f.resolver,
		)
	}, f.burstMultiplier)
}
